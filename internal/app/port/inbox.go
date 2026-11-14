package port

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// NoticeKind は、通知の種類(06「通知の種類」)。
type NoticeKind string

const (
	NoticeAutomation        NoticeKind = "automation"         // 自動化が変えた。ステータスの削除でルールが変わった
	NoticeAutomationStopped NoticeKind = "automation_stopped" // 自動化が制約で止まった
	NoticeMention           NoticeKind = "mention"            // コメントでメンションされた
	NoticeAssign            NoticeKind = "assign"             // アイテムの担当にされた
	NoticeWatch             NoticeKind = "watch"              // ウォッチしているアイテムが変わった
	NoticeOverwrite         NoticeKind = "overwrite"          // 同時の編集で、あなたの変更が上書きされた
	NoticeRepair            NoticeKind = "repair"             // 修復でデータが直された
)

// Notice は、あなた宛ての通知 1 件。差出人は種類で決まる(自動化は「hedgehug」)。
type Notice struct {
	Kind      NoticeKind
	Store     StoreID
	Workspace workspace.ID
	Project   project.ID // プロジェクトを指す通知の対象。アイテムの通知では空
	Item      item.ID    // 対象のアイテム。ルールが変わった通知では空(開く先は設定の「自動化」)
	Rule      change.ID  // 自動化のルール
	Text      string     // 06 の本文
	Reason    string     // 止まった理由(NoticeAutomationStopped だけ)
	Status    change.ID  // 止まった実行が変えようとしたステータス(「制約を無視して…」に使う)
	From      change.ID  // 止まったときのステータス
	Display   string     // 作ったときのアイテムの ID。アイテムがなければ空
	Sender    string     // 差出人のログイン名。空は「hedgehug」
	Source    string     // メンションの本文の鍵。メンション以外は空
	At        time.Time  // Mention が既存の行を書き換えるときに刻む時刻(本文を保存した時刻)。足すときと Add は使わない(受信箱の時計)
}

// Inbox は、受信箱の口(B4-12)。通知を足す。
type Inbox interface {
	Add(ctx context.Context, n Notice) error
	// Mention は、同じ Store・Source の行があれば text・sender・at(n.At)を書き換えて未読に戻し、
	// なければ Add と同じ受信箱の時計の時刻(受け取った時刻)で足す。
	// n.Source が空ならエラー。
	Mention(ctx context.Context, n Notice) error
}

// NoticeID は、通知 1 件の番号(受信箱の中で一意)。
type NoticeID int64

// NoticeResult は、通知の操作の結果。空は「まだ操作していない」。
type NoticeResult string

const (
	NoticeApplied NoticeResult = "applied" // 通知から操作を実行した
	NoticeKept    NoticeResult = "kept"    // 通知から「そのままにする」を選んだ
)

// NoticeRow は、保存した通知 1 件。
type NoticeRow struct {
	ID NoticeID
	Notice
	At     time.Time // Add と Mention が足したときに刻んだ時刻。Mention が書き換えた行は n.At(Notice.At を隠す)
	Read   bool
	Result NoticeResult
}

// NoticeCursor は、一覧の続きの位置(この行より古い行から返す)。
type NoticeCursor struct {
	At time.Time
	ID NoticeID
}

// NoticeQuery は、一覧の条件。並びは At と ID の新しい順。
type NoticeQuery struct {
	Stores []StoreID
	Kinds  []NoticeKind // 空はすべて
	Before *NoticeCursor
	Limit  int
}

// InboxBook は、保存した通知を読み書きする口。
type InboxBook interface {
	Inbox
	// List は、条件に合う通知を新しい順に返す。Stores が空なら空。
	List(ctx context.Context, q NoticeQuery) ([]NoticeRow, error)
	// Unread は、未読の件数を種類ごとに返す。
	Unread(ctx context.Context, stores []StoreID) (map[NoticeKind]int, error)
	// Get は、通知 1 件を返す。なければ ErrNotFound。
	Get(ctx context.Context, id NoticeID) (NoticeRow, error)
	// SetRead は、既読・未読を書く。なければ ErrNotFound。
	SetRead(ctx context.Context, id NoticeID, read bool) error
	// ReadAll は、Stores の kinds(空はすべて)の通知をすべて既読にする。
	ReadAll(ctx context.Context, stores []StoreID, kinds []NoticeKind) error
	// Resolve は、既読にして結果を書く。なければ ErrNotFound、結果がすでにあれば断り。
	Resolve(ctx context.Context, id NoticeID, r NoticeResult) error
	// Prune は、アイテムが targets にある通知と、プロジェクトが targets にある通知と、
	// アイテムもプロジェクトもない通知のうちワークスペースが targets にあるものを消す。
	// 通知のワークスペースは作ったときの値なので、アイテムとプロジェクトの通知はワークスペースでは消さない(移ったアイテムの通知を残す)。
	Prune(ctx context.Context, store StoreID, targets []change.ID) error
	// ForgetStore は、Store の通知をすべて消す。
	ForgetStore(ctx context.Context, store StoreID) error
}
