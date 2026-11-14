package repairing

import (
	"context"
	"errors"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// wsOrphans は、ワークスペース ws のログにあって、持ち主のプロジェクトが ws にないルール。
type wsOrphans struct {
	ws    workspace.ID
	rules []repair.OrphanRules
}

// dropOrphanRules は、持ち主のプロジェクトがワークスペースにないルール(プロジェクト別のルールと専用ルール)を
// 外す((9)。決定 C5b1-3)。プロジェクト 1 つを 1 回の保存にする。件名の {タイトル} は projectTitle の順で決める。
func dropOrphanRules(_ context.Context, w port.RepairWriter) error {
	var found []wsOrphans
	titles := map[project.ID]string{}
	err := w.Read(func(r port.Reader) error {
		wss, err := r.Workspaces().List()
		if err != nil {
			return err
		}
		for _, ws := range wss {
			o, err := orphansOf(r, ws.ID)
			if err != nil {
				return err
			}
			for _, or := range o.rules {
				if titles[or.Project], err = projectTitle(r, or.Project); err != nil {
					return err
				}
			}
			found = append(found, o)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, o := range found {
		for _, or := range o.rules {
			op := port.Operation{Actor: syncActor, Summary: "プロジェクト「" + oneLine(titles[or.Project]) + "」のルールを削除(プロジェクトがない)"}
			err := w.Write(op, func(tx port.Tx) error { return tx.WriteWorkspaces().Append(o.ws, or.Drafts...) })
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// orphansOf は、ワークスペース ws のログにある、ws にないプロジェクトのルール。ws のプロジェクトは削除済みを含む。
func orphansOf(r port.Reader, ws workspace.ID) (wsOrphans, error) {
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	if err != nil {
		return wsOrphans{}, err
	}
	ps, err := r.Projects().List(ws)
	if err != nil {
		return wsOrphans{}, err
	}
	in := func(id project.ID) bool {
		return slices.ContainsFunc(ps, func(p project.Project) bool { return p.ID == id })
	}
	return wsOrphans{ws: ws, rules: repair.Orphans(log, in)}, nil
}

// projectTitle は、なくなったプロジェクト id の名前(決定 C5b1-4)。読めるプロジェクトの名前、最後の転送記録の
// old、purged.jsonl の display、内部 ID の順。
func projectTitle(r port.Reader, id project.ID) (string, error) {
	p, err := r.Projects().Get(id)
	if err == nil {
		return p.Title, nil
	}
	if !errors.Is(err, port.ErrNotFound) {
		return "", err
	}
	trs, err := r.Transfers().List()
	if err != nil {
		return "", err
	}
	for _, t := range slices.Backward(trs) {
		if t.Target == change.ID(id) {
			return t.Old, nil
		}
	}
	purged, err := r.Purged().List()
	if err != nil {
		return "", err
	}
	for _, pg := range purged {
		if pg.Target == change.ID(id) && pg.Display != "" {
			return pg.Display, nil
		}
	}
	return string(id), nil
}
