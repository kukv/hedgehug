package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

// newTwoStoreEnv is an environment with two stores, alpha (AA) and
// bravo-long (BB), each with tasks.
func newTwoStoreEnv(t *testing.T) *testEnv {
	t.Helper()
	te := newTestEnv(t)
	te.ok("store", "init", "alpha", "--prefix", "AA")
	te.ok("store", "init", "bravo-long", "--prefix", "BB")
	p := te.addJSON("--store", "alpha", "Alpha project", "--kind", "project")
	te.addJSON("--store", "alpha", "Alpha child", "--parent", p.ID)
	te.addJSON("--store", "bravo-long", "Bravo task", "--status", "done")
	te.addJSON("--store", "bravo-long", "Bravo other")
	return te
}

func TestLsAcrossStores(t *testing.T) {
	t.Parallel()
	te := newTwoStoreEnv(t)
	te.goldenOK("ls", "ls")
	te.goldenOK("group", "ls", "group:status")
	te.goldenOK("sort", "ls", "sort:-title")
	te.goldenOK("one-by-flag", "ls", "--store", "alpha")
	te.goldenOK("one-by-term", "ls", "store:bravo-long")
	te.goldenOK("not-one", "ls", "--", "-store:bravo-long")
	te.goldenOK("tree", "ls", "--tree")
	te.goldenOK("tree-found", "ls", "--tree", "child")
	te.goldenOK("tree-one", "ls", "--tree", "store:alpha")
	te.goldenOK("json", "ls", "--json")
	te.goldenOK("format", "ls", "--format", "{{.store}} {{.id}} {{.title}}")
	te.goldenOK("tree-format", "ls", "--tree", "--format", "{{.store}} {{.id}} {{.context}}")
	te.goldenRun("unknown-store", 2, "ls", "store:nope")

	var all []ops.TaskView
	if err := json.Unmarshal([]byte(te.ok("ls", "--json")), &all); err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 || all[0].Store != "alpha" || all[3].Store != "bravo-long" {
		t.Errorf("ls --json = %+v", all)
	}
	var tree []ops.TaskView
	if err := json.Unmarshal([]byte(te.ok("ls", "--tree", "--json")), &tree); err != nil {
		t.Fatal(err)
	}
	if len(tree) != 3 || tree[0].Store != "alpha" || tree[1].Store != "bravo-long" || len(tree[0].Children) != 1 || tree[0].Children[0].Store != "alpha" {
		t.Errorf("ls --tree --json = %+v", tree)
	}
	if n := len(strings.Fields(te.ok("ls", "--ulids-only"))); n != 4 {
		t.Errorf("--ulids-only has %d ULIDs; want 4, one per task", n)
	}
}

func TestLsOneStoreHasNoStoreColumn(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.addJSON("Only")
	te.goldenOK("ls", "ls")
	te.goldenOK("tree", "ls", "--tree")
	te.goldenOK("show", "show", "ME-1")
}

func TestShowTellsItsStore(t *testing.T) {
	t.Parallel()
	te := newTwoStoreEnv(t)
	te.goldenOK("show", "show", "BB-1")
	var got ops.ShowView
	if err := json.Unmarshal([]byte(te.ok("show", "--json", "BB-2")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Store != "bravo-long" || len(got.History) == 0 || got.History[0].Store != "bravo-long" {
		t.Errorf("show --json: store %q, history %+v", got.Store, got.History)
	}
}

func TestLogAcrossStores(t *testing.T) {
	t.Parallel()
	te := newTwoStoreEnv(t)
	// three commits in each store: its init and two adds
	out := te.ok("log")
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 6 {
		t.Fatalf("log has %d lines; want 6:\n%s", len(lines), out)
	}
	// the name of the store leads each line, padded to the longest name
	re := regexp.MustCompile(`^(alpha {5}|bravo-long)  [0-9a-f]{7}  \d{4}-\d{2}-\d{2} \d{2}:\d{2}  `)
	counts := map[string]int{}
	for _, l := range lines {
		m := re.FindStringSubmatch(l)
		if m == nil {
			t.Errorf("line %q does not start with a padded store name", l)
			continue
		}
		counts[strings.TrimSpace(m[1])]++
	}
	if counts["alpha"] != 3 || counts["bravo-long"] != 3 {
		t.Errorf("commits per store = %v; want 3 and 3", counts)
	}
	// the limit is for all the stores
	if got := strings.Split(strings.TrimRight(te.ok("log", "-n", "3"), "\n"), "\n"); len(got) != 3 {
		t.Errorf("log -n 3 has %d lines", len(got))
	}
	// --store leaves one store of two mounted, which is still named; one task is not
	if out := te.ok("log", "--store", "alpha"); strings.Count(out, "alpha  ") != 3 || strings.Contains(out, "bravo") {
		t.Errorf("log --store alpha =\n%s", out)
	}
	if out := te.ok("log", "BB-1"); strings.Contains(out, "bravo-long") || strings.Count(out, "\n") != 1 {
		t.Errorf("log BB-1 =\n%s", out)
	}
	var views []ops.LogView
	if err := json.Unmarshal([]byte(te.ok("log", "--json")), &views); err != nil {
		t.Fatal(err)
	}
	stores := map[string]int{}
	for _, v := range views {
		stores[v.Store]++
	}
	if len(views) != 6 || stores["alpha"] != 3 || stores["bravo-long"] != 3 {
		t.Errorf("log --json stores = %v of %d", stores, len(views))
	}
}

// With several stores the parent-cycle section is told from the heading of a
// store by naming its store.
func TestLsTreeCycleAcrossStores(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("store", "init", "bravo", "--prefix", "BB")
	x := te.addJSON("--store", "me", "X")
	y := te.addJSON("--store", "me", "Y", "--parent", x.ID)
	te.addJSON("--store", "bravo", "Plain")
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	xULID, _ := model.ParseULID(x.ULID)
	yULID, _ := model.ParseULID(y.ULID)
	writeCycle(t, te, s, xULID, yULID)
	te.goldenOK("tree", "ls", "--tree")
	te.goldenOK("tree-one", "ls", "--tree", "store:me")
	out := te.ok("ls", "--tree")
	if !strings.Contains(out, "== me (in a parent cycle)\n") || strings.Contains(out, "\n== in a parent cycle") || !strings.Contains(out, "== bravo\n") {
		t.Errorf("tree =\n%s", out)
	}
}

func TestLsIDInAnotherStore(t *testing.T) {
	t.Parallel()
	te := newTwoStoreEnv(t)
	// AA-1 is only in alpha: bravo-long finds nothing, which is no error
	out := te.ok("ls", "project:AA-1")
	if !strings.Contains(out, "Alpha child") || strings.Contains(out, "Bravo") {
		t.Errorf("project:AA-1 =\n%s", out)
	}
	// the negation matches everything in the store without it
	if out := te.ok("ls", "--", "-project:AA-1"); !strings.Contains(out, "Bravo task") || !strings.Contains(out, "Bravo other") || strings.Contains(out, "Alpha child") {
		t.Errorf("-project:AA-1 =\n%s", out)
	}
	if out := te.ok("ls", "project:alpha:AA-1"); !strings.Contains(out, "Alpha child") {
		t.Errorf("project:alpha:AA-1 =\n%s", out)
	}
	// written for the other store, it finds nothing there, and nowhere
	code, _, errs := te.run("ls", "project:bravo-long:AA-1")
	if code != 2 || !strings.Contains(errs, "not found") {
		t.Errorf("project:bravo-long:AA-1: exit %d, stderr %q", code, errs)
	}
	code, _, errs = te.run("ls", "project:ZZ-9")
	if code != 2 || !strings.Contains(errs, "not found") {
		t.Errorf("project:ZZ-9: exit %d, stderr %q", code, errs)
	}
}
