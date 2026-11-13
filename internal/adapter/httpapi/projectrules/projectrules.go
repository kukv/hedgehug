// Package projectrules は、プロジェクト別のルール(B5a)の HTTP の入口。本文の型には機能の名前を付ける。
package projectrules

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	appprojectrules "github.com/kukv/hedgehug/internal/app/projectrules"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
)

const rulesPath = "/api/stores/{store}/projects/{project}/rules"

// SetProjectStatusUsed は、ステータスを使う・使わないの本文。
type SetProjectStatusUsed struct {
	Used bool `json:"used"`
}

// SetProjectTransition は、遷移の制約のセルの本文。
type SetProjectTransition struct {
	Allowed bool `json:"allowed"`
}

// SetProjectTemplateUsed は、テンプレートを使う・使わないの本文。
type SetProjectTemplateUsed struct {
	Used bool `json:"used"`
}

// SetProjectTemplateDefault は、種類ごとの既定の本文。
type SetProjectTemplateDefault struct {
	Template string `json:"template" doc:"テンプレートの ID。空ならワークスペースの既定に戻す。@none なら「なし」"`
}

// SetProjectRuleRuns は、ワークスペースのルールを動かす・動かさないの本文。
type SetProjectRuleRuns struct {
	Runs bool `json:"runs"`
}

type statusInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Status  string `path:"status"`
	Body    SetProjectStatusUsed
}

type transitionInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	From    string `path:"from"`
	To      string `path:"to"`
	Body    SetProjectTransition
}

type templateInput struct {
	Store    string `path:"store"`
	Project  string `path:"project"`
	Template string `path:"template"`
	Body     SetProjectTemplateUsed
}

type templateDefaultInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Kind    string `path:"kind" enum:"Epic,Story,Task,Bug,Step"`
	Body    SetProjectTemplateDefault
}

type ruleInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Rule    string `path:"rule"`
	Body    SetProjectRuleRuns
}

type resetInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Section string `path:"section" doc:"statuses・workflow・templates・automation。ほかは見つからない"`
}

// Register は、プロジェクト別のルールの操作を登録する。
func Register(api huma.API, svc appprojectrules.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "set-project-status-used", Method: http.MethodPut, Path: rulesPath + "/statuses/{status}",
		Summary: "プロジェクトで使うステータスを変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *statusInput) (*struct{}, error) {
		return nil, apierr.From(svc.SetStatusUsed(ctx, port.StoreID(in.Store), project.ID(in.Project), change.ID(in.Status), in.Body.Used))
	})
	huma.Register(api, huma.Operation{
		OperationID: "set-project-transition", Method: http.MethodPut, Path: rulesPath + "/workflow/{from}/{to}",
		Summary: "プロジェクトの遷移の制約のセルを変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *transitionInput) (*struct{}, error) {
		return nil, apierr.From(svc.SetTransition(ctx, port.StoreID(in.Store), project.ID(in.Project),
			change.ID(in.From), change.ID(in.To), in.Body.Allowed))
	})
	huma.Register(api, huma.Operation{
		OperationID: "set-project-template-used", Method: http.MethodPut, Path: rulesPath + "/templates/{template}",
		Summary: "プロジェクトで使うテンプレートを変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *templateInput) (*struct{}, error) {
		return nil, apierr.From(svc.SetTemplateUsed(ctx, port.StoreID(in.Store), project.ID(in.Project), change.ID(in.Template), in.Body.Used))
	})
	huma.Register(api, huma.Operation{
		OperationID: "set-project-template-default", Method: http.MethodPut, Path: rulesPath + "/template-defaults/{kind}",
		Summary: "プロジェクトの種類ごとの既定のテンプレートを変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *templateDefaultInput) (*struct{}, error) {
		return nil, apierr.From(svc.SetTemplateDefault(ctx, port.StoreID(in.Store), project.ID(in.Project), in.Kind, in.Body.Template))
	})
	huma.Register(api, huma.Operation{
		OperationID: "set-project-rule-runs", Method: http.MethodPut, Path: rulesPath + "/automation/{rule}",
		Summary: "プロジェクトでワークスペースのルールを動かす・動かさないを変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *ruleInput) (*struct{}, error) {
		return nil, apierr.From(svc.SetRuleRuns(ctx, port.StoreID(in.Store), project.ID(in.Project), change.ID(in.Rule), in.Body.Runs))
	})
	huma.Register(api, huma.Operation{
		OperationID: "reset-project-rules", Method: http.MethodPost, Path: rulesPath + "/{section}/reset",
		Summary: "区画をワークスペースの設定に戻す", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *resetInput) (*struct{}, error) {
		return nil, apierr.From(svc.Reset(ctx, port.StoreID(in.Store), project.ID(in.Project), projectrule.Section(in.Section)))
	})
}
