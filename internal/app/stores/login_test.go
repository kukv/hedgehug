package stores_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/stores"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const loginRefusal = "英数字・ハイフン・アンダースコアのログイン名(39 文字以内)を入力してください"

// newLogin は、アカウントなしの team-a と、登録したアカウントを割り当てた team-b を持ち、マシンのログイン名が login の Login。
func newLogin(login string) (stores.Login, *apptest.StoreGitHub, *apptest.Events) {
	gh := &apptest.StoreGitHub{Login: login, Stores: []port.StoreGitHubInfo{
		{ID: "sa", Name: "team-a"},
		{ID: "sb", Name: "team-b", Account: "a1"},
	}}
	ev := &apptest.Events{}
	return stores.Login{GitHub: gh, Accounts: registered(), Events: ev}, gh, ev
}

// registered は、アカウント a1(ログイン名 hanako)だけを登録した Accounts。
func registered() *apptest.Accounts {
	accounts := apptest.NewAccounts()
	if err := accounts.Add(context.Background(), account.Account{ID: "a1", Name: "仕事", Login: "hanako", Method: account.GhCLI}); err != nil {
		panic(err)
	}
	return accounts
}

// 割り当てたアカウントが登録にない Store も、「あなた」はマシンのログイン名なので(Identity と同じ。G1-5)、確認と知らせの対象にする。
func TestLoginIncludesStoresWithUnregisteredAccount(t *testing.T) {
	ctx := context.Background()
	l, gh, ev := newLogin("taro")
	gh.Stores = append(gh.Stores, port.StoreGitHubInfo{ID: "sc", Name: "team-c", Account: "gone"})
	got, err := l.Preview(ctx, "tarou")
	want := []string{"team-a での あなた が @taro から tarou に変わります。", "team-c での あなた が @taro から tarou に変わります。"}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("Preview = %q, %v", got, err)
	}
	if err := l.Set(ctx, "tarou"); err != nil {
		t.Fatal(err)
	}
	if got := ev.List(); !slices.Equal(got, []port.Event{{Kind: port.EventStore, Store: "sa"}, {Kind: port.EventStore, Store: "sc"}}) {
		t.Errorf("events = %+v", got)
	}
}

func TestLoginGet(t *testing.T) {
	l, _, _ := newLogin("taro")
	if got, err := l.Get(context.Background()); err != nil || got != "taro" {
		t.Errorf("Get = %q, %v", got, err)
	}
}

func TestLoginPreview(t *testing.T) {
	want := []string{"team-a での あなた が @taro から tarou に変わります。"}
	tests := []struct {
		name, machine, login string
		want                 []string
	}{
		{"changed", "taro", "tarou", want},
		{"trimmed", "taro", " tarou ", want},
		{"case only", "taro", "Taro", nil},
		{"same", "taro", "taro", nil},
		{"no machine login", "", "tarou", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, _, _ := newLogin(tt.machine)
			got, err := l.Preview(context.Background(), tt.login)
			if err != nil || !slices.Equal(got, tt.want) {
				t.Errorf("Preview(%q) = %q, %v", tt.login, got, err)
			}
		})
	}
}

func TestLoginPreviewRefusesBadNames(t *testing.T) {
	for _, login := range []string{"ta ro", "", "  ", strings.Repeat("a", 40)} {
		l, _, _ := newLogin("taro")
		_, err := l.Preview(context.Background(), login)
		if r := (*refusal.Refusal)(nil); !errors.As(err, &r) || r.Message != loginRefusal {
			t.Errorf("Preview(%q) err = %v", login, err)
		}
	}
}

func TestLoginSet(t *testing.T) {
	ctx := context.Background()
	l, gh, ev := newLogin("taro")
	if err := l.Set(ctx, " tarou "); err != nil {
		t.Fatal(err)
	}
	if got, _ := gh.MachineLogin(ctx); got != "tarou" {
		t.Errorf("login = %q", got)
	}
	if got := ev.List(); !slices.Equal(got, []port.Event{{Kind: port.EventStore, Store: "sa"}}) {
		t.Errorf("events = %+v", got)
	}
}

// マシンのログイン名が空から値に変わるときも、「あなた」が決まるので知らせる(確認の行は出ない)。
func TestLoginSetFromEmptyPublishes(t *testing.T) {
	ctx := context.Background()
	l, gh, ev := newLogin("")
	if err := l.Set(ctx, "tarou"); err != nil {
		t.Fatal(err)
	}
	if got, _ := gh.MachineLogin(ctx); got != "tarou" {
		t.Errorf("login = %q", got)
	}
	if got := ev.List(); !slices.Equal(got, []port.Event{{Kind: port.EventStore, Store: "sa"}}) {
		t.Errorf("events = %+v", got)
	}
}

// listFails は、List だけが失敗する StoreGitHub。
type listFails struct{ *apptest.StoreGitHub }

func (listFails) List(context.Context) ([]port.StoreGitHubInfo, error) {
	return nil, errors.New("list failed")
}

// 知らせる先を読めなければ、保存しない(保存したのに知らせない、をなくす)。
func TestLoginSetSavesNothingWhenTheListFails(t *testing.T) {
	ctx := context.Background()
	gh := &apptest.StoreGitHub{Login: "taro"}
	ev := &apptest.Events{}
	if err := (stores.Login{GitHub: listFails{gh}, Accounts: registered(), Events: ev}).Set(ctx, "tarou"); err == nil {
		t.Error("Set succeeded")
	}
	if got, _ := gh.MachineLogin(ctx); got != "taro" || len(ev.List()) != 0 {
		t.Errorf("login = %q, events = %+v", got, ev.List())
	}
}

func TestLoginSetSameDoesNothing(t *testing.T) {
	ctx := context.Background()
	l, gh, ev := newLogin("taro")
	if err := l.Set(ctx, "taro "); err != nil {
		t.Fatal(err)
	}
	if got, _ := gh.MachineLogin(ctx); got != "taro" || len(ev.List()) != 0 {
		t.Errorf("login = %q, events = %+v", got, ev.List())
	}
}

// 大文字と小文字だけが違うときは、確認なしで保存する(G1-15)。
func TestLoginSetCaseOnlySaves(t *testing.T) {
	ctx := context.Background()
	l, gh, _ := newLogin("taro")
	if err := l.Set(ctx, "Taro"); err != nil {
		t.Fatal(err)
	}
	if got, _ := gh.MachineLogin(ctx); got != "Taro" {
		t.Errorf("login = %q", got)
	}
}

func TestLoginSetRefusesBadName(t *testing.T) {
	ctx := context.Background()
	l, gh, ev := newLogin("taro")
	err := l.Set(ctx, "ta ro")
	if r := (*refusal.Refusal)(nil); !errors.As(err, &r) || r.Message != loginRefusal {
		t.Errorf("err = %v", err)
	}
	if got, _ := gh.MachineLogin(ctx); got != "taro" || len(ev.List()) != 0 {
		t.Errorf("login = %q, events = %+v", got, ev.List())
	}
}
