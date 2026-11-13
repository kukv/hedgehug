package transfers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	apptransfers "github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// LocatedMovedFrom は、旧 ID から開いた印(A5a-15・A5a-16)。文は画面が作る。
type LocatedMovedFrom struct {
	Old     string `json:"old" doc:"旧 ID(HH-2)"`
	At      string `json:"at" doc:"旧 ID が一致した記録の日時"`
	Display string `json:"display" doc:"旧 ID が一致した記録の新 ID"`
	Current string `json:"current" doc:"開くアイテムのいまの ID"`
}

// Located は、画面のアドレスの ID の解決の結果。kind が found なら開くアイテム、gone なら開けない理由と材料
// (文は画面が作る)。
type Located struct {
	Kind       string            `json:"kind" enum:"found,gone"`
	StoreID    string            `json:"storeId,omitempty" doc:"found のとき。開くアイテムの Store"`
	ProjectID  string            `json:"projectId,omitempty" doc:"found のとき"`
	ItemID     string            `json:"itemId,omitempty" doc:"found のとき"`
	MovedFrom  *LocatedMovedFrom `json:"movedFrom,omitempty" doc:"found で、転送記録から見つけたとき"`
	Reason     string            `json:"reason,omitempty" enum:"deleted,purged,hidden,elsewhere" doc:"gone のとき。deleted は移した先で削除済み、purged は完全に削除、hidden は移動先の Store が表示中でない、elsewhere はそのあと表示中でない Store へ移した"`
	Old        string            `json:"old,omitempty" doc:"gone のとき。旧 ID"`
	At         string            `json:"at,omitempty" doc:"gone のとき。旧 ID が一致した記録の日時"`
	NewDisplay string            `json:"newDisplay,omitempty" doc:"gone のとき。旧 ID が一致した記録の新 ID"`
	StoreName  string            `json:"storeName,omitempty" doc:"gone の hidden・elsewhere のとき。表示中でない Store の名前"`
}

type locateInput struct {
	Store string `query:"store" required:"true" doc:"画面のアドレスの Store 名"`
	ID    string `query:"id" required:"true" doc:"画面のアドレスの ID(HH-2)"`
}

type locateOutput struct{ Body Located }

func registerLocate(api huma.API, svc apptransfers.Service) {
	huma.Register(api, huma.Operation{OperationID: "locate-item", Method: http.MethodGet, Path: "/api/locate", Summary: "画面のアドレスの ID(旧 ID を含む)が指すアイテムを探す"},
		func(ctx context.Context, in *locateInput) (*locateOutput, error) {
			l, err := svc.Locate(ctx, in.Store, in.ID)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &locateOutput{Body: toLocated(l)}, nil
		})
}

func toLocated(l apptransfers.Located) Located {
	if !l.Found {
		g := l.Gone
		return Located{Kind: "gone", Reason: g.Reason, Old: g.Old, At: g.At.Format(change.TimeLayout), NewDisplay: g.NewDisplay, StoreName: g.StoreName}
	}
	out := Located{Kind: "found", StoreID: string(l.Store), ProjectID: string(l.Project), ItemID: string(l.Item)}
	if m := l.MovedFrom; m != nil {
		out.MovedFrom = &LocatedMovedFrom{Old: m.Old, At: m.At.Format(change.TimeLayout), Display: m.Display, Current: m.Current}
	}
	return out
}
