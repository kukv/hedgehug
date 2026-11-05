package tasks_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// The commits that sync issues are not undone: the next sync would write the
// same again.
func TestUndoPassesOverTheCommitsOfIssueSync(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Set(ts.Session, "ME-1", []usecase.Assign{{Key: "title", Op: "=", Value: "b"}}); err != nil {
		t.Fatal(err)
	}
	s, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, err := ts.Resolve(ix, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{usecase.OpGitHubMirror, usecase.OpGitHubPush} {
		if _, err := ts.Update(s, store.Op{Name: op, Task: e.ULID, Automated: true}, func(tx *store.Tx) error {
			tx.AllowEmpty()
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	res, err := tasks.Undo(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	if res.Undone == nil || res.Undone.Op == nil || *res.Undone.Op != "set" {
		t.Errorf("undone = %+v", res.Undone)
	}
}
