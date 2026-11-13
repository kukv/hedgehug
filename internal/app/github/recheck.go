package github

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// Recheck は、認証エラーのアカウント(デバイスフローを除く)のトークンを読み直して本人を確かめ、ログイン名が
// 登録と同じなら利用可能に戻す(07「状態」。この小計画の決定 D4a-18)。戻ったものがあれば true。
// トークンが読めない・拒否された・別人・通信の失敗は、そのまま認証エラーにして次の間隔でまた試す。
// 返すのはアカウントの一覧を読めなかったときの失敗だけ。
func (c Connector) Recheck(ctx context.Context) (bool, error) {
	accs, err := c.Accounts.List(ctx)
	if err != nil {
		return false, err
	}
	recovered := false
	for _, a := range accs {
		if broken, _ := c.Session.state(a.ID); !broken || a.Method == account.DeviceFlow {
			continue
		}
		tok, err := c.Secrets.Token(ctx, a)
		if err != nil {
			continue
		}
		login, err := c.GitHub.Viewer(ctx, tok)
		if err != nil || !strings.EqualFold(login, a.Login) {
			continue
		}
		c.Session.markOK(a.ID, fingerprint(tok))
		recovered = true
	}
	return recovered, nil
}
