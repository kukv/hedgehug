package search

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	wsA workspace.ID = "01TESTWS0000000000000000A"
	wsB workspace.ID = "01TESTWS0000000000000000B"
	wsC workspace.ID = "01TESTWS0000000000000000C"
)

// heldElsewhere は、S1 のワークスペース wsA 以外に、キー customer を持つものがあるか。
func heldElsewhere(t *testing.T, f fixture) bool {
	t.Helper()
	got, err := f.svc.KeyHeldElsewhere(t.Context(), store1, wsA, "customer")
	must(t, err)
	return got
}

func TestKeyHeldElsewhere(t *testing.T) {
	f := newFixture(t)
	addWorkspace(t, f, store1, wsA, "a", withProp("customer", "顧客", workspace.TypeText))
	if heldElsewhere(t, f) {
		t.Error("自分のワークスペースだけなら false のはず")
	}
	addWorkspace(t, f, store2, wsB, "a", withProp("Customer", "顧客", workspace.TypeText))
	if !heldElsewhere(t, f) {
		t.Error("別の Store のワークスペースにもあるので true のはず")
	}
}

func TestKeyHeldElsewhereHiddenStore(t *testing.T) {
	f := newFixture(t)
	addWorkspace(t, f, store1, wsA, "a", withProp("customer", "顧客", workspace.TypeText))
	addWorkspace(t, f, store3, wsC, "a", withProp("customer", "顧客", workspace.TypeText))
	if !heldElsewhere(t, f) {
		t.Error("非表示の Store のワークスペースにあるので true のはず")
	}
}

func TestKeyHeldElsewhereIgnoresDeleted(t *testing.T) {
	f := newFixture(t)
	addWorkspace(t, f, store1, wsA, "a", withProp("customer", "顧客", workspace.TypeText))
	addWorkspace(t, f, store2, wsB, "a", withProp("customer", "顧客", workspace.TypeText))
	op := port.Operation{Actor: change.User("taro"), Summary: "テスト"}
	must(t, f.stores.Write(t.Context(), store2, op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(wsB, change.NewDeleteEntity(true))
	}))
	if heldElsewhere(t, f) {
		t.Error("削除済みのワークスペースだけなら false のはず")
	}
}

func TestKeyHeldElsewhereIgnoresBuiltin(t *testing.T) {
	f := newFixture(t)
	addWorkspace(t, f, store1, wsA, "a", nil)
	addWorkspace(t, f, store2, wsB, "a", nil)
	got, err := f.svc.KeyHeldElsewhere(t.Context(), store1, wsA, "points")
	must(t, err)
	if got {
		t.Error("組み込みのプロパティは追加プロパティに数えない")
	}
}
