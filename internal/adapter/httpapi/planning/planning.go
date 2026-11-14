// Package planning は、スプリントとリリースの API(仕様 05「スプリント」「リリース」)。
package planning

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appplanning "github.com/kukv/hedgehug/internal/app/planning"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const base = "/api/stores/{store}/workspaces/{workspace}"

type wsInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
}

type sprintInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Sprint    string `path:"sprint"`
}

type createSprintInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Body      SprintInput
}

type updateSprintInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Sprint    string `path:"sprint"`
	Body      SprintInput
}

type startInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Sprint    string `path:"sprint"`
	Body      StartSprint
}

type closeInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Sprint    string `path:"sprint"`
	Body      CloseSprint
}

type (
	overviewOutput    struct{ Body PlanningOverview }
	sprintCardsOutput struct{ Body []SprintCard }
	sprintOutput      struct{ Body Sprint }
)

func wsOf(store, ws string) (port.StoreID, workspace.ID) {
	return port.StoreID(store), workspace.ID(ws)
}

func idsOf(store, ws, id string) (port.StoreID, workspace.ID, change.ID) {
	return port.StoreID(store), workspace.ID(ws), change.ID(id)
}

func (in SprintInput) app() appplanning.SprintInput {
	return appplanning.SprintInput{Name: in.Name, Start: in.Start, End: in.End, Goal: in.Goal}
}

// Register は、スプリントとリリース、アイテムのスプリントとリリースの API を登録する。
func Register(api huma.API, svc appplanning.Service) {
	registerReads(api, svc)
	registerSprints(api, svc)
	registerSprintStates(api, svc)
	registerReleases(api, svc)
	registerReleaseStates(api, svc)
	registerItems(api, svc)
}

func registerReads(api huma.API, svc appplanning.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-planning", Method: http.MethodGet, Path: base + "/planning",
		Summary: "スプリントとリリースの選択肢、実行中のスプリント、作成の既定",
	},
		func(ctx context.Context, in *wsInput) (*overviewOutput, error) {
			store, ws := wsOf(in.Store, in.Workspace)
			v, err := svc.Overview(ctx, store, ws)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &overviewOutput{Body: toOverview(v)}, nil
		})
	huma.Register(api, huma.Operation{
		OperationID: "list-sprint-cards", Method: http.MethodGet, Path: base + "/sprints",
		Summary: "スプリントのカード",
	},
		func(ctx context.Context, in *wsInput) (*sprintCardsOutput, error) {
			store, ws := wsOf(in.Store, in.Workspace)
			cs, err := svc.SprintCards(ctx, store, ws)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &sprintCardsOutput{Body: toSprintCards(cs)}, nil
		})
	huma.Register(api, huma.Operation{
		OperationID: "list-release-cards", Method: http.MethodGet, Path: base + "/releases",
		Summary: "リリースのカード",
	},
		func(ctx context.Context, in *wsInput) (*releaseCardsOutput, error) {
			store, ws := wsOf(in.Store, in.Workspace)
			cs, err := svc.ReleaseCards(ctx, store, ws)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &releaseCardsOutput{Body: toReleaseCards(cs)}, nil
		})
}

func registerSprints(api huma.API, svc appplanning.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "create-sprint", Method: http.MethodPost, Path: base + "/sprints",
		Summary: "スプリントを作成", DefaultStatus: http.StatusCreated,
	},
		func(ctx context.Context, in *createSprintInput) (*sprintOutput, error) {
			if err := apierr.CheckDates(in.Body.Start, in.Body.End); err != nil {
				return nil, err
			}
			store, ws := wsOf(in.Store, in.Workspace)
			sp, err := svc.CreateSprint(ctx, store, ws, in.Body.app())
			if err != nil {
				return nil, apierr.From(err)
			}
			return &sprintOutput{Body: toSprint(sp)}, nil
		})
	huma.Register(api, huma.Operation{
		OperationID: "update-sprint", Method: http.MethodPut, Path: base + "/sprints/{sprint}",
		Summary: "スプリントを編集", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *updateSprintInput) (*struct{}, error) {
			if err := apierr.CheckDates(in.Body.Start, in.Body.End); err != nil {
				return nil, err
			}
			store, ws, id := idsOf(in.Store, in.Workspace, in.Sprint)
			return nil, apierr.From(svc.UpdateSprint(ctx, store, ws, id, in.Body.app()))
		})
	huma.Register(api, huma.Operation{
		OperationID: "delete-sprint", Method: http.MethodDelete, Path: base + "/sprints/{sprint}",
		Summary: "スプリントを削除", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *sprintInput) (*struct{}, error) {
			store, ws, id := idsOf(in.Store, in.Workspace, in.Sprint)
			return nil, apierr.From(svc.DeleteSprint(ctx, store, ws, id))
		})
	huma.Register(api, huma.Operation{
		OperationID: "restore-sprint", Method: http.MethodPost, Path: base + "/sprints/{sprint}/restore",
		Summary: "削除したスプリントを復元", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *sprintInput) (*struct{}, error) {
			store, ws, id := idsOf(in.Store, in.Workspace, in.Sprint)
			return nil, apierr.From(svc.RestoreSprint(ctx, store, ws, id))
		})
}

func registerSprintStates(api huma.API, svc appplanning.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "start-sprint", Method: http.MethodPost, Path: base + "/sprints/{sprint}/start",
		Summary: "計画中のスプリントを開始", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *startInput) (*struct{}, error) {
			if err := apierr.CheckDates(in.Body.Start, in.Body.End); err != nil {
				return nil, err
			}
			store, ws, id := idsOf(in.Store, in.Workspace, in.Sprint)
			return nil, apierr.From(svc.StartSprint(ctx, store, ws, id, in.Body.Start, in.Body.End))
		})
	huma.Register(api, huma.Operation{
		OperationID: "close-sprint", Method: http.MethodPost, Path: base + "/sprints/{sprint}/close",
		Summary: "実行中のスプリントを完了し、未完了のアイテムを移す", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *closeInput) (*struct{}, error) {
			store, ws, id := idsOf(in.Store, in.Workspace, in.Sprint)
			return nil, apierr.From(svc.CloseSprint(ctx, store, ws, id, ref(in.Body.Destination)))
		})
}
