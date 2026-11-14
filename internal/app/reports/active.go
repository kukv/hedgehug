package reports

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Active は、実行中のスプリントの数字(F3-15)。Sprint は実行中のスプリント(なければ nil で、単位はポイント、ほかは空)。
// Done・Total は進捗(中止を除く)。
type Active struct {
	Sprint   *planning.Sprint
	Unit     planning.Unit
	Burndown planning.Burndown
	Added    float64
	Done     planning.Totals
	Total    planning.Totals
}

// Active は、実行中のスプリントの小さいバーンダウン・途中で追加した量・進捗を返す。
func (s Service) Active(ctx context.Context, store port.StoreID, ws workspace.ID) (Active, error) {
	sn, err := s.load(ctx, store, ws)
	if err != nil {
		return Active{}, err
	}
	sp := planning.ActiveSprint(sn.live)
	if sp == nil {
		return Active{Unit: planning.UnitPoints}, nil
	}
	now := s.Clock.Now()
	unit, _ := planning.EffectiveUnit(sn.its, ws, sn.sel.Unit)
	a := Active{Sprint: sp, Unit: unit}
	a.Burndown = planning.Burn(sn.works, ws, *sp, sn.st, unit, now)
	a.Added = planning.Added(sn.works, ws, *sp, sn.st, unit, now)
	a.Done, a.Total = planning.Progress(sn.its, ws, sp.ID, sn.st)
	return a, nil
}
