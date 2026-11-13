package automations_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

func TestApplyIgnoringWorkflow(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, _ := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, apptest.SampleStatusDone); err != nil {
		t.Fatal(err)
	}
	if last := rec.summaries[len(rec.summaries)-1]; last != "「完了で外す」の変更を制約を無視して適用" {
		t.Errorf("summary = %q", last)
	}
	var recs []change.Record
	_ = rec.Read(ctx, store, func(rd port.Reader) error {
		recs, _ = rd.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		return nil
	})
	got := recs[len(recs)-1]
	want := change.Actor{Kind: change.ActorAutomation, Login: "hanako", Rule: r.ID}
	if got.F != "status" || got.Actor != want || string(got.To) != `"`+string(apptest.SampleStatusDone)+`"` {
		t.Errorf("row = %+v", got) // 自動化が行った変更、by は押した人(B4-11)
	}
	n := len(recs)
	_ = svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, apptest.SampleStatusDone) // すでに Done: 何も保存しない
	_ = rec.Read(ctx, store, func(rd port.Reader) error {
		recs, _ = rd.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		return nil
	})
	if len(recs) != n {
		t.Error("the same status must not be saved")
	}
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, "01NORULE", apptest.SampleStatusDone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("no rule: %v", err)
	}
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, "01NOSTATUS"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("no status: %v", err)
	}
}
