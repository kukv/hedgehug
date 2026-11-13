package notes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	apprelations "github.com/kukv/hedgehug/internal/app/relations"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
)

// RelationRow は、関連の一覧の 1 行。
type RelationRow struct {
	Label         string `json:"label" doc:"表記(blocks・blocked by・causes・caused by・duplicates・duplicated by・relates to・clones・cloned by)"`
	ID            string `json:"id" doc:"相手の内部 ID"`
	Display       string `json:"display" doc:"相手の画面の ID(HH-3)"`
	Title         string `json:"title"`
	WorkspaceName string `json:"workspaceName" doc:"相手が別のワークスペースのときだけ。同じなら空"`
	StatusName    string `json:"statusName" doc:"相手のステータスの名前。値がなければ空"`
	StatusColor   string `json:"statusColor" doc:"ステータスの色の名前。値がなければ空"`
}

// RelationCandidate は、相手の候補。
type RelationCandidate struct {
	ID             string `json:"id" doc:"相手の内部 ID"`
	Display        string `json:"display"`
	Title          string `json:"title"`
	WorkspaceName  string `json:"workspaceName" doc:"別のワークスペースの候補だけ。同じなら空"`
	DisabledReason string `json:"disabledReason" doc:"選べない理由。選べるなら空"`
}

// RelationInput は、関連の追加・解除の本文。Label は、このアイテムから見た表記。
type RelationInput struct {
	Label string `json:"label"`
	Other string `json:"other" doc:"相手の内部 ID"`
}

type relationsPath struct {
	Store string `path:"store"`
	Item  string `path:"item"`
}

type candidatesInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Label string `query:"label"`
	Q     string `query:"q"`
}

type relationEditInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  RelationInput
}

type relationRowsOutput struct{ Body []RelationRow }

type relationCandidatesOutput struct{ Body []RelationCandidate }

func registerRelations(api huma.API, svc apprelations.Service) {
	huma.Register(api, huma.Operation{OperationID: "list-relations", Method: http.MethodGet, Path: "/api/stores/{store}/items/{item}/relations", Summary: "アイテムの関連を、表記の順で返す(逆向きの表記を含む)"},
		func(ctx context.Context, in *relationsPath) (*relationRowsOutput, error) {
			rows, err := svc.List(ctx, port.StoreID(in.Store), item.ID(in.Item))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &relationRowsOutput{Body: []RelationRow{}}
			for _, r := range rows {
				out.Body = append(out.Body, RelationRow{
					Label: string(r.Label), ID: string(r.Item.ID), Display: r.Display, Title: r.Item.Title,
					WorkspaceName: r.WorkspaceName, StatusName: r.Status.Name, StatusColor: string(r.Status.Color),
				})
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "list-relation-candidates", Method: http.MethodGet, Path: "/api/stores/{store}/items/{item}/relation-candidates", Summary: "関連の相手の候補を返す(ID かタイトルで絞る。20 件まで)"},
		func(ctx context.Context, in *candidatesInput) (*relationCandidatesOutput, error) {
			cs, err := svc.Candidates(ctx, port.StoreID(in.Store), item.ID(in.Item), relation.Label(in.Label), in.Q)
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &relationCandidatesOutput{Body: []RelationCandidate{}}
			for _, c := range cs {
				out.Body = append(out.Body, RelationCandidate{
					ID: string(c.Item.ID), Display: c.Display, Title: c.Item.Title,
					WorkspaceName: c.WorkspaceName, DisabledReason: c.DisabledReason,
				})
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "add-relation", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/relations", Summary: "関連を足す", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *relationEditInput) (*struct{}, error) {
			err := svc.Add(ctx, port.StoreID(in.Store), item.ID(in.Item), relation.Label(in.Body.Label), item.ID(in.Body.Other))
			return nil, apierr.From(err)
		})
	huma.Register(api, huma.Operation{OperationID: "remove-relation", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/relations/remove", Summary: "関連を外す", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *relationEditInput) (*struct{}, error) {
			err := svc.Remove(ctx, port.StoreID(in.Store), item.ID(in.Item), relation.Label(in.Body.Label), item.ID(in.Body.Other))
			return nil, apierr.From(err)
		})
}
