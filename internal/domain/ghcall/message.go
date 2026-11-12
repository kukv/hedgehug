package ghcall

import (
	"errors"
	"fmt"
	"time"

	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// Refuse は、err の Failure を 07 の文言の断り(refusal)に直す。Failure でないエラーはそのまま返す。
// now は利用上限の時刻の書き方に使う(このマシンの時刻帯で書く)。
func Refuse(err error, now time.Time) error {
	var f *Failure
	if !errors.As(err, &f) {
		return err
	}
	return refusal.New(message(f, now))
}

const unreachableText = "GitHub に接続できません。接続を確かめて、もう一度お試しください"

func message(f *Failure, now time.Time) string {
	switch f.Kind {
	case AuthError:
		return fmt.Sprintf("%s(@%s)の認証エラーのため、GitHub に接続できません。設定の「アカウントとメンバー」で再認証してください", f.Name, f.Login)
	case RepoUnreadable:
		return f.Repo + " を読めません。リポジトリの権限と、使うアカウントを確かめてください"
	case AppNotInstalled:
		return fmt.Sprintf("%s に GitHub App が入っていないため、%s を読めません", f.Org, f.Repo)
	case NoWrite:
		return f.Repo + " に書き込む権限がありません"
	case NotFound:
		return fmt.Sprintf("%s#%d が見つかりません", f.Repo, f.Number)
	case RateLimited:
		return rateLimited(f.ResetAt, now)
	case NoAccount:
		return f.Repo + " を読めるアカウントがありません。設定の「Store」で GitHub に使うアカウントを選んでください"
	case LoginMismatch:
		return "このトークンは @" + f.Login + " のものです。ログイン名を直してください"
	case KeychainMissing:
		return "OS のキーチェーンに、サービス hedgehug・アカウント github.com/" + f.Login + " のトークンが見つかりません"
	case Forbidden:
		if f.Repo != "" {
			return message(&Failure{Kind: RepoUnreadable, Repo: f.Repo}, now)
		}
		return unreachableText
	}
	if text, ok := fixedText[f.Kind]; ok {
		return text
	}
	return unreachableText
}

// fixedText は、値を入れない文言。
var fixedText = map[Kind]string{ //nolint:gosec // G101: 利用者に見せる文言で、秘密の値ではない
	Unreachable:         unreachableText,
	TokenRejected:       "トークンが GitHub に受け付けられませんでした。認証方法を確かめてください",
	GhTokenMissing:      "gh CLI のトークンが見つかりません。端末で `gh auth login` を実行してください",
	GhTimeout:           "gh CLI から応答がありません。端末で `gh auth status` を確かめてください",
	EnvMissing:          "環境変数 GH_TOKEN が設定されていません",
	KeychainUnavailable: "このマシンでは OS のキーチェーンを使えません。gh CLI のトークンか環境変数を使ってください",
}

// rateLimited は 07 の利用上限の文言。時刻は分に切り上げ、翌日以降なら日付を前に足す(01「日付と時刻」)。
func rateLimited(reset, now time.Time) string {
	if reset.IsZero() {
		return "GitHub の利用上限に達しました。しばらくしてからもう一度お試しください"
	}
	at := reset.In(now.Location())
	if t := at.Truncate(time.Minute); !t.Equal(at) {
		at = t.Add(time.Minute)
	}
	text := at.Format("15:04")
	ny, nm, nd := now.Date()
	if y, m, d := at.Date(); y != ny || m != nm || d != nd {
		if y == ny {
			text = fmt.Sprintf("%d/%d %s", int(m), d, text)
		} else {
			text = at.Format("2006/01/02") + " " + text
		}
	}
	return "GitHub の利用上限に達しました。" + text + " 以降にもう一度お試しください"
}
