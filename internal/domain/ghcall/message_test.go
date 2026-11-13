package ghcall_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

var jst = time.FixedZone("JST", 9*3600)

// refused は、包んだ Failure を Refuse で直した文言を返す。
func refused(t *testing.T, f *ghcall.Failure, now time.Time) string {
	t.Helper()
	var r *refusal.Refusal
	if err := ghcall.Refuse(fmt.Errorf("op: %w", f), now); !errors.As(err, &r) {
		t.Fatalf("Refuse(%v) = %v, not a refusal", f, err)
	}
	return r.Message
}

func TestMessages(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, jst)
	cases := []struct {
		f    *ghcall.Failure
		want string
	}{
		{&ghcall.Failure{Kind: ghcall.Unreachable}, "GitHub に接続できません。接続を確かめて、もう一度お試しください"},
		{&ghcall.Failure{Kind: ghcall.AuthError, Name: "work", Login: "taro"}, "work(@taro)の認証エラーのため、GitHub に接続できません。設定の「アカウントとメンバー」で再認証してください"},
		{&ghcall.Failure{Kind: ghcall.RepoUnreadable, Repo: "org/web"}, "org/web を読めません。リポジトリの権限と、使うアカウントを確かめてください"},
		{&ghcall.Failure{Kind: ghcall.AppNotInstalled, Org: "org", Repo: "org/web"}, "org に GitHub App が入っていないため、org/web を読めません"},
		{&ghcall.Failure{Kind: ghcall.NoWrite, Repo: "org/web"}, "org/web に書き込む権限がありません"},
		{&ghcall.Failure{Kind: ghcall.NotFound, Repo: "org/web", Number: 12}, "org/web#12 が見つかりません"},
		{&ghcall.Failure{Kind: ghcall.NoAccount, Repo: "org/web"}, "org/web を読めるアカウントがありません。設定の「Store」で GitHub に使うアカウントを選んでください"},
		{&ghcall.Failure{Kind: ghcall.TokenRejected}, "トークンが GitHub に受け付けられませんでした。認証方法を確かめてください"},
		{&ghcall.Failure{Kind: ghcall.LoginMismatch, Login: "hanako"}, "このトークンは @hanako のものです。ログイン名を直してください"},
		{&ghcall.Failure{Kind: ghcall.GhTokenMissing}, "gh CLI のトークンが見つかりません。端末で `gh auth login` を実行してください"},
		{&ghcall.Failure{Kind: ghcall.GhTimeout}, "gh CLI から応答がありません。端末で `gh auth status` を確かめてください"},
		{&ghcall.Failure{Kind: ghcall.EnvMissing}, "環境変数 GH_TOKEN が設定されていません"},
		{&ghcall.Failure{Kind: ghcall.KeychainMissing, Login: "taro"}, "OS のキーチェーンに、サービス hedgehug・アカウント github.com/taro のトークンが見つかりません"},
		{&ghcall.Failure{Kind: ghcall.KeychainUnavailable}, "このマシンでは OS のキーチェーンを使えません。gh CLI のトークンか環境変数を使ってください"},
		{&ghcall.Failure{Kind: ghcall.Forbidden, Repo: "org/web"}, "org/web を読めません。リポジトリの権限と、使うアカウントを確かめてください"},
		{&ghcall.Failure{Kind: ghcall.Forbidden}, "GitHub に接続できません。接続を確かめて、もう一度お試しください"},
	}
	for _, c := range cases {
		if got := refused(t, c.f, now); got != c.want {
			t.Errorf("%s: %q\nwant %q", c.f.Kind, got, c.want)
		}
	}
}

func TestRateLimitTime(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, jst)
	const head = "GitHub の利用上限に達しました。"
	cases := []struct {
		name  string
		reset time.Time
		want  string
	}{
		{"same day, rounded up to the minute", time.Date(2026, 11, 12, 1, 30, 1, 0, time.UTC), head + "10:31 以降にもう一度お試しください"},
		{"next day", time.Date(2026, 11, 13, 9, 0, 0, 0, jst), head + "11/13 09:00 以降にもう一度お試しください"},
		{"next year", time.Date(2027, 1, 3, 9, 0, 0, 0, jst), head + "2027/01/03 09:00 以降にもう一度お試しください"},
		{"unknown", time.Time{}, head + "しばらくしてからもう一度お試しください"},
		// このマシンの時計が GitHub より進んでいて、戻る時刻がもう過ぎている: 過ぎた時刻は出さない
		{"past", time.Date(2026, 11, 12, 9, 59, 0, 0, jst), head + "しばらくしてからもう一度お試しください"},
		{"now", now, head + "しばらくしてからもう一度お試しください"},
	}
	for _, c := range cases {
		got := refused(t, &ghcall.Failure{Kind: ghcall.RateLimited, ResetAt: c.reset}, now)
		if got != c.want {
			t.Errorf("%s: %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestRefuseKeepsOtherErrors(t *testing.T) {
	boom := errors.New("boom")
	if ghcall.Refuse(boom, time.Now()) != boom || ghcall.Refuse(nil, time.Now()) != nil {
		t.Error("Refuse changed a non-failure")
	}
}
