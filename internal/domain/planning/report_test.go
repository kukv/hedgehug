package planning_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

func reportSprints() []planning.Sprint {
	return []planning.Sprint{
		{ID: "A", Name: "A", State: planning.Active, Start: "2026-11-10", End: "2026-11-23"},
		{ID: "B", Name: "B", State: planning.Closed, Start: "2026-10-27", End: "2026-11-09"},
		{ID: "C", Name: "C", State: planning.Closed, Start: "2026-10-28", End: "2026-11-09"},
		{ID: "D", Name: "D", State: planning.Planned, Start: "2026-11-24", End: "2026-12-07"},
		{ID: "E", Name: "E", State: planning.Closed, Start: "2026-10-13", End: "2026-10-26", Deleted: &change.Deletion{}},
	}
}

func TestResolveSelection(t *testing.T) {
	live := reportSprints()
	withoutA := slices.DeleteFunc(slices.Clone(live), func(s planning.Sprint) bool { return s.ID == "A" })
	def := planning.Selection{Unit: planning.UnitPoints, Sprint: "A", Layers: planning.LayersCategory}
	for _, c := range []struct {
		name  string
		saved planning.Selection
		found bool
		live  []planning.Sprint
		want  planning.Selection
	}{
		{"選択なし", planning.Selection{Sprint: "B"}, false, live, def},
		{"保存した Sprint が計画中", planning.Selection{Sprint: "D"}, true, live, def},
		{"保存した Sprint が削除済み", planning.Selection{Sprint: "E"}, true, live, def},
		{"保存した Sprint がない", planning.Selection{Sprint: "Z"}, true, live, def},
		{
			"実行中がなく保存なし",
			planning.Selection{},
			false, withoutA,
			planning.Selection{Unit: planning.UnitPoints, Sprint: "C", Layers: planning.LayersCategory},
		},
		{
			"実行中も完了もない",
			planning.Selection{},
			false,
			[]planning.Sprint{live[3]},
			planning.Selection{Unit: planning.UnitPoints, Layers: planning.LayersCategory},
		},
		{
			"Estimate が削除済み",
			planning.Selection{Sprint: "B", Estimate: "E"},
			true, live,
			planning.Selection{Unit: planning.UnitPoints, Sprint: "B", Layers: planning.LayersCategory},
		},
		{
			"Estimate は残る",
			planning.Selection{Sprint: "Z", Estimate: "B"},
			true, live,
			planning.Selection{Unit: planning.UnitPoints, Sprint: "A", Layers: planning.LayersCategory, Estimate: "B"},
		},
		{"読めない単位と層", planning.Selection{Unit: "hours", Layers: "x"}, true, live, def},
		{
			"件数とステータス",
			planning.Selection{Unit: planning.UnitCount, Layers: planning.LayersStatus},
			true, live,
			planning.Selection{Unit: planning.UnitCount, Sprint: "A", Layers: planning.LayersStatus},
		},
	} {
		if got := planning.ResolveSelection(c.saved, c.found, c.live); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
	// 終了日も開始日も同じなら名前の昇順で先。
	tie := []planning.Sprint{
		{ID: "X", Name: "b", State: planning.Closed, Start: "2026-10-27", End: "2026-11-09"},
		{ID: "Y", Name: "a", State: planning.Closed, Start: "2026-10-27", End: "2026-11-09"},
	}
	if got := planning.ResolveSelection(planning.Selection{}, false, tie).Sprint; got != "Y" {
		t.Errorf("同じなら名前の昇順で先: %q", got)
	}
}

// 同期で実行中が 2 つ届いたときは、開始日の早い方(同じなら名前の昇順で先)。05 の 159 行と同じ。
func TestActiveSprint(t *testing.T) {
	two := []planning.Sprint{
		{ID: "L", Name: "a", State: planning.Active, Start: "2026-11-12"},
		{ID: "P", Name: "p", State: planning.Planned, Start: "2026-11-01"},
		{ID: "E", Name: "b", State: planning.Active, Start: "2026-11-10"},
		{ID: "T", Name: "a", State: planning.Active, Start: "2026-11-10"},
	}
	if got := planning.ActiveSprint(two); got == nil || got.ID != "T" {
		t.Errorf("ActiveSprint = %+v, want T", got)
	}
	if got := planning.ResolveSelection(planning.Selection{}, false, two).Sprint; got != "T" {
		t.Errorf("既定のバーンダウン = %q, want T", got)
	}
	if got := planning.ActiveSprint(two[1:2]); got != nil {
		t.Errorf("実行中がなければ nil: %+v", got)
	}
}

func TestReportSprints(t *testing.T) {
	live := append(reportSprints(), planning.Sprint{ID: "F", Name: "0", State: planning.Closed, Start: "2026-10-27"})
	var got []change.ID
	for _, s := range planning.ReportSprints(live) {
		got = append(got, s.ID)
	}
	if want := []change.ID{"A", "C", "F", "B"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMeasure(t *testing.T) {
	for _, c := range []struct {
		it   item.Item
		u    planning.Unit
		want float64
	}{
		{item.Item{Points: pt(3)}, planning.UnitPoints, 3},
		{item.Item{}, planning.UnitPoints, 0},
		{item.Item{Points: pt(3)}, planning.UnitCount, 1},
	} {
		if got := planning.Measure(c.it, c.u); got != c.want {
			t.Errorf("Measure(%+v, %v) = %v, want %v", c.it, c.u, got, c.want)
		}
	}
}

func TestEffectiveUnit(t *testing.T) {
	story := func(ws item.Place, p *float64) item.Item { return item.Item{Kind: item.Story, Place: ws, Points: p} }
	here, other := item.Place{Workspace: "W"}, item.Place{Workspace: "W2"}
	deleted := story(here, pt(2))
	deleted.Deleted = &change.Deletion{}
	epic := item.Item{Kind: item.Epic, Place: here, Points: pt(21)}
	for _, c := range []struct {
		name     string
		its      []item.Item
		u        planning.Unit
		want     planning.Unit
		fallback bool
	}{
		{"ポイントあり", []item.Item{story(here, nil), story(here, pt(1))}, planning.UnitPoints, planning.UnitPoints, false},
		{"Epic だけ", []item.Item{epic, story(here, nil)}, planning.UnitPoints, planning.UnitCount, true},
		{"削除済みだけ", []item.Item{deleted}, planning.UnitPoints, planning.UnitCount, true},
		{"別のワークスペースだけ", []item.Item{story(other, pt(1))}, planning.UnitPoints, planning.UnitCount, true},
		{"件数を選んでいる", []item.Item{story(here, nil)}, planning.UnitCount, planning.UnitCount, false},
	} {
		got, fb := planning.EffectiveUnit(c.its, "W", c.u)
		if got != c.want || fb != c.fallback {
			t.Errorf("%s: got (%v, %v), want (%v, %v)", c.name, got, fb, c.want, c.fallback)
		}
	}
}

func TestRound1(t *testing.T) {
	// 実行時の積(定数の計算は正確になるので、変数に通す)。3 * 0.35 は 1.0499999999999998 で、x*10 を直接丸めると 1.0 になる。
	third, share := 3.0, 0.35
	for _, c := range []struct{ in, want float64 }{
		{3.55, 3.6}, {6.4, 6.4}, {3 + 0.55*(4-3), 3.6}, {0.05, 0.1}, {2.25, 2.3}, {0, 0}, {third * share, 1.1},
	} {
		if got := planning.Round1(c.in); got != c.want {
			t.Errorf("Round1(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestCheckPeriod(t *testing.T) {
	const today = "2026-11-14"
	for _, c := range []struct{ from, to, msg string }{
		{"", "2026-11-10", "開始日を入力してください"},
		{"", "", "開始日を入力してください"},
		{"2026-11-10", "", "終了日を入力してください"},
		{"2026-11-10", "2026-11-09", "終了日は開始日以降にしてください"},
		{"2026-11-10", "2026-11-15", "終了日は今日以前にしてください"},
		{"2026-11-14", "2026-11-14", ""},
		{"2026-10-25", "2026-11-14", ""},
	} {
		err := planning.CheckPeriod(c.from, c.to, today)
		if c.msg == "" {
			if err != nil {
				t.Errorf("%+v: %v", c, err)
			}
			continue
		}
		var r *refusal.Refusal
		if !errors.As(err, &r) || r.Message != c.msg {
			t.Errorf("%+v: err = %v, want %q", c, err, c.msg)
		}
	}
}

func TestSteps(t *testing.T) {
	values := func(log []change.Record) (vs []change.ID, createdFirst bool) {
		ss := planning.Steps(log, "status")
		for _, s := range ss {
			vs = append(vs, s.Value)
		}
		return vs, len(ss) > 0 && ss[0].Created
	}
	log := created(jst(10, 9, 0), item.Item{Status: "Todo"})
	log = setStatus(log, jst(10, 10, 0), "Todo", "Doing")
	log = setStatus(log, jst(10, 11, 0), "Doing", "Done")
	if vs, first := values(log); !slices.Equal(vs, []change.ID{"Todo", "Doing", "Done"}) || !first {
		t.Fatalf("steps = %v (Created 先頭 %v)", vs, first)
	}
	if got := planning.Steps(log, "status")[0].At; !got.Equal(jst(10, 9, 0)) {
		t.Errorf("最初の点は作成の行の時刻: %v", got)
	}
	// 同じ値が続けば 1 点にまとめる。
	same := setStatus(log, jst(10, 12, 0), "Done", "Done")
	if vs, _ := values(same); !slices.Equal(vs, []change.ID{"Todo", "Doing", "Done"}) {
		t.Errorf("同じ値はまとめる: %v", vs)
	}
	// 時計のずれ: あとの行(Done)の at が前の行(Doing)より前でも、prev でつながるので Done が最後。
	// Doing は点に出ない。11:00 の直後は Done の行だけが見え(prev の Doing の行はまだ at の前)、Done になる。
	// 12:00 の直後は両方の行が見えるが、Done の行の prev が Doing の行を指すので、先端は Done のまま変わらない。
	skew := created(jst(10, 9, 0), item.Item{Status: "Todo"})
	skew = setStatus(skew, jst(10, 12, 0), "Todo", "Doing")
	skew = setStatus(skew, jst(10, 11, 0), "Doing", "Done")
	if vs, _ := values(skew); !slices.Equal(vs, []change.ID{"Todo", "Done"}) {
		t.Errorf("あとの行の値が最後の点: %v", vs)
	}
	// 作成以前の at の行は、作成の時刻に寄せた点になる(作成の点の値は作成の行だけのもの)。
	early := created(jst(10, 9, 0), item.Item{Status: "Doing"})
	early = setStatus(early, jst(10, 8, 0), "Doing", "Done")
	ss := planning.Steps(early, "status")
	if vs, _ := values(early); !slices.Equal(vs, []change.ID{"Doing", "Done"}) || !ss[1].At.Equal(jst(10, 9, 0)) {
		t.Errorf("作成以前の行は作成の時刻の点: %v %+v", vs, ss)
	}
	// 作成の行がなければ空。
	if got := planning.Steps(log[1:], "status"); len(got) != 0 {
		t.Errorf("作成の行がなければ空: %+v", got)
	}
}
