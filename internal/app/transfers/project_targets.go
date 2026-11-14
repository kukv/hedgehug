package transfers

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ProjectTarget は、プロジェクトを移す先の候補のワークスペース。
type ProjectTarget struct {
	Store     port.StoreInfo
	Workspace workspace.Workspace
}

// ProjectTargets は、プロジェクト id を移す先の候補を返す(02 **移動**、A5b-16)。候補は、表示中で閲覧のみでない
// Store の、自分のワークスペースと削除済みのワークスペースを除くすべてのワークスペース(プロジェクトの有無を問わない)。
// 並びは Store の並び → ワークスペースの並び(rank の昇順、同じなら内部 ID の順)。プロジェクトがないか削除済みなら
// port.ErrNotFound。Store は 1 つずつ順に読む(読み取りを入れ子にしない)。
func (s Service) ProjectTargets(ctx context.Context, store port.StoreID, id project.ID) ([]ProjectTarget, error) {
	var own workspace.ID
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		p, err := port.LiveProject(r, id)
		own = p.Place.Workspace
		return err
	})
	if err != nil {
		return nil, err
	}
	infos, err := s.StoreList.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []ProjectTarget
	for _, info := range infos {
		if info.ReadOnly {
			continue
		}
		var skip workspace.ID
		if info.ID == store {
			skip = own
		}
		ts, err := s.projectStoreTargets(ctx, info, skip)
		if err != nil {
			return nil, err
		}
		out = append(out, ts...)
	}
	return out, nil
}

// projectStoreTargets は、Store info の削除されていないワークスペースを、ワークスペースの並びで返す。skip は除く。
func (s Service) projectStoreTargets(ctx context.Context, info port.StoreInfo, skip workspace.ID) ([]ProjectTarget, error) {
	var out []ProjectTarget
	ranks := map[workspace.ID]string{}
	err := s.Stores.Read(ctx, info.ID, func(r port.Reader) error {
		wss, err := port.LiveWorkspaces(r)
		if err != nil {
			return err
		}
		for _, w := range wss {
			if w.ID == skip {
				continue
			}
			log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: w.ID})
			if err != nil {
				return err
			}
			ranks[w.ID] = workspace.RankOf(log)
			out = append(out, ProjectTarget{Store: info, Workspace: w})
		}
		return nil
	})
	sortByWorkspaceRank(out, ranks, func(t ProjectTarget) workspace.ID { return t.Workspace.ID })
	return out, err
}
