package watch_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/automations"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/app/usecase/watch"
)

func TestPollAutomations(t *testing.T) {
	t.Parallel()
	f := pollFixture(t)
	f.settle()
	if err := automations.Add(f.session(), automations.AddInput{ID: "a", On: "pr.opened", Category: "done"}); err != nil {
		t.Fatal(err)
	}
	e := waitEvent[automationsChanged](t, f.ch, nil)
	if e.Store != "me" {
		t.Errorf("event = %+v", e)
	}
}

func TestPollTellsWhatARuleWrote(t *testing.T) {
	t.Parallel()
	f := pollFixture(t)
	f.settle() // the first look sets the HEADs; it sends nothing
	res, err := tasks.Add(f.session(), tasks.AddInput{Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	if e := waitEvent[watch.Change](t, f.ch, func(e watch.Change) bool { return e.Commit == res.Commit }); e.Automated {
		t.Errorf("a person's commit = %+v", e)
	}
	f.ts.MustEdit("me", "gh-fetch", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		return tx.PutFile("note", []byte("x"))
	})
	e := waitEvent[watch.Change](t, f.ch, func(e watch.Change) bool { return e.Op == "gh-fetch" })
	if !e.Automated {
		t.Errorf("a rule's commit = %+v", e)
	}
}
