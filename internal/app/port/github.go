package port

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/domain/ghread"
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
	// ReadMany は、repo の numbers の PR・Issue を、100 件ずつまとめて読む(D4a-3)。改名・移管は Lookup と同じに追う。
	// 番号がなかったものは Batch.Missing に入れ、失敗にしない。リポジトリを読めなければ RepoUnreadable、
	// 利用上限で断られたら RateLimited。
	ReadMany(ctx context.Context, token, repo string, numbers []int) (Batch, error)
}

// Found は、GitHub から読んだ PR・Issue。
type Found struct {
	Kind   itemlink.Kind
	Repo   string // GitHub のいまの名前
	Number int
	itemlink.Values
	Checks itemlink.Checks // PR のチェックの結果。Issue は NoChecks
}

// Batch は、ReadMany の答え。ReadMany は GitHub のインターフェースに、実装の infra/github.Client と同時に足す。
type Batch struct {
	Repo    string        // GitHub のいまの名前
	Found   map[int]Found // 番号から引く
	Missing []int         // 答えになかった番号
	Rate    Rate          // 最後の答えの rateLimit
}

// Rate は、GitHub の利用上限(GraphQL の rateLimit)。
type Rate struct {
	Limit, Remaining int
	ResetAt          time.Time
}

// Checks は、このマシンが読んだ PR のチェックの結果(07「表示と更新」。Store に保存しない)の口。
// 実装は infra/statedb。
type Checks interface {
	// Read は、Store のリンクのチェックの結果を、Ref.Key() から引く表で返す。
	Read(ctx context.Context, store StoreID) (map[string]itemlink.Checks, error)
	// Save は、結果を置く。NoChecks なら消す。前と変わったら changed。
	Save(ctx context.Context, store StoreID, key string, c itemlink.Checks) (changed bool, err error)
}

// GitHubReads は、GitHub を読んだ記録と、取り直しの失敗の文言の口(D4a-5・D4a-11)。実装は infra/statedb。
type GitHubReads interface {
	// Records は、Store のリポジトリ(小文字)から引く、前回の読み取りの記録。
	Records(ctx context.Context, store StoreID) (map[string]ghread.Record, error)
	// SaveRecord は、repo(小文字)の記録を置く。
	SaveRecord(ctx context.Context, store StoreID, repo string, r ghread.Record) error
	// Failures は、Store のリンクの Ref.Key() から引く、取り直しの失敗の文言。
	Failures(ctx context.Context, store StoreID) (map[string]string, error)
	// SetFailure は、文言を置く。空なら消す。前と変わったら changed。
	SetFailure(ctx context.Context, store StoreID, key, text string) (changed bool, err error)
}
