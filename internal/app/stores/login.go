package stores

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// Login は、設定の「一般」のマシンの設定「ログイン名」を読み書きする(08。G1-15〜G1-17)。
type Login struct {
	GitHub   port.StoreGitHub
	Accounts port.Accounts
	Events   port.Events
}

// Get は、いまのマシンのログイン名。
func (l Login) Get(ctx context.Context) (string, error) {
	return l.GitHub.MachineLogin(ctx)
}

// Preview は、login(前後の空白を除く)に変えたときの確認の本文の行。空なら確認なしで保存する。
// 対象は「あなた」がマシンのログイン名になる Store(unassigned)で、いまのログイン名が空なら行を出さない(G1-16)。使えない名前は断る。
func (l Login) Preview(ctx context.Context, login string) ([]string, error) {
	to, err := validLogin(login)
	if err != nil {
		return nil, err
	}
	from, err := l.GitHub.MachineLogin(ctx)
	if err != nil || from == "" {
		return nil, err
	}
	infos, err := l.unassigned(ctx)
	if err != nil {
		return nil, err
	}
	uses := make([]account.StoreUse, 0, len(infos))
	for _, s := range infos {
		uses = append(uses, account.StoreUse{Name: s.Name, Remote: s.Remote})
	}
	return account.LoginLines(from, to, uses), nil
}

// Set は、login(前後の空白を除く)を保存し、「あなた」が変わった Store ごとに EventStore を知らせる(G1-17)。
// Preview と同じく使えない名前は断る(保存すると、以後の人の保存がすべて断られるため)。いまと同じなら何もしない。
func (l Login) Set(ctx context.Context, login string) error {
	to, err := validLogin(login)
	if err != nil {
		return err
	}
	from, err := l.GitHub.MachineLogin(ctx)
	if err != nil || from == to {
		return err
	}
	// 知らせる先を保存の前に読む(読めなければ保存しない。保存したのに知らせない、をなくす)
	uses, err := l.unassigned(ctx)
	if err != nil {
		return err
	}
	if err := l.GitHub.SetMachineLogin(ctx, to); err != nil {
		return err
	}
	for _, u := range uses {
		l.Events.Publish(port.Event{Kind: port.EventStore, Store: u.ID})
	}
	return nil
}

// unassigned は、「あなた」がマシンのログイン名になる Store(非表示・閲覧のみを含む): アカウントが「なし」か、
// 割り当てたアカウントが登録にない(Identity と同じ判定。G1-5)。
func (l Login) unassigned(ctx context.Context) ([]port.StoreGitHubInfo, error) {
	all, err := l.GitHub.List(ctx)
	if err != nil {
		return nil, err
	}
	accounts, err := l.Accounts.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []port.StoreGitHubInfo
	for _, s := range all {
		if loginOf(accounts, s.Account) == "" {
			out = append(out, s)
		}
	}
	return out, nil
}

// validLogin は、前後の空白を除いた login。ログイン名に使えなければ断る(08)。
func validLogin(login string) (string, error) {
	s := strings.TrimSpace(login)
	if !member.ValidLogin(s) {
		return "", refusal.New("英数字・ハイフン・アンダースコアのログイン名(39 文字以内)を入力してください")
	}
	return s, nil
}
