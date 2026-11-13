package planning_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// tokyo は、テストのこのマシンの時刻帯(Asia/Tokyo。tzdata に頼らないよう固定の差で作る)。
var tokyo = time.FixedZone("Asia/Tokyo", 9*60*60)

// jst は、2026 年 11 月 day 日 hour:min(Asia/Tokyo)。Today が now.Local() を使うので、now には
// UTC と日付が同じになる 9 時以降を使う。
func jst(day, hour, minute int) time.Time { return time.Date(2026, 11, day, hour, minute, 0, 0, tokyo) }

// stampAt は、drafts を時刻 when の op の行としてログの続きにする(テスト用)。
func stampAt(log []change.Record, when time.Time, ds ...change.Draft) []change.Record {
	n := len(log)
	meta := change.Meta{Op: change.ID("op" + strconv.Itoa(n)), At: when, Actor: change.User("taro")}
	return append(log, change.Stamp(log, ds, meta, func() change.ID {
		n++
		return change.ID("r" + strconv.Itoa(n))
	})...)
}

// created は、時刻 when に作ったアイテムのログ。
func created(when time.Time, it item.Item) []change.Record {
	if it.Place.Workspace == "" {
		it.Place.Workspace = "W"
	}
	if it.Kind == "" {
		it.Kind = item.Story
	}
	return stampAt(nil, when, item.Create(it))
}

func setSprint(log []change.Record, when time.Time, from, to any) []change.Record {
	return stampAt(log, when, change.NewSet(item.SprintField, from, to))
}

func setStatus(log []change.Record, when time.Time, from, to change.ID) []change.Record {
	return stampAt(log, when, change.NewSet("status", from, to))
}

func work(t *testing.T, log []change.Record) planning.Work {
	t.Helper()
	it, ok := item.FromLog(log)
	if !ok {
		t.Fatalf("作成の行がない: %+v", log)
	}
	return planning.Work{Item: it, Log: log}
}

func pt(v float64) *float64 { return &v }

func settings() workspace.Settings {
	return workspace.Settings{Statuses: []workspace.Value{
		{ID: "Todo", Category: workspace.CategoryTodo},
		{ID: "Doing", Category: workspace.CategoryDoing},
		{ID: "Done", Category: workspace.CategoryDone},
		{ID: "Canceled", Category: workspace.CategoryCanceled},
	}}
}

func TestCategory(t *testing.T) {
	st := settings()
	for id, want := range map[change.ID]workspace.Category{
		"Doing": workspace.CategoryDoing, "Canceled": workspace.CategoryCanceled,
		"Gone": workspace.CategoryTodo, "": workspace.CategoryTodo,
	} {
		if got := planning.Category(st, id); got != want {
			t.Errorf("Category(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestIsWork(t *testing.T) {
	for k, want := range map[item.Kind]bool{
		item.Story: true, item.Task: true, item.Bug: true, item.Epic: false, item.Step: false,
	} {
		if planning.IsWork(k) != want {
			t.Errorf("IsWork(%s) = %v", k, !want)
		}
	}
}

func TestEndOfDay(t *testing.T) {
	if got, want := planning.EndOfDay("2026-11-02", tokyo), jst(3, 0, 0); !got.Equal(want) {
		t.Fatalf("EndOfDay = %v, want %v", got, want)
	}
	if got := planning.EndOfDay("2026-11-30", tokyo); !got.Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, tokyo)) {
		t.Fatalf("月末の EndOfDay = %v", got)
	}
	if got := planning.EndOfDay("", tokyo); !got.IsZero() {
		t.Fatalf("読めない日付は時刻ゼロ: %v", got)
	}
}

func TestAmount(t *testing.T) {
	if got := planning.Amount(item.Item{Points: pt(2.5)}); got != 2.5 {
		t.Fatalf("Amount = %v", got)
	}
	if got := planning.Amount(item.Item{}); got != 0 {
		t.Fatalf("ポイントなしは 0: %v", got)
	}
}

func TestValueAt(t *testing.T) {
	log := created(jst(1, 10, 0), item.Item{ID: "I1", Status: "Todo"})
	log = setStatus(log, jst(2, 23, 0), "Todo", "Doing")
	if _, ok := planning.ValueAt(log, "status", jst(1, 10, 0)); ok {
		t.Fatal("作成と同じ時刻は、作成の前(at が t より前の行だけ)")
	}
	if v, ok := planning.ValueAt(log, "status", jst(1, 10, 1)); !ok || string(v) != `"Todo"` {
		t.Fatalf("作成の値: %s %v", v, ok)
	}
	if v, ok := planning.ValueAt(log, "status", jst(3, 0, 0)); !ok || string(v) != `"Doing"` {
		t.Fatalf("変更のあとの値: %s %v", v, ok)
	}
	if v, ok := planning.ValueAt(log, item.SprintField, jst(3, 0, 0)); !ok || v != nil {
		t.Fatalf("値のない項目は nil で作成はある: %s %v", v, ok)
	}
}

func TestValueAtOrder(t *testing.T) {
	log := created(jst(1, 10, 0), item.Item{ID: "I1", Status: "Todo"})
	log = setStatus(log, jst(2, 12, 0), "Todo", "Doing")
	log = setStatus(log, jst(2, 11, 0), "Doing", "Done") // 時計がずれて、あとの行の at が前
	if v, ok := planning.ValueAt(log, "status", jst(3, 0, 0)); !ok || string(v) != `"Done"` {
		t.Fatalf("prev でつながったあとの行の値: %s %v", v, ok)
	}
}

// confirmWorks は、TestConfirmCommitted の (a)〜(g) と、計画にも実績にも入る 4 pt のアイテム。
func confirmWorks(t *testing.T) []planning.Work {
	t.Helper()
	day1 := jst(1, 10, 0)
	a := created(day1, item.Item{ID: "A", Sprint: "S1", Status: "Doing", Points: pt(3)})
	b := created(day1, item.Item{ID: "B", Status: "Todo", Points: pt(2)})
	b = setSprint(b, jst(3, 10, 0), nil, "S1")
	b = setStatus(b, jst(10, 10, 0), "Todo", "Done")
	c := created(day1, item.Item{ID: "C", Sprint: "S1", Status: "Todo", Points: pt(5)})
	c = setSprint(c, jst(2, 23, 0), "S1", nil)
	c = setSprint(c, jst(5, 10, 0), nil, "S1")
	d := created(day1, item.Item{ID: "D", Sprint: "S1", Status: "Todo", Points: pt(1)})
	d = setStatus(d, jst(2, 20, 0), "Todo", "Canceled")
	e := created(day1, item.Item{ID: "E", Sprint: "S1", Status: "Done", Points: pt(8)})
	e = stampAt(e, jst(4, 10, 0), change.NewDeleteEntity(true))
	f := created(day1, item.Item{ID: "F", Place: item.Place{Workspace: "W2"}, Sprint: "S1", Status: "Done", Points: pt(13)})
	g := created(day1, item.Item{ID: "G", Sprint: "S1", Status: "Todo"})
	i := created(jst(2, 10, 0), item.Item{ID: "I", Sprint: "S1", Status: "Done", Points: pt(4)})
	epic := created(day1, item.Item{ID: "P", Kind: item.Epic, Sprint: "S1", Status: "Done", Points: pt(21)})
	var out []planning.Work
	for _, log := range [][]change.Record{a, b, c, d, e, f, g, i, epic} {
		out = append(out, work(t, log))
	}
	return out
}

func TestConfirmCommitted(t *testing.T) {
	got := planning.Confirm(confirmWorks(t), "W", sprint1(), settings(), jst(15, 15, 0))
	// 計画は (a) 3 pt・(g) 0 pt・2 日に足した 4 pt。(b)(c)(d)(e)(f) と Epic は入らない。
	if want := (planning.Totals{Points: 3 + 0 + 4, Count: 3}); got.Committed != want {
		t.Fatalf("Committed = %+v, want %+v", got.Committed, want)
	}
	// 実績は (b) 2 pt と 4 pt。
	if want := (planning.Totals{Points: 2 + 4, Count: 2}); got.Completed != want {
		t.Fatalf("Completed = %+v, want %+v", got.Completed, want)
	}
}

func TestConfirmCommittedStartAfterToday(t *testing.T) {
	// (h) 開始日が明日(同期で届いた場合)は、今日の終わりで数える。
	s := sprint1()
	s.Start = "2026-11-16"
	now := jst(15, 15, 0)
	before := created(jst(15, 10, 0), item.Item{ID: "A", Sprint: "S1", Status: "Todo", Points: pt(3)})
	// 時計の進んだマシンが明日の朝に入れた行。開始日の終わりなら入るが、今日の終わりより後。
	ahead := created(jst(15, 10, 0), item.Item{ID: "B", Status: "Todo", Points: pt(2)})
	ahead = setSprint(ahead, jst(16, 9, 0), nil, "S1")
	got := planning.Confirm([]planning.Work{work(t, before), work(t, ahead)}, "W", s, settings(), now)
	if want := (planning.Totals{Points: 3, Count: 1}); got.Committed != want {
		t.Fatalf("Committed = %+v, want %+v", got.Committed, want)
	}
}

func TestConfirmCompleted(t *testing.T) {
	day1 := jst(1, 10, 0)
	done := created(day1, item.Item{ID: "A", Sprint: "S1", Status: "Done", Points: pt(3)})
	canceled := created(day1, item.Item{ID: "B", Sprint: "S1", Status: "Canceled", Points: pt(2)})
	unknown := created(day1, item.Item{ID: "C", Sprint: "S1", Status: "Gone", Points: pt(5)})
	moved := created(day1, item.Item{ID: "D", Sprint: "S1", Status: "Done", Points: pt(1)})
	moved = setSprint(moved, jst(14, 10, 0), "S1", "S2")
	doing := created(day1, item.Item{ID: "E", Sprint: "S1", Status: "Doing", Points: pt(8)})
	var works []planning.Work
	for _, log := range [][]change.Record{done, canceled, unknown, moved, doing} {
		works = append(works, work(t, log))
	}
	got := planning.Confirm(works, "W", sprint1(), settings(), jst(15, 15, 0))
	if want := (planning.Totals{Points: 3, Count: 1}); got.Completed != want {
		t.Fatalf("Completed = %+v, want %+v", got.Completed, want)
	}
	// 計画は中止を除く 4 件(設定にないステータスは未着手として数える)。
	if want := (planning.Totals{Points: 3 + 5 + 1 + 8, Count: 4}); got.Committed != want {
		t.Fatalf("Committed = %+v, want %+v", got.Committed, want)
	}
}

func TestValueAtNoCreate(t *testing.T) {
	log := created(jst(5, 10, 0), item.Item{ID: "I1", Sprint: "S1"})
	if v, ok := planning.ValueAt(log, item.SprintField, jst(3, 0, 0)); ok || v != nil {
		t.Fatalf("作成前は false: %s %v", v, ok)
	}
	if _, ok := planning.ValueAt(nil, item.SprintField, jst(3, 0, 0)); ok {
		t.Fatal("空のログは false")
	}
}
