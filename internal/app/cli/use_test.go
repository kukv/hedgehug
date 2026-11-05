package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUse(t *testing.T) {
	t.Parallel()
	te := newTwoStoreEnv(t)
	te.goldenOK("none", "use")
	te.goldenOK("none-json", "use", "--json")
	te.goldenOK("set", "use", "alpha:AA-1")
	te.goldenOK("show", "use")
	te.goldenOK("show-json", "use", "--json")
	te.goldenOK("set-json", "use", "--json", "AA-1")
	te.goldenRun("not-project", 1, "use", "AA-2")
	te.goldenRun("not-found", 1, "use", "ZZ-9")
	te.goldenRun("both", 2, "use", "AA-1", "--clear")
	te.goldenOK("clear", "use", "--clear")
	te.goldenOK("clear-again", "use", "--clear")
	te.goldenOK("clear-json", "use", "--clear", "--json")
	te.goldenOK("cleared", "use")
}

func TestUseJSONShape(t *testing.T) {
	t.Parallel()
	te := newTwoStoreEnv(t)
	te.ok("use", "AA-1")
	var got struct {
		Store   string `json:"store"`
		Project struct {
			ID, ULID, Title string
		} `json:"project"`
	}
	if err := json.Unmarshal([]byte(te.ok("use", "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Store != "alpha" || got.Project.ID != "AA-1" || got.Project.Title != "Alpha project" || len(got.Project.ULID) != 26 {
		t.Errorf("use --json = %+v", got)
	}
}

func TestUseWarnsOfAProjectThatIsGone(t *testing.T) {
	t.Parallel()
	te := newTwoStoreEnv(t)
	te.ok("use", "AA-1")
	te.ok("rm", "AA-1")
	te.goldenRun("gone", 0, "use")
}

// hh use --json gives the id as the file has it, even where the text shows
// it with the store's name.
func TestUseJSONIDIsPlainWhenThePrefixIsShared(t *testing.T) {
	t.Parallel()
	te := newSharedPrefixEnv(t)
	te.addJSON("--store", "org-a", "Alpha project", "--kind", "project")
	te.addJSON("--store", "org-b", "Bravo project", "--kind", "project")
	te.ok("use", "org-b:API-2")
	if got := te.ok("use"); !strings.Contains(got, "org-b:API-2") {
		t.Fatalf("use = %q; want the id with the store's name", got)
	}
	var got struct {
		Store   string `json:"store"`
		Project struct {
			ID, Title string
		} `json:"project"`
	}
	if err := json.Unmarshal([]byte(te.ok("use", "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Store != "org-b" || got.Project.ID != "API-2" || got.Project.Title != "Bravo project" {
		t.Errorf("use --json = %+v; want org-b's API-2", got)
	}
}

// hh use --store looks in that store only.
func TestUseWithStoreLooksInThatStoreOnly(t *testing.T) {
	t.Parallel()
	te := newSharedPrefixEnv(t)
	te.addJSON("--store", "org-a", "Alpha project", "--kind", "project")
	te.addJSON("--store", "org-b", "Bravo project", "--kind", "project")
	if code, _, _ := te.run("use", "API-2"); code != 2 {
		t.Errorf("use API-2 in two stores: exit %d; want 2", code)
	}
	te.goldenOK("by-flag", "use", "--store", "org-b", "API-2")
	te.goldenRun("conflict", 2, "use", "--store", "org-b", "org-a:API-2")
	te.goldenRun("unknown", 1, "use", "--store", "nope", "API-2")
}
