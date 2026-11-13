package items

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
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
	// Statuses は、プロジェクトが変わるときの、移動先が使わないステータスの ID → 置き換え先の ID。指定のないものは、同じ分類の移動先の既定にする
	Statuses map[string]string `json:"statuses,omitempty" doc:"プロジェクトが変わるときの、移動先が使わないステータスの ID → 置き換え先のステータスの ID。指定のないものは同じ分類の移動先の既定になる"`
}

// MovePreview は、移動の確認ダイアログに出す件数。
type MovePreview struct {
	Below    int                 `json:"below" doc:"下のもの(削除済みを除く)の件数"`
	Statuses []MovePreviewStatus `json:"statuses" nullable:"false" doc:"移動先が使わないステータス。ワークスペースのステータスの順。なければ空"`
}

// MovePreviewStatus は、移動先が使わないステータスと、それを持つ移すアイテムの件数。
type MovePreviewStatus struct {
	Status  string `json:"status" doc:"ステータスの ID"`
	Count   int    `json:"count" doc:"そのステータスを持つ移すアイテム(削除済みと自分自身を含む)の件数"`
	Default string `json:"default" doc:"置き換えの初めの値。同じ分類の、移動先の既定のステータスの ID"`
}

type movePreviewInput struct {
	Store   string `path:"store"`
	Item    string `path:"item"`
	Project string `query:"project" doc:"移し先のプロジェクトの内部 ID(Epic)"`
	Parent  string `query:"parent" doc:"新しい親の内部 ID(Epic 以外)"`
}

type movePreviewOutput struct{ Body MovePreview }

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
	huma.Register(api, huma.Operation{OperationID: "preview-item-move", Method: http.MethodGet, Path: "/api/stores/{store}/items/{item}/move-preview", Summary: "移動の確認ダイアログに出す件数を返す(保存しない)"},
		func(ctx context.Context, in *movePreviewInput) (*movePreviewOutput, error) {
			p, err := svc.MovePreview(ctx, port.StoreID(in.Store), item.ID(in.Item), toMoveInput(in.Parent, in.Project, nil))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &movePreviewOutput{Body: MovePreview{Below: p.Below, Statuses: []MovePreviewStatus{}}}
			for _, c := range p.Statuses {
				out.Body.Statuses = append(out.Body.Statuses, MovePreviewStatus{Status: string(c.Status), Count: c.Count, Default: string(c.Default)})
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "move-item", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/move", Summary: "同じワークスペースの中でアイテムを移す", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *moveInput) (*struct{}, error) {
			req := toMoveInput(in.Body.Parent, in.Body.Project, in.Body.Statuses)
			if err := svc.Move(ctx, port.StoreID(in.Store), item.ID(in.Item), req); err != nil {
				return nil, apierr.From(err)
			}
			return nil, nil //nolint:nilnil // 204 は本文なし
		})
}

// toMoveInput は、空でないほうの移し先(親かプロジェクト)と置き換えを、usecase の入力にする。
func toMoveInput(parent, proj string, statuses map[string]string) appitems.MoveInput {
	req := appitems.MoveInput{}
	if parent != "" {
		p := item.ID(parent)
		req.Parent = &p
	}
	if proj != "" {
		p := project.ID(proj)
		req.Project = &p
	}
	if len(statuses) > 0 {
		req.Statuses = make(map[change.ID]change.ID, len(statuses))
		for from, to := range statuses {
			req.Statuses[change.ID(from)] = change.ID(to)
		}
	}
	return req
}
