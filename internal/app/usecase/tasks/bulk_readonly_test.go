package tasks_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestBulkRefusesAReadOnlyStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	a := ts.AddTask("a", nil)
	usecasetest.MakeReadOnly(t, ts, replicate.ReasonGitHub)
	head := ts.Head()
	_, err := tasks.Bulk(ts.Session, tasks.BulkInput{Targets: []usecase.TaskTarget{{Store: "me", ULID: a.ULID}}, Assigns: set("labels", "x")})
	if err == nil {
		t.Fatal("a read-only store was written")
	}
	if ts.Head() != head {
		t.Error("a refused update made a commit")
	}
}
