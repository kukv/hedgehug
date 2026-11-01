package main

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

func TestMvJSONShowsWhatItTookOff(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("workspace", "add", "api", "--prefix", "API")
	p := te.addJSON("p", "--workspace", "me")
	q := te.addJSON("q", "--workspace", "me")
	ws := mustULID(t, p.Workspace.ULID)
	sp := &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: "S3"}
	rel := &model.Release{ULID: model.NewULID(), Workspace: ws, Name: "v1"}
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(store.Op{Name: "update", Author: gitrepo.Signature{Name: "T", Email: "t@example.com"}}, func(tx *store.Tx) error {
		for _, err := range []error{
			tx.PutSprint(sp), tx.PutRelease(rel),
			changeTask(p.ULID, func(t *model.Task) { t.Sprint, t.Releases = &sp.ULID, []model.ULID{rel.ULID} })(tx),
		} {
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var got struct {
		ops.TaskView
		Detached []ops.Detached `json:"detached"`
	}
	if err := json.Unmarshal([]byte(te.ok("mv", p.ID, "--workspace", "api", "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "API-1" || len(got.Detached) != 1 || got.Detached[0].ID != "API-1" || got.Detached[0].Sprint != "S3" ||
		!slices.Equal(got.Detached[0].Releases, []string{"v1"}) {
		t.Errorf("mv --json = %+v", got)
	}

	// Nothing taken off: the task alone, with no detached key.
	var plain map[string]any
	if err := json.Unmarshal([]byte(te.ok("mv", q.ID, "--workspace", "api", "--json")), &plain); err != nil {
		t.Fatal(err)
	}
	if _, has := plain["detached"]; has || plain["id"] != "API-2" {
		t.Errorf("mv --json = %v", plain)
	}
}
