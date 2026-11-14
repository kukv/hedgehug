package port

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

// Replicator は、Store をほかの場所(remote・ミラー)と合わせる口。git の版だけが持つ(設計書 3 節)。
// 受け取り、合わせ、送る。合わせたあとの修復は、送る前に Sync の中で RepairFunc が保存する(決定 C3a-2)。
type Replicator interface {
	// Sync は、remote から受け取り、合わせ、送る(ReceiveOnly なら送らない)。
	Sync(ctx context.Context, storeID StoreID, remote Remote) (SyncResult, error)
	// SyncSave は、1 つのロックの中で、受け取り・修復・保存・送信を行う(02「保存」)。修復は Sync と同じく、合わせた
	// あと送る前に RepairFunc が保存する(決定 C3a-2)。受け取りか修復が失敗したら保存しない(fn を呼ばない)。
	// 修復の失敗は通信の失敗ではないので *SaveError で包み、中のエラーの種類(ErrNotFound など)は外から読めない形に
	// する(移動の断りや「ない」と読まれないため。Sync は修復の失敗を包まない)。送れなければ保存のコミットだけを
	// 捨てて、受け取って合わせた結果は残す。送信が他の人の更新と重なったら、保存を捨てて受け取り直し、fn を呼び直す
	// (最大 3 回)。そのため fn は試みごとに呼ばれ、
	// 毎回新しい Tx を受け取る(前の呼び出しの書き込みは残っていない)。fn は読む値を毎回その Tx から読む。
	// 保存の失敗(修復の失敗・fn のエラー・op.Check・コミットの失敗)は *SaveError、通信の失敗は Err* のどれか。
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

// SaveError は、SyncSave の保存の失敗(修復の失敗・fn のエラー・op.Check・コミットの失敗)。通信の失敗は包まない。
type SaveError struct{ Err error }

func (e *SaveError) Error() string { return "save: " + e.Err.Error() }
func (e *SaveError) Unwrap() error { return e.Err }

// ErrSaveNotDiscarded は、SyncSave で送れなかった保存のコミットを捨てられなかったこと。保存はこのマシンに残る。
var ErrSaveNotDiscarded = errors.New("the save that could not be sent was not discarded")

// RepairFunc は、同期で合わせたあと、送る前に呼ぶ修復(保存形式「同期の修復」)。まだ送っていない合わせが
// あるときだけ呼び、ReceiveOnly の同期では呼ばない。エラーを返すと、Sync はそのエラーを返して送らない。
type RepairFunc func(ctx context.Context, w RepairWriter) error

// RepairWriter は、修復が Store を読み書きする口。Sync が持っている Store のロックの中で動く
// (Stores.Read・Stores.Write を中から呼ぶと止まるので、こちらを使う)。
type RepairWriter interface {
	// Facts は、まだ送っていない合わせの、共通の祖先にない行の ID を、自分の側と remote の側に分けたもの。
	Facts() repair.Facts
	// Purged は、完全に削除したエンティティの内部 ID(purged.jsonl の行の target)。完全に削除したものは
	// ログもフォルダもないので Read では見えない。同じ内部 ID で作り直すと、次の同期の (1) で消されてしまうため、
	// 修復で内部 ID を決めるときに、これも使われているものとして扱う。
	Purged() (map[change.ID]bool, error)
	// Store は、修復を書いている Store の ID。転送記録の to_store に書く。
	Store() (StoreID, error)
	Read(fn func(r Reader) error) error
	// Write の約束は Stores.Write と同じ(行も説明の書き換えもなければ何も残さない、op.Check、fn のエラーで何も残さない)。
	// 1 回の Write が 1 つの保存になる。
	Write(op Operation, fn func(tx Tx) error) error
}

// EntityRows は、受け取った保存 1 件が 1 つのエンティティに書いたもの。
type EntityRows struct {
	Ref change.Ref
	// Rows は、その保存が足した行。
	Rows []change.Record
	// Description は、その保存で説明が前の版と違えば(新しく書いたときを含む)、保存の時点の説明。違わなければ nil。
	Description *string
}

// ReceivedSave は、通知の判定をまだしていない保存か合わせ 1 件(決定 C4b2-2)。受け取った保存と、このマシンが
// 書いた同期の修復の保存と、消えたエンティティに共通の祖先にない行がある合わせ。
type ReceivedSave struct {
	Key      SaveKey
	Entities []EntityRows
	// Repair は、同期の修復の保存(作者が hedgehug の bot で、足した行がすべて実行者 sync か、行がない)。
	Repair bool
	// Merge は、合わせ(2 つの版を合わせたもの)。Entities は空で、Key.Op も空。
	Merge bool
	// Removed は、その保存か合わせでフォルダごと消えたエンティティ。修復の保存と合わせのときだけ持つ。
	Removed []RemovedEntity
	// Concurrent は、合わせのときだけ、片側で親と一緒に削除され、もう片側で人が変えたエンティティ(決定 C4b2-17)。
	Concurrent []ConcurrentRows
	// DroppedPlan は、合わせのときだけ、合わせで取り除かれた(完全に削除した)スプリント・リリースの行(決定 C5a-8)。
	DroppedPlan []DroppedPlan
	// DroppedRules は、合わせのときだけ、合わせで取り除かれた、完全に削除したプロジェクトのルールの行(決定 C5b1-6)。
	DroppedRules []DroppedRules
	// Sides は、restore の行を書いた同期の修復の保存のときだけ、その保存の前を(ほかの修復の保存を通って)辿った
	// いちばん近い合わせで、共通の祖先にない行の ID を、その合わせの 2 つの側に分けたもの。見るのは、その保存が
	// 書いたエンティティのログだけ。合わせに届かなければ空(決定 C4b2-9)。
	Sides repair.Facts
}

// ConcurrentRows は、合わせの結果に残るエンティティ 1 つで、片側の共通の祖先にない行に root: false の削除があり、
// もう片側の共通の祖先にない行に人の変更(削除・取り消し以外)があるもの(決定 C4b2-17)。
type ConcurrentRows struct {
	Ref change.Ref
	// Deleted は、削除した側の、共通の祖先にない root: false の削除の行。
	Deleted []change.Record
	// Changed は、変えた側の、共通の祖先にない人の変更の行。
	Changed []change.Record
}

// DroppedPlan は、合わせの片側のワークスペースのログにあって、合わせの結果では取り除かれたスプリント・リリース
// 1 つの行(決定 C5a-8)。
type DroppedPlan struct {
	Workspace change.ID
	// Target は、スプリント・リリースの内部 ID。
	Target change.ID
	// Log は、その側のその Target の行。
	Log []change.Record
	// New は、Log のうち、共通の祖先のログにない行。
	New []change.Record
}

// DroppedRules は、合わせの片側のワークスペースのログにあって、合わせの結果では取り除かれた、完全に削除した
// プロジェクト 1 つのルールの行(project_rules/<P>/… と、scope が P の専用ルールの行。決定 C5b1-6)。
type DroppedRules struct {
	Workspace change.ID
	// Project は、完全に削除したプロジェクトの内部 ID。
	Project change.ID
	// Log は、その側のその Project のルールの行。
	Log []change.Record
	// New は、Log のうち、共通の祖先のログにない行。
	New []change.Record
}

// RemovedEntity は、保存か合わせで消えたエンティティ 1 つ(決定 C4b2-3)。
type RemovedEntity struct {
	Ref change.Ref
	// Log は、消える前のログ。保存なら前の版の、合わせならその側の版の。
	Log []change.Record
	// New は、合わせのときだけ、その側の行のうち、共通の祖先の同じエンティティのログにない行。
	New []change.Record
}

// Received は、判定の差し込み口に渡すもの。Saves は古い順(前の版が先。同じ深さは時刻、版の印の順)。
// ReadOnly は、閲覧のみの同期(受け取るだけで送らない)のとき。
type Received struct {
	Saves    []ReceivedSave
	ReadOnly bool
}

// JudgeFunc は、同期の最後に、受け取った保存・このマシンの修復の保存・合わせから通知を判定する差し込み口
// (決定 C4b1-2・C4b2-2)。read は、同期が持っている Store のロックの中で、Store のいまの状態を読む
// (Stores.Read を中から呼ぶと止まるので、こちらを使う)。エラーを返すと、渡した保存と合わせを判定済みに
// しない(同期の結果は変えない)。
type JudgeFunc func(ctx context.Context, store StoreID, in Received, read func(fn func(r Reader) error) error) error

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
