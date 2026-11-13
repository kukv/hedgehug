package planning

import (
	"cmp"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/item"
)

// ScreenOrder は、スプリントの画面の並び。実行中、計画中(開始日の早い順・期間未定は最後)、完了(終了日の新しい順)。同じ日付は名前の昇順。
func ScreenOrder(live []Sprint) []Sprint {
	out := slices.Clone(live)
	slices.SortStableFunc(out, func(a, b Sprint) int {
		if c := cmp.Compare(sprintRank(a.State), sprintRank(b.State)); c != 0 {
			return c
		}
		switch a.State {
		case Planned:
			if c := cmpEmptyLast(a.Start, b.Start); c != 0 {
				return c
			}
		case Closed:
			if c := cmp.Compare(b.End, a.End); c != 0 {
				return c
			}
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return out
}

// PlannedOrder は、計画中のスプリントだけを ScreenOrder の計画中の順にしたもの。
func PlannedOrder(live []Sprint) []Sprint {
	var out []Sprint
	for _, s := range ScreenOrder(live) {
		if s.State == Planned {
			out = append(out, s)
		}
	}
	return out
}

// ReleaseOrder は、リリースの並び。状態(未リリース、リリース済み、アーカイブ)、予定日の早い順(なしは同じ状態の最後)、名前の昇順。
func ReleaseOrder(live []Release) []Release {
	out := slices.Clone(live)
	slices.SortStableFunc(out, func(a, b Release) int {
		if c := cmp.Compare(releaseRank(a.State), releaseRank(b.State)); c != 0 {
			return c
		}
		if c := cmpEmptyLast(a.Date, b.Date); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return out
}

func sprintRank(s SprintState) int {
	switch s {
	case Active:
		return 0
	case Planned:
		return 1
	}
	return 2
}

func releaseRank(s ReleaseState) int {
	switch s {
	case Unreleased:
		return 0
	case Released:
		return 1
	}
	return 2
}

// cmpEmptyLast は、日付の早い順で、空を最後にする比べ。
func cmpEmptyLast(a, b string) int {
	switch {
	case a == b:
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	}
	return cmp.Compare(a, b)
}

// WorkOrder は、アイテムのワークスペース全体の並び。item.SortSiblings(並び順の値、作成の時刻、内部 ID)をそのまま全体に当てる。
func WorkOrder(its []item.Item) []item.Item {
	out := slices.Clone(its)
	item.SortSiblings(out)
	return out
}
