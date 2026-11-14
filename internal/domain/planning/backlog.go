package planning

import (
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Frame は、バックログの画面の枠(F2a-1)。Sprint が nil ならバックログの枠。
// Total は、スプリントの枠は中止を除いた件数とポイント、バックログの枠は全件。
type Frame struct {
	Sprint *Sprint
	Total  Totals
	Items  []item.Item
}

// BacklogFrames は、バックログの画面の枠。実行中・計画中のスプリントを ScreenOrder の順に、最後にバックログの枠(F2a-1)。
// 各枠のアイテムはワークスペースの並び順(WorkOrder)。
func BacklogFrames(its []item.Item, ws workspace.ID, plan Plan, st workspace.Settings) []Frame {
	works := workItems(its, ws, "")
	var frames []Frame
	for _, s := range ScreenOrder(plan.LiveSprints()) {
		if s.State == Closed {
			continue
		}
		f := Frame{Sprint: &s}
		for _, it := range works {
			if it.Sprint != s.ID {
				continue
			}
			f.Items = append(f.Items, it)
			if Category(st, it.Status) != workspace.CategoryCanceled {
				f.Total.add(it)
			}
		}
		frames = append(frames, f)
	}
	var backlog Frame
	for _, it := range works {
		if inFrame(it, "", plan, st) {
			backlog.Items = append(backlog.Items, it)
			backlog.Total.add(it)
		}
	}
	return append(frames, backlog)
}

// workItems は、ワークスペース ws の削除されていない作業項目(except を除く)を WorkOrder の順に並べたもの。
func workItems(its []item.Item, ws workspace.ID, except item.ID) []item.Item {
	var works []item.Item
	for _, it := range its {
		if counted(it, ws) && it.ID != except {
			works = append(works, it)
		}
	}
	return WorkOrder(works)
}

// WorkEnd は、ワークスペースの削除されていない作業項目の最大の順位。なければ空。
func WorkEnd(its []item.Item, ws workspace.ID) string {
	end := ""
	for _, it := range its {
		if counted(it, ws) && it.Rank > end {
			end = it.Rank
		}
	}
	return end
}

// EndRank は、新しく並びに入る kind のアイテムの順位(F2a-5)。
// 作業項目は親の子の末尾 siblingEnd とワークスペースの作業項目の末尾の大きい方の後ろ、それ以外は親の子の末尾の後ろ。
func EndRank(siblingEnd string, its []item.Item, ws workspace.ID, kind item.Kind) string {
	if IsWork(kind) {
		return rank.After(max(siblingEnd, WorkEnd(its, ws)))
	}
	return rank.After(siblingEnd)
}

// Placement は、anchor の直前(after なら直後)に moving を置く順位(F2a-2)。
// ワークスペースの作業項目の並び(moving は除く)の中で、同じ順位の群の中には入らず、群の手前(after なら後ろ)に入る。
// anchor が並びにない(作業項目でない・削除済み・別のワークスペース・moving 自身)なら false。
func Placement(its []item.Item, ws workspace.ID, moving, anchor item.ID, after bool) (string, bool) {
	works := workItems(its, ws, moving)
	at := -1
	for i, it := range works {
		if it.ID == anchor {
			at = i
		}
	}
	if at < 0 {
		return "", false
	}
	r := works[at].Rank
	if after {
		for _, it := range works[at+1:] {
			if it.Rank > r {
				return rank.Between(r, it.Rank), true
			}
		}
		return rank.After(r), true
	}
	for i := at - 1; i >= 0; i-- {
		if works[i].Rank < r {
			return rank.Between(works[i].Rank, r), true
		}
	}
	return rank.Between("", r), true
}

// AppendWorks は、moved の作業項目を WorkOrder の順に、start から rank.After を続けた順位(F2a-6)。作業項目でないものは入らない。
func AppendWorks(moved []item.Item, start string) map[item.ID]string {
	var works []item.Item
	for _, it := range moved {
		if IsWork(it.Kind) {
			works = append(works, it)
		}
	}
	out := map[item.ID]string{}
	r := start
	for _, it := range WorkOrder(works) {
		r = rank.After(r)
		out[it.ID] = r
	}
	return out
}
