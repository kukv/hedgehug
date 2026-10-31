package ops

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

const bobAccount = "\naccounts:\n  - name: bob\n    host: github.com\n    login: bob\n"

// asWriter makes the accounts of config.yaml those of from's replacement to,
// and returns a session of its own, which has not read the writer yet.
func (ts *testSession) asWriter(from, to string) *Session {
	ts.t.Helper()
	_, p, err := ts.Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	b, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		ts.t.Fatal(err)
	}
	if err := os.WriteFile(p.ConfigFile, []byte(strings.Replace(string(b), from, to, 1)), 0o600); err != nil {
		ts.t.Fatal(err)
	}
	return &Session{Getenv: ts.Getenv, Now: ts.Now, Warn: ts.Warn}
}

func TestUndoTakesBackMyLastWrite(t *testing.T) {
	ts := newTestSession(t)
	ts.appendConfig(aliceAccount)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Set("ME-1", []Assign{{Key: "title", Op: "=", Value: "b"}}); err != nil {
		t.Fatal(err)
	}
	res, err := ts.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if !res.Found || !res.Changed || len(res.Kept) != 0 || UndoSubject(res.Commit) != "set ME-1 b" {
		t.Errorf("Undo = found %v, changed %v, kept %v, subject %q; want true, true, none, \"set ME-1 b\"", res.Found, res.Changed, res.Kept, UndoSubject(res.Commit))
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
	ts := newTestSession(t)
	ts.appendConfig(aliceAccount)
	_, err := ts.Undo()
	var oe *Error
	if KindOf(err) != KindConflict || !errors.As(err, &oe) || oe.Code != "undo.nothing" || err.Error() != "nothing to undo" {
		t.Fatalf("Undo = %v (kind %v); want KindConflict undo.nothing \"nothing to undo\"", err, KindOf(err))
	}
}

func TestUndoNeedsAccounts(t *testing.T) {
	ts := newTestSession(t)
	_, err := ts.Undo()
	if err == nil || err.Error() != "accounts are not set up in config.yaml; hedgehug undo needs them to find your operations" {
		t.Fatalf("Undo = %v; want the accounts message", err)
	}
	var oe *Error
	if KindOf(err) != KindConflict || !errors.As(err, &oe) || oe.Code != "account.none" {
		t.Errorf("Undo = %v (kind %v); want KindConflict account.none", err, KindOf(err))
	}
}

func TestUndoKeepsWhatOthersChangedAfterwards(t *testing.T) {
	ts := newTestSession(t)
	ts.appendConfig(aliceAccount)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Set("ME-1", []Assign{{Key: "title", Op: "=", Value: "b"}}); err != nil {
		t.Fatal(err)
	}
	bob := ts.asWriter(aliceAccount, bobAccount)
	if _, err := bob.SetStatus("ME-1", "in_progress"); err != nil {
		t.Fatal(err)
	}
	alice := ts.asWriter(bobAccount, aliceAccount)
	res, err := alice.Undo()
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
	ts := newTestSession(t)
	ts.appendConfig(aliceAccount)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Set("ME-1", []Assign{{Key: "title", Op: "=", Value: "b"}}); err != nil {
		t.Fatal(err)
	}
	bob := ts.asWriter(aliceAccount, bobAccount)
	if _, err := bob.Set("ME-1", []Assign{{Key: "title", Op: "=", Value: "c"}}); err != nil {
		t.Fatal(err)
	}
	alice := ts.asWriter(bobAccount, aliceAccount)
	res, err := alice.Undo()
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
	ts := newTestSession(t)
	ts.appendConfig(aliceAccount)
	s, _, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	ws := &model.Workspace{ULID: model.NewULID(), Name: "w2", IDPrefix: "WW"}
	if _, err := ts.Update(s, store.Op{Name: "workspace-add"}, func(tx *store.Tx) error { return tx.PutWorkspace(ws) }); err != nil {
		t.Fatal(err)
	}
	bob := ts.asWriter(aliceAccount, bobAccount)
	if _, err := bob.Add(AddInput{Title: "in w2", Workspace: "w2"}); err != nil {
		t.Fatal(err)
	}
	alice := ts.asWriter(bobAccount, aliceAccount)
	_, err = alice.Undo()
	var oe *Error
	if KindOf(err) != KindConflict || !errors.As(err, &oe) || oe.Code != "undo.blocked" || !strings.Contains(err.Error(), "the workspace still has 1 tasks") {
		t.Fatalf("Undo = %v; want KindConflict undo.blocked naming 1 task", err)
	}
}

func TestUndoTarget(t *testing.T) {
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
		got, ok := UndoTarget(c.log, "me")
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

func TestCountTasks(t *testing.T) {
	ts := newTestSession(t)
	for _, title := range []string{"live", "gone"} {
		if _, err := ts.Add(AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ts.Remove("ME-2"); err != nil {
		t.Fatal(err)
	}
	_, ix, _ := ts.Open()
	ws := ix.Workspaces()[0].ULID
	if n := CountTasks(ix, ws, false); n != 1 {
		t.Errorf("CountTasks(live) = %d; want 1", n)
	}
	if n := CountTasks(ix, ws, true); n != 2 {
		t.Errorf("CountTasks(all) = %d; want 2", n)
	}
}
