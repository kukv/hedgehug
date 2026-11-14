package planning_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/planning"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	dp "github.com/kukv/hedgehug/internal/domain/planning"
)

// releases は、保存されているリリース(削除済みを除く)。
func releases(t *testing.T, svc planning.Service) []dp.Release {
	t.Helper()
	var out []dp.Release
	err := svc.Stores.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
		out = dp.FromLog(log).LiveReleases()
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCreateRelease(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, err := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: " v2.1.0 ", Date: "2026-12-01", Description: "d"})
	if err != nil {
		t.Fatal(err)
	}
	if r.ID == "" || r.Name != "v2.1.0" || r.State != dp.Unreleased || r.Date != "2026-12-01" || r.Description != "d" {
		t.Errorf("CreateRelease = %+v", r)
	}
	if got := lastSummary(rec); got != "リリース「v2.1.0」を作成" {
		t.Errorf("summary = %q", got)
	}
	if got := releases(t, svc); len(got) != 1 || got[0].ID != r.ID || got[0].State != dp.Unreleased {
		t.Errorf("stored = %+v", got)
	}
	_, err = svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v2.1.0"})
	if message(err) != "同じ名前のリリースがあります" {
		t.Errorf("duplicate: %v", err)
	}
	_, err = svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "  "})
	if message(err) != "名前を入力してください" {
		t.Errorf("empty: %v", err)
	}
	if got := releases(t, svc); len(got) != 1 {
		t.Errorf("refused saved: %+v", got)
	}
	if _, err := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "A\nB"}); err != nil {
		t.Fatal(err)
	}
	if got := lastSummary(rec); got != "リリース「A B」を作成" {
		t.Errorf("summary = %q", got)
	}
}

func TestUpdateRelease(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	a, _ := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v1"})
	_, _ = svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v2"})
	if err := svc.SetReleaseState(ctx, store, apptest.SampleWorkspace, a.ID, dp.Archived); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateRelease(ctx, store, apptest.SampleWorkspace, a.ID, planning.ReleaseInput{Name: " v1.1 ", Date: "2026-12-05", Description: "x"}); err != nil {
		t.Fatal(err)
	}
	if got := lastSummary(rec); got != "リリース「v1.1」を編集" {
		t.Errorf("summary = %q", got)
	}
	var got dp.Release
	for _, r := range releases(t, svc) {
		if r.ID == a.ID {
			got = r
		}
	}
	if got.Name != "v1.1" || got.Date != "2026-12-05" || got.Description != "x" || got.State != dp.Archived {
		t.Errorf("stored = %+v", got)
	}
	err := svc.UpdateRelease(ctx, store, apptest.SampleWorkspace, a.ID, planning.ReleaseInput{Name: "v2"})
	if message(err) != "同じ名前のリリースがあります" {
		t.Errorf("duplicate: %v", err)
	}
	err = svc.UpdateRelease(ctx, store, apptest.SampleWorkspace, "01NOSUCHRELEASE0000000000", planning.ReleaseInput{Name: "z"})
	if !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

// findRelease は、保存されているリリース id。なければ空。
func findRelease(t *testing.T, svc planning.Service, id change.ID) dp.Release {
	t.Helper()
	for _, r := range releases(t, svc) {
		if r.ID == id {
			return r
		}
	}
	return dp.Release{}
}

// setState は、状態を変え、want が空でなければアクティビティの文も確かめる。
func setState(t *testing.T, svc planning.Service, rec *recorder, id change.ID, to dp.ReleaseState, want string) {
	t.Helper()
	if err := svc.SetReleaseState(t.Context(), store, apptest.SampleWorkspace, id, to); err != nil {
		t.Fatal(err)
	}
	if want != "" && lastSummary(rec) != want {
		t.Errorf("summary = %q, want %q", lastSummary(rec), want)
	}
}

func TestReleaseState(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	a, _ := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v2.1.0"})
	b, _ := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v3", Date: "2027-01-01"})
	find := func(id change.ID) dp.Release { return findRelease(t, svc, id) }
	set := func(id change.ID, to dp.ReleaseState, want string) { setState(t, svc, rec, id, to, want) }
	set(a.ID, dp.Released, "リリース「v2.1.0」をリリース済みにする")
	if r := find(a.ID); r.State != dp.Released || r.Date != "2026-11-13" {
		t.Errorf("予定日なし: %+v", r)
	}
	set(b.ID, dp.Released, "リリース「v3」をリリース済みにする")
	if r := find(b.ID); r.Date != "2027-01-01" {
		t.Errorf("予定日あり: %+v", r)
	}
	set(a.ID, dp.Archived, "リリース「v2.1.0」をアーカイブにする")
	set(a.ID, dp.Unreleased, "リリース「v2.1.0」を未リリースにする")
	if r := find(a.ID); r.State != dp.Unreleased || r.Date != "2026-11-13" {
		t.Errorf("戻す: %+v", r)
	}
	// 同じ状態は何も保存しない。
	n := len(rec.summaries)
	set(a.ID, dp.Unreleased, "")
	if len(rec.summaries) != n {
		t.Errorf("同じ状態で保存した: %v", rec.summaries[n:])
	}
}

func TestReleaseStateRefused(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	a, _ := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v1"})
	n := len(rec.summaries)
	if err := svc.SetReleaseState(ctx, store, apptest.SampleWorkspace, a.ID, "bogus"); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("unknown state: %v", err)
	}
	if err := svc.SetReleaseState(ctx, store, apptest.SampleWorkspace, "01NOSUCHRELEASE0000000000", dp.Released); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown release: %v", err)
	}
	if len(rec.summaries) != n {
		t.Errorf("refused saved: %v", rec.summaries[n:])
	}
}
