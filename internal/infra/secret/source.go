// Package secret は、アカウントのトークンを、その認証方法の置き場所から読む(port.Secrets の実装)。
// トークンと、gh の出力はエラーの文に書かない。
package secret

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// Service は、キーチェーンでトークンを置くサービスの名前(前回と同じ)。
const Service = "hedgehug"

// ghTimeout は、gh CLI の応答を待つ時間(07「認証の確認」)。
const ghTimeout = 10 * time.Second

// KeychainKey は、キーチェーンのアカウントの名前(github.com/<ログイン名>)。
func KeychainKey(login string) string { return account.Host + "/" + login }

// Keyring は OS のキーチェーン。テストでは差し替える。
type Keyring interface {
	Get(service, user string) (string, error)
}

// Source はトークンの読み出し。
type Source struct {
	Getenv    func(string) string
	Run       func(ctx context.Context, name string, args ...string) ([]byte, error)
	Keyring   Keyring
	GhTimeout time.Duration
}

func New() *Source {
	return &Source{Getenv: os.Getenv, Run: run, Keyring: osKeyring{}, GhTimeout: ghTimeout}
}

// Token は、アカウントの認証方法でトークンを読む。
func (s *Source) Token(ctx context.Context, a account.Account) (string, error) {
	switch a.Method {
	case account.GhCLI:
		return s.ghToken(ctx, a.Login)
	case account.EnvVar:
		if v := strings.TrimSpace(s.Getenv("GH_TOKEN")); v != "" {
			return v, nil
		}
		return "", &ghcall.Failure{Kind: ghcall.EnvMissing}
	case account.Keychain:
		v, err := s.Keyring.Get(Service, KeychainKey(a.Login))
		switch {
		case err != nil && !errors.Is(err, ErrNotFound):
			// 見つからない以外の失敗は、キーチェーンそのものが使えない(WSL など。前回の実装と同じ扱い)。
			return "", &ghcall.Failure{Kind: ghcall.KeychainUnavailable}
		case err != nil || strings.TrimSpace(v) == "":
			return "", &ghcall.Failure{Kind: ghcall.KeychainMissing, Login: a.Login}
		}
		return strings.TrimSpace(v), nil
	}
	return "", fmt.Errorf("no token source for the method %q", a.Method)
}

func (s *Source) ghToken(ctx context.Context, login string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, s.GhTimeout)
	defer cancel()
	out, err := s.Run(ctx, "gh", "auth", "token", "--hostname", account.Host, "--user", login)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", &ghcall.Failure{Kind: ghcall.GhTimeout}
	}
	tok := strings.TrimSpace(string(out))
	if err != nil || tok == "" {
		return "", &ghcall.Failure{Kind: ghcall.GhTokenMissing}
	}
	return tok, nil
}

// run はコマンドを実行し、標準出力を返す。標準エラー出力はトークンを含むことがあるので読まない。
func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output() //nolint:gosec // 実行するのは gh だけで、引数はこのパッケージが決める
}
