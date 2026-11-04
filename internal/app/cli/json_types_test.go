package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/accounts"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/views"
	"github.com/kukv/hedgehug/internal/app/usecase/workspaces"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
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
		{"add", "a"},
		{"add", "b"},
		{"set", "ME-1", "title=A"},
		{"status", "ME-1", "todo"},
		{"done", "ME-1"},
		{"dep", "add", "ME-2", "--blocked-by", "ME-1"},
		{"dep", "rm", "ME-2", "--blocked-by", "ME-1"},
		{"watch", "ME-1"},
		{"unwatch", "ME-1"},
		{"mv", "ME-2", "--parent", "ME-1"},
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

	// The stores, the default store and the state of the syncs.
	var items []stores.StoreItem
	strictly(t, te.ok("store", "list", "--json"), &items)
	if len(items) != 1 || items[0].Name != "me" || !items[0].Default {
		t.Errorf("store list = %+v", items)
	}
	var dv stores.DefaultView
	strictly(t, te.ok("store", "default", "--json"), &dv)
	if dv.Name == nil || *dv.Name != "me" {
		t.Errorf("store default = %+v", dv)
	}
	var iv stores.StoreInitView
	strictly(t, te.ok("store", "init", "two", "--prefix", "TW", "--json"), &iv)
	if iv.Name != "two" || iv.Workspace.IDPrefix != "TW" || iv.Remotes == nil || iv.Pushed == nil {
		t.Errorf("store init = %+v", iv)
	}
	var svs []usecase.SyncStatusView
	strictly(t, te.ok("sync", "--status", "--json"), &svs)
	if len(svs) != 2 || svs[0].Interval <= 0 || svs[0].PushInterval <= 0 {
		t.Errorf("sync --status = %+v", svs)
	}

	// The inbox of nothing.
	var ns []notifications.Notification
	strictly(t, te.ok("inbox", "--json"), &ns)
	var rr notifications.ReadResult
	strictly(t, te.ok("inbox", "read", "--all", "--json"), &rr)
	if rr.Read == nil {
		t.Errorf("inbox read = %+v, want an empty array", rr)
	}
}

// store add prints a StoreView, and mv --store a MoveStoreResultView.
func TestStoreAddAndMoveJSONAreTheUsecaseTypes(t *testing.T) {
	t.Parallel()
	alice, _, remote := twoUsers(t)
	carol := alice.other()
	var sv stores.StoreView
	strictly(t, carol.ok("store", "add", "org", "--remote", remote, "--json"), &sv)
	if sv.Name != "org" || len(sv.Remotes) != 1 {
		t.Errorf("store add = %+v", sv)
	}

	te := newMoveStoreEnv(t)
	var mv stores.MoveStoreResultView
	strictly(t, te.ok("mv", "AA-2", "--store", "bravo-long", "--yes", "--json"), &mv)
	if mv.From != "alpha" || mv.To != "bravo-long" {
		t.Errorf("mv --store = %+v", mv)
	}
}

// store discover prints the candidates with a MountView.
func TestDiscoverJSONIsTheUsecaseTypes(t *testing.T) {
	t.Parallel()
	remotes := filepath.Join(t.TempDir(), "remotes")
	storesUnder(t, remotes, map[string]string{
		"https://github.com/org-a/aa.git": "AA",
		"https://github.com/org-b/bb.git": "BB",
	})
	te := newTestEnv(t)
	te.writeConfigYAML(rewriteTo(remotes))
	type discovered struct {
		Candidates []stores.Candidate `json:"candidates"`
		stores.MountView
	}
	var d discovered
	strictly(t, te.ok("store", "discover", "--from", remotes, "--json"), &d)
	if len(d.Candidates) != 2 || d.Mounted == nil || d.Failed == nil {
		t.Errorf("discover = %+v, want two candidates and empty arrays", d)
	}
	d = discovered{}
	strictly(t, te.ok("store", "discover", "--from", remotes, "--all", "--json"), &d)
	if len(d.Mounted) != 2 || d.Failed == nil {
		t.Errorf("discover --all = %+v", d)
	}
}

// auth login prints an AuthLoginResult, with uses for an account that takes
// its token from elsewhere, and auth logout an AuthLogoutResult.
func TestAuthJSONIsTheUsecaseTypes(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.withFake(fakegh.State{AutoApprove: "alice"})
	var lr accounts.AuthLoginResult
	strictly(t, te.ok("auth", "login", "--json"), &lr)
	if lr.Account != "alice" || lr.ID != "github.com/alice" || lr.Uses != "" {
		t.Errorf("login = %+v", lr)
	}
	var lo accounts.AuthLogoutResult
	strictly(t, te.ok("auth", "logout", "alice", "--json"), &lo)
	if lo.Account != "alice" || lo.ID != "github.com/alice" || !lo.Removed {
		t.Errorf("logout = %+v", lo)
	}

	env := newTestEnv(t)
	env.writeConfigYAML("accounts:\n  - {name: nt, host: github.com, login: z, token: \"env:HH_NOPE\"}\n")
	lr = accounts.AuthLoginResult{}
	strictly(t, env.ok("auth", "login", "nt", "--json"), &lr)
	if lr.Uses != "env:HH_NOPE" || lr.Account != "nt" {
		t.Errorf("login of an account that uses env = %+v", lr)
	}
	lo = accounts.AuthLogoutResult{}
	strictly(t, env.ok("auth", "logout", "nt", "--json"), &lo)
	if lo.Removed || lo.Token != "env:HH_NOPE" {
		t.Errorf("logout of an account that uses env = %+v", lo)
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

	// The items left are Notifications, and inbox read prints a ReadResult.
	var ns []notifications.Notification
	strictly(t, bob.ok("inbox", "--all", "--json"), &ns)
	if len(ns) == 0 || ns[0].ULID == "" || ns[0].Kind == "" || ns[0].Store != "org" {
		t.Fatalf("inbox --all = %+v", ns)
	}
	var rr notifications.ReadResult
	strictly(t, bob.ok("inbox", "read", ns[0].ULID, "--json"), &rr)
	if len(rr.Read) > 1 {
		t.Errorf("inbox read = %+v", rr)
	}
}
