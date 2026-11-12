package syncing

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
)

// NotifyWrites は、Write が成功するたびに onWrite を呼ぶ port.Stores を返す(「書き込み後」の契機。
// 決定 C2-19)。同期の修復の保存(C3)は、包む前の Stores で書く(修復で次の同期を呼ばないため)。
func NotifyWrites(s port.Stores, onWrite func(port.StoreID)) port.Stores {
	return notifying{Stores: s, onWrite: onWrite}
}

type notifying struct {
	port.Stores
	onWrite func(port.StoreID)
}

func (n notifying) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	if err := n.Stores.Write(ctx, id, op, fn); err != nil {
		return err
	}
	n.onWrite(id)
	return nil
}
