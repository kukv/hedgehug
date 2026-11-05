package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
)

func TestLinkCommands(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	if out := te.ok("link", a.ID, "--pr", "org-a/web#1"); !strings.Contains(out, "Linked "+a.ID) {
		t.Errorf("link: %s", out)
	}
	if out := te.ok("unlink", a.ID, "--pr", "org-a/web#1"); !strings.Contains(out, "Unlinked "+a.ID) {
		t.Errorf("unlink: %s", out)
	}
	if code, _, _ := te.run("link", a.ID); code != 2 {
		t.Errorf("no target: %d", code)
	}
	if code, _, _ := te.run("link", a.ID, "--pr", "org-a/web#1", "--issue", "org-a/web#2"); code != 2 {
		t.Errorf("both: %d", code)
	}
}

func TestLinkJSON(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	var got usecase.TaskWrite
	if err := json.Unmarshal([]byte(te.ok("link", a.ID, "--issue", "org-a/web#2", "--sync", "push", "--json")), &got); err != nil || got.Task == nil || got.Commit == nil {
		t.Fatalf("link --json = %+v, %v", got, err)
	}
	if l := te.task(a.ULID).Links; len(l) != 1 || l[0].Sync != "push" || l[0].Ref != "github.com/org-a/web/issues/2" {
		t.Errorf("links = %+v", l)
	}
}
