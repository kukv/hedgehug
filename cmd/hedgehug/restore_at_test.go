package main

import (
	"strings"
	"testing"
	"time"
)

// The store clock is the real one, which commands in tests cannot replace,
// so the commits of these tests are told apart by waiting.
const storeTick = 1100 * time.Millisecond

func TestRestoreAt(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	time.Sleep(storeTick)
	te.ok("add", "v1")
	time.Sleep(storeTick)
	at := time.Now().UTC().Format(time.RFC3339)
	time.Sleep(storeTick)
	te.ok("set", "ME-1", "title=v2")
	te.ok("rm", "ME-1")
	out := te.ok("restore", "ME-1", "--at", at)
	if !strings.HasPrefix(out, "Restored ME-1") {
		t.Errorf("restore = %s", out)
	}
	got := te.ok("show", "ME-1", "--json")
	if !strings.Contains(got, `"title": "v1"`) || strings.Contains(got, `"deleted_at": "2`) {
		t.Errorf("show = %s", got)
	}
	c := te.headCommit("me")
	if !strings.Contains(c.Message, "Hedgehug-Op: restore") {
		t.Errorf("message = %s", c.Message)
	}
	_, _, errs := te.run("restore", "ME-1", "--at", "2000-01-01T00:00:00Z")
	if !strings.Contains(errs, "ME-1 did not exist at 2000-01-01T00:00:00Z") {
		t.Errorf("before creation: %s", errs)
	}
	if code, _, errs := te.run("restore", "ME-1", "--at", "yesterday-ish"); code == 0 || !strings.Contains(errs, "neither a time nor a commit") {
		t.Errorf("bad point: code %d %s", code, errs)
	}
}

func TestRestoreAtCommitAndWorkspaceNote(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "WEB", "--workspace", "web")
	te.ok("workspace", "add", "api", "--prefix", "API")
	te.ok("add", "x", "--workspace", "web")
	h := te.headCommit("me").Hash.String()[:7]
	te.ok("mv", "WEB-1", "--workspace", "api")
	te.ok("set", "API-1", "title=later")
	out := te.ok("restore", "API-1", "--at", h)
	if !strings.Contains(out, "the workspace stays api; move it with hedgehug mv API-1 --workspace web") {
		t.Errorf("restore = %s", out)
	}
	if got := te.ok("show", "API-1", "--json"); !strings.Contains(got, `"title": "x"`) || !strings.Contains(got, `"id": "API-1"`) {
		t.Errorf("show = %s", got)
	}
}

func TestRestoreAtRefusedParent(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "p")
	te.ok("add", "c", "--parent", "ME-1")
	h := te.headCommit("me").Hash.String()
	te.ok("mv", "ME-2", "--root")
	te.ok("rm", "ME-1")
	before := te.headCommit("me").Hash
	if code, _, errs := te.run("restore", "ME-2", "--at", h); code != 1 || !strings.Contains(errs, "the parent ME-1") || !strings.Contains(errs, "is deleted") {
		t.Errorf("the parent is deleted: code %d stderr %s", code, errs)
	}
	if te.headCommit("me").Hash != before {
		t.Error("a refused restore --at committed")
	}
}

func TestParseAt(t *testing.T) {
	loc := time.FixedZone("x", 9*3600)
	for _, c := range []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"2026-10-30T10:00:00+02:00", time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC), true},
		{"2026-10-30", time.Date(2026, 10, 30, 23, 59, 59, 0, loc), true},
		{"abc1234", time.Time{}, false},
		{"2026-13-45", time.Time{}, false},
	} {
		got, ok := parseAtTime(c.in, loc)
		if ok != c.ok || !got.Equal(c.want) {
			t.Errorf("parseAtTime(%q) = %v %v, want %v %v", c.in, got, ok, c.want, c.ok)
		}
	}
}
