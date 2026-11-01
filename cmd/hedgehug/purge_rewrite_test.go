package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// newRewriteEnv is newMoveStoreEnv with a remote for alpha, after AA-2
// moved to bravo-long (as BB-3): alpha holds a forward file, and origin has
// it.
func newRewriteEnv(t *testing.T) (te *testEnv, remote, ulid string) {
	t.Helper()
	te = newTestEnv(t)
	remote = te.bare("alpha")
	te.ok("store", "init", "alpha", "--prefix", "AA", "--remote", remote)
	te.ok("store", "init", "bravo-long", "--prefix", "BB")
	p := te.addJSON("--store", "alpha", "Alpha project", "--kind", "project")
	child := te.addJSON("--store", "alpha", "Alpha child", "--parent", p.ID)
	te.ok("mv", "AA-2", "--store", "bravo-long", "--yes")
	te.ok("sync", "--store", "alpha")
	return te, remote, child.ULID
}

func remoteMain(t *testing.T, bare string) string {
	t.Helper()
	r, err := git.PlainOpen(bare)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Reference("refs/heads/main", true)
	if err != nil {
		t.Fatal(err)
	}
	return ref.Hash().String()
}

func TestPurgeRewriteHistoryNeedsATerminal(t *testing.T) {
	t.Parallel()
	te, remote, _ := newRewriteEnv(t)
	before := remoteMain(t, remote)
	te.goldenRun("without-yes", 2, "purge", "--rewrite-history", "AA-2", "--store", "alpha")
	// --yes does not go through (decision 17)
	te.goldenRun("with-yes", 2, "purge", "--rewrite-history", "AA-2", "--store", "alpha", "--yes")
	if got := remoteMain(t, remote); got != before {
		t.Errorf("origin moved to %s", got)
	}
}

func TestPurgeRewriteHistoryCanceled(t *testing.T) {
	t.Parallel()
	te, remote, _ := newRewriteEnv(t)
	before := remoteMain(t, remote)
	te.terminal = true
	commits := len(te.log("alpha"))
	for label, in := range map[string]string{"wrong-name": "bravo-long\n", "empty": "\n", "eof": ""} {
		te.stdin = in
		te.goldenRun(label, 1, "purge", "--rewrite-history", "AA-2", "--store", "alpha")
	}
	if got := remoteMain(t, remote); got != before || len(te.log("alpha")) != commits {
		t.Errorf("origin %s, %d commits; want %s, %d", got, len(te.log("alpha")), before, commits)
	}
}

func TestPurgeRewriteHistory(t *testing.T) {
	t.Parallel()
	te, remote, ulid := newRewriteEnv(t)
	te.terminal = true
	te.stdin = "alpha\n"
	te.goldenRun("rewritten", 0, "purge", "--rewrite-history", "AA-2")
	if got := remoteMain(t, remote); got != te.log("alpha")[0].Hash.String() {
		t.Errorf("origin = %s; want the head of alpha", got)
	}
	// nothing of the task is left in the files or the history
	for _, c := range te.log("alpha") {
		if strings.Contains(c.Message, "Alpha child") {
			t.Errorf("a message still has the title: %q", c.Message)
		}
		tree, err := c.Tree()
		if err != nil {
			t.Fatal(err)
		}
		err = tree.Files().ForEach(func(f *object.File) error {
			if strings.Contains(f.Name, ulid) {
				t.Errorf("%s in %s", f.Name, c.Hash)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if out := te.ok("ls", "--store", "alpha"); !strings.Contains(out, "Alpha project") || strings.Contains(out, "Alpha child") {
		t.Errorf("alpha:\n%s", out)
	}
	if out := te.ok("ls", "--store", "bravo-long"); !strings.Contains(out, "Alpha child") {
		t.Errorf("bravo-long:\n%s", out)
	}
	te.goldenRun("sync-after", 0, "sync", "--store", "alpha")
	// the task is gone for good: asking again finds nothing
	te.goldenRun("again", 1, "purge", "--rewrite-history", "AA-2", "--store", "alpha")
}

func TestPurgeRewriteHistoryJSON(t *testing.T) {
	t.Parallel()
	te, remote, _ := newRewriteEnv(t)
	te.terminal = true
	te.stdin = "alpha\n"
	out := te.ok("purge", "--rewrite-history", "AA-2", "--store", "alpha", "--json")
	te.golden("json", out)
	var got struct {
		Store string `json:"store"`
		Tasks []struct {
			ULID string `json:"ulid"`
			ID   string `json:"id"`
		} `json:"tasks"`
		Rewritten int      `json:"rewritten"`
		Pushed    []string `json:"pushed"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.Store != "alpha" || len(got.Tasks) != 1 || got.Tasks[0].ID != "AA-2" || got.Rewritten == 0 || len(got.Pushed) != 1 {
		t.Errorf("json = %+v", got)
	}
	if remoteMain(t, remote) != te.log("alpha")[0].Hash.String() {
		t.Error("origin is not at the head")
	}
	if _, err := os.Stat(filepath.Join(te.storeDir("alpha"), ".hedgehug", "local", "rewrite-in-progress.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("progress file: %v", err)
	}
}

func TestPurgeRewriteHistoryUsage(t *testing.T) {
	t.Parallel()
	te, _, _ := newRewriteEnv(t)
	te.goldenRun("no-id", 2, "purge", "--rewrite-history", "--store", "alpha")
	te.goldenRun("with-moved", 2, "purge", "--rewrite-history", "--moved", "AA-2", "--store", "alpha")
}
