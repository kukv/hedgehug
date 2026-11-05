package cli

import (
	"strings"
	"testing"
)

// Two users set different sprints on a task: the one who loses is told, and
// can write the value back.
func TestInboxRevertsAnOverwrittenSprint(t *testing.T) {
	t.Parallel()
	alice, bob, _ := twoUsers(t)
	alice.ok("sprint", "add", "s1")
	alice.ok("sprint", "add", "s2")
	alice.ok("sync")
	bob.ok("sync")
	conflict(t, alice, bob, "sprint", "s1", "s2")
	bob.ok("inbox", "revert", itemOfKind(t, bob, "overwritten"))
	if got := bob.ok("show", "ORG-1"); !strings.Contains(got, "s1") {
		t.Fatalf("revert did not write s1 back:\n%s", got)
	}
}
