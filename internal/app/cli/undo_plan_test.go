package cli

import (
	"strings"
	"testing"
)

func TestUndoPlanningOperations(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	addStartedSprint(te)
	te.ok("sprint", "add", "s2")
	te.ok("set", a.ID, "sprint=s1")

	// sprint-close takes back the state and the task's move.
	te.ok("sprint", "close", "s1", "--to", "s2")
	te.ok("undo")
	if v := showJSON(t, te, a.ID); v.Sprint == nil || v.Sprint.Name != "s1" || v.Sprint.State != "active" {
		t.Fatalf("after undoing close: %+v", v.Sprint)
	}

	// sprint-rm brings the sprint and the task's reference back.
	te.ok("sprint", "rm", "s2")
	te.ok("undo")
	if got := te.ok("sprint", "ls"); !strings.Contains(got, "s2") {
		t.Fatalf("s2 is gone:\n%s", got)
	}

	// sprint-add removes the file.
	te.ok("sprint", "add", "s3")
	te.ok("undo")
	if got := te.ok("sprint", "ls"); strings.Contains(got, "s3") {
		t.Fatalf("s3 stays:\n%s", got)
	}

	te.ok("release", "add", "v1")
	te.ok("release", "set", "v1", "state=released")
	te.ok("undo")
	if got := te.ok("release", "ls"); !strings.Contains(got, "unreleased") {
		t.Fatalf("release state stays:\n%s", got)
	}

	te.ok("worklog", a.ID, "1h")
	te.ok("undo")
	if v := showJSON(t, te, a.ID); v.Worklog != nil {
		t.Fatalf("worklog stays: %+v", v.Worklog)
	}
}

func TestSameSprintNameInTwoClonesIsReported(t *testing.T) {
	t.Parallel()
	alice, bob, _ := twoUsers(t)
	alice.ok("sprint", "add", "s1")
	bob.ok("sprint", "add", "s1")
	alice.ok("sync")
	bob.ok("sync")
	alice.ok("sync")
	_, out, _ := bob.run("doctor")
	if !strings.Contains(out, "has the same name") {
		t.Fatalf("doctor:\n%s", out)
	}
}
