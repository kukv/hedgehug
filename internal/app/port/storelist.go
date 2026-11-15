package port

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// StoreInfo は、このマシンに取り付けた Store の 1 つ。名前・色・閲覧のみの印は、マシンごとの設定(01)。
type StoreInfo struct {
	ID       StoreID
	Name     string // Store 名(英数字・ハイフン・アンダースコア。08)
	Color    string // 画面のトークン --t-<色> の名前(例 "sky")
	ReadOnly bool
}

// StoreList は、表示中の Store の一覧の口。
type StoreList interface {
	// List は、表示中の Store を、サイドバーの並びで返す。非表示の Store は返さない。
	List(ctx context.Context) ([]StoreInfo, error)
}

// StoreVersions は、Store ごとのいまの版の印の口。保存のたびに、書き手を問わず変わる(06「画面への反映」)。
type StoreVersions interface {
	// Versions は、このマシンで開いているすべての Store の版の印。読めない Store は含めない。
	Versions(ctx context.Context) (map[StoreID]string, error)
}

// Commit は、保存のコミット 1 つ(C4d-2)。
type Commit struct {
	SHA     string
	Summary string
	Op      change.ID
	Author  string
	At      time.Time
	// Changes は、第 1 親との差分で足された行。
	Changes []EntityRows
	// Removed は、フォルダごと消えたエンティティ。
	Removed []change.Ref
	// StoreLog は、.hedgehug/log.jsonl を変えたか。
	StoreLog bool
}

// StoreCommits は、Store の保存のコミットの口(C4d-2)。git の版だけが持つ。
type StoreCommits interface {
	// Commits は、HEAD から辿れる、親が 1 つで Hedgehug-Op を持つコミットを、At の新しい順(同じなら SHA の辞書順)で返す。
	Commits(ctx context.Context, store StoreID) ([]Commit, error)
}
