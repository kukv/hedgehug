package store

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func TestUpdateRefusesViolation(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("")
	_, err := s.Update(Op{Name: "add", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(task) })
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Task != task.ULID || len(ve.Violations) != 1 || ve.Violations[0].Code != schema.CodeMissingTitle {
		t.Fatalf("err = %v, want a ValidationError for the empty title", err)
	}
	if _, err := os.Stat(s.path(model.TaskPath(task.ULID))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("refused task was written: %v", err)
	}
	if n := len(headLog(t, s.dir)); n != 1 {
		t.Errorf("commits = %d, want 1", n)
	}
}

func TestUpdateAllowsOldViolation(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	addTask(t, s, task)
	// A sync may bring a kind this schema does not know.
	mustWrite(t, s, model.TaskPath(task.ULID), "---\nulid: "+task.ULID.String()+"\nworkspace: "+testWorkspaceULID.String()+"\nkind: nope\ntitle: x\nstatus: todo\n---\n")

	got, err := s.Task(task.ULID)
	if err != nil {
		t.Fatal(err)
	}
	got.Title = "renamed"
	if _, err := s.Update(Op{Name: "update", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(got) }); err != nil {
		t.Errorf("editing a task with an old violation failed: %v", err)
	}
}

func TestUpdateRemoveThenPutIsValidated(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	addTask(t, s, task)

	bad := newTask("")
	bad.ULID = task.ULID
	_, err := s.Update(Op{Name: "update", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(tx.RemoveTask(task.ULID), tx.PutTask(bad))
	})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Violations[0].Code != schema.CodeMissingTitle {
		t.Errorf("err = %v, want a ValidationError for the empty title", err)
	}
	if n := len(headLog(t, s.dir)); n != 2 {
		t.Errorf("commits = %d, want 2", n)
	}
}

func TestUpdateRepairsCorruptTask(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("repaired")
	mustWrite(t, s, model.TaskPath(task.ULID), "not a task file\n")
	if _, err := s.Update(Op{Name: "update", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(task) }); err != nil {
		t.Fatalf("repairing a corrupt task failed: %v", err)
	}
	if got, err := s.Task(task.ULID); err != nil || got.Title != "repaired" {
		t.Errorf("task = %+v, %v", got, err)
	}
}

func TestUpdatePlacement(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	parent := newTask("parent")
	child := newTask("child")
	child.Kind = "bug" // a bug cannot sit under a task
	child.Parent = &parent.ULID
	_, err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(tx.PutTask(parent), tx.PutTask(child))
	})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Task != child.ULID || ve.Violations[0].Code != schema.CodePlacement {
		t.Errorf("err = %v, want a placement violation on the child", err)
	}
}

func TestUpdateParentAndChildTogether(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	parent := newTask("parent")
	parent.Kind = "epic"
	child := newTask("child")
	child.Parent = &parent.ULID
	if _, err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		// The child is put first: the check runs after fn, when both are queued.
		return errors.Join(tx.PutTask(child), tx.PutTask(parent))
	}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateMissingParent(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	child := newTask("child")
	missing := model.NewULID()
	child.Parent = &missing
	_, err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(child) })
	wantViolation(t, err, child.ULID, schema.CodeMissingRef, "parent")
	if _, err := os.Stat(s.path(model.TaskPath(child.ULID))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("refused task was written: %v", err)
	}

	// Moving an existing task under a missing parent is refused too.
	moved := newTask("moved")
	addTask(t, s, moved)
	before, err := os.ReadFile(s.path(model.TaskPath(moved.ULID)))
	if err != nil {
		t.Fatal(err)
	}
	moved.Parent = &missing
	_, err = s.Update(Op{Name: "mv", Task: moved.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(moved) })
	wantViolation(t, err, moved.ULID, schema.CodeMissingRef, "parent")
	if after, err := os.ReadFile(s.path(model.TaskPath(moved.ULID))); err != nil || string(after) != string(before) {
		t.Errorf("refused move was written: %q, %v", after, err)
	}
	if n := len(headLog(t, s.dir)); n != 2 {
		t.Errorf("commits = %d, want 2 (init + add)", n)
	}
}

// purgeAndRename purges gone, then renames task, whose parent or farther
// ancestor gone was.
func purgeAndRename(t *testing.T, s *Store, gone, task *model.Task) error {
	t.Helper()
	if _, err := s.Update(Op{Name: "purge", Task: gone.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.RemoveTask(gone.ULID) }); err != nil {
		t.Fatal(err)
	}
	got, err := s.Task(task.ULID)
	if err != nil {
		t.Fatal(err)
	}
	got.Title = "renamed"
	_, err = s.Update(Op{Name: "update", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(got) })
	return err
}

func TestUpdateUnderPurgedParent(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	parent, child := newTask("parent"), newTask("child")
	child.Parent = &parent.ULID
	addTask(t, s, parent)
	addTask(t, s, child)
	if err := purgeAndRename(t, s, parent, child); err != nil {
		t.Errorf("renaming a task whose parent was purged failed: %v", err)
	}
}

func TestUpdateUnderPurgedGrandparent(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	grand, parent, child := newTask("grand"), newTask("parent"), newTask("child")
	parent.Parent = &grand.ULID
	child.Parent = &parent.ULID
	addTask(t, s, grand)
	addTask(t, s, parent)
	addTask(t, s, child)
	if err := purgeAndRename(t, s, grand, child); err != nil {
		t.Errorf("renaming a task whose grandparent was purged failed: %v", err)
	}
}

func TestUpdateParentCycle(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	a, b := newTask("a"), newTask("b")
	a.Parent, b.Parent = &b.ULID, &a.ULID
	_, err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(tx.PutTask(a), tx.PutTask(b))
	})
	wantViolation(t, err, a.ULID, schema.CodeCycle, "parent")
}

func TestUpdateUsesProjectSchema(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	project := newTask("project")
	project.Kind = "project"
	addTask(t, s, project)

	sc := schema.Default()
	sc.Scoped = append(sc.Scoped, schema.Scoped{
		Where:    "project:" + project.ULID.String(),
		Workflow: &schema.Workflow{Required: schema.Lists{{Key: "in_progress", Values: []string{"assignees"}}}},
	})
	if _, err := s.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(sc) }); err != nil {
		t.Fatal(err)
	}

	inside := newTask("inside")
	inside.Parent = &project.ULID
	inside.Status = "in_progress"
	_, err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(inside) })
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Violations[0].Code != schema.CodeRequired {
		t.Errorf("inside the project: err = %v, want a required violation", err)
	}

	outside := newTask("outside")
	outside.Status = "in_progress"
	if _, err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(outside) }); err != nil {
		t.Errorf("outside the project: %v", err)
	}
}

// refusedMove moves task as fn changes it and expects a violation of code,
// with nothing written.
func refusedMove(t *testing.T, s *Store, task *model.Task, code string, fn func(*model.Task)) {
	t.Helper()
	before, err := os.ReadFile(s.path(model.TaskPath(task.ULID)))
	if err != nil {
		t.Fatal(err)
	}
	n := len(headLog(t, s.dir))
	got, err := s.Task(task.ULID)
	if err != nil {
		t.Fatal(err)
	}
	fn(got)
	_, err = s.Update(Op{Name: "mv", Task: task.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(got) })
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Task != task.ULID || ve.Violations[0].Code != code {
		t.Errorf("err = %v, want a %s violation", err, code)
	}
	if after, err := os.ReadFile(s.path(model.TaskPath(task.ULID))); err != nil || string(after) != string(before) {
		t.Errorf("refused move was written: %q, %v", after, err)
	}
	if m := len(headLog(t, s.dir)); m != n {
		t.Errorf("commits = %d, want %d", m, n)
	}
}

func TestUpdateMoveChecksPlacement(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	parent := newTask("parent")
	bug := newTask("bug")
	bug.Kind = "bug" // fine at the root, not under a task
	addTask(t, s, parent)
	addTask(t, s, bug)
	refusedMove(t, s, bug, schema.CodePlacement, func(b *model.Task) { b.Parent = &parent.ULID })
}

func TestUpdateMoveUsesProjectSchema(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	project := newTask("project")
	project.Kind = "project"
	addTask(t, s, project)
	sc := schema.Default()
	sc.Scoped = append(sc.Scoped, schema.Scoped{
		Where:    "project:" + project.ULID.String(),
		Workflow: &schema.Workflow{Required: schema.Lists{{Key: "in_progress", Values: []string{"assignees"}}}},
	})
	if _, err := s.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(sc) }); err != nil {
		t.Fatal(err)
	}
	task := newTask("task")
	addTask(t, s, task)
	refusedMove(t, s, task, schema.CodeRequired, func(m *model.Task) {
		m.Parent = &project.ULID
		m.Status = "in_progress"
	})
}

func TestValidateWorkspace(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	write := func(task *model.Task) error {
		_, err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(task) })
		return err
	}
	var ve *ValidationError
	if err := write(&model.Task{ULID: model.NewULID(), Kind: "task", Title: "x", Status: "todo"}); !errors.As(err, &ve) || ve.Violations[0].Code != CodeNoWorkspace {
		t.Errorf("no workspace: %v", err)
	}
	if err := write(&model.Task{ULID: model.NewULID(), Workspace: model.NewULID(), Kind: "task", Title: "x", Status: "todo"}); !errors.As(err, &ve) || ve.Violations[0].Code != CodeNoWorkspace {
		t.Errorf("unknown workspace: %v", err)
	}
	api := &model.Workspace{ULID: model.NewULID(), Name: "api", IDPrefix: "API"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(api) }); err != nil {
		t.Fatal(err)
	}
	parent := newTaskIn(ws, "p")
	mustPut(t, s, parent)
	child := newTaskIn(api, "c")
	child.Parent = &parent.ULID
	if err := write(child); !errors.As(err, &ve) || ve.Violations[0].Code != CodeWorkspaceMismatch {
		t.Errorf("mismatch: %v", err)
	}
	dup := &model.Workspace{ULID: model.NewULID(), Name: "other", IDPrefix: "API"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(dup) }); err == nil {
		t.Error("a duplicate prefix is accepted")
	}
	same := &model.Workspace{ULID: model.NewULID(), Name: "test", IDPrefix: "APX"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(same) }); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("a duplicate name: %v", err)
	}
	// A file written by hand may have an upper-case name; the same name in
	// lower case is still the same name.
	upper := &model.Workspace{ULID: model.NewULID(), Name: "Web", IDPrefix: "WEB"}
	b, err := upper.Encode()
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, s, model.WorkspacePath(upper.ULID), string(b))
	lower := &model.Workspace{ULID: model.NewULID(), Name: "web", IDPrefix: "WBX"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(lower) }); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("a duplicate name in another case: %v", err)
	}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveWorkspace(ws.ULID) }); err == nil {
		t.Error("a workspace with tasks is removed")
	}
}
