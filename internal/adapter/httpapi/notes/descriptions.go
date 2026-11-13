package notes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appdescriptions "github.com/kukv/hedgehug/internal/app/descriptions"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// DescriptionText は、説明の本文。
type DescriptionText struct {
	Text string `json:"text" doc:"説明の本文(Markdown)。説明がなければ空"`
}

// DescribeBody は、説明の保存の本文。
type DescribeBody struct {
	Base string `json:"base" doc:"編集を始めたときの説明。保存するときの共通の祖先になる"`
	Text string `json:"text" doc:"入力した説明。空白だけは空として保存する(説明を消す)"`
}

// DescribeResult は、説明の保存の応答。
type DescribeResult struct {
	Text     string `json:"text" doc:"保存したあとの説明"`
	Conflict bool   `json:"conflict" doc:"同じ箇所が同時に編集されて、両方の版を残したとき true"`
}

type itemKey struct {
	Store string `path:"store"`
	Item  string `path:"item"`
}

type projectKey struct {
	Store   string `path:"store"`
	Project string `path:"project"`
}

type describeItemInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  DescribeBody
}

type describeProjectInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Body    DescribeBody
}

type textOutput struct{ Body DescriptionText }

type describeOutput struct{ Body DescribeResult }

func registerDescriptions(api huma.API, svc appdescriptions.Service) {
	huma.Register(api, huma.Operation{OperationID: "get-item-description", Method: http.MethodGet, Path: "/api/stores/{store}/items/{item}/description", Summary: "アイテムの説明を返す"},
		func(ctx context.Context, in *itemKey) (*textOutput, error) {
			text, err := svc.GetItem(ctx, port.StoreID(in.Store), item.ID(in.Item))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &textOutput{Body: DescriptionText{Text: text}}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "describe-item", Method: http.MethodPut, Path: "/api/stores/{store}/items/{item}/description", Summary: "アイテムの説明を保存する"},
		func(ctx context.Context, in *describeItemInput) (*describeOutput, error) {
			res, err := svc.SaveItem(ctx, port.StoreID(in.Store), item.ID(in.Item), in.Body.Base, in.Body.Text)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &describeOutput{Body: DescribeResult{Text: res.Text, Conflict: res.Conflict}}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "get-project-description", Method: http.MethodGet, Path: "/api/stores/{store}/projects/{project}/description", Summary: "プロジェクトの説明を返す"},
		func(ctx context.Context, in *projectKey) (*textOutput, error) {
			text, err := svc.GetProject(ctx, port.StoreID(in.Store), project.ID(in.Project))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &textOutput{Body: DescriptionText{Text: text}}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "describe-project", Method: http.MethodPut, Path: "/api/stores/{store}/projects/{project}/description", Summary: "プロジェクトの説明を保存する"},
		func(ctx context.Context, in *describeProjectInput) (*describeOutput, error) {
			res, err := svc.SaveProject(ctx, port.StoreID(in.Store), project.ID(in.Project), in.Body.Base, in.Body.Text)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &describeOutput{Body: DescribeResult{Text: res.Text, Conflict: res.Conflict}}, nil
		})
}
