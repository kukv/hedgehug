package usecase

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// ghostAuthor is the author of a comment whose user GitHub no longer has.
const ghostAuthor = model.MirroredAuthorPrefix + "ghost"

// commentsLink is the link of links whose issue the comments of the task are
// taken from: the first mirror link of an issue that syncs comments, as for the
// other fields.
func commentsLink(links []model.Link, rules *automation.File) (model.Link, bool) {
	for _, l := range links {
		if l.Type == model.LinkIssue && l.Sync == model.SyncMirror && slices.Contains(rules.IssueFields.For(l), automation.FieldComments) {
			return l, true
		}
	}
	return model.Link{}, false
}

// readMirrorComments reads the comments of the issues of repo that the tasks of
// ix take comments from, into got. An issue that was not read, or whose
// comments cannot be read in full, has none in got, and its comments are left
// as they are.
func readMirrorComments(cl *github.Client, acct config.Account, ix *index.Index, repo string, rules *automation.File, got fetched, rep *GitHubFetchReport) {
	if rules == nil {
		return
	}
	tried := map[string]bool{}
	for _, x := range ix.Tasks() {
		if x.DeletedAt != nil || x.Broken != "" {
			continue
		}
		l, ok := commentsLink(x.Links, rules)
		if !ok {
			continue
		}
		ref := model.CanonRef(l.Ref)
		r, ok := model.ParseGitHubRef(ref)
		if !ok || strings.ToLower(r.Repo) != repo || tried[ref] {
			continue
		}
		if _, read := got.issues[ref]; !read {
			continue
		}
		tried[ref] = true
		parts := strings.SplitN(repo, "/", 3)
		list, err := cl.IssueComments(parts[1], parts[2], r.Number)
		if err != nil {
			rep.Warnings = append(rep.Warnings, github.Describe(err, acct.ID(), acct.Name, "read the comments of "+r.Label()).Error())
			continue
		}
		got.comments[ref] = list
	}
}

// commentMirror is what the mirror writes to the comments of one task: the
// comments as they are to be, new or changed.
type commentMirror struct {
	puts []*model.Comment
	// ghOnly are comments that got their gh back: only gh is written, and
	// updated stays.
	ghOnly []*model.Comment
}

// planCommentMirror compares the comments of the issue that t takes its
// comments from with the comments of t that stand for them, matched by the ID in
// gh, and returns what to write; nil when there is nothing. now is the time of a
// deletion.
func planCommentMirror(s *store.Store, t *model.Task, got fetched, rules *automation.File, now time.Time) (*commentMirror, error) {
	l, ok := commentsLink(t.Links, rules)
	if !ok {
		return nil, nil //nolint:nilnil // no comments to take is no error
	}
	ref := model.CanonRef(l.Ref)
	list, read := got.comments[ref]
	if !read {
		return nil, nil //nolint:nilnil // not read in full: nothing is changed
	}
	byID := map[int64]*model.Comment{}
	byULID := map[model.ULID]*model.Comment{}
	for c, err := range s.Comments(t.ULID) {
		if err != nil {
			return nil, err
		}
		byULID[c.ULID] = c
		links, err := c.GitHub()
		if err != nil {
			return nil, fmt.Errorf("comment %s: %w", c.ULID, err)
		}
		if i := slices.IndexFunc(links, func(x model.CommentLink) bool { return model.CanonRef(x.Ref) == ref }); i >= 0 {
			byID[links[i].ID] = c
		}
	}
	m := &commentMirror{}
	seen := map[int64]bool{}
	relinked := map[model.ULID]bool{}
	// held are the comments whose gh has an ID that GitHub still lists.
	held := map[model.ULID]bool{}
	for _, x := range list {
		if c, ok := byID[x.ID]; ok {
			held[c.ULID] = true
		}
	}
	for _, x := range list {
		seen[x.ID] = true
		text, _, _ := model.SplitComment(x.Body)
		if c, ok := byID[x.ID]; ok {
			if c.DeletedAt != nil || model.NormalizeBody(c.Body) == text {
				continue
			}
			links, _ := c.GitHub() // read above
			i := slices.IndexFunc(links, func(y model.CommentLink) bool { return model.CanonRef(y.Ref) == ref })
			links[i].BodySynced = text
			c.Body = text
			if err := c.SetGitHub(links); err != nil {
				return nil, err
			}
			m.puts = append(m.puts, c)
			continue
		}
		// The mark of a comment of this task: it is that comment, whose gh was
		// lost or has an ID that is gone. The one whose ID gh holds is it (above);
		// without one, the first with the mark gets its gh back, and body, as
		// GitHub is the master here. The others with the mark are not comments.
		if _, mark, marked := model.SplitComment(x.Body); marked && byULID[mark] != nil {
			if held[mark] || relinked[mark] {
				continue
			}
			c := byULID[mark]
			relinked[mark] = true
			links, _ := c.GitHub() // read above
			link := model.CommentLink{Ref: ref, ID: x.ID, BodySynced: text}
			if i := slices.IndexFunc(links, func(y model.CommentLink) bool { return model.CanonRef(y.Ref) == ref }); i >= 0 {
				delete(byID, links[i].ID) // gone from GitHub, but not deleted: this is it
				links[i] = link
			} else {
				links = append(links, link)
			}
			changed := model.NormalizeBody(c.Body) != text
			if changed {
				c.Body = text
			}
			if err := c.SetGitHub(links); err != nil {
				return nil, err
			}
			if changed {
				m.puts = append(m.puts, c)
			} else {
				m.ghOnly = append(m.ghOnly, c)
			}
			continue
		}
		u := model.DerivedULID(x.CreatedAt, fmt.Sprintf("%s#%d", ref, x.ID))
		if byULID[u] != nil {
			continue // the same comment, which another way of making it has recorded
		}
		author := ghostAuthor
		if x.User != nil {
			author = model.MirroredAuthorPrefix + x.User.Login
		}
		c := &model.Comment{ULID: u, Author: author, Created: x.CreatedAt.UTC(), Body: text}
		if err := c.SetGitHub([]model.CommentLink{{Ref: ref, ID: x.ID, BodySynced: text}}); err != nil {
			return nil, err
		}
		m.puts = append(m.puts, c)
	}
	// Gone from GitHub, now that the whole list was read.
	for id, c := range byID {
		if !seen[id] && c.DeletedAt == nil {
			at := now
			c.DeletedAt = &at
			m.puts = append(m.puts, c)
		}
	}
	if len(m.puts)+len(m.ghOnly) == 0 {
		return nil, nil //nolint:nilnil // nothing to write is no error
	}
	slices.SortFunc(m.puts, func(a, b *model.Comment) int { return strings.Compare(a.ULID.String(), b.ULID.String()) })
	return m, nil
}
