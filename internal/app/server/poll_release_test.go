package server

import (
	"testing"
)

// The session the server gives lets the poller go of a store before
// doctor --fix rebuilds it (D26); the poller then opens the store again and
// tells the clients that its tasks changed.
func TestSessionBeforeRebuildReleasesTheStore(t *testing.T) {
	t.Parallel()
	ts, ch := pollServer(t)
	held := func() bool {
		stores, _ := ts.srv.poller.snapshot()
		for _, st := range stores {
			if st.Store == "me" {
				return true
			}
		}
		return false
	}
	if !held() {
		t.Fatal("the poller does not hold the store")
	}

	before := ts.srv.session("").BeforeRebuild
	if before == nil {
		t.Fatal("the server's session has no BeforeRebuild")
	}
	before(ts.uc.StoreDir("me"))

	ev := waitEvent[tasksChanged](t, ch, func(x tasksChanged) bool { return x.Store == "me" })
	if len(ev.ULIDs) != 0 || len(ev.Removed) != 0 {
		t.Errorf("event = %+v", ev)
	}
	eventually(t, "the poller holds the store opened again", held)
}
