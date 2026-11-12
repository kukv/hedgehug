# hedgehug 小計画 D2 画面: 設定の「アカウントとメンバー」の GitHub アカウント

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 設定画面の枠と「アカウントとメンバー」タブの「GitHub アカウント」を作り、D1・D2 のサーバー(API)で、アカウントの登録・編集・削除・再認証とデバイスフローのサインインを画面から行えるようにする。

**Architecture:** A2 と同じ形にする。操作の流れ(一覧・行の赤字・ダイアログ・サインインの問い合わせ)は `core/features/accounts` に置き、`api` が API を core の口(`AccountsPort`)に合わせ、`app` の Svelte は core の状態を出して events を返すだけにする。設定画面はアドレス `/settings/{タブ}` を持ち、サイドバーのアプリ名の横の歯車から開く。e2e は、e2e のビルドにだけデバイスフローの答えを決める入口を足して、サーバーまで通しで確かめる。

**Tech Stack:** 土台と同じ(Go 1.27.3、huma v2、Svelte 5、orval、Vitest、Testing Library、Playwright)。新しい依存は足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/07-github.md`「アカウント」の全節(GitHub App 未導入の帯・「再確認」・自動の確かめ直しを除く)、「GitHub との通信に失敗したとき」の冒頭(問い合わせ中の表示)
- 08「設定画面の構成」、04「サイドバー」の 1(歯車)、01「画面のアドレス」(設定の行)
- `docs/superpowers/plans/2026-11-13-hedgehug-track-d2.md`(サーバーの決定 D2-1〜D2-15 と「D2 のあと」)、PR #158 本文の申し送り
- `docs/superpowers/plans/2026-11-13-hedgehug-track-a2.md`(画面の決定 1〜29。とくに 13・14・15・24・26・29)
- 見た目: モック版 19(https://claude.ai/artifact/Qw2jqgdbYtbcA4VTMHGCVA)の `settings.js`(`PageSettings`・`SettingsAccounts`・`M.account`)。振る舞いは仕様に従い、モックの動きを写さない。

**ブランチ:** `origin/develop` から `feat/track-d2-screen`。PR の向き先は `develop`。計画書とすべてのタスクを 1 本の PR にまとめる。

## Global Constraints

A2 の計画の Global Constraints をすべて引き継ぐ。再掲する:

- 画面の文言・トースト・断りの文言は、仕様の文字どおりに書く。仕様にない文言は足さない。足すときは「この計画で決めたこと」に載せる。
- 新しい依存(npm・Go)は足さない。
- `api/openapi.yaml` と `web/packages/api/src/generated.ts` は手で直さない。API を変えたタスクで `make generate` と `cd web && pnpm gen` を実行し、同じコミットに入れる。
- 大きさの上限(Svelte 200 行・TS 300 行・Go ファイル 300 行・関数 60 行・認知的複雑度 15)を超えたら、上限を上げずに分ける。
- ui の部品は渡された値を出して events を返すだけ。値の計算・検査・操作の流れは core に置く(設計書 6 節)。
- 計画やテストに、トークンらしい値(`ghp_`・`gho_`・`ghu_`・`ghs_`・`ghr_`・`github_pat_` で始まる文字列)を書かない。テストのトークンは `tok-taro` のような値にする。
- 本物の組み立てだけで使う型と関数は `//go:build !e2e`、e2e のビルドだけで使うものは `//go:build e2e` のファイルに置く。e2e のビルドは CI で `go vet -tags e2e` と `golangci-lint run --build-tags e2e` を通す。
- force push はしない。
- 各タスクは TDD で進める(失敗するテストを書く → 落ちるのを見る → 実装 → 通す → コミット)。コミットの前に、そのタスクで触ったパッケージのテスト・lint・型検査を通す。

---

## この計画で決めたこと(既存の規則の当てはめ)

| # | 決めたこと | 根拠 |
|---|---|---|
| S1 | 範囲は、設定画面の枠と「アカウントとメンバー」タブの「GitHub アカウント」の区画だけ。作らないもの: 「メンバー」の区画(08。メンバーの持ち場)、ほかの 4 つのタブ(一般・Store・削除済み・診断と修復。G ほか)、GitHub App 未導入の状態・帯・「再確認」・自動の確かめ直し(D2-4 で D4)、「GitHub を使わない場合」の無効の部品(下の S2) | A2 の決定 15・24(まだない画面・行き先のない項目を出さない) |
| S2 | 「GitHub を使わない場合」の無効の入口(「GitHub のアカウントを登録すると使えます」)は、入口を作るトラックが作る(リンクの追加は D3、コラボレーターの取り込み・Store の検索は D4、同期は C2 画面)。D2 の計画の「D2 のあと」に書いた範囲から外す | 入口になる部品がまだ 1 つもない。置き場のない部品は確かめられない(A2 の決定 24 と同じ考え) |
| S3 | 設定のアドレスは `/settings/{タブ}`。タブは、いまは `accounts`(アカウントとメンバー)だけ。`/settings` は `/settings/accounts` と同じ画面にする。ほかの綴りはホーム。タブの切り替えは履歴に積む | 01「画面のアドレス」(設定はタブを区別する。タブの切り替えは積む)。A2 の決定 4 の綴りの形 |
| S4 | 設定画面は、見出し「設定」、補足「このマシンの設定と、Store の管理」、タブ「アカウントとメンバー」だけを出す。ui の `PageHead` に補足(`note`)を足す | 08「設定画面の構成」。S1 |
| S5 | 歯車は、サイドバーのアプリ名の右に、リンク(`aria-label="設定"`)として置く。ほかのサイドバーの項目と同じく、Ctrl・Cmd・Shift+クリックと中クリックで新しいタブに開く。行き先は `/settings/accounts` | 04「サイドバー」1。01「新しいタブで開く」(サイドバーの項目) |
| S6 | `/api/accounts/{id}/reauth` と `PUT /api/accounts/{id}` が `ErrDeviceFlow` を返したら 409 にする(いまは 500)。画面はデバイスフローのアカウントの再認証と、デバイスフローへの編集を、`/api/sign-ins`(本文の `account` に ID)で行い、この 2 つを呼ばない | #158 の申し送り。誤った呼び出しはサーバーの失敗(500)でなく、呼び方の誤り(409)。仕様の文言はないので 422 にしない |
| S7 | 編集でログイン名を変えたときは、「保存」を押したら先に `edit-impact` を読み、行があれば「アカウントを変更」の確認を出す。「変更」を押してから、認証の確認(PUT かサインイン)をする。行がなければ確認を出さない | 07「編集」(保存の前に確認)。PUT は確かめと保存を 1 回で行うので、確認はその前に置く。モックは確かめたあとに確認を出すが、モックの動きは写さない |
| S8 | サインインの問い合わせ(`GET /api/sign-ins/{id}`)は core の `signIn` が行う。同時に 1 つだけ。間隔は、サーバーが返した値(コードの `interval`、答えの `interval`)を毎回使う。コードの期限(`expires_in`)が来たら、間隔を待たずに 1 回問い合わせる(問い合わせ中なら、答えのあとすぐ)。キャッシュは使わない(`cache: "no-store"`)。前もって読まない | 07「デバイスフロー」(期限が切れたら、すぐに)。#158 の申し送り(副作用があるので prefetch・キャッシュしない) |
| S9 | 問い合わせの失敗: サーバーに届かない(fetch の失敗・500)ときは、同じ間隔で待ち続ける。404(流れがない)は期限切れと同じに出す。422(断り)は「失敗」と同じに扱い、文言を出す | 07「待っているあいだに通信が切れたときは、つながるまで待ち続ける」。#158 の Q1・Q4 の答えがどちらでも、画面の出し方が変わらないようにする |
| S10 | 「GitHub にサインイン」を閉じる方法(「キャンセル」・Esc・外のクリック・画面の移動)は、どれもサーバーの流れを捨てる(`DELETE /api/sign-ins/{id}`)。閉じたあとに届いた答えが「済んだ」なら、トーストは出さずに一覧だけ読み直す | 07「ダイアログを閉じる」。サーバーで保存が先に済んだときに、一覧が古いままにならないようにする |
| S11 | サインインが済んだときのトーストは、追加なら「@{ログイン名} としてサインインしました」、編集なら「アカウントを変更しました」、再認証なら「{表示名}: @{ログイン名} として再認証しました」 | 07「デバイスフロー」「編集」「再認証」 |
| S12 | 追加・編集のサインインが「ログイン名が違う」で終わったら、「GitHub にサインイン」を閉じて元のダイアログ(認証方法はデバイスフロー、入力はそのまま)に戻り、ログイン名の欄の下に文言を赤字で出す。「失敗」で終わったら、同じく戻ってダイアログに赤字。再認証のサインインの失敗は、「GitHub にサインイン」の中に赤字で出し、「再試行」を置く。「再試行」でコードを取れなかったときは、追加・編集は元のダイアログに戻して赤字、再認証は「GitHub にサインイン」の中に赤字 | 07「デバイスフロー」「状態」(デバイスフローの再認証の失敗は、開いたダイアログの中) |
| S13 | デバイスフローの行の「再認証」でコードを取れなかったときは、その行の中(状態の下)に赤字で出す | 07「状態」(再認証の通信の失敗は行の中) |
| S14 | 行の赤字は、その行の次の操作(「再認証」「編集」「削除」を押す)、設定画面を開き直す、成功のどれかで消す。問い合わせ中の行は「再認証」を無効にし、「GitHub に問い合わせています」を出す | 07「状態」。タブは 1 つなので、「別のタブへ移る」は画面を離れることにあたる |
| S15 | 「アカウントを追加」「アカウントを編集」で GitHub に問い合わせているあいだは、「追加」「保存」を無効にし、「GitHub に問い合わせています」を出す。断りは、ダイアログの赤字に出す(A2 の決定 13・14 と同じく、検査はサーバーに任せる) | 07「GitHub との通信に失敗したとき」の冒頭 |
| S16 | 「アカウントを変更」の確認を出しているあいだは、「アカウントを編集」を隠す(Esc で 2 つが同時に閉じないように)。「キャンセル」で編集に戻る | ui の `Dialog` は window の Esc を聞くので、重ねると両方が閉じる |
| S17 | コードの「コピー」の結果は、詳細パネルの ID のコピーと同じ文言(`copiedMessage`・`copyFailedMessage`)をトーストで出す。例: 「コピーしました: 0000-0001」 | 07 にコードのコピーの結果の文言がない。A2 の詳細パネルの文言を使う |
| S18 | アカウントが 0 件のときは、一覧に何も出さない | A2 の決定 26(仕様に 0 件の文言がない) |
| S19 | ダイアログの見出しは「アカウントを追加」「アカウントを編集」「GitHub にサインイン」「アカウントを変更」「アカウントを削除」。足元は、追加・編集が「キャンセル」と「追加」/「保存」、確認が「キャンセル」と「変更」/「削除」(警告色)、サインインが「キャンセル」(終わったら「再試行」も)。表示名の欄の例は「work」 | 07 の各節。区画の見出し「GitHub アカウント」とボタン「追加」は 07「登録」 |
| S20 | 断り(`Refused`)以外の失敗(通信の失敗・500)は、画面に出さない(サインインの問い合わせを除く。S9) | A2 の決定 29 |
| S21 | e2e のビルドには、デバイスフローの答えをテストが決める入口 `POST /e2e/device?code={user_code}&result=grant|deny|expire&login={ログイン名}` を足す(API と同じトークンが要る)。コードは始めるたびに `0000-0001` から順に振り、間隔は 1 秒。`grant` は、そのログイン名のトークンを GitHub の偽物に足してから許可にする | D2 の「D2 のあと」(e2e で許可・拒否・期限切れを起こす方法を決める)。3 つの幅が 1 つのサーバーを共有するので、答えはコードごとに持つ |
| S22 | e2e のサーバーには、gh CLI のトークンで確かめられるログイン名を `gh-{幅}-{やり直しの回数}-{a か b}`(12 人)と `taro` だけ置く。テストで作るアカウントの表示名とログイン名には、幅とやり直しの回数を入れる | アカウントはマシンに 1 つの一覧で、同じログイン名は断られる(07「登録」)。A2 の決定 21 と同じ考え |
| S23 | 「認証エラー」の行は、e2e では一覧と再認証の応答を Playwright で差し替えて確かめる。e2e のビルドには認証エラーにする経路がない(`Connector` の呼び出し元は D3 から) | 行の表示を通しで確かめる手段がほかにない。状態の流れは core のテストで確かめる |

**確認待ちのまま残るもの(画面には影響しない):** #158 の Q1〜Q4 と「重なった Poll は waiting」。S9 により、答えがどれでも画面の出し方は同じになる。

---

## ファイルの地図

```
Go(Task 1)
internal/adapter/httpapi/github/accounts.go(変更)   ErrDeviceFlow を 409 に
internal/adapter/httpapi/github/accounts_test.go(変更)
internal/app/apptest/github.go(変更)                 GitHub.AddUser
cmd/hedgehug/device_e2e.go                           e2e のデバイスフローと /e2e/device、ログイン名の種
cmd/hedgehug/device_e2e_test.go
cmd/hedgehug/wire_e2e.go(変更)、wire.go(変更)、main.go(変更)   wrap
api/openapi.yaml・web/packages/api/src/generated.ts(生成物)

web(Task 2: 設定画面の枠)
web/packages/core/src/features/router/route.ts・resolve.ts(変更)と各テスト
web/packages/ui/src/layout/PageHead.svelte(変更)    note
web/app/src/shell/Sidebar.svelte(変更)              歯車
web/app/src/shell/Screen.svelte(変更)               設定の画面
web/app/src/screens/settings/SettingsScreen.svelte
web/e2e/settings.spec.ts

web(Task 3: 口)
web/packages/core/src/model/account.ts
web/packages/core/src/port/accounts.ts
web/packages/api/src/accounts.ts・accounts.test.ts
web/packages/core/src/index.ts・web/packages/api/src/index.ts(変更)

web(Task 4〜6: core の流れ)
web/packages/core/src/features/accounts/text.ts       文言と認証方法の表
web/packages/core/src/features/accounts/fixtures.test-util.ts
web/packages/core/src/features/accounts/list.ts       一覧・行の赤字・再認証・削除
web/packages/core/src/features/accounts/form.ts       追加・編集・変更の確認
web/packages/core/src/features/accounts/signIn.ts     GitHub にサインイン
(各 .test.ts)

web(Task 7・8: 画面)
web/packages/ui/src/patterns/ConfirmDialog.svelte・confirm.test.ts
web/app/src/screens/settings/AccountsSection.svelte
web/app/src/screens/settings/AccountItem.svelte
web/app/src/screens/settings/AccountDialog.svelte
web/app/src/screens/settings/SignInDialog.svelte
web/app/src/wire.ts(変更)

web(Task 9: 通し)
web/e2e/accounts.spec.ts
```

## 型と口(タスクをまたいで使う名前)

各タスクの実装者は、自分のタスクしか読まない。ほかのタスクとの境目の名前と型は、ここに書いたものを正とする。

```ts
// core/src/model/account.ts
export type AuthMethod = "device" | "gh" | "env" | "keychain";
export type TokenMethod = Exclude<AuthMethod, "device">;
export interface Account { id: string; name: string; login: string; method: AuthMethod; }
export interface AccountRow extends Account { status: "ok" | "auth_error"; reason: string; }
export interface AccountValues { name: string; login: string; method: AuthMethod; }
export interface TokenValues { name: string; login: string; method: TokenMethod; }
export interface SignInStart { account?: string; name: string; login: string; }
export interface SignInCode { id: string; userCode: string; verificationUri: string; expiresInSec: number; intervalSec: number; }
export type SignInStatus = "waiting" | "expired" | "denied" | "mismatch" | "failed" | "done";
export interface SignInResult { status: SignInStatus; message: string; intervalSec: number; account: Account | null; }
export type SignInPurpose = "add" | "edit" | "reauth";
export interface SignInTarget { purpose: SignInPurpose; original: Account | null; name: string; login: string; }
export interface SignInFailure { error?: string; loginError?: string; }

// core/src/port/accounts.ts
export interface AccountsPort {
  list(): Promise<AccountRow[]>;
  add(values: TokenValues): Promise<Account>;
  edit(id: string, values: TokenValues): Promise<Account>;
  reauth(id: string): Promise<Account>;
  remove(id: string): Promise<void>;
  deleteImpact(id: string): Promise<string[]>;
  editImpact(id: string, login: string): Promise<string[]>;
  startSignIn(input: SignInStart): Promise<SignInCode>;
  pollSignIn(id: string): Promise<SignInResult>;
  cancelSignIn(id: string): Promise<void>;
}

// core/src/features/router/route.ts(追加分)
export type SettingsTab = "accounts";
// Route に { name: "settings"; tab: SettingsTab }、Resolved に { name: "settings"; tab: SettingsTab }

// core/src/features/accounts
createAccountList(deps: { accounts; toasts; reauthDevice(account: Account): Promise<void> }): AccountList
createAccountForm(deps: { accounts; toasts; signIn(target: SignInTarget, code: SignInCode): void; onSaved(): void }): AccountForm
createSignIn(deps: { accounts; toasts; timer: Timer; back(target: SignInTarget, failure: SignInFailure): void; onDone(): void }): SignIn
```

---

### Task 1: サーバーの小さな変更と、e2e のデバイスフロー

**Files:**
- Modify: `internal/adapter/httpapi/github/accounts.go`
- Modify: `internal/adapter/httpapi/github/accounts_test.go`
- Modify: `internal/app/apptest/github.go`
- Create: `cmd/hedgehug/device_e2e.go`、`cmd/hedgehug/device_e2e_test.go`
- Modify: `cmd/hedgehug/wire_e2e.go`、`cmd/hedgehug/wire.go`、`cmd/hedgehug/main.go`
- Regenerate: `api/openapi.yaml`、`web/packages/api/src/generated.ts`

**Interfaces:**
- Produces: 409(`reauth-account`・`edit-account` でデバイスフローのとき)。e2e のビルドの `POST /e2e/device`(決定 S21)。e2e のログイン名 `gh-{phone|tablet|desktop}-{0|1}-{a|b}`(決定 S22)。`apptest.GitHub.AddUser(token, login string)`。

- [ ] **Step 1: 409 の失敗するテストを書く**

`internal/adapter/httpapi/github/accounts_test.go` の末尾に足す:

```go
func TestDeviceFlowAccountIsReauthenticatedBySigningIn(t *testing.T) {
	s := newServices(&apptest.GitHub{})
	if err := s.Reauth.Accounts.Add(t.Context(), account.Account{ID: "A9", Name: "dev", Login: "jiro", Method: account.DeviceFlow}); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	ghapi.Register(api, s)
	if res := api.Post("/api/accounts/A9/reauth", map[string]string{}); res.Code != http.StatusConflict {
		t.Errorf("status %d %s", res.Code, res.Body)
	}
}
```

- [ ] **Step 2: 落ちるのを見る**

Run: `go test ./internal/adapter/httpapi/github/ -run TestDeviceFlowAccountIsReauthenticatedBySigningIn`
Expected: FAIL(status 500)

- [ ] **Step 3: 実装する**

`accounts.go` の import に `"errors"` を足し、ファイルの末尾に足す:

```go
// fromErr は、apierr.From に加えて、デバイスフローのアカウントをこの API で再認証・保存しようとしたときを 409 にする
// (デバイスフローは /api/sign-ins でサインインし直す。D2 画面の計画の決定 S6)。
func fromErr(err error) error {
	if errors.Is(err, ghapp.ErrDeviceFlow) {
		return huma.Error409Conflict("デバイスフローのアカウントは /api/sign-ins でサインインし直す")
	}
	return apierr.From(err)
}
```

`edit-account` と `reauth-account` のハンドラーの `apierr.From(err)` を `fromErr(err)` に替え、両方の `Errors` に `http.StatusConflict` を足す(`[]int{http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity}`)。

- [ ] **Step 4: 通るのを見て、生成物を作り直す**

Run: `go test ./internal/adapter/httpapi/github/ && make generate && cd web && pnpm gen && cd ..`
Expected: PASS。`api/openapi.yaml` と `generated.ts` に 409 が入る。

- [ ] **Step 5: e2e のデバイスフローの失敗するテストを書く**

`cmd/hedgehug/device_e2e_test.go`:

```go
//go:build e2e

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

func TestE2EDeviceAnswersByCode(t *testing.T) {
	gh := &apptest.GitHub{}
	d := newE2EDevice(gh)
	a, _ := d.StartDevice(t.Context(), "c")
	b, _ := d.StartDevice(t.Context(), "c")
	if a.UserCode != "0000-0001" || b.UserCode != "0000-0002" || a.DeviceCode == b.DeviceCode {
		t.Fatalf("codes: %+v %+v", a, b)
	}
	if g, _ := d.PollDevice(t.Context(), "c", a.DeviceCode); g.Status != port.DevicePending {
		t.Errorf("before an answer: %v", g.Status)
	}
	if err := d.answer(a.UserCode, "grant", "taro"); err != nil {
		t.Fatal(err)
	}
	g, _ := d.PollDevice(t.Context(), "c", a.DeviceCode)
	if login, err := gh.Viewer(t.Context(), g.Token.Access); g.Status != port.DeviceGranted || err != nil || login != "taro" {
		t.Errorf("grant: %+v %q %v", g, login, err)
	}
	for result, want := range map[string]port.DeviceStatus{"deny": port.DeviceDenied, "expire": port.DeviceExpired} {
		if err := d.answer(b.UserCode, result, ""); err != nil {
			t.Fatal(err)
		}
		if g, _ := d.PollDevice(t.Context(), "c", b.DeviceCode); g.Status != want {
			t.Errorf("%s: %v", result, g.Status)
		}
	}
	if d.answer("9999-9999", "grant", "x") == nil || d.answer(b.UserCode, "maybe", "") == nil {
		t.Error("an unknown code or result was accepted")
	}
}

func TestE2EDeviceHandler(t *testing.T) {
	d := newE2EDevice(&apptest.GitHub{})
	c, _ := d.StartDevice(t.Context(), "c")
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := d.handler("tok", next)
	do := func(method, target, auth string) int {
		r := httptest.NewRequest(method, target, nil)
		if auth != "" {
			r.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	deny := "/e2e/device?code=" + c.UserCode + "&result=deny"
	for _, tc := range []struct {
		method, target, auth string
		want                 int
	}{
		{http.MethodGet, "/api/accounts", "", http.StatusTeapot},
		{http.MethodPost, deny, "Bearer nope", http.StatusUnauthorized},
		{http.MethodGet, deny, "Bearer tok", http.StatusMethodNotAllowed},
		{http.MethodPost, "/e2e/device?code=0000-9999&result=deny", "Bearer tok", http.StatusBadRequest},
		{http.MethodPost, deny, "Bearer tok", http.StatusNoContent},
	} {
		if got := do(tc.method, tc.target, tc.auth); got != tc.want {
			t.Errorf("%s %s: %d, want %d", tc.method, tc.target, got, tc.want)
		}
	}
}

func TestE2ELoginsHaveTokens(t *testing.T) {
	tokens, users := e2eLogins()
	if users[tokens["gh-desktop-1-b"]] != "gh-desktop-1-b" || users[tokens["taro"]] != "taro" || len(tokens) != 13 {
		t.Errorf("tokens %v users %v", tokens, users)
	}
}
```

- [ ] **Step 6: 落ちるのを見る**

Run: `go test -tags e2e ./cmd/hedgehug/ -run 'TestE2E'`
Expected: FAIL(`newE2EDevice`・`e2eLogins` が未定義)

- [ ] **Step 7: 実装する**

`internal/app/apptest/github.go` の `Viewer` の下に足す:

```go
// AddUser は、トークン token をログイン名 login の人のものにする(通しのテストで、デバイスフローの許可に使う)。
func (g *GitHub) AddUser(token, login string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Users == nil {
		g.Users = map[string]string{}
	}
	g.Users[token] = login
}
```

`cmd/hedgehug/device_e2e.go`:

```go
//go:build e2e

package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// e2eGitHub・e2eDevice・e2eTokens は、wire が口に差し込み、wrap の入口が答えを書き込む。同じものを使うので、
// パッケージに 1 つずつ置く(D2 画面の計画の決定 S21・S22)。
var (
	e2eTokens, e2eUsers = e2eLogins()
	e2eGitHub           = &apptest.GitHub{Users: e2eUsers}
	e2eDeviceFlow       = newE2EDevice(e2eGitHub)
)

// e2eLogins は、gh CLI などのトークンで確かめられるログイン名とトークン。taro と、
// 3 つの幅 × やり直し 2 回 × 2 人(gh-{幅}-{回数}-{a か b})。
func e2eLogins() (tokens, users map[string]string) {
	tokens = map[string]string{"taro": "e2e-gh-token"}
	users = map[string]string{"e2e-gh-token": "taro"}
	for _, width := range []string{"phone", "tablet", "desktop"} {
		for _, retry := range []string{"0", "1"} {
			for _, who := range []string{"a", "b"} {
				login := "gh-" + width + "-" + retry + "-" + who
				tokens[login] = "tok-" + login
				users["tok-"+login] = login
			}
		}
	}
	return tokens, users
}

// e2eDevice は、通しのテストで GitHub のデバイスフローの代わりをする。コードは始めるたびに 0000-0001 から順に振り、
// テストが answer(POST /e2e/device)で、そのコードの答えを決める。答えがなければ待っている。
type e2eDevice struct {
	mu      sync.Mutex
	gh      *apptest.GitHub
	started int
	devices map[string]string           // user_code → device_code
	answers map[string]port.DeviceGrant // device_code → 答え
}

func newE2EDevice(gh *apptest.GitHub) *e2eDevice {
	return &e2eDevice{gh: gh, devices: map[string]string{}, answers: map[string]port.DeviceGrant{}}
}

func (d *e2eDevice) StartDevice(context.Context, string) (port.DeviceCode, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.started++
	dc := fmt.Sprintf("device-%d", d.started)
	uc := fmt.Sprintf("%04d-%04d", d.started/10000, d.started%10000)
	d.devices[uc] = dc
	return port.DeviceCode{
		DeviceCode: dc, UserCode: uc, VerificationURI: "https://github.com/login/device",
		ExpiresIn: 15 * time.Minute, Interval: time.Second,
	}, nil
}

func (d *e2eDevice) PollDevice(_ context.Context, _, deviceCode string) (port.DeviceGrant, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if g, ok := d.answers[deviceCode]; ok {
		return g, nil
	}
	return port.DeviceGrant{Status: port.DevicePending}, nil
}

func (d *e2eDevice) RefreshDevice(context.Context, string, string) (account.DeviceToken, error) {
	return account.DeviceToken{}, &ghcall.Failure{Kind: ghcall.TokenRejected}
}

// answer は、userCode のコードの答えを決める。grant は、login の人が許可したことにする(トークンを GitHub の偽物に足す)。
func (d *e2eDevice) answer(userCode, result, login string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	dc, ok := d.devices[userCode]
	if !ok {
		return fmt.Errorf("unknown code %q", userCode)
	}
	switch result {
	case "grant":
		tok := "tok-" + dc
		d.gh.AddUser(tok, login)
		d.answers[dc] = port.DeviceGrant{Status: port.DeviceGranted, Token: account.DeviceToken{Access: tok}}
	case "deny":
		d.answers[dc] = port.DeviceGrant{Status: port.DeviceDenied}
	case "expire":
		d.answers[dc] = port.DeviceGrant{Status: port.DeviceExpired}
	default:
		return errors.New("result is grant, deny or expire")
	}
	return nil
}

// handler は、POST /e2e/device?code=…&result=…&login=… を受け、ほかの要求を next に渡す。API と同じトークンを求める。
func (d *e2eDevice) handler(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/e2e/device" {
			next.ServeHTTP(w, r)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		if err := d.answer(q.Get("code"), q.Get("result"), q.Get("login")); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// wrap は、通しのテストがデバイスフローの答えを決める入口を足す。
func wrap(token string, h http.Handler) http.Handler { return e2eDeviceFlow.handler(token, h) }
```

`cmd/hedgehug/wire_e2e.go` の `httpapi.Deps` の 3 行を替える:

```go
		Secrets:     &apptest.Secrets{Tokens: e2eTokens},
		GitHub:      e2eGitHub,
		DeviceFlow:  e2eDeviceFlow,
```

`cmd/hedgehug/wire.go`(`!e2e`)の末尾に足す(import に `"net/http"`):

```go
// wrap は、本物のビルドでは何も足さない(e2e のビルドだけが、テストの入口を足す)。
func wrap(_ string, h http.Handler) http.Handler { return h }
```

`cmd/hedgehug/main.go` の `run` の最後の行を替える:

```go
	return serve(ctx, ln, wrap(token, httpapi.New(token, web.Dist(), deps)))
```

- [ ] **Step 8: 通るのを見る**

Run: `go test -tags e2e ./cmd/hedgehug/ && go test ./cmd/hedgehug/ ./internal/... && go vet -tags e2e ./cmd/hedgehug && golangci-lint run ./... && golangci-lint run --build-tags e2e ./cmd/...`
Expected: すべて PASS

- [ ] **Step 9: コミット**

```bash
git add internal/adapter/httpapi/github internal/app/apptest/github.go cmd/hedgehug api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat: デバイスフローのアカウントの再認証を 409 にし、e2e でデバイスフローの答えを決められるようにする"
```

---

### Task 2: 設定画面の枠(アドレス・歯車・見出し・タブ)

**Files:**
- Modify: `web/packages/core/src/features/router/route.ts`・`route.test.ts`・`resolve.ts`・`resolve.test.ts`
- Modify: `web/packages/core/src/index.ts`(`SettingsTab` を出す)
- Modify: `web/packages/ui/src/layout/PageHead.svelte`・`layout.test.ts`
- Modify: `web/app/src/shell/Sidebar.svelte`・`web/app/src/shell/Screen.svelte`
- Create: `web/app/src/screens/settings/SettingsScreen.svelte`
- Create: `web/e2e/settings.spec.ts`

**Interfaces:**
- Produces: `Route`・`Resolved` の `{ name: "settings"; tab: SettingsTab }`、`SettingsTab = "accounts"`、`PageHead` の `note?: string`、`SettingsScreen`(props `tab`・`ondrawer`)。

- [ ] **Step 1: 失敗するテストを書く**

`route.test.ts` に足す:

```ts
describe("settings", () => {
  it("reads /settings and /settings/accounts as the accounts tab", () => {
    expect(parseRoute("/settings")).toEqual({ name: "settings", tab: "accounts" });
    expect(parseRoute("/settings/accounts")).toEqual({ name: "settings", tab: "accounts" });
  });

  it("goes home for a tab that is not there", () => {
    expect(parseRoute("/settings/general")).toEqual({ name: "home" });
    expect(parseRoute("/settings/accounts/x")).toEqual({ name: "home" });
  });

  it("writes the settings address", () => {
    expect(formatRoute({ name: "settings", tab: "accounts" })).toBe("/settings/accounts");
  });
});
```

`resolve.test.ts` に足す(そのファイルの一覧の作り方に合わせ、Store のない一覧で確かめる):

```ts
it("resolves the settings without any store", async () => {
  const items = { findByNumber: async () => { throw new Error("unused"); } } as unknown as ItemsPort;
  expect(await resolveRoute({ name: "settings", tab: "accounts" }, [], items)).toEqual({
    name: "settings",
    tab: "accounts",
  });
});
```

(`ItemsPort` の import がなければ `import type { ItemsPort } from "../../port/items";` を足す。)

`layout.test.ts` の `describe("PageHead")` に足す:

```ts
  it("shows the note under the title", () => {
    render(PageHead, {
      props: { title: "設定", note: "このマシンの設定と、Store の管理", ondrawer: vi.fn() },
    });
    expect(screen.getByText("このマシンの設定と、Store の管理")).toBeInTheDocument();
  });
```

- [ ] **Step 2: 落ちるのを見る**

Run: `cd web && pnpm --filter @hedgehug/core test && pnpm --filter @hedgehug/ui test`
Expected: FAIL(settings の Route がない、note が出ない)

- [ ] **Step 3: 実装する**

`route.ts`: 先頭のコメントの綴りの表に `//   /settings/{タブ}                設定(D2 画面の計画の決定 S3)` を足し、次のように変える:

```ts
export type SettingsTab = "accounts";

export type Route =
  | { name: "home" }
  | { name: "workspace"; store: string; prefix: string }
  | { name: "project"; store: string; project: string; tab: ProjectTab }
  | { name: "item"; store: string; item: string }
  | { name: "settings"; tab: SettingsTab };

const settingsTabs: readonly string[] = ["accounts"] satisfies SettingsTab[];
```

`parseRoute` の `const [s, store, kind, id, tab, ...rest] = segs as string[];` の直前に足す:

```ts
  if (segs[0] === "settings" && segs.length <= 2) {
    const t = segs[1] ?? "accounts";
    return settingsTabs.includes(t) ? { name: "settings", tab: t as SettingsTab } : { name: "home" };
  }
```

`formatRoute` の switch に足す:

```ts
    case "settings":
      return `/settings/${route.tab}`;
```

`resolve.ts`: `Resolved` に `| { name: "settings"; tab: SettingsTab }` を足し(`SettingsTab` を `./route` から import)、`resolveRoute` の switch に足す:

```ts
    case "settings":
      return { name: "settings", tab: route.tab };
```

`core/src/index.ts` の route の export に `type SettingsTab,` を足す。

`PageHead.svelte`: Props に `note?: string;` を足して受け取り、`<h1 class="title">{title}</h1>` の下に足す:

```svelte
    {#if note}
      <p class="note">{note}</p>
    {/if}
```

style に足す:

```css
  .note {
    margin: 2px 0 0;
    color: var(--muted);
    font-size: 12.5px;
  }
```

`web/app/src/screens/settings/SettingsScreen.svelte`:

```svelte
<script lang="ts">
  import type { SettingsTab } from "@hedgehug/core";
  import { PageHead, Tabs } from "@hedgehug/ui";
  import { navigation } from "../../wire";

  // 設定の画面(08「設定画面の構成」)。タブは、作ったものだけを出す(D2 画面の計画の決定 S1・S4)。
  interface Props {
    tab: SettingsTab;
    ondrawer: () => void;
  }

  let { tab, ondrawer }: Props = $props();
  const tabs = [{ id: "accounts", label: "アカウントとメンバー" }];
</script>

<PageHead title="設定" note="このマシンの設定と、Store の管理" {ondrawer} />
<Tabs
  {tabs}
  current={tab}
  onselect={(id) => navigation.go({ name: "settings", tab: id as SettingsTab })}
/>
<div class="content"></div>

<style>
  .content {
    flex: 1;
    overflow: auto;
    padding: 16px 24px 40px;
    min-height: 0;
  }
  @container frame (max-width: 860px) {
    .content {
      padding: 12px max(16px, env(safe-area-inset-right, 0px)) 32px
        max(16px, env(safe-area-inset-left, 0px));
    }
  }
</style>
```

`Screen.svelte`: import に `import SettingsScreen from "../screens/settings/SettingsScreen.svelte";` を足し、`store` の derived を設定でも動くように替える:

```ts
  const store = $derived(
    current && "storeId" in current ? stores.find((s) => s.id === current.storeId) : undefined,
  );
```

`{#key current}` の中の最初の分岐の前に足す:

```svelte
  {#if current?.name === "settings"}
    <SettingsScreen tab={current.tab} {ondrawer} />
  {:else if current?.name === "project"}
```

(元の `{#if current?.name === "project"}` を `{:else if ...}` にする。)

`Sidebar.svelte`: `<Brand />` を次に替え、script に `import type { Route } from "@hedgehug/core";` と `const settings: Route = { name: "settings", tab: "accounts" };` を足す:

```svelte
<div class="top">
  <Brand />
  <a
    class="gear"
    href={links.href(settings)}
    aria-label="設定"
    title="設定"
    aria-current={current?.name === "settings" ? "page" : undefined}
    onclick={(e) => links.click(e, settings)}
    onauxclick={(e) => links.click(e, settings)}
  >
    <svg width="16" height="16" viewBox="0 0 24 24" aria-hidden="true">
      <circle cx="12" cy="12" r="3" />
      <path
        d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
      />
    </svg>
  </a>
</div>
```

style に足す:

```css
  .top {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .gear {
    display: inline-grid;
    place-items: center;
    width: 28px;
    height: 28px;
    border-radius: 8px;
    color: var(--muted);
  }
  .gear:hover,
  .gear[aria-current="page"] {
    background: var(--hover);
    color: var(--ink);
  }
  .gear svg {
    stroke: currentColor;
    fill: none;
    stroke-width: 1.8;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
```

`web/e2e/settings.spec.ts`:

```ts
import { expect, test } from "@playwright/test";
import { sidebar, tokenUrl } from "./helpers";

test("the gear in the sidebar opens the settings, and a reload keeps them", async ({
  page,
}, info) => {
  await page.goto(tokenUrl("/"));
  const side = await sidebar(page, info);
  await side.getByRole("link", { name: "設定" }).click();
  await expect(page).toHaveURL(/\/settings\/accounts$/);
  await expect(page.getByRole("heading", { name: "設定" })).toBeVisible();
  await expect(page.getByText("このマシンの設定と、Store の管理")).toBeVisible();
  await expect(page.getByRole("tab", { name: "アカウントとメンバー" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await page.reload();
  await expect(page.getByRole("heading", { name: "設定" })).toBeVisible();
});
```

- [ ] **Step 4: 通るのを見る**

Run: `cd web && pnpm test && pnpm lint && pnpm check && pnpm deps && pnpm size && cd .. && make e2e`
Expected: すべて PASS(e2e は 3 つの幅で settings.spec.ts を含めて通る)

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): 設定画面の枠と、サイドバーの歯車を足す"
```

---

### Task 3: アカウントの型と口(core・api)

**Files:**
- Create: `web/packages/core/src/model/account.ts`、`web/packages/core/src/port/accounts.ts`
- Modify: `web/packages/core/src/index.ts`
- Create: `web/packages/api/src/accounts.ts`、`web/packages/api/src/accounts.test.ts`
- Modify: `web/packages/api/src/index.ts`

**Interfaces:**
- Produces: 「型と口」の節の `model/account.ts` と `AccountsPort` のすべて。`createAccountsPort(): AccountsPort`。

- [ ] **Step 1: core の型と口を書く**(型だけでテストの対象がないので、この 2 ファイルは先に書く)

`web/packages/core/src/model/account.ts`:

```ts
// GitHub のアカウント(07「アカウント」)。トークンは画面に来ない。

export type AuthMethod = "device" | "gh" | "env" | "keychain";
// TokenMethod は、利用者が置いたトークンを読む認証方法。デバイスフローはサインインで登録・保存する。
export type TokenMethod = Exclude<AuthMethod, "device">;

export interface Account {
  id: string;
  name: string;
  login: string;
  method: AuthMethod;
}

// AccountRow は一覧の 1 行。reason は「認証エラー」の下の赤字(なければ空)。
export interface AccountRow extends Account {
  status: "ok" | "auth_error";
  reason: string;
}

export interface AccountValues {
  name: string;
  login: string;
  method: AuthMethod;
}

export interface TokenValues {
  name: string;
  login: string;
  method: TokenMethod;
}

// SignInStart は、サインインを始める入力。account は編集・再認証するアカウントの ID(追加では省く)。
export interface SignInStart {
  account?: string;
  name: string;
  login: string;
}

export interface SignInCode {
  id: string;
  userCode: string;
  verificationUri: string;
  expiresInSec: number;
  intervalSec: number;
}

export type SignInStatus = "waiting" | "expired" | "denied" | "mismatch" | "failed" | "done";

// SignInResult は、サインインの問い合わせの答え。account は done のときだけ。
export interface SignInResult {
  status: SignInStatus;
  message: string;
  intervalSec: number;
  account: Account | null;
}

export type SignInPurpose = "add" | "edit" | "reauth";

// SignInTarget は、サインインで登録・保存するもの。original は編集・再認証するアカウント(追加では null)。
export interface SignInTarget {
  purpose: SignInPurpose;
  original: Account | null;
  name: string;
  login: string;
}

// SignInFailure は、サインインが終わったときの赤字。loginError はログイン名の欄の下、error はダイアログの赤字。
export interface SignInFailure {
  error?: string;
  loginError?: string;
}
```

`web/packages/core/src/port/accounts.ts`:

```ts
import type {
  Account,
  AccountRow,
  SignInCode,
  SignInResult,
  SignInStart,
  TokenValues,
} from "../model/account";

// アカウントの口。実装は api パッケージにある。断りは Refused、ないものは NotFound で投げる。
export interface AccountsPort {
  list(): Promise<AccountRow[]>;
  add(values: TokenValues): Promise<Account>;
  edit(id: string, values: TokenValues): Promise<Account>;
  reauth(id: string): Promise<Account>;
  remove(id: string): Promise<void>;
  // deleteImpact は「アカウントを削除」の本文の行、editImpact は「アカウントを変更」の本文の行(空なら確認を出さない)。
  deleteImpact(id: string): Promise<string[]>;
  editImpact(id: string, login: string): Promise<string[]>;
  startSignIn(input: SignInStart): Promise<SignInCode>;
  pollSignIn(id: string): Promise<SignInResult>;
  cancelSignIn(id: string): Promise<void>;
}
```

`core/src/index.ts` に足す:

```ts
export type {
  Account,
  AccountRow,
  AccountValues,
  AuthMethod,
  SignInCode,
  SignInFailure,
  SignInPurpose,
  SignInResult,
  SignInStart,
  SignInStatus,
  SignInTarget,
  TokenMethod,
  TokenValues,
} from "./model/account";
export type { AccountsPort } from "./port/accounts";
```

- [ ] **Step 2: api の失敗するテストを書く**

`web/packages/api/src/accounts.test.ts`:

```ts
import { Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createAccountsPort } from "./accounts";
import { setToken } from "./fetcher";

afterEach(() => {
  vi.unstubAllGlobals();
});

function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async () =>
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetch);
  setToken("tok");
  return fetch;
}

const sent = (fetch: ReturnType<typeof respond>) =>
  fetch.mock.calls[0] as unknown as [string, RequestInit];

const work = { id: "A1", name: "work", login: "taro", method: "device" };

describe("accounts port", () => {
  it("lists the rows with their state, and reads null as none", async () => {
    const fetch = respond(200, {
      accounts: [{ ...work, method: "gh", status: "auth_error", reason: "x" }],
    });
    expect(await createAccountsPort().list()).toEqual([
      { ...work, method: "gh", status: "auth_error", reason: "x" },
    ]);
    expect(sent(fetch)[0]).toBe("/api/accounts");
    respond(200, { accounts: null });
    expect(await createAccountsPort().list()).toEqual([]);
  });

  it("turns a 422 into a refusal with the spec text", async () => {
    respond(422, { detail: "同じ表示名のアカウントがあります" });
    const e = await createAccountsPort()
      .add({ name: "w", login: "t", method: "gh" })
      .catch((x: unknown) => x);
    expect(e).toBeInstanceOf(Refused);
    expect((e as Refused).message).toBe("同じ表示名のアカウントがあります");
  });

  it("starts a sign-in with the account and maps the code", async () => {
    const fetch = respond(201, {
      id: "F1",
      user_code: "WDJB-MJHT",
      verification_uri: "https://github.com/login/device",
      expires_in: 900,
      interval: 5,
    });
    expect(
      await createAccountsPort().startSignIn({ account: "A1", name: "work", login: "taro" }),
    ).toEqual({
      id: "F1",
      userCode: "WDJB-MJHT",
      verificationUri: "https://github.com/login/device",
      expiresInSec: 900,
      intervalSec: 5,
    });
    expect(JSON.parse(String(sent(fetch)[1].body))).toEqual({
      account: "A1",
      name: "work",
      login: "taro",
    });
  });

  it("asks the sign-in without the cache and maps the answer", async () => {
    const fetch = respond(200, { state: "done", message: "", interval: 5, account: work });
    expect(await createAccountsPort().pollSignIn("F1")).toEqual({
      status: "done",
      message: "",
      intervalSec: 5,
      account: work,
    });
    expect(sent(fetch)[0]).toBe("/api/sign-ins/F1");
    expect(sent(fetch)[1].cache).toBe("no-store");
    respond(200, { state: "waiting", message: "", interval: 10 });
    expect((await createAccountsPort().pollSignIn("F1")).account).toBeNull();
  });

  it("sends the new login to the edit impact, and reads null lines as none", async () => {
    const fetch = respond(200, { lines: null });
    expect(await createAccountsPort().editImpact("A1", "jiro")).toEqual([]);
    expect(sent(fetch)[0]).toBe("/api/accounts/A1/edit-impact?login=jiro");
  });
});
```

- [ ] **Step 3: 落ちるのを見る**

Run: `cd web && pnpm --filter @hedgehug/api test`
Expected: FAIL(`./accounts` がない)

- [ ] **Step 4: 実装する**

`web/packages/api/src/accounts.ts`:

```ts
import type { Account, AccountRow, AccountsPort, SignInResult } from "@hedgehug/core";
import { call } from "./errors";
import {
  type AccountBody,
  accountDeleteImpact,
  accountEditImpact,
  addAccount,
  cancelSignIn,
  deleteAccount,
  editAccount,
  listAccounts,
  pollSignIn,
  reauthAccount,
  startSignIn,
} from "./generated";

// アカウントの口(07「アカウント」)。サインインの問い合わせ(GET)は登録の副作用を持つので、
// キャッシュに乗せない(D2 画面の計画の決定 S8)。

const toAccount = (a: AccountBody): Account => ({
  id: a.id,
  name: a.name,
  login: a.login,
  method: a.method,
});

export function createAccountsPort(): AccountsPort {
  return {
    list: () =>
      call(async () =>
        ((await listAccounts()).accounts ?? []).map(
          (r): AccountRow => ({ ...toAccount(r), status: r.status, reason: r.reason }),
        ),
      ),
    add: (values) => call(async () => toAccount(await addAccount(values))),
    edit: (id, values) => call(async () => toAccount(await editAccount(id, values))),
    reauth: (id) => call(async () => toAccount(await reauthAccount(id))),
    remove: (id) => call(() => deleteAccount(id)),
    deleteImpact: (id) => call(async () => (await accountDeleteImpact(id)).lines ?? []),
    editImpact: (id, login) =>
      call(async () => (await accountEditImpact(id, { login })).lines ?? []),
    startSignIn: (input) =>
      call(async () => {
        const c = await startSignIn(input);
        return {
          id: c.id,
          userCode: c.user_code,
          verificationUri: c.verification_uri,
          expiresInSec: c.expires_in,
          intervalSec: c.interval,
        };
      }),
    pollSignIn: (id) =>
      call(async (): Promise<SignInResult> => {
        const r = await pollSignIn(id, { cache: "no-store" });
        return {
          status: r.state,
          message: r.message,
          intervalSec: r.interval,
          account: r.account ? toAccount(r.account) : null,
        };
      }),
    cancelSignIn: (id) => call(() => cancelSignIn(id)),
  };
}
```

`web/packages/api/src/index.ts` に `export { createAccountsPort } from "./accounts";` を足す(並びはアルファベット順で先頭)。

- [ ] **Step 5: 通るのを見る**

Run: `cd web && pnpm test && pnpm lint && pnpm check && pnpm deps`
Expected: PASS

- [ ] **Step 6: コミット**

```bash
git add web/packages
git commit -m "feat(web): アカウントの型と口、API の結び付けを足す"
```

---

### Task 4: 文言と一覧(行の赤字・再認証・削除)

**Files:**
- Create: `web/packages/core/src/features/accounts/text.ts`・`text.test.ts`
- Create: `web/packages/core/src/features/accounts/fixtures.test-util.ts`
- Create: `web/packages/core/src/features/accounts/list.ts`・`list.test.ts`
- Modify: `web/packages/core/src/index.ts`

**Interfaces:**
- Consumes: Task 3 の型と `AccountsPort`。`Toasts`(`features/toasts/toasts.ts`)。
- Produces:
  - `text.ts`: `methodChoices: { value: AuthMethod; label: string }[]`、`methodLine(m: AuthMethod): string`、`addedMessage`・`changedMessage`・`deletedMessage`(string)、`signedInMessage(login: string)`、`reauthedMessage(a: { name: string; login: string })`、`askingText`・`waitingText`・`expiredText`・`deniedText`(string)
  - `list.ts`: `createAccountList(deps: AccountListDeps): AccountList`、`AccountListState`(下のコード)
  - `fixtures.test-util.ts`: `row`・`code`・`waiting`・`fakeAccounts`・`fakeTimer`・`fakeToasts`・`flush`(Task 5・6 でも使う)

- [ ] **Step 1: テストの道具を書く**(テストだけが使う)

`web/packages/core/src/features/accounts/fixtures.test-util.ts`:

```ts
import type { Account, AccountRow, SignInCode, SignInResult } from "../../model/account";
import { NotFound, Refused } from "../../model/errors";
import type { AccountsPort } from "../../port/accounts";
import { get } from "../../reactive/store";
import { createToasts, type Timer } from "../toasts/toasts";

export function row(id: string, patch: Partial<AccountRow> = {}): AccountRow {
  return { id, name: `${id} の名前`, login: id.toLowerCase(), method: "gh", status: "ok", reason: "", ...patch };
}

export function code(id: string): SignInCode {
  return {
    id,
    userCode: `CODE-${id}`,
    verificationUri: "https://github.com/login/device",
    expiresInSec: 900,
    intervalSec: 5,
  };
}

export const waiting: SignInResult = { status: "waiting", message: "", intervalSec: 5, account: null };

// fakeAccounts は、メモリの口。calls に呼ばれた操作を「名前 引数…」(省いた引数は -)で残す。
// refuse[名前] があれば、その操作を Refused(その文言)で断る。impacts は削除・変更の確認に足す行。
// polls は pollSignIn が順に返す答え(Error なら投げる。尽きたら waiting)。startSignIn のコードは S1 から順に振る。
export function fakeAccounts(rows: AccountRow[] = []) {
  const calls: string[] = [];
  const refuse: Record<string, string> = {};
  const impacts = { delete: [] as string[], edit: [] as string[] };
  const polls: (SignInResult | Error)[] = [];
  let started = 0;
  function op(name: string, ...args: (string | undefined)[]) {
    calls.push([name, ...args.map((a) => a ?? "-")].join(" "));
    const m = refuse[name];
    if (m !== undefined) throw new Refused(m);
  }
  function find(id: string): AccountRow {
    const r = rows.find((x) => x.id === id);
    if (!r) throw new NotFound("not found");
    return r;
  }
  const toAccount = (r: AccountRow): Account => ({ id: r.id, name: r.name, login: r.login, method: r.method });
  const port: AccountsPort = {
    list: async () => rows.map((r) => ({ ...r })),
    async add(v) {
      op("add", v.name, v.login, v.method);
      const r = row(`A${rows.length + 1}`, { ...v });
      rows.push(r);
      return toAccount(r);
    },
    async edit(id, v) {
      op("edit", id, v.name, v.login, v.method);
      const r = find(id);
      Object.assign(r, v);
      return toAccount(r);
    },
    async reauth(id) {
      op("reauth", id);
      const r = find(id);
      r.status = "ok";
      r.reason = "";
      return toAccount(r);
    },
    async remove(id) {
      op("remove", id);
      find(id);
      rows.splice(
        rows.findIndex((x) => x.id === id),
        1,
      );
    },
    async deleteImpact(id) {
      op("deleteImpact", id);
      return [`「${find(id).name}」を削除します。`, ...impacts.delete];
    },
    async editImpact(id, login) {
      op("editImpact", id, login);
      find(id);
      return impacts.edit;
    },
    async startSignIn(input) {
      op("startSignIn", input.account, input.name, input.login);
      started += 1;
      return code(`S${started}`);
    },
    async pollSignIn(id) {
      op("pollSignIn", id);
      const p = polls.shift() ?? waiting;
      if (p instanceof Error) throw p;
      return p;
    },
    async cancelSignIn(id) {
      op("cancelSignIn", id);
    },
  };
  return { port, rows, calls, refuse, impacts, polls };
}

// fakeTimer は、時間を手で進める時計。
export function fakeTimer() {
  let now = 0;
  let seq = 0;
  const pending = new Map<number, { at: number; fn: () => void }>();
  const timer: Timer = {
    set(fn, ms) {
      seq += 1;
      pending.set(seq, { at: now + ms, fn });
      return seq;
    },
    clear(handle) {
      pending.delete(handle as number);
    },
  };
  const advance = (ms: number) => {
    now += ms;
    for (const [h, p] of [...pending]) {
      if (p.at <= now) {
        pending.delete(h);
        p.fn();
      }
    }
  };
  return { timer, advance, pending };
}

// fakeToasts は、出したトーストの文を読めるトースト。
export function fakeToasts() {
  const toasts = createToasts({ set: () => 0, clear: () => undefined });
  return { toasts, texts: () => get(toasts.list).map((t) => t.text) };
}

// flush は、待っている Promise の続きを進める。
export const flush = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
```

- [ ] **Step 2: 失敗するテストを書く**

`text.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { methodChoices, methodLine, reauthedMessage, signedInMessage } from "./text";

describe("account texts", () => {
  it("lists the four methods in the order of 07, the device flow first", () => {
    expect(methodChoices).toEqual([
      { value: "device", label: "ブラウザでサインイン(デバイスフロー)" },
      { value: "gh", label: "gh CLI のトークン" },
      { value: "env", label: "環境変数" },
      { value: "keychain", label: "OS のキーチェーン" },
    ]);
  });

  it("writes the method line of a row", () => {
    expect(methodLine("device")).toBe("github.com · デバイスフローのトークン");
    expect(methodLine("gh")).toBe("github.com · gh CLI のトークン");
    expect(methodLine("env")).toBe("github.com · 環境変数 GH_TOKEN");
    expect(methodLine("keychain")).toBe("github.com · OS のキーチェーン");
  });

  it("writes the toasts with the names", () => {
    expect(signedInMessage("taro")).toBe("@taro としてサインインしました");
    expect(reauthedMessage({ name: "work", login: "taro" })).toBe("work: @taro として再認証しました");
  });
});
```

`list.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { Account, AccountRow } from "../../model/account";
import { Refused } from "../../model/errors";
import { get } from "../../reactive/store";
import { fakeAccounts, fakeToasts, row } from "./fixtures.test-util";
import { createAccountList } from "./list";

function setup(rows: AccountRow[]) {
  const f = fakeAccounts(rows);
  const { toasts, texts } = fakeToasts();
  const device: string[] = [];
  let refusal = "";
  const list = createAccountList({
    accounts: f.port,
    toasts,
    async reauthDevice(a: Account) {
      device.push(a.id);
      if (refusal) throw new Refused(refusal);
    },
  });
  const refuseDevice = (m: string) => {
    refusal = m;
  };
  return { f, texts, device, list, refuseDevice };
}

describe("createAccountList", () => {
  it("loads the rows when opened", async () => {
    const { list } = setup([row("A1"), row("A2", { status: "auth_error", reason: "x" })]);
    await list.open();
    expect(get(list.state).rows.map((r) => [r.id, r.status])).toEqual([
      ["A1", "ok"],
      ["A2", "auth_error"],
    ]);
  });

  it("reauths a gh account, tells it and reloads", async () => {
    const { f, list, texts } = setup([
      row("A1", { name: "work", login: "taro", status: "auth_error" }),
    ]);
    await list.open();
    await list.reauth("A1");
    expect(f.calls).toEqual(["reauth A1"]);
    expect(texts()).toEqual(["work: @taro として再認証しました"]);
    expect(get(list.state).rows[0]?.status).toBe("ok");
  });

  it("marks the row as asking while GitHub is asked, and does not ask twice", async () => {
    const { f, list } = setup([row("A1", { status: "auth_error" })]);
    await list.open();
    let release: () => void = () => undefined;
    const reauth = f.port.reauth;
    f.port.reauth = async (id) => {
      await new Promise<void>((r) => {
        release = r;
      });
      return reauth(id);
    };
    const first = list.reauth("A1");
    expect(get(list.state).asking).toEqual(["A1"]);
    await list.reauth("A1");
    release();
    await first;
    expect(get(list.state).asking).toEqual([]);
    expect(f.calls).toEqual(["reauth A1"]);
  });

  it("keeps a failed reauth in the row until the row is touched or the screen is opened again", async () => {
    const { f, list } = setup([row("A1", { status: "auth_error" })]);
    await list.open();
    f.refuse.reauth = "環境変数 GH_TOKEN が設定されていません";
    await list.reauth("A1");
    expect(get(list.state).errors).toEqual({ A1: "環境変数 GH_TOKEN が設定されていません" });
    list.touch("A1");
    expect(get(list.state).errors).toEqual({});
    await list.reauth("A1");
    await list.open();
    expect(get(list.state).errors).toEqual({});
  });

  it("opens the sign-in for a device flow account, and shows its refusal in the row", async () => {
    const { list, device, refuseDevice } = setup([
      row("A1", { method: "device", status: "auth_error" }),
    ]);
    await list.open();
    await list.reauth("A1");
    expect(device).toEqual(["A1"]);
    refuseDevice("GitHub に接続できません。接続を確かめて、もう一度お試しください");
    await list.reauth("A1");
    expect(get(list.state).errors.A1).toBe(
      "GitHub に接続できません。接続を確かめて、もう一度お試しください",
    );
  });

  it("asks with the impact before deleting, and deletes on confirm", async () => {
    const { f, list, texts } = setup([row("A1", { name: "work" })]);
    await list.open();
    f.impacts.delete = ["チーム の同期に使っているため、それらは同期できなくなります。"];
    await list.askDelete("A1");
    expect(get(list.state).deleting).toEqual({
      id: "A1",
      lines: ["「work」を削除します。", "チーム の同期に使っているため、それらは同期できなくなります。"],
    });
    list.cancelDelete();
    expect(get(list.state).deleting).toBeNull();
    await list.askDelete("A1");
    await list.confirmDelete();
    expect(f.calls).toContain("remove A1");
    expect(texts()).toEqual(["アカウントを削除しました"]);
    expect(get(list.state).rows).toEqual([]);
    expect(get(list.state).deleting).toBeNull();
  });
});
```

- [ ] **Step 3: 落ちるのを見る**

Run: `cd web && pnpm --filter @hedgehug/core test -- accounts`
Expected: FAIL(`./text`・`./list` がない)

- [ ] **Step 4: 実装する**

`web/packages/core/src/features/accounts/text.ts`:

```ts
import type { AuthMethod } from "../../model/account";

// アカウントの画面の文言(07「アカウント」)。

// methodChoices は、「認証方法」の選択肢。並びは 07「登録」の表の順で、既定はデバイスフロー。
export const methodChoices: { value: AuthMethod; label: string }[] = [
  { value: "device", label: "ブラウザでサインイン(デバイスフロー)" },
  { value: "gh", label: "gh CLI のトークン" },
  { value: "env", label: "環境変数" },
  { value: "keychain", label: "OS のキーチェーン" },
];

const listWords: Record<AuthMethod, string> = {
  device: "デバイスフローのトークン",
  gh: "gh CLI のトークン",
  env: "環境変数 GH_TOKEN",
  keychain: "OS のキーチェーン",
};

// methodLine は、一覧の行の「github.com · {一覧に出す文言}」。
export const methodLine = (m: AuthMethod) => `github.com · ${listWords[m]}`;

export const addedMessage = "アカウントを追加しました";
export const changedMessage = "アカウントを変更しました";
export const deletedMessage = "アカウントを削除しました";
export const signedInMessage = (login: string) => `@${login} としてサインインしました`;
export const reauthedMessage = (a: { name: string; login: string }) =>
  `${a.name}: @${a.login} として再認証しました`;

export const askingText = "GitHub に問い合わせています";
export const waitingText = "許可を待っています";
export const expiredText = "コードの期限が切れました";
export const deniedText = "GitHub で許可が拒否されました";
```

`web/packages/core/src/features/accounts/list.ts`:

```ts
import type { Account, AccountRow } from "../../model/account";
import { NotFound, Refused } from "../../model/errors";
import type { AccountsPort } from "../../port/accounts";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Toasts } from "../toasts/toasts";
import { deletedMessage, reauthedMessage } from "./text";

// 設定「アカウントとメンバー」の「GitHub アカウント」の一覧(07「状態」「再認証」「削除の影響」)。

export interface AccountListState {
  rows: AccountRow[];
  // errors は、行の中(状態の下)の赤字。行の次の操作・画面を開き直す・成功のどれかで消す(D2 画面の計画の決定 S14)。
  errors: Record<string, string>;
  // asking は、GitHub に問い合わせている行の ID。そのあいだ「再認証」を無効にする。
  asking: string[];
  // deleting は、「アカウントを削除」の確認。開いていなければ null。
  deleting: { id: string; lines: string[] } | null;
}

export interface AccountList {
  state: Readable<AccountListState>;
  // open は、設定画面を開いたときに呼ぶ。行の赤字を消して読み直す。
  open(): Promise<void>;
  reload(): Promise<void>;
  // touch は、行の「編集」などを押したときに、その行の赤字を消す。
  touch(id: string): void;
  reauth(id: string): Promise<void>;
  askDelete(id: string): Promise<void>;
  confirmDelete(): Promise<void>;
  cancelDelete(): void;
}

export interface AccountListDeps {
  accounts: AccountsPort;
  toasts: Toasts;
  // reauthDevice は、デバイスフローの行の「再認証」。コードを取って「GitHub にサインイン」を開く。
  // コードを取れなければ Refused を投げる(行の赤字にする。決定 S13)。
  reauthDevice(account: Account): Promise<void>;
}

const initial = (): AccountListState => ({ rows: [], errors: {}, asking: [], deleting: null });

export function createAccountList(deps: AccountListDeps): AccountList {
  const state = writable<AccountListState>(initial());
  const patch = (p: Partial<AccountListState>) => state.update((s) => ({ ...s, ...p }));

  function setError(id: string, message: string) {
    state.update((s) => {
      const errors = Object.fromEntries(Object.entries(s.errors).filter(([k]) => k !== id));
      return { ...s, errors: message === "" ? errors : { ...errors, [id]: message } };
    });
  }

  function setAsking(id: string, on: boolean) {
    state.update((s) => ({
      ...s,
      asking: on ? [...s.asking, id] : s.asking.filter((x) => x !== id),
    }));
  }

  async function reload() {
    patch({ rows: await deps.accounts.list() });
  }

  async function ask(row: AccountRow) {
    if (row.method === "device") {
      await deps.reauthDevice(row);
      return;
    }
    deps.toasts.show(reauthedMessage(await deps.accounts.reauth(row.id)));
    await reload();
  }

  return {
    state: readonly(state),
    async open() {
      state.set(initial());
      await reload();
    },
    reload,
    touch: (id) => setError(id, ""),
    async reauth(id) {
      const row = get(state).rows.find((r) => r.id === id);
      if (!row || get(state).asking.includes(id)) return;
      setError(id, "");
      setAsking(id, true);
      try {
        await ask(row);
      } catch (e) {
        if (e instanceof Refused) setError(id, e.message);
        else if (e instanceof NotFound) await reload();
        else throw e;
      } finally {
        setAsking(id, false);
      }
    },
    async askDelete(id) {
      setError(id, "");
      try {
        patch({ deleting: { id, lines: await deps.accounts.deleteImpact(id) } });
      } catch (e) {
        if (!(e instanceof NotFound)) throw e;
        await reload();
      }
    },
    async confirmDelete() {
      const d = get(state).deleting;
      if (!d) return;
      patch({ deleting: null });
      try {
        await deps.accounts.remove(d.id);
        deps.toasts.show(deletedMessage);
      } catch (e) {
        if (!(e instanceof NotFound)) throw e;
      }
      await reload();
    },
    cancelDelete: () => patch({ deleting: null }),
  };
}
```

`core/src/index.ts` に足す:

```ts
export {
  type AccountList,
  type AccountListState,
  createAccountList,
} from "./features/accounts/list";
export {
  addedMessage,
  askingText,
  changedMessage,
  deletedMessage,
  deniedText,
  expiredText,
  methodChoices,
  methodLine,
  reauthedMessage,
  signedInMessage,
  waitingText,
} from "./features/accounts/text";
```

- [ ] **Step 5: 通るのを見る**

Run: `cd web && pnpm test && pnpm lint && pnpm check && pnpm size`
Expected: PASS

- [ ] **Step 6: コミット**

```bash
git add web/packages/core
git commit -m "feat(web): アカウントの一覧(行の赤字・再認証・削除の確認)の状態を足す"
```

---

### Task 5: 「アカウントを追加」「アカウントを編集」と「アカウントを変更」の確認

**Files:**
- Create: `web/packages/core/src/features/accounts/form.ts`・`form.test.ts`
- Modify: `web/packages/core/src/index.ts`

**Interfaces:**
- Consumes: Task 3 の型・口、Task 4 の `text.ts` と `fixtures.test-util.ts`。
- Produces: `createAccountForm(deps: AccountFormDeps): AccountForm`、`AccountFormState`(下のコード)。

- [ ] **Step 1: 失敗するテストを書く**

`form.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { AccountRow, SignInCode, SignInTarget } from "../../model/account";
import { get } from "../../reactive/store";
import { code, fakeAccounts, fakeToasts, row } from "./fixtures.test-util";
import { createAccountForm } from "./form";

function setup(rows: AccountRow[] = []) {
  const f = fakeAccounts(rows);
  const { toasts, texts } = fakeToasts();
  const opened: { target: SignInTarget; code: SignInCode }[] = [];
  let saved = 0;
  const form = createAccountForm({
    accounts: f.port,
    toasts,
    signIn: (target, c) => void opened.push({ target, code: c }),
    onSaved: () => {
      saved += 1;
    },
  });
  return { f, texts, opened, form, saved: () => saved };
}

const clientIdText =
  "デバイスフローを使うには、config.yaml の hosts に github.com の client_id を設定してください。gh CLI のトークン・環境変数・OS のキーチェーンは、設定なしで使えます";

describe("createAccountForm", () => {
  it("opens add with the device flow first and empty fields", () => {
    const { form } = setup();
    form.openAdd();
    expect(get(form.state)).toEqual({
      original: null,
      form: { name: "", login: "", method: "device" },
      error: "",
      loginError: "",
      asking: false,
      confirm: null,
    });
  });

  it("adds a gh account after GitHub confirms it", async () => {
    const { f, form, texts, saved } = setup();
    form.openAdd();
    form.update({ name: "work", login: "taro", method: "gh" });
    await form.submit();
    expect(f.calls).toEqual(["add work taro gh"]);
    expect(get(form.state)).toBeNull();
    expect(texts()).toEqual(["アカウントを追加しました"]);
    expect(saved()).toBe(1);
  });

  it("shows the refusal and keeps the input", async () => {
    const { f, form } = setup();
    form.openAdd();
    form.update({ name: " ", login: "taro", method: "env" });
    f.refuse.add = "表示名とログイン名を入力してください";
    await form.submit();
    expect(get(form.state)).toMatchObject({
      error: "表示名とログイン名を入力してください",
      asking: false,
      form: { name: " ", login: "taro", method: "env" },
    });
  });

  it("does not send twice while asking", async () => {
    const { f, form } = setup();
    form.openAdd();
    form.update({ name: "work", login: "taro", method: "gh" });
    await Promise.all([form.submit(), form.submit()]);
    expect(f.calls).toEqual(["add work taro gh"]);
  });

  it("gets a code and hands it to the sign-in for the device flow", async () => {
    const { f, form, opened } = setup();
    form.openAdd();
    form.update({ name: "work", login: "taro" });
    await form.submit();
    expect(f.calls).toEqual(["startSignIn - work taro"]);
    expect(get(form.state)).toBeNull();
    expect(opened).toEqual([
      { target: { purpose: "add", original: null, name: "work", login: "taro" }, code: code("S1") },
    ]);
  });

  it("shows why the code could not be had, in the dialog", async () => {
    const { f, form, opened } = setup();
    form.openAdd();
    form.update({ name: "work", login: "taro" });
    f.refuse.startSignIn = clientIdText;
    await form.submit();
    expect(get(form.state)?.error).toBe(clientIdText);
    expect(opened).toEqual([]);
  });

  it("cancels the code when the dialog was closed while asking", async () => {
    const { f, form, opened } = setup();
    form.openAdd();
    form.update({ name: "work", login: "taro" });
    const sending = form.submit();
    form.close();
    await sending;
    expect(f.calls).toEqual(["startSignIn - work taro", "cancelSignIn S1"]);
    expect(opened).toEqual([]);
  });

  it("edits with the current values, and asks first when あなた changes", async () => {
    const a = row("A1", { name: "work", login: "taro" });
    const { f, form, texts } = setup([a]);
    form.openEdit(a);
    expect(get(form.state)?.form).toEqual({ name: "work", login: "taro", method: "gh" });
    expect(get(form.state)?.original).toEqual({ id: "A1", name: "work", login: "taro", method: "gh" });
    form.update({ login: "jiro" });
    f.impacts.edit = ["チーム での あなた が @taro から jiro に変わります。"];
    await form.submit();
    expect(get(form.state)?.confirm).toEqual(["チーム での あなた が @taro から jiro に変わります。"]);
    expect(f.calls).toEqual(["editImpact A1 jiro"]);
    form.cancelConfirm();
    expect(get(form.state)?.confirm).toBeNull();
    await form.submit();
    await form.confirm();
    expect(f.calls).toEqual(["editImpact A1 jiro", "editImpact A1 jiro", "edit A1 work jiro gh"]);
    expect(get(form.state)).toBeNull();
    expect(texts()).toEqual(["アカウントを変更しました"]);
  });

  it("saves without asking when the login stays or あなた does not change", async () => {
    const a = row("A1", { name: "work", login: "taro" });
    const { f, form } = setup([a]);
    form.openEdit(a);
    form.update({ name: "home" });
    await form.submit();
    form.openEdit(a);
    form.update({ login: "jiro" });
    await form.submit();
    expect(f.calls).toEqual(["edit A1 home taro gh", "editImpact A1 jiro", "edit A1 home jiro gh"]);
  });

  it("signs in again with the account when the method becomes the device flow", async () => {
    const a = row("A1", { name: "work", login: "taro" });
    const { f, form, opened } = setup([a]);
    form.openEdit(a);
    form.update({ method: "device" });
    await form.submit();
    expect(f.calls).toEqual(["startSignIn A1 work taro"]);
    expect(opened[0]?.target).toEqual({
      purpose: "edit",
      original: { id: "A1", name: "work", login: "taro", method: "gh" },
      name: "work",
      login: "taro",
    });
  });

  it("comes back from the sign-in with the device flow and the red text", () => {
    const { form } = setup();
    form.back(
      { purpose: "add", original: null, name: "work", login: "taro" },
      { loginError: "このトークンは @jiro のものです。ログイン名を直してください" },
    );
    expect(get(form.state)).toMatchObject({
      original: null,
      form: { name: "work", login: "taro", method: "device" },
      loginError: "このトークンは @jiro のものです。ログイン名を直してください",
      error: "",
    });
  });
});
```

- [ ] **Step 2: 落ちるのを見る**

Run: `cd web && pnpm --filter @hedgehug/core test -- form`
Expected: FAIL(`./form` がない)

- [ ] **Step 3: 実装する**

`web/packages/core/src/features/accounts/form.ts`:

```ts
import type {
  Account,
  AccountValues,
  SignInCode,
  SignInFailure,
  SignInTarget,
} from "../../model/account";
import { NotFound, Refused } from "../../model/errors";
import type { AccountsPort } from "../../port/accounts";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Toasts } from "../toasts/toasts";
import { addedMessage, changedMessage } from "./text";

// 「アカウントを追加」「アカウントを編集」(07「登録」「認証の確認」「編集」)。検査と確かめはサーバーに任せ、
// 断りの文言を赤字で出す。認証方法がデバイスフローなら、コードを取って「GitHub にサインイン」へ渡す。
// 編集でログイン名を変えたら、先に「アカウントを変更」の確認を出す(D2 画面の計画の決定 S7)。

export interface AccountFormState {
  // original は、編集するアカウントのいまの値。追加では null。
  original: Account | null;
  form: AccountValues;
  error: string;
  // loginError は、ログイン名の欄の下の赤字(デバイスフローで別の人が許可したとき)。
  loginError: string;
  // asking は、GitHub に問い合わせているあいだ。「追加」「保存」を無効にする。
  asking: boolean;
  // confirm は、「アカウントを変更」の確認の本文の行。開いていなければ null。
  confirm: string[] | null;
}

export interface AccountForm {
  state: Readable<AccountFormState | null>;
  openAdd(): void;
  openEdit(account: Account): void;
  update(patch: Partial<AccountValues>): void;
  submit(): Promise<void>;
  // confirm は確認の「変更」、cancelConfirm は「キャンセル」(編集に戻る)。
  confirm(): Promise<void>;
  cancelConfirm(): void;
  // back は、サインインが別の人の許可・保存の失敗・コードを取れないことで終わったとき、元のダイアログに戻す。
  back(target: SignInTarget, failure: SignInFailure): void;
  close(): void;
}

export interface AccountFormDeps {
  accounts: AccountsPort;
  toasts: Toasts;
  // signIn は、取ったコードで「GitHub にサインイン」を開く。
  signIn(target: SignInTarget, code: SignInCode): void;
  // onSaved は、保存したあと(と、編集中のアカウントがなかったとき)に一覧を読み直す。
  onSaved(): void;
}

const pick = (a: Account): Account => ({ id: a.id, name: a.name, login: a.login, method: a.method });

export function createAccountForm(deps: AccountFormDeps): AccountForm {
  const state = writable<AccountFormState | null>(null);
  // opened は、開くたび・閉じるたびに進める。問い合わせのあいだに閉じた・開き直したかを見分ける。
  let opened = 0;
  // busy は、送っているあいだ(二重に送らない)。
  let busy = false;
  const patch = (p: Partial<AccountFormState>) => state.update((s) => (s ? { ...s, ...p } : s));

  function open(original: Account | null, form: AccountValues, failure: SignInFailure = {}) {
    opened += 1;
    state.set({
      original,
      form,
      error: failure.error ?? "",
      loginError: failure.loginError ?? "",
      asking: false,
      confirm: null,
    });
  }

  function close() {
    opened += 1;
    state.set(null);
  }

  async function signIn(s: AccountFormState, mine: number) {
    const { original, form } = s;
    const code = await deps.accounts.startSignIn({
      account: original?.id,
      name: form.name,
      login: form.login,
    });
    if (mine !== opened) {
      void deps.accounts.cancelSignIn(code.id).catch(() => undefined);
      return;
    }
    close();
    deps.signIn(
      { purpose: original ? "edit" : "add", original, name: form.name, login: form.login },
      code,
    );
  }

  async function saveToken(s: AccountFormState, mine: number) {
    const { original, form } = s;
    if (form.method === "device") return;
    const values = { name: form.name, login: form.login, method: form.method };
    if (original) await deps.accounts.edit(original.id, values);
    else await deps.accounts.add(values);
    if (mine === opened) close();
    deps.toasts.show(original ? changedMessage : addedMessage);
    deps.onSaved();
  }

  // save は、確かめて保存する。mine は送り始めたときの opened(あいだに閉じたら、結果をダイアログに出さない)。
  async function save(s: AccountFormState, mine: number) {
    patch({ asking: true, error: "", loginError: "", confirm: null });
    try {
      if (s.form.method === "device") await signIn(s, mine);
      else await saveToken(s, mine);
    } catch (e) {
      if (mine === opened) patch({ asking: false });
      if (e instanceof Refused) {
        if (mine === opened) patch({ error: e.message });
        return;
      }
      if (!(e instanceof NotFound)) throw e;
      if (mine === opened) close();
      deps.onSaved();
    }
  }

  // needsConfirm は、ログイン名を変えた編集で「あなた」が変わる Store があれば、確認の行を出して true を返す。
  async function needsConfirm(s: AccountFormState): Promise<boolean> {
    if (!s.original || s.form.login === s.original.login) return false;
    const lines = await deps.accounts.editImpact(s.original.id, s.form.login);
    if (lines.length === 0) return false;
    patch({ confirm: lines });
    return true;
  }

  return {
    state: readonly(state),
    openAdd: () => open(null, { name: "", login: "", method: "device" }),
    openEdit: (a) => open(pick(a), { name: a.name, login: a.login, method: a.method }),
    update(p) {
      state.update((s) => (s ? { ...s, form: { ...s.form, ...p } } : s));
    },
    async submit() {
      const s = get(state);
      if (!s || busy) return;
      busy = true;
      const mine = opened;
      try {
        if (await needsConfirm(s)) return;
        await save(s, mine);
      } catch (e) {
        if (!(e instanceof NotFound)) throw e;
        if (mine === opened) close();
        deps.onSaved();
      } finally {
        busy = false;
      }
    },
    async confirm() {
      const s = get(state);
      if (!s?.confirm || busy) return;
      busy = true;
      try {
        await save(s, opened);
      } finally {
        busy = false;
      }
    },
    cancelConfirm: () => patch({ confirm: null }),
    back: (target, failure) =>
      open(target.original, { name: target.name, login: target.login, method: "device" }, failure),
    close,
  };
}
```

`core/src/index.ts` に足す:

```ts
export {
  type AccountForm,
  type AccountFormState,
  createAccountForm,
} from "./features/accounts/form";
```

- [ ] **Step 4: 通るのを見る**

Run: `cd web && pnpm test && pnpm lint && pnpm check && pnpm size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core
git commit -m "feat(web): アカウントの追加・編集と、変更の確認の状態を足す"
```

---

### Task 6: 「GitHub にサインイン」(問い合わせの流れ)

**Files:**
- Create: `web/packages/core/src/features/accounts/signIn.ts`・`signIn.test.ts`
- Modify: `web/packages/core/src/index.ts`

**Interfaces:**
- Consumes: Task 3 の型・口、Task 4 の `text.ts`・`fixtures.test-util.ts`。`Timer`・`Toasts`(`features/toasts/toasts.ts`)。
- Produces: `createSignIn(deps: SignInDeps): SignIn`、`SignInDialogState`・`SignInEnd`(下のコード)。

- [ ] **Step 1: 失敗するテストを書く**

`signIn.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type {
  Account,
  AccountRow,
  SignInFailure,
  SignInResult,
  SignInTarget,
} from "../../model/account";
import { NotFound, Refused } from "../../model/errors";
import { get } from "../../reactive/store";
import { code, fakeAccounts, fakeTimer, fakeToasts, flush, waiting } from "./fixtures.test-util";
import { createSignIn } from "./signIn";

function setup(rows: AccountRow[] = []) {
  const f = fakeAccounts(rows);
  const t = fakeTimer();
  const { toasts, texts } = fakeToasts();
  const back: { target: SignInTarget; failure: SignInFailure }[] = [];
  let done = 0;
  const signIn = createSignIn({
    accounts: f.port,
    toasts,
    timer: t.timer,
    back: (target, failure) => void back.push({ target, failure }),
    onDone: () => {
      done += 1;
    },
  });
  return { f, t, texts, back, signIn, done: () => done };
}

const add: SignInTarget = { purpose: "add", original: null, name: "work", login: "taro" };
const taro: Account = { id: "A1", name: "work", login: "taro", method: "device" };
const doneWith = (a: Account): SignInResult => ({ ...waiting, status: "done", account: a });
const mismatch = "このトークンは @jiro のものです。ログイン名を直してください";
const noKeychain =
  "このマシンでは OS のキーチェーンを使えません。gh CLI のトークンか環境変数を使ってください";
const unreachable = "GitHub に接続できません。接続を確かめて、もう一度お試しください";

async function tick(t: ReturnType<typeof fakeTimer>, ms: number) {
  t.advance(ms);
  await flush();
}

describe("createSignIn", () => {
  it("asks after the interval the server gives each time", async () => {
    const { f, t, signIn } = setup();
    signIn.begin(add, code("F1"));
    expect(get(signIn.state)).toMatchObject({ target: add, ended: "", error: "", retrying: false });
    await tick(t, 4999);
    expect(f.calls).toEqual([]);
    f.polls.push({ ...waiting, intervalSec: 10 });
    await tick(t, 1);
    expect(f.calls).toEqual(["pollSignIn F1"]);
    await tick(t, 5000);
    expect(f.calls).toEqual(["pollSignIn F1"]);
    await tick(t, 5000);
    expect(f.calls).toEqual(["pollSignIn F1", "pollSignIn F1"]);
  });

  it("asks at once when the code expires, and stops after it", async () => {
    const { f, t, signIn } = setup();
    signIn.begin(add, { ...code("F1"), expiresInSec: 3 });
    f.polls.push({ ...waiting, status: "expired" });
    await tick(t, 3000);
    expect(get(signIn.state)?.ended).toBe("expired");
    expect(f.calls).toEqual(["pollSignIn F1", "cancelSignIn F1"]);
    await tick(t, 60_000);
    expect(f.calls).toHaveLength(2);
  });

  it("closes and tells by purpose when GitHub allows it", async () => {
    const cases = [
      ["add", "@taro としてサインインしました"],
      ["edit", "アカウントを変更しました"],
      ["reauth", "work: @taro として再認証しました"],
    ] as const;
    for (const [purpose, want] of cases) {
      const { f, t, signIn, texts, done } = setup();
      signIn.begin({ ...add, purpose }, code("F1"));
      f.polls.push(doneWith(taro));
      await tick(t, 5000);
      expect(get(signIn.state)).toBeNull();
      expect(texts()).toEqual([want]);
      expect(done()).toBe(1);
    }
  });

  it("goes back to add or edit when another person allows it or the save fails", async () => {
    const { f, t, signIn, back } = setup();
    signIn.begin(add, code("F1"));
    f.polls.push({ ...waiting, status: "mismatch", message: mismatch });
    await tick(t, 5000);
    expect(get(signIn.state)).toBeNull();
    expect(back).toEqual([{ target: add, failure: { loginError: mismatch } }]);
    signIn.begin(add, code("F2"));
    f.polls.push({ ...waiting, status: "failed", message: noKeychain });
    await tick(t, 5000);
    expect(back[1]).toEqual({ target: add, failure: { error: noKeychain } });
  });

  it("keeps a failed reauth in its own dialog, and retries with a new code", async () => {
    const { f, t, signIn, back } = setup();
    const reauth: SignInTarget = { purpose: "reauth", original: taro, name: "work", login: "taro" };
    signIn.begin(reauth, code("F1"));
    f.polls.push({ ...waiting, status: "mismatch", message: mismatch });
    await tick(t, 5000);
    expect(get(signIn.state)).toMatchObject({ ended: "failed", error: mismatch });
    expect(back).toEqual([]);
    await signIn.retry();
    expect(f.calls).toContain("startSignIn A1 work taro");
    expect(get(signIn.state)).toMatchObject({ ended: "", error: "", code: { id: "S1" } });
  });

  it("goes back to add when a retry cannot get a code", async () => {
    const { f, t, signIn, back } = setup();
    signIn.begin(add, code("F1"));
    f.polls.push({ ...waiting, status: "denied" });
    await tick(t, 5000);
    expect(get(signIn.state)?.ended).toBe("denied");
    f.refuse.startSignIn = unreachable;
    await signIn.retry();
    expect(get(signIn.state)).toBeNull();
    expect(back).toEqual([{ target: add, failure: { error: unreachable } }]);
  });

  it("cancels the flow on close; a later answer that it was done only reloads the list", async () => {
    const { f, t, signIn, texts, done } = setup();
    signIn.begin(add, code("F1"));
    let answer: (r: SignInResult) => void = () => undefined;
    f.port.pollSignIn = (id) => {
      f.calls.push(`pollSignIn ${id}`);
      return new Promise((resolve) => {
        answer = resolve;
      });
    };
    await tick(t, 5000);
    signIn.close();
    expect(get(signIn.state)).toBeNull();
    expect(f.calls).toEqual(["pollSignIn F1", "cancelSignIn F1"]);
    answer(doneWith(taro));
    await flush();
    expect(texts()).toEqual([]);
    expect(done()).toBe(1);
    await tick(t, 60_000);
    expect(f.calls).toHaveLength(2);
  });

  it("keeps waiting when the server cannot be reached, and shows a gone flow as expired", async () => {
    const { f, t, signIn } = setup();
    signIn.begin(add, code("F1"));
    f.polls.push(new TypeError("Failed to fetch"), new NotFound("not found"));
    await tick(t, 5000);
    expect(get(signIn.state)?.ended).toBe("");
    await tick(t, 5000);
    expect(get(signIn.state)?.ended).toBe("expired");
  });

  it("treats a refusal of the question like a failed sign-in", async () => {
    const { f, t, signIn, back } = setup();
    signIn.begin(add, code("F1"));
    f.polls.push(new Refused(noKeychain));
    await tick(t, 5000);
    expect(back).toEqual([{ target: add, failure: { error: noKeychain } }]);
  });

  it("starts a reauth with the account, and lets the caller show a refusal", async () => {
    const { f, signIn } = setup();
    await signIn.reauth(taro);
    expect(f.calls).toEqual(["startSignIn A1 work taro"]);
    expect(get(signIn.state)?.target).toEqual({
      purpose: "reauth",
      original: taro,
      name: "work",
      login: "taro",
    });
    f.refuse.startSignIn = unreachable;
    await expect(signIn.reauth(taro)).rejects.toBeInstanceOf(Refused);
  });
});
```

- [ ] **Step 2: 落ちるのを見る**

Run: `cd web && pnpm --filter @hedgehug/core test -- signIn`
Expected: FAIL(`./signIn` がない)

- [ ] **Step 3: 実装する**

`web/packages/core/src/features/accounts/signIn.ts`:

```ts
import type {
  Account,
  SignInCode,
  SignInFailure,
  SignInResult,
  SignInStart,
  SignInTarget,
} from "../../model/account";
import { NotFound, Refused } from "../../model/errors";
import type { AccountsPort } from "../../port/accounts";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Timer, Toasts } from "../toasts/toasts";
import { changedMessage, reauthedMessage, signedInMessage } from "./text";

// 「GitHub にサインイン」(07「デバイスフロー」)。サーバーの流れに、サーバーが返す間隔ごとに 1 回ずつ問い合わせる
// (同時に 1 つ)。コードの期限が来たら、間隔を待たずに問い合わせる(D2 画面の計画の決定 S8〜S12)。

// SignInEnd は、待つのをやめた理由。failed は再認証の失敗(error に文言)。
export type SignInEnd = "" | "expired" | "denied" | "failed";

export interface SignInDialogState {
  target: SignInTarget;
  code: SignInCode;
  ended: SignInEnd;
  error: string;
  // retrying は、「再試行」で新しいコードを取っているあいだ。
  retrying: boolean;
}

export interface SignIn {
  state: Readable<SignInDialogState | null>;
  // begin は、取ったコードで開き、許可を待ち始める。
  begin(target: SignInTarget, code: SignInCode): void;
  // reauth は、デバイスフローの行の「再認証」。コードを取ってから開く。取れなければ Refused を投げる。
  reauth(account: Account): Promise<void>;
  retry(): Promise<void>;
  // close は、「キャンセル」・Esc・外のクリック・画面の移動。サーバーの流れも捨てる(何も登録しない)。
  close(): void;
}

export interface SignInDeps {
  accounts: AccountsPort;
  toasts: Toasts;
  timer: Timer;
  // back は、追加・編集のサインインが失敗で終わったとき、元のダイアログに戻して赤字を出す。
  back(target: SignInTarget, failure: SignInFailure): void;
  // onDone は、サインインが済んだあとに一覧を読み直す。
  onDone(): void;
}

const second = 1000;

const startOf = (t: SignInTarget): SignInStart => ({
  account: t.original?.id,
  name: t.name,
  login: t.login,
});

function doneMessage(target: SignInTarget, a: Account): string {
  if (target.purpose === "add") return signedInMessage(a.login);
  if (target.purpose === "edit") return changedMessage;
  return reauthedMessage(a);
}

export function createSignIn(deps: SignInDeps): SignIn {
  const state = writable<SignInDialogState | null>(null);
  // flow は、いま待っているサーバーの流れの ID。待っていなければ null。
  let flow: string | null = null;
  let next: unknown = null;
  let expiry: unknown = null;
  let inFlight = false;
  // again は、問い合わせのあいだに期限が来たとき、答えを受けたらすぐ問い合わせ直す印。
  let again = false;
  const patch = (p: Partial<SignInDialogState>) => state.update((s) => (s ? { ...s, ...p } : s));

  function stop() {
    if (next !== null) deps.timer.clear(next);
    if (expiry !== null) deps.timer.clear(expiry);
    next = null;
    expiry = null;
    again = false;
  }

  // drop は、流れを捨てる(問い合わせを止め、サーバーにも捨てさせる)。
  function drop() {
    const id = flow;
    flow = null;
    stop();
    if (id !== null) void deps.accounts.cancelSignIn(id).catch(() => undefined);
  }

  function wait(sec: number) {
    next = deps.timer.set(() => {
      next = null;
      void poll();
    }, sec * second);
  }

  function begin(target: SignInTarget, code: SignInCode) {
    drop();
    flow = code.id;
    state.set({ target, code, ended: "", error: "", retrying: false });
    wait(code.intervalSec);
    expiry = deps.timer.set(() => {
      expiry = null;
      void poll();
    }, code.expiresInSec * second);
  }

  function end(ended: SignInEnd, error = "") {
    drop();
    patch({ ended, error, retrying: false });
  }

  function fail(target: SignInTarget, failure: SignInFailure) {
    if (target.purpose === "reauth") {
      end("failed", failure.loginError ?? failure.error ?? "");
      return;
    }
    drop();
    state.set(null);
    deps.back(target, failure);
  }

  function finish(target: SignInTarget, account: Account | null) {
    flow = null;
    stop();
    state.set(null);
    const a: Account = account ?? {
      id: target.original?.id ?? "",
      name: target.name,
      login: target.login,
      method: "device",
    };
    deps.toasts.show(doneMessage(target, a));
    deps.onDone();
  }

  function answer(target: SignInTarget, r: SignInResult) {
    switch (r.status) {
      case "waiting":
        if (again) {
          again = false;
          void poll();
        } else wait(r.intervalSec);
        return;
      case "expired":
      case "denied":
        end(r.status);
        return;
      case "done":
        finish(target, r.account);
        return;
      case "mismatch":
        fail(target, { loginError: r.message });
        return;
      case "failed":
        fail(target, { error: r.message });
        return;
    }
  }

  // failed は、問い合わせ自体が失敗したとき。届かなければ待ち続け、流れがなければ期限切れ、断りは失敗と同じ(決定 S9)。
  function failed(s: SignInDialogState, e: unknown) {
    if (e instanceof NotFound) end("expired");
    else if (e instanceof Refused) fail(s.target, { error: e.message });
    else wait(s.code.intervalSec);
  }

  async function poll() {
    const id = flow;
    const s = get(state);
    if (id === null || s === null) return;
    if (inFlight) {
      again = true;
      return;
    }
    if (next !== null) {
      deps.timer.clear(next);
      next = null;
    }
    inFlight = true;
    let r: SignInResult;
    try {
      r = await deps.accounts.pollSignIn(id);
    } catch (e) {
      inFlight = false;
      if (id === flow) failed(s, e);
      return;
    }
    inFlight = false;
    if (id === flow) answer(s.target, r);
    // 閉じたあとに済んだと分かったら、一覧だけ読み直す(決定 S10)。
    else if (r.status === "done") deps.onDone();
  }

  return {
    state: readonly(state),
    begin,
    async reauth(account) {
      const target: SignInTarget = {
        purpose: "reauth",
        original: { id: account.id, name: account.name, login: account.login, method: account.method },
        name: account.name,
        login: account.login,
      };
      begin(target, await deps.accounts.startSignIn(startOf(target)));
    },
    async retry() {
      const s = get(state);
      if (!s || s.retrying) return;
      patch({ retrying: true, error: "" });
      try {
        const code = await deps.accounts.startSignIn(startOf(s.target));
        if (get(state) === null) {
          void deps.accounts.cancelSignIn(code.id).catch(() => undefined);
          return;
        }
        begin(s.target, code);
      } catch (e) {
        if (!(e instanceof Refused)) {
          patch({ retrying: false });
          throw e;
        }
        if (get(state) !== null) fail(s.target, { error: e.message });
      }
    },
    close() {
      drop();
      state.set(null);
    },
  };
}
```

`core/src/index.ts` に足す:

```ts
export {
  createSignIn,
  type SignIn,
  type SignInDialogState,
  type SignInEnd,
} from "./features/accounts/signIn";
```

- [ ] **Step 4: 通るのを見る**

Run: `cd web && pnpm test && pnpm lint && pnpm check && pnpm size`
Expected: PASS。Biome の認知的複雑度に当たったら、関数を分ける(上限は上げない)。

- [ ] **Step 5: コミット**

```bash
git add web/packages/core
git commit -m "feat(web): デバイスフローのサインインの問い合わせの流れを足す"
```

---

### Task 7: ui の確認ダイアログ

**Files:**
- Create: `web/packages/ui/src/patterns/ConfirmDialog.svelte`・`confirm.test.ts`
- Modify: `web/packages/ui/src/index.ts`

**Interfaces:**
- Produces: `ConfirmDialog`(props `title: string`・`lines: string[]`・`ok: string`・`danger?: boolean`・`onok: () => void`・`oncancel: () => void`)。

- [ ] **Step 1: 失敗するテストを書く**

`confirm.test.ts`:

```ts
import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import ConfirmDialog from "./ConfirmDialog.svelte";

describe("ConfirmDialog", () => {
  it("shows a paragraph per line and answers with the buttons and Escape", async () => {
    const onok = vi.fn();
    const oncancel = vi.fn();
    render(ConfirmDialog, {
      props: {
        title: "アカウントを削除",
        lines: ["「work」を削除します。", "チーム の同期に使っているため、それらは同期できなくなります。"],
        ok: "削除",
        danger: true,
        onok,
        oncancel,
      },
    });
    const dialog = screen.getByRole("dialog", { name: "アカウントを削除" });
    expect(dialog.querySelectorAll("p")).toHaveLength(2);
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    expect(onok).toHaveBeenCalledOnce();
    await fireEvent.click(screen.getByRole("button", { name: "キャンセル" }));
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(oncancel).toHaveBeenCalledTimes(2);
  });
});
```

- [ ] **Step 2: 落ちるのを見る**

Run: `cd web && pnpm --filter @hedgehug/ui test -- confirm`
Expected: FAIL(`./ConfirmDialog.svelte` がない)

- [ ] **Step 3: 実装する**

`ConfirmDialog.svelte`:

```svelte
<script lang="ts">
  import Button from "../primitives/Button.svelte";
  import Dialog from "../primitives/Dialog.svelte";

  // 確認のダイアログ。本文は 1 行を 1 段落にする。ok は実行のボタンの文言、danger なら警告色。
  interface Props {
    title: string;
    lines: string[];
    ok: string;
    danger?: boolean;
    onok: () => void;
    oncancel: () => void;
  }

  let { title, lines, ok, danger = false, onok, oncancel }: Props = $props();
</script>

<Dialog open {title} onclose={oncancel}>
  {#each lines as line, i (i)}
    <p class="line">{line}</p>
  {/each}
  {#snippet footer()}
    <span class="grow"></span>
    <Button onclick={oncancel}>キャンセル</Button>
    <Button variant={danger ? "danger" : "primary"} onclick={onok}>{ok}</Button>
  {/snippet}
</Dialog>

<style>
  .line {
    margin: 0;
  }
  .grow {
    flex: 1;
  }
</style>
```

`ui/src/index.ts` に `export { default as ConfirmDialog } from "./patterns/ConfirmDialog.svelte";` を足す(並びは patterns の先頭)。

- [ ] **Step 4: 通るのを見る**

Run: `cd web && pnpm --filter @hedgehug/ui test && pnpm lint && pnpm check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/ui
git commit -m "feat(web): 確認のダイアログの部品を足す"
```

---

### Task 8: 「GitHub アカウント」の画面

**Files:**
- Create: `web/app/src/screens/settings/AccountsSection.svelte`・`AccountItem.svelte`・`AccountDialog.svelte`・`SignInDialog.svelte`
- Modify: `web/app/src/screens/settings/SettingsScreen.svelte`
- Modify: `web/app/src/wire.ts`

**Interfaces:**
- Consumes: Task 3〜7 のすべて。`copiedMessage`・`copyFailedMessage`(`@hedgehug/core`、A2)。
- Produces: `wire.ts` の `accountList`・`accountForm`・`signIn`。

app の Svelte は単体テストを持たない(A2 と同じ)。このタスクの確かめは、型検査と、Task 2 の e2e が通り続けることと、Task 9 の e2e。

- [ ] **Step 1: 組み立てを足す**

`wire.ts`: api の import に `createAccountsPort`、core の import に `createAccountForm`・`createAccountList`・`createSignIn` を足す。`createToasts` の呼び出しを次に替える:

```ts
const timer = {
  set: (fn: () => void, ms: number) => setTimeout(fn, ms),
  clear: (handle: unknown) => clearTimeout(handle as ReturnType<typeof setTimeout>),
};
export const toasts = createToasts(timer);
```

`screen` の定義のあとに足す:

```ts
// 設定「アカウントとメンバー」の GitHub アカウント(07)。3 つは互いを呼ぶので、呼ぶのは作ったあと。
const accountsPort = createAccountsPort();
export const accountList = createAccountList({
  accounts: accountsPort,
  toasts,
  reauthDevice: (account) => signIn.reauth(account),
});
export const accountForm = createAccountForm({
  accounts: accountsPort,
  toasts,
  signIn: (target, code) => signIn.begin(target, code),
  onSaved: () => void accountList.reload(),
});
export const signIn = createSignIn({
  accounts: accountsPort,
  toasts,
  timer,
  back: (target, failure) => accountForm.back(target, failure),
  onDone: () => void accountList.reload(),
});
```

`navigation.route.subscribe` の中に足す:

```ts
  accountForm.close();
  signIn.close();
  accountList.cancelDelete();
```

- [ ] **Step 2: 行を書く**

`AccountItem.svelte`:

```svelte
<script lang="ts">
  import { type AccountRow, askingText, methodLine } from "@hedgehug/core";
  import { Button, IconButton } from "@hedgehug/ui";

  // 一覧の 1 行(07「状態」)。表示名・@ログイン名・認証方法・状態・「編集」・削除。赤字は状態の下に出す。
  interface Props {
    row: AccountRow;
    error: string;
    asking: boolean;
    onreauth: () => void;
    onedit: () => void;
    ondelete: () => void;
  }

  let { row, error, asking, onreauth, onedit, ondelete }: Props = $props();
</script>

<li class="row" aria-label={row.name}>
  <strong>{row.name}</strong>
  <span>@{row.login}</span>
  <span class="muted">{methodLine(row.method)}</span>
  <span class="meta">
    {#if row.status === "auth_error"}
      <span class="pill bad">認証エラー</span>
      <Button size="xs" disabled={asking} onclick={onreauth}>再認証</Button>
    {:else}
      <span class="pill ok">利用可能</span>
    {/if}
    {#if asking}
      <span class="muted">{askingText}</span>
    {/if}
    <Button size="xs" onclick={onedit}>編集</Button>
    <IconButton label="削除" size="xs" onclick={ondelete}>
      <svg width="12" height="12" viewBox="0 0 24 24" aria-hidden="true">
        <path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" />
      </svg>
    </IconButton>
  </span>
  {#if row.status === "auth_error" && row.reason}
    <p class="error">{row.reason}</p>
  {/if}
  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}
</li>

<style>
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 12px;
    padding: 10px 0;
    border-top: 1px solid var(--line);
  }
  .muted {
    color: var(--muted);
    font-size: 12.5px;
  }
  .meta {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-left: auto;
    flex-wrap: wrap;
  }
  .pill {
    border-radius: 999px;
    padding: 1px 8px;
    font-size: 12px;
  }
  .ok {
    background: var(--c-green-bg);
    color: var(--c-green-fg);
  }
  .bad {
    background: var(--c-red-bg);
    color: var(--c-red-fg);
  }
  .error {
    flex-basis: 100%;
    margin: 0;
    color: var(--danger);
    font-size: 12.5px;
  }
  svg {
    stroke: currentColor;
    fill: none;
    stroke-width: 1.8;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
</style>
```

- [ ] **Step 3: ダイアログを書く**

`AccountDialog.svelte`:

```svelte
<script lang="ts">
  import { type AuthMethod, askingText, methodChoices } from "@hedgehug/core";
  import { Button, ConfirmDialog, Dialog, Input, Select } from "@hedgehug/ui";
  import { accountForm } from "../../wire";

  // 「アカウントを追加」「アカウントを編集」(07「登録」「編集」)と「アカウントを変更」の確認。
  // 確認を出しているあいだは、編集のダイアログを隠す(D2 画面の計画の決定 S16)。
  const dialog = accountForm.state;

  function onkeydown(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) void accountForm.submit();
  }
</script>

{#if $dialog?.confirm}
  <ConfirmDialog
    title="アカウントを変更"
    lines={$dialog.confirm}
    ok="変更"
    danger
    onok={() => void accountForm.confirm()}
    oncancel={accountForm.cancelConfirm}
  />
{:else if $dialog}
  {@const s = $dialog}
  <Dialog open title={s.original ? "アカウントを編集" : "アカウントを追加"} onclose={accountForm.close}>
    <div class="form">
      <div class="field">
        <span>表示名</span>
        <Input
          label="表示名"
          placeholder="work"
          focusOnMount
          {onkeydown}
          bind:value={() => s.form.name, (v) => accountForm.update({ name: v })}
        />
      </div>
      <div class="field">
        <span>GitHub のログイン名</span>
        <Input
          label="GitHub のログイン名"
          {onkeydown}
          bind:value={() => s.form.login, (v) => accountForm.update({ login: v })}
        />
        {#if s.loginError}
          <p class="form-error" role="alert">{s.loginError}</p>
        {/if}
      </div>
      <div class="field">
        <span>認証方法</span>
        <Select
          label="認証方法"
          options={methodChoices}
          bind:value={
            () => s.form.method,
            (v) => accountForm.update({ method: v as AuthMethod })
          }
        />
      </div>
      {#if s.error}
        <p class="form-error" role="alert">{s.error}</p>
      {/if}
      {#if s.asking}
        <p class="muted">{askingText}</p>
      {/if}
    </div>
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={accountForm.close}>キャンセル</Button>
      <Button variant="primary" disabled={s.asking} onclick={() => void accountForm.submit()}>
        {s.original ? "保存" : "追加"}
      </Button>
    {/snippet}
  </Dialog>
{/if}

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .field > span {
    font-size: 12.5px;
    color: var(--muted);
  }
  .form-error {
    color: var(--danger);
    margin: 0;
    font-size: 13px;
  }
  .muted {
    color: var(--muted);
    margin: 0;
    font-size: 13px;
  }
  .grow {
    flex: 1;
  }
</style>
```

`SignInDialog.svelte`:

```svelte
<script lang="ts">
  import {
    askingText,
    copiedMessage,
    copyFailedMessage,
    deniedText,
    expiredText,
    waitingText,
  } from "@hedgehug/core";
  import { Button, Dialog } from "@hedgehug/ui";
  import { signIn, toasts } from "../../wire";

  // 「GitHub にサインイン」(07「デバイスフロー」)。許可を待つ問い合わせは core の signIn が行う。
  const dialog = signIn.state;

  // コピーの結果は、詳細パネルの ID のコピーと同じ文言(D2 画面の計画の決定 S17)。
  async function copy(code: string) {
    try {
      await navigator.clipboard.writeText(code);
      toasts.show(copiedMessage(code));
    } catch {
      toasts.show(copyFailedMessage(code), { error: true });
    }
  }
</script>

{#if $dialog}
  {@const s = $dialog}
  <Dialog open title="GitHub にサインイン" onclose={signIn.close}>
    <p class="line">このコードを GitHub で入力してください。</p>
    <div class="code-row">
      <code class="code">{s.code.userCode}</code>
      <Button size="sm" onclick={() => void copy(s.code.userCode)}>コピー</Button>
    </div>
    <p class="line">
      <a href={s.code.verificationUri} target="_blank" rel="noopener noreferrer"
        >{s.code.verificationUri}</a
      > を開き、コードを入力して許可します。
    </p>
    {#if s.retrying}
      <p class="muted">{askingText}</p>
    {:else if s.ended === "expired"}
      <p class="form-error" role="alert">{expiredText}</p>
    {:else if s.ended === "denied"}
      <p class="form-error" role="alert">{deniedText}</p>
    {:else if s.ended === "failed"}
      <p class="form-error" role="alert">{s.error}</p>
    {:else}
      <p class="muted">{waitingText}</p>
    {/if}
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={signIn.close}>キャンセル</Button>
      {#if s.ended}
        <Button variant="primary" disabled={s.retrying} onclick={() => void signIn.retry()}>
          再試行
        </Button>
      {/if}
    {/snippet}
  </Dialog>
{/if}

<style>
  .line {
    margin: 0;
  }
  .code-row {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .code {
    font-family: var(--font-mono);
    font-size: 22px;
    letter-spacing: 0.08em;
  }
  .form-error {
    color: var(--danger);
    margin: 0;
    font-size: 13px;
  }
  .muted {
    color: var(--muted);
    margin: 0;
    font-size: 13px;
  }
  .grow {
    flex: 1;
  }
</style>
```

- [ ] **Step 4: 区画を書いて、設定画面に入れる**

`AccountsSection.svelte`:

```svelte
<script lang="ts">
  import { Button, ConfirmDialog } from "@hedgehug/ui";
  import { onMount } from "svelte";
  import { accountForm, accountList } from "../../wire";
  import AccountDialog from "./AccountDialog.svelte";
  import AccountItem from "./AccountItem.svelte";
  import SignInDialog from "./SignInDialog.svelte";

  // 「GitHub アカウント」の区画(07「アカウント」)。開くたびに行の赤字を消して読み直す(決定 S14)。
  const list = accountList.state;
  onMount(() => {
    void accountList.open();
  });
</script>

<section class="card" aria-labelledby="accounts-title">
  <div class="head">
    <h2 id="accounts-title">GitHub アカウント</h2>
    <span class="grow"></span>
    <Button size="sm" onclick={accountForm.openAdd}>追加</Button>
  </div>
  <ul class="rows" aria-label="GitHub アカウント">
    {#each $list.rows as row (row.id)}
      <AccountItem
        {row}
        error={$list.errors[row.id] ?? ""}
        asking={$list.asking.includes(row.id)}
        onreauth={() => void accountList.reauth(row.id)}
        onedit={() => {
          accountList.touch(row.id);
          accountForm.openEdit(row);
        }}
        ondelete={() => void accountList.askDelete(row.id)}
      />
    {/each}
  </ul>
</section>
<AccountDialog />
<SignInDialog />
{#if $list.deleting}
  <ConfirmDialog
    title="アカウントを削除"
    lines={$list.deleting.lines}
    ok="削除"
    danger
    onok={() => void accountList.confirmDelete()}
    oncancel={accountList.cancelDelete}
  />
{/if}

<style>
  .card {
    background: var(--surface);
    border: 1px solid var(--line);
    border-radius: 12px;
    padding: 14px 16px 6px;
    max-width: 860px;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
  }
  .head h2 {
    margin: 0;
    font-size: 15px;
  }
  .grow {
    flex: 1;
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
  }
</style>
```

`SettingsScreen.svelte`: `import AccountsSection from "./AccountsSection.svelte";` を足し、`<div class="content"></div>` を次に替える:

```svelte
<div class="content">
  {#if tab === "accounts"}
    <AccountsSection />
  {/if}
</div>
```

- [ ] **Step 5: 通るのを見る**

Run: `cd web && pnpm lint && pnpm check && pnpm test && pnpm deps && pnpm size && pnpm build && cd .. && make e2e`
Expected: PASS(既存の e2e と Task 2 の settings.spec.ts が通る)。手でも `make e2e-bin` のバイナリを起動し、設定 → 追加 → gh CLI のトークンで `gh-desktop-0-a` を登録できることを見る。

- [ ] **Step 6: コミット**

```bash
git add web/app
git commit -m "feat(web): 設定の GitHub アカウントの一覧・追加・編集・削除・再認証・サインインの画面を足す"
```

---

### Task 9: 通しのテスト(3 つの幅)

**Files:**
- Create: `web/e2e/accounts.spec.ts`

**Interfaces:**
- Consumes: Task 1 の `POST /e2e/device` とログイン名の種、Task 8 の画面。

- [ ] **Step 1: テストを書く**

`web/e2e/accounts.spec.ts`:

```ts
import { expect, type Page, type TestInfo, test } from "@playwright/test";
import { tokenUrl } from "./helpers";

// アカウントはマシンに 1 つの一覧で、3 つの幅が 1 つのサーバーを共有する。表示名とログイン名には幅とやり直しの
// 回数を入れる。gh CLI のトークンで確かめられるのは gh-{幅}-{回数}-a・-b だけ(D2 画面の計画の決定 S22)。
const tag = (info: TestInfo) => `${info.project.name}-${info.retry}`;

// answer は、e2e のサーバーに、デバイスフローのコードの答えを決めさせる(決定 S21)。
async function answer(page: Page, code: string, result: "grant" | "deny" | "expire", login = "") {
  const q = new URLSearchParams({ code, result, login });
  const res = await page.request.post(`/e2e/device?${q}`, {
    headers: { Authorization: "Bearer e2e-token" },
  });
  expect(res.status()).toBe(204);
}

async function openAdd(page: Page) {
  await page.goto(tokenUrl("/settings/accounts"));
  await page.getByRole("button", { name: "追加", exact: true }).click();
  return page.getByRole("dialog", { name: "アカウントを追加" });
}

const codePattern = /^\d{4}-\d{4}$/;

test("adds, edits and deletes an account that uses the gh CLI token", async ({ page }, info) => {
  const t = tag(info);
  const dialog = await openAdd(page);
  await dialog.getByRole("button", { name: "追加" }).click();
  await expect(dialog.getByText("表示名とログイン名を入力してください")).toBeVisible();
  await dialog.getByRole("textbox", { name: "表示名" }).fill(`gh ${t}`);
  await dialog.getByRole("textbox", { name: "GitHub のログイン名" }).fill(`gh-${t}-a`);
  await dialog.getByRole("combobox", { name: "認証方法" }).selectOption("gh");
  await dialog.getByRole("button", { name: "追加" }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText("アカウントを追加しました")).toBeVisible();
  const row = page.getByRole("listitem", { name: `gh ${t}` });
  await expect(row.getByText(`@gh-${t}-a`)).toBeVisible();
  await expect(row.getByText("github.com · gh CLI のトークン")).toBeVisible();
  await expect(row.getByText("利用可能")).toBeVisible();

  await row.getByRole("button", { name: "編集" }).click();
  const edit = page.getByRole("dialog", { name: "アカウントを編集" });
  await expect(edit.getByRole("textbox", { name: "表示名" })).toHaveValue(`gh ${t}`);
  await edit.getByRole("textbox", { name: "GitHub のログイン名" }).fill(`gh-${t}-b`);
  await edit.getByRole("button", { name: "保存" }).click();
  await expect(edit).toBeHidden();
  await expect(page.getByText("アカウントを変更しました")).toBeVisible();
  await expect(row.getByText(`@gh-${t}-b`)).toBeVisible();

  await row.getByRole("button", { name: "削除" }).click();
  const confirm = page.getByRole("dialog", { name: "アカウントを削除" });
  await expect(confirm.getByText(`「gh ${t}」を削除します。`)).toBeVisible();
  await confirm.getByRole("button", { name: "削除" }).click();
  await expect(page.getByText("アカウントを削除しました")).toBeVisible();
  await expect(row).toHaveCount(0);
});

test("signs in with the device flow, and goes back when another person allows it", async ({
  page,
}, info) => {
  const t = tag(info);
  const dialog = await openAdd(page);
  await dialog.getByRole("textbox", { name: "表示名" }).fill(`dev ${t}`);
  await dialog.getByRole("textbox", { name: "GitHub のログイン名" }).fill(`dev-${t}`);
  await expect(dialog.getByRole("combobox", { name: "認証方法" })).toHaveValue("device");
  await dialog.getByRole("button", { name: "追加" }).click();
  const signIn = page.getByRole("dialog", { name: "GitHub にサインイン" });
  await expect(signIn.getByText("このコードを GitHub で入力してください。")).toBeVisible();
  await expect(signIn.getByRole("link", { name: "https://github.com/login/device" })).toBeVisible();
  await expect(signIn.getByText("許可を待っています")).toBeVisible();

  await answer(page, (await signIn.getByText(codePattern).textContent()) ?? "", "grant", `other-${t}`);
  await expect(signIn).toBeHidden();
  await expect(
    dialog.getByText(`このトークンは @other-${t} のものです。ログイン名を直してください`),
  ).toBeVisible();

  await dialog.getByRole("button", { name: "追加" }).click();
  await answer(page, (await signIn.getByText(codePattern).textContent()) ?? "", "grant", `dev-${t}`);
  await expect(signIn).toBeHidden();
  await expect(page.getByText(`@dev-${t} としてサインインしました`)).toBeVisible();
  const row = page.getByRole("listitem", { name: `dev ${t}` });
  await expect(row.getByText("github.com · デバイスフローのトークン")).toBeVisible();
});

test("shows an expired or denied code with a retry, and cancelling adds nothing", async ({
  page,
}, info) => {
  const t = tag(info);
  const dialog = await openAdd(page);
  await dialog.getByRole("textbox", { name: "表示名" }).fill(`exp ${t}`);
  await dialog.getByRole("textbox", { name: "GitHub のログイン名" }).fill(`exp-${t}`);
  await dialog.getByRole("button", { name: "追加" }).click();
  const signIn = page.getByRole("dialog", { name: "GitHub にサインイン" });
  const code = signIn.getByText(codePattern);
  const first = (await code.textContent()) ?? "";
  await answer(page, first, "expire");
  await expect(signIn.getByText("コードの期限が切れました")).toBeVisible();
  await signIn.getByRole("button", { name: "再試行" }).click();
  await expect(code).not.toHaveText(first);
  await expect(signIn.getByText("許可を待っています")).toBeVisible();
  await answer(page, (await code.textContent()) ?? "", "deny");
  await expect(signIn.getByText("GitHub で許可が拒否されました")).toBeVisible();
  await signIn.getByRole("button", { name: "キャンセル" }).click();
  await expect(signIn).toBeHidden();
  await expect(page.getByRole("listitem", { name: `exp ${t}` })).toHaveCount(0);
});

test("shows an authentication error in the row, and a failed reauth under it", async ({ page }) => {
  // e2e のサーバーには認証エラーにする経路がないので、一覧と再認証の応答を差し替える(決定 S23)。
  await page.route("**/api/accounts", (route) =>
    route.request().method() === "GET"
      ? route.fulfill({
          json: {
            accounts: [
              {
                id: "AERR",
                name: "broken",
                login: "taro",
                method: "gh",
                status: "auth_error",
                reason: "このトークンは @jiro のものです。ログイン名を直してください",
              },
            ],
          },
        })
      : route.fallback(),
  );
  await page.route("**/api/accounts/AERR/reauth", (route) =>
    route.fulfill({
      status: 422,
      json: { detail: "gh CLI のトークンが見つかりません。端末で `gh auth login` を実行してください" },
    }),
  );
  await page.goto(tokenUrl("/settings/accounts"));
  const row = page.getByRole("listitem", { name: "broken" });
  await expect(row.getByText("認証エラー")).toBeVisible();
  await expect(
    row.getByText("このトークンは @jiro のものです。ログイン名を直してください"),
  ).toBeVisible();
  await row.getByRole("button", { name: "再認証" }).click();
  await expect(
    row.getByText("gh CLI のトークンが見つかりません。端末で `gh auth login` を実行してください"),
  ).toBeVisible();
});
```

- [ ] **Step 2: 走らせる**

Run: `make e2e`
Expected: すべての spec が 3 つの幅で PASS。落ちたら、テストを緩めずに原因(画面・core・サーバー)を直す。

- [ ] **Step 3: 全体を確かめる**

Run: `make check && go vet -tags e2e ./cmd/hedgehug && golangci-lint run --build-tags e2e ./cmd/... && go test -tags e2e ./cmd/hedgehug/`
Expected: PASS

- [ ] **Step 4: コミット**

```bash
git add web/e2e
git commit -m "test(e2e): GitHub アカウントの追加・編集・削除・サインイン・認証エラーを 3 つの幅で確かめる"
```

---

## D2 画面のあと

- D3: `Connector.TokenFor` を使うようになったら、認証エラーの行を e2e で本物のサーバーから出せる(決定 S23 の差し替えを外す)。
- D3・D4・C2 画面: 「GitHub を使わない場合」の無効の入口を、入口と一緒に作る(決定 S2)。
- D4: GitHub App 未導入の状態・帯・「再確認」と、認証エラーの自動の確かめ直し。行の「GitHub に問い合わせています」と赤字は、この計画の `accountList` の `asking`・`errors` を使う。
- メンバーの区画(08)は、メンバーの持ち場の計画で、この設定画面のタブに足す。
