package statedb

import (
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/planning"
)

func TestReportSelections(t *testing.T) {
	ctx := t.Context()
	r := openTemp(t).ReportSelections()

	if _, ok, err := r.Get(ctx, "S1", "W1"); err != nil || ok {
		t.Fatalf("empty: ok=%v err=%v", ok, err)
	}

	// 空の Sprint・Estimate も、そのまま保つ。
	want := planning.Selection{Unit: planning.UnitCount, Layers: planning.LayersStatus}
	if err := r.Put(ctx, "S1", "W1", want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := r.Get(ctx, "S1", "W1")
	if err != nil || !ok || got != want {
		t.Fatalf("get: %+v ok=%v err=%v, want %+v", got, ok, err, want)
	}

	// 2 回目の Put は上書き。
	want = planning.Selection{Unit: planning.UnitPoints, Sprint: "SP1", Layers: planning.LayersCategory, Estimate: "SP2"}
	if err := r.Put(ctx, "S1", "W1", want); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := r.Get(ctx, "S1", "W1"); got != want {
		t.Errorf("overwrite: %+v, want %+v", got, want)
	}

	// 別の Store・別のワークスペースの行と混ざらない。
	other := planning.Selection{Unit: planning.UnitCount, Sprint: "SPX", Layers: planning.LayersStatus}
	if err := r.Put(ctx, "S2", "W1", other); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.Get(ctx, "S1", "W2"); ok {
		t.Error("W2 of S1 must be absent")
	}
	if got, _, _ := r.Get(ctx, "S1", "W1"); got != want {
		t.Errorf("S1 changed by S2: %+v", got)
	}
	if got, _, _ := r.Get(ctx, "S2", "W1"); got != other {
		t.Errorf("S2: %+v, want %+v", got, other)
	}
}

func TestReportSelectionsSurviveReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	want := planning.Selection{Unit: planning.UnitCount, Sprint: "SP1", Layers: planning.LayersStatus}
	if err := d.ReportSelections().Put(t.Context(), "S1", "W1", want); err != nil {
		t.Fatal(err)
	}
	_ = d.Close()
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	if got, ok, _ := d.ReportSelections().Get(t.Context(), "S1", "W1"); !ok || got != want {
		t.Errorf("after reopen: %+v ok=%v", got, ok)
	}
}
