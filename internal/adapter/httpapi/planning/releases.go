package planning

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appplanning "github.com/kukv/hedgehug/internal/app/planning"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

type releaseInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Release   string `path:"release"`
}

type createReleaseInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Body      ReleaseInput
}

type updateReleaseInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Release   string `path:"release"`
	Body      ReleaseInput
}

type stateInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Release   string `path:"release"`
	Body      SetReleaseState
}

type (
	releaseCardsOutput struct{ Body []ReleaseCard }
	releaseOutput      struct{ Body Release }
)

func (in ReleaseInput) app() appplanning.ReleaseInput {
	return appplanning.ReleaseInput{Name: in.Name, Date: in.Date, Description: in.Description}
}

func registerReleases(api huma.API, svc appplanning.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "create-release", Method: http.MethodPost, Path: base + "/releases",
		Summary: "リリースを作成", DefaultStatus: http.StatusCreated,
	},
		func(ctx context.Context, in *createReleaseInput) (*releaseOutput, error) {
			if err := apierr.CheckDates(in.Body.Date); err != nil {
				return nil, err
			}
			store, ws := wsOf(in.Store, in.Workspace)
			r, err := svc.CreateRelease(ctx, store, ws, in.Body.app())
			if err != nil {
				return nil, apierr.From(err)
			}
			return &releaseOutput{Body: toRelease(r)}, nil
		})
	huma.Register(api, huma.Operation{
		OperationID: "update-release", Method: http.MethodPut, Path: base + "/releases/{release}",
		Summary: "リリースを編集", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *updateReleaseInput) (*struct{}, error) {
			if err := apierr.CheckDates(in.Body.Date); err != nil {
				return nil, err
			}
			store, ws, id := idsOf(in.Store, in.Workspace, in.Release)
			return nil, apierr.From(svc.UpdateRelease(ctx, store, ws, id, in.Body.app()))
		})
}

func registerReleaseStates(api huma.API, svc appplanning.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "set-release-state", Method: http.MethodPut, Path: base + "/releases/{release}/state",
		Summary: "リリースの状態を変える", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *stateInput) (*struct{}, error) {
			store, ws, id := idsOf(in.Store, in.Workspace, in.Release)
			return nil, apierr.From(svc.SetReleaseState(ctx, store, ws, id, planning.ReleaseState(in.Body.State)))
		})
	huma.Register(api, huma.Operation{
		OperationID: "delete-release", Method: http.MethodDelete, Path: base + "/releases/{release}",
		Summary: "リリースを削除", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *releaseInput) (*struct{}, error) {
			store, ws, id := idsOf(in.Store, in.Workspace, in.Release)
			return nil, apierr.From(svc.DeleteRelease(ctx, store, ws, id))
		})
	huma.Register(api, huma.Operation{
		OperationID: "restore-release", Method: http.MethodPost, Path: base + "/releases/{release}/restore",
		Summary: "削除したリリースを復元", DefaultStatus: http.StatusNoContent,
	},
		func(ctx context.Context, in *releaseInput) (*struct{}, error) {
			store, ws, id := idsOf(in.Store, in.Workspace, in.Release)
			return nil, apierr.From(svc.RestoreRelease(ctx, store, ws, id))
		})
}
