package workspaces_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// deleteFixture は、ワークスペースに作ったプロジェクト 3 つとアイテム。
// pa にアイテム 2 件(goneA は削除済み)、pb にアイテム 1 件、pc は削除済み(下のアイテム 1 件は削除されていない)。
type deleteFixture struct {
	pa, pb, pc                     project.Project
	liveA, goneA, liveB, underGone item.ID
}

func newDeleteFixture(t *testing.T, s workspaces.Service, ws workspace.ID) deleteFixture {
	t.Helper()
	mk := func(n int, rank string) project.Project {
		return project.Project{ID: s.IDs.New(), Place: project.Place{Workspace: ws, Number: n}, Rank: rank, Title: "p", Status: project.Todo}
	}
	f := deleteFixture{pa: mk(1, "V"), pb: mk(2, "W"), pc: mk(3, "X")}
	f.liveA = putItemIn(t, s, f.pa, 1)
	f.goneA = putItemIn(t, s, f.pa, 2)
	f.liveB = putItemIn(t, s, f.pb, 1)
	f.underGone = putItemIn(t, s, f.pc, 1)
	ctx := t.Context()
	if err := apptest.DeleteItems(ctx, s.Stores, store, f.goneA); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteProject(ctx, s.Stores, store, f.pc.ID); err != nil {
		t.Fatal(err)
	}
	return f
}

// logOf は、ref のログを返す。
func logOf(t *testing.T, s workspaces.Service, ref change.Ref) []change.Record {
	t.Helper()
	var out []change.Record
	if err := s.Stores.Read(t.Context(), store, func(r port.Reader) error {
		var err error
		out, err = r.Changes().Of(ref)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestDeletePreviewCountsLiveProjectsAndItems(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	newDeleteFixture(t, s, w.ID)
	got, err := s.DeletePreview(t.Context(), store, w.ID)
	// 生きているプロジェクトは pa・pb の 2 件、アイテムは liveA・liveB の 2 件。削除済みは数えない。
	if err != nil || got.Projects != 2 || got.Items != 2 || got.RetentionDays != 30 {
		t.Errorf("preview = %+v, %v", got, err)
	}
}

func TestDeleteWritesRowsInOneOperation(t *testing.T) {
	s := setup(t)
	rec := &recorder{Stores: s.Stores}
	s.Stores = rec
	w, _ := s.Create(t.Context(), store, "Web\n  App", "HH")
	f := newDeleteFixture(t, s, w.ID)
	logLens := map[change.Ref]int{}
	for _, ref := range []change.Ref{
		{Kind: change.KindProject, ID: f.pc.ID}, {Kind: change.KindItem, ID: f.goneA}, {Kind: change.KindItem, ID: f.underGone},
	} {
		logLens[ref] = len(logOf(t, s, ref))
	}
	if err := s.Delete(t.Context(), store, w.ID); err != nil {
		t.Fatal(err)
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "ワークスペース「Web App」を削除" {
		t.Errorf("summary = %q", got)
	}
	last := func(ref change.Ref) change.Record {
		log := logOf(t, s, ref)
		return log[len(log)-1]
	}
	wsRow := last(change.Ref{Kind: change.KindWorkspace, ID: w.ID})
	if wsRow.T != change.Delete || wsRow.F != change.DeletedField || wsRow.Root == nil || !*wsRow.Root {
		t.Fatalf("workspace row = %+v", wsRow)
	}
	for _, ref := range []change.Ref{
		{Kind: change.KindProject, ID: f.pa.ID},
		{Kind: change.KindProject, ID: f.pb.ID},
		{Kind: change.KindItem, ID: f.liveA},
		{Kind: change.KindItem, ID: f.liveB},
	} {
		row := last(ref)
		if row.T != change.Delete || row.F != change.DeletedField || row.Root == nil || *row.Root || row.Op != wsRow.Op {
			t.Errorf("%v: row = %+v, want root false in op %s", ref, row, wsRow.Op)
		}
	}
	for ref, n := range logLens {
		if got := len(logOf(t, s, ref)); got != n {
			t.Errorf("%v: log grew %d -> %d", ref, n, got)
		}
	}
}

func TestDeleteRefusesWhatIsGone(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	if err := s.Delete(t.Context(), store, w.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []workspace.ID{w.ID, "01NOSUCHWS00000000000000001"} {
		if err := s.Delete(t.Context(), store, id); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("Delete(%s) = %v", id, err)
		}
		if _, err := s.DeletePreview(t.Context(), store, id); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("DeletePreview(%s) = %v", id, err)
		}
	}
}

// 設定には行を足さない。削除のあとも設定は削除前のまま読める(B5b-2)。
func TestDeleteKeepsSettings(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	before := settingsOf(t, s, w.ID)
	if err := s.Delete(t.Context(), store, w.ID); err != nil {
		t.Fatal(err)
	}
	after := settingsOf(t, s, w.ID)
	if len(after.Statuses) == 0 || len(after.Statuses) != len(before.Statuses) || after.Statuses[0] != before.Statuses[0] {
		t.Errorf("settings changed: %+v -> %+v", before, after)
	}
}

// Store のワークスペースが 1 つだけでも削除できる。そのあとの一覧は空(Store はワークスペース 0 件でも Store)。
func TestDeleteLastWorkspace(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	if err := s.Delete(t.Context(), store, w.ID); err != nil {
		t.Fatal(err)
	}
	got, err := s.List(t.Context(), store)
	if err != nil || len(got) != 0 {
		t.Errorf("List = %+v, %v", got, err)
	}
}
