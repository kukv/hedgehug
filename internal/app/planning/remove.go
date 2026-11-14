package planning

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// DeleteSprint は、スプリント id を削除済みにする(F1-13)。1 回の保存で、入っている削除済みでない
// アイテム(完了・中止を含む)からスプリントを外す。rank は変えない。実行中は断る。
func (s Service) DeleteSprint(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID) error {
	return s.writePlan(ctx, store, ws, revival{},
		func(p planning.Plan) (string, error) {
			sp, err := deletableSprint(p, id)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("スプリント「%s」を削除", oneLine(sp.Name)), nil
		},
		func(tx port.Tx, p planning.Plan) error {
			if _, err := deletableSprint(p, id); err != nil {
				return err
			}
			if err := clearRefs(tx, ws, item.SprintField, id, func(it item.Item) change.ID { return it.Sprint }); err != nil {
				return err
			}
			return tx.WriteWorkspaces().Append(ws, planning.DeleteSprint(id))
		})
}

// deletableSprint は、削除できるスプリント id。削除済みかなければ ErrNotFound、実行中なら断る。
func deletableSprint(p planning.Plan, id change.ID) (planning.Sprint, error) {
	sp, ok := p.UsableSprint(id)
	if !ok {
		return planning.Sprint{}, port.ErrNotFound
	}
	if sp.State == planning.Active {
		return planning.Sprint{}, refusal.New("実行中は削除できません")
	}
	return sp, nil
}

// DeleteRelease は、リリース id を削除済みにする(F1-13)。どの状態でも消せる。1 回の保存で、
// 入っている削除済みでないアイテムからリリースを外す。
func (s Service) DeleteRelease(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID) error {
	return s.writePlan(ctx, store, ws, revival{},
		func(p planning.Plan) (string, error) {
			r, ok := p.UsableRelease(id)
			if !ok {
				return "", port.ErrNotFound
			}
			return fmt.Sprintf("リリース「%s」を削除", oneLine(r.Name)), nil
		},
		func(tx port.Tx, p planning.Plan) error {
			if _, ok := p.UsableRelease(id); !ok {
				return port.ErrNotFound
			}
			if err := clearRefs(tx, ws, item.ReleaseField, id, func(it item.Item) change.ID { return it.Release }); err != nil {
				return err
			}
			return tx.WriteWorkspaces().Append(ws, planning.DeleteRelease(id))
		})
}

// clearRefs は、ワークスペース ws の削除済みでないアイテムのうち、ref が id のものの項目 f を null にする。
func clearRefs(tx port.Tx, ws workspace.ID, f string, id change.ID, ref func(item.Item) change.ID) error {
	its, err := items(tx, ws)
	if err != nil {
		return err
	}
	for _, it := range its {
		if it.Deleted != nil || it.Place.Workspace != ws || ref(it) != id {
			continue
		}
		if err := tx.WriteItems().Append(it.ID, change.NewSet(f, id, nil)); err != nil {
			return err
		}
	}
	return nil
}

// RestoreSprint は、削除済みのスプリント id を戻す(F1-14)。削除済みでなければ ErrNotFound。
// ワークスペースが削除済みか、削除済みでない同じ名前のスプリントがあれば断る。
func (s Service) RestoreSprint(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID) error {
	return s.writePlanFrom(ctx, store, ws, restorePlan, revival{},
		func(p planning.Plan) (string, error) {
			sp, err := restorableSprint(p, id)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("スプリント「%s」を復元", oneLine(sp.Name)), nil
		},
		func(tx port.Tx, p planning.Plan) error {
			sp, err := restorableSprint(p, id)
			if err != nil {
				return err
			}
			return undoOp(tx, ws, sp.Deleted.Op)
		})
}

// restorePlan は、復元の前に読む計画。ワークスペースがなければ ErrNotFound、削除済みなら、名前の重なりより
// 先に断る(08「親が削除済みのときの復元」)。
func restorePlan(r port.Reader, ws workspace.ID) (planning.Plan, error) {
	w, err := r.Workspaces().Get(ws)
	if err != nil {
		return planning.Plan{}, err
	}
	if w.Deleted != nil {
		return planning.Plan{}, refusal.New("ワークスペースも削除されています。先にワークスペースを復元してください")
	}
	return logPlan(r, ws)
}

func restorableSprint(p planning.Plan, id change.ID) (planning.Sprint, error) {
	for _, sp := range p.Sprints {
		if sp.ID != id || sp.Deleted == nil {
			continue
		}
		for _, o := range p.LiveSprints() {
			if planning.NormalizeName(o.Name) == planning.NormalizeName(sp.Name) {
				return planning.Sprint{}, refusal.New(sprintNameTaken)
			}
		}
		return sp, nil
	}
	return planning.Sprint{}, port.ErrNotFound
}

// RestoreRelease は、削除済みのリリース id を戻す(F1-14)。削除済みでなければ ErrNotFound。
// ワークスペースが削除済みか、削除済みでない同じ名前のリリースがあれば断る。
func (s Service) RestoreRelease(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID) error {
	return s.writePlanFrom(ctx, store, ws, restorePlan, revival{},
		func(p planning.Plan) (string, error) {
			r, err := restorableRelease(p, id)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("リリース「%s」を復元", oneLine(r.Name)), nil
		},
		func(tx port.Tx, p planning.Plan) error {
			r, err := restorableRelease(p, id)
			if err != nil {
				return err
			}
			return undoOp(tx, ws, r.Deleted.Op)
		})
}

func restorableRelease(p planning.Plan, id change.ID) (planning.Release, error) {
	for _, r := range p.Releases {
		if r.ID != id || r.Deleted == nil {
			continue
		}
		for _, o := range p.LiveReleases() {
			if planning.NormalizeName(o.Name) == planning.NormalizeName(r.Name) {
				return planning.Release{}, refusal.New(releaseNameTaken)
			}
		}
		return r, nil
	}
	return planning.Release{}, port.ErrNotFound
}

// undoOp は、削除の op の行を、その行があるログ(ワークスペースと、削除済みも含むアイテム)ごとに取り消す。
// あとで別の操作が上書きした行は、change.Undo が先端でないとして戻さない。
func undoOp(tx port.Tx, ws workspace.ID, op change.ID) error {
	log, err := tx.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	if err != nil {
		return err
	}
	if err := tx.WriteWorkspaces().Append(ws, change.Undo(log, op)...); err != nil {
		return err
	}
	all, err := works(tx, ws)
	if err != nil {
		return err
	}
	for _, w := range all {
		if ds := change.Undo(w.Log, op); len(ds) > 0 {
			if err := tx.WriteItems().Append(w.Item.ID, ds...); err != nil {
				return err
			}
		}
	}
	return nil
}
