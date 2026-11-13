# hedgehug トラック C3c(他のタブで削除したコメント・設定の値への保存)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 別のブラウザのタブ(や別のプログラム)で削除されたコメント・ワークスペースの設定の値(プロパティ・値・テンプレート・自動化のルール)に、このタブから変更を保存したとき、断らずに変更を保存して削除を取り消し、トーストを出す。

**Architecture:** 設定の取り消しは、ワークスペースのログの `delete` の行(項目名 `statuses/<ID>` など)のうち、保存が書く項目名の頭に当たるものを、`change.Undo` で op ごとに戻す(同期の `repair.hasChange` と同じ規則)。usecase は tx の最初に `port.ReviveSetting` を呼んでから、いまの処理を続ける(C3a-screen の決定 S10 と同じ形)。コメントは `delete` の行ではなく `to: null` の行で消えるので、編集の行を足して戻す。取り消したものは context の入れ物に種類つきで積み、`httpapi` のミドルウェアが種類ごとの応答ヘッダーに載せ、画面は `fetcher` で読んでトーストを出す。

**Tech Stack:** Go 1.27.3(huma v2、net/http、net/url)、TypeScript・Svelte(web/packages/api・core、web/app)、Playwright。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`「他の書き手との重なり」(トースト)、「同時に編集したとき」の 3 段落目(設定・コメントも項目ごとの後勝ちで、編集と削除が重なったら削除を取り消す)と表の最後の行、「反映を待つとき」
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/03-workspace-settings.md` の名前の重複の断り(`:215`「同じ名前のプロパティがあります」、`:270`「同じ名前の値があります」、`:415`「同じ名前のテンプレートがあります」、`:491`「同じ名前のルールがあります」)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`「同期の修復」の「削除と変更が重なった」の行
- `docs/superpowers/plans/2026-11-14-hedgehug-track-c3a.md` の決定 C3a-7(`restore` の行)・C3a-8(`change.Undo`)・C3a-9(変更に数える行)、`internal/domain/repair/overlap.go` の `hasChange`
- `docs/superpowers/plans/2026-11-14-hedgehug-track-c3a-screen.md` の決定 S1〜S11
- `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md` の C3c の行(完成の条件)

## Global Constraints

土台・トラック C・C3a-screen の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個。`internal/app/port` は 20 個で満杯なので、Task 1 で `storeversions.go` の中身を `storelist.go` へ移してから `revived.go` を足す。`internal/app/workspaces` は 20 個で満杯なので、ファイルを足さない。`internal/domain/workspace` は 18 個(足すのは `revive.go` の 1 つ)。`internal/domain/workspace/settings_log.go` は 265 行なので触らない。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- トーストの文(決定 R7):
  - 設定の値: `「{名前}」は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました`。`{名前}` は改行と続いた空白を 1 つの空白にしたもの(同期の取り消しのアクティビティ「「{名前}」の削除を取り消し」と同じ読み方。`workspace.NameOfDeleted`)。
  - コメント: `コメントは別の画面で削除されていましたが、変更を保存したため、削除を取り消しました`
- 応答ヘッダー(決定 R8): `Hedgehug-Restored`(表示 ID。C3a-screen のまま変えない)、`Hedgehug-Restored-Name`(設定の値の名前を Go の `url.PathEscape` で符号化したもの。画面は `decodeURIComponent` で戻す)、`Hedgehug-Restored-Comment`(コメントの持ち主の表示 ID)。どれも取り消したものごとに `Header().Add`。
- 手元の検証は 1 回で足りる(CI でも流れる)。`make lint`・`make test`・e2e のビルドの lint(`golangci-lint run --build-tags e2e ./...`)・この計画で足した e2e の spec。

## 取り消す操作と取り消さない操作

規則(決定 R1): 保存が、ワークスペースのログの取り消されていない `delete` の行の項目名 `f` について、`f + "/"` で始まる項目名の行(実行者「あなた」)を書くなら、その削除を取り消す。同期で重なりに数える行(`repair.hasChange`)と同じなので、同期と画面で結果がそろう。それ以外の項目名を書く操作は、削除済みのものを指していれば、いまのまま 404。

| 経路(`/api/stores/{store}/…`) | 書く項目名 | 扱い | Task |
|---|---|---|---|
| `PUT workspaces/{ws}/properties/{p}/name` | `properties/<p>/name` | 取り消す | 2 |
| `PUT …/properties/{p}/kinds` | `properties/<p>/kinds` | 取り消す | 2 |
| `PUT …/properties/{p}/hidden` | `properties/<p>/hidden`(組み込みだけ。組み込みは削除できないので、起きない) | 取り消す | 2 |
| `POST …/properties/{p}/move` | `properties/<p>/rank` と隣の rank | 取り消す(隣は生きているものだけ) | 2 |
| `PUT …/lists/{list}/values/{v}/name` | `statuses/<v>/name` など | 取り消す | 2 |
| `PUT …/lists/{list}/values/{v}/color` | `…/<v>/color` | 取り消す | 2 |
| `POST …/lists/{list}/values/{v}/move` | `…/<v>/rank` と隣の rank | 取り消す | 2 |
| `PUT …/statuses/{v}/category` | `statuses/<v>/category` | 取り消す | 2 |
| `PUT …/templates/{t}` | `templates/<t>/…`(子・Step は `templates/<t>/children/<c>/…`・`templates/<t>/steps/<s>/…`) | 取り消す(テンプレートと、変更のある子・Step。決定 R5) | 3 |
| `PUT …/automation/rules/{r}` | `rules/<r>/…` | 取り消す | 4 |
| `PUT …/automation/rules/{r}/enabled` | `rules/<r>/enabled` | 取り消す | 4 |
| `PUT items/{item}/comments/{c}`、`PUT projects/{p}/comments/{c}` | `comment` の行(f はコメントの ID) | 取り消す(自分のコメントだけ。決定 R6) | 5 |
| 選択肢の追加・名前・色・並び(削除済みのプロパティの選択肢 `choices/<p>/…`) | `choices/<p>/…`(`properties/<p>` の下ではない) | いまのまま 404 | 2(テスト) |
| `PUT …/initial-status`・`initial-priority`・`category-defaults/{c}` | `initial_status`・`initial_priority`・`category_default/<c>` | いまのまま 404(値を指すだけ) | 2(テスト) |
| `PUT …/workflow/{from}/{to}` | `workflow/<from>` | いまのまま 404 | —(既存のまま) |
| `PUT …/template-defaults/{kind}` | `template_default/<kind>` | いまのまま 404 | —(既存のまま) |
| `POST items/{item}/automation/{rule}/apply` | アイテムのログ | ルールが削除済みなら、いまのまま 404(アイテムの取り消しは C3a-screen) | —(既存のまま) |
| プロジェクト別のルールの経路(`set-project-status-used` など 6 つ) | ワークスペースのログの `project_rules/<p>/…` | いまのまま 404。06「プロジェクト別のルールの変更は、プロジェクトへの変更に数えない」。指すステータス・テンプレート・ルールが削除済みでも、指すだけなので取り消さない | —(既存のまま) |
| 設定の値・プロパティ・テンプレート・ルールの削除、コメントの削除 | — | いまのまま 404 | — |

06:193 が挙げるほかのものの扱い(決定 R9):

| もの | 扱い | 根拠 |
|---|---|---|
| 作業時間 | 対象なし | 追加と削除しかなく、編集の経路がない(`internal/app/worklogs`) |
| メンバーの表示名・保持日数 | 対象なし | 削除がない |
| ワークスペースの名前・接頭辞 | 対象なし | ワークスペースの削除は B5b。develop にあとから入る方が、削除済みのワークスペースの名前・接頭辞の変更での取り消しを足す(`port.ReviveSetting` と同じ形で、ワークスペースの削除の行を戻す) |
| スプリント・リリース | 対象なし | F1 がまだ develop にない。F1 はワークスペースのログに書く(F1 の計画)ので、develop にあとから入る方が、スプリント・リリースの編集の経路で `port.ReviveSetting` を呼ぶ(トーストは設定の値と同じ「「{名前}」は…」) |

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| R1 | 取り消す操作は上の表の規則で決める(削除の行の `f + "/"` で始まる項目名の行を、人が書く操作)。 | `repair.hasChange`(同期で重なりに数える行)。C3a-screen S1 と同じく、同期と画面を同じ規則にする |
| R2 | 取り消すのは、保存が書く項目名を覆う削除の行(`f` が同じか、`f + "/"` が頭に当たるもの。テンプレートの子なら、子とテンプレートの両方)の op ごとに、その op の行があるログ(ワークスペースのログ、ワークスペースのプロジェクトとアイテムのログ)すべてへ `change.Undo` を書く。変更と同じ 1 回の保存に入れる。値の削除で置き換えたアイテムのステータスも、先端なら戻る。 | C3a-screen S3、C3a の同期の `undo`(op の行があるログすべて)。01「書き込みの単位」 |
| R3 | 何も変わらない保存(同じ名前・同じ色・端での並べ替え・同じ分類 など、行を書かずに返す分かれ道)は、取り消さず `port.ErrNotFound`(404)。 | C3a-screen S4 |
| R4 | 取り消したあと、取り消したプロパティ・値・テンプレート・ルールの名前が、同じ一覧の削除されていないものと重なるなら(比べ方は作成・名前の変更と同じ)、保存ごと断る。文言は 03 の重複の断り(「同じ名前のプロパティがあります」「同じ名前の値があります」「同じ名前のテンプレートがあります」「同じ名前のルールがあります」)。名前を変える保存は、変更後の名前で判定する。 | 03 の名前の重複を認めない規則。C3a-screen S11(08「名前が重なるときの復元」)と同じ形。同期の「 (2)」は同期の修復だけのもの(03:67) |
| R5 | テンプレートの保存(画面はテンプレート全体を送る)では、取り消すのは、行を書く子・Step だけ。削除済みの子・Step で、送られた内容が削除の前と同じものは、保存の内容から除き(削除済みのまま)、行を書かない。 | R1(変更のない子は同期でも重なりに数えない)。除かないと、削除済みの項目名の下に行を書き、あとの同期で重なりに数えられる |
| R6 | 自分の削除済みのコメントの編集は、`comment` の行(from は null、to は新しい本文)を足して戻す。本文が削除の前と同じなら `port.ErrNotFound`。他人のコメントは、いまのまま 404。持ち主(アイテム・プロジェクト)も削除済み(root: true)なら、持ち主とコメントの両方を取り消し、トーストは持ち主・コメントの順に 2 つ。削除済みのコメントの削除は、いまのまま 404。 | コメントの削除は `to: null` の行で、`change.Current` は先端の編集を残す(`comment.Find`)。R3 |
| R7 | トーストの文は Global Constraints のとおり。 | 06「他の書き手との重なり」の文の `{ID}` を、ID のないものに当てた。名前は同期の取り消しのアクティビティ(C3a)と同じく「」でくくる。Task 7 で 06 に書き足す |
| R8 | 応答ヘッダーは Global Constraints のとおり。名前は `url.PathEscape` で符号化する(空白は `%20`、`,` は `%2C` になり、`fetch` が同じ名前のヘッダーを `", "` でつないでも分けられる。`QueryEscape` は空白を `+` にし、`decodeURIComponent` で戻らないので使わない)。 | HTTP のヘッダーの値に日本語を生で入れると、`fetch` の `Headers.get` が Latin-1 として読み、文字化けする |
| R9 | 06:193 のほかのものは、上の表のとおり。 | 調べた結果 |
| R10 | アクティビティの文は、操作のもの(例: 「ステータス「Review」の分類を変更」)のまま。保存の前の読みは、取り消す削除の行を除いたログから作った設定(`workspace.SettingsReviving`)で行う。 | C3a-screen S3。いまの前の読みは削除済みを持たず、名前を読めない |

## Review Focus

1. **値の削除で置き換えたアイテムの戻り。** ステータス「Review」を削除してアイテムが「Todo」になったあと、別のタブから「Review」の色を変えると、「Review」とアイテムのステータスが戻る。削除のあとにステータスを人が変えたアイテムは戻らない(先端でない)。Task 2 のテストに入れる。
2. **`editValue` を共有する、取り消さない操作。** 削除済みのステータスを初期ステータスにする保存は 404 で、`restore` の行は書かれない。Task 2 のテストに入れる。
3. **テンプレートの、変更のない削除済みの子。** 別のタブで子を削除したあと、このタブでテンプレートの名前だけを変えて保存すると、子は削除済みのままで、子の項目名の行は書かれない。Task 3 のテストに入れる。
4. **名前の符号化。** `,`・空白・「」を含む名前が、ヘッダーを通って元の名前に戻る。Task 6・Task 7 のテストに入れる。
5. **持ち主とコメントの両方の削除。** 削除済みのアイテムの、削除済みの自分のコメントを編集すると、両方が戻り、トーストが 2 つ出る。Task 5 のテストに入れる。

---

### Task 1: 設定の取り消しの口(`domain/workspace`・`app/port`)

**Files:**
- Create: `internal/domain/workspace/revive.go`、`internal/domain/workspace/revive_test.go`
- Create: `internal/app/port/revived.go`(`revive.go` の入れ物の部分を移して広げる)
- Modify: `internal/app/port/revive.go`(`ReviveSetting` を足し、`noteRevived` の呼び出しを `NoteRevived` に)
- Modify: `internal/app/port/storelist.go`(`storeversions.go` の中身を移す)
- Delete: `internal/app/port/storeversions.go`
- Modify: `internal/adapter/httpapi/server.go`(`port.Revived` の型が変わる分だけ。ヘッダーを足すのは Task 6)
- Test: `internal/app/port/revive_test.go`(書き足す)、`internal/app/apptest/stores_contract_isolation.go`・`stores_contract.go`(契約の 1 行)

**Interfaces:**
- Produces(domain/workspace):
  - `func Covering(log []change.Record, f string) []change.Record` — 取り消されていない(`change.Restored` で見る)`delete` の行のうち、`r.F == f` か `strings.HasPrefix(f, r.F+"/")` のもの。ログの順。
  - `func SettingsReviving(log []change.Record, fs ...string) (Settings, bool)` — `fs` のそれぞれの `Covering` の行を除いたログで `SettingsFromLog` を呼ぶ。
  - `func CheckRevivedName(st Settings, f string) error` — `f` が `properties/<id>` なら `CheckPropertyName(st, id, いまの名前)`、値(`statuses/<id>`・`priorities/<id>`・`systems/<id>`・`choices/<p>/<id>`)なら `CheckValueName(一覧, id, いまの名前)`。ほか(テンプレート・ルールは各 usecase の検査が名前を見る)と、見つからないものは nil。
- Produces(app/port):
  - `type RevivalKind int`、定数 `RevivedID`(アイテム・プロジェクトの表示 ID)・`RevivedName`(設定の値の名前)・`RevivedComment`(コメントの持ち主の表示 ID)。
  - `type Revival struct { Kind RevivalKind; Text string }`
  - `func Revived(ctx context.Context) []Revival` — 積んだ順。入れ物がなければ nil。(いまの `[]string` から変える。)
  - `func NoteRevived(ctx context.Context, r Revival)` — 同じものは 1 つ。入れ物がなければ捨てる。(いまの `noteRevived` を公開にして、型を変える。)
  - `func ReviveSetting(ctx context.Context, tx Tx, ws workspace.ID, f string) (bool, error)` — tx の中でワークスペースのログを読み、`workspace.Covering(log, f)` の行ごとに(同じ op は 1 回)、op の行があるログ(ワークスペース・ワークスペースのプロジェクトとアイテム)へ、読み直したログで `change.Undo(log, op)` を書き、`Revival{RevivedName, 名前}`(名前は `workspace.NameOfDeleted(log, 削除の行の F)` を `strings.Fields` で 1 行にしたもの)を積む。取り消したものがあれば true。

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/workspace/revive_test.go`:
- `TestCovering`: ログに `delete statuses/A`(op1)、`delete templates/T`(op2)、`delete templates/T/children/C`(op3)、`restore` で戻した `delete statuses/B`。`Covering(log, "statuses/A")` は op1 の行 1 つ。`Covering(log, "templates/T/children/C")` は op2・op3 の行の 2 つ(ログの順)。`Covering(log, "statuses/B")` と `Covering(log, "statuses/AB")` は空。
- `TestSettingsReviving`: ステータス「Review」を削除したログで、`SettingsReviving(log, "statuses/<Review>")` の `Statuses` に「Review」がある。`SettingsFromLog(log)` にはない。
- `TestCheckRevivedName`: 削除されていない「Review」と、同じ名前の別の値(取り消したもの)がある設定で、取り消したものの `f` に `refusal` の「同じ名前の値があります」。プロパティは「同じ名前のプロパティがあります」。重ならなければ nil。

`internal/app/port/revive_test.go`(`apptest` の偽物の Store):
- `TestReviveSettingRestoresTheWholeOp`: ステータス「Review」を、アイテムのステータスを「Todo」に置き換える行と同じ op で削除する(1 回の Write で `WriteWorkspaces().Append(ws, change.NewDelete("statuses/<Review>"))` と `WriteItems().Append(it, change.NewSet("status", review, todo))`)。`ctx := port.WithRevived(...)` の Write で `ReviveSetting(ctx, tx, ws, "statuses/<Review>/color")` が true。Write のあと、設定に「Review」があり、アイテムのステータスが「Review」。`port.Revived(ctx)` が `[]port.Revival{{Kind: port.RevivedName, Text: "Review"}}`。
- `TestReviveSettingTwiceWritesOnce`: 同じ Write で 2 回呼ぶと、ワークスペースのログの `restore` は 1 行で、2 回目は false。
- `TestReviveSettingOnLiveDoesNothing`: 削除されていない値では false、行も `Revived` も増えない。
- `TestReviveSettingTemplateChild`: テンプレート T(op2)と子 C(それより前の op3)を別々に削除。`ReviveSetting(ctx, tx, ws, "templates/T/children/C")` で、T と C の両方が戻り、`Revived` に T の名前と C の題名が、ログの順に積まれる。
- 既存の `Revived` を使うテストは、`[]port.Revival{{Kind: port.RevivedID, Text: "HH-1"}}` の形に直す。

本物の gitstore でも、利用者の操作の設定の `restore` の行が読めることを、Store の契約に足す(`stores_contract_isolation.go` に書き足し、`stores_contract.go` の表に 1 行足す):
- `contractUserSettingRestoreSeenInTx`: ステータスを削除したあと、実行者 `change.User("taro")` の 1 回の Write の中で `ReviveSetting` を 2 回呼ぶ。2 回目の前に `tx.Workspaces().Settings` にそのステータスがあり、Write のあとワークスペースのログの `restore` の行は 1 つ。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/workspace/... ./internal/app/port/...`
Expected: FAIL(`undefined: workspace.Covering`・`port.ReviveSetting` など)

- [ ] **Step 3: `storeversions.go` を `storelist.go` へ移し、`revived.go`・`ReviveSetting`・domain の 3 つを書く**

ワークスペースのプロジェクトとアイテムは `tx.Projects().List(ws)` と `tx.Items().List(p)` で集める(`workspaces/items.go` の `workspaceItems` と同じ集め方。port からは呼べないので、`revive.go` に同じ小さな関数を置く)。ログに op の行があるかは `slices.ContainsFunc`。`server.go` は `port.Revived` のうち `RevivedID` の `Text` だけを `Hedgehug-Restored` に載せる形に直す(ふるまいは変えない)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/workspace/... ./internal/app/... ./internal/adapter/httpapi/... ./internal/infra/gitstore/... ./internal/archtest/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/workspace internal/app/port internal/app/apptest internal/adapter/httpapi/server.go
git commit -m "feat: 削除済みの設定の値への変更で削除を取り消す口を足す"
```

### Task 2: プロパティと値の変更

**Files:**
- Modify: `internal/app/workspaces/edit.go`(`editAt` を足し、`edit` はそれを呼ぶ)、`property_edit.go`(`editProperty`)、`value_add.go`(`editValue` の隣に `changeValue`)、`value_edit.go`、`value_marks.go`(`SetStatusCategory` だけ)
- Test: `internal/app/workspaces/deleted_test.go`(書き足す。600 行を超えるなら、同じパッケージに `revive_test.go` を作る。テストのファイルは数の上限に入らない)

**Interfaces:**
- Consumes: Task 1 の `ReviveSetting`・`SettingsReviving`・`CheckRevivedName`・`WithRevived`・`Revived`
- Produces:
  - `func (s Service) editAt(ctx context.Context, store port.StoreID, ws workspace.ID, targets []string, summary func(workspace.Settings) (string, error), rows func(tx port.Tx, st workspace.Settings) ([]change.Draft, error)) error` — `edit` と同じで、`targets`(取り消してよい項目名の頭)があれば:
    - 保存の前の読みは `workspace.SettingsReviving(ワークスペースのログ, targets...)`(決定 R10)。
    - tx の最初で `targets` ごとに `port.ReviveSetting` を呼び、そのあと設定を読み直して `rows` を呼ぶ。
    - 取り消したのに行が 0 なら `port.ErrNotFound`(決定 R3。保存ごと何も残さない)。
    - 取り消したなら、行を書いたあとに設定を読み直し、`targets` ごとに `workspace.CheckRevivedName`(決定 R4)。
  - `edit(…)` は `editAt(…, nil, …)`。
  - `func (s Service) changeValue(…)` — `editValue` と同じ引数で、`targets` に `l.Ref(id)` を渡す版。`RenameValue`・`SetValueColor`・`MoveValue`・`SetStatusCategory` が使う。`editValue`(取り消さない)は `SetInitialStatus`・`SetInitialPriority`・`SetCategoryDefault`・`DeleteValue` が使い続ける。
  - `editProperty` は `targets` に `"properties/" + id` を渡す(使うのは名前・種類・表示・並べ替えだけ)。

- [ ] **Step 1: 失敗するテストを書く**

どれも、別のタブの削除に当たるものとして、`Service.DeleteValue`・`DeleteProperty` で削除してから、`port.WithRevived` の context で操作を呼ぶ。

- `TestChangeDeletedValueRevives`(表で回す): ステータスの名前・色・並べ替え・分類、優先度の色、関連システムの名前、単一選択の選択肢の色。成功し、設定に値が戻り、`Revived` が `[]port.Revival{{Kind: port.RevivedName, Text: "<値の名前>"}}`。名前の変更では、変更後の名前になる。
- `TestColorDeletedStatusRestoresItems`: ステータス「Review」のアイテム A・B があり、「Review」を(置き換え先「Todo」で)削除したあと、A のステータスを人が「Done」に変える。「Review」の色を変えると、B は「Review」に戻り、A は「Done」のまま(Review Focus 1)。アクティビティ(コミットの Summary)は「ステータス「Review」の色を変更」の 1 件(いまの `SetValueColor` の文のまま)。
- `TestChangeDeletedPropertyRevives`(表で回す): 追加プロパティの名前・種類・並べ替え。成功し、プロパティが戻る。削除で外したアイテムの値も戻る。
- `TestChangeDeletedValueUnchangedIsNotFound`: 削除済みの値に同じ色・同じ分類・端での並べ替えは `port.ErrNotFound`、`restore` の行がない(決定 R3)。
- `TestChangeDeletedValueNameTaken`: 「Review」を削除したあと、同じ名前の「Review」を新しく足す。古い「Review」の色を変えると `refusal` の「同じ名前の値があります」で、何も保存されない。古い方の名前を「Review 2」に変える保存は成功する(決定 R4)。プロパティでも同じ(「同じ名前のプロパティがあります」)。
- `TestMarksOnDeletedStatusAreNotFound`(表で回す): 削除済みのステータスを初期ステータス・分類の既定にする保存と、削除済みの優先度を初期優先度にする保存は `port.ErrNotFound`、`restore` の行がない(Review Focus 2)。
- `TestChoiceOfDeletedPropertyIsNotFound`: 削除済みの単一選択のプロパティの選択肢の色の変更と、選択肢の追加は `port.ErrNotFound`、プロパティは削除済みのまま。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/workspaces/ -run 'Deleted|Revive|Marks|Choice'`
Expected: FAIL(`ErrNotFound`)

- [ ] **Step 3: `editAt`・`changeValue` を書き、呼ぶ側を替える**

`editAt` は 60 行の上限に収まるよう、tx の中の処理を小さな関数に分ける。`valueOf`(`value_add.go`)は変えない(取り消したあとの設定では見つかる)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/workspaces/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/workspaces
git commit -m "feat: 別の画面で削除されたプロパティと値を変更したら削除を取り消す"
```

### Task 3: テンプレートの保存

**Files:**
- Modify: `internal/domain/workspace/revive.go`(`RevivedTemplate` を足す)、`internal/app/workspaces/template.go`(`UpdateTemplate`)
- Test: `internal/domain/workspace/revive_test.go`、`internal/app/workspaces/template_test.go`(または Task 2 の `revive_test.go`)

**Interfaces:**
- Consumes: Task 1 の口、Task 2 の `editAt`
- Produces: `func RevivedTemplate(log []change.Record, in Template) (Template, []string)` — `in`(画面が送ったテンプレート)のうち、削除済み(`Covering` が空でない)の子・Step を、`SettingsReviving` で読んだ削除の前の内容と比べ、行を書かないもの(`TemplateDrafts` の結果に、その子・Step の項目名の頭の行がないもの)を `in` から除いた版と、取り消す項目名の頭(テンプレート自身 `templates/<t>` と、残した削除済みの子・Step の `templates/<t>/children/<c>`・`templates/<t>/steps/<s>`)を返す(決定 R5)。

- [ ] **Step 1: 失敗するテストを書く**

- `TestUpdateDeletedTemplateRevives`: テンプレートを削除したあと、名前を変えて `UpdateTemplate` すると成功し、テンプレートが戻り、種類ごとの既定から外した行も先端なら戻る。`Revived` が新しい名前ではなく削除の前の名前 1 つ(`NameOfDeleted` は削除の行の時点の名前を読む)。
- `TestUpdateTemplateWithEditedDeletedChild`: 子 C を別の保存で削除したあと、C の題名を変えた内容で `UpdateTemplate` すると、C が戻り、題名が変わる。`Revived` に C の削除の前の題名。
- `TestUpdateTemplateWithUnchangedDeletedChild`: 子 C を削除したあと、C を削除の前のまま含め、テンプレートの名前だけを変えて `UpdateTemplate` すると成功し、C は削除済みのまま、ログに `templates/<t>/children/<C>/` で始まる新しい行がない、`Revived` は空(Review Focus 3)。
- `TestUpdateDeletedTemplateUnchangedIsNotFound`: 削除済みのテンプレートを削除の前と同じ内容で送ると `port.ErrNotFound`、`restore` の行がない。
- `TestUpdateDeletedTemplateNameTaken`: 削除したあと同じ名前のテンプレートを作り、古い方を名前を変えずに保存すると「同じ名前のテンプレートがあります」(いまの `CheckTemplate` が断る。決定 R4)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/workspace/ ./internal/app/workspaces/ -run 'Template'`
Expected: FAIL

- [ ] **Step 3: `RevivedTemplate` を書き、`UpdateTemplate` を `editAt` に替える**

`UpdateTemplate` は保存の前の読みでワークスペースのログを読み、`RevivedTemplate` で `in` と `targets` を決めて `editAt` に渡す。tx の中でも、読み直したログで `RevivedTemplate` をもう一度呼び、前の読みと `targets` が違えば、tx の中の結果を使う(取り消すのは tx の中の `targets`。`editAt` の `targets` は前の読みのもの)。この 2 つを 1 つにするため、`editAt` の `targets` を関数 `func(log []change.Record) []string` で渡せるようにしてよい(その場合、Task 2 の呼ぶ側は固定の項目名を返す関数を渡す)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/workspace/... ./internal/app/workspaces/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/workspace internal/app/workspaces
git commit -m "feat: 別の画面で削除されたテンプレートと子を保存したら削除を取り消す"
```

### Task 4: 自動化のルール

**Files:**
- Modify: `internal/app/automations/service.go`(`edit` に `target string` を足す。空なら取り消さない)、`rules.go`(`Update`・`SetEnabled`)
- Test: `internal/app/automations/deleted_test.go`(書き足す)

**Interfaces:**
- Consumes: Task 1 の口
- Produces: `edit` の `target`。`Update`・`SetEnabled` は `"rules/" + id`(`automation` の項目名の頭。`automation.DeleteDraft` と同じ)、`Create`・`Delete` は空。

`edit` の中は Task 2 の `editAt` と同じ順: 前の読みのルールは `SettingsReviving` と同じく、取り消す削除の行を除いたログから `automation.FromLog` で作る。tx の最初で `port.ReviveSetting`、取り消したのに行が 0 なら `port.ErrNotFound`。名前の重なりは、`Update` はいまの `automation.Check` が断る。`SetEnabled` は、取り消したときだけ `automation.Check(ほかのルール, n)` を足す(決定 R4)。

- [ ] **Step 1: 失敗するテストを書く**

- `TestUpdateDeletedRuleRevives`・`TestSetEnabledDeletedRuleRevives`: ルールを削除したあと(削除でプロジェクト別のルールの「動かさないルール」から外した行も同じ op)、操作が成功し、ルールが戻り、プロジェクト別のルールの指定も戻る。`Revived` がルールの名前 1 つ。
- `TestSetEnabledDeletedRuleSameIsNotFound`: 削除の前と同じ有効・無効なら `port.ErrNotFound`。
- `TestSetEnabledDeletedRuleNameTaken`: 削除のあと同じ名前のルールを作ると、古い方の有効・無効の変更は「同じ名前のルールがあります」。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/automations/ -run 'Deleted'`
Expected: FAIL

- [ ] **Step 3: `edit` に `target` を足し、`Update`・`SetEnabled` から渡す**

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/automations/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/automations
git commit -m "feat: 別の画面で削除された自動化のルールを変更したら削除を取り消す"
```

### Task 5: 削除済みのコメントの編集

**Files:**
- Modify: `internal/domain/comment/comment.go`(`FindDeleted` を足す)、`internal/app/comments/edit.go`
- Test: `internal/domain/comment/comment_test.go`、`internal/app/comments/deleted_test.go`

**Interfaces:**
- Consumes: Task 1 の `NoteRevived`・`Revival{RevivedComment, 持ち主の表示 ID}`
- Produces: `func FindDeleted(log []change.Record, id change.ID) (Comment, bool)` — いまの値(`change.Current`)が削除(`to` が null)のコメントを、削除の行の `from` の本文で返す(`By`・`At` は最初の行のもの)。ないか、削除されていなければ false。

- [ ] **Step 1: 失敗するテストを書く**

`comment_test.go`:
- `TestFindDeleted`: 投稿・削除のログで、本文が削除の前のもの、`By` が投稿者。削除されていないコメントでは false。削除のあとに編集の行を足したログでは、`Find` で見つかり、`FindDeleted` は false。

`comments/deleted_test.go`(アイテムとプロジェクトの両方。表で回す):
- `TestEditDeletedCommentRevives`: 自分のコメントを `Delete` したあと、本文を変えて `Edit` すると成功し、`List` にコメントがあり本文が新しい。`Revived` が `[]port.Revival{{Kind: port.RevivedComment, Text: "HH-1"}}`(プロジェクトなら `HH-P1`)。アクティビティは「コメントを編集」。
- `TestEditDeletedCommentSameBodyIsNotFound`: 削除の前と同じ本文なら `port.ErrNotFound`、行が増えない。
- `TestEditDeletedCommentOfOthersIsNotFound`: 他人の削除済みのコメントは `port.ErrNotFound`。
- `TestEditDeletedCommentOnDeletedOwnerRevivesBoth`: アイテムを root: true で削除し、その前に自分のコメントも削除しておく。`Edit` で両方が戻り、`Revived` が `[{RevivedID, "HH-1"}, {RevivedComment, "HH-1"}]`(Review Focus 5)。
- `TestDeleteDeletedCommentIsNotFound`: 削除済みのコメントの `Delete` は、いまのまま `port.ErrNotFound`。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/comment/... ./internal/app/comments/...`
Expected: FAIL

- [ ] **Step 3: `FindDeleted` を書き、`Edit` を直す**

`Edit` の tx: `reviveOwner` のあと、`currentOf` が `port.ErrNotFound` なら、持ち主のログで `comment.FindDeleted` を読む。見つかり、投稿者が「あなた」(大文字と小文字を区別しない)で、本文が違えば、`change.NewComment(id, nil, comment.Value(body))` を足し、`port.NoteRevived(ctx, port.Revival{Kind: port.RevivedComment, Text: 持ち主の表示 ID})`(表示 ID は `editableDisplay`)。それ以外は `port.ErrNotFound`。`edit.go` の Doc コメントの「ないか削除されたコメント … は port.ErrNotFound」を、この規則に合わせて書き直す。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/comment/... ./internal/app/comments/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/comment internal/app/comments
git commit -m "feat: 別の画面で削除された自分のコメントを編集したら削除を取り消す"
```

### Task 6: 応答ヘッダー `Hedgehug-Restored-Name`・`Hedgehug-Restored-Comment`

**Files:**
- Modify: `internal/adapter/httpapi/server.go`(`restoredWriter.WriteHeader`)
- Test: `internal/adapter/httpapi/server_test.go`、`internal/adapter/httpapi/workspaces/` の既存のテストのファイル(1 経路の通し)

**Interfaces:**
- Consumes: Task 1 の `Revived`・`Revival`

- [ ] **Step 1: 失敗するテストを書く**

`server_test.go`:
- `TestRestoredHeaderKinds`: Handler が `{RevivedID, "HH-1"}`・`{RevivedName, "レビュー, 待ち 「A」"}`・`{RevivedComment, "HH-2"}` を積んで 200 を返すと、`Hedgehug-Restored` が `["HH-1"]`、`Hedgehug-Restored-Name` が `[url.PathEscape("レビュー, 待ち 「A」")]` で、その値に `,` と空白がなく、`url.PathUnescape` で元に戻る。`Hedgehug-Restored-Comment` が `["HH-2"]`(Review Focus 4)。
- 既存の `TestRestoredHeaderNotOnFailure` に、名前とコメントを積んでも 422 では付かないことを足す。

`workspaces` の通し: `TestRenameDeletedValueSetsRestoredNameHeader`: 削除済みのステータスへ `PUT …/lists/statuses/values/{v}/name` が 200(または経路のいまの成功の状態)で、`Hedgehug-Restored-Name` を `url.PathUnescape` すると削除の前の名前。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/...`
Expected: FAIL

- [ ] **Step 3: 種類ごとにヘッダーを足す**

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/adapter/httpapi/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/adapter/httpapi
git commit -m "feat: 設定の値とコメントの削除を取り消した保存の応答にヘッダーを付ける"
```

### Task 7: 画面のトーストと仕様の文

**Files:**
- Modify: `web/packages/api/src/fetcher.ts`、`web/packages/api/src/index.ts`(`Restored` の型を公開)
- Modify: `web/packages/core/src/features/sync/restored.ts`、`web/packages/core/src/index.ts`
- Modify: `web/app/src/wireBase.ts`
- Modify: `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`「他の書き手との重なり」
- Test: `web/packages/api/src/fetcher.test.ts`、`web/packages/core/src/features/sync/restored.test.ts`

**Interfaces:**
- Produces:
  - api: `export type Restored = { kind: "id" | "name" | "comment"; text: string }`。`onRestored(listener: ((restored: Restored[]) => void) | null): void`(引数の型を変える)。`fetcher` は成功した応答で、`Hedgehug-Restored`(`kind: "id"`)・`Hedgehug-Restored-Name`(`kind: "name"`、各値を `decodeURIComponent`)・`Hedgehug-Restored-Comment`(`kind: "comment"`)を、この順に、各ヘッダーを `","` で分けて前後の空白を除いた並びにし、空でなければ 1 回呼ぶ。
  - core: `restoredNameToast(name: string): string` — `「${name}」は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました`。`restoredCommentToast(): string` — `コメントは別の画面で削除されていましたが、変更を保存したため、削除を取り消しました`。`restoredToast(id)` は変えない。

- [ ] **Step 1: 失敗するテストを書く**

- `fetcher.test.ts`: ヘッダー `Hedgehug-Restored: HH-1`・`Hedgehug-Restored-Name: <encodeURIComponent("レビュー, 待ち 「A」")>`・`Hedgehug-Restored-Comment: HH-2` の 200 で、受け手が `[{kind:"id",text:"HH-1"},{kind:"name",text:"レビュー, 待ち 「A」"},{kind:"comment",text:"HH-2"}]` で 1 回呼ばれる。名前のヘッダーが 2 つ(`fetch` が `", "` でつなぐ)でも 2 つに分かれる。422 では呼ばれない。既存のテストは新しい型に直す。
- `restored.test.ts`: `restoredNameToast("Review") === "「Review」は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました"`、`restoredCommentToast() === "コメントは別の画面で削除されていましたが、変更を保存したため、削除を取り消しました"`。

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web test`
Expected: FAIL

- [ ] **Step 3: 書く**

`wireBase.ts` は `kind` で文を選び、受け取った順に `toasts.show`(通常のトースト)。06「他の書き手との重なり」の、トースト「{ID} は…」の文のあとに、次を足す: 「設定の値(プロパティ・値・テンプレート・自動化のルール)は「「{名前}」は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました」、自分のコメントは「コメントは別の画面で削除されていましたが、変更を保存したため、削除を取り消しました」を出す。」

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web deps && pnpm -C web size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/api web/packages/core web/app/src/wireBase.ts docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md
git commit -m "feat: 別の画面で削除されていた設定の値とコメントへの保存でトーストを出す"
```

### Task 8: e2e(2 つのタブ)

**Files:**
- Modify: `web/e2e/deleted-elsewhere.spec.ts`(書き足す)

**Interfaces:**
- Consumes: `web/e2e/helpers.ts` の `createWorkspace`・`newTaskUnder`・`prefixOf`・`tokenUrl`・`unique`、Task 7 のトースト

このタブは、入力欄にカーソルがあるあいだ・コメントを編集しているあいだ(06「反映を待つとき」)なので、別のタブの削除が反映されず、画面が開いたまま保存できる。

- [ ] **Step 1: テストを書く**

- `"renaming a status deleted in another tab restores it"`: このタブでワークスペースの設定の「ステータス」の一覧を開き、ステータス(例「Review」)の名前の入力欄に「Reviewing」と入力する(Enter はまだ押さない)。別のタブ(`page.context().newPage()`)で同じ一覧を開き、同じステータスを削除する(削除の操作とダイアログは `web/e2e/workspace-settings.spec.ts` のものを使う)。このタブで Enter を押すと、トースト「「Review」は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました」が完全一致で出る。別のタブで一覧を開き直すと「Reviewing」がある。
- `"editing a comment deleted in another tab restores it"`: このタブでアイテムを作り、コメントを投稿して、コメントの「編集」で本文を変える(保存はまだ)。別のタブで同じアイテムを開き、同じコメントを削除する(操作は `web/e2e/comments.spec.ts` のものを使う)。このタブで保存すると、トースト「コメントは別の画面で削除されていましたが、変更を保存したため、削除を取り消しました」が完全一致で出て、新しい本文が表示される。別のタブで開き直すと、コメントがある。

- [ ] **Step 2: 流して通ることを確かめる**

Run: `make e2e-bin && pnpm -C web e2e e2e/deleted-elsewhere.spec.ts`
Expected: 4 passed(C3a-screen の 2 つを含む)

- [ ] **Step 3: コミット**

```bash
git add web/e2e/deleted-elsewhere.spec.ts
git commit -m "test: 別のタブで削除した設定の値とコメントへの保存を e2e で確かめる"
```

### Task 9: 全体の検証と計画の表

**Files:**
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`(C3c の行に、この計画のファイル名を書く)

- [ ] **Step 1:** `make lint && make test && golangci-lint run --build-tags e2e ./...` がすべて通る。
- [ ] **Step 2:** 上の「取り消す操作と取り消さない操作」の表の、Task の欄が数字の各行に、その Task のテストが 1 つ以上あることを確かめる(足りなければ、その行の Task のテストのファイルに足す)。
- [ ] **Step 3: コミット**

```bash
git add docs/superpowers/plans/2026-11-12-hedgehug-track-c.md
git commit -m "docs: トラック C の計画の C3c の行に計画のファイルを書く"
```
