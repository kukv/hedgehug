// Package ghcall は、GitHub とのやりとりの失敗を表す。種類は 07「GitHub との通信に失敗したとき」と
// 「認証の確認」の表の行に 1 対 1 で対応する。usecase は種類で振る舞いを分け(認証エラーにするなど)、
// 利用者に返すときに Refuse で 07 の文言の断り(refusal)に直す。トークンは持たない。
package ghcall

import (
	"errors"
	"fmt"
	"time"
)

// Kind は失敗の種類。
type Kind string

const (
	Unreachable     Kind = "unreachable"
	AuthError       Kind = "auth_error"
	RepoUnreadable  Kind = "repo_unreadable"
	AppNotInstalled Kind = "app_not_installed"
	NoWrite         Kind = "no_write"
	NotFound        Kind = "not_found"
	RateLimited     Kind = "rate_limited"
	NoAccount       Kind = "no_account"
	TokenRejected   Kind = "token_rejected"
	LoginMismatch   Kind = "login_mismatch"
	GhTokenMissing  Kind = "gh_token_missing"
	GhTimeout       Kind = "gh_timeout"
	EnvMissing      Kind = "env_missing"
	KeychainMissing Kind = "keychain_missing"
	// KeychainUnavailable は、このマシンに OS のキーチェーンがない(WSL など)。
	KeychainUnavailable Kind = "keychain_unavailable"
	// Forbidden は、403 で利用上限でないもの。リポジトリを読む操作で、RepoUnreadable・AppNotInstalled・
	// NoWrite に分ける(D3 以降)。
	Forbidden Kind = "forbidden"
)

// Failure は GitHub とのやりとりの失敗。使わない値は空。
type Failure struct {
	Kind    Kind
	Name    string    // アカウントの表示名(AuthError)
	Login   string    // アカウントのログイン名(AuthError・KeychainMissing)、実際のログイン名(LoginMismatch)
	Repo    string    // owner/name
	Org     string    // 組織名(AppNotInstalled)
	Number  int       // PR・Issue の番号(NotFound)
	ResetAt time.Time // 利用上限が戻る時刻。分からなければゼロ(RateLimited)
}

// Error は種類とリポジトリだけを書く(ログ用)。トークンや GitHub の応答の本文は書かない。
func (f *Failure) Error() string {
	switch {
	case f.Repo != "" && f.Number > 0:
		return fmt.Sprintf("github: %s (%s#%d)", f.Kind, f.Repo, f.Number)
	case f.Repo != "":
		return fmt.Sprintf("github: %s (%s)", f.Kind, f.Repo)
	}
	return "github: " + string(f.Kind)
}

// Is は、err が種類 k の Failure を含むか。
func Is(err error, k Kind) bool {
	var f *Failure
	return errors.As(err, &f) && f.Kind == k
}
