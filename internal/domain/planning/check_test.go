package planning_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// アイテムを入れられるスプリント(F1-5・F1-15)。作成と詳細パネルの保存が同じ決まりを使う。
func TestAssignableSprint(t *testing.T) {
	p := planning.Plan{Sprints: []planning.Sprint{
		{ID: "S1", Name: "Sprint 1", State: planning.Planned},
		{ID: "S2", Name: "Sprint 2", State: planning.Closed},
		{ID: "S3", Name: "Sprint 3", State: planning.Active, Deleted: &change.Deletion{}},
	}}
	if sp, err := planning.AssignableSprint(p, "S1", item.Task, ""); err != nil || sp.ID != "S1" {
		t.Errorf("計画中: %+v, %v", sp, err)
	}
	if _, err := planning.AssignableSprint(p, "S1", item.Epic, ""); !errors.Is(err, planning.ErrNotWork) {
		t.Errorf("Epic: %v", err)
	}
	for _, id := range []change.ID{"S3", "S9"} {
		if _, err := planning.AssignableSprint(p, id, item.Task, ""); !errors.Is(err, planning.ErrNoSprint) {
			t.Errorf("%s: %v", id, err)
		}
	}
	_, err := planning.AssignableSprint(p, "S2", item.Bug, "")
	wantRefusal(t, err, "完了したスプリントには入れられません")
	if sp, err := planning.AssignableSprint(p, "S2", item.Bug, "S2"); err != nil || sp.ID != "S2" {
		t.Errorf("いま入っている完了: %+v, %v", sp, err)
	}
}

func wantRefusal(t *testing.T, err error, msg string) {
	t.Helper()
	if msg == "" {
		if err != nil {
			t.Fatalf("通るはずが断られた: %v", err)
		}
		return
	}
	var r *refusal.Refusal
	ok := errors.As(err, &r)
	if !ok || r.Message != msg {
		t.Fatalf("err = %v, want 断り %q", err, msg)
	}
}

func TestCheckSprint(t *testing.T) {
	const today = "2026-11-14"
	live := []planning.Sprint{
		{ID: "S1", Name: "Sprint 1", State: planning.Planned},
		{ID: "S2", Name: "Sprint 2", State: planning.Planned},
	}
	tests := []struct {
		name string
		s    planning.Sprint
		want string
	}{
		{"空白だけの名前", planning.Sprint{ID: "S3", Name: "  ", State: planning.Planned}, "名前を入力してください"},
		{"自分以外に同じ名前", planning.Sprint{ID: "S3", Name: " Sprint 1 ", State: planning.Planned}, "同じ名前のスプリントがあります"},
		{"自分の名前は重ならない", planning.Sprint{ID: "S1", Name: "Sprint 1", State: planning.Planned}, ""},
		{"大文字小文字は別", planning.Sprint{ID: "S3", Name: "sprint 1", State: planning.Planned}, ""},
		{"実行中で終了日が空", planning.Sprint{ID: "S3", Name: "A", State: planning.Active, Start: "2026-11-01"}, "実行中のスプリントには開始日と終了日が必要です"},
		{"完了で開始日が空", planning.Sprint{ID: "S3", Name: "A", State: planning.Closed, End: "2026-11-01"}, "実行中のスプリントには開始日と終了日が必要です"},
		{"終了日が開始日より前", planning.Sprint{ID: "S3", Name: "A", State: planning.Planned, Start: "2026-11-10", End: "2026-11-09"}, "終了日は開始日以降にしてください"},
		{"完了で開始日が明日", planning.Sprint{ID: "S3", Name: "A", State: planning.Closed, Start: "2026-11-15", End: "2026-11-20"}, "開始日は今日以前にしてください"},
		{"計画中で開始日が明日", planning.Sprint{ID: "S3", Name: "A", State: planning.Planned, Start: "2026-11-15", End: "2026-11-20"}, ""},
		{"計画中で両方空", planning.Sprint{ID: "S3", Name: "A", State: planning.Planned}, ""},
		{"表の上の文が先(名前の重複と日付)", planning.Sprint{ID: "S3", Name: "Sprint 2", State: planning.Active, Start: "2026-11-10", End: "2026-11-09"}, "同じ名前のスプリントがあります"},
		{"表の上の文が先(空と逆転)", planning.Sprint{ID: "S3", Name: "A", State: planning.Active, Start: "2026-11-20"}, "実行中のスプリントには開始日と終了日が必要です"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantRefusal(t, planning.CheckSprint(tt.s, live, today), tt.want)
		})
	}
}

func TestCheckStart(t *testing.T) {
	const today = "2026-11-14"
	tests := []struct {
		name, start, end, want string
	}{
		{"開始日が空", "", "2026-11-20", "開始日と終了日を入力してください"},
		{"終了日が空", "2026-11-10", "", "開始日と終了日を入力してください"},
		{"逆転", "2026-11-12", "2026-11-11", "終了日は開始日以降にしてください"},
		{"明日の開始日", "2026-11-15", "2026-11-20", "開始日は今日以前にしてください"},
		{"同じ日", "2026-11-14", "2026-11-14", ""},
		{"過去の開始日", "2026-11-01", "2026-11-14", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantRefusal(t, planning.CheckStart(tt.start, tt.end, today), tt.want)
		})
	}
}

func TestCheckRelease(t *testing.T) {
	live := []planning.Release{{ID: "R1", Name: "v1"}}
	wantRefusal(t, planning.CheckRelease(planning.Release{ID: "R2", Name: " "}, live), "名前を入力してください")
	wantRefusal(t, planning.CheckRelease(planning.Release{ID: "R2", Name: " v1 "}, live), "同じ名前のリリースがあります")
	wantRefusal(t, planning.CheckRelease(planning.Release{ID: "R1", Name: "v1"}, live), "")
	wantRefusal(t, planning.CheckRelease(planning.Release{ID: "R2", Name: "V1"}, live), "")
}
