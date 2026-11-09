package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

func TestDep(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	b := te.addJSON("Beta")
	te.ok("dep", "add", a.ID, "--blocked-by", b.ID)
	if bb := te.task(a.ULID).BlockedBy; len(bb) != 1 || bb[0].String() != b.ULID {
		t.Errorf("blocked_by = %v", bb)
	}
	if !strings.Contains(te.log("me")[0].Message, "Hedgehug-Op: dep") {
		t.Error("dep add is not a dep commit")
	}
	if code, _, stderr := te.run("dep", "add", a.ID, "--blocked-by", b.ID); code != 1 || !strings.Contains(stderr, "already") {
		t.Errorf("a second dep add: exit %d, stderr %q", code, stderr)
	}
	if code, _, stderr := te.run("dep", "add", b.ID, "--blocked-by", a.ID); code != 1 || !strings.Contains(stderr, "leads back") {
		t.Errorf("a cycle: exit %d, stderr %q", code, stderr)
	}
	te.ok("dep", "rm", a.ID, "--blocked-by", b.ID)
	if bb := te.task(a.ULID).BlockedBy; len(bb) != 0 {
		t.Errorf("blocked_by = %v after dep rm", bb)
	}
	if code, _, stderr := te.run("dep", "rm", a.ID, "--blocked-by", b.ID); code != 1 || !strings.Contains(stderr, "does not have") {
		t.Errorf("dep rm of nothing: exit %d, stderr %q", code, stderr)
	}
	if code, _, _ := te.run("dep", "add", a.ID); code != 2 {
		t.Errorf("dep add with no flag: exit %d, want 2", code)
	}
}

// Every relation has its flag, a flag can be given more than once, and all of
// them are written in one commit.
func TestDepManyFlags(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	b := te.addJSON("Beta")
	c := te.addJSON("Gamma")
	d := te.addJSON("Delta")
	head := len(te.log("me"))
	te.ok("dep", "add", a.ID, "--blocked-by", b.ID, "--causes", b.ID, "--causes", c.ID, "--relates", c.ID, "--duplicates", d.ID, "--clones", d.ID+","+b.ID)
	if n := len(te.log("me")); n != head+1 {
		t.Errorf("dep wrote %d commits, want 1", n-head)
	}
	got := te.task(a.ULID)
	for rel, want := range map[string][]string{
		"blocked_by": {b.ULID}, "causes": {b.ULID, c.ULID}, "relates": {c.ULID}, "duplicates": {d.ULID}, "clones": {d.ULID, b.ULID},
	} {
		r, _, _ := schema.RelationOf(rel)
		var have []string
		for _, u := range r.Get(got) {
			have = append(have, u.String())
		}
		if !slices.Equal(have, want) {
			t.Errorf("%s = %v, want %v", rel, have, want)
		}
	}
	if got.Props.Get("causes") != nil {
		t.Error("a relation went to props")
	}

	// A pair already tied is refused for the pair; nothing else is written.
	head = len(te.log("me"))
	code, _, stderr := te.run("dep", "add", a.ID, "--relates", c.ID, "--causes", c.ID)
	if code != 1 || !strings.Contains(stderr, "already has relates") || !strings.Contains(stderr, "already has causes") {
		t.Errorf("a tied pair: exit %d, stderr %q", code, stderr)
	}
	if n := len(te.log("me")); n != head {
		t.Error("a refused dep wrote")
	}
	te.ok("dep", "rm", a.ID, "--causes", b.ID, "--clones", d.ID)
	got = te.task(a.ULID)
	if len(got.Causes) != 1 || got.Causes[0].String() != c.ULID || len(got.Clones) != 1 || got.Clones[0].String() != b.ULID {
		t.Errorf("after dep rm: causes %v, clones %v", got.Causes, got.Clones)
	}
	// The other direction has no flag.
	if code, _, _ := te.run("dep", "add", a.ID, "--blocks", b.ID); code != 2 {
		t.Errorf("--blocks: exit %d, want 2", code)
	}
	// A task cannot point at itself except through blocked_by, which is a cycle.
	if code, _, stderr := te.run("dep", "add", a.ID, "--relates", a.ID); code != 1 || !strings.Contains(stderr, "自分自身は指せません") {
		t.Errorf("relates to itself: exit %d, stderr %q", code, stderr)
	}
	if code, _, stderr := te.run("dep", "add", a.ID, "--blocked-by", a.ID); code != 1 || !strings.Contains(stderr, "leads back") {
		t.Errorf("blocked-by itself: exit %d, stderr %q", code, stderr)
	}
}
