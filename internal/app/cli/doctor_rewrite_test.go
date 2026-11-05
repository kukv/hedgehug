package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// A rebuild that stopped is finished by doctor --fix; until then the store
// does not open.
func TestDoctorFixFinishesAStoppedRebuild(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("add", "Alpha")
	dir := te.storeDir("me")
	h := gitrepotest.HeadHash(t, dir)
	rec := `{"phase":"apply","old_head":"` + h + `","target":"` + h + `","origin":"` + h + `","mark":"` + h + `"}`
	if err := os.WriteFile(filepath.Join(dir, ".hedgehug", "local", "rebuild.json"), []byte(rec), 0o600); err != nil {
		t.Fatal(err)
	}

	if code, out, _ := te.run("doctor"); code == 0 || strings.Contains(out, "No problems") {
		t.Errorf("doctor of a store that stopped: exit %d\n%s", code, out)
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
