package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
)

// The whole round: origin is force pushed, hh sync stops and says why, hh
// doctor tells of it, hh doctor --fix clones the store again with the
// unpushed work put back, and the sync after it pushes that work.
func TestDoctorFixClonesAForcePushedStoreAgain(t *testing.T) {
	t.Parallel()
	alice, bob, remote := twoUsers(t)
	bob.ok("add", "bobs")
	gitrepotest.RewriteRemote(t, remote)

	code, _, errs := bob.run("sync")
	if code != 1 || !strings.Contains(errs, "differs from this store's") {
		t.Errorf("sync: exit %d, stderr %q", code, errs)
	}
	code, out, _ := bob.run("doctor")
	if code != 1 || !strings.Contains(out, "org: the history of origin differs from this store's") || !strings.Contains(out, "hint: ") {
		t.Errorf("doctor: exit %d\n%s", code, out)
	}

	out = bob.ok("doctor", "--fix")
	if !strings.Contains(out, "Cloned org again from origin; put back 1 unpushed commit\n") || strings.Contains(out, "Could not put back") || !strings.Contains(out, "No problems") {
		t.Errorf("doctor --fix:\n%s", out)
	}
	if ls := bob.ok("ls"); !strings.Contains(ls, "bobs") || !strings.Contains(ls, "ORG-1") {
		t.Errorf("ls after the fix:\n%s", ls)
	}
	bob.ok("sync") // pushes the work onto the new history

	// alice followed the old history: her sync stops, and --fix catches her up
	code, _, errs = alice.run("sync")
	if code != 1 || !strings.Contains(errs, "differs from this store's") {
		t.Errorf("alice's sync: exit %d, stderr %q", code, errs)
	}
	out = alice.ok("doctor", "--fix")
	if !strings.Contains(out, "Cloned org again from origin; put back 0 unpushed commits\n") {
		t.Errorf("alice's doctor --fix:\n%s", out)
	}
	alice.ok("sync")
	if ls := alice.ok("ls"); !strings.Contains(ls, "bobs") {
		t.Errorf("alice's ls after the fix:\n%s", ls)
	}
}

// doctor --fix --json has the rebuild in "rebuilt"; a store that was not
// force pushed has none.
func TestDoctorFixJSONHasTheRebuild(t *testing.T) {
	t.Parallel()
	_, bob, remote := twoUsers(t)
	bob.ok("add", "bobs")
	var plain []struct {
		Rebuilt *struct{} `json:"rebuilt"`
	}
	if err := json.Unmarshal([]byte(bob.ok("doctor", "--fix", "--json")), &plain); err != nil || len(plain) != 1 || plain[0].Rebuilt != nil {
		t.Fatalf("doctor --fix --json of a store not force pushed: %+v, %v", plain, err)
	}
	gitrepotest.RewriteRemote(t, remote)
	bob.run("sync")
	var rs []struct {
		Rebuilt *struct {
			Replayed int      `json:"replayed"`
			Dropped  []string `json:"dropped"`
			Kept     string   `json:"kept"`
		} `json:"rebuilt"`
	}
	if err := json.Unmarshal([]byte(bob.ok("doctor", "--fix", "--json")), &rs); err != nil || len(rs) != 1 || rs[0].Rebuilt == nil {
		t.Fatalf("doctor --fix --json: %+v, %v", rs, err)
	}
	if r := rs[0].Rebuilt; r.Replayed != 1 || r.Dropped == nil || len(r.Dropped) != 0 || r.Kept != "" {
		t.Errorf("rebuilt = %+v", r)
	}
}
