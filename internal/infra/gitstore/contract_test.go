package gitstore

import (
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

var _ port.Stores = (*Stores)(nil)

func TestContract(t *testing.T) {
	apptest.RunStoresContract(t, func(t *testing.T) (port.Stores, port.StoreID, port.StoreID) {
		s, id, _ := newStore(t)
		dir := filepath.Join(t.TempDir(), "other")
		if err := Create(dir, "01STORE0000000000000000002", t0); err != nil {
			t.Fatal(err)
		}
		other, err := s.Attach(dir)
		if err != nil {
			t.Fatal(err)
		}
		return s, id, other
	})
}
