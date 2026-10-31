# フェーズ 3a の残り（フェーズ4a の後）実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 3a の残り（旧 plan の Task 8 の修正と Task 9〜14）を、フェーズ4a の後のコード（`internal/ops`）と、見直しで決めた「共通の決まり」に合わせて作り、3a を終える。

**Architecture:** フェーズ3の処理は `internal/ops` の `Session` のメソッドに置き、`cmd/hedgehug` は端末の入出力と表示だけを持つ。`internal/sync`・`internal/gitrepo`・`internal/github`・`internal/auth`・`internal/fakegh`・`tools/devenv` は 4a で変わっていないので、旧 plan のコードがそのまま使える。

**Tech Stack:** Go、go-git v5、cobra、go.yaml.in/yaml/v4、`github.com/zalando/go-keyring`、net/http

**Spec:** `docs/superpowers/specs/2026-11-01-phase3-stores-accounts-design.md`（実装前に必ず全体を読む。特に「共通の決まり」）。見直しの経緯は `docs/superpowers/specs/2026-11-01-phase3-design-review.md`。食い違えば spec が正。spec と違う挙動になりそうなら、実装を止めてコントローラーに報告する。

**旧 plan:** `docs/superpowers/plans/2026-11-01-phase3a-auth-github-discover.md`。Task 1〜7 は完了済み。Task 8〜14 はこの plan で置き換える。旧 plan の Task 9〜13 のテストとコードは、この plan が「旧 Task N のコードを使う」と書いたところで、下の「書き換えの規則」を当てて使う。

## Global Constraints

- 旧 plan の Global Constraints はすべて有効（git の実行ファイルを使わない、トークンは通信するときにだけ取る、Save は知らないキーとコメントを残す、`url_rewrites` は通信の直前だけ、API のタイムアウトとヘッダー、5分、`--json`、終了コード、英語の文言、`go test -race ./...` と `golangci-lint run ./...`、Task ごとのコミット）。
- spec の「共通の決まり」に従う。とくに: 処理は `internal/ops`、host とアカウント ID は `config.SameHost`・`config.SameAccount` で比べる、警告は `Session.Warn` か結果の `Warnings`、エラーは `ops.Error` の Kind/Code、ops は端末に聞かない（`choice.required`・`confirm.required`）、config.yaml は `Config.Update` で書く。
- コミットメッセージの末尾に `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`（実装したモデルの名前）を付ける。
- Minor の指摘もその Task のうちに直す（CLAUDE.md の残タスク禁止）。

## 書き換えの規則（旧 plan のコードを使うとき）

| 旧 plan | この plan |
| --- | --- |
| `func (a *app) xxx(...)` で処理と表示が一緒 | 処理は `func (ss *Session) Xxx(in XxxInput) (XxxResult, error)`（`internal/ops`）、表示は `cmd/hedgehug` の `a.xxx` が結果を出す |
| `a.config()` | `ss.Config()` |
| `a.tokenFor`・`a.authOptions`・`a.connFor`・`a.githubFor`・`a.pickAccount` | `ss.Token`・`ss.AuthOptions`・`ss.Conn`・`ss.GitHub`・`ss.PickAccount`（Task 2 で作る） |
| `a.syncOptions`・`a.runSync` | `ss.SyncOptions`・`ss.RunSync` |
| `a.open()`（書き込み） | `ss.OpenForWrite()`（Task 5 で作る） |
| `usageError{err}`・`fmt.Errorf(...)` | `newErr(Kind..., "<code>", err, args...)`（spec の「エラーと警告」の表。表にないものは既存の Code の付け方に合わせる） |
| `fmt.Fprintf(a.stderr, "warning: ...")` | `ss.warn(...)` |
| `c.Save()` | `c.Update(func(c *config.Config) error { ... })`（Task 1） |
| `x.Host == host`、`a == b`（アカウント ID） | `config.SameHost`、`config.SameAccount` |
| テストの `te.run`・`te.ok` | ops の単体テスト（`internal/ops/helpers_test.go` のヘルパー）を主にし、CLI のテストは表示と終了コードだけを確かめる |

---

### Task 1: 共通の決まりの土台（config・ops のエラー・自分の判定）

**Files:**
- Modify: `internal/config/hosts.go`、`internal/config/config.go`、`internal/ops/errors.go`、`internal/query/eval.go`（`@me` の比較）、`internal/inbox`（`Catch` の比較）、`internal/ops/storeops.go`（`c.Save()` → `Update`）
- Test: `internal/config/hosts_test.go`、`internal/config/config_test.go`、`internal/ops/errors_test.go`、`internal/query` と `internal/inbox` の既存のテストに足す

**Interfaces:**
- Produces:
  - `func SameHost(a, b string) bool`（`strings.EqualFold`）、`func SameAccount(a, b string) bool`（`<host>/<login>` を `strings.EqualFold`）
  - `Config.Host`・`IsGitHubHost`・`loadHosts` の重複検査が `SameHost` を使う
  - 読み込みの検査: 同じ host と login（`SameHost` と login の `EqualFold`）のアカウントが2つあればエラー（ファイルと行）
  - 空の値: 省略できる値は前後の空白を除いて空ならないのと同じ。必須の値（spec の「空の値」の一覧）が空なら読み込みのエラー
  - 非スカラーの値（`default_store: [a]`、`url_rewrites: x` など）のエラー文を、ほかのキーのエラーと同じ形（`<file>:<line>: <key> must be ...`）にする（旧 Task 1 の Minor）
  - `func (c *Config) Update(fn func(*Config) error) error`: 設定ディレクトリの `config.lock` を `store.WithFileLock`（30秒）で取り、ファイルを読み直して `fn` を当て、Save する。`fn` がエラーなら書かない。`c` の中身を書いた後の値にする（`internal/store` は `internal/config` に依存しないので、`config` から `store.WithFileLock` を直接使える）
  - `internal/ops` の `c.Save()` をすべて `c.Update` にする（StoreInit・StoreAdd・StoreRemove）。`Update` の `fn` の中では、読み直した config に同じ変更を当てる（`storeExists` の検査も `fn` の中でやり直す）
  - `@me` と `created_by` の比較（`query`）、`inbox.Catch` の自分の判定が `config.SameAccount` と同じ比べ方（`strings.EqualFold`）をする。`query` と `inbox` は `config` を import しないので、`strings.EqualFold` を直接使い、コメントで spec の「大文字小文字」を指す
  - `ops/errors.go`: Code の定数は作らない（既存は文字列のまま）。`func choiceRequired(what string, candidates []string) *Error`（KindUsage、`choice.required`、Args: `what`・`candidates`。メッセージは `<what> is needed; pick one of: a, b` の形）、`func confirmRequired(what string) *Error`（KindUsage、`confirm.required`。`<what>: confirm with --yes`）

- [ ] **Step 1: 失敗するテストを書く**
  - `SameHost("GitHub.com", "github.com")` が true、`hosts: [{host: GHE.example.com}]` で `IsGitHubHost("ghe.example.com")` が true、`Host("ghe.example.com").API` が書いた `api`
  - `hosts` に `GHE.example.com` と `ghe.example.com` → 重複のエラー
  - `accounts` に `{host: github.com, login: Alice}` と `{host: GitHub.com, login: alice}` → 重複のエラー
  - `token: "  "`・`default_store: ""` → ないのと同じ。`hosts: [{host: " "}]`・`url_rewrites: [{from: "", to: x}]` → エラー
  - `default_store: [a]` のエラー文が `config.yaml:<行>: default_store must be a store name` の形
  - `Update`: 2つの goroutine が別々のアカウントを足しても両方残る。`fn` がエラーならファイルが変わらない。知らないキーとコメントが残る
  - `query`: `assignee:@me` が、config のアカウント `github.com/Alice` と担当者 `github.com/alice` で当たる
  - `inbox.Catch`: Me が `github.com/Alice`、通知の対象が `github.com/alice` で届く
  - `choiceRequired`・`confirmRequired` の Kind・Code・Args・メッセージ
- [ ] **Step 2: 失敗を確かめる**（`go test ./internal/config/ ./internal/ops/ ./internal/query/ ./internal/inbox/`）
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**（ops の Update への置き換えで既存のテストが通ること）
- [ ] **Step 5: コミット**（`feat(config): host とアカウント ID の比較・空の値・config.lock を共通の決まりにそろえる`）

---

### Task 2: 認証と通信を ops に移す

rebase で一時的に入れた `ops.Session.Conn`（フック）を消し、`cmd/hedgehug/conn.go` の処理を `internal/ops` に移す。

**Files:**
- Create: `internal/ops/conn.go`（`cmd/hedgehug/conn.go` の中身を移す）、`internal/ops/conn_test.go`（`cmd/hedgehug/conn_test.go` を移す）
- Delete: `cmd/hedgehug/conn.go`、`cmd/hedgehug/conn_test.go`
- Modify: `internal/ops/session.go`、`internal/ops/storeops.go`、`cmd/hedgehug/main.go`、`cmd/hedgehug/app.go`（`tokens` を消す）、`cmd/hedgehug/auth.go`

**Interfaces:**
- Produces:
  - `Session` のフィールド: `Keyring auth.Keyring`（nil は OS の keychain）、`HTTPClient *http.Client`（nil は30秒のタイムアウトのクライアント）、`Run func(name string, args ...string) (stdout, stderr []byte, err error)`（nil はコマンドを実行する）。`ForStore` が写す。`Conn` フィールドは消す
  - トークンの記憶: 1回の実行（CLI）・1つの要求（GUI）で同じアカウントのトークンを1度だけ取る。mutex 付きの `*tokenCache` を Session に持ち、`ForStore` はポインタを写して共有する（トークンはアカウントごとで、Store によらないため）
  - `func (ss *Session) AuthOptions(c *config.Config, acct config.Account) auth.Options`
  - `func (ss *Session) Token(c *config.Config, acct config.Account) (string, error)`
  - `func (ss *Session) Conn(c *config.Config, e config.StoreEntry) gitrepo.Conn`（トークンは書き換え後の URL の host が `config.SameHost` でアカウントの host と同じときだけ）
  - `func (ss *Session) GitHub(c *config.Config, acct config.Account, host string) (*github.Client, error)`（旧 Task 9 の `githubFor`。token のないアカウントは `auth.token` のエラー）
  - `func (ss *Session) PickAccount(c *config.Config, name string) (config.Account, bool, error)`（旧 Task 9 の `pickAccount`。ない名前は `KindNotFound`、`account.not_found`、Args: `name`・`candidates`）
  - `internal/github`: `Link` の `next` の比較を host からオリジン（スキーム・host・ポート）にする（spec の「秘密の送り先」）。テストを足す
  - `internal/auth`: gh の標準エラーを添えるとき、トークンの形（`gho_`・`ghp_`・`ghs_`・`ghu_`・`github_pat_` で始まる語）を `***` にする。gh・keychain・device・env が空のトークン（前後の空白を除いて空）を返したら、取り方ごとの案内を添えたエラーにする（spec の「トークンの取得」の表）。env の値の前後の空白を除く
  - `SyncOptions` と `StoreAdd` の clone は `ss.Conn` を使う

- [ ] **Step 1: 失敗するテストを書く**: 移した conn のテスト（ops で）、`Link` の `next` が同じ host の別ポートを指したらトークンを送らずエラー、gh の標準エラーのトークンが伏せられる、空のトークン4種のエラーと案内、env の前後の空白
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 移して実装する**。`cmd/hedgehug/main.go` は `ops.Session` に `Keyring`・`HTTPClient`・`Run` を env から入れる
- [ ] **Step 4: `go test -race ./...` と lint を通す**
- [ ] **Step 5: コミット**（`refactor(ops): トークンの取得・git の通信・GitHub のクライアントを Session に移す`）

---

### Task 3: `hh auth` と `hh store default` を ops に移し、旧 Task 8 のレビューを直す

**Files:**
- Create: `internal/ops/auth.go`、`internal/ops/auth_test.go`
- Modify: `cmd/hedgehug/auth.go`（入出力と表示だけにする）、`cmd/hedgehug/auth_test.go`、`internal/ops/storeops.go`（`StoreDefault`）、`cmd/hedgehug/store_default.go`、`internal/auth`（Delete・テスト）

**Interfaces:**
- Produces:
  - `type AuthLoginInput struct { Name, Host string; HostGiven bool; Token string /* --with-token で読んだもの。空なら Device Flow */; WithToken bool; ShowCode func(*github.DeviceCode) }`、`type AuthLoginResult struct { Account, ID string; Added bool }`、`func (ss *Session) AuthLogin(in AuthLoginInput) (AuthLoginResult, error)`
  - `type AuthStatus struct { Name, ID, Token string; Expires *time.Time; State string /* ok|error|none */; Login, Error string }`、`func (ss *Session) AuthStatus() ([]AuthStatus, error)`（エラーが1つでもあれば、結果と一緒に `errFindings` に当たるエラーを返す。CLI は終了コード1）
  - `func (ss *Session) AuthLogout(name string) (removed bool, err error)`
  - `func (ss *Session) StoreDefault(name string) (string, error)`（name が空なら今の値。今と同じ名前なら書かない。ない名前は `store.not_found` と候補）
- 直すこと（旧 Task 8 のレビューと見直し）:
  1. login の比較は大文字小文字を区別しない。同じ host・login の重複判定も `config.SameHost` と `EqualFold`。config.yaml の重複の読み込みエラー（Task 1）と合わせたテストを足す
  2. `--host` が config のアカウントの host と違えば `KindUsage`（終了コード2）。比較は `SameHost`
  3. 書く順は keychain → config.yaml（`Config.Update`）。config.yaml が書けなければ keychain を前の値に戻す（前の値がなければ消す）
  4. `device` のトークンの keychain への保存と `auth logout` の削除を `auth.lock` の中で行う。Device Flow のポーリングの間は取らない
  5. `auth status` の JSON を `state`（`ok`・`error`・`none`）にし、`ok` をやめる。token のないアカウントは `state: none` で、`token`・`expires`・`login`・`error` を省く。表の STATE は `ok`・エラーの文・`-`
  6. 期限なしの device トークン（`expires_at` がない）は EXPIRES が `-`、JSON は `expires` を省く
- 旧 Task 2 の Minor（テスト不足）をここで足す: `auth.Delete` の `ErrNotFound`、壊れた DeviceToken の JSON、未知の Kind、device 未ログインの案内、`wrapKeyring` のヒント、`Options.Keychain()` の nil

- [ ] **Step 1: 失敗するテストを書く**（ops の単体テスト。keychain は `keyring.MockInit()`、Device Flow は fakegh）: 上の 1〜6 と Minor のテスト、`StoreDefault` が同じ値で config.yaml を書かない（更新時刻か内容で確かめる）
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**。CLI の `auth login` は、`--with-token` なら標準入力の1行を読んで `Token` に入れ、Device Flow なら `ShowCode` に今の `showDeviceCode`（端末ならブラウザを開くかを聞く）を渡す
- [ ] **Step 4: `go test -race ./...` と lint を通す**。既存の `cmd/hedgehug/auth_test.go` は表示と終了コードのテストとして残し、JSON の期待値を `state` に直す
- [ ] **Step 5: コミット**（`fix(ops): hh auth を ops に移し、login の比較・巻き戻し・auth.lock・status の JSON を直す`）

---

### Task 4: `store init` の remote の用意（旧 Task 9）

**Files:**
- Modify: `internal/ops/storeops.go`（`StoreInitInput` に `Account`・`Path`、`prepareRemotes`・`madeRemotes`）、`cmd/hedgehug/store_init.go`（`--account`・`--path`）
- Test: `internal/ops/storeops_test.go`（旧 Task 9 のテストを ops に移したもの）、`cmd/hedgehug/store_init_test.go`（フラグと表示）

**Interfaces:**
- 旧 Task 9 のコード（`storeInit`・`madeRemotes`・`prepareRemotes`）を、書き換えの規則で ops に移す。挙動は旧 Task 9 と spec の「`hh store init`」のとおり。
- 変わるところ:
  - アカウントは `ss.PickAccount(c, in.Account)`。GitHub のホストの remote があって決まらなければ `choiceRequired("--account", <アカウント名>)`（spec の「端末と非端末」の選択。CLI は端末なら番号で選ばせて入れ直す）
  - 「GitHub のホスト」は `context.Split` の host を `c.IsGitHubHost`（`SameHost`）で見る
  - エラーの Code: リポジトリが空でない `store.remote_not_empty`（既存）、作成の 403 `github.forbidden`（付録の案内を添える）、401 `auth.unauthorized`
  - config.yaml は `Update` で書く。`Update` が失敗したら、作った Store のディレクトリとローカルの bare を消し、GitHub のリポジトリは残して案内する
  - 結果の警告は `ss.warn`

- [ ] **Step 1: 失敗するテストを書く**（旧 Task 9 のテスト7本を ops に移す。加えて: アカウントが2つで GitHub の remote → `choice.required` と候補、`hosts` に大文字で書いた host の remote が GitHub と判定される）
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と lint を通す**（`store init` がアカウント1つのときも `stores[].account` を書くので、config.yaml を完全一致で比べている既存のテストがあれば期待値に `account:` の行を足す）
- [ ] **Step 5: コミット**（`feat(ops): store init がローカルの bare と GitHub のリポジトリを用意し、--account と --path を受け取る`）

---

### Task 5: 読み取り専用とメンバーの状態、同期での GitHub の情報（旧 Task 10）

**Files:** 旧 Task 10 と同じ（`internal/sync/access.go`・`members.go`・`github.go`・`sync.go` とテスト）。加えて `internal/ops/session.go`（`Unpushed` が `sync.Unpushed` を呼ぶ）。

**Interfaces:** 旧 Task 10 のとおり。`internal/sync` は 4a で変わっていないので、旧 Task 10 のテストとコードをそのまま使う。次だけ変える。
- GitHub の情報を取るのに失敗しても `checked` を進める（次は5分後）。判定は変えない。テストを足す（失敗の直後の同期で API を呼ばない）
- `access.json`・`members.json` は同期の中（`ss.RunSync` が取る `sync.lock`）で書く。`CheckAccess` を同期の外から呼ぶ側（Task 6 の `store add`）は `s.WithLocalLock("sync.lock", ...)` の中で呼ぶ。`CheckAccess` の doc にそう書く
- `ops.Unpushed` の本体を `return sync.Unpushed(s.Repo())` にする（数える処理は `sync.Unpushed` の1つ）
- 「GitHub のホスト」の判定は Task 1 の `IsGitHubHost`（呼び出し側の ops が `GitHubRepo` を作るときに使う。Task 6）

- [ ] Step 1〜7 は旧 Task 10 のとおり（コミットは `feat(sync): 読み取り専用とメンバーの状態を持ち、同期で GitHub の権限とコラボレーターを取る`）

---

### Task 6: 読み取り専用の拒否、`store add --account`、同期の表示（旧 Task 11）

**Files:**
- Create: `internal/ops/readonly.go`、`internal/ops/readonly_test.go`
- Modify: `internal/ops/session.go`（`OpenForWrite`、`UpdateCommit` の検査、`SyncOptions` の `GitHub`、`githubRepoOf`）、書き込みの操作のファイル（`task.go`・`edit.go`・`comment.go`・`move.go`・`delete.go`・`workspace.go`・`undo.go`・`inbox.go` の revert・`maintain.go`）、`internal/ops/storeops.go`（`StoreAdd` の `account` と判定）、`internal/ops/sync.go`（`Explicit`、`Unpushable`、`StoreStatus.ReadOnly`）、`cmd/hedgehug/store_add.go`、`cmd/hedgehug/sync.go`
- Test: `internal/ops/readonly_test.go`、`cmd/hedgehug/readonly_test.go`（表示と終了コード）

**Interfaces:**
- Produces:
  - `type ReadOnlyError`（旧 Task 11 の `readOnlyError`。`Error()` は `<store> is read-only (<why>)`）を `newErr(KindConflict, "store.read_only", ..., "store", name, "reason", acc.Reason)` で包む
  - `func (ss *Session) checkWritable(s *store.Store) error`
  - `func (ss *Session) OpenForWrite() (*store.Store, *index.Index, error)`: `Open` の後に `checkWritable`。書き込みの操作はすべて `Open` の代わりにこれを使う（ID の解決や書き込み前の検査より先に拒否する）
  - `UpdateCommit` の最初と `fixStore` の最初でも `checkWritable`（入口を通らない書き込みの取りこぼしを防ぐ）
  - `func (ss *Session) githubRepoOf(c *config.Config, e config.StoreEntry) *sync.GitHubRepo`（origin が GitHub のホストでなければ nil。判定は書き換え前の URL）
  - `SyncOptions` の `GitHub` にそれを入れる
  - `func (ss *Session) Sync(all, explicit bool)`（または入力の型）。CLI の `hh sync` と GUI の「今すぐ同期」は `explicit` を true、バックグラウンドの同期は false。`explicit` なら `sync.Options.Explicit`
  - `SyncReport.Unpushable int`、`StoreStatus.ReadOnly string`（理由の文。JSON `read_only,omitempty`）
  - `func (ss *Session) StoreAdd(name string, remotes []string, path, account string)`: `account` を確かめ（`PickAccount` と同じエラー）、clone は `ss.Conn`、config.yaml は `Update` で `AddStore` と `SetStoreAccount`。登録の後に `sync.lock` の中で `CheckAccess`（失敗は警告）
- CLI: 旧 Task 11 の表示（`read-only: N commits not pushed`、`sync --status` の `read-only:` の行）を `cmd/hedgehug/sync.go` に置く

- [ ] **Step 1: 失敗するテストを書く**（旧 Task 11 の `readonly_test.go` を ops に移す。加えて: 読み取り専用の Store で存在しない ID を `set` → `store.read_only`（`not found` ではない）、未削除のタスクの `purge` → `store.read_only`、`doctor --fix` → `store.read_only`、`Sync(explicit=false)` は `push_denied` を外さない、`store add` の判定が `sync.lock` を取っている（別の goroutine が `sync.lock` を持っている間は待つ））
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と lint を通す**
- [ ] **Step 5: コミット**（`feat(ops): 読み取り専用の Store への書き込みを開いた時点で拒否し、store add --account と同期の表示を足す`）

---

### Task 7: `hh store discover`（旧 Task 12）

**Files:**
- Create: `internal/ops/discover.go`、`internal/ops/discover_test.go`、`cmd/hedgehug/store_discover.go`、`cmd/hedgehug/store_discover_test.go`
- Modify: `cmd/hedgehug/store_init.go`（`storeCmd` に `storeDiscoverCmd`）

**Interfaces:**
- Produces:
  - `type Candidate struct { Name, URL, Account, Permission string }`（JSON `name`・`url`・`account`・`permission`）
  - `type DiscoverInput struct { Account, From string }`、`func (ss *Session) StoreDiscover(in DiscoverInput) ([]Candidate, error)`: 旧 Task 12 の候補の集め方・マウント済みの除外・名前の提案。`--from` でアカウントが決まらない（`Account` が空でアカウントが2つ以上）ときは `choiceRequired("--account", 名前)`
  - `type MountResult struct { Mounted []string; Failed []MountFailure }`、`type MountFailure struct { Name string; Err error }`、`func (ss *Session) StoreMount(cands []Candidate) (MountResult, error)`: 1つずつ `StoreAdd`。失敗したものは `StoreAdd` が巻き戻す。残りを続け、1つでも失敗したら結果と一緒にエラー（CLI は終了コード1）
- CLI: 旧 Task 12 の表示・端末の質問（番号 / all / 空でやめる）・端末でないときは一覧だけ・`--all`・`--json` の形（`{"candidates": [...], "mounted": [...]}`。失敗があれば `"failed": [{"name", "error"}]` を足す）。`choice.required` を受けたら、端末なら番号でアカウントを選ばせて入れ直す

- [ ] **Step 1: 失敗するテストを書く**（旧 Task 12 のテストを ops と CLI に分けて移す。加えて: `--from` でアカウント2つ・`--account` なし → `choice.required`、2つ選んで1つ目の clone が失敗 → 2つ目はマウントされ、1つ目のディレクトリは残らず、終了コード1）
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と lint を通す**
- [ ] **Step 5: コミット**（`feat(ops): store discover で GitHub とローカルの Store を見つけてマウントする`）

---

### Task 8: devenv の 3a 分と 3a の E2E（旧 Task 13）

**Files・Interfaces:** 旧 Task 13 のとおり（`tools/devenv` は 4a で変わっていない）。変わるところ:
- `TestE2EAuthLoginAndStatus` は、このプロセスの中で `run` を呼ぶ（旧 Task 13 のとおり）。`run` が作る `ops.Session` に `Keyring` が入るので、`keyring.MockInit()` のモックがそのまま効く
- `auth status --json` の期待値は `state`（Task 3）

- [ ] Step 1〜7 は旧 Task 13 のとおり（コミットは `feat(devenv): 3a の構成（fakegh・url_rewrites・org-b・alice-corp）と E2E を足す`）

---

### Task 9: 全体の確認と spec との突き合わせ（旧 Task 14）

- [ ] **Step 1: 全体のチェック**: `go test -race ./...`、`golangci-lint run ./...`（gosec の指摘は旧 Task 14 の書き方で）
- [ ] **Step 2: git を外したテスト**: 旧 Task 14 の Step 2 のとおり（`CGO_ENABLED=0`、PATH は `go` と `gotestsum` だけ）
- [ ] **Step 3: grep での確認**:
  - remote URL を読むのが `context.Split` だけか（`url.Parse` や `strings.Split` で remote を読んでいる箇所がないか）
  - host の比較が `config.SameHost` を通っているか（`\.Host ==`、`== .*\.Host`）
  - トークンを出力・エラーに入れていないか（`Token`・`tok` を `fmt` に渡している箇所）
  - `cmd/hedgehug` に処理が残っていないか（`config.Load`、`store.Open`、`github.New` を直接呼んでいないか）
- [ ] **Step 4: spec との突き合わせ**: spec の 3a の節と「共通の決まり」を1行ずつ読み、対応する Task とテストを挙げた表を作る（作業メモ。コミットしない）。対応のない行、spec と違う挙動があれば、直さずに一覧にしてコントローラーに報告する
- [ ] **Step 5: 報告**: テスト・lint の結果と突き合わせの一覧をコントローラーへ
