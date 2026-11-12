# hedgehug 小計画 D2: アカウントの残りの操作(サーバー)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 07 のアカウントのうち、D1 で作らなかった操作(デバイスフロー、接続のたびの照合、認証エラーの状態、再認証、編集、削除と削除の影響、Store に割り当てたアカウントでトークンを得ること)を、サーバーの API まで作る。

**Architecture:** トラック D の計画(`2026-11-12-hedgehug-track-d.md`)の形をそのまま使う。usecase は `app/github` に置き、口(`port.Accounts`・`Secrets`・`GitHub`・`DeviceFlow`・`Hosts`・`StoreGitHub`)越しにだけ外に触る。トークンを得る入口は `app/github.Connector` 1 つにし、読み直し・指紋の照合・期限付きトークンの取り直し・認証エラーへの変更をそこに集める。アプリを開いているあいだだけの状態(認証エラー、最後に確かめたトークンの指紋、サインインの途中の流れ)は `app/github.Session` がメモリに持つ。

**Tech Stack:** Go 1.27.3、net/http、github.com/zalando/go-keyring、go.yaml.in/yaml/v4、huma v2。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/07-github.md`「アカウント」の全節(「登録」を除く)、「GitHub との通信に使うアカウント」
- 01「「あなた」の決まり方」、08「GitHub に使うアカウント」
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`「このマシンに持つもの」
- `docs/superpowers/plans/2026-11-12-hedgehug-track-d.md`(トラック D の決定と D1 の型)

**ブランチ:** `origin/develop` から `feat/track-d2`。PR の向き先は `develop`。Task 1(口の追加)は共有部分なので、その PR を先に出す。D1 と同じく、残りは 1 本の PR にまとめてよい。

## Global Constraints

トラック D の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- トークンらしい値(`ghp_`・`gho_`・`ghu_`・`ghs_`・`ghr_`・`github_pat_` で始まる文字列)を、計画にもテストにも書かない(gitleaks)。テストのトークンは `tok-taro` のような値にする。
- デバイスフローの `device_code` とトークンは、サーバーの中だけに持ち、API の応答に出さない。画面に出すのは `user_code`・確認の URL・期限・間隔だけ。
- 本物の組み立てだけで使う型と関数は `//go:build !e2e` のファイルに置く(e2e のビルドも lint するため)。
- API を変えたら `make generate` と `cd web && pnpm gen` で生成物を作り直す。
- force push はしない。載せ直すときは別名のブランチで出す。

---

## この小計画で決めたこと(既存の規則の当てはめ)

| # | 決めたこと | 根拠 |
|---|---|---|
| D2-1 | D2 はサーバー(API)までにする。設定の「アカウントとメンバー」の画面と「GitHub を使わない場合」の無効の部品は、小計画 **D2 画面** として、トラック A の A2 がルーター・サイドバー・トースト・断りの読み取り(`Refused`)を develop に入れてから計画を書く | A2 の計画(作成中)が、画面のアドレス(`core/features/router`)・サイドバー(`app/src/shell`)・トースト・422 の `detail` の読み取りを持つ。トラック A の計画の未決の点 2 の推奨(先に要るトラックが入れる)に従うと、A2 が先に要る。計画は要る直前に書く(進め方の決定) |
| D2-2 | 最後に確かめたトークンの指紋は、メモリ(`Session`)だけに持つ。保存の設計の「OS のキーチェーンに置く」は直す | 07「接続のたびの照合」は、アプリを開いてから初めての接続で必ず照合する。再起動をまたいで覚えても、見える振る舞いは変わらない。このマシン(WSL)のようにキーチェーンがない環境でも同じに動く |
| D2-3 | 認証エラーの状態と、その理由の文言(ログイン名の不一致)は、メモリ(`Session`)だけに持つ。再起動すると「利用可能」から始まり、最初の接続で照合し直す | 保存の設計の state.db の中身に、認証エラーの状態はない。gh・環境変数・キーチェーンは最初の接続で照合し直すので結果は同じになる。デバイスフローは、次に拒否されたときに認証エラーに戻る |
| D2-4 | 「GitHub App 未導入」(状態・帯・依頼文のコピー・「再確認」)は、検出と同じ D4 で作る。トラック D の計画の一覧と漏れの表を直す | 未導入の記録は state.db に持つ(保存の設計)。検出は D4。記録のない D2 で表示だけを作っても確かめられない |
| D2-5 | デバイスフローで得たトークンは、キーチェーンのサービス `hedgehug`・アカウント `github.com/{ログイン名}` に、JSON(`access_token`・`refresh_token`・`expires_at`・`refresh_expires_at`)で置く。認証方法が「OS のキーチェーン」のアカウントは、その項目が JSON で `access_token` を持てばそれを、そうでなければ値をそのままトークンとして読む | 07「接続のたびの照合」(同じ場所に保存)。前回の実装と同じ形。編集で「デバイスフロー → OS のキーチェーン」に替えても、同じ項目を読める |
| D2-6 | 期限付きトークンは、期限の 5 分前から取り直す。取り直しは `Session` のロックで 1 つずつ行い、ロックを取ったあとにキーチェーンを読み直して、取り直し済みなら使う。`expires_in` が 0 なら期限なし | 前回の実装(`refreshBefore`)。いまはサーバーの 1 プロセスだけが取り直すので、プロセスの中のロックで足りる。CLI を作るときにファイルのロックへ広げる |
| D2-7 | 取り直しに失敗したとき(更新用のトークンの期限切れ・GitHub の拒否・`client_id` がない・新しいトークンをキーチェーンに置けない)は認証エラーにする。通信できないときは認証エラーにせず、「通信できない」で返す | 07「デバイスフロー」(取り直しの失敗は拒否と同じ)、07「状態」(認証エラーは拒否のときだけ) |
| D2-8 | サインインの途中の流れは、サーバーのメモリ(`Session`)に置き、画面は流れの ID で問い合わせる(`GET /api/sign-ins/{id}`)。サーバーは問い合わせのたびに GitHub に 1 回だけ聞く。GitHub に届かないときは「待っている」と返す。期限は GitHub が返した `expires_in` から決め、過ぎたら「期限切れ」を返す | 07「デバイスフロー」(待機中の通信切断はつながるまで待つ、期限は GitHub に従う)。`device_code` を画面に出さない |
| D2-9 | サインインの流れは、登録・編集・再認証の 3 つで同じ API を使う。編集と再認証は、流れを始めるときにアカウントの ID を渡す。許可されたトークンのログイン名が入力と違うときは、状態「ログイン名が違う」と 07 の文言を返す(422 にしない)。キーチェーンに置けないときは、状態「失敗」と 07 の文言を返す | 07「デバイスフロー」は、この 2 つを「アカウントを追加」に戻して出し分ける(ログイン名の欄の下と、ダイアログの赤字)。状態で分ければ、画面は文言を見ずに出し分けられる |
| D2-10 | 削除の影響と編集の確認の本文は、サーバーの domain(`account.DeleteLines`・`account.EditLines`)が 07 の文言で組み、API は行の配列で返す | トラック D の決定(文言はサーバーの domain が持つ)。Store 名・remote・マシンのログイン名はサーバーにある |
| D2-11 | Store の GitHub に関わる値(名前・割り当てたアカウント・remote の有無)と、マシンの設定「ログイン名」は、口 `port.StoreGitHub` で読む。削除で割り当てを「なし」にする書き込み(`Unassign`)も同じ口に置く。本物はトラック G が作る。それまでは、本物の組み立ては Store のない実装(`noStoreGitHub`)を使い、e2e は偽物とサンプルの Store を使う | トラック D の決定 1。Store の一覧とマシンのログイン名は G の持ち場(config.yaml) |
| D2-12 | アカウントを削除したとき、認証方法がデバイスフローなら、キーチェーンのそのトークンも消す。ほかの認証方法では消さない(利用者が置いたものなので) | 07「アカウント」(トークンは hedgehug がデバイスフローで保存したもの)。前回の実装の `Delete` |
| D2-13 | 編集でデバイスフローのアカウントのログイン名を変えたときは、サインインのあと、古いログイン名の項目を消す。認証方法をデバイスフローからほかへ替えたときは、項目を消さない(決定 D2-5 で読めるため) | 決定 D2-12 と同じ考え方。残しても誰も読まない項目だけを消す |
| D2-14 | `config.yaml` の `hosts` は、前回と同じ `- {host: github.com, client_id: ...}` の形で読む。`host` と `client_id` のほかの鍵(前回の `api`・`web`)は読まない | 07「デバイスフロー」(ホスト `github.com` の `client_id`)。ホストは github.com に固定(07「登録」) |
| D2-15 | 再認証と編集の確かめ方は、D1 の登録(`AddAccount`)の確かめ方を `verifyToken` に出して共用する | 07「再認証」「編集」(登録と同じ認証の確認) |

## ファイルの地図

```
internal/domain/account/device.go          DeviceToken・Fresh・RefreshExpired
internal/domain/account/impact.go          StoreUse・DeleteLines・EditLines(07「削除の影響」「編集」)
internal/domain/account/device_test.go・impact_test.go

internal/app/port/accounts.go(変更)        Update・Remove を足す
internal/app/port/secrets.go(変更)         DeviceToken・SaveDeviceToken・DeleteDeviceToken を足す
internal/app/port/deviceflow.go            DeviceFlow(StartDevice・PollDevice・RefreshDevice)
internal/app/port/hosts.go                 Hosts(ClientID)
internal/app/port/storegithub.go           StoreGitHub(List・Unassign・MachineLogin)
internal/app/apptest/accounts.go(変更)、accounts_contract.go(変更)
internal/app/apptest/github.go(変更)       Secrets にデバイスフローのトークン
internal/app/apptest/deviceflow.go         DeviceFlow・Hosts の偽物
internal/app/apptest/storegithub.go        StoreGitHub の偽物

internal/infra/machineconfig/accounts.go(変更)  Update・Remove
internal/infra/machineconfig/hosts.go           Hosts(ClientID)
internal/infra/machineconfig/file.go(変更)       config に hosts
internal/infra/secret/keyring.go(変更)          Set・Delete
internal/infra/secret/device.go                 デバイスフローのトークンの読み書き
internal/infra/secret/source.go(変更)            キーチェーンの JSON を読む
internal/infra/github/device.go                 github.com/login/device/code・/login/oauth/access_token
internal/infra/github/client.go(変更)            WebURL

internal/app/github/session.go             Session(認証エラー・指紋・サインインの流れ)
internal/app/github/verify.go              verifyToken(登録・編集・再認証の確かめ方)
internal/app/github/connector.go           Connector(Token・TokenFor・Rejected)
internal/app/github/list_accounts.go(変更) 状態を付けて返す
internal/app/github/add_account.go(変更)   verifyToken を使う
internal/app/github/reauth.go              Reauth
internal/app/github/edit_account.go        EditAccount
internal/app/github/delete_account.go      DeleteAccount・Impact
internal/app/github/sign_in.go             SignIn(Start・Poll・Cancel)
internal/app/github/*_test.go

internal/adapter/httpapi/github/accounts.go(変更)  一覧に状態、PUT・DELETE・再認証・影響
internal/adapter/httpapi/github/sign_in.go         POST・GET・DELETE /api/sign-ins
internal/adapter/httpapi/routes.go(変更)、cmd/hedgehug/wire.go・wire_e2e.go(変更)、cmd/hedgehug/nostores.go
api/openapi.yaml・web/packages/api/src/generated.ts(生成物)
```

---

### Task 1: 口と domain の型と偽物(共有の小さな PR)

**Files:**
- Create: `internal/domain/account/device.go`、`internal/domain/account/device_test.go`、`internal/domain/account/impact.go`、`internal/domain/account/impact_test.go`
- Create: `internal/app/port/deviceflow.go`、`internal/app/port/hosts.go`、`internal/app/port/storegithub.go`
- Modify: `internal/app/port/accounts.go`、`internal/app/port/secrets.go`
- Create: `internal/app/apptest/deviceflow.go`、`internal/app/apptest/storegithub.go`
- Modify: `internal/app/apptest/accounts.go`、`internal/app/apptest/accounts_contract.go`、`internal/app/apptest/github.go`
- Modify: `internal/infra/machineconfig/accounts.go`(照合テストを通すため Update・Remove を足す)
- Modify: `internal/infra/secret/source.go`・`keyring.go`(口を満たすため。中身は Task 2)

**Interfaces:**
- Produces:
  - `account.DeviceToken{Access, Refresh string; ExpiresAt, RefreshExpiresAt time.Time}`、`account.RefreshBefore`、`(DeviceToken) Fresh(now) bool`、`(DeviceToken) RefreshExpired(now) bool`
  - `account.StoreUse{Name string; Remote bool}`、`account.DeleteLines(a Account, uses []StoreUse, machineLogin string) []string`、`account.EditLines(a Account, newLogin string, uses []StoreUse) []string`
  - `port.Accounts` に `Update(ctx, a) error`・`Remove(ctx, id) error`(どちらも、なければ `port.ErrNotFound`)
  - `port.Secrets` に `DeviceToken(ctx, login) (account.DeviceToken, error)`・`SaveDeviceToken(ctx, login, t) error`・`DeleteDeviceToken(ctx, login) error`
  - `port.DeviceFlow`、`port.DeviceCode`、`port.DeviceGrant`、`port.DeviceStatus`(`DevicePending`・`DeviceSlowDown`・`DeviceExpired`・`DeviceDenied`・`DeviceGranted`)
  - `port.Hosts`(`ClientID(ctx) (string, error)`)
  - `port.StoreGitHub`(`List`・`Unassign`・`MachineLogin`)、`port.StoreGitHubInfo{ID StoreID; Name string; Account account.ID; Remote bool}`
  - 偽物: `apptest.DeviceFlow`、`apptest.Hosts`、`apptest.StoreGitHub`、`apptest.Secrets.Device`

- [ ] **Step 1: domain の失敗するテストを書く**

`internal/domain/account/device_test.go`:

```go
package account

import (
	"testing"
	"time"
)

func TestDeviceTokenFresh(t *testing.T) {
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		tok  DeviceToken
		want bool
	}{
		{"no end", DeviceToken{Access: "a"}, true},
		{"ends in 6 minutes", DeviceToken{Access: "a", ExpiresAt: now.Add(6 * time.Minute)}, true},
		{"ends in 5 minutes", DeviceToken{Access: "a", ExpiresAt: now.Add(5 * time.Minute)}, false},
		{"ended", DeviceToken{Access: "a", ExpiresAt: now.Add(-time.Second)}, false},
	}
	for _, c := range cases {
		if got := c.tok.Fresh(now); got != c.want {
			t.Errorf("%s: Fresh = %v", c.name, got)
		}
	}
}

func TestDeviceTokenRefreshExpired(t *testing.T) {
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	if (DeviceToken{}).RefreshExpired(now) {
		t.Error("no end of the refresh token is not expired")
	}
	if !(DeviceToken{RefreshExpiresAt: now}).RefreshExpired(now) {
		t.Error("the refresh token that ends now is expired")
	}
	if (DeviceToken{RefreshExpiresAt: now.Add(time.Hour)}).RefreshExpired(now) {
		t.Error("the refresh token that ends later is not expired")
	}
}
```

`internal/domain/account/impact_test.go`:

```go
package account

import (
	"slices"
	"testing"
)

func TestDeleteLines(t *testing.T) {
	a := Account{ID: "A1", Name: "work", Login: "taro", Method: GhCLI}
	uses := []StoreUse{{Name: "チーム", Remote: true}, {Name: "個人", Remote: false}, {Name: "OSS", Remote: true}}
	got := DeleteLines(a, uses, "hanako")
	want := []string{
		"「work」を削除します。",
		"チーム・OSS の同期に使っているため、それらは同期できなくなります。",
		"チーム での あなた が @taro から hanako に変わります。",
		"個人 での あなた が @taro から hanako に変わります。",
		"OSS での あなた が @taro から hanako に変わります。",
	}
	if !slices.Equal(got, want) {
		t.Errorf("DeleteLines =\n%q\nwant\n%q", got, want)
	}
}

func TestDeleteLinesOnlyWhatApplies(t *testing.T) {
	a := Account{ID: "A1", Name: "work", Login: "taro", Method: GhCLI}
	// remote がなく、マシンのログイン名が同じ人(大文字と小文字は区別しない)なら、1 行目だけ。
	got := DeleteLines(a, []StoreUse{{Name: "個人"}}, "Taro")
	if !slices.Equal(got, []string{"「work」を削除します。"}) {
		t.Errorf("DeleteLines = %q", got)
	}
}

func TestEditLines(t *testing.T) {
	a := Account{ID: "A1", Name: "work", Login: "taro", Method: GhCLI}
	uses := []StoreUse{{Name: "チーム", Remote: true}, {Name: "個人"}}
	want := []string{
		"チーム での あなた が @taro から jiro に変わります。",
		"個人 での あなた が @taro から jiro に変わります。",
	}
	if got := EditLines(a, " jiro ", uses); !slices.Equal(got, want) {
		t.Errorf("EditLines = %q", got)
	}
	if got := EditLines(a, "TARO", uses); len(got) != 0 {
		t.Errorf("same login: EditLines = %q", got)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/account/`
Expected: FAIL(`undefined: DeviceToken`・`undefined: DeleteLines`)

- [ ] **Step 3: domain を書く**

`internal/domain/account/device.go`:

```go
package account

import "time"

// RefreshBefore は、期限付きのトークンを、期限のどれだけ前から取り直すか(前回の実装と同じ)。
const RefreshBefore = 5 * time.Minute

// DeviceToken は、デバイスフローで得たトークンの組。OS のキーチェーンにだけ置く(07「デバイスフロー」)。
type DeviceToken struct {
	Access           string
	Refresh          string    // 更新用のトークン。期限のないトークンでは空のことがある
	ExpiresAt        time.Time // Access の期限。ゼロなら期限なし(expires_in が 0)
	RefreshExpiresAt time.Time // Refresh の期限。ゼロなら分からない
}

// Fresh は、まだ取り直さなくてよいか(期限がないか、期限まで RefreshBefore より長い)。
func (t DeviceToken) Fresh(now time.Time) bool {
	return t.ExpiresAt.IsZero() || now.Add(RefreshBefore).Before(t.ExpiresAt)
}

// RefreshExpired は、更新用のトークンの期限が過ぎたか。
func (t DeviceToken) RefreshExpired(now time.Time) bool {
	return !t.RefreshExpiresAt.IsZero() && !now.Before(t.RefreshExpiresAt)
}
```

`internal/domain/account/impact.go`:

```go
package account

import (
	"fmt"
	"strings"
)

// StoreUse は、アカウントを割り当てた Store(非表示・閲覧のみを含む)。削除・編集の確認に使う。
type StoreUse struct {
	Name   string
	Remote bool
}

// DeleteLines は、07「削除の影響」の確認の本文を、当てはまる部品だけ、この順に返す。
// uses は a を割り当てた Store、machineLogin はこのマシンの設定「ログイン名」(削除後の「あなた」)。
func DeleteLines(a Account, uses []StoreUse, machineLogin string) []string {
	lines := []string{"「" + a.Name + "」を削除します。"}
	var synced []string
	for _, u := range uses {
		if u.Remote {
			synced = append(synced, u.Name)
		}
	}
	if len(synced) > 0 {
		lines = append(lines, strings.Join(synced, "・")+" の同期に使っているため、それらは同期できなくなります。")
	}
	return append(lines, identityLines(a.Login, machineLogin, uses)...)
}

// EditLines は、07「編集」の「アカウントを変更」の確認の本文。「あなた」が変わる Store がなければ空。
func EditLines(a Account, newLogin string, uses []StoreUse) []string {
	return identityLines(a.Login, strings.TrimSpace(newLogin), uses)
}

func identityLines(from, to string, uses []StoreUse) []string {
	if strings.EqualFold(from, to) {
		return nil
	}
	lines := make([]string, 0, len(uses))
	for _, u := range uses {
		lines = append(lines, fmt.Sprintf("%s での あなた が @%s から %s に変わります。", u.Name, from, to))
	}
	return lines
}
```

- [ ] **Step 4: domain のテストが通ることを確かめる**

Run: `go test ./internal/domain/account/`
Expected: PASS

- [ ] **Step 5: 口を足す**

`internal/app/port/accounts.go` を次にする:

```go
package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// Accounts は、このマシンに登録したアカウントの一覧の口(01「マシンごとに持つ」)。
type Accounts interface {
	List(ctx context.Context) ([]account.Account, error) // 登録した順
	Add(ctx context.Context, a account.Account) error
	// Update は、ID が同じアカウントの値を入れ替える(並びは変えない)。なければ ErrNotFound。
	Update(ctx context.Context, a account.Account) error
	// Remove は、アカウントを一覧から外す。なければ ErrNotFound。
	Remove(ctx context.Context, id account.ID) error
}
```

`internal/app/port/secrets.go` を次にする:

```go
package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// Secrets はトークンの口。トークンは、この口の外では、問い合わせに渡すあいだだけ持つ。
type Secrets interface {
	// Token は、アカウントの認証方法でトークンを読む(デバイスフローは除く)。読めなければ *ghcall.Failure
	// (GhTokenMissing・GhTimeout・EnvMissing・KeychainMissing・KeychainUnavailable)を返す。
	Token(ctx context.Context, a account.Account) (string, error)
	// DeviceToken は、デバイスフローで得たトークンの組を読む。項目がなければ TokenRejected、
	// キーチェーンがなければ KeychainUnavailable の *ghcall.Failure。
	DeviceToken(ctx context.Context, login string) (account.DeviceToken, error)
	// SaveDeviceToken は、デバイスフローで得たトークンの組を置く。キーチェーンがなければ KeychainUnavailable。
	SaveDeviceToken(ctx context.Context, login string, t account.DeviceToken) error
	// DeleteDeviceToken は、デバイスフローで得たトークンの組を消す。項目がなくてもエラーにしない。
	DeleteDeviceToken(ctx context.Context, login string) error
}
```

`internal/app/port/deviceflow.go`:

```go
package port

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// DeviceCode は、デバイスフローの始まりに GitHub が返すもの。DeviceCode は画面に出さない。
type DeviceCode struct {
	DeviceCode      string
	UserCode        string // XXXX-XXXX
	VerificationURI string
	ExpiresIn       time.Duration
	Interval        time.Duration
}

// DeviceStatus は、許可を待つ問い合わせの答え。
type DeviceStatus string

const (
	DevicePending  DeviceStatus = "pending"
	DeviceSlowDown DeviceStatus = "slow_down" // 間隔を 5 秒延ばす
	DeviceExpired  DeviceStatus = "expired"
	DeviceDenied   DeviceStatus = "denied"
	DeviceGranted  DeviceStatus = "granted"
)

// DeviceGrant は、許可を待つ問い合わせの結果。Token は Granted のときだけ。
type DeviceGrant struct {
	Status DeviceStatus
	Token  account.DeviceToken
}

// DeviceFlow は、GitHub のデバイスフローの口(07「デバイスフロー」)。失敗は *ghcall.Failure。
type DeviceFlow interface {
	StartDevice(ctx context.Context, clientID string) (DeviceCode, error)
	PollDevice(ctx context.Context, clientID, deviceCode string) (DeviceGrant, error)
	// RefreshDevice は、更新用のトークンで取り直す。GitHub が拒否したら TokenRejected。
	RefreshDevice(ctx context.Context, clientID, refresh string) (account.DeviceToken, error)
}
```

`internal/app/port/hosts.go`:

```go
package port

import "context"

// Hosts は、config.yaml の hosts の口。
type Hosts interface {
	// ClientID は、ホスト github.com のデバイスフローの client_id。書かれていなければ空。
	ClientID(ctx context.Context) (string, error)
}
```

`internal/app/port/storegithub.go`:

```go
package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// StoreGitHubInfo は、Store の GitHub に関わる値(08「GitHub に使うアカウント」)。
type StoreGitHubInfo struct {
	ID      StoreID
	Name    string
	Account account.ID // 割り当てたアカウント。「なし」なら空
	Remote  bool       // remote を持つか
}

// StoreGitHub は、Store の一覧のうち GitHub に関わる値と、マシンの設定「ログイン名」の口。
// 本物の実装はトラック G が足す(トラック D の決定 1)。
type StoreGitHub interface {
	// List は、このマシンのすべての Store(非表示・閲覧のみを含む)を返す。
	List(ctx context.Context) ([]StoreGitHubInfo, error)
	// Unassign は、id を割り当てた Store の割り当てを「なし」にする(07「削除の影響」)。
	Unassign(ctx context.Context, id account.ID) error
	// MachineLogin は、このマシンの設定「ログイン名」(01「「あなた」の決まり方」)。
	MachineLogin(ctx context.Context) (string, error)
}
```

- [ ] **Step 6: 照合テストに Update・Remove を足す(失敗するテスト)**

`internal/app/apptest/accounts_contract.go` の `RunAccountsContract` の最後に足す:

```go
	t.Run("updates in place and removes", func(t *testing.T) {
		accs := newAccounts(t)
		a := account.Account{ID: "01ACC00000000000000000001", Name: "work", Login: "taro", Method: account.GhCLI}
		b := account.Account{ID: "01ACC00000000000000000002", Name: "家", Login: "taro-home", Method: account.DeviceFlow}
		for _, x := range []account.Account{a, b} {
			if err := accs.Add(context.Background(), x); err != nil {
				t.Fatal(err)
			}
		}
		a2 := account.Account{ID: a.ID, Name: "仕事", Login: "jiro", Method: account.Keychain}
		if err := accs.Update(context.Background(), a2); err != nil {
			t.Fatal(err)
		}
		if err := accs.Remove(context.Background(), b.ID); err != nil {
			t.Fatal(err)
		}
		got, err := accs.List(context.Background())
		if err != nil || !slices.Equal(got, []account.Account{a2}) {
			t.Errorf("List = %+v, %v", got, err)
		}
		missing := account.Account{ID: "01ACC00000000000000000009", Name: "x", Login: "x", Method: account.GhCLI}
		if err := accs.Update(context.Background(), missing); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("Update missing = %v", err)
		}
		if err := accs.Remove(context.Background(), missing.ID); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("Remove missing = %v", err)
		}
	})
```

`import` に `"errors"` を足す。

Run: `go test ./internal/app/apptest/ ./internal/infra/machineconfig/`
Expected: ビルドの失敗(`*Accounts does not implement port.Accounts (missing method Remove)`)

- [ ] **Step 7: 偽物と config.yaml の Update・Remove を書く**

`internal/app/apptest/accounts.go` に足す(`import` に `"github.com/kukv/hedgehug/internal/app/port"`):

```go
func (a *Accounts) Update(_ context.Context, acc account.Account) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.list {
		if a.list[i].ID == acc.ID {
			a.list[i] = acc
			return nil
		}
	}
	return port.ErrNotFound
}

func (a *Accounts) Remove(_ context.Context, id account.ID) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.list {
		if a.list[i].ID == id {
			a.list = slices.Delete(a.list, i, i+1)
			return nil
		}
	}
	return port.ErrNotFound
}
```

`internal/infra/machineconfig/accounts.go` に足す(`import` に `"slices"`):

```go
func (a accounts) Update(_ context.Context, acc account.Account) error {
	return a.f.update(func(c *config) error {
		for i := range c.Accounts {
			if c.Accounts[i].ID == string(acc.ID) {
				c.Accounts[i] = toYAML(acc)
				return nil
			}
		}
		return port.ErrNotFound
	})
}

func (a accounts) Remove(_ context.Context, id account.ID) error {
	return a.f.update(func(c *config) error {
		for i := range c.Accounts {
			if c.Accounts[i].ID == string(id) {
				c.Accounts = slices.Delete(c.Accounts, i, i+1)
				return nil
			}
		}
		return port.ErrNotFound
	})
}

func toYAML(acc account.Account) accountYAML {
	return accountYAML{ID: string(acc.ID), Name: acc.Name, Login: acc.Login, Method: string(acc.Method)}
}
```

`Add` の中の `accountYAML{...}` を `toYAML(acc)` に替える。

- [ ] **Step 8: Secrets の偽物と本物を口に合わせる**

`internal/app/apptest/github.go` の `Secrets` を次にする(`Device` はログイン名からトークンの組。`NoKeychain` が true なら、キーチェーンがない環境):

```go
// Secrets は port.Secrets の偽物。Tokens はログイン名からトークンへの対応。ない、または Err があれば失敗する。
// Device はデバイスフローのトークンの組(ログイン名から)。NoKeychain なら、デバイスフローのトークンを置けない。
type Secrets struct {
	mu                sync.Mutex
	Tokens            map[string]string
	Device            map[string]account.DeviceToken
	NoKeychain        bool
	NoKeychainForSave bool // 読めるが置けない(取り直したトークンを置けないとき)
	Err               error
}

func (s *Secrets) DeviceToken(_ context.Context, login string) (account.DeviceToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.NoKeychain {
		return account.DeviceToken{}, &ghcall.Failure{Kind: ghcall.KeychainUnavailable}
	}
	t, ok := s.Device[login]
	if !ok {
		return account.DeviceToken{}, &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	return t, nil
}

func (s *Secrets) SaveDeviceToken(_ context.Context, login string, t account.DeviceToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.NoKeychain || s.NoKeychainForSave {
		return &ghcall.Failure{Kind: ghcall.KeychainUnavailable}
	}
	if s.Device == nil {
		s.Device = map[string]account.DeviceToken{}
	}
	s.Device[login] = t
	return nil
}

func (s *Secrets) DeleteDeviceToken(_ context.Context, login string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Device, login)
	return nil
}
```

本物(`infra/secret`)はこの時点では口を満たすだけにし、中身は Task 2 で書く。`internal/infra/secret/source.go` に足す:

```go
func (s *Source) DeviceToken(context.Context, string) (account.DeviceToken, error) {
	return account.DeviceToken{}, errors.New("not yet")
}

func (s *Source) SaveDeviceToken(context.Context, string, account.DeviceToken) error {
	return errors.New("not yet")
}

func (s *Source) DeleteDeviceToken(context.Context, string) error { return errors.New("not yet") }
```

(Task 2 の Step 3 で本物に置き換える。「not yet」を develop に入れないため、Task 1 と Task 2 は同じ共有の PR に入れる。)

- [ ] **Step 9: DeviceFlow・Hosts・StoreGitHub の偽物を書く**

`internal/app/apptest/deviceflow.go`:

```go
package apptest

import (
	"context"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// DeviceFlow は port.DeviceFlow の偽物。Polls は PollDevice が順に返す答え(使い切ったら最後のものを返し続ける)。
// StartErr・PollErr があれば、その問い合わせはそれを返す。Refreshed は RefreshDevice が返すトークンの組
// (Access が空なら TokenRejected)。Starts・Refreshes は呼ばれた回数。
type DeviceFlow struct {
	mu        sync.Mutex
	Code      port.DeviceCode
	Polls     []port.DeviceGrant
	StartErr  error
	PollErr   error
	Refreshed account.DeviceToken
	RefreshErr error
	Starts    int
	Refreshes int
}

// NewDeviceFlow は、コード WDJB-MJHT、期限 15 分、間隔 5 秒で始まり、ずっと待っている偽物を返す。
func NewDeviceFlow() *DeviceFlow {
	return &DeviceFlow{
		Code: port.DeviceCode{
			DeviceCode: "device-1", UserCode: "WDJB-MJHT", VerificationURI: "https://github.com/login/device",
			ExpiresIn: 15 * time.Minute, Interval: 5 * time.Second,
		},
		Polls: []port.DeviceGrant{{Status: port.DevicePending}},
	}
}

func (d *DeviceFlow) StartDevice(context.Context, string) (port.DeviceCode, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Starts++
	if d.StartErr != nil {
		return port.DeviceCode{}, d.StartErr
	}
	return d.Code, nil
}

func (d *DeviceFlow) PollDevice(context.Context, string, string) (port.DeviceGrant, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.PollErr != nil {
		return port.DeviceGrant{}, d.PollErr
	}
	g := d.Polls[0]
	if len(d.Polls) > 1 {
		d.Polls = d.Polls[1:]
	}
	return g, nil
}

func (d *DeviceFlow) RefreshDevice(context.Context, string, string) (account.DeviceToken, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Refreshes++
	if d.RefreshErr != nil {
		return account.DeviceToken{}, d.RefreshErr
	}
	if d.Refreshed.Access == "" {
		return account.DeviceToken{}, &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	return d.Refreshed, nil
}

// Hosts は port.Hosts の偽物。
type Hosts struct{ ID string }

func (h Hosts) ClientID(context.Context) (string, error) { return h.ID, nil }
```

`internal/app/apptest/storegithub.go`:

```go
package apptest

import (
	"context"
	"slices"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// StoreGitHub は port.StoreGitHub の偽物。
type StoreGitHub struct {
	mu     sync.Mutex
	Stores []port.StoreGitHubInfo
	Login  string // マシンの設定「ログイン名」
}

func (s *StoreGitHub) List(context.Context) ([]port.StoreGitHubInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.Stores), nil
}

func (s *StoreGitHub) Unassign(_ context.Context, id account.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Stores {
		if s.Stores[i].Account == id {
			s.Stores[i].Account = ""
		}
	}
	return nil
}

func (s *StoreGitHub) MachineLogin(context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Login, nil
}
```

- [ ] **Step 10: 全体のテストと lint を通す**

Run: `go test ./internal/... && make lint`
Expected: PASS。`port.Accounts` の照合テストが偽物と config.yaml の両方で通る。

- [ ] **Step 11: コミット**

```bash
git add internal/domain/account internal/app/port internal/app/apptest internal/infra/machineconfig/accounts.go internal/infra/secret/source.go
git commit -m "feat(app): アカウントの編集・削除とデバイスフローの口と偽物を足す(D2 の共有部分)"
```

---

### Task 2: キーチェーンのデバイスフローのトークン(infra/secret)

**Files:**
- Modify: `internal/infra/secret/keyring.go`、`internal/infra/secret/source.go`
- Create: `internal/infra/secret/device.go`、`internal/infra/secret/device_test.go`
- Modify: `internal/infra/secret/source_test.go`(`fakeKeyring` に Set・Delete、キーチェーンの JSON の読み方)

**Interfaces:**
- Consumes: `port.Secrets`(Task 1)、`account.DeviceToken`
- Produces: `secret.Keyring` に `Set(service, user, value string) error`・`Delete(service, user string) error`。`(*Source).DeviceToken`・`SaveDeviceToken`・`DeleteDeviceToken`

- [ ] **Step 1: 失敗するテストを書く**

`internal/infra/secret/source_test.go` の `fakeKeyring` に足す:

```go
func (k fakeKeyring) Set(service, user, value string) error {
	if k == nil {
		return errors.New("no secret service on this machine")
	}
	k[service+"|"+user] = value
	return nil
}

func (k fakeKeyring) Delete(service, user string) error {
	if k == nil {
		return errors.New("no secret service on this machine")
	}
	if _, ok := k[service+"|"+user]; !ok {
		return ErrNotFound
	}
	delete(k, service+"|"+user)
	return nil
}
```

`TestDeviceFlowIsNotReadHere` は残す(`Token` はデバイスフローを読まない)。`TestKeychainToken` の最後に足す:

```go
	// デバイスフローが置いた JSON の項目は、access_token をトークンとして読む(この小計画の決定 D2-5)。
	s.Keyring = fakeKeyring{"hedgehug|github.com/taro": `{"access_token":"tok-device","refresh_token":"r"}`}
	if tok, err := s.Token(context.Background(), acc(account.Keychain)); err != nil || tok != "tok-device" {
		t.Errorf("device JSON: Token = %q, %v", tok, err)
	}
```

`internal/infra/secret/device_test.go`:

```go
package secret

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

func TestDeviceTokenRoundTrip(t *testing.T) {
	k := fakeKeyring{}
	s := &Source{Keyring: k}
	want := account.DeviceToken{
		Access: "tok-a", Refresh: "tok-r",
		ExpiresAt:        time.Date(2026, 11, 13, 18, 0, 0, 0, time.UTC),
		RefreshExpiresAt: time.Date(2027, 5, 13, 10, 0, 0, 0, time.UTC),
	}
	if err := s.SaveDeviceToken(context.Background(), "taro", want); err != nil {
		t.Fatal(err)
	}
	if v := k["hedgehug|github.com/taro"]; !strings.Contains(v, `"access_token":"tok-a"`) {
		t.Errorf("kept %q", v)
	}
	got, err := s.DeviceToken(context.Background(), "taro")
	if err != nil || !got.ExpiresAt.Equal(want.ExpiresAt) || got.Access != want.Access || got.Refresh != want.Refresh ||
		!got.RefreshExpiresAt.Equal(want.RefreshExpiresAt) {
		t.Errorf("DeviceToken = %+v, %v", got, err)
	}
	if err := s.DeleteDeviceToken(context.Background(), "taro"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteDeviceToken(context.Background(), "taro"); err != nil {
		t.Errorf("deleting what is not there: %v", err)
	}
	if _, err := s.DeviceToken(context.Background(), "taro"); !ghcall.Is(err, ghcall.TokenRejected) {
		t.Errorf("after delete: %v", err)
	}
}

func TestDeviceTokenPlainValue(t *testing.T) {
	// OS のキーチェーンの認証方法から替えたときの、素のトークン(期限なし)。
	s := &Source{Keyring: fakeKeyring{"hedgehug|github.com/taro": " tok-plain "}}
	got, err := s.DeviceToken(context.Background(), "taro")
	if err != nil || got.Access != "tok-plain" || !got.ExpiresAt.IsZero() {
		t.Errorf("DeviceToken = %+v, %v", got, err)
	}
}

func TestDeviceTokenNoKeychain(t *testing.T) {
	s := &Source{Keyring: fakeKeyring(nil)}
	if _, err := s.DeviceToken(context.Background(), "taro"); !ghcall.Is(err, ghcall.KeychainUnavailable) {
		t.Errorf("read: %v", err)
	}
	if err := s.SaveDeviceToken(context.Background(), "taro", account.DeviceToken{Access: "tok-a"}); !ghcall.Is(err, ghcall.KeychainUnavailable) {
		t.Errorf("save: %v", err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/infra/secret/`
Expected: FAIL(`fakeKeyring does not implement Keyring` ではなく、`Keyring` に Set がないので `s.Keyring.Set undefined`、または `not yet` で失敗)

- [ ] **Step 3: 書く**

`internal/infra/secret/keyring.go` を次にする:

```go
package secret

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// ErrNotFound は、キーチェーンに項目がないこと。ほかの失敗は、キーチェーンそのものが使えないこと。
var ErrNotFound = errors.New("not in the keychain")

// osKeyring は OS のキーチェーン(macOS のキーチェーン、Linux の Secret Service)。
type osKeyring struct{}

func (osKeyring) Get(service, user string) (string, error) {
	v, err := keyring.Get(service, user)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return v, err
}

func (osKeyring) Set(service, user, value string) error { return keyring.Set(service, user, value) }

func (osKeyring) Delete(service, user string) error {
	err := keyring.Delete(service, user)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
```

`internal/infra/secret/source.go` の `Keyring` を次にし、Task 1 の仮の 3 つを消す:

```go
// Keyring は OS のキーチェーン。テストでは差し替える。
type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, value string) error
	Delete(service, user string) error
}
```

`Token` の `case account.Keychain:` の最後の `return strings.TrimSpace(v), nil` を次にする:

```go
		return accessOf(v), nil
```

`internal/infra/secret/device.go`:

```go
package secret

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// deviceJSON は、デバイスフローのトークンの組をキーチェーンに置く形(前回の実装と同じ。この小計画の決定 D2-5)。
type deviceJSON struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token,omitempty"`
	ExpiresAt        time.Time `json:"expires_at,omitzero"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at,omitzero"`
}

// parseDevice は、キーチェーンの値を読む。JSON でなければ、値そのものを期限のないトークンとして読む。
func parseDevice(v string) account.DeviceToken {
	var d deviceJSON
	if err := json.Unmarshal([]byte(v), &d); err != nil || d.AccessToken == "" {
		return account.DeviceToken{Access: strings.TrimSpace(v)}
	}
	return account.DeviceToken{
		Access: strings.TrimSpace(d.AccessToken), Refresh: d.RefreshToken,
		ExpiresAt: d.ExpiresAt, RefreshExpiresAt: d.RefreshExpiresAt,
	}
}

// accessOf は、キーチェーンの値のうち、問い合わせに使うトークン。
func accessOf(v string) string { return parseDevice(v).Access }

func (s *Source) DeviceToken(_ context.Context, login string) (account.DeviceToken, error) {
	v, err := s.Keyring.Get(Service, KeychainKey(login))
	switch {
	case errors.Is(err, ErrNotFound):
		return account.DeviceToken{}, &ghcall.Failure{Kind: ghcall.TokenRejected}
	case err != nil:
		return account.DeviceToken{}, &ghcall.Failure{Kind: ghcall.KeychainUnavailable}
	}
	t := parseDevice(v)
	if t.Access == "" {
		return account.DeviceToken{}, &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	return t, nil
}

func (s *Source) SaveDeviceToken(_ context.Context, login string, t account.DeviceToken) error {
	b, err := json.Marshal(deviceJSON{ //nolint:gosec // G117: JSON はキーチェーンにだけ置く
		AccessToken: t.Access, RefreshToken: t.Refresh, ExpiresAt: t.ExpiresAt, RefreshExpiresAt: t.RefreshExpiresAt,
	})
	if err != nil {
		return err
	}
	if err := s.Keyring.Set(Service, KeychainKey(login), string(b)); err != nil {
		return &ghcall.Failure{Kind: ghcall.KeychainUnavailable}
	}
	return nil
}

func (s *Source) DeleteDeviceToken(_ context.Context, login string) error {
	err := s.Keyring.Delete(Service, KeychainKey(login))
	if err == nil || errors.Is(err, ErrNotFound) {
		return nil
	}
	return &ghcall.Failure{Kind: ghcall.KeychainUnavailable}
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/infra/secret/ && make lint`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/infra/secret
git commit -m "feat(infra): デバイスフローのトークンの組をキーチェーンに JSON で置く"
```

---

### Task 3: config.yaml の hosts(infra/machineconfig)

**Files:**
- Modify: `internal/infra/machineconfig/file.go`
- Create: `internal/infra/machineconfig/hosts.go`、`internal/infra/machineconfig/hosts_test.go`

**Interfaces:**
- Consumes: `port.Hosts`(Task 1)
- Produces: `(*machineconfig.File) Hosts() port.Hosts`

- [ ] **Step 1: 失敗するテストを書く**

`internal/infra/machineconfig/hosts_test.go`:

```go
package machineconfig

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/account"
)

func TestClientID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if id, err := Open(path).Hosts().ClientID(context.Background()); err != nil || id != "" {
		t.Errorf("no file: %q, %v", id, err)
	}
	body := "hosts:\n  - {host: example.com, client_id: other}\n  - {host: github.com, client_id: \" Iv23abc \"}\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if id, err := Open(path).Hosts().ClientID(context.Background()); err != nil || id != "Iv23abc" {
		t.Errorf("ClientID = %q, %v", id, err)
	}
}

func TestHostsSurviveAccountWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("hosts:\n  - {host: github.com, client_id: Iv23abc}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := Open(path)
	a := account.Account{ID: "01ACC00000000000000000001", Name: "work", Login: "taro", Method: account.GhCLI}
	if err := f.Accounts().Add(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if id, err := f.Hosts().ClientID(context.Background()); err != nil || id != "Iv23abc" {
		t.Errorf("after Add: %q, %v", id, err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/infra/machineconfig/`
Expected: FAIL(`f.Hosts undefined`)

- [ ] **Step 3: 書く**

`internal/infra/machineconfig/file.go` の `config` を次にする:

```go
// config は config.yaml の中身。項目は、持つトラックが足す(トラック G が Store の一覧などを足す)。
type config struct {
	Accounts []accountYAML `yaml:"accounts"`
	Hosts    []hostYAML    `yaml:"hosts,omitempty"`
}
```

`internal/infra/machineconfig/hosts.go`:

```go
package machineconfig

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// hostYAML は hosts の 1 行(前回と同じ形。host と client_id だけを読む。この小計画の決定 D2-14)。
type hostYAML struct {
	Host     string `yaml:"host"`
	ClientID string `yaml:"client_id,omitempty"`
}

// Hosts は、config.yaml の hosts を読む port.Hosts。
func (f *File) Hosts() port.Hosts { return hosts{f} }

type hosts struct{ f *File }

func (h hosts) ClientID(context.Context) (string, error) {
	c, err := h.f.read()
	if err != nil {
		return "", err
	}
	for _, x := range c.Hosts {
		if strings.TrimSpace(x.Host) == account.Host {
			return strings.TrimSpace(x.ClientID), nil
		}
	}
	return "", nil
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/infra/machineconfig/ && make lint`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/infra/machineconfig
git commit -m "feat(infra): config.yaml の hosts からデバイスフローの client_id を読む"
```

---

### Task 4: GitHub のデバイスフロー(infra/github)

**Files:**
- Modify: `internal/infra/github/client.go`(`WebURL` と `postForm`)
- Create: `internal/infra/github/device.go`、`internal/infra/github/device_test.go`

**Interfaces:**
- Consumes: `port.DeviceFlow`・`port.DeviceCode`・`port.DeviceGrant`(Task 1)
- Produces: `(*github.Client) StartDevice`・`PollDevice`・`RefreshDevice`、`Client.WebURL`(既定は `https://github.com`)

GitHub のデバイスフローは `api.github.com` ではなく `github.com/login/...` に、フォームで POST する。失敗は 200 の本文の `error` で返る。

- [ ] **Step 1: 失敗するテストを書く**

`internal/infra/github/device_test.go`:

```go
package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

func deviceServer(t *testing.T, answer func(form map[string]string) any) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Accept") != "application/json" {
			t.Errorf("%s %s accept %q", r.Method, r.URL.Path, r.Header.Get("Accept"))
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		form := map[string]string{"path": r.URL.Path}
		for k := range r.PostForm {
			form[k] = r.PostForm.Get(k)
		}
		_ = json.NewEncoder(w).Encode(answer(form))
	}))
	t.Cleanup(srv.Close)
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	return &Client{WebURL: srv.URL, HTTP: srv.Client(), Now: func() time.Time { return now }}
}

func TestStartDevice(t *testing.T) {
	c := deviceServer(t, func(form map[string]string) any {
		if form["path"] != "/login/device/code" || form["client_id"] != "Iv23abc" {
			t.Errorf("form %v", form)
		}
		return map[string]any{"device_code": "dc", "user_code": "WDJB-MJHT", "verification_uri": "https://github.com/login/device", "expires_in": 900, "interval": 5}
	})
	got, err := c.StartDevice(context.Background(), "Iv23abc")
	want := port.DeviceCode{DeviceCode: "dc", UserCode: "WDJB-MJHT", VerificationURI: "https://github.com/login/device", ExpiresIn: 15 * time.Minute, Interval: 5 * time.Second}
	if err != nil || got != want {
		t.Errorf("StartDevice = %+v, %v", got, err)
	}
}

func TestStartDeviceRefused(t *testing.T) {
	c := deviceServer(t, func(map[string]string) any { return map[string]any{"error": "unauthorized_client"} })
	if _, err := c.StartDevice(context.Background(), "bad"); !ghcall.Is(err, ghcall.Unreachable) {
		t.Errorf("StartDevice = %v", err)
	}
}

func TestPollDevice(t *testing.T) {
	cases := map[string]port.DeviceStatus{
		"authorization_pending": port.DevicePending,
		"slow_down":             port.DeviceSlowDown,
		"expired_token":         port.DeviceExpired,
		"access_denied":         port.DeviceDenied,
		"incorrect_device_code": port.DeviceExpired,
	}
	for code, want := range cases {
		c := deviceServer(t, func(form map[string]string) any {
			if form["path"] != "/login/oauth/access_token" || form["device_code"] != "dc" ||
				form["grant_type"] != "urn:ietf:params:oauth:grant-type:device_code" {
				t.Errorf("form %v", form)
			}
			return map[string]any{"error": code}
		})
		if g, err := c.PollDevice(context.Background(), "Iv23abc", "dc"); err != nil || g.Status != want {
			t.Errorf("%s: %+v, %v", code, g, err)
		}
	}
}

func TestPollDeviceGranted(t *testing.T) {
	c := deviceServer(t, func(map[string]string) any {
		return map[string]any{"access_token": "tok-a", "refresh_token": "tok-r", "expires_in": 28800, "refresh_token_expires_in": 15897600}
	})
	g, err := c.PollDevice(context.Background(), "Iv23abc", "dc")
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	if err != nil || g.Status != port.DeviceGranted || g.Token.Access != "tok-a" || g.Token.Refresh != "tok-r" ||
		!g.Token.ExpiresAt.Equal(now.Add(8*time.Hour)) || !g.Token.RefreshExpiresAt.Equal(now.Add(15897600*time.Second)) {
		t.Errorf("PollDevice = %+v, %v", g, err)
	}
}

func TestPollDeviceNoEnd(t *testing.T) {
	c := deviceServer(t, func(map[string]string) any { return map[string]any{"access_token": "tok-a", "expires_in": 0} })
	g, err := c.PollDevice(context.Background(), "Iv23abc", "dc")
	if err != nil || !g.Token.ExpiresAt.IsZero() || !g.Token.RefreshExpiresAt.IsZero() {
		t.Errorf("PollDevice = %+v, %v", g, err)
	}
}

func TestRefreshDevice(t *testing.T) {
	c := deviceServer(t, func(form map[string]string) any {
		if form["grant_type"] != "refresh_token" || form["refresh_token"] != "tok-r" || form["client_id"] != "Iv23abc" {
			t.Errorf("form %v", form)
		}
		return map[string]any{"access_token": "tok-b", "refresh_token": "tok-r2", "expires_in": 28800}
	})
	got, err := c.RefreshDevice(context.Background(), "Iv23abc", "tok-r")
	if err != nil || got.Access != "tok-b" || got.Refresh != "tok-r2" {
		t.Errorf("RefreshDevice = %+v, %v", got, err)
	}
	bad := deviceServer(t, func(map[string]string) any { return map[string]any{"error": "bad_refresh_token"} })
	if _, err := bad.RefreshDevice(context.Background(), "Iv23abc", "tok-r"); !ghcall.Is(err, ghcall.TokenRejected) {
		t.Errorf("rejected: %v", err)
	}
}

func TestDeviceUnreachable(t *testing.T) {
	c := &Client{WebURL: "http://127.0.0.1:1", HTTP: http.DefaultClient, Now: time.Now}
	if _, err := c.PollDevice(context.Background(), "Iv23abc", "dc"); !ghcall.Is(err, ghcall.Unreachable) {
		t.Errorf("PollDevice = %v", err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/infra/github/`
Expected: FAIL(`unknown field WebURL`)

- [ ] **Step 3: 書く**

`internal/infra/github/client.go` の定数に `webURL = "https://github.com"` を足し、`Client` に `WebURL string` を足し、`New` で `WebURL: webURL` を入れる:

```go
const (
	apiURL     = "https://api.github.com"
	webURL     = "https://github.com"
	apiVersion = "2022-11-28"
	// timeout は 1 回の問い合わせの打ち切り(07「GitHub との通信に失敗したとき」)。
	timeout = 30 * time.Second
	// maxBody は、読む応答の大きさの上限。
	maxBody = 10 << 20
)

// Client は GitHub の API に問い合わせる。
type Client struct {
	BaseURL string
	WebURL  string // デバイスフローの宛先(github.com)
	HTTP    *http.Client
	Now     func() time.Time
	// OnSkew は、応答の Date からこのマシンの時計を引いたずれを、応答のたびに受け取る(トラック C が記録する)。
	OnSkew func(time.Duration)
}

func New(onSkew func(time.Duration)) *Client {
	return &Client{BaseURL: apiURL, WebURL: webURL, HTTP: &http.Client{Timeout: timeout}, Now: time.Now, OnSkew: onSkew}
}
```

`internal/infra/github/device.go`:

```go
package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// tokenAnswer は /login/oauth/access_token の答え。失敗は 200 の本文の error で返る。
type tokenAnswer struct {
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
	Error                 string `json:"error"`
}

// StartDevice は、デバイスフローを始める(POST /login/device/code)。
func (c *Client) StartDevice(ctx context.Context, clientID string) (port.DeviceCode, error) {
	var a struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
		Error           string `json:"error"`
	}
	if err := c.postForm(ctx, "/login/device/code", url.Values{"client_id": {clientID}}, &a); err != nil {
		return port.DeviceCode{}, err
	}
	if a.Error != "" || a.DeviceCode == "" {
		return port.DeviceCode{}, &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	return port.DeviceCode{
		DeviceCode: a.DeviceCode, UserCode: a.UserCode, VerificationURI: a.VerificationURI,
		ExpiresIn: time.Duration(a.ExpiresIn) * time.Second, Interval: time.Duration(a.Interval) * time.Second,
	}, nil
}

// PollDevice は、許可されたかを 1 回だけ聞く。
func (c *Client) PollDevice(ctx context.Context, clientID, deviceCode string) (port.DeviceGrant, error) {
	var a tokenAnswer
	err := c.postForm(ctx, "/login/oauth/access_token", url.Values{
		"client_id": {clientID}, "device_code": {deviceCode},
		"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"},
	}, &a)
	if err != nil {
		return port.DeviceGrant{}, err
	}
	switch a.Error {
	case "":
		if a.AccessToken == "" {
			return port.DeviceGrant{}, &ghcall.Failure{Kind: ghcall.Unreachable}
		}
		return port.DeviceGrant{Status: port.DeviceGranted, Token: c.tokenOf(a)}, nil
	case "authorization_pending":
		return port.DeviceGrant{Status: port.DevicePending}, nil
	case "slow_down":
		return port.DeviceGrant{Status: port.DeviceSlowDown}, nil
	case "access_denied":
		return port.DeviceGrant{Status: port.DeviceDenied}, nil
	}
	// expired_token と、コードがもう使えないほかの答え(incorrect_device_code など)。再試行で新しいコードを出す。
	return port.DeviceGrant{Status: port.DeviceExpired}, nil
}

// RefreshDevice は、更新用のトークンで取り直す。
func (c *Client) RefreshDevice(ctx context.Context, clientID, refresh string) (account.DeviceToken, error) {
	var a tokenAnswer
	err := c.postForm(ctx, "/login/oauth/access_token", url.Values{
		"client_id": {clientID}, "refresh_token": {refresh}, "grant_type": {"refresh_token"},
	}, &a)
	if err != nil {
		return account.DeviceToken{}, err
	}
	if a.Error != "" || a.AccessToken == "" {
		return account.DeviceToken{}, &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	return c.tokenOf(a), nil
}

// tokenOf は、答えのトークンの組。expires_in が 0 なら期限なし。
func (c *Client) tokenOf(a tokenAnswer) account.DeviceToken {
	now := c.Now()
	t := account.DeviceToken{Access: a.AccessToken, Refresh: a.RefreshToken}
	if a.ExpiresIn > 0 {
		t.ExpiresAt = now.Add(time.Duration(a.ExpiresIn) * time.Second)
	}
	if a.RefreshTokenExpiresIn > 0 {
		t.RefreshExpiresAt = now.Add(time.Duration(a.RefreshTokenExpiresIn) * time.Second)
	}
	return t
}

// postForm は、WebURL+path にフォームを POST し、200 なら本文を out に読む。失敗は *ghcall.Failure。
func (c *Client) postForm(ctx context.Context, path string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.WebURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	defer func() { _ = res.Body.Close() }()
	if f := c.classify(res); f != nil {
		return f
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(out); err != nil {
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	return nil
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/infra/github/ && make lint`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/infra/github
git commit -m "feat(infra): GitHub のデバイスフローと期限付きトークンの取り直しを足す"
```

---

### Task 5: トークンを得る入口と状態(app/github の Session・Connector・一覧)

**Files:**
- Create: `internal/app/github/session.go`、`internal/app/github/verify.go`、`internal/app/github/connector.go`、`internal/app/github/connector_test.go`
- Modify: `internal/app/github/add_account.go`(`verifyToken` を使う)、`internal/app/github/list_accounts.go`、`internal/app/github/accounts_test.go`

**Interfaces:**
- Consumes: Task 1 の口と偽物
- Produces:
  - `github.NewSession() *Session`
  - `github.Connector{Session, Accounts, Secrets, GitHub, Device, Hosts, Stores, Clock}`、`(Connector) Token(ctx, a account.Account) (string, error)`、`(Connector) TokenFor(ctx, store port.StoreID, repo string) (string, account.Account, error)`、`(Connector) Rejected(id account.ID)`
  - `github.ListAccounts{Accounts, Session}`、`(ListAccounts) Do(ctx) ([]AccountState, error)`、`github.AccountState{Account account.Account; AuthError bool; Reason string}`
  - パッケージの中: `verifyToken(ctx, secrets, gh, a) ([32]byte, error)`、`fingerprint(token) [32]byte`、`(*Session) markOK(id, fp)`・`markBroken(id, reason)`・`state(id) (broken bool, reason string)`・`checked(id, fp) bool`・`forget(id)`

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/github/connector_test.go`:

```go
package github_test

import (
	"context"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

var start = time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

type connFixture struct {
	accounts *apptest.Accounts
	secrets  *apptest.Secrets
	gh       *countingGitHub
	device   *apptest.DeviceFlow
	stores   *apptest.StoreGitHub
	clock    *apptest.Clock
	session  *github.Session
	conn     github.Connector
}

// countingGitHub は、Viewer を呼んだ回数を数える。
type countingGitHub struct {
	apptest.GitHub
	calls int
}

func (g *countingGitHub) Viewer(ctx context.Context, token string) (string, error) {
	g.calls++
	return g.GitHub.Viewer(ctx, token)
}

var (
	work = account.Account{ID: "A1", Name: "work", Login: "taro", Method: account.GhCLI}
	home = account.Account{ID: "A2", Name: "家", Login: "hanako", Method: account.DeviceFlow}
)

func newConn(t *testing.T) connFixture {
	t.Helper()
	f := connFixture{
		accounts: apptest.NewAccounts(),
		secrets:  &apptest.Secrets{Tokens: map[string]string{"taro": "tok-taro"}},
		gh:       &countingGitHub{GitHub: apptest.GitHub{Users: map[string]string{"tok-taro": "taro", "tok-hanako": "hanako"}}},
		device:   apptest.NewDeviceFlow(),
		stores:   &apptest.StoreGitHub{Stores: []port.StoreGitHubInfo{{ID: "S1", Name: "チーム", Account: "A1", Remote: true}, {ID: "S2", Name: "個人"}}},
		clock:    apptest.NewClock(start),
		session:  github.NewSession(),
	}
	for _, a := range []account.Account{work, home} {
		if err := f.accounts.Add(context.Background(), a); err != nil {
			t.Fatal(err)
		}
	}
	f.conn = github.Connector{
		Session: f.session, Accounts: f.accounts, Secrets: f.secrets, GitHub: f.gh, Device: f.device,
		Hosts: apptest.Hosts{ID: "Iv23abc"}, Stores: f.stores, Clock: f.clock,
	}
	return f
}

func TestTokenChecksOnceUntilTheTokenChanges(t *testing.T) {
	f := newConn(t)
	for range 2 {
		if tok, err := f.conn.Token(context.Background(), work); err != nil || tok != "tok-taro" {
			t.Fatalf("Token = %q, %v", tok, err)
		}
	}
	if f.gh.calls != 1 {
		t.Errorf("asked GitHub %d times; want 1 (the first connection only)", f.gh.calls)
	}
	f.secrets.Tokens["taro"] = "tok-taro2"
	f.gh.Users["tok-taro2"] = "taro"
	if _, err := f.conn.Token(context.Background(), work); err != nil || f.gh.calls != 2 {
		t.Errorf("changed token: %v, calls %d", err, f.gh.calls)
	}
}

func TestTokenOfAnotherPersonIsAnAuthError(t *testing.T) {
	f := newConn(t)
	f.secrets.Tokens["taro"] = "tok-hanako"
	_, err := f.conn.Token(context.Background(), work)
	if !ghcall.Is(err, ghcall.AuthError) {
		t.Fatalf("Token = %v", err)
	}
	list, _ := github.ListAccounts{Accounts: f.accounts, Session: f.session}.Do(context.Background())
	if !list[0].AuthError || list[0].Reason != "このトークンは @hanako のものです。ログイン名を直してください" {
		t.Errorf("state = %+v", list[0])
	}
	// 認証エラーのあいだは、別のトークンになっても問い合わせない(戻るのは再認証か、D4 の自動の確かめ直し)。
	f.secrets.Tokens["taro"] = "tok-taro"
	calls := f.gh.calls
	if _, err := f.conn.Token(context.Background(), work); !ghcall.Is(err, ghcall.AuthError) || f.gh.calls != calls {
		t.Errorf("while broken: %v, calls %d → %d", err, calls, f.gh.calls)
	}
}

func TestRejectedTokenIsAnAuthError(t *testing.T) {
	f := newConn(t)
	f.secrets.Tokens["taro"] = "tok-revoked"
	_, err := f.conn.Token(context.Background(), work)
	var fail *ghcall.Failure
	if !errorsAs(err, &fail) || fail.Kind != ghcall.AuthError || fail.Name != "work" || fail.Login != "taro" {
		t.Fatalf("Token = %v", err)
	}
	list, _ := github.ListAccounts{Accounts: f.accounts, Session: f.session}.Do(context.Background())
	if !list[0].AuthError || list[0].Reason != "" {
		t.Errorf("state = %+v", list[0])
	}
}

func TestUnreachableIsNotAnAuthError(t *testing.T) {
	f := newConn(t)
	f.gh.Err = &ghcall.Failure{Kind: ghcall.Unreachable}
	if _, err := f.conn.Token(context.Background(), work); !ghcall.Is(err, ghcall.Unreachable) {
		t.Fatalf("Token = %v", err)
	}
	list, _ := github.ListAccounts{Accounts: f.accounts, Session: f.session}.Do(context.Background())
	if list[0].AuthError {
		t.Error("an unreachable GitHub made the account an auth error")
	}
}

func TestRejectedByALaterCall(t *testing.T) {
	f := newConn(t)
	f.conn.Rejected(work.ID)
	if _, err := f.conn.Token(context.Background(), work); !ghcall.Is(err, ghcall.AuthError) {
		t.Errorf("Token = %v", err)
	}
}

func TestDeviceTokenFreshIsUsedAsItIs(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-hanako", Refresh: "r", ExpiresAt: start.Add(time.Hour)}}
	if tok, err := f.conn.Token(context.Background(), home); err != nil || tok != "tok-hanako" {
		t.Errorf("Token = %q, %v", tok, err)
	}
	if f.gh.calls != 0 || f.device.Refreshes != 0 {
		t.Errorf("device flow token was checked (%d) or refreshed (%d)", f.gh.calls, f.device.Refreshes)
	}
}

func TestDeviceTokenIsRefreshedBeforeItEnds(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-old", Refresh: "r", ExpiresAt: start.Add(4 * time.Minute)}}
	f.device.Refreshed = account.DeviceToken{Access: "tok-new", Refresh: "r2", ExpiresAt: start.Add(8 * time.Hour)}
	if tok, err := f.conn.Token(context.Background(), home); err != nil || tok != "tok-new" {
		t.Fatalf("Token = %q, %v", tok, err)
	}
	if got := f.secrets.Device["hanako"]; got.Access != "tok-new" || got.Refresh != "r2" {
		t.Errorf("kept %+v", got)
	}
}

func TestDeviceRefreshFailuresAreAuthErrors(t *testing.T) {
	cases := map[string]func(f *connFixture){
		"rejected":         func(*connFixture) {},
		"refresh expired":  func(f *connFixture) { f.secrets.Device["hanako"] = account.DeviceToken{Access: "tok-old", Refresh: "r", ExpiresAt: start, RefreshExpiresAt: start} },
		"no client id":     func(f *connFixture) { f.conn.Hosts = apptest.Hosts{} },
		"not in keychain":  func(f *connFixture) { delete(f.secrets.Device, "hanako") },
		"cannot keep it": func(f *connFixture) {
			f.device.Refreshed = account.DeviceToken{Access: "tok-new"}
			f.secrets.NoKeychainForSave = true
		},
	}
	for name, setup := range cases {
		f := newConn(t)
		f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-old", Refresh: "r", ExpiresAt: start}}
		setup(&f)
		if _, err := f.conn.Token(context.Background(), home); !ghcall.Is(err, ghcall.AuthError) {
			t.Errorf("%s: Token = %v", name, err)
		}
	}
}

func TestDeviceRefreshUnreachableKeepsTheAccount(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-old", Refresh: "r", ExpiresAt: start}}
	f.device.RefreshErr = &ghcall.Failure{Kind: ghcall.Unreachable}
	if _, err := f.conn.Token(context.Background(), home); !ghcall.Is(err, ghcall.Unreachable) {
		t.Fatalf("Token = %v", err)
	}
	list, _ := github.ListAccounts{Accounts: f.accounts, Session: f.session}.Do(context.Background())
	if list[1].AuthError {
		t.Error("an unreachable GitHub made the account an auth error")
	}
}

func TestTokenFor(t *testing.T) {
	f := newConn(t)
	tok, a, err := f.conn.TokenFor(context.Background(), "S1", "kukv/hedgehug")
	if err != nil || tok != "tok-taro" || a.ID != work.ID {
		t.Errorf("S1: %q %+v %v", tok, a, err)
	}
	_, _, err = f.conn.TokenFor(context.Background(), "S2", "kukv/hedgehug")
	var fail *ghcall.Failure
	if !errorsAs(err, &fail) || fail.Kind != ghcall.NoAccount || fail.Repo != "kukv/hedgehug" {
		t.Errorf("S2 (no account): %v", err)
	}
}
```

`errorsAs` は `accounts_test.go` に置く小さな関数:

```go
func errorsAs(err error, target **ghcall.Failure) bool { return errors.As(err, target) }
```

`accounts_test.go` の既存のテストのうち、`ListAccounts` を使うものがあれば、`Do` の戻り値の `[]AccountState` に合わせて `.Account` を付ける。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/github/`
Expected: FAIL(`undefined: github.NewSession`)

- [ ] **Step 3: Session を書く**

`internal/app/github/session.go`:

```go
package github

import (
	"crypto/sha256"
	"sync"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// Session は、アプリを開いているあいだだけ持つアカウントの状態。再起動すると消え、最初の接続で照合し直す
// (この小計画の決定 D2-2・D2-3)。トークンは持たず、指紋(SHA-256)だけを持つ。
type Session struct {
	mu      sync.Mutex
	fps     map[account.ID][32]byte // 最後に確かめたトークンの指紋(07「接続のたびの照合」)
	broken  map[account.ID]string   // 認証エラーと、行に出す理由(空なら理由の文言なし)
	refresh sync.Mutex              // 期限付きトークンの取り直しを 1 つずつ行う(決定 D2-6)
}

func NewSession() *Session {
	return &Session{fps: map[account.ID][32]byte{}, broken: map[account.ID]string{}}
}

func fingerprint(token string) [32]byte { return sha256.Sum256([]byte(token)) }

// markOK は、本人を確かめたことを記録し、認証エラーを外す。fp がゼロ値なら指紋は覚えない(デバイスフロー)。
func (s *Session) markOK(id account.ID, fp [32]byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.broken, id)
	if fp != ([32]byte{}) {
		s.fps[id] = fp
	}
}

func (s *Session) markBroken(id account.ID, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.broken[id] = reason
	delete(s.fps, id)
}

func (s *Session) state(id account.ID) (broken bool, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reason, broken = s.broken[id]
	return broken, reason
}

func (s *Session) checked(id account.ID, fp [32]byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	got, ok := s.fps[id]
	return ok && got == fp
}

func (s *Session) forget(id account.ID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.broken, id)
	delete(s.fps, id)
}
```

- [ ] **Step 4: verifyToken と Connector を書く**

`internal/app/github/verify.go`:

```go
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
```

`internal/app/github/add_account.go` の `verify` の、トークンを読むところから最後までを次にする:

```go
	if a.Method == account.DeviceFlow {
		return account.Account{}, ErrDeviceFlow
	}
	if _, err := verifyToken(ctx, u.Secrets, u.GitHub, a); err != nil {
		return account.Account{}, err
	}
	return a, nil
```

(`strings` の import を外す。)

`internal/app/github/connector.go`:

```go
package github

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// Connector は、GitHub に接続するたびにトークンを得る入口(07「接続のたびの照合」「デバイスフロー」の期限付き
// トークン、「GitHub との通信に使うアカウント」)。認証エラーへの変更はここに集める。
type Connector struct {
	Session  *Session
	Accounts port.Accounts
	Secrets  port.Secrets
	GitHub   port.GitHub
	Device   port.DeviceFlow
	Hosts    port.Hosts
	Stores   port.StoreGitHub
	Clock    port.Clock
}

// TokenFor は、Store に割り当てたアカウントのトークンと、そのアカウントを返す。割り当てがなければ
// NoAccount(repo は文言に使う)。認証エラーなら AuthError。別のアカウントでは読まない。
func (c Connector) TokenFor(ctx context.Context, store port.StoreID, repo string) (string, account.Account, error) {
	a, err := c.storeAccount(ctx, store)
	if err != nil {
		return "", account.Account{}, err
	}
	if a.ID == "" {
		return "", account.Account{}, &ghcall.Failure{Kind: ghcall.NoAccount, Repo: repo}
	}
	tok, err := c.Token(ctx, a)
	return tok, a, err
}

func (c Connector) storeAccount(ctx context.Context, store port.StoreID) (account.Account, error) {
	stores, err := c.Stores.List(ctx)
	if err != nil {
		return account.Account{}, err
	}
	var id account.ID
	for _, s := range stores {
		if s.ID == store {
			id = s.Account
		}
	}
	if id == "" {
		return account.Account{}, nil
	}
	accs, err := c.Accounts.List(ctx)
	if err != nil {
		return account.Account{}, err
	}
	for _, a := range accs {
		if a.ID == id {
			return a, nil
		}
	}
	return account.Account{}, nil
}

// Rejected は、a のトークンが GitHub に拒否された(401)ことを知らせ、認証エラーにする(07「状態」)。
func (c Connector) Rejected(id account.ID) { c.Session.markBroken(id, "") }

// Token は、a のトークンを返す。認証エラーなら AuthError の Failure。
func (c Connector) Token(ctx context.Context, a account.Account) (string, error) {
	if broken, _ := c.Session.state(a.ID); broken {
		return "", authError(a)
	}
	if a.Method == account.DeviceFlow {
		return c.deviceToken(ctx, a)
	}
	tok, err := c.Secrets.Token(ctx, a)
	if err != nil {
		return "", err
	}
	fp := fingerprint(tok)
	if c.Session.checked(a.ID, fp) {
		return tok, nil
	}
	login, err := c.GitHub.Viewer(ctx, tok)
	switch {
	case ghcall.Is(err, ghcall.TokenRejected):
		c.Session.markBroken(a.ID, "")
		return "", authError(a)
	case err != nil:
		return "", err
	case !strings.EqualFold(login, a.Login):
		c.Session.markBroken(a.ID, mismatchText(login))
		return "", authError(a)
	}
	c.Session.markOK(a.ID, fp)
	return tok, nil
}

// deviceToken は、デバイスフローのトークン。期限が近ければ取り直す(この小計画の決定 D2-6・D2-7)。
func (c Connector) deviceToken(ctx context.Context, a account.Account) (string, error) {
	t, err := c.Secrets.DeviceToken(ctx, a.Login)
	if err != nil {
		return "", c.brokenOr(a, err)
	}
	if t.Fresh(c.Clock.Now()) {
		return t.Access, nil
	}
	c.Session.refresh.Lock()
	defer c.Session.refresh.Unlock()
	if t, err = c.Secrets.DeviceToken(ctx, a.Login); err != nil {
		return "", c.brokenOr(a, err)
	}
	if t.Fresh(c.Clock.Now()) {
		return t.Access, nil // ロックを待つあいだに、ほかの接続が取り直した
	}
	clientID, err := c.Hosts.ClientID(ctx)
	if err != nil {
		return "", err
	}
	if t.Refresh == "" || t.RefreshExpired(c.Clock.Now()) || clientID == "" {
		c.Session.markBroken(a.ID, "")
		return "", authError(a)
	}
	nt, err := c.Device.RefreshDevice(ctx, clientID, t.Refresh)
	if err != nil {
		return "", c.brokenOr(a, err)
	}
	if err := c.Secrets.SaveDeviceToken(ctx, a.Login, nt); err != nil {
		// 古い更新用のトークンはもう使えないので、新しい組を置けなければ使えない。
		c.Session.markBroken(a.ID, "")
		return "", authError(a)
	}
	return nt.Access, nil
}

// brokenOr は、拒否(TokenRejected)なら認証エラーにし、それ以外の失敗はそのまま返す。
func (c Connector) brokenOr(a account.Account, err error) error {
	if ghcall.Is(err, ghcall.TokenRejected) {
		c.Session.markBroken(a.ID, "")
		return authError(a)
	}
	return err
}

func authError(a account.Account) error {
	return &ghcall.Failure{Kind: ghcall.AuthError, Name: a.Name, Login: a.Login}
}

// mismatchText は、トークンのログイン名が登録と違うときの文言(07「接続のたびの照合」。登録と同じ文言)。
func mismatchText(login string) string {
	return "このトークンは @" + login + " のものです。ログイン名を直してください"
}
```

`mismatchText` は、文言を 2 か所に持たないよう、`ghcall.Refuse` で作る(`refusal.Refusal.Error()` は文言そのもの)。上の `connector.go` の `mismatchText` を次にし、`import` に `"time"` を足す:

```go
func mismatchText(login string) string {
	return ghcall.Refuse(&ghcall.Failure{Kind: ghcall.LoginMismatch, Login: login}, time.Time{}).Error()
}
```

- [ ] **Step 5: ListAccounts に状態を付ける**

`internal/app/github/list_accounts.go`:

```go
package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// AccountState は、一覧の 1 行(07「状態」)。GitHub App 未導入は D4 が足す。
type AccountState struct {
	Account   account.Account
	AuthError bool
	Reason    string // 「認証エラー」の下に出す赤字(ログイン名の不一致)。なければ空
}

// ListAccounts は、登録したアカウントを登録した順に、状態を付けて返す。
type ListAccounts struct {
	Accounts port.Accounts
	Session  *Session
}

func (u ListAccounts) Do(ctx context.Context) ([]AccountState, error) {
	accs, err := u.Accounts.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AccountState, 0, len(accs))
	for _, a := range accs {
		broken, reason := u.Session.state(a.ID)
		out = append(out, AccountState{Account: a, AuthError: broken, Reason: reason})
	}
	return out, nil
}
```

- [ ] **Step 6: 通ることを確かめる**

Run: `go test ./internal/app/github/ && make lint`
Expected: PASS(httpapi の `list.Do` の呼び方は Task 8 で直すので、`go build ./...` はまだ失敗してよい。この Task だけで PR にするときは、`internal/adapter/httpapi/github/accounts.go` の一覧の組み立てを `toBody(s.Account)` にし、`routes.go` の `ListAccounts` に `Session: ghapp.NewSession()` を渡す)

- [ ] **Step 7: コミット**

```bash
git add internal/app/github internal/app/apptest/github.go
git commit -m "feat(app): 接続のたびの照合と期限付きトークンの取り直しを、トークンを得る入口に集める"
```

---

### Task 6: 再認証・編集・削除と削除の影響(app/github)

**Files:**
- Create: `internal/app/github/reauth.go`、`internal/app/github/edit_account.go`、`internal/app/github/delete_account.go`、`internal/app/github/manage_test.go`

**Interfaces:**
- Consumes: `Session`・`verifyToken`(Task 5)、`account.DeleteLines`・`EditLines`(Task 1)
- Produces:
  - `github.Reauth{Session, Accounts, Secrets, GitHub, Clock}`、`(Reauth) Do(ctx, id account.ID) (account.Account, error)`
  - `github.EditAccount{Session, Accounts, Secrets, GitHub, Clock}`、`github.EditAccountInput{ID account.ID; Name, Login string; Method account.Method}`、`(EditAccount) Do(ctx, in) (account.Account, error)`
  - `github.DeleteAccount{Session, Accounts, Secrets, Stores}`、`(DeleteAccount) Do(ctx, id) error`
  - `github.Impact{Accounts, Stores}`、`(Impact) Delete(ctx, id) ([]string, error)`、`(Impact) Edit(ctx, id, login string) ([]string, error)`
  - パッケージの中: `find(ctx, accounts, id) (account.Account, error)`(なければ `port.ErrNotFound`)

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/github/manage_test.go`(Task 5 の `newConn`・`work`・`home` を使う):

```go
package github_test

import (
	"context"
	"errors"
	"slices"
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
	f.secrets.Err = nil
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
	if _, err := f.edit().Do(context.Background(), github.EditAccountInput{ID: work.ID, Name: "WORK", Login: "TARO", Method: account.GhCLI}); err != nil {
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

func TestDeleteAccount(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-hanako"}}
	f.stores.Stores[1].Account = home.ID
	del := github.DeleteAccount{Session: f.session, Accounts: f.accounts, Secrets: f.secrets, Stores: f.stores}
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
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/github/`
Expected: FAIL(`undefined: github.Reauth`)

- [ ] **Step 3: 書く**

`internal/app/github/reauth.go`:

```go
package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// Reauth は、gh CLI・環境変数・OS のキーチェーンのアカウントの認証の確認をやり直す(07「再認証」)。
// デバイスフローはサインイン(SignIn)でやり直す。失敗しても状態は変えない。
type Reauth struct {
	Session  *Session
	Accounts port.Accounts
	Secrets  port.Secrets
	GitHub   port.GitHub
	Clock    port.Clock
}

func (u Reauth) Do(ctx context.Context, id account.ID) (account.Account, error) {
	a, err := find(ctx, u.Accounts, id)
	if err != nil {
		return account.Account{}, err
	}
	if a.Method == account.DeviceFlow {
		return account.Account{}, ErrDeviceFlow
	}
	fp, err := verifyToken(ctx, u.Secrets, u.GitHub, a)
	if err != nil {
		return account.Account{}, ghcall.Refuse(err, u.Clock.Now())
	}
	u.Session.markOK(a.ID, fp)
	return a, nil
}

// find は、ID のアカウント。なければ port.ErrNotFound。
func find(ctx context.Context, accounts port.Accounts, id account.ID) (account.Account, error) {
	accs, err := accounts.List(ctx)
	if err != nil {
		return account.Account{}, err
	}
	for _, a := range accs {
		if a.ID == id {
			return a, nil
		}
	}
	return account.Account{}, port.ErrNotFound
}
```

`internal/app/github/edit_account.go`:

```go
package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// EditAccount は、アカウントの値を入れ替える(07「編集」)。新しい値で認証の確認をして、確かめられたときだけ保存する。
// 新しい認証方法がデバイスフローなら、サインイン(SignIn)で保存する。Store の割り当ては ID で持つので変わらない。
type EditAccount struct {
	Session  *Session
	Accounts port.Accounts
	Secrets  port.Secrets
	GitHub   port.GitHub
	Clock    port.Clock
}

type EditAccountInput struct {
	ID     account.ID
	Name   string
	Login  string
	Method account.Method
}

func (u EditAccount) Do(ctx context.Context, in EditAccountInput) (account.Account, error) {
	a, fp, err := u.verify(ctx, in)
	if err != nil {
		return account.Account{}, ghcall.Refuse(err, u.Clock.Now())
	}
	if err := u.Accounts.Update(ctx, a); err != nil {
		return account.Account{}, err
	}
	u.Session.markOK(a.ID, fp)
	return a, nil
}

func (u EditAccount) verify(ctx context.Context, in EditAccountInput) (account.Account, [32]byte, error) {
	existing, err := u.Accounts.List(ctx)
	if err != nil {
		return account.Account{}, [32]byte{}, err
	}
	if _, err := find(ctx, u.Accounts, in.ID); err != nil {
		return account.Account{}, [32]byte{}, err
	}
	a, err := account.Check(account.Account{Name: in.Name, Login: in.Login, Method: in.Method}, existing, in.ID)
	if err != nil {
		return account.Account{}, [32]byte{}, err
	}
	a.ID = in.ID
	if a.Method == account.DeviceFlow {
		return account.Account{}, [32]byte{}, ErrDeviceFlow
	}
	fp, err := verifyToken(ctx, u.Secrets, u.GitHub, a)
	return a, fp, err
}
```

`internal/app/github/delete_account.go`:

```go
package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// DeleteAccount は、アカウントをすぐに削除する(07「削除の影響」)。割り当てた Store は「なし」になる。
// デバイスフローのトークンはキーチェーンから消す(この小計画の決定 D2-12)。始まっている処理は、取り出し済みの
// トークンで最後まで続く(トークンは Connector の外では、その処理のあいだだけ持つ)。
type DeleteAccount struct {
	Session  *Session
	Accounts port.Accounts
	Secrets  port.Secrets
	Stores   port.StoreGitHub
}

func (u DeleteAccount) Do(ctx context.Context, id account.ID) error {
	a, err := find(ctx, u.Accounts, id)
	if err != nil {
		return err
	}
	// 先に割り当てを外す。一覧から外したあとで失敗すると、どの Store にも見えない割り当てが残るため。
	if err := u.Stores.Unassign(ctx, id); err != nil {
		return err
	}
	if err := u.Accounts.Remove(ctx, id); err != nil {
		return err
	}
	u.Session.forget(id)
	if a.Method == account.DeviceFlow {
		return u.Secrets.DeleteDeviceToken(ctx, a.Login)
	}
	return nil
}

// Impact は、削除と編集の確認の本文を組む(07「削除の影響」「編集」、この小計画の決定 D2-10)。
type Impact struct {
	Accounts port.Accounts
	Stores   port.StoreGitHub
}

// Delete は、「アカウントを削除」の確認の本文の行。
func (u Impact) Delete(ctx context.Context, id account.ID) ([]string, error) {
	a, uses, err := u.uses(ctx, id)
	if err != nil {
		return nil, err
	}
	login, err := u.Stores.MachineLogin(ctx)
	if err != nil {
		return nil, err
	}
	return account.DeleteLines(a, uses, login), nil
}

// Edit は、ログイン名を login に変えるときの「アカウントを変更」の確認の本文の行。空なら確認を出さない。
func (u Impact) Edit(ctx context.Context, id account.ID, login string) ([]string, error) {
	a, uses, err := u.uses(ctx, id)
	if err != nil {
		return nil, err
	}
	return account.EditLines(a, login, uses), nil
}

func (u Impact) uses(ctx context.Context, id account.ID) (account.Account, []account.StoreUse, error) {
	a, err := find(ctx, u.Accounts, id)
	if err != nil {
		return account.Account{}, nil, err
	}
	stores, err := u.Stores.List(ctx)
	if err != nil {
		return account.Account{}, nil, err
	}
	var uses []account.StoreUse
	for _, s := range stores {
		if s.Account == id {
			uses = append(uses, account.StoreUse{Name: s.Name, Remote: s.Remote})
		}
	}
	return a, uses, nil
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/github/ && make lint`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/github
git commit -m "feat(app): アカウントの再認証・編集・削除と、削除の影響の文を足す"
```

---

### Task 7: デバイスフローのサインイン(app/github の SignIn)

**Files:**
- Modify: `internal/app/github/session.go`(`flows`)
- Create: `internal/app/github/sign_in.go`、`internal/app/github/sign_in_test.go`

**Interfaces:**
- Consumes: `Session`・`find`(Task 5・6)、`port.DeviceFlow`・`port.Hosts`
- Produces:
  - `github.SignIn{Session, Accounts, Secrets, GitHub, Device, Hosts, IDs, Clock}`
  - `github.SignInInput{Account account.ID; Name, Login string}`(`Account` が空なら登録、あれば編集か再認証)
  - `github.SignInCode{ID string; UserCode, VerificationURI string; ExpiresIn, Interval time.Duration}`
  - `github.SignInState`(`SignInWaiting`・`SignInExpired`・`SignInDenied`・`SignInMismatch`・`SignInFailed`・`SignInDone`)
  - `github.SignInResult{State SignInState; Message string; Interval time.Duration; Account account.Account}`
  - `(SignIn) Start(ctx, in) (SignInCode, error)`、`(SignIn) Poll(ctx, id string) (SignInResult, error)`(流れがなければ `port.ErrNotFound`)、`(SignIn) Cancel(id string)`

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/github/sign_in_test.go`:

```go
package github_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

func (f connFixture) signIn() github.SignIn {
	return github.SignIn{
		Session: f.session, Accounts: f.accounts, Secrets: f.secrets, GitHub: f.gh, Device: f.device,
		Hosts: apptest.Hosts{ID: "Iv23abc"}, IDs: apptest.NewIDs(), Clock: f.clock,
	}
}

var granted = port.DeviceGrant{Status: port.DeviceGranted, Token: account.DeviceToken{Access: "tok-saburo", Refresh: "r"}}

func TestSignInAddsAnAccount(t *testing.T) {
	f := newConn(t)
	f.gh.Users["tok-saburo"] = "saburo"
	f.device.Polls = []port.DeviceGrant{{Status: port.DevicePending}, granted}
	s := f.signIn()
	code, err := s.Start(context.Background(), github.SignInInput{Name: " 副業 ", Login: "saburo"})
	if err != nil || code.ID == "" || code.UserCode != "WDJB-MJHT" || code.Interval != 5*time.Second {
		t.Fatalf("Start = %+v, %v", code, err)
	}
	if r, err := s.Poll(context.Background(), code.ID); err != nil || r.State != github.SignInWaiting {
		t.Fatalf("1st Poll = %+v, %v", r, err)
	}
	r, err := s.Poll(context.Background(), code.ID)
	if err != nil || r.State != github.SignInDone || r.Account.Name != "副業" || r.Account.Method != account.DeviceFlow {
		t.Fatalf("2nd Poll = %+v, %v", r, err)
	}
	if list, _ := f.accounts.List(context.Background()); len(list) != 3 || list[2] != r.Account {
		t.Errorf("accounts %+v", list)
	}
	if f.secrets.Device["saburo"].Access != "tok-saburo" {
		t.Errorf("keychain %+v", f.secrets.Device)
	}
	// 終わった流れは、もう問い合わせられない。
	if _, err := s.Poll(context.Background(), code.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("after done: %v", err)
	}
}

func TestSignInStartRefusals(t *testing.T) {
	f := newConn(t)
	s := f.signIn()
	if _, err := s.Start(context.Background(), github.SignInInput{Name: "work", Login: "x"}); message(err) != "同じ表示名のアカウントがあります" {
		t.Errorf("same name: %v", err)
	}
	s.Hosts = apptest.Hosts{}
	want := "デバイスフローを使うには、config.yaml の hosts に github.com の client_id を設定してください。gh CLI のトークン・環境変数・OS のキーチェーンは、設定なしで使えます"
	if _, err := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "x"}); message(err) != want {
		t.Errorf("no client id: %v", err)
	}
	s.Hosts = apptest.Hosts{ID: "Iv23abc"}
	f.device.StartErr = &ghcall.Failure{Kind: ghcall.Unreachable}
	if _, err := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "x"}); message(err) != "GitHub に接続できません。接続を確かめて、もう一度お試しください" {
		t.Errorf("unreachable: %v", err)
	}
	if f.count(t) != 2 {
		t.Error("an account was added")
	}
}

func (f connFixture) count(t *testing.T) int {
	t.Helper()
	list, _ := f.accounts.List(context.Background())
	return len(list)
}

func TestSignInWaitsWhileOffline(t *testing.T) {
	f := newConn(t)
	f.device.PollErr = &ghcall.Failure{Kind: ghcall.Unreachable}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "x"})
	if r, err := s.Poll(context.Background(), code.ID); err != nil || r.State != github.SignInWaiting {
		t.Errorf("Poll = %+v, %v", r, err)
	}
}

func TestSignInSlowDown(t *testing.T) {
	f := newConn(t)
	f.device.Polls = []port.DeviceGrant{{Status: port.DeviceSlowDown}}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "x"})
	if r, _ := s.Poll(context.Background(), code.ID); r.State != github.SignInWaiting || r.Interval != 10*time.Second {
		t.Errorf("Poll = %+v", r)
	}
}

func TestSignInEnds(t *testing.T) {
	cases := map[port.DeviceStatus]github.SignInState{port.DeviceExpired: github.SignInExpired, port.DeviceDenied: github.SignInDenied}
	for status, want := range cases {
		f := newConn(t)
		f.device.Polls = []port.DeviceGrant{{Status: status}}
		s := f.signIn()
		code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "x"})
		if r, _ := s.Poll(context.Background(), code.ID); r.State != want {
			t.Errorf("%s: %+v", status, r)
		}
		if _, err := s.Poll(context.Background(), code.ID); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: the flow is left: %v", status, err)
		}
	}
}

func TestSignInExpiresByTheClock(t *testing.T) {
	f := newConn(t)
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "x"})
	f.clock.Step = 16 * time.Minute
	f.clock.Now()
	if r, _ := s.Poll(context.Background(), code.ID); r.State != github.SignInExpired {
		t.Errorf("Poll = %+v", r)
	}
}

func TestSignInByAnotherPerson(t *testing.T) {
	f := newConn(t)
	f.gh.Users["tok-saburo"] = "shiro"
	f.device.Polls = []port.DeviceGrant{granted}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "saburo"})
	r, err := s.Poll(context.Background(), code.ID)
	if err != nil || r.State != github.SignInMismatch || r.Message != "このトークンは @shiro のものです。ログイン名を直してください" {
		t.Errorf("Poll = %+v, %v", r, err)
	}
	if f.count(t) != 2 || len(f.secrets.Device) != 0 {
		t.Error("an account or a token was kept")
	}
}

func TestSignInWithoutKeychain(t *testing.T) {
	f := newConn(t)
	f.gh.Users["tok-saburo"] = "saburo"
	f.secrets.NoKeychain = true
	f.device.Polls = []port.DeviceGrant{granted}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "saburo"})
	r, _ := s.Poll(context.Background(), code.ID)
	if r.State != github.SignInFailed || r.Message != "このマシンでは OS のキーチェーンを使えません。gh CLI のトークンか環境変数を使ってください" || f.count(t) != 2 {
		t.Errorf("Poll = %+v", r)
	}
}

func TestSignInReauthsAndEdits(t *testing.T) {
	f := newConn(t)
	f.gh.Users["tok-saburo"] = "hanako"
	f.device.Polls = []port.DeviceGrant{granted}
	f.conn.Rejected(home.ID)
	s := f.signIn()
	// 再認証: いまの値のまま、アカウントの ID を渡す。
	code, err := s.Start(context.Background(), github.SignInInput{Account: home.ID, Name: home.Name, Login: home.Login})
	if err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Poll(context.Background(), code.ID); r.State != github.SignInDone || r.Account != home {
		t.Fatalf("reauth: %+v", r)
	}
	if f.state(t, 1).AuthError || f.count(t) != 2 {
		t.Errorf("after reauth: %+v", f.state(t, 1))
	}
	// 編集: work を、デバイスフローの saburo に替える。古いキーチェーンの項目は、デバイスフローでなかったので触らない。
	f.gh.Users["tok-saburo"] = "saburo"
	f.device.Polls = []port.DeviceGrant{granted}
	code, _ = s.Start(context.Background(), github.SignInInput{Account: work.ID, Name: "work", Login: "saburo"})
	r, _ := s.Poll(context.Background(), code.ID)
	want := account.Account{ID: work.ID, Name: "work", Login: "saburo", Method: account.DeviceFlow}
	if r.State != github.SignInDone || r.Account != want {
		t.Fatalf("edit: %+v", r)
	}
	if list, _ := f.accounts.List(context.Background()); list[0] != want {
		t.Errorf("accounts %+v", list)
	}
}

func TestSignInEditMovesTheDeviceToken(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-old"}}
	f.gh.Users["tok-saburo"] = "saburo"
	f.device.Polls = []port.DeviceGrant{granted}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Account: home.ID, Name: home.Name, Login: "saburo"})
	if r, _ := s.Poll(context.Background(), code.ID); r.State != github.SignInDone {
		t.Fatalf("Poll = %+v", r)
	}
	if _, ok := f.secrets.Device["hanako"]; ok {
		t.Error("the token of the old login was left")
	}
}

func TestSignInCancel(t *testing.T) {
	f := newConn(t)
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "x"})
	s.Cancel(code.ID)
	if _, err := s.Poll(context.Background(), code.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Poll = %v", err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/github/`
Expected: FAIL(`undefined: github.SignIn`)

- [ ] **Step 3: Session に流れを足す**

`internal/app/github/session.go` の `Session` に `flows map[string]*flow` を足し、`NewSession` で `flows: map[string]*flow{}` を入れる。末尾に足す:

```go
func (s *Session) addFlow(id string, f *flow) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flows[id] = f
}

func (s *Session) flow(id string) (*flow, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.flows[id]
	return f, ok
}

func (s *Session) endFlow(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.flows, id)
}

// dropExpired は、期限を過ぎた流れを捨てる(閉じたダイアログの流れを残し続けないため)。
func (s *Session) dropExpired(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, f := range s.flows {
		if !now.Before(f.expiresAt) {
			delete(s.flows, id)
		}
	}
}
```

(`import` に `"time"`。)

- [ ] **Step 4: SignIn を書く**

`internal/app/github/sign_in.go`:

```go
package github

import (
	"context"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// noClientIDText は、client_id がないときの「アカウントを追加」の赤字(07「デバイスフロー」)。
const noClientIDText = "デバイスフローを使うには、config.yaml の hosts に github.com の client_id を設定してください。gh CLI のトークン・環境変数・OS のキーチェーンは、設定なしで使えます"

// slowDownStep は、GitHub が slow_down を返したときに延ばす間隔(RFC 8628)。
const slowDownStep = 5 * time.Second

// SignIn は、デバイスフローのサインイン(07「デバイスフロー」)。登録・編集・再認証で使う。
// 流れはサーバーのメモリに持ち、画面は ID で問い合わせる(この小計画の決定 D2-8・D2-9)。
type SignIn struct {
	Session  *Session
	Accounts port.Accounts
	Secrets  port.Secrets
	GitHub   port.GitHub
	Device   port.DeviceFlow
	Hosts    port.Hosts
	IDs      port.IDs
	Clock    port.Clock
}

// SignInInput の Account が空なら登録、あれば、そのアカウントの編集か再認証(再認証はいまの値を渡す)。
type SignInInput struct {
	Account account.ID
	Name    string
	Login   string
}

// SignInCode は、画面に出すコード。device_code は持たない。
type SignInCode struct {
	ID              string
	UserCode        string
	VerificationURI string
	ExpiresIn       time.Duration
	Interval        time.Duration
}

type SignInState string

const (
	SignInWaiting  SignInState = "waiting"
	SignInExpired  SignInState = "expired"
	SignInDenied   SignInState = "denied"
	SignInMismatch SignInState = "mismatch" // ログイン名の欄の下に Message を出す
	SignInFailed   SignInState = "failed"   // 「アカウントを追加」に Message を出す
	SignInDone     SignInState = "done"
)

type SignInResult struct {
	State    SignInState
	Message  string
	Interval time.Duration // 次に問い合わせるまでの間隔
	Account  account.Account
}

// flow はサインインの途中の流れ。
type flow struct {
	target     account.Account // 登録・保存する値(登録では ID が空)
	previous   account.Account // 編集・再認証の前の値
	clientID   string
	deviceCode string
	expiresAt  time.Time
	interval   time.Duration
}

func (u SignIn) Start(ctx context.Context, in SignInInput) (SignInCode, error) {
	now := u.Clock.Now()
	u.Session.dropExpired(now)
	f, err := u.prepare(ctx, in)
	if err != nil {
		return SignInCode{}, ghcall.Refuse(err, now)
	}
	dc, err := u.Device.StartDevice(ctx, f.clientID)
	if err != nil {
		return SignInCode{}, ghcall.Refuse(err, now)
	}
	f.deviceCode, f.expiresAt, f.interval = dc.DeviceCode, now.Add(dc.ExpiresIn), dc.Interval
	id := string(u.IDs.New())
	u.Session.addFlow(id, f)
	return SignInCode{ID: id, UserCode: dc.UserCode, VerificationURI: dc.VerificationURI, ExpiresIn: dc.ExpiresIn, Interval: dc.Interval}, nil
}

// prepare は、入力を 07「登録」の表の順に確かめ、client_id を読む。
func (u SignIn) prepare(ctx context.Context, in SignInInput) (*flow, error) {
	existing, err := u.Accounts.List(ctx)
	if err != nil {
		return nil, err
	}
	f := &flow{}
	if in.Account != "" {
		if f.previous, err = find(ctx, u.Accounts, in.Account); err != nil {
			return nil, err
		}
	}
	if f.target, err = account.Check(account.Account{Name: in.Name, Login: in.Login, Method: account.DeviceFlow}, existing, in.Account); err != nil {
		return nil, err
	}
	f.target.ID = in.Account
	if f.clientID, err = u.Hosts.ClientID(ctx); err != nil {
		return nil, err
	}
	if f.clientID == "" {
		return nil, refusal.New(noClientIDText)
	}
	return f, nil
}

// Poll は、許可されたかを GitHub に 1 回だけ聞く。GitHub に届かないときは待ち続ける。
func (u SignIn) Poll(ctx context.Context, id string) (SignInResult, error) {
	f, ok := u.Session.flow(id)
	if !ok {
		return SignInResult{}, port.ErrNotFound
	}
	if !u.Clock.Now().Before(f.expiresAt) {
		return u.end(id, SignInResult{State: SignInExpired}), nil
	}
	g, err := u.Device.PollDevice(ctx, f.clientID, f.deviceCode)
	if err != nil {
		return SignInResult{State: SignInWaiting, Interval: u.Session.intervalOf(f)}, nil
	}
	switch g.Status {
	case port.DeviceSlowDown:
		return SignInResult{State: SignInWaiting, Interval: u.Session.slowDown(f, slowDownStep)}, nil
	case port.DeviceExpired:
		return u.end(id, SignInResult{State: SignInExpired}), nil
	case port.DeviceDenied:
		return u.end(id, SignInResult{State: SignInDenied}), nil
	case port.DeviceGranted:
		return u.finish(ctx, id, f, g.Token)
	}
	return SignInResult{State: SignInWaiting, Interval: u.Session.intervalOf(f)}, nil
}

// finish は、許可されたトークンの持ち主を確かめ、キーチェーンに置いて、アカウントを登録・保存する。
func (u SignIn) finish(ctx context.Context, id string, f *flow, tok account.DeviceToken) (SignInResult, error) {
	login, err := u.GitHub.Viewer(ctx, tok.Access)
	if err != nil {
		// 許可は済んでいるが確かめられない。流れを捨て、理由を「アカウントを追加」に出す。
		return u.end(id, SignInResult{State: SignInFailed, Message: ghcall.Refuse(err, u.Clock.Now()).Error()}), nil
	}
	if !strings.EqualFold(login, f.target.Login) {
		return u.end(id, SignInResult{State: SignInMismatch, Message: mismatchText(login)}), nil
	}
	if err := u.Secrets.SaveDeviceToken(ctx, f.target.Login, tok); err != nil {
		return u.end(id, SignInResult{State: SignInFailed, Message: ghcall.Refuse(err, u.Clock.Now()).Error()}), nil
	}
	a, err := u.save(ctx, f)
	if err != nil {
		u.Session.endFlow(id)
		return SignInResult{}, err
	}
	u.Session.markOK(a.ID, [32]byte{})
	return u.end(id, SignInResult{State: SignInDone, Account: a}), nil
}

func (u SignIn) save(ctx context.Context, f *flow) (account.Account, error) {
	a := f.target
	if a.ID == "" {
		a.ID = account.ID(u.IDs.New())
		return a, u.Accounts.Add(ctx, a)
	}
	if err := u.Accounts.Update(ctx, a); err != nil {
		return account.Account{}, err
	}
	// デバイスフローのままログイン名を変えたら、古いログイン名の項目は誰も読まないので消す(決定 D2-13)。
	if f.previous.Method == account.DeviceFlow && !strings.EqualFold(f.previous.Login, a.Login) {
		return a, u.Secrets.DeleteDeviceToken(ctx, f.previous.Login)
	}
	return a, nil
}

func (u SignIn) end(id string, r SignInResult) SignInResult {
	u.Session.endFlow(id)
	return r
}

// Cancel は、流れを捨てる(キャンセル・Esc・外側のクリック)。何も登録しない。
func (u SignIn) Cancel(id string) { u.Session.endFlow(id) }
```

`flow.interval` は `slow_down` で延びる。2 つの問い合わせが同時に来ても競合しないよう、読み書きは `Session.mu` の中で行う。`session.go` に足す:

```go
// slowDown は、流れの間隔を延ばして返す。
func (s *Session) slowDown(f *flow, step time.Duration) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	f.interval += step
	return f.interval
}

func (s *Session) intervalOf(f *flow) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return f.interval
}
```

- [ ] **Step 5: 通ることを確かめる**

Run: `go test -race ./internal/app/github/ && make lint`
Expected: PASS

- [ ] **Step 6: コミット**

```bash
git add internal/app/github
git commit -m "feat(app): デバイスフローのサインインで、アカウントを登録・編集・再認証する"
```

---

### Task 8: API と組み立て

**Files:**
- Modify: `internal/adapter/httpapi/github/accounts.go`、`internal/adapter/httpapi/github/accounts_test.go`
- Create: `internal/adapter/httpapi/github/sign_in.go`、`internal/adapter/httpapi/github/sign_in_test.go`
- Modify: `internal/adapter/httpapi/routes.go`、`cmd/hedgehug/wire.go`、`cmd/hedgehug/wire_e2e.go`
- Create: `cmd/hedgehug/nostores.go`(`//go:build !e2e`)
- Regenerate: `api/openapi.yaml`、`web/packages/api/src/generated.ts`

**Interfaces:**
- Consumes: Task 5〜7 の usecase
- Produces(API。誤りは `apierr.From`: 断り → 422 と `detail`、見つからない → 404):
  - `GET /api/accounts`(`list-accounts`): `{accounts: [{id, name, login, method, status: "ok"|"auth_error", reason}]}`
  - `PUT /api/accounts/{id}`(`edit-account`): 本文 `{name, login, method: gh|env|keychain}` → アカウント
  - `DELETE /api/accounts/{id}`(`delete-account`): 204
  - `POST /api/accounts/{id}/reauth`(`reauth-account`): → アカウント
  - `GET /api/accounts/{id}/delete-impact`(`account-delete-impact`): `{lines: [...]}`
  - `GET /api/accounts/{id}/edit-impact?login=`(`account-edit-impact`): `{lines: [...]}`(空なら確認を出さない)
  - `POST /api/sign-ins`(`start-sign-in`): 本文 `{account?, name, login}` → 201 `{id, user_code, verification_uri, expires_in, interval}`(秒)
  - `GET /api/sign-ins/{id}`(`poll-sign-in`): `{state, message, interval, account?}`
  - `DELETE /api/sign-ins/{id}`(`cancel-sign-in`): 204
  - `httpapi.Deps` に `DeviceFlow port.DeviceFlow`・`Hosts port.Hosts`・`StoreGitHub port.StoreGitHub`

- [ ] **Step 1: 失敗するテストを書く**

`internal/adapter/httpapi/github/accounts_test.go` の `newAPI` を、Task 8 の `Register` に合わせて次にする:

```go
func newAPI(t *testing.T, gh *apptest.GitHub) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	ghapi.Register(api, newServices(gh))
	return api
}

func newServices(gh *apptest.GitHub) ghapi.Services {
	accounts := apptest.NewAccounts()
	secrets := &apptest.Secrets{Tokens: map[string]string{"taro": "tok-taro"}}
	clock := apptest.NewClock(time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC))
	stores := &apptest.StoreGitHub{Login: "taro", Stores: []port.StoreGitHubInfo{{ID: "S1", Name: "チーム", Remote: true}}}
	session := ghapp.NewSession()
	device := apptest.NewDeviceFlow()
	hosts := apptest.Hosts{ID: "Iv23abc"}
	ids := apptest.NewIDs()
	return ghapi.Services{
		List:   ghapp.ListAccounts{Accounts: accounts, Session: session},
		Add:    ghapp.AddAccount{Accounts: accounts, Secrets: secrets, GitHub: gh, IDs: ids, Clock: clock},
		Edit:   ghapp.EditAccount{Session: session, Accounts: accounts, Secrets: secrets, GitHub: gh, Clock: clock},
		Delete: ghapp.DeleteAccount{Session: session, Accounts: accounts, Secrets: secrets, Stores: stores},
		Reauth: ghapp.Reauth{Session: session, Accounts: accounts, Secrets: secrets, GitHub: gh, Clock: clock},
		Impact: ghapp.Impact{Accounts: accounts, Stores: stores},
		SignIn: ghapp.SignIn{Session: session, Accounts: accounts, Secrets: secrets, GitHub: gh, Device: device, Hosts: hosts, IDs: ids, Clock: clock},
	}
}
```

(`import` に `"github.com/kukv/hedgehug/internal/app/port"`。)

同じファイルに足す:

```go
func TestEditDeleteReauthImpact(t *testing.T) {
	gh := &apptest.GitHub{Users: map[string]string{"tok-taro": "taro"}}
	api := newAPI(t, gh)
	res := api.Post("/api/accounts", map[string]string{"name": "work", "login": "taro", "method": "gh"})
	var acc struct{ ID string }
	_ = json.Unmarshal(res.Body.Bytes(), &acc)

	res = api.Put("/api/accounts/"+acc.ID, map[string]string{"name": "仕事", "login": "taro", "method": "keychain"})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"name":"仕事"`) {
		t.Fatalf("PUT: %d %s", res.Code, res.Body)
	}
	res = api.Post("/api/accounts/"+acc.ID+"/reauth", map[string]string{})
	if res.Code != http.StatusOK {
		t.Fatalf("reauth: %d %s", res.Code, res.Body)
	}
	res = api.Get("/api/accounts/" + acc.ID + "/edit-impact?login=jiro")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"lines":[]`) {
		t.Errorf("edit-impact (no store uses it): %d %s", res.Code, res.Body)
	}
	res = api.Get("/api/accounts/" + acc.ID + "/delete-impact")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "「仕事」を削除します。") {
		t.Errorf("delete-impact: %d %s", res.Code, res.Body)
	}
	if res = api.Delete("/api/accounts/" + acc.ID); res.Code != http.StatusNoContent {
		t.Fatalf("DELETE: %d %s", res.Code, res.Body)
	}
	if res = api.Delete("/api/accounts/" + acc.ID); res.Code != http.StatusNotFound {
		t.Errorf("DELETE again: %d", res.Code)
	}
}

func TestListShowsAuthError(t *testing.T) {
	gh := &apptest.GitHub{Users: map[string]string{"tok-taro": "taro"}}
	api := newAPI(t, gh)
	res := api.Post("/api/accounts", map[string]string{"name": "work", "login": "taro", "method": "gh"})
	var acc struct{ ID string }
	_ = json.Unmarshal(res.Body.Bytes(), &acc)
	gh.Err = &ghcall.Failure{Kind: ghcall.TokenRejected}
	res = api.Post("/api/accounts/"+acc.ID+"/reauth", map[string]string{})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "トークンが GitHub に受け付けられませんでした") {
		t.Errorf("reauth: %d %s", res.Code, res.Body)
	}
	res = api.Get("/api/accounts")
	if !strings.Contains(res.Body.String(), `"status":"ok"`) {
		t.Errorf("a failed reauth changed the state: %s", res.Body)
	}
}
```

`internal/adapter/httpapi/github/sign_in_test.go`:

```go
package github_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	ghapi "github.com/kukv/hedgehug/internal/adapter/httpapi/github"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

func TestSignInAPI(t *testing.T) {
	gh := &apptest.GitHub{Users: map[string]string{"tok-saburo": "saburo"}}
	svc := newServices(gh)
	device := svc.SignIn.Device.(*apptest.DeviceFlow)
	device.Polls = []port.DeviceGrant{{Status: port.DevicePending}, {Status: port.DeviceGranted, Token: account.DeviceToken{Access: "tok-saburo"}}}
	_, api := humatest.New(t)
	ghapi.Register(api, svc)

	res := api.Post("/api/sign-ins", map[string]string{"name": "副業", "login": "saburo"})
	if res.Code != http.StatusCreated || strings.Contains(res.Body.String(), "device-1") {
		t.Fatalf("POST: %d %s (the device code must not be shown)", res.Code, res.Body)
	}
	var code struct {
		ID        string `json:"id"`
		UserCode  string `json:"user_code"`
		ExpiresIn int    `json:"expires_in"`
		Interval  int    `json:"interval"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &code)
	if code.UserCode != "WDJB-MJHT" || code.ExpiresIn != 900 || code.Interval != 5 {
		t.Errorf("code %+v", code)
	}
	if res = api.Get("/api/sign-ins/" + code.ID); !strings.Contains(res.Body.String(), `"state":"waiting"`) {
		t.Errorf("1st GET: %s", res.Body)
	}
	res = api.Get("/api/sign-ins/" + code.ID)
	if !strings.Contains(res.Body.String(), `"state":"done"`) || !strings.Contains(res.Body.String(), `"login":"saburo"`) ||
		strings.Contains(res.Body.String(), "tok-saburo") {
		t.Errorf("2nd GET: %s", res.Body)
	}
	if res = api.Get("/api/sign-ins/" + code.ID); res.Code != http.StatusNotFound {
		t.Errorf("after done: %d", res.Code)
	}
	res = api.Post("/api/sign-ins", map[string]string{"name": "x", "login": "x"})
	var other struct{ ID string }
	_ = json.Unmarshal(res.Body.Bytes(), &other)
	if res = api.Delete("/api/sign-ins/" + other.ID); res.Code != http.StatusNoContent {
		t.Errorf("DELETE: %d", res.Code)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/...`
Expected: FAIL(`undefined: ghapi.Services`)

- [ ] **Step 3: API を書く**

`internal/adapter/httpapi/github/accounts.go` を次にする:

```go
// Package github は、GitHub 連携の API(仕様 07)を持つ。
package github

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// Services は、GitHub 連携の API が使う usecase。
type Services struct {
	List   ghapp.ListAccounts
	Add    ghapp.AddAccount
	Edit   ghapp.EditAccount
	Delete ghapp.DeleteAccount
	Reauth ghapp.Reauth
	Impact ghapp.Impact
	SignIn ghapp.SignIn
}

type accountBody struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Login  string `json:"login"`
	Method string `json:"method" enum:"device,gh,env,keychain"`
}

func toBody(a account.Account) accountBody {
	return accountBody{ID: string(a.ID), Name: a.Name, Login: a.Login, Method: string(a.Method)}
}

// accountRow は一覧の 1 行(07「状態」)。reason は「認証エラー」の下の赤字(なければ空)。
type accountRow struct {
	accountBody
	Status string `json:"status" enum:"ok,auth_error"`
	Reason string `json:"reason"`
}

type listOutput struct {
	Body struct {
		Accounts []accountRow `json:"accounts"`
	}
}

type valuesBody struct {
	Name   string `json:"name"`
	Login  string `json:"login"`
	Method string `json:"method" enum:"gh,env,keychain" doc:"デバイスフローはサインインの API で登録・保存する"`
}

type addInput struct {
	Body valuesBody
}

type accountOutput struct {
	Body accountBody
}

type idInput struct {
	ID string `path:"id"`
}

type editInput struct {
	ID   string `path:"id"`
	Body valuesBody
}

type editImpactInput struct {
	ID    string `path:"id"`
	Login string `query:"login"`
}

type impactOutput struct {
	Body struct {
		Lines []string `json:"lines"`
	}
}

// Register は、アカウントとサインインの API を登録する。
func Register(api huma.API, s Services) {
	registerAccounts(api, s)
	registerManage(api, s)
	registerSignIn(api, s.SignIn)
}

func registerAccounts(api huma.API, s Services) {
	huma.Register(api, huma.Operation{
		OperationID: "list-accounts", Method: http.MethodGet, Path: "/api/accounts",
		Summary: "登録したアカウントを、状態を付けて返す",
	}, func(ctx context.Context, _ *struct{}) (*listOutput, error) {
		rows, err := s.List.Do(ctx)
		if err != nil {
			return nil, apierr.From(err)
		}
		out := &listOutput{}
		out.Body.Accounts = make([]accountRow, 0, len(rows))
		for _, r := range rows {
			status := "ok"
			if r.AuthError {
				status = "auth_error"
			}
			out.Body.Accounts = append(out.Body.Accounts, accountRow{accountBody: toBody(r.Account), Status: status, Reason: r.Reason})
		}
		return out, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "add-account", Method: http.MethodPost, Path: "/api/accounts",
		Summary: "GitHub で確かめてからアカウントを登録する", DefaultStatus: http.StatusCreated,
		Errors: []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *addInput) (*accountOutput, error) {
		a, err := s.Add.Do(ctx, ghapp.AddAccountInput{Name: in.Body.Name, Login: in.Body.Login, Method: account.Method(in.Body.Method)})
		if err != nil {
			return nil, apierr.From(err)
		}
		return &accountOutput{Body: toBody(a)}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "edit-account", Method: http.MethodPut, Path: "/api/accounts/{id}",
		Summary: "新しい値を GitHub で確かめてから、アカウントの値を入れ替える",
		Errors:  []int{http.StatusNotFound, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *editInput) (*accountOutput, error) {
		a, err := s.Edit.Do(ctx, ghapp.EditAccountInput{
			ID: account.ID(in.ID), Name: in.Body.Name, Login: in.Body.Login, Method: account.Method(in.Body.Method),
		})
		if err != nil {
			return nil, apierr.From(err)
		}
		return &accountOutput{Body: toBody(a)}, nil
	})
}

func registerManage(api huma.API, s Services) {
	huma.Register(api, huma.Operation{
		OperationID: "delete-account", Method: http.MethodDelete, Path: "/api/accounts/{id}",
		Summary: "アカウントを削除し、割り当てた Store を「なし」にする", DefaultStatus: http.StatusNoContent,
		Errors: []int{http.StatusNotFound},
	}, func(ctx context.Context, in *idInput) (*struct{}, error) {
		return nil, apierr.From(s.Delete.Do(ctx, account.ID(in.ID)))
	})
	huma.Register(api, huma.Operation{
		OperationID: "reauth-account", Method: http.MethodPost, Path: "/api/accounts/{id}/reauth",
		Summary: "gh CLI・環境変数・OS のキーチェーンのアカウントの認証の確認をやり直す",
		Errors:  []int{http.StatusNotFound, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *idInput) (*accountOutput, error) {
		a, err := s.Reauth.Do(ctx, account.ID(in.ID))
		if err != nil {
			return nil, apierr.From(err)
		}
		return &accountOutput{Body: toBody(a)}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "account-delete-impact", Method: http.MethodGet, Path: "/api/accounts/{id}/delete-impact",
		Summary: "「アカウントを削除」の確認の本文の行", Errors: []int{http.StatusNotFound},
	}, func(ctx context.Context, in *idInput) (*impactOutput, error) {
		lines, err := s.Impact.Delete(ctx, account.ID(in.ID))
		return impact(lines, err)
	})
	huma.Register(api, huma.Operation{
		OperationID: "account-edit-impact", Method: http.MethodGet, Path: "/api/accounts/{id}/edit-impact",
		Summary: "ログイン名を変えるときの「アカウントを変更」の確認の本文の行(空なら確認を出さない)",
		Errors:  []int{http.StatusNotFound},
	}, func(ctx context.Context, in *editImpactInput) (*impactOutput, error) {
		lines, err := s.Impact.Edit(ctx, account.ID(in.ID), in.Login)
		return impact(lines, err)
	})
}

func impact(lines []string, err error) (*impactOutput, error) {
	if err != nil {
		return nil, apierr.From(err)
	}
	out := &impactOutput{}
	out.Body.Lines = append([]string{}, lines...)
	return out, nil
}
```

`ErrDeviceFlow`(編集・再認証でデバイスフローを渡した)は、画面がサインインの API を使えば起きない呼び方の誤りなので、`apierr.From` のとおり 500 にする(A1 の `ErrParentElsewhere` と同じ扱い)。

`internal/adapter/httpapi/github/sign_in.go`:

```go
package github

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/domain/account"
)

type signInInput struct {
	Body struct {
		Account string `json:"account,omitempty" doc:"編集・再認証するアカウントの ID。登録では空"`
		Name    string `json:"name"`
		Login   string `json:"login"`
	}
}

type signInCode struct {
	ID              string `json:"id"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in" doc:"秒"`
	Interval        int    `json:"interval" doc:"秒"`
}

type signInCodeOutput struct {
	Body signInCode
}

type signInIDInput struct {
	ID string `path:"id"`
}

type signInResult struct {
	State    string       `json:"state" enum:"waiting,expired,denied,mismatch,failed,done"`
	Message  string       `json:"message"`
	Interval int          `json:"interval" doc:"次に問い合わせるまでの秒"`
	Account  *accountBody `json:"account,omitempty"`
}

type signInResultOutput struct {
	Body signInResult
}

func registerSignIn(api huma.API, s ghapp.SignIn) {
	huma.Register(api, huma.Operation{
		OperationID: "start-sign-in", Method: http.MethodPost, Path: "/api/sign-ins",
		Summary: "デバイスフローのサインインを始め、GitHub で入力するコードを返す", DefaultStatus: http.StatusCreated,
		Errors: []int{http.StatusNotFound, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *signInInput) (*signInCodeOutput, error) {
		c, err := s.Start(ctx, ghapp.SignInInput{Account: account.ID(in.Body.Account), Name: in.Body.Name, Login: in.Body.Login})
		if err != nil {
			return nil, apierr.From(err)
		}
		return &signInCodeOutput{Body: signInCode{
			ID: c.ID, UserCode: c.UserCode, VerificationURI: c.VerificationURI,
			ExpiresIn: int(c.ExpiresIn.Seconds()), Interval: int(c.Interval.Seconds()),
		}}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "poll-sign-in", Method: http.MethodGet, Path: "/api/sign-ins/{id}",
		Summary: "サインインが許可されたかを GitHub に 1 回聞く", Errors: []int{http.StatusNotFound},
	}, func(ctx context.Context, in *signInIDInput) (*signInResultOutput, error) {
		r, err := s.Poll(ctx, in.ID)
		if err != nil {
			return nil, apierr.From(err)
		}
		out := &signInResultOutput{Body: signInResult{State: string(r.State), Message: r.Message, Interval: int(r.Interval.Seconds())}}
		if r.State == ghapp.SignInDone {
			b := toBody(r.Account)
			out.Body.Account = &b
		}
		return out, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "cancel-sign-in", Method: http.MethodDelete, Path: "/api/sign-ins/{id}",
		Summary: "サインインをやめる(何も登録しない)", DefaultStatus: http.StatusNoContent,
	}, func(_ context.Context, in *signInIDInput) (*struct{}, error) {
		s.Cancel(in.ID)
		return nil, nil
	})
}
```

- [ ] **Step 4: 組み立てを直す**

`internal/adapter/httpapi/routes.go` の `Deps` のトラック D の欄を次にし、`ghapi.Register` の呼び出しを替える:

```go
	// トラック D(GitHub)
	Accounts    port.Accounts
	Secrets     port.Secrets
	GitHub      port.GitHub
	DeviceFlow  port.DeviceFlow
	Hosts       port.Hosts
	StoreGitHub port.StoreGitHub
	Clock       port.Clock
```

```go
	ghapi.Register(api, githubServices(deps))
	return api
}

// githubServices は、GitHub 連携の usecase を組む。Session はサーバーに 1 つ(アプリを開いているあいだの状態)。
func githubServices(d Deps) ghapi.Services {
	s := ghapp.NewSession()
	return ghapi.Services{
		List:   ghapp.ListAccounts{Accounts: d.Accounts, Session: s},
		Add:    ghapp.AddAccount{Accounts: d.Accounts, Secrets: d.Secrets, GitHub: d.GitHub, IDs: d.IDs, Clock: d.Clock},
		Edit:   ghapp.EditAccount{Session: s, Accounts: d.Accounts, Secrets: d.Secrets, GitHub: d.GitHub, Clock: d.Clock},
		Delete: ghapp.DeleteAccount{Session: s, Accounts: d.Accounts, Secrets: d.Secrets, Stores: d.StoreGitHub},
		Reauth: ghapp.Reauth{Session: s, Accounts: d.Accounts, Secrets: d.Secrets, GitHub: d.GitHub, Clock: d.Clock},
		Impact: ghapp.Impact{Accounts: d.Accounts, Stores: d.StoreGitHub},
		SignIn: ghapp.SignIn{
			Session: s, Accounts: d.Accounts, Secrets: d.Secrets, GitHub: d.GitHub, Device: d.DeviceFlow,
			Hosts: d.Hosts, IDs: d.IDs, Clock: d.Clock,
		},
	}
}
```

D3 で `Connector` を使う API を足すときは、同じ `s` から `ghapp.Connector{Session: s, ...}` を作る(Session を 2 つ作らない)。

`cmd/hedgehug/nostores.go`:

```go
//go:build !e2e

package main

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// noStoreGitHub は、Store の一覧(トラック G)が入るまでの仮。Store はなく、マシンのログイン名は未設定。
type noStoreGitHub struct{}

func (noStoreGitHub) List(context.Context) ([]port.StoreGitHubInfo, error) { return nil, nil }
func (noStoreGitHub) Unassign(context.Context, account.ID) error          { return nil }
func (noStoreGitHub) MachineLogin(context.Context) (string, error)        { return "", nil }
```

`cmd/hedgehug/wire.go` の `wire` を次にする:

```go
func wire(context.Context) (httpapi.Deps, error) {
	stores := gitstore.New(sysclock.Clock{}, sysclock.IDs{})
	cfg, err := configPath()
	if err != nil {
		return httpapi.Deps{}, err
	}
	file := machineconfig.Open(cfg)
	gh := github.New(nil) // 時計のずれはトラック C が受け取る(トラック D の計画の決定 3)
	return httpapi.Deps{
		Stores:      stores,
		IDs:         sysclock.IDs{},
		Identity:    noIdentity{},
		Accounts:    file.Accounts(),
		Secrets:     secret.New(),
		GitHub:      gh,
		DeviceFlow:  gh,
		Hosts:       file.Hosts(),
		StoreGitHub: noStoreGitHub{},
		Clock:       sysclock.Clock{},
	}, nil
}
```

`cmd/hedgehug/wire_e2e.go` の `return httpapi.Deps{...}` の GitHub の欄を次にする(D2 画面の e2e が偽物を使う。サンプルの Store「チーム」は remote を持ち、アカウントは割り当てない):

```go
		Accounts:    apptest.NewAccounts(),
		Secrets:     &apptest.Secrets{Tokens: map[string]string{"taro": "e2e-gh-token"}},
		GitHub:      &apptest.GitHub{Users: map[string]string{"e2e-gh-token": "taro"}},
		DeviceFlow:  apptest.NewDeviceFlow(),
		Hosts:       apptest.Hosts{ID: "e2e-client"},
		StoreGitHub: &apptest.StoreGitHub{Login: "taro", Stores: []port.StoreGitHubInfo{{ID: e2eStore, Name: "チーム", Remote: true}}},
		Clock:       apptest.NewClock(time.Now()),
```

(`import` に `"github.com/kukv/hedgehug/internal/app/port"`。デバイスフローの許可・拒否・期限切れを e2e で起こす方法は、D2 画面の計画で決める。)

- [ ] **Step 5: 生成物を作り直し、全体を確かめる**

Run: `make generate && (cd web && pnpm gen) && make check`
Expected: PASS。`api/openapi.yaml` に 9 つの operation(`list-accounts`・`add-account`・`edit-account`・`delete-account`・`reauth-account`・`account-delete-impact`・`account-edit-impact`・`start-sign-in`・`poll-sign-in`・`cancel-sign-in`)がある。`go build -tags e2e ./cmd/hedgehug` も通る。

- [ ] **Step 6: コミット**

```bash
git add internal/adapter/httpapi cmd/hedgehug api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat: アカウントの編集・削除・再認証とデバイスフローのサインインの API を足す"
```

---

## D2 のあと

- **D2 画面** の計画を、トラック A の A2 のルーター・サイドバー・トースト・断りの読み取りが develop に入ってから書く。範囲: 設定画面の枠と「アカウントとメンバー」タブ(枠がなければ共有の小さな PR)、「GitHub アカウント」の一覧(3 つの状態のうち利用可能・認証エラー)、「アカウントを追加」「アカウントを編集」「GitHub にサインイン」のダイアログ、削除と変更の確認、行の中の赤字と「GitHub に問い合わせています」、「GitHub を使わない場合」の無効の部品、Playwright(3 幅)。e2e でデバイスフローの許可・拒否・期限切れを起こす方法もそこで決める。
- D3 は `Connector.TokenFor` でトークンを得て、401 を `Connector.Rejected` で知らせる。
