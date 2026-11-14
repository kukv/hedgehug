package reports_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	dp "github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestActive(t *testing.T) {
	svc, sel, s := setup(t)
	a, err := svc.Active(t.Context(), store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if a.Sprint != nil || len(a.Burndown.Days) != 0 || a.Added != 0 || a.Done != (dp.Totals{}) {
		t.Errorf("実行中がなければ空: %+v", a)
	}

	activeWithWork(t, s)
	a, err = svc.Active(t.Context(), store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if a.Sprint == nil || a.Sprint.ID != sprint1 || a.Unit != dp.UnitPoints {
		t.Fatalf("Sprint = %+v, Unit = %s", a.Sprint, a.Unit)
	}
	if a.Done != (dp.Totals{Points: 3, Count: 1}) || a.Total != (dp.Totals{Points: 5, Count: 2}) {
		t.Errorf("Done = %+v, Total = %+v", a.Done, a.Total)
	}
	if a.Added != 5 || a.Burndown.Shown != 5 {
		t.Errorf("Added = %v, Shown = %d", a.Added, a.Burndown.Shown)
	}

	sel.rows = map[workspace.ID]dp.Selection{apptest.SampleWorkspace: {Unit: dp.UnitCount}}
	a, err = svc.Active(t.Context(), store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if a.Unit != dp.UnitCount || a.Added != 2 {
		t.Errorf("件数: Unit = %s, Added = %v", a.Unit, a.Added)
	}
}
