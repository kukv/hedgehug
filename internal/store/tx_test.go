package store

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func newTask(title string) *model.Task {
	return &model.Task{ULID: model.NewULID(), Workspace: testWorkspaceULID, Kind: "task", Title: title, Status: "todo"}
}

func addTask(t *testing.T, s *Store, task *model.Task) {
	t.Helper()
	if _, err := s.Update(Op{Name: "add", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error {
		return tx.PutTask(task)
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPutFile(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	if _, err := s.Update(Op{Name: "setup", Author: testAuthor}, func(tx *Tx) error {
		return tx.PutFile(".gitattributes", []byte("*.yaml merge=hedgehug\n"))
	}); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(s.path(".gitattributes")); err != nil || string(b) != "*.yaml merge=hedgehug\n" {
		t.Errorf(".gitattributes = %q, %v", b, err)
	}
	if files, _ := headLog(t, s.dir)[0].Stats(); len(files) != 1 || files[0].Name != ".gitattributes" {
		t.Errorf("committed %v", files)
	}
	for _, bad := range []string{".hedgehug/local/x", ".hedgehug/local", ".Hedgehug/Local/x", ".git/config", ".GIT/config", "../x", "/x", "a/../../x"} {
		if _, err := s.Update(Op{Name: "x", Author: testAuthor}, func(tx *Tx) error {
			return tx.PutFile(bad, []byte("x"))
		}); err == nil {
			t.Errorf("PutFile(%q) succeeded", bad)
		}
	}
}

func TestUpdateCommitsOnce(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("リリース準備")
	addTask(t, s, task)

	got, err := s.Task(task.ULID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "リリース準備" || !got.Created.Equal(testNow) || !got.Updated.Equal(testNow) {
		t.Errorf("task = %+v", got)
	}
	log := headLog(t, s.dir)
	if len(log) != 2 {
		t.Fatalf("commits = %d, want 2 (init + add)", len(log))
	}
	want := "hedgehug: add TST-1 リリース準備\n\nHedgehug-Op: add\nHedgehug-Task: " + task.ULID.String() + "\n"
	if log[0].Message != want {
		t.Errorf("message = %q, want %q", log[0].Message, want)
	}
	if !log[0].Author.When.Equal(testNow) {
		t.Errorf("commit time = %v", log[0].Author.When)
	}
}

func TestUpdateSubjectUsesID(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	task.ID = "TST-42"
	addTask(t, s, task)
	if msg := headLog(t, s.dir)[0].Message; !strings.HasPrefix(msg, "hedgehug: add TST-42 x\n") {
		t.Errorf("message = %q", msg)
	}
}

func TestUpdateWithoutTask(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	if _, err := s.Update(Op{Name: "bulk", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(task) }); err != nil {
		t.Fatal(err)
	}
	if msg := headLog(t, s.dir)[0].Message; msg != "hedgehug: bulk\n\nHedgehug-Op: bulk\n" {
		t.Errorf("message = %q", msg)
	}
}

func TestUpdateNoChange(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	addTask(t, s, task)

	later := testNow.Add(time.Hour)
	s.now = func() time.Time { return later }
	same, err := s.Task(task.ULID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(Op{Name: "update", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error {
		return tx.PutTask(same)
	}); err != nil {
		t.Fatal(err)
	}
	if n := len(headLog(t, s.dir)); n != 2 {
		t.Errorf("commits = %d, want 2: an unchanged write must not commit", n)
	}
	got, err := s.Task(task.ULID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Updated.Equal(testNow) {
		t.Errorf("updated = %v, want it unchanged", got.Updated)
	}

	got.Title = "y"
	if _, err := s.Update(Op{Name: "update", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error {
		return tx.PutTask(got)
	}); err != nil {
		t.Fatal(err)
	}
	again, err := s.Task(task.ULID)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Updated.Equal(later) || !again.Created.Equal(testNow) {
		t.Errorf("created, updated = %v, %v; want %v, %v", again.Created, again.Updated, testNow, later)
	}
}

func TestUpdateFnError(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	boom := errors.New("boom")
	_, err := s.Update(Op{Name: "add", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error {
		if err := tx.PutTask(task); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if _, err := os.Stat(s.path(model.TaskPath(task.ULID))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("task file exists after a failed fn: %v", err)
	}
	if n := len(headLog(t, s.dir)); n != 1 {
		t.Errorf("commits = %d, want 1", n)
	}
}

func TestUpdateCommitFailureRollsBack(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	kept := newTask("kept")
	addTask(t, s, kept)
	before, err := os.ReadFile(s.path(model.TaskPath(kept.ULID)))
	if err != nil {
		t.Fatal(err)
	}

	boom := errors.New("commit failed")
	s.commit = func(gitrepo.Commit) (plumbing.Hash, error) { return plumbing.ZeroHash, boom }
	edited, err := s.Task(kept.ULID)
	if err != nil {
		t.Fatal(err)
	}
	edited.Title = "changed"
	added := newTask("added")
	_, err = s.Update(Op{Name: "update", Author: testAuthor}, func(tx *Tx) error {
		if err := tx.PutTask(edited); err != nil {
			return err
		}
		return tx.PutTask(added)
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the commit error", err)
	}
	after, err := os.ReadFile(s.path(model.TaskPath(kept.ULID)))
	if err != nil || string(after) != string(before) {
		t.Errorf("edited file was not restored: %q, %v", after, err)
	}
	if _, err := os.Stat(s.path(model.TaskPath(added.ULID))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("new file was not removed: %v", err)
	}
}

func TestTxReadsItsWrites(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	if _, err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		if err := tx.PutTask(task); err != nil {
			return err
		}
		got, err := tx.Task(task.ULID)
		if err != nil {
			return err
		}
		if got.Title != "x" {
			t.Errorf("tx.Task = %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestTxUnusableAfterUpdate(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	var saved *Tx
	if _, err := s.Update(Op{Name: "noop", Author: testAuthor}, func(tx *Tx) error {
		saved = tx
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := saved.PutTask(newTask("late")); !errors.Is(err, errTxDone) {
		t.Errorf("err = %v, want errTxDone", err)
	}
}

func TestUpdateLockedElsewhere(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	s.lockWait = 100 * time.Millisecond
	l, err := acquire(s.path(lockPath), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.release() }()
	_, err = s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(newTask("x")) })
	if !errors.Is(err, ErrLocked) {
		t.Errorf("err = %v, want ErrLocked", err)
	}
}

func TestUpdateFromTwoStores(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	other, err := Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	const perStore = 5
	var wg sync.WaitGroup
	errs := make(chan error, 2*perStore)
	for _, st := range []*Store{s, other} {
		wg.Go(func() {
			for range perStore {
				task := newTask("x")
				_, err := st.Update(Op{Name: "add", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error {
					return tx.PutTask(task)
				})
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := len(headLog(t, s.dir)); n != 1+2*perStore {
		t.Errorf("commits = %d, want %d", n, 1+2*perStore)
	}
	n := 0
	for _, err := range s.Tasks() {
		if err != nil {
			t.Fatal(err)
		}
		n++
	}
	if n != 2*perStore {
		t.Errorf("tasks = %d, want %d", n, 2*perStore)
	}
}

func TestUpdateNeedsOpName(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	if _, err := s.Update(Op{Author: testAuthor}, func(*Tx) error { return nil }); err == nil {
		t.Error("Update without Op.Name succeeded")
	}
}

func TestPutEveryFileType(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	c := &model.Comment{ULID: model.NewULID(), Author: "github.com/me", Body: "コメント\n"}
	w := &model.Worklog{ULID: model.NewULID(), Author: "github.com/me", Body: "作業\n"}
	sp := &model.Sprint{ULID: model.NewULID(), Workspace: testWorkspaceULID, Name: "S1", State: "planned"}
	r := &model.Release{ULID: model.NewULID(), Workspace: testWorkspaceULID, Name: "v1", State: "unreleased"}
	tp := &model.Template{Name: "bug"}
	tp.Item.Kind = "bug"

	if _, err := s.Update(Op{Name: "add", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(
			tx.PutTask(task),
			tx.PutComment(task.ULID, c),
			tx.PutWorklog(task.ULID, w),
			tx.PutSprint(sp),
			tx.PutRelease(r),
			tx.PutTemplate(tp),
		)
	}); err != nil {
		t.Fatal(err)
	}
	if n := len(headLog(t, s.dir)); n != 2 {
		t.Errorf("commits = %d, want 2", n)
	}

	n := 0
	for got, err := range s.Comments(task.ULID) {
		if err != nil || got.Body != "コメント\n" || !got.Created.Equal(testNow) {
			t.Errorf("comment = %+v, %v", got, err)
		}
		n++
	}
	if n != 1 {
		t.Errorf("comments = %d", n)
	}
	for got, err := range s.Worklogs(task.ULID) {
		if err != nil || !got.Updated.Equal(testNow) {
			t.Errorf("worklog = %+v, %v", got, err)
		}
	}
	if got, err := s.Sprint(sp.ULID); err != nil || got.Name != "S1" || !got.Created.Equal(testNow) {
		t.Errorf("sprint = %+v, %v", got, err)
	}
	if got, err := s.Release(r.ULID); err != nil || got.Name != "v1" {
		t.Errorf("release = %+v, %v", got, err)
	}
	if got, err := s.Template("bug"); err != nil || !got.Updated.Equal(testNow) {
		t.Errorf("template = %+v, %v", got, err)
	}
}

func TestPutTemplateBadName(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	_, err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		return tx.PutTemplate(&model.Template{Name: "../x"})
	})
	if err == nil {
		t.Error("PutTemplate accepted a name outside templates/")
	}
}

func TestPurgeSubjectUsesID(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("title")
	task.ID = "TST-3"
	addTask(t, s, task)
	if _, err := s.Update(Op{Name: "purge", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error {
		return tx.RemoveTask(task.ULID)
	}); err != nil {
		t.Fatal(err)
	}
	if msg := headLog(t, s.dir)[0].Message; !strings.HasPrefix(msg, "hedgehug: purge TST-3 title\n") {
		t.Errorf("message = %q", msg)
	}
}

func TestRemoveTask(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	c := &model.Comment{ULID: model.NewULID(), Author: "github.com/me"}
	w := &model.Worklog{ULID: model.NewULID(), Author: "github.com/me"}
	if _, err := s.Update(Op{Name: "add", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(tx.PutTask(task), tx.PutComment(task.ULID, c), tx.PutWorklog(task.ULID, w))
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Update(Op{Name: "purge", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error {
		return tx.RemoveTask(task.ULID)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Task(task.ULID); !errors.Is(err, ErrNotFound) {
		t.Errorf("task still readable: %v", err)
	}
	for got, err := range s.Comments(task.ULID) {
		t.Errorf("a comment is left: %+v, %v", got, err)
	}
	log := headLog(t, s.dir)
	if len(log) != 3 {
		t.Fatalf("commits = %d, want 3", len(log))
	}
	if want := "hedgehug: purge TST-1 x\n"; !strings.HasPrefix(log[0].Message, want) {
		t.Errorf("message = %q, want prefix %q", log[0].Message, want)
	}
	tree, err := log[0].Tree()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{model.TaskPath(task.ULID), model.CommentPath(task.ULID, c.ULID), model.WorklogPath(task.ULID, w.ULID)} {
		if _, err := tree.File(p); err == nil {
			t.Errorf("%s is still in the tree", p)
		}
	}
}

func TestPutSchema(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	sc := schema.Dev()
	if _, err := s.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(sc) }); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Schema().Status("review"); !ok {
		t.Error("Schema() was not replaced")
	}
	o, err := Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := o.Schema().Status("review"); !ok {
		t.Error("schema.yaml was not written")
	}

	bad := schema.Default()
	bad.Statuses = append(bad.Statuses, bad.Statuses[0]) // a duplicate name
	_, err = s.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(bad) })
	var se *SchemaError
	if !errors.As(err, &se) || len(se.Problems) == 0 {
		t.Errorf("err = %v, want a *SchemaError", err)
	}
}

func TestUpdateRereadsSchema(t *testing.T) {
	t.Parallel()
	a := newTestStore(t)
	b, err := Open(a.dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(schema.Dev()) }); err != nil {
		t.Fatal(err)
	}
	task := newTask("x")
	task.Status = "review"
	if _, err := b.Update(Op{Name: "add", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(task) }); err != nil {
		t.Errorf("the other store did not see the new schema: %v", err)
	}
	if _, ok := b.Schema().Status("review"); !ok {
		t.Error("Schema() of the other store was not refreshed")
	}
}

func TestUpdateFailsOnBadSchemaFile(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	mustWrite(t, s, schemaPath, "statuses: [\n")
	if _, err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(newTask("x")) }); err == nil {
		t.Error("Update succeeded with a schema.yaml that does not decode")
	}
}

func TestSchemaWhileUpdating(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-done:
				return
			default:
				_ = s.Schema()
			}
		}
	})
	for _, sc := range []*schema.Schema{schema.Dev(), schema.Default()} {
		if _, err := s.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(sc) }); err != nil {
			t.Error(err)
		}
	}
	close(done)
	wg.Wait()
}

func TestPutRefusesZeroULID(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ulid := model.NewULID()
	for _, tc := range []struct {
		name string
		put  func(*Tx) error
	}{
		{"PutComment task", func(tx *Tx) error { return tx.PutComment(model.ULID{}, &model.Comment{ULID: model.NewULID()}) }},
		{"PutComment entry", func(tx *Tx) error { return tx.PutComment(ulid, &model.Comment{}) }},
		{"PutWorklog task", func(tx *Tx) error { return tx.PutWorklog(model.ULID{}, &model.Worklog{ULID: model.NewULID()}) }},
		{"PutWorklog entry", func(tx *Tx) error { return tx.PutWorklog(ulid, &model.Worklog{}) }},
		{"PutSprint", func(tx *Tx) error { return tx.PutSprint(&model.Sprint{Name: "S1", State: "planned"}) }},
		{"PutRelease", func(tx *Tx) error { return tx.PutRelease(&model.Release{Name: "v1", State: "unreleased"}) }},
		{"RemoveTask", func(tx *Tx) error { return tx.RemoveTask(model.ULID{}) }},
	} {
		if _, err := s.Update(Op{Name: "add", Author: testAuthor}, tc.put); err == nil {
			t.Errorf("%s accepted a zero ULID", tc.name)
		}
	}
	if n := len(headLog(t, s.dir)); n != 1 {
		t.Errorf("commits = %d, want 1", n)
	}
}
