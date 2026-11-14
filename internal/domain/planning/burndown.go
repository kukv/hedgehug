package planning

import (
	"strconv"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// BurnDay は、バーンダウンの 1 日。Remaining は表示する日より後で nil(F3-9: Total はその日の終わり時点の値を出す)。
type BurnDay struct {
	Date      string
	Total     float64
	Remaining *float64
	Ideal     float64
}

// Burndown は、スプリントのバーンダウン。Days は開始日から終了日のすべての日。Shown は表示する日の数(05「表示する日」)。
// Committed は、完了したスプリントで総量(開始日)が確定の計画と違うときだけ、その確定の計画(単位に合わせた値)。
type Burndown struct {
	Days      []BurnDay
	Shown     int
	Committed *float64
}

// Burn は、スプリント s のバーンダウン(05「バーンダウン」)。works は削除済みを含んでよい。
// u は実際に数える単位(件数への切り替えは呼び出し側が EffectiveUnit で決める)。今日は now の、このマシンの時刻帯の日。
func Burn(works []Work, ws workspace.ID, s Sprint, st workspace.Settings, u Unit, now time.Time) Burndown {
	var b Burndown
	for _, d := range daysBetween(s.Start, s.End) {
		b.Days = append(b.Days, BurnDay{Date: d})
	}
	last := lastShown(s, now)
	for i := range b.Days {
		day := &b.Days[i]
		var remaining float64
		day.Total, remaining = dayValues(works, ws, s, st, u, day.Date)
		if day.Date <= last {
			b.Shown++
			day.Remaining = &remaining
		}
	}
	fillIdeal(b.Days)
	b.Committed = committedNote(b.Days, s, u)
	return b
}

// dayValues は、日 date の総量と残り(05「日ごとの値」)。
func dayValues(works []Work, ws workspace.ID, s Sprint, st workspace.Settings, u Unit, date string) (total, remaining float64) {
	t := EndOfDay(date, time.Local)
	for _, w := range works {
		if !counted(w.Item, ws) || !inScope(w, s, t) {
			continue
		}
		c := Category(st, idAt(w.Log, "status", t))
		if c == workspace.CategoryCanceled {
			continue
		}
		total += Measure(w.Item, u)
		if c == workspace.CategoryTodo || c == workspace.CategoryDoing {
			remaining += Measure(w.Item, u)
		}
	}
	return total, remaining
}

// Added は、スプリント s の途中で追加した量(05「途中で追加」)。表示する最後の日の終わり時点の範囲のうち、
// 分類が中止でない作業項目で、開始日の終わり時点の範囲になかったものの量の合計。表示する日が 0 日なら 0。
func Added(works []Work, ws workspace.ID, s Sprint, st workspace.Settings, u Unit, now time.Time) float64 {
	last := lastShown(s, now)
	if last < s.Start {
		return 0
	}
	start, end := EndOfDay(s.Start, time.Local), EndOfDay(last, time.Local)
	var sum float64
	for _, w := range works {
		if counted(w.Item, ws) && inScope(w, s, end) && !inScope(w, s, start) &&
			Category(st, idAt(w.Log, "status", end)) != workspace.CategoryCanceled {
			sum += Measure(w.Item, u)
		}
	}
	return sum
}

// Progress は、スプリント sprint の進捗(05「進捗」)。its のうち、ワークスペース ws の削除済みでない作業項目で
// 分類が中止でないものを数え、完了の分類のものを done にする。
func Progress(its []item.Item, ws workspace.ID, sprint change.ID, st workspace.Settings) (done, total Totals) {
	for _, it := range its {
		if !counted(it, ws) || it.Sprint != sprint {
			continue
		}
		switch Category(st, it.Status) {
		case workspace.CategoryCanceled:
		case workspace.CategoryDone:
			done.add(it)
			total.add(it)
		default:
			total.add(it)
		}
	}
	return done, total
}

// lastShown は、表示する最後の日。終了日・今日・(完了していれば)完了した日のうち最も早い日(開始日より前なら 0 日)。
func lastShown(s Sprint, now time.Time) string {
	last := min(s.End, Today(now))
	if s.Closing != nil {
		last = min(last, Today(s.Closing.At))
	}
	return last
}

// inScope は、時点 t(日の終わり)に、アイテム w がスプリント s に入っていたか。
// t が完了した日の終わり以前で、完了で移した行(完了の記録と同じ op の set sprint。F3-3)があれば、
// t とその行の時刻の早い方の時点の値で決める。完了による持ち越しを、完了した日の終わりまで範囲に残すため
// (それより後の日には残さない)。同じ日にさらに別の枠へ移した行も、その行より前の行だけを見るので影響しない。
func inScope(w Work, s Sprint, t time.Time) bool {
	if s.Closing != nil && !t.After(EndOfDay(Today(s.Closing.At), time.Local)) {
		for _, r := range w.Log {
			if r.Op == s.Closing.Op && r.T == change.Set && r.F == item.SprintField && r.At.Before(t) {
				t = r.At
			}
		}
	}
	return idAt(w.Log, item.SprintField, t) == s.ID
}

// fillIdeal は、理想の線(開始日の総量から終了日に 0 へ。途中の増減では引き直さない)。
// 値は Round1 で小数第 1 位にする(F3-22。表の値と図の点を同じ値にする)。
func fillIdeal(days []BurnDay) {
	if len(days) == 0 {
		return
	}
	first := days[0].Total
	if len(days) == 1 {
		days[0].Ideal = Round1(first)
		return
	}
	for k := range days {
		days[k].Ideal = Round1(first - first*float64(k)/float64(len(days)-1))
	}
}

// committedNote は、完了したスプリントの確定の計画のうち、総量(開始日)と違うときのもの。
// 画面の pt と同じ 12 桁の文字列で比べ、浮動小数点の誤差で注記を出さない。
func committedNote(days []BurnDay, s Sprint, u Unit) *float64 {
	if s.State != Closed || s.Confirmed == nil || len(days) == 0 {
		return nil
	}
	v := s.Confirmed.Committed.Points
	if u == UnitCount {
		v = float64(s.Confirmed.Committed.Count)
	}
	if digits(v) == digits(days[0].Total) {
		return nil
	}
	return &v
}

func digits(v float64) string { return strconv.FormatFloat(v, 'g', 12, 64) }
