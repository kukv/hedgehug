package workspaces

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Detail は、設定の画面が読むワークスペースの中身。
type Detail struct {
	Workspace   workspace.Workspace
	Settings    workspace.Settings
	OldPrefixes []string
	// ProjectRules は、プロジェクトごとのルール(B5a-18)。
	ProjectRules projectrule.Set
}

// Get は、ワークスペースと設定と旧い接頭辞とプロジェクト別のルールを返す。
func (s Service) Get(ctx context.Context, store port.StoreID, id workspace.ID) (Detail, error) {
	var out Detail
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		w, err := r.Workspaces().Get(id)
		if err != nil {
			return err
		}
		st, err := r.Workspaces().Settings(id)
		if err != nil {
			return err
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: id})
		if err != nil {
			return err
		}
		out = Detail{Workspace: w, Settings: st, OldPrefixes: workspace.OldPrefixes(log), ProjectRules: projectrule.FromLog(log)}
		return nil
	})
	return out, err
}
