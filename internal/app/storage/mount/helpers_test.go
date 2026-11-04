package mount

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/storage/config"
)

// openSet opens the stores of es.
func openSet(t *testing.T, es ...config.StoreEntry) *Set {
	t.Helper()
	set := Open(&config.Config{Stores: es}, Options{})
	return set
}
