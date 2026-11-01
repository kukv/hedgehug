package ops

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	gosync "sync"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// moveFx is org-a (API) with a project API-2, its child API-3 and that
// one's child API-4, and API-5 outside; API-3 has a comment and a worklog
// entry; API-4 is blocked by API-1 and API-5 by API-3. org-b is WEB.
type moveFx struct {
	ts                     *testSession
	p, c, g, o, first      model.ULID
	comment, worklog       model.ULID
	commentRel, worklogRel string
}

func newMoveFx(t *testing.T) *moveFx {
	t.Helper()
	ts := newTwoStoreSession(t)
	a := ts.ForStore("org-a")
	add := func(title, parent, kind string) model.ULID {
		r, err := a.Add(AddInput{Title: title, Parent: parent, Kind: kind})
		if err != nil {
			t.Fatal(err)
		}
		return r.ULID
	}
	f := &moveFx{ts: ts}
	f.p = add("P", "", "project")
	f.c = add("C", "API-2", "")
	f.g = add("G", "API-3", "")
	f.o = add("O", "", "")
	ix := mustIndex(t, ts, "org-a")
	e, err := a.Resolve(ix, "API-1")
	if err != nil {
		t.Fatal(err)
	}
	f.first = e.ULID
	if _, err := a.Dep("API-4", "API-1", true); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Dep("API-5", "API-3", true); err != nil {
		t.Fatal(err)
	}
	f.comment, f.worklog = model.NewULID(), model.NewULID()
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Update(s, store.Op{Name: "comment", Task: f.c}, func(tx *store.Tx) error {
		if err := tx.PutComment(f.c, &model.Comment{ULID: f.comment, Author: "github.com/alice", Body: "first"}); err != nil {
			return err
		}
		return tx.PutWorklog(f.c, &model.Worklog{ULID: f.worklog, Author: "github.com/alice", Duration: 3600, Body: "work"})
	})
	if err != nil {
		t.Fatal(err)
	}
	f.commentRel, f.worklogRel = model.CommentPath(f.c, f.comment), model.WorklogPath(f.c, f.worklog)
	return f
}

func (f *moveFx) in() MoveStoreInput {
	return MoveStoreInput{Arg: "API-2", To: "org-b", Yes: true}
}

func (f *moveFx) head(t *testing.T, name string) string {
	t.Helper()
	s, err := f.ts.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	h, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	return h.String()
}

func (f *moveFx) log(t *testing.T, name string) []gitrepo.LogEntry {
	t.Helper()
	s, err := f.ts.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	l, err := s.Repo().Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// setSchema changes the schema of org-a.
func (f *moveFx) setSchema(t *testing.T, fn func(*schema.Schema)) {
	t.Helper()
	ss := f.ts.ForStore("org-a")
	s, err := ss.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	sc, err := cloneSchemaOf(s.Schema())
	if err != nil {
		t.Fatal(err)
	}
	fn(sc)
	if _, err := ss.Update(s, store.Op{Name: "schema"}, func(tx *store.Tx) error { return tx.PutSchema(sc) }); err != nil {
		t.Fatal(err)
	}
}

func mustCode(t *testing.T, err error, kind Kind, code string) {
	t.Helper()
	if e := AsError(err); err == nil || e.Kind != kind || e.Code != code {
		t.Fatalf("err = %v; want kind %v code %s", err, kind, code)
	}
}

// argsOf are the Args of err, which has been checked to be an *Error.
func argsOf(err error) map[string]any { return AsError(err).Args }

func TestMoveStore(t *testing.T) {
	f := newMoveFx(t)
	ts := f.ts
	headsBefore := len(f.log(t, "org-a")) + len(f.log(t, "org-b"))

	res, err := ts.MoveStore(f.in())
	if err != nil {
		t.Fatal(err)
	}
	if res.Resumed || len(res.Moved) != 3 {
		t.Fatalf("res = %+v", res)
	}
	want := []struct {
		ulid     model.ULID
		old, now string
	}{{f.p, "API-2", "WEB-2"}, {f.c, "API-3", "WEB-3"}, {f.g, "API-4", "WEB-4"}}
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
	g, _ := bs.Task(f.g)
	if len(g.BlockedBy) != 0 {
		t.Errorf("G keeps the blocker that stayed: %v", g.BlockedBy)
	}
	var comments int
	for c, err := range bs.Comments(f.c) {
		if err != nil {
			t.Fatal(err)
		}
		if c.ULID == f.comment {
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
	notes, err := as.NoteFiles(f.c)
	if err != nil || len(notes) != 0 {
		t.Errorf("notes in the source: %v, %v", notes, err)
	}
	o, err := as.Task(f.o)
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
	if got := len(f.log(t, "org-a")) + len(f.log(t, "org-b")); got != headsBefore+2 {
		t.Errorf("%d commits made; want 2", got-headsBefore)
	}
	in, out := f.log(t, "org-b")[0], f.log(t, "org-a")[0]
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
	f := newMoveFx(t)
	if _, err := f.ts.ForStore("org-b").WorkspaceAdd("second", "SEC"); err != nil {
		t.Fatal(err)
	}
	_, err := f.ts.MoveStore(f.in())
	mustCode(t, err, KindUsage, "choice.required")
	e := argsOf(err)
	if e["what"] != "--workspace" {
		t.Errorf("what = %v", e["what"])
	}
	in := f.in()
	in.Workspace = "second"
	in.Parent = "WEB-1" // the parent is in org-b's first workspace
	_, err = f.ts.MoveStore(in)
	mustCode(t, err, KindInvalid, "move.invalid")
	if !strings.Contains(err.Error(), "the parent is in workspace") {
		t.Errorf("err = %v; want the parent to be named as in another workspace", err)
	}

	in.Parent = ""
	if _, err := f.ts.MoveStore(in); err != nil {
		t.Fatal(err)
	}
	ix := mustIndex(t, f.ts, "org-b")
	e2, _ := ix.Task(f.p)
	if e2.ID != "SEC-1" || e2.Parent != nil {
		t.Errorf("P = %s parent %v", e2.ID, e2.Parent)
	}
}

func TestMoveStoreUnderAParent(t *testing.T) {
	f := newMoveFx(t)
	pb, err := f.ts.ForStore("org-b").Add(AddInput{Title: "B project", Kind: "project"})
	if err != nil {
		t.Fatal(err)
	}
	in := f.in()
	in.Arg, in.Parent = "API-3", "WEB-2" // C, with G, under a project of org-b
	if _, err := f.ts.MoveStore(in); err != nil {
		t.Fatal(err)
	}
	ix := mustIndex(t, f.ts, "org-b")
	e, _ := ix.Task(f.c)
	if e.Parent == nil || *e.Parent != pb.ULID {
		t.Errorf("parent = %v; want WEB-2", e.Parent)
	}
	// a parent of another store is not found there
	f2 := newMoveFx(t)
	in = f2.in()
	in.Parent = "API-1"
	_, err = f2.ts.MoveStore(in)
	if err == nil || KindOf(err) != KindInvalid || AsError(err).Code != "ref.cross_store" {
		t.Errorf("err = %v; want KindInvalid ref.cross_store for a parent of the source", err)
	}
}

func TestMoveStoreRefusedBySchemaCommitsNothing(t *testing.T) {
	f := newMoveFx(t)
	f.setSchema(t, func(sc *schema.Schema) {
		sc.Statuses = append(sc.Statuses, schema.Status{Name: "review", Category: schema.CategoryDoing})
		sc.Kinds = append(sc.Kinds, schema.Kind{Name: "spike", Role: schema.RoleWork, Parents: []string{schema.Root, "project", "epic", "story", "task"}})
	})
	a := f.ts.ForStore("org-a")
	if _, err := a.Set("API-4", []Assign{{Key: "status", Op: "=", Value: "review"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Set("API-4", []Assign{{Key: "kind", Op: "=", Value: "spike"}}); err != nil {
		t.Fatal(err)
	}
	ha, hb := f.head(t, "org-a"), f.head(t, "org-b")

	_, err := f.ts.MoveStore(f.in())
	mustCode(t, err, KindInvalid, "move.invalid")
	e := argsOf(err)
	lines, _ := e["violations"].([]string)
	if len(lines) != 2 {
		t.Fatalf("violations = %q", lines)
	}
	if !strings.Contains(lines[0], "API-4") || !strings.Contains(lines[0], "spike") || !strings.Contains(lines[1], "API-4") || !strings.Contains(lines[1], "review") {
		t.Errorf("violations = %q", lines)
	}
	if f.head(t, "org-a") != ha || f.head(t, "org-b") != hb {
		t.Error("a store was written")
	}
}

func TestMoveStoreNeedsConfirmationAndScopedChoice(t *testing.T) {
	f := newMoveFx(t)
	f.setSchema(t, func(sc *schema.Schema) {
		sc.Statuses = append(sc.Statuses, schema.Status{Name: "review", Category: schema.CategoryDoing})
		sc.Scoped = []schema.Scoped{{
			Where:    "project:" + f.p.String(),
			Statuses: []string{"backlog", "todo", "review", "in_progress", "done", "canceled"},
			Workflow: &schema.Workflow{Transitions: schema.Lists{{Key: "todo", Values: []string{"review", "in_progress"}}}},
		}}
	})
	in := f.in()
	in.Yes = false
	_, err := f.ts.MoveStore(in)
	mustCode(t, err, KindUsage, "choice.required")
	e := argsOf(err)
	if e["what"] != "--scoped" || !slices.Equal(e["candidates"].([]string), []string{"carry", "drop"}) {
		t.Errorf("args = %v", e)
	}
	in.Yes = true
	_, err = f.ts.MoveStore(in)
	mustCode(t, err, KindUsage, "choice.required")

	in.Scoped = "carry"
	in.Yes = false
	_, err = f.ts.MoveStore(in)
	mustCode(t, err, KindUsage, "confirm.required")
	e = argsOf(err)
	plan, ok := e["plan"].(MovePlan)
	if !ok || len(plan.Tasks) != 3 || plan.Scoped == nil || !slices.Contains(plan.Scoped.Dropped, "status review") {
		t.Fatalf("plan = %+v", e["plan"])
	}
	if f.head(t, "org-b") == "" || len(f.log(t, "org-b")) != 2 { // init and add
		t.Error("the destination was written")
	}

	// the plan shown is checked against what the stores hold at the time
	if _, err := f.ts.ForStore("org-a").Add(AddInput{Title: "late", Parent: "API-4"}); err != nil {
		t.Fatal(err)
	}
	in.Yes, in.Confirmed = true, &plan
	_, err = f.ts.MoveStore(in)
	mustCode(t, err, KindConflict, "move.changed")

	res, err := f.ts.MoveStore(MoveStoreInput{Arg: "API-2", To: "org-b", Scoped: "carry", Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Carried || len(res.Moved) != 4 {
		t.Errorf("res = %+v", res)
	}
	bs, _ := f.ts.ForStore("org-b").OpenStore()
	got := bs.Schema().Scoped
	if len(got) != 1 || got[0].Where != "project:"+f.p.String() || slices.Contains(got[0].Statuses, "review") {
		t.Errorf("scoped in the destination = %+v", got)
	}
	if v, _ := got[0].Workflow.Transitions.Get("todo"); !slices.Equal(v, []string{"in_progress"}) {
		t.Errorf("transitions = %v", v)
	}
	as, _ := f.ts.ForStore("org-a").OpenStore()
	if len(as.Schema().Scoped) != 0 {
		t.Errorf("scoped left in the source: %+v", as.Schema().Scoped)
	}
}

func TestMoveStoreDropsScoped(t *testing.T) {
	f := newMoveFx(t)
	f.setSchema(t, func(sc *schema.Schema) {
		sc.Scoped = []schema.Scoped{{Where: "project:" + f.p.String(), Statuses: []string{"backlog", "todo", "in_progress", "done", "canceled"}}}
	})
	in := f.in()
	in.Scoped = "drop"
	res, err := f.ts.MoveStore(in)
	if err != nil {
		t.Fatal(err)
	}
	if res.Carried {
		t.Error("carried")
	}
	bs, _ := f.ts.ForStore("org-b").OpenStore()
	as, _ := f.ts.ForStore("org-a").OpenStore()
	if len(bs.Schema().Scoped) != 0 || len(as.Schema().Scoped) != 0 {
		t.Errorf("scoped: dest %+v, source %+v", bs.Schema().Scoped, as.Schema().Scoped)
	}
}

func TestMoveStoreScopedIgnoredWhenNotNeeded(t *testing.T) {
	f := newMoveFx(t)
	in := f.in()
	in.Scoped = "carry" // D7: nothing to carry
	if _, err := f.ts.MoveStore(in); err != nil {
		t.Fatal(err)
	}
	in = f.in()
	in.Scoped = "sideways"
	_, err := f.ts.MoveStore(in)
	mustCode(t, err, KindUsage, "usage")
}

func TestMoveStoreScopedThatCannotBeCarried(t *testing.T) {
	f := newMoveFx(t)
	f.setSchema(t, func(sc *schema.Schema) {
		// review is the default of doing here, so a project can hide in_progress;
		// the destination has no review, and needs in_progress
		for i := range sc.Statuses {
			if sc.Statuses[i].Name == "in_progress" {
				sc.Statuses[i].Default = false
			}
		}
		sc.Statuses = append(sc.Statuses, schema.Status{Name: "review", Category: schema.CategoryDoing, Default: true})
		sc.Scoped = []schema.Scoped{{Where: "project:" + f.p.String(), Statuses: []string{"backlog", "todo", "review", "done", "canceled"}}}
	})
	in := f.in()
	in.Scoped = "carry"
	_, err := f.ts.MoveStore(in)
	mustCode(t, err, KindInvalid, "move.invalid")
	in.Scoped = "drop"
	if _, err := f.ts.MoveStore(in); err != nil {
		t.Fatal(err)
	}
}

func TestMoveStoreRerunFinishesTheSource(t *testing.T) {
	f := newMoveFx(t)
	f.setSchema(t, func(sc *schema.Schema) {
		sc.Scoped = []schema.Scoped{{Where: "project:" + f.p.String(), Statuses: []string{"backlog", "todo", "in_progress", "done", "canceled"}}}
	})
	in := f.in()
	in.Scoped = "carry"
	as, _ := f.ts.ForStore("org-a").OpenStore()
	gPath := filepath.Join(as.Dir(), filepath.FromSlash(model.TaskPath(f.g)))
	gBytes, err := os.ReadFile(gPath)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	f.ts.BeforeUpdate = func() {
		if calls++; calls == 2 { // the write into the source
			_ = os.Remove(gPath)
		}
	}
	_, err = f.ts.MoveStore(in)
	mustCode(t, err, KindNotFound, "move.unfinished")
	if !strings.Contains(err.Error(), "moved into org-b; run the same command again to finish the source") {
		t.Errorf("err = %v", err)
	}
	f.ts.BeforeUpdate = nil
	if err := os.WriteFile(gPath, gBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	hb := f.head(t, "org-b")
	if _, err := as.Task(f.p); err != nil {
		t.Fatalf("the source changed: %v", err)
	}

	res, err := f.ts.MoveStore(in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resumed || !res.Carried || len(res.Moved) != 3 {
		t.Errorf("res = %+v", res)
	}
	if f.head(t, "org-b") != hb {
		t.Error("the destination was written again")
	}
	as2, _ := f.ts.ForStore("org-a").OpenStore()
	if _, err := as2.Task(f.p); !errors.Is(err, model.ErrForward) {
		t.Errorf("P in the source: %v", err)
	}
	if op, _ := f.log(t, "org-a")[0].Trailer("Hedgehug-Op"); op != "move-out" {
		t.Errorf("last op of the source = %s", op)
	}
}

func TestMoveStoreRefusesWhatIsInTheDestination(t *testing.T) {
	f := newMoveFx(t)
	bss := f.ts.ForStore("org-b")
	bs, ix, err := bss.Open()
	if err != nil {
		t.Fatal(err)
	}
	// the project is in org-b, with no move-in to say why
	stray := &model.Task{ULID: f.p, Title: "stray", Kind: "project", Status: "todo", Workspace: ix.Workspaces()[0].ULID}
	if _, err := bss.Update(bs, store.Op{Name: "add", Task: f.p}, func(tx *store.Tx) error { return tx.PutTask(stray) }); err != nil {
		t.Fatal(err)
	}
	hs, hd := f.head(t, "org-a"), f.head(t, "org-b")
	_, err = f.ts.MoveStore(f.in())
	mustCode(t, err, KindConflict, "move.exists")
	e := argsOf(err)
	if !strings.Contains(err.Error(), f.p.String()+" is already in org-b") {
		t.Errorf("err = %v (%v)", err, e)
	}
	if f.head(t, "org-a") != hs || f.head(t, "org-b") != hd {
		t.Error("a store was written")
	}
}

func TestMoveStoreSameStoreAndReadOnly(t *testing.T) {
	f := newMoveFx(t)
	in := f.in()
	in.To = "org-a"
	_, err := f.ts.MoveStore(in)
	mustCode(t, err, KindUsage, "move.same_store")
	e := argsOf(err)
	if !strings.Contains(err.Error(), "hedgehug mv --workspace") {
		t.Errorf("err = %v (%v)", err, e)
	}
	_, err = f.ts.MoveStore(MoveStoreInput{Arg: "API-2", To: "nowhere", Yes: true})
	mustCode(t, err, KindNotFound, "store.not_found")

	for _, name := range []string{"org-a", "org-b"} {
		s, err := f.ts.ForStore(name).OpenStore()
		if err != nil {
			t.Fatal(err)
		}
		if err := sync.WriteAccess(s, sync.Access{ReadOnly: true, Reason: sync.ReasonNotWritable, Checked: f.ts.Now()}); err != nil {
			t.Fatal(err)
		}
		hs, hd := f.head(t, "org-a"), f.head(t, "org-b")
		_, err = f.ts.MoveStore(f.in())
		mustCode(t, err, KindConflict, "store.read_only")
		_, err = f.ts.PrepareMoveStore(f.in())
		mustCode(t, err, KindConflict, "store.read_only")
		if f.head(t, "org-a") != hs || f.head(t, "org-b") != hd {
			t.Errorf("%s read-only: a store was written", name)
		}
		if err := sync.WriteAccess(s, sync.Access{Checked: f.ts.Now()}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMoveStoreQualifiedSource(t *testing.T) {
	f := newMoveFx(t)
	in := f.in()
	in.Arg = "org-a:API-2"
	if _, err := f.ts.MoveStore(in); err != nil {
		t.Fatal(err)
	}
}

func TestMoveStoreBothWaysAtOnce(t *testing.T) {
	f := newMoveFx(t)
	b, err := f.ts.ForStore("org-b").Add(AddInput{Title: "from b"})
	if err != nil {
		t.Fatal(err)
	}
	var wg gosync.WaitGroup
	errs := make([]error, 2)
	for i, in := range []MoveStoreInput{
		{Arg: "API-2", To: "org-b", Yes: true},
		{Arg: "WEB-2", To: "org-a", Yes: true},
	} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = f.ts.MoveStore(in)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("move %d: %v", i, err)
		}
	}
	as, _ := f.ts.ForStore("org-a").OpenStore()
	if nt, err := as.Task(b.ULID); err != nil || nt.ID == "" {
		t.Errorf("the task from b: %v", err)
	}
	bs, _ := f.ts.ForStore("org-b").OpenStore()
	if _, err := bs.Task(f.p); err != nil {
		t.Errorf("P in org-b: %v", err)
	}
}

func TestMoveStoreHidden(t *testing.T) {
	f := newMoveFx(t)
	p, err := f.ts.PrepareMoveStore(f.in())
	if err != nil {
		t.Fatal(err)
	}
	if p.Hidden != nil {
		t.Errorf("hidden = %v without member lists", p.Hidden)
	}
	set := func(name string, ids ...string) {
		s, err := f.ts.ForStore(name).OpenStore()
		if err != nil {
			t.Fatal(err)
		}
		cs := []sync.Collaborator{}
		for _, id := range ids {
			cs = append(cs, sync.Collaborator{ID: id, Permission: "write"})
		}
		if err := sync.WriteMembers(s, sync.Members{Collaborators: cs}); err != nil {
			t.Fatal(err)
		}
	}
	set("org-a", "github.com/alice", "github.com/bob", "github.com/Carol")
	p, _ = f.ts.PrepareMoveStore(f.in())
	if p.Hidden != nil {
		t.Errorf("hidden = %v when the destination's list is unknown", p.Hidden)
	}
	set("org-b", "github.com/alice", "github.com/carol")
	p, err = f.ts.PrepareMoveStore(f.in())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.Hidden, []string{"github.com/bob"}) {
		t.Errorf("hidden = %v", p.Hidden)
	}
}

func TestMoveStoreUnlinksSprintAndReleases(t *testing.T) {
	f := newMoveFx(t)
	sp := &model.Sprint{ULID: model.NewULID(), Name: "Sprint 1"}
	as, _ := f.ts.ForStore("org-a").OpenStore()
	_, err := f.ts.ForStore("org-a").Update(as, store.Op{Name: "sprint"}, func(tx *store.Tx) error {
		if err := tx.PutSprint(sp); err != nil {
			return err
		}
		c, err := tx.Task(f.c)
		if err != nil {
			return err
		}
		c.Sprint = &sp.ULID
		return tx.PutTask(c)
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.ts.PrepareMoveStore(f.in())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(p.UnlinkLines(), "unlink: API-3 sprint Sprint 1") {
		t.Errorf("lines = %q", p.UnlinkLines())
	}
	if _, err := f.ts.MoveStore(f.in()); err != nil {
		t.Fatal(err)
	}
	bs, _ := f.ts.ForStore("org-b").OpenStore()
	if c, _ := bs.Task(f.c); c.Sprint != nil {
		t.Errorf("sprint = %v in the destination", c.Sprint)
	}
}

func TestMoveStoreParentsComeBeforeTheirChildren(t *testing.T) {
	ts := newTwoStoreSession(t)
	a := ts.ForStore("org-a")
	add := func(title, parent string) model.ULID {
		r, err := a.Add(AddInput{Title: title, Parent: parent})
		if err != nil {
			t.Fatal(err)
		}
		return r.ULID
	}
	root := add("R", "")
	g := add("G", "API-2") // older than C
	c := add("C", "API-2")
	if _, err := a.Move(MoveInput{Arg: "API-3", Parent: "API-4"}); err != nil {
		t.Fatal(err)
	}
	if g.String() >= c.String() {
		t.Fatalf("G %s should sort before C %s", g, c)
	}
	res, err := ts.MoveStore(MoveStoreInput{Arg: "API-2", To: "org-b", Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Moved) != 3 || res.Moved[0].ULID != root || res.Moved[1].ULID != c || res.Moved[2].ULID != g {
		t.Errorf("moved = %+v; want R, C, G", res.Moved)
	}
}

func TestMoveStoreWarnsOnceOfAFileThatDoesNotDecode(t *testing.T) {
	f := newMoveFx(t)
	as, _ := f.ts.ForStore("org-a").OpenStore()
	bad := model.NewULID()
	rel := model.TaskPath(bad)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(as.Dir(), filepath.FromSlash(rel))), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(as.Dir(), filepath.FromSlash(rel)), []byte("not a task"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.ts.MoveStore(f.in()); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, w := range f.ts.warns {
		if strings.Contains(w, rel) && strings.Contains(w, "does not decode") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d warnings about %s; want 1 (all: %q)", n, rel, f.ts.warns)
	}
}

// A task that moved away can move back: the move-in replaces the forward
// with the task, which gets a new id of the source, keeps its former ids, and
// the store it left now holds the forward.
func TestMoveStoreBackToTheStoreItLeft(t *testing.T) {
	f := newMoveFx(t)
	ts := f.ts
	if _, err := ts.MoveStore(f.in()); err != nil {
		t.Fatal(err)
	}
	res, err := ts.MoveStore(MoveStoreInput{Arg: "WEB-2", To: "org-a", Yes: true})
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
	p, err := as.Task(f.p)
	if err != nil {
		t.Fatalf("the project is not a task of org-a again: %v", err)
	}
	// API-5 is taken and API-2..4 are recorded as given: the numbers go on.
	if p.ID != "API-6" || !slices.Contains(p.FormerIDs, "API-2") || !slices.Contains(p.FormerIDs, "WEB-2") {
		t.Errorf("id %s former %v", p.ID, p.FormerIDs)
	}
	for _, u := range []model.ULID{f.p, f.c, f.g} {
		if _, err := bs.Task(u); !errors.Is(err, model.ErrForward) {
			t.Errorf("%s in org-b: %v; want a forward", u, err)
		}
	}
}
