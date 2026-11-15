# hedgehug トラック C5c1(名前・接頭辞・キーの重複の同期の修復)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 同期の修復 (11) のうち、名前 8 種(ワークスペース・プロジェクト・プロパティ・値・テンプレート・ルール・スプリント・リリース)・接頭辞・プロパティのキーの重複を直す。あとから同期した方の名前に「 (2)」、接頭辞とキーに数字を付け、直したものを作った人に「同期の修復」の通知を 1 回だけ出す。同期でワークスペースの削除を取り消して、名前・接頭辞が新しいワークスペースと重なった場合も同じ規則で直す。

**Architecture:** 重なりの見つけ方と新しい値の決め方は `internal/domain/repair/duplicates.go` に置く。残す 1 件は番号の重なり(`numbers.go` の `moved`)と同じ規則で、その規則を共通の関数にして両方から使う。修復の段 `renameDuplicates` は `internal/app/repairing/duplicates.go`(ワークスペース・プロジェクト)と `duplicates_settings.go`(ワークスペースのログの中の名前・キー)に置き、`steps` の最後(`resetFinishedInitial` のあと)に足す。通知は `internal/app/noticing/repair_duplicates.go` の `duplicates` を `kinds` の最後に足す。`port` と gitstore は変えない。

**Tech Stack:** Go 1.27.3(go-git v5、modernc.org/sqlite)。web は変えない。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`
  - `:254-266`(1 つしかないものが 2 つになったとき)
  - `:347-353`(修復の決まり・順)
  - `:458`(開く先)
  - `:518`(アクティビティの文「{種類}「{旧}」を「{新}」に変更」)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/03-workspace-settings.md`
  - `:65-67`(名前の重複・比べ方)
  - `:75`・`:84-85`・`:107-117`(接頭辞の形・旧い接頭辞)
  - `:223-236`(プロパティのキー・予約語)
  - `:283`(値の名前は大文字と小文字を区別しない)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md` `:531-533`(プロジェクトの名前)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/05-planning-reports.md` `:82`・`:167`(スプリント・リリースの名前)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` `:214`(同期の修復の表の (11))
- `docs/superpowers/plans/2026-11-14-hedgehug-track-b5b.md` `:54`(ワークスペースの削除の取り消しとの重なり)
- `2026-11-14-hedgehug-track-c3a.md`(C3a-13 番号の重なりの残す 1 件、C3a-18 1 件 1 保存)、`2026-11-14-hedgehug-track-c4b2.md`(C4b2-6〜C4b2-12 通知の仕組み)、`2026-11-15-hedgehug-track-c5b2.md`(隣の段と通知の形)
- `2026-11-12-hedgehug-track-c.md` の C5c の行

## Global Constraints

土台・トラック C・C4a・C4b1・C4b2・C5a・C5b1・C5b2 の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限は `internal/archtest/size.go` のとおり。
  - Go ファイルは 300 行、テストは 600 行まで。関数は 60 行、認知的複雑度は 15 まで。
  - パッケージのテスト以外のファイルは 20 個まで。`internal/infra/gitstore`(直下)と `internal/app/port` は満杯で、この計画では触らない。
  - 足すのは `domain/repair/duplicates.go`・`app/repairing/duplicates.go`・`app/repairing/duplicates_settings.go`・`app/noticing/repair_duplicates.go` だけにする(`noticing` は 18 → 19 個)。
- app の機能どうしは import しない。`noticing` は `repairing` を import しない。
- 修復は 1 件を 1 回の保存にする(C3a-18)。実行者は `change.ActorSync`。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(`go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 修復の保存の件名(06 `:518`。名前は改行と続いた空白を 1 つの空白にする): 「{種類}「{旧}」を「{新}」に変更」。{種類} は C5c1-7 の表。
- 通知の本文(06 `:266`): 「2 台で同時に「{旧}」が作られたため、あとから届いた方を「{新}」に変更しました」。接頭辞・キーも同じ文で、{旧}・{新} に接頭辞・キーを入れる。
- C4d(アクティビティ)・C5d(時計のずれ)は並行で進む。`noticing` の `kinds` や `repairing` の `steps` が重なったら、develop にあとから入る方が合わせる。
- 手元の検証は 1 回で足りる(CI でも流れる)。`make lint`・`make test`・e2e のビルドの lint(`golangci-lint run --build-tags e2e ./...`)。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| C5c1-1 | 範囲は名前 8 種・接頭辞・プロパティのキー。Issue のリンクとメンバーは C5c2 | ユーザーの決定(2026-11-15、C5c を 2 本に分ける) |
| C5c1-2 | 重なりを数える範囲と比べ方は、書き込みのときの検査と同じにする。<br>・ワークスペースの名前・接頭辞: Store の削除済みでないワークスペース<br>・プロジェクトの名前: 同じワークスペースの削除済みでないプロジェクト<br>・プロパティの名前: `Settings.Properties`(組み込み・非表示を含む)<br>・プロパティのキー: `Settings.Properties` の組み込みでないもの<br>・値の名前: 同じ一覧(ステータス・優先度・関連システム・プロパティごとの選択肢)。大文字と小文字を区別しない(`strings.EqualFold`)<br>・テンプレート: `Settings.Templates`<br>・ルール: `automation.FromLog`(専用ルールを含む。無効も数える)<br>・スプリント・リリース: `planning.FromLog` の `LiveSprints`・`LiveReleases`(完了・アーカイブを含む)<br>どれも削除済みは数えない。値の名前のほかは完全一致。比べる前に前後の空白を除く(各 `NormalizeName`) | 03 `:65`・`:283`、02 `:533`、05 `:82`。書き込みで断る範囲と同じにすれば、修復のあとの状態が書き込みの検査を通る |
| C5c1-3 | 「名前を決めた行」: その項目(`name`・`prefix`・`title`・`properties/<P>/name` など)の `change.Current(log, change.Set, f, …)` の行。`set` がなければ、その項目を作った `create` の行(ワークスペースの作成と一緒に作った初期の設定、プロジェクトの作成) | 名前の変更で重なった場合(A が「X」を「foo」に変え、B が「foo」を作る)も、重なりを作った行で決めるため |
| C5c1-4 | 残す 1 件と直すもの: 番号の重なりと同じ規則(C3a-13)を、名前を決めた行に当てる。名前を決めた行が自分の側(`Facts.Side == 1`)にないものの中で、行の `at` が最も古いもの(同じなら行の ID が小さいもの)を残す。どれも自分の側なら全体で最も古いもの。`numbers.go` の `moved` の規則を共通の `keeper` にして両方から使う | 06 `:256`「番号の重なりと同じ決め方」 |
| C5c1-5 | 新しい値の決め方。重なった値の辞書順に組を見て、組の中の直すものはエンティティの内部 ID の順に、n = 2 から 1 つずつ試し、範囲の中のいまの値と、この修復で先に決めた新しい値のどれとも重ならない最初の値にする(比べ方は C5c1-2)。<br>・名前: `{値} (n)`<br>・接頭辞: `{値}{n}`。10 文字を超えるときは、値の末尾を削って 10 文字にする(ABCDEFGHIJ → ABCDEFGHI2)<br>・キー: `{値}{n}`(予約語はどれも数字を含まないので、`{値}{n}` は予約語に当たらない) | 06 `:260-262`、03 `:232` |
| C5c1-6 | 段の順(すべて (10) の段のあと): ワークスペースの名前 → 接頭辞 → ワークスペースの内部 ID 順に、プロジェクト → プロパティの名前 → プロパティのキー → 値(ステータス・優先度・関連システム・選択肢をプロパティの並び順)→ テンプレート → ルール → スプリント → リリース | 06 `:353`。名前を先に直すので、同じ保存に 2 つの項目が入らない |
| C5c1-7 | 件名と通知の {種類}: ワークスペース・接頭辞・プロジェクト・プロパティ・プロパティのキー・ステータス・優先度・関連システム・選択肢・テンプレート・自動化・スプリント・リリース | 06 `:518` の {種類}。画面のサブタブ名とルールの呼び方(06 `:542`「自動化「{名前}」」)に合わせた |
| C5c1-8 | 書く行は `set {項目}`(from = いまの値、to = 新しい値)だけ。プロジェクトはプロジェクトのログ、ほかはワークスペースのログ。接頭辞を変えた旧い接頭辞は `workspace.OldPrefixes` がログから読むので、別の行は書かない。キーは作ったあとに画面からは変えられないが、`SettingsFromLog` は `set properties/<P>/key` を読む(`setProperty` の `key`)。アイテムの値は `fields/<プロパティの内部 ID>` で持ち(`item.FieldKey`)、キーは検索と移動の対応付けだけが使うので、キーの `set` 1 行で足りる | 保存形式 `:214`、03 `:115`・`:234`。ワークスペースのフォルダは内部 ID なので、同じ接頭辞が 2 つあっても直すまで読み書きできる |
| C5c1-9 | 通知の見分け方: 修復の保存(`Repair`)のうち `restore` の行を含まないものの `set` の行で、項目が C5c1-3 の項目のどれか。同期の修復でこれらの項目に `set` を書くのは (11) と、削除の取り消し(`change.Undo` は削除の op の中の `set` も戻す)だけで、取り消しの保存は必ず `restore` の行を含む | C4b2-8(保存の行で見分ける) |
| C5c1-10 | 宛先: 修復の直前のログ(`beforeSave`)での、名前を決めた行(C5c1-3)の `by`。実行者が人でない行なら作らない | 06 `:266`「直したものを作った人」。名前の変更で重なったときは、名前を変えた人に、06 の本文のまま出す(ユーザーの決定、2026-11-15) |
| C5c1-11 | 鍵: `repair:duplicate:{エンティティの ID}:{項目}:{名前を決めた行の ID}`(修復の op を含めない。2 台が同じ修復を書いても 1 件) | C5b2-12 と同じ作り |
| C5c1-12 | 開く先: プロジェクトはプロジェクト(`entityOf`)。ルールは `Rule: R`・`Section: "automation"`。ほかはワークスペースと既存の `section(f)`(名前・接頭辞は `general`、プロパティ・キー・選択肢は `properties`、値は一覧の区画、テンプレートは `templates`、スプリント・リリースはそのタブ) | 06 `:458`、C4b2-10 |
| C5c1-13 | 同期でワークスペースの削除を取り消して、名前・接頭辞が新しいワークスペースと重なった場合も、C5c1-4 で直す。取り消したワークスペースの名前を決めた行は共通の祖先にあるので、どちらのマシンが同期しても、新しいワークスペースの方が直る。プロジェクト・設定・スプリント・リリースの削除の取り消しも同じ | B5b `:54`(ユーザーの決定、2026-11-14) |
| C5c1-14 | 閲覧のみの Store では修復しない(既存の `Run` の呼び方のまま) | 06 `:349` |
| C5c1-15 | e2e は足さない。2 台の場面は `cmd/hedgehug` の本物の gitstore と state.db で確かめる | C5b2-15 と同じ |

## Review Focus

1. **すでに「 (2)」があるとき:** ワークスペース「メイン」に「開発」と「開発 (2)」があり、同期で 2 つ目の「開発」が届いた。直すのは「開発 (3)」。
   - テスト: Task 1 の `TestNameRenamesSkipTaken`
2. **10 文字の接頭辞:** 2 台で接頭辞 ABCDEFGHIJ を作った。直した接頭辞は ABCDEFGHI2 で、形の検査(`^[A-Z][A-Z0-9]{0,9}$`)を通る。
   - テスト: Task 1 の `TestPrefixRenamesTruncate`
3. **大文字と小文字だけが違う値:** 太郎さんがステータス「Review」、花子さんが「review」を同時に足した。値は大文字と小文字を区別しないので重なり、あとから届いた方が「review (2)」になる。プロジェクトの「Web」と「web」は重ならない。
   - テスト: Task 1 の `TestNameRenamesFold`、Task 3 の `TestRenameDuplicatesValuesFold`
4. **削除済みのものと同じ名前:** 削除済みのスプリント「Sprint 1」があるところへ、同期で「Sprint 1」が届いた。直さない。専用ルールとワークスペース全体のルールの同じ名前は直す。
   - テスト: Task 3 の `TestRenameDuplicatesSkipsDeleted`・`TestRenameDuplicatesScopedRule`
5. **2 台が同じ修復を書いたとき・2 回目の同期:** 受信箱は 1 行で、もう一度 `Run` しても何も書かない。
   - テスト: Task 2 の `TestRenameDuplicatesWorkspaces`(2 回目の `Run`)、Task 4 の `TestDuplicatesOnceAcrossMachines`

---

### Task 1: 重なりの見つけ方と新しい値(`domain/repair/duplicates.go`)

**Files:**
- Create: `internal/domain/repair/duplicates.go`、Test: `internal/domain/repair/duplicates_test.go`
- Modify: `internal/domain/repair/numbers.go`(`moved` の残す 1 件の選び方を `keeper` に置き換える)

**Interfaces:**
- Produces(`repair`):
  - `type Named struct { Ref change.Ref; Field string; Value string; Naming change.Record }`: 範囲の中のもの 1 つ。Value は前後の空白を除いた値、Naming は名前を決めた行(C5c1-3)。
  - `type Rename struct { Named; To string }`
  - `func NamingRow(log []change.Record, f string) (change.Record, bool)`: C5c1-3。
  - `func NameRenames(all []Named, f Facts, fold bool) []Rename`・`func PrefixRenames(all []Named, f Facts) []Rename`・`func KeyRenames(all []Named, f Facts) []Rename`: C5c1-4・C5c1-5。並びは C5c1-5 の順。重ならなければ空。
  - `func keeper[T any](g []T, row func(T) change.Record, f Facts) T`(パッケージの中だけ): C5c1-4。`moved` もこれを使う。

- [ ] **Step 1: 失敗するテストを書く**
  - `TestNameRenames`: 「開発」が 2 つ。名前を決めた行の一方が自分の側 → 自分の側のものが `To == "開発 (2)"`。`Facts` が空 → 行の `at` があとの方が直る。
  - `TestNameRenamesSkipTaken`: 「開発」2 つと「開発 (2)」→ `To == "開発 (3)"`。「開発」が 3 つ → 内部 ID の順に「開発 (2)」「開発 (3)」。
  - `TestNameRenamesFold`: `fold == true` で「Review」「review」→ 1 件直る。「Review (2)」がすでにあれば「review (3)」。`fold == false` → 空。
  - `TestPrefixRenames`: WEB が 2 つ → WEB2。WEB2 がすでにある → WEB3。
  - `TestPrefixRenamesTruncate`: ABCDEFGHIJ が 2 つ → ABCDEFGHI2。`workspace` の接頭辞の形の検査を通る。
  - `TestKeyRenames`: customer が 2 つ → customer2。customer2 がある → customer3。
  - `TestNamingRow`: `set name` があればその先端、なければ `create` の行。
  - 既存の `TestRenumbers` が変わらず通る。
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/domain/repair/` Expected: FAIL(`undefined: NameRenames` ほか)
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/domain/... && golangci-lint run ./internal/domain/...` Expected: PASS・指摘なし
- [ ] **Step 5: コミット**

```bash
git add internal/domain/repair
git commit -m "feat(repair): 重なった名前・接頭辞・キーの直すものと新しい値を決める規則を足す(C5c1)"
```

---

### Task 2: ワークスペースとプロジェクトの段(`renameDuplicates`)

**Files:**
- Create: `internal/app/repairing/duplicates.go`
- Modify: `internal/app/repairing/run.go`(`steps` の `resetFinishedInitial` のあとに `renameDuplicates`)
- Test: `internal/app/repairing/duplicates_test.go`(既存の `newEnv`・`e.write`・`e.run`・`wantOps` を使う)

**Interfaces:**
- Consumes: Task 1 の `repair.NamingRow`・`NameRenames`・`PrefixRenames`、`port.LiveWorkspaces`・`port.LiveProjects`
- Produces:
  - `func renameDuplicates(ctx context.Context, w port.RepairWriter) error`(`step`): C5c1-6 の順に、`Rename` 1 つを 1 回の保存で書く(C5c1-8)。件名は Global Constraints の文。
  - `type renameFix struct { ref change.Ref; kind string; rename repair.Rename }`(`kind` は C5c1-7 の {種類})と、ワークスペースごとの集め方の口 `func settingsRenames(r port.Reader, ws workspace.ID, log []change.Record) ([]renameFix, error)`(Task 3 が中身を書く。この Task では空を返す)。

- [ ] **Step 1: 失敗するテストを書く**
  - `TestRenameDuplicatesWorkspaces`: 自分の側でワークスペース「開発」(接頭辞 WEB)を作り、remote の側でも「開発」(WEB)を作った。
    - 保存は 2 つで、件名は「ワークスペース「開発」を「開発 (2)」に変更」「接頭辞「WEB」を「WEB2」に変更」の順。どちらも自分の側のワークスペース。
    - 直したワークスペースの WEB-1 のアイテムの ID が WEB2-1。`workspace.OldPrefixes` に WEB がある。
    - もう一度 `Run` しても何も書かない。
  - `TestRenameDuplicatesProjects`: 同じワークスペースに「Web」が 2 つ → 「プロジェクト「Web」を「Web (2)」に変更」(プロジェクトのログの `set title`)。別のワークスペースの「Web」は直さない。「Web」と「web」は直さない。
  - `TestRenameDuplicatesRenamed`: 共通の祖先にある「X」を自分の側で「開発」に変え、remote の側で「開発」を作った → 自分の側の「X」だったものが「開発 (2)」(C5c1-3)。
  - `TestRenameDuplicatesRevivedWorkspace`(C5c1-13): 共通の祖先にある「開発」(WEB)を remote の側で削除し、同時に自分の側でステータスを足した(`restoreOverlaps` が削除を取り消す)。remote の側では新しい「開発」(WEB)も作った → 新しいワークスペースが「開発 (2)」・WEB2。自分と remote の側を入れ替えても同じ。
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/repairing/` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/repairing/ ./internal/infra/gitstore/...` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/repairing
git commit -m "feat(repairing): 重なったワークスペースの名前・接頭辞とプロジェクトの名前を同期で直す(C5c1)"
```

---

### Task 3: ワークスペースのログの中の名前とキー(`settingsRenames`)

**Files:**
- Create: `internal/app/repairing/duplicates_settings.go`
- Modify: `internal/app/repairing/duplicates.go`(Task 2 の空の `settingsRenames` を消す)
- Test: `internal/app/repairing/duplicates_test.go`

**Interfaces:**
- Consumes: Task 1・Task 2、`workspace.SettingsFromLog`・`workspace.List.Field`・`automation.FromLog`・`automation.Field`・`planning.FromLog`・`workspace.TemplateField`
- Produces: `func settingsRenames(r port.Reader, ws workspace.ID, log []change.Record) ([]renameFix, error)`: C5c1-2 の範囲で、C5c1-6 の順(プロパティの名前〜リリース)。

- [ ] **Step 1: 失敗するテストを書く**(どれも、一方を自分の側、他方を remote の側で足す)
  - `TestRenameDuplicatesSettings`(表。件名で確かめる):
    - プロパティ「顧客」(キー customer)2 つ → 「プロパティ「顧客」を「顧客 (2)」に変更」「プロパティのキー「customer」を「customer2」に変更」の順で 2 つの保存。`SettingsFromLog` の直したプロパティの `Key == "customer2"`
    - 組み込み「担当者」を「Owner」に変え、別の側で「Owner」を足した → 足した方が直る
    - 選択肢「A」2 つ(同じプロパティ)→ 「選択肢「A」を「A (2)」に変更」。別のプロパティの「A」は直さない
    - テンプレート「バグ報告」2 つ → 「テンプレート「バグ報告」を「バグ報告 (2)」に変更」
    - ルール「通知」2 つ → 「自動化「通知」を「通知 (2)」に変更」
    - スプリント「Sprint 1」2 つ(一方は完了)→ 「スプリント「Sprint 1」を「Sprint 1 (2)」に変更」
    - リリース「v1」2 つ(一方はアーカイブ)→ 「リリース「v1」を「v1 (2)」に変更」
  - `TestRenameDuplicatesValuesFold`: ステータス「Review」と「review」→ 「ステータス「review」を「review (2)」に変更」。優先度・関連システムも同じ一覧の中だけ。
  - `TestRenameDuplicatesSkipsDeleted`: 削除済みのスプリント「Sprint 1」と「Sprint 1」→ 何も書かない。削除済みのプロパティのキー customer と customer → 何も書かない。
  - `TestRenameDuplicatesScopedRule`: 専用ルール「通知」とワークスペース全体のルール「通知」→ 直す。無効のルールも数える。
  - `TestRenameDuplicatesOrder`: 1 回の `Run` で、ワークスペースの名前・プロジェクト・ステータス・スプリントの重なりがある → 件名が C5c1-6 の順。
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/repairing/ -run RenameDuplicates` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/repairing/ && golangci-lint run ./internal/app/...` Expected: PASS・指摘なし
- [ ] **Step 5: コミット**

```bash
git add internal/app/repairing
git commit -m "feat(repairing): 重なった設定・ルール・スプリント・リリースの名前とプロパティのキーを同期で直す(C5c1)"
```

---

### Task 4: 重なりの修復の通知(`duplicates`)

**Files:**
- Create: `internal/app/noticing/repair_duplicates.go`
- Modify: `internal/app/noticing/repair.go`(`kinds` の `finishedInitial` のあとに `duplicates`)
- Test: `internal/app/noticing/repair_duplicates_test.go`(`fixture` の `repairSave` を使う)

**Interfaces:**
- Consumes: Task 1 の `repair.NamingRow`、既存の `beforeRead`・`entityOf`・`section`・`oneLine`
- Produces: `func duplicates(r port.Reader, b batch, i int, me string) ([]port.Notice, error)`: 見分け方は C5c1-9、宛先は C5c1-10、鍵は C5c1-11、開く先は C5c1-12。行ごとに 1 件。

- [ ] **Step 1: 失敗するテストを書く**(あなたは `hanako`)
  - `TestDuplicatesWorkspace`: hanako が作った「開発」を「開発 (2)」にした修復 → 「2 台で同時に「開発」が作られたため、あとから届いた方を「開発 (2)」に変更しました」・`Workspace == ws`・`Section == "general"`・`Source == "repair:duplicate:{ws}:name:{create の行の ID}"`
  - `TestDuplicatesPrefix`: 「2 台で同時に「WEB」が作られたため、あとから届いた方を「WEB2」に変更しました」
  - `TestDuplicatesProject`: `Project == P1`
  - `TestDuplicatesRule`: `Rule == R1`・`Section == "automation"`
  - `TestDuplicatesValue`(表): ステータス → `Section == "statuses"`、選択肢・キー → `"properties"`、スプリント → `"sprints"`
  - `TestDuplicatesRenamer`: taro が作り hanako が「開発」に名前を変えた → hanako に作る。taro には作らない
  - `TestDuplicatesRecipient`: 作ったのが taro → 作らない
  - `TestDuplicatesSkipsOtherRepairs`(表): (10) のルールの無効化(`set rules/R1/enabled`)の修復 → 作らない。`restore` の行と `set statuses/<V>/name` の行を含む取り消しの保存 → 作らない
  - `TestDuplicatesOnceAcrossMachines`: 行の ID が違う同じ修復の保存を 2 つ判定する → 受信箱は 1 行
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/noticing/ -run Duplicates` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/... && golangci-lint run ./internal/...` Expected: PASS・指摘なし
- [ ] **Step 5: コミット**

```bash
git add internal/app/noticing
git commit -m "feat(noticing): 重なった名前・接頭辞・キーの修復を、作った人に知らせる(C5c1)"
```

---

### Task 5: 2 台の場面テスト

**Files:**
- Create: `cmd/hedgehug/duplicates_repair_scenario_test.go`

**Interfaces:**
- Consumes: Task 1〜4、`cmd/hedgehug/repair_helpers_test.go` の `firstMachine`・`cloneMachine`・`writeAs`・`sync`・`noticed`・`sameFiles`・`newTaskAs`、`repair_notice_scenario_test.go` の `wantRepair`・`wantNoRepair`

- [ ] **Step 1: 場面テストを書く**(A = taro、B = hanako。本物の gitstore とテストごとの state.db)
  - `TestRepairDuplicatesWorkspace`:
    1. A がワークスペース「メイン」を作って同期する。B が clone する。
    2. A がワークスペース「開発」(WEB)とアイテム WEB-1 を作る。B も同時に「開発」(WEB)を作る。
    3. A が同期し、B が同期し、A がもう一度同期する。
    - 期待: どの同期もエラーにならない。2 台とも B の「開発」が「開発 (2)」・WEB2 で、B のアイテムが WEB2-1。2 台のファイルが同じ。B に「2 台で同時に「開発」が作られたため、あとから届いた方を「開発 (2)」に変更しました」と「…「WEB」…「WEB2」に変更しました」が 1 行ずつ。A には出ない。
  - `TestRepairDuplicatesRevivedWorkspace`(C5c1-13):
    1. A が「開発」(WEB)を作って同期する。B が clone する。
    2. A が「開発」を削除し、新しい「開発」(WEB)を作る。B は同時に古い「開発」にステータス「Review」を足す。
    3. A が同期し、B が同期し、A が同期する。
    - 期待: 古い「開発」は戻り(削除の取り消し)、A の新しい「開発」が「開発 (2)」・WEB2。A に 2 行。2 台のファイルが同じ。
  - `TestRepairDuplicatesSettings`: A と B が同時に、同じワークスペースにプロパティ「顧客」(キー customer)・ステータス「Review」と「review」・スプリント「Sprint 1」を足す。同期のあと、B の側のものが「顧客 (2)」・customer2・「review (2)」・「Sprint 1 (2)」。B に 4 行(キーも 1 行)。
- [ ] **Step 2: 流して、足りない組み立てを直す** — Run: `go test ./cmd/hedgehug/ -run 'RepairDuplicates'` Expected: PASS
- [ ] **Step 3: 全体を確かめる** — Run: `go test ./... && golangci-lint run ./... && golangci-lint run --build-tags e2e ./...` Expected: PASS・指摘なし
- [ ] **Step 4: コミット**

```bash
git add cmd/hedgehug
git commit -m "test(sync): 2 台の場面で、重なった名前・接頭辞・キーの修復と通知を確かめる(C5c1)"
```

---

### Task 6: 保存形式・トラック C の計画の記録と全体の検証

**Files:**
- Modify: `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` `:214`
  - 見つけ方: 「あとから同期した方(番号の重なりと同じ決め方)を直す」を「名前を決めた行(その項目の最後の `set`、なければ作成の行)が自分の側にないものの中で最も古いものを残し、ほかを直す(番号の重なりと同じ決め方)。数える範囲と比べ方は書き込みの検査と同じで、削除済みは数えない」にする。
  - 書く行: 「キーは末尾に数字」のあとに「。すでにあれば次の数字(名前は (3)、(4)…)」を足す。
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md` の C5c の行
  - 「C5c1(名前・接頭辞・キー。計画: `2026-11-15-hedgehug-track-c5c1.md`)と C5c2(Issue のリンク・メンバー)に分ける(ユーザーの決定、2026-11-15)」を足す。

- [ ] **Step 1: 保存形式とトラック C の計画を書き直す**(上のとおり)。
- [ ] **Step 2: 全体を確かめる** — Run: `make lint && make test && golangci-lint run --build-tags e2e ./...` Expected: すべて PASS
- [ ] **Step 3: コミット**

```bash
git add docs
git commit -m "docs: 重なった名前・接頭辞・キーの修復(C5c1)を保存形式とトラック C の計画に書く"
```
