package port

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghread"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/prlink"
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
	// CheckRepo は、repo を読めるかを確かめる(「再確認」)。読めれば nil。失敗は Lookup と同じ種類
	// (読めなければ RepoUnreadable、GitHub App が入っていなければ AppNotInstalled)。
	CheckRepo(ctx context.Context, token, repo string) error
	// ReadPulls は repo の PR を更新の新しい順に読む。since がゼロなら最初の 100 件だけ、そうでなければ
	// UpdatedAt が since 以上のものを最後まで(100 件ずつめくる)。改名・移管は Lookup と同じに追う。
	// めくっている間に PR が更新されると、同じ番号が 2 回返ることがある。
	// 失敗は Lookup と同じ種類(App 未導入の見分けを含む)。
	ReadPulls(ctx context.Context, token, repo string, since time.Time) (Pulls, error)
}

// Pull は、自動リンクのために読んだ PR(07「PR の自動リンク」)。
type Pull struct {
	Number int
	Text   prlink.Text // ブランチ名・タイトル・本文・コメント(古い順に 100 件まで)
	itemlink.Values
	Checks    itemlink.Checks
	UpdatedAt time.Time // GitHub の時刻
}

// Pulls は、ReadPulls の答え。Repo は GitHub の nameWithOwner。Rate は rateLimit のあった最後のページの値。
type Pulls struct {
	Repo  string
	Pulls []Pull
	Rate  Rate
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

// ScanRecord は、Store のリポジトリ 1 つの、PR を探した記録(D4b-4)。ゼロ値は「まだ探していない」。
type ScanRecord struct {
	Read    bool      // 一度でも探し終えた
	Since   time.Time // 見た PR の updatedAt の最大(GitHub の時刻)
	Failure string    // 前回の失敗の文言(07 の表の文。空なら成功)
}

// PRScans は、PR を探した記録と、リポジトリの改名の対応の口(D4b-4、D4b 最終レビュー R-1)。実装は infra/statedb。
type PRScans interface {
	// Scans は、Store のリポジトリ(小文字)から引く記録を返す。
	Scans(ctx context.Context, store StoreID) (map[string]ScanRecord, error)
	// SaveScan は、repo の記録を、いまの記録と合わせて置く(同時に動いた探索が古い読み取りで書いても戻らないように):
	// Read は論理和、Since は大きい方、Failure は r のもの。合わせた値が前と変わったら changed。
	SaveScan(ctx context.Context, store StoreID, repo string, r ScanRecord) (changed bool, err error)
	// Renames は、Store のリポジトリの改名・移管の対応(旧い名前の小文字 → いまの名前の小文字)を返す。
	Renames(ctx context.Context, store StoreID) (map[string]string, error)
	// SaveRename は、from が to に改名・移管されたことを小文字で記録する。大文字小文字だけの違いは記録しない。
	// to から先の対応は消す(元の名前に戻す改名で輪にしない)。
	SaveRename(ctx context.Context, store StoreID, from, to string) error
}

// MissingOrg は、App 未導入の組織の記録(D4b-12)。
type MissingOrg struct {
	Org  string // 表示する組織名(最初に断られたリポジトリの owner の書き方)
	Repo string // 断られたリポジトリ(「再確認」で読む)
}

// AppMissing は、GitHub App 未導入の組織の記録の口(D4b-12)。実装は infra/statedb。
type AppMissing interface {
	// List は、アカウントごとの記録を、記録した順で返す。
	List(ctx context.Context) (map[account.ID][]MissingOrg, error)
	// Put は記録する。同じ組織(小文字)なら Repo だけ置き換え、changed は false。
	Put(ctx context.Context, a account.ID, m MissingOrg) (changed bool, err error)
	// Clear は、記録した組織 org のリポジトリが repo のときだけ記録を消す(同じ組織のほかのリポジトリが読めても
	// 消さない)。org と repo は大文字と小文字を区別しない。
	Clear(ctx context.Context, a account.ID, org, repo string) (changed bool, err error)
	// Forget は、アカウント a の記録をすべて消す(アカウントの削除)。
	Forget(ctx context.Context, a account.ID) error
}
