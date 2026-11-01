package main

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
	hhsync "github.com/kukv/hedgehug/internal/sync"
)

// newMoveStoreEnv is newTwoStoreEnv (alpha AA-1 project and AA-2 under it;
// bravo-long BB-1, BB-2) with AA-3 blocked by AA-2.
func newMoveStoreEnv(t *testing.T) *testEnv {
	t.Helper()
	te := newTwoStoreEnv(t)
	te.addJSON("--store", "alpha", "Loose")
	te.ok("dep", "add", "AA-3", "--blocked-by", "AA-2")
	return te
}

// putSchema changes the schema of the store name.
func (te *testEnv) putStoreSchema(name string, fn func(*schema.Schema)) {
	te.t.Helper()
	s, err := store.Open(te.storeDir(name))
	if err != nil {
		te.t.Fatal(err)
	}
	b, err := s.Schema().Encode()
	if err != nil {
		te.t.Fatal(err)
	}
	sc, err := schema.Decode(b)
	if err != nil {
		te.t.Fatal(err)
	}
	fn(sc)
	if _, err := s.Update(store.Op{Name: "schema", Author: gitrepo.Signature{Name: "Alice", Email: "alice@example.com"}}, func(tx *store.Tx) error { return tx.PutSchema(sc) }); err != nil {
		te.t.Fatal(err)
	}
}

func TestMvStoreYes(t *testing.T) {
	te := newMoveStoreEnv(t)
	got := te.ok("mv", "AA-1", "--store", "bravo-long", "--yes")
	te.golden("out", got)
	te.goldenOK("ls", "ls", "--store", "bravo-long")
}

func TestMvStoreJSON(t *testing.T) {
	te := newMoveStoreEnv(t)
	out := te.ok("mv", "AA-2", "--store", "bravo-long", "--yes", "--json")
	te.golden("json", out)
	var got struct {
		From  string `json:"from"`
		To    string `json:"to"`
		Tasks []struct {
			OldID string `json:"old_id"`
			NewID string `json:"new_id"`
		} `json:"tasks"`
		Unlinked []struct {
			Field string `json:"field"`
		} `json:"unlinked"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.From != "alpha" || got.To != "bravo-long" || len(got.Tasks) != 1 || got.Tasks[0].OldID != "AA-2" || got.Tasks[0].NewID != "BB-3" || len(got.Unlinked) != 1 || got.Unlinked[0].Field != "blocked_by" {
		t.Errorf("json = %+v", got)
	}
}

func TestMvStoreConfirmOnATerminal(t *testing.T) {
	te := newMoveStoreEnv(t)
	te.terminal = true
	te.stdin = "n\n"
	before := len(te.log("alpha")) + len(te.log("bravo-long"))
	te.goldenRun("declined", 1, "mv", "AA-2", "--store", "bravo-long")
	if got := len(te.log("alpha")) + len(te.log("bravo-long")); got != before {
		t.Errorf("a declined move wrote %d commits", got-before)
	}

	te.stdin = "y\n"
	te.goldenRun("accepted", 0, "mv", "AA-2", "--store", "bravo-long")
}

func TestMvStoreWithoutAYesOffATerminal(t *testing.T) {
	te := newMoveStoreEnv(t)
	te.goldenRun("no-yes", 2, "mv", "AA-1", "--store", "bravo-long")
	te.goldenRun("same-store", 2, "mv", "AA-1", "--store", "alpha", "--yes")
	te.goldenRun("with-before", 2, "mv", "AA-1", "--store", "bravo-long", "--before", "AA-3")
	te.goldenRun("with-root", 2, "mv", "AA-1", "--store", "bravo-long", "--root")
	te.goldenRun("yes-alone", 2, "mv", "AA-1", "--yes")
	te.goldenRun("unknown-store", 1, "mv", "AA-1", "--store", "nowhere", "--yes")
}

func TestMvStoreScopedQuestion(t *testing.T) {
	te := newMoveStoreEnv(t)
	p := te.addJSON("--store", "alpha", "Scoped project", "--kind", "project")
	te.putStoreSchema("alpha", func(sc *schema.Schema) {
		sc.Statuses = append(sc.Statuses, schema.Status{Name: "review", Category: schema.CategoryDoing})
		u, _ := model.ParseULID(p.ULID)
		sc.Scoped = []schema.Scoped{{
			Where: "project:" + u.String(), Statuses: []string{"backlog", "todo", "review", "in_progress", "done", "canceled"},
			Workflow: &schema.Workflow{Transitions: schema.Lists{{Key: "backlog", Values: []string{"review"}}}},
		}}
	})
	// off a terminal the choice is a usage error
	te.goldenRun("no-scoped", 2, "mv", p.ID, "--store", "bravo-long", "--yes")

	te.terminal = true
	te.stdin = "x\n"
	te.goldenRun("bad-answer", 2, "mv", p.ID, "--store", "bravo-long")

	te.stdin = "c\ny\n"
	te.goldenRun("carried", 0, "mv", p.ID, "--store", "bravo-long")
}

func TestMvStoreScopedDrop(t *testing.T) {
	te := newMoveStoreEnv(t)
	p := te.addJSON("--store", "alpha", "Scoped project", "--kind", "project")
	te.putStoreSchema("alpha", func(sc *schema.Schema) {
		u, _ := model.ParseULID(p.ULID)
		sc.Scoped = []schema.Scoped{{Where: "project:" + u.String(), Statuses: []string{"backlog", "todo", "in_progress", "done", "canceled"}}}
	})
	te.goldenOK("dropped", "mv", p.ID, "--store", "bravo-long", "--scoped", "drop", "--yes", "--json")
}

func TestMvStoreAsksTheWorkspace(t *testing.T) {
	te := newMoveStoreEnv(t)
	te.ok("--store", "bravo-long", "workspace", "add", "second", "--prefix", "SEC")
	te.goldenRun("off-terminal", 2, "mv", "AA-1", "--store", "bravo-long", "--yes")
	te.terminal = true
	te.stdin = "2\ny\n"
	te.goldenRun("asked", 0, "mv", "AA-1", "--store", "bravo-long")
}

func TestMvStoreTellsWhoCannotSeeItAnymore(t *testing.T) {
	te := newMoveStoreEnv(t)
	for name, ids := range map[string][]string{
		"alpha":      {"github.com/alice", "github.com/bob", "github.com/carol"},
		"bravo-long": {"github.com/alice"},
	} {
		s, err := store.Open(te.storeDir(name))
		if err != nil {
			t.Fatal(err)
		}
		cs := []hhsync.Collaborator{}
		for _, id := range ids {
			cs = append(cs, hhsync.Collaborator{ID: id, Permission: "write"})
		}
		if err := hhsync.WriteMembers(s, hhsync.Members{Collaborators: cs}); err != nil {
			t.Fatal(err)
		}
	}
	te.terminal = true
	te.stdin = "y\n"
	te.goldenRun("hidden", 0, "mv", "AA-2", "--store", "bravo-long")
}
