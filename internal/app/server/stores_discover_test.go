package server

import (
	"net/url"
	"os"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/accounts"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
)

func TestStoreDiscoverAndMountAPI(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ch, cancel := ts.srv.hub.subscribe()
	t.Cleanup(cancel)
	root, bare := remoteStore(t, "shared", "SH")
	from := "/api/stores/discover?from=" + url.QueryEscape(root)

	// Without --from the accounts look on GitHub, and the only account has no
	// token (no network is used).
	if p := ts.problem("GET", "/api/stores/discover", nil, 409); p.Type != "account.none" {
		t.Errorf("no token: %+v", p)
	}

	var cands []stores.Candidate
	ts.call("GET", from, nil, 200, &cands)
	if len(cands) != 1 || cands[0].Name != "shared" || cands[0].URL != bare || cands[0].Account != "alice" || cands[0].Permission != "write" {
		t.Fatalf("candidates = %+v", cands)
	}
	var none []stores.Candidate
	ts.call("GET", "/api/stores/discover?from="+url.QueryEscape(t.TempDir()), nil, 200, &none)
	if none == nil || len(none) != 0 {
		t.Errorf("an empty directory = %#v; want []", none)
	}

	// The name can be changed on the way: it is the one the candidate is mounted
	// under. One that cannot be mounted does not stop the rest.
	body := []map[string]any{
		{"name": "shared-2", "url": cands[0].URL, "account": cands[0].Account},
		{"name": "gone", "url": bare + "-missing"},
	}
	var res struct {
		Mounted []string `json:"mounted"`
		Failed  []struct {
			Name    string `json:"name"`
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"failed"`
	}
	ts.call("POST", "/api/stores/mount", body, 200, &res)
	if len(res.Mounted) != 1 || res.Mounted[0] != "shared-2" || len(res.Failed) != 1 || res.Failed[0].Name != "gone" || res.Failed[0].Code != "store.clone_failed" || res.Failed[0].Message == "" {
		t.Fatalf("mount = %+v", res)
	}
	if rows := ts.storeRows(); len(rows) != 2 || rows[1].Name != "shared-2" {
		t.Errorf("stores = %+v", rows)
	}
	if _, err := os.Stat(ts.storeRows()[1].Path); err != nil {
		t.Errorf("the clone of shared-2 is not there: %v", err)
	}
	if n := countStoresChanged(drainEvents(ch)); n != 1 {
		t.Errorf("%d stores.changed after a mount of one, want 1", n)
	}

	// The same name again is a failure of that row, with the code of the problem.
	ts.call("POST", "/api/stores/mount", body[:1], 200, &res)
	if len(res.Mounted) != 0 || len(res.Failed) != 1 || res.Failed[0].Code != "store.exists" {
		t.Errorf("again = %+v", res)
	}
	if n := countStoresChanged(drainEvents(ch)); n != 0 {
		t.Errorf("%d stores.changed after a mount of none", n)
	}
	ts.problem("POST", "/api/stores/mount", []map[string]any{}, 400)
}

// With --from, and two accounts, and none named, the candidates have no
// account to take: the answer is the choice, 422, with the names.
func TestStoreDiscoverNeedsAnAccount(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	root, _ := remoteStore(t, "shared", "SH")
	if _, err := accounts.AuthAdd(ts.session(), accounts.AuthAddInput{Name: "bob", Login: "bob"}); err != nil {
		t.Fatal(err)
	}
	from := "/api/stores/discover?from=" + url.QueryEscape(root)
	p := ts.problem("GET", from, nil, 422)
	cands, _ := p.Args["candidates"].([]any)
	if p.Type != "choice.required" || p.Args["what"] != "--account" || len(cands) != 2 || cands[0] != "alice" || cands[1] != "bob" {
		t.Errorf("problem = %+v", p)
	}
	var out []stores.Candidate
	ts.call("GET", from+"&account=bob", nil, 200, &out)
	if len(out) != 1 || out[0].Account != "bob" {
		t.Errorf("with the account named = %+v", out)
	}
}
