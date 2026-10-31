package inbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// clone is a clone of a store with the account that writes in it.
type clone struct {
	s    *store.Store
	acct string
}

func (c *clone) op(name string, task model.ULID) store.Op {
	login := c.acct[strings.LastIndex(c.acct, "/")+1:]
	return store.Op{Name: name, Task: task, Author: gitrepo.Signature{Name: login, Email: login + "@x"}, Account: c.acct}
}

// twoClones makes a store written by github.com/a, pushed to a bare remote,
// and a clone of it written by github.com/b.
func twoClones(t *testing.T) (*clone, *clone) {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "r.git")
	if err := gitrepo.InitBare(remote); err != nil {
		t.Fatal(err)
	}
	s, err := store.Create(filepath.Join(t.TempDir(), "a"), &model.StoreMeta{ULID: model.NewULID(), Name: "s"}, nil,
		&model.Workspace{ULID: model.NewULID(), Name: "web", IDPrefix: "WEB"}, gitrepo.Signature{Name: "a", Email: "a@x"}, "github.com/a")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Repo().SetRemotes([]string{remote}); err != nil {
		t.Fatal(err)
	}
	if err := s.Repo().Push("origin"); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "b")
	if _, err := gitrepo.Clone(remote, dir); err != nil {
		t.Fatal(err)
	}
	sb, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := sb.Repo().SetRemotes([]string{remote}); err != nil {
		t.Fatal(err)
	}
	return &clone{s, "github.com/a"}, &clone{sb, "github.com/b"}
}

// syncBoth brings what a wrote to b: a pushes, b fetches and fast-forwards.
func syncBoth(t *testing.T, a, b *clone) {
	t.Helper()
	if err := a.s.Repo().Push("origin"); err != nil {
		t.Fatal(err)
	}
	r := b.s.Repo()
	to, err := r.Fetch("origin")
	if err != nil {
		t.Fatal(err)
	}
	h := head(t, b)
	from, err := r.TreeOf(h)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := r.TreeOf(to)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := r.DiffTrees(from, tree, "")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, c := range cs {
		paths = append(paths, c.Path)
	}
	if err := b.s.FastForward(h, to, paths); err != nil {
		t.Fatal(err)
	}
}

func head(t *testing.T, c *clone) plumbing.Hash {
	t.Helper()
	h, err := c.s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// addTask adds a root task to the first workspace; fn, if not nil, changes
// it first.
func addTask(t *testing.T, c *clone, title string, fn func(*model.Task)) *model.Task {
	t.Helper()
	var ws model.ULID
	for w, err := range c.s.Workspaces() {
		if err != nil {
			t.Fatal(err)
		}
		ws = w.ULID
	}
	r, err := model.Between("", "")
	if err != nil {
		t.Fatal(err)
	}
	x := &model.Task{ULID: model.NewULID(), Workspace: ws, Kind: "task", Title: title, Status: "todo", Rank: r}
	if fn != nil {
		fn(x)
	}
	if _, err := c.s.Update(c.op("add", x.ULID), func(tx *store.Tx) error { return tx.PutTask(x) }); err != nil {
		t.Fatal(err)
	}
	return x
}

func update(t *testing.T, c *clone, ulid model.ULID, fn func(*model.Task)) {
	t.Helper()
	if _, err := c.s.Update(c.op("update", ulid), func(tx *store.Tx) error {
		x, err := tx.Task(ulid)
		if err != nil {
			return err
		}
		fn(x)
		return tx.PutTask(x)
	}); err != nil {
		t.Fatal(err)
	}
}

func addComment(t *testing.T, c *clone, task model.ULID, body string) *model.Comment {
	t.Helper()
	now := time.Now().Truncate(time.Second)
	cm := &model.Comment{ULID: model.NewULID(), Author: c.acct, Created: now, Updated: now, Body: body}
	if _, err := c.s.Update(c.op("comment", task), func(tx *store.Tx) error { return tx.PutComment(task, cm) }); err != nil {
		t.Fatal(err)
	}
	return cm
}

func kinds(items []Item) map[Kind]int {
	out := map[Kind]int{}
	for _, it := range items {
		out[it.Kind]++
	}
	return out
}

func editComment(t *testing.T, c *clone, task model.ULID, cm *model.Comment, body string) {
	t.Helper()
	cm.Body, cm.Updated = body, cm.Updated.Add(time.Second)
	if _, err := c.s.Update(c.op("comment", task), func(tx *store.Tx) error { return tx.PutComment(task, cm) }); err != nil {
		t.Fatal(err)
	}
}

func TestCollectMentionAssignWatch(t *testing.T) {
	a, b := twoClones(t) // accounts github.com/a and github.com/b
	x := addTask(t, a, "x", func(t *model.Task) { t.Watchers = []string{"github.com/b"} })
	syncBoth(t, a, b)
	before := head(t, b)
	update(t, a, x.ULID, func(t *model.Task) {
		t.Body = "ping @b and `@b` in code\n"
		t.Assignees = []string{"github.com/b"}
		t.Status = "in_progress"
	})
	syncBoth(t, a, b)
	items, err := Collect(b.s, before, head(t, b), []string{"github.com/b"})
	if err != nil {
		t.Fatal(err)
	}
	if k := kinds(items); k[Mention] != 1 || k[Assigned] != 1 || k[Watch] != 1 || len(items) != 3 {
		t.Errorf("kinds = %v", k) // the mention in inline code does not count
	}
	for _, it := range items {
		if it.Task != x.ULID || it.ID != x.ID || it.ID == "" || it.Author != "github.com/a" || it.To != "github.com/b" || it.Commit != head(t, b).String() {
			t.Errorf("item = %+v", it)
		}
		if it.Kind == Watch && string(it.Detail) != `{"changed":["status","assignees"]}` {
			t.Errorf("watch detail = %s", it.Detail)
		}
	}
}

func TestCollectAccountsWithoutCase(t *testing.T) {
	a, b := twoClones(t)
	before := head(t, b)
	addTask(t, a, "x", func(t *model.Task) {
		t.Body = "for @B\n"
		t.Assignees = []string{"GitHub.com/B"}
	})
	syncBoth(t, a, b)
	items, err := Collect(b.s, before, head(t, b), []string{"github.com/b"})
	if k := kinds(items); err != nil || k[Mention] != 1 || k[Assigned] != 1 || len(items) != 2 {
		t.Errorf("kinds = %v, %v", k, err)
	}
}

func TestCollectOnlyNewMentions(t *testing.T) {
	a, b := twoClones(t)
	x := addTask(t, a, "x", func(t *model.Task) { t.Body = "for @b\n" })
	syncBoth(t, a, b)
	before := head(t, b)
	update(t, a, x.ULID, func(t *model.Task) { t.Body = "for @b, and @github.com/c\n" })
	syncBoth(t, a, b)
	items, err := Collect(b.s, before, head(t, b), []string{"github.com/b"})
	if err != nil || len(items) != 0 {
		t.Errorf("a mention already there made %v, %v", items, err)
	}
}

func TestCollectComment(t *testing.T) {
	a, b := twoClones(t)
	x := addTask(t, a, "x", func(t *model.Task) { t.Watchers = []string{"github.com/b"} })
	y := addTask(t, a, "y", nil)
	before := head(t, b)
	addComment(t, a, x.ULID, "looks fine\n")
	addComment(t, a, y.ULID, "@gitlab.com/b is not github.com/b; @github.com/b is\n")
	syncBoth(t, a, b)
	items, err := Collect(b.s, before, head(t, b), []string{"github.com/b"})
	if err != nil {
		t.Fatal(err)
	}
	if k := kinds(items); k[Watch] != 1 || k[Mention] != 1 || len(items) != 2 {
		t.Fatalf("kinds = %v", k)
	}
	for _, it := range items {
		if it.Kind == Watch && it.Task != x.ULID || it.Kind == Mention && it.Task != y.ULID || !strings.Contains(string(it.Detail), `"comment":`) {
			t.Errorf("item = %+v", it)
		}
	}
}

func TestCollectEditedComment(t *testing.T) {
	a, b := twoClones(t)
	x := addTask(t, a, "x", func(t *model.Task) { t.Watchers = []string{"github.com/b"} })
	cm := addComment(t, a, x.ULID, "first\n")
	syncBoth(t, a, b)
	before := head(t, b)
	editComment(t, a, x.ULID, cm, "first, and @b\n")
	syncBoth(t, a, b)
	items, err := Collect(b.s, before, head(t, b), []string{"github.com/b"})
	if err != nil || len(items) != 1 || items[0].Kind != Mention || items[0].Detail == nil {
		t.Errorf("items = %+v, %v", items, err) // a mention, and no watch: the comment is not new
	}
	before = head(t, b)
	editComment(t, a, x.ULID, cm, "first, and @b, again\n")
	syncBoth(t, a, b)
	if items, err := Collect(b.s, before, head(t, b), []string{"github.com/b"}); err != nil || len(items) != 0 {
		t.Errorf("a mention already there made %+v, %v", items, err)
	}
}

func TestCollectSkipsOwnCommits(t *testing.T) {
	a, _ := twoClones(t)
	before := head(t, a)
	addTask(t, a, "x", func(t *model.Task) { t.Assignees = []string{"github.com/a"} })
	if items, _ := Collect(a.s, before, head(t, a), []string{"github.com/a"}); len(items) != 0 {
		t.Errorf("own write made %v", items)
	}
}

func TestCollectOverwrittenAndRenumberedFromOwnMerge(t *testing.T) {
	_, b := twoClones(t)
	x := addTask(t, b, "x", nil)
	before := head(t, b)
	p := model.TaskPath(x.ULID)
	trailers := []gitrepo.Trailer{
		{Key: "Hedgehug-Overwritten", Value: p + ` title "mine: with spaces" github.com/b`}, // the JSON has a space and ": "
		{Key: "Hedgehug-Repaired", Value: x.ULID.String() + ` id_collision {"old":"WEB-1","new":"WEB-2"} github.com/b`},
		{Key: "Hedgehug-Overwritten", Value: p + ` status "todo" github.com/a`}, // someone else's: no item
	}
	h := head(t, b)
	if _, err := b.s.CommitMerge(h, []plumbing.Hash{h}, map[string][]byte{"notes.md": []byte("n\n")},
		store.Op{Name: "merge", Detail: "origin", Author: gitrepo.Signature{Name: "b", Email: "b@x"}, Account: "github.com/b"}, trailers); err != nil {
		t.Fatal(err)
	}
	items, err := Collect(b.s, before, head(t, b), []string{"github.com/b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Kind != Overwritten || !strings.Contains(string(items[0].Detail), `mine: with spaces`) || items[1].Kind != Renumbered || string(items[1].Detail) != `{"old":"WEB-1","new":"WEB-2"}` {
		t.Fatalf("items = %+v", items)
	}
	for _, it := range items {
		if it.Task != x.ULID || it.ID != x.ID || it.To != "github.com/b" || it.Author != "github.com/b" {
			t.Errorf("item = %+v", it)
		}
	}
	if items[0].Path != p {
		t.Errorf("path = %q", items[0].Path)
	}
}

func TestCollectRepairedTrailers(t *testing.T) {
	_, b := twoClones(t)
	x := addTask(t, b, "x", nil)
	cm := addComment(t, b, x.ULID, "c\n")
	var ws model.ULID
	for w, err := range b.s.Workspaces() {
		if err != nil {
			t.Fatal(err)
		}
		ws = w.ULID
	}
	// x and its comment are removed, then a merge brings them back.
	h := head(t, b)
	files := map[string][]byte{}
	for _, p := range []string{model.TaskPath(x.ULID), model.CommentPath(x.ULID, cm.ULID)} {
		data, err := b.s.Repo().ReadAt(h, p)
		if err != nil {
			t.Fatal(err)
		}
		files[p] = data
	}
	if _, err := b.s.CommitMerge(h, []plumbing.Hash{h}, map[string][]byte{model.TaskPath(x.ULID): nil, model.CommentPath(x.ULID, cm.ULID): nil}, b.op("rm", x.ULID), nil); err != nil {
		t.Fatal(err)
	}
	before := head(t, b)
	other := model.NewULID()
	trailers := []gitrepo.Trailer{
		{Key: "Hedgehug-Repaired", Value: x.ULID.String() + ` parent_cycle "` + other.String() + `" github.com/b`},
		{Key: "Hedgehug-Repaired", Value: x.ULID.String() + ` workspace_mismatch "` + other.String() + `" github.com/b`},
		{Key: "Hedgehug-Repaired", Value: ws.String() + ` prefix_duplicate "WEB" github.com/b`},
		{Key: "Hedgehug-Repaired", Value: ws.String() + ` id_prefix "web" github.com/b`},
		{Key: "Hedgehug-Repaired", Value: x.ULID.String() + ` id_renumbered {"old":"WEB-1","new":"WEB-2"} -`},
		{Key: "Hedgehug-Repaired", Value: x.ULID.String() + ` revived - github.com/b`},
		{Key: "Hedgehug-Repaired", Value: cm.ULID.String() + ` revived - github.com/b`},
		{Key: "Hedgehug-Repaired", Value: url.PathEscape("templates/bug report.md") + ` revived - github.com/b`},
		{Key: "Hedgehug-Repaired", Value: x.ULID.String() + ` revived - github.com/a`},
	}
	h = head(t, b)
	if _, err := b.s.CommitMerge(h, []plumbing.Hash{h}, files, b.op("repair", model.ULID{}), trailers); err != nil {
		t.Fatal(err)
	}
	items, err := Collect(b.s, before, head(t, b), []string{"github.com/b"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, it := range items {
		got = append(got, fmt.Sprintf("%s %s %s %s %s", it.Kind, it.Task, it.ID, it.Path, string(it.Detail)))
	}
	zero := model.ULID{}.String()
	want := []string{
		fmt.Sprintf(`repaired %s %s  {"kind":"parent_cycle","value":"%s"}`, x.ULID, x.ID, other),
		fmt.Sprintf(`repaired %s %s  {"kind":"workspace_mismatch","value":"%s"}`, x.ULID, x.ID, other),
		fmt.Sprintf(`prefix_duplicate %s   {"kind":"prefix_duplicate","workspace":"%s","value":"WEB"}`, zero, ws),
		fmt.Sprintf(`repaired %s   {"kind":"id_prefix","workspace":"%s","value":"web"}`, zero, ws),
		fmt.Sprintf(`revived %s %s %s {"kind":"revived","value":null}`, x.ULID, x.ID, model.TaskPath(x.ULID)),
		fmt.Sprintf(`revived %s %s %s {"kind":"revived","value":null}`, x.ULID, x.ID, model.CommentPath(x.ULID, cm.ULID)),
		fmt.Sprintf(`revived %s  templates/bug report.md {"kind":"revived","value":null}`, zero),
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("items:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Two clones that merge the same pair make two merge commits with the same
// trailers; a clone that gets both is told once.
func TestSameTrailersInTwoMergeCommits(t *testing.T) {
	_, b := twoClones(t)
	x := addTask(t, b, "x", nil)
	before := head(t, b)
	p := url.PathEscape(model.TaskPath(x.ULID))
	trailers := []gitrepo.Trailer{
		{Key: "Hedgehug-Overwritten", Value: p + ` title "mine" github.com/b`},
		{Key: "Hedgehug-Repaired", Value: x.ULID.String() + ` id_collision {"old":"WEB-1","new":"WEB-2"} github.com/b`},
		{Key: "Hedgehug-Repaired", Value: x.ULID.String() + ` parent_cycle null github.com/b`},
		{Key: "Hedgehug-Repaired", Value: url.PathEscape("templates/bug.md") + ` revived - github.com/b`},
	}
	for i := range 2 {
		h := head(t, b)
		if _, err := b.s.CommitMerge(h, []plumbing.Hash{h}, map[string][]byte{"notes.md": []byte(fmt.Sprint(i))}, b.op("merge", model.ULID{}), trailers); err != nil {
			t.Fatal(err)
		}
	}
	items, err := Collect(b.s, before, head(t, b), []string{"github.com/b"})
	if err != nil || len(items) != 8 {
		t.Fatalf("items = %+v, %v", items, err)
	}
	added, err := Open(b.s).Add(items)
	if k := kinds(added); err != nil || len(added) != 4 || k[Overwritten] != 1 || k[Renumbered] != 1 || k[Repaired] != 1 || k[Revived] != 1 {
		t.Errorf("added = %v, %v", k, err)
	}
}

func TestCatch(t *testing.T) {
	_, b := twoClones(t)
	x := addTask(t, b, "x", nil)
	box := Open(b.s)
	if added, err := box.Catch([]string{"github.com/b"}, head(t, b)); err != nil || len(added) != 0 {
		t.Fatalf("first Catch = %+v, %v", added, err)
	}
	// A merge made outside a sync, as by hand.
	h := head(t, b)
	trailers := []gitrepo.Trailer{{Key: "Hedgehug-Overwritten", Value: url.PathEscape(model.TaskPath(x.ULID)) + ` title "mine" github.com/b`}}
	if _, err := b.s.CommitMerge(h, []plumbing.Hash{h}, map[string][]byte{"notes.md": []byte("n\n")}, b.op("merge", model.ULID{}), trailers); err != nil {
		t.Fatal(err)
	}
	rec := `{"kind":"revived","path":"templates/bug.md","author":"github.com/b","commit":"","time":"2026-10-31T10:00:00Z"}`
	if err := b.s.WriteLocal("overwritten/"+model.NewULID().String()+".json", []byte(rec)); err != nil {
		t.Fatal(err)
	}
	// since counts only when the inbox has caught nothing yet.
	added, err := box.Catch([]string{"github.com/b"}, plumbing.ZeroHash)
	if k := kinds(added); err != nil || len(added) != 2 || k[Overwritten] != 1 || k[Revived] != 1 {
		t.Errorf("Catch = %+v, %v", added, err)
	}
	if added, err := box.Catch([]string{"github.com/b"}, plumbing.ZeroHash); err != nil || len(added) != 0 {
		t.Errorf("Catch again = %+v, %v", added, err)
	}
}

func TestImportMergeDriverRecords(t *testing.T) {
	_, b := twoClones(t)
	rec := `{"path":"tasks/0A/P0/01JBZ8K3QW7M2X9V4T6R5N8P0A.md","field":"title","value":"\"mine\"","author":"github.com/b","commit":"","time":"2026-10-31T10:00:00Z"}`
	name := "overwritten/" + model.NewULID().String() + ".json"
	if err := b.s.WriteLocal(name, []byte(rec)); err != nil {
		t.Fatal(err)
	}
	theirs := "overwritten/" + model.NewULID().String() + ".json"
	if err := b.s.WriteLocal(theirs, []byte(strings.Replace(rec, "github.com/b", "github.com/a", 1))); err != nil {
		t.Fatal(err)
	}
	box := Open(b.s)
	if err := box.Import([]string{"github.com/b"}); err != nil {
		t.Fatal(err)
	}
	items, _ := box.List(false)
	if len(items) != 1 || items[0].Kind != Overwritten {
		t.Fatalf("items = %+v", items)
	}
	if it := items[0]; it.Path != "tasks/0A/P0/01JBZ8K3QW7M2X9V4T6R5N8P0A.md" || it.To != "github.com/b" || it.Author != "" || string(it.Detail) != `{"field":"title","value":"mine"}` ||
		!it.Time.Equal(time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("item = %+v", it)
	}
	for _, n := range []string{name, theirs} {
		if _, err := b.s.ReadLocal(n); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("the record %s is left: %v", n, err)
		}
	}
	// The same value lost again, as a trailer tells it, is not a new item.
	if added, err := box.Add([]Item{{Kind: Overwritten, Path: "tasks/0A/P0/01JBZ8K3QW7M2X9V4T6R5N8P0A.md", To: "github.com/b", Commit: "c", Detail: json.RawMessage(`{"field":"title","value":"mine"}`)}}); err != nil || len(added) != 0 {
		t.Errorf("added = %v, %v", added, err)
	}
}

func TestImportLeavesBadRecords(t *testing.T) {
	_, b := twoClones(t)
	x := addTask(t, b, "x", nil)
	bad := map[string]string{
		"overwritten/1.json": "{",
		"overwritten/2.json": `{"kind":"mention","path":"tasks/a.md","author":"github.com/b"}`,
		"overwritten/3.json": `{"field":"title","author":"github.com/b"}`,
	}
	for n, data := range bad {
		if err := b.s.WriteLocal(n, []byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	good := `{"kind":"revived","path":"` + model.TaskPath(x.ULID) + `","author":"github.com/b","commit":"","time":"2026-10-31T10:00:00Z"}`
	if err := b.s.WriteLocal("overwritten/4.json", []byte(good)); err != nil {
		t.Fatal(err)
	}
	box := Open(b.s)
	if err := box.Import([]string{"github.com/b"}); err != nil {
		t.Fatal(err)
	}
	items, _ := box.List(false)
	if len(items) != 1 || items[0].Kind != Revived || items[0].Task != x.ULID || items[0].ID != x.ID || string(items[0].Detail) != `{"kind":"revived","value":null}` {
		t.Errorf("items = %+v", items)
	}
	if left, _ := b.s.LocalFiles("overwritten"); !slices.Equal(left, []string{"1.json", "2.json", "3.json"}) {
		t.Errorf("left = %v", left)
	}
	if w := box.Warnings(); len(w) != 3 || !strings.Contains(w[0], "overwritten/1.json") || !strings.Contains(w[1], `unknown kind "mention"`) || !strings.Contains(w[2], "no path") {
		t.Errorf("warnings = %q", w)
	}
}
