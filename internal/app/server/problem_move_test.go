package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
)

// The confirmation of a move reaches a client as a problem that names the
// tasks and the cuts, and nothing of their content.
func TestProblemOfAMoveConfirmationIsSmall(t *testing.T) {
	t.Parallel()
	ts := twoStores(t)
	ss := ts.session()
	ss.StoreName = "me"
	r, err := tasks.Add(ss, tasks.AddInput{Title: "moves"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := comments.CommentAdd(ss, r.ULID.String(), "a comment that must not travel"); err != nil {
		t.Fatal(err)
	}
	_, err = stores.MoveStore(ts.session(), stores.MoveStoreInput{Arg: r.ULID.String(), To: "team"})
	p := problemOf(err)
	if p.Status != http.StatusBadRequest || p.Type != "confirm.required" {
		t.Fatalf("problem = %d %q", p.Status, p.Type)
	}
	if _, ok := p.Args["plan"].(stores.MoveView); !ok {
		t.Fatalf("plan = %T, want a MoveView", p.Args["plan"])
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "must not travel") || len(b) > 2000 {
		t.Errorf("the problem is %d bytes and holds content: %s", len(b), b)
	}
}
