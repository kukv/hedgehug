package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

func TestDoctorNoProblems(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("add", "Alpha")
	code, out, stderr := te.run("doctor")
	if code != 0 || out != "No problems\n" || stderr != "" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	code, out, _ = te.run("doctor", "--json")
	var rs []struct {
		Store    string
		Repaired []json.RawMessage
		Findings []json.RawMessage
	}
	if code != 0 || json.Unmarshal([]byte(out), &rs) != nil || len(rs) != 1 || rs[0].Store != "me" || rs[0].Repaired == nil || rs[0].Findings == nil || len(rs[0].Findings) != 0 {
		t.Errorf("--json: exit %d, stdout %q; want one report of me with empty lists", code, out)
	}
}

func TestDoctorBrokenSchemaFile(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("add", "Alpha")
	if err := os.WriteFile(filepath.Join(te.storeDir("me"), "schema.yaml"), []byte("statuses: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := te.run("doctor")
	if code != 1 || stderr != "" {
		t.Fatalf("exit %d, stderr %q\n%s", code, stderr, out)
	}
	if !strings.Contains(out, "schema.yaml: ") || !strings.Contains(out, "  hint: restore schema.yaml") || !strings.HasSuffix(out, " problems\n") {
		t.Errorf("output:\n%s", out)
	}
	code, out, _ = te.run("doctor", "--json")
	var rs []struct {
		Store    string
		Findings []map[string]string
	}
	if code != 1 || json.Unmarshal([]byte(out), &rs) != nil || len(rs) != 1 || len(rs[0].Findings) == 0 ||
		rs[0].Findings[0]["check"] != "schema" || rs[0].Findings[0]["severity"] != "error" {
		t.Errorf("--json: exit %d\n%s", code, out)
	}
}

func TestDoctorTakesNoArguments(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	if code, _, _ := te.run("doctor", "x"); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}

func TestDoctorFixWithoutRemote(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "a")
	te.ok("add", "b")
	te.handEdit("me", "ME-2", "id: ME-2", "id: ME-1") // writes the file and commits it as a hand edit would
	code, out, _ := te.run("doctor")
	if code != 1 || !strings.Contains(out, "hint: hedgehug doctor --fix fixes it") {
		t.Errorf("doctor found nothing: %d %s", code, out)
	}
	before := len(te.log("me"))
	out = te.ok("doctor", "--fix")
	if !regexp.MustCompile(`repaired: ME-2 \([0-9A-Z]{26}\) b: id_collision ME-1 → ME-2`).MatchString(out) {
		t.Errorf("fix = %s", out)
	}
	if n := len(te.log("me")); n != before+1 || !strings.Contains(te.headCommit("me").Message, "Hedgehug-Op: repair") {
		t.Errorf("commits %d -> %d, head %q", before, n, te.headCommit("me").Message)
	}
	if code, out, _ := te.run("doctor"); code != 0 {
		t.Errorf("after fix: %d %s", code, out)
	}
	if out := te.ok("doctor", "--fix"); !strings.Contains(out, "Nothing to repair") || !strings.Contains(out, "No problems") {
		t.Errorf("second fix = %s", out)
	}
	if n := len(te.log("me")); n != before+1 {
		t.Errorf("the second fix committed: %d", n)
	}
}

func TestDoctorFixJSON(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "a")
	te.ok("add", "b")
	te.handEdit("me", "ME-2", "id: ME-2", "id: ME-1")
	code, out, _ := te.run("doctor", "--fix", "--json")
	var reports []struct {
		Store    string
		Repaired []struct {
			ULID, ID, Title, Kind string
			Value                 json.RawMessage
		}
		Findings []map[string]string
	}
	if code != 0 || json.Unmarshal([]byte(out), &reports) != nil || len(reports) != 1 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	got := reports[0]
	if got.Store != "me" || len(got.Repaired) != 1 || got.Findings == nil || len(got.Findings) != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if r := got.Repaired[0]; r.Kind != "id_collision" || r.ID != "ME-2" || r.Title != "b" || len(r.ULID) != 26 || !strings.Contains(string(r.Value), `"old": "ME-1"`) {
		t.Errorf("repaired = %+v", r)
	}
}

func TestDoctorFixLeavesBrokenFiles(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "a")
	te.handEdit("me", "ME-1", "title: a", "title: [")
	code, out, _ := te.run("doctor", "--fix")
	if code != 1 || !strings.Contains(out, "does not decode") || !strings.Contains(out, "Nothing to repair") {
		t.Errorf("code %d out %s", code, out)
	}
}

func TestDoctorFixTakesNoArguments(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	if code, _, _ := te.run("doctor", "--fix", "x"); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}

func TestDoctorMergeDriverAndInbox(t *testing.T) {
	te := newStoreEnv(t)
	// Not parallel: PATH is the process's. TestMain put hedgehug on it.
	path := os.Getenv("PATH")
	t.Setenv("PATH", "")
	code, out, _ := te.run("doctor")
	if code != 1 || !strings.Contains(out, "hedgehug is not on PATH") {
		t.Errorf("exit %d\n%s", code, out)
	}
	t.Setenv("PATH", path)
	rec := filepath.Join(te.storeDir("me"), ".hedgehug", "local", "overwritten", "01JC1111111111111111111111.json")
	if err := os.MkdirAll(filepath.Dir(rec), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rec, []byte(`{"path":"tasks/x.md","field":"title","value":"\"a\"","author":"a","commit":"","time":"2026-10-31T00:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, _ = te.run("doctor")
	if code != 1 || !strings.Contains(out, "1 merge driver records are not in the inbox yet") || !strings.Contains(out, "hint: run hedgehug inbox") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

// insertField adds a line of front matter to the task id, as a hand edit
// would, before its kind.
func (te *testEnv) insertField(id, line string) {
	te.t.Helper()
	te.handEdit("me", id, "\nkind: ", "\n"+line+"\nkind: ")
}

func TestDoctorFixKinds(t *testing.T) {
	t.Parallel()
	const hint = "hint: hedgehug doctor --fix fixes it"
	tests := []struct {
		name  string
		setup func(te *testEnv)
		line  string // of the repair
	}{
		{"parent_cycle", func(te *testEnv) {
			te.ok("add", "a")
			b := te.addJSON("b", "--parent", "ME-1")
			te.insertField("ME-1", "parent: "+b.ULID)
		}, "parent_cycle: moved back to"},
		{"parent_deleted", func(te *testEnv) {
			te.ok("add", "a")
			te.ok("add", "b", "--parent", "ME-1")
			te.insertField("ME-1", "deleted_at: 2026-10-30T12:00:00Z")
		}, "parent_deleted: moved to the root"},
		{"workspace_mismatch", func(te *testEnv) {
			te.ok("workspace", "add", "other", "--prefix", "OT")
			c := te.addJSON("c", "--workspace", "other")
			te.ok("add", "d", "--workspace", "me")
			te.insertField("ME-1", "parent: "+c.ULID)
		}, "workspace_mismatch"},
		{"id_prefix", func(te *testEnv) {
			te.ok("add", "a")
			te.handEdit("me", "ME-1", "id: ME-1", "id: XX-1")
		}, "id_renumbered"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := newTestEnv(t)
			te.ok("store", "init", "me", "--prefix", "ME")
			tt.setup(te)
			code, out, _ := te.run("doctor")
			if code != 1 || !strings.Contains(out, hint) {
				t.Fatalf("doctor: %d\n%s", code, out)
			}
			out = te.ok("doctor", "--fix")
			if !strings.Contains(out, "repaired: ") || !strings.Contains(out, tt.line) {
				t.Errorf("fix:\n%s", out)
			}
			if code, out, _ := te.run("doctor"); code != 0 {
				t.Errorf("after fix: %d\n%s", code, out)
			}
		})
	}
}

func TestDoctorFixError(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "a")
	b := te.addJSON("b")
	te.handEdit("me", "ME-2", "id: ME-2", "id: ME-1")
	// An uncommitted change to the file the repair writes.
	u, err := model.ParseULID(b.ULID)
	if err != nil {
		t.Fatal(err)
	}
	if err := appendFile(filepath.Join(te.storeDir("me"), filepath.FromSlash(model.TaskPath(u))), "unsaved\n"); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := te.run("doctor", "--fix")
	if code == 0 || !strings.Contains(stderr, "uncommitted changes") || strings.Contains(out, "Nothing to repair") {
		t.Errorf("exit %d\nstdout %s\nstderr %s", code, out, stderr)
	}
}

func TestDoctorAcrossStores(t *testing.T) {
	t.Parallel()
	te := newTwoStoreEnv(t)
	te.goldenOK("clean", "doctor")
	te.goldenOK("clean-json", "doctor", "--json")
	te.goldenOK("clean-fix", "doctor", "--fix")
	te.goldenOK("one-store", "doctor", "--store", "alpha")
}

func TestDoctorCrossStoreRefIsAWarning(t *testing.T) {
	t.Parallel()
	te := newTwoStoreEnv(t)
	target := te.addJSON("--store", "bravo-long", "Bravo target")
	te.handEdit("alpha", "AA-2", "\nkind: ", "\nblocked_by: ["+target.ULID+"]\nkind: ")
	te.goldenRun("text", 1, "doctor")
	te.goldenRun("json", 1, "doctor", "--json")
	te.goldenRun("one-store", 1, "doctor", "--store", "alpha")
	te.goldenOK("other-store", "doctor", "--store", "bravo-long")
	te.goldenRun("fix", 1, "doctor", "--fix")
}

func TestDoctorFixOfAReadOnlyStoreWarns(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("add", "one")
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sync.WriteAccess(s, sync.Access{ReadOnly: true, Reason: sync.ReasonGitHub, Checked: te.now}); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := te.run("doctor", "--fix")
	if code != 0 || !strings.Contains(stderr, "warning: me is read-only, so it is not fixed (the GitHub permission is read)") || !strings.Contains(out, "Nothing to repair") {
		t.Errorf("exit %d\nstdout %s\nstderr %s", code, out, stderr)
	}
}
