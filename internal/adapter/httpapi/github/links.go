package github

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// LinkRow は、PR・Issue のリンクの 1 行。
type LinkRow struct {
	Kind   string `json:"kind" enum:"pr,issue"`
	Repo   string `json:"repo" doc:"owner/name"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state" enum:"open,draft,merged,closed"`
	Review string `json:"review" enum:"none,pending,approved,changes_requested" doc:"Issue は none"`
	Checks string `json:"checks" enum:",success,failure,pending" doc:"このマシンが読んだ PR のチェックの結果。なければ空"`
	Error  string `json:"error" doc:"取り直しの失敗(「状態を更新できませんでした: …」)。なければ空"`
}

// ProjectLinkRow は、プロジェクトの概要の「PR と Issue」の 1 行。
type ProjectLinkRow struct {
	LinkRow
	Item    string `json:"item" doc:"アイテムの内部 ID"`
	Display string `json:"display" doc:"アイテムの画面の ID(HH-2)"`
}

// LinkInput は「PR をリンク」「Issue をリンク」の本文。
type LinkInput struct {
	Kind         string `json:"kind" enum:"pr,issue"`
	Repo         string `json:"repo"`
	Number       string `json:"number" doc:"入力のまま。正の整数でなければ断る"`
	AddToProject bool   `json:"addToProject" doc:"プロジェクトのリポジトリにも追加する"`
}

// LinkRef は、解除するリンク。
type LinkRef struct {
	Kind   string `json:"kind" enum:"pr,issue"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
}

type linksPath struct {
	Store string `path:"store"`
	Item  string `path:"item"`
}

type projectLinksPath struct {
	Store   string `path:"store"`
	Project string `path:"project"`
}

type addLinkInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  LinkInput
}

type removeLinkInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  LinkRef
}

type linkRowsOutput struct{ Body []LinkRow }

type projectLinkRowsOutput struct{ Body []ProjectLinkRow }

func registerLinks(api huma.API, svc ghapp.Links) {
	const path = "/api/stores/{store}/items/{item}/links"
	huma.Register(api, huma.Operation{OperationID: "list-links", Method: http.MethodGet, Path: path, Summary: "アイテムの PR・Issue のリンクを、リンクした順に返す(GitHub には問い合わせない)"},
		func(ctx context.Context, in *linksPath) (*linkRowsOutput, error) {
			rows, err := svc.List(ctx, port.StoreID(in.Store), item.ID(in.Item))
			return linkRows(rows, err)
		})
	huma.Register(api, huma.Operation{OperationID: "refresh-links", Method: http.MethodPost, Path: path + "/refresh", Summary: "アイテムのリンクを GitHub から取り直し、一覧を返す"},
		func(ctx context.Context, in *linksPath) (*linkRowsOutput, error) {
			rows, err := svc.Refresh(ctx, port.StoreID(in.Store), item.ID(in.Item))
			return linkRows(rows, err)
		})
	huma.Register(api, huma.Operation{OperationID: "add-link", Method: http.MethodPost, Path: path, DefaultStatus: http.StatusNoContent, Summary: "PR か Issue を GitHub で確かめてリンクする"},
		func(ctx context.Context, in *addLinkInput) (*struct{}, error) {
			b := in.Body
			err := svc.Add(ctx, port.StoreID(in.Store), item.ID(in.Item), ghapp.AddLinkInput{
				Kind: itemlink.Kind(b.Kind), Repo: b.Repo, Number: b.Number, AddToProject: b.AddToProject,
			})
			return nil, apierr.From(err)
		})
	huma.Register(api, huma.Operation{OperationID: "remove-link", Method: http.MethodPost, Path: path + "/remove", DefaultStatus: http.StatusNoContent, Summary: "リンクを外す(PR は解除の記録に入る)"},
		func(ctx context.Context, in *removeLinkInput) (*struct{}, error) {
			b := in.Body
			err := svc.Remove(ctx, port.StoreID(in.Store), item.ID(in.Item), itemlink.Ref{Kind: itemlink.Kind(b.Kind), Repo: b.Repo, Number: b.Number})
			return nil, apierr.From(err)
		})
	huma.Register(api, huma.Operation{OperationID: "list-project-links", Method: http.MethodGet, Path: "/api/stores/{store}/projects/{project}/links", Summary: "プロジェクトの下のアイテムのリンクを、アイテムの番号順に返す"},
		func(ctx context.Context, in *projectLinksPath) (*projectLinkRowsOutput, error) {
			rows, err := svc.ProjectLinks(ctx, port.StoreID(in.Store), project.ID(in.Project))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &projectLinkRowsOutput{Body: []ProjectLinkRow{}}
			for _, r := range rows {
				out.Body = append(out.Body, ProjectLinkRow{LinkRow: linkRow(r.LinkRow), Item: string(r.Item), Display: r.Display})
			}
			return out, nil
		})
}

func linkRows(rows []ghapp.LinkRow, err error) (*linkRowsOutput, error) {
	if err != nil {
		return nil, apierr.From(err)
	}
	out := &linkRowsOutput{Body: []LinkRow{}}
	for _, r := range rows {
		out.Body = append(out.Body, linkRow(r))
	}
	return out, nil
}

func linkRow(r ghapp.LinkRow) LinkRow {
	return LinkRow{
		Kind: string(r.Kind), Repo: r.Repo, Number: r.Number, Title: r.Title, State: string(r.State),
		Review: string(r.Review), Checks: string(r.Checks), Error: r.Error,
	}
}
