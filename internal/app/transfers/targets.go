package transfers

import (
	"context"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Target は、移す先の候補のワークスペース。Projects は削除されていないプロジェクト(並び順)、Epics はプロジェクト
// ごとの削除されていない Epic(兄弟の並び順)。
type Target struct {
	Store     port.StoreInfo
	Workspace workspace.Workspace
	Projects  []project.Project
	Epics     map[project.ID][]item.Item
}

// Targets は、アイテム id を移す先の候補を返す(02「移す先の選び方」、A5a-13)。候補は、表示中で閲覧のみでない
// Store の、削除されていないプロジェクトが 1 つ以上ある、自分のワークスペース以外のワークスペース。並びは
// Store の並び → ワークスペースの並び(rank の昇順、同じなら内部 ID の順)。移すアイテムがない・削除済み・
// Step なら port.ErrNotFound。Store は 1 つずつ順に読む(読み取りを入れ子にしない)。
func (s Service) Targets(ctx context.Context, store port.StoreID, id item.ID) ([]Target, error) {
	var own workspace.ID
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		src, err := readSource(r, id)
		own = src.ws.ID
		return err
	})
	if err != nil {
		return nil, err
	}
	infos, err := s.StoreList.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []Target
	for _, info := range infos {
		if info.ReadOnly {
			continue
		}
		var skip workspace.ID
		if info.ID == store {
			skip = own
		}
		ts, err := s.storeTargets(ctx, info, skip)
		if err != nil {
			return nil, err
		}
		out = append(out, ts...)
	}
	return out, nil
}

// storeTargets は、Store info の候補を、ワークスペースの並びで返す。skip のワークスペースは除く。
func (s Service) storeTargets(ctx context.Context, info port.StoreInfo, skip workspace.ID) ([]Target, error) {
	var out []Target
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
			t, rk, err := workspaceTarget(r, info, w)
			if err != nil {
				return err
			}
			if len(t.Projects) > 0 {
				ranks[w.ID] = rk
				out = append(out, t)
			}
		}
		return nil
	})
	sortByWorkspaceRank(out, ranks, func(t Target) workspace.ID { return t.Workspace.ID })
	return out, err
}

// workspaceTarget は、ワークスペース w の削除されていないプロジェクトと、その Epic と、ワークスペースの並びの値を読む。
func workspaceTarget(r port.Reader, info port.StoreInfo, w workspace.Workspace) (Target, string, error) {
	ps, err := port.LiveProjects(r, w.ID)
	if err != nil {
		return Target{}, "", err
	}
	t := Target{Store: info, Workspace: w, Projects: ps, Epics: map[project.ID][]item.Item{}}
	for _, p := range ps {
		its, err := port.LiveItems(r, p.ID)
		if err != nil {
			return Target{}, "", err
		}
		epics := slices.DeleteFunc(its, func(it item.Item) bool { return it.Kind != item.Epic })
		item.SortSiblings(epics)
		t.Epics[p.ID] = epics
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: w.ID})
	if err != nil {
		return Target{}, "", err
	}
	return t, workspace.RankOf(log), nil
}

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
