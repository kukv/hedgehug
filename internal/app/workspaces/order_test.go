package workspaces_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
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

// 別のマシンで同時に作って同期すると、rank が同じワークスペースができる。
// 同じ rank をまたいで動かしても panic せず、並びが表示の順(内部 ID の順)のとおりに動く。
// 空でなく厳密に増えている rank は書き換えない。
func TestWorkspaceOrderWithEqualRanks(t *testing.T) {
	cases := []struct {
		name    string
		ranks   []string
		move    int
		up      bool
		want    []int
		written []int // 並べ替えで書き足された(行が増えた)ワークスペース
	}{
		{"同じ rank の 2 件のあいだを上へ", []string{"W", "W", "X"}, 2, true, []int{0, 2, 1}, []int{1, 2}},
		{"同じ rank の 2 件のあいだを下へ", []string{"V", "W", "W"}, 0, false, []int{1, 0, 2}, []int{0, 2}},
		{"全部同じ rank で上へ", []string{"W", "W", "W"}, 1, true, []int{1, 0, 2}, []int{1, 2}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := setup(t)
			ids := createWithRanks(t, s, c.ranks)
			if err := s.MoveWorkspace(t.Context(), store, ids[c.move], c.up); err != nil {
				t.Fatal(err)
			}
			var got, want []workspace.ID
			list, _ := s.List(t.Context(), store)
			for _, w := range list {
				got = append(got, w.ID)
			}
			for _, i := range c.want {
				want = append(want, ids[i])
			}
			if !slices.Equal(got, want) {
				t.Errorf("order = %v, want %v", got, want)
			}
			assertWritten(t, s, ids, c.written)
		})
	}
}

// createWithRanks は、作成の行のあとに rank の set を 1 行持つワークスペースを ranks の数だけ作り、ID を返す。
func createWithRanks(t *testing.T, s workspaces.Service, ranks []string) []workspace.ID {
	t.Helper()
	var ids []workspace.ID
	for i, rk := range ranks {
		letter := string(rune('A' + i))
		id := workspace.ID("01EQ0000000000000000000" + letter + "01")
		d, _ := workspace.Create(workspace.Workspace{ID: id, Name: letter, Prefix: "E" + letter})
		op := port.Operation{Actor: change.User("taro"), Summary: "準備"}
		if err := s.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
			return tx.WriteWorkspaces().Append(id, d, change.NewSet("rank", nil, rk))
		}); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

// assertWritten は、ids のうち written の位置のものだけ、作成の準備の 2 行より多く行が書かれているかを確かめる。
func assertWritten(t *testing.T, s workspaces.Service, ids []workspace.ID, written []int) {
	t.Helper()
	for i, id := range ids {
		if _, log := stored(t, s, id); (len(log) > 2) != slices.Contains(written, i) {
			t.Errorf("%d 番目: log = %d 行", i, len(log))
		}
	}
}
