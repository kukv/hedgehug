package items

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// NextNumber は、次に作るアイテムの番号(ID の予告の目安)。
type NextNumber struct {
	Number int `json:"number"`
}

type wsInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
}

type byNumberInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Number    int    `path:"number" minimum:"1"`
}

type nextOutput struct{ Body NextNumber }

func registerNumbers(api huma.API, svc appitems.Service) {
	huma.Register(api, huma.Operation{OperationID: "next-item-number", Method: http.MethodGet, Path: "/api/stores/{store}/workspaces/{workspace}/next-item-number", Summary: "ワークスペースで次に作るアイテムの番号を返す"},
		func(ctx context.Context, in *wsInput) (*nextOutput, error) {
			n, err := svc.NextNumber(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &nextOutput{Body: NextNumber{Number: n}}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "find-item-by-number", Method: http.MethodGet, Path: "/api/stores/{store}/workspaces/{workspace}/items/by-number/{number}", Summary: "ワークスペースの番号でアイテムを返す"},
		func(ctx context.Context, in *byNumberInput) (*oneOutput, error) {
			it, err := svc.FindByNumber(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), in.Number)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &oneOutput{Body: toAPI(it)}, nil
		})
}
