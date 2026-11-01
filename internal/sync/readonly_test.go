package sync

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
)

func TestReadOnlySyncDoesNotPush(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	c.addTask(0, "local") // a commit that origin lacks
	if err := WriteAccess(c.stores[0], Access{ReadOnly: true, Reason: ReasonGitHub, Checked: time.Now()}); err != nil {
		t.Fatal(err)
	}
	res := c.sync(0)
	if len(res.Pushed) != 0 || res.Unpushable != 1 {
		t.Errorf("pushed %v, unpushable %d", res.Pushed, res.Unpushable)
	}
	if n, err := Unpushed(c.stores[0].Repo()); err != nil || n != 1 {
		t.Errorf("Unpushed = %d, %v; want 1", n, err)
	}
}

func TestPushDeniedMakesReadOnly(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	c.addTask(0, "local")
	unlock := gitrepotest.LockObjects(t, c.remote)
	_, err := Run(c.stores[0], c.opts[0])
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("err = %v", err)
	}
	if a, _ := ReadAccess(c.stores[0]); !a.ReadOnly || a.Reason != ReasonPushDenied {
		t.Errorf("access = %+v", a)
	}
	unlock()
	opt := c.opts[0]
	opt.Explicit = true // hh sync tries the push again
	if _, err := Run(c.stores[0], opt); err != nil {
		t.Fatal(err)
	}
	if a, _ := ReadAccess(c.stores[0]); a.ReadOnly {
		t.Errorf("access after a push that went = %+v", a)
	}
}

// Without Explicit (the background sync), a push_denied stays.
func TestPushDeniedStaysForBackgroundSync(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	c.addTask(0, "local")
	unlock := gitrepotest.LockObjects(t, c.remote)
	if _, err := Run(c.stores[0], c.opts[0]); err == nil {
		t.Fatal("want an error")
	}
	unlock()
	res, err := Run(c.stores[0], c.opts[0])
	if err != nil || len(res.Pushed) != 0 || res.Unpushable != 1 {
		t.Errorf("background sync = %+v %v", res, err)
	}
}
