# 構成見直し 回 4: 入口の境界 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** cli と server が、テストを含めて storage・remote・lib/gitrepo・lib/github を import せず、そのフィールドやメソッドにも触らず、usecase（直下と領域）・domain・lib/plural・lib/osfs・web とテスト用の部品だけで動くようにする。CLI の `--json` を server と同じ usecase の型（包みを含む）にし、server の Poller の中身を `usecase/watch` に移し、確認の部品を `cli/prompt` にまとめ、cli・server の depguard を有効にする。

**Architecture:** usecase の公開 API を「cli・server が storage の型を見ずに済む形」に直し（表示用の View 型、書き込みの包みの型、エラーの文言、設定の取り出し）、cli・server をその上に書き直す。順は、土台 → 領域 → 読み取り → 書き込み → 監視 → 確認 → 本体の残り → テスト → lint。各 Task の終わりで build・vet と、その Task が触ったパッケージのテストが通る。全テストは Task 11 で 1 回だけ回す。

**Tech Stack:** Go、huma v2（`api/openapi.yaml` の生成）、pnpm（`web/core/api/schema.d.ts` の生成）、golangci-lint 2.15.0（depguard）、golang.org/x/tools（`tools/restructure/uses`・`leaks`）。

**Spec:** `docs/superpowers/specs/2026-11-04-codebase-restructure-design.md`（4.2〜4.4 節、決定 7・11〜15、6 節の PR 4）。概要 plan: `docs/superpowers/plans/2026-11-04-codebase-restructure.md`

## Global Constraints

- 概要 plan の Global Constraints をすべて守る
- 挙動は変えない。変えてよいのは spec 4.3 の `--json` の形だけ（下の「変わる JSON の一覧」）。テキストの出力・終了コード・stderr の文言・コミットの中身は変えない
- 形が変わる `--json` は、新しい形を期待するテストを先に書き、落ちることを見てから本体を変える
- テストの中身の変更は、import・型の名前の書き換え、上の JSON の期待の書き換え、決定 11 の準備と確認の書き換え（確かめる内容は変えない）、下の「この回で決めたこと」の 9 にあるものだけにする
- server の応答の形が変わる Task では、同じコミットで `go generate ./internal/app/server`（`api/openapi.yaml`）と `cd web && pnpm gen`（`web/core/api/schema.d.ts`）を走らせて生成物を入れる。`TestOpenAPIUpToDate` と `make web-check` が通ること
- 各 Task の終わりに `go build ./...` と `go vet ./...` を通し、Task に書いたテストを回す。`go test ./...` 全体と `-race` は Task 11 で 1 回
- テストが落ちたら、失敗の出力と実行条件（コマンド、`-race` の有無）を報告に残す。推測で直さない
- spec・この plan と違う挙動が要るとわかったら、止めて報告する（自分で決めない）
- `docs/superpowers/specs/` と `docs/superpowers/plans/` の既存の文書に出てくる旧名は書き換えない

## 変わる JSON の一覧（spec 4.3。2026-11-04 利用者が「包みまで server に揃える」と決定）

| コマンド | 今 | 回 4 の後（usecase の型） |
| --- | --- | --- |
| add・edit・set・done・status・dep・watch・unwatch・inbox revert・mv・restore --at | 裸の TaskView（mv は TaskView に detached を平らに足した形） | `usecase.TaskWrite` `{task, commit, note?, detached?}`。task は checklist・progress 付き |
| rm・restore | TaskView の配列 | `usecase.AffectedWrite` `{tasks, commit}`（指定したタスクが先頭） |
| comment add・edit・rm | `{ulid, task, author, created, updated, deleted_at, body}` | `comments.CommentWrite` `{comment:{ulid, task, author, created, updated?, deleted_at?, body}, commit}` |
| view save・rm・rename | 裸の SavedView | `views.ViewWrite` `{view, commit}` |
| workspace add・rename・prefix・rm | 裸の WorkspaceView | `workspaces.WorkspaceWrite` `{workspace, commit, ids?}` |
| schema migrate | `{changed, skipped}`（changed は checklist・progress なし） | `schemas.MigrateWrite` `{changed, skipped, commit}` |
| schema check | Finding の配列 | `schemas.CheckResult` `{findings}` |
| ls | TaskView の配列 | `browse.TaskList` `{tasks, groups?}`（tasks は checklist・progress 付き） |
| ls --tree | roots と cycle を混ぜた配列 | `browse.TaskList` `{roots, cycle?}` |
| undo | `{undone: "<hash>", subject, changed, kept}` | `tasks.UndoWrite` `{undone: LogView\|null, subject, changed, commit, kept}` |
| auth login | uses のときはテキスト | `accounts.AuthLoginResult` に `uses?`。uses のときも JSON |
| auth logout | `{account, removed}` | `accounts.AuthLogoutResult` `{account, id, token?, removed}` |
| store add | `{name, path, remotes}` | `stores.StoreView`（store list の 1 行と同じ） |
| store list | StoreView の配列 | `stores.StoreItem` の配列（`default` が付く） |
| store init | `{name, path, workspace:{ulid,name,id_prefix}, remotes?, pushed?}` | `stores.StoreInitView`（workspace は WorkspaceView、remotes・pushed は常に配列） |
| store default | `{default: name\|null}` | `stores.DefaultView` `{name: name\|null}` |
| store discover | `{candidates, mounted, failed?[{name, error}]}` | `{candidates, mounted, failed[{name, code, message}]}`（mounted と failed は `stores.MountView`） |
| inbox ls | InboxItemView の配列 | `notifications.Notification` の配列（`read` が付く） |
| sync --status | StoreStatus の配列 | `usecase.SyncStatusView` の配列（`interval`・`push_interval` が付く。`next_fetch`・`push_at` は server のときだけ） |

同じまま（テストと golden で確かめる）: doctor・show・auth add・auth status・log・store remove・store set・sync・view ls・schema scope・workspace ls・purge・inbox read・mv --store・use。

## この回で決めたこと（plan を書くときに決めた。spec にない細部。利用者の承認を取る）

1. **`usecase/watch` は状態を持つ型 `watch.Watcher` にする。** 決定 13（領域の操作は `*usecase.Session` を引数に取る関数）は「Session を包む型を作らない」という決まりで、Watcher は Session を包まない（Poll のたびに Session を作って閉じる）。Store を開いたまま次の Poll まで持つ部品なので、関数にすると状態を呼び手に渡すことになり、非公開のフィールドが漏れる
2. **領域どうしの import は作らない（spec 2.3）。** watch が使う同期の状態（`StoreStatus`・`SyncStatus`）と同期の間隔は土台に、`TaskOfPath` は `model` に下ろす。未読の数は watch に関数で渡す。undo（tasks）と log（browse）が使う `LogViewOf` は土台に下ろす。`stores.StoreInitView` は `workspaces.WorkspaceView` を使うので、stores → workspaces の import を 1 本だけ足す（循環しない）
3. **テキストの表示にだけ使う項目は usecase の型に `json:"-"` で持たせる**（spec 4.3 の最後の項）。`TaskView.DisplayID`・`ParentRef`・`FormerDisplayIDs`・`PropList`、`Ref.DisplayID`・`Missing`、`WorkspaceRef.Known`、`ShowView.CreatedBy`・`Relations`、`LogView.CurrentSubject`、`UndoKept.Label`、`Detached.DisplayID`、`GroupView.Tasks`、`browse.ListView.Many` など
4. **エラーの文言は土台の 1 か所にする。** cli の `describe` と server の `detailOf` は同じ処理なので `usecase.Describe(err)` に、違反の取り出しは `usecase.ViolationsOf(err)`、`store.ErrLocked` は `var ErrLocked = store.ErrLocked`（sentinel の再公開。型の別名ではない）、problem の type は `usecase.ProblemCode(err)`。`schemas.ViolationsText(*store.ValidationError)` は `usecase.ViolationsText([]schema.Violation)` にする
5. **書き込みの後の処理。** `Session.AfterWrite` を `func(storeName string)` にし、CLI のバックグラウンド同期の起動は `Session.BackgroundSync bool` を立てて usecase の中（`MarkWritten`）で行う。`auth.Keyring` は `usecase.Keyring`（同じメソッドの interface）で受ける
6. **`ls --format` のキーは `ls --json` の tasks の 1 行と同じになる**（checklist・progress が増える）。テンプレートが使えるキーが増えるだけで、今通るテンプレートの出力は変わらない。`ls --tree --format` は今どおり context・in_cycle を明示して入れ、children は入れない
7. **確認の部品（spec 4.4）**: `cli/prompt` に `Confirm`（はい・いいえ）と `Choose`（番号で選ぶ・語の頭文字で選ぶ）を置く。端末かどうかの判定も部品が持つ（`Asker.Interactive()`）。今の 7 つの文言・既定の答え・エラーの文言（`canceled` と `Canceled` の違いを含む）はそのまま。**ひとつだけ挙動が揃う**: 「Open the editor again? [Y/n]」は今 `y`・`Y`・空だけをはいとし、`yes` をいいえにしている。部品にすると `yes`・`YES` もはいになる（他の 3 つの [Y/n]・[y/N] と同じ）
8. **server のテストのうち、Poller の中を見るものは `usecase/watch` のテストに移す**（`package watch_test`、非公開の名前は `export_test.go` の別名）。イベントの配信と JSON の形を見るテストは server に残す。`TestTestServerWaitsForTheHistoryBuild` は watch の `Wait` のテストになり、server 側の同名のテストはなくなる。`TestSessionBeforeSwapReleasesTheStore` は `*polled` の同一性を見られなくなるので、「開き直した look の tasks.changed が来る」ことで確かめる。この 2 つが決定 11（確かめる内容は変えない）の例外
9. **テストの準備の補助**: 決定 11 の書き直しのための書き込みの補助（store の Tx を domain の型だけで包んだ `usecasetest.Tx`、`Edit`、アクセス・メンバー・inbox の項目を書く補助）は `usecasetest` に、コミットと bare の補助（`Bare`・`CommitFiles`・`Log`・`HeadHasFile`・`HeadHash`）は `gitrepotest` に置く。usecase 本体には足さない。補助のシグネチャは domain・usecase の型と標準の型だけで書く。`usecasetest.Edit` は store の `Update` を直接呼ぶ（`Session.Update` は作者を書き手のアカウントで上書きするので、今のテストの作者 T・Alice と変わる）。server の testServer は `*usecasetest.Session` を埋め込まず、非公開のフィールドで持つ（埋め込むと `ts.Open()` などが storage の型を返して見えてしまう）

## 調べてわかっていること（main 6e2b895）

- cli・server が触っている禁止対象の名前は、`tools/restructure/uses` で数える（フィールドとメソッドを含む。テストのファイルも含む）。回 4 の初めは 457 行（本体: cli 22 ファイル・server 12 ファイル。テスト: cli 34 ファイル・server 21 ファイル。テスト用の部品の名前を含む）。リポジトリのルートで:

  ```bash
  go -C tools/restructure run ./uses "$PWD" app/storage/,app/remote/,lib/gitrepo,lib/github ./internal/app/cli/... ./internal/app/server/... \
    | grep -vE '	(gitrepotest|historytest|mounttest|testenv|fakegh|usecasetest)\.'
  ```

  Task 8 の後に本体（`_test.go` 以外）が 0 行、Task 10 の後にすべて 0 行になる
- usecase の公開 API に禁止対象の型が出ているものは `tools/restructure/leaks` で見られる（引数は uses と同じ並び。回 4 の初めは 130 行）。usecase の中だけで使うもの（`Session.Config`・`Entry`・`Open` など、領域が使うもの）は残ってよい。cli・server がそれを使わないことは uses で確かめる
- cli・server の本体が呼んでいる Session のメソッドで storage の型が出るのは `Config`（cli/app.go:104、server/loop.go・syncapi.go・poll.go）、`Entry`（server/stores.go:149・admin.go:256）、`SyncTargets`（server/loop.go・poll.go）、`OpenStore`（server/loop.go:218・poll.go）、`Open`・`Resolve`（server/comments.go・admin.go・tasks_write.go・activity.go）だけ
- usecase の領域どうしは本番のコードで 1 本も import していない。server/poll.go だけが replication（`SyncStatus`・`StoreStatus`・`TaskOfPath`）と notifications（`InboxList`）を使う
- huma は Go の型名から schema の名前を作る（パッケージ名は付かない。名前がぶつかると panic）。`server.go` の `schemaName` が例外を持つ。web のソースが名前で参照している schema は 20 個で、この回で名前が変わる schema（`AccountLoginBody`→`AuthLoginResult`、`FindingsOutputBody`→`CheckResult`、`MigrateOutputBody`→`MigrateWrite`、`StoreInitOutputBody`→`StoreInitView`、`StoreDefaultOutputBody`→`DefaultView`、`NotificationReadOutputBody`→`ReadResult`、`MountOutputBody`→`MountView`、`ListOutputBody`→`TaskList`、`IdChange`→`IDChange`）は含まない。`SyncStatus` は web が使う（web/core/connection.ts:8）ので、`schemaName` で `usecase.SyncStatusView` を `SyncStatus` と呼ばせる
- web/app/undo.ts:13 は `r.undone.id`・`r.undone.op` を読む。`undone` が null を許すようになるので `r.undone?.id` に直す
- `printJSON` は `SetIndent("", "  ")` で行末に改行を付ける（cli/output.go:44）
- depguard は今は無効（`.golangci.yaml` の linters の注記）。ルール無しで有効にすると標準ライブラリ以外を拒否するので、ルールと一緒に有効にする

---

### Task 1: 土台 — エラーの文言、Keyring、書き込みの後、設定の取り出し、同期の状態

**Files:**
- Modify: `internal/app/usecase/errors.go`（`Describe`・`ViolationsOf`・`ViolationsText`・`ProblemCode`・`ErrLocked`）
- Create: `internal/app/usecase/keyring.go`
- Modify: `internal/app/usecase/session.go`（`Keyring` の型、`AfterWrite`、`BackgroundSync`、`ForStore` の複製、`MarkWritten`、`AccountCount`・`DataDir`・`CurrentStore`・`TaskULID`・`SyncIntervals`）
- Modify: `internal/app/usecase/sync_shared.go`（replication から `StoreStatus`・`SyncStatus` を下ろす。`SyncIntervals`・`SyncSchedule`・`SyncStatusView`・`NewSyncStatusView`）
- Modify: `internal/app/domain/model/`（`TaskOfPath` を `ParseCommentPath` の隣へ。`usecase/replication/sync.go:150` から移す）
- Modify: `internal/app/usecase/schemas/schemacheck.go`（`ViolationsText` を消して土台を呼ぶ）
- Modify（呼び出し側）: `internal/app/cli/app.go`・`run.go`・`ui.go`・`edit.go`、`internal/app/server/server.go`・`problem.go`・`stores.go`・`comments.go`・`syncapi.go`・`admin.go`（list-stores の既定は Task 5 まで今のまま）、`internal/app/usecase/replication/*.go`（`SyncStatus` の呼び出し）、usecase のテスト 3 か所の `AfterWrite`（session_test.go:17、session_more_test.go:110、maintenance/maintain_test.go:112）と `TestViolationsText`
- Test: `internal/app/usecase/errors_test.go`（Describe・ViolationsOf・ProblemCode）、`session_test.go`（BackgroundSync）

**Interfaces（Produces）:**

```go
// package usecase — errors.go
var ErrLocked = store.ErrLocked
// ViolationsOf は、書き込みがスキーマに拒まれたときの違反。そうでなければ ok=false。
func ViolationsOf(err error) (vs []schema.Violation, ok bool)
// ViolationsText は拒まれた書き込みの文言（今の schemas.ViolationsText の本体）。
func ViolationsText(vs []schema.Violation) string
// Describe は CLI が "hedgehug: " の後に出す文で、server の Problem.Detail と同じ。
// 違反 → ViolationsText、ErrLocked → "another hedgehug is writing to the store; try again"、他は err.Error()
// （今の cli/app.go:148 の describe と server/problem.go:97 の detailOf をそのまま移す）
func Describe(err error) string
// ProblemCode は problem の type。AsError(err) の Code、なければ "internal"
func ProblemCode(err error) string

// keyring.go
type Keyring interface { // auth.Keyring と同じメソッド集合
	Get(service, user string) (string, error)
	Set(service, user, secret string) error
	Delete(service, user string) error
}

// session.go
//   Keyring        Keyring             // nil は OS のキーチェーン
//   AfterWrite     func(storeName string) // コミットの後。nil なら何もしない
//   BackgroundSync bool                // true: リモートのある store に書いたらバックグラウンドの同期を始める（CLI）
func (ss *Session) AccountCount() (int, error)          // config.Load のエラーはそのまま
func (ss *Session) DataDir() (string, error)            // config.Resolve(ss.Getenv) の DataDir
type StoreRef struct{ Name, Path string }
func (ss *Session) CurrentStore() (StoreRef, error)     // Entry() の Name・Path
func (ss *Session) TaskULID(id string) (model.ULID, error) // Open + Resolve（server/comments.go:52 の taskULID）
type SyncIntervals struct{ Interval, PushInterval time.Duration }
func (ss *Session) SyncIntervals() (SyncIntervals, error) // ss.Config() の sync.* だけ（SyncTargets は呼ばない）

// sync_shared.go（replication から下ろす。中身は変えない）
type StoreStatus struct{ /* 今の replication.StoreStatus */ }
func SyncStatus(ss *Session) ([]StoreStatus, error)
// SyncSchedule は同期のループの予定。server の *Loop が満たす。
type SyncSchedule interface {
	NextFetch(store string) (time.Time, bool)
	PushAt(store string, pending time.Time, interval, push time.Duration) (time.Time, bool)
	FetchInterval(interval time.Duration) time.Duration
}
// SyncStatusView は今の server.syncStatus（json・doc タグもそのまま。next_fetch・push_at の doc に
// "; absent when no server runs the loop" を足す）
type SyncStatusView struct {
	StoreStatus
	NextFetch    *time.Time `json:"next_fetch,omitempty" doc:"..."`
	PushAt       *time.Time `json:"push_at,omitempty" doc:"..."`
	Interval     float64    `json:"interval" doc:"sync.interval of config.yaml, in seconds (fractions allowed)"`
	PushInterval float64    `json:"push_interval" doc:"sync.push_interval of config.yaml, in seconds (fractions allowed)"`
}
// NewSyncStatusView は今の server の newSyncStatus と同じ計算。sch が nil なら NextFetch・PushAt は nil。
func NewSyncStatusView(st StoreStatus, sch SyncSchedule, iv SyncIntervals) SyncStatusView

// package model
func TaskOfPath(p string) (ULID, bool)
```

`MarkWritten`（session.go:552）は次の順にする（今の CLI の AfterWrite と同じ順・同じ文言）:

```go
func (ss *Session) MarkWritten(s *store.Store) {
	if err := replicate.MarkPending(s, ss.Now()); err != nil {
		ss.AddWarning("the write is not marked for sync: " + err.Error())
		return
	}
	_, e, err := ss.Entry()
	if err != nil {
		return
	}
	if ss.BackgroundSync && len(e.Remotes) > 0 { // replication.SyncsInBackground と同じ条件
		if err := replicate.Start(s, e.Name, ss.Getenv); err != nil {
			ss.AddWarning("the background sync did not start: " + err.Error())
		}
	}
	if ss.AfterWrite != nil {
		ss.AfterWrite(e.Name)
	}
}
```

（今の MarkWritten の先頭の「AfterWrite が nil なら return」の位置を、実装の前に読んで確かめる。上と違う順になっていたら止めて報告する。）

- [ ] **Step 1:** `errors_test.go` に `Describe`（違反あり・ErrLocked・ほか）、`ViolationsOf`、`ProblemCode` のテストを書き、`go test ./internal/app/usecase/ -run 'Describe|ViolationsOf|ProblemCode'` が未定義で落ちることを見る
- [ ] **Step 2:** errors.go・keyring.go・session.go・sync_shared.go・model を書く。`schemas.ViolationsText` を消し、`schemacheck.go:105` は `usecase.ViolationsText(ve.Violations)`。replication の `StoreStatus`・`SyncStatus`・`TaskOfPath` を消し、replication の中の呼び出しを `usecase.`・`model.` に替える。`ForStore`（session.go:100）の複製に `BackgroundSync` を足す
- [ ] **Step 3:** cli を書き換える: app.go の `describe` は `usecase.Describe(err)` の 1 行、`beforeRun` のアカウントの数は `a.ss.AccountCount()`（`beforeCommand` の cfg の引数は Task 2 で消す。ここでは `a.ss.Config()` を残してよい）、edit.go:54 は `usecase.ViolationsOf`、run.go は `AfterWrite` をやめて `BackgroundSync: true`、`env.keyring` は `usecase.Keyring`、ui.go は `a.ss.DataDir()`
- [ ] **Step 4:** server を書き換える: problem.go は `usecase.ViolationsOf`・`usecase.Describe`・`usecase.ErrLocked`（`detailOf` を消す。呼び出しは `usecase.Describe`）、server.go の `Options.Keyring` は `usecase.Keyring`、`AfterWrite: func(string) { s.loop.Wake() }`、stores.go:149 は `ss.CurrentStore()`、comments.go の taskULID は `ss.TaskULID(id)`、syncapi.go は `ss.SyncIntervals()` と `usecase.NewSyncStatusView(st, l, iv)`（`syncStatus` を `usecase.SyncStatusView` に置き換え、`newSyncStatus` を消す）。loop.go の `fetchInterval(c *config.Config)` を `FetchInterval(interval time.Duration) time.Duration` にし、`*Loop` が `usecase.SyncSchedule` を満たすようにする（`NextFetch`・`PushAt` の今の名前と引数を読んで合わせる。合わなければメソッドの名前を合わせて、呼び出し側を直す）。`schemaName`（server.go:109）に、`usecase.SyncStatusView` を `SyncStatus` と呼ぶ 1 行を足す
- [ ] **Step 5:** usecase のテストの `AfterWrite` を `func(string)` に、`TestViolationsText` を土台の関数に直す。session_test.go に「`BackgroundSync` が false ならリモートがあっても起動しない、true なら 1 回起動する」を `usecasetest.StubSpawn`（Task 2 で足す。ここでは `replicate.SetSpawn` を usecase のテストから直接使ってよい）で書く
- [ ] **Step 6:** `go build ./... && go vet ./...`。`go generate ./internal/app/server` の後 `git diff --exit-code api/openapi.yaml`（差が出ないこと。出たら schema 名か doc の差なので中身を確かめ、`SyncStatus` の doc の 1 行以外なら止めて報告）。`cd web && pnpm gen && git diff --exit-code core/api/schema.d.ts`（同上）
- [ ] **Step 7:** `go test ./internal/app/usecase/... ./internal/app/cli/ ./internal/app/server/ -run 'Describe|Violations|ProblemCode|Session|Background|AccountRequired|Edit|Problem|Locked|Sync|Loop|PushAt|Comment|StoreDelete|UI'`。cli の background_test.go（書き込みで同期が 1 回だけ起動する）・account_required_test.go・edit_test.go、server の server_test.go の 409、loop_test.go の `TestLoopPushesAfterWrite`、syncapi_test.go が通ること
- [ ] **Step 8:** コミット `refactor(usecase): エラーの文言・設定の取り出し・同期の状態を土台に置き、cli と server から使う`

### Task 2: replication と accounts — merge driver、コマンドの前の同期、同期のループ

**Files:**
- Create: `internal/app/usecase/replication/mergedriver.go`（cli/merge_driver.go の本体: `driverSchema`・`mergedSchema`・`recordLosses`・`mergedIn` ごと）
- Create: `internal/app/usecase/replication/schedule.go`
- Modify: `internal/app/usecase/replication/sync.go`（`BeforeCommand` の引数）
- Modify: `internal/app/usecase/accounts/auth.go`（`AuthLoginInput.ShowCode` を消す）
- Modify: `internal/app/usecase/usecasetest/session.go`（`BeforeCommand`、`StubSpawn`）
- Modify: `internal/app/cli/merge_driver.go`・`app.go`・`auth.go`、`internal/app/server/loop.go`・`syncapi.go`
- Test: `internal/app/usecase/replication/sync_test.go:126-167`（起動の数え方を `usecasetest.StubSpawn` に）、新規 `schedule_test.go`

**Interfaces:**
- Consumes: Task 1 の `usecase.SyncIntervals`・`Session.SyncIntervals`
- Produces:

```go
// package replication
type MergeDriverInput struct {
	Dir                      string   // 作業ツリー（os.Getwd）
	Base, Ours, Theirs, Path string   // git が渡す 3 つのファイルと store の中の相対パス
	Environ                  []string // os.Environ()。GITHEAD_* を探す
}
// MergeDriver は 1 つのファイルをマージして Ours に書く。負けた値は .hedgehug/local/overwritten/ に残す。
// 時刻は ss.Now()（今の a.now().UTC() と同じ値。UTC にするのも中で行う）
func MergeDriver(ss *usecase.Session, in MergeDriverInput) error

// BeforeCommand は中で ss.Config() を読み直し、バックグラウンドの同期は replicate.Start で始める
func BeforeCommand(ss *usecase.Session, report func(store, msg string))

type Schedule struct {
	usecase.SyncIntervals
	Stores []string // バックグラウンドで同期する store（リモートのあるもの）の名前
}
// LoadSchedule は server の Loop.targets の置き換え（Config → SyncTargets(c, true) → SyncsInBackground）。エラーは今のまま
func LoadSchedule(ss *usecase.Session) (Schedule, error)
// PendingSince は push していない最初の書き込みの時刻。なし・読めないときは zero（Loop.pendingSince と同じ）
func PendingSince(ss *usecase.Session) time.Time
// SyncOne は Loop.syncStore の置き換え（Config → Select → SyncStore → AddDuplicateWarnings）。
// err は Config・Select・SyncStore のエラー、同期の失敗は rep.Err
func SyncOne(ss *usecase.Session) (rep usecase.SyncReport, err error)

// package usecasetest
func StubSpawn(f func(args []string)) (restore func()) // replicate.SetSpawn を包む
```

- [ ] **Step 1:** `schedule_test.go` に `LoadSchedule`（リモートのない store を除く、store がないとき今と同じエラー）と `PendingSince`（書いた後は非 zero、push の後は zero）のテストを書き、未定義で落ちることを見る
- [ ] **Step 2:** schedule.go・mergedriver.go を書き、`BeforeCommand` の `cfg`・`start` の引数を消す。sync_test.go:126-167 の 3 つのテストは `start` の差し替えの代わりに `usecasetest.StubSpawn` で数える（確かめる回数は変えない）。usecasetest の `ts.BeforeCommand` を新しい引数に合わせる
- [ ] **Step 3:** cli: merge_driver.go は cobra の定義と、`os.Getwd()`・`os.Environ()` を `replication.MergeDriver(a.ss, ...)` に渡す数行だけにする。app.go の `beforeCommand(cmd, cfg)` は `replication.BeforeCommand(a.ss, report)` に（cfg の引数を消し、`a.ss.Config()` の残りも消す）。auth.go の `ShowCode: a.showDeviceCode` は `OnCode`、`showDeviceCode` は `accounts.DeviceCode` を受ける（accounts/auth.go:17-24 に既にある）
- [ ] **Step 4:** server/loop.go: `targets()` は `replication.LoadSchedule`、`noteNext` は `ss.SyncIntervals()`（`LoadSchedule` にしない。store が 0 件のとき ErrNoStores のログが増えるため）、`pendingSince(name string)` は `replication.PendingSince(l.s.session(name))`、`syncStore` は `replication.SyncOne`（ログは `logf("sync %s: %s", name, usecase.Describe(err))` で今と同じ文字列）、`due` と Drain は名前を取る。syncapi.go は Task 1 のまま
- [ ] **Step 5:** `go build ./... && go vet ./...`。`go test ./internal/app/usecase/replication/ ./internal/app/usecase/accounts/ ./internal/app/cli/ ./internal/app/server/ -run 'Merge|Schedule|Pending|BeforeCommand|Background|Meanwhile|Auth|Device|Loop|Sync|PushAt|Drain'`。cli の merge_driver_test.go・merge_driver_more_test.go（git pull を含む）が通ること
- [ ] **Step 6:** コミット `refactor(replication): merge driver と同期のループの読み取りを usecase に移す`

### Task 3: 読み取りの表示と JSON — show・ls・tree・log

**Files:**
- Modify: `internal/app/usecase/view.go`（`TaskView`・`Ref`・`WorkspaceRef` の表示用の項目、`ListViewOf`、`Ref.Name`、`TaskView.Label`）
- Modify: `internal/app/usecase/read_shared.go`（`ShowView`・`LogView`・`CommentView`、`LogViewOf` を browse から下ろす）
- Modify: `internal/app/usecase/browse/read.go`（`Show`・`List`・`Tree`・`Log`・`LogIn` の戻り）
- Modify: `internal/app/cli/show.go`・`ls.go`・`tree.go`・`log.go`・`output.go`（`printTasks`・`taskRow`）・`view.go`（`viewShow`）
- Modify: `internal/app/server/tasks.go`（list・tree・show・history）、`activity.go`（activity の絞り込み）
- Test: cli の `ls_test.go`（`TestLs`）、`ls_stores_test.go`（`TestLsAcrossStores`）、`tree_test.go`（`TestLsTree`・`TestLsTreeCycleNested`・`TestLsTreeProgress`）の JSON の期待、server の tasks のテストの型名

**Interfaces:**

```go
// package usecase — view.go（既存の項目と json タグはそのまま）
type TaskView struct {
	// ...既存...
	DisplayID        string     `json:"-"` // ix.DisplayID(e)
	ParentRef        *Ref       `json:"-"` // e.Parent があれば RefOf(ix, *e.Parent)
	FormerDisplayIDs []string   `json:"-"` // ix.QualifyID(各 FormerIDs)
	PropList         []PropView `json:"-"` // e.Props の順
}
type PropView struct {
	Key    string
	Values []string // taskref でないとき
	Refs   []Ref    // taskref のとき（e.Targets(Key) を RefOf）
	IsRef  bool     // schema の型が TypeTaskRef
}
// Ref に足す: DisplayID string `json:"-"`、Missing bool `json:"-"`（index にない）。RefOf が詰める
func (r Ref) Name() string        // 今の RefName と同じ文字列
func (v *TaskView) Label() string // 今の TaskLabel と同じ文字列
// WorkspaceRef に足す: Known bool `json:"-"`（ix.Workspace で見つかった。名前が空のワークスペースがあるので Name で判定しない）
// ListViewOf は TaskViewOf に Checklist と Progress を足した一覧の 1 行（server/tasks.go:23 の listView を移す）
func ListViewOf(ix *index.Index, e *index.Entry) *TaskView

// read_shared.go
// ShowView に足す: CreatedBy string `json:"-"`、Relations []RelationView `json:"-"`
type RelationView struct {
	Key, Label string
	Tasks      []Ref
}
// LogView に足す: CurrentSubject string `json:"-"`（今の browse.SubjectNow）
func (l LogView) Trailer(key string) (string, bool)
// LogViewOf は browse の logViewOf を下ろしたもの（tasks の undo も使う）
func LogViewOf(ix *index.Index, e gitrepo.LogEntry) LogView

// package browse
func Show(ss *usecase.Session, arg string) (*usecase.ShowView, error) // ShowViewOf を中に畳む
type GroupView struct {
	Key   string              `json:"key" doc:"The value of the group; empty for the tasks with none"`
	ULIDs []string            `json:"ulids"`
	Tasks []*usecase.TaskView `json:"-"`
}
// TaskList は GET /api/tasks と hh ls --json の形（server の listOutput の Body を移す。タグはそのまま）
type TaskList struct {
	Tasks  *[]*usecase.TaskView `json:"tasks,omitempty" doc:"The tasks found, without tree"`
	Groups []GroupView          `json:"groups,omitempty" doc:"The groups, when the query has group:"`
	Roots  *[]*usecase.TaskView `json:"roots,omitempty" doc:"The roots of the tree, with tree"`
	Cycle  []*usecase.TaskView  `json:"cycle,omitempty" doc:"The tasks in a parent cycle, with tree"`
}
type ListView struct {
	Tasks   []*usecase.TaskView // ListViewOf。グループの順
	Groups  []GroupView         // group: のときだけ
	Many    bool                // 2 つ以上の store がマウントされている
}
func List(ss *usecase.Session, query string) (ListView, error) // 中で ParseQuery
func (v ListView) JSON() TaskList
type TreeStoreView struct {
	Store        string
	Roots, Cycle []*usecase.TaskView // 子は Children。Context・InCycle・Checklist・Progress 付き（今の TreeViewOf）
}
type TreeView struct {
	Stores []TreeStoreView
	Many   bool
}
func Tree(ss *usecase.Session, query string) (TreeView, error) // group・sort の使用エラーは parse の直後（今の read.go:97）
func (s TreeStoreView) Drawn() []*usecase.TaskView // 今の Flatten(roots+cycle)
func (t TreeView) JSON() TaskList                  // Roots は空でも []、Cycle は全 store をつなぐ
type LogResult struct {
	Logs []usecase.LogView // Store・CurrentSubject 入り
	Many bool
}
func Log(ss *usecase.Session, args []string, limit int) (LogResult, error)
func LogIn(ss *usecase.Session, args, workspaces []string, limit int) (LogResult, error)
```

消すもの: `browse.ShowViewOf`・`TreeViewOf`・`Flatten`・`LogViewsOf`・`StoreLogViews`・`StoreLog`・`SubjectNow`（非公開にする）・`Entry`・`ListResult`・`TreeStore`・`TreeResult`・`ShowResult`・`Relation`、`usecase.HasTerm` の cli・server からの呼び出し、server の `listView`・`groupView`・list の無名の Body。

- [ ] **Step 1:** cli のテストで、`ls --json` を `{tasks: [...]}`（行に checklist・progress）、`ls --json 'group:status'` を `{tasks, groups}`、`ls --tree --json` を `{roots, cycle?}` として読む期待に書き換える（`TestLs`・`TestLsAcrossStores`・`TestLsTree`・`TestLsTreeCycleNested`・`TestLsTreeProgress`。ほかに `ls --json` の出力を配列として読むテストがあれば、`grep -n 'ls.*--json\|"--json".*"ls"' internal/app/cli/*_test.go` で洗い出して同じく直す）。`go test ./internal/app/cli/ -run 'TestLs'` が落ちることを見る
- [ ] **Step 2:** usecase の型と browse の関数を書く。server/tasks.go は `browse.List(ss, in.Q)` の `JSON()`・`browse.Tree(...)` の `JSON()`・`browse.Show`・`browse.Log` を使い、`e.Store.Index()`・`ParseQuery`・`HasTerm` を消す。activity.go の絞り込みは `l.Trailer(...)` で書き、`StoreLogViews` をやめる
- [ ] **Step 3:** cli: `show.go` は `showText(v *usecase.ShowView)`（下の表）。`printTasks(vs []*usecase.TaskView, withStore bool)`・`taskRow(v)`（id は `v.DisplayID`、空なら `?`。`v.Broken != ""` なら kind と status は `?`）。`ls.go` はクエリの文字列を `browse.List` に渡し、JSON は `res.JSON()`、`--format` は `json.Marshal(v)` を map にする（決めたこと 6）。`tree.go` は `browse.Tree`、`--format` の map には `context`・`in_cycle` を明示して入れ、`children` は消す。`log.go` は `LogView` から行を作る（`l.Hash[:7]`、`l.Date.In(time.Local)`、`l.Author.Name`、`l.CurrentSubject`、many なら `l.Store` の幅で揃える）。`view.go` の `viewShow` はクエリの文字列を渡す

  show のテキストの対応（今の show.go:39-158 と 1 バイトも変えない）: title→`v.Title`、id→`v.DisplayID`、ulid→`v.ULID`、store→`v.Store`、workspace→`v.Workspace`（`Known` なら `Name (IDPrefix, ULID)`、でなければ ULID）、former_ids→`v.FormerDisplayIDs`、checklist・progress→`usecase.ChecklistText(v.Checklist)`・`usecase.ProgressFractionText(v.Progress)`、parent→`v.ParentRef.Name()`（nil なら `-`）、watchers・repos→`v.Broken == ""` のときだけ、blocked_by→`v.BlockedBy[i].Name()`、props→`v.PropList`（`IsRef` なら `Refs[i].Name()`）、created→`v.Created` と `v.CreatedBy`（空は `-`）、updated・deleted、Problems→`v.Problems`、Related→`v.Relations`、Referenced by→`v.ReferencedBy[i].Name()`、broken、body→`v.Body`（`v.Broken == ""` のとき）、Comments、History→`logText(v.History)`
- [ ] **Step 4:** `go generate ./internal/app/server`、`cd web && pnpm gen`。openapi.yaml の差が `ListOutputBody`→`TaskList` の改名と doc だけであることを見る
- [ ] **Step 5:** `go build ./... && go vet ./...`。`go test ./internal/app/usecase/... ./internal/app/cli/ ./internal/app/server/ -run 'Show|Ls|Tree|Log|History|Activity|List|View|Golden|Prefix|Format|Display|Workspace|OpenAPI'`。golden（`internal/app/cli/testdata/`）が 1 つも変わらないこと（`git status internal/app/cli/testdata` が空）
- [ ] **Step 6:** コミット `refactor(browse): 読み取りの結果を View の型で返し、ls と tree の --json を server の形にする`

### Task 4: 書き込みの結果と包み — tasks・comments・views・workspaces・schemas・undo

**Files:**
- Create: `internal/app/usecase/write_view.go`（`CommitPtr`・`TaskWrite`・`AffectedWrite`・`TaskResult.Write`・`MoveResult.Write`・`IDChange`）
- Modify: `internal/app/usecase/task_shared.go`（`TaskResult.View` を `ListViewOf` に、`Index` を非公開に）、`move_tx.go`（`Renumbered`、`Detached.DisplayID`）
- Modify: `internal/app/usecase/tasks/`（delete.go の `AffectedResult.Write`・`RestoreAtResult.Write`、edit.go の `EditSession.FileName`・`Check`・`Unchanged`、addhere.go の `AddHereResult`・`BranchQuestion`、undo.go の `UndoResult`・`UndoKept`・`UndoWrite`）
- Modify: `internal/app/usecase/comments/comment.go`（`CommentWrite`、`CommentResult.Write`・`CommentULID`、`CommentSession.FileName`、`CommentEditSession.CommentULID`・`Body`・`Result`）、`read_shared.go` の `CommentView`
- Modify: `internal/app/usecase/views/`（`ViewWrite`、`ViewResult.View` と `ViewGet` を `SavedView` の値に）、`workspaces/`（`WorkspaceWrite`、`WorkspaceResult.View`・`Write`・`Renumbering`、`WorkspaceList` の戻り）、`schemas/`（`MigrateWrite`・`Migrate` の戻り、`CheckResult`・`CheckResultOf`）
- Modify: cli の `output.go`（`printTask`・`printAffected`）・`add.go`・`edit.go`・`set.go`・`status.go`・`dep.go`・`watch.go`・`inbox.go`（revert）・`mv.go`（--store なし）・`restore_at.go`・`rm.go`・`comment.go`・`view.go`・`workspace.go`・`migrate.go`・`schema.go`・`undo.go`
- Modify: server の `tasks_write.go`・`comments.go`・`views.go`・`admin.go`（workspace・migrate・schema check）・`activity.go`（undo・inbox revert）
- Modify: `web/app/undo.ts:13`（`r.undone?.id`・`r.undone?.op`）と `web/app/undo.test.ts`
- Test: cli の JSON の期待（下の Step 1 の表）、usecase の新しい小さなテスト

**Interfaces:**

```go
// package usecase — write_view.go（server の taskWrite・affectedWrite・commitPtr を移す。タグはそのまま）
func CommitPtr(hex string) *string
type TaskWrite struct {
	Task     *TaskView  `json:"task"`
	Commit   *string    `json:"commit" doc:"The commit made; null when nothing changed"`
	Note     string     `json:"note,omitempty" doc:"..."`
	Detached []Detached `json:"detached,omitempty" doc:"..."`
}
type AffectedWrite struct {
	Tasks  []*TaskView `json:"tasks,omitempty" doc:"The task and the tasks under it, as they are now"`
	Commit *string     `json:"commit"`
}
type IDChange struct {
	Old string `json:"old"`
	New string `json:"new"`
}
func (r TaskResult) View() (*TaskView, error)  // Refresh してから ListViewOf
func (r TaskResult) Write() (TaskWrite, error)
func (r MoveResult) Write() (TaskWrite, error) // Detached 付き
func (r MoveResult) Renumbered() []IDChange    // QualifyID 済み（mv.go:84-86 の行）
// Detached に DisplayID string `json:"-"`

// package tasks
func (r AffectedResult) Views() ([]*usecase.TaskView, error) // ListViewOf。[0] が指定したタスク、重複なし（server の affectedOut の順）
func (r AffectedResult) Write() (usecase.AffectedWrite, error)
func (r RestoreAtResult) Write() (usecase.TaskWrite, error) // Note を詰める
func (es *EditSession) FileName() string
func (es *EditSession) Check(edited []byte) (*model.Task, error)
func (es *EditSession) Unchanged() usecase.TaskResult
type BranchQuestion struct{ ParentID, ParentTitle string } // ParentID は ID が空なら ULID（今の shownID）
type AddHereResult struct {
	usecase.TaskResult
	Destination string // "<store> / <親のタイトル または ワークスペース名>"（今の destinationLabel）
}
func AddHere(ss *usecase.Session, in AddHereInput) (AddHereResult, *BranchQuestion, *usecase.Choice, error)
type UndoKept struct {
	Path   string `json:"path"`
	Task   string `json:"task,omitempty" doc:"The ID of the task; empty for other files"`
	Label  string `json:"-"`
	Field  string `json:"field"`
	Author string `json:"author"`
}
type UndoWrite struct {
	Undone  *usecase.LogView `json:"undone" doc:"The commit taken back; null when there was nothing to undo"`
	Subject string           `json:"subject" doc:"The subject of the commit taken back, without the hedgehug: prefix"`
	Changed bool             `json:"changed" doc:"false when every field was changed later by others, so nothing was written"`
	Commit  *string          `json:"commit"`
	Kept    []UndoKept       `json:"kept" doc:"What others changed after it, which undo kept"`
}
// UndoResult は gitrepo.LogEntry を公開しない: Undone *usecase.LogView（commit の後に Refresh した index で LogViewOf）
func (r UndoResult) Write() UndoWrite // Kept は nil なら []
func UndoSubject(subject string) string

// package comments
type CommentWrite struct {
	Comment usecase.CommentView `json:"comment" doc:"The comment as written"`
	Commit  *string             `json:"commit" doc:"The commit made; null when nothing changed"`
}
func (r CommentResult) Write() CommentWrite
func (r CommentResult) CommentULID() string
// usecase.CommentView に足す: Task string `json:"task,omitempty"`、Updated *time.Time `json:"updated,omitempty"`、
// DeletedAt *time.Time `json:"deleted_at,omitempty"`（show は詰めない）

// package views
type ViewWrite struct {
	View   SavedView `json:"view"`
	Commit *string   `json:"commit"`
}
func (r ViewResult) Write() ViewWrite
func ViewGet(ss *usecase.Session, name string) (SavedView, error)

// package workspaces
type WorkspaceWrite struct {
	Workspace WorkspaceView      `json:"workspace"`
	Commit    *string            `json:"commit"`
	IDs       []usecase.IDChange `json:"ids,omitempty" doc:"With a new prefix: the IDs the tasks had and have now"`
}
func (r WorkspaceResult) View() WorkspaceView
func (r WorkspaceResult) Write() WorkspaceWrite
func (r WorkspaceResult) Renumbering() []usecase.IDChange
func WorkspaceList(ss *usecase.Session) ([]WorkspaceView, error)

// package schemas
type CheckResult struct {
	Findings []usecase.Finding `json:"findings"`
}
func CheckResultOf(fs []usecase.Finding) CheckResult // nil は []
type MigrateWrite struct {
	Changed []*usecase.TaskView `json:"changed"`
	Skipped []string            `json:"skipped"`
	Commit  *string             `json:"commit"`
}
func Migrate(ss *usecase.Session, m schema.Migration, dryRun bool) (MigrateWrite, error) // dry-run の changed も中で作る（今の admin.go:229-241）
```

`--json` のときの CLI の出し方: printTask は `r.Write()`、printAffected は `r.Write()`、comment は `r.Write()`、view は `r.Write()`、workspace は `r.Write()`、migrate は `Migrate` の戻り、schema check は `CheckResultOf(fs)`、undo は `res.Write()`（取り消すものがないときは `UndoResult{}.Write()` で `{undone:null, subject:"", changed:false, commit:null, kept:[]}`。終了コードは今のまま 0）、mv は常に `res.Write()`。restore --at の note は、JSON のときは `note` に入り、テキストのときだけ今どおり出す（今の出し先を読んで同じにする）。テキストの出力は今と同じ（`v.Label()`・`v.Title`、`res.Renumbered()`、`d.DisplayID`、`r.Renumbering()`、`r.Changed[i].Label()`）。comment のテキストは `r.CommentULID()` を使い、JSON のときだけ `Write()` を呼ぶ。

- [ ] **Step 1:** 新しい形を期待するテストを先に書く: `internal/app/cli/json_types_test.go` の `TestJSONIsTheUsecaseTypes` で、各コマンドの `--json` を `json.Decoder.DisallowUnknownFields()` で usecase の型に読む（add・set・edit・done・dep・watch・unwatch・inbox revert・mv・restore --at → `usecase.TaskWrite`、rm・restore → `usecase.AffectedWrite`、comment add/edit/rm → `comments.CommentWrite`、view save/rm/rename → `views.ViewWrite`、workspace add/rename/prefix/rm → `workspaces.WorkspaceWrite`、schema migrate → `schemas.MigrateWrite`、schema check → `schemas.CheckResult`、undo → `tasks.UndoWrite`。型はまだないので、このステップではテストのファイルに同じ形の構造体を一時に置き、Step 3 で usecase の型に差し替える）。既存のテストの JSON の期待を新しい形に書き換える: add_golden_test.go（`TestAddContextGolden`・`TestAddJSONAtTheRootHasNullParent`）、add_test.go、edit_test.go（`TestEdit`）、restore_at_test.go、rm_test.go（`TestRmJSON`）、undo_test.go（`TestUndoJSON` ほか）、undo_more_test.go、undo_kept_test.go、mv_sprint_json_test.go、comment_test.go、watch_test.go、view_test.go（`TestViewCommands`）、workspace_test.go（`TestWorkspaceJSON`）、migrate_test.go、schema_test.go（`TestSchemaCheck*`）。ほかに `--json` の出力を読むテスト（`add --json` から ulid を取るものなど）は `grep -ln 'json.Unmarshal' internal/app/cli/*_test.go` で洗い出し、`.task.ulid` などに直す。golden のうち形が変わるものは、期待を手で書き換える（`-update` で作り直さない）。`go test ./internal/app/cli/ -run 'JSON|Add|Edit|Restore|Rm|Undo|Mv|Comment|Watch|View|Workspace|Migrate|Schema'` が落ちることを見る
- [ ] **Step 2:** usecase の型と関数を書く。usecase に `UndoWrite{}`（取り消すものがない）、`AffectedWrite` の並び（指定したタスクが先頭、重複なし）、`CheckResultOf(nil)` の `[]` のテストを足す
- [ ] **Step 3:** cli と server を書き換える（上の出し方）。server から `taskWrite`・`affectedWrite`・`commentWrite`・`workspaceWrite`・`viewWrite`・`undoWrite`・`undoKept`・`idChange`・`commitPtr`・`listView`・`viewsOf`・`writeOut`・`affectedOut`・`commentOut`・`viewOut`・`workspaceOut`・migrate と schema check の無名の Body を消し、Output の Body を usecase の型にする。server の undo が `undo.nothing` で problem を返すのは今のまま。server のテストの型名を書き換える。Step 1 の一時の構造体を usecase の型に差し替える
- [ ] **Step 4:** `go generate ./internal/app/server`、web/app/undo.ts と undo.test.ts を直し、`cd web && pnpm gen && pnpm check && pnpm test`
- [ ] **Step 5:** `go build ./... && go vet ./...`。`go test ./internal/app/usecase/... ./internal/app/cli/ ./internal/app/server/`（この 3 つは全部。-race なし）
- [ ] **Step 6:** コミット `refactor(usecase): 書き込みの応答の包みを usecase に移し、CLI の --json も同じ形で出す`

### Task 5: stores・accounts・notifications・同期の状態の JSON

**Files:**
- Modify: `internal/app/usecase/stores/`（`StoreItem`・`StoreItems`・`ViewOf`・`StoreInitView`・`StoreInitViewOf`・`DefaultView`・`DefaultViewOf`・`MountFailed`・`MountView`・`MountResult.View`）
- Modify: `internal/app/usecase/accounts/auth.go`（`AuthLoginResult.Uses` の json タグ）
- Modify: `internal/app/usecase/notifications/inbox.go`（`InboxItemView` の埋め込みをやめて平らに、`Notification`・`InboxNotifications`・`ReadResult`）
- Modify: `internal/app/usecase/sync_shared.go`（`SyncStatusViews`）
- Modify: cli の `store_add.go`・`store_init.go`・`store_default.go`・`store_discover.go`・`auth.go`・`inbox.go`・`sync.go`、server の `admin.go`・`stores.go`・`accounts.go`・`activity.go`・`syncapi.go`
- Test: 下の Step 1

**Interfaces:**

```go
// package stores（server の storeItem・storeViewOf・storeInitOutput・storeDefault の Body・mountFailed を移す）
type StoreItem struct {
	StoreView
	Default bool `json:"default" doc:"The store hedgehug picks without --store"`
}
func StoreItems(ss *usecase.Session) ([]StoreItem, error)
func ViewOf(ss *usecase.Session, name string) (StoreView, error)
type StoreInitView struct {
	Name      string                   `json:"name"`
	Path      string                   `json:"path"`
	Workspace workspaces.WorkspaceView `json:"workspace"`
	Remotes   []string                 `json:"remotes"`
	Pushed    []string                 `json:"pushed"`
}
func StoreInitViewOf(ss *usecase.Session, r StoreInitResult) StoreInitView // PrefixSet も中で読む（admin.go:277-287）
type DefaultView struct {
	Name *string `json:"name" doc:"The default store; null when there is none"`
}
func DefaultViewOf(name string) DefaultView
type MountFailed struct {
	Name    string `json:"name"`
	Code    string `json:"code" doc:"What went wrong, as the type of a problem"`
	Message string `json:"message" doc:"The message in English, as the CLI prints it"`
}
type MountView struct {
	Mounted []string      `json:"mounted" doc:"The names of the stores that were added"`
	Failed  []MountFailed `json:"failed" doc:"The candidates that could not be added; the others were"`
}
func (r MountResult) View() MountView // Code は usecase.ProblemCode、Message は usecase.Describe

// package accounts: AuthLoginResult.Uses を `json:"uses,omitempty" doc:"gh or env:VAR: ..."`（server の accountLoginBody の doc）

// package notifications
// InboxItemView は inbox.Item を埋め込まず、今の JSON と同じ項目・同じ順・同じタグで持つ（ULID・Kind は string）
type Notification struct {
	InboxItemView
	Read bool `json:"read"`
}
func InboxNotifications(ss *usecase.Session, all bool) ([]Notification, error) // activity.go:157-188 を移す。警告の重複を除くのも中で
type ReadResult struct {
	Read []string `json:"read" nullable:"false" doc:"The ULIDs of the items marked read, as hh inbox read --json"`
}

// package usecase
func SyncStatusViews(ss *Session) ([]SyncStatusView, error) // SyncStatus + SyncIntervals、sch は nil
```

CLI: store add は `stores.ViewOf(a.ss, r.Name)`、store list は `StoreItems`（テキストは今のまま）、store init は `StoreInitViewOf(a.ss.ForStore(r.Name), r)`（今の Session の作り方を読んで server と同じにする）、store default は `DefaultViewOf(def)`、store discover は cli の中で `struct{ Candidates []stores.Candidate `json:"candidates"`; stores.MountView }`、auth login は JSON の判定を `Uses` の分岐より前に、auth logout は `AuthLogoutResult` をそのまま、inbox ls は `InboxNotifications`、inbox read は `ReadResult`、sync --status は `SyncStatusViews`。

- [ ] **Step 1:** `TestJSONIsTheUsecaseTypes` に auth login・logout、store add・list・init・default・discover、inbox ls・read、sync --status、mv --store（`stores.MoveStoreResultView`）を足し、既存の期待を書き換える: auth_test.go（`TestAuthJSON`）、e2e_3a_test.go（`TestE2EAuthLoginAndStatus`）、store_add_test.go（`TestStoreAddSetsUp`・`TestStoreAddPath`・`TestStoreListJSON`）、store_more_test.go（`TestStoreInitRemoteJSON`・`TestStoreListAccountIsID`・`TestSyncStatusSkipsBrokenStore` と `testdata/TestSyncStatusSkipsBrokenStore/status-json.golden`）、store_default_test.go、store_init_test.go、store_discover_test.go（`TestDiscover*`）、inbox_test.go（`TestInboxJSON` ほか）、sync_test.go（`TestSyncStatus`）、workspace_test.go（`TestStoreInitJSONHasTheWorkspace`）。inbox read・mv --store・inbox の `InboxItemView` の部分は今と同じバイトであることを golden で確かめる。落ちることを見る
- [ ] **Step 2:** usecase の型を書き、cli と server を書き換える。server から `storeItem`・`storeViewOf`・`mountFailed`・`notification`・`accountLoginBody`・`loginBody`・store init・default・mount・notification read の無名の Body を消す。usecase に `StoreItems`（既定の store に `default: true`）・`StoreInitViewOf`（PrefixSet）のテストを足す
- [ ] **Step 3:** `go generate ./internal/app/server`、`cd web && pnpm gen && pnpm check && pnpm test`
- [ ] **Step 4:** `go build ./... && go vet ./...`。`go test ./internal/app/usecase/... ./internal/app/cli/ ./internal/app/server/`
- [ ] **Step 5:** コミット `refactor(usecase): store・アカウント・通知・同期の状態の応答を usecase に移し、CLI の --json も同じ形で出す`

### Task 6: usecase/watch — server の Poller の中身を移す

**Files:**
- Create: `internal/app/usecase/watch/watch.go`（server/poll.go の `polled`・`newPolled`・`stopHistory`・`pollLocked`・`headMoved`・`publishDiff`・`splitMax`・`childTask`・`addOnce`・`refresh`・`buildHistory`・`historyRounds`・`openHistory`・`configSnapshot`・`Release`・`ReleaseFor`・`releaseLocked`・`waitBuilds`・`snapshot`・`hasLooked`）、`export_test.go`
- Modify: `internal/app/server/poll.go`（薄い Poller）、`events.go`
- Move: server の poll のテスト（下）を `internal/app/usecase/watch/` へ
- Modify: `internal/app/usecase/usecasetest/`（watch のテストが要る補助があれば）

**Interfaces:**
- Consumes: Task 1 の `usecase.StoreStatus`・`SyncStatus`・`SyncSchedule`・`SyncStatusView`・`NewSyncStatusView`・`SyncIntervals`、`model.TaskOfPath`
- Produces:

```go
package watch

// Change は tasks.changed の中身。server の tasksChanged と同じ項目で、server は型の変換で包む
type Change struct {
	Store   string
	ULIDs   []string // 空でも非 nil
	Removed []string // 空でも非 nil
	Commit  string
	Op      string
	Author  string
}
type StoreState struct {
	Store string
	Head  string
	Sync  usecase.SyncStatusView
}
// Sink は見つけたことの渡し先。呼ぶ順は今の poll と同じ。どのメソッドも Watcher を呼び返してはならない
// （Unread は Watcher の mu を持ったまま呼ぶ）
type Sink interface {
	StoresChanged()
	TasksChanged(Change)
	SchemaChanged(store string)
	ViewsChanged(store string)
	SyncState(usecase.SyncStatusView)
	Unread(count int)
}
type Options struct {
	Session  func(storeName string) *usecase.Session // server.session
	Schedule usecase.SyncSchedule                    // *Loop
	Sink     Sink
	Unread   func(*usecase.Session) (int, error) // server は notifications.InboxList の件数を返す関数を渡す
	Logf     func(format string, args ...any)
}
type Watcher struct{ /* 今の Poller の pollMu・mu・stores・cfg・unread・looked・scanned・lastFail・released・restored */ }
func New(o Options) *Watcher
func (w *Watcher) Poll(publish bool)                // 今の Poller.poll
func (w *Watcher) Snapshot() ([]StoreState, int)    // 今の snapshot（!hasLooked の二重の確かめを含む）
func (w *Watcher) Release(path string)
func (w *Watcher) ReleaseFor(path string, fn func() error) error
func (w *Watcher) Wait()                            // 今の waitBuilds
```

server に残すもの: `Hub`、間隔のループ（`pollInterval`・`kick`・`Run`）、イベントの JSON の形（`tasksChanged`・`unreadEvent`・`schemaChanged`・`viewsChanged`・`storesChanged`・`storeState`・`helloEvent`）、`hubSink`（Sink の各メソッドで `hub.Publish(...)` するだけ）、`Server.session` の `BeforeSwap` からの `ReleaseFor`・`Kick`。server の Poller は `w *watch.Watcher`・`hub`・`kick`・`interval` だけを持ち、`poll(publish)`・`waitBuilds()`・`snapshot()` はテストのために名前を残して Watcher に渡す。

挙動を変えないために守ること:
1. 本体はほぼそのまま移し、`p.hub.Publish(x)` を `w.sink.X(...)` に 1 対 1 で置き換えるだけにする。順は、config の変化の StoresChanged → store ごとの新規・入れ替わりの TasksChanged → headMoved の TasksChanged・SchemaChanged・ViewsChanged → SyncState → ループの後の Unread
2. publish の条件の式は 1 文字も変えない（`(swapped || scanned) && !restored && publish`、`publish && (!fresh || announced)`、`len(ev.ULIDs) > 0 || len(ev.Removed) > 0 || (!schemaCh && !viewsCh)`、`len(items) != unread`）
3. 開き直した look は、Store を `w.stores` に保つ前に publish する（`TestStoreDeleteRefusedIsQuiet` が固定している）。だから Sink はコールバックで、イベントを slice で返さない
4. `histDone.Wait()` は mu を持たずに行う
5. SyncState の比較は今どおり `reflect.DeepEqual`（`SyncStatusView` 同士）

テストの行き先: watch へ（`package watch_test`、記録用の Sink をテストに置く）— poll_test.go の `TestPollTasksChanged`・`TestPollTellsEachCommitOfALook`・`TestPollTellsManyCommitsAsOne`・`TestPollCommentChangesItsTask`・`TestPollRemoved`・`TestPollSchemaAndViews`・`TestPollUnread`・`TestPollFirstLookSendsNothing`・`TestPollInboxStampIsStable`・`TestChildTaskIsCommentsOnly`・`TestPollNewStoreTellsTheClients`・`TestPollStoreThatComesBackTellsTheClients`・`TestPollFirstStoreAfterNoStoreTellsTheClients`・`TestPollFailedFirstLookIsNotTheFirstLook`、poll_moved_test.go の `TestPollMovedTaskIsRemoved`、poll_history_test.go の `TestPollBuildsTheHistoryIndex`・`TestReleaseCancelsTheHistoryBuild`・`TestTestServerWaitsForTheHistoryBuild`（`Wait` のテストとして）、poll_stores_test.go の `TestPollStoresChanged`・`TestPollerReleaseForKeepsPollsOut`。server に残す — `TestPollSyncState`、poll_release_test.go の `TestSessionBeforeSwapReleasesTheStore`（決めたこと 8）、stores_write_test.go の `TestStoreDeleteAPI`・`TestStoreDeleteRefusedIsQuiet`、events_test.go、scoped_test.go。移したテストの期待（イベントの中身・数・順）は書き換えない。受け取り方だけを Sink に替える。

- [ ] **Step 1:** watch.go を書き、poll.go の本体を移す。server の Poller を薄くし、`newPoller` で `watch.New(watch.Options{Session: s.session, Schedule: s.loop, Sink: hubSink{hub}, Unread: ..., Logf: s.logf})` を作る（`s.loop` は server.go で `newPoller` より前に作られていることを確かめる）
- [ ] **Step 2:** テストを上のとおり移す。`export_test.go` に `SplitMax`・`ChildTask`・`OpenHistory` の差し替え口など、移したテストが使う非公開の名前の別名を置く
- [ ] **Step 3:** `go build ./... && go vet ./...`。`go test -race ./internal/app/usecase/watch/ ./internal/app/server/`（-race はこの Task だけ例外で回す。並行の部品を動かしたため）。`go list -deps ./internal/app/server | grep -E 'app/(storage|remote)/|lib/(gitrepo|github)$'` に poll.go 由来の行が残らないこと（他の Task の残りはあってよい。`go -C tools/restructure run ./uses ...` で poll.go の行が 0 になったことを見る）
- [ ] **Step 4:** コミット `refactor(watch): server の Poller の中身を usecase/watch に移す`

### Task 7: cli/prompt — 確認と選択の部品（spec 4.4）

**Files:**
- Create: `internal/app/cli/prompt/prompt.go`、`prompt_test.go`
- Modify: `internal/app/cli/add.go`（`askBranchParent`・`askPick`）、`editor.go`（`askRetry`）、`store_init.go`（`askChoice`・`choice`）、`mv.go`（`askScoped`・`confirmMove`）、`purge.go`（`confirmPurge`）、`store_discover.go`・`run.go`（`a.ask` を作る）

**Interfaces:**

```go
package prompt

// Asker は確認と選択を端末で行う。Out は stderr、ReadLine は 1 行を読む（今の a.readLine）。
type Asker struct {
	Out      io.Writer
	ReadLine func() (string, error)
	Terminal bool // stdin が端末か（今の a.isTerminal(a.stdin)）
}
func (a Asker) Interactive() bool

// YesNo は はい・いいえ の確認。
type YesNo struct {
	Question string // そのまま出す（"[Y/n] " などを含む）
	Default  bool   // 空の答え
	Canceled string // 入力が終わったときのエラーの文言（"canceled" か "Canceled"）。"" なら false を返してエラーにしない（askRetry）
	Strict   bool   // y・yes・n・no 以外を usage エラー（"%q is not y or n"）にする（askBranchParent）。false なら Default の逆
	Refuse   string // 答えがいいえのときに返すエラーの文言（confirmMove・confirmPurge の "Canceled"）。"" ならエラーにしない
}
func (a Asker) Confirm(q YesNo) (bool, error) // 答えは小文字にして trim。y・yes ははい、n・no はいいえ

// Choose は一覧から選ぶ。
type Choose struct {
	Question string   // 一覧の前の行。ByWord のときは語の前の文
	Options  []string // 選ぶ値
	Labels   []string // 番号のとき、値の後ろに出す
	Prompt   string   // 番号のときの入力の見出し。"" なら "Number"
	Default  string   // 番号のとき、空の答えで選ぶ値
	ByWord   bool     // 語か頭文字で選ぶ: "[c]arry / [d]rop: "、違えば "%q is not carry or drop"
	Canceled string
}
func (a Asker) Choose(c Choose) (string, error)
```

usage エラーは cli の `usageErrorf` と同じ扱い（終了コード 2）が要る。prompt は cli を import できないので、`prompt.UsageError`（`error` を満たす型）を返し、cli の終了コードの判定で `errors.As(err, *prompt.UsageError)` も usage として扱う（今の usageErrorf の型を読んで、同じ判定になるように足す）。

7 つの書き直し（文言は今のまま。`a.ask` は run.go で `prompt.Asker{Out: a.stderr, ReadLine: a.readLine, Terminal: a.isTerminal(a.stdin)}` を作るか、呼ぶたびに作る。テストが `isTerminal` を差し替えているので、差し替えが効く作り方にする）:
- askBranchParent → `Confirm(YesNo{Question: fmt.Sprintf("Add as a child of %s %s? [Y/n] ", q.ParentID, q.ParentTitle), Default: true, Canceled: "canceled", Strict: true})`
- askRetry → `if !a.ask.Interactive() { return false }`、`Confirm(YesNo{Question: "Open the editor again? [Y/n] ", Default: true})`（決めたこと 7）
- confirmMove・confirmPurge → 一覧の行を作ってから `Confirm(YesNo{Question: b.String(), Canceled: "Canceled", Refuse: "Canceled"})`
- askScoped → `Choose(Choose{Question: "The scoped schema of " + strings.Join(projects, ", ") + " — ", Options: []string{"carry", "drop"}, ByWord: true, Canceled: "Canceled"})`
- askChoice・askPick → `Choose(Choose{Question: c.question, Options: candidates, Labels: c.labels, Prompt: c.prompt, Default: c.def, Canceled: "canceled"})`
- 呼び出し側の `a.isTerminal(a.stdin)` の判定（add.go:58、edit、store_init.go:66、store_discover.go:37・49、mv.go:110、purge.go:57）は `a.ask.Interactive()` にする

- [ ] **Step 1:** prompt_test.go に、7 つの今の挙動を表で書く（入力の行・EOF → 戻り値・エラーの文言・出した文字列）。今の cli の関数で同じ表を回すテストを先に cli 側に一時に置いて通ることを確かめ、prompt に移してから落ちることを見る
- [ ] **Step 2:** prompt.go を書き、7 つを書き直す
- [ ] **Step 3:** `go build ./... && go vet ./...`。`go test ./internal/app/cli/... -run 'Prompt|Add|Edit|Retry|Mv|Purge|StoreInit|Discover|Context|Golden'`
- [ ] **Step 4:** コミット `refactor(cli): 確認と選択を cli/prompt の 2 つの部品にまとめる`

### Task 8: 本体の残り — cli と server の本体から禁止対象をなくす

**Files:**
- Modify: Task 1〜7 の後に uses で残る本体のファイル（予想: cli の `use.go`（`stores.UseResult` の `*index.Entry`）、`mv.go`（`stores.MovePlan.Tasks`・`store.Field`・`store.Target`）、`comment.go`（`store.Comment`・`CommentRef`）、`inbox.go`（`inbox.Kind`・`ULID`）、`migrate.go`・`workspace.go` の残り、server の `admin.go`・`tasks_write.go`・`activity.go`・`comments.go` の残り）と、それを直すための usecase の型
- Test: なし（既存のテストが通ることで確かめる）

- [ ] **Step 1:** uses を走らせ、`_test.go` 以外の行を一覧にする（Global の「調べてわかっていること」のコマンドに `| grep -v '_test.go'`）
- [ ] **Step 2:** 1 行ずつ、usecase の返す型（Task 3〜5 の View 型、または同じ作りで足す型）で置き換える。usecase に足す型は「表示に使う項目は `json:"-"`」（決めたこと 3）、JSON に出る項目は今の JSON と同じタグ。`stores.UseResult` は `Project *usecase.Ref` などで持ち、`use --json`（`useJSON`）の出力が変わらないことを use_test.go で確かめる。`stores.MovePlan.Tasks` は確認の表示に必要な項目だけの View 型にする（mv --store の確認の文言を変えない）
- [ ] **Step 3:** uses の `_test.go` 以外が 0 行になるまで 1〜2 を繰り返す
- [ ] **Step 4:** `go build ./... && go vet ./...`。`go test ./internal/app/usecase/... ./internal/app/cli/ ./internal/app/server/`。`go generate ./internal/app/server` と `cd web && pnpm gen` で差が出ないこと（出たら JSON の形を変えている。止めて報告）
- [ ] **Step 5:** コミット `refactor(cli,server): 本体に残る storage の型を usecase の型に置き換える`

### Task 9: cli のテストを usecase を通して書き直す（決定 11）

**Files:**
- Modify: `internal/app/usecase/usecasetest/`（新規 `edit.go`・`replica.go`・`inboxitem.go` など。補助は下）
- Modify: `internal/lib/gitrepo/gitrepotest/`（新規 `repo.go`: `Bare`・`CommitFiles`・`Log`・`HeadHasFile`・`HeadHash`）
- Modify: cli のテスト 34 ファイル（uses の `cli<TAB>*_test.go` の行）

**Interfaces（Produces）:**

```go
// package usecasetest
// Attach は getenv の指すデータディレクトリを見る Session。ストアも config も作らない（cli の testEnv と共有する）
func Attach(t *testing.T, getenv func(string) string, now func() time.Time) *Session
type Author struct{ Name, Email string }
var (
	TAuthor     = Author{"T", "t@example.com"}
	AliceAuthor = Author{"Alice", "alice@example.com"}
)
// Tx は *store.Tx を domain の型だけで包む
type Tx struct{ /* tx *store.Tx */ }
func (x *Tx) Task(u model.ULID) (*model.Task, error)
func (x *Tx) PutTask(t *model.Task) error
func (x *Tx) PutWorkspace(w *model.Workspace) error
func (x *Tx) PutSprint(s *model.Sprint) error
func (x *Tx) PutRelease(r *model.Release) error
func (x *Tx) PutComment(task model.ULID, c *model.Comment) error
func (x *Tx) PutSchema(s *schema.Schema) error
func (x *Tx) PutFile(rel string, data []byte) error
func (x *Tx) Comments(task model.ULID) ([]*model.Comment, error)
func (x *Tx) Workspaces() ([]*model.Workspace, error)
// Edit は storeName を開き、store の Update を Op{Name: op, Author: by} で呼ぶ（Session.Update は使わない。決めたこと 9）
func (ts *Session) Edit(storeName, op string, by Author, fn func(*Tx) error) error
func (ts *Session) MustEdit(storeName, op string, by Author, fn func(*Tx) error)
func (ts *Session) TaskByULID(storeName string, u model.ULID) *model.Task
func (ts *Session) WorkspaceByULID(storeName string, u model.ULID) *model.Workspace
func (ts *Session) TaskExists(storeName string, u model.ULID) bool
func (ts *Session) StoreMeta(storeName string) model.StoreMeta
func (ts *Session) SchemaOf(storeName string) *schema.Schema
func (ts *Session) TaskULIDs(storeName string) []model.ULID
func (ts *Session) StoreDir(storeName string) string
func (ts *Session) WriteCycle(storeName string, x, y model.ULID) // tree_test.go の writeCycle を移す（作者 T、time.Now、.hedgehug/local/ を足さない）
func (ts *Session) ResolveErr(arg string) error                 // ls_test の index.Open + Resolve
func (ts *Session) HistoryAtHead(storeName string)              // history_prebuilt_test の requireHistoryAtHead
func (ts *Session) LocksFree(storeName string, names ...string) bool
func (ts *Session) NoLeftover(dir string) error
func NewOldStoreWithAttributes(t *testing.T, remote string)     // store_add_test.go:68-80
type Access struct {
	ReadOnly bool
	Reason   string
	Checked  time.Time
}
const (
	ReasonGitHub      = "github_permission" // 実装の前に replicate の定数の値を読んで合わせる
	ReasonNotWritable = "not_writable"
)
func (ts *Session) WriteAccess(storeName string, a Access)
func (ts *Session) ReadAccess(storeName string) Access
type Member struct{ ID, Permission string }
func (ts *Session) WriteMembers(storeName string, ms []Member)
func (ts *Session) ReadMembers(storeName string) (ms []Member, known bool) // known=false は Collaborators が nil
func (ts *Session) AgeChecks(storeName string, checked time.Time)
func (ts *Session) Unpushed(storeName string) int
type InboxItem struct {
	Kind   string // inbox の Kind の定数の値（実装の前に storage/inbox を読んで合わせる）
	Task   model.ULID
	ID     string
	Path   string
	Commit string
	Time   time.Time
	Detail json.RawMessage
}
func (ts *Session) AddInboxItem(storeName string, it InboxItem) model.ULID
func InboxContent(ts *Session, storeName string, it InboxItem, withIndex bool) string // notifications.InboxContent を包む

// package gitrepotest
func Bare(t testing.TB, dir string)
func HeadHasFile(dir, rel string) (bool, error)
func CommitFiles(t testing.TB, dir, subject, name, email string, when time.Time, rels ...string)
func HeadHash(t testing.TB, dir string) string
type Trailer struct{ Key, Value string }
type Commit struct {
	Hash     string
	Subject  string
	Trailers []Trailer
}
func (c Commit) Trailer(key string) (string, bool)
func (c Commit) TrailerValues(key string) []string
func Log(t testing.TB, dir string, limit int) []Commit // first-parent、新しい順
```

置き換えの対応（ファイル:行は回 4 の初め。Task 1〜8 で動いていたら grep で探し直す）:
- store の直接の書き込み（`store.Open` + `s.Update(store.Op{...}, func(tx *store.Tx))`）→ `te.uc().MustEdit("me", "update", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {...})`。作者・op 名は元のまま（show_test・mv_sprint_test・mv_sprint_json_test・meanwhile_test・comment_test・prefix_stores_test・migrate_test・promised_test・final_review_test・mv_store_test・purge_test・e2e_3b_test・sync_test）。testEnv に `uc()`（初回だけ `Attach(t, te.getenv, func() time.Time { return te.now })`）を足す。meanwhile_test.go の `changeTask` などの引数の型は `func(*usecasetest.Tx) error` に
- helpers_test.go の handEdit → `gitrepotest.CommitFiles(t, dir, "hand edit", "Alice", "alice@example.com", te.now, rel)`、bare → `gitrepotest.Bare`、stubSpawn → `usecasetest.StubSpawn`。log・headCommit は go-git のまま（spec は go-git を禁じていない）
- `gitrepo.InitBare` → `gitrepotest.Bare`（readonly_test・store_discover_test・store_init_remote_test・sync_conn_test）、`gitrepo.HeadHasFile` → `gitrepotest.HeadHasFile`、raw コミット → `CommitFiles`（schema_test・purge_moved_test）、merge_driver_more_test の HEAD → `HeadHash`、e2e_3c_test の storeLog・logOf → `gitrepotest.Log`
- replicate（doctor_test・readonly_test・e2e_3a/3b/3c・comment_test・mv_store_test・sync_test）→ `WriteAccess`・`ReadAccess`・`WriteMembers`・`ReadMembers`・`AgeChecks`・`Unpushed`
- inbox（prefix_stores_test・inbox_test）→ `AddInboxItem`・`usecasetest.InboxContent`
- そのほか: add_test の `te.task` → `TaskByULID`、ls_stores_test・tree_test の writeCycle → `WriteCycle`、history_prebuilt_test → `HistoryAtHead`、background_test → `LocksFree`、store_init_test → `StoreMeta`・`SchemaOf`、doctor_rewrite_test → `NoLeftover`、ls_test → `ResolveErr`、store_add_test → `NewOldStoreWithAttributes`

書き直しで確かめる内容を変えていないことの確かめ方:
1. 補助を書いたら、書き換えの前に、補助 1 つにつき旧い書き方と同じ結果になることを一時のテスト（コミットしない）で見る（コミットの作者名・メール・件名・Hedgehug-Op・ツリーのハッシュ）。特に `Edit` の作者が `T` になること
2. 書き換えたテストのうち準備が複雑なもの（meanwhile・comment の並行の書き込み・e2e・readonly・show の移動済み）は、確かめている振る舞いを本体で 1 行壊して落ちることを見てから戻す
3. 書き換えの前後で、各テストファイルの `t.Error*`・`t.Fatal*` の数と、比べている文字列のリテラルが変わっていないことを `git diff` で見る
4. golden（`internal/app/cli/testdata/`）が 1 バイトも変わらないこと

- [ ] **Step 1:** gitrepotest と usecasetest の補助を書き、上の 1 を確かめる
- [ ] **Step 2:** helpers_test.go と、書き込みの準備を持つテスト（show・meanwhile・comment・mv_sprint ×2・prefix_stores・migrate・promised・final_review・mv_store・purge・e2e_3b・sync）を書き換える
- [ ] **Step 3:** 残り（gitrepo・replicate・inbox・その他）を書き換える。uses の `cli<TAB>*_test.go` の行が 0 になるまで
- [ ] **Step 4:** 上の 2・3・4 を確かめる
- [ ] **Step 5:** `go build ./... && go vet ./...`。`go test ./internal/app/cli/... ./internal/lib/gitrepo/... ./internal/app/usecase/usecasetest/...`
- [ ] **Step 6:** コミット `test(cli): テストの準備と確認を usecasetest と gitrepotest の補助で書く`

### Task 10: server のテストを usecase を通して書き直す（決定 11）

**Files:**
- Modify: `internal/app/usecase/usecasetest/`（`Empty`・`AccountIDs`・`AccountTokenKind`・`DeviceTokens`・`RemoveStore`・`SetRemote`・`MarkPending`・`LastSuccess`・`InboxStamp`・`EditTask`）
- Modify: server のテスト（uses の `server<TAB>*_test.go` の行。Task 6 で watch に移したものを除く）

**Interfaces（Produces）:**

```go
// package usecasetest
// Empty は HOME と HEDGEHUG_HOME を新しく作り、config.yaml に既定アカウントだけを書く。ストアは作らない
func Empty(t *testing.T, now func() time.Time) *Session
func (ts *Session) AccountIDs() []string
func (ts *Session) AccountTokenKind(name string) (kind, id string, ok bool)
func (ts *Session) DeviceTokens(k *Keyring, name string) (access, refresh string, err error)
func (ts *Session) RemoveStore(name string) error
func (ts *Session) SetRemote(storeName, remote string)
func (ts *Session) MarkPending(storeName string, at time.Time)
func (ts *Session) LastSuccess(storeName string) time.Time
func (ts *Session) InboxStamp(storeName string) string
// EditTask は Edit に Hedgehug-Task の trailer を足す（move_detached_test・movestore_test の ss.Update の置き換え。作者は AliceAuthor 相当の config.CommitAuthor(TestAccount)）
func (ts *Session) EditTask(storeName string, task model.ULID, op string, by Author, fn func(*Tx) error) error
```

- testServer は `uc *usecasetest.Session` を非公開のフィールドで持つ（埋め込まない。決めたこと 9）。`newTestServerWith` の env は `usecasetest.Empty(t, time.Now)` の `Env`。`ts.session()` は `ts.uc.Session` を返す 1 行にし、呼び出し側のうち storage の型を受け取る・返すもの（`.Config()`・`.OpenStore()`・`.Entry()`・`.Open()`・`.Update()`・`.Resolve()`）を補助に替える
- helpers_test.go の `testAccount`・`defaultAccountYAML`・`storeAccountRE`・`dropDefaultAccount` は usecasetest の同じものと同じ内容なので消して usecasetest を使う。accountID → `AccountIDs()[0]`、addRemote → `usecasetest.AddBareRemote`、config.yaml の追記 → `AppendConfig`、notifyIn・notifyMe・overwrittenItem → `AddInboxItem`・`InboxItem`、state・waitFirstSync → `LastSuccess`
- accounts_test.go の memKeyring は消して `usecasetest.NewKeyring()`（`k.has` → `k.Has`）
- server_test.go の 409 は `usecase.ErrLocked`、ValidationError のケースは usecase のエラーを作る関数があればそれで、なければ `ts.uc` で実際にスキーマに拒まれる書き込みをして作る。problem_forcepush_test.go の `replicate.ErrForcePush` は `var ErrForcePush = replicate.ErrForcePush` を usecase の土台に置いて使う
- admin_more_test の commitRaw・tasks_read_test の rawTask → `ts.uc.CommitRawFile(rel, body)`
- loop_duplicates_test・move_detached_test・movestore_test の準備 → `TaskByULID`・`MustEdit`・`EditTask`・`UlidOf`・`AddedTask`
- 確かめ方は Task 9 の 1〜3 と同じ

- [ ] **Step 1:** usecasetest の補助を書き、Task 9 の 1 のとおり確かめる
- [ ] **Step 2:** helpers_test.go と testServer を書き換える
- [ ] **Step 3:** 残りのテストを書き換える。uses の `server<TAB>*_test.go` の行が 0 になるまで
- [ ] **Step 4:** Task 9 の 2・3 のとおり確かめる
- [ ] **Step 5:** `go build ./... && go vet ./...`。`go test ./internal/app/server/... ./internal/app/usecase/...`
- [ ] **Step 6:** コミット `test(server): テストの準備と確認を usecasetest の補助で書く`

### Task 11: depguard を有効にし、全体を確かめる

**Files:**
- Modify: `.golangci.yaml`
- Modify: `docs/superpowers/plans/2026-11-04-codebase-restructure.md`（回 4 の完了の記録）

- [ ] **Step 1:** `.golangci.yaml` の linters に `depguard` を足し（上の注記の 2 行を消す）、settings に次を足す:

```yaml
    depguard:
      rules:
        # cli と server は usecase を通す（spec 4.2）。テストも同じ（決定 11）。
        # テスト用の部品（決定 12）は許す。
        entry:
          list-mode: lax
          files:
            - "**/internal/app/cli/**"
            - "**/internal/app/server/**"
          allow:
            - github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest
            - github.com/kukv/hedgehug/internal/app/storage/history/historytest
            - github.com/kukv/hedgehug/internal/app/storage/mount/mounttest
          deny:
            - pkg: github.com/kukv/hedgehug/internal/app/storage
              desc: cli と server は usecase を通す（spec 4.2）
            - pkg: github.com/kukv/hedgehug/internal/app/remote
              desc: cli と server は usecase を通す（spec 4.2）
            - pkg: github.com/kukv/hedgehug/internal/lib/gitrepo
              desc: cli と server は usecase を通す（spec 4.2）
            - pkg: github.com/kukv/hedgehug/internal/lib/github
              desc: cli と server は usecase を通す（spec 4.2）
```

  depguard の deny は前方一致なので、`lib/gitrepo` は `gitrepotest` にも、`lib/github` は `lib/githubX` のような名前にも当たる。allow と deny が両方当たるときにどちらが勝つかを、次の Step で実際に確かめる（golangci-lint 2.15.0 の depguard の README を読み、より長い一致が勝つことを確かめる。勝たないなら `$` を付けた完全一致など、README の書き方に合わせる）
- [ ] **Step 2:** 効いていることを確かめる: (a) `make lint` が 0 issues、(b) cli の本体のファイルに一時に `_ "github.com/kukv/hedgehug/internal/app/storage/store"` を足して `make lint` が depguard で落ちる、(c) cli のテストのファイルに同じものを足しても落ちる、(d) gitrepotest を import している cli のテストが落ちない（(b)(c) は戻す）
- [ ] **Step 3:** uses を走らせ、すべて 0 行（テスト用の部品の名前を除く）であることを見る。leaks の行のうち、cli・server が使っているものがないことは uses の 0 行で言える
- [ ] **Step 4:** `make fmt-check`・`make tidy-check`・`make web-check`・`go test -race ./...`（ここで全体を 1 回）。落ちたら出力と条件を記録して報告する
- [ ] **Step 5:** テスト関数の数を回 4 の初め（main 6e2b895）と比べる: `git grep -hE '^func (Test|Fuzz|Bench)' 6e2b895 -- '*.go' | wc -l` と今の数。差は、Task 4・5 の `TestJSONIsTheUsecaseTypes`、usecase の新しい小さなテスト、Task 7 の prompt のテスト、Task 2 の schedule のテスト、watch の `TestMain`（あれば）と、決めたこと 8 のとおり server から watch に移ったものの出入りで説明できること
- [ ] **Step 6:** 概要 plan の回 4 の行の完了条件の後ろに「（<完了した日> 完了）」を、その日の日付で書き足す
- [ ] **Step 7:** コミット `build(lint): cli と server の depguard を有効にする`
