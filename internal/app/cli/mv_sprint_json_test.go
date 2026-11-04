package cli

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestMvJSONShowsWhatItTookOff(t *testing.T) {
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

	var got usecase.TaskWrite
	if err := json.Unmarshal([]byte(te.ok("mv", p.ID, "--workspace", "api", "--json")), &got); err != nil || got.Task == nil {
		t.Fatal(err)
	}
	if got.Task.ID != "API-1" || len(got.Detached) != 1 || got.Detached[0].ID != "API-1" || got.Detached[0].Sprint != "S3" ||
		!slices.Equal(got.Detached[0].Releases, []string{"v1"}) {
		t.Errorf("mv --json = %+v", got)
	}

	// Nothing taken off: no detached key.
	var plain map[string]any
	if err := json.Unmarshal([]byte(te.ok("mv", q.ID, "--workspace", "api", "--json")), &plain); err != nil {
		t.Fatal(err)
	}
	if task, _ := plain["task"].(map[string]any); task == nil || task["id"] != "API-2" || plain["detached"] != nil {
		t.Errorf("mv --json = %v", plain)
	}
}
