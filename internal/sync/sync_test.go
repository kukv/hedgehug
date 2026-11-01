package sync

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/merge"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

func TestFastForward(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	c.sync(0)
	c.sync(1)
	if got := c.task(1, x.ULID); got.Title != "x" {
		t.Errorf("clone 1 has %v", got)
	}
}

func TestMergeBothSidesAndConverge(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	c.sync(0)
	c.sync(1)
	c.set(0, x.ULID, func(t *model.Task) { t.Title = "from 0" })
	c.set(1, x.ULID, func(t *model.Task) { t.Status = "in_progress" })
	c.sync(0)
	r := c.sync(1)
	c.sync(0)
	if r.Merges != 1 {
		t.Errorf("merges = %d", r.Merges)
	}
	got := c.task(0, x.ULID)
	if got.Title != "from 0" || got.Status != "in_progress" {
		t.Errorf("merged = %s %s", got.Title, got.Status)
	}
	if c.headTree(0) != c.headTree(1) {
		t.Error("clones differ")
	}
}

func TestOverwrittenTrailer(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	c.sync(0)
	c.sync(1)
	c.set(0, x.ULID, func(t *model.Task) { t.Title = "a" })
	c.set(1, x.ULID, func(t *model.Task) { t.Title = "b" }) // later: wins
	c.sync(0)
	r := c.sync(1)
	if len(r.Overwritten) != 1 || r.Overwritten[0].Author != "github.com/u0" || r.Overwritten[0].Value != `"a"` {
		t.Errorf("overwritten = %+v", r.Overwritten)
	}
	e, _ := c.stores[1].Repo().LogFrom(mustHead(t, c.stores[1]), 1)
	if v, ok := e[0].Trailer("Hedgehug-Overwritten"); !ok || !strings.HasPrefix(v, url.PathEscape(model.TaskPath(x.ULID))+" title \"a\" github.com/u0") {
		t.Errorf("trailer = %q", v)
	}
	// The merge commit is u1's, the lost value u0's: only u0 is told, at
	// u0's next sync.
	if items, _ := inbox.Open(c.stores[1]).List(true); len(items) != 0 {
		t.Errorf("u1's inbox = %+v", items)
	}
	r = c.sync(0)
	items, err := inbox.Open(c.stores[0]).List(false)
	if err != nil || len(items) != 1 || items[0].Kind != inbox.Overwritten || items[0].Task != x.ULID || items[0].To != "github.com/u0" || items[0].Author != "" {
		t.Fatalf("u0's inbox = %+v, %v", items, err)
	}
	if len(r.Notices) != 1 || r.Notices[0].ULID != items[0].ULID {
		t.Errorf("notices = %+v", r.Notices)
	}
	if r = c.sync(0); len(r.Notices) != 0 {
		t.Errorf("the next sync told again: %+v", r.Notices)
	}
}

// A sync that fails after HEAD moved still fills the inbox.
func TestInboxAfterFailedPush(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	c.sync(0)
	c.sync(1)
	c.set(0, x.ULID, func(t *model.Task) { t.Title = "a" })
	c.set(1, x.ULID, func(t *model.Task) { t.Title = "b" }) // later: wins
	c.sync(1)
	// u0's merge, where u0's value loses, is made; then its push fails.
	c.opts[0].BeforePush = func() {
		if err := os.Rename(c.remote, c.remote+".away"); err != nil {
			t.Fatal(err)
		}
	}
	r, err := Run(c.stores[0], c.opts[0])
	if err := os.Rename(c.remote+".away", c.remote); err != nil {
		t.Fatal(err)
	}
	if err == nil {
		t.Fatal("the push did not fail")
	}
	items, _ := inbox.Open(c.stores[0]).List(false)
	if len(items) != 1 || items[0].Kind != inbox.Overwritten || len(r.Notices) != 1 {
		t.Errorf("inbox = %+v, notices = %+v", items, r.Notices)
	}
	c.opts[0].BeforePush = nil
	if r := c.sync(0); len(r.Notices) != 0 {
		t.Errorf("told again: %+v", r.Notices)
	}
}

// A Catch between the merge and the end of the sync, on a store the inbox
// has not caught yet, starts from where the sync did.
func TestCatchDuringSync(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	c.sync(0)
	c.sync(1)
	c.set(0, x.ULID, func(t *model.Task) { t.Title = "a" })
	c.set(1, x.ULID, func(t *model.Task) { t.Title = "b" }) // later: wins
	c.sync(1)
	if err := c.stores[0].RemoveLocal("inbox-head"); err != nil {
		t.Fatal(err)
	}
	c.opts[0].BeforePush = func() { // after u0's merge
		h := mustHead(t, c.stores[0])
		if _, err := inbox.Open(c.stores[0]).Catch(c.opts[0].Me, h); err != nil {
			t.Fatal(err)
		}
	}
	c.sync(0)
	if items, _ := inbox.Open(c.stores[0]).List(false); len(items) != 1 || items[0].Kind != inbox.Overwritten {
		t.Errorf("inbox = %+v", items)
	}
}

func TestSyncImportsMergeDriverRecords(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	rec := `{"path":"tasks/0A/P0/01JBZ8K3QW7M2X9V4T6R5N8P0A.md","field":"title","value":"\"mine\"","author":"github.com/u0","commit":"","time":"2026-10-31T10:00:00Z"}`
	if err := c.stores[0].WriteLocal("overwritten/"+model.NewULID().String()+".json", []byte(rec)); err != nil {
		t.Fatal(err)
	}
	c.sync(0)
	if items, _ := inbox.Open(c.stores[0]).List(false); len(items) != 1 || items[0].Kind != inbox.Overwritten {
		t.Errorf("inbox = %+v", items)
	}
}

func TestIDCollisionRenumbersUnpushedSide(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	a := c.addTask(0, "a") // WEB-1 in both clones
	b := c.addTask(1, "b")
	if a.ID != "WEB-1" || b.ID != "WEB-1" {
		t.Fatalf("ids %s %s", a.ID, b.ID)
	}
	c.sync(0) // a reaches origin first
	r := c.sync(1)
	if got := c.task(1, b.ULID); got.ID != "WEB-2" || !slices.Contains(got.FormerIDs, "WEB-1") {
		t.Errorf("b = %s %v", got.ID, got.FormerIDs)
	}
	if got := c.task(1, a.ULID); got.ID != "WEB-1" {
		t.Errorf("a = %s", got.ID)
	}
	if len(r.Repaired) != 1 || r.Repaired[0].Kind != "id_collision" || r.Repaired[0].Author != "github.com/u1" {
		t.Errorf("repaired = %+v", r.Repaired)
	}
	e, _ := c.stores[1].Repo().LogFrom(mustHead(t, c.stores[1]), 1)
	want := b.ULID.String() + ` id_collision {"old":"WEB-1","new":"WEB-2"} github.com/u1`
	if v, _ := e[0].Trailer("Hedgehug-Repaired"); v != want {
		t.Errorf("trailer = %q, want %q", v, want)
	}
	c.sync(0)
	if c.headTree(0) != c.headTree(1) {
		t.Error("clones differ")
	}
}

func TestPushRejectedRetries(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 3)
	c.addTask(0, "a")
	c.addTask(2, "c")
	pushed := false
	c.opts[0].BeforePush = func() {
		if !pushed {
			pushed = true
			c.sync(2) // another clone pushes between our merge and our push
		}
	}
	r := c.sync(0)
	if !slices.Contains(r.Pushed, "origin") {
		t.Errorf("pushed = %v", r.Pushed)
	}
	// Clone 0 was ahead of origin: only the rejected push makes it merge.
	if !pushed || r.Merges != 1 {
		t.Errorf("rejected %v, merges = %d", pushed, r.Merges)
	}
	c.sync(1)
	c.sync(2)
	if c.headTree(0) != c.headTree(1) || c.headTree(1) != c.headTree(2) {
		t.Error("clones differ")
	}
}

func TestLocalWriteDuringPushIsKept(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	c.addTask(0, "a")
	var during *model.Task
	c.opts[0].BeforePush = func() { during = c.addTask(0, "during") } // the store lock is free here
	c.sync(0)
	c.opts[0].BeforePush = nil
	c.sync(0)
	if got := c.task(0, during.ULID); got == nil {
		t.Error("the write during the push was lost")
	}
}

func TestDirty(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	c.sync(0)
	c.sync(1)
	c.set(0, x.ULID, func(t *model.Task) { t.Title = "a" })
	c.sync(0)
	p := filepath.Join(c.stores[1].Dir(), filepath.FromSlash(model.TaskPath(x.ULID)))
	if err := os.WriteFile(p, []byte("hand edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := c.headTree(1)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrDirty) {
		t.Errorf("Run = %v", err)
	}
	if c.headTree(1) != before {
		t.Error("a dirty sync changed HEAD")
	}
}

// A file only the remote changed is written by the merge too, so a hand
// edit of it stops the sync.
func TestDirtyTheirsOnlyPath(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	y := c.addTask(0, "y")
	c.sync(0)
	c.sync(1)
	c.set(0, x.ULID, func(t *model.Task) { t.Title = "a" })
	c.set(1, y.ULID, func(t *model.Task) { t.Title = "b" }) // a merge, not a fast-forward
	c.sync(0)
	p := filepath.Join(c.stores[1].Dir(), filepath.FromSlash(model.TaskPath(x.ULID)))
	if err := os.WriteFile(p, []byte("hand edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := mustHead(t, c.stores[1])
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrDirty) {
		t.Errorf("Run = %v", err)
	}
	if mustHead(t, c.stores[1]) != before {
		t.Error("a dirty sync changed HEAD")
	}
	if b, _ := os.ReadFile(p); string(b) != "hand edit\n" {
		t.Errorf("the hand edit is gone: %q", b)
	}
}

// A store has the merge driver's .gitattributes from its first commit, so a
// sync has nothing to set up in it.
func TestInitHasGitattributes(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	b, err := os.ReadFile(filepath.Join(c.stores[0].Dir(), ".gitattributes"))
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range []string{"tasks/**/*.md merge=hedgehug", "comments/**/*.md merge=hedgehug", "worklogs/**/*.md merge=hedgehug", "workspaces/*.md merge=hedgehug", "*.yaml merge=hedgehug"} {
		if !strings.Contains(string(b), l) {
			t.Errorf(".gitattributes lacks %q", l)
		}
	}
	cfg, _ := os.ReadFile(filepath.Join(c.stores[0].Dir(), ".git", "config"))
	if !strings.Contains(string(cfg), "driver = hedgehug merge-driver %O %A %B %P") {
		t.Errorf("driver not registered:\n%s", cfg)
	}
	e, _ := c.stores[0].Repo().LogFrom(mustHead(t, c.stores[0]), 1)
	if op, _ := e[0].Trailer("Hedgehug-Op"); op != "init" {
		t.Errorf("HEAD is %+v, want the init commit: the sync set up what init wrote", e[0])
	}
}

func TestSetupKeepsLinesAndCommitsOnce(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	s := c.stores[0]
	// Another line of the user's, and a line setup writes taken away.
	if _, err := s.Update(store.Op{Name: "edit", Author: c.opts[0].Author}, func(tx *store.Tx) error {
		return tx.PutFile(".gitattributes", []byte("*.png binary\ntasks/**/*.md merge=hedgehug"))
	}); err != nil {
		t.Fatal(err)
	}
	c.sync(0)
	b, _ := os.ReadFile(filepath.Join(s.Dir(), ".gitattributes"))
	want := "*.png binary\ntasks/**/*.md merge=hedgehug\ncomments/**/*.md merge=hedgehug\nworklogs/**/*.md merge=hedgehug\nworkspaces/*.md merge=hedgehug\n*.yaml merge=hedgehug\n"
	if string(b) != want {
		t.Errorf(".gitattributes = %q, want %q", b, want)
	}
	head := mustHead(t, s)
	e, _ := s.Repo().LogFrom(head, 1)
	if op, _ := e[0].Trailer("Hedgehug-Op"); op != "setup" || e[0].Subject != "hedgehug: setup merge driver" {
		t.Errorf("HEAD is not the setup commit: %+v", e[0])
	}
	c.sync(0)
	if mustHead(t, s) != head {
		t.Error("a second sync made a commit")
	}
}

func TestMirrorPush(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	mirror := filepath.Join(t.TempDir(), "m.git")
	_ = gitrepo.InitBare(mirror)
	c.opts[0].Remotes = append(c.opts[0].Remotes, mirror)
	c.addTask(0, "x")
	r := c.sync(0)
	if !slices.Equal(r.Pushed, []string{"origin", "mirror1"}) {
		t.Errorf("pushed = %v", r.Pushed)
	}
}

// A mirror that cannot be reached does not stop the sync with origin; it is
// told of as a warning. An origin that cannot be reached fails the sync.
func TestMirrorDownIsAWarning(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	missing := filepath.Join(t.TempDir(), "gone.git")
	c.opts[0].Remotes = append(c.opts[0].Remotes, missing)
	x := c.addTask(0, "x")
	r := c.sync(0)
	if !slices.Equal(r.Pushed, []string{"origin"}) || len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "mirror1") {
		t.Errorf("pushed = %v, warnings = %q", r.Pushed, r.Warnings)
	}
	c.sync(1)
	if c.task(1, x.ULID) == nil {
		t.Error("the task did not reach origin")
	}
	c.opts[0].Remotes = []string{missing}
	if _, err := Run(c.stores[0], c.opts[0]); err == nil {
		t.Error("a sync with origin down succeeded")
	}
}

func TestNoRemoteDoesNothing(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	head := mustHead(t, c.stores[0])
	c.opts[0].Remotes = nil
	if r, err := Run(c.stores[0], c.opts[0]); err != nil || r.Merges != 0 || r.Pushed != nil {
		t.Errorf("Run = %+v, %v", r, err)
	}
	if mustHead(t, c.stores[0]) != head {
		t.Error("HEAD moved")
	}
}

func TestUnrelatedHistories(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	other, err := store.Create(filepath.Join(t.TempDir(), "o"), &model.StoreMeta{ULID: model.NewULID(), Name: "o"}, nil,
		&model.Workspace{ULID: model.NewULID(), Name: "api", IDPrefix: "API"}, gitrepo.Signature{Name: "o", Email: "o@x"}, "", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	head := mustHead(t, other)
	opt := c.opts[0]
	if _, err := Run(other, opt); !errors.Is(err, ErrUnrelated) {
		t.Errorf("Run = %v", err)
	}
	// Nothing is committed, not even the missing .gitattributes.
	if mustHead(t, other) != head {
		t.Error("HEAD moved")
	}
}

func TestRepairCommitAfterHandMerge(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	a := c.addTask(0, "a")
	b := c.addTask(0, "b")
	// A hand-edited file gives b the id of a.
	c.set(0, b.ULID, func(t *model.Task) { t.ID = a.ID }) // store.Update keeps an id that fits the prefix
	r := c.sync(0)
	if len(r.Repaired) != 1 || r.Repaired[0].Kind != "id_collision" {
		t.Errorf("repaired = %+v", r.Repaired)
	}
	e, _ := c.stores[0].Repo().LogFrom(mustHead(t, c.stores[0]), 1)
	if op, _ := e[0].Trailer("Hedgehug-Op"); op != "repair" {
		t.Errorf("HEAD is not the repair commit: %+v", e[0])
	}
	if e[0].Subject != "hedgehug: repair 1 issues" {
		t.Errorf("subject = %q", e[0].Subject)
	}
}

func TestRevivedTrailer(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	c.sync(0)
	c.sync(1)
	c.set(0, x.ULID, func(t *model.Task) { t.DeletedAt = new(time.Now().Truncate(time.Second)) })
	c.set(1, x.ULID, func(t *model.Task) { t.Title = "kept" })
	c.sync(0)
	c.sync(1)
	if got := c.task(1, x.ULID); got.DeletedAt != nil || got.Title != "kept" {
		t.Errorf("x = %+v", got)
	}
	e, _ := c.stores[1].Repo().LogFrom(mustHead(t, c.stores[1]), 1)
	want := x.ULID.String() + " revived - github.com/u0"
	if !slices.Contains(e[0].TrailerValues("Hedgehug-Repaired"), want) {
		t.Errorf("trailers = %v, want %q", e[0].TrailerValues("Hedgehug-Repaired"), want)
	}
}

func TestPrefixDuplicateAcrossStores(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name       string
		otherSet   time.Time
		wantNotice bool
	}{
		{"other set earlier", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), true},
		{"other set later", time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), false},
	} {
		t.Run(c.name, func(t *testing.T) {
			cl := newCluster(t, 1)
			dir := filepath.Join(t.TempDir(), "other")
			ws := &model.Workspace{ULID: model.NewULID(), Name: "o", IDPrefix: "WEB", PrefixSet: c.otherSet}
			if _, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "other"}, nil, ws, gitrepo.Signature{Name: "o", Email: "o@x"}, "", time.Now); err != nil {
				t.Fatal(err)
			}
			cl.opts[0].Others = []OtherStore{{Name: "other", Path: dir}, {Name: "gone", Path: filepath.Join(t.TempDir(), "gone")}}
			r := cl.sync(0)
			got := slices.ContainsFunc(r.Repaired, func(x merge.Repaired) bool {
				return x.Kind == "prefix_duplicate" && strings.Contains(x.Value, `"store":"other"`) && x.Author == "github.com/u0"
			})
			if got != c.wantNotice {
				t.Errorf("notice = %v, want %v (%+v)", got, c.wantNotice, r.Repaired)
			}
			if items, _ := inbox.Open(cl.stores[0]).List(false); len(items) != len(r.Notices) || got != (len(items) == 1) ||
				got && (items[0].Kind != inbox.PrefixDuplicate || !strings.Contains(string(items[0].Detail), `"store":"other"`)) {
				t.Errorf("inbox = %+v, notices = %+v", items, r.Notices)
			}
			if r := cl.sync(0); len(r.Notices) != 0 { // told once, though each sync finds it
				t.Errorf("the next sync told again: %+v", r.Notices)
			}
		})
	}
}

func TestMergeSchemaBothSides(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	c.sync(1)
	addProp := func(i int, name string) {
		t.Helper()
		sc := *c.stores[i].Schema()
		sc.Props = append(slices.Clone(sc.Props), schema.Prop{Name: name, Type: schema.TypeText})
		if _, err := c.stores[i].Update(c.op(i, "schema", model.ULID{}), func(tx *store.Tx) error { return tx.PutSchema(&sc) }); err != nil {
			t.Fatal(err)
		}
	}
	addProp(0, "alpha")
	addProp(1, "beta")
	c.sync(0)
	r := c.sync(1)
	if r.Merges != 1 || len(r.Overwritten) != 0 {
		t.Errorf("result = %+v", r)
	}
	for _, name := range []string{"alpha", "beta"} {
		if _, ok := c.stores[1].Schema().Prop(name); !ok {
			t.Errorf("the merged schema lacks %s", name)
		}
	}
	c.sync(0)
	if c.headTree(0) != c.headTree(1) {
		t.Error("clones differ")
	}
}

func TestMergeViewBothSides(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	const p = "views/w.yaml"
	put := func(i int, data string) {
		t.Helper()
		if _, err := c.stores[i].Update(c.op(i, "view", model.ULID{}), func(tx *store.Tx) error { return tx.PutFile(p, []byte(data)) }); err != nil {
			t.Fatal(err)
		}
	}
	put(0, "layout: table\nquery: \"\"\n")
	c.sync(0)
	c.sync(1)
	put(0, "layout: kanban\nquery: \"\"\n")
	put(1, "layout: table\nquery: \"status:todo\"\n")
	c.sync(0)
	r := c.sync(1)
	if r.Merges != 1 || len(r.Overwritten) != 0 {
		t.Errorf("result = %+v", r)
	}
	b, err := os.ReadFile(filepath.Join(c.stores[1].Dir(), filepath.FromSlash(p)))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "layout: kanban\nquery: \"status:todo\"\n" {
		t.Errorf("merged view =\n%s", b)
	}
	c.sync(0)
	if c.headTree(0) != c.headTree(1) {
		t.Error("clones differ")
	}
}

func TestMergeViewSameKey(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	const p = "views/w.yaml"
	base := time.Now().Add(-time.Hour).Truncate(time.Second)
	put := func(i int, at time.Duration, data string) {
		t.Helper()
		c.stores[i].SetNow(func() time.Time { return base.Add(at) })
		if _, err := c.stores[i].Update(c.op(i, "view", model.ULID{}), func(tx *store.Tx) error { return tx.PutFile(p, []byte(data)) }); err != nil {
			t.Fatal(err)
		}
	}
	put(0, 0, "layout: table\nquery: \"\"\n")
	c.sync(0)
	c.sync(1)
	// u1 changes the query first, then the layout, so the file as a whole
	// changed last on u1's side; the query itself changed last on u0's.
	put(1, 10*time.Second, "layout: table\nquery: \"status:done\"\n")
	put(0, 20*time.Second, "layout: table\nquery: \"status:todo\"\n")
	put(1, 30*time.Second, "layout: kanban\nquery: \"status:done\"\n")
	c.sync(0)
	r := c.sync(1)
	if len(r.Overwritten) != 1 || r.Overwritten[0].Path != p || r.Overwritten[0].Field != "query" || r.Overwritten[0].Value != `"status:done"` {
		t.Errorf("overwritten = %+v", r.Overwritten)
	}
	b, err := os.ReadFile(filepath.Join(c.stores[1].Dir(), filepath.FromSlash(p)))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "layout: kanban\nquery: \"status:todo\"\n" {
		t.Errorf("merged view =\n%s", b)
	}
}

func TestState(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	s := c.stores[0]
	if st, err := ReadState(s); err != nil || st != (State{}) {
		t.Fatalf("ReadState = %+v, %v", st, err)
	}
	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	got, err := UpdateState(s, func(st *State) { st.PendingSince = at; st.LastError = "boom" })
	if err != nil || !got.PendingSince.Equal(at) || got.LastError != "boom" {
		t.Fatalf("UpdateState = %+v, %v", got, err)
	}
	if _, err := UpdateState(s, func(st *State) { st.ErrorReported = true }); err != nil {
		t.Fatal(err)
	}
	st, err := ReadState(s)
	if err != nil || !st.PendingSince.Equal(at) || st.LastError != "boom" || !st.ErrorReported || !st.LastSuccess.IsZero() {
		t.Errorf("ReadState = %+v, %v", st, err)
	}
	if b, _ := s.ReadLocal("sync.json"); strings.Contains(string(b), "last_success") {
		t.Errorf("sync.json = %s", b)
	}
}

func TestInvalidAfterMerge(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	y := c.addTask(0, "y")
	c.sync(0)
	c.sync(1)
	// Each side makes the other task block it: a cycle of blockers, which
	// the repair leaves for is:invalid.
	c.set(0, x.ULID, func(t *model.Task) { t.BlockedBy = []model.ULID{y.ULID} })
	c.set(1, y.ULID, func(t *model.Task) { t.BlockedBy = []model.ULID{x.ULID} })
	c.sync(0)
	r := c.sync(1)
	slices.SortFunc(r.Invalid, compareULIDs)
	want := []model.ULID{x.ULID, y.ULID}
	slices.SortFunc(want, compareULIDs)
	if !slices.Equal(r.Invalid, want) {
		t.Errorf("invalid = %v, want %v", r.Invalid, want)
	}
}

// Both sides remove the same file: the merge has nothing to write for it.
func TestMergeBothRemoveSameFile(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	w := &model.Workspace{ULID: model.NewULID(), Name: "api", IDPrefix: "API"}
	if _, err := c.stores[0].Update(c.op(0, "workspace", model.ULID{}), func(tx *store.Tx) error { return tx.PutWorkspace(w) }); err != nil {
		t.Fatal(err)
	}
	c.sync(0)
	c.sync(1)
	for i := range 2 {
		if _, err := c.stores[i].Update(c.op(i, "workspace", model.ULID{}), func(tx *store.Tx) error { return tx.RemoveWorkspace(w.ULID) }); err != nil {
			t.Fatal(err)
		}
	}
	c.sync(0)
	c.sync(1)
	c.sync(0)
	if c.headTree(0) != c.headTree(1) {
		t.Error("clones differ")
	}
	if _, err := c.stores[1].Workspace(w.ULID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("workspace after both removed it: %v", err)
	}
}

// A side that deleted a task and restored it did not change deleted_at as a
// three-way merge sees it, so a delete on the other side stays, and nothing
// is recorded as overwritten.
func TestRestoreAfterDeleteIsNoChange(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	c.sync(0)
	c.sync(1)
	now := time.Now().Truncate(time.Second)
	c.set(0, x.ULID, func(t *model.Task) { t.DeletedAt = &now })
	c.set(0, x.ULID, func(t *model.Task) { t.DeletedAt = nil })
	c.set(1, x.ULID, func(t *model.Task) { t.DeletedAt = &now })
	c.sync(0)
	r := c.sync(1)
	c.sync(0)
	if got := c.task(0, x.ULID); got.DeletedAt == nil {
		t.Error("the task is not deleted")
	}
	if len(r.Overwritten) != 0 {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
	if c.headTree(0) != c.headTree(1) {
		t.Error("clones differ")
	}
}

func TestMergeViewCreatedOnBothSides(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	const p = "views/w.yaml"
	put := func(i int, data string) {
		t.Helper()
		if _, err := c.stores[i].Update(c.op(i, "view", model.ULID{}), func(tx *store.Tx) error { return tx.PutFile(p, []byte(data)) }); err != nil {
			t.Fatal(err)
		}
	}
	put(0, "layout: table\n")
	put(1, "query: \"status:todo\"\n")
	c.sync(0)
	c.sync(1)
	c.sync(0)
	b, err := os.ReadFile(filepath.Join(c.stores[1].Dir(), filepath.FromSlash(p)))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); !strings.Contains(got, "layout: table") || !strings.Contains(got, `query: "status:todo"`) {
		t.Errorf("merged view =\n%s", got)
	}
	if c.headTree(0) != c.headTree(1) {
		t.Error("clones differ")
	}
}

func TestMergeViewRemovedAndChanged(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	const p = "views/w.yaml"
	if _, err := c.stores[0].Update(c.op(0, "view", model.ULID{}), func(tx *store.Tx) error {
		return tx.PutFile(p, []byte("layout: table\nquery: \"\"\n"))
	}); err != nil {
		t.Fatal(err)
	}
	c.sync(0)
	c.sync(1)
	if _, err := c.stores[0].Update(c.op(0, "view", model.ULID{}), func(tx *store.Tx) error { return tx.RemoveFile(p) }); err != nil {
		t.Fatal(err)
	}
	if _, err := c.stores[1].Update(c.op(1, "view", model.ULID{}), func(tx *store.Tx) error {
		return tx.PutFile(p, []byte("layout: table\nquery: \"status:todo\"\n"))
	}); err != nil {
		t.Fatal(err)
	}
	c.sync(0)
	c.sync(1)
	c.sync(0)
	for i := range 2 {
		b, err := os.ReadFile(filepath.Join(c.stores[i].Dir(), filepath.FromSlash(p)))
		if err != nil {
			t.Fatalf("clone %d lost the changed view: %v", i, err)
		}
		if !strings.Contains(string(b), "status:todo") {
			t.Errorf("clone %d view =\n%s", i, b)
		}
	}
	if c.headTree(0) != c.headTree(1) {
		t.Error("clones differ")
	}
}
