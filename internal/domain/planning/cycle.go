package planning

import (
	"math"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Durations は、サイクルタイムまたはリードタイムの集計(F3-21)。N は数えた件数、Median・P85 は日数(Round1 済み、0 件なら nil)。
// Skipped は、サイクルタイムで着手の時刻がなくて数えなかった件数(リードタイムでは 0)。
type Durations struct {
	N       int
	Median  *float64
	P85     *float64
	Skipped int
}

// Cycle は、いまの分類が完了の作業項目のサイクルタイムとリードタイム(F3-21、05「サイクルタイムとリードタイム」)。
func Cycle(works []Work, ws workspace.ID, st workspace.Settings) (cycle, lead Durations) {
	var cycles, leads []float64
	skipped := 0
	for _, w := range works {
		if !counted(w.Item, ws) || Category(st, w.Item.Status) != workspace.CategoryDone {
			continue
		}
		ss := steps(w.Log, "status")
		reach := reachedDone(ss, st)
		if reach < 0 {
			continue
		}
		leads = append(leads, days(ss[reach].At.Sub(ss[0].At)))
		if k := startedDoing(ss, reach, st); k >= 0 {
			cycles = append(cycles, days(ss[reach].At.Sub(ss[k].At)))
		} else {
			skipped++
		}
	}
	return summarize(cycles, skipped), summarize(leads, 0)
}

// days は、24 時間を 1 日とした日数。時計のずれで負になったときは 0。
func days(d time.Duration) float64 { return math.Max(0, d.Seconds()/86400) }

// reachedDone は、分類が完了以外から完了へ変わった最後の点の位置。なければ -1(作成のときから完了のものを含む)。
func reachedDone(ss []step, st workspace.Settings) int {
	for i := len(ss) - 1; i >= 1; i-- {
		if isDone(st, ss[i]) && !isDone(st, ss[i-1]) {
			return i
		}
	}
	return -1
}

// startedDoing は、到達の点 reach より前の区間で、最初に分類が進行中である点の位置。なければ -1。
// 区間の始まりは、完了から完了以外へ変わった最後の点(なければ作成の点)。
func startedDoing(ss []step, reach int, st workspace.Settings) int {
	from := 0
	for j := reach - 1; j >= 1; j-- {
		if !isDone(st, ss[j]) && isDone(st, ss[j-1]) {
			from = j
			break
		}
	}
	for k := from; k < reach; k++ {
		if Category(st, ss[k].Value) == workspace.CategoryDoing {
			return k
		}
	}
	return -1
}

func isDone(st workspace.Settings, s step) bool {
	return Category(st, s.Value) == workspace.CategoryDone
}

// summarize は、日数 ds の件数・中央値・85% の値(どちらも Round1 済み)。
func summarize(ds []float64, skipped int) Durations {
	out := Durations{N: len(ds), Skipped: skipped}
	if len(ds) == 0 {
		return out
	}
	slices.Sort(ds)
	median, p85 := Round1(percentile(ds, 0.5)), Round1(percentile(ds, 0.85))
	out.Median, out.P85 = &median, &p85
	return out
}

// percentile は、昇順の ds の位置 (N-1)*p の値を、前後 2 つの線形補間で求めたもの。
func percentile(ds []float64, p float64) float64 {
	pos := float64(len(ds)-1) * p
	lo := int(math.Floor(pos))
	if lo+1 >= len(ds) {
		return ds[lo]
	}
	return ds[lo] + (pos-float64(lo))*(ds[lo+1]-ds[lo])
}
