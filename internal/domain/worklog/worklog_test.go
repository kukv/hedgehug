package worklog_test

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

func at(minute int) time.Time { return time.Date(2026, 11, 13, 10, minute, 0, 0, time.UTC) }

func row(id, f string, minute int, by string, from, to any, prev ...change.ID) change.Record {
	d := change.NewWorklog(change.ID(f), from, to)
	return change.Record{ID: change.ID(id), Op: change.ID(id), At: at(minute), By: by, Actor: change.User(by), T: d.T, F: d.F, From: d.From, To: d.To, Prev: prev}
}

func TestFromLogOrdersByRecordAndLeavesOutDeleted(t *testing.T) {
	log := []change.Record{
		row("w1", "W1", 1, "hanako", nil, worklog.Value(125, "下書き", "2026-11-13"), "c"),
		row("w2", "W2", 2, "taro", nil, worklog.Value(30, "", "2026-11-13"), "c"),
		row("w3", "W3", 3, "taro", nil, worklog.Value(100, "消す", "2026-11-13"), "c"),
		row("w3d", "W3", 4, "hanako", worklog.Value(100, "消す", "2026-11-13"), nil, "w3"),
	}
	want := []worklog.Entry{
		{ID: "W1", By: "hanako", Date: "2026-11-13", Hundredths: 125, Memo: "下書き"},
		{ID: "W2", By: "taro", Date: "2026-11-13", Hundredths: 30},
	}
	got := worklog.FromLog(log)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FromLog = %+v\nwant %+v", got, want)
	}
	rev := slices.Clone(log)
	slices.Reverse(rev)
	if !reflect.DeepEqual(worklog.FromLog(rev), want) {
		t.Error("FromLog depends on the order of the rows")
	}
	if e, ok := worklog.Find(log, "W2"); !ok || e.Hundredths != 30 {
		t.Errorf("Find W2 = %+v, %v", e, ok)
	}
	if _, ok := worklog.Find(log, "W3"); ok {
		t.Error("Find of a deleted record")
	}
}

// 時間は JSON の数で書くので、100 分の 1 時間の整数に戻すとき丸める。0.29・0.07・1.1 のような 2 進で表せない値も戻る。
func TestHoursRoundTripThroughTheLog(t *testing.T) {
	for h := 1; h <= 20000; h++ {
		log := []change.Record{row("w", "W1", 1, "taro", nil, worklog.Value(h, "", "2026-11-13"), "c")}
		got := worklog.FromLog(log)
		if len(got) != 1 || got[0].Hundredths != h {
			t.Fatalf("%d hundredths came back as %+v", h, got)
		}
	}
	for _, h := range []int{29, 7, 110, 115, 99999999900} {
		log := []change.Record{row("w", "W1", 1, "taro", nil, worklog.Value(h, "", "2026-11-13"), "c")}
		if got := worklog.FromLog(log); len(got) != 1 || got[0].Hundredths != h {
			t.Errorf("%d hundredths came back as %+v", h, got)
		}
	}
}

func TestFromLogSkipsUnreadableRows(t *testing.T) {
	log := []change.Record{
		row("w1", "W1", 1, "taro", nil, "形が違う", "c"),
		row("w2", "W2", 2, "taro", nil, map[string]any{"hours": 0, "memo": "", "date": "2026-11-13"}, "c"),
	}
	if got := worklog.FromLog(log); len(got) != 0 {
		t.Errorf("FromLog = %+v, want none", got)
	}
}
