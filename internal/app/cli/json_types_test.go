package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/views"
	"github.com/kukv/hedgehug/internal/app/usecase/workspaces"
)

// strictly reads out as JSON into v, and refuses a key v does not have.
func strictly(t *testing.T, out string, v any) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader([]byte(out)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
}

// Every write prints the type the server answers with.
func TestJSONIsTheUsecaseTypes(t *testing.T) {
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")

	var tw usecase.TaskWrite
	for _, args := range [][]string{
		{"add", "a"}, {"add", "b"}, {"set", "ME-1", "title=A"}, {"status", "ME-1", "todo"}, {"done", "ME-1"},
		{"dep", "add", "ME-2", "--blocked-by", "ME-1"}, {"dep", "rm", "ME-2", "--blocked-by", "ME-1"},
		{"watch", "ME-1"}, {"unwatch", "ME-1"}, {"mv", "ME-2", "--parent", "ME-1"},
		{"restore", "ME-1", "--at", "2999-01-01"},
	} {
		tw = usecase.TaskWrite{}
		strictly(t, te.ok(append(args, "--json")...), &tw)
		if tw.Task == nil || tw.Task.ID == "" {
			t.Errorf("%v: task = %+v", args, tw.Task)
		}
	}
	x := te.addJSON("edited")
	te.editWith(setLine(te.raw(x.ULID), "title", "Again"))
	strictly(t, te.ok("edit", x.ID, "--json"), &tw)

	var aw usecase.AffectedWrite
	strictly(t, te.ok("rm", "ME-1", "--json"), &aw)
	if len(aw.Tasks) != 2 || aw.Tasks[0].ID != "ME-1" || aw.Commit == nil {
		t.Errorf("rm = %+v", aw)
	}
	aw = usecase.AffectedWrite{}
	strictly(t, te.ok("restore", "ME-1", "--json"), &aw)

	var cw comments.CommentWrite
	strictly(t, te.ok("comment", "add", "ME-3", "hello", "--json"), &cw)
	te.editWith("changed\n")
	strictly(t, te.ok("comment", "edit", cw.Comment.ULID, "--json"), &cw)
	strictly(t, te.ok("comment", "rm", cw.Comment.ULID, "--json"), &cw)
	if cw.Comment.Task == "" || cw.Comment.DeletedAt == nil || cw.Commit == nil {
		t.Errorf("comment rm = %+v", cw)
	}

	var vw views.ViewWrite
	strictly(t, te.ok("view", "save", "v", "--layout", "table", "--json"), &vw)
	strictly(t, te.ok("view", "rename", "v", "w", "--json"), &vw)
	strictly(t, te.ok("view", "rm", "w", "--json"), &vw)

	var ww workspaces.WorkspaceWrite
	strictly(t, te.ok("workspace", "add", "web", "--prefix", "WEB", "--json"), &ww)
	strictly(t, te.ok("workspace", "rename", "web", "front", "--json"), &ww)
	strictly(t, te.ok("workspace", "set-prefix", "me", "NEW", "--json"), &ww)
	if len(ww.IDs) == 0 {
		t.Errorf("set-prefix ids = %+v", ww)
	}
	strictly(t, te.ok("workspace", "rm", "front", "--json"), &ww)

	var mw schemas.MigrateWrite
	strictly(t, te.ok("schema", "migrate", "status", "todo", "ready", "--json"), &mw)
	var cr schemas.CheckResult
	strictly(t, te.ok("schema", "check", "--json"), &cr)

	var uw tasks.UndoWrite
	strictly(t, te.ok("undo", "--json"), &uw)
	if uw.Undone == nil || uw.Commit == nil {
		t.Errorf("undo = %+v", uw)
	}
}

// inbox revert prints a TaskWrite too.
func TestInboxRevertJSONIsTaskWrite(t *testing.T) {
	t.Parallel()
	alice, bob, _ := twoUsers(t)
	conflict(t, alice, bob, "title", "bob", "alice")
	var tw usecase.TaskWrite
	strictly(t, bob.ok("inbox", "revert", itemOfKind(t, bob, "overwritten"), "--json"), &tw)
	if tw.Task == nil || tw.Task.Title != "bob" || tw.Commit == nil {
		t.Errorf("inbox revert = %+v", tw)
	}
}
