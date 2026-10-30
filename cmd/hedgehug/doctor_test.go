package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
