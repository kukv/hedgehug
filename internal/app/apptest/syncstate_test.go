package apptest_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

func TestSyncStateContract(t *testing.T) {
	apptest.RunSyncStateContract(t, func(*testing.T) port.SyncState { return apptest.NewSyncState() })
}

func TestJudgedSavesContract(t *testing.T) {
	s := apptest.NewJudgedSaves()
	apptest.RunJudgedSavesContract(t, func() port.JudgedSaves { return s })
}
