package ops

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

func TestWorkspaceAddRenamePrefixRemove(t *testing.T) {
	ts := newTestSession(t)
	r, err := ts.WorkspaceAdd("web", "WEB")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Committed || r.Workspace.Name != "web" || r.Workspace.IDPrefix != "WEB" || !r.Workspace.PrefixSet.Equal(ts.Now()) || r.Index == nil {
		t.Errorf("WorkspaceAdd = %+v (%+v)", r, r.Workspace)
	}
	if _, err := ts.Add(AddInput{Title: "a", Workspace: "web"}); err != nil {
		t.Fatal(err)
	}
	r, err = ts.WorkspaceRename("web", "front")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Committed || r.OldName != "web" || r.Workspace.Name != "front" || r.Workspace.IDPrefix != "WEB" {
		t.Errorf("WorkspaceRename = %+v (%+v)", r, r.Workspace)
	}
	r, err = ts.WorkspacePrefix("front", "FE")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Committed || r.Renumbered != 1 || !slices.Equal(r.IDs, []IDChange{{Old: "WEB-1", New: "FE-1"}}) || r.Workspace.IDPrefix != "FE" {
		t.Errorf("WorkspacePrefix = %+v (%+v)", r, r.Workspace)
	}
	if WorkspaceViewOf(r.Index, r.Workspace).Tasks != 1 {
		t.Errorf("the refreshed index counts %d tasks; want 1", WorkspaceViewOf(r.Index, r.Workspace).Tasks)
	}
	_, ix, _ := ts.Open()
	if e, err := ts.Resolve(ix, "FE-1"); err != nil || e.Title != "a" {
		t.Errorf("FE-1 after the prefix change: %v, %v", e, err)
	}
	if e, err := ts.Resolve(ix, "WEB-1"); err != nil || e.ID != "FE-1" {
		t.Errorf("the former id WEB-1 resolves to %v, %v; want FE-1", e, err)
	}
	_, err = ts.WorkspaceRemove("front")
	wantErr(t, err, KindConflict, "workspace.not_empty")
	if want := "workspace front still has 1 tasks; move them with hedgehug mv --workspace or purge them"; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	ts.addWorkspace("empty", "EM")
	r, err = ts.WorkspaceRemove("EM")
	if err != nil || !r.Committed || r.Workspace.Name != "empty" {
		t.Fatalf("WorkspaceRemove = %+v, %v", r, err)
	}
	if _, wss, _ := ts.WorkspaceList(); len(wss) != 2 {
		t.Errorf("workspaces after the removal = %v; want me and front", wss)
	}
}

func TestWorkspaceBadNameAndPrefixAreUsageErrors(t *testing.T) {
	ts := newTestSession(t)
	_, err := ts.WorkspaceAdd("web", "w-b")
	wantErr(t, err, KindUsage, "workspace.bad_prefix")
	if e := new(Error); !asError(err, &e) || e.Args["value"] != "w-b" {
		t.Errorf("args = %v; want the prefix as value", e)
	}
	_, err = ts.WorkspaceAdd("", "WEB")
	wantErr(t, err, KindUsage, "workspace.bad_name")
	_, err = ts.WorkspaceRename("me", "")
	wantErr(t, err, KindUsage, "workspace.bad_name")
	_, err = ts.WorkspacePrefix("me", "x y")
	wantErr(t, err, KindUsage, "workspace.bad_prefix")
	if _, wss, _ := ts.WorkspaceList(); len(wss) != 1 {
		t.Errorf("a refused call wrote: %v", wss)
	}
}

func TestWorkspaceNameAndPrefixTakenInTheStore(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.WorkspaceAdd("web", "WEB"); err != nil {
		t.Fatal(err)
	}
	_, err := ts.WorkspaceAdd("web2", "WEB")
	wantErr(t, err, KindConflict, "")
	if c := classify(err); c == nil || c.Code != "workspace.taken" {
		t.Errorf("classify(%v) = %+v; want workspace.taken", err, c)
	}
	if want := "the id prefix WEB is already used by workspace web"; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	_, err = ts.WorkspaceAdd("web", "XX")
	wantErr(t, err, KindConflict, "")
	if want := "a workspace named web already exists"; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
}

func TestWorkspaceNotFoundAndAmbiguous(t *testing.T) {
	ts := newTestSession(t)
	_, err := ts.WorkspaceRename("nope", "x")
	wantErr(t, err, KindNotFound, "workspace.not_found")
	if e := new(Error); !asError(err, &e) || e.Args["input"] != "nope" {
		t.Errorf("args = %v; want the input", e)
	}
	_, err = ts.WorkspaceRemove("nope")
	wantErr(t, err, KindNotFound, "workspace.not_found")
	if _, err := ts.WorkspaceAdd("api", "BE"); err != nil {
		t.Fatal(err)
	}
	be, err := ts.WorkspaceAdd("be", "API")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ts.WorkspacePrefix("api", "ZZ")
	wantErr(t, err, KindAmbiguous, "workspace.ambiguous")
	if !strings.HasPrefix(err.Error(), `"api" matches 2 workspaces:`+"\n  api (BE, ") || !strings.Contains(err.Error(), "\n  be (API, "+be.Workspace.ULID.String()+")") {
		t.Errorf("message = %q", err)
	}
}

func TestWorkspacePrefixUnchangedWritesNothing(t *testing.T) {
	ts := newTestSession(t)
	s, _, _ := ts.Open()
	before, _ := s.Repo().Head()
	r, err := ts.WorkspacePrefix("me", "ME")
	if err != nil || r.Committed || r.Workspace.IDPrefix != "ME" || r.Renumbered != 0 {
		t.Fatalf("WorkspacePrefix = %+v, %v; want no change", r, err)
	}
	if after, _ := s.Repo().Head(); after != before {
		t.Error("the unchanged prefix was committed")
	}
}

func TestWorkspacePrefixRenumbersDeletedTasksToo(t *testing.T) {
	ts := newTestSession(t)
	for _, title := range []string{"a", "b", "c"} {
		if _, err := ts.Add(AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	ts.deleteTask("ME-2")
	r, err := ts.WorkspacePrefix("me", "NEW")
	if err != nil {
		t.Fatal(err)
	}
	want := []IDChange{{"ME-1", "NEW-1"}, {"ME-2", "NEW-2"}, {"ME-3", "NEW-3"}}
	if r.Renumbered != 3 || !slices.Equal(r.IDs, want) {
		t.Errorf("IDs = %v (%d); want %v", r.IDs, r.Renumbered, want)
	}
}

func TestWorkspaceListCountsTasksNotDeleted(t *testing.T) {
	ts := newTestSession(t)
	ts.addWorkspace("api", "API")
	if _, err := ts.Add(AddInput{Title: "a", Workspace: "me"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Add(AddInput{Title: "b", Workspace: "me"}); err != nil {
		t.Fatal(err)
	}
	ts.deleteTask("ME-2")
	ix, wss, err := ts.WorkspaceList()
	if err != nil || ix == nil || len(wss) != 2 {
		t.Fatalf("WorkspaceList = %v, %v", wss, err)
	}
	// by name: api, me
	if wss[0].Name != "api" || wss[0].IDPrefix != "API" || wss[0].Tasks != 0 || wss[1].Name != "me" || wss[1].Tasks != 1 {
		t.Errorf("rows = %+v", wss)
	}
	if _, err := model.ParseULID(wss[0].ULID); err != nil {
		t.Errorf("ULID %q: %v", wss[0].ULID, err)
	}
}

func TestCheckPrefixElsewhere(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.StoreInit(StoreInitInput{Name: "team", Prefix: "TM"}); err != nil {
		t.Fatal(err)
	}
	err := ts.CheckPrefixElsewhere("TM", "me")
	wantErr(t, err, KindConflict, "workspace.prefix_taken")
	if e := new(Error); !asError(err, &e) || e.Args["prefix"] != "TM" {
		t.Errorf("args = %v; want the prefix", e)
	}
	if want := "id prefix TM is used by workspace team of store team"; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	if err := ts.CheckPrefixElsewhere("TM", "team"); err != nil {
		t.Errorf("a store's own prefix is refused: %v", err)
	}
	if err := ts.CheckPrefixElsewhere("ZZ", "me"); err != nil {
		t.Errorf("a free prefix is refused: %v", err)
	}
	// The workspace operations on the store me refuse it as well.
	me := &Session{Getenv: ts.Getenv, Now: ts.Now, Warn: ts.Warn, StoreName: "me"}
	_, err = me.WorkspaceAdd("other", "TM")
	wantErr(t, err, KindConflict, "workspace.prefix_taken")
	_, err = me.WorkspacePrefix("me", "TM")
	wantErr(t, err, KindConflict, "workspace.prefix_taken")
}

func TestCheckPrefixElsewhereSkipsABrokenStoreWithAWarning(t *testing.T) {
	ts := newTestSession(t)
	res, err := ts.StoreInit(StoreInitInput{Name: "other", Prefix: "OT"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(res.Path, model.StoreMetaPath), []byte("not: [yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ts.warns = nil
	if err := ts.CheckPrefixElsewhere("WEB", "me"); err != nil {
		t.Fatal(err)
	}
	if len(ts.warns) != 1 || !strings.HasPrefix(ts.warns[0], "store other is not checked for id prefixes: ") {
		t.Errorf("warns = %q", ts.warns)
	}
}

func TestWorkspaceOnANamedStoreDoesNotTouchTheSession(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.StoreInit(StoreInitInput{Name: "team", Prefix: "TM"}); err != nil {
		t.Fatal(err)
	}
	sub := &Session{Getenv: ts.Getenv, Now: ts.Now, Warn: ts.Warn, StoreName: "team"}
	if _, err := sub.WorkspaceAdd("api", "API"); err != nil {
		t.Fatal(err)
	}
	if ts.StoreName != "" {
		t.Errorf("StoreName = %q; want it untouched", ts.StoreName)
	}
	me := &Session{Getenv: ts.Getenv, Now: ts.Now, Warn: ts.Warn, StoreName: "me"}
	if _, wss, _ := me.WorkspaceList(); len(wss) != 1 {
		t.Errorf("the store me has %d workspaces; want 1", len(wss))
	}
	if _, wss, _ := sub.WorkspaceList(); len(wss) != 2 {
		t.Errorf("the store team has %d workspaces; want 2", len(wss))
	}
}
