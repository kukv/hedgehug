// Package planning は、スプリントとリリースの操作(仕様 05「スプリント」「リリース」)を持つ。
package planning

import (
	"cmp"
	"context"
	"errors"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Service は、スプリントとリリースの操作が使う口。
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
	Clock    port.Clock
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// planOf は、ワークスペースのスプリントとリリース。ワークスペースがないか削除済みなら ErrNotFound(B5b-4)。
func planOf(r port.Reader, ws workspace.ID) (planning.Plan, error) {
	if _, err := port.LiveWorkspace(r, ws); err != nil {
		return planning.Plan{}, err
	}
	return logPlan(r, ws)
}

// logPlan は、ワークスペースのログのスプリントとリリース。ワークスペースが削除済みかは見ない。
func logPlan(r port.Reader, ws workspace.ID) (planning.Plan, error) {
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	if err != nil {
		return planning.Plan{}, err
	}
	return planning.FromLog(log), nil
}

// items は、ワークスペースの全プロジェクトの全アイテム。削除済みも含む(絞るのは呼ぶ側)。
func items(r port.Reader, ws workspace.ID) ([]item.Item, error) {
	ps, err := r.Projects().List(ws)
	if err != nil {
		return nil, err
	}
	var out []item.Item
	for _, p := range ps {
		its, err := r.Items().List(p.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, its...)
	}
	return out, nil
}

// works は、items に、それぞれのログを添えたもの(確定値や取り消しにログが要るときだけ使う)。
func works(r port.Reader, ws workspace.ID) ([]planning.Work, error) {
	its, err := items(r, ws)
	if err != nil {
		return nil, err
	}
	out := make([]planning.Work, 0, len(its))
	for _, it := range its {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: it.ID})
		if err != nil {
			return nil, err
		}
		out = append(out, planning.Work{Item: it, Log: log})
	}
	return out, nil
}

// editPlan は、summary で決めた文で 1 回保存する。drafts は保存の中のいまの計画から行を作る。
// rv のものが削除済みなら、保存で削除を取り消す。
func (s Service) editPlan(ctx context.Context, store port.StoreID, ws workspace.ID, rv revival,
	summary func(p planning.Plan) (string, error),
	drafts func(p planning.Plan) ([]change.Draft, error),
) error {
	return s.writePlan(ctx, store, ws, rv, summary, func(tx port.Tx, p planning.Plan) error {
		ds, err := drafts(p)
		if err != nil {
			return err
		}
		if len(ds) == 0 {
			return errNoRows
		}
		return tx.WriteWorkspaces().Append(ws, ds...)
	})
}

// writePlan は、summary で決めた文で 1 回保存する。write は保存の中のいまの計画を見て書く。
// rv のものが削除済みなら、保存で削除を取り消す。
func (s Service) writePlan(ctx context.Context, store port.StoreID, ws workspace.ID, rv revival,
	summary func(p planning.Plan) (string, error),
	write func(tx port.Tx, p planning.Plan) error,
) error {
	return s.writePlanFrom(ctx, store, ws, rv.plan, rv, summary, write)
}

// writePlanFrom は、writePlan の、計画を plan で読む版。summary か write が errNoRows を返したら、取り消すものが
// あれば ErrNotFound、なければ何も保存しない(summary が返したときは errNoRows をそのまま返す)。
func (s Service) writePlanFrom(ctx context.Context, store port.StoreID, ws workspace.ID,
	plan func(r port.Reader, ws workspace.ID) (planning.Plan, error), rv revival,
	summary func(p planning.Plan) (string, error),
	write func(tx port.Tx, p planning.Plan) error,
) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	var text string
	err = s.Stores.Read(ctx, store, func(r port.Reader) error {
		p, err := plan(r, ws)
		if err != nil {
			return err
		}
		if text, err = summary(p); !errors.Is(err, errNoRows) {
			return err
		}
		if pending, perr := rv.pending(r, ws); perr != nil || pending {
			return cmp.Or(perr, port.ErrNotFound)
		}
		return err
	})
	if err != nil {
		return err
	}
	return s.Stores.Write(ctx, store, port.Operation{Actor: change.User(me), Summary: text}, func(tx port.Tx) error {
		return rv.write(ctx, tx, ws, plan, write)
	})
}
