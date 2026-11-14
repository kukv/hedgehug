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

// BulkTarget は、一括削除で選んだアイテム 1 つ。
type BulkTarget struct {
	Store string `json:"store" doc:"Store の ID"`
	Item  string `json:"item" doc:"アイテムの内部 ID"`
}

// BulkDeleteRequest は、一括削除の予告と実行の本文。
type BulkDeleteRequest struct {
	Items []BulkTarget `json:"items" minItems:"1" nullable:"false" doc:"選んだアイテム。複数の Store にまたがってよい"`
}

// BulkDeletePreview は、一括削除の確認ダイアログの中身(F2b1-10)。
type BulkDeletePreview struct {
	Stores        int `json:"stores" doc:"削除するアイテムのある Store の数"`
	RetentionDays int `json:"retentionDays" doc:"Store が 1 つのとき、その Store の削除済みの保持日数。ほかは 0"`
	ReadOnly      int `json:"readOnly" doc:"閲覧のみの Store にあるため削除しない、選んだ件数"`
}

// BulkDeleted は、一括削除の結果(F2b1-12)。
type BulkDeleted struct {
	Deleted  int `json:"deleted" doc:"書いた削除の行の数(下のものを含む)"`
	ReadOnly int `json:"readOnly" doc:"閲覧のみの Store にあるため削除しなかった、選んだ件数"`
}

type bulkInput struct{ Body BulkDeleteRequest }

type bulkPreviewOutput struct{ Body BulkDeletePreview }

type bulkDeletedOutput struct{ Body BulkDeleted }

func bulkTargets(req BulkDeleteRequest) []appitems.BulkTarget {
	out := make([]appitems.BulkTarget, 0, len(req.Items))
	for _, t := range req.Items {
		out = append(out, appitems.BulkTarget{Store: port.StoreID(t.Store), ID: item.ID(t.Item)})
	}
	return out
}

// registerBulk は、複数の Store をまたぐので /api/stores/{store} の下に置かない。
func registerBulk(api huma.API, svc appitems.Service) {
	huma.Register(api, huma.Operation{OperationID: "preview-bulk-delete", Method: http.MethodPost, Path: "/api/bulk-delete/preview", Summary: "選んだアイテムを一括削除するときの確認の中身を返す"},
		func(ctx context.Context, in *bulkInput) (*bulkPreviewOutput, error) {
			p, err := svc.BulkDeletePreview(ctx, bulkTargets(in.Body))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &bulkPreviewOutput{Body: BulkDeletePreview{Stores: p.Stores, RetentionDays: p.RetentionDays, ReadOnly: p.ReadOnly}}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "bulk-delete", Method: http.MethodPost, Path: "/api/bulk-delete", Summary: "選んだアイテムを、その下のものごと Store ごとに削除する"},
		func(ctx context.Context, in *bulkInput) (*bulkDeletedOutput, error) {
			d, err := svc.BulkDelete(ctx, bulkTargets(in.Body))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &bulkDeletedOutput{Body: BulkDeleted{Deleted: d.Deleted, ReadOnly: d.ReadOnly}}, nil
		})
}
