package statedb

import (
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func TestChecksContract(t *testing.T) {
	apptest.RunChecksContract(t, func(t *testing.T) port.Checks { return openTemp(t).Checks() })
}

func TestChecksSurviveReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Checks().Save(t.Context(), "S1", "pr:org/web#1", itemlink.Success); err != nil {
		t.Fatal(err)
	}
	_ = d.Close()
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	if got, _ := d.Checks().Read(t.Context(), "S1"); got["pr:org/web#1"] != itemlink.Success {
		t.Errorf("after reopen: %v", got)
	}
}
