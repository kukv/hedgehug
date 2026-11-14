# hedgehug トラック C4c2(上書きの値の表示と「自分の値に戻す」)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 受信箱の「上書き」の行のうち、アイテムとプロジェクトの値の項目に「あなたの値 X → 残った値 Y」と「自分の値に戻す」を出し、押すとあなたの値を新しい 1 回の保存として書く。ボタンの有効・無効と理由、押した瞬間の重なりの断り、戻したあとの「自分の値に戻しました」まで。

**Architecture:** 判断は `internal/app/inbox` の 1 つの関数 `slotOf`(通知と読み取りから、項目・いまの値・表示・使えない理由・書く行を返す)にまとめ、行を作るとき(`resolve`)と `Stores.Write` の中(`Service.Revert`)の両方で呼ぶ。`inbox` は `items`・`projects`・`planning` を import できないので、port と domain(`item`・`project`・`planning`・`workspace`)だけで書く。止まった通知と同じく、結果は `Book.Resolve(NoticeApplied)` で書き、画面は種類で文言を分ける。

**Tech Stack:** Go 1.27.3(huma v2、modernc.org/sqlite)、TypeScript・Svelte(web/packages/core・web/packages/api・web/app)、vitest、Playwright。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md` `:217`(負けたマシンの例「あなたの値 X → 残った値 Y」)、`:219-235`(上書きの通知の表・戻すボタンを出さない理由・押したときの保存とアクティビティ・削除済みと閲覧のみ)、`:461`(操作の「上書きの通知」)、`:471-476`(ボタンの有効と無効・使えない理由・押した瞬間の重なり)、`:478`(保存に入らない操作)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/03-workspace-settings.md` `:392`(遷移の制約を適用しない)、`:514`(自動化は動かない)、`:561`(使わないステータスにもなる)
- `docs/superpowers/plans/2026-11-15-hedgehug-track-c4c1.md`(`Notice.Field`・`Mine`・`Kept` の形。決定 C4c1-9・C4c1-10)
- `docs/superpowers/plans/2026-11-14-hedgehug-track-c4a.md`(止まった通知の `Action`・`Resolve`・C4a-10〜C4a-12)
- `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md` の C4c2 の行

## 始める前に

- develop(C4c1 の PR #223 を含む)から `feat/track-c4c2` を作ってある。
- 実装で計画から変えたことは、この節の下に表で足す。

### 実装で計画から変えたこと

| # | 変えたこと | 理由 |
|---|---|---|
| 1 | C4c2-6 に「プロジェクトのタイトルは、同じワークスペースに同じ名前の生きているプロジェクトがあれば `project.CheckRename` の断りの文」を足す | 通常の保存(`projects.Rename`)が同じ名前を断るので、戻す保存だけが同じ名前を作れる経路にしない |
| 2 | `revert_slot.go` が 300 行を超えるので、戻す行の組み立てを `revert_drafts.go` に分けた。Task 2 で `internal/adapter/httpapi/inbox/inbox_test.go` の setup に Clock を足した | ファイルの大きさの上限。受信箱の行の表示に時計を使うため |
| 3 | Task 3 で `slotOf` に状態 `slotGone` を足した。Store にないエンティティ・いまのワークスペースにないプロパティは本文だけとし、押したときは「{ID} は先に変更されたため、適用しませんでした」と断る。本文だけの項目は項目名だけで `slotNone` に決め、`ErrInvalidOperation` にする | 消えた対象を「戻せる」と見せず、押した瞬間の重なりと同じ断りにするため |
| 4 | 断りの HTTP の状態は 409 ではなく、止まった通知と同じ `apierr.From` の 422 にした | 止まった通知の断りと画面側の扱いを揃えるため |
| 5 | `make generate` のあと `pnpm --dir web gen` で `generated.ts` を作り直す | API の型が変わると画面の型が古くなるため |
| 6 | あなたの値と残った値が JSON の上で同じ「空」(例: チェックの false と null)のときは、行を書かずに処理済みにする(「自分の値に戻しました」) | C4c1 がバイトで負けを判定したときだけ起き、戻す相手がない。ふつうの保存では起きない |
| 7 | スプリント・リリースの名前は、表示と理由の文で空白を 1 つにつめる(`oneLine`) | 通常のスプリントの保存(`planning.SetSprint`)のアクティビティと表記を揃える |
| 8 | 使えない(削除済みの)スプリントからバックログへ戻すとき、枠の末尾の順位はアイテム自身を数えることがあるが、そのまま | 末尾に置くという結果は変わらない |

## Global Constraints

土台・トラック C・C4a・C4c1 の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個(`internal/archtest/size.go`)。web は .svelte 200 行(`InboxRow.svelte` はいま 164 行なので、上書きの表示は新しい部品にする)、.ts 300 行。
- app の機能どうしは import しない(`inbox` は `items`・`projects`・`planning`・`history` を import しない)。表示名の引き方(「不明なユーザー」)は、ほかの app パッケージと同じく `inbox` の中に小さく書く。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- e2e のビルドだけで使う関数は `//go:build e2e` のファイルに、本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。
- 文言(06 のとおり):
  - 表示「あなたの値 {X} → 残った値 {Y}」(複数の値は「, 」でつなぎ、空は「なし」)
  - ボタン「自分の値に戻す」、戻したあと「自分の値に戻しました」、トースト「上書き前の値に戻しました」
  - アクティビティ「{ID} の{項目名}を上書き前の値に戻す」、プロジェクトは「{タイトル} の{項目名}を上書き前の値に戻す」
  - 理由「閲覧のみの Store です」「そのあとさらに変更されています(いまは {値})」「「{名前}」は削除されています」「スプリント「{名前}」は完了しています」「担当者 {ログイン名} はこの Store のメンバーにいません」
  - 押した瞬間の重なり「{ID} は先に変更されたため、適用しませんでした」
- 手元の検証は 1 回で足りる(CI でも流れる)。`make lint`・`make test`・e2e のビルドの lint(`golangci-lint run --build-tags e2e ./...`)・`make web-check`・`inbox-overwrite.spec.ts` の e2e。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| C4c2-1 | 範囲: 受信箱の上書きの行の値の表示・「自分の値に戻す」・有効と無効と理由・戻した保存とアクティビティの文・押した瞬間の重なり。アクティビティの画面は C4d | トラック C の C4c2 の行 |
| C4c2-2 | 06 の表にない項目(ワークスペースの設定・スプリントとリリースの定義・コメント・作業時間・メンバーの表示名)の上書きの行は、本文だけ(値もボタンも出さない)。06 の表に 1 行足す | ユーザーの決定(2026-11-15) |
| C4c2-3 | 値とボタンを出す項目は、通知の対象と `Field` の組で決める。アイテム(`Item` があり `Project` が空): `title`・`sprint`・`release`・プロパティの項目(`item.Field(p)`。組み込みのキーと `fields/<ID>`)。プロジェクト(`Project` があり `Field` が `project_rules/` で始まらない): `title`・`status`・`owner`・`start`・`end`。ほか(場所・順位・種類・関連・リンク・コメント・作業時間・説明・プロジェクト別のルール・ワークスペース・Store)は本文だけ | 06 `:225-229`。`title`・`status`・`start`・`end` はアイテムとプロジェクトで重なるので、対象で分ける。プロジェクト別のルールは `Project` を持つが本文だけ(06 `:228`) |
| C4c2-4 | 状態は上から順に決める: (1) 閲覧のみの Store → ボタンを出して無効、理由「閲覧のみの Store です」。(2) エンティティが生きていない(Store にない・削除済み・ワークスペースが削除済み)→ ボタンを出さない。(3) プロパティがアイテムのいまのワークスペースにない(プロパティの削除・別のワークスペースへの移動)→ ボタンを出さない。(4) いまの値が `Kept` と違う → 無効、「そのあとさらに変更されています(いまは {値})」。(5) 戻す先の値が使えない → 無効、C4c2-6 の理由。どれでもなければ有効 | 06 `:235`・`:471`(閲覧のみが先、削除済みはボタンを出さない)、`:474-475`。(3) は戻す先の項目がないので (2) と同じ扱い |
| C4c2-5 | 値の表示: X と Y は通知のワークスペース(`n.Workspace`)の設定とログで、「いまは {値}」はアイテム・プロジェクトのいまのワークスペースで引く(止まった通知の `action` と同じ)。プロパティは `item.Display`(今日は時計の今)。ただし削除済みの値は `workspace.NameOfDeleted` の名前で出す(引けなければ「(削除された値)」)。複数の値は設定の並び(削除済みの値は末尾、ID 順)に並べ直してからつなぐ。タイトルは 1 行にした文字列。スプリント・リリースは名前(削除済みも名前、空は「なし」)。プロジェクトのステータスは値のまま、オーナーは表示名(メンバーにいなければ「不明なユーザー」、空は「なし」)、日付は `project.DateText`。X・Y・いまの値は、エンティティが Store にあれば (2)(3) でも出す。Store にない(別の Store へ移った)ときと、(3) でプロパティを引けないときは本文だけ | 06 `:227`(「, 」と「なし」。例の「Web, API」は設定の並び)。`Mine`・`Kept` は値の JSON の辞書順(C4c1-10)なので並べ直す |
| C4c2-6 | 戻す先の値が使えない理由(C4c2-4 の (5)。上から最初の 1 つ): ステータス・優先度・単一選択の値、関連システム・複数選択の値(`Mine` のうち最初の 1 つ)がいまのワークスペースの設定にない → 「「{名前}」は削除されています」。ユーザーのプロパティのログイン名がメンバーにいない → 「{プロパティ名} {ログイン名} はこの Store のメンバーにいません」(担当者は「担当者 …」)。プロジェクトのオーナー → 「オーナー {ログイン名} はこの Store のメンバーにいません」。スプリント: 使えない(削除済み・ない)→ 「「{名前}」は削除されています」、完了していていまのスプリントでない → 「スプリント「{名前}」は完了しています」、作業項目でない種類 → 「{種類} はスプリントに入れられません」。リリース: 使えない → 「「{名前}」は削除されています」、アーカイブでいまのリリースでない → 「リリース「{名前}」はアーカイブされています」、Step → 「Step はリリースに入れられません」。開始日・終了日: もう片方と比べて `item.CheckDates`・`project.CheckDates` が断ったら、その断りの文 | 06 `:475` の 3 つの例と、通常の保存(`planning.AssignableSprint`・`SetRelease`・`checkDatePair`・`SetDate`・`SetOwner`)の検査の当てはめ |
| C4c2-7 | 戻す保存が外す検査は、ワークフローの遷移と、プロジェクトが使わないステータス(`projectrule.CheckTransition`・`CheckUse`)。自動化は動かさない | 06 `:233`、03 `:392`・`:514`・`:561`。止まった通知の「制約を無視して…」(`automations.applyStatus`、C4a-11)と同じ扱い |
| C4c2-8 | 戻す保存: 1 回の保存。アイテムの `title`・単一の値のプロパティ・`release` は `set`(from はいまの値、to は `Mine`。`Mine` が null か空なら null)。複数の値は、いまの値と `Mine` の差を `add`・`remove` の行で(値の JSON の辞書順、remove が先)。`sprint` は `SetSprint` と同じく、移動先の枠の末尾の `rank` も同じ保存で書く(`planning.FrameEnd` が空なら rank は書かない)。プロジェクトは `set`(空は `""`。`SetOwner`・`SetDate` と同じ形)。実行者は「あなた」(`change.User(me)`)。アクティビティは「{ID} の{項目名}を上書き前の値に戻す」(アイテム)、「{タイトル} の{項目名}を上書き前の値に戻す」(プロジェクト。タイトルは空白を 1 つにつめる)。{項目名} はプロパティ名、`title` タイトル、`sprint` スプリント、`release` リリース、`status` ステータス、`owner` オーナー、`start` 開始日、`end` 終了日 | 06 `:233`、02 の通常の保存の行の形 |
| C4c2-9 | 押した瞬間: `Stores.Write` の中で `slotOf` を呼び直す。閲覧のみ → 「閲覧のみの Store です」、結果がすでにある → 「この通知はすでに処理されています」、上書きでない・値の項目でない → `port.ErrInvalidOperation`、(2)(3)(4) → 「{ID} は先に変更されたため、適用しませんでした」({ID} はアイテム・プロジェクトの ID。引けなければ通知の `Display`)、(5) → その理由。どれも何も書かない。削除済みのものの削除は取り消さない。保存できたら `Book.Resolve(NoticeApplied)` で既読にし、出来事 inbox を出す | 06 `:235`・`:476`、C4a-11・C4a-12 と同じ |
| C4c2-10 | 値を比べるとき(いまの値と `Kept`)は、JSON を読んで比べる。null・`""`・`false`・空の配列を同じ「空」とし、配列は集合で比べ、数は値で比べる | 06 `:474`(複数の値は集合)。アイテムは空を null、プロジェクトは `""`、チェックの未設定は `false` で書くため、バイトでは比べない |
| C4c2-11 | 画面: 上書きの行の本文の下に、新しい部品 `InboxOverwrite.svelte` で「あなたの値 X → 残った値 Y」、ボタン、理由を出す。戻したあとはボタンの代わりにピル「自分の値に戻しました」。押して成功したらトースト「上書き前の値に戻しました」。断りはその文言のエラーのトーストで出し、行を読み直す(止まった通知の `write` と同じ) | 06 `:233`・`:461`・`:476` |
| C4c2-12 | 通しのテストで値付きの上書きの行を作るため、e2e のビルドだけに入口 `POST /e2e/notice`(本文は JSON の `port.Notice` の形。API と同じトークン)を足す。同期は e2e のビルドで再現できないので、本物の負けの検出は `cmd/hedgehug` の 2 台の場面で確かめる | 同期は e2e で再現できない(C4c1-15)。`/e2e/github` と同じ形 |

## Review Focus

1. **戻したあと、時計が進んだ相手にまた負ける:** 戻した保存が、時計の進んだ相手の値にまた負けたら、新しい上書きの行が届き、前の行は「自分の値に戻しました」のまま(Task 6 の場面 `TestRevertLosesAgain`)。
2. **複数の値の集合:** 関連システム「Web, API」で、自分が API を外して負けた行を戻すと、API を外す 1 行だけが書かれ、ほかの人があとで足した値は残す。いまの値の並びが違っても集合が同じなら有効(Task 1 の `TestSameValue`、Task 3 の `TestRevertSystems`)。
3. **空の書き方の違い:** プロジェクトのオーナーを空に戻す、チェックのプロパティを未設定(false)に戻す、担当者を「なし」に戻す。どれも `Kept` と比べて有効になり、正しい空で書く(Task 1 の `TestSameValue`、Task 3 の `TestRevertEmpty`)。
4. **押す前に別のタブで変えた:** 画面を読み直す前に別のタブで同じ項目を変えてから押すと、何も書かずに「{ID} は先に変更されたため、適用しませんでした」(Task 3 の `TestRevertChangedMeanwhile`)。
5. **削除済みと別の Store:** アイテムを削除したあとは、値は出してボタンを出さない。別の Store へ移したあとは本文だけ(Task 2 の `TestOverwriteDeleted`・`TestOverwriteMoved`)。

---

### Task 1: 値の比べ方と表示(`revert_value.go`)

**Files:**
- Create: `internal/app/inbox/revert_value.go`
- Test: `internal/app/inbox/revert_value_internal_test.go`(package inbox)

**Interfaces:**
- Produces:
  - `func sameValue(a, b json.RawMessage) bool` — 決定 C4c2-10
  - `func showProperty(p workspace.Property, v json.RawMessage, st workspace.Settings, log []change.Record, names item.Names, now time.Time) string` — 決定 C4c2-5。log はワークスペースのログ(削除済みの値の名前)
  - `func showPlan(v json.RawMessage, log []change.Record, prefix string) string` — スプリント(`prefix` = `sprints/`)・リリース(`releases/`)の名前。空は「なし」
  - `func showProject(f string, v json.RawMessage, names item.Names, now time.Time) string`
  - `func memberNames(ms []member.Member) item.Names` — 大文字と小文字を区別せずに引き、いなければ「不明なユーザー」

- [ ] **Step 1: 失敗するテストを書く**
  - `TestSameValue`(表で): `null` と `""` と `false` と `[]` と nil はどれも同じ、`["b","a"]` と `["a","b"]` は同じ、`["a"]` と `["a","b"]` は違う、`1` と `1.0` は同じ、`"Todo"` と `"Done"` は違う
  - `TestShowPropertyMultiOrder`: 設定の関連システムが Web, API の順、値 `["API","Web"]`(ID の辞書順)→ 「Web, API」。`[]` → 「なし」
  - `TestShowPropertyDeletedValue`: ログで消したステータス「Todo」の ID → 「Todo」。ログにもない ID → 「(削除された値)」
  - `TestShowPlan`: スプリント「Sprint 25」の ID → 「Sprint 25」、削除済みのスプリントも名前、null → 「なし」
  - `TestShowProject`: `owner` の `"hanako"` → 表示名、メンバーにいない → 「不明なユーザー」、`""` → 「なし」。`start` の今年の日付 → 「5/1」
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/inbox/ -run 'SameValue|Show'` Expected: FAIL(関数がない)
- [ ] **Step 3: 実装する** — 削除済みの値の名前は `workspace.NameOfDeleted(log, "statuses/"+id)` のように、値の種類の頭(`statuses/`・`priorities/`・`systems/`・`choices/<プロパティ>/`)で引く。`item.Display` に渡す前に、複数の値を設定の並びに並べ直す
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/inbox/` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/inbox/revert_value.go internal/app/inbox/revert_value_internal_test.go
git commit -m "feat(inbox): 上書きの値の比べ方と表示を足す(C4c2)"
```

---

### Task 2: 戻す相手の決め方と行の表示(`slotOf`・`Row.Overwrite`)

**Files:**
- Create: `internal/app/inbox/revert_slot.go`(アイテムとプロジェクトの `slot` と `slotOf`)、`internal/app/inbox/revert_reason.go`(決定 C4c2-6 の理由)、`internal/app/inbox/overwrite.go`(`Overwrite` と `overwriteOf`)
- Modify: `internal/app/inbox/page.go`(`Row.Overwrite`)、`internal/app/inbox/target.go`(`resolve` が上書きの行で `overwriteOf` を呼ぶ)、`internal/app/inbox/service.go`(`Identity port.Identity`・`Clock port.Clock`)
- Test: `internal/app/inbox/overwrite_test.go`

**Interfaces:**
- Consumes: Task 1 の関数、C4c1 の `Notice.Field`・`Mine`・`Kept`
- Produces:
  - `type Overwrite struct { Mine, Kept string /* 表示 */; Visible bool /* ボタンを出す */; Enabled bool; Note string; Result port.NoticeResult }`
  - `Row.Overwrite *Overwrite` — 上書きの行で、値を出す項目(C4c2-3)で、エンティティとプロパティを引けるときだけ。ほかは nil(本文だけ)
  - `type slotState int`(`slotNone` 本文だけ・`slotShown` 値だけ・`slotLive` 戻せる)
  - `type slot struct { display string /* HH-1・HH-P1 */; label string /* 項目名 */; summary string /* アクティビティの文 */; now json.RawMessage; mine, kept, current string /* 表示 */; reason string /* C4c2-6。空なら使える */; drafts func(tx port.Tx) ([]change.Draft, error); write func(tx port.Tx, ds []change.Draft) error }`
  - `func slotOf(r port.Reader, n port.NoticeRow, now time.Time) (slot, slotState, error)` — `resolve` と `Service.Revert`(Task 3)の両方が呼ぶ。閲覧のみは見ない(呼ぶ側が `info.ReadOnly` で決める)

- [ ] **Step 1: 失敗するテストを書く**(`setup` の fixture の Store に、上書きの通知を `Field`・`Mine`・`Kept` 付きで足し、`page` の 1 行目の `Overwrite` を見る)
  - `TestOverwriteStatusEnabled`: HH-2 のいまのステータスが Done、通知が `Field: "status"`・`Mine: Todo の ID`・`Kept: Done の ID` → `Mine == "Todo"`・`Kept == "Done"`・`Visible`・`Enabled`・`Note == ""`
  - `TestOverwriteReadOnly`: 閲覧のみの Store → `Visible`・`!Enabled`・`Note == "閲覧のみの Store です"`(アイテムが削除済みでも同じ)
  - `TestOverwriteDeleted`: アイテムを削除済みにする → `Mine`・`Kept` は出し、`!Visible`
  - `TestOverwriteMoved`: Store にないアイテム → `Overwrite == nil`
  - `TestOverwriteChangedAfter`: いまのステータスが InProgress → `!Enabled`・`Note == "そのあとさらに変更されています(いまは InProgress)"`
  - `TestOverwriteReasons`(表で): 戻す先のステータスを削除 → 「「Todo」は削除されています」、担当者 `ghost` → 「担当者 ghost はこの Store のメンバーにいません」、完了したスプリント「Sprint 25」→ 「スプリント「Sprint 25」は完了しています」、開始日を終了日よりあとに戻す → `item.CheckDates` の文
  - `TestOverwriteProject`: プロジェクトのオーナーの通知(`Mine: "hanako"`・`Kept: "taro"`、いまのオーナー taro)→ 表示名で出て有効。`Mine: ""` なら「なし」
  - `TestOverwriteBodyOnly`(表で): `Field` が `place`・`rank`・`kind`・`relations`・`links`・`comment:<ID>`・`desc`・`project_rules/<ID>/hidden_statuses`、ワークスペースの通知(`name`)、メンバーの通知 → `Overwrite == nil`
  - `TestOverwriteResult`: `Resolve(NoticeApplied)` 済みの通知 → `Result == "applied"`
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/inbox/ -run Overwrite` Expected: FAIL
- [ ] **Step 3: 実装する** — `slotOf` は C4c2-3 で振り分け、アイテムは `r.Items().Get`(ないなら `slotNone`)、削除済みか `port.LiveWorkspace` が見つからないなら `slotShown`。プロパティは、いまのワークスペースの設定から `item.Field(p) == n.Field` で探す(ないなら `slotNone`)。`overwriteOf` は C4c2-4 の順で `Overwrite` を作る。`routes.go` の `appinbox.Service` に `Identity: deps.Identity, Clock: deps.Clock` を渡す
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/inbox/ ./internal/adapter/...` Expected: PASS(既存の止まった通知のテストを含む)
- [ ] **Step 5: コミット**

```bash
git add internal/app/inbox/ internal/adapter/httpapi/routes.go
git commit -m "feat(inbox): 上書きの行にあなたの値と残った値を出し、戻せるかを決める(C4c2)"
```

---

### Task 3: 「自分の値に戻す」の保存(`Service.Revert`)

**Files:**
- Create: `internal/app/inbox/revert.go`
- Test: `internal/app/inbox/revert_test.go`

**Interfaces:**
- Consumes: Task 2 の `slotOf`・`slot`
- Produces: `func (s Service) Revert(ctx context.Context, id port.NoticeID) error` — 決定 C4c2-8・C4c2-9

- [ ] **Step 1: 失敗するテストを書く**
  - `TestRevertStatus`: いまのステータス Done、`Mine` Todo → HH-2 が Todo、最後の保存のアクティビティが「HH-2 のステータスを上書き前の値に戻す」、実行者が「あなた」、通知が既読で `Result == applied`、出来事 inbox が 1 回。ワークフローで Done → Todo を禁じていても書ける。ステータスを Done にする自動化のルールがあっても動かない
  - `TestRevertSystems`: 前「Web, API」、`Mine ["Web"]`、`Kept ["API","Web"]`、いまの値も同じ → `remove systems "API"` の 1 行だけ
  - `TestRevertEmpty`(表で): 担当者を null に、チェックのプロパティを false(`Kept` は true)に、プロジェクトのオーナーを `""` に戻す → それぞれ null・false・`""` の `set`
  - `TestRevertSprint`: `Mine` がスプリント S → `sprint` の `set` と、S の枠の末尾の `rank` の `set` が同じ保存
  - `TestRevertProjectTitle`: プロジェクトのタイトル → アクティビティ「{タイトル} のタイトルを上書き前の値に戻す」
  - `TestRevertChangedMeanwhile`: 通知を作ったあと、いまのステータスを InProgress にする → 断り「HH-2 は先に変更されたため、適用しませんでした」、保存なし、結果は空のまま
  - `TestRevertRefusals`(表で): 閲覧のみの Store → 「閲覧のみの Store です」、結果があと → 「この通知はすでに処理されています」、削除済みのアイテム → 「HH-2 は先に変更されたため…」(削除は取り消さない)、戻す先のステータスを削除 → 「「Todo」は削除されています」、本文だけの項目・上書きでない通知 → `port.ErrInvalidOperation`、表示中でない Store → `port.ErrNotFound`
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/inbox/ -run Revert` Expected: FAIL
- [ ] **Step 3: 実装する** — 通知の検査は止まった通知の `stopped` と同じ順(種類 → 結果 → 表示中 → 閲覧のみ)。保存の文は保存の前に `Stores.Read` の中の `slotOf` で決め、`Stores.Write` の中で `slotOf` を呼び直して C4c2-9 のとおり断る。行がなければ書かない(`Kept` と同じなので起きないが、`port.Unchanged` と同じく何も残さない)
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/inbox/` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/inbox/revert.go internal/app/inbox/revert_test.go
git commit -m "feat(inbox): 上書きの通知から自分の値に戻す(C4c2)"
```

---

### Task 4: HTTP の口と API の型

**Files:**
- Modify: `internal/adapter/httpapi/inbox/dto.go`(`InboxOverwrite`・`InboxRow.Overwrite`)、`internal/adapter/httpapi/inbox/inbox.go`(`revert-inbox-notice`)
- Test: `internal/adapter/httpapi/inbox/inbox_test.go`
- Modify: `api/openapi.yaml`・`web/packages/api/src/generated.ts`(`make generate`)

**Interfaces:**
- Produces:
  - `POST /api/inbox/{notice}/revert`(OperationID `revert-inbox-notice`、Summary「上書きされた値を自分の値に戻す」、204)
  - `InboxOverwrite { mine string; kept string; visible bool; enabled bool; note string; result string(enum ",applied") }`、`InboxRow.Overwrite *InboxOverwrite`(`json:"overwrite,omitempty"`、doc「上書きの行で、値を出す項目だけ」)

- [ ] **Step 1: 失敗するテストを書く** — `TestRevertRoute`: 上書きの通知の行の JSON に `overwrite.mine`・`kept`・`enabled` がある。`POST …/revert` が 204 で、通知が既読。断りは 409(既存の `apierr.From` の断りの扱い)
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/adapter/httpapi/inbox/` Expected: FAIL
- [ ] **Step 3: 実装する** — `registerActions` に足す。`make generate` で `api/openapi.yaml` と web の生成物を作り直す
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/adapter/... && make web-check` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/adapter/httpapi/inbox/ api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat(api): 上書きの値と自分の値に戻す口を足す(C4c2)"
```

---

### Task 5: 画面(受信箱の上書きの行)

**Files:**
- Modify: `web/packages/core/src/model/inbox.ts`(`InboxOverwrite`・`InboxRow.overwrite`)、`web/packages/core/src/port/inbox.ts`(`revert(id)`)、`web/packages/core/src/features/inbox/inbox.ts`(`revert`・`inboxRevertedMessage`)、Test: `web/packages/core/src/features/inbox/inbox.test.ts`
- Modify: `web/packages/api/src/inbox.ts`(`revert`)、Test: `web/packages/api/src/inbox.test.ts`
- Create: `web/app/src/screens/inbox/InboxOverwrite.svelte`
- Modify: `web/app/src/screens/inbox/InboxRow.svelte`(上書きの行で `InboxOverwrite` を出す)

**Interfaces:**
- Produces(core): `Inbox.revert(row: InboxRow): Promise<void>`、`export const inboxRevertedMessage = "上書き前の値に戻しました"`、`InboxPort.revert(id: number): Promise<void>`

- [ ] **Step 1: 失敗するテストを書く**
  - core `revert`: 成功で `port.revert(row.id)` を呼び、トースト「上書き前の値に戻しました」、一覧を読み直す。断り(`Error("HH-2 は先に変更されたため、適用しませんでした")`)はその文言のエラーのトーストで、読み直す
  - api `revert`: `POST /api/inbox/7/revert` を送る
- [ ] **Step 2: 失敗を確かめる** — Run: `(cd web && pnpm vitest run packages/core/src/features/inbox packages/api/src/inbox.test.ts)` Expected: FAIL
- [ ] **Step 3: 実装する** — `revert` は `apply` と同じく `write(() => port.revert(row.id), () => toasts.show(inboxRevertedMessage))`。`InboxOverwrite.svelte` は「あなたの値 {mine} → 残った値 {kept}」の 1 行、`result` があればピル「自分の値に戻しました」、なければ `visible` のときボタン「自分の値に戻す」(`disabled={!enabled}`)と `note`。見た目は `InboxAction.svelte` に合わせる
- [ ] **Step 4: 通ることを確かめる** — Run: `make web-check` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add web/packages/core/src web/packages/api/src/inbox.ts web/packages/api/src/inbox.test.ts web/app/src/screens/inbox/
git commit -m "feat(web): 受信箱の上書きの行に値と「自分の値に戻す」を出す(C4c2)"
```

---

### Task 6: 2 台の場面と e2e

**Files:**
- Modify: `cmd/hedgehug/notices_overwrite_test.go`(場面を足す。600 行を超えるなら `cmd/hedgehug/notices_revert_test.go` に分ける)
- Create: `cmd/hedgehug/notice_e2e.go`(`//go:build e2e`。`POST /e2e/notice`)、Test: `cmd/hedgehug/notice_e2e_test.go`(`//go:build e2e`)
- Modify: `cmd/hedgehug/device_e2e.go`(`wrap` に足す)
- Create: `web/e2e/inbox-overwrite.spec.ts`

**Interfaces:**
- Produces: `func noticeHandler(token string, book port.Inbox, next http.Handler) http.Handler` — `POST /e2e/notice`(本文は `port.Notice` の JSON。トークンが違えば 401、POST でなければ 405、読めなければ 400、成功は 204)

- [ ] **Step 1: 場面テストを書く**(本物の gitstore と state.db と `inbox.Service`。C4c1 の `overwriteTrio` を使う)
  - `TestRevertOverwrite`: A がステータスを InProgress、B が同時に Done(あと)→ A の上書きの行が有効、`Revert` で A の HH-2 が InProgress、同期で B も InProgress。B には上書きの行が出ない(B の値はもう負けていない。戻しは新しい保存)
  - `TestRevertLosesAgain`(Review Focus 1): B の時計が A より進んでいる。A が戻したあと、B が同じ項目をまた変えて同期 → A に 2 行目の上書き、1 行目は `Result == applied`
- [ ] **Step 2: e2e の入口を書く** — `TestNoticeHandler`: 401・405・400・204 と、204 のあと受信箱に 1 行
- [ ] **Step 3: e2e を書く**(`inbox.spec.ts` と同じく、テストごとのワークスペースで HH 番号を作り、`/e2e/notice` で上書きの行を入れる。行は `Text` に `unique` の名前を入れて探す)
  - 有効: いまのステータス Done、`Mine` Todo・`Kept` Done → 行に「あなたの値 Todo → 残った値 Done」。「自分の値に戻す」を押す → トースト「上書き前の値に戻しました」、ボタンの代わりに「自分の値に戻しました」、詳細パネルのステータスが Todo
  - 無効: 行を入れたあと API でステータスを InProgress にする → 「自分の値に戻す」が無効で「そのあとさらに変更されています(いまは InProgress)」
  - 閲覧のみ: readonly の Store のアイテムの行 → 無効で「閲覧のみの Store です」
  - 本文だけ: `Field: "place"` の行 → 値もボタンも出ない
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./cmd/hedgehug/ -run Revert && go test -tags e2e ./cmd/hedgehug/ -run NoticeHandler && make e2e-bin && (cd web && pnpm exec playwright test e2e/inbox-overwrite.spec.ts)` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add cmd/hedgehug/ web/e2e/inbox-overwrite.spec.ts
git commit -m "test: 自分の値に戻すを 2 台の場面と e2e で確かめる(C4c2)"
```

---

### Task 7: 仕様と計画の記録、全体の検証

**Files:**
- Modify: `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`(上書きの通知の表に「ワークスペースの設定・スプリントとリリースの定義・コメント・作業時間・メンバーの表示名 | 本文だけ | 出さない」の 1 行(C4c2-2))
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`(C4c2 の行の中身を、この計画の範囲に合わせる)
- Modify: この計画の「始める前に」(実装で変えたこと)

- [ ] **Step 1: 文書を直す**(上の 3 か所)
- [ ] **Step 2: 全体を確かめる** — Run: `make lint && make test && golangci-lint run --build-tags e2e ./... && make web-check` Expected: すべて成功
- [ ] **Step 3: コミット**

```bash
git add docs/
git commit -m "docs: C4c2 の決定を仕様と計画に記録する"
```
