package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// inCodeRepo makes a work tree with one commit on main the directory of the
// runs of te.
func (te *testEnv) inCodeRepo() {
	te.t.Helper()
	te.dir = te.t.TempDir()
	_, err := git.PlainInitWithOptions(te.dir, &git.PlainInitOptions{
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	if err != nil {
		te.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(te.dir, "a.txt"), []byte("a\n"), 0o600); err != nil {
		te.t.Fatal(err)
	}
	gitrepotest.CommitFiles(te.t, te.dir, "first", "t", "t@example.com", time.Unix(1_700_000_000, 0), "a.txt")
}

func TestStartCommand(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "Fix the login")
	te.ok("add", "b")
	te.inCodeRepo()
	if out := te.ok("start", "ME-1"); !strings.HasPrefix(out, "Started ME-1 ") || !strings.HasSuffix(out, " Fix the login\nOn branch hh/ME-1-fix-the-login\n") {
		t.Errorf("start:\n%s", out)
	}
	if out := te.ok("start", "ME-1"); !strings.HasPrefix(out, "No changes ME-1 ") || !strings.HasSuffix(out, " Fix the login\nOn branch hh/ME-1-fix-the-login\n") {
		t.Errorf("start again:\n%s", out)
	}
	// ME-2 has no branch, and starting it moves the developer on.
	if out := te.ok("start", "ME-2"); !strings.Contains(out, "On branch hh/ME-2-b\n") {
		t.Errorf("start ME-2:\n%s", out)
	}
	te.ok("status", "ME-1", "todo")
	if out := te.ok("start", "ME-1"); !strings.Contains(out, "git switch hh/ME-1-fix-the-login\n") {
		t.Errorf("start with the branch elsewhere:\n%s", out)
	}
}

func TestStartOutsideARepositoryExits1(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "a")
	te.dir = t.TempDir()
	if code, _, errs := te.run("start", "ME-1"); code != 1 || !strings.Contains(errs, "not in a git repository") {
		t.Errorf("exit %d: %s", code, errs)
	}
}

func TestStartJSON(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "a")
	te.inCodeRepo()
	var w integration.StartWrite
	strictly(t, te.ok("start", "ME-1", "--json"), &w)
	if w.Branch != "hh/ME-1-a" || !w.Created || w.Switch || w.Task.Status != "in_progress" || w.Commit == nil {
		t.Errorf("json = %+v", w)
	}
}
