package ghread_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/ghread"
)

// 07「GitHub を読む間隔」: 間隔、残り 10% 未満での 3 倍、読まない期限。
func TestDue(t *testing.T) {
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	every := 5 * time.Minute
	cases := []struct {
		name string
		r    ghread.Record
		want bool
	}{
		{"まだ読んでいない", ghread.Record{}, true},
		{"間隔が過ぎた", ghread.Record{Attempt: now.Add(-5 * time.Minute)}, true},
		{"間隔の途中", ghread.Record{Attempt: now.Add(-4 * time.Minute)}, false},
		{"失敗のあとも間隔まで待つ", ghread.Record{Attempt: now.Add(-4 * time.Minute), Failed: true}, false},
		{"残り 10% 未満なら 3 倍", ghread.Record{Attempt: now.Add(-14 * time.Minute), Limit: 5000, Remaining: 499, ResetAt: now.Add(time.Hour)}, false},
		{"3 倍が過ぎた", ghread.Record{Attempt: now.Add(-15 * time.Minute), Limit: 5000, Remaining: 499, ResetAt: now.Add(time.Hour)}, true},
		{"ちょうど 10% は 3 倍にしない", ghread.Record{Attempt: now.Add(-5 * time.Minute), Limit: 5000, Remaining: 500, ResetAt: now.Add(time.Hour)}, true},
		{"上限が戻ったら 3 倍をやめる", ghread.Record{Attempt: now.Add(-5 * time.Minute), Limit: 5000, Remaining: 1, ResetAt: now.Add(-time.Second)}, true},
		{"読まない期限の前", ghread.Record{Attempt: now.Add(-time.Hour), NotBefore: now.Add(time.Minute)}, false},
		{"読まない期限ちょうど", ghread.Record{Attempt: now.Add(-time.Hour), NotBefore: now}, true},
	}
	for _, c := range cases {
		if got := ghread.Due(now, c.r, every); got != c.want {
			t.Errorf("%s: Due = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLow(t *testing.T) {
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	if ghread.Low(now, ghread.Record{Remaining: 0, ResetAt: now.Add(time.Hour)}) {
		t.Error("上限が分からない(Limit 0)ときは Low にしない")
	}
	if !ghread.Low(now, ghread.Record{Limit: 5000, Remaining: 499, ResetAt: now.Add(time.Hour)}) {
		t.Error("10% 未満で戻る時刻が未来なら Low")
	}
}

func TestLimited(t *testing.T) {
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	if got := ghread.Limited(now, time.Time{}); !got.Equal(now.Add(time.Minute)) {
		t.Errorf("分からないとき = %v, want 1 分後", got)
	}
	reset := now.Add(23 * time.Minute)
	if got := ghread.Limited(now, reset); !got.Equal(reset) {
		t.Errorf("戻る時刻 = %v, want %v", got, reset)
	}
}
