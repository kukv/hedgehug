package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/planning"
)

// addStartedSprint adds the sprint s1 and starts it.
func addStartedSprint(te *testEnv) {
	te.ok("sprint", "add", "s1", "--start", "2026-11-02", "--end", "2026-11-13", "--goal", "ship")
	te.ok("sprint", "start", "s1")
}

func TestSprintCommands(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	addStartedSprint(te)
	te.ok("sprint", "add", "s2", "--start", "2026-11-16", "--end", "2026-11-27")
	te.ok("set", a.ID, "sprint=s1", "points=3")

	out := te.ok("sprint", "ls")
	for _, want := range []string{"NAME", "s1", "active", "2026-11-02", "0/1", "s2", "planned"} {
		if !strings.Contains(out, want) {
			t.Errorf("ls lacks %q:\n%s", want, out)
		}
	}
	if got := te.ok("sprint", "ls", "--state", "planned"); strings.Contains(got, "s1") {
		t.Errorf("--state planned shows s1:\n%s", got)
	}
	var vs []planning.SprintView
	if err := json.Unmarshal([]byte(te.ok("sprint", "ls", "--json")), &vs); err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 || vs[0].Points != 3 || vs[0].Goal != "ship" {
		t.Fatalf("json = %+v", vs)
	}
	if got := te.ok("ls", "sprint:current"); !strings.Contains(got, "a") {
		t.Errorf("sprint:current:\n%s", got)
	}

	if got := te.ok("sprint", "close", "s1", "--to", "next"); !strings.Contains(got, "moved 1 task to s2") {
		t.Errorf("close:\n%s", got)
	}
	v := showJSON(t, te, a.ID)
	if v.Sprint == nil || v.Sprint.Name != "s2" {
		t.Fatalf("sprint = %+v", v.Sprint)
	}

	te.ok("sprint", "set", "s2", "name=later", "goal=more")
	te.ok("sprint", "rm", "later")
	if v := showJSON(t, te, a.ID); v.Sprint != nil {
		t.Fatalf("sprint = %+v", v.Sprint)
	}
}

func TestSprintCloseAsksOnATerminal(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	addStartedSprint(te)
	te.ok("set", a.ID, "sprint=s1")

	code, _, stderr := te.run("sprint", "close", "s1")
	if code != 2 || !strings.Contains(stderr, "--to") {
		t.Fatalf("without a terminal: code %d, stderr %s", code, stderr)
	}

	te.terminal = true
	te.stdin = "b\n"
	if got := te.ok("sprint", "close", "s1"); !strings.Contains(got, "moved 1 task to backlog") {
		t.Errorf("close:\n%s", got)
	}
	if got := te.task(a.ULID); got.Sprint != nil {
		t.Errorf("sprint = %v", got.Sprint)
	}
}

func TestSprintCloseDoesNotAskWithNothingOpen(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	addStartedSprint(te)
	te.terminal = true
	if got := te.ok("sprint", "close", "s1"); strings.Contains(got, "moved") {
		t.Errorf("close:\n%s", got)
	}
}

func TestReleaseCommands(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("first")
	te.ok("release", "add", "v1", "--date", "2026-12-01")
	te.ok("set", a.ID, "releases+=v1", "status=done")
	out := te.ok("release", "ls")
	for _, want := range []string{"v1", "unreleased", "2026-12-01", "1/1"} {
		if !strings.Contains(out, want) {
			t.Errorf("ls lacks %q:\n%s", want, out)
		}
	}
	if got := te.ok("release", "notes", "v1"); !strings.Contains(got, "# v1") || !strings.Contains(got, a.ID+" first") {
		t.Errorf("notes:\n%s", got)
	}
	te.ok("release", "set", "v1", "state=released")
	if got := te.ok("ls", "release:v1"); !strings.Contains(got, "first") {
		t.Errorf("release:v1:\n%s", got)
	}
	te.ok("release", "rm", "v1")
	if got := te.task(a.ULID); len(got.Releases) != 0 {
		t.Errorf("releases = %v", got.Releases)
	}
}

func TestWorklogCommands(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	te.ok("worklog", a.ID, "1h30m", "fixing", "it")
	te.ok("worklog", a.ID, "45m", "--date", "2026-10-01")
	out := te.ok("worklog", "ls", a.ID)
	for _, want := range []string{"1h30m", "fixing it", "2026-10-01", "45m"} {
		if !strings.Contains(out, want) {
			t.Errorf("ls lacks %q:\n%s", want, out)
		}
	}
	if got := te.ok("show", a.ID); !strings.Contains(got, "2h15m") {
		t.Errorf("show lacks the total:\n%s", got)
	}
	v := showJSON(t, te, a.ID)
	if v.Worklog == nil || v.Worklog.Total != "2h15m" || v.Worklog.Count != 2 {
		t.Fatalf("worklog = %+v", v.Worklog)
	}

	var list []struct{ ULID string }
	if err := json.Unmarshal([]byte(te.ok("worklog", "ls", a.ID, "--json")), &list); err != nil {
		t.Fatal(err)
	}
	te.ok("worklog", "rm", a.ID, list[0].ULID)
	if v := showJSON(t, te, a.ID); v.Worklog == nil || v.Worklog.Count != 1 {
		t.Fatalf("after rm: %+v", v.Worklog)
	}

	for _, args := range [][]string{
		{"worklog", a.ID, "soon"},
		{"worklog", a.ID, "1h", "--date", "soon"},
		{"worklog", a.ID, "0m"},
	} {
		if code, _, _ := te.run(args...); code == 0 {
			t.Errorf("%v succeeded", args)
		}
	}
	if code, _, _ := te.run("worklog", "rm", a.ID, model.NewULID().String()); code == 0 {
		t.Error("rm of an unknown entry succeeded")
	}
}

func TestWorklogRmRefusesAnothersEntry(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	var w struct {
		Worklog struct{ ULID string }
	}
	if err := json.Unmarshal([]byte(te.ok("worklog", a.ID, "1h", "--json")), &w); err != nil {
		t.Fatal(err)
	}
	te.withOtherWriter("github.com/bob", func() {
		if code, _, stderr := te.run("worklog", "rm", a.ID, w.Worklog.ULID); code == 0 || !strings.Contains(stderr, "cannot be removed") {
			t.Fatalf("code %d, stderr %s", code, stderr)
		}
	})
}
