package transfers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/port"
	apptransfers "github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// TransferItem は、移動の本文(A5a-19)。
type TransferItem struct {
	ToStore       string            `json:"toStore" doc:"移動先の Store の ID"`
	ToWorkspace   string            `json:"toWorkspace" doc:"移動先のワークスペースの内部 ID"`
	ToProject     string            `json:"toProject" doc:"移動先のプロジェクトの内部 ID"`
	ToParent      string            `json:"toParent,omitempty" doc:"移動先の親(Epic)の内部 ID。省略するとプロジェクトの直下(Epic は省略する)"`
	StatusChoices map[string]string `json:"statusChoices" doc:"ステータスの置き換え(移動元のステータスの内部 ID → 移動先のステータスの内部 ID)"`
}

// TransferResult は、移動の結果。
type TransferResult struct {
	ToStore string   `json:"toStore" doc:"移動先の Store の ID"`
	ItemID  string   `json:"itemId" doc:"移したアイテムの内部 ID(別の Store へ移しても変わらない)"`
	Display string   `json:"display" doc:"移したアイテムの新しい ID(MB-2)"`
	Emptied []string `json:"emptied" doc:"合う値がなく空になった項目の名前(プロパティの定義の順)"`
}

type transferInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  TransferItem
}

type transferOutput struct{ Body TransferResult }

func registerMove(api huma.API, svc apptransfers.Service) {
	huma.Register(api, huma.Operation{OperationID: "transfer-item", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/transfer", Summary: "アイテムを別のワークスペース・別の Store へ移す"},
		func(ctx context.Context, in *transferInput) (*transferOutput, error) {
			m, err := svc.Move(ctx, port.StoreID(in.Store), item.ID(in.Item), toMoveInput(in.Body))
			if err != nil {
				return nil, moveError(err)
			}
			emptied := m.Emptied
			if emptied == nil {
				emptied = []string{}
			}
			return &transferOutput{Body: TransferResult{ToStore: string(m.ToStore), ItemID: string(m.Item), Display: m.Display, Emptied: emptied}}, nil
		})
}

func toMoveInput(b TransferItem) apptransfers.MoveInput {
	in := apptransfers.MoveInput{
		ToStore: port.StoreID(b.ToStore), ToWorkspace: workspace.ID(b.ToWorkspace), ToProject: project.ID(b.ToProject),
		StatusChoices: map[change.ID]change.ID{},
	}
	if b.ToParent != "" {
		p := item.ID(b.ToParent)
		in.ToParent = &p
	}
	for from, to := range b.StatusChoices {
		in.StatusChoices[change.ID(from)] = change.ID(to)
	}
	return in
}
