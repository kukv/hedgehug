package items_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

func raw(s string) json.RawMessage { return json.RawMessage(s) }

func TestSetPropertySavesOneChange(t *testing.T) {
	svc, rec := setupRec(t)
	it, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", raw(`"`+string(apptest.SampleStatusDone)+`"`))
	if err != nil || it.Status != apptest.SampleStatusDone {
		t.Fatalf("SetProperty = %+v, %v", it, err)
	}
	if last(rec) != "HH-2 のステータスを Done に変更" {
		t.Errorf("summary = %q", last(rec))
	}
	n := logLen(t, rec, apptest.SampleTask)
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", raw(`"`+string(apptest.SampleStatusDone)+`"`)); err != nil {
		t.Fatal(err)
	}
	if logLen(t, rec, apptest.SampleTask) != n {
		t.Error("the same value must not be saved")
	}
}

func TestSetPropertyWritesTheValueText(t *testing.T) {
	svc, rec := setupRec(t)
	cases := []struct{ prop, in, want string }{
		{"assignee", `"taro"`, "HH-2 の担当者を taro に変更"},
		{"assignee", `""`, "HH-2 の担当者を なし に変更"},
		{"start", `"2026-11-11"`, "HH-2 の開始日を 11/11 に変更"},
		{"estimate", `"1.25"`, "HH-2 の見積もり時間を 1.25 時間 に変更"},
		{"points", `"3"`, "HH-2 のポイントを 3 に変更"},
	}
	for _, c := range cases {
		if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, change.ID(c.prop), raw(c.in)); err != nil {
			t.Fatalf("%s: %v", c.prop, err)
		}
		if last(rec) != c.want {
			t.Errorf("summary = %q, want %q", last(rec), c.want)
		}
	}
}

func TestSetPropertyRefusesBadDatesAndNumbers(t *testing.T) {
	svc, rec := setupRec(t)
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "start", raw(`"2026-11-11"`)); err != nil {
		t.Fatal(err)
	}
	n := logLen(t, rec, apptest.SampleTask)
	cases := []struct{ prop, in, want string }{
		{"end", `"2026-11-10"`, "終了日は開始日以降にしてください"},
		{"points", `"-1"`, "ポイントは 0 以上の数で入力してください"},
		{"estimate", `"0.123"`, "見積もり時間は小数第 2 位までで入力してください"},
	}
	for _, c := range cases {
		_, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, change.ID(c.prop), raw(c.in))
		var r *refusal.Refusal
		if !errors.As(err, &r) || r.Message != c.want {
			t.Errorf("%s: err = %v", c.prop, err)
		}
	}
	if logLen(t, rec, apptest.SampleTask) != n {
		t.Error("refused changes must not be saved")
	}
}

func TestSetPropertyUnknownIsNotFound(t *testing.T) {
	svc, _ := setupRec(t)
	_, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", raw(`"01NOPE00000000000000000000"`))
	if !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
	_, err = svc.SetProperty(t.Context(), store, apptest.SampleTask, "01NOPROP000000000000000000", raw(`null`))
	if !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestToggleSystems(t *testing.T) {
	svc, rec := setupRec(t)
	web := apptest.SampleSystemWeb
	it, err := svc.AddValue(t.Context(), store, apptest.SampleTask, "systems", web)
	if err != nil || len(it.Systems) != 1 || last(rec) != "HH-2 の関連システムを Web に変更" {
		t.Fatalf("AddValue = %+v, %v, %q", it, err, last(rec))
	}
	n := logLen(t, rec, apptest.SampleTask)
	_, _ = svc.AddValue(t.Context(), store, apptest.SampleTask, "systems", web)
	if logLen(t, rec, apptest.SampleTask) != n {
		t.Error("adding a present value must not be saved")
	}
	it, err = svc.RemoveValue(t.Context(), store, apptest.SampleTask, "systems", web)
	if err != nil || len(it.Systems) != 0 || last(rec) != "HH-2 の関連システムを なし に変更" {
		t.Errorf("RemoveValue = %+v, %v, %q", it, err, last(rec))
	}
}

func TestCheckStep(t *testing.T) {
	svc, rec := setupRec(t)
	step, err := svc.AddStep(t.Context(), store, apptest.SampleTask, "確認")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetProperty(t.Context(), store, step.ID, "status", raw(`"`+string(apptest.SampleStatusInProgress)+`"`)); err != nil {
		t.Fatal(err)
	}
	it, err := svc.CheckStep(t.Context(), store, step.ID, true)
	if err != nil || it.Status != apptest.SampleStatusDone || last(rec) != "HH-3 のステータスを Done に変更" {
		t.Fatalf("check = %+v, %v, %q", it, err, last(rec))
	}
	// 進行中から完了にして外すと、未着手の既定に戻る(02「Step のチェック」)。
	it, err = svc.CheckStep(t.Context(), store, step.ID, false)
	if err != nil || it.Status != apptest.SampleStatusTodo {
		t.Errorf("uncheck = %+v, %v", it, err)
	}
	if _, err := svc.CheckStep(t.Context(), store, apptest.SampleTask, true); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("a Task is not a Step: %v", err)
	}
}
