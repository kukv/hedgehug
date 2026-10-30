package store

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
)

func TestCreateWritesWorkspace(t *testing.T) {
	s := newTestStore(t)
	var got []*model.Workspace
	for w, err := range s.Workspaces() {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, w)
	}
	if len(got) != 1 || got[0].KeyPrefix != "TST" || got[0].Name != "test" || got[0].PrefixSet.IsZero() {
		t.Fatalf("workspaces = %+v", got)
	}
}

func TestPutAndRemoveWorkspace(t *testing.T) {
	s := newTestStore(t)
	w := &model.Workspace{ID: model.NewID(), Name: "api", KeyPrefix: "API"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(w) }); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Workspace(w.ID); err != nil || got.KeyPrefix != "API" {
		t.Fatalf("Workspace = %+v %v", got, err)
	}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveWorkspace(w.ID) }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Workspace(w.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("after remove: %v", err)
	}
}

func TestPurgeRecordsNumbers(t *testing.T) {
	s := newTestStore(t)
	task := &model.Task{ID: model.NewID(), Workspace: testWorkspace(t, s).ID, Kind: "task", Title: "x", Status: "todo", Key: "TST-9", FormerKeys: []string{"OLD-12"}}
	if err := put(s, task); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(Op{Name: "purge", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveTask(task.ID) }); err != nil {
		t.Fatal(err)
	}
	n, err := s.Numbers()
	if err != nil || n["TST"] != 9 || n["OLD"] != 12 {
		t.Errorf("Numbers = %v %v", n, err)
	}
}

// subjectAndBody returns the subject of the last commit and its whole message.
func subjectAndBody(t *testing.T, s *Store) (string, string) {
	t.Helper()
	msg := headLog(t, s.dir)[0].Message
	return strings.SplitN(msg, "\n", 2)[0], msg
}

func TestWorkspaceCommitSubject(t *testing.T) {
	s := newTestStore(t)
	w := &model.Workspace{ID: model.NewID(), Name: "api", KeyPrefix: "API"}
	if _, err := s.Update(Op{Name: "workspace", Detail: "add api (API)", Workspace: w.ID, Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(w) }); err != nil {
		t.Fatal(err)
	}
	subj, msg := subjectAndBody(t, s)
	if subj != "hedgehug: workspace add api (API)" {
		t.Errorf("subject = %q", subj)
	}
	for _, want := range []string{"Hedgehug-Op: workspace", "Hedgehug-Workspace: " + w.ID.String()} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q:\n%s", want, msg)
		}
	}
}

func TestCreateRefusesZeroWorkspaceID(t *testing.T) {
	meta := &model.StoreMeta{ID: model.NewID(), Name: "test"}
	_, err := Create(filepath.Join(t.TempDir(), "store"), meta, nil, &model.Workspace{Name: "test", KeyPrefix: "TST"}, testAuthor)
	if err == nil || !strings.Contains(err.Error(), "no id") {
		t.Errorf("err = %v", err)
	}
}

func TestRemoveWorkspaceRefusesBrokenTaskNamingIt(t *testing.T) {
	s := newTestStore(t)
	w := &model.Workspace{ID: model.NewID(), Name: "api", KeyPrefix: "API"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(w) }); err != nil {
		t.Fatal(err)
	}
	other := model.TaskPath(model.NewID())
	mustWrite(t, s, other, "not a task\n") // names no workspace: does not block
	broken := model.TaskPath(model.NewID())
	mustWrite(t, s, broken, "---\nworkspace: "+w.ID.String()+"\ntitle: [unclosed\n---\n")
	_, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveWorkspace(w.ID) })
	if err == nil || !strings.Contains(err.Error(), broken) || strings.Contains(err.Error(), other) {
		t.Fatalf("err = %v", err)
	}
	if err := os.Remove(s.path(broken)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveWorkspace(w.ID) }); err != nil {
		t.Errorf("after the broken file is gone: %v", err)
	}
}

func TestTxWorkspaceSeesTheOperation(t *testing.T) {
	s := newTestStore(t)
	w := &model.Workspace{ID: model.NewID(), Name: "api", KeyPrefix: "API"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error {
		if err := tx.PutWorkspace(w); err != nil {
			return err
		}
		if got, err := tx.Workspace(w.ID); err != nil || got != w {
			t.Errorf("put earlier: %v %v", got, err)
		}
		if err := tx.RemoveWorkspace(testWorkspaceID); err != nil {
			return err
		}
		if _, err := tx.Workspace(testWorkspaceID); !errors.Is(err, ErrNotFound) {
			t.Errorf("removed earlier: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestNumbersWithoutFile(t *testing.T) {
	n, err := newTestStore(t).Numbers()
	if err != nil || n == nil || len(n) != 0 {
		t.Errorf("Numbers = %v %v", n, err)
	}
}

// purge runs one purge operation removing ids.
func purge(t *testing.T, s *Store, ids ...model.ID) {
	t.Helper()
	if _, err := s.Update(Op{Name: "purge", Author: testAuthor}, func(tx *Tx) error {
		for _, id := range ids {
			if err := tx.RemoveTask(id); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// committedPaths returns the paths the last commit changed.
func committedPaths(t *testing.T, s *Store) []string {
	t.Helper()
	stats, err := headLog(t, s.dir)[0].Stats()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range stats {
		out = append(out, f.Name)
	}
	return out
}

func TestPurgeKeepsLargerNumbers(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	big, small := newTaskIn(ws, "big"), newTaskIn(ws, "small")
	big.Key, small.Key = "TST-50", "TST-3"
	mustPut(t, s, big, small)
	purge(t, s, big.ID)
	before, err := os.ReadFile(s.path(model.NumbersPath))
	if err != nil {
		t.Fatal(err)
	}
	purge(t, s, small.ID)
	after, err := os.ReadFile(s.path(model.NumbersPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("numbers.yaml changed:\n%s", after)
	}
	if paths := committedPaths(t, s); slices.Contains(paths, model.NumbersPath) {
		t.Errorf("the purge of TST-3 rewrote %s: %v", model.NumbersPath, paths)
	}
}

func TestPurgeRecordsNumbersOfSeveralTasksInOneCommit(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	a, b := newTaskIn(ws, "a"), newTaskIn(ws, "b")
	a.Key = "TST-4"
	b.Key, b.FormerKeys = "TST-2", []string{"OLD-9"}
	mustPut(t, s, a, b)
	purge(t, s, a.ID, b.ID)
	n, err := s.Numbers()
	if err != nil || n["TST"] != 4 || n["OLD"] != 9 {
		t.Errorf("Numbers = %v %v", n, err)
	}
	paths := committedPaths(t, s)
	for _, want := range []string{model.NumbersPath, model.TaskPath(a.ID), model.TaskPath(b.ID)} {
		if !slices.Contains(paths, want) {
			t.Errorf("the commit lacks %s: %v", want, paths)
		}
	}
}

func TestPurgeSubjectWithoutKeyUsesID(t *testing.T) {
	s := newTestStore(t)
	x := newTaskIn(testWorkspace(t, s), "keyless")
	b, err := x.Encode()
	if err != nil {
		t.Fatal(err)
	}
	rel := model.TaskPath(x.ID)
	mustWrite(t, s, rel, string(b))
	if err := s.commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: testAuthor, When: testNow}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(Op{Name: "purge", Task: x.ID, Author: testAuthor}, func(tx *Tx) error { return tx.RemoveTask(x.ID) }); err != nil {
		t.Fatal(err)
	}
	if subj, _ := subjectAndBody(t, s); subj != "hedgehug: purge "+x.ID.String()+" keyless" {
		t.Errorf("subject = %q", subj)
	}
}
