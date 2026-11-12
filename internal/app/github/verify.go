package github

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// verifyToken は、登録・編集・再認証の認証の確認(07「認証の確認」)。a の認証方法でトークンを読み、
// GitHub でログイン名を照らす。デバイスフローは扱わない。確かめたトークンの指紋を返す。
func verifyToken(ctx context.Context, secrets port.Secrets, gh port.GitHub, a account.Account) ([32]byte, error) {
	token, err := secrets.Token(ctx, a)
	if err != nil {
		return [32]byte{}, err
	}
	login, err := gh.Viewer(ctx, token)
	if err != nil {
		return [32]byte{}, err
	}
	if !strings.EqualFold(login, a.Login) {
		return [32]byte{}, &ghcall.Failure{Kind: ghcall.LoginMismatch, Login: login}
	}
	return fingerprint(token), nil
}
