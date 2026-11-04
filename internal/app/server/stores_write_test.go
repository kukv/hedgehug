package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// remoteStore makes the store name, as someone else would, in a bare
// repository under a directory of its own: the directory (to look for stores
// under) and the repository.
func remoteStore(t *testing.T, name, prefix string) (root, bare string) {
	t.Helper()
	maker := usecasetest.Empty(t, time.Now)
	root = filepath.Join(t.TempDir(), "remotes")
	bare = filepath.Join(root, name+".git")
	if _, err := stores.StoreInit(maker.Session, stores.StoreInitInput{Name: name, Prefix: prefix, Remotes: []string{bare}}); err != nil {
		t.Fatal(err)
	}
	return root, bare
}

// storeRow is a store of GET /api/stores as the tests read it.
type storeRow struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Color    string `json:"color"`
	Hidden   bool   `json:"hidden"`
	ReadOnly string `json:"read_only"`
	Default  bool   `json:"default"`

	DefaultStore bool `json:"default_store"`
}

func (ts *testServer) storeRows() []storeRow {
	ts.t.Helper()
	var rows []storeRow
	ts.call("GET", "/api/stores", nil, 200, &rows)
	return rows
}

func TestStoreAddAPI(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ch, cancel := ts.srv.hub.subscribe()
	t.Cleanup(cancel)
	_, bare := remoteStore(t, "shared", "SH")

	var v stores.StoreView
	ts.call("POST", "/api/stores/add", map[string]any{"name": "shared", "remotes": []string{bare}}, 201, &v)
	if v.Name != "shared" || len(v.Remotes) != 1 || v.Remotes[0] != bare || v.Path == "" {
		t.Fatalf("added = %+v", v)
	}
	if _, err := os.Stat(v.Path); err != nil {
		t.Errorf("the clone is not at %s: %v", v.Path, err)
	}
	if rows := ts.storeRows(); len(rows) != 2 || rows[1].Name != "shared" {
		t.Errorf("stores = %+v", rows)
	}
	waitEvent[storesChanged](t, ch, nil)

	// A failure is a problem and tells nobody.
	if p := ts.problem("POST", "/api/stores/add", map[string]any{"name": "shared", "remotes": []string{bare}}, 409); p.Type != "store.exists" {
		t.Errorf("same name: %+v", p)
	}
	if p := ts.problem("POST", "/api/stores/add", map[string]any{"name": "x", "remotes": []string{}}, 400); p.Type != "request.invalid" {
		t.Errorf("no remote: %+v", p)
	}
	if p := ts.problem("POST", "/api/stores/add", map[string]any{"name": "Bad Name", "remotes": []string{bare}}, 400); p.Type != "store.bad_name" {
		t.Errorf("bad name: %+v", p)
	}
	if n := countStoresChanged(drainEvents(ch)); n != 0 {
		t.Errorf("%d stores.changed after failures", n)
	}
}

func TestStoreDeleteAPI(t *testing.T) {
	t.Parallel()
	ts, ch := pollServer(t)
	bare := filepath.Join(t.TempDir(), "team.git")
	// A store with a remote it pushed to can be removed with its clone.
	if _, err := stores.StoreInit(ts.session(), stores.StoreInitInput{Name: "team", Prefix: "TM", Remotes: []string{bare}}); err != nil {
		t.Fatal(err)
	}
	// And one with no remote cannot, unless the clone is kept.
	ts.call("POST", "/api/stores", map[string]any{"name": "local", "prefix": "LC"}, 201, nil)
	held := func(name string) bool {
		return holds(ts.srv.poller, name)
	}
	eventually(t, "the poller holds team and local", func() bool { return held("team") && held("local") })
	drainEvents(ch) // what the poller told of the two stores that came

	rows := ts.storeRows()
	var teamPath, localPath string
	for _, r := range rows {
		switch r.Name {
		case "team":
			teamPath = r.Path
		case "local":
			localPath = r.Path
		}
	}

	// Nothing is lost by the refusal: the store stays, held by the poller.
	if p := ts.problem("DELETE", "/api/stores/local", nil, 409); p.Type != "store.unpushed" || p.Args["name"] != "local" {
		t.Errorf("unpushed: %+v", p)
	}
	if _, err := os.Stat(localPath); err != nil {
		t.Errorf("a refused removal removed the clone: %v", err)
	}
	ts.problem("DELETE", "/api/stores/nope", nil, 404)

	// keep=true takes the store out of the config and leaves the clone.
	var res stores.StoreRemoveResult
	ts.call("DELETE", "/api/stores/local?keep=true", nil, 200, &res)
	if res.Name != "local" || res.Path != localPath || res.Deleted {
		t.Errorf("keep = %+v", res)
	}
	if _, err := os.Stat(localPath); err != nil {
		t.Errorf("keep=true removed the clone: %v", err)
	}
	waitEvent[storesChanged](t, ch, nil)

	// The default removes the clone, after the poller let go of it.
	ts.call("DELETE", "/api/stores/team", nil, 200, &res)
	if res.Name != "team" || res.Path != teamPath || !res.Deleted {
		t.Errorf("delete = %+v", res)
	}
	if _, err := os.Stat(teamPath); !os.IsNotExist(err) {
		t.Errorf("the clone is left: %v", err)
	}
	eventually(t, "the poller to let go of team", func() bool { return !held("team") })
	waitEvent[storesChanged](t, ch, nil)
	if rows := ts.storeRows(); len(rows) != 1 || rows[0].Name != "me" {
		t.Errorf("stores = %+v", rows)
	}
}

// A removal that is refused (409) lets go of the store only for a moment: the
// poller opens it again, and the clients are not told that anything changed.
func TestStoreDeleteRefusedIsQuiet(t *testing.T) {
	t.Parallel()
	ts, ch := pollServer(t)
	ts.call("POST", "/api/stores", map[string]any{"name": "local", "prefix": "LC"}, 201, nil)
	p := ts.srv.poller
	eventually(t, "the poller holds local", func() bool { return holds(p, "local") })
	drainEvents(ch)

	if pr := ts.problem("DELETE", "/api/stores/local", nil, 409); pr.Type != "store.unpushed" {
		t.Fatalf("problem = %+v", pr)
	}
	// The look that opens it again publishes before it keeps the store, so
	// once the store is held again, what it told is in ch.
	eventually(t, "the poller to open local again", func() bool { return holds(p, "local") })
	p.poll(true)
	for _, e := range drainEvents(ch) {
		if tc, ok := e.(tasksChanged); ok && tc.Store == "local" {
			t.Errorf("a refused removal announced %+v", tc)
		}
		if _, ok := e.(storesChanged); ok {
			t.Error("a refused removal announced stores.changed")
		}
	}
}

func TestStoreDefaultAPI(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ch, cancel := ts.srv.hub.subscribe()
	t.Cleanup(cancel)
	ts.call("POST", "/api/stores", map[string]any{"name": "team", "prefix": "TM"}, 201, nil)
	drainEvents(ch)

	var out struct {
		Name string `json:"name"`
	}
	ts.call("PUT", "/api/stores/default", map[string]any{"name": "team"}, 200, &out)
	if out.Name != "team" {
		t.Errorf("default = %+v", out)
	}
	// default_store marks the store config.yaml names. (`default` is the store
	// the CLI picks without --store, and with two stores that is none; see
	// TestStoreDefault.)
	for _, r := range ts.storeRows() {
		if r.DefaultStore != (r.Name == "team") {
			t.Errorf("row %+v; want only team to be default_store", r)
		}
	}
	waitEvent[storesChanged](t, ch, nil)

	if p := ts.problem("PUT", "/api/stores/default", map[string]any{"name": "nope"}, 404); p.Type != "store.not_found" {
		t.Errorf("unknown: %+v", p)
	}
	// An empty name would only read the default; the API wants one.
	if p := ts.problem("PUT", "/api/stores/default", map[string]any{"name": ""}, 400); p.Type != "request.invalid" {
		t.Errorf("empty: %+v", p)
	}
	if n := countStoresChanged(drainEvents(ch)); n != 0 {
		t.Errorf("%d stores.changed after failures", n)
	}
}

func TestStorePatchAPI(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ch, cancel := ts.srv.hub.subscribe()
	t.Cleanup(cancel)
	ts.call("POST", "/api/stores", map[string]any{"name": "team", "prefix": "TM"}, 201, nil)
	drainEvents(ch)

	row := func() storeRow {
		for _, r := range ts.storeRows() {
			if r.Name == "team" {
				return r
			}
		}
		t.Fatal("team is not listed")
		return storeRow{}
	}
	type patched struct {
		Name   string `json:"name"`
		Color  string `json:"color"`
		Hidden bool   `json:"hidden"`
	}
	var v patched

	ts.call("PATCH", "/api/stores/team", map[string]any{"color": "teal"}, 200, &v)
	if v.Name != "team" || v.Color != "teal" || v.Hidden {
		t.Errorf("color = %+v", v)
	}
	// GET /api/stores carries the same fields.
	if r := row(); r.Color != "teal" || r.Hidden {
		t.Errorf("listed = %+v", r)
	}
	waitEvent[storesChanged](t, ch, nil)

	// Only what is given changes: hidden leaves the color alone.
	ts.call("PATCH", "/api/stores/team", map[string]any{"hidden": true}, 200, &v)
	if v.Color != "teal" || !v.Hidden {
		t.Errorf("hidden = %+v", v)
	}
	waitEvent[storesChanged](t, ch, nil)

	// An empty color is given, and puts it back to automatic (the response
	// leaves an empty color out, so v starts again).
	v = patched{}
	ts.call("PATCH", "/api/stores/team", map[string]any{"color": "", "hidden": false}, 200, &v)
	if v.Color != "" || v.Hidden {
		t.Errorf("reset = %+v", v)
	}
	waitEvent[storesChanged](t, ch, nil)

	if p := ts.problem("PATCH", "/api/stores/team", map[string]any{}, 400); p.Type != "store.set_args" {
		t.Errorf("neither: %+v", p)
	}
	if p := ts.problem("PATCH", "/api/stores/nope", map[string]any{"hidden": true}, 404); p.Type != "store.not_found" {
		t.Errorf("unknown: %+v", p)
	}
	if n := countStoresChanged(drainEvents(ch)); n != 0 {
		t.Errorf("%d stores.changed after failures", n)
	}
}
