package items_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// addSteps は、サンプルの Task の下に Step を末尾へ足す(HH-3 から順に)。
func addSteps(t *testing.T, svc items.Service, titles ...string) []item.Item {
	t.Helper()
	var out []item.Item
	for _, title := range titles {
		st, err := svc.AddStep(t.Context(), store, apptest.SampleTask, title)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, st)
	}
	return out
}

// order は、Task の下の削除されていない Step のタイトルを、画面の並び(並び順の値 → 作成の時刻 → 内部 ID)で返す。
func order(t *testing.T, svc items.Service) []string {
	t.Helper()
	all, err := svc.List(t.Context(), store, apptest.SampleProject)
	if err != nil {
		t.Fatal(err)
	}
	var kids []item.Item
	for _, it := range item.Live(all) {
		if it.Place.Parent != nil && *it.Place.Parent == apptest.SampleTask {
			kids = append(kids, it)
		}
	}
	item.SortSiblings(kids)
	var out []string
	for _, k := range kids {
		out = append(out, k.Title)
	}
	return out
}

// rankRows は、アイテムのログの `set rank` の行の数。
func rankRows(t *testing.T, s port.Stores, id item.ID) int {
	t.Helper()
	n := 0
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		for _, rec := range log {
			if rec.T == change.Set && rec.F == "rank" {
				n++
			}
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestReorderPutsTheItemBeforeTheAnchor(t *testing.T) {
	svc, rec := setupRec(t)
	st := addSteps(t, svc, "a", "b", "c")
	if err := svc.Reorder(t.Context(), store, st[1].ID, st[0].ID, false); err != nil {
		t.Fatal(err)
	}
	if got := last(rec); got != "HH-4 の順位を変更" {
		t.Errorf("summary = %q", got)
	}
	if got := order(t, svc); !slices.Equal(got, []string{"b", "a", "c"}) {
		t.Errorf("order = %v", got)
	}
	// 動いたのは b だけ。
	if rankRows(t, rec, st[0].ID) != 0 || rankRows(t, rec, st[1].ID) != 1 || rankRows(t, rec, st[2].ID) != 0 {
		t.Error("something other than b was written")
	}
	if err := svc.Reorder(t.Context(), store, st[2].ID, st[1].ID, false); err != nil {
		t.Fatal(err)
	}
	if got := order(t, svc); !slices.Equal(got, []string{"c", "b", "a"}) {
		t.Errorf("order = %v", got)
	}
}

func TestReorderPutsTheItemAfterTheAnchor(t *testing.T) {
	svc, rec := setupRec(t)
	st := addSteps(t, svc, "a", "b", "c")
	if err := svc.Reorder(t.Context(), store, st[0].ID, st[1].ID, true); err != nil {
		t.Fatal(err)
	}
	if got := order(t, svc); !slices.Equal(got, []string{"b", "a", "c"}) {
		t.Errorf("order = %v", got)
	}
	if err := svc.Reorder(t.Context(), store, st[0].ID, st[2].ID, true); err != nil {
		t.Fatal(err)
	}
	if got := order(t, svc); !slices.Equal(got, []string{"b", "c", "a"}) {
		t.Errorf("order = %v", got)
	}
	if rankRows(t, rec, st[1].ID) != 0 || rankRows(t, rec, st[2].ID) != 0 {
		t.Error("something other than a was written")
	}
}

// 04 の例: 並びが HH-3・HH-4・HH-5・HH-7・HH-9 で、HH-4 と HH-7 が見えていないとき、見えている直前の兄弟
// HH-3 を基準にした HH-5 の「上へ移動」で、並びは HH-5・HH-3・HH-4・HH-7・HH-9 になる(見えていない兄弟は動かない)。
func TestReorderLeavesTheInvisibleSiblingsAlone(t *testing.T) {
	svc, rec := setupRec(t)
	st := addSteps(t, svc, "3", "4", "5", "7", "9")
	if err := svc.Reorder(t.Context(), store, st[2].ID, st[0].ID, false); err != nil {
		t.Fatal(err)
	}
	if got := order(t, svc); !slices.Equal(got, []string{"5", "3", "4", "7", "9"}) {
		t.Errorf("order = %v", got)
	}
	// 下へ移動: 見えている直後の兄弟(9)のすぐ後ろ。
	if err := svc.Reorder(t.Context(), store, st[0].ID, st[4].ID, true); err != nil {
		t.Fatal(err)
	}
	if got := order(t, svc); !slices.Equal(got, []string{"5", "4", "7", "9", "3"}) {
		t.Errorf("order = %v", got)
	}
	for _, other := range []item.Item{st[1], st[3], st[4]} {
		if rankRows(t, rec, other.ID) != 0 {
			t.Errorf("%s was written", other.Title)
		}
	}
}

func TestReorderToTheSamePlaceSavesNothing(t *testing.T) {
	svc, rec := setupRec(t)
	st := addSteps(t, svc, "a", "b", "c")
	before := logLen(t, rec, st[1].ID)
	for _, c := range []struct {
		anchor item.ID
		after  bool
	}{{st[2].ID, false}, {st[0].ID, true}} { // b は a の直後で、c の直前
		if err := svc.Reorder(t.Context(), store, st[1].ID, c.anchor, c.after); err != nil {
			t.Errorf("Reorder: %v", err)
		}
	}
	if logLen(t, rec, st[1].ID) != before {
		t.Error("a row was written")
	}
}

// 削除済みの兄弟は、いないものとして数える。a と b の間の削除済みの d は、「すでにその位置」の判定に入らない。
func TestReorderIgnoresDeletedSiblings(t *testing.T) {
	svc, rec := setupRec(t)
	st := addSteps(t, svc, "a", "d", "b")
	op := port.Operation{Actor: change.User("taro"), Summary: "d を削除"}
	err := rec.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(st[1].ID, change.NewDeleteEntity(true))
	})
	if err != nil {
		t.Fatal(err)
	}
	before := logLen(t, rec, st[2].ID)
	if err := svc.Reorder(t.Context(), store, st[2].ID, st[0].ID, true); err != nil || logLen(t, rec, st[2].ID) != before {
		t.Errorf("already after a: %v, log %d → %d", err, before, logLen(t, rec, st[2].ID))
	}
	if err := svc.Reorder(t.Context(), store, st[2].ID, st[1].ID, false); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("a deleted anchor: %v", err)
	}
}

func TestReorderEpicsInTheProject(t *testing.T) {
	svc, _ := setupRec(t)
	second, err := svc.Create(t.Context(), items.CreateInput{Store: store, Project: apptest.SampleProject, Kind: item.Epic, Title: "決済"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Reorder(t.Context(), store, second.ID, apptest.SampleEpic, false); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(t.Context(), store, second.ID)
	first, _ := svc.Get(t.Context(), store, apptest.SampleEpic)
	if err != nil || got.Rank >= first.Rank {
		t.Errorf("second %q must come before %q (%v)", got.Rank, first.Rank, err)
	}
}

func TestReorderRefuses(t *testing.T) {
	svc, rec := setupRec(t)
	st := addSteps(t, svc, "a", "b")
	cases := []struct {
		name       string
		id, anchor item.ID
	}{
		{"itself", st[0].ID, st[0].ID},
		{"an unknown anchor", st[0].ID, "01NOPE00000000000000000000"},
		{"an unknown item", "01NOPE00000000000000000000", st[0].ID},
		{"the parent", st[0].ID, apptest.SampleTask},
		{"a sibling of another parent", st[0].ID, apptest.SampleEpic},
	}
	before := logLen(t, rec, st[0].ID)
	for _, c := range cases {
		if err := svc.Reorder(t.Context(), store, c.id, c.anchor, false); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	if logLen(t, rec, st[0].ID) != before {
		t.Error("a refused reorder wrote a row")
	}
}

// tiedSteps は、Step a(V)・b(W)・c を足し、c の順位を b と同じにする(スプリントに入れたときの rank.After などで生まれる)。
func tiedSteps(t *testing.T) (items.Service, *recorder, []item.Item) {
	t.Helper()
	svc, rec := setupRec(t)
	st := addSteps(t, svc, "a", "b", "c")
	op := port.Operation{Actor: change.User("taro"), Summary: "順位を重ねる"}
	err := rec.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(st[2].ID, change.NewSet("rank", st[2].Rank, st[1].Rank))
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, rec, st
}

// 兄弟と同じ順位があっても動かす。同じ順位の群の中には入らず、群の手前(直後に置くなら後ろ)に入る。
func TestReorderTies(t *testing.T) {
	// c を b(作成が早い)の直前へ: a と b の間に入る。
	svc, rec, st := tiedSteps(t)
	before := rankRows(t, rec, st[2].ID)
	if err := svc.Reorder(t.Context(), store, st[2].ID, st[1].ID, false); err != nil {
		t.Fatal(err)
	}
	if got := order(t, svc); !slices.Equal(got, []string{"a", "c", "b"}) {
		t.Errorf("before b: order = %v", got)
	}
	if rankRows(t, rec, st[2].ID) != before+1 {
		t.Error("before b: c must get one set rank row")
	}
	// a を b の直後へ: 直後の c も b と同じ順位なので、群の後ろに入る。
	svc, rec, st = tiedSteps(t)
	if err := svc.Reorder(t.Context(), store, st[0].ID, st[1].ID, true); err != nil {
		t.Fatal(err)
	}
	if got := order(t, svc); !slices.Equal(got, []string{"b", "c", "a"}) {
		t.Errorf("after b: order = %v", got)
	}
	if rankRows(t, rec, st[0].ID) != 1 || rankRows(t, rec, st[1].ID) != 0 {
		t.Error("after b: only a must be written")
	}
}
