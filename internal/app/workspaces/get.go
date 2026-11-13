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

// List は、store の削除されていないワークスペースを並びの順(rank、同じなら内部 ID の順。rank のないものは先頭)に返す(B2-7)。
func (s Service) List(ctx context.Context, store port.StoreID) ([]workspace.Workspace, error) {
	var out []workspace.Workspace
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		list, err := orderedLiveWorkspaces(r)
		for _, w := range list {
			out = append(out, w.Workspace)
		}
		return err
	})
	return out, err
}

// Get は、ワークスペースと設定と旧い接頭辞とプロジェクト別のルールを返す。
func (s Service) Get(ctx context.Context, store port.StoreID, id workspace.ID) (Detail, error) {
	var out Detail
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		w, err := port.LiveWorkspace(r, id)
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
