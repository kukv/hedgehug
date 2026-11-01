package store

import (
	"cmp"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// newMoveStore makes a store with the id prefix prefix and the schema sc
// (the default when nil). Stores made later have larger ULIDs.
func newMoveStore(t *testing.T, name, prefix string, sc *schema.Schema) (*Store, *model.Workspace) {
	t.Helper()
	ws := &model.Workspace{ULID: model.NewULID(), Name: name, IDPrefix: prefix}
	meta := &model.StoreMeta{ULID: model.NewULID(), Name: name}
	s, err := Create(filepath.Join(t.TempDir(), name), meta, sc, ws, testAuthor, "", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return testNow }
	return s, ws
}

func lastCommit(t *testing.T, s *Store) gitrepo.LogEntry {
	t.Helper()
	es, err := s.repo.Log(nil, 1)
	if err != nil || len(es) != 1 {
		t.Fatalf("log = %v, %v", es, err)
	}
	return es[0]
}

var scopedStatuses = []string{"todo", "in_progress", "done", "canceled"}

func compareUnlink(a, b Unlink) int {
	return cmp.Or(
		strings.Compare(a.Task.String(), b.Task.String()),
		strings.Compare(a.Field, b.Field),
		strings.Compare(a.Target.String(), b.Target.String()),
	)
}

func TestUnlinks(t *testing.T) {
	sc := schema.Default()
	a, b, c, x, y := newTask("a"), newTask("b"), newTask("c"), newTask("x"), newTask("y")
	a.BlockedBy = []model.ULID{b.ULID, x.ULID} // one inside M, one out
	b.BlockedBy = []model.ULID{x.ULID, x.ULID}
	c.Props.Set("relates", ulidList(y.ULID, a.ULID))
	x.BlockedBy = []model.ULID{a.ULID} // from the rest into M
	y.Props.Set("duplicates", ulidList(c.ULID))
	y.Props.Set("priority", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "high"})
	moved, rest := []*model.Task{a, b, c}, []*model.Task{x, y}
	got := Unlinks(sc, moved, rest)
	want := []Unlink{
		{a.ULID, "blocked_by", x.ULID},
		{b.ULID, "blocked_by", x.ULID},
		{c.ULID, "props.relates", y.ULID},
		{x.ULID, "blocked_by", a.ULID},
		{y.ULID, "props.duplicates", c.ULID},
	}
	slices.SortFunc(want, compareUnlink)
	if !slices.Equal(got, want) {
		t.Errorf("Unlinks =\n%v\nwant\n%v", got, want)
	}
	if s := got[0].String(); s != got[0].Task.String()+" "+got[0].Field+" "+got[0].Target.String() {
		t.Errorf("String = %q", s)
	}
	if n := len(Unlinks(sc, moved, nil)); n != 0 {
		t.Errorf("no rest gave %d unlinks", n)
	}
}

// moveFixture is a source store with a tree P > C > G (P blocked by an
// outsider, C blocked by G, G related to the outsider, the outsider blocked
// by G) and a comment and a worklog entry on C.
type moveFixture struct {
	src, dst     *Store
	srcWS, dstWS *model.Workspace
	p, c, g, out *model.Task
	comment      model.ULID
	notes        map[string][]byte
	moved        []*model.Task
	unlinked     []Unlink
	commentPath  string
	worklogPath  string
}

func newMoveFixture(t *testing.T, srcPrefix, dstPrefix string, dstSchema *schema.Schema) *moveFixture {
	t.Helper()
	f := &moveFixture{}
	f.src, f.srcWS = newMoveStore(t, "src", srcPrefix, nil)
	f.dst, f.dstWS = newMoveStore(t, "dst", dstPrefix, dstSchema)
	f.out = newTaskIn(f.srcWS, "outsider")
	f.p = newTaskIn(f.srcWS, "P")
	f.c = newTaskIn(f.srcWS, "C")
	f.g = newTaskIn(f.srcWS, "G")
	f.out.Rank, f.p.Rank, f.c.Rank, f.g.Rank = "a0", "a1", "a0", "a0"
	f.c.Parent, f.g.Parent = &f.p.ULID, &f.c.ULID
	f.p.BlockedBy = []model.ULID{f.out.ULID}
	f.c.BlockedBy = []model.ULID{f.g.ULID}
	f.g.Props.Set("relates", ulidList(f.out.ULID))
	mustPut(t, f.src, f.out, f.p, f.c, f.g)
	f.out.BlockedBy = []model.ULID{f.g.ULID}
	mustPut(t, f.src, f.out)

	cm := &model.Comment{ULID: model.NewULID(), Author: "github.com/a", Body: "hello"}
	wl := &model.Worklog{ULID: model.NewULID(), Author: "github.com/a", Body: "work"}
	f.comment = cm.ULID
	if _, err := f.src.Update(Op{Name: "comment", Task: f.c.ULID, Author: testAuthor}, func(tx *Tx) error {
		if err := tx.PutComment(f.c.ULID, cm); err != nil {
			return err
		}
		return tx.PutWorklog(f.c.ULID, wl)
	}); err != nil {
		t.Fatal(err)
	}
	f.commentPath, f.worklogPath = model.CommentPath(f.c.ULID, cm.ULID), model.WorklogPath(f.c.ULID, wl.ULID)
	f.notes = map[string][]byte{}
	for _, rel := range []string{f.commentPath, f.worklogPath} {
		b, err := os.ReadFile(f.src.path(rel))
		if err != nil {
			t.Fatal(err)
		}
		f.notes[rel] = b
	}
	var err error
	for _, p := range []**model.Task{&f.p, &f.c, &f.g, &f.out} {
		if *p, err = f.src.Task((*p).ULID); err != nil {
			t.Fatal(err)
		}
	}
	f.moved = []*model.Task{f.p, f.c, f.g}
	f.unlinked = Unlinks(f.src.Schema(), f.moved, []*model.Task{f.out})
	if len(f.unlinked) != 3 { // P to the outsider, G to the outsider, the outsider to G
		t.Fatalf("unlinked = %v", f.unlinked)
	}
	return f
}

func (f *moveFixture) in() MoveIn {
	return MoveIn{
		From: f.src.Meta().ULID, Tasks: f.moved, Workspace: f.dstWS.ULID,
		Notes: f.notes, Unlinked: f.unlinked,
	}
}

func (f *moveFixture) moveIn(in MoveIn) error {
	_, err := f.dst.Update(Op{Name: "move-in", Task: f.p.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.MoveIn(in) })
	return err
}

func (f *moveFixture) moveOut(scoped string) error {
	_, err := f.src.Update(Op{Name: "move-out", Author: testAuthor}, func(tx *Tx) error {
		return tx.MoveOut(MoveOut{
			To: f.dst.Meta().ULID, Tasks: []model.ULID{f.p.ULID, f.c.ULID, f.g.ULID},
			Unlinked: f.unlinked, Scoped: scoped,
		})
	})
	return err
}

func (f *moveFixture) restSorted() []string {
	rest := []string{f.c.ULID.String(), f.g.ULID.String()}
	slices.Sort(rest)
	return rest
}

func TestMoveIn(t *testing.T) {
	f := newMoveFixture(t, "SRC", "DST", nil)
	sib := newTaskIn(f.dstWS, "sibling") // DST-1; the root goes after it
	sib.Rank = "a5"
	mustPut(t, f.dst, sib)
	oldID := f.p.ID
	if err := f.moveIn(f.in()); err != nil {
		t.Fatal(err)
	}
	if f.p.ID != oldID || f.p.Workspace != f.srcWS.ULID {
		t.Errorf("the caller's task changed: %s %s", f.p.ID, f.p.Workspace)
	}

	p, err := f.dst.Task(f.p.ULID)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := f.dst.Task(f.c.ULID)
	g, _ := f.dst.Task(f.g.ULID)
	if p.ID != "DST-2" || c.ID != "DST-3" || g.ID != "DST-4" {
		t.Errorf("ids = %s %s %s", p.ID, c.ID, g.ID)
	}
	if !slices.Equal(p.FormerIDs, []string{oldID}) {
		t.Errorf("former_ids = %v, want [%s]", p.FormerIDs, oldID)
	}
	for _, x := range []*model.Task{p, c, g} {
		if x.Workspace != f.dstWS.ULID || !x.Updated.Equal(testNow) {
			t.Errorf("%s: workspace %s, updated %v", x.ID, x.Workspace, x.Updated)
		}
	}
	if p.Parent != nil || p.Rank <= sib.Rank {
		t.Errorf("root: parent %v, rank %q; want the top level after %q", p.Parent, p.Rank, sib.Rank)
	}
	if c.Parent == nil || *c.Parent != p.ULID || *g.Parent != c.ULID {
		t.Error("the tree is not kept")
	}
	if len(p.BlockedBy) != 0 {
		t.Errorf("P blocked_by = %v, want none (the outsider is not here)", p.BlockedBy)
	}
	if !slices.Equal(c.BlockedBy, []model.ULID{g.ULID}) {
		t.Errorf("C blocked_by = %v, want [G]", c.BlockedBy)
	}
	if g.Props.Get("relates") != nil {
		t.Error("G still relates to the outsider")
	}
	for rel, want := range f.notes {
		if got, err := os.ReadFile(f.dst.path(rel)); err != nil || string(got) != string(want) {
			t.Errorf("%s = %q, %v", rel, got, err)
		}
	}

	e := lastCommit(t, f.dst)
	if e.Subject != "hedgehug: move-in DST-2 P" {
		t.Errorf("subject = %q", e.Subject)
	}
	if v, _ := e.Trailer("Hedgehug-Op"); v != "move-in" {
		t.Errorf("op = %q", v)
	}
	if v, _ := e.Trailer("Hedgehug-Moved-From"); v != f.src.Meta().ULID.String() {
		t.Errorf("moved-from = %q", v)
	}
	if got := e.TrailerValues("Hedgehug-Task"); !slices.Equal(got, append([]string{f.p.ULID.String()}, f.restSorted()...)) {
		t.Errorf("tasks = %v", got)
	}
	var wantUn []string
	for _, u := range f.unlinked {
		wantUn = append(wantUn, u.String())
	}
	if got := e.TrailerValues("Hedgehug-Unlinked"); !slices.Equal(got, wantUn) {
		t.Errorf("unlinked = %v, want %v", got, wantUn)
	}
}

// Without Op.Task the root's Hedgehug-Task is added by MoveIn.
func TestMoveInNamesTheRootWhenTheOpDoesNot(t *testing.T) {
	f := newMoveFixture(t, "SRC", "DST", nil)
	if _, err := f.dst.Update(Op{Name: "move-in", Detail: "x", Author: testAuthor}, func(tx *Tx) error { return tx.MoveIn(f.in()) }); err != nil {
		t.Fatal(err)
	}
	if got := lastCommit(t, f.dst).TrailerValues("Hedgehug-Task"); !slices.Equal(got, append([]string{f.p.ULID.String()}, f.restSorted()...)) {
		t.Errorf("tasks = %v", got)
	}
}

func TestMoveInUnderAParent(t *testing.T) {
	f := newMoveFixture(t, "SRC", "DST", nil)
	parent := newTaskIn(f.dstWS, "parent")
	mustPut(t, f.dst, parent)
	in := f.in()
	in.Parent = &parent.ULID
	if err := f.moveIn(in); err != nil {
		t.Fatal(err)
	}
	p, _ := f.dst.Task(f.p.ULID)
	if p.Parent == nil || *p.Parent != parent.ULID {
		t.Errorf("parent = %v", p.Parent)
	}
}

func TestMoveInSamePrefixGetsNewNumbers(t *testing.T) {
	f := newMoveFixture(t, "ABC", "ABC", nil)
	mustPut(t, f.dst, newTaskIn(f.dstWS, "early"), newTaskIn(f.dstWS, "early2"), newTaskIn(f.dstWS, "early3"))
	if f.p.ID != "ABC-2" || f.c.ID != "ABC-3" {
		t.Fatalf("source ids: %s %s", f.p.ID, f.c.ID)
	}
	if err := f.moveIn(f.in()); err != nil {
		t.Fatal(err)
	}
	p, _ := f.dst.Task(f.p.ULID)
	c, _ := f.dst.Task(f.c.ULID)
	g, _ := f.dst.Task(f.g.ULID)
	// The former ids of the tasks (ABC-2..4) count, as in any store.
	if p.ID != "ABC-5" || c.ID != "ABC-6" || g.ID != "ABC-7" {
		t.Errorf("ids = %s %s %s, want new numbers ABC-5..7", p.ID, c.ID, g.ID)
	}
	if !slices.Equal(p.FormerIDs, []string{"ABC-2"}) || !slices.Equal(c.FormerIDs, []string{"ABC-3"}) {
		t.Errorf("former_ids = %v %v", p.FormerIDs, c.FormerIDs)
	}
}

func TestMoveInKeepsFormerIDs(t *testing.T) {
	f := newMoveFixture(t, "SRC", "DST", nil)
	f.p.FormerIDs = []string{"OLD-9"}
	if err := f.moveIn(f.in()); err != nil {
		t.Fatal(err)
	}
	p, _ := f.dst.Task(f.p.ULID)
	if !slices.Equal(p.FormerIDs, []string{"OLD-9", "SRC-2"}) {
		t.Errorf("former_ids = %v", p.FormerIDs)
	}
}

func TestMoveInRefusesWhatTheSchemaDoes(t *testing.T) {
	sc := schema.Default()
	sc.Kinds = slices.DeleteFunc(sc.Kinds, func(k schema.Kind) bool { return k.Name == "bug" })
	f := newMoveFixture(t, "SRC", "DST", sc)
	f.g.Kind = "bug"
	err := f.moveIn(f.in())
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Task != f.g.ULID {
		t.Fatalf("err = %v, want a *ValidationError for G", err)
	}
	if _, err := f.dst.Task(f.p.ULID); !errors.Is(err, ErrNotFound) {
		t.Errorf("P was written: %v", err)
	}
}

func TestMoveInScoped(t *testing.T) {
	f := newMoveFixture(t, "SRC", "DST", nil)
	in := f.in()
	in.Scoped = &schema.Scoped{Where: "project:" + f.p.ULID.String(), Statuses: scopedStatuses}
	if err := f.moveIn(in); err != nil {
		t.Fatal(err)
	}
	if sc := f.dst.Schema(); len(sc.Scoped) != 1 || sc.Scoped[0].Where != in.Scoped.Where {
		t.Errorf("scoped = %+v", sc.Scoped)
	}
	if err := f.moveIn(in); err != nil { // a rerun replaces the entry
		t.Fatal(err)
	}
	if n := len(f.dst.Schema().Scoped); n != 1 {
		t.Errorf("%d scoped entries after a rerun", n)
	}
}

func TestMoveInRefusesBadInput(t *testing.T) {
	f := newMoveFixture(t, "SRC", "DST", nil)
	in := f.in()
	in.Tasks = []*model.Task{f.p, f.g} // G's parent C is not moved
	if err := f.moveIn(in); err == nil {
		t.Error("a task without its parent was accepted")
	}
	in = f.in()
	in.Notes = map[string][]byte{model.CommentPath(f.out.ULID, f.comment): []byte("x")}
	if err := f.moveIn(in); err == nil {
		t.Error("a note of a task that stays was accepted")
	}
}

func TestMoveOut(t *testing.T) {
	f := newMoveFixture(t, "SRC", "DST", nil)
	scoped := "project:" + f.p.ULID.String()
	sc, err := cloneSchema(f.src.Schema())
	if err != nil {
		t.Fatal(err)
	}
	other := "project:" + f.out.ULID.String()
	sc.Scoped = []schema.Scoped{{Where: scoped, Statuses: scopedStatuses}, {Where: other, Statuses: scopedStatuses}}
	if _, err := f.src.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(sc) }); err != nil {
		t.Fatal(err)
	}
	// the outsider also relates to P through a property
	f.out.Props.Set("relates", ulidList(f.p.ULID))
	mustPut(t, f.src, f.out)
	f.out, _ = f.src.Task(f.out.ULID)
	f.unlinked = Unlinks(f.src.Schema(), f.moved, []*model.Task{f.out})

	if err := f.moveOut(scoped); err != nil {
		t.Fatal(err)
	}
	to := f.dst.Meta().ULID
	for _, x := range f.moved {
		if _, err := f.src.Task(x.ULID); !errors.Is(err, model.ErrForward) {
			t.Errorf("Task(%s) err = %v, want a forward", x.Title, err)
		}
		b, _ := os.ReadFile(f.src.path(model.TaskPath(x.ULID)))
		fw, err := model.DecodeForward(b)
		if err != nil || fw.MovedTo != to || fw.ULID != x.ULID {
			t.Errorf("forward of %s = %+v, %v", x.Title, fw, err)
		}
	}
	for _, rel := range []string{f.commentPath, f.worklogPath} {
		if _, err := os.Stat(f.src.path(rel)); err == nil {
			t.Errorf("%s is still there", rel)
		}
	}
	out, err := f.src.Task(f.out.ULID)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.BlockedBy) != 0 || out.Props.Get("relates") != nil {
		t.Errorf("the outsider keeps references: %v %v", out.BlockedBy, out.Props)
	}
	n, err := f.src.Numbers()
	if err != nil {
		t.Fatal(err)
	}
	if n["SRC"] != 4 { // outsider 1, P 2, C 3, G 4
		t.Errorf("numbers = %v, want SRC: 4", n)
	}
	if got := f.src.Schema().Scoped; len(got) != 1 || got[0].Where != other {
		t.Errorf("scoped = %+v, want only %s", got, other)
	}

	e := lastCommit(t, f.src)
	if e.Subject != "hedgehug: move-out SRC-2 P" {
		t.Errorf("subject = %q", e.Subject)
	}
	if v, _ := e.Trailer("Hedgehug-Op"); v != "move-out" {
		t.Errorf("op = %q", v)
	}
	if v, _ := e.Trailer("Hedgehug-Moved-To"); v != to.String() {
		t.Errorf("moved-to = %q", v)
	}
	if got := e.TrailerValues("Hedgehug-Task"); !slices.Equal(got, append([]string{f.p.ULID.String()}, f.restSorted()...)) {
		t.Errorf("tasks = %v", got)
	}
	if got := e.TrailerValues("Hedgehug-Unlinked"); len(got) != len(f.unlinked) || len(got) != 4 {
		t.Errorf("unlinked = %v", got)
	}
}

func TestMoveOutNumbersOnlyOfThisStoresPrefixes(t *testing.T) {
	f := newMoveFixture(t, "SRC", "DST", nil)
	f.c.FormerIDs = []string{"ELSE-77", "SRC-9"}
	mustPut(t, f.src, f.c)
	if err := f.moveOut(""); err != nil {
		t.Fatal(err)
	}
	n, _ := f.src.Numbers()
	if n["SRC"] != 9 || n["ELSE"] != 0 || len(n) != 1 {
		t.Errorf("numbers = %v, want only SRC: 9", n)
	}
}

// The pair a mv does, with both stores locked: refs between the tasks moved
// pass the destination's validation in one Tx, and the source ends with
// forwards.
func TestMoveBothSides(t *testing.T) {
	f := newMoveFixture(t, "SRC", "DST", nil)
	l, err := LockInOrder(f.dst, f.src)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	if err := f.moveIn(f.in()); err != nil {
		t.Fatal(err)
	}
	if err := f.moveOut(""); err != nil {
		t.Fatal(err)
	}
	var live, forwards int
	for _, err := range f.src.Tasks() {
		if err == nil {
			live++
		}
	}
	for _, err := range f.src.Forwards() {
		if err == nil {
			forwards++
		}
	}
	if live != 1 || forwards != 3 {
		t.Errorf("source has %d tasks and %d forwards, want 1 and 3", live, forwards)
	}
}

func TestLockInOrder(t *testing.T) {
	a, _ := newMoveStore(t, "a", "AAA", nil)
	b, _ := newMoveStore(t, "b", "BBB", nil)
	var order []string
	afterLock = func(s *Store) { order = append(order, s.meta.Name) }
	t.Cleanup(func() { afterLock = func(*Store) {} })

	l, err := LockInOrder(b, a, b)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, []string{"a", "b"}) {
		t.Errorf("locked in the order %v, want a then b", order)
	}

	// Another goroutine's Update waits for the lock like another process's.
	done := make(chan error, 1)
	go func() {
		_, err := a.Update(Op{Name: "x", Author: testAuthor}, func(tx *Tx) error { return tx.PutFile("a.txt", []byte("x")) })
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("another goroutine's Update did not wait: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	// The holder is not kept waiting by its own locks.
	for _, s := range []*Store{a, b} {
		if _, err := s.Update(Op{Name: "y", Author: testAuthor}, func(tx *Tx) error { return tx.PutFile("y.txt", []byte("y")) }); err != nil {
			t.Errorf("the holder's Update on %s: %v", s.meta.Name, err)
		}
	}

	l.Release()
	l.Release() // more than once
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("the waiting Update: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the waiting Update did not go on after Release")
	}
	if a.held.Load() != nil || b.held.Load() != nil {
		t.Error("a store is still marked held")
	}
	// After Release the holder takes the lock itself again.
	if _, err := a.Update(Op{Name: "z", Author: testAuthor}, func(tx *Tx) error { return tx.PutFile("z.txt", []byte("z")) }); err != nil {
		t.Error(err)
	}
}

func TestLockInOrderFailureReleasesWhatItTook(t *testing.T) {
	a, _ := newMoveStore(t, "a", "AAA", nil)
	b, _ := newMoveStore(t, "b", "BBB", nil)
	b.lockWait = 50 * time.Millisecond
	other, err := acquire(b.path(lockPath), time.Second) // another process holds b
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LockInOrder(a, b); !errors.Is(err, ErrLocked) {
		t.Fatalf("err = %v, want ErrLocked", err)
	}
	if err := other.release(); err != nil {
		t.Fatal(err)
	}
	if a.held.Load() != nil {
		t.Error("a is still marked held")
	}
	l, err := LockInOrder(a, b) // a was let go, and b is free now
	if err != nil {
		t.Fatal(err)
	}
	l.Release()
}

// A property that lists several tasks loses only the one cut.
func TestMoveInKeepsTheOtherTargetsOfAProperty(t *testing.T) {
	f := newMoveFixture(t, "SRC", "DST", nil)
	f.c.Props.Set("relates", ulidList(f.out.ULID, f.g.ULID))
	mustPut(t, f.src, f.c)
	f.c, _ = f.src.Task(f.c.ULID)
	f.moved = []*model.Task{f.p, f.c, f.g}
	f.unlinked = Unlinks(f.src.Schema(), f.moved, []*model.Task{f.out})
	if err := f.moveIn(f.in()); err != nil {
		t.Fatal(err)
	}
	c, _ := f.dst.Task(f.c.ULID)
	n := c.Props.Get("relates")
	if n == nil || len(n.Content) != 1 || n.Content[0].Value != f.g.ULID.String() {
		t.Errorf("relates = %+v, want only G", n)
	}
	if got := f.c.Props.Get("relates"); len(got.Content) != 2 {
		t.Error("the caller's task changed")
	}
}
