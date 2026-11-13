# hedgehug D4a の続き(間隔ごとの取り直しで削除済みのアイテムを読まない)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** D4a の計画の「A4b が入ったときに足すもの」を、A4b(削除の印・`port.LiveItem` など)が develop に入ったいまの形に合わせて片付ける。間隔ごとの GitHub の読み取り(`Links.ReadOnce`)は、削除済みのアイテムと削除済みのプロジェクトの下のアイテムのリンクを GitHub に頼まず、GitHub から読んだあと書くまでのあいだに削除されたアイテムのリンクは何も書かずに飛ばす。

**Architecture:** `app/githublink` の中だけを変える。集める側(`read_collect.go`)は `port.LiveProjects`・`port.LiveItems`(A4b)で読む。書く側(`read.go` の `writeTarget`)は、`writeFound` が `port.ErrNotFound` を返したら(保存の中の `readItem` が削除済みを 404 にする。A4b-4)、そのリンクを飛ばして続ける。`fire` は変えない。

**Tech Stack:** Go 1.27.3。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/03-workspace-settings.md`「自動化」の「実行の規則」(削除済みのアイテムには動かない)
- 07「GitHub を読む間隔」「PR の自動リンク」(探すリポジトリは削除済みでないアイテム)
- `docs/superpowers/plans/2026-11-13-hedgehug-track-d4a.md`(D4a-10・D4a-14、Review Focus 1・5)、`2026-11-13-hedgehug-track-a4b.md`(A4b-3・A4b-4)

## Global Constraints

D4a の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個。`app/githublink` のテスト以外のファイルは足さない。
- 依存ライブラリは足さない。
- コードのコメントに、ほかのトラック・計画への先送りの書き方(「〜が足す」「計画 X を見よ」)を残さない。
- トークンらしい値(`ghp_` などで始まる文字列)を書かない。

## 決めたこと

| # | 決定 | 根拠 |
|---|---|---|
| DD-1 | `fire` には削除済みの判定を足さない。`fire` を呼ぶのは手でのリンク(`saveLink`)と取り直しの保存(`writeRefresh`)だけで、どちらも保存の中で `readItem`(`port.LiveItem`・`port.LiveProject`)を通すので、削除済みのアイテムと削除済みのプロジェクトの下のアイテムは `port.ErrNotFound` で止まり、値も書かない。`fire` のコメントは、そう書き直す。D4a の計画の `TestFireSkipsDeletedItem`(値は書くがルールは動かない)は作らない | A4b-4(削除済みのアイテムの読み書きは 404)。起きない場合のための判定は書かない |
| DD-2 | 詳細パネルを開いたときの取り直し(`Links.Refresh`)は、削除済みのアイテムでは `port.ErrNotFound`(A4b-4。`TestLinksOfADeletedItemAreNotFound` が確かめている)。D4a-10 の「削除済みのアイテムのリンクは、詳細パネルを開いたときだけ取り直す」はこれに置き換える | A4b-4 |
| DD-3 | 間隔ごとの取り直しは、削除済みのプロジェクトと、削除済みのアイテムのリンクを集めない(`port.LiveProjects`・`port.LiveItems`)。どのアイテムからも集めなかった番号は GitHub に頼まず、失敗の文言も置かない | D4a-10、03「実行の規則」。削除済みのプロジェクトの下のアイテムは `readItem` が 404 にするので、集めると書くときに止まる(いまの develop では、削除済みのアイテムにリンクが 1 本あるだけで、その Store の読み取りが `not found` のエラーで止まり、残りのリンクも書かれない) |
| DD-4 | 間隔ごとの取り直しで、GitHub から読んだあと書くまでのあいだにアイテムかプロジェクトが削除されたら(`writeFound` が `port.ErrNotFound`)、そのリンクは、書くまでのあいだに解除されたリンク(Review Focus 1)と同じく、値もきっかけも書かず、失敗の文言も置かずに飛ばし、ほかのリンクは書き続ける | 利用者の判断(2026-11-14)。1 本の削除で Store の残りの書き込みを止めない |

---

### Task 1: 間隔ごとの取り直しで、削除済みのプロジェクト・アイテムのリンクを集めない

**Files:**
- Modify: `internal/app/githublink/read_collect.go`(`collect` と `collectProject`)
- Modify: `internal/app/githublink/read_test.go`(`TestReadOnceSkipsStores` の中の「削除の印は A4b が足す」のコメント 1 行を消す。この Store のテストは削除済みのアイテムを扱わない)
- Test: `internal/app/githublink/deleted_test.go`(既存のファイルにテストを足す)

**Interfaces:**
- Consumes: `port.LiveProjects(r port.Reader, ws workspace.ID) ([]project.Project, error)`・`port.LiveItems(r port.Reader, p project.ID) ([]item.Item, error)`(`internal/app/port/stores.go`)、テストの `apptest.DeleteItems`・`apptest.DeleteProject`、`read_test.go` の `newRead`・`readFixture.linkIn`・`readPR`・`every`・`readFixture.calls`・`f.reads.failures`
- Produces: なし(振る舞いだけ)

- [ ] **Step 1: 失敗するテストを書く**(`deleted_test.go` に足す)

- `TestReadOnceSkipsDeletedItems`: `linkTask`(HH-2)に `org/web#1`、`linkEpic`(HH-1)に `org/web#2` と `org/web#3` を、`linkTask` にも `org/web#3` を `linkIn` でリンクし(どれも Open)、GitHub には 3 本とも Merged で `Put` する。`apptest.DeleteItems(ctx, f.stores.Stores, linkStore, linkTask)` で HH-2 を削除してから `f.read(t, every, false)`。確かめること:
  - `ReadOnce` がエラーを返さない(`f.read` が `t.Fatal` しない)。
  - `org/web` の `ReadMany` は 1 回。
  - HH-1 の 2 本(#2・#3)は Merged になっている(`f.links.List(ctx, linkStore, linkEpic)` の行の `State`)。
  - HH-2 のログに、削除のあとの行(実行者 GitHub の行・自動化の行)が増えていない(`f.taskLog(t)` を削除の直後と読み取りのあとで比べる)。
  - 失敗の文言を置いていない(`f.reads.failures[linkStore]` が空)。
  - #1 を頼んでいないことは、偽の GitHub が頼まれた番号を覚えていないので、`org/web#1` だけを `f.gh.Fail("org/web", 1, errors.New("boom"))`(NotFound でない失敗は問い合わせ全体の失敗になる)にしておき、読み取りが成功することで確かめる。
- `TestReadOnceSkipsDeletedProject`: `linkTask` に `org/web#1` をリンクし、GitHub に Merged で `Put` する。`apptest.DeleteProject(ctx, f.stores.Stores, linkStore, apptest.SampleProject)`(下のアイテムには書かない)のあと `f.read(t, every, false)`。確かめること: エラーを返さない、`org/web` の `ReadMany` は 0 回、失敗の文言を置いていない。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/githublink/ -run 'TestReadOnceSkipsDeleted'`
Expected: FAIL(`github read S1: not found`。いまは削除済みのアイテムのリンクも集め、書くときに `readItem` が 404 にする)

- [ ] **Step 3: 実装する**

`collect` の `r.Projects().List(w.ID)` を `port.LiveProjects(r, w.ID)` に、`collectProject` の `r.Items().List(p)` を `port.LiveItems(r, p)` に替える。`collectProject` のループの「削除の印は A4b が足す」のコメントは消し、関数のコメントを「削除済みでないアイテムのリンクを c に足す(D4a-10)」のように直す。`collect` のコメントにも、削除済みのプロジェクトを読まないことを書く。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/app/githublink/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/githublink/read_collect.go internal/app/githublink/read_test.go internal/app/githublink/deleted_test.go
git commit -m "fix: 間隔ごとの取り直しで、削除済みのプロジェクト・アイテムのリンクを GitHub に頼まない"
```

---

### Task 2: 読んだあと書くまでのあいだに削除されたアイテムのリンクを飛ばす

**Files:**
- Modify: `internal/app/githublink/read.go`(`writeTarget`)
- Test: `internal/app/githublink/deleted_test.go`

**Interfaces:**
- Consumes: `linkFixture.stores.before`(偽の `Stores` で、次の書き込みの前に 1 回だけ呼ぶ関数。`fire_test.go` の `TestRefreshUnlinkedMeanwhile` と同じ使い方)、`linkFixture.withRules`・`statusRule`・`ruleMerged`(`fire_test.go`)、`apptest.DeleteItems`
- Produces: なし

- [ ] **Step 1: 失敗するテストを書く**

- `TestReadOnceItemDeletedMeanwhile`(DD-4): ルール「マージで完了」(`statusRule(ruleMerged, "マージで完了", automation.TriggerPRMerged, apptest.SampleStatusDone)`)を置き、`linkTask` に `org/web#1`、`linkEpic` に `org/web#2` を Open でリンクし、GitHub には 2 本とも Merged で `Put` する。`f.stores.before` に、`apptest.DeleteItems(ctx, f.stores.Stores, linkStore, linkTask)` を呼ぶ関数を置く(GitHub から読んだあと、最初の書き込みの前に HH-2 が削除される)。`f.read(t, every, false)` のあと確かめること:
  - エラーを返さない。
  - HH-1 の #2 は Merged になっている。
  - HH-2 のログに、削除の行のあとの行(実行者 GitHub の行・自動化の行)がない。
  - 失敗の文言を置いていない(`f.reads.failures[linkStore]` が空)。
  - 受信箱の通知は HH-1 の 1 件だけ(`f.inbox.Notices()`)。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/githublink/ -run TestReadOnceItemDeletedMeanwhile`
Expected: FAIL(`not found`)

- [ ] **Step 3: 実装する**

`writeTarget` で、`writeFound` の返したエラーが `errors.Is(err, port.ErrNotFound)` なら、文言を置かずに `false, nil` を返す(DD-4)。`writeTarget` のコメントに「読んだあとで削除されたアイテム(プロジェクト)のリンクは、何も書かずに飛ばす」を足す。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/app/githublink/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/githublink/read.go internal/app/githublink/deleted_test.go
git commit -m "fix: 間隔ごとの取り直しで、読んだあとに削除されたアイテムのリンクは書かずに飛ばす"
```

---

### Task 3: fire のコメントと D4a の計画を、A4b の決定に合わせる

**Files:**
- Modify: `internal/app/githublink/fire.go`(`fire` の関数のコメントと、`tx.Items().Get(id)` の上のコメント)
- Modify: `docs/superpowers/plans/2026-11-13-hedgehug-track-d4a.md`(D4a-10 の行、Task 11 の Step 1 の `TestFireSkipsDeletedItem` の箇条と Run の `|TestFireSkips`、Task 11 の Interfaces の `fire` のコメント、末尾の節「A4b が入ったときに足すもの」)

- [ ] **Step 1: fire.go のコメントを直す**(DD-1)

関数のコメントの「削除済みのアイテムで動かさない判定は、削除の印とともに A4b が足す(計画 D4a「A4b が入ったときに足すもの」)。」を、「削除済みのアイテム(プロジェクト)は、呼ぶ側が保存の中の readItem で port.ErrNotFound にするので、ここには来ない(03「削除済みのアイテムには動かない」、A4b-4)。」に替える。`tx.Items().Get(id)` の上の 1 行のコメントは消す。コードは変えない。

- [ ] **Step 2: D4a の計画を直す**

- D4a-10 の決定の文を「間隔ごとの取り直しの対象は、削除済みでないプロジェクトの、削除済みでないアイテムのリンク。削除済みのアイテムのリンクは、詳細パネルを開いても取り直さない(A4b-4 で 404)」にし、根拠に A4b-4 を足す。
- Task 11 の `fire` の Interfaces のコメント「削除済みのアイテムでは動かさない。」を「削除済みのアイテムは呼ぶ側が 404 にする。」に替える。
- Task 11 の Step 1 から `TestFireSkipsDeletedItem` の箇条を消し、Run の正規表現から `|TestFireSkips` を消す。
- 末尾の節「A4b が入ったときに足すもの」を、節の見出しは残したまま、「A4b が入ったあとの扱いは `2026-11-14-hedgehug-d4a-deleted-items.md` の DD-1〜DD-4」とする 3 行の表(場所・したこと・テスト)に置き換える:
  - `fire` … 判定は足さない(DD-1)… `TestLinksOfADeletedItemAreNotFound`
  - `collect`・`collectProject` … `port.LiveProjects`・`LiveItems` で集める(DD-3)… `TestReadOnceSkipsDeletedItems`・`TestReadOnceSkipsDeletedProject`
  - `writeTarget` … 読んだあとで削除されたら飛ばす(DD-4)… `TestReadOnceItemDeletedMeanwhile`
  - `readItem`・Refresh … A4b-4 の 404 を採った(DD-2)… `TestLinksOfADeletedItemAreNotFound`

- [ ] **Step 3: 残りがないことを確かめる**

Run: `grep -rn "A4b が足す\|A4b が入ったときに足す\|TestFireSkips" internal docs/superpowers/plans/2026-11-13-hedgehug-track-d4a.md`
Expected: 計画の節の見出し「A4b が入ったときに足すもの」の 1 行だけ

Run: `make check`
Expected: すべて緑

- [ ] **Step 4: コミット**

```bash
git add internal/app/githublink/fire.go docs/superpowers/plans/2026-11-13-hedgehug-track-d4a.md
git commit -m "docs: fire のコメントと D4a の計画を、削除済みを 404 にした A4b の決定に合わせる"
```

---

### Task 4: 最終レビューと PR

- [ ] **Step 1: 最終レビュー**(subagent-driven-development の最後の全体レビュー)
- [ ] **Step 2: PR を出す**

向き先 develop、タイトル「fix: 間隔ごとの取り直しで、削除済みのアイテムのリンクを読まない(D4a の続き)」。本文に、直した不具合(いまの develop で削除済みのアイテムにリンクがあると、その Store の間隔ごとの読み取りが止まる)、決めたこと(DD-1〜DD-4)、確認の結果、使ったスキルと省いた手順を書く。
