package cli

import (
	"os"
	"strings"
	"testing"
)

// A swap that stopped (the store moved away, the rebuilt one not yet in its
// place) is finished by doctor --fix.
func TestDoctorFixFinishesAStoppedRewriteSwap(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("add", "Alpha")
	dir := te.storeDir("me")
	if err := os.CopyFS(dir+".rewrite-new", os.DirFS(dir)); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(dir, dir+".before-rewrite-20260101T000000.000000000"); err != nil {
		t.Fatal(err)
	}

	if code, out, _ := te.run("doctor"); code == 0 || strings.Contains(out, "No problems") {
		t.Errorf("doctor of a store that is not there: exit %d\n%s", code, out)
	}
	if code, out, stderr := te.run("doctor", "--fix"); code != 0 || !strings.Contains(out, "No problems") {
		t.Errorf("--fix: exit %d\nstdout %s\nstderr %s", code, out, stderr)
	}
	if err := te.uc().NoLeftover(dir); err != nil {
		t.Errorf("leftover = %v", err)
	}
	if out := te.ok("ls"); !strings.Contains(out, "Alpha") {
		t.Errorf("ls after the fix:\n%s", out)
	}
}
