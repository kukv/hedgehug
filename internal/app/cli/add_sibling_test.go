package cli

import (
	"strings"
	"testing"
)

// --after and --before without --parent put the task where the sibling is:
// the branch of the clone and the repository are not asked about.
func TestAddNextToASiblingTakesItsParent(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("Login work")
	other := te.addJSON("Other work")
	kid := te.addJSON("kid", "--parent", p.ID)
	te.cloneDir("hh/"+other.ID+"-x", "https://github.com/org-a/web")
	te.terminal = true
	te.stdin = ""
	code, out, errs := te.run("add", "next", "--after", kid.ID)
	if code != 0 || strings.Contains(errs, "Add as a child") || !strings.HasSuffix(out, "→ added to me / Login work\n") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errs)
	}
	got := te.addJSON("before", "--before", kid.ID)
	if got.Parent == nil || *got.Parent != p.ULID {
		t.Errorf("--before: parent = %v; want %s", got.Parent, p.ULID)
	}
	// a sibling at the root: the root, though the branch names a task
	root := te.addJSON("beside", "--after", other.ID)
	if root.Parent != nil {
		t.Errorf("a root sibling: parent = %v; want none", root.Parent)
	}
	// a --workspace that is not the sibling's is add.position, though the sibling has a parent
	te.ok("workspace", "add", "second", "--prefix", "SC")
	code, _, errs = te.run("add", "elsewhere", "--workspace", "second", "--after", kid.ID)
	if code == 0 || !strings.Contains(errs, "sibling") {
		t.Errorf("a workspace that does not fit: exit %d, stderr %q", code, errs)
	}
	// an explicit --parent that does not fit the sibling is add.position
	code, _, errs = te.run("add", "bad", "--parent", other.ID, "--after", kid.ID)
	if code == 0 || !strings.Contains(errs, "sibling") {
		t.Errorf("a parent that does not fit: exit %d, stderr %q", code, errs)
	}
}
