package integration_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/github"
)

func TestAutoLinksFindTasks(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	live := ts.AddTask("live", nil)
	old := ts.AddTask("old id", func(x *model.Task) { x.FormerIDs = []string{"OLD-9"} })
	gone := ts.AddTask("deleted", nil)
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	goneEntry, _ := ix.Task(gone.ULID)
	goneID := goneEntry.ID
	if _, err := tasks.Remove(ts.Session, goneID); err != nil {
		t.Fatal(err)
	}
	_, ix, err = ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := ix.Task(live.ULID)
	oldE, _ := ix.Task(old.ULID)
	pulls := []github.Pull{
		{Number: 1, Head: "hh/" + e.ID + "-x"},
		{Number: 2, Body: "see " + oldE.ID + " and OLD-9, " + e.ID},
		{Number: 3, Comments: []string{"hh:" + live.ULID.String()}},
		{Number: 4, Body: "NOPE-1 " + goneID},
	}
	got := integration.AutoLinks(ix, "github.com/Org-A/Web", pulls)
	ref := func(n int) string {
		return model.GitHubRef{Repo: "github.com/org-a/web", Kind: model.LinkPR, Number: n}.String()
	}
	if want := []string{ref(1), ref(2), ref(3)}; !slices.Equal(got[live.ULID], want) {
		t.Errorf("live = %v, want %v", got[live.ULID], want)
	}
	if want := []string{ref(2)}; !slices.Equal(got[old.ULID], want) {
		t.Errorf("old = %v, want %v", got[old.ULID], want)
	}
	if len(got) != 2 {
		t.Errorf("got %v: a deleted task or an unknown id was linked", got)
	}
}
