package workspaces_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

var (
	statuses   = workspace.List{Kind: workspace.ListStatuses}
	priorities = workspace.List{Kind: workspace.ListPriorities}
	systems    = workspace.List{Kind: workspace.ListSystems}
)

// logOf は、ワークスペース ws のログ。
func logOf(t *testing.T, s workspaces.Service, ws workspace.ID) []change.Record {
	t.Helper()
	var log []change.Record
	if err := s.Stores.Read(t.Context(), store, func(r port.Reader) error {
		var err error
		log, err = r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return log
}

// restores は、ワークスペース ws のログの restore の行の数。
func restores(t *testing.T, s workspaces.Service, ws workspace.ID) int {
	t.Helper()
	n := 0
	for _, r := range logOf(t, s, ws) {
		if r.T == change.Restore {
			n++
		}
	}
	return n
}

// valueIn は、設定の一覧 l の値 id。
func valueIn(t *testing.T, s workspaces.Service, ws workspace.ID, l workspace.List, id change.ID) (workspace.Value, bool) {
	t.Helper()
	vs, _ := settingsOf(t, s, ws).Values(l)
	i := slices.IndexFunc(vs, func(v workspace.Value) bool { return v.ID == id })
	if i < 0 {
		return workspace.Value{}, false
	}
	return vs[i], true
}

// addValue は、一覧 l に名前 name の値を足す。
func addValue(t *testing.T, s workspaces.Service, ws workspace.ID, l workspace.List, name string) workspace.Value {
	t.Helper()
	v, err := s.AddValue(t.Context(), store, ws, l, name)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// deleteValue は、別のタブの削除に当たる、値 id の削除(置き換え先は既定)。
func deleteValue(t *testing.T, s workspaces.Service, ws workspace.ID, l workspace.List, id change.ID) {
	t.Helper()
	if err := s.DeleteValue(t.Context(), store, ws, l, id, ""); err != nil {
		t.Fatal(err)
	}
}

// 別のタブで削除された値に、このタブから変更を保存すると、削除を取り消して変更を残す(決定 R2)。
func TestChangeDeletedValueRevives(t *testing.T) {
	type valueOp func(ctx context.Context, s workspaces.Service, ws workspace.ID, l workspace.List, id change.ID) error
	color := func(c workspace.Color) valueOp {
		return func(ctx context.Context, s workspaces.Service, ws workspace.ID, l workspace.List, id change.ID) error {
			return s.SetValueColor(ctx, store, ws, l, id, c)
		}
	}
	rename := func(ctx context.Context, s workspaces.Service, ws workspace.ID, l workspace.List, id change.ID) error {
		return s.RenameValue(ctx, store, ws, l, id, "Reviewing")
	}
	tests := []struct {
		name     string
		list     func(s workspaces.Service, ws workspace.ID) workspace.List
		value    string // 足して削除する値の名前
		change   valueOp
		want     func(v workspace.Value) bool
		wantName string // 設定に戻った値の名前
	}{
		{"status name", fixed(statuses), "Review", rename, nil, "Reviewing"},
		{"status color", fixed(statuses), "Review", color("red"), func(v workspace.Value) bool { return v.Color == "red" }, "Review"},
		{
			"status move", fixed(statuses), "Review",
			func(ctx context.Context, s workspaces.Service, ws workspace.ID, l workspace.List, id change.ID) error {
				return s.MoveValue(ctx, store, ws, l, id, true)
			}, nil, "Review",
		},
		{
			"status category", fixed(statuses), "Review",
			func(ctx context.Context, s workspaces.Service, ws workspace.ID, _ workspace.List, id change.ID) error {
				return s.SetStatusCategory(ctx, store, ws, id, workspace.CategoryDoing)
			}, func(v workspace.Value) bool { return v.Category == workspace.CategoryDoing }, "Review",
		},
		{"priority color", fixed(priorities), "Urgent", color("red"), func(v workspace.Value) bool { return v.Color == "red" }, "Urgent"},
		{"system name", fixed(systems), "Review", rename, nil, "Reviewing"},
		{"choice color", choiceList, "A", color("red"), func(v workspace.Value) bool { return v.Color == "red" }, "A"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setup(t)
			w, _ := s.Create(t.Context(), store, "Web", "HH")
			l := tt.list(s, w.ID)
			v := addValue(t, s, w.ID, l, tt.value)
			deleteValue(t, s, w.ID, l, v.ID)
			ctx := port.WithRevived(t.Context())
			if err := tt.change(ctx, s, w.ID, l, v.ID); err != nil {
				t.Fatal(err)
			}
			got, ok := valueIn(t, s, w.ID, l, v.ID)
			if !ok || got.Name != tt.wantName || (tt.want != nil && !tt.want(got)) {
				t.Errorf("value = %+v, %v", got, ok)
			}
			if want := []port.Revival{{Kind: port.RevivedName, Text: tt.value}}; !slices.Equal(port.Revived(ctx), want) {
				t.Errorf("Revived = %v, want %v", port.Revived(ctx), want)
			}
		})
	}
}

func fixed(l workspace.List) func(workspaces.Service, workspace.ID) workspace.List {
	return func(workspaces.Service, workspace.ID) workspace.List { return l }
}

// choiceList は、ws に単一選択のプロパティを足し、その選択肢の一覧を返す。
func choiceList(s workspaces.Service, ws workspace.ID) workspace.List {
	p, err := s.AddProperty(context.Background(), store, ws, workspaces.NewProperty{Name: "顧客", Key: "customer", Type: workspace.TypeSingle})
	if err != nil {
		panic(err)
	}
	return workspace.List{Kind: workspace.ListChoices, Property: p.ID}
}

// 値の削除で置き換えたアイテムは、値の取り消しで戻る。削除のあとに人が変えたアイテムは戻らない(Review Focus 1)。
func TestColorDeletedStatusRestoresItems(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	todo, done := st.Statuses[0].ID, st.Statuses[2].ID
	review := addValue(t, s, w.ID, statuses, "Review")
	a := putItem(t, s, w.ID, change.NewSet("status", nil, review.ID))
	b := putItem(t, s, w.ID, change.NewSet("status", nil, review.ID))
	if err := s.DeleteValue(ctx, store, w.ID, statuses, review.ID, todo); err != nil {
		t.Fatal(err)
	}
	err := s.Stores.Write(ctx, store, port.Operation{Actor: change.User("taro"), Summary: "ステータスを変更"}, func(tx port.Tx) error {
		return tx.WriteItems().Append(a, change.NewSet("status", todo, done))
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s.Stores}
	s.Stores = rec
	if err := s.SetValueColor(port.WithRevived(ctx), store, w.ID, statuses, review.ID, "red"); err != nil {
		t.Fatal(err)
	}
	if got := itemOf(t, s, b).Status; got != review.ID {
		t.Errorf("B status = %q, want Review", got)
	}
	if got := itemOf(t, s, a).Status; got != done {
		t.Errorf("A status = %q, want Done", got)
	}
	if want := []string{"ステータス「Review」の色を変更"}; !slices.Equal(rec.summaries, want) {
		t.Errorf("summaries = %q, want %q", rec.summaries, want)
	}
}

// deletedMemo は、テキストのプロパティ「メモ」と、その値の入ったアイテムを作り、プロパティを削除する。
func deletedMemo(t *testing.T) (workspaces.Service, workspace.Workspace, workspace.Property, item.ID) {
	t.Helper()
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	memo, err := s.AddProperty(t.Context(), store, w.ID, workspaces.NewProperty{Name: "メモ", Key: "memo", Type: workspace.TypeText})
	if err != nil {
		t.Fatal(err)
	}
	id := putItem(t, s, w.ID, change.NewSet(item.FieldKey(memo.ID), nil, "下書き"))
	if err := s.DeleteProperty(t.Context(), store, w.ID, memo.ID); err != nil {
		t.Fatal(err)
	}
	return s, w, memo, id
}

// 別のタブで削除された追加プロパティに変更を保存すると、プロパティと、削除で外したアイテムの値が戻る。
func TestChangeDeletedPropertyRevives(t *testing.T) {
	tests := []struct {
		name   string
		change func(ctx context.Context, s workspaces.Service, ws workspace.ID, id change.ID) error
		want   func(p workspace.Property) bool
	}{
		{"name", func(ctx context.Context, s workspaces.Service, ws workspace.ID, id change.ID) error {
			return s.RenameProperty(ctx, store, ws, id, "備考")
		}, func(p workspace.Property) bool { return p.Name == "備考" }},
		{"kinds", func(ctx context.Context, s workspaces.Service, ws workspace.ID, id change.ID) error {
			return s.SetPropertyKinds(ctx, store, ws, id, []string{"Task"})
		}, func(p workspace.Property) bool { return slices.Equal(p.Kinds, []string{"Task"}) }},
		{"move", func(ctx context.Context, s workspaces.Service, ws workspace.ID, id change.ID) error {
			return s.MoveProperty(ctx, store, ws, id, true)
		}, func(p workspace.Property) bool { return p.Name == "メモ" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, w, memo, id := deletedMemo(t)
			ctx := port.WithRevived(t.Context())
			if err := tt.change(ctx, s, w.ID, memo.ID); err != nil {
				t.Fatal(err)
			}
			if p, ok := settingsOf(t, s, w.ID).Property(memo.ID); !ok || !tt.want(p) {
				t.Errorf("property = %+v, %v", p, ok)
			}
			if got := itemOf(t, s, id).Fields; len(got) != 1 {
				t.Errorf("item fields = %v, want the value back", got)
			}
			if want := []port.Revival{{Kind: port.RevivedName, Text: "メモ"}}; !slices.Equal(port.Revived(ctx), want) {
				t.Errorf("Revived = %v, want %v", port.Revived(ctx), want)
			}
		})
	}
}

// 削除済みの値への、何も変わらない保存は、取り消さず ErrNotFound(決定 R3)。
func TestChangeDeletedValueUnchangedIsNotFound(t *testing.T) {
	tests := []struct {
		name   string
		change func(ctx context.Context, s workspaces.Service, ws workspace.ID, v workspace.Value) error
	}{
		{"same color", func(ctx context.Context, s workspaces.Service, ws workspace.ID, v workspace.Value) error {
			return s.SetValueColor(ctx, store, ws, statuses, v.ID, v.Color)
		}},
		{"same category", func(ctx context.Context, s workspaces.Service, ws workspace.ID, v workspace.Value) error {
			return s.SetStatusCategory(ctx, store, ws, v.ID, v.Category)
		}},
		{"move at the edge", func(ctx context.Context, s workspaces.Service, ws workspace.ID, v workspace.Value) error {
			return s.MoveValue(ctx, store, ws, statuses, v.ID, false)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setup(t)
			w, _ := s.Create(t.Context(), store, "Web", "HH")
			v := addValue(t, s, w.ID, statuses, "Review") // 末尾
			deleteValue(t, s, w.ID, statuses, v.ID)
			ctx := port.WithRevived(t.Context())
			if err := tt.change(ctx, s, w.ID, v); !errors.Is(err, port.ErrNotFound) {
				t.Errorf("err = %v, want port.ErrNotFound", err)
			}
			if n := restores(t, s, w.ID); n != 0 {
				t.Errorf("restore rows = %d, want 0", n)
			}
		})
	}
}

// 取り消した値・プロパティの名前が、削除されていないものと重なるなら、保存ごと断る。名前を変える保存は、
// 変更後の名前で判定する(決定 R4)。
func TestChangeDeletedValueNameTaken(t *testing.T) {
	s := setup(t)
	ctx := port.WithRevived(t.Context())
	w, _ := s.Create(ctx, store, "Web", "HH")
	old := addValue(t, s, w.ID, statuses, "Review")
	deleteValue(t, s, w.ID, statuses, old.ID)
	addValue(t, s, w.ID, statuses, "Review")
	before := len(logOf(t, s, w.ID))
	rejected(t, s.SetValueColor(ctx, store, w.ID, statuses, old.ID, "red"), "同じ名前の値があります")
	if after := len(logOf(t, s, w.ID)); after != before {
		t.Errorf("log rows %d -> %d; nothing must be saved", before, after)
	}
	if err := s.RenameValue(ctx, store, w.ID, statuses, old.ID, "Review 2"); err != nil {
		t.Fatal(err)
	}
	if v, ok := valueIn(t, s, w.ID, statuses, old.ID); !ok || v.Name != "Review 2" {
		t.Errorf("value = %+v, %v", v, ok)
	}

	memo, _ := s.AddProperty(ctx, store, w.ID, workspaces.NewProperty{Name: "メモ", Key: "memo", Type: workspace.TypeText})
	if err := s.DeleteProperty(ctx, store, w.ID, memo.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddProperty(ctx, store, w.ID, workspaces.NewProperty{Name: "メモ", Key: "memo2", Type: workspace.TypeText}); err != nil {
		t.Fatal(err)
	}
	before = len(logOf(t, s, w.ID))
	rejected(t, s.SetPropertyKinds(ctx, store, w.ID, memo.ID, []string{"Task"}), "同じ名前のプロパティがあります")
	if after := len(logOf(t, s, w.ID)); after != before {
		t.Errorf("log rows %d -> %d; nothing must be saved", before, after)
	}
	if err := s.RenameProperty(ctx, store, w.ID, memo.ID, "メモ 2"); err != nil {
		t.Fatal(err)
	}
	if p, ok := settingsOf(t, s, w.ID).Property(memo.ID); !ok || p.Name != "メモ 2" {
		t.Errorf("property = %+v, %v", p, ok)
	}
}

// 初期ステータス・分類の既定・初期優先度にする保存は、削除済みの値を取り消さず ErrNotFound(Review Focus 2)。
func TestMarksOnDeletedStatusAreNotFound(t *testing.T) {
	tests := []struct {
		name   string
		list   workspace.List
		change func(ctx context.Context, s workspaces.Service, ws workspace.ID, id change.ID) error
	}{
		{"initial status", statuses, func(ctx context.Context, s workspaces.Service, ws workspace.ID, id change.ID) error {
			return s.SetInitialStatus(ctx, store, ws, id)
		}},
		{"category default", statuses, func(ctx context.Context, s workspaces.Service, ws workspace.ID, id change.ID) error {
			return s.SetCategoryDefault(ctx, store, ws, workspace.CategoryTodo, id)
		}},
		{"initial priority", priorities, func(ctx context.Context, s workspaces.Service, ws workspace.ID, id change.ID) error {
			return s.SetInitialPriority(ctx, store, ws, id)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setup(t)
			w, _ := s.Create(t.Context(), store, "Web", "HH")
			v := addValue(t, s, w.ID, tt.list, "Review")
			deleteValue(t, s, w.ID, tt.list, v.ID)
			if err := tt.change(port.WithRevived(t.Context()), s, w.ID, v.ID); !errors.Is(err, port.ErrNotFound) {
				t.Errorf("err = %v, want port.ErrNotFound", err)
			}
			if n := restores(t, s, w.ID); n != 0 {
				t.Errorf("restore rows = %d, want 0", n)
			}
		})
	}
}

// 削除済みの単一選択のプロパティの選択肢への変更と追加は ErrNotFound で、プロパティは削除済みのまま。
func TestChoiceOfDeletedPropertyIsNotFound(t *testing.T) {
	s := setup(t)
	ctx := port.WithRevived(t.Context())
	w, _ := s.Create(ctx, store, "Web", "HH")
	l := choiceList(s, w.ID)
	a := addValue(t, s, w.ID, l, "A")
	if err := s.DeleteProperty(ctx, store, w.ID, l.Property); err != nil {
		t.Fatal(err)
	}
	if err := s.SetValueColor(ctx, store, w.ID, l, a.ID, "red"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("color: err = %v, want port.ErrNotFound", err)
	}
	if _, err := s.AddValue(ctx, store, w.ID, l, "B"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("add: err = %v, want port.ErrNotFound", err)
	}
	if _, ok := settingsOf(t, s, w.ID).Property(l.Property); ok {
		t.Error("the property must stay deleted")
	}
	if n := restores(t, s, w.ID); n != 0 {
		t.Errorf("restore rows = %d, want 0", n)
	}
}
