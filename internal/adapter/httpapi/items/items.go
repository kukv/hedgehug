// Package items は、アイテムの API を持つ。
package items

import (
	"context"
	"encoding/json"
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
	Blocked   bool     `json:"blocked" doc:"ブロック中か。ブロックしている相手に、完了・中止でないものがある"`
	CreatedAt string   `json:"createdAt"`
	CreatedBy string   `json:"createdBy"`

	Status   string                     `json:"status" doc:"ステータスの内部 ID。空は値なし"`
	Priority string                     `json:"priority"`
	Assignee string                     `json:"assignee" doc:"ログイン名。空は未割り当て"`
	Start    string                     `json:"start" doc:"YYYY-MM-DD。空は値なし"`
	End      string                     `json:"end"`
	Sprint   string                     `json:"sprint" doc:"スプリントの内部 ID。空はバックログ"`
	Release  string                     `json:"release" doc:"リリースの内部 ID。空は「なし」"`
	Points   *float64                   `json:"points"`
	Estimate *float64                   `json:"estimate" doc:"時間"`
	Systems  []string                   `json:"systems" nullable:"false"`
	Choices  map[string][]string        `json:"choices" nullable:"false" doc:"選択式の追加プロパティの値(プロパティの内部 ID → 値の内部 ID)"`
	Fields   map[string]json.RawMessage `json:"fields" nullable:"false" doc:"追加プロパティの値(プロパティの内部 ID → 値)。型は設定で読む"`
}

func toAPI(it item.Item) Item {
	out := Item{
		ID: string(it.ID), Workspace: string(it.Place.Workspace), Project: string(it.Place.Project),
		Number: it.Place.Number, Kind: string(it.Kind), Rank: it.Rank, Title: it.Title, Watchers: it.Watchers, Blocked: it.Blocked,
		CreatedAt: it.Created.At.Format(change.TimeLayout), CreatedBy: it.Created.By,
		Status: string(it.Status), Priority: string(it.Priority), Assignee: it.Assignee, Start: it.Start, End: it.End,
		Sprint: string(it.Sprint), Release: string(it.Release),
		Points: it.Points, Estimate: it.Estimate, Systems: []string{}, Choices: map[string][]string{}, Fields: map[string]json.RawMessage{},
	}
	for _, s := range it.Systems {
		out.Systems = append(out.Systems, string(s))
	}
	for k, vs := range it.Choices {
		ids := make([]string, 0, len(vs))
		for _, v := range vs {
			ids = append(ids, string(v))
		}
		out.Choices[string(k)] = ids
	}
	for k, v := range it.Fields {
		out.Fields[string(k)] = v
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

	Description string                     `json:"description,omitempty" doc:"もとになるアイテムの説明(Markdown)。空は説明なし。子の説明はテンプレートのものを使う"`
	Values      map[string]json.RawMessage `json:"values,omitempty" doc:"プロパティの値(キーはプロパティの内部 ID。組み込みはキー)"`
	Template    string                     `json:"template,omitempty" doc:"一緒に作るテンプレートの内部 ID。空は「なし」"`
	Sprint      *string                    `json:"sprint,omitempty" nullable:"true" doc:"入れるスプリントの内部 ID。null と空はバックログ"`
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
	registerCreateRead(api, svc)
	registerEdit(api, svc)
	registerDelete(api, svc)
	registerNumbers(api, svc)
	registerValues(api, svc)
	registerMove(api, svc)
	registerKind(api, svc)
	registerDuplicate(api, svc)
}

func registerCreateRead(api huma.API, svc appitems.Service) {
	huma.Register(api, huma.Operation{OperationID: "create-item", Method: http.MethodPost, Path: "/api/stores/{store}/items", Summary: "アイテムを作成"},
		func(ctx context.Context, in *createInput) (*oneOutput, error) {
			return one(svc.Create(ctx, createRequest(in)))
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

// createRequest は、作成の本文を usecase の入力にする。空の ID は「選んでいない」。
func createRequest(in *createInput) appitems.CreateInput {
	req := appitems.CreateInput{Store: port.StoreID(in.Store), Project: project.ID(in.Body.Project), Kind: item.Kind(in.Body.Kind), Title: in.Body.Title, Description: in.Body.Description}
	if len(in.Body.Values) > 0 {
		req.Values = make(map[change.ID]json.RawMessage, len(in.Body.Values))
		for k, v := range in.Body.Values {
			req.Values[change.ID(k)] = v
		}
	}
	if in.Body.Parent != "" {
		p := item.ID(in.Body.Parent)
		req.Parent = &p
	}
	if in.Body.Sprint != nil && *in.Body.Sprint != "" {
		sp := change.ID(*in.Body.Sprint)
		req.Sprint = &sp
	}
	if in.Body.Template != "" {
		tid := change.ID(in.Body.Template)
		req.Template = &tid
	}
	return req
}
