package workspaces_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/app/usecase/workspaces"
)

func TestWorkspaceDeletedCount(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddWorkspace("docs", "DOC")
	var last string
	for _, title := range []string{"a", "b", "c"} {
		r, err := tasks.Add(ts.Session, tasks.AddInput{Title: title, Workspace: "docs"})
		if err != nil {
			t.Fatal(err)
		}
		last = r.ULID.String()
	}
	if _, err := tasks.Remove(ts.Session, last); err != nil {
		t.Fatal(err)
	}
	_, wss, err := workspaces.WorkspaceList(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	var docs *workspaces.WorkspaceView
	for i := range wss {
		if wss[i].Name == "docs" {
			docs = &wss[i]
		}
	}
	if docs == nil || docs.Tasks != 2 || docs.Deleted != 1 {
		t.Fatalf("docs = %+v; want 2 tasks and 1 deleted", docs)
	}

	// the refusal counts the deleted ones too, and gives the number
	_, err = workspaces.WorkspaceRemove(ts.Session, "docs")
	usecasetest.WantErr(t, err, usecase.KindConflict, "workspace.not_empty")
	if n, ok := usecase.AsError(err).Args["tasks"].(int); !ok || n != 3 {
		t.Errorf("args = %#v; want tasks = 3 (deleted included)", usecase.AsError(err).Args)
	}
}

func TestWorkspaceAddRenamePrefixRemove(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	r, err := workspaces.WorkspaceAdd(ts.Session, "web", "WEB")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Committed || r.Workspace.Name != "web" || r.Workspace.IDPrefix != "WEB" || !r.Workspace.PrefixSet.Equal(ts.Now()) || r.Index == nil {
		t.Errorf("WorkspaceAdd = %+v (%+v)", r, r.Workspace)
	}
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a", Workspace: "web"}); err != nil {
		t.Fatal(err)
	}
	r, err = workspaces.WorkspaceRename(ts.Session, "web", "front")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Committed || r.OldName != "web" || r.Workspace.Name != "front" || r.Workspace.IDPrefix != "WEB" {
		t.Errorf("WorkspaceRename = %+v (%+v)", r, r.Workspace)
	}
	r, err = workspaces.WorkspacePrefix(ts.Session, "front", "FE")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Committed || r.Renumbered != 1 || !slices.Equal(r.IDs, []workspaces.IDChange{{Old: "WEB-1", New: "FE-1"}}) || r.Workspace.IDPrefix != "FE" {
		t.Errorf("WorkspacePrefix = %+v (%+v)", r, r.Workspace)
	}
	if workspaces.WorkspaceViewOf(r.Index, r.Workspace).Tasks != 1 {
		t.Errorf("the refreshed index counts %d tasks; want 1", workspaces.WorkspaceViewOf(r.Index, r.Workspace).Tasks)
	}
	_, ix, _ := ts.Open()
	if e, err := ts.Resolve(ix, "FE-1"); err != nil || e.Title != "a" {
		t.Errorf("FE-1 after the prefix change: %v, %v", e, err)
	}
	if e, err := ts.Resolve(ix, "WEB-1"); err != nil || e.ID != "FE-1" {
		t.Errorf("the former id WEB-1 resolves to %v, %v; want FE-1", e, err)
	}
	_, err = workspaces.WorkspaceRemove(ts.Session, "front")
	usecasetest.WantErr(t, err, usecase.KindConflict, "workspace.not_empty")
	if want := "workspace front still has 1 task; move it with hedgehug mv --workspace, or delete it with hedgehug rm and remove it with hedgehug purge --older-than 0d --store <store>"; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	ts.AddWorkspace("empty", "EM")
	r, err = workspaces.WorkspaceRemove(ts.Session, "EM")
	if err != nil || !r.Committed || r.Workspace.Name != "empty" {
		t.Fatalf("WorkspaceRemove = %+v, %v", r, err)
	}
	if _, wss, _ := workspaces.WorkspaceList(ts.Session); len(wss) != 2 {
		t.Errorf("workspaces after the removal = %v; want me and front", wss)
	}
}

func TestWorkspaceBadNameAndPrefixAreUsageErrors(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	_, err := workspaces.WorkspaceAdd(ts.Session, "web", "w-b")
	usecasetest.WantErr(t, err, usecase.KindUsage, "workspace.bad_prefix")
	if e := new(usecase.Error); !usecasetest.AsError(err, &e) || e.Args["value"] != "w-b" {
		t.Errorf("args = %v; want the prefix as value", e)
	}
	_, err = workspaces.WorkspaceAdd(ts.Session, "", "WEB")
	usecasetest.WantErr(t, err, usecase.KindUsage, "workspace.bad_name")
	_, err = workspaces.WorkspaceRename(ts.Session, "me", "")
	usecasetest.WantErr(t, err, usecase.KindUsage, "workspace.bad_name")
	_, err = workspaces.WorkspacePrefix(ts.Session, "me", "x y")
	usecasetest.WantErr(t, err, usecase.KindUsage, "workspace.bad_prefix")
	if _, wss, _ := workspaces.WorkspaceList(ts.Session); len(wss) != 1 {
		t.Errorf("a refused call wrote: %v", wss)
	}
}

func TestWorkspaceNameAndPrefixTakenInTheStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := workspaces.WorkspaceAdd(ts.Session, "web", "WEB"); err != nil {
		t.Fatal(err)
	}
	_, err := workspaces.WorkspaceAdd(ts.Session, "web2", "WEB")
	usecasetest.WantErr(t, err, usecase.KindConflict, "")
	if c := usecase.Classify(err); c == nil || c.Code != "workspace.taken" {
		t.Errorf("classify(%v) = %+v; want workspace.taken", err, c)
	}
	if want := "the id prefix WEB is already used by workspace web"; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	_, err = workspaces.WorkspaceAdd(ts.Session, "web", "XX")
	usecasetest.WantErr(t, err, usecase.KindConflict, "")
	if want := "a workspace named web already exists"; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
}

func TestWorkspaceNotFoundAndAmbiguous(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	_, err := workspaces.WorkspaceRename(ts.Session, "nope", "x")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "workspace.not_found")
	if e := new(usecase.Error); !usecasetest.AsError(err, &e) || e.Args["input"] != "nope" {
		t.Errorf("args = %v; want the input", e)
	}
	_, err = workspaces.WorkspaceRemove(ts.Session, "nope")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "workspace.not_found")
	if _, err := workspaces.WorkspaceAdd(ts.Session, "api", "BE"); err != nil {
		t.Fatal(err)
	}
	be, err := workspaces.WorkspaceAdd(ts.Session, "be", "API")
	if err != nil {
		t.Fatal(err)
	}
	_, err = workspaces.WorkspacePrefix(ts.Session, "api", "ZZ")
	usecasetest.WantErr(t, err, usecase.KindAmbiguous, "workspace.ambiguous")
	if !strings.HasPrefix(err.Error(), `"api" matches 2 workspaces:`+"\n  api (BE, ") || !strings.Contains(err.Error(), "\n  be (API, "+be.Workspace.ULID.String()+")") {
		t.Errorf("message = %q", err)
	}
}

func TestWorkspacePrefixUnchangedWritesNothing(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	s, _, _ := ts.Open()
	before, _ := s.Repo().Head()
	r, err := workspaces.WorkspacePrefix(ts.Session, "me", "ME")
	if err != nil || r.Committed || r.Workspace.IDPrefix != "ME" || r.Renumbered != 0 {
		t.Fatalf("WorkspacePrefix = %+v, %v; want no change", r, err)
	}
	if after, _ := s.Repo().Head(); after != before {
		t.Error("the unchanged prefix was committed")
	}
}

func TestWorkspacePrefixRenumbersDeletedTasksToo(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	for _, title := range []string{"a", "b", "c"} {
		if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	ts.DeleteTask("ME-2")
	r, err := workspaces.WorkspacePrefix(ts.Session, "me", "NEW")
	if err != nil {
		t.Fatal(err)
	}
	want := []workspaces.IDChange{{"ME-1", "NEW-1"}, {"ME-2", "NEW-2"}, {"ME-3", "NEW-3"}}
	if r.Renumbered != 3 || !slices.Equal(r.IDs, want) {
		t.Errorf("IDs = %v (%d); want %v", r.IDs, r.Renumbered, want)
	}
}

func TestWorkspaceListCountsTasksNotDeleted(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddWorkspace("api", "API")
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a", Workspace: "me"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "b", Workspace: "me"}); err != nil {
		t.Fatal(err)
	}
	ts.DeleteTask("ME-2")
	ix, wss, err := workspaces.WorkspaceList(ts.Session)
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
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := stores.StoreInit(ts.Session, stores.StoreInitInput{Name: "team", Prefix: "TM"}); err != nil {
		t.Fatal(err)
	}
	err := ts.CheckPrefixElsewhere("TM", "me")
	usecasetest.WantErr(t, err, usecase.KindConflict, "workspace.prefix_taken")
	if e := new(usecase.Error); !usecasetest.AsError(err, &e) || e.Args["prefix"] != "TM" {
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
	me := &usecase.Session{Getenv: ts.Getenv, Now: ts.Now, Warn: ts.Warn, StoreName: "me"}
	_, err = workspaces.WorkspaceAdd(me, "other", "TM")
	usecasetest.WantErr(t, err, usecase.KindConflict, "workspace.prefix_taken")
	_, err = workspaces.WorkspacePrefix(me, "me", "TM")
	usecasetest.WantErr(t, err, usecase.KindConflict, "workspace.prefix_taken")
}

func TestCheckPrefixElsewhereSkipsABrokenStoreWithAWarning(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	res, err := stores.StoreInit(ts.Session, stores.StoreInitInput{Name: "other", Prefix: "OT"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(res.Path, model.StoreMetaPath), []byte("not: [yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ts.Warns = nil
	if err := ts.CheckPrefixElsewhere("WEB", "me"); err != nil {
		t.Fatal(err)
	}
	if len(ts.Warns) != 1 || !strings.HasPrefix(ts.Warns[0], "store other is not checked for id prefixes: ") {
		t.Errorf("warns = %q", ts.Warns)
	}
}

func TestWorkspaceOnANamedStoreDoesNotTouchTheSession(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := stores.StoreInit(ts.Session, stores.StoreInitInput{Name: "team", Prefix: "TM"}); err != nil {
		t.Fatal(err)
	}
	sub := &usecase.Session{Getenv: ts.Getenv, Now: ts.Now, Warn: ts.Warn, StoreName: "team"}
	if _, err := workspaces.WorkspaceAdd(sub, "api", "API"); err != nil {
		t.Fatal(err)
	}
	if ts.StoreName != "" {
		t.Errorf("StoreName = %q; want it untouched", ts.StoreName)
	}
	me := &usecase.Session{Getenv: ts.Getenv, Now: ts.Now, Warn: ts.Warn, StoreName: "me"}
	if _, wss, _ := workspaces.WorkspaceList(me); len(wss) != 1 {
		t.Errorf("the store me has %d workspaces; want 1", len(wss))
	}
	if _, wss, _ := workspaces.WorkspaceList(sub); len(wss) != 2 {
		t.Errorf("the store team has %d workspaces; want 2", len(wss))
	}
}

func TestWorkspaceRemoveRefusalSaysThemForSeveralTasks(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddWorkspace("api", "API")
	for _, title := range []string{"a", "b"} {
		if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: title, Workspace: "api"}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := workspaces.WorkspaceRemove(ts.Session, "api")
	usecasetest.WantErr(t, err, usecase.KindConflict, "workspace.not_empty")
	if want := "workspace api still has 2 tasks; move them with hedgehug mv --workspace, or delete them with hedgehug rm and remove them with hedgehug purge --older-than 0d --store <store>"; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
}
