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
