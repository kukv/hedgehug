package inbox

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

const (
	draftSprint change.ID = "01SAMPLESPRINT000000000001"
	otherTask   item.ID   = "01SAMPLEIT0000000000000005"
	systemAPI   change.ID = "01SAMPLEVAL000000000000012"
	systemDB    change.ID = "01SAMPLEVAL000000000000013"
)

// slotDrafts は、Sample の Store に prepare を書いてから、通知 n の slot と、その drafts の行を返す。
func slotDrafts(t *testing.T, n port.NoticeRow, prepare ...change.Draft) (slot, []change.Draft) {
	t.Helper()
	st := apptest.NewStores(apptest.NewClock(revertNow), apptest.NewIDs())
	st.AddStore("s1")
	if err := apptest.Sample(t.Context(), st, "s1"); err != nil {
		t.Fatal(err)
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "テスト"}
	err := st.Write(t.Context(), "s1", op, func(tx port.Tx) error {
		sp := planning.Sprint{ID: draftSprint, Name: "Sprint 1", State: planning.Planned}
		other := item.Item{
			ID: otherTask, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 5},
			Kind: item.Task, Rank: "x", Title: "別", Status: apptest.SampleStatusTodo,
		}
		if err := tx.WriteWorkspaces().Append(apptest.SampleWorkspace, planning.SprintDrafts(planning.Sprint{}, sp)...); err != nil {
			return err
		}
		if err := tx.WriteItems().Append(other.ID, item.Create(other)); err != nil {
			return err
		}
		if len(prepare) == 0 {
			return nil
		}
		return tx.WriteItems().Append(apptest.SampleTask, prepare...)
	})
	if err != nil {
		t.Fatal(err)
	}
	var s slot
	var ds []change.Draft
	// drafts は保存の中で読む。何も書かずに捨てる
	errDiscard := errors.New("discard")
	err = st.Write(t.Context(), "s1", op, func(tx port.Tx) error {
		var state slotState
		if s, state, err = slotOf(tx, n, revertNow); err != nil || state != slotLive {
			t.Fatalf("state = %v, err = %v", state, err)
		}
		if ds, err = s.drafts(tx); err != nil {
			return err
		}
		return errDiscard
	})
	if !errors.Is(err, errDiscard) {
		t.Fatal(err)
	}
	return s, ds
}

func itemNotice(field string, mine, kept any) port.NoticeRow {
	return port.NoticeRow{Notice: port.Notice{
		Kind: port.NoticeOverwrite, Store: "s1", Workspace: apptest.SampleWorkspace, Item: apptest.SampleTask,
		Field: field, Mine: change.Value(mine), Kept: change.Value(kept),
	}}
}

func draftText(ds []change.Draft) []string {
	out := []string{}
	for _, d := range ds {
		out = append(out, string(d.T)+" "+d.F+" "+string(d.From)+" "+string(d.To))
	}
	return out
}

// 複数の値は、いまの値と Mine の差を remove が先で書く(C4c2-8)。
func TestSlotDraftsMulti(t *testing.T) {
	mine := []change.ID{systemAPI, systemDB}
	s, ds := slotDrafts(t, itemNotice("systems", mine, []change.ID{apptest.SampleSystemWeb, systemAPI}),
		change.NewAdd("systems", apptest.SampleSystemWeb), change.NewAdd("systems", systemAPI))
	want := []string{
		`remove systems "` + string(apptest.SampleSystemWeb) + `" `,
		`add systems  "` + string(systemDB) + `"`,
	}
	if got := draftText(ds); len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("drafts = %q, want %q", got, want)
	}
	if s.summary != "HH-2 の関連システムを上書き前の値に戻す" {
		t.Errorf("summary = %q", s.summary)
	}
}

// スプリントは、移動先の枠の末尾の rank も書く。バックログの末尾は、別の Task。
func TestSlotDraftsSprint(t *testing.T) {
	_, ds := slotDrafts(t, itemNotice("sprint", nil, draftSprint), change.NewSet("sprint", nil, draftSprint))
	got := draftText(ds)
	if len(got) != 2 || got[0] != `set sprint "`+string(draftSprint)+`" null` || ds[1].F != "rank" {
		t.Fatalf("drafts = %q, want set sprint and rank", got)
	}
	var r string
	if err := json.Unmarshal(ds[1].To, &r); err != nil || r <= "x" {
		t.Errorf("rank = %q, want after x", r)
	}
}

// プロジェクトの空は ""(SetOwner と同じ形)。
func TestSlotDraftsProjectOwner(t *testing.T) {
	n := itemNotice("owner", "", "taro")
	n.Item, n.Project = "", apptest.SampleProject
	s, ds := slotDrafts(t, n)
	if got := draftText(ds); len(got) != 1 || got[0] != `set owner "taro" ""` {
		t.Errorf("drafts = %q", got)
	}
	if s.summary != "ログイン のオーナーを上書き前の値に戻す" {
		t.Errorf("summary = %q", s.summary)
	}
}
