// Package transfers は、アイテムの別のワークスペース・別の Store への移動と、旧 ID の解決の API(A5a-19)。
package transfers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	apptransfers "github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// TransferEpic は、移す先の候補のプロジェクトの Epic(移動先の親に選べるもの)。
type TransferEpic struct {
	ID      string `json:"id"`
	Display string `json:"display" doc:"画面の ID(MB-1)"`
	Title   string `json:"title"`
}

// TransferProject は、移す先の候補のワークスペースの、削除されていないプロジェクト。
type TransferProject struct {
	ID    string         `json:"id"`
	Title string         `json:"title"`
	Epics []TransferEpic `json:"epics" doc:"削除されていない Epic(並び順)"`
}

// TransferTarget は、移す先の候補のワークスペース(A5a-13)。
type TransferTarget struct {
	StoreID       string            `json:"storeId"`
	StoreName     string            `json:"storeName"`
	WorkspaceID   string            `json:"workspaceId"`
	WorkspaceName string            `json:"workspaceName"`
	Prefix        string            `json:"prefix"`
	Projects      []TransferProject `json:"projects"`
}

// TransferChoice は、ステータスの置き換えで選べる移動先のステータス。
type TransferChoice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// TransferReplacement は、ダイアログの「ステータスの置き換え」の 1 行。
type TransferReplacement struct {
	Status  string           `json:"status" doc:"移動元のステータスの内部 ID"`
	Name    string           `json:"name" doc:"移動元のステータスの名前"`
	Count   int              `json:"count" doc:"そのステータスの移すアイテム(下のもの・削除済みを含む)の数"`
	Choices []TransferChoice `json:"choices" doc:"移動先のプロジェクトが使うステータス"`
	Default string           `json:"default" doc:"同じ分類の移動先の既定"`
}

// TransferPreview は、移動のダイアログの予告(A5a-14)。
type TransferPreview struct {
	Below                int                   `json:"below" doc:"下の削除されていないものの数"`
	Deleted              int                   `json:"deleted" doc:"下の削除済みのものの数"`
	NextDisplay          string                `json:"nextDisplay" doc:"移動先の次の番号の ID(MB-2)"`
	CrossStore           bool                  `json:"crossStore" doc:"別の Store への移動か"`
	RemoteNotice         bool                  `json:"remoteNotice" doc:"別の Store で、移動先に remote がなく移動元にあるか"`
	DestinationHasRemote bool                  `json:"destinationHasRemote" doc:"移動先の Store に remote があるか"`
	Replacements         []TransferReplacement `json:"replacements" doc:"行の並びは移動元のワークスペースのステータスの並び"`
}

type itemInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
}

type previewInput struct {
	Store       string `path:"store"`
	Item        string `path:"item"`
	ToStore     string `query:"toStore" required:"true"`
	ToWorkspace string `query:"toWorkspace" required:"true"`
	ToProject   string `query:"toProject" required:"true"`
}

type targetsOutput struct{ Body []TransferTarget }

type previewOutput struct{ Body TransferPreview }

// Register は、移動と旧 ID の解決の API を登録する。
func Register(api huma.API, svc apptransfers.Service) {
	huma.Register(api, huma.Operation{OperationID: "list-transfer-targets", Method: http.MethodGet, Path: "/api/stores/{store}/items/{item}/transfer-targets", Summary: "「別のワークスペースへ移動…」の移す先の候補を返す"},
		func(ctx context.Context, in *itemInput) (*targetsOutput, error) {
			ts, err := svc.Targets(ctx, port.StoreID(in.Store), item.ID(in.Item))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &targetsOutput{Body: []TransferTarget{}}
			for _, t := range ts {
				out.Body = append(out.Body, toTarget(t))
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "get-transfer-preview", Method: http.MethodGet, Path: "/api/stores/{store}/items/{item}/transfer-preview", Summary: "移動のダイアログの予告を返す"},
		func(ctx context.Context, in *previewInput) (*previewOutput, error) {
			to := apptransfers.MoveInput{ToStore: port.StoreID(in.ToStore), ToWorkspace: workspace.ID(in.ToWorkspace), ToProject: project.ID(in.ToProject)}
			p, err := svc.Preview(ctx, port.StoreID(in.Store), item.ID(in.Item), to)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &previewOutput{Body: toPreview(p)}, nil
		})
	registerMove(api, svc)
	registerLocate(api, svc)
	registerProject(api, svc)
	registerWorkspace(api, svc)
}

func toTarget(t apptransfers.Target) TransferTarget {
	out := TransferTarget{
		StoreID: string(t.Store.ID), StoreName: t.Store.Name, WorkspaceID: string(t.Workspace.ID), WorkspaceName: t.Workspace.Name,
		Prefix: t.Workspace.Prefix, Projects: []TransferProject{},
	}
	for _, p := range t.Projects {
		tp := TransferProject{ID: string(p.ID), Title: p.Title, Epics: []TransferEpic{}}
		for _, e := range t.Epics[p.ID] {
			tp.Epics = append(tp.Epics, TransferEpic{ID: string(e.ID), Display: e.DisplayID(t.Workspace.Prefix), Title: e.Title})
		}
		out.Projects = append(out.Projects, tp)
	}
	return out
}

func toPreview(p apptransfers.Preview) TransferPreview {
	out := TransferPreview{
		Below: p.Below, Deleted: p.Deleted, NextDisplay: p.NextDisplay, CrossStore: p.CrossStore, RemoteNotice: p.RemoteNotice,
		DestinationHasRemote: p.DestinationHasRemote, Replacements: []TransferReplacement{},
	}
	for _, r := range p.Replacements {
		row := TransferReplacement{Status: string(r.Status), Name: r.Name, Count: r.Count, Choices: []TransferChoice{}, Default: string(r.Default)}
		for _, c := range r.Choices {
			row.Choices = append(row.Choices, TransferChoice{ID: string(c.ID), Name: c.Name})
		}
		out.Replacements = append(out.Replacements, row)
	}
	return out
}
