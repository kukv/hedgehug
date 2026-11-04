package notifications_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestDetachedKindsShowNames(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddWorkspace("api", "API")
	sp, rel := ts.PutSprintRelease(ts.WorkspaceULID("api"), "S4", "v7")
	s, _, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	gone := model.NewULID()
	const tail = " (it is not in the workspace the task moved to)"
	for _, c := range []struct{ kind, value, want string }{
		{"sprint_detached", `"` + sp.String() + `"`, `sprint_detached: removed from sprint S4` + tail},
		{"release_detached", `"` + rel.String() + `"`, `release_detached: removed from release v7` + tail},
		{"sprint_detached", `"` + gone.String() + `"`, `sprint_detached: removed from sprint ` + gone.String() + tail},
	} {
		detail, _ := json.Marshal(map[string]any{"kind": c.kind, "value": json.RawMessage(usecase.DetachedName(s, c.kind, c.value))})
		if got := usecase.RepairedContent(nil, inbox.Item{Detail: detail}); got != c.want {
			t.Errorf("%s %s: got %q, want %q", c.kind, c.value, got, c.want)
		}
	}
	if got := usecase.DetachedName(s, "id_prefix", `"web"`); got != `"web"` {
		t.Errorf("other kinds changed: %s", got)
	}
}
