package sync

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/github"
)

// A store that is read-only by its GitHub permission is judged again by an
// explicit sync, whatever infoInterval says, so the permission that came back
// lets the same hh sync push. A background sync keeps the interval.
func TestExplicitSyncRechecksAReadOnlyStore(t *testing.T) {
	fake := fakegh.New(fakegh.State{
		Users:  []string{"alice"},
		Tokens: map[string]string{"t": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "x", Perms: map[string]string{"alice": "read"}}},
	})
	srv := httptest.NewServer(fake)
	defer srv.Close()
	c := newCluster(t, 1)
	c.addTask(0, "local")
	now := time.Now().Add(time.Hour)
	opt := c.opts[0]
	opt.Now = func() time.Time { return now }
	opt.GitHub = &GitHubRepo{
		Host: "github.com", Owner: "org-a", Name: "x", AccountID: "github.com/alice", AccountName: "me",
		Client: func() (*github.Client, error) { return github.New(config.Host{API: srv.URL}, "t", nil), nil },
	}
	if a, _, err := CheckAccess(c.stores[0], opt, now); err != nil || !a.ReadOnly || a.Reason != ReasonGitHub {
		t.Fatalf("setup access = %+v %v", a, err)
	}
	if err := fake.SetPerm("org-a/x", "alice", "write"); err != nil {
		t.Fatal(err)
	}

	// Within the interval, a background sync still takes the store as read-only.
	now = now.Add(time.Minute)
	res, err := Run(c.stores[0], opt)
	if err != nil || len(res.Pushed) != 0 || res.Unpushable != 1 {
		t.Fatalf("background sync = %+v %v", res, err)
	}

	opt.Explicit = true
	res, err = Run(c.stores[0], opt)
	if err != nil || len(res.Pushed) != 1 || res.Unpushable != 0 {
		t.Fatalf("explicit sync = %+v %v; want it pushed", res, err)
	}
	if a, _ := ReadAccess(c.stores[0]); a.ReadOnly {
		t.Errorf("access after the explicit sync = %+v", a)
	}
	if n, err := Unpushed(c.stores[0].Repo()); err != nil || n != 0 {
		t.Errorf("Unpushed = %d, %v", n, err)
	}
}
