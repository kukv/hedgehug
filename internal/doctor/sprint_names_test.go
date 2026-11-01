package doctor

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

func TestDoctorNamesDifferingInCaseAreDuplicates(t *testing.T) {
	s, dir := newStore(t)
	ws := firstWorkspace(t, s)
	a := commitSprint(t, s, dir, &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: "Sprint"})
	b := commitSprint(t, s, dir, &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: "sprint"})
	fs := run(t, dir)
	for _, rel := range []string{a, b} {
		if f := find(t, fs, "file", rel); !strings.Contains(f.Msg, "has the same name") {
			t.Errorf("%s: %+v", rel, f)
		}
	}
}

func TestDoctorTaskPointsAtMissingSprintOrRelease(t *testing.T) {
	s, dir := newStore(t)
	ws := firstWorkspace(t, s)
	sp := &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: "ok"}
	commitSprint(t, s, dir, sp)
	goneS, goneR := model.NewULID(), model.NewULID()
	task := newTask("bad")
	task.Sprint, task.Releases = &goneS, []model.ULID{goneR}
	ok := newTask("fine")
	ok.Sprint = &sp.ULID
	putRaw(t, s, dir, task)
	putRaw(t, s, dir, ok)
	var msgs string
	n := 0
	for _, f := range run(t, dir) {
		if f.Check == "task" {
			msgs += f.Msg + "\n"
			n++
		}
	}
	if n != 2 || !strings.Contains(msgs, "sprint: sprint "+goneS.String()+" is not in this store") ||
		!strings.Contains(msgs, "releases: release "+goneR.String()+" is not in this store") {
		t.Errorf("task findings = %d:\n%s", n, msgs)
	}
}
