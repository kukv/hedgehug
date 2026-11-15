package stores

import (
	"context"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// Identity は、Store ごとの「あなた」のログイン名を決める port.Identity(G1-5)。
// Store は読まない(同期の Store のロックの中から呼ばれる)。
type Identity struct {
	GitHub   port.StoreGitHub
	Accounts port.Accounts
}

var _ port.Identity = Identity{}

// Me は、store に割り当てたアカウントがあり、それが登録されていればそのログイン名、
// なければマシンのログイン名を返す。どちらもなければ port.ErrNoLogin、store が一覧になければ port.ErrStoreNotFound。
func (i Identity) Me(ctx context.Context, store port.StoreID) (string, error) {
	infos, err := i.GitHub.List(ctx)
	if err != nil {
		return "", err
	}
	k := slices.IndexFunc(infos, func(s port.StoreGitHubInfo) bool { return s.ID == store })
	if k < 0 {
		return "", port.ErrStoreNotFound
	}
	if login, err := i.accountLogin(ctx, infos[k].Account); err != nil || login != "" {
		return login, err
	}
	login, err := i.GitHub.MachineLogin(ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(login) == "" {
		return "", port.ErrNoLogin
	}
	return login, nil
}

// accountLogin は、登録されたアカウント id のログイン名。id が空・登録にないときは空。
func (i Identity) accountLogin(ctx context.Context, id account.ID) (string, error) {
	if id == "" {
		return "", nil
	}
	accounts, err := i.Accounts.List(ctx)
	if err != nil {
		return "", err
	}
	for _, a := range accounts {
		if a.ID == id {
			return a.Login, nil
		}
	}
	return "", nil
}
