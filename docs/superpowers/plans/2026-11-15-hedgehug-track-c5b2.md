# hedgehug トラック C5b2(削除された値を指す設定の同期の修復)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 同期の修復 (10) を作る。初期ステータス・分類の既定・初期優先度にした値の同時の削除を取り消す。削除された値を指すプロジェクト別のルール・種類ごとの既定のテンプレート・自動化のルールを直す。完了・中止の分類にされた初期ステータスを戻す。それぞれの「同期の修復」の通知を 1 回だけ出す。

**Architecture:** 見つけ方と直す行は `internal/domain/repair/values.go`(値の削除の取り消し・完了の初期ステータス)と `valuerefs.go`(削除された値を指すもの)に置く。修復の段 3 つ(`restoreReferenced`・`dropDanglingRefs`・`resetFinishedInitial`)は `internal/app/repairing/values.go` に置き、`steps` の `dropOrphanRules` のあとに、この順で足す(06 `:353`)。値の削除の取り消しは、削除と変更の重なりと同じ `undo`(削除の op 全体の `change.Undo`)を使う。通知は C4b2 の仕組みに足す。取り消しの本文は既存の `restores` の中で分け、ほかは `noticing/repair_values.go` の 2 つの種類にする。`port` と gitstore は変えない。

**Tech Stack:** Go 1.27.3(go-git v5、modernc.org/sqlite)。web は変えない。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`
  - `:268-270`(削除された値を指す設定)
  - `:272-287`(削除された値を指すプロジェクト別のルール)
  - `:297-299`(外された初期ステータス・既定のステータス)
  - `:301-303`(完了・中止の分類にされた初期ステータス)
  - `:347-353`(修復の決まり・順)
  - `:455`(開く先)
  - `:510-543`(アクティビティの文)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/03-workspace-settings.md`
  - `:300-334`(分類ごとの既定・初期ステータス)
  - `:336-340`(初期優先度)
  - `:346-371`(値の削除で同時に変わるもの)
  - `:467-469`(種類ごとの既定)
  - `:546-584`(プロジェクト別のルール)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` `:211-213`(同期の修復の表の (10))
- `docs/superpowers/plans/2026-11-15-hedgehug-track-c5b1.md`(C5b1-11・C5b1-15。この計画はその隣に同じ形で足す)
- `2026-11-14-hedgehug-track-c3a.md`(C3a-9・C3a-18)、`2026-11-14-hedgehug-track-c4b2.md`(C4b2-6〜C4b2-12)
- `2026-11-12-hedgehug-track-c.md` の C5b2 の行

## Global Constraints

土台・トラック C・C4a・C4b1・C4b2・C5a・C5b1 の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限は `internal/archtest/size.go` のとおり。
  - Go ファイルは 300 行、テストは 600 行まで。関数は 60 行、認知的複雑度は 15 まで。
  - パッケージのテスト以外のファイルは 20 個まで。`internal/infra/gitstore`(直下)と `internal/app/port` は満杯。この計画では触らない。
  - 足すのは `domain/repair/values.go`・`domain/repair/valuerefs.go`・`app/repairing/values.go`・`app/noticing/repair_values.go` だけにする(`noticing` は 17 → 18 個)。
- app の機能どうしは import しない。`noticing` は `repairing` を import しない。
- 修復は 1 件を 1 回の保存にする(C3a-18)。実行者は `change.ActorSync`。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(`go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 修復の保存の件名(06 `:510-543`。名前は改行と続いた空白を 1 つの空白にする):
  - 値の削除の取り消し: 「「{値の名前}」の削除を取り消し」(既存の `summary` のまま)
  - プロジェクト別のルール: 「プロジェクト「{タイトル}」のルールから「{値の名前}」を削除(削除された値)」
  - ワークスペースの種類ごとの既定: 「{種類} の既定のテンプレートを変更(削除された値)」
  - ルール: 「自動化「{名前}」を無効にする(削除された値)」
  - 完了・中止の初期ステータス: 「初期ステータスを「{未着手の既定}」に戻し」
- 通知の本文({…} はそのまま置き換える。助詞の前の空白は既存の `withParticle` の規則):
  - 既定にした値の削除の取り消し: 「「{値の名前}」は {変更した人} が同時に既定にしたため、削除を取り消しました」
  - 分類の最後のステータスの削除の取り消し: 「「{値の名前}」は{分類}の最後のステータスのため、削除を取り消しました」
  - プロジェクト別のルール: 「プロジェクト「{タイトル}」の{ワークフローの制約|使うテンプレート|{種類}の既定のテンプレート}から、{削除した人} が「{値の名前}」を削除したため、「{値の名前}」を外しました」
  - ワークスペースの種類ごとの既定: 「{種類}の既定のテンプレートから、{削除した人} が「{値の名前}」を削除したため、「なし」に戻しました」
  - ルール: 「自動化「{名前}」は、{削除した人} が「{値の名前}」を削除したため、無効にしました」
  - 完了・中止の初期ステータス: 「「{X}」は {変更した人} が{分類}の分類にしたため、初期ステータスを「{未着手の既定}」に戻しました」
- C4d(アクティビティ)・C5d(時計のずれ)は並行で進む。`noticing` の `kinds` や `repairing` の `steps` が重なったら、develop にあとから入る方が合わせる。
- 手元の検証は 1 回で足りる(CI でも流れる)。`make lint`・`make test`・e2e のビルドの lint(`golangci-lint run --build-tags e2e ./...`)。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| C5b2-1 | 範囲は 06 の 4 節。<br>・削除された値を指す設定<br>・削除された値を指すプロジェクト別のルール(種類ごとの既定のテンプレート・自動化のルールを含む)<br>・外された初期ステータス・既定のステータス(直さないことの確認だけ)<br>・完了・中止の分類にされた初期ステータス | ユーザーの決定(2026-11-15、C5b を 2 本に分ける)。トラック C の計画の C5b2 の行 |
| C5b2-2 | 段の順は、値の削除の取り消し → 削除された値を指すもの → 完了・中止の初期ステータス。どれも `dropOrphanRules` のあと。<br>取り消した値は指されたままでよくなる。未着手の既定は、初期ステータスを戻す前にあるようにしておく | 06 `:353` の (10) の中の並び |
| C5b2-3 | 値の削除の取り消しは、削除と変更の重なりと同じく、削除の op 全体を `change.Undo` で取り消す(既存の `undo`)。<br>その削除で書き換えたアイテムのステータス・ワークフロー・ルールも戻る。件名も既存の `summary` | 06 `:270`「削除と変更の重なりと同じく、削除を取り消して値を残す」。保存形式 `:212` の文をこれに合わせる(Task 7) |
| C5b2-4 | 取り消す削除の見つけ方(状態で決める)。<br>・指している: 初期ステータス・`category_default/<分類>`(4 つ)・初期優先度の値が、取り消されていない `delete` のある値(`statuses/<ID>`・`priorities/<ID>`)。<br>・分類が 0 個: 削除済みを含めて数えたその分類のステータスが、すべて削除されている(上で取り消すものは残るものに数える)。<br>・分類が 0 個のときに取り消すのは、`Facts` の自分の側(あとから同期した方)の削除。自分の側になければ相手の側の削除、どちらにもなければ `at` が最もあと(同じなら ID の大きい方)の削除。どの側でも、その中で `at` が最もあと(同じなら ID の大きい方) | 06 `:270`。`Facts` が空のとき(合わせのない同期・手での書き換え)も同じ答えにするため |
| C5b2-5 | 分類の既定・初期優先度のステータスの分類が、同時の変更で合わなくなったもの(既定の分類の変更と、別の人の既定の指定が重なった)は直さない | 06 `:349`(上の各節以外のずれは同期では直さず、診断で見つける) |
| C5b2-6 | 削除された値を指すものの直し方。値は、取り消されていない `delete` のある `statuses/<ID>`・`templates/<ID>`。<br>・プロジェクト別のルール: (プロジェクト, 値) ごとに 1 回の保存。行は `projectrule.StatusDeleteDrafts`・`TemplateDropDrafts` に、そのプロジェクトだけの `Set` を渡したもの。同じプロジェクトで値の ID の順に先の値の保存が持つ行(遷移の両端が削除されたときの同じ `remove`)は、あとの値の保存から除き、行がなくなった保存は書かない。<br>・ワークスペースの種類ごとの既定: 種類ごとに 1 回の保存で `set template_default/<種類>`(値 → null)。既定は削除済みのテンプレートを隠すので、`workspace.SettingsReviving` で読む。<br>・ルール: 有効なルールのうち `automation.Statuses` に削除された値があるものを、ルールごとに 1 回の保存で `automation.Drafts(r, r の Enabled=false)`。無効のルールは書かない。<br>・並びは、プロジェクト(内部 ID 順、その中は値の ID 順)、種類(`workspace.Kinds` の順)、ルール(内部 ID 順) | 06 `:274-283`、保存形式 `:211`。件名が 1 つの値・種類・ルールを指す(06 `:540-542`)。値の削除の追従(B5a-16・B3-12)と同じ行 |
| C5b2-7 | ワークスペースのワークフロー(`workflow/<From>`)が削除された値を指しても直さない。`Settings.Allowed` はいまあるステータスだけを数えるので、見え方は削除の追従と同じになる | 06 `:276-282` の表にない。06 `:349` |
| C5b2-8 | プロジェクト別のルールの修復の行は、使わないステータスも同じ保存に入れる。通知は、区画(ワークフローの制約・使うテンプレート・種類ごとの既定)ごとに作り、使わないステータスでは作らない | 06 `:278` |
| C5b2-9 | 取り消しの通知の本文の分け方(`restores` の中)。取り消した削除の行がワークスペースの値(`statuses/`・`priorities/`)で、修復の直前のログで次に当たれば (10) の本文、どちらにも当たらなければ既存の本文。<br>1. 指している(C5b2-4) → 既定の本文。{変更した人} は、その項目をその値にした人の `set` の最後の行の人(なければ「不明なユーザー」)。項目は、初期ステータス・分類の既定(分類の順)・初期優先度の順で最初に当たるもの。<br>2. 分類が 0 個 → 分類の本文。<br>鍵は既存の `repair:restore:{op}` のまま | 06 `:270`。同じ削除を重なりの修復と (10) の両方が指すのは、別の人が同時に既定にしたときだけなので、(10) の本文で事実と合う |
| C5b2-10 | 削除された値を指すものの通知(`valueRefs`)の見分け方: 修復の保存のワークスペースのログの行のうち、外した値(行の `from`。ワークフローの行は `from` か、項目の From)が、修復の直前のログで削除されているもの。<br>プロジェクト別のルールは、持ち主のプロジェクトが保存のあとでそのワークスペースにあるときだけ(C5b1-11 の反対)。ルールは `set rules/<R>/enabled` の true → false | (9) の通知と取り違えないため |
| C5b2-11 | 宛先と {削除した人}。{削除した人} は、値を覆う削除の行(`workspace.Covering`)の最初の行の `by`。<br>・プロジェクト別のルール: C5b1-15 の `putter`(足した人・既定を選んだ人)<br>・ワークスペースの種類ごとの既定: その項目をその値にした人の `set` の最後の行の人<br>・ルール: そのルールの行(delete・restore を除く)を最後に保存した人(06「作った(編集した)人」)。値が 2 つ削除されていたら、`automation.Statuses` の順で最初のもの<br>・完了・中止の初期ステータス: 初期ステータスを X にした人の `set` の最後の行の人。{変更した人} は X の分類の `set` の最後の行の人 | 06 `:270`・`:278-282`・`:303` |
| C5b2-12 | 鍵(修復の op を含めない。同じ修復を何台が書いても 1 件)。<br>・プロジェクト別のルール: `repair:valueref:{P}:{workflow\|templates\|template_default/<種類>}:{削除の op}`<br>・ワークスペースの既定: `repair:valueref:{ワークスペース}:template_default/<種類>:{削除の op}`<br>・ルール: `repair:valueref:rule:{R}:{削除の op}`<br>・完了・中止の初期ステータス: `repair:initial:{ワークスペース}:{初期ステータスを X にした set の行の ID}` | 06 `:287`(1 台につき 1 件)。C5a の鍵(人の行の ID を使う)と同じ作り |
| C5b2-13 | 開く先(06 `:455`)。<br>・プロジェクト別のルール: プロジェクト(`entityOf` のプロジェクトの下地)<br>・ワークスペースの既定: `Section: "templates"`<br>・ルール: `Rule: R`・`Section: "automation"`<br>・完了・中止の初期ステータス: `Section: "statuses"` | C4b2-10・C4c1-8 |
| C5b2-14 | 外された初期ステータス・既定のステータスは、修復の保存も通知も作らない。確かめるのはテストだけ | 06 `:299` |
| C5b2-15 | e2e は足さない。2 台の場面は、`cmd/hedgehug` の本物の gitstore と state.db で確かめる。合わせたあと直す前の状態でも、同期が止まらないことも確かめる | C5b1-14 と同じ |
| C5b2-16 | 分類の最後のステータスの本文の {分類} の前後には空白を置かない(「「Done」は完了の最後のステータスのため、削除を取り消しました」)。助詞の前の空白は既存の withParticle の規則(英数字で終わる呼び方のときだけ空白)に従う | 06 :303 の「が{分類}の分類」と同じ書き方。06 :270 の空白は、ほかの本文の書き方と合わない |

## Review Focus

1. **取り消しで戻るアイテム:** 花子さんがステータス「Review」を削除し、「Review」だったアイテム HH-3 が「Todo」になった。同じころ太郎さんが「Review」を進行中の既定にした。同期のあと、「Review」は残り、HH-3 は「Review」に戻る。
   - テスト: Task 2 の `TestRestoreReferencedUndoesReplacements`
2. **2 台が同じ修復を書いたとき:** 太郎さんと花子さんのマシンが同時に同じ修復を書いても、受信箱は 1 行。
   - テスト: Task 5 の `TestValueRefsOnceAcrossMachines`
3. **無効のルール:** 削除されたステータスを指すルールが無効なら、修復の保存を書かない。何回同期しても書かない。
   - テスト: Task 3 の `TestDropDanglingRefsSkipsDisabledRule`
4. **(9) と (10) の取り違え:** プロジェクト「Web」が同じ Store の別のワークスペースへ移り、「メイン」に残った「Web」のルールが削除済みのステータスを指していた。(9) の保存だけを書き、(10) の通知は出ない。
   - テスト: Task 3 の `TestDropDanglingRefsAfterOrphans`、Task 5 の `TestValueRefsSkipsOrphanRepair`
5. **使うテンプレートの限定が空になったとき:** 「Web」の使うテンプレートが削除済みの「バグ報告」だけだった。修復のあと、限定がなくなり、すべてのテンプレートが使える。
   - テスト: Task 1 の `TestDanglingRefsTemplateLimitLifted`

---

### Task 1: 見つけ方と直す行(`domain/repair/values.go`・`valuerefs.go`)

**Files:**
- Create: `internal/domain/repair/values.go`、Test: `internal/domain/repair/values_test.go`
- Create: `internal/domain/repair/valuerefs.go`、Test: `internal/domain/repair/valuerefs_test.go`

**Interfaces:**
- Produces(`repair`、values.go):
  - `type RestoreReason struct { Field string; Category workspace.Category }`: 指している項目(`initial_status`・`category_default/<分類>`・`initial_priority`)か、0 個になる分類。どちらか一方だけを持つ。
  - `func ReasonOf(log []change.Record, f string) (RestoreReason, bool)`: 削除の行の f(`statuses/<ID>`・`priorities/<ID>`)の取り消しの理由(C5b2-4・C5b2-9 の順)。どちらでもなければ false。
  - `func ValueRestores(log []change.Record, f Facts) []Overlap`: 取り消す削除(C5b2-4)。指しているものを先に数える。結果は Op の辞書順で、同じ op は 1 つ。
  - `func FinishedInitial(log []change.Record) (change.Draft, bool)`: 初期ステータスの分類が完了・中止なら `set initial_status`(X → 未着手の既定)。未着手の既定がない・X と同じなら false。
- Produces(`repair`、valuerefs.go):
  - `type RefKind int`(`RefProject`・`RefTemplateDefault`・`RefRule`)
  - `type RefRepair struct { Kind RefKind; Project project.ID; Value change.ID; TemplateKind string; Rule change.ID; Drafts []change.Draft }`
  - `func DanglingRefs(log []change.Record) []RefRepair`: C5b2-6 の対象・行・並び。行がないものは入れない。

- [ ] **Step 1: 失敗するテストを書く**
  - values_test.go:
    - `TestReasonOf`(表): 「Review」(進行中)を削除したログで、
      - 初期ステータスが「Review」→ `Field == "initial_status"`
      - 進行中の既定が「Review」→ `Field == "category_default/doing"`
      - 優先度「High」の削除で初期優先度が「High」→ `Field == "initial_priority"`
      - 進行中のステータスが「Review」と「Doing」で、両方削除 → `Category == CategoryDoing`
      - どれでもない → false
    - `TestValueRestoresReferenced`: 初期ステータスが削除された「Review」 → 「Review」の削除の op が 1 つ。
    - `TestValueRestoresEmptyCategory`(表): 完了が「Done」「Closed」で、「Done」を自分の側、「Closed」を remote の側で削除。
      - `Facts` あり → 自分の側の「Done」の削除だけ。
      - `Facts` が空 → `at` があとの方の削除だけ。
      - 既定が指していて取り消す「Done」があれば、分類の分は足さない。
    - `TestFinishedInitial`(表): 初期ステータス「Review」の分類が完了 → `set initial_status`(Review → Todo)。中止も同じ。進行中 → false。
  - valuerefs_test.go:
    - `TestDanglingRefs`: 「Review」と「バグ報告」を削除したログ。次のものがある。
      - 「Web」(P1)の使わないステータス「Review」、ワークフロー Todo→Review と Review→Done、使うテンプレート「バグ報告」、Bug の既定「バグ報告」
      - ワークスペースの Bug の既定「バグ報告」
      - ルール R1(きっかけ「Review」、有効)
      - 期待: `{RefProject, P1, Review}`(行は `remove hidden_statuses Review`・`remove workflow/Review Done`・`remove workflow/Todo Review`)、`{RefProject, P1, バグ報告}`(`remove templates`・`set template_default/Bug → null`)、`{RefTemplateDefault, Bug}`、`{RefRule, R1}`(`set rules/R1/enabled true → false`)の順。
    - `TestDanglingRefsSkipsDisabledRule`: R1 が無効 → R1 は入らない。
    - `TestDanglingRefsTemplateLimitLifted`: 使うテンプレートが「バグ報告」だけ → 行を当てたあとの `projectrule.FromLog` の P1 の `Templates` が空。
    - `TestDanglingRefsRestored`: 削除が取り消し済み → 空。
    - `TestDanglingRefsHiddenInitial`: 使わないステータス「Review」が初期ステータス(削除されていない)→ 空(C5b2-14)。
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/domain/repair/` Expected: FAIL(`undefined: ReasonOf` ほか)
- [ ] **Step 3: 実装する** — 削除済みの値の名前と分類は `workspace.SettingsReviving`、削除の行は `workspace.Covering` で読む。初期ステータス・分類の既定・初期優先度は `workspace.SettingsFromLog` の値をそのまま使う(削除済みでも残る)。
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/domain/... && golangci-lint run ./internal/domain/...` Expected: PASS・指摘なし
- [ ] **Step 5: コミット**

```bash
git add internal/domain/repair
git commit -m "feat(repair): 削除された値を指す設定・ルールと、完了の初期ステータスを見つける規則を足す(C5b2)"
```

---

### Task 2: 値の削除の取り消し(`restoreReferenced`)

**Files:**
- Create: `internal/app/repairing/values.go`
- Modify: `internal/app/repairing/run.go`(`steps` の `dropOrphanRules` のあとに `restoreReferenced`)
- Test: `internal/app/repairing/values_test.go`(既存の `newEnv`・`e.write`・`e.run`・`wantOps` を使う)

**Interfaces:**
- Consumes: Task 1 の `repair.ValueRestores`、既存の `readSnapshot`・`undo`・`found`
- Produces: `func restoreReferenced(ctx context.Context, w port.RepairWriter) error`(`step`)。ワークスペースのエンティティごとに `ValueRestores(e.log, w.Facts())` を求め、1 つずつ `undo`。

- [ ] **Step 1: 失敗するテストを書く**
  - `TestRestoreReferencedDefault`: remote の側で「Review」を削除し、自分の側で「Review」を進行中の既定にした。
    - 保存は 1 つで、件名は「「Review」の削除を取り消し」。
    - `workspace.SettingsFromLog` に「Review」がある。
    - もう一度 `Run` しても何も書かない。
  - `TestRestoreReferencedUndoesReplacements`: 削除の op で HH-3 を Review → Todo にしていた → 取り消しのあと HH-3 は「Review」。
  - `TestRestoreReferencedInitialPriority`: 初期優先度にした「High」の同時の削除 → 取り消す。
  - `TestRestoreReferencedEmptyCategory`: 完了の「Done」を自分の側、「Closed」を remote の側で削除(既定の付け替えと同時)→ 自分の側の削除だけを取り消す。
  - `TestRestoreReferencedAfterOverlap`: 名前の変更と重なった削除を、`restoreOverlaps` がもう取り消した → この段は何も書かない。
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/repairing/` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/repairing/` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/repairing
git commit -m "feat(repairing): 初期値・既定にした値の同時の削除を同期で取り消す(C5b2)"
```

---

### Task 3: 削除された値を指すものと、完了の初期ステータスの段

**Files:**
- Modify: `internal/app/repairing/values.go`(`dropDanglingRefs`・`resetFinishedInitial`)
- Modify: `internal/app/repairing/run.go`(`restoreReferenced` のあとに、この順で 2 つ)
- Test: `internal/app/repairing/values_test.go`

**Interfaces:**
- Consumes: Task 1 の `repair.DanglingRefs`・`repair.FinishedInitial`、`Reader` の `Workspaces()`・`Changes()`・`Projects()`
- Produces:
  - `func dropDanglingRefs(ctx context.Context, w port.RepairWriter) error`: ワークスペースの順に、`RefRepair` 1 つを 1 回の保存で `tx.WriteWorkspaces().Append(ws, r.Drafts...)`。件名は Global Constraints の文。{タイトル} はプロジェクトのタイトル(読めなければ内部 ID)、{値の名前} は `workspace.NameOfDeleted`。
  - `func resetFinishedInitial(ctx context.Context, w port.RepairWriter) error`: ワークスペースごとに `FinishedInitial` があれば 1 回の保存。

- [ ] **Step 1: 失敗するテストを書く**
  - `TestDropDanglingRefs`: Task 1 の `TestDanglingRefs` の場面。件名が次の順になる。
    - 「プロジェクト「Web」のルールから「Review」を削除(削除された値)」
    - 「プロジェクト「Web」のルールから「バグ報告」を削除(削除された値)」
    - 「Bug の既定のテンプレートを変更(削除された値)」
    - 「自動化「レビューで通知」を無効にする(削除された値)」
    - もう一度 `Run` しても何も書かない。
  - `TestDropDanglingRefsSkipsDisabledRule`: ルールが無効 → 何も書かない。2 回 `Run` しても同じ。
  - `TestDropDanglingRefsAfterOrphans`: 「Web」が同じ Store の「サブ」へ移っていて、「メイン」に残った「Web」のルールが削除済みの「Review」を指す → (9) の件名だけ。
  - `TestResetFinishedInitial`: 初期ステータス「Review」の分類が完了 → 件名「初期ステータスを「Todo」に戻し」、初期ステータスが「Todo」、「Review」の分類は完了のまま。
  - `TestValuesStepOrder`: 1 回の `Run` で、既定の「Review」の削除の取り消し、「バグ報告」のルールの修復、完了の初期ステータスの戻しが要る場面。件名がこの順になる。
  - `TestHiddenInitialNotRepaired`: 「Web」の使わないステータス「Review」を、別の側で初期ステータスにした → 何も書かない(C5b2-14)。
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/repairing/` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/repairing/ ./internal/infra/gitstore/...` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/repairing
git commit -m "feat(repairing): 削除された値を指すルールを直し、完了の初期ステータスを戻す(C5b2)"
```

---

### Task 4: 取り消しの通知の本文を分ける(`restores`)

**Files:**
- Create: `internal/app/noticing/repair_values.go`(`restoreText`)
- Modify: `internal/app/noticing/repair_restore.go`(`restores` の本文の組み立てを `restoreText` に置き換えるところだけ)
- Test: `internal/app/noticing/repair_values_test.go`(`fixture` の `repairSave` を使う)

**Interfaces:**
- Consumes: Task 1 の `repair.ReasonOf`、既存の `memberName`・`lastSaver`・`rowsWhere`・`sameValue`・`withParticle`
- Produces: `func restoreText(r port.Reader, ref change.Ref, before []change.Record, d change.Record, target string, sides repair.Facts) (string, error)`(C5b2-9)。ワークスペースの値でなければ既存の本文。

- [ ] **Step 1: 失敗するテストを書く**(あなたは `hanako`、削除したのは hanako)
  - `TestRestoreTextDefault`: taro が「Review」を進行中の既定にした → 「「Review」は 太郎 が同時に既定にしたため、削除を取り消しました」・`Source == "repair:restore:{op}"`・`Section == "statuses"`
  - `TestRestoreTextEmptyCategory`: 完了の最後の「Done」 → 「「Done」は完了の最後のステータスのため、削除を取り消しました」
  - `TestRestoreTextInitialPriority`: 「「High」は 太郎 が同時に既定にしたため、削除を取り消しました」・`Section == "priorities"`
  - `TestRestoreTextOverlapUnchanged`: 名前の変更と重なった取り消し → 既存の「「Review」は 太郎 が同時に変更したため、削除を取り消しました」
  - `TestRestoreTextUnknownSetter`: 既定にした行が人の行でない → 「「Review」は 不明なユーザー が同時に既定にしたため、削除を取り消しました」
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/noticing/ -run RestoreText` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/noticing/` Expected: PASS(既存の `restores` のテストも変わらず通る)
- [ ] **Step 5: コミット**

```bash
git add internal/app/noticing
git commit -m "feat(noticing): 既定にした値と分類の最後の値の削除の取り消しを、06 の本文で知らせる(C5b2)"
```

---

### Task 5: 削除された値を指すものと、完了の初期ステータスの通知

**Files:**
- Modify: `internal/app/noticing/repair_values.go`(`valueRefs`・`finishedInitial`)
- Modify: `internal/app/noticing/repair.go`(`kinds` の `movedRules` のあとに `valueRefs`・`finishedInitial`)
- Test: `internal/app/noticing/repair_values_test.go`

**Interfaces:**
- Consumes: 既存の `putter`・`lastSaver`・`rowsWhere`・`entityOf`・`memberName`・`nameOf`・`oneLine`・`same`、`workspace.Covering`・`workspace.NameOfDeleted`・`workspace.CategoryName`、`automation.FromLog`・`automation.Statuses`
- Produces:
  - `func valueRefs(r port.Reader, b batch, i int, me string) ([]port.Notice, error)`: 見分け方は C5b2-10、宛先は C5b2-11、鍵は C5b2-12、開く先は C5b2-13。プロジェクト別のルールは (プロジェクト, 区画, 値) ごとに 1 件。
  - `func finishedInitial(r port.Reader, b batch, i int, me string) ([]port.Notice, error)`: 修復の保存の `set initial_status` の `from` の分類が、修復の直前のログで完了・中止なら 1 件。

- [ ] **Step 1: 失敗するテストを書く**(あなたは `hanako`、削除したのは taro)
  - `TestValueRefsWorkflow`: hanako が「Web」に Todo→Review のチェックを足していた → 「プロジェクト「Web」のワークフローの制約から、太郎 が「Review」を削除したため、「Review」を外しました」・`Project == P1`・`Source == "repair:valueref:{P1}:workflow:{削除の op}"`
  - `TestValueRefsHiddenOnly`: 使わないステータスだけを外した → 作らない。
  - `TestValueRefsTemplates`(表): 使うテンプレート → 「…の使うテンプレートから、…」、Bug の既定 → 「…のBugの既定のテンプレートから、…」。
  - `TestValueRefsWorkspaceDefault`: hanako が Bug の既定にしていた → 「Bugの既定のテンプレートから、太郎 が「バグ報告」を削除したため、「なし」に戻しました」・`Section == "templates"`
  - `TestValueRefsRule`: hanako が最後に編集したルール → 「自動化「レビューで通知」は、太郎 が「Review」を削除したため、無効にしました」・`Rule == R1`・`Section == "automation"`
  - `TestValueRefsRecipient`: 足したのが taro → 作らない。
  - `TestValueRefsSkipsOrphanRepair`: (9) の保存(「Web」が同じ Store の別のワークスペース)→ 作らない。
  - `TestValueRefsOnceAcrossMachines`: 行の ID が違う同じ修復の保存を 2 つ判定する → 受信箱は 1 行。
  - `TestFinishedInitialNotice`: hanako が「Review」を初期ステータスにし、taro が完了の分類にした → 「「Review」は 太郎 が完了の分類にしたため、初期ステータスを「Todo」に戻しました」・`Section == "statuses"`・`Source == "repair:initial:{ws}:{set の行の ID}"`。初期ステータスにしたのが taro → 作らない。
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/noticing/ -run 'ValueRefs|FinishedInitial'` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/... && golangci-lint run ./internal/...` Expected: PASS・指摘なし(C5b1 の `TestOrphanRulesSkipsValueRepair` も変わらず通る。(10) の保存で (9) の通知が出ない)
- [ ] **Step 5: コミット**

```bash
git add internal/app/noticing
git commit -m "feat(noticing): 削除された値を指すルールの修復と、完了の初期ステータスの戻しを知らせる(C5b2)"
```

---

### Task 6: 2 台の場面テスト

**Files:**
- Create: `cmd/hedgehug/values_repair_scenario_test.go`

**Interfaces:**
- Consumes:
  - Task 1〜5
  - `cmd/hedgehug/repair_helpers_test.go` の `firstMachine`・`cloneMachine`・`writeAs`・`sync`・`noticed`・`sameFiles`・`newTaskAs`
  - `repair_notice_scenario_test.go` の `wantRepair`・`wantNoRepair`
- 値の削除は、`writeAs` の中で、`app/workspaces` の `DeleteValue` と同じ行(値の `delete`、アイテムの `set status`、`workspace.WorkflowDeleteDrafts`、`automation.StatusDeleteDrafts`、`projectrule.StatusDeleteDrafts`)を書く(`app/workspaces` は組み立てない)。

- [ ] **Step 1: 場面テストを書く**(A = taro、B = hanako。本物の gitstore とテストごとの state.db)
  - `TestRepairValuesDefaultRestored`:
    1. A がワークスペース「メイン」とステータス「Review」(進行中)とアイテム HH-3(「Review」)を作って同期する。B が clone する。
    2. A が「Review」を進行中の既定にする。B は同時に「Review」を削除する(HH-3 は「Todo」)。
    3. A が同期し、B が同期し、A がもう一度同期する。
    - 期待: どの同期もエラーにならない。2 台とも「Review」があり、HH-3 は「Review」。2 台のファイルが同じ。B に「「Review」は 太郎 が同時に既定にしたため、削除を取り消しました」が 1 行。A には出ない。
  - `TestRepairValuesRules`:
    1. A が「メイン」とプロジェクト「Web」を作って同期する。B が clone する。
    2. A が「Web」に Todo→Review のチェックと、きっかけ「Review」のルール「レビューで通知」を足す。B は同時に「Review」を削除する。
    3. A が同期し、B が同期し、A が同期する。
    - 期待: 2 台の「Web」のワークフローの制約に「Review」がなく、ルールは無効。A に「プロジェクト「Web」のワークフローの制約から、花子 が「Review」を削除したため、「Review」を外しました」と「自動化「レビューで通知」は、花子 が「Review」を削除したため、無効にしました」が 1 行ずつ。B には出ない。
  - `TestRepairValuesFinishedInitial`: A が「Review」を完了の分類にし、B が同時に「Review」を初期ステータスにする。同期のあと、初期ステータスは「Todo」。B に「「Review」は 太郎 が完了の分類にしたため、初期ステータスを「Todo」に戻しました」。
  - `TestRepairValuesHiddenInitial`: A が「Web」で「Review」を使わないにし、B が同時に「Review」を初期ステータスにする。同期のあと、修復の保存はなく(アクティビティに同期の修復の行がない)、通知もない。「Web」の使わないステータスに「Review」が残る。
- [ ] **Step 2: 流して、足りない組み立てを直す** — Run: `go test ./cmd/hedgehug/ -run 'RepairValues'` Expected: PASS
- [ ] **Step 3: 全体を確かめる** — Run: `go test ./... && golangci-lint run ./... && golangci-lint run --build-tags e2e ./...` Expected: PASS・指摘なし
- [ ] **Step 4: コミット**

```bash
git add cmd/hedgehug
git commit -m "test(sync): 2 台の場面で、削除された値を指す設定の修復と通知を確かめる(C5b2)"
```

---

### Task 7: 保存形式・トラック C の計画の記録と全体の検証

**Files:**
- Modify: `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`
  - `:211` の書く行: 「ルールは `set` で無効にする」を「ルールは、有効なものだけ `set` で無効にする」にする。
  - `:212` の書く行: 「値の `restore`(分類が 0 個になるときは、あとから同期した方の削除)」を「その削除の `op` 全体の `restore`(削除と変更の重なりと同じ。分類が 0 個になるときは、あとから同期した方(合わせの自分の側)の削除。自分の側になければ相手の側の削除、どちらにもなければ `at` が最もあとの削除)」にする。
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`
  - C5b2 の行に「(計画: `2026-11-15-hedgehug-track-c5b2.md`。範囲は C5b2-1)」を足す。

- [ ] **Step 1: 保存形式とトラック C の計画を書き直す**(上のとおり)。
- [ ] **Step 2: 全体を確かめる** — Run: `make lint && make test && golangci-lint run --build-tags e2e ./...` Expected: すべて PASS
- [ ] **Step 3: コミット**

```bash
git add docs
git commit -m "docs: 削除された値を指す設定の修復(C5b2)を保存形式とトラック C の計画に書く"
```
