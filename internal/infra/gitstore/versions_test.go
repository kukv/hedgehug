package gitstore

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
)

var _ port.StoreVersions = (*Stores)(nil)

func TestVersionsChangeWithEverySave(t *testing.T) {
	m := localMachine(t)
	v1, err := m.stores.Versions(t.Context())
	if err != nil || v1[m.id] == "" {
		t.Fatalf("Versions = %v, %v", v1, err)
	}
	rename(t, m, "新しい名前")
	v2, err := m.stores.Versions(t.Context())
	if err != nil || v2[m.id] == v1[m.id] || v2[m.id] != headCommit(t, m.dir).Hash.String() {
		t.Errorf("after a save: %v -> %v, %v", v1, v2, err)
	}
}
