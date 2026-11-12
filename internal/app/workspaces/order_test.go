package workspaces_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestWorkspaceOrder(t *testing.T) {
	s := setup(t)
	rec := &recorder{Stores: s.Stores}
	s.Stores = rec
	ctx := t.Context()
	a, _ := s.Create(ctx, store, "Web", "HH")
	b, _ := s.Create(ctx, store, "API", "API")
	c, _ := s.Create(ctx, store, "Docs", "DOC")
	if err := s.MoveWorkspace(ctx, store, c.ID, true); err != nil {
		t.Fatal(err)
	}
	list, err := s.List(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, w := range list {
		names = append(names, w.Name)
	}
	if !slices.Equal(names, []string{"Web", "Docs", "API"}) {
		t.Errorf("order = %v", names)
	}
	if last := rec.summaries[len(rec.summaries)-1]; last != "Docs の並びを変更" {
		t.Errorf("summary = %q", last)
	}
	// 位置が変わるのは動かした 1 件だけ(04)。B のログは作成の 1 行のまま(rank は作成の行に入る)。
	if _, log := stored(t, s, b.ID); len(log) != 1 {
		t.Errorf("neighbor was written: %+v", log)
	}
	if _, log := stored(t, s, c.ID); len(log) != 2 {
		t.Errorf("moved log = %+v", log)
	}
	// 先頭で上へ・末尾で下へは、何も保存しない。
	_ = s.MoveWorkspace(ctx, store, a.ID, true)
	_ = s.MoveWorkspace(ctx, store, b.ID, false)
	if _, log := stored(t, s, a.ID); len(log) != 1 {
		t.Errorf("log = %+v", log)
	}
	if _, log := stored(t, s, b.ID); len(log) != 1 {
		t.Errorf("log = %+v", log)
	}
}

// rank のないワークスペース(B2 より前に作ったもの、apptest.Sample)は先頭に内部 ID の順で並ぶ。
// 並べ替えると、同じ保存で、rank のないものに表示の順で rank を振ってから動かす(B2-7)。
func TestWorkspaceOrderWithoutRanks(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	// workspace.Create の行だけを書き、rank を持たない 2 つを作る。
	for _, w := range []workspace.Workspace{
		{ID: "01OLD0000000000000000000A1", Name: "旧A", Prefix: "OA"},
		{ID: "01OLD0000000000000000000B1", Name: "旧B", Prefix: "OB"},
	} {
		d, _ := workspace.Create(w)
		if err := s.Stores.Write(ctx, store, port.Operation{Actor: change.User("taro"), Summary: "準備"}, func(tx port.Tx) error {
			return tx.WriteWorkspaces().Append(w.ID, d)
		}); err != nil {
			t.Fatal(err)
		}
	}
	n, _ := s.Create(ctx, store, "新", "NEW")
	list, _ := s.List(ctx, store)
	if len(list) != 3 || list[2].ID != n.ID {
		t.Fatalf("before move = %+v", list)
	}
	if err := s.MoveWorkspace(ctx, store, "01OLD0000000000000000000B1", true); err != nil {
		t.Fatal(err)
	}
	list, _ = s.List(ctx, store)
	var ids []workspace.ID
	for _, w := range list {
		ids = append(ids, w.ID)
	}
	if !slices.Equal(ids, []workspace.ID{"01OLD0000000000000000000B1", "01OLD0000000000000000000A1", n.ID}) {
		t.Errorf("order = %v", ids)
	}
	// 新しいものの rank は書き換えない。
	if _, log := stored(t, s, n.ID); len(log) != 1 {
		t.Errorf("ranked workspace was written: %+v", log)
	}
}
