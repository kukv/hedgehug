package port

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
)

func TestCheckOpDrafts(t *testing.T) {
	sync := change.Actor{Kind: change.ActorSync}
	draft := change.NewDeleteEntity(false)
	cases := []struct {
		name   string
		actor  change.Actor
		drafts []change.Draft
		bad    bool
	}{
		{"person with an op row", change.User("taro"), []change.Draft{draft.InOp("01OP")}, true},
		{"github with an op row", change.Actor{Kind: change.ActorGitHub}, []change.Draft{draft.InOp("01OP")}, true},
		{"sync with a malformed op", sync, []change.Draft{draft.InOp("bad id!")}, true},
		{"sync with a valid op", sync, []change.Draft{draft.InOp("01OP")}, false},
		{"person without an op row", change.User("taro"), []change.Draft{draft}, false},
	}
	for _, c := range cases {
		err := CheckOpDrafts(c.actor, c.drafts)
		if c.bad != errors.Is(err, ErrInvalidOperation) || (!c.bad && err != nil) {
			t.Errorf("%s: err = %v, want invalid = %v", c.name, err, c.bad)
		}
	}
}
