# 構成見直し 回 3: usecase の再編 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** mount と context（current）を解体して既存の層に配り、force push 復旧の途中残りの検出を storage/store に移し、usecase を土台と 11 の領域のサブパッケージに分ける。領域の操作は `*usecase.Session` を引数に取る関数にし、usecase のテストを領域へ移す。挙動は変えない。

**Architecture:** 手で書き換えるには量が多い（Session のメソッド 113 個、呼び出し 89 種類、テスト 70 ファイル）ので、go/types で参照を解いて書き換える道具を `tools/restructure/`（本体とは別の Go モジュール）に置き、手順をスクリプトと指示ファイルに固めた。各 Task は `tools/restructure/round3/run.sh <step>` を 1 回走らせ、確かめてコミットする。道具の使い方はそれぞれの main.go の先頭のコメントにある。

**Tech Stack:** Go、golang.org/x/tools（go/packages）、perl、golangci-lint 2.15.0（`golangci-lint fmt` が使わなくなった import を消す）。

**Spec:** `docs/superpowers/specs/2026-11-04-codebase-restructure-design.md`（2.2・2.3 節、決定 12〜14、6 節の PR 3）。概要 plan: `docs/superpowers/plans/2026-11-04-codebase-restructure.md`

## Global Constraints

- 概要 plan の Global Constraints をすべて守る
- この回では、spec 2.2 の mount と context の解体、途中残りの検出の移動、2.3 の領域分け（決定 13・14）のほかに何も変えない
- テストの中身の変更は、import・名前・パッケージの書き換え（道具が行う）と、下の「この回で決めたこと」の 7 にある 4 か所だけにする。テスト関数の数と名前は変えない（Task 8 で比べる）
- Task の途中で何かが失敗したら（`run.sh` がエラーで終わる、確かめる手順の結果が違う）、出力を記録し、`git reset --hard` と `git clean -fd internal tools` で前の Task のコミットに戻してから報告する。途中まで書き換わった状態で続けない・やり直さない（`hoist` 自身はエラーなら何も書かないが、2・3・6 の手順は hoist の前後に perl と `git mv` を走らせる）
- `docs/superpowers/specs/` と `docs/superpowers/plans/` の既存の文書に出てくる旧名は書き換えない
- 道具とスクリプトの中身を Task の途中で直さない。直す必要が出たら止めて報告する（plan の空実行で確かめた手順を崩さないため）

## この回で決めたこと（plan を書くときに決めた。spec にない細部）

1. 道具とスクリプトは `tools/restructure/` に置く。独自の go.mod を持つ別モジュールなので、本体の `go build ./...`・`make lint`・`make test` の対象にならない。回 4・5 でも使う
2. テストの補助
   - usecase のテストの共通の補助は `usecase/usecasetest`（決定 14）。補助は tasks・stores・replication の操作を呼ぶので、usecasetest はそれらを import する。そのため usecase の直下と領域のテストは、すべて外部テストパッケージ（`package tasks_test` など）にする
   - 外部テストが使う非公開の名前は、各パッケージの `export_test.go` で別名を付けて見せる（道具が書く。`var RevertField = revertField` など 22 個）
   - mount のテストの補助（ストアを作る、タスクを書く）は `storage/mount/mounttest` に置く。決定 12 のテスト用の部品に加える
3. 本番に足すもの（テストが構造体の中を直接触っていたため、外部テストにすると見えなくなる）: `Session.ForgetWriter`（旧テストの `resetWriter`）、`Session.ForgetMounts`、`mount.Mounted.IndexOpened`、`mount.Set.Unopened`（usecase に移す `ResolveIn` が使う）
4. 土台（usecase 直下）に下ろすもの。領域をまたいで使われているものを、go/types で参照をたどって洗い出した（plan の空実行で、本番のコードに領域どうしの参照が 0 件になるまで繰り返した）
   - ファイルごと: `assign.go`（`Assign` は `key=value` の解析）・`progress.go`・`duplicates.go`・`crossrefs.go`・mount から来る `refs.go`（`CrossRefs`）
   - 関数ごと: `task_shared.go`（`TaskResult`・`NotDeleted`・`LiveParent`・`ResolveWorkspace`・`WatchInTx`・`Descendants`・`CountTasks` など）、`move_tx.go`（`MoveInTx` と、それが使う順番の計算）、`read_shared.go`（`ShowView`・`LogView`・`ParseQuery` など）、`sync_shared.go`（`SyncReport`・`SyncTargets`・`finishRebuild` など）、`store_shared.go`（`storeExists`・`updateConfig`・`githubErr`・`CheckPrefixElsewhere` など）
   - 旧 doctor の `Finding`・`Severity`・`SortFindings`・`FindingsFail` と、スキーマの検査（`CheckSchema`・`CheckSchemaFile`）。schemas（`SchemaCheck`）と maintenance（`Doctor`）の両方が使うため
5. 領域が使う土台の非公開の名前は公開する（58 個。`warn` → `AddWarning`、`taskLabel` → `FormatTaskLabel`、`jsonValue` → `JSONValue` のほかは先頭を大文字にするだけ）
6. 名前を変えるもの
   - mount から usecase へ: `Set.Resolve` → `ResolveIn(set, arg)`、`Set.Moved` → `MovedIn(set, ...)`
   - current から usecase へ: `current.Resolve` → `ResolveDestination`、`Ref.Find` → `FindProject(ref, set)`。current の `Kind`・`Error`・`newError` は usecase の `Kind`・`Error`・`newErr` と同じ形なので一本化し、`contextError` の Kind の詰め替えを消す（エラーの Kind・Code・Args・文言は変わらない）
   - 関数になると型と名前がぶつかる `Session.AuthStatus` → `accounts.AuthStatuses`
   - 旧 doctor の `Error`・`Warning`・`Info`（Severity）→ `SeverityError`・`SeverityWarning`・`SeverityInfo`、`Sort` → `SortFindings`、`Failing` → `FindingsFail`、`Schema` → `CheckSchema`、`SchemaFile` → `CheckSchemaFile`
7. テストの中身を変える 4 か所: `ts.mounts = nil` → `ts.ForgetMounts()`、`sub.tokens == nil || sub.tokens != src.tokens` → `!sub.SharesTokensWith(src)`、`sub.writerDone` → `sub.WriterCached()`、`got.key = ""` → `ClearCandidateKey(&got)`（後の 3 つは `export_test.go` に置く）
8. maintain.go は 2 つに分ける: `SchemaCheck`・`Migrate` と関連は `schemas/schemacheck.go`、`Doctor` と関連は `maintenance/maintain.go`。`crossrefs_test.go` は Doctor のテストが主なので maintenance へ
9. ファイルの改名: current の `repoid.go` → `lib/giturl/giturl.go`、`resolve.go` → `usecase/destination.go`、mount の `moved_test.go` → `usecase/resolve_moved_test.go`、旧 doctor の `schema.go` → `usecase/schema_findings.go`

## 調べてわかっていること（main 9c73096）

- 回 3 の全 Task を 2026-11-04 に空実行し、Task ごとに `go build ./...`・`go vet ./internal/... ./tools/...` が通り、最後に `make lint`（0 issues）・`make fmt-check`・`make tidy-check`・`go test -race ./...`（39 パッケージ ok、FAIL 0）が通ることを確かめた。テスト関数（`Test`・`Fuzz`・`Bench`）は 2035 個から 2045 個になり、増えたのは新しい 10 パッケージの `TestMain` だけだった。確かめた後は元に戻した
- Go では別パッケージに `Session` のメソッドを置けない（決定 13）。道具 `hoist` は `func (ss *Session) X(a)` を `func X(ss *usecase.Session, a)` に、呼び出しの `ss.X(a)` を `tasks.X(ss, a)` に書き換える。テストの `ts.X(a)`（`ts` は `*usecasetest.Session` で Session を埋め込む）は `tasks.X(ts.Session, a)` になる
- 書き換わる呼び出し側は usecase の外にもある: cli・server・`tools/devenv`。run.sh は最後に `git add -A` する
- current の `resolve.go` は mount の `Set.Resolve` を使うので、current を先に解体する（Task 2 → 3 の順）。旧 doctor の `Finding` を土台に下ろすのは、schemas と maintenance に分けた後（Task 7）。先に下ろすと usecase と maintenance が循環する
- `tools/restructure/deps` は usecase のファイルどうしの参照を、ファイルの割り当て表で数える道具。回 3 の手順では使わないが、割り当てを決めるのに使った（回 4・5 で使う）

---

### Task 1: 途中残りの検出を storage/store へ

**Files:**
- Create: `internal/app/storage/store/leftover.go`（`Leftover`・`None`・`FindLeftover`・`ReadParent`・`LeftoverIn`・`LeftoverMessage`・`LeftoverError` と、`RewriteNewSuffix`・`BeforeRewritePrefix`）
- Modify: `internal/app/remote/replicate/rebuild.go`（上を消し、`newSuffix`・`beforePrefix` は store の定数を指す）と、`replicate.X` → `store.X` の呼び出し側（replicate のテスト、`usecase/mount/mount.go`、`usecase/session.go`・`rebuild.go`、cli と usecase のテスト）

**Interfaces:**
- Produces: `store.FindLeftover(dir string) (store.Leftover, error)`、`store.ReadParent(dir string) ([]fs.DirEntry, error)`、`store.LeftoverIn(dir string, entries []fs.DirEntry) store.Leftover`、`store.LeftoverMessage(name string) string`、`type store.LeftoverError struct{ Name string }`。`replicate.FinishLeftover`・`FinishLeftoverKept` は replicate に残り、`store.Leftover` を受け取る

- [ ] **Step 1: 実行する**

Run: `bash tools/restructure/round3/run.sh 1`
Expected: 最後の行が `step 1 done`

- [ ] **Step 2: 確かめる**

Run: `grep -rn "replicate\.\(FindLeftover\|ReadParent\|LeftoverIn\|LeftoverMessage\|LeftoverError\)" internal cmd tools --include='*.go'`
Expected: 出力なし

Run: `grep -n "replicate" internal/app/usecase/mount/mount.go`
Expected: 出力なし（mount が replicate を import しなくなる）

Run: `go test ./internal/app/storage/store/ ./internal/app/remote/replicate/ ./internal/app/usecase/...`
Expected: すべて `ok`

- [ ] **Step 3: コミットする**

```bash
git commit -m "refactor: force push 復旧の途中残りの検出を replicate から storage/store に移す"
```

### Task 2: current を解体する

**Files:**
- Create: `internal/lib/giturl/giturl.go`・`giturl_test.go`（旧 `current/repoid.go`・`repoid_test.go`。`Split`・`RepoID`）
- Create: `internal/app/usecase/destination.go`・`destination_test.go`・`destination_worktree_test.go`（旧 `current/resolve.go` とそのテスト）
- Move: `internal/app/usecase/current/` の残り（`state.go`・`state_test.go`・`testmain_test.go`）→ `internal/app/storage/current/`
- Modify: `internal/app/usecase/addhere.go`（`contextError` から current の Kind の詰め替えを消す）、`discover.go`・`readonly.go`・`storeops.go`（`giturl`）、`use.go`・`view.go`、`internal/app/cli/add.go`

**Interfaces:**
- Consumes: なし
- Produces: `giturl.Split(u string) (host, owner, repo string, ok bool)`、`giturl.RepoID(u string) (string, bool)`。`current.State`・`current.Ref`・`current.Load`・`current.Save`（`internal/app/storage/current`）。usecase の `ResolveDestination(set *mount.Set, st current.State, in Input, warn func(string)) (*Destination, *Confirm, *Choice, error)`、`FindProject(ref current.Ref, set *mount.Set) (...)`、`Input`・`Destination`・`Confirm`・`Choice`・`Answer`（`Unasked`・`Yes`・`No`）・`LookupError`・`ProjectKind`・`UseGoneWarning`。拒否はすべて `*usecase.Error`

- [ ] **Step 1: 実行する**

Run: `bash tools/restructure/round3/run.sh 2`
Expected: `hoist: wrote 6 files`、`hoist: wrote 10 files`、`step 2 done`

- [ ] **Step 2: 確かめる**

Run: `ls internal/app/usecase/current 2>&1; ls internal/app/storage/current internal/lib/giturl`
Expected: 1 つ目は「そのようなファイルやディレクトリはありません」。2 つ目は `state.go state_test.go testmain_test.go` と `giturl.go giturl_test.go`

Run: `grep -c "^type Kind\|^type Error\|^func newError" internal/app/usecase/destination.go`
Expected: `0`（current の Kind と Error の定義が残っていない）

Run: `go test ./internal/app/usecase/ ./internal/app/storage/current/ ./internal/lib/giturl/ ./internal/app/cli/`
Expected: すべて `ok`

- [ ] **Step 3: コミットする**

```bash
git commit -m "refactor: current を解体し、URL の分解を lib/giturl、context.json を storage/current、行き先の判断を usecase に移す"
```

### Task 3: mount を解体する

**Files:**
- Move: `internal/app/usecase/mount/` → `internal/app/storage/mount/`（`mount.go` と、そのテスト `mount_test.go`・`closerepo_test.go`・`leftover_test.go`・`set_close_test.go`・`qualified_test.go`・`helpers_test.go`・`testmain_test.go`）
- Create: `internal/app/storage/mount/mounttest/mounttest.go`（`Author`・`NewStoreAt`・`PutTasks`。元は mount の `helpers_test.go`）
- Create: `internal/app/usecase/resolve.go`・`query.go`・`refs.go` と、そのテスト `resolve_test.go`・`resolve_moved_test.go`・`query_test.go`・`refs_test.go`
- Modify: `internal/app/storage/mount/mount.go`（`IndexOpened`・`Unopened` を足す）、cli の `add.go`・`ls.go`・`output.go`・`tree.go`、usecase の呼び出し側

**Interfaces:**
- Consumes: Task 1 の `store.ReadParent`・`store.LeftoverIn`・`store.LeftoverError`
- Produces: `mount.Open`・`mount.Set`・`mount.Mounted`・`mount.Options`・`mount.Skipped`・`mount.SharedPrefixes`、`(*mount.Mounted).IndexOpened() bool`、`(*mount.Set).Unopened(mount.Skipped)`。usecase の `ResolveIn(s *mount.Set, arg string) (Hit, error)`、`MovedIn(s *mount.Set, from *mount.Mounted, input string, f *index.ForwardEntry) *MovedError`、`Hit`・`AmbiguousError`・`MovedError`、`Query(set *mount.Set, q *query.Query, only string, opt QueryOptions) (*Result, error)`・`Entry`・`Group`・`Result`、`CrossRefs(set *mount.Set, name string) ([]CrossRef, error)`・`CrossRef`・`TaskRef`

- [ ] **Step 1: 実行する**

Run: `bash tools/restructure/round3/run.sh 3`
Expected: `hoist: wrote 18 files`、`step 3 done`

- [ ] **Step 2: 確かめる**

Run: `ls internal/app/usecase/mount 2>&1; go list -deps ./internal/app/storage/mount | grep -c "remote/replicate"`
Expected: 1 つ目は「そのようなファイルやディレクトリはありません」。2 つ目は `0`

Run: `go test ./internal/app/storage/mount/... ./internal/app/usecase/ ./internal/app/cli/ ./internal/app/server/`
Expected: すべて `ok`（mounttest は `[no test files]`）

- [ ] **Step 3: コミットする**

```bash
git commit -m "refactor: mount を解体し、Store の集合の開閉を storage/mount、ID の解決と横断クエリを usecase に移す"
```

### Task 4: 領域をまたいで使うものを土台に下ろし、公開する

**Files:**
- Create: `internal/app/usecase/schemacheck.go`（maintain.go の `SchemaCheck`・`Migrate` と関連）、`task_shared.go`・`move_tx.go`・`read_shared.go`・`sync_shared.go`・`store_shared.go`
- Modify: `internal/app/usecase/` の `maintain.go`・`crossrefs.go`・`duplicates.go`・`task.go`・`delete.go`・`undo.go`・`mention.go`・`move.go`・`read.go`・`sync.go`・`rebuild.go`・`inbox.go`・`storeops.go`・`workspace.go`・`comment.go`・`mount_ops.go`（宣言を移すだけ）と、公開の改名を受けるファイル（usecase と、テスト）

**Interfaces:**
- Consumes: Task 2・3 の usecase
- Produces: 上の「この回で決めたこと」4・5 の名前。`tools/restructure/round3/4-export.spec` の `rename` の行が、公開する名前の一覧（例: `Session.warn` → `AddWarning`、`newErr` → `NewErr`、`Session.forTask` → `ForTask`、`Session.openHistory` → `OpenHistory`）

- [ ] **Step 1: 実行する**

Run: `bash tools/restructure/round3/run.sh 4`
Expected: `hoist: wrote 58 files`、`step 4 done`

- [ ] **Step 2: 確かめる**

Run: `grep -c "^func (ss \*Session) warn(" internal/app/usecase/session.go; grep -c "^func (ss \*Session) AddWarning(" internal/app/usecase/session.go`
Expected: `0` と `1`

Run: `go test ./internal/app/usecase/...`
Expected: すべて `ok`

- [ ] **Step 3: コミットする**

```bash
git commit -m "refactor(usecase): 領域をまたいで使う関数を土台のファイルに集め、領域から使う名前を公開する"
```

### Task 5: テストの補助を集め、構造体の中を触るテストを直す

**Files:**
- Modify: `internal/app/usecase/helpers_test.go`（`testSession` のメソッドと、2 つ以上の領域のテストが使う補助をすべて集める。元は 24 個のテストファイル）、`session.go`（`resetWriter` を移す。名前はまだ変えない）
- Modify: `internal/app/usecase/session.go`（`ForgetMounts` を足す）、`use_test.go`・`final_fixes_test.go`・`discover_test.go`（「この回で決めたこと」7 の 4 か所）
- Create: `internal/app/usecase/export_test.go`（`SharesTokensWith`・`WriterCached`）、`discover_export_test.go`（`ClearCandidateKey`）

**Interfaces:**
- Produces: `func (ss *Session) ForgetMounts()`。Task 6 は `helpers_test.go` をまるごと `usecasetest/session.go` にする

- [ ] **Step 1: 実行する**

Run: `bash tools/restructure/round3/run.sh 5`
Expected: `step 5 done`

- [ ] **Step 2: 確かめる**

Run: `grep -c "^func (ts \*testSession)" internal/app/usecase/helpers_test.go; grep -l "^func (ts \*testSession)" internal/app/usecase/*_test.go`
Expected: 1 つ目は `35`、2 つ目は `internal/app/usecase/helpers_test.go` だけ

Run: `go test ./internal/app/usecase/`
Expected: `ok`

- [ ] **Step 3: コミットする**

```bash
git commit -m "test(usecase): 領域をまたぐテストの補助を helpers_test.go に集め、構造体の中を触るテストをメソッド経由にする"
```

### Task 6: usecase を 11 の領域に分ける

**Files:**
- Create: `internal/app/usecase/{tasks,comments,browse,views,schemas,workspaces,stores,replication,notifications,accounts}/`（spec 2.3 のファイルと、そのテスト。割り当ては `tools/restructure/round3/6-split.spec` の `to`・`file` の行）と各 `testmain_test.go`・`export_test.go`
- Create: `internal/app/usecase/usecasetest/session.go`（旧 `helpers_test.go`。`testSession` → `Session`、`newTestSession` → `New`、`newTwoStoreSession` → `NewTwoStore` など）
- Move: `maintain.go`（Doctor）・`purge.go`・`rebuild.go` とそのテストを `internal/app/usecase/maintenance/` へ
- Modify: usecase 直下に残るテスト（すべて `package usecase_test` になる）、cli・server・`tools/devenv` の呼び出し側（`ss.Add(in)` → `tasks.Add(ss, in)` など）、`.golangci.yaml`（usecasetest を forbidigo と gosec の対象外にする。gitrepotest などと同じ扱い）

**Interfaces:**
- Consumes: Task 4・5 の土台
- Produces: 領域の操作 `<area>.<Op>(ss *usecase.Session, ...)`。例: `tasks.Add(ss, tasks.AddInput{...})`、`browse.List(ss, q)`、`stores.StoreInit(ss, in)`、`replication.Sync(ss, all, explicit)`、`notifications.InboxList(ss, all)`、`accounts.AuthStatuses(ss)`、`maintenance.Doctor(ss, fix)`、`schemas.SchemaCheck(ss)`。テスト用に `usecasetest.New(t)`・`usecasetest.NewTwoStore(t)`・`(*usecasetest.Session).AddTask` など

- [ ] **Step 1: 実行する**

Run: `bash tools/restructure/round3/run.sh 6`
Expected: `hoist: wrote 181 files`、`step 6 done`

- [ ] **Step 2: 確かめる**

Run: `ls internal/app/usecase/*.go | grep -v _test | xargs -n1 basename | tr '\n' ' '`
Expected: `assign.go conn.go crossrefs.go destination.go duplicates.go errors.go mount_ops.go move_tx.go progress.go query.go read_shared.go readonly.go refs.go resolve.go session.go store_shared.go sync_shared.go task_shared.go view.go`（query.go は browse へ移るので、ここに出ないこと）

Run: `grep -L "^package usecase_test$" internal/app/usecase/*_test.go`
Expected: `internal/app/usecase/export_test.go` だけ（`package usecase`）

Run: `go test ./internal/app/...`
Expected: すべて `ok`

- [ ] **Step 3: コミットする**

```bash
git commit -m "refactor(usecase): 領域ごとのサブパッケージに分け、操作を Session を受け取る関数にする

テストは領域へ移して外部テストパッケージにし、共通の補助を usecasetest に置く。"
```

### Task 7: 旧 doctor の Finding とスキーマの検査を土台に下ろす

**Files:**
- Move: `internal/app/usecase/maintenance/finding.go` → `internal/app/usecase/finding.go`、`maintenance/schema.go` → `internal/app/usecase/schema_findings.go`（`taskWhere` を doctor.go から連れていく）
- Modify: `internal/app/usecase/maintenance/doctor.go`（パッケージのコメントを移す）、schemas・maintenance・cli・server の呼び出し側（`maintenance.Finding` → `usecase.Finding`、`maintenance.Error` → `usecase.SeverityError` など）

**Interfaces:**
- Produces: `usecase.Finding`・`usecase.Severity`・`usecase.SeverityError`・`SeverityWarning`・`SeverityInfo`・`usecase.SortFindings`・`usecase.FindingsFail`・`usecase.CheckSchema`・`usecase.CheckSchemaFile`・`usecase.TaskWhere`・`usecase.BrokenSchemaHint`

- [ ] **Step 1: 実行する**

Run: `bash tools/restructure/round3/run.sh 7`
Expected: `hoist: wrote 17 files`、`step 7 done`

- [ ] **Step 2: 確かめる**

Run: `go list -deps ./internal/app/usecase/schemas | grep -c "usecase/maintenance"`
Expected: `0`（schemas は maintenance を使わない）

Run: `go list -deps ./internal/app/usecase | grep -c "internal/app/usecase/"`
Expected: `0`（土台は領域を使わない）

- [ ] **Step 3: コミットする**

```bash
git commit -m "refactor(usecase): 旧 doctor の Finding とスキーマの検査を土台に下ろす"
```

### Task 8: 全体を確かめ、文書を直す

**Files:**
- Modify: `docs/superpowers/plans/2026-11-04-codebase-restructure.md`（回 3 の行に結果を書く）

- [ ] **Step 1: 完了条件とテスト関数を確かめる**

Run: `bash tools/restructure/round3/check.sh`（比べる相手は、このブランチが origin/main から分かれたコミット）
Expected: 出力は `TestMain added: 10` の 1 行だけで、終了コード 0。領域どうしの import、土台から領域への import、storage/mount から replicate への import があればその行を出し、テスト関数（`Test`・`Fuzz`・`Bench`）が main と違えばその行を出して 1 で終わる

- [ ] **Step 2: 全体の検査（仕上げに 1 回）**

Run: `make lint && make fmt-check && make tidy-check && make test`
Expected: lint は `0 issues.`、テストはすべて通る。落ちたら、失敗の出力と実行条件（コマンド、`-race` の有無）を作業記録に残す

- [ ] **Step 3: 概要 plan の回 3 の行を直してコミットする**

概要 plan の表の回 3 の「完了条件」の後ろに、次を足す:「（2026-11-04 完了。領域の操作は関数、テストは外部テストパッケージ、補助は usecasetest と mounttest）」

```bash
git add docs/superpowers/plans/2026-11-04-codebase-restructure.md
git commit -m "docs(plan): 構成見直しの回 3 を完了にする"
```

- [ ] **Step 4: push して PR を作る**

利用者に確かめてから `git push -u origin restructure3-usecase` で push し（ブランチの upstream は外してある。素の `git push` はしない）、PR を作る。PR の説明には、spec 決定 13・14（領域の操作を関数にする、テストを領域へ移す）、この plan の「この回で決めたこと」、道具 `tools/restructure` の置き場所、挙動を変えていないこと（テスト関数が main と同じこと）を書く。
