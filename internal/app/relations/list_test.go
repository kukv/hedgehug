package relations_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/relations"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
)

func TestListShowsBothSidesAcrossWorkspaces(t *testing.T) {
	svc, rec := setup(t)
	addRaw(t, rec, apptest.SampleEpic, "blocks", apptest.SampleTask)
	addRaw(t, rec, itDone, "blocks", apptest.SampleEpic)
	addRaw(t, rec, apptest.SampleEpic, "relates", itMobile)
	addRaw(t, rec, apptest.SampleEpic, "clones", itDone)
	rows, err := svc.List(t.Context(), store, apptest.SampleEpic)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, fmt.Sprintf("%s %s", r.Label, r.Display))
	}
	want := []string{"blocks HH-2", "blocked by HH-3", "relates to MB-1", "clones HH-3"}
	if !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if rows[0].WorkspaceName != "" || rows[2].WorkspaceName != "Mobile" {
		t.Errorf("別のワークスペースの相手にだけ名前を添える: %q, %q", rows[0].WorkspaceName, rows[2].WorkspaceName)
	}
	if !rows[1].HasStatus || rows[1].Status.Name != "Done" || rows[2].HasStatus {
		t.Errorf("相手のステータス: %+v, %+v", rows[1].Status, rows[2].HasStatus)
	}
	if rows[0].Item.Title != "ログイン画面を作る" {
		t.Errorf("Item = %+v", rows[0].Item)
	}
}

func TestListOfNothingAndOfMissing(t *testing.T) {
	svc, _ := setup(t)
	rows, err := svc.List(t.Context(), store, apptest.SampleEpic)
	if err != nil || len(rows) != 0 {
		t.Errorf("関連がなければ空: %+v, %v", rows, err)
	}
	if _, err := svc.List(t.Context(), store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("ないアイテム: %v", err)
	}
}

func shown(cs []relations.Candidate) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Display+"|"+c.DisabledReason)
	}
	return out
}

func TestCandidates(t *testing.T) {
	svc, rec := setup(t)
	addRaw(t, rec, apptest.SampleEpic, "blocks", apptest.SampleTask)
	cases := []struct {
		name  string
		self  item.ID
		label relation.Label
		q     string
		want  []string
	}{
		// 自分(HH-2)と Step(HH-4)は出ない。HH-1 は、向きが逆でも blocks があるので選べない。
		{"ID の前方一致", apptest.SampleTask, relation.Blocks, "hh", []string{"HH-1|すでに blocks の関連があります", "HH-3|"}},
		{"大文字小文字を区別しない", apptest.SampleTask, relation.Blocks, "HH-3", []string{"HH-3|"}},
		{"タイトルの部分一致", apptest.SampleTask, relation.Blocks, "決", []string{"HH-3|"}},
		{"逆向きの表記でも同じ種類は 1 つ", apptest.SampleTask, relation.BlockedBy, "HH-1", []string{"HH-1|すでに blocked by の関連があります"}},
		{"種類が違えば選べる", apptest.SampleTask, relation.Causes, "HH-1", []string{"HH-1|"}},
		{"空の検索語は出さない", apptest.SampleTask, relation.Blocks, "  ", nil},
		{"Step も自分の側からは関連を持てるが、候補には Step を出さない", apptest.SampleEpic, relation.RelatesTo, "設計", nil},
	}
	for _, c := range cases {
		got, err := svc.Candidates(t.Context(), store, c.self, c.label, c.q)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !slices.Equal(shown(got), c.want) {
			t.Errorf("%s: candidates = %v, want %v", c.name, shown(got), c.want)
		}
	}
	got, err := svc.Candidates(t.Context(), store, apptest.SampleEpic, relation.RelatesTo, "ios")
	if err != nil || len(got) != 1 || got[0].Display != "MB-1" || got[0].WorkspaceName != "Mobile" || got[0].Item.ID != itMobile {
		t.Errorf("別のワークスペースの候補にはワークスペース名を添える: %+v, %v", got, err)
	}
	if _, err := svc.Candidates(t.Context(), store, apptest.SampleTask, "friends with", "HH"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("知らない表記: %v", err)
	}
}

func TestCandidatesDisableACycle(t *testing.T) {
	svc, rec := setup(t)
	addRaw(t, rec, apptest.SampleEpic, "blocks", apptest.SampleTask) // HH-1 → HH-2
	addRaw(t, rec, apptest.SampleTask, "blocks", itDone)             // HH-2 → HH-3
	// HH-3 が HH-1 をブロックすると、HH-1 → HH-2 → HH-3 → HH-1 の輪になる。
	got, err := svc.Candidates(t.Context(), store, itDone, relation.Blocks, "HH-1")
	if err != nil || !slices.Equal(shown(got), []string{"HH-1|循環になります"}) {
		t.Errorf("blocks: %v, %v", shown(got), err)
	}
	// HH-1 が HH-3 をブロックするのは近道を足すだけで、輪にならない。
	got, err = svc.Candidates(t.Context(), store, itDone, relation.BlockedBy, "HH-1")
	if err != nil || !slices.Equal(shown(got), []string{"HH-1|"}) {
		t.Errorf("blocked by: %v, %v", shown(got), err)
	}
	// ほかの種類は循環を検査しない。
	got, err = svc.Candidates(t.Context(), store, itDone, relation.Causes, "HH-1")
	if err != nil || !slices.Equal(shown(got), []string{"HH-1|"}) {
		t.Errorf("causes: %v, %v", shown(got), err)
	}
}

func TestCandidatesStopAtTwenty(t *testing.T) {
	svc, rec := setup(t)
	err := rec.Write(t.Context(), store, port.Operation{Actor: change.User("taro"), Summary: "まとめて作る"}, func(tx port.Tx) error {
		epic := apptest.SampleEpic
		for i := range 25 {
			it := item.Item{
				ID: item.ID(fmt.Sprintf("01TESTBK%018d", i)), Kind: item.Task, Rank: "W", Title: fmt.Sprintf("まとめ %d", i),
				Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &epic, Number: 10 + i},
			}
			if err := tx.WriteItems().Append(it.ID, item.Create(it)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Candidates(t.Context(), store, apptest.SampleTask, relation.RelatesTo, "まとめ")
	if err != nil || len(got) != 20 || got[0].Display != "HH-10" || got[19].Display != "HH-29" {
		t.Errorf("20 件で打ち切り、番号の順: %d 件, %v", len(got), err)
	}
}
