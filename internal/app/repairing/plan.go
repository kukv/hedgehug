package repairing

import (
	"context"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// planWorkspace は、修復 (6)〜(8) が読むワークスペース 1 つ。items はそのワークスペースのアイテム(削除済みを含む)とログ。
type planWorkspace struct {
	id       workspace.ID
	prefix   string
	log      []change.Record
	settings workspace.Settings
	items    []repair.ItemLog
}

// readPlanWorkspaces は、すべてのワークスペースを、ログと設定とアイテムつきで読む。
func readPlanWorkspaces(w port.RepairWriter) ([]planWorkspace, error) {
	var out []planWorkspace
	err := w.Read(func(r port.Reader) error {
		wss, err := r.Workspaces().List()
		if err != nil {
			return err
		}
		for _, ws := range wss {
			pw, err := readPlanWorkspace(r, ws)
			if err != nil {
				return err
			}
			out = append(out, pw)
		}
		return nil
	})
	return out, err
}

func readPlanWorkspace(r port.Reader, ws workspace.Workspace) (planWorkspace, error) {
	pw := planWorkspace{id: ws.ID, prefix: ws.Prefix}
	var err error
	if pw.log, err = r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws.ID}); err != nil {
		return pw, err
	}
	if pw.settings, err = r.Workspaces().Settings(ws.ID); err != nil {
		return pw, err
	}
	ps, err := r.Projects().List(ws.ID)
	if err != nil {
		return pw, err
	}
	its, err := itemsOf(r, ps)
	if err != nil {
		return pw, err
	}
	for _, it := range its {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: it.ID})
		if err != nil {
			return pw, err
		}
		pw.items = append(pw.items, repair.ItemLog{Item: it, Log: log})
	}
	return pw, nil
}

func (pw planWorkspace) item(id item.ID) item.Item {
	i := slices.IndexFunc(pw.items, func(il repair.ItemLog) bool { return il.Item.ID == id })
	return pw.items[i].Item
}

// stopExtraActive は、実行中のスプリントが 2 つ以上あるとき、先頭以外を計画中に戻す((6)。決定 C5a-3)。
// スプリント 1 つを 1 回の保存にする。
func stopExtraActive(_ context.Context, w port.RepairWriter) error {
	pws, err := readPlanWorkspaces(w)
	if err != nil {
		return err
	}
	for _, pw := range pws {
		plan := planning.FromLog(pw.log)
		for _, id := range repair.ExtraActive(pw.log) {
			i := slices.IndexFunc(plan.Sprints, func(s planning.Sprint) bool { return s.ID == id })
			old := plan.Sprints[i]
			planned := old
			planned.State = planning.Planned
			op := port.Operation{Actor: syncActor, Summary: "スプリント「" + oneLine(old.Name) + "」を計画中に戻す"}
			err := w.Write(op, func(tx port.Tx) error {
				return tx.WriteWorkspaces().Append(pw.id, planning.SprintDrafts(old, planned)...)
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// backlogClosed は、完了したスプリントに入った未完了の作業項目をバックログに戻す((7)。決定 C5a-4)。
// アイテム 1 つを 1 回の保存にする。
func backlogClosed(_ context.Context, w port.RepairWriter) error {
	pws, err := readPlanWorkspaces(w)
	if err != nil {
		return err
	}
	for _, pw := range pws {
		for _, id := range repair.ClosedSprintItems(pw.log, pw.settings, pw.items, w.Facts()) {
			it := pw.item(id)
			err := writeCleared(w, it, item.SprintField, it.Sprint, it.DisplayID(pw.prefix)+" をバックログに戻す")
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// clearDanglingPlan は、存在しないスプリント・リリースを指すアイテムの欄を空にする((8)。決定 C5a-5)。
// 欄 1 つを 1 回の保存にする。件名は、どれかのワークスペースの計画にあれば「ワークスペースにない」、なければ「完全に削除」。
func clearDanglingPlan(_ context.Context, w port.RepairWriter) error {
	pws, err := readPlanWorkspaces(w)
	if err != nil {
		return err
	}
	var items []item.Item
	plans := map[workspace.ID]planning.Plan{}
	prefixes := map[workspace.ID]string{}
	var all []planning.Plan
	for _, pw := range pws {
		plans[pw.id] = planning.FromLog(pw.log)
		prefixes[pw.id] = pw.prefix
		all = append(all, plans[pw.id])
		for _, il := range pw.items {
			items = append(items, il.Item)
		}
	}
	byID := map[item.ID]item.Item{}
	for _, it := range items {
		byID[it.ID] = it
	}
	for _, ref := range repair.DanglingPlanRefs(items, plans) {
		it := byID[ref.Item]
		summary := danglingSummary(it.DisplayID(prefixes[it.Place.Workspace]), ref, all)
		if err := writeCleared(w, it, ref.Field, ref.Target, summary); err != nil {
			return err
		}
	}
	return nil
}

// danglingSummary は、ref を空にする保存の件名。display はアイテムの画面の ID、plans はすべてのワークスペースの計画。
func danglingSummary(display string, ref repair.PlanRef, plans []planning.Plan) string {
	if ref.Field == item.SprintField {
		reason := "完全に削除"
		if slices.ContainsFunc(plans, func(p planning.Plan) bool {
			return slices.ContainsFunc(p.Sprints, func(s planning.Sprint) bool { return s.ID == ref.Target })
		}) {
			reason = "ワークスペースにない"
		}
		return display + " をバックログに戻す(スプリントが" + reason + ")"
	}
	reason := "完全に削除"
	if slices.ContainsFunc(plans, func(p planning.Plan) bool {
		return slices.ContainsFunc(p.Releases, func(r planning.Release) bool { return r.ID == ref.Target })
	}) {
		reason = "ワークスペースにない"
	}
	return display + " のリリースを外す(リリースが" + reason + ")"
}

// writeCleared は、アイテム it の欄 field(いまの値 from)を空にする行を 1 回保存する。
func writeCleared(w port.RepairWriter, it item.Item, field string, from change.ID, summary string) error {
	op := port.Operation{Actor: syncActor, Summary: summary}
	return w.Write(op, func(tx port.Tx) error {
		return tx.WriteItems().Append(it.ID, change.NewSet(field, from, nil))
	})
}
