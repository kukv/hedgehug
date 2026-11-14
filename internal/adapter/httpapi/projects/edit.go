package projects

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	appprojects "github.com/kukv/hedgehug/internal/app/projects"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// 本文は名前のある型にする(名前のない型だと、OpenAPI のスキーマ名が登録の順で決まる)。

// RenameProject は、タイトルの変更の本文。前後の空白は除いて保存する。
type RenameProject struct {
	Title string `json:"title" pattern:"^[^\\r\\n]*$" doc:"1 行。改行は入れられない"`
}

// SetProjectStatus は、ステータスの変更の本文。
type SetProjectStatus struct {
	Status string `json:"status" enum:"Todo,InProgress,Done,Pending,Cancel"`
}

// SetProjectOwner は、オーナーの変更の本文。
type SetProjectOwner struct {
	Owner string `json:"owner" doc:"ログイン名。空は「なし」。メンバーでなければ 404"`
}

// SetProjectDate は、開始日か終了日の変更の本文。
type SetProjectDate struct {
	Field string `json:"field" enum:"start,end"`
	Value string `json:"value" pattern:"^(\\d{4}-\\d{2}-\\d{2})?$" doc:"YYYY-MM-DD。空は「なし」"`
}

// ProjectRepo は、リポジトリの追加・削除の本文。
type ProjectRepo struct {
	Repo string `json:"repo" doc:"owner/name。前後の空白は除く"`
}

// 入力の型は、パスの項目を埋め込みにせず直接持つ(埋め込むと huma がパスの値を入れない)。
type renameInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Body    RenameProject
}

type statusInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Body    SetProjectStatus
}

type ownerInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Body    SetProjectOwner
}

type dateInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Body    SetProjectDate
}

type repoInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Body    ProjectRepo
}

func one(p project.Project, err error) (*oneOutput, error) {
	if err != nil {
		return nil, apierr.From(err)
	}
	return &oneOutput{Body: toAPI(p)}, nil
}

// registerEdit は、プロジェクトの編集の API を登録する。scan は、リポジトリを足したあとの PR の探索(nil なら探さない)。
func registerEdit(api huma.API, svc appprojects.Service, scan ScanFunc) {
	const path = "/api/stores/{store}/projects/{project}"
	huma.Register(api, huma.Operation{OperationID: "rename-project", Method: http.MethodPut, Path: path + "/title", Summary: "プロジェクトのタイトルを変える"},
		func(ctx context.Context, in *renameInput) (*oneOutput, error) {
			return one(svc.Rename(ctx, port.StoreID(in.Store), project.ID(in.Project), in.Body.Title))
		})
	huma.Register(api, huma.Operation{OperationID: "set-project-status", Method: http.MethodPut, Path: path + "/status", Summary: "プロジェクトのステータスを変える"},
		func(ctx context.Context, in *statusInput) (*oneOutput, error) {
			return one(svc.SetStatus(ctx, port.StoreID(in.Store), project.ID(in.Project), project.Status(in.Body.Status)))
		})
	huma.Register(api, huma.Operation{OperationID: "set-project-owner", Method: http.MethodPut, Path: path + "/owner", Summary: "プロジェクトのオーナーを変える"},
		func(ctx context.Context, in *ownerInput) (*oneOutput, error) {
			return one(svc.SetOwner(ctx, port.StoreID(in.Store), project.ID(in.Project), in.Body.Owner))
		})
	huma.Register(api, huma.Operation{OperationID: "set-project-date", Method: http.MethodPut, Path: path + "/date", Summary: "プロジェクトの開始日か終了日を変える"},
		func(ctx context.Context, in *dateInput) (*oneOutput, error) {
			if err := apierr.CheckDates(in.Body.Value); err != nil {
				return nil, err
			}
			return one(svc.SetDate(ctx, port.StoreID(in.Store), project.ID(in.Project), appprojects.DateField(in.Body.Field), in.Body.Value))
		})
	huma.Register(api, huma.Operation{OperationID: "add-project-repo", Method: http.MethodPost, Path: path + "/repos", Summary: "プロジェクトにリポジトリを結びつける"},
		func(ctx context.Context, in *repoInput) (*oneOutput, error) {
			p, err := svc.AddRepo(ctx, port.StoreID(in.Store), project.ID(in.Project), in.Body.Repo)
			if err == nil && scan != nil {
				// 足したリポジトリは、プロジェクトに保存した書き方(前後の空白を除いたもの)で探す
				scan(ctx, port.StoreID(in.Store), strings.TrimSpace(in.Body.Repo))
			}
			return one(p, err)
		})
	huma.Register(api, huma.Operation{OperationID: "remove-project-repo", Method: http.MethodPost, Path: path + "/repos/remove", Summary: "プロジェクトからリポジトリを外す"},
		func(ctx context.Context, in *repoInput) (*oneOutput, error) {
			return one(svc.RemoveRepo(ctx, port.StoreID(in.Store), project.ID(in.Project), in.Body.Repo))
		})
}
