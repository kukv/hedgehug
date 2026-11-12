package workspaces_test

import (
	"context"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
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
	want := []string{
		"ワークスペース「Web」を作成",
		"ワークスペースの名前を「フロント」に変更",
		"接頭辞を HH から WEB に変更",
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
