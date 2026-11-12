// Package items は、アイテムの API を持つ。
package items

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// Item は、API が返すアイテム。
type Item struct {
	ID        string   `json:"id"`
	Workspace string   `json:"workspace"`
	Project   string   `json:"project"`
	Parent    *string  `json:"parent" doc:"親の内部 ID。Epic は null"`
	Number    int      `json:"number"`
	Kind      string   `json:"kind" enum:"Epic,Story,Task,Bug,Step"`
	Rank      string   `json:"rank"`
	Title     string   `json:"title"`
	Watchers  []string `json:"watchers"`
	CreatedAt string   `json:"createdAt"`
	CreatedBy string   `json:"createdBy"`
}

func toAPI(it item.Item) Item {
	out := Item{
		ID: string(it.ID), Workspace: string(it.Place.Workspace), Project: string(it.Place.Project),
		Number: it.Place.Number, Kind: string(it.Kind), Rank: it.Rank, Title: it.Title, Watchers: it.Watchers,
		CreatedAt: it.Created.At.Format(change.TimeLayout), CreatedBy: it.Created.By,
	}
	if out.Watchers == nil {
		out.Watchers = []string{}
	}
	if it.Place.Parent != nil {
		p := string(*it.Place.Parent)
		out.Parent = &p
	}
	return out
}

// CreateItem は、アイテムの作成の本文。名前のない型にすると、OpenAPI のスキーマ名が
// CreateInputBody・CreateInputBody1 のように、ほかの機能との登録の順で決まる。
type CreateItem struct {
	Project string `json:"project" doc:"空はプロジェクトを選んでいない"`
	Parent  string `json:"parent,omitempty" doc:"空は親を選んでいない"`
	Kind    string `json:"kind" enum:"Epic,Story,Task,Bug,Step"`
	Title   string `json:"title"`
}

type createInput struct {
	Store string `path:"store"`
	Body  CreateItem
}

type oneOutput struct{ Body Item }

type listOutput struct{ Body []Item }

type getInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
}

type listInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
}

// Register は、アイテムの API を登録する。
func Register(api huma.API, svc appitems.Service) {
	huma.Register(api, huma.Operation{OperationID: "create-item", Method: http.MethodPost, Path: "/api/stores/{store}/items", Summary: "アイテムを作成"},
		func(ctx context.Context, in *createInput) (*oneOutput, error) {
			req := appitems.CreateInput{Store: port.StoreID(in.Store), Project: project.ID(in.Body.Project), Kind: item.Kind(in.Body.Kind), Title: in.Body.Title}
			if in.Body.Parent != "" {
				p := item.ID(in.Body.Parent)
				req.Parent = &p
			}
			it, err := svc.Create(ctx, req)
			if errors.Is(err, appitems.ErrParentElsewhere) {
				// 画面の親の候補は同じプロジェクトだけなので、呼び出しの誤り。仕様の文言はない
				return nil, huma.Error422UnprocessableEntity(err.Error())
			}
			if err != nil {
				return nil, apierr.From(err)
			}
			return &oneOutput{Body: toAPI(it)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "get-item", Method: http.MethodGet, Path: "/api/stores/{store}/items/{item}", Summary: "アイテムを返す"},
		func(ctx context.Context, in *getInput) (*oneOutput, error) {
			it, err := svc.Get(ctx, port.StoreID(in.Store), item.ID(in.Item))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &oneOutput{Body: toAPI(it)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "list-project-items", Method: http.MethodGet, Path: "/api/stores/{store}/projects/{project}/items", Summary: "プロジェクトのすべてのアイテムを番号の順で返す"},
		func(ctx context.Context, in *listInput) (*listOutput, error) {
			its, err := svc.List(ctx, port.StoreID(in.Store), project.ID(in.Project))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &listOutput{Body: []Item{}}
			for _, it := range its {
				out.Body = append(out.Body, toAPI(it))
			}
			return out, nil
		})
}
