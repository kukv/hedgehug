package gitstore

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

var _ port.Stores = (*Stores)(nil)

func TestContract(t *testing.T) {
	apptest.RunStoresContract(t, func(t *testing.T) (port.Stores, port.StoreID) {
		s, id, _ := newStore(t)
		return s, id
	})
}
