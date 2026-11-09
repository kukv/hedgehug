package usecase

import (
	"errors"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// maxCommentsCreated is how many comments one push, all the repositories of
// one sync together, creates on GitHub. What comes after is for the next sync:
// it keeps the first sync after comments is added from flooding the issues.
const maxCommentsCreated = 30

// commentWork is a comment of a task that a push may send, with its gh as it
// is being changed.
type commentWork struct {
	c     *model.Comment
	links []model.CommentLink
	dirty bool
}

// commentPush is what the push of the comments of one task found to write.
type commentPush struct {
	work  []*commentWork // in ULID order
	notes []issueNote
}

func (w *commentWork) at(ref string) int {
	return slices.IndexFunc(w.links, func(l model.CommentLink) bool { return model.CanonRef(l.Ref) == ref })
}

// setLink puts l in the gh of w, in the place of the one with its ref.
func (w *commentWork) setLink(l model.CommentLink) {
	if i := w.at(model.CanonRef(l.Ref)); i >= 0 {
		w.links[i] = l
	} else {
		w.links = append(w.links, l)
	}
	w.dirty = true
}

func (w *commentWork) dropLink(ref string) {
	w.links = slices.DeleteFunc(w.links, func(l model.CommentLink) bool { return model.CanonRef(l.Ref) == ref })
	w.dirty = true
}

// commentsToPush reads the comments of t that this clone sends: those written
// by one of me. The ones taken from GitHub, and the ones with no author, are
// not.
func commentsToPush(s *store.Store, t *model.Task, me []string, rep *GitHubFetchReport) ([]*commentWork, error) {
	var out []*commentWork
	for c, err := range s.Comments(t.ULID) {
		if err != nil {
			return nil, err
		}
		if c.Mirrored() || !slices.Contains(me, c.Author) {
			continue
		}
		links, err := c.GitHub()
		if err != nil {
			rep.Warnings = append(rep.Warnings, t.ULID.String()+": comment "+c.ULID.String()+": "+err.Error())
			continue
		}
		out = append(out, &commentWork{c: c, links: links})
	}
	slices.SortFunc(out, func(a, b *commentWork) int { return strings.Compare(a.c.ULID.String(), b.c.ULID.String()) })
	return out, nil
}

// pushLinkComments makes the comments of the issue of the link l the comments
// of the task: it creates, changes and deletes. A failure is a warning, and
// leaves the difference for the next sync. created counts the comments made in
// this run of the push.
func pushLinkComments(cp *commentPush, l model.Link, number int, cl *github.Client, acct config.Account, created *int, rep *GitHubFetchReport) {
	ref := model.CanonRef(l.Ref)
	parts := strings.SplitN(repoOf(ref), "/", 3)
	owner, name := parts[1], parts[2]
	label := model.GitHubRef{Repo: repoOf(ref), Kind: model.LinkIssue, Number: number}.Label()
	warn := func(err error) {
		rep.Warnings = append(rep.Warnings, github.Describe(err, acct.ID(), acct.Name, "write a comment on "+label).Error())
	}
	if !slices.ContainsFunc(cp.work, func(w *commentWork) bool { return w.c.DeletedAt == nil || w.at(ref) >= 0 }) {
		return // nothing to send, and nothing there that is ours
	}
	list, err := cl.IssueComments(owner, name, number)
	if err != nil {
		rep.Warnings = append(rep.Warnings, github.Describe(err, acct.ID(), acct.Name, "read the comments of "+label).Error())
		return
	}
	byID := map[int64]github.IssueComment{}
	byMark := map[model.ULID]github.IssueComment{}
	for _, x := range list {
		byID[x.ID] = x
		if _, u, ok := model.SplitComment(x.Body); ok {
			if _, seen := byMark[u]; !seen {
				byMark[u] = x
			}
		}
	}
	stopCreating := false
	// create makes the comment w on the issue; the ones after it wait when it
	// cannot, or when the push has made its share.
	create := func(w *commentWork, want string) {
		if stopCreating || *created >= maxCommentsCreated {
			return
		}
		made, err := cl.CreateIssueComment(owner, name, number, model.MarkedComment(w.c.ULID, want))
		if err != nil {
			warn(err)
			stopCreating = true // the ones after it would be out of order
			return
		}
		*created++
		w.setLink(model.CommentLink{Ref: ref, ID: made.ID, BodySynced: want})
	}
	for _, w := range cp.work {
		c := w.c
		at := w.at(ref)
		if c.DeletedAt != nil {
			if at < 0 {
				continue
			}
			if err := cl.DeleteIssueComment(owner, name, w.links[at].ID); err != nil && !errors.Is(err, github.ErrNotFound) {
				warn(err)
				continue
			}
			w.dropLink(ref)
			continue
		}
		want := model.NormalizeBody(c.Body)
		var lk model.CommentLink
		var there github.IssueComment
		var found bool
		if at >= 0 {
			lk = w.links[at]
			there, found = byID[lk.ID]
		}
		if !found {
			// No ID, or GitHub no longer has it: the mark may find it under
			// another ID, which a lost gh left there.
			if there, found = byMark[c.ULID]; found {
				text, _, _ := model.SplitComment(there.Body)
				lk = model.CommentLink{Ref: ref, ID: there.ID, BodySynced: text}
				w.setLink(lk)
			}
		}
		if !found {
			create(w, want)
			continue
		}
		text, mark, marked := model.SplitComment(there.Body)
		if marked && mark == c.ULID && text == want {
			if lk.BodySynced != want {
				lk.BodySynced = want
				w.setLink(lk)
			}
			continue
		}
		// The text on GitHub is not the task's, and is written over. When it is
		// not the one last made the same, it was edited there.
		if _, err := cl.UpdateIssueComment(owner, name, lk.ID, model.MarkedComment(c.ULID, want)); err != nil {
			if errors.Is(err, github.ErrNotFound) {
				create(w, want) // deleted there since the list: the task is the master
			} else {
				warn(err)
			}
			continue
		}
		if text != lk.BodySynced {
			cp.notes = append(cp.notes, issueNote{ref, automation.Overwrite{Field: "comments:" + c.ULID.String(), Side: automation.SideGitHub, Lost: []string{text}}})
		}
		lk.BodySynced = want
		w.setLink(lk)
	}
}
