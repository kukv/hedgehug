package doctor

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// commitSprint and commitRelease commit a sprint or a release past the checks
// of the store, as a sync or a hand edit could.
func commitSprint(t *testing.T, s *store.Store, dir string, sp *model.Sprint) string {
	t.Helper()
	b, err := sp.Encode()
	if err != nil {
		t.Fatal(err)
	}
	rel := model.SprintPath(sp.ULID)
	writeFile(t, dir, rel, string(b))
	if _, err := s.Repo().Commit(commitOf(rel)); err != nil {
		t.Fatal(err)
	}
	return rel
}

func commitRelease(t *testing.T, s *store.Store, dir string, r *model.Release) string {
	t.Helper()
	b, err := r.Encode()
	if err != nil {
		t.Fatal(err)
	}
	rel := model.ReleasePath(r.ULID)
	writeFile(t, dir, rel, string(b))
	if _, err := s.Repo().Commit(commitOf(rel)); err != nil {
		t.Fatal(err)
	}
	return rel
}

func secondWorkspace(t *testing.T, s *store.Store, dir string) model.ULID {
	t.Helper()
	w := &model.Workspace{ULID: model.NewULID(), Name: "two", IDPrefix: "TWO", PrefixSet: time.Now(), Created: time.Now()}
	commitWorkspace(t, s, dir, w)
	return w.ULID
}

func TestDoctorSprintAndReleaseNeedWorkspaceOfStore(t *testing.T) {
	s, dir := newStore(t)
	ws := firstWorkspace(t, s)
	ok := commitSprint(t, s, dir, &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: "ok"})
	none := commitSprint(t, s, dir, &model.Sprint{ULID: model.NewULID(), Name: "none"})
	lost := commitSprint(t, s, dir, &model.Sprint{ULID: model.NewULID(), Workspace: model.NewULID(), Name: "lost"})
	noneR := commitRelease(t, s, dir, &model.Release{ULID: model.NewULID(), Name: "none"})
	lostR := commitRelease(t, s, dir, &model.Release{ULID: model.NewULID(), Workspace: model.NewULID(), Name: "lost"})
	fs := run(t, dir)
	if !strings.Contains(find(t, fs, "file", none).Msg, "workspace is missing") {
		t.Errorf("none: %+v", fs)
	}
	if !strings.Contains(find(t, fs, "file", noneR).Msg, "workspace is missing") {
		t.Errorf("none release: %+v", fs)
	}
	for _, rel := range []string{lost, lostR} {
		if f := find(t, fs, "file", rel); !strings.Contains(f.Msg, "is not a workspace of this store") {
			t.Errorf("%s: %+v", rel, f)
		}
	}
	for _, f := range fs {
		if f.Where == ok {
			t.Errorf("a sprint in a workspace of the store: %+v", f)
		}
	}
}

func TestDoctorSprintAndReleaseNamesAreUniqueInWorkspace(t *testing.T) {
	s, dir := newStore(t)
	one, two := firstWorkspace(t, s), secondWorkspace(t, s, dir)
	a := commitSprint(t, s, dir, &model.Sprint{ULID: model.NewULID(), Workspace: one, Name: "S1"})
	b := commitSprint(t, s, dir, &model.Sprint{ULID: model.NewULID(), Workspace: one, Name: "S1"})
	other := commitSprint(t, s, dir, &model.Sprint{ULID: model.NewULID(), Workspace: two, Name: "S1"})
	ra := commitRelease(t, s, dir, &model.Release{ULID: model.NewULID(), Workspace: one, Name: "v1"})
	rb := commitRelease(t, s, dir, &model.Release{ULID: model.NewULID(), Workspace: one, Name: "v1"})
	// A sprint and a release may share a name.
	mixed := commitRelease(t, s, dir, &model.Release{ULID: model.NewULID(), Workspace: one, Name: "S1"})
	fs := run(t, dir)
	for _, rel := range []string{a, b, ra, rb} {
		if f := find(t, fs, "file", rel); !strings.Contains(f.Msg, "has the same name") {
			t.Errorf("%s: %+v", rel, f)
		}
	}
	for _, f := range fs {
		if f.Where == other || f.Where == mixed {
			t.Errorf("no duplicate expected: %+v", f)
		}
	}
}

func TestDoctorTaskSprintAndReleasesOfOtherWorkspace(t *testing.T) {
	s, dir := newStore(t)
	one, two := firstWorkspace(t, s), secondWorkspace(t, s, dir)
	mine := &model.Sprint{ULID: model.NewULID(), Workspace: one, Name: "mine"}
	theirs := &model.Sprint{ULID: model.NewULID(), Workspace: two, Name: "theirs"}
	myRel := &model.Release{ULID: model.NewULID(), Workspace: one, Name: "v1"}
	theirRel := &model.Release{ULID: model.NewULID(), Workspace: two, Name: "v2"}
	commitSprint(t, s, dir, mine)
	commitSprint(t, s, dir, theirs)
	commitRelease(t, s, dir, myRel)
	commitRelease(t, s, dir, theirRel)

	good, bad := newTask("good"), newTask("bad")
	good.Sprint, good.Releases = &mine.ULID, []model.ULID{myRel.ULID}
	bad.Sprint, bad.Releases = &theirs.ULID, []model.ULID{myRel.ULID, theirRel.ULID}
	putRaw(t, s, dir, good)
	putRaw(t, s, dir, bad)

	var got []Finding
	for _, f := range run(t, dir) {
		if f.Check == "task" {
			got = append(got, f)
		}
	}
	if len(got) != 2 {
		t.Fatalf("task findings = %+v; want two, for the sprint and for the release of the other workspace", got)
	}
	for _, f := range got {
		if !strings.Contains(f.Where, "bad") {
			t.Errorf("finding on the wrong task: %+v", f)
		}
	}
	if !strings.Contains(got[0].Msg+got[1].Msg, "sprint: sprint "+theirs.ULID.String()) || !strings.Contains(got[0].Msg+got[1].Msg, "releases: release "+theirRel.ULID.String()) {
		t.Errorf("findings = %+v", got)
	}
}
