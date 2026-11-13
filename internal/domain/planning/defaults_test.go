package planning_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

func TestSprintDefaults(t *testing.T) {
	const today = "2026-11-14"
	s7 := planning.Sprint{ID: "S7", Name: "Sprint 7", State: planning.Closed, Start: "2026-11-02", End: "2026-11-15"}
	s8 := planning.Sprint{ID: "S8", Name: "Sprint 8", State: planning.Planned}
	s9 := planning.Sprint{ID: "S9", Name: "Sprint 9", State: planning.Planned}
	tests := []struct {
		label            string
		live             []planning.Sprint
		name, start, end string
	}{
		{"なし", nil, "Sprint 1", "2026-11-14", "2026-11-27"},
		{"基準の数字に 1", []planning.Sprint{s7}, "Sprint 8", "2026-11-16", "2026-11-29"},
		{"重なれば進める", []planning.Sprint{s7, s8}, "Sprint 9", "2026-11-16", "2026-11-29"},
		{"さらに重なる", []planning.Sprint{s7, s8, s9}, "Sprint 10", "2026-11-16", "2026-11-29"},
		{"数字で終わらない基準は数+1", []planning.Sprint{
			{ID: "a", Name: "リリース前", End: "2026-11-01"},
			{ID: "b", Name: "x", End: "2026-11-02"},
			{ID: "c", Name: "リリース前", End: "2026-11-20"},
		}, "Sprint 4", "2026-11-21", "2026-12-04"},
		{"基準の名前の末尾の空白は除く", []planning.Sprint{{ID: "a", Name: "Sprint 7 ", End: "2026-11-15"}}, "Sprint 8", "2026-11-16", "2026-11-29"},
		{"桁を保つ(先頭 0)", []planning.Sprint{{ID: "a", Name: "Sprint 07", End: "2026-11-15"}}, "Sprint 08", "2026-11-16", "2026-11-29"},
		{"桁が増える", []planning.Sprint{{ID: "a", Name: "S-099", End: "2026-11-15"}}, "S-100", "2026-11-16", "2026-11-29"},
		{"9 の繰り上がり", []planning.Sprint{{ID: "a", Name: "Sprint 9", End: "2026-11-15"}}, "Sprint 10", "2026-11-16", "2026-11-29"},
		{"終了日が同じなら開始日が遅い方", []planning.Sprint{
			{ID: "a", Name: "A 1", Start: "2026-11-01", End: "2026-11-15"},
			{ID: "b", Name: "B 5", Start: "2026-11-05", End: "2026-11-15"},
		}, "B 6", "2026-11-16", "2026-11-29"},
		{"開始日も同じなら名前が後の方", []planning.Sprint{
			{ID: "a", Name: "A 1", Start: "2026-11-01", End: "2026-11-15"},
			{ID: "b", Name: "B 5", Start: "2026-11-01", End: "2026-11-15"},
		}, "B 6", "2026-11-16", "2026-11-29"},
		{"終了日のあるものがなければ今日", []planning.Sprint{s8}, "Sprint 2", "2026-11-14", "2026-11-27"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			name, start, end := planning.SprintDefaults(tt.live, today)
			if name != tt.name || start != tt.start || end != tt.end {
				t.Fatalf("got %q %q %q, want %q %q %q", name, start, end, tt.name, tt.start, tt.end)
			}
		})
	}
}

func TestSprintDefaultsExcludesDeletedByCaller(t *testing.T) {
	// 呼び出し側は LiveSprints を渡す。削除済みの Sprint 8 は数にも重なりにも入らない。
	p := planning.Plan{Sprints: []planning.Sprint{
		{ID: "S7", Name: "Sprint 7", End: "2026-11-15"},
		{ID: "S8", Name: "Sprint 8", End: "2026-11-30", Deleted: &change.Deletion{Op: "o1"}},
	}}
	name, start, _ := planning.SprintDefaults(p.LiveSprints(), "2026-11-14")
	if name != "Sprint 8" || start != "2026-11-16" {
		t.Fatalf("got %q %q", name, start)
	}
}

func TestRemainingDays(t *testing.T) {
	if n, over := planning.RemainingDays("2026-11-14", "2026-11-14"); n != 1 || over {
		t.Fatalf("当日 = %d %v", n, over)
	}
	if n, over := planning.RemainingDays("2026-11-15", "2026-11-14"); n != 2 || over {
		t.Fatalf("明日 = %d %v", n, over)
	}
	if _, over := planning.RemainingDays("2026-11-13", "2026-11-14"); !over {
		t.Fatal("昨日は期限切れ")
	}
}

func TestTodayAndAddDays(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.Local)
	if got := planning.Today(now); got != "2026-11-14" {
		t.Fatalf("Today = %q", got)
	}
	if got := planning.AddDays("2026-11-30", 1); got != "2026-12-01" {
		t.Fatalf("AddDays = %q", got)
	}
	if got := planning.AddDays("2026-11-14", 13); got != "2026-11-27" {
		t.Fatalf("AddDays = %q", got)
	}
}

func TestNormalizeName(t *testing.T) {
	if got := planning.NormalizeName("  Sprint 1 \n"); got != "Sprint 1" {
		t.Fatalf("NormalizeName = %q", got)
	}
}
