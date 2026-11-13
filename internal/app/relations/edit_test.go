package relations_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
)

func TestAddWritesToTheSideThatHoldsTheRelation(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	// blocks: 自分の側に持つ。
	if err := svc.Add(ctx, store, apptest.SampleEpic, relation.Blocks, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if got := relationsOf(t, rec, apptest.SampleEpic); !slices.Equal(got, []item.Relation{{Type: "blocks", To: apptest.SampleTask}}) || rec.last() != "HH-1 blocks HH-2" {
		t.Errorf("blocks: %+v, %q", got, rec.last())
	}
	// blocked by: 相手がこのアイテムをブロックする関連を、相手のログに足す。
	if err := svc.Add(ctx, store, apptest.SampleTask, relation.BlockedBy, itDone); err != nil {
		t.Fatal(err)
	}
	if got := relationsOf(t, rec, itDone); !slices.Equal(got, []item.Relation{{Type: "blocks", To: apptest.SampleTask}}) || rec.last() != "HH-2 blocked by HH-3" {
		t.Errorf("blocked by: %+v, %q", got, rec.last())
	}
	if got := relationsOf(t, rec, apptest.SampleTask); len(got) != 0 {
		t.Errorf("blocked by は自分の側に書かない: %+v", got)
	}
	// relates to は足した側に置く。ワークスペースをまたぐ相手でもよい。
	if err := svc.Add(ctx, store, apptest.SampleEpic, relation.RelatesTo, itMobile); err != nil {
		t.Fatal(err)
	}
	if got := relationsOf(t, rec, apptest.SampleEpic); !slices.Contains(got, item.Relation{Type: "relates", To: itMobile}) || rec.last() != "HH-1 relates to MB-1" {
		t.Errorf("relates: %+v, %q", got, rec.last())
	}
	// Step も、Step 以外のアイテムとの関連を持てる。
	if err := svc.Add(ctx, store, itStep, relation.Causes, apptest.SampleEpic); err != nil {
		t.Errorf("Step から: %v", err)
	}
}

func TestAddRefuses(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	if err := svc.Add(ctx, store, apptest.SampleEpic, relation.Blocks, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := svc.Add(ctx, store, apptest.SampleTask, relation.Blocks, itDone); err != nil {
		t.Fatal(err)
	}
	before := logLen(t, rec, apptest.SampleEpic) + logLen(t, rec, apptest.SampleTask) + logLen(t, rec, itDone)
	cases := []struct {
		name  string
		self  item.ID
		label relation.Label
		other item.ID
		want  string
	}{
		{"向きが逆でも同じ種類は 1 つ", apptest.SampleTask, relation.BlockedBy, apptest.SampleEpic, "すでに blocked by の関連があります"},
		{"同じ向きの重複", apptest.SampleEpic, relation.Blocks, apptest.SampleTask, "すでに blocks の関連があります"},
		{"循環", itDone, relation.Blocks, apptest.SampleEpic, "循環になります"},
	}
	for _, c := range cases {
		err := svc.Add(ctx, store, c.self, c.label, c.other)
		if message(err) != c.want {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
	}
	for name, err := range map[string]error{
		"自分自身":   svc.Add(ctx, store, apptest.SampleEpic, relation.Causes, apptest.SampleEpic),
		"相手が Step": svc.Add(ctx, store, apptest.SampleEpic, relation.Causes, itStep),
		"ないアイテム": svc.Add(ctx, store, apptest.SampleEpic, relation.Causes, "01NOPE00000000000000000000"),
		"知らない表記": svc.Add(ctx, store, apptest.SampleEpic, "friends with", itDone),
	} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if after := logLen(t, rec, apptest.SampleEpic) + logLen(t, rec, apptest.SampleTask) + logLen(t, rec, itDone); after != before {
		t.Errorf("断ったときは何も保存しない: ログの行 %d → %d", before, after)
	}
}

func TestRemove(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	if err := svc.Add(ctx, store, apptest.SampleEpic, relation.Blocks, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	// 持たれる側の表記で外すと、持っている側のログに remove を書く。
	if err := svc.Remove(ctx, store, apptest.SampleTask, relation.BlockedBy, apptest.SampleEpic); err != nil {
		t.Fatal(err)
	}
	if got := relationsOf(t, rec, apptest.SampleEpic); len(got) != 0 || rec.last() != "HH-2 と HH-1 の blocked by を解除" {
		t.Errorf("remove: %+v, %q", got, rec.last())
	}
	// 持っていなければ何も保存しない。
	n := logLen(t, rec, apptest.SampleEpic) + logLen(t, rec, apptest.SampleTask)
	if err := svc.Remove(ctx, store, apptest.SampleTask, relation.BlockedBy, apptest.SampleEpic); err != nil {
		t.Errorf("もう外れている関連を外す: %v", err)
	}
	if after := logLen(t, rec, apptest.SampleEpic) + logLen(t, rec, apptest.SampleTask); after != n {
		t.Errorf("外れている関連を外しても何も保存しない: ログの行 %d → %d", n, after)
	}
	// 向きが違う表記では外れない(blocks の持ち主は逆)。
	if err := svc.Add(ctx, store, apptest.SampleEpic, relation.Blocks, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	n = logLen(t, rec, apptest.SampleEpic)
	if err := svc.Remove(ctx, store, apptest.SampleTask, relation.Blocks, apptest.SampleEpic); err != nil {
		t.Fatal(err)
	}
	if after := logLen(t, rec, apptest.SampleEpic); after != n || len(relationsOf(t, rec, apptest.SampleEpic)) != 1 {
		t.Errorf("HH-2 が blocks している関連はないので何も外れない: ログの行 %d → %d", n, after)
	}
	// relates to は、どちらの側が持っていても外れる。
	if err := svc.Add(ctx, store, apptest.SampleEpic, relation.RelatesTo, itMobile); err != nil {
		t.Fatal(err)
	}
	if err := svc.Remove(ctx, store, itMobile, relation.RelatesTo, apptest.SampleEpic); err != nil {
		t.Fatal(err)
	}
	if got := relationsOf(t, rec, apptest.SampleEpic); slices.Contains(got, item.Relation{Type: "relates", To: itMobile}) || rec.last() != "MB-1 と HH-1 の relates to を解除" {
		t.Errorf("relates の解除: %+v, %q", got, rec.last())
	}
	if err := svc.Remove(ctx, store, apptest.SampleEpic, relation.Blocks, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("ないアイテム: %v", err)
	}
}
