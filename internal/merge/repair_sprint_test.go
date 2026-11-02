package merge

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

func sprintFile(ulid, ws string) []byte {
	return []byte("---\nulid: " + ulid + "\nworkspace: " + ws + "\nname: s\nupdated: 2026-10-30T00:00:00Z\n---\n")
}

func TestRepairWorkspaceMismatchDetachesSprintAndReleases(t *testing.T) {
	t.Parallel()
	p, c := "01JD0000000000000000000001", "01JD0000000000000000000002"
	spA, relA, relB := "01JD0000000000000000000011", "01JD0000000000000000000012", "01JD0000000000000000000013"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		model.WorkspacePath(mustULID(wsB)): wsFile(wsB, "api", "API"),
		model.SprintPath(mustULID(spA)):    sprintFile(spA, wsA),
		model.ReleasePath(mustULID(relA)):  sprintFile(relA, wsA),
		model.ReleasePath(mustULID(relB)):  sprintFile(relB, wsB),
		pathOf(p):                          taskFile(p, "API-1", wsB, "", "2026-10-31T10:00:00Z", "former_ids: [WEB-1]"),
		pathOf(c):                          taskFile(c, "WEB-2", wsA, p, "2026-10-31T10:00:00Z", "sprint: "+spA, "releases: ["+relA+", "+relB+"]"),
	}
	after, rs := repair(t, RepairInput{Files: f})
	got := decoded(t, after[pathOf(c)])
	if got.Workspace.String() != wsB || got.Sprint != nil || !slices.Equal(got.Releases, []model.ULID{mustULID(relB)}) {
		t.Errorf("child = workspace %s sprint %v releases %v; want %s, no sprint, [%s]", got.Workspace, got.Sprint, got.Releases, wsB, relB)
	}
	want := []Repaired{
		{ULID: mustULID(c), Kind: "workspace_mismatch", Value: `"` + wsA + `"`, Author: "github.com/x"},
		{ULID: mustULID(c), Kind: "sprint_detached", Value: `"` + spA + `"`, Author: "github.com/x"},
		{ULID: mustULID(c), Kind: "release_detached", Value: `"` + relA + `"`, Author: "github.com/x"},
	}
	if !slices.Equal(rs, want) {
		t.Errorf("repaired = %+v; want %+v", rs, want)
	}
}

func TestRepairWorkspaceMismatchDetachesWhatCannotBeRead(t *testing.T) {
	t.Parallel()
	p, c := "01JD0000000000000000000001", "01JD0000000000000000000002"
	gone := "01JD0000000000000000000011"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		model.WorkspacePath(mustULID(wsB)): wsFile(wsB, "api", "API"),
		pathOf(p):                          taskFile(p, "API-1", wsB, "", "2026-10-31T10:00:00Z", "former_ids: [WEB-1]"),
		pathOf(c):                          taskFile(c, "WEB-2", wsA, p, "2026-10-31T10:00:00Z", "sprint: "+gone),
	}
	after, rs := repair(t, RepairInput{Files: f})
	if got := decoded(t, after[pathOf(c)]); got.Sprint != nil {
		t.Errorf("sprint = %v; want none", got.Sprint)
	}
	if len(rs) != 2 || rs[1].Kind != "sprint_detached" || rs[1].Value != `"`+gone+`"` {
		t.Errorf("repaired = %+v", rs)
	}
}

func TestRepairKeepsSprintOfTaskThatDidNotMove(t *testing.T) {
	t.Parallel()
	x := "01JD0000000000000000000001"
	sp := "01JD0000000000000000000011"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		model.SprintPath(mustULID(sp)):     sprintFile(sp, wsA),
		pathOf(x):                          taskFile(x, "WEB-1", wsA, "", "2026-10-31T10:00:00Z", "sprint: "+sp),
	}
	after, rs := repair(t, RepairInput{Files: f})
	if len(rs) != 0 || after[pathOf(x)] == nil || decoded(t, after[pathOf(x)]).Sprint == nil {
		t.Errorf("repaired = %+v", rs)
	}
}
