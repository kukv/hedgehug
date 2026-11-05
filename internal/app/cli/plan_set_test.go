package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// putPlans writes a sprint and two releases into the workspace of ws.
func putPlans(t *testing.T, te *testEnv, ws string) {
	t.Helper()
	u := mustULID(t, ws)
	te.uc().MustEdit("me", "update", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		for _, err := range []error{
			tx.PutSprint(&model.Sprint{ULID: model.NewULID(), Workspace: u, Name: "s1", State: model.SprintPlanned}),
			tx.PutRelease(&model.Release{ULID: model.NewULID(), Workspace: u, Name: "v1", State: model.ReleaseUnreleased}),
			tx.PutRelease(&model.Release{ULID: model.NewULID(), Workspace: u, Name: "v2", State: model.ReleaseUnreleased}),
		} {
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func showJSON(t *testing.T, te *testEnv, id string) usecase.ShowView {
	t.Helper()
	var v usecase.ShowView
	if err := json.Unmarshal([]byte(te.ok("show", id, "--json")), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestSetSprintAndReleases(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	putPlans(t, te, a.Workspace.ULID)

	te.ok("set", a.ID, "sprint=S1", "releases=v1", "releases+=v2")
	v := showJSON(t, te, a.ID)
	if v.Sprint == nil || v.Sprint.Name != "s1" || v.Sprint.State != model.SprintPlanned {
		t.Fatalf("sprint = %+v", v.Sprint)
	}
	if len(v.Releases) != 2 || v.Releases[0].Name != "v1" || v.Releases[1].Name != "v2" {
		t.Fatalf("releases = %+v", v.Releases)
	}
	out := te.ok("show", a.ID)
	for _, want := range []string{"s1", "v1, v2"} {
		if !strings.Contains(out, want) {
			t.Errorf("show lacks %q:\n%s", want, out)
		}
	}

	te.ok("set", a.ID, "sprint=", "releases-=v1")
	v = showJSON(t, te, a.ID)
	if v.Sprint != nil || len(v.Releases) != 1 || v.Releases[0].Name != "v2" {
		t.Fatalf("after clearing: sprint %+v releases %+v", v.Sprint, v.Releases)
	}
}

func TestSetSprintUnknownName(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	code, _, stderr := te.run("set", a.ID, "sprint=nope")
	if code == 0 || !strings.Contains(stderr, `sprint "nope"`) {
		t.Fatalf("code %d, stderr %s", code, stderr)
	}
	code, _, stderr = te.run("set", a.ID, "releases=nope")
	if code == 0 || !strings.Contains(stderr, `release "nope"`) {
		t.Fatalf("code %d, stderr %s", code, stderr)
	}
}

func TestSetSprintOfAnotherWorkspace(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("workspace", "add", "api", "--prefix", "API")
	a := te.addJSON("a", "--workspace", "me")
	b := te.addJSON("b", "--workspace", "api")
	putPlans(t, te, a.Workspace.ULID)
	if code, _, stderr := te.run("set", b.ID, "sprint=s1"); code == 0 || !strings.Contains(stderr, "sprint") {
		t.Fatalf("code %d, stderr %s", code, stderr)
	}
}

func TestAddWithSprintAndRelease(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	putPlans(t, te, a.Workspace.ULID)
	b := te.addJSON("b", "--sprint", "s1", "--release", "v1", "--release", "v2")
	v := showJSON(t, te, b.ID)
	if v.Sprint == nil || v.Sprint.Name != "s1" || len(v.Releases) != 2 {
		t.Fatalf("sprint %+v releases %+v", v.Sprint, v.Releases)
	}
}
