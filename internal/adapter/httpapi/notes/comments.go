package notes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appcomments "github.com/kukv/hedgehug/internal/app/comments"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// Comment は、API が返すコメント。
type Comment struct {
	ID     string `json:"id"`
	By     string `json:"by" doc:"投稿者のログイン名"`
	ByName string `json:"byName" doc:"投稿者の表示名。メンバーにいなければ「不明なユーザー」"`
	At     string `json:"at" doc:"投稿した時刻(RFC 3339)"`
	Body   string `json:"body"`
	Edited bool   `json:"edited"`
}

// CommentText は、コメントの投稿と編集の本文。
type CommentText struct {
	Body string `json:"body"`
}

type commentOutput struct{ Body Comment }

type commentsOutput struct{ Body []Comment }

func toComment(v appcomments.View) Comment {
	return Comment{ID: string(v.ID), By: v.By, ByName: v.ByName, At: v.At.Format(change.TimeLayout), Body: v.Body, Edited: v.Edited}
}

func oneComment(v appcomments.View, err error) (*commentOutput, error) {
	if err != nil {
		return nil, apierr.From(err)
	}
	return &commentOutput{Body: toComment(v)}, nil
}

func manyComments(vs []appcomments.View, err error) (*commentsOutput, error) {
	if err != nil {
		return nil, apierr.From(err)
	}
	out := &commentsOutput{Body: []Comment{}}
	for _, v := range vs {
		out.Body = append(out.Body, toComment(v))
	}
	return out, nil
}

// アイテムのコメントの入力。パスの名前が違うので、プロジェクトのものと型を分ける。
type itemRef struct {
	Store string `path:"store"`
	Item  string `path:"item"`
}

type itemCommentRef struct {
	Store   string `path:"store"`
	Item    string `path:"item"`
	Comment string `path:"comment"`
}

type postItemComment struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  CommentText
}

type editItemComment struct {
	Store   string `path:"store"`
	Item    string `path:"item"`
	Comment string `path:"comment"`
	Body    CommentText
}

type projectRef struct {
	Store   string `path:"store"`
	Project string `path:"project"`
}

type projectCommentRef struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Comment string `path:"comment"`
}

type postProjectComment struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Body    CommentText
}

type editProjectComment struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Comment string `path:"comment"`
	Body    CommentText
}

// registerComments は、アイテムとプロジェクトのコメントの API を登録する。
func registerComments(api huma.API, svc appcomments.Service) {
	registerItemComments(api, svc)
	registerProjectComments(api, svc)
}

func registerItemComments(api huma.API, svc appcomments.Service) {
	at := func(id string) appcomments.Target {
		return appcomments.Target{Kind: change.KindItem, ID: change.ID(id)}
	}
	const base = "/api/stores/{store}/items/{item}/comments"
	huma.Register(api, huma.Operation{OperationID: "list-item-comments", Method: http.MethodGet, Path: base, Summary: "アイテムのコメントを古い順に返す"},
		func(ctx context.Context, in *itemRef) (*commentsOutput, error) {
			return manyComments(svc.List(ctx, port.StoreID(in.Store), at(in.Item)))
		})
	huma.Register(api, huma.Operation{OperationID: "post-item-comment", Method: http.MethodPost, Path: base, Summary: "アイテムにコメントを投稿する"},
		func(ctx context.Context, in *postItemComment) (*commentOutput, error) {
			return oneComment(svc.Post(ctx, port.StoreID(in.Store), at(in.Item), in.Body.Body))
		})
	huma.Register(api, huma.Operation{OperationID: "edit-item-comment", Method: http.MethodPut, Path: base + "/{comment}", Summary: "アイテムのコメントを編集する"},
		func(ctx context.Context, in *editItemComment) (*commentOutput, error) {
			return oneComment(svc.Edit(ctx, port.StoreID(in.Store), at(in.Item), change.ID(in.Comment), in.Body.Body))
		})
	huma.Register(api, huma.Operation{OperationID: "delete-item-comment", Method: http.MethodDelete, Path: base + "/{comment}", Summary: "アイテムのコメントを削除する", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *itemCommentRef) (*struct{}, error) {
			return nil, apierr.From(svc.Delete(ctx, port.StoreID(in.Store), at(in.Item), change.ID(in.Comment)))
		})
}

func registerProjectComments(api huma.API, svc appcomments.Service) {
	at := func(id string) appcomments.Target {
		return appcomments.Target{Kind: change.KindProject, ID: change.ID(id)}
	}
	const base = "/api/stores/{store}/projects/{project}/comments"
	huma.Register(api, huma.Operation{OperationID: "list-project-comments", Method: http.MethodGet, Path: base, Summary: "プロジェクトのコメントを古い順に返す"},
		func(ctx context.Context, in *projectRef) (*commentsOutput, error) {
			return manyComments(svc.List(ctx, port.StoreID(in.Store), at(in.Project)))
		})
	huma.Register(api, huma.Operation{OperationID: "post-project-comment", Method: http.MethodPost, Path: base, Summary: "プロジェクトにコメントを投稿する"},
		func(ctx context.Context, in *postProjectComment) (*commentOutput, error) {
			return oneComment(svc.Post(ctx, port.StoreID(in.Store), at(in.Project), in.Body.Body))
		})
	huma.Register(api, huma.Operation{OperationID: "edit-project-comment", Method: http.MethodPut, Path: base + "/{comment}", Summary: "プロジェクトのコメントを編集する"},
		func(ctx context.Context, in *editProjectComment) (*commentOutput, error) {
			return oneComment(svc.Edit(ctx, port.StoreID(in.Store), at(in.Project), change.ID(in.Comment), in.Body.Body))
		})
	huma.Register(api, huma.Operation{OperationID: "delete-project-comment", Method: http.MethodDelete, Path: base + "/{comment}", Summary: "プロジェクトのコメントを削除する", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *projectCommentRef) (*struct{}, error) {
			return nil, apierr.From(svc.Delete(ctx, port.StoreID(in.Store), at(in.Project), change.ID(in.Comment)))
		})
}
