package scenario

import (
	"encoding/json"
	"testing"
	"time"
)

// TestClosedSprintReports closes a sprint with two users and reads its
// burndown and velocity from the history, on the clone that did not close it.
func TestClosedSprintReports(t *testing.T) {
	t.Parallel()
	bin, devenv := buildHedgehug(t), buildDevenv(t)
	dev, _, _ := initDevenv(t, devenv)
	alice, bob := devUserOf(t, bin, devenv, dev, "alice"), devUserOf(t, bin, devenv, dev, "bob")
	const org = "--store=org-a"
	today := time.Now().Format(time.DateOnly)

	alice.ok("sprint", "add", "s1", "--start", today, "--end", today, org)
	alice.ok("sprint", "add", "s2", "--start", today, "--end", today, org)
	alice.ok("add", "finished", "--sprint", "s1", "-p", "points=3", org)
	alice.ok("add", "carried", "--sprint", "s1", "-p", "points=2", org)
	alice.ok("sprint", "start", "s1", org)
	sleepToNextSecond()
	alice.ok("add", "added late", "--sprint", "s1", "-p", "points=5", org)
	alice.ok("sync")
	bob.ok("sync")

	bob.ok("set", "--query", "finished", "status=done", "--yes", org)
	bob.ok("sync")
	alice.ok("sync")
	alice.ok("sprint", "close", "s1", "--to", "next", org)
	alice.ok("sync")
	bob.ok("sync")

	var b struct {
		Days []struct {
			Scope, Done, Remaining, Added float64
		}
	}
	if err := json.Unmarshal([]byte(bob.ok("report", "burndown", "s1", "--json", org)), &b); err != nil {
		t.Fatal(err)
	}
	if len(b.Days) != 1 {
		t.Fatalf("burndown = %+v", b)
	}
	if d := b.Days[0]; d.Scope != 10 || d.Done != 3 || d.Remaining != 7 || d.Added != 5 {
		t.Fatalf("burndown = %+v", d)
	}

	var v []struct {
		Points float64
		Tasks  int
	}
	if err := json.Unmarshal([]byte(bob.ok("report", "velocity", "--json", org)), &v); err != nil {
		t.Fatal(err)
	}
	if len(v) != 1 || v[0].Points != 3 || v[0].Tasks != 1 {
		t.Fatalf("velocity = %+v", v)
	}
}
