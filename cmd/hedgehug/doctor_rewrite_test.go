package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/sync"
)

// A swap after a history rewrite that stopped (the store moved away, the
// rebuilt one not yet in its place) is a warning of doctor, and --fix swaps
// it in.
func TestDoctorFixFinishesAStoppedRewriteSwap(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("add", "Alpha")
	dir := te.storeDir("me")
	if err := os.CopyFS(dir+".rewrite-new", os.DirFS(dir)); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(dir, dir+".before-rewrite-20260101T000000.000000000"); err != nil {
		t.Fatal(err)
	}

	code, out, _ := te.run("doctor")
	if code != 1 || strings.Count(out, "is left from a rebuild after a history rewrite (warning)") != 2 || !strings.Contains(out, "hint: run hedgehug doctor --fix") {
		t.Errorf("doctor: exit %d\n%s", code, out)
	}
	code, out, _ = te.run("doctor", "--json")
	var rs []struct {
		Findings []map[string]string
	}
	if code != 1 || json.Unmarshal([]byte(out), &rs) != nil || len(rs) != 1 || len(rs[0].Findings) != 2 ||
		rs[0].Findings[0]["check"] != "rewrite_leftover" || rs[0].Findings[0]["severity"] != "warning" {
		t.Errorf("--json: exit %d\n%s", code, out)
	}

	if code, out, stderr := te.run("doctor", "--fix"); code != 0 || !strings.Contains(out, "No problems") {
		t.Errorf("--fix: exit %d\nstdout %s\nstderr %s", code, out, stderr)
	}
	if left, err := sync.FindLeftover(dir); err != nil || !left.None() {
		t.Errorf("leftover = %+v, %v", left, err)
	}
	if out := te.ok("ls"); !strings.Contains(out, "Alpha") {
		t.Errorf("ls after the fix:\n%s", out)
	}
}
