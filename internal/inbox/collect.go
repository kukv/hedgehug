package inbox

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// Collect makes the items for me from the commits between before and after,
// in the order of the history. The Hedgehug-Overwritten and
// Hedgehug-Repaired trailers make an item when the account they name is one
// of me, whoever made the commit. The changes of a commit make mention,
// assigned and watch items unless one of me wrote it; a merge commit's are
// skipped, as the commits it brings are among the others.
func Collect(s *store.Store, before, after plumbing.Hash, me []string) ([]Item, error) {
	r := s.Repo()
	es, err := r.Between(after, before)
	if err != nil {
		return nil, err
	}
	var out []Item
	for _, e := range es {
		c := &commit{r: r, e: e, me: me}
		for _, t := range e.Trailers {
			var err error
			switch t.Key {
			case "Hedgehug-Overwritten":
				c.overwritten(t.Value)
			case "Hedgehug-Repaired":
				err = c.repaired(t.Value)
			}
			if err != nil {
				return nil, err
			}
		}
		if len(e.Parents) == 1 && !slices.Contains(me, author(e)) {
			if err := c.changes(); err != nil {
				return nil, err
			}
		}
		out = append(out, c.items...)
	}
	return out, nil
}

// Notice makes the item for a repair the sync tells of without a commit,
// such as a prefix two workspaces share: target is the workspace or task,
// commit the HEAD it was seen at. ok is false when the repair makes no item.
func Notice(target model.ULID, kind, value, author, commit string, when time.Time) (Item, bool) {
	it, ok := repairItem(target, kind, value, author)
	it.Commit, it.Time = commit, when
	return it, ok
}

// commit makes the items of one commit.
type commit struct {
	r     *gitrepo.Repo
	e     gitrepo.LogEntry
	me    []string
	items []Item
}

func (c *commit) add(it Item) {
	it.Commit, it.Time = c.e.Hash.String(), c.e.When
	if it.Author == "" {
		it.Author = author(c.e)
	}
	c.items = append(c.items, it)
}

// splitTrailer splits a trailer value into its first word (unescaped), its
// second word, the JSON value and the last word. The value can hold spaces;
// the words cannot.
func splitTrailer(v string) (first, second, value, last string, ok bool) {
	first, rest, ok1 := strings.Cut(v, " ")
	second, rest, ok2 := strings.Cut(rest, " ")
	i := strings.LastIndex(rest, " ")
	if !ok1 || !ok2 || i < 0 {
		return "", "", "", "", false
	}
	first, err := url.PathUnescape(first)
	if err != nil {
		return "", "", "", "", false
	}
	return first, second, rest[:i], rest[i+1:], true
}

// overwritten reads `<path> <field> <JSON value> <author>`. A trailer that
// does not read so is not one Hedgehug wrote and is skipped.
func (c *commit) overwritten(v string) {
	p, field, value, who, ok := splitTrailer(v)
	if !ok || !json.Valid([]byte(value)) || !slices.Contains(c.me, who) {
		return
	}
	c.add(Item{Kind: Overwritten, Task: taskOf(p), Path: p, Author: who, Detail: overwrittenDetail(field, value)})
}

// repaired reads `<ULID or path> <kind> <JSON value or -> <author>`.
func (c *commit) repaired(v string) error {
	target, kind, value, who, ok := splitTrailer(v)
	if !ok || !slices.Contains(c.me, who) {
		return nil
	}
	u, err := model.ParseULID(target)
	isULID := err == nil
	if kind == "revived" {
		return c.revived(u, isULID, target, who)
	}
	if !isULID {
		return nil
	}
	if it, ok := repairItem(u, kind, value, who); ok {
		c.add(it)
	}
	return nil
}

// workspaceKinds are the repairs and notices whose ULID is a workspace's.
var workspaceKinds = []string{"workspace_revived", "id_prefix", "prefix_duplicate", "name_duplicate"}

// repairItem makes the item of the repair kind of u, told to who.
func repairItem(u model.ULID, kind, value, who string) (Item, bool) {
	raw := json.RawMessage(value)
	if kind == "id_renumbered" || !json.Valid(raw) { // id_renumbered: the other side of an id_collision, told there
		return Item{}, false
	}
	it := Item{Kind: Repaired, Task: u, Author: who}
	d := repairDetail{Kind: kind, Value: raw}
	if slices.Contains(workspaceKinds, kind) {
		it.Task, d.Workspace = model.ULID{}, u
	}
	switch kind {
	case "id_collision":
		it.Kind, it.Detail = Renumbered, raw
	case "prefix_duplicate", "name_duplicate":
		it.Kind = PrefixDuplicate
	}
	if it.Detail == nil {
		it.Detail, _ = json.Marshal(d) // valid JSON always encodes
	}
	return it, true
}

// revived tells of a file a merge brought back. The trailer names it by
// the ULID of its name, or by its path when its name is no ULID.
func (c *commit) revived(u model.ULID, isULID bool, target, who string) error {
	p := target
	if isULID {
		var err error
		if p, err = c.find(u); err != nil || p == "" {
			return err
		}
	}
	c.add(Item{Kind: Revived, Task: taskOf(p), Path: p, Author: who})
	return nil
}

// find returns the path of the file named by u in the commit, or "" when
// there is none.
func (c *commit) find(u model.ULID) (string, error) {
	if _, err := c.r.ReadAt(c.e.Hash, model.TaskPath(u)); err == nil {
		return model.TaskPath(u), nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	tree, err := c.r.TreeOf(c.e.Hash)
	if err != nil {
		return "", err
	}
	for p, err := range c.r.Files(tree, "") {
		if err != nil {
			return "", err
		}
		if path.Base(p) == u.String()+".md" {
			return p, nil
		}
	}
	return "", nil
}

// changes makes the items of what the commit changed from its parent: the
// tasks it changed and the comments it added. A file that does not decode
// makes none; doctor tells of it.
func (c *commit) changes() error {
	from, err := c.r.TreeOf(c.e.Parents[0])
	if err != nil {
		return err
	}
	to, err := c.r.TreeOf(c.e.Hash)
	if err != nil {
		return err
	}
	cs, err := c.r.DiffTrees(from, to, "")
	if err != nil {
		return err
	}
	for _, ch := range cs {
		if ch.Deleted {
			continue
		}
		var err error
		if u, perr := model.ParseTaskPath(ch.Path); perr == nil {
			err = c.task(u, from, to)
		} else if task, cm, perr := model.ParseCommentPath(ch.Path); perr == nil {
			err = c.comment(task, cm, from, to)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *commit) task(u model.ULID, from, to plumbing.Hash) error {
	now, err := readTask(c.r, to, u)
	if err != nil || now == nil {
		return err
	}
	was, err := readTask(c.r, from, u)
	if err != nil {
		return err
	}
	old := &model.Task{}
	if was != nil {
		old = was
	}
	if slices.ContainsFunc(now.Assignees, func(a string) bool { return slices.Contains(c.me, a) && !slices.Contains(old.Assignees, a) }) {
		c.add(Item{Kind: Assigned, Task: u})
	}
	oldMentions := index.Mentions(old.Body)
	if slices.ContainsFunc(index.Mentions(now.Body), func(m string) bool { return !slices.Contains(oldMentions, m) && mentions(m, c.me) }) {
		c.add(Item{Kind: Mention, Task: u})
	}
	if was == nil || !slices.ContainsFunc(now.Watchers, func(w string) bool { return slices.Contains(c.me, w) }) {
		return nil
	}
	var changed []string
	if now.Status != old.Status {
		changed = append(changed, "status")
	}
	if !slices.Equal(now.Assignees, old.Assignees) {
		changed = append(changed, "assignees")
	}
	if len(changed) > 0 {
		d, _ := json.Marshal(map[string][]string{"changed": changed}) // strings always encode
		c.add(Item{Kind: Watch, Task: u, Detail: d})
	}
	return nil
}

// comment makes the items of a comment the commit added.
func (c *commit) comment(task, cm model.ULID, from, to plumbing.Hash) error {
	p := model.CommentPath(task, cm)
	if _, err := c.r.ReadFile(from, p); err == nil {
		return nil // changed, not added
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	b, err := c.r.ReadFile(to, p)
	if err != nil {
		return err
	}
	x, err := model.DecodeComment(b)
	if err != nil {
		return nil //nolint:nilerr // a comment that does not decode makes no item
	}
	d, _ := json.Marshal(map[string]model.ULID{"comment": cm}) // a ULID always encodes
	if slices.ContainsFunc(index.Mentions(x.Body), func(m string) bool { return mentions(m, c.me) }) {
		c.add(Item{Kind: Mention, Task: task, Detail: d})
	}
	t, err := readTask(c.r, to, task)
	if err != nil {
		return err
	}
	if t != nil && slices.ContainsFunc(t.Watchers, func(w string) bool { return slices.Contains(c.me, w) }) {
		c.add(Item{Kind: Watch, Task: task, Detail: d})
	}
	return nil
}

// readTask reads the task u in tree; nil when it is missing or does not
// decode.
func readTask(r *gitrepo.Repo, tree plumbing.Hash, u model.ULID) (*model.Task, error) {
	b, err := r.ReadFile(tree, model.TaskPath(u))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t, err := model.DecodeTask(b)
	if err != nil {
		return nil, nil //nolint:nilerr // a task that does not decode makes no item
	}
	return t, nil
}

// mentions reports whether the mention m, as written, is one of me: @login
// is any account with that login, @host/login that account.
func mentions(m string, me []string) bool {
	if strings.Contains(m, "/") {
		return slices.Contains(me, m)
	}
	return slices.ContainsFunc(me, func(a string) bool { return strings.HasSuffix(a, "/"+m) })
}

// taskOf is the task a file of the store belongs to: the task of a task,
// comment or worklog file; zero for any other.
func taskOf(p string) model.ULID {
	if u, err := model.ParseTaskPath(p); err == nil {
		return u
	}
	if u, _, err := model.ParseCommentPath(p); err == nil {
		return u
	}
	if u, _, err := model.ParseWorklogPath(p); err == nil {
		return u
	}
	return model.ULID{}
}

// author is who wrote a commit: its Hedgehug-Author, or the mail of its
// author when it has none.
func author(e gitrepo.LogEntry) string {
	if a, ok := e.Trailer("Hedgehug-Author"); ok {
		return a
	}
	return e.Author.Email
}
