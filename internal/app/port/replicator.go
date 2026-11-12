package port

import (
	"context"
	"errors"
)

// Replicator は、Store をほかの場所(remote)と合わせる口。git の版だけが持つ(設計書 3 節)。
// 受け取り、合わせ、送る。合わせたあとの修復は、app/sync が Stores.Write で保存する。
type Replicator interface {
	Sync(ctx context.Context, storeID StoreID, remote Remote) (SyncResult, error)
}

// Remote は合わせる相手。Token が空なら、認証なしで読み書きする。
type Remote struct {
	URL   string
	Token string
}

// SyncResult は、送った保存と受け取った保存の数(06「同期の結果」)。
type SyncResult struct {
	Sent     int
	Received int
}

// 同期を断る理由(06「同期のエラー」)。
// ErrNoCommonHistory・ErrRemoteNotStore・ErrRemoteNewerFormat は、最初の試みで返すときは、このマシンの Store を
// 変えない。ErrSendConflict のときと、2 回目からの試みで断るときは、前の試みで合わせた結果がこのマシンに残り、
// 次の同期で続ける。
var (
	ErrNoCommonHistory   = errors.New("the remote has no history in common with the store")
	ErrRemoteNotStore    = errors.New("the remote is not a hedgehug store")
	ErrRemoteNewerFormat = errors.New("the remote was saved by a newer hedgehug")
	ErrSendConflict      = errors.New("sending kept overlapping with other updates")
)
