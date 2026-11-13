package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// GitHub は GitHub の API の口。失敗は *ghcall.Failure で返す。使う操作だけを持つ。
type GitHub interface {
	// Viewer は、token の持ち主のログイン名を返す(07「認証の確認」)。
	Viewer(ctx context.Context, token string) (login string, err error)
	// Lookup は、repo(owner/name)の番号 number の PR か Issue を読む(07「追加」「表示と更新」)。
	// 改名・移管されたリポジトリは移動先で読み、Found.Repo に移動先の名前を入れる。
	// リポジトリを読めなければ RepoUnreadable、番号がなければ NotFound(RepoUnreadable は Repo 付き、NotFound は Repo・Number 付き)。
	Lookup(ctx context.Context, token, repo string, number int) (Found, error)
}

// Found は、GitHub から読んだ PR・Issue。
type Found struct {
	Kind   itemlink.Kind
	Repo   string // GitHub のいまの名前
	Number int
	itemlink.Values
	Checks itemlink.Checks // PR のチェックの結果。Issue は NoChecks
}

// Checks は、このマシンが読んだ PR のチェックの結果(07「表示と更新」。Store に保存しない)の口。
// 実装は infra/statedb。
type Checks interface {
	// Read は、Store のリンクのチェックの結果を、Ref.Key() から引く表で返す。
	Read(ctx context.Context, store StoreID) (map[string]itemlink.Checks, error)
	// Save は、結果を置く。NoChecks なら消す。前と変わったら changed。
	Save(ctx context.Context, store StoreID, key string, c itemlink.Checks) (changed bool, err error)
}
