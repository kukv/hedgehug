package palette

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// snapshot は、表示中の 1 つの Store の、削除されていないワークスペース・プロジェクト・アイテムと、転送記録
// (app/transfers の snapshots の写し。app の機能どうしは import しない)。
type snapshot struct {
	info       port.StoreInfo
	workspaces []wsSnap
	records    []transfer.Record
}

// wsSnap は、削除されていないワークスペースと、その旧い接頭辞・削除されていないプロジェクト。
type wsSnap struct {
	ws       workspace.Workspace
	old      []string
	projects []projectSnap
}

// projectSnap は、削除されていないプロジェクトと、その削除されていないアイテム。
type projectSnap struct {
	project project.Project
	items   []item.Item
}

// snapshots は、表示中のすべての Store(閲覧のみを含む)を、1 つずつ読む(読み取りを入れ子にしない)。
func (s Service) snapshots(ctx context.Context) ([]snapshot, error) {
	infos, err := s.StoreList.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]snapshot, 0, len(infos))
	for _, info := range infos {
		var snap snapshot
		if err := s.Stores.Read(ctx, info.ID, func(r port.Reader) (err error) {
			snap, err = readSnapshot(r, info)
			return err
		}); err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, nil
}

func readSnapshot(r port.Reader, info port.StoreInfo) (snapshot, error) {
	snap := snapshot{info: info}
	wss, err := port.LiveWorkspaces(r)
	if err != nil {
		return snap, err
	}
	for _, w := range wss {
		ws, err := readWorkspace(r, w)
		if err != nil {
			return snap, err
		}
		snap.workspaces = append(snap.workspaces, ws)
	}
	snap.records, err = r.Transfers().List()
	return snap, err
}

// readWorkspace は、ワークスペース w の旧い接頭辞と、削除されていないプロジェクト・アイテムを読む。
func readWorkspace(r port.Reader, w workspace.Workspace) (wsSnap, error) {
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: w.ID})
	if err != nil {
		return wsSnap{}, err
	}
	out := wsSnap{ws: w, old: workspace.OldPrefixes(log)}
	ps, err := port.LiveProjects(r, w.ID)
	if err != nil {
		return wsSnap{}, err
	}
	for _, p := range ps {
		its, err := port.LiveItems(r, p.ID)
		if err != nil {
			return wsSnap{}, err
		}
		out.projects = append(out.projects, projectSnap{project: p, items: its})
	}
	return out, nil
}
