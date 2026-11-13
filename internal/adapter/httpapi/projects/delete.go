package projects

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	appprojects "github.com/kukv/hedgehug/internal/app/projects"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// DeleteProjectPreview は、プロジェクトの削除の確認ダイアログの中身(A4b-7)。
type DeleteProjectPreview struct {
	Items         int `json:"items" doc:"下にある、削除されていないアイテムの数(Epic から Step まで全部)"`
	RetentionDays int `json:"retentionDays" doc:"その Store の削除済みの保持日数"`
}

type deletePreviewOutput struct{ Body DeleteProjectPreview }

func registerDelete(api huma.API, svc appprojects.Service) {
	const path = "/api/stores/{store}/projects/{project}"
	huma.Register(api, huma.Operation{OperationID: "get-project-delete-preview", Method: http.MethodGet, Path: path + "/delete-preview", Summary: "プロジェクトを削除したときに一緒に消えるアイテムの数と保持日数を返す"},
		func(ctx context.Context, in *getInput) (*deletePreviewOutput, error) {
			p, err := svc.DeletePreview(ctx, port.StoreID(in.Store), project.ID(in.Project))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &deletePreviewOutput{Body: DeleteProjectPreview{Items: p.Items, RetentionDays: p.RetentionDays}}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "delete-project", Method: http.MethodDelete, Path: path, Summary: "プロジェクトと、その下のアイテムを削除する", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *getInput) (*struct{}, error) {
			if err := svc.Delete(ctx, port.StoreID(in.Store), project.ID(in.Project)); err != nil {
				return nil, apierr.From(err)
			}
			return nil, nil //nolint:nilnil // 204 は本文なし
		})
}
