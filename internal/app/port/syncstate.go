package port

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

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
