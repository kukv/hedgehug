package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDoctorNoProblems(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("add", "Alpha")
	code, out, stderr := te.run("doctor")
	if code != 0 || out != "No problems\n" || stderr != "" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	code, out, _ = te.run("doctor", "--json")
	if code != 0 || strings.TrimSpace(out) != "[]" {
		t.Errorf("--json: exit %d, stdout %q", code, out)
	}
}

func TestDoctorBrokenSchemaFile(t *testing.T) {
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
	var fs []map[string]string
	if code != 1 || json.Unmarshal([]byte(out), &fs) != nil || len(fs) == 0 || fs[0]["check"] != "schema" {
		t.Errorf("--json: exit %d\n%s", code, out)
	}
}

func TestDoctorTakesNoArguments(t *testing.T) {
	te := newStoreEnv(t)
	if code, _, _ := te.run("doctor", "x"); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}

func TestDoctorFixWithoutRemote(t *testing.T) {
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
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "a")
	te.ok("add", "b")
	te.handEdit("me", "ME-2", "id: ME-2", "id: ME-1")
	code, out, _ := te.run("doctor", "--fix", "--json")
	var got struct {
		Repaired []struct {
			ULID, ID, Title, Kind string
			Value                 json.RawMessage
		}
		Findings []map[string]string
	}
	if code != 0 || json.Unmarshal([]byte(out), &got) != nil || len(got.Repaired) != 1 || got.Findings == nil || len(got.Findings) != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if r := got.Repaired[0]; r.Kind != "id_collision" || r.ID != "ME-2" || r.Title != "b" || len(r.ULID) != 26 || !strings.Contains(string(r.Value), `"old": "ME-1"`) {
		t.Errorf("repaired = %+v", r)
	}
}

func TestDoctorFixLeavesBrokenFiles(t *testing.T) {
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
	te := newStoreEnv(t)
	if code, _, _ := te.run("doctor", "--fix", "x"); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}

func TestDoctorMergeDriverAndInbox(t *testing.T) {
	te := newStoreEnv(t)
	bin := buildHedgehug(t)
	t.Setenv("PATH", "")
	code, out, _ := te.run("doctor")
	if code != 1 || !strings.Contains(out, "hedgehug is not on PATH") {
		t.Errorf("exit %d\n%s", code, out)
	}
	t.Setenv("PATH", filepath.Dir(bin))
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
