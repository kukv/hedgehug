package items

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// DeleteItemPreview は、アイテムの削除の確認ダイアログの中身(A4b-7)。
type DeleteItemPreview struct {
	Count         int      `json:"count" doc:"下にある、削除されていないアイテムの数"`
	Kinds         []string `json:"kinds" nullable:"false" doc:"下にあるものの種類。Epic・Story・Task・Bug・Step の順で重ねない"`
	RetentionDays int      `json:"retentionDays" doc:"その Store の削除済みの保持日数"`
}

type deletePreviewOutput struct{ Body DeleteItemPreview }

func registerDelete(api huma.API, svc appitems.Service) {
	huma.Register(api, huma.Operation{OperationID: "get-item-delete-preview", Method: http.MethodGet, Path: "/api/stores/{store}/items/{item}/delete-preview", Summary: "アイテムを削除したときに一緒に消えるものと保持日数を返す"},
		func(ctx context.Context, in *getInput) (*deletePreviewOutput, error) {
			p, err := svc.DeletePreview(ctx, port.StoreID(in.Store), item.ID(in.Item))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &deletePreviewOutput{Body: DeleteItemPreview{Count: p.Count, Kinds: []string{}, RetentionDays: p.RetentionDays}}
			for _, k := range p.Kinds {
				out.Body.Kinds = append(out.Body.Kinds, string(k))
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "delete-item", Method: http.MethodDelete, Path: "/api/stores/{store}/items/{item}", Summary: "アイテムと、その下のものを削除する", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *getInput) (*struct{}, error) {
			if err := svc.Delete(ctx, port.StoreID(in.Store), item.ID(in.Item)); err != nil {
				return nil, apierr.From(err)
			}
			return nil, nil //nolint:nilnil // 204 は本文なし
		})
}
