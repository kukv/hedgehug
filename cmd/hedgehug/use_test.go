package main

import (
	"encoding/json"
	"testing"
)

func TestUse(t *testing.T) {
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
	te := newTwoStoreEnv(t)
	te.ok("use", "AA-1")
	te.ok("rm", "AA-1")
	te.goldenRun("gone", 0, "use")
}
