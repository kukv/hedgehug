package cli

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestMvWorkspaceTellsWhatItTookOff(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("workspace", "add", "api", "--prefix", "API")
	p := te.addJSON("p", "--workspace", "me")
	q := te.addJSON("q", "--workspace", "me")
	ws := mustULID(t, p.Workspace.ULID)
	sp := &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: "S3"}
	rel := &model.Release{ULID: model.NewULID(), Workspace: ws, Name: "v1"}
	te.uc().MustEdit("me", "update", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		for _, err := range []error{
			tx.PutSprint(sp), tx.PutRelease(rel),
			changeTask(p.ULID, func(t *model.Task) { t.Sprint, t.Releases = &sp.ULID, []model.ULID{rel.ULID} })(tx),
		} {
			if err != nil {
				return err
			}
		}
		return nil
	})

	out := te.ok("mv", p.ID, "--workspace", "api")
	for _, want := range []string{"ME-1 → API-1", "API-1: removed from sprint S3", "API-1: removed from release v1"} {
		if !strings.Contains(out, want) {
			t.Errorf("mv lacks %q:\n%s", want, out)
		}
	}
	if got := te.task(p.ULID); got.Sprint != nil || len(got.Releases) != 0 {
		t.Errorf("task keeps sprint %v releases %v", got.Sprint, got.Releases)
	}

	// A task without a sprint or a release has nothing to be told.
	if out := te.ok("mv", q.ID, "--workspace", "api"); strings.Contains(out, "removed from") {
		t.Errorf("mv =\n%s", out)
	}
}
