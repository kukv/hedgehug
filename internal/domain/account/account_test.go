package account_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

var existing = []account.Account{
	{ID: "A1", Name: "work", Login: "taro", Method: account.GhCLI},
	{ID: "A2", Name: "home", Login: "taro-home", Method: account.EnvVar},
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

// 07「登録」の表の順に確かめる。最初に当てはまったものを返す。
func TestCheckOrder(t *testing.T) {
	cases := []struct {
		name string
		in   account.Account
		self account.ID
		want string
	}{
		{"empty name", account.Account{Name: " ", Login: "x", Method: account.GhCLI}, "", "表示名とログイン名を入力してください"},
		{"empty login", account.Account{Name: "x", Login: "", Method: account.GhCLI}, "", "表示名とログイン名を入力してください"},
		{"empty before duplicate", account.Account{Name: "WORK", Login: "", Method: account.GhCLI}, "", "表示名とログイン名を入力してください"},
		{"duplicate name ignores case", account.Account{Name: "WORK", Login: "TARO", Method: account.GhCLI}, "", "同じ表示名のアカウントがあります"},
		{"duplicate login ignores case", account.Account{Name: "new", Login: "TARO", Method: account.GhCLI}, "", "同じログイン名のアカウントがあります"},
		{"editing itself is not a duplicate", account.Account{Name: "work", Login: "taro", Method: account.Keychain}, "A1", ""},
	}
	for _, c := range cases {
		_, err := account.Check(c.in, existing, c.self)
		if got := message(err); got != c.want {
			t.Errorf("%s: message = %q, want %q (err %v)", c.name, got, c.want, err)
		}
	}
}

func TestCheckUnknownMethodIsNotARefusal(t *testing.T) {
	// 認証方法は画面の選択肢から選ぶので、知らない値は呼び出しの誤り(500)。仕様の文言はない。
	_, err := account.Check(account.Account{Name: "new", Login: "jiro", Method: "ssh"}, existing, "")
	if err == nil || message(err) != "" {
		t.Errorf("err = %v", err)
	}
}

func TestCheckTrims(t *testing.T) {
	got, err := account.Check(account.Account{Name: "  new ", Login: " jiro\t", Method: account.DeviceFlow}, existing, "")
	if err != nil || got.Name != "new" || got.Login != "jiro" {
		t.Errorf("Check = %+v, %v", got, err)
	}
}
