package port

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// Recent は、最近開いたアイテム 1 件(どの Store のどのアイテムを、いつ開いたか)。
type Recent struct {
	Store StoreID
	Item  change.ID
	At    time.Time
}

// RecentItems は、このマシンで最近開いたアイテム(保存形式「このマシンに持つもの」)。
type RecentItems interface {
	// Touch は、item を開いたと記録する(すでにあれば Store と時刻を替える)。At の新しい順で keep 件を超えた記録は消す。
	Touch(ctx context.Context, store StoreID, item change.ID, at time.Time, keep int) error
	// List は、記録を At の新しい順で返す。
	List(ctx context.Context) ([]Recent, error)
	// Forget は、items の記録を消す。なければ何もしない。
	Forget(ctx context.Context, items []change.ID) error
}

// SyncRecord は、1 つの Store の、このマシンでの同期の記録。
type SyncRecord struct {
	Store       StoreID
	LastAttempt time.Time         // 最後に試みた時刻(成功・失敗を問わない)。なければゼロ
	LastSuccess time.Time         // 最後に成功した時刻。なければゼロ
	Reason      syncstatus.Reason // 最後の試みが失敗なら、その理由。成功なら空
}

// MirrorRecord は、1 つのミラーへの、最後の送信の結果。
type MirrorRecord struct {
	Mirror string            // ミラーの URL
	At     time.Time         // 最後に送ろうとした時刻
	Reason syncstatus.Reason // 送れなかった理由。送れたら空
}

// SyncState は、このマシンの同期の状態(保存形式「このマシンに持つもの」)。
type SyncState interface {
	// Records は、記録のあるすべての Store を、Store ID の順で返す。
	Records(ctx context.Context) ([]SyncRecord, error)
	// Failed は、失敗した試みを記録する。最後に成功した時刻と、前に受け取った位置は変えない。
	Failed(ctx context.Context, store StoreID, at time.Time, reason syncstatus.Reason) error
	// Succeeded は、成功を記録し(理由を消す)、remoteKey の remote から受け取った位置 seen を覚える。
	Succeeded(ctx context.Context, store StoreID, at time.Time, remoteKey, seen string) error
	// Seen は、remoteKey の remote から前に受け取った位置。記録がないか、覚えた remote が違えば空。
	Seen(ctx context.Context, store StoreID, remoteKey string) (string, error)
	// Mirrored は、ミラーへの送信の結果を記録する。reason が空なら送れた。
	Mirrored(ctx context.Context, store StoreID, mirror string, at time.Time, reason syncstatus.Reason) error
	// Mirrors は、Store のミラーごとの最後の結果を、URL の順で返す。
	Mirrors(ctx context.Context, store StoreID) ([]MirrorRecord, error)
}

// SaveKey は、同期で受け取った保存 1 件の見分け方。
type SaveKey struct {
	Version string // 保存の版の印
	Author  string
	At      time.Time
	Op      change.ID // 合わせ・最初の保存は空
}

// Match は、保存が通知の判定を済ませたものと合うか、の結果。
type Match int

const (
	NotJudged   Match = iota // 判定していない
	SameVersion              // 版の印が同じ。ここで辿るのを止める
	SameSave                 // 操作・作者・時刻の組だけが同じ。この保存は外して辿り続ける
)

// JudgedSaves は、通知の判定を済ませた保存の記録(保存形式「このマシンに持つもの」)。
type JudgedSaves interface {
	// Started は、store の記録が 1 件でもあるか。
	Started(ctx context.Context, store StoreID) (bool, error)
	// Known は、k が記録のどれと合うかを返す。版の印が同じなら SameVersion。そうでなく Op が空でなく、
	// 同じ Store で Op・Author・At が同じ記録があれば SameSave。
	Known(ctx context.Context, store StoreID, k SaveKey) (Match, error)
	// Mark は、keys をまとめて 1 回で記録する。すでにあるものは重ねない。
	Mark(ctx context.Context, store StoreID, keys []SaveKey) error
	// ForgetStore は、store の記録をすべて消す。
	ForgetStore(ctx context.Context, store StoreID) error
}
