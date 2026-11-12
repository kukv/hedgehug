package apptest_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

func TestSyncStateContract(t *testing.T) {
	apptest.RunSyncStateContract(t, func(*testing.T) port.SyncState { return apptest.NewSyncState() })
}
