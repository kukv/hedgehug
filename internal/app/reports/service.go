// Package reports は、レポートと実行中のスプリントの数字を読み、レポートの選択を保存する(仕様 05「レポート」)。
package reports

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Selections は、レポートの選択の保存先(state.db。F3-5)。Get の found は、保存した行があるか。
type Selections interface {
	Get(ctx context.Context, store port.StoreID, ws workspace.ID) (planning.Selection, bool, error)
	Put(ctx context.Context, store port.StoreID, ws workspace.ID, sel planning.Selection) error
}

// Service は、レポートが使う口。
type Service struct {
	Stores     port.Stores
	Selections Selections
	Clock      port.Clock
}

// Patch は、選択の変更。nil の項目は変えない(F3-7)。
type Patch struct {
	Unit     *planning.Unit
	Sprint   *change.ID
	Layers   *planning.Layers
	Estimate *change.ID
}

// snapshot は、1 回の読み取りで集めたワークスペースの状態。
type snapshot struct {
	ws    workspace.Workspace
	live  []planning.Sprint
	st    workspace.Settings
	works []planning.Work
	its   []item.Item
	sel   planning.Selection
}

// load は、ワークスペース ws がなければ(削除済みも)ErrNotFound。選択は読むときに直した値(F3-5)。
func (s Service) load(ctx context.Context, store port.StoreID, ws workspace.ID) (snapshot, error) {
	var sn snapshot
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		w, err := port.LiveWorkspace(r, ws)
		if err != nil {
			return err
		}
		sn.ws = w
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
		if err != nil {
			return err
		}
		sn.live = planning.FromLog(log).LiveSprints()
		if sn.st, err = r.Workspaces().Settings(ws); err != nil {
			return err
		}
		if sn.works, err = works(r, ws); err != nil {
			return err
		}
		sn.its = make([]item.Item, 0, len(sn.works))
		for _, w := range sn.works {
			sn.its = append(sn.its, w.Item)
		}
		return nil
	})
	if err != nil {
		return snapshot{}, err
	}
	saved, found, err := s.Selections.Get(ctx, store, ws)
	if err != nil {
		return snapshot{}, err
	}
	sn.sel = planning.ResolveSelection(saved, found, sn.live)
	return sn, nil
}

// works は、ワークスペースの全プロジェクトの全アイテム(削除済みも含む)に、それぞれのログを添えたもの。
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

// Select は、選択を保存する(F3-7)。state.db の生の行に、送られた項目だけを重ねる。Store には書かないので、
// 閲覧のみの Store でも変えられる。ワークスペースがないか削除済みなら ErrNotFound、単位・層が読めない値なら ErrInvalidOperation。
func (s Service) Select(ctx context.Context, store port.StoreID, ws workspace.ID, p Patch) error {
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		_, err := port.LiveWorkspace(r, ws)
		return err
	})
	if err != nil {
		return err
	}
	if p.Unit != nil && *p.Unit != planning.UnitPoints && *p.Unit != planning.UnitCount {
		return fmt.Errorf("%w: unit %q", port.ErrInvalidOperation, *p.Unit)
	}
	if p.Layers != nil && *p.Layers != planning.LayersCategory && *p.Layers != planning.LayersStatus {
		return fmt.Errorf("%w: layers %q", port.ErrInvalidOperation, *p.Layers)
	}
	sel, _, err := s.Selections.Get(ctx, store, ws)
	if err != nil {
		return err
	}
	if p.Unit != nil {
		sel.Unit = *p.Unit
	}
	if p.Sprint != nil {
		sel.Sprint = *p.Sprint
	}
	if p.Layers != nil {
		sel.Layers = *p.Layers
	}
	if p.Estimate != nil {
		sel.Estimate = *p.Estimate
	}
	return s.Selections.Put(ctx, store, ws, sel)
}
