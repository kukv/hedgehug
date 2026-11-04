package stores_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	gosync "sync"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/app/usecase/workspaces"
)

// argsOf are the Args of err, which has been checked to be an *Error.
func argsOf(err error) map[string]any { return usecase.AsError(err).Args }

func TestMoveStore(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	ts := f.TS
	headsBefore := len(f.Log(t, "org-a")) + len(f.Log(t, "org-b"))

	res, err := stores.MoveStore(ts.Session, f.In())
	if err != nil {
		t.Fatal(err)
	}
	if res.Resumed || len(res.Moved) != 3 {
		t.Fatalf("res = %+v", res)
	}
	want := []struct {
		ulid     model.ULID
		old, now string
	}{{f.P, "API-2", "WEB-2"}, {f.C, "API-3", "WEB-3"}, {f.G, "API-4", "WEB-4"}}
	for i, w := range want {
		m := res.Moved[i]
		if m.ULID != w.ulid || m.OldID != w.old || m.NewID != w.now {
			t.Errorf("Moved[%d] = %+v, want %+v", i, m, w)
		}
	}

	// the destination has the three, with the old ids kept
	bs, err := ts.ForStore("org-b").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range want {
		nt, err := bs.Task(w.ulid)
		if err != nil {
			t.Fatal(err)
		}
		if nt.ID != w.now || !slices.Contains(nt.FormerIDs, w.old) {
			t.Errorf("%s: id %s former %v", w.old, nt.ID, nt.FormerIDs)
		}
	}
	g, _ := bs.Task(f.G)
	if len(g.BlockedBy) != 0 {
		t.Errorf("G keeps the blocker that stayed: %v", g.BlockedBy)
	}
	var comments int
	for c, err := range bs.Comments(f.C) {
		if err != nil {
			t.Fatal(err)
		}
		if c.ULID == f.Comment {
			comments++
		}
	}
	if comments != 1 {
		t.Errorf("%d comments in the destination", comments)
	}

	// the source has forwards, no notes, and the outsider is free
	as, err := ts.ForStore("org-a").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range want {
		if _, err := as.Task(w.ulid); !errors.Is(err, model.ErrForward) {
			t.Errorf("%s in the source: %v; want a forward", w.old, err)
		}
	}
	notes, err := as.NoteFiles(f.C)
	if err != nil || len(notes) != 0 {
		t.Errorf("notes in the source: %v, %v", notes, err)
	}
	o, err := as.Task(f.O)
	if err != nil || len(o.BlockedBy) != 0 {
		t.Errorf("O = %+v, %v", o, err)
	}
	if len(res.Plan.Unlinked) != 2 {
		t.Errorf("unlinked = %v", res.Plan.Unlinked)
	}
	lines := res.Plan.UnlinkLines()
	if !slices.Contains(lines, "unlink: API-4 blocked_by API-1") || !slices.Contains(lines, "unlink: API-5 blocked_by API-3") {
		t.Errorf("unlink lines = %q", lines)
	}

	// two commits, each by the account of its store
	if got := len(f.Log(t, "org-a")) + len(f.Log(t, "org-b")); got != headsBefore+2 {
		t.Errorf("%d commits made; want 2", got-headsBefore)
	}
	in, out := f.Log(t, "org-b")[0], f.Log(t, "org-a")[0]
	if op, _ := in.Trailer("Hedgehug-Op"); op != "move-in" || in.Author.Name != "bob" {
		t.Errorf("move-in: op %q author %q", op, in.Author.Name)
	}
	if op, _ := out.Trailer("Hedgehug-Op"); op != "move-out" || out.Author.Name != "alice" {
		t.Errorf("move-out: op %q author %q", op, out.Author.Name)
	}
	if from, _ := in.Trailer("Hedgehug-Moved-From"); from != as.Meta().ULID.String() {
		t.Errorf("Moved-From = %s", from)
	}
	if to, _ := out.Trailer("Hedgehug-Moved-To"); to != bs.Meta().ULID.String() {
		t.Errorf("Moved-To = %s", to)
	}
	if len(in.TrailerValues("Hedgehug-Unlinked")) != 2 || len(out.TrailerValues("Hedgehug-Unlinked")) != 2 {
		t.Errorf("Unlinked trailers: %v / %v", in.TrailerValues("Hedgehug-Unlinked"), out.TrailerValues("Hedgehug-Unlinked"))
	}
}

func TestMoveStoreFollowsTheGivenParentAndWorkspace(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	if _, err := workspaces.WorkspaceAdd(f.TS.ForStore("org-b"), "second", "SEC"); err != nil {
		t.Fatal(err)
	}
	_, err := stores.MoveStore(f.TS.Session, f.In())
	usecasetest.MustCode(t, err, usecase.KindUsage, "choice.required")
	e := argsOf(err)
	if e["what"] != "--workspace" {
		t.Errorf("what = %v", e["what"])
	}
	in := f.In()
	in.Workspace = "second"
	in.Parent = "WEB-1" // the parent is in org-b's first workspace
	_, err = stores.MoveStore(f.TS.Session, in)
	usecasetest.MustCode(t, err, usecase.KindInvalid, "move.invalid")
	if !strings.Contains(err.Error(), "the parent is in workspace") {
		t.Errorf("err = %v; want the parent to be named as in another workspace", err)
	}

	in.Parent = ""
	if _, err := stores.MoveStore(f.TS.Session, in); err != nil {
		t.Fatal(err)
	}
	ix := usecasetest.MustIndex(t, f.TS, "org-b")
	e2, _ := ix.Task(f.P)
	if e2.ID != "SEC-1" || e2.Parent != nil {
		t.Errorf("P = %s parent %v", e2.ID, e2.Parent)
	}
}

func TestMoveStoreUnderAParent(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	pb, err := tasks.Add(f.TS.ForStore("org-b"), tasks.AddInput{Title: "B project", Kind: "project"})
	if err != nil {
		t.Fatal(err)
	}
	in := f.In()
	in.Arg, in.Parent = "API-3", "WEB-2" // C, with G, under a project of org-b
	if _, err := stores.MoveStore(f.TS.Session, in); err != nil {
		t.Fatal(err)
	}
	ix := usecasetest.MustIndex(t, f.TS, "org-b")
	e, _ := ix.Task(f.C)
	if e.Parent == nil || *e.Parent != pb.ULID {
		t.Errorf("parent = %v; want WEB-2", e.Parent)
	}
	// a parent of another store is not found there
	f2 := usecasetest.NewMoveFx(t)
	in = f2.In()
	in.Parent = "API-1"
	_, err = stores.MoveStore(f2.TS.Session, in)
	if err == nil || usecase.KindOf(err) != usecase.KindInvalid || usecase.AsError(err).Code != "ref.cross_store" {
		t.Errorf("err = %v; want KindInvalid ref.cross_store for a parent of the source", err)
	}
}

func TestMoveStoreRefusedBySchemaCommitsNothing(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetSchema(t, func(sc *schema.Schema) {
		sc.Statuses = append(sc.Statuses, schema.Status{Name: "review", Category: schema.CategoryDoing})
		sc.Kinds = append(sc.Kinds, schema.Kind{Name: "spike", Role: schema.RoleWork, Parents: []string{schema.Root, "project", "epic", "story", "task"}})
	})
	a := f.TS.ForStore("org-a")
	if _, err := tasks.Set(a, "API-4", []usecase.Assign{{Key: "status", Op: "=", Value: "review"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Set(a, "API-4", []usecase.Assign{{Key: "kind", Op: "=", Value: "spike"}}); err != nil {
		t.Fatal(err)
	}
	ha, hb := f.Head(t, "org-a"), f.Head(t, "org-b")

	_, err := stores.MoveStore(f.TS.Session, f.In())
	usecasetest.MustCode(t, err, usecase.KindInvalid, "move.invalid")
	e := argsOf(err)
	lines, _ := e["violations"].([]string)
	if len(lines) != 2 {
		t.Fatalf("violations = %q", lines)
	}
	if !strings.Contains(lines[0], "API-4") || !strings.Contains(lines[0], "spike") || !strings.Contains(lines[1], "API-4") || !strings.Contains(lines[1], "review") {
		t.Errorf("violations = %q", lines)
	}
	if f.Head(t, "org-a") != ha || f.Head(t, "org-b") != hb {
		t.Error("a store was written")
	}
}

func TestMoveStoreNeedsConfirmationAndScopedChoice(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetSchema(t, func(sc *schema.Schema) {
		sc.Statuses = append(sc.Statuses, schema.Status{Name: "review", Category: schema.CategoryDoing})
		sc.Scoped = []schema.Scoped{{
			Where:    "project:" + f.P.String(),
			Statuses: []string{"backlog", "todo", "review", "in_progress", "done", "canceled"},
			Workflow: &schema.Workflow{Transitions: schema.Lists{{Key: "todo", Values: []string{"review", "in_progress"}}}},
		}}
	})
	in := f.In()
	in.Yes = false
	_, err := stores.MoveStore(f.TS.Session, in)
	usecasetest.MustCode(t, err, usecase.KindUsage, "choice.required")
	e := argsOf(err)
	if e["what"] != "--scoped" || !slices.Equal(e["candidates"].([]string), []string{"carry", "drop"}) {
		t.Errorf("args = %v", e)
	}
	in.Yes = true
	_, err = stores.MoveStore(f.TS.Session, in)
	usecasetest.MustCode(t, err, usecase.KindUsage, "choice.required")

	in.Scoped = "carry"
	in.Yes = false
	_, err = stores.MoveStore(f.TS.Session, in)
	usecasetest.MustCode(t, err, usecase.KindUsage, "confirm.required")
	e = argsOf(err)
	view, ok := e["plan"].(stores.MoveView)
	if !ok || len(view.Tasks) != 3 || !slices.Contains(view.Scoped, `scoped: dropped status "review" (not in org-b)`) {
		t.Fatalf("plan = %+v", e["plan"])
	}
	var ce *stores.MoveConfirmError
	if !errors.As(err, &ce) || ce.Plan.Scoped == nil || !slices.Contains(ce.Plan.Scoped.Dropped, "status review") {
		t.Fatalf("the error has no plan: %v", err)
	}
	plan := ce.Plan
	if f.Head(t, "org-b") == "" || len(f.Log(t, "org-b")) != 2 { // init and add
		t.Error("the destination was written")
	}

	// the plan shown is checked against what the stores hold at the time
	if _, err := tasks.Add(f.TS.ForStore("org-a"), tasks.AddInput{Title: "late", Parent: "API-4"}); err != nil {
		t.Fatal(err)
	}
	in.Yes, in.Confirmed = true, &plan
	_, err = stores.MoveStore(f.TS.Session, in)
	usecasetest.MustCode(t, err, usecase.KindConflict, "move.changed")

	res, err := stores.MoveStore(f.TS.Session, stores.MoveStoreInput{Arg: "API-2", To: "org-b", Scoped: "carry", Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Carried || len(res.Moved) != 4 {
		t.Errorf("res = %+v", res)
	}
	bs, _ := f.TS.ForStore("org-b").OpenStore()
	got := bs.Schema().Scoped
	if len(got) != 1 || got[0].Where != "project:"+f.P.String() || slices.Contains(got[0].Statuses, "review") {
		t.Errorf("scoped in the destination = %+v", got)
	}
	if v, _ := got[0].Workflow.Transitions.Get("todo"); !slices.Equal(v, []string{"in_progress"}) {
		t.Errorf("transitions = %v", v)
	}
	as, _ := f.TS.ForStore("org-a").OpenStore()
	if len(as.Schema().Scoped) != 0 {
		t.Errorf("scoped left in the source: %+v", as.Schema().Scoped)
	}
}

func TestMoveStoreDropsScoped(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetSchema(t, func(sc *schema.Schema) {
		sc.Scoped = []schema.Scoped{{Where: "project:" + f.P.String(), Statuses: []string{"backlog", "todo", "in_progress", "done", "canceled"}}}
	})
	in := f.In()
	in.Scoped = "drop"
	res, err := stores.MoveStore(f.TS.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	if res.Carried {
		t.Error("carried")
	}
	bs, _ := f.TS.ForStore("org-b").OpenStore()
	as, _ := f.TS.ForStore("org-a").OpenStore()
	if len(bs.Schema().Scoped) != 0 || len(as.Schema().Scoped) != 0 {
		t.Errorf("scoped: dest %+v, source %+v", bs.Schema().Scoped, as.Schema().Scoped)
	}
}

func TestMoveStoreScopedIgnoredWhenNotNeeded(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	in := f.In()
	in.Scoped = "carry" // D7: nothing to carry
	if _, err := stores.MoveStore(f.TS.Session, in); err != nil {
		t.Fatal(err)
	}
	in = f.In()
	in.Scoped = "sideways"
	_, err := stores.MoveStore(f.TS.Session, in)
	usecasetest.MustCode(t, err, usecase.KindUsage, "usage")
}

func TestMoveStoreScopedThatCannotBeCarried(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetSchema(t, func(sc *schema.Schema) {
		// review is the default of doing here, so a project can hide in_progress;
		// the destination has no review, and needs in_progress
		for i := range sc.Statuses {
			if sc.Statuses[i].Name == "in_progress" {
				sc.Statuses[i].Default = false
			}
		}
		sc.Statuses = append(sc.Statuses, schema.Status{Name: "review", Category: schema.CategoryDoing, Default: true})
		sc.Scoped = []schema.Scoped{{Where: "project:" + f.P.String(), Statuses: []string{"backlog", "todo", "review", "done", "canceled"}}}
	})
	in := f.In()
	in.Scoped = "carry"
	_, err := stores.MoveStore(f.TS.Session, in)
	usecasetest.MustCode(t, err, usecase.KindInvalid, "move.invalid")
	in.Scoped = "drop"
	if _, err := stores.MoveStore(f.TS.Session, in); err != nil {
		t.Fatal(err)
	}
}

func TestMoveStoreRerunFinishesTheSource(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	f.SetSchema(t, func(sc *schema.Schema) {
		sc.Scoped = []schema.Scoped{{Where: "project:" + f.P.String(), Statuses: []string{"backlog", "todo", "in_progress", "done", "canceled"}}}
	})
	in := f.In()
	in.Scoped = "carry"
	as, _ := f.TS.ForStore("org-a").OpenStore()
	gPath := filepath.Join(as.Dir(), filepath.FromSlash(model.TaskPath(f.G)))
	gBytes, err := os.ReadFile(gPath)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	f.TS.BeforeUpdate = func() {
		if calls++; calls == 2 { // the write into the source
			_ = os.Remove(gPath)
		}
	}
	_, err = stores.MoveStore(f.TS.Session, in)
	usecasetest.MustCode(t, err, usecase.KindNotFound, "move.unfinished")
	if !strings.Contains(err.Error(), "moved into org-b; run the same command again to finish the source") {
		t.Errorf("err = %v", err)
	}
	f.TS.BeforeUpdate = nil
	if err := os.WriteFile(gPath, gBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	hb := f.Head(t, "org-b")
	if _, err := as.Task(f.P); err != nil {
		t.Fatalf("the source changed: %v", err)
	}

	res, err := stores.MoveStore(f.TS.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resumed || !res.Carried || len(res.Moved) != 3 {
		t.Errorf("res = %+v", res)
	}
	if f.Head(t, "org-b") != hb {
		t.Error("the destination was written again")
	}
	as2, _ := f.TS.ForStore("org-a").OpenStore()
	if _, err := as2.Task(f.P); !errors.Is(err, model.ErrForward) {
		t.Errorf("P in the source: %v", err)
	}
	if op, _ := f.Log(t, "org-a")[0].Trailer("Hedgehug-Op"); op != "move-out" {
		t.Errorf("last op of the source = %s", op)
	}
}

func TestMoveStoreRefusesWhatIsInTheDestination(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	bss := f.TS.ForStore("org-b")
	bs, ix, err := bss.Open()
	if err != nil {
		t.Fatal(err)
	}
	// the project is in org-b, with no move-in to say why
	stray := &model.Task{ULID: f.P, Title: "stray", Kind: "project", Status: "todo", Workspace: ix.Workspaces()[0].ULID}
	if _, err := bss.Update(bs, store.Op{Name: "add", Task: f.P}, func(tx *store.Tx) error { return tx.PutTask(stray) }); err != nil {
		t.Fatal(err)
	}
	hs, hd := f.Head(t, "org-a"), f.Head(t, "org-b")
	_, err = stores.MoveStore(f.TS.Session, f.In())
	usecasetest.MustCode(t, err, usecase.KindConflict, "move.exists")
	e := argsOf(err)
	if !strings.Contains(err.Error(), f.P.String()+" is already in org-b") {
		t.Errorf("err = %v (%v)", err, e)
	}
	if f.Head(t, "org-a") != hs || f.Head(t, "org-b") != hd {
		t.Error("a store was written")
	}
}

func TestMoveStoreSameStoreAndReadOnly(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	in := f.In()
	in.To = "org-a"
	_, err := stores.MoveStore(f.TS.Session, in)
	usecasetest.MustCode(t, err, usecase.KindUsage, "move.same_store")
	e := argsOf(err)
	if !strings.Contains(err.Error(), "hedgehug mv --workspace") {
		t.Errorf("err = %v (%v)", err, e)
	}
	_, err = stores.MoveStore(f.TS.Session, stores.MoveStoreInput{Arg: "API-2", To: "nowhere", Yes: true})
	usecasetest.MustCode(t, err, usecase.KindNotFound, "store.not_found")

	for _, name := range []string{"org-a", "org-b"} {
		s, err := f.TS.ForStore(name).OpenStore()
		if err != nil {
			t.Fatal(err)
		}
		if err := replicate.WriteAccess(s, replicate.Access{ReadOnly: true, Reason: replicate.ReasonNotWritable, Checked: f.TS.Now()}); err != nil {
			t.Fatal(err)
		}
		hs, hd := f.Head(t, "org-a"), f.Head(t, "org-b")
		_, err = stores.MoveStore(f.TS.Session, f.In())
		usecasetest.MustCode(t, err, usecase.KindConflict, "store.read_only")
		_, err = stores.PrepareMoveStore(f.TS.Session, f.In())
		usecasetest.MustCode(t, err, usecase.KindConflict, "store.read_only")
		if f.Head(t, "org-a") != hs || f.Head(t, "org-b") != hd {
			t.Errorf("%s read-only: a store was written", name)
		}
		if err := replicate.WriteAccess(s, replicate.Access{Checked: f.TS.Now()}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMoveStoreQualifiedSource(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	in := f.In()
	in.Arg = "org-a:API-2"
	if _, err := stores.MoveStore(f.TS.Session, in); err != nil {
		t.Fatal(err)
	}
}

func TestMoveStoreBothWaysAtOnce(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	b, err := tasks.Add(f.TS.ForStore("org-b"), tasks.AddInput{Title: "from b"})
	if err != nil {
		t.Fatal(err)
	}
	var wg gosync.WaitGroup
	errs := make([]error, 2)
	for i, in := range []stores.MoveStoreInput{
		{Arg: "API-2", To: "org-b", Yes: true},
		{Arg: "WEB-2", To: "org-a", Yes: true},
	} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = stores.MoveStore(f.TS.Session, in)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("move %d: %v", i, err)
		}
	}
	as, _ := f.TS.ForStore("org-a").OpenStore()
	if nt, err := as.Task(b.ULID); err != nil || nt.ID == "" {
		t.Errorf("the task from b: %v", err)
	}
	bs, _ := f.TS.ForStore("org-b").OpenStore()
	if _, err := bs.Task(f.P); err != nil {
		t.Errorf("P in org-b: %v", err)
	}
}

func TestMoveStoreHidden(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	p, err := stores.PrepareMoveStore(f.TS.Session, f.In())
	if err != nil {
		t.Fatal(err)
	}
	if p.Hidden != nil {
		t.Errorf("hidden = %v without member lists", p.Hidden)
	}
	set := func(name string, ids ...string) {
		s, err := f.TS.ForStore(name).OpenStore()
		if err != nil {
			t.Fatal(err)
		}
		cs := []replicate.Collaborator{}
		for _, id := range ids {
			cs = append(cs, replicate.Collaborator{ID: id, Permission: "write"})
		}
		if err := replicate.WriteMembers(s, replicate.Members{Collaborators: cs}); err != nil {
			t.Fatal(err)
		}
	}
	set("org-a", "github.com/alice", "github.com/bob", "github.com/Carol")
	p, _ = stores.PrepareMoveStore(f.TS.Session, f.In())
	if p.Hidden != nil {
		t.Errorf("hidden = %v when the destination's list is unknown", p.Hidden)
	}
	set("org-b", "github.com/alice", "github.com/carol")
	p, err = stores.PrepareMoveStore(f.TS.Session, f.In())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.Hidden, []string{"github.com/bob"}) {
		t.Errorf("hidden = %v", p.Hidden)
	}
}

func TestMoveStoreUnlinksSprintAndReleases(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	as, _ := f.TS.ForStore("org-a").OpenStore()
	_, err := f.TS.ForStore("org-a").Update(as, store.Op{Name: "sprint"}, func(tx *store.Tx) error {
		c, err := tx.Task(f.C)
		if err != nil {
			return err
		}
		// a sprint belongs to a workspace; the task's own one here
		sp := &model.Sprint{ULID: model.NewULID(), Workspace: c.Workspace, Name: "Sprint 1"}
		if err := tx.PutSprint(sp); err != nil {
			return err
		}
		c.Sprint = &sp.ULID
		return tx.PutTask(c)
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := stores.PrepareMoveStore(f.TS.Session, f.In())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(p.Unlinked, "unlink: API-3 sprint Sprint 1") {
		t.Errorf("lines = %q", p.Unlinked)
	}
	if _, err := stores.MoveStore(f.TS.Session, f.In()); err != nil {
		t.Fatal(err)
	}
	bs, _ := f.TS.ForStore("org-b").OpenStore()
	if c, _ := bs.Task(f.C); c.Sprint != nil {
		t.Errorf("sprint = %v in the destination", c.Sprint)
	}
}

func TestMoveStoreParentsComeBeforeTheirChildren(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	a := ts.ForStore("org-a")
	add := func(title, parent string) model.ULID {
		r, err := tasks.Add(a, tasks.AddInput{Title: title, Parent: parent})
		if err != nil {
			t.Fatal(err)
		}
		return r.ULID
	}
	root := add("R", "")
	g := add("G", "API-2") // older than C
	c := add("C", "API-2")
	if _, err := tasks.Move(a, tasks.MoveInput{Arg: "API-3", Parent: "API-4"}); err != nil {
		t.Fatal(err)
	}
	if g.String() >= c.String() {
		t.Fatalf("G %s should sort before C %s", g, c)
	}
	res, err := stores.MoveStore(ts.Session, stores.MoveStoreInput{Arg: "API-2", To: "org-b", Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Moved) != 3 || res.Moved[0].ULID != root || res.Moved[1].ULID != c || res.Moved[2].ULID != g {
		t.Errorf("moved = %+v; want R, C, G", res.Moved)
	}
}

func TestMoveStoreWarnsOnceOfAFileThatDoesNotDecode(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	as, _ := f.TS.ForStore("org-a").OpenStore()
	bad := model.NewULID()
	rel := model.TaskPath(bad)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(as.Dir(), filepath.FromSlash(rel))), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(as.Dir(), filepath.FromSlash(rel)), []byte("not a task"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := stores.MoveStore(f.TS.Session, f.In()); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, w := range f.TS.Warns {
		if strings.Contains(w, rel) && strings.Contains(w, "does not decode") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d warnings about %s; want 1 (all: %q)", n, rel, f.TS.Warns)
	}
}

// A task that moved away can move back: the move-in replaces the forward
// with the task, which gets a new id of the source, keeps its former ids, and
// the store it left now holds the forward.
func TestMoveStoreBackToTheStoreItLeft(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	ts := f.TS
	if _, err := stores.MoveStore(ts.Session, f.In()); err != nil {
		t.Fatal(err)
	}
	res, err := stores.MoveStore(ts.Session, stores.MoveStoreInput{Arg: "WEB-2", To: "org-a", Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Resumed || len(res.Moved) != 3 {
		t.Fatalf("res = %+v", res)
	}
	as, err := ts.ForStore("org-a").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	bs, err := ts.ForStore("org-b").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	p, err := as.Task(f.P)
	if err != nil {
		t.Fatalf("the project is not a task of org-a again: %v", err)
	}
	// API-5 is taken and API-2..4 are recorded as given: the numbers go on.
	if p.ID != "API-6" || !slices.Contains(p.FormerIDs, "API-2") || !slices.Contains(p.FormerIDs, "WEB-2") {
		t.Errorf("id %s former %v", p.ID, p.FormerIDs)
	}
	for _, u := range []model.ULID{f.P, f.C, f.G} {
		if _, err := bs.Task(u); !errors.Is(err, model.ErrForward) {
			t.Errorf("%s in org-b: %v; want a forward", u, err)
		}
	}
}
