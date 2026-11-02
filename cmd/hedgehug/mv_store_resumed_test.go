package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// unfinishedMove stops the move of AA-1 (with AA-2 under it) to bravo-long
// after the destination was written: the source file of AA-2 cannot be read.
func unfinishedMove(t *testing.T, te *testEnv) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("a file that cannot be read needs a permission that is enforced")
	}
	path, _ := te.storeTaskFile("alpha", "AA-2")
	calls := 0
	te.beforeUpdate = func() {
		if calls++; calls == 2 { // the write into the source
			_ = os.Chmod(path, 0)
		}
	}
	code, _, errs := te.run("mv", "AA-1", "--store", "bravo-long", "--yes")
	te.beforeUpdate = nil
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if code == 0 || !strings.Contains(errs, "moved into bravo-long; run the same command again") {
		t.Fatalf("the first run: exit %d, stderr %q", code, errs)
	}
}

// A rerun that only finishes the source says so before the Moved lines.
func TestMvStoreRerunSaysItFinishedTheMove(t *testing.T) {
	t.Parallel()
	te := newMoveStoreEnv(t)
	unfinishedMove(t, te)
	out := te.ok("mv", "AA-1", "--store", "bravo-long", "--yes")
	first, _, _ := strings.Cut(out, "\n")
	if first != "Finished the move: alpha was left unfinished last time" || strings.Count(out, "Moved ") != 2 {
		t.Errorf("out = %q", out)
	}
	// a move that did not stop says nothing of the kind
	te2 := newMoveStoreEnv(t)
	if out := te2.ok("mv", "AA-1", "--store", "bravo-long", "--yes"); strings.Contains(out, "Finished") {
		t.Errorf("a fresh move says it finished one: %q", out)
	}
}

// --json has resumed: true after a finished move, and no key otherwise.
func TestMvStoreRerunJSONResumed(t *testing.T) {
	t.Parallel()
	te := newMoveStoreEnv(t)
	unfinishedMove(t, te)
	out := te.ok("mv", "AA-1", "--store", "bravo-long", "--yes", "--json")
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got["resumed"] != true {
		t.Errorf("json = %s; want resumed: true", out)
	}
	te2 := newMoveStoreEnv(t)
	out = te2.ok("mv", "AA-1", "--store", "bravo-long", "--yes", "--json")
	if strings.Contains(out, "resumed") {
		t.Errorf("a fresh move has the key resumed: %s", out)
	}
}
