package workspaces_test

import (
	"context"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// recorder は、Write に渡された Operation の Summary(アクティビティの文)を控える Stores。
type recorder struct {
	port.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

// アクティビティの文は仕様 03 の文字どおり。
func TestActivitySummaries(t *testing.T) {
	s := setup(t)
	rec := &recorder{Stores: s.Stores}
	s.Stores = rec
	w, err := s.Create(t.Context(), store, "Web", "HH")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Rename(t.Context(), store, w.ID, " フロント "); err != nil {
		t.Fatal(err)
	}
	if err := s.ChangePrefix(t.Context(), store, w.ID, "web"); err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	pr, err := s.AddProperty(ctx, store, w.ID, workspaces.NewProperty{Name: "顧客", Key: "customer", Type: workspace.TypeText})
	if err != nil {
		t.Fatal(err)
	}
	for _, err := range []error{
		s.RenameProperty(ctx, store, w.ID, pr.ID, "取引先"),
		s.SetPropertyKinds(ctx, store, w.ID, pr.ID, []string{"Task"}),
		s.SetPropertyHidden(ctx, store, w.ID, "points", true),
		s.SetPropertyHidden(ctx, store, w.ID, "points", false),
		s.MoveProperty(ctx, store, w.ID, pr.ID, true),
		s.DeleteProperty(ctx, store, w.ID, pr.ID),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	want := []string{
		"ワークスペース「Web」を作成",
		"ワークスペースの名前を「フロント」に変更",
		"接頭辞を HH から WEB に変更",
		"プロパティ「顧客」を追加",
		"プロパティ「顧客」を「取引先」に改名",
		"プロパティ「取引先」を使う種類を変更",
		"プロパティ「ポイント」を非表示に設定",
		"プロパティ「ポイント」を表示に設定",
		"プロパティの順番を変更",
		"プロパティ「取引先」を削除",
	}
	if len(rec.summaries) != len(want) {
		t.Fatalf("summaries = %q", rec.summaries)
	}
	for i := range want {
		if rec.summaries[i] != want[i] {
			t.Errorf("summary %d = %q, want %q", i, rec.summaries[i], want[i])
		}
	}
}

// 値の操作のアクティビティの文は仕様 03 の表の文字どおり。
func TestValueActivitySummaries(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, err := s.Create(ctx, store, "Web", "HH")
	if err != nil {
		t.Fatal(err)
	}
	pr, err := s.AddProperty(ctx, store, w.ID, workspaces.NewProperty{Name: "環境", Key: "env", Type: workspace.TypeSingle})
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s.Stores}
	s.Stores = rec
	st := settingsOf(t, s, w.ID)
	statuses, envs := workspace.ParseList("statuses"), workspace.ParseList(string(pr.ID))
	review, err := s.AddValue(ctx, store, w.ID, statuses, "Review")
	if err != nil {
		t.Fatal(err)
	}
	prod, err := s.AddValue(ctx, store, w.ID, envs, "本番")
	if err != nil {
		t.Fatal(err)
	}
	for _, err := range []error{
		s.RenameValue(ctx, store, w.ID, statuses, review.ID, "Reviewing"),
		s.SetValueColor(ctx, store, w.ID, statuses, review.ID, "purple"),
		s.SetStatusCategory(ctx, store, w.ID, review.ID, workspace.CategoryDoing),
		s.MoveValue(ctx, store, w.ID, statuses, review.ID, true),
		s.SetInitialStatus(ctx, store, w.ID, review.ID),
		s.SetInitialPriority(ctx, store, w.ID, st.Priorities[0].ID),
		s.SetCategoryDefault(ctx, store, w.ID, workspace.CategoryTodo, st.Statuses[3].ID),
		s.DeleteValue(ctx, store, w.ID, statuses, st.Statuses[0].ID, ""),
		s.DeleteValue(ctx, store, w.ID, envs, prod.ID, ""),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	want := []string{
		"ステータスに「Review」を追加",
		"「環境」の選択肢に「本番」を追加",
		"ステータス「Review」を「Reviewing」に改名",
		"ステータス「Reviewing」の色を変更",
		"ステータス「Reviewing」の分類を変更",
		"ステータスの順番を変更",
		"ステータス「Reviewing」を初期ステータスに設定",
		"優先度「" + st.Priorities[0].Name + "」を初期優先度に設定",
		"分類「未着手」の既定を「" + st.Statuses[3].Name + "」に変更",
		"ステータス「" + st.Statuses[0].Name + "」を削除",
		"「環境」の選択肢「本番」を削除",
	}
	if len(rec.summaries) != len(want) {
		t.Fatalf("summaries = %q", rec.summaries)
	}
	for i := range want {
		if rec.summaries[i] != want[i] {
			t.Errorf("summary %d = %q, want %q", i, rec.summaries[i], want[i])
		}
	}
}
