package tasks_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/app/usecase/workspaces"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

func TestUndoTakesBackMyLastWrite(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Set(ts.Session, "ME-1", []usecase.Assign{{Key: "title", Op: "=", Value: "b"}}); err != nil {
		t.Fatal(err)
	}
	res, err := tasks.Undo(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Found || !res.Changed || len(res.Kept) != 0 || tasks.UndoSubject(res.Undone.Subject) != "set ME-1 b" {
		t.Errorf("Undo = found %v, changed %v, kept %v, subject %q; want true, true, none, \"set ME-1 b\"", res.Found, res.Changed, res.Kept, tasks.UndoSubject(res.Undone.Subject))
	}
	_, ix, _ := ts.Open()
	e, err := ts.Resolve(ix, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if e.Title != "a" {
		t.Errorf("title after undo = %q; want a", e.Title)
	}
}

func TestUndoOnEmptyHistoryIsConflict(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	_, err := tasks.Undo(ts.Session)
	var oe *usecase.Error
	if usecase.KindOf(err) != usecase.KindConflict || !errors.As(err, &oe) || oe.Code != "undo.nothing" || err.Error() != "nothing to undo" {
		t.Fatalf("Undo = %v (kind %v); want KindConflict undo.nothing \"nothing to undo\"", err, usecase.KindOf(err))
	}
}

func TestUndoNeedsAccounts(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.DropDefaultAccount()
	_, err := tasks.Undo(ts.Session)
	if err == nil || err.Error() != "no account is set up; hedgehug undo needs one to find your operations; add one with hedgehug auth add <name> --login <login> (or hedgehug auth login)" {
		t.Fatalf("Undo = %v; want the accounts message", err)
	}
	var oe *usecase.Error
	if usecase.KindOf(err) != usecase.KindConflict || !errors.As(err, &oe) || oe.Code != "account.none" {
		t.Errorf("Undo = %v (kind %v); want KindConflict account.none", err, usecase.KindOf(err))
	}
}

func TestUndoKeepsWhatOthersChangedAfterwards(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Set(ts.Session, "ME-1", []usecase.Assign{{Key: "title", Op: "=", Value: "b"}}); err != nil {
		t.Fatal(err)
	}
	bob := ts.AsWriter(usecasetest.AliceAccount, usecasetest.BobAccount)
	if _, err := tasks.SetStatus(bob, "ME-1", "in_progress"); err != nil {
		t.Fatal(err)
	}
	alice := ts.AsWriter(usecasetest.BobAccount, usecasetest.AliceAccount)
	res, err := tasks.Undo(alice)
	if err != nil {
		t.Fatal(err)
	}
	// The title goes back; the status bob changed after the commit stays.
	if !res.Changed {
		t.Errorf("Changed = false; want true (the title went back)")
	}
	_, ix, _ := alice.Open()
	e, _ := alice.Resolve(ix, "ME-1")
	if e.Title != "a" || e.Status != "in_progress" {
		t.Errorf("after undo title %q status %q; want a, in_progress", e.Title, e.Status)
	}
}

func TestUndoKeptNamesTheFieldAndItsAuthor(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Set(ts.Session, "ME-1", []usecase.Assign{{Key: "title", Op: "=", Value: "b"}}); err != nil {
		t.Fatal(err)
	}
	bob := ts.AsWriter(usecasetest.AliceAccount, usecasetest.BobAccount)
	if _, err := tasks.Set(bob, "ME-1", []usecase.Assign{{Key: "title", Op: "=", Value: "c"}}); err != nil {
		t.Fatal(err)
	}
	alice := ts.AsWriter(usecasetest.BobAccount, usecasetest.AliceAccount)
	res, err := tasks.Undo(alice)
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed || len(res.Kept) != 1 {
		t.Fatalf("Undo = changed %v, kept %v; want unchanged and one kept field", res.Changed, res.Kept)
	}
	k := res.Kept[0]
	if k.Task != "ME-1" || k.Field != "title" || k.Author != "github.com/bob" || !strings.HasPrefix(k.Path, "tasks/") {
		t.Errorf("kept = %+v; want ME-1 title by github.com/bob in a task file", k)
	}
}

func TestUndoBlockedByTasksOfAWorkspace(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	s, _, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	ws := &model.Workspace{ULID: model.NewULID(), Name: "w2", IDPrefix: "WW"}
	if _, err := ts.Update(s, store.Op{Name: "workspace-add"}, func(tx *store.Tx) error { return tx.PutWorkspace(ws) }); err != nil {
		t.Fatal(err)
	}
	bob := ts.AsWriter(usecasetest.AliceAccount, usecasetest.BobAccount)
	if _, err := tasks.Add(bob, tasks.AddInput{Title: "in w2", Workspace: "w2"}); err != nil {
		t.Fatal(err)
	}
	alice := ts.AsWriter(usecasetest.BobAccount, usecasetest.AliceAccount)
	_, err = tasks.Undo(alice)
	var oe *usecase.Error
	if usecase.KindOf(err) != usecase.KindConflict || !errors.As(err, &oe) || oe.Code != "undo.blocked" || !strings.Contains(err.Error(), "the workspace still has 1 task") {
		t.Fatalf("Undo = %v; want KindConflict undo.blocked naming 1 task", err)
	}
}

func TestUndoRestoresARemovedWorkspace(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	if _, err := workspaces.WorkspaceAdd(ts.Session, "w2", "WW"); err != nil {
		t.Fatal(err)
	}
	if _, err := workspaces.WorkspaceRemove(ts.Session, "w2"); err != nil {
		t.Fatal(err)
	}
	if wss, err := workspaces.WorkspaceList(ts.Session); err != nil || listsWorkspace(wss, "w2") {
		t.Fatalf("workspaces after rm = %v, %v; want w2 gone", wss, err)
	}
	res, err := tasks.Undo(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Found || !res.Changed || len(res.Kept) != 0 {
		t.Errorf("Undo = found %v, changed %v, kept %v; want true, true, none", res.Found, res.Changed, res.Kept)
	}
	wss, err := workspaces.WorkspaceList(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	if !listsWorkspace(wss, "w2") {
		t.Errorf("workspaces after undo = %v; want w2 back", wss)
	}
}

func listsWorkspace(wss []usecase.WorkspaceView, name string) bool {
	for _, w := range wss {
		if w.Name == name {
			return true
		}
	}
	return false
}

func TestUndoScannerTarget(t *testing.T) {
	t.Parallel()
	hash := func(b byte) plumbing.Hash { return plumbing.Hash{b} }
	entry := func(h byte, author, op string, parents int, undoes ...byte) gitrepo.LogEntry {
		e := gitrepo.LogEntry{Hash: hash(h), Trailers: []gitrepo.Trailer{{Key: "Hedgehug-Author", Value: author}, {Key: "Hedgehug-Op", Value: op}}}
		for range parents {
			e.Parents = append(e.Parents, hash(0xff))
		}
		for _, u := range undoes {
			e.Trailers = append(e.Trailers, gitrepo.Trailer{Key: "Hedgehug-Undo", Value: hash(u).String()})
		}
		return e
	}
	cases := []struct {
		name string
		log  []gitrepo.LogEntry
		want int // index into log, -1 for none
	}{
		{"newest of mine", []gitrepo.LogEntry{entry(1, "me", "set", 1), entry(2, "me", "add", 1)}, 0},
		{"others are skipped", []gitrepo.LogEntry{entry(1, "you", "set", 1), entry(2, "me", "add", 1)}, 1},
		{"merge repair setup undo init are skipped", []gitrepo.LogEntry{
			entry(1, "me", "merge", 2), entry(2, "me", "repair", 1), entry(3, "me", "setup", 1),
			entry(4, "me", "undo", 1), entry(5, "me", "init", 1), entry(6, "me", "add", 1),
		}, 5},
		{"the root commit is skipped", []gitrepo.LogEntry{entry(1, "me", "add", 0)}, -1},
		{"an undone one is skipped", []gitrepo.LogEntry{entry(3, "me", "undo", 1, 2), entry(2, "me", "set", 1), entry(1, "me", "add", 1)}, 2},
		{"empty", nil, -1},
	}
	for _, c := range cases {
		var sc tasks.UndoScanner
		var got gitrepo.LogEntry
		var ok bool
		for _, e := range c.log {
			if got, ok = sc.Next(e, "me"); ok {
				break
			}
		}
		if c.want < 0 {
			if ok {
				t.Errorf("%s: target %v; want none", c.name, got.Hash)
			}
			continue
		}
		if !ok || got.Hash != c.log[c.want].Hash {
			t.Errorf("%s: target %v, %v; want log[%d]", c.name, got.Hash, ok, c.want)
		}
	}
}

func TestUndoScannerStopsAtTarget(t *testing.T) {
	t.Parallel()
	hash := func(b byte) plumbing.Hash { return plumbing.Hash{b} }
	other := func(b byte) gitrepo.LogEntry {
		return gitrepo.LogEntry{Hash: hash(b), Parents: []plumbing.Hash{hash(0xff)}, Trailers: []gitrepo.Trailer{{Key: "Hedgehug-Author", Value: "you"}, {Key: "Hedgehug-Op", Value: "set"}}}
	}
	mine := gitrepo.LogEntry{Hash: hash(3), Parents: []plumbing.Hash{hash(0xff)}, Trailers: []gitrepo.Trailer{{Key: "Hedgehug-Author", Value: "me"}, {Key: "Hedgehug-Op", Value: "add"}}}
	log := []gitrepo.LogEntry{other(10), other(9), mine}
	for b := byte(8); b >= 4; b-- {
		log = append(log, other(b))
	}
	var sc tasks.UndoScanner
	n := 0
	var got gitrepo.LogEntry
	for _, e := range log {
		n++
		if c, ok := sc.Next(e, "me"); ok {
			got = c
			break
		}
	}
	if n != 3 {
		t.Fatalf("read %d entries, want 3", n)
	}
	if got.Hash != mine.Hash {
		t.Fatalf("target %v, want %v", got.Hash, mine.Hash)
	}
}

func TestCountTasks(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	for _, title := range []string{"live", "gone"} {
		if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tasks.Remove(ts.Session, "ME-2"); err != nil {
		t.Fatal(err)
	}
	_, ix, _ := ts.Open()
	ws := ix.Workspaces()[0].ULID
	if n := usecase.CountTasks(ix, ws, false); n != 1 {
		t.Errorf("CountTasks(live) = %d; want 1", n)
	}
	if n := usecase.CountTasks(ix, ws, true); n != 2 {
		t.Errorf("CountTasks(all) = %d; want 2", n)
	}
}
