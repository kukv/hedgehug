package planning

import (
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// VelocityBar は、ベロシティの 1 本(スプリントごとの計画と完了)。
type VelocityBar struct {
	Sprint        Sprint
	Planned, Done float64
}

// velocityWindow は、平均に使う完了したスプリントの数。
const velocityWindow = 3

// Velocity は、ベロシティの棒と、完了した直近 3 スプリントの平均(05「ベロシティ」)。
// 棒は削除済みでない実行中と完了を開始日の早い順(同じなら名前の昇順)。平均は完了がなければ nil。
// 完了は確定値、実行中は計画を Confirm の計画、完了を今日の終わり時点の範囲で分類が完了のものから求める。
func Velocity(works []Work, ws workspace.ID, live []Sprint, st workspace.Settings, u Unit, now time.Time) ([]VelocityBar, *float64) {
	var sprints []Sprint
	for _, s := range live {
		if s.Deleted == nil && (s.State == Active || s.State == Closed) {
			sprints = append(sprints, s)
		}
	}
	slices.SortFunc(sprints, func(a, b Sprint) int {
		if a.Start != b.Start {
			return strings.Compare(a.Start, b.Start)
		}
		return strings.Compare(a.Name, b.Name)
	})
	bars := make([]VelocityBar, len(sprints))
	var dones []float64
	for i, s := range sprints {
		bars[i] = VelocityBar{Sprint: s}
		if s.State == Closed {
			bars[i].Planned, bars[i].Done = confirmedBar(s, u)
			dones = append(dones, bars[i].Done)
		} else {
			bars[i].Planned, bars[i].Done = activeBar(works, ws, s, st, u, now)
		}
	}
	return bars, average(dones)
}

// confirmedBar は、完了したスプリントの確定値(Confirmed がなければ 0)。
func confirmedBar(s Sprint, u Unit) (planned, done float64) {
	if s.Confirmed == nil {
		return 0, 0
	}
	if u == UnitCount {
		return float64(s.Confirmed.Committed.Count), float64(s.Confirmed.Completed.Count)
	}
	return s.Confirmed.Committed.Points, s.Confirmed.Completed.Points
}

// activeBar は、実行中のスプリントの計画と完了。
func activeBar(works []Work, ws workspace.ID, s Sprint, st workspace.Settings, u Unit, now time.Time) (planned, done float64) {
	c := Confirm(works, ws, s, st, now)
	planned = c.Committed.Points
	if u == UnitCount {
		planned = float64(c.Committed.Count)
	}
	t := EndOfDay(Today(now), time.Local)
	for _, w := range works {
		if counted(w.Item, ws) && inScope(w, s, t) && Category(st, idAt(w.Log, "status", t)) == workspace.CategoryDone {
			done += Measure(w.Item, u)
		}
	}
	return planned, done
}

// average は、dones(完了のみ・開始日の早い順の完了の値)の最後の 3 件の平均を小数第 1 位にしたもの。空なら nil。
func average(dones []float64) *float64 {
	if len(dones) == 0 {
		return nil
	}
	last := dones[max(0, len(dones)-velocityWindow):]
	var sum float64
	for _, v := range last {
		sum += v
	}
	avg := Round1(sum / float64(len(last)))
	return &avg
}
