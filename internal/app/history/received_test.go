package history_test

import (
	"context"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	apphistory "github.com/kukv/hedgehug/internal/app/history"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// receivedOps は apphistory.ReceivedOps の偽物。ops にある op を、同期で受け取った保存のものとして返す。
type receivedOps struct{ ops []change.ID }

func (f receivedOps) Received(_ context.Context, _ port.StoreID, ops []change.ID) (map[change.ID]bool, error) {
	out := map[change.ID]bool{}
	for _, op := range ops {
		if slices.Contains(f.ops, op) {
			out[op] = true
		}
	}
	return out, nil
}

// lastOp は、ref のログの最後の行の op。
func lastOp(t *testing.T, s port.Stores, ref change.Ref) change.ID {
	t.Helper()
	var op change.ID
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(ref)
		if len(log) > 0 {
			op = log[len(log)-1].Op
		}
		return err
	})
	if err != nil || op == "" {
		t.Fatalf("op = %q, %v", op, err)
	}
	return op
}

func received(rows []apphistory.Row) []bool {
	out := make([]bool, len(rows))
	for i, r := range rows {
		out[i] = r.Received
	}
	return out
}

// 同期で受け取った保存の op の行だけ Received(決定 C4d-16)。Received が nil なら、どの行にも添えない。
func TestItemHistoryReceived(t *testing.T) {
	svc, s := setup(t)
	write(t, s, "hanako", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "ログイン"))
	})
	op := lastOp(t, s, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
	write(t, s, "taro", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン", "ログイン画面"))
	})
	rows, err := svc.Item(t.Context(), store, apptest.SampleTask)
	if err != nil {
		t.Fatal(err)
	}
	if got := received(rows); !slices.Equal(got, []bool{false, false, false}) {
		t.Errorf("nil: received = %v", got)
	}
	svc.Received = receivedOps{ops: []change.ID{op}}
	if rows, err = svc.Item(t.Context(), store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if got := received(rows); !slices.Equal(got, []bool{false, true, false}) {
		t.Errorf("received = %v (%q)", got, texts(rows))
	}
	if rows[1].Op != op {
		t.Errorf("op = %q, want %q", rows[1].Op, op)
	}
}

// 実行者「同期」の行(同期の修復)には、op が受け取った保存のものでも添えない(決定 C4d-16)。
func TestItemHistoryReceivedSkipsSync(t *testing.T) {
	svc, s := setup(t)
	op := port.Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: "同期の修復"}
	if err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "ログイン"))
	}); err != nil {
		t.Fatal(err)
	}
	svc.Received = receivedOps{ops: []change.ID{lastOp(t, s, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})}}
	rows, err := svc.Item(t.Context(), store, apptest.SampleTask)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Who != "同期" || rows[0].Received {
		t.Errorf("rows = %+v", rows)
	}
}

// プロジェクトの履歴も、同期で受け取った保存の op の行に Received を添える。1 回の保存の行はどれも添える。
func TestProjectHistoryReceived(t *testing.T) {
	svc, s := setup(t)
	write(t, s, "hanako", func(tx port.Tx) error {
		return tx.WriteProjects().Append(apptest.SampleProject,
			change.NewSet("title", "ログイン", "認証"),
			change.NewSet("owner", "taro", "hanako"),
		)
	})
	svc.Received = receivedOps{ops: []change.ID{lastOp(t, s, change.Ref{Kind: change.KindProject, ID: apptest.SampleProject})}}
	rows, err := svc.Project(t.Context(), store, apptest.SampleProject)
	if err != nil {
		t.Fatal(err)
	}
	if got := received(rows); !slices.Equal(got, []bool{true, true, false}) {
		t.Errorf("received = %v (%q)", got, texts(rows))
	}
}
