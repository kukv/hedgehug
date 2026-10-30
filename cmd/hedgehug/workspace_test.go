package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

func TestWorkspaceCommands(t *testing.T) {
	te := newStoreEnv(t) // me, prefix ME
	te.ok("workspace", "add", "api", "--prefix", "API")
	if code, _, errs := te.run("add", "x"); code == 0 || !strings.Contains(errs, "--workspace") {
		t.Errorf("add with two workspaces: %d %s", code, errs)
	}
	a := te.addJSON("a", "--workspace", "api")
	if a.Key != "API-1" {
		t.Errorf("key = %s", a.Key)
	}
	child := te.addJSON("c", "--parent", a.Key)
	if child.Key != "API-2" {
		t.Errorf("child key = %s", child.Key)
	}
	if code, _, _ := te.run("add", "y", "--parent", a.Key, "--workspace", "me"); code == 0 {
		t.Error("a parent in another workspace is accepted")
	}
	out := te.ok("workspace", "prefix", "api", "BE")
	if !strings.Contains(out, "API-1 → BE-1") || !strings.Contains(out, "API-2 → BE-2") {
		t.Errorf("prefix =\n%s", out)
	}
	if got := te.task(a.ID); got.Key != "BE-1" || !slices.Equal(got.FormerKeys, []string{"API-1"}) {
		t.Errorf("after prefix: %s %v", got.Key, got.FormerKeys)
	}
	te.ok("show", "API-1") // a former key still resolves
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
	te := newStoreEnv(t)
	te.ok("workspace", "rm", "me")
	if code, _, errs := te.run("add", "x"); code == 0 || !strings.Contains(errs, "the store has no workspace; add one with hedgehug workspace add") {
		t.Errorf("add with no workspace: %d %s", code, errs)
	}
}

func TestWorkspacePrefixSame(t *testing.T) {
	te := newStoreEnv(t)
	te.addJSON("a")
	before := len(te.log("me"))
	if out := te.ok("workspace", "prefix", "me", "ME"); !strings.HasPrefix(out, "No changes") || strings.Contains(out, "→") {
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
// into v.
func (te *testEnv) wsJSON(v any, args ...string) {
	te.t.Helper()
	out := te.ok(append(append([]string{"workspace"}, args...), "--json")...)
	if err := json.Unmarshal([]byte(out), v); err != nil {
		te.t.Fatalf("%v: %s", err, out)
	}
}

func TestWorkspaceCommitSubjects(t *testing.T) {
	te := newStoreEnv(t)
	te.addJSON("a")
	te.ok("workspace", "add", "web", "--prefix", "WEB")
	var ls []map[string]any
	te.wsJSON(&ls, "ls")
	ids := map[string]string{}
	for _, w := range ls {
		ids[w["name"].(string)] = w["id"].(string)
	}
	for _, c := range []struct {
		args []string
		want string
		ws   string // the name of the workspace changed
	}{
		{nil, "hedgehug: workspace add web (WEB)", "web"},
		{[]string{"rename", "web", "front"}, "hedgehug: workspace rename web → front (WEB)", "web"},
		{[]string{"prefix", "me", "BE"}, "hedgehug: workspace prefix me ME → BE", "me"},
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
		if !strings.Contains(msg, "Hedgehug-Workspace: "+ids[c.ws]) {
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
	te := newStoreEnv(t)
	te.addJSON("a")
	want := []string{"id", "key_prefix", "name", "prefix_set", "tasks"}
	var added map[string]any
	te.wsJSON(&added, "add", "web", "--prefix", "WEB")
	if !slices.Equal(fieldNames(added), want) || added["name"] != "web" || added["key_prefix"] != "WEB" || added["tasks"] != 0.0 {
		t.Errorf("add --json = %v", added)
	}
	var ls []map[string]any
	te.wsJSON(&ls, "ls")
	if len(ls) != 2 || !slices.Equal(fieldNames(ls[0]), want) || ls[0]["name"] != "me" || ls[0]["tasks"] != 1.0 || ls[1]["name"] != "web" {
		t.Errorf("ls --json = %v", ls)
	}
	var renamed map[string]any
	te.wsJSON(&renamed, "rename", "web", "front")
	if !slices.Equal(fieldNames(renamed), want) || renamed["name"] != "front" || renamed["id"] != added["id"] {
		t.Errorf("rename --json = %v", renamed)
	}
	var prefixed map[string]any
	te.wsJSON(&prefixed, "prefix", "me", "BE")
	if !slices.Equal(fieldNames(prefixed), want) || prefixed["key_prefix"] != "BE" || prefixed["name"] != "me" {
		t.Errorf("prefix --json = %v", prefixed)
	}
	var removed map[string]any
	te.wsJSON(&removed, "rm", "front")
	if !slices.Equal(fieldNames(removed), want) || removed["name"] != "front" || removed["key_prefix"] != "WEB" {
		t.Errorf("rm --json = %v", removed)
	}
}

func TestStoreInitJSONHasTheWorkspace(t *testing.T) {
	te := newTestEnv(t)
	var got struct {
		Workspace map[string]any `json:"workspace"`
	}
	if err := json.Unmarshal([]byte(te.ok("store", "init", "me", "--prefix", "ME", "--workspace", "main", "--json")), &got); err != nil {
		t.Fatal(err)
	}
	w := got.Workspace
	if !slices.Equal(fieldNames(w), []string{"id", "key_prefix", "name"}) || w["name"] != "main" || w["key_prefix"] != "ME" {
		t.Fatalf("workspace = %v", w)
	}
	if _, err := model.ParseID(w["id"].(string)); err != nil {
		t.Errorf("id: %v", err)
	}
}

func TestAmbiguousWorkspace(t *testing.T) {
	te := newStoreEnv(t)
	var api, be map[string]any
	te.wsJSON(&api, "add", "api", "--prefix", "BE")
	te.wsJSON(&be, "add", "be", "--prefix", "API")
	code, _, errs := te.run("workspace", "rename", "api", "x")
	if code != 1 {
		t.Errorf("exit %d", code)
	}
	for _, want := range []string{
		`"api" matches 2 workspaces:`,
		"\n  api (BE, " + api["id"].(string) + ")",
		"\n  be (API, " + be["id"].(string) + ")",
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("stderr lacks %q:\n%s", want, errs)
		}
	}
}

func TestCheckPrefixElsewhereSkipsABrokenStore(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("store", "init", "other", "--prefix", "OT")
	if err := os.WriteFile(filepath.Join(te.storeDir("other"), model.StoreMetaPath), []byte("not: [yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errs := te.run("--store", "me", "workspace", "add", "web", "--prefix", "WEB")
	if code != 0 || !strings.Contains(errs, "warning: store other is not checked for key prefixes") {
		t.Errorf("exit %d, stderr %q", code, errs)
	}
	if out := te.ok("--store", "me", "workspace", "ls"); !strings.Contains(out, "web") {
		t.Errorf("the workspace was not added:\n%s", out)
	}
}

func TestAddUnderAParentInAMissingWorkspace(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("p")
	gone := model.NewID().String()
	te.commitFile(p.ID, setLine(te.raw(p.ID), "workspace", gone))
	if code, _, errs := te.run("add", "c", "--parent", p.Key, "--workspace", "me"); code != 1 || !strings.Contains(errs, "the parent's workspace "+gone+" is not in this store") {
		t.Errorf("with --workspace: exit %d, stderr %q", code, errs)
	}
	if code, _, errs := te.run("add", "c", "--parent", p.Key); code != 1 || !strings.Contains(errs, "workspace "+gone+" is not in this store") {
		t.Errorf("without --workspace: exit %d, stderr %q", code, errs)
	}
}

func TestTaskInAMissingWorkspace(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("x")
	gone := model.NewID().String()
	te.commitFile(x.ID, setLine(te.raw(x.ID), "workspace", gone))
	var got taskJSON
	if err := json.Unmarshal([]byte(te.ok("show", x.Key, "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Workspace == nil || *got.Workspace != (workspaceRefJSON{ID: gone}) {
		t.Errorf("workspace = %+v", got.Workspace)
	}
	if out := te.ok("show", x.Key); !strings.Contains(out, "  workspace   "+gone+"\n") {
		t.Errorf("show:\n%s", out)
	}
}

func TestAmbiguousTask(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("first")
	b := te.addJSON("second")
	te.commitFile(b.ID, setLine(te.raw(b.ID), "key", a.Key))
	code, _, errs := te.run("show", a.Key)
	if code != 1 {
		t.Errorf("exit %d", code)
	}
	for _, want := range []string{
		`"` + a.Key + `" matches 2 tasks:`,
		"\n  " + a.Key + " (" + a.ID + ") first",
		"\n  " + a.Key + " (" + b.ID + ") second",
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("stderr lacks %q:\n%s", want, errs)
		}
	}
}
