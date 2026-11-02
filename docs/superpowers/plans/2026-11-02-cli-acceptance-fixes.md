# CLI の受入テストの修正 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 2026-11-02 の CLI の受入テストで決めた 12 の修正（コマンド名、`show`、`store remove`、アカウント必須、client ID、`add --repo`、worktree、`purge --older-than`、`--rewrite-history` の廃止、GUI と API の完全削除の廃止、文言）を入れる。

**Architecture:** 影響が全体に及ぶ「アカウント必須」を最初に入れ（テストの fixture を先に揃え、そのあと `.gitconfig` の経路を消す）、あとはコマンドごとの小さな Task にする。完全削除の経路は、API → 履歴の書き換え → `purge` の作り直しの順に外し、各 Task のコミットがそれぞれ緑になるようにする。最後に文言をまとめて揃え、既存の spec と Artifact を直し、受入テスト手順を通す。

**Tech Stack:** Go（cobra、go-git v5、huma v2）、TypeScript（web/core）。既存のまま。

**Spec:** `docs/superpowers/specs/2026-11-02-cli-acceptance-fixes-design.md`（以下「spec」。節番号は spec の番号）。親設計は Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc、rev 356 で spec に合わせ済み）。

## Global Constraints

- コードの識別子・コメント・CLI の文言は英語（既存と同じ）。GUI の文言は日本語
- リリース前なので、廃止するコマンド名・オプション・ファイル形式の互換は残さない（spec §1、§9.1）
- 廃止したコマンド・オプションは使い方の誤り（終了コード 2）になることをテストで確かめる（spec「テスト」）
- 新しいテストには `t.Parallel()` を付ける
- 小さな直しのコミットの前は `go vet ./...` と、触ったパッケージのテストだけを回す。全テスト（`make test`）は Task 13 で1回だけ回す（利用者の希望）。サブエージェントのテストは前面で、Bash の timeout は 600000
- spec・親設計と違う挙動にしたくなったら、実装を止めて報告する（自分で決めない）
- 各 Task のコミットは、それだけで `go build ./...`・`go vet ./...`・触ったパッケージのテストが通る状態にする
- コミットメッセージは既存の形（`feat(cli): …`・`refactor(sync): …` など、本文は日本語）。末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`

## Task の一覧と順序

| Task | 内容 | spec |
| --- | --- | --- |
| 1 | テストの環境に既定でアカウントを書き、CLI のテストの期待値をアカウントの作者に揃える | §5.5 |
| 2 | アカウントを必須にする（入口の判定、Store のアカウントの決め方、`.gitconfig` の経路の削除） | §5.1〜5.3 |
| 3 | `hh auth add` と、2 つ目のアカウントを足すときの `stores[].account` の書き込み | §5.4 |
| 4 | client ID を組み込まない | §6 |
| 5 | `workspace set-prefix` と `comment add` | §1、§2 |
| 6 | `show` の `title`・`checklist`・`progress` の行 | §3 |
| 7 | `store remove` の既定の削除と `--keep` | §4 |
| 8 | `add --repo` と git worktree の中のリポジトリ | §7、§8 |
| 9 | GUI と API の完全削除を外す | §11 |
| 10 | `--rewrite-history` と書き換えの記録を消し、force push を doctor --fix で直す | §10 |
| 11 | `purge --older-than` と転送ファイルの `moved_at` | §9、§9.1 |
| 12 | 英語の複数形と `ls --help` | §12 |
| 13 | 既存の spec・Artifact の更新、受入テスト手順の通し、全テスト | 「後片付け」「テスト」 |

順序の理由: Task 1・2 は全 CLI テストの期待値を変えるので最初に入れ、後の Task のテストはアカウントがある前提で書く。Task 9（API）→ 10（書き換え）→ 11（purge）は、`purge` を使う側から外していき、Task 11 で `purge` を 1 つの形に作り直す。Task 12 は文言が出そろってから一度に揃える。

### Task 1: テストの環境に既定でアカウントを書き、期待値をアカウントの作者に揃える

spec §5.5。このあとの Task 2 でアカウントを必須にし、`.gitconfig` の経路を消す。そのとき全 CLI テストの期待値が一度に動くと差分が読めなくなるので、先に「テストの環境が既定でアカウントを1つ持つ」状態に揃え、期待値だけをここで直す。製品のコードはこの Task では変えない（`config.Author` の経路も残す。テストが緑のままコミットできるようにするため）。

**この Task の方針（実際に試して決めた）。** 試しにヘルパーだけを変えて 3 つのパッケージを回すと、落ちるテストは `cmd/hedgehug` が 20、`internal/ops` が 19、`internal/server` が 7、`internal/config`・`internal/sync`・`internal/mount`・`tools/devenv` は 0 だった（この数え方は下の Step 5 の表のとおり）。落ちる理由は 2 つだけ。

- A. **アカウントなしの動きを確かめるテスト。** 既定のアカウントがあると前提が崩れる。最初に `dropDefaultAccount()` を呼んでアカウントを外す。Task 2 でアカウントなしの書き込みが拒否になるので、書き込みを伴う A のテストは Task 2 で作り直すか消す（Task 2 に一覧がある）。
- B. **作者や `by -` を期待値に書いたテスト。** 作者が `.gitconfig` の Alice から、アカウントの login（`alice`、メールは `alice@users.noreply.github.com`）に変わる。`show` の `created … by -` は `by github.com/alice`、`watchers -` は `watchers github.com/alice`（書き込んだ人が自動で watcher になる）に変わる。期待値を新しい値に直す。ゴールデンは `-update` で作り直して diff を読む。

**Files:**
- Modify: `cmd/hedgehug/helpers_test.go`（`newTestEnv` が `config.yaml` に既定のアカウントを書く。`dropDefaultAccount` を足す。`writeAccounts`・`writeConfigYAML` が既定のアカウントを置き換える）
- Modify: `internal/ops/helpers_test.go`（`newTestSession`・`otherSession` が既定のアカウントを書く。`dropDefaultAccount`・`appendDefaultAccount`・`testAccount` を足す。`appendConfig` が既定のアカウントを置き換える）
- Modify: `internal/server/helpers_test.go`（`newTestServerWith` が既定のアカウントを書く。`dropDefaultAccount` を足し、`withAccount` を消す）
- Modify（期待値を直す。Step 5 の表が全部）: `cmd/hedgehug/{auth,inbox,undo,watch,store_init,background,add,log,scenario,comment,tree}_test.go` ほか、`internal/ops/{auth,task,session_more,undo,mention,inbox,read,people,conn,discover,storeops,storeinit_remote,token_kind}_test.go`、`internal/server/{comments,activity,multistore,people,stores_account}_test.go`
- Modify（`-update` で作り直す）: `cmd/hedgehug/testdata/TestLsTreeProgress/*.golden`、`TestPrefixAcrossStores/*.golden`、`TestShowTellsItsStore/show.golden`、`TestLsOneStoreHasNoStoreColumn/show.golden`、`TestStoreAddMirror/list.golden`、`TestStoreAddWarnsPrefixDuplicate/list.golden`

**Interfaces:**
- Consumes: 既存の `testEnv`・`testSession`・`testServer`、`config.Account`、`Session.resetWriter`
- Produces（Task 2 以降のテストが使う）:
  - `cmd/hedgehug`: `const defaultAccountYAML`、`(*testEnv).dropDefaultAccount()`
  - `internal/ops`: `const defaultAccountYAML`、`var testAccount config.Account`（name・login は `alice`、host は `github.com`）、`(*testSession).appendDefaultAccount()`、`(*testSession).dropDefaultAccount()`。`appendConfig(text)` は text に `accounts:` があれば既定のアカウントを外してから足す。`authSession(t, st, "")`・`fakeSession(t, st, "")` はアカウント文字列が空なら既定のアカウントも外す（「アカウントなし」の意味を保つ）
  - `internal/server`: `const defaultAccountYAML`、`(*testServer).dropDefaultAccount()`
  - 既定のアカウントの名前は `alice`、ID は `github.com/alice`。コミットの作者は `alice <alice@users.noreply.github.com>`

- [ ] **Step 1: `cmd/hedgehug` のヘルパーを変える**

`cmd/hedgehug/helpers_test.go`。import に `regexp` を足す（`"path/filepath"` の次の行）。`testEnv` の説明と `newTestEnv` の終わりを変える。

```go
// testEnv runs the command with HEDGEHUG_HOME and HOME in temporary
// directories, an author in HOME/.gitconfig, and the account alice in
// config.yaml.
```

`newTestEnv` の `.gitconfig` を書く `if` の直後、`return te` の前に足す。

```go
	if err := os.MkdirAll(te.hhHome, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(te.hhHome, "config.yaml"), []byte(defaultAccountYAML), 0o600); err != nil {
		t.Fatal(err)
	}
```

`newTestEnv` の直後に足す。

```go
// defaultAccountYAML is the account every test environment starts with: the
// commits are alice's and the id in the output is github.com/alice.
const defaultAccountYAML = "accounts:\n  - name: alice\n    host: github.com\n    login: alice\n"

// storeAccountRE matches the account: alice line of a store in config.yaml,
// which store init writes.
var storeAccountRE = regexp.MustCompile(`(?m)^[ \t]+account: alice\n`)

// dropDefaultAccount removes the account newTestEnv wrote, and the stores'
// reference to it, for the tests that have no account or set up their own.
func (te *testEnv) dropDefaultAccount() {
	te.t.Helper()
	p := filepath.Join(te.hhHome, "config.yaml")
	b, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		te.t.Fatal(err)
	}
	text := storeAccountRE.ReplaceAllString(strings.Replace(string(b), defaultAccountYAML, "", 1), "")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		te.t.Fatal(err)
	}
}
```

`writeAccounts` と `writeConfigYAML` の先頭（`te.t.Helper()` の次）に足す。`writeConfigYAML` は `accounts:` を含むときだけ。

```go
func (te *testEnv) writeAccounts(host, login string) { //nolint:unparam // …（既存のまま）
	te.t.Helper()
	te.dropDefaultAccount()
	…
}

func (te *testEnv) writeConfigYAML(s string) {
	te.t.Helper()
	if strings.Contains(s, "accounts:") {
		te.dropDefaultAccount()
	}
	…
}
```

- [ ] **Step 2: `internal/ops` のヘルパーを変える**

`internal/ops/helpers_test.go`。import に `"regexp"` と `"strings"` を足す。

`appendConfig` を次に置き換え、その前に既定のアカウントの定義を足す。既定のアカウントは **config.yaml の末尾** に置く（`asAccount`（`mention_test.go`）が `"\naccounts:"` から後ろを切り捨てるため、先頭に置くと stores まで消える）。

```go
// defaultAccountYAML is the account every test session starts with: the
// commits are alice's.
const defaultAccountYAML = "accounts:\n  - name: alice\n    host: github.com\n    login: alice\n"

// testAccount is the account of defaultAccountYAML.
var testAccount = config.Account{Name: "alice", Host: "github.com", Login: "alice"}

var storeAccountRE = regexp.MustCompile(`(?m)^[ \t]+account: alice\n`)

// appendDefaultAccount adds defaultAccountYAML to the end of config.yaml.
func (ts *testSession) appendDefaultAccount() {
	ts.t.Helper()
	if err := os.MkdirAll(ts.env["HEDGEHUG_HOME"], 0o750); err != nil {
		ts.t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // the config of the test
	if err != nil {
		ts.t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(defaultAccountYAML); err != nil {
		ts.t.Fatal(err)
	}
}

// dropDefaultAccount removes defaultAccountYAML, and the stores' reference to
// it, for the tests that have no account or set up their own.
func (ts *testSession) dropDefaultAccount() {
	ts.t.Helper()
	p := filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml")
	b, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil {
		ts.t.Fatal(err)
	}
	text := storeAccountRE.ReplaceAllString(strings.Replace(string(b), defaultAccountYAML, "", 1), "")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		ts.t.Fatal(err)
	}
	ts.resetWriter()
}

// appendConfig adds text to config.yaml; text that has accounts: replaces the
// default account.
func (ts *testSession) appendConfig(text string) {
	ts.t.Helper()
	if strings.Contains(text, "accounts:") {
		ts.dropDefaultAccount()
	}
	_, p, err := ts.Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	f, err := os.OpenFile(p.ConfigFile, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		ts.t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(text); err != nil {
		ts.t.Fatal(err)
	}
}
```

`newTestSession`。ストアの作者をアカウントにし、`c.Save()` の後で既定のアカウントを足す（`Session` の `Getenv` は `ts.Getenv`）。

```go
	author, err := config.CommitAuthor(testAccount, true, ts.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	…（store.Create と AddStore は今のまま）
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	ts.appendDefaultAccount()
	return ts
```

`otherSession` は `return c` の前に `c.appendDefaultAccount()` を足す。`newTwoStoreSessionWith` は自分でアカウントを足す作りなので変えない。

`authSession`（`auth_test.go`）と `fakeSession`（`storeinit_remote_test.go`）は、アカウント文字列が空のとき既定のアカウントを外す。`ts.appendConfig(` の行の前に足す。

```go
	if accounts == "" {
		ts.dropDefaultAccount()
	}
```

- [ ] **Step 3: `internal/server` のヘルパーを変える**

`internal/server/helpers_test.go`。import に `"regexp"` と `"strings"` を足す。`newTestServerWith` で `StoreInit` を呼ぶ前に、`HEDGEHUG_HOME` に既定のアカウントを書く（`StoreInit` は唯一のアカウントを選んで `stores[].account` に書く）。

```go
	if err := os.MkdirAll(filepath.Join(d, "hh"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "hh", "config.yaml"), []byte(defaultAccountYAML), 0o600); err != nil {
		t.Fatal(err)
	}
```

`withAccount` を消して `dropDefaultAccount` に置き換える（`withAccount` の呼び出し 8 か所は、アカウントがもうあるので行ごと消す: `multistore_test.go` 2、`activity_test.go` 3、`comments_test.go` 2。`grep -rn 'withAccount()' internal/server` で洗い出す）。

```go
// defaultAccountYAML is the account every test server starts with.
const defaultAccountYAML = "accounts:\n  - name: alice\n    host: github.com\n    login: alice\n"

// dropDefaultAccount removes it, and the stores' reference to it, for the
// tests that have no account or set up their own.
func (ts *testServer) dropDefaultAccount() {
	ts.t.Helper()
	p := filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml")
	b, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil {
		ts.t.Fatal(err)
	}
	text := strings.Replace(string(b), defaultAccountYAML, "", 1)
	text = regexp.MustCompile(`(?m)^\s+account: alice\n`).ReplaceAllString(text, "")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		ts.t.Fatal(err)
	}
}
```

`accountID()` は残す（`c.Accounts[0].ID()` が `github.com/alice` を返す）。

- [ ] **Step 4: 落ちるテストを洗い出す**

Run: `go vet ./cmd/hedgehug ./internal/ops ./internal/server && go test ./cmd/hedgehug ./internal/ops ./internal/server -count=1 2>&1 | grep -E '^(--- FAIL|FAIL|ok)'`（Bash の timeout は 600000）
Expected: FAIL。落ちるテストは次の表のとおり。パッケージの中で 1 つでも panic すると後ろのテストが走らないので、表のテストを直しながら繰り返し回して、表にないテストが落ちたらその理由を A か B に当てはめて直す（`internal/ops` の `TestPickAccount` は、アカウントなしの部分が落ちると後ろで nil を参照して panic する）。

- [ ] **Step 5: 期待値を直す**

**`cmd/hedgehug`（20）**

| テスト | 場所 | 分類 | 直し方 |
| --- | --- | --- | --- |
| `TestAuthLoginEmptyStdin` | `auth_test.go:215` | A | `te := newTestEnv(t)` の次に `te.dropDefaultAccount()` |
| `TestAuthStatusEmptyAndBare` | `auth_test.go:380` | A | 同上（Task 2 で文言を直す） |
| `TestInboxWithoutAccounts` | `inbox_test.go:208` | A | `store init` の前に `te.dropDefaultAccount()` |
| `TestUndoWithoutAccounts` | `undo_test.go:93` | A | 同上 |
| `TestNoWatcherWithoutAccounts` | `watch_test.go:87` | A | 同上 |
| `TestStoreInitNoAuthor` | `store_init_test.go:124` | A | `.gitconfig` を消す前に `te.dropDefaultAccount()` |
| `TestSyncBackgroundRecordsAFailureBeforeTheRun` | `background_test.go:90` | A | `store init` のあと、`.gitconfig` を消す前に `te.dropDefaultAccount()` |
| `TestAddWithoutAccountUsesGitConfig` | `add_test.go:186` | A | `newStoreEnv(t)` を `newTestEnv(t)` + `te.dropDefaultAccount()` + `te.ok("store", "init", "me", "--prefix", "ME")` に変える |
| `TestStoreInit` | `store_init_test.go:30` | B | 作者を `log[0].Author.Name != "alice" \|\| log[0].Author.Email != "alice@users.noreply.github.com"`、メッセージの「one commit by Alice」を alice に |
| `TestLog` | `log_test.go:21` | B | 正規表現の `Alice` と `want` の `"  Alice  "` を `alice` に |
| `TestLogJSON` | `log_test.go:93` | B | `e.Author.Name`・`Email` を alice の値に |
| `TestScenario` | `scenario_test.go:106` | B | 「store init はアカウントを書く前だった」の分岐（最後のコミットだけ `alice@example.com`）をやめ、全コミットが `alice@users.noreply.github.com` |
| `TestCommentAddFromArgs` | `comment_test.go:42` | B | `got.Author != ""` を `got.Author != "github.com/alice"` |
| `TestShowCommentULID` | `comment_test.go:353` | B | 「作者のないコメント」は既存ファイルの形としてだけ残る。`putComments`（`comment_test.go:320`）を `putCommentsBy(ulid, author string, commentULIDs ...string)` に一般化して（`putComments` は `"github.com/alice"` で呼ぶ）、`te.putCommentsBy(x.ULID, "", a)` で作者が空のコメントを書き、`show` の出力が `"\n  " + a + "  -  "` を含むことを確かめる。`created` の `2026-` は求めない（書き込み用のヘルパーの Created は零値） |
| `TestLsTreeProgress` | `tree_test.go:73-75` | B（golden） | `-update` |
| `TestPrefixAcrossStores` | `prefix_stores_test.go:53-54` | B（golden） | `-update` |
| `TestShowTellsItsStore` | `ls_stores_test.go:76` | B（golden） | `-update` |
| `TestLsOneStoreHasNoStoreColumn` | `ls_stores_test.go:70` | B（golden） | `-update` |
| `TestStoreAddMirror` | `store_more_test.go:51` | B（golden） | `-update` |
| `TestStoreAddWarnsPrefixDuplicate` | `store_add_test.go:23` | B（golden） | `-update` |

ゴールデンを作り直す: `go test ./cmd/hedgehug -run 'TestLsTreeProgress|TestPrefixAcrossStores|TestShowTellsItsStore|TestLsOneStoreHasNoStoreColumn|TestStoreAddMirror|TestStoreAddWarnsPrefixDuplicate' -update -count=1`（`golden_test.go` の `-update` フラグ）。そのあと `git diff cmd/hedgehug/testdata` を読む。変わってよいのは次の 4 種類だけ。これ以外が出たらコードの誤りなので止める。

- `watchers    -` → `watchers    github.com/alice`
- `created     <TIME> by -` → `created     <TIME> by github.com/alice`
- 履歴の行の `Alice` → `alice`
- `--json` の `"watchers": []` → `["github.com/alice"]`、作者の `Alice`・`alice@example.com` → `alice`・`alice@users.noreply.github.com`

`TestInboxWithoutAccounts`・`TestUndoWithoutAccounts` の `testdata` は、A の直しで元の出力のまま通る（`-update` は要らない）。

**`internal/ops`（19）**

| テスト | 場所 | 分類 | 直し方 |
| --- | --- | --- | --- |
| `TestWatchAddsAndRemovesMe` | `task_test.go:265` | A | アカウントなしの部分の前に `ts.dropDefaultAccount()` |
| `TestWriterHost` | `task_test.go:460` | A | アカウントなしの部分の前に `ts.dropDefaultAccount()` |
| `TestAccountIDWithoutAccountIsEmpty` | `session_more_test.go:93` | A | 先頭に `ts.dropDefaultAccount()` |
| `TestUndoNeedsAccounts` | `undo_test.go:79` | A | 先頭に `ts.dropDefaultAccount()` |
| `TestCommentWithoutAuthor` | `mention_test.go:116` | A | 先頭に `ts.dropDefaultAccount()` |
| `TestInboxStoresWarnsOfMissingAccounts` | `inbox_test.go:74` | A | 先頭に `ts.dropDefaultAccount()`（最後の `appendConfig(aliceAccount)` はそのまま） |
| `TestAuthStatusAllGood` | `auth_test.go:468` | A | 直さない（`authSession(t, …, "")` が自動で外す）。直したあと通ることを確かめる |
| `TestAuthLoginEmptyToken` | `auth_test.go:237` | A | 同上 |
| `TestAuthLoginBadTokenIsUnauthorized` | `token_kind_test.go:57` | A | 同上 |
| `TestAuthLoginNewAccountByDevice` | `auth_test.go:133` | A | 同上 |
| `TestAuthLoginRollsBackTheKeychain` | `auth_test.go:307` | A | 同上（「another writer added the account」が既定の alice と重なっていた） |
| `TestStoreInitNoAccountAtAll` | `storeinit_remote_test.go:278` | A | 同上（`fakeSession(t, …, "")`） |
| `TestPickAccount` | `conn_test.go:270` | A | `newTestSession` の次に `ts.dropDefaultAccount()` |
| `TestDiscoverFrom` | `discover_test.go:74` | B | `dropDefaultAccount()` を呼ばない。`newTestSession` のままで、`want` の 2 つの候補に `Account: "alice"` を足す（唯一のアカウントが選ばれる）。Task 2 で書き込みを伴うようになる（`StoreMount` が `StoreAdd` を呼ぶ）ので、`dropDefaultAccount` にはしない |
| `TestDiscoverFromAccounts` | `discover_test.go:98` | B | 「アカウントなし」の部分: `ts.dropDefaultAccount()` を呼ばず、`cands[0].Account != "alice"` を期待し、コメントを「The only account: the candidate has it.」に |
| `TestPeopleFoldsCase` | `people_test.go:176` | B | `dropDefaultAccount()` を呼ばず、`want` を `[]string{"ALICE", "bob", "github.com/alice"}` に |
| `TestShowViewOfEmpty` | `read_test.go:363` | B | `dropDefaultAccount()` を呼ばない。`ts.Add` の次に `ts.Watch("ME-1", false)`（書き込んだ人が watcher になるので外す）を足し、空の `watchers` の確認を通す |
| `TestLogViewsOfAndSubjectNow` | `read_test.go:490` | B | `v.Author.Name != "alice" \|\| v.Author.Email != "alice@users.noreply.github.com"` |
| `TestStoreInitListRemove` | `storeops_test.go:44` | B | 2 つ目の Store の `Account` の期待値を `"github.com/alice"` に（メッセージの「no account」も） |

**`internal/server`（7）**

| テスト | 場所 | 分類 | 直し方 |
| --- | --- | --- | --- |
| `TestWatchMeWithoutAccountIsConflict` | `comments_test.go:74` | A | `newTestServer` の次に `ts.dropDefaultAccount()`（Task 2 で並べ替える） |
| `TestUndoWithoutAccountIsConflict` | `activity_test.go:36` | A | 同上 |
| `TestNotificationsWarnOnce` | `activity_test.go:139` | A | 同上（コメントの「no account」を残す） |
| `TestStoresAccountIsHostSlashLogin` | `stores_account_test.go:11` | A | `ts.dropDefaultAccount()` を先頭に。そのあと `work` を足す今の流れのまま通る |
| `TestPeopleAPI` | `people_test.go:8` | B | `want` に `"github.com/alice"` を足す（`bob`・`carol`・`github.com/alice`・`zed`） |
| `TestActivity` | `activity_test.go:44`（64 行目） | B | `author=` で絞ると `store init` のコミットも alice のものになる。期待を 2 から 3（init と 2 つの add）に、メッセージも直す |
| `TestActivityFiltersMergedList` | `multistore_test.go:240`（246 行目） | B | 同じ理由で期待を 4 から 8 に（2 つの Store の init と、`twoStores` の最初の add と、ここで足す 4 つの add）。数を `go test` の出力で確かめ、メッセージを直す |

- [ ] **Step 6: 緑を確かめる**

Run: `go vet ./... && go test ./cmd/hedgehug ./internal/ops ./internal/server ./internal/config ./tools/devenv -count=1`（timeout 600000）
Expected: すべて `ok`。`git diff --stat` に製品のコード（`_test.go` と `testdata` 以外）がないこと。

- [ ] **Step 7: コミット**

```bash
git add cmd/hedgehug internal/ops internal/server
git commit -m "test: テストの環境に既定でアカウントを書き、期待値をアカウントの作者に揃える

テストの環境が alice のアカウントを 1 つ持つ状態から始まるようにする。
コミットの作者は .gitconfig の Alice ではなくアカウントの login になり、
show の by - は by github.com/alice になる。アカウントなしの動きを
確かめるテストだけが dropDefaultAccount でアカウントを外す。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

#### spec にない細部（Task 1）

1. 既定のアカウントの名前・host・login は `alice`・`github.com`・`alice`。`writeAccounts("github.com", "alice")` を呼んでいる既存の 60 前後のテストと同じ値で、`name` と `login` が同じなので `stores[].account` の `alice` もそのまま通る。
2. `writeAccounts`・`writeConfigYAML`（`accounts:` を含むとき）・`appendConfig`（同）は、既定のアカウントを外してから書く。既存のテストが「アカウントを足した」つもりで書いているものを壊さないため。
3. `internal/ops` の既定のアカウントは config.yaml の末尾に置く。`asAccount`（`mention_test.go`）が `"\naccounts:"` から後ろを切り捨てるので、先頭に置くと Store の一覧まで消える。
4. `dropDefaultAccount` は Store の `account: alice` の行も外す。`store init` が唯一のアカウントを選んで `stores[].account` に書くので、行が残るとアカウントを外したあとの読み込みが `account "alice" is not in accounts` で落ちる。
5. `internal/server` の `withAccount`（8 か所）は、アカウントが最初からあるので消す。`accountID()` は残す。

---

### Task 2: アカウントを必須にし、`.gitconfig` の経路を消す

spec §5.1〜5.3。

**Files:**
- Modify: `cmd/hedgehug/app.go`（入口の判定。`PersistentPreRun` → `PersistentPreRunE`、`beforeRun`、`noAccountCommands`）
- Modify: `cmd/hedgehug/auth.go:119`（`auth status` の文言）
- Modify: `internal/config/author.go`（`CommitAuthor` だけにする。`Author`・`readGitConfig`・`includePath` を消す）
- Delete: `internal/config/author_test.go`
- Modify: `internal/config/config.go`（`loadStores`: アカウントが 2 つ以上で `account` のない Store は読み込みのエラー）
- Modify: `internal/ops/session.go`（`UpdateCommit`・`SyncOptions`）、`internal/ops/storeops.go`（`StoreInit`・`StoreAdd`・`noAccountChoice`）
- Modify: `tools/devenv/main.go`（`export GIT_CONFIG_GLOBAL` と `.gitconfig` を消す。`addDevStore` の未使用の引数 `getenv` を消す。ファイル先頭の説明の「git identity」を消す）
- Modify: `go.mod`・`go.sum`（`go mod tidy`。`github.com/go-git/gcfg/v2` が要らなくなる）
- Test（新規）: `cmd/hedgehug/account_required_test.go`
- Test（直す）: `internal/config/config_test.go`、`internal/ops/{helpers,storeops,storeinit_remote,task,read,people,mention,session_more,undo,final_review,crossrefs,discover,conn}_test.go` と `CommitAuthor` を呼ぶテスト一式、`internal/server/{helpers,comments,admin_more,tasks_read}_test.go`、`cmd/hedgehug/{helpers,store_init,add,background,inbox,undo,watch,auth,comment,e2e_3a}_test.go`、`tools/devenv/main_test.go`

**Interfaces:**
- Consumes: Task 1 の `defaultAccountYAML`・`dropDefaultAccount`・`testAccount`・`appendConfig`（各パッケージ）、`ops.Session.PickAccount`・`choiceRequired`・`errNoAccounts`（`internal/ops/task.go:283`、`KindConflict`・`account.none`）
- Produces:
  - `config.CommitAuthor(acct Account) gitrepo.Signature`（Task 3 の `auth add` のテストと、`tools/devenv` が使う）
  - 入口の許可リスト `noAccountCommands`（`hedgehug auth login`・`hedgehug auth add`・`hedgehug auth status`・`hedgehug help`・`hedgehug merge-driver`）。Task 3 は `auth add` のコマンドを足すだけでよい
  - 入口の文言（終了コード 1、標準エラーに）: `hedgehug: no account is set up; add one with hedgehug auth add <name> --login <login> (or hedgehug auth login)`
  - `auth status` の文言（アカウントなし）: `No account is set up; add one with hedgehug auth add <name> --login <login> (or hedgehug auth login)`
  - config.yaml の読み込みのエラー: `<config.yaml のパス>:<行>: stores: "<Store 名>" has no account; with <N> accounts every store needs account: <name>`
  - `internal/ops` のテスト用: `(*testSession).nameAccountOfStores(name)`、`(*testSession).dropAccounts()`、`appendConfig` が 2 つ以上のアカウントを足すときに Store へアカウントを書く

- [ ] **Step 1: 失敗するテストを書く**

`cmd/hedgehug/account_required_test.go` を作る。

```go
package main

import (
	"strings"
	"testing"
)

const noAccountMessage = "hedgehug: no account is set up; add one with hedgehug auth add <name> --login <login> (or hedgehug auth login)\n"

// Without an account only the commands that set one up run (spec §5.1);
// every other one stops before it does anything.
func TestCommandsNeedAnAccount(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"ls"}, {"add", "x"}, {"show", "ME-1"}, {"store", "init", "me", "--prefix", "ME"}, {"store", "list"},
		{"sync"}, {"doctor"}, {"inbox"}, {"undo"}, {"ui"}, {"auth", "logout", "x"}, {"workspace", "ls"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			te := newTestEnv(t)
			te.dropDefaultAccount()
			code, out, errs := te.run(args...)
			if code != 1 || out != "" || errs != noAccountMessage {
				t.Errorf("exit %d, stdout %q, stderr %q; want exit 1 and %q", code, out, errs, noAccountMessage)
			}
		})
	}
}

func TestCommandsThatRunWithoutAnAccount(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"--help"}, {"help"}, {"help", "ls"}, {"ls", "--help"}, {"--version"}, {"auth", "status"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			te := newTestEnv(t)
			te.dropDefaultAccount()
			if code, _, errs := te.run(args...); code != 0 || strings.Contains(errs, "no account is set up") {
				t.Errorf("exit %d, stderr %q", code, errs)
			}
		})
	}
	// auth login and merge-driver get to their own checks.
	te := newTestEnv(t)
	te.dropDefaultAccount()
	if code, _, errs := te.run("auth", "login", "--with-token"); code != 1 || !strings.Contains(errs, "no token on stdin") {
		t.Errorf("auth login: exit %d, stderr %q", code, errs)
	}
	if code, _, errs := te.run("merge-driver"); code == 0 || strings.Contains(errs, "no account is set up") {
		t.Errorf("merge-driver: exit %d, stderr %q; want its own argument error", code, errs)
	}
}
```

（`hedgehug auth add` が許可リストにあることのテストは、コマンドを作る Task 3 に書く。この Task ではコマンドがなく、`auth add` は未知のサブコマンドになる。）

`cmd/hedgehug/auth_test.go` の `TestAuthStatusEmptyAndBare`（380 行目）の最初の確認を、文言まで固定したものに変える。

```go
	const want = "No account is set up; add one with hedgehug auth add <name> --login <login> (or hedgehug auth login)\n"
	if out := te.ok("auth", "status"); out != want {
		t.Errorf("status = %q, want %q", out, want)
	}
```

`internal/config/config_test.go` の末尾に足す（import に `"github.com/kukv/hedgehug/internal/gitrepo"` を足す）。

```go
func TestCommitAuthor(t *testing.T) {
	t.Parallel()
	got := CommitAuthor(Account{Name: "work", Host: "github.com", Login: "alice-w"})
	if want := (gitrepo.Signature{Name: "alice-w", Email: "alice-w@users.noreply.github.com"}); got != want {
		t.Errorf("CommitAuthor = %+v, want %+v", got, want)
	}
}

// A store with no account uses the only account; with two or more it does
// not load (spec §5.2).
func TestLoadStoreWithoutAccount(t *testing.T) {
	t.Parallel()
	one := "accounts:\n  - {name: a, host: github.com, login: a}\nstores:\n  - {name: s, path: /s}\n"
	c, err := Load(cfgPaths(t, one))
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := c.Writer(c.Stores[0]); !ok || a.Name != "a" {
		t.Errorf("one account: Writer = %+v, %v", a, ok)
	}
	two := "accounts:\n  - {name: a, host: github.com, login: a}\n  - {name: b, host: github.com, login: b}\nstores:\n  - {name: s, path: /s}\n  - {name: t, path: /t, account: b}\n"
	p := cfgPaths(t, two)
	_, err = Load(p)
	if err == nil || !strings.Contains(err.Error(), p.ConfigFile) || !strings.Contains(err.Error(), `stores: "s" has no account`) {
		t.Errorf("Load = %v; want the file and the store s", err)
	}
}
```

`internal/ops/storeops_test.go`（`TestStoreAddClonesToAPath` の次）に足す。

```go
// With two accounts a store cannot do without one: store add asks for it.
func TestStoreAddNeedsTheAccountWhenThereAreTwo(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	remote := addBareRemote(t, ts)
	if _, err := ts.Sync(false, true); err != nil {
		t.Fatal(err)
	}
	other := otherSession(t, ts, ts.Now())
	other.appendConfig("accounts:\n  - {name: a, host: github.com, login: alice}\n  - {name: b, host: github.com, login: bob}\n")
	_, err := other.StoreAdd("copy", []string{remote}, "", "")
	wantErr(t, err, KindUsage, "choice.required")
	if _, err := other.StoreAdd("copy", []string{remote}, "", "b"); err != nil {
		t.Fatal(err)
	}
	if list, _ := other.StoreList(); len(list) != 1 || list[0].Account != "github.com/bob" {
		t.Errorf("StoreList = %+v; want the account github.com/bob", list)
	}
}

func TestWriteWithoutAnAccountIsRefused(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.dropDefaultAccount()
	_, err := ts.Add(AddInput{Title: "x"})
	wantErr(t, err, KindConflict, "account.none")
}
```

Run: `go test ./cmd/hedgehug -run 'TestCommandsNeedAnAccount|TestCommandsThatRunWithoutAnAccount|TestAuthStatusEmptyAndBare' -count=1; go test ./internal/config -run 'TestCommitAuthor|TestLoadStoreWithoutAccount' -count=1; go test ./internal/ops -run 'TestStoreAddNeedsTheAccountWhenThereAreTwo|TestWriteWithoutAnAccountIsRefused' -count=1`（timeout 600000）
Expected: FAIL。`TestCommandsNeedAnAccount` は終了コード 0（まだ止まらない）、`TestCommitAuthor` は引数の数でビルドできない（`internal/config`）、`TestStoreAddNeedsTheAccountWhenThereAreTwo` はエラーが返らない、`TestWriteWithoutAnAccountIsRefused` は `.gitconfig` の作者で書けてしまう。

- [ ] **Step 2: `internal/config` を直す**

`internal/config/author.go` を丸ごと置き換える（`Author`・`maxIncludeDepth`・`readGitConfig`・`includePath` を消す。`goos` は `paths.go` のものなので触らない）。

```go
package config

import "github.com/kukv/hedgehug/internal/gitrepo"

// CommitAuthor returns the author of the commits of a store: the login of the
// account that writes it, with the noreply address of its host.
func CommitAuthor(acct Account) gitrepo.Signature {
	return gitrepo.Signature{Name: acct.Login, Email: acct.Login + "@users.noreply." + acct.Host}
}
```

`internal/config/author_test.go` を消す（`TestAuthor`・`TestCommitAuthorFromAccount`・`TestCommitAuthorWithoutAccountReadsGitConfig`。`writeFile` は `config_test.go` など他のテストが使っていないことを `grep -rn 'writeFile(' internal/config` で確かめる）。

`internal/config/config.go` の `loadStores`、`e.Account = a.Value` を含む `if` の閉じかっこの次、`c.Stores = append(c.Stores, e)` の前に足す。

```go
		if e.Account == "" && len(c.Accounts) > 1 {
			return c.errAt(it, fmt.Sprintf("stores: %q has no account; with %d accounts every store needs account: <name>", e.Name, len(c.Accounts)))
		}
```

`config_test.go` の既存の 4 つが、この規則で落ちる。直し方:
- `TestSetStoreAccount`（128 行目）: YAML の Store `a` に `    account: personal` を足す（`b` と同じ）
- `TestLoadPhase3Keys`（631 行目で落ちる）: 3 つのアカウントのある YAML の Store `me` に `account: <最初のアカウントの name>` を足す
- `TestWriterRules`（448 行目）: 最初の `two` を読む部分を、`Load` のエラーの確認に置き換える（`TestLoadStoreWithoutAccount` と同じ）。「アカウントが 2 つで `stores[].account` なし → writer なし」は、`c := &Config{Accounts: []Account{{Name: "a", Host: "github.com", Login: "a"}, {Name: "b", Host: "github.com", Login: "b"}}}` を作って `c.Writer(StoreEntry{Name: "s"})` が ok でないことで確かめる。`named account` の確認（`account: b` の Store）は残す
- `TestNullAccountIsNoAccount`（531 行目）: アカウントを 1 つにする（`account: null` の Store がそのアカウントを使うことは `Account == ""` のまま確かめられる）

- [ ] **Step 3: `internal/ops` を直す**

`internal/ops/session.go`、`UpdateCommit`。`config.CommitAuthor(acct, ok, ss.Getenv)` から `ok` の分岐までを置き換える。

```go
	acct, ok, err := ss.Writer()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if !ok {
		return plumbing.ZeroHash, errNoAccounts
	}
	op.Author = config.CommitAuthor(acct)
	op.Account = acct.ID()
```

doc コメントの「The author is the store's account, or the git config when the store has none.」を「The author is the store's account.」にする。

`SyncOptions`（`session.go` の 526 行目〜）。

```go
	acct, ok := c.Writer(e)
	if !ok {
		return sync.Options{}, errNoAccounts
	}
	opt := sync.Options{Name: e.Name, Remotes: e.Remotes, Author: config.CommitAuthor(acct), Account: acct.ID(), Me: c.Me(), Now: ss.Now, Conn: ss.Conn(c, e), GitHub: ss.githubRepoOf(c, e)}
```

そのすぐ下の `if ok { opt.Account = acct.ID() }` は消す。

`internal/ops/storeops.go`。`noAccountFor` の前に足す。

```go
// noAccountChoice is the error for a store made with no account named while
// c has two or more: the choice among them. With none at all it is
// errNoAccounts.
func noAccountChoice(c *config.Config) error {
	if len(c.Accounts) == 0 {
		return errNoAccounts
	}
	names := make([]string, len(c.Accounts))
	for i, a := range c.Accounts {
		names[i] = a.Name
	}
	return choiceRequired("--account", names)
}
```

`StoreInit`（74〜100 行目）。コメントと `CommitAuthor` の部分を変え、`prepareRemotes` の次で、アカウントが決まらなかったときに止める。GitHub のホストのリモートがあるときの今の案内（`noAccountFor`、そのホストのアカウントだけを選択肢にする）は `prepareRemotes` の中で先に働くので、そのまま残る。

```go
	// Account, or the only account. With neither, a remote on a GitHub host
	// fails in prepareRemotes; any other store is stopped after it.
	acct, haveAcct, err := ss.PickAccount(c, in.Account)
	if err != nil {
		return StoreInitResult{}, err
	}
	dir := filepath.Join(p.StoresDir, name)
	…（in.Path と e の作りは今のまま）
	made, err := ss.prepareRemotes(c, e, acct, haveAcct)
	if err != nil {
		// made holds what was made before the failure, GitHub repos too.
		return StoreInitResult{}, made.note(err)
	}
	if !haveAcct {
		// No remote asked for an account; a store needs one all the same.
		return StoreInitResult{}, made.note(errors.Join(noAccountChoice(c), made.undo()))
	}
	author, accountID := config.CommitAuthor(acct), acct.ID()
```

`StoreAdd`（`e := config.StoreEntry{Name: name, Remotes: remotes}` からの `if account != ""` のブロック）を置き換える。

```go
	e := config.StoreEntry{Name: name, Remotes: remotes}
	acct, ok, err := ss.PickAccount(c, account)
	if err != nil {
		return StoreAddResult{}, err
	}
	if !ok {
		return StoreAddResult{}, noAccountChoice(c)
	}
	if account != "" {
		e.Account = acct.Name
	}
```

関数の doc コメントの「(none is set when empty, and the only account writes)」を「(the only account when empty; with two or more it is needed)」に直す。

- [ ] **Step 4: `cmd/hedgehug` の入口を作る**

`cmd/hedgehug/app.go`。`PersistentPreRun: a.beforeCommand,` を置き換える。

```go
		// Every subcommand uses this one: none has a hook of its own.
		PersistentPreRunE: a.beforeRun,
```

`beforeCommand` の前に足す。

```go
// noAccountCommands are the commands that run when no account is set up
// (spec §5.1): the ones that set one up, and merge-driver, which git calls in
// the middle of a sync. --help and --version never reach beforeRun.
var noAccountCommands = map[string]bool{
	"hedgehug auth login":   true,
	"hedgehug auth add":     true,
	"hedgehug auth status":  true,
	"hedgehug help":         true,
	"hedgehug merge-driver": true,
}

// errNoAccountSetUp is what a command that needs an account answers when
// there is none.
var errNoAccountSetUp = errors.New("no account is set up; add one with hedgehug auth add <name> --login <login> (or hedgehug auth login)")

// beforeRun is the hook of every command: it stops a command that needs an
// account when none is set up, then runs beforeCommand.
func (a *app) beforeRun(cmd *cobra.Command, args []string) error {
	if !noAccountCommands[cmd.CommandPath()] {
		c, _, err := a.ss.Config()
		if err != nil {
			return err
		}
		if len(c.Accounts) == 0 {
			return errNoAccountSetUp
		}
	}
	a.beforeCommand(cmd, args)
	return nil
}
```

`cmd/hedgehug/auth.go:119` を置き換える。

```go
		_, perr := fmt.Fprintln(a.stdout, "No account is set up; add one with hedgehug auth add <name> --login <login> (or hedgehug auth login)")
```

- [ ] **Step 5: `tools/devenv` を直す**

`tools/devenv/main.go`:
- 先頭の説明（2 行目）の「each with their own HEDGEHUG_HOME, git identity, accounts and」から「git identity, 」を消す
- `initUser`（292 行目〜）: 説明を `// initUser makes the home of the user u: config.yaml and the stores.` にし、`gitconfig := …` から `os.WriteFile(filepath.Join(home, ".gitconfig"), …)` の `if` までを消す
- `printEnv`（476 行目）: `fmt.Fprintf(&b, "export HEDGEHUG_HOME=%s\n", shellQuote(home))` にする（`GIT_CONFIG_GLOBAL` を消す）
- `addDevStore`（385・413 行目の `config.CommitAuthor(acct, true, getenv)` を、それぞれ `author := config.CommitAuthor(acct)` に（`if err != nil { return err }` も消す）。`addDevStore` はもう `getenv` を使わないので、引数を消す: `func addDevStore(p config.Paths, ds devStore) error`、呼び出しは `addDevStore(p, ds)`（`initUser` の 1 か所）。`addWebProject` は `Session{Getenv: getenv}` に使うので残す

`tools/devenv/main_test.go`:
- `TestInitCreatesUsers`（38〜46 行目）: `.gitconfig` を読む部分を、ないことの確認に置き換える

```go
		if _, err := os.Stat(filepath.Join(home, ".gitconfig")); err == nil {
			t.Errorf("%s has a .gitconfig; the author is the account's login", u)
		}
```
- `TestEnv`（208 行目）: `want` から `export GIT_CONFIG_GLOBAL=…` の行を消す

```go
	want := "export HEDGEHUG_HOME='" + home + "'\n" +
		"export HH_DEV_TOKEN='dev-alice'\nexport HH_DEV_TOKEN_WORK='dev-alice-corp'\n"
```
- `TestInitPhase3a`（311〜314 行目）: `alice's .gitconfig` の `insteadOf` の確認を消す

- [ ] **Step 6: テストを直す**

`internal/config` と `tools/devenv` は Step 2・5 で済んでいる。残りを、パッケージごとに `go test` を回して落ちるテストを直す。次の表は、Task 1 の状態に Step 2〜5 を入れて実際に回した結果。

**共通の機械的な変更（コンパイルが通らなくなる）。** `config.CommitAuthor(…)` を呼ぶテストを新しい署名に合わせる。`grep -rn 'config.CommitAuthor' --include='*_test.go' .` で洗い出した 20 か所（`internal/ops`: `duplicates_test.go:37`・`maintain_test.go:39`・`move_test.go:113`・`read_test.go:206`・`crossrefs_test.go:53,178`・`delete_test.go:348`・`inbox_test.go:484,601`・`storeops_test.go:200,293`・`helpers_test.go:94,242`・`rewrite_history_only_test.go:174`、`internal/server`: `admin_more_test.go:70`・`tasks_read_test.go:216`）。形は 3 つ。

```go
// 前（3 行のエラー処理つき）
author, err := config.CommitAuthor(config.Account{}, false, ts.Getenv)
if err != nil {
	t.Fatal(err)
}
// 後
author := config.CommitAuthor(testAccount)
```

`acct, true` で呼んでいるもの（`helpers_test.go:242`・`rewrite_history_only_test.go:174`）は `config.CommitAuthor(acct)`、`author, _ := config.CommitAuthor(config.Account{}, false, ts.Getenv)`（`storeops_test.go`）は `author := config.CommitAuthor(testAccount)`。`internal/server` には `testAccount` がないので、`helpers_test.go` に `var testAccount = config.Account{Name: "alice", Host: "github.com", Login: "alice"}` を足し、import に `internal/config` を足す。`newTestSession` の `config.CommitAuthor(testAccount, true, ts.Getenv)`（Task 1 で書いたもの）も `config.CommitAuthor(testAccount)` にする。

**`internal/ops` のヘルパー。** アカウントを 2 つ以上足すテストでは、Store `me` に `account` がないと読み込めない。`appendConfig` が、足すテキストに 2 つ以上のアカウント（`login:` が 2 回以上）があるときに、`account` のない Store へ最初のアカウントの名前を書く。`helpers_test.go` に足す（import に `"go.yaml.in/yaml/v4"` を足す）。

```go
// nameAccountOfStores gives the stores that have no account the account name,
// the first of the accounts that replaced the default one: with two or more
// accounts a store without one does not load.
func (ts *testSession) nameAccountOfStores(name string) {
	ts.t.Helper()
	p := filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml")
	b, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil {
		ts.t.Fatal(err)
	}
	var doc yaml.Node
	if err := yaml.Load(b, &doc); err != nil {
		ts.t.Fatal(err)
	}
	root := doc.Content[0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "stores" {
			continue
		}
		for _, st := range root.Content[i+1].Content {
			named := false
			for j := 0; j+1 < len(st.Content); j += 2 {
				named = named || st.Content[j].Value == "account"
			}
			if !named {
				st.Content = append(st.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "account"},
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name})
			}
		}
	}
	out, err := yaml.Dump(&doc, yaml.WithLineWidth(-1), yaml.WithIndent(2))
	if err != nil {
		ts.t.Fatal(err)
	}
	if err := os.WriteFile(p, out, 0o600); err != nil {
		ts.t.Fatal(err)
	}
	ts.resetWriter()
}

// firstAccountName is the name: of the first account in the accounts: text.
func firstAccountName(text string) string {
	m := regexp.MustCompile(`name:\s*"?([\w-]+)`).FindStringSubmatch(text[strings.Index(text, "accounts:"):])
	if m == nil {
		return ""
	}
	return m[1]
}

// dropAccounts removes the accounts: section from config.yaml.
func (ts *testSession) dropAccounts() {
	ts.t.Helper()
	p := filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml")
	b, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil {
		ts.t.Fatal(err)
	}
	text, _, _ := strings.Cut("\n"+string(b), "\naccounts:")
	text = strings.TrimPrefix(text, "\n")
	if text != "" {
		text += "\n"
	}
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		ts.t.Fatal(err)
	}
	ts.resetWriter()
}
```

`appendConfig` の `accounts:` の分岐を変える。

```go
	if strings.Contains(text, "accounts:") {
		ts.dropDefaultAccount()
		if strings.Count(text, "login:") > 1 {
			defer ts.nameAccountOfStores(firstAccountName(text))
		}
	}
```

**`internal/ops`（試走で 14。Task 1 の B にしたテストは落ちない）**

| テスト | 場所 | 直し方 |
| --- | --- | --- |
| `TestStoreInitNoAccountPicked` | `storeinit_remote_test.go:98` | 2 つのアカウントで `--account` なしの `StoreInit` は拒否になる。名前を「…IsRefused」に変え、`wantErr(t, err, KindUsage, "choice.required")` と、`namedStores(t, ts, "x")` が空であることを確かめ、そのあと `Account: "b"` を付けると成功することを確かめる |
| `TestWatchAddsAndRemovesMe` | `task_test.go:255` | アカウントなしの `Watch` の確認は、先にアカウントのあるうちに `ts.Add` し、書き込んだ人が watcher になっているので `ts.Watch("ME-1", false)` で外してから、`ts.dropDefaultAccount()` してエラーを確かめる。そのあとの `ts.useAccount()` 以降は今のまま通る |
| `TestCommentWithoutAuthor` | `mention_test.go:108` | 作者のないコメントは `CommentAdd` では書けなくなった。作者のないコメントのファイル（古い Store が持つ形）を直接書いて、編集・削除が `comment.no_author` になることを確かめる形に直す。`ts.dropDefaultAccount()` は呼ばない（下のコード） |
| `TestPickAccount` | `conn_test.go:262` | 2 つ目のアカウントを足す `appendConfig("  - {name: b, …}\n")`（`accounts:` なしの続き行）は、Store `me` にアカウントがなくなるので読み込めない。`ts.dropAccounts()` のあと、`a` と `b` を 1 回の `appendConfig("accounts:\n  - {name: a, …}\n  - {name: b, …}\n")` で足す |
| `TestWarnMayCallBackIntoTheSession`・`TestInboxSkipsAStoreThatDoesNotOpen`・`TestStoreNameKeepsTheWarningsOfOtherStoresQuiet`・`TestSyncDoesNotWarnTwiceOfAStoreItReports`（`final_review_test.go`）、`TestMuteOpenWarningsIsCountedPerCaller`・`TestConcurrentDoctorCallsDoNotWarnOfWhatTheyReport`・`TestDoctorKeepsTheOtherStoresWhenOneDoesNotOpen`（`crossrefs_test.go`） | `final_review_test.go:28`（`addGoneStore`）、`crossrefs_test.go:203,264,302` | 2 つのアカウントの config に `c.AddStore("gone", …)` で Store を足すところで、続けて `c.SetStoreAccount("gone", "alice")` を呼ぶ |
| `TestSyncWarnsOfCrossStoreRefs` | `crossrefs_test.go:342` | `c.RemoveStore("org-a")` → `c.AddStore("org-a", …)` で足し直すとアカウントが消える。`c.AddStore` のあとに `c.SetStoreAccount("org-a", "alice")` |

`TestCommentWithoutAuthor` の本体（`import` に `internal/model`・`internal/store` は `mention_test.go` に既にある）:

```go
func TestCommentWithoutAuthor(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	r, err := ts.Add(AddInput{Title: "a"})
	if err != nil {
		t.Fatal(err)
	}
	// A comment file with no author, as an older store may hold one.
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	cu := model.NewULID()
	if _, err := ts.Update(s, store.Op{Name: "comment", Task: r.ULID}, func(tx *store.Tx) error {
		return tx.PutComment(r.ULID, &model.Comment{ULID: cu, Body: "nobody\n"})
	}); err != nil {
		t.Fatal(err)
	}
	_, err = ts.CommentEdit(cu.String(), "x")
	wantErr(t, err, KindConflict, "comment.no_author")
	_, err = ts.CommentRemove(cu.String())
	wantErr(t, err, KindConflict, "comment.no_author")
}
```

`TestStoreInitChoiceIsOfTheRemotesHost`・`TestStoreInitNoAccountOfTheRemotesHost`・`TestStoreInitNoAccountAtAll` は、GitHub のホストのリモートがあるときの案内（そのホストのアカウントだけを選ぶ、なければ `account.none`）が `prepareRemotes` の中で先に働くので、直さずに通る。

**`internal/server`（試走で 1）**

| テスト | 場所 | 直し方 |
| --- | --- | --- |
| `TestWatchMeWithoutAccountIsConflict` | `comments_test.go:74` | `id := ts.add("one", "")` を先に呼び、そのあとで `ts.dropDefaultAccount()` |

**`cmd/hedgehug`**

| テスト | 直し方 |
| --- | --- |
| `TestInboxWithoutAccounts`（`inbox_test.go:208`）、`TestUndoWithoutAccounts`（`undo_test.go:93`）、`TestNoWatcherWithoutAccounts`（`watch_test.go:87`） | 消す。アカウントなしでは `store init` が入口で止まる（`TestCommandsNeedAnAccount` が代わりに確かめる）。`cmd/hedgehug/testdata/TestInboxWithoutAccounts/`・`TestUndoWithoutAccounts/` のディレクトリも消す |
| `TestStoreInitNoAuthor`（`store_init_test.go:124`）、`TestAddWithoutAccountUsesGitConfig`（`add_test.go:186`） | 消す（`.gitconfig` の経路がなくなった） |
| `TestSyncBackgroundRecordsAFailureBeforeTheRun`（`background_test.go:90`） | 消す。アカウントがないと `sync` が入口で止まり、SyncOptions が失敗する経路は CLI から届かなくなった。「記録された失敗を次のコマンドが知らせる」確認は `background_test.go:142` が残る |
| `TestAuthLoginEmptyStdin`・`TestAuthStatusEmptyAndBare`（`auth_test.go`） | 入口を通るので、Task 1 の `dropDefaultAccount()` のまま通る |
| `TestE2EPhase3a`（`e2e_3a_test.go:182`） | `carol`（`HEDGEHUG_HOME` が空のディレクトリ）に `store discover` を実行する前に、アカウントを書く: `os.WriteFile(filepath.Join(carol, "config.yaml"), []byte("accounts:\n  - {name: carol, host: github.com, login: carol}\n"), 0o600)`（Task 3 で `hedgehug auth add` に置き換えられる） |
| `store_init_test.go:127`・`background_test.go:97`・`helpers_test.go:56` | `.gitconfig` を書く・消す行を消す。`testEnv` の説明の「an author in HOME/.gitconfig」も消す。`merge_driver_test.go:22`・`merge_driver_more_test.go:101` の `GIT_CONFIG_GLOBAL=os.DevNull` は git 自身の署名設定を避けるためのもので、hedgehug は読まない。残す |

Run: `go vet ./... && go test ./cmd/hedgehug ./internal/config ./internal/ops ./internal/server ./tools/devenv -count=1`（timeout 600000）
Expected: すべて `ok`。表にないテストが落ちたら、理由を確かめて直す（アカウントなしで書いている、Store に `account` がない、のどちらか）。

- [ ] **Step 7: 参照の消し残しと `go mod tidy`**

Run: `grep -rn 'GIT_CONFIG_GLOBAL\|\.gitconfig\|config\.Author\|readGitConfig' --include='*.go' --include='Makefile' --include='*.toml' --include='*.md' . | grep -v '^./docs/' | grep -v '^./web/node_modules'`
Expected: `merge_driver_test.go`・`merge_driver_more_test.go` の `GIT_CONFIG_GLOBAL=os.DevNull` と、`e2e_test.go` が環境変数を除く `GIT_CONFIG_` の前方一致だけ。ほかが出たら消す。

Run: `go mod tidy && git diff --stat go.mod go.sum`
Expected: `go.mod` から `github.com/go-git/gcfg/v2` が消える（`author.go` だけが使っていた）。ほかの行が動いたら止めて報告する。

Run: `go build ./... && go vet ./... && go test ./cmd/hedgehug ./internal/config ./internal/ops ./internal/server ./internal/sync ./internal/mount ./tools/devenv -count=1`（timeout 600000）
Expected: すべて `ok`。

- [ ] **Step 8: コミット**

```bash
git add -A cmd internal tools go.mod go.sum
git commit -m "feat(cli): アカウントを必須にし、コミットの作者をアカウントだけから作る

アカウントが 1 つもないときは、auth login・auth add・auth status・help・
--version・merge-driver 以外のコマンドを止める。Store のアカウントは、
アカウントが 1 つならそれを使い、2 つ以上で account のない Store は
config.yaml の読み込みのエラーにする。コミットの作者は常に Store の
アカウントから作り、.gitconfig を読む config.Author と devenv の
GIT_CONFIG_GLOBAL・.gitconfig を消す。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

#### spec にない細部（Task 2）

1. **`auth status` の文言（アカウントなし）。** spec は「`auth add` も案内する文に変える」とだけ書く。入口の文言（`no account is set up; add one with …`）とそろえて `No account is set up; add one with hedgehug auth add <name> --login <login> (or hedgehug auth login)` にした。
2. **入口の判定の単位。** `cmd.CommandPath()` が許可リストにあるかで決める。サブコマンドを持つ親（`hedgehug auth`、`hedgehug store` だけを打った場合）は許可リストにないので、アカウントがなければ使い方の誤りの前に入口で止まる。config.yaml が読めないときは、その読み込みのエラーをそのまま返す（別のエラーを重ねない）。
3. **アカウントが 2 つ以上で `--account` がないときの `store init`・`store add`。** spec §5.2 は「`--account` の既定（アカウントが 1 つならそれ）は今と同じ」とするが、今は 2 つ以上で GitHub のホストのリモートがなければ「アカウントのない Store」を作る。その Store は読み込めなくなるので、`choice.required`（使い方の誤り、終了コード 2、選択肢はアカウントの名前）で拒否する。`store init` は GitHub のホストのリモートがあれば、これまでどおりそのホストのアカウントだけを選択肢にする（`noAccountFor`）。端末なら `store init`・`store discover` が今も番号で聞く。`store add` には端末で聞く処理がないので、エラーで `--account` を案内する。
4. **ops の「アカウントなし」の分岐は残す。** `Session.Writer()` の `ok`、`watch`・`undo` の `account.none`、`WriterHost`・`AccountID` の空の値、`ops.NoAccountsWarning` は、CLI の入口が止めるので CLI からは届かない。ただし `internal/ops` は単独の Session でも使えるライブラリで、消すと `Writer()` の戻り値が全呼び出しに波及するので、そのままにする。書き込み（`UpdateCommit`）と同期の準備（`SyncOptions`）は、アカウントがなければ `errNoAccounts`（`account.none`、文言は `no account is set up; add one with hedgehug auth add <name> --login <login> (or hedgehug auth login)`）で拒否する（`.gitconfig` へ落とさない）。
5. **`config.CommitAuthor` の署名。** `(acct Account) gitrepo.Signature`（エラーも `getenv` も取らない）にした。作者を作る元が `acct` だけになり、失敗する経路がないため。
6. **`tools/devenv` の `.gitconfig`。** 作者だけでなく `insteadOf = https://github.com/`（素の `git` が devenv の remotes を向く設定）も消える。hedgehug は `config.yaml` の `url_rewrites` で同じ先を向くので、hedgehug の動きは変わらない。devenv の home で素の `git clone https://github.com/…` を打つと本物の GitHub へ行く。
7. **`TestSyncBackgroundRecordsAFailureBeforeTheRun` は消す。** アカウントがないと `sync` が入口で止まる。ほかの理由でバックグラウンドの同期が止まる場合は `background_test.go:142`（`setSyncError`）が確かめている。
8. **e2e の `carol`。** この Task では config.yaml を直接書く。`hedgehug auth add` ができる Task 3 で置き換える。
9. **Playwright の試験環境（フェーズ4d-3、PR #31）。** このブランチにはまだなく、`GIT_CONFIG_GLOBAL` と `.gitconfig` を書く試験環境が 4d-3 の plan にある。このブランチと main を取り込む側で、`HEDGEHUG_HOME` にアカウントを書く形に直す（Task 13 の通しの前に確認する）。

### Task 3: `hh auth add` と、2 つ目のアカウントを足すときの `stores[].account` の書き込み

spec §5.4（と §5.1 の `auth status` の文言）。トークンを使わないアカウントを足す `hh auth add` を作り、アカウントが 1 つから 2 つになるとき、`stores[].account` のない Store に今のアカウントを書く（書かないと、spec §5.2 の読み込みのエラーになる）。`hh auth login` が新しいアカウントを足すときも同じ。書き込みは既存の `Config.Update`（`Session.updateConfig` 経由）の中で 1 回に行う。

Task 1・2 は実装済みとする（テストの config は既定でアカウントを書く。アカウントなしで動くコマンドに `auth add` が入っている）。この Task のテストは、Task 1 の既定に頼らないように、config.yaml を全部書き直すヘルパー（`setConfig`・`replaceConfigYAML`）を自前で持つ。

**Files:**
- Modify: `internal/ops/auth.go`（`AuthAddInput`・`AuthAddResult`・`Pinned`・`AuthAdd`・`addAccount`・`checkNewAccount`・`parseAddToken` を足す。`AuthLoginResult` に `Pinned`、`AuthLogin` の新規アカウントの書き込みを `addAccount` に）
- Modify: `cmd/hedgehug/auth.go`（`authAddCmd`・`authAdd`・`printPinned`。`authLogin` の出力に 1 行。`authStatus` のアカウントなしの文）
- Create: `internal/ops/auth_add_test.go`（ヘルパー `cfgStore`・`setConfig`・`storeAccounts`・`authFixture` と、`AuthAdd`・`AuthLogin` のテスト）
- Create: `cmd/hedgehug/auth_add_test.go`（ヘルパー `replaceConfigYAML`・`dropStoreAccounts`・`fakeHosts` と、CLI のテスト）

**Interfaces:**
- Consumes: `Session.updateConfig`（`internal/ops/storeops.go`）、`config.Config.AddAccount`・`SetStoreAccount`・`Stores`・`Accounts`、`config.SameAccount`、`newErr`・`KindUsage`・`KindConflict`、CLI の `usageErrorf`・`exactArgs`・`printJSON`、Task 1 の `newTestEnv`・`newTestSession`、Task 2 のアカウントなしで動くコマンドの一覧（`auth add` を含む）
- Produces:
  - `ops.AuthAddInput{Name, Login, Host, Token string}`、`ops.AuthAddResult{Name, ID, Token string; Pinned ops.Pinned}`（JSON は `{name, id, token}`、token は書いたときだけ）、`(*Session).AuthAdd(AuthAddInput) (AuthAddResult, error)`
  - `ops.Pinned{Account string; Stores []string}`: 足す前に書き込んだ `stores[].account`（`Account` は今までのアカウントの名前）。`AuthLoginResult.Pinned` も同じ（どちらも JSON には出さない）
  - エラー: 名前・login が空、`--token` の誤りは `KindUsage`（`auth.name`・`auth.login`・`auth.token_source`、終了コード 2）。名前か host+login の重複は `KindConflict`（`account.exists`、終了コード 1）
  - 文言: `Added account <name> (<host>/<login>)`、`Set account <今のアカウント> on stores <a>, <b>`（1 つなら `store`）、`Give it the token with: hedgehug auth login <name> --with-token`

- [ ] **Step 1: ops のテストを書く（ヘルパーと AuthAdd）**

`internal/ops/auth_add_test.go` を作る。

```go
package ops

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/fakegh"
)

// cfgStore is a store that setConfig writes into config.yaml; account is
// empty for a store with no account: of its own.
type cfgStore struct{ name, account string }

// setConfig replaces config.yaml with head (accounts:, hosts:, ...) and the
// stores. The store "me" is the one of the test session; any other is only an
// entry, with a directory that does not exist.
func (ts *testSession) setConfig(head string, stores ...cfgStore) {
	ts.t.Helper()
	c, p, err := ts.Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	me, err := c.Select("me")
	if err != nil {
		ts.t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString(head)
	b.WriteString("stores:\n")
	for _, s := range stores {
		path := me.Path
		if s.name != "me" {
			path = filepath.Join(ts.t.TempDir(), s.name)
		}
		fmt.Fprintf(&b, "  - {name: %s, path: %q", s.name, path)
		if s.account != "" {
			fmt.Fprintf(&b, ", account: %s", s.account)
		}
		b.WriteString("}\n")
	}
	if err := os.WriteFile(p.ConfigFile, []byte(b.String()), 0o600); err != nil {
		ts.t.Fatal(err)
	}
}

// storeAccounts is the account: of each store in config.yaml, by store name.
func (ts *testSession) storeAccounts() map[string]string {
	ts.t.Helper()
	c, _, err := ts.Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	m := map[string]string{}
	for _, e := range c.Stores {
		m[e.Name] = e.Account
	}
	return m
}

// configBytes is config.yaml as it is on disk.
func (ts *testSession) configBytes() []byte {
	ts.t.Helper()
	_, p, err := ts.Config()
	if err != nil {
		ts.t.Fatal(err)
	}
	b, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		ts.t.Fatal(err)
	}
	return b
}

// authFixture is authSession with config.yaml written whole: github.com on a
// fakegh, accounts, and the stores. The keychain is in memory.
func authFixture(t *testing.T, st fakegh.State, accounts string, stores ...cfgStore) (*testSession, *kr) {
	t.Helper()
	zero := 0
	if st.DeviceInterval == nil {
		st.DeviceInterval = &zero
	}
	if st.Users == nil {
		st.Users = []string{"alice", "bob"}
	}
	srv := httptest.NewServer(fakegh.New(st))
	t.Cleanup(srv.Close)
	ts := newTestSession(t)
	k := newKR()
	ts.Keyring = k
	ts.setConfig("hosts:\n  - {host: github.com, api: \""+srv.URL+"\", web: \""+srv.URL+"\", client_id: Iv23test}\n"+accounts, stores...)
	return ts, k
}

const aliceAccount = "accounts:\n  - {name: personal, host: github.com, login: alice}\n"

func TestAuthAddFirstAccount(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.setConfig("", cfgStore{"me", ""})
	res, err := ts.AuthAdd(AuthAddInput{Name: "personal", Login: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "personal" || res.ID != "github.com/alice" || res.Token != "" || len(res.Pinned.Stores) != 0 {
		t.Errorf("result = %+v", res)
	}
	as := ts.accounts()
	if len(as) != 1 || as[0].Host != "github.com" || as[0].Login != "alice" || as[0].Token.Kind != "" {
		t.Errorf("accounts = %+v", as)
	}
	if strings.Contains(string(ts.configBytes()), "token") {
		t.Errorf("config.yaml = %s; want no token:", ts.configBytes())
	}
	// There was no account to keep for the store, so it is left as it is.
	if got := ts.storeAccounts(); got["me"] != "" {
		t.Errorf("store accounts = %v", got)
	}
}

// The token is not fetched: neither gh nor the keychain is asked, and the
// keychain is not written.
func TestAuthAddTokenSources(t *testing.T) {
	t.Parallel()
	for token, want := range map[string]string{"gh": "gh", "env:HH_TOKEN": "env:HH_TOKEN", "keychain": "keychain"} {
		t.Run(token, func(t *testing.T) {
			t.Parallel()
			ts := newTestSession(t)
			k := newKR()
			ts.Keyring = k
			ts.Run = func(name string, _ ...string) ([]byte, []byte, error) {
				t.Errorf("ran %s", name)
				return nil, nil, errors.New("must not run")
			}
			ts.setConfig("", cfgStore{"me", ""})
			res, err := ts.AuthAdd(AuthAddInput{Name: "work", Login: "bob", Host: " GitHub.com ", Token: " " + token + " "})
			if err != nil {
				t.Fatal(err)
			}
			if res.Token != want || res.ID != "GitHub.com/bob" {
				t.Errorf("result = %+v", res)
			}
			as := ts.accounts()
			if len(as) != 1 || as[0].Token.String() != want || as[0].Host != "GitHub.com" {
				t.Errorf("accounts = %+v", as)
			}
			if len(k.m) != 0 {
				t.Errorf("keychain = %v; want nothing written", k.m)
			}
		})
	}
}

// A second account would leave the stores with no account: to a reader of
// config.yaml, so they are given the one they used, in the same write.
func TestAuthAddPinsTheStoresOfTheOnlyAccount(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.setConfig(aliceAccount, cfgStore{"me", ""}, cfgStore{"org-a", ""}, cfgStore{"org-b", "personal"})
	res, err := ts.AuthAdd(AuthAddInput{Name: "work", Login: "bob", Token: "gh"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Pinned.Account != "personal" || !slices.Equal(res.Pinned.Stores, []string{"me", "org-a"}) {
		t.Errorf("Pinned = %+v; want personal on me and org-a (org-b had its own)", res.Pinned)
	}
	want := map[string]string{"me": "personal", "org-a": "personal", "org-b": "personal"}
	if got := ts.storeAccounts(); !maps.Equal(got, want) {
		t.Errorf("store accounts = %v, want %v", got, want)
	}
	if as := ts.accounts(); len(as) != 2 || as[1].Name != "work" {
		t.Errorf("accounts = %+v", as)
	}
	// A third account finds every store with an account already.
	res, err = ts.AuthAdd(AuthAddInput{Name: "extra", Login: "carol"})
	if err != nil || len(res.Pinned.Stores) != 0 {
		t.Errorf("third account: %+v, %v; want nothing pinned", res, err)
	}
}

func TestAuthAddRefusals(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.setConfig(aliceAccount, cfgStore{"me", ""})
	before := ts.configBytes()
	for _, c := range []struct {
		name string
		in   AuthAddInput
		kind Kind
		code string
	}{
		{"same name", AuthAddInput{Name: "personal", Login: "bob"}, KindConflict, "account.exists"},
		{"same host and login, other case", AuthAddInput{Name: "other", Login: "ALICE", Host: "GitHub.com"}, KindConflict, "account.exists"},
		{"no name", AuthAddInput{Name: " ", Login: "bob"}, KindUsage, "auth.name"},
		{"no login", AuthAddInput{Name: "work"}, KindUsage, "auth.login"},
		{"device", AuthAddInput{Name: "work", Login: "bob", Token: "device"}, KindUsage, "auth.token_source"},
		{"empty env", AuthAddInput{Name: "work", Login: "bob", Token: "env:"}, KindUsage, "auth.token_source"},
		{"unknown", AuthAddInput{Name: "work", Login: "bob", Token: "nope"}, KindUsage, "auth.token_source"},
	} {
		_, err := ts.AuthAdd(c.in)
		t.Run(c.name, func(t *testing.T) { wantErr(t, err, c.kind, c.code) })
	}
	if after := ts.configBytes(); !bytes.Equal(before, after) {
		t.Errorf("config.yaml changed by refused adds:\n%s", after)
	}
}

// A login that adds an account gives the stores of the only account their
// account too; a login that only replaces a token does not.
func TestAuthLoginPinsTheStoresOfTheOnlyAccount(t *testing.T) {
	t.Parallel()
	ts, _ := authFixture(t, fakegh.State{Tokens: map[string]string{"t-bob": "bob"}}, aliceAccount,
		cfgStore{"me", ""}, cfgStore{"org-a", ""})
	res, err := ts.AuthLogin(AuthLoginInput{Name: "work", Host: "github.com", WithToken: true, Token: "t-bob"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Added || res.Pinned.Account != "personal" || !slices.Equal(res.Pinned.Stores, []string{"me", "org-a"}) {
		t.Errorf("result = %+v", res)
	}
	want := map[string]string{"me": "personal", "org-a": "personal"}
	if got := ts.storeAccounts(); !maps.Equal(got, want) {
		t.Errorf("store accounts = %v, want %v", got, want)
	}
	res, err = ts.AuthLogin(AuthLoginInput{Name: "work", Host: "github.com", WithToken: true, Token: "t-bob"})
	if err != nil || res.Added || len(res.Pinned.Stores) != 0 {
		t.Errorf("second login = %+v, %v; want the token replaced and nothing pinned", res, err)
	}
}
```

- [ ] **Step 2: ops のテストの失敗を確かめる**

Run: `go test ./internal/ops -run 'TestAuthAdd|TestAuthLoginPins' -count=1`（Bash の timeout は 600000）
Expected: FAIL（ビルドできない: `ts.AuthAdd undefined`・`AuthAddInput undefined`・`res.Pinned undefined`）

- [ ] **Step 3: ops を実装する**

`internal/ops/auth.go`。

(a) `AuthLoginResult` に `Pinned` を足す（変える前後）。

```go
// before
type AuthLoginResult struct {
	Account string `json:"account"`
	ID      string `json:"id"`
	Added   bool   `json:"added"`
	Uses    string `json:"-"`
}

// after
type AuthLoginResult struct {
	Account string `json:"account"`
	ID      string `json:"id"`
	Added   bool   `json:"added"`
	Uses    string `json:"-"`
	Pinned  Pinned `json:"-"` // when an account was added: the stores that were given the account they used
}
```

(b) `AuthLogin` の config の書き込み（`err = ss.updateConfig(c, func(c *config.Config) error {` から最後の `return AuthLoginResult{Account: acct.Name, ID: acct.ID(), Added: added}, nil` まで）の 3 か所を変える。

```go
// before
	err = ss.updateConfig(c, func(c *config.Config) error {
		if added {
			for _, x := range c.Accounts { // another writer may have added it since the read
				if config.SameAccount(x.ID(), acct.ID()) {
					return newErr(KindConflict, "account.exists", fmt.Errorf("account %s (%s) is set up already; run hedgehug auth login %s", x.Name, x.ID(), x.Name),
						"account", x.Name)
				}
			}
			return c.AddAccount(acct)
		}

// after
	var pinned Pinned
	err = ss.updateConfig(c, func(c *config.Config) error {
		if added {
			for _, x := range c.Accounts { // another writer may have added it since the read
				if config.SameAccount(x.ID(), acct.ID()) {
					return newErr(KindConflict, "account.exists", fmt.Errorf("account %s (%s) is set up already; run hedgehug auth login %s", x.Name, x.ID(), x.Name),
						"account", x.Name)
				}
			}
			p, err := addAccount(c, acct)
			pinned = p
			return err
		}
```

```go
// before
	return AuthLoginResult{Account: acct.Name, ID: acct.ID(), Added: added}, nil

// after
	return AuthLoginResult{Account: acct.Name, ID: acct.ID(), Added: added, Pinned: pinned}, nil
```

(c) ファイルの `AuthStatus` の前（`AuthLogin` の直後）に足す。

```go
// Pinned is what addAccount wrote before it added an account: Stores, the
// stores that had no account: of their own, were given Account, the only
// account there was.
type Pinned struct {
	Account string
	Stores  []string
}

// addAccount adds acct to c. A store with no account: uses the only account
// there is, and has none to use once a second one is added (config.yaml then
// fails to load), so the stores are given that account first.
func addAccount(c *config.Config, acct config.Account) (Pinned, error) {
	var pinned Pinned
	if len(c.Accounts) == 1 {
		pinned.Account = c.Accounts[0].Name
		for _, e := range c.Stores {
			if e.Account != "" {
				continue
			}
			if err := c.SetStoreAccount(e.Name, pinned.Account); err != nil {
				return Pinned{}, err
			}
			pinned.Stores = append(pinned.Stores, e.Name)
		}
	}
	return pinned, c.AddAccount(acct)
}

// AuthAddInput is what hh auth add is given. Host "" is github.com. Token is
// "gh", "env:VAR" or "keychain"; "" is an account that uses no token.
type AuthAddInput struct {
	Name  string
	Login string
	Host  string
	Token string
}

// AuthAddResult is the account AuthAdd wrote, as auth add --json prints it.
// Token is where its token comes from, left out for none.
type AuthAddResult struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	Token  string `json:"token,omitempty"`
	Pinned Pinned `json:"-"`
}

// AuthAdd adds an account to config.yaml. It gets no token (neither gh nor
// the keychain is asked) and asks GitHub nothing; auth status checks it. The
// stores that would be left with no account get it in the same write
// (addAccount).
func (ss *Session) AuthAdd(in AuthAddInput) (AuthAddResult, error) {
	name, login, host := strings.TrimSpace(in.Name), strings.TrimSpace(in.Login), strings.TrimSpace(in.Host)
	if name == "" {
		return AuthAddResult{}, newErr(KindUsage, "auth.name", errors.New("an account name is needed"))
	}
	if login == "" {
		return AuthAddResult{}, newErr(KindUsage, "auth.login", errors.New("the login of the account is needed"))
	}
	if host == "" {
		host = "github.com"
	}
	tok, err := parseAddToken(strings.TrimSpace(in.Token))
	if err != nil {
		return AuthAddResult{}, err
	}
	acct := config.Account{Name: name, Host: host, Login: login, Token: tok}
	c, _, err := ss.Config()
	if err != nil {
		return AuthAddResult{}, err
	}
	var pinned Pinned
	if err := ss.updateConfig(c, func(c *config.Config) error {
		if err := checkNewAccount(c, acct); err != nil {
			return err
		}
		p, err := addAccount(c, acct)
		pinned = p
		return err
	}); err != nil {
		return AuthAddResult{}, err
	}
	return AuthAddResult{Name: acct.Name, ID: acct.ID(), Token: tok.String(), Pinned: pinned}, nil
}

// parseAddToken reads --token of auth add: gh, env:VAR or keychain. device
// is left out: it comes only from a login.
func parseAddToken(s string) (config.TokenSource, error) {
	switch {
	case s == "":
		return config.TokenSource{}, nil
	case s == "gh" || s == "keychain":
		return config.TokenSource{Kind: s}, nil
	}
	if v, ok := strings.CutPrefix(s, "env:"); ok && v != "" {
		return config.TokenSource{Kind: "env", Env: v}, nil
	}
	return config.TokenSource{}, newErr(KindUsage, "auth.token_source", fmt.Errorf("--token %q: want gh, env:<VAR> or keychain", s), "token", s)
}

// checkNewAccount refuses an account whose name, or whose host and login
// (compared without case), an account of c has already: the rules config.yaml
// is loaded by.
func checkNewAccount(c *config.Config, acct config.Account) error {
	for _, x := range c.Accounts {
		switch {
		case x.Name == acct.Name:
			return newErr(KindConflict, "account.exists", fmt.Errorf("an account named %q is set up already (%s)", x.Name, x.ID()), "name", x.Name)
		case config.SameAccount(x.ID(), acct.ID()):
			return newErr(KindConflict, "account.exists", fmt.Errorf("account %s (%s) is set up already", x.Name, x.ID()), "name", x.Name)
		}
	}
	return nil
}
```

- [ ] **Step 4: ops のテストの成功を確かめる**

Run: `go test ./internal/ops -run 'TestAuthAdd|TestAuthLogin' -count=1`（timeout 600000）
Expected: PASS（既存の `TestAuthLogin*` も通る）

- [ ] **Step 5: CLI のテストを書く**

`cmd/hedgehug/auth_add_test.go` を作る。

```go
package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/kukv/hedgehug/internal/fakegh"
)

// replaceConfigYAML replaces config.yaml with s.
func (te *testEnv) replaceConfigYAML(s string) {
	te.t.Helper()
	if err := os.MkdirAll(te.hhHome, 0o750); err != nil {
		te.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(te.hhHome, "config.yaml"), []byte(s), 0o600); err != nil {
		te.t.Fatal(err)
	}
}

// dropStoreAccounts takes the account: line out of every store of
// config.yaml, as a config.yaml written by hand has it. The accounts are
// written in the flow style, so no line of them starts with account:.
func (te *testEnv) dropStoreAccounts() {
	te.t.Helper()
	var kept []string
	for _, line := range strings.SplitAfter(te.configText(), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "account: ") {
			kept = append(kept, line)
		}
	}
	te.replaceConfigYAML(strings.Join(kept, ""))
	if strings.Contains(te.configText(), "account:") {
		te.t.Fatalf("config.yaml still has an account: %s", te.configText())
	}
}

// fakeHosts starts a fakegh with alice and bob and returns the hosts: of a
// config.yaml that points github.com at it.
func (te *testEnv) fakeHosts(st fakegh.State) string {
	te.t.Helper()
	zero := 0
	if st.DeviceInterval == nil {
		st.DeviceInterval = &zero
	}
	if st.Users == nil {
		st.Users = []string{"alice", "bob"}
	}
	srv := httptest.NewServer(fakegh.New(st))
	te.t.Cleanup(srv.Close)
	return "hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\", client_id: Iv23test}\n"
}

// storeAccountIDs is the account of each store in store list, in the order of
// the list.
func (te *testEnv) storeAccountIDs() []string {
	te.t.Helper()
	var list []struct {
		Account string `json:"account"`
	}
	if err := json.Unmarshal([]byte(te.ok("store", "list", "--json")), &list); err != nil {
		te.t.Fatal(err)
	}
	ids := make([]string, len(list))
	for i, x := range list {
		ids[i] = x.Account
	}
	return ids
}

func TestAuthAdd(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.replaceConfigYAML("")
	if out := te.ok("auth", "add", "personal", "--login", "alice"); out != "Added account personal (github.com/alice)\n" {
		t.Errorf("stdout = %q", out)
	}
	if b := te.configText(); !strings.Contains(b, "name: personal") || !strings.Contains(b, "login: alice") || strings.Contains(b, "token:") {
		t.Errorf("config.yaml = %s", b)
	}
	if out := te.ok("auth", "status"); !strings.Contains(out, "personal") || !strings.Contains(out, "github.com/alice") {
		t.Errorf("status = %q", out)
	}
}

func TestAuthAddKeychainSaysHowToGiveTheToken(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.replaceConfigYAML("")
	out := te.ok("auth", "add", "work", "--login", "bob", "--host", "github.com", "--token", "keychain")
	if !strings.Contains(out, "Added account work (github.com/bob)\n") || !strings.Contains(out, "Give it the token with: hedgehug auth login work --with-token\n") {
		t.Errorf("stdout = %q", out)
	}
	if b := te.configText(); !strings.Contains(b, "token: keychain") {
		t.Errorf("config.yaml = %s", b)
	}
}

func TestAuthAddJSON(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.replaceConfigYAML("")
	var got struct {
		Name  string `json:"name"`
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	out := te.ok("auth", "add", "work", "--login", "bob", "--token", "env:HH_TOKEN", "--json")
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.Name != "work" || got.ID != "github.com/bob" || got.Token != "env:HH_TOKEN" {
		t.Errorf("add --json = %s, %v", out, err)
	}
	// token is written only when the account has one.
	if out := te.ok("auth", "add", "plain", "--login", "carol", "--json"); strings.Contains(out, "token") {
		t.Errorf("add --json without a token = %s", out)
	}
}

// The stores that had no account: use the only account; once there are two
// they would fail to load, so auth add writes the account to them.
func TestAuthAddSecondAccountSetsStoreAccounts(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.replaceConfigYAML("accounts:\n  - {name: personal, host: github.com, login: alice}\n")
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("store", "init", "org-a", "--prefix", "OA")
	te.dropStoreAccounts()

	out := te.ok("auth", "add", "work", "--login", "bob")
	if want := "Added account work (github.com/bob)\nSet account personal on stores me, org-a\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	if got := strings.Count(te.configText(), "account: personal"); got != 2 {
		t.Errorf("config.yaml has %d account: personal, want 2:\n%s", got, te.configText())
	}
	// config.yaml loads, and each store is still written by alice.
	if ids := te.storeAccountIDs(); len(ids) != 2 || ids[0] != "github.com/alice" || ids[1] != "github.com/alice" {
		t.Errorf("store accounts = %v", ids)
	}
	// One store is a "store", not "stores".
	te.replaceConfigYAML("accounts:\n  - {name: personal, host: github.com, login: alice}\n")
	te.ok("store", "init", "solo", "--prefix", "SO")
	te.dropStoreAccounts()
	if out := te.ok("auth", "add", "work", "--login", "bob"); !strings.Contains(out, "Set account personal on store solo\n") {
		t.Errorf("stdout = %q", out)
	}
}

func TestAuthAddErrors(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.replaceConfigYAML("accounts:\n  - {name: personal, host: github.com, login: alice}\n")
	before := te.configText()
	for _, c := range []struct {
		name string
		args []string
		code int
		msg  string
	}{
		{"no login", []string{"auth", "add", "work"}, 2, "--login is required"},
		{"no name", []string{"auth", "add", "--login", "bob"}, 2, ""},
		{"device", []string{"auth", "add", "work", "--login", "bob", "--token", "device"}, 2, "want gh, env:<VAR> or keychain"},
		{"same name", []string{"auth", "add", "personal", "--login", "bob"}, 1, "set up already"},
		{"same login", []string{"auth", "add", "other", "--login", "Alice"}, 1, "set up already"},
	} {
		code, _, errs := te.run(c.args...)
		if code != c.code || !strings.Contains(errs, c.msg) {
			t.Errorf("%s: exit %d, stderr %q; want exit %d and %q", c.name, code, errs, c.code, c.msg)
		}
	}
	if after := te.configText(); after != before {
		t.Errorf("config.yaml changed by refused adds:\n%s", after)
	}
}

// 直列: keyring.MockInit で go-keyring のモックを作り直す
func TestAuthLoginSecondAccountSetsStoreAccounts(t *testing.T) {
	keyring.MockInit()
	te := newTestEnv(t)
	te.replaceConfigYAML(te.fakeHosts(fakegh.State{Tokens: map[string]string{"t-bob": "bob"}}) +
		"accounts:\n  - {name: personal, host: github.com, login: alice}\n")
	te.ok("store", "init", "me", "--prefix", "ME")
	te.dropStoreAccounts()
	te.stdin = "t-bob\n"
	out := te.ok("auth", "login", "work", "--with-token")
	if want := "Logged in as github.com/bob (account work)\nSet account personal on store me\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	if ids := te.storeAccountIDs(); len(ids) != 1 || ids[0] != "github.com/alice" {
		t.Errorf("store accounts = %v", ids)
	}
}
```

- [ ] **Step 6: CLI のテストの失敗を確かめる**

Run: `go test ./cmd/hedgehug -run 'TestAuthAdd|TestAuthLoginSecondAccount' -count=1`（timeout 600000）
Expected: FAIL（`unknown command "add" for "hedgehug auth"` で終了コード 2。`TestAuthLoginSecondAccountSetsStoreAccounts` は出力の `Set account …` の行がなくて FAIL）

- [ ] **Step 7: CLI を実装する**

`cmd/hedgehug/auth.go`。

(a) `authCmd` の `login` を作る箇所の直前に `add` を作り、`cmd.AddCommand` に足す。

```go
// before
	cmd.AddCommand(login, status, logout)

// after
	cmd.AddCommand(a.authAddCmd(), login, status, logout)
```

(b) ファイルの `authCmd` の後ろに足す。

```go
func (a *app) authAddCmd() *cobra.Command {
	var login, host, token string
	cmd := &cobra.Command{
		Use:   "add <name> --login <login>",
		Short: "Add an account to config.yaml",
		Long: `Add writes an account to config.yaml. It gets no token and asks GitHub nothing; hedgehug auth status checks the account.

--token says where the token comes from: gh, env:<VAR> or keychain. Without it the account uses no token. For keychain, give the token afterwards with hedgehug auth login <name> --with-token.

When this is the second account, the stores that had no account: are given the first, so that they keep the account they used.`,
		Args: exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if login == "" {
				return usageErrorf("--login is required: the GitHub login of the account")
			}
			return a.authAdd(args[0], login, host, token)
		},
	}
	cmd.Flags().StringVar(&login, "login", "", "the GitHub login of the account")
	cmd.Flags().StringVar(&host, "host", "github.com", "the host of the account")
	cmd.Flags().StringVar(&token, "token", "", "where the token comes from: gh, env:<VAR> or keychain (default: none)")
	return cmd
}

func (a *app) authAdd(name, login, host, token string) error {
	res, err := a.ss.AuthAdd(ops.AuthAddInput{Name: name, Login: login, Host: host, Token: token})
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(res)
	}
	if _, err := fmt.Fprintf(a.stdout, "Added account %s (%s)\n", res.Name, res.ID); err != nil {
		return err
	}
	if err := a.printPinned(res.Pinned); err != nil {
		return err
	}
	if res.Token == "keychain" {
		_, err = fmt.Fprintf(a.stdout, "Give it the token with: hedgehug auth login %s --with-token\n", res.Name)
	}
	return err
}

// printPinned tells which stores were given an account when a second account
// was added; nothing when none was.
func (a *app) printPinned(p ops.Pinned) error {
	if len(p.Stores) == 0 {
		return nil
	}
	word := "stores"
	if len(p.Stores) == 1 {
		word = "store"
	}
	_, err := fmt.Fprintf(a.stdout, "Set account %s on %s %s\n", p.Account, word, strings.Join(p.Stores, ", "))
	return err
}
```

(c) `authLogin` の最後（変える前後）。

```go
// before
	_, err = fmt.Fprintf(a.stdout, "Logged in as %s (account %s)\n", res.ID, res.Account)
	return err
}

// after
	if _, err := fmt.Fprintf(a.stdout, "Logged in as %s (account %s)\n", res.ID, res.Account); err != nil {
		return err
	}
	return a.printPinned(res.Pinned)
}
```

（`authStatus` のアカウントなしの文は Task 2 で変え済み。）

- [ ] **Step 8: 成功を確かめて、コミットする**

Run: `go vet ./internal/ops ./cmd/hedgehug && go test ./internal/ops ./cmd/hedgehug -run 'TestAuth' -count=1`（timeout 600000）
Expected: PASS（`go vet` は指摘なし。既存の `TestAuth*` も通る）

```bash
git add internal/ops/auth.go internal/ops/auth_add_test.go cmd/hedgehug/auth.go cmd/hedgehug/auth_add_test.go
git commit -m "$(cat <<'EOF'
feat(cli): トークンなしのアカウントを足す hh auth add を作る

アカウントが1つから2つになるとき、account: のない Store に今のアカウントを同じ書き込みで書く。hh auth login が新しいアカウントを足すときも同じにした。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

#### spec にない細部（Task 3）

1. `--host` の既定は `github.com`（`auth login` と同じ。spec の書式が `[--host github.com]` なので）。
2. 誤りの種別: 名前・`--login` が空と `--token` の誤りは使い方の誤り（終了コード 2）、名前か host+login の重複は衝突（終了コード 1）。spec は「config.yaml の読み込みと同じ検査」とだけ書くので、`auth login` の重複（`account.exists`）に合わせた。
3. `--token` は `gh`・`env:VAR`・`keychain` だけ。`device` は spec の書式にないので拒否する（`device` は `auth login` のデバイスフローでしか作れない）。
4. name・login・host・`--token` の前後の空白は除く（config.yaml の読み込みと同じ扱い）。
5. `Set account … on …` の行は、Store が 1 つなら `store`、2 つ以上なら `stores`（spec の例は複数形だけ。Task 12 の `plural` は数を付けるので使わない）。
6. `--json` に書き込んだ Store は含めない（spec の `{name, id, token}` のまま。`auth login --json` も同じ）。
7. `keychain` の案内文言を `Give it the token with: hedgehug auth login <name> --with-token` にした。
8. 足す前に `stores[].account` を書くのは、足す前のアカウントがちょうど 1 つのときだけ（0 のときは使うアカウントがなく、2 つ以上のときは読み込みの時点で全 Store に account がある）。

---

### Task 4: client ID を組み込まない

spec §6。`builtinClientID` を消し、`ClientID(h)` は `hosts[].client_id` だけを返す。`client_id` が空のホストでは、デバイスフローを何もせずに止め、リフレッシュは通信の前に失敗させる。

調べた結果: Device Flow が `client_id` を使うのは `internal/auth/device.go` の 2 か所（`DeviceLogin` と、リフレッシュの `github.RefreshToken`）だけ。`tools/devenv` は `hosts[]` に `client_id` を書かず（`configHead`）、アカウントのトークンを `token: env:<VAR>` で渡すので、Device Flow を使わない。`internal/fakegh/server.go` は `client_id` が空でなければ何でも受ける（空は 400）。したがって devenv と fakegh は変えない。テストは、Device Flow を通すものがすべて `hosts[].client_id` を自分で書いている（`internal/auth/device_test.go` の `fakeHost`、`internal/ops/auth_test.go` の `authSession`、`cmd/hedgehug/auth_test.go` の `withFake`、`e2e_3a_test.go`）ので、直す必要があるのは `TestClientID` と、`client_id` がないときを確かめる 1 か所だけ。

**Files:**
- Modify: `internal/auth/source.go`（`builtinClientID` を消す。`ClientID`）
- Modify: `internal/auth/device.go`（`DeviceLogin` の文言、`deviceToken` のリフレッシュの前の確認）
- Modify: `internal/config/hosts.go:20`（`Host.ClientID` のコメント）
- Test: `internal/auth/source_test.go`（`TestClientID`）、`internal/auth/device_test.go`（`TestDeviceLogin` の `client_id` の確認、`TestDeviceRefreshNeedsClientID`）、`internal/ops/auth_add_test.go`（Task 3 で作ったファイルに `TestAuthLoginDeviceNeedsClientID`）、`cmd/hedgehug/auth_test.go`（`TestAuthLoginDeviceNeedsClientID`）

**Interfaces:**
- Consumes: `auth.ClientID`・`auth.DeviceLogin`・`auth.Token`、Task 3 の `setConfig`・`replaceConfigYAML`
- Produces: `auth.ClientID(h) == h.ClientID`。デバイスフローの `client_id` なしのエラー（終了コード 1）: `the device flow needs hosts[].client_id for <host> in config.yaml; or use token: gh, token: env:<VAR>, or hedgehug auth login <name> --with-token`（CLI では先頭に `hedgehug: `）。リフレッシュの `client_id` なしのエラー: `refreshing the token of <host>/<login> needs hosts[].client_id for <host> in config.yaml; run hedgehug auth login <name>`

- [ ] **Step 1: テストを書く**

`internal/auth/source_test.go` の `TestClientID`（コメント `// 直列: …` の行ごと）を置き換える。パッケージ変数の差し替えがなくなるので並列にできる。

```go
// before
// 直列: パッケージ変数 builtinClientID の差し替え
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
	if got := ClientID(config.Host{Host: "GitHub.com"}); got != "Iv23builtin" {
		t.Errorf("GitHub.com (case) = %q", got)
	}
}

// after
// Hedgehug has no client ID of its own: it is the one hosts: names, on any
// host, github.com too.
func TestClientID(t *testing.T) {
	t.Parallel()
	for _, host := range []string{"github.com", "GitHub.com", "ghe"} {
		if got := ClientID(config.Host{Host: host, ClientID: "Iv23x"}); got != "Iv23x" {
			t.Errorf("%s with client_id = %q", host, got)
		}
		if got := ClientID(config.Host{Host: host}); got != "" {
			t.Errorf("%s without client_id = %q; want none", host, got)
		}
	}
}
```

`internal/auth/device_test.go`。`TestDeviceLogin` の最後の確認（変える前後）と、`TestDeviceRefreshNeedsHost` の直後に新しいテストを足す。

```go
// before
	if _, err := DeviceLogin(config.Host{Host: "ghe", Web: h.Web}, Options{}, func(*github.DeviceCode) {}); err == nil || !strings.Contains(err.Error(), "client_id") {
		t.Errorf("no client ID = %v", err)
	}

// after
	// No client_id, no request: the host is github.com too, and its web is a
	// live fakegh that would answer.
	noID := config.Host{Host: "github.com", Web: h.Web}
	_, err = DeviceLogin(noID, Options{}, func(*github.DeviceCode) { t.Error("the code was shown") })
	want := "the device flow needs hosts[].client_id for github.com in config.yaml; or use token: gh, token: env:<VAR>, or hedgehug auth login <name> --with-token"
	if err == nil || err.Error() != want {
		t.Errorf("no client ID = %v; want %q", err, want)
	}
```

（`TestDeviceLogin` の先頭は `s, h, _ := fakeHost(...)` ではなく `_, h, _ := fakeHost(...)` の形で、`dt, err := DeviceLogin(...)` が `err` を宣言済みなので、上の `_, err = DeviceLogin(...)` は `=` のままでよい。）

```go
func TestDeviceRefreshNeedsClientID(t *testing.T) {
	t.Parallel()
	_, h, refreshes := fakeHost(t, fakegh.State{Users: []string{"alice"}})
	h.ClientID = ""
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	a := config.Account{Name: "a", Host: "github.com", Login: "alice", Token: config.TokenSource{Kind: "device"}}
	k := newMem()
	opt := Options{Keyring: k, Host: h, Now: func() time.Time { return now }, LockFile: filepath.Join(t.TempDir(), "auth.lock")}

	// A token that does not end soon needs no client_id.
	_ = SaveDeviceToken(a, DeviceToken{AccessToken: "x", RefreshToken: "r", ExpiresAt: now.Add(8 * time.Hour)}, k)
	if tok, err := Token(a, opt); err != nil || tok != "x" {
		t.Errorf("fresh token = %q, %v", tok, err)
	}
	// One that ends soon needs a refresh, which fails before any request.
	_ = SaveDeviceToken(a, DeviceToken{AccessToken: "x", RefreshToken: "r", ExpiresAt: now}, k)
	_, err := Token(a, opt)
	if err == nil || !strings.Contains(err.Error(), "needs hosts[].client_id for github.com in config.yaml") || !strings.Contains(err.Error(), "hedgehug auth login a") {
		t.Errorf("refresh without client_id = %v", err)
	}
	if refreshes.Load() != 0 {
		t.Errorf("%d refresh requests were made", refreshes.Load())
	}
}
```

`internal/ops/auth_add_test.go`（Task 3 のファイル）の末尾に足す。

```go
// No client_id, no device flow: the login stops before it asks GitHub for a
// code, and writes nothing.
func TestAuthLoginDeviceNeedsClientID(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	k := newKR()
	ts.Keyring = k
	ts.setConfig("", cfgStore{"me", ""})
	before := ts.configBytes()
	_, err := ts.AuthLogin(AuthLoginInput{Host: "github.com", ShowCode: func(*github.DeviceCode) { t.Error("the code was shown") }})
	if err == nil || !strings.Contains(err.Error(), "the device flow needs hosts[].client_id for github.com in config.yaml") {
		t.Fatalf("err = %v", err)
	}
	if len(k.m) != 0 || !bytes.Equal(before, ts.configBytes()) {
		t.Errorf("keychain = %v, config.yaml changed = %v", k.m, !bytes.Equal(before, ts.configBytes()))
	}
}
```

（この追加で `internal/ops/auth_add_test.go` の import に `"github.com/kukv/hedgehug/internal/github"` を足す。）

`cmd/hedgehug/auth_test.go` の末尾に足す（`TestAuthLoginFillsEmptyToken` の後）。

```go
func TestAuthLoginDeviceNeedsClientID(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.replaceConfigYAML("accounts:\n  - {name: work, host: github.com, login: alice}\n")
	code, out, errs := te.run("auth", "login", "work")
	want := "hedgehug: the device flow needs hosts[].client_id for github.com in config.yaml; or use token: gh, token: env:<VAR>, or hedgehug auth login <name> --with-token\n"
	if code != 1 || out != "" || !strings.Contains(errs, want) {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 1 and %q", code, out, errs, want)
	}
	if b := te.configText(); strings.Contains(b, "token:") {
		t.Errorf("config.yaml = %s; want no token written", b)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/auth ./internal/ops ./cmd/hedgehug -run 'TestClientID|TestDeviceLogin$|TestDeviceRefreshNeedsClientID|TestAuthLoginDeviceNeedsClientID' -count=1`（timeout 600000）
Expected: FAIL。`TestDeviceLogin` は `no client ID = no GitHub App client ID for github.com; …`（文言の違い）、`TestDeviceRefreshNeedsClientID` は `refresh without client_id = …`（今は空の `client_id` のまま通信してしまう）、`internal/ops` と `cmd/hedgehug` の `TestAuthLoginDeviceNeedsClientID` も文言の違いで落ちる。`TestClientID` は `builtinClientID` が空のままなので、書き換えただけでは通る（定数を消す Step 3 で、古いテストが参照していた変数がなくなることを確かめる）

- [ ] **Step 3: 実装する**

`internal/auth/source.go`。

```go
// before
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
	if config.SameHost(h.Host, "github.com") {
		return builtinClientID
	}
	return ""
}

// after
// ClientID is the client ID of the device flow on the host h: client_id of
// hosts: in config.yaml, or empty. Hedgehug carries no client ID of its own.
func ClientID(h config.Host) string { return h.ClientID }
```

`internal/auth/device.go`。

```go
// before
	id := ClientID(h)
	if id == "" {
		return DeviceToken{}, fmt.Errorf("no GitHub App client ID for %s; set client_id of the host in hosts of config.yaml", h.Host)
	}

// after
	id := ClientID(h)
	if id == "" {
		return DeviceToken{}, fmt.Errorf("the device flow needs hosts[].client_id for %s in config.yaml; or use token: gh, token: env:<VAR>, or hedgehug auth login <name> --with-token", h.Host)
	}
```

リフレッシュは、`opt.Host.Web == ""` の確認の直後（ロックを取る前）に足す。

```go
// before
	if opt.Host.Web == "" {
		return "", errors.New("auth: Host is not set")
	}
	err = WithLock(opt, func() error {

// after
	if opt.Host.Web == "" {
		return "", errors.New("auth: Host is not set")
	}
	if ClientID(opt.Host) == "" {
		return "", fmt.Errorf("refreshing the token of %s needs hosts[].client_id for %s in config.yaml; run hedgehug auth login %s", Key(a), a.Host, a.Name)
	}
	err = WithLock(opt, func() error {
```

`internal/config/hosts.go:20`。

```go
// before
	ClientID string // empty: the built-in one of internal/auth, on github.com

// after
	ClientID string // client_id of the GitHub App for the device flow; empty: the device flow is not available on the host
```

- [ ] **Step 4: 成功を確かめて、コミットする**

Run: `go vet ./internal/auth ./internal/config ./internal/ops ./cmd/hedgehug && go test ./internal/auth ./internal/config ./internal/ops ./cmd/hedgehug -run 'TestClientID|TestDevice|TestAuth|TestHost' -count=1`（timeout 600000）
Expected: PASS。続けて `grep -rn builtinClientID --include='*.go' .` が何も出さないこと（`docs/` の spec と plan に残る記述は Task 13 が直す）

```bash
git add internal/auth/source.go internal/auth/device.go internal/config/hosts.go internal/auth/source_test.go internal/auth/device_test.go internal/ops/auth_add_test.go cmd/hedgehug/auth_test.go
git commit -m "$(cat <<'EOF'
feat(auth): GitHub App の client ID をバイナリに組み込まない

デバイスフローは hosts[].client_id を書いたときだけ使える。空なら何も通信せずに止め、リフレッシュも通信の前に失敗させる。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

#### spec にない細部（Task 4）

1. リフレッシュの文言は `refreshing the token of <host>/<login> needs hosts[].client_id for <host> in config.yaml; run hedgehug auth login <name>` にした（spec は「今と同じく `hh auth login <name>` を案内する」とだけ書く）。案内した `auth login` は、`client_id` がなければ §6 の文で止まるので、次に何をすればよいかはそこで分かる。
2. リフレッシュの確認は、リフレッシュが要るとき（期限の 5 分前から）だけ行う。期限まで余裕のあるトークンは、`client_id` がなくても使える。ロックを取る前に失敗させるので、ロックの待ちも通信も起きない。
3. `ClientID(h)` の関数は残す（spec: 「`ClientID(h)` は `hosts[].client_id` だけを返す」）。呼び出しが 2 か所あり、「何を client ID とするか」の定義を 1 か所に置くため。

---

### Task 5: `workspace set-prefix` と `comment add`

spec §1・§2。コマンドの名前を変えるだけで、動き・出力・`--json` は変えない。旧名（`workspace prefix`、`comment <id>`）は残さず、使い方の誤り（終了コード 2）にする。案内の文言が旧名を指している箇所（inbox・doctor・採番のエラー・store add の警告）も新しい名前にする。

**変えないもの（意図して残す）:**

- コミットの件名 `hedgehug: workspace prefix <ws> ME → BE` と `Hedgehug-Op: workspace`（履歴に残る記録で、CLI の名前ではない。`internal/ops/workspace.go` が作る）。`cmd/hedgehug/workspace_test.go` の `TestWorkspaceCommitSubjects` の期待値、`cmd/hedgehug/meanwhile_test.go:272` の件名の期待値もそのまま
- Go のメソッド名 `Session.WorkspacePrefix`（API の `PATCH /api/workspaces/…` も使う。spec §1）
- `internal/ops/inbox.go:709` の `"is given by the store with the workspace prefix; …"`（コマンドの案内ではない）
- 出力の `Changed the id prefix of … renumbered N tasks` の複数形は Task 12 が直す

**Files:**

- Modify: `cmd/hedgehug/workspace.go`（`prefix` → `set-prefix`）、`cmd/hedgehug/comment.go`（`comment add` を足し、親を「サブコマンドが要る」にする）
- Modify（案内の文言）: `internal/ops/inbox.go:688`、`internal/doctor/doctor.go:325`、`internal/store/ids.go:85`、`internal/ops/storeops.go:515`
- Modify（上の文言のテスト）: `internal/ops/inbox_test.go:454-458,492`、`cmd/hedgehug/inbox_test.go:402`、`internal/store/ids_test.go:115`、`internal/ops/storeops_test.go:282,303`
- Modify（ゴールデン）: `cmd/hedgehug/testdata/TestInboxPrefixDuplicate/inbox.golden`、`cmd/hedgehug/testdata/TestInboxDuplicateNamesEveryMatch/inbox.golden`、`cmd/hedgehug/testdata/TestStoreAddWarningNamesLater/add.err.golden`、`cmd/hedgehug/testdata/TestStoreAddWarnsPrefixDuplicate/add.err.golden`
- Modify（旧名で呼んでいるテスト）: `cmd/hedgehug/workspace_test.go:33,66,110,160`、`cmd/hedgehug/meanwhile_test.go:75,271,304`、`hh comment <id>` で呼んでいる `cmd/hedgehug/` の次のテスト: `undo_more_test.go:32,48`、`readonly_test.go:35`、`add_test.go:179`、`e2e_3b_test.go:225,246,277`、`log_test.go:19,141`、`inbox_test.go:127,154`、`watch_test.go:30,31,74,92,123,147`、`comment_test.go`（`te.ok("comment", x.ID…)`・`te.run("comment", x.ID…)`・`te.commentJSON("comment", x.ID…)` のすべて）、`e2e_3c_test.go:157`、`scenario_test.go:51`
- Modify（GUI が見せる行）: `web/core/cli.ts:87-92`（`cliComment`）、`web/core/cli.test.ts:105`
- Test: `cmd/hedgehug/workspace_test.go`（新しいテストを足す）、`cmd/hedgehug/comment_test.go`（新しいテストを足す）、`web/core/cli.test.ts`

洗い出しの根拠（実装前に同じ grep を回して、一覧との差がないか確かめる）:

```bash
grep -rn 'workspace prefix' cmd internal web tools api --include='*' -I | grep -v node_modules
grep -rn '"comment", ' cmd/hedgehug/*_test.go
grep -rn 'hh comment\|hedgehug comment' cmd internal web tools -I | grep -v node_modules
```

**Interfaces:**

- Consumes: Task 1・2 のテスト環境（`newStoreEnv` が既定でアカウントを書く）。`usageErrorf`、`minArgs`、`exactArgs`（`cmd/hedgehug/app.go`）
- Produces: `hh workspace set-prefix <workspace> <P>`、`hh comment add <id> [text...]`。`hh comment <id> …` は終了コード 2 で、標準エラーに `hedgehug: "<id>" is not a subcommand; add a comment with hedgehug comment add <id> [text...] (the others are edit and rm)`。`hh workspace prefix …` は cobra の `unknown command "prefix" for "hedgehug workspace"`（終了コード 2、`main.go` の `unknown command` の扱い）。案内の文言の新しい形は `… change it with hedgehug workspace set-prefix <ws> <new prefix>`

- [ ] **Step 1: 新しい名前と旧名の失敗を確かめるテストを書く**

`cmd/hedgehug/workspace_test.go` の `TestWorkspacePrefixSame` の前に足す。

```go
func TestWorkspaceSetPrefixAndTheOldNameIsAUsageError(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.addJSON("a")
	before := len(te.log("me"))

	code, _, errs := te.run("workspace", "prefix", "me", "BE")
	if code != 2 || !strings.Contains(errs, `unknown command "prefix"`) {
		t.Errorf("workspace prefix: exit %d, stderr %q; want 2 and an unknown command", code, errs)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d: the old name wrote", n, before)
	}

	out := te.ok("workspace", "set-prefix", "me", "BE")
	if !strings.Contains(out, "Changed the id prefix of me to BE") || !strings.Contains(out, "ME-1 → BE-1") {
		t.Errorf("set-prefix =\n%s", out)
	}
	if n := len(te.log("me")); n != before+1 {
		t.Errorf("commits = %d, want %d", n, before+1)
	}
}
```

`cmd/hedgehug/comment_test.go` の `TestCommentAddFromArgs` の前に足す。

```go
func TestCommentNeedsASubcommand(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	before := len(te.log("me"))

	code, _, errs := te.run("comment")
	if code != 2 || !strings.Contains(errs, "a subcommand is needed") {
		t.Errorf("comment: exit %d, stderr %q", code, errs)
	}
	for _, args := range [][]string{{"comment", x.ID}, {"comment", x.ID, "hello", "world"}} {
		code, _, errs := te.run(args...)
		if code != 2 || !strings.Contains(errs, "hedgehug comment add <id> [text...]") || !strings.Contains(errs, "edit and rm") {
			t.Errorf("%v: exit %d, stderr %q; want 2 and the way to add", args, code, errs)
		}
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d: nothing is written", n, before)
	}
	if cs := te.comments(x.ULID); len(cs) != 0 {
		t.Errorf("comments = %+v", cs)
	}
}
```

`web/core/cli.test.ts:105` を書き換える（`edit`・`rm` の行は変えない）。

```ts
    expect(cliComment("REL-8", "add")).toBe("hh comment add REL-8");
```

- [ ] **Step 2: 失敗を確かめる**

Run（timeout 600000）: `go test ./cmd/hedgehug -run 'TestWorkspaceSetPrefixAndTheOldNameIsAUsageError|TestCommentNeedsASubcommand' -count=1`
Expected: 2 つとも FAIL（`workspace prefix` がまだ動く、`comment <id>` が追加になる）。

Run: `cd web && pnpm exec vitest run core/cli.test.ts`
Expected: `cliComment` の 1 件が FAIL（`hh comment REL-8` が返る）。

- [ ] **Step 3: `workspace set-prefix` に改名する**

`cmd/hedgehug/workspace.go` の `prefix` のサブコマンドを書き換える（`workspacePrefix` の関数名と中身は変えない）。

変更前:

```go
		&cobra.Command{
			Use:   "prefix <workspace> <P>",
			Short: "Change the id prefix of a workspace and renumber its tasks",
```

変更後:

```go
		&cobra.Command{
			Use:   "set-prefix <workspace> <P>",
			Short: "Change the id prefix of a workspace and renumber its tasks",
```

- [ ] **Step 4: `comment add` にする**

`cmd/hedgehug/comment.go` の `commentCmd` の先頭（`cmd := …` から `cmd.AddCommand(` の手前まで）を次に置き換える。`edit`・`rm` の定義は変えず、`AddCommand` の最初に `add` を足す。

変更前:

```go
	cmd := &cobra.Command{
		Use:   "comment <id> [text...]",
		Short: "Add a comment to a task, or edit or delete one",
		Long:  "Add a comment to a task. Without text, the body is written in $EDITOR.",
		Args:  minArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.commentAdd(args[0], args[1:]) },
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "edit <comment-ulid>",
```

変更後:

```go
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Add a comment to a task, or edit or delete one",
		// hedgehug comment <id> was the way to add one; say so rather than
		// fail with an unknown command.
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usageErrorf("%q is not a subcommand; add a comment with hedgehug comment add <id> [text...] (the others are edit and rm)", args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "add <id> [text...]",
			Short: "Add a comment to a task",
			Long:  "Add a comment to a task. Without text, the body is written in $EDITOR.",
			Args:  minArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return a.commentAdd(args[0], args[1:]) },
		},
		&cobra.Command{
			Use:   "edit <comment-ulid>",
```

- [ ] **Step 5: 旧名で呼んでいるテストを新しい名前に直す**

リポジトリのルートで、次を順に実行する。1 つ目は `cmd/hedgehug` のテストの `"comment", <id>…` を `"comment", "add", <id>…` にする。`Op{Name: "comment", …}`（`Name: ` の直後）、すでに `"add"`・`"edit"`・`"rm"` が続くもの、`scenario_test.go` の `step("comment", "comment", …)` の 1 つ目（op 名）、`verb.cmd` は除く。

```bash
perl -pi -e 's/(?<!Name: )"comment", (?!"edit"|"rm"|"add"|"comment"|verb\.cmd)/"comment", "add", /g' cmd/hedgehug/*_test.go
perl -pi -e 's/"workspace", "prefix"/"workspace", "set-prefix"/g' cmd/hedgehug/*_test.go
```

次の 2 行は上で変わらないので手で直す。

- `cmd/hedgehug/workspace_test.go:110`: `{[]string{"prefix", "me", "BE"}, "hedgehug: workspace prefix me ME → BE", "me"},` → `{[]string{"set-prefix", "me", "BE"}, "hedgehug: workspace prefix me ME → BE", "me"},`（件名は変えない）
- `cmd/hedgehug/workspace_test.go:160`: `te.wsJSON(&prefixed, "prefix", "me", "BE")` → `te.wsJSON(&prefixed, "set-prefix", "me", "BE")`

置き換えの結果を確かめる。

```bash
grep -n '"comment", ' cmd/hedgehug/*_test.go | grep -v '"comment", "\(add\|edit\|rm\)"' | grep -v 'Name: "comment"'
```

Expected: `cmd/hedgehug/comment_test.go` の `te.run("comment", verb.cmd, c.ULID)` の 1 行だけ（`verb.cmd` は `edit` か `rm`）。ほかに出たら、その行が追加の呼び出しなら `"add"` を足す。

```bash
grep -n '"prefix"' cmd/hedgehug/workspace_test.go cmd/hedgehug/meanwhile_test.go
```

Expected: 何も出ない。

- [ ] **Step 6: 案内の文言を新しい名前にする**

ソースを直す（`inbox.go:709` は直さない）。

- `internal/ops/inbox.go:688`: `"%s is also used by %s; change it with hedgehug%s workspace prefix %s <new prefix>"` の `workspace prefix` → `workspace set-prefix`
- `internal/doctor/doctor.go:325`: `Hint: "change one with hedgehug workspace prefix (or rename)",` → `Hint: "change one with hedgehug workspace set-prefix (or rename)",`
- `internal/store/ids.go:85`: `"%s: %w; fix it with hedgehug workspace prefix"` → `"%s: %w; fix it with hedgehug workspace set-prefix"`
- `internal/ops/storeops.go:515`: `… should change it with hedgehug workspace prefix",` → `… should change it with hedgehug workspace set-prefix",`

テストとゴールデンは機械的に直せる（これらのファイルに「コマンドの案内ではない」`workspace prefix` はない）。

```bash
sed -i 's/workspace prefix /workspace set-prefix /g' internal/ops/inbox_test.go cmd/hedgehug/inbox_test.go \
  cmd/hedgehug/testdata/TestInboxPrefixDuplicate/inbox.golden cmd/hedgehug/testdata/TestInboxDuplicateNamesEveryMatch/inbox.golden
sed -i 's/hedgehug workspace prefix/hedgehug workspace set-prefix/g' internal/store/ids_test.go internal/ops/storeops_test.go \
  cmd/hedgehug/testdata/TestStoreAddWarningNamesLater/add.err.golden cmd/hedgehug/testdata/TestStoreAddWarnsPrefixDuplicate/add.err.golden
```

残りを確かめる。

```bash
grep -rn 'workspace prefix' cmd internal web tools api -I | grep -v node_modules
```

Expected: 次の 4 種類だけ。ほかが出たら案内の文言なので直す。(1) `internal/ops/inbox.go:709` の `the workspace prefix;`、(2) コミットの件名（`workspace_test.go` の `TestWorkspaceCommitSubjects` の期待値、`meanwhile_test.go` の件名の期待値と `strings.Contains(subject, "workspace prefix")`、`internal/ops/workspace.go` の件名を作る行）、(3) `internal/sync/ops_test.go` の `{"workspace prefix", opWSPrefix}` とそのコメント、(4) Go の識別子（`WorkspacePrefix` など）と `TestWorkspacePrefix…` のテスト名。

- [ ] **Step 7: GUI が見せる行を直す**

`web/core/cli.ts` の `cliComment` とその上のコメントを置き換える。

変更前:

```ts
// cliComment is hh comment (add), hh comment edit <comment> or hh comment rm
// <comment>; edit and rm take the ULID of the comment as cid.
export function cliComment(id: string, kind: "add" | "edit" | "rm", cid?: string): string {
  if (kind === "add") return cliLine(["comment", id]);
  return cliLine(["comment", kind, cid ?? id]);
}
```

変更後:

```ts
// cliComment is hh comment add <id>, hh comment edit <comment> or hh comment
// rm <comment>; edit and rm take the ULID of the comment as cid.
export function cliComment(id: string, kind: "add" | "edit" | "rm", cid?: string): string {
  if (kind === "add") return cliLine(["comment", "add", id]);
  return cliLine(["comment", kind, cid ?? id]);
}
```

`web/core/api/schema.d.ts` と `api/openapi.yaml` の `as hh comment add` などの要約はすでに新しい名前なので変えない。

- [ ] **Step 8: 通す**

Run（timeout 600000）: `go build ./... && go vet ./...`
Expected: エラーなし。

Run（timeout 600000）: `go test ./cmd/hedgehug -run 'TestWorkspace|TestComment|TestScenario|TestInbox|TestStoreAdd|TestMeanwhile|TestWorkspacePrefix|TestUndo|TestLog|TestWatch|TestReadOnly|TestAdd|TestE2E' -count=1 && go test ./internal/ops ./internal/store ./internal/doctor -count=1`
Expected: PASS（`TestE2E…` は devenv を組むので時間がかかる）。

Run: `cd web && pnpm exec vitest run core/cli.test.ts && pnpm check && pnpm lint`
Expected: PASS、警告・指摘ゼロ。

- [ ] **Step 9: コミット**

```bash
git add cmd/hedgehug internal/ops internal/store internal/doctor web/core/cli.ts web/core/cli.test.ts
git commit -m "feat(cli): workspace prefix を workspace set-prefix に、comment <id> を comment add <id> に改める

comment edit・comment rm と揃えて add を足し、hedgehug comment <id> は
サブコマンドが要る使い方の誤りにする。旧名は残さない。案内の文言
(inbox・doctor・採番・store add の警告)と GUI が見せる行も新しい名前にする。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

#### spec にない細部

1. `hh comment <id> …`（サブコマンドなしで引数あり）の文言: spec は「`add`・`edit`・`rm` を案内する」とだけ書く。`hh dep` は cobra の `unknown command` のままで案内がないので、`comment` だけ `Args` を持たせ、`"<id>" is not a subcommand; add a comment with hedgehug comment add <id> [text...] (the others are edit and rm)` を返す（引数なしは `hh dep` と同じ `a subcommand is needed` と使い方の表示）。なぜ: 旧名で打つ利用者に、新しい形をその場で見せるため。
2. コミットの件名 `hedgehug: workspace prefix …` は変えない。なぜ: CLI の名前ではなく履歴の記録で、API の `PATCH` も同じ件名を使う。変えるなら過去のコミットと食い違う。
3. `Session.WorkspacePrefix` などの Go の識別子は変えない。なぜ: spec §1「API は CLI の名前と関係しないので変えない」。

---

### Task 6: `show` の `title`・`checklist`・`progress` の行

spec §3。1 行目の見出し（件名だけの行）と、その次の行の進捗（`☑ 1/3  1/2`）をやめ、項目の行として出す。`--json` と `ls --tree` の行は変えない。

**Files:**

- Modify: `internal/ops/progress.go`（`ChecklistText`・`ProgressFractionText` を足し、`ProgressText` をそれで組む）
- Modify: `cmd/hedgehug/show.go`（`showText` の見出しと進捗の行を消し、`title`・`checklist`・`progress` を項目として出す）
- Modify（期待値）: `cmd/hedgehug/show_test.go:48-52`（`TestShow` の先頭の検査）
- Modify（ゴールデン。`go test -update` で作り直す）: `cmd/hedgehug/testdata/TestLsTreeProgress/show.golden`・`show-a.golden`、`cmd/hedgehug/testdata/TestShowTellsItsStore/show.golden`、`cmd/hedgehug/testdata/TestPrefixAcrossStores/show.golden`・`show-web.golden`、`cmd/hedgehug/testdata/TestLsOneStoreHasNoStoreColumn/show.golden`
- Test: `internal/ops/progress_test.go`、`cmd/hedgehug/show_test.go`

`show` の本文を 1 行目から読む他のテストは、`strings.Contains` か正規表現で項目を探していて、1 行目が件名であることに頼っていない（`edit_test.go:89,263`、`comment_test.go:239,361`、`log_test.go:120,143`、`scenario_test.go:60`、`workspace_test.go:253`、`inbox_test.go:95,242,315,341`、`watch_test.go:120,124`）。変える必要はないが、Step 5 の全体で通ることを確かめる。`ProgressText` は `cmd/hedgehug/tree.go:120` の `ls --tree` の行が使い続けるので消さない。

**Interfaces:**

- Consumes: `ops.ChecklistOf`・`ops.ProgressOf`（`internal/ops/progress.go`）。`showText` の中の `field(k, vs...)`（値が空なら `-`、キーの幅は 11 列に揃える）
- Produces: `ops.ChecklistText(c *Count) string`（nil なら `""`、そうでなければ `☑ 1/3`）、`ops.ProgressFractionText(c *Count) string`（nil なら `""`、そうでなければ `1/2`）。`show` の出力は、`title`（最初の行）、`id`、`ulid`、…、`status`、`checklist`、`progress`、`parent`、…の順

- [ ] **Step 1: 失敗するテストを書く**

`internal/ops/progress_test.go` の `TestChecklistOfAndText` の末尾（`ProgressText` の表の `for` のあと、関数を閉じる `}` の前）に足す。

```go
	for _, c := range []struct {
		c    *Count
		want string
	}{
		{nil, ""},
		{&Count{0, 0}, "☑ 0/0"},
		{&Count{1, 3}, "☑ 1/3"},
	} {
		if got := ChecklistText(c.c); got != c.want {
			t.Errorf("ChecklistText(%v) = %q; want %q", c.c, got, c.want)
		}
	}
	for _, c := range []struct {
		c    *Count
		want string
	}{
		{nil, ""},
		{&Count{0, 0}, "0/0"},
		{&Count{1, 2}, "1/2"},
	} {
		if got := ProgressFractionText(c.c); got != c.want {
			t.Errorf("ProgressFractionText(%v) = %q; want %q", c.c, got, c.want)
		}
	}
```

`cmd/hedgehug/show_test.go` の `TestShowErrors` の前に足す。`editWith` が `t.Setenv` を使うので `t.Parallel()` は付けない（`TestLsTreeProgress` と同じ）。

```go
// 直列: editWith が t.Setenv（EDITOR と HH_TEST_EDITOR_SCRIPT）を使う
func TestShowTitleChecklistAndProgressAreFields(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("Project", "--kind", "project")
	a := te.addJSON("Alpha", "--parent", p.ID, "--status", "done")
	b := te.addJSON("Beta", "--parent", p.ID)
	te.editWith(te.raw(a.ULID) + "- [x] one\n- [ ] two\n- [ ] three\n")
	te.ok("edit", a.ID)

	// a project: the progress of its tasks (Alpha is done, Beta is not), no checklist
	out := te.ok("show", p.ID)
	if want := "  title       Project\n  id          " + p.ID + "\n"; !strings.HasPrefix(out, want) {
		t.Errorf("show of the project does not start with %q:\n%s", want, out)
	}
	if want := "  status      todo\n  progress    1/2\n  parent      -\n"; !strings.Contains(out, want) {
		t.Errorf("show of the project lacks %q:\n%s", want, out)
	}
	if strings.Contains(out, "checklist") {
		t.Errorf("a project without a checklist has a checklist line:\n%s", out)
	}

	// a task with a checklist in the body: the checklist, no progress
	out = te.ok("show", a.ID)
	if want := "  title       Alpha\n"; !strings.HasPrefix(out, want) {
		t.Errorf("show of the task does not start with %q:\n%s", want, out)
	}
	if want := "  status      done\n  checklist   ☑ 1/3\n  parent      "; !strings.Contains(out, want) {
		t.Errorf("show of the task lacks %q:\n%s", want, out)
	}
	if strings.Contains(out, "progress") {
		t.Errorf("a task of the work role has a progress line:\n%s", out)
	}

	// neither: no line for either
	out = te.ok("show", b.ID)
	if strings.Contains(out, "checklist") || strings.Contains(out, "progress") {
		t.Errorf("a task with neither has a checklist or progress line:\n%s", out)
	}

	// the heading line is gone: the output no longer starts with the title alone
	if strings.HasPrefix(out, "Beta\n") {
		t.Errorf("show still starts with a heading:\n%s", out)
	}
}
```

`cmd/hedgehug/show_test.go` の `TestShowBroken` で、`out := te.ok("show", x.ULID)` の直後の `if !strings.Contains(out, "\nbroken: ")` の前に足す（壊れていてタイトルがないときは `-`）。

```go
	if !strings.HasPrefix(out, "  title       -\n") {
		t.Errorf("show of a broken task does not start with a title of -:\n%s", out)
	}
```

`cmd/hedgehug/show_test.go:49-52`（`TestShow` の先頭の検査）を書き換える。

変更前:

```go
	// The heading is the title alone; the id, the ULID and the store are the first fields.
	if want := "Write the spec\n  id          " + a.ID + "\n  ulid        " + a.ULID + "\n  store       me\n  workspace   me (ME, "; !strings.HasPrefix(out, want) {
```

変更後:

```go
	// The title is the first field; the id, the ULID and the store follow.
	if want := "  title       Write the spec\n  id          " + a.ID + "\n  ulid        " + a.ULID + "\n  store       me\n  workspace   me (ME, "; !strings.HasPrefix(out, want) {
```

- [ ] **Step 2: 失敗を確かめる**

Run（timeout 600000）: `go test ./internal/ops -run 'TestChecklistOfAndText' -count=1 && go test ./cmd/hedgehug -run 'TestShow' -count=1`
Expected: ops は `undefined: ChecklistText` でビルド失敗。cmd は `TestShow`・`TestShowBroken`・`TestShowTitleChecklistAndProgressAreFields` が FAIL。

- [ ] **Step 3: `ops` に 2 つの部分を足す**

`internal/ops/progress.go` の `ProgressText` を置き換える。

変更前:

```go
// ProgressText is the checklist and the progress as the CLI writes them:
// "☑ 2/5  3/14", either alone, or "" with neither.
func ProgressText(checklist, progress *Count) string {
	var parts []string
	if checklist != nil {
		parts = append(parts, fmt.Sprintf("☑ %d/%d", checklist.Done, checklist.Total))
	}
	if progress != nil {
		parts = append(parts, fmt.Sprintf("%d/%d", progress.Done, progress.Total))
	}
	return strings.Join(parts, "  ")
}
```

変更後:

```go
// ChecklistText is the checklist as the CLI writes it, "☑ 2/5", or "" with
// none.
func ChecklistText(c *Count) string {
	if c == nil {
		return ""
	}
	return fmt.Sprintf("☑ %d/%d", c.Done, c.Total)
}

// ProgressFractionText is the progress as the CLI writes it, "3/14", or ""
// with none.
func ProgressFractionText(c *Count) string {
	if c == nil {
		return ""
	}
	return fmt.Sprintf("%d/%d", c.Done, c.Total)
}

// ProgressText is the checklist and the progress as one line of the tree
// writes them: "☑ 2/5  3/14", either alone, or "" with neither.
func ProgressText(checklist, progress *Count) string {
	var parts []string
	for _, p := range []string{ChecklistText(checklist), ProgressFractionText(progress)} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "  ")
}
```

- [ ] **Step 4: `show` を直す**

`cmd/hedgehug/show.go` の `showText` の先頭を置き換える。

変更前:

```go
	st := a.style()
	var b strings.Builder
	title := e.Title
	if title == "" {
		title = "-" // a file that does not decode
	}
	b.WriteString(title + "\n")
	if p := ops.ProgressText(ops.ChecklistOf(e), ops.ProgressOf(ix, e)); p != "" {
		b.WriteString("  " + p + "\n")
	}
	field := func(k string, vs ...string) {
```

変更後:

```go
	st := a.style()
	var b strings.Builder
	field := func(k string, vs ...string) {
```

`field("id", ix.DisplayID(e))` の前（`refs` の定義のあと）に `title` を足す。タイトルがなければ（壊れたファイル）`field` が `-` にする。

変更前:

```go
	field("id", ix.DisplayID(e))
	field("ulid", e.ULID.String())
```

変更後:

```go
	field("title", e.Title) // "-" for a file that does not decode
	field("id", ix.DisplayID(e))
	field("ulid", e.ULID.String())
```

`status` の直後に `checklist` と `progress` を足す。

変更前:

```go
	field("status", e.Status)
	if e.Parent != nil {
```

変更後:

```go
	field("status", e.Status)
	if c := ops.ChecklistText(ops.ChecklistOf(e)); c != "" {
		field("checklist", c)
	}
	if p := ops.ProgressFractionText(ops.ProgressOf(ix, e)); p != "" {
		field("progress", p)
	}
	if e.Parent != nil {
```

- [ ] **Step 5: ゴールデンを作り直し、差分を確かめる**

Run（timeout 600000）: `go test ./cmd/hedgehug -run 'TestLsTreeProgress|TestShowTellsItsStore|TestPrefixAcrossStores|TestLsOneStoreHasNoStoreColumn' -count=1 -update`

差分は `show*.golden` の次の変更だけであることを `git diff --stat cmd/hedgehug/testdata` と `git diff cmd/hedgehug/testdata` で確かめる。ほかのゴールデンが変わっていたら、`show` と関係のない変更が混じっているので原因を調べる。

- 1 行目の `<件名>` が `  title       <件名>` に変わり、`  id` の前に来る
- `TestLsTreeProgress/show.golden`: `  1/2` の行が消え、`  status      todo` の次に `  progress    1/2`
- `TestLsTreeProgress/show-a.golden`: `  ☑ 1/2` の行が消え、`  status      done` の次に `  checklist   ☑ 1/2`
- そのほかの 4 つのゴールデンは、1 行目の変更だけ

Run（timeout 600000）: `go test ./internal/ops ./cmd/hedgehug -run 'Show|Progress|Checklist|TestLsTree|TestPrefixAcrossStores|TestLsOneStore|TestEdit|TestComment|TestLog|TestScenario|TestInbox|TestWatch|TestWorkspace' -count=1`
Expected: PASS。

Run: `go vet ./...`
Expected: エラーなし。

- [ ] **Step 6: コミット**

```bash
git add internal/ops/progress.go internal/ops/progress_test.go cmd/hedgehug/show.go cmd/hedgehug/show_test.go cmd/hedgehug/testdata
git commit -m "feat(cli): show の件名と進捗を項目の行として出す

1 行目の見出しと、その次の ☑ 1/3  1/2 の行をやめ、title・checklist・
progress を id などと同じ項目の行にする。checklist と progress は status の
直後に、本文にチェックリストがあるとき・進捗を持つ種類のときだけ出す。
--json と ls --tree の行は変えない。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

#### spec にない細部

1. `ops` に `ChecklistText`・`ProgressFractionText` を足し、`ProgressText` はそれで組み直して残す。なぜ: spec は「今の `ProgressText` の 2 つの部分を分けたもの」と書く。`ls --tree` の行（`cmd/hedgehug/tree.go:120`）は 2 つを 1 行に並べるので `ProgressText` が要る。
2. 進捗が 0/0 の container（子のない project）も `progress    0/0` を出す。なぜ: spec は「出す条件は今の `ProgressText` が空でない条件と同じ」で、今は `ProgressOf` が nil でなければ出す（`0/0` を含む）。

---

### Task 7: `store remove` の既定の削除と `--keep`

spec §4。`hh store remove <name>` は、config.yaml から外してローカルのディレクトリも消す。消すとタスクが戻らない場合（push していないコミットがある、remote がない）は拒否する。`--keep` で config.yaml から外すだけにする。`--purge` は廃止する。

今の `StoreRemove(name string, purge bool)` を `StoreRemove(name string, keep bool)` に変える。**真偽の意味が反転する**ので、既存の呼び出し（`false` は「残す」だった）をすべて `true` に直す。直し忘れると、テストの Store が消える。

spec の「消す処理は、ディレクトリのロック（store のロック）を取ってから行う（今の `--purge` と同じ）」について: 今の `--purge` はロックを取っていない（`checkPushed` は開いて調べるだけで、`os.RemoveAll` もロックの外）。spec の意図（調べてから消すまでの間に書き込みが入らない）に合わせて、この Task で取る。

**Files:**
- Modify: `internal/ops/storeops.go`（`StoreRemoveResult`・`StoreRemove`・`checkPushed` → `lockToDelete` と `checkPushed`。559〜601 行、635〜665 行）
- Modify: `cmd/hedgehug/store_add.go`（`storeRemoveCmd`・`storeRemove`。77〜105 行）
- Modify: `cmd/hedgehug/testdata/TestStoreRemoveJSON/remove.golden`（`purged` → `deleted`）
- Test（`StoreRemove` の呼び出しを直す）: `internal/ops/storeops_test.go`（46 行目、378〜462 行目のテスト 3 つ）、`internal/ops/config_recheck_test.go`（83 行目）、`internal/ops/purgemoved_test.go`（84 行目）、`internal/ops/duplicates_test.go`（135 行目）、`internal/ops/maintain_test.go`（226 行目）、`internal/ops/auth_test.go`（519 行目）
- Test（CLI の `store remove` を直す）: `cmd/hedgehug/store_add_test.go`（`TestStoreListAndRemove`・`TestStoreRemovePurge`・`TestStoreRemovePurgeWithoutRemote`）、`cmd/hedgehug/store_more_test.go`（`TestStoreRemoveJSON`・`TestStoreRemovePurgeRefusesDirtyTree`）、`cmd/hedgehug/store_default_test.go`（26 行目）、`cmd/hedgehug/purge_moved_test.go`（102・123 行目）

行番号は目安。着手の前に洗い出す: `grep -rn 'StoreRemove(' --include='*.go' .` と `grep -rn '"store", "remove"' --include='*.go' cmd/`。`--purge` の参照は、`grep -rn -- '--purge' --include='*.go' .` で `cmd/hedgehug/store_add.go`・`store_add_test.go`・`store_more_test.go` の 3 ファイルだけ（`internal/` と `web/` にはない。`internal/server` は `StoreRemove` を呼ばない）。

**Interfaces:**
- Consumes: `Session.updateConfig`、`store.LockInOrder`・`(*store.Locked).Release`、`classify`、`Unpushed`、`Session.openStoreAt`、`addBareRemote`・`heldElsewhere`（ops のテストのヘルパー）、`twoUsers`・`newTestEnv`（CLI のテストのヘルパー）
- Produces: `(*Session).StoreRemove(name string, keep bool) (StoreRemoveResult, error)`、`StoreRemoveResult{Name, Path string; Deleted bool}`（JSON は `{name, path, deleted}`。`purged` は消える）。エラーは今までと同じ `KindConflict`・`store.unpushed`。出力: `Removed store <name> and deleted <path>`／`--keep` のとき `Removed store <name> from the config; <path> is left`／ディレクトリがもうなかったとき `Removed store <name> from the config; <path> was not there`。`--purge` は未知のフラグ（終了コード 2）

- [ ] **Step 1: ops のテストを書き換える**

`internal/ops/storeops_test.go`。46 行目の呼び出しを直す。

```go
// before
	rm, err := ts.StoreRemove("team", false)
	if err != nil {
		t.Fatal(err)
	}
	if rm != (StoreRemoveResult{Name: "team", Path: res.Path}) || !exists(res.Path) {
		t.Errorf("StoreRemove = %+v; want the directory left", rm)
	}

// after
	rm, err := ts.StoreRemove("team", true)
	if err != nil {
		t.Fatal(err)
	}
	if rm != (StoreRemoveResult{Name: "team", Path: res.Path}) || !exists(res.Path) {
		t.Errorf("StoreRemove --keep = %+v; want the directory left", rm)
	}
```

`TestStoreRemoveNotFound`（368 行目）の `ts.StoreRemove("nope", false)` は、Store がないので `keep` に関係なく同じ。直さない。

`TestStoreRemovePurgeRefusals`・`TestStoreRemovePurgeOfAStoreThatDoesNotOpen`・`TestStoreRemovePurgeDeletesAPushedStore` の 3 つ（`TestStoreRemoveNotFound` の次から `storeops_test.go` の最後の `}` まで）を、次の 5 つのテストに置き換える。

```go
func TestStoreRemoveRefusals(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	e := func() config.StoreEntry { _, e, _ := ts.Entry(); return e }()

	// No remote: the commits are only here.
	_, err := ts.StoreRemove("me", false)
	wantErr(t, err, KindConflict, "store.unpushed")
	if want := "store me has no remote, so removing " + e.Path + " would lose its tasks for good; use --keep to take the store out of the config and leave the directory, or delete the directory yourself if you do not need it"; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}

	remote := addBareRemote(t, ts)
	_ = remote
	// A remote the store never pushed to: all its commits are unpushed.
	_, err = ts.StoreRemove("me", false)
	wantErr(t, err, KindConflict, "store.unpushed")
	if !strings.Contains(err.Error(), "store me has ") || !strings.Contains(err.Error(), "not pushed; run hedgehug sync --store me first") || !strings.Contains(err.Error(), "--keep") {
		t.Errorf("message = %q", err)
	}

	if _, err := ts.Sync(false, true); err != nil {
		t.Fatal(err)
	}
	// Changes that are not committed.
	if err := os.WriteFile(filepath.Join(e.Path, "stray.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = ts.StoreRemove("me", false)
	wantErr(t, err, KindConflict, "store.unpushed")
	if !strings.HasPrefix(err.Error(), "store me has changes that are not committed (") || !strings.Contains(err.Error(), "stray.txt") {
		t.Errorf("message = %q", err)
	}
	if err := os.Remove(filepath.Join(e.Path, "stray.txt")); err != nil {
		t.Fatal(err)
	}

	// A commit after the push.
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err = ts.StoreRemove("me", false)
	wantErr(t, err, KindConflict, "store.unpushed")
	if !strings.Contains(err.Error(), "run hedgehug sync --store me first") || !strings.Contains(err.Error(), "--keep") {
		t.Errorf("message = %q", err)
	}
	if list, _ := ts.StoreList(); len(list) != 1 || !exists(e.Path) {
		t.Error("a refused removal removed the store")
	}

	// --keep takes the store out all the same, and checks nothing.
	res, err := ts.StoreRemove("me", true)
	if err != nil || res != (StoreRemoveResult{Name: "me", Path: e.Path}) || !exists(e.Path) {
		t.Errorf("StoreRemove --keep = %+v, %v; want the store out of the config and the directory left", res, err)
	}
}

func TestStoreRemoveOfAStoreThatDoesNotOpen(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	addBareRemote(t, ts)
	_, e, _ := ts.Entry()
	if err := os.WriteFile(filepath.Join(e.Path, model.StoreMetaPath), []byte("not: [yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ts.StoreRemove("me", false)
	wantErr(t, err, KindConflict, "store.unpushed")
	if !strings.HasPrefix(err.Error(), "store me: ") || !strings.HasSuffix(err.Error(), "; delete the directory by hand if you mean to") {
		t.Errorf("message = %q", err)
	}
}

func TestStoreRemoveDeletesAPushedStore(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	addBareRemote(t, ts)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Sync(false, true); err != nil {
		t.Fatal(err)
	}
	_, e, _ := ts.Entry()
	res, err := ts.StoreRemove("me", false)
	if err != nil {
		t.Fatal(err)
	}
	if res != (StoreRemoveResult{Name: "me", Path: e.Path, Deleted: true}) || exists(e.Path) {
		t.Errorf("StoreRemove = %+v; want the directory deleted (%v)", res, exists(e.Path))
	}
	if list, _ := ts.StoreList(); len(list) != 0 {
		t.Errorf("StoreList = %+v", list)
	}
}

// A directory that is not there has nothing to lose: the store is only taken
// out of the config, with or without a remote.
func TestStoreRemoveOfADirectoryThatIsGone(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	_, e, _ := ts.Entry()
	if err := os.RemoveAll(e.Path); err != nil {
		t.Fatal(err)
	}
	res, err := ts.StoreRemove("me", false)
	if err != nil || res != (StoreRemoveResult{Name: "me", Path: e.Path}) {
		t.Errorf("StoreRemove = %+v, %v; want the config changed and nothing deleted", res, err)
	}
	if list, _ := ts.StoreList(); len(list) != 0 {
		t.Errorf("StoreList = %+v", list)
	}
}

// The store lock is held from the check until the directory is gone, so that
// nothing is written between the two: it is held when the config is written,
// which comes between them.
func TestStoreRemoveHoldsTheStoreLock(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	addBareRemote(t, ts)
	if _, err := ts.Sync(false, true); err != nil {
		t.Fatal(err)
	}
	_, e, _ := ts.Entry()
	lock := filepath.Join(e.Path, ".hedgehug", "local", "lock")
	if heldElsewhere(lock) {
		t.Fatal("the store is locked before the removal")
	}
	held := false
	ts.BeforeUpdate = func() { held = heldElsewhere(lock) }
	if _, err := ts.StoreRemove("me", false); err != nil {
		t.Fatal(err)
	}
	if !held {
		t.Error("the store lock was not held while the config was written")
	}
}
```

そのほかの `StoreRemove(…, false)` は、設定から外すだけのつもりで書かれている。すべて `true` に直す（意味は変わらない）。

```go
// internal/ops/config_recheck_test.go:83
	_, err := ts.StoreRemove("team", true)
// internal/ops/purgemoved_test.go:84
	if _, err := f.ts.StoreRemove("org-b", true); err != nil {
// internal/ops/duplicates_test.go:135
	if _, err := f.ts.StoreRemove("org-b", true); err != nil {
// internal/ops/maintain_test.go:226
	if _, err := ts.StoreRemove("me", true); err != nil {
// internal/ops/auth_test.go:519
	if _, err := ts.StoreRemove("me", true); err != nil {
```

（`config_recheck_test.go` は特に大事: `false` のままだと、remote のない Store の拒否が config の更新より先に起き、`store.not_found` を確かめるテストの意味が変わる。）

- [ ] **Step 2: ops のテストの失敗を確かめる**

Run: `go test ./internal/ops -run 'TestStoreRemove|TestStoreInitListRemove' -count=1`（timeout 600000）
Expected: FAIL（`res.Deleted undefined` で、ビルドできない）

- [ ] **Step 3: ops を実装する**

`internal/ops/storeops.go`。559〜601 行（`StoreRemoveResult` と `StoreRemove`）を置き換える。

```go
// StoreRemoveResult is the store StoreRemove took out of the config, as
// store remove --json prints it. Deleted is whether the directory of the
// store was deleted.
type StoreRemoveResult struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Deleted bool   `json:"deleted"`
}

// StoreRemove takes the store name out of config.yaml and deletes its
// directory. With keep it only takes the store out; and a directory that is
// not there is not deleted either. The deletion is refused, with KindConflict
// "store.unpushed", when the tasks could not be got back: commits are not
// pushed, or there is no remote. The store lock is held from that check until
// the directory is gone.
func (ss *Session) StoreRemove(name string, keep bool) (StoreRemoveResult, error) {
	c, _, err := ss.Config()
	if err != nil {
		return StoreRemoveResult{}, err
	}
	e, err := c.Select(name)
	if err != nil {
		if errors.Is(err, config.ErrNoStores) {
			return StoreRemoveResult{}, err
		}
		return StoreRemoveResult{}, newErr(KindNotFound, "store.not_found", err, "name", name)
	}
	deleting := false
	if !keep {
		if _, serr := os.Stat(e.Path); !errors.Is(serr, fs.ErrNotExist) {
			l, err := ss.lockToDelete(e)
			if err != nil {
				return StoreRemoveResult{}, err
			}
			defer l.Release()
			deleting = true
		}
	}
	if err := ss.updateConfig(c, func(c *config.Config) error {
		if !storeExists(c, name) { // another writer may have removed it since the lookup
			return newErr(KindNotFound, "store.not_found", fmt.Errorf("no store is named %q", name), "name", name)
		}
		return c.RemoveStore(name)
	}); err != nil {
		return StoreRemoveResult{}, err
	}
	if deleting {
		if err := os.RemoveAll(e.Path); err != nil {
			return StoreRemoveResult{}, err
		}
	}
	return StoreRemoveResult{Name: name, Path: e.Path, Deleted: deleting}, nil
}
```

635〜665 行（`checkPushed`）を置き換える。

```go
// lockToDelete checks that deleting the directory of e loses nothing, and
// returns the lock of the store, which the caller lets go of once the
// directory is gone.
func (ss *Session) lockToDelete(e config.StoreEntry) (*store.Locked, error) {
	if len(e.Remotes) == 0 {
		return nil, newErr(KindConflict, "store.unpushed",
			fmt.Errorf("store %s has no remote, so removing %s would lose its tasks for good; use --keep to take the store out of the config and leave the directory, or delete the directory yourself if you do not need it", e.Name, e.Path),
			"name", e.Name)
	}
	s, err := ss.openStoreAt(e.Path)
	if err != nil {
		return nil, newErr(KindConflict, "store.unpushed",
			fmt.Errorf("store %s: %w; delete the directory by hand if you mean to", e.Name, err), "name", e.Name)
	}
	l, err := store.LockInOrder(s)
	if err != nil {
		return nil, classify(err)
	}
	if err := checkPushed(e, s); err != nil {
		l.Release()
		return nil, err
	}
	return l, nil
}

// checkPushed refuses a store whose commits are not all on its origin, or
// that has changes that are not committed.
func checkPushed(e config.StoreEntry, s *store.Store) error {
	unpushed := func(err error) error { return newErr(KindConflict, "store.unpushed", err, "name", e.Name) }
	n, err := Unpushed(s)
	if err != nil {
		return err
	}
	st, err := s.Repo().Status()
	if err != nil {
		return err
	}
	if len(st) > 0 {
		paths := make([]string, len(st))
		for i, f := range st {
			paths[i] = f.Path
		}
		return unpushed(fmt.Errorf("store %s has changes that are not committed (%s); commit or restore them, or delete the directory by hand if you mean to lose them", e.Name, strings.Join(paths, ", ")))
	}
	if n > 0 {
		return unpushed(fmt.Errorf("store %s has %d commits not pushed; run hedgehug sync --store %s first, or use --keep to leave the directory", e.Name, n, e.Name))
	}
	return nil
}
```

（`io/fs` と `internal/store` は `storeops.go` が import 済み。`ss.checkPushed` は他から呼ばれていない。）

- [ ] **Step 4: ops のテストの成功を確かめる**

Run: `go test ./internal/ops -count=1`（timeout 600000）
Expected: PASS（`StoreRemove` の呼び出しを直した他のテスト、`TestDoctorNoStores`・`TestStoreDefaultNoStore`・`TestPurgeMovedForwardToAStoreNotMounted` なども通る）

- [ ] **Step 5: CLI のテストを書き換える**

`cmd/hedgehug/store_add_test.go`。`TestStoreListAndRemove`・`TestStoreRemovePurge`・`TestStoreRemovePurgeWithoutRemote` の 3 つを置き換える。

```go
func TestStoreListAndRemove(t *testing.T) {
	t.Parallel()
	alice, _, _ := twoUsers(t)
	out := alice.ok("store", "list")
	if !strings.Contains(out, "org") || !strings.Contains(out, "shared.git") {
		t.Errorf("list = %s", out)
	}
	alice.ok("add", "unpushed")
	code, _, errs := alice.run("store", "remove", "org")
	if code != 1 || !strings.Contains(errs, "hedgehug sync --store org") || !strings.Contains(errs, "--keep") {
		t.Errorf("remove with unpushed commits: %d %s", code, errs)
	}
	if _, err := os.Stat(alice.storeDir("org")); err != nil {
		t.Errorf("a refused removal removed the directory: %v", err)
	}
	if out := alice.ok("store", "list"); !strings.Contains(out, "org") {
		t.Errorf("a refused removal took the store out of the config: %s", out)
	}
	if out := alice.ok("store", "remove", "org", "--keep"); out != "Removed store org from the config; "+alice.storeDir("org")+" is left\n" {
		t.Errorf("remove --keep = %q", out)
	}
	if out := alice.ok("store", "list"); strings.Contains(out, "org") {
		t.Errorf("list after remove = %s", out)
	}
	if _, err := os.Stat(alice.storeDir("org")); err != nil {
		t.Errorf("remove --keep removed the directory: %v", err)
	}
}

func TestStoreRemoveDeletes(t *testing.T) {
	t.Parallel()
	alice, _, _ := twoUsers(t)
	if out := alice.ok("store", "remove", "org"); out != "Removed store org and deleted "+alice.storeDir("org")+"\n" {
		t.Errorf("remove = %q", out)
	}
	if _, err := os.Stat(alice.storeDir("org")); !os.IsNotExist(err) {
		t.Errorf("the directory is left: %v", err)
	}
	if code, _, errs := alice.run("store", "remove", "org"); code != 1 || errs == "" {
		t.Errorf("remove again: code %d, stderr %s", code, errs)
	}
}

func TestStoreRemoveWithoutRemote(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	code, _, errs := te.run("store", "remove", "me")
	if code != 1 || !strings.Contains(errs, "no remote") || !strings.Contains(errs, "--keep") {
		t.Errorf("code %d, stderr %s", code, errs)
	}
	if _, err := os.Stat(te.storeDir("me")); err != nil {
		t.Errorf("a refused removal removed the directory: %v", err)
	}
	te.ok("store", "remove", "me", "--keep")
}

// --purge is gone: deleting is what remove does.
func TestStoreRemovePurgeFlagIsGone(t *testing.T) {
	t.Parallel()
	alice, _, _ := twoUsers(t)
	if code, _, errs := alice.run("store", "remove", "org", "--purge"); code != 2 || !strings.Contains(errs, "unknown flag: --purge") {
		t.Errorf("remove --purge: exit %d, stderr %q", code, errs)
	}
	if _, err := os.Stat(alice.storeDir("org")); err != nil {
		t.Errorf("a refused command removed the directory: %v", err)
	}
}
```

`cmd/hedgehug/store_more_test.go`。`TestStoreRemoveJSON` と `TestStoreRemovePurgeRefusesDirtyTree` を置き換える。

```go
func TestStoreRemoveJSON(t *testing.T) {
	t.Parallel()
	alice, _, _ := twoUsers(t)
	var got struct {
		Name    string `json:"name"`
		Deleted bool   `json:"deleted"`
	}
	out := alice.goldenOK("remove", "store", "remove", "org", "--json")
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.Name != "org" || !got.Deleted {
		t.Errorf("remove --json = %s, %v", out, err)
	}
}

func TestStoreRemoveRefusesDirtyTree(t *testing.T) {
	t.Parallel()
	alice, _, _ := twoUsers(t)
	dir := alice.storeDir("org")
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("hand edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errs := alice.run("store", "remove", "org")
	if code != 1 || !strings.Contains(errs, "not committed") || !strings.Contains(errs, "notes.md") {
		t.Errorf("code %d, stderr %s", code, errs)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("a refused removal removed the directory: %v", err)
	}
	if out := alice.ok("store", "list"); !strings.Contains(out, "org") {
		t.Errorf("a refused removal took the store out of the config: %s", out)
	}
}
```

golden `cmd/hedgehug/testdata/TestStoreRemoveJSON/remove.golden` を直す（`go test ./cmd/hedgehug -run TestStoreRemoveJSON -update` で作り直してよい。内容は次のとおり）。

```
{
  "name": "org",
  "path": "<TMP>/001/hh/stores/org",
  "deleted": true
}
```

remote のない Store を `store remove` で外している 3 か所は、既定の削除では拒否されるので `--keep` を付ける。

```go
// cmd/hedgehug/store_default_test.go:26
	te.ok("store", "remove", "org", "--keep")
// cmd/hedgehug/purge_moved_test.go:102 と :123
	te.ok("store", "remove", "bravo-long", "--keep")
```

（`purge_moved_test.go` は Task 11 で作り直す。ここでは呼び出しだけを直す。）

- [ ] **Step 6: CLI のテストの失敗を確かめる**

Run: `go test ./cmd/hedgehug -run 'TestStore' -count=1`（timeout 600000）
Expected: FAIL（`unknown flag: --keep`。`TestStoreRemovePurgeFlagIsGone` は `--purge` がまだ通って終了コード 0）

- [ ] **Step 7: CLI を実装する**

`cmd/hedgehug/store_add.go`。`storeRemoveCmd` と `storeRemove`（77〜105 行）を置き換える。

```go
func (a *app) storeRemoveCmd() *cobra.Command {
	var keep bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a store and delete its directory",
		Long: `Remove takes the store out of config.yaml and deletes its directory.

It refuses when the store has commits that are not pushed, or has no remote: the tasks could not be got back. Run hedgehug sync --store <name> first. With --keep it only takes the store out of config.yaml and leaves the directory, and checks nothing.`,
		Args: exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error { return a.storeRemove(args[0], keep) },
	}
	cmd.Flags().BoolVar(&keep, "keep", false, "take the store out of the config and leave its directory")
	return cmd
}

func (a *app) storeRemove(name string, keep bool) error {
	r, err := a.ss.StoreRemove(name, keep)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(r)
	}
	switch {
	case r.Deleted:
		_, err = fmt.Fprintf(a.stdout, "Removed store %s and deleted %s\n", name, r.Path)
	case keep:
		_, err = fmt.Fprintf(a.stdout, "Removed store %s from the config; %s is left\n", name, r.Path)
	default:
		_, err = fmt.Fprintf(a.stdout, "Removed store %s from the config; %s was not there\n", name, r.Path)
	}
	return err
}
```

- [ ] **Step 8: 成功を確かめて、コミットする**

Run: `go vet ./... && go test ./internal/ops ./cmd/hedgehug -run 'TestStore|TestPurgeMoved|TestDoctorForwardTarget' -count=1`（timeout 600000）
Expected: PASS。続けて `grep -rn -- '--purge' --include='*.go' .` が `purge --rewrite-history` と `purge --moved` の行（Task 10・11 が消す）以外を出さないこと

```bash
git add internal/ops/storeops.go cmd/hedgehug/store_add.go cmd/hedgehug/testdata/TestStoreRemoveJSON/remove.golden internal/ops/storeops_test.go internal/ops/config_recheck_test.go internal/ops/purgemoved_test.go internal/ops/duplicates_test.go internal/ops/maintain_test.go internal/ops/auth_test.go cmd/hedgehug/store_add_test.go cmd/hedgehug/store_more_test.go cmd/hedgehug/store_default_test.go cmd/hedgehug/purge_moved_test.go
git commit -m "$(cat <<'EOF'
feat(cli): store remove が既定でディレクトリも消すようにし、--keep を足して --purge をやめる

消すとタスクが戻らない場合（push していないコミット、remote なし）は拒否する。調べてから消すまでの間、Store のロックを持つ。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

#### spec にない細部（Task 7）

1. `--json` の項目を `purged` から `deleted` に改名する。`--purge` を廃止するので、`purged` という名前の根拠がなくなる。意味は「ディレクトリを消したか」。
2. ディレクトリがすでにない Store は、remote の有無に関係なく config.yaml から外すだけ（spec の表は最後の行に書くが、remote なしの行より優先する。今は remote がなければ、ディレクトリがなくても拒否していた）。
3. ディレクトリがもうなかったときの出力は `Removed store <name> from the config; <path> was not there`（spec は「今と同じ」。今は `is left` と出すが、ないものを「残した」とは言えない）。
4. spec は「ロックを取ってから消す（今の `--purge` と同じ）」と書くが、今の `--purge` はロックを取っていない。調べてから消すまでの間に書き込みが入らないよう、spec の意図に合わせて、確認の前から消し終わるまで Store のロックを持つ。
5. remote なしの拒否の文言は、spec の旨に沿って `store <name> has no remote, so removing <path> would lose its tasks for good; use --keep to take the store out of the config and leave the directory, or delete the directory yourself if you do not need it` にした。push していないコミットの文は、今の文に `, or use --keep to leave the directory` を足しただけ。コミットしていない変更と、Store が開けない場合の文は変えない（どちらも元から「手で消す」を案内している）。
6. `StoreRemove` の引数は `purge` から `keep` に変わり、真偽が反転する。既存の呼び出し 6 か所（`false` で「残す」だったもの）は `true` に直す。拒否の確認に使っていた `true` の 6 か所は `false` に直す。

### Task 8: `add --repo` と git worktree の中のリポジトリ

spec §7・§8。`hh add` は既定でカレントのリポジトリを `repos` に書かず、`--repo` を付けたときだけ書く。`--no-repo` は廃止する。入れ先の決め方（コンテキスト解決の 4 段目）は変えない。もう 1 つ、git worktree の中でもカレントのリポジトリを見つけられるようにする。

**Files:**

- Modify: `internal/context/resolve.go`（`Input.NeedRepo` を足す。`Resolve` で repo ID が取れなければ使い方の誤りにする。`openRepo` に `EnableDotGitCommonDir: true`）
- Modify: `internal/ops/addhere.go`（`AddHereInput.NoRepo` → `Repo`、既定で書かない）
- Modify: `cmd/hedgehug/add.go`（`--no-repo` を消し、`--repo` を足す）
- Modify（`--no-repo` の参照を消す）: `internal/ops/addhere_test.go:100-113`、`internal/ops/addhere_fix_test.go:12-29`、`cmd/hedgehug/add_context_test.go:35-58`、`cmd/hedgehug/add_golden_test.go:17-33`、`cmd/hedgehug/e2e_3b_test.go:159-190`
- Delete（ゴールデン）: `cmd/hedgehug/testdata/TestAddContextGolden/no-repo.out.golden`・`no-repo.err.golden`（`go test -update` で `linked` が作られたあとで消す）
- Modify（ゴールデン。`-update` で作り直す）: `cmd/hedgehug/testdata/TestAddContextGolden/` の各ファイル
- Create: `internal/context/resolve_worktree_test.go`、`cmd/hedgehug/add_worktree_test.go`
- Test: `internal/context/resolve_test.go`（`NeedRepo` のテストを足す）、`internal/ops/addhere_test.go`、`cmd/hedgehug/add_context_test.go`

`--no-repo` の参照の洗い出し（実装前に同じ grep を回して、一覧との差がないか確かめる）:

```bash
grep -rn 'NoRepo\|no-repo\|noRepo' cmd internal web tools -I | grep -v node_modules
```

出てくるのは `cmd/hedgehug/add.go:18,31,43`、`internal/ops/addhere.go:16,28,80`、上の 5 つのテスト（`internal/ops/addhere_test.go:107,112`、`internal/ops/addhere_fix_test.go:12,20,25`、`cmd/hedgehug/add_context_test.go:53-56`、`cmd/hedgehug/add_golden_test.go:24`、`cmd/hedgehug/e2e_3b_test.go:187-188`）。docs の `--no-repo`（phase3 の spec、親設計）は Task 13 で直す。

既定で書かなくなると、「リポジトリにひも付いたプロジェクト」を作るテストは、プロジェクトに `--repo` を付けないと 4 段目で見つけられなくなる。影響するのは `cloneDir` を使う `cmd/hedgehug/add_context_test.go`・`add_golden_test.go` と、clone で `add` する `cmd/hedgehug/e2e_3b_test.go`。`tools/devenv` はプロジェクトを `hedgehug add` でなく Store に直接書くので、変えない。GUI の作成は `ops.Add` を直接呼ぶので変わらない（spec §7）。

**Interfaces:**

- Consumes: `hhctx.Resolve`（`internal/context/resolve.go:143`）、`repoIDsOf`、`ops.newErr`・`KindUsage`、本物の git を使う既存のテストの skip の仕方（`cmd/hedgehug/merge_driver_test.go:30`: `exec.LookPath("git")` が失敗したら `t.Skip("git is not installed")`）、テスト用の git の環境（同ファイル 17-18 行: `GIT_CONFIG_GLOBAL=os.DevNull`、`GIT_CONFIG_NOSYSTEM=1`）
- Produces: `hhctx.Input.NeedRepo bool`（`true` で repo ID が取れなければ `Resolve` が `KindUsage`・コード `add.repo_unavailable` で失敗する。質問より前に判定する）、`ops.AddHereInput.Repo bool`（`--repo`）、`hh add --repo`。`--repo` でリポジトリがないときの文言: `--repo needs the current directory to be in a git repository with a remote that gives a repo ID`（終了コード 2）。`--no-repo` は cobra の `unknown flag: --no-repo`（終了コード 2）

- [ ] **Step 1: 失敗するテストを書く（context）**

`internal/context/resolve_test.go` の `TestRepoIDIsSetWhateverStageDecided` の後に足す。

```go
func TestNeedRepoFailsBeforeAnyQuestion(t *testing.T) {
	t.Parallel()
	_, set := newWorld(t)
	// not a repository, a repository without a remote, and a remote that gives no repo ID
	for name, dir := range map[string]string{
		"no repository": t.TempDir(),
		"no remote":     gitDir(t, ""),
		"no repo id":    gitDir(t, "", "origin", "/tmp/x.git"),
		"no directory":  "",
	} {
		_, cf, ch, _, err := resolve(t, set, State{}, Input{Dir: dir, Interactive: true, NeedRepo: true})
		if cf != nil || ch != nil {
			t.Errorf("%s: asked %v %v before failing", name, cf, ch)
		}
		wantKind(t, err, KindUsage, "add.repo_unavailable")
	}
	// with a repo ID it is not an error, and the stages are the same as without
	dir := gitDir(t, "", "origin", "https://github.com/o/r.git")
	d := mustDest(t, set, State{}, Input{Dir: dir, Parent: "WEB-1", NeedRepo: true})
	if d.Stage != 1 || d.RepoID != "github.com/o/r" {
		t.Errorf("stage %d RepoID %q", d.Stage, d.RepoID)
	}
}
```

`/tmp/x.git` は `RepoID` が repo ID を作れない入力（`internal/context/repoid_test.go:26`）。

- [ ] **Step 2: 失敗するテストを書く（worktree。本物の git）**

`internal/context/resolve_worktree_test.go` を作る。

```go
package context

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// worktreeDir makes a repository with origin url and one empty commit, adds a
// git worktree of it on a new branch with the real git, and returns the
// directory of the worktree. The test is skipped without git.
func worktreeDir(t *testing.T, branch, url string) string {
	t.Helper()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	main := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(gitBin, append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = main
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1") // not the settings of whoever runs the test
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("remote", "add", "origin", url)
	run("commit", "-q", "--allow-empty", "-m", "init")
	dir := filepath.Join(t.TempDir(), "wt")
	run("worktree", "add", "-q", "-b", branch, dir)
	return dir
}

// A worktree has a .git file that points into the .git of the main
// repository, where the remotes are. The repo ID comes from there and the
// branch from the HEAD of the worktree.
func TestWorktreeFindsTheRepoAndItsOwnBranch(t *testing.T) {
	t.Parallel()
	w, set := newWorld(t)
	p := put(t, w.a, "Web release", project("github.com/org-a/web"))
	set = openSet(t, w.a, w.b)
	dir := worktreeDir(t, "hh/WEB-1-login", "https://github.com/Org-A/Web.git")
	if fi, err := os.Stat(filepath.Join(dir, ".git")); err != nil || fi.IsDir() {
		t.Fatalf(".git of the worktree = %v, %v; want a file", fi, err)
	}

	// stage 3: the branch of the worktree names WEB-1
	_, cf, _, _, err := resolve(t, set, State{}, Input{Dir: dir, Interactive: true})
	if err != nil || cf == nil || cf.Parent.ULID != w.web1.ULID {
		t.Fatalf("Confirm = %+v, %v; want the task WEB-1 of the branch", cf, err)
	}
	// stage 4 (the branch is declined): the repo ID of origin finds the project
	d := mustDest(t, set, State{}, Input{Dir: dir, Interactive: true, BranchParent: No})
	if d.Stage != 4 || d.Parent == nil || d.Parent.ULID != p.ULID || d.RepoID != "github.com/org-a/web" {
		t.Errorf("destination = stage %d parent %v repo %q; want stage 4, the project, github.com/org-a/web", d.Stage, d.Parent, d.RepoID)
	}
}
```

`put`・`project`・`openSet`・`newWorld`・`resolve`・`mustDest` は `resolve_test.go` のヘルパー。`w.a` の project を足したあとに `set` を作り直すのは、`TestRepoStage` と同じ手順。

- [ ] **Step 3: 失敗するテストを書く（ops）**

`internal/ops/addhere_test.go` の `TestAddHerePutsTheRepoOnTheTask` を置き換える（関数全体）。

```go
func TestAddHereRepoPutsTheRepoOnTheTask(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	dir := repoDir(t, "", "https://github.com/Org-A/Web.git")
	// without Repo the repository is not put on the task
	r, _, _, err := ts.AddHere(AddHereInput{Context: hhctx.Input{Dir: dir, Store: "org-a"}, Task: AddInput{Title: "unlinked"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := reposOf(t, ts, "org-a", r.ULID); len(got) != 0 {
		t.Errorf("without Repo: repos = %q", got)
	}
	r, _, _, err = ts.AddHere(AddHereInput{Context: hhctx.Input{Dir: dir, Store: "org-a"}, Repo: true, Task: AddInput{Title: "linked"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := reposOf(t, ts, "org-a", r.ULID); len(got) != 1 || got[0] != "github.com/org-a/web" {
		t.Errorf("Repo: repos = %q; want the repo ID", got)
	}
	// the repo ID goes first, and is not repeated when the task has it
	r, _, _, err = ts.AddHere(AddHereInput{Context: hhctx.Input{Dir: dir, Store: "org-a"}, Repo: true,
		Task: AddInput{Title: "both", Repos: []string{"github.com/o/other", "github.com/org-a/web"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := reposOf(t, ts, "org-a", r.ULID); len(got) != 2 || got[0] != "github.com/o/other" || got[1] != "github.com/org-a/web" {
		t.Errorf("Repo with repos given: repos = %q", got)
	}
	// Repo outside a repository is a usage error
	_, _, _, err = ts.AddHere(AddHereInput{Context: hhctx.Input{Dir: t.TempDir(), Store: "org-a"}, Repo: true, Task: AddInput{Title: "bare"}})
	wantCode(t, "Repo outside a repository", err, KindUsage, "add.repo_unavailable")
	// no repository, no link
	r, _, _, err = ts.AddHere(AddHereInput{Context: hhctx.Input{Dir: t.TempDir(), Store: "org-a"}, Task: AddInput{Title: "bare"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := reposOf(t, ts, "org-a", r.ULID); len(got) != 0 {
		t.Errorf("no repository: repos = %q", got)
	}
}
```

`internal/ops/addhere_fix_test.go` の `TestAddHereNoRepoStillSetsRecent` を書き換える。

変更前:

```go
func TestAddHereNoRepoStillSetsRecent(t *testing.T) {
```

変更後:

```go
func TestAddHereWithoutRepoStillSetsRecent(t *testing.T) {
```

変更前:

```go
	r, _, _, err := ts.AddHere(AddHereInput{Context: hhctx.Input{Dir: dir}, NoRepo: true, Task: AddInput{Title: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(reposOf(t, ts, "org-a", r.ULID)) != 0 {
		t.Errorf("--no-repo put a repo on the task")
	}
```

変更後:

```go
	r, _, _, err := ts.AddHere(AddHereInput{Context: hhctx.Input{Dir: dir}, Task: AddInput{Title: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(reposOf(t, ts, "org-a", r.ULID)) != 0 {
		t.Errorf("an add without Repo put a repo on the task")
	}
```

- [ ] **Step 4: 失敗するテストを書く（CLI）**

`cmd/hedgehug/add_context_test.go` の `TestAddInACloneLinksTheRepoAndGoesUnderItsProject`（35-67 行目付近の関数全体）を置き換える。

```go
func TestAddInACloneGoesUnderItsProjectAndLinksTheRepoOnlyWithRepo(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.cloneDir("", "https://github.com/Org-A/Web.git")
	// the project is found by the repo on it, so it is added with --repo
	p := te.addJSON("Web release", "--kind", "project", "--repo")
	if got := te.task(p.ULID).Repos; len(got) != 1 || got[0] != "github.com/org-a/web" {
		t.Fatalf("project repos = %q; want the repo ID of origin", got)
	}
	out := te.ok("add", "login page")
	if !strings.HasSuffix(out, "→ added to me / Web release\n") {
		t.Errorf("stdout = %q", out)
	}
	// without --repo the place is the same and the repository is not on the task
	got := te.addJSON("again")
	if got.Store != "me" || got.Parent == nil || *got.Parent != p.ULID || got.Workspace == nil || got.Workspace.Name != "me" {
		t.Errorf("task = %+v; want store me, parent %s, workspace me", got, p.ULID)
	}
	if repos := te.task(got.ULID).Repos; len(repos) != 0 {
		t.Errorf("without --repo: repos = %q", repos)
	}
	// --repo puts the repository first
	l := te.addJSON("linked", "--repo", "-p", "repos=github.com/o/other")
	if repos := te.task(l.ULID).Repos; len(repos) != 2 || repos[0] != "github.com/org-a/web" || repos[1] != "github.com/o/other" {
		t.Errorf("--repo: repos = %q; want the repository first, then the one given", repos)
	}
	// an explicit workspace is the root, whatever the repository says
	root := te.addJSON("elsewhere", "--workspace", "me")
	if root.Parent != nil {
		t.Errorf("--workspace: parent = %v; want none (the root)", root.Parent)
	}
	if out := te.ok("add", "root line", "--workspace", "me"); !strings.HasSuffix(out, "→ added to me / me\n") {
		t.Errorf("stdout = %q", out)
	}
}

func TestAddRepoNeedsARepositoryAndNoRepoIsGone(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	before := len(te.log("me"))
	// no current repository
	code, _, errs := te.run("add", "x", "--repo")
	if code != 2 || !strings.Contains(errs, "--repo needs the current directory to be in a git repository with a remote that gives a repo ID") {
		t.Errorf("--repo outside a repository: exit %d, stderr %q", code, errs)
	}
	// a repository without a remote
	te.cloneDir("", "")
	code, _, errs = te.run("add", "x", "--repo")
	if code != 2 || !strings.Contains(errs, "--repo needs the current directory") {
		t.Errorf("--repo in a repository without a remote: exit %d, stderr %q", code, errs)
	}
	// --no-repo was removed
	code, _, errs = te.run("add", "x", "--no-repo")
	if code != 2 || !strings.Contains(errs, "unknown flag: --no-repo") {
		t.Errorf("--no-repo: exit %d, stderr %q; want 2 and an unknown flag", code, errs)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d: nothing is written", n, before)
	}
}
```

`te.cloneDir("", "")` は `CreateRemote` に空の URL を渡して失敗する。`cloneDir`（`add_context_test.go:16-33`）の `CreateRemote` を `url != ""` のときだけ呼ぶように直す（`internal/ops/addhere_test.go` の `repoDir` と同じ形）。

変更前:

```go
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{url}}); err != nil {
		te.t.Fatal(err)
	}
	if branch != "" {
```

変更後:

```go
	if url != "" {
		if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{url}}); err != nil {
			te.t.Fatal(err)
		}
	}
	if branch != "" {
```

`cmd/hedgehug/add_worktree_test.go` を作る。

```go
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// worktreeDir makes a repository with origin url and one empty commit, adds a
// worktree of it with the real git, and makes the worktree the current
// directory of the runs. The test is skipped without git.
func (te *testEnv) worktreeDir(branch, url string) {
	te.t.Helper()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		te.t.Skip("git is not installed")
	}
	main := te.t.TempDir()
	run := func(args ...string) {
		te.t.Helper()
		cmd := exec.Command(gitBin, append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = main
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1") // not the settings of whoever runs the test
		if out, err := cmd.CombinedOutput(); err != nil {
			te.t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("remote", "add", "origin", url)
	run("commit", "-q", "--allow-empty", "-m", "init")
	dir := filepath.Join(te.t.TempDir(), "wt")
	run("worktree", "add", "-q", "-b", branch, dir)
	te.dir = dir
}

func TestAddInAWorktreeFindsTheRepo(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.worktreeDir("feature", "https://github.com/Org-A/Web.git")
	p := te.addJSON("Web release", "--kind", "project", "--repo")
	if got := te.task(p.ULID).Repos; len(got) != 1 || got[0] != "github.com/org-a/web" {
		t.Fatalf("project repos = %q; want the repo ID of origin, read through the worktree", got)
	}
	out := te.ok("add", "from the worktree")
	if !strings.HasSuffix(out, "→ added to me / Web release\n") {
		t.Errorf("stdout = %q; want the project found by the repo ID", out)
	}
}
```

- [ ] **Step 5: 失敗を確かめる**

Run（timeout 600000）: `go test ./internal/context ./internal/ops ./cmd/hedgehug -run 'NeedRepo|Worktree|AddHereRepo|AddHereWithoutRepo|TestAddInAClone|TestAddRepoNeeds' -count=1`
Expected: ビルド失敗（`NeedRepo`・`Repo` が未定義、`cloneDir` の修正前は `--repo` が未定義）。ビルドが通ったあとは、worktree のテストは「repo ID が取れない」で FAIL（`git` がなければ SKIP）。

- [ ] **Step 6: `context` を直す**

`internal/context/resolve.go` の `Input` に足す。

変更前:

```go
	Interactive                       bool   // stdin is a terminal: stages 3–5 may ask
```

変更後:

```go
	Interactive                       bool   // stdin is a terminal: stages 3–5 may ask
	NeedRepo                          bool   // --repo: fail unless the current directory has a repo ID
```

`Resolve` の `ids := repoIDsOf(repo)` の直後に足す。

変更前:

```go
	ids := repoIDsOf(repo)
	stages := []func(
```

変更後:

```go
	ids := repoIDsOf(repo)
	if in.NeedRepo && len(ids) == 0 {
		return nil, nil, nil, newError(KindUsage, "add.repo_unavailable",
			errors.New("--repo needs the current directory to be in a git repository with a remote that gives a repo ID"))
	}
	stages := []func(
```

`openRepo` を直す。

変更前:

```go
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
```

変更後:

```go
	// A worktree's .git is a file that points into the .git of the main
	// repository, where the remotes are: without the common dir the
	// repository looks like one with no remote. HEAD is the worktree's own.
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true, EnableDotGitCommonDir: true})
```

- [ ] **Step 7: `ops` を直す**

`internal/ops/addhere.go` を直す。

変更前:

```go
type AddHereInput struct {
	Context hhctx.Input
	NoRepo  bool
	Task    AddInput // Store, Parent and Workspace are set from the destination
}
```

変更後:

```go
type AddHereInput struct {
	Context hhctx.Input
	Repo    bool     // put the repo ID of the current repository on the task
	Task    AddInput // Store, Parent and Workspace are set from the destination
}
```

変更前:

```go
// repo ID of the current repository unless NoRepo, and sets recent when
```

変更後:

```go
// repo ID of the current repository when Repo, and sets recent when
```

変更前:

```go
	ctx := in.Context
	if ctx.Store == "" {
```

変更後:

```go
	ctx := in.Context
	ctx.NeedRepo = in.Repo
	if ctx.Store == "" {
```

変更前:

```go
	if dest.RepoID != "" && !in.NoRepo && !slices.Contains(task.Repos, dest.RepoID) {
```

変更後:

```go
	if in.Repo && !slices.Contains(task.Repos, dest.RepoID) {
```

`Repo` のとき `dest.RepoID` が空になることはない（`Resolve` が `NeedRepo` で先に失敗する）。

- [ ] **Step 8: `add` を直す**

`cmd/hedgehug/add.go` を直す。

変更前:

```go
	var noRepo bool
```

変更後:

```go
	var withRepo bool
```

変更前:

```go
				NoRepo:  noRepo,
```

変更後:

```go
				Repo:    withRepo,
```

変更前:

```go
	f.BoolVar(&noRepo, "no-repo", false, "do not put the current repository on the task")
```

変更後:

```go
	f.BoolVar(&withRepo, "repo", false, "put the current repository on the task (a repo ID from its origin)")
```

- [ ] **Step 9: `--no-repo` を使っているほかのテストを直す**

`cmd/hedgehug/add_golden_test.go` の `TestAddContextGolden` を直す。プロジェクトはリポジトリにひも付けるので `--repo` を付ける。`no-repo` の項目は `--repo` を付けた `linked` にする。

変更前:

```go
	te.goldenRun("project", 0, "add", "Web A", "--kind", "project")
	te.goldenRun("second-project", 0, "add", "Web B", "--kind", "project", "--workspace", "me")
	te.goldenRun("choice-required", 2, "add", "ambiguous")
	te.goldenRun("no-repo", 0, "add", "unlinked", "--no-repo", "--workspace", "me")
```

変更後:

```go
	te.goldenRun("project", 0, "add", "Web A", "--kind", "project", "--repo")
	te.goldenRun("second-project", 0, "add", "Web B", "--kind", "project", "--workspace", "me", "--repo")
	te.goldenRun("choice-required", 2, "add", "ambiguous")
	te.goldenRun("with-repo", 0, "add", "linked", "--repo", "--workspace", "me")
```

`cmd/hedgehug/e2e_3b_test.go` を直す。159-160 行目（`x`）はリポジトリを付けて、168 行目の検査を残す。

変更前:

```go
	out, _ = alice.okIn(web, "add", "x")
```

変更後:

```go
	out, _ = alice.okIn(web, "add", "x", "--repo")
```

187-189 行目を置き換える。

変更前:

```go
	if out, _ := alice.okIn(web, "add", "z", "--no-repo"); !strings.Contains(out, "added to org-a / Web release") {
		t.Errorf("add --no-repo in the clone:\n%s", out)
	}
```

変更後:

```go
	// without --repo the task goes to the same place and has no repo on it
	var plain taskInfo
	out, _ = alice.okIn(web, "add", "z", "--json")
	if err := json.Unmarshal([]byte(out), &plain); err != nil || plain.Parent != projectULID {
		t.Errorf("add without --repo in the clone: %v %q (project %s)", err, out, projectULID)
	} else if x := taskOf(t, alice, plain.ULID); len(x.Repos) != 0 {
		t.Errorf("a task added in the clone without --repo has repos %q", x.Repos)
	}
```

- [ ] **Step 10: ゴールデンを作り直し、通す**

Run（timeout 600000）: `go test ./cmd/hedgehug -run 'TestAddContextGolden' -count=1 -update`

`git status cmd/hedgehug/testdata/TestAddContextGolden` で、`with-repo.out.golden`・`with-repo.err.golden` が増え、`no-repo.*.golden` が残っていることを確かめる。`no-repo.out.golden`・`no-repo.err.golden` を `git rm` する。`git diff cmd/hedgehug/testdata/TestAddContextGolden` で、`project.out.golden` などの変更が ID の採番の変化（`with-repo` が `no-repo` の位置に入る）だけであることを確かめる。

Run（timeout 600000）: `go build ./... && go vet ./...`
Expected: エラーなし。

Run（timeout 600000）: `go test ./internal/context ./internal/ops -count=1 && go test ./cmd/hedgehug -run 'TestAdd|TestE2EPhase3b|TestUse' -count=1`
Expected: PASS。`git` がない環境では `TestWorktreeFindsTheRepoAndItsOwnBranch`・`TestAddInAWorktreeFindsTheRepo` が SKIP になる。このマシンには git があるので、2 つとも PASS（SKIP でない）ことを `-v` で確かめる。

worktree のテストで、ブランチ（3 段目）の検査だけが落ちる場合（repo ID は取れるが `HEAD` が本体のものに見える）は、実装を止めて報告する（spec §8 は「ブランチ名は worktree 側の `HEAD` から読む」と書いており、go-git の挙動が違えば設計の見直しになる）。

- [ ] **Step 11: コミット**

```bash
git add internal/context internal/ops cmd/hedgehug
git commit -m "feat(cli): add --repo でだけカレントのリポジトリを付け、worktree の中でもリポジトリを見つける

hh add は既定でリポジトリを repos に書かず、--repo を付けたときだけ書く。
--no-repo は廃止する。--repo でリポジトリが見つからないときは、質問の前に
使い方の誤りにする。git worktree の .git は本体の .git を指すファイルなので、
go-git に EnableDotGitCommonDir を付けて本体の remote を読む。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

#### spec にない細部

1. `--repo` でリポジトリがないときの失敗を `Resolve` の中（`Input.NeedRepo`）で、質問より前に返す。コードは `add.repo_unavailable`、文言は `--repo needs the current directory to be in a git repository with a remote that gives a repo ID`。なぜ: spec は「使い方の誤り（終了コード 2）で知らせる」とだけ書く。`AddHere` の最後で調べると、利用者が入れ先の質問に答えてから失敗する。
2. repo ID が取れない例として、リポジトリでない・リポジトリだが remote がない・remote の URL から repo ID を作れない・カレントがない（`Dir` が空）の 4 つをすべて同じ失敗にする。なぜ: spec は「カレントがリポジトリでない、または remote から repo ID を作れないとき」と書く。remote がないのは後者に含める。
3. `--repo` と `-p repos=…` を同時に渡したとき、リポジトリの repo ID を `repos` の先頭に足す（すでに入っていれば足さない）。なぜ: spec は「`repos` の先頭に足す（今の既定の動きと同じ）」で、今の `AddHere` もそうしている。
4. worktree のテストは `internal/context` に本物の git で作る 1 つ（repo ID とブランチ名）と、`cmd/hedgehug` に 1 つ（`add` が入れ先を見つける）を置く。skip は既存の `merge_driver_test.go` と同じ（`exec.LookPath("git")` が失敗したら `t.Skip`）。

### Task 9: GUI と API の完全削除を外す

spec §11。`DELETE /api/tasks/{id}` から `purge`・`confirm` を外し、API・生成物・`web/core`・phase4 の spec から完全削除を消す。CLI の `purge` の形（`purge <id>`・`--moved`）は Task 11 で作り直すので、ここでは触らない。`ops.Session.Purge`・`PurgeTargets`・`PurgeUnchecked` と、そのエラーコード（`purge.changed`・`purge.child_not_deleted`）は CLI の `purge <id>` がまだ使うので残す。

**消す参照の洗い出し（`grep -rn -i 'purge'` を `internal/server`・`api`・`web` に対して取った結果。`cmd/`・`internal/ops` は Task 11 の範囲）:**

| 場所 | 消すもの |
| --- | --- |
| `internal/server/tasks_write.go` | `affectedWrite.ULIDs`、`deleteInput`、`delete-task` の `in.Purge` の分岐、`purge`・`purgeFailed`・`unconfirmed`・`sameULIDs`、import の `strings`・`internal/index` |
| `internal/server/tasks_write_test.go` | `TestPurgeNeedsConfirm`・`TestPurgeLiveTaskIsConflict`・`TestPurgeConfirmNotAULID`・`TestPurgeFailed`、`affectedBody.ULIDs`、import の `strings`・`internal/index` |
| `api/openapi.yaml` | `delete-task` の `purge`・`confirm` のクエリ、`AffectedWrite` の `ulids`、summary（`go generate ./internal/server` で作り直す） |
| `web/core/api/schema.d.ts` | 同上（`cd web && pnpm gen` で作り直す） |
| `web/core/cli.ts`・`cli.test.ts` | `cliPurge` と、その import・期待値 |
| `web/core/messages.ts`・`messages.test.ts` | `purge.changed`・`purge.child_not_deleted`・`purge.unconfirmed` と、`purge.unconfirmed` のテスト |
| `web/core/api/client.ts` | コメントの `(purge.unconfirmed lists its targets)` |
| `web/core/cache.ts`・`cache.test.ts` | `applyWrite` の `ulids`（完全削除の答えを取り込む引数。`DELETE` の答えに `ulids` がなくなるので使い道がない）と、それを使うテスト 3 つ |
| `docs/superpowers/specs/2026-11-01-phase4-gui-design.md` | タスク詳細・Dialogs の「完全削除」、API の表の「完全削除」の行。4e の範囲に期間を指定した purge を足す |

`cache.test.ts` の `changed("shared", [], "c2", "purge", ["A"])`（CLI の purge が `removed` で伝える）と `poll.go` の `Removed` の説明は、CLI の purge が残るので消さない。

**Files:**
- Modify: `internal/server/tasks_write.go`（`affectedWrite` 163 行付近、`deleteInput` 229–233 行、`delete-task` の登録 316–331 行、`purge` 以下 350 行〜末尾）
- Test: `internal/server/tasks_write_test.go`（`affectedBody` 20–24 行、`TestPurgeNeedsConfirm` 135 行〜`TestPurgeLiveTaskIsConflict` 174 行、`TestPurgeConfirmNotAULID`・`TestPurgeFailed` 215–246 行）
- Modify（生成）: `api/openapi.yaml`、`web/core/api/schema.d.ts`
- Modify: `web/core/cli.ts`（83 行）、`web/core/cli.test.ts`（7・103 行）、`web/core/messages.ts`（74–76・111–120・128–129 行）、`web/core/messages.test.ts`（68–71 行）、`web/core/api/client.ts`（12 行）、`web/core/cache.ts`（199–219 行）、`web/core/cache.test.ts`（231–239・316–347 行）
- Modify: `docs/superpowers/specs/2026-11-01-phase4-gui-design.md`（18・50・68・74・79・210 行）

**Interfaces:**
- Consumes: Task 1〜8 の成果は使わない（`DELETE` の通常の削除 `ss.Remove` はそのまま）。
- Produces: `DELETE /api/tasks/{id}` は削除（`hh rm`）だけ。応答は `{tasks, commit}`（`ulids` はなくなる）。`?purge=true` は未知のクエリとして無視される。`web/core` から `cliPurge` と `purge.*` のメッセージが消える。Task 11 は GUI 側を気にせず `purge` を作り直せる。

- [ ] **Step 1: 失敗するテストを書く**

`internal/server/tasks_write_test.go` の `TestPurgeNeedsConfirm` の直前に足す（このあと次のステップで `TestPurge*` は消す）。

```go
// The API has no purge: ?purge=true is not a parameter, so the call is the
// plain delete of hh rm and the task stays, deleted.
func TestDeleteTaskHasNoPurge(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	p := ts.add("p", "")
	var out affectedBody
	ts.call("DELETE", "/api/tasks/"+p+"?purge=true", nil, 200, &out)
	if len(out.Tasks) != 1 || out.Tasks[0].DeletedAt == nil || out.Commit == nil {
		t.Fatalf("delete = %+v, want the task deleted", out)
	}
	var got ops.TaskView
	ts.call("GET", "/api/tasks/"+p, nil, 200, &got)
	if got.DeletedAt == nil {
		t.Errorf("the task is %+v, want it still there, deleted", got)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/server -run TestDeleteTaskHasNoPurge -count=1`（timeout 600000）
Expected: FAIL。`DELETE …?purge=true: status 409, want 200`（`purge.unconfirmed`）。

- [ ] **Step 3: サーバーを直す**

`internal/server/tasks_write.go`:

(a) `affectedWrite` から `ULIDs` を消す。

```go
type affectedWrite struct {
	Tasks  []*ops.TaskView `json:"tasks,omitempty" doc:"The task and the tasks under it, as they are now"`
	Commit *string         `json:"commit"`
}
```

(b) `deleteInput` の型を消す（229–233 行の 5 行）。

```go
type deleteInput struct {
	TaskParam
	Purge   bool   `query:"purge" doc:"Remove the files for good; the task must be deleted already"`
	Confirm string `query:"confirm" doc:"With purge: the ULIDs the user agreed to purge, comma-separated, as purge.unconfirmed listed them"`
}
```

(c) `delete-task` の登録を次に置き換える。

```go
	huma.Register(s.api, huma.Operation{
		OperationID: "delete-task", Method: http.MethodDelete, Path: "/api/tasks/{id}",
		Summary: "Delete a task and the tasks under it (hh rm)", Tags: []string{"tasks"},
	}, func(_ context.Context, in *TaskParam) (*affectedOutput, error) {
		ss := s.session(in.Store)
		res, err := ss.Remove(in.ID)
		if err != nil {
			return nil, problemOf(err)
		}
		return s.affectedOut(ss, res)
	})
```

(d) ファイルの末尾、`// purge purges the task id after the confirmation:` のコメントから `sameULIDs` の閉じ括弧まで（350 行〜末尾）を消す。

(e) import から `"strings"` と `"github.com/kukv/hedgehug/internal/index"` を消す（残りの `slices`・`model`・`fmt`・`errors`・`strconv` は使われている）。

- [ ] **Step 4: 完全削除のテストを消す**

`internal/server/tasks_write_test.go`:
- `affectedBody` から `ULIDs  []string        \`json:"ulids"\`` の行を消す。
- `TestPurgeNeedsConfirm`、`TestPurgeLiveTaskIsConflict`、`TestPurgeConfirmNotAULID`、`TestPurgeFailed` を消す。
- import から `"strings"` と `"github.com/kukv/hedgehug/internal/index"` を消す（`errors`・`slices` は残る）。

- [ ] **Step 5: 成功を確かめ、`api/openapi.yaml` を作り直す**

```bash
go build ./... && go vet ./internal/server
go generate ./internal/server
```

`git diff api/openapi.yaml` が、`delete-task` の `purge`・`confirm` の 2 つのクエリ、`AffectedWrite` の `ulids`、summary の `, or purge them (hh purge)` だけの差であることを確かめる。

Run: `go test ./internal/server -count=1`（timeout 600000）
Expected: PASS（`TestDeleteTaskHasNoPurge` も、`api/openapi.yaml` が生成物と一致する検査も）。

- [ ] **Step 6: `web/core` から完全削除を消す**

```bash
cd web && pnpm gen
```

`git diff web/core/api/schema.d.ts` が、手順 5 と同じ 3 か所（`delete-task` の summary、`AffectedWrite.ulids`、クエリの `purge`・`confirm`）だけの差であることを確かめる。

`web/core/cli.ts` から次の 1 行を消す。

```ts
export const cliPurge = (id: string): string => cliLine(["purge", id]);
```

`web/core/cli.test.ts`: import の `  cliPurge,` の行と、次の期待値の行を消す。

```ts
    expect(cliPurge("REL-8")).toBe("hh purge REL-8");
```

`web/core/messages.ts`: `// deleting and restoring` の下にある次の 3 つを消す（`restore_at.*`・`restore.forward` は残す）。

```ts
  "purge.changed": () => "確認のあとにタスクが変わりました。もう一度確かめてください",
  "purge.child_not_deleted": (a) =>
    `${str(a, "id")} が削除されていないため、完全には削除できません`,
  "purge.unconfirmed": (a) => {
    const t = a.targets;
    const n = Array.isArray(t) ? t.length : 0;
    return n > 0
      ? `${n} 件のタスクを完全に削除します。対象を確かめてください`
      : "完全に削除するには確認が必要です";
  },
```

`web/core/messages.test.ts`: 次のテストを、直後の空行ごと消す。

```ts
  it("counts the targets of purge.unconfirmed", () => {
    const targets = [{ ulid: "a" }, { ulid: "b" }];
    expect(errorMessage(err("purge.unconfirmed", { targets }))).toContain("2 件");
  });
```

`web/core/api/client.ts` のコメント（`body` は `schema.violation` の違反の一覧を読むのに使われている）:

```ts
// ApiError is a problem+json response of the server. type is the Code of the
// error (task.not_found...); args are its arguments; body is the whole
// problem, for what args do not hold (schema.violation lists what it broke).
```

`web/core/cache.ts` の `applyWrite` から完全削除の答えの取り込みを消す。

```ts
  // applyWrite takes in the answer of a write: the tasks as they are now.
  applyWrite(r: { task?: TaskView; tasks?: TaskView[]; commit?: string | null }): void {
    for (const t of [...(r.task ? [r.task] : []), ...(r.tasks ?? [])]) {
      this.markWrite(t.ulid);
      const row = this.ingest(t);
      const show = this.shows.get(t.ulid);
      if (show) this.putShow({ ...show, ...row });
    }
    this.noteOwnCommit(r.commit);
    this.flush();
  }
```

`web/core/cache.test.ts`:
- `it("forgets the tasks purged", …)`（`cache.applyWrite({ ulids: ["A"], commit: "c9" })` を使うもの）を消す。
- `applyWrite({ ulids: … })` を使う 2 つのテストを、CLI の purge が `removed` で伝える経路（`tasks.changed` のイベント）で書き直す。2 つの `it("does not bring back a task purged after the list was sent", …)` と `it("does not bring back a task purged after the tree was sent", …)` を次に置き換える。最初の取得は purge の前に出ていて、あとの取得はサーバーが A を返さない、という並びを呼び出しの回数で作る。

```ts
  it("does not bring back a task purged after the list was sent", async () => {
    let open!: () => void;
    const gate = new Promise<void>((r) => {
      open = r;
    });
    let calls = 0;
    const { cache, conn } = setup(async () => {
      const first = calls++ === 0;
      await gate;
      return { tasks: first ? [tv("A"), tv("B")] : [tv("B")] };
    });
    const w = cache.watchList({ q: "", tree: false });
    await vi.waitFor(() => expect(calls).toBe(1));
    conn.emit("tasks.changed", changed("shared", [], "c1", "purge", ["A"]));
    open();
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    await vi.waitFor(() => expect(w.ulids()).toEqual(["B"]));
    expect(cache.task("A")).toBeUndefined();
    expect(cache.task("B")).toBeDefined();
    w.dispose();
  });

  it("does not bring back a task purged after the tree was sent", async () => {
    let open!: () => void;
    const gate = new Promise<void>((r) => {
      open = r;
    });
    let calls = 0;
    const { cache, conn } = setup(async () => {
      const first = calls++ === 0;
      await gate;
      return { roots: [tv("R", "r", { children: first ? [tv("A"), tv("B")] : [tv("B")] })] };
    });
    const w = cache.watchTree({ q: "", tree: true });
    await vi.waitFor(() => expect(calls).toBe(1));
    conn.emit("tasks.changed", changed("shared", [], "c1", "purge", ["A"]));
    open();
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    expect(w.roots()).toEqual([{ ulid: "R", children: [{ ulid: "B", children: [] }] }]);
    w.dispose();
  });
```

GUI からは完全削除できなくなったので、`web/core/messages.ts` の、ワークスペースが空でないときの 2 つの案内（`workspace.not_empty`・`undo.blocked`）の「完全に削除してください」を、CLI の案内（spec §9）に合わせて直す。

```ts
  "workspace.not_empty": () =>
    "ワークスペースにまだタスクがあります（削除済みを含む）。別のワークスペースへ移すか、削除したうえで hh purge --older-than 0d で消してください",
```

```ts
  "undo.blocked": () =>
    "ワークスペースにまだタスクがあるため取り消せません。別のワークスペースへ移すか、削除したうえで hh purge --older-than 0d で消してください",
```

- [ ] **Step 7: phase4 の spec を直す**

`docs/superpowers/specs/2026-11-01-phase4-gui-design.md`（行番号は 2026-11-02 の main 時点。各行は先頭の文字列で確かめてから直す）:

1. 18 行（`| 4e 残りの画面 |` の行）を次にする。

```
| 4e 残りの画面 | 「画面と範囲」の残りすべて。設定 → 同期と保守（SettingsGeneral）に、期間を指定した purge（`hh purge --older-than`）の画面と API を足す（2026-11-02 に利用者が決定。GUI のタスク詳細からの完全削除は持たない） | 「画面と範囲」がすべて GUI で動く |
```

2. 68 行（TaskDetail の行）の `削除と完全削除 |` を `削除 |` にする。
3. 74 行（Dialogs の行）の `削除・完全削除の確認、` を `削除の確認、` にする。
4. 79 行（SettingsGeneral の行）の `同期の状態と間隔、doctor と修復` を `同期の状態と間隔、doctor と修復、期間を指定した purge（削除済みタスクと転送ファイルの完全削除。`hh purge --older-than` と同じ。4e）` にする。
5. 210 行（`| 完全削除 | \`DELETE /tasks/{id}?purge=true&confirm=…` の行）を消す。
6. 50 行（`- 4d の細部（2026-11-02）` の箇条）の直後に 1 行足す。

```bash
sed -n 50p docs/superpowers/specs/2026-11-01-phase4-gui-design.md | grep -c '^- 4d の細部'   # 1
sed -i '50a\
- GUI と API の完全削除を外す（2026-11-02 に利用者が決定）: `DELETE /tasks/{id}` の `purge`・`confirm` と `purge.unconfirmed` の応答、タスク詳細の「完全削除」を持たない。完全削除は CLI の `hh purge [--older-than <期間>]` が行い、GUI では「設定 → 同期と保守」に期間を指定した purge を置く（画面と API は 4e で作る）' docs/superpowers/specs/2026-11-01-phase4-gui-design.md
```

行を消す・足すと後ろの行番号がずれるので、直す順は 6（50 行のあとに足す）→ 5（210 行を消す）→ 4・3・2・1（79・74・68・18 行）とする。1〜4 は同じ行の中の書き換えで、行番号は変わらない。

確かめ:

```bash
grep -n '完全削除\|purge=true' docs/superpowers/specs/2026-11-01-phase4-gui-design.md
```

残りは、手順 4・6 で足した文と、4e の範囲の文だけ。

- [ ] **Step 8: web の検査とコミット**

Run（`web/` で）: `pnpm test && pnpm check && pnpm lint`（timeout 600000）
Expected: PASS（警告 0、指摘 0）。

Run: `go build ./... && go vet ./internal/server && go test ./internal/server -count=1`（timeout 600000）
Expected: PASS

```bash
git add internal/server api web/core docs/superpowers/specs/2026-11-01-phase4-gui-design.md
git commit -m "$(cat <<'EOF'
refactor(api): GUI と API の完全削除を外す

DELETE /api/tasks/{id} の purge・confirm と purge.unconfirmed の応答、web/core の
cliPurge と purge.* のメッセージ、cache.applyWrite の ulids を消す。完全削除は
hh purge --older-than に一本化し、GUI は 4e で「設定 → 同期と保守」に置く。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

#### spec にない細部

1. `DELETE /api/tasks/{id}?purge=true` は、未知のクエリとして無視され、通常の削除（`hh rm`）になる。リリース前で、GUI は送らないため、互換のための拒否は足さない。
2. `web/core/messages.ts` の `workspace.not_empty`・`undo.blocked` の「完全に削除してください」を、`hh purge --older-than 0d` を案内する文にする。GUI から完全削除できなくなるため、そのままでは行き場のない案内になる。CLI の文言（spec §9）は Task 11 で直す。
3. `cache.applyWrite` の `ulids`（完全削除の答えの取り込み）を消し、それを使っていた 3 つのテストのうち 2 つを、CLI の purge が `removed` で伝えるイベントの経路で書き直す。`gone()`（取得の途中で消えたタスクを戻さない）の検査を残すため。
4. phase4 の spec の SettingsGeneral の行と 4e の範囲に、期間を指定した purge を足す。spec §11 の「4e の範囲に足す」の具体的な置き場所を決めた。

---

### Task 10: `--rewrite-history` と書き換えの記録を消し、force push を doctor --fix で直す

spec §10。履歴を書き換える機能（`purge --rewrite-history`）と、書き換えの記録に頼る追従（`rewrites.yaml`・`rewrites-applied.json`・同期の中の自動の作り直し・ミラーの扱い・`rewrite_leftover`）を消す。かわりに、同期は origin の履歴が手元と食い違っていれば止まり（今の `ErrForcePush`）、`hh doctor --fix` が記録なしで clone し直して未 push の変更を戻す。

**考え方（作り直しの流れ）:**

1. `hh sync` は fetch のあと、origin の先頭が手元で追っていた履歴の子孫でなければ、追跡の参照を戻し、`force-push.json` を書いて `ErrForcePush` で止まる（今と同じ）。同期の中では何も作り直さない。
2. `hh doctor` は `force-push.json` があれば `unexpected_force_push` を出す（今と同じ）。
3. `hh doctor --fix` は、まず止まった入れ替えの跡を仕上げ（`finishRebuild`、今と同じ）、次に `force-push.json` のある Store を作り直し（新しい `sync.MakeRebuild` → `sync.Swap`）、そのあとで 1 コミットの修復をする。
4. 作り直しは `<path>.rewrite-new` に origin を clone し、`tracked`（手元が追っていた origin）より上の未 push のコミットを、1 つの `replay` コミットにして載せる。新しい履歴に無いタスクの未 push の変更は戻せないので、そのタスクは載せず（`Rebuild.Purged`）、古い Store を `<path>.kept-<UTC 時刻>` に残して、そのパスを知らせる。
5. 入れ替えの直前に古い Store の HEAD が変わっていた（`ErrRebuildStale`）ときは、入れ替えずにやめて `hedgehug doctor --fix` のやり直しを案内する。
6. 作り直しは push しない。載せた `replay` コミットは次の同期で push される。

**消す参照の洗い出し（`grep -rn -E 'rewrite-history|RewriteHistory|PrepareRewrite|RewritesPath|DecodeRewrites|EncodeRewrites|model\.Rewrite|ReadApplied|WriteApplied|rewrites-applied|rewrites\.yaml|rewrite-in-progress|rewrite_leftover|staleMirror|mirrorStale|mirrorDiverged|BackgroundThen|ResetMain|HasSameCommit|\.ForcePush\(' --include='*.go' .` の結果。`url_rewrites` の `Rewrite` と、`migrate` の「rewrite the tasks」は別物なので除く）:**

| 場所 | 扱い |
| --- | --- |
| `cmd/hedgehug/purge_rewrite.go`・`purge_rewrite_test.go`・`testdata/TestPurgeRewriteHistory*/` | 消す |
| `cmd/hedgehug/rm.go`（`purgeCmd` の `--rewrite-history`） | フラグと分岐を消す |
| `cmd/hedgehug/mv.go` 165 行 | 確認の文言から `; use hedgehug purge --rewrite-history to remove it` を消す。5 つの golden も |
| `cmd/hedgehug/e2e_3c_test.go` | 手順 5（`purge --rewrite-history`）と、それだけが使うヘルパー（`inProcess`・`pathsWith`・`inHistory`）を消す |
| `cmd/hedgehug/doctor.go`・`doctor_rewrite_test.go` | 作り直しの行を足す。止まった入れ替えのテストを `rewrite_leftover` なしに直す |
| `internal/ops/rewrite.go`・`rewrite_test.go`・`rewrite_history_only_test.go`・`rewrite_once_test.go` | 消す |
| `internal/ops/rewrite_take.go`・`rewrite_take_test.go`・`rewrite_take_bg_test.go`・`rewrite_take_log_test.go` | `rewrite_take.go` は `rebuild.go` に改名して直す。テストは `rebuild_test.go` に作り直し、残りは消す |
| `internal/ops/undo.go` 88 行・`moved_test.go` 141 行 | `rewrite-history` の理由を消す |
| `internal/ops/session.go`・`sync.go` | 同期の中の入れ替え（`runSyncLocked` の分け方・`swapRebuilt`・`SyncBackground` の入れ替え・`reopenLog`・`res.Reopen`）を消す |
| `internal/ops/maintain.go`・`maintain_test.go` | `rewrite_leftover` を消し、作り直しの段を足す。`unexpected_force_push` の文言 |
| `internal/doctor/finding.go` | `checkOrder` から `rewrite_leftover` |
| `internal/gitrepo/rewrite.go`・`rewrite_test.go` | `RewriteHistory`・`ResetMain`・`ForcePush` とその道具を消す。`SetTracking`・`Close`・`FindSameCommit` は残す（置き場を変える） |
| `internal/gitrepo/remote.go`（`push` の `force`・`lease`） | 使い道がなくなるので消す |
| `internal/gitrepo/hassame_test.go` | `HasSameCommit` を `FindSameCommit` に |
| `internal/model/rewrites.go`・`rewrites_test.go` | 消す |
| `internal/sync/rewrite.go`・`rewrite_test.go`・`rewrite_fork_test.go` | `rewrite.go` は `forcepush.go` に改名して縮める。テストは `forcepush_test.go` に作り直す |
| `internal/sync/rebuild.go`・`rebuild_*_test.go` | `rebuild` を `MakeRebuild` にして記録なしにする。`Swap` に古い Store を残す場合を足す |
| `internal/sync/sync.go` | `Result` の `Rewrite`・`Rebuild`・`Swapped`・`Reopen`、記録の判定、ミラーの `staleMirror` の分岐 |
| `internal/sync/background.go`・`childlog_unix.go`・`background_stderr_test.go`・`childlog_close_test.go` | 入れ替えのための `BackgroundThen`・`BoundedLog` の `Release`/`Reopen`/`Close`・stderr の付け替えを消す |
| `web/core/messages.ts`・`messages.test.ts` | `sync.force_push` の文言 |
| `internal/ops/session.go` 46 行・`internal/server/server.go` 152 行・`poll.go` 86 行のコメント | 「同期が入れ替える」を「doctor --fix が入れ替える」に |

`cmd/hedgehug/e2e_3c_test.go` の手順 4（`purge --moved`）、`internal/ops` の `Purge*`、`undo.go` の `purge`・`purge-moved` の行は Task 11 の範囲なので触らない。

**Files:**
- Create: `internal/gitrepo/gitrepotest/rewrite.go`（テストの道具: origin の履歴を書き換える）
- Create: `internal/gitrepo/samecommit.go`
- Create: `internal/gitrepo/close_test.go`
- Create: `internal/sync/forcepush_test.go`
- Create: `internal/ops/rebuild_test.go`
- Create: `cmd/hedgehug/doctor_forcepush_test.go`
- Rename + Modify: `internal/sync/rewrite.go` → `internal/sync/forcepush.go`、`internal/ops/rewrite_take.go` → `internal/ops/rebuild.go`
- Modify: `internal/sync/rebuild.go`・`sync.go`・`background.go`、`internal/gitrepo/remote.go`・`repo.go`、`internal/ops/session.go`・`sync.go`・`maintain.go`・`undo.go`、`internal/doctor/finding.go`、`internal/server/server.go`・`poll.go`（コメント）、`cmd/hedgehug/rm.go`・`mv.go`・`doctor.go`、`web/core/messages.ts`
- Modify（テスト）: `internal/sync/rebuild_test.go`・`rebuild_local_test.go`・`rebuild_gap_test.go`、`internal/gitrepo/hassame_test.go`、`internal/ops/maintain_test.go`・`moved_test.go`、`cmd/hedgehug/doctor_rewrite_test.go`・`e2e_3c_test.go`、`web/core/messages.test.ts`、`cmd/hedgehug/testdata/TestMvStore*/*.err.golden`（5 つ）
- Delete: `cmd/hedgehug/purge_rewrite.go`・`purge_rewrite_test.go`・`testdata/TestPurgeRewriteHistory*/`、`internal/ops/rewrite.go`・`rewrite_test.go`・`rewrite_history_only_test.go`・`rewrite_once_test.go`・`rewrite_take_test.go`・`rewrite_take_bg_test.go`・`rewrite_take_log_test.go`、`internal/gitrepo/rewrite.go`・`rewrite_test.go`、`internal/model/rewrites.go`・`rewrites_test.go`、`internal/sync/rewrite_test.go`・`rewrite_fork_test.go`・`childlog_unix.go`・`background_stderr_test.go`・`childlog_close_test.go`

**Interfaces:**
- Consumes: Task 1〜8 の成果は使わない。既存の `store.LockInOrder`・`sync.Swap`・`sync.FindLeftover`・`ops.Session.SyncOptions`・`ss.BeforeSwap`・`ss.closeMounted`・`ss.dropMount`・テストの `newCluster`・`newTestSession`・`addBareRemote`・`cloneSession`・`twoUsers`。
- Produces（Task 11 以降が使う、または使わなくなるもの）:
  - `sync.MakeRebuild(s *store.Store, opt sync.Options) (*sync.Rebuild, error)`（未 push の変更を戻した clone を `<path>.rewrite-new` に作る。origin が手元の子孫に戻っていれば `nil, nil` で `force-push.json` を消す）、`sync.Rebuild.Kept string`（`Swap` が古い Store を残した場所。残さなければ空）
  - `ops.Rebuilt{Replayed int; Dropped []string; Kept string}` と `ops.DoctorReport.Rebuilt *Rebuilt`（JSON は `rebuilt`、なければ省く）
  - `gitrepotest.RewriteRemote(t, bare string, drop ...string) (old, rewritten string)`・`gitrepotest.SetRemoteMain(t, bare, hash string)`（origin の履歴を `git filter-repo` のように書き換えるテストの道具）
  - doctor の出力の行: `Cloned <store> again from origin; put back N unpushed commit(s)`、戻せなかった変更があれば `Could not put back the changes to <id>, …; the old store is kept at <path>`
  - 消えるもの: `ops.RewriteInput`・`RewritePlan`・`RewriteResult`・`Session.PrepareRewrite`・`RewriteHistory`、`model.Rewrite` 系、`sync.Detect`・`ReadApplied`・`WriteApplied`、`sync.Result.Rewrite`・`Rebuild`・`Swapped`・`Reopen`、`sync.BackgroundThen`、`gitrepo.Repo.RewriteHistory`・`ResetMain`・`ForcePush`・`HasSameCommit`

- [ ] **Step 1: テストの道具を足す**

`internal/gitrepo/gitrepotest/rewrite.go`（新規）。bare の origin の `main` を、`git filter-repo` のあとのような「親のない 1 つのコミット」に置き換える。作者・コミッターと時刻・メッセージは古い先頭のものを写す（`FindSameCommit` が同じコミットを見つけられる）。木は古い先頭の木から `drop` のパスのファイルとディレクトリを除いたもの。

```go
package gitrepotest

import (
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const mainRef = plumbing.ReferenceName("refs/heads/main")

// RewriteRemote makes main of the bare repository at bare a history of its
// own, as a git filter-repo leaves one: a single commit with no parent, with
// the author, committer and message of the old head and its tree without
// the files and directories drop names (paths from the root, slash
// separated; one that is not there is skipped). It returns the old head and
// the new one, as hex; SetRemoteMain puts the old one back.
func RewriteRemote(t testing.TB, bare string, drop ...string) (old, rewritten string) {
	t.Helper()
	r, err := git.PlainOpen(bare)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Reference(mainRef, true)
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.CommitObject(ref.Hash())
	if err != nil {
		t.Fatal(err)
	}
	tree := c.TreeHash
	for _, p := range drop {
		tree, _ = treeWithout(t, r, tree, p)
	}
	nc := &object.Commit{Author: c.Author, Committer: c.Committer, Message: c.Message, TreeHash: tree}
	obj := r.Storer.NewEncodedObject()
	if err := nc.Encode(obj); err != nil {
		t.Fatal(err)
	}
	h, err := r.Storer.SetEncodedObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	SetRemoteMain(t, bare, h.String())
	return ref.Hash().String(), h.String()
}

// SetRemoteMain points main of the bare repository at bare to the commit
// hash (hex), which need not descend from it.
func SetRemoteMain(t testing.TB, bare, hash string) {
	t.Helper()
	r, err := git.PlainOpen(bare)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Storer.SetReference(plumbing.NewHashReference(mainRef, plumbing.NewHash(hash))); err != nil {
		t.Fatal(err)
	}
}

// treeWithout stores the tree h without the entry at path, and returns the
// new tree and whether nothing is left in it.
func treeWithout(t testing.TB, r *git.Repository, h plumbing.Hash, path string) (plumbing.Hash, bool) {
	t.Helper()
	tree, err := r.TreeObject(h)
	if err != nil {
		t.Fatal(err)
	}
	name, rest, nested := strings.Cut(path, "/")
	var entries []object.TreeEntry
	for _, e := range tree.Entries {
		switch {
		case e.Name != name:
			entries = append(entries, e)
		case nested:
			if sub, empty := treeWithout(t, r, e.Hash, rest); !empty {
				e.Hash = sub
				entries = append(entries, e)
			}
		}
	}
	obj := r.Storer.NewEncodedObject()
	if err := (&object.Tree{Entries: entries}).Encode(obj); err != nil {
		t.Fatal(err)
	}
	nh, err := r.Storer.SetEncodedObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	return nh, len(entries) == 0
}
```

- [ ] **Step 2: sync のテストを書く（失敗する）**

`internal/sync/rewrite_test.go` と `internal/sync/rewrite_fork_test.go` を消し、`internal/sync/forcepush_test.go` を作る。`rewriteOrigin` が origin を書き換える（`drop` のタスクのファイルとコメント・作業時間のディレクトリを除く）。

```go
package sync

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// rewriteOrigin makes origin a history of its own, as a git filter-repo
// leaves one, without the files of the tasks in drop (the task, its comments
// and its worklog). The clones that followed the old history find that
// origin is not a descendant of it. It returns the old head and the new one.
func (c *cluster) rewriteOrigin(drop ...model.ULID) (old, rewritten plumbing.Hash) {
	c.t.Helper()
	var paths []string
	for _, u := range drop {
		paths = append(paths, model.TaskPath(u), path.Dir(model.CommentPath(u, model.ULID{})), path.Dir(model.WorklogPath(u, model.ULID{})))
	}
	o, n := gitrepotest.RewriteRemote(c.t, c.remote, paths...)
	return plumbing.NewHash(o), plumbing.NewHash(n)
}

// cloneFresh adds a clone made from origin as it is now, and returns its
// index.
func (c *cluster) cloneFresh() int {
	c.t.Helper()
	i := len(c.stores)
	dir := filepath.Join(c.t.TempDir(), fmt.Sprint("s", i))
	if _, err := gitrepo.Clone(c.remote, dir); err != nil {
		c.t.Fatal(err)
	}
	s, err := store.Open(dir)
	if err != nil {
		c.t.Fatal(err)
	}
	c.add(s, i)
	return i
}

func headOf(t *testing.T, c *cluster, i int) plumbing.Hash {
	t.Helper()
	h, err := c.stores[i].Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func trackOf(t *testing.T, c *cluster) plumbing.Hash {
	t.Helper()
	h, err := c.stores[1].Repo().RemoteHead("origin")
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// A force push stops the sync, leaves the clone as it was and writes
// force-push.json; the file goes when origin follows the clone again.
func TestRunStopsOnAForcePush(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	c.sync(0)
	c.sync(1)
	headB, trackB := headOf(t, c, 1), trackOf(t, c)

	old, newHead := c.rewriteOrigin(gone.ULID)
	_, err := Run(c.stores[1], c.opts[1])
	if !errors.Is(err, ErrForcePush) {
		t.Fatalf("Run = %v, want ErrForcePush", err)
	}
	if got := headOf(t, c, 1); got != headB {
		t.Errorf("HEAD moved to %s", got)
	}
	if got := trackOf(t, c); got != trackB {
		t.Errorf("tracking = %s, want %s", got, trackB)
	}
	f, ok, err := ReadForcePush(c.stores[1])
	if err != nil || !ok || f.Origin != newHead.String() || f.Tracked != trackB.String() || f.Checked.IsZero() {
		t.Errorf("force-push.json = %+v, %v, %v", f, ok, err)
	}
	if c.task(1, gone.ULID) == nil {
		t.Error("the clone changed")
	}

	// origin goes back: the next sync is quiet and the file goes.
	gitrepotest.SetRemoteMain(t, c.remote, old.String())
	c.sync(1)
	if _, ok, err := ReadForcePush(c.stores[1]); err != nil || ok {
		t.Errorf("force-push.json after origin went back = %v, %v", ok, err)
	}
}

// A history of origin with no task dropped, only other commits, is a force
// push too: there is no record to tell it from one that removed a task.
func TestRunStopsOnARewriteThatDropsNothing(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	c.addTask(0, "keep")
	c.sync(0)
	c.sync(1)
	c.rewriteOrigin()
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("Run = %v, want ErrForcePush", err)
	}
}
```

`internal/sync/rebuild_test.go` の先頭（`swap` から `TestRebuildMergesConflicts` の終わりまで）と `TestSwapStale`・`TestRebuildRefusesUncommittedChanges`・`TestFinishLeftoverNoStore`・`TestRebuildInboxSeesWhatCameAfterTheRewrite` を、次のとおりに直す。`exists`・`TestFinishLeftoverOtherStates`・`TestFinishLeftoverCarriesLocalFiles`・`TestFindLeftoverIgnoresOtherNames` は変えない。

import は `errors`・`os`・`path/filepath`・`slices`・`strings`・`testing`・`time`・`plumbing`・`inbox`・`model`・`store` のまま（`fmt` と `gitrepo` は `forcepush_test.go` が使う）。

```go
// swap puts the rebuild b in the place of clone 1, as ops does after it
// released the locks, and opens the store again.
func (c *cluster) swap(b *Rebuild) {
	c.t.Helper()
	dir := c.stores[1].Dir()
	if err := c.stores[1].Repo().Close(); err != nil {
		c.t.Fatal(err)
	}
	if err := Swap(dir, b); err != nil {
		c.t.Fatalf("Swap: %v", err)
	}
	s, err := store.Open(dir)
	if err != nil {
		c.t.Fatal(err)
	}
	c.stores[1] = s
}

// rebuild makes clone i again from origin, as doctor --fix does.
func (c *cluster) rebuild(i int) *Rebuild {
	c.t.Helper()
	b, err := MakeRebuild(c.stores[i], c.opts[i])
	if err != nil {
		c.t.Fatalf("MakeRebuild: %v", err)
	}
	if b == nil {
		c.t.Fatal("origin follows the clone: there is nothing to rebuild")
	}
	return b
}

func (c *cluster) commentsOf(i int, task model.ULID) []string {
	c.t.Helper()
	var out []string
	for cm, err := range c.stores[i].Comments(task) {
		if err != nil {
			c.t.Fatal(err)
		}
		out = append(out, cm.Body)
	}
	return out
}

// rewriteScene is origin rewritten without gone while clone 1, which did
// the work before it, has not synced; the sync of clone 1 stopped, and rb is
// its rebuild.
type rewriteScene struct {
	keep, gone, other *model.Task
	rb                *Rebuild
}

func newRewriteScene(t *testing.T, c *cluster, work func(sc *rewriteScene)) *rewriteScene {
	t.Helper()
	sc := &rewriteScene{}
	sc.keep = c.addTask(0, "keep")
	sc.gone = c.addTask(0, "gone")
	sc.other = c.addTask(0, "other")
	c.sync(0)
	c.sync(1)
	work(sc)
	c.rewriteOrigin(sc.gone.ULID)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	sc.rb = c.rebuild(1)
	return sc
}

// The unpushed work of the clone is put back on the new history; what it
// did to the task the new history lacks is not, and is told of; the old
// store is kept.
func TestRebuildReplaysUnpushedChanges(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(sc *rewriteScene) {
		c.set(1, sc.other.ULID, func(x *model.Task) { x.Title = "other by b" })
		c.set(1, sc.gone.ULID, func(x *model.Task) { x.Title = "gone by b" })
		c.comment(1, sc.keep.ULID, "from b", model.NewULID())
		c.comment(1, sc.gone.ULID, "to the task that is gone", model.NewULID())
	})
	if sc.rb.Replayed != 4 {
		t.Errorf("Replayed = %d, want 4", sc.rb.Replayed)
	}
	if !slices.Equal(sc.rb.Purged, []string{c.task(1, sc.gone.ULID).ID}) || sc.rb.PurgedTasks[0] != sc.gone.ULID {
		t.Errorf("Purged = %v %v", sc.rb.Purged, sc.rb.PurgedTasks)
	}
	if trackOf(t, c) == mustHead(t, c.stores[1]) {
		t.Fatal("setup: the clone had no unpushed work")
	}
	c.swap(sc.rb)
	if left, err := FindLeftover(c.stores[1].Dir()); err != nil || !left.None() {
		t.Errorf("leftover = %+v, %v", left, err)
	}
	if sc.rb.Kept == "" || !exists(t, sc.rb.Kept) {
		t.Errorf("Kept = %q, want the old store beside the new one", sc.rb.Kept)
	}
	if got := c.task(1, sc.other.ULID); got == nil || got.Title != "other by b" {
		t.Errorf("other = %+v", got)
	}
	if c.task(1, sc.gone.ULID) != nil {
		t.Error("the task that is gone came back")
	}
	if got := c.commentsOf(1, sc.keep.ULID); !slices.Equal(got, []string{"from b"}) {
		t.Errorf("comments = %v", got)
	}
	if got := c.commentsOf(1, sc.gone.ULID); len(got) != 0 {
		t.Errorf("comments of the task that is gone = %v", got)
	}
	// One commit on the new head of origin.
	r := c.stores[1].Repo()
	origin, err := r.RemoteHead("origin")
	if err != nil {
		t.Fatal(err)
	}
	head := mustHead(t, c.stores[1])
	log, err := r.Between(head, origin)
	if err != nil || len(log) != 1 || !slices.Equal(log[0].Parents, []plumbing.Hash{origin}) {
		t.Fatalf("commits above origin = %+v, %v", log, err)
	}
	if op, _ := log[0].Trailer("Hedgehug-Op"); op != "replay" {
		t.Errorf("Hedgehug-Op = %q", op)
	}
	kinds := map[inbox.Kind]inbox.Item{}
	items, err := inbox.Open(c.stores[1]).List(true)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		kinds[it.Kind] = it
	}
	if it, ok := kinds[inbox.RewriteApplied]; !ok || !strings.Contains(string(it.Detail), `"replayed":4`) {
		t.Errorf("rewrite_applied = %+v", it)
	}
	if it, ok := kinds[inbox.PurgedChange]; !ok || it.Task != sc.gone.ULID {
		t.Errorf("purged_change = %+v", it)
	}
	// The next sync pushes it onto the new history.
	if r := c.sync(1); len(r.Pushed) == 0 {
		t.Errorf("Pushed = %v", r.Pushed)
	}
	if got, want := trackOf(t, c), mustHead(t, c.stores[1]); got != want {
		t.Errorf("origin is at %s, the clone at %s", got, want)
	}
}

// A clone with nothing unpushed is rebuilt as the new origin, and no old
// store is kept.
func TestRebuildWithoutUnpushedChanges(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(*rewriteScene) {})
	if sc.rb.Replayed != 0 || len(sc.rb.Purged) != 0 {
		t.Errorf("rebuild = %+v", sc.rb)
	}
	c.swap(sc.rb)
	if got, want := mustHead(t, c.stores[1]), trackOf(t, c); got != want {
		t.Errorf("HEAD = %s, want the head of origin %s", got, want)
	}
	if sc.rb.Kept != "" {
		t.Errorf("Kept = %q, want none", sc.rb.Kept)
	}
	if m, err := filepath.Glob(c.stores[1].Dir() + ".kept-*"); err != nil || len(m) != 0 {
		t.Errorf("old stores kept = %v, %v", m, err)
	}
}

// Unpushed work on the tasks the new history has is all put back, and the
// old store is not kept.
func TestRebuildRemovesTheOldStoreWhenNothingWasDropped(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(sc *rewriteScene) {
		c.set(1, sc.other.ULID, func(x *model.Task) { x.Title = "other by b" })
	})
	if sc.rb.Replayed != 1 || len(sc.rb.Purged) != 0 {
		t.Fatalf("rebuild = %+v", sc.rb)
	}
	c.swap(sc.rb)
	if sc.rb.Kept != "" {
		t.Errorf("Kept = %q, want none", sc.rb.Kept)
	}
	if m, err := filepath.Glob(c.stores[1].Dir() + ".*"); err != nil || len(m) != 0 {
		t.Errorf("something is left beside the store: %v, %v", m, err)
	}
	if got := c.task(1, sc.other.ULID); got == nil || got.Title != "other by b" {
		t.Errorf("other = %+v", got)
	}
}

// A task that only the unpushed commits have is not one the new history
// lacks: it is put back.
func TestRebuildKeepsATaskOnlyTheCloneHas(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	var mine *model.Task
	sc := newRewriteScene(t, c, func(*rewriteScene) {
		mine = c.addTask(1, "mine")
		c.comment(1, mine.ULID, "on mine", model.NewULID())
	})
	if len(sc.rb.Purged) != 0 {
		t.Errorf("Purged = %v", sc.rb.Purged)
	}
	c.swap(sc.rb)
	if c.task(1, mine.ULID) == nil {
		t.Error("the clone's own task is lost")
	}
	if got := c.commentsOf(1, mine.ULID); !slices.Equal(got, []string{"on mine"}) {
		t.Errorf("comments = %v", got)
	}
}

// A change both the new history and this clone made to a field is decided
// as a merge decides it, and the loser is recorded.
func TestRebuildMergesConflicts(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	other := c.addTask(0, "other")
	c.sync(0)
	c.sync(1)
	c.set(1, other.ULID, func(x *model.Task) { x.Title = "b" })
	c.rewriteOrigin(gone.ULID)
	// the edit made on the new history is the later one
	i := c.cloneFresh()
	c.stores[i].SetNow(func() time.Time { return time.Now().Add(time.Hour) })
	c.set(i, other.ULID, func(x *model.Task) { x.Title = "a" })
	c.sync(i)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	c.swap(c.rebuild(1))
	got := c.task(1, other.ULID)
	if got == nil || got.Title != "a" {
		t.Fatalf("title = %+v, want the later edit, a", got)
	}
	r := c.stores[1].Repo()
	origin, _ := r.RemoteHead("origin")
	log, err := r.Between(mustHead(t, c.stores[1]), origin)
	if err != nil || len(log) != 1 {
		t.Fatalf("log = %+v, %v", log, err)
	}
	found := false
	for _, tr := range log[0].Trailers {
		if tr.Key == "Hedgehug-Overwritten" && strings.Contains(tr.Value, " title ") && strings.Contains(tr.Value, `"b"`) {
			found = true
		}
	}
	if !found {
		t.Errorf("no Hedgehug-Overwritten for the title b: %+v", log[0].Trailers)
	}
}

// origin that follows the clone again needs no rebuild: MakeRebuild removes
// force-push.json and makes nothing.
func TestMakeRebuildWhenOriginFollowsAgain(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	gone := c.addTask(0, "gone")
	c.sync(0)
	c.sync(1)
	old, _ := c.rewriteOrigin(gone.ULID)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	gitrepotest.SetRemoteMain(t, c.remote, old.String())
	b, err := MakeRebuild(c.stores[1], c.opts[1])
	if err != nil || b != nil {
		t.Fatalf("MakeRebuild = %+v, %v; want nothing", b, err)
	}
	if _, ok, err := ReadForcePush(c.stores[1]); err != nil || ok {
		t.Errorf("force-push.json = %v, %v", ok, err)
	}
	if exists(t, c.stores[1].Dir()+newSuffix) {
		t.Error("a rebuild is left beside the store")
	}
}

// A write that lands between the rebuild and the swap stops the swap.
func TestSwapStale(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(*rewriteScene) {})
	late := c.addTask(1, "late")
	dir := c.stores[1].Dir()
	if err := c.stores[1].Repo().Close(); err != nil {
		t.Fatal(err)
	}
	if err := Swap(dir, sc.rb); !errors.Is(err, ErrRebuildStale) {
		t.Fatalf("Swap = %v, want ErrRebuildStale", err)
	}
	if _, err := os.Stat(sc.rb.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the rebuild is still there: %v", err)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	c.stores[1] = s
	// doctor --fix makes it again, and the late task is put back.
	b := c.rebuild(1)
	if b.Replayed != 1 {
		t.Fatalf("second rebuild = %+v", b)
	}
	c.swap(b)
	if c.task(1, late.ULID) == nil {
		t.Error("the late task is lost")
	}
}

// A change git has not committed stops the rebuild.
func TestRebuildRefusesUncommittedChanges(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	keep := c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	c.sync(0)
	c.sync(1)
	c.rewriteOrigin(gone.ULID)
	p := filepath.Join(c.stores[1].Dir(), filepath.FromSlash(model.TaskPath(keep.ULID)))
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(data, "\nedited by hand\n"...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("Run = %v, want ErrForcePush", err)
	}
	if _, err := MakeRebuild(c.stores[1], c.opts[1]); !errors.Is(err, ErrDirty) || !errors.Is(err, ErrDirtyRebuild) {
		t.Fatalf("MakeRebuild = %v, want ErrDirtyRebuild", err)
	}
	if left, err := FindLeftover(c.stores[1].Dir()); err != nil || !left.None() {
		t.Errorf("leftover = %+v, %v", left, err)
	}
}
```

`TestFinishLeftoverNoStore` は `sc.res.Rebuild.Dir` を `sc.rb.Dir` に直す（それ以外は同じ）。

`TestRebuildInboxSeesWhatCameAfterTheRewrite` は次に置き換える（新しい履歴の上に別の clone が足した変更が、作り直したあとの inbox に出る）。

```go
// What was done on the new history after the force push is new to this
// clone's inbox, though the history it comes in is not the one the inbox
// followed.
func TestRebuildInboxSeesWhatCameAfterTheRewrite(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	c.addTask(0, "keep")
	gone := c.addTask(0, "gone")
	other := c.addTask(0, "other")
	c.sync(0)
	c.sync(1)
	c.rewriteOrigin(gone.ULID)
	i := c.cloneFresh()
	c.set(i, other.ULID, func(x *model.Task) { x.Assignees = []string{"github.com/u1"} })
	c.sync(i)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	c.swap(c.rebuild(1))
	c.sync(1) // the inbox catches up on the new history
	items, err := inbox.Open(c.stores[1]).List(true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(items, func(it inbox.Item) bool { return it.Kind == inbox.Assigned && it.Task == other.ULID }) {
		t.Errorf("no assigned item for the task: %+v", items)
	}
}
```

`internal/sync/rebuild_local_test.go`: `c.swap(sc.res)` を `c.swap(sc.rb)` にし、最後の 4 行（`// the files the rebuild writes itself …` のコメントと `applied, err := ReadApplied(s)` から `}` まで）を消す。import は変わらない。

`internal/sync/rebuild_gap_test.go`: `c.swap(sc.res)` を `c.swap(sc.rb)` に、`Swap(dir, sc.res.Rebuild)` を `Swap(dir, sc.rb)` に直す。`TestBoundedLogReleaseAndReopen`（97 行〜末尾のコメントごと）を消し、使われなくなった import の `"os"` と `"path/filepath"` を消す。

`internal/sync/background_stderr_test.go` と `childlog_close_test.go` を消す。

- [ ] **Step 3: gitrepo のテストを直す**

`internal/gitrepo/rewrite_test.go` を消し、`TestClose` だけを `internal/gitrepo/close_test.go` に移す。

```go
package gitrepo

import "testing"

func TestClose(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, _ := Init(dir)
	h := commitHead(t, r, dir, map[string]string{"a.md": "a\n"})
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	r2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := r2.ReadAt(h, "a.md"); err != nil || string(got) != "a\n" {
		t.Errorf("a.md after reopen = %q, %v", got, err)
	}
}
```

`internal/gitrepo/hassame_test.go`: `TestHasSameCommit` を `TestFindSameCommit` にする。`putCommit` はそのまま。

```go
func TestFindSameCommit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	commitFiles(t, r, dir, map[string]string{"a.md": "a\n"})
	at := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	sig := func(name string, when time.Time) object.Signature {
		return object.Signature{Name: name, Email: name + "@x", When: when}
	}
	root := putCommit(t, r, plumbing.ZeroHash, sig("root", at), sig("root", at))
	tracked := putCommit(t, r, root, sig("u", at), sig("u", at.Add(time.Second)))
	// Only the committer time differs.
	late := putCommit(t, r, root, sig("u", at), sig("u", at.Add(2*time.Second)))
	// The same identities on another commit, as a rewrite makes them.
	twin := putCommit(t, r, late, sig("u", at), sig("u", at.Add(time.Second)))
	onTop := putCommit(t, r, twin, sig("v", at), sig("v", at))
	// Only the author differs.
	other := putCommit(t, r, root, sig("w", at), sig("u", at.Add(time.Second)))

	for _, tc := range []struct {
		name string
		from plumbing.Hash
		want bool
	}{
		{"the commit itself", tracked, true},
		{"a twin below the head", onTop, true},
		{"committer time differs", late, false},
		{"author differs", other, false},
	} {
		_, got, err := r.FindSameCommit(tc.from, tracked)
		if err != nil || got != tc.want {
			t.Errorf("%s: FindSameCommit found = %v, %v; want %v", tc.name, got, err, tc.want)
		}
	}
	if got, found, err := r.FindSameCommit(onTop, tracked); err != nil || !found || got != twin {
		t.Errorf("FindSameCommit(onTop) = %s, %v, %v; want the twin %s", got, found, err, twin)
	}
}
```

- [ ] **Step 4: ops のテストを書く（失敗する）**

`internal/ops/rewrite_test.go`・`rewrite_history_only_test.go`・`rewrite_once_test.go`・`rewrite_take_test.go`・`rewrite_take_bg_test.go`・`rewrite_take_log_test.go` を消し、`internal/ops/rebuild_test.go` を作る。

```go
package ops

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/doctor"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// forcePushFx is a store "me" with a remote (ts), cloned by a second user
// (clone) who has unpushed work: a task of their own, and with editGone an
// edit of the task gone. Origin is then rewritten without gone, as a git
// filter-repo leaves it.
type forcePushFx struct {
	ts, clone        *testSession
	remote           string
	keep, gone, mine *model.Task
}

func newForcePushFx(t *testing.T, editGone bool) *forcePushFx {
	t.Helper()
	ts := newTestSession(t)
	f := &forcePushFx{ts: ts, remote: addBareRemote(t, ts)}
	f.keep = ts.addTask("keep", nil)
	f.gone = ts.addTask("gone", nil)
	if _, err := ts.Sync(false, true); err != nil {
		t.Fatal(err)
	}
	f.clone = cloneSession(t, ts, f.remote, ts.Now().Add(time.Minute))
	f.mine = f.clone.addTask("mine", nil)
	if editGone {
		s, _, err := f.clone.Open()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.clone.Update(s, store.Op{Name: "update", Task: f.gone.ULID}, func(tx *store.Tx) error {
			x, err := tx.Task(f.gone.ULID)
			if err != nil {
				return err
			}
			x.Title = "gone, edited"
			return tx.PutTask(x)
		}); err != nil {
			t.Fatal(err)
		}
	}
	gitrepotest.RewriteRemote(t, f.remote, model.TaskPath(f.gone.ULID), path.Dir(model.CommentPath(f.gone.ULID, model.ULID{})))
	return f
}

func (f *forcePushFx) path(t *testing.T) string {
	t.Helper()
	_, e, err := f.clone.Entry()
	if err != nil {
		t.Fatal(err)
	}
	return e.Path
}

// stoppedSwap leaves the clone's store as a swap that stopped after the old
// store was moved away: no store at the path, the old one and a copy of it
// as the rebuild beside it.
func (f *forcePushFx) stoppedSwap(t *testing.T) (dir string) {
	t.Helper()
	dir = f.path(t)
	if err := os.CopyFS(dir+".rewrite-new", os.DirFS(dir)); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(dir, dir+".before-rewrite-20260101T000000.000000000"); err != nil {
		t.Fatal(err)
	}
	return dir
}

func originHead(t *testing.T, bare string) plumbing.Hash {
	t.Helper()
	r, err := git.PlainOpen(bare)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Reference("refs/heads/main", true)
	if err != nil {
		t.Fatal(err)
	}
	return ref.Hash()
}

func mustHead(t *testing.T, s *store.Store) plumbing.Hash {
	t.Helper()
	head, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	return head
}

// A sync that finds origin force pushed stops and changes nothing; doctor
// tells of it.
func TestSyncStopsOnAForcePush(t *testing.T) {
	t.Parallel()
	f := newForcePushFx(t, true)
	s, err := f.clone.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	before := mustHead(t, s)
	if _, err := f.clone.Sync(false, true); !errors.Is(err, sync.ErrForcePush) {
		t.Fatalf("Sync = %v, want ErrForcePush", err)
	}
	if s, err = f.clone.OpenStore(); err != nil || mustHead(t, s) != before {
		t.Errorf("the store changed: %v", err)
	}
	rs, err := f.clone.Doctor(false)
	if err != nil || len(rs) != 1 || len(rs[0].Findings) != 1 {
		t.Fatalf("Doctor = %+v, %v; want one finding", rs, err)
	}
	x := rs[0].Findings[0]
	if x.Check != "unexpected_force_push" || x.Where != "me" || !strings.Contains(x.Msg, "differs from this store's") || !strings.Contains(x.Hint, "doctor --fix") {
		t.Errorf("finding = %+v", x)
	}
	if rs[0].Rebuilt != nil {
		t.Errorf("doctor without --fix rebuilt the store: %+v", rs[0].Rebuilt)
	}
}

// doctor --fix clones the store again: BeforeSwap is told first, the unpushed
// work is put back except what was done to the task that is gone, the old
// store is kept where the report says, and the next sync pushes the work.
func TestDoctorFixClonesTheStoreAgain(t *testing.T) {
	t.Parallel()
	f := newForcePushFx(t, true)
	dir := f.path(t)
	var told []string
	f.clone.BeforeSwap = func(p string) {
		told = append(told, p)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("the store is gone before BeforeSwap: %v", err)
		}
	}
	if _, err := f.clone.Sync(false, true); !errors.Is(err, sync.ErrForcePush) {
		t.Fatalf("Sync = %v, want ErrForcePush", err)
	}
	rs, err := f.clone.Doctor(true)
	if err != nil || len(rs) != 1 || rs[0].Rebuilt == nil {
		t.Fatalf("Doctor = %+v, %v; want a rebuild", rs, err)
	}
	b := rs[0].Rebuilt
	if b.Replayed != 2 || !slices.Equal(b.Dropped, []string{f.gone.ID}) || !strings.HasPrefix(b.Kept, dir+".kept-") {
		t.Errorf("rebuilt = %+v", b)
	}
	if _, err := os.Stat(b.Kept); err != nil {
		t.Errorf("the old store is not at Kept: %v", err)
	}
	if !slices.Equal(told, []string{dir}) {
		t.Errorf("BeforeSwap was told %v, want %s", told, dir)
	}
	if len(rs[0].Findings) != 0 {
		t.Errorf("findings after --fix: %+v", rs[0].Findings)
	}
	if left, err := sync.FindLeftover(dir); err != nil || !left.None() {
		t.Errorf("leftover = %+v, %v", left, err)
	}
	s, err := f.clone.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Task(f.mine.ULID); err != nil {
		t.Errorf("the unpushed task: %v", err)
	}
	if _, err := s.Task(f.keep.ULID); err != nil {
		t.Errorf("the kept task: %v", err)
	}
	if _, err := s.Task(f.gone.ULID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("the task that is gone = %v, want not found", err)
	}
	if _, ok, err := sync.ReadForcePush(s); err != nil || ok {
		t.Errorf("force-push.json = %v, %v", ok, err)
	}
	if _, err := f.clone.Sync(false, true); err != nil {
		t.Fatalf("the sync after --fix: %v", err)
	}
	if s, err = f.clone.OpenStore(); err != nil {
		t.Fatal(err)
	}
	if got, want := originHead(t, f.remote), mustHead(t, s); got != want {
		t.Errorf("origin is at %s, the clone at %s", got, want)
	}
}

// With nothing dropped no old store is kept, and the report says so.
func TestDoctorFixKeepsNoOldStoreWhenNothingWasDropped(t *testing.T) {
	t.Parallel()
	f := newForcePushFx(t, false)
	dir := f.path(t)
	if _, err := f.clone.Sync(false, true); !errors.Is(err, sync.ErrForcePush) {
		t.Fatalf("Sync = %v, want ErrForcePush", err)
	}
	rs, err := f.clone.Doctor(true)
	if err != nil || len(rs) != 1 || rs[0].Rebuilt == nil {
		t.Fatalf("Doctor = %+v, %v; want a rebuild", rs, err)
	}
	if b := rs[0].Rebuilt; b.Replayed != 1 || len(b.Dropped) != 0 || b.Kept != "" {
		t.Errorf("rebuilt = %+v", b)
	}
	if m, err := filepath.Glob(dir + ".*"); err != nil || len(m) != 0 {
		t.Errorf("something is left beside the store: %v, %v", m, err)
	}
}

// A store that was not force pushed is not rebuilt.
func TestDoctorFixDoesNotRebuildAStoreThatWasNotForcePushed(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	addBareRemote(t, ts)
	ts.addTask("a", nil)
	if _, err := ts.Sync(false, true); err != nil {
		t.Fatal(err)
	}
	rs, err := ts.Doctor(true)
	if err != nil || len(rs) != 1 || rs[0].Rebuilt != nil {
		t.Fatalf("Doctor = %+v, %v; want no rebuild", rs, err)
	}
}

// A write that lands between the rebuild and the swap stops the swap with a
// warning and no rebuild; the next doctor --fix makes it again.
func TestDoctorFixRebuildStaleIsAWarning(t *testing.T) {
	t.Parallel()
	f := newForcePushFx(t, true)
	if _, err := f.clone.Sync(false, true); !errors.Is(err, sync.ErrForcePush) {
		t.Fatalf("Sync = %v, want ErrForcePush", err)
	}
	var late *model.Task
	f.clone.BeforeSwap = func(string) {
		if late == nil {
			late = f.clone.addTask("late", nil)
		}
	}
	f.clone.warns = nil
	rs, err := f.clone.Doctor(true)
	if err != nil || len(rs) != 1 || rs[0].Rebuilt != nil {
		t.Fatalf("Doctor = %+v, %v; want no rebuild", rs, err)
	}
	if !slices.ContainsFunc(f.clone.warns, func(w string) bool { return strings.Contains(w, "run hedgehug doctor --fix again") }) {
		t.Errorf("warnings = %q", f.clone.warns)
	}
	if len(rs[0].Findings) != 1 || rs[0].Findings[0].Check != "unexpected_force_push" {
		t.Errorf("findings = %+v, want the force push still", rs[0].Findings)
	}
	if left, _ := sync.FindLeftover(f.path(t)); !left.None() {
		t.Errorf("leftover = %+v", left)
	}
	f.clone.BeforeSwap = nil
	rs, err = f.clone.Doctor(true)
	if err != nil || len(rs) != 1 || rs[0].Rebuilt == nil || rs[0].Rebuilt.Replayed != 3 {
		t.Fatalf("second Doctor = %+v, %v; want a rebuild of 3 commits", rs, err)
	}
	s, err := f.clone.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []model.ULID{late.ULID, f.mine.ULID} {
		if _, err := s.Task(u); err != nil {
			t.Errorf("%s: %v", u, err)
		}
	}
}

// A store whose swap stopped does not open, says why, and is finished by the
// next sync, which then finds the force push as usual.
func TestSyncFinishesAStoppedSwap(t *testing.T) {
	t.Parallel()
	f := newForcePushFx(t, true)
	dir := f.stoppedSwap(t)
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the store is there: %v", err)
	}
	f.clone.warns = nil
	_, err := f.clone.OpenStore()
	var le *sync.LeftoverError
	if err == nil || !strings.Contains(err.Error(), sync.LeftoverMessage("me")) || !errors.As(err, &le) {
		t.Errorf("OpenStore = %v", err)
	}
	if _, err := f.clone.Sync(false, true); !errors.Is(err, sync.ErrForcePush) {
		t.Fatalf("Sync = %v, want ErrForcePush after the swap is finished", err)
	}
	if left, _ := sync.FindLeftover(dir); !left.None() {
		t.Errorf("leftover = %+v", left)
	}
	s, err := f.clone.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Task(f.mine.ULID); err != nil {
		t.Error(err)
	}
}

// A store with a leftover beside it opens with a warning, once.
func TestOpenStoreWarnsOfALeftover(t *testing.T) {
	t.Parallel()
	f := newForcePushFx(t, true)
	dir := f.path(t)
	if err := os.MkdirAll(dir+".before-rewrite-1", 0o750); err != nil {
		t.Fatal(err)
	}
	f.clone.warns = nil
	for range 2 {
		if _, err := f.clone.OpenStore(); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.clone.warns; len(got) != 1 || got[0] != sync.LeftoverMessage("me") {
		t.Errorf("warnings = %q", got)
	}
	// the mount tells of it too, and the store is in it
	f.clone.warns = nil
	set, err := f.clone.Mount()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := set.Get("me"); !ok {
		t.Error("the store is not mounted")
	}
	if len(set.Unfinished) != 1 {
		t.Errorf("Unfinished = %+v", set.Unfinished)
	}
}

// doctor --fix finishes a swap that stopped, also when the store itself is
// missing; doctor without --fix tells that the store does not open.
func TestDoctorFixFinishesAStoppedSwap(t *testing.T) {
	t.Parallel()
	f := newForcePushFx(t, true)
	dir := f.stoppedSwap(t)
	rs, err := f.clone.Doctor(false)
	if err != nil || len(rs) != 1 || !doctor.Failing(rs[0].Findings) {
		t.Fatalf("Doctor of a store that is not there = %+v, %v; want findings", rs, err)
	}
	if _, err := f.clone.Doctor(true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(dir); err != nil {
		t.Errorf("the store does not open: %v", err)
	}
	if left, _ := sync.FindLeftover(dir); !left.None() {
		t.Errorf("leftover = %+v", left)
	}
}

// With the store there, a leftover is removed: the old store of a finished
// swap, and a rebuild never swapped in.
func TestDoctorFixRemovesTheOldStore(t *testing.T) {
	t.Parallel()
	f := newForcePushFx(t, true)
	dir := f.path(t)
	for _, suffix := range []string{".before-rewrite-1", ".rewrite-new"} {
		if err := os.MkdirAll(filepath.Join(dir+suffix, "x"), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.clone.Doctor(true); err != nil {
		t.Fatal(err)
	}
	if left, _ := sync.FindLeftover(dir); !left.None() {
		t.Errorf("leftover = %+v", left)
	}
}
```

`internal/ops/maintain_test.go` の `TestDoctorFindsAnUnexpectedForcePush`（377〜395 行）の最後の検査を次にする（コメントの `(D23)` はそのまま）。

```go
	f := rep.Findings[0]
	if f.Check != "unexpected_force_push" || f.Where != e.Name || !strings.Contains(f.Msg, "differs from this store's") || !strings.Contains(f.Hint, "doctor --fix") {
		t.Errorf("finding = %+v", f)
	}
```

`internal/ops/moved_test.go`: `TestUndoRefusesMovesAndRewrites` を `TestUndoRefusesMoves` に改名し、表から `{"rewrite-history", "a history rewrite cannot be undone"},` の行を消す。

- [ ] **Step 5: CLI のテストを書く（失敗する）**

`cmd/hedgehug/purge_rewrite_test.go` と `testdata/TestPurgeRewriteHistoryNeedsATerminal/`・`TestPurgeRewriteHistoryCanceled/`・`TestPurgeRewriteHistory/`・`TestPurgeRewriteHistoryJSON/`・`TestPurgeRewriteHistoryUsage/`（あるものだけ）を消す。

```bash
rm cmd/hedgehug/purge_rewrite_test.go
rm -r cmd/hedgehug/testdata/TestPurgeRewriteHistory*
```

`cmd/hedgehug/doctor_forcepush_test.go`（新規）:

```go
package main

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
)

// The option that rewrote the history is gone: an unknown flag is a usage error.
func TestPurgeRewriteHistoryIsGone(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	x := te.addJSON("x")
	code, _, errs := te.run("purge", "--rewrite-history", x.ID)
	if code != 2 || !strings.Contains(errs, "unknown flag: --rewrite-history") {
		t.Errorf("exit %d, stderr %q; want a usage error", code, errs)
	}
}

// The whole round: origin is force pushed, hh sync stops and says why, hh
// doctor tells of it, hh doctor --fix clones the store again with the
// unpushed work put back, and the sync after it pushes that work.
func TestDoctorFixClonesAForcePushedStoreAgain(t *testing.T) {
	t.Parallel()
	alice, bob, remote := twoUsers(t)
	bob.ok("add", "bobs")
	gitrepotest.RewriteRemote(t, remote)

	code, _, errs := bob.run("sync")
	if code != 1 || !strings.Contains(errs, "differs from this store's") {
		t.Errorf("sync: exit %d, stderr %q", code, errs)
	}
	code, out, _ := bob.run("doctor")
	if code != 1 || !strings.Contains(out, "org: the history of origin differs from this store's") || !strings.Contains(out, "hint: ") {
		t.Errorf("doctor: exit %d\n%s", code, out)
	}

	out = bob.ok("doctor", "--fix")
	if !strings.Contains(out, "Cloned org again from origin; put back 1 unpushed commit\n") || strings.Contains(out, "Could not put back") || !strings.Contains(out, "No problems") {
		t.Errorf("doctor --fix:\n%s", out)
	}
	if ls := bob.ok("ls"); !strings.Contains(ls, "bobs") || !strings.Contains(ls, "ORG-1") {
		t.Errorf("ls after the fix:\n%s", ls)
	}
	bob.ok("sync") // pushes the work onto the new history

	// alice followed the old history: her sync stops, and --fix catches her up
	code, _, errs = alice.run("sync")
	if code != 1 || !strings.Contains(errs, "differs from this store's") {
		t.Errorf("alice's sync: exit %d, stderr %q", code, errs)
	}
	out = alice.ok("doctor", "--fix")
	if !strings.Contains(out, "Cloned org again from origin; put back 0 unpushed commits\n") {
		t.Errorf("alice's doctor --fix:\n%s", out)
	}
	alice.ok("sync")
	if ls := alice.ok("ls"); !strings.Contains(ls, "bobs") {
		t.Errorf("alice's ls after the fix:\n%s", ls)
	}
}
```

同じファイルに、`doctor --fix --json` の `rebuilt` を確かめるテストを足す。

```go
// doctor --fix --json has the rebuild in "rebuilt"; a store that was not
// force pushed has none.
func TestDoctorFixJSONHasTheRebuild(t *testing.T) {
	t.Parallel()
	_, bob, remote := twoUsers(t)
	bob.ok("add", "bobs")
	var plain []struct {
		Rebuilt *struct{} `json:"rebuilt"`
	}
	if err := json.Unmarshal([]byte(bob.ok("doctor", "--fix", "--json")), &plain); err != nil || len(plain) != 1 || plain[0].Rebuilt != nil {
		t.Fatalf("doctor --fix --json of a store not force pushed: %+v, %v", plain, err)
	}
	gitrepotest.RewriteRemote(t, remote)
	bob.run("sync")
	var rs []struct {
		Rebuilt *struct {
			Replayed int      `json:"replayed"`
			Dropped  []string `json:"dropped"`
			Kept     string   `json:"kept"`
		} `json:"rebuilt"`
	}
	if err := json.Unmarshal([]byte(bob.ok("doctor", "--fix", "--json")), &rs); err != nil || len(rs) != 1 || rs[0].Rebuilt == nil {
		t.Fatalf("doctor --fix --json: %+v, %v", rs, err)
	}
	if r := rs[0].Rebuilt; r.Replayed != 1 || r.Dropped == nil || len(r.Dropped) != 0 || r.Kept != "" {
		t.Errorf("rebuilt = %+v", r)
	}
}
```

import に `"encoding/json"` を足す。

`cmd/hedgehug/doctor_rewrite_test.go` の `TestDoctorFixFinishesAStoppedRewriteSwap` を、`rewrite_leftover` の検査なしに直す（止まった入れ替えは `doctor` では「Store が開かない」の指摘になり、`--fix` が仕上げる）。ファイル全体を次にする。

```go
package main

import (
	"os"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/sync"
)

// A swap that stopped (the store moved away, the rebuilt one not yet in its
// place) is finished by doctor --fix.
func TestDoctorFixFinishesAStoppedRewriteSwap(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("add", "Alpha")
	dir := te.storeDir("me")
	if err := os.CopyFS(dir+".rewrite-new", os.DirFS(dir)); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(dir, dir+".before-rewrite-20260101T000000.000000000"); err != nil {
		t.Fatal(err)
	}

	if code, out, _ := te.run("doctor"); code == 0 || strings.Contains(out, "No problems") {
		t.Errorf("doctor of a store that is not there: exit %d\n%s", code, out)
	}
	if code, out, stderr := te.run("doctor", "--fix"); code != 0 || !strings.Contains(out, "No problems") {
		t.Errorf("--fix: exit %d\nstdout %s\nstderr %s", code, out, stderr)
	}
	if left, err := sync.FindLeftover(dir); err != nil || !left.None() {
		t.Errorf("leftover = %+v, %v", left, err)
	}
	if out := te.ok("ls"); !strings.Contains(out, "Alpha") {
		t.Errorf("ls after the fix:\n%s", out)
	}
}
```

`cmd/hedgehug/e2e_3c_test.go` から手順 5 と、それだけが使うものを消す。

1. `TestE2EPhase3c` の直前のコメントの 2 行目 `// other members see of it, and the rewrite of the history.` を `// other members see of it.` に直す。
2. 157〜159 行の、`// the paths of the child and of bob's comment on it, while he has them` と `childPaths := pathsWith(t, bob, "org-a", child.ULID)` の 2 行を消す。
3. 226〜231 行の、`// the paths of everything the rewrite must remove from bob's history:` から `t.Fatalf("paths of the project, the child and the comment = %v", purged)` と `}` までを消す。
4. `// 5. bob has an edit not pushed;` のコメントの行から、`inbox = bob.ok("inbox")` ・`if !strings.Contains(inbox, "rewrite_applied") { … }` までを消す。関数は、手順 4 の最後の `bob.ok("sync")` のあと、`}` で終わる。
5. `inProcess`・`pathsWith`・`inHistory` を関数ごとコメント付きで消す（`overwrittenNaming`・`storeLog`・`taskFilePath` は残す）。
6. import の `"bytes"` と `"slices"` を消す（他は使われている）。

```bash
go vet ./cmd/hedgehug
```

が通ること（未使用の変数・import が残っていれば指摘される）。

- [ ] **Step 6: 失敗を確かめる**

Run: `go vet ./internal/sync ./internal/ops ./cmd/hedgehug 2>&1 | head -30`（timeout 600000）
Expected: ビルドの失敗。`undefined: MakeRebuild`（`internal/sync`）、`unknown field Rebuilt`／`rs[0].Rebuilt undefined`（`internal/ops`）。`cmd/hedgehug` は、`doctor_forcepush_test.go` が `gitrepotest.RewriteRemote` を参照するのでこの時点では通るが、テストの実行は失敗する（Step 13 で通す）。

- [ ] **Step 7: sync を実装する**

(a) `internal/sync/rewrite.go` を `internal/sync/forcepush.go` に改名し（`git mv`）、中身を次の全文にする。

```go
package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/store"
)

const forcePushFile = "force-push.json"

// ErrForcePush means the history of origin is not one the store follows: it
// was rewritten or reset. The store is as it was; hedgehug doctor --fix
// clones it again.
var ErrForcePush = errors.New("the history of origin differs from this store's; check origin, then run hedgehug doctor --fix to clone it again")

// ForcePush is force-push.json: the force push the last sync found.
type ForcePush struct {
	Origin  string    `json:"origin"`  // the new head of origin
	Tracked string    `json:"tracked"` // the origin the clone had followed
	Checked time.Time `json:"checked"`
}

// ReadForcePush reads force-push.json; ok is false when there is none.
func ReadForcePush(s *store.Store) (ForcePush, bool, error) {
	var f ForcePush
	b, err := s.ReadLocal(forcePushFile)
	if errors.Is(err, store.ErrNotFound) {
		return f, false, nil
	}
	if err != nil {
		return f, false, err
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return ForcePush{}, false, fmt.Errorf("%s: %w", forcePushFile, err)
	}
	return f, true, nil
}

func writeForcePush(s *store.Store, f ForcePush) error {
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return s.WriteLocal(forcePushFile, append(b, '\n'))
}

// checkOrigin compares the head of origin after a fetch with tracked, the
// value refs/remotes/origin/main had before it. A head that does not
// descend from tracked is a force push: it puts the tracking ref back, so
// nothing of the clone changes, writes force-push.json and returns
// ErrForcePush. Otherwise it removes force-push.json.
func checkOrigin(s *store.Store, tracked plumbing.Hash, opt Options) error {
	r := s.Repo()
	head, err := r.RemoteHead("origin")
	if err != nil {
		return err
	}
	follows := true
	if !head.IsZero() {
		if follows, err = r.IsAncestor(tracked, head); err != nil {
			return err
		}
	}
	if follows {
		// A missing file is not an error for RemoveLocal, so this writes
		// nothing in the usual sync.
		return s.RemoveLocal(forcePushFile)
	}
	if err := r.SetTracking("origin", tracked); err != nil {
		return err
	}
	if err := writeForcePush(s, ForcePush{Origin: head.String(), Tracked: tracked.String(), Checked: opt.Now()}); err != nil {
		return err
	}
	return ErrForcePush
}
```

(b) `internal/sync/sync.go`:

`Result` から、`Rewrite`・`Rebuild`・`Swapped, Reopen` を消す（次の 3 つのコメント付きのかたまり）。

```go
	// Rewrite is the history rewrite of origin that is recorded and not yet
	// taken in; the sync stopped before changing the store.
	Rewrite *model.Rewrite
	// Rebuild is the clone made again for Rewrite, ready to be swapped in;
	// Run does not swap it (the caller releases the locks first). The store
	// is as it was.
	Rebuild *Rebuild
	// Swapped is set by the caller that put Rebuild in the place of the
	// store. Reopen is set by a caller that closed the Store handle the sync
	// was given (it swapped, or tried to): the caller of that opens it again.
	Swapped, Reopen bool
```

`Run` の、`diverged` の宣言を消す。

```go
	down := map[string]error{}
	diverged := map[string]bool{}
	for round := 0; ; round++ {
```
→
```go
	down := map[string]error{}
	for round := 0; ; round++ {
```

origin の検査を直す。

```go
			if i == 0 {
				if err := checkOrigin(s, tracked, opt, &res); err != nil {
					return res, err
				}
				if res.Rewrite != nil {
					return res, rebuild(s, tracked, opt, &res)
				}
			}
```
→
```go
			if i == 0 {
				if err := checkOrigin(s, tracked, opt); err != nil {
					return res, err
				}
			}
```

古い履歴のミラーの扱いを消し、どのリモートも同じに merge する。`// A mirror that still has the history before a rewrite is not merged` のコメントから、`for i, name := range up {` … `mergeRemote` … `}` の閉じ括弧までを次に置き換える。

```go
		for _, name := range up {
			if err := mergeRemote(s, name, opt, &res); err != nil {
				return res, err
			}
		}
```

push のところ。

```go
		for _, name := range up[1:] {
			if left[name] {
				continue
			}
			var err error
			if mirror, old := stale[name]; old {
				err = r.ForcePush(name, mirror)
			} else {
				err = r.Push(name)
			}
			switch {
```
→
```go
		for _, name := range up[1:] {
			err := r.Push(name)
			switch {
```

警告のところ。

```go
		warn("sync mirror "+name, down[name])
		if diverged[name] {
			res.Warnings = append(res.Warnings, name+" has commits that are in neither history; it is not pushed")
		}
```
→
```go
		warn("sync mirror "+name, down[name])
```

(c) `internal/sync/rebuild.go`:

定数に古い Store を残す場合の名前を足す。

```go
const (
	newSuffix    = ".rewrite-new"
	staleMarker  = ".rebuild-stale" // in the rebuild: the old store is the one to keep
	beforePrefix = ".before-rewrite-"
	// keptPrefix names the old store that Swap leaves beside the new one
	// when the rebuild could not put back all its unpushed changes. It is
	// not a leftover (FindLeftover does not see it): nothing removes it.
	keptPrefix = ".kept-"
)
```

`dirtyRebuildError` の文言:

```go
func (dirtyRebuildError) Error() string {
	return "the store has uncommitted changes, which the rebuild would drop; commit or restore them, then run hedgehug doctor --fix"
}
```

`Rebuild` 型と `rebuild` 関数を次に置き換える（`recordsAt` はこの置き換えで消える）。

```go
// Rebuild is a clone made again from origin after a force push, ready to
// take the place of the store.
type Rebuild struct {
	Dir      string        // <path>.rewrite-new
	OldHead  plumbing.Hash // HEAD of the store when it was made
	Replayed int           // the unpushed changes (commits) put back
	// Purged are the ids of the tasks whose unpushed changes were not put
	// back, because the new history lacks the task (the ULID for one the
	// old history has no id of), and PurgedTasks their ULIDs, in the same
	// order.
	Purged      []string
	PurgedTasks []model.ULID
	// Notices are the items put in the inbox of the rebuild; whoever swaps
	// it in tells of them.
	Notices []inbox.Item
	// Kept is where Swap left the old store; "" when it removed it. The old
	// store is kept when Purged is not empty.
	Kept string

	seen map[string]stamp // the local files copyLocal copied
}

// MakeRebuild makes the clone again from origin, for hedgehug doctor --fix
// after a sync stopped with ErrForcePush: origin is cloned beside the store
// and the unpushed changes of the store are put back on it as one commit.
// The store is as it was. When origin has become a history the store
// follows again, it removes force-push.json and returns nil. The caller
// holds sync.lock; the store lock is held here for the whole rebuild, and a
// write that lands after it is let go is found by Swap.
func MakeRebuild(s *store.Store, opt Options) (b *Rebuild, err error) {
	if opt.Now == nil {
		opt.Now = time.Now
	}
	r := s.Repo()
	r.SetConn(opt.Conn)
	held, err := store.LockInOrder(s)
	if err != nil {
		return nil, err
	}
	defer held.Release()
	st, err := r.Status()
	if err != nil {
		return nil, err
	}
	if len(st) > 0 {
		// The swap would drop the changes that were not committed.
		return nil, ErrDirtyRebuild
	}
	oldHead, err := r.Head()
	if err != nil {
		return nil, err
	}
	// What the store followed: the sync that found the force push put the
	// tracking ref back.
	tracked, err := r.RemoteHead("origin")
	if err != nil {
		return nil, err
	}
	dir := s.Dir()
	newDir := dir + newSuffix
	// The store is here, so a rebuild left beside it is of a swap that
	// never began.
	if err := os.RemoveAll(newDir); err != nil {
		return nil, err
	}
	nr, err := gitrepo.CloneWith(opt.Remotes[0], newDir, opt.Conn)
	if err != nil {
		return nil, fmt.Errorf("rebuild after a force push: %w", err)
	}
	defer func() {
		_ = nr.Close()
		if err != nil || b == nil {
			_ = os.RemoveAll(newDir)
		}
	}()
	if err = nr.SetRemotes(opt.Remotes); err != nil {
		return nil, err
	}
	if err = nr.RegisterMergeDriver(); err != nil {
		return nil, err
	}
	head, err := nr.Head()
	if err != nil {
		return nil, err
	}
	// The clone's head came after the last fetch, so the old repository may
	// lack its objects; the replay reads both histories there.
	if _, err = r.TreeOf(head); errors.Is(err, plumbing.ErrObjectNotFound) {
		if _, err = r.Fetch("origin"); err == nil {
			err = r.SetTracking("origin", tracked)
		}
	}
	if err != nil {
		return nil, err
	}
	follows, err := r.IsAncestor(tracked, head)
	if err != nil {
		return nil, err
	}
	if follows {
		// origin went back to a history the store follows: the next sync
		// takes it as usual.
		return nil, s.RemoveLocal(forcePushFile)
	}
	b = &Rebuild{Dir: newDir, OldHead: oldHead}
	ns, err := store.Open(newDir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = ns.Repo().Close() }()
	ns.SetNow(opt.Now)
	newHead, err := replay(r, ns, nr, tracked, oldHead, head, opt, b)
	if err != nil {
		return nil, err
	}
	if err = copyLocal(s, ns, b); err != nil {
		return nil, err
	}
	// The inbox goes on from where the new history has the commit it had
	// followed: what came before is what it has told of, what comes after is
	// new to it, and so is the replay commit.
	mark, found, err := r.FindSameCommit(head, tracked)
	if err != nil {
		return nil, err
	}
	if !found {
		mark = head
	}
	if err = inbox.Open(ns).SetMark(mark); err != nil {
		return nil, err
	}
	if b.Notices, err = tellRewrite(ns, newHead, opt, b); err != nil {
		return nil, err
	}
	return b, nil
}
```

`replay` と `findPurgedChanges` を次に置き換える（`recordsAt` と `taskOfPath` のあいだ、`taskOfPath` はそのまま）。

```go
// replay puts the unpushed changes of oldHead back on head, the new head of
// origin, as one commit in ns; it returns the head of ns then. The files of
// the tasks the new history lacks are left out of the merge, and a task of
// them that the unpushed commits changed goes to b.Purged.
func replay(r *gitrepo.Repo, ns *store.Store, nr *gitrepo.Repo, tracked, oldHead, head plumbing.Hash, opt Options, b *Rebuild) (plumbing.Hash, error) {
	mine, err := r.Between(oldHead, tracked)
	if err != nil {
		return head, err
	}
	if len(mine) == 0 {
		return head, nil
	}
	b.Replayed = len(mine)
	base, err := r.MergeBase(oldHead, tracked)
	if err != nil {
		return head, err
	}
	if base.IsZero() {
		return head, fmt.Errorf("replay: %w", ErrUnrelated)
	}
	dropped, err := findDropped(r, nr, base, oldHead, head, b)
	if err != nil {
		return head, err
	}
	skip := func(p string) bool {
		u, ok := taskOfPath(p)
		return ok && dropped[u]
	}
	// The merge is computed from the new head's side, so that its files are
	// the changes to make to the clone; the rule is symmetric.
	opt.origin, opt.skip = head, skip
	m, err := mergeTrees(r, base, head, oldHead, opt)
	if err != nil {
		return head, err
	}
	// What the new history already has is no change.
	for p, data := range m.files {
		cur, err := nr.ReadAt(head, p)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return head, err
		}
		if (data == nil && err != nil) || (data != nil && err == nil && string(cur) == string(data)) {
			delete(m.files, p)
		}
	}
	// A commit without a file records what lost, as a merge commit does.
	if len(m.files) == 0 && len(m.lost) == 0 && len(m.notices) == 0 && len(m.repaired) == 0 {
		return head, nil
	}
	op := store.Op{Name: "replay", Detail: fmt.Sprintf("%d unpushed commits", len(mine)), Author: opt.Author, Account: opt.Account, AllowEmpty: true}
	return ns.CommitMerge(head, []plumbing.Hash{head}, m.files, op, m.trailers())
}

// findDropped puts in b the tasks that the commits from base to oldHead
// changed (the task, a comment or a worklog entry) and that the new history
// lacks: the base has the task's file and the new head does not. A task
// that only the unpushed commits have is not one of them. It returns them as
// a set.
func findDropped(r, nr *gitrepo.Repo, base, oldHead, head plumbing.Hash, b *Rebuild) (map[model.ULID]bool, error) {
	bt, err := r.TreeOf(base)
	if err != nil {
		return nil, err
	}
	ot, err := r.TreeOf(oldHead)
	if err != nil {
		return nil, err
	}
	changes, err := r.DiffTrees(bt, ot, "")
	if err != nil {
		return nil, err
	}
	dropped := map[model.ULID]bool{}
	seen := map[model.ULID]bool{}
	for _, c := range changes {
		u, ok := taskOfPath(c.Path)
		if !ok || seen[u] {
			continue
		}
		seen[u] = true
		data, err := r.ReadFile(bt, model.TaskPath(u))
		if errors.Is(err, fs.ErrNotExist) {
			continue // made by the unpushed commits
		}
		if err != nil {
			return nil, err
		}
		if _, err := nr.ReadAt(head, model.TaskPath(u)); err == nil {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		id := u.String()
		if t, err := model.DecodeTask(data); err == nil && t.ID != "" {
			id = t.ID
		}
		dropped[u] = true
		b.PurgedTasks = append(b.PurgedTasks, u)
		b.Purged = append(b.Purged, id)
	}
	return dropped, nil
}
```

`skipLocal` から `name == "rewrite-in-progress.json" ||` を消し、`ownedLocal` を `[]string{"inbox-head"}` にする。

```go
func skipLocal(name string) bool {
	return name == "lock" || strings.HasSuffix(name, ".lock") || name == "index" || name == forcePushFile ||
		strings.HasPrefix(name, ".")
}

// ownedLocal are the files the rebuild writes in its own way, so that the
// copy made right before the swap leaves them alone.
var ownedLocal = []string{"inbox-head"}
```

`Swap` に、古い Store を残す場合を足す。doc コメントの末尾に 1 文足す: `Swap leaves the old store at b.Kept, beside the new one, when b.Purged is not empty (keptPrefix).`。本体は 2 か所:

```go
	before := dir + beforePrefix + time.Now().UTC().Format("20060102T150405.000000000")
```
→
```go
	prefix := beforePrefix
	if len(b.Purged) > 0 {
		prefix = keptPrefix
	}
	before := dir + prefix + time.Now().UTC().Format("20060102T150405.000000000")
```

関数の最後の `_ = os.RemoveAll(before)` / `return nil` を次にする。

```go
	if len(b.Purged) > 0 {
		b.Kept = before
		return nil
	}
	_ = os.RemoveAll(before)
	return nil
```

`recordsAt` を消す。import の `slices` は `syncLocal`・`carryMissing` が使うので残る。`encoding/json` も `tellRewrite` が使う。

(d) `internal/sync/background.go`: `Background` と `BackgroundThen` を 1 つにする（`then` は入れ替えのためだけにあった）。

```go
// Background is the body of sync --background: wait for push_interval after
// pending_since, run, repeat while something is pending. run takes sync.lock
// itself. A run that fails
// ends it, since run keeps the error in sync.json for the next command to
// show and to start another.
func Background(s *store.Store, opt Options, pushInterval time.Duration, run func() error) error {
	now := opt.Now
	if now == nil {
		now = time.Now
	}
	err := s.WithLocalLock(bgLock, 0, func() error {
		for {
			st, err := ReadState(s)
			if err != nil {
				return err
			}
			if !st.PendingSince.IsZero() {
				if d := st.PendingSince.Add(pushInterval).Sub(now()); d > 0 {
					time.Sleep(d)
				}
			}
			if err := run(); err != nil {
				return err
			}
			if st, err = ReadState(s); err != nil {
				return err
			}
			if st.PendingSince.IsZero() {
				return nil
			}
		}
	})
	if errors.Is(err, store.ErrLocked) {
		return nil // another one waits already
	}
	return err
}
```

`BoundLog`〜`ReopenAt` の部分（`BoundLog` の関数から `ReopenAt` の終わりまで）を、ログを 1 MiB に収めるだけの形に置き換える。

```go
// BoundLog returns w, which is the stderr of the child that Start started,
// as a writer that keeps sync.log within logMax while the child runs.
func BoundLog(s *store.Store, w io.Writer) *BoundedLog {
	return &BoundedLog{w: w, path: logPath(s)}
}

// BoundedLog trims the log after each write that makes it larger than
// logMax. w appends to the file at path.
type BoundedLog struct {
	mu   gosync.Mutex
	w    io.Writer
	path string
}

func (b *BoundedLog) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n, err := b.w.Write(p)
	if terr := trimLog(b.path); err == nil {
		err = terr
	}
	return n, err
}
```

`internal/sync/childlog_unix.go` を消す（`redirectStderr` はこれだけが使っていた。`golang.org/x/sys` は `internal/store/lock_unix.go` が使うので go.mod は変わらない）。

(e) `go build ./internal/sync` が通るまで、未使用の import を直す。

- [ ] **Step 8: gitrepo を実装する**

`internal/gitrepo/rewrite.go` を消し、使うものを移す。

`internal/gitrepo/samecommit.go`（新規）:

```go
package gitrepo

import (
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// FindSameCommit returns a commit of the history of from that has the same
// author and committer (name, email and time) as the commit like: a history
// rewrite keeps these, so it finds where like went. Both commits must be in
// the repository. Commits of one author and committer in the same second
// cannot be told apart, so it returns the oldest of them that the walk from
// from meets (a line of history is walked from the newest): a caller that
// goes on from it sees the commits again at worst, never misses one.
func (r *Repo) FindSameCommit(from, like plumbing.Hash) (plumbing.Hash, bool, error) {
	want, err := r.commit(like)
	if err != nil {
		return plumbing.ZeroHash, false, err
	}
	start, err := r.commit(from)
	if err != nil {
		return plumbing.ZeroHash, false, err
	}
	same := func(a, b object.Signature) bool {
		return a.Name == b.Name && a.Email == b.Email && a.When.Equal(b.When)
	}
	var found plumbing.Hash
	err = object.NewCommitPreorderIter(start, nil, nil).ForEach(func(c *object.Commit) error {
		if same(c.Author, want.Author) && same(c.Committer, want.Committer) {
			found = c.Hash
		}
		return nil
	})
	return found, !found.IsZero(), err
}
```

`internal/gitrepo/remote.go`: `trackingRef` の直後（101 行の関数のあと）に `SetTracking` を足す。

```go
// SetTracking sets refs/remotes/<remote>/main to h.
func (r *Repo) SetTracking(remote string, h plumbing.Hash) error {
	return r.r.Storer.SetReference(plumbing.NewHashReference(trackingRef(remote), h))
}
```

同じ `remote.go` の `Push` と `push` を、強制の分岐なしの 1 つにする。

```go
func (r *Repo) Push(remote string) error { return r.push(remote, false, plumbing.ZeroHash) }

// push sends main to the remote; force sends it even when the remote is not
// its ancestor.
func (r *Repo) push(remote string, force bool, lease plumbing.Hash) error {
```
→
```go
func (r *Repo) Push(remote string) error {
```

```go
	spec := h.String() + ":refs/heads/main"
	if force {
		spec = "+" + spec
	}
	opts := &git.PushOptions{
		RemoteName: remote,
		RemoteURL:  to,
		Auth:       auth,
		RefSpecs:   []config.RefSpec{config.RefSpec(spec)},
		Force:      force,
	}
	if force && !lease.IsZero() {
		// go-git checks a lease for a ref source only, not for a hash
		// source; the caller holds the store lock, so main is h.
		opts.RefSpecs = []config.RefSpec{config.RefSpec("+" + mainRef.String() + ":refs/heads/main")}
		opts.ForceWithLease = &git.ForceWithLease{RefName: mainRef, Hash: lease}
	}
	err = p.r.Push(opts)
```
→
```go
	err = p.r.Push(&git.PushOptions{
		RemoteName: remote,
		RemoteURL:  to,
		Auth:       auth,
		RefSpecs:   []config.RefSpec{config.RefSpec(h.String() + ":refs/heads/main")},
	})
```

`internal/gitrepo/repo.go` の末尾に `Close` を足す。

```go
// Close lets go of the files the repository holds open (the object
// storage), so its directory can be renamed on Windows.
func (r *Repo) Close() error {
	if c, ok := r.r.Storer.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}
```

`go build ./internal/gitrepo` が通るまで import を直す。

- [ ] **Step 9: model の記録を消す**

```bash
git rm internal/model/rewrites.go internal/model/rewrites_test.go
```

- [ ] **Step 10: ops を実装する**

(a) 消す。

```bash
git rm internal/ops/rewrite.go
git mv internal/ops/rewrite_take.go internal/ops/rebuild.go
```

(b) `internal/ops/rebuild.go` を次の全文にする。`finishRebuild` はそのまま（コメントの「history rewrite」は止まった入れ替えの話として残す）。`rewriteLeftoverFinding` は消し、`Rebuilt` と `rebuildStore` を足す。

```go
package ops

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// finishRebuild completes a swap that stopped, or removes what a rebuild
// left, for the store e, before it is opened. Nothing left is nothing done.
// While the store is there, the leftover is touched only under sync.lock: a
// rebuild holds it, so a rebuild in the making is not taken for a leftover.
func (ss *Session) finishRebuild(e config.StoreEntry) error {
	l, err := sync.FindLeftover(e.Path)
	if err != nil || l.None() {
		return err
	}
	s, err := store.Open(e.Path)
	if err != nil {
		// No store (or a broken one) to lock: the swap is finished as it is.
		if _, serr := os.Lstat(e.Path); errors.Is(serr, fs.ErrNotExist) {
			return sync.FinishLeftover(e.Path, l)
		}
		return nil // the store's own problem; the sync tells of it
	}
	defer func() { _ = s.Repo().Close() }()
	return s.WithLocalLock("sync.lock", syncLockWait, func() error {
		l, err := sync.FindLeftover(e.Path)
		if err != nil || l.None() {
			return err
		}
		return sync.FinishLeftover(e.Path, l)
	})
}

// Rebuilt is what doctor --fix did to a store a sync found force pushed: it
// cloned the store again from origin.
type Rebuilt struct {
	Replayed int      `json:"replayed"`       // the unpushed commits put back, as one commit
	Dropped  []string `json:"dropped"`        // the ids of the tasks the new history lacks, whose unpushed changes were not put back
	Kept     string   `json:"kept,omitempty"` // where the old store is, when some changes were not put back
}

// rebuildStore clones the store e again from origin and puts the unpushed
// changes back (sync.MakeRebuild), when a sync found its origin force pushed
// (force-push.json). It swaps the new store in after the sync lock is let
// go, and does not push. It returns nil when the store was not force pushed,
// does not open, origin follows it again, or the store changed while it was
// made again (a warning says to run doctor --fix again).
func (ss *Session) rebuildStore(c *config.Config, e config.StoreEntry) (*Rebuilt, error) {
	s, err := ss.openStoreAt(e.Path)
	if err != nil {
		return nil, nil // the findings tell of a store that does not open
	}
	if _, ok, err := sync.ReadForcePush(s); err != nil || !ok {
		return nil, err
	}
	opt, err := ss.SyncOptions(c, e)
	if err != nil {
		return nil, err
	}
	var b *sync.Rebuild
	err = s.WithLocalLock("sync.lock", syncLockWait, func() error {
		var err error
		b, err = sync.MakeRebuild(s, opt)
		return err
	})
	if err != nil || b == nil {
		return nil, err
	}
	dir := s.Dir()
	if ss.BeforeSwap != nil {
		ss.BeforeSwap(dir)
	}
	ss.closeMounted(dir)
	ss.dropMount()
	if err := s.Repo().Close(); err != nil {
		sync.Discard(b)
		return nil, err
	}
	err = sync.Swap(dir, b)
	if errors.Is(err, sync.ErrRebuildStale) {
		ss.warn(fmt.Sprintf("%s changed while it was cloned again, so it was not replaced; run hedgehug doctor --fix again", e.Name))
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if left, lerr := sync.FindLeftover(dir); lerr == nil && !left.None() {
		ss.warn(sync.LeftoverMessage(e.Name))
	}
	return &Rebuilt{Replayed: b.Replayed, Dropped: append([]string{}, b.Purged...), Kept: b.Kept}, nil
}
```

(c) `internal/ops/maintain.go`:

`DoctorReport` に足す。

```go
type DoctorReport struct {
	Store    string           `json:"store"`
	Rebuilt  *Rebuilt         `json:"rebuilt,omitempty"`
	Repaired []Fixed          `json:"repaired"`
	Findings []doctor.Finding `json:"findings"`
}
```

`Doctor` の、`// A swap after a history rewrite that stopped is finished first` のコメントから `finishErr` のループの終わりまでを次にする。

```go
	// A swap that stopped is finished first, so the store opens. A store a
	// sync found force pushed is then cloned again, before the repairs: they
	// are made on the new history. One that fails is that store's finding;
	// the others go on.
	finishErr := map[string]error{}
	rebuildErr := map[string]error{}
	rebuilt := make([]*Rebuilt, len(entries))
	if fix {
		for i, e := range entries {
			if err := ss.finishRebuild(e); err != nil {
				finishErr[e.Name] = err
				continue
			}
			var err error
			if rebuilt[i], err = ss.rebuildStore(c, e); err != nil {
				rebuildErr[e.Name] = err
			}
		}
	}
```

報告を作るループの、`finishErr` の扱いのところを次にする（`rewrite_leftover` の指摘をやめ、`git` の指摘にする。作り直しの失敗は `unexpected_force_push` の指摘）。

```go
		r, err := ss.doctorStore(e, len(entries) == 1, repaired[i], duplicatesOnce)
		if ferr := finishErr[e.Name]; ferr != nil {
			if err != nil {
				// the store did not open for want of the finish: say that
				r, err = DoctorReport{Store: e.Name, Repaired: repaired[i]}, nil
			}
			r.Findings = append(r.Findings, doctor.Finding{Check: "git", Where: e.Path, Msg: "cannot finish the rebuild after a history rewrite: " + ferr.Error(), Hint: "fix the cause and run hedgehug doctor --fix again"})
		}
		if rerr := rebuildErr[e.Name]; rerr != nil {
			if err != nil {
				r, err = DoctorReport{Store: e.Name, Repaired: repaired[i]}, nil
			}
			r.Findings = append(r.Findings, doctor.Finding{Check: "unexpected_force_push", Where: e.Name, Msg: "cannot clone the store again: " + rerr.Error(), Hint: "fix the cause and run hedgehug doctor --fix again"})
		}
		if err != nil {
			return nil, err
		}
		r.Rebuilt = rebuilt[i]
		out = append(out, r)
```

`doctorStore` から止まった入れ替えの指摘を消す。

```go
	r := DoctorReport{Store: e.Name, Repaired: repaired}
	left, err := rewriteLeftoverFinding(e)
	if err != nil {
		return DoctorReport{}, err
	}
	if _, serr := os.Lstat(e.Path); os.IsNotExist(serr) && len(left) > 0 {
		// The swap after a history rewrite stopped before the rebuilt store
		// took the place: nothing else can be said until it is finished.
		r.Findings = left
		return r, nil
	}
	// A store that does not open is told of by the findings, which say why;
	// its references are not looked at.
	opened, openErr := ss.openStoreAt(e.Path)
	fs, err := doctor.Run(e.Path)
	if err != nil {
		return DoctorReport{}, err
	}
	fs = append(fs, left...)
```
→
```go
	r := DoctorReport{Store: e.Name, Repaired: repaired}
	// A store that does not open is told of by the findings, which say why;
	// its references are not looked at.
	opened, openErr := ss.openStoreAt(e.Path)
	fs, err := doctor.Run(e.Path)
	if err != nil {
		return DoctorReport{}, err
	}
```

`forcePushFinding` の指摘:

```go
	return []doctor.Finding{{Check: "unexpected_force_push", Where: name, Msg: sync.ErrForcePush.Error(), Hint: "hedgehug doctor --fix clones the store again from origin and puts the unpushed changes back; the old store is kept when some could not be put back"}}, nil
```

コメント（`// forcePushFinding is unexpected_force_push (D23): …`）の「without a record」を消す: `// forcePushFinding is unexpected_force_push (D23): the last sync found the history of origin differing from the store's, and the file it left is still there.`

`go build ./internal/ops` が通るまで import（`os` など）を直す。

(d) `internal/ops/session.go`:

`BeforeSwap` のコメントを直す。

```go
	// BeforeSwap is told the path of a store before doctor --fix swaps the
	// directory of a rebuild in for it (after a force push): whoever holds
	// that store open, such as the GUI server's index, lets go of it.
```

`RunSync` から `swapRebuilt` の終わり（`Unpushed` の直前）までを次に置き換える。

```go
// RunSync syncs s under .hedgehug/local/sync.lock, and keeps in sync.json
// how it went. A write that was pending when the sync began has been pushed
// by it; one made later has not. reported says that the one who ran it is
// told of a failure, so the next command does not tell of it again. A
// history of origin that differs from the store's stops it (ErrForcePush);
// hedgehug doctor --fix clones the store again.
func (ss *Session) RunSync(s *store.Store, opt sync.Options, reported bool) (res sync.Result, err error) {
	// The sync writes objects through the handle s; the stores mounted are
	// opened again after it, so that they see them.
	defer ss.dropMount()
	err = s.WithLocalLock("sync.lock", syncLockWait, func() error {
		started := ss.Now()
		if _, err := sync.UpdateState(s, func(st *sync.State) { st.LastAttempt = started }); err != nil {
			return err
		}
		var runErr error
		res, runErr = sync.Run(s, opt)
		_, stateErr := sync.UpdateState(s, func(st *sync.State) {
			if runErr != nil {
				st.LastError = runErr.Error()
				st.ErrorReported = reported
				return
			}
			st.LastError, st.ErrorReported = "", false
			st.LastSuccess = ss.Now()
			// A write made while the sync ran finds pending_since set and
			// leaves it; it stays pending while HEAD has commits origin lacks.
			// Counted under the state lock, a commit after the count marks
			// pending again.
			if n, err := Unpushed(s); st.PendingSince.Before(started) && err == nil && n == 0 {
				st.PendingSince = time.Time{}
			}
		})
		return errors.Join(runErr, stateErr)
	})
	return res, err
}
```

(e) `internal/ops/sync.go`:

`SyncStore` の `res.Reopen` を消す。

```go
	res, err := ss.RunSync(s, opt, true)
	if res.Reopen {
		// s was closed for the swap.
		var oerr error
		if s, oerr = ss.openStoreAt(e.Path); oerr != nil {
			err = errors.Join(err, oerr)
		}
	}
	if err != nil {
		return r, err
	}
```
→
```go
	res, err := ss.RunSync(s, opt, true)
	if err != nil {
		return r, err
	}
```

`SyncStore` の冒頭のコメントを直す: `// A swap that stopped is finished first.`（`(spec: 作り直しと未 push の変更の取り込み)` を消す）。

`SyncBackground` を次に置き換える（入れ替えの後始末がなくなる）。関数の直前のコメントは変えない。`reopenLog` は関数ごと消す。

```go
func (ss *Session) SyncBackground(log io.Writer) error {
	c, _, err := ss.Config()
	if err != nil {
		return backgroundFailed(ss, log, "", err)
	}
	if ss.StoreName == "" {
		return newErr(KindUsage, "sync.background_needs_store", errors.New("--background needs --store"))
	}
	e, err := c.Select(ss.StoreName)
	if err != nil {
		return backgroundFailed(ss, log, "", err)
	}
	if err := ss.finishRebuild(e); err != nil {
		return backgroundFailed(ss, log, e.Name, err)
	}
	s, err := ss.openStoreAt(e.Path)
	if err != nil {
		return backgroundFailed(ss, log, e.Name, err)
	}
	log = sync.BoundLog(s, log)
	opt, err := ss.SyncOptions(c, e)
	if err == nil {
		err = sync.Background(s, opt, c.Sync.PushInterval, func() error {
			res, err := ss.RunSync(s, opt, false)
			for _, w := range res.Warnings { // such as a mirror that is down; for sync.log
				_, _ = fmt.Fprintln(log, "warning: "+w)
			}
			return err
		})
	}
	if err != nil {
		// The same text a failed run left in sync.json already; the others
		// (SyncOptions, the lock) are told of by the next command.
		_, stateErr := sync.UpdateState(s, func(st *sync.State) {
			st.LastError, st.ErrorReported = err.Error(), false
		})
		err = errors.Join(err, stateErr)
	}
	return backgroundFailed(ss, log, e.Name, err)
}
```

`go build ./internal/ops` が通るまで import（`os`・`time` など）を直す。

(f) `internal/ops/undo.go` の `undoBlocked` から `"rewrite-history": "a history rewrite cannot be undone",` の行を消す。

(g) `internal/doctor/finding.go` の `checkOrder` から `, "rewrite_leftover"` を消す。

(h) コメントを直す: `internal/server/server.go` 152〜153 行 `// A sync that swaps a rebuilt store in (after a history rewrite) needs the files of the old one closed.` を `// doctor --fix swaps a rebuilt store in (after a force push) and needs the files of the old one closed.` に、`internal/server/poll.go` 86〜87 行 `// swapped for a rebuild after a history rewrite (D26)` を `// swapped for a rebuild after a force push (D26)` に。

- [ ] **Step 11: cmd を実装する**

(a) 消す。

```bash
git rm cmd/hedgehug/purge_rewrite.go
```

(b) `cmd/hedgehug/rm.go` の `purgeCmd` を次にする。

```go
func (a *app) purgeCmd() *cobra.Command {
	var yes, moved bool
	cmd := &cobra.Command{
		Use:   "purge <id> [--yes]\n  hedgehug purge --moved [--store <store>] [--yes]",
		Short: "Remove the files of a deleted task and the tasks under it, for good; or the forward files of moved tasks",
		Args: func(cmd *cobra.Command, args []string) error {
			if moved {
				if len(args) > 0 {
					return usageErrorf("--moved takes no task id")
				}
				return nil
			}
			return exactArgs(1)(cmd, args)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			if moved {
				return a.purgeMoved(yes)
			}
			return a.purge(args[0], yes)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "do not ask")
	cmd.Flags().BoolVar(&moved, "moved", false, "remove the forward files that moves to other stores left in the store")
	return cmd
}
```

(c) `cmd/hedgehug/mv.go` の確認の文言（165 行）。

```go
	fmt.Fprintf(&b, "The content before the move stays in the git history of %[1]s, and anyone who can read %[1]s can see it\n", v.From)
```

golden を直す。

```bash
grep -rl 'purge --rewrite-history' cmd/hedgehug/testdata | xargs sed -i 's/; use hedgehug purge --rewrite-history to remove it//'
grep -rn 'rewrite-history' cmd internal --include='*' -I | grep -v '^internal/sync\|^internal/ops'
```

(d) `cmd/hedgehug/doctor.go`: import に `"strings"` を足し、`writeDoctor` の `if fix {` を次にし、`writeRebuilt` を足す。

```go
	if fix {
		if r.Rebuilt != nil {
			if err := a.writeRebuilt(r.Store, r.Rebuilt); err != nil {
				return err
			}
		}
		if len(r.Repaired) == 0 && r.Rebuilt == nil {
			if _, err := fmt.Fprintln(a.stdout, "Nothing to repair"); err != nil {
				return err
			}
		}
		for _, x := range r.Repaired {
			if _, err := fmt.Fprintln(a.stdout, x.Line); err != nil {
				return err
			}
		}
	}
```

```go
// writeRebuilt tells of a store that was cloned again from origin after a
// force push, on lines of their own: what was put back, then, when some
// changes could not be, where the old store is.
func (a *app) writeRebuilt(store string, b *ops.Rebuilt) error {
	if _, err := fmt.Fprintf(a.stdout, "Cloned %s again from origin; put back %d %s\n", store, b.Replayed, plural(b.Replayed, "unpushed commit", "unpushed commits")); err != nil {
		return err
	}
	if len(b.Dropped) == 0 {
		return nil
	}
	_, err := fmt.Fprintf(a.stdout, "Could not put back the changes to %s; the old store is kept at %s\n", strings.Join(b.Dropped, ", "), b.Kept)
	return err
}
```

`--fix` の説明（`cmd.Flags().BoolVar(&fix, "fix", …)`）に、作り直しを足す。

```go
	cmd.Flags().BoolVar(&fix, "fix", false, "repair parent cycles, children of deleted tasks, tasks in another workspace than their parent and ids that clash, in one commit; clone a store again from origin after a force push; then tell what is left")
```

`Short` の `and with --fix repair what a sync repairs` はそのまま。

- [ ] **Step 12: GUI の文言を直す**

`web/core/messages.ts`:

```ts
  "sync.force_push": () =>
    "リモートの履歴が手元と食い違っています。確かめてから、hh doctor --fix で clone し直してください",
```

`web/core/messages.test.ts`:

```ts
  it("words sync.force_push", () => {
    expect(errorMessage(err("sync.force_push"))).toBe(
      "リモートの履歴が手元と食い違っています。確かめてから、hh doctor --fix で clone し直してください",
    );
  });
```

- [ ] **Step 13: 成功を確かめる**

```bash
go build ./... && go vet ./...
grep -rn -E 'RewriteHistory|PrepareRewrite|RewritesPath|DecodeRewrites|EncodeRewrites|model\.Rewrite|ReadApplied|WriteApplied|rewrites-applied|rewrites\.yaml|rewrite-in-progress|rewrite_leftover|staleMirror|mirrorStale|BackgroundThen|ResetMain|HasSameCommit|\.ForcePush\(|purge --rewrite-history' --include='*.go' --include='*.ts' --include='*.golden' . | grep -v node_modules
```

後の `grep` は何も出さないこと（`docs/` の spec と過去の plan は Task 13 の範囲）。

Run: `go test ./internal/gitrepo ./internal/sync ./internal/doctor ./internal/model ./internal/mount -count=1`（timeout 600000）
Expected: PASS

Run: `go test ./internal/ops ./internal/server -count=1`（timeout 600000）
Expected: PASS

Run: `go test ./cmd/hedgehug -count=1`（timeout 600000）
Expected: PASS（`TestMvStore*` の golden は Step 11(c) で直し済み。ほかの golden が差分を出したら、差が `purge --rewrite-history` の文言だけであることを見てから `go test ./cmd/hedgehug -run <名前> -update` で直す）

Run（`web/` で）: `pnpm test && pnpm lint`
Expected: PASS

- [ ] **Step 14: コミット**

```bash
git add -A internal cmd web/core
git commit -m "$(cat <<'EOF'
refactor(sync): purge --rewrite-history と書き換えの記録を消し、force push を doctor --fix で直す

履歴を書き換える機能と、その記録（rewrites.yaml・rewrites-applied.json）に頼る
自動の追従（同期の中の作り直し、ミラーの扱い、rewrite_leftover の検査）を消す。
同期は origin の履歴が手元と食い違っていれば止まり、hh doctor --fix が記録なしで
clone し直して未 push の変更を戻す。新しい履歴に無いタスクの変更は戻せないので、
古い Store を残してパスを知らせる。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

#### spec にない細部

1. **同期の中の入れ替えを全部消す。** spec は「同期の中では自動で直さない」と言う。それに合わせ、同期が作り直した clone を入れ替えるための仕組み（`ops.swapRebuilt`・`runSyncLocked` の分け方・`Result.Rebuild`/`Swapped`/`Reopen`・バックグラウンドの `BackgroundThen`・`BoundedLog` の `Release`/`Reopen`/`ReopenAt`/`Close`・stderr の付け替え・`reopenLog`）を消した。入れ替えは `doctor --fix` の `rebuildStore` だけが行う。
2. **ミラーは普通に merge と push をする。** 書き換えの記録を読んで古い履歴のミラーを force push する処理（`staleMirror`・`mirrorHoldsPurged`・`gitrepo.Repo.ForcePush`）を消した。手作業の `git filter-repo` のあとは、利用者が全 remote に force push する。
3. **`rewrite_leftover` の検査は消し、止まった入れ替えの跡は次に Store を開くとき（`OpenStore` の警告・`sync`・`doctor --fix` の `finishRebuild`）が仕上げる。** spec は検査を消すとだけ言う。仕上げに失敗したときの指摘は、同じ名前を使わず `git` の検査（`cannot finish the rebuild after a history rewrite: …`）にした。`doctor`（`--fix` なし）は、入れ替えの途中で Store がない状態を「Store が開かない」の指摘として出す。
4. **「戻せなかった変更」の決め方。** 未 push のコミットが変えたタスク（ファイル・コメント・作業時間）のうち、手元が追っていた origin（`tracked`）にはあって、新しい origin の先頭にはないものを、戻せなかったものとする（`Rebuild.Purged`）。そのタスクは載せない。手元のコミットだけが持つ新しいタスクは戻す。記録がないので、「新しい履歴が捨てたタスク」を木の差で決めた。
5. **退避先は古い Store のディレクトリ。** 戻せなかった変更があるとき、`Swap` は古い Store を消さず `<path>.kept-<UTC 時刻>` に置き、`Rebuild.Kept` とレポートの `rebuilt.kept` と出力の行で知らせる。戻せなかった変更がなければ、今までどおり消す。`.kept-` は `FindLeftover` が見ない名前にして、`FinishLeftover` が消さないようにした。
6. **`doctor --fix` の順序と失敗。** 止まった入れ替えの仕上げ → force push の作り直し → 1 コミットの修復、の順にする（修復は新しい履歴の上で行う）。作り直しの失敗は、その Store の `unexpected_force_push` の指摘（`cannot clone the store again: …`）に足し、ほかの Store は続ける。入れ替えの直前に古い HEAD が変わっていたときは、エラーにせず警告（`… run hedgehug doctor --fix again`）を出し、`unexpected_force_push` の指摘は残る（終了コード 1）。
7. **origin が手元の子孫に戻っていたとき。** `MakeRebuild` は何も作らず `force-push.json` を消して `nil` を返す。`doctor --fix` の出力に作り直しの行は出ない。
8. **作り直しは push しない。** 載せた `replay` コミットは、次の同期（`hh sync` かバックグラウンド）が push する。今までの「入れ替えのあとすぐ同期」はやめた。
9. **読み取り専用の Store も作り直す。** 作り直しは手元だけの変更で、push を伴わないので `checkWritable` は通さない。
10. **文言。** `ErrForcePush` は `the history of origin differs from this store's; check origin, then run hedgehug doctor --fix to clone it again`（spec の文の英訳）。`ErrDirtyRebuild` は `… then run hedgehug doctor --fix`。`doctor --fix` の出力は `Cloned <store> again from origin; put back N unpushed commit(s)` と、戻せなかった変更があれば `Could not put back the changes to <id>, …; the old store is kept at <path>`。GUI の `sync.force_push` は `リモートの履歴が手元と食い違っています。確かめてから、hh doctor --fix で clone し直してください`。
11. **inbox の `rewrite_applied`・`purged_change` は残す。** 作り直しが新しい Store の inbox に書く知らせで、`doctor --fix` の出力と二重になるが、あとから `hh inbox` で見られるのでそのまま。
12. **`rewrites-applied.json`・`rewrite-in-progress.json` は読まず、作り直しでもコピーしない。** 以前の版が書いたファイルが Store に残っていても無視する（リリース前）。
13. **テストの道具は `gitrepotest.RewriteRemote`。** `RewriteHistory` を消すと、origin の書き換えを作る手段がなくなるので、bare の `main` を親のない 1 つのコミット（作者・コミッター・メッセージは古い先頭のもの、木は指定のパスを除いたもの）に置き換える道具を足した。単一のコミットなので、同期の `FindSameCommit`（inbox の印を探す）は古い先頭の写しを見つける。
14. **`gitrepo.Repo.push` から `force`・`lease` を消す。** `ForcePush` を消すと使う人がいなくなるため。
15. **`mount`・`inbox` のコメントの「history rewrite」は変えない。** 止まった入れ替えと作り直しの Store の話として、今も合っている。

### Task 11: `purge --older-than` と転送ファイルの `moved_at`

spec §9・§9.1。`purge` を `hh purge [--older-than <N>d] [--store <name>] [--yes]` の1つの形に作り直し、転送ファイルに `moved_at` を持たせる。Task 9 で API の purge が、Task 10 で `--rewrite-history` とその記録が消えている前提で、残っている `purge <id>`・`purge --moved` の経路を消して置き換える。

Task の中の順序: ① 小さな共有パッケージ `internal/plural`（この Task の出力が複数形を使うので先に作る。使い方の全体は Task 12）→ ② 転送ファイルの形（model・store・index・merge）と、手書きの転送ファイルの fixture → ③ ops の purge → ④ CLI の purge と、古い purge のテストの置き換え。コミットは最後に1つ（どの途中のステップでも全体は通らない。②の model を変えた時点で、手書き fixture を直すまで赤になる）。

**Files:**
- Create: `internal/plural/plural.go`、`internal/plural/plural_test.go`
- Modify: `internal/model/forward.go`（`MovedAt`）、`internal/model/forward_test.go`
- Modify: `internal/store/move.go`（`MoveOut` が `moved_at` を書く、411 行付近）、`internal/store/move_test.go`（`TestMoveOut` に1項）、`internal/store/forward_test.go`（fixture 2か所と op 名）
- Modify: `internal/index/forward.go`（`ForwardEntry.MovedAt`、`newForwardEntry`・`decodeForwardEntry`）、`internal/index/build.go`（`cacheFormat` 7 → 8、173 行）、`internal/index/forward_test.go`・`internal/index/forward_broken_test.go`（fixture）
- Modify: `internal/merge/forward.go`（同じ移動先の転送ファイルは `moved_at` が大きい方）、`internal/merge/forward_test.go`（`forwardFile` に `moved_at`、新しいテスト）、`internal/merge/forward_same_test.go`・`internal/merge/repair_test.go`（fixture）
- Modify: fixture の手書き転送ファイルに `moved_at` を足す: `internal/doctor/forward_test.go:16-18`、`internal/mount/moved_test.go:29`、`internal/ops/moved_test.go:38`、`internal/ops/purgemoved_broken_test.go:20,84`（このファイルはこの Task で `purge_broken_test.go` に改名する）、`cmd/hedgehug/show_test.go:251`
- Create: `internal/ops/purge.go`（`ParseOlderThan`・`PurgePlan`・`Session.PurgeList`・`Purge`・`PurgeConfirmed`）、`internal/ops/purge_test.go`
- Delete: `internal/ops/purgemoved.go`、`internal/ops/purgemoved_test.go`
- Rename: `internal/ops/purgemoved_broken_test.go` → `internal/ops/purge_broken_test.go`（中身を書き換える）
- Modify: `internal/ops/delete.go`（`PurgeTargets`・`Purge`・`PurgeUnchecked`・`purge`・`purgeTargets`・`sameTasks` を消す。`isDeleted` の `hint` 引数を消す）、`internal/ops/delete_test.go`（古い purge のテストを置き換える）、`internal/ops/readonly_test.go:97-99`
- Modify: `internal/ops/undo.go`（`undoBlocked` に `purge`、393 行の文言）、`internal/ops/workspace.go:255`（文言）、`internal/ops/duplicates.go:65`（案内の文言）
- Modify: `internal/ops/undo_test.go:164`、`internal/ops/workspace_test.go:52`、`internal/ops/duplicates_test.go:77`、`internal/ops/moved_test.go`（`TestUndoRefusesMovesAndRewrites` に `purge` の行）
- Modify: `cmd/hedgehug/rm.go`（`purgeCmd`・`purge`・`confirm`・`forwardLine`・`forwardFiles`・`purgeMoved`・`printPurgedMoved` を消し、`printAffected` の `refresh` 引数を消す）
- Create: `cmd/hedgehug/purge.go`、`cmd/hedgehug/purge_test.go`
- Modify: `cmd/hedgehug/purge_moved_test.go`（`TestPurgeMoved*` の6つを消す。ヘルパー `storeTaskFile`・`newForwardEnv` と、後ろの doctor・重複のテストは残す。重複のテスト2つの文言を直す）
- Modify: `cmd/hedgehug/rm_test.go`（`TestPurge` を置き換える）、`cmd/hedgehug/meanwhile_test.go`（purge の3つ）、`cmd/hedgehug/scenario_test.go:91`、`cmd/hedgehug/readonly_test.go:37`、`cmd/hedgehug/e2e_3c_test.go:234-248`
- Delete（golden）: `cmd/hedgehug/testdata/TestPurgeMovedAndAnIDDoNotMix`・`TestPurgeMovedListAndConfirmOnATerminal`・`TestPurgeMovedListsAStoreNotMounted`・`TestPurgeMovedNeedsAStoreWhenThereAreSeveral`・`TestPurgeMovedYesAndJSON`・`TestPurgeMovedYesText` のディレクトリ
- Modify（golden）: `cmd/hedgehug/testdata/TestUndoWorkspaceWithTasksIsRefused/undo.err.golden:1`、`cmd/hedgehug/testdata/TestDuplicateAcrossStoresIsAWarning/doctor.out.golden:2,6`・`doctor-json.out.golden:9,21`

**Interfaces:**
- Consumes: Task 9 で API の purge が消えたこと（`internal/server` が `Session.PurgeTargets`・`Purge`・`PurgeUnchecked` を呼ばない）、Task 10 で `internal/ops/rewrite.go`・`cmd/hedgehug/purge_rewrite.go`・`.hedgehug/rewrites.yaml` の記録・`undo` の `rewrite-history` の行が消えたこと。既存の `Session.Entry()`（`--store` の解決。複数 Store で省略すると `store.choose`）、`Session.OpenForWrite`（読み取り専用の拒否）、`Session.UpdateCommit`、`Tx.RemoveTask`（コメント・作業時間のファイルと `.hedgehug/numbers.yaml` の記録も扱う）、`index.Index.Forwards()`、`ops.Descendants`・`every`、`confirmRequired`、`commitHex`
- Produces:
  - `plural.Count(n int, noun string) string`（`"1 task"`・`"2 tasks"`・`"0 tasks"`）と `plural.Pick(n int, one, many string) string`（`Pick(1, "has", "have") == "has"`）。Task 12 が全体に使う
  - `model.Forward.MovedAt time.Time`（必須。ファイルの項目は `moved_at`、RFC 3339）
  - `index.ForwardEntry.MovedAt time.Time`
  - `ops.DefaultPurgeDays = 90`、`ops.ParseOlderThan(s string) (days int, err error)`（`KindUsage`、`purge.bad_period`）
  - `ops.PurgePlan{Store string; Tasks []PurgeTask; Forwards []PurgeForward}` と `(PurgePlan).Empty() bool`、`ops.PurgeResult{PurgePlan; Commit string}`。JSON は `{store, tasks:[{ulid,id,title}], forwards:[{ulid,moved_to,moved_at}], commit}`（`tasks`・`forwards` は空でも `[]`、`commit` はコミットがなければ省く）
  - `Session.PurgeList(days int) (PurgePlan, error)`、`Session.Purge(days int, yes bool) (PurgeResult, error)`（`yes` なしで対象があれば `confirm.required`）、`Session.PurgeConfirmed(days int, confirmed PurgePlan) (PurgeResult, error)`（対象が変わっていたら `purge.changed`）
  - コミットの op は `purge` だけ（`purge-moved` は消える）。`Detail` は `older than 90d`
  - 文言: `Purged N tasks and M forward files from <store>`・`Nothing to purge`・拒否の文言（spec §9）

#### A. 共有の `internal/plural`

- [ ] **Step 1: テストを書く**

`internal/plural/plural_test.go` を作る。

```go
package plural

import "testing"

func TestCount(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		n    int
		noun string
		want string
	}{
		{0, "task", "0 tasks"},
		{1, "task", "1 task"},
		{2, "task", "2 tasks"},
		{1, "forward file", "1 forward file"},
		{3, "forward file", "3 forward files"},
	} {
		if got := Count(c.n, c.noun); got != c.want {
			t.Errorf("Count(%d, %q) = %q; want %q", c.n, c.noun, got, c.want)
		}
	}
}

func TestPick(t *testing.T) {
	t.Parallel()
	if got := Pick(1, "has", "have"); got != "has" {
		t.Errorf("Pick(1) = %q", got)
	}
	for _, n := range []int{0, 2, 10} {
		if got := Pick(n, "has", "have"); got != "have" {
			t.Errorf("Pick(%d) = %q", n, got)
		}
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/plural -count=1`（timeout 600000）
Expected: FAIL（`undefined: Count`）

- [ ] **Step 3: 実装する**

`internal/plural/plural.go` を作る。

```go
// Package plural writes a count with its noun in English: "1 task", "2 tasks".
// Every message that has a number in it goes through here.
package plural

import "strconv"

// Count is n and noun, with an s after the noun unless n is 1.
func Count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// Pick is one when n is 1 and many otherwise, for the word that agrees with a
// count: Pick(n, "has", "have").
func Pick(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
```

- [ ] **Step 4: 成功を確かめる**

Run: `go test ./internal/plural -count=1`（timeout 600000）
Expected: PASS

#### B. 転送ファイルの `moved_at`（model・store・index・merge）

- [ ] **Step 5: model のテストを `moved_at` ありに直す**

`internal/model/forward_test.go` の定数の下に1つ足す。

```go
const fwdAt = "2026-10-31T10:00:00Z"
```

`TestDecodeForward` を次にする。

```go
func TestDecodeForward(t *testing.T) {
	t.Parallel()
	f, err := DecodeForward([]byte("---\nulid: " + fwdULID + "\nmoved_to: " + fwdStore + "\nmoved_at: " + fwdAt + "\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if f.ULID.String() != fwdULID || f.MovedTo.String() != fwdStore || !f.MovedAt.Equal(time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("got %+v", f)
	}
}
```

（`import` に `"time"` を足す。）`TestDecodeForwardErrors` の表を次にする。`no moved_at`・`bad moved_at` が新しい行、ほかは `moved_at` を足した形（足りない項目だけが原因になるように）。

```go
	for name, src := range map[string]string{
		"no moved_to":   "---\nulid: " + fwdULID + "\nmoved_at: " + fwdAt + "\n---\n",
		"null moved_to": "---\nulid: " + fwdULID + "\nmoved_to:\nmoved_at: " + fwdAt + "\n---\n",
		"no ulid":       "---\nmoved_to: " + fwdStore + "\nmoved_at: " + fwdAt + "\n---\n",
		"no moved_at":   "---\nulid: " + fwdULID + "\nmoved_to: " + fwdStore + "\n---\n",
		"bad moved_at":  "---\nulid: " + fwdULID + "\nmoved_to: " + fwdStore + "\nmoved_at: nope\n---\n",
		"other key":     "---\nulid: " + fwdULID + "\nmoved_to: " + fwdStore + "\nmoved_at: " + fwdAt + "\ntitle: x\n---\n",
		"bad moved_to":  "---\nulid: " + fwdULID + "\nmoved_to: nope\nmoved_at: " + fwdAt + "\n---\n",
		"no front":      "moved_to: x",
	} {
```

続く `title` の検査（`_, err := DecodeForward(...)` で `err` が `title` を名指す）の入力にも `\nmoved_at: " + fwdAt + "` を `moved_to` の後ろに足す。`no moved_at` が `moved_at` を名指すことも確かめる行を、表のループの後ろに足す。

```go
	_, err = DecodeForward([]byte("---\nulid: " + fwdULID + "\nmoved_to: " + fwdStore + "\n---\n"))
	if err == nil || !strings.Contains(err.Error(), "moved_at") {
		t.Errorf("err = %v, want it to name moved_at", err)
	}
```

（直前の `_, err := DecodeForward(` が `:=` なのでその次の行は `=` にする。）`TestForwardEncodeRoundTrip` の `src` と最後の組み立てを次にする。

```go
	src := "---\nulid: " + fwdULID + "\nmoved_to: " + fwdStore + "\nmoved_at: " + fwdAt + "\n---\n"
```
```go
	ulid, _ := ParseULID(fwdULID)
	store, _ := ParseULID(fwdStore)
	at := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	out, err = (&Forward{ULID: ulid, MovedTo: store, MovedAt: at}).Encode()
```

`TestIsForward` の最初の行は `moved_at` を足さなくてもよい（`IsForward` は `moved_to` の有無だけを見る）。

- [ ] **Step 6: 失敗を確かめる**

Run: `go test ./internal/model -run 'Forward' -count=1`（timeout 600000）
Expected: FAIL（`f.MovedAt undefined` などのコンパイルエラー）

- [ ] **Step 7: model を実装する**

`internal/model/forward.go` を次にする（変えるのは、型・`forwardKeys`・`DecodeForward`・`Encode`・コメントだけ）。

```go
package model

import (
	"errors"
	"fmt"
	"time"
)

// ErrForward means a task file is a forward: the task moved to another
// store and only its ulid, moved_to and moved_at are left.
var ErrForward = errors.New("the task moved to another store")

// Forward is a task file left behind by a move to another store.
type Forward struct {
	ULID    ULID
	MovedTo ULID      // the ulid of the store it moved to (store.yaml's ulid)
	MovedAt time.Time // when the task moved; purge --older-than goes by it

	doc *Doc
}

var forwardKeys = []string{"ulid", "moved_to", "moved_at"}
```

`DecodeForward` の `decodeAll` の行と、その後ろの検査を次にする。

```go
	f := &Forward{doc: d}
	if err := decodeAll(d, map[string]any{"ulid": &f.ULID, "moved_to": &f.MovedTo, "moved_at": &f.MovedAt}); err != nil {
		return nil, err
	}
	if f.ULID.IsZero() {
		return nil, &DecodeError{Field: "ulid", Err: errors.New("missing")}
	}
	if f.MovedTo.IsZero() {
		return nil, &DecodeError{Field: "moved_to", Err: errors.New("missing")}
	}
	if f.MovedAt.IsZero() {
		return nil, &DecodeError{Field: "moved_at", Err: errors.New("missing")}
	}
	if extra := d.Extra(forwardKeys); len(extra) > 0 {
		return nil, &DecodeError{Field: extra[0].Key, Err: fmt.Errorf("a forward holds only ulid, moved_to and moved_at")}
	}
	return f, nil
```

`DecodeForward` の上のコメントは「ulid, moved_to and moved_at are required」にし、`Encode` は `e.Put("moved_to", f.MovedTo, false)` の次に1行足す。コメントは「front matter with ulid, moved_to and moved_at, no body」。

```go
	e.Put("moved_at", f.MovedAt, false)
```

- [ ] **Step 8: model のテストが通ることを確かめる**

Run: `go test ./internal/model -count=1`（timeout 600000）
Expected: PASS

- [ ] **Step 9: 手書きの転送ファイルの fixture に `moved_at` を足す**

どれも、`moved_to` の行の後ろ（`title: x` の前）に `moved_at: 2026-10-31T10:00:00Z` の行を足す。足りない項目だけが原因で壊れた扱いになる fixture を作らないための直し。

| ファイル:行 | 今 | 直した後 |
| --- | --- | --- |
| `internal/index/forward_test.go:21` | `"...moved_to: " + dest + "\n---\n"` | `"...moved_to: " + dest + "\nmoved_at: 2026-10-31T10:00:00Z\n---\n"` |
| `internal/index/forward_test.go:147` | `...moved_to: "+destStore+"\ntitle: x\n---\n"` | `...moved_to: "+destStore+"\nmoved_at: 2026-10-31T10:00:00Z\ntitle: x\n---\n"` |
| `internal/index/forward_broken_test.go:17` | 同上 | 同上 |
| `internal/store/forward_test.go:16` | `"---\nulid: " + ulid + "\nmoved_to: " + forwardStoreULID + "\n---\n"` | `...\nmoved_to: " + forwardStoreULID + "\nmoved_at: 2026-10-31T10:00:00Z\n---\n"` |
| `internal/store/forward_test.go:27` | `...moved_to: "+forwardStoreULID+"\ntitle: x\n---\n"` | `...moved_to: "+forwardStoreULID+"\nmoved_at: 2026-10-31T10:00:00Z\ntitle: x\n---\n"` |
| `internal/merge/forward_test.go:20` | `forwardFile` の1行 | 下の `forwardFileAt` に置き換える |
| `internal/merge/forward_same_test.go:14` | `"---\nmoved_to:   " + storeB + "\nulid: " + taskU + "\n---\n\n"` | `"---\nmoved_to:   " + storeB + "\nulid: " + taskU + "\nmoved_at: 2026-10-31T10:00:00Z\n---\n\n"` |
| `internal/merge/repair_test.go:430` | `...\nmoved_to: 01JC0M3R5W7M2X9V4T6R5N8P1B\n---\n` | `...\nmoved_to: 01JC0M3R5W7M2X9V4T6R5N8P1B\nmoved_at: 2026-10-31T10:00:00Z\n---\n` |
| `internal/doctor/forward_test.go:16,17,18` | `...moved_to: "+dest+"\n---\n"`（17 行は `\ntitle: x` つき） | それぞれ `moved_to: "+dest+"\nmoved_at: 2026-10-31T10:00:00Z` を足す（17 行は `title: x` の前） |
| `internal/mount/moved_test.go:29` | `fwd := "---\nulid: " + ... + "\nmoved_to: " + bs.Meta().ULID.String() + "\n---\n"` | `... + "\nmoved_at: 2026-10-31T10:00:00Z\n---\n"` |
| `internal/ops/moved_test.go:38` | 同上の形 | 同上 |
| `internal/ops/purgemoved_broken_test.go:20` | `...moved_to: " + model.NewULID().String() + "\ntitle: keep me\n---\n"` | `... + "\nmoved_at: 2026-10-31T10:00:00Z\ntitle: keep me\n---\n"` |
| `internal/ops/purgemoved_broken_test.go:84` | `fwd := ...moved_to: " + model.NewULID().String() + "\n---\n"` | `... + "\nmoved_at: 2026-10-31T10:00:00Z\n---\n"` |
| `cmd/hedgehug/show_test.go:251` | `fwd := "---\nulid: " + moved.ULID + "\nmoved_to: " + sb.Meta().ULID.String() + "\n---\n"` | `... + "\nmoved_at: 2026-10-31T10:00:00Z\n---\n"` |

`internal/merge/forward_test.go` の `forwardFile` は、`moved_at` を渡せる `forwardFileAt` に直し、`forwardFile` はそれに固定の時刻を渡す形にする。

```go
const movedAt = "2026-10-31T10:00:00Z"

func forwardFile(ulid, to string) []byte { return forwardFileAt(ulid, to, movedAt) }

func forwardFileAt(ulid, to, at string) []byte {
	return []byte("---\nulid: " + ulid + "\nmoved_to: " + to + "\nmoved_at: " + at + "\n---\n")
}
```

`model.IsForward` だけを見る箇所（`internal/model/task_test.go:146`、`internal/ops/edit_test.go:104` の `moved_to` を手で書く拒否）は直さない。`internal/inbox/collect_test.go`・`internal/merge/fuzz_test.go`・`internal/sync/forward_test.go`・`internal/store/move_test.go` は `forwardFile` や `MoveOut` を通して作るので、直す行はない。

- [ ] **Step 10: 転送ファイルの時刻のテストを足す（store・index・merge）**

`internal/store/move_test.go` の `TestMoveOut` の中、転送ファイルを読む検査（`fw, err := model.DecodeForward(b)` の `if` の直後）に足す。`newMoveStore` の時計は `testNow`（`internal/store/helpers_test.go:17`）。

```go
		if !fw.MovedAt.Equal(testNow.Truncate(time.Second)) {
			t.Errorf("moved_at of %s = %v; want the time of the move, %v", x.Title, fw.MovedAt, testNow)
		}
```

（`move_test.go` の `import` に `"time"` がなければ足す。）

`internal/index/forward_test.go` の、転送ファイルを `commitRaw` で書いて `ix.Forward(...)` を見るテストのどれか（`TestForwardAfterForwardKeepsTheID` の後ろ）に、新しいテストを足す。

```go
func TestForwardEntryHasMovedAt(t *testing.T) {
	t.Parallel()
	s, dir := newStore(t)
	moved := putWithID(t, s, newTask("moved"))
	commitRaw(t, s, dir, model.TaskPath(moved.ULID), forwardOf(moved.ULID, destStore))
	ix, err := index.Build(s)
	if err != nil {
		t.Fatal(err)
	}
	f, ok := ix.Forward(moved.ULID)
	if !ok || !f.MovedAt.Equal(time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("forward = %+v, %v; want moved_at 2026-10-31T10:00:00Z", f, ok)
	}
}
```

（このファイルで索引を作る関数の名前と `import` は、同じファイルの隣のテスト（`TestForwardAfterForwardKeepsTheID` の続き）が使っているものに合わせる。`index.Build(s)` でなければ、隣のテストの呼び方に直す。`"time"` を `import` に足す。）

`internal/merge/forward_test.go` に足す。

```go
// Two forwards of the same store: the later moved_at is kept, whichever
// side it is on and whatever the commit times say, and nothing is recorded as
// lost (the destination is the same).
func TestMergeTwoForwardsOfTheSameStoreKeepTheLaterMovedAt(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", "title: old")
	early := forwardFileAt(taskU, storeB, "2026-10-31T10:00:00Z")
	late := forwardFileAt(taskU, storeB, "2026-10-31T11:00:00Z")
	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	// the side with the earlier moved_at was written later by the clock of the commit
	withEarly := Side{Data: early, Author: "a", Time: at.Add(time.Minute)}
	withLate := Side{Data: late, Author: "b", Time: at}
	for name, sides := range map[string][2]Side{"ours": {withLate, withEarly}, "theirs": {withEarly, withLate}} {
		r := mergeTask(t, side(base, ""), sides[0], sides[1])
		if !bytes.Equal(r.Data, late) || len(r.Overwritten) != 0 {
			t.Errorf("%s has the later moved_at: got %s, overwritten %+v", name, r.Data, r.Overwritten)
		}
	}
}
```

- [ ] **Step 11: 失敗を確かめる**

Run: `go test ./internal/store ./internal/index ./internal/merge -count=1`（timeout 600000）
Expected: FAIL（`MoveOut` が `moved_at` を書かない: `Encode` が `moved_at` の欠けで読めないファイルを作り、`TestMoveOut` ほか多数が落ちる。`ForwardEntry.MovedAt` が未定義でコンパイルエラーにもなる）

- [ ] **Step 12: store・index・merge を実装する**

`internal/store/move.go` の転送ファイルを書く所（411 行付近）を次にする。

```go
		b, err := (&model.Forward{ULID: ulid, MovedTo: out.To, MovedAt: tx.s.now().UTC().Truncate(time.Second)}).Encode()
```

（`import` に `"time"` がなければ足す。）`MoveOut` の上のコメント「each task file becomes a forward to out.To」に「(with the time of the move as moved_at)」を足す。

`internal/index/forward.go` の `ForwardEntry` に項目を足す。

```go
type ForwardEntry struct {
	ULID      model.ULID
	MovedTo   model.ULID // the ulid of the store it moved to
	MovedAt   time.Time  // when it moved
	ID        string     // the id the task had when it was last a task here; "" when not found
	FormerIDs []string
	Broken    string // why the forward did not decode; empty when it did
}
```

`newForwardEntry` の `e := &ForwardEntry{ULID: ulid, MovedTo: f.MovedTo}` と、`decodeForwardEntry` の `return &ForwardEntry{ULID: ulid, MovedTo: f.MovedTo}` に `MovedAt: f.MovedAt` を足す。`import` に `"time"` を足す。

`internal/index/build.go:173` を `cacheFormat = 8       // bump when Entry or cache changes` にする（キャッシュは gob で `ForwardEntry` を持つので、形が変わったら捨てさせる）。

`internal/merge/forward.go` を直す。`forward` の `case of && tf:` を次にし、関数を1つ足す。

```go
	case of && tf:
		if !later(ours.Time, theirs.Time, ours.Author, theirs.Author, ours.Data, theirs.Data) {
			win, lose = theirs, ours
		}
		// the same destination: the later move is the one to keep
		if w, l, ok := laterMovedAt(ours, theirs); ok {
			win, lose = w, l
		}
```

```go
// laterMovedAt picks, of two forwards that name the same store, the one with
// the later moved_at. ok is false when either does not decode, they name
// different stores, or the times are the same; the caller then goes by the
// time of the commits.
func laterMovedAt(ours, theirs Side) (win, lose Side, ok bool) {
	o, oerr := model.DecodeForward(ours.Data)
	t, terr := model.DecodeForward(theirs.Data)
	if oerr != nil || terr != nil || o.MovedTo != t.MovedTo || o.MovedAt.Equal(t.MovedAt) {
		return Side{}, Side{}, false
	}
	if t.MovedAt.After(o.MovedAt) {
		return theirs, ours, true
	}
	return ours, theirs, true
}
```

`forward` の上のコメントに1文足す: 「Two forwards that name the same store are merged by moved_at: the later one is kept.」。`m.p == OursWins` の `case` が先に評価されるので、`OursWins` は今のまま ours を取る。

- [ ] **Step 13: 成功を確かめる**

Run: `go test ./internal/model ./internal/store ./internal/index ./internal/merge ./internal/doctor ./internal/mount ./internal/sync ./internal/inbox -count=1`（timeout 600000）
Expected: PASS。落ちるテストが手書きの転送ファイルの読み込みなら、Step 9 の表で拾えなかった fixture（`grep -rn 'moved_to' --include='*_test.go' internal cmd` で、`moved_at` のないものを探す）。

#### C. ops の purge

- [ ] **Step 14: ops の古い purge を洗い出す**

Run: `grep -rn 'PurgeTargets\|PurgeUnchecked\|\.Purge(\|PurgeMoved\|ForwardView\|forwardViews\|sameTasks\|purgeTargets\|warnBrokenForwards\|purge.child_not_deleted\|purge-moved' --include='*.go' internal cmd`
Expected: 使う側は `cmd/hedgehug/rm.go`、`internal/ops/{delete,purgemoved}.go` とそれらのテストだけ（Task 9 で `internal/server` から、Task 10 で `internal/ops/rewrite*.go` から消えている）。これ以外に残っていたら、その場所もこの Task で新しい API に直す。

- [ ] **Step 15: ops のテストを書く**

`internal/ops/purge_test.go` を作る。固定の時計（`2026-10-30T12:00:00Z`）のままで、日数と `deleted_at`・`moved_at` を変えて試す（時計を進めない）。

```go
package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
)

// addDeleted writes a task that was deleted at.
func (ts *testSession) addDeleted(title string, at time.Time, edit func(*model.Task)) *model.Task {
	ts.t.Helper()
	return ts.addTask(title, func(t *model.Task) {
		t.DeletedAt = &at
		if edit != nil {
			edit(t)
		}
	})
}

func daysAgo(ts *testSession, n int) time.Time { return ts.Now().AddDate(0, 0, -n) }

func purgedULIDs(p PurgePlan) []model.ULID {
	var out []model.ULID
	for _, t := range p.Tasks {
		out = append(out, t.ULID)
	}
	return out
}

func lastOp(t *testing.T, ts *testSession) string {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	head, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	log, err := s.Repo().LogFrom(head, 1)
	if err != nil || len(log) == 0 {
		t.Fatalf("log = %v, %v", log, err)
	}
	op, _ := log[0].Trailer("Hedgehug-Op")
	return op
}

func TestParseOlderThan(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]int{"90d": 90, "0d": 0, "1d": 1, "007d": 7, "36500d": 36500} {
		if got, err := ParseOlderThan(in); err != nil || got != want {
			t.Errorf("ParseOlderThan(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "d", "90", "-5d", "+5d", "1w", "9d0", "1.5d", "36501d", "99999999999999999999d", " 5d"} {
		_, err := ParseOlderThan(in)
		wantErr(t, err, KindUsage, "purge.bad_period")
	}
}

func TestPurgeRemovesWhatIsOlder(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	old := ts.addDeleted("old", daysAgo(ts, 100), nil)
	recent := ts.addDeleted("recent", daysAgo(ts, 10), nil)
	live := ts.addTask("live", nil)

	plan, err := ts.PurgeList(90)
	if err != nil {
		t.Fatal(err)
	}
	if got := purgedULIDs(plan); len(got) != 1 || got[0] != old.ULID || plan.Store != "me" || plan.Tasks[0].Title != "old" || plan.Tasks[0].ID != old.ID {
		t.Fatalf("plan = %+v; want only the old one", plan)
	}
	_, err = ts.Purge(90, false)
	wantErr(t, err, KindUsage, "confirm.required")
	if _, ok := mustIndex2(t, ts).Task(old.ULID); !ok {
		t.Fatal("a refused purge removed the task")
	}

	res, err := ts.Purge(90, true)
	if err != nil || res.Commit == "" || len(res.Tasks) != 1 {
		t.Fatalf("Purge = %+v, %v", res, err)
	}
	if op := lastOp(t, ts); op != "purge" {
		t.Errorf("Hedgehug-Op = %q; want purge", op)
	}
	ix := mustIndex2(t, ts)
	for _, c := range []struct {
		u    model.ULID
		want bool
	}{{old.ULID, false}, {recent.ULID, true}, {live.ULID, true}} {
		if _, ok := ix.Task(c.u); ok != c.want {
			t.Errorf("task %s present = %v; want %v", c.u, ok, c.want)
		}
	}
}

// The boundary: what was deleted exactly N days ago is older than N days, so
// 0d takes what was deleted just now.
func TestPurgeBoundary(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.addDeleted("just now", ts.Now(), nil)
	ts.addDeleted("a day ago", daysAgo(ts, 1), nil)
	for days, want := range map[int]int{0: 2, 1: 1, 2: 0} {
		plan, err := ts.PurgeList(days)
		if err != nil || len(plan.Tasks) != want {
			t.Errorf("PurgeList(%d) = %d tasks, %v; want %d", days, len(plan.Tasks), err, want)
		}
	}
}

func TestPurgeNothingIsNoCommit(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.addTask("live", nil)
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	before := headOf(t, s)
	for _, yes := range []bool{false, true} {
		res, err := ts.Purge(0, yes)
		if err != nil || !res.Empty() || res.Commit != "" || res.Tasks == nil || res.Forwards == nil {
			t.Errorf("yes=%v: %+v, %v; want an empty plan with empty lists and no commit", yes, res, err)
		}
	}
	if headOf(t, s) != before {
		t.Error("head moved")
	}
}

// A deleted task with a task under it that is not deleted stays, and the
// warning names it. Its deleted children go.
func TestPurgeKeepsATaskWithALiveChild(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	at := daysAgo(ts, 100)
	parent := ts.addDeleted("parent", at, nil)
	live := ts.addTask("live child", func(t *model.Task) { t.Parent = &parent.ULID })
	gone := ts.addDeleted("gone child", at, func(t *model.Task) { t.Parent = &parent.ULID })

	plan, err := ts.PurgeList(90)
	if err != nil {
		t.Fatal(err)
	}
	if got := purgedULIDs(plan); len(got) != 1 || got[0] != gone.ULID {
		t.Fatalf("plan = %+v; want only the deleted child", plan)
	}
	if len(ts.warns) != 1 || !strings.Contains(ts.warns[0], parent.ID) || !strings.Contains(ts.warns[0], "is not deleted") {
		t.Errorf("warnings = %q; want one naming %s", ts.warns, parent.ID)
	}
	if _, err := ts.Purge(90, true); err != nil {
		t.Fatal(err)
	}
	ix := mustIndex2(t, ts)
	if _, ok := ix.Task(parent.ULID); !ok {
		t.Error("the parent was purged")
	}
	if _, ok := ix.Task(live.ULID); !ok {
		t.Error("the live child was purged")
	}
}

// A task whose child was deleted later than the limit waits for the child.
func TestPurgeWaitsForARecentlyDeletedChild(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	parent := ts.addDeleted("parent", daysAgo(ts, 100), nil)
	ts.addDeleted("child", daysAgo(ts, 10), func(t *model.Task) { t.Parent = &parent.ULID })
	plan, err := ts.PurgeList(90)
	if err != nil || !plan.Empty() || len(ts.warns) != 0 {
		t.Errorf("plan = %+v, %v, warnings %q; want nothing and no warning", plan, err, ts.warns)
	}
}

func TestPurgeRecordsTheNumbers(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	for _, title := range []string{"a", "b"} {
		if _, err := ts.Add(AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"ME-1", "ME-2"} {
		if _, err := ts.Remove(id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ts.Purge(0, true); err != nil {
		t.Fatal(err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.Numbers()
	if err != nil || n["ME"] != 2 {
		t.Errorf("numbers = %v, %v; want ME: 2", n, err)
	}
}

func TestPurgeConfirmedRefusesChanges(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.addDeleted("first", daysAgo(ts, 100), nil)
	plan, err := ts.PurgeList(90)
	if err != nil {
		t.Fatal(err)
	}
	// a task that is old enough turns up after the list
	later := ts.addDeleted("later", daysAgo(ts, 100), nil)
	_, err = ts.PurgeConfirmed(90, plan)
	wantErr(t, err, KindConflict, "purge.changed")
	if _, ok := mustIndex2(t, ts).Task(later.ULID); !ok {
		t.Error("a refused purge removed a task")
	}
	// nothing confirmed is no confirmation
	_, err = ts.PurgeConfirmed(90, PurgePlan{})
	wantErr(t, err, KindUsage, "confirm.required")

	plan, err = ts.PurgeList(90)
	if err != nil {
		t.Fatal(err)
	}
	res, err := ts.PurgeConfirmed(90, plan)
	if err != nil || len(res.Tasks) != 2 || res.Commit == "" {
		t.Errorf("PurgeConfirmed = %+v, %v; want both tasks", res, err)
	}
}

func mustIndex2(t *testing.T, ts *testSession) *index.Index {
	t.Helper()
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	return ix
}
```

（`ts.Open()` は呼ぶたびに索引を読み直す。`delete_test.go` の `_, ix, _ = ts.Open()` と同じ使い方。`headOf(t, s)` は `internal/ops/moved_test.go` にある既存のヘルパー。`undo` が `purge` を断ることは、下の `moved_test.go` の表で確かめる。）

転送ファイルのテストは `moveFx`（`internal/ops/movestore_test.go`）を使う。`internal/ops/purge_broken_test.go`（`purgemoved_broken_test.go` を `git mv` で改名した後の中身）を次にする。`brokenForward` はそのまま残し（fixture は Step 9 で `moved_at` ありに直してある。`title` が余るので壊れたまま）、テストは作り直す。

```go
package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// brokenForward puts a forward file with a key it must not have in org-a.
func (f *moveFx) brokenForward(t *testing.T) model.ULID {
	t.Helper()
	u := model.NewULID()
	a := f.ts.ForStore("org-a")
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	data := "---\nulid: " + u.String() + "\nmoved_to: " + model.NewULID().String() + "\nmoved_at: 2026-10-31T10:00:00Z\ntitle: keep me\n---\n"
	if _, err := a.Update(s, store.Op{Name: "raw"}, func(tx *store.Tx) error {
		return tx.PutFile(model.TaskPath(u), []byte(data))
	}); err != nil {
		t.Fatal(err)
	}
	return u
}

// oldForward puts a forward file that moved at in org-a.
func (f *moveFx) oldForward(t *testing.T, at time.Time) model.ULID {
	t.Helper()
	u := model.NewULID()
	a := f.ts.ForStore("org-a")
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	data, err := (&model.Forward{ULID: u, MovedTo: model.NewULID(), MovedAt: at}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Update(s, store.Op{Name: "raw"}, func(tx *store.Tx) error {
		return tx.PutFile(model.TaskPath(u), data)
	}); err != nil {
		t.Fatal(err)
	}
	return u
}

func TestPurgeForwardsByTheirMovedAt(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	if _, err := f.ts.MoveStore(f.in()); err != nil { // 3 tasks move now: 3 forwards, moved_at = the clock
		t.Fatal(err)
	}
	a := f.ts.ForStore("org-a")
	old := f.oldForward(t, a.Now().AddDate(0, 0, -100))

	plan, err := a.PurgeList(90)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Tasks) != 0 || len(plan.Forwards) != 1 || plan.Forwards[0].ULID != old {
		t.Fatalf("plan = %+v; want only the old forward", plan)
	}
	plan, err = a.PurgeList(0)
	if err != nil || len(plan.Forwards) != 4 {
		t.Fatalf("plan(0d) = %d forwards, %v; want 4", len(plan.Forwards), err)
	}
	for _, p := range plan.Forwards {
		if p.ULID != old && !p.MovedAt.Equal(a.Now().UTC().Truncate(time.Second)) {
			t.Errorf("moved_at of %s = %v; want the time of the move", p.ULID, p.MovedAt)
		}
	}

	before := len(f.log(t, "org-a"))
	res, err := a.Purge(0, true)
	if err != nil || len(res.Forwards) != 4 || res.Commit == "" {
		t.Fatalf("Purge = %+v, %v", res, err)
	}
	l := f.log(t, "org-a")
	if len(l) != before+1 {
		t.Fatalf("commits = %d; want one more than %d", len(l), before)
	}
	if op, _ := l[0].Trailer("Hedgehug-Op"); op != "purge" {
		t.Errorf("Hedgehug-Op = %q; want purge", op)
	}
	if plan, err = a.PurgeList(0); err != nil || !plan.Empty() {
		t.Errorf("plan after = %+v, %v", plan, err)
	}
	// the moved tasks stay in org-b
	if _, ok := mustIndex(t, f.ts, "org-b").Task(f.p); !ok {
		t.Error("the moved task is gone from org-b")
	}
}

// A forward that does not decode may still hold a task's content: purge
// leaves it and tells of it, and the lookup of a moved task does not answer
// with it.
func TestPurgeLeavesABrokenForward(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	if _, err := f.ts.MoveStore(f.in()); err != nil {
		t.Fatal(err)
	}
	broken := f.brokenForward(t)
	var warned []string
	a := f.ts.ForStore("org-a")
	a.Warn = func(m string) { warned = append(warned, m) }

	plan, err := a.PurgeList(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Forwards) != 3 {
		t.Errorf("plan = %+v; want the 3 forwards that decode", plan)
	}
	if len(warned) != 1 || !strings.Contains(warned[0], "1 broken forward files left; see hedgehug doctor") {
		t.Errorf("warnings = %v", warned)
	}
	res, err := a.Purge(0, true)
	if err != nil || len(res.Forwards) != 3 {
		t.Fatalf("Purge = %+v, %v; want 3", res, err)
	}
	if _, ok := mustIndex(t, f.ts, "org-a").Forward(broken); !ok {
		t.Error("the broken forward file was removed")
	}
}

// A forward with no moved_at is a broken one.
func TestAForwardWithoutMovedAtIsBroken(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	u := model.NewULID()
	a := f.ts.ForStore("org-a")
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	data := "---\nulid: " + u.String() + "\nmoved_to: " + model.NewULID().String() + "\n---\n"
	if _, err := a.Update(s, store.Op{Name: "raw"}, func(tx *store.Tx) error { return tx.PutFile(model.TaskPath(u), []byte(data)) }); err != nil {
		t.Fatal(err)
	}
	plan, err := a.PurgeList(0)
	if err != nil || !plan.Empty() {
		t.Errorf("plan = %+v, %v; want nothing (the file is broken, not a forward to purge)", plan, err)
	}
	if fe, ok := mustIndex(t, f.ts, "org-a").Forward(u); !ok || fe.Broken == "" {
		t.Errorf("forward = %+v, %v; want a broken one", fe, ok)
	}
}

// A forward that is gone under the lock is no commit.
func TestPurgeGoneUnderTheLockIsNoCommit(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	if _, err := f.ts.MoveStore(f.in()); err != nil {
		t.Fatal(err)
	}
	a := f.ts.ForStore("org-a")
	plan, err := a.PurgeList(0)
	if err != nil {
		t.Fatal(err)
	}
	// someone else purges first
	if _, err := a.Purge(0, true); err != nil {
		t.Fatal(err)
	}
	res, err := a.PurgeConfirmed(0, plan)
	wantErr(t, err, KindConflict, "purge.changed")
	if res.Commit != "" {
		t.Errorf("commit = %q", res.Commit)
	}
}
```

（`moveFx.log(t, name)`・`f.in()`・`mustIndex`・`newMoveFx` は `internal/ops/movestore_test.go` と `moved_test.go` の既存のヘルパー。`f.p` は移した根のタスクの ULID。`wantErr` は `internal/ops/task_test.go`。）

古いテストを消す: `internal/ops/purgemoved_test.go` を丸ごと消し、`purgemoved_broken_test.go` は `git mv internal/ops/purgemoved_broken_test.go internal/ops/purge_broken_test.go` の後で上の内容に置き換える。

`internal/ops/delete_test.go` の古い purge のテストを置き換える。

- 消す: `TestPurgeNeedsAConfirmation`（26 行）、`TestPurgeChildNotDeleted`（194 行）、`TestPurgeWithStaleTargetsIsConflict`（214 行）、`TestSameTasks`（485 行付近）、ヘルパー `ulidsOf`（17 行。ほかに使う所がなければ。`grep -n 'ulidsOf' internal/ops/*_test.go` で確かめる）。これらの意味は `purge_test.go` の新しいテストが引き継ぐ（確認の要求・対象が変わったときの `purge.changed`・生きた子がいる親を残す）。
- `TestRemoveRestorePurge`（63 行）は、`ts.Restore("ME-1")` の検査（`r.Targets` の2件）までを残し、その後ろ（`_, _, _, err = ts.PurgeTargets("ME-1")` から関数の終わりまで）を次にする。

```go
	if _, err := ts.Remove("ME-1"); err != nil {
		t.Fatal(err)
	}
	pr, err := ts.Purge(0, true)
	if err != nil {
		t.Fatal(err)
	}
	if pr.Commit == "" || len(pr.Tasks) != 2 || pr.Tasks[0].ID != "ME-1" {
		t.Errorf("Purge = %+v; want ME-1 and ME-2 and a commit", pr)
	}
	_, ix, _ := ts.Open()
	for _, id := range []string{"ME-1", "ME-2"} {
		_, err := ts.Resolve(ix, id)
		wantErr(t, err, KindNotFound, "task.not_found")
	}
```

  （`pr.Tasks[0].ID` が `ME-1` になるのは、親が先に作られて ULID が小さいため。）
- `TestCurrentOfPurgedTaskIsGone`（`ts.PurgeUnchecked("ME-1")` の行）は `ts.Purge(0, true)` にする。
- `isDeleted` から `hint` 引数をなくす（呼ぶ所が1つになるため）。`internal/ops/delete.go` の `isDeleted` を `func isDeleted(ix *index.Index, e *index.Entry) error` にし、メッセージは `"%s is not deleted"`、呼び出し（`restorable` の中の1か所）を `isDeleted(ix, e)` にする。`internal/ops/delete_test.go` の `isDeleted` を直接呼ぶテストがあれば合わせる（`grep -n 'isDeleted(' internal/ops/*_test.go`）。

`internal/ops/readonly_test.go:97-99` の3行を次にし、`live`・`liveID`（64〜68 行）がほかで使われなくなったら消す（`go vet` が `declared and not used` を出す）。`model` の `import` が余ったら消す。

```go
		{"PurgeList", func() error { _, err := ts.PurgeList(0); return err }},
		{"Purge", func() error { _, err := ts.Purge(0, true); return err }},
		{"PurgeConfirmed", func() error {
			_, err := ts.PurgeConfirmed(0, PurgePlan{Tasks: []PurgeTask{{ULID: model.NewULID()}}})
			return err
		}},
```

その直前のコメント（`// A task that is not deleted: purge says "not deleted" once it is let through, ...`）は消す。

`internal/ops/moved_test.go` の `TestUndoRefusesMovesAndRewrites` の表（Task 10 で `rewrite-history` の行は消えている）に1行足す。

```go
		{"purge", "a purge cannot be undone"},
```

テストの名前は `TestUndoRefusesMovesAndRewrites` のままでよい（Task 10 で直していればそれに従う）。

- [ ] **Step 16: 失敗を確かめる**

Run: `go test ./internal/ops -run 'Purge|ParseOlderThan|UndoRefuses|AForwardWithoutMovedAt|RemoveRestorePurge' -count=1`（timeout 600000）
Expected: FAIL（`undefined: ParseOlderThan`・`PurgePlan`・`Session.PurgeList` などのコンパイルエラー）

- [ ] **Step 17: ops を実装する**

`internal/ops/purge.go` を作る。

```go
package ops

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/plural"
	"github.com/kukv/hedgehug/internal/store"
)

// DefaultPurgeDays is how old hedgehug purge wants what it removes to be,
// when --older-than is not given.
const DefaultPurgeDays = 90

// maxPurgeDays keeps days * 24h inside a time.Duration (about 292 years).
const maxPurgeDays = 36500

// ParseOlderThan reads a period such as 90d: digits and d. 0d is all.
func ParseOlderThan(s string) (days int, err error) {
	digits, ok := strings.CutSuffix(s, "d")
	if ok && digits != "" && strings.Trim(digits, "0123456789") == "" {
		if days, err = strconv.Atoi(digits); err == nil && days <= maxPurgeDays {
			return days, nil
		}
	}
	return 0, newErr(KindUsage, "purge.bad_period",
		fmt.Errorf("%q is not a period; write it in days, such as 90d (0d is everything)", s), "value", s)
}

// PurgeTask is a deleted task purge removes.
type PurgeTask struct {
	ULID  model.ULID `json:"ulid"`
	ID    string     `json:"id"`
	Title string     `json:"title"`
}

// PurgeForward is a forward file purge removes.
type PurgeForward struct {
	ULID    model.ULID `json:"ulid"`
	MovedTo model.ULID `json:"moved_to"`
	MovedAt time.Time  `json:"moved_at"`
}

// PurgePlan is what purge removes from a store.
type PurgePlan struct {
	Store    string         `json:"store"`
	Tasks    []PurgeTask    `json:"tasks"`
	Forwards []PurgeForward `json:"forwards"`
}

// Empty reports whether there is nothing to purge.
func (p PurgePlan) Empty() bool { return len(p.Tasks) == 0 && len(p.Forwards) == 0 }

// keys are the ULIDs of the plan, sorted, to tell whether two plans are the
// same.
func (p PurgePlan) keys() []string {
	out := make([]string, 0, len(p.Tasks)+len(p.Forwards))
	for _, t := range p.Tasks {
		out = append(out, t.ULID.String())
	}
	for _, f := range p.Forwards {
		out = append(out, f.ULID.String())
	}
	slices.Sort(out)
	return out
}

// PurgeResult is what a purge did. Commit is empty when nothing was removed.
type PurgeResult struct {
	PurgePlan
	Commit string `json:"commit,omitempty"`
}

// purgePlan works out what to remove from the store name, whose index is ix:
// the tasks deleted at least days ago and the forwards that moved at least
// days ago. A deleted task is removed only when everything under it is
// deleted and old enough too. notDeleted names the deleted tasks that stay
// because a task under them is not deleted; broken counts the forwards that
// do not decode, which stay.
func (ss *Session) purgePlan(name string, ix *index.Index, days int) (plan PurgePlan, notDeleted []string, broken int) {
	cutoff := ss.Now().Add(-time.Duration(days) * 24 * time.Hour)
	plan = PurgePlan{Store: name, Tasks: []PurgeTask{}, Forwards: []PurgeForward{}}
	tasks := ix.Tasks()
	slices.SortFunc(tasks, func(a, b *index.Entry) int { return cmp.Compare(a.ULID.String(), b.ULID.String()) })
	old := make(map[model.ULID]bool, len(tasks))
	for _, e := range tasks {
		if e.DeletedAt != nil && !e.DeletedAt.After(cutoff) {
			old[e.ULID] = true
		}
	}
	for _, e := range tasks {
		if !old[e.ULID] {
			continue
		}
		under := Descendants(ix, e.ULID, every)
		if slices.ContainsFunc(under, func(d *index.Entry) bool { return d.DeletedAt == nil }) {
			notDeleted = append(notDeleted, cmp.Or(ix.DisplayID(e), e.ULID.String()))
			continue
		}
		if slices.ContainsFunc(under, func(d *index.Entry) bool { return !old[d.ULID] }) {
			continue // it goes once what is under it is old enough
		}
		plan.Tasks = append(plan.Tasks, PurgeTask{ULID: e.ULID, ID: ix.DisplayID(e), Title: e.Title})
	}
	for _, f := range ix.Forwards() {
		switch {
		case f.Broken != "":
			broken++
		case !f.MovedAt.After(cutoff):
			plan.Forwards = append(plan.Forwards, PurgeForward{ULID: f.ULID, MovedTo: f.MovedTo, MovedAt: f.MovedAt.UTC()})
		}
	}
	return plan, notDeleted, broken
}

// warnPurgeLeft tells of what purge leaves although it is old enough.
func (ss *Session) warnPurgeLeft(notDeleted []string, broken int) {
	if len(notDeleted) > 0 {
		ss.warn("kept (a task under each is not deleted): " + strings.Join(notDeleted, ", "))
	}
	if broken > 0 {
		ss.warn(fmt.Sprintf("%d broken forward files left; see hedgehug doctor", broken))
	}
}

// PurgeList lists what Purge would remove from the store (a store-unit
// command) and warns of what it leaves.
func (ss *Session) PurgeList(days int) (PurgePlan, error) {
	_, e, err := ss.Entry()
	if err != nil {
		return PurgePlan{}, err
	}
	_, ix, err := ss.OpenForWrite()
	if err != nil {
		return PurgePlan{}, err
	}
	plan, notDeleted, broken := ss.purgePlan(e.Name, ix, days)
	ss.warnPurgeLeft(notDeleted, broken)
	return plan, nil
}

// Purge removes the deleted tasks (with their comments and work logs) and the
// forwards that are at least days old, in one commit (Hedgehug-Op: purge).
// Without yes it is confirm.required. None is no commit and no error. With
// yes it removes what there is under the lock.
func (ss *Session) Purge(days int, yes bool) (PurgeResult, error) {
	return ss.purge(days, nil, yes)
}

// PurgeConfirmed is Purge for what the caller showed to the user and the user
// agreed to (PurgeList lists it): it fails with purge.changed when what is to
// be removed under the lock is no longer that.
func (ss *Session) PurgeConfirmed(days int, confirmed PurgePlan) (PurgeResult, error) {
	if confirmed.Empty() {
		return PurgeResult{}, confirmRequired("purge")
	}
	return ss.purge(days, &confirmed, true)
}

// purge is Purge; a nil confirmed skips the comparison.
func (ss *Session) purge(days int, confirmed *PurgePlan, yes bool) (PurgeResult, error) {
	_, e, err := ss.Entry()
	if err != nil {
		return PurgeResult{}, err
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return PurgeResult{}, err
	}
	plan, _, _ := ss.purgePlan(e.Name, ix, days)
	if plan.Empty() && confirmed == nil {
		return PurgeResult{PurgePlan: plan}, nil
	}
	if !yes {
		return PurgeResult{}, confirmRequired("purge")
	}
	op := store.Op{Name: "purge", Detail: fmt.Sprintf("older than %dd", days)}
	hash, err := ss.UpdateCommit(s, op, func(tx *store.Tx) error {
		// Read again under the lock: what came in by a sync since is removed
		// too when the removal was not confirmed one by one; if it was, a
		// change is refused (purge.changed).
		if err := ix.Refresh(); err != nil {
			return err
		}
		plan, _, _ = ss.purgePlan(e.Name, ix, days)
		if confirmed != nil && !slices.Equal(plan.keys(), confirmed.keys()) {
			return newErr(KindConflict, "purge.changed", errors.New("what is to be purged changed after the confirmation; run purge again"))
		}
		for _, t := range plan.Tasks {
			if err := tx.RemoveTask(t.ULID); err != nil {
				return err
			}
		}
		for _, f := range plan.Forwards {
			if err := tx.RemoveTask(f.ULID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return PurgeResult{}, err
	}
	return PurgeResult{PurgePlan: plan, Commit: commitHex(hash)}, nil
}
```

（`warnPurgeLeft` の `fmt.Sprintf("%d broken forward files left; …")` は今の文言のまま。複数形は Task 12 が `plural.Count` に直す。ここで `plural` を `import` していると未使用になるので、この Step では `plural` の `import` は書かない。上のコードの `import` から `"github.com/kukv/hedgehug/internal/plural"` の行を消す。）

`internal/ops/purgemoved.go` を消す。`internal/ops/delete.go` から `PurgeTargets`・`Purge`・`PurgeUnchecked`・`purge`・`purgeTargets`・`sameTasks`（`// PurgeTargets lists ...` のコメントから `sameTasks` の終わりまで。`current` と `notDeletedYet` と `isDeleted` と `restorable` は残す）を消し、`isDeleted` を次にする。

```go
// isDeleted refuses to restore the task e when it is not deleted.
func isDeleted(ix *index.Index, e *index.Entry) error {
	if e.DeletedAt == nil {
		return newErr(KindConflict, "task.not_deleted", fmt.Errorf("%s is not deleted", RefName(ix, e.ULID)), "id", TaskFileName(e))
	}
	return nil
}
```

`restorable` の中の呼び出しを `isDeleted(ix, e)` にする。使われなくなった `import`（`slices`、`model` など）は `go vet` が示すので消す。`AffectedResult.Views` は触らない。

`internal/ops/undo.go` の `undoBlocked` に1行足し（`rewrite-history` の行は Task 10 で消えている）、393 行の文言を直す。

```go
var undoBlocked = map[string]string{
	"move-in":  "a move between stores cannot be undone; move it back with hedgehug mv --store",
	"move-out": "a move between stores cannot be undone; move it back with hedgehug mv --store",
	"purge":    "a purge cannot be undone",
}
```
```go
			return newErr(KindConflict, "undo.blocked", fmt.Errorf("the workspace still has %d tasks; move them with hedgehug mv --workspace, or delete them with hedgehug rm and remove them with hedgehug purge --older-than 0d --store <store>", n))
```

`internal/ops/workspace.go:255` を次にする。

```go
			fmt.Errorf("workspace %s still has %d tasks; move them with hedgehug mv --workspace, or delete them with hedgehug rm and remove them with hedgehug purge --older-than 0d --store <store>", ws.Name, n))
```

`internal/ops/duplicates.go:65`（と 55 行のコメントの引用）を次にする。重複を消す手順が、`purge` の既定の90日で消えないままになるのを避ける。

```go
	return "in " + list + "; remove the one not needed with hedgehug rm and hedgehug purge --older-than 0d --store <store>"
```

テストの文言を合わせる: `internal/ops/undo_test.go:164` は `"the workspace still has 1 tasks"` のままでよい（前半だけを見る）。`internal/ops/workspace_test.go:52` の `want` を `"workspace front still has 1 tasks; move them with hedgehug mv --workspace, or delete them with hedgehug rm and remove them with hedgehug purge --older-than 0d --store <store>"` にする。`internal/ops/duplicates_test.go:77` の `want` の末尾を `hedgehug purge --older-than 0d --store <store>` にする。`cmd/hedgehug/testdata/TestUndoWorkspaceWithTasksIsRefused/undo.err.golden` の1行目を `hedgehug: workspaces/<ULID-1>.md: the workspace still has 1 tasks; move them with hedgehug mv --workspace, or delete them with hedgehug rm and remove them with hedgehug purge --older-than 0d --store <store>` にする。`cmd/hedgehug/testdata/TestDuplicateAcrossStoresIsAWarning/doctor.out.golden` の2行目と6行目、`doctor-json.out.golden` の9行目と21行目の `hedgehug purge` を `hedgehug purge --older-than 0d --store <store>` にする（行の中身は `... and hedgehug purge (warning)` → `... and hedgehug purge --older-than 0d --store <store> (warning)` のように、`hedgehug purge` の後ろに足すだけ）。`cmd/hedgehug/purge_moved_test.go` の重複のテスト2つ（`TestDuplicateAcrossStoresIsAWarning`・`TestSyncBringsADuplicateAcrossStores`）の `"... and hedgehug purge"` の期待を、同じく `hedgehug purge --older-than 0d --store <store>` で終わる形にする。

`internal/store/forward_test.go:57,63` の op 名 `purge-moved` を `purge` にし、コメントの `(purge --moved)` を `(purge)` にする。`internal/index/forward_test.go:109` はすでに `purge`。

- [ ] **Step 18: ops のテストが通ることを確かめる**

Run: `go vet ./internal/... && go test ./internal/ops ./internal/plural ./internal/store ./internal/index ./internal/merge -count=1`（timeout 600000）
Expected: PASS

#### D. CLI の purge

- [ ] **Step 19: CLI のテストを書く**

`cmd/hedgehug/purge_test.go` を作る。`te.now` は `2026-10-30T12:00:00Z` で、`te.now = te.now.AddDate(0, 0, n)` で進める（`rm` の `deleted_at` と `mv --store` の `moved_at` は `te.now`）。

```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func TestPurgeOlderThan(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	b := te.addJSON("b")
	te.ok("rm", a.ID)
	te.now = te.now.AddDate(0, 0, 50)
	te.ok("rm", b.ID)
	te.now = te.now.AddDate(0, 0, 50) // a was deleted 100 days ago, b 50 days ago
	if out := te.ok("purge", "--older-than", "90d", "--yes"); out != "Purged 1 task and 0 forward files from me\n" {
		t.Errorf("purge --older-than 90d printed %q", out)
	}
	if out := te.ok("ls", "is:deleted", "--ulids-only"); strings.TrimSpace(out) != b.ULID {
		t.Errorf("deleted tasks left = %q; want only b (%s)", out, b.ULID)
	}
	if !strings.Contains(te.log("me")[0].Message, "Hedgehug-Op: purge\n") {
		t.Errorf("not a purge commit:\n%s", te.log("me")[0].Message)
	}
	// the default is 90 days: b is 50 days old, and 91 a little later
	if out := te.ok("purge", "--yes"); out != "Nothing to purge\n" {
		t.Errorf("purge printed %q", out)
	}
	te.now = te.now.AddDate(0, 0, 41)
	if out := te.ok("purge", "--yes"); out != "Purged 1 task and 0 forward files from me\n" {
		t.Errorf("purge printed %q", out)
	}
}

func TestPurgeZeroDaysAndNothingToDo(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("p")
	te.ok("rm", p.ID)
	if out := te.ok("purge", "--older-than", "1d", "--yes"); out != "Nothing to purge\n" {
		t.Errorf("1d printed %q", out)
	}
	te.ok("purge", "--older-than", "0d", "--yes")
	before := len(te.log("me"))
	if out := te.ok("purge", "--older-than", "0d", "--yes"); out != "Nothing to purge\n" {
		t.Errorf("printed %q", out)
	}
	if got := len(te.log("me")); got != before {
		t.Errorf("nothing to purge wrote %d commits", got-before)
	}
}

func TestPurgeConfirmOnATerminal(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("P")
	te.addJSON("C", "--parent", p.ID)
	te.ok("rm", p.ID)
	prompt := "me:\n  ME-1  P\n  ME-2  C\nPurge 2 tasks and 0 forward files from me for good? [y/N] "
	te.terminal = true

	te.stdin = "n\n"
	code, out, errs := te.run("purge", "--older-than", "0d")
	if code != 1 || out != "" || errs != prompt+"hedgehug: Canceled\n" {
		t.Errorf("declined: exit %d, stdout %q, stderr %q", code, out, errs)
	}
	if got := te.ok("ls", "is:deleted", "--ulids-only"); strings.Count(got, "\n") != 2 {
		t.Errorf("a declined purge removed tasks: %q", got)
	}

	te.stdin = "y\n"
	code, out, errs = te.run("purge", "--older-than", "0d")
	if code != 0 || out != "Purged 2 tasks and 0 forward files from me\n" || errs != prompt {
		t.Errorf("accepted: exit %d, stdout %q, stderr %q", code, out, errs)
	}
}

func TestPurgeWithoutYesOffATerminal(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("p")
	te.ok("rm", p.ID)
	before := len(te.log("me"))
	code, _, errs := te.run("purge", "--older-than", "0d")
	if code != 2 || errs != "hedgehug: purge: confirm with --yes\n" {
		t.Errorf("exit %d, stderr %q; want 2 and the refusal", code, errs)
	}
	if got := len(te.log("me")); got != before {
		t.Errorf("a refused purge wrote %d commits", got-before)
	}
}

// The forward files of moved tasks go by their moved_at.
func TestPurgeForwardsAndJSON(t *testing.T) {
	t.Parallel()
	te := newForwardEnv(t) // AA-2 moved to bravo-long at te.now
	if out := te.ok("purge", "--older-than", "1d", "--store", "alpha", "--yes"); out != "Nothing to purge\n" {
		t.Errorf("1d printed %q; the forward is new", out)
	}
	te.now = te.now.AddDate(0, 0, 10)
	if out := te.ok("purge", "--older-than", "11d", "--store", "alpha", "--yes"); out != "Nothing to purge\n" {
		t.Errorf("11d printed %q", out)
	}
	out := te.ok("purge", "--older-than", "10d", "--store", "alpha", "--yes", "--json")
	var got struct {
		Store string `json:"store"`
		Tasks []any  `json:"tasks"`
		Fwds  []struct {
			ULID    string `json:"ulid"`
			MovedTo string `json:"moved_to"`
			MovedAt string `json:"moved_at"`
		} `json:"forwards"`
		Commit string `json:"commit"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got.Store != "alpha" || got.Tasks == nil || len(got.Tasks) != 0 || len(got.Fwds) != 1 || got.Commit == "" ||
		got.Fwds[0].MovedAt != "2026-10-30T12:00:00Z" || got.Fwds[0].ULID == "" || got.Fwds[0].MovedTo == "" {
		t.Errorf("json = %s", out)
	}
	var none map[string]any
	if err := json.Unmarshal([]byte(te.ok("purge", "--older-than", "0d", "--store", "alpha", "--yes", "--json")), &none); err != nil {
		t.Fatal(err)
	}
	if f, ok := none["forwards"].([]any); !ok || len(f) != 0 || none["commit"] != nil {
		t.Errorf("none = %v; want empty lists and no commit", none)
	}
	if code, _, errs := te.run("show", "alpha:AA-2"); code == 0 || strings.Contains(errs, "moved to") {
		t.Errorf("show of the old id after the purge: exit %d, %q", code, errs)
	}
}

func TestPurgeNeedsAStoreWhenThereAreSeveral(t *testing.T) {
	t.Parallel()
	te := newForwardEnv(t)
	code, out, errs := te.run("purge", "--yes")
	if code != 1 || out != "" || errs != "hedgehug: several stores are set up; pick one with --store (stores: alpha, bravo-long)\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errs)
	}
}

// A forward to a store that is not mounted is purged like the others.
func TestPurgeAForwardToAStoreNotMounted(t *testing.T) {
	t.Parallel()
	te := newForwardEnv(t)
	te.ok("store", "remove", "bravo-long")
	if out := te.ok("purge", "--older-than", "0d", "--yes"); out != "Purged 0 tasks and 1 forward file from alpha\n" {
		t.Errorf("printed %q", out)
	}
}

func TestPurgeUsageErrors(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"purge", "ME-1"}, "--older-than"},
		{[]string{"purge", "--moved"}, "unknown flag: --moved"},
		{[]string{"purge", "--older-than", "90"}, `"90" is not a period`},
		{[]string{"purge", "--older-than=-5d"}, `"-5d" is not a period`},
		{[]string{"purge", "--older-than", "1w"}, `"1w" is not a period`},
		{[]string{"purge", "--older-than", "d"}, `"d" is not a period`},
	} {
		code, _, errs := te.run(c.args...)
		if code != 2 || !strings.Contains(errs, c.want) {
			t.Errorf("%v: exit %d, stderr %q; want 2 and %q", c.args, code, errs, c.want)
		}
	}
}

// A deleted task with a task under it that is not deleted stays; the other
// deleted tasks go, and purge tells of the one that stays.
func TestPurgeKeepsATaskWithALiveChild(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("p")
	c := te.addJSON("c", "--parent", p.ID)
	g := te.addJSON("g")
	te.ok("rm", p.ID)
	te.ok("rm", g.ID)
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	// the child is alive again under a deleted parent (restore would refuse it)
	if _, err := s.Update(store.Op{Name: "update", Author: gitrepo.Signature{Name: "T", Email: "t@example.com"}},
		changeTask(c.ULID, func(t *model.Task) { t.DeletedAt = nil })); err != nil {
		t.Fatal(err)
	}
	code, out, errs := te.run("purge", "--older-than", "0d", "--yes")
	if code != 0 || out != "Purged 1 task and 0 forward files from me\n" || !strings.Contains(errs, "warning: kept (a task under each is not deleted): "+p.ID) {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errs)
	}
	if got := te.task(p.ULID); got.DeletedAt == nil {
		t.Error("the deleted parent lost its deleted_at")
	}
}

func TestPurgeRecordsTheNumbers(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	b := te.addJSON("b")
	te.ok("rm", a.ID)
	te.ok("rm", b.ID)
	te.ok("purge", "--older-than", "0d", "--yes")
	data, err := os.ReadFile(filepath.Join(te.storeDir("me"), filepath.FromSlash(model.NumbersPath)))
	if err != nil || !strings.Contains(string(data), "ME: 2\n") {
		t.Errorf("numbers.yaml = %q, %v; want ME: 2", data, err)
	}
	if next := te.addJSON("c"); next.ID != "ME-3" {
		t.Errorf("the next id = %s; want ME-3, not a number that was purged", next.ID)
	}
}

func TestUndoDoesNotTakeBackAPurge(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("p")
	te.ok("rm", p.ID)
	te.ok("purge", "--older-than", "0d", "--yes")
	before := len(te.log("me"))
	code, _, errs := te.run("undo")
	if code != 1 || !strings.Contains(errs, "a purge cannot be undone") {
		t.Errorf("undo: exit %d, stderr %q", code, errs)
	}
	if got := len(te.log("me")); got != before {
		t.Errorf("undo wrote %d commits", got-before)
	}
}

// workspace rm counts the deleted tasks too, and says how to get rid of them.
func TestWorkspaceRmAfterRmNeedsAPurge(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("workspace", "add", "api", "--prefix", "API")
	x := te.addJSON("x", "--workspace", "api")
	te.ok("rm", x.ID)
	code, _, errs := te.run("workspace", "rm", "api")
	want := "move them with hedgehug mv --workspace, or delete them with hedgehug rm and remove them with hedgehug purge --older-than 0d --store <store>"
	if code != 1 || !strings.Contains(errs, want) {
		t.Errorf("workspace rm: exit %d, stderr %q; want it to say %q", code, errs, want)
	}
	te.ok("purge", "--older-than", "0d", "--yes")
	te.ok("workspace", "rm", "api")
}
```

（`te.task(ulid)`・`changeTask` は `cmd/hedgehug/add_test.go`・`meanwhile_test.go` の既存。`newForwardEnv` は `purge_moved_test.go` に残すヘルパー。`newStoreEnv` の Store は `me`、接頭辞は `ME`。`TestPurgeKeepsATaskWithALiveChild` は `p` が `rm` で子の `c` ごと消えた状態から、`c` だけ生き返らせる。`g` は消えた別のタスクで、こちらは消える。）

古いテストを置き換える。

- `cmd/hedgehug/purge_moved_test.go`: `TestPurgeMovedListAndConfirmOnATerminal` から `TestPurgeMovedAndAnIDDoNotMix` まで（43〜116 行の6つ）を消す。`storeTaskFile`・`newForwardEnv`・`TestDoctorForwardTargetIsAnInfo` 以降は残す。使われなくなった `import`（`encoding/json` など）は `go vet` が示すので消す。
- `cmd/hedgehug/testdata/` の `TestPurgeMovedAndAnIDDoNotMix`・`TestPurgeMovedListAndConfirmOnATerminal`・`TestPurgeMovedListsAStoreNotMounted`・`TestPurgeMovedNeedsAStoreWhenThereAreSeveral`・`TestPurgeMovedYesAndJSON`・`TestPurgeMovedYesText` のディレクトリを `git rm -r` する。
- `cmd/hedgehug/rm_test.go` の `TestPurge`（115〜145 行）を消す。意味は `purge_test.go` の `TestPurgeOlderThan`・`TestPurgeConfirmOnATerminal`・`TestPurgeKeepsATaskWithALiveChild` が引き継ぐ。
- `cmd/hedgehug/meanwhile_test.go` の `TestPurgeSeesChildrenAddedMeanwhile`（111 行）と `TestPurgeRefusesATaskRestoredMeanwhile`（198 行）を次にする。

```go
func TestPurgeSeesChildrenAddedMeanwhile(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("p")
	te.ok("rm", p.ID)
	var c model.ULID
	te.meanwhile(putChild(p.ULID, &c)) // deleted, so it goes with p
	te.ok("purge", "--older-than", "0d", "--yes")
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Task(c); err == nil {
		t.Error("the deleted child added meanwhile is not purged")
	}

	// A child added meanwhile that is not deleted keeps its deleted parent.
	q := te.addJSON("q")
	te.ok("rm", q.ID)
	te.meanwhile(func(tx *store.Tx) error {
		if err := putChild(q.ULID, &c)(tx); err != nil {
			return err
		}
		live, err := tx.Task(c)
		if err != nil {
			return err
		}
		live.DeletedAt = nil
		return tx.PutTask(live)
	})
	if code, out, stderr := te.run("purge", "--older-than", "0d", "--yes"); code != 0 || out != "Nothing to purge\n" {
		t.Errorf("purge over a live child added meanwhile: exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	if got := te.task(q.ULID); got.DeletedAt == nil {
		t.Error("the parent of a live child lost its deleted_at")
	}

	// After a confirmation, a change to what is purged stops the purge.
	r := te.addJSON("r")
	te.ok("rm", r.ID)
	te.terminal, te.stdin = true, "y\n"
	te.meanwhile(putChild(r.ULID, &c))
	if code, _, stderr := te.run("purge", "--older-than", "0d"); code != 1 || !strings.Contains(stderr, "run purge again") {
		t.Errorf("purge after a change: exit %d, stderr %q", code, stderr)
	}
	if _, err := s.Task(c); err != nil {
		t.Errorf("a task that was not confirmed is purged: %v", err)
	}
}
```
```go
func TestPurgeSeesATaskRestoredMeanwhile(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("p")
	te.ok("rm", p.ID)
	te.meanwhile(changeTask(p.ULID, func(t *model.Task) { t.DeletedAt = nil }))
	if code, out, stderr := te.run("purge", "--older-than", "0d", "--yes"); code != 0 || out != "Nothing to purge\n" {
		t.Errorf("purge: exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	if got := te.task(p.ULID); got.DeletedAt != nil {
		t.Error("the task restored meanwhile is changed")
	}
}
```

- `cmd/hedgehug/scenario_test.go:91` を `step("purge", "purge", "--older-than", "0d", "--yes")` にする（コミット数 20 は変わらない: purge は1コミット）。
- `cmd/hedgehug/readonly_test.go:37` を `{"purge", "--older-than", "0d", "--yes"},` にする。
- `cmd/hedgehug/e2e_3c_test.go:234-248` の手順 4 を次にする（Task 10 が手順 5 以降を消している）。

```go
	// 4. purge takes the forward files away.
	pm := alice.ok("purge", "--older-than", "0d", "--store", "org-a", "--yes")
	if !strings.Contains(pm, "2 forward files") {
		t.Errorf("purge printed %q, want the two forwards removed", pm)
	}
```

  続く `show of the old id after purge --moved` のエラーメッセージ文字列の `purge --moved` も `purge` にする。

- [ ] **Step 20: 失敗を確かめる**

Run: `go test ./cmd/hedgehug -run 'Purge|UndoDoesNot|WorkspaceRmAfterRm' -count=1`（timeout 600000）
Expected: FAIL（まだ `purge.go` がなく、古い `purge` が動く。`unknown flag: --older-than`）

- [ ] **Step 21: CLI を実装する**

`cmd/hedgehug/rm.go` から `purgeCmd`・`purge`・`confirm`・`forwardLine`・`forwardFiles`・`purgeMoved`・`printPurgedMoved` を消し、`printAffected` を次にする（呼ぶのは `rm` と `restore` だけになり、`refresh` は常に true になるため引数を消す）。`rm` と `restore` の呼び出しから最後の `true` を消す。

```go
// printAffected prints the tasks a write changed, read again after the
// write: as JSON, or as one line about the task named.
func (a *app) printAffected(ix *index.Index, e *index.Entry, targets []*index.Entry, verb string) error {
	if a.jsonOut {
		if err := ix.Refresh(); err != nil {
			return err
		}
		out := make([]*ops.TaskView, 0, len(targets))
		for _, x := range targets {
			cur, _ := ix.Task(x.ULID)
			out = append(out, ops.TaskViewOf(ix, cur))
		}
		return a.printJSON(out)
	}
	line := fmt.Sprintf("%s %s %s", verb, a.style().id(ops.TaskLabel(ix, e)), e.Title)
	switch n := len(targets) - 1; n {
	case 0:
	case 1:
		line += " and 1 task under it"
	default:
		line += fmt.Sprintf(" and %d tasks under it", n)
	}
	_, err := fmt.Fprintln(a.stdout, line)
	return err
}
```

使われなくなった `import`（`errors`・`strings`・`model`）を消す。

`cmd/hedgehug/purge.go` を作る。

```go
package main

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/plural"
)

func (a *app) purgeCmd() *cobra.Command {
	var olderThan string
	var yes bool
	cmd := &cobra.Command{
		Use:   "purge [--older-than <N>d] [--store <name>] [--yes]",
		Short: "Remove, for good, the deleted tasks and the forward files of moved tasks that are older than a time",
		Long: "Purge removes the files of the tasks deleted more than N days ago, with their comments and work logs, and the forward files that moves to other stores left more than N days ago. " +
			"N is 90 unless --older-than says; 0d is everything.\n\n" +
			"A deleted task with a task under it that is not deleted stays, and purge tells of it.",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usageErrorf("purge takes no task id; it removes by age: hedgehug purge --older-than <N>d (0d is everything)")
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			days, err := ops.ParseOlderThan(olderThan)
			if err != nil {
				return err
			}
			return a.purge(days, yes)
		},
	}
	cmd.Flags().StringVar(&olderThan, "older-than", fmt.Sprintf("%dd", ops.DefaultPurgeDays), "remove what is older than this many days, such as 90d (0d for everything)")
	cmd.Flags().BoolVar(&yes, "yes", false, "do not ask")
	return cmd
}

// purge lists what is to go and, on a terminal, asks; off a terminal it goes
// only with --yes.
func (a *app) purge(days int, yes bool) error {
	plan, err := a.ss.PurgeList(days)
	if err != nil {
		return err
	}
	if plan.Empty() {
		return a.printPurged(ops.PurgeResult{PurgePlan: plan})
	}
	var res ops.PurgeResult
	switch {
	case yes:
		res, err = a.ss.Purge(days, true)
	case !a.isTerminal(a.stdin):
		res, err = a.ss.Purge(days, false) // the refusal the operation gives
	default:
		if err = a.confirmPurge(plan); err == nil {
			res, err = a.ss.PurgeConfirmed(days, plan)
		}
	}
	if err != nil {
		return err
	}
	return a.printPurged(res)
}

// confirmPurge shows what is to go and asks.
func (a *app) confirmPurge(p ops.PurgePlan) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s:\n", p.Store)
	for _, t := range p.Tasks {
		fmt.Fprintf(&b, "  %s  %s\n", cmp.Or(t.ID, t.ULID.String()), t.Title)
	}
	if len(p.Forwards) > 0 {
		fmt.Fprintf(&b, "  %s\n", plural.Count(len(p.Forwards), "forward file"))
	}
	fmt.Fprintf(&b, "Purge %s and %s from %s for good? [y/N] ", plural.Count(len(p.Tasks), "task"), plural.Count(len(p.Forwards), "forward file"), p.Store)
	if _, err := fmt.Fprint(a.stderr, b.String()); err != nil {
		return err
	}
	line, err := a.readLine()
	if err != nil && line == "" {
		return errors.New("Canceled")
	}
	if ans := strings.ToLower(strings.TrimSpace(line)); ans != "y" && ans != "yes" {
		return errors.New("Canceled")
	}
	return nil
}

// printPurged prints what purge removed: a line, or JSON.
func (a *app) printPurged(r ops.PurgeResult) error {
	if a.jsonOut {
		return a.printJSON(r)
	}
	if r.Empty() {
		_, err := fmt.Fprintln(a.stdout, "Nothing to purge")
		return err
	}
	_, err := fmt.Fprintf(a.stdout, "Purged %s and %s from %s\n", plural.Count(len(r.Tasks), "task"), plural.Count(len(r.Forwards), "forward file"), r.Store)
	return err
}
```

- [ ] **Step 22: 成功を確かめる**

Run: `go build ./... && go vet ./... && go test ./cmd/hedgehug ./internal/ops ./internal/store ./internal/index ./internal/merge ./internal/model ./internal/plural ./internal/doctor ./internal/mount ./internal/sync ./internal/inbox ./internal/server -count=1`（timeout 600000）
Expected: PASS。`internal/server` が落ちたら、Task 9 で API から消したはずの purge の呼び出しが残っている（`grep -rn 'Purge' internal/server`）。

- [ ] **Step 23: 消し残しがないことを確かめる**

Run: `grep -rn 'PurgeTargets\|PurgeUnchecked\|PurgeMoved\|purge-moved\|purge --moved\|purge.child_not_deleted\|purge.unconfirmed\|ForwardView' --include='*.go' . ; grep -rn 'purge <id>\|purge --moved' cmd internal web/core 2>/dev/null | grep -v node_modules`
Expected: 出力なし。あれば、その行が参照する物をこの Task で直す。

- [ ] **Step 24: コミットする**

```bash
git add -A internal cmd
git commit -m "$(cat <<'EOF'
feat(cli): purge を --older-than の1つの形にし、転送ファイルに moved_at を持たせる

hh purge [--older-than <N>d] [--store <name>] [--yes] にする。N 日より前に削除したタスクと、
N 日より前に移した転送ファイルを1コミットで消す。既定は 90 日、0d はすべて。purge <id> と
purge --moved は使い方の誤りにする。転送ファイルは ulid・moved_to・moved_at の3つを必須にし、
mv --store が時刻を書く。同じ移動先の転送ファイルのマージは moved_at が大きい方を取る。
コミットの op は purge 1つにし、undo は purge を取り消さない。workspace rm・undo の拒否の文言と、
重複の案内を purge --older-than 0d に合わせる。複数形の小さな共有パッケージ internal/plural を足す。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

#### spec にない細部

1. **`internal/plural` を Task 11 で先に作る。** この Task の出力（`Purged 1 task and 2 forward files`・確認の画面）が複数形を使い、Task 12 より前に入るため。使い方の全体と置き場所の理由は Task 12 の細部 1。
2. **「N 日より前」の境界は以下（`deleted_at`・`moved_at` が「今 − N 日」以前）を含む。** spec の `0d` は「すべて」で、`rm` した直後（`deleted_at` = 今）にも効く必要があり、`workspace rm` の案内（`rm` の直後に `purge --older-than 0d`）がこれに頼る。なので「より前」を「以前」にした。
3. **期間の上限は `36500d`（100 年）、書き方は数字と `d` だけ。** `time.Duration` が溢れないため。`+5d`・`-5d`・`1w`・空・`d` だけは使い方の誤り（`purge.bad_period`、終了コード 2）。spec は「負の値や書き方の誤りは使い方の誤り」までしか決めていない。
4. **子孫の規則。** 削除済みで古いタスクは、下のタスクがすべて削除済みで、すべて古いときだけ消す。下に削除されていないタスクがあれば消さずに警告で ID を並べる（spec どおり）。下に削除済みだが新しいタスクがあるだけなら、警告なしで残す（その子が古くなったら一緒に消える。先に親を消すと子の親がなくなる）。spec は前者だけを決めている。
5. **警告の文言。** `kept (a task under each is not deleted): ME-1, ME-2`。spec は「警告で ID を並べる」まで。警告は `PurgeList` だけが出す（CLI は常に先に `PurgeList` を呼ぶので、1回だけ出る）。
6. **確認の画面の形。** `<store>:` の行、削除済みタスクごとに `  <ID>  <タイトル>`、転送ファイルがあれば `  N forward file(s)`、最後に `Purge N tasks and M forward files from <store> for good? [y/N] `。やめたときのエラーは `Canceled`（今の `purge --moved` と同じ）。
7. **JSON の `commit` は、コミットがなければ省く。`tasks`・`forwards` は空でも `[]`。** 今の `purge --moved --json` の形に合わせた。
8. **コミットの `Detail` は `older than 90d`。** 今の `purge` は `Task` を、`purge-moved` は `forwards of <store>` を付けていた。1コミットに複数のタスクがあるので `Task` は付けない。
9. **`undo` は `purge` を `undo.blocked` で断る。文言は `a purge cannot be undone`。** spec は「今の `purge` と同じく取り消せない操作として扱う（理由の文言も今のまま）」と書くが、今のコードには `purge` を断る処理も文言もない（`undoBlocked` は `move-in`・`move-out`・`rewrite-history` だけで、`purge` の undo は消えたファイルを戻そうとする）。spec の意図（取り消せない扱い）に従い、`rewrite-history` の文言（`a history rewrite cannot be undone`）に並べて文言を新しく決めた。**利用者の承認が要る。**
10. **`ops.ParseOlderThan` を ops に置き、エラーは `KindUsage`・`purge.bad_period`。** 4e で GUI が期間つきの purge を呼ぶとき、同じ検査を使えるようにするため。
11. **`workspace rm` と `undo` の拒否の文言の `<store>` は、そのままの文字。** spec の文をそのまま使った。`still has %d tasks` の複数形は Task 12 が直す。
12. **重複の案内（`doctor`・`sync` の `remove the one not needed with hedgehug rm and hedgehug purge`）に `--older-than 0d --store <store>` を足す。** `purge` の既定が90日になり、`rm` の直後に消えなくなるため。spec は触れていない。
13. **同じ移動先の転送ファイルのマージは `moved_at` が大きい方、違う移動先は今のまま（コミット時刻の新しい方）。** spec は「競合したら大きい方」とだけ書く。違う移動先の勝ち方は「今の扱いのまま」を優先し、同じ移動先のときだけ `moved_at` で選ぶ。`moved_at` が同じなら今の規則（コミット時刻）。
14. **`moved_at` は `mv --store` の操作の時刻（ストアの時計）を、秒に切って UTC で書く。** コミットの時刻とは別に `tx` が読む（秒の境目で1秒ずれることがあるが、purge の判断には影響しない）。
15. **索引のキャッシュ形式を 7 → 8 に上げる。** `ForwardEntry` の形が変わるため（上げないと古いキャッシュを読んだとき `MovedAt` がゼロになる）。
16. **`moved_at` のない転送ファイルは壊れた転送ファイル。** purge は消さずに `N broken forward files left; see hedgehug doctor` と警告し、`doctor` が読めないファイルとして出す（spec §9.1 のとおり。リリース前のため移行はしない）。
17. **`printAffected` の `refresh` 引数と `isDeleted` の `hint` 引数を消す。** この Task で呼ぶ所が1つだけになるため（自分の変更で不要になったものの片付け）。
18. **ops のテストは時計を進めず、`deleted_at`・`moved_at` を手で書いて日数を変える。** `internal/ops` のテスト用セッションの時計は固定のため。CLI のテストは `te.now` を進める。

---

### Task 12: 英語の複数形と `ls --help`

spec §12。数を入れる英語の文言をすべて、1のとき単数・それ以外は複数にし、`ls --help` に `-` で始まる項の書き方を足す。Task 11 が作った `internal/plural`（`plural.Count`・`plural.Pick`）を、`cmd/hedgehug`・`internal/ops` と、同じ誤りのあった `internal/doctor`・`internal/store`・`internal/sync` が通す。

洗い出しの方法: `grep -rnE '%d|%\[[0-9]\]d' --include='*.go' cmd internal | grep -v _test.go` で `%d` を含む行をすべて見て、数に名詞が続くものを拾った。行番号は 2026-11-02 の main（6f060f0）のもので、Task 1〜11 で動いている。Step 1 で grep を取り直して一覧と突き合わせ、動いていれば合わせる。

#### 一覧

直すもの（数が 1 になりうる）:

| # | ファイル:行 | 今の文言 | 直した文言 |
| --- | --- | --- | --- |
| 1 | `cmd/hedgehug/sync.go:80` | `Synced %s: %d merges` | `Synced %s: %s`（`plural.Count(r.Merges, "merge")`）。`Synced org: 1 merge, pushed to origin` |
| 2 | `cmd/hedgehug/sync.go:87-91` | `read-only: 1 commit not pushed` / `read-only: %d commits not pushed`（手書きの switch） | `if r.Unpushable > 0` で `  read-only: %s not pushed\n`（`plural.Count(r.Unpushable, "commit")`） |
| 3 | `cmd/hedgehug/sync.go:111` | `  invalid: %d tasks now have problems; find them with hedgehug ls is:invalid` | `  invalid: %s now %s problems; find %s with hedgehug ls is:invalid`（`Count(n, "task")`・`Pick(n, "has", "have")`・`Pick(n, "it", "them")`）。`1 task now has problems; find it with …` |
| 4 | `cmd/hedgehug/sync.go:164-170` | `  1 commit not pushed` / `  %d commits not pushed`（手書きの switch） | `*x.Unpushed` が nil でない場合を1つにして `  %s not pushed\n`（`plural.Count(*x.Unpushed, "commit")`）。`0 commits not pushed` は変わらない |
| 5 | `cmd/hedgehug/schema.go:82` | `%d problems` | `plural.Count(len(fs), "problem")`。`1 problem` |
| 6 | `cmd/hedgehug/workspace.go:98` | `…; renumbered %d tasks` | `…; renumbered %s`（`Count(r.Renumbered, "task")`）。Task 5 の `set-prefix` の出力 |
| 7 | `cmd/hedgehug/mv.go:169` | `Move %d tasks to %s? [y/N] ` | `Move %s to %s? [y/N] `（`Count(len(v.Tasks), "task")`） |
| 8 | `cmd/hedgehug/rm.go`（`printAffected`、170 行付近） | `and 1 task under it` / `and %d tasks under it`（手書きの switch） | `if n := len(targets) - 1; n > 0 { line += " and " + plural.Count(n, "task") + " under it" }` |
| 9 | `cmd/hedgehug/inbox.go:94`・`98-103` | `Marked %d %s read` と 3 引数の `plural(n, one, many)` | `Marked %s read`（`Count(len(read), "item")`）。関数 `plural` を消す |
| 10 | `internal/ops/movestore.go:179` | `move %d tasks to %s` | `move %s to %s`（`Count(len(w.tasks), "task")`）。`confirm required` のエラー |
| 11 | `internal/ops/undo.go:393` | `the workspace still has %d tasks; …` | `the workspace still has %s; …`（`Count(n, "task")`） |
| 12 | `internal/ops/workspace.go:255` | `workspace %s still has %d tasks; …` | `workspace %s still has %s; …`（`Count(n, "task")`） |
| 13 | `internal/ops/discover.go:99` | `%d of %d stores could not be added: …` | `%d of %s could not be added: …`（2つ目を `Count(len(cands), "store")`）。`1 of 1 store could not be added` |
| 14 | `internal/ops/storeops.go:662` | `store %s has %d commits not pushed; …` | `store %s has %s not pushed; …`（`Count(n, "commit")`） |
| 15 | `internal/ops/maintain.go:170-175` | `TaskCount` の手書きの if | 本体を `return plural.Count(n, "task")` にする（呼び出しは変えない） |
| 16 | `internal/ops/purge.go`（`warnPurgeLeft`、Task 11） | `%d broken forward files left; see hedgehug doctor` | `%s left; see hedgehug doctor`（`Count(broken, "broken forward file")`）。`1 broken forward file left; …` |
| 17 | `internal/ops/inbox.go:537-540` | `%d unpushed changes` と、1 のときの手書きの代入 | Task 10 が `RewriteApplied` を消していればこの行はない。残っていれば `plural.Count(d.Replayed, "unpushed change")` の1行にする |
| 18 | `internal/doctor/doctor.go:202` | `%d merge driver records are not in the inbox yet` | `%s %s not in the inbox yet`（`Count(waiting, "merge driver record")`・`Pick(waiting, "is", "are")`）。`1 merge driver record is not in the inbox yet` |
| 19 | `internal/store/validate.go:249` | `workspace %s still has %d tasks` | `workspace %s still has %s`（`Count(count[ulid], "task")`） |
| 20 | `internal/store/sprint_refs.go:131,134` | `workspace %s still has %d sprints` / `… %d releases` | `… still has %s`（`Count(sprints[ulid], "sprint")`・`Count(releases[ulid], "release")`） |
| 21 | `internal/sync/rebuild.go:259` | `Detail: fmt.Sprintf("%d unpushed commits", …)` | `Detail: plural.Count(len(mine), "unpushed commit")` |
| 22 | `internal/sync/sync.go:882` | `Detail: fmt.Sprintf("%d issues", …)` | `Detail: plural.Count(len(repaired), "issue")`。コミットの件名が `hedgehug: repair 1 issue` になる |

直さないもの（理由つき）:

| ファイル:行 | 文言 | 理由 |
| --- | --- | --- |
| `internal/ops/session.go:580`、`internal/mount/resolve.go:45,49`、`internal/index/resolve.go:20`、`internal/index/workspace.go:23`、`internal/context/resolve.go:363` | `%q matches %d tasks` など | 曖昧さのエラーで、候補が2以上のときだけ出る |
| `cmd/hedgehug/ls.go:81` | `== %s (%d)` | 名詞が続かない件数 |
| `cmd/hedgehug/store_init.go:119,130,146` | `%d) …`・`[%d]`・`1-%d` | 選択肢の番号 |
| `internal/ops/progress.go:70,73` | `☑ %d/%d`・`%d/%d` | 名詞が続かない |
| `internal/doctor/schema.go`・`internal/schema/check.go` の `[%d]` | 添字、`has %d`（`needs exactly one default status, has %d`） | 添字。後者は「ちょうど1つ必要」への違反なので 1 にならない |
| `internal/doctor/doctor.go:251` | `the number %d is less than 1` | 名詞が続かない |
| `internal/model/*.go` の `line %d`・`format %d`、`internal/query/parse.go` の `term %d`、`internal/config/config.go:600` | 行番号・番号 | 件数ではない |
| `internal/github/*.go`・`internal/fakegh/*.go` | HTTP の状態コード | 件数ではない |
| `internal/ops/addhere.go:135` | `unknown kind %d` | 内部の列挙値 |
| `internal/model/rewrites.go:31` | `record %d` | Task 10 で消える。番号 |

- [ ] **Step 1: 一覧を取り直す**

Run: `grep -rnE '%d|%\[[0-9]\]d|Itoa\(' --include='*.go' cmd internal | grep -v _test.go | grep -v 'internal/fakegh\|internal/github'`
Expected: 上の「直すもの」の行がすべて見つかり、「直さないもの」以外に数に名詞が続く行がない。あれば、その行を一覧に足して同じ関数で直す。

- [ ] **Step 2: テストを直す（新しい文言に）**

今の誤った文言を固定しているテストと golden を、新しい文言にする。

| ファイル:行 | 今 | 直した後 |
| --- | --- | --- |
| `cmd/hedgehug/testdata/TestSyncShowsOverwritten/sync.golden:1` | `Synced org: 1 merges, pushed to origin` | `Synced org: 1 merge, pushed to origin` |
| `cmd/hedgehug/testdata/TestSyncShowsRenumbered/sync.golden:1` | 同上 | 同上 |
| `cmd/hedgehug/testdata/TestSyncTwoUsers/bob.golden:1` | 同上 | 同上 |
| `cmd/hedgehug/testdata/TestMvStoreConfirmOnATerminal/declined.err.golden:3` | `Move 1 tasks to bravo-long? [y/N] hedgehug: Canceled` | `Move 1 task to bravo-long? [y/N] hedgehug: Canceled` |
| `cmd/hedgehug/testdata/TestMvStoreConfirmOnATerminal/accepted.err.golden:3` | `Move 1 tasks to bravo-long? [y/N] ` | `Move 1 task to bravo-long? [y/N] ` |
| `cmd/hedgehug/testdata/TestMvStoreScopedQuestion/carried.err.golden:4` | `Move 1 tasks to bravo-long? [y/N] ` | `Move 1 task to bravo-long? [y/N] ` |
| `cmd/hedgehug/testdata/TestMvStoreTellsWhoCannotSeeItAnymore/hidden.err.golden:4` | `Move 1 tasks to bravo-long? [y/N] ` | `Move 1 task to bravo-long? [y/N] ` |
| `cmd/hedgehug/testdata/TestDoctorForwardTargetIsAnInfo/text.out.golden:2` | `1 problems` | `1 problem` |
| `cmd/hedgehug/testdata/TestDuplicateAcrossStoresIsAWarning/doctor.out.golden:3,7` | `1 problems` | `1 problem` |
| `cmd/hedgehug/testdata/TestUndoWorkspaceWithTasksIsRefused/undo.err.golden:1` | `…the workspace still has 1 tasks; …` | `…the workspace still has 1 task; …` |
| `cmd/hedgehug/doctor_test.go:164` | `"1 merge driver records are not in the inbox yet"` | `"1 merge driver record is not in the inbox yet"` |
| `internal/ops/undo_test.go:164` | `"the workspace still has 1 tasks"` | `"the workspace still has 1 task"` |
| `internal/ops/workspace_test.go:52` | `workspace front still has 1 tasks; move them …` | `workspace front still has 1 task; move them …`（後ろは Task 11 のまま） |
| `internal/ops/storeops_test.go:421` | `store me has 1 commits not pushed; run hedgehug sync --store me first` | `store me has 1 commit not pushed; run hedgehug sync --store me first` |
| `internal/ops/purge_broken_test.go`（Task 11 の `TestPurgeLeavesABrokenForward`） | `1 broken forward files left; see hedgehug doctor` | `1 broken forward file left; see hedgehug doctor` |
| `internal/sync/sync_test.go:415` | `"hedgehug: repair 1 issues"` | `"hedgehug: repair 1 issue"` |
| `internal/store/sprint_workspace_test.go:109` | `"still has 1 "+what` | `what` が複数形（`sprints`・`releases`）なら、`what` の末尾の `s` を取った形で検査する（`"still has 1 "+strings.TrimSuffix(what, "s")`） |

`internal/store/validate.go:249` の文言（`workspace … still has 1 tasks`）を固定するテストがあれば同じく直す: `grep -rn 'still has' --include='*_test.go' internal cmd`。`internal/store/merge_test.go:22,27` は `CommitMerge` に手で `Detail: "1 issues"` を渡すテストなので直さない（同期のコードを通らない）。

テストを足す。1 のときの文言を確かめる。

`cmd/hedgehug/sync_test.go`（なければ `cmd/hedgehug/sync_conn_test.go` の隣の既存の sync のテストファイル）に足す。

```go
func TestSyncStatusSaysOneCommit(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	remote := te.bare("org")
	te.ok("store", "remove", "me")
	te.ok("store", "init", "org", "--prefix", "ORG", "--remote", remote)
	te.ok("add", "one")
	if out := te.ok("sync", "status"); !strings.Contains(out, "  1 commit not pushed\n") || strings.Contains(out, "1 commits") {
		t.Errorf("sync status:\n%s", out)
	}
}
```

（`sync status` のサブコマンドの綴りと、Store の作り方は、`cmd/hedgehug/testdata/TestSyncStatus` を作っているテストに合わせる。`grep -n 'func TestSyncStatus' cmd/hedgehug/*_test.go` で見て、そのテストの準備に `add` を1つだけ足した形にする。上の例の準備と違えば、そちらに合わせてよい。確かめるのは「未 push のコミットが1つのとき `1 commit not pushed`」だけ。）

`cmd/hedgehug/ls_test.go` に足す。

```go
func TestLsHelpExplainsATermThatStartsWithADash(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	out := te.ok("ls", "--help")
	want := "A term that starts with - (a negation, such as -status:done) is read as a\nflag; put it after --: hedgehug ls -- -status:done"
	if !strings.Contains(out, want) {
		t.Errorf("ls --help lacks the note about -:\n%s", out)
	}
	// and what it says works
	todo := te.addJSON("still open")
	done := te.addJSON("finished")
	te.ok("done", done.ID)
	got := te.ok("ls", "--ulids-only", "--", "-status:done")
	if !strings.Contains(got, todo.ULID) || strings.Contains(got, done.ULID) {
		t.Errorf("ls -- -status:done = %q; want only %s", got, todo.ULID)
	}
}
```

- [ ] **Step 3: 失敗を確かめる**

Run: `go test ./cmd/hedgehug ./internal/ops ./internal/sync ./internal/store ./internal/doctor -count=1`（timeout 600000）
Expected: FAIL（新しい文言を期待するテスト・golden が、まだ古い文言を見て落ちる。`ls --help` のテストも落ちる）

- [ ] **Step 4: 実装する**

一覧の1〜22を、表の「直した文言」のとおりに直す。コードの形は次のとおり（`import` に `"github.com/kukv/hedgehug/internal/plural"` を足す。`cmd/hedgehug` と `internal/ops` では同じ名前でよい）。

`cmd/hedgehug/sync.go` の `printSync`:

```go
		fmt.Fprintf(&b, "Synced %s: %s", r.Store, plural.Count(r.Merges, "merge"))
```
```go
	if r.Unpushable > 0 {
		fmt.Fprintf(&b, "  read-only: %s not pushed\n", plural.Count(r.Unpushable, "commit"))
	}
```
```go
	if n := len(r.Invalid); n > 0 {
		fmt.Fprintf(&b, "  invalid: %s now %s problems; find %s with hedgehug ls is:invalid\n",
			plural.Count(n, "task"), plural.Pick(n, "has", "have"), plural.Pick(n, "it", "them"))
	}
```

同じファイルの状態の表示（164〜170 行）:

```go
		switch {
		case x.Unpushed == nil:
			b.WriteString("  no remote\n")
		default:
			fmt.Fprintf(&b, "  %s not pushed\n", plural.Count(*x.Unpushed, "commit"))
		}
```

`cmd/hedgehug/schema.go:82`:

```go
	_, err := fmt.Fprintln(a.stdout, plural.Count(len(fs), "problem"))
```

`cmd/hedgehug/workspace.go:98`:

```go
	fmt.Fprintf(&b, "Changed the id prefix of %s to %s (%s); renumbered %s\n", w.Name, w.IDPrefix, w.ULID, plural.Count(r.Renumbered, "task"))
```

`cmd/hedgehug/mv.go:169`:

```go
	fmt.Fprintf(&b, "Move %s to %s? [y/N] ", plural.Count(len(v.Tasks), "task"), v.To)
```

`cmd/hedgehug/rm.go` の `printAffected` の switch を次にする。

```go
	if n := len(targets) - 1; n > 0 {
		line += " and " + plural.Count(n, "task") + " under it"
	}
```

`cmd/hedgehug/inbox.go`:

```go
	_, err = fmt.Fprintf(a.stdout, "Marked %s read\n", plural.Count(len(read), "item"))
```

同じファイルの `func plural(n int, one, many string) string { … }`（98〜103 行）を消す。Task 10 の `doctor --fix` の出力（`writeRebuilt`、`put back N unpushed commit(s)`）がこの 3 引数の `plural` を使っているので、そこも `plural.Count(n, "unpushed commit")` に直す。最後に `grep -rn 'plural(' cmd` で、ほかに使う所がないことを確かめる。

`internal/ops/movestore.go:179`:

```go
		ce := confirmRequired(fmt.Sprintf("move %s to %s", plural.Count(len(w.tasks), "task"), j.toName))
```

`internal/ops/undo.go:393`（Task 11 の文言の `%d tasks` だけを替える）:

```go
			return newErr(KindConflict, "undo.blocked", fmt.Errorf("the workspace still has %s; move them with hedgehug mv --workspace, or delete them with hedgehug rm and remove them with hedgehug purge --older-than 0d --store <store>", plural.Count(n, "task")))
```

`internal/ops/workspace.go:255`:

```go
			fmt.Errorf("workspace %s still has %s; move them with hedgehug mv --workspace, or delete them with hedgehug rm and remove them with hedgehug purge --older-than 0d --store <store>", ws.Name, plural.Count(n, "task")))
```

`internal/ops/discover.go:99`:

```go
		fmt.Errorf("%d of %s could not be added: %s", len(res.Failed), plural.Count(len(cands), "store"), strings.Join(names, ", ")),
```

`internal/ops/storeops.go:662`:

```go
		return unpushed(fmt.Errorf("store %s has %s not pushed; run hedgehug sync --store %s first", e.Name, plural.Count(n, "commit"), e.Name))
```

`internal/ops/maintain.go` の `TaskCount`:

```go
// TaskCount is n tasks in words: "1 task", "3 tasks".
func TaskCount(n int) string { return plural.Count(n, "task") }
```

`internal/ops/purge.go` の `warnPurgeLeft` の2つ目:

```go
		ss.warn(plural.Count(broken, "broken forward file") + " left; see hedgehug doctor")
```

`internal/doctor/doctor.go:202`:

```go
			Msg:  fmt.Sprintf("%s %s not in the inbox yet", plural.Count(waiting, "merge driver record"), plural.Pick(waiting, "is", "are")),
```

`internal/store/validate.go:249`:

```go
			return fmt.Errorf("workspace %s still has %s", ulid, plural.Count(count[ulid], "task"))
```

`internal/store/sprint_refs.go:131,134`:

```go
			return fmt.Errorf("workspace %s still has %s", ulid, plural.Count(sprints[ulid], "sprint"))
```
```go
			return fmt.Errorf("workspace %s still has %s", ulid, plural.Count(releases[ulid], "release"))
```

`internal/sync/rebuild.go:259` と `internal/sync/sync.go:882`:

```go
	op := store.Op{Name: "replay", Detail: plural.Count(len(mine), "unpushed commit"), Author: opt.Author, Account: opt.Account, AllowEmpty: true}
```
```go
		op := store.Op{Name: "repair", Detail: plural.Count(len(repaired), "issue"), Author: opt.Author, Account: opt.Account}
```

使われなくなった `fmt` の `import` などは `go vet` が示すので消す。一覧17（`internal/ops/inbox.go`）が残っていれば次にする。

```go
		changes := plural.Count(d.Replayed, "unpushed change")
```

`cmd/hedgehug/ls.go` の `lsCmd` に `Long` を足す（`Short` の次の行）。

```go
		Long: "List the tasks that match the query.\n\n" +
			"A term that starts with - (a negation, such as -status:done) is read as a\n" +
			"flag; put it after --: hedgehug ls -- -status:done",
```

- [ ] **Step 5: 成功を確かめる**

Run: `go build ./... && go vet ./... && go test ./cmd/hedgehug ./internal/ops ./internal/sync ./internal/store ./internal/doctor ./internal/plural -count=1`（timeout 600000）
Expected: PASS。golden の差が出たら、`go test ./cmd/hedgehug -update` で作り直す前に、差が表の変更だけであることを `git diff cmd/hedgehug/testdata` で確かめる。

- [ ] **Step 6: 複数形の取りこぼしがないことを確かめる**

Run: `grep -rnE '%d (tasks|commits|merges|problems|items|files|stores|sprints|releases|issues|changes|records)|%d unpushed|%d broken' --include='*.go' cmd internal | grep -v _test.go`
Expected: 出力なし（一覧の「直さないもの」は `matches %d …` の形で、この grep には掛からない）。

- [ ] **Step 7: コミットする**

```bash
git add -A cmd internal
git commit -m "$(cat <<'EOF'
fix(cli): 英語の複数形を揃え、ls --help に - で始まる項の書き方を足す

数を入れる文言を internal/plural の Count・Pick に通し、1 のとき単数にする
（1 merge、1 commit not pushed、1 task now has problems、Move 1 task など）。
同期・doctor・store・sync にあった同じ誤りも直す。ls --help に、- で始まる項は
-- の後ろに置くことを書く。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

#### spec にない細部

1. **複数形の関数は、新しい小さなパッケージ `internal/plural`（`Count`・`Pick`）に1つだけ置く。** spec は `cmd/hedgehug` と `internal/ops` に `plural(n int, one string) string` を置く想定だが、同じ誤りが `internal/doctor`（`1 merge driver records`）・`internal/store`（`still has 1 tasks`）・`internal/sync`（コミットの件名 `repair 1 issues`）にもあり、これらは `ops` を import できない（`ops` が import している）。重複させると5か所に同じ関数ができるので、葉のパッケージにした。名前は spec の例の `plural` ではなく `plural.Count`（呼び出しが `plural.Count(2, "task")` と読める）。**spec は cmd と ops だけを対象にしているので、doctor・store・sync まで広げる点の承認が要る。** 広げずに cmd と ops だけにするなら、一覧の 18〜22 を外す。
2. **`Pick` は動詞や代名詞の一致のために足した。** `1 task now has problems; find it with …`・`1 merge driver record is not in the inbox yet` のため。名詞に s を付けるだけでは文が合わない箇所が2つあった（一覧の 3 と 18）。
3. **名詞は `s` を付けるだけで複数形になるものだけを渡す。** 渡す名詞は `task`・`merge`・`commit`・`problem`・`item`・`store`・`sprint`・`release`・`issue`・`forward file`・`broken forward file`・`unpushed commit`・`unpushed change`・`merge driver record` で、不規則なものはない。
4. **0 は複数形（`0 commits not pushed`、`0 merges`）。** 英語の普通の扱いで、今の出力と変わらない。
5. **`ops.TaskCount` は残して `plural.Count` に任せる。** 呼び出し（`migrate` の出力、`migrate.invalid` のエラー）を変えないため。
6. **曖昧さのエラー（`matches %d tasks`）は直さない。** 候補が2以上のときだけ出るので、複数形が正しい。
7. **`sync` のコミットの件名（`hedgehug: repair 1 issue`・`replay 1 unpushed commit`）も直す。** `hh log` で利用者が読む文言のため。件名を読んで分岐するコードはない（`grep -rn 'unpushed commits\|issues' --include='*.go' --include='*.ts'` で確かめた）。
8. **`sprint_workspace_test.go:109` は `what` の末尾の `s` を取って検査する。** テストの `what` が複数形の名詞（`sprints`・`releases`）で、メッセージの名詞は単数になるため。

### Task 13: 既存の spec・Artifact の更新、受入テスト手順の通し、全テスト

controller（このセッション）が行う。サブエージェントに渡さない（Artifact の更新と、利用者に見せる確認があるため）。

**Files:**
- Modify: `docs/superpowers/specs/2026-10-30-phase1d1-cli-design.md`（`comment`、`purge <id>`・`--moved`・`--rewrite-history`、`show`、`add --no-repo`、`.gitconfig` の作者）
- Modify: `docs/superpowers/specs/2026-10-30-phase1d2-maintenance-design.md`（`comment <id>`、`.gitconfig` の作者）
- Modify: `docs/superpowers/specs/2026-10-31-phase1-5-workspace-keys-design.md`（`workspace prefix`）
- Modify: `docs/superpowers/specs/2026-10-31-phase2-sync-merge-design.md`（`workspace prefix`、`--rewrite-history`、`store remove`、コミットの作者）
- Modify: `docs/superpowers/specs/2026-11-01-phase3-stores-accounts-design.md`・`2026-11-01-phase3-design-review.md`（`--no-repo`、`purge --moved`、`--rewrite-history`、`store remove --purge`、`builtinClientID`、アカウントなしの動き）
- （phase4 の spec は Task 9 で直し済み）
- Artifact: CLI リファレンス https://claude.ai/artifact/Arnn3ZSK2uuCD16fC1XBGN、ローカル受入テスト手順 https://claude.ai/artifact/PwxHewn4T7H1i94o9r9U8b

**Interfaces:**
- Consumes: Task 1〜12 の変更後のコマンドと出力

- [ ] **Step 1: 既存の spec に変更の節を足す**

各ファイルの冒頭（タイトルと前書きの直後）に次の節を足し、本文の該当箇所の行末に `（2026-11-02 に変更。冒頭の節を参照）` を付ける。本文の記述は消さない（その時点の決定の記録として残す）。

```markdown
## 2026-11-02 の変更

CLI の受入テストで、この spec の次の点を変えた。詳細は `docs/superpowers/specs/2026-11-02-cli-acceptance-fixes-design.md`。

- <このファイルに関係する項目だけを、spec の節番号付きで1行ずつ>
```

該当箇所は `grep -nF` で探す: `comment <id>`、`workspace prefix`、`no-repo`、`purge --moved`、`purge <id>`、`moved_to`、`転送ファイル`、`rewrite-history`、`rewrites.yaml`、`store remove`、`--purge`、`builtinClientID`、`client ID`、`user.name`、`.gitconfig`、`アカウントのない`、`account がない`。

- [ ] **Step 2: spec のコミット**

```bash
git add docs/superpowers/specs
git commit -m "docs(spec): 受入テストでの変更を既存の spec に記録する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 3: バイナリを作り、ローカル受入テスト手順を最初から最後まで通す**

`make build` で `bin/hedgehug` を作り、手順の Artifact（`Artifact` の read で取り出す）の各ステップを変更後のコマンドで実行する。手順の書き換え: config.yaml の手書きを `hh auth add me --login alice`（2 人目は `--login bob`）に、`.gitconfig` を作る手順は消す、`comment <id>` → `comment add <id>`、`workspace prefix` → `workspace set-prefix`、`purge <id>` → `rm` のあと `purge --older-than 0d --store <store> --yes`、`store remove --purge` → `store remove`（`--keep` の例も1つ）、`add` で repos を付ける例は `--repo` を付ける。各ステップの実際の出力を手順の期待値に写す。

- [ ] **Step 4: Artifact を更新する**

CLI リファレンスと受入テスト手順の HTML を、Step 3 の結果に合わせて直し、同じ URL に publish する（`url` を渡す）。CLI リファレンスには `auth add`、`purge --older-than`、`store remove --keep`、`add --repo`、`show` の新しい出力を入れ、`purge --moved`・`--rewrite-history`・`--no-repo`・`--purge` を消す。

- [ ] **Step 5: 全テスト**

Run: `make test`（timeout 600000）。web は `make web-check`。E2E は `make e2e`（Task 9 で GUI の完全削除を外したため）。
Expected: すべて PASS。落ちたらその場で直してコミットする。

- [ ] **Step 6: 記録とコミット**

plan の末尾に「## 実装で plan から変えた点」を足し、各 Task で plan から変えた点を書く。

```bash
git add docs/superpowers/plans/2026-11-02-cli-acceptance-fixes.md
git commit -m "docs(plan): CLI の受入テストの修正で plan から変えた点を記録する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

#### spec にない細部

1. 既存の spec は本文を書き換えず、冒頭に「2026-11-02 の変更」の節を足し、該当箇所に印を付ける（spec の「後片付け」は「節を足すか、記述を直す」のどちらでもよいとしている。決定の経緯を残すため前者にする）


## 実装で plan から変えた点

### Task 10（PR は Task 10 だけ。ほかの Task はそのマージ後）

- `Swap` は古い Store を最初から `.kept-` に動かさず、入れ替えの間は `.before-rewrite-` で動かす。戻せなかった変更（`Purged`）があるときは、最初の rename の前に古い Store の `.hedgehug/local/rebuild-keep` に印を書き、入れ替えが済んでから `<dir>.kept-<UTC 時刻>` に改名する。`FinishLeftover` は印のある `.before-rewrite-` を消さずに `.kept-` に改名し、改名できなければ残してそのパスをエラーで返す（レビューの指摘: `.kept-` へ直接 rename すると、止まった入れ替えの仕上げが古い Store を見落とし、間に入ったコミットが本流から外れうる）。
- 入れ替えのあとの `syncLocal` の失敗は、作り直しの失敗の指摘にせず、`Rebuild.Warnings` の警告にする。
- テストの細部: `internal/ops/rebuild_test.go` の指摘の数は `merge_driver` の指摘を除いて数える（`hedgehug` が PATH にない環境のため）。`internal/index/forward_merge_test.go` は消した `ResetMain` の代わりに go-git の HardReset を使う。`api/openapi.yaml` と `web/core/api/schema.d.ts` を作り直した。

### Task 9・11（PR は Task 9・11 だけ。Task 1〜8 より先に入れた）

- Task 1〜8 より先に入れたので、テストは今の環境に合わせた（`TestUndoDoesNotTakeBackAPurge` は `writeAccounts` を自分で呼ぶ）。挙動は plan のまま。
- Task 9: phase4 の spec の完全削除の記述は、表にあった箇所のほかに 3 か所（4c の細部、E2E の行、Dialogs の追記）も消した。`internal/server/poll_test.go` の `PurgeUnchecked` は `Purge(0, true)` にした。
- Task 11: `cmd/hedgehug` にあった同名の `plural` 関数は消して `plural.Pick` にした。JSON の golden の `<store>` は `<store>` になる。
- Task 11: `--yes` でロックの下で計算し直したときに新しく「残す」ことになったタスクは、一覧のときに出していない ID だけを警告する（spec §9 の「残して警告で ID を並べる」）。壊れた転送ファイルは数が増えたときだけ警告する。
- Task 11: GUI の `undo.blocked` は拒否の理由（`op`: `purge`・`move-in`・`move-out`、なければワークスペース）で文言を分ける。
- Task 10 の後回しの Minor 5 件もこの回で直した（`unexpected_force_push` を 1 つにまとめる、doc コメントの折り返し、`openStoreAt` のハンドルをすべての経路で閉じる、`TestPurgeRewriteHistoryIsGone` を purge のテストに移す、`Purged` のない入れ替えは古い `rebuild-keep` の印を先に消す）。
- phase3 の spec と design-review に「2026-11-02 の変更」の節を足し、転送ファイルの `moved_at` と `purge --moved` の廃止を書いた。Task 13 Step 1 の探す語に `moved_to`・`転送ファイル` を足した。
