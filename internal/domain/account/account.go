// Package account は、GitHub のアカウント(GitHub に接続するときの本人確認の手段に名前を付けたもの)を持つ。
// アカウントはマシンごとに持ち、Store には保存しない。トークンはここに持たない(01・07)。
package account

import (
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// ID はアカウントの内部 ID(ULID)。
type ID string

// Host はアカウントのホスト。github.com に固定する(07「登録」)。
const Host = "github.com"

// Method は認証方法(07「登録」)。
type Method string

const (
	DeviceFlow Method = "device"
	GhCLI      Method = "gh"
	EnvVar     Method = "env"
	Keychain   Method = "keychain"
)

// Valid は、4 つの認証方法のどれかか。
func (m Method) Valid() bool {
	switch m {
	case DeviceFlow, GhCLI, EnvVar, Keychain:
		return true
	}
	return false
}

// Account は登録したアカウント。
type Account struct {
	ID     ID
	Name   string // 表示名
	Login  string // GitHub のログイン名
	Method Method
}

// Check は、登録・編集の入力を 07「登録」の表の順に確かめ、前後の空白を取り除いた値を返す。
// self は編集するアカウントの ID で、同じ表示名・ログイン名の検査から除く(登録では空)。
func Check(in Account, existing []Account, self ID) (Account, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Login = strings.TrimSpace(in.Login)
	if in.Name == "" || in.Login == "" {
		return Account{}, refusal.New("表示名とログイン名を入力してください")
	}
	for _, a := range existing {
		if a.ID != self && strings.EqualFold(a.Name, in.Name) {
			return Account{}, refusal.New("同じ表示名のアカウントがあります")
		}
	}
	for _, a := range existing {
		if a.ID != self && strings.EqualFold(a.Login, in.Login) {
			return Account{}, refusal.New("同じログイン名のアカウントがあります")
		}
	}
	if !in.Method.Valid() {
		return Account{}, fmt.Errorf("unknown account method %q", in.Method)
	}
	return in, nil
}
