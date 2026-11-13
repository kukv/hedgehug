package github_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

func (f connFixture) reauth() github.Reauth {
	return github.Reauth{Session: f.session, Accounts: f.accounts, Secrets: f.secrets, GitHub: f.gh, Clock: f.clock}
}

func (f connFixture) edit() github.EditAccount {
	return github.EditAccount{Session: f.session, Accounts: f.accounts, Secrets: f.secrets, GitHub: f.gh, Clock: f.clock}
}

func (f connFixture) state(t *testing.T, i int) github.AccountState {
	t.Helper()
	list, err := github.ListAccounts{Accounts: f.accounts, Session: f.session}.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return list[i]
}

func TestReauth(t *testing.T) {
	f := newConn(t)
	f.conn.Rejected(work.ID)
	f.secrets.Tokens["taro"] = "tok-revoked"
	_, err := f.reauth().Do(context.Background(), work.ID)
	if message(err) != "トークンが GitHub に受け付けられませんでした。認証方法を確かめてください" || !f.state(t, 0).AuthError {
		t.Fatalf("failed reauth: %v, %+v", err, f.state(t, 0))
	}
	f.secrets.Tokens["taro"] = "tok-taro"
	if _, err := f.reauth().Do(context.Background(), work.ID); err != nil || f.state(t, 0).AuthError {
		t.Fatalf("reauth: %v, %+v", err, f.state(t, 0))
	}
	// 再認証で確かめたトークンは、次の接続で問い合わせ直さない。
	calls := f.gh.calls
	if _, err := f.conn.Token(context.Background(), work); err != nil || f.gh.calls != calls {
		t.Errorf("after reauth: %v, calls %d → %d", err, calls, f.gh.calls)
	}
}

func TestReauthRefusesDeviceFlowAndUnknown(t *testing.T) {
	f := newConn(t)
	if _, err := f.reauth().Do(context.Background(), home.ID); !errors.Is(err, github.ErrDeviceFlow) {
		t.Errorf("device flow: %v", err)
	}
	if _, err := f.reauth().Do(context.Background(), "A9"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

func TestEditAccount(t *testing.T) {
	f := newConn(t)
	f.secrets.Tokens["jiro"] = "tok-jiro"
	f.gh.Users["tok-jiro"] = "jiro"
	got, err := f.edit().Do(context.Background(), github.EditAccountInput{ID: work.ID, Name: " 仕事 ", Login: "jiro", Method: account.Keychain})
	want := account.Account{ID: work.ID, Name: "仕事", Login: "jiro", Method: account.Keychain}
	if err != nil || got != want {
		t.Fatalf("Do = %+v, %v", got, err)
	}
	list, _ := f.accounts.List(context.Background())
	if list[0] != want {
		t.Errorf("kept %+v", list[0])
	}
	// Store の割り当ては ID で持つので、そのまま残る。
	if s, _ := f.stores.List(context.Background()); s[0].Account != work.ID {
		t.Errorf("assignment %+v", s[0])
	}
}

func TestEditAccountRefusals(t *testing.T) {
	f := newConn(t)
	cases := []struct {
		in   github.EditAccountInput
		want string
	}{
		{github.EditAccountInput{ID: work.ID, Name: "家", Login: "taro", Method: account.GhCLI}, "同じ表示名のアカウントがあります"},
		{github.EditAccountInput{ID: work.ID, Name: "work", Login: "Hanako", Method: account.GhCLI}, "同じログイン名のアカウントがあります"},
		{github.EditAccountInput{ID: work.ID, Name: "work", Login: "jiro", Method: account.EnvVar}, "環境変数 GH_TOKEN が設定されていません"},
	}
	for _, c := range cases {
		if c.in.Method == account.EnvVar {
			f.secrets.Err = &ghcall.Failure{Kind: ghcall.EnvMissing}
		}
		if _, err := f.edit().Do(context.Background(), c.in); message(err) != c.want {
			t.Errorf("%+v: %v; want %q", c.in, err, c.want)
		}
	}
	// 自分自身の表示名・ログイン名は重なりに数えない。
	f.secrets.Err = nil
	if _, err := f.edit().Do(context.Background(), github.EditAccountInput{ID: work.ID, Name: "WORK", Login: "taro", Method: account.GhCLI}); err != nil {
		t.Errorf("self: %v", err)
	}
	if list, _ := f.accounts.List(context.Background()); list[0].Name != "WORK" {
		t.Errorf("kept %+v", list[0])
	}
}

func TestEditAccountToDeviceFlowSignsIn(t *testing.T) {
	f := newConn(t)
	_, err := f.edit().Do(context.Background(), github.EditAccountInput{ID: work.ID, Name: "work", Login: "taro", Method: account.DeviceFlow})
	if !errors.Is(err, github.ErrDeviceFlow) {
		t.Errorf("Do = %v", err)
	}
}

// デバイスフローから替えるときは、誰も読まなくなるキーチェーンのトークンを消す(決定 D2-13)。
func TestEditAccountFromDeviceFlowDropsTheOldToken(t *testing.T) {
	cases := []struct {
		name     string
		in       github.EditAccountInput
		wantKept bool
	}{
		{"gh", github.EditAccountInput{ID: home.ID, Name: "家", Login: "hanako", Method: account.GhCLI}, false},
		{"keychain, same login", github.EditAccountInput{ID: home.ID, Name: "家", Login: "hanako", Method: account.Keychain}, true},
		{"keychain, case-only change", github.EditAccountInput{ID: home.ID, Name: "家", Login: "Hanako", Method: account.Keychain}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newConn(t)
			f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-hanako"}}
			f.secrets.Tokens["hanako"] = "tok-hanako"
			f.secrets.Tokens["Hanako"] = "tok-hanako"
			if _, err := f.edit().Do(context.Background(), c.in); err != nil {
				t.Fatal(err)
			}
			if _, ok := f.secrets.Device["hanako"]; ok != c.wantKept {
				t.Errorf("device token kept = %v; want %v", ok, c.wantKept)
			}
		})
	}
}

func TestEditAccountKeepsGoingWhenTheOldTokenCannotBeDeleted(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-hanako"}}
	f.secrets.Tokens["hanako"] = "tok-hanako"
	f.secrets.DeleteErr = errors.New("locked")
	in := github.EditAccountInput{ID: home.ID, Name: "家", Login: "hanako", Method: account.GhCLI}
	if _, err := f.edit().Do(context.Background(), in); err != nil {
		t.Fatalf("Do = %v", err)
	}
	if got := f.state(t, 1); got.Account.Method != account.GhCLI {
		t.Errorf("not saved: %+v", got)
	}
}

func TestEditAccountLeavesOtherDeviceTokens(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"taro": {Access: "tok-taro"}}
	in := github.EditAccountInput{ID: work.ID, Name: "WORK", Login: "taro", Method: account.GhCLI}
	if _, err := f.edit().Do(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.secrets.Device["taro"]; !ok {
		t.Error("a token of a non-device-flow account was deleted")
	}
}

func TestEditAccountKeepsTheOldTokenWhenVerifyFails(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-hanako"}}
	in := github.EditAccountInput{ID: home.ID, Name: "家", Login: "hanako", Method: account.GhCLI}
	if _, err := f.edit().Do(context.Background(), in); err == nil {
		t.Fatal("want a refusal (no gh token for hanako)")
	}
	if _, ok := f.secrets.Device["hanako"]; !ok {
		t.Error("deleted before the edit succeeded")
	}
}

func (f connFixture) del() github.DeleteAccount {
	return github.DeleteAccount{Session: f.session, Accounts: f.accounts, Secrets: f.secrets, Stores: f.stores, Missing: &memMissing{rows: map[account.ID][]port.MissingOrg{}}}
}

func TestDeleteAccount(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-hanako"}}
	f.stores.Stores[1].Account = home.ID
	del := f.del()
	if err := del.Do(context.Background(), home.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := f.accounts.List(context.Background()); !slices.Equal(list, []account.Account{work}) {
		t.Errorf("accounts %+v", list)
	}
	if s, _ := f.stores.List(context.Background()); s[1].Account != "" || s[0].Account != work.ID {
		t.Errorf("stores %+v", s)
	}
	if _, ok := f.secrets.Device["hanako"]; ok {
		t.Error("the device flow token was left in the keychain")
	}
	// gh・環境変数・キーチェーンのトークンは消さない(利用者が置いたもの)。
	if err := del.Do(context.Background(), work.ID); err != nil || f.secrets.Tokens["taro"] != "tok-taro" {
		t.Errorf("gh: %v, tokens %v", err, f.secrets.Tokens)
	}
	if err := del.Do(context.Background(), work.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("again: %v", err)
	}
}

func TestDeleteAccountSucceedsWhenTheTokenCannotBeDeleted(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-hanako"}}
	f.secrets.DeleteErr = errors.New("locked")
	f.stores.Stores[1].Account = home.ID
	if err := f.del().Do(context.Background(), home.ID); err != nil {
		t.Fatalf("Do = %v", err)
	}
	if list, _ := f.accounts.List(context.Background()); len(list) != 1 {
		t.Errorf("accounts %+v", list)
	}
	if s, _ := f.stores.List(context.Background()); s[1].Account != "" {
		t.Errorf("stores %+v", s)
	}
}

// 削除したアカウントの認証エラーを、同じ ID で作り直したアカウントに引き継がない。
func TestDeleteAccountForgetsTheSessionState(t *testing.T) {
	f := newConn(t)
	f.conn.Rejected(work.ID)
	if err := f.del().Do(context.Background(), work.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.accounts.Add(context.Background(), work); err != nil {
		t.Fatal(err)
	}
	if got := f.state(t, 1); got.AuthError {
		t.Errorf("state survived the delete: %+v", got)
	}
}

func TestImpact(t *testing.T) {
	f := newConn(t)
	f.stores.Login = "hanako-machine"
	f.stores.Stores = append(f.stores.Stores, port.StoreGitHubInfo{ID: "S3", Name: "OSS", Account: work.ID})
	im := github.Impact{Accounts: f.accounts, Stores: f.stores}
	got, err := im.Delete(context.Background(), work.ID)
	want := []string{
		"「work」を削除します。",
		"チーム の同期に使っているため、それらは同期できなくなります。",
		"チーム での あなた が @taro から hanako-machine に変わります。",
		"OSS での あなた が @taro から hanako-machine に変わります。",
	}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("Delete = %q, %v", got, err)
	}
	got, err = im.Edit(context.Background(), work.ID, "jiro")
	if err != nil || len(got) != 2 || got[0] != "チーム での あなた が @taro から jiro に変わります。" {
		t.Errorf("Edit = %q, %v", got, err)
	}
	if got, _ := im.Edit(context.Background(), home.ID, "someone"); len(got) != 0 {
		t.Errorf("no store uses home: %q", got)
	}
}

func TestImpactWithoutMachineLoginHasNoYouLines(t *testing.T) {
	f := newConn(t)
	f.stores.Login = ""
	got, err := github.Impact{Accounts: f.accounts, Stores: f.stores}.Delete(context.Background(), work.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range got {
		if strings.Contains(line, "あなた が") {
			t.Errorf("line with the machine login: %q", line)
		}
	}
}
