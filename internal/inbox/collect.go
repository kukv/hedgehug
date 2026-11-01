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
// skipped, as the commits it brings are among the others. Accounts compare
// without case.
func Collect(s *store.Store, before, after plumbing.Hash, me []string) ([]Item, error) {
	r := s.Repo()
	es, err := r.Between(after, before)
	if err != nil {
		return nil, err
	}
	var out []Item
	for _, e := range es {
		tree, err := r.TreeOf(e.Hash)
		if err != nil {
			return nil, err
		}
		c := &commit{r: r, e: e, tree: tree, me: me}
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
		if _, mine := isMe(me, e.Writer()); len(e.Parents) == 1 && !mine {
			if err := c.changes(); err != nil {
				return nil, err
			}
		}
		c.dropCoveredWatches()
		if err := c.ids(); err != nil {
			return nil, err
		}
		out = append(out, c.items...)
	}
	return out, nil
}

// Notice makes the item for a repair the sync tells of without a commit,
// such as a prefix two workspaces share: target is the workspace or task,
// to whom it tells, commit the HEAD it was seen at. ok is false when the
// repair makes no item.
func Notice(target model.ULID, kind, value, to, commit string, when time.Time) (Item, bool) {
	it, ok := repairItem(target, kind, value, to)
	it.Commit, it.Time = commit, when
	return it, ok
}

// commit makes the items of one commit.
type commit struct {
	r     *gitrepo.Repo
	e     gitrepo.LogEntry
	tree  plumbing.Hash
	me    []string
	items []Item
	// changed maps the names of the files the commit changed from any of
	// its parents to their paths; made when a trailer needs it.
	changed map[string]string
}

// add puts in an item of a trailer: who caused it the trailer does not
// tell.
func (c *commit) add(it Item) {
	it.Commit, it.Time = c.e.Hash.String(), c.e.When
	c.items = append(c.items, it)
}

// addChange puts in an item of what the commit changed, which its writer
// caused.
func (c *commit) addChange(it Item) {
	it.Author = c.e.Writer()
	c.add(it)
}

// dropCoveredWatches removes the watch items that an assigned or mention
// item of the same task and account covers: one change tells an account
// once, and an account that was just made a watcher by the change it is told
// of needs no second notice.
func (c *commit) dropCoveredWatches() {
	covered := func(w Item) bool {
		return slices.ContainsFunc(c.items, func(x Item) bool {
			// Account IDs are compared without case (spec: 共通の決まり, 大文字小文字); inbox does not import config, so strings.EqualFold, like config.SameAccount.
			return (x.Kind == Assigned || x.Kind == Mention) && x.Task == w.Task && strings.EqualFold(x.To, w.To)
		})
	}
	c.items = slices.DeleteFunc(c.items, func(it Item) bool { return it.Kind == Watch && covered(it) })
}

// ids gives the items of tasks the task's id at the commit.
func (c *commit) ids() error {
	got := map[model.ULID]string{}
	for i, it := range c.items {
		if it.Task.IsZero() {
			continue
		}
		id, ok := got[it.Task]
		if !ok {
			t, err := readTask(c.r, c.tree, it.Task)
			if err != nil {
				return err
			}
			if t != nil {
				id = t.ID
			}
			got[it.Task] = id
		}
		c.items[i].ID = id
	}
	return nil
}

// splitTrailer splits a trailer value into its first and second words
// (unescaped), the JSON value and the last word. The value can hold spaces;
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
	if second, err = url.PathUnescape(second); err != nil {
		return "", "", "", "", false
	}
	return first, second, rest[:i], rest[i+1:], true
}

// overwritten reads `<path> <field> <JSON value> <author>`. A trailer that
// does not read so is not one Hedgehug wrote and is skipped.
func (c *commit) overwritten(v string) {
	p, field, value, who, ok := splitTrailer(v)
	if _, mine := isMe(c.me, who); !ok || !mine || !json.Valid([]byte(value)) {
		return
	}
	c.add(Item{Kind: Overwritten, Task: taskOf(p), Path: p, To: who, Detail: overwrittenDetail(field, value)})
}

// repaired reads `<ULID or path> <kind> <JSON value or -> <author>`.
func (c *commit) repaired(v string) error {
	target, kind, value, who, ok := splitTrailer(v)
	if _, mine := isMe(c.me, who); !ok || !mine {
		return nil
	}
	u, err := model.ParseULID(target)
	isULID := err == nil
	if kind == "revived" {
		return c.revived(u, isULID, target, value, who)
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

// repairItem makes the item of the repair kind of u, told to to.
func repairItem(u model.ULID, kind, value, to string) (Item, bool) {
	raw := json.RawMessage(value)
	if kind == "id_renumbered" || !json.Valid(raw) { // id_renumbered: the other side of an id_collision, told there
		return Item{}, false
	}
	it := Item{Kind: Repaired, Task: u, To: to}
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
		it.Detail = mustJSON(d)
	}
	return it, true
}

// revived tells of a file a merge brought back. The trailer names it by
// the ULID of its name, or by its path when its name is no ULID.
func (c *commit) revived(u model.ULID, isULID bool, target, value, who string) error {
	p := target
	if isULID {
		var err error
		if p, err = c.find(u); err != nil || p == "" {
			return err
		}
	}
	c.add(Item{Kind: Revived, Task: taskOf(p), Path: p, To: who, Detail: mustJSON(repairDetail{Kind: "revived", Value: jsonOrNull(value)})})
	return nil
}

// find returns the path of the file named by u that the commit changed
// from one of its parents, as a merge that brings a file back does; "" when
// there is none.
func (c *commit) find(u model.ULID) (string, error) {
	if c.changed == nil {
		c.changed = map[string]string{}
		for _, parent := range c.e.Parents {
			from, err := c.r.TreeOf(parent)
			if err != nil {
				return "", err
			}
			cs, err := c.r.DiffTrees(from, c.tree, "")
			if err != nil {
				return "", err
			}
			for _, ch := range cs {
				if !ch.Deleted {
					c.changed[path.Base(ch.Path)] = ch.Path
				}
			}
		}
	}
	return c.changed[u.String()+".md"], nil
}

// changes makes the items of what the commit changed from its parent: the
// tasks it changed and the comments it added or changed. A file that does
// not decode makes none; doctor tells of it.
func (c *commit) changes() error {
	from, err := c.r.TreeOf(c.e.Parents[0])
	if err != nil {
		return err
	}
	cs, err := c.r.DiffTrees(from, c.tree, "")
	if err != nil {
		return err
	}
	for _, ch := range cs {
		if ch.Deleted {
			continue
		}
		var err error
		if u, perr := model.ParseTaskPath(ch.Path); perr == nil {
			err = c.task(u, from)
		} else if task, cm, perr := model.ParseCommentPath(ch.Path); perr == nil {
			err = c.comment(task, cm, from)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *commit) task(u model.ULID, from plumbing.Hash) error {
	now, err := readTask(c.r, c.tree, u)
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
	for _, to := range c.added(old.Assignees, now.Assignees) {
		c.addChange(Item{Kind: Assigned, Task: u, To: to})
	}
	for _, to := range c.mentioned(old.Body, now.Body) {
		c.addChange(Item{Kind: Mention, Task: u, To: to})
	}
	watchers := c.watchers(now)
	if was == nil || len(watchers) == 0 {
		return nil
	}
	var changed []string
	if now.Status != old.Status {
		changed = append(changed, "status")
	}
	if !slices.Equal(now.Assignees, old.Assignees) {
		changed = append(changed, "assignees")
	}
	for _, to := range watchers {
		if len(changed) > 0 {
			c.addChange(Item{Kind: Watch, Task: u, To: to, Detail: mustJSON(map[string][]string{"changed": changed})})
		}
	}
	return nil
}

// comment makes the items of a comment the commit added or changed: a new
// mention of me, and for a new comment a watch.
func (c *commit) comment(task, cm model.ULID, from plumbing.Hash) error {
	was, err := readComment(c.r, from, task, cm)
	if err != nil {
		return err
	}
	now, err := readComment(c.r, c.tree, task, cm)
	if err != nil || now == nil {
		return err
	}
	old := &model.Comment{}
	if was != nil {
		old = was
	}
	d := mustJSON(map[string]model.ULID{"comment": cm})
	for _, to := range c.mentioned(old.Body, now.Body) {
		c.addChange(Item{Kind: Mention, Task: task, To: to, Detail: d})
	}
	if was != nil {
		return nil
	}
	t, err := readTask(c.r, c.tree, task)
	if err != nil || t == nil {
		return err
	}
	for _, to := range c.watchers(t) {
		c.addChange(Item{Kind: Watch, Task: task, To: to, Detail: d})
	}
	return nil
}

// added returns the accounts of me in now and not in old.
func (c *commit) added(old, now []string) []string {
	var out []string
	for _, a := range now {
		_, had := isMe(old, a)
		// Account IDs are compared without case (spec: 共通の決まり, 大文字小文字); inbox does not import config, so strings.EqualFold, like config.SameAccount.
		if to, ok := isMe(c.me, a); ok && !had && !slices.ContainsFunc(out, func(o string) bool { return strings.EqualFold(o, to) }) {
			out = append(out, to)
		}
	}
	return out
}

// mentioned returns the accounts of me that the mentions in now and not in
// old name.
func (c *commit) mentioned(old, now string) []string {
	was := index.Mentions(old)
	var out []string
	for _, m := range index.Mentions(now) {
		// Account IDs are compared without case (spec: 共通の決まり, 大文字小文字); inbox does not import config, so strings.EqualFold, like config.SameAccount.
		if slices.ContainsFunc(was, func(w string) bool { return strings.EqualFold(w, m) }) {
			continue
		}
		for _, a := range c.me {
			if index.MentionNames(m, a) && !slices.Contains(out, a) {
				out = append(out, a)
			}
		}
	}
	return out
}

// watchers returns the accounts of me that watch t.
func (c *commit) watchers(t *model.Task) []string {
	var out []string
	for _, w := range t.Watchers {
		// Account IDs are compared without case (spec: 共通の決まり, 大文字小文字); inbox does not import config, so strings.EqualFold, like config.SameAccount.
		if to, ok := isMe(c.me, w); ok && !slices.ContainsFunc(out, func(o string) bool { return strings.EqualFold(o, to) }) {
			out = append(out, to)
		}
	}
	return out
}

// readTask reads the task u in tree; nil when it is missing or does not
// decode.
func readTask(r *gitrepo.Repo, tree plumbing.Hash, u model.ULID) (*model.Task, error) {
	b, err := readAt(r, tree, model.TaskPath(u))
	if err != nil || b == nil {
		return nil, err
	}
	t, err := model.DecodeTask(b)
	if err != nil {
		return nil, nil //nolint:nilerr // a task that does not decode makes no item
	}
	return t, nil
}

// readComment reads the comment cm of task in tree; nil when it is missing
// or does not decode.
func readComment(r *gitrepo.Repo, tree plumbing.Hash, task, cm model.ULID) (*model.Comment, error) {
	b, err := readAt(r, tree, model.CommentPath(task, cm))
	if err != nil || b == nil {
		return nil, err
	}
	x, err := model.DecodeComment(b)
	if err != nil {
		return nil, nil //nolint:nilerr // a comment that does not decode makes no item
	}
	return x, nil
}

// readAt reads p in tree; nil when it is missing.
func readAt(r *gitrepo.Repo, tree plumbing.Hash, p string) ([]byte, error) {
	b, err := r.ReadFile(tree, p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

// isMe returns the account of me that a is, comparing without case as
// config.SameAccount does (spec: 共通の決まり, 大文字小文字).
func isMe(me []string, a string) (string, bool) {
	for _, m := range me {
		if strings.EqualFold(m, a) {
			return a, true
		}
	}
	return "", false
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
