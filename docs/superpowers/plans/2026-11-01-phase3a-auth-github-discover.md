# フェーズ 3a: 認証・GitHub API・Store の作成と発見 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 設定（hosts・default_store・url_rewrites・repo_accounts・token）、4種類の認証と `hh auth`、GitHub の REST クライアントと fakegh、git の通信の書き換えと認証、`store init` の remote の用意、`store add --account`、`store discover`、読み取り専用とメンバー一覧を作り、フェーズ3の 3a を終える。

**Architecture:** 設定は `internal/config` に足す。トークンは `internal/auth` が取り、`internal/github`（REST）と `internal/gitrepo`（git の通信の `Conn`）に渡す。fakegh は `internal/fakegh`（ハンドラ）と `cmd/fakegh`（実行ファイル）で、テストは httptest で動かす。読み取り専用とメンバーの状態は `.hedgehug/local/` の JSON で、同期（`internal/sync`）が5分おきに GitHub から取って更新する。CLI は `cmd/hedgehug` のコマンドを足し・変える。

**Tech Stack:** Go、go-git v5、cobra、go.yaml.in/yaml/v4、`github.com/zalando/go-keyring`（新規）、net/http（標準）

**Spec:** `docs/superpowers/specs/2026-11-01-phase3-stores-accounts-design.md`（実装前に必ず全体を読む。この plan は 3a の範囲だけを扱う。食い違えば spec が正。spec と違う挙動になりそうなら、実装を止めてコントローラーに報告する）

## Global Constraints

- 3a の範囲は spec の「設定」「認証」「GitHub API」「fakegh」「git の通信」「Store の作成と発見」（読み取り専用・メンバー一覧・同期での GitHub の情報の更新を含む）と、「開発環境」「テスト」の 3a の分。3b（`internal/mount`、統合ビュー、コンテキスト解決）と 3c（Store 間の移動、履歴の書き換え）は作らない。
- Hedgehug は git の実行ファイルを使わない（CI の test-without-git が通ること）。gh だけは `token: gh` のときに実行してよい（テストでは差し替える）。
- トークンは通信するときにだけ取る。ローカルで済むコマンドで gh も keychain も呼ばない。
- 認証情報は Store（git）に入れない。keychain はサービス `hedgehug`、キー `<host>/<login>`。
- config.yaml の Save は、知らないキーとコメントを残す（今と同じ）。
- `url_rewrites` は fetch・push・clone の直前にだけ適用する。Store の `.git/config`、config.yaml、表示は書き換え前の値。
- GitHub API のタイムアウトは1リクエスト30秒。ヘッダーは `Accept: application/vnd.github+json`、`X-GitHub-Api-Version: 2022-11-28`、`Authorization: Bearer <token>`。
- 同期で GitHub の情報を取る間隔は5分。取れないことは同期の失敗にせず警告にする。
- 足すコマンドはすべて `--json` に対応する。終了コードは 1: 実行の失敗、2: 使い方の誤り。
- 人に見せる文言は既存の CLI に合わせて英語で書く（spec の日本語の文面は意味を合わせて英語にする）。
- テストは `go test -race ./...` と `golangci-lint run ./...` を通す。コミットは Task ごと。コミットメッセージの末尾に `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`（実装したモデルの名前）を付ける。

## spec にない細部の判断

spec が書いていない細部を、次のように埋めた（挙動は spec と変えていない）。

1. GitHub App の組み込みの client ID は `internal/auth` の変数 `builtinClientID`（いまは空）。`config.Host.ClientID` が空で host が github.com のときに使う。
2. `accounts[].token` は `config.TokenSource{Kind, Env}` で持つ。`Kind` は `""`・`gh`・`env`・`keychain`・`device`。
3. 機械全体のロック `auth.lock` は config.yaml と同じディレクトリに置き、`store.WithFileLock`（`acquire` を公開する薄い関数）で取る。待ち時間は30秒。
4. `device` のリフレッシュが失敗したとき（401・`bad_refresh_token`）は、`hh auth login <name>` を案内するエラーにする。
5. 読み取り専用の状態（`access.json`）とメンバー一覧（`members.json`）の読み書きは `internal/sync` に置く（spec のパッケージ構成では `internal/mount` だが、3a に mount はなく、同期が書くので sync に置く。3b の mount は sync の関数を呼ぶ）。
6. `members.json` に、author を集めた最後の HEAD（`head`）を足す。次の同期でそこからの差分だけを読むため。
7. ローカルのパスの origin の「書けるか」の判定は、GitHub の情報と同じ5分おきの時機で行う（ほかの時機は spec どおり add・discover の直後）。
8. 書き込みの拒否は、コミットを作る入口（`app.update` と `doctor --fix`）の最初で行う。`purge` は `purgeTargets` が「削除済みでなければ `not deleted`」を先に返すので、`rm.go` の `purge` では `a.open()` の直後（`purgeTargets` より前）で同じ関数を呼ぶ。
9. 読み取り専用の Store の同期では、マージと修復は行い、push だけをしない。push できないコミット数を `sync.Result.Unpushable` に入れ、`hh sync` と `sync --status` に出す。
10. origin への push が `ErrPushDenied` のときは、`access.json` に `push_denied` を書いてから、`org-a is read-only (the remote denied the push)` のエラーにする。ミラーへの push の拒否はミラーの警告（今の扱い）にする。
11. `hh sync`（明示の実行）は、理由が `push_denied` の判定を、同期の前に外す。
12. `hh auth status` は、トークンが取れない・`/user` が失敗した・login が合わない、のどれかがあれば終了コード1（spec は login の不一致だけを書くが、取れないのも確かめられない状態なので同じ扱いにした）。token のないアカウントは `-` と出して検査しない。
13. Device Flow でブラウザを開くかは、端末のときだけ `Open the browser? [Y/n]` で聞く。開く処理は `env.openURL`（macOS は `open`、Windows は `rundll32 url.dll,FileProtocolHandler`、それ以外は `xdg-open`）。失敗しても URL とコードは出ているので警告だけにする。
14. discover の提案名は、repo 名、使われていれば `<owner>-<repo>`、それも使われていれば `<owner>-<repo>-2`、`-3`…（spec のとおり）。どれも Store の名前の規則（`model.ValidName`）に合わないときは、英小文字にし、使えない文字を `-` にして作る。owner と repo から有効な名前が作れないとき（日本語だけなど）は `store-<番号>` で、番号は 1 から数えて、使われていない最初のもの（`store-1`、`store-2`…）。
15. `store discover --from` の bare リポジトリの判定は `HEAD` ファイル・`objects/`・`refs/` の3つがあること。見つけたら、その下へは入らない。
16. devenv の alice の2つ目のアカウント（alice-corp）は、`token: env:HH_DEV_TOKEN_WORK` にする（spec は `env:HH_DEV_TOKEN` だが、1つの変数では2つのアカウントのトークンを分けられないため）。`devenv env alice` が両方を export する。
17. devenv の fakegh の URL は `devenv init --fakegh <url>`（既定 `http://127.0.0.1:7777`）で変えられるようにする（E2E が httptest の URL を渡すため）。
18. HTTPS の git 通信に Basic 認証が付くことのテストは、spec の「httptest の smart HTTP サーバー」ではなく、テスト用に `https` の transport を差し替え、渡された `AuthMethod` を記録して確かめる（go-git に smart HTTP のサーバーがなく、git 本体なしでは立てられないため）。差し替えは `internal/gitrepo/gitrepotest` に置き、テストだけが使う。
19. fakegh の Device Flow の `interval` は seed の `device_interval`（秒、既定5）。テストは0にし、`auto_approve` で発行と同時に承認させる。
20. repo ID の正規化（discover でマウント済みを外すのに使う）は、3b で使う `internal/context/repoid.go` を 3a で先に作る（Task 4）。remote URL を読む処理はそこの `Split` の1つだけで、`store init`・読み取り専用の判定・メンバー一覧・discover が使う。`Split` は owner と repo の大文字小文字を保ち（GitHub に作るリポジトリの名前を変えないため）、`RepoID` は `Split` を小文字でつないだもの。
21. 読み取り専用の判定とメンバーの取得で「origin が GitHub のホストか」は、`url_rewrites` で書き換える前の URL で見る（spec の devenv の「bob の読み取り専用は fakegh の権限で再現する」は、書き換えた後がローカルのパスでも GitHub の権限を見ることを求めるため）。`store init` の分岐は spec どおり書き換えた後の URL で見る。どちらも、GitHub のホストかは URL の形（https・ssh://・`user@host:path`）ではなく `Split` が返す host で決め、`github.com` か `hosts[]` の host なら GitHub のホストとする。
22. `Split` の host はポートを含まない（spec が repo ID でポートを捨てると決めているため）。そのため `hosts[].host` もポートなしのホスト名で書く。ポート付きの host を `hosts[]` に書いても、remote の URL とは一致しない。（spec は `hosts[].host` にポートを書けるかを書いていない。）
23. 認証と GitHub API の失敗は `github.Describe(err, <host>/<login>, <アカウント名>, <していたこと>)` を通して、`<host>/<login> cannot <していたこと>: <原因>` の形にし、401 のときは末尾に `(run hedgehug auth login <アカウント名>)` を足す（spec の「エラー」）。`sync` は `config` を import しないので、`sync.GitHubRepo` にアカウントの ID と名前を文字列で持たせる。
24. `store init` でリポジトリの作成が 422 のときの「remote に ref が1つもないか」は、`gitrepo.RemoteEmpty` が git の通信（go-git の `Remote.List`。`url_rewrites` と認証は `Conn` を通す）で確かめる。fakegh は API だけに答えて git の通信をしないので、テストでは「GitHub 側にある」を fakegh の `State.Repos`（作成が 422 になる）に、「空」「中身あり」を `gitrepotest.InstallHTTPS` で同じ owner/name の bare に向けて置く。既存の空のリポジトリを使うときは、自分が作ったものではないので、失敗しても「GitHub に作った」とは言わない。
25. `store init` は、アカウントが決まれば（`--account`、またはアカウントが1つ）その名前を `stores[].account` に書く（spec の「選んだアカウントを `stores[].account` に書く」）。決まらなければ書かない。
26. config にあり `token` が空のアカウントに `hh auth login <name>` を実行したときは、Device Flow（`--with-token` なら標準入力のトークンを keychain へ）でトークンを取り、`GET /user` の login が config と合えば `token: device`（または `keychain`）を書く。合わなければ何も書かずにエラー（spec の「`hh auth`」の表）。
27. コラボレーターが取れないとき（403 に限らず、失敗はすべて）は `members.json` の `collaborators` を `null` にする（以前の一覧は残さない）。403 は読み取りだけの人に起きるのがふつうなので警告にしない。ほかの失敗は警告にする。
28. `Options.keyring()` は `Options.Keychain()` として公開し、`hh auth` もそこから keychain を得る（「nil なら OS の keychain」の分岐は1つ）。HEAD にあって origin にないコミットの数え方は `sync.Unpushed` の1つ。読み取り専用の bare を作るテスト用の chmod は `gitrepotest.LockObjects` の1つ。
29. 3a の E2E の `auth login` は、バイナリではなくこのプロセスの中で `run` を呼んで動かす（バイナリは本物の OS の keychain に書くので、keychain のない CI では通らず、spec は keychain の代わりのファイル保存を作らないため）。`store init` の GitHub 分岐の E2E は、`hosts` に `127.0.0.1` を GitHub のホストとして登録し、remote を何も待ち受けていない `http://127.0.0.1:1/...` にして、fakegh への作成と topic、残したリポジトリの案内までを確かめる（push は本物の `github.com` へ行かせない）。

---

## ファイル構成

```
internal/config/
  config.go          TokenSource、Host、URLRewrite、RepoAccount、DefaultStore を読む・書く
  hosts.go           新規。Host の既定値、IsGitHubHost、Rewrite、Unrewrite
internal/store/
  lock.go            WithFileLock を足す
internal/auth/       新規
  source.go          Token、ClientID、Key、Options（Keychain を含む）
  keychain.go        Keyring と OS の実装
  device.go          Login（Device Flow）、リフレッシュ
internal/github/     新規
  client.go          Client、エラー、Describe、ページ
  repos.go           User、CreateRepo、SetTopics、UserRepos、Repo、Collaborators
  device.go          RequestDeviceCode、PollToken、RefreshToken
internal/fakegh/     新規
  state.go           State、Repo、Load
  server.go          Server（GitHub の部分）
  admin.go           /_fakegh/ と Client（操作用）
cmd/fakegh/          新規
  main.go            serve、perm、repo、device、token
internal/gitrepo/
  conn.go            新規。Conn、SetConn、CloneWith、ErrPushDenied、LocalPath、IsHTTP、Writable、HeadHasFile、RemoteEmpty
  remote.go          Fetch・Push・Clone が Conn を使う
  gitrepotest/       新規。テスト用の https transport（https.go）と、読み取り専用の bare（lock.go の LockObjects）
internal/context/    新規（3a は repoid.go だけ。Task 4 で作る）
  repoid.go          Split（remote URL を読む唯一の関数）、RepoID
internal/sync/
  access.go          新規。Access、ReadAccess、WriteAccess
  members.go         新規。Members、ReadMembers、WriteMembers、UpdateAuthors
  github.go          新規。GitHubRepo、CheckAccess、refreshInfo
  sync.go            Conn・GitHub・Explicit、読み取り専用での同期、ErrPushDenied、Unpushed
cmd/hedgehug/
  auth.go            新規。auth login/status/logout
  conn.go            新規。connFor、authOptions、tokenFor
  github.go          新規。githubFor、pickAccount、accountNames
  store_default.go   新規
  store_discover.go  新規
  store_init.go      remote の用意、--account、--path
  store_add.go       --account、読み取り専用の判定、storeAdd の引数
  readonly.go        新規。checkWritable、readOnlyError
  app.go             update が checkWritable を呼ぶ、syncOptions が Conn・GitHub を入れる
  sync.go            Explicit、Unpushable の表示、status の read-only、unpushed が sync.Unpushed を呼ぶ
  doctor.go          --fix の前に checkWritable
  rm.go              purge の a.open() の直後に checkWritable
  main.go            env.openURL
tools/devenv/main.go hosts、fakegh.json（org-a/web を含む4件）、url_rewrites、org-b、alice-corp、--fakegh、トークンの export
```

---

### Task 1: 設定の新しいキーと `hh store default`

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/config/hosts.go`
- Test: `internal/config/config_test.go`、`internal/config/hosts_test.go`（新規）
- Create: `cmd/hedgehug/store_default.go`
- Modify: `cmd/hedgehug/store_init.go`（`storeCmd` に `storeDefaultCmd` を足す）
- Test: `cmd/hedgehug/store_default_test.go`（新規）

**Interfaces:**
- Produces:
  - `type TokenSource struct{ Kind, Env string }`（`Kind`: `""`/`"gh"`/`"env"`/`"keychain"`/`"device"`。`Env` は `env` のときの変数名）。`func (t TokenSource) String() string`（`gh`、`env:VAR`、`keychain`、`device`、空なら `""`）
  - `Account` に `Token TokenSource` を足す
  - `type Host struct{ Host, API, Web, ClientID string }`
  - `type URLRewrite struct{ From, To string }`、`type RepoAccount struct{ Match, Account string }`
  - `Config` に `Hosts []Host`、`DefaultStore string`、`URLRewrites []URLRewrite`、`RepoAccounts []RepoAccount`
  - `func (c *Config) Host(name string) Host`（既定値を埋めたもの）
  - `func (c *Config) IsGitHubHost(name string) bool`
  - `func (c *Config) Rewrite(url string) string`、`func (c *Config) Unrewrite(path string) string`
  - `func (c *Config) Account(name string) (Account, bool)`
  - `func (c *Config) AddAccount(a Account) error`、`func (c *Config) SetAccountToken(name string, t TokenSource) error`
  - `func (c *Config) SetDefaultStore(name string) error`（`""` で消す）
  - `AddStore` は `DefaultStore` が空ならその Store を既定にする。`RemoveStore` は既定の Store を外したら既定を消す

- [ ] **Step 1: 読み込みの失敗するテストを書く**

`internal/config/config_test.go` に足す:

```go
func TestLoadPhase3Keys(t *testing.T) {
	p := testPaths(t)
	writeConfig(t, p, `accounts:
  - name: personal
    host: github.com
    login: alice
    token: gh
  - name: work
    host: github.com
    login: alice-corp
    token: env:WORK_TOKEN
  - name: plain
    host: ghe.example.com
    login: al
hosts:
  - host: ghe.example.com
    client_id: Iv23abc
  - host: github.com
    api: http://127.0.0.1:7777
    web: http://127.0.0.1:7777
default_store: me
url_rewrites:
  - {from: "https://github.com/", to: "/tmp/remotes/"}
repo_accounts:
  - {match: github.com/org-a/*, account: work}
  - {match: "*", account: personal}
stores:
  - name: me
    path: /tmp/me
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Accounts[0].Token; got != (TokenSource{Kind: "gh"}) {
		t.Errorf("personal token = %+v", got)
	}
	if got := c.Accounts[1].Token; got != (TokenSource{Kind: "env", Env: "WORK_TOKEN"}) {
		t.Errorf("work token = %+v", got)
	}
	if got := c.Accounts[2].Token; got != (TokenSource{}) {
		t.Errorf("plain token = %+v", got)
	}
	if c.DefaultStore != "me" {
		t.Errorf("default_store = %q", c.DefaultStore)
	}
	if len(c.URLRewrites) != 1 || c.URLRewrites[0] != (URLRewrite{From: "https://github.com/", To: "/tmp/remotes/"}) {
		t.Errorf("url_rewrites = %+v", c.URLRewrites)
	}
	if len(c.RepoAccounts) != 2 || c.RepoAccounts[0] != (RepoAccount{Match: "github.com/org-a/*", Account: "work"}) {
		t.Errorf("repo_accounts = %+v", c.RepoAccounts)
	}
	if h := c.Host("github.com"); h.API != "http://127.0.0.1:7777" || h.Web != "http://127.0.0.1:7777" {
		t.Errorf("github.com = %+v", h)
	}
	if h := c.Host("ghe.example.com"); h.API != "https://ghe.example.com/api/v3" || h.Web != "https://ghe.example.com" || h.ClientID != "Iv23abc" {
		t.Errorf("ghe = %+v", h)
	}
}

func TestLoadPhase3KeyErrors(t *testing.T) {
	for _, tc := range []struct{ name, yaml, want string }{
		{"bad token", "accounts:\n  - {name: a, host: h, login: l, token: password}\n", "token"},
		{"empty env", "accounts:\n  - {name: a, host: h, login: l, token: \"env:\"}\n", "token"},
		{"host missing", "hosts:\n  - {api: http://x}\n", "hosts: host is missing"},
		{"host twice", "hosts:\n  - {host: h}\n  - {host: h}\n", "listed twice"},
		{"bad api", "hosts:\n  - {host: h, api: \"ftp://x\"}\n", "api"},
		{"unknown default", "default_store: nope\n", "default_store"},
		{"rewrite from missing", "url_rewrites:\n  - {to: /x}\n", "url_rewrites"},
		{"repo account unknown", "repo_accounts:\n  - {match: \"*\", account: nope}\n", "repo_accounts"},
		{"bad glob", "accounts:\n  - {name: a, host: h, login: l}\nrepo_accounts:\n  - {match: \"[\", account: a}\n", "match"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testPaths(t)
			writeConfig(t, p, tc.yaml)
			_, err := Load(p)
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), p.ConfigFile) {
				t.Errorf("err = %v, want one with %q and the file", err, tc.want)
			}
		})
	}
}
```

`testPaths`・`writeConfig`・`cfgPaths` は `config_test.go` にすでにあるヘルパーなので、足さない（同名を足すと重複定義でコンパイルできない）。

- [ ] **Step 2: 書き込みと hosts の失敗するテストを書く**

`internal/config/config_test.go` に足す:

```go
func TestDefaultStoreFollowsAddAndRemove(t *testing.T) {
	p := testPaths(t)
	writeConfig(t, p, "# keep me\n")
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("me", filepath.Join(p.Home, "me"), nil); err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("org", filepath.Join(p.Home, "org"), nil); err != nil {
		t.Fatal(err)
	}
	if c.DefaultStore != "me" {
		t.Errorf("default = %q, want the first store", c.DefaultStore)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	c, err = Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.DefaultStore != "me" {
		t.Errorf("default after load = %q", c.DefaultStore)
	}
	if err := c.SetDefaultStore("org"); err != nil {
		t.Fatal(err)
	}
	if err := c.SetDefaultStore("nope"); err == nil {
		t.Error("SetDefaultStore(nope) = nil")
	}
	if err := c.RemoveStore("org"); err != nil {
		t.Fatal(err)
	}
	if c.DefaultStore != "" {
		t.Errorf("default after removing it = %q", c.DefaultStore)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p.ConfigFile)
	if strings.Contains(string(b), "default_store") || !strings.Contains(string(b), "# keep me") {
		t.Errorf("config.yaml = %s", b)
	}
}

func TestAddAccountAndToken(t *testing.T) {
	p := testPaths(t)
	writeConfig(t, p, "accounts:\n  - name: a\n    host: github.com\n    login: alice\n    note: keep\n")
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddAccount(Account{Name: "a", Host: "github.com", Login: "x"}); err == nil {
		t.Error("AddAccount with a name in use = nil")
	}
	if err := c.AddAccount(Account{Name: "work", Host: "github.com", Login: "alice-corp", Token: TokenSource{Kind: "device"}}); err != nil {
		t.Fatal(err)
	}
	if err := c.SetAccountToken("a", TokenSource{Kind: "keychain"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	c, err = Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := c.Account("a"); a.Token.Kind != "keychain" {
		t.Errorf("a = %+v", a)
	}
	if a, ok := c.Account("work"); !ok || a.Login != "alice-corp" || a.Token.Kind != "device" {
		t.Errorf("work = %+v %v", a, ok)
	}
	b, _ := os.ReadFile(p.ConfigFile)
	if !strings.Contains(string(b), "note: keep") {
		t.Errorf("an unknown key was lost: %s", b)
	}
}
```

`internal/config/hosts_test.go`:

```go
package config

import "testing"

func TestRewrite(t *testing.T) {
	c := &Config{URLRewrites: []URLRewrite{
		{From: "https://github.com/", To: "/r/"},
		{From: "https://github.com/org-a/", To: "/a/"},
	}}
	for in, want := range map[string]string{
		"https://github.com/kukv/x.git":  "/r/kukv/x.git",
		"https://github.com/org-a/y.git": "/a/y.git", // the longest from wins
		"https://gitlab.com/z.git":       "https://gitlab.com/z.git",
	} {
		if got := c.Rewrite(in); got != want {
			t.Errorf("Rewrite(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"/r/kukv/x.git": "https://github.com/kukv/x.git",
		"/a/y.git":      "https://github.com/org-a/y.git", // the longest to wins
		"/elsewhere/z":  "/elsewhere/z",
	} {
		if got := c.Unrewrite(in); got != want {
			t.Errorf("Unrewrite(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsGitHubHost(t *testing.T) {
	c := &Config{Hosts: []Host{{Host: "ghe.example.com"}}}
	if !c.IsGitHubHost("github.com") || !c.IsGitHubHost("ghe.example.com") || c.IsGitHubHost("gitlab.com") {
		t.Error("IsGitHubHost is wrong")
	}
	if h := c.Host("github.com"); h.API != "https://api.github.com" || h.Web != "https://github.com" {
		t.Errorf("github.com default = %+v", h)
	}
}
```

- [ ] **Step 3: テストが失敗することを確かめる**

Run: `go test ./internal/config/`
Expected: FAIL（`TokenSource`、`URLRewrite` などが未定義のコンパイルエラー）

- [ ] **Step 4: hosts.go を書く**

```go
package config

import (
	"net/url"
	"strings"
)

// Host is where a GitHub host serves its API and its web pages, and the
// client ID of the GitHub App for the device flow there.
type Host struct {
	Host     string
	API      string // the REST API base, without a trailing slash
	Web      string // the web base, for the device flow
	ClientID string // empty: the built-in one of internal/auth, on github.com
}

// URLRewrite points the URLs that start with From at To, right before a
// fetch, push or clone.
type URLRewrite struct{ From, To string }

// RepoAccount picks the account for the repositories whose repo ID matches
// Match (a path.Match pattern; "*" matches every ID). Phase 5 uses it.
type RepoAccount struct{ Match, Account string }

// Host returns the host name with its defaults filled in: the API at
// https://api.github.com for github.com and https://<host>/api/v3 elsewhere,
// the web at https://<host>.
func (c *Config) Host(name string) Host {
	h := Host{Host: name}
	for _, x := range c.Hosts {
		if x.Host == name {
			h = x
		}
	}
	if h.API == "" {
		h.API = "https://" + name + "/api/v3"
		if name == "github.com" {
			h.API = "https://api.github.com"
		}
	}
	if h.Web == "" {
		h.Web = "https://" + name
	}
	h.API = strings.TrimSuffix(h.API, "/")
	h.Web = strings.TrimSuffix(h.Web, "/")
	return h
}

// IsGitHubHost reports whether name is github.com or a host of hosts.
func (c *Config) IsGitHubHost(name string) bool {
	if name == "github.com" {
		return true
	}
	for _, x := range c.Hosts {
		if x.Host == name {
			return true
		}
	}
	return false
}

// Rewrite applies the url_rewrites rule with the longest From that url
// starts with; url is returned as it is when none does.
func (c *Config) Rewrite(u string) string {
	best := -1
	for i, r := range c.URLRewrites {
		if strings.HasPrefix(u, r.From) && (best < 0 || len(r.From) > len(c.URLRewrites[best].From)) {
			best = i
		}
	}
	if best < 0 {
		return u
	}
	return c.URLRewrites[best].To + strings.TrimPrefix(u, c.URLRewrites[best].From)
}

// Unrewrite turns a rewritten URL or path back: the rule with the longest
// To that p starts with gives its From. p is returned as it is when none does.
func (c *Config) Unrewrite(p string) string {
	best := -1
	for i, r := range c.URLRewrites {
		if strings.HasPrefix(p, r.To) && (best < 0 || len(r.To) > len(c.URLRewrites[best].To)) {
			best = i
		}
	}
	if best < 0 {
		return p
	}
	return c.URLRewrites[best].From + strings.TrimPrefix(p, c.URLRewrites[best].To)
}

// webURL reports whether s is an http or https URL with a host.
func webURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
```

- [ ] **Step 5: config.go の読み込みと書き込みを足す**

`TokenSource` と `Account.Token`、`Config` のフィールドを足す:

```go
// TokenSource is where the token of an account comes from: gh, an
// environment variable, the keychain, or the device flow. Kind is empty for
// an account that uses no token.
type TokenSource struct {
	Kind string // "", "gh", "env", "keychain" or "device"
	Env  string // the variable, for env
}

// String is the value of token: in config.yaml.
func (t TokenSource) String() string {
	if t.Kind == "env" {
		return "env:" + t.Env
	}
	return t.Kind
}

func parseTokenSource(s string) (TokenSource, bool) {
	switch s {
	case "gh", "keychain", "device":
		return TokenSource{Kind: s}, true
	}
	if v, ok := strings.CutPrefix(s, "env:"); ok && v != "" {
		return TokenSource{Kind: "env", Env: v}, true
	}
	return TokenSource{}, false
}
```

```go
type Account struct {
	Name  string
	Host  string
	Login string
	Token TokenSource
}
```

```go
type Config struct {
	Stores       []StoreEntry
	Accounts     []Account
	Hosts        []Host
	DefaultStore string
	URLRewrites  []URLRewrite
	RepoAccounts []RepoAccount
	Sync         SyncConfig
	// … the private fields as they are
}
```

`Load` で、`loadAccounts` の後に `loadHosts`、`loadRewrites` を、stores の読み込みの後に `loadRepoAccounts` と `default_store` の検査を呼ぶ（`repo_accounts` と `default_store` はアカウントと Store を参照するので後）。stores のループを抜けた後の `return c, nil` を次に置き換える:

```go
	if err := c.loadRepoAccounts(valueOf(root, "repo_accounts")); err != nil {
		return nil, err
	}
	if d := valueOf(root, "default_store"); d != nil && d.Tag != "!!null" {
		if !slices.ContainsFunc(c.Stores, func(s StoreEntry) bool { return s.Name == d.Value }) {
			return nil, c.errAt(d, fmt.Sprintf("default_store: no store is named %q", d.Value))
		}
		c.DefaultStore = d.Value
	}
	return c, nil
```

（`stores` が空で early return している箇所 `if seq == nil || seq.Tag == "!!null" { return c, nil }` も、この後半を通るように、stores の読み込みを `if seq != nil && seq.Tag != "!!null" { … }` で囲む形に直す。）

`loadAccounts` の各要素で、`login` の後に token を読む:

```go
		if v := valueOf(it, "token"); v != nil && v.Tag != "!!null" {
			ts, ok := parseTokenSource(v.Value)
			if !ok {
				return c.errAt(v, fmt.Sprintf("accounts: token %q: want gh, env:<VAR>, keychain or device", v.Value))
			}
			a.Token = ts
		}
```

新しい読み込みの関数:

```go
// loadHosts reads hosts: a host each, with an optional api, web and client_id.
func (c *Config) loadHosts(seq *yaml.Node) error {
	if seq == nil || seq.Tag == "!!null" {
		return nil
	}
	if seq.Kind != yaml.SequenceNode {
		return c.errAt(seq, "hosts: want a list")
	}
	for _, it := range seq.Content {
		if it.Kind != yaml.MappingNode {
			return c.errAt(it, "hosts: want a mapping with host")
		}
		var h Host
		for _, f := range []struct {
			key string
			dst *string
		}{{"host", &h.Host}, {"api", &h.API}, {"web", &h.Web}, {"client_id", &h.ClientID}} {
			if v := valueOf(it, f.key); v != nil && v.Tag != "!!null" {
				*f.dst = v.Value
			}
		}
		if h.Host == "" {
			return c.errAt(it, "hosts: host is missing")
		}
		for _, f := range []struct{ key, v string }{{"api", h.API}, {"web", h.Web}} {
			if f.v != "" && !webURL(f.v) {
				return c.errAt(it, fmt.Sprintf("hosts: %s %q: want an http or https URL", f.key, f.v))
			}
		}
		if slices.ContainsFunc(c.Hosts, func(x Host) bool { return x.Host == h.Host }) {
			return c.errAt(it, fmt.Sprintf("hosts: %q is listed twice", h.Host))
		}
		c.Hosts = append(c.Hosts, h)
	}
	return nil
}

// loadRewrites reads url_rewrites: a from and a to each.
func (c *Config) loadRewrites(seq *yaml.Node) error {
	if seq == nil || seq.Tag == "!!null" {
		return nil
	}
	if seq.Kind != yaml.SequenceNode {
		return c.errAt(seq, "url_rewrites: want a list")
	}
	for _, it := range seq.Content {
		from, to := valueOf(it, "from"), valueOf(it, "to")
		if it.Kind != yaml.MappingNode || from == nil || from.Value == "" || to == nil || to.Value == "" {
			return c.errAt(it, "url_rewrites: want a mapping with from and to")
		}
		c.URLRewrites = append(c.URLRewrites, URLRewrite{From: from.Value, To: to.Value})
	}
	return nil
}

// loadRepoAccounts reads repo_accounts: a match pattern and an account each.
func (c *Config) loadRepoAccounts(seq *yaml.Node) error {
	if seq == nil || seq.Tag == "!!null" {
		return nil
	}
	if seq.Kind != yaml.SequenceNode {
		return c.errAt(seq, "repo_accounts: want a list")
	}
	for _, it := range seq.Content {
		m, a := valueOf(it, "match"), valueOf(it, "account")
		if it.Kind != yaml.MappingNode || m == nil || m.Value == "" || a == nil || a.Value == "" {
			return c.errAt(it, "repo_accounts: want a mapping with match and account")
		}
		if _, err := path.Match(m.Value, ""); err != nil {
			return c.errAt(m, fmt.Sprintf("repo_accounts: match %q: %v", m.Value, err))
		}
		if !c.hasAccount(a.Value) {
			return c.errAt(a, fmt.Sprintf("repo_accounts: account %q is not in accounts", a.Value))
		}
		c.RepoAccounts = append(c.RepoAccounts, RepoAccount{Match: m.Value, Account: a.Value})
	}
	return nil
}
```

（`import` に `"path"` を足す。）

書き込みの関数:

```go
// Account returns the account named name.
func (c *Config) Account(name string) (Account, bool) {
	for _, a := range c.Accounts {
		if a.Name == name {
			return a, true
		}
	}
	return Account{}, false
}

// root returns the top mapping of the document, making one when the file
// is empty.
func (c *Config) root() *yaml.Node {
	if c.doc == nil {
		c.doc = &yaml.Node{Kind: yaml.DocumentNode}
	}
	if len(c.doc.Content) == 0 {
		c.doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	return c.doc.Content[0]
}

// setKey sets key of the mapping m to v, adding it at the end when missing.
func setKey(m *yaml.Node, key string, v *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = v
			return
		}
	}
	m.Content = append(m.Content, scalar(key), v)
}

// deleteKey removes key from the mapping m.
func deleteKey(m *yaml.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = slices.Delete(m.Content, i, i+2)
			return
		}
	}
}

// AddAccount adds an account to the end of accounts. Save writes it.
func (c *Config) AddAccount(a Account) error {
	if c.hasAccount(a.Name) {
		return fmt.Errorf("an account named %q is already set up", a.Name)
	}
	root := c.root()
	seq := valueOf(root, "accounts")
	if seq == nil || seq.Tag == "!!null" {
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		setKey(root, "accounts", seq)
	}
	item := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		scalar("name"), scalar(a.Name), scalar("host"), scalar(a.Host), scalar("login"), scalar(a.Login),
	}}
	if a.Token.Kind != "" {
		item.Content = append(item.Content, scalar("token"), scalar(a.Token.String()))
	}
	seq.Content = append(seq.Content, item)
	c.Accounts = append(c.Accounts, a)
	return nil
}

// SetAccountToken changes where the token of the account name comes from.
// Save writes it.
func (c *Config) SetAccountToken(name string, t TokenSource) error {
	i := slices.IndexFunc(c.Accounts, func(a Account) bool { return a.Name == name })
	if i < 0 {
		return fmt.Errorf("no account is named %q", name)
	}
	for _, it := range valueOf(c.root(), "accounts").Content {
		if n := valueOf(it, "name"); n != nil && n.Value == name {
			if t.Kind == "" {
				deleteKey(it, "token")
			} else {
				setKey(it, "token", scalar(t.String()))
			}
		}
	}
	c.Accounts[i].Token = t
	return nil
}

// SetDefaultStore makes name the default store; "" removes the default.
// Save writes it.
func (c *Config) SetDefaultStore(name string) error {
	if name == "" {
		if c.doc != nil && len(c.doc.Content) > 0 {
			deleteKey(c.doc.Content[0], "default_store")
		}
		c.DefaultStore = ""
		return nil
	}
	if !slices.ContainsFunc(c.Stores, func(s StoreEntry) bool { return s.Name == name }) {
		return fmt.Errorf("no store is named %q", name)
	}
	setKey(c.root(), "default_store", scalar(name))
	c.DefaultStore = name
	return nil
}
```

`AddStore` の最初の `c.doc` を作る部分を `root := c.root()` に置き換え、末尾（`c.Stores = append(…)` の後）に:

```go
	if c.DefaultStore == "" {
		return c.SetDefaultStore(name)
	}
	return nil
```

`RemoveStore` の末尾（`c.Stores = slices.Delete(…)` の後）に:

```go
	if c.DefaultStore == name {
		return c.SetDefaultStore("")
	}
	return nil
```

`Config` の先頭のコメントを「Only stores, accounts, hosts, default_store, url_rewrites, repo_accounts and sync are read; …」に直す。`loadAccounts` のコメントの「(a token, say)」を消す（token は読むようになった）。

- [ ] **Step 6: 既存の config テストを新しい動きに合わせて直す**

`AddStore` は `default_store` がなければその Store を書く（`AddStore` の結果は末尾に `default_store: <name>` の1行が付く）ので、`AddStore` の結果の config.yaml を完全一致で比べる既存のテストが変わる。また、`token:` に `gh`・`env:<VAR>`・`keychain`・`device` 以外を書いた既存のテストは、新しい検査で読み込みのエラーになる。`internal/config/config_test.go` を次のとおり直す（`strings.Contains` や読み直しで比べるほかのテストは、そのままで通る）。

1. `TestSaveKeepsSequenceIndent` の表（`addAndRead` が `AddStore("b", …)` を呼ぶ6ケース）を、どの `want` も末尾に `default_store: b\n` を足した形にする。

```go
		{"indented", "stores:\n  - name: a\n    path: /x\n", "stores:\n  - name: a\n    path: /x\n  - name: b\n    path: /elsewhere/b\ndefault_store: b\n"},
		{"flush", "stores:\n- name: a\n  path: /x\n", "stores:\n- name: a\n  path: /x\n- name: b\n  path: /elsewhere/b\ndefault_store: b\n"},
		{"no file", "", "stores:\n  - name: b\n    path: /elsewhere/b\ndefault_store: b\n"},
		{"other list flush, no stores", "other:\n- x\n", "other:\n- x\nstores:\n- name: b\n  path: /elsewhere/b\ndefault_store: b\n"},
		{"other list indented, no stores", "other:\n  - x\n", "other:\n  - x\nstores:\n  - name: b\n    path: /elsewhere/b\ndefault_store: b\n"},
		{"empty stores", "stores:\n", "stores:\n  - name: b\n    path: /elsewhere/b\ndefault_store: b\n"},
```

2. `TestAddAndRemoveStoreKeepsComments` の `want` を次にする（`b` を足したとき既定の Store がなかったので `b` が既定になる。`a` を外しても既定は変わらない）:

```go
	want := "# mine\nstores:\n  - name: c\n    path: /c # third\n  - name: b\n    path: /b\n    remotes: [/r/b.git]\nlog_level: debug # keep\ndefault_store: b\n"
```

3. `TestLoadAccountsRemotesSync` の設定の `    token: secret` を `    token: gh` にする（`secret` は新しい検査で読み込みのエラーになる）。

- [ ] **Step 7: config のテストが通ることを確かめる**

Run: `go test ./internal/config/`
Expected: PASS

- [ ] **Step 8: `hh store default` の失敗するテストを書く**

`cmd/hedgehug/store_default_test.go`:

```go
package main

import (
	"strings"
	"testing"
)

func TestStoreDefault(t *testing.T) {
	te := newTestEnv(t)
	if out := te.ok("store", "default"); strings.TrimSpace(out) != "-" {
		t.Errorf("no default: %q", out)
	}
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("store", "init", "org", "--prefix", "ORG")
	if out := te.ok("store", "default"); strings.TrimSpace(out) != "me" {
		t.Errorf("the first store is the default: %q", out)
	}
	te.ok("store", "default", "org")
	if out := te.ok("store", "default", "--json"); !strings.Contains(out, `"default": "org"`) {
		t.Errorf("json = %q", out)
	}
	if code, _, errs := te.run("store", "default", "nope"); code != 1 || !strings.Contains(errs, "me, org") {
		t.Errorf("unknown name: exit %d, %q", code, errs)
	}
	te.ok("store", "remove", "org")
	if out := te.ok("store", "default"); strings.TrimSpace(out) != "-" {
		t.Errorf("after removing the default: %q", out)
	}
}
```

- [ ] **Step 9: 失敗を確かめる**

Run: `go test ./cmd/hedgehug/ -run TestStoreDefault`
Expected: FAIL（`unknown command "default"`）

- [ ] **Step 10: store_default.go を書く**

```go
package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func (a *app) storeDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "default [<name>]",
		Short: "Show or set the default store, where hedgehug add puts a task when nothing else decides",
		Args:  asUsage(cobra.MaximumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			c, _, err := a.config()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				if err := c.SetDefaultStore(args[0]); err != nil {
					names := make([]string, len(c.Stores))
					for i, s := range c.Stores {
						names[i] = s.Name
					}
					return fmt.Errorf("%w (stores: %s)", err, strings.Join(names, ", "))
				}
				if err := c.Save(); err != nil {
					return err
				}
			}
			if a.jsonOut {
				var d *string
				if c.DefaultStore != "" {
					d = &c.DefaultStore
				}
				return a.printJSON(struct {
					Default *string `json:"default"`
				}{d})
			}
			name := c.DefaultStore
			if name == "" {
				name = "-"
			}
			_, err = fmt.Fprintln(a.stdout, name)
			return err
		},
	}
}
```

`storeCmd` の `cmd.AddCommand(…)` に `a.storeDefaultCmd()` を足す。

- [ ] **Step 11: テストを通す**

Run: `go test ./internal/config/ ./cmd/hedgehug/`
Expected: PASS（既存のゴールデンテストに `default_store` の行が出るものがあれば、出力が spec どおりかを確かめて更新する）

- [ ] **Step 12: コミット**

```bash
git add internal/config cmd/hedgehug/store_default.go cmd/hedgehug/store_default_test.go cmd/hedgehug/store_init.go
git commit -m "feat(config): token・hosts・default_store・url_rewrites・repo_accounts を読み書きし、hh store default を足す"
```

---

### Task 2: `internal/auth` のトークンの取得元と keychain

**Files:**
- Modify: `internal/store/lock.go`（`WithFileLock`）
- Test: `internal/store/lock_test.go`
- Create: `internal/auth/source.go`、`internal/auth/keychain.go`
- Test: `internal/auth/source_test.go`
- Modify: `go.mod`、`go.sum`（`github.com/zalando/go-keyring`）

**Interfaces:**
- Consumes: `config.Account`、`config.TokenSource`、`config.Host`（Task 1）
- Produces:
  - `func store.WithFileLock(path string, wait time.Duration, fn func() error) error`
  - `package auth`: `const Service = "hedgehug"`、`func Key(a config.Account) string`
  - `type Keyring interface { Get(service, user string) (string, error); Set(service, user, secret string) error; Delete(service, user string) error }`、`var ErrNotFound`、`func OSKeyring() Keyring`
  - `type Options struct { Getenv func(string) string; Run func(name string, args ...string) (stdout, stderr []byte, err error); Keyring Keyring; Now func() time.Time; Host config.Host; HTTP *http.Client; LockFile string; Sleep func(time.Duration) }`、`func (o Options) Keychain() Keyring`（`Keyring` が nil なら OS の keychain。コマンドも keychain をここから得る）
  - `func Token(a config.Account, opt Options) (string, error)`（`device` の分は Task 5 で足す。この Task では `device` は保存済みのアクセストークンをそのまま返す）
  - `func SaveToken(a config.Account, token string, k Keyring) error`、`func Delete(a config.Account, k Keyring) error`
  - `type DeviceToken struct{ AccessToken, RefreshToken string; ExpiresAt, RefreshExpiresAt time.Time }`（JSON は `access_token` など）、`func SaveDeviceToken(a config.Account, t DeviceToken, k Keyring) error`、`func LoadDeviceToken(a config.Account, k Keyring) (DeviceToken, error)`
  - `func ClientID(h config.Host) string`

- [ ] **Step 1: WithFileLock の失敗するテストを書く**

`internal/store/lock_test.go` に足す:

```go
func TestWithFileLock(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "auth.lock")
	inside := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = WithFileLock(p, time.Second, func() error {
			close(inside)
			<-release
			return nil
		})
	}()
	<-inside
	if err := WithFileLock(p, 100*time.Millisecond, func() error { return nil }); !errors.Is(err, ErrLocked) {
		t.Errorf("second lock = %v, want ErrLocked", err)
	}
	close(release)
	want := errors.New("from fn")
	if err := WithFileLock(p, time.Second, func() error { return want }); !errors.Is(err, want) {
		t.Errorf("err = %v, want fn's", err)
	}
}
```

- [ ] **Step 2: 失敗を確かめ、WithFileLock を書く**

Run: `go test ./internal/store/ -run TestWithFileLock` → FAIL（未定義）

`internal/store/lock.go` に足す:

```go
// WithFileLock runs fn holding the OS lock on the file path, made with its
// directory when missing, waiting up to wait for another holder. It is for
// locks outside a store, such as the one that keeps two processes from
// refreshing a token at once.
func WithFileLock(path string, wait time.Duration, fn func() error) (err error) {
	l, err := acquire(path, wait)
	if err != nil {
		return err
	}
	defer func() {
		if rerr := l.release(); err == nil {
			err = rerr
		}
	}()
	return fn()
}
```

Run: `go test ./internal/store/ -run TestWithFileLock` → PASS

- [ ] **Step 3: 依存を足す**

Run: `go get github.com/zalando/go-keyring@v0.2.6`
Expected: go.mod の require に `github.com/zalando/go-keyring v0.2.6` が入る

- [ ] **Step 4: auth の失敗するテストを書く**

`internal/auth/source_test.go`:

```go
package auth

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
)

// memKeyring is a Keyring in memory, safe for goroutines.
type memKeyring struct {
	mu sync.Mutex
	m  map[string]string
}

func newMem() *memKeyring { return &memKeyring{m: map[string]string{}} }

func (k *memKeyring) Get(service, user string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.m[service+"|"+user]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (k *memKeyring) Set(service, user, secret string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.m[service+"|"+user] = secret
	return nil
}

func (k *memKeyring) Delete(service, user string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.m[service+"|"+user]; !ok {
		return ErrNotFound
	}
	delete(k.m, service+"|"+user)
	return nil
}

func acct(kind, env string) config.Account {
	return config.Account{Name: "a", Host: "github.com", Login: "alice", Token: config.TokenSource{Kind: kind, Env: env}}
}

func TestTokenGh(t *testing.T) {
	var got []string
	opt := Options{Run: func(name string, args ...string) ([]byte, []byte, error) {
		got = append([]string{name}, args...)
		return []byte(" gho_abc \n"), nil, nil
	}}
	tok, err := Token(acct("gh", ""), opt)
	if err != nil || tok != "gho_abc" {
		t.Fatalf("Token = %q, %v", tok, err)
	}
	if strings.Join(got, " ") != "gh auth token --hostname github.com --user alice" {
		t.Errorf("ran %q", got)
	}
	opt.Run = func(string, ...string) ([]byte, []byte, error) {
		return nil, []byte("no oauth token"), errors.New("exit status 1")
	}
	if _, err := Token(acct("gh", ""), opt); err == nil || !strings.Contains(err.Error(), "gh auth login --hostname github.com") || !strings.Contains(err.Error(), "no oauth token") {
		t.Errorf("err = %v", err)
	}
}

func TestTokenEnv(t *testing.T) {
	opt := Options{Getenv: func(k string) string {
		if k == "HH_TOKEN" {
			return "t1"
		}
		return ""
	}}
	if tok, err := Token(acct("env", "HH_TOKEN"), opt); err != nil || tok != "t1" {
		t.Errorf("Token = %q, %v", tok, err)
	}
	if _, err := Token(acct("env", "NOPE"), opt); err == nil || !strings.Contains(err.Error(), "NOPE is not set") {
		t.Errorf("err = %v", err)
	}
}

func TestTokenKeychain(t *testing.T) {
	k := newMem()
	opt := Options{Keyring: k}
	if _, err := Token(acct("keychain", ""), opt); err == nil || !strings.Contains(err.Error(), "hedgehug auth login a --with-token") {
		t.Errorf("err = %v", err)
	}
	if err := SaveToken(acct("keychain", ""), "secret", k); err != nil {
		t.Fatal(err)
	}
	if k.m["hedgehug|github.com/alice"] != "secret" {
		t.Errorf("keyring = %v", k.m)
	}
	if tok, err := Token(acct("keychain", ""), opt); err != nil || tok != "secret" {
		t.Errorf("Token = %q, %v", tok, err)
	}
	if err := Delete(acct("keychain", ""), k); err != nil || len(k.m) != 0 {
		t.Errorf("Delete = %v, %v", err, k.m)
	}
}

func TestTokenNone(t *testing.T) {
	if _, err := Token(acct("", ""), Options{}); !errors.Is(err, ErrNoToken) {
		t.Errorf("err = %v, want ErrNoToken", err)
	}
}

func TestDeviceTokenRoundTrip(t *testing.T) {
	k := newMem()
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	dt := DeviceToken{AccessToken: "ghu_1", RefreshToken: "ghr_1", ExpiresAt: now.Add(8 * time.Hour), RefreshExpiresAt: now.Add(4000 * time.Hour)}
	if err := SaveDeviceToken(acct("device", ""), dt, k); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDeviceToken(acct("device", ""), k)
	if err != nil || got != dt {
		t.Errorf("Load = %+v, %v", got, err)
	}
}

func TestClientID(t *testing.T) {
	if got := ClientID(config.Host{Host: "ghe", ClientID: "Iv23x"}); got != "Iv23x" {
		t.Errorf("override = %q", got)
	}
	if got := ClientID(config.Host{Host: "ghe"}); got != "" {
		t.Errorf("ghe without one = %q", got)
	}
	old := builtinClientID
	builtinClientID = "Iv23builtin"
	defer func() { builtinClientID = old }()
	if got := ClientID(config.Host{Host: "github.com"}); got != "Iv23builtin" {
		t.Errorf("github.com = %q", got)
	}
}
```

- [ ] **Step 5: 失敗を確かめる**

Run: `go test ./internal/auth/`
Expected: FAIL（パッケージがない）

- [ ] **Step 6: keychain.go を書く**

```go
package auth

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// ErrNotFound means the keychain has nothing under the key.
var ErrNotFound = errors.New("not in the keychain")

// Keyring is the OS credential store, or a stand-in in tests.
type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, secret string) error
	Delete(service, user string) error
}

// OSKeyring is the credential store of the OS: the macOS Keychain, the
// Windows Credential Manager or the Secret Service.
func OSKeyring() Keyring { return osKeyring{} }

type osKeyring struct{}

func (osKeyring) Get(service, user string) (string, error) {
	v, err := keyring.Get(service, user)
	return v, wrapKeyring(err)
}

func (osKeyring) Set(service, user, secret string) error {
	return wrapKeyring(keyring.Set(service, user, secret))
}

func (osKeyring) Delete(service, user string) error {
	return wrapKeyring(keyring.Delete(service, user))
}

// wrapKeyring turns go-keyring's not found into ErrNotFound, and adds a hint
// to the other errors: a machine with no Secret Service (WSL, say) fails here.
func wrapKeyring(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, keyring.ErrNotFound):
		return ErrNotFound
	default:
		return fmt.Errorf("the keychain: %w (where there is no keychain, use token: gh or token: env:<VAR>)", err)
	}
}
```

- [ ] **Step 7: source.go を書く**

```go
// Package auth gets the token of an account: from gh, an environment
// variable, the keychain or the device flow of the Hedgehug GitHub App.
package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/config"
)

// Service is the keychain service the tokens are kept under.
const Service = "hedgehug"

// ErrNoToken means the account has no token: in config.yaml.
var ErrNoToken = errors.New("the account has no token")

// builtinClientID is the client ID of the Hedgehug GitHub App on github.com.
// It is set once the App is registered (see the appendix of the phase 3
// spec); a client ID is not a secret.
var builtinClientID = ""

// ClientID is the client ID of the device flow on the host h: the one of
// hosts, or the built-in one on github.com.
func ClientID(h config.Host) string {
	if h.ClientID != "" {
		return h.ClientID
	}
	if h.Host == "github.com" {
		return builtinClientID
	}
	return ""
}

// Options is what getting a token needs from outside.
type Options struct {
	Getenv   func(string) string
	Run      func(name string, args ...string) (stdout, stderr []byte, err error) // nil runs the command
	Keyring  Keyring                                                              // nil is the OS keychain
	Now      func() time.Time
	Host     config.Host  // of the account, for the device flow
	HTTP     *http.Client // nil is a client with a 30-second timeout
	LockFile string       // auth.lock; a refresh holds it
	Sleep    func(time.Duration)
}

// Keychain is the keyring to use: Keyring, or the OS keychain when it is nil.
// The commands get the keyring from here too, so the choice is made once.
func (o Options) Keychain() Keyring {
	if o.Keyring == nil {
		return OSKeyring()
	}
	return o.Keyring
}

func (o Options) now() time.Time {
	if o.Now == nil {
		return time.Now()
	}
	return o.Now()
}

func (o Options) run(name string, args ...string) ([]byte, []byte, error) {
	if o.Run != nil {
		return o.Run(name, args...)
	}
	var out, errb bytes.Buffer
	cmd := exec.Command(name, args...) //nolint:gosec // G204: name is gh, and args are the host and login of an account in config.yaml
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	return out.Bytes(), errb.Bytes(), err
}

// Key is the keychain key of the account: <host>/<login>.
func Key(a config.Account) string { return a.Host + "/" + a.Login }

// Token returns the token of the account a, from where its token: says.
func Token(a config.Account, opt Options) (string, error) {
	switch a.Token.Kind {
	case "":
		return "", ErrNoToken
	case "gh":
		out, errb, err := opt.run("gh", "auth", "token", "--hostname", a.Host, "--user", a.Login)
		if err != nil {
			return "", fmt.Errorf("gh auth token for %s: %w: %s (run gh auth login --hostname %s, or change token: of account %s)",
				Key(a), err, strings.TrimSpace(string(errb)), a.Host, a.Name)
		}
		return strings.TrimSpace(string(out)), nil
	case "env":
		v := ""
		if opt.Getenv != nil {
			v = opt.Getenv(a.Token.Env)
		}
		if v == "" {
			return "", fmt.Errorf("%s is not set (the token of account %s)", a.Token.Env, a.Name)
		}
		return v, nil
	case "keychain":
		v, err := opt.Keychain().Get(Service, Key(a))
		if errors.Is(err, ErrNotFound) {
			return "", fmt.Errorf("the keychain has no token of %s; run hedgehug auth login %s --with-token", Key(a), a.Name)
		}
		return v, err
	case "device":
		return deviceToken(a, opt)
	}
	return "", fmt.Errorf("account %s: unknown token source %q", a.Name, a.Token.Kind)
}

// SaveToken keeps token in the keychain as the token of a.
func SaveToken(a config.Account, token string, k Keyring) error {
	return k.Set(Service, Key(a), token)
}

// Delete removes the token of a from the keychain; none there is no error.
func Delete(a config.Account, k Keyring) error {
	if err := k.Delete(Service, Key(a)); err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	return nil
}

// DeviceToken is what the device flow leaves in the keychain.
type DeviceToken struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	ExpiresAt        time.Time `json:"expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

// SaveDeviceToken keeps t in the keychain as JSON.
func SaveDeviceToken(a config.Account, t DeviceToken, k Keyring) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return k.Set(Service, Key(a), string(b))
}

// LoadDeviceToken reads what SaveDeviceToken kept.
func LoadDeviceToken(a config.Account, k Keyring) (DeviceToken, error) {
	var t DeviceToken
	v, err := k.Get(Service, Key(a))
	if err != nil {
		return t, err
	}
	if err := json.Unmarshal([]byte(v), &t); err != nil {
		return t, fmt.Errorf("the keychain entry of %s: %w", Key(a), err)
	}
	return t, nil
}

// deviceToken returns the access token of a device login. Task 5 adds the
// refresh; until then the token is used as it is.
func deviceToken(a config.Account, opt Options) (string, error) {
	t, err := LoadDeviceToken(a, opt.Keychain())
	if errors.Is(err, ErrNotFound) {
		return "", fmt.Errorf("%s is not logged in; run hedgehug auth login %s", Key(a), a.Name)
	}
	if err != nil {
		return "", err
	}
	return t.AccessToken, nil
}
```

（`net/http` は Options の型で使う。）

- [ ] **Step 8: テストを通す**

Run: `go test ./internal/auth/ ./internal/store/`
Expected: PASS

- [ ] **Step 9: コミット**

```bash
git add go.mod go.sum internal/auth internal/store/lock.go internal/store/lock_test.go
git commit -m "feat(auth): gh・env・keychain からトークンを取り、keychain に保存する"
```

---

### Task 3: fakegh のハンドラ（`internal/fakegh`）

**Files:**
- Create: `internal/fakegh/state.go`、`internal/fakegh/server.go`、`internal/fakegh/admin.go`
- Test: `internal/fakegh/server_test.go`

**Interfaces:**
- Produces:
  - `type State struct { Users []string; Orgs map[string][]string; Tokens map[string]string; Repos []*Repo; DeviceInterval *int; AutoApprove string }`（JSON: `users`、`orgs`、`tokens`、`repos`、`device_interval`、`auto_approve`）
  - `type Repo struct { Owner, Name string; Private bool; Topics []string; Perms map[string]string }`（JSON: `owner`、`name`、`private`、`topics`、`perms`。perm は `read`/`write`/`admin`/`none`）
  - `func Load(path string) (State, error)`
  - `func New(st State) *Server`、`func (s *Server) ServeHTTP(http.ResponseWriter, *http.Request)`、`func (s *Server) Reset(st State)`、`func (s *Server) SetNow(func() time.Time)`
  - `func (s *Server) SetPerm(repo, login, perm string) error`、`AddRepo(repo string, topics []string, public bool) error`、`ApproveDevice(userCode, login string) error`、`AddToken(token, login string)`、`ExpireTokens(login string)`、`func (s *Server) Repo(full string) (*Repo, bool)`（コピーを返す）
  - 管理用の HTTP: `POST /_fakegh/perm`（`{"repo","login","perm"}`）、`POST /_fakegh/repo`（`{"repo","topics","public"}`）、`POST /_fakegh/device/approve`（`{"user_code","login"}`）、`POST /_fakegh/token`（`{"token","login"}`）、`POST /_fakegh/token/expire`（`{"login"}`）
  - `type Admin struct{ URL string; HTTP *http.Client }` と、同じ名前のメソッド（cmd/fakegh が使う）

GitHub の部分（spec の「GitHub API」の表と同じ）:

| エンドポイント | 応答 |
| --- | --- |
| `GET /user` | `{"login": …}` |
| `POST /user/repos`、`POST /orgs/{org}/repos` | 201 と repo の JSON。同じ名前があれば 422 `{"message":"Repository creation failed.","errors":[{"resource":"Repository","code":"custom","field":"name","message":"name already exists on this account"}]}`。org で呼び手が org のメンバーでなければ 403 |
| `PUT /repos/{o}/{r}/topics` | 呼び手が admin でなければ 403。`{"names":[…]}` |
| `GET /user/repos` | 呼び手が owner か perms に read 以上のある repo。`per_page`（既定30）と `page`（1から）でページに分け、次があれば `Link: <…&page=N>; rel="next"` |
| `GET /repos/{o}/{r}` | 見えなければ 404。`permissions` を付ける |
| `GET /repos/{o}/{r}/collaborators` | 呼び手が write 以上でなければ 403。owner（ユーザーのとき）と perms の read 以上の人を、`login` と `role_name` で。ページは上と同じ |
| `POST /login/device/code` | フォーム `client_id`。`{"device_code","user_code","verification_uri","expires_in":900,"interval":…}`。`auto_approve` があればその login で承認済みにする |
| `POST /login/oauth/access_token` | フォーム。`grant_type=urn:ietf:params:oauth:grant-type:device_code` なら `device_code` を見て、未承認は `{"error":"authorization_pending"}`、期限切れは `expired_token`、知らないコードは `bad_verification_code`。承認済みならトークンを発行する。`grant_type=refresh_token` なら `refresh_token` を見て、知らなければ `{"error":"bad_refresh_token"}`、知っていれば新しい組を発行し古いリフレッシュトークンを無効にする |

repo の JSON は `id`、`node_id`、`name`、`full_name`、`owner: {login}`、`private`、`html_url`、`clone_url`（`https://github.com/<o>/<r>.git`）、`topics`、`permissions: {admin, push, pull}`。発行するトークンはアクセストークン `ghu_<16進>`（8時間）とリフレッシュトークン `ghr_<16進>`（6か月）。アクセストークンの期限が切れていれば 401。

- [ ] **Step 1: 失敗するテストを書く**

`internal/fakegh/server_test.go`:

```go
package fakegh

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func seed() State {
	zero := 0
	return State{
		Users:  []string{"alice", "bob"},
		Orgs:   map[string][]string{"org-a": {"alice"}},
		Tokens: map[string]string{"t-alice": "alice", "t-bob": "bob"},
		Repos: []*Repo{
			{Owner: "org-a", Name: "hh-shared", Private: true, Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice": "admin", "bob": "read"}},
			{Owner: "alice", Name: "notes", Private: true},
		},
		DeviceInterval: &zero,
	}
}

func do(t *testing.T, srv *httptest.Server, method, path, token, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if strings.HasPrefix(body, "{") {
		req.Header.Set("Content-Type", "application/json")
	} else if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, string(b)
}

func TestUserAndRepos(t *testing.T) {
	srv := httptest.NewServer(New(seed()))
	defer srv.Close()
	if res, body := do(t, srv, "GET", "/user", "t-bob", ""); res.StatusCode != 200 || !strings.Contains(body, `"login":"bob"`) {
		t.Errorf("/user = %d %s", res.StatusCode, body)
	}
	if res, _ := do(t, srv, "GET", "/user", "nope", ""); res.StatusCode != 401 {
		t.Errorf("unknown token = %d", res.StatusCode)
	}
	res, body := do(t, srv, "GET", "/user/repos?per_page=100", "t-bob", "")
	var repos []map[string]any
	if err := json.Unmarshal([]byte(body), &repos); err != nil || res.StatusCode != 200 || len(repos) != 1 {
		t.Fatalf("bob's repos = %d %s", res.StatusCode, body)
	}
	if repos[0]["full_name"] != "org-a/hh-shared" || repos[0]["permissions"].(map[string]any)["push"] != false {
		t.Errorf("repo = %v", repos[0])
	}
	if res, _ := do(t, srv, "GET", "/repos/org-a/hh-shared/collaborators", "t-bob", ""); res.StatusCode != 403 {
		t.Errorf("collaborators for a reader = %d", res.StatusCode)
	}
	res, body = do(t, srv, "GET", "/repos/org-a/hh-shared/collaborators", "t-alice", "")
	if res.StatusCode != 200 || !strings.Contains(body, `"login":"bob"`) || !strings.Contains(body, `"role_name":"read"`) {
		t.Errorf("collaborators = %d %s", res.StatusCode, body)
	}
	if res, _ := do(t, srv, "GET", "/repos/alice/notes", "t-bob", ""); res.StatusCode != 404 {
		t.Errorf("a repo bob cannot see = %d", res.StatusCode)
	}
}

func TestPages(t *testing.T) {
	st := seed()
	for i := range 3 {
		st.Repos = append(st.Repos, &Repo{Owner: "alice", Name: "r" + string(rune('0'+i))})
	}
	srv := httptest.NewServer(New(st))
	defer srv.Close()
	res, _ := do(t, srv, "GET", "/user/repos?per_page=2", "t-alice", "")
	link := res.Header.Get("Link")
	if !strings.Contains(link, `rel="next"`) || !strings.Contains(link, "page=2") {
		t.Errorf("Link = %q", link)
	}
	res, _ = do(t, srv, "GET", "/user/repos?per_page=2&page=3", "t-alice", "")
	if res.Header.Get("Link") != "" {
		t.Errorf("the last page has a next link")
	}
}

func TestCreateRepoAndTopics(t *testing.T) {
	s := New(seed())
	srv := httptest.NewServer(s)
	defer srv.Close()
	res, body := do(t, srv, "POST", "/orgs/org-a/repos", "t-alice", `{"name":"hh-new","private":true}`)
	if res.StatusCode != 201 || !strings.Contains(body, `"clone_url":"https://github.com/org-a/hh-new.git"`) {
		t.Fatalf("create = %d %s", res.StatusCode, body)
	}
	if res, body := do(t, srv, "POST", "/orgs/org-a/repos", "t-alice", `{"name":"hh-new"}`); res.StatusCode != 422 || !strings.Contains(body, "name already exists") {
		t.Errorf("again = %d %s", res.StatusCode, body)
	}
	if res, _ := do(t, srv, "POST", "/orgs/org-a/repos", "t-bob", `{"name":"x"}`); res.StatusCode != 403 {
		t.Errorf("not a member = %d", res.StatusCode)
	}
	if res, _ := do(t, srv, "PUT", "/repos/org-a/hh-new/topics", "t-alice", `{"names":["hedgehug-store"]}`); res.StatusCode != 200 {
		t.Errorf("topics = %d", res.StatusCode)
	}
	if r, ok := s.Repo("org-a/hh-new"); !ok || len(r.Topics) != 1 || r.Perms["alice"] != "admin" {
		t.Errorf("repo = %+v", r)
	}
	if res, _ := do(t, srv, "POST", "/user/repos", "t-bob", `{"name":"own"}`); res.StatusCode != 201 {
		t.Errorf("user repo = %d", res.StatusCode)
	}
}

func TestDeviceFlow(t *testing.T) {
	s := New(seed())
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	s.SetNow(func() time.Time { return now })
	srv := httptest.NewServer(s)
	defer srv.Close()
	_, body := do(t, srv, "POST", "/login/device/code", "", "client_id=Iv23x")
	var dc struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
	}
	if err := json.Unmarshal([]byte(body), &dc); err != nil || dc.DeviceCode == "" {
		t.Fatalf("device code = %s", body)
	}
	poll := url.Values{"client_id": {"Iv23x"}, "device_code": {dc.DeviceCode}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}.Encode()
	if _, body := do(t, srv, "POST", "/login/oauth/access_token", "", poll); !strings.Contains(body, "authorization_pending") {
		t.Errorf("before approval = %s", body)
	}
	if err := s.ApproveDevice(dc.UserCode, "alice"); err != nil {
		t.Fatal(err)
	}
	_, body = do(t, srv, "POST", "/login/oauth/access_token", "", poll)
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal([]byte(body), &tok); err != nil || !strings.HasPrefix(tok.AccessToken, "ghu_") || tok.ExpiresIn != 28800 {
		t.Fatalf("token = %s", body)
	}
	if _, body := do(t, srv, "GET", "/user", tok.AccessToken, ""); !strings.Contains(body, `"alice"`) {
		t.Errorf("/user with the new token = %s", body)
	}
	now = now.Add(9 * time.Hour)
	if res, _ := do(t, srv, "GET", "/user", tok.AccessToken, ""); res.StatusCode != 401 {
		t.Errorf("expired token = %d", res.StatusCode)
	}
	refresh := url.Values{"client_id": {"Iv23x"}, "refresh_token": {tok.RefreshToken}, "grant_type": {"refresh_token"}}.Encode()
	if _, body := do(t, srv, "POST", "/login/oauth/access_token", "", refresh); !strings.Contains(body, "ghu_") {
		t.Errorf("refresh = %s", body)
	}
	if _, body := do(t, srv, "POST", "/login/oauth/access_token", "", refresh); !strings.Contains(body, "bad_refresh_token") {
		t.Errorf("a used refresh token = %s", body)
	}
}

func TestAdmin(t *testing.T) {
	s := New(seed())
	srv := httptest.NewServer(s)
	defer srv.Close()
	a := Admin{URL: srv.URL}
	if err := a.SetPerm("org-a/hh-shared", "bob", "write"); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Repo("org-a/hh-shared"); r.Perms["bob"] != "write" {
		t.Errorf("perm = %v", r.Perms)
	}
	if err := a.AddRepo("org-a/hh-x", []string{"hedgehug-store"}, false); err != nil {
		t.Fatal(err)
	}
	if err := a.AddToken("t-new", "bob"); err != nil {
		t.Fatal(err)
	}
	if res, _ := do(t, srv, "GET", "/user", "t-new", ""); res.StatusCode != 200 {
		t.Errorf("added token = %d", res.StatusCode)
	}
	if err := a.SetPerm("nope/x", "bob", "read"); err == nil {
		t.Error("perm on a missing repo = nil")
	}
}

// TestAdminDeviceApproveAndExpire goes through the two endpoints the other
// tests reach by calling the server's methods: /_fakegh/device/approve and
// /_fakegh/token/expire.
func TestAdminDeviceApproveAndExpire(t *testing.T) {
	s := New(seed())
	srv := httptest.NewServer(s)
	defer srv.Close()
	a := Admin{URL: srv.URL}
	_, body := do(t, srv, "POST", "/login/device/code", "", "client_id=Iv23x")
	var dc struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
	}
	if err := json.Unmarshal([]byte(body), &dc); err != nil || dc.UserCode == "" {
		t.Fatalf("device code = %s", body)
	}
	poll := url.Values{"client_id": {"Iv23x"}, "device_code": {dc.DeviceCode}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}.Encode()
	if _, body := do(t, srv, "POST", "/login/oauth/access_token", "", poll); !strings.Contains(body, "authorization_pending") {
		t.Errorf("before approval = %s", body)
	}
	if err := a.ApproveDevice("NOPE-0000", "alice"); err == nil {
		t.Error("approving a code nobody has = nil")
	}
	if err := a.ApproveDevice(dc.UserCode, "alice"); err != nil {
		t.Fatal(err)
	}
	_, body = do(t, srv, "POST", "/login/oauth/access_token", "", poll)
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal([]byte(body), &tok); err != nil || !strings.HasPrefix(tok.AccessToken, "ghu_") {
		t.Fatalf("token after approval = %s", body)
	}
	if res, _ := do(t, srv, "GET", "/user", tok.AccessToken, ""); res.StatusCode != 200 {
		t.Errorf("issued token = %d", res.StatusCode)
	}
	// Expiring bob's tokens leaves alice's alone; expiring alice's makes the
	// access token 401 while the refresh token still works.
	if err := a.ExpireTokens("bob"); err != nil {
		t.Fatal(err)
	}
	if res, _ := do(t, srv, "GET", "/user", tok.AccessToken, ""); res.StatusCode != 200 {
		t.Errorf("after expiring bob's tokens = %d", res.StatusCode)
	}
	if err := a.ExpireTokens("alice"); err != nil {
		t.Fatal(err)
	}
	if res, _ := do(t, srv, "GET", "/user", tok.AccessToken, ""); res.StatusCode != 401 {
		t.Errorf("after expiring alice's tokens = %d, want 401", res.StatusCode)
	}
	refresh := url.Values{"client_id": {"Iv23x"}, "refresh_token": {tok.RefreshToken}, "grant_type": {"refresh_token"}}.Encode()
	if _, body := do(t, srv, "POST", "/login/oauth/access_token", "", refresh); !strings.Contains(body, "ghu_") {
		t.Errorf("refresh after expiring = %s", body)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/fakegh/`
Expected: FAIL（パッケージがない）

- [ ] **Step 3: state.go を書く**

```go
// Package fakegh is a fake of the GitHub API that Hedgehug uses, for tests
// and the development environment. It keeps its state in memory and answers
// only the endpoints Hedgehug calls.
package fakegh

import (
	"encoding/json"
	"os"
)

// State is what the fake knows: users, organizations, tokens and
// repositories. A seed file is this as JSON.
type State struct {
	Users          []string            `json:"users"`
	Orgs           map[string][]string `json:"orgs"`   // org → members
	Tokens         map[string]string   `json:"tokens"` // token → login
	Repos          []*Repo             `json:"repos"`
	DeviceInterval *int                `json:"device_interval,omitempty"` // seconds; 5 when missing
	AutoApprove    string              `json:"auto_approve,omitempty"`    // approve each device code for this login at once
}

// Repo is a repository and who may do what with it. The owner of a user's
// repository is its admin.
type Repo struct {
	Owner   string            `json:"owner"`
	Name    string            `json:"name"`
	Private bool              `json:"private"`
	Topics  []string          `json:"topics"`
	Perms   map[string]string `json:"perms"` // login → read, write, admin or none
}

// Load reads a seed file.
func Load(path string) (State, error) {
	var st State
	b, err := os.ReadFile(path) //nolint:gosec // the seed the developer names
	if err != nil {
		return st, err
	}
	err = json.Unmarshal(b, &st)
	return st, err
}
```

- [ ] **Step 4: server.go を書く**

```go
package fakegh

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	accessLife  = 8 * time.Hour
	refreshLife = 4416 * time.Hour // about six months
)

type device struct {
	code, userCode, login string
	expires              time.Time
}

type issued struct {
	login   string
	expires time.Time
}

// Server is the fake. It is an http.Handler.
type Server struct {
	mu      sync.Mutex
	st      State
	now     func() time.Time
	devices map[string]*device // device code → device
	access  map[string]issued  // access tokens it issued
	refresh map[string]string  // refresh token → login
	nextID  int
}

// New makes a fake with the state st.
func New(st State) *Server {
	s := &Server{now: time.Now}
	s.Reset(st)
	return s
}

// Reset replaces the state and forgets the device codes and issued tokens.
func (s *Server) Reset(st State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st.Orgs == nil {
		st.Orgs = map[string][]string{}
	}
	if st.Tokens == nil {
		st.Tokens = map[string]string{}
	}
	for _, r := range st.Repos {
		if r.Perms == nil {
			r.Perms = map[string]string{}
		}
	}
	s.st = st
	s.devices, s.access, s.refresh = map[string]*device{}, map[string]issued{}, map[string]string{}
}

// SetNow replaces the clock.
func (s *Server) SetNow(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = now
}

// Repo returns a copy of the repository owner/name.
func (s *Server) Repo(full string) (*Repo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(full)
	if r == nil {
		return nil, false
	}
	c := *r
	c.Topics = slices.Clone(r.Topics)
	c.Perms = map[string]string{}
	for k, v := range r.Perms {
		c.Perms[k] = v
	}
	return &c, true
}

func (s *Server) find(full string) *Repo {
	for _, r := range s.st.Repos {
		if strings.EqualFold(r.Owner+"/"+r.Name, full) {
			return r
		}
	}
	return nil
}

func (s *Server) isUser(login string) bool { return slices.Contains(s.st.Users, login) }

// perm is what login may do with r: admin, write, read or "".
func perm(r *Repo, login string) string {
	if r.Owner == login {
		return "admin"
	}
	switch p := r.Perms[login]; p {
	case "read", "write", "admin":
		return p
	}
	return ""
}

func rank(p string) int { return map[string]int{"read": 1, "write": 2, "admin": 3}[p] }

func token() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"message": msg})
}

// caller is the login of the bearer token, or "".
func (s *Server) caller(r *http.Request) string {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		tok, _ = strings.CutPrefix(r.Header.Get("Authorization"), "token ")
	}
	if l, ok := s.st.Tokens[tok]; ok {
		return l
	}
	if is, ok := s.access[tok]; ok && s.now().Before(is.expires) {
		return is.login
	}
	return ""
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/_fakegh/") {
		s.admin(w, r)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case r.Method == "POST" && r.URL.Path == "/login/device/code":
		s.deviceCode(w, r)
		return
	case r.Method == "POST" && r.URL.Path == "/login/oauth/access_token":
		s.accessToken(w, r)
		return
	}
	login := s.caller(r)
	if login == "" {
		fail(w, http.StatusUnauthorized, "Bad credentials")
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case r.Method == "GET" && r.URL.Path == "/user":
		writeJSON(w, 200, map[string]string{"login": login})
	case r.Method == "GET" && r.URL.Path == "/user/repos":
		var out []*Repo
		for _, x := range s.st.Repos {
			if perm(x, login) != "" {
				out = append(out, x)
			}
		}
		s.page(w, r, len(out), func(i int) any { return s.repoJSON(out[i], login) })
	case r.Method == "POST" && r.URL.Path == "/user/repos":
		s.create(w, r, login, login)
	case r.Method == "POST" && len(parts) == 3 && parts[0] == "orgs" && parts[2] == "repos":
		if !slices.Contains(s.st.Orgs[parts[1]], login) {
			fail(w, http.StatusForbidden, "You need admin access to the organization before adding a repository to it.")
			return
		}
		s.create(w, r, parts[1], login)
	case len(parts) >= 3 && parts[0] == "repos":
		s.repoRoute(w, r, parts, login)
	default:
		fail(w, http.StatusNotFound, "Not Found")
	}
}

func (s *Server) repoRoute(w http.ResponseWriter, r *http.Request, parts []string, login string) {
	x := s.find(parts[1] + "/" + parts[2])
	if x == nil || perm(x, login) == "" {
		fail(w, http.StatusNotFound, "Not Found")
		return
	}
	switch {
	case r.Method == "GET" && len(parts) == 3:
		writeJSON(w, 200, s.repoJSON(x, login))
	case r.Method == "PUT" && len(parts) == 4 && parts[3] == "topics":
		if perm(x, login) != "admin" {
			fail(w, http.StatusForbidden, "Must have admin rights to Repository.")
			return
		}
		var body struct {
			Names []string `json:"names"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		x.Topics = body.Names
		writeJSON(w, 200, map[string][]string{"names": x.Topics})
	case r.Method == "GET" && len(parts) == 4 && parts[3] == "collaborators":
		if rank(perm(x, login)) < rank("write") {
			fail(w, http.StatusForbidden, "Must have push access to view repository collaborators.")
			return
		}
		type collab struct {
			Login    string `json:"login"`
			RoleName string `json:"role_name"`
		}
		var out []collab
		if s.isUser(x.Owner) {
			out = append(out, collab{x.Owner, "admin"})
		}
		names := make([]string, 0, len(x.Perms))
		for l := range x.Perms {
			names = append(names, l)
		}
		slices.Sort(names)
		for _, l := range names {
			if p := perm(x, l); p != "" && l != x.Owner {
				out = append(out, collab{l, p})
			}
		}
		s.page(w, r, len(out), func(i int) any { return out[i] })
	default:
		fail(w, http.StatusNotFound, "Not Found")
	}
}

// page writes items [from, to) of n as a JSON array, with a Link to the next
// page as GitHub does.
func (s *Server) page(w http.ResponseWriter, r *http.Request, n int, item func(int) any) {
	per, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if per <= 0 {
		per = 30
	}
	pg, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if pg <= 0 {
		pg = 1
	}
	from, to := min((pg-1)*per, n), min(pg*per, n)
	out := make([]any, 0, to-from)
	for i := from; i < to; i++ {
		out = append(out, item(i))
	}
	if to < n {
		q := r.URL.Query()
		q.Set("page", strconv.Itoa(pg+1))
		w.Header().Set("Link", fmt.Sprintf(`<http://%s%s?%s>; rel="next"`, r.Host, r.URL.Path, q.Encode()))
	}
	writeJSON(w, 200, out)
}

func (s *Server) repoJSON(x *Repo, login string) map[string]any {
	s.nextID++
	p := perm(x, login)
	topics := x.Topics
	if topics == nil {
		topics = []string{}
	}
	return map[string]any{
		"id": s.nextID, "node_id": fmt.Sprintf("R_%d", s.nextID),
		"name": x.Name, "full_name": x.Owner + "/" + x.Name,
		"owner":    map[string]string{"login": x.Owner},
		"private":  x.Private,
		"html_url": "https://github.com/" + x.Owner + "/" + x.Name,
		"clone_url": "https://github.com/" + x.Owner + "/" + x.Name + ".git",
		"topics":   topics,
		"permissions": map[string]bool{"admin": p == "admin", "push": rank(p) >= rank("write"), "pull": p != ""},
	}
}

func (s *Server) create(w http.ResponseWriter, r *http.Request, owner, login string) {
	var body struct {
		Name    string `json:"name"`
		Private bool   `json:"private"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		fail(w, http.StatusUnprocessableEntity, "name is missing")
		return
	}
	if s.find(owner+"/"+body.Name) != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"message": "Repository creation failed.",
			"errors":  []map[string]string{{"resource": "Repository", "code": "custom", "field": "name", "message": "name already exists on this account"}},
		})
		return
	}
	x := &Repo{Owner: owner, Name: body.Name, Private: body.Private, Perms: map[string]string{}}
	if owner != login {
		x.Perms[login] = "admin"
	}
	s.st.Repos = append(s.st.Repos, x)
	writeJSON(w, http.StatusCreated, s.repoJSON(x, login))
}

func (s *Server) deviceCode(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || r.PostForm.Get("client_id") == "" {
		writeJSON(w, 200, map[string]string{"error": "unauthorized_client"})
		return
	}
	d := &device{code: token(), userCode: strings.ToUpper(token()[:4] + "-" + token()[:4]), expires: s.now().Add(15 * time.Minute)}
	if s.st.AutoApprove != "" {
		d.login = s.st.AutoApprove
	}
	s.devices[d.code] = d
	interval := 5
	if s.st.DeviceInterval != nil {
		interval = *s.st.DeviceInterval
	}
	writeJSON(w, 200, map[string]any{
		"device_code": d.code, "user_code": d.userCode,
		"verification_uri": "http://" + r.Host + "/login/device",
		"expires_in":       900, "interval": interval,
	})
}

func (s *Server) issue(w http.ResponseWriter, login string) {
	at, rt := "ghu_"+token(), "ghr_"+token()
	s.access[at] = issued{login, s.now().Add(accessLife)}
	s.refresh[rt] = login
	writeJSON(w, 200, map[string]any{
		"access_token": at, "token_type": "bearer", "scope": "",
		"expires_in": int(accessLife.Seconds()), "refresh_token": rt, "refresh_token_expires_in": int(refreshLife.Seconds()),
	})
}

func (s *Server) accessToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, 200, map[string]string{"error": "invalid_request"})
		return
	}
	f := r.PostForm
	switch f.Get("grant_type") {
	case "urn:ietf:params:oauth:grant-type:device_code":
		d, ok := s.devices[f.Get("device_code")]
		switch {
		case !ok:
			writeJSON(w, 200, map[string]string{"error": "bad_verification_code"})
		case s.now().After(d.expires):
			writeJSON(w, 200, map[string]string{"error": "expired_token"})
		case d.login == "":
			writeJSON(w, 200, map[string]string{"error": "authorization_pending"})
		default:
			delete(s.devices, d.code)
			s.issue(w, d.login)
		}
	case "refresh_token":
		login, ok := s.refresh[f.Get("refresh_token")]
		if !ok {
			writeJSON(w, 200, map[string]string{"error": "bad_refresh_token"})
			return
		}
		delete(s.refresh, f.Get("refresh_token"))
		s.issue(w, login)
	default:
		writeJSON(w, 200, map[string]string{"error": "unsupported_grant_type"})
	}
}

// SetPerm sets what login may do with repo (owner/name); none takes it away.
func (s *Server) SetPerm(repo, login, p string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	x := s.find(repo)
	if x == nil {
		return fmt.Errorf("no repository %s", repo)
	}
	if !slices.Contains([]string{"read", "write", "admin", "none"}, p) {
		return fmt.Errorf("perm %q: want read, write, admin or none", p)
	}
	x.Perms[login] = p
	return nil
}

// AddRepo adds the repository owner/name.
func (s *Server) AddRepo(repo string, topics []string, public bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return fmt.Errorf("repo %q: want owner/name", repo)
	}
	if s.find(repo) != nil {
		return fmt.Errorf("%s exists", repo)
	}
	s.st.Repos = append(s.st.Repos, &Repo{Owner: owner, Name: name, Private: !public, Topics: topics, Perms: map[string]string{}})
	return nil
}

// ApproveDevice approves the device code shown as userCode for login.
func (s *Server) ApproveDevice(userCode, login string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.devices {
		if d.userCode == userCode {
			d.login = login
			return nil
		}
	}
	return errors.New("no device code " + userCode)
}

// AddToken makes token a token of login.
func (s *Server) AddToken(tok, login string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.Tokens[tok] = login
}

// ExpireTokens makes the access tokens issued to login expired, so a
// refresh is needed.
func (s *Server) ExpireTokens(login string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, is := range s.access {
		if is.login == login {
			is.expires = s.now().Add(-time.Second)
			s.access[k] = is
		}
	}
}
```

- [ ] **Step 5: admin.go を書く**

```go
package fakegh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type adminBody struct {
	Repo     string   `json:"repo,omitempty"`
	Login    string   `json:"login,omitempty"`
	Perm     string   `json:"perm,omitempty"`
	Topics   []string `json:"topics,omitempty"`
	Public   bool     `json:"public,omitempty"`
	UserCode string   `json:"user_code,omitempty"`
	Token    string   `json:"token,omitempty"`
}

// admin serves /_fakegh/, what the fakegh command calls to change the state.
func (s *Server) admin(w http.ResponseWriter, r *http.Request) {
	var b adminBody
	if r.Method != "POST" || json.NewDecoder(r.Body).Decode(&b) != nil {
		fail(w, http.StatusBadRequest, "want POST with a JSON body")
		return
	}
	var err error
	switch r.URL.Path {
	case "/_fakegh/perm":
		err = s.SetPerm(b.Repo, b.Login, b.Perm)
	case "/_fakegh/repo":
		err = s.AddRepo(b.Repo, b.Topics, b.Public)
	case "/_fakegh/device/approve":
		err = s.ApproveDevice(b.UserCode, b.Login)
	case "/_fakegh/token":
		s.AddToken(b.Token, b.Login)
	case "/_fakegh/token/expire":
		s.ExpireTokens(b.Login)
	default:
		fail(w, http.StatusNotFound, "Not Found")
		return
	}
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// Admin calls the /_fakegh/ endpoints of a running fake.
type Admin struct {
	URL  string
	HTTP *http.Client
}

func (a Admin) post(path string, b adminBody) error {
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	hc := a.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	res, err := hc.Post(a.URL+path, "application/json", bytes.NewReader(data)) //nolint:gosec // G107, G704: the URL is the running fake the developer named with --server
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != 200 {
		msg, _ := io.ReadAll(res.Body)
		return fmt.Errorf("%s: %d %s", path, res.StatusCode, bytes.TrimSpace(msg))
	}
	return nil
}

func (a Admin) SetPerm(repo, login, perm string) error {
	return a.post("/_fakegh/perm", adminBody{Repo: repo, Login: login, Perm: perm})
}

func (a Admin) AddRepo(repo string, topics []string, public bool) error {
	return a.post("/_fakegh/repo", adminBody{Repo: repo, Topics: topics, Public: public})
}

func (a Admin) ApproveDevice(userCode, login string) error {
	return a.post("/_fakegh/device/approve", adminBody{UserCode: userCode, Login: login})
}

func (a Admin) AddToken(token, login string) error {
	return a.post("/_fakegh/token", adminBody{Token: token, Login: login})
}

func (a Admin) ExpireTokens(login string) error {
	return a.post("/_fakegh/token/expire", adminBody{Login: login})
}
```

（`ServeHTTP` は `/_fakegh/` を `s.mu` を取らずに `admin` へ回し、各メソッドがロックを取る。）

- [ ] **Step 6: テストを通す**

Run: `go test -race ./internal/fakegh/`
Expected: PASS

- [ ] **Step 7: コミット**

```bash
git add internal/fakegh
git commit -m "feat(fakegh): フェーズ3が使う GitHub API のフェイクを作る"
```

---

### Task 4: GitHub の REST クライアント（`internal/github`）

**Files:**
- Create: `internal/github/client.go`、`internal/github/repos.go`、`internal/github/device.go`
- Test: `internal/github/client_test.go`
- Create: `internal/context/repoid.go`（remote URL を読む処理はここの1か所。`store init`・読み取り専用の判定・discover が使うので、このTaskで作る）
- Test: `internal/context/repoid_test.go`

**Interfaces:**
- Consumes: `config.Host`（Task 1）、`fakegh.New`（Task 3、テストだけ）
- Produces:
  - `type Client struct{ … }`、`func New(h config.Host, token string, hc *http.Client) *Client`（`hc` が nil なら30秒のタイムアウトの client）
  - `var ErrUnauthorized, ErrForbidden, ErrNotFound, ErrExists error`、`type RateLimitError struct{ Reset time.Time }`（`Error()` は `GitHub API rate limit; try again after <時刻>`）
  - `func Describe(err error, accountID, accountName, doing string) error`（エラーに「どのアカウントで何をしようとしたか」を添える。`<accountID> cannot <doing>: <err>`。`errors.Is(err, ErrUnauthorized)` のときは末尾に `(run hedgehug auth login <accountName>)` を足す。spec の「エラー」の節。認証と GitHub API を呼ぶ側はすべてこれを通す）
  - `type Repo struct { Name, FullName, CloneURL string; Owner struct{ Login string }; Private bool; Topics []string; Permissions struct{ Admin, Push, Pull bool } }`（JSON タグは GitHub の名前）
  - `type Collaborator struct{ Login, RoleName string }`
  - `func (c *Client) User() (string, error)`
  - `func (c *Client) CreateRepo(owner, name string, user bool) (*Repo, error)`（`user` が true なら `/user/repos`）
  - `func (c *Client) SetTopics(owner, name string, topics []string) error`
  - `func (c *Client) UserRepos() ([]Repo, error)`
  - `func (c *Client) Repo(owner, name string) (*Repo, error)`
  - `func (c *Client) Collaborators(owner, name string) ([]Collaborator, error)`
  - `type DeviceCode struct{ DeviceCode, UserCode, VerificationURI string; ExpiresIn, Interval int }`
  - `type TokenResponse struct{ AccessToken, RefreshToken, Error string; ExpiresIn, RefreshTokenExpiresIn int }`
  - `package context`（`internal/context`）: `func Split(u string) (host, owner, repo string, ok bool)`（remote URL を読む唯一の関数。`https`・`http`・`ssh`・`git` の URL と `user@host:owner/repo`。host は小文字でポートなし、owner と repo は書かれたまま、repo の `.git` は除く。ローカルのパスや owner/repo を取れない URL は ok が false）、`func RepoID(u string) (string, bool)`（`Split` を小文字でつないだ `<host>/<owner>/<repo>`。spec の「repo ID の正規化」）
  - `func RequestDeviceCode(hc *http.Client, web, clientID string) (*DeviceCode, error)`
  - `func PollToken(hc *http.Client, web, clientID, deviceCode string) (*TokenResponse, error)`（`Error` が `authorization_pending` などのときも err は nil）
  - `func RefreshToken(hc *http.Client, web, clientID, refresh string) (*TokenResponse, error)`

- [ ] **Step 1: 失敗するテストを書く**

`internal/github/client_test.go`:

```go
package github

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/fakegh"
)

func fake(t *testing.T) (*fakegh.Server, config.Host) {
	t.Helper()
	zero := 0
	s := fakegh.New(fakegh.State{
		Users:  []string{"alice", "bob"},
		Orgs:   map[string][]string{"org-a": {"alice"}},
		Tokens: map[string]string{"t-alice": "alice", "t-bob": "bob"},
		Repos: []*fakegh.Repo{
			{Owner: "org-a", Name: "hh-shared", Private: true, Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice": "admin", "bob": "read"}},
		},
		DeviceInterval: &zero,
	})
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return s, config.Host{Host: "github.com", API: srv.URL, Web: srv.URL}
}

func TestClient(t *testing.T) {
	s, h := fake(t)
	c := New(h, "t-alice", nil)
	if l, err := c.User(); err != nil || l != "alice" {
		t.Errorf("User = %q, %v", l, err)
	}
	r, err := c.CreateRepo("org-a", "hh-new", false)
	if err != nil || r.FullName != "org-a/hh-new" || r.CloneURL != "https://github.com/org-a/hh-new.git" {
		t.Fatalf("CreateRepo = %+v, %v", r, err)
	}
	if _, err := c.CreateRepo("org-a", "hh-new", false); !errors.Is(err, ErrExists) {
		t.Errorf("again = %v, want ErrExists", err)
	}
	if err := c.SetTopics("org-a", "hh-new", []string{"hedgehug-store"}); err != nil {
		t.Fatal(err)
	}
	if x, _ := s.Repo("org-a/hh-new"); len(x.Topics) != 1 {
		t.Errorf("topics = %v", x.Topics)
	}
	repos, err := c.UserRepos()
	if err != nil || len(repos) != 2 {
		t.Fatalf("UserRepos = %v, %v", repos, err)
	}
	cs, err := c.Collaborators("org-a", "hh-shared")
	if err != nil || len(cs) != 2 || cs[1] != (Collaborator{Login: "bob", RoleName: "read"}) {
		t.Errorf("Collaborators = %+v, %v", cs, err)
	}
	bob := New(h, "t-bob", nil)
	if r, err := bob.Repo("org-a", "hh-shared"); err != nil || r.Permissions.Push {
		t.Errorf("bob's Repo = %+v, %v", r, err)
	}
	if _, err := bob.Collaborators("org-a", "hh-shared"); !errors.Is(err, ErrForbidden) {
		t.Errorf("bob's Collaborators = %v, want ErrForbidden", err)
	}
	if _, err := bob.Repo("org-a", "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing = %v", err)
	}
	if _, err := New(h, "bad", nil).User(); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("bad token = %v", err)
	}
}

func TestUserReposPages(t *testing.T) {
	s, h := fake(t)
	for i := range 150 {
		name := fmt.Sprintf("org-a/r%03d", i)
		if err := s.AddRepo(name, nil, false); err != nil {
			t.Fatal(err)
		}
		if err := s.SetPerm(name, "alice", "read"); err != nil {
			t.Fatal(err)
		}
	}
	repos, err := New(h, "t-alice", nil).UserRepos()
	if err != nil || len(repos) != 151 {
		t.Errorf("UserRepos = %d, %v", len(repos), err)
	}
}

func TestRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1793500000")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer srv.Close()
	_, err := New(config.Host{API: srv.URL}, "t", nil).User()
	var rl *RateLimitError
	if !errors.As(err, &rl) || !rl.Reset.Equal(time.Unix(1793500000, 0)) {
		t.Errorf("err = %v", err)
	}
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv2.Close()
	if _, err := New(config.Host{API: srv2.URL}, "t", nil).User(); !errors.As(err, &rl) {
		t.Errorf("429 = %v", err)
	}
}

func TestDescribe(t *testing.T) {
	err := Describe(ErrUnauthorized, "github.com/alice-corp", "work", "read the permission of org-a/hh-shared")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("Describe lost the cause: %v", err)
	}
	for _, want := range []string{"github.com/alice-corp cannot read the permission of org-a/hh-shared", "401", "run hedgehug auth login work"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to contain %q", err, want)
		}
	}
	// Only a 401 is cured by logging in again.
	if err := Describe(ErrForbidden, "github.com/bob", "me", "list the collaborators of org-a/x"); !errors.Is(err, ErrForbidden) || strings.Contains(err.Error(), "auth login") {
		t.Errorf("403 = %v", err)
	}
	if Describe(nil, "github.com/bob", "me", "do it") != nil {
		t.Error("Describe(nil) is not nil")
	}
}

func TestDevice(t *testing.T) {
	s, h := fake(t)
	dc, err := RequestDeviceCode(nil, h.Web, "Iv23x")
	if err != nil || dc.UserCode == "" {
		t.Fatalf("RequestDeviceCode = %+v, %v", dc, err)
	}
	tr, err := PollToken(nil, h.Web, "Iv23x", dc.DeviceCode)
	if err != nil || tr.Error != "authorization_pending" {
		t.Errorf("before approval = %+v, %v", tr, err)
	}
	if err := s.ApproveDevice(dc.UserCode, "alice"); err != nil {
		t.Fatal(err)
	}
	tr, err = PollToken(nil, h.Web, "Iv23x", dc.DeviceCode)
	if err != nil || tr.AccessToken == "" || tr.RefreshToken == "" || tr.ExpiresIn == 0 {
		t.Fatalf("token = %+v, %v", tr, err)
	}
	tr2, err := RefreshToken(nil, h.Web, "Iv23x", tr.RefreshToken)
	if err != nil || tr2.AccessToken == "" || tr2.AccessToken == tr.AccessToken {
		t.Errorf("refresh = %+v, %v", tr2, err)
	}
}
```

`internal/context/repoid_test.go`（remote URL の解析のテストはここだけ。`store init`・読み取り専用の判定・discover は `Split` を使い、別の解析を持たない）:

```go
package context

import "testing"

func TestSplit(t *testing.T) {
	for in, want := range map[string][3]string{
		"https://github.com/Org-A/HH-Shared.git":     {"github.com", "Org-A", "HH-Shared"},
		"https://GitHub.com/org-a/api/":              {"github.com", "org-a", "api"},
		"http://127.0.0.1:7777/x/y.git":              {"127.0.0.1", "x", "y"},
		"https://ghe.example.com:8443/x/y":           {"ghe.example.com", "x", "y"},
		"git@github.com:org-a/api.git":               {"github.com", "org-a", "api"},
		"git@github.com:Org-A/API":                   {"github.com", "Org-A", "API"},
		"ssh://git@github.com:22/org-a/api.git":      {"github.com", "org-a", "api"},
		"ssh://ghe.example.com/org-a/api":            {"ghe.example.com", "org-a", "api"},
		"git://github.com/org-a/api":                 {"github.com", "org-a", "api"},
	} {
		h, o, n, ok := Split(in)
		if !ok || [3]string{h, o, n} != want {
			t.Errorf("Split(%q) = %q %q %q %v, want %v", in, h, o, n, ok, want)
		}
	}
	for _, in := range []string{"/tmp/x.git", "file:///tmp/x", "https://github.com/only", "https://github.com/a/b/c", `C:\x\y`, ""} {
		if h, o, n, ok := Split(in); ok {
			t.Errorf("Split(%q) = %q %q %q, want none", in, h, o, n)
		}
	}
}

func TestRepoID(t *testing.T) {
	for in, want := range map[string]string{
		"https://github.com/Org-A/API.git":      "github.com/org-a/api",
		"http://github.com/org-a/api/":          "github.com/org-a/api",
		"ssh://git@github.com:22/org-a/api.git": "github.com/org-a/api",
		"git@github.com:org-a/api.git":          "github.com/org-a/api",
		"git://github.com/org-a/api":            "github.com/org-a/api",
		"https://ghe.example.com:8443/x/y":      "ghe.example.com/x/y",
	} {
		if got, ok := RepoID(in); !ok || got != want {
			t.Errorf("RepoID(%q) = %q %v, want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"/tmp/x.git", "file:///tmp/x", "https://github.com/only", `C:\x\y`} {
		if got, ok := RepoID(in); ok {
			t.Errorf("RepoID(%q) = %q, want none", in, got)
		}
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/github/ ./internal/context/`
Expected: FAIL（パッケージがない）

- [ ] **Step 3: client.go を書く**

```go
// Package github is a client of the GitHub REST API: the endpoints Hedgehug
// uses to find, create and check its store repositories, and the device flow.
package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/kukv/hedgehug/internal/config"
)

// The errors of a call, by the status of the answer.
var (
	ErrUnauthorized = errors.New("GitHub refused the token (401)")
	ErrForbidden    = errors.New("GitHub denied it (403)")
	ErrNotFound     = errors.New("GitHub has no such thing, or it is not visible to the token (404)")
	ErrExists       = errors.New("a repository of that name exists")
)

// RateLimitError means the rate limit is used up until Reset.
type RateLimitError struct{ Reset time.Time }

func (e *RateLimitError) Error() string {
	return "GitHub API rate limit; try again after " + e.Reset.Format(time.RFC3339)
}

// Describe says in err which account did what, and for a 401 what to run: a
// failure of the API is shown to the person as
// "github.com/alice-corp cannot read the permission of org-a/hh-shared: …
// (run hedgehug auth login work)". accountID is <host>/<login>, accountName
// the name in config.yaml, doing what was being done.
func Describe(err error, accountID, accountName, doing string) error {
	if err == nil {
		return nil
	}
	err = fmt.Errorf("%s cannot %s: %w", accountID, doing, err)
	if errors.Is(err, ErrUnauthorized) {
		err = fmt.Errorf("%w (run hedgehug auth login %s)", err, accountName)
	}
	return err
}

// Client calls the REST API of one host with one token.
type Client struct {
	api, token string
	hc         *http.Client
}

func defaultHTTP(hc *http.Client) *http.Client {
	if hc == nil {
		return &http.Client{Timeout: 30 * time.Second}
	}
	return hc
}

// New makes a client of the host h with token. hc nil is a client with a
// 30-second timeout.
func New(h config.Host, token string, hc *http.Client) *Client {
	return &Client{api: h.API, token: token, hc: defaultHTTP(hc)}
}

// do calls method on the API path with the JSON of in (nil for none), and
// decodes the answer into out (nil to drop it). It returns the response for
// its headers.
func (c *Client) do(method, url string, in, out any) (*http.Response, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.hc.Do(req) //nolint:gosec // G704: the URL is the API base of a host in config.yaml
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	data, _ := io.ReadAll(res.Body)
	if err := statusError(res, data); err != nil {
		return res, fmt.Errorf("%s %s: %w", method, url, err)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return res, fmt.Errorf("%s %s: %w", method, url, err)
		}
	}
	return res, nil
}

func statusError(res *http.Response, data []byte) error {
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	if res.StatusCode == http.StatusTooManyRequests || res.StatusCode == http.StatusForbidden && (res.Header.Get("X-RateLimit-Remaining") == "0" || res.Header.Get("Retry-After") != "") {
		reset := time.Now().Add(time.Minute)
		if s, err := strconv.ParseInt(res.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			reset = time.Unix(s, 0)
		} else if s, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil {
			reset = time.Now().Add(time.Duration(s) * time.Second)
		}
		return &RateLimitError{Reset: reset}
	}
	var msg struct {
		Message string `json:"message"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(data, &msg)
	switch res.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return fmt.Errorf("%w: %s", ErrForbidden, msg.Message)
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusUnprocessableEntity:
		for _, e := range msg.Errors {
			if bytes.Contains([]byte(e.Message), []byte("already exists")) {
				return ErrExists
			}
		}
	}
	return fmt.Errorf("GitHub answered %d: %s", res.StatusCode, msg.Message)
}

var nextLink = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

// getAll gets every page of a list from url and appends its items to out.
func getAll[T any](c *Client, url string) ([]T, error) {
	var all []T
	for url != "" {
		var page []T
		res, err := c.do("GET", url, nil, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		url = ""
		if m := nextLink.FindStringSubmatch(res.Header.Get("Link")); m != nil {
			url = m[1]
		}
	}
	return all, nil
}
```

- [ ] **Step 4: repos.go を書く**

```go
package github

import "net/url"

// Repo is a repository as the API gives it.
type Repo struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	CloneURL string `json:"clone_url"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
	Private     bool     `json:"private"`
	Topics      []string `json:"topics"`
	Permissions struct {
		Admin bool `json:"admin"`
		Push  bool `json:"push"`
		Pull  bool `json:"pull"`
	} `json:"permissions"`
}

// Collaborator is a person who may read the repository, and as what.
type Collaborator struct {
	Login    string `json:"login"`
	RoleName string `json:"role_name"`
}

// User returns the login of the token.
func (c *Client) User() (string, error) {
	var u struct {
		Login string `json:"login"`
	}
	_, err := c.do("GET", c.api+"/user", nil, &u)
	return u.Login, err
}

// CreateRepo makes the private repository owner/name: as the token's own
// when user is true, in the organization owner otherwise.
func (c *Client) CreateRepo(owner, name string, user bool) (*Repo, error) {
	path := c.api + "/orgs/" + url.PathEscape(owner) + "/repos"
	if user {
		path = c.api + "/user/repos"
	}
	var r Repo
	_, err := c.do("POST", path, map[string]any{"name": name, "private": true, "auto_init": false}, &r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// SetTopics replaces the topics of owner/name.
func (c *Client) SetTopics(owner, name string, topics []string) error {
	_, err := c.do("PUT", c.repoURL(owner, name)+"/topics", map[string][]string{"names": topics}, nil)
	return err
}

// UserRepos lists every repository the token can read: its own, those it
// collaborates on, and those of its organizations.
func (c *Client) UserRepos() ([]Repo, error) {
	return getAll[Repo](c, c.api+"/user/repos?affiliation=owner,collaborator,organization_member&per_page=100")
}

// Repo gets owner/name with the token's permissions.
func (c *Client) Repo(owner, name string) (*Repo, error) {
	var r Repo
	if _, err := c.do("GET", c.repoURL(owner, name), nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// Collaborators lists who may read owner/name. GitHub answers 403 to a token
// without push access.
func (c *Client) Collaborators(owner, name string) ([]Collaborator, error) {
	return getAll[Collaborator](c, c.repoURL(owner, name)+"/collaborators?per_page=100")
}

func (c *Client) repoURL(owner, name string) string {
	return c.api + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
}
```

- [ ] **Step 5: device.go を書く**

```go
package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// DeviceCode is the start of the device flow.
type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse is the answer of the token endpoint. Error is set, with no
// tokens, while the user has not approved or when it failed.
type TokenResponse struct {
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
	Error                 string `json:"error"`
}

func postForm(hc *http.Client, u string, form url.Values, out any) error {
	req, err := http.NewRequest("POST", u, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := defaultHTTP(hc).Do(req) //nolint:gosec // G704: web is the web base of a host in config.yaml
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("POST %s: %d %s", u, res.StatusCode, strings.TrimSpace(string(data)))
	}
	return json.Unmarshal(data, out)
}

// RequestDeviceCode starts the device flow of the App clientID at web.
func RequestDeviceCode(hc *http.Client, web, clientID string) (*DeviceCode, error) {
	var raw struct {
		DeviceCode
		Error string `json:"error"`
	}
	if err := postForm(hc, web+"/login/device/code", url.Values{"client_id": {clientID}}, &raw); err != nil {
		return nil, err
	}
	if raw.Error != "" || raw.DeviceCode.DeviceCode == "" {
		return nil, fmt.Errorf("the device flow did not start: %s", raw.Error)
	}
	return &raw.DeviceCode, nil
}

// PollToken asks once whether the device code was approved.
func PollToken(hc *http.Client, web, clientID, deviceCode string) (*TokenResponse, error) {
	var tr TokenResponse
	err := postForm(hc, web+"/login/oauth/access_token", url.Values{
		"client_id": {clientID}, "device_code": {deviceCode},
		"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"},
	}, &tr)
	return &tr, err
}

// RefreshToken trades a refresh token for a new pair of tokens.
func RefreshToken(hc *http.Client, web, clientID, refresh string) (*TokenResponse, error) {
	var tr TokenResponse
	err := postForm(hc, web+"/login/oauth/access_token", url.Values{
		"client_id": {clientID}, "refresh_token": {refresh}, "grant_type": {"refresh_token"},
	}, &tr)
	return &tr, err
}
```

- [ ] **Step 6: repoid.go を書く**

`internal/context/repoid.go`:

```go
// Package context decides where a new task goes when the command does not
// say, and reads remote URLs. Phase 3a has only the remote URLs.
package context

import (
	"net/url"
	"regexp"
	"strings"
)

// scpLike is user@host:owner/repo, the short form of ssh.
var scpLike = regexp.MustCompile(`^[^@/]+@([^:/]+):(.+)$`)

// Split reads a remote URL: https, http, ssh, git and user@host:path. The
// host is in lower case and has no port; the owner and the repo are as
// written, and the repo has no .git or trailing slash. A local path, or a URL
// that does not end in an owner and a repo, is not ok.
//
// This is the one place that reads a remote URL: store init (is this a
// GitHub host, and which repository to make), the read-only check and the
// member list use the owner and the repo from here, and RepoID lower-cases
// it.
func Split(u string) (host, owner, repo string, ok bool) {
	var p string
	if m := scpLike.FindStringSubmatch(u); m != nil && !strings.Contains(u, "://") {
		host, p = m[1], m[2]
	} else {
		x, err := url.Parse(u)
		if err != nil {
			return "", "", "", false
		}
		switch x.Scheme {
		case "https", "http", "ssh", "git":
		default:
			return "", "", "", false
		}
		host, p = x.Hostname(), x.Path
	}
	parts := strings.Split(strings.Trim(strings.TrimSuffix(strings.Trim(p, "/"), ".git"), "/"), "/")
	if host == "" || len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", false
	}
	return strings.ToLower(host), parts[0], parts[1], true
}

// RepoID turns a remote URL into <host>/<owner>/<repo> in lower case, without
// .git, a trailing slash or a port. A local path, or a URL without an owner
// and a repo, has none.
func RepoID(u string) (string, bool) {
	host, owner, repo, ok := Split(u)
	if !ok {
		return "", false
	}
	return strings.ToLower(host + "/" + owner + "/" + repo), true
}
```

- [ ] **Step 7: テストを通す**

Run: `go test -race ./internal/github/ ./internal/context/`
Expected: PASS

- [ ] **Step 8: コミット**

```bash
git add internal/github internal/context
git commit -m "feat(github): フェーズ3で使う REST API と Device Flow のクライアントを作り、remote URL の解析を internal/context に置く"
```

---

### Task 5: Device Flow でのログインとリフレッシュ（`internal/auth`）

**Files:**
- Create: `internal/auth/device.go`
- Modify: `internal/auth/source.go`（`deviceToken` を device.go へ移し、リフレッシュを足す）
- Test: `internal/auth/device_test.go`

**Interfaces:**
- Consumes: `github.RequestDeviceCode`、`github.PollToken`、`github.RefreshToken`（Task 4）、`store.WithFileLock`（Task 2）
- Produces:
  - `type Prompt func(dc *github.DeviceCode)`（コードと URL を見せる呼び出し側の関数）
  - `func DeviceLogin(h config.Host, opt Options, show Prompt) (DeviceToken, error)`
  - `deviceToken`（`Token` の `device`）: 期限まで5分を切っていたら `auth.lock` を取り、keychain を読み直し、まだ近ければ `RefreshToken` して保存する

- [ ] **Step 1: 失敗するテストを書く**

`internal/auth/device_test.go`:

```go
package auth

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/github"
)

func fakeHost(t *testing.T, st fakegh.State) (*fakegh.Server, config.Host, *atomic.Int32) {
	t.Helper()
	s := fakegh.New(st)
	var refreshes atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/oauth/access_token" {
			_ = r.ParseForm()
			if r.PostForm.Get("grant_type") == "refresh_token" {
				refreshes.Add(1)
			}
		}
		s.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return s, config.Host{Host: "github.com", API: srv.URL, Web: srv.URL, ClientID: "Iv23x"}, &refreshes
}

func TestDeviceLogin(t *testing.T) {
	zero := 0
	_, h, _ := fakeHost(t, fakegh.State{Users: []string{"alice"}, DeviceInterval: &zero, AutoApprove: "alice"})
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	var shown string
	dt, err := DeviceLogin(h, Options{Now: func() time.Time { return now }, Sleep: func(time.Duration) {}}, func(dc *github.DeviceCode) { shown = dc.UserCode })
	if err != nil || dt.AccessToken == "" || shown == "" {
		t.Fatalf("DeviceLogin = %+v, %v (shown %q)", dt, err, shown)
	}
	if !dt.ExpiresAt.Equal(now.Add(8*time.Hour)) || dt.RefreshExpiresAt.Before(now.Add(4000*time.Hour)) {
		t.Errorf("expiry = %v, %v", dt.ExpiresAt, dt.RefreshExpiresAt)
	}
	if _, err := DeviceLogin(config.Host{Host: "ghe", Web: h.Web}, Options{}, func(*github.DeviceCode) {}); err == nil || !strings.Contains(err.Error(), "client_id") {
		t.Errorf("no client ID = %v", err)
	}
}

func TestDeviceLoginWaits(t *testing.T) {
	zero := 0
	s, h, _ := fakeHost(t, fakegh.State{Users: []string{"alice"}, DeviceInterval: &zero})
	var sleeps int
	opt := Options{Sleep: func(time.Duration) {
		sleeps++
		if sleeps == 2 {
			// The user approves while the CLI waits.
			for _, c := range lastCodes {
				_ = s.ApproveDevice(c, "alice")
			}
		}
	}}
	dt, err := DeviceLogin(h, opt, func(dc *github.DeviceCode) { lastCodes = append(lastCodes, dc.UserCode) })
	if err != nil || dt.AccessToken == "" || sleeps < 2 {
		t.Errorf("DeviceLogin = %+v, %v after %d sleeps", dt, err, sleeps)
	}
}

var lastCodes []string

func TestDeviceRefresh(t *testing.T) {
	zero := 0
	s, h, refreshes := fakeHost(t, fakegh.State{Users: []string{"alice"}, DeviceInterval: &zero, AutoApprove: "alice"})
	_ = s
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	k := newMem()
	a := config.Account{Name: "a", Host: "github.com", Login: "alice", Token: config.TokenSource{Kind: "device"}}
	opt := Options{Keyring: k, Host: h, Now: func() time.Time { return now }, Sleep: func(time.Duration) {}, LockFile: filepath.Join(t.TempDir(), "auth.lock")}
	dt, err := DeviceLogin(h, opt, func(*github.DeviceCode) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveDeviceToken(a, dt, k); err != nil {
		t.Fatal(err)
	}
	if tok, err := Token(a, opt); err != nil || tok != dt.AccessToken || refreshes.Load() != 0 {
		t.Errorf("fresh token = %q, %v, %d refreshes", tok, err, refreshes.Load())
	}
	now = now.Add(8*time.Hour - 4*time.Minute) // within 5 minutes of the end
	var wg sync.WaitGroup
	toks := make([]string, 4)
	for i := range toks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			toks[i], _ = Token(a, opt)
		}()
	}
	wg.Wait()
	if refreshes.Load() != 1 {
		t.Errorf("refreshes = %d, want 1 for 4 callers at once", refreshes.Load())
	}
	for _, tok := range toks {
		if tok == "" || tok == dt.AccessToken || tok != toks[0] {
			t.Errorf("tokens = %q", toks)
		}
	}
	saved, _ := LoadDeviceToken(a, k)
	if saved.AccessToken != toks[0] {
		t.Errorf("saved = %+v", saved)
	}
	// A refresh token that GitHub no longer takes asks for a new login.
	saved.RefreshToken = "ghr_gone"
	saved.ExpiresAt = now
	_ = SaveDeviceToken(a, saved, k)
	if _, err := Token(a, opt); err == nil || !strings.Contains(err.Error(), "hedgehug auth login a") {
		t.Errorf("bad refresh = %v", err)
	}
}
```

（`newMem` は Task 2 のテストのもの。ゴルーチンから並行に呼んでよい。）

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/auth/`
Expected: FAIL（`DeviceLogin` が未定義）

- [ ] **Step 3: device.go を書き、source.go の deviceToken を消す**

```go
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/github"
	"github.com/kukv/hedgehug/internal/store"
)

// refreshBefore is how long before its end an access token is refreshed.
const refreshBefore = 5 * time.Minute

// lockWait is how long a refresh waits for another one to end.
const lockWait = 30 * time.Second

// Prompt shows the user the code and where to enter it.
type Prompt func(dc *github.DeviceCode)

// DeviceLogin runs the device flow of the Hedgehug GitHub App on the host h
// and returns the tokens it got.
func DeviceLogin(h config.Host, opt Options, show Prompt) (DeviceToken, error) {
	id := ClientID(h)
	if id == "" {
		return DeviceToken{}, fmt.Errorf("no GitHub App client ID for %s; set client_id of the host in hosts of config.yaml", h.Host)
	}
	dc, err := github.RequestDeviceCode(opt.HTTP, h.Web, id)
	if err != nil {
		return DeviceToken{}, err
	}
	show(dc)
	sleep := opt.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	interval := time.Duration(dc.Interval) * time.Second
	deadline := opt.now().Add(time.Duration(dc.ExpiresIn) * time.Second)
	for {
		tr, err := github.PollToken(opt.HTTP, h.Web, id, dc.DeviceCode)
		if err != nil {
			return DeviceToken{}, err
		}
		switch tr.Error {
		case "":
			return deviceTokenOf(tr, opt.now()), nil
		case "authorization_pending":
		case "slow_down":
			interval += 5 * time.Second
		case "expired_token":
			return DeviceToken{}, errors.New("the code expired before it was entered; run hedgehug auth login again")
		case "access_denied":
			return DeviceToken{}, errors.New("the login was denied")
		default:
			return DeviceToken{}, fmt.Errorf("the device flow failed: %s", tr.Error)
		}
		if opt.now().After(deadline) {
			return DeviceToken{}, errors.New("the code expired before it was entered; run hedgehug auth login again")
		}
		sleep(interval)
	}
}

func deviceTokenOf(tr *github.TokenResponse, now time.Time) DeviceToken {
	return DeviceToken{
		AccessToken: tr.AccessToken, RefreshToken: tr.RefreshToken,
		ExpiresAt:        now.Add(time.Duration(tr.ExpiresIn) * time.Second),
		RefreshExpiresAt: now.Add(time.Duration(tr.RefreshTokenExpiresIn) * time.Second),
	}
}

// deviceToken returns the access token of a device login, refreshed first
// when it ends within refreshBefore. The refresh holds auth.lock and reads
// the keychain again under it, so of two processes only one refreshes.
func deviceToken(a config.Account, opt Options) (string, error) {
	k := opt.Keychain()
	t, err := LoadDeviceToken(a, k)
	if errors.Is(err, ErrNotFound) {
		return "", fmt.Errorf("%s is not logged in; run hedgehug auth login %s", Key(a), a.Name)
	}
	if err != nil {
		return "", err
	}
	if opt.now().Add(refreshBefore).Before(t.ExpiresAt) {
		return t.AccessToken, nil
	}
	if opt.LockFile == "" {
		return "", errors.New("auth: LockFile is not set")
	}
	err = store.WithFileLock(opt.LockFile, lockWait, func() error {
		if t, err = LoadDeviceToken(a, k); err != nil {
			return err
		}
		if opt.now().Add(refreshBefore).Before(t.ExpiresAt) {
			return nil // another process refreshed it
		}
		if !t.RefreshExpiresAt.IsZero() && opt.now().After(t.RefreshExpiresAt) {
			return fmt.Errorf("the login of %s has expired; run hedgehug auth login %s", Key(a), a.Name)
		}
		tr, err := github.RefreshToken(opt.HTTP, opt.Host.Web, ClientID(opt.Host), t.RefreshToken)
		if err != nil {
			return err
		}
		if tr.Error != "" {
			return fmt.Errorf("refreshing the token of %s failed (%s); run hedgehug auth login %s", Key(a), tr.Error, a.Name)
		}
		t = deviceTokenOf(tr, opt.now())
		return SaveDeviceToken(a, t, k)
	})
	if err != nil {
		return "", err
	}
	return t.AccessToken, nil
}
```

`Options.LockFile` は呼び出し側（cmd）が必ず渡す。空のままリフレッシュに来たらエラーにする（上のコード）。

- [ ] **Step 4: テストを通す**

Run: `go test -race ./internal/auth/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/auth
git commit -m "feat(auth): Device Flow でログインし、期限の近いトークンをロックの中でリフレッシュする"
```

---

### Task 6: `cmd/fakegh`

**Files:**
- Create: `cmd/fakegh/main.go`
- Test: `cmd/fakegh/main_test.go`

**Interfaces:**
- Consumes: `fakegh.New`、`fakegh.Load`、`fakegh.Admin`（Task 3）
- Produces: 実行ファイル `fakegh`。`serve [--addr 127.0.0.1:7777] [--seed <json>]`、`perm set <owner/repo> <login> <perm>`、`repo add <owner/repo> [--topic t]... [--public]`、`device approve <user_code> --user <login>`、`token add <token> --user <login>`、`token expire <login>`。操作用は `--server`（既定 `http://127.0.0.1:7777`）

- [ ] **Step 1: 失敗するテストを書く**

`cmd/fakegh/main_test.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/fakegh"
)

func TestCommands(t *testing.T) {
	s := fakegh.New(fakegh.State{Users: []string{"bob"}, Repos: []*fakegh.Repo{{Owner: "org-a", Name: "x"}}})
	srv := httptest.NewServer(s)
	defer srv.Close()
	for _, args := range [][]string{
		{"perm", "set", "org-a/x", "bob", "read", "--server", srv.URL},
		{"repo", "add", "org-a/y", "--topic", "hedgehug-store", "--server", srv.URL},
		{"token", "add", "t1", "--user", "bob", "--server", srv.URL},
		{"token", "expire", "bob", "--server", srv.URL},
	} {
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != 0 {
			t.Errorf("fakegh %s: exit %d: %s", strings.Join(args, " "), code, errb.String())
		}
	}
	if r, _ := s.Repo("org-a/x"); r.Perms["bob"] != "read" {
		t.Errorf("perm = %v", r.Perms)
	}
	if r, ok := s.Repo("org-a/y"); !ok || r.Topics[0] != "hedgehug-store" {
		t.Errorf("repo = %+v", r)
	}

	// device approve: the user_code of a device code, approved from the
	// command, makes the next poll give a token.
	res, err := http.PostForm(srv.URL+"/login/device/code", url.Values{"client_id": {"Iv23x"}})
	if err != nil {
		t.Fatal(err)
	}
	var dc struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
	}
	err = json.NewDecoder(res.Body).Decode(&dc)
	_ = res.Body.Close()
	if err != nil || dc.UserCode == "" {
		t.Fatalf("device code = %+v, %v", dc, err)
	}
	poll := func() string {
		res, err := http.PostForm(srv.URL+"/login/oauth/access_token", url.Values{
			"client_id": {"Iv23x"}, "device_code": {dc.DeviceCode},
			"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"},
		})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = res.Body.Close() }()
		b, _ := io.ReadAll(res.Body)
		return string(b)
	}
	if got := poll(); !strings.Contains(got, "authorization_pending") {
		t.Errorf("before approval = %s", got)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"device", "approve", dc.UserCode, "--user", "bob", "--server", srv.URL}, &out, &errb); code != 0 {
		t.Errorf("device approve: exit %d: %s", code, errb.String())
	}
	if got := poll(); !strings.Contains(got, "ghu_") {
		t.Errorf("after approval = %s", got)
	}
	errb.Reset()
	if code := run([]string{"device", "approve", "NOPE-0000", "--user", "bob", "--server", srv.URL}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "NOPE-0000") {
		t.Errorf("device approve of a code nobody has: exit %d: %s", code, errb.String())
	}
	if code := run([]string{"device", "approve", dc.UserCode, "--server", srv.URL}, &out, &errb); code != 2 {
		t.Errorf("device approve without --user: exit %d", code)
	}
	errb.Reset()
	if code := run([]string{"perm", "set", "org-a/x"}, &bytes.Buffer{}, &errb); code != 2 {
		t.Errorf("too few args: exit %d", code)
	}
	if code := run([]string{"nope"}, &bytes.Buffer{}, &errb); code != 2 {
		t.Errorf("unknown command: exit %d", code)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./cmd/fakegh/`
Expected: FAIL（`run` が未定義）

- [ ] **Step 3: main.go を書く**

```go
// Command fakegh serves a fake of the GitHub API that Hedgehug uses, and
// changes its state while it runs. It is for the development environment
// and tests, and is not shipped.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/fakegh"
)

const usage = `usage:
  fakegh serve [--addr 127.0.0.1:7777] [--seed <json>]
  fakegh perm set <owner/repo> <login> <read|write|admin|none>
  fakegh repo add <owner/repo> [--topic t]... [--public]
  fakegh device approve <user_code> --user <login>
  fakegh token add <token> --user <login>
  fakegh token expire <login>
The commands but serve take --server (default http://127.0.0.1:7777).`

type usageError struct{ error }

// topics is a flag that may be given more than once.
type topics []string

func (t *topics) String() string     { return strings.Join(*t, ",") }
func (t *topics) Set(v string) error { *t = append(*t, v); return nil }

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	err := dispatch(args, stdout, stderr)
	var ue usageError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &ue):
		_, _ = fmt.Fprintf(stderr, "fakegh: %v\n%s\n", err, usage)
		return 2
	default:
		_, _ = fmt.Fprintln(stderr, "fakegh:", err)
		return 1
	}
}

// parse parses the flags of fs wherever they are among args and returns the
// positional arguments; it wants exactly n of them.
func parse(fs *flag.FlagSet, args []string, n int) ([]string, error) {
	var pos []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return nil, usageError{err}
		}
		args = fs.Args()
		if len(args) > 0 {
			pos, args = append(pos, args[0]), args[1:]
		}
	}
	if len(pos) != n {
		return nil, usageError{fmt.Errorf("want %d arguments, got %d", n, len(pos))}
	}
	return pos, nil
}

func dispatch(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return usageError{errors.New("a command is needed")}
	}
	fs := flag.NewFlagSet("fakegh", flag.ContinueOnError)
	fs.SetOutput(stderr)
	server := fs.String("server", "http://127.0.0.1:7777", "the URL of the running fake")
	user := fs.String("user", "", "the login")
	var ts topics
	cmd := args[0]
	if len(args) > 1 && cmd != "serve" {
		cmd += " " + args[1]
	}
	admin := func() fakegh.Admin { return fakegh.Admin{URL: strings.TrimSuffix(*server, "/")} }
	switch cmd {
	case "serve":
		addr := fs.String("addr", "127.0.0.1:7777", "where to listen")
		seed := fs.String("seed", "", "a JSON file of the state to start with")
		if _, err := parse(fs, args[1:], 0); err != nil {
			return err
		}
		var st fakegh.State
		if *seed != "" {
			var err error
			if st, err = fakegh.Load(*seed); err != nil {
				return err
			}
		}
		_, _ = fmt.Fprintf(stdout, "fakegh listening on http://%s\n", *addr)
		srv := &http.Server{Addr: *addr, Handler: fakegh.New(st), ReadHeaderTimeout: 10 * time.Second}
		return srv.ListenAndServe()
	case "perm set":
		pos, err := parse(fs, args[2:], 3)
		if err != nil {
			return err
		}
		return admin().SetPerm(pos[0], pos[1], pos[2])
	case "repo add":
		fs.Var(&ts, "topic", "a topic; give it again for more")
		public := fs.Bool("public", false, "make it public")
		pos, err := parse(fs, args[2:], 1)
		if err != nil {
			return err
		}
		return admin().AddRepo(pos[0], ts, *public)
	case "device approve":
		pos, err := parse(fs, args[2:], 1)
		if err != nil {
			return err
		}
		if *user == "" {
			return usageError{errors.New("--user is required")}
		}
		return admin().ApproveDevice(pos[0], *user)
	case "token add":
		pos, err := parse(fs, args[2:], 1)
		if err != nil {
			return err
		}
		if *user == "" {
			return usageError{errors.New("--user is required")}
		}
		return admin().AddToken(pos[0], *user)
	case "token expire":
		pos, err := parse(fs, args[2:], 1)
		if err != nil {
			return err
		}
		return admin().ExpireTokens(pos[0])
	}
	return usageError{fmt.Errorf("unknown command %q", cmd)}
}
```

- [ ] **Step 4: テストを通す**

Run: `go test ./cmd/fakegh/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add cmd/fakegh
git commit -m "feat(fakegh): fakegh の serve と状態を変えるコマンドを作る"
```

---

### Task 7: git の通信（`url_rewrites`・HTTPS のトークン・`ErrPushDenied`）

**Files:**
- Create: `internal/gitrepo/conn.go`
- Modify: `internal/gitrepo/remote.go`（`Fetch`・`Push`・`Clone` が `Conn` を使う）
- Create: `internal/gitrepo/gitrepotest/https.go`、`internal/gitrepo/gitrepotest/lock.go`（読み取り専用の bare を作るテスト用ヘルパー。Task 10・11 のテストもこれを使う）
- Test: `internal/gitrepo/conn_test.go`
- Modify: `internal/sync/sync.go`（`Options.Conn`、`Run` の最初で `r.SetConn(opt.Conn)`）
- Create: `cmd/hedgehug/conn.go`
- Modify: `cmd/hedgehug/app.go`（`syncOptions` が `Conn` を入れる）、`cmd/hedgehug/store_add.go`（clone に `Conn`）、`cmd/hedgehug/main.go`（`env.keyring`）
- Test: `cmd/hedgehug/conn_test.go`

**Interfaces:**
- Consumes: `config.Config.Rewrite`、`config.Account.Token`（Task 1）、`auth.Token`、`auth.Options`（Task 2・5）
- Produces:
  - `type Conn struct { Rewrite func(url string) string; Auth func(url string) (transport.AuthMethod, error) }`（`Auth` に渡すのは書き換え後の URL）
  - `var ErrPushDenied = errors.New("the remote denied the push")`
  - `func (r *Repo) SetConn(c Conn)`、`func CloneWith(url, dir string, c Conn) (*Repo, error)`（`Clone(url, dir)` は `CloneWith(url, dir, Conn{})`）
  - `func LocalPath(url string) (string, bool)`（ローカルのパスか `file://` の URL なら、そのパス）、`func IsHTTP(url string) bool`、`func Writable(dir string) bool`（`dir/objects` にファイルを作って消せるか）
  - `func HeadHasFile(dir, path string) (bool, error)`（bare を含むリポジトリの HEAD のツリーに path があるか。Task 9 のテストと Task 12 が使う）
  - `func RemoteEmpty(u string, c Conn) (bool, error)`（`u` は `.git/config` にある書き換え前の URL。`c` で書き換えと認証を通して、go-git の `Remote.List` で remote の ref を取り、1つもなければ true。`transport.ErrEmptyRemoteRepository` も空として true。リポジトリがない・通信できないなどはエラー。Task 9 の `store init` が、リポジトリの作成が 422 のときに「既存の空のリポジトリか」を確かめるのに使う。git の通信で確かめるので、本物の GitHub でも SSH でも、テストの `gitrepotest.InstallHTTPS` でも同じ関数で動く）
  - `gitrepotest.LockObjects(t testing.TB, dir string) (unlock func())`: bare リポジトリ `dir` の `objects` とその下のディレクトリをすべて読み取り専用（0o500）にして、push と `Writable` を失敗させる。`unlock` で戻す（`t.Cleanup` でも戻すので TempDir を消せる）。Windows と root では `t.Skip`。読み取り専用の chmod の処理はここの1つだけで、各パッケージのテストは複製しない
  - `gitrepotest.InstallHTTPS(t testing.TB, route func(url string) (dir string, ok bool), auths *[]transport.AuthMethod, deny func(url string) bool)`: `https` と `http` の transport を、URL を dir の bare リポジトリに向けるインプロセスの server に差し替え、渡された AuthMethod を記録する。`deny` が true の URL は receive-pack で `transport.ErrAuthorizationFailed` を返す。`t.Cleanup` で元に戻す
  - `sync.Options.Conn gitrepo.Conn`
  - cmd: `func (a *app) connFor(c *config.Config, e config.StoreEntry) gitrepo.Conn`、`func (a *app) authOptions(c *config.Config, acct config.Account) auth.Options`、`func (a *app) tokenFor(c *config.Config, acct config.Account) (string, error)`（1回の実行の中でアカウントごとに覚える）
  - `env` に `keyring auth.Keyring`（nil なら OS の keychain）と `httpClient *http.Client`（nil なら既定）を足す

- [ ] **Step 1: gitrepotest を書く**

`internal/gitrepo/gitrepotest/https.go`:

```go
// Package gitrepotest serves https remotes from local bare repositories in
// tests, in-process, and records the auth each session was given.
package gitrepotest

import (
	"net/url"
	"sync"
	"testing"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

type loader struct {
	route func(string) (string, bool)
}

func (l loader) Load(ep *transport.Endpoint) (storer.Storer, error) {
	dir, ok := l.route(ep.String())
	if !ok {
		return nil, transport.ErrRepositoryNotFound
	}
	return filesystem.NewStorage(osfs.New(dir), cache.NewObjectLRUDefault()), nil
}

type recorder struct {
	transport.Transport
	mu    *sync.Mutex
	auths *[]transport.AuthMethod
	deny  func(string) bool
}

func (r recorder) record(a transport.AuthMethod) {
	if r.auths != nil {
		r.mu.Lock()
		*r.auths = append(*r.auths, a)
		r.mu.Unlock()
	}
}

func (r recorder) NewUploadPackSession(ep *transport.Endpoint, a transport.AuthMethod) (transport.UploadPackSession, error) {
	r.record(a)
	return r.Transport.NewUploadPackSession(ep, a)
}

func (r recorder) NewReceivePackSession(ep *transport.Endpoint, a transport.AuthMethod) (transport.ReceivePackSession, error) {
	r.record(a)
	if r.deny != nil && r.deny(ep.String()) {
		return nil, transport.ErrAuthorizationFailed
	}
	return r.Transport.NewReceivePackSession(ep, a)
}

// InstallHTTPS serves http and https URLs from the bare repositories route
// gives, records the auth of each session in auths (when not nil), and fails
// the pushes to the URLs deny reports. The usual transports are back after
// the test. The tests that call it must not run in parallel.
func InstallHTTPS(t testing.TB, route func(u string) (dir string, ok bool), auths *[]transport.AuthMethod, deny func(u string) bool) {
	t.Helper()
	tr := recorder{Transport: server.NewClient(loader{route: route}), mu: &sync.Mutex{}, auths: auths, deny: deny}
	client.InstallProtocol("https", tr)
	client.InstallProtocol("http", tr)
	t.Cleanup(func() {
		client.InstallProtocol("https", githttp.DefaultClient)
		client.InstallProtocol("http", githttp.DefaultClient)
	})
}

// Under routes the URLs under base (such as https://github.com/) to the
// same path under dir.
func Under(base, dir string) func(string) (string, bool) {
	return func(u string) (string, bool) {
		bu, err1 := url.Parse(base)
		uu, err2 := url.Parse(u)
		if err1 != nil || err2 != nil || bu.Host != uu.Host {
			return "", false
		}
		return dir + uu.Path, true
	}
}
```

`internal/gitrepo/gitrepotest/lock.go`:

```go
package gitrepotest

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// LockObjects makes the objects directory of the bare repository dir, and
// every directory in it, read-only, so that a push to it is denied and
// gitrepo.Writable is false. unlock gives the write permission back; the
// cleanup of the test calls it too, so that TempDir can remove the
// directory. The test is skipped where a directory cannot be made read-only
// to the user who runs it: on Windows, and as root.
func LockObjects(t testing.TB, dir string) (unlock func()) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("no read-only directories here")
	}
	var dirs []string
	_ = filepath.WalkDir(filepath.Join(dir, "objects"), func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			dirs = append(dirs, p)
		}
		return nil
	})
	for _, p := range dirs {
		if err := os.Chmod(p, 0o500); err != nil { //nolint:gosec // G302: a read-only directory is the point of this helper
			t.Fatal(err)
		}
	}
	unlock = func() {
		for _, p := range dirs {
			_ = os.Chmod(p, 0o700) //nolint:gosec // G302: give the write permission back so that TempDir can remove the directory
		}
	}
	t.Cleanup(unlock)
	return unlock
}
```

- [ ] **Step 2: conn の失敗するテストを書く**

`internal/gitrepo/conn_test.go`:

```go
package gitrepo

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
)

func TestRewriteFetchPush(t *testing.T) {
	d := t.TempDir()
	bare := filepath.Join(d, "remotes", "org", "x.git")
	if err := InitBare(bare); err != nil {
		t.Fatal(err)
	}
	r := initWithCommit(t, filepath.Join(d, "work"))
	if err := r.SetRemotes([]string{"https://github.com/org/x.git"}); err != nil {
		t.Fatal(err)
	}
	r.SetConn(Conn{Rewrite: func(u string) string {
		if u == "https://github.com/org/x.git" {
			return bare
		}
		return u
	}})
	if err := r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Fetch("origin"); err != nil {
		t.Fatal(err)
	}
	// .git/config keeps the URL as written.
	cfg, _ := r.r.Config()
	if got := cfg.Remotes["origin"].URLs[0]; got != "https://github.com/org/x.git" {
		t.Errorf("origin = %q", got)
	}
	c, err := CloneWith("https://github.com/org/x.git", filepath.Join(d, "clone"), Conn{Rewrite: func(string) string { return bare }})
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ = c.r.Config()
	if got := cfg.Remotes["origin"].URLs[0]; got != "https://github.com/org/x.git" {
		t.Errorf("the clone's origin = %q", got)
	}
}

func TestHTTPSAuthAndDenied(t *testing.T) {
	d := t.TempDir()
	if err := InitBare(filepath.Join(d, "org", "x.git")); err != nil {
		t.Fatal(err)
	}
	var auths []transport.AuthMethod
	denied := false
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), &auths, func(string) bool { return denied })
	r := initWithCommit(t, filepath.Join(d, "work"))
	if err := r.SetRemotes([]string{"https://github.com/org/x.git"}); err != nil {
		t.Fatal(err)
	}
	r.SetConn(Conn{Auth: func(u string) (transport.AuthMethod, error) {
		return &githttp.BasicAuth{Username: "x-access-token", Password: "tok"}, nil
	}})
	if err := r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	if len(auths) == 0 {
		t.Fatal("no session")
	}
	if b, ok := auths[0].(*githttp.BasicAuth); !ok || b.Username != "x-access-token" || b.Password != "tok" {
		t.Errorf("auth = %#v", auths[0])
	}
	denied = true
	commitFile(t, r, "b.txt", "b")
	if err := r.Push("origin"); !errors.Is(err, ErrPushDenied) {
		t.Errorf("denied push = %v, want ErrPushDenied", err)
	}
}

func TestLocalPathAndWritable(t *testing.T) {
	for in, want := range map[string]bool{"/tmp/x.git": true, "file:///tmp/x.git": true, "https://h/x": false, "git@h:o/r.git": false, `C:\x\y.git`: true} {
		if _, ok := LocalPath(in); ok != want {
			t.Errorf("LocalPath(%q) ok = %v", in, ok)
		}
	}
	if !IsHTTP("https://h/x") || IsHTTP("/x") {
		t.Error("IsHTTP is wrong")
	}
	d := filepath.Join(t.TempDir(), "x.git")
	if err := InitBare(d); err != nil {
		t.Fatal(err)
	}
	if !Writable(d) {
		t.Error("a fresh bare is not writable")
	}
	gitrepotest.LockObjects(t, d) // skips the rest where there are no read-only directories
	if Writable(d) {
		t.Error("a read-only bare is writable")
	}
}

func TestRemoteEmpty(t *testing.T) {
	d := t.TempDir()
	bare := filepath.Join(d, "org", "x.git")
	if err := InitBare(bare); err != nil {
		t.Fatal(err)
	}
	var auths []transport.AuthMethod
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), &auths, nil)
	conn := Conn{Auth: func(string) (transport.AuthMethod, error) {
		return &githttp.BasicAuth{Username: "x-access-token", Password: "tok"}, nil
	}}
	const u = "https://github.com/org/x.git"
	if empty, err := RemoteEmpty(u, conn); err != nil || !empty {
		t.Errorf("a bare with no ref = %v, %v", empty, err)
	}
	if len(auths) == 0 {
		t.Fatal("the listing sent no auth")
	}
	if b, ok := auths[0].(*githttp.BasicAuth); !ok || b.Password != "tok" {
		t.Errorf("auth = %#v", auths[0])
	}
	r := initWithCommit(t, filepath.Join(d, "work"))
	if err := r.SetRemotes([]string{bare}); err != nil {
		t.Fatal(err)
	}
	if err := r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	if empty, err := RemoteEmpty(u, conn); err != nil || empty {
		t.Errorf("a bare with a commit = %v, %v", empty, err)
	}
	// The rewrite applies: the URL as written is not served at all.
	rw := Conn{Rewrite: func(string) string { return bare }}
	if empty, err := RemoteEmpty("https://example.invalid/org/x.git", rw); err != nil || empty {
		t.Errorf("through a rewrite = %v, %v", empty, err)
	}
	// A repository that is not there (here: a host nothing serves) is an
	// error, not an empty one.
	if empty, err := RemoteEmpty("https://elsewhere.test/org/x.git", conn); err == nil || empty {
		t.Errorf("a missing repository = %v, %v", empty, err)
	}
}

func TestHeadHasFile(t *testing.T) {
	d := t.TempDir()
	r := initWithCommit(t, filepath.Join(d, "work"))
	bare := filepath.Join(d, "x.git")
	if err := InitBare(bare); err != nil {
		t.Fatal(err)
	}
	if err := r.SetRemotes([]string{bare}); err != nil {
		t.Fatal(err)
	}
	if err := r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	if ok, err := HeadHasFile(bare, "a.txt"); err != nil || !ok {
		t.Errorf("a.txt = %v, %v", ok, err)
	}
	if ok, _ := HeadHasFile(bare, "nope"); ok {
		t.Error("nope is there")
	}
	empty := filepath.Join(d, "empty.git")
	_ = InitBare(empty)
	if ok, err := HeadHasFile(empty, "a.txt"); err != nil || ok {
		t.Errorf("empty = %v, %v", ok, err)
	}
}
```

同じファイルの末尾に足す（`commitFiles` は `tree_test.go` の既存のヘルパー）:

```go
func initWithCommit(t *testing.T, dir string) *Repo {
	t.Helper()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	commitFiles(t, r, dir, map[string]string{"a.txt": "a"})
	return r
}

func commitFile(t *testing.T, r *Repo, name, content string) {
	t.Helper()
	commitFiles(t, r, r.dir, map[string]string{name: content})
}
```

- [ ] **Step 3: 失敗を確かめる**

Run: `go test ./internal/gitrepo/ -run 'TestRewriteFetchPush|TestHTTPSAuthAndDenied|TestLocalPathAndWritable|TestHeadHasFile'`
Expected: FAIL（`Conn`、`SetConn` などが未定義）

- [ ] **Step 4: conn.go を書き、remote.go を直す**

`internal/gitrepo/conn.go`:

```go
package gitrepo

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"
)

// ErrPushDenied means the remote refused the push for want of the right to
// write: a 403, or a local repository that cannot be written.
var ErrPushDenied = errors.New("the remote denied the push")

// Conn is how a fetch, push or clone reaches a remote. Rewrite turns the
// URL of .git/config into the one to reach (url_rewrites); Auth gives the
// auth for the rewritten URL. Both may be nil.
type Conn struct {
	Rewrite func(url string) string
	Auth    func(url string) (transport.AuthMethod, error)
}

// endpoint is the URL to reach for u and its auth.
func (c Conn) endpoint(u string) (string, transport.AuthMethod, error) {
	if c.Rewrite != nil {
		u = c.Rewrite(u)
	}
	if c.Auth == nil {
		return u, nil, nil
	}
	a, err := c.Auth(u)
	return u, a, err
}

// SetConn makes the fetches and pushes of r go through c.
func (r *Repo) SetConn(c Conn) { r.conn = c }

// remoteURL is the URL of the remote name as .git/config has it.
func (r *Repo) remoteURL(name string) (string, error) {
	cfg, err := r.r.Config()
	if err != nil {
		return "", err
	}
	rc, ok := cfg.Remotes[name]
	if !ok || len(rc.URLs) == 0 {
		return "", fmt.Errorf("no remote %s", name)
	}
	return rc.URLs[0], nil
}

// pushDenied reports whether err is a refusal to write.
func pushDenied(err error) bool {
	return errors.Is(err, transport.ErrAuthorizationFailed) || errors.Is(err, fs.ErrPermission) ||
		strings.Contains(strings.ToLower(err.Error()), "permission denied")
}

// LocalPath returns the local path of a remote that is one: a path, or a
// file:// URL.
func LocalPath(u string) (string, bool) {
	if p, ok := strings.CutPrefix(u, "file://"); ok {
		return filepath.FromSlash(localPath(p)), true
	}
	if filepath.IsAbs(u) || driveRE.MatchString("/"+u) {
		return u, true
	}
	return "", false
}

// IsHTTP reports whether u is an http or https URL.
func IsHTTP(u string) bool {
	p, err := url.Parse(u)
	return err == nil && (p.Scheme == "http" || p.Scheme == "https") && p.Host != ""
}

// Writable reports whether a file can be made and removed in dir/objects,
// which a push to the bare repository dir needs.
func Writable(dir string) bool {
	f, err := os.CreateTemp(filepath.Join(dir, "objects"), ".hedgehug-write-test-*") //nolint:gosec // G703: dir is the bare repository of a remote in config.yaml
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name) == nil //nolint:gosec // G703: the file made just above
}

// RemoteEmpty reports whether the remote at url has no ref at all, listing
// its refs through c (url_rewrites and auth, as for a fetch). A remote the
// server calls empty, and one that lists nothing, are both empty; a
// repository that is not there, or a failed connection, is an error.
func RemoteEmpty(u string, c Conn) (bool, error) {
	to, auth, err := c.endpoint(u)
	if err != nil {
		return false, fmt.Errorf("list the refs of %s: %w", u, err)
	}
	rm := git.NewRemote(memory.NewStorage(), &gitconfig.RemoteConfig{Name: "origin", URLs: []string{to}})
	refs, err := rm.List(&git.ListOptions{Auth: auth})
	if errors.Is(err, transport.ErrEmptyRemoteRepository) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("list the refs of %s: %w", u, err)
	}
	return len(refs) == 0, nil
}

// HeadHasFile reports whether the tree of HEAD of the repository at dir,
// bare or not, has the file p. A repository with no commit has none.
func HeadHasFile(dir, p string) (bool, error) {
	r, err := git.PlainOpen(dir)
	if err != nil {
		return false, err
	}
	ref, err := r.Head()
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	c, err := r.CommitObject(ref.Hash())
	if err != nil {
		return false, err
	}
	_, err = c.File(p)
	if err != nil {
		return false, nil //nolint:nilerr // a missing file is the answer
	}
	return true, nil
}
```

`internal/gitrepo/repo.go` の `Repo` に `conn Conn` のフィールドを足す。

`remote.go` の `Fetch`:

```go
func (r *Repo) Fetch(remote string) (plumbing.Hash, error) {
	u, err := r.remoteURL(remote)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	to, auth, err := r.conn.endpoint(u)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("fetch %s: %w", remote, err)
	}
	err = r.r.Fetch(&git.FetchOptions{RemoteName: remote, RemoteURL: to, Auth: auth})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) && !errors.Is(err, transport.ErrEmptyRemoteRepository) {
		return plumbing.ZeroHash, fmt.Errorf("fetch %s: %w", remote, err)
	}
	return r.RemoteHead(remote)
}
```

`Push` の `r.r.Push(…)` の前で同じく `u, to, auth` を求めて `PushOptions` に `RemoteURL: to, Auth: auth` を足し、エラーの分岐の `ErrRejected` の後に:

```go
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) && pushDenied(err) {
		return fmt.Errorf("push %s: %w: %w", remote, ErrPushDenied, err)
	}
```

`Clone` を `CloneWith` に置き換える:

```go
// Clone is CloneWith with no rewrite and no auth.
func Clone(url, dir string) (*Repo, error) { return CloneWith(url, dir, Conn{}) }

// CloneWith clones the main branch of url into dir through c; .git/config
// keeps url as given. dir must be missing or an empty directory; anything
// else is refused untouched. A failed clone removes what it created: the
// directory and its missing parents, or only its contents when dir existed.
func CloneWith(url, dir string, c Conn) (*Repo, error) {
	created, err := checkCloneTarget(dir)
	if err != nil {
		return nil, fmt.Errorf("clone %s: %w", url, err)
	}
	to, auth, err := c.endpoint(url)
	if err != nil {
		return nil, fmt.Errorf("clone %s: %w", url, err)
	}
	r, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:           to,
		Auth:          auth,
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName("main"),
		SingleBranch:  true,
	})
	if err == nil {
		repo := &Repo{dir: dir, r: r, conn: c}
		if err = afterClone(repo, url); err == nil {
			return repo, nil
		}
	} else {
		err = fmt.Errorf("clone %s: %w", url, err)
	}
	removeCreated(dir, created)
	return nil, err
}
```

（`afterClone` は `SetRemotes([]string{url})` で、書き換える前の URL を `.git/config` に戻す。今のままでよい。）

- [ ] **Step 5: gitrepo のテストを通す**

Run: `go test -race ./internal/gitrepo/...`
Expected: PASS

- [ ] **Step 6: sync と cmd に Conn を通す**

`internal/sync/sync.go` の `Options` に足す:

```go
	Conn       gitrepo.Conn // how the remotes are reached: url_rewrites and the token
```

`Run` の `r := s.Repo()` の直後に `r.SetConn(opt.Conn)` を置く。

`cmd/hedgehug/main.go` の `env` に足す:

```go
	keyring    auth.Keyring  // nil is the OS keychain
	httpClient *http.Client  // nil is a client with a 30-second timeout
	openURL    func(string) error // opens a URL in the browser; Task 8 sets it in main
	run        func(name string, args ...string) (stdout, stderr []byte, err error) // nil runs the command
```

`cmd/hedgehug/conn.go`:

```go
package main

import (
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/kukv/hedgehug/internal/auth"
	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
)

// authOptions is what getting the token of acct needs.
func (a *app) authOptions(c *config.Config, acct config.Account) auth.Options {
	return auth.Options{
		Getenv: a.getenv, Run: a.run, Keyring: a.keyring, Now: a.now, HTTP: a.httpClient,
		Host:     c.Host(acct.Host),
		LockFile: filepath.Join(filepath.Dir(c.File()), "auth.lock"),
	}
}

// tokenFor returns the token of acct, getting it once per run.
func (a *app) tokenFor(c *config.Config, acct config.Account) (string, error) {
	if tok, ok := a.tokens[acct.Name]; ok {
		return tok, nil
	}
	tok, err := auth.Token(acct, a.authOptions(c, acct))
	if err != nil {
		return "", err
	}
	if a.tokens == nil {
		a.tokens = map[string]string{}
	}
	a.tokens[acct.Name] = tok
	return tok, nil
}

// connFor is how the remotes of the store e are reached: through
// url_rewrites, and with the token of its account over http and https.
func (a *app) connFor(c *config.Config, e config.StoreEntry) gitrepo.Conn {
	acct, ok := c.Writer(e)
	return gitrepo.Conn{
		Rewrite: c.Rewrite,
		Auth: func(u string) (transport.AuthMethod, error) {
			if !ok || acct.Token.Kind == "" || !gitrepo.IsHTTP(u) {
				return nil, nil
			}
			tok, err := a.tokenFor(c, acct)
			if err != nil {
				return nil, err
			}
			return &githttp.BasicAuth{Username: "x-access-token", Password: tok}, nil
		},
	}
}
```

SSH の remote（`ssh://` と `user@host:path`）には `Auth` で nil を返す。go-git は SSH の URL で auth が nil なら ssh-agent を使い、`~/.ssh/known_hosts` でホスト鍵を確かめる（spec の「SSH は go-git の SSH で ssh-agent を使い、`known_hosts` を確かめる」は go-git の既定のとおりで、コードは要らない）。SSH のサーバーを立てられないので、SSH の通信のテストは書かない。

`config.Config` に `func (c *Config) File() string { return c.file }` を足す（Task 1 のファイルに追記）。`app` に `tokens map[string]string` を足す。

`app.syncOptions` の `opt := sync.Options{…}` に `Conn: a.connFor(c, e)` を足す。

`store_add.go` の `gitrepo.Clone(remotes[0], dir)` を、`e := config.StoreEntry{Name: name, Path: dir, Remotes: remotes, Account: account}` を先に作ってから `gitrepo.CloneWith(remotes[0], dir, a.connFor(c, e))` にする（`account` は Task 11 で足す引数。この Task では `""`）。

`tools/devenv/main.go` の `sync.Run` と `gitrepo.Clone` は、この Task では変えない（Task 13 で書き換えに移す）。

- [ ] **Step 7: cmd の失敗するテストを書く**

`cmd/hedgehug/conn_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
)

func TestSyncThroughRewrite(t *testing.T) {
	te := newTestEnv(t)
	remotes := filepath.Join(t.TempDir(), "remotes")
	if err := gitrepo.InitBare(filepath.Join(remotes, "org-a", "x.git")); err != nil {
		t.Fatal(err)
	}
	te.writeConfigYAML("url_rewrites:\n  - {from: \"https://github.com/\", to: \"" + filepath.ToSlash(remotes) + "/\"}\n")
	te.ok("store", "init", "me", "--prefix", "ME", "--remote", "https://github.com/org-a/x.git")
	te.ok("add", "one")
	te.ok("sync")
	if ok, err := gitrepo.HeadHasFile(filepath.Join(remotes, "org-a", "x.git"), ".hedgehug/store.yaml"); err != nil || !ok {
		t.Errorf("the remote has no store: %v %v", ok, err)
	}
	b, _ := os.ReadFile(filepath.Join(te.storeDir("me"), ".git", "config"))
	if !strings.Contains(string(b), "https://github.com/org-a/x.git") {
		t.Errorf(".git/config = %s", b)
	}
}

func TestSyncSendsToken(t *testing.T) {
	te := newTestEnv(t)
	d := t.TempDir()
	if err := gitrepo.InitBare(filepath.Join(d, "org-a", "x.git")); err != nil {
		t.Fatal(err)
	}
	var auths []transport.AuthMethod
	// Not a GitHub host: store init (Task 9) makes no repository there.
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://git.example.com/", d), &auths, nil)
	te.env["HH_TOKEN"] = "tok-1"
	te.writeConfigYAML("accounts:\n  - {name: me, host: git.example.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	te.ok("store", "init", "me", "--prefix", "ME", "--remote", "https://git.example.com/org-a/x.git")
	if len(auths) == 0 {
		t.Fatal("no git session")
	}
	for _, a := range auths {
		if b, ok := a.(*githttp.BasicAuth); !ok || b.Password != "tok-1" || b.Username != "x-access-token" {
			t.Errorf("auth = %#v", a)
		}
	}
}
```

`helpers_test.go` に足す:

```go
// writeConfigYAML puts s at the start of config.yaml.
func (te *testEnv) writeConfigYAML(s string) {
	te.t.Helper()
	if err := os.MkdirAll(te.hhHome, 0o750); err != nil {
		te.t.Fatal(err)
	}
	p := filepath.Join(te.hhHome, "config.yaml")
	old, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		te.t.Fatal(err)
	}
	if err := os.WriteFile(p, append([]byte(s), old...), 0o600); err != nil {
		te.t.Fatal(err)
	}
}
```

`TestMain`（helpers_test.go）の最初で `keyring.MockInit()`（`github.com/zalando/go-keyring`）を呼び、テストが本物の keychain に触れないようにする。

- [ ] **Step 8: テストを通す**

Run: `go test -race ./internal/... ./cmd/hedgehug/`
Expected: PASS（既存の同期のテストも通る）

- [ ] **Step 9: コミット**

```bash
git add internal/gitrepo internal/sync/sync.go internal/config cmd/hedgehug
git commit -m "feat(gitrepo): url_rewrites と HTTPS のトークンで通信し、push の拒否を見分ける"
```

---

### Task 8: `hh auth login/status/logout`

> **置き換え済み（2026-11-01）**: Task 8 の修正と Task 9〜14 は、フェーズ4a の後のコードと設計の見直しに合わせて `docs/superpowers/plans/2026-11-01-phase3a-rest-after-4a.md` で置き換えた。以下の Task 8〜14 のテストとコードは、新しい plan が「旧 Task N のコードを使う」と書いたところで参照する。冒頭の「spec にない細部の判断」1〜29 は spec に移した。

**Files:**
- Create: `cmd/hedgehug/auth.go`
- Modify: `cmd/hedgehug/app.go`（`rootCmd` に `a.authCmd()`）、`cmd/hedgehug/main.go`（`openURL`）
- Test: `cmd/hedgehug/auth_test.go`

**Interfaces:**
- Consumes: `auth.DeviceLogin`、`auth.SaveToken`、`auth.SaveDeviceToken`、`auth.Delete`、`auth.LoadDeviceToken`、`auth.Token`、`auth.Options.Keychain`（Task 2・5）、`github.New`、`(*github.Client).User`、`github.Describe`（Task 4）、`config.AddAccount`、`SetAccountToken`、`Account`（Task 1）、`a.authOptions`、`a.tokenFor`（Task 7）
- Produces: `hh auth login [<name>] [--host h] [--with-token]`、`hh auth status`、`hh auth logout <name>`（spec の「`hh auth`」の表と「spec にない細部の判断」12・13。config にあり `token` が空のアカウントに `hh auth login <name>` を実行したときは、Device Flow（`--with-token` なら標準入力のトークンを keychain へ）でトークンを取り、`GET /user` の login が config と合えば `token: device`（または `keychain`）を書く。合わなければ何も書かずにエラー。spec の表のとおり）
- `--json`: login は `{"account": name, "id": "<host>/<login>", "added": bool}`、status は配列 `[{"name","id","token","ok","login","expires","error"}]`、logout は `{"account": name, "removed": bool}`

- [ ] **Step 1: 失敗するテストを書く**

`cmd/hedgehug/auth_test.go`:

```go
package main

import (
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/kukv/hedgehug/internal/fakegh"
)

// withFake starts a fakegh with alice and bob, and points github.com of the
// config at it.
func (te *testEnv) withFake(st fakegh.State) *fakegh.Server {
	te.t.Helper()
	zero := 0
	if st.DeviceInterval == nil {
		st.DeviceInterval = &zero
	}
	if st.Users == nil {
		st.Users = []string{"alice", "bob"}
	}
	s := fakegh.New(st)
	srv := httptest.NewServer(s)
	te.t.Cleanup(srv.Close)
	te.writeConfigYAML("hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\", client_id: Iv23test}\n")
	return s
}

func TestAuthLoginDevice(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.withFake(fakegh.State{AutoApprove: "alice"})
	_, out, errs := te.run("auth", "login")
	if !strings.Contains(errs, "enter the code") {
		t.Errorf("stderr = %q", errs)
	}
	if !strings.Contains(out, "Logged in as github.com/alice") {
		t.Errorf("stdout = %q", out)
	}
	b, _ := os.ReadFile(filepath.Join(te.hhHome, "config.yaml"))
	if !strings.Contains(string(b), "login: alice") || !strings.Contains(string(b), "token: device") {
		t.Errorf("config.yaml = %s", b)
	}
	if out := te.ok("auth", "status"); !strings.Contains(out, "alice") || !strings.Contains(out, "device") || !strings.Contains(out, "ok") {
		t.Errorf("status = %q", out)
	}
	te.ok("auth", "logout", "alice")
	if code, _, _ := te.run("auth", "status"); code != 1 {
		t.Errorf("status after logout: exit %d", code)
	}
}

func TestAuthLoginWithToken(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.withFake(fakegh.State{Tokens: map[string]string{"t-bob": "bob"}})
	te.stdin = "t-bob\n"
	out := te.ok("auth", "login", "work", "--with-token")
	if !strings.Contains(out, "github.com/bob") {
		t.Errorf("stdout = %q", out)
	}
	b, _ := os.ReadFile(filepath.Join(te.hhHome, "config.yaml"))
	if !strings.Contains(string(b), "name: work") || !strings.Contains(string(b), "token: keychain") {
		t.Errorf("config.yaml = %s", b)
	}
	if v, _ := keyring.Get("hedgehug", "github.com/bob"); v != "t-bob" {
		t.Errorf("keychain = %q", v)
	}
	// The same host and login again replaces the token, and adds no account.
	te.stdin = "t-bob\n"
	te.ok("auth", "login", "other", "--with-token")
	b, _ = os.ReadFile(filepath.Join(te.hhHome, "config.yaml"))
	if strings.Count(string(b), "login: bob") != 1 {
		t.Errorf("config.yaml = %s", b)
	}
}

func TestAuthStatusMismatch(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.withFake(fakegh.State{Tokens: map[string]string{"t-bob": "bob"}})
	te.env["HH_TOKEN"] = "t-bob"
	te.writeConfigYAML("accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n  - {name: plain, host: github.com, login: x}\n")
	code, out, _ := te.run("auth", "status")
	if code != 1 || !strings.Contains(out, "bob") || !strings.Contains(out, "plain") {
		t.Errorf("exit %d, %q", code, out)
	}
	if out := te.ok("auth", "login", "me"); !strings.Contains(out, "Nothing to log in") {
		t.Errorf("env account login = %q", out)
	}
}

// An account in config.yaml with no token: (every account of phase 2 is one)
// gets its token from auth login, once GitHub says the token is of its login.
func TestAuthLoginFillsEmptyToken(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.withFake(fakegh.State{AutoApprove: "alice"})
	te.writeConfigYAML("accounts:\n  - {name: work, host: github.com, login: alice}\n")
	out := te.ok("auth", "login", "work")
	if !strings.Contains(out, "Logged in as github.com/alice (account work)") {
		t.Errorf("stdout = %q", out)
	}
	b, _ := os.ReadFile(filepath.Join(te.hhHome, "config.yaml"))
	if strings.Count(string(b), "login: alice") != 1 || !strings.Contains(string(b), "token: device") {
		t.Errorf("config.yaml = %s", b)
	}
	if v, err := keyring.Get("hedgehug", "github.com/alice"); err != nil || !strings.Contains(v, "access_token") {
		t.Errorf("keychain = %q, %v", v, err)
	}
	if out := te.ok("auth", "status"); !strings.Contains(out, "device") || !strings.Contains(out, "ok") {
		t.Errorf("status = %q", out)
	}
}

// A token of another login is refused, and nothing is written.
func TestAuthLoginEmptyTokenOfAnotherLogin(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.withFake(fakegh.State{AutoApprove: "bob", Tokens: map[string]string{"t-alice": "alice", "t-bob": "bob"}})
	te.writeConfigYAML("accounts:\n  - {name: work, host: github.com, login: alice}\n")
	noToken := func(what string) {
		t.Helper()
		b, _ := os.ReadFile(filepath.Join(te.hhHome, "config.yaml"))
		if strings.Contains(string(b), "token:") {
			t.Errorf("%s: config.yaml has a token:\n%s", what, b)
		}
		for _, k := range []string{"github.com/alice", "github.com/bob"} {
			if _, err := keyring.Get("hedgehug", k); !errors.Is(err, keyring.ErrNotFound) {
				t.Errorf("%s: the keychain has %s: %v", what, k, err)
			}
		}
	}
	code, _, errs := te.run("auth", "login", "work") // the device flow approves bob
	if code != 1 || !strings.Contains(errs, "github.com/bob") || !strings.Contains(errs, "account work") {
		t.Errorf("device flow: exit %d, %q", code, errs)
	}
	noToken("device flow")
	te.stdin = "t-bob\n"
	code, _, errs = te.run("auth", "login", "work", "--with-token")
	if code != 1 || !strings.Contains(errs, "github.com/bob") {
		t.Errorf("--with-token: exit %d, %q", code, errs)
	}
	noToken("--with-token")
	// The right token is kept in the keychain.
	te.stdin = "t-alice\n"
	te.ok("auth", "login", "work", "--with-token")
	b, _ := os.ReadFile(filepath.Join(te.hhHome, "config.yaml"))
	if !strings.Contains(string(b), "token: keychain") {
		t.Errorf("config.yaml = %s", b)
	}
	if v, _ := keyring.Get("hedgehug", "github.com/alice"); v != "t-alice" {
		t.Errorf("keychain = %q", v)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./cmd/hedgehug/ -run TestAuth`
Expected: FAIL（`unknown command "auth"`）

- [ ] **Step 3: auth.go を書く**

```go
package main

import (
	"bufio"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/auth"
	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/github"
)

func (a *app) authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage the accounts and their tokens",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	var host string
	var withToken bool
	login := &cobra.Command{
		Use:   "login [<name>]",
		Short: "Log in with the device flow, or keep a token given on stdin",
		Args:  asUsage(cobra.MaximumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return a.authLogin(name, host, withToken)
		},
	}
	login.Flags().StringVar(&host, "host", "github.com", "the host of a new account")
	login.Flags().BoolVar(&withToken, "with-token", false, "read a token from stdin and keep it in the keychain")
	status := &cobra.Command{
		Use:   "status",
		Short: "Check the token of each account",
		Args:  cobra.NoArgs,
		RunE:  func(_ *cobra.Command, _ []string) error { return a.authStatus() },
	}
	logout := &cobra.Command{
		Use:   "logout <name>",
		Short: "Remove the token of an account from the keychain",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.authLogout(args[0]) },
	}
	cmd.AddCommand(login, status, logout)
	return cmd
}

func (a *app) authLogin(name, host string, withToken bool) error {
	c, _, err := a.config()
	if err != nil {
		return err
	}
	acct, known := c.Account(name)
	if known {
		// An account with no token: falls through to the device flow, or
		// to --with-token, which gives it one.
		switch acct.Token.Kind {
		case "gh", "env":
			_, err := fmt.Fprintf(a.stdout, "Nothing to log in for %s: it uses %s\n", name, acct.Token.String())
			return err
		case "keychain":
			if !withToken {
				return usageErrorf("account %s keeps a token in the keychain; give it on stdin with --with-token", name)
			}
		case "device":
			if withToken {
				return usageErrorf("account %s logs in with the device flow; leave out --with-token", name)
			}
		}
		host = acct.Host
	}
	h := c.Host(host)
	var tok string
	var dt auth.DeviceToken
	kind := "device"
	if withToken {
		kind = "keychain"
		line, err := bufio.NewReader(a.stdin).ReadString('\n')
		if tok = strings.TrimSpace(line); tok == "" {
			return errors.Join(errors.New("no token on stdin"), err)
		}
	} else {
		dt, err = auth.DeviceLogin(h, a.authOptions(c, config.Account{Host: host}), a.showDeviceCode)
		if err != nil {
			return err
		}
		tok = dt.AccessToken
	}
	login, err := github.New(h, tok, a.httpClient).User()
	if err != nil {
		return fmt.Errorf("check the token: %w", err)
	}
	if !known {
		// An account of the same host and login takes the token instead.
		for _, x := range c.Accounts {
			if x.Host == host && x.Login == login {
				acct, known = x, true
			}
		}
	}
	added := false
	switch {
	case known:
		if acct.Login != login {
			return fmt.Errorf("the token is of %s/%s, not of account %s (%s)", host, login, acct.Name, auth.Key(acct))
		}
		if err := c.SetAccountToken(acct.Name, config.TokenSource{Kind: kind}); err != nil {
			return err
		}
		acct.Token = config.TokenSource{Kind: kind}
	default:
		if name == "" {
			name = login
		}
		acct = config.Account{Name: name, Host: host, Login: login, Token: config.TokenSource{Kind: kind}}
		if err := c.AddAccount(acct); err != nil {
			return err
		}
		added = true
	}
	k := a.authOptions(c, acct).Keychain()
	if kind == "device" {
		err = auth.SaveDeviceToken(acct, dt, k)
	} else {
		err = auth.SaveToken(acct, tok, k)
	}
	if err != nil {
		return err
	}
	if err := c.Save(); err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(struct {
			Account string `json:"account"`
			ID      string `json:"id"`
			Added   bool   `json:"added"`
		}{acct.Name, acct.ID(), added})
	}
	_, err = fmt.Fprintf(a.stdout, "Logged in as %s (account %s)\n", acct.ID(), acct.Name)
	return err
}

// showDeviceCode tells where to enter the code, and on a terminal offers to
// open the browser there.
func (a *app) showDeviceCode(dc *github.DeviceCode) {
	_, _ = fmt.Fprintf(a.stderr, "Open %s and enter the code %s\n", dc.VerificationURI, dc.UserCode)
	if !a.isTerminal(a.stdin) || a.openURL == nil {
		return
	}
	_, _ = fmt.Fprint(a.stderr, "Open the browser? [Y/n] ")
	line, _ := bufio.NewReader(a.stdin).ReadString('\n')
	if ans := strings.ToLower(strings.TrimSpace(line)); ans == "" || ans == "y" || ans == "yes" {
		if err := a.openURL(dc.VerificationURI); err != nil {
			_, _ = fmt.Fprintln(a.stderr, "warning: the browser did not open:", err)
		}
	}
}

type authStatusJSON struct {
	Name    string     `json:"name"`
	ID      string     `json:"id"`
	Token   string     `json:"token"`
	OK      bool       `json:"ok"`
	Login   string     `json:"login,omitempty"` // what GitHub says the token is of
	Expires *time.Time `json:"expires,omitempty"`
	Error   string     `json:"error,omitempty"`
}

func (a *app) authStatus() error {
	c, _, err := a.config()
	if err != nil {
		return err
	}
	out := make([]authStatusJSON, len(c.Accounts))
	bad := false
	for i, acct := range c.Accounts {
		x := authStatusJSON{Name: acct.Name, ID: acct.ID(), Token: acct.Token.String(), OK: true}
		if acct.Token.Kind == "" {
			out[i] = x
			continue
		}
		tok, err := a.tokenFor(c, acct)
		if err == nil {
			x.Login, err = github.New(c.Host(acct.Host), tok, a.httpClient).User()
			err = github.Describe(err, acct.ID(), acct.Name, "check the token")
		}
		switch {
		case err != nil:
			x.OK, x.Error = false, err.Error()
		case x.Login != acct.Login:
			x.OK, x.Error = false, fmt.Sprintf("the token is of %s/%s", acct.Host, x.Login)
		}
		if acct.Token.Kind == "device" {
			if dt, err := auth.LoadDeviceToken(acct, a.authOptions(c, acct).Keychain()); err == nil {
				x.Expires = timePtr(dt.ExpiresAt)
			}
		}
		bad = bad || !x.OK
		out[i] = x
	}
	if a.jsonOut {
		if err := a.printJSON(out); err != nil {
			return err
		}
	} else {
		rows := make([][]string, len(out))
		for i, x := range out {
			token, state := x.Token, "ok"
			if token == "" {
				token, state = "-", "-"
			} else if !x.OK {
				state = x.Error
			}
			rows[i] = []string{x.Name, x.ID, token, expiryText(x.Expires), state}
		}
		if len(rows) == 0 {
			_, err := fmt.Fprintln(a.stdout, "No account is set up; run hedgehug auth login")
			return err
		}
		if err := writeTable(a.stdout, []string{"NAME", "ACCOUNT", "TOKEN", "EXPIRES", "STATE"}, rows, nil); err != nil {
			return err
		}
	}
	if bad {
		return errFindings
	}
	return nil
}

func (a *app) authLogout(name string) error {
	c, _, err := a.config()
	if err != nil {
		return err
	}
	acct, ok := c.Account(name)
	if !ok {
		return fmt.Errorf("no account is named %q", name)
	}
	removed := false
	if acct.Token.Kind == "keychain" || acct.Token.Kind == "device" {
		if err := auth.Delete(acct, a.authOptions(c, acct).Keychain()); err != nil {
			return err
		}
		removed = true
	}
	if a.jsonOut {
		return a.printJSON(struct {
			Account string `json:"account"`
			Removed bool   `json:"removed"`
		}{name, removed})
	}
	if !removed {
		_, err = fmt.Fprintf(a.stdout, "Hedgehug keeps nothing for %s: it uses %s\n", name, acct.Token.String())
		return err
	}
	_, err = fmt.Fprintf(a.stdout, "Removed the token of %s from the keychain\n", acct.ID())
	return err
}
```

同じファイルに、期限のないものに `-` を出す関数を足す（`timeText(nil)` は `never` を返すので、表では使わない）:

```go
// expiryText is the expiry for the table: - for a token that has none.
func expiryText(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return timeText(t)
}
```

（`errFindings` は doctor と同じく「出力は済んだので終了コード1」の印。）

`rootCmd` の `root.AddCommand(…)` に `a.authCmd()` を足す。`main.go` の `main` で `openURL: openURL` を渡し、同じファイルに:

```go
// openURL opens u in the browser of the OS.
func openURL(u string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u) //nolint:gosec // G204: the command is fixed; u is the verification URL the host gave
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u) //nolint:gosec // G204: the command is fixed; u is the verification URL the host gave
	default:
		cmd = exec.Command("xdg-open", u) //nolint:gosec // G204: the command is fixed; u is the verification URL the host gave
	}
	return cmd.Start()
}
```

- [ ] **Step 4: テストを通す**

Run: `go test -race ./cmd/hedgehug/ -run TestAuth`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add cmd/hedgehug
git commit -m "feat(cli): hh auth login/status/logout を作る"
```

---

### Task 9: `store init` の remote の用意（ローカルの bare・GitHub）と `--account`・`--path`

**Files:**
- Modify: `cmd/hedgehug/store_init.go`
- Create: `cmd/hedgehug/github.go`（`githubFor`・`pickAccount`・`accountNames`）
- Test: `cmd/hedgehug/store_init_test.go`

**Interfaces:**
- Consumes: `config.Config.Rewrite`、`IsGitHubHost`、`Host`、`StoreEntry`、`SetStoreAccount`（Task 1 と既存）、`context.Split`（Task 4。remote URL を読む唯一の関数）、`(*github.Client).CreateRepo`、`SetTopics`、`ErrExists`、`ErrForbidden`、`github.Describe`（Task 4）、`gitrepo.LocalPath`、`InitBare`、`RemoteEmpty`、`gitrepotest.InstallHTTPS`（Task 7）、`a.tokenFor`、`a.connFor`（Task 7）
- Produces:
  - `func (a *app) githubFor(c *config.Config, acct config.Account, host string) (*github.Client, error)`
  - `func (a *app) pickAccount(c *config.Config, flag string) (config.Account, bool, error)`（`--account` か、アカウントが1つならそれ）
  - `store init` のフラグ `--account`、`--path`
  - `store init` は、アカウントが決まったら（`--account`、またはアカウントが1つ）、そのアカウントの名前を `stores[].account` に書く（spec の「選んだアカウントを `stores[].account` に書く」）。決まらなければ書かない
  - GitHub のホストの remote の用意（spec の「Store の作成と発見」の「GitHub のホスト」）: `https`・`ssh://`・`user@host:owner/repo` のどの形でも、書き換え後の URL の host が `github.com` か `hosts[]` の host なら GitHub の remote。private のリポジトリを作って topic `hedgehug-store` を付ける。作成が 422（`ErrExists`）のときは、`gitrepo.RemoteEmpty` で remote に ref が1つもないかを git の通信で確かめる。空なら作成を飛ばして topic を付け、初回の push へ進む。中身があればエラーで `hedgehug store add` を案内する

**テストでの「既存の空のリポジトリ」と「中身のあるリポジトリ」の置き方:** fakegh は API だけに答え、git の通信はしない。そこで、GitHub 側の「リポジトリがある」は fakegh の `State.Repos` に置いて（作成が 422 になる）、git 側の「空」「中身あり」は `gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), nil, nil)` で同じ owner/name の bare リポジトリ（`d/org-a/hh-empty.git`、`d/org-a/hh-full.git`）に向けて置く。空は `gitrepo.InitBare` しただけの bare、中身ありは別の `testEnv` で `store init --remote <その bare のパス>` して push したもの。`RemoteEmpty` はこの transport を通して ref を取る。

- [ ] **Step 1: 失敗するテストを書く**

`cmd/hedgehug/store_init_test.go` に足す（import に `net/http/httptest`、`os`、`path/filepath`、`strings`、`internal/fakegh`、`internal/gitrepo`、`internal/gitrepo/gitrepotest` のうち、まだないものを足す）:

```go
func TestStoreInitMakesLocalBare(t *testing.T) {
	te := newTestEnv(t)
	remotes := filepath.Join(t.TempDir(), "remotes")
	te.writeConfigYAML("url_rewrites:\n  - {from: \"https://github.com/\", to: \"" + filepath.ToSlash(remotes) + "/\"}\n")
	te.ok("store", "init", "me", "--prefix", "ME", "--remote", "https://github.com/kukv/hh-tasks.git")
	if ok, err := gitrepo.HeadHasFile(filepath.Join(remotes, "kukv", "hh-tasks.git"), ".hedgehug/store.yaml"); err != nil || !ok {
		t.Errorf("the bare was not made and pushed: %v %v", ok, err)
	}
}

// The only account is the one that writes the store, so it is the one written
// to stores[].account, with or without --account.
func TestStoreInitWritesTheAccount(t *testing.T) {
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	b, _ := os.ReadFile(filepath.Join(te.hhHome, "config.yaml"))
	if !strings.Contains(string(b), "account: alice") {
		t.Errorf("the only account is not written:\n%s", b)
	}
	te.ok("store", "init", "other", "--prefix", "OT", "--account", "alice")
	b, _ = os.ReadFile(filepath.Join(te.hhHome, "config.yaml"))
	if strings.Count(string(b), "account: alice") != 2 {
		t.Errorf("--account is not written:\n%s", b)
	}
}

func TestStoreInitOnGitHub(t *testing.T) {
	te := newTestEnv(t)
	d := t.TempDir()
	fake := te.withFake(fakegh.State{Orgs: map[string][]string{"org-a": {"alice"}}, Tokens: map[string]string{"t-alice": "alice"}})
	te.env["HH_TOKEN"] = "t-alice"
	te.writeConfigYAML("accounts:\n  - {name: work, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	// The fake creates no git repository; the test serves the push from a bare.
	if err := gitrepo.InitBare(filepath.Join(d, "org-a", "hh-new.git")); err != nil {
		t.Fatal(err)
	}
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), nil, nil)
	te.ok("store", "init", "org", "--prefix", "ORG", "--remote", "https://github.com/org-a/hh-new.git", "--path", filepath.Join(d, "clone"))
	r, ok := fake.Repo("org-a/hh-new")
	if !ok || !r.Private || len(r.Topics) != 1 || r.Topics[0] != "hedgehug-store" {
		t.Errorf("repo = %+v %v", r, ok)
	}
	b, _ := os.ReadFile(filepath.Join(te.hhHome, "config.yaml"))
	if !strings.Contains(string(b), "account: work") || !strings.Contains(string(b), filepath.ToSlash(filepath.Join(d, "clone"))) {
		t.Errorf("config.yaml = %s", b)
	}
	// Again: the repository exists, and now it has the store in it.
	code, _, errs := te.run("store", "init", "org2", "--prefix", "ORGB", "--remote", "https://github.com/org-a/hh-new.git")
	if code != 1 || !strings.Contains(errs, "not empty") || !strings.Contains(errs, "hedgehug store add org2") {
		t.Errorf("exists with content: exit %d, %q", code, errs)
	}
	if _, err := os.Stat(te.storeDir("org2")); !os.IsNotExist(err) {
		t.Errorf("the store directory of the failed init is left: %v", err)
	}
}

// A repository on GitHub with no ref (an empty one that a person made by
// hand, say) is used as it is: the creation is skipped, the topic is set, and
// the store is pushed. One with a commit is not touched.
func TestStoreInitOnAnExistingRepo(t *testing.T) {
	te := newTestEnv(t)
	d := t.TempDir()
	// The API side of the two repositories is in the state of the fake, so that
	// creating them is a 422; the git side is a bare each, which the test serves.
	fake := te.withFake(fakegh.State{
		Orgs:   map[string][]string{"org-a": {"alice"}},
		Tokens: map[string]string{"t-alice": "alice"},
		Repos: []*fakegh.Repo{
			{Owner: "org-a", Name: "hh-empty", Private: true, Perms: map[string]string{"alice": "admin"}},
			{Owner: "org-a", Name: "hh-full", Private: true, Perms: map[string]string{"alice": "admin"}},
		},
	})
	te.env["HH_TOKEN"] = "t-alice"
	te.writeConfigYAML("accounts:\n  - {name: work, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	empty := filepath.Join(d, "org-a", "hh-empty.git")
	if err := gitrepo.InitBare(empty); err != nil {
		t.Fatal(err)
	}
	maker := newTestEnv(t)
	maker.ok("store", "init", "full", "--prefix", "FULL", "--remote", filepath.Join(d, "org-a", "hh-full.git"))
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), nil, nil)

	te.ok("store", "init", "org", "--prefix", "ORG", "--remote", "https://github.com/org-a/hh-empty.git")
	if r, _ := fake.Repo("org-a/hh-empty"); len(r.Topics) != 1 || r.Topics[0] != "hedgehug-store" {
		t.Errorf("the topic of the empty repository = %v", r.Topics)
	}
	if ok, err := gitrepo.HeadHasFile(empty, ".hedgehug/store.yaml"); err != nil || !ok {
		t.Errorf("the store was not pushed to the empty repository: %v %v", ok, err)
	}
	b, _ := os.ReadFile(filepath.Join(te.hhHome, "config.yaml"))
	if !strings.Contains(string(b), "account: work") {
		t.Errorf("config.yaml = %s", b)
	}

	code, _, errs := te.run("store", "init", "org2", "--prefix", "ORGB", "--remote", "https://github.com/org-a/hh-full.git")
	if code != 1 || !strings.Contains(errs, "not empty") || !strings.Contains(errs, "hedgehug store add org2 --remote https://github.com/org-a/hh-full.git") {
		t.Errorf("a repository with a commit: exit %d, %q", code, errs)
	}
	if r, _ := fake.Repo("org-a/hh-full"); len(r.Topics) != 0 {
		t.Errorf("the topic was set on a repository that was not used: %v", r.Topics)
	}
	if _, err := os.Stat(te.storeDir("org2")); !os.IsNotExist(err) {
		t.Errorf("the store directory of the failed init is left: %v", err)
	}
}

// An ssh remote is on a GitHub host by its host, like an https one: the
// repository is made from the owner and the name in the ssh URL.
func TestStoreInitSSHRemoteIsOnGitHub(t *testing.T) {
	te := newTestEnv(t)
	fake := fakegh.New(fakegh.State{Users: []string{"alice"}, Orgs: map[string][]string{"org-a": {"alice"}}, Tokens: map[string]string{"t-alice": "alice"}})
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	te.env["HH_TOKEN"] = "t-alice"
	te.writeConfigYAML("accounts:\n  - {name: work, host: 127.0.0.1, login: alice, token: \"env:HH_TOKEN\"}\nhosts:\n  - {host: 127.0.0.1, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\"}\n")
	// Nothing listens on port 1, so the push fails; the repository is made
	// first, and is left there with the way to use it.
	code, _, errs := te.run("store", "init", "org", "--prefix", "ORG", "--remote", "ssh://git@127.0.0.1:1/org-a/hh-ssh.git")
	if code != 1 || !strings.Contains(errs, "hedgehug store add org") {
		t.Errorf("exit %d, %q", code, errs)
	}
	if r, ok := fake.Repo("org-a/hh-ssh"); !ok || len(r.Topics) != 1 || r.Topics[0] != "hedgehug-store" {
		t.Errorf("repo = %+v %v", r, ok)
	}
}

func TestStoreInitGitHubFailureKeepsRepo(t *testing.T) {
	te := newTestEnv(t)
	fake := te.withFake(fakegh.State{Orgs: map[string][]string{"org-a": {"alice"}}, Tokens: map[string]string{"t-alice": "alice"}})
	te.env["HH_TOKEN"] = "t-alice"
	te.writeConfigYAML("accounts:\n  - {name: work, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	// No transport serves the push: it fails after the repository is made.
	gitrepotest.InstallHTTPS(t, func(string) (string, bool) { return "", false }, nil, nil)
	code, _, errs := te.run("store", "init", "org", "--prefix", "ORG", "--remote", "https://github.com/org-a/hh-x.git")
	if code != 1 || !strings.Contains(errs, "https://github.com/org-a/hh-x.git") || !strings.Contains(errs, "hedgehug store add org") {
		t.Errorf("exit %d, %q", code, errs)
	}
	if _, ok := fake.Repo("org-a/hh-x"); !ok {
		t.Error("the repository made on GitHub was removed")
	}
	if _, err := os.Stat(te.storeDir("org")); !os.IsNotExist(err) {
		t.Errorf("the store directory is left: %v", err)
	}
}

// A 403 on creating (alice is not a member of org-a) tells how to go on
// with a repository made by hand; a 401 names the account and the command.
func TestStoreInitGitHubErrors(t *testing.T) {
	te := newTestEnv(t)
	fake := te.withFake(fakegh.State{Orgs: map[string][]string{"org-a": {"bob"}}, Tokens: map[string]string{"t-alice": "alice"}})
	te.env["HH_TOKEN"] = "t-alice"
	te.writeConfigYAML("accounts:\n  - {name: work, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	code, _, errs := te.run("store", "init", "org", "--prefix", "ORG", "--remote", "https://github.com/org-a/x.git")
	if code != 1 || !strings.Contains(errs, "github.com/alice cannot create org-a/x") || !strings.Contains(errs, "make an empty repository on GitHub") {
		t.Errorf("403: exit %d, %q", code, errs)
	}
	if _, ok := fake.Repo("org-a/x"); ok {
		t.Error("a repository was made for a token that may not")
	}
	te.env["HH_TOKEN"] = "stale"
	code, _, errs = te.run("store", "init", "org", "--prefix", "ORG", "--remote", "https://github.com/org-a/x.git")
	if code != 1 || !strings.Contains(errs, "github.com/alice cannot create org-a/x") || !strings.Contains(errs, "run hedgehug auth login work") {
		t.Errorf("401: exit %d, %q", code, errs)
	}
}

func TestStoreInitAccountChoice(t *testing.T) {
	te := newTestEnv(t)
	te.withFake(fakegh.State{})
	te.writeConfigYAML("accounts:\n  - {name: a, host: github.com, login: alice, token: gh}\n  - {name: b, host: github.com, login: bob, token: gh}\n")
	code, _, errs := te.run("store", "init", "org", "--prefix", "ORG", "--remote", "https://github.com/org-a/x.git")
	if code != 1 || !strings.Contains(errs, "--account") || !strings.Contains(errs, "a, b") {
		t.Errorf("exit %d, %q", code, errs)
	}
	if code, _, errs := te.run("store", "init", "org", "--prefix", "ORG", "--account", "nope"); code != 1 || !strings.Contains(errs, "nope") {
		t.Errorf("unknown account: exit %d, %q", code, errs)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./cmd/hedgehug/ -run TestStoreInit`
Expected: FAIL（`--path`・`--account` が未知のフラグ、bare が作られない）

- [ ] **Step 3: github.go と store_init.go を書く**

`cmd/hedgehug/github.go`:

```go
package main

import (
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/github"
)

// githubFor is a client of host with the token of acct.
func (a *app) githubFor(c *config.Config, acct config.Account, host string) (*github.Client, error) {
	if acct.Token.Kind == "" {
		return nil, fmt.Errorf("account %s has no token: to set; run hedgehug auth login %s, or set token: in config.yaml", acct.Name, acct.Name)
	}
	tok, err := a.tokenFor(c, acct)
	if err != nil {
		return nil, err
	}
	return github.New(c.Host(host), tok, a.httpClient), nil
}

// pickAccount is the account --account names, or the only one; ok is false
// when there is none to pick.
func (a *app) pickAccount(c *config.Config, name string) (config.Account, bool, error) {
	if name != "" {
		acct, ok := c.Account(name)
		if !ok {
			return config.Account{}, false, fmt.Errorf("no account is named %q (accounts: %s)", name, accountNames(c))
		}
		return acct, true, nil
	}
	if len(c.Accounts) == 1 {
		return c.Accounts[0], true, nil
	}
	return config.Account{}, false, nil
}

func accountNames(c *config.Config) string {
	names := make([]string, len(c.Accounts))
	for i, x := range c.Accounts {
		names[i] = x.Name
	}
	return strings.Join(names, ", ")
}
```

`store_init.go` の変更:

- import に `io/fs`、`hhctx "github.com/kukv/hedgehug/internal/context"`、`internal/github` を足す。
- `storeCmd` の `AddCommand` は Task 1 と Task 12 が足す。ここでは変えない。
- `storeInitCmd` に変数 `account, path string` と、フラグ `cmd.Flags().StringVar(&account, "account", "", "the account that writes the store and makes its GitHub repository (default: the only account)")`、`cmd.Flags().StringVar(&path, "path", "", "where to make the store (default: under the data directory)")` を足し、`RunE` の最後を `return a.storeInit(args[0], prefix, wsName, sc, remotes, account, path)` にする。
- `storeInit` を次に置き換える（`addCommand` はそのまま）:

```go
func (a *app) storeInit(name, prefix, wsName string, sc *schema.Schema, remotes []string, account, path string) error {
	if err := config.ValidStoreName(name); err != nil {
		return usageError{err}
	}
	if err := model.ValidIDPrefix(prefix); err != nil {
		return usageError{err}
	}
	if wsName == "" {
		wsName = name
	}
	if err := model.ValidWorkspaceName(wsName); err != nil {
		return usageError{err}
	}
	c, p, err := a.config()
	if err != nil {
		return err
	}
	for _, e := range c.Stores {
		if e.Name == name {
			return fmt.Errorf("a store named %q is already set up", name)
		}
	}
	if err := a.checkPrefixElsewhere(prefix, name); err != nil {
		return err
	}
	// --account, or the only account. Without either, the store has no account
	// (a remote on a GitHub host then fails in prepareRemotes).
	acct, haveAcct, err := a.pickAccount(c, account)
	if err != nil {
		return err
	}
	author, err := config.CommitAuthor(acct, haveAcct, a.getenv)
	if err != nil {
		return err
	}
	accountID := ""
	if haveAcct {
		accountID = acct.ID()
	}
	dir := filepath.Join(p.StoresDir, name)
	if path != "" {
		if dir, err = filepath.Abs(path); err != nil {
			return err
		}
	}
	e := config.StoreEntry{Name: name, Path: dir, Remotes: remotes}
	if haveAcct {
		e.Account = acct.Name
	}
	made, err := a.prepareRemotes(c, e, acct, haveAcct)
	if err != nil {
		return err
	}
	ws := &model.Workspace{ULID: model.NewULID(), Name: wsName, IDPrefix: prefix}
	s, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: name}, sc, ws, author, accountID)
	if err != nil {
		return made.note(errors.Join(err, made.undo()))
	}
	undo := func(err error) error {
		return made.note(errors.Join(err, os.RemoveAll(dir), made.undo())) //nolint:gosec // G703: dir is the store this command just made
	}
	if err := s.Repo().RegisterMergeDriver(); err != nil {
		return undo(err)
	}
	var pushed []string
	if len(remotes) > 0 {
		opt, err := a.syncOptions(c, e)
		if err != nil {
			return undo(err)
		}
		res, err := a.runSync(s, opt)
		if errors.Is(err, gitrepo.ErrRejected) || errors.Is(err, sync.ErrUnrelated) {
			err = fmt.Errorf("the remote is not empty; to use the store there, run %s", addCommand(name, remotes))
		}
		if err != nil {
			return undo(err)
		}
		pushed = res.Pushed
		for _, w := range res.Warnings {
			_, _ = fmt.Fprintf(a.stderr, "warning: %s: %s\n", name, w)
		}
	}
	if err := c.AddStore(name, dir, remotes); err != nil {
		return undo(err)
	}
	// The account that was picked is written, whether --account named it or it
	// was the only one.
	if e.Account != "" {
		if err := c.SetStoreAccount(name, e.Account); err != nil {
			return undo(err)
		}
	}
	if err := c.Save(); err != nil {
		return undo(err)
	}
	if a.jsonOut {
		type wsJSON struct {
			ULID     string `json:"ulid"`
			Name     string `json:"name"`
			IDPrefix string `json:"id_prefix"`
		}
		return a.printJSON(struct {
			Name      string   `json:"name"`
			Path      string   `json:"path"`
			Workspace wsJSON   `json:"workspace"`
			Remotes   []string `json:"remotes,omitempty"`
			Pushed    []string `json:"pushed,omitempty"`
		}{name, dir, wsJSON{ws.ULID.String(), ws.Name, ws.IDPrefix}, remotes, pushed})
	}
	_, err = fmt.Fprintf(a.stdout, "Created store %s at %s with workspace %s (%s)\n", name, dir, ws.Name, ws.IDPrefix)
	if err == nil && len(pushed) > 0 {
		_, err = fmt.Fprintf(a.stdout, "Pushed to %s\n", strings.Join(pushed, ", "))
	}
	return err
}
```

```go
// madeRemotes is what prepareRemotes made: local bare repositories, which a
// failure removes, and GitHub repositories, which it leaves and tells of.
type madeRemotes struct {
	bares  []string
	github []string // URLs
	name   string
}

func (m madeRemotes) undo() error {
	var errs []error
	for _, b := range m.bares {
		errs = append(errs, os.RemoveAll(b)) //nolint:gosec // G703: a bare repository this command just made
	}
	return errors.Join(errs...)
}

// note tells of the GitHub repositories left behind.
func (m madeRemotes) note(err error) error {
	if len(m.github) == 0 {
		return err
	}
	return fmt.Errorf("%w\nthe repository %s was made on GitHub and is left there; to use it, run %s",
		err, strings.Join(m.github, ", "), addCommand(m.name, m.github))
}

// prepareRemotes makes what the remotes of e need before the first push: a
// bare repository for a local path that is missing, a private repository with
// the hedgehug-store topic on a GitHub host. Other remotes must exist
// already. Which kind a remote is, is told by the URL after url_rewrites, and
// a GitHub host is told by its host in any form of a URL (https, ssh://,
// user@host:path).
func (a *app) prepareRemotes(c *config.Config, e config.StoreEntry, acct config.Account, haveAcct bool) (madeRemotes, error) {
	m := madeRemotes{name: e.Name}
	for _, u := range e.Remotes {
		to := c.Rewrite(u)
		if p, ok := gitrepo.LocalPath(to); ok {
			if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) { //nolint:gosec // G703: p is a remote written by the person who runs this
				if err := gitrepo.InitBare(p); err != nil {
					return m, errors.Join(err, m.undo())
				}
				m.bares = append(m.bares, p)
			}
			continue
		}
		host, owner, repo, ok := hhctx.Split(to)
		if !ok || !c.IsGitHubHost(host) {
			continue
		}
		if !haveAcct {
			return m, errors.Join(fmt.Errorf("%s is on GitHub, so an account makes it; pick one with --account (accounts: %s)", u, accountNames(c)), m.undo())
		}
		gh, err := a.githubFor(c, acct, host)
		if err != nil {
			return m, errors.Join(err, m.undo())
		}
		doing := "create " + owner + "/" + repo
		_, err = gh.CreateRepo(owner, repo, strings.EqualFold(owner, acct.Login))
		switch {
		case errors.Is(err, github.ErrExists):
			// 422: the repository is there. One with no ref is used as it is;
			// one with content is a store, or something else, for store add.
			empty, lerr := gitrepo.RemoteEmpty(u, a.connFor(c, e))
			if lerr != nil {
				return m, errors.Join(fmt.Errorf("%s exists on GitHub, and whether it is empty could not be checked: %w", u, lerr), m.undo())
			}
			if !empty {
				return m, errors.Join(fmt.Errorf("%s exists on GitHub and is not empty; to use the store there, run %s", u, addCommand(e.Name, e.Remotes)), m.undo())
			}
		case errors.Is(err, github.ErrForbidden):
			err = github.Describe(err, acct.ID(), acct.Name, doing)
			return m, errors.Join(fmt.Errorf("%w\nmake an empty repository on GitHub and give its URL to hedgehug store init --remote; the creation is skipped", err), m.undo())
		case err != nil:
			return m, errors.Join(github.Describe(err, acct.ID(), acct.Name, doing), m.undo())
		default:
			m.github = append(m.github, u)
		}
		if err := gh.SetTopics(owner, repo, []string{"hedgehug-store"}); err != nil {
			return m, errors.Join(m.note(github.Describe(err, acct.ID(), acct.Name, "set the topic of "+owner+"/"+repo)), m.undo())
		}
	}
	return m, nil
}
```

（403 の案内は spec 付録の「GitHub で空のリポジトリを作り、hh store init --remote でその URL を指定してください（作成は飛ばします）」に当たる。その空のリポジトリで `store init` をすると作成が 422 になり、`RemoteEmpty` が空と答えるので、作成を飛ばして使う。中身のあるリポジトリは `store add` を案内する。どちらも spec の「Store の作成と発見」のとおり。）

- [ ] **Step 4: テストを通す**

Run: `go test -race ./cmd/hedgehug/ -run 'TestStoreInit|TestStoreSelection'`
Expected: PASS

そのうえで `go test -race ./cmd/hedgehug/` を通す。`store init` がアカウントを1つのときも `stores[].account` に書くようになったので、`account:` の行が増えたことで落ちる既存のテストがあれば、期待値に `account: <name>` の行を足して直す（`writeAccounts` でアカウントを1つ書いた上で `store init` し、config.yaml を完全一致で比べているもの）。`strings.Contains` や読み直しで比べているテストは変わらない。

- [ ] **Step 5: コミット**

```bash
git add cmd/hedgehug
git commit -m "feat(cli): store init がローカルの bare と GitHub のリポジトリを用意し、--account と --path を受け付ける"
```

---

### Task 10: 読み取り専用とメンバーの状態、同期での GitHub の情報の更新

**Files:**
- Create: `internal/sync/access.go`、`internal/sync/members.go`、`internal/sync/github.go`
- Modify: `internal/sync/sync.go`（`Options.GitHub`・`Options.Explicit`、`Run` の最後で `refreshInfo`、`Result.Unpushable`、読み取り専用の同期、`ErrPushDenied`）
- Test: `internal/sync/access_test.go`、`internal/sync/github_test.go`、`internal/sync/readonly_test.go`

**Interfaces:**
- Consumes: `gitrepo.ErrPushDenied`、`LocalPath`、`Writable`、`Conn`、`gitrepotest.LockObjects`（Task 7。テストだけ）、`github.Client`、`Repo`、`Collaborators`、`ErrForbidden`、`Describe`（Task 4）
- Produces:
  - `type Access struct { ReadOnly bool; Reason string; Checked time.Time }`（JSON `read_only`、`reason`、`checked`）、`const ReasonGitHub = "github_permission"`、`ReasonNotWritable = "not_writable"`、`ReasonPushDenied = "push_denied"`
  - `func ReadAccess(s *store.Store) (Access, error)`、`func WriteAccess(s *store.Store, a Access) error`、`func (a Access) Why() string`（`the GitHub permission is read`、`the remote cannot be written`、`the remote denied the push`）
  - `type Members struct { Collaborators []Collaborator; Authors []string; Head string; Checked time.Time }`（JSON `collaborators`（nil は `null`）、`authors`、`head`、`checked`）、`type Collaborator struct{ ID, Permission string }`（JSON `id`、`permission`）
  - `func ReadMembers(s *store.Store) (Members, error)`、`func WriteMembers(s *store.Store, m Members) error`、`func UpdateAuthors(s *store.Store) error`
  - `type GitHubRepo struct { Host, Owner, Name string; AccountID, AccountName string; Client func() (*github.Client, error) }`（`AccountID` は `<host>/<login>`、`AccountName` は config のアカウント名。GitHub から取れなかったときの警告に「どのアカウントで何をしようとしたか」と、401 なら `hedgehug auth login <AccountName>` を添えるのに使う。`sync` は `config` を import しないので、文字列で持つ）
  - `Options.GitHub *GitHubRepo`（origin が GitHub のホストでなければ nil）、`Options.Explicit bool`（`hh sync` の明示の実行）
  - `func CheckAccess(s *store.Store, opt Options, now time.Time) (Access, []string, error)`: origin の種類ごとに判定して `access.json`（と GitHub なら `members.json` の collaborators）を書き、警告を返す
  - `Result.Unpushable int`
  - `func Unpushed(r *gitrepo.Repo) (int, error)`（HEAD にあって、最後の fetch か push の時点で origin にないコミットの数。Store が一度も push していなければ全部。cmd の `unpushed` もこれを呼ぶ。数える処理はここの1つだけ）
  - `infoInterval = 5 * time.Minute`

- [ ] **Step 1: 失敗するテストを書く**

`internal/sync/access_test.go`（`newCluster` は `helpers_test.go` の既存のヘルパー。最初の `Run` で access.json と members.json ができるので、空の状態は `store.Create` だけの Store で確かめる）:

```go
package sync

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// newLoneStore makes a store that was never synced, written by github.com/u0.
func newLoneStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Create(filepath.Join(t.TempDir(), "s"), &model.StoreMeta{ULID: model.NewULID(), Name: "s"}, nil,
		&model.Workspace{ULID: model.NewULID(), Name: "web", IDPrefix: "WEB"}, gitrepo.Signature{Name: "u0", Email: "u0@x"}, "github.com/u0")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAccessAndMembersRoundTrip(t *testing.T) {
	s := newLoneStore(t)
	if a, err := ReadAccess(s); err != nil || a.ReadOnly {
		t.Errorf("no access.json = %+v, %v", a, err)
	}
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if err := WriteAccess(s, Access{ReadOnly: true, Reason: ReasonGitHub, Checked: now}); err != nil {
		t.Fatal(err)
	}
	if a, _ := ReadAccess(s); !a.ReadOnly || a.Why() != "the GitHub permission is read" || !a.Checked.Equal(now) {
		t.Errorf("access = %+v", a)
	}
	m, err := ReadMembers(s)
	if err != nil || m.Collaborators != nil {
		t.Errorf("no members.json = %+v, %v", m, err)
	}
	if err := UpdateAuthors(s); err != nil {
		t.Fatal(err)
	}
	m, _ = ReadMembers(s)
	if m.Head == "" || !slices.Contains(m.Authors, "github.com/u0") {
		t.Errorf("members = %+v", m)
	}
}
```

`internal/sync/github_test.go`:

```go
package sync

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/github"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
)

func TestCheckAccessGitHub(t *testing.T) {
	fake := fakegh.New(fakegh.State{
		Users:  []string{"alice", "bob"},
		Tokens: map[string]string{"t-bob": "bob"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "x", Perms: map[string]string{"alice": "admin", "bob": "read"}}},
	})
	srv := httptest.NewServer(fake)
	defer srv.Close()
	s := newLoneStore(t)
	opt := Options{Remotes: []string{"https://github.com/org-a/x.git"}, GitHub: &GitHubRepo{Host: "github.com", Owner: "org-a", Name: "x",
		AccountID: "github.com/bob", AccountName: "me",
		Client: func() (*github.Client, error) {
			return github.New(config.Host{API: srv.URL}, "t-bob", nil), nil
		}}}
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	a, warns, err := CheckAccess(s, opt, now)
	if err != nil || !a.ReadOnly || a.Reason != ReasonGitHub || len(warns) != 0 {
		t.Errorf("read = %+v %v %v", a, warns, err)
	}
	m, _ := ReadMembers(s)
	if m.Collaborators != nil {
		t.Errorf("a reader gets no collaborators: %+v", m.Collaborators)
	}
	if err := fake.SetPerm("org-a/x", "bob", "write"); err != nil {
		t.Fatal(err)
	}
	a, _, err = CheckAccess(s, opt, now.Add(time.Minute))
	if err != nil || a.ReadOnly {
		t.Errorf("write = %+v %v", a, err)
	}
	m, _ = ReadMembers(s)
	if len(m.Collaborators) != 2 || m.Collaborators[1] != (Collaborator{ID: "github.com/bob", Permission: "write"}) {
		t.Errorf("collaborators = %+v", m.Collaborators)
	}
	// An API that fails leaves the access as it was, with a warning.
	opt.GitHub.Client = func() (*github.Client, error) { return github.New(config.Host{API: srv.URL}, "bad", nil), nil }
	a, warns, err = CheckAccess(s, opt, now.Add(2*time.Minute))
	if err != nil || a.ReadOnly || len(warns) != 1 {
		t.Fatalf("failing API = %+v %v %v", a, warns, err)
	}
	// The warning says which account failed to do what, and for a 401 what to run.
	for _, want := range []string{"github.com/bob cannot read the permission of org-a/x", "401", "run hedgehug auth login me"} {
		if !strings.Contains(warns[0], want) {
			t.Errorf("warning = %q, want it to contain %q", warns[0], want)
		}
	}
}

// Collaborators that cannot be read, for whatever reason, are unknown (null),
// not the list of an earlier check. A 403 is no warning (it is how a token
// without push access is told); any other failure is.
func TestCheckAccessCollaboratorsUnknownOnAnyFailure(t *testing.T) {
	fake := fakegh.New(fakegh.State{
		Users:  []string{"alice"},
		Tokens: map[string]string{"t-alice": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "x", Perms: map[string]string{"alice": "admin"}}},
	})
	var broken atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if broken.Load() && strings.HasSuffix(r.URL.Path, "/collaborators") {
			http.Error(w, `{"message":"boom"}`, http.StatusInternalServerError)
			return
		}
		fake.ServeHTTP(w, r)
	}))
	defer srv.Close()
	s := newLoneStore(t)
	opt := Options{Remotes: []string{"https://github.com/org-a/x.git"}, GitHub: &GitHubRepo{Host: "github.com", Owner: "org-a", Name: "x",
		AccountID: "github.com/alice", AccountName: "me",
		Client: func() (*github.Client, error) {
			return github.New(config.Host{API: srv.URL}, "t-alice", nil), nil
		}}}
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if _, warns, err := CheckAccess(s, opt, now); err != nil || len(warns) != 0 {
		t.Fatalf("first check = %v %v", warns, err)
	}
	if m, _ := ReadMembers(s); len(m.Collaborators) != 1 || m.Collaborators[0].ID != "github.com/alice" {
		t.Fatalf("collaborators = %+v", m.Collaborators)
	}
	broken.Store(true)
	a, warns, err := CheckAccess(s, opt, now.Add(time.Minute))
	if err != nil || a.ReadOnly {
		t.Errorf("access after a failure = %+v %v", a, err)
	}
	m, _ := ReadMembers(s)
	if m.Collaborators != nil || !m.Checked.Equal(now.Add(time.Minute)) {
		t.Errorf("members after a failure = %+v, want collaborators null and a new time", m)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "github.com/alice cannot read the collaborators of org-a/x") {
		t.Errorf("warnings = %v", warns)
	}
}

func TestCheckAccessLocal(t *testing.T) {
	c := newCluster(t, 1)
	a, _, err := CheckAccess(c.stores[0], c.opts[0], time.Now())
	if err != nil || a.ReadOnly {
		t.Errorf("writable bare = %+v %v", a, err)
	}
	gitrepotest.LockObjects(t, c.remote)
	a, _, err = CheckAccess(c.stores[0], c.opts[0], time.Now())
	if err != nil || !a.ReadOnly || a.Reason != ReasonNotWritable {
		t.Errorf("read-only bare = %+v %v", a, err)
	}
}
```

`internal/sync/readonly_test.go`:

```go
package sync

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
)

func TestReadOnlySyncDoesNotPush(t *testing.T) {
	c := newCluster(t, 1)
	c.addTask(0, "local") // a commit that origin lacks
	if err := WriteAccess(c.stores[0], Access{ReadOnly: true, Reason: ReasonGitHub, Checked: time.Now()}); err != nil {
		t.Fatal(err)
	}
	res := c.sync(0)
	if len(res.Pushed) != 0 || res.Unpushable != 1 {
		t.Errorf("pushed %v, unpushable %d", res.Pushed, res.Unpushable)
	}
}

func TestPushDeniedMakesReadOnly(t *testing.T) {
	c := newCluster(t, 1)
	c.addTask(0, "local")
	unlock := gitrepotest.LockObjects(t, c.remote)
	_, err := Run(c.stores[0], c.opts[0])
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("err = %v", err)
	}
	if a, _ := ReadAccess(c.stores[0]); !a.ReadOnly || a.Reason != ReasonPushDenied {
		t.Errorf("access = %+v", a)
	}
	unlock()
	opt := c.opts[0]
	opt.Explicit = true // hh sync tries the push again
	if _, err := Run(c.stores[0], opt); err != nil {
		t.Fatal(err)
	}
	if a, _ := ReadAccess(c.stores[0]); a.ReadOnly {
		t.Errorf("access after a push that went = %+v", a)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/sync/ -run 'TestAccess|TestCheckAccess|TestReadOnly|TestPushDenied'`
Expected: FAIL（未定義）

- [ ] **Step 3: access.go と members.go を書く**

```go
package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kukv/hedgehug/internal/store"
)

const accessFile = "access.json"

// The reasons a store is read-only.
const (
	ReasonGitHub      = "github_permission"
	ReasonNotWritable = "not_writable"
	ReasonPushDenied  = "push_denied"
)

// Access is whether this machine may write to the store, as last checked.
type Access struct {
	ReadOnly bool      `json:"read_only"`
	Reason   string    `json:"reason,omitempty"`
	Checked  time.Time `json:"checked,omitzero"`
}

// Why is the reason in words.
func (a Access) Why() string {
	switch a.Reason {
	case ReasonGitHub:
		return "the GitHub permission is read"
	case ReasonNotWritable:
		return "the remote cannot be written"
	case ReasonPushDenied:
		return "the remote denied the push"
	}
	return a.Reason
}

// ReadAccess reads access.json; a missing file is a writable store.
func ReadAccess(s *store.Store) (Access, error) {
	var a Access
	b, err := s.ReadLocal(accessFile)
	if errors.Is(err, store.ErrNotFound) {
		return a, nil
	}
	if err != nil {
		return a, err
	}
	if err := json.Unmarshal(b, &a); err != nil {
		return Access{}, fmt.Errorf("%s: %w", accessFile, err)
	}
	return a, nil
}

// WriteAccess replaces access.json.
func WriteAccess(s *store.Store, a Access) error {
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return s.WriteLocal(accessFile, append(b, '\n'))
}
```

```go
package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/store"
)

const membersFile = "members.json"

// Collaborator is a person GitHub lets read the store, as an account ID.
type Collaborator struct {
	ID         string `json:"id"`
	Permission string `json:"permission"`
}

// Members is who is known to use the store: the collaborators on GitHub
// (nil when they could not be read) and the writers of its commits.
type Members struct {
	Collaborators []Collaborator `json:"collaborators"`
	Authors       []string       `json:"authors"`
	Head          string         `json:"head,omitempty"` // the HEAD the authors were read up to
	Checked       time.Time      `json:"checked,omitzero"`
}

// ReadMembers reads members.json; a missing file has no one.
func ReadMembers(s *store.Store) (Members, error) {
	var m Members
	b, err := s.ReadLocal(membersFile)
	if errors.Is(err, store.ErrNotFound) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return Members{}, fmt.Errorf("%s: %w", membersFile, err)
	}
	return m, nil
}

// WriteMembers replaces members.json.
func WriteMembers(s *store.Store, m Members) error {
	if m.Authors == nil {
		m.Authors = []string{}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return s.WriteLocal(membersFile, append(b, '\n'))
}

// UpdateAuthors adds the Hedgehug-Author of the commits HEAD gained since
// the last update.
func UpdateAuthors(s *store.Store) error {
	m, err := ReadMembers(s)
	if err != nil {
		return err
	}
	r := s.Repo()
	head, err := r.Head()
	if err != nil || head.IsZero() {
		return err
	}
	from := plumbing.ZeroHash
	if m.Head != "" {
		from = plumbing.NewHash(m.Head)
		if ok, err := r.IsAncestor(from, head); err != nil || !ok {
			from = plumbing.ZeroHash // history changed: read it all
		}
	}
	es, err := r.Between(head, from)
	if err != nil {
		return err
	}
	for _, e := range es {
		if id, ok := e.Trailer("Hedgehug-Author"); ok && !slices.Contains(m.Authors, id) {
			m.Authors = append(m.Authors, id)
		}
	}
	slices.Sort(m.Authors)
	m.Head = head.String()
	return WriteMembers(s, m)
}
```

- [ ] **Step 4: github.go を書く**

```go
package sync

import (
	"errors"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/github"
	"github.com/kukv/hedgehug/internal/store"
)

// infoInterval is how often a sync reads the permission and the
// collaborators from GitHub.
const infoInterval = 5 * time.Minute

// GitHubRepo is the repository on GitHub that origin is. Client gets the
// token only when called.
type GitHubRepo struct {
	Host, Owner, Name string
	// AccountID (<host>/<login>) and AccountName (as in config.yaml) say who
	// asks, for the warnings; sync does not import config.
	AccountID, AccountName string
	Client                 func() (*github.Client, error)
}

// CheckAccess finds out whether the store may be pushed to, by what origin
// is: the permission on GitHub, whether a local bare can be written, or
// nothing to check (a push that is denied tells). It writes access.json, and
// for GitHub the collaborators to members.json. What it could not read is a
// warning, and the access is left as it was.
func CheckAccess(s *store.Store, opt Options, now time.Time) (Access, []string, error) {
	a, err := ReadAccess(s)
	if err != nil {
		return a, nil, err
	}
	var warns []string
	switch {
	case opt.GitHub != nil:
		gh, err := opt.GitHub.Client()
		var r *github.Repo
		if err == nil {
			r, err = gh.Repo(opt.GitHub.Owner, opt.GitHub.Name)
		}
		if err != nil {
			doing := "read the permission of " + opt.GitHub.Owner + "/" + opt.GitHub.Name
			return a, []string{github.Describe(err, opt.GitHub.AccountID, opt.GitHub.AccountName, doing).Error()}, nil
		}
		a = Access{ReadOnly: !r.Permissions.Push, Checked: now}
		if a.ReadOnly {
			a.Reason = ReasonGitHub
		}
		if w := updateCollaborators(s, gh, opt.GitHub, now); w != "" {
			warns = append(warns, w)
		}
	case len(opt.Remotes) > 0:
		to := opt.Remotes[0]
		if opt.Conn.Rewrite != nil {
			to = opt.Conn.Rewrite(to)
		}
		p, ok := gitrepo.LocalPath(to)
		if !ok {
			return a, nil, nil
		}
		a = Access{ReadOnly: !gitrepo.Writable(p), Checked: now}
		if a.ReadOnly {
			a.Reason = ReasonNotWritable
		}
	default:
		return a, nil, nil
	}
	return a, warns, WriteAccess(s, a)
}

// updateCollaborators writes the collaborators to members.json. When they
// cannot be read, for whatever reason, they are unknown (null), not the list
// of an earlier check (spec: 取れなかったとき（403 など）は null). A 403 is how
// GitHub says the token cannot push, which a reader is not told to fix, so it
// is no warning; any other failure is. It returns a warning, or "".
func updateCollaborators(s *store.Store, gh *github.Client, r *GitHubRepo, now time.Time) string {
	m, err := ReadMembers(s)
	if err != nil {
		return "read members.json: " + err.Error()
	}
	warn := ""
	cs, err := gh.Collaborators(r.Owner, r.Name)
	if err == nil {
		m.Collaborators = make([]Collaborator, len(cs))
		for i, c := range cs {
			m.Collaborators[i] = Collaborator{ID: r.Host + "/" + c.Login, Permission: c.RoleName}
		}
	} else {
		m.Collaborators = nil
		if !errors.Is(err, github.ErrForbidden) {
			warn = github.Describe(err, r.AccountID, r.AccountName, "read the collaborators of "+r.Owner+"/"+r.Name).Error()
		}
	}
	m.Checked = now
	if err := WriteMembers(s, m); err != nil {
		return "write members.json: " + err.Error()
	}
	return warn
}

// refreshInfo checks the access and the collaborators when infoInterval has
// passed since the last check, and adds the authors of the new commits.
func refreshInfo(s *store.Store, opt Options, res *Result) {
	now := opt.Now()
	a, err := ReadAccess(s)
	if err == nil && now.Sub(a.Checked) >= infoInterval {
		var warns []string
		_, warns, err = CheckAccess(s, opt, now)
		res.Warnings = append(res.Warnings, warns...)
	}
	if err != nil {
		res.Warnings = append(res.Warnings, "check the access: "+err.Error())
	}
	if err := UpdateAuthors(s); err != nil {
		res.Warnings = append(res.Warnings, "read the authors: "+err.Error())
	}
}
```

- [ ] **Step 5: sync.go を直す**

`Options` に足す:

```go
	GitHub   *GitHubRepo // origin on GitHub; nil when it is not
	Explicit bool        // hh sync, not the background one: a push_denied is tried again
```

`Result` に足す:

```go
	// Unpushable counts the commits not pushed because the store is read-only.
	Unpushable int
```

`Run` の変更:

1. `r.SetConn(opt.Conn)` の後で:

```go
	acc, err := ReadAccess(s)
	if err != nil {
		return res, err
	}
	if opt.Explicit && acc.Reason == ReasonPushDenied {
		acc = Access{}
		if err := WriteAccess(s, acc); err != nil {
			return res, err
		}
	}
```

2. ラウンドのループの `err := r.Push("origin")` の直前に:

```go
		if acc.ReadOnly {
			n, err := Unpushed(r)
			if err != nil {
				return res, err
			}
			res.Unpushable = n
			break
		}
```

（`break` は for ループを抜ける。ミラーへの push もしない。）

3. `err := r.Push("origin")` の分岐に、`ErrRejected` の前に:

```go
		if errors.Is(err, gitrepo.ErrPushDenied) {
			denied := Access{ReadOnly: true, Reason: ReasonPushDenied, Checked: opt.Now()}
			return res, errors.Join(fmt.Errorf("the store is read-only (%s): %w", denied.Why(), err), WriteAccess(s, denied))
		}
```

4. 押し終わった後の警告の列（`warn("check the other stores' id prefixes", …)` の前）に `refreshInfo(s, opt, &res)` を置く。読み取り専用で push しなかったときも、ここを通る。

5. ファイルの末尾に:

```go
// Unpushed counts the commits of HEAD that origin did not have as of the last
// fetch or push. A store that never pushed has all of them. The commands that
// show it call this too.
func Unpushed(r *gitrepo.Repo) (int, error) {
	head, err := r.Head()
	if err != nil {
		return 0, err
	}
	remote, err := r.RemoteHead("origin")
	if err != nil {
		return 0, err
	}
	es, err := r.Between(head, remote)
	return len(es), err
}
```

- [ ] **Step 6: テストを通す**

Run: `go test -race ./internal/sync/`
Expected: PASS（既存の同期のテストも通る。`Now` が nil のテストは `Run` が `time.Now` を入れるので `refreshInfo` も動く）

- [ ] **Step 7: コミット**

```bash
git add internal/sync
git commit -m "feat(sync): 読み取り専用とメンバーの状態を持ち、5分おきに GitHub から更新し、読み取り専用では push しない"
```

---

### Task 11: 読み取り専用の CLI（書き込みの拒否、`store add --account`、同期の表示）

**Files:**
- Create: `cmd/hedgehug/readonly.go`
- Modify: `cmd/hedgehug/app.go`（`update` の最初で `checkWritable`、`syncOptions` に `GitHub`）、`cmd/hedgehug/doctor.go`（`--fix` の前）、`cmd/hedgehug/rm.go`（`purge` の `a.open()` の直後）、`cmd/hedgehug/store_add.go`（`--account`、clone の直後の `CheckAccess`）、`cmd/hedgehug/sync.go`（`Explicit`、`Unpushable`、status の read-only）
- Test: `cmd/hedgehug/readonly_test.go`

**Interfaces:**
- Consumes: `sync.ReadAccess`、`CheckAccess`、`Access.Why`、`Result.Unpushable`、`Unpushed`、`Options.GitHub`、`Options.Explicit`、`GitHubRepo`（Task 10）、`gitrepotest.LockObjects`（Task 7。テストだけ）、`context.Split`（Task 4）、`a.githubFor`、`a.pickAccount`（Task 9）、`a.connFor`（Task 7）
- Produces:
  - `type readOnlyError struct{ store string; access sync.Access }`（`Error()` は `<store> is read-only (<why>)`）
  - `func (a *app) checkWritable(s *store.Store) error`
  - `func (a *app) githubRepoOf(c *config.Config, e config.StoreEntry) *sync.GitHubRepo`
  - `func (a *app) storeAdd(name string, remotes []string, path, account string) error`（Task 12 の discover も呼ぶ）
  - `syncJSON` に `Unpushable int \`json:"unpushable,omitempty"\``、`statusJSON` に `ReadOnly string \`json:"read_only,omitempty"\``（理由の文）

- [ ] **Step 1: 失敗するテストを書く**

`cmd/hedgehug/readonly_test.go`:

```go
package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

func TestReadOnlyRefusesWrites(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "one")
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sync.WriteAccess(s, sync.Access{ReadOnly: true, Reason: sync.ReasonGitHub, Checked: te.now}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "two"}, {"set", "ME-1", "title=x"}, {"done", "ME-1"}, {"rm", "ME-1"},
		{"comment", "ME-1", "hi"}, {"workspace", "add", "w", "--prefix", "W"}, {"purge", "ME-1", "--yes"},
	} {
		code, _, errs := te.run(args...)
		if code != 1 || !strings.Contains(errs, "me is read-only (the GitHub permission is read)") {
			t.Errorf("%v: exit %d, %q", args, code, errs)
		}
	}
	if out := te.ok("ls"); !strings.Contains(out, "one") {
		t.Errorf("reading still works: %q", out)
	}
}

func TestStoreAddChecksAccess(t *testing.T) {
	te := newTestEnv(t)
	d := t.TempDir()
	// alice makes the store in a bare, which bob reaches as github.com.
	alice := newTestEnv(t)
	alice.ok("store", "init", "org", "--prefix", "ORG", "--remote", filepath.Join(d, "org-a", "x.git"))
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), nil, nil)
	// bob may only read it on GitHub.
	te.withFake(fakegh.State{
		Tokens: map[string]string{"t-bob": "bob"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "x", Perms: map[string]string{"bob": "read"}}},
	})
	te.env["HH_TOKEN"] = "t-bob"
	te.writeConfigYAML("accounts:\n  - {name: me, host: github.com, login: bob, token: \"env:HH_TOKEN\"}\n")
	te.ok("store", "add", "org", "--remote", "https://github.com/org-a/x.git", "--account", "me")
	code, _, errs := te.run("add", "x", "--store", "org")
	if code != 1 || !strings.Contains(errs, "read-only") {
		t.Errorf("exit %d, %q", code, errs)
	}
	out := te.ok("sync", "--status")
	if !strings.Contains(out, "read-only") {
		t.Errorf("status = %q", out)
	}
}

func TestSyncShowsUnpushable(t *testing.T) {
	te := newTestEnv(t)
	bare := filepath.Join(t.TempDir(), "x.git")
	if err := gitrepo.InitBare(bare); err != nil {
		t.Fatal(err)
	}
	te.ok("store", "init", "me", "--prefix", "ME", "--remote", bare)
	te.ok("add", "one")
	gitrepotest.LockObjects(t, bare) // skips the test where there are no read-only directories
	code, _, errs := te.run("sync")
	if code != 1 || !strings.Contains(errs, "read-only (the remote denied the push)") {
		t.Errorf("sync: exit %d, %q", code, errs)
	}
	te.now = te.now.Add(time.Minute)
	out := te.ok("sync", "--status")
	if !strings.Contains(out, "read-only") || !strings.Contains(out, "1 commit not pushed") {
		t.Errorf("status = %q", out)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./cmd/hedgehug/ -run 'TestReadOnly|TestStoreAddChecksAccess|TestSyncShowsUnpushable'`
Expected: FAIL

- [ ] **Step 3: readonly.go を書き、入口で呼ぶ**

```go
package main

import (
	"fmt"

	"github.com/kukv/hedgehug/internal/config"
	hhctx "github.com/kukv/hedgehug/internal/context"
	"github.com/kukv/hedgehug/internal/github"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// readOnlyError refuses a write to a read-only store.
type readOnlyError struct {
	store  string
	access sync.Access
}

func (e *readOnlyError) Error() string {
	return fmt.Sprintf("%s is read-only (%s)", e.store, e.access.Why())
}

// checkWritable refuses when the store the command works on is read-only.
func (a *app) checkWritable(s *store.Store) error {
	acc, err := sync.ReadAccess(s)
	if err != nil || !acc.ReadOnly {
		return err
	}
	name := s.Meta().Name
	if c, _, err := a.config(); err == nil {
		for _, e := range c.Stores {
			if e.Path == s.Dir() {
				name = e.Name
			}
		}
	}
	return &readOnlyError{store: name, access: acc}
}

// githubRepoOf is the GitHub repository origin of e is, or nil. It looks at
// the URL as written, before url_rewrites: a store of github.com that the
// development environment keeps in a local bare is still checked on GitHub.
func (a *app) githubRepoOf(c *config.Config, e config.StoreEntry) *sync.GitHubRepo {
	if len(e.Remotes) == 0 {
		return nil
	}
	// The host decides, in any form of a URL (https, ssh://, user@host:path).
	host, owner, name, ok := hhctx.Split(e.Remotes[0])
	if !ok || !c.IsGitHubHost(host) {
		return nil
	}
	acct, ok := c.Writer(e)
	if !ok {
		return nil
	}
	return &sync.GitHubRepo{Host: host, Owner: owner, Name: name, AccountID: acct.ID(), AccountName: acct.Name,
		Client: func() (*github.Client, error) {
			return a.githubFor(c, acct, host)
		}}
}
```

- `app.update` の最初（`a.writer()` の前）に `if err := a.checkWritable(s); err != nil { return false, err }`。
- `rm.go` の `purge` で、`s, ix, err := a.open()` の直後（`a.resolve` と `purgeTargets` より前）に `if err := a.checkWritable(s); err != nil { return err }`。`purge` は `purgeTargets` が「削除済みでなければ `not deleted`」で拒否するので、`confirm` の前に置くと、読み取り専用の Store の未削除のタスクは read-only ではなく `ME-1 is not deleted` のエラーになる（`TestReadOnlyRefusesWrites` の `purge ME-1 --yes` が落ちる）。`a.open()` の直後なら、削除済みかどうかにかかわらず読み取り専用が先に拒否する。
- `doctor.go` の `fixStore` の最初に `if err := a.checkWritable(s); err != nil { return nil, err }`。
- `app.syncOptions` の `opt` に `GitHub: a.githubRepoOf(c, e)`。
- `sync.go`（cmd）の `syncStore` で `opt.Explicit = true` を `a.runSync` の前に入れる（`syncBackground` では入れない）。`res.Unpushable` を `r.Unpushable` に写し、`printSync` で `Unpushable > 0` なら `  read-only: %d commits not pushed\n` を出す。`storeStatus` で `sync.ReadAccess(s)` が読み取り専用なら `x.ReadOnly = acc.Why()`、表示では `  read-only:     <why>\n` を `last error` の後に出す。`syncStore` のエラーで `res.Unpushable` が 0 でも、`runSync` が `read-only` のエラーを返したら今のとおりエラーとして出す（`describe` の結果）。
- `sync.go`（cmd）の `unpushed(s *store.Store)` の本体を `return sync.Unpushed(s.Repo())` に置き換える（関数 `unpushed` は残し、呼び出し側の `store_add.go`・`sync.go` とそのテストは変えない。`Head`・`RemoteHead`・`Between` で数える処理は `sync.Unpushed` の1つだけにする）。
- `store_add.go`: `--account` フラグを足し、`storeAdd(name, remotes, path, account)` に。`account` が空でなければ `c.Account(account)` を確かめ（なければ `pickAccount` と同じエラー）、`e.Account = account` で `connFor` を作って clone し、`c.AddStore` の後に `c.SetStoreAccount(name, account)`。登録の後（`c.Save()` の後）に:

```go
	opt, err := a.syncOptions(c, e)
	if err == nil {
		var warns []string
		_, warns, err = sync.CheckAccess(s, opt, a.now())
		for _, w := range warns {
			_, _ = fmt.Fprintf(a.stderr, "warning: %s: %s\n", name, w)
		}
	}
	if err != nil {
		_, _ = fmt.Fprintf(a.stderr, "warning: %s: the access is not checked: %v\n", name, err)
	}
```

（clone を `c.Save()` の後で巻き戻すことはしない。判定の失敗は警告。）

- [ ] **Step 4: テストを通す**

Run: `go test -race ./cmd/hedgehug/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add cmd/hedgehug
git commit -m "feat(cli): 読み取り専用の Store への書き込みを拒否し、store add --account と同期の表示を足す"
```

---

### Task 12: `hh store discover`

**Files:**
- Create: `cmd/hedgehug/store_discover.go`
- Modify: `cmd/hedgehug/store_init.go`（`storeCmd` に `storeDiscoverCmd`）
- Test: `cmd/hedgehug/store_discover_test.go`

**Interfaces:**
- Consumes: `(*github.Client).UserRepos`、`github.Describe`、`context.RepoID`（Task 4。repo ID の正規化は Task 4 で作り済み）、`gitrepo.HeadHasFile`、`Writable`、`LocalPath`（Task 7）、`config.Unrewrite`（Task 1）、`a.storeAdd`（Task 11）、`a.githubFor`（Task 9）
- Produces:
  - `hh store discover [--account a] [--from <dir>] [--all]`（spec の「`hh store discover`」）
  - `--json`: `{"candidates": [{"name","url","account","permission"}], "mounted": [names]}`

- [ ] **Step 1: discover の失敗するテストを書く**

`cmd/hedgehug/store_discover_test.go`:

```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
)

func TestDiscoverFrom(t *testing.T) {
	remotes := filepath.Join(t.TempDir(), "remotes")
	maker := newTestEnv(t)
	maker.writeConfigYAML("url_rewrites:\n  - {from: \"https://github.com/\", to: \"" + filepath.ToSlash(remotes) + "/\"}\n")
	maker.ok("store", "init", "hh-shared", "--prefix", "SH", "--remote", "https://github.com/org-a/hh-shared.git")
	maker.ok("store", "init", "hh-ops", "--prefix", "OPS", "--remote", "https://github.com/org-b/hh-ops.git")
	// A bare that is not a store is not offered.
	if err := gitrepo.InitBare(filepath.Join(remotes, "org-a", "web.git")); err != nil {
		t.Fatal(err)
	}

	te := newTestEnv(t)
	te.writeConfigYAML("url_rewrites:\n  - {from: \"https://github.com/\", to: \"" + filepath.ToSlash(remotes) + "/\"}\n")
	out := te.ok("store", "discover", "--from", remotes)
	for _, want := range []string{"hh-shared", "https://github.com/org-a/hh-shared.git", "hh-ops", "write"} {
		if !strings.Contains(out, want) {
			t.Errorf("list lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "web") {
		t.Errorf("a bare that is not a store is offered:\n%s", out)
	}
	if out := te.ok("store", "list"); strings.Contains(out, "hh-shared") {
		t.Errorf("not a terminal: nothing is mounted:\n%s", out)
	}
	te.ok("store", "discover", "--from", remotes, "--all")
	if out := te.ok("store", "list"); !strings.Contains(out, "hh-shared") || !strings.Contains(out, "hh-ops") {
		t.Errorf("--all did not mount them:\n%s", out)
	}
	out = te.ok("store", "discover", "--from", remotes, "--json")
	var got struct {
		Candidates []map[string]string `json:"candidates"`
		Mounted    []string            `json:"mounted"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || len(got.Candidates) != 0 {
		t.Errorf("mounted ones are offered again: %s", out)
	}
}

func TestDiscoverGitHub(t *testing.T) {
	te := newTestEnv(t)
	d := t.TempDir()
	te.withFake(fakegh.State{
		Tokens: map[string]string{"t-alice": "alice", "t-corp": "alice-corp"},
		Repos: []*fakegh.Repo{
			{Owner: "org-a", Name: "hh-shared", Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice-corp": "write", "alice": "read"}},
			{Owner: "org-b", Name: "hh-shared", Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice-corp": "read"}},
			{Owner: "org-a", Name: "web", Perms: map[string]string{"alice-corp": "write"}},
		},
	})
	te.env["T_ALICE"], te.env["T_CORP"] = "t-alice", "t-corp"
	te.writeConfigYAML("accounts:\n  - {name: personal, host: github.com, login: alice, token: \"env:T_ALICE\"}\n  - {name: work, host: github.com, login: alice-corp, token: \"env:T_CORP\"}\n")
	out := te.ok("store", "discover", "--json")
	var got struct {
		Candidates []struct{ Name, URL, Account, Permission string } `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 2 {
		t.Fatalf("candidates = %+v", got.Candidates)
	}
	a, b := got.Candidates[0], got.Candidates[1]
	// org-a/hh-shared is seen by both accounts; work can push, so work takes it.
	if a.Name != "hh-shared" || a.Account != "work" || a.Permission != "write" || a.URL != "https://github.com/org-a/hh-shared.git" {
		t.Errorf("first = %+v", a)
	}
	// The same repo name in another org gets the owner in front.
	if b.Name != "org-b-hh-shared" || b.Permission != "read" {
		t.Errorf("second = %+v", b)
	}
	// Picking one on a terminal adds it through store add, with its account.
	maker := newTestEnv(t)
	maker.ok("store", "init", "hh-shared", "--prefix", "SH", "--remote", filepath.Join(d, "org-a", "hh-shared.git"))
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), nil, nil)
	te.terminal, te.stdin = true, "1\n"
	te.ok("store", "discover")
	b, _ := os.ReadFile(filepath.Join(te.hhHome, "config.yaml"))
	if !strings.Contains(string(b), "name: hh-shared") || !strings.Contains(string(b), "account: work") {
		t.Errorf("config.yaml = %s", b)
	}
}

func TestDiscoverTerminal(t *testing.T) {
	remotes := filepath.Join(t.TempDir(), "remotes")
	maker := newTestEnv(t)
	maker.writeConfigYAML("url_rewrites:\n  - {from: \"https://github.com/\", to: \"" + filepath.ToSlash(remotes) + "/\"}\n")
	maker.ok("store", "init", "aa", "--prefix", "AA", "--remote", "https://github.com/o/aa.git")
	maker.ok("store", "init", "bb", "--prefix", "BB", "--remote", "https://github.com/o/bb.git")
	te := newTestEnv(t)
	te.writeConfigYAML("url_rewrites:\n  - {from: \"https://github.com/\", to: \"" + filepath.ToSlash(remotes) + "/\"}\n")
	te.terminal, te.stdin = true, "2\n"
	te.ok("store", "discover", "--from", remotes)
	out := te.ok("store", "list")
	if strings.Contains(out, "aa") || !strings.Contains(out, "bb") {
		t.Errorf("picked 2, got:\n%s", out)
	}
}

// The names offered: the repo name; <owner>-<repo> when that is used; then
// <owner>-<repo>-2, -3...; and store-1, store-2... where no valid name can be
// made of the owner and the repo.
func TestNameCandidates(t *testing.T) {
	c := &config.Config{Stores: []config.StoreEntry{{Name: "hh-shared"}, {Name: "org-a-hh-shared"}}}
	// In order: both names used; lower case; the repo name used; no valid
	// name (twice).
	cands := []candidate{
		{URL: "https://github.com/org-a/hh-shared.git"},
		{URL: "https://github.com/org-b/Hh_Shared.git"},
		{URL: "https://github.com/org-b/hh-shared.git"},
		{URL: "/srv/日本/日本.git"},
		{URL: "/srv/語/語.git"},
	}
	nameCandidates(c, cands)
	want := []string{"org-a-hh-shared-2", "hh_shared", "org-b-hh-shared", "store-1", "store-2"}
	for i, x := range cands {
		if x.Name != want[i] {
			t.Errorf("name of %s = %q, want %q", x.URL, x.Name, want[i])
		}
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./cmd/hedgehug/ -run TestDiscover`
Expected: FAIL（`unknown command "discover"`）

- [ ] **Step 3: store_discover.go を書く**

```go
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/config"
	hhctx "github.com/kukv/hedgehug/internal/context"
	"github.com/kukv/hedgehug/internal/github"
	"github.com/kukv/hedgehug/internal/gitrepo"
)

// candidate is a store that discover offers.
type candidate struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	Account    string `json:"account,omitempty"`
	Permission string `json:"permission"` // read or write
	key        string // what tells it from a mounted store
}

func (a *app) storeDiscoverCmd() *cobra.Command {
	var account, from string
	var all bool
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Find the stores you can read and offer to add them",
		Long:  "Discover lists the GitHub repositories with the hedgehug-store topic that the accounts can read, or with --from the bare repositories under a directory that hold a store, leaving out the stores set up already. On a terminal it asks which to add; elsewhere --all adds every one.",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.storeDiscover(account, from, all)
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "look with this account only")
	cmd.Flags().StringVar(&from, "from", "", "look for bare repositories under this directory instead of on GitHub")
	cmd.Flags().BoolVar(&all, "all", false, "add every store found")
	return cmd
}

func (a *app) storeDiscover(account, from string, all bool) error {
	c, _, err := a.config()
	if err != nil {
		return err
	}
	var cands []candidate
	if from != "" {
		cands, err = a.discoverFrom(c, from, account)
	} else {
		cands, err = a.discoverGitHub(c, account)
	}
	if err != nil {
		return err
	}
	cands = dropMounted(c, cands)
	nameCandidates(c, cands)
	var picked []candidate
	switch {
	case all:
		picked = cands
	case a.isTerminal(a.stdin) && !a.jsonOut && len(cands) > 0:
		if err := a.printCandidates(cands); err != nil {
			return err
		}
		if picked, err = a.pickCandidates(cands); err != nil {
			return err
		}
	}
	var mounted []string
	var errs []error
	for _, x := range picked {
		if err := a.storeAdd(x.Name, []string{x.URL}, "", x.Account); err != nil {
			errs = append(errs, fmt.Errorf("add %s: %w", x.Name, err))
			continue
		}
		mounted = append(mounted, x.Name)
	}
	if a.jsonOut {
		if cands == nil {
			cands = []candidate{}
		}
		if mounted == nil {
			mounted = []string{}
		}
		if err := a.printJSON(struct {
			Candidates []candidate `json:"candidates"`
			Mounted    []string    `json:"mounted"`
		}{cands, mounted}); err != nil {
			return err
		}
		return errors.Join(errs...)
	}
	if len(cands) == 0 {
		_, err := fmt.Fprintln(a.stdout, "No store found")
		return err
	}
	if !all && !a.isTerminal(a.stdin) {
		if err := a.printCandidates(cands); err != nil {
			return err
		}
	}
	return errors.Join(errs...)
}

func (a *app) printCandidates(cands []candidate) error {
	rows := make([][]string, len(cands))
	for i, x := range cands {
		acct := x.Account
		if acct == "" {
			acct = "-"
		}
		rows[i] = []string{strconv.Itoa(i + 1), x.Name, x.URL, acct, x.Permission}
	}
	if err := writeTable(a.stdout, []string{"#", "NAME", "URL", "ACCOUNT", "PERMISSION"}, rows, nil); err != nil {
		return err
	}
	_, err := fmt.Fprintln(a.stdout, "To add one under another name, use hedgehug store add <name> --remote <url>")
	return err
}

// pickCandidates asks which candidates to add: numbers with commas, all, or
// nothing to stop.
func (a *app) pickCandidates(cands []candidate) ([]candidate, error) {
	if _, err := fmt.Fprint(a.stderr, "Add which? (e.g. 1,3 / all / empty to stop) "); err != nil {
		return nil, err
	}
	line, _ := bufio.NewReader(a.stdin).ReadString('\n')
	line = strings.TrimSpace(line)
	switch strings.ToLower(line) {
	case "":
		return nil, nil
	case "all":
		return cands, nil
	}
	var out []candidate
	for _, f := range strings.Split(line, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || n < 1 || n > len(cands) {
			return nil, usageErrorf("%q is not a number from the list", strings.TrimSpace(f))
		}
		out = append(out, cands[n-1])
	}
	return out, nil
}

// discoverGitHub lists the repositories with the hedgehug-store topic that
// the accounts with a token can read. A repository two accounts see goes
// with the one that can push, else the one listed first.
func (a *app) discoverGitHub(c *config.Config, only string) ([]candidate, error) {
	var accts []config.Account
	for _, x := range c.Accounts {
		if (only == "" && x.Token.Kind != "") || x.Name == only {
			accts = append(accts, x)
		}
	}
	if only != "" && len(accts) == 0 {
		return nil, fmt.Errorf("no account is named %q (accounts: %s)", only, accountNames(c))
	}
	if len(accts) == 0 {
		return nil, errors.New("no account has a token to look on GitHub with; run hedgehug auth login, or use --from")
	}
	var out []candidate
	for _, acct := range accts {
		gh, err := a.githubFor(c, acct, acct.Host)
		if err != nil {
			return nil, err
		}
		repos, err := gh.UserRepos()
		if err != nil {
			return nil, github.Describe(err, acct.ID(), acct.Name, "list the repositories")
		}
		for _, r := range repos {
			if !slices.Contains(r.Topics, "hedgehug-store") {
				continue
			}
			perm := "read"
			if r.Permissions.Push {
				perm = "write"
			}
			key, _ := hhctx.RepoID(r.CloneURL)
			x := candidate{URL: r.CloneURL, Account: acct.Name, Permission: perm, key: key}
			if i := slices.IndexFunc(out, func(y candidate) bool { return y.key == key }); i >= 0 {
				if out[i].Permission == "read" && perm == "write" {
					out[i] = x
				}
				continue
			}
			out = append(out, x)
		}
	}
	return out, nil
}

// discoverFrom lists the bare repositories under dir whose HEAD holds a
// store. The URL goes back through url_rewrites to the form before them.
func (a *app) discoverFrom(c *config.Config, dir, account string) ([]candidate, error) {
	acct, ok, err := a.pickAccount(c, account)
	if err != nil {
		return nil, err
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	var out []candidate
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || !isBare(p) {
			return err
		}
		if has, err := gitrepo.HeadHasFile(p, ".hedgehug/store.yaml"); err == nil && has {
			perm := "read"
			if gitrepo.Writable(p) {
				perm = "write"
			}
			x := candidate{URL: c.Unrewrite(filepath.ToSlash(p)), Permission: perm, key: filepath.Clean(p)}
			if ok {
				x.Account = acct.Name
			}
			out = append(out, x)
		}
		return filepath.SkipDir
	})
	return out, err
}

func isBare(p string) bool {
	if fi, err := os.Stat(filepath.Join(p, "HEAD")); err != nil || fi.IsDir() { //nolint:gosec // G703: p is a directory under the --from the person gave
		return false
	}
	for _, d := range []string{"objects", "refs"} {
		if fi, err := os.Stat(filepath.Join(p, d)); err != nil || !fi.IsDir() { //nolint:gosec // G703: p is a directory under the --from the person gave
			return false
		}
	}
	return true
}

// dropMounted leaves out the candidates that a store set up already uses: by
// repo ID, or by the cleaned path for a local remote.
func dropMounted(c *config.Config, cands []candidate) []candidate {
	have := map[string]bool{}
	for _, e := range c.Stores {
		for _, r := range e.Remotes {
			if id, ok := hhctx.RepoID(r); ok {
				have[id] = true
			}
			if p, ok := gitrepo.LocalPath(c.Rewrite(r)); ok {
				have[filepath.Clean(p)] = true
			}
		}
	}
	return slices.DeleteFunc(cands, func(x candidate) bool {
		if id, ok := hhctx.RepoID(x.URL); ok && have[id] {
			return true
		}
		if p, ok := gitrepo.LocalPath(c.Rewrite(x.URL)); ok && have[filepath.Clean(p)] {
			return true
		}
		return have[x.key]
	})
}

var badNameChars = regexp.MustCompile(`[^a-z0-9_-]+`)

// nameCandidates gives each candidate a store name (spec: the repo name; when
// that is used, <owner>-<repo>; when that is used too, -2, -3 after it). A
// name the rule of store names (model.ValidName) does not take is made lower
// case with - for the other characters. Where nothing valid can be made of
// the owner and the repo, the name is store-1, store-2... (the first number
// that is not used, from 1).
func nameCandidates(c *config.Config, cands []candidate) {
	used := map[string]bool{}
	for _, e := range c.Stores {
		used[e.Name] = true
	}
	fix := func(s string) string {
		s = strings.Trim(badNameChars.ReplaceAllString(strings.ToLower(s), "-"), "-")
		if config.ValidStoreName(s) != nil {
			return ""
		}
		return s
	}
	for i := range cands {
		u := strings.TrimSuffix(filepath.ToSlash(cands[i].URL), "/")
		repo := strings.TrimSuffix(path.Base(u), ".git")
		owner := path.Base(path.Dir(u))
		var name string
		base := fix(owner + "-" + repo)
		for _, n := range []string{fix(repo), base} {
			if n != "" && !used[n] {
				name = n
				break
			}
		}
		switch {
		case name != "":
		case base != "":
			// <owner>-<repo> is used too: -2, -3...
			for n := 2; name == ""; n++ {
				if x := base + "-" + strconv.Itoa(n); !used[x] {
					name = x
				}
			}
		default:
			// Nothing valid can be made of the owner and the repo: store-1, store-2...
			for n := 1; name == ""; n++ {
				if x := "store-" + strconv.Itoa(n); !used[x] {
					name = x
				}
			}
		}
		used[name] = true
		cands[i].Name = name
	}
}
```

`storeCmd` の `AddCommand` に `a.storeDiscoverCmd()` を足す。

- [ ] **Step 4: テストを通す**

Run: `go test -race ./cmd/hedgehug/ -run 'TestDiscover|TestNameCandidates'`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add cmd/hedgehug
git commit -m "feat(cli): store discover で GitHub とディレクトリから Store を見つけてマウントする"
```

---

### Task 13: devenv の 3a 分と 3a の E2E

**Files:**
- Modify: `tools/devenv/main.go`（全体を下の内容にする）
- Test: `tools/devenv/main_test.go`
- Create: `cmd/hedgehug/e2e_3a_test.go`
- Modify: `cmd/hedgehug/e2e_test.go`（`TestE2EAliceAndBob` の devenv の用意を `initDevenv` に置き換える）

**Interfaces:**
- Consumes: すべての前の Task
- Produces:
  - `devenv init [--dir d] [--fakegh <url>]`、`devenv reset [--dir d] [--fakegh <url>]`（`--fakegh` の既定は `http://127.0.0.1:7777`。各ユーザーの config.yaml の `hosts` の `api` と `web` になる）
  - `<dir>/fakegh.json`（spec の「devenv」の初期データ）
  - 各ユーザーの config.yaml: `accounts`（token は env）、`hosts`（github.com を fakegh へ）、`url_rewrites`（`https://github.com/` → `<dir>/remotes/`）、`stores`（remote は `https://github.com/...`）、`default_store`
  - 各ユーザーの `.gitconfig`: `[user]` と、同じ書き換えの `[url "<dir>/remotes/"] insteadOf = https://github.com/`
  - `devenv env <user>` が `HH_DEV_TOKEN`（alice は `HH_DEV_TOKEN_WORK` も）を export する
  - alice: アカウント personal（alice、`env:HH_DEV_TOKEN`）と work（alice-corp、`env:HH_DEV_TOKEN_WORK`）。Store は me（`https://github.com/kukv/hh-tasks.git`、personal）、org-a（`https://github.com/org-a/hh-shared.git`、work、接頭辞 ORG）、org-b（`https://github.com/org-b/hh-ops.git`、work、接頭辞 OPS）
  - bob: アカウント personal（bob、`env:HH_DEV_TOKEN`）。Store は me（ローカル、remote なし）、org-a と org-b（alice が push したものの clone、personal）
  - fakegh.json: users alice・alice-corp・bob、orgs org-a・org-b（alice-corp・bob）、repos 4件（`kukv/hh-tasks` alice admin、`org-a/hh-shared` alice-corp admin・bob write、`org-b/hh-ops` alice-corp admin・bob read、この3件は topic `hedgehug-store`。`org-a/web` は topic なしで alice-corp・bob が write）、tokens（`dev-alice`→alice、`dev-alice-corp`→alice-corp、`dev-bob`→bob）
  - テスト用 `initDevenv(t, devenv) (dev string, fake *fakegh.Server, apiURL string)`（`cmd/hedgehug/e2e_3a_test.go`）: httptest で fakegh を起動し、その URL を `--fakegh` に渡して `devenv init` し、できた `fakegh.json` で fakegh を `Reset` する。`apiURL` は httptest の URL。`TestE2EAliceAndBob` と `TestE2EPhase3a` が使う

- [ ] **Step 1: devenv のテストを直し、新しいテストを書く**

`tools/devenv/main_test.go` の既存のテストのうち、3つが新しい構成に合わなくなる。次のとおり直す（`TestInitCreatesUsers`・`TestInitPushesAlicesStoreOnly` とそれ以外は、そのままで通る。`TestInitCreatesUsers` の「`me` の最初のコミットの作者が u」は、me のアカウントが personal（login は u）のままなので変わらない）。

1. `TestInitMakesSharedStore` を次に置き換える。alice の org-a は work（alice-corp）のアカウントで作るので、最初のコミットの作者とメールが alice-corp になり、`account:` の数がユーザーごとに変わる:

```go
func TestInitMakesSharedStore(t *testing.T) {
	dir := initDir(t)
	for _, p := range []string{"remotes/kukv/hh-tasks.git/HEAD", "remotes/org-a/hh-shared.git/HEAD", "remotes/org-b/hh-ops.git/HEAD"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
	for _, tc := range []struct {
		user           string
		personal, work int // the stores that name each account
	}{{"alice", 1, 2}, {"bob", 3, 0}} {
		u := tc.user
		home := filepath.Join(dir, "users", u)
		cfg, err := os.ReadFile(filepath.Join(home, "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"login: " + u, "host: github.com", "name: org-a", "hh-shared.git"} {
			if !strings.Contains(string(cfg), want) {
				t.Errorf("%s config.yaml lacks %q:\n%s", u, want, cfg)
			}
		}
		if n := strings.Count(string(cfg), "account: personal"); n != tc.personal {
			t.Errorf("%s config.yaml names personal on %d stores, want %d:\n%s", u, n, tc.personal, cfg)
		}
		if n := strings.Count(string(cfg), "account: work"); n != tc.work {
			t.Errorf("%s config.yaml names work on %d stores, want %d:\n%s", u, n, tc.work, cfg)
		}
		s, err := store.Open(filepath.Join(home, "stores", "org-a"))
		if err != nil {
			t.Fatalf("%s has no org-a: %v", u, err)
		}
		if ok, err := s.Repo().MergeDriverRegistered(); err != nil || !ok {
			t.Errorf("%s: merge driver registered = %v, %v", u, ok, err)
		}
		if got := s.Repo().Remotes(); len(got) != 1 || got[0] != "origin" {
			t.Errorf("%s: org-a remotes = %v, want [origin]", u, got)
		}
		log, err := s.Repo().Log(nil, 0)
		if err != nil || len(log) == 0 {
			t.Fatalf("%s: log = %v, %v", u, log, err)
		}
		if got := log[len(log)-1].Author.Name; got != "alice-corp" {
			t.Errorf("%s: first commit of org-a by %q, want alice-corp (alice's work account)", u, got)
		}
		if got, want := log[0].Author.Email, "alice-corp@users.noreply.github.com"; got != want {
			t.Errorf("%s: org-a head author email = %q, want %q", u, got, want)
		}
	}
}
```

2. `TestEnv` の `want` を、トークンの export を足した形にする:

```go
	want := "export HEDGEHUG_HOME='" + home + "'\nexport GIT_CONFIG_GLOBAL='" + filepath.Join(home, ".gitconfig") + "'\n" +
		"export HH_DEV_TOKEN='dev-alice'\nexport HH_DEV_TOKEN_WORK='dev-alice-corp'\n"
```

3. `TestUsageErrors` の表に、使えない `--fakegh` を足す: `{"init", "--fakegh", "not a url"}`（終了コード2）。

4. 新しいテストを足す（import に `bytes` は既にあるので、`internal/fakegh` を足す）:

```go
func TestInitPhase3a(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dev")
	var out, errb bytes.Buffer
	if code := run([]string{"init", "--dir", dir, "--fakegh", "http://127.0.0.1:9999"}, &out, &errb, t.TempDir()); code != 0 {
		t.Fatalf("init: %d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "go run ./cmd/fakegh serve --addr 127.0.0.1:9999 --seed ") {
		t.Errorf("init does not say how to start the fake:\n%s", out.String())
	}
	st, err := fakegh.Load(filepath.Join(dir, "fakegh.json"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Tokens["dev-alice-corp"] != "alice-corp" || st.Tokens["dev-alice"] != "alice" || st.Tokens["dev-bob"] != "bob" {
		t.Errorf("tokens = %v", st.Tokens)
	}
	if len(st.Repos) != 4 {
		t.Fatalf("repos = %d, want 4", len(st.Repos))
	}
	for _, r := range st.Repos {
		hasTopic := len(r.Topics) == 1 && r.Topics[0] == "hedgehug-store"
		if isWeb := r.Owner == "org-a" && r.Name == "web"; isWeb == hasTopic {
			t.Errorf("%s/%s topics = %v: only org-a/web has none", r.Owner, r.Name, r.Topics)
		}
	}
	if st.Repos[2].Perms["bob"] != "read" {
		t.Errorf("org-b/hh-ops perms = %v, bob reads it only", st.Repos[2].Perms)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "users", "alice", "config.yaml"))
	for _, want := range []string{"http://127.0.0.1:9999", "url_rewrites", "https://github.com/org-b/hh-ops.git", "login: alice-corp", "token: env:HH_DEV_TOKEN_WORK", "default_store: me"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("alice's config lacks %q:\n%s", want, b)
		}
	}
	gc, _ := os.ReadFile(filepath.Join(dir, "users", "alice", ".gitconfig"))
	if !strings.Contains(string(gc), "insteadOf = https://github.com/") {
		t.Errorf("alice's .gitconfig = %q", gc)
	}
	for _, r := range []string{"kukv/hh-tasks.git", "org-a/hh-shared.git", "org-b/hh-ops.git"} {
		if ok, err := gitrepo.HeadHasFile(filepath.Join(dir, "remotes", r), ".hedgehug/store.yaml"); err != nil || !ok {
			t.Errorf("%s has no store: %v %v", r, ok, err)
		}
	}
	if code := run([]string{"init", "--dir", filepath.Join(t.TempDir(), "x"), "--fakegh", "not a url"}, &out, &errb, t.TempDir()); code != 2 {
		t.Errorf("a --fakegh that is not a URL: exit %d, want 2", code)
	}
}
```

（import に `github.com/kukv/hedgehug/internal/fakegh` と `github.com/kukv/hedgehug/internal/gitrepo` を足す。）

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./tools/devenv/`
Expected: FAIL（`--fakegh` が未知のフラグ、`fakegh.json` がない）

- [ ] **Step 3: devenv を書き換える**

`tools/devenv/main.go` を次の内容にする（`resolveDir`・`findRoot`・`shellQuote` は今のコードのまま。`userEnv`・`newUserEnv`・`initShared` はなくなり、`initUser` と `addDevStore` に置き換わる）:

```go
// Command devenv sets up a throwaway development environment with two users,
// alice and bob, each with their own HEDGEHUG_HOME, git identity, accounts and
// stores, and the seed of the fake GitHub (fakegh) they talk to. It is for
// developers of Hedgehug and is not shipped.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

const (
	markerFile    = ".hedgehug-devenv"
	defaultFakegh = "http://127.0.0.1:7777"
)

// bareRemotes are the bare repositories under the environment's directory:
// the "GitHub" that url_rewrites points https://github.com/ at.
var bareRemotes = []string{
	filepath.Join("remotes", "kukv", "hh-tasks.git"),
	filepath.Join("remotes", "org-a", "hh-shared.git"),
	filepath.Join("remotes", "org-b", "hh-ops.git"),
}

var users = []string{"alice", "bob"}

// account is an account of a user's config.yaml; its token is in the
// environment variable tokenVar.
type account struct{ name, login, tokenVar string }

// accounts are the accounts of each user: alice has her own and the one of
// her organizations.
var accounts = map[string][]account{
	"alice": {{"personal", "alice", "HH_DEV_TOKEN"}, {"work", "alice-corp", "HH_DEV_TOKEN_WORK"}},
	"bob":   {{"personal", "bob", "HH_DEV_TOKEN"}},
}

// tokens are what each user's HH_DEV_TOKEN* hold; devSeed has the same.
var tokens = map[string]map[string]string{
	"alice": {"HH_DEV_TOKEN": "dev-alice", "HH_DEV_TOKEN_WORK": "dev-alice-corp"},
	"bob":   {"HH_DEV_TOKEN": "dev-bob"},
}

// devStore is a store of a user: its remote as config.yaml has it (on
// "GitHub"; empty for a store with no remote), the id prefix of its first
// workspace, the account that writes it, and whether the user clones it (the
// one before made it) or makes it.
type devStore struct {
	name, remote, prefix, account string
	clone                         bool
}

// stores are the stores of each user, in the order they are made: alice's
// first, so that bob can clone what she pushed.
var stores = map[string][]devStore{
	"alice": {
		{name: "me", remote: "https://github.com/kukv/hh-tasks.git", prefix: "DEV", account: "personal"},
		{name: "org-a", remote: "https://github.com/org-a/hh-shared.git", prefix: "ORG", account: "work"},
		{name: "org-b", remote: "https://github.com/org-b/hh-ops.git", prefix: "OPS", account: "work"},
	},
	"bob": {
		{name: "me", prefix: "DEV", account: "personal"},
		{name: "org-a", remote: "https://github.com/org-a/hh-shared.git", prefix: "ORG", account: "personal", clone: true},
		{name: "org-b", remote: "https://github.com/org-b/hh-ops.git", prefix: "OPS", account: "personal", clone: true},
	},
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "devenv:", err)
		os.Exit(1)
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, cwd))
}

// usageError marks a mistake in the arguments (exit code 2).
type usageError struct{ error }

func run(args []string, stdout, stderr io.Writer, cwd string) int {
	err := dispatch(args, stdout, stderr, cwd)
	var ue usageError
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return 0
	case errors.As(err, &ue):
		_, _ = fmt.Fprintln(stderr, "devenv:", err) // nothing useful to do if stderr is unwritable
		return 2
	default:
		_, _ = fmt.Fprintln(stderr, "devenv:", err) // nothing useful to do if stderr is unwritable
		return 1
	}
}

func dispatch(args []string, stdout, stderr io.Writer, cwd string) error {
	if len(args) == 0 {
		return usageError{errors.New("usage: devenv init|reset|env <user> [--dir <path>] [--fakegh <url>]")}
	}
	cmd, rest := args[0], args[1:]
	fs := flag.NewFlagSet("devenv "+cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "directory of the environment (default: .devenv at the repository root, found by walking up to go.mod)")
	fakeghURL := fs.String("fakegh", defaultFakegh, "the URL of the fake GitHub that the users' config.yaml point github.com at (init and reset)")

	var want int // number of positional arguments
	switch cmd {
	case "init", "reset":
	case "env":
		want = 1
	default:
		return usageError{fmt.Errorf("unknown command %q (init, reset or env)", cmd)}
	}
	// Allow the flag before or after the positional argument.
	var pos []string
	for len(rest) > 0 {
		if err := fs.Parse(rest); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return err
			}
			return usageError{err}
		}
		rest = fs.Args()
		if len(rest) > 0 {
			pos = append(pos, rest[0])
			rest = rest[1:]
		}
	}
	if len(pos) != want {
		return usageError{fmt.Errorf("%s takes %d argument(s), got %d", cmd, want, len(pos))}
	}
	if u, err := url.Parse(*fakeghURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return usageError{fmt.Errorf("--fakegh %q: want an http or https URL", *fakeghURL)}
	}

	d, err := resolveDir(*dir, cwd)
	if err != nil {
		return err
	}
	switch cmd {
	case "init":
		return initEnv(d, *fakeghURL, stdout)
	case "reset":
		return reset(d, *fakeghURL, stdout)
	default:
		return printEnv(d, pos[0], stdout)
	}
}

// resolveDir is the absolute directory of the environment. A relative --dir
// is relative to cwd; without --dir it is .devenv in the repository root.
func resolveDir(dir, cwd string) (string, error) {
	if dir == "" {
		root, err := findRoot(cwd)
		if err != nil {
			return "", err
		}
		return filepath.Join(root, ".devenv"), nil
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(cwd, dir)
	}
	return filepath.Clean(dir), nil
}

// findRoot walks up from dir to the directory that holds go.mod.
func findRoot(dir string) (string, error) {
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", fmt.Errorf("no go.mod found above %s; run in the repository or pass --dir", dir)
		}
		d = parent
	}
}

func initEnv(dir, fakeghURL string, stdout io.Writer) error {
	if _, err := os.Lstat(dir); err == nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return fmt.Errorf("%s already exists; run `devenv reset` to recreate it", dir)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, markerFile), nil, 0o600); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	for _, r := range bareRemotes {
		if err := gitrepo.InitBare(filepath.Join(dir, r)); err != nil {
			return err
		}
	}
	seed := filepath.Join(dir, "fakegh.json")
	if err := writeSeed(seed); err != nil {
		return err
	}
	for _, u := range users {
		if err := initUser(dir, u, fakeghURL); err != nil {
			return fmt.Errorf("%s: %w", u, err)
		}
	}
	fu, err := url.Parse(fakeghURL)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Created %s with users %s.\nStart the fake GitHub with: go run ./cmd/fakegh serve --addr %s --seed %s\nUse one with: eval \"$(go run ./tools/devenv env %s --dir %s)\"\n",
		dir, strings.Join(users, " and "), fu.Host, shellQuote(seed), users[0], shellQuote(dir))
	return err
}

// devSeed is the state the fake GitHub starts with: the users and the two
// organizations, the repositories (the three stores have the hedgehug-store
// topic; org-a/web is code and has none), who may do what with them, and the
// tokens the HH_DEV_TOKEN variables hold. bob reads org-b/hh-ops only.
func devSeed() fakegh.State {
	interval := 5
	return fakegh.State{
		Users: []string{"alice", "alice-corp", "bob"},
		Orgs: map[string][]string{
			"org-a": {"alice-corp", "bob"},
			"org-b": {"alice-corp", "bob"},
		},
		Tokens: map[string]string{"dev-alice": "alice", "dev-alice-corp": "alice-corp", "dev-bob": "bob"},
		Repos: []*fakegh.Repo{
			{Owner: "kukv", Name: "hh-tasks", Private: true, Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice": "admin"}},
			{Owner: "org-a", Name: "hh-shared", Private: true, Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice-corp": "admin", "bob": "write"}},
			{Owner: "org-b", Name: "hh-ops", Private: true, Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice-corp": "admin", "bob": "read"}},
			{Owner: "org-a", Name: "web", Private: true, Perms: map[string]string{"alice-corp": "write", "bob": "write"}},
		},
		DeviceInterval: &interval,
	}
}

// writeSeed writes devSeed to path, for `fakegh serve --seed`.
func writeSeed(path string) error {
	b, err := json.MarshalIndent(devSeed(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600) //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
}

// configHead is the start of the config.yaml of user u, written before the
// stores are added: the accounts with their token variables, github.com on
// the fake, and url_rewrites to the bare repositories.
func configHead(dir, u, fakeghURL string) string {
	var b strings.Builder
	b.WriteString("accounts:\n")
	for _, a := range accounts[u] {
		fmt.Fprintf(&b, "  - name: %s\n    host: github.com\n    login: %s\n    token: env:%s\n", a.name, a.login, a.tokenVar)
	}
	fmt.Fprintf(&b, "hosts:\n  - host: github.com\n    api: %s\n    web: %s\n", fakeghURL, fakeghURL)
	fmt.Fprintf(&b, "url_rewrites:\n  - from: \"https://github.com/\"\n    to: %q\n", filepath.ToSlash(filepath.Join(dir, "remotes"))+"/")
	return b.String()
}

// initUser makes the home of the user u: .gitconfig (the author, and the
// rewrite of url_rewrites as insteadOf), config.yaml and the stores.
func initUser(dir, u, fakeghURL string) error {
	home := filepath.Join(dir, "users", u)
	if err := os.MkdirAll(home, 0o750); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	gitconfig := "[user]\n\tname = " + u + "\n\temail = " + u + "@example.com\n" +
		"[url \"" + filepath.ToSlash(filepath.Join(dir, "remotes")) + "/\"]\n\tinsteadOf = https://github.com/\n"
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gitconfig), 0o600); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	getenv := func(k string) string {
		switch k {
		case "HEDGEHUG_HOME", "HOME", "USERPROFILE":
			return home
		}
		return ""
	}
	p, err := config.Resolve(getenv)
	if err != nil {
		return err
	}
	// The accounts, hosts and url_rewrites are written before the config is
	// loaded, so that AddStore keeps them.
	if err := os.WriteFile(p.ConfigFile, []byte(configHead(dir, u, fakeghURL)), 0o600); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	for _, ds := range stores[u] {
		if err := addDevStore(p, getenv, ds); err != nil {
			return fmt.Errorf("store %s: %w", ds.name, err)
		}
	}
	return nil
}

// addDevStore makes (and pushes) or clones the store ds as the user of p, and
// adds it to their config.yaml with its account. It works in this process, so
// no background sync starts, and reaches the remote through url_rewrites.
func addDevStore(p config.Paths, getenv func(string) string, ds devStore) error {
	c, err := config.Load(p)
	if err != nil {
		return err
	}
	acct, ok := c.Account(ds.account)
	if !ok {
		return fmt.Errorf("no account named %s", ds.account)
	}
	author, err := config.CommitAuthor(acct, true, getenv)
	if err != nil {
		return err
	}
	var remotes []string
	if ds.remote != "" {
		remotes = []string{ds.remote}
	}
	conn := gitrepo.Conn{Rewrite: c.Rewrite}
	dir := filepath.Join(p.StoresDir, ds.name)
	var s *store.Store
	if ds.clone {
		if _, err = gitrepo.CloneWith(ds.remote, dir, conn); err != nil {
			return err
		}
		s, err = store.Open(dir)
	} else {
		ws := &model.Workspace{ULID: model.NewULID(), Name: ds.name, IDPrefix: ds.prefix}
		s, err = store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: ds.name}, nil, ws, author, acct.ID())
	}
	if err != nil {
		return err
	}
	if err := s.Repo().RegisterMergeDriver(); err != nil {
		return err
	}
	if !ds.clone && len(remotes) > 0 {
		if _, err := sync.Run(s, sync.Options{Remotes: remotes, Author: author, Account: acct.ID(), Me: c.Me(), Conn: conn}); err != nil {
			return err
		}
	}
	if err := c.AddStore(ds.name, dir, remotes); err != nil {
		return err
	}
	if err := c.SetStoreAccount(ds.name, ds.account); err != nil {
		return err
	}
	return c.Save()
}

// reset removes dir when it is a devenv (it holds the marker file) and
// creates it again. A missing dir is only created.
func reset(dir, fakeghURL string, stdout io.Writer) error {
	if _, err := os.Lstat(dir); err == nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		if _, err := os.Stat(filepath.Join(dir, markerFile)); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
			return fmt.Errorf("%s was not made by devenv (no %s in it); not removing it", dir, markerFile)
		}
		if err := os.RemoveAll(dir); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
			return err
		}
	}
	return initEnv(dir, fakeghURL, stdout)
}

func printEnv(dir, user string, stdout io.Writer) error {
	home := filepath.Join(dir, "users", user)
	if !slices.Contains(users, user) {
		return fmt.Errorf("no user named %q (users: %s)", user, strings.Join(users, ", "))
	}
	if _, err := os.Stat(home); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return fmt.Errorf("%s: %w; run `devenv init` first", home, err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "export HEDGEHUG_HOME=%s\nexport GIT_CONFIG_GLOBAL=%s\n", shellQuote(home), shellQuote(filepath.Join(home, ".gitconfig")))
	for _, k := range slices.Sorted(maps.Keys(tokens[user])) {
		fmt.Fprintf(&b, "export %s=%s\n", k, shellQuote(tokens[user][k]))
	}
	_, err := io.WriteString(stdout, b.String())
	return err
}

// shellQuote quotes s for sh with single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
```

既存の `TestE2EAliceAndBob`（phase 2）は、devenv の構成が変わっても、alice の org-a と bob の org-a の同期が書き換えを通って動くことで通る。ただし、同期は GitHub のホストの Store で権限とコラボレーターを取りに行くので、fakegh がいないと毎回「接続できない」警告が出る（失敗にはならない）。`127.0.0.1:7777` に別の fakegh が立っていると結果が変わってしまうので、`e2e_test.go` の `TestE2EAliceAndBob` で、devenv の用意の 4 行

```go
	dev := filepath.Join(t.TempDir(), "hh-dev")
	if out, err := exec.Command(devenv, "init", "--dir", dev).CombinedOutput(); err != nil { //nolint:gosec // the binary the test built
		t.Fatalf("devenv init: %v\n%s", err, out)
	}
```

を、次の1行に置き換える（`initDevenv` は Step 5 で `e2e_3a_test.go` に書く）:

```go
	dev, _, _ := initDevenv(t, devenv)
```

- [ ] **Step 4: devenv のテストを通す**

Run: `go test ./tools/devenv/`
Expected: PASS

- [ ] **Step 5: 3a の E2E を書く**

`cmd/hedgehug/e2e_3a_test.go`。spec の 3a の E2E の項目と、このファイルのテストの対応:

| spec の項目 | テスト |
| --- | --- |
| `auth login`（Device Flow を fakegh で承認）→ `auth status` | `TestE2EAuthLoginAndStatus`（このプロセスの中で実行。下の理由）と、`TestE2EPhase3a` の最初の `auth status`（バイナリ、env のトークン） |
| `store init` のローカル分岐 | `TestE2EPhase3a`: alice が `https://github.com/kukv/hh-extra.git` を `url_rewrites` 付きで init し、bare ができて push され、fakegh にリポジトリができない |
| `store init` の GitHub 分岐（書き換えなしの構成で fakegh にリポジトリができる） | `TestE2EPhase3a`: `url_rewrites` のない構成で init し、fakegh に private のリポジトリと topic ができる |
| `discover` と `discover --from` | `TestE2EPhase3a` |
| bob の読み取り専用（書き込みの拒否、同期は fetch だけ、権限を戻すと外れる） | `TestE2EPhase3a` |

`auth login` を `TestE2EAuthLoginAndStatus` でバイナリではなくこのプロセスの中で動かす理由: バイナリは本物の OS の keychain にトークンを書く。CI のマシン（ubuntu）には Secret Service がなく、spec は keychain の代わりのファイル保存を作らないと決めているので、バイナリでは `auth login` が通らない。cmd のテストは `keyring.MockInit()`（go-keyring のメモリ上のモック）で keychain を差し替えているので、同じ方法で `run` を直接呼ぶ。`store init` の GitHub 分岐も、バイナリは書き換えなしの構成では本物の `github.com` へ push しに行ってしまうので、`hosts` に `127.0.0.1` を GitHub のホストとして登録し、remote を `http://127.0.0.1:1/...`（何も待ち受けていないポート）にする。リポジトリの作成と topic は fakegh に届き、push だけが失敗するので、「GitHub に作ったリポジトリを残し、`hedgehug store add` を案内する」ことまで確かめられる。

```go
package main

import (
	"bytes"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/store"
	hhsync "github.com/kukv/hedgehug/internal/sync"
)

// initDevenv runs devenv init with the fake GitHub of this test as the one the
// users' config.yaml point at, and loads the seed devenv wrote into the fake.
func initDevenv(t *testing.T, devenv string) (dev string, fake *fakegh.Server, apiURL string) {
	t.Helper()
	fake = fakegh.New(fakegh.State{})
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	dev = filepath.Join(t.TempDir(), "hh-dev")
	if out, err := exec.Command(devenv, "init", "--dir", dev, "--fakegh", srv.URL).CombinedOutput(); err != nil { //nolint:gosec // the binary the test built
		t.Fatalf("devenv init: %v\n%s", err, out)
	}
	seed, err := fakegh.Load(filepath.Join(dev, "fakegh.json"))
	if err != nil {
		t.Fatal(err)
	}
	fake.Reset(seed)
	return dev, fake, srv.URL
}

// fail runs hedgehug and returns its exit code and stderr; it fails the test
// when the command succeeds.
func (u *devUser) fail(args ...string) (int, string) {
	u.t.Helper()
	cmd := exec.Command(u.bin, args...) //nolint:gosec // the binary the test built
	cmd.Env = u.env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		u.t.Fatalf("hedgehug %s succeeded", strings.Join(args, " "))
	}
	return cmd.ProcessState.ExitCode(), stderr.String()
}

// TestE2EAuthLoginAndStatus logs in with the device flow, which the fake
// approves, and checks the login with auth status. It runs the commands in
// this process, with go-keyring's mock as the keychain (see above).
func TestE2EAuthLoginAndStatus(t *testing.T) {
	keyring.MockInit()
	zero := 0
	srv := httptest.NewServer(fakegh.New(fakegh.State{Users: []string{"carol"}, AutoApprove: "carol", DeviceInterval: &zero}))
	defer srv.Close()
	te := newTestEnv(t)
	te.writeConfigYAML("hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\", client_id: Iv23e2e}\n")
	code, out, errs := te.run("auth", "login")
	if code != 0 || !strings.Contains(errs, "enter the code") || !strings.Contains(out, "Logged in as github.com/carol") {
		t.Fatalf("auth login: exit %d\nstdout: %s\nstderr: %s", code, out, errs)
	}
	if out := te.ok("auth", "status"); !strings.Contains(out, "github.com/carol") || !strings.Contains(out, "device") || !strings.Contains(out, " ok") {
		t.Errorf("auth status:\n%s", out)
	}
}

func TestE2EPhase3a(t *testing.T) {
	bin, devenv := buildHedgehug(t), buildDevenv(t)
	dev, fake, apiURL := initDevenv(t, devenv)
	alice, bob := devUserOf(t, bin, devenv, dev, "alice"), devUserOf(t, bin, devenv, dev, "bob")

	// alice's two accounts each have their token, and GitHub says they are
	// of the logins in config.yaml.
	if out := alice.ok("auth", "status"); strings.Count(out, " ok") != 2 {
		t.Errorf("alice's auth status:\n%s", out)
	}

	// store init, local branch: url_rewrites points github.com at a local
	// path, so the command makes the bare there and pushes, and does not call
	// GitHub.
	alice.ok("store", "init", "extra", "--prefix", "EXT", "--account", "personal", "--remote", "https://github.com/kukv/hh-extra.git")
	if ok, err := gitrepo.HeadHasFile(filepath.Join(dev, "remotes", "kukv", "hh-extra.git"), ".hedgehug/store.yaml"); err != nil || !ok {
		t.Errorf("the bare of the local branch was not made and pushed: %v %v", ok, err)
	}
	if _, ok := fake.Repo("kukv/hh-extra"); ok {
		t.Error("the local branch made a repository on the fake GitHub")
	}

	// store init, GitHub branch: with no url_rewrites, the remote is on a
	// GitHub host (127.0.0.1 is one, by hosts), so the command makes the
	// repository there, through the fake, with the topic. The push then fails,
	// as nothing listens on port 1; the repository is left, and the way to use
	// it is told.
	dave := filepath.Join(t.TempDir(), "dave")
	if err := os.MkdirAll(dave, 0o750); err != nil {
		t.Fatal(err)
	}
	cfg := "accounts:\n  - {name: personal, host: 127.0.0.1, login: alice, token: \"env:HH_DEV_TOKEN\"}\nhosts:\n  - {host: 127.0.0.1, api: \"" + apiURL + "\", web: \"" + apiURL + "\"}\n"
	if err := os.WriteFile(filepath.Join(dave, "config.yaml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	d := &devUser{t: t, bin: bin, home: dave, env: append(slicesWithout(alice.env, "HEDGEHUG_HOME=", "HOME=", "USERPROFILE="), "HEDGEHUG_HOME="+dave, "HOME="+dave, "USERPROFILE="+dave)}
	code, errs := d.fail("store", "init", "gh", "--prefix", "GH", "--remote", "http://127.0.0.1:1/alice/hh-gh.git")
	if code != 1 || !strings.Contains(errs, "hedgehug store add gh") {
		t.Errorf("store init on the GitHub branch: exit %d, %q", code, errs)
	}
	if r, ok := fake.Repo("alice/hh-gh"); !ok || !r.Private || len(r.Topics) != 1 || r.Topics[0] != "hedgehug-store" {
		t.Errorf("the repository on the fake GitHub = %+v %v", r, ok)
	}

	// bob may only read org-b: the sync finds out, and writes are refused.
	bob.ok("sync")
	if code, errs := bob.fail("add", "x", "--store", "org-b"); code != 1 || !strings.Contains(errs, "org-b is read-only (the GitHub permission is read)") {
		t.Errorf("bob's add: exit %d, %q", code, errs)
	}
	bob.ok("add", "y", "--store", "org-a") // org-a he may write

	// The sync of a read-only store only fetches: what alice pushes comes in,
	// and nothing of bob's goes out.
	alice.ok("add", "from alice", "--store", "org-b")
	alice.ok("sync")
	bob.ok("sync")
	if out := bob.ok("ls", "--store", "org-b"); !strings.Contains(out, "from alice") {
		t.Errorf("bob's sync did not fetch alice's task:\n%s", out)
	}
	s, err := store.Open(filepath.Join(bob.home, "stores", "org-b"))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := hhsync.Unpushed(s.Repo()); err != nil || n != 0 {
		t.Errorf("bob has %d commits of org-b that origin lacks (%v), want none", n, err)
	}

	// Given write, he may write after the next check.
	if err := fake.SetPerm("org-b/hh-ops", "bob", "write"); err != nil {
		t.Fatal(err)
	}
	acc, _ := hhsync.ReadAccess(s)
	acc.Checked = time.Now().Add(-10 * time.Minute) // the 5 minutes have passed
	if err := hhsync.WriteAccess(s, acc); err != nil {
		t.Fatal(err)
	}
	bob.ok("sync")
	if acc, _ := hhsync.ReadAccess(s); acc.ReadOnly {
		t.Errorf("org-b is still read-only: %+v", acc)
	}
	bob.ok("add", "x", "--store", "org-b")

	// discover: a new store repository on GitHub (the code repository
	// org-a/web has no topic and is not offered), and the bares under remotes.
	if err := fake.AddRepo("org-a/hh-more", []string{"hedgehug-store"}, false); err != nil {
		t.Fatal(err)
	}
	if err := fake.SetPerm("org-a/hh-more", "alice-corp", "write"); err != nil {
		t.Fatal(err)
	}
	if out := alice.ok("store", "discover"); !strings.Contains(out, "https://github.com/org-a/hh-more.git") || strings.Contains(out, "hh-shared") || strings.Contains(out, "web") {
		t.Errorf("alice's discover:\n%s", out)
	}
	carol := filepath.Join(t.TempDir(), "carol")
	if err := os.MkdirAll(carol, 0o750); err != nil {
		t.Fatal(err)
	}
	c := &devUser{t: t, bin: bin, home: carol, env: append(slicesWithout(bob.env, "HEDGEHUG_HOME=", "HOME=", "USERPROFILE="), "HEDGEHUG_HOME="+carol, "HOME="+carol, "USERPROFILE="+carol)}
	c.ok("store", "discover", "--from", filepath.Join(dev, "remotes"), "--all")
	if out := c.ok("store", "list"); !strings.Contains(out, "hh-ops") || !strings.Contains(out, "hh-shared") || !strings.Contains(out, "hh-tasks") {
		t.Errorf("carol's stores:\n%s", out)
	}
}

// slicesWithout drops the entries of env that start with one of prefixes.
func slicesWithout(env []string, prefixes ...string) []string {
	var out []string
	for _, kv := range env {
		keep := true
		for _, p := range prefixes {
			if strings.HasPrefix(kv, p) {
				keep = false
			}
		}
		if keep {
			out = append(out, kv)
		}
	}
	return out
}
```

（carol は config.yaml がないので `url_rewrites` もなく、`--from` で見つけた bare はパスのまま登録される。spec どおり「一致する規則がなければパスのまま」。）

- [ ] **Step 6: E2E を通す**

Run: `go test -race ./cmd/hedgehug/ -run 'TestE2E' && go test -race ./tools/devenv/`
Expected: PASS

- [ ] **Step 7: コミット**

```bash
git add tools/devenv cmd/hedgehug/e2e_3a_test.go cmd/hedgehug/e2e_test.go
git commit -m "feat(devenv): fakegh・url_rewrites・org-b・alice-corp を用意し、3a の E2E を足す"
```

---

### Task 14: 全体の確認と spec との突き合わせ

**Files:**
- Modify: 直すものがあればそのファイル
- Modify: `docs/superpowers/specs/2026-11-01-phase3-stores-accounts-design.md`（実装で決めた細部のうち、利用者が承認したものだけ）

- [ ] **Step 1: 全体のチェック**

Run: `make check`
Expected: `tidy-check`・`lint`・`fmt-check`・`test` がすべて通る

lint（gosec を含む）が新しいコードで指摘したものは、この plan の `//nolint:gosec` のほかに足りないものがあれば、既存のコードと同じ書き方（`//nolint:gosec // G703: <理由>`。理由は、そのパスや引数がどこから来るか）で、指摘された行に足す。理由なしの `//nolint` は足さない。

- [ ] **Step 2: git を外したテスト**

CI の `test-without-git`（`.github/workflows/ci.yaml`）と同じ条件で流す: `CGO_ENABLED=0`、PATH には `go` と `gotestsum` だけ、`gotestsum --format testdox -- ./...`。`go-keyring`（cgo なし）を足したので、`CGO_ENABLED=0` で通ることもここで確かめる。

Run:

```bash
go install gotest.tools/gotestsum@v1.13.0
nogit="$(mktemp -d)"
ln -s "$(command -v go)" "$nogit/go"
ln -s "$(go env GOPATH)/bin/gotestsum" "$nogit/gotestsum"
if PATH="$nogit" command -v git; then echo "git is still on PATH" >&2; exit 1; fi
CGO_ENABLED=0 PATH="$nogit" gotestsum --format testdox -- ./...
```

Expected: PASS（git の実行ファイルがなくても通る。gh を呼ぶテストはない。fakegh は Go の HTTP サーバーなので git を要らない）

- [ ] **Step 3: spec との突き合わせ**

spec の 3a の節（「設定」「認証」「GitHub API」「fakegh」「git の通信」「Store の作成と発見」「開発環境」の devenv の 3a、「テスト」の 3a）を1行ずつ読み、対応する Task とテストを挙げた表を作る（作業メモ。コミットしない）。対応のない行、spec と違う挙動、この plan の「spec にない細部の判断」にない判断があれば、直さずに一覧にしてコントローラーに報告する（利用者に聞いてから直す）。

- [ ] **Step 4: 報告**

コントローラーへ、テスト・lint の結果、突き合わせの一覧（すべて対応済み、または利用者に聞くべき点）を報告する。spec の変更は利用者の承認の後に別のコミットで行う。
