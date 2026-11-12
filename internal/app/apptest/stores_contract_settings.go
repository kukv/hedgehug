package apptest

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// contractSettings は、設定を含めて作ったワークスペースの設定が、そのまま読めることを確かめる。
func contractSettings(t *testing.T, s port.Stores, id port.StoreID) {
	n := 0
	settings := workspace.DefaultSettings(func() change.ID {
		n++
		return change.ID("01CONTRACTVAL" + string(rune('A'+n)) + "000000000000")
	})
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		d, err := workspace.CreateWithSettings(cWorkspace, settings)
		if err != nil {
			return err
		}
		return tx.WriteWorkspaces().Append(cWorkspace.ID, d)
	})
	mustRead(t, s, id, func(r port.Reader) error {
		got, err := r.Workspaces().Settings(cWorkspace.ID)
		if err != nil || !reflect.DeepEqual(got, settings) {
			t.Errorf("Settings = %+v, %v", got, err)
		}
		if _, err := r.Workspaces().Settings("01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("unknown workspace: %v", err)
		}
		return nil
	})
}
