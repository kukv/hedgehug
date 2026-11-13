// Package planning は、スプリントとリリースの操作(仕様 05「スプリント」「リリース」)を持つ。
package planning

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
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

// planOf は、ワークスペースのスプリントとリリース。ワークスペースがなければ ErrNotFound。
func planOf(r port.Reader, ws workspace.ID) (planning.Plan, error) {
	if _, err := r.Workspaces().Get(ws); err != nil {
		return planning.Plan{}, err
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	if err != nil {
		return planning.Plan{}, err
	}
	return planning.FromLog(log), nil
}

// works は、ワークスペースの全プロジェクトの全アイテムと、そのログ。削除済みも含む(絞るのは呼ぶ側)。
func works(r port.Reader, ws workspace.ID) ([]planning.Work, error) {
	ps, err := r.Projects().List(ws)
	if err != nil {
		return nil, err
	}
	var out []planning.Work
	for _, p := range ps {
		its, err := r.Items().List(p.ID)
		if err != nil {
			return nil, err
		}
		for _, it := range its {
			log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: it.ID})
			if err != nil {
				return nil, err
			}
			out = append(out, planning.Work{Item: it, Log: log})
		}
	}
	return out, nil
}

// editPlan は、summary で決めた文で 1 回保存する。drafts は保存の中のいまの計画から行を作る。
func (s Service) editPlan(ctx context.Context, store port.StoreID, ws workspace.ID,
	summary func(p planning.Plan) (string, error),
	drafts func(p planning.Plan) ([]change.Draft, error),
) error {
	return s.writePlan(ctx, store, ws, summary, func(tx port.Tx, p planning.Plan) error {
		ds, err := drafts(p)
		if err != nil {
			return err
		}
		return tx.WriteWorkspaces().Append(ws, ds...)
	})
}

// writePlan は、summary で決めた文で 1 回保存する。write は保存の中のいまの計画を見て書く。
func (s Service) writePlan(ctx context.Context, store port.StoreID, ws workspace.ID,
	summary func(p planning.Plan) (string, error),
	write func(tx port.Tx, p planning.Plan) error,
) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	var text string
	err = s.Stores.Read(ctx, store, func(r port.Reader) error {
		p, err := planOf(r, ws)
		if err != nil {
			return err
		}
		text, err = summary(p)
		return err
	})
	if err != nil {
		return err
	}
	return s.Stores.Write(ctx, store, port.Operation{Actor: change.User(me), Summary: text}, func(tx port.Tx) error {
		p, err := planOf(tx, ws)
		if err != nil {
			return err
		}
		return write(tx, p)
	})
}
