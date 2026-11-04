# フェーズ 4f-2: アカウント・Store の設定と Onboarding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** アカウントの追加（Device Flow・トークンの貼り付け・gh・環境変数・トークンなし）とログアウト、Store の発見とマウント・URL で追加・解除・既定・色・表示を GUI で動かし、Store が0件のときの Onboarding を3段（アカウント・Store を選ぶ・はじめる）にする。

**Architecture:** フェーズ 4f の3回のうちの2回目。決定事項と spec にない細部は概要 `docs/superpowers/plans/2026-11-04-phase4f-screens.md` にあり（この回は細部 17〜33、4f-1 で決めた API の決まり 52〜70、この回で足した 71〜）、各 Task はそこを前提にする（細部の番号はそこを指す）。API は 4f-1 で入っているので、画面は web だけで作る。Go に足すのは E2E のための keyring（Task 1。2026-11-04 に利用者が決定）だけで、API の形は変えない（足りないものが見つかったら止めて報告する）。画面ごとに 1 Task で、各 Task がルート・画面・入口・単体テスト・E2E をまとめて入れる。

**Tech Stack:** TypeScript、Svelte 5（runes）、Vite、Vitest と @testing-library/svelte、Playwright（`web/e2e`、`harness.ts`）、Biome、svelte-check。Task 1 は Go（go-keyring の `auth.Keyring`、ビルドタグ）と Makefile。依存は足さない。

**Spec:** `docs/superpowers/specs/2026-11-01-phase4-gui-design.md`（「4f の細部」）、`docs/superpowers/specs/2026-11-01-phase3-stores-accounts-design.md`、概要 `docs/superpowers/plans/2026-11-04-phase4f-screens.md`。画面はキャンバス（https://claude.ai/artifact/K14rwULxyyS2nYNbMKSr5L）の SettingsAccounts・SettingsStores・Onboarding・SettingsNav を正とし、概要 plan の「作らないもの」と細部で直したところだけ変える。

**この回の Task:** Task 1（E2E の keyring と fakegh）、Task 2（core: `stores.changed`・閲覧のみの判定・`AccountsCache`・CLI の行）、Task 3（設定のナビとルート）、Task 4（アカウントと認証）、Task 5（マウント中の Store）、Task 6（発見・Onboarding・CreateStore のアカウント）、Task 7（仕上げ）。番号の順に実行する（Task 4 の追加の部品を Task 6 の Onboarding が、Task 5 の発見の入口が Task 6 の発見の部品を使う。Task 5 の「Store を探す」は Task 6 で足す）。

**この plan の形:** 4e-4 と同じく、web のコードは書かない（2026-11-03 に利用者が決定）。Task ごとにファイル・使うもの・作るもの・ふるまい・テストの観点を書き、実装者がテストから書く（TDD）。既存の部品と書き方（4e の画面）に合わせること。

**着手の前に:** main を取り込み、各 Task の「使うもの」の関数・型・部品がまだその場所と名前であることを確かめる（構成見直しの回 3 以降が先に入っていれば、移った先に合わせる。API の形と画面の言葉は変えない）。

## Global Constraints

4d・4e・4f の概要 plan の Global Constraints をすべて引き継ぐ。

- 画面の文言はすべて日本語。コードの識別子・コメントは英語（既存のコードと同じ）
- コンポーネントは色・寸法の 16 進数や px を直接書かず、`web/app/styles/tokens.css` の CSS 変数を使う。足りないトークンは `tokens.css` に Foundations の値で足す
- ダイアログと操作の足元には同じ操作の `hh` コマンドを出し、コピーできるようにする（`ui/CliLine.svelte`、文字列は `web/core/cli.ts` に関数を足して作る）。GUI が出す `hh` の行は実在するコマンドとフラグだけ（下の「CLI の表記」）。トークンは行に入れない
- エラーはサーバーの problem+json を `errorMessage`（`web/core/messages.ts`）で出す。分岐は `type`（Code）で行い、status では分岐しない（細部 65）。足りない Code の文言は `messages.ts` に足し、`messages.test.ts` の網羅の検査を通す
- トークンはどの画面の状態にも残さない（貼り付けの欄は送ったら空にする。ログ・トースト・CLI の行に出さない）
- 書き込みのあとは `GET /api/stores`（`StoreDirectory.reload()`）と、アカウントの操作なら `GET /api/accounts`（`AccountsCache.reload()`）を取り直す（細部 22・68・70）。既定の印は `default_store` を使い、`default` は使わない（細部 64）
- 取り消しにくい操作（ログアウト、マウントの解除）だけ確認ダイアログ（`ui/Dialog.svelte`）
- 4f-3 に回したもの（閲覧のみの編集の無効 34〜37、非表示の Store を一覧から外す 38〜40、マイタスク 41〜42、Store 名付きの ID 43、コメントの本人確認 44 の画面側、StoreMove 45〜50）は作らない。この回の「色・表示が GUI で動く」は、`PATCH` で保存され、SettingsStores のカードがそれを映し、`hh store list --json` に出ること（サイドバーの色と非表示は 4f-3 の細部 38〜40）
- まだない画面への入口は出さない。概要 plan の「作らないもの」は出さず、出さないことをテストで確かめる
- `svelte-check` は警告ゼロ、`biome ci` は指摘ゼロ。各 Task の終わりに `cd web && pnpm fix && pnpm check && pnpm exec biome ci . && pnpm exec vitest run`（全件）。E2E は `make e2e`（その Task の spec だけを先に `cd web && pnpm exec playwright test e2e/<file>` で流してよい。そのときは `make e2e-bin` で作ったバイナリを使う）
- Go を変える Task は `go vet ./...`・`gofmt -l .`・`golangci-lint run ./...`・`go test -race ./...` も流す。Go の新しいテストには `t.Parallel()`
- spec・設計と違う挙動にしたくなったら、実装を止めて報告する（自分で決めない）
- テストは前面で実行し、Bash の timeout は 600000。落ちたテストは失敗の出力と実行条件を残す
- コミットのメッセージは日本語で、末尾は `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`

## CLI の表記（実際のコマンド）

| 操作 | コマンド |
|---|---|
| Device Flow | `hh auth login [<名前>] [--host <ホスト>]`（`--host` は github.com 以外のときだけ） |
| トークンを貼る | `hh auth login <名前> --with-token [--host <ホスト>]`（トークンは標準入力。行に入れない） |
| gh・環境変数・トークンなし | `hh auth add <名前> --login <login> [--host <ホスト>] [--token gh\|env:<変数>]` |
| ログアウト | `hh auth logout <名前>` |
| アカウントの一覧 | `hh auth status` |
| 発見 | `hh store discover [--account <名前>] [--from <フォルダ>]` |
| URL で追加 | `hh store add <名前> --remote <URL>... [--account <名前>]`（`--account` は選んだときだけ） |
| 解除 | `hh store remove <名前> [--keep]` |
| 既定 | `hh store default <名前>` |
| 色と表示 | `hh store set <名前> [--color <色>] [--hidden=true\|false]`（`--color ""` は自動に戻す） |
| Store を作る | `hh store init <名前> --prefix <接頭辞> [--workspace] [--remote]... [--account <名前>]` |

## この回で足した細部（71〜）

概要 plan の細部 17〜33 に足すもの。Task 7 で概要 plan の「4f-2 の plan で足した細部」として写す。

71. **E2E の keyring**（2026-11-04 に利用者が決定）: Device Flow の完了・トークンの貼り付け・ログアウトは OS の keychain に書くが、CI の E2E（ubuntu）には Secret Service がなく、手元の macOS では本物の Keychain に書いてしまう。Go に `auth.FileKeyring(path)`（JSON のファイルに持つ `auth.Keyring`）を足し、ビルドタグ `e2e` のときだけ CLI が `$HEDGEHUG_HOME/e2e-keyring.json` のそれを使う。`make e2e` はタグ付きで `bin/hedgehug-e2e` を作り、harness はそれを使う。リリースのバイナリ（タグなし）は今のまま OS の keychain を使う
72. **E2E の GitHub**: harness は Device Flow とトークンの貼り付けを使う spec だけ `cmd/fakegh` を起動し（`fakegh serve --addr 127.0.0.1:0 --seed <json>`。`device_interval: 0`、利用者 `e2e`）、config.yaml に `hosts: [{host: github.com, api: <fakegh>, web: <fakegh>, client_id: e2e}]` を書く。承認は `fakegh device approve <コード> --user e2e --server <fakegh>`、貼るトークンは `fakegh token add <トークン> --user e2e`
73. **閲覧のみの判定を前倒し**: 細部 34 の `StoreDirectory.readOnly(store)` と理由の日本語を、設定の画面（細部 17・23）が使うのでこの回で作る。`read_only` の値は usecase の `Access.Why()` の英文（`the GitHub permission is read`・`the remote cannot be written`・`the remote denied the push`）なので、その3つを「GitHub の権限が読み取りです」「remote に書き込めません」「remote が push を拒みました」に直し、ほかの文はそのまま出す（`web/core/stores.ts` の `readOnlyReason`）。4f-3 はこれを使う
74. **`AccountsCache` の取り直し**: `GET /api/accounts` は `GET /user` を呼ぶので、自分からは読み込まない。アカウントと認証・Onboarding・CreateStore・URL で追加の画面が開いたときに `load()`（読み込み済みなら何もしない）、操作のあとに `reload()`。`stores.changed` では、読み込み済みのときだけ取り直す（細部 44 の「`stores.changed` と操作のあと」を、読み込んでいないときは呼ばない形にした）
75. **`stores.changed` で捨てる同期の状態**: `Connection` に `retainSync(names)` を足し、`StoreDirectory` が Store の一覧を取り直したあとに今の Store の名前で呼ぶ（細部 33 の「消えた Store の `conn.sync` の項目を捨てる」）。変わったときだけ購読者に知らせる
76. **設定のルート**: `/settings/accounts`（`{ page: "accounts" }`）と `/settings/stores`（`{ page: "stores" }`）。Store の名前を持たない（このマシンの設定）。キャンバスの順に「アカウントと認証」「マウント中の Store」「同期と保守」。ナビのバッジ（キャンバスの数）は出さない（何の数かを決める元がない）
77. **パレット**: 「設定のアカウントと認証を開く」「設定のマウント中の Store を開く」を足す（細部 105 と同じく、画面を作った Task で足す）
78. **Onboarding の閉じ方**: Onboarding は Store が0件になったときに出て、「始める」を押すか、CreateStore で作り終えるまで出し続ける（マウントで Store が1件以上になっても、はじめるの段を見せるため閉じない）。`App.svelte` が「Onboarding の途中」を持つ。最後の Store を解除して0件になれば、また出る（細部 27）
79. **Onboarding のアカウントの段**: アカウントが0件なら「次へ」を無効にし、理由（「アカウントを1つ追加してください」）を出す（Store を書くにも作るにもアカウントが要る）。トークンなしのアカウントでも次へ進める（細部 29）。アカウントの追加は細部 19 の部品をそのまま使う
80. **発見の段のアカウントなし**: トークンを持つアカウントがないときは自動では探さず、「GitHub で探すにはトークンのあるアカウントが要ります」と「フォルダから探す」だけを出す（usecase は `account.none` で断るので、先に呼ばない）
81. **発見の候補の名前**: 候補の `name`（usecase が提案した名前）を呼び名の欄の初期値にする。マウントの本文は選んだ候補の `{name, url, account, permission}` を、呼び名を欄の値に変えて送る。空の呼び名の候補は送らず、その行に「名前を入れてください」を出す
82. **Device Flow の引き継ぎ**: 開始が 409 `auth.device_running` なら、`args` にコードがあればそのフローを引き継いで同じ画面を出し（「別の画面で始めたログインを続けます」）、`id` だけなら「やめてやり直す」（`DELETE` してから開始し直す）を出す（細部 69）。期限（`expires_at`）を過ぎたら「コードの期限が切れました」と「やり直す」を出す。`auth.device` の `error` は `errorMessage({type: code, detail: message})` で出す
83. **`gh`・環境変数のカード**: 取得元が `gh`・`env:VAR` のアカウントの「…」には「Hedgehug が持っているものはありません」だけを出す（細部 20）。カードの状態が `error` のとき、その文（`AuthStatus.error`）は英文のまま出す（GitHub やコマンドの文で、Code がない）
84. **URL で追加の名前**: 名前は必須で、URL の最後の部分（`.git` を除く）を、名前が空のあいだだけ提案する。URL は1行に1つ（空行は捨てる）
85. **CreateStore のアカウント**: アカウントの一覧は `AccountsCache` から取る（細部 32）。読み込み中・読めないときは選択欄を出さず、`account` を送らない

---

### Task 1: E2E の keyring と fakegh（細部 71・72）

**Files:**
- Create: `internal/app/remote/auth/filekeyring.go`、`internal/app/remote/auth/filekeyring_test.go`
- Create: `internal/app/cli/keyring.go`（`//go:build !e2e`。`mainKeyring(getenv) auth.Keyring` は nil を返す＝OS の keychain）、`internal/app/cli/keyring_e2e.go`（`//go:build e2e`。`auth.FileKeyring(filepath.Join(<HEDGEHUG_HOME>, "e2e-keyring.json"))`。`HEDGEHUG_HOME` が空なら nil ではなく起動の誤りにする）
- Modify: `internal/app/cli/run.go`（`Main` の `env` に `keyring: mainKeyring(os.Getenv)`）
- Modify: `Makefile`（`e2e-bin`: `go build -tags e2e -o bin/hedgehug-e2e ./cmd/hedgehug` と `go build -o bin/fakegh ./cmd/fakegh`。`e2e` は `web` と `e2e-bin` に依存する。`.PHONY` に足す）
- Modify: `.github/workflows/ci-backend.yaml`（vet か lint の段に `go vet -tags e2e ./internal/app/cli` を足す。タグ付きのファイルが壊れていても今の検査では気づかないため。置く段は今の構成を読んで決める）
- Modify: `web/e2e/harness.ts`（`BIN` の既定を `bin/hedgehug-e2e`、`FAKEGH` を `bin/fakegh`（`HEDGEHUG_FAKEGH` で上書き）。`binDir` は今のまま（名前が `hedgehug` でなければ symlink を作る）。フィクスチャ `github`（`test.use({ github: true })` の spec だけ fakegh を起動し、`hh` より先に用意して config.yaml に `hosts` を書く。止めるのは `hh` と一緒）。`Hedgehug` に `fakegh(...args)`（`--server` を付けて走らせる）
- Create: `web/e2e/keyring.spec.ts`（この Task の煙の検査）

**使うもの:** `auth.Keyring`（`Get`・`Set`・`Delete`、`ErrNotFound`）、`internal/lib/osfs` の読み書き（forbidigo のため `os` を直接呼ばない。既存の auth のファイルの書き方に合わせる）、`fakegh serve`（`fakegh listening on http://<addr>` を標準出力に出す）、`fakegh.State` の JSON（`device_interval`・利用者。`internal/lib/fakegh/state.go` を読んで項目名を合わせる）

**作るもの:**
- `auth.FileKeyring(path string) Keyring`: `{"<service>\x00<user>": "<secret>"}` の JSON を、書くたびに読み直して書く（ほかの hh のプロセスと共有するため。書き込みは一時ファイルから rename）。ファイルの権限は 0600。ないキーの `Get`・`Delete` は `ErrNotFound`。ファイルがなければ空として扱う
- harness の `github` フィクスチャ: fakegh を `--addr 127.0.0.1:0` で起動し、出力の URL を読む（15 秒で諦める）。シードは `{"users": ["e2e"], "device_interval": 0}`（`internal/lib/fakegh/state.go` の `State`。`auto_approve` は使わない: 承認を待つ画面を E2E で見るため）

**ふるまい:**
- タグなしのビルドは今と同じ（`keyring` が nil で OS の keychain）
- `bin/hedgehug-e2e` の `hh auth login e2e --with-token`（標準入力にトークン）は `e2e-keyring.json` に書き、`hh auth status` が `keychain` の有効なアカウントとして出す

**テストの観点:**
- Go: `FileKeyring` の Set → Get、上書き、Delete、ない キーの `ErrNotFound`、ファイルがないときの Get、別の `FileKeyring` 値（別のプロセスの代わり）で書いたものが読めること、権限 0600
- E2E（`keyring.spec.ts`、`github: true`）: `hh.fakegh("token", "add", "t1", "--user", "e2e")` → `hh auth login e2e --with-token`（標準入力に t1）→ `hh auth status --json` が e2e を `ok` で出し、OS の keychain に触っていない（`e2e-keyring.json` がある）こと。harness に標準入力を渡す `hhIn(input, ...args)` を足す

- [x] Step 1: `filekeyring_test.go` を書いて落ちることを確かめる → `filekeyring.go` を書いて通す
- [x] Step 2: `keyring.go`・`keyring_e2e.go`・`run.go` を書き、`go build ./cmd/hedgehug` と `go build -tags e2e ./cmd/hedgehug` が通ることを確かめる
- [x] Step 3: Makefile と CI の段を足し、`make e2e-bin` で2つのバイナリができることを確かめる
- [x] Step 4: harness の `github` フィクスチャ・`fakegh`・`hhIn` と `keyring.spec.ts` を書いて通す。既存の E2E が `bin/hedgehug-e2e` で通ることを `make e2e` で確かめる
- [x] Step 5: 検査（Global Constraints の Go と web）を流してコミット `test(e2e): E2E のバイナリにファイルの keyring を入れ、fakegh を起動できるようにする`

---

### Task 2: core（細部 33・44 の core・73〜75）

**Files:**
- Modify: `web/core/connection.ts`・`connection.test.ts`（`stores.changed` の受け取り、`retainSync`）
- Modify: `web/core/stores.ts`・`stores.test.ts`（`stores.changed` で取り直す、取り直したあとの `retainSync`、`readOnly(store)`、`readOnlyReason(why)`）
- Create: `web/core/accounts.ts`・`accounts.test.ts`（`AccountsCache`）
- Modify: `web/app/core-factory.ts`・`core-factory.test.ts`、`web/app/context.ts`（`accounts` を core に足す）、`web/core/testing.ts` と各 testkit（core を作る部品が `accounts` を持つように。`pnpm check` の指摘に従う）
- Modify: `web/core/cli.ts`・`cli.test.ts`（下の「CLI の表記」の関数）
- Modify: `web/core/messages.ts`・`messages.test.ts`（足りない Code があれば）

**使うもの:** `Connection.on`・`emit`・`_sync`、SSE の `stores.changed`（本文は空。細部 54 のとおり1回の書き込みで2回届くことがある）、`StoreDirectory.queueReload`、`GET /api/accounts`（`AuthStatus[]`）、`ApiError`・`unwrap`・`callAll`

**作るもの:**
- `Connection`: `Handlers` に `"stores.changed": () => void` と `on` の型。`retainSync(names: string[])`（names にない Store の項目を `_sync` から捨て、変わったら購読者に知らせる）
- `StoreDirectory`: `stores.changed` で `queueReload()`。`reload()` の成功のあとに `conn.retainSync(今の Store の名前)`。`readOnly(store): string | null`（`StoreItem.read_only` か `conn.sync.get(store)?.read_only` の英文。どちらもなければ null）。`tone` は今の判定に `StoreItem.read_only` も足す（色の `color` を使うのは 4f-3 の細部 40）
- `readOnlyReason(why: string): string`（細部 73 の3つの英文を日本語に、ほかはそのまま）
- `AccountsCache`: `accounts(): AuthStatus[]`、`loaded()`、`error(): ApiError | null`、`load()`（読み込み済みか読み込み中なら何もしない）、`reload()`（新しいものが走れば古い結果は捨てる。`StoreDirectory.reload` の `seq` と同じ形）、`subscribe`。`stores.changed` と `resync` で、読み込み済みのときだけ `reload()`。取得元を `source(a): "gh" | "env" | "keychain" | "device" | "none"` と変数名で返す関数（`AuthStatus.token` の `gh`・`env:VAR`・`keychain`・`device`・空）
- `cli.ts`: `cliAuthLogin({name?, host?, withToken?})`、`cliAuthAdd({name, login, host?, token?})`、`cliAuthLogout(name)`、`cliStoreDiscover({account?, from?})`、`cliStoreAdd({name, remotes, account?})`、`cliStoreRemove(name, keep)`、`cliStoreDefault(name)`、`cliStoreSet({name, color?, hidden?})`（`color: ""` は `--color ""`）、`cliStoreInit` に `account?`。ホストは github.com のとき省く。`Placeholder` を受ける欄は既存の関数と同じ

**ふるまい:** `stores.changed` が1回の turn に2回届いても `GET /api/stores` は1回（`queueReload`）。`AccountsCache` が読み込み前なら `stores.changed` で `GET /api/accounts` を呼ばない

**テストの観点:** `stores.changed` で `GET /api/stores` が呼ばれる・2回で1回・消えた Store の `sync` が捨てられ購読者に届く・残る Store の `sync` は同じオブジェクト・`readOnly` の2つの元と null・`readOnlyReason` の3つと未知の文・`tone` が `StoreItem.read_only` で灰色・`AccountsCache` の `load` の1回だけ・`reload` の古い結果の破棄・読めないときの `error`・`stores.changed` と `resync` で読み込み済みのときだけ取り直す・`source` の5つ・`cli.ts` の各関数（ホストの省略、`--with-token` にトークンが入らないこと、`--color ""`、`--keep`、`--account`）

- [ ] Step 1: `connection.test.ts` → `connection.ts`
- [ ] Step 2: `stores.test.ts` → `stores.ts`
- [ ] Step 3: `accounts.test.ts` → `accounts.ts`、core への組み込み
- [ ] Step 4: `cli.test.ts` → `cli.ts`
- [ ] Step 5: 検査を流してコミット `feat(web): stores.changed の受け取り、閲覧のみの判定、アカウントのキャッシュと CLI の行`

---

### Task 3: 設定のナビとルート（細部 17・76・77）

**Files:**
- Modify: `web/app/router.ts`・`router.test.ts`（`SettingsPage` に `{ page: "accounts" }`・`{ page: "stores" }`、`/settings/accounts`・`/settings/stores`）
- Modify: `web/app/settings/SettingsNav.svelte`・`settings.test.ts`
- Modify: `web/app/settings/SettingsRoute.svelte`（振り分け。中身は Task 4・5 で入れる。この Task では Task 4・5 のページの空の骨組み（見出しだけ）を置く）
- Create: `web/app/settings/SettingsAccounts.svelte`・`web/app/settings/SettingsStores.svelte`（見出しと説明だけ。Task 4・5 で中身）
- Modify: `web/app/palette/search.ts`・`search.test.ts`・`CommandPalette.test.ts`（細部 77）
- Modify: `web/e2e/routes.spec.ts`（2つのルート）

**使うもの:** `hrefOf`・`onLinkClick`・`router`、`StoreDirectory.stores()`・`readOnly()`、`icons/Icon.svelte`（足りないアイコンは `icons` に足す。キャンバスの SettingsNav の人と円柱の形）

**ふるまい:**
- 「このマシン」の段: 「アカウントと認証」「マウント中の Store」「同期と保守」の順。今のページなら強調と `aria-current`
- 「Store ごとの設定」の選択欄: 閲覧のみの Store は「<名前>（閲覧のみ）」。非表示の Store も並べる（今と同じく全部）。Store が1つなら選択欄を出さない（今のまま）
- このマシンのページ（accounts・stores・general）は Store を持たないので、選択欄は今の general と同じく「最後に選んだもの」を保つ
- 見出し: 「アカウントと認証」と説明（キャンバスの文から `assignee:@me` の説明はそのまま、「トークンはこのマシンにだけ保存し、Store には書きません」）。「マウント中の Store」と説明（「このマシンで扱う Store です。見える範囲は各リポジトリのアクセス権で決まります。「表示」を外した Store のワークスペースは、サイドバーにもビューにも出しません。」）。サイドバーとビューから外すのは 4f-3 だが、文は完成の形で出す（4f は 4f-3 で揃う）

**テストの観点:** 2つのルートの読み書き・ナビの順と強調・閲覧のみの「（閲覧のみ）」（`StoreItem.read_only` と `sync.read_only` の両方）・非表示の Store も選択欄に出る・`settings.test.ts` の「まだない画面の項目（アカウントなど）は出さない」を「自動化・テンプレートは出さない」に直し、アカウント・マウント中の Store が出ることを確かめる・パレットの2つのコマンド・`routes.spec.ts`

- [ ] Step 1: router のテスト → 実装
- [ ] Step 2: `settings.test.ts` を直して落ちることを確かめる → ナビ・骨組み・振り分け
- [ ] Step 3: パレットのテスト → 実装
- [ ] Step 4: `routes.spec.ts` を通す
- [ ] Step 5: 検査を流してコミット `feat(web): 設定のナビにアカウントと認証・マウント中の Store を足す`

---

### Task 4: アカウントと認証（細部 18〜22・82・83）

**Files:**
- Modify: `web/app/settings/SettingsAccounts.svelte`
- Create: `web/app/settings/AccountCard.svelte`、`web/app/accounts/AccountAdd.svelte`（取得元の選択とフォーム。Onboarding も使うので `settings` の外に置く）、`web/app/accounts/DeviceFlow.svelte`（コード・リンク・残り時間・完了の待ち）
- Test: `web/app/settings/SettingsAccounts.test.ts`、`web/app/accounts/AccountAdd.test.ts`、`web/app/accounts/DeviceFlow.test.ts`
- Modify: `web/core/messages.ts`（足りない Code があれば）
- Create: `web/e2e/accounts.spec.ts`

**使うもの:** `AccountsCache`（Task 2）、`StoreDirectory.stores()`（使っている Store）、`GET /api/accounts`、`POST /api/accounts`（`{name, login, host?, token?}`）、`PUT /api/accounts/{name}/token`（`{token, host?}`。応答 `AccountLoginBody`）、`DELETE /api/accounts/{name}/token`、`POST /api/auth/device`（`{name?, host?}`。201 で `DeviceStartBody`）、`DELETE /api/auth/device/{id}`、`c.conn.on("auth.device")`、`ui/Avatar`・`ui/Menu`・`ui/Dialog`・`ui/Button`・`ui/CliLine`・`ui/FormError`、`ui/clipboard.ts`、`toast`、`errorMessage`、`cli.ts` の Task 2 の関数

**ふるまい:**
- 一覧（細部 18）: カードは頭文字のアバター、呼び名、`login@host`（login がなければ `id`）、状態のバッジ（`ok` は「有効」、`error` は「エラー」とカードの下に `error` の文、`none` は「トークンなし」。形でも区別する: 点の形かアイコン）、使っている Store（「使っている Store: a · b」、なければ「なし」）、トークンの取得元（`gh` は「gh」と `gh auth token --hostname <host> --user <login>`、`env:VAR` は「環境変数」と変数名、`keychain` は「keychain」と「OS の資格情報ストア」、`device` は「Device Flow」と「OS の資格情報ストア」と期限（`expires` を `YYYY-MM-DD HH:MM` のローカル時刻で「アクセストークンの期限」）、なしは「なし」）
- 読み込み中は 4d の States の読み込み、読めないときは文言と「もう一度読む」。0件は「アカウントがありません」と「アカウントを追加」
- 見出しの「アカウントを追加」で、一覧の下に追加の欄（キャンバスのとおり枠で囲む。ダイアログではない）を開く。閉じるボタンで閉じる
- 追加（細部 19）: 取得元のラジオ（Device Flow・トークンを貼る・gh・環境変数・トークンなし。それぞれキャンバスの説明の言い方で1行。既定は Device Flow）
  - Device Flow: 呼び名（任意。空は login）・ホスト（テキスト欄、既定 github.com）→「GitHub でログイン」で `POST /api/auth/device`。`DeviceFlow` がコード（コピーのボタン）、`verification_uri` のリンク（新しいタブ、`rel="noopener noreferrer"`）、「承認を待っています · コードの期限まで mm:ss」を出す。`auth.device` の `done` でトースト「アカウント <account> を追加しました」（ログインし直しなら「<account> にログインしました」）、欄を閉じ、`AccountsCache.reload()`・`StoreDirectory.reload()`。`error` はその文言を出して「やり直す」。欄を閉じる・取得元を変える・画面を離れるときは、走っているフローを `DELETE /api/auth/device/{id}`（失敗は無視）。409 は細部 82
  - トークンを貼る: 呼び名（必須）・ホスト・トークン（`type=password`、`autocomplete="off"`）→ `PUT /api/accounts/{name}/token`。成功でトースト（`added` なら「アカウント <account> を追加しました」、でなければ「<account> のトークンを保存しました」）。送ったらトークンの欄を空にする（失敗でも）
  - gh・環境変数・トークンなし: 呼び名・login（必須）・ホスト（環境変数は変数名も必須。英大文字・数字・`_`）→ `POST /api/accounts`（`token` は `gh`・`env:<変数>`・省略）
  - 足元の CLI の行は、選んだ取得元の行（CLI の表記）。トークンは入れない
- 操作（細部 20）: カードの「…」（`ui/Menu`、名前は「<呼び名> の操作」）。`keychain` は「ログインし直す」（トークンを貼るを、呼び名とホストを埋めて開く）と「ログアウト…」、`device` は「ログインし直す」（Device Flow を、呼び名とホストを埋めて開く）と「ログアウト…」。ログアウトは確認ダイアログ（「<呼び名> からログアウトします。keychain のトークンを消します。アカウントは config.yaml に残ります」、足元 `hh auth logout <名前>`）→ `DELETE /api/accounts/{name}/token`。`gh`・`env` は細部 83。トークンなしは「GitHub でログイン」（Device Flow を、呼び名とホストを埋めて開く）と「トークンを貼る」（usecase の `AuthLogin` はトークンなしのアカウントにどちらも許す）
- 削除は作らない（細部 21）。キャンバスの「GitHub 連携で使うアカウント」と GitHub App の警告は作らない（作らないもの）
- 操作のあとは `AccountsCache.reload()` と `StoreDirectory.reload()`（細部 22・70）

**テストの観点（単体）:** カードの各項目（取得元5つ・状態3つ・使っている Store のあり／なし・`device` の期限）・読み込み中・エラー・0件・Device Flow の開始→コード→`done` でトーストと取り直し・`error` の文言・閉じたら `DELETE`・409 の引き継ぎ（コードあり）とやり直し（`id` だけ）・期限切れ・残り時間の表示（偽の時計）・コピー・トークンを貼るの成功（`added` の2つのトースト）と失敗（トークンの欄が空になる）・gh・環境変数・トークンなしの本文・必須の欄・環境変数の名前の形・足元の CLI の行（トークンが入らない）・メニューの出し分け（取得元ごと）・ログアウトの確認と `DELETE`・削除と「GitHub 連携で使うアカウント」を出さないこと

**E2E（`accounts.spec.ts`、`github: true`）:**
- トークンなしのアカウントを追加（呼び名 `bot`・login `bot`）→ カードが「トークンなし」で出る、`hh auth status --json` に `bot`
- Device Flow: 呼び名 `work` で開始 → コードが出る → `hh.fakegh("device", "approve", <コード>, "--user", "e2e")` → `work` が「有効」で出る（トースト）。Device Flow を開始して欄を閉じると `hh` 側でフローが止まる（もう一度開始して 409 にならない）
- トークンを貼る: `fakegh token add t9 --user e2e` → 呼び名 `e2e` に t9 を貼る → 「有効」で取得元 keychain
- ログアウト: keychain の `e2e` をログアウト → 「トークンなし」。`hh auth status --json` でも消えている

- [ ] Step 1: `DeviceFlow.test.ts` → `DeviceFlow.svelte`
- [ ] Step 2: `AccountAdd.test.ts` → `AccountAdd.svelte`
- [ ] Step 3: `SettingsAccounts.test.ts` → `SettingsAccounts.svelte`・`AccountCard.svelte`
- [ ] Step 4: `accounts.spec.ts` を書いて通す
- [ ] Step 5: 検査を流してコミット `feat(web): アカウントと認証の画面（Device Flow・トークン・gh・環境変数・トークンなし、ログアウト）`

---

### Task 5: マウント中の Store（細部 23〜28・84）

**Files:**
- Modify: `web/app/settings/SettingsStores.svelte`
- Create: `web/app/settings/StoreCard.svelte`、`web/app/settings/StoreColorPicker.svelte`、`web/app/settings/StoreRemoveDialog.svelte`、`web/app/settings/StoreAddDialog.svelte`
- Test: `web/app/settings/SettingsStores.test.ts`、`StoreCard.test.ts`、`StoreRemoveDialog.test.ts`、`StoreAddDialog.test.ts`
- Modify: `web/core/messages.ts`（足りない Code があれば）
- Create: `web/e2e/stores.spec.ts`

**使うもの:** `StoreDirectory.stores()`・`reload()`・`readOnly()`（Task 2）、`readOnlyReason`、`c.conn.sync`（最後の同期 `last_success` は `StoreItem.last_success`）、`AccountsCache`（URL で追加のアカウントの選択欄）、`PATCH /api/stores/{name}`（`{color?, hidden?}`）、`PUT /api/stores/default`（`{name}`）、`DELETE /api/stores/{name}?keep=`（409 `store.unpushed`）、`POST /api/stores/add`（`{name, remotes, account?}`。400 `choice.required` の `--account`）、`create-state.svelte.ts`（CreateStore を開く。既存の開き方に合わせる）、`ui/Dialog`・`ui/Button`・`ui/CliLine`・`ui/FormError`、`toast`、`errorMessage`、`format.ts` の相対時刻（既存の「n 分前」の関数）

**ふるまい:**
- 一覧（細部 23）: config の順（`GET /api/stores` の順）にカード。色のボタン（色の点。`color` があればその色、なければ並び順の色（`StoreDirectory.tone` と同じ並び）、閲覧のみは灰色）、名前、権限のバッジ（「書き込み」、閲覧のみは「閲覧のみ」と `readOnlyReason`）、`default_store` なら「既定の Store」、`remotes[0] · account`（remote がなければ「remote なし」、アカウントがなければ省く）、最後の同期（「n 分前に同期」、なければ「未同期」）、「表示」のチェック、詳細の開閉（「詳細を開く: <名前>」/「閉じる: <名前>」）
- 色（細部 24）: ボタン「<名前> の色を変える（このマシンだけ）」でパレット（7色と「自動」。今の色に印。`ui/Menu` か同じ形のポップオーバー、キーボードで選べる）。選ぶと `PATCH {color}`（「自動」は `{color: ""}`）。足元や tooltip に CLI は出さない（パレットはダイアログではない）。閲覧のみの Store の点は灰色のままで、パレットの今の色の印は保存した色に付ける
- 表示（細部 25）: チェックを外すと `PATCH {hidden: true}`、付けると `{hidden: false}`。表示中が1つだけなら、そのチェックを無効にして「すべての Store を非表示にはできません」
- 詳細（細部 26）: remotes（2つ目以降に「ミラー」）、アカウント（なければ「なし」）、ローカルのパス（`path`）、操作「既定にする」（`default_store` でないとき。`PUT /api/stores/default`、トースト「既定の Store を <名前> にしました」）と「マウントを解除…」。キャンバスの merge driver・remote を追加・フォルダを開く・形式・メンバーは作らない
- 解除（細部 27）: 確認ダイアログ「<名前> のマウントを解除」。「ローカルのクローンも消す」のチェック（既定でオン）、足元 `hh store remove <名前> [--keep]`。409 `store.unpushed` はダイアログに文言を出して留まり、「クローンを残して解除」のボタンを出す（`keep=true` で送り直す）。成功でトースト「<名前> のマウントを解除しました」。最後の Store なら Onboarding に戻る（App の判定のまま）
- 見出しの操作（細部 28）: 「URL で追加」（`StoreAddDialog`: 名前（細部 84）、URL（複数行のテキスト欄、1行に1つ）、アカウント（`AccountsCache` が2つ以上のときだけ選択欄。未選択の既定は「選ばない」で `account` を送らない）。足元 `hh store add`。成功でトースト「Store「<名前>」を追加しました」、400 `choice.required` はその文言でアカウントの選択欄を出す）、「Store を作る」（CreateStore）。「Store を探す」は Task 6 で足す
- キャンバスの「新しく見つかった Store」の帯とタスク数は作らない（作らないもの・細部 23）
- 書き込みのあとは `StoreDirectory.reload()`。SSE の `stores.changed` でも一覧は取り直る（Task 2）

**テストの観点（単体）:** カードの各項目（色の3つの元・権限と理由・既定の印は `default_store` だけで `default` を見ない・remote なし・未同期）・色のパレットと `PATCH`（自動は `""`）・表示の `PATCH` と最後の1つの無効と理由・詳細の項目とミラー・既定にする・解除の確認と `keep` の2つ・409 `store.unpushed` で留まって「クローンを残して解除」・URL で追加の名前の提案と複数の URL・アカウントの選択欄の出し分け・`choice.required`・作らないもの（merge driver・remote を追加・フォルダを開く・形式・メンバー・新しく見つかった Store・タスク数）を出さない・足元の CLI の行

**E2E（`stores.spec.ts`）:** harness に Store を2つ作る手段（`hh store init` をもう1回、`StoreSpec` の配列か `hh.hh` で足す）と、bare の remote を持つ Store を別の home から作る手段を足す
- 色: 2つ目の Store を `sky` にする → カードの点が変わり、`hh store list --json` の `color` が `sky`。「自動」で消える
- 表示: 1つ目を非表示にする → `hidden: true`。残りの1つのチェックは無効
- 既定: 2つ目を既定にする → 「既定の Store」が移り、`hh store default` が2つ目
- URL で追加: 別の home で作って push した bare の URL を入れ、名前 `team` で追加 → カードが出る、`hh store list` に `team`
- 解除: `team` をクローンも消して解除 → カードが消え、ディレクトリがない。push していない変更がある Store（`hh add` してから sync しない）の解除は文言が出て、「クローンを残して解除」で解除できる（ディレクトリは残る）
- 最後の Store を解除すると Onboarding が出る

- [ ] Step 1: `StoreCard.test.ts` → `StoreCard.svelte`・`StoreColorPicker.svelte`
- [ ] Step 2: `StoreRemoveDialog.test.ts` → `StoreRemoveDialog.svelte`
- [ ] Step 3: `StoreAddDialog.test.ts` → `StoreAddDialog.svelte`
- [ ] Step 4: `SettingsStores.test.ts` → `SettingsStores.svelte`
- [ ] Step 5: `stores.spec.ts`（と harness の部品）を書いて通す
- [ ] Step 6: 検査を流してコミット `feat(web): マウント中の Store の画面（色・表示・既定・解除・URL で追加）`

---

### Task 6: 発見・Onboarding・CreateStore のアカウント（細部 28 の「Store を探す」・29〜32・78〜81・85）

**Files:**
- Create: `web/app/stores/StoreDiscover.svelte`・`StoreDiscover.test.ts`（候補の一覧・フォルダから探す・マウント。Onboarding と設定のダイアログが使う）、`web/app/stores/StoreDiscoverDialog.svelte`（設定の「Store を探す」）
- Modify: `web/app/states/Onboarding.svelte`、`web/app/states/states.test.ts`（Onboarding の検査）
- Modify: `web/app/App.svelte`・`App.test.ts` か `gates.test.ts`（細部 78 の「Onboarding の途中」）
- Modify: `web/app/settings/SettingsStores.svelte`・`SettingsStores.test.ts`（「Store を探す」）
- Modify: `web/app/create/CreateStore.svelte`・`CreateStore.test.ts`（細部 32・85）
- Modify: `web/e2e/onboarding.spec.ts`（3段の Onboarding に合わせる）、`web/e2e/screens.ts` のコメント（Onboarding の文言）
- Create: `web/e2e/discover.spec.ts`

**使うもの:** `GET /api/stores/discover?account=&from=`（`Candidate[]`。422 `choice.required` の `args.what` は `--account`、`args.candidates` はアカウントの名前）、`POST /api/stores/mount`（`MountCandidate[]`。応答 `{mounted, failed: [{name, code, message}]}`）、`AccountsCache`・`source()`、`AccountAdd`（Task 4）、CreateStore、`StoreDirectory.reload()`、4d の States の読み込み中（細部 31）、`ui/Dialog`・`ui/Button`・`ui/CliLine`、`toast`、`errorMessage`

**ふるまい:**
- 発見（細部 30・80・81）: トークンを持つアカウント（`source` が `none` 以外で状態が `ok`）があれば開いたときに `GET /api/stores/discover` を呼ぶ（読み込み中は States）。候補は `url` の owner（`github.com/<owner>/…` の `<owner>`。owner が取れない URL は「その他」）ごとに見出し（owner と、候補の `account`）。行は選択のチェック（既定はオフ）、repo（URL の `owner/name`、取れなければ URL）、権限（`write` は「書き込み」、`read` は「閲覧のみ」、ほかはその値）、呼び名の欄（`aria-label`「<repo> の呼び名」）。0件は「見つかりませんでした」と「探しているチームの Store が見つからないときは、そのリポジトリの権限をチームの管理者に依頼してください」。読めないときは文言と「もう一度探す」
- 「フォルダから探す」: パスの欄と「探す」で `from` を付けて呼ぶ。422 `choice.required` ならアカウントの選択欄（`args.candidates`）を出し、選んで探し直す（`account` を付ける）
- マウント: 「マウント」（Onboarding は「マウントして次へ」）で選んだ候補を `POST /api/stores/mount`。`failed` はその行に `errorMessage({type: code, detail: message})`、`mounted` の行は「マウントしました」にしてチェックを外す。1つ以上マウントしたら `StoreDirectory.reload()`。足元は `hh store discover [--account] [--from]`
- 設定の「Store を探す」（細部 28）: `StoreDiscoverDialog`（見出し「Store を探す」）で `StoreDiscover` を出す。マウントしたらトースト「n 個の Store をマウントしました」
- Onboarding（細部 29・78・79）: 段の表示（アカウント・Store を選ぶ・はじめる。今の段に `aria-current="step"`）
  - アカウントの段: 見出し「GitHub のアカウントを登録します」と説明（キャンバスの文）。`AccountsCache.load()`。アカウントがあればカード風の一覧（呼び名・`login@host`・状態）、「アカウントを追加」で `AccountAdd`。なければ `AccountAdd` を開いた状態で出す。「次へ: Store を探す」（0件なら無効と理由）。「あとから 設定 › アカウントと認証 で変えられます」。キャンバスの gh の自動検出は作らない
  - Store を選ぶの段: 見出し「使う Store を選んでください」と説明、`StoreDiscover`。「戻る」「マウントして次へ」。マウントしたものが1つ以上あれば、はじめるの段へ（失敗した行があれば段に留まり、「次へ」で進める）
  - はじめるの段: マウントした Store を並べ（名前と権限）、「始める」でホームへ（Onboarding を閉じる）
  - どの段の足元にも「新しく Store を作る」（CreateStore を開く。作り終えたら Onboarding を閉じ、今の CreateStore の遷移のまま）
- App（細部 78）: `loaded && count === 0` になったら「Onboarding の途中」にし、Onboarding の「始める」か CreateStore の成功で終える。途中のあいだは Store が増えても Onboarding を出し続ける
- CreateStore（細部 32・85）: 開いたときに `AccountsCache.load()`。アカウントが2つ以上なら「アカウント」の選択欄（既定は先頭ではなく「選ぶ」で、未選択なら送信を止めて「アカウントを選んでください」）。1つなら出さず送らない。`POST /api/stores` に `account`、足元に `--account`

**テストの観点（単体）:** 発見の自動の呼び出しとトークンなしで呼ばないこと・owner ごとの見出し・権限の言い方・呼び名の初期値と空の名前・フォルダから探すと `choice.required` の選択欄・0件・読めない・マウントの `failed` の行の文言と `mounted`・足元の CLI の行・Onboarding の段の移り（0件で次へが無効、トークンなしでも進める、戻る、マウント後のはじめる、始めるで閉じる）・どの段にも「新しく Store を作る」・gh の自動検出を出さない・App の「途中」で Store が増えても Onboarding が出たまま・CreateStore のアカウントの出し分けと送信と `--account`

**E2E:**
- `onboarding.spec.ts`: harness のアカウント `e2e`（トークンなし）があるので、アカウントの段に `e2e` が出て「次へ」で進める。「新しく Store を作る」から今の作成の検査を通す（文言の変わったところだけ直す）
- `discover.spec.ts`（`store: null`）: 別の home で Store を2つ作って `<home>/remotes/` の bare に push しておく → Onboarding でアカウントの段を進み、「フォルダから探す」にそのパスを入れて探す → 候補が2つ → 1つの呼び名を変えて両方マウント → はじめるの段に2つ → 「始める」でホーム、サイドバーに2つの Store のワークスペース。設定の「Store を探す」で同じフォルダを探すと、マウント済みのものは出ない（usecase がマウント済みを外す）

- [ ] Step 1: `StoreDiscover.test.ts` → `StoreDiscover.svelte`・`StoreDiscoverDialog.svelte`、SettingsStores の「Store を探す」
- [ ] Step 2: Onboarding と App のテスト → 実装
- [ ] Step 3: `CreateStore.test.ts` → 実装
- [ ] Step 4: `onboarding.spec.ts` を直し、`discover.spec.ts` を書いて通す
- [ ] Step 5: 検査を流してコミット `feat(web): Store の発見とマウント、3段の Onboarding、CreateStore のアカウント`

---

### Task 7: 仕上げ

- [ ] Step 1: 細部 17〜33 と 71〜85 の全行について、実装とテストの場所を plan の末尾に表で残す。足りないものがあればこの回で入れる
- [ ] Step 2: 「作らないもの」を出していないことを grep とテストで確かめる（`GitHub 連携で使うアカウント`・`merge driver`・`remote を追加`・`フォルダを開く`・`メンバー`・`新しく見つかった`・`gh にログイン済み`）
- [ ] Step 3: 概要 plan に「4f-2 の plan で足した細部」（71〜85 と、実装で決まったもの）を写す。spec の「4f の細部」に、E2E の keyring（細部 71）の1文を足す
- [ ] Step 4: 全体の検査（`go vet`・`gofmt -l .`・`golangci-lint run ./...`・`go test -race ./...`、`pnpm fix`・`pnpm check`・`biome ci`・`vitest`・`pnpm build`、`make e2e` を3回）を流し、結果を plan の末尾に残す
- [ ] Step 5: 引き継ぎ `docs/superpowers/plans/2026-11-04-phase4f2-handoff.md` を書いてコミット `docs(plan): 4f-2 の実装で決めた点と引き継ぎを書く`
