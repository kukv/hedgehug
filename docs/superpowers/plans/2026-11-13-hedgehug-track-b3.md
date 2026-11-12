# hedgehug トラック B3(ワークフローとテンプレート)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 03 の「ワークフロー」(遷移の制約の表と判定)と「テンプレート」(持てるもの、一覧と編集、アイテムから作る、適用の規則、種類ごとの既定)を、サーバーと設定の画面に作る。判定と適用の規則は関数として持ち、アイテムの操作と作成ダイアログへのはめ込みは A3 後半(A3b)が行う。

**Architecture:** 遷移の制約とテンプレートは、B1・B2 と同じくワークスペースの `log.jsonl` の項目として持ち、`workspace.SettingsFromLog` で `Settings` に読む。規則(判定・検査・整え方・行の作り方)は `domain/workspace`、操作は `app/workspaces` に 1 ファイル 1 操作、HTTP は `adapter/httpapi/workspaces`。画面は `core/features/workspaces` に行の作りと文言、`app/screens/workspace-settings` に見た目を置く。

**Tech Stack:** 土台と同じ(Go 1.27.3、huma v2、Svelte 5、orval、Vitest、Playwright)。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/03-workspace-settings.md`(正本): 冒頭の表のうち「遷移の制約」「テンプレートの作成・編集・削除」「種類ごとの既定」の行、名前の重複の規則(テンプレート)、「値の編集」の「削除」(テンプレートとワークフローの追従)、「プロパティ」の「削除」(テンプレートの値)、「ワークフロー」全節、「テンプレート」全節、「変更が及ぶ範囲」の遷移の制約の行
- 02「アイテムの作成」(テンプレートの欄・一緒に作るもの・ダイアログの中での連動。適用の規則の関数が満たす振る舞い)、「詳細パネル」のメニューと閲覧のみ(「テンプレートとして保存…」)
- 06「同時に編集したとき」(設定も項目ごとの後勝ち)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`(「変更記録」の `f`、`add`・`remove` の対象に「ワークフローの遷移」、「同期の修復」の (8)(9))
- `docs/superpowers/plans/2026-11-12-hedgehug-track-b.md`(B の決定 1〜13 と小計画の一覧)、`docs/superpowers/plans/2026-11-13-hedgehug-track-b2.md`(B2-1〜B2-21。とくに B2-10・B2-12・B2-21)

**持ち場:** `internal/domain/workspace`、`internal/app/workspaces`、`internal/adapter/httpapi/workspaces`、`internal/infra/gitstore/yaml_settings*.go`、`web/packages/core/src/features/workspaces`、`web/packages/core/src/model/workspace.ts`・`port/workspaces.ts`、`web/packages/api/src/workspaces.ts`、`web/app/src/screens/workspace-settings`。例外として、詳細パネルのメニュー(`web/app/src/screens/item/DetailHeader.svelte` にメニューのボタンを 1 つ)を作る(決定 B3-17)。

**待つもの:** B2(#166)と Menu の修正(#167)。どちらも develop に入っている。

## Global Constraints

土台・トラック B・B2 の Global Constraints をすべて引き継ぐ。主なもの:

- 依存の向き: `app/workspaces` は `domain` と `app/port` だけを import する。`domain/item` は `domain/workspace` を import するので、`domain/workspace` から `domain/item` を import しない(アイテムからテンプレートを作る処理は `app/workspaces` に置く)。
- 画面の文言・アクティビティの文・断りの文は、仕様 03 の文字どおりに書く。仕様にない文言を足さない(足すときは決定の表に理由を書く)。
- 操作が条件に合わないときは `refusal.New(文言)`、画面から起きない要求(ない ID、テンプレートに持たないプロパティなど)は `port.ErrNotFound`(B2-12)。
- `api/openapi.yaml` と `web/packages/api/src/generated.ts` は手で直さない。`make generate` と `cd web && pnpm gen` で作り直し、同じコミットに入れる。
- 大きさの上限: Go ファイル 300 行・関数 60 行・認知的複雑度 15・パッケージ 20 ファイル(`internal/app/workspaces` はテストを除き 20 ファイル以内に収める)、Svelte 200 行、TS 300 行。
- 依存を足さない(足すなら版を明記し、公開から 7 日以上のものだけ)。
- 計画やテストに `ghp_` などトークンらしい値を書かない。
- force push は拒否される。載せ直すときは別名のブランチで出す。

---

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| B3-1 | 遷移の制約は、From のステータスごとの項目 `workflow/<From の値 ID>` に、許可する To の値 ID を `add`・`remove` で持つ。`Settings.Workflow`(`map[change.ID][]change.ID`)で読む。行がない・空の From は制限なし。B5 のプロジェクトの制約も同じ形(`project_rules/<P>/workflow/<From>` に To を `add`・`remove`)にする | 保存形式「変更記録」(`add`・`remove` の対象に「ワークフローの遷移」)。06「同時に編集したとき」(値を複数持つ項目は 1 つずつの追加と削除を両方生かす)。From と To の組ごとに勝ち負けが決まる |
| B3-2 | 判定は `workspace.CheckTransition(s, from, to) error`(断りは 03 の文言)と `Settings.Allowed(from) ([]change.ID, bool)`。from と to が同じ、from が空、制限のない行は通す。許可先は、いまあるステータスだけを、ステータスの並びで数える。プロジェクトの制約(「両方が許す To」と「変更できる先がありません」)は B5 が引数を足して広げる | 03「遷移の制約」。B の未決 3(判定は domain の関数で、A の操作が呼ぶ) |
| B3-3 | 画面のメニューで無効の理由を出すため、同じ判定を core の `transitionRefusal(settings, from, to)` にも置く(文言は Go と同じ)。A3b がステータスのメニューと Step のチェックから呼ぶ | B2 の R10(確認の前の断りを core にも置いた)と同じ分け方。03「ステータスのメニュー: 選べない項目を無効にし、理由を添える」 |
| B3-4 | テンプレートは、テンプレートごとの項目 `templates/<T>/{name,kind,title,description}`、値 `templates/<T>/values/<プロパティ ID>`、子 `templates/<T>/children/<C>/{kind,title,description,rank}` と `templates/<T>/children/<C>/values/<プロパティ ID>`、Step `templates/<T>/steps/<S>/{parent,title,assignee,rank}`(`parent` は空ならもとになるアイテム、ほかは子の ID)で持つ。どれも `set`。削除は `delete` の行(`templates/<T>`、`templates/<T>/children/<C>`、`templates/<T>/steps/<S>`)。子を消すときは、その子の Step の行は書かない(読むときに持ち主のない Step を捨てる) | B の決定 2(1 つの値ごとに 1 項目)。06「同時に編集したとき」(テンプレートも項目ごとの後勝ち。編集と削除が重なったら編集を残す)。子の削除が同期で取り消されたとき、Step も一緒に戻るため |
| B3-5 | 値は、プロパティの ID ごとの JSON の値で持つ。形はアイテムの `item.Current` と同じ(優先度・単一選択は値の ID、担当者・ユーザーはログイン名、関連システム・複数選択は値の ID の配列、数値は数、日付は `YYYY-MM-DD`、チェックは真偽、テキスト・URL は文字列)。ない項目は「指定しない」。空の配列と `null` は持たない | 03「持てるもの」。組み込みのプロパティの ID はキーと同じ(B の決定 3)なので、組み込みも追加も同じ形で扱える |
| B3-6 | テンプレートに持つプロパティは、組み込みの優先度・担当者・関連システムと、追加プロパティのすべて(ステータス・開始日・終了日・ポイント・見積もり時間は持たない)。保存のときに、その種類で使わないプロパティの値を捨てる。非表示のプロパティの値は、画面に出さないが捨てない | 03「持てるもの」「一覧と編集」(新しい種類で使わないプロパティの値は保存しない。編集して保存しても、ほかの項目の値は消えない) |
| B3-7 | 担当者の「未割り当て」は「値なし」として持つ(作成ダイアログで入力済みの担当者を置き換えない)。優先度の「指定しない」も値なし | 03「持てるもの」の値の欄(担当者は「未割り当て」とメンバー、優先度は「指定しない」と値)と「適用するときの規則」(テンプレートに値のある項目だけを置き換える)。担当者には「指定しない」の選択肢がないので、「未割り当て」がそれに当たる |
| B3-8 | Step の担当者は文字列で、`@parent`(親と同じ。初期値)、`@none`(未割り当て)、またはメンバーのログイン名。ログイン名に `@` は入らない | 03「持てるもの」の Step の担当者の 3 つ |
| B3-9 | 編集の保存は、ダイアログの中身をまるごと送り、サーバーが、書く Tx の中で読み直したテンプレートと比べて、変わった項目の行だけを書く。値は、送られた項目を置き換え(`null` は外す)、送られなかった項目はいまの値を残す(非表示のプロパティの値を消さない)。種類で使わなくなった値だけを外す | 06(項目ごとの後勝ち)。03「編集して保存しても、ほかの項目の値は消えない」 |
| B3-10 | 子と Step の並びは、保存のたびに、送られた並びで `rank.First` から `rank.After` を順に振り直し、値の変わった行だけ `rank` を書く。テンプレートの一覧の並びは内部 ID の順(作った順)。テンプレートの並べ替えはない | 03 に一覧の並べ替えがない。子と Step は「上へ」「下へ」で動かす(03「一覧と編集」) |
| B3-11 | 名前は前後の空白を除いて保存し、完全一致で重複を断る(大文字と小文字・全角と半角は区別する)。タイトルの書き出しは、入力どおりに保存する(「調査: 」の末尾の空白は、続きを打つための書き出しの一部)。子と Step のタイトルは前後の空白を除き、空なら捨てる | 03 冒頭「名前の重複」(空白を除くのは名前だけ)。03「タイトルが空の子と Step の行は、保存するときに捨てる」。子と Step のタイトルは、作るときにアイテムのタイトルとして整える(02「タイトル」)ので、除いても結果は同じ |
| B3-12 | 種類ごとの既定は `template_default/<種類>`(`set`。「なし」は `null`)で持ち、`Settings.TemplateDefaults`(種類 → テンプレート ID)で読む。テンプレートの削除・種類の変更では、同じ保存で、そのテンプレートを指す既定を `null` にする | 03「種類ごとの既定」。保存形式の `project_rules/<P>/template_default/<種類>` と同じ形 |
| B3-13 | テンプレートの作成・編集・削除のアクティビティの文は「テンプレート「X」を作成」「テンプレート「X」を編集」「テンプレート「X」を削除」で、編集の X は保存したあとの名前。トーストは作成「テンプレートを作成しました」、削除「テンプレートを削除しました」だけで、編集は出さない | 03 冒頭の表。表のトーストの欄は作成と削除の文しか持たず、編集に「作成しました」を出すと誤りになる |
| B3-14 | 種類を変えて保存するとき、捨てる構成があれば確認ダイアログ「テンプレートを保存」(ボタン「保存」)に「子 N 件・Step M 件は、この種類では保存されません」を出す。0 件の方は書かない(「Step 3 件は、この種類では保存されません」) | 03「一覧と編集」。確認ダイアログの見出しとボタンは仕様にないので、ダイアログの動詞に合わせた |
| B3-15 | 値の削除・プロパティの削除での追従(B2-21): ステータスの削除で、その行と、ほかの行の許可先から `remove` で外す(許可先がなくなった行は「どこへでも」に戻る)。優先度・単一選択の削除で、テンプレートの値を代わりの値(単一選択の「なし」は値を外す)に、関連システム・複数選択の削除で、その値を外す(空になったら値を外す)。追加プロパティの削除で、テンプレートの値を外す。どれも同じ保存に入れる。遷移の制約は検査しない(B2-10) | 03「値の編集」の「削除」、「プロパティ」の「削除」 |
| B3-16 | アイテムからテンプレートを作るのは、`POST /api/stores/{store}/items/{item}/save-as-template`。種類・値(B3-6 のうち、その種類で使うもの)、子と Step(並び順のまま)を写す。説明はまだアイテムにない(A4)ので写さず、A4 が説明を足すときに写す処理を足す。削除済みの子もまだない(A4) | 03「アイテムからテンプレートを作る」 |
| B3-17 | 詳細パネルのメニュー(ヘッダーの「…」)は B3 が作り、「テンプレートとして保存…」だけを置く。ほかの項目(上へ移動・移動・種類の変更・複製・ID をコピー・削除ほか)は、作るトラック(A4)が足す。閲覧のみの Store では無効にし、「閲覧のみの Store です」を添える | B2-17(サイドバーの Store のメニューは、最初に要る B2 が作り、自分の項目だけを置いた)と同じ分け方。02「詳細パネル」のメニューと閲覧のみ。A2 の決定 18(メニューは A4)を変えるので、ユーザーの確認を待つ。確認で「A4 に任せる」となったら、Task 15 は core とダイアログまでにし、メニューの項目は A4 が足す |
| B3-18 | 作成ダイアログでの適用の規則(タイトル・説明・値の入れ方、テンプレート由来の説明の見分け、使えるテンプレートと既定、「一緒に作るもの」の行と見出し)は、core の関数 `applyTemplate`・`usableTemplates`・`defaultTemplate`・`templateStructure` として持つ。作成ダイアログへのはめ込み、テンプレートでまとめて作る保存(番号・スプリント)は A3b | トラック B の小計画の一覧(「適用の規則を core の関数として持つ」)。A3 前半の計画の決定 1・5 |
| B3-19 | 設定の画面は、テンプレートのタブを開くとき、Store のメンバーを読む(担当者の選択肢)。`createWorkspaceSettings` の deps に `listMembers` を足す | 03「持てるもの」(担当者は Store のメンバー)。プロジェクトの作成ダイアログと同じ口(`CatalogPort.listMembers`)を使う |
| B3-20 | テンプレートの値の入力は、型ごとに `@hedgehug/ui` の `Select`・`Input`・`Checkbox` で作る(A3 前半の `PropertyField` はまだ develop にないので使わない)。チェック型は「指定しない」「オン」「オフ」のセレクトにする(値なしと「オフ」を分けるため) | 03「持てるもの」(値のない項目は置き換えない)。チェックボックス 1 つでは「指定しない」を表せない |
| B3-21 | 仕様にない文言を 3 つ足す: 「テンプレートとして保存…」のダイアログの見出し「テンプレートとして保存」、テンプレートの値のセレクトの空の選択肢「指定しない」(優先度以外の単一選択・ユーザー型・チェックにも使う)、編集ダイアログの見出し「テンプレートの編集」(新規は「テンプレートを作成」) | 見出しはメニューの項目の動詞からとった。「指定しない」は 03 が優先度に使う語で、値なしの意味が同じ |

---

## 漏れの表(B3 の範囲 → タスク、範囲の外 → 持ち主)

| 仕様の節・項目 | 持ち場 |
|---|---|
| 03 冒頭の表: 遷移の制約・テンプレートの作成・編集・削除・種類ごとの既定(文とトースト) | Task 4・5(文)、Task 11〜13(トースト) |
| 03 名前の重複(テンプレート) | Task 3 |
| 03 同期で同じ名前のテンプレートが 2 つ、削除されたテンプレートを指す既定の修復 | トラック C(B の決定 10。保存形式「同期の修復」(8)(9)) |
| 03 ワークフロー: 表、「どこへでも」「制限あり」、「—」、チェックのたびに保存、説明 | Task 1(判定)・Task 4(操作)・Task 11(画面) |
| 同: 制約に反する変更の断りと文言 | Task 1(Go)・Task 11(core の `transitionRefusal`) |
| 同: 断りの見せ方(メニューの無効、ドラッグ・セル・Step のチェック・Space キー、一括変更、自動化) | A3b(メニュー・Step のチェック)、E(ドラッグ・セル・一括変更・Space キー)、B4(自動化) |
| 同: 制約を適用しない変更(作成・移動・種類の変更・同期・修復・値の削除の置き換え・受信箱) | 各操作のトラック(呼ばないことを守る)。値の削除は Task 7 で検査しない |
| 同: プロジェクトの制約と「両方が許す To」「変更できる先がありません」、使わないステータス | B5 |
| 03 テンプレート / 持てるもの | Task 2(型)・Task 3(整え方) |
| 同 / 一覧と編集(一覧の行、件数、ないときの文、作成、編集ダイアログ、子の編集、捨てる確認、保存の条件、削除の確認) | Task 5(サーバー)・Task 12・13(画面) |
| 同: 削除の確認に添える「{N} 件のプロジェクト別のルールからも削除されます。」「{ID} は、すべてのテンプレートを使えるようになります。」 | B5 |
| 同 / アイテムからテンプレートを作る | Task 6(サーバー)・Task 15(画面)。説明を写すのは A4(B3-16) |
| 同 / 適用するときの規則(作成ダイアログに入れる) | Task 14(core の関数)。はめ込みは A3b |
| 同: 作成ボタンで子と Step をまとめて作る、番号を深さ優先で振る、スプリントはもとだけ | A3b(02「作成したあと」の「一緒に作るもの」) |
| 同: 「続けて作成」で説明を空にしたあとは入れ直さない | A3b(作成ダイアログの状態) |
| 同 / 種類ごとの既定 | Task 5(サーバー)・Task 12(画面) |
| 同: プロジェクトの使うテンプレートと種類ごとの既定 | B5 |
| 03 値の編集 / 削除: ワークフロー・テンプレートの追従 | Task 7 |
| 同: 自動化・プロジェクト別のルールの追従 | B4・B5 |
| 03 プロパティ / 削除: テンプレートの値が消える | Task 7 |
| 03 変更が及ぶ範囲: 遷移の制約の変更(既存のアイテムは変えない) | Task 4(アイテムに書かない) |
| 保存形式: `workspace.yaml` に遷移の制約・テンプレート・既定 | Task 8 |
| 02 詳細パネルのメニュー「テンプレートとして保存…」と閲覧のみの無効 | Task 15(B3-17)。ほかの項目は A4 |
| 02 作成ダイアログのテンプレートの欄、「一緒に作るもの」の表示、連動 | A3b(Task 14 の関数を使う) |
| 06 通知の「開く先」(ワークフロー・テンプレートのサブタブ) | トラック C(受信箱) |

## タスクの順と PR

| Task | 中身 | PR |
|---|---|---|
| 1〜9 | サーバー(遷移の制約、テンプレートの型・ログ・検査、操作、アイテムから作る、削除の追従、yaml、API) | B3 の PR 1 本(共有部分を変えないので、共有の PR はない) |
| 10〜16 | 画面と e2e | 同じ B3 の PR |

Task は番号の順に 1 つずつ行う。依存: 2 は 1(`builder` の形)、3 は 2、4 は 1、5 は 3・4(`summary_test.go` を 4・5 の両方が直す)、6 は 5、7 は 1・3・4・5、8 は 1・3、9 は 4〜7、10 は 9、11・12 は 10、13・14・15 は 12(`TemplatesPanel`・`templateSummary`・`templateCreated`)、16 は最後。

## ファイルの地図

```
サーバー(Task 1〜9)
internal/domain/workspace/workflow.go          WorkflowField・Allowed・CheckTransition・WorkflowDeleteDrafts
internal/domain/workspace/workspace.go         opts.Multi に workflow/ を足す(1 行)
internal/domain/workspace/settings.go          Settings に Workflow・Templates・TemplateDefaults
internal/domain/workspace/settings_log.go      builder から workflow・templates・template_default を読む呼び出し
internal/domain/workspace/template.go          Template・TemplateChild・TemplateStep・StepParent・StepNone・HoldsTemplateValue
internal/domain/workspace/template_log.go      テンプレートの項目を読む(builder のメソッド)
internal/domain/workspace/template_drafts.go   TemplateDrafts(差分の行)・Ranked
internal/domain/workspace/template_check.go    NormalizeTemplate・CheckTemplate・DroppedByKind・ValidTemplate
internal/domain/workspace/template_follow.go   TemplateValueDrafts・TemplatePropertyDrafts
internal/domain/workspace/*_test.go

internal/app/workspaces/workflow.go            SetTransition
internal/app/workspaces/template.go            CreateTemplate・UpdateTemplate・DeleteTemplate・SetTemplateDefault
internal/app/workspaces/template_item.go       CreateTemplateFromItem
internal/app/workspaces/value_delete.go        遷移の制約とテンプレートの追従を足す
internal/app/workspaces/property_delete.go     テンプレートの値の追従を足す
internal/app/workspaces/*_test.go

internal/infra/gitstore/yaml_settings.go       workspace.yaml に workflow・templates・template_defaults
internal/infra/gitstore/yaml_templates.go      テンプレートの YAML の組み立て
internal/infra/gitstore/yaml_settings_test.go

internal/adapter/httpapi/workspaces/dto.go     WorkspaceSettings に workflow・templates・templateDefaults
internal/adapter/httpapi/workspaces/templates.go  テンプレートと遷移の制約の API
internal/adapter/httpapi/workspaces/workspaces.go Register から registerTemplates を呼ぶ
internal/adapter/httpapi/workspaces/templates_test.go
api/openapi.yaml / web/packages/api/src/generated.ts(生成物)

画面(Task 10〜16)
web/packages/core/src/model/workspace.ts                 WorkspaceTemplate ほかの型
web/packages/core/src/port/workspaces.ts                 口に 6 つ足す
web/packages/api/src/workspaces.ts                       口の実装
web/packages/core/src/features/workspaces/settings.ts    members を読む
web/packages/core/src/features/workspaces/workflow.ts    表の行・transitionRefusal
web/packages/core/src/features/workspaces/templates.ts   一覧の行・種類ごとの既定・削除の本文
web/packages/core/src/features/workspaces/templateForm.ts 編集ダイアログの中身・値の欄・捨てる確認・送る形
web/packages/core/src/features/workspaces/templateApply.ts 適用の規則(A3b が使う)
web/packages/core/src/features/workspaces/saveAsTemplate.ts 「テンプレートとして保存…」の中身
web/app/src/screens/workspace-settings/WorkflowPanel.svelte
web/app/src/screens/workspace-settings/TemplatesPanel.svelte・KindDefaults.svelte
web/app/src/screens/workspace-settings/TemplateDialog.svelte・TemplateValueFields.svelte・TemplateSteps.svelte・TemplateChildren.svelte・TemplateChildDialog.svelte
web/app/src/screens/workspace-settings/SettingsTab.svelte  ワークフロー・テンプレートのタブ
web/app/src/screens/item/DetailHeader.svelte・ItemMenu.svelte・SaveAsTemplateDialog.svelte
web/app/src/wire.ts
web/e2e/templates.spec.ts
```

---
## サーバーのタスクに共通すること

- テストは `internal/domain/workspace` では `workspace_test` パッケージ、`internal/app/workspaces` では `workspaces_test` パッケージに書き、既存の道具(`rows`・`seqIDs`・`setup`・`settingsOf`・`putItem`・`putItemIn`・`itemOf`・`rejected`・`recorder`)を使う。
- 既定の設定(`workspace.DefaultSettings`)のステータスは Todo・InProgress・Done・Pending・Cancel の順。テストでは `s.Statuses[0]` のように並びで指す。
- 新しいテストファイルを作る(既存のテストファイルを 300 行より大きくしない)。
- 各タスクの最後に `go test ./internal/... && golangci-lint run ./...` を流す。
- `internal/app/workspaces` のテストでないファイルは、この計画で 17 → 20 になる(上限ちょうど)。ファイルを足さない。

### Task 1: 遷移の制約の型・読み取り・判定

**Files:**
- Create: `internal/domain/workspace/workflow.go`、Test: `internal/domain/workspace/workflow_test.go`
- Modify: `internal/domain/workspace/settings.go`(`Settings.Workflow`)、`internal/domain/workspace/workspace.go`(`opts.Multi`)、`internal/domain/workspace/settings_log.go`(`SettingsFromLog` で読む)

**Interfaces:**
- Consumes: B2 の `Settings`・`SettingsFromLog`・`statusName`(`value_check.go`)・`refusal.New`
- Produces:

```go
// settings.go の Settings に足す
Workflow map[change.ID][]change.ID // From のステータス ID → 許可する To(読んだまま)

// workflow.go
func WorkflowField(from change.ID) string                         // "workflow/<from>"
func (s Settings) Allowed(from change.ID) ([]change.ID, bool)      // 許可先(ステータスの並び)、制限があるか
func CheckTransition(s Settings, from, to change.ID) error         // 断りは refusal
func WorkflowDeleteDrafts(s Settings, id change.ID) []change.Draft // ステータス id を表から外す行(Task 7 が使う)
```

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/workspace/workflow_test.go`:

```go
package workspace_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// workflowSettings は、既定の設定の作成の行に drafts を足して読み直した設定。
func workflowSettings(t *testing.T, drafts func(s workspace.Settings) []change.Draft) workspace.Settings {
	t.Helper()
	w := workspace.Workspace{ID: "01WS0000000000000000000000", Name: "Web", Prefix: "HH"}
	s := workspace.DefaultSettings(seqIDs())
	create, err := workspace.CreateWithSettings(w, s)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := workspace.SettingsFromLog(rows(append([]change.Draft{create}, drafts(s)...)...))
	if !ok {
		t.Fatal("no create")
	}
	return got
}

// refusedWith は、断りの文言。断りでなければ空。
func refusedWith(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

// sameDrafts は、a と b が同じ行の集合か(並びは問わない)。
func sameDrafts(a, b []change.Draft) bool {
	if len(a) != len(b) {
		return false
	}
	used := make([]bool, len(b))
	for _, x := range a {
		found := false
		for i, y := range b {
			if !used[i] && reflect.DeepEqual(x, y) {
				used[i], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Todo から InProgress・Done を許し、Done を外して Cancel を足す。
func TestWorkflowFromLog(t *testing.T) {
	st := workflowSettings(t, func(s workspace.Settings) []change.Draft {
		todo, doing, done, cancel := s.Statuses[0].ID, s.Statuses[1].ID, s.Statuses[2].ID, s.Statuses[4].ID
		f := workspace.WorkflowField(todo)
		return []change.Draft{
			change.NewAdd(f, done), change.NewAdd(f, doing),
			change.NewRemove(f, done), change.NewAdd(f, cancel),
		}
	})
	got, restricted := st.Allowed(st.Statuses[0].ID)
	want := []change.ID{st.Statuses[1].ID, st.Statuses[4].ID} // ステータスの並び(InProgress・Cancel)
	if !restricted || !reflect.DeepEqual(got, want) {
		t.Errorf("Allowed(Todo) = %v %v, want %v", got, restricted, want)
	}
	if _, r := st.Allowed(st.Statuses[1].ID); r {
		t.Error("InProgress has no row and must be unrestricted")
	}
}

func TestCheckTransition(t *testing.T) {
	st := workflowSettings(t, func(s workspace.Settings) []change.Draft {
		f := workspace.WorkflowField(s.Statuses[0].ID)
		return []change.Draft{change.NewAdd(f, s.Statuses[1].ID), change.NewAdd(f, s.Statuses[3].ID)}
	})
	todo, doing, done, pending := st.Statuses[0].ID, st.Statuses[1].ID, st.Statuses[2].ID, st.Statuses[3].ID
	for _, ok := range [][2]change.ID{{todo, doing}, {todo, pending}, {todo, todo}, {done, todo}, {"", done}} {
		if err := workspace.CheckTransition(st, ok[0], ok[1]); err != nil {
			t.Errorf("%v: %v", ok, err)
		}
	}
	want := "ワークフローの制約: Todo から Done には変更できません(InProgress・Pending のみ)"
	if got := refusedWith(workspace.CheckTransition(st, todo, done)); got != want {
		t.Errorf("refusal = %q, want %q", got, want)
	}
}

// 消す行は、その行の許可先と、ほかの行のそのステータス。削除したステータスは許可先に数えない。
func TestWorkflowDeleteDrafts(t *testing.T) {
	var todo, doing, done change.ID
	st := workflowSettings(t, func(s workspace.Settings) []change.Draft {
		todo, doing, done = s.Statuses[0].ID, s.Statuses[1].ID, s.Statuses[2].ID
		return []change.Draft{
			change.NewAdd(workspace.WorkflowField(todo), doing),
			change.NewAdd(workspace.WorkflowField(doing), done),
			change.NewAdd(workspace.WorkflowField(doing), todo),
		}
	})
	want := []change.Draft{
		change.NewRemove(workspace.WorkflowField(doing), done),
		change.NewRemove(workspace.WorkflowField(doing), todo),
		change.NewRemove(workspace.WorkflowField(todo), doing),
	}
	if got := workspace.WorkflowDeleteDrafts(st, doing); !sameDrafts(got, want) {
		t.Errorf("drafts = %v, want %v", got, want)
	}
	after := workflowSettings(t, func(workspace.Settings) []change.Draft {
		return []change.Draft{
			change.NewAdd(workspace.WorkflowField(todo), doing),
			change.NewDelete("statuses/" + string(doing)),
		}
	})
	if _, r := after.Allowed(todo); r {
		t.Error("a row whose only target was deleted must be unrestricted")
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/workspace/ -run 'Workflow|CheckTransition'`
Expected: FAIL(`WorkflowField` が未定義)

- [ ] **Step 3: 実装する**

`settings.go` の `Settings` に、`CategoryDefaults` の次へ足す:

```go
	// Workflow は、遷移の制約(03「遷移の制約」)。From のステータス ID → 許可する To。行のない From は制限なし。
	Workflow map[change.ID][]change.ID
```

`workspace.go` の `opts.Multi` を次にする:

```go
	Multi: func(f string) bool { return strings.HasSuffix(f, "/kinds") || strings.HasPrefix(f, workflowPrefix) },
```

`settings_log.go`: `SettingsFromLog` の `for f, list := range st.Lists` の中で、`b.setKinds(f, list)` に続けて `b.setWorkflow(f, list)` を呼ぶ。`Workflow` は `newBuilder` で作らず、`setWorkflow` が書くときにだけ作る(行がなければ nil のまま)。既存のテスト(`settings_test.go` の `reflect.DeepEqual(got, s)`、`stores_contract_values.go` の照合)が、読み直した設定を `DefaultSettings` と丸ごと比べているため。

`workflow.go`:

```go
package workspace

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// 遷移の制約(03「遷移の制約」)。From のステータスごとの項目 workflow/<From> に、
// 許可する To を add・remove で持つ(B3-1)。

const workflowPrefix = "workflow/"

// WorkflowField は、From の行の項目名。
func WorkflowField(from change.ID) string { return workflowPrefix + string(from) }

func (b *builder) setWorkflow(f string, list []json.RawMessage) {
	from, ok := strings.CutPrefix(f, workflowPrefix)
	if !ok {
		return
	}
	for _, raw := range list {
		var to change.ID
		if json.Unmarshal(raw, &to) != nil {
			continue
		}
		if b.acc.Workflow == nil {
			b.acc.Workflow = map[change.ID][]change.ID{}
		}
		b.acc.Workflow[change.ID(from)] = append(b.acc.Workflow[change.ID(from)], to)
	}
}

// Allowed は、from から変更できる To を、ステータスの並びで返す。いまあるステータスだけを数え、
// 1 つもなければ制限なし(false)。
func (s Settings) Allowed(from change.ID) ([]change.ID, bool) {
	set := s.Workflow[from]
	var to []change.ID
	for _, v := range s.Statuses {
		if v.ID != from && slices.Contains(set, v.ID) {
			to = append(to, v.ID)
		}
	}
	return to, len(to) > 0
}

// CheckTransition は、from から to への変更を遷移の制約で検査する(B3-2)。同じステータス、
// from が空、制限のない行は通す。
func CheckTransition(s Settings, from, to change.ID) error {
	if from == "" || from == to {
		return nil
	}
	allowed, restricted := s.Allowed(from)
	if !restricted || slices.Contains(allowed, to) {
		return nil
	}
	names := make([]string, 0, len(allowed))
	for _, id := range allowed {
		names = append(names, statusName(s, id))
	}
	return refusal.New(fmt.Sprintf("ワークフローの制約: %s から %s には変更できません(%s のみ)",
		statusName(s, from), statusName(s, to), strings.Join(names, "・")))
}

// WorkflowDeleteDrafts は、ステータス id を表から外す行(B3-15)。id の行の許可先と、ほかの行の id。
func WorkflowDeleteDrafts(s Settings, id change.ID) []change.Draft {
	var ds []change.Draft
	for _, to := range s.Workflow[id] {
		ds = append(ds, change.NewRemove(WorkflowField(id), to))
	}
	froms := make([]change.ID, 0, len(s.Workflow))
	for from := range s.Workflow {
		froms = append(froms, from)
	}
	slices.Sort(froms)
	for _, from := range froms {
		if from != id && slices.Contains(s.Workflow[from], id) {
			ds = append(ds, change.NewRemove(WorkflowField(from), id))
		}
	}
	return ds
}
```

- [ ] **Step 4: 通るのを確かめる**

Run: `go test ./internal/domain/workspace/ && golangci-lint run ./internal/domain/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/workspace
git commit -m "feat(workspace): 遷移の制約を設定に読み、変更の可否を判定する(B3 Task 1)"
```

---

### Task 2: テンプレートの型と読み取り

**Files:**
- Create: `internal/domain/workspace/template.go`、`internal/domain/workspace/template_log.go`、Test: `internal/domain/workspace/template_log_test.go`
- Modify: `internal/domain/workspace/settings.go`(`Templates`・`TemplateDefaults`)、`internal/domain/workspace/settings_log.go`(`builder` に持たせ、`set` と `settings` から呼ぶ)

**Interfaces:**
- Consumes: Task 1 の `builder` の形(`b.deleted`・`b.acc`)、B2 の `byRank`
- Produces:

```go
// template.go
const (
	StepParent = "@parent" // Step の担当者「親と同じ」(初期値。B3-8)
	StepNone   = "@none"   // Step の担当者「未割り当て」
)

type TemplateStep struct {
	ID       change.ID
	Title    string
	Assignee string // StepParent・StepNone・メンバーのログイン名
	Rank     string
}

type TemplateChild struct {
	ID          change.ID
	Kind        string // Story・Task・Bug
	Title       string
	Description string
	Values      map[change.ID]json.RawMessage
	Steps       []TemplateStep // Rank の順
	Rank        string
}

type Template struct {
	ID          change.ID
	Name        string
	Kind        string // Epic・Story・Task・Bug・Step
	Title       string // タイトルの書き出し
	Description string
	Values      map[change.ID]json.RawMessage // プロパティの ID → 値(B3-5)。ない項目は「指定しない」
	Children    []TemplateChild               // Epic のときだけ。Rank の順
	Steps       []TemplateStep                // Story・Task・Bug のときだけ。Rank の順
}

func TemplateField(t change.ID, rest ...string) string  // "templates/<t>/<rest...>"
func (s Settings) Template(id change.ID) (Template, bool)
func HoldsTemplateValue(p Property) bool                 // テンプレートに持てるプロパティか(B3-6)

// settings.go の Settings に足す
Templates        []Template           // 内部 ID の順(作った順。B3-10)
TemplateDefaults map[string]change.ID // 種類 → 既定のテンプレート(B3-12)。「なし」は入れない
```

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/workspace/template_log_test.go`(行は手で書く。差分の行は Task 3):

```go
package workspace_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	tplA  change.ID = "01TPL0000000000000000000A0"
	tplB  change.ID = "01TPL0000000000000000000B0"
	child change.ID = "01CHILD00000000000000000C1"
	stepR change.ID = "01STEP000000000000000000R1"
	stepC change.ID = "01STEP000000000000000000C1"
	stepX change.ID = "01STEP000000000000000000X1"
)

func set(f string, v any) change.Draft { return change.NewSet(f, nil, v) }

func tf(id change.ID, rest ...string) string { return workspace.TemplateField(id, rest...) }

// Epic のテンプレート A(子 1 つ、子の Step 1 つ)と、Task のテンプレート B(Step 1 つ)を読む。
// 消した Step と、「なし」に戻した既定は出ない。
func TestTemplatesFromLog(t *testing.T) {
	st := workflowSettings(t, func(s workspace.Settings) []change.Draft {
		c, sc, sr, sx := string(child), string(stepC), string(stepR), string(stepX)
		return []change.Draft{
			set(tf(tplA, "name"), "リリース作業"), set(tf(tplA, "kind"), "Epic"), set(tf(tplA, "title"), "リリース "),
			set(tf(tplA, "values", "priority"), s.Priorities[1].ID),
			set(tf(tplA, "children", c, "kind"), "Story"), set(tf(tplA, "children", c, "title"), "QA"),
			set(tf(tplA, "children", c, "rank"), "V"), set(tf(tplA, "children", c, "values", "assignee"), "hanako"),
			set(tf(tplA, "steps", sc, "parent"), c), set(tf(tplA, "steps", sc, "title"), "確認"),
			set(tf(tplA, "steps", sc, "assignee"), workspace.StepParent), set(tf(tplA, "steps", sc, "rank"), "V"),
			set(tf(tplB, "name"), "調査"), set(tf(tplB, "kind"), "Task"),
			set(tf(tplB, "steps", sr, "parent"), ""), set(tf(tplB, "steps", sr, "title"), "再現"),
			set(tf(tplB, "steps", sr, "assignee"), workspace.StepNone), set(tf(tplB, "steps", sr, "rank"), "V"),
			set(tf(tplB, "steps", sx, "parent"), ""), set(tf(tplB, "steps", sx, "title"), "消す"),
			set(tf(tplB, "steps", sx, "rank"), "W"), change.NewDelete(tf(tplB, "steps", sx)),
			set("template_default/Epic", tplA), set("template_default/Task", tplB),
			change.NewSet("template_default/Task", tplB, nil),
		}
	})
	if len(st.Templates) != 2 || st.Templates[0].ID != tplA || st.Templates[1].ID != tplB {
		t.Fatalf("templates = %+v", st.Templates)
	}
	a := st.Templates[0]
	if a.Name != "リリース作業" || a.Kind != "Epic" || a.Title != "リリース " || len(a.Children) != 1 {
		t.Errorf("A = %+v", a)
	}
	if string(a.Values["priority"]) != `"`+string(st.Priorities[1].ID)+`"` {
		t.Errorf("A priority = %s", a.Values["priority"])
	}
	c := a.Children[0]
	if c.ID != child || c.Kind != "Story" || c.Title != "QA" || string(c.Values["assignee"]) != `"hanako"` {
		t.Errorf("child = %+v", c)
	}
	if len(c.Steps) != 1 || c.Steps[0].Title != "確認" || c.Steps[0].Assignee != workspace.StepParent {
		t.Errorf("child steps = %+v", c.Steps)
	}
	b := st.Templates[1]
	if len(b.Steps) != 1 || b.Steps[0].ID != stepR || b.Steps[0].Assignee != workspace.StepNone {
		t.Errorf("B steps = %+v", b.Steps)
	}
	if st.TemplateDefaults["Epic"] != tplA || st.TemplateDefaults["Task"] != "" {
		t.Errorf("defaults = %v", st.TemplateDefaults)
	}
}

// テンプレートを消すと出ず、その既定も出ない。子を消すと、その子の Step も出ない。null の値は「指定しない」。
func TestTemplatesDeleted(t *testing.T) {
	st := workflowSettings(t, func(workspace.Settings) []change.Draft {
		c, sc := string(child), string(stepC)
		return []change.Draft{
			set(tf(tplA, "name"), "A"), set(tf(tplA, "kind"), "Epic"),
			set(tf(tplA, "values", "systems"), []string{"x"}),
			change.NewSet(tf(tplA, "values", "systems"), []string{"x"}, nil),
			set(tf(tplA, "children", c, "kind"), "Task"), set(tf(tplA, "children", c, "title"), "子"),
			set(tf(tplA, "steps", sc, "parent"), c), set(tf(tplA, "steps", sc, "title"), "Step"),
			change.NewDelete(tf(tplA, "children", c)),
			set(tf(tplB, "name"), "B"), set(tf(tplB, "kind"), "Task"),
			set("template_default/Task", tplB), change.NewDelete(tf(tplB)),
		}
	})
	if len(st.Templates) != 1 || len(st.Templates[0].Children) != 0 || len(st.Templates[0].Values) != 0 {
		t.Fatalf("templates = %+v", st.Templates)
	}
	if _, ok := st.Template(tplB); ok || st.TemplateDefaults["Task"] != "" {
		t.Errorf("deleted template B is still visible: %v", st.TemplateDefaults)
	}
}

func TestHoldsTemplateValue(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	var got []string
	for _, p := range s.Properties {
		if workspace.HoldsTemplateValue(p) {
			got = append(got, p.Key)
		}
	}
	slices.Sort(got)
	if strings.Join(got, ",") != "assignee,priority,systems" {
		t.Errorf("holds = %v", got)
	}
	if !workspace.HoldsTemplateValue(workspace.Property{Key: "customer", Type: workspace.TypeText}) {
		t.Error("added properties are held")
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/workspace/ -run 'Templates|HoldsTemplate'`
Expected: FAIL(`TemplateField` が未定義)

- [ ] **Step 3: 実装する**

`template.go`(型は Interfaces のとおり。ほかに次の関数):

```go
// TemplateField は、テンプレート t の項目名 templates/<t>/<rest...>(B3-4)。rest がなければ templates/<t>。
func TemplateField(t change.ID, rest ...string) string {
	return strings.Join(append([]string{"templates", string(t)}, rest...), "/")
}

// Template は、id のテンプレート。
func (s Settings) Template(id change.ID) (Template, bool) {
	i := slices.IndexFunc(s.Templates, func(t Template) bool { return t.ID == id })
	if i < 0 {
		return Template{}, false
	}
	return s.Templates[i], true
}

// HoldsTemplateValue は、テンプレートに値を持てるプロパティか(B3-6)。組み込みは優先度・担当者・関連システムだけ。
func HoldsTemplateValue(p Property) bool {
	if !p.Builtin {
		return true
	}
	return p.Key == "priority" || p.Key == "assignee" || p.Key == "systems"
}
```

`template_log.go`(builder の部品。`settings_log.go` の `builder` に `tpl *templateBuilder` を持たせ、`newBuilder` で `tpl: newTemplateBuilder()` にする。`Templates`・`TemplateDefaults` は、1 つもなければ nil のままにする(Task 1 の `Workflow` と同じ理由)):

```go
package workspace

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// templateBuilder は、テンプレートの項目(B3-4)を集める。
type templateBuilder struct {
	tpls     map[change.ID]*Template
	children map[change.ID]map[change.ID]*TemplateChild // テンプレート → 子
	steps    map[change.ID]map[change.ID]*stepAcc       // テンプレート → Step
}

type stepAcc struct {
	TemplateStep
	parent change.ID // 空ならもとになるアイテム
}

func newTemplateBuilder() *templateBuilder {
	return &templateBuilder{
		tpls: map[change.ID]*Template{}, children: map[change.ID]map[change.ID]*TemplateChild{},
		steps: map[change.ID]map[change.ID]*stepAcc{},
	}
}

// set は、parts(templates/<T>/... を / で分けたもの)の値を入れる。
func (tb *templateBuilder) set(parts []string, raw json.RawMessage) {
	id := change.ID(parts[1])
	t := tb.template(id)
	switch {
	case len(parts) == 3:
		setTemplateField(t, parts[2], raw)
	case len(parts) == 4 && parts[2] == "values":
		putValue(&t.Values, parts[3], raw)
	case len(parts) == 5 && parts[2] == "children":
		setChildField(tb.child(id, parts[3]), parts[4], raw)
	case len(parts) == 6 && parts[2] == "children" && parts[4] == "values":
		putValue(&tb.child(id, parts[3]).Values, parts[5], raw)
	case len(parts) == 5 && parts[2] == "steps":
		setStepField(tb.step(id, parts[3]), parts[4], raw)
	}
}

func (tb *templateBuilder) template(id change.ID) *Template {
	t, ok := tb.tpls[id]
	if !ok {
		t = &Template{ID: id}
		tb.tpls[id] = t
	}
	return t
}

func (tb *templateBuilder) child(t change.ID, id string) *TemplateChild {
	if tb.children[t] == nil {
		tb.children[t] = map[change.ID]*TemplateChild{}
	}
	c, ok := tb.children[t][change.ID(id)]
	if !ok {
		c = &TemplateChild{ID: change.ID(id)}
		tb.children[t][change.ID(id)] = c
	}
	return c
}

func (tb *templateBuilder) step(t change.ID, id string) *stepAcc {
	if tb.steps[t] == nil {
		tb.steps[t] = map[change.ID]*stepAcc{}
	}
	s, ok := tb.steps[t][change.ID(id)]
	if !ok {
		s = &stepAcc{TemplateStep: TemplateStep{ID: change.ID(id)}}
		tb.steps[t][change.ID(id)] = s
	}
	return s
}

func setTemplateField(t *Template, field string, raw json.RawMessage) {
	switch field {
	case "name":
		_ = json.Unmarshal(raw, &t.Name)
	case "kind":
		_ = json.Unmarshal(raw, &t.Kind)
	case "title":
		_ = json.Unmarshal(raw, &t.Title)
	case "description":
		_ = json.Unmarshal(raw, &t.Description)
	}
}

func setChildField(c *TemplateChild, field string, raw json.RawMessage) {
	switch field {
	case "kind":
		_ = json.Unmarshal(raw, &c.Kind)
	case "title":
		_ = json.Unmarshal(raw, &c.Title)
	case "description":
		_ = json.Unmarshal(raw, &c.Description)
	case "rank":
		_ = json.Unmarshal(raw, &c.Rank)
	}
}

func setStepField(s *stepAcc, field string, raw json.RawMessage) {
	switch field {
	case "parent":
		_ = json.Unmarshal(raw, &s.parent)
	case "title":
		_ = json.Unmarshal(raw, &s.Title)
	case "assignee":
		_ = json.Unmarshal(raw, &s.Assignee)
	case "rank":
		_ = json.Unmarshal(raw, &s.Rank)
	}
}

// putValue は、値を入れる。null は「指定しない」なので、入っていた値を外す(B3-5)。
func putValue(m *map[change.ID]json.RawMessage, prop string, raw json.RawMessage) {
	if string(raw) == "null" {
		delete(*m, change.ID(prop))
		return
	}
	if *m == nil {
		*m = map[change.ID]json.RawMessage{}
	}
	(*m)[change.ID(prop)] = raw
}

// templates は、消したもの・名前のないもの・持ち主のないものを除いて並べた一覧。
func (tb *templateBuilder) templates(deleted map[string]bool) []Template {
	var out []Template // 1 つもなければ nil
	for id, t := range tb.tpls {
		if t.Name == "" || deleted[TemplateField(id)] {
			continue
		}
		tt := *t
		tt.Children = tb.liveChildren(id, deleted)
		tt.Steps = tb.liveSteps(id, "", deleted)
		for i := range tt.Children {
			tt.Children[i].Steps = tb.liveSteps(id, tt.Children[i].ID, deleted)
		}
		out = append(out, tt)
	}
	slices.SortFunc(out, func(a, b Template) int { return strings.Compare(string(a.ID), string(b.ID)) })
	return out
}

func (tb *templateBuilder) liveChildren(t change.ID, deleted map[string]bool) []TemplateChild {
	m := maps.Clone(tb.children[t])
	maps.DeleteFunc(m, func(id change.ID, c *TemplateChild) bool {
		return c.Kind == "" || c.Title == "" || deleted[TemplateField(t, "children", string(id))]
	})
	if len(m) == 0 {
		return nil
	}
	return byRank(m, func(c *TemplateChild) (string, change.ID) { return c.Rank, c.ID })
}

func (tb *templateBuilder) liveSteps(t, parent change.ID, deleted map[string]bool) []TemplateStep {
	m := map[change.ID]*TemplateStep{}
	for id, s := range tb.steps[t] {
		if s.parent == parent && s.Title != "" && !deleted[TemplateField(t, "steps", string(id))] {
			m[id] = &s.TemplateStep
		}
	}
	if len(m) == 0 {
		return nil
	}
	return byRank(m, func(s *TemplateStep) (string, change.ID) { return s.Rank, s.ID })
}
```

`settings_log.go`:
- `builder.set` の `switch` に 2 つの `case` を足す: `case len(parts) >= 3 && parts[0] == "templates": b.tpl.set(parts, raw)`。`case len(parts) == 2 && parts[0] == "template_default":` は、`slices.Contains(Kinds, parts[1])` のときだけ、値が `null` なら `delete(b.acc.TemplateDefaults, parts[1])`、そうでなければ(map が nil なら作ってから)`change.ID` に読んで入れる。
- `builder.settings()` の末尾(`b.attachChoices` のあと)で `s.Templates = b.tpl.templates(b.deleted)` とし、続けて `maps.DeleteFunc(s.TemplateDefaults, func(_ string, id change.ID) bool { _, ok := s.Template(id); return !ok })`(消したテンプレートを指す既定を出さない)。消した結果が空なら `s.TemplateDefaults = nil`。
- `go test ./internal/app/...` も流し、照合テスト(`stores_contract_values.go`)が通ることを確かめる。
- `settings_log.go` が 300 行を超えるなら、`set` の `templates` と `template_default` の分岐を `template_log.go` の `func (b *builder) setTemplate(parts []string, raw json.RawMessage) bool`(扱ったら true)に移し、`set` の先頭で呼ぶ。

- [ ] **Step 4: 通るのを確かめる**

Run: `go test ./internal/domain/workspace/ && golangci-lint run ./internal/domain/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/workspace
git commit -m "feat(workspace): テンプレートと種類ごとの既定を設定に読む(B3 Task 2)"
```

---

### Task 3: テンプレートの整え方・検査・行

**Files:**
- Create: `internal/domain/workspace/template_check.go`、`internal/domain/workspace/template_drafts.go`、Test: `internal/domain/workspace/template_check_test.go`、`internal/domain/workspace/template_drafts_test.go`

**Interfaces:**
- Consumes: Task 2 の型、B2 の `NormalizeName`、`domain/rank` の `After`
- Produces:

```go
// template_check.go
func NormalizeTemplate(s Settings, t Template) Template           // B3-6・B3-11 の整え方
func CheckTemplate(s Settings, t Template) error                   // 名前の空・重複(t は整えたあと)
func DroppedByKind(t Template, kind string) (children, steps int)  // 種類を kind にしたら捨てる構成の数(B3-14)
func ValidTemplate(s Settings, t Template) bool                    // 種類・子の種類・値の参照が正しいか(false は ErrNotFound)

// template_drafts.go
func (t Template) Ranked() Template                                // 子と Step の並びを振り直す(B3-10)
func TemplateDrafts(old, t Template) []change.Draft                // old から t への行(old がゼロ値なら作成)
func TemplateDeleteDrafts(s Settings, id change.ID) []change.Draft // 削除の行と、指していた既定を「なし」に
func DefaultClearDrafts(s Settings, id change.ID, keep string) []change.Draft // id を指す既定(keep の種類以外)を「なし」に
```

- [ ] **Step 1: 失敗するテストを書く**

`template_check_test.go`:

```go
package workspace_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

const envProp change.ID = "01P0000000000000000000000E"

// 名前の前後の空白を除き(タイトルの書き出しはそのまま)、空の Step を捨て、種類で使わない値・持たない値・持てない構成を捨てる。
func TestNormalizeTemplate(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	s.Properties = append(s.Properties, workspace.Property{ID: envProp, Key: "env", Name: "環境", Type: workspace.TypeText, Kinds: []string{"Story"}})
	in := workspace.Template{
		Name: " 調査 ", Kind: "Task", Title: " 調査: ",
		Values: map[change.ID]json.RawMessage{
			"priority": raw(s.Priorities[0].ID), envProp: raw("本番"), "status": raw(s.Statuses[0].ID), "systems": raw([]string{}),
		},
		Steps:    []workspace.TemplateStep{{Title: " 再現 "}, {Title: "  "}},
		Children: []workspace.TemplateChild{{Kind: "Story", Title: "捨てる"}},
	}
	got := workspace.NormalizeTemplate(s, in)
	if got.Name != "調査" || got.Title != " 調査: " {
		t.Errorf("name/title = %q %q", got.Name, got.Title)
	}
	if len(got.Values) != 1 || got.Values["priority"] == nil {
		t.Errorf("values = %v", got.Values)
	}
	if len(got.Steps) != 1 || got.Steps[0].Title != "再現" || got.Steps[0].Assignee != workspace.StepParent || got.Children != nil {
		t.Errorf("structure = %+v %+v", got.Steps, got.Children)
	}
}

func TestCheckTemplate(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	s.Templates = []workspace.Template{{ID: tplA, Name: "調査", Kind: "Task"}}
	cases := []struct {
		name, want string
		id         change.ID
	}{
		{"", "名前を入力してください", ""},
		{"調査", "同じ名前のテンプレートがあります", ""},
		{"調査", "", tplA}, // 自分の名前のまま
		{"調査 2", "", ""},
		{"ちょうさ", "", ""},
	}
	for _, c := range cases {
		if got := refusedWith(workspace.CheckTemplate(s, workspace.Template{ID: c.id, Name: c.name, Kind: "Task"})); got != c.want {
			t.Errorf("%q: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDroppedByKind(t *testing.T) {
	epic := workspace.Template{Kind: "Epic", Children: []workspace.TemplateChild{
		{Title: "QA", Steps: []workspace.TemplateStep{{Title: "a"}, {Title: "b"}}}, {Title: "本番"},
	}}
	task := workspace.Template{Kind: "Task", Steps: []workspace.TemplateStep{{Title: "a"}}}
	for _, c := range []struct {
		t        workspace.Template
		kind     string
		children int
		steps    int
	}{{epic, "Task", 2, 2}, {epic, "Epic", 0, 0}, {task, "Bug", 0, 0}, {task, "Step", 0, 1}, {task, "Epic", 0, 1}} {
		if ch, st := workspace.DroppedByKind(c.t, c.kind); ch != c.children || st != c.steps {
			t.Errorf("%s→%s = %d %d", c.t.Kind, c.kind, ch, st)
		}
	}
}

func TestValidTemplate(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	ok := workspace.Template{Name: "a", Kind: "Epic", Values: map[change.ID]json.RawMessage{
		"priority": raw(s.Priorities[0].ID), "systems": raw([]change.ID{s.Systems[0].ID}), "assignee": raw("taro"),
	}, Children: []workspace.TemplateChild{{Kind: "Story", Title: "QA"}}}
	if !workspace.ValidTemplate(s, ok) {
		t.Error("valid template refused")
	}
	for name, bad := range map[string]workspace.Template{
		"kind":       {Name: "a", Kind: "Sprint"},
		"child kind": {Name: "a", Kind: "Epic", Children: []workspace.TemplateChild{{Kind: "Epic", Title: "x"}}},
		"priority":   {Name: "a", Kind: "Task", Values: map[change.ID]json.RawMessage{"priority": raw("nope")}},
		"systems":    {Name: "a", Kind: "Task", Values: map[change.ID]json.RawMessage{"systems": raw([]string{"nope"})}},
		"property":   {Name: "a", Kind: "Task", Values: map[change.ID]json.RawMessage{"nope": raw("x")}},
		"status":     {Name: "a", Kind: "Task", Values: map[change.ID]json.RawMessage{"status": raw(s.Statuses[0].ID)}},
	} {
		if workspace.ValidTemplate(s, bad) {
			t.Errorf("%s: invalid template accepted", name)
		}
	}
}
```

`template_drafts_test.go`:

```go
package workspace_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const child2 change.ID = "01CHILD00000000000000000C2"

// 作成の行を読み直すと、同じテンプレートになる。並びは送った順に V・W…。
func TestTemplateDraftsRoundTrip(t *testing.T) {
	in := workspace.Template{
		ID: tplA, Name: "リリース作業", Kind: "Epic", Title: "リリース", Description: "## 手順",
		Values: map[change.ID]json.RawMessage{"assignee": raw("taro")},
		Children: []workspace.TemplateChild{
			{ID: child, Kind: "Story", Title: "QA", Steps: []workspace.TemplateStep{{ID: stepC, Title: "確認", Assignee: workspace.StepParent}}},
			{ID: child2, Kind: "Task", Title: "本番反映"},
		},
	}.Ranked()
	if in.Children[0].Rank != "V" || in.Children[1].Rank != "W" || in.Children[0].Steps[0].Rank != "V" {
		t.Errorf("ranks = %+v", in.Children)
	}
	st := workflowSettings(t, func(workspace.Settings) []change.Draft {
		return workspace.TemplateDrafts(workspace.Template{}, in)
	})
	got, ok := st.Template(tplA)
	if !ok || got.Name != in.Name || got.Description != "## 手順" || len(got.Children) != 2 ||
		got.Children[1].Title != "本番反映" || len(got.Children[0].Steps) != 1 || string(got.Values["assignee"]) != `"taro"` {
		t.Errorf("got %+v", got)
	}
}

// 変えた項目だけを行にする。消した子は削除の行、外した値は null。
func TestTemplateDraftsDiff(t *testing.T) {
	old := workspace.Template{
		ID: tplA, Name: "A", Kind: "Epic", Title: "t",
		Values:   map[change.ID]json.RawMessage{"assignee": raw("taro"), "priority": raw("P1")},
		Children: []workspace.TemplateChild{{ID: child, Kind: "Story", Title: "QA", Rank: "V"}, {ID: child2, Kind: "Task", Title: "x", Rank: "W"}},
	}
	next := old
	next.Title = "t2"
	next.Values = map[change.ID]json.RawMessage{"assignee": json.RawMessage(` "taro" `)} // 空白の違いは同じ値
	next.Children = []workspace.TemplateChild{{ID: child, Kind: "Story", Title: "QA", Rank: "V"}}
	want := []change.Draft{
		change.NewSet(tf(tplA, "title"), "t", "t2"),
		change.NewSet(tf(tplA, "values", "priority"), raw("P1"), nil),
		change.NewDelete(tf(tplA, "children", string(child2))),
	}
	if got := workspace.TemplateDrafts(old, next); !sameDrafts(got, want) {
		t.Errorf("drafts = %v\nwant %v", got, want)
	}
}

// 削除は、テンプレートの削除の行と、指していた既定を「なし」にする行。
func TestTemplateDeleteDrafts(t *testing.T) {
	s := workspace.Settings{
		Templates:        []workspace.Template{{ID: tplA, Name: "A", Kind: "Task"}},
		TemplateDefaults: map[string]change.ID{"Task": tplA, "Bug": tplB},
	}
	want := []change.Draft{change.NewDelete(tf(tplA)), change.NewSet("template_default/Task", tplA, nil)}
	if got := workspace.TemplateDeleteDrafts(s, tplA); !sameDrafts(got, want) {
		t.Errorf("drafts = %v", got)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/workspace/ -run 'Template|DroppedByKind'`
Expected: FAIL(`NormalizeTemplate` などが未定義)

- [ ] **Step 3: 実装する**

`template_check.go`:

```go
package workspace

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// childKinds は、Epic の子に置ける種類(01「用語と階層」)。
var childKinds = []string{"Story", "Task", "Bug"}

// hasSteps は、もとになるアイテムが Step を持てる種類か。
func hasSteps(kind string) bool { return slices.Contains(childKinds, kind) }

// NormalizeTemplate は、保存する形にする(B3-6・B3-11): 名前の前後の空白を除き(タイトルの書き出しはそのまま)、
// タイトルが空の子と Step を捨て、種類で持てない構成と、持たない・種類で使わないプロパティの値を捨てる。
// Step の担当者が空なら「親と同じ」。
func NormalizeTemplate(s Settings, t Template) Template {
	t.Name = NormalizeName(t.Name)
	t.Values = keepValues(s, t.Kind, t.Values)
	steps, cs := t.Steps, t.Children
	t.Steps, t.Children = nil, nil
	if hasSteps(t.Kind) {
		t.Steps = normalizeSteps(steps)
	}
	if t.Kind == "Epic" {
		t.Children = normalizeChildren(s, cs)
	}
	return t
}

func normalizeChildren(s Settings, cs []TemplateChild) []TemplateChild {
	var out []TemplateChild
	for _, c := range cs {
		c.Title = strings.TrimSpace(c.Title)
		if c.Title == "" {
			continue
		}
		c.Values = keepValues(s, c.Kind, c.Values)
		c.Steps = normalizeSteps(c.Steps)
		out = append(out, c)
	}
	return out
}

func normalizeSteps(ss []TemplateStep) []TemplateStep {
	var out []TemplateStep
	for _, st := range ss {
		st.Title = strings.TrimSpace(st.Title)
		if st.Title == "" {
			continue
		}
		if st.Assignee == "" {
			st.Assignee = StepParent
		}
		out = append(out, st)
	}
	return out
}

// keepValues は、kind の種類で使い、テンプレートに持てるプロパティの値だけを残す。空の値は値なし。
func keepValues(s Settings, kind string, vs map[change.ID]json.RawMessage) map[change.ID]json.RawMessage {
	var out map[change.ID]json.RawMessage
	for id, v := range vs {
		p, ok := s.Property(id)
		if !ok || !HoldsTemplateValue(p) || !slices.Contains(p.Kinds, kind) || emptyValue(v) {
			continue
		}
		if out == nil {
			out = map[change.ID]json.RawMessage{}
		}
		out[id] = v
	}
	return out
}

func emptyValue(v json.RawMessage) bool {
	s := strings.TrimSpace(string(v))
	return s == "" || s == "null" || s == "[]" || s == `""`
}

// CheckTemplate は、保存の検査(03「一覧と編集」)。t は NormalizeTemplate のあと。
func CheckTemplate(s Settings, t Template) error {
	if t.Name == "" {
		return refusal.New("名前を入力してください")
	}
	for _, o := range s.Templates {
		if o.ID != t.ID && o.Name == t.Name {
			return refusal.New("同じ名前のテンプレートがあります")
		}
	}
	return nil
}

// DroppedByKind は、t を kind の種類で保存したときに捨てる子と Step の数(B3-14)。
func DroppedByKind(t Template, kind string) (children, steps int) {
	if kind != "Epic" {
		children = len(t.Children)
		for _, c := range t.Children {
			steps += len(c.Steps)
		}
	}
	if !hasSteps(kind) {
		steps += len(t.Steps)
	}
	return children, steps
}

// ValidTemplate は、種類・子の種類・値の参照が正しいか。画面からは起きないので、false は ErrNotFound にする(B2-12)。
func ValidTemplate(s Settings, t Template) bool {
	if !slices.Contains(Kinds, t.Kind) || !validValues(s, t.Values) {
		return false
	}
	for _, c := range t.Children {
		if !slices.Contains(childKinds, c.Kind) || !validValues(s, c.Values) {
			return false
		}
	}
	return true
}

func validValues(s Settings, vs map[change.ID]json.RawMessage) bool {
	for id, v := range vs {
		p, ok := s.Property(id)
		if !ok || !HoldsTemplateValue(p) || !validValue(s, p, v) {
			return false
		}
	}
	return true
}

// validValue は、値の ID がその一覧にあるか。ほかの型は形を問わない(画面が型どおりに送る)。
func validValue(s Settings, p Property, v json.RawMessage) bool {
	switch {
	case p.Key == "priority" && p.Builtin:
		return validRef(s.Priorities, v)
	case p.Key == "systems" && p.Builtin:
		return validRefs(s.Systems, v)
	case p.Type == TypeSingle:
		return validRef(p.Choices, v)
	case p.Type == TypeMulti:
		return validRefs(p.Choices, v)
	}
	return true
}

func validRef(vs []Value, v json.RawMessage) bool {
	var id change.ID
	return json.Unmarshal(v, &id) == nil && hasValue(vs, id)
}

func validRefs(vs []Value, v json.RawMessage) bool {
	var ids []change.ID
	if json.Unmarshal(v, &ids) != nil {
		return false
	}
	for _, id := range ids {
		if !hasValue(vs, id) {
			return false
		}
	}
	return true
}

func hasValue(vs []Value, id change.ID) bool {
	return slices.ContainsFunc(vs, func(v Value) bool { return v.ID == id })
}
```

`template_drafts.go`:

```go
package workspace

import (
	"bytes"
	"encoding/json"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

// Ranked は、子と Step の並びを、いまの並びで rank.First から振り直す(B3-10)。
func (t Template) Ranked() Template {
	t.Children = slices.Clone(t.Children)
	r := ""
	for i := range t.Children {
		r = rank.After(r)
		t.Children[i].Rank = r
		t.Children[i].Steps = rankSteps(t.Children[i].Steps)
	}
	t.Steps = rankSteps(t.Steps)
	return t
}

func rankSteps(ss []TemplateStep) []TemplateStep {
	ss = slices.Clone(ss)
	r := ""
	for i := range ss {
		r = rank.After(r)
		ss[i].Rank = r
	}
	return ss
}

// TemplateDrafts は、old から t への行(B3-4・B3-9)。変わった項目だけを書く。old がゼロ値なら作成で、
// 空でない項目をすべて書く。old にあって t にない子と Step は削除の行(消した子の Step は書かない)。
func TemplateDrafts(old, t Template) []change.Draft {
	fresh := old.ID == ""
	f := func(rest ...string) string { return TemplateField(t.ID, rest...) }
	var ds []change.Draft
	ds = setText(ds, f("name"), old.Name, t.Name, fresh)
	ds = setText(ds, f("kind"), old.Kind, t.Kind, fresh)
	ds = setText(ds, f("title"), old.Title, t.Title, fresh)
	ds = setText(ds, f("description"), old.Description, t.Description, fresh)
	ds = setValues(ds, f("values"), old.Values, t.Values)
	ds = childDrafts(ds, t.ID, old.Children, t.Children)
	ds = stepDrafts(ds, t.ID, "", old.Steps, t.Steps)
	for _, c := range t.Children {
		var oldSteps []TemplateStep
		if i := slices.IndexFunc(old.Children, func(o TemplateChild) bool { return o.ID == c.ID }); i >= 0 {
			oldSteps = old.Children[i].Steps
		}
		ds = stepDrafts(ds, t.ID, c.ID, oldSteps, c.Steps)
	}
	return ds
}

// setText は、文字の項目の行。fresh(新しいもの)なら空でないときだけ、from を null にして書く。
func setText(ds []change.Draft, f, from, to string, fresh bool) []change.Draft {
	switch {
	case fresh && to == "":
		return ds
	case fresh:
		return append(ds, change.NewSet(f, nil, to))
	case from == to:
		return ds
	}
	return append(ds, change.NewSet(f, from, to))
}

// setValues は、値の行。足した値・変わった値・なくなった値(null)を、キーの順に書く。
func setValues(ds []change.Draft, prefix string, old, next map[change.ID]json.RawMessage) []change.Draft {
	keys := make([]change.ID, 0, len(old)+len(next))
	for k := range old {
		keys = append(keys, k)
	}
	for k := range next {
		if _, ok := old[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		o, had := old[k]
		n, has := next[k]
		f := prefix + "/" + string(k)
		switch {
		case has && !had:
			ds = append(ds, change.NewSet(f, nil, n))
		case has && !sameJSON(o, n):
			ds = append(ds, change.NewSet(f, o, n))
		case !has:
			ds = append(ds, change.NewSet(f, o, nil))
		}
	}
	return ds
}

func sameJSON(a, b json.RawMessage) bool {
	var ca, cb bytes.Buffer
	if json.Compact(&ca, a) != nil || json.Compact(&cb, b) != nil {
		return bytes.Equal(a, b)
	}
	return bytes.Equal(ca.Bytes(), cb.Bytes())
}

func childDrafts(ds []change.Draft, t change.ID, old, next []TemplateChild) []change.Draft {
	for _, c := range next {
		i := slices.IndexFunc(old, func(o TemplateChild) bool { return o.ID == c.ID })
		var o TemplateChild
		if i >= 0 {
			o = old[i]
		}
		f := func(rest ...string) string {
			return TemplateField(t, append([]string{"children", string(c.ID)}, rest...)...)
		}
		ds = setText(ds, f("kind"), o.Kind, c.Kind, i < 0)
		ds = setText(ds, f("title"), o.Title, c.Title, i < 0)
		ds = setText(ds, f("description"), o.Description, c.Description, i < 0)
		ds = setText(ds, f("rank"), o.Rank, c.Rank, i < 0)
		ds = setValues(ds, f("values"), o.Values, c.Values)
	}
	for _, o := range old {
		if !slices.ContainsFunc(next, func(c TemplateChild) bool { return c.ID == o.ID }) {
			ds = append(ds, change.NewDelete(TemplateField(t, "children", string(o.ID))))
		}
	}
	return ds
}

// stepDrafts は、持ち主 parent(空ならもと)の Step の行。新しい Step は parent も書く。
func stepDrafts(ds []change.Draft, t, parent change.ID, old, next []TemplateStep) []change.Draft {
	for _, s := range next {
		i := slices.IndexFunc(old, func(o TemplateStep) bool { return o.ID == s.ID })
		var o TemplateStep
		if i >= 0 {
			o = old[i]
		} else {
			ds = append(ds, change.NewSet(TemplateField(t, "steps", string(s.ID), "parent"), nil, string(parent)))
		}
		f := func(field string) string { return TemplateField(t, "steps", string(s.ID), field) }
		ds = setText(ds, f("title"), o.Title, s.Title, i < 0)
		ds = setText(ds, f("assignee"), o.Assignee, s.Assignee, i < 0)
		ds = setText(ds, f("rank"), o.Rank, s.Rank, i < 0)
	}
	for _, o := range old {
		if !slices.ContainsFunc(next, func(s TemplateStep) bool { return s.ID == o.ID }) {
			ds = append(ds, change.NewDelete(TemplateField(t, "steps", string(o.ID))))
		}
	}
	return ds
}

// TemplateDeleteDrafts は、テンプレート id の削除の行と、id を指していた種類ごとの既定を「なし」にする行(B3-12)。
func TemplateDeleteDrafts(s Settings, id change.ID) []change.Draft {
	return append([]change.Draft{change.NewDelete(TemplateField(id))}, DefaultClearDrafts(s, id, "")...)
}

// DefaultClearDrafts は、id を指す種類ごとの既定のうち、keep の種類以外を「なし」にする行。
// 種類を変えたときは keep に新しい種類を渡す。
func DefaultClearDrafts(s Settings, id change.ID, keep string) []change.Draft {
	var ds []change.Draft
	for _, k := range Kinds {
		if k != keep && s.TemplateDefaults[k] == id {
			ds = append(ds, change.NewSet("template_default/"+k, id, nil))
		}
	}
	return ds
}
```

- [ ] **Step 4: 通るのを確かめる**

Run: `go test ./internal/domain/workspace/ && golangci-lint run ./internal/domain/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/workspace
git commit -m "feat(workspace): テンプレートの整え方・検査・差分の行を足す(B3 Task 3)"
```

---

### Task 4: 遷移の制約の操作

**Files:**
- Create: `internal/app/workspaces/workflow.go`、Test: `internal/app/workspaces/workflow_test.go`
- Modify: `internal/app/workspaces/summary_test.go`(文の確かめに 1 行)

**Interfaces:**
- Consumes: Task 1 の `WorkflowField`・`CheckTransition`、B2 の `Service.edit`
- Produces:

```go
// SetTransition は、遷移の制約の表のセル(from の行の to の列)のチェックを付ける(allowed)・外す。
// from と to が同じ、ないステータスは ErrNotFound。いまと同じなら何も保存しない。
func (s Service) SetTransition(ctx context.Context, store port.StoreID, ws workspace.ID, from, to change.ID, allowed bool) error
```

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/workspaces/workflow_test.go`:

```go
package workspaces_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Todo の行に InProgress をチェックすると、Todo から Done へは変えられなくなる。外すと戻る。
func TestSetTransition(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	todo, doing, done := st.Statuses[0].ID, st.Statuses[1].ID, st.Statuses[2].ID
	if err := s.SetTransition(ctx, store, w.ID, todo, doing, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTransition(ctx, store, w.ID, todo, doing, true); err != nil { // 同じ: 何もしない
		t.Fatal(err)
	}
	st = settingsOf(t, s, w.ID)
	rejected(t, workspace.CheckTransition(st, todo, done), "ワークフローの制約: Todo から Done には変更できません(InProgress のみ)")
	if err := s.SetTransition(ctx, store, w.ID, todo, doing, false); err != nil {
		t.Fatal(err)
	}
	if _, r := settingsOf(t, s, w.ID).Allowed(todo); r {
		t.Error("the row must be unrestricted again")
	}
	for _, pair := range [][2]change.ID{{todo, todo}, {todo, "nope"}, {"nope", todo}} {
		if err := s.SetTransition(ctx, store, w.ID, pair[0], pair[1], true); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%v: %v", pair, err)
		}
	}
}
```

`summary_test.go` の `TestValueActivitySummaries` の操作の並びの末尾に、`s.SetTransition(ctx, store, w.ID, <Todo>, <InProgress>, true)` を足し、期待する文の末尾に `"ワークフローの遷移の制約を変更"` を足す(ステータスの ID は、そのテストの中で `settingsOf` から取る)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/workspaces/ -run 'SetTransition|ActivitySummaries'`
Expected: FAIL(`SetTransition` が未定義)

- [ ] **Step 3: 実装する**

`internal/app/workspaces/workflow.go`:

```go
package workspaces

import (
	"context"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// SetTransition は、遷移の制約の表のセルのチェックを付ける・外す(03「遷移の制約」。チェックのたびに保存)。
// 既存のアイテムは変えない(03「変更が及ぶ範囲」)。
func (s Service) SetTransition(ctx context.Context, store port.StoreID, ws workspace.ID, from, to change.ID, allowed bool) error {
	return s.edit(ctx, store, ws,
		func(st workspace.Settings) (string, error) {
			if err := checkPair(st, from, to); err != nil {
				return "", err
			}
			return "ワークフローの遷移の制約を変更", nil
		},
		func(_ port.Tx, st workspace.Settings) ([]change.Draft, error) {
			if err := checkPair(st, from, to); err != nil {
				return nil, err
			}
			has := slices.Contains(st.Workflow[from], to)
			switch {
			case allowed && !has:
				return []change.Draft{change.NewAdd(workspace.WorkflowField(from), to)}, nil
			case !allowed && has:
				return []change.Draft{change.NewRemove(workspace.WorkflowField(from), to)}, nil
			}
			return nil, nil
		})
}

// checkPair は、表のセルになる組か(どちらもあるステータスで、同じでない)。
func checkPair(st workspace.Settings, from, to change.ID) error {
	has := func(id change.ID) bool {
		return slices.ContainsFunc(st.Statuses, func(v workspace.Value) bool { return v.ID == id })
	}
	if from == to || !has(from) || !has(to) {
		return port.ErrNotFound
	}
	return nil
}
```

- [ ] **Step 4: 通るのを確かめる**

Run: `go test ./internal/app/workspaces/ && golangci-lint run ./internal/app/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/workspaces
git commit -m "feat(app): 遷移の制約の表のチェックを付け外しする(B3 Task 4)"
```

---

### Task 5: テンプレートの作成・編集・削除・種類ごとの既定

**Files:**
- Create: `internal/app/workspaces/template.go`、Test: `internal/app/workspaces/template_test.go`
- Modify: `internal/app/workspaces/summary_test.go`(文の確かめ)

**Interfaces:**
- Consumes: Task 2・3 の `Template`・`NormalizeTemplate`・`CheckTemplate`・`ValidTemplate`・`TemplateDrafts`・`TemplateDeleteDrafts`・`DefaultClearDrafts`・`Ranked`
- Produces:

```go
// 入力は workspace.Template。子と Step の ID は、いまあるものはその ID、新しいものは空。
// Values の null は「指定しない」に戻す。編集で送らなかった値は残す(B3-9)。
func (s Service) CreateTemplate(ctx context.Context, store port.StoreID, ws workspace.ID, in workspace.Template) (workspace.Template, error)
func (s Service) UpdateTemplate(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, in workspace.Template) error
func (s Service) DeleteTemplate(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID) error
// SetTemplateDefault は、種類 kind の既定のテンプレートを変える。id が空なら「なし」。
func (s Service) SetTemplateDefault(ctx context.Context, store port.StoreID, ws workspace.ID, kind string, id change.ID) error
```

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/workspaces/template_test.go`:

```go
package workspaces_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func jsonOf(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

// Epic のテンプレートを、子 2 つ(1 つは Step 付き)で作る。名前の重複は断る。
func TestCreateTemplate(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	in := workspace.Template{
		Name: " リリース作業 ", Kind: "Epic", Title: "リリース ", Description: "## 手順",
		Values: map[change.ID]json.RawMessage{"priority": jsonOf(st.Priorities[1].ID), "assignee": json.RawMessage("null")},
		Children: []workspace.TemplateChild{
			{Kind: "Story", Title: "QA", Steps: []workspace.TemplateStep{{Title: "確認"}, {Title: " "}}},
			{Kind: "Task", Title: "本番反映"},
			{Kind: "Task", Title: ""},
		},
	}
	got, err := s.CreateTemplate(ctx, store, w.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := settingsOf(t, s, w.ID).Template(got.ID)
	if !ok || saved.Name != "リリース作業" || saved.Title != "リリース " || len(saved.Children) != 2 ||
		len(saved.Children[0].Steps) != 1 || saved.Children[0].Steps[0].Assignee != workspace.StepParent ||
		len(saved.Values) != 1 {
		t.Errorf("saved = %+v", saved)
	}
	_, err = s.CreateTemplate(ctx, store, w.ID, workspace.Template{Name: "リリース作業", Kind: "Task"})
	rejected(t, err, "同じ名前のテンプレートがあります")
	_, err = s.CreateTemplate(ctx, store, w.ID, workspace.Template{Name: " ", Kind: "Task"})
	rejected(t, err, "名前を入力してください")
	if _, err := s.CreateTemplate(ctx, store, w.ID, workspace.Template{Name: "x", Kind: "Sprint"}); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("bad kind: %v", err)
	}
}

// 編集: 送らなかった値は残り、null は外れる。消した Step は消え、種類を変えると既定から外れる。
func TestUpdateTemplate(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	tpl, err := s.CreateTemplate(ctx, store, w.ID, workspace.Template{
		Name: "調査", Kind: "Task",
		Values: map[change.ID]json.RawMessage{"priority": jsonOf(st.Priorities[0].ID), "assignee": jsonOf("taro")},
		Steps:  []workspace.TemplateStep{{Title: "再現"}, {Title: "原因"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetTemplateDefault(ctx, store, w.ID, "Task", tpl.ID); err != nil {
		t.Fatal(err)
	}
	keep := tpl.Steps[1]
	err = s.UpdateTemplate(ctx, store, w.ID, tpl.ID, workspace.Template{
		Name: "調査 2", Kind: "Task",
		Values: map[change.ID]json.RawMessage{"assignee": json.RawMessage("null")}, // 優先度は送らない
		Steps:  []workspace.TemplateStep{keep, {Title: "報告", Assignee: workspace.StepNone}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := settingsOf(t, s, w.ID).Template(tpl.ID)
	if got.Name != "調査 2" || string(got.Values["priority"]) != string(jsonOf(st.Priorities[0].ID)) || got.Values["assignee"] != nil {
		t.Errorf("values = %+v", got)
	}
	if len(got.Steps) != 2 || got.Steps[0].ID != keep.ID || got.Steps[1].Title != "報告" {
		t.Errorf("steps = %+v", got.Steps)
	}
	if err := s.UpdateTemplate(ctx, store, w.ID, tpl.ID, workspace.Template{Name: "調査 2", Kind: "Bug"}); err != nil {
		t.Fatal(err)
	}
	after := settingsOf(t, s, w.ID)
	if after.TemplateDefaults["Task"] != "" {
		t.Errorf("Task default must be cleared: %v", after.TemplateDefaults)
	}
	if err := s.UpdateTemplate(ctx, store, w.ID, "nope", workspace.Template{Name: "x", Kind: "Task"}); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

// 削除で既定からも外れる。既定は同じ種類のテンプレートだけ。
func TestDeleteTemplateAndDefaults(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	tpl, _ := s.CreateTemplate(ctx, store, w.ID, workspace.Template{Name: "不具合", Kind: "Bug"})
	if err := s.SetTemplateDefault(ctx, store, w.ID, "Task", tpl.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("other kind: %v", err)
	}
	if err := s.SetTemplateDefault(ctx, store, w.ID, "Sprint", ""); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("bad kind: %v", err)
	}
	if err := s.SetTemplateDefault(ctx, store, w.ID, "Bug", tpl.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTemplate(ctx, store, w.ID, tpl.ID); err != nil {
		t.Fatal(err)
	}
	st := settingsOf(t, s, w.ID)
	if len(st.Templates) != 0 || st.TemplateDefaults["Bug"] != "" {
		t.Errorf("after delete = %+v %v", st.Templates, st.TemplateDefaults)
	}
	if err := s.DeleteTemplate(ctx, store, w.ID, tpl.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("twice: %v", err)
	}
}
```

`summary_test.go` に `TestTemplateActivitySummaries` を足す。`recorder` を差し込んだサービスで、`CreateTemplate`(名前「調査」、種類 Task)、`UpdateTemplate`(名前「調査 2」、種類 Task)、`SetTemplateDefault`(Task、そのテンプレート)、`DeleteTemplate` の順に呼び、`rec.summaries` の末尾 4 つが `テンプレート「調査」を作成`、`テンプレート「調査 2」を編集`、`Task の既定のテンプレートを変更`、`テンプレート「調査 2」を削除` であることを確かめる。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/workspaces/ -run 'Template'`
Expected: FAIL(`CreateTemplate` が未定義)

- [ ] **Step 3: 実装する**

`internal/app/workspaces/template.go`:

```go
package workspaces

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// CreateTemplate は、テンプレートを作る(03「一覧と編集」の「テンプレートを作成」)。作ったものを返す。
func (s Service) CreateTemplate(ctx context.Context, store port.StoreID, ws workspace.ID, in workspace.Template) (workspace.Template, error) {
	in.ID = s.IDs.New()
	in = s.withIDs(in)
	in.Values = mergeValues(nil, in.Values)
	for i := range in.Children {
		in.Children[i].Values = mergeValues(nil, in.Children[i].Values)
	}
	var saved workspace.Template
	err := s.edit(ctx, store, ws,
		func(workspace.Settings) (string, error) {
			return fmt.Sprintf("テンプレート「%s」を作成", oneLine(workspace.NormalizeName(in.Name))), nil
		},
		func(_ port.Tx, st workspace.Settings) ([]change.Draft, error) {
			t, err := prepare(st, in)
			if err != nil {
				return nil, err
			}
			saved = t
			return workspace.TemplateDrafts(workspace.Template{}, t), nil
		})
	if err != nil {
		return workspace.Template{}, err
	}
	return saved, nil
}

// UpdateTemplate は、テンプレートを直す(B3-9)。種類を変えたら、前の種類の既定から外す(B3-12)。
func (s Service) UpdateTemplate(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, in workspace.Template) error {
	in.ID = id
	in = s.withIDs(in)
	return s.edit(ctx, store, ws,
		func(st workspace.Settings) (string, error) {
			if _, ok := st.Template(id); !ok {
				return "", port.ErrNotFound
			}
			return fmt.Sprintf("テンプレート「%s」を編集", oneLine(workspace.NormalizeName(in.Name))), nil
		},
		func(_ port.Tx, st workspace.Settings) ([]change.Draft, error) {
			old, ok := st.Template(id)
			if !ok {
				return nil, port.ErrNotFound
			}
			t, err := prepare(st, merged(old, in))
			if err != nil {
				return nil, err
			}
			ds := workspace.TemplateDrafts(old, t)
			if old.Kind != t.Kind {
				ds = append(ds, workspace.DefaultClearDrafts(st, id, t.Kind)...)
			}
			return ds, nil
		})
}

// DeleteTemplate は、テンプレートを消す。種類ごとの既定からも外す。作ったアイテムは変えない。
func (s Service) DeleteTemplate(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID) error {
	return s.edit(ctx, store, ws,
		func(st workspace.Settings) (string, error) {
			t, ok := st.Template(id)
			if !ok {
				return "", port.ErrNotFound
			}
			return fmt.Sprintf("テンプレート「%s」を削除", oneLine(t.Name)), nil
		},
		func(_ port.Tx, st workspace.Settings) ([]change.Draft, error) {
			if _, ok := st.Template(id); !ok {
				return nil, port.ErrNotFound
			}
			return workspace.TemplateDeleteDrafts(st, id), nil
		})
}

// SetTemplateDefault は、種類 kind の既定のテンプレートを変える(03「種類ごとの既定」)。id が空なら「なし」。
// 候補はその種類のテンプレートだけ。ほかは ErrNotFound(B2-12)。
func (s Service) SetTemplateDefault(ctx context.Context, store port.StoreID, ws workspace.ID, kind string, id change.ID) error {
	if !slices.Contains(workspace.Kinds, kind) {
		return port.ErrNotFound
	}
	return s.edit(ctx, store, ws,
		func(workspace.Settings) (string, error) {
			return fmt.Sprintf("%s の既定のテンプレートを変更", kind), nil
		},
		func(_ port.Tx, st workspace.Settings) ([]change.Draft, error) {
			if t, ok := st.Template(id); id != "" && (!ok || t.Kind != kind) {
				return nil, port.ErrNotFound
			}
			cur := st.TemplateDefaults[kind]
			if cur == id {
				return nil, nil
			}
			return []change.Draft{change.NewSet("template_default/"+kind, orNull(cur), orNull(id))}, nil
		})
}

func orNull(id change.ID) any {
	if id == "" {
		return nil
	}
	return id
}

// prepare は、検査して保存する形にする。ない種類・値の参照は ErrNotFound、名前は断り。
func prepare(st workspace.Settings, in workspace.Template) (workspace.Template, error) {
	if !workspace.ValidTemplate(st, in) {
		return workspace.Template{}, port.ErrNotFound
	}
	t := workspace.NormalizeTemplate(st, in).Ranked()
	if err := workspace.CheckTemplate(st, t); err != nil {
		return workspace.Template{}, err
	}
	return t, nil
}

// merged は、編集の入力の値を、いまの値に重ねる(B3-9)。子は同じ ID のものに重ねる。
func merged(old, in workspace.Template) workspace.Template {
	in.Values = mergeValues(old.Values, in.Values)
	for i, c := range in.Children {
		var prev map[change.ID]json.RawMessage
		if j := slices.IndexFunc(old.Children, func(o workspace.TemplateChild) bool { return o.ID == c.ID }); j >= 0 {
			prev = old.Children[j].Values
		}
		in.Children[i].Values = mergeValues(prev, c.Values)
	}
	return in
}

// mergeValues は、old に in を重ねる。in の null は外す。
func mergeValues(old, in map[change.ID]json.RawMessage) map[change.ID]json.RawMessage {
	out := maps.Clone(old)
	for k, v := range in {
		if string(v) == "null" {
			delete(out, k)
			continue
		}
		if out == nil {
			out = map[change.ID]json.RawMessage{}
		}
		out[k] = v
	}
	return out
}

// withIDs は、ID のない子と Step に新しい ID を振る(入力の配列は書き換えない)。
func (s Service) withIDs(t workspace.Template) workspace.Template {
	t.Children = slices.Clone(t.Children)
	for i := range t.Children {
		if t.Children[i].ID == "" {
			t.Children[i].ID = s.IDs.New()
		}
		t.Children[i].Steps = s.stepIDs(t.Children[i].Steps)
	}
	t.Steps = s.stepIDs(t.Steps)
	return t
}

func (s Service) stepIDs(ss []workspace.TemplateStep) []workspace.TemplateStep {
	ss = slices.Clone(ss)
	for i := range ss {
		if ss[i].ID == "" {
			ss[i].ID = s.IDs.New()
		}
	}
	return ss
}
```

(`ValidTemplate` は整える前の入力に当てる。`mergeValues` で `null` を外したあとなので、値の参照だけを見る。)

- [ ] **Step 4: 通るのを確かめる**

Run: `go test ./internal/app/workspaces/ && golangci-lint run ./internal/app/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/workspaces
git commit -m "feat(app): テンプレートの作成・編集・削除と種類ごとの既定を足す(B3 Task 5)"
```

---

### Task 6: アイテムからテンプレートを作る

**Files:**
- Create: `internal/app/workspaces/template_item.go`、Test: `internal/app/workspaces/template_item_test.go`

**Interfaces:**
- Consumes: Task 5 の `CreateTemplate`、`item.Current`・`item.SortSiblings`、`port.ItemReader.Get`・`List`
- Produces:

```go
// FromItem は、「テンプレートとして保存…」のダイアログの入力(03「アイテムからテンプレートを作る」)。
type FromItem struct {
	Name      string
	Title     string // タイトルの書き出し
	Structure bool   // 子と Step を含める
}

// CreateTemplateFromItem は、アイテム id をもとに、そのワークスペースにテンプレートを作る。
func (s Service) CreateTemplateFromItem(ctx context.Context, store port.StoreID, id item.ID, in FromItem) (workspace.Template, error)
```

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/workspaces/template_item_test.go`(Epic「リリース作業」(担当 taro、優先度 High)の下に Story「QA」(担当 hanako)、その Step「確認」(hanako)・「記録」(なし)・「報告」(jiro)を作る):

```go
package workspaces_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// putChild は、親 parent の下に種類 kind のアイテムを作る(番号 number、並び rank)。
func putChild(t *testing.T, s workspaces.Service, p project.Project, parent item.ID, kind item.Kind, number int, rank, title string, drafts ...change.Draft) item.ID {
	t.Helper()
	it := item.Item{ID: s.IDs.New(), Place: item.Place{Workspace: p.Place.Workspace, Project: p.ID, Parent: &parent, Number: number}, Kind: kind, Rank: rank, Title: title}
	err := s.Stores.Write(t.Context(), store, port.Operation{Actor: change.User("taro"), Summary: "準備"}, func(tx port.Tx) error {
		return tx.WriteItems().Append(it.ID, append([]change.Draft{item.Create(it)}, drafts...)...)
	})
	if err != nil {
		t.Fatal(err)
	}
	return it.ID
}

func TestCreateTemplateFromItem(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	p := project.Project{ID: s.IDs.New(), Place: project.Place{Workspace: w.ID, Number: 1}, Rank: "V", Title: "p", Status: project.Todo}
	epic := putItemIn(t, s, p, 1,
		change.NewSet("assignee", nil, "taro"), change.NewSet("priority", nil, st.Priorities[1].ID),
		change.NewSet("status", nil, st.Statuses[1].ID), change.NewSet("points", nil, 3))
	qa := putChild(t, s, p, epic, item.Story, 2, "V", "QA", change.NewSet("assignee", nil, "hanako"))
	putChild(t, s, p, qa, item.Step, 3, "W", "記録")
	putChild(t, s, p, qa, item.Step, 4, "V", "確認", change.NewSet("assignee", nil, "hanako"))
	putChild(t, s, p, qa, item.Step, 5, "X", "報告", change.NewSet("assignee", nil, "jiro"))

	tpl, err := s.CreateTemplateFromItem(ctx, store, epic, workspaces.FromItem{Name: "リリース作業", Title: "リリース", Structure: true})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := settingsOf(t, s, w.ID).Template(tpl.ID)
	if got.Kind != "Epic" || got.Title != "リリース" || len(got.Children) != 1 {
		t.Fatalf("template = %+v", got)
	}
	// ステータスとポイントは写さない。Epic は担当者を使う種類なら担当者を写す(既定の設定は全種類で使う)。
	if got.Values["status"] != nil || got.Values["points"] != nil || string(got.Values["assignee"]) != `"taro"` {
		t.Errorf("values = %v", got.Values)
	}
	c := got.Children[0]
	var steps [][2]string
	for _, st := range c.Steps {
		steps = append(steps, [2]string{st.Title, st.Assignee})
	}
	want := [][2]string{{"確認", workspace.StepParent}, {"記録", workspace.StepNone}, {"報告", "jiro"}}
	if c.Kind != "Story" || c.Title != "QA" || len(steps) != 3 || steps[0] != want[0] || steps[1] != want[1] || steps[2] != want[2] {
		t.Errorf("child = %+v steps = %v", c, steps)
	}

	// 含めないなら、構成は写さない。名前の重複は断る。
	plain, err := s.CreateTemplateFromItem(ctx, store, epic, workspaces.FromItem{Name: "リリース 2", Title: "リリース", Structure: false})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := settingsOf(t, s, w.ID).Template(plain.ID); len(got.Children) != 0 {
		t.Errorf("children copied: %+v", got.Children)
	}
	_, err = s.CreateTemplateFromItem(ctx, store, epic, workspaces.FromItem{Name: "リリース作業", Title: "x"})
	rejected(t, err, "同じ名前のテンプレートがあります")
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/workspaces/ -run 'FromItem'`
Expected: FAIL(`FromItem` が未定義)

- [ ] **Step 3: 実装する**

`internal/app/workspaces/template_item.go`:

```go
package workspaces

import (
	"context"
	"encoding/json"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// FromItem は、「テンプレートとして保存…」のダイアログの入力(03「アイテムからテンプレートを作る」)。
type FromItem struct {
	Name      string
	Title     string // タイトルの書き出し
	Structure bool   // 子と Step を含める
}

// CreateTemplateFromItem は、アイテム id をもとに、そのワークスペースにテンプレートを作る。
// 種類・値・(Structure なら)子と Step を、並び順のまま写す(B3-16)。保存と検査は CreateTemplate と同じ。
func (s Service) CreateTemplateFromItem(ctx context.Context, store port.StoreID, id item.ID, in FromItem) (workspace.Template, error) {
	var t workspace.Template
	var ws workspace.ID
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := r.Items().Get(id)
		if err != nil {
			return err
		}
		st, err := r.Workspaces().Settings(it.Place.Workspace)
		if err != nil {
			return err
		}
		var family []item.Item
		if in.Structure {
			if family, err = r.Items().List(it.Place.Project); err != nil {
				return err
			}
		}
		ws, t = it.Place.Workspace, templateOf(st, it, family, in)
		return nil
	})
	if err != nil {
		return workspace.Template{}, err
	}
	return s.CreateTemplate(ctx, store, ws, t)
}

func templateOf(st workspace.Settings, it item.Item, family []item.Item, in FromItem) workspace.Template {
	t := workspace.Template{Name: in.Name, Kind: string(it.Kind), Title: in.Title, Values: valuesOf(st, it)}
	for _, c := range childrenOf(family, it.ID) {
		switch {
		case it.Kind == item.Epic:
			t.Children = append(t.Children, workspace.TemplateChild{
				Kind: string(c.Kind), Title: c.Title, Values: valuesOf(st, c), Steps: stepsOf(family, c),
			})
		case c.Kind == item.Step:
			t.Steps = append(t.Steps, stepOf(it, c))
		}
	}
	return t
}

// valuesOf は、テンプレートに持てるプロパティの、アイテムの値(B3-5・B3-6)。使わない・空の値は
// NormalizeTemplate が捨てる。
func valuesOf(st workspace.Settings, it item.Item) map[change.ID]json.RawMessage {
	out := map[change.ID]json.RawMessage{}
	for _, p := range st.Properties {
		if workspace.HoldsTemplateValue(p) {
			out[p.ID] = item.Current(it, p)
		}
	}
	return out
}

// childrenOf は、parent の直下のアイテムを並び順で。
func childrenOf(family []item.Item, parent item.ID) []item.Item {
	var out []item.Item
	for _, x := range family {
		if x.Place.Parent != nil && *x.Place.Parent == parent {
			out = append(out, x)
		}
	}
	item.SortSiblings(out)
	return out
}

func stepsOf(family []item.Item, parent item.Item) []workspace.TemplateStep {
	var out []workspace.TemplateStep
	for _, c := range childrenOf(family, parent.ID) {
		out = append(out, stepOf(parent, c))
	}
	return out
}

// stepOf は、Step の担当者を、すぐ上のアイテムと比べて決める(03「アイテムからテンプレートを作る」)。
func stepOf(parent, step item.Item) workspace.TemplateStep {
	a := step.Assignee
	switch {
	case a == parent.Assignee:
		a = workspace.StepParent
	case a == "":
		a = workspace.StepNone
	}
	return workspace.TemplateStep{Title: step.Title, Assignee: a}
}
```

`valuesOf` の値は `null` を含みうる。`CreateTemplate` の `mergeValues(nil, …)` が `null` を外し、`NormalizeTemplate` が空の配列と使わないプロパティを捨てる。チェック型は、値がなくても `item.Current` が `false` を返すので「オフ」として写る(アイテムの見た目と同じ)。

- [ ] **Step 4: 通るのを確かめる**

Run: `go test ./internal/app/workspaces/ && golangci-lint run ./internal/app/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/workspaces
git commit -m "feat(app): アイテムからテンプレートを作る(B3 Task 6)"
```

---

### Task 7: 値・プロパティの削除で、遷移の制約とテンプレートを追従させる

**Files:**
- Create: `internal/domain/workspace/template_follow.go`、Test: `internal/domain/workspace/template_follow_test.go`
- Modify: `internal/app/workspaces/value_delete.go`、`internal/app/workspaces/property_delete.go`
- Test: `internal/app/workspaces/follow_test.go`

**Interfaces:**
- Consumes: Task 1 の `WorkflowDeleteDrafts`、Task 2 の型、B2 の `List`・`Settings.Single`
- Produces:

```go
// template_follow.go
// TemplateValueDrafts は、一覧 l の値 id の削除で、テンプレート(もとと子)の値を置き換える・外す行。
// 単一の値は repl に(空なら外す)、複数の値は id を外す(空になったら外す)。ステータスの一覧は行なし。
func TemplateValueDrafts(s Settings, l List, id, repl change.ID) []change.Draft
// TemplatePropertyDrafts は、追加プロパティ p の削除で、テンプレートの値を外す行。
func TemplatePropertyDrafts(s Settings, p change.ID) []change.Draft
```

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/workspace/template_follow_test.go`:

```go
package workspace_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestTemplateValueDrafts(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	high, low := s.Priorities[1].ID, s.Priorities[3].ID
	web, api := s.Systems[0].ID, s.Systems[1].ID
	s.Templates = []workspace.Template{{
		ID: tplA, Name: "A", Kind: "Epic",
		Values: map[change.ID]json.RawMessage{"priority": raw(high), "systems": raw([]change.ID{web, api})},
		Children: []workspace.TemplateChild{{ID: child, Kind: "Story", Title: "QA",
			Values: map[change.ID]json.RawMessage{"systems": raw([]change.ID{web})}}},
	}}
	got := workspace.TemplateValueDrafts(s, workspace.List{Kind: workspace.ListPriorities}, high, low)
	want := []change.Draft{change.NewSet(tf(tplA, "values", "priority"), raw(high), low)}
	if !sameDrafts(got, want) {
		t.Errorf("priority: %v", got)
	}
	got = workspace.TemplateValueDrafts(s, workspace.List{Kind: workspace.ListSystems}, web, "")
	want = []change.Draft{
		change.NewSet(tf(tplA, "values", "systems"), raw([]change.ID{web, api}), []change.ID{api}),
		change.NewSet(tf(tplA, "children", string(child), "values", "systems"), raw([]change.ID{web}), nil),
	}
	if !sameDrafts(got, want) {
		t.Errorf("systems: %v", got)
	}
	if got := workspace.TemplateValueDrafts(s, workspace.List{Kind: workspace.ListStatuses}, s.Statuses[0].ID, s.Statuses[1].ID); got != nil {
		t.Errorf("statuses: %v", got)
	}
}

func TestTemplatePropertyDrafts(t *testing.T) {
	s := workspace.Settings{Templates: []workspace.Template{
		{ID: tplA, Name: "A", Kind: "Task", Values: map[change.ID]json.RawMessage{envProp: raw("本番")}},
		{ID: tplB, Name: "B", Kind: "Task"},
	}}
	want := []change.Draft{change.NewSet(tf(tplA, "values", string(envProp)), raw("本番"), nil)}
	if got := workspace.TemplatePropertyDrafts(s, envProp); !sameDrafts(got, want) {
		t.Errorf("drafts = %v", got)
	}
}
```

`internal/app/workspaces/follow_test.go`(サービスを通して、同じ保存に入ることを確かめる):

```go
package workspaces_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ステータス Review を消すと、遷移の制約から外れ、置き換えのアイテムは制約を通さずに変わる(B2-10)。
func TestDeleteStatusFollowsWorkflow(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	statuses := workspace.ParseList("statuses")
	review, _ := s.AddValue(ctx, store, w.ID, statuses, "Review")
	st := settingsOf(t, s, w.ID)
	todo, done := st.Statuses[0].ID, st.Statuses[2].ID
	for _, p := range [][2]change.ID{{todo, review.ID}, {review.ID, done}} {
		if err := s.SetTransition(ctx, store, w.ID, p[0], p[1], true); err != nil {
			t.Fatal(err)
		}
	}
	it := putItem(t, s, w.ID, change.NewSet("status", nil, review.ID))
	if err := s.DeleteValue(ctx, store, w.ID, statuses, review.ID, done); err != nil {
		t.Fatal(err)
	}
	after := settingsOf(t, s, w.ID)
	if _, r := after.Allowed(todo); r || len(after.Workflow[review.ID]) != 0 {
		t.Errorf("workflow = %v", after.Workflow)
	}
	if got := itemOf(t, s, it); got.Status != done {
		t.Errorf("status = %s", got.Status)
	}
}

// 優先度・追加プロパティを消すと、テンプレートの値が置き換わる・外れる。
func TestDeleteValueAndPropertyFollowTemplates(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	env, _ := s.AddProperty(ctx, store, w.ID, workspaces.NewProperty{Name: "環境", Key: "env", Type: workspace.TypeText})
	high := st.Priorities[1].ID
	tpl, err := s.CreateTemplate(ctx, store, w.ID, workspace.Template{Name: "調査", Kind: "Task", Values: map[change.ID]json.RawMessage{
		"priority": jsonOf(high), env.ID: jsonOf("本番"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteValue(ctx, store, w.ID, workspace.ParseList("priorities"), high, st.Priorities[3].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProperty(ctx, store, w.ID, env.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := settingsOf(t, s, w.ID).Template(tpl.ID)
	if string(got.Values["priority"]) != string(jsonOf(st.Priorities[3].ID)) || got.Values[env.ID] != nil {
		t.Errorf("values = %v", got.Values)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/workspace/ ./internal/app/workspaces/ -run 'Follow|TemplateValueDrafts|TemplatePropertyDrafts'`
Expected: FAIL(`TemplateValueDrafts` が未定義)

- [ ] **Step 3: 実装する**

`internal/domain/workspace/template_follow.go`:

```go
package workspace

import (
	"encoding/json"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// listProperty は、一覧 l の値を持つプロパティの ID。テンプレートはステータスを持たないので false。
func listProperty(l List) (change.ID, bool) {
	switch l.Kind {
	case ListPriorities:
		return "priority", true
	case ListSystems:
		return "systems", true
	case ListChoices:
		return l.Property, true
	}
	return "", false
}

// TemplateValueDrafts は、一覧 l の値 id の削除で、テンプレートの値を置き換える・外す行(B3-15)。
func TemplateValueDrafts(s Settings, l List, id, repl change.ID) []change.Draft {
	prop, ok := listProperty(l)
	if !ok {
		return nil
	}
	single := s.Single(l)
	var ds []change.Draft
	for _, t := range s.Templates {
		ds = append(ds, followValue(TemplateField(t.ID, "values", string(prop)), t.Values[prop], id, repl, single)...)
		for _, c := range t.Children {
			f := TemplateField(t.ID, "children", string(c.ID), "values", string(prop))
			ds = append(ds, followValue(f, c.Values[prop], id, repl, single)...)
		}
	}
	return ds
}

func followValue(f string, cur json.RawMessage, id, repl change.ID, single bool) []change.Draft {
	if cur == nil {
		return nil
	}
	if single {
		var v change.ID
		if json.Unmarshal(cur, &v) != nil || v != id {
			return nil
		}
		return []change.Draft{change.NewSet(f, cur, orNil(repl))}
	}
	var vs []change.ID
	if json.Unmarshal(cur, &vs) != nil || !slices.Contains(vs, id) {
		return nil
	}
	rest := slices.DeleteFunc(slices.Clone(vs), func(v change.ID) bool { return v == id })
	if len(rest) == 0 {
		return []change.Draft{change.NewSet(f, cur, nil)}
	}
	return []change.Draft{change.NewSet(f, cur, rest)}
}

func orNil(id change.ID) any {
	if id == "" {
		return nil
	}
	return id
}

// TemplatePropertyDrafts は、追加プロパティ p の削除で、テンプレート(もとと子)の値を外す行(B3-15)。
func TemplatePropertyDrafts(s Settings, p change.ID) []change.Draft {
	var ds []change.Draft
	for _, t := range s.Templates {
		if v, ok := t.Values[p]; ok {
			ds = append(ds, change.NewSet(TemplateField(t.ID, "values", string(p)), v, nil))
		}
		for _, c := range t.Children {
			if v, ok := c.Values[p]; ok {
				ds = append(ds, change.NewSet(TemplateField(t.ID, "children", string(c.ID), "values", string(p)), v, nil))
			}
		}
	}
	return ds
}
```

`internal/app/workspaces/value_delete.go` の `DeleteValue` の行を返すところを次にする(置き換えのアイテムの行はそのまま。遷移の制約は検査しない。B2-10):

```go
			ds := []change.Draft{change.NewDelete(l.Ref(id))}
			if l.Kind == workspace.ListStatuses {
				ds = append(ds, workspace.WorkflowDeleteDrafts(st, id)...)
			}
			return append(ds, workspace.TemplateValueDrafts(st, l, id, repl)...), nil
```

`internal/app/workspaces/property_delete.go` の `DeleteProperty` の行を返すところを次にする:

```go
			ds := []change.Draft{change.NewDelete("properties/" + string(id))}
			return append(ds, workspace.TemplatePropertyDrafts(st, id)...), nil
```

- [ ] **Step 4: 通るのを確かめる**

Run: `go test ./internal/domain/workspace/ ./internal/app/workspaces/ && golangci-lint run ./internal/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/workspace internal/app/workspaces
git commit -m "feat(app): 値とプロパティの削除で、遷移の制約とテンプレートを追従させる(B3 Task 7)"
```

---

### Task 8: workspace.yaml に遷移の制約・テンプレート・既定を書く

**Files:**
- Create: `internal/infra/gitstore/yaml_templates.go`
- Modify: `internal/infra/gitstore/yaml_settings.go`(`workspaceToYAML` にキーを 3 つ)
- Test: `internal/infra/gitstore/yaml_settings_test.go`(`TestWorkspaceYAMLHasWorkflowAndTemplates` を足す)

**Interfaces:**
- Consumes: Task 1・2 の `Settings.Workflow`・`Allowed`・`Templates`・`TemplateDefaults`
- Produces: `workspace.yaml` の末尾(`category_defaults` のあと)に、この順で `workflow`・`templates`・`template_defaults`。

```yaml
workflow:
  01S...TODO: [01S...DOING, 01S...PENDING] # Todo → InProgress・Pending
templates:
  - id: 01T...
    name: リリース作業
    kind: Epic
    title: リリース
    description: "## 手順"
    values: {priority: 01P...HIGH}
    children:
      - {id: 01C..., kind: Story, title: QA, description: "", rank: V, values: {}, steps: [{id: 01S..., title: 確認, assignee: '@parent', rank: V}]}
    steps: []
template_defaults: {Epic: 01T...} # (注釈なし)
```

- [ ] **Step 1: 失敗するテストを書く**

`yaml_settings_test.go` の末尾に足す(`ySeed` のあとに、遷移の制約とテンプレートの行を書き足す):

```go
func TestWorkspaceYAMLHasWorkflowAndTemplates(t *testing.T) {
	s, id, dir, st := ySeed(t)
	todo, doing := st.Statuses[0].ID, st.Statuses[1].ID
	tpl := workspace.Template{ID: "01YTPL00000000000000000001", Name: "リリース作業", Kind: "Epic", Title: "リリース",
		Children: []workspace.TemplateChild{{ID: "01YCHD00000000000000000001", Kind: "Story", Title: "QA",
			Steps: []workspace.TemplateStep{{ID: "01YSTP00000000000000000001", Title: "確認", Assignee: workspace.StepParent}}}},
	}.Ranked()
	ds := append([]change.Draft{change.NewAdd(workspace.WorkflowField(todo), doing)}, workspace.TemplateDrafts(workspace.Template{}, tpl)...)
	ds = append(ds, change.NewSet("template_default/Epic", nil, tpl.ID))
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro"), Summary: "準備"}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(yWS, ds...)
	})
	if err != nil {
		t.Fatal(err)
	}
	got := readFileString(t, dir, "workspaces/"+string(yWS)+"/workspace.yaml")
	mustContain(t, "workspace.yaml", got,
		"workflow:\n", "  "+string(todo)+": ["+string(doing)+"] # Todo → InProgress\n",
		"templates:\n", "name: リリース作業\n", "kind: Epic\n", "children:\n", "title: QA",
		"assignee: '@parent'", "template_defaults: {Epic: 01YTPL00000000000000000001}",
	)
}
```

ほかの既存のテスト(`TestRebuildYAMLMatchesWritePath`)が、同期のあとの作り直しと書くときで同じ中身になることを確かめているので、そのまま通ることも確かめる。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/infra/gitstore/ -run 'WorkspaceYAML'`
Expected: FAIL(`workflow:` がない)

- [ ] **Step 3: 実装する**

`yaml_settings.go` の `workspaceToYAML` の `mapping` の末尾に足す:

```go
		key("workflow"), workflowNode(s),
		key("templates"), templatesNode(s.Templates),
		key("template_defaults"), templateDefaultsNode(s),
```

`yaml_templates.go`:

```go
package gitstore

import (
	"encoding/json"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// workflowNode は、遷移の制約(ステータスの並び)。制限のない行は書かない。行末に名前の注釈。
func workflowNode(s workspace.Settings) *yaml.Node {
	var kv []*yaml.Node
	for _, v := range s.Statuses {
		to, restricted := s.Allowed(v.ID)
		if !restricted {
			continue
		}
		ids, names := make([]string, 0, len(to)), make([]string, 0, len(to))
		for _, id := range to {
			ids = append(ids, string(id))
			names = append(names, valueName(s.Statuses, id))
		}
		seq := flowSeq(ids...)
		seq.LineComment = comment(v.Name + " → " + strings.Join(names, "・")) // initial_status と同じく値の節に注釈
		kv = append(kv, key(string(v.ID)), seq)
	}
	return mapping(len(kv) == 0, kv...)
}

func templatesNode(ts []workspace.Template) *yaml.Node {
	items := make([]*yaml.Node, 0, len(ts))
	for _, t := range ts {
		children := make([]*yaml.Node, 0, len(t.Children))
		for _, c := range t.Children {
			children = append(children, mapping(true,
				key("id"), strNode(string(c.ID)), key("kind"), strNode(c.Kind), key("title"), strNode(c.Title),
				key("description"), strNode(c.Description), key("rank"), strNode(c.Rank),
				key("values"), valuesMap(c.Values), key("steps"), stepsNode(c.Steps)))
		}
		items = append(items, mapping(false,
			key("id"), strNode(string(t.ID)), key("name"), strNode(t.Name), key("kind"), strNode(t.Kind),
			key("title"), strNode(t.Title), key("description"), strNode(t.Description),
			key("values"), valuesMap(t.Values), key("children"), seqOf(children), key("steps"), stepsNode(t.Steps)))
	}
	return seqOf(items)
}

func stepsNode(ss []workspace.TemplateStep) *yaml.Node {
	items := make([]*yaml.Node, 0, len(ss))
	for _, s := range ss {
		items = append(items, mapping(true,
			key("id"), strNode(string(s.ID)), key("title"), strNode(s.Title),
			key("assignee"), strNode(s.Assignee), key("rank"), strNode(s.Rank)))
	}
	n := seqOf(items)
	n.Style = yaml.FlowStyle
	return n
}

// valuesMap は、テンプレートの値(キーの順、1 行)。値は JSON のまま YAML の値にする。
func valuesMap(vs map[change.ID]json.RawMessage) *yaml.Node {
	keys := make([]change.ID, 0, len(vs))
	for k := range vs {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	kv := make([]*yaml.Node, 0, 2*len(keys))
	for _, k := range keys {
		var v yaml.Node
		if yaml.Unmarshal(vs[k], &v) != nil || len(v.Content) == 0 {
			continue
		}
		kv = append(kv, key(string(k)), v.Content[0])
	}
	return mapping(true, kv...)
}

// templateDefaultsNode は、種類ごとの既定(種類の並び)。
func templateDefaultsNode(s workspace.Settings) *yaml.Node {
	var kv []*yaml.Node
	for _, k := range workspace.Kinds {
		if id, ok := s.TemplateDefaults[k]; ok {
			kv = append(kv, key(k), strNode(string(id)))
		}
	}
	return mapping(true, kv...)
}
```

(JSON は YAML として読めるので、`yaml.Unmarshal` で値の節にする。`valuesMap` の値が 1 行の flow になるよう、取り出した節の `Style` に `yaml.FlowStyle` を足す。)

- [ ] **Step 4: 通るのを確かめる**

Run: `go test ./internal/infra/gitstore/ && golangci-lint run ./internal/infra/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/infra/gitstore
git commit -m "feat(gitstore): workspace.yaml に遷移の制約・テンプレート・種類ごとの既定を書く(B3 Task 8)"
```

---

### Task 9: 遷移の制約とテンプレートの API

**Files:**
- Create: `internal/adapter/httpapi/workspaces/templates.go`、Test: `internal/adapter/httpapi/workspaces/templates_test.go`
- Modify: `internal/adapter/httpapi/workspaces/dto.go`(設定に 3 つ、テンプレートの型)、`internal/adapter/httpapi/workspaces/workspaces.go`(`Register` から `registerTemplates` を呼ぶ)
- 生成物: `api/openapi.yaml`、`web/packages/api/src/generated.ts`

**Interfaces:**
- Consumes: Task 4〜6 の操作
- Produces(HTTP。`{ws}` は `/api/stores/{store}/workspaces/{workspace}`):

| OperationID | Method・Path | 本文 | 返す |
|---|---|---|---|
| `set-workspace-transition` | `PUT {ws}/workflow/{from}/{to}` | `SetWorkspaceTransition{allowed bool}` | 204 |
| `create-workspace-template` | `POST {ws}/templates` | `WorkspaceTemplateInput` | 201 `WorkspaceTemplate` |
| `update-workspace-template` | `PUT {ws}/templates/{template}` | `WorkspaceTemplateInput` | 204 |
| `delete-workspace-template` | `DELETE {ws}/templates/{template}` | — | 204 |
| `set-workspace-template-default` | `PUT {ws}/template-defaults/{kind}` | `SetWorkspaceTemplateDefault{template string}`(空は「なし」) | 204 |
| `save-item-as-template` | `POST /api/stores/{store}/items/{item}/save-as-template` | `SaveItemAsTemplate{name, title string; structure bool}` | 201 `WorkspaceTemplate` |

DTO(`dto.go`):

```go
// WorkspaceTemplateStep は、テンプレートの Step。
type WorkspaceTemplateStep struct {
	ID       string `json:"id" doc:"新しく足すときは空"`
	Title    string `json:"title"`
	Assignee string `json:"assignee" doc:"@parent(親と同じ)・@none(未割り当て)・メンバーのログイン名"`
}

// WorkspaceTemplateChild は、テンプレートの子(Epic のときだけ)。
type WorkspaceTemplateChild struct {
	ID          string                  `json:"id" doc:"新しく足すときは空"`
	Kind        string                  `json:"kind" enum:"Story,Task,Bug"`
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	Values      map[string]any          `json:"values" doc:"プロパティの ID → 値。null は指定しない"`
	Steps       []WorkspaceTemplateStep `json:"steps" nullable:"false"`
}

// WorkspaceTemplateInput は、テンプレートの作成・編集の本文。編集で送らなかった値は残る。
type WorkspaceTemplateInput struct {
	Name        string                   `json:"name"`
	Kind        string                   `json:"kind" enum:"Epic,Story,Task,Bug,Step"`
	Title       string                   `json:"title"`
	Description string                   `json:"description"`
	Values      map[string]any           `json:"values" doc:"プロパティの ID → 値。null は指定しない"`
	Children    []WorkspaceTemplateChild `json:"children" nullable:"false"`
	Steps       []WorkspaceTemplateStep  `json:"steps" nullable:"false"`
}

// WorkspaceTemplate は、テンプレート。
type WorkspaceTemplate struct {
	ID string `json:"id"`
	WorkspaceTemplateInput
}
```

`WorkspaceSettings` に足す:

```go
	Workflow         map[string][]string `json:"workflow" doc:"From のステータス ID → 許可する To(ステータスの並び)。制限のない From は入れない"`
	Templates        []WorkspaceTemplate `json:"templates" nullable:"false"`
	TemplateDefaults map[string]string   `json:"templateDefaults" doc:"種類 → 既定のテンプレート ID。なしの種類は入れない"`
```

- [ ] **Step 1: 失敗するテストを書く**

`templates_test.go`(既存の `newAPI`・`create`・`base`・`idOf` を使う):

```go
package workspaces_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type templatesBody struct {
	Settings struct {
		Statuses []struct {
			ID string `json:"id"`
		} `json:"statuses"`
		Workflow         map[string][]string `json:"workflow"`
		Templates        []json.RawMessage   `json:"templates"`
		TemplateDefaults map[string]string   `json:"templateDefaults"`
	} `json:"settings"`
}

func TestWorkflowAndTemplatesAPI(t *testing.T) {
	api := newAPI(t)
	ws := create(t, api, "Web", "HH")
	var d templatesBody
	_ = json.Unmarshal(api.Get(base+"/"+ws).Body.Bytes(), &d)
	todo, doing := d.Settings.Statuses[0].ID, d.Settings.Statuses[1].ID
	if res := api.Put(base+"/"+ws+"/workflow/"+todo+"/"+doing, map[string]any{"allowed": true}); res.Code != http.StatusNoContent {
		t.Fatalf("transition: %d %s", res.Code, res.Body)
	}
	if res := api.Put(base+"/"+ws+"/workflow/"+todo+"/"+todo, map[string]any{"allowed": true}); res.Code != http.StatusNotFound {
		t.Errorf("self: %d", res.Code)
	}
	in := map[string]any{"name": "調査", "kind": "Task", "title": "調査: ", "description": "", "values": map[string]any{},
		"children": []any{}, "steps": []any{map[string]any{"id": "", "title": "再現", "assignee": "@none"}}}
	res := api.Post(base+"/"+ws+"/templates", in)
	if res.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", res.Code, res.Body)
	}
	tpl := idOf(t, res.Body.Bytes())
	if res := api.Post(base+"/"+ws+"/templates", in); res.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(res.Body.String(), "同じ名前のテンプレートがあります") {
		t.Errorf("duplicate: %d %s", res.Code, res.Body)
	}
	if res := api.Put(base+"/"+ws+"/template-defaults/Task", map[string]any{"template": tpl}); res.Code != http.StatusNoContent {
		t.Fatalf("default: %d %s", res.Code, res.Body)
	}
	_ = json.Unmarshal(api.Get(base+"/"+ws).Body.Bytes(), &d)
	if len(d.Settings.Workflow[todo]) != 1 || len(d.Settings.Templates) != 1 || d.Settings.TemplateDefaults["Task"] != tpl {
		t.Errorf("settings = %+v", d.Settings)
	}
	in["name"] = "調査 2"
	if res := api.Put(base+"/"+ws+"/templates/"+tpl, in); res.Code != http.StatusNoContent {
		t.Errorf("update: %d %s", res.Code, res.Body)
	}
	if res := api.Delete(base + "/" + ws + "/templates/" + tpl); res.Code != http.StatusNoContent {
		t.Errorf("delete: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items/nope/save-as-template",
		map[string]any{"name": "x", "title": "x", "structure": true}); res.Code != http.StatusNotFound {
		t.Errorf("save-as-template unknown item: %d %s", res.Code, res.Body)
	}
}
```

(`S1` は `base` の Store 名。アイテムから作る正しい流れは Task 6 のテストと e2e で確かめる。)

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/workspaces/ -run 'WorkflowAndTemplates'`
Expected: FAIL(404 か 405)

- [ ] **Step 3: 実装する**

`dto.go`: 上の型を足し、`toSettings` で `Workflow`(ステータスの並びで `s.Allowed(v.ID)` が制限ありのものだけ)、`Templates`(`toTemplate`)、`TemplateDefaults` を入れる。値の変換は 2 つの関数で行う:

```go
// fromValues は、本文の値を JSON の値にする(nil は null。「指定しない」に戻す)。
func fromValues(vs map[string]any) map[change.ID]json.RawMessage {
	out := make(map[change.ID]json.RawMessage, len(vs))
	for k, v := range vs {
		b, _ := json.Marshal(v)
		out[change.ID(k)] = b
	}
	return out
}

// toValues は、JSON の値を本文の値にする。
func toValuesMap(vs map[change.ID]json.RawMessage) map[string]any {
	out := make(map[string]any, len(vs))
	for k, raw := range vs {
		var v any
		_ = json.Unmarshal(raw, &v)
		out[string(k)] = v
	}
	return out
}
```

あわせて `toTemplate(t workspace.Template) WorkspaceTemplate` と `fromTemplate(in WorkspaceTemplateInput) workspace.Template`(子と Step の配列を変換。`nil` は空の配列にして返す)を置く。`dto.go` が 300 行を超えるなら、テンプレートの型と変換を `templates_dto.go` に分ける。

`templates.go`:

```go
package workspaces

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	appworkspaces "github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// SetWorkspaceTransition は、遷移の制約の表のセルの本文。
type SetWorkspaceTransition struct {
	Allowed bool `json:"allowed"`
}

// SetWorkspaceTemplateDefault は、種類ごとの既定の本文。
type SetWorkspaceTemplateDefault struct {
	Template string `json:"template" doc:"テンプレートの ID。空なら「なし」"`
}

// SaveItemAsTemplate は、「テンプレートとして保存…」の本文。
type SaveItemAsTemplate struct {
	Name      string `json:"name"`
	Title     string `json:"title"`
	Structure bool   `json:"structure" doc:"子と Step を含める"`
}

type transitionInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	From      string `path:"from"`
	To        string `path:"to"`
	Body      SetWorkspaceTransition
}

type createTemplateInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Body      WorkspaceTemplateInput
}

type updateTemplateInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Template  string `path:"template"`
	Body      WorkspaceTemplateInput
}

type templatePath struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Template  string `path:"template"`
}

type templateDefaultInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Kind      string `path:"kind" enum:"Epic,Story,Task,Bug,Step"`
	Body      SetWorkspaceTemplateDefault
}

type saveAsTemplateInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  SaveItemAsTemplate
}

type templateOutput struct{ Body WorkspaceTemplate }

func registerTemplates(api huma.API, svc appworkspaces.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "set-workspace-transition", Method: http.MethodPut, Path: wsPath + "/workflow/{from}/{to}",
		Summary: "遷移の制約のセルを変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *transitionInput) (*struct{}, error) {
		return nil, apierr.From(svc.SetTransition(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace),
			change.ID(in.From), change.ID(in.To), in.Body.Allowed))
	})
	huma.Register(api, huma.Operation{
		OperationID: "create-workspace-template", Method: http.MethodPost, Path: wsPath + "/templates",
		Summary: "テンプレートを作成", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createTemplateInput) (*templateOutput, error) {
		t, err := svc.CreateTemplate(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), fromTemplate(in.Body))
		if err != nil {
			return nil, apierr.From(err)
		}
		return &templateOutput{Body: toTemplate(t)}, nil
	})
	registerTemplateEdits(api, svc)
}

func registerTemplateEdits(api huma.API, svc appworkspaces.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "update-workspace-template", Method: http.MethodPut, Path: wsPath + "/templates/{template}",
		Summary: "テンプレートを編集", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *updateTemplateInput) (*struct{}, error) {
		return nil, apierr.From(svc.UpdateTemplate(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace),
			change.ID(in.Template), fromTemplate(in.Body)))
	})
	huma.Register(api, huma.Operation{
		OperationID: "delete-workspace-template", Method: http.MethodDelete, Path: wsPath + "/templates/{template}",
		Summary: "テンプレートを削除", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *templatePath) (*struct{}, error) {
		return nil, apierr.From(svc.DeleteTemplate(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), change.ID(in.Template)))
	})
	huma.Register(api, huma.Operation{
		OperationID: "set-workspace-template-default", Method: http.MethodPut, Path: wsPath + "/template-defaults/{kind}",
		Summary: "種類ごとの既定のテンプレートを変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *templateDefaultInput) (*struct{}, error) {
		return nil, apierr.From(svc.SetTemplateDefault(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace),
			in.Kind, change.ID(in.Body.Template)))
	})
	huma.Register(api, huma.Operation{
		OperationID: "save-item-as-template", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/save-as-template",
		Summary: "アイテムをテンプレートとして保存", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *saveAsTemplateInput) (*templateOutput, error) {
		t, err := svc.CreateTemplateFromItem(ctx, port.StoreID(in.Store), item.ID(in.Item), appworkspaces.FromItem{
			Name: in.Body.Name, Title: in.Body.Title, Structure: in.Body.Structure,
		})
		if err != nil {
			return nil, apierr.From(err)
		}
		return &templateOutput{Body: toTemplate(t)}, nil
	})
}
```

`workspaces.go` の `Register` の末尾に `registerTemplates(api, svc)` を足す。そのあと `make generate` と `cd web && pnpm gen` を流す。

- [ ] **Step 4: 通るのを確かめる**

Run: `go test ./internal/... && golangci-lint run ./... && make generate && git diff --stat api/ && cd web && pnpm gen && pnpm check`
Expected: PASS。`api/openapi.yaml` と `generated.ts` に 6 つの操作が足される。

- [ ] **Step 5: コミット**

```bash
git add internal/adapter api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat(api): 遷移の制約とテンプレートの API を足す(B3 Task 9)"
```

---

## 画面のタスクに共通すること

B2 の「画面のタスクに共通すること」をすべて引き継ぐ。主なもの:

- ロジック(行の作り、文言、無効の理由、送る形)は `core` に置き、Vitest で確かめる。Svelte は `core` の値を出して操作を呼ぶだけにする。
- 操作は `workspaceSettings.run(op, ok?, onRefused?)` に渡す。断りは、ダイアログの中ならダイアログに赤字(`onRefused`)、ほかはエラーのトースト。
- 成功のトーストは「テンプレートを作成しました」「テンプレートを削除しました」だけ(B3-13)。
- 閲覧のみの Store では、Store に書く入力欄とボタンをすべて無効にし、`readOnlyNote` を `title` に添える。テンプレートの編集ダイアログは開けるが、中の入力欄・追加・並べ替え・保存は無効で、「閉じる」だけ使える(03「範囲と画面の構成」)。
- 部品は `@hedgehug/ui` の `Button`・`IconButton`・`Input`・`Select`・`Checkbox`・`Dialog`・`ConfirmDialog`・`Menu`・`KindIcon`・`StatusDot` を使う。ui に部品を足さない。
- Svelte 200 行、TS 300 行を超えたら分ける。
- 各タスクの最後に `cd web && pnpm check && pnpm test` を流す。

### Task 10: 型・口・設定の状態(メンバー)

**Files:**
- Modify: `web/packages/core/src/model/workspace.ts`、`web/packages/core/src/port/workspaces.ts`、`web/packages/core/src/index.ts`
- Modify: `web/packages/core/src/features/workspaces/settings.ts`(`members`)、`settings.test.ts`、`fixtures.test-util.ts`(既定の設定と偽の口)
- Modify: `web/packages/api/src/workspaces.ts`(口の実装)、Test: `web/packages/api/src/ports.test.ts`(既存の口のテストに足す)
- Modify: `web/app/src/wire.ts`(`listMembers` を渡す)
- `WorkspaceSettings` を作っているほかのテスト用の値(`features/items`・`features/projects` の fixtures など)に、新しい 3 つの欄を足す(`pnpm check` が指す所だけ)

**Interfaces:**
- Consumes: Task 9 の API(`generated.ts` の `setWorkspaceTransition`・`createWorkspaceTemplate`・`updateWorkspaceTemplate`・`deleteWorkspaceTemplate`・`setWorkspaceTemplateDefault`・`saveItemAsTemplate`)
- Produces:

```ts
// model/workspace.ts
export const stepParent = "@parent"; // Step の担当者「親と同じ」
export const stepNone = "@none";     // Step の担当者「未割り当て」
export type ChildKind = "Story" | "Task" | "Bug";
export interface TemplateStep { id: string; title: string; assignee: string }
export interface TemplateChild {
  id: string; kind: ChildKind; title: string; description: string;
  values: Record<string, unknown>; steps: TemplateStep[];
}
export interface TemplateInput {
  name: string; kind: string; title: string; description: string;
  values: Record<string, unknown>; children: TemplateChild[]; steps: TemplateStep[];
}
export interface WorkspaceTemplate extends TemplateInput { id: string }
export interface SaveAsTemplateInput { name: string; title: string; structure: boolean }
// WorkspaceSettings に足す
workflow: Record<string, string[]>;            // From → 許可する To(制限のない From はない)
templates: WorkspaceTemplate[];
templateDefaults: Partial<Record<string, string>>; // 種類 → テンプレート ID

// port/workspaces.ts の WorkspacesPort に足す
setTransition(storeId: string, workspaceId: string, from: string, to: string, allowed: boolean): Promise<void>;
createTemplate(storeId: string, workspaceId: string, input: TemplateInput): Promise<WorkspaceTemplate>;
updateTemplate(storeId: string, workspaceId: string, templateId: string, input: TemplateInput): Promise<void>;
deleteTemplate(storeId: string, workspaceId: string, templateId: string): Promise<void>;
// templateId が空なら「なし」
setTemplateDefault(storeId: string, workspaceId: string, kind: string, templateId: string): Promise<void>;
saveItemAsTemplate(storeId: string, itemId: string, input: SaveAsTemplateInput): Promise<WorkspaceTemplate>;

// settings.ts
SettingsState に members: Member[] を足す(Store のメンバー。テンプレートの担当者の選択肢。B3-19)
createWorkspaceSettings の deps に listMembers(storeId: string): Promise<Member[]> を足す
```

- [ ] **Step 1: 失敗するテストを書く**

`settings.test.ts` に足す:

```ts
it("loads the store's members with the settings", async () => {
  const port = fakeWorkspacesPort();
  const screen = createWorkspaceSettings({
    port, catalog: catalogWith("s1"), toasts: createToasts(),
    listMembers: async () => [{ login: "taro", name: "田中" }],
  });
  await screen.open("s1", "w1");
  expect(get(screen.state)?.members).toEqual([{ login: "taro", name: "田中" }]);
});
```

(`catalogWith` は、既存の `settings.test.ts` が Catalog を作っているやり方に合わせる。)

`ports.test.ts` に、`createTemplate` が `values` の `null` をそのまま送り、返った `WorkspaceTemplate` の `children`・`steps` が `null` で届いても空の配列にすることを足す(既存の口のテストの書き方に合わせ、`fetch` の偽物で本文と返り値を確かめる)。

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test -- settings && pnpm --filter @hedgehug/api test`
Expected: FAIL

- [ ] **Step 3: 実装する**

- 型と口を Interfaces のとおりに足す。`fixtures.test-util.ts` の `defaultDetail` の設定に `workflow: {}`・`templates: []`・`templateDefaults: {}` を足し、`fakeWorkspacesPort` に 6 つの操作を足す(呼ばれた引数を `calls` に控え、`createTemplate`・`saveItemAsTemplate` は `{ id: "t-new", ...input }` を返す。既存の偽の操作と同じ書き方)。
- `settings.ts`: `reload` の `Promise.all` に `deps.listMembers(storeId)` を足して `members` に入れる。
- `api/src/workspaces.ts`: 6 つの操作を、生成された関数で実装する。設定の `toDetail` で `workflow ?? {}`・`templates`(`children`・`steps`・`values` の `null` を空に)・`templateDefaults ?? {}` を入れる。`createTemplate`・`saveItemAsTemplate` の返り値も同じ変換を通す。
- `wire.ts`: `createWorkspaceSettings` に `listMembers: (s) => catalogPort.listMembers(s)` を渡す。

- [ ] **Step 4: 通るのを確かめる**

Run: `cd web && pnpm check && pnpm test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages web/app/src/wire.ts
git commit -m "feat(web): 遷移の制約とテンプレートの型・口と、設定の画面のメンバーを足す(B3 Task 10)"
```

---

### Task 11: ワークフローのタブ

**Files:**
- Create: `web/packages/core/src/features/workspaces/workflow.ts`、`workflow.test.ts`
- Create: `web/app/src/screens/workspace-settings/WorkflowPanel.svelte`
- Modify: `web/app/src/screens/workspace-settings/SettingsTab.svelte`、`web/packages/core/src/index.ts`

**Interfaces:**
- Consumes: Task 10 の `WorkspaceSettings.workflow`・`setTransition`
- Produces:

```ts
export const workflowHelp =
  "行に表示されているステータスから、チェックしたステータスにだけ遷移できるよう制御することができます。";
export interface WorkflowCell { to: string; name: string; self: boolean; checked: boolean }
export interface WorkflowRow {
  from: string; name: string; color: string; category: Category;
  cells: WorkflowCell[];          // ステータスの並び。自分自身は self(「—」を出す)
  note: "どこへでも" | "制限あり";
}
export function workflowRows(settings: WorkspaceSettings): WorkflowRow[];
// transitionRefusal は、Go の CheckTransition と同じ判定と文言(B3-3)。通るなら null。A3b がメニューから呼ぶ。
export function transitionRefusal(settings: WorkspaceSettings, from: string, to: string): string | null;
```

- [ ] **Step 1: 失敗するテストを書く**

`workflow.test.ts`(設定は `defaultDetail(...).settings`。ステータスの ID はその中の並びで取る):

```ts
import { describe, expect, it } from "vitest";
import { defaultDetail } from "./fixtures.test-util";
import { transitionRefusal, workflowRows } from "./workflow";

const settingsWith = (workflow: Record<string, string[]>) => ({
  ...defaultDetail("s1", "w1").settings,
  workflow,
});

describe("workflow rows", () => {
  it("shows どこへでも for a row without checks and — for itself", () => {
    const s = settingsWith({});
    const rows = workflowRows(s);
    expect(rows.map((r) => r.name)).toEqual(["Todo", "InProgress", "Done", "Pending", "Cancel"]);
    expect(rows[0]?.note).toBe("どこへでも");
    expect(rows[0]?.cells.map((c) => c.self)).toEqual([true, false, false, false, false]);
  });

  it("checks the allowed targets and says 制限あり", () => {
    const base = settingsWith({});
    const [todo, doing] = base.statuses.map((v) => v.id);
    const rows = workflowRows(settingsWith({ [todo as string]: [doing as string] }));
    expect(rows[0]?.note).toBe("制限あり");
    expect(rows[0]?.cells.map((c) => c.checked)).toEqual([false, true, false, false, false]);
  });
});

describe("transition refusal", () => {
  it("uses the words of spec 03 and lists the allowed targets in status order", () => {
    const base = settingsWith({});
    const [todo, doing, done, pending] = base.statuses.map((v) => v.id) as string[];
    const s = settingsWith({ [todo as string]: [pending as string, doing as string] });
    expect(transitionRefusal(s, todo as string, done as string)).toBe(
      "ワークフローの制約: Todo から Done には変更できません(InProgress・Pending のみ)",
    );
    expect(transitionRefusal(s, todo as string, doing as string)).toBeNull();
    expect(transitionRefusal(s, done as string, todo as string)).toBeNull();
    expect(transitionRefusal(s, todo as string, todo as string)).toBeNull();
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test -- workflow`
Expected: FAIL

- [ ] **Step 3: 実装する**

`workflow.ts`:

```ts
import type { Category, WorkspaceSettings } from "../../model/workspace";

// ワークフローのタブ(03「遷移の制約」)。

export const workflowHelp =
  "行に表示されているステータスから、チェックしたステータスにだけ遷移できるよう制御することができます。";

export interface WorkflowCell {
  to: string;
  name: string;
  self: boolean;
  checked: boolean;
}

export interface WorkflowRow {
  from: string;
  name: string;
  color: string;
  category: Category;
  cells: WorkflowCell[];
  note: "どこへでも" | "制限あり";
}

// allowed は、from から変えられる To(ステータスの並び。いまあるものだけ)。空なら制限なし。
function allowed(settings: WorkspaceSettings, from: string): string[] {
  const set = settings.workflow[from] ?? [];
  return settings.statuses.map((v) => v.id).filter((id) => id !== from && set.includes(id));
}

export function workflowRows(settings: WorkspaceSettings): WorkflowRow[] {
  return settings.statuses.map((v) => {
    const ok = allowed(settings, v.id);
    return {
      from: v.id,
      name: v.name,
      color: v.color,
      category: v.category ?? "todo",
      cells: settings.statuses.map((t) => ({
        to: t.id,
        name: t.name,
        self: t.id === v.id,
        checked: ok.includes(t.id),
      })),
      note: ok.length > 0 ? "制限あり" : "どこへでも",
    };
  });
}

export function transitionRefusal(
  settings: WorkspaceSettings,
  from: string,
  to: string,
): string | null {
  if (from === "" || from === to) return null;
  const ok = allowed(settings, from);
  if (ok.length === 0 || ok.includes(to)) return null;
  const name = (id: string) => settings.statuses.find((v) => v.id === id)?.name ?? "";
  return `ワークフローの制約: ${name(from)} から ${name(to)} には変更できません(${ok.map(name).join("・")} のみ)`;
}
```

`WorkflowPanel.svelte`(props: `state: SettingsState`): 説明 `workflowHelp`、表(`<table>`。見出しの行は空のセルとステータスの名前。行の頭は `StatusDot` と名前、セルは `self` なら「—」、ほかは `Checkbox`(ラベルは `${row.name} から ${cell.name}`、`checked`、閲覧のみなら無効で `title` に `readOnlyNote`)。変えたら `workspaceSettings.run((p, st, w) => p.setTransition(st, w, row.from, cell.to, checked))`。行の右に `row.note`。表が画面の幅より広いときは、表の箱だけを横に動かせる(`overflow-x: auto`)。

`SettingsTab.svelte`: `{:else if $settings.tab === "workflow"}<WorkflowPanel state={$settings} />`。

- [ ] **Step 4: 通るのを確かめる**

Run: `cd web && pnpm check && pnpm test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core web/app/src
git commit -m "feat(web): ワークフローのタブ(遷移の制約の表)を足す(B3 Task 11)"
```

---

### Task 12: テンプレートの一覧・種類ごとの既定・削除

**Files:**
- Create: `web/packages/core/src/features/workspaces/templates.ts`、`templates.test.ts`
- Create: `web/app/src/screens/workspace-settings/TemplatesPanel.svelte`、`KindDefaults.svelte`
- Modify: `web/app/src/screens/workspace-settings/SettingsTab.svelte`、`web/packages/core/src/index.ts`

**Interfaces:**
- Consumes: Task 10
- Produces:

```ts
export const templatesEmpty = "テンプレートはありません";
export const templateCreated = "テンプレートを作成しました";
export const templateDeleted = "テンプレートを削除しました";
export const deleteTemplateTitle = "テンプレートを削除";
export const deleteTemplateBody = (name: string) => `「${name}」を削除します。作ったアイテムには影響しません。`;
// templateSummary は、構成の件数「子 3 件・Step 8 件」。ないものは出さない。どちらもなければ ""。
export function templateSummary(t: Pick<TemplateInput, "children" | "steps">): string;
export interface TemplateRow { id: string; name: string; kind: string; summary: string }
export function templateRows(settings: WorkspaceSettings): TemplateRow[];
export interface KindDefault { kind: string; value: string; options: { value: string; label: string }[] }
// kindDefaults は、5 つの種類(Epic・Story・Task・Bug・Step の順)の既定のセレクト。候補は「なし」とその種類のテンプレート。
export function kindDefaults(settings: WorkspaceSettings): KindDefault[];
```

- [ ] **Step 1: 失敗するテストを書く**

`templates.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { WorkspaceTemplate } from "../../model/workspace";
import { defaultDetail } from "./fixtures.test-util";
import { kindDefaults, templateRows, templateSummary } from "./templates";

const tpl = (id: string, name: string, kind: string, extra: Partial<WorkspaceTemplate> = {}): WorkspaceTemplate => ({
  id, name, kind, title: "", description: "", values: {}, children: [], steps: [], ...extra,
});
const step = (title: string) => ({ id: title, title, assignee: "@parent" });

describe("template rows", () => {
  it("counts children and all steps, and hides zero parts", () => {
    const epic = tpl("t1", "リリース作業", "Epic", {
      children: [
        { id: "c1", kind: "Story", title: "QA", description: "", values: {}, steps: [step("a"), step("b")] },
        { id: "c2", kind: "Task", title: "本番", description: "", values: {}, steps: [] },
      ],
    });
    expect(templateSummary(epic)).toBe("子 2 件・Step 2 件");
    expect(templateSummary(tpl("t2", "調査", "Task", { steps: [step("a")] }))).toBe("Step 1 件");
    expect(templateSummary(tpl("t3", "Step", "Step"))).toBe("");
    const s = { ...defaultDetail("s1", "w1").settings, templates: [epic] };
    expect(templateRows(s)).toEqual([{ id: "t1", name: "リリース作業", kind: "Epic", summary: "子 2 件・Step 2 件" }]);
  });
});

describe("kind defaults", () => {
  it("offers なし and the templates of the kind", () => {
    const s = {
      ...defaultDetail("s1", "w1").settings,
      templates: [tpl("t1", "調査", "Task"), tpl("t2", "不具合", "Bug"), tpl("t3", "手順", "Task")],
      templateDefaults: { Task: "t3" },
    };
    const ds = kindDefaults(s);
    expect(ds.map((d) => d.kind)).toEqual(["Epic", "Story", "Task", "Bug", "Step"]);
    const task = ds[2];
    expect(task?.value).toBe("t3");
    expect(task?.options).toEqual([
      { value: "", label: "なし" },
      { value: "t1", label: "調査" },
      { value: "t3", label: "手順" },
    ]);
    expect(ds[0]?.value).toBe("");
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test -- templates`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `templates.ts`: Interfaces のとおり。種類の並びは `["Epic", "Story", "Task", "Bug", "Step"]`(core に同じ並びの定数があればそれを使う)。
- `TemplatesPanel.svelte`(props: `state`): 上に「種類ごとの既定」(`KindDefaults`)、その下に一覧。行は `KindIcon`、名前、`summary`(空なら出さない)、「編集」(Task 13 のダイアログを開く)、削除の `IconButton`(ごみ箱)。行がないときは `templatesEmpty`。末尾に「テンプレートを作成」のボタン(Task 13 のダイアログを新規で開く)。削除は `ConfirmDialog`(`title={deleteTemplateTitle}`、`lines={[deleteTemplateBody(name)]}`、`ok="削除"`、`danger`)で、`run((p, st, w) => p.deleteTemplate(st, w, id), templateDeleted)`。プロジェクト別のルールに添える文は B5。
- `KindDefaults.svelte`: 5 つの `Select`(ラベルは種類の名前に「の既定」を付けない。種類のアイコンと名前を前に置く)。選んだら `run((p, st, w) => p.setTemplateDefault(st, w, d.kind, value))`。閲覧のみなら無効。
- Task 13 のダイアログはまだないので、この Task では「編集」「テンプレートを作成」のボタンは `editing` の状態(`string | "new" | null`)を変えるだけにして、Task 13 でダイアログをつなぐ。
- `SettingsTab.svelte`: `{:else if $settings.tab === "templates"}<TemplatesPanel state={$settings} />`。

- [ ] **Step 4: 通るのを確かめる**

Run: `cd web && pnpm check && pnpm test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core web/app/src
git commit -m "feat(web): テンプレートの一覧・種類ごとの既定・削除を足す(B3 Task 12)"
```

---

### Task 13: テンプレートの編集ダイアログ

**Files:**
- Create: `web/packages/core/src/features/workspaces/templateForm.ts`、`templateForm.test.ts`
- Create: `web/app/src/screens/workspace-settings/TemplateDialog.svelte`、`TemplateValueFields.svelte`、`TemplateSteps.svelte`、`TemplateChildren.svelte`、`TemplateChildDialog.svelte`
- Modify: `web/app/src/screens/workspace-settings/TemplatesPanel.svelte`、`web/packages/core/src/index.ts`

**Interfaces:**
- Consumes: Task 10・12、`Member`(`model/member.ts`)
- Produces:

```ts
export interface StepForm { id: string; title: string; assignee: string }
export interface ChildForm {
  id: string; kind: ChildKind; title: string; description: string;
  values: Record<string, string | string[]>; steps: StepForm[];
}
// 値の欄の入力は文字(選択式は ID、数は文字、チェックは "on"・"off"、複数は配列)。空は "" か []。
export interface TemplateForm {
  id: string; // 新規は ""
  name: string; kind: string; title: string; description: string;
  values: Record<string, string | string[]>;
  children: ChildForm[]; steps: StepForm[];
}
export function emptyTemplateForm(): TemplateForm; // 種類は Task(03「持てるもの」)
export function formOfTemplate(t: WorkspaceTemplate): TemplateForm;
export interface ValueField {
  propertyId: string;
  label: string;
  input: "select" | "multi" | "text" | "number" | "date" | "url";
  options: { value: string; label: string }[]; // select・multi のとき
}
// valueFields は、その種類で使い、表示中で、テンプレートに持てるプロパティの欄(設定の並び)。
// 優先度は「指定しない」+値、担当者は「未割り当て」+メンバー、ユーザー型・単一選択は「指定しない」+候補、
// チェックは「指定しない」「オン」「オフ」(B3-20)、関連システム・複数選択は複数。
export function valueFields(settings: WorkspaceSettings, kind: string, members: Member[]): ValueField[];
export function stepAssigneeOptions(members: Member[]): { value: string; label: string }[]; // 親と同じ・未割り当て・メンバー
export type Structure = "children" | "steps" | null;
export function structureOf(kind: string): Structure; // Epic は子、Story・Task・Bug は Step、Step はなし
// kindChangeNote は、いまの構成を kind で保存したら捨てるものの文(B3-14)。捨てるものがなければ null。
export function kindChangeNote(form: TemplateForm, kind: string): string | null;
// toTemplateInput は、送る形。shown(画面に出した欄)だけを値として送り、空は null。ほかの値は送らない(B3-9)。
export function toTemplateInput(form: TemplateForm, settings: WorkspaceSettings): TemplateInput;
```

- [ ] **Step 1: 失敗するテストを書く**

`templateForm.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { defaultDetail } from "./fixtures.test-util";
import {
  emptyTemplateForm,
  kindChangeNote,
  stepAssigneeOptions,
  toTemplateInput,
  valueFields,
} from "./templateForm";

const settings = defaultDetail("s1", "w1").settings;
const members = [{ login: "taro", name: "田中" }];

describe("value fields", () => {
  it("shows priority, assignee and systems with their empty choices", () => {
    const fields = valueFields(settings, "Task", members);
    expect(fields.map((f) => f.label)).toEqual(["担当者", "優先度", "関連システム"]);
    expect(fields[0]?.options[0]).toEqual({ value: "", label: "未割り当て" });
    expect(fields[0]?.options[1]).toEqual({ value: "taro", label: "田中" });
    expect(fields[1]?.options[0]).toEqual({ value: "", label: "指定しない" });
    expect(fields[2]?.input).toBe("multi");
  });

  it("hides hidden properties and those the kind does not use", () => {
    const s = {
      ...settings,
      properties: settings.properties.map((p) =>
        p.key === "priority" ? { ...p, hidden: true } : p.key === "assignee" ? { ...p, kinds: ["Epic"] } : p,
      ),
    };
    expect(valueFields(s, "Task", members).map((f) => f.label)).toEqual(["関連システム"]);
  });
});

describe("step assignee options", () => {
  it("starts with 親と同じ and 未割り当て", () => {
    expect(stepAssigneeOptions(members)).toEqual([
      { value: "@parent", label: "親と同じ" },
      { value: "@none", label: "未割り当て" },
      { value: "taro", label: "田中" },
    ]);
  });
});

describe("kind change", () => {
  it("names what is not saved and omits zero parts", () => {
    const f = { ...emptyTemplateForm(), kind: "Epic", children: [
      { id: "", kind: "Story" as const, title: "QA", description: "", values: {}, steps: [{ id: "", title: "a", assignee: "@parent" }] },
    ] };
    expect(kindChangeNote(f, "Task")).toBe("子 1 件・Step 1 件は、この種類では保存されません");
    expect(kindChangeNote(f, "Epic")).toBeNull();
    const t = { ...emptyTemplateForm(), steps: [{ id: "", title: "a", assignee: "@parent" }] };
    expect(kindChangeNote(t, "Step")).toBe("Step 1 件は、この種類では保存されません");
  });
});

describe("input", () => {
  it("sends shown fields, null for empty ones, and drops empty rows", () => {
    const high = settings.priorities[1]?.id as string;
    const f = {
      ...emptyTemplateForm(), name: "調査", values: { priority: high, assignee: "", systems: [] },
      steps: [{ id: "", title: "再現", assignee: "@none" }, { id: "", title: " ", assignee: "@parent" }],
    };
    const input = toTemplateInput(f, settings);
    expect(input.values).toEqual({ priority: high, assignee: null, systems: null });
    expect(input.steps).toEqual([{ id: "", title: "再現", assignee: "@none" }]);
    expect(input.children).toEqual([]);
  });
});
```

(並びは既定の設定の順。組み込みの 8 種のうち、持てるのは担当者・優先度・関連システムで、設定の並びはステータス・担当者・優先度・開始日・終了日・ポイント・見積もり時間・関連システム。並びが違えば、既定の設定の順に合わせて期待を直す。)

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test -- templateForm`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `templateForm.ts`: Interfaces のとおり。持てるプロパティは Go の `HoldsTemplateValue` と同じ(組み込みは `priority`・`assignee`・`systems`、追加はすべて)。`toTemplateInput` は、もとの欄は `valueFields(settings, form.kind, [])` の `propertyId`、子は `valueFields(settings, child.kind, [])` の欄だけを送る。変換は、`number` は `Number(v)`、チェックの `"on"`・`"off"` は `true`・`false`、ほかはそのまま、`""` と `[]` は `null`。構成は `structureOf(form.kind)` で持てるものだけを送り、タイトルが空白だけの子と Step を捨てる。`formOfTemplate` は逆の変換(数は `String(n)`、真偽は `"on"`・`"off"`)。
- `TemplateDialog.svelte`(props: `state`、`template: WorkspaceTemplate | null`、`onclose`): `Dialog` の見出しは新規なら「テンプレートを作成」、編集なら「テンプレートの編集」。上から、名前(`Input`)・種類(`Select`。Epic・Story・Task・Bug・Step)・タイトルの書き出し(`Input`)・説明(複数行の `textarea`)・値(`TemplateValueFields`)・構成(`structureOf` で `TemplateChildren` か `TemplateSteps`)。赤字のエラーの場所を 1 つ持つ。「保存」で、`kindChangeNote(元の構成, form.kind)` があれば `ConfirmDialog`(`title="テンプレートを保存"`、`lines=[note]`、`ok="保存"`)を出し、承認したら保存する。保存は、新規なら `run((p, st, w) => p.createTemplate(st, w, input), templateCreated, (m) => (error = m))`、編集なら `run((p, st, w) => p.updateTemplate(st, w, id, input), undefined, (m) => (error = m))`。成功したら閉じる。閲覧のみなら入力と保存を無効にし、「閉じる」だけ使える。
- `TemplateValueFields.svelte`(props: `fields`、`values`(bind)、`disabled`): `select` は `Select`、`multi` は `Checkbox` の並び、`text`・`url` は `Input`、`number` は `Input type="number"`、`date` は `Input type="date"`。
- `TemplateSteps.svelte`(props: `steps`(bind)、`members`、`disabled`): 1 行に、タイトルの `Input`、担当者の `Select`(`stepAssigneeOptions`)、「上へ」「下へ」の `IconButton`(端は無効)、削除の「×」。末尾に「Step を追加」(`{ id: "", title: "", assignee: "@parent" }` を足す)。
- `TemplateChildren.svelte`(props: `children`(bind)、`state`、`disabled`): 1 行に、種類の `Select`(Story・Task・Bug)、タイトルの `Input`、「編集」(`TemplateChildDialog` を開く)、「上へ」「下へ」、「×」。末尾に「子を追加」(種類 Task)。
- `TemplateChildDialog.svelte`(props: `child`(bind)、`state`、`disabled`、`onclose`): 見出しは子のタイトル。説明・値(`valueFields(settings, child.kind, members)`)・Step(`TemplateSteps`)。「閉じる」で戻る(保存は親のダイアログの「保存」)。
- `TemplatesPanel.svelte`: Task 12 の `editing` で `TemplateDialog` を開く。
- それぞれ 200 行を超えないようにする。

- [ ] **Step 4: 通るのを確かめる**

Run: `cd web && pnpm check && pnpm test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core web/app/src
git commit -m "feat(web): テンプレートの編集ダイアログ(値・子・Step)を足す(B3 Task 13)"
```

---

### Task 14: 作成ダイアログでの適用の規則(core の関数)

**Files:**
- Create: `web/packages/core/src/features/workspaces/templateApply.ts`、`templateApply.test.ts`
- Modify: `web/packages/core/src/index.ts`

**Interfaces:**
- Consumes: Task 10 の型、Task 12 の `templateSummary`
- Produces(A3b が作成ダイアログから呼ぶ。B3-18):

```ts
// usableTemplates は、その種類のテンプレート(作成ダイアログのセレクトの候補。「なし」は呼ぶ側が足す)。
// プロジェクトの使うテンプレートでの絞り込みは B5 が引数を足す。
export function usableTemplates(settings: WorkspaceSettings, kind: string): WorkspaceTemplate[];
// defaultTemplate は、その種類の既定(使えないときは null)。プロジェクトの既定は B5。
export function defaultTemplate(settings: WorkspaceSettings, kind: string): WorkspaceTemplate | null;
export interface ApplyState {
  title: string;
  description: string;
  values: Record<string, unknown>;
  template: WorkspaceTemplate | null; // いま当てはめているテンプレート
}
// auto: ダイアログを開いた・種類を変えた・ワークスペースを変えた。select: 利用者がセレクトで選んだ。
export type ApplyMode = "auto" | "select";
// applyTemplate は、03「適用するときの規則」の入れ方。next が null なら「なし」に戻す。
// visible は、その欄を表示しているか(表示中でない・その種類で使わないプロパティは適用しない)。
export function applyTemplate(
  state: ApplyState,
  next: WorkspaceTemplate | null,
  mode: ApplyMode,
  visible: (propertyId: string) => boolean,
): ApplyState;
export interface StructureRow { depth: 0 | 1; kind: string; title: string; assignee: string } // 担当者は表示名か「未割り当て」
// templateStructure は、「一緒に作るもの」の見出しと行(02「ダイアログの項目」)。構成がなければ null。
// parentAssignee は、作成ダイアログの担当者(Story・Task・Bug の Step の「親と同じ」に使う)。
export function templateStructure(
  t: WorkspaceTemplate,
  parentAssignee: string,
  members: Member[],
): { heading: string; rows: StructureRow[] } | null;
```

- [ ] **Step 1: 失敗するテストを書く**

`templateApply.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { WorkspaceTemplate } from "../../model/workspace";
import { applyTemplate, templateStructure } from "./templateApply";

const tpl = (id: string, extra: Partial<WorkspaceTemplate>): WorkspaceTemplate => ({
  id, name: id, kind: "Task", title: "", description: "", values: {}, children: [], steps: [], ...extra,
});
const taskT = tpl("task", { title: "調査: ", description: "## 手順", values: { priority: "high", assignee: "taro" } });
const bugT = tpl("bug", { kind: "Bug", description: "## 概要" });
const all = () => true;
const empty = { title: "", description: "", values: {}, template: null };

describe("apply template", () => {
  it("fills an empty title and keeps a typed one", () => {
    expect(applyTemplate(empty, taskT, "auto", all).title).toBe("調査: ");
    expect(applyTemplate({ ...empty, title: "ログイン" }, taskT, "select", all).title).toBe("ログイン");
  });

  it("auto: puts the description only when empty or from the current template", () => {
    const applied = applyTemplate(empty, taskT, "auto", all);
    expect(applied.description).toBe("## 手順");
    // 種類を Bug に切り替えた: テンプレート由来なので置き換わる。
    expect(applyTemplate(applied, bugT, "auto", all).description).toBe("## 概要");
    // 1 文字でも足していれば残る。
    expect(applyTemplate({ ...applied, description: "## 手順!" }, bugT, "auto", all).description).toBe("## 手順!");
  });

  it("select: replaces the description; なし clears only a template's description", () => {
    const typed = { ...empty, description: "メモ" };
    expect(applyTemplate(typed, taskT, "select", all).description).toBe("## 手順");
    const applied = applyTemplate(empty, taskT, "select", all);
    expect(applyTemplate(applied, null, "select", all).description).toBe("");
    expect(applyTemplate({ ...applied, description: "メモ" }, null, "select", all).description).toBe("メモ");
  });

  it("overrides only the values the template has and the dialog shows", () => {
    const state = { ...empty, values: { priority: "low", systems: ["web"] } };
    const got = applyTemplate(state, taskT, "select", (id) => id !== "assignee");
    expect(got.values).toEqual({ priority: "high", systems: ["web"] });
    expect(got.template?.id).toBe("task");
  });
});

describe("template structure", () => {
  const members = [{ login: "taro", name: "田中" }, { login: "hanako", name: "佐藤" }];

  it("lists children and their steps for an Epic", () => {
    const epic = tpl("epic", { kind: "Epic", children: [
      { id: "c1", kind: "Story", title: "QA", description: "", values: { assignee: "hanako" }, steps: [
        { id: "s1", title: "確認", assignee: "@parent" }, { id: "s2", title: "記録", assignee: "@none" },
      ] },
      { id: "c2", kind: "Task", title: "本番", description: "", values: {}, steps: [] },
    ] });
    expect(templateStructure(epic, "taro", members)).toEqual({
      heading: "一緒に作るもの: 子 2 件・Step 2 件",
      rows: [
        { depth: 0, kind: "Story", title: "QA", assignee: "佐藤" },
        { depth: 1, kind: "Step", title: "確認", assignee: "佐藤" },
        { depth: 1, kind: "Step", title: "記録", assignee: "未割り当て" },
        { depth: 0, kind: "Task", title: "本番", assignee: "未割り当て" },
      ],
    });
  });

  it("uses the dialog's assignee for a Task's 親と同じ", () => {
    const t = tpl("t", { steps: [{ id: "s1", title: "再現", assignee: "@parent" }] });
    expect(templateStructure(t, "taro", members)?.rows).toEqual([
      { depth: 0, kind: "Step", title: "再現", assignee: "田中" },
    ]);
    expect(templateStructure(tpl("n", {}), "taro", members)).toBeNull();
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test -- templateApply`
Expected: FAIL

- [ ] **Step 3: 実装する**

`templateApply.ts`:

```ts
import type { Member } from "../../model/member";
import type { WorkspaceSettings, WorkspaceTemplate } from "../../model/workspace";
import { stepNone, stepParent } from "../../model/workspace";
import { templateSummary } from "./templates";

// 作成ダイアログでのテンプレートの適用(03「適用するときの規則」、02「ダイアログの中での連動」)。
// 作成ダイアログへのはめ込みは A3b(B3-18)。

export function usableTemplates(settings: WorkspaceSettings, kind: string): WorkspaceTemplate[] {
  return settings.templates.filter((t) => t.kind === kind);
}

export function defaultTemplate(settings: WorkspaceSettings, kind: string): WorkspaceTemplate | null {
  const id = settings.templateDefaults[kind];
  return usableTemplates(settings, kind).find((t) => t.id === id) ?? null;
}

export interface ApplyState {
  title: string;
  description: string;
  values: Record<string, unknown>;
  template: WorkspaceTemplate | null;
}

export type ApplyMode = "auto" | "select";

export function applyTemplate(
  state: ApplyState,
  next: WorkspaceTemplate | null,
  mode: ApplyMode,
  visible: (propertyId: string) => boolean,
): ApplyState {
  // 説明が空か、いまのテンプレートの説明と一字一句同じなら、テンプレート由来とみなす。
  const derived =
    state.description === "" ||
    (state.template !== null && state.description === state.template.description);
  let description = state.description;
  if (next === null) {
    if (derived) description = "";
  } else if (mode === "select" || derived) {
    description = next.description;
  }
  const values = { ...state.values };
  for (const [id, v] of Object.entries(next?.values ?? {})) {
    if (visible(id)) values[id] = v;
  }
  return {
    title: state.title.trim() === "" && next !== null ? next.title : state.title,
    description,
    values,
    template: next,
  };
}

export interface StructureRow {
  depth: 0 | 1;
  kind: string;
  title: string;
  assignee: string;
}

export function templateStructure(
  t: WorkspaceTemplate,
  parentAssignee: string,
  members: Member[],
): { heading: string; rows: StructureRow[] } | null {
  const summary = templateSummary(t);
  if (summary === "") return null;
  const name = (login: string) =>
    login === "" ? "未割り当て" : (members.find((m) => m.login === login)?.name ?? login);
  const stepRows = (steps: WorkspaceTemplate["steps"], parent: string, depth: 0 | 1): StructureRow[] =>
    steps.map((s) => ({
      depth,
      kind: "Step",
      title: s.title,
      assignee: name(s.assignee === stepParent ? parent : s.assignee === stepNone ? "" : s.assignee),
    }));
  const rows: StructureRow[] = [];
  for (const c of t.children) {
    const a = typeof c.values.assignee === "string" ? c.values.assignee : "";
    rows.push({ depth: 0, kind: c.kind, title: c.title, assignee: name(a) });
    rows.push(...stepRows(c.steps, a, 1));
  }
  rows.push(...stepRows(t.steps, parentAssignee, 0));
  return { heading: `一緒に作るもの: ${summary}`, rows };
}
```

- [ ] **Step 4: 通るのを確かめる**

Run: `cd web && pnpm check && pnpm test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core
git commit -m "feat(core): 作成ダイアログでのテンプレートの適用の規則を足す(B3 Task 14)"
```

---

### Task 15: 詳細パネルのメニューと「テンプレートとして保存…」

**Files:**
- Create: `web/packages/core/src/features/workspaces/saveAsTemplate.ts`、`saveAsTemplate.test.ts`
- Create: `web/app/src/screens/item/ItemMenu.svelte`、`web/app/src/screens/item/SaveAsTemplateDialog.svelte`
- Modify: `web/app/src/screens/item/DetailHeader.svelte`(メニューのボタンを 1 つ)、`web/app/src/wire.ts`、`web/packages/core/src/index.ts`

**Interfaces:**
- Consumes: Task 10 の `saveItemAsTemplate`、Task 12 の `templateCreated`、A2 の `DetailView`、`readOnlyNote`
- Produces:

```ts
export const saveAsTemplateLabel = "テンプレートとして保存…";
export interface SaveAsTemplateModel {
  name: string;          // 初期値はアイテムのタイトル
  title: string;         // 初期値はアイテムのタイトル
  showStructure: boolean; // 子も Step もないアイテムでは出さない
}
export function saveAsTemplateDialog(view: DetailView): SaveAsTemplateModel;
export interface SaveAsTemplate {
  // save は、保存してトースト「テンプレートを作成しました」を出す。断りは onRefused に渡し、false を返す。
  save(view: DetailView, input: SaveAsTemplateInput, onRefused: (message: string) => void): Promise<boolean>;
}
export function createSaveAsTemplate(deps: { port: WorkspacesPort; toasts: Toasts }): SaveAsTemplate;
```

- [ ] **Step 1: 失敗するテストを書く**

`saveAsTemplate.test.ts`(`DetailView` は、`features/items/fixtures.test-util.ts` の既存の作り方で作る。子のない Task と、Step が 1 つの Task):

```ts
import { describe, expect, it } from "vitest";
import { Refused } from "../../model/errors";
import { get } from "../../reactive/store";
import { createToasts } from "../toasts/toasts";
import { fakeWorkspacesPort } from "./fixtures.test-util";
import { createSaveAsTemplate, saveAsTemplateDialog } from "./saveAsTemplate";

describe("save as template", () => {
  it("starts from the item's title and hides the structure check without children", () => {
    expect(saveAsTemplateDialog(viewOf("ログイン", []))).toEqual({
      name: "ログイン", title: "ログイン", showStructure: false,
    });
    expect(saveAsTemplateDialog(viewOf("ログイン", ["設計"])).showStructure).toBe(true);
  });

  it("toasts on success and passes a refusal to the dialog", async () => {
    const port = fakeWorkspacesPort();
    const toasts = createToasts();
    const s = createSaveAsTemplate({ port, toasts });
    expect(await s.save(viewOf("x", []), { name: "x", title: "x", structure: true }, () => {})).toBe(true);
    expect(get(toasts.items).at(-1)?.text).toBe("テンプレートを作成しました");
    port.saveItemAsTemplate = async () => {
      throw new Refused("同じ名前のテンプレートがあります");
    };
    let msg = "";
    expect(await s.save(viewOf("x", []), { name: "x", title: "x", structure: true }, (m) => (msg = m))).toBe(false);
    expect(msg).toBe("同じ名前のテンプレートがあります");
  });
});
```

`viewOf(title, childTitles)` は、テストの中に置く道具(`DetailView` を、Store `s1`・アイテム `i1`・種類 Task・子は Step で作る)。トーストの読み方(`toasts.items`・`text`)は、既存の `toasts` のテストに合わせる。

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test -- saveAsTemplate`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `saveAsTemplate.ts`: Interfaces のとおり。`save` は `port.saveItemAsTemplate(view.store.id, view.item.id, input)`。`Refused` なら `onRefused(e.message)` で `false`、成功なら `toasts.show(templateCreated)` で `true`。ほかの誤りは投げ直す。設定の画面が開いていれば読み直すため、`wire.ts` では成功のあとに `workspaceSettings.reload()` を呼ぶ(開いていなければ何もしない)。
- `ItemMenu.svelte`(props: `v: DetailView`): `IconButton`(ラベル「メニュー」)で `Menu` を開き、項目は `saveAsTemplateLabel` だけ(B3-17)。閲覧のみの Store では無効にし、`title` に `readOnlyNote`。押すと `SaveAsTemplateDialog` を開く。
- `SaveAsTemplateDialog.svelte`(props: `v`、`onclose`): `Dialog`(見出し「テンプレートとして保存」)。名前・タイトルの書き出しの `Input`、`showStructure` のときだけ `Checkbox`「子と Step を含める」(初期値オン)、赤字のエラーの場所、「保存」。保存できたら閉じる。
- `DetailHeader.svelte`: 全画面・閉じるのボタンの前に `<ItemMenu {v} />` を置く。コメントの「ウォッチとメニューは A4」を「ウォッチと、メニューのほかの項目は A4。メニューの枠と「テンプレートとして保存…」は B3(B3-17)」に直す。

- [ ] **Step 4: 通るのを確かめる**

Run: `cd web && pnpm check && pnpm test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core web/app/src
git commit -m "feat(web): 詳細パネルのメニューに「テンプレートとして保存…」を足す(B3 Task 15)"
```

---

### Task 16: e2e(3 幅)

**Files:**
- Create: `web/e2e/templates.spec.ts`

**Interfaces:**
- Consumes: Task 11〜15 の画面。e2e のサーバーの Store「e2e」(ワークスペース `HH`、プロジェクト `HH-P1`、Epic「認証」)と、`workspace-settings.spec.ts` の作り方(作成ダイアログでワークスペースを作る)

- [ ] **Step 1: テストを書く**

`web/e2e/templates.spec.ts`。サーバーは 3 つの幅で共有なので、ワークスペースの名前・接頭辞・テンプレートの名前は `unique` と乱数で毎回変える(`workspace-settings.spec.ts` の `prefixOf` と `createWorkspace` を同じ形でこのファイルに置く)。

1. **ワークフロー**: 新しいワークスペースの設定の「ワークフロー」を開く。「Todo」の行に「どこへでも」が出る。チェックボックス「Todo から InProgress」を押すと、行が「制限あり」になり、読み直しても残る。もう一度押すと「どこへでも」に戻る。自分自身のセルは「—」。
2. **テンプレートの作成と一覧**: 「テンプレート」を開くと「テンプレートはありません」。「テンプレートを作成」で、名前に一意の名前、種類 Task、タイトルの書き出し「調査: 」、優先度 High、Step「再現」(担当者「未割り当て」)を入れて「保存」。トースト「テンプレートを作成しました」、一覧の行に名前と「Step 1 件」。同じ名前でもう 1 つ作ると、ダイアログの中に「同じ名前のテンプレートがあります」。
3. **種類ごとの既定**: 「Task」の既定のセレクトでそのテンプレートを選ぶ。読み直しても選ばれている。
4. **種類の変更**: 「編集」で種類を Epic にして「保存」すると、確認ダイアログ「テンプレートを保存」に「Step 1 件は、この種類では保存されません」。「保存」で一覧の行の件数が消え、「Task」の既定は「なし」に戻る。
5. **削除**: ごみ箱で確認ダイアログ「テンプレートを削除」に「「{名前}」を削除します。作ったアイテムには影響しません。」。「削除」でトースト「テンプレートを削除しました」、一覧は「テンプレートはありません」。
6. **アイテムから作る**: `HH-P1` のツリーで「認証」を開き、詳細パネルから Task を作り(`detail.spec.ts` と同じ手順)、Step「設計」を足す。パネルの「メニュー」から「テンプレートとして保存…」を押すと、名前とタイトルの書き出しにタスクのタイトルが入り、「子と Step を含める」がオン。名前を一意の名前にして「保存」。トースト「テンプレートを作成しました」。`/s/e2e/w/HH/settings` の「テンプレート」に、その名前と「Step 1 件」の行が出る。
7. **閲覧のみ**: 閲覧のみの Store(e2e のサーバーにある、`detail.spec.ts` の「a read-only store's panel…」で使っているもの)のアイテムを開くと、メニューの「テンプレートとして保存…」が無効。

- [ ] **Step 2: 流す**

Run: `make e2e`
Expected: PASS(3 幅すべて。既存の e2e も通る)

- [ ] **Step 3: コミット**

```bash
git add web/e2e/templates.spec.ts
git commit -m "test(e2e): ワークフローとテンプレートの通しのテストを足す(B3 Task 16)"
```

---

## 最後に

- [ ] ブランチ全体を最終レビューする(superpowers:requesting-code-review)。指摘を直す。
- [ ] `make check` と `make e2e` を流す。
- [ ] トラックの PR を 1 本出す(向き先 develop、タイトル「feat: トラック B3(ワークフローとテンプレート)」)。本文に、できること・決めたこと(B3-1〜B3-20)・A3b・A4・B4・B5・C・E への申し送り(漏れの表の「範囲の外」の行)・確認の結果を書く。force push は拒否されるので、載せ直すときは別名のブランチで出す。
- [ ] A3b への申し送り: `transitionRefusal`・`CheckTransition` を呼ぶ場所(ステータスのメニュー・Step のチェック・`SetProperty` のステータス・`CheckStep`)、作成ダイアログで `usableTemplates`・`defaultTemplate`・`applyTemplate`・`templateStructure` を使うこと、テンプレートでまとめて作る保存(番号は深さ優先、スプリントはもとだけ、子の優先度はテンプレートの値か初期優先度、Step は初期優先度、担当者の「親と同じ」「未割り当て」)、「続けて作成」で説明を空にしたあとは入れ直さないこと。
- [ ] A4 への申し送り: 詳細パネルのメニューの枠は B3 が作った(B3-17)。ほかの項目はそこへ足す。説明を足すときに、アイテムから作るテンプレートへ説明を写す。削除済みの子を写さない。
- [ ] A3a(別のセッションで進行中)への申し送り: `WorkspaceSettings` に必須の欄 `workflow`・`templates`・`templateDefaults` が増えた。あとから develop を取り込む側で、テスト用の設定(A3a の `features/items/settings.test-util.ts` など)に 3 つの欄を足さないと `pnpm check` が落ちる。
