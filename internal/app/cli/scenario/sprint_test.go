package scenario

import (
	"strings"
	"testing"
)

// TestSprintFlow runs a sprint from its creation to its close with two users,
// the second one seeing the plan through sync.
func TestSprintFlow(t *testing.T) {
	t.Parallel()
	bin, devenv := buildHedgehug(t), buildDevenv(t)
	dev, _, _ := initDevenv(t, devenv)
	alice, bob := devUserOf(t, bin, devenv, dev, "alice"), devUserOf(t, bin, devenv, dev, "bob")
	const org = "--store=org-a"

	alice.ok("sprint", "add", "Sprint 1", "--start", "2026-11-02", "--end", "2026-11-13", "--goal", "first", org)
	alice.ok("sprint", "add", "Sprint 2", "--start", "2026-11-16", "--end", "2026-11-27", org)
	alice.ok("release", "add", "v1", "--date", "2026-12-01", org)
	alice.ok("add", "finish", "--sprint", "Sprint 1", "--release", "v1", org)
	alice.ok("add", "carry", "--sprint", "Sprint 1", org)
	alice.ok("sprint", "start", "Sprint 1", org)
	alice.ok("sync")
	bob.ok("sync")

	if got := bob.ok("ls", "sprint:current", org); !strings.Contains(got, "finish") || !strings.Contains(got, "carry") {
		t.Fatalf("bob's sprint:current:\n%s", got)
	}
	finish := strings.TrimSpace(bob.ok("ls", "finish", "--ulids-only", org))
	bob.ok("set", finish, "status=done", org)
	bob.ok("worklog", finish, "2h", "done", org)
	bob.ok("sync")
	alice.ok("sync")

	if got := alice.ok("sprint", "close", "Sprint 1", "--to", "next", org); !strings.Contains(got, "moved 1 task to Sprint 2") {
		t.Fatalf("close:\n%s", got)
	}
	if got := alice.ok("ls", `sprint:"Sprint 2"`, org); !strings.Contains(got, "carry") || strings.Contains(got, "finish") {
		t.Fatalf("Sprint 2:\n%s", got)
	}
	if got := alice.ok("release", "notes", "v1", org); !strings.Contains(got, " finish") {
		t.Fatalf("notes:\n%s", got)
	}
	if got := alice.ok("show", finish, org); !strings.Contains(got, "2h") {
		t.Fatalf("show lacks the worklog:\n%s", got)
	}
}
