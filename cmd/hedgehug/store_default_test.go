package main

import (
	"strings"
	"testing"
)

func TestStoreDefault(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	if out := te.ok("store", "default"); strings.TrimSpace(out) != "-" {
		t.Errorf("no default: %q", out)
	}
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("store", "init", "org", "--prefix", "ORG")
	if out := te.ok("store", "default"); strings.TrimSpace(out) != "me" {
		t.Errorf("the first store is the default: %q", out)
	}
	te.ok("store", "default", "org")
	if out := te.ok("store", "default", "--json"); !strings.Contains(out, `"default": "org"`) {
		t.Errorf("json = %q", out)
	}
	if code, _, errs := te.run("store", "default", "nope"); code != 1 || !strings.Contains(errs, "me, org") {
		t.Errorf("unknown name: exit %d, %q", code, errs)
	}
	te.ok("store", "remove", "org")
	if out := te.ok("store", "default"); strings.TrimSpace(out) != "-" {
		t.Errorf("after removing the default: %q", out)
	}
}
