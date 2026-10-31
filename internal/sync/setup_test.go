package sync

import (
	"testing"

	"github.com/kukv/hedgehug/internal/store"
)

func TestNeedsSetup(t *testing.T) {
	c := newCluster(t, 1)
	s := c.stores[0]
	if need, err := NeedsSetup(s); err != nil || need {
		t.Fatalf("NeedsSetup after init = %v, %v", need, err)
	}
	if _, err := s.Update(store.Op{Name: "edit", Author: c.opts[0].Author}, func(tx *store.Tx) error {
		return tx.PutFile(".gitattributes", []byte("*.yaml merge=hedgehug\n"))
	}); err != nil {
		t.Fatal(err)
	}
	if need, err := NeedsSetup(s); err != nil || !need {
		t.Errorf("NeedsSetup with lines missing = %v, %v", need, err)
	}
	c.sync(0)
	if need, err := NeedsSetup(s); err != nil || need {
		t.Errorf("NeedsSetup after the sync = %v, %v", need, err)
	}
}
