// Package views は、保存したビューの API(E3b)。Store をまたぐので /api/stores/{store} の下に置かない。
package views

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appviews "github.com/kukv/hedgehug/internal/app/views"
)

// maxContentBody は、ビューの内容の JSON の大きさの上限(E3b-3。バイト数)。
const maxContentBody = 64 * 1024

// ViewSort は、テーブルの並べ替え。
type ViewSort struct {
	_    struct{} `nullable:"true"`
	Key  string   `json:"key"`
	Desc bool     `json:"desc"`
}

// ViewTable は、テーブルの設定。
type ViewTable struct {
	Columns []string  `json:"columns" nullable:"false" doc:"列の鍵(id・title・p:{キー} など)"`
	Group   string    `json:"group" doc:"グループの設定。空なら設定なし"`
	Sort    *ViewSort `json:"sort" required:"true" doc:"列見出しの並べ替え。なければ null"`
}

// ViewContent は、ビューの内容(検索の画面の状態と同じ形)。
type ViewContent struct {
	Query  string    `json:"query" doc:"クエリの文字列"`
	Layout string    `json:"layout" enum:"tree,table" doc:"表示の形"`
	Table  ViewTable `json:"table"`
}

// View は、保存したビュー。
type View struct {
	ID      string      `json:"id" pattern:"^[0-9a-z]{26}$" doc:"ULID を小文字にした 26 文字"`
	Name    string      `json:"name"`
	Content ViewContent `json:"content"`
}

// ViewList は、ビューの一覧。
type ViewList struct {
	Views []View `json:"views" nullable:"false" doc:"名前の順(大文字と小文字を区別しない)"`
}

// KeyImpact は、追加プロパティの削除がビューに及ぼす影響。
type KeyImpact struct {
	Affected bool `json:"affected" doc:"削除すると、保存したビューの条件にそのキーが使えなくなるか"`
}

type idInput struct {
	ID string `path:"id" pattern:"^[0-9a-z]{26}$" doc:"ビューの識別子"`
}

// CreateViewBody は、ビューを作る本文。
type CreateViewBody struct {
	Name    string      `json:"name" doc:"ビューの名前"`
	Content ViewContent `json:"content"`
}

// SaveViewBody は、ビューの内容を保存する本文。
type SaveViewBody struct {
	Content ViewContent `json:"content"`
}

// RenameViewBody は、ビューの名前を変える本文。
type RenameViewBody struct {
	Name string `json:"name" doc:"新しい名前"`
}

type listOutput struct{ Body ViewList }

type viewOutput struct{ Body View }

type createInput struct{ Body CreateViewBody }

type saveInput struct {
	ID   string `path:"id" pattern:"^[0-9a-z]{26}$" doc:"ビューの識別子"`
	Body SaveViewBody
}

type renameInput struct {
	ID   string `path:"id" pattern:"^[0-9a-z]{26}$" doc:"ビューの識別子"`
	Body RenameViewBody
}

type impactInput struct {
	StoreID     string `query:"storeId" required:"true" doc:"消すプロパティのあるワークスペースの Store"`
	WorkspaceID string `query:"workspaceId" required:"true" doc:"消すプロパティのあるワークスペース"`
	Key         string `query:"key" required:"true" doc:"消す追加プロパティのキー"`
}

type impactOutput struct{ Body KeyImpact }

// Register は、保存したビューの API を登録する。
func Register(api huma.API, svc *appviews.Service) {
	huma.Register(api, huma.Operation{OperationID: "list-views", Method: http.MethodGet, Path: "/api/views", Summary: "保存したビューの一覧を返す"},
		func(ctx context.Context, _ *struct{}) (*listOutput, error) {
			list, err := svc.List(ctx)
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &listOutput{Body: ViewList{Views: []View{}}}
			for _, v := range list {
				out.Body.Views = append(out.Body.Views, toView(v))
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "create-view", Method: http.MethodPost, Path: "/api/views", Summary: "ビューを作る"},
		func(ctx context.Context, in *createInput) (*viewOutput, error) {
			if err := checkSize(in.Body.Content); err != nil {
				return nil, err
			}
			return viewResult(svc.Create(ctx, in.Body.Name, toContent(in.Body.Content)))
		})
	huma.Register(api, huma.Operation{OperationID: "save-view", Method: http.MethodPut, Path: "/api/views/{id}", Summary: "ビューの内容を置き換える"},
		func(ctx context.Context, in *saveInput) (*viewOutput, error) {
			if err := checkSize(in.Body.Content); err != nil {
				return nil, err
			}
			return viewResult(svc.Save(ctx, in.ID, toContent(in.Body.Content)))
		})
	huma.Register(api, huma.Operation{OperationID: "rename-view", Method: http.MethodPost, Path: "/api/views/{id}/rename", Summary: "ビューの名前を変える"},
		func(ctx context.Context, in *renameInput) (*viewOutput, error) {
			return viewResult(svc.Rename(ctx, in.ID, in.Body.Name))
		})
	huma.Register(api, huma.Operation{OperationID: "duplicate-view", Method: http.MethodPost, Path: "/api/views/{id}/duplicate", Summary: "ビューを複製する"},
		func(ctx context.Context, in *idInput) (*viewOutput, error) {
			return viewResult(svc.Duplicate(ctx, in.ID))
		})
	huma.Register(api, huma.Operation{OperationID: "delete-view", Method: http.MethodDelete, Path: "/api/views/{id}", Summary: "ビューを削除する", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *idInput) (*struct{}, error) {
			return nil, apierr.From(svc.Delete(ctx, in.ID))
		})
	huma.Register(api, huma.Operation{OperationID: "view-key-impact", Method: http.MethodGet, Path: "/api/views/key-impact", Summary: "追加プロパティの削除が、保存したビューに影響するかを返す"},
		func(ctx context.Context, in *impactInput) (*impactOutput, error) {
			affected, err := svc.KeyImpact(ctx, portStore(in.StoreID), workspaceID(in.WorkspaceID), in.Key)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &impactOutput{Body: KeyImpact{Affected: affected}}, nil
		})
}

func viewResult(v appviews.View, err error) (*viewOutput, error) {
	if err != nil {
		return nil, apierr.From(err)
	}
	return &viewOutput{Body: toView(v)}, nil
}

// checkSize は、内容の JSON が上限を超えたら断る(E3b-3)。
func checkSize(c ViewContent) error {
	b, _ := json.Marshal(c)
	if len(b) > maxContentBody {
		return huma.Error422UnprocessableEntity("ビューが大きすぎます(64 KiB まで)")
	}
	return nil
}
