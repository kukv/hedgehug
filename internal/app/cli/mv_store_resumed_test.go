package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unfinishedMove stops the move of AA-1 (with AA-2 under it) to bravo-long
// after the destination was written: the directory of the source file of
// AA-2 cannot be written to, so the source cannot drop the file.
func unfinishedMove(t *testing.T, te *testEnv) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("a directory that cannot be written to needs a permission that is enforced")
	}
	path, _ := te.storeTaskFile("alpha", "AA-2")
	dir := filepath.Dir(path)
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, fi.Mode().Perm()) }) // so that the directory of the test can be removed
	code, _, errs := te.run("mv", "AA-1", "--store", "bravo-long", "--yes")
	if err := os.Chmod(dir, fi.Mode().Perm()); err != nil {
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
