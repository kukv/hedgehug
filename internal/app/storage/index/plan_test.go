package index

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

func newSprint(t *testing.T, ws model.ULID, name, start, state string) *model.Sprint {
	t.Helper()
	sp := &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: name, State: state}
	if start != "" {
		d, err := model.ParseDate(start)
		if err != nil {
			t.Fatal(err)
		}
		sp.Start = d
	}
	return sp
}

func putPlans(t *testing.T, s *store.Store, sprints []*model.Sprint, releases []*model.Release) {
	t.Helper()
	if _, err := s.Update(store.Op{Name: "sprint-add", Author: testAuthor}, func(tx *store.Tx) error {
		for _, sp := range sprints {
			if err := tx.PutSprint(sp); err != nil {
				return err
			}
		}
		for _, r := range releases {
			if err := tx.PutRelease(r); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func sprintNames(sps []*model.Sprint) []string {
	out := make([]string, len(sps))
	for i, sp := range sps {
		out[i] = sp.Name
	}
	return out
}

func TestPlansAreRead(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t)
	ws := firstWorkspace(t, s)
	ix := mustOpen(t, s)
	late := newSprint(t, ws, "late", "2026-12-01", model.SprintPlanned)
	early := newSprint(t, ws, "early", "2026-11-01", model.SprintPlanned)
	undated := newSprint(t, ws, "undated", "", model.SprintPlanned)
	active := newSprint(t, ws, "active", "2026-10-01", model.SprintActive)
	rel := &model.Release{ULID: model.NewULID(), Workspace: ws, Name: "v1", State: model.ReleaseUnreleased}
	putPlans(t, s, []*model.Sprint{late, early, undated, active}, []*model.Release{rel})
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}

	got := sprintNames(ix.Sprints(ws))
	want := []string{"active", "early", "late", "undated"}
	if len(got) != len(want) {
		t.Fatalf("Sprints = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Sprints = %v, want %v", got, want)
		}
	}
	if sp, ok := ix.SprintNamed(ws, "EARLY"); !ok || sp.ULID != early.ULID {
		t.Errorf("SprintNamed(EARLY) = %v, %v", sp, ok)
	}
	if _, ok := ix.SprintNamed(model.NewULID(), "early"); ok {
		t.Error("SprintNamed found a sprint of another workspace")
	}
	if sp, ok := ix.NextSprint(ws); !ok || sp.ULID != early.ULID {
		t.Errorf("NextSprint = %v, %v, want early", sp, ok)
	}
	if r, ok := ix.ReleaseNamed(ws, "V1"); !ok || r.ULID != rel.ULID {
		t.Errorf("ReleaseNamed(V1) = %v, %v", r, ok)
	}
	if r, ok := ix.Release(rel.ULID); !ok || r.Name != "v1" {
		t.Errorf("Release = %v, %v", r, ok)
	}
	if len(ix.Releases(ws)) != 1 || len(ix.Releases(model.ULID{})) != 1 {
		t.Errorf("Releases = %v", ix.Releases(ws))
	}
}

func TestNextSprintSkipsUndatedLast(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t)
	ws := firstWorkspace(t, s)
	undated := newSprint(t, ws, "undated", "", model.SprintPlanned)
	putPlans(t, s, []*model.Sprint{undated}, nil)
	ix := mustOpen(t, s)
	if sp, ok := ix.NextSprint(ws); !ok || sp.ULID != undated.ULID {
		t.Errorf("NextSprint = %v, %v, want the undated one", sp, ok)
	}
	if _, ok := ix.NextSprint(model.NewULID()); ok {
		t.Error("NextSprint found one in a workspace with none")
	}
}

func TestEntryHasSprintAndReleases(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t)
	ws := firstWorkspace(t, s)
	sp := newSprint(t, ws, "s1", "", model.SprintPlanned)
	rel := &model.Release{ULID: model.NewULID(), Workspace: ws, Name: "v1", State: model.ReleaseUnreleased}
	putPlans(t, s, []*model.Sprint{sp}, []*model.Release{rel})
	a := newTask("a")
	a.Sprint, a.Releases = &sp.ULID, []model.ULID{rel.ULID}
	put(t, s, a)
	ix := mustOpen(t, s)
	e, ok := ix.Task(a.ULID)
	if !ok || e.Sprint == nil || *e.Sprint != sp.ULID || len(e.Releases) != 1 || e.Releases[0] != rel.ULID {
		t.Fatalf("entry = %+v", e)
	}
	// A second open reads the entry from the cache.
	ix = mustOpen(t, s)
	if e, _ := ix.Task(a.ULID); e.Sprint == nil || *e.Sprint != sp.ULID {
		t.Fatalf("cached entry = %+v", e)
	}
}

func TestBrokenPlanFileIsSkipped(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t)
	ws := firstWorkspace(t, s)
	good := newSprint(t, ws, "good", "", model.SprintPlanned)
	putPlans(t, s, []*model.Sprint{good}, nil)
	if _, err := s.Update(store.Op{Name: "broken", Author: testAuthor}, func(tx *store.Tx) error {
		return tx.PutFile(model.SprintPath(model.NewULID()), []byte("not a sprint"))
	}); err != nil {
		t.Fatal(err)
	}
	ix := mustOpen(t, s)
	if got := sprintNames(ix.Sprints(ws)); len(got) != 1 || got[0] != "good" {
		t.Fatalf("Sprints = %v", got)
	}
}
