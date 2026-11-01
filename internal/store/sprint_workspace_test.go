package store

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// addWorkspace puts a second workspace into the store and returns its ULID.
func addWorkspace(t *testing.T, s *Store) model.ULID {
	t.Helper()
	w := &model.Workspace{ULID: model.NewULID(), Name: "api", IDPrefix: "API"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(w) }); err != nil {
		t.Fatal(err)
	}
	return w.ULID
}

func putSprint(s *Store, sp *model.Sprint) error {
	_, err := s.Update(Op{Name: "sprint", Author: testAuthor}, func(tx *Tx) error { return tx.PutSprint(sp) })
	return err
}

func putRelease(s *Store, r *model.Release) error {
	_, err := s.Update(Op{Name: "release", Author: testAuthor}, func(tx *Tx) error { return tx.PutRelease(r) })
	return err
}

func TestPutSprintNeedsWorkspaceOfStore(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ws := testWorkspace(t, s).ULID
	if err := putSprint(s, &model.Sprint{ULID: model.NewULID(), Name: "S1"}); err == nil {
		t.Error("a sprint without a workspace was accepted")
	}
	if err := putSprint(s, &model.Sprint{ULID: model.NewULID(), Workspace: model.NewULID(), Name: "S1"}); err == nil {
		t.Error("a sprint in a workspace that is not in the store was accepted")
	}
	if err := putSprint(s, &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: "S1"}); err != nil {
		t.Errorf("a sprint in a workspace of the store: %v", err)
	}
}

func TestPutReleaseNeedsWorkspaceOfStore(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ws := testWorkspace(t, s).ULID
	if err := putRelease(s, &model.Release{ULID: model.NewULID(), Name: "v1"}); err == nil {
		t.Error("a release without a workspace was accepted")
	}
	if err := putRelease(s, &model.Release{ULID: model.NewULID(), Workspace: model.NewULID(), Name: "v1"}); err == nil {
		t.Error("a release in a workspace that is not in the store was accepted")
	}
	if err := putRelease(s, &model.Release{ULID: model.NewULID(), Workspace: ws, Name: "v1"}); err != nil {
		t.Errorf("a release in a workspace of the store: %v", err)
	}
}

func TestPutSprintWithWorkspaceOfSameOperation(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	w := &model.Workspace{ULID: model.NewULID(), Name: "api", IDPrefix: "API"}
	sp := &model.Sprint{ULID: model.NewULID(), Workspace: w.ULID, Name: "S1"}
	r := &model.Release{ULID: model.NewULID(), Workspace: w.ULID, Name: "v1"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error {
		for _, err := range []error{tx.PutSprint(sp), tx.PutRelease(r), tx.PutWorkspace(w)} {
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Errorf("a sprint and a release put with their workspace: %v", err)
	}
}

func TestSprintInWorkspaceRemovedInSameOperation(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	other := addWorkspace(t, s)
	sp := &model.Sprint{ULID: model.NewULID(), Workspace: other, Name: "S1"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error {
		if err := tx.PutSprint(sp); err != nil {
			return err
		}
		return tx.RemoveWorkspace(other)
	}); err == nil {
		t.Error("a sprint in a workspace removed in the same operation was accepted")
	}
}

func TestRemoveWorkspaceWithSprintOrRelease(t *testing.T) {
	t.Parallel()
	for _, what := range []string{"sprints", "releases"} {
		s := newTestStore(t)
		other := addWorkspace(t, s)
		var err error
		if what == "sprints" {
			err = putSprint(s, &model.Sprint{ULID: model.NewULID(), Workspace: other, Name: "S1"})
		} else {
			err = putRelease(s, &model.Release{ULID: model.NewULID(), Workspace: other, Name: "v1"})
		}
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveWorkspace(other) })
		if err == nil || !strings.Contains(err.Error(), "still has 1 "+what) {
			t.Errorf("%s: err = %v, want a refusal that says the workspace still has 1 %s", what, err, what)
		}
		if _, err := s.Workspace(other); err != nil {
			t.Errorf("%s: the workspace was removed: %v", what, err)
		}
	}
}

func TestRemoveWorkspaceAfterSprintMovedOutInSameOperation(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	other := addWorkspace(t, s)
	sp := &model.Sprint{ULID: model.NewULID(), Workspace: other, Name: "S1"}
	if err := putSprint(s, sp); err != nil {
		t.Fatal(err)
	}
	home := testWorkspace(t, s).ULID
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error {
		sp.Workspace = home
		if err := tx.PutSprint(sp); err != nil {
			return err
		}
		return tx.RemoveWorkspace(other)
	}); err != nil {
		t.Errorf("a workspace whose sprint left in the same operation: %v", err)
	}
}

func TestTaskSprintAndReleasesMustBeOfItsWorkspace(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	home := testWorkspace(t, s).ULID
	other := addWorkspace(t, s)
	mine := &model.Sprint{ULID: model.NewULID(), Workspace: home, Name: "mine"}
	theirs := &model.Sprint{ULID: model.NewULID(), Workspace: other, Name: "theirs"}
	myRel := &model.Release{ULID: model.NewULID(), Workspace: home, Name: "v1"}
	theirRel := &model.Release{ULID: model.NewULID(), Workspace: other, Name: "v2"}
	for _, err := range []error{putSprint(s, mine), putSprint(s, theirs), putRelease(s, myRel), putRelease(s, theirRel)} {
		if err != nil {
			t.Fatal(err)
		}
	}

	task := newTask("x")
	task.Sprint, task.Releases = &mine.ULID, []model.ULID{myRel.ULID}
	if err := put(s, task); err != nil {
		t.Fatalf("refs in the workspace of the task: %v", err)
	}

	bad := newTask("y")
	bad.Sprint = &theirs.ULID
	wantViolation(t, put(s, bad), bad.ULID, CodeRefWorkspace, "sprint")

	bad = newTask("z")
	bad.Releases = []model.ULID{myRel.ULID, theirRel.ULID}
	wantViolation(t, put(s, bad), bad.ULID, CodeRefWorkspace, "releases")

	gone := model.NewULID()
	bad = newTask("w")
	bad.Sprint = &gone
	wantViolation(t, put(s, bad), bad.ULID, schema.CodeMissingRef, "sprint")

	bad = newTask("v")
	bad.Releases = []model.ULID{gone}
	wantViolation(t, put(s, bad), bad.ULID, schema.CodeMissingRef, "releases")
}

func TestTaskKeepsOldSprintOfOtherWorkspace(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	other := addWorkspace(t, s)
	theirs := &model.Sprint{ULID: model.NewULID(), Workspace: other, Name: "theirs"}
	theirRel := &model.Release{ULID: model.NewULID(), Workspace: other, Name: "v2"}
	if err := putSprint(s, theirs); err != nil {
		t.Fatal(err)
	}
	if err := putRelease(s, theirRel); err != nil {
		t.Fatal(err)
	}
	// A task that a sync left with refs of another workspace.
	task := newTask("x")
	mustWrite(t, s, model.TaskPath(task.ULID), "---\nulid: "+task.ULID.String()+"\nid: TST-1\nworkspace: "+task.Workspace.String()+
		"\nkind: task\ntitle: x\nstatus: todo\nsprint: "+theirs.ULID.String()+"\nreleases: [\""+theirRel.ULID.String()+"\"]\n---\n")
	got, err := s.Task(task.ULID)
	if err != nil {
		t.Fatal(err)
	}
	got.Title = "renamed"
	if err := put(s, got); err != nil {
		t.Errorf("editing a task with old refs of another workspace failed: %v", err)
	}
}

func TestTaskRefsToSprintPutInSameOperation(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	home := testWorkspace(t, s).ULID
	sp := &model.Sprint{ULID: model.NewULID(), Workspace: home, Name: "S1"}
	task := newTask("x")
	task.Sprint = &sp.ULID
	if _, err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		if err := tx.PutTask(task); err != nil {
			return err
		}
		return tx.PutSprint(sp)
	}); err != nil {
		t.Errorf("a task that points at a sprint put in the same operation: %v", err)
	}
}
