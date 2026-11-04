package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

func TestWorkspaceCommands(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t) // me, prefix ME
	te.ok("workspace", "add", "api", "--prefix", "API")
	if code, _, errs := te.run("add", "x"); code == 0 || !strings.Contains(errs, "--workspace") {
		t.Errorf("add with two workspaces: %d %s", code, errs)
	}
	a := te.addJSON("a", "--workspace", "api")
	if a.ID != "API-1" {
		t.Errorf("id = %s", a.ID)
	}
	child := te.addJSON("c", "--parent", a.ID)
	if child.ID != "API-2" {
		t.Errorf("child id = %s", child.ID)
	}
	if code, _, _ := te.run("add", "y", "--parent", a.ID, "--workspace", "me"); code == 0 {
		t.Error("a parent in another workspace is accepted")
	}
	out := te.ok("workspace", "set-prefix", "api", "BE")
	if !strings.Contains(out, "API-1 → BE-1") || !strings.Contains(out, "API-2 → BE-2") {
		t.Errorf("prefix =\n%s", out)
	}
	if got := te.task(a.ULID); got.ID != "BE-1" || !slices.Equal(got.FormerIDs, []string{"API-1"}) {
		t.Errorf("after prefix: %s %v", got.ID, got.FormerIDs)
	}
	te.ok("show", "API-1") // a former id still resolves
	te.ok("workspace", "rename", "be", "backend")
	if code, _, errs := te.run("workspace", "rm", "backend"); code == 0 || !strings.Contains(errs, "2 tasks") {
		t.Errorf("rm with tasks: %d %s", code, errs)
	}
	te.ok("workspace", "add", "empty", "--prefix", "EM")
	te.ok("workspace", "rm", "EM")
	if code, _, _ := te.run("workspace", "add", "dup", "--prefix", "BE"); code == 0 {
		t.Error("a duplicate prefix is accepted")
	}
}

func TestAddWithoutWorkspaces(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("workspace", "rm", "me")
	if code, _, errs := te.run("add", "x"); code == 0 || !strings.Contains(errs, "the store has no workspace; add one with hedgehug workspace add") {
		t.Errorf("add with no workspace: %d %s", code, errs)
	}
}

func TestWorkspaceSetPrefixAndTheOldNameIsAUsageError(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.addJSON("a")
	before := len(te.log("me"))

	code, _, errs := te.run("workspace", "prefix", "me", "BE")
	if code != 2 || !strings.Contains(errs, `unknown command "prefix"`) {
		t.Errorf("workspace prefix: exit %d, stderr %q; want 2 and an unknown command", code, errs)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d: the old name wrote", n, before)
	}

	out := te.ok("workspace", "set-prefix", "me", "BE")
	if !strings.Contains(out, "Changed the id prefix of me to BE") || !strings.Contains(out, "ME-1 → BE-1") {
		t.Errorf("set-prefix =\n%s", out)
	}
	if n := len(te.log("me")); n != before+1 {
		t.Errorf("commits = %d, want %d", n, before+1)
	}
}

func TestWorkspacePrefixSame(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.addJSON("a")
	before := len(te.log("me"))
	if out := te.ok("workspace", "set-prefix", "me", "ME"); !strings.HasPrefix(out, "No changes") || strings.Contains(out, "→") {
		t.Errorf("prefix to the same =\n%s", out)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

// lastCommit returns the subject of the last commit of the store me and its
// whole message.
func (te *testEnv) lastCommit() (subject, msg string) {
	te.t.Helper()
	msg = te.log("me")[0].Message
	return strings.SplitN(msg, "\n", 2)[0], msg
}

// wsJSON runs a workspace command with --json and decodes what it prints
// into v. A write prints {workspace, commit}; v gets the workspace.
func (te *testEnv) wsJSON(v any, args ...string) {
	te.t.Helper()
	out := te.ok(append(append([]string{"workspace"}, args...), "--json")...)
	if args[0] == "ls" {
		if err := json.Unmarshal([]byte(out), v); err != nil {
			te.t.Fatalf("%v: %s", err, out)
		}
		return
	}
	var w struct {
		Workspace json.RawMessage `json:"workspace"`
	}
	if err := json.Unmarshal([]byte(out), &w); err != nil || len(w.Workspace) == 0 {
		te.t.Fatalf("%v: %s", err, out)
	}
	if err := json.Unmarshal(w.Workspace, v); err != nil {
		te.t.Fatalf("%v: %s", err, out)
	}
}

func TestWorkspaceCommitSubjects(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.addJSON("a")
	te.ok("workspace", "add", "web", "--prefix", "WEB")
	var ls []map[string]any
	te.wsJSON(&ls, "ls")
	ulids := map[string]string{}
	for _, w := range ls {
		ulids[w["name"].(string)] = w["ulid"].(string)
	}
	for _, c := range []struct {
		args []string
		want string
		ws   string // the name of the workspace changed
	}{
		{nil, "hedgehug: workspace add web (WEB)", "web"},
		{[]string{"rename", "web", "front"}, "hedgehug: workspace rename web → front (WEB)", "web"},
		{[]string{"set-prefix", "me", "BE"}, "hedgehug: workspace prefix me ME → BE", "me"},
		{[]string{"rm", "front"}, "hedgehug: workspace rm front (WEB)", "web"},
	} {
		if c.args != nil {
			te.ok(append([]string{"workspace"}, c.args...)...)
		}
		subj, msg := te.lastCommit()
		if subj != c.want {
			t.Errorf("subject = %q, want %q", subj, c.want)
		}
		if !strings.Contains(msg, "Hedgehug-Op: workspace") {
			t.Errorf("%q: no Hedgehug-Op trailer:\n%s", c.want, msg)
		}
		if !strings.Contains(msg, "Hedgehug-Workspace: "+ulids[c.ws]) {
			t.Errorf("%q: no Hedgehug-Workspace trailer:\n%s", c.want, msg)
		}
	}
}

// fieldNames returns the keys of m in order.
func fieldNames(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func TestWorkspaceJSON(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.addJSON("a")
	want := []string{"deleted", "id_prefix", "name", "prefix_set", "tasks", "ulid"}
	// A write prints {workspace, commit, ids?}.
	write := func(args ...string) (ws map[string]any, w map[string]any) {
		te.t.Helper()
		out := te.ok(append(append([]string{"workspace"}, args...), "--json")...)
		if err := json.Unmarshal([]byte(out), &w); err != nil {
			te.t.Fatalf("%v: %s", err, out)
		}
		ws, _ = w["workspace"].(map[string]any)
		if _, ok := w["commit"].(string); !ok {
			te.t.Errorf("%v: commit = %v, want the commit made", args, w["commit"])
		}
		return ws, w
	}
	added, _ := write("add", "web", "--prefix", "WEB")
	if !slices.Equal(fieldNames(added), want) || added["name"] != "web" || added["id_prefix"] != "WEB" || added["tasks"] != 0.0 {
		t.Errorf("add --json = %v", added)
	}
	var ls []map[string]any
	te.wsJSON(&ls, "ls")
	if len(ls) != 2 || !slices.Equal(fieldNames(ls[0]), want) || ls[0]["name"] != "me" || ls[0]["tasks"] != 1.0 || ls[1]["name"] != "web" {
		t.Errorf("ls --json = %v", ls)
	}
	renamed, _ := write("rename", "web", "front")
	if !slices.Equal(fieldNames(renamed), want) || renamed["name"] != "front" || renamed["ulid"] != added["ulid"] {
		t.Errorf("rename --json = %v", renamed)
	}
	prefixed, whole := write("set-prefix", "me", "BE")
	if !slices.Equal(fieldNames(prefixed), want) || prefixed["id_prefix"] != "BE" || prefixed["name"] != "me" {
		t.Errorf("prefix --json = %v", prefixed)
	}
	if ids, _ := whole["ids"].([]any); len(ids) != 1 || !slices.Equal(fieldNames(ids[0].(map[string]any)), []string{"new", "old"}) { //nolint:forcetypeassert // --json prints an object
		t.Errorf("prefix --json ids = %v, want the one task renumbered", whole["ids"])
	}
	removed, _ := write("rm", "front")
	if !slices.Equal(fieldNames(removed), want) || removed["name"] != "front" || removed["id_prefix"] != "WEB" {
		t.Errorf("rm --json = %v", removed)
	}
}

func TestStoreInitJSONHasTheWorkspace(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	var got struct {
		Workspace map[string]any `json:"workspace"`
	}
	if err := json.Unmarshal([]byte(te.ok("store", "init", "me", "--prefix", "ME", "--workspace", "main", "--json")), &got); err != nil {
		t.Fatal(err)
	}
	w := got.Workspace
	if !slices.Equal(fieldNames(w), []string{"id_prefix", "name", "ulid"}) || w["name"] != "main" || w["id_prefix"] != "ME" {
		t.Fatalf("workspace = %v", w)
	}
	if _, err := model.ParseULID(w["ulid"].(string)); err != nil {
		t.Errorf("ulid: %v", err)
	}
}

func TestAmbiguousWorkspace(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	var api, be map[string]any
	te.wsJSON(&api, "add", "api", "--prefix", "BE")
	te.wsJSON(&be, "add", "be", "--prefix", "API")
	code, _, errs := te.run("workspace", "rename", "api", "x")
	if code != 2 {
		t.Errorf("exit %d", code)
	}
	for _, want := range []string{
		`"api" matches 2 workspaces:`,
		"\n  api (BE, " + api["ulid"].(string) + ")",
		"\n  be (API, " + be["ulid"].(string) + ")",
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("stderr lacks %q:\n%s", want, errs)
		}
	}
}

func TestCheckPrefixElsewhereSkipsABrokenStore(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("store", "init", "other", "--prefix", "OT")
	if err := os.WriteFile(filepath.Join(te.storeDir("other"), model.StoreMetaPath), []byte("not: [yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errs := te.run("--store", "me", "workspace", "add", "web", "--prefix", "WEB")
	if code != 0 || !strings.Contains(errs, "warning: store other is not checked for id prefixes") {
		t.Errorf("exit %d, stderr %q", code, errs)
	}
	if out := te.ok("--store", "me", "workspace", "ls"); !strings.Contains(out, "web") {
		t.Errorf("the workspace was not added:\n%s", out)
	}
}

func TestAddUnderAParentInAMissingWorkspace(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("p")
	gone := model.NewULID().String()
	te.commitFile(p.ULID, setLine(te.raw(p.ULID), "workspace", gone))
	if code, _, errs := te.run("add", "c", "--parent", p.ID, "--workspace", "me"); code != 1 || !strings.Contains(errs, "the parent's workspace "+gone+" is not in this store") {
		t.Errorf("with --workspace: exit %d, stderr %q", code, errs)
	}
	if code, _, errs := te.run("add", "c", "--parent", p.ID); code != 1 || !strings.Contains(errs, "workspace "+gone+" is not in this store") {
		t.Errorf("without --workspace: exit %d, stderr %q", code, errs)
	}
}

func TestTaskInAMissingWorkspace(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	x := te.addJSON("x")
	gone := model.NewULID().String()
	te.commitFile(x.ULID, setLine(te.raw(x.ULID), "workspace", gone))
	var got usecase.TaskView
	if err := json.Unmarshal([]byte(te.ok("show", x.ID, "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Workspace == nil || *got.Workspace != (usecase.WorkspaceRef{ULID: gone}) {
		t.Errorf("workspace = %+v", got.Workspace)
	}
	if out := te.ok("show", x.ID); !strings.Contains(out, "  workspace   "+gone+"\n") {
		t.Errorf("show:\n%s", out)
	}
}

func TestAmbiguousTask(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("first")
	b := te.addJSON("second")
	te.commitFile(b.ULID, setLine(te.raw(b.ULID), "id", a.ID))
	code, _, errs := te.run("show", a.ID)
	if code != 2 {
		t.Errorf("exit %d", code)
	}
	for _, want := range []string{
		`"` + a.ID + `" matches 2 tasks:`,
		"\n  " + a.ID + " (" + a.ULID + ") first",
		"\n  " + a.ID + " (" + b.ULID + ") second",
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("stderr lacks %q:\n%s", want, errs)
		}
	}
}
