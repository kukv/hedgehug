package items

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// MoveCandidate は、「移動…」の移し先の候補。Epic にはプロジェクト、ほかにはアイテムが入る。
type MoveCandidate struct {
	Kind         string `json:"kind" enum:"item,project" doc:"item はアイテム(親になる Epic・Story・Task・Bug)、project はプロジェクト(Epic の移し先)"`
	ID           string `json:"id" doc:"アイテムかプロジェクトの内部 ID"`
	Display      string `json:"display" doc:"画面の ID(HH-12・HH-P2)"`
	Title        string `json:"title"`
	ProjectID    string `json:"projectId" doc:"移したあとのプロジェクトの内部 ID。アイテムの候補なら属するプロジェクト、プロジェクトの候補なら自分"`
	ProjectTitle string `json:"projectTitle" doc:"アイテムの候補が属するプロジェクトの名前。プロジェクトの候補では空"`
}

// MoveItem は、移動の本文。parent と project のどちらか一方だけを入れる。
type MoveItem struct {
	Parent  string `json:"parent,omitempty" doc:"新しい親の内部 ID(Epic 以外)"`
	Project string `json:"project,omitempty" doc:"移し先のプロジェクトの内部 ID(Epic)"`
}

type moveInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  MoveItem
}

type candidatesOutput struct{ Body []MoveCandidate }

func toCandidate(c appitems.MoveCandidate) MoveCandidate {
	if c.Project != nil {
		return MoveCandidate{Kind: "project", ID: string(c.Project.ID), Display: c.Display, Title: c.Project.Title, ProjectID: string(c.ProjectID)}
	}
	return MoveCandidate{Kind: "item", ID: string(c.Item.ID), Display: c.Display, Title: c.Item.Title, ProjectID: string(c.ProjectID), ProjectTitle: c.ProjectTitle}
}

func registerMove(api huma.API, svc appitems.Service) {
	huma.Register(api, huma.Operation{OperationID: "list-move-candidates", Method: http.MethodGet, Path: "/api/stores/{store}/items/{item}/move-candidates", Summary: "「移動…」の移し先の候補を返す"},
		func(ctx context.Context, in *getInput) (*candidatesOutput, error) {
			cs, err := svc.MoveCandidates(ctx, port.StoreID(in.Store), item.ID(in.Item))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &candidatesOutput{Body: []MoveCandidate{}}
			for _, c := range cs {
				out.Body = append(out.Body, toCandidate(c))
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "move-item", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/move", Summary: "同じワークスペースの中でアイテムを移す", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *moveInput) (*struct{}, error) {
			req := appitems.MoveInput{}
			if in.Body.Parent != "" {
				p := item.ID(in.Body.Parent)
				req.Parent = &p
			}
			if in.Body.Project != "" {
				p := project.ID(in.Body.Project)
				req.Project = &p
			}
			if err := svc.Move(ctx, port.StoreID(in.Store), item.ID(in.Item), req); err != nil {
				return nil, apierr.From(err)
			}
			return nil, nil //nolint:nilnil // 204 は本文なし
		})
}
