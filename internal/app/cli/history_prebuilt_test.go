package cli

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// requireHistoryAtHead fails unless the saved history index of the store is
// there and already at HEAD: opening it again changes nothing on disk.
func requireHistoryAtHead(t *testing.T, te *testEnv, name string) {
	t.Helper()
	te.uc().HistoryAtHead(name)
}

// store add builds the history index of the store it cloned.
func TestStoreAddBuildsTheHistoryIndex(t *testing.T) {
	t.Parallel()
	_, bob, _ := twoUsers(t)
	requireHistoryAtHead(t, bob, "org")
}

// doctor --fix builds the history index of the store it cloned again.
func TestDoctorFixBuildsTheHistoryIndex(t *testing.T) {
	t.Parallel()
	_, bob, remote := twoUsers(t)
	bob.ok("add", "bobs")
	gitrepotest.RewriteRemote(t, remote)
	bob.run("sync")
	if out := bob.ok("doctor", "--fix"); !strings.Contains(out, "Cloned org again") {
		t.Fatalf("doctor --fix:\n%s", out)
	}
	requireHistoryAtHead(t, bob, "org")
}
