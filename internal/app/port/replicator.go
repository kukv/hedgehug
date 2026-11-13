package port

import (
	"context"
	"errors"
)

// Replicator は、Store をほかの場所(remote・ミラー)と合わせる口。git の版だけが持つ(設計書 3 節)。
// 受け取り、合わせ、送る。合わせたあとの修復は、app/syncing が Stores.Write で保存する。
type Replicator interface {
	// Sync は、remote から受け取り、合わせ、送る(ReceiveOnly なら送らない)。
	Sync(ctx context.Context, storeID StoreID, remote Remote) (SyncResult, error)
	// SyncSave は、1 つのロックの中で、受け取り・保存・送信を行う(02「保存」)。受け取りが失敗したら保存しない
	// (fn を呼ばない)。送れなければ保存のコミットだけを捨てて、受け取って合わせた結果は残す。送信が他の人の
	// 更新と重なったら、保存を捨てて受け取り直し、fn を呼び直す(最大 3 回)。そのため fn は試みごとに呼ばれ、
	// 毎回新しい Tx を受け取る(前の呼び出しの書き込みは残っていない)。fn は読む値を毎回その Tx から読む。
	// 保存の失敗(fn のエラー・op.Check・コミットの失敗)は *SaveError、通信の失敗は Err* のどれか。
	// 送れなかった保存のコミットを捨てられなかったときは、送信のエラーに ErrSaveNotDiscarded を Join して返す
	// (やり直さない。保存は残り、次の同期で送られるかもしれない)。
	// ReceiveOnly の remote には保存せず、ErrInvalidOperation を返す。
	SyncSave(ctx context.Context, storeID StoreID, remote Remote, op Operation, fn func(tx Tx) error) (SyncResult, error)
	// Mirror は、Store の保存をミラーへ送るだけで、受け取らない(保存形式「同期」の手順 7)。
	// ミラーが空なら最初の送信として送る。共通の履歴がなければ ErrMirrorUnrelated、ミラーにだけ保存が
	// あれば ErrMirrorAhead。どちらも送らない。
	Mirror(ctx context.Context, storeID StoreID, mirror Remote) error
	// Unsent は、まだ remote に送っていない保存の数(06「同期の状態の表示」の未送信)。
	Unsent(ctx context.Context, storeID StoreID) (int, error)
}

// Remote は合わせる相手。Token が空なら、認証なしで読み書きする。
type Remote struct {
	URL   string
	Token string
	// Seen は、前の同期で同じ remote から受け取った位置(前の SyncResult.Seen)。空でなく、remote の履歴が
	// その続きでなければ、Sync は ErrRemoteRewritten を返す。空なら確かめない。
	Seen string
	// ReceiveOnly なら、受け取って合わせるだけで、送らない(閲覧のみの Store)。
	ReceiveOnly bool
}

// SaveError は、SyncSave の保存の失敗(fn のエラー・op.Check・コミットの失敗)。通信の失敗は包まない。
type SaveError struct{ Err error }

func (e *SaveError) Error() string { return "save: " + e.Err.Error() }
func (e *SaveError) Unwrap() error { return e.Err }

// ErrSaveNotDiscarded は、SyncSave で送れなかった保存のコミットを捨てられなかったこと。保存はこのマシンに残る。
var ErrSaveNotDiscarded = errors.New("the save that could not be sent was not discarded")

// SyncResult は、送った保存と受け取った保存の数(06「同期の結果」)と、今回受け取った位置。
// Seen は、次の同期の Remote.Seen に渡す。remote が空のまま終わったら空。
type SyncResult struct {
	Sent     int
	Received int
	Seen     string
}

// 同期を断る理由(06「同期のエラー」)。
// ErrNoCommonHistory・ErrRemoteNotStore・ErrRemoteNewerFormat・ErrRemoteRewritten は、最初の試みで返すときは、
// このマシンの Store を変えない。ErrSendConflict のときと、2 回目からの試みで断るときは、前の試みで合わせた
// 結果がこのマシンに残り、次の同期で続ける。
var (
	ErrNoCommonHistory   = errors.New("the remote has no history in common with the store")
	ErrRemoteNotStore    = errors.New("the remote is not a hedgehug store")
	ErrRemoteNewerFormat = errors.New("the remote was saved by a newer hedgehug")
	ErrSendConflict      = errors.New("sending kept overlapping with other updates")
	ErrRemoteRewritten   = errors.New("the remote history was rewritten")
)

// remote との通信の失敗(06「同期のエラー」)。ミラーへの送信でも同じものを返す。
var (
	ErrAuthFailed       = errors.New("the remote refused the credentials")     // 401
	ErrRemoteUnreadable = errors.New("the remote cannot be read")              // 404、受け取りの 403
	ErrWriteDenied      = errors.New("the remote refused writing")             // 送信の 403
	ErrUnreachable      = errors.New("the remote cannot be reached")           // つながらない、途中で切れた、60 秒進まない
	ErrMirrorUnrelated  = errors.New("the mirror has no history in common")    // ミラーが空でなく、共通の履歴がない
	ErrMirrorAhead      = errors.New("the mirror has saves the store has not") // ミラーにだけ保存がある
)
