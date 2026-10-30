package store

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func newTask(title string) *model.Task {
	return &model.Task{ID: model.NewID(), Kind: "task", Title: title, Status: "todo"}
}

func addTask(t *testing.T, s *Store, task *model.Task) {
	t.Helper()
	if err := s.Update(Op{Name: "add", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
		return tx.PutTask(task)
	}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateCommitsOnce(t *testing.T) {
	s := newTestStore(t)
	task := newTask("リリース準備")
	addTask(t, s, task)

	got, err := s.Task(task.ID)
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
	want := "hedgehug: add " + task.ID.String() + " リリース準備\n\nHedgehug-Op: add\nHedgehug-Task: " + task.ID.String() + "\n"
	if log[0].Message != want {
		t.Errorf("message = %q, want %q", log[0].Message, want)
	}
	if !log[0].Author.When.Equal(testNow) {
		t.Errorf("commit time = %v", log[0].Author.When)
	}
}

func TestUpdateSubjectUsesKey(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	task.Key = "WEB-42"
	addTask(t, s, task)
	if msg := headLog(t, s.dir)[0].Message; !strings.HasPrefix(msg, "hedgehug: add WEB-42 x\n") {
		t.Errorf("message = %q", msg)
	}
}

func TestUpdateWithoutTask(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	if err := s.Update(Op{Name: "bulk", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(task) }); err != nil {
		t.Fatal(err)
	}
	if msg := headLog(t, s.dir)[0].Message; msg != "hedgehug: bulk\n\nHedgehug-Op: bulk\n" {
		t.Errorf("message = %q", msg)
	}
}

func TestUpdateNoChange(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	addTask(t, s, task)

	later := testNow.Add(time.Hour)
	s.now = func() time.Time { return later }
	same, err := s.Task(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(Op{Name: "update", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
		return tx.PutTask(same)
	}); err != nil {
		t.Fatal(err)
	}
	if n := len(headLog(t, s.dir)); n != 2 {
		t.Errorf("commits = %d, want 2: an unchanged write must not commit", n)
	}
	got, err := s.Task(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Updated.Equal(testNow) {
		t.Errorf("updated = %v, want it unchanged", got.Updated)
	}

	got.Title = "y"
	if err := s.Update(Op{Name: "update", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
		return tx.PutTask(got)
	}); err != nil {
		t.Fatal(err)
	}
	again, err := s.Task(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Updated.Equal(later) || !again.Created.Equal(testNow) {
		t.Errorf("created, updated = %v, %v; want %v, %v", again.Created, again.Updated, testNow, later)
	}
}

func TestUpdateFnError(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	boom := errors.New("boom")
	err := s.Update(Op{Name: "add", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
		if err := tx.PutTask(task); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if _, err := os.Stat(s.path(model.TaskPath(task.ID))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("task file exists after a failed fn: %v", err)
	}
	if n := len(headLog(t, s.dir)); n != 1 {
		t.Errorf("commits = %d, want 1", n)
	}
}

func TestUpdateCommitFailureRollsBack(t *testing.T) {
	s := newTestStore(t)
	kept := newTask("kept")
	addTask(t, s, kept)
	before, err := os.ReadFile(s.path(model.TaskPath(kept.ID)))
	if err != nil {
		t.Fatal(err)
	}

	boom := errors.New("commit failed")
	s.commit = func(gitrepo.Commit) error { return boom }
	edited, err := s.Task(kept.ID)
	if err != nil {
		t.Fatal(err)
	}
	edited.Title = "changed"
	added := newTask("added")
	err = s.Update(Op{Name: "update", Author: testAuthor}, func(tx *Tx) error {
		if err := tx.PutTask(edited); err != nil {
			return err
		}
		return tx.PutTask(added)
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the commit error", err)
	}
	after, err := os.ReadFile(s.path(model.TaskPath(kept.ID)))
	if err != nil || string(after) != string(before) {
		t.Errorf("edited file was not restored: %q, %v", after, err)
	}
	if _, err := os.Stat(s.path(model.TaskPath(added.ID))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("new file was not removed: %v", err)
	}
}

func TestTxReadsItsWrites(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	if err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		if err := tx.PutTask(task); err != nil {
			return err
		}
		got, err := tx.Task(task.ID)
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
	s := newTestStore(t)
	var saved *Tx
	if err := s.Update(Op{Name: "noop", Author: testAuthor}, func(tx *Tx) error {
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
	s := newTestStore(t)
	s.lockWait = 100 * time.Millisecond
	l, err := acquire(s.path(lockPath), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.release() }()
	err = s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(newTask("x")) })
	if !errors.Is(err, ErrLocked) {
		t.Errorf("err = %v, want ErrLocked", err)
	}
}

func TestUpdateFromTwoStores(t *testing.T) {
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
				errs <- st.Update(Op{Name: "add", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
					return tx.PutTask(task)
				})
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
	s := newTestStore(t)
	if err := s.Update(Op{Author: testAuthor}, func(*Tx) error { return nil }); err == nil {
		t.Error("Update without Op.Name succeeded")
	}
}

func TestPutEveryFileType(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	c := &model.Comment{ID: model.NewID(), Author: "github.com/me", Body: "コメント\n"}
	w := &model.Worklog{ID: model.NewID(), Author: "github.com/me", Body: "作業\n"}
	sp := &model.Sprint{ID: model.NewID(), Name: "S1", State: "planned"}
	r := &model.Release{ID: model.NewID(), Name: "v1", State: "unreleased"}
	tp := &model.Template{Name: "bug"}
	tp.Item.Kind = "bug"

	if err := s.Update(Op{Name: "add", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(
			tx.PutTask(task),
			tx.PutComment(task.ID, c),
			tx.PutWorklog(task.ID, w),
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
	for got, err := range s.Comments(task.ID) {
		if err != nil || got.Body != "コメント\n" || !got.Created.Equal(testNow) {
			t.Errorf("comment = %+v, %v", got, err)
		}
		n++
	}
	if n != 1 {
		t.Errorf("comments = %d", n)
	}
	for got, err := range s.Worklogs(task.ID) {
		if err != nil || !got.Updated.Equal(testNow) {
			t.Errorf("worklog = %+v, %v", got, err)
		}
	}
	if got, err := s.Sprint(sp.ID); err != nil || got.Name != "S1" || !got.Created.Equal(testNow) {
		t.Errorf("sprint = %+v, %v", got, err)
	}
	if got, err := s.Release(r.ID); err != nil || got.Name != "v1" {
		t.Errorf("release = %+v, %v", got, err)
	}
	if got, err := s.Template("bug"); err != nil || !got.Updated.Equal(testNow) {
		t.Errorf("template = %+v, %v", got, err)
	}
}

func TestPutTemplateBadName(t *testing.T) {
	s := newTestStore(t)
	err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		return tx.PutTemplate(&model.Template{Name: "../x"})
	})
	if err == nil {
		t.Error("PutTemplate accepted a name outside templates/")
	}
}

func TestRemoveTask(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	c := &model.Comment{ID: model.NewID(), Author: "github.com/me"}
	w := &model.Worklog{ID: model.NewID(), Author: "github.com/me"}
	if err := s.Update(Op{Name: "add", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(tx.PutTask(task), tx.PutComment(task.ID, c), tx.PutWorklog(task.ID, w))
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.Update(Op{Name: "purge", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
		return tx.RemoveTask(task.ID)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Task(task.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("task still readable: %v", err)
	}
	for got, err := range s.Comments(task.ID) {
		t.Errorf("a comment is left: %+v, %v", got, err)
	}
	log := headLog(t, s.dir)
	if len(log) != 3 {
		t.Fatalf("commits = %d, want 3", len(log))
	}
	if want := "hedgehug: purge " + task.ID.String() + "\n"; !strings.HasPrefix(log[0].Message, want) {
		t.Errorf("message = %q, want prefix %q", log[0].Message, want)
	}
	tree, err := log[0].Tree()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{model.TaskPath(task.ID), model.CommentPath(task.ID, c.ID), model.WorklogPath(task.ID, w.ID)} {
		if _, err := tree.File(p); err == nil {
			t.Errorf("%s is still in the tree", p)
		}
	}
}

func TestPutSchema(t *testing.T) {
	s := newTestStore(t)
	sc := schema.Dev()
	if err := s.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(sc) }); err != nil {
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
	err = s.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(bad) })
	var se *SchemaError
	if !errors.As(err, &se) || len(se.Problems) == 0 {
		t.Errorf("err = %v, want a *SchemaError", err)
	}
}

func TestUpdateRereadsSchema(t *testing.T) {
	a := newTestStore(t)
	b, err := Open(a.dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(schema.Dev()) }); err != nil {
		t.Fatal(err)
	}
	task := newTask("x")
	task.Status = "review"
	if err := b.Update(Op{Name: "add", Task: task.ID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(task) }); err != nil {
		t.Errorf("the other store did not see the new schema: %v", err)
	}
	if _, ok := b.Schema().Status("review"); !ok {
		t.Error("Schema() of the other store was not refreshed")
	}
}

func TestUpdateFailsOnBadSchemaFile(t *testing.T) {
	s := newTestStore(t)
	mustWrite(t, s, schemaPath, "statuses: [\n")
	if err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(newTask("x")) }); err == nil {
		t.Error("Update succeeded with a schema.yaml that does not decode")
	}
}

func TestSchemaWhileUpdating(t *testing.T) {
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
		if err := s.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(sc) }); err != nil {
			t.Error(err)
		}
	}
	close(done)
	wg.Wait()
}

func TestPutRefusesZeroID(t *testing.T) {
	s := newTestStore(t)
	id := model.NewID()
	for _, tc := range []struct {
		name string
		put  func(*Tx) error
	}{
		{"PutComment task", func(tx *Tx) error { return tx.PutComment(model.ID{}, &model.Comment{ID: model.NewID()}) }},
		{"PutComment entry", func(tx *Tx) error { return tx.PutComment(id, &model.Comment{}) }},
		{"PutWorklog task", func(tx *Tx) error { return tx.PutWorklog(model.ID{}, &model.Worklog{ID: model.NewID()}) }},
		{"PutWorklog entry", func(tx *Tx) error { return tx.PutWorklog(id, &model.Worklog{}) }},
		{"PutSprint", func(tx *Tx) error { return tx.PutSprint(&model.Sprint{Name: "S1", State: "planned"}) }},
		{"PutRelease", func(tx *Tx) error { return tx.PutRelease(&model.Release{Name: "v1", State: "unreleased"}) }},
		{"RemoveTask", func(tx *Tx) error { return tx.RemoveTask(model.ID{}) }},
	} {
		if err := s.Update(Op{Name: "add", Author: testAuthor}, tc.put); err == nil {
			t.Errorf("%s accepted a zero ID", tc.name)
		}
	}
	if n := len(headLog(t, s.dir)); n != 1 {
		t.Errorf("commits = %d, want 1", n)
	}
}
