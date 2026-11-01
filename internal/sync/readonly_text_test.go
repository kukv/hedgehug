package sync

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
)

// The error names the store first and says the reason once; the push error
// stays reachable for errors.Is.
func TestPushDeniedErrorText(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	c.addTask(0, "local")
	gitrepotest.LockObjects(t, c.remote)
	opt := c.opts[0]
	opt.Name = "org-a"
	_, err := Run(c.stores[0], opt)
	if err == nil || err.Error() != "org-a is read-only (the remote denied the push)" {
		t.Fatalf("err = %v", err)
	}
	if !errors.Is(err, gitrepo.ErrPushDenied) {
		t.Errorf("errors.Is(err, ErrPushDenied) = false")
	}
	opt.Name = ""
	opt.Explicit = true // the denial is tried again
	if _, err := Run(c.stores[0], opt); err == nil || err.Error() != "the store is read-only (the remote denied the push)" {
		t.Errorf("without a name: %v", err)
	}
}

// An explicit sync clears push_denied before it fetches; a fetch that fails
// leaves it cleared and returns the fetch error.
func TestExplicitSyncClearsPushDeniedBeforeFetch(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	if err := WriteAccess(c.stores[0], Access{ReadOnly: true, Reason: ReasonPushDenied, Checked: time.Now()}); err != nil {
		t.Fatal(err)
	}
	opt := c.opts[0]
	opt.Explicit = true
	opt.Remotes = []string{filepath.Join(t.TempDir(), "gone.git")}
	_, err := Run(c.stores[0], opt)
	if err == nil || errors.Is(err, gitrepo.ErrPushDenied) {
		t.Fatalf("err = %v, want the fetch error", err)
	}
	if a, _ := ReadAccess(c.stores[0]); a.ReadOnly || a.Reason != "" {
		t.Errorf("access = %+v, want push_denied cleared", a)
	}
}
