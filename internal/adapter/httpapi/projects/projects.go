// Package projects は、プロジェクトの API を持つ。
package projects

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	appprojects "github.com/kukv/hedgehug/internal/app/projects"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Project は、API が返すプロジェクト。
type Project struct {
	ID        string `json:"id"`
	Workspace string `json:"workspace"`
	Number    int    `json:"number"`
	Rank      string `json:"rank"`
	Title     string `json:"title"`
	Owner     string `json:"owner" doc:"ログイン名。空は「なし」"`
	Status    string `json:"status" enum:"Todo,InProgress,Done,Pending,Cancel"`
	Start     string `json:"start" doc:"YYYY-MM-DD。空は「なし」"`
	End       string `json:"end"`
}

func toAPI(p project.Project) Project {
	return Project{
		ID: string(p.ID), Workspace: string(p.Place.Workspace), Number: p.Place.Number, Rank: p.Rank,
		Title: p.Title, Owner: p.Owner, Status: string(p.Status), Start: p.Start, End: p.End,
	}
}

type wsPath struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
}

// CreateProject は、プロジェクトの作成の本文。名前のない型にすると、OpenAPI のスキーマ名が
// CreateInputBody・CreateInputBody1 のように、ほかの機能との登録の順で決まる。
type CreateProject struct {
	Title  string `json:"title"`
	Owner  string `json:"owner,omitempty"`
	Status string `json:"status,omitempty" enum:"Todo,InProgress,Done,Pending,Cancel,"`
	Start  string `json:"start,omitempty" pattern:"^(\\d{4}-\\d{2}-\\d{2})?$"`
	End    string `json:"end,omitempty" pattern:"^(\\d{4}-\\d{2}-\\d{2})?$"`
}

// createInput は、パスの項目を wsPath の埋め込みにせず直接持つ(埋め込むと huma がパスの値を入れない)。
type createInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Body      CreateProject
}

type oneOutput struct{ Body Project }

type listOutput struct{ Body []Project }

type getInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
}

// Register は、プロジェクトの API を登録する。
func Register(api huma.API, svc appprojects.Service) {
	huma.Register(api, huma.Operation{OperationID: "create-project", Method: http.MethodPost, Path: "/api/stores/{store}/workspaces/{workspace}/projects", Summary: "プロジェクトを作成"},
		func(ctx context.Context, in *createInput) (*oneOutput, error) {
			p, err := svc.Create(ctx, appprojects.CreateInput{
				Store: port.StoreID(in.Store), Workspace: workspace.ID(in.Workspace), Title: in.Body.Title,
				Owner: in.Body.Owner, Status: project.Status(in.Body.Status), Start: in.Body.Start, End: in.Body.End,
			})
			if err != nil {
				return nil, apierr.From(err)
			}
			return &oneOutput{Body: toAPI(p)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "list-projects", Method: http.MethodGet, Path: "/api/stores/{store}/workspaces/{workspace}/projects", Summary: "ワークスペースのプロジェクトを並び順で返す"},
		func(ctx context.Context, in *wsPath) (*listOutput, error) {
			ps, err := svc.List(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &listOutput{Body: []Project{}}
			for _, p := range ps {
				out.Body = append(out.Body, toAPI(p))
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "get-project", Method: http.MethodGet, Path: "/api/stores/{store}/projects/{project}", Summary: "プロジェクトを返す"},
		func(ctx context.Context, in *getInput) (*oneOutput, error) {
			p, err := svc.Get(ctx, port.StoreID(in.Store), project.ID(in.Project))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &oneOutput{Body: toAPI(p)}, nil
		})
}
