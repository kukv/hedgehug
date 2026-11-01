package main

import (
	"strings"
	"testing"
)

func TestAddJSONAtTheRootHasNullParent(t *testing.T) {
	te := newStoreEnv(t)
	out := te.ok("add", "root", "--json")
	if !strings.Contains(out, `"parent": null`) || !strings.Contains(out, `"store": "me"`) || !strings.Contains(out, `"workspace": {`) {
		t.Errorf("stdout = %s", out)
	}
}

func TestAddContextGolden(t *testing.T) {
	te := newStoreEnv(t)
	te.cloneDir("", "https://github.com/org-a/web")
	te.goldenRun("project", 0, "add", "Web A", "--kind", "project")
	te.goldenRun("second-project", 0, "add", "Web B", "--kind", "project", "--workspace", "me")
	te.goldenRun("choice-required", 2, "add", "ambiguous")
	te.goldenRun("no-repo", 0, "add", "unlinked", "--no-repo", "--workspace", "me")
	te.terminal = true
	te.stdin = "1\n"
	te.goldenRun("choice-terminal", 0, "add", "picked")
	te.stdin = "\n"
	te.goldenRun("choice-default", 0, "add", "default")
	te.terminal = false
	te.goldenRun("in-clone", 0, "add", "login page")
	te.goldenRun("in-clone-json", 0, "add", "second", "--json")

	te.cloneDir("hh/ME-2-login", "https://github.com/org-a/other")
	te.terminal = true
	te.stdin = "\n"
	te.goldenRun("branch-yes", 0, "add", "on the branch")
	te.stdin = "n\n"
	te.goldenRun("branch-no", 0, "add", "off the branch")
}
