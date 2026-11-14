package planning

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Unit は、レポートの数える単位(F3-4)。
type Unit string

// レポートの単位。
const (
	UnitPoints Unit = "points"
	UnitCount  Unit = "count"
)

// Layers は、累積フロー図の層の分け方(F3-12)。
type Layers string

// 累積フロー図の層の分け方。
const (
	LayersCategory Layers = "category"
	LayersStatus   Layers = "status"
)

// Selection は、レポートの選択(F3-5)。Estimate の空は「すべて」。
type Selection struct {
	Unit     Unit
	Sprint   change.ID
	Layers   Layers
	Estimate change.ID
}

// Measure は、アイテム it を単位 u で数えた量。ポイントは未設定を 0、件数は 1。
func Measure(it item.Item, u Unit) float64 {
	if u == UnitCount {
		return 1
	}
	return Amount(it)
}

// ReportSprints は、レポートのスプリントの選択肢(F3-6)。削除済みでない実行中と完了を、開始日の新しい順、同じなら名前の昇順。
func ReportSprints(live []Sprint) []Sprint {
	var out []Sprint
	for _, s := range live {
		if s.Deleted == nil && (s.State == Active || s.State == Closed) {
			out = append(out, s)
		}
	}
	slices.SortFunc(out, func(a, b Sprint) int {
		if a.Start != b.Start {
			return strings.Compare(b.Start, a.Start)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

// ResolveSelection は、保存した選択 saved(なければ found が偽)を、いまのスプリント live で直した選択(F3-5)。
// 読めない単位・層は既定(ポイント・分類)に戻す。2 つのスプリントは別々に確かめる。
func ResolveSelection(saved Selection, found bool, live []Sprint) Selection {
	options := ReportSprints(live)
	sel := Selection{Unit: UnitPoints, Layers: LayersCategory, Sprint: defaultSprint(options)}
	if !found {
		return sel
	}
	if saved.Unit == UnitCount {
		sel.Unit = UnitCount
	}
	if saved.Layers == LayersStatus {
		sel.Layers = LayersStatus
	}
	if slices.ContainsFunc(options, func(s Sprint) bool { return s.ID == saved.Sprint }) {
		sel.Sprint = saved.Sprint
	}
	if slices.ContainsFunc(options, func(s Sprint) bool { return s.ID == saved.Estimate }) {
		sel.Estimate = saved.Estimate
	}
	return sel
}

// defaultSprint は、保存がないときのバーンダウンのスプリント。実行中、なければ終了日が最も遅い完了
// (同じなら開始日が遅いもの、それも同じなら名前の昇順で先)、どちらもなければ空。options は ReportSprints の結果。
func defaultSprint(options []Sprint) change.ID {
	for _, s := range options {
		if s.State == Active {
			return s.ID
		}
	}
	var best *Sprint
	for i := range options {
		if options[i].State == Closed && (best == nil || closedBefore(*best, options[i])) {
			best = &options[i]
		}
	}
	if best == nil {
		return ""
	}
	return best.ID
}

// closedBefore は、完了のスプリント b が a より優先されるか。
func closedBefore(a, b Sprint) bool {
	if a.End != b.End {
		return b.End > a.End
	}
	if a.Start != b.Start {
		return b.Start > a.Start
	}
	return b.Name < a.Name
}

// EffectiveUnit は、単位 u で実際に数える単位と、件数に切り替えたか(F3-4)。its は削除済みを含んでよい。
// ポイントを選んでいても、ワークスペース ws の作業項目にポイントのあるものが 1 件もなければ件数にする。
func EffectiveUnit(its []item.Item, ws workspace.ID, u Unit) (Unit, bool) {
	if u != UnitPoints {
		return UnitCount, false
	}
	for _, it := range its {
		if counted(it, ws) && it.Points != nil {
			return UnitPoints, false
		}
	}
	return UnitCount, true
}

// Round1 は、小数第 2 位を四捨五入して小数第 1 位にしたもの(F3-8)。
// 浮動小数点の誤差(3.55 が 3.5499999…)を消すため、小数第 3 位で一度丸める。
func Round1(x float64) float64 {
	return math.Round(math.Round(x*1000)/100) / 10
}

// CheckPeriod は、累積フロー図の期間(from〜to、today は今日。どれも YYYY-MM-DD)を検査する(F3-11)。
func CheckPeriod(from, to, today string) error {
	switch {
	case from == "":
		return refusal.New("開始日を入力してください")
	case to == "":
		return refusal.New("終了日を入力してください")
	case to < from:
		return refusal.New("終了日は開始日以降にしてください")
	case to > today:
		return refusal.New("終了日は今日以前にしてください")
	}
	return nil
}

// step は、項目の値が変わった点。Created が真の点は、作成の行の時刻とその値。
type step struct {
	At      time.Time
	Value   change.ID
	Created bool
}

// steps は、アイテムのログの項目 f(内部 ID の値)の変遷(F3-13)。作成の行の時刻を最初の点(値は作成の行だけのもの)にして、
// あとの行は At の順(同じなら ID の順)に、各行の At の直後の値(ValueAt)を点にする。同じ値が続く点はまとめる。
// 作成以前の At の行(時計のずれ)は、作成の時刻に寄せた点にする。
// 時計がずれて at が前後しても、change.Resolve が prev で決めた値になる。作成の行がなければ空。
func steps(log []change.Record, f string) []step {
	created := change.Resolve(log, change.Options{}).Created
	if created == nil {
		return nil
	}
	out := []step{{At: created.At, Value: idAt([]change.Record{*created}, f, created.At.Add(time.Nanosecond)), Created: true}}
	rows := slices.Clone(log)
	slices.SortFunc(rows, func(a, b change.Record) int {
		if c := a.At.Compare(b.At); c != 0 {
			return c
		}
		return strings.Compare(string(a.ID), string(b.ID))
	})
	for _, r := range rows {
		if r.ID == created.ID {
			continue
		}
		at := r.At
		if at.Before(created.At) {
			at = created.At
		}
		v := idAt(log, f, at.Add(time.Nanosecond))
		if v != out[len(out)-1].Value {
			out = append(out, step{At: at, Value: v})
		}
	}
	return out
}
