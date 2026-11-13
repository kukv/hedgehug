package transfers_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// statusIn は、Store store のワークスペース ws の、名前が name のステータスの内部 ID。
func statusIn(t *testing.T, s port.Stores, store port.StoreID, ws workspace.ID, name string) change.ID {
	t.Helper()
	var st workspace.Settings
	if err := s.Read(t.Context(), store, func(r port.Reader) (err error) { st, err = r.Workspaces().Settings(ws); return }); err != nil {
		t.Fatal(err)
	}
	for _, v := range st.Statuses {
		if v.Name == name {
			return v.ID
		}
	}
	t.Fatalf("no status %q in %s", name, ws)
	return ""
}

// hideStatus は、プロジェクト p で、ワークスペース ws のステータス name を使わないことにする(B5 のプロジェクト別のルール)。
func hideStatus(t *testing.T, s port.Stores, store port.StoreID, ws workspace.ID, p project.ID, name string) change.ID {
	t.Helper()
	id := statusIn(t, s, store, ws, name)
	write(t, s, store, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(ws, change.NewAdd(projectrule.HiddenField(p), id))
	})
	return id
}

// usesStatus は、移した it のステータスを、移動先のプロジェクトが使うか(projectrule.Uses)。
func usesStatus(t *testing.T, s port.Stores, store port.StoreID, it item.Item) bool {
	t.Helper()
	var ok bool
	if err := s.Read(t.Context(), store, func(r port.Reader) error {
		st, err := r.Workspaces().Settings(it.Place.Workspace)
		if err != nil {
			return err
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: it.Place.Workspace})
		ok = projectrule.Uses(st, projectrule.FromLog(log).Of(it.Place.Project), it.Status)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return ok
}

// pendingTaskWithStep は、HH-2 を Pending にし、その下に Pending の Step を足す。
func pendingTaskWithStep(t *testing.T, f fixture) item.Item {
	t.Helper()
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusPending))
	})
	parent := apptest.SampleTask
	return addItem(t, f.rec, store1, item.Item{ID: "01TESTIT000000000000000030", Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &parent, Number: 3}, Kind: item.Step, Rank: "V", Title: "s", Status: apptest.SampleStatusPending})
}

// 02「値の引き直し」: 名前(Pending)が移動先のワークスペースにあっても、移動先のプロジェクトが使わなければ、予告に置き換えの
// 行を出し、選択肢にそのステータスを出さない。移動では、使わないステータスを置き換え先に選んでも、同じ分類の既定にする。
func TestMoveReplacesAStatusTheDestinationProjectDoesNotUse(t *testing.T) {
	f := newFixture(t)
	step := pendingTaskWithStep(t, f)
	hidden := hideStatus(t, f.rec, store1, mobileWS, mobileProject, "Pending")
	preview, err := f.svc.Preview(t.Context(), store1, apptest.SampleTask, toMobile())
	if err != nil || len(preview.Replacements) != 1 {
		t.Fatalf("rows = %+v, %v", preview.Replacements, err)
	}
	row := preview.Replacements[0]
	todo := statusIn(t, f.rec, store1, mobileWS, "Todo")
	if row.Status != apptest.SampleStatusPending || row.Count != 2 || row.Default != todo || len(row.Choices) != 4 ||
		slices.ContainsFunc(row.Choices, func(c workspace.Value) bool { return c.ID == hidden }) {
		t.Errorf("row = %+v, want Pending with 4 choices without the hidden one", row)
	}
	in := toMobile()
	in.StatusChoices = map[change.ID]change.ID{apptest.SampleStatusPending: hidden}
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, in); err != nil {
		t.Fatal(err)
	}
	for _, id := range []item.ID{apptest.SampleTask, step.ID} {
		if it := itemOf(t, f.rec, store1, id); it.Status != todo || !usesStatus(t, f.rec, store1, it) {
			t.Errorf("%s status = %q, want Todo %q", id, it.Status, todo)
		}
	}
}

// 別の Store へ移すときも同じ規則(移動先の Team のプロジェクトが Pending を使わない)。
func TestMoveAcrossStoresReplacesAStatusTheDestinationProjectDoesNotUse(t *testing.T) {
	f := newFixture(t)
	step := pendingTaskWithStep(t, f)
	hideStatus(t, f.rec, store2, teamWS, teamProject, "Pending")
	preview, err := f.svc.Preview(t.Context(), store1, apptest.SampleTask, toTeam())
	if err != nil || len(preview.Replacements) != 1 || preview.Replacements[0].Status != apptest.SampleStatusPending {
		t.Fatalf("rows = %+v, %v", preview.Replacements, err)
	}
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); err != nil {
		t.Fatal(err)
	}
	todo := statusIn(t, f.rec, store2, teamWS, "Todo")
	for _, id := range []item.ID{apptest.SampleTask, step.ID} {
		if it := itemOf(t, f.rec, store2, id); it.Status != todo || !usesStatus(t, f.rec, store2, it) {
			t.Errorf("%s status = %q, want Todo %q", id, it.Status, todo)
		}
	}
}
