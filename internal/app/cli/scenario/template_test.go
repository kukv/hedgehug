package scenario

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTemplateAndBulkFlow writes a template in an editor, expands it, sees the
// tasks from the other clone, and closes them with one bulk update.
func TestTemplateAndBulkFlow(t *testing.T) {
	t.Parallel()
	bin, devenv := buildHedgehug(t), buildDevenv(t)
	dev, _, _ := initDevenv(t, devenv)
	alice, bob := devUserOf(t, bin, devenv, dev, "alice"), devUserOf(t, bin, devenv, dev, "bob")
	const org = "--store=org-a"

	// The editor uses only what the shell has: a CI job runs this with no
	// commands on PATH.
	const content = "---\nkind: task\ntitle: Bug\nlabels: [bug]\nchildren:\n  - title: Reproduce\n    labels: [bug]\n  - title: Fix\n    labels: [bug]\n---\n"
	editor := filepath.Join(t.TempDir(), "editor.sh")
	if err := os.WriteFile(editor, []byte("#!/bin/sh\nprintf '%s' '"+content+"' > \"$1\"\n"), 0o700); err != nil { //nolint:gosec // the editor of the test runs
		t.Fatal(err)
	}
	alice.env = append(alice.env, "EDITOR="+editor)
	alice.ok("schema", "labels", "add", "bug", org)
	alice.ok("template", "edit", "bug", org)
	alice.ok("add", "Platform", org) // ORG-2; a task sits under a project
	if got := alice.ok("add", "--template", "bug", "Login fails", "--parent", "ORG-2", org); !strings.Contains(got, "with 2 tasks under it") {
		t.Fatalf("add:\n%s", got)
	}
	alice.ok("sync")
	bob.ok("sync")

	if got := bob.ok("set", "--query", "label:bug", "status=done", "--dry-run", org); !strings.Contains(got, "Reproduce") || !strings.Contains(got, "Login fails") {
		t.Fatalf("dry run:\n%s", got)
	}
	if got := bob.ok("set", "--query", "label:bug", "status=done", "--yes", org); !strings.Contains(got, "Updated 3 tasks") {
		t.Fatalf("set:\n%s", got)
	}
	bob.ok("sync")
	alice.ok("sync")
	if got := alice.ok("ls", "label:bug", "status:done", org); !strings.Contains(got, "Fix") || !strings.Contains(got, "Login fails") {
		t.Fatalf("after sync:\n%s", got)
	}
}
