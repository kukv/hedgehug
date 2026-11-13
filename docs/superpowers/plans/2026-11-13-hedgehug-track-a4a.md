# トラック A4 前半(詳細パネルの残り) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 詳細パネルの説明・Markdown・メンションの表示・関連・作業時間・コメント・ウォッチ・履歴と、作成ダイアログの説明、プロジェクトの概要の編集・リポジトリの追加と削除を、仕様どおりに保存し表示する。

**Architecture:** 説明は `description.md` に本文を置き、`desc` の行で履歴に残す。保存の口(`port`)に説明の読み書きを足し、保存のときの 3 方向の行マージは domain の `textmerge` に移して、同期(gitstore)と保存(app)で同じ関数を使う。コメント・作業時間は `comment`・`worklog` の行、関連・ウォッチ・リポジトリは `add`・`remove` の行で持つ。新しい操作は機能ごとの新しい app パッケージ(`descriptions`・`comments`・`worklogs`・`relations`・`watching`・`history`)に置き、プロジェクトの概要の編集は `app/projects` に足す。画面は core に状態と規則(Markdown の字句分け、書きかけの判定、作業時間の検査と表記)を置き、ui は見た目だけ、app は結び付けだけにする。

**Tech Stack:** Go(huma・domain/app/infra/adapter の層)、TypeScript(core・api の orval)、Svelte 5(ui・app)、Vitest、Playwright。

**Spec:** `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md`(「詳細パネル」の「説明」「Markdown」「メンション」「関連」「作業時間」「コメント」「ウォッチ」「履歴」、「アイテムの作成」の説明の欄と「ダイアログの中での連動」、「プロジェクト」の「概要の編集」「リポジトリの追加と削除」、「操作ごとの記録と通知」)、`03-workspace-settings.md`(「適用するときの規則」の説明、「アイテムからテンプレートを作る」)、`06-sync-notifications.md`(「同時に編集したとき」の説明、「画面への反映」の反映を待つとき)、`07-github.md`(`owner/name` の形)。保存形式は `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`(「ファイル構成」「アイテムの例」「変更記録」「アクティビティと履歴」)。

**前提として読むもの:** トラック A の計画 `2026-11-12-hedgehug-track-a.md` の A4 の行、A3 後半の計画 `2026-11-13-hedgehug-track-a3b.md` の「A4・B5 への申し送り」、B3 の計画の B3-16・B3-17、B2 の計画の B2-9・B2-11、PR #166・#170・#172 本文の A4 への申し送り。

## Global Constraints

- 依存は追加しない(Markdown も自前で字句に分ける。足すときは版を明記し、公開から 7 日以上の版だけ)。
- 大きさの上限: Go ファイル 300 行(テスト 600 行)・関数 60 行・認知的複雑度 15・パッケージ 20 ファイル、Svelte 200 行、TS 300 行。超えたら分ける。パッケージのファイル数はテストを除いて数える。上限の 20 にいる `internal/infra/gitstore`・`internal/app/workspaces`・`internal/app/apptest` には新しいファイルを足さず、既存のファイルに足す(`internal/app/items` は 13)。`web/packages/core/src/features/items/detail.ts` は 256 行、`web/app/src/screens/item/PropertyField.svelte` は 194 行、`TreeRow.svelte` は 196 行。
- 向き: adapter→app→domain、infra→app/port。app の機能どうしは import しない(テストも)。
- 画面の文言は仕様の文字どおり。モックの動きは写さない(見た目だけ参考: モック版 19 の `detail.js`・`screens.js`)。
- 通知(メンション・ウォッチ・オーナー・「上書き」)は作らない。受信箱と通知の判定は C4 が、同期で受け取った行から作る。A4a は行を正しく書くだけ。
- 閲覧のみの Store では、書く操作を無効にし「閲覧のみの Store です」を添える(02「詳細パネル」の冒頭の一覧)。
- 各タスクは TDD(失敗するテスト → 失敗を確かめる → 実装 → 通す → コミット)。タスクごとにレビューする。
- コミットの末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`。
- PR はトラックで 1 本(develop 向け、ブランチ `feat/track-a4a`)。force push は拒否されるので、載せ直すときは別名のブランチで出す。
- 計画やテストにトークンらしい値(ghp_ など)を書かない。
- 確認のコマンド: Go は `go test ./internal/...`、全体は `make check`、画面は `pnpm -C web test`・`pnpm -C web check`、API の型は `make generate` のあと `pnpm -C web gen`、e2e は `pnpm -C web e2e`(各タスクの手順に正確な形を書く)。

---

## 持ち場と境目

| 入れるもの | 仕様 |
|---|---|
| アイテム・プロジェクトの説明の表示と編集(入力中の確認、保存のときの行マージ、同じ行のトースト、書きかけのあいだ反映を待つ) | 02「説明」、06「同時に編集したとき」 |
| Markdown の表示(説明・コメント)とメンションの表示 | 02「Markdown」「メンション」 |
| 作成ダイアログの説明の欄、テンプレートの説明の当てはめ(自動とセレクト、テンプレート由来の判定、「続けて作成」で空に)、テンプレートの子の説明の保存、アイテムから作るテンプレートへ説明を写す | 02「ダイアログの項目」「ダイアログの中での連動」「作成したあと」、03「適用するときの規則」「アイテムからテンプレートを作る」 |
| 関連の一覧・追加・解除、候補の検査(同じ種類・循環)、「ブロック中」(詳細パネルの種類の行とツリーの行) | 02「関連」「上から順の構成」、04「ツリー」の行 |
| 作業時間の一覧・記録・削除と見出しの合計 | 02「作業時間」 |
| コメントの一覧・投稿・編集・削除とメンションのボタン(アイテムとプロジェクト) | 02「コメント」「メンション」、「概要の編集」のコメント |
| ウォッチのボタン | 02「ウォッチ」 |
| 履歴(アイテムとプロジェクト。40 件と「すべて表示」) | 02「履歴」、「概要の編集」の履歴 |
| プロジェクトの概要の編集(タイトル・説明・ステータス・オーナー・開始日・終了日)、リポジトリの追加と削除、Epic が 0 件の文言 | 02「概要の編集」「リポジトリの追加と削除」 |

| 入れないもの | 回す先 | 理由 |
|---|---|---|
| 削除・移動・種類の変更・複製・メニューの残りの項目・上下移動・ツリーのキー操作(Space を含む)・Step の「×」・プロジェクトの名前の変更と削除、削除済みを件数・候補・一覧から除くこと(B2-11、A2 の MoveUp) | A4b | ユーザー決定(2026-11-13、A4 を 2 分割)。削除がまだないので、削除済みはまだ起きない |
| 通知と受信箱(メンション・ウォッチ・オーナー・「上書き」・担当者) | C4 | 通知は受け取ったマシンが作る(06)。`port.Inbox` は PR #173 で未マージ |
| 履歴の「同期で受信」 | C4 | どの保存を同期で受け取ったかは、このマシンの記録(statedb)が要り、C4 のアクティビティと同じ材料。保存形式の行には載らない |
| 履歴の「スプリントを変更」「リリースを変更」「復元」「{旧 ID} から移動」の文 | F・A4b・A5・G | その行を書くトラックが文を足す。A4a は知らない種類の行を出さない |
| リポジトリの「PR を探せませんでした: {理由}」、PR と Issue の区画の中身 | D3・D4 | GitHub を読むトラックの表示 |
| 関連の検索のキー(`blocks:` など) | E | 検索のトラック |
| プロジェクトのメニューの「名前を変更…」ダイアログと、設定の画面の「名前」の区画 | A4b | プロジェクトの設定の画面(削除の区画)と同じところ。`rename-project`(Task 18)をそのまま使う |
| 詳細パネルの全画面の 2 列(本文とプロパティ) | 範囲の外(A2 の残り) | A4a の区画は列の分け方によらず動く。いまの `SidePanel` にないことは PR で知らせる |

## 決めたこと

| # | 決定 | 根拠 |
|---|---|---|
| A4a-1 | 説明の本文は、アイテムは `items/<ID>/description.md`、プロジェクトは `projects/<ID>/description.md` に置く。空の説明はファイルを置かない(消す)。保存の口に `ItemReader.Description(id) (string, error)`・`ItemWriter.Describe(id, text string) error`、プロジェクトにも同じ 2 つを足す。`Describe` はファイルだけを書き、履歴の `desc` の行は app が `change.NewDesc()` で別に足す | 保存形式「ファイル構成」「変更記録」の `desc`。口は git の言葉を使わない |
| A4a-2 | 行マージ `MergeText` を `internal/infra/gitstore/filemerge` から `internal/domain/textmerge` に移し、同じ箇所を両方が変えたかを返す `Merge(base, older, newer string) (string, bool)` を足す。gitstore の同期は `Merge` の文字列だけを使う | app は infra を import できない。保存(02「説明」)と同期(06)で同じ規則 |
| A4a-3 | 説明の保存は、画面が「編集を始めたときの説明」(`base`)と入力(`text`)を送る。サーバーは保存の中で読んだいまの説明を `older`、入力を `newer` にして `textmerge.Merge` で合わせる。合わせた結果が保存してある説明と同じなら何も保存しない。両方を残したときは応答の `conflict: true` で知らせ、画面がトースト「説明の同じ箇所が同時に編集されたため、両方を残しました」を出す | 02「説明」、06「同時に編集したとき」(古いほうを先にする = 保存してあるほうが先) |
| A4a-4 | 作成の入力に説明(`description`)を足す。作成と同じ保存で `Describe` する(`desc` の行は書かない。履歴は「作成」1 行)。テンプレートの子の説明も、子の作成と同じ保存で書く。Step は説明を持たない(テンプレートの Step に説明はない) | 02「作成したあと」、03「適用するときの規則」、A3b の申し送り |
| A4a-5 | アイテムからテンプレートを作る(B3 の `save-as-template`)ときに、アイテムと子の説明を写す | B3-16、PR #170 の申し送り |
| A4a-6 | 作成ダイアログの説明は、A3b の `applyDefault` を `"auto"` で呼ぶ。自動で当てはめるときは、説明が空か、いま当てはめているテンプレートの説明と一字一句同じときだけ入れる。セレクトで選んだときは置き換える。「なし」に戻すときは、テンプレート由来なら空に戻す。「続けて作成」で空にする | 02「開き方と初期値」「ダイアログの中での連動」「作成したあと」、03「適用するときの規則」 |
| A4a-7 | コメントは、アイテム(プロジェクト)のログの `t: "comment"` の行で持つ。`f` はコメントの内部 ID、`to` は `{"body": 本文}`、削除は `to` を `null` にした行。投稿者と投稿の時刻は、そのコメントの最初の行の `by`・`at`。行が 2 つ以上あり、いまの値が削除でなければ「(編集済み)」。いまの値は `change.Resolve` と同じ先端の規則で決め、先端に削除と編集が両方あれば編集を残す | 保存形式「変更記録」の `comment`、06「一方が削除し、他方が変更した」 |
| A4a-8 | 作業時間は `t: "worklog"` の行で持つ。`f` は記録の内部 ID、`to` は `{"hours": 数, "memo": 文字列, "date": "YYYY-MM-DD"}`、削除は `to` を `null`。記録した人は最初の行の `by`。日付は保存したマシンの今日(`Clock`)。決め方は A4a-7 と同じ | 保存形式、02「作業時間」 |
| A4a-9 | 作業時間の検査は domain(`worklog.ParseHours`)に置き、同じ規則を core にも置く: 空・数でない・0 以下は「時間を入力してください」、小数第 3 位以下があれば「時間は小数第 2 位までで入力してください」。時間は 100 分の 1 時間の整数で持ち、合計も整数で足す。表記は小数第 2 位まで・末尾の 0 を省き「N 時間」 | 02「作業時間」 |
| A4a-10 | 関連は、関連を足した側のアイテムのログに、複数の値の項目 `relations` の `add`・`remove` で持つ。値は `{"type": 種類, "to": 相手の内部 ID}`。種類は `blocks`・`causes`・`duplicates`・`relates`・`clones`。「blocked by HH-3」の追加は HH-3 のログに `{blocks, このアイテム}` を足す。`relates to` は足した側に置く。解除は、関連を持っている側のログに `remove` を書く | 保存形式「アイテムの例」の `relations`、02「関連」 |
| A4a-11 | 関連の一覧(逆向きの表記を含む)・「ブロック中」・候補・循環の検査は、Store のすべてのアイテムを読んで作る(ワークスペース → プロジェクト → アイテムの順に既存の口で読む。新しい口は足さない)。規則は domain の `relation` パッケージに置く | 02「関連」。ワークスペースをまたぐ |
| A4a-12 | 関連の候補は、サーバーの `GET …/items/{item}/relation-candidates?label=…&q=…` が返す。ID(表示の ID、大文字小文字を区別しない前方一致)かタイトル(部分一致、大文字小文字を区別しない)で絞り、ワークスペースの並び → 番号の順に最大 20 件。同じ種類の関連がある候補は `disabled` と理由「すでに {表記} の関連があります」、blocks/blocked by で循環になる候補は「循環になります」 | 02「関連」の「追加」。件数の上限は仕様にないので、画面に収まる 20 件にする |
| A4a-13 | 「ブロック中」は、アイテムの応答に `blocked: boolean` を足して返す(詳細とツリーの一覧の両方)。ブロックしている相手が完了・中止の分類でないときに真。削除済みの相手を除くのは A4b | 02「上から順の構成」、A2 の決定 17・A3a の決定 22 |
| A4a-14 | ウォッチは既存の `watchers` の項目に、あなたのログイン名を `add`・`remove` する。すでにその状態なら何も保存しない。アクティビティは「{ID} のウォッチを開始」「{ID} のウォッチを解除」 | 02「ウォッチ」 |
| A4a-15 | 履歴は、サーバーが行の文まで作って返す(`GET …/items/{item}/history`・`GET …/projects/{project}/history`)。新しい順にすべて返し、画面が 40 件で切って「すべて表示」を出す。出す行は `create`・`set`(プロパティ・タイトル・親と場所・種類)・`desc`、関連システムと複数選択のプロパティの `add`/`remove`。`comment`・`worklog`・関連・ウォッチ・リンク・リポジトリの `add`/`remove`・`delete` は出さない。知らない種類の行も出さない。旧と新は、ログをその行の直前までとその行までで切って `change.Resolve` で決める(同時の変更も `prev` の規則で正しく並ぶ) | 02「履歴」、保存形式「アクティビティと履歴」 |
| A4a-16 | 履歴で、ワークスペースのログに同じ `op` の `delete` の行(値・プロパティの削除)があるアイテムの行は出さない | B2-9 |
| A4a-17 | 履歴の操作した人: `actor` が自動化は「自動化」、GitHub は「GitHub」、同期は「同期」、人はメンバーの表示名(いなければ「不明なユーザー」)。値の名前は、いまのワークスペースの設定で引き、ないものは「(削除された値)」 | 02「履歴」、01 |
| A4a-18 | プロジェクトの概要の編集は `app/projects` に `Rename`・`SetStatus`・`SetOwner`・`SetDate`(開始日・終了日を 1 つずつ)・`AddRepo`・`RemoveRepo` を足す(説明は Task 3 の `app/descriptions`)。アクティビティは「{タイトル} の{項目名}を {値} に変更」(説明は「{タイトル} の説明を変更」)。{タイトル} は変える前のもの。タイトルの重複の比べ方は作成と同じ(`project` の既存の検査) | 02「概要の編集」 |
| A4a-19 | リポジトリは、プロジェクトのログの複数の値の項目 `repos` に `owner/name` の文字列で持つ。形は 07 の `^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`(前後の空白を除いて見る)、重複は大文字小文字を区別せずに比べる | 02「リポジトリの追加と削除」、07 |
| A4a-20 | Markdown は core の `markdown.ts` が行とインラインの字句に分け、ui の `Markdown.svelte` が Svelte の要素で描く(`{@html}` は使わない)。リンクは `http://`・`https://` だけ、`target="_blank" rel="noopener noreferrer"`。`@ログイン名` は、Store のメンバーのログイン名と大文字小文字を区別せずに一致したものだけをメンションの見た目にする | 02「Markdown」「メンション」。XSS を避ける |
| A4a-21 | 書きかけ(説明・コメントの入力欄で、中身が空でなく保存してあるものと違う)は、core の `drafts` に登録し、app の `holdReasons`(`web/app/src/tab.ts`)に 1 つ足して、自動の同期と画面の反映を待たせる | 02「説明」、06「反映を待つとき」、C2 画面の決定 S9 |
| A4a-22 | 入力を破棄する確認(「入力を破棄」)は、入力欄の中の Esc、別のアイテムの詳細パネルを開く、パネルを閉じる(「×」・Esc)、画面を移る、で出す。core の `drafts` が「離れてよいか」を答え、詳細パネルとルーターがそれを聞く。日本語の変換中の Esc は対象外 | 02「説明」 |
| A4a-23 | メンションのボタン(コメントの入力欄の下)は、あなた以外のメンバーのログイン名を並べ、押すと入力の末尾に `@ログイン名 ` を足す | 02「メンション」 |
| A4a-24 | 区画ごとに読み取りの API を分ける(説明・コメント・作業時間・関連・履歴)。書く操作は 1 操作 1 エンドポイント(下の「タスク間の名前」) | 区画ごとにタスクを独立させる。ローカルのサーバーなので読み取りの回数は問題にならない |
| A4a-25 | プロジェクトのタイトルの編集(概要)は、トラック A の表で A4 の「名前の変更」と同じ操作なので A4a に入れる。A4b に残るのはプロジェクトの削除だけ | 02「概要の編集」「名前の変更・移動・削除」 |
| A4a-26 | 作業時間の時間は、API では文字列で受け取り、サーバーが A4a-9 で検査する(画面も同じ検査を先にする) | 小数第 3 位の判定を数の丸めに左右させない |

---

## ファイルの地図

| 場所 | 新規・変更 | 役割 |
|---|---|---|
| `internal/domain/textmerge/` | 新規(`filemerge/text.go` から移す) | 説明の 3 方向の行マージ |
| `internal/domain/change/record.go`・`draft.go` | 変更 | `Desc`・`Comment`・`Worklog` の種類と `NewDesc`・`NewComment`・`NewWorklog` |
| `internal/domain/comment/` | 新規 | コメントの行の決め方(A4a-7) |
| `internal/domain/worklog/` | 新規 | 作業時間の行の決め方・検査・表記(A4a-8・9) |
| `internal/domain/relation/` | 新規 | 関連の種類・表記・一覧・循環・ブロック中(A4a-10〜13) |
| `internal/domain/history/` | 新規 | 履歴の行の文(A4a-15〜17) |
| `internal/domain/project/` | 変更 | リポジトリの形と、プロジェクトの項目の変更の行 |
| `internal/app/port/stores.go`・`internal/app/apptest/`・`internal/infra/gitstore/` | 変更 | 説明の読み書きの口・偽物・本物・照合テスト |
| `internal/app/descriptions/`・`comments/`・`worklogs/`・`relations/`・`watching/`・`history/` | 新規 | 操作(1 ファイル 1 操作) |
| `internal/app/items/create.go`・`createtemplate.go` | 変更 | 作成の説明(A4a-4) |
| `internal/app/workspaces/`(save-as-template のファイル) | 変更 | 説明を写す(A4a-5) |
| `internal/app/projects/` | 変更 | 概要の編集・リポジトリ |
| `internal/adapter/httpapi/` の `items`・`projects` と新しい `notes`(コメント・作業時間・関連・ウォッチ・履歴・説明) | 変更・新規 | API |
| `web/packages/core/src/features/markdown/`・`drafts/`・`notes/` | 新規 | Markdown の字句、書きかけ、コメント・作業時間・関連・履歴の状態 |
| `web/packages/ui/src/patterns/` | 変更 | `Markdown.svelte` ほか見た目の部品 |
| `web/app/src/screens/item/`・`project/` | 変更 | 詳細パネルと概要の結び付け |

## タスクの一覧

| # | タスク | 依存 |
|---|---|---|
| 1 | 行マージを domain の `textmerge` に移し、両方を残したかを返す | なし |
| 2 | 説明の読み書きの口(port・偽物・gitstore・照合テスト)と `desc` の行 | 1 |
| 3 | 説明の保存(アイテム・プロジェクト)と「詳細の付録」の読み取り API | 2 |
| 4 | 作成の説明・テンプレートの子の説明・アイテムから作るテンプレートへ写す | 2 |
| 5 | Markdown の字句分けと `Markdown.svelte` | なし |
| 6 | 書きかけ(`drafts`)・破棄の確認・反映を待つ条件 | なし |
| 7 | 詳細パネルの説明の区画(表示・編集・保存・同じ行のトースト) | 3・5・6 |
| 8 | 作成ダイアログの説明の欄とテンプレートの説明の当てはめ | 4 |
| 9 | コメントの行の決め方と操作(アイテム・プロジェクト)・API | 2・3 |
| 10 | コメントの区画とメンションのボタン | 5・6・7・9 |
| 11 | 作業時間の行の決め方・検査と操作・API | 2・3・9 |
| 12 | 作業時間の区画 | 7・11 |
| 13 | 関連の規則・操作・候補 API・ブロック中 | 3 |
| 14 | 関連の区画と「ブロック中」の表示(詳細パネル・ツリー) | 7・13 |
| 15 | ウォッチの操作と API・ヘッダーのボタン | 7 |
| 16 | 履歴の文と API(アイテム・プロジェクト、B2-9) | 2 |
| 17 | 履歴の区画(アイテム・プロジェクト) | 7・16 |
| 18 | プロジェクトの概要の編集とリポジトリ(サーバー) | なし |
| 19 | プロジェクトの概要の画面(編集・説明・リポジトリ・コメント・履歴・Epic 0 件) | 7・10・17・18 |
| 20 | 読む用の `item.yaml` に関連・コメント・作業時間を書き出す | 9・11・13 |
| 21 | 概要の Epic の一覧に期限・進捗・ステータス・担当者を出す | 19 |
| 22 | e2e の通しのテストと PR | すべて |

実装は番号の順に 1 つずつ進める(共有のファイルは、先のタスクが作り、あとのタスクが足す)。

## タスク間の名前

各タスクの実装担当は自分のタスクしか読まないので、タスクをまたいで使う名前をここで決める。

### Go

| 名前 | 置き場所 | 形 |
|---|---|---|
| `textmerge.Merge` | `internal/domain/textmerge` | `func Merge(base, older, newer string) (merged string, conflict bool)`。`MergeText(base, older, newer string) string` も残す(`Merge` の文字列だけ) |
| `change.Desc`・`change.Comment`・`change.Worklog` | `internal/domain/change/record.go` | `Type` の `"desc"`・`"comment"`・`"worklog"` |
| `change.NewDesc`・`NewComment`・`NewWorklog` | `internal/domain/change/draft.go` | `NewDesc() Draft`、`NewComment(id ID, from, to any) Draft`(`to` が nil なら削除)、`NewWorklog(id ID, from, to any) Draft` |
| 説明の口 | `internal/app/port/stores.go` | `ItemReader.Description(id item.ID) (string, error)`、`ItemWriter.Describe(id item.ID, text string) error`、`ProjectReader.Description(id project.ID) (string, error)`、`ProjectWriter.Describe(id project.ID, text string) error`。ないアイテムは `ErrNotFound`、説明がなければ `""` |
| `comment.Comment`・`comment.FromLog` | `internal/domain/comment` | `type Comment struct { ID change.ID; By string; At time.Time; Body string; Edited bool }`、`func FromLog(log []change.Record) []Comment`(投稿の古い順、削除済みを除く) |
| `worklog.Entry`・`worklog.FromLog`・`worklog.ParseHours`・`worklog.Format` | `internal/domain/worklog` | `type Entry struct { ID change.ID; By string; Date string; Hundredths int; Memo string }`、`func FromLog(log []change.Record) []Entry`(記録の古い順)、`func ParseHours(s string) (int, error)`(断りは `refusal`)、`func Format(hundredths int) string`(「1.25」) |
| `relation.Label`・`relation.Rows`・`relation.WouldCycle`・`relation.Blocked` | `internal/domain/relation` | `Label` は 9 つの表記(`"blocks"`・`"blocked by"`・…・`"cloned by"`)、`func Rows(self item.ID, all []item.Item) []Row`、`type Row struct { Label Label; Other item.ID }`、`func WouldCycle(from, to item.ID, all []item.Item) bool`、`func Blocked(self item.ID, all []item.Item, closed func(item.Item) bool) bool` |
| `item.Item.Relations` | `internal/domain/item/item.go` | `item` 側に `type Relation struct { Type string \`json:"type"\`; To ID \`json:"to"\` }` を置き、`Relations []Relation` を足す。`relation` パッケージは `item` を import する |
| `history.Row`・`history.ItemRows`・`history.ProjectRows` | `internal/domain/history` | `type Row struct { Who string; Text string; At time.Time }`、引数は ログ・設定・名前を引く関数・隠す `op` の集合(タスク 16 で決める) |
| `project.Project.Repos` | `internal/domain/project` | `Repos []string`、`func NormalizeRepo(s string) (string, error)` |

### HTTP API(`/api/stores/{store}/…`)

| operationId | メソッドとパス | 本文 → 応答 | タスク |
|---|---|---|---|
| `get-item-description` | GET `items/{item}/description` | → `{text}` | 3 |
| `describe-item` | PUT `items/{item}/description` | `{base, text}` → `{text, conflict}` | 3 |
| `get-project-description` | GET `projects/{project}/description` | → `{text}` | 3 |
| `describe-project` | PUT `projects/{project}/description` | `{base, text}` → `{text, conflict}` | 3 |
| `create-item` | 既存 | 本文に `description` を足す | 4 |
| `list-item-comments`・`post-item-comment`・`edit-item-comment`・`delete-item-comment` | GET・POST `items/{item}/comments`、PUT・DELETE `items/{item}/comments/{comment}` | `{body}` → `Comment{id, by, byName, at, body, edited}`。削除は 204 | 9 |
| `list-project-comments`・`post-project-comment`・`edit-project-comment`・`delete-project-comment` | 同じ形で `projects/{project}/comments` | 同上 | 9 |
| `list-worklogs`・`add-worklog`・`delete-worklog` | GET・POST `items/{item}/worklogs`、DELETE `items/{item}/worklogs/{worklog}` | `{hours: string, memo}` → `Worklog{id, by, byName, date, hours: string}`(`hours` は `worklog.Format` の文字列)。削除は 204 | 11 |
| `list-relations` | GET `items/{item}/relations` | → `RelationRow{label, id, display, title, workspaceName, statusName, statusColor}[]`(`workspaceName` は別のワークスペースのときだけ) | 13 |
| `list-relation-candidates` | GET `items/{item}/relation-candidates?label=&q=` | → `RelationCandidate{id, display, title, workspaceName, disabledReason}[]` | 13 |
| `add-relation`・`remove-relation` | POST `items/{item}/relations`、POST `items/{item}/relations/remove` | `{label, other}` → 204 | 13 |
| `set-watch` | PUT `items/{item}/watch` | `{on}` → `Item` | 15 |
| `get-item-history`・`get-project-history` | GET `items/{item}/history`、`projects/{project}/history` | → `HistoryRow{who, text, at}[]`(新しい順) | 16 |
| `rename-project`・`set-project-status`・`set-project-owner`・`set-project-date` | PUT `projects/{project}/title`・`status`・`owner`・`date` | `{title}`・`{status}`・`{owner}`・`{field: "start"\|"end", value}` → `Project` | 18 |
| `add-project-repo`・`remove-project-repo` | POST `projects/{project}/repos`、POST `projects/{project}/repos/remove` | `{repo}` → `Project` | 18 |

`Item` の応答に `blocked: boolean`(タスク 13)、`Project` の応答に `repos: string[]`(タスク 18)を足す。

### 画面

| 名前 | 置き場所 | 形 |
|---|---|---|
| `NotesPort` | `web/packages/core/src/port/notes.ts` | 上の API を 1 メソッドずつ持つ口。対象は `NoteTarget = { kind: "item" \| "project"; storeId: string; id: string }`。各タスクが自分のメソッドを足す。api の実装は `web/packages/api/src/notes.ts` |
| `tokenize`・`Block`・`Inline` | `web/packages/core/src/features/markdown/markdown.ts` | `tokenize(text: string, isMember: (login: string) => boolean): Block[]` |
| `Markdown.svelte` | `web/packages/ui/src/patterns/` | props `{ blocks: Block[] }` |
| `createDrafts`・`Drafts` | `web/packages/core/src/features/drafts/drafts.ts` | `register(key: string, dirty: () => boolean, held?: () => boolean): () => void`(解除の関数を返す。`held` を省くと `dirty` と同じ)、`anyHeld(): boolean`(空でない書きかけがあるか。反映を待つ条件)、`anyDirty(): boolean`(編集を始めたときと違う入力があるか。破棄の確認)、`confirmLeave(key?: string): Promise<boolean>`(違う入力があれば確認ダイアログを出し、「破棄」なら true。`key` を渡すとその入力欄だけを見る) |
| `createDescriptionEditor` | `web/packages/core/src/features/notes/description.ts` | 状態 `{ text, editing, input, base }`、`start()`・`cancel()`・`save()`(同じ行のときトーストの文言を返す) |
| `DescriptionSection.svelte`・`CommentsSection.svelte`・`HistorySection.svelte` | `web/app/src/screens/notes/` | props `{ target: NoteTarget; readonly: boolean }`。アイテムの詳細パネルとプロジェクトの概要の両方で使う |
| `WorklogSection.svelte`・`RelationsSection.svelte`・`WatchButton.svelte` | `web/app/src/screens/item/` | アイテムだけ |

---
## 共有のものの作り手と、上の表から変えた名前

この節は、各タスクの本文より優先する。各タスクの本文は別々に書いたので、食い違ったときはこの節に従う。

| もの | 作るタスク | あとのタスク |
|---|---|---|
| `internal/adapter/httpapi/notes`(`Services`・`Register`・`routes.go` の呼び出し・テストの `newAPI`) | 3 | 9・11・13・16 がフィールドと登録の 1 行を足す |
| `change.Desc` / `change.Comment` と `change.Current`・`First`・`Absent` / `change.Worklog` | 2 / 9 / 11 | — |
| `web/packages/core/src/notesIndex.ts`(A4a の core の export。`index.ts` は 298 行なので、末尾の `export * from "./notesIndex";` だけを足す) | 5 | 6〜19 が export を足す |
| `drafts`(`register(key, dirty, held?)`・`confirmLeave(key?)`。key は `description:` か `comment:` で始め、確認の本文はこの接頭辞で決まる)と `wire.ts` の `drafts` | 6 | 7・10・19 |
| `NotesPort`・`NoteTarget`(`core/src/port/notes.ts`)、偽物 `fakeNotesPort`(`features/notes/fixtures.test-util.ts`)、api の `createNotesPort`(`api/src/notes.ts`)、`wire.ts` の `notes`(`export const notes = createNotesPort();`) | 7 | 9〜19 がメソッドと偽物を同じコミットで足す |
| ui の `Textarea.svelte`(`ui/primitives`) | 7 | 8・10・19 が使う。足りない props はそのタスクが足す |
| ui の `Markdown.svelte` と、ui 側の型 `MarkdownBlock`(ui は core を import できない) | 5 | 7・10 |

上の「タスク間の名前」から変えたもの:

- `set-watch` は `Item` を返すので `internal/adapter/httpapi/items/watch.go` に置く(`notes` ではない)。
- 作業時間の応答は `Worklog{id, by, byName, date, hours, memo}`(`memo` を足した)。
- 関連の候補の型は `RelationCandidate`、追加と解除の本文は `RelationInput`。
- `HistorySection.svelte` の props は `{ target, refresh }`。`refresh` が変わるたびに読み直す。詳細パネルは、アイテムと説明の保存回数を渡す。
- `item.Item.Blocked` は保存しない派生の値で、`app/items` の `Get`・`List`・`FindByNumber` が埋める。
- `project.Project.Repos` を足すので、`Project` を `==` で比べているテスト 5 か所を `reflect.DeepEqual` にする(Task 18)。

画面の名前: 区画の名前、入力欄とボタンの名前は、各タスクの本文のとおりに作る。Task 22 の冒頭の表は先に推測で書いたものなので、Task 22 が実際に作られた名前に合わせて直す(前のタスクを表に合わせて作り直さない)。ただし `data-mention="{小文字のログイン名}"` は Task 5 の `Markdown.svelte` がメンションの要素に付ける(本文のテストは `class="mention"` だけを見ているので、属性とテストを足す)。

計画のコードの確かめ具合: Task 1〜4・9〜12・15 の Go と core の TS は、書き手が作業用の複製でテストを通した。Task 5 の `tokenize` も通した。Task 13・14・16・17(書き直した履歴の決め方を含む)・18・19・20・21・22 と、すべての Svelte のファイルは一度も走らせていない。そこでは仕様とこの節・「タスク間の名前」の表が正で、計画のコードは下書きとして扱う。計画から変えたことは、最後に「実装で計画から変えたこと」に書く。

ほかに、各タスクの本文に足すこと:

- Task 3: プロジェクトの説明のアクティビティ「{タイトル} の説明を変更」のタイトルは、Task 18 の `label` と同じく、空白と改行を 1 つの空白にまとめてから書く(`Operation.Check` は複数行の Summary を断る)。
- Task 4: `create-item` の本文の `description` は省略できる(`omitempty`)。Task 8 は空のとき送らない。
- Task 22: 概要の Epic の行(Task 21)の確認を足す。進捗の「100%」、棒の title「1 / 1」、ステータスの変更。

同期と別のタブの反映(`wire.ts` の `reflect()`)では、コメント・作業時間・関連・履歴の区画も読み直す(Task 10・12・14・17 がそれぞれ足す)。

---

### Task 1: 行マージを domain の `textmerge` に移し、両方を残したかを返す

**Files:**
- Move: `internal/infra/gitstore/filemerge/text.go` → `internal/domain/textmerge/merge.go`
- Move: `internal/infra/gitstore/filemerge/text_test.go` → `internal/domain/textmerge/merge_test.go`
- Modify: `internal/domain/textmerge/merge.go`(`Merge` を足す)
- Modify: `internal/infra/gitstore/sync_tree_more.go`(import と呼び出しの名前だけ)
- Test: `internal/domain/textmerge/conflict_test.go`(新規)

`internal/infra/gitstore/filemerge` には `log.go`・`log_test.go` が残る(ログの行の合わせ方。この計画では触らない)。

**Interfaces:**
- Consumes: なし
- Produces:
  - `func textmerge.Merge(base, older, newer string) (merged string, conflict bool)`(同じ箇所を両方が変えて、両方を残したときだけ `conflict` が true)
  - `func textmerge.MergeText(base, older, newer string) string`(`Merge` の文字列だけ。gitstore の同期が使う)

- [ ] **Step 1: ファイルを移し、パッケージ名を変える(まだ振る舞いは変えない)**

```bash
mkdir internal/domain/textmerge
git mv internal/infra/gitstore/filemerge/text.go internal/domain/textmerge/merge.go
git mv internal/infra/gitstore/filemerge/text_test.go internal/domain/textmerge/merge_test.go
sed -i 's/^package filemerge/package textmerge/' internal/domain/textmerge/merge.go internal/domain/textmerge/merge_test.go
```

`internal/domain/textmerge/merge.go` の先頭(`package` の行の前)に、パッケージのコメントを足す。

```go
// Package textmerge は、説明(description.md)の 3 方向の行マージの規則を持つ。入出力をしない。
// 保存(app)と同期(infra)が同じ関数を使う(06「同時に編集したとき」)。
package textmerge
```

`internal/infra/gitstore/sync_tree_more.go` の import と呼び出しを替える(import は並びの順で `project` と `workspace` のあいだに入れる)。

```go
// 変更前
	"github.com/kukv/hedgehug/internal/domain/workspace"
	"github.com/kukv/hedgehug/internal/infra/gitstore/filemerge"
// 変更後
	"github.com/kukv/hedgehug/internal/domain/textmerge"
	"github.com/kukv/hedgehug/internal/domain/workspace"
```

```go
// 変更前
		return filemerge.MergeText(string(base[p]), textA, textB)
	}
	return filemerge.MergeText(string(base[p]), textB, textA)
// 変更後
		return textmerge.MergeText(string(base[p]), textA, textB)
	}
	return textmerge.MergeText(string(base[p]), textB, textA)
```

Run: `go test ./internal/domain/textmerge/ ./internal/infra/gitstore/...`
Expected: PASS(移しただけ。`filemerge` には `log_test.go` だけが残る)

- [ ] **Step 2: 失敗するテストを書く**

`internal/domain/textmerge/conflict_test.go`(新規)

```go
package textmerge

import "testing"

func TestMergeReportsWhetherBothVersionsWereKept(t *testing.T) {
	cases := []struct {
		name, base, older, newer, want string
		conflict                       bool
	}{
		{"nothing changed", "a\nb\n", "a\nb\n", "a\nb\n", "a\nb\n", false},
		{"only the newer changed", "a\nb\n", "a\nb\n", "a\nB\n", "a\nB\n", false},
		{"only the older changed", "a\nb\n", "A\nb\n", "a\nb\n", "A\nb\n", false},
		{"both changed to the same", "a\nb\n", "a\nX\n", "a\nX\n", "a\nX\n", false},
		{"different lines", "a\nb\nc\n", "A\nb\nc\n", "a\nb\nC\n", "A\nb\nC\n", false},
		{"the same line", "a\nb\nc\n", "a\nX\nc\n", "a\nY\nc\n", "a\nX\n-----\nY\nc\n", true},
		{"both insert at the same position", "a\nb\n", "a\nx\nb\n", "a\ny\nb\n", "a\nx\n-----\ny\nb\n", true},
		{"both wrote to an empty description", "", "x\n", "y\n", "x\n-----\ny\n", true},
	}
	for _, c := range cases {
		got, conflict := Merge(c.base, c.older, c.newer)
		if got != c.want || conflict != c.conflict {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", c.name, got, conflict, c.want, c.conflict)
		}
		if text := MergeText(c.base, c.older, c.newer); text != got {
			t.Errorf("%s: MergeText = %q, want %q", c.name, text, got)
		}
	}
}
```

- [ ] **Step 3: 失敗を確かめる**

Run: `go test ./internal/domain/textmerge/ -run TestMergeReportsWhetherBothVersionsWereKept`
Expected: FAIL(`undefined: Merge`。コンパイルが通らない)

- [ ] **Step 4: 実装する**

`internal/domain/textmerge/merge.go` の `MergeText` を、次の `MergeText` と `Merge` に替える(関数の前のコメントごと)。

```go
// 変更前
// MergeText は、base から older と newer に分かれた説明を、行ごとに合わせる。別々の行の変更は両方残す。
// 同じ箇所を両方が変えたときは、older の版、「-----」、newer の版の順に両方を残す。
func MergeText(base, older, newer string) string {
	switch {
	case older == newer, newer == base:
		return older
	case older == base:
		return newer
	}
	b, o, n := strings.Split(base, "\n"), strings.Split(older, "\n"), strings.Split(newer, "\n")
	hunks := slices.Concat(diffHunks(b, o, true), diffHunks(b, n, false))
	slices.SortStableFunc(hunks, func(x, y hunk) int {
		if c := x.start - y.start; c != 0 {
			return c
		}
		return x.end - y.end
	})
	var out []string
	pos := 0
	for _, c := range clusters(hunks) {
		out = append(out, b[pos:c.start]...)
		out = append(out, mergeChunk(b[c.start:c.end], c.apply(b, true), c.apply(b, false))...)
		pos = c.end
	}
	out = append(out, b[pos:]...)
	return strings.Join(out, "\n")
}
```

```go
// 変更後
// MergeText は、Merge の合わせた文字列だけを返す。
func MergeText(base, older, newer string) string {
	merged, _ := Merge(base, older, newer)
	return merged
}

// Merge は、base から older と newer に分かれた説明を、行ごとに合わせる。別々の行の変更は両方残す。
// 同じ箇所を両方が変えたときは、older の版、「-----」、newer の版の順に両方を残し、conflict を true にする。
func Merge(base, older, newer string) (merged string, conflict bool) {
	switch {
	case older == newer, newer == base:
		return older, false
	case older == base:
		return newer, false
	}
	b, o, n := strings.Split(base, "\n"), strings.Split(older, "\n"), strings.Split(newer, "\n")
	hunks := slices.Concat(diffHunks(b, o, true), diffHunks(b, n, false))
	slices.SortStableFunc(hunks, func(x, y hunk) int {
		if c := x.start - y.start; c != 0 {
			return c
		}
		return x.end - y.end
	})
	var out []string
	pos := 0
	for _, c := range clusters(hunks) {
		out = append(out, b[pos:c.start]...)
		chunk, both := mergeChunk(b[c.start:c.end], c.apply(b, true), c.apply(b, false))
		out = append(out, chunk...)
		conflict = conflict || both
		pos = c.end
	}
	out = append(out, b[pos:]...)
	return strings.Join(out, "\n"), conflict
}
```

同じファイルの `mergeChunk` を、両方を残したかも返す形にする。

```go
// 変更前
// mergeChunk は、安定な行のあいだの 1 区間を合わせる。
func mergeChunk(base, o, n []string) []string {
	switch {
	case slices.Equal(o, n), slices.Equal(n, base):
		return o
	case slices.Equal(o, base):
		return n
	}
	// 説明のなかった末尾に両方が書いたときなど、空の行だけの版は無視しない。
	return slices.Concat(o, []string{conflictSeparator}, n)
}
```

```go
// 変更後
// mergeChunk は、安定な行のあいだの 1 区間を合わせる。両方を残したときは true。
func mergeChunk(base, o, n []string) ([]string, bool) {
	switch {
	case slices.Equal(o, n), slices.Equal(n, base):
		return o, false
	case slices.Equal(o, base):
		return n, false
	}
	// 説明のなかった末尾に両方が書いたときなど、空の行だけの版は無視しない。
	return slices.Concat(o, []string{conflictSeparator}, n), true
}
```

- [ ] **Step 5: 通ることを確かめる**

Run: `go test ./internal/domain/textmerge/ ./internal/infra/gitstore/... && make lint`
Expected: PASS、`0 issues.`(`Merge` は 60 行以内、認知的複雑度は 15 以下)

- [ ] **Step 6: コミット**

```bash
git add internal/domain/textmerge internal/infra/gitstore/filemerge internal/infra/gitstore/sync_tree_more.go
git commit -m "refactor(domain): 説明の行マージを textmerge に移し、両方を残したかを返す(A4a-2)

保存(app)と同期(infra)が同じ規則を使うため、gitstore/filemerge から domain に移した。
Merge は、同じ箇所を両方が変えて両方を残したときだけ conflict を true にする。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 2: 説明の読み書きの口(port・偽物・gitstore・照合テスト)と `desc` の行

**Files:**
- Modify: `internal/domain/change/record.go`(`Desc`・`Comment`・`Worklog` の種類)
- Modify: `internal/domain/change/draft.go`(`NewDesc`・`NewComment`・`NewWorklog`)
- Modify: `internal/app/port/stores.go`(説明の読み書きの口)
- Modify: `internal/app/apptest/stores.go`・`internal/app/apptest/stores_read.go`(偽物)
- Modify: `internal/infra/gitstore/files.go`・`read.go`・`write.go`・`commit.go`(本物)
- Modify: `internal/infra/gitstore/sync_tree_more.go`(自前の `descType` をやめて `change.Desc` を使う)
- Modify: `internal/app/apptest/stores_contract.go`・`stores_contract_values.go`(照合テスト)
- Test: `internal/domain/change/draft_test.go`(新規)、`internal/infra/gitstore/description_test.go`(新規)

大きさの注意: `internal/infra/gitstore`・`internal/app/apptest`・`internal/app/workspaces` は、テストを除くファイルがすでに 20 個(上限)で、新しいファイルを足せない。この計画は既存のファイルに足す(どれも 300 行以内に収まる)。

**Interfaces:**
- Consumes: なし(Task 1 とは独立。タスクの一覧では 1 のあとだが、このタスクは `textmerge` を使わない)
- Produces:
  - `change.Desc`・`change.Comment`・`change.Worklog`(`change.Type`)
  - `func change.NewDesc() change.Draft`、`func change.NewComment(id change.ID, from, to any) change.Draft`、`func change.NewWorklog(id change.ID, from, to any) change.Draft`(`to` が nil なら削除の行。`to` は `null` で書く)
  - `port.ItemReader.Description(id item.ID) (string, error)`、`port.ProjectReader.Description(id project.ID) (string, error)`
  - `port.ItemWriter.Describe(id item.ID, text string) error`、`port.ProjectWriter.Describe(id project.ID, text string) error`
  - 決まり: 説明がなければ `""`。ログのないアイテム(プロジェクト)の `Description` と `Describe` は `port.ErrNotFound`、`change.ValidID` でない ID の `Describe` は `port.ErrInvalidOperation` を包んだエラー(`Description` は `ErrNotFound`)。`Describe` は同じ `Write` の中の読み取りに見える。保存してある説明と同じ本文は何もしない。空の本文は説明のファイルを消す。説明だけを書き換える `Write` も、行を書く `Write` と同じく `op.Check()` を通し、1 回の保存(1 コミット)になる。失敗した `Write` は説明も残さない。

- [ ] **Step 1: 失敗するテストを書く(`desc`・`comment`・`worklog` の行)**

`internal/domain/change/draft_test.go`(新規)

```go
package change_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

func TestDescCommentWorklogDrafts(t *testing.T) {
	if d := change.NewDesc(); d.T != change.Desc || d.F != "" || d.From != nil || d.To != nil {
		t.Errorf("NewDesc = %+v", d)
	}
	c := change.NewComment("C1", nil, map[string]string{"body": "確認"})
	if c.T != change.Comment || c.F != "C1" || string(c.From) != "null" || string(c.To) != `{"body":"確認"}` {
		t.Errorf("NewComment = %+v", c)
	}
	del := change.NewWorklog("W1", map[string]any{"hours": 100}, nil)
	if del.T != change.Worklog || del.F != "W1" || string(del.To) != "null" {
		t.Errorf("NewWorklog (delete) = %+v", del)
	}
	// desc の行は f・from・to を持たず、削除の to は null のまま書いて読み戻せる。
	n := 0
	meta := change.Meta{Op: "o", At: time.Date(2026, 11, 13, 9, 0, 0, 0, jst), Actor: change.User("taro")}
	recs := change.Stamp(nil, []change.Draft{change.NewDesc(), del}, meta, func() change.ID { n++; return change.ID("r" + string(rune('0'+n))) })
	want := []string{
		`{"id":"r1","op":"o","at":"2026-11-13T09:00:00.000+09:00","by":"taro","actor":"user","t":"desc","prev":[]}`,
		`{"id":"r2","op":"o","at":"2026-11-13T09:00:00.000+09:00","by":"taro","actor":"user","t":"worklog","f":"W1","from":{"hours":100},"to":null,"prev":[]}`,
	}
	for i, r := range recs {
		b, err := json.Marshal(r)
		if err != nil || string(b) != want[i] {
			t.Fatalf("line %d = %s, %v\nwant %s", i, b, err, want[i])
		}
		var back change.Record
		if err := json.Unmarshal(b, &back); err != nil || back.T != r.T || string(back.To) != string(r.To) {
			t.Errorf("round trip %d: %+v, %v", i, back, err)
		}
	}
}
```

(`jst` は同じパッケージの `record_test.go` にある。)

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/change/ -run TestDescCommentWorklogDrafts`
Expected: FAIL(`undefined: change.Desc` ほか。コンパイルが通らない)

- [ ] **Step 3: 実装する**

`internal/domain/change/record.go`

```go
// 変更前
// Type は変更の種類(保存形式「変更記録」の t)。下の 5 つ(delete を含む)を扱い、ほかの種類はトラックが足す。
type Type string

const (
	Create Type = "create"
	Set    Type = "set"
	Add    Type = "add"
	Remove Type = "remove"
	// Delete は削除。f は消すもの(値なら "statuses/<ID>"、プロパティなら "properties/<ID>")。
	Delete Type = "delete"
)
```

```go
// 変更後
// Type は変更の種類(保存形式「変更記録」の t)。下の種類を扱い、ほかの種類はトラックが足す。
type Type string

const (
	Create Type = "create"
	Set    Type = "set"
	Add    Type = "add"
	Remove Type = "remove"
	// Delete は削除。f は消すもの(値なら "statuses/<ID>"、プロパティなら "properties/<ID>")。
	Delete Type = "delete"
	// Desc は説明を保存した(本文は description.md。f も from も to も持たない)。
	Desc Type = "desc"
	// Comment・Worklog は、コメントと作業時間の追加・編集・削除。f は内部 ID、削除は to が null。
	Comment Type = "comment"
	Worklog Type = "worklog"
)
```

`internal/domain/change/draft.go` の末尾に足す。

```go
// NewDesc は、説明を保存した記録。本文は Describe で別に書く(description.md)。
func NewDesc() Draft { return Draft{T: Desc} }

// NewComment は、コメント id の追加・編集・削除。to が nil なら削除(to は null で書く)。
func NewComment(id ID, from, to any) Draft {
	return Draft{T: Comment, F: string(id), From: Value(from), To: Value(to)}
}

// NewWorklog は、作業時間の記録 id の追加・編集・削除。to が nil なら削除。
func NewWorklog(id ID, from, to any) Draft {
	return Draft{T: Worklog, F: string(id), From: Value(from), To: Value(to)}
}
```

`internal/infra/gitstore/sync_tree_more.go` は、自前の `descType` をやめて `change.Desc` を使う。

```go
// 変更前
// descType は、説明を保存した変更記録の種類(保存形式「変更記録」の desc)。型はトラック A が足す。
const descType change.Type = "desc"

// mergeDescription は
```

```go
// 変更後
// mergeDescription は
```

```go
// 変更前
		if r.T == descType && !inBase[r.ID] && r.At.After(last) {
// 変更後
		if r.T == change.Desc && !inBase[r.ID] && r.At.After(last) {
```

Run: `go test ./internal/domain/change/ ./internal/infra/gitstore/`
Expected: PASS(`Desc` を足しただけなので、同期の説明の合わせ方のテストはそのまま通る)

- [ ] **Step 4: コミット**

```bash
git add internal/domain/change internal/infra/gitstore/sync_tree_more.go
git commit -m "feat(domain): desc・comment・worklog の変更記録の種類と行の作り方を足す(A4a)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 5: 失敗する照合テストを書く(口・偽物・本物で同じ振る舞い)**

`internal/app/apptest/stores_contract.go` の `cases` に 3 行足す。

```go
// 変更前
		{"item values and deletes round trip", contractItemValues},
	}
// 変更後
		{"item values and deletes round trip", contractItemValues},
		{"descriptions round trip", contractDescription},
		{"a failed write leaves the description", contractDescriptionRollback},
		{"describing needs an existing entity and a summary", contractDescriptionRules},
	}
```

`internal/app/apptest/stores_contract_values.go` の末尾に足す。

```go
// wantDescription は、説明の読み取りの結果が、エラーなしで want であることを確かめる。
func wantDescription(t *testing.T, label, got string, err error, want string) {
	t.Helper()
	if err != nil || got != want {
		t.Errorf("%s = %q, %v; want %q", label, got, err, want)
	}
}

// contractDescription は、説明(アイテムとプロジェクト)が、書いたとおりに読め、空にすると消えることを確かめる。
func contractDescription(t *testing.T, s port.Stores, id port.StoreID) {
	const body = "## 手順\n1. 確認\n"
	mustWrite(t, s, id, taro, writeFixture)
	mustRead(t, s, id, func(r port.Reader) error {
		got, err := r.Items().Description(cItem.ID)
		wantDescription(t, "a new item's description", got, err, "")
		return nil
	})
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		if err := tx.WriteItems().Describe(cItem.ID, body); err != nil {
			return err
		}
		got, err := tx.Items().Description(cItem.ID)
		wantDescription(t, "a tx must see its own description", got, err, body)
		return errors.Join(
			tx.WriteItems().Append(cItem.ID, change.NewDesc()),
			tx.WriteProjects().Describe(cProject.ID, "概要"),
			tx.WriteProjects().Append(cProject.ID, change.NewDesc()),
		)
	})
	mustRead(t, s, id, func(r port.Reader) error {
		got, err := r.Items().Description(cItem.ID)
		wantDescription(t, "item description", got, err, body)
		got, err = r.Projects().Description(cProject.ID)
		wantDescription(t, "project description", got, err, "概要")
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: cItem.ID})
		if err != nil || log[len(log)-1].T != change.Desc {
			t.Errorf("item log = %+v, %v", log, err)
		}
		return nil
	})
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		return errors.Join(tx.WriteItems().Describe(cItem.ID, ""), tx.WriteItems().Append(cItem.ID, change.NewDesc()))
	})
	mustRead(t, s, id, func(r port.Reader) error {
		got, err := r.Items().Description(cItem.ID)
		wantDescription(t, "an emptied description", got, err, "")
		got, err = r.Projects().Description(cProject.ID)
		wantDescription(t, "the other description must stay", got, err, "概要")
		return nil
	})
}

// contractDescriptionRollback は、失敗した Write が説明を残さず、同じ Write の中で書いて戻した説明も残さないことを確かめる。
func contractDescriptionRollback(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	boom := errors.New("boom")
	err := s.Write(t.Context(), id, taro, func(tx port.Tx) error {
		return errors.Join(tx.WriteItems().Describe(cItem.ID, "消える"), boom)
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Write = %v", err)
	}
	// 書いてから同じ説明(空)に戻した Write は、何も残さず、Summary も確かめない。
	mustWrite(t, s, id, port.Operation{Actor: taro.Actor}, func(tx port.Tx) error {
		return errors.Join(tx.WriteItems().Describe(cItem.ID, "一時"), tx.WriteItems().Describe(cItem.ID, ""))
	})
	mustRead(t, s, id, func(r port.Reader) error {
		got, err := r.Items().Description(cItem.ID)
		wantDescription(t, "description", got, err, "")
		return nil
	})
}

// contractDescriptionRules は、ないアイテムと使えない ID、Summary のない説明の書き換えを断ることを確かめる。
func contractDescriptionRules(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	const missing = "01CONTRACTNO00000000000001"
	mustRead(t, s, id, func(r port.Reader) error {
		if _, err := r.Items().Description(missing); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("unknown item: %v", err)
		}
		if _, err := r.Projects().Description("../x"); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("malformed id: %v", err)
		}
		return nil
	})
	err := s.Write(t.Context(), id, taro, func(tx port.Tx) error { return tx.WriteItems().Describe(missing, "x") })
	if !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Describe unknown = %v", err)
	}
	err = s.Write(t.Context(), id, taro, func(tx port.Tx) error { return tx.WriteItems().Describe("../x", "x") })
	if !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("Describe malformed = %v", err)
	}
	err = s.Write(t.Context(), id, port.Operation{Actor: taro.Actor}, func(tx port.Tx) error {
		return tx.WriteItems().Describe(cItem.ID, "要約なし")
	})
	if !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("Describe without a summary = %v", err)
	}
	mustRead(t, s, id, func(r port.Reader) error {
		got, err := r.Items().Description(cItem.ID)
		wantDescription(t, "a refused write must leave the description", got, err, "")
		return nil
	})
}
```

`internal/infra/gitstore/description_test.go`(新規。本物だけの確かめ: `description.md` が同じコミットに入る・空にすると消える・コミットに失敗すると戻る)

```go
package gitstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

func describeTask(t *testing.T, s *Stores, id port.StoreID, text string) error {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "HH-2 の説明を変更"}
	return s.Write(t.Context(), id, op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Describe(apptest.SampleTask, text),
			tx.WriteItems().Append(apptest.SampleTask, change.NewDesc()),
		)
	})
}

func TestDescribeWritesDescriptionMdInTheSameCommit(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	if err := describeTask(t, s, id, "## 手順\n"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "items", string(apptest.SampleTask), "description.md")
	if b, err := os.ReadFile(file); err != nil || string(b) != "## 手順\n" {
		t.Errorf("description.md = %q, %v", b, err)
	}
	if st := worktreeStatus(t, dir); !st.IsClean() {
		t.Errorf("worktree after describing:\n%s", st)
	}
	if n := headCount(t, dir); n != 3 {
		t.Errorf("commits = %d, want 3 (store.yaml, sample, describe)", n)
	}
	// 空にするとファイルを消し、同じコミットに含める。
	if err := describeTask(t, s, id, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("description.md must be removed: %v", err)
	}
	if st := worktreeStatus(t, dir); !st.IsClean() || headCount(t, dir) != 4 {
		t.Errorf("after emptying: status\n%s\ncommits = %d", st, headCount(t, dir))
	}
}

func TestDescribeFailedCommitLeavesTheOldDescription(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	if err := describeTask(t, s, id, "古い\n"); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	s.beforeCommit = func() error { return boom }
	if err := describeTask(t, s, id, "新しい\n"); !errors.Is(err, boom) {
		t.Fatalf("Write error = %v", err)
	}
	file := filepath.Join(dir, "items", string(apptest.SampleTask), "description.md")
	if b, _ := os.ReadFile(file); string(b) != "古い\n" {
		t.Errorf("description.md = %q", b)
	}
	if st := worktreeStatus(t, dir); !st.IsClean() {
		t.Errorf("worktree after a failed commit:\n%s", st)
	}
	// 消す途中で失敗しても、消した説明が戻る。
	if err := describeTask(t, s, id, ""); !errors.Is(err, boom) {
		t.Fatalf("Write error = %v", err)
	}
	if b, _ := os.ReadFile(file); string(b) != "古い\n" {
		t.Errorf("description.md after a failed removal = %q", b)
	}
}
```

- [ ] **Step 6: 失敗を確かめる**

Run: `go test ./internal/app/apptest/ ./internal/infra/gitstore/ -run 'Contract|FakeStores|Describe'`
Expected: FAIL(`r.Items().Description undefined`・`tx.WriteItems().Describe undefined`。コンパイルが通らない)

- [ ] **Step 7: 口(port)を足す**

`internal/app/port/stores.go`

```go
// 変更前
	// 行を 1 つ以上書くときに op.Check がエラーなら、何も残さずそのエラーを返す。行のない Write は
	// 何も残さず、op を確かめない(Summary が空でもよい)。
// 変更後
	// 行を 1 つ以上書くとき、または説明を書き換えるときに op.Check がエラーなら、何も残さずそのエラーを返す。
	// 行も説明の書き換えもない Write は何も残さず、op を確かめない(Summary が空でもよい)。
```

```go
// 変更前
type ProjectReader interface {
	Get(id project.ID) (project.Project, error)
	// List は、ワークスペースのプロジェクトを、並び順の値の順(同じなら内部 ID の順)で返す。
	List(ws workspace.ID) ([]project.Project, error)
}

type ItemReader interface {
	Get(id item.ID) (item.Item, error)
	// List は、プロジェクトのすべてのアイテムを、番号の順で返す。
	List(p project.ID) ([]item.Item, error)
}
```

```go
// 変更後
type ProjectReader interface {
	Get(id project.ID) (project.Project, error)
	// List は、ワークスペースのプロジェクトを、並び順の値の順(同じなら内部 ID の順)で返す。
	List(ws workspace.ID) ([]project.Project, error)
	// Description は、プロジェクトの説明の本文を返す。アイテムと同じ決まり。
	Description(id project.ID) (string, error)
}

type ItemReader interface {
	Get(id item.ID) (item.Item, error)
	// List は、プロジェクトのすべてのアイテムを、番号の順で返す。
	List(p project.ID) ([]item.Item, error)
	// Description は、アイテムの説明の本文を返す(description.md)。説明がなければ ""。
	// アイテムがなければ ErrNotFound。同じ Tx で書いた説明を含む。
	Description(id item.ID) (string, error)
}
```

```go
// 変更前
type ProjectWriter interface {
	Append(id project.ID, drafts ...change.Draft) error
}

type ItemWriter interface {
	Append(id item.ID, drafts ...change.Draft) error
}
```

```go
// 変更後
type ProjectWriter interface {
	Append(id project.ID, drafts ...change.Draft) error
	// Describe は、説明の本文を書く。空の本文はファイルを消す。いまと同じなら何もしない。
	// 履歴の desc の行は書かない(Append で change.NewDesc を足す)。ないプロジェクトは ErrNotFound、
	// change.ValidID でない ID は ErrInvalidOperation を包んだエラー。ほかの Append と同じ 1 回の Write で残る。
	Describe(id project.ID, text string) error
}

type ItemWriter interface {
	Append(id item.ID, drafts ...change.Draft) error
	// Describe は ProjectWriter.Describe と同じ決まりで、アイテムの説明を書く。
	Describe(id item.ID, text string) error
}
```

- [ ] **Step 8: 偽物(apptest)を直す**

`internal/app/apptest/stores.go`

```go
// 変更前
	logs map[change.Ref][]change.Record
	// writes は、保存が成功した回数(Versions の印)。
// 変更後
	logs map[change.Ref][]change.Record
	// texts は、説明の本文(ない説明は入れない)。
	texts map[change.Ref]string
	// writes は、保存が成功した回数(Versions の印)。
```

```go
// 変更前
	s.stores[id] = &store{logs: map[change.Ref][]change.Record{}}
// 変更後
	s.stores[id] = &store{logs: map[change.Ref][]change.Record{}, texts: map[change.Ref]string{}}
```

```go
// 変更前
	return fn(&view{base: st.logs})
// 変更後
	return fn(&view{base: st.logs, baseTexts: st.texts})
```

```go
// 変更前
		view: view{base: logs, pending: map[change.Ref][]change.Record{}},
// 変更後
		view: view{base: logs, baseTexts: st.texts, pending: map[change.Ref][]change.Record{}, texts: map[change.Ref]string{}},
```

```go
// 変更前
	if len(t.pending) == 0 {
		return nil
	}
	if err := op.Check(); err != nil {
		return err
	}
	for ref, recs := range t.pending {
		logs[ref] = append(logs[ref], recs...)
	}
	st.writes++
// 変更後
	if len(t.pending) == 0 && len(t.texts) == 0 {
		return nil
	}
	if err := op.Check(); err != nil {
		return err
	}
	for ref, recs := range t.pending {
		logs[ref] = append(logs[ref], recs...)
	}
	for ref, text := range t.texts {
		if text == "" {
			delete(st.texts, ref)
		} else {
			st.texts[ref] = text
		}
	}
	st.writes++
```

```go
// 変更前
func (t *tx) WriteWorkspaces() port.WorkspaceWriter { return idWriter{t, change.KindWorkspace} }
// 変更後
// describe は、説明を書く。保存してある説明と同じなら何もしない。
func (t *tx) describe(ref change.Ref, text string) error {
	if !ref.Valid() {
		return fmt.Errorf("%w: id %q", port.ErrInvalidOperation, ref.ID)
	}
	if len(t.log(ref)) == 0 {
		return port.ErrNotFound
	}
	if text == t.baseTexts[ref] {
		delete(t.texts, ref)
		return nil
	}
	t.texts[ref] = text
	return nil
}

func (t *tx) WriteWorkspaces() port.WorkspaceWriter { return idWriter{t, change.KindWorkspace} }
```

```go
// 変更前
type membersWriter struct{ t *tx }
// 変更後
func (w idWriter) Describe(id change.ID, text string) error {
	return w.t.describe(change.Ref{Kind: w.kind, ID: id}, text)
}

type membersWriter struct{ t *tx }
```

`internal/app/apptest/stores_read.go`

```go
// 変更前
type view struct {
	base    map[change.Ref][]change.Record
	pending map[change.Ref][]change.Record
}
// 変更後
type view struct {
	base    map[change.Ref][]change.Record
	pending map[change.Ref][]change.Record
	// baseTexts は保存済みの説明、texts は同じ Tx で書いた説明(ない説明は "")。
	baseTexts map[change.Ref]string
	texts     map[change.Ref]string
}
```

```go
// 変更前
func (v *view) ids(kind change.Kind) []change.ID {
// 変更後
// description は、エンティティの説明を返す。ログがなければ port.ErrNotFound。
func (v *view) description(ref change.Ref) (string, error) {
	if len(v.log(ref)) == 0 {
		return "", port.ErrNotFound
	}
	if text, ok := v.texts[ref]; ok {
		return text, nil
	}
	return v.baseTexts[ref], nil
}

func (v *view) ids(kind change.Kind) []change.ID {
```

```go
// 変更前
type members struct{ v *view }
// 変更後
func (r projects) Description(id project.ID) (string, error) {
	return r.v.description(change.Ref{Kind: change.KindProject, ID: id})
}

func (r items) Description(id item.ID) (string, error) {
	return r.v.description(change.Ref{Kind: change.KindItem, ID: id})
}

type members struct{ v *view }
```

- [ ] **Step 9: 本物(gitstore)を直す**

`internal/infra/gitstore/files.go` の末尾に足す(`errors`・`os`・`filepath`・`change` は import 済み)。

```go
// descriptionPath は、エンティティの説明のファイル(Store の根からの相対パス)。
func descriptionPath(ref change.Ref) (string, error) {
	d, err := dirOf(ref)
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "description.md"), nil
}

// readDescription は、作業ツリーの説明を読む。ファイルがなければ ""。
func readDescription(root string, ref change.Ref) (string, error) {
	p, err := descriptionPath(ref)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(filepath.Join(root, p)) //nolint:gosec // G304: パスは Store の根と dirOf で検査済みの ID から作る
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}
```

`internal/infra/gitstore/read.go`

```go
// 変更前
type view struct {
	root    string
	pending map[change.Ref][]change.Record
}
// 変更後
type view struct {
	root    string
	pending map[change.Ref][]change.Record
	// texts は、同じ Tx で書いた説明(ない説明は "")。いまの作業ツリーと同じ説明は入れない。
	texts map[change.Ref]string
}
```

```go
// 変更前
// get は、ログを読んで from で値にする。作成の行がなければ port.ErrNotFound。
// 変更後
// description は、エンティティの説明を返す。ログがなければ port.ErrNotFound。同じ Tx で書いた説明(texts)を含む。
func (v *view) description(ref change.Ref) (string, error) {
	log, err := v.log(ref)
	if err != nil {
		return "", err
	}
	if len(log) == 0 {
		return "", port.ErrNotFound
	}
	if text, ok := v.texts[ref]; ok {
		return text, nil
	}
	return readDescription(v.root, ref)
}

// get は、ログを読んで from で値にする。作成の行がなければ port.ErrNotFound。
```

```go
// 変更前
type members struct{ v *view }
// 変更後
func (r projects) Description(id project.ID) (string, error) {
	return r.v.description(change.Ref{Kind: change.KindProject, ID: id})
}

func (r items) Description(id item.ID) (string, error) {
	return r.v.description(change.Ref{Kind: change.KindItem, ID: id})
}

type members struct{ v *view }
```

`internal/infra/gitstore/write.go`

```go
// 変更前
		view: view{root: root, pending: map[change.Ref][]change.Record{}},
// 変更後
		view: view{root: root, pending: map[change.Ref][]change.Record{}, texts: map[change.Ref]string{}},
```

```go
// 変更前
	if len(t.pending) == 0 {
		return nil
	}
// 変更後
	if len(t.pending) == 0 && len(t.texts) == 0 {
		return nil
	}
```

```go
// 変更前
func (t *tx) WriteWorkspaces() port.WorkspaceWriter { return idWriter{t, change.KindWorkspace} }
// 変更後
// describe は、説明を書く(コミットのときにファイルにする)。作業ツリーと同じなら何もしない。
func (t *tx) describe(ref change.Ref, text string) error {
	if !ref.Valid() {
		return fmt.Errorf("%w: id %q", port.ErrInvalidOperation, ref.ID)
	}
	log, err := t.log(ref)
	if err != nil {
		return err
	}
	if len(log) == 0 {
		return port.ErrNotFound
	}
	saved, err := readDescription(t.root, ref)
	if err != nil {
		return err
	}
	if text == saved {
		delete(t.texts, ref)
		return nil
	}
	t.texts[ref] = text
	return nil
}

func (t *tx) WriteWorkspaces() port.WorkspaceWriter { return idWriter{t, change.KindWorkspace} }
```

```go
// 変更前
type membersWriter struct{ t *tx }
// 変更後
func (w idWriter) Describe(id change.ID, text string) error {
	return w.t.describe(change.Ref{Kind: w.kind, ID: id}, text)
}

type membersWriter struct{ t *tx }
```

`internal/infra/gitstore/commit.go`: エンティティのファイルを書いたあと(`rewriteStale` の前)に説明を書く。

```go
// 変更前
	if err := rewriteStale(wt, t, sc, stale); err != nil {
		return err
	}
	if err := s.hook(s.beforeCommit); err != nil {
// 変更後
	if err := writeTexts(wt, t); err != nil {
		return err
	}
	if err := rewriteStale(wt, t, sc, stale); err != nil {
		return err
	}
	if err := s.hook(s.beforeCommit); err != nil {
```

```go
// 変更前
// rewriteStale は、設定が変わったワークスペースの、ほかのアイテムの item.yaml を同じコミットで作り直す(B2-19)。
// 変更後
// writeTexts は、この Tx で書いた説明を description.md にする(空なら消す)。エンティティのフォルダを
// 作ったあと(writeEntity のあと)に呼ぶ。
func writeTexts(wt *git.Worktree, t *tx) error {
	refs := make([]change.Ref, 0, len(t.texts))
	for ref := range t.texts {
		refs = append(refs, ref)
	}
	slices.SortFunc(refs, func(a, b change.Ref) int {
		return strings.Compare(string(a.Kind)+"/"+string(a.ID), string(b.Kind)+"/"+string(b.ID))
	})
	for _, ref := range refs {
		p, err := descriptionPath(ref)
		if err != nil {
			return err
		}
		if t.texts[ref] == "" {
			// describe は、作業ツリーと同じ説明を入れないので、空のときはファイルがある。
			if _, err := wt.Remove(p); err != nil {
				return err
			}
			continue
		}
		if err := os.WriteFile(filepath.Join(t.root, p), []byte(t.texts[ref]), 0o644); err != nil { //nolint:gosec // G306: Store のファイルは共有するもので秘密ではない
			return err
		}
		if err := addPaths(wt, p); err != nil {
			return err
		}
	}
	return nil
}

// rewriteStale は、設定が変わったワークスペースの、ほかのアイテムの item.yaml を同じコミットで作り直す(B2-19)。
```

- [ ] **Step 10: 通ることを確かめる**

Run: `go build -tags e2e ./... && go test ./internal/... && make lint`
Expected: PASS、`0 issues.`(偽物の照合 `TestFakeStores`、本物の照合 `TestContract` に 3 つの新しい case が出る。`internal/archtest` も通る)

`go build -tags e2e ./...` は、e2e 用のバイナリが偽物を差し込むので、口の変更で壊れていないことの確認。`make lint` に失敗したら、ファイルの行数・関数の長さ・認知的複雑度の上限を確かめる(`commit.go` は 232 行、`read.go` は 212 行、`apptest/stores.go` は 178 行)。

- [ ] **Step 11: コミット**

```bash
git add internal/app/port internal/app/apptest internal/infra/gitstore
git commit -m "feat(port): 説明の読み書きの口を足し、偽物と gitstore に実装する(A4a-1)

ItemReader・ProjectReader に Description、ItemWriter・ProjectWriter に Describe を足す。
本文は items/<ID>/description.md と projects/<ID>/description.md に置き、空の説明はファイルを消す。
履歴の desc の行は app が change.NewDesc で別に書く。説明だけの書き換えも 1 回の保存(1 コミット)で、
失敗したら説明も戻す。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 3: 説明の保存(アイテム・プロジェクト)と「詳細の付録」の読み取り API

**Files:**
- Create: `internal/app/descriptions/service.go`・`get.go`・`save.go`・`saveitem.go`・`saveproject.go`
- Create: `internal/adapter/httpapi/notes/notes.go`・`descriptions.go`
- Modify: `internal/adapter/httpapi/routes.go`
- Modify(生成): `api/openapi.yaml`、`web/packages/api/src/generated.ts`
- Test: `internal/app/descriptions/descriptions_test.go`、`internal/adapter/httpapi/notes/notes_test.go`・`descriptions_test.go`

**Interfaces:**
- Consumes: `port.ItemReader.Description`・`port.ItemWriter.Describe` ほか(Task 2)、`textmerge.Merge`(Task 1)、`change.NewDesc`(Task 2)
- Produces:
  - `descriptions.Service{Stores port.Stores; Identity port.Identity}`、`descriptions.Result{Text string; Conflict bool}`
  - `func (s Service) GetItem(ctx, store port.StoreID, id item.ID) (string, error)`、`GetProject(ctx, store, id project.ID) (string, error)`
  - `func (s Service) SaveItem(ctx, store port.StoreID, id item.ID, base, text string) (Result, error)`、`SaveProject(ctx, store, id project.ID, base, text string) (Result, error)`
  - `notes.Services{Descriptions appdescriptions.Service}` と `notes.Register(api huma.API, s Services)`。コメント・作業時間・関連・履歴の API を足すタスク(9・11・13・16)は、`Services` に自分の usecase を足し、`Register` に `registerXxx(api, s.Xxx)` を 1 行足す。
  - HTTP: `get-item-description`(GET `/api/stores/{store}/items/{item}/description` → `{text}`)、`describe-item`(PUT 同じパス、`{base, text}` → `{text, conflict}`)、`get-project-description`・`describe-project`(`projects/{project}/description`)

**決めたこと(このタスクの中):**
- 保存してあるいまの説明を `older`、入力を `newer`、画面が送る `base` を共通の祖先にして `textmerge.Merge` で合わせる(A4a-3)。`text` が `base` と同じ(画面で何も変えていない)なら、いまの説明をそのまま返して何も保存しない。
- `base` と `text` は、空白と改行だけなら空として扱う(空の説明はファイルを置かない。A4a-1)。

- [ ] **Step 1: 失敗するテストを書く(app)**

`internal/app/descriptions/descriptions_test.go`(新規)

```go
package descriptions_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/descriptions"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

const store port.StoreID = "S1"

// recorder は、Write に渡したアクティビティの 1 行を覚える。
type recorder struct {
	*apptest.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func setup(t *testing.T) (descriptions.Service, *recorder) {
	t.Helper()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore(store)
	if err := apptest.Sample(context.Background(), s, store); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return descriptions.Service{Stores: rec, Identity: apptest.Identity{Login: "hanako"}}, rec
}

func logTypes(t *testing.T, s port.Stores, ref change.Ref) []change.Type {
	t.Helper()
	var out []change.Type
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(ref)
		for _, rec := range log {
			out = append(out, rec.T)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

var taskRef = change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}

func TestSaveItemWritesTheTextAndOneDescRow(t *testing.T) {
	svc, rec := setup(t)
	before := len(logTypes(t, rec, taskRef))
	res, err := svc.SaveItem(t.Context(), store, apptest.SampleTask, "", "## 手順\n1. 確認\n")
	if err != nil || res.Text != "## 手順\n1. 確認\n" || res.Conflict {
		t.Fatalf("SaveItem = %+v, %v", res, err)
	}
	if got, err := svc.GetItem(t.Context(), store, apptest.SampleTask); err != nil || got != "## 手順\n1. 確認\n" {
		t.Errorf("GetItem = %q, %v", got, err)
	}
	types := logTypes(t, rec, taskRef)
	if len(types) != before+1 || types[len(types)-1] != change.Desc {
		t.Errorf("log types = %v", types)
	}
	if last := rec.summaries[len(rec.summaries)-1]; last != "HH-2 の説明を変更" {
		t.Errorf("activity = %q", last)
	}
}

func TestSaveItemSavesNothingWhenTheTextIsTheSame(t *testing.T) {
	svc, rec := setup(t)
	if _, err := svc.SaveItem(t.Context(), store, apptest.SampleTask, "", "本文\n"); err != nil {
		t.Fatal(err)
	}
	before := len(logTypes(t, rec, taskRef))
	res, err := svc.SaveItem(t.Context(), store, apptest.SampleTask, "本文\n", "本文\n")
	if err != nil || res.Text != "本文\n" || res.Conflict || len(logTypes(t, rec, taskRef)) != before {
		t.Errorf("same text: %+v, %v, log %d → %d", res, err, before, len(logTypes(t, rec, taskRef)))
	}
	// 別の人が先に同じ説明にしていても、同じになるなら保存しない。
	res, err = svc.SaveItem(t.Context(), store, apptest.SampleTask, "", "本文\n")
	if err != nil || res.Conflict || len(logTypes(t, rec, taskRef)) != before {
		t.Errorf("already the same: %+v, %v", res, err)
	}
}

func TestSaveItemMergesWithWhatWasSavedMeanwhile(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	if _, err := svc.SaveItem(ctx, store, apptest.SampleTask, "", "a\nb\n"); err != nil {
		t.Fatal(err)
	}
	// 編集を始めたときの説明は "a\nb\n"。そのあいだに別の人が 1 行目を変えた。
	if _, err := svc.SaveItem(ctx, store, apptest.SampleTask, "a\nb\n", "A\nb\n"); err != nil {
		t.Fatal(err)
	}
	// 別の行なら両方が残り、トーストは出さない。
	res, err := svc.SaveItem(ctx, store, apptest.SampleTask, "a\nb\n", "a\nB\n")
	if err != nil || res.Text != "A\nB\n" || res.Conflict {
		t.Fatalf("different lines = %+v, %v", res, err)
	}
	// 片方しか変えていなければ、そちらを取る。
	res, err = svc.SaveItem(ctx, store, apptest.SampleTask, "A\nB\n", "A\nC\n")
	if err != nil || res.Text != "A\nC\n" || res.Conflict {
		t.Fatalf("one side = %+v, %v", res, err)
	}
	// 同じ行なら、保存してあるほうを先にして両方を残す。
	if _, err := svc.SaveItem(ctx, store, apptest.SampleTask, "A\nC\n", "A\nD\n"); err != nil {
		t.Fatal(err)
	}
	res, err = svc.SaveItem(ctx, store, apptest.SampleTask, "A\nC\n", "A\nE\n")
	if err != nil || res.Text != "A\nD\n-----\nE\n" || !res.Conflict {
		t.Fatalf("same line = %+v, %v", res, err)
	}
	if got, _ := svc.GetItem(ctx, store, apptest.SampleTask); got != "A\nD\n-----\nE\n" {
		t.Errorf("saved = %q", got)
	}
}

func TestSaveItemTreatsBlankAsEmptyAndRemovesTheText(t *testing.T) {
	svc, _ := setup(t)
	if _, err := svc.SaveItem(t.Context(), store, apptest.SampleTask, "", "本文\n"); err != nil {
		t.Fatal(err)
	}
	res, err := svc.SaveItem(t.Context(), store, apptest.SampleTask, "本文\n", " \n\n")
	if err != nil || res.Text != "" {
		t.Fatalf("blank = %+v, %v", res, err)
	}
	if got, err := svc.GetItem(t.Context(), store, apptest.SampleTask); err != nil || got != "" {
		t.Errorf("GetItem = %q, %v", got, err)
	}
}

func TestSaveProjectAndGet(t *testing.T) {
	svc, rec := setup(t)
	res, err := svc.SaveProject(t.Context(), store, apptest.SampleProject, "", "ログインの作り直し")
	if err != nil || res.Text != "ログインの作り直し" {
		t.Fatalf("SaveProject = %+v, %v", res, err)
	}
	if got, err := svc.GetProject(t.Context(), store, apptest.SampleProject); err != nil || got != "ログインの作り直し" {
		t.Errorf("GetProject = %q, %v", got, err)
	}
	if last := rec.summaries[len(rec.summaries)-1]; last != "ログイン の説明を変更" {
		t.Errorf("activity = %q", last)
	}
	types := logTypes(t, rec, change.Ref{Kind: change.KindProject, ID: apptest.SampleProject})
	if types[len(types)-1] != change.Desc {
		t.Errorf("project log types = %v", types)
	}
}

func TestUnknownIDsAreNotFound(t *testing.T) {
	svc, _ := setup(t)
	const missing = "01NOPE00000000000000000000"
	if _, err := svc.GetItem(t.Context(), store, missing); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("GetItem = %v", err)
	}
	if _, err := svc.SaveItem(t.Context(), store, missing, "", "x"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SaveItem = %v", err)
	}
	if _, err := svc.GetProject(t.Context(), store, missing); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("GetProject = %v", err)
	}
	if _, err := svc.SaveProject(t.Context(), store, missing, "", "x"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SaveProject = %v", err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/descriptions/`
Expected: FAIL(`no non-test Go files in …/internal/app/descriptions`。パッケージがまだない)

- [ ] **Step 3: app の実装を書く**

`internal/app/descriptions/service.go`(新規)

```go
// Package descriptions は、アイテムとプロジェクトの説明の読み取りと保存(02「説明」)を持つ。1 ファイル 1 操作。
package descriptions

import "github.com/kukv/hedgehug/internal/app/port"

// Service は、説明の操作が使う口。
type Service struct {
	Stores   port.Stores
	Identity port.Identity
}

// Result は、説明の保存の結果。Text は保存したあとの説明、Conflict は、同じ箇所が同時に編集されて
// 両方の版を残したとき true(画面がトーストを出す)。
type Result struct {
	Text     string
	Conflict bool
}
```

`internal/app/descriptions/get.go`(新規)

```go
package descriptions

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// GetItem は、アイテムの説明の本文を返す。説明がなければ ""。アイテムがなければ port.ErrNotFound。
func (s Service) GetItem(ctx context.Context, store port.StoreID, id item.ID) (string, error) {
	var text string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		text, err = r.Items().Description(id)
		return err
	})
	return text, err
}

// GetProject は、プロジェクトの説明の本文を返す。決まりは GetItem と同じ。
func (s Service) GetProject(ctx context.Context, store port.StoreID, id project.ID) (string, error) {
	var text string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		text, err = r.Projects().Description(id)
		return err
	})
	return text, err
}
```

`internal/app/descriptions/save.go`(新規。アイテムとプロジェクトで共通の本体)

```go
package descriptions

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/textmerge"
)

// slot は、アイテムとプロジェクトで違う、説明の読み書き。write は、本文と履歴の desc の行を書く。
type slot struct {
	read  func(r port.Reader) (string, error)
	write func(tx port.Tx, text string) error
}

// save は、保存の中で読んだいまの説明を older、入力を newer にして、編集を始めたときの説明(base)と
// 行ごとに合わせる(A4a-3、06「同時に編集したとき」。保存してあるほうが先)。合わせた結果がいまの
// 説明と同じなら何も保存しない。
func (s Service) save(ctx context.Context, store port.StoreID, summary, base, text string, sl slot) (Result, error) {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return Result{}, err
	}
	op := port.Operation{Actor: change.User(me), Summary: summary}
	var res Result
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		saved, err := sl.read(tx)
		if err != nil {
			return err
		}
		merged, conflict := textmerge.Merge(blankIsEmpty(base), saved, blankIsEmpty(text))
		res = Result{Text: merged, Conflict: conflict}
		if merged == saved {
			return nil // 行のない Write は何も残さない
		}
		return sl.write(tx, merged)
	})
	if err != nil {
		return Result{}, err
	}
	return res, nil
}

// blankIsEmpty は、空白と改行だけの説明を空にする(空の説明はファイルを置かない。A4a-1)。
func blankIsEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return s
}
```

`internal/app/descriptions/saveitem.go`(新規)

```go
package descriptions

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// SaveItem は、アイテムの説明を保存する(02「説明」)。base は編集を始めたときの説明、text は入力。
// アクティビティは「{ID} の説明を変更」、履歴には desc の行が 1 つ残る。
func (s Service) SaveItem(ctx context.Context, store port.StoreID, id item.ID, base, text string) (Result, error) {
	display, err := s.displayID(ctx, store, id)
	if err != nil {
		return Result{}, err
	}
	return s.save(ctx, store, display+" の説明を変更", base, text, slot{
		read: func(r port.Reader) (string, error) { return r.Items().Description(id) },
		write: func(tx port.Tx, merged string) error {
			return errors.Join(tx.WriteItems().Describe(id, merged), tx.WriteItems().Append(id, change.NewDesc()))
		},
	})
}

// displayID は、アクティビティの 1 行に書く、アイテムの画面の ID(HH-12)。保存の前に決めるので別に読む。
func (s Service) displayID(ctx context.Context, store port.StoreID, id item.ID) (string, error) {
	var display string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := r.Items().Get(id)
		if err != nil {
			return err
		}
		w, err := r.Workspaces().Get(it.Place.Workspace)
		if err != nil {
			return err
		}
		display = it.DisplayID(w.Prefix)
		return nil
	})
	return display, err
}
```

`internal/app/descriptions/saveproject.go`(新規)

```go
package descriptions

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// SaveProject は、プロジェクトの説明を保存する。決まりは SaveItem と同じで、
// アクティビティは「{タイトル} の説明を変更」(A4a-18)。
func (s Service) SaveProject(ctx context.Context, store port.StoreID, id project.ID, base, text string) (Result, error) {
	var title string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		p, err := r.Projects().Get(id)
		title = p.Title
		return err
	})
	if err != nil {
		return Result{}, err
	}
	return s.save(ctx, store, title+" の説明を変更", base, text, slot{
		read: func(r port.Reader) (string, error) { return r.Projects().Description(id) },
		write: func(tx port.Tx, merged string) error {
			return errors.Join(tx.WriteProjects().Describe(id, merged), tx.WriteProjects().Append(id, change.NewDesc()))
		},
	})
}
```

- [ ] **Step 4: app のテストが通ることを確かめる**

Run: `go test ./internal/app/descriptions/`
Expected: PASS

- [ ] **Step 5: 失敗するテストを書く(HTTP)**

`internal/adapter/httpapi/notes/notes_test.go`(新規。ほかの区画のタスクが `Services` にフィールドを足すときは、ここの `Register` の呼び出しにも足す)

```go
package notes_test

import (
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/notes"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appdescriptions "github.com/kukv/hedgehug/internal/app/descriptions"
)

func newAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	notes.Register(api, notes.Services{
		Descriptions: appdescriptions.Service{Stores: s, Identity: apptest.Identity{Login: "taro"}},
	})
	return api
}
```

`internal/adapter/httpapi/notes/descriptions_test.go`(新規)

```go
package notes_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
)

const (
	taskDescription    = "/api/stores/S1/items/" + string(apptest.SampleTask) + "/description"
	projectDescription = "/api/stores/S1/projects/" + string(apptest.SampleProject) + "/description"
)

func TestItemDescriptionRoundTripAndConflict(t *testing.T) {
	api := newAPI(t)
	if res := api.Get(taskDescription); res.Code != http.StatusOK || res.Body.String() != `{"text":""}`+"\n" {
		t.Fatalf("empty: %d %s", res.Code, res.Body)
	}
	res := api.Put(taskDescription, map[string]any{"base": "", "text": "a\nb\n"})
	if res.Code != http.StatusOK || res.Body.String() != `{"text":"a\nb\n","conflict":false}`+"\n" {
		t.Fatalf("save: %d %s", res.Code, res.Body)
	}
	if res := api.Get(taskDescription); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"text":"a\nb\n"`) {
		t.Errorf("get: %d %s", res.Code, res.Body)
	}
	// 別の人が 1 行目を変えたあとに、編集を始めたときの説明 "a\nb\n" から 1 行目を変えて保存すると、両方を残す。
	if res := api.Put(taskDescription, map[string]any{"base": "a\nb\n", "text": "X\nb\n"}); res.Code != http.StatusOK {
		t.Fatalf("other: %d %s", res.Code, res.Body)
	}
	res = api.Put(taskDescription, map[string]any{"base": "a\nb\n", "text": "Y\nb\n"})
	if res.Code != http.StatusOK || res.Body.String() != `{"text":"X\n-----\nY\nb\n","conflict":true}`+"\n" {
		t.Errorf("conflict: %d %s", res.Code, res.Body)
	}
}

func TestProjectDescription(t *testing.T) {
	api := newAPI(t)
	res := api.Put(projectDescription, map[string]any{"base": "", "text": "概要"})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"text":"概要"`) {
		t.Fatalf("save: %d %s", res.Code, res.Body)
	}
	if res := api.Get(projectDescription); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"text":"概要"`) {
		t.Errorf("get: %d %s", res.Code, res.Body)
	}
}

func TestDescriptionOfUnknownIsNotFound(t *testing.T) {
	api := newAPI(t)
	const (
		item    = "/api/stores/S1/items/01NOPE00000000000000000000/description"
		project = "/api/stores/S1/projects/01NOPE00000000000000000000/description"
	)
	if res := api.Get(item); res.Code != http.StatusNotFound {
		t.Errorf("get item: %d", res.Code)
	}
	if res := api.Put(item, map[string]any{"base": "", "text": "x"}); res.Code != http.StatusNotFound {
		t.Errorf("put item: %d", res.Code)
	}
	if res := api.Get(project); res.Code != http.StatusNotFound {
		t.Errorf("get project: %d", res.Code)
	}
	if res := api.Put(project, map[string]any{"base": "", "text": "x"}); res.Code != http.StatusNotFound {
		t.Errorf("put project: %d", res.Code)
	}
}

func TestDescribeNeedsBaseAndText(t *testing.T) {
	if res := newAPI(t).Put(taskDescription, map[string]any{"text": "x"}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("without base: %d %s", res.Code, res.Body)
	}
}
```

- [ ] **Step 6: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/notes/`
Expected: FAIL(`no non-test Go files in …/httpapi/notes`)

- [ ] **Step 7: HTTP の実装を書く**

`internal/adapter/httpapi/notes/notes.go`(新規)

```go
// Package notes は、詳細パネルとプロジェクトの概要の付録(説明・コメント・作業時間・関連・履歴)の API を持つ。
// 区画ごとに 1 ファイルで、Register に 1 行ずつ足す。
package notes

import (
	"github.com/danielgtaylor/huma/v2"

	appdescriptions "github.com/kukv/hedgehug/internal/app/descriptions"
)

// Services は、付録の API が使う usecase。区画のタスクが、自分のものを足す。
type Services struct {
	Descriptions appdescriptions.Service
}

// Register は、付録の API を登録する。
func Register(api huma.API, s Services) {
	registerDescriptions(api, s.Descriptions)
}
```

`internal/adapter/httpapi/notes/descriptions.go`(新規。入力の型は、パスのフィールドを各型に書く: 名前のない埋め込みの型だと huma がパスの値を入れない)

```go
package notes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appdescriptions "github.com/kukv/hedgehug/internal/app/descriptions"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// DescriptionText は、説明の本文。
type DescriptionText struct {
	Text string `json:"text" doc:"説明の本文(Markdown)。説明がなければ空"`
}

// DescribeBody は、説明の保存の本文。
type DescribeBody struct {
	Base string `json:"base" doc:"編集を始めたときの説明。保存するときの共通の祖先になる"`
	Text string `json:"text" doc:"入力した説明。空白だけは空として保存する(説明を消す)"`
}

// DescribeResult は、説明の保存の応答。
type DescribeResult struct {
	Text     string `json:"text" doc:"保存したあとの説明"`
	Conflict bool   `json:"conflict" doc:"同じ箇所が同時に編集されて、両方の版を残したとき true"`
}

type itemKey struct {
	Store string `path:"store"`
	Item  string `path:"item"`
}

type projectKey struct {
	Store   string `path:"store"`
	Project string `path:"project"`
}

type describeItemInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  DescribeBody
}

type describeProjectInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Body    DescribeBody
}

type textOutput struct{ Body DescriptionText }

type describeOutput struct{ Body DescribeResult }

func registerDescriptions(api huma.API, svc appdescriptions.Service) {
	huma.Register(api, huma.Operation{OperationID: "get-item-description", Method: http.MethodGet, Path: "/api/stores/{store}/items/{item}/description", Summary: "アイテムの説明を返す"},
		func(ctx context.Context, in *itemKey) (*textOutput, error) {
			text, err := svc.GetItem(ctx, port.StoreID(in.Store), item.ID(in.Item))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &textOutput{Body: DescriptionText{Text: text}}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "describe-item", Method: http.MethodPut, Path: "/api/stores/{store}/items/{item}/description", Summary: "アイテムの説明を保存する"},
		func(ctx context.Context, in *describeItemInput) (*describeOutput, error) {
			res, err := svc.SaveItem(ctx, port.StoreID(in.Store), item.ID(in.Item), in.Body.Base, in.Body.Text)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &describeOutput{Body: DescribeResult{Text: res.Text, Conflict: res.Conflict}}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "get-project-description", Method: http.MethodGet, Path: "/api/stores/{store}/projects/{project}/description", Summary: "プロジェクトの説明を返す"},
		func(ctx context.Context, in *projectKey) (*textOutput, error) {
			text, err := svc.GetProject(ctx, port.StoreID(in.Store), project.ID(in.Project))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &textOutput{Body: DescriptionText{Text: text}}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "describe-project", Method: http.MethodPut, Path: "/api/stores/{store}/projects/{project}/description", Summary: "プロジェクトの説明を保存する"},
		func(ctx context.Context, in *describeProjectInput) (*describeOutput, error) {
			res, err := svc.SaveProject(ctx, port.StoreID(in.Store), project.ID(in.Project), in.Body.Base, in.Body.Text)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &describeOutput{Body: DescribeResult{Text: res.Text, Conflict: res.Conflict}}, nil
		})
}
```

- [ ] **Step 8: ルートに登録し、API の生成物を作り直す**

`internal/adapter/httpapi/routes.go`

```go
// 変更前(import)
	"github.com/kukv/hedgehug/internal/adapter/httpapi/members"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/projects"
// 変更後
	"github.com/kukv/hedgehug/internal/adapter/httpapi/members"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/notes"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/projects"
```

```go
// 変更前(import)
	ghapp "github.com/kukv/hedgehug/internal/app/github"
// 変更後
	appdescriptions "github.com/kukv/hedgehug/internal/app/descriptions"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
```

```go
// 変更前
	workspaces.Register(api, appworkspaces.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity})
	ghapi.Register(api, githubServices(deps))
// 変更後
	workspaces.Register(api, appworkspaces.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity})
	notes.Register(api, notes.Services{
		Descriptions: appdescriptions.Service{Stores: deps.Stores, Identity: deps.Identity},
	})
	ghapi.Register(api, githubServices(deps))
```

Run: `go test ./internal/adapter/httpapi/`
Expected: FAIL(`TestOpenAPIYAMLIsUpToDate`: `api/openapi.yaml is stale; run make generate`。ほかは PASS。新しい API が `api/openapi.yaml` に入っていない)

Run: `make generate && pnpm -C web gen`
Expected: `api/openapi.yaml` に `DescribeBody`・`DescribeResult`・`DescriptionText` と 4 つの operationId(`get-item-description`・`describe-item`・`get-project-description`・`describe-project`)が増える。`web/packages/api/src/generated.ts` に `getItemDescription`・`describeItem`・`getProjectDescription`・`describeProject` と同名の型が増える。

- [ ] **Step 9: 全部通ることを確かめる**

Run: `go test ./internal/... && make lint && pnpm -C web check`
Expected: PASS、`0 issues.`(`pnpm -C web check` は、生成した型で画面の型検査が壊れていないことの確認。画面の口(`NotesPort`)はタスク 7 が足す)

- [ ] **Step 10: コミット**

```bash
git add internal/app/descriptions internal/adapter/httpapi/notes internal/adapter/httpapi/routes.go api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat(app,api): アイテムとプロジェクトの説明を保存し、読み取りの API を足す(A4a-3・A4a-24)

編集を始めたときの説明(base)・保存してある説明・入力を textmerge.Merge で合わせ、
同じ箇所を両方が変えたときは conflict: true で知らせる。同じになる保存は何もしない。
履歴に desc の行、アクティビティに「{ID} の説明を変更」(プロジェクトは「{タイトル} の説明を変更」)を残す。
付録の API は httpapi/notes にまとめ、区画ごとに Register へ足す形にした。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 4: 作成の説明・テンプレートの子の説明・アイテムから作るテンプレートへ写す

**Files:**
- Modify: `internal/app/items/create.go`(`CreateInput.Description`、`writeCreated` が説明を書く)
- Modify: `internal/app/items/createtemplate.go`(子の説明、`descTexts`)
- Modify: `internal/app/workspaces/template_item.go`(アイテムと子の説明を写す)
- Modify: `internal/adapter/httpapi/items/items.go`(作成の本文に `description`)
- Modify(生成): `api/openapi.yaml`、`web/packages/api/src/generated.ts`
- Test: `internal/app/items/description_test.go`(新規)、`internal/app/workspaces/template_description_test.go`(新規)、`internal/adapter/httpapi/items/description_test.go`(新規)

**Interfaces:**
- Consumes: `port.ItemWriter.Describe`・`port.ItemReader.Description`(Task 2)
- Produces:
  - `items.CreateInput.Description string`(もとになるアイテムの説明。空白だけは説明なし)
  - API `create-item` の本文の `description`(省略できる)
  - 子の説明: テンプレートの子(`workspace.TemplateChild.Description`)を、子の作成と同じ保存で書く。Step は説明を持たない。
  - `workspaces.Service.CreateTemplateFromItem` が、アイテムの説明を `Template.Description` に、Epic の直下の子の説明を `TemplateChild.Description` に写す(「子と Step を含める」がオフなら、アイテムの説明だけ)。

**決めたこと(このタスクの中):**
- 説明は、作成の行を書いたあとに本文だけを書く。履歴の `desc` の行は書かない(履歴は「作成」の 1 行。A4a-4)。
- サーバーは、テンプレートの説明をもとになるアイテムに入れ直さない。入力の `description` をそのまま使う(テンプレートの説明の当てはめは、作成ダイアログが A4a-6 の規則で行う。A3b-8 と同じ考え方)。
- 空白だけの説明は、説明なしとして書かない(ファイルを置かない)。

#### Part A: 作成と、テンプレートの子の説明

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/items/description_test.go`(新規。`setupRec`・`last`・`saveTemplate`・`byNumber`・`ptr` は同じパッケージの既存のテストの補助関数)

```go
package items_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// descriptionOf は、アイテムの説明の本文。
func descriptionOf(t *testing.T, s port.Stores, id item.ID) string {
	t.Helper()
	var text string
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		var err error
		text, err = r.Items().Description(id)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return text
}

// logTypes は、アイテムのログの行の種類を、書いた順に返す。
func logTypes(t *testing.T, s port.Stores, id item.ID) []change.Type {
	t.Helper()
	var out []change.Type
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		for _, rec := range log {
			out = append(out, rec.T)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCreateWritesTheDescriptionWithoutADescRow(t *testing.T) {
	svc, rec := setupRec(t)
	it, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: item.Task,
		Title: "API を作る", Description: "## 手順\n1. 確認\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := descriptionOf(t, rec, it.ID); got != "## 手順\n1. 確認\n" {
		t.Errorf("description = %q", got)
	}
	// 履歴は「作成」の 1 行だけ(02「作成したあと」)。desc の行は書かない。
	if types := logTypes(t, rec, it.ID); len(types) != 1 || types[0] != change.Create {
		t.Errorf("log types = %v", types)
	}
	if got := last(rec); got != `Task「API を作る」を作成` {
		t.Errorf("summary = %q", got)
	}
}

func TestCreateWithABlankDescriptionWritesNoDescription(t *testing.T) {
	svc, rec := setupRec(t)
	it, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: item.Task,
		Title: "API を作る", Description: " \n\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := descriptionOf(t, rec, it.ID); got != "" {
		t.Errorf("description = %q", got)
	}
}

func TestCreateFromATemplateWritesTheChildrenDescriptions(t *testing.T) {
	svc, rec := setupRec(t)
	tp := saveTemplate(t, svc, rec, workspace.Template{
		Name: "リリース", Kind: "Epic", Title: "リリース", Description: "## テンプレートの説明",
		Children: []workspace.TemplateChild{
			{Kind: "Story", Title: "QA", Description: "## 確認項目\n", Steps: []workspace.TemplateStep{{Title: "手順書", Assignee: workspace.StepParent}}},
			{Kind: "Task", Title: "本番反映"},
		},
	})
	root, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Kind: "Epic", Title: "11 月のリリース", Template: &tp.ID,
		Description: "入力した説明",
	})
	if err != nil {
		t.Fatal(err)
	}
	// 番号は深さ優先: Epic 3、Story 4、Step 5、Task 6。
	all := byNumber(t, svc)
	qa, step, deploy := all[4], all[5], all[6]
	if got := descriptionOf(t, rec, root.ID); got != "入力した説明" {
		t.Errorf("root description = %q (the template's text is applied by the dialog, not the server)", got)
	}
	if got := descriptionOf(t, rec, qa.ID); got != "## 確認項目\n" {
		t.Errorf("child description = %q", got)
	}
	if descriptionOf(t, rec, step.ID) != "" || descriptionOf(t, rec, deploy.ID) != "" {
		t.Errorf("a step and a child without a description must have none")
	}
	if types := logTypes(t, rec, qa.ID); len(types) != 1 || types[0] != change.Create {
		t.Errorf("child log types = %v", types)
	}
}

func TestCreateWithoutADescriptionLeavesTheRootEmptyEvenForATemplateWithOne(t *testing.T) {
	svc, rec := setupRec(t)
	tp := saveTemplate(t, svc, rec, workspace.Template{Name: "手順", Kind: "Task", Title: "作業", Description: "## 手順"})
	it, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: item.Task, Title: "作業", Template: &tp.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := descriptionOf(t, rec, it.ID); got != "" {
		t.Errorf("description = %q", got)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/items/ -run 'Description'`
Expected: FAIL(`unknown field Description in struct literal of type items.CreateInput`。コンパイルが通らない)

- [ ] **Step 3: 実装する**

`internal/app/items/create.go`

```go
// 変更前
// Template は、一緒に作る構成を持つテンプレート(nil は「なし」。A3b-6)。
type CreateInput struct {
	Store    port.StoreID
	Project  project.ID
	Parent   *item.ID
	Kind     item.Kind
	Title    string
	Values   map[change.ID]json.RawMessage
	Template *change.ID
}
```

```go
// 変更後
// Template は、一緒に作る構成を持つテンプレート(nil は「なし」。A3b-6)。
// Description は、もとになるアイテムの説明(空白だけは説明なし。A4a-4)。
type CreateInput struct {
	Store       port.StoreID
	Project     project.ID
	Parent      *item.ID
	Kind        item.Kind
	Title       string
	Description string
	Values      map[change.ID]json.RawMessage
	Template    *change.ID
}
```

```go
// 変更前
		all, err := s.withTemplate(tx, in, t, created, me)
		if err != nil {
			return err
		}
		id = created.ID
		return writeCreated(tx, all)
```

```go
// 変更後
		all, texts, err := s.withTemplate(tx, in, t, created, me)
		if err != nil {
			return err
		}
		id = created.ID
		return writeCreated(tx, all, texts)
```

```go
// 変更前
// writeCreated は、作るアイテムを番号の順に書く。番号の行も 1 つずつ書く(A3b-11)。
func writeCreated(tx port.Tx, all []item.Item) error {
	var errs []error
	for _, it := range all {
		errs = append(errs,
			tx.WriteWorkspaces().Append(it.Place.Workspace, numbering.Draft(numbering.Items, it.Place.Number)),
			tx.WriteItems().Append(it.ID, item.Create(it)),
		)
	}
	return errors.Join(errs...)
}
```

```go
// 変更後
// writeCreated は、作るアイテムを番号の順に書く。番号の行も 1 つずつ書く(A3b-11)。説明は、そのアイテムの
// 作成の行を書いたあとに本文だけを書く(履歴は「作成」の 1 行。desc の行は書かない。A4a-4)。
func writeCreated(tx port.Tx, all []item.Item, texts descTexts) error {
	var errs []error
	for _, it := range all {
		errs = append(errs,
			tx.WriteWorkspaces().Append(it.Place.Workspace, numbering.Draft(numbering.Items, it.Place.Number)),
			tx.WriteItems().Append(it.ID, item.Create(it)),
		)
		if text := texts[it.ID]; text != "" {
			errs = append(errs, tx.WriteItems().Describe(it.ID, text))
		}
	}
	return errors.Join(errs...)
}
```

`internal/app/items/createtemplate.go`

```go
// 変更前(import)
import (
	"context"
	"encoding/json"

// 変更後
import (
	"context"
	"encoding/json"
	"strings"

```

```go
// 変更前
// others は、テンプレートと一緒に作る子と Step の数。
// 変更後
// descTexts は、作るアイテムの説明の本文(内部 ID → 本文)。
type descTexts map[item.ID]string

// set は、空白だけでない説明を入れる(空の説明はファイルを置かない。A4a-1)。
func (d descTexts) set(id item.ID, text string) {
	if strings.TrimSpace(text) != "" {
		d[id] = text
	}
}

// others は、テンプレートと一緒に作る子と Step の数。
```

```go
// 変更前
// withTemplate は、もとになるアイテム root と、テンプレートの構成から作る子と Step を、番号の順に返す。
// テンプレートを選んだのに読めなかったときは、ほかの検査のあとで ErrNotFound にする(A3b-7)。
func (s Service) withTemplate(tx port.Tx, in CreateInput, t *workspace.Template, root item.Item, me string) ([]item.Item, error) {
	if in.Template == nil {
		return []item.Item{root}, nil
	}
	if t == nil {
		return nil, port.ErrNotFound
	}
	st, err := tx.Workspaces().Settings(root.Place.Workspace)
	if err != nil {
		return nil, err
	}
	e := expander{st: st, me: me, newID: s.IDs.New, next: root.Place.Number, out: []item.Item{root}}
	e.steps(root, t.Steps)
	r := ""
	for _, c := range t.Children {
		r = rank.After(r)
		child := e.add(item.Kind(c.Kind), root, c.Title, r, workspace.TemplateValuesFor(st, c.Kind, c.Values))
		e.steps(child, c.Steps)
	}
	return e.out, nil
}
```

```go
// 変更後
// withTemplate は、もとになるアイテム root と、テンプレートの構成から作る子と Step を、番号の順に返す。
// 2 つ目の値は、作るアイテムの説明(説明のないアイテムは入れない)。もとになるアイテムは入力の説明、
// 子はテンプレートの子の説明、Step は説明を持たない(A4a-4)。
// テンプレートを選んだのに読めなかったときは、ほかの検査のあとで ErrNotFound にする(A3b-7)。
func (s Service) withTemplate(tx port.Tx, in CreateInput, t *workspace.Template, root item.Item, me string) ([]item.Item, descTexts, error) {
	texts := descTexts{}
	texts.set(root.ID, in.Description)
	if in.Template == nil {
		return []item.Item{root}, texts, nil
	}
	if t == nil {
		return nil, nil, port.ErrNotFound
	}
	st, err := tx.Workspaces().Settings(root.Place.Workspace)
	if err != nil {
		return nil, nil, err
	}
	e := expander{st: st, me: me, newID: s.IDs.New, next: root.Place.Number, out: []item.Item{root}}
	e.steps(root, t.Steps)
	r := ""
	for _, c := range t.Children {
		r = rank.After(r)
		child := e.add(item.Kind(c.Kind), root, c.Title, r, workspace.TemplateValuesFor(st, c.Kind, c.Values))
		texts.set(child.ID, c.Description)
		e.steps(child, c.Steps)
	}
	return e.out, texts, nil
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/items/ && make lint`
Expected: PASS、`0 issues.`(`create.go` は 215 行、`createtemplate.go` は 131 行)

- [ ] **Step 5: コミット**

```bash
git add internal/app/items
git commit -m "feat(app): 作成の説明と、テンプレートの子の説明を、作成と同じ保存で書く(A4a-4)

CreateInput に Description を足す。もとになるアイテムは入力の説明、子はテンプレートの子の説明を、
作成の行のあとに本文だけ書く(履歴は「作成」の 1 行)。Step は説明を持たない。空白だけは書かない。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

#### Part B: アイテムから作るテンプレートへ説明を写す

- [ ] **Step 6: 失敗するテストを書く**

`internal/app/workspaces/template_description_test.go`(新規。`setup`・`putItemIn`・`putChild`・`settingsOf` は同じパッケージの既存のテストの補助関数)

```go
package workspaces_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// describe は、アイテムの説明を書く(履歴の desc の行も書く)。
func describe(t *testing.T, s workspaces.Service, id item.ID, text string) {
	t.Helper()
	err := s.Stores.Write(t.Context(), store, port.Operation{Actor: change.User("taro"), Summary: "準備"}, func(tx port.Tx) error {
		return errors.Join(tx.WriteItems().Describe(id, text), tx.WriteItems().Append(id, change.NewDesc()))
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateTemplateFromItemCopiesTheDescriptions(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	p := project.Project{ID: s.IDs.New(), Place: project.Place{Workspace: w.ID, Number: 1}, Rank: "V", Title: "p", Status: project.Todo}
	epic := putItemIn(t, s, p, 1)
	qa := putChild(t, s, p, epic, item.Story, 2, "V", "QA")
	putChild(t, s, p, epic, item.Task, 3, "W", "反映")
	putChild(t, s, p, qa, item.Step, 4, "V", "確認")
	describe(t, s, epic, "## 手順\n")
	describe(t, s, qa, "## 確認項目\n")

	tpl, err := s.CreateTemplateFromItem(ctx, store, epic, workspaces.FromItem{Name: "リリース作業", Title: "リリース", Structure: true})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := settingsOf(t, s, w.ID).Template(tpl.ID)
	if got.Description != "## 手順\n" || len(got.Children) != 2 {
		t.Fatalf("template = %+v", got)
	}
	// 子は説明を写す。説明のない子は空のまま。Step は説明を持たない。
	if got.Children[0].Title != "QA" || got.Children[0].Description != "## 確認項目\n" || got.Children[1].Description != "" {
		t.Errorf("children = %+v", got.Children)
	}

	// 子と Step を含めないなら、アイテムの説明だけを写す。
	plain, err := s.CreateTemplateFromItem(ctx, store, epic, workspaces.FromItem{Name: "リリース 2", Title: "リリース", Structure: false})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := settingsOf(t, s, w.ID).Template(plain.ID); got.Description != "## 手順\n" || len(got.Children) != 0 {
		t.Errorf("plain template = %+v", got)
	}
}

func TestCreateTemplateFromItemWithoutADescription(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	p := project.Project{ID: s.IDs.New(), Place: project.Place{Workspace: w.ID, Number: 1}, Rank: "V", Title: "p", Status: project.Todo}
	epic := putItemIn(t, s, p, 1)
	tpl, err := s.CreateTemplateFromItem(t.Context(), store, epic, workspaces.FromItem{Name: "作業", Title: "作業"})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := settingsOf(t, s, w.ID).Template(tpl.ID); got.Description != "" {
		t.Errorf("description = %q", got.Description)
	}
}
```

- [ ] **Step 7: 失敗を確かめる**

Run: `go test ./internal/app/workspaces/ -run 'TestCreateTemplateFromItemCopiesTheDescriptions'`
Expected: FAIL(`template = {… Description: …}`。`got.Description` が空で、`## 手順` を写していない)

- [ ] **Step 8: 実装する**

`internal/app/workspaces/template_item.go`

```go
// 変更前
// CreateTemplateFromItem は、アイテム id をもとに、そのワークスペースにテンプレートを作る。
// 種類・値・(Structure なら)子と Step を、並び順のまま写す(B3-16)。保存と検査は CreateTemplate と同じ。
```

```go
// 変更後
// CreateTemplateFromItem は、アイテム id をもとに、そのワークスペースにテンプレートを作る。
// 種類・説明・値・(Structure なら)子(説明を含む)と Step を、並び順のまま写す(B3-16、A4a-5)。
// 保存と検査は CreateTemplate と同じ。
```

```go
// 変更前
		ws, t = it.Place.Workspace, templateOf(st, it, family, in)
		return nil
```

```go
// 変更後
		texts, err := descriptionsOf(r, it, family)
		if err != nil {
			return err
		}
		ws, t = it.Place.Workspace, templateOf(st, it, family, texts, in)
		return nil
```

```go
// 変更前
func templateOf(st workspace.Settings, it item.Item, family []item.Item, in FromItem) workspace.Template {
	t := workspace.Template{Name: in.Name, Kind: string(it.Kind), Title: in.Title, Values: valuesOf(st, it)}
```

```go
// 変更後
// descriptionsOf は、テンプレートに写す説明(アイテムと、含める子。Step は説明を持たない)。
func descriptionsOf(r port.Reader, it item.Item, family []item.Item) (map[item.ID]string, error) {
	ids := []item.ID{it.ID}
	if it.Kind == item.Epic {
		for _, c := range childrenOf(family, it.ID) {
			ids = append(ids, c.ID)
		}
	}
	out := make(map[item.ID]string, len(ids))
	for _, id := range ids {
		text, err := r.Items().Description(id)
		if err != nil {
			return nil, err
		}
		out[id] = text
	}
	return out, nil
}

func templateOf(st workspace.Settings, it item.Item, family []item.Item, texts map[item.ID]string, in FromItem) workspace.Template {
	t := workspace.Template{Name: in.Name, Kind: string(it.Kind), Title: in.Title, Description: texts[it.ID], Values: valuesOf(st, it)}
```

```go
// 変更前
				Kind: string(c.Kind), Title: c.Title, Values: valuesOf(st, c), Steps: stepsOf(family, c),
// 変更後
				Kind: string(c.Kind), Title: c.Title, Description: texts[c.ID], Values: valuesOf(st, c), Steps: stepsOf(family, c),
```

(`family` は「子と Step を含める」がオフのとき空なので、`descriptionsOf` が読むのはアイテム自身だけになる。)

- [ ] **Step 9: 通ることを確かめる**

Run: `go test ./internal/app/workspaces/ && make lint`
Expected: PASS、`0 issues.`(`template_item.go` は 130 行)

- [ ] **Step 10: コミット**

```bash
git add internal/app/workspaces
git commit -m "feat(app): アイテムから作るテンプレートに、アイテムと子の説明を写す(A4a-5)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

#### Part C: 作成の API に説明を足す

- [ ] **Step 11: 失敗するテストを書く**

`internal/adapter/httpapi/items/description_test.go`(新規)

```go
package items_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/items"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
)

func TestCreateAcceptsADescriptionAndSavesIt(t *testing.T) {
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	items.Register(api, appitems.Service{Stores: s, IDs: ids, Identity: apptest.Identity{Login: "taro"}, Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))})

	res := api.Post("/api/stores/S1/items", map[string]any{
		"project": string(apptest.SampleProject), "parent": string(apptest.SampleEpic), "kind": "Task", "title": "API",
		"description": "## 手順\n",
	})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"number":3`) {
		t.Fatalf("create: %d %s", res.Code, res.Body)
	}
	var text string
	err := s.Read(t.Context(), "S1", func(r port.Reader) error {
		list, err := r.Items().List(apptest.SampleProject)
		if err != nil {
			return err
		}
		text, err = r.Items().Description(list[len(list)-1].ID)
		return err
	})
	if err != nil || text != "## 手順\n" {
		t.Errorf("description = %q, %v", text, err)
	}
}
```

- [ ] **Step 12: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/items/ -run TestCreateAcceptsADescriptionAndSavesIt`
Expected: FAIL(`create: 422 …`。本文に知らない項目 `description` があるので huma が断る)

- [ ] **Step 13: 実装する**

`internal/adapter/httpapi/items/items.go`

```go
// 変更前
	Title   string `json:"title"`

	Values   map[string]json.RawMessage `json:"values,omitempty" doc:"プロパティの値(キーはプロパティの内部 ID。組み込みはキー)"`
	Template string                     `json:"template,omitempty" doc:"一緒に作るテンプレートの内部 ID。空は「なし」"`
}
```

```go
// 変更後
	Title   string `json:"title"`

	Description string                     `json:"description,omitempty" doc:"もとになるアイテムの説明(Markdown)。空は説明なし。子の説明はテンプレートのものを使う"`
	Values      map[string]json.RawMessage `json:"values,omitempty" doc:"プロパティの値(キーはプロパティの内部 ID。組み込みはキー)"`
	Template    string                     `json:"template,omitempty" doc:"一緒に作るテンプレートの内部 ID。空は「なし」"`
}
```

```go
// 変更前
			req := appitems.CreateInput{Store: port.StoreID(in.Store), Project: project.ID(in.Body.Project), Kind: item.Kind(in.Body.Kind), Title: in.Body.Title}
// 変更後
			req := appitems.CreateInput{Store: port.StoreID(in.Store), Project: project.ID(in.Body.Project), Kind: item.Kind(in.Body.Kind), Title: in.Body.Title, Description: in.Body.Description}
```

- [ ] **Step 14: API の生成物を作り直し、全部通ることを確かめる**

Run: `go test ./internal/adapter/httpapi/items/`
Expected: PASS

Run: `make generate && pnpm -C web gen`
Expected: `api/openapi.yaml` の `CreateItem` に `description`(文字列、必須でない)が増える。`web/packages/api/src/generated.ts` の `CreateItem` に `description?: string` が増える。

Run: `go test ./internal/... && make lint && pnpm -C web check && pnpm -C web test`
Expected: PASS、`0 issues.`(画面はまだ `description` を送らない。送るのはタスク 8)

- [ ] **Step 15: コミット**

```bash
git add internal/adapter/httpapi/items api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat(api): アイテムの作成の本文に説明を足す(A4a-4)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 5: Markdown の字句分けと `Markdown.svelte`

説明とコメントを表示する Markdown(02「Markdown」「メンション」)。core が行とインラインの字句に分け(DOM を知らない)、ui が Svelte の要素で描く。`{@html}` は使わない。Task 7(説明)と Task 10(コメント)が使う。

**Files:**
- Create: `web/packages/core/src/features/markdown/markdown.ts`
- Test: `web/packages/core/src/features/markdown/markdown.test.ts`
- Create: `web/packages/core/src/notesIndex.ts`(A4a の core の export をまとめる。`index.ts` が 298 行で上限(300)に近いので、ここに分ける)
- Modify: `web/packages/core/src/index.ts`(末尾に 1 行)
- Create: `web/packages/ui/src/patterns/Markdown.svelte`
- Test: `web/packages/ui/src/patterns/markdown.test.ts`
- Modify: `web/packages/ui/src/types.ts`(末尾に足す)
- Modify: `web/packages/ui/src/index.ts`

**Interfaces:**
- Produces(core):
  - `type Inline = { kind: "text"; text: string } | { kind: "bold"; text: string } | { kind: "code"; text: string } | { kind: "link"; url: string } | { kind: "mention"; login: string }`
  - `type Check = "none" | "unchecked" | "checked"`、`interface ListItem { check: Check; inlines: Inline[] }`
  - `type Block = { kind: "heading"; level: 1 | 2 | 3; inlines: Inline[] } | { kind: "list"; ordered: boolean; items: ListItem[] } | { kind: "code"; text: string } | { kind: "paragraph"; inlines: Inline[] }`
  - `function tokenize(text: string, isMember: (login: string) => boolean): Block[]`
  - `function isMemberOf(members: Member[]): (login: string) => boolean`(ログイン名を大文字小文字を区別せずに比べる)
  - `notesIndex.ts` から `MarkdownBlock`(= `Block`)・`MarkdownInline`(= `Inline`)・`tokenize`・`isMemberOf` を公開し、`index.ts` が `export * from "./notesIndex"` で再公開する
- Produces(ui): `Markdown.svelte`(props `{ blocks: MarkdownBlock[] }`)。ui は core を import できないので、ui の `types.ts` に同じ形の `MarkdownBlock`・`MarkdownInline`・`MarkdownListItem` を置く(app が core の結果をそのまま渡す。形がずれると app の `pnpm -C web check` が落ちる)

**決めたこと(仕様の表に書いていない細部):**
- `1.` で始まる行は `<ol>`、`-` `*` の行は `<ul>`。同じ種類の行が続くあいだは 1 つの list にする(`1.` は `\d+.` を指す)
- 見出しは `#` の後に空白が要る(`#空白なし` は段落)。`####` 以上は段落
- 行頭の `[ ]` と `[x]`(小文字)だけを印にする
- 空行(空白だけの行も)は読み飛ばし、段落を作らない。閉じていないコードブロックは最後の行まで。開きの行の言語名(```` ```ts ````)は使わない
- URL の末尾の `.,;:!?)'*` はリンクに含めない。ホストのない `http://` はリンクにしない
- メンションのログイン名は `[A-Za-z0-9_-]{1,39}`(`internal/domain/member` の `loginPattern` と同じ)。`@` の前の文字は問わない(02「メンション」)。メンバーでない `@名前` はただの文字。表示は打ったとおりの綴り
- 見出しは、詳細パネルの区画の見出し(`h3`)の下になるよう `h4`・`h5`・`h6` で出す

- [ ] **Step 1: core の失敗するテストを書く**

`web/packages/core/src/features/markdown/markdown.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { isMemberOf, tokenize } from "./markdown";

const nobody = () => false;
const text = (s: string) => ({ kind: "text" as const, text: s });

describe("tokenize blocks", () => {
  it("makes headings of three levels; four # or no space is a paragraph", () => {
    expect(tokenize("# 見出し1\n## 見出し2\n### 見出し3\n#### 4つ\n#空白なし", nobody)).toEqual([
      { kind: "heading", level: 1, inlines: [text("見出し1")] },
      { kind: "heading", level: 2, inlines: [text("見出し2")] },
      { kind: "heading", level: 3, inlines: [text("見出し3")] },
      { kind: "paragraph", inlines: [text("#### 4つ")] },
      { kind: "paragraph", inlines: [text("#空白なし")] },
    ]);
  });

  it("groups list lines by kind and reads the checkbox marks", () => {
    expect(tokenize("- 一つ目\n* 二つ目\n- [ ] 未\n- [x] 済\n1. 番号\n2. 番号2", nobody)).toEqual([
      {
        kind: "list",
        ordered: false,
        items: [
          { check: "none", inlines: [text("一つ目")] },
          { check: "none", inlines: [text("二つ目")] },
          { check: "unchecked", inlines: [text("未")] },
          { check: "checked", inlines: [text("済")] },
        ],
      },
      {
        kind: "list",
        ordered: true,
        items: [
          { check: "none", inlines: [text("番号")] },
          { check: "none", inlines: [text("番号2")] },
        ],
      },
    ]);
  });

  it("keeps a fenced code block as it is; an unclosed fence runs to the end", () => {
    expect(tokenize("前\n```ts\nconst a = 1;\n\n  b\n```\n後", nobody)).toEqual([
      { kind: "paragraph", inlines: [text("前")] },
      { kind: "code", text: "const a = 1;\n\n  b" },
      { kind: "paragraph", inlines: [text("後")] },
    ]);
    expect(tokenize("```\nabc\ndef", nobody)).toEqual([{ kind: "code", text: "abc\ndef" }]);
    expect(tokenize("```\n```", nobody)).toEqual([{ kind: "code", text: "" }]);
  });

  it("makes one paragraph per other line and skips blank lines and CRLF", () => {
    expect(tokenize("一行目\r\n\r\n二行目\n   \n三行目", nobody)).toEqual([
      { kind: "paragraph", inlines: [text("一行目")] },
      { kind: "paragraph", inlines: [text("二行目")] },
      { kind: "paragraph", inlines: [text("三行目")] },
    ]);
    expect(tokenize("", nobody)).toEqual([]);
  });
});

describe("tokenize inlines", () => {
  const isTaro = isMemberOf([{ login: "Taro", name: "太郎" }]);

  it("reads bold, code, a link without its trailing dot, and members' mentions", () => {
    expect(
      tokenize("**太字** と `code` と https://example.com/a?b=1. の @TARO さん @guest", isTaro),
    ).toEqual([
      {
        kind: "paragraph",
        inlines: [
          { kind: "bold", text: "太字" },
          text(" と "),
          { kind: "code", text: "code" },
          text(" と "),
          { kind: "link", url: "https://example.com/a?b=1" },
          text(". の "),
          // 大文字小文字を区別せずに一致し、表示は打ったとおり。メンバーでない @guest はただの文字。
          { kind: "mention", login: "TARO" },
          text(" さん @guest"),
        ],
      },
    ]);
  });

  it("treats markup, other schemes and a bare http:// as plain text (XSS)", () => {
    const line = "<script>alert(1)</script> [x](javascript:alert(1)) http:// https://.";
    expect(tokenize(line, nobody)).toEqual([{ kind: "paragraph", inlines: [text(line)] }]);
  });

  it("takes @ after any character as a mention, but not inside a URL", () => {
    expect(tokenize("https://a.example/@taro と foo@Taro", isTaro)).toEqual([
      {
        kind: "paragraph",
        inlines: [
          { kind: "link", url: "https://a.example/@taro" },
          text(" と foo"),
          { kind: "mention", login: "Taro" },
        ],
      },
    ]);
  });

  it("reads inlines inside a list item", () => {
    expect(tokenize("- 担当 @taro", isTaro)).toEqual([
      {
        kind: "list",
        ordered: false,
        items: [{ check: "none", inlines: [text("担当 "), { kind: "mention", login: "taro" }] }],
      },
    ]);
  });
});

describe("isMemberOf", () => {
  it("compares logins without regard to case", () => {
    const is = isMemberOf([{ login: "Taro", name: "太郎" }]);
    expect(is("taro")).toBe(true);
    expect(is("jiro")).toBe(false);
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web/packages/core exec vitest run src/features/markdown/markdown.test.ts`
Expected: FAIL(`Failed to resolve import "./markdown"`)

- [ ] **Step 3: core の実装を書く**

`web/packages/core/src/features/markdown/markdown.ts`:

```ts
import type { Member } from "../../model/member";

// Markdown の字句分け(02「Markdown」「メンション」)。説明とコメントで使える書き方だけを読み、それ以外は
// 文字のまま残す。HTML は作らない(描くのは ui の Markdown.svelte。{@html} は使わない)。

export type Inline =
  | { kind: "text"; text: string }
  | { kind: "bold"; text: string }
  | { kind: "code"; text: string }
  | { kind: "link"; url: string }
  | { kind: "mention"; login: string };

export type Check = "none" | "unchecked" | "checked";

export interface ListItem {
  check: Check;
  inlines: Inline[];
}

export type Block =
  | { kind: "heading"; level: 1 | 2 | 3; inlines: Inline[] }
  | { kind: "list"; ordered: boolean; items: ListItem[] }
  | { kind: "code"; text: string }
  | { kind: "paragraph"; inlines: Inline[] };

// 左から順に、行内のコード・太字・リンク(http と https だけ)・メンション。ログイン名の形は
// internal/domain/member の loginPattern と同じ。
const inlinePattern =
  /`([^`]+)`|\*\*(.+?)\*\*|(https?:\/\/[A-Za-z0-9._~:/?#[\]@!$&'()*+,;=%-]+)|@([A-Za-z0-9_-]{1,39})/g;
const urlTail = /[.,;:!?)'*]+$/;
const urlWithoutHost = /^https?:\/\/$/;
const headingLine = /^(#{1,3})\s+(\S.*)$/;
const listLine = /^\s*(?:[-*]|(\d+\.))\s+(.*)$/;
const checkLine = /^\[( |x)\](?:\s+(.*))?$/;
const fenceLine = /^\s*```/;

// isMemberOf は、メンバーのログイン名と大文字小文字を区別せずに一致するかを返す関数を作る。
export const isMemberOf = (members: Member[]) => {
  const logins = new Set(members.map((m) => m.login.toLowerCase()));
  return (login: string) => logins.has(login.toLowerCase());
};

function inlinesOf(line: string, isMember: (login: string) => boolean): Inline[] {
  const out: Inline[] = [];
  const text = (s: string) => {
    if (s === "") return;
    const last = out.at(-1);
    if (last?.kind === "text") last.text += s;
    else out.push({ kind: "text", text: s });
  };
  let at = 0;
  for (const m of line.matchAll(inlinePattern)) {
    const [whole, code, bold, link, login] = m;
    const index = m.index ?? 0;
    text(line.slice(at, index));
    at = index + (whole?.length ?? 0);
    if (code !== undefined) out.push({ kind: "code", text: code });
    else if (bold !== undefined) out.push({ kind: "bold", text: bold });
    else if (link !== undefined) {
      // 末尾の句読点はリンクに含めない。ホストのない http:// はリンクにしない。
      const url = link.replace(urlTail, "");
      if (urlWithoutHost.test(url)) text(link);
      else {
        out.push({ kind: "link", url });
        at = index + url.length;
      }
    } else if (login !== undefined) {
      // メンバーでない @名前 は、ただの文字(メンションの表示にしない)。
      if (isMember(login)) out.push({ kind: "mention", login });
      else text(whole ?? "");
    }
  }
  text(line.slice(at));
  return out;
}

function listItemOf(rest: string, isMember: (login: string) => boolean): ListItem {
  const box = checkLine.exec(rest);
  if (!box) return { check: "none", inlines: inlinesOf(rest, isMember) };
  return {
    check: box[1] === "x" ? "checked" : "unchecked",
    inlines: inlinesOf(box[2] ?? "", isMember),
  };
}

// tokenize は、本文を行ごとに読んで Block の並びにする。isMember が true の @ログイン名だけをメンションにする。
export function tokenize(text: string, isMember: (login: string) => boolean): Block[] {
  const lines = text.split(/\r?\n/);
  const blocks: Block[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i++] ?? "";
    if (fenceLine.test(line)) {
      const body: string[] = [];
      while (i < lines.length && !fenceLine.test(lines[i] ?? "")) body.push(lines[i++] ?? "");
      i++; // 閉じの行(なければ最後まで読んでいる)
      blocks.push({ kind: "code", text: body.join("\n") });
      continue;
    }
    if (line.trim() === "") continue;
    const heading = headingLine.exec(line);
    if (heading) {
      const level = (heading[1] ?? "#").length as 1 | 2 | 3;
      const inlines = inlinesOf((heading[2] ?? "").trimEnd(), isMember);
      blocks.push({ kind: "heading", level, inlines });
      continue;
    }
    const row = listLine.exec(line);
    if (row) {
      const ordered = row[1] !== undefined;
      const item = listItemOf((row[2] ?? "").trimEnd(), isMember);
      const last = blocks.at(-1);
      if (last?.kind === "list" && last.ordered === ordered) last.items.push(item);
      else blocks.push({ kind: "list", ordered, items: [item] });
      continue;
    }
    blocks.push({ kind: "paragraph", inlines: inlinesOf(line, isMember) });
  }
  return blocks;
}
```

`web/packages/core/src/notesIndex.ts`(新規):

```ts
// 詳細パネルの説明・Markdown・コメントなど(トラック A4a)の core の export。index.ts が全部を再公開する
// (index.ts は 300 行の上限に近いので、A4a の分はここに足す)。
export {
  type Block as MarkdownBlock,
  type Inline as MarkdownInline,
  isMemberOf,
  tokenize,
} from "./features/markdown/markdown";
```

`web/packages/core/src/index.ts` の末尾(`writable,\n} from "./reactive/store";` の次の行)に足す:

```ts
export * from "./notesIndex";
```

- [ ] **Step 4: core のテストを通す**

Run: `pnpm -C web/packages/core exec vitest run src/features/markdown/markdown.test.ts`
Expected: PASS(8 件)

- [ ] **Step 5: ui の失敗するテストを書く**

`web/packages/ui/src/patterns/markdown.test.ts`:

```ts
import { render, screen, within } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import type { MarkdownBlock } from "../types";
import Markdown from "./Markdown.svelte";

const text = (s: string) => ({ kind: "text" as const, text: s });

const blocks: MarkdownBlock[] = [
  { kind: "heading", level: 1, inlines: [text("概要")] },
  {
    kind: "paragraph",
    inlines: [
      { kind: "bold", text: "太字" },
      text(" と "),
      { kind: "code", text: "go test" },
      text(" と "),
      { kind: "link", url: "https://example.com/a" },
      text(" と "),
      { kind: "mention", login: "taro" },
    ],
  },
  {
    kind: "list",
    ordered: false,
    items: [
      { check: "unchecked", inlines: [text("未")] },
      { check: "checked", inlines: [text("済")] },
      { check: "none", inlines: [text("印なし")] },
    ],
  },
  { kind: "list", ordered: true, items: [{ check: "none", inlines: [text("手順")] }] },
  { kind: "code", text: "<script>alert(1)</script>" },
];

describe("Markdown", () => {
  it("renders every kind of block and inline", () => {
    const { container } = render(Markdown, { props: { blocks } });
    // 見出しは、区画の見出し(h3)の下になるよう h4 から。
    expect(screen.getByRole("heading", { level: 4, name: "概要" })).toBeInTheDocument();
    const para = container.querySelector("p");
    expect(para?.textContent).toBe("太字 と go test と https://example.com/a と @taro");
    expect(container.querySelector("strong")).toHaveTextContent("太字");
    expect(container.querySelector("p code")).toHaveTextContent("go test");
    expect(screen.getByText("@taro")).toHaveClass("mention");
    const lists = screen.getAllByRole("list");
    expect(lists).toHaveLength(2);
    expect(within(lists[0] as HTMLElement).getAllByRole("listitem")).toHaveLength(3);
    expect(container.querySelector("ol")).toHaveTextContent("手順");
    expect(screen.getByRole("img", { name: "未チェック" })).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "チェック済み" })).toBeInTheDocument();
  });

  it("opens links in a new tab without leaking the opener", () => {
    render(Markdown, { props: { blocks } });
    const link = screen.getByRole("link", { name: "https://example.com/a" });
    expect(link).toHaveAttribute("href", "https://example.com/a");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("shows markup as text and never makes an element from it (XSS)", () => {
    const { container } = render(Markdown, { props: { blocks } });
    expect(container.querySelector("script")).toBeNull();
    expect(screen.getByText("<script>alert(1)</script>")).toBeInTheDocument();
  });
});
```

- [ ] **Step 6: 失敗を確かめる**

Run: `pnpm -C web/packages/ui exec vitest run src/patterns/markdown.test.ts`
Expected: FAIL(`Failed to resolve import "./Markdown.svelte"`)

- [ ] **Step 7: ui の実装を書く**

`web/packages/ui/src/types.ts` の末尾に足す:

```ts

// Markdown の表示(Markdown.svelte)に渡す形。core の markdown.ts の Block・Inline と同じ形で、ui は core を
// import できないのでここに持つ。app が core の字句分けの結果をそのまま渡す。
export type MarkdownInline =
  | { kind: "text"; text: string }
  | { kind: "bold"; text: string }
  | { kind: "code"; text: string }
  | { kind: "link"; url: string }
  | { kind: "mention"; login: string };

export interface MarkdownListItem {
  check: "none" | "unchecked" | "checked";
  inlines: MarkdownInline[];
}

export type MarkdownBlock =
  | { kind: "heading"; level: 1 | 2 | 3; inlines: MarkdownInline[] }
  | { kind: "list"; ordered: boolean; items: MarkdownListItem[] }
  | { kind: "code"; text: string }
  | { kind: "paragraph"; inlines: MarkdownInline[] };
```

`web/packages/ui/src/index.ts`: `KindIcon` の行の次に足し、型の export に `MarkdownBlock`・`MarkdownInline`・`MarkdownListItem` を足す(`pnpm -C web fix` が並べ替える)。

```ts
export { default as Markdown } from "./patterns/Markdown.svelte";
```

```ts
export type {
  Category,
  KindName,
  MarkdownBlock,
  MarkdownInline,
  MarkdownListItem,
  MenuItem,
  SelectOption,
  SyncCardView,
  Tab,
  Toast,
  ValueOption,
} from "./types";
```

`web/packages/ui/src/patterns/Markdown.svelte`:

```svelte
<script lang="ts">
  import type { MarkdownBlock, MarkdownInline, MarkdownListItem } from "../types";

  // Markdown の表示(02「Markdown」)。{@html} は使わない。文字はすべて本文として出すので、タグや
  // javascript: のような文字列は、ただの文字になる。リンクは http(s) だけ(core の字句分けが決める)。
  interface Props {
    blocks: MarkdownBlock[];
  }

  let { blocks }: Props = $props();
</script>

{#snippet inlines(list: MarkdownInline[])}
  {#each list as n, i (i)}
    {#if n.kind === "bold"}
      <strong>{n.text}</strong>
    {:else if n.kind === "code"}
      <code>{n.text}</code>
    {:else if n.kind === "link"}
      <a href={n.url} target="_blank" rel="noopener noreferrer">{n.url}</a>
    {:else if n.kind === "mention"}
      <span class="mention">@{n.login}</span>
    {:else}
      {n.text}
    {/if}
  {/each}
{/snippet}

{#snippet items(list: MarkdownListItem[])}
  {#each list as it, i (i)}
    <li class:marked={it.check !== "none"}>
      {#if it.check === "checked"}
        <span class="mark" role="img" aria-label="チェック済み">☑</span>
      {:else if it.check === "unchecked"}
        <span class="mark" role="img" aria-label="未チェック">☐</span>
      {/if}
      {@render inlines(it.inlines)}
    </li>
  {/each}
{/snippet}

<div class="md">
  {#each blocks as b, i (i)}
    {#if b.kind === "heading"}
      <svelte:element this={`h${b.level + 3}`} class="h{b.level}">
        {@render inlines(b.inlines)}
      </svelte:element>
    {:else if b.kind === "list"}
      {#if b.ordered}
        <ol>{@render items(b.items)}</ol>
      {:else}
        <ul>{@render items(b.items)}</ul>
      {/if}
    {:else if b.kind === "code"}
      <pre><code>{b.text}</code></pre>
    {:else}
      <p>{@render inlines(b.inlines)}</p>
    {/if}
  {/each}
</div>

<style>
  .md {
    display: flex;
    flex-direction: column;
    gap: 6px;
    overflow-wrap: anywhere;
  }
  p,
  h4,
  h5,
  h6 {
    margin: 0;
  }
  .h1 {
    font-size: 16px;
  }
  .h2 {
    font-size: 14.5px;
  }
  .h3 {
    font-size: 13.5px;
  }
  ul,
  ol {
    margin: 0;
    padding-left: 20px;
  }
  li.marked {
    list-style: none;
    margin-left: -20px;
  }
  .mark {
    color: var(--muted);
  }
  pre {
    margin: 0;
    padding: 8px 10px;
    background: var(--sunken);
    border-radius: 6px;
    overflow-x: auto;
  }
  code {
    background: var(--sunken);
    border-radius: 4px;
    padding: 1px 4px;
  }
  pre code {
    background: none;
    padding: 0;
  }
  .mention {
    color: var(--accent-ink);
    background: var(--accent-weak);
    border-radius: 4px;
    padding: 0 3px;
  }
</style>
```

- [ ] **Step 8: 通ることと、検査を確かめる**

Run: `pnpm -C web/packages/ui exec vitest run src/patterns/markdown.test.ts`
Expected: PASS(3 件)。段落の `textContent` が空白の違いで落ちたら、`{#each}` と `{#if}` の周りの改行を詰めて空白を出さないようにする(文字の間の空白は `n.text` の中だけにある)。

Run: `pnpm -C web fix && pnpm -C web lint && pnpm -C web check && pnpm -C web size && pnpm -C web deps`
Expected: すべて成功(`deps` は ui が core を import していないことも見る)。

- [ ] **Step 9: コミット**

```bash
git add web/packages/core/src/features/markdown web/packages/core/src/notesIndex.ts web/packages/core/src/index.ts web/packages/ui/src/patterns/Markdown.svelte web/packages/ui/src/patterns/markdown.test.ts web/packages/ui/src/types.ts web/packages/ui/src/index.ts
git commit -m "feat(web): Markdown の字句分けと表示の部品を足す(A4a-20)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 6: 書きかけ(`drafts`)・破棄の確認・反映を待つ条件

説明・コメントの入力欄が自分を登録する場所(A4a-21)と、「入力を破棄」の確認(A4a-22)。確認を出す先は、入力欄の中の Esc(Task 7・10 が呼ぶ)、別のアイテムの詳細パネルを開く、パネルを閉じる、画面を移る(この Task で結び付ける)。登録するのは Task 7(説明)と Task 10(コメント)。

**Files:**
- Create: `web/packages/core/src/features/drafts/drafts.ts`
- Test: `web/packages/core/src/features/drafts/drafts.test.ts`
- Modify: `web/packages/core/src/features/router/navigation.ts`
- Create: `web/packages/core/src/features/router/history.test-util.ts`(`navigation.test.ts` の `fakeHistory` を移す)
- Modify: `web/packages/core/src/features/router/navigation.test.ts`(`fakeHistory` を import に替える)
- Test: `web/packages/core/src/features/router/navigationLeave.test.ts`
- Modify: `web/packages/core/src/features/items/detail.ts`(`leave`・`requestClose`)
- Test: `web/packages/core/src/features/items/detailLeave.test.ts`
- Modify: `web/packages/core/src/notesIndex.ts`
- Create: `web/app/src/drafts.ts`
- Modify: `web/app/src/tab.ts`
- Modify: `web/app/src/wire.ts`
- Create: `web/app/src/screens/notes/DiscardDialog.svelte`
- Modify: `web/app/src/App.svelte`
- Modify: `web/app/src/screens/item/DetailPanel.svelte`・`web/app/src/screens/item/DetailHeader.svelte`

**Interfaces:**
- Produces(core, `drafts.ts`):
  - `interface Drafts { register(key: string, dirty: () => boolean, held?: () => boolean): () => void; anyHeld(): boolean; anyDirty(): boolean; confirmLeave(key?: string): Promise<boolean>; prompt: Readable<DiscardPrompt | null>; answer(discard: boolean): void }`
  - `interface DiscardPrompt { title: string; lines: string[]; ok: string }`(title は「入力を破棄」、ok は「破棄」)
  - `interface LeaveGuard { anyDirty(): boolean; confirmLeave(): Promise<boolean> }`(`Drafts` が満たす)
  - `function createDrafts(): Drafts`
  - `key` は `description:<何か>` か `comment:<何か>` にする。`:` より前の語で確認の本文を決める(`comment` は「保存していないコメントを破棄します。」、それ以外は「保存していない説明の変更を破棄します。」)。`dirty` は「保存してあるものと違う」、`held`(省略すると `dirty` と同じ)は「違い、かつ空でない」(02「説明」: 書きかけは「保存していない入力が空でない」もの)
- Produces(core): `createNavigation(history: HistoryPort, guard?: LeaveGuard): Navigation`(形は変えない。`guard` が `anyDirty()` のとき、`go` と戻る・進むは `confirmLeave()` が true のときだけ移る。`replace` は確認しない)
- Produces(core): `DetailDeps.leave?: () => Promise<boolean>`、`Detail.requestClose(): Promise<void>`(`open` は、別のアイテムが開いているとき `leave` が true のときだけ開く。`close()` は確認しない: 画面の移り変わりと、反映でアイテムがなくなったときの閉じ方)
- Produces(app): `web/app/src/drafts.ts` の `drafts`(`createDrafts()` を 1 つ作る。`wire.ts` と `tab.ts` の両方が使うので `wire.ts` の外に置く)、`DiscardDialog.svelte`

**決めたこと:**
- 確認が出るのは、書きかけが「保存してあるものと違う」(`dirty`)とき。空にしただけの入力も違うので出る。一方、反映を待つのは「違い、かつ空でない」(`held`)ときだけ
- 日本語の変換中の Esc は、入力欄の側(Task 7・10)が `event.isComposing` で除く
- `replace` は確認しない(アイテムの入口のアドレスを画面のアドレスに置き換える内部の動きで、画面を移る操作ではない)
- 「破棄」を選んだあと、入力欄そのものを捨てるのは確認を呼んだ側。この部品は答えを返すだけ
- 戻る・進むを「キャンセル」したときは、ブラウザのアドレスがもう変わっているので、いまの画面のアドレスを積み直して留まる(履歴の先の項目は失われる)

- [ ] **Step 1: drafts の失敗するテストを書く**

`web/packages/core/src/features/drafts/drafts.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createDrafts } from "./drafts";

describe("drafts", () => {
  it("holds only a non-empty input that differs; dirty is any difference", () => {
    const d = createDrafts();
    const box = { text: "", saved: "" };
    d.register(
      "description:item/I1",
      () => box.text !== box.saved,
      () => box.text !== box.saved && box.text !== "",
    );
    expect([d.anyHeld(), d.anyDirty()]).toEqual([false, false]);
    box.text = "書きかけ";
    expect([d.anyHeld(), d.anyDirty()]).toEqual([true, true]);
    box.saved = "書きかけ";
    expect([d.anyHeld(), d.anyDirty()]).toEqual([false, false]);
    // 保存してある説明を空にしようとしている: 確認は要るが、反映を待たせるほどの書きかけではない。
    box.saved = "古い説明";
    box.text = "";
    expect([d.anyHeld(), d.anyDirty()]).toEqual([false, true]);
  });

  it("holds as soon as it is dirty when no held is given, and unregistering removes it", () => {
    const d = createDrafts();
    const off = d.register("comment:item/I1", () => true);
    expect([d.anyHeld(), d.anyDirty()]).toEqual([true, true]);
    off();
    expect([d.anyHeld(), d.anyDirty()]).toEqual([false, false]);
  });

  it("keeps a registration made again under the same key when the old one is removed", () => {
    const d = createDrafts();
    const old = d.register("description:item/I1", () => true);
    d.register("description:item/I1", () => true);
    old();
    expect(d.anyDirty()).toBe(true);
  });

  it("answers true at once when nothing is dirty", async () => {
    const d = createDrafts();
    d.register("description:item/I1", () => false);
    expect(await d.confirmLeave()).toBe(true);
    expect(get(d.prompt)).toBeNull();
  });

  it("asks once, with a line per kind, and answers with the button", async () => {
    const d = createDrafts();
    d.register("comment:item/I1", () => true);
    d.register("description:item/I1", () => true);
    d.register("comment:item/I1/2", () => true);
    const first = d.confirmLeave();
    const second = d.confirmLeave();
    expect(get(d.prompt)).toEqual({
      title: "入力を破棄",
      lines: ["保存していないコメントを破棄します。", "保存していない説明の変更を破棄します。"],
      ok: "破棄",
    });
    d.answer(true);
    expect([await first, await second]).toEqual([true, true]);
    expect(get(d.prompt)).toBeNull();
    const third = d.confirmLeave();
    d.answer(false);
    expect(await third).toBe(false);
  });

  it("asks only about the given key", async () => {
    const d = createDrafts();
    d.register("comment:item/I1", () => true);
    d.register("description:item/I1", () => false);
    expect(await d.confirmLeave("description:item/I1")).toBe(true);
    const asked = d.confirmLeave("comment:item/I1");
    expect(get(d.prompt)?.lines).toEqual(["保存していないコメントを破棄します。"]);
    d.answer(false);
    expect(await asked).toBe(false);
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web/packages/core exec vitest run src/features/drafts/drafts.test.ts`
Expected: FAIL(`Failed to resolve import "./drafts"`)

- [ ] **Step 3: drafts を実装する**

`web/packages/core/src/features/drafts/drafts.ts`:

```ts
import { type Readable, readonly, writable } from "../../reactive/store";

// 書きかけ(説明・コメントの入力欄の、保存していない入力)の登録簿(02「説明」、06「反映を待つとき」)。
// 入力欄が register で自分を登録する。app は anyHeld を反映を待つ条件に、confirmLeave を画面を離れる前の
// 確認に使う。

export const discardTitle = "入力を破棄";
export const discardOk = "破棄";
const commentLine = "保存していないコメントを破棄します。";
const descriptionLine = "保存していない説明の変更を破棄します。";

export interface DiscardPrompt {
  title: string;
  lines: string[];
  ok: string;
}

// LeaveGuard は、画面を離れてよいかを聞く側(ルーター)が使う部分。
export interface LeaveGuard {
  anyDirty(): boolean;
  confirmLeave(): Promise<boolean>;
}

export interface Drafts extends LeaveGuard {
  // register は、入力欄を登録し、外す関数を返す。key は "description:…" か "comment:…"(: より前の語で確認の
  // 本文を決める)。dirty は保存してあるものと違うか、held は違い、かつ空でないか(省略すると dirty と同じ)。
  register(key: string, dirty: () => boolean, held?: () => boolean): () => void;
  // anyHeld は、空でない書きかけがあるか(自動の同期と画面の反映を待つ条件)。
  anyHeld(): boolean;
  // confirmLeave は、違う入力があれば確認を出し、「破棄」なら true を返す。key を渡すと、その入力だけを見る。
  confirmLeave(key?: string): Promise<boolean>;
  // prompt は、出している確認(なければ null)。answer は、そのボタン(true が「破棄」)。
  prompt: Readable<DiscardPrompt | null>;
  answer(discard: boolean): void;
}

interface Entry {
  dirty(): boolean;
  held(): boolean;
}

const lineOf = (key: string) => (key.split(":")[0] === "comment" ? commentLine : descriptionLine);

export function createDrafts(): Drafts {
  const entries = new Map<string, Entry>();
  const prompt = writable<DiscardPrompt | null>(null);
  // waiting は、いま出している確認の答えを待つ関数。asking は、そのあいだに重ねて聞かれたときに返す同じ Promise。
  let waiting = null as ((discard: boolean) => void) | null;
  let asking: Promise<boolean> | null = null;
  const dirtyKeys = (only?: string) =>
    [...entries]
      .filter(([key, e]) => (only === undefined || key === only) && e.dirty())
      .map(([key]) => key);

  return {
    register(key, dirty, held = dirty) {
      const entry = { dirty, held };
      entries.set(key, entry);
      // 同じ key で登録し直したあとに、前の解除が走っても、新しい登録は消さない。
      return () => {
        if (entries.get(key) === entry) entries.delete(key);
      };
    },
    anyHeld: () => [...entries.values()].some((e) => e.held()),
    anyDirty: () => dirtyKeys().length > 0,
    confirmLeave(key) {
      const keys = dirtyKeys(key);
      if (keys.length === 0) return Promise.resolve(true);
      asking ??= new Promise<boolean>((resolve) => {
        waiting = resolve;
        const lines = [...new Set(keys.map(lineOf))];
        prompt.set({ title: discardTitle, lines, ok: discardOk });
      });
      return asking;
    },
    answer(discard) {
      const done = waiting;
      waiting = null;
      asking = null;
      prompt.set(null);
      done?.(discard);
    },
    prompt: readonly(prompt),
  };
}
```

`web/packages/core/src/notesIndex.ts` に足す(`pnpm -C web fix` が並べ替える):

```ts
export {
  createDrafts,
  type DiscardPrompt,
  type Drafts,
  type LeaveGuard,
} from "./features/drafts/drafts";
```

- [ ] **Step 4: drafts のテストを通す**

Run: `pnpm -C web/packages/core exec vitest run src/features/drafts/drafts.test.ts`
Expected: PASS(6 件)

- [ ] **Step 5: ルーターの失敗するテストを書く**

まず `fakeHistory` を共有の場所へ移す。`web/packages/core/src/features/router/history.test-util.ts`(新規。`navigation.test.ts` の `fakeHistory` をそのまま `export` にしたもの):

```ts
import type { HistoryPort } from "../../port/history";

// fakeHistory は、積んだパスを持つ履歴の偽物。back は、ブラウザの戻るを押したときの動き。
export function fakeHistory(start: string) {
  const stack = [start];
  let index = 0;
  const listeners = new Set<() => void>();
  const history: HistoryPort = {
    path: () => stack[index] ?? "/",
    push(path) {
      stack.splice(index + 1, stack.length, path);
      index = stack.length - 1;
    },
    replace(path) {
      stack[index] = path;
    },
    onPop(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
  const back = () => {
    index -= 1;
    for (const l of listeners) l();
  };
  return { history, stack, back };
}
```

`navigation.test.ts` の先頭の import から `fakeHistory` の定義の終わり(`return { history, stack, back };\n}`)までを、次の 4 行に替える(`HistoryPort` の import は要らなくなる)。

```ts
import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { fakeHistory } from "./history.test-util";
import { createNavigation } from "./navigation";
```

`web/packages/core/src/features/router/navigationLeave.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { fakeHistory } from "./history.test-util";
import { createNavigation } from "./navigation";

// 書きかけがあるかと、確認への答えを決められる偽物。asked は、確認を出した回数。
function fakeGuard(dirty: boolean, answer: boolean) {
  const guard = {
    dirty,
    answer,
    asked: 0,
    anyDirty: () => guard.dirty,
    async confirmLeave() {
      guard.asked += 1;
      return guard.answer;
    },
  };
  return guard;
}

const settle = async () => {
  for (let i = 0; i < 5; i++) await Promise.resolve();
};
const tree = { name: "project", store: "work", project: "HH-P1", tab: "tree" } as const;
const treePath = "/s/work/p/HH-P1/tree";

describe("navigation with unsaved input", () => {
  it("moves at once and does not ask when nothing is dirty", () => {
    const { history } = fakeHistory("/");
    const guard = fakeGuard(false, false);
    const nav = createNavigation(history, guard);
    nav.go(tree);
    expect(get(nav.route)).toMatchObject({ name: "project" });
    expect(guard.asked).toBe(0);
  });

  it("asks first when something is dirty, and stays when canceled", async () => {
    const { history, stack } = fakeHistory("/");
    const guard = fakeGuard(true, false);
    const nav = createNavigation(history, guard);
    nav.go(tree);
    await settle();
    expect(guard.asked).toBe(1);
    expect(get(nav.route)).toEqual({ name: "home" });
    expect(stack).toEqual(["/"]);
  });

  it("moves once the input is discarded", async () => {
    const { history, stack } = fakeHistory("/");
    const nav = createNavigation(history, fakeGuard(true, true));
    nav.go(tree);
    await settle();
    expect(get(nav.route)).toMatchObject({ name: "project" });
    expect(stack).toEqual(["/", treePath]);
  });

  it("does not ask for replace (an internal address swap, not a move)", () => {
    const { history } = fakeHistory("/");
    const guard = fakeGuard(true, false);
    const nav = createNavigation(history, guard);
    nav.replace(tree);
    expect(get(nav.route)).toMatchObject({ name: "project" });
    expect(guard.asked).toBe(0);
  });

  it("puts the address back when the back button is canceled", async () => {
    const { history, stack, back } = fakeHistory("/");
    const guard = fakeGuard(false, false);
    const nav = createNavigation(history, guard);
    nav.start();
    nav.go(tree);
    guard.dirty = true;
    back();
    await settle();
    expect(guard.asked).toBe(1);
    expect(get(nav.route)).toMatchObject({ name: "project" });
    expect(history.path()).toBe(treePath);
    expect(stack).toEqual(["/", treePath]);
  });

  it("follows the back button once the input is discarded", async () => {
    const { history, back } = fakeHistory("/");
    const guard = fakeGuard(false, true);
    const nav = createNavigation(history, guard);
    nav.start();
    nav.go(tree);
    guard.dirty = true;
    back();
    await settle();
    expect(get(nav.route)).toEqual({ name: "home" });
  });
});
```

- [ ] **Step 6: 失敗を確かめる**

Run: `pnpm -C web/packages/core exec vitest run src/features/router/`
Expected: `navigationLeave.test.ts` が FAIL(`createNavigation` が第 2 引数を使わず、確認を出さないまま移る)。`navigation.test.ts` は PASS のまま(`fakeHistory` を移しただけ)。

- [ ] **Step 7: ルーターを実装する**

`web/packages/core/src/features/router/navigation.ts` を次にする(`go`・`start` が確認を聞く。形は変えない)。

```ts
import type { HistoryPort } from "../../port/history";
import { type Readable, readonly, writable } from "../../reactive/store";
import type { LeaveGuard } from "../drafts/drafts";
import { formatRoute, parseRoute, type Route } from "./route";

// 画面の移り方。go は履歴に積み、replace は積まない(01「画面のアドレス」の積む・積まない)。
// guard があり、書きかけがあるときは、go と戻る・進むは「入力を破棄」の確認が「破棄」のときだけ移る(02「説明」)。
export interface Navigation {
  route: Readable<Route>;
  go(route: Route): void;
  replace(route: Route): void;
  // start は、戻る・進むを受け始め、止める関数を返す。
  start(): () => void;
}

export function createNavigation(history: HistoryPort, guard?: LeaveGuard): Navigation {
  const route = writable<Route>(parseRoute(history.path()));
  // shown は、いま出している画面のアドレス。戻る・進むを断ったとき、ここへ戻す。
  let shown = history.path();
  const show = (path: string) => {
    shown = path;
    route.set(parseRoute(path));
  };
  return {
    route: readonly(route),
    go(next) {
      const path = formatRoute(next);
      // いまと同じ画面へは移らない(履歴を積まず、画面を移ったときの動きもしない)。
      if (path === history.path()) return;
      const move = () => {
        history.push(path);
        show(path);
      };
      if (!guard?.anyDirty()) return move();
      void guard.confirmLeave().then((ok) => {
        if (ok) move();
      });
    },
    replace(next) {
      const path = formatRoute(next);
      history.replace(path);
      show(path);
    },
    start() {
      return history.onPop(() => {
        const path = history.path();
        if (!guard?.anyDirty()) return show(path);
        // ブラウザのアドレスはもう変わっている。断ったら、いまの画面のアドレスを積み直して留まる。
        void guard.confirmLeave().then((ok) => {
          if (ok) show(path);
          else history.push(shown);
        });
      });
    },
  };
}
```

- [ ] **Step 8: ルーターのテストを通す**

Run: `pnpm -C web/packages/core exec vitest run src/features/router/`
Expected: PASS(`navigation.test.ts` 4 件・`navigationLeave.test.ts` 6 件・ほかの router のテスト)

- [ ] **Step 9: 詳細パネルの失敗するテストを書く**

`web/packages/core/src/features/items/detailLeave.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createDetail } from "./detail";
import { fakeItemsPort, item } from "./fixtures.test-util";
import { fakeWorkspaces } from "./settings.test-util";

// 入力中のものがあるとき、別のアイテムを開く・パネルを閉じるは、確認が「破棄」のときだけ進む(02「説明」)。
async function setup(answer: boolean) {
  const asked = { n: 0 };
  const catalogPort = fakeCatalogPort();
  const catalog = createCatalog(catalogPort);
  await catalog.load();
  const epic = item("S1", "S1-P1", 1, "Epic", null, "認証");
  const task = item("S1", "S1-P1", 2, "Task", epic.id, "画面");
  const detail = createDetail({
    catalog,
    items: fakeItemsPort([epic, task]),
    toasts: createToasts({ set: () => 0, clear: () => {} }),
    workspaces: fakeWorkspaces(),
    catalogPort,
    onChanged: () => {},
    leave: async () => {
      asked.n += 1;
      return answer;
    },
  });
  return { detail, asked, epic, task };
}

describe("detail with unsaved input", () => {
  it("asks before another item opens, and keeps the open one when canceled", async () => {
    const { detail, asked, epic, task } = await setup(false);
    await detail.open("S1", task.id);
    await detail.open("S1", epic.id);
    expect(asked.n).toBe(1);
    expect(get(detail.view)?.item.id).toBe(task.id);
  });

  it("opens the other item once the input is discarded", async () => {
    const { detail, epic, task } = await setup(true);
    await detail.open("S1", task.id);
    await detail.open("S1", epic.id);
    expect(get(detail.view)?.item.id).toBe(epic.id);
  });

  it("does not ask for the first item or for the same item again", async () => {
    const { detail, asked, task } = await setup(false);
    await detail.open("S1", task.id);
    await detail.open("S1", task.id);
    expect(asked.n).toBe(0);
    expect(get(detail.view)?.item.id).toBe(task.id);
  });

  it("requestClose closes on yes and stays open on no; close never asks", async () => {
    const no = await setup(false);
    await no.detail.open("S1", no.task.id);
    await no.detail.requestClose();
    expect(get(no.detail.view)).not.toBeNull();
    no.detail.close();
    expect(get(no.detail.view)).toBeNull();
    expect(no.asked.n).toBe(1);

    const yes = await setup(true);
    await yes.detail.open("S1", yes.task.id);
    await yes.detail.requestClose();
    expect(get(yes.detail.view)).toBeNull();
  });
});
```

- [ ] **Step 10: 失敗を確かめる**

Run: `pnpm -C web/packages/core exec vitest run src/features/items/detailLeave.test.ts`
Expected: FAIL(`requestClose is not a function`、別のアイテムが確認なしで開く)

- [ ] **Step 11: 詳細パネルを実装する**

`web/packages/core/src/features/items/detail.ts` を 3 か所変える(256 行 → 270 行台。ここから先の足しは新しいファイルへ)。

`Detail` の `close(): void;` を次にする。

```ts
  close(): void;
  // requestClose は、入力中のものがあれば「入力を破棄」の確認をしてから閉じる(「×」・Esc)。断ったら開いたまま。
  // close は確認しない(画面の移り変わりと、反映でアイテムがなくなったときの閉じ方)。
  requestClose(): Promise<void>;
```

`DetailDeps` の `onChanged(): void;` の次に足す。

```ts
  // leave は、入力中のものがあるとき、捨ててよいかを聞く(別のアイテムを開く・パネルを閉じる)。ないときは常に true。
  leave?: () => Promise<boolean>;
```

返すオブジェクトの `open: (storeId, itemId) => load(storeId, itemId),` を次にし、`close()` の次に `requestClose` を足す。

```ts
    async open(storeId, itemId) {
      // 別のアイテムが開いているときは、入力中のものを捨ててよいかを先に聞く(02「説明」)。
      const other = target !== null && (target.storeId !== storeId || target.itemId !== itemId);
      if (other && deps.leave && !(await deps.leave())) return;
      await load(storeId, itemId);
    },
    close() {
      latest += 1;
      target = null;
      contexts = new Map();
      view.set(null);
      full.set(false);
    },
    async requestClose() {
      if (!deps.leave || (await deps.leave())) this.close();
    },
```

(`close()` は元のままで、`requestClose` の `this.close()` で呼ぶ。`requestClose` を取り出して渡しても動くよう、画面側は `() => void detail.requestClose()` と書く。)

`web/packages/core/src/notesIndex.ts` は Task 6 の Step 3 で足し済み。

- [ ] **Step 12: core のテストを通す**

Run: `pnpm -C web/packages/core exec vitest run`
Expected: PASS(core のすべて。`detail.test.ts` は `leave` を渡さないので変わらない)

- [ ] **Step 13: app に結び付ける**

`web/app/src/drafts.ts`(新規):

```ts
import { createDrafts } from "@hedgehug/core";

// 書きかけの登録簿。wire.ts(画面の移り方・詳細パネル)と tab.ts(反映を待つ条件)の両方が使うので、
// wire.ts の外に置く(tab.ts は wire.ts から読まれるので、tab.ts が wire.ts を読むと循環する)。
export const drafts = createDrafts();
```

`web/app/src/tab.ts`: 先頭の `// このタブの状態を…` の次の行に import を足し、`holdReasons` に 1 つ足す。

```ts
import { drafts } from "./drafts";
```

```ts
// holdReasons は、自動の同期と画面への反映を待つ条件(06「自動で同期する」。C2 画面の決定 S9)。
// ドラッグは、それを作るトラックがここに足す。
export const holdReasons: (() => boolean)[] = [
  () => document.querySelector('[aria-modal="true"]') !== null,
  () => document.querySelector('[role="menu"]') !== null,
  () =>
    document.activeElement?.closest(
      "input:not([type='checkbox']):not([type='radio']), textarea, select, [contenteditable='true']",
    ) != null,
  // 説明・コメントの書きかけ(保存していない入力が空でないもの)。フォーカスが外れても待つ(A4a-21)。
  () => drafts.anyHeld(),
];
```

`web/app/src/wire.ts`:

`import { held, online as isOnline, tabId, visible } from "./tab";` の前に足す。

```ts
import { drafts } from "./drafts";
```

`export const navigation = createNavigation(browserHistory());` を替える。

```ts
export const navigation = createNavigation(browserHistory(), drafts);
```

`createDetail({` の引数に足す(`onChanged` の次の行)。

```ts
  // 別のアイテムを開く・パネルを閉じるとき、書きかけがあれば「入力を破棄」を出す(02「説明」)。
  leave: () => drafts.confirmLeave(),
```

`web/app/src/screens/notes/DiscardDialog.svelte`(新規):

```svelte
<script lang="ts">
  import { ConfirmDialog } from "@hedgehug/ui";
  import { drafts } from "../../drafts";

  // 「入力を破棄」の確認(02「説明」)。出す条件と答えの扱いは core の drafts が持つ。「キャンセル」は
  // 入力欄に戻る(確認を呼んだ側が、false の答えで入力欄にフォーカスを戻す)。
  const prompt = drafts.prompt;
</script>

{#if $prompt}
  <ConfirmDialog
    title={$prompt.title}
    lines={$prompt.lines}
    ok={$prompt.ok}
    danger
    onok={() => drafts.answer(true)}
    oncancel={() => drafts.answer(false)}
  />
{/if}
```

`web/app/src/App.svelte`: import を足し、`<CreateItemDialog />` の次の行に置く。

```svelte
  import DiscardDialog from "./screens/notes/DiscardDialog.svelte";
```

```svelte
<CreateItemDialog />
<DiscardDialog />
```

`web/app/src/screens/item/DetailPanel.svelte`: `<SidePanel … onclose={detail.close}>` の `onclose` を替える。

```svelte
<SidePanel
  open={$view !== null}
  full={$full}
  label="詳細パネル"
  onclose={() => void detail.requestClose()}
>
```

`web/app/src/screens/item/DetailHeader.svelte`: `<IconButton label="閉じる" size="sm" onclick={detail.close}>×</IconButton>` を替える。

```svelte
<IconButton label="閉じる" size="sm" onclick={() => void detail.requestClose()}>×</IconButton>
```

- [ ] **Step 14: 検査して、コミットする**

Run: `pnpm -C web fix && pnpm -C web lint && pnpm -C web check && pnpm -C web test && pnpm -C web size && pnpm -C web deps`
Expected: すべて成功。画面の動き(入力欄から確認を出す)は、Task 7 の e2e で確かめる。

```bash
git add web/packages/core/src/features/drafts web/packages/core/src/features/router web/packages/core/src/features/items/detail.ts web/packages/core/src/features/items/detailLeave.test.ts web/packages/core/src/notesIndex.ts web/app/src/drafts.ts web/app/src/tab.ts web/app/src/wire.ts web/app/src/screens/notes/DiscardDialog.svelte web/app/src/App.svelte web/app/src/screens/item/DetailPanel.svelte web/app/src/screens/item/DetailHeader.svelte
git commit -m "feat(web): 書きかけの登録と「入力を破棄」の確認、反映を待つ条件を足す(A4a-21・A4a-22)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 7: 詳細パネルの説明の区画

アイテムの詳細パネルに説明の区画を足す(表示・編集・保存・同じ行のトースト)。プロジェクトの概要(Task 19)も同じ `DescriptionSection.svelte` を `target.kind === "project"` で使う。説明の読み書きは Task 3 の API。

**Files:**
- Create: `web/packages/ui/src/primitives/Textarea.svelte`
- Test: `web/packages/ui/src/primitives/textarea.test.ts`
- Modify: `web/packages/ui/src/index.ts`
- Create: `web/packages/core/src/port/notes.ts`
- Create: `web/packages/core/src/features/notes/description.ts`
- Create: `web/packages/core/src/features/notes/fixtures.test-util.ts`
- Test: `web/packages/core/src/features/notes/description.test.ts`
- Modify: `web/packages/core/src/notesIndex.ts`
- Create: `web/packages/api/src/notes.ts`
- Test: `web/packages/api/src/notes.test.ts`
- Modify: `web/packages/api/src/index.ts`
- Modify: `web/app/src/wire.ts`
- Create: `web/app/src/screens/notes/DescriptionSection.svelte`
- Modify: `web/app/src/screens/item/DetailPanel.svelte`
- Test: `web/e2e/description.spec.ts`

**Interfaces:**
- Consumes: Task 3 の `get-item-description`・`describe-item`・`get-project-description`・`describe-project`(orval が作る `getItemDescription(store, item)`・`describeItem(store, item, { base, text })`・`getProjectDescription(store, project)`・`describeProject(store, project, { base, text })`。応答は `{ text }`・`{ text, conflict }`)、Task 5 の `tokenize`・`isMemberOf`・`Markdown.svelte`、Task 6 の `drafts`・`Drafts.confirmLeave(key)`
- Produces(core, `port/notes.ts`): `interface NoteTarget { kind: "item" | "project"; storeId: string; id: string }`、`interface DescribeResult { text: string; conflict: boolean }`、`interface NotesPort { description(target: NoteTarget): Promise<string>; describe(target: NoteTarget, base: string, text: string): Promise<DescribeResult> }`。コメント・作業時間・関連・履歴の Task が、自分のメソッドをこの `NotesPort` に足す(その Task が先にこのファイルを作っていなければ、ここを作る Task 7 のあとに足す)
- Produces(core, `description.ts`): `createDescriptionEditor(deps: DescriptionDeps): DescriptionEditor`、`DescriptionState`、`descriptionConflictMessage`
- Produces(api): `createNotesPort(): NotesPort`(`web/packages/api/src/notes.ts`。コメントなどのメソッドは各 Task が足す)
- Produces(ui): `Textarea.svelte`(props `{ value?, label, rows?, placeholder?, disabled?, title?, focusOnMount?, onkeydown? }`、`focus()` を公開)。Task 8・10 も使う
- Produces(app): `DescriptionSection.svelte`(props `{ target: NoteTarget; readonly: boolean }`)、`wire.ts` の `notes`(`NotesPort`)と `listMembers`

**決めたこと:**
- 書きかけの「保存してあるものと違う」は、**編集を始めたときの説明(`base`)** と比べる。反映で他の人の変更を受け取っても、打っていない入力欄は書きかけにならない(02「説明」は「保存してある説明」と書くが、打っていない入力が確認や反映の待ちを起こさないようにする)
- 前と同じ(`input` が `base` か保存してある説明と同じ)なら、保存せずに入力欄を閉じる(02「保存したとき、前と同じなら何も保存しない」)
- 入力欄の中の Esc は、書きかけ(違う入力)があるときだけ確認を出す。違わないときは、ふつうの Esc と同じにパネルを閉じる方へ進む
- 確認の「破棄」は入力欄を閉じる(入力を捨てる)。パネルは閉じない。「キャンセル」は入力欄にフォーカスを戻す
- 読み終わるまで、区画の中身は出さない(「説明を追加」を一瞬出さないため)
- 保存が断られたとき(`Refused`)は、エラーのトーストを出し、入力欄と入力を残す
- 閲覧のみの Store では、説明の部分を押せなくし、「説明を追加」と編集のきっかけに「閲覧のみの Store です」を添える

- [ ] **Step 1: `Textarea` の失敗するテストを書く**

`web/packages/ui/src/primitives/textarea.test.ts`:

```ts
import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import Textarea from "./Textarea.svelte";

describe("Textarea", () => {
  it("is named by its label, takes the rows, and puts the caret at the end when it takes focus", () => {
    render(Textarea, { props: { label: "説明", rows: 8, value: "本文", focusOnMount: true } });
    const box = screen.getByRole("textbox", { name: "説明" }) as HTMLTextAreaElement;
    expect(box).toHaveAttribute("rows", "8");
    expect(box).toHaveValue("本文");
    expect(box).toHaveFocus();
    expect(box.selectionStart).toBe(2);
  });

  it("does not take focus unless asked", () => {
    render(Textarea, { props: { label: "説明" } });
    expect(screen.getByRole("textbox", { name: "説明" })).not.toHaveFocus();
  });

  it("reports keydown", async () => {
    const onkeydown = vi.fn();
    render(Textarea, { props: { label: "説明", onkeydown } });
    await fireEvent.keyDown(screen.getByRole("textbox", { name: "説明" }), { key: "Escape" });
    expect(onkeydown).toHaveBeenCalledWith(expect.objectContaining({ key: "Escape" }));
  });

  it("is disabled with the note", () => {
    render(Textarea, { props: { label: "説明", disabled: true, title: "閲覧のみの Store です" } });
    const box = screen.getByRole("textbox", { name: "説明" });
    expect(box).toBeDisabled();
    expect(box).toHaveAttribute("title", "閲覧のみの Store です");
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web/packages/ui exec vitest run src/primitives/textarea.test.ts`
Expected: FAIL(`Failed to resolve import "./Textarea.svelte"`)

- [ ] **Step 3: `Textarea` を実装する**

`web/packages/ui/src/primitives/Textarea.svelte`:

```svelte
<script lang="ts">
  interface Props {
    value?: string;
    label: string;
    rows?: number;
    placeholder?: string;
    disabled?: boolean;
    title?: string;
    // focusOnMount は、出たときにフォーカスを置き、キャレットを末尾に置く(Input と同じ理由で autofocus は使わない)。
    focusOnMount?: boolean;
    onkeydown?: (event: KeyboardEvent) => void;
  }

  let {
    value = $bindable(""),
    label,
    rows = 4,
    placeholder,
    disabled = false,
    title,
    focusOnMount = false,
    onkeydown,
  }: Props = $props();

  let element: HTMLTextAreaElement | undefined = $state();

  $effect(() => {
    if (!focusOnMount || !element) return;
    element.focus();
    element.setSelectionRange(element.value.length, element.value.length);
  });

  // focus は、確認で「キャンセル」を押したあとに、入力欄へフォーカスを戻す。
  export function focus() {
    element?.focus();
  }
</script>

<textarea
  bind:this={element}
  class="textarea"
  aria-label={label}
  {rows}
  {placeholder}
  {disabled}
  {title}
  {onkeydown}
  bind:value
></textarea>

<style>
  .textarea {
    border: 1px solid var(--line-input);
    border-radius: 7px;
    background: var(--surface);
    padding: 6px 10px;
    min-width: 0;
    resize: vertical;
  }
  .textarea:focus {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
    border-color: transparent;
  }
  .textarea:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
</style>
```

`web/packages/ui/src/index.ts` の `Switch` の行の次に足す。

```ts
export { default as Textarea } from "./primitives/Textarea.svelte";
```

Run: `pnpm -C web/packages/ui exec vitest run src/primitives/textarea.test.ts`
Expected: PASS(4 件)

- [ ] **Step 4: core の失敗するテストを書く**

`web/packages/core/src/port/notes.ts`(新規):

```ts
// 説明・コメント・作業時間・関連・履歴の口(アイテムとプロジェクト)。実装は api パッケージにある。
// 失敗は、断りなら Refused、ないものなら NotFound、それ以外はそのまま投げる。
// 区画ごとのタスクが、自分のメソッドをここに足す。

// NoteTarget は、対象のアイテムかプロジェクト(内部 ID)。
export interface NoteTarget {
  kind: "item" | "project";
  storeId: string;
  id: string;
}

export interface DescribeResult {
  // text は、保存したあとの説明(行マージの結果)。
  text: string;
  // conflict は、同じ行を別の人も変えていて、両方の版を残したとき。
  conflict: boolean;
}

export interface NotesPort {
  // description は、保存してある説明(なければ "")。
  description(target: NoteTarget): Promise<string>;
  // describe は、説明を保存する。base は編集を始めたときの説明(行マージの共通の祖先)。
  describe(target: NoteTarget, base: string, text: string): Promise<DescribeResult>;
}
```

`web/packages/core/src/features/notes/fixtures.test-util.ts`(新規。ほかの区画のタスクは、同じ偽物に自分のメソッドを足す):

```ts
import { Refused } from "../../model/errors";
import type { NotesPort, NoteTarget } from "../../port/notes";

export const keyOf = (t: NoteTarget) => `${t.kind}/${t.storeId}/${t.id}`;

export interface FakeNotes extends NotesPort {
  // texts は、対象ごとの保存してある説明(keyOf の形)。
  texts: Map<string, string>;
  // describeCalls は、describe に渡された base と text の記録。
  describeCalls: { base: string; text: string }[];
  // conflictNext は、次の describe を、merged を結果にして conflict: true で返させる(1 回だけ)。
  conflictNext(merged: string): void;
  // refuseNext は、次の describe を、この文言の断りにする(1 回だけ)。
  refuseNext(message: string): void;
}

// テスト用の説明の口。メモリ上に説明を持つ。
export function fakeNotesPort(initial: Record<string, string> = {}): FakeNotes {
  const texts = new Map(Object.entries(initial));
  const describeCalls: { base: string; text: string }[] = [];
  let merged: string | null = null;
  let refusal = "";
  return {
    texts,
    describeCalls,
    conflictNext(text) {
      merged = text;
    },
    refuseNext(message) {
      refusal = message;
    },
    async description(t) {
      return texts.get(keyOf(t)) ?? "";
    },
    async describe(t, base, text) {
      describeCalls.push({ base, text });
      if (refusal !== "") {
        const message = refusal;
        refusal = "";
        throw new Refused(message);
      }
      const saved = merged ?? text;
      const conflict = merged !== null;
      merged = null;
      texts.set(keyOf(t), saved);
      return { text: saved, conflict };
    },
  };
}
```

`web/packages/core/src/features/notes/description.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { NotFound } from "../../model/errors";
import { get } from "../../reactive/store";
import { createToasts } from "../toasts/toasts";
import { createDescriptionEditor, descriptionConflictMessage } from "./description";
import { fakeNotesPort, keyOf } from "./fixtures.test-util";

const target = { kind: "item", storeId: "S1", id: "I1" } as const;
const first = "## 手順\n@taro を見る";

function setup() {
  const notes = fakeNotesPort({ [keyOf(target)]: first });
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const editor = createDescriptionEditor({
    port: notes,
    toasts,
    target: () => target,
    members: async () => [{ login: "taro", name: "太郎" }],
  });
  const shown = () => get(toasts.list).map((t) => t.text);
  return { notes, toasts, editor, shown };
}

describe("description editor", () => {
  it("shows nothing until it has loaded, then the text and its Markdown with member mentions", async () => {
    const { editor } = setup();
    expect(get(editor.state)).toMatchObject({ loaded: false, text: "", blocks: [] });
    await editor.load();
    const s = get(editor.state);
    expect(s.loaded).toBe(true);
    expect(s.text).toBe(first);
    expect(s.blocks).toEqual([
      { kind: "heading", level: 2, inlines: [{ kind: "text", text: "手順" }] },
      {
        kind: "paragraph",
        inlines: [
          { kind: "mention", login: "taro" },
          { kind: "text", text: " を見る" },
        ],
      },
    ]);
  });

  it("is dirty when the input differs from where editing began, and held only when it is not empty", async () => {
    const { editor } = setup();
    editor.start(); // 読む前は始まらない
    expect(get(editor.state).editing).toBe(false);
    await editor.load();
    editor.start();
    expect([editor.dirty(), editor.held()]).toEqual([false, false]);
    editor.setInput("変えた");
    expect([editor.dirty(), editor.held()]).toEqual([true, true]);
    editor.setInput("");
    expect([editor.dirty(), editor.held()]).toEqual([true, false]);
    editor.setInput(first);
    expect([editor.dirty(), editor.held()]).toEqual([false, false]);
  });

  it("saves with the text where editing began as the common ancestor", async () => {
    const { editor, notes, shown } = setup();
    await editor.load();
    editor.start();
    editor.setInput("## 手順\n書き換えた");
    await editor.save();
    expect(notes.describeCalls).toEqual([{ base: first, text: "## 手順\n書き換えた" }]);
    expect(get(editor.state)).toMatchObject({ editing: false, text: "## 手順\n書き換えた" });
    expect(shown()).toEqual([]);
  });

  it("saves nothing and closes when the input is the same as before", async () => {
    const { editor, notes } = setup();
    await editor.load();
    editor.start();
    await editor.save();
    expect(notes.describeCalls).toEqual([]);
    expect(get(editor.state).editing).toBe(false);
  });

  it("shows the merged text and the toast when both versions were kept", async () => {
    const { editor, notes, shown } = setup();
    await editor.load();
    editor.start();
    editor.setInput("自分の版");
    notes.conflictNext("-----\n自分の版\n-----\n他の人の版\n-----");
    await editor.save();
    expect(get(editor.state).text).toBe("-----\n自分の版\n-----\n他の人の版\n-----");
    expect(shown()).toEqual([descriptionConflictMessage]);
    expect(descriptionConflictMessage).toBe("説明の同じ箇所が同時に編集されたため、両方を残しました");
  });

  it("keeps the input and the editor open when the save is refused", async () => {
    const { editor, notes, toasts } = setup();
    await editor.load();
    editor.start();
    editor.setInput("変えた");
    notes.refuseNext("閲覧のみの Store です");
    await editor.save();
    expect(get(toasts.list)[0]).toMatchObject({ text: "閲覧のみの Store です", error: true });
    expect(get(editor.state)).toMatchObject({ editing: true, input: "変えた", saving: false });
  });

  it("does not rewrite the input when the saved text changes meanwhile (reflection)", async () => {
    const { editor, notes } = setup();
    await editor.load();
    editor.start();
    editor.setInput("入力中");
    notes.texts.set(keyOf(target), "他の人の説明");
    await editor.load();
    expect(get(editor.state)).toMatchObject({ text: "他の人の説明", input: "入力中", base: first });
    // 保存では、編集を始めたときの説明が祖先のまま。
    await editor.save();
    expect(notes.describeCalls[0]?.base).toBe(first);
  });

  it("does not make an untouched input dirty when the saved text changes", async () => {
    const { editor, notes } = setup();
    await editor.load();
    editor.start();
    notes.texts.set(keyOf(target), "他の人の説明");
    await editor.load();
    expect([editor.dirty(), editor.held()]).toEqual([false, false]);
  });

  it("throws the input away on cancel without saving", async () => {
    const { editor, notes } = setup();
    await editor.load();
    editor.start();
    editor.setInput("捨てる");
    editor.cancel();
    expect(get(editor.state)).toMatchObject({ editing: false, input: "", base: "" });
    expect(editor.dirty()).toBe(false);
    expect(notes.describeCalls).toEqual([]);
  });

  it("ignores an item that is gone (the panel closes through the reflection)", async () => {
    const { editor, notes } = setup();
    notes.description = async () => {
      throw new NotFound("gone");
    };
    await editor.load();
    expect(get(editor.state).loaded).toBe(false);
  });
});
```

- [ ] **Step 5: 失敗を確かめる**

Run: `pnpm -C web/packages/core exec vitest run src/features/notes/description.test.ts`
Expected: FAIL(`Failed to resolve import "./description"`)

- [ ] **Step 6: core を実装する**

`web/packages/core/src/features/notes/description.ts`:

```ts
import { NotFound, Refused } from "../../model/errors";
import type { Member } from "../../model/member";
import type { DescribeResult, NotesPort, NoteTarget } from "../../port/notes";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import { type Block, isMemberOf, tokenize } from "../markdown/markdown";
import type { Toasts } from "../toasts/toasts";

// 説明の区画の状態(02「説明」。アイテムとプロジェクトで同じ)。自動保存はない。入力中は、反映で
// 保存してある説明が変わっても入力欄を書き換えない。保存では、編集を始めたときの説明(base)を
// 共通の祖先として送り、行マージはサーバーが行う(A4a-3)。

export const descriptionConflictMessage = "説明の同じ箇所が同時に編集されたため、両方を残しました";

export interface DescriptionState {
  // loaded は、保存してある説明を読み終えたか(読むまで区画の中身を出さない)。
  loaded: boolean;
  // text は、保存してある説明。blocks は、その Markdown(メンションはメンバーのものだけ)。
  text: string;
  blocks: Block[];
  editing: boolean;
  // input は入力欄の中身、base は編集を始めたときの説明。
  input: string;
  base: string;
  saving: boolean;
}

export interface DescriptionDeps {
  port: NotesPort;
  toasts: Toasts;
  // target は、いまの対象。画面への反映で対象の値が新しくなるので、呼ぶたびに引く。
  target(): NoteTarget;
  members(storeId: string): Promise<Member[]>;
}

export interface DescriptionEditor {
  state: Readable<DescriptionState>;
  // load は、保存してある説明を読む(開いたとき・画面への反映のとき)。編集中の入力欄は書き換えない。
  load(): Promise<void>;
  start(): void;
  setInput(text: string): void;
  cancel(): void;
  // save は、説明を保存して入力欄を閉じる。前と同じなら何も保存せずに閉じる。断られたら入力欄を残す。
  save(): Promise<void>;
  // dirty は、入力が編集を始めたときと違うか。held は、違い、かつ空でないか(drafts に渡す)。
  dirty(): boolean;
  held(): boolean;
}

const idle: DescriptionState = {
  loaded: false,
  text: "",
  blocks: [],
  editing: false,
  input: "",
  base: "",
  saving: false,
};

export function createDescriptionEditor(deps: DescriptionDeps): DescriptionEditor {
  const state = writable<DescriptionState>(idle);
  let members: Member[] = [];
  // latest は、読み直しの世代。保存や新しい読み直しのあとに、古い読み取りの結果を捨てる。
  let latest = 0;
  const shown = (text: string) => ({ text, blocks: tokenize(text, isMemberOf(members)) });

  const dirty = () => {
    const s = get(state);
    return s.editing && s.input !== s.base;
  };
  const cancel = () =>
    state.update((s) => ({ ...s, editing: false, input: "", base: "", saving: false }));

  async function load() {
    latest += 1;
    const mine = latest;
    const t = deps.target();
    try {
      const [text, list] = await Promise.all([
        deps.port.description(t),
        deps.members(t.storeId).catch((): Member[] => []),
      ]);
      if (mine !== latest) return;
      members = list;
      state.update((s) => ({ ...s, loaded: true, ...shown(text) }));
    } catch (e) {
      // 開いている対象がなくなったら、パネルは反映で閉じる。ここでは何も出さない。
      if (!(e instanceof NotFound)) throw e;
    }
  }

  async function save() {
    const s = get(state);
    if (!s.editing || s.saving) return;
    // 前と同じ(編集を始めたときのまま、または保存してあるものと同じ)なら、何も保存しない。
    if (s.input === s.base || s.input === s.text) return cancel();
    state.update((x) => ({ ...x, saving: true }));
    let result: DescribeResult;
    try {
      result = await deps.port.describe(deps.target(), s.base, s.input);
    } catch (e) {
      state.update((x) => ({ ...x, saving: false }));
      if (!(e instanceof Refused)) throw e;
      deps.toasts.show(e.message, { error: true });
      return;
    }
    latest += 1;
    state.update((x) => ({ ...x, ...shown(result.text), editing: false, input: "", base: "", saving: false }));
    if (result.conflict) deps.toasts.show(descriptionConflictMessage);
  }

  return {
    state: readonly(state),
    load,
    start() {
      const s = get(state);
      if (!s.loaded || s.editing) return;
      state.update((x) => ({ ...x, editing: true, input: s.text, base: s.text }));
    },
    setInput: (text) => state.update((s) => ({ ...s, input: text })),
    cancel,
    save,
    dirty,
    held: () => dirty() && get(state).input !== "",
  };
}
```

`web/packages/core/src/notesIndex.ts` に足す(`pnpm -C web fix` が並べ替える):

```ts
export {
  createDescriptionEditor,
  type DescriptionDeps,
  type DescriptionEditor,
  type DescriptionState,
  descriptionConflictMessage,
} from "./features/notes/description";
export type { DescribeResult, NotesPort, NoteTarget } from "./port/notes";
```

- [ ] **Step 7: core のテストを通す**

Run: `pnpm -C web/packages/core exec vitest run src/features/notes/description.test.ts`
Expected: PASS(10 件)

- [ ] **Step 8: api の失敗するテストを書く**

`web/packages/api/src/notes.test.ts`:

```ts
import { Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createNotesPort } from "./notes";

afterEach(() => {
  vi.unstubAllGlobals();
});

// respond は、fetch を 1 つの応答に差し替え、呼び出しを返す。
function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async () =>
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetch);
  setToken("tok");
  return fetch;
}

const call = (fetch: ReturnType<typeof respond>) =>
  fetch.mock.calls[0] as unknown as [string, RequestInit];

const item = { kind: "item", storeId: "S1", id: "I1" } as const;
const project = { kind: "project", storeId: "S1", id: "P1" } as const;

describe("notes port: description", () => {
  it("reads an item's description", async () => {
    const fetch = respond(200, { text: "## 手順" });
    expect(await createNotesPort().description(item)).toBe("## 手順");
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/description");
  });

  it("reads a project's description", async () => {
    const fetch = respond(200, { text: "" });
    expect(await createNotesPort().description(project)).toBe("");
    expect(call(fetch)[0]).toBe("/api/stores/S1/projects/P1/description");
  });

  it("saves with the base and the text, and returns the merged text and the conflict", async () => {
    const fetch = respond(200, { text: "合わせた", conflict: true });
    const result = await createNotesPort().describe(item, "前", "後");
    const [url, init] = call(fetch);
    expect(url).toBe("/api/stores/S1/items/I1/description");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(String(init.body))).toEqual({ base: "前", text: "後" });
    expect(result).toEqual({ text: "合わせた", conflict: true });
  });

  it("saves a project's description at its own address", async () => {
    const fetch = respond(200, { text: "後", conflict: false });
    await createNotesPort().describe(project, "前", "後");
    expect(call(fetch)[0]).toBe("/api/stores/S1/projects/P1/description");
  });

  it("turns a 422 into a refusal with the server's message", async () => {
    respond(422, { detail: "閲覧のみの Store です" });
    await expect(createNotesPort().describe(item, "", "x")).rejects.toThrow(Refused);
  });
});
```

- [ ] **Step 9: 失敗を確かめる**

Run: `pnpm -C web/packages/api exec vitest run src/notes.test.ts`
Expected: FAIL(`Failed to resolve import "./notes"`)

- [ ] **Step 10: api を実装する**

`web/packages/api/src/notes.ts`:

```ts
import type { NotesPort } from "@hedgehug/core";
import { call } from "./errors";
import {
  describeItem,
  describeProject,
  getItemDescription,
  getProjectDescription,
} from "./generated";

// 説明・コメント・作業時間・関連・履歴の口の実装。区画ごとのタスクが、自分のメソッドをここに足す。
export function createNotesPort(): NotesPort {
  return {
    description: (t) =>
      call(async () => {
        const read =
          t.kind === "item"
            ? await getItemDescription(t.storeId, t.id)
            : await getProjectDescription(t.storeId, t.id);
        return read.text;
      }),
    describe: (t, base, text) =>
      call(async () => {
        const body = { base, text };
        const saved =
          t.kind === "item"
            ? await describeItem(t.storeId, t.id, body)
            : await describeProject(t.storeId, t.id, body);
        return { text: saved.text, conflict: saved.conflict };
      }),
  };
}
```

`web/packages/api/src/index.ts` の `createItemsPort` の行の次に足す。

```ts
export { createNotesPort } from "./notes";
```

Run: `pnpm -C web/packages/api exec vitest run src/notes.test.ts`
Expected: PASS(5 件)。`getItemDescription` などが `generated` にないと型で落ちる(Task 3 で `make generate` と `pnpm -C web gen` を済ませていること)。

- [ ] **Step 11: app に結び付ける**

`web/app/src/wire.ts` の import に足す(`createItemsPort` の行の次)。

```ts
  createNotesPort,
```

`const catalogPort = createCatalogPort();` の次の行に足す。

```ts
// 詳細パネルとプロジェクトの概要の、説明・コメントなどの口と、メンションの表示に使うメンバーの一覧。
export const notes = createNotesPort();
export const listMembers = (storeId: string) => catalogPort.listMembers(storeId);
```

`web/app/src/screens/notes/DescriptionSection.svelte`(新規):

```svelte
<script lang="ts">
  import { createDescriptionEditor, type NoteTarget, readOnlyNote } from "@hedgehug/core";
  import { Button, Markdown, Textarea } from "@hedgehug/ui";
  import { drafts } from "../../drafts";
  import { listMembers, notes, toasts } from "../../wire";

  // 説明の区画(02「説明」)。アイテムの詳細パネルとプロジェクトの概要の両方で使う。状態と規則は core の
  // createDescriptionEditor が持ち、ここは結び付けと見た目だけ。
  interface Props {
    target: NoteTarget;
    readonly: boolean;
  }

  let { target, readonly }: Props = $props();

  const editor = createDescriptionEditor({
    port: notes,
    toasts,
    target: () => target,
    members: listMembers,
  });
  const view = editor.state;
  const note = $derived(readonly ? readOnlyNote : undefined);
  let area: ReturnType<typeof Textarea> | undefined = $state();

  // 開いたとき、画面への反映で target が新しくなるたびに、保存してある説明を読み直す(編集中の入力欄は書き換えない)。
  $effect(() => {
    void target;
    void editor.load();
  });

  // 書きかけとして登録する(確認と、反映を待つ条件)。key の "description:" で確認の本文が決まる。
  $effect(() => drafts.register(`description:${target.kind}/${target.id}`, editor.dirty, editor.held));

  async function onkeydown(event: KeyboardEvent) {
    if (event.isComposing) return;
    if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      void editor.save();
      return;
    }
    // 入力欄の中の Esc は、違う入力があるときだけ確認する。確認の Esc がパネルを閉じる Esc と重ならないよう止める。
    if (event.key !== "Escape" || !editor.dirty()) return;
    event.stopPropagation();
    if (await drafts.confirmLeave(`description:${target.kind}/${target.id}`)) editor.cancel();
    else area?.focus();
  }
</script>

<section class="peek-sec" aria-label="説明">
  <h3>説明</h3>
  {#if $view.loaded}
    {#if $view.editing}
      <Textarea
        bind:this={area}
        label="説明"
        rows={8}
        focusOnMount
        {onkeydown}
        bind:value={() => $view.input, editor.setInput}
      />
      <p class="hint">Markdown を使用できます。@ログイン名 でメンション</p>
      <div class="actions">
        <Button variant="primary" disabled={$view.saving} onclick={() => void editor.save()}>
          保存
        </Button>
        <Button onclick={editor.cancel}>キャンセル</Button>
      </div>
    {:else if $view.text === ""}
      <button type="button" class="add" disabled={readonly} title={note} onclick={editor.start}>
        説明を追加
      </button>
    {:else}
      <!-- 説明の部分を押すと入力欄になる。押す場所は、内容の下に敷いた本物のボタン(中のリンクは押せる)。 -->
      <div class="view">
        <button
          type="button"
          class="hit"
          aria-label="説明を編集"
          disabled={readonly}
          title={note}
          onclick={editor.start}
        ></button>
        <div class="content"><Markdown blocks={$view.blocks} /></div>
      </div>
    {/if}
  {/if}
</section>

<style>
  .peek-sec {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  h3 {
    font-size: 13px;
    margin: 0;
  }
  .hint {
    margin: 0;
    color: var(--muted);
    font-size: 12.5px;
  }
  .actions {
    display: flex;
    gap: 6px;
  }
  .add {
    align-self: flex-start;
    border: 0;
    background: none;
    padding: 2px 0;
    color: var(--muted);
  }
  .add:hover:not(:disabled) {
    color: var(--ink);
    text-decoration: underline;
  }
  .add:disabled {
    cursor: not-allowed;
    opacity: 0.6;
  }
  .view {
    position: relative;
    border-radius: 6px;
  }
  .hit {
    position: absolute;
    inset: -4px -6px;
    border: 0;
    border-radius: 6px;
    background: transparent;
    cursor: text;
  }
  .hit:hover:not(:disabled) {
    background: var(--hover);
  }
  .hit:disabled {
    cursor: default;
  }
  .content {
    position: relative;
    pointer-events: none;
  }
  .content :global(a) {
    pointer-events: auto;
  }
</style>
```

`web/app/src/screens/item/DetailPanel.svelte`: import を足し、`<DetailProperties {v} />` の次に区画を足す(説明は、プロパティと子のあいだ。02「上から順の構成」)。

```svelte
  import DescriptionSection from "../notes/DescriptionSection.svelte";
```

```svelte
    <DetailProperties {v} />
    <!-- 開いたアイテムが変わったら、入力欄の状態ごと作り直す。反映(v の読み直し)では作り直さず、target だけ新しくなる。 -->
    {#key v.item.id}
      <DescriptionSection
        target={{ kind: "item", storeId: v.store.id, id: v.item.id }}
        readonly={v.readOnly}
      />
    {/key}
    {#if v.item.kind !== "Step"}
```

(元の `{#if v.item.kind !== "Step"}` の行はそのまま残す。説明は Step にもあるので、`{#if}` の外に置く。)

- [ ] **Step 12: 検査する**

Run: `pnpm -C web fix && pnpm -C web lint && pnpm -C web check && pnpm -C web test && pnpm -C web size && pnpm -C web deps`
Expected: すべて成功。`svelte-check` が `bind:this` の型や `bind:value` の関数の形で落ちたら、`ReturnType<typeof Textarea>` と `bind:value={() => …, …}` の形(`CreateItemFields.svelte` の `bind:value` と同じ)に合わせる。

- [ ] **Step 13: e2e のテストを書いて通す**

`web/e2e/description.spec.ts`(新規):

```ts
import { expect, test } from "@playwright/test";
import { newTask } from "./helpers";

// 詳細パネルの説明(A4a)。サーバーは 3 つの幅で共有なので、Task の名前は毎回変える(helpers の newTask)。
const body = "## 手順\n- 一つ目\n**太字** と https://example.com/guide を見る";

test("writes a description, shows it as Markdown and asks before discarding a change", async ({
  page,
}, info) => {
  const { panel } = await newTask(page, info, "説明の確認");
  const section = panel.getByRole("region", { name: "説明" });
  const edit = section.getByRole("textbox", { name: "説明" });

  // 空のときは「説明を追加」。押すと入力欄になり、下に案内と 2 つのボタンが出る。
  await section.getByRole("button", { name: "説明を追加" }).click();
  await expect(section.getByText("Markdown を使用できます。@ログイン名 でメンション")).toBeVisible();
  await edit.fill(body);
  await edit.press("Control+Enter");
  await expect(edit).toBeHidden();
  await expect(section.getByRole("heading", { name: "手順" })).toBeVisible();
  await expect(section.getByRole("listitem")).toHaveText(["一つ目"]);
  const link = section.getByRole("link", { name: "https://example.com/guide" });
  await expect(link).toHaveAttribute("target", "_blank");
  await expect(link).toHaveAttribute("rel", "noopener noreferrer");

  // 変えずに保存したときは、何も保存せずに閉じる。
  await section.getByRole("button", { name: "説明を編集" }).click();
  await expect(edit).toHaveValue(body);
  await edit.press("Control+Enter");
  await expect(edit).toBeHidden();

  // 変えて Esc: 確認が出る。「キャンセル」は入力欄に戻る。
  await section.getByRole("button", { name: "説明を編集" }).click();
  await edit.fill(`${body}\n追記`);
  await edit.press("Escape");
  const confirm = page.getByRole("dialog", { name: "入力を破棄" });
  await expect(confirm).toContainText("保存していない説明の変更を破棄します。");
  await confirm.getByRole("button", { name: "キャンセル" }).click();
  await expect(confirm).toBeHidden();
  await expect(edit).toBeFocused();
  await expect(edit).toHaveValue(`${body}\n追記`);

  // パネルを閉じる操作(「×」)でも確認が出る。「キャンセル」ならパネルは開いたまま。
  await panel.getByRole("button", { name: "閉じる" }).click();
  await expect(confirm).toBeVisible();
  await confirm.getByRole("button", { name: "キャンセル" }).click();
  await expect(panel).toBeVisible();

  // 「破棄」は入力欄を閉じ、保存してある説明に戻る(パネルは閉じない)。
  await edit.press("Escape");
  await confirm.getByRole("button", { name: "破棄" }).click();
  await expect(edit).toBeHidden();
  await expect(panel).toBeVisible();
  await expect(section.getByRole("heading", { name: "手順" })).toBeVisible();

  // 「キャンセル」ボタンは確認なしで入力を捨てる。
  await section.getByRole("button", { name: "説明を編集" }).click();
  await edit.fill("捨てる");
  await section.getByRole("button", { name: "キャンセル" }).click();
  await expect(confirm).toBeHidden();
  await expect(edit).toBeHidden();
});
```

Run: `make e2e-bin && pnpm -C web exec playwright test e2e/description.spec.ts`
Expected: PASS(3 つの幅で各 1 件)。`e2e-bin` は Go のサーバー(Task 3 の説明の API)と画面をまとめて作る。

- [ ] **Step 14: コミット**

```bash
git add web/packages/ui/src/primitives/Textarea.svelte web/packages/ui/src/primitives/textarea.test.ts web/packages/ui/src/index.ts web/packages/core/src/port/notes.ts web/packages/core/src/features/notes web/packages/core/src/notesIndex.ts web/packages/api/src/notes.ts web/packages/api/src/notes.test.ts web/packages/api/src/index.ts web/app/src/wire.ts web/app/src/screens/notes/DescriptionSection.svelte web/app/src/screens/item/DetailPanel.svelte web/e2e/description.spec.ts
git commit -m "feat(web): 詳細パネルに説明の区画を足す(表示・編集・保存・同じ行のトースト。A4a-3・A4a-20〜22)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 8: 作成ダイアログの説明の欄とテンプレートの説明の当てはめ

作成ダイアログに説明の欄(4 行)を足し、テンプレートの説明を当てはめる(A4a-6)。自動(開く・種類を切り替える・ワークスペースを変える)では、説明が空か、いま当てはめているテンプレートの説明と一字一句同じときだけ入れる。セレクトで選ぶと置き換える。「なし」に戻すとテンプレート由来なら空に戻す。「続けて作成」はタイトルと説明とエラーを空にする。作成の入力に `description` を送る(Task 4)。

**Files:**
- Modify: `web/packages/core/src/features/workspaces/templateApply.ts`(`nextDescription` を取り出す)
- Test: `web/packages/core/src/features/workspaces/templateApply.test.ts`
- Modify: `web/packages/core/src/features/items/createTemplate.ts`
- Test: `web/packages/core/src/features/items/createTemplate.test.ts`
- Modify: `web/packages/core/src/features/items/createDialogOptions.ts`(`carried`)
- Modify: `web/packages/core/src/features/items/createDialog.ts`(288 行 → 293 行。300 行の上限に近いので、これ以上は足さない)
- Modify: `web/packages/core/src/features/items/fixtures.test-util.ts`(`createdDescriptions`)
- Test: `web/packages/core/src/features/items/createDialogTemplate.test.ts`
- Modify: `web/packages/core/src/model/item.ts`(`NewItem.description`)
- Modify: `web/packages/api/src/items.ts`
- Test: `web/packages/api/src/itemDescription.test.ts`
- Modify: `web/app/src/screens/item/CreateItemFields.svelte`
- Modify: `web/e2e/create-template.spec.ts`

**Interfaces:**
- Consumes: Task 4 の `create-item` の本文の `description`(orval の `createItem(store, { …, description })`)、Task 7 の `Textarea.svelte`
- Produces(core): `nextDescription(current: string, applied: string, next: WorkspaceTemplate | null, mode: ApplyMode): string`(`templateApply.ts`)、`TemplateForm` に `description: string`・`templateDescription: string`、`ItemForm` にも同じ 2 つ、`NewItem.description?: string`、`applyToForm(form, settings, templateId, mode = "select")`、`carried(loaded, now, values)`(`createDialogOptions.ts`)、`FakeItems.createdDescriptions: string[]`
- 変わる動き: `applyDefault` は `"auto"` で当てはめる(いまは `"select"` 相当)。`applyToForm` は、いま当てはめているテンプレートを設定から引かず、フォームが持つ `templateDescription` と比べる

**決めたこと:**
- `templateDescription` は、いま当てはめているテンプレートの説明(当てはめていなければ ""。説明のないテンプレートも "")。説明が `""` か `templateDescription` と一字一句同じなら、テンプレート由来(02「開き方と初期値」)。ワークスペースを変えると、前のテンプレートは新しい設定にないので、設定から引けない。だからフォームが持つ
- `applyDefault` の「いまのテンプレートを先に「なし」にする」(`templateId: ""`)は、`applyToForm` が設定からいまのテンプレートを引かなくなったので意味がなくなる。そのまま `applyToForm(form, …, "auto")` を呼ぶ
- 作成に送る `description` は、空なら送らない(api の `create` で `templateId` と同じ扱い)
- 説明の欄の位置は 02「ダイアログの項目」の表の順(プロパティの次、一緒に作るものの前)。見出し(ラベル)は「説明」、案内文は出さない(仕様にない)
- プロジェクトを変えたときの「選んでいるテンプレートが新しいプロジェクトで使えないなら…」は、使えるテンプレートがワークスペースの設定だけで決まる間(B5 まで)は起きないので、手を入れない

- [ ] **Step 1: `nextDescription` の失敗するテストを書く**

`web/packages/core/src/features/workspaces/templateApply.test.ts` の import を替え、`describe("apply template", …)` の前に足す(`tpl`・`taskT` はこのファイルにある)。

```ts
import { applyTemplate, nextDescription, templateStructure } from "./templateApply";
```

```ts
describe("nextDescription", () => {
  const next = taskT; // 説明は "## 手順"
  it("auto: fills an empty description, keeps a typed one, replaces the applied template's own", () => {
    expect(nextDescription("", "", next, "auto")).toBe("## 手順");
    expect(nextDescription("メモ", "", next, "auto")).toBe("メモ");
    expect(nextDescription("## 概要", "## 概要", next, "auto")).toBe("## 手順");
    expect(nextDescription("## 概要!", "## 概要", next, "auto")).toBe("## 概要!");
  });

  it("select: always the template's description", () => {
    expect(nextDescription("メモ", "", next, "select")).toBe("## 手順");
  });

  it("なし: clears a template's description and keeps a typed one", () => {
    expect(nextDescription("## 手順", "## 手順", null, "select")).toBe("");
    expect(nextDescription("メモ", "## 手順", null, "select")).toBe("メモ");
    expect(nextDescription("", "", null, "auto")).toBe("");
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web/packages/core exec vitest run src/features/workspaces/templateApply.test.ts`
Expected: FAIL(`nextDescription is not a function`)

- [ ] **Step 3: `nextDescription` を取り出す**

`web/packages/core/src/features/workspaces/templateApply.ts` の `applyTemplate` の説明の部分を、関数に取り出す。`export type ApplyMode` の次に足す。

```ts
// nextDescription は、テンプレートを当てはめたあとの説明。applied は、いま当てはめているテンプレートの説明
// (当てはめていなければ "")。説明が空か applied と一字一句同じなら、テンプレート由来とみなす。
export function nextDescription(
  current: string,
  applied: string,
  next: WorkspaceTemplate | null,
  mode: ApplyMode,
): string {
  const derived = current === "" || current === applied;
  if (next === null) return derived ? "" : current;
  return mode === "select" || derived ? next.description : current;
}
```

`applyTemplate` の本体のうち、次の部分

```ts
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
```

を消し、返す値の `description,` を次に替える。

```ts
    description: nextDescription(
      state.description,
      state.template?.description ?? "",
      next,
      mode,
    ),
```

Run: `pnpm -C web/packages/core exec vitest run src/features/workspaces/`
Expected: PASS(既存の `applyTemplate` のテストも変わらず通る)

- [ ] **Step 4: `createTemplate` の失敗するテストを書く**

`web/packages/core/src/features/items/createTemplate.test.ts` を変える。

`form` の既定に 2 つ足す。

```ts
const form = (f: Partial<TemplateForm> = {}): TemplateForm => ({
  kind: "Task",
  title: "",
  values: { status: "st1", priority: "pr2" },
  templateId: "",
  description: "",
  templateDescription: "",
  ...f,
});
```

ファイルの末尾に足す。

```ts
// 説明のあるテンプレート: D1・D2 は Task、DB は Bug。
const d1 = tpl({ id: "D1", kind: "Task", description: "## 手順" });
const d2 = tpl({ id: "D2", kind: "Task", description: "## 別の手順" });
const dBug = tpl({ id: "DB", kind: "Bug", description: "## 概要" });
const withDescriptions: WorkspaceSettings = {
  ...settings,
  templates: [d1, d2, dBug],
  templateDefaults: { Task: "D1", Bug: "DB" },
};

describe("the template's description", () => {
  it("select replaces whatever is typed, and remembers the template's description", () => {
    const out = applyToForm(form({ description: "メモ" }), withDescriptions, "D1");
    expect(out).toMatchObject({
      description: "## 手順",
      templateDescription: "## 手順",
      templateId: "D1",
    });
    expect(applyToForm(out, withDescriptions, "D2").description).toBe("## 別の手順");
  });

  it("the default fills an empty description, keeps a typed one, replaces the template's own", () => {
    const filled = applyDefault(form(), withDescriptions);
    expect(filled.description).toBe("## 手順");
    // 種類を Bug に切り替えた: テンプレート由来なので、新しい既定の説明に置き換わる。
    expect(applyDefault({ ...filled, kind: "Bug" }, withDescriptions).description).toBe("## 概要");
    // 1 文字足していれば、利用者の入力として残る。
    const typed = { ...filled, kind: "Bug" as const, description: "## 手順!" };
    expect(applyDefault(typed, withDescriptions)).toMatchObject({
      description: "## 手順!",
      templateDescription: "## 概要",
      templateId: "DB",
    });
  });

  it("compares with the template that was applied, even when the new settings do not have it", () => {
    // ワークスペースを変えた: 前のワークスペースのテンプレート D1 は、新しい設定にない。
    const applied = applyDefault(form(), withDescriptions);
    const other: WorkspaceSettings = {
      ...settings,
      templates: [tpl({ id: "X1", kind: "Task", description: "## 別" })],
      templateDefaults: { Task: "X1" },
    };
    expect(applyDefault(applied, other).description).toBe("## 別");
  });

  it("なし clears a template's description and keeps a typed one", () => {
    const applied = applyToForm(form(), withDescriptions, "D1");
    expect(applyToForm(applied, withDescriptions, "")).toMatchObject({
      description: "",
      templateDescription: "",
      templateId: "",
    });
    expect(applyToForm({ ...applied, description: "メモ" }, withDescriptions, "")).toMatchObject({
      description: "メモ",
      templateId: "",
    });
  });
});
```

- [ ] **Step 5: 失敗を確かめる**

Run: `pnpm -C web/packages/core exec vitest run src/features/items/createTemplate.test.ts`
Expected: FAIL(型の誤りと、`description`・`templateDescription` が `undefined`)

- [ ] **Step 6: `createTemplate` を実装する**

`web/packages/core/src/features/items/createTemplate.ts`:

import を替える。

```ts
import {
  type ApplyMode,
  applyTemplate,
  defaultTemplate,
  nextDescription,
  type StructureRow,
  templateStructure,
  usableTemplates,
} from "../workspaces/templateApply";
```

ファイル先頭のコメントの「説明の当てはめは A4(A3a の決定 6)」を「説明の当てはめは A4a-6」に替え、`TemplateForm` に 2 つ足す。

```ts
export interface TemplateForm {
  kind: Kind;
  title: string;
  values: Values;
  // templateId は、当てはめているテンプレート。空は「なし」。
  templateId: string;
  // description は説明の欄。templateDescription は、いま当てはめているテンプレートの説明(当てはめていなければ "")。
  // 説明が "" かこれと一字一句同じなら、テンプレート由来とみなす(02「開き方と初期値」)。ワークスペースを変えると
  // 前のテンプレートは新しい設定にないので、設定から引かず、フォームが持つ。
  description: string;
  templateDescription: string;
}
```

`applyToForm` と `applyDefault` を次にする。

```ts
// applyToForm は、templateId のテンプレートを当てはめた入力。空なら「なし」にする(入力は残す)。
// mode は、説明の扱い: "select" はセレクトで選んだとき(置き換える)、"auto" は開いたとき・種類やワークスペースを
// 変えたとき(空かテンプレート由来のときだけ入れる)。
export function applyToForm<F extends TemplateForm>(
  form: F,
  settings: WorkspaceSettings,
  templateId: string,
  mode: ApplyMode = "select",
): F {
  const shown = new Set(visibleProperties(settings, form.kind).map((p) => p.id));
  const next = find(settings, templateId);
  const out = applyTemplate(
    { title: form.title, description: "", values: {}, template: null },
    next,
    mode,
    (id) => shown.has(id),
  );
  const values = { ...form.values };
  for (const [id, v] of Object.entries(out.values)) values[id] = toInput(v);
  return {
    ...form,
    title: out.title,
    values,
    templateId: next?.id ?? "",
    description: nextDescription(form.description, form.templateDescription, next, mode),
    templateDescription: next?.description ?? "",
  };
}

// applyDefault は、その種類の既定のテンプレートを当てはめる(A3b-14。説明は自動の扱い。A4a-6)。
export function applyDefault<F extends TemplateForm>(form: F, settings: WorkspaceSettings): F {
  return applyToForm(form, settings, defaultTemplate(settings, form.kind)?.id ?? "", "auto");
}
```

Run: `pnpm -C web/packages/core exec vitest run src/features/items/createTemplate.test.ts`
Expected: PASS(既存 7 件と新しい 4 件)

- [ ] **Step 7: ダイアログの失敗するテストを書く**

`web/packages/core/src/features/items/fixtures.test-util.ts` に `createdDescriptions` を足す。`FakeItems` の `createdTemplates: (string | undefined)[];` の次に:

```ts
  // createdDescriptions は、create に渡された description の記録(なければ "")。
  createdDescriptions: string[];
```

`fakeItemsPort` の `const createdTemplates: (string | undefined)[] = [];` の次に `const createdDescriptions: string[] = [];`、返すオブジェクトの `createdTemplates,` の次に `createdDescriptions,`、`create` の `createdTemplates.push(input.templateId);` の次に `createdDescriptions.push(input.description ?? "");` を足す。

`web/packages/core/src/model/item.ts` の `NewItem` の末尾に足す。

```ts
  // description は、作るアイテムの説明(空・なしは説明なし)。
  description?: string;
```

`web/packages/core/src/features/items/createDialogTemplate.test.ts`:

テンプレートの定義に説明を足す(4 か所。`base` の `description: ""` を上書きする)。

```ts
    {
      ...base,
      id: "T1",
      kind: "Task",
      name: "調査",
      title: "調査: ",
      description: "## 調査の手順",
      values: { priority: "pr1" },
    },
    {
      ...base,
      id: "T2",
      kind: "Task",
      name: "作業",
      title: "",
      description: "## 作業の手順",
      values: {},
      steps,
    },
    {
      ...base,
      id: "B1",
      kind: "Bug",
      name: "不具合",
      title: "不具合: ",
      description: "## 不具合の概要",
      values: {},
    },
```

`ws2` の `templates` を次にする。

```ts
  templates: [
    {
      ...base,
      id: "X1",
      kind: "Task",
      name: "別",
      title: "別: ",
      description: "## 別の手順",
      values: { assignee: "jiro" },
    },
  ],
```

ファイルの末尾に足す。

```ts
describe("create dialog description", () => {
  const description = (dialog: Awaited<ReturnType<typeof setup>>["dialog"]) =>
    get(dialog.state)?.form.description;

  it("fills the default template's description when it opens, and sends it", async () => {
    const { dialog, items } = await setup();
    expect(description(dialog)).toBe("## 調査の手順");
    await dialog.update({ title: "調べる" });
    await dialog.submit();
    expect(items.createdDescriptions.at(-1)).toBe("## 調査の手順");
  });

  it("replaces the template's own description when the kind changes, and keeps a typed one", async () => {
    const { dialog } = await setup();
    await dialog.update({ kind: "Bug" });
    expect(description(dialog)).toBe("## 不具合の概要");
    await dialog.update({ description: "## 不具合の概要!" });
    await dialog.update({ kind: "Task" });
    expect(description(dialog)).toBe("## 不具合の概要!");
  });

  it("replaces the template's own description when the workspace changes, and keeps a typed one", async () => {
    const { dialog } = await setup();
    await dialog.update({ workspaceId: "S2-W1" });
    expect(description(dialog)).toBe("## 別の手順");
    await dialog.update({ workspaceId: "S1-W1" });
    await dialog.update({ description: "メモ" });
    await dialog.update({ workspaceId: "S2-W1" });
    expect(description(dialog)).toBe("メモ");
  });

  it("select replaces a typed description; なし clears only a template's own", async () => {
    const { dialog } = await setup();
    await dialog.update({ description: "メモ" });
    await dialog.update({ templateId: "T2" });
    expect(description(dialog)).toBe("## 作業の手順");
    await dialog.update({ templateId: "" });
    expect(description(dialog)).toBe("");
    await dialog.update({ templateId: "T1" });
    await dialog.update({ description: "メモ" });
    await dialog.update({ templateId: "" });
    expect(description(dialog)).toBe("メモ");
  });

  it("continuing clears the title and the description, and keeps the template", async () => {
    const { dialog } = await setup();
    await dialog.update({ title: "続けて", keepOpen: true });
    await dialog.submit();
    expect(get(dialog.state)?.form).toMatchObject({ title: "", description: "", templateId: "T1" });
  });

  it("keeps a description typed while the dialog reloads", async () => {
    const { dialog } = await setup();
    const reloading = dialog.update({ projectId: "S1-P2" });
    await dialog.update({ description: "読み直し中に打った" });
    await reloading;
    expect(description(dialog)).toBe("読み直し中に打った");
  });
});
```

- [ ] **Step 8: 失敗を確かめる**

Run: `pnpm -C web/packages/core exec vitest run src/features/items/createDialogTemplate.test.ts`
Expected: FAIL(`form.description` が `undefined`、`createdDescriptions` が空)

- [ ] **Step 9: ダイアログを実装する**

`web/packages/core/src/features/items/createDialogOptions.ts`: import に `Values` を足し、末尾に足す。

```ts
import type { Values } from "./createValues";
```

```ts
// carried は、読み直しを待つあいだに入力された分(タイトル・説明・テンプレート・「続けて作成」)を、読み直した
// 入力へ引き継ぐ(決定 27)。値は、呼び出し側が決めたもの。
export const carried = <F extends TemplateForm & { keepOpen: boolean }>(
  loaded: F,
  now: F,
  values: Values,
): F => ({
  ...loaded,
  title: now.title,
  description: now.description,
  templateId: now.templateId,
  templateDescription: now.templateDescription,
  keepOpen: now.keepOpen,
  values,
});
```

`web/packages/core/src/features/items/createDialog.ts` を 5 か所変える(288 行 → 293 行)。

1. ファイル先頭のコメントの「スプリントは F、説明は A4。」を「スプリントは F。」に替える。
2. import: `import { missing, parentOptions, viewOf, wsOptions } from "./createDialogOptions";` を `import { carried, missing, parentOptions, viewOf, wsOptions } from "./createDialogOptions";` に替える。
3. `ItemForm` の `templateId: string;` の次に足す。

```ts
  // description は説明の欄。templateDescription は、いま当てはめているテンプレートの説明(なければ空)。
  description: string;
  templateDescription: string;
```

4. `reloaded` の `const { title, keepOpen, templateId } = now.form;` の行を消し、`let kept: ItemForm = { …, templateId };` を次にする(`values` はその上で決めている)。

```ts
    let kept = carried(loaded.next.form, now.form, values);
```

5. `send` の `create` に `description` を足し(`title,` の次)、「続けて作成」で説明も空にする。

```ts
        title,
        description: s.form.description,
        values: shownValues(s.form.values, s.settings, kind),
```

```ts
      // 続けて作成: タイトル・説明・エラーだけを空にし、ほかは残す(02「作成したあと」)。
      const now = get(state);
      if (now) {
        const form = { ...now.form, title: "", description: "" };
```

`open` の `let form: ItemForm = { … templateId: "", };` に 2 つ足す。

```ts
        templateId: "",
        description: "",
        templateDescription: "",
      };
```

Run: `pnpm -C web/packages/core exec vitest run src/features/items/`
Expected: PASS(新しい 6 件と既存のすべて)。`createDialog.ts` が 300 行以内であることは、Step 12 の `pnpm -C web size` で確かめる。

- [ ] **Step 10: api の失敗するテストを書く**

`web/packages/api/src/itemDescription.test.ts`:

```ts
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createItemsPort } from "./items";

afterEach(() => {
  vi.unstubAllGlobals();
});

const apiItem = {
  id: "I1",
  workspace: "W1",
  project: "P1",
  parent: null,
  number: 13,
  kind: "Epic",
  rank: "V",
  title: "認証",
  watchers: null,
  createdAt: "2026-11-13T09:00:00.000Z",
  createdBy: "taro",
};

describe("items port: description", () => {
  it("sends the description of a new item only when there is one", async () => {
    const fetch = vi.fn(
      async () =>
        new Response(JSON.stringify(apiItem), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    );
    vi.stubGlobal("fetch", fetch);
    setToken("tok");
    const input = { projectId: "P1", parentId: null, kind: "Epic" as const, title: "認証", values: {} };
    await createItemsPort().create("S1", { ...input, description: "## 概要" });
    await createItemsPort().create("S1", input);
    const bodies = fetch.mock.calls.map((c) =>
      JSON.parse(String((c as unknown as [string, RequestInit])[1].body)),
    );
    expect(bodies[0]).toMatchObject({ description: "## 概要" });
    expect(bodies[1]).not.toHaveProperty("description");
  });
});
```

- [ ] **Step 11: api を実装する**

Run(失敗の確認): `pnpm -C web/packages/api exec vitest run src/itemDescription.test.ts`
Expected: FAIL(1 件目の本文に `description` がない)

`web/packages/api/src/items.ts` の `create` の本文に足す(`values: input.values,` の次)。

```ts
            ...(input.description ? { description: input.description } : {}),
```

Run: `pnpm -C web/packages/api exec vitest run`
Expected: PASS(`itemDescription.test.ts` と既存のすべて)。`description` が `createItem` の本文の型にないと型で落ちる(Task 4 で `make generate` と `pnpm -C web gen` を済ませていること。本文の `description` は省略できる形であること)。

- [ ] **Step 12: 画面に説明の欄を足す**

`web/app/src/screens/item/CreateItemFields.svelte`: import を替える。

```ts
  import { Input, Select, Textarea } from "@hedgehug/ui";
```

`<CreatePropertyFields {s} />` と `{#if s.structure}` のあいだに足す(02「ダイアログの項目」の表の順)。

```svelte
<CreatePropertyFields {s} />
<div class="field">
  <span>説明</span>
  <Textarea
    label="説明"
    rows={4}
    bind:value={() => s.form.description, (v) => void itemDialog.update({ description: v })}
  />
</div>
{#if s.structure}
```

先頭のコメント(`// 作成ダイアログの項目(02「ダイアログの項目」のうち、…)`)の「タイトル・一緒に作るもの」を「タイトル・説明・一緒に作るもの」に替える。

Run: `pnpm -C web fix && pnpm -C web lint && pnpm -C web check && pnpm -C web test && pnpm -C web size && pnpm -C web deps`
Expected: すべて成功。

- [ ] **Step 13: e2e を足して通す**

`web/e2e/create-template.spec.ts` の「creates a task with its template's steps in one go」を 3 か所変える。

テンプレートの作成で、「タイトルの書き出し」の入力の次の行に足す。

```ts
  await template.getByRole("textbox", { name: "タイトルの書き出し" }).fill("手順: ");
  await template.getByRole("textbox", { name: "説明" }).fill("## 準備\n手順書を読む");
```

作成ダイアログを開いた直後の、書き出しの確認の次の行に足す。

```ts
  await expect(dialog.getByRole("textbox", { name: "タイトル" })).toHaveValue("手順: ");
  await expect(dialog.getByRole("textbox", { name: "説明" })).toHaveValue("## 準備\n手順書を読む");
```

最後の Step の番号の確認の次に足す(作った Task の詳細パネルに、説明が Markdown で出る)。

```ts
  await expect(steps.getByText(`${prefix}-4`, { exact: true })).toBeVisible();
  await expect(panel.getByRole("heading", { name: "準備" })).toBeVisible();
```

Run: `make e2e-bin && pnpm -C web exec playwright test e2e/create-template.spec.ts`
Expected: PASS(3 つの幅)。`e2e-bin` は Go のサーバー(Task 4 の作成の説明)と画面をまとめて作る。

- [ ] **Step 14: コミット**

```bash
git add web/packages/core/src/features/workspaces/templateApply.ts web/packages/core/src/features/workspaces/templateApply.test.ts web/packages/core/src/features/items web/packages/core/src/model/item.ts web/packages/api/src/items.ts web/packages/api/src/itemDescription.test.ts web/app/src/screens/item/CreateItemFields.svelte web/e2e/create-template.spec.ts
git commit -m "feat(web): 作成ダイアログに説明の欄を足し、テンプレートの説明を当てはめる(A4a-6)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 9: コメントの行の決め方と操作(アイテム・プロジェクト)・API

**Files:**
- Modify: `internal/domain/change/record.go`(`Comment` の種類)
- Modify: `internal/domain/change/draft.go`(`NewComment`)
- Create: `internal/domain/change/current.go`(`Absent`・`Current`・`First`)
- Test: `internal/domain/change/current_test.go`
- Create: `internal/domain/comment/comment.go`
- Test: `internal/domain/comment/comment_test.go`
- Create: `internal/app/comments/service.go`・`list.go`・`post.go`・`edit.go`・`delete.go`
- Test: `internal/app/comments/comments_test.go`
- Create: `internal/adapter/httpapi/notes/comments.go`
- Modify: `internal/adapter/httpapi/notes/notes.go`(Task 3 が作る。`Services` に `Comments appcomments.Service` を足し、`Register` に `registerComments(api, s.Comments)` を 1 行足す)
- Test: `internal/adapter/httpapi/notes/comments_test.go`
- Modify: `internal/adapter/httpapi/routes.go`(`notes.Register` の `Services` に `Comments` を足す)
- 生成物: `api/openapi.yaml`・`web/packages/api/src/generated.ts`(`make generate`・`pnpm -C web gen`)

**Interfaces:**
- Consumes: Task 3 の `notes.Services`・`notes.Register`、`change.Stamp`(行の `prev` は書く時点の先端)、`port.Reader.Changes().Of`、`port.Tx.WriteItems()`・`WriteProjects()`、`port.Identity`、`port.IDs`
- Produces:
  - `change.Comment Type = "comment"`、`func NewComment(id ID, from, to any) Draft`(`from` が nil なら追加、`to` が nil なら削除。どちらも JSON の `null` で書く)
  - `func Absent(v json.RawMessage) bool`(`to` が欠けているか `null`)
  - `func Current(log []Record, t Type, f string, alive func(Record) bool) (Record, bool)`(`t` と `f` が同じ行の先端から、いまの値の行を選ぶ。`alive` が真の先端があればその中で最もあとの行、なければ先端で最もあとの行。06「一方が削除し、他方が変更した」)
  - `func First(log []Record, t Type, f string) (Record, bool)`(最初に書いた行。`prev` が同じ項目の行を指さない行のうち、最も古いもの)
  - `comment.Comment{ID change.ID; By string; At time.Time; Body string; Edited bool}`、`comment.FromLog(log []change.Record) []comment.Comment`、`comment.Find(log []change.Record, id change.ID) (comment.Comment, bool)`、`comment.Value(body string) any`
  - `comments.Service{Stores port.Stores; IDs port.IDs; Identity port.Identity}`、`comments.Target{Kind change.Kind; ID change.ID}`(`Kind` は `change.KindItem` か `change.KindProject`)、`comments.View{comment.Comment; ByName string}`
  - `func (s Service) List(ctx context.Context, store port.StoreID, t Target) ([]View, error)`、`Post(ctx, store, t, body string) (View, error)`、`Edit(ctx, store, t, id change.ID, body string) (View, error)`、`Delete(ctx, store, t, id change.ID) error`。`Edit`・`Delete` は、投稿者が「あなた」でないコメントを `port.ErrNotFound` で断る(ログイン名は大文字と小文字を区別せずに比べる。画面からは起きない操作で、B2-12 と同じ扱い)
  - `notes.Services.Comments`(`appcomments.Service`)と、`Register` が呼ぶ `registerComments(api huma.API, svc appcomments.Service)`、API 本文の型 `notes.Comment`・`notes.CommentText`(operationId は骨組みの表のとおり)

**行の形(A4a-7):** `t: "comment"`、`f` はコメントの内部 ID、`to` は `{"body": 本文}`。投稿は `from: null`、編集は `from` に前の本文、削除は `to: null`。投稿者と投稿の時刻は、そのコメントの最初の行の `by`・`at`。

- [ ] **Step 1: `change` の失敗するテストを書く**

`internal/domain/change/current_test.go`(同じパッケージの `resolve_test.go` の補助 `rec`・`at`・`create` を使う):

```go
package change_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
)

func note(id string, minute int, by string, from, to any, prev ...change.ID) change.Record {
	return rec(id, at(minute), by, change.NewComment("K1", from, to), prev...)
}

func text(s string) map[string]string { return map[string]string{"body": s} }

func alive(r change.Record) bool { return !change.Absent(r.To) }

func current(log []change.Record) (change.Record, bool) {
	return change.Current(log, change.Comment, "K1", alive)
}

func TestNewCommentWritesNullForAddAndDelete(t *testing.T) {
	post := change.NewComment("K1", nil, text("a"))
	del := change.NewComment("K1", text("a"), nil)
	if post.T != change.Comment || post.F != "K1" || string(post.From) != "null" || string(post.To) != `{"body":"a"}` {
		t.Errorf("post = %+v", post)
	}
	if string(del.To) != "null" || !change.Absent(del.To) || change.Absent(post.To) || !change.Absent(nil) {
		t.Errorf("delete = %+v", del)
	}
}

func TestCurrentFollowsTheEditChain(t *testing.T) {
	log := []change.Record{
		create(map[string]any{"title": "a"}),
		note("p", 1, "hanako", nil, text("a"), "c"),
		note("e", 2, "hanako", text("a"), text("b"), "p"),
	}
	if got, ok := current(log); !ok || got.ID != "e" {
		t.Errorf("Current = %v, %v; want e", got.ID, ok)
	}
	if _, ok := change.Current(log, change.Comment, "K2", alive); ok {
		t.Error("a comment with no rows must have no value")
	}
}

func TestCurrentDeleteThatSawTheEditWins(t *testing.T) {
	log := []change.Record{
		note("p", 1, "hanako", nil, text("a"), "c"),
		note("e", 2, "hanako", text("a"), text("b"), "p"),
		note("d", 3, "taro", text("b"), nil, "e"),
	}
	got, ok := current(log)
	if !ok || got.ID != "d" || !change.Absent(got.To) {
		t.Errorf("Current = %v, %v; want the delete d", got.ID, ok)
	}
}

func TestCurrentKeepsAnEditAgainstAConcurrentDelete(t *testing.T) {
	// 同じ行 p を見て、一方が編集し(a・b)、他方が削除した(d。時計は最もあと)。編集を残し、同時の編集は時計が遅いほう。
	log := []change.Record{
		note("p", 1, "hanako", nil, text("a"), "c"),
		note("a", 2, "hanako", text("a"), text("A"), "p"),
		note("b", 3, "taro", text("a"), text("B"), "p"),
		note("d", 9, "taro", text("a"), nil, "p"),
	}
	want, ok := current(log)
	if !ok || want.ID != "b" {
		t.Fatalf("Current = %v, %v; want b", want.ID, ok)
	}
	rev := slices.Clone(log)
	slices.Reverse(rev)
	if got, _ := current(rev); got.ID != want.ID {
		t.Errorf("Current of the reversed log = %v, want %v", got.ID, want.ID)
	}
}

func TestFirstIsTheRowThatDoesNotFollowAnother(t *testing.T) {
	// 編集の行の時計が投稿より前にずれていても、最初の行は prev で決める。
	log := []change.Record{
		note("e", 1, "taro", text("a"), text("b"), "p"),
		note("p", 5, "hanako", nil, text("a"), "c"),
	}
	got, ok := change.First(log, change.Comment, "K1")
	if !ok || got.ID != "p" || got.By != "hanako" {
		t.Errorf("First = %v, %v; want p by hanako", got.ID, ok)
	}
	if _, ok := change.First(log, change.Comment, "K2"); ok {
		t.Error("First of a comment with no rows")
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/change/ -run 'NewComment|Current|First'`
Expected: FAIL(ビルドエラー `undefined: change.Comment`・`change.NewComment`・`change.Absent`・`change.Current`・`change.First`)

- [ ] **Step 3: 実装する**

`record.go` の `Type` の定数に足す(`Delete` の下)。`Desc` と `Worklog` はそれぞれ Task 2 と Task 11 が足す。同じ `const` ブロックに並べ、すでにあれば重ねない。

```go
	// Comment はコメントの追加・編集・削除。f はコメントの内部 ID、to は {"body": 本文}、削除は to が null(A4a-7)。
	Comment Type = "comment"
```

`draft.go` の末尾に足す。

```go
// NewComment は、コメントの追加・編集・削除の変更。f はコメントの内部 ID。
// from が nil なら追加、to が nil なら削除(どちらも null で書く)。
func NewComment(id ID, from, to any) Draft {
	return Draft{T: Comment, F: string(id), From: Value(from), To: Value(to)}
}
```

`internal/domain/change/current.go`:

```go
package change

import (
	"bytes"
	"encoding/json"
	"slices"
)

// Absent は、値がないことを表すか(to が欠けているか null)。コメントと作業時間の削除の行がこれになる。
func Absent(v json.RawMessage) bool {
	s := bytes.TrimSpace(v)
	return len(s) == 0 || bytes.Equal(s, []byte("null"))
}

// rowsOf は、t と f が同じ行(1 つのコメントや作業時間の行の連なり)。create は含まない。
func rowsOf(log []Record, t Type, f string) []Record {
	var rows []Record
	for _, r := range log {
		if r.T == t && r.F == f {
			rows = append(rows, r)
		}
	}
	return rows
}

// Current は、t と f が同じ行から、いまの値を表す行を返す。先端(同じ項目の行の prev に挙がっていない行)のうち
// alive が真のものがあれば、その中で最もあとの行を選ぶ。一方が削除し他方が変更したときに変更を残すため
// (06「同時に編集したとき」)。alive が真のものがなければ、先端で最もあとの行(削除)を返す。
// 行がないか、prev が輪になって先端がなければ false。結果は log の行の並びによらない。
func Current(log []Record, t Type, f string, alive func(Record) bool) (Record, bool) {
	tips := tipsOf(rowsOf(log, t, f))
	var live []Record
	for _, r := range tips {
		if alive(r) {
			live = append(live, r)
		}
	}
	if w := winner(live); w != nil {
		return *w, true
	}
	if w := winner(tips); w != nil {
		return *w, true
	}
	return Record{}, false
}

// First は、t と f が同じ行のうち、最初に書いた行(投稿・記録)を返す。prev が同じ項目の行を指さない行のうち、
// at が最も古いもの(同じなら id が小さいもの)。prev が輪になっていれば、すべての行から選ぶ。
// 時計がずれて、あとの行の at が前になっていても、prev で決まる。
func First(log []Record, t Type, f string) (Record, bool) {
	rows := rowsOf(log, t, f)
	if len(rows) == 0 {
		return Record{}, false
	}
	ids := map[ID]bool{}
	for _, r := range rows {
		ids[r.ID] = true
	}
	var roots []Record
	for _, r := range rows {
		if !slices.ContainsFunc(r.Prev, func(p ID) bool { return ids[p] }) {
			roots = append(roots, r)
		}
	}
	if len(roots) == 0 {
		roots = rows
	}
	first := roots[0]
	for _, r := range roots[1:] {
		if r.At.Before(first.At) || (r.At.Equal(first.At) && r.ID < first.ID) {
			first = r
		}
	}
	return first, true
}
```

- [ ] **Step 4: 通ることを確かめて、コミットする**

Run: `go test ./internal/domain/change/`
Expected: PASS(`FuzzXxx` のシード、既存の `Resolve` のテストも通る)

```bash
git add internal/domain/change/
git commit -m "feat(domain): コメントの行の種類と、いまの値・最初の行の決め方を足す(A4a-7)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 5: `comment` の失敗するテストを書く**

`internal/domain/comment/comment_test.go`:

```go
package comment_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
)

func at(minute int) time.Time { return time.Date(2026, 11, 12, 10, minute, 0, 0, time.UTC) }

func row(id, f string, minute int, by string, from, to any, prev ...change.ID) change.Record {
	d := change.NewComment(change.ID(f), from, to)
	return change.Record{ID: change.ID(id), Op: change.ID(id), At: at(minute), By: by, Actor: change.User(by), T: d.T, F: d.F, From: d.From, To: d.To, Prev: prev}
}

func TestFromLogOrdersByPostAndMarksEdited(t *testing.T) {
	log := []change.Record{
		row("k1p", "K1", 1, "hanako", nil, comment.Value("最初"), "c"),
		row("k2p", "K2", 3, "taro", nil, comment.Value("二番目"), "c"),
		row("k1e", "K1", 5, "hanako", comment.Value("最初"), comment.Value("直した"), "k1p"),
	}
	got := comment.FromLog(log)
	want := []comment.Comment{
		{ID: "K1", By: "hanako", At: at(1), Body: "直した", Edited: true},
		{ID: "K2", By: "taro", At: at(3), Body: "二番目"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FromLog = %+v\nwant %+v", got, want)
	}
}

func TestFromLogLeavesOutDeletedAndUnreadable(t *testing.T) {
	log := []change.Record{
		row("k1p", "K1", 1, "hanako", nil, comment.Value("消す"), "c"),
		row("k1d", "K1", 2, "hanako", comment.Value("消す"), nil, "k1p"),
		row("k2p", "K2", 3, "taro", nil, "本文の形が違う", "c"),
		{ID: "t", Op: "t", At: at(4), By: "taro", Actor: change.User("taro"), T: change.Set, F: "title", To: json.RawMessage(`"x"`)},
	}
	if got := comment.FromLog(log); len(got) != 0 {
		t.Errorf("FromLog = %+v, want none", got)
	}
}

func TestFromLogKeepsAnEditAgainstAConcurrentDeleteInAnyOrder(t *testing.T) {
	log := []change.Record{
		row("p", "K1", 1, "hanako", nil, comment.Value("a"), "c"),
		row("e", "K1", 2, "taro", comment.Value("a"), comment.Value("b"), "p"),
		row("d", "K1", 9, "hanako", comment.Value("a"), nil, "p"),
	}
	got := comment.FromLog(log)
	if len(got) != 1 || got[0].Body != "b" || !got[0].Edited || got[0].By != "hanako" {
		t.Fatalf("FromLog = %+v", got)
	}
	rev := slices.Clone(log)
	slices.Reverse(rev)
	if !reflect.DeepEqual(comment.FromLog(rev), got) {
		t.Errorf("the reversed log gave %+v", comment.FromLog(rev))
	}
}

func TestFind(t *testing.T) {
	log := []change.Record{row("p", "K1", 1, "hanako", nil, comment.Value("a"), "c")}
	if c, ok := comment.Find(log, "K1"); !ok || c.Body != "a" {
		t.Errorf("Find K1 = %+v, %v", c, ok)
	}
	if _, ok := comment.Find(log, "K9"); ok {
		t.Error("Find of an unknown comment")
	}
}
```

- [ ] **Step 6: 失敗を確かめる**

Run: `go test ./internal/domain/comment/`
Expected: FAIL(`no non-test Go files in …/internal/domain/comment` またはビルドエラー `undefined: comment.Value`)

- [ ] **Step 7: 実装する**

`internal/domain/comment/comment.go`:

```go
// Package comment は、アイテムとプロジェクトのコメントの行の決め方を持つ(保存形式「変更記録」の comment)。
package comment

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Comment は、いまのコメント 1 件。By と At は、そのコメントの最初の行の by と at(投稿者と投稿の時刻)。
// Edited は、投稿のあとに編集の行があるとき真。
type Comment struct {
	ID     change.ID
	By     string
	At     time.Time
	Body   string
	Edited bool
}

type value struct {
	Body string `json:"body"`
}

// Value は、コメントの値(行の from・to に書く形)。
func Value(body string) any { return value{Body: body} }

func alive(r change.Record) bool { return !change.Absent(r.To) }

func bodyOf(raw json.RawMessage) (string, bool) {
	var v struct {
		Body *string `json:"body"`
	}
	if json.Unmarshal(raw, &v) != nil || v.Body == nil {
		return "", false
	}
	return *v.Body, true
}

// FromLog は、ログのいまのコメントを、投稿の古い順(同じなら ID の順)で返す。削除されたものと、
// 本文を読めないものは除く。いまの値は change.Current が決める(先端に削除と編集があれば編集を残す)。
func FromLog(log []change.Record) []Comment {
	ids := map[change.ID]bool{}
	for _, r := range log {
		if r.T == change.Comment {
			ids[change.ID(r.F)] = true
		}
	}
	var out []Comment
	for id := range ids {
		if c, ok := Find(log, id); ok {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b Comment) int {
		return cmp.Or(a.At.Compare(b.At), strings.Compare(string(a.ID), string(b.ID)))
	})
	return out
}

// Find は、いまのコメントを 1 件返す。ないか、削除されているか、本文を読めなければ false。
func Find(log []change.Record, id change.ID) (Comment, bool) {
	f := string(id)
	cur, ok := change.Current(log, change.Comment, f, alive)
	if !ok || change.Absent(cur.To) {
		return Comment{}, false
	}
	body, ok := bodyOf(cur.To)
	if !ok {
		return Comment{}, false
	}
	first, _ := change.First(log, change.Comment, f)
	return Comment{ID: id, By: first.By, At: first.At, Body: body, Edited: cur.ID != first.ID}, true
}
```

- [ ] **Step 8: 通ることを確かめて、コミットする**

Run: `go test ./internal/domain/comment/ ./internal/domain/change/`
Expected: PASS

```bash
git add internal/domain/comment/
git commit -m "feat(domain): コメントの行から、いまのコメントの一覧を決める(A4a-7)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 9: 操作の失敗するテストを書く**

`internal/app/comments/comments_test.go`(アクティビティの 1 行を覚える `recorder` は、`internal/app/items/edit_test.go` と同じ形をこのパッケージに書く。app の機能どうしは import しない):

```go
package comments_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/comments"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const store port.StoreID = "S1"

var (
	onTask    = comments.Target{Kind: change.KindItem, ID: apptest.SampleTask}
	onProject = comments.Target{Kind: change.KindProject, ID: apptest.SampleProject}
)

// recorder は、Write に渡したアクティビティの 1 行を覚える。
type recorder struct {
	*apptest.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func setup(t *testing.T, login string) (comments.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	if err := apptest.Sample(t.Context(), s, store); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return comments.Service{Stores: rec, IDs: ids, Identity: apptest.Identity{Login: login}}, rec
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func logLen(t *testing.T, s port.Stores, tg comments.Target) int {
	t.Helper()
	n := 0
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: tg.Kind, ID: tg.ID})
		n = len(log)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPostThenList(t *testing.T) {
	svc, rec := setup(t, "hanako")
	v, err := svc.Post(t.Context(), store, onTask, "  @taro 見てください\n")
	if err != nil {
		t.Fatal(err)
	}
	if v.Body != "@taro 見てください" || v.By != "hanako" || v.ByName != "hanako" || v.Edited || v.ID == "" {
		t.Errorf("posted %+v", v)
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "HH-2 にコメント" {
		t.Errorf("summary = %q", got)
	}
	list, err := svc.List(t.Context(), store, onTask)
	if err != nil || len(list) != 1 || list[0].ID != v.ID {
		t.Errorf("List = %+v, %v", list, err)
	}
	if other, _ := svc.List(t.Context(), store, onProject); len(other) != 0 {
		t.Errorf("a comment on the item showed up on the project: %+v", other)
	}
}

func TestPostOnAProject(t *testing.T) {
	svc, rec := setup(t, "taro")
	if _, err := svc.Post(t.Context(), store, onProject, "概要へ"); err != nil {
		t.Fatal(err)
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "HH-P1 にコメント" {
		t.Errorf("summary = %q", got)
	}
	if list, _ := svc.List(t.Context(), store, onProject); len(list) != 1 || list[0].Body != "概要へ" {
		t.Errorf("List = %+v", list)
	}
}

func TestPostAnEmptyBodyIsRefusedAndSavesNothing(t *testing.T) {
	svc, rec := setup(t, "hanako")
	before := logLen(t, rec, onTask)
	if _, err := svc.Post(t.Context(), store, onTask, " \n "); message(err) != "コメントを入力してください" {
		t.Errorf("err = %v", err)
	}
	if logLen(t, rec, onTask) != before {
		t.Error("a refused post wrote a row")
	}
}

func TestAuthorMissingFromMembersIsAnUnknownUser(t *testing.T) {
	svc, _ := setup(t, "ghost")
	v, err := svc.Post(t.Context(), store, onTask, "こんにちは")
	if err != nil || v.By != "ghost" || v.ByName != "不明なユーザー" {
		t.Errorf("posted %+v, %v", v, err)
	}
}

func TestEdit(t *testing.T) {
	svc, rec := setup(t, "hanako")
	v, _ := svc.Post(t.Context(), store, onTask, "前")
	before := logLen(t, rec, onTask)
	same, err := svc.Edit(t.Context(), store, onTask, v.ID, " 前 ")
	if err != nil || same.Edited || logLen(t, rec, onTask) != before {
		t.Errorf("an unchanged edit: %+v, %v; log %d → %d", same, err, before, logLen(t, rec, onTask))
	}
	got, err := svc.Edit(t.Context(), store, onTask, v.ID, "後")
	if err != nil || got.Body != "後" || !got.Edited || got.By != "hanako" {
		t.Errorf("edited %+v, %v", got, err)
	}
	if last := rec.summaries[len(rec.summaries)-1]; last != "コメントを編集" {
		t.Errorf("summary = %q", last)
	}
	if _, err := svc.Edit(t.Context(), store, onTask, v.ID, "  "); message(err) != "コメントを入力してください" {
		t.Errorf("an empty edit: %v", err)
	}
	if _, err := svc.Edit(t.Context(), store, onTask, "01NOPE00000000000000000000", "x"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("an unknown comment: %v", err)
	}
}

func TestDelete(t *testing.T) {
	svc, rec := setup(t, "hanako")
	v, _ := svc.Post(t.Context(), store, onTask, "消す")
	if err := svc.Delete(t.Context(), store, onTask, v.ID); err != nil {
		t.Fatal(err)
	}
	if last := rec.summaries[len(rec.summaries)-1]; last != "コメントを削除" {
		t.Errorf("summary = %q", last)
	}
	if list, _ := svc.List(t.Context(), store, onTask); len(list) != 0 {
		t.Errorf("List after delete = %+v", list)
	}
	if err := svc.Delete(t.Context(), store, onTask, v.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("deleting twice: %v", err)
	}
	if _, err := svc.Edit(t.Context(), store, onTask, v.ID, "戻す"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("editing a deleted comment: %v", err)
	}
}

func TestOnlyTheAuthorCanEditOrDelete(t *testing.T) {
	svc, _ := setup(t, "hanako")
	v, err := svc.Post(t.Context(), store, onTask, "自分の")
	if err != nil {
		t.Fatal(err)
	}
	other := svc
	other.Identity = apptest.Identity{Login: "taro"}
	if _, err := other.Edit(t.Context(), store, onTask, v.ID, "乗っ取り"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("another person's edit: %v", err)
	}
	if err := other.Delete(t.Context(), store, onTask, v.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("another person's delete: %v", err)
	}
	if list, _ := svc.List(t.Context(), store, onTask); len(list) != 1 || list[0].Body != "自分の" || list[0].Edited {
		t.Errorf("a refused edit or delete changed the comment: %+v", list)
	}
	// 同じ人なら、ログイン名の大文字と小文字が違っても自分のコメント。
	same := svc
	same.Identity = apptest.Identity{Login: "HANAKO"}
	if got, err := same.Edit(t.Context(), store, onTask, v.ID, "大文字でも自分"); err != nil || got.Body != "大文字でも自分" {
		t.Errorf("own edit with another case: %+v, %v", got, err)
	}
	if err := same.Delete(t.Context(), store, onTask, v.ID); err != nil {
		t.Errorf("own delete with another case: %v", err)
	}
}

func TestUnknownOwner(t *testing.T) {
	svc, _ := setup(t, "hanako")
	gone := comments.Target{Kind: change.KindItem, ID: "01NOPE00000000000000000000"}
	if _, err := svc.List(t.Context(), store, gone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("List: %v", err)
	}
	if _, err := svc.Post(t.Context(), store, gone, "x"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Post: %v", err)
	}
	// 種類と ID が合わない(プロジェクトの ID をアイテムとして指す)ときも、ないものとして扱う。
	wrong := comments.Target{Kind: change.KindItem, ID: apptest.SampleProject}
	if _, err := svc.Post(t.Context(), store, wrong, "x"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Post with a mismatched kind: %v", err)
	}
}
```

- [ ] **Step 10: 失敗を確かめる**

Run: `go test ./internal/app/comments/`
Expected: FAIL(`no non-test Go files in …/internal/app/comments`)

- [ ] **Step 11: 実装する**

`internal/app/comments/service.go`:

```go
// Package comments は、アイテムとプロジェクトのコメントの操作(02「コメント」)を持つ。1 ファイル 1 操作。
package comments

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
)

const (
	emptyBody   = "コメントを入力してください"
	unknownUser = "不明なユーザー" // 01「画面の移り方と、共通の表示」
)

// Service は、コメントの操作が使う口。
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
}

// Target は、コメントの持ち主。Kind は change.KindItem か change.KindProject。
type Target struct {
	Kind change.Kind
	ID   change.ID
}

func (t Target) ref() change.Ref { return change.Ref{Kind: t.Kind, ID: t.ID} }

// View は、画面に出すコメント。ByName は投稿者の表示名で、メンバーにいなければ「不明なユーザー」。
type View struct {
	comment.Comment
	ByName string
}

// display は、持ち主の画面の ID(HH-12・HH-P1)。持ち主がなければ port.ErrNotFound。
func display(r port.Reader, t Target) (string, error) {
	switch t.Kind {
	case change.KindItem:
		it, err := r.Items().Get(t.ID)
		if err != nil {
			return "", err
		}
		w, err := r.Workspaces().Get(it.Place.Workspace)
		if err != nil {
			return "", err
		}
		return it.DisplayID(w.Prefix), nil
	case change.KindProject:
		p, err := r.Projects().Get(t.ID)
		if err != nil {
			return "", err
		}
		w, err := r.Workspaces().Get(p.Place.Workspace)
		if err != nil {
			return "", err
		}
		return p.DisplayID(w.Prefix), nil
	}
	return "", port.ErrNotFound
}

// appendTo は、持ち主のログに行を足す。
func appendTo(tx port.Tx, t Target, drafts ...change.Draft) error {
	switch t.Kind {
	case change.KindItem:
		return tx.WriteItems().Append(t.ID, drafts...)
	case change.KindProject:
		return tx.WriteProjects().Append(t.ID, drafts...)
	}
	return port.ErrNotFound
}

// displayOf は、アクティビティの 1 行に書く持ち主の ID。保存の前に読む。
func (s Service) displayOf(ctx context.Context, store port.StoreID, t Target) (string, error) {
	var shown string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		shown, err = display(r, t)
		return err
	})
	return shown, err
}

// currentOf は、持ち主のログのいまのコメント。ないか削除されていれば port.ErrNotFound。
func currentOf(r port.Reader, t Target, id change.ID) (comment.Comment, error) {
	log, err := r.Changes().Of(t.ref())
	if err != nil {
		return comment.Comment{}, err
	}
	c, ok := comment.Find(log, id)
	if !ok {
		return comment.Comment{}, port.ErrNotFound
	}
	return c, nil
}

// names は、ログイン名から表示名を引く関数(大文字と小文字を区別しない。メンバーにいなければ「不明なユーザー」)。
func names(r port.Reader) (func(login string) string, error) {
	ms, err := r.Members().List()
	if err != nil {
		return nil, err
	}
	byLogin := map[string]string{}
	for _, m := range ms {
		byLogin[strings.ToLower(m.Login)] = m.Name
	}
	return func(login string) string {
		if n, ok := byLogin[strings.ToLower(login)]; ok {
			return n
		}
		return unknownUser
	}, nil
}

// find は、書いたあとのコメントを読み直して返す。
func (s Service) find(ctx context.Context, store port.StoreID, t Target, id change.ID) (View, error) {
	list, err := s.List(ctx, store, t)
	if err != nil {
		return View{}, err
	}
	for _, v := range list {
		if v.ID == id {
			return v, nil
		}
	}
	return View{}, port.ErrNotFound
}
```

`list.go`:

```go
package comments

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/comment"
)

// List は、持ち主のコメントを古い順にすべて返す(02「コメント」)。持ち主がなければ port.ErrNotFound。
func (s Service) List(ctx context.Context, store port.StoreID, t Target) ([]View, error) {
	var out []View
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := display(r, t); err != nil {
			return err
		}
		log, err := r.Changes().Of(t.ref())
		if err != nil {
			return err
		}
		nameOf, err := names(r)
		if err != nil {
			return err
		}
		for _, c := range comment.FromLog(log) {
			out = append(out, View{Comment: c, ByName: nameOf(c.By)})
		}
		return nil
	})
	return out, err
}
```

`post.go`:

```go
package comments

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// Post は、コメントを投稿する(02「コメント」の投稿)。本文の前後の空白を除き、空なら断る。
// アクティビティは「{ID} にコメント」。投稿したコメントを返す。
func (s Service) Post(ctx context.Context, store port.StoreID, t Target, body string) (View, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return View{}, refusal.New(emptyBody)
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return View{}, err
	}
	shown, err := s.displayOf(ctx, store, t)
	if err != nil {
		return View{}, err
	}
	id := s.IDs.New()
	op := port.Operation{Actor: change.User(me), Summary: shown + " にコメント"}
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		if _, err := display(tx, t); err != nil {
			return err
		}
		return appendTo(tx, t, change.NewComment(id, nil, comment.Value(body)))
	})
	if err != nil {
		return View{}, err
	}
	return s.find(ctx, store, t, id)
}
```

`edit.go`:

```go
package comments

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// Edit は、コメントの本文を変える(02「コメント」の編集)。前後の空白を除いたあとが空なら断り、
// 前と同じなら何も保存しない。アクティビティは「コメントを編集」。ないか削除されたコメント、
// 投稿者が「あなた」でないコメントは port.ErrNotFound(画面からは起きない操作。ログイン名は大文字と小文字を区別しない)。
func (s Service) Edit(ctx context.Context, store port.StoreID, t Target, id change.ID, body string) (View, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return View{}, refusal.New(emptyBody)
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return View{}, err
	}
	op := port.Operation{Actor: change.User(me), Summary: "コメントを編集"}
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		cur, err := currentOf(tx, t, id)
		if err != nil {
			return err
		}
		if !strings.EqualFold(cur.By, me) {
			return port.ErrNotFound
		}
		if cur.Body == body {
			return nil // 行のない Write は何も残さない
		}
		return appendTo(tx, t, change.NewComment(id, comment.Value(cur.Body), comment.Value(body)))
	})
	if err != nil {
		return View{}, err
	}
	return s.find(ctx, store, t, id)
}
```

`delete.go`:

```go
package comments

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
)

// Delete は、コメントを削除する(to を null にした行を足す)。アクティビティは「コメントを削除」。
// ないか、すでに削除されたコメント、投稿者が「あなた」でないコメントは port.ErrNotFound
// (画面からは起きない操作。ログイン名は大文字と小文字を区別しない)。
func (s Service) Delete(ctx context.Context, store port.StoreID, t Target, id change.ID) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: "コメントを削除"}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		cur, err := currentOf(tx, t, id)
		if err != nil {
			return err
		}
		if !strings.EqualFold(cur.By, me) {
			return port.ErrNotFound
		}
		return appendTo(tx, t, change.NewComment(id, comment.Value(cur.Body), nil))
	})
}
```

- [ ] **Step 12: 通ることを確かめて、コミットする**

Run: `go test ./internal/app/comments/ ./internal/archtest/`
Expected: PASS(archtest は、app のパッケージが別の app の機能を import していないことと、ファイル数・行数の上限を確かめる)

```bash
git add internal/app/comments/
git commit -m "feat(app): アイテムとプロジェクトのコメントの一覧・投稿・編集・削除(A4a-7)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 13: API の失敗するテストを書く**

`internal/adapter/httpapi/notes/comments_test.go`:

```go
package notes_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/notes"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appcomments "github.com/kukv/hedgehug/internal/app/comments"
)

func newCommentsAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	notes.Register(api, notes.Services{
		Comments: appcomments.Service{Stores: s, IDs: ids, Identity: apptest.Identity{Login: "hanako"}},
	})
	return api
}

func TestItemCommentLifecycle(t *testing.T) {
	api := newCommentsAPI(t)
	base := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/comments"
	res := api.Post(base, map[string]any{"body": " @taro 見てください "})
	body := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(body, `"body":"@taro 見てください"`) ||
		!strings.Contains(body, `"byName":"hanako"`) || !strings.Contains(body, `"edited":false`) {
		t.Fatalf("post: %d %s", res.Code, body)
	}
	var posted notes.Comment
	if err := json.Unmarshal(res.Body.Bytes(), &posted); err != nil || posted.ID == "" || posted.At == "" {
		t.Fatalf("posted = %+v, %v", posted, err)
	}
	one := base + "/" + posted.ID
	if res := api.Put(one, map[string]any{"body": "直した"}); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"edited":true`) {
		t.Errorf("edit: %d %s", res.Code, res.Body)
	}
	if res := api.Get(base); res.Code != http.StatusOK || strings.Count(res.Body.String(), `"id":`) != 1 {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
	if res := api.Delete(one); res.Code != http.StatusNoContent {
		t.Errorf("delete: %d %s", res.Code, res.Body)
	}
	if res := api.Get(base); res.Code != http.StatusOK || res.Body.String() != "[]\n" {
		t.Errorf("list after delete: %d %q", res.Code, res.Body)
	}
	if res := api.Delete(one); res.Code != http.StatusNotFound {
		t.Errorf("delete twice: %d", res.Code)
	}
}

func TestProjectComments(t *testing.T) {
	api := newCommentsAPI(t)
	base := "/api/stores/S1/projects/" + string(apptest.SampleProject) + "/comments"
	if res := api.Post(base, map[string]any{"body": "概要へ"}); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"body":"概要へ"`) {
		t.Fatalf("post: %d %s", res.Code, res.Body)
	}
	if res := api.Get(base); res.Code != http.StatusOK || strings.Count(res.Body.String(), `"id":`) != 1 {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
	// アイテムのコメントには出ない。
	if res := api.Get("/api/stores/S1/items/" + string(apptest.SampleTask) + "/comments"); res.Body.String() != "[]\n" {
		t.Errorf("item list: %s", res.Body)
	}
}

func TestCommentRefusalsAndUnknowns(t *testing.T) {
	api := newCommentsAPI(t)
	base := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/comments"
	res := api.Post(base, map[string]any{"body": "  "})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "コメントを入力してください") {
		t.Errorf("empty: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items/01NOPE00000000000000000000/comments", map[string]any{"body": "x"}); res.Code != http.StatusNotFound {
		t.Errorf("unknown item: %d", res.Code)
	}
	if res := api.Get("/api/stores/S1/projects/01NOPE00000000000000000000/comments"); res.Code != http.StatusNotFound {
		t.Errorf("unknown project: %d", res.Code)
	}
	if res := api.Put(base+"/01NOPE00000000000000000000", map[string]any{"body": "x"}); res.Code != http.StatusNotFound {
		t.Errorf("unknown comment: %d", res.Code)
	}
}
```

- [ ] **Step 14: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/notes/`
Expected: FAIL(ビルドエラー `unknown field Comments in struct literal of type notes.Services`)

- [ ] **Step 15: 実装する**

`internal/adapter/httpapi/notes/comments.go`:

```go
package notes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appcomments "github.com/kukv/hedgehug/internal/app/comments"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// Comment は、API が返すコメント。
type Comment struct {
	ID     string `json:"id"`
	By     string `json:"by" doc:"投稿者のログイン名"`
	ByName string `json:"byName" doc:"投稿者の表示名。メンバーにいなければ「不明なユーザー」"`
	At     string `json:"at" doc:"投稿した時刻(RFC 3339)"`
	Body   string `json:"body"`
	Edited bool   `json:"edited"`
}

// CommentText は、コメントの投稿と編集の本文。
type CommentText struct {
	Body string `json:"body"`
}

type commentOutput struct{ Body Comment }

type commentsOutput struct{ Body []Comment }

func toComment(v appcomments.View) Comment {
	return Comment{ID: string(v.ID), By: v.By, ByName: v.ByName, At: v.At.Format(change.TimeLayout), Body: v.Body, Edited: v.Edited}
}

func oneComment(v appcomments.View, err error) (*commentOutput, error) {
	if err != nil {
		return nil, apierr.From(err)
	}
	return &commentOutput{Body: toComment(v)}, nil
}

func manyComments(vs []appcomments.View, err error) (*commentsOutput, error) {
	if err != nil {
		return nil, apierr.From(err)
	}
	out := &commentsOutput{Body: []Comment{}}
	for _, v := range vs {
		out.Body = append(out.Body, toComment(v))
	}
	return out, nil
}

// アイテムのコメントの入力。パスの名前が違うので、プロジェクトのものと型を分ける。
type itemRef struct {
	Store string `path:"store"`
	Item  string `path:"item"`
}

type itemCommentRef struct {
	Store   string `path:"store"`
	Item    string `path:"item"`
	Comment string `path:"comment"`
}

type postItemComment struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  CommentText
}

type editItemComment struct {
	Store   string `path:"store"`
	Item    string `path:"item"`
	Comment string `path:"comment"`
	Body    CommentText
}

type projectRef struct {
	Store   string `path:"store"`
	Project string `path:"project"`
}

type projectCommentRef struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Comment string `path:"comment"`
}

type postProjectComment struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Body    CommentText
}

type editProjectComment struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Comment string `path:"comment"`
	Body    CommentText
}

// registerComments は、アイテムとプロジェクトのコメントの API を登録する。
func registerComments(api huma.API, svc appcomments.Service) {
	registerItemComments(api, svc)
	registerProjectComments(api, svc)
}

func registerItemComments(api huma.API, svc appcomments.Service) {
	at := func(id string) appcomments.Target { return appcomments.Target{Kind: change.KindItem, ID: change.ID(id)} }
	const base = "/api/stores/{store}/items/{item}/comments"
	huma.Register(api, huma.Operation{OperationID: "list-item-comments", Method: http.MethodGet, Path: base, Summary: "アイテムのコメントを古い順に返す"},
		func(ctx context.Context, in *itemRef) (*commentsOutput, error) {
			return manyComments(svc.List(ctx, port.StoreID(in.Store), at(in.Item)))
		})
	huma.Register(api, huma.Operation{OperationID: "post-item-comment", Method: http.MethodPost, Path: base, Summary: "アイテムにコメントを投稿する"},
		func(ctx context.Context, in *postItemComment) (*commentOutput, error) {
			return oneComment(svc.Post(ctx, port.StoreID(in.Store), at(in.Item), in.Body.Body))
		})
	huma.Register(api, huma.Operation{OperationID: "edit-item-comment", Method: http.MethodPut, Path: base + "/{comment}", Summary: "アイテムのコメントを編集する"},
		func(ctx context.Context, in *editItemComment) (*commentOutput, error) {
			return oneComment(svc.Edit(ctx, port.StoreID(in.Store), at(in.Item), change.ID(in.Comment), in.Body.Body))
		})
	huma.Register(api, huma.Operation{OperationID: "delete-item-comment", Method: http.MethodDelete, Path: base + "/{comment}", Summary: "アイテムのコメントを削除する", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *itemCommentRef) (*struct{}, error) {
			return nil, apierr.From(svc.Delete(ctx, port.StoreID(in.Store), at(in.Item), change.ID(in.Comment)))
		})
}

func registerProjectComments(api huma.API, svc appcomments.Service) {
	at := func(id string) appcomments.Target { return appcomments.Target{Kind: change.KindProject, ID: change.ID(id)} }
	const base = "/api/stores/{store}/projects/{project}/comments"
	huma.Register(api, huma.Operation{OperationID: "list-project-comments", Method: http.MethodGet, Path: base, Summary: "プロジェクトのコメントを古い順に返す"},
		func(ctx context.Context, in *projectRef) (*commentsOutput, error) {
			return manyComments(svc.List(ctx, port.StoreID(in.Store), at(in.Project)))
		})
	huma.Register(api, huma.Operation{OperationID: "post-project-comment", Method: http.MethodPost, Path: base, Summary: "プロジェクトにコメントを投稿する"},
		func(ctx context.Context, in *postProjectComment) (*commentOutput, error) {
			return oneComment(svc.Post(ctx, port.StoreID(in.Store), at(in.Project), in.Body.Body))
		})
	huma.Register(api, huma.Operation{OperationID: "edit-project-comment", Method: http.MethodPut, Path: base + "/{comment}", Summary: "プロジェクトのコメントを編集する"},
		func(ctx context.Context, in *editProjectComment) (*commentOutput, error) {
			return oneComment(svc.Edit(ctx, port.StoreID(in.Store), at(in.Project), change.ID(in.Comment), in.Body.Body))
		})
	huma.Register(api, huma.Operation{OperationID: "delete-project-comment", Method: http.MethodDelete, Path: base + "/{comment}", Summary: "プロジェクトのコメントを削除する", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *projectCommentRef) (*struct{}, error) {
			return nil, apierr.From(svc.Delete(ctx, port.StoreID(in.Store), at(in.Project), change.ID(in.Comment)))
		})
}
```

(`return nil, apierr.From(...)` は、成功なら `nil, nil` になる。`apierr.From(nil)` は nil を返す。既存の `move-item-up` が `nilnil` の指摘を `//nolint:nilnil` で抑えているのは `return nil, nil` と書くためで、ここは書かないので要らない。)

`internal/adapter/httpapi/notes/notes.go`(Task 3 が作る)に足す。import に `appcomments "github.com/kukv/hedgehug/internal/app/comments"` を足す。

```go
type Services struct {
	Descriptions appdescriptions.Service
	Comments     appcomments.Service
}

func Register(api huma.API, s Services) {
	registerDescriptions(api, s.Descriptions)
	registerComments(api, s.Comments)
}
```

`internal/adapter/httpapi/routes.go` の `notes.Register(api, notes.Services{…})` に足す(import に `appcomments "github.com/kukv/hedgehug/internal/app/comments"`)。

```go
		Comments: appcomments.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity},
```

- [ ] **Step 16: 生成して、通ることを確かめて、コミットする**

Run: `make generate && pnpm -C web gen`
Expected: `api/openapi.yaml` と `web/packages/api/src/generated.ts` に `list-item-comments` など 8 つの operation(`listItemComments`・`postItemComment`・`editItemComment`・`deleteItemComment` と、`Project` の 4 つ)と、スキーマ `Comment`・`CommentText` が出る。

Run: `go test ./internal/... && pnpm -C web check`
Expected: PASS(`TestOpenAPIYAMLIsUpToDate` も通る)

```bash
git add internal api web/packages/api/src/generated.ts
git commit -m "feat(api): アイテムとプロジェクトのコメントの API を足す(A4a-7・A4a-24)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 10: コメントの区画とメンションのボタン

**Files:**
- Create: `web/packages/core/src/model/notes.ts`(`Comment`。Task 12 が `Worklog` を足す)
- Modify: `web/packages/core/src/port/notes.ts`(Task 7 が作る。`NotesPort` にコメントのメソッドを足す)
- Modify: `web/packages/core/src/features/notes/fixtures.test-util.ts`(Task 7 の `fakeNotesPort` に、コメントのメソッドを足す。`NotesPort` にメソッドが増えると偽物の型が合わなくなるので、同じコミットで)
- Modify: `web/packages/api/src/notes.ts`・`web/packages/api/src/notes.test.ts`(Task 7 が作る。`createNotesPort` にコメントのメソッドを足す。`api/src/index.ts` の export は Task 7 で済み)
- Create: `web/packages/core/src/features/notes/mentions.ts`、Test: `mentions.test.ts`(`mentionTargets`・`appendMention` だけ。`isMemberOf` は Task 5 の `markdown.ts`)
- Create: `web/packages/core/src/features/notes/comments.ts`、Test: `comments.test.ts`
- Modify: `web/packages/core/src/notesIndex.ts`(Task 5 が作る。A4a の core の export はここに足す。`index.ts` は変えない)
- Create: `web/app/src/screens/notes/CommentsSection.svelte`・`CommentItem.svelte`
- Modify: `web/app/src/wire.ts`(`itemComments`・`projectComments`、画面への反映)
- Modify: `web/app/src/screens/item/DetailPanel.svelte`(アイテムの詳細パネルにコメントの区画を足す)

**Interfaces:**
- Consumes:
  - API `list-item-comments`・`post-item-comment`・`edit-item-comment`・`delete-item-comment` と `list-project-comments` など(Task 9)。orval が作る `listItemComments(store, item)`・`postItemComment(store, item, body)`・`editItemComment(store, item, comment, body)`・`deleteItemComment(store, item, comment)` と、`Project` の同じ形の 4 つ、型 `Comment`
  - `tokenize(text: string, isMember: (login: string) => boolean): Block[]`(Task 5)、`Markdown.svelte`(props `{ blocks: Block[] }`。`@hedgehug/ui` が `Markdown` として export する。Task 5)
  - `Drafts`(Task 6。`register(key: string, dirty: () => boolean, held?: () => boolean): () => void` と `confirmLeave(key?: string): Promise<boolean>`)と、`web/app/src/drafts.ts` の `drafts`(`wire.ts` が import 済み)
  - `NotesPort`・`NoteTarget`・`fakeNotesPort`・`createNotesPort`・`wire.ts` の `notes`(Task 7)、`Textarea`(`@hedgehug/ui`。Task 7。props `{ value?(bindable); label; rows?; placeholder?; disabled?; title?; focusOnMount?; onkeydown? }`)、`isMemberOf`(Task 5)
  - `relativeTime(at: Date, now: Date)`・`syncNow`(wire)、`ConfirmDialog`、`Avatar`、`readOnlyNote`、`Toasts`、`CatalogPort.listMembers`・`me`
- Produces:
  - `Comment { id; by; byName; at; body; edited }`(`web/packages/core/src/model/notes.ts`)
  - `NotesPort.listComments(target): Promise<Comment[]>`・`postComment(target, body): Promise<Comment>`・`editComment(target, commentId, body): Promise<Comment>`・`deleteComment(target, commentId): Promise<void>`
  - `mentionTargets(members: Member[], me: string): string[]`、`appendMention(text: string, login: string): string`
  - `createComments(deps: CommentsDeps): Comments`(`CommentsDeps = { notes: Pick<NotesPort, "listComments" | "postComment" | "editComment" | "deleteComment">; catalogPort: Pick<CatalogPort, "listMembers" | "me">; toasts: Toasts; drafts: Pick<Drafts, "register" | "confirmLeave">; scope: "item" | "project" }`)
  - `Comments { state: Readable<CommentsState>; open(target): Promise<void>; reload(): Promise<void>; close(): void; setNew(text): void; post(): Promise<boolean>; escapeNew(): Promise<void>; startEdit(id): void; setEditing(text): void; cancelEdit(): void; saveEdit(): Promise<boolean>; escapeEdit(): Promise<void>; askDelete(id): void; cancelDelete(): void; confirmDelete(): Promise<void> }`、`isMine(c: Comment, me: string): boolean`、`commentAvatar(c: Comment): string`、文言 `commentPlaceholder`・`commentDeletedMessage`・`commentDeleteDialog`
  - `wire.ts` の `itemComments`・`projectComments`(`Comments`。詳細パネルとプロジェクトの概要が同時に開いても、入力が混ざらないよう 2 つ作る)
  - `CommentsSection.svelte`(props `{ target: NoteTarget; readonly: boolean }`。アイテムの詳細パネルとプロジェクトの概要(Task 19)が使う。使う側では `target.kind` を変えない)

**書きかけと破棄の確認(A4a-21・22):** 新しいコメントの入力は、空白だけでないとき、編集の入力は、保存してある本文と違うとき、書きかけとして `drafts.register` に登録する(キーは `comment:new:<scope>`・`comment:edit:<scope>`。`comment:` で始めると、確認の本文「保存していないコメントを破棄します。」が選ばれる。Task 6)。編集の `held`(反映を待つ条件)は、保存と違い、かつ空白だけでないとき。入力欄の中の Esc は、書きかけがあれば `drafts.confirmLeave(その入力欄のキー)` で、その入力欄のぶんだけ確認する。日本語の変換中の Esc は対象にしない。入力欄の下の「キャンセル」は確認しない。

- [ ] **Step 1: メンションの失敗するテストを書く**

`web/packages/core/src/features/notes/mentions.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { appendMention, mentionTargets } from "./mentions";

const members = [
  { login: "taro", name: "太郎" },
  { login: "Hanako", name: "花子" },
  { login: "jiro", name: "次郎" },
];

describe("mentions", () => {
  it("lists the logins of the members other than me, in the member order", () => {
    expect(mentionTargets(members, "hanako")).toEqual(["taro", "jiro"]);
    expect(mentionTargets(members, "")).toEqual(["taro", "Hanako", "jiro"]);
  });

  it("appends the login and a space to the end of the text", () => {
    expect(appendMention("", "taro")).toBe("@taro ");
    expect(appendMention("見てください ", "taro")).toBe("見てください @taro ");
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- mentions`
Expected: FAIL(`Failed to resolve import "./mentions"`)

- [ ] **Step 3: 実装する**

`web/packages/core/src/features/notes/mentions.ts`:

```ts
import type { Member } from "../../model/member";

// メンション(02「メンション」)。ログイン名は、大文字と小文字を区別せずに比べる(01)。
// `@ログイン名` をメンションの見た目にする判定 isMemberOf は、Markdown の字句分け(Task 5)にある。

// mentionTargets は、コメントの入力欄の下に並べるメンションのボタンのログイン名(あなた以外のメンバー。A4a-23)。
export function mentionTargets(members: Member[], me: string): string[] {
  return members.filter((m) => m.login.toLowerCase() !== me.toLowerCase()).map((m) => m.login);
}

// appendMention は、入力の末尾に `@ログイン名 ` を足す。
export function appendMention(text: string, login: string): string {
  return `${text}@${login} `;
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- mentions`
Expected: PASS

- [ ] **Step 5: 口の失敗するテストを書く**

`web/packages/api/src/notes.test.ts` に足す(Task 7 が作ったファイルの `respond`・`call`・`item`・`project` を使う。先頭の import は `import { NotFound, Refused } from "@hedgehug/core";` にする):

```ts
const apiComment = {
  id: "K1",
  by: "hanako",
  byName: "花子",
  at: "2026-11-13T09:00:00.000Z",
  body: "見てください",
  edited: false,
};

describe("notes port: comments", () => {
  it("lists the comments of an item and of a project, reading null as empty", async () => {
    const fetch = respond(200, [apiComment]);
    await expect(createNotesPort().listComments(item)).resolves.toEqual([apiComment]);
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/comments");
    const none = respond(200, null);
    await expect(createNotesPort().listComments(project)).resolves.toEqual([]);
    expect(call(none)[0]).toBe("/api/stores/S1/projects/P1/comments");
  });

  it("posts, edits and deletes", async () => {
    const posted = respond(200, apiComment);
    await createNotesPort().postComment(item, "見てください");
    expect(call(posted)[0]).toBe("/api/stores/S1/items/I1/comments");
    expect(call(posted)[1].method).toBe("POST");
    expect(JSON.parse(String(call(posted)[1].body))).toEqual({ body: "見てください" });

    const edited = respond(200, { ...apiComment, edited: true });
    await expect(createNotesPort().editComment(project, "K1", "直した")).resolves.toMatchObject({
      edited: true,
    });
    expect(call(edited)[0]).toBe("/api/stores/S1/projects/P1/comments/K1");
    expect(call(edited)[1].method).toBe("PUT");

    const deleted = respond(204, undefined);
    await expect(createNotesPort().deleteComment(item, "K1")).resolves.toBeUndefined();
    expect(call(deleted)[0]).toBe("/api/stores/S1/items/I1/comments/K1");
    expect(call(deleted)[1].method).toBe("DELETE");
  });

  it("turns a 422 into Refused and a 404 into NotFound", async () => {
    respond(422, { title: "Unprocessable Entity", status: 422, detail: "コメントを入力してください" });
    const err = await createNotesPort()
      .postComment(item, " ")
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(Refused);
    expect((err as Refused).message).toBe("コメントを入力してください");
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    await expect(createNotesPort().listComments(item)).rejects.toBeInstanceOf(NotFound);
  });
});
```

- [ ] **Step 6: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/api test -- notes`
Expected: FAIL(`Failed to resolve import "./notes"`)

- [ ] **Step 7: 実装する**

`web/packages/core/src/model/notes.ts`(新規):

```ts
// コメント 1 件(02「コメント」)。by は投稿者のログイン名、byName は表示名(メンバーにいなければ「不明なユーザー」)、
// at は投稿の時刻(RFC 3339)。edited は、投稿のあとに編集されているとき真。
export interface Comment {
  id: string;
  by: string;
  byName: string;
  at: string;
  body: string;
  edited: boolean;
}
```

`web/packages/core/src/port/notes.ts` の `NotesPort` に足す(`import type { Comment } from "../model/notes";` を足す)。

```ts
  // listComments は、コメントを古い順にすべて返す。
  listComments(target: NoteTarget): Promise<Comment[]>;
  postComment(target: NoteTarget, body: string): Promise<Comment>;
  editComment(target: NoteTarget, commentId: string, body: string): Promise<Comment>;
  deleteComment(target: NoteTarget, commentId: string): Promise<void>;
```

`web/packages/core/src/features/notes/fixtures.test-util.ts` の `fakeNotesPort` が返すオブジェクトに、型を合わせるためのメソッドを足す(呼ばれたら投げる単純なスタブ。コメントの試験は、この偽物でなく、`createComments` の `Pick` した口に自前の関数を渡す)。

```ts
    async listComments() {
      return [];
    },
    async postComment() {
      throw new Error("postComment is not faked");
    },
    async editComment() {
      throw new Error("editComment is not faked");
    },
    async deleteComment() {
      throw new Error("deleteComment is not faked");
    },
```

`web/packages/api/src/notes.ts` に足す。import に、生成された `deleteItemComment`・`deleteProjectComment`・`editItemComment`・`editProjectComment`・`listItemComments`・`listProjectComments`・`postItemComment`・`postProjectComment` と `type Comment as ApiComment` を、core の import に `Comment` を足す。

```ts
function toComment(c: ApiComment): Comment {
  return { id: c.id, by: c.by, byName: c.byName, at: c.at, body: c.body, edited: c.edited };
}
```

`createNotesPort` が返すオブジェクトに足す。

```ts
    listComments: (t) =>
      call(async () => {
        const list =
          t.kind === "item"
            ? await listItemComments(t.storeId, t.id)
            : await listProjectComments(t.storeId, t.id);
        return (list ?? []).map(toComment);
      }),
    postComment: (t, body) =>
      call(async () =>
        toComment(
          t.kind === "item"
            ? await postItemComment(t.storeId, t.id, { body })
            : await postProjectComment(t.storeId, t.id, { body }),
        ),
      ),
    editComment: (t, commentId, body) =>
      call(async () =>
        toComment(
          t.kind === "item"
            ? await editItemComment(t.storeId, t.id, commentId, { body })
            : await editProjectComment(t.storeId, t.id, commentId, { body }),
        ),
      ),
    deleteComment: (t, commentId) =>
      call(async () => {
        if (t.kind === "item") await deleteItemComment(t.storeId, t.id, commentId);
        else await deleteProjectComment(t.storeId, t.id, commentId);
      }),
```

`web/packages/core/src/notesIndex.ts` に足す(並びは `pnpm -C web fix` が整える)。

```ts
export type { Comment } from "./model/notes";
```

- [ ] **Step 8: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/api test -- notes && pnpm -C web check`
Expected: PASS

- [ ] **Step 9: コメントの状態の失敗するテストを書く**

`web/packages/core/src/features/notes/comments.test.ts`:

```ts
import { describe, expect, it, vi } from "vitest";
import { NotFound, Refused } from "../../model/errors";
import type { Comment } from "../../model/notes";
import type { NoteTarget } from "../../port/notes";
import { get } from "../../reactive/store";
import { createToasts } from "../toasts/toasts";
import {
  commentAvatar,
  commentDeletedMessage,
  commentDeleteDialog,
  commentPlaceholder,
  createComments,
  isMine,
} from "./comments";

const target: NoteTarget = { kind: "item", storeId: "S1", id: "I1" };
const other: NoteTarget = { kind: "item", storeId: "S1", id: "I2" };
const members = [
  { login: "taro", name: "太郎" },
  { login: "hanako", name: "花子" },
];

function comment(id: string, by: string, body: string, edited = false): Comment {
  return { id, by, byName: by === "ghost" ? "不明なユーザー" : by, at: "2026-11-13T09:00:00.000Z", body, edited };
}

function setup(initial: Comment[] = [comment("K1", "hanako", "前")]) {
  let rows = initial;
  const calls: string[] = [];
  const notes = {
    listComments: vi.fn(async (_t: NoteTarget) => rows),
    postComment: vi.fn(async (_t: NoteTarget, body: string) => {
      calls.push(`post:${body}`);
      const c = comment(`K${rows.length + 1}`, "hanako", body);
      rows = [...rows, c];
      return c;
    }),
    editComment: vi.fn(async (_t: NoteTarget, id: string, body: string) => {
      calls.push(`edit:${id}:${body}`);
      rows = rows.map((r) => (r.id === id ? { ...r, body, edited: true } : r));
      return rows.find((r) => r.id === id) as Comment;
    }),
    deleteComment: vi.fn(async (_t: NoteTarget, id: string) => {
      calls.push(`delete:${id}`);
      rows = rows.filter((r) => r.id !== id);
    }),
  };
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const dirty: Record<string, () => boolean> = {};
  const held: Record<string, () => boolean> = {};
  const confirmLeave = vi.fn(async (_key?: string) => true);
  const drafts = {
    register: (key: string, d: () => boolean, h: () => boolean = d) => {
      dirty[key] = d;
      held[key] = h;
      return () => {};
    },
    confirmLeave,
  };
  const comments = createComments({
    notes,
    catalogPort: { listMembers: async () => members, me: async () => "hanako" },
    toasts,
    drafts,
    scope: "item",
  });
  return { comments, notes, toasts, dirty, held, confirmLeave, calls, setRows: (r: Comment[]) => (rows = r) };
}

describe("createComments", () => {
  it("opens a target with its comments, members and me", async () => {
    const { comments } = setup();
    await comments.open(target);
    const s = get(comments.state);
    expect(s.rows.map((r) => r.id)).toEqual(["K1"]);
    expect(s.members).toEqual(members);
    expect(s.me).toBe("hanako");
  });

  it("keeps the input when the same target is opened again, and clears it for another target", async () => {
    const { comments } = setup();
    await comments.open(target);
    comments.setNew("書きかけ");
    await comments.open(target);
    expect(get(comments.state).newBody).toBe("書きかけ");
    await comments.open(other);
    expect(get(comments.state).newBody).toBe("");
  });

  it("posts the trimmed body, clears the input and reloads", async () => {
    const { comments, calls } = setup();
    await comments.open(target);
    comments.setNew("  @taro 見てください \n");
    expect(await comments.post()).toBe(true);
    expect(calls).toEqual(["post:@taro 見てください"]);
    const s = get(comments.state);
    expect(s.newBody).toBe("");
    expect(s.rows.map((r) => r.body)).toEqual(["前", "@taro 見てください"]);
  });

  it("does nothing when the body is empty", async () => {
    const { comments, notes } = setup();
    await comments.open(target);
    comments.setNew(" \n ");
    expect(await comments.post()).toBe(false);
    expect(notes.postComment).not.toHaveBeenCalled();
  });

  it("keeps the input and shows the refusal in a toast", async () => {
    const { comments, notes, toasts } = setup();
    notes.postComment.mockRejectedValueOnce(new Refused("コメントを入力してください"));
    await comments.open(target);
    comments.setNew("x");
    expect(await comments.post()).toBe(false);
    expect(get(comments.state).newBody).toBe("x");
    expect(get(toasts.list)[0]).toMatchObject({ text: "コメントを入力してください", error: true });
  });

  it("edits: saves a changed body, and closes without saving an empty or unchanged one", async () => {
    const { comments, calls } = setup();
    await comments.open(target);
    comments.startEdit("K1");
    expect(get(comments.state).editing).toEqual({ id: "K1", body: "前" });
    comments.setEditing("前");
    expect(await comments.saveEdit()).toBe(true);
    expect(get(comments.state).editing).toBeNull();
    comments.startEdit("K1");
    comments.setEditing("  ");
    expect(await comments.saveEdit()).toBe(true);
    expect(get(comments.state).editing).toBeNull();
    expect(calls).toEqual([]);
    comments.startEdit("K1");
    comments.setEditing("後");
    expect(await comments.saveEdit()).toBe(true);
    expect(calls).toEqual(["edit:K1:後"]);
    expect(get(comments.state).rows[0]).toMatchObject({ body: "後", edited: true });
    expect(get(comments.state).editing).toBeNull();
  });

  it("asks before deleting, then deletes, shows the toast and reloads", async () => {
    const { comments, calls, toasts } = setup();
    await comments.open(target);
    comments.askDelete("K1");
    expect(get(comments.state).deleting).toBe("K1");
    comments.cancelDelete();
    expect(get(comments.state).deleting).toBeNull();
    comments.askDelete("K1");
    await comments.confirmDelete();
    expect(calls).toEqual(["delete:K1"]);
    expect(get(comments.state).rows).toEqual([]);
    expect(get(comments.state).deleting).toBeNull();
    expect(get(toasts.list)[0]).toMatchObject({ text: commentDeletedMessage, error: false });
  });

  it("reloads quietly when the comment was already deleted elsewhere", async () => {
    const { comments, notes, toasts, setRows } = setup();
    await comments.open(target);
    notes.deleteComment.mockImplementationOnce(async () => {
      setRows([]);
      throw new NotFound("not found");
    });
    comments.askDelete("K1");
    await comments.confirmDelete();
    expect(get(comments.state).rows).toEqual([]);
    expect(get(toasts.list)).toEqual([]);
  });

  it("registers a draft only for a non-blank new comment and a changed edit", async () => {
    const { comments, dirty, held } = setup();
    await comments.open(target);
    expect(dirty["comment:new:item"]?.()).toBe(false);
    comments.setNew("   ");
    expect(dirty["comment:new:item"]?.()).toBe(false);
    comments.setNew("書きかけ");
    expect(dirty["comment:new:item"]?.()).toBe(true);
    expect(dirty["comment:edit:item"]?.()).toBe(false);
    comments.startEdit("K1");
    expect(dirty["comment:edit:item"]?.()).toBe(false);
    comments.setEditing("");
    expect(dirty["comment:edit:item"]?.()).toBe(true);
    // 空にした編集は、保存と違うが、失うものがないので反映は待たない。
    expect(held["comment:edit:item"]?.()).toBe(false);
    comments.setEditing("変えた");
    expect(held["comment:edit:item"]?.()).toBe(true);
  });

  it("asks before discarding a new comment on Escape, only when there is something to lose", async () => {
    const { comments, confirmLeave } = setup();
    await comments.open(target);
    await comments.escapeNew();
    expect(confirmLeave).not.toHaveBeenCalled();
    comments.setNew("書きかけ");
    confirmLeave.mockResolvedValueOnce(false);
    await comments.escapeNew();
    expect(get(comments.state).newBody).toBe("書きかけ");
    await comments.escapeNew();
    expect(confirmLeave).toHaveBeenCalledTimes(2);
    expect(confirmLeave).toHaveBeenCalledWith("comment:new:item");
    expect(get(comments.state).newBody).toBe("");
  });

  it("closes an unchanged edit on Escape without asking, and asks for a changed one", async () => {
    const { comments, confirmLeave } = setup();
    await comments.open(target);
    comments.startEdit("K1");
    await comments.escapeEdit();
    expect(confirmLeave).not.toHaveBeenCalled();
    expect(get(comments.state).editing).toBeNull();
    comments.startEdit("K1");
    comments.setEditing("変えた");
    confirmLeave.mockResolvedValueOnce(false);
    await comments.escapeEdit();
    expect(get(comments.state).editing).not.toBeNull();
    await comments.escapeEdit();
    expect(confirmLeave).toHaveBeenCalledWith("comment:edit:item");
    expect(get(comments.state).editing).toBeNull();
  });

  it("drops the input and the rows when closed", async () => {
    const { comments } = setup();
    await comments.open(target);
    comments.setNew("x");
    comments.close();
    expect(get(comments.state)).toMatchObject({ target: null, rows: [], newBody: "" });
  });

  it("knows my comments and the avatar of an unknown author", () => {
    expect(isMine(comment("K1", "Hanako", "x"), "hanako")).toBe(true);
    expect(isMine(comment("K1", "taro", "x"), "hanako")).toBe(false);
    expect(isMine(comment("K1", "taro", "x"), "")).toBe(false);
    expect(commentAvatar(comment("K1", "taro", "x"))).toBe("taro");
    expect(commentAvatar(comment("K1", "ghost", "x"))).toBe("");
  });

  it("uses the words of the spec", () => {
    expect(commentPlaceholder).toBe("コメントを書く。@ログイン名 でメンション。Ctrl+Enter で送信");
    expect(commentDeleteDialog).toEqual({
      title: "コメントを削除",
      lines: ["このコメントを削除します。"],
      ok: "削除",
    });
  });
});
```

- [ ] **Step 10: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- comments`
Expected: FAIL(`Failed to resolve import "./comments"`)

- [ ] **Step 11: 実装する**

`web/packages/core/src/features/notes/comments.ts`:

```ts
import { NotFound, Refused } from "../../model/errors";
import type { Member } from "../../model/member";
import type { Comment } from "../../model/notes";
import type { CatalogPort } from "../../port/catalog";
import type { NoteTarget, NotesPort } from "../../port/notes";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Drafts } from "../drafts/drafts";
import type { Toasts } from "../toasts/toasts";

// コメントの区画の状態(02「コメント」)。アイテムの詳細パネルとプロジェクトの概要が、それぞれ 1 つずつ持つ(A4a-7・A4a-21)。

export const commentPlaceholder = "コメントを書く。@ログイン名 でメンション。Ctrl+Enter で送信";
export const commentDeletedMessage = "コメントを削除しました";
export const commentDeleteDialog = {
  title: "コメントを削除",
  lines: ["このコメントを削除します。"],
  ok: "削除",
};
const unknownUser = "不明なユーザー";

export interface CommentsState {
  target: NoteTarget | null;
  rows: Comment[];
  members: Member[];
  me: string;
  newBody: string;
  // editing は、編集中のコメントの内部 ID と入力。なければ null。
  editing: { id: string; body: string } | null;
  // deleting は、削除の確認を出しているコメントの内部 ID。なければ null。
  deleting: string | null;
  // busy は、送信中(二重に投稿しない)。
  busy: boolean;
}

export interface Comments {
  state: Readable<CommentsState>;
  // open は、持ち主のコメントを読む。持ち主が変わったら入力を空にする。同じ持ち主なら入力は残す。
  open(target: NoteTarget): Promise<void>;
  // reload は、画面への反映(06)。入力は書き換えない。
  reload(): Promise<void>;
  // close は、区画を閉じたとき。入力と一覧を捨てる。
  close(): void;
  setNew(text: string): void;
  // post は、新しいコメントを投稿する。空なら何もしない(false)。
  post(): Promise<boolean>;
  escapeNew(): Promise<void>;
  startEdit(id: string): void;
  setEditing(text: string): void;
  cancelEdit(): void;
  // saveEdit は、編集を保存する。空にしたときと前と同じときは、保存せずに閉じる(true)。断られたら閉じない(false)。
  saveEdit(): Promise<boolean>;
  escapeEdit(): Promise<void>;
  askDelete(id: string): void;
  cancelDelete(): void;
  confirmDelete(): Promise<void>;
}

export interface CommentsDeps {
  notes: Pick<NotesPort, "listComments" | "postComment" | "editComment" | "deleteComment">;
  catalogPort: Pick<CatalogPort, "listMembers" | "me">;
  toasts: Toasts;
  drafts: Pick<Drafts, "register" | "confirmLeave">;
  // scope は、書きかけのキーを分ける。詳細パネルとプロジェクトの概要が同時に開くため。
  scope: "item" | "project";
}

// isMine は、自分が書いたコメントか(編集と削除のボタンを出す。ログイン名は大文字と小文字を区別しない)。
export const isMine = (c: Comment, me: string): boolean =>
  me !== "" && c.by.toLowerCase() === me.toLowerCase();

// commentAvatar は、アバターに出す名前。メンバーにいない人は中身のない丸(01「不明なユーザー」)。
export const commentAvatar = (c: Comment): string => (c.byName === unknownUser ? "" : c.byName);

const sameTarget = (a: NoteTarget | null, b: NoteTarget) =>
  a !== null && a.kind === b.kind && a.storeId === b.storeId && a.id === b.id;

const initial = (): CommentsState => ({
  target: null,
  rows: [],
  members: [],
  me: "",
  newBody: "",
  editing: null,
  deleting: null,
  busy: false,
});

export function createComments(deps: CommentsDeps): Comments {
  const state = writable<CommentsState>(initial());
  const patch = (p: Partial<CommentsState>) => state.update((s) => ({ ...s, ...p }));
  let latest = 0;

  async function load(target: NoteTarget) {
    latest += 1;
    const mine = latest;
    try {
      const [rows, members, me] = await Promise.all([
        deps.notes.listComments(target),
        deps.catalogPort.listMembers(target.storeId).catch((): Member[] => []),
        // me が決まらないときは空(A2 の決定 3)。
        deps.catalogPort.me(target.storeId).catch(() => ""),
      ]);
      if (mine === latest) patch({ rows, members, me });
    } catch (e) {
      // 持ち主がなくなっていたら、詳細パネルが閉じるので何もしない。
      if (!(e instanceof NotFound)) throw e;
    }
  }

  // refusedToast は、断りをエラーのトーストで出す。断り以外は投げる。
  function refusedToast(e: unknown) {
    if (!(e instanceof Refused)) throw e;
    deps.toasts.show(e.message, { error: true });
  }

  const original = (id: string) => get(state).rows.find((r) => r.id === id)?.body ?? "";

  function editDirty(): boolean {
    const { editing } = get(state);
    return editing !== null && editing.body !== original(editing.id);
  }

  // キーは comment: で始める(確認の本文が決まる。Task 6)。Esc はその入力欄のぶんだけ確認する。
  const newKey = `comment:new:${deps.scope}`;
  const editKey = `comment:edit:${deps.scope}`;
  deps.drafts.register(newKey, () => get(state).newBody.trim() !== "");
  // 反映を待つのは、失うものがあるとき(空にした編集は、違うが、失うものがない)。
  deps.drafts.register(editKey, editDirty, () => editDirty() && (get(state).editing?.body.trim() ?? "") !== "");

  async function post(): Promise<boolean> {
    const { target, newBody, busy } = get(state);
    const body = newBody.trim();
    if (!target || body === "" || busy) return false;
    patch({ busy: true });
    try {
      await deps.notes.postComment(target, body);
    } catch (e) {
      refusedToast(e);
      return false;
    } finally {
      patch({ busy: false });
    }
    if (sameTarget(get(state).target, target)) patch({ newBody: "" });
    await load(target);
    return true;
  }

  async function saveEdit(): Promise<boolean> {
    const { target, editing, busy } = get(state);
    if (!target || !editing || busy) return false;
    const body = editing.body.trim();
    if (body === "" || body === original(editing.id)) {
      patch({ editing: null });
      return true;
    }
    patch({ busy: true });
    try {
      await deps.notes.editComment(target, editing.id, body);
    } catch (e) {
      refusedToast(e);
      return false;
    } finally {
      patch({ busy: false });
    }
    patch({ editing: null });
    await load(target);
    return true;
  }

  async function confirmDelete() {
    const { target, deleting } = get(state);
    patch({ deleting: null });
    if (!target || !deleting) return;
    try {
      await deps.notes.deleteComment(target, deleting);
    } catch (e) {
      // ほかで消えていたら、読み直すだけにする。
      if (!(e instanceof NotFound)) {
        refusedToast(e);
        return;
      }
      await load(target);
      return;
    }
    deps.toasts.show(commentDeletedMessage);
    await load(target);
  }

  return {
    state: readonly(state),
    open(target) {
      if (!sameTarget(get(state).target, target)) state.set({ ...initial(), target });
      return load(target);
    },
    async reload() {
      const { target } = get(state);
      if (target) await load(target);
    },
    close() {
      latest += 1;
      state.set(initial());
    },
    setNew: (text) => patch({ newBody: text }),
    post,
    async escapeNew() {
      // 空なら何も失わないので、確認せず、Esc はパネルを閉じる側に任せる(呼ぶ側が空のときは呼ばない)。
      if (get(state).newBody.trim() === "") return;
      if (await deps.drafts.confirmLeave(newKey)) patch({ newBody: "" });
    },
    startEdit(id) {
      patch({ editing: { id, body: original(id) } });
    },
    setEditing: (text) => state.update((s) => (s.editing ? { ...s, editing: { ...s.editing, body: text } } : s)),
    cancelEdit: () => patch({ editing: null }),
    saveEdit,
    async escapeEdit() {
      if (!editDirty() || (await deps.drafts.confirmLeave(editKey))) patch({ editing: null });
    },
    askDelete: (id) => patch({ deleting: id }),
    cancelDelete: () => patch({ deleting: null }),
    confirmDelete,
  };
}
```

`web/packages/core/src/notesIndex.ts` に足す(`index.ts` は変えない)。

```ts
export {
  type Comments,
  type CommentsDeps,
  type CommentsState,
  commentAvatar,
  commentDeleteDialog,
  commentDeletedMessage,
  commentPlaceholder,
  createComments,
  isMine,
} from "./features/notes/comments";
export { appendMention, mentionTargets } from "./features/notes/mentions";
```

- [ ] **Step 12: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- notes && pnpm -C web check`
Expected: PASS

- [ ] **Step 13: 画面を作る**

`web/app/src/screens/notes/CommentItem.svelte`(コメント 1 件。表示と編集と削除のボタン):

```svelte
<script lang="ts">
  import {
    type Comment,
    type Comments,
    type CommentsState,
    commentAvatar,
    isMemberOf,
    isMine,
    readOnlyNote,
    relativeTime,
    tokenize,
  } from "@hedgehug/core";
  import { Avatar, Button, Markdown, Textarea } from "@hedgehug/ui";
  import { syncNow } from "../../wire";

  // コメント 1 件(02「コメント」)。自分が書いたコメントにだけ、編集と削除のボタンを出す。
  interface Props {
    c: Comment;
    comments: Comments;
    s: CommentsState;
    readOnly: boolean;
  }

  let { c, comments, s, readOnly }: Props = $props();

  const note = $derived(readOnly ? readOnlyNote : undefined);
  const editing = $derived(s.editing?.id === c.id ? s.editing : null);
  const blocks = $derived(tokenize(c.body, isMemberOf(s.members)));

  function onkeydown(event: KeyboardEvent) {
    if (event.isComposing) return;
    if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      void comments.saveEdit();
    } else if (event.key === "Escape") {
      // 変えていなければそのまま閉じ、変えていれば確認する。パネルは閉じない。
      event.stopPropagation();
      void comments.escapeEdit();
    }
  }
</script>

<article class="comment">
  <header class="meta">
    <Avatar name={commentAvatar(c)} title={c.byName} />
    <span class="who">{c.byName}</span>
    <time class="when" datetime={c.at}>{relativeTime(new Date(c.at), $syncNow)}</time>
    {#if c.edited}<span class="when">(編集済み)</span>{/if}
    {#if isMine(c, s.me) && !editing}
      <span class="grow"></span>
      <Button size="xs" disabled={readOnly} title={note} onclick={() => comments.startEdit(c.id)}>編集</Button>
      <Button size="xs" disabled={readOnly} title={note} onclick={() => comments.askDelete(c.id)}>削除</Button>
    {/if}
  </header>
  {#if editing}
    <Textarea
      label="コメントを編集"
      focusOnMount
      {onkeydown}
      bind:value={() => editing.body, (v) => comments.setEditing(v)}
    />
    <div class="actions">
      <Button size="sm" onclick={comments.cancelEdit}>キャンセル</Button>
      <Button size="sm" variant="primary" disabled={s.busy} onclick={() => void comments.saveEdit()}>保存</Button>
    </div>
  {:else}
    <Markdown {blocks} />
  {/if}
</article>

<style>
  .comment {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 8px 0;
    border-bottom: 1px solid var(--line);
  }
  .meta {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12.5px;
  }
  .who {
    font-weight: 600;
  }
  .when {
    color: var(--muted);
  }
  .grow {
    flex: 1;
  }
  .actions {
    display: flex;
    gap: 6px;
    justify-content: flex-end;
  }
</style>
```

`web/app/src/screens/notes/CommentsSection.svelte`:

```svelte
<script lang="ts">
  import {
    appendMention,
    commentDeleteDialog,
    commentPlaceholder,
    mentionTargets,
    type NoteTarget,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Button, ConfirmDialog, Textarea } from "@hedgehug/ui";
  import { onDestroy, untrack } from "svelte";
  import { itemComments, projectComments } from "../../wire";
  import CommentItem from "./CommentItem.svelte";

  // コメントの区画(02「コメント」「メンション」)。アイテムの詳細パネルとプロジェクトの概要の両方が使う。
  // target.kind は、この区画を作ったあとで変えない(詳細パネルは item、概要は project)。
  interface Props {
    target: NoteTarget;
    readonly: boolean;
  }

  let { target, readonly: readOnly }: Props = $props();

  const comments = untrack(() => (target.kind === "item" ? itemComments : projectComments));
  const store = comments.state;
  const note = $derived(readOnly ? readOnlyNote : undefined);

  $effect(() => {
    void comments.open(target);
  });
  onDestroy(() => comments.close());

  function onkeydown(event: KeyboardEvent) {
    if (event.isComposing) return;
    if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      void comments.post();
    } else if (event.key === "Escape" && $store.newBody.trim() !== "") {
      // 書きかけがあるときだけ確認する。ないときの Esc はパネルを閉じる。
      event.stopPropagation();
      void comments.escapeNew();
    }
  }
</script>

<section class="peek-sec" aria-label="コメント">
  <h3>コメント</h3>
  {#each $store.rows as c (c.id)}
    <CommentItem {c} {comments} s={$store} {readOnly} />
  {/each}
  <Textarea
    label="コメント"
    placeholder={commentPlaceholder}
    disabled={readOnly}
    title={note}
    {onkeydown}
    bind:value={() => $store.newBody, (v) => comments.setNew(v)}
  />
  <div class="row">
    {#each mentionTargets($store.members, $store.me) as login (login)}
      <Button
        size="xs"
        disabled={readOnly}
        title={note}
        onclick={() => comments.setNew(appendMention($store.newBody, login))}
        >@{login}</Button
      >
    {/each}
    <span class="grow"></span>
    <Button variant="primary" size="sm" disabled={readOnly || $store.busy} title={note} onclick={() => void comments.post()}>投稿</Button>
  </div>
  {#if $store.deleting}
    <ConfirmDialog
      title={commentDeleteDialog.title}
      lines={commentDeleteDialog.lines}
      ok={commentDeleteDialog.ok}
      danger
      onok={() => void comments.confirmDelete()}
      oncancel={comments.cancelDelete}
    />
  {/if}
</section>

<style>
  .peek-sec {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  h3 {
    font-size: 13px;
    margin: 0;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }
  .grow {
    flex: 1;
  }
</style>
```

`web/app/src/wire.ts`: `createComments` を `@hedgehug/core` の import に足す(並びは `pnpm -C web fix`)。`notes`(Task 7 が作る `NotesPort`)と `drafts`(Task 6 が `./drafts` から import 済み)を使い、`detail` の定義の近くに足す。

```ts
// アイテムの詳細パネルとプロジェクトの概要は、同時に開くので、コメントの状態を別々に持つ。
export const itemComments = createComments({ notes, catalogPort, toasts, drafts, scope: "item" });
export const projectComments = createComments({ notes, catalogPort, toasts, drafts, scope: "project" });
```

画面への反映(`reflect`)で、コメントも読み直す。入力は書き換えない(`reload` は入力に触れない。書きかけがあるあいだは反映自体が待つ)。

```ts
  if (kinds.includes("store")) {
    jobs.push(
      catalog
        .load()
        .then(() =>
          Promise.all([projectScreen.reload(), detail.reload(), itemComments.reload(), projectComments.reload()]),
        ),
    );
  }
```

`web/app/src/screens/item/DetailPanel.svelte`: import を足し、GitHub の区画(と、Task 12 の作業時間の区画があればその次)の直後に足す。

```svelte
  import CommentsSection from "../notes/CommentsSection.svelte";
```

```svelte
    <CommentsSection target={{ kind: "item", storeId: v.store.id, id: v.item.id }} readonly={v.readOnly} />
```

- [ ] **Step 14: 確かめる**

Run: `pnpm -C web fix && pnpm -C web check && pnpm -C web test && pnpm -C web lint && pnpm -C web deps && pnpm -C web size`
Expected: すべて PASS(`check` は `svelte-check --fail-on-warnings`。`Markdown`・`tokenize`・`isMemberOf`(Task 5)、`Drafts`・`drafts`(Task 6)、`Textarea`・`notes`(Task 7)が入っていることが前提。`CommentItem.svelte`・`CommentsSection.svelte` は 200 行以下)

画面の動き(投稿・編集・削除・メンションのボタン・Esc の確認・閲覧のみの Store)は、Task 22 の e2e で通しで確かめる。ここで手で確かめるなら `make e2e-bin` のバイナリを動かし、詳細パネルでコメントを投稿して、リロードしても残ることを見る。

- [ ] **Step 15: コミット**

```bash
git add web
git commit -m "feat(web): 詳細パネルにコメントの区画とメンションのボタンを足す(A4a-7・A4a-21〜23)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 11: 作業時間の行の決め方・検査と操作・API

**Files:**
- Modify: `internal/domain/change/record.go`(`Worklog` の種類)
- Modify: `internal/domain/change/draft.go`(`NewWorklog`)
- Test: `internal/domain/change/worklog_draft_test.go`
- Create: `internal/domain/worklog/hours.go`・`worklog.go`
- Test: `internal/domain/worklog/hours_test.go`・`worklog_test.go`
- Create: `internal/app/worklogs/service.go`・`list.go`・`add.go`・`delete.go`
- Test: `internal/app/worklogs/worklogs_test.go`
- Create: `internal/adapter/httpapi/notes/worklogs.go`
- Modify: `internal/adapter/httpapi/notes/notes.go`(`Services` に `Worklogs appworklogs.Service` を足し、`Register` に `registerWorklogs(api, s.Worklogs)` を 1 行足す)
- Test: `internal/adapter/httpapi/notes/worklogs_test.go`
- Modify: `internal/adapter/httpapi/routes.go`(`notes.Register` の `Services` に `Worklogs` を足す)
- 生成物: `api/openapi.yaml`・`web/packages/api/src/generated.ts`

**Interfaces:**
- Consumes: Task 3 の `notes.Services`・`notes.Register`、Task 9 の `change.Absent`・`change.Current`・`change.First`(このタスクは Task 9 のあとに行う)、`port.Clock`(`Deps.Clock`)、`refusal.New`
- Produces:
  - `change.Worklog Type = "worklog"`、`func NewWorklog(id ID, from, to any) Draft`(`from` が nil なら追加、`to` が nil なら削除)
  - `worklog.Entry{ID change.ID; By string; Date string; Hundredths int; Memo string}`、`worklog.FromLog(log []change.Record) []worklog.Entry`(記録の古い順)、`worklog.Find(log []change.Record, id change.ID) (worklog.Entry, bool)`、`worklog.Value(hundredths int, memo, date string) any`
  - `worklog.ParseHours(s string) (int, error)`(断りは `refusal.New`)、`worklog.Format(hundredths int) string`(`"1.25"`・`"0.3"`・`"2"`。「 時間」は付けない)
  - `worklogs.Service{Stores port.Stores; IDs port.IDs; Identity port.Identity; Clock port.Clock}`、`worklogs.View{worklog.Entry; ByName string}`
  - `func (s Service) List(ctx context.Context, store port.StoreID, id item.ID) ([]View, error)`、`Add(ctx, store, id item.ID, hours, memo string) (View, error)`、`Delete(ctx, store, id item.ID, worklog change.ID) error`
  - `notes.Services.Worklogs`(`appworklogs.Service`)と、`Register` が呼ぶ `registerWorklogs(api huma.API, svc appworklogs.Service)`、API 本文の型 `notes.Worklog`・`notes.WorklogInput`

**行の形(A4a-8・9):** `t: "worklog"`、`f` は記録の内部 ID、`to` は `{"hours": 数, "memo": 文字列, "date": "YYYY-MM-DD"}`、削除は `to: null`。記録した人は最初の行の `by`。日付は保存したマシンの今日(`Clock`)。時間は 100 分の 1 時間の整数で持ち、行には JSON の数(`float64(h)/100`)で書き、読むときに `int(math.Round(x*100))` で戻す。

- [ ] **Step 1: 失敗するテストを書く(`change`)**

`internal/domain/change/worklog_draft_test.go`:

```go
package change_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
)

func TestNewWorklogWritesNullForAddAndDelete(t *testing.T) {
	add := change.NewWorklog("W1", nil, map[string]any{"hours": 1.25})
	del := change.NewWorklog("W1", map[string]any{"hours": 1.25}, nil)
	if add.T != change.Worklog || add.F != "W1" || string(add.From) != "null" || string(add.To) != `{"hours":1.25}` {
		t.Errorf("add = %+v", add)
	}
	if !change.Absent(del.To) || change.Absent(add.To) {
		t.Errorf("delete = %+v", del)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/change/ -run NewWorklog`
Expected: FAIL(ビルドエラー `undefined: change.Worklog`・`change.NewWorklog`)

- [ ] **Step 3: 実装する**

`record.go` の `Type` の定数に足す(Task 9 の `Comment` の下)。

```go
	// Worklog は作業時間の追加・削除。f は記録の内部 ID、to は {"hours": 数, "memo": 文字列, "date": "YYYY-MM-DD"}、削除は to が null(A4a-8)。
	Worklog Type = "worklog"
```

`draft.go` の末尾に足す。

```go
// NewWorklog は、作業時間の追加・削除の変更。f は記録の内部 ID。
// from が nil なら追加、to が nil なら削除(どちらも null で書く)。行に編集はない(02「作業時間」)。
func NewWorklog(id ID, from, to any) Draft {
	return Draft{T: Worklog, F: string(id), From: Value(from), To: Value(to)}
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/change/`
Expected: PASS

- [ ] **Step 5: 失敗するテストを書く(`worklog` の検査と表記)**

`internal/domain/worklog/hours_test.go`。同じ表を TypeScript の `hours.test.ts`(Task 12)にも写す。

```go
package worklog_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

const (
	msgEmpty = "時間を入力してください"
	msgScale = "時間は小数第 2 位までで入力してください"
)

func message(err error) string {
	if r, ok := err.(*refusal.Refusal); ok {
		return r.Message
	}
	return ""
}

func TestParseHours(t *testing.T) {
	ok := map[string]int{
		"1": 100, "1.5": 150, "0.25": 25, "0.1": 10, ".5": 50, "1.": 100, "1.250": 125, "0.07": 7,
		" 2 ": 200, "12": 1200, "00.5": 50, "999999999": 99999999900,
	}
	for in, want := range ok {
		if got, err := worklog.ParseHours(in); err != nil || got != want {
			t.Errorf("ParseHours(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", " ", "0", "0.0", "0.000", "-1", "-0.5", "abc", "1e2", "1,5", ".", "１", "1000000000"} {
		if _, err := worklog.ParseHours(in); message(err) != msgEmpty {
			t.Errorf("ParseHours(%q) = %v; want %q", in, err, msgEmpty)
		}
	}
	for _, in := range []string{"0.123", "1.005", "0.001", "2.999"} {
		if _, err := worklog.ParseHours(in); message(err) != msgScale {
			t.Errorf("ParseHours(%q) = %v; want %q", in, err, msgScale)
		}
	}
}

func TestFormat(t *testing.T) {
	want := map[int]string{0: "0", 7: "0.07", 10: "0.1", 25: "0.25", 30: "0.3", 100: "1", 110: "1.1", 125: "1.25", 200: "2", 1234: "12.34"}
	for h, s := range want {
		if got := worklog.Format(h); got != s {
			t.Errorf("Format(%d) = %q, want %q", h, got, s)
		}
	}
}
```

`internal/domain/worklog/worklog_test.go`:

```go
package worklog_test

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

func at(minute int) time.Time { return time.Date(2026, 11, 13, 10, minute, 0, 0, time.UTC) }

func row(id, f string, minute int, by string, from, to any, prev ...change.ID) change.Record {
	d := change.NewWorklog(change.ID(f), from, to)
	return change.Record{ID: change.ID(id), Op: change.ID(id), At: at(minute), By: by, Actor: change.User(by), T: d.T, F: d.F, From: d.From, To: d.To, Prev: prev}
}

func TestFromLogOrdersByRecordAndLeavesOutDeleted(t *testing.T) {
	log := []change.Record{
		row("w1", "W1", 1, "hanako", nil, worklog.Value(125, "下書き", "2026-11-13"), "c"),
		row("w2", "W2", 2, "taro", nil, worklog.Value(30, "", "2026-11-13"), "c"),
		row("w3", "W3", 3, "taro", nil, worklog.Value(100, "消す", "2026-11-13"), "c"),
		row("w3d", "W3", 4, "hanako", worklog.Value(100, "消す", "2026-11-13"), nil, "w3"),
	}
	want := []worklog.Entry{
		{ID: "W1", By: "hanako", Date: "2026-11-13", Hundredths: 125, Memo: "下書き"},
		{ID: "W2", By: "taro", Date: "2026-11-13", Hundredths: 30},
	}
	got := worklog.FromLog(log)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FromLog = %+v\nwant %+v", got, want)
	}
	rev := slices.Clone(log)
	slices.Reverse(rev)
	if !reflect.DeepEqual(worklog.FromLog(rev), want) {
		t.Error("FromLog depends on the order of the rows")
	}
	if e, ok := worklog.Find(log, "W2"); !ok || e.Hundredths != 30 {
		t.Errorf("Find W2 = %+v, %v", e, ok)
	}
	if _, ok := worklog.Find(log, "W3"); ok {
		t.Error("Find of a deleted record")
	}
}

// 時間は JSON の数で書くので、100 分の 1 時間の整数に戻すとき丸める。0.29・0.07・1.1 のような 2 進で表せない値も戻る。
func TestHoursRoundTripThroughTheLog(t *testing.T) {
	for h := 1; h <= 20000; h++ {
		log := []change.Record{row("w", "W1", 1, "taro", nil, worklog.Value(h, "", "2026-11-13"), "c")}
		got := worklog.FromLog(log)
		if len(got) != 1 || got[0].Hundredths != h {
			t.Fatalf("%d hundredths came back as %+v", h, got)
		}
	}
	for _, h := range []int{29, 7, 110, 115, 99999999900} {
		log := []change.Record{row("w", "W1", 1, "taro", nil, worklog.Value(h, "", "2026-11-13"), "c")}
		if got := worklog.FromLog(log); len(got) != 1 || got[0].Hundredths != h {
			t.Errorf("%d hundredths came back as %+v", h, got)
		}
	}
}

func TestFromLogSkipsUnreadableRows(t *testing.T) {
	log := []change.Record{
		row("w1", "W1", 1, "taro", nil, "形が違う", "c"),
		row("w2", "W2", 2, "taro", nil, map[string]any{"hours": 0, "memo": "", "date": "2026-11-13"}, "c"),
	}
	if got := worklog.FromLog(log); len(got) != 0 {
		t.Errorf("FromLog = %+v, want none", got)
	}
}
```

- [ ] **Step 6: 失敗を確かめる**

Run: `go test ./internal/domain/worklog/`
Expected: FAIL(`no non-test Go files in …/internal/domain/worklog`)

- [ ] **Step 7: 実装する**

`internal/domain/worklog/hours.go`:

```go
// Package worklog は、作業時間の行の決め方と、時間の検査・表記を持つ(02「作業時間」)。
package worklog

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const (
	msgEmpty = "時間を入力してください"
	msgScale = "時間は小数第 2 位までで入力してください"
)

// decimal は、10 進の書き方だけを受ける(符号・指数・カンマは数として扱わない)。
var decimal = regexp.MustCompile(`^(\d*)(?:\.(\d*))?$`)

// maxWholeDigits は、整数部の桁数の上限。100 分の 1 時間にして 2^53 を超えないようにする(仕様に上限はない。
// 実際に届く値ではない)。
const maxWholeDigits = 9

// ParseHours は、時間の入力を 100 分の 1 時間の整数にする(A4a-9)。空・数でない・0 以下は「時間を入力してください」、
// 小数第 3 位以下に 0 でない数字があれば「時間は小数第 2 位までで入力してください」(末尾の 0 は数えない。1.250 は 1.25)。
// 文字列のまま見るので、浮動小数点の丸めに左右されない(A4a-26)。
func ParseHours(s string) (int, error) {
	m := decimal.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil || (m[1] == "" && m[2] == "") {
		return 0, refusal.New(msgEmpty)
	}
	whole := strings.TrimLeft(m[1], "0")
	frac := strings.TrimRight(m[2], "0")
	if len(frac) > 2 {
		return 0, refusal.New(msgScale)
	}
	if len(whole) > maxWholeDigits {
		return 0, refusal.New(msgEmpty)
	}
	w, _ := strconv.Atoi("0" + whole)
	f, _ := strconv.Atoi((frac + "00")[:2])
	if h := w*100 + f; h > 0 {
		return h, nil
	}
	return 0, refusal.New(msgEmpty)
}

// Format は、100 分の 1 時間の整数を、小数第 2 位まで・末尾の 0 を省いた表記にする(0.3・1.25・2)。
// 「 時間」は付けない。
func Format(hundredths int) string {
	whole, frac := hundredths/100, hundredths%100
	if frac == 0 {
		return strconv.Itoa(whole)
	}
	return fmt.Sprintf("%d.%s", whole, strings.TrimRight(fmt.Sprintf("%02d", frac), "0"))
}
```

`internal/domain/worklog/worklog.go`:

```go
package worklog

import (
	"cmp"
	"encoding/json"
	"math"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Entry は、いまの作業時間の記録 1 件。By は記録した人(最初の行の by)。Date は記録した日(YYYY-MM-DD)。
type Entry struct {
	ID         change.ID
	By         string
	Date       string
	Hundredths int
	Memo       string
}

type value struct {
	Hours float64 `json:"hours"`
	Memo  string  `json:"memo"`
	Date  string  `json:"date"`
}

// Value は、作業時間の値(行の from・to に書く形)。時間は JSON の数で書く。
func Value(hundredths int, memo, date string) any {
	return value{Hours: float64(hundredths) / 100, Memo: memo, Date: date}
}

func alive(r change.Record) bool { return !change.Absent(r.To) }

func read(raw json.RawMessage) (value, bool) {
	var v value
	if json.Unmarshal(raw, &v) != nil || math.Round(v.Hours*100) <= 0 {
		return value{}, false
	}
	return v, true
}

// FromLog は、ログのいまの作業時間を、記録の古い順(同じなら ID の順)で返す。削除されたものと、
// 読めないものは除く。いまの値は change.Current が決める。
func FromLog(log []change.Record) []Entry {
	ids := map[change.ID]bool{}
	for _, r := range log {
		if r.T == change.Worklog {
			ids[change.ID(r.F)] = true
		}
	}
	type dated struct {
		Entry
		recorded change.Record
	}
	var found []dated
	for id := range ids {
		if e, first, ok := find(log, id); ok {
			found = append(found, dated{Entry: e, recorded: first})
		}
	}
	slices.SortFunc(found, func(a, b dated) int {
		return cmp.Or(a.recorded.At.Compare(b.recorded.At), strings.Compare(string(a.ID), string(b.ID)))
	})
	out := make([]Entry, 0, len(found))
	for _, d := range found {
		out = append(out, d.Entry)
	}
	return out
}

// Find は、いまの記録を 1 件返す。ないか、削除されているか、読めなければ false。
func Find(log []change.Record, id change.ID) (Entry, bool) {
	e, _, ok := find(log, id)
	return e, ok
}

func find(log []change.Record, id change.ID) (Entry, change.Record, bool) {
	f := string(id)
	cur, ok := change.Current(log, change.Worklog, f, alive)
	if !ok || change.Absent(cur.To) {
		return Entry{}, change.Record{}, false
	}
	v, ok := read(cur.To)
	if !ok {
		return Entry{}, change.Record{}, false
	}
	first, _ := change.First(log, change.Worklog, f)
	return Entry{ID: id, By: first.By, Date: v.Date, Hundredths: int(math.Round(v.Hours * 100)), Memo: v.Memo}, first, true
}
```

- [ ] **Step 8: 通ることを確かめて、コミットする**

Run: `go test ./internal/domain/worklog/ ./internal/domain/change/`
Expected: PASS

```bash
git add internal/domain/change internal/domain/worklog
git commit -m "feat(domain): 作業時間の行の決め方と、時間の検査・表記を足す(A4a-8・A4a-9)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 9: 操作の失敗するテストを書く**

`internal/app/worklogs/worklogs_test.go`:

```go
package worklogs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/worklogs"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const store port.StoreID = "S1"

// recorder は、Write に渡したアクティビティの 1 行を覚える。
type recorder struct {
	*apptest.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func setup(t *testing.T) (worklogs.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	if err := apptest.Sample(t.Context(), s, store); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return worklogs.Service{
		Stores: rec, IDs: ids, Identity: apptest.Identity{Login: "hanako"},
		Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)),
	}, rec
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func logLen(t *testing.T, s port.Stores) int {
	t.Helper()
	n := 0
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		n = len(log)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAddThenList(t *testing.T) {
	svc, rec := setup(t)
	v, err := svc.Add(t.Context(), store, apptest.SampleTask, " 1.25 ", " 下書き ")
	if err != nil {
		t.Fatal(err)
	}
	if v.Hundredths != 125 || v.Memo != "下書き" || v.Date != "2026-11-13" || v.By != "hanako" || v.ByName != "hanako" || v.ID == "" {
		t.Errorf("added %+v", v)
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "HH-2 に 1.25 時間を記録" {
		t.Errorf("summary = %q", got)
	}
	if _, err := svc.Add(t.Context(), store, apptest.SampleTask, ".5", ""); err != nil {
		t.Fatal(err)
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "HH-2 に 0.5 時間を記録" {
		t.Errorf("summary = %q", got)
	}
	list, err := svc.List(t.Context(), store, apptest.SampleTask)
	if err != nil || len(list) != 2 || list[0].Hundredths != 125 || list[1].Hundredths != 50 {
		t.Errorf("List = %+v, %v", list, err)
	}
}

func TestAddRefusesBadHoursAndSavesNothing(t *testing.T) {
	svc, rec := setup(t)
	before := logLen(t, rec)
	cases := map[string]string{
		"": "時間を入力してください", "0": "時間を入力してください", "-1": "時間を入力してください", "abc": "時間を入力してください",
		"0.123": "時間は小数第 2 位までで入力してください",
	}
	for in, want := range cases {
		if _, err := svc.Add(t.Context(), store, apptest.SampleTask, in, "x"); message(err) != want {
			t.Errorf("Add(%q) = %v, want %q", in, err, want)
		}
	}
	if logLen(t, rec) != before {
		t.Error("a refused record wrote a row")
	}
}

func TestAnyoneWhoCanWriteCanDeleteAnyRecord(t *testing.T) {
	svc, rec := setup(t)
	v, _ := svc.Add(t.Context(), store, apptest.SampleTask, "2", "")
	other := svc
	other.Identity = apptest.Identity{Login: "taro"}
	if err := other.Delete(t.Context(), store, apptest.SampleTask, v.ID); err != nil {
		t.Fatal(err)
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "HH-2 の作業時間を削除" {
		t.Errorf("summary = %q", got)
	}
	if list, _ := svc.List(t.Context(), store, apptest.SampleTask); len(list) != 0 {
		t.Errorf("List after delete = %+v", list)
	}
	if err := svc.Delete(t.Context(), store, apptest.SampleTask, v.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("deleting twice: %v", err)
	}
}

func TestAuthorMissingFromMembersIsAnUnknownUser(t *testing.T) {
	svc, _ := setup(t)
	svc.Identity = apptest.Identity{Login: "ghost"}
	v, err := svc.Add(t.Context(), store, apptest.SampleTask, "1", "")
	if err != nil || v.By != "ghost" || v.ByName != "不明なユーザー" {
		t.Errorf("added %+v, %v", v, err)
	}
}

func TestUnknownItem(t *testing.T) {
	svc, _ := setup(t)
	gone := change.ID("01NOPE00000000000000000000")
	if _, err := svc.List(t.Context(), store, gone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("List: %v", err)
	}
	if _, err := svc.Add(t.Context(), store, gone, "1", ""); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Add: %v", err)
	}
	if err := svc.Delete(t.Context(), store, apptest.SampleTask, gone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Delete of an unknown record: %v", err)
	}
}
```

- [ ] **Step 10: 失敗を確かめる**

Run: `go test ./internal/app/worklogs/`
Expected: FAIL(`no non-test Go files in …/internal/app/worklogs`)

- [ ] **Step 11: 実装する**

`internal/app/worklogs/service.go`:

```go
// Package worklogs は、アイテムの作業時間の操作(02「作業時間」)を持つ。1 ファイル 1 操作。
package worklogs

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

const unknownUser = "不明なユーザー" // 01「画面の移り方と、共通の表示」

// Service は、作業時間の操作が使う口。Clock は、記録の日付(保存したマシンの今日)を決める。
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
	Clock    port.Clock
}

// View は、画面に出す作業時間の記録。ByName は記録した人の表示名で、メンバーにいなければ「不明なユーザー」。
type View struct {
	worklog.Entry
	ByName string
}

// display は、アイテムの画面の ID(HH-12)。アイテムがなければ port.ErrNotFound。
func display(r port.Reader, id item.ID) (string, error) {
	it, err := r.Items().Get(id)
	if err != nil {
		return "", err
	}
	w, err := r.Workspaces().Get(it.Place.Workspace)
	if err != nil {
		return "", err
	}
	return it.DisplayID(w.Prefix), nil
}

// displayOf は、アクティビティの 1 行に書くアイテムの ID。保存の前に読む。
func (s Service) displayOf(ctx context.Context, store port.StoreID, id item.ID) (string, error) {
	var shown string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		shown, err = display(r, id)
		return err
	})
	return shown, err
}

// names は、ログイン名から表示名を引く関数(大文字と小文字を区別しない。メンバーにいなければ「不明なユーザー」)。
func names(r port.Reader) (func(login string) string, error) {
	ms, err := r.Members().List()
	if err != nil {
		return nil, err
	}
	byLogin := map[string]string{}
	for _, m := range ms {
		byLogin[strings.ToLower(m.Login)] = m.Name
	}
	return func(login string) string {
		if n, ok := byLogin[strings.ToLower(login)]; ok {
			return n
		}
		return unknownUser
	}, nil
}

// find は、書いたあとの記録を読み直して返す。
func (s Service) find(ctx context.Context, store port.StoreID, id item.ID, w change.ID) (View, error) {
	list, err := s.List(ctx, store, id)
	if err != nil {
		return View{}, err
	}
	for _, v := range list {
		if v.ID == w {
			return v, nil
		}
	}
	return View{}, port.ErrNotFound
}
```

`list.go`:

```go
package worklogs

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

// List は、アイテムの作業時間の記録を古い順にすべて返す(02「作業時間」)。アイテムがなければ port.ErrNotFound。
func (s Service) List(ctx context.Context, store port.StoreID, id item.ID) ([]View, error) {
	var out []View
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := display(r, id); err != nil {
			return err
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		if err != nil {
			return err
		}
		nameOf, err := names(r)
		if err != nil {
			return err
		}
		for _, e := range worklog.FromLog(log) {
			out = append(out, View{Entry: e, ByName: nameOf(e.By)})
		}
		return nil
	})
	return out, err
}
```

`add.go`:

```go
package worklogs

import (
	"context"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

// Add は、作業時間を記録する(02「作業時間」)。hours は worklog.ParseHours で検査し、断りはその文言。
// 日付は保存したマシンの今日、記録した人は「あなた」。メモは前後の空白を除く。
// アクティビティは「{ID} に N 時間を記録」。記録を返す。
func (s Service) Add(ctx context.Context, store port.StoreID, id item.ID, hours, memo string) (View, error) {
	h, err := worklog.ParseHours(hours)
	if err != nil {
		return View{}, err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return View{}, err
	}
	shown, err := s.displayOf(ctx, store, id)
	if err != nil {
		return View{}, err
	}
	w := s.IDs.New()
	date := s.Clock.Now().Format("2006-01-02")
	op := port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("%s に %s 時間を記録", shown, worklog.Format(h))}
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		if _, err := display(tx, id); err != nil {
			return err
		}
		return tx.WriteItems().Append(id, change.NewWorklog(w, nil, worklog.Value(h, strings.TrimSpace(memo), date)))
	})
	if err != nil {
		return View{}, err
	}
	return s.find(ctx, store, id, w)
}
```

`delete.go`:

```go
package worklogs

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

// Delete は、作業時間の記録を削除する(to を null にした行を足す)。誰の記録でも、書き込める人なら消せる(02「作業時間」)。
// アクティビティは「{ID} の作業時間を削除」。ないか、すでに削除された記録は port.ErrNotFound。
func (s Service) Delete(ctx context.Context, store port.StoreID, id item.ID, w change.ID) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	shown, err := s.displayOf(ctx, store, id)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: shown + " の作業時間を削除"}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		log, err := tx.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		if err != nil {
			return err
		}
		e, ok := worklog.Find(log, w)
		if !ok {
			return port.ErrNotFound
		}
		return tx.WriteItems().Append(id, change.NewWorklog(w, worklog.Value(e.Hundredths, e.Memo, e.Date), nil))
	})
}
```

- [ ] **Step 12: 通ることを確かめて、コミットする**

Run: `go test ./internal/app/worklogs/ ./internal/archtest/`
Expected: PASS

```bash
git add internal/app/worklogs
git commit -m "feat(app): 作業時間の一覧・記録・削除(A4a-8)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 13: API の失敗するテストを書く**

`internal/adapter/httpapi/notes/worklogs_test.go`:

```go
package notes_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/notes"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appworklogs "github.com/kukv/hedgehug/internal/app/worklogs"
)

func newWorklogsAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	notes.Register(api, notes.Services{
		Worklogs: appworklogs.Service{
			Stores: s, IDs: ids, Identity: apptest.Identity{Login: "hanako"},
			Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)),
		},
	})
	return api
}

func TestWorklogLifecycle(t *testing.T) {
	api := newWorklogsAPI(t)
	base := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/worklogs"
	res := api.Post(base, map[string]any{"hours": "1.25", "memo": "下書き"})
	body := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(body, `"hours":"1.25"`) || !strings.Contains(body, `"date":"2026-11-13"`) ||
		!strings.Contains(body, `"memo":"下書き"`) || !strings.Contains(body, `"byName":"hanako"`) {
		t.Fatalf("add: %d %s", res.Code, body)
	}
	var added notes.Worklog
	if err := json.Unmarshal(res.Body.Bytes(), &added); err != nil || added.ID == "" {
		t.Fatalf("added = %+v, %v", added, err)
	}
	if res := api.Get(base); res.Code != http.StatusOK || strings.Count(res.Body.String(), `"id":`) != 1 {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
	if res := api.Delete(base + "/" + added.ID); res.Code != http.StatusNoContent {
		t.Errorf("delete: %d %s", res.Code, res.Body)
	}
	if res := api.Get(base); res.Code != http.StatusOK || res.Body.String() != "[]\n" {
		t.Errorf("list after delete: %d %q", res.Code, res.Body)
	}
	if res := api.Delete(base + "/" + added.ID); res.Code != http.StatusNotFound {
		t.Errorf("delete twice: %d", res.Code)
	}
}

func TestWorklogRefusalsAndUnknowns(t *testing.T) {
	api := newWorklogsAPI(t)
	base := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/worklogs"
	res := api.Post(base, map[string]any{"hours": "0.123"})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "時間は小数第 2 位までで入力してください") {
		t.Errorf("scale: %d %s", res.Code, res.Body)
	}
	res = api.Post(base, map[string]any{"hours": ""})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "時間を入力してください") {
		t.Errorf("empty: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items/01NOPE00000000000000000000/worklogs", map[string]any{"hours": "1"}); res.Code != http.StatusNotFound {
		t.Errorf("unknown item: %d", res.Code)
	}
}
```

- [ ] **Step 14: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/notes/ -run Worklog`
Expected: FAIL(ビルドエラー `unknown field Worklogs in struct literal of type notes.Services`)

- [ ] **Step 15: 実装する**

`internal/adapter/httpapi/notes/worklogs.go`:

```go
package notes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	appworklogs "github.com/kukv/hedgehug/internal/app/worklogs"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

// Worklog は、API が返す作業時間の記録。Hours は「小数第 2 位まで・末尾の 0 を省いた」表記(「 時間」は付けない)。
type Worklog struct {
	ID     string `json:"id"`
	By     string `json:"by" doc:"記録した人のログイン名"`
	ByName string `json:"byName" doc:"記録した人の表示名。メンバーにいなければ「不明なユーザー」"`
	Date   string `json:"date" doc:"記録した日(YYYY-MM-DD)"`
	Hours  string `json:"hours" doc:"1.25 のような表記"`
	Memo   string `json:"memo"`
}

// WorklogInput は、作業時間の記録の本文。時間は文字列で受け取り、サーバーが検査する(A4a-26)。
type WorklogInput struct {
	Hours string `json:"hours" doc:"時間。小数第 2 位まで"`
	Memo  string `json:"memo,omitempty"`
}

type worklogOutput struct{ Body Worklog }

type worklogsOutput struct{ Body []Worklog }

type worklogsRef struct {
	Store string `path:"store"`
	Item  string `path:"item"`
}

type addWorklog struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  WorklogInput
}

type worklogRef struct {
	Store   string `path:"store"`
	Item    string `path:"item"`
	Worklog string `path:"worklog"`
}

func toWorklog(v appworklogs.View) Worklog {
	return Worklog{ID: string(v.ID), By: v.By, ByName: v.ByName, Date: v.Date, Hours: worklog.Format(v.Hundredths), Memo: v.Memo}
}

// registerWorklogs は、アイテムの作業時間の API を登録する。
func registerWorklogs(api huma.API, svc appworklogs.Service) {
	const base = "/api/stores/{store}/items/{item}/worklogs"
	huma.Register(api, huma.Operation{OperationID: "list-worklogs", Method: http.MethodGet, Path: base, Summary: "アイテムの作業時間の記録を古い順に返す"},
		func(ctx context.Context, in *worklogsRef) (*worklogsOutput, error) {
			vs, err := svc.List(ctx, port.StoreID(in.Store), item.ID(in.Item))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &worklogsOutput{Body: []Worklog{}}
			for _, v := range vs {
				out.Body = append(out.Body, toWorklog(v))
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "add-worklog", Method: http.MethodPost, Path: base, Summary: "作業時間を記録する"},
		func(ctx context.Context, in *addWorklog) (*worklogOutput, error) {
			v, err := svc.Add(ctx, port.StoreID(in.Store), item.ID(in.Item), in.Body.Hours, in.Body.Memo)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &worklogOutput{Body: toWorklog(v)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "delete-worklog", Method: http.MethodDelete, Path: base + "/{worklog}", Summary: "作業時間の記録を削除する", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *worklogRef) (*struct{}, error) {
			return nil, apierr.From(svc.Delete(ctx, port.StoreID(in.Store), item.ID(in.Item), change.ID(in.Worklog)))
		})
}
```

`notes.go` の `Services` に `Worklogs appworklogs.Service` を、`Register` に `registerWorklogs(api, s.Worklogs)` を足す(import に `appworklogs "github.com/kukv/hedgehug/internal/app/worklogs"`)。`routes.go` の `notes.Register(api, notes.Services{…})` に足す。

```go
		Worklogs: appworklogs.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity, Clock: deps.Clock},
```

- [ ] **Step 16: 生成して、通ることを確かめて、コミットする**

Run: `make generate && pnpm -C web gen`
Expected: `api/openapi.yaml` と `generated.ts` に `listWorklogs`・`addWorklog`・`deleteWorklog` と、スキーマ `Worklog`・`WorklogInput` が出る。

Run: `go test ./internal/... && pnpm -C web check`
Expected: PASS

```bash
git add internal api web/packages/api/src/generated.ts
git commit -m "feat(api): 作業時間の API を足す(A4a-8・A4a-24・A4a-26)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 12: 作業時間の区画

**Files:**
- Modify: `web/packages/core/src/model/notes.ts`(Task 10 が作る。`Worklog` を足す)
- Modify: `web/packages/core/src/port/notes.ts`(Task 7 が作る。作業時間のメソッドを足す)
- Modify: `web/packages/core/src/features/notes/fixtures.test-util.ts`(Task 7 の `fakeNotesPort` に作業時間のメソッドを足す。同じコミットで)
- Modify: `web/packages/api/src/notes.ts`・`notes.test.ts`(Task 7 が作る。作業時間のメソッドを足す)
- Create: `web/packages/core/src/features/notes/hours.ts`、Test: `hours.test.ts`
- Create: `web/packages/core/src/features/notes/worklogs.ts`、Test: `worklogs.test.ts`
- Modify: `web/packages/core/src/notesIndex.ts`(core の export はここに足す。`index.ts` は変えない)
- Create: `web/app/src/screens/item/WorklogSection.svelte`
- Modify: `web/app/src/wire.ts`(`worklogs`、画面への反映)
- Modify: `web/app/src/screens/item/DetailPanel.svelte`

**Interfaces:**
- Consumes: API `list-worklogs`・`add-worklog`・`delete-worklog`(Task 11)。orval の `listWorklogs(store, item)`・`addWorklog(store, item, body)`・`deleteWorklog(store, item, worklog)` と型 `Worklog`。`ConfirmDialog`・`Avatar`・`Input`・`IconButton`・`Button`、`formatDay(ymd: string, thisYear: number)`、`readOnlyNote`、`Toasts`、`DetailView`(`item.estimate`・`item.points`・`readOnly`)
- Produces:
  - `Worklog { id; by; byName; date; hours: string; memo: string }`(`hours` は「1.25」の表記)
  - `NotesPort.listWorklogs(storeId, itemId): Promise<Worklog[]>`・`addWorklog(storeId, itemId, hours: string, memo: string): Promise<Worklog>`・`deleteWorklog(storeId, itemId, worklogId): Promise<void>`
  - `parseHours(input: string): ParsedHours`(`ParsedHours = { ok: true; hundredths: number } | { ok: false; message: string }`)、`formatHours(hundredths: number): string`、`hoursOf(hours: number): string`(見積もりのような時間の数を同じ表記に)、`stepHours(text: string, steps: number): string`(0.25 時間刻み)、`worklogHeading(rows: Worklog[], item: { estimate: number | null; points: number | null }): string`
  - `createWorklogs(deps: WorklogsDeps): Worklogs`(`WorklogsDeps = { notes: Pick<NotesPort, "listWorklogs" | "addWorklog" | "deleteWorklog">; toasts: Toasts; now: () => Date }`)
  - `Worklogs { state: Readable<WorklogsState>; open(storeId, itemId): Promise<void>; reload(): Promise<void>; close(): void; setHours(text): void; setMemo(text): void; step(steps: number): void; add(): Promise<boolean>; askDelete(id): void; cancelDelete(): void; confirmDelete(): Promise<void> }`、文言 `worklogDeleteDialog`・`worklogDeletedMessage`
  - `WorklogSection.svelte`(props `{ v: DetailView }`。アイテムだけ)、`wire.ts` の `worklogs`

**文言(02「作業時間」):** 見出しは「作業時間」のあとに「N 時間」を出し、見積もり時間があれば「 · 見積もり {N} 時間」、ポイントがあれば「 · {N} pt」を、この順に続ける。時間は小数第 2 位まで・末尾の 0 を省く。各行に、記録した人のアバター、「N 時間」、メモ、日付、削除の「×」。入力は、時間・メモ(任意)・「記録」。時間の欄は 0.25 時間刻みで増減できる。メモ欄で Enter を押しても記録する。0 以下・数でない・空は「時間を入力してください」、小数第 3 位以下は「時間は小数第 2 位までで入力してください」(入力欄の下に赤字で出して記録しない)。削除の確認はタイトル「作業時間を削除」、本文「{日付} の {N} 時間の記録を削除します。」、ボタン「削除」(危険な操作の見た目)と「キャンセル」。トースト「作業時間を削除しました」。閲覧のみの Store では、入力と「×」を無効にして「閲覧のみの Store です」を添える。日付は、01 の日付の表示(今年なら M/D、ほかの年なら YYYY/MM/DD。`formatDay`)で出す。

- [ ] **Step 1: 時間の検査と表記の失敗するテストを書く**

`web/packages/core/src/features/notes/hours.test.ts`。表は Go の `internal/domain/worklog/hours_test.go` と同じにする(サーバーと画面で同じ規則)。

```ts
import { describe, expect, it } from "vitest";
import type { Worklog } from "../../model/notes";
import { formatHours, hoursOf, parseHours, stepHours, worklogHeading } from "./hours";

const empty = "時間を入力してください";
const scale = "時間は小数第 2 位までで入力してください";

describe("parseHours", () => {
  it("reads a decimal into hundredths of an hour", () => {
    const ok: Record<string, number> = {
      "1": 100,
      "1.5": 150,
      "0.25": 25,
      "0.1": 10,
      ".5": 50,
      "1.": 100,
      "1.250": 125,
      "0.07": 7,
      " 2 ": 200,
      "12": 1200,
      "00.5": 50,
      "999999999": 99999999900,
    };
    for (const [input, hundredths] of Object.entries(ok)) {
      expect(parseHours(input), input).toEqual({ ok: true, hundredths });
    }
  });

  it("refuses an empty, non-numeric or non-positive input", () => {
    for (const input of ["", " ", "0", "0.0", "0.000", "-1", "-0.5", "abc", "1e2", "1,5", ".", "１", "1000000000"]) {
      expect(parseHours(input), input).toEqual({ ok: false, message: empty });
    }
  });

  it("refuses a nonzero digit past the second decimal place", () => {
    for (const input of ["0.123", "1.005", "0.001", "2.999"]) {
      expect(parseHours(input), input).toEqual({ ok: false, message: scale });
    }
  });
});

describe("formatHours", () => {
  it("shows two decimals at most and drops trailing zeros", () => {
    const want: Record<number, string> = { 0: "0", 7: "0.07", 10: "0.1", 25: "0.25", 30: "0.3", 100: "1", 110: "1.1", 125: "1.25", 200: "2", 1234: "12.34" };
    for (const [h, s] of Object.entries(want)) expect(formatHours(Number(h))).toBe(s);
  });

  it("formats a number of hours the same way", () => {
    expect(hoursOf(6)).toBe("6");
    expect(hoursOf(1.5)).toBe("1.5");
    expect(hoursOf(0.29)).toBe("0.29");
  });
});

describe("stepHours", () => {
  it("moves by a quarter hour and never below zero", () => {
    expect(stepHours("", 1)).toBe("0.25");
    expect(stepHours("1", 1)).toBe("1.25");
    expect(stepHours("1.25", -1)).toBe("1");
    expect(stepHours("0.25", -1)).toBe("");
    expect(stepHours("0", -1)).toBe("");
    expect(stepHours("abc", 1)).toBe("0.25");
    expect(stepHours("0.1", 1)).toBe("0.35");
  });
});

function w(id: string, hours: string): Worklog {
  return { id, by: "hanako", byName: "花子", date: "2026-11-13", hours, memo: "" };
}

describe("worklogHeading", () => {
  it("adds the hundredths, then the estimate and the points when there are any", () => {
    const rows = [w("1", "0.3"), w("2", "1.25"), w("3", "0.45")];
    expect(worklogHeading(rows, { estimate: null, points: null })).toBe("2 時間");
    expect(worklogHeading(rows, { estimate: 6, points: 3 })).toBe("2 時間 · 見積もり 6 時間 · 3 pt");
    expect(worklogHeading([], { estimate: 1.5, points: null })).toBe("0 時間 · 見積もり 1.5 時間");
    expect(worklogHeading(rows, { estimate: null, points: 0.5 })).toBe("2 時間 · 0.5 pt");
  });

  it("adds in hundredths so that 0.1 + 0.2 is 0.3", () => {
    expect(worklogHeading([w("1", "0.1"), w("2", "0.2")], { estimate: null, points: null })).toBe("0.3 時間");
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- hours`
Expected: FAIL(`Failed to resolve import "./hours"`)

- [ ] **Step 3: 実装する**

`web/packages/core/src/model/notes.ts` に足す(`Comment` と同じファイル)。

```ts
// 作業時間の記録 1 件(02「作業時間」)。hours は「1.25」の表記(小数第 2 位まで・末尾の 0 を省く)、
// date は記録した日(YYYY-MM-DD)、byName は記録した人の表示名(メンバーにいなければ「不明なユーザー」)。
export interface Worklog {
  id: string;
  by: string;
  byName: string;
  date: string;
  hours: string;
  memo: string;
}
```

`web/packages/core/src/features/notes/hours.ts`:

```ts
import type { Worklog } from "../../model/notes";

// 作業時間の時間の検査と表記(02「作業時間」)。サーバーの internal/domain/worklog と同じ規則で、
// 画面でも先に検査する(A4a-9)。時間は 100 分の 1 時間の整数で持ち、文字列のまま見る(浮動小数点の丸めに左右されない)。

const empty = "時間を入力してください";
const scale = "時間は小数第 2 位までで入力してください";
const decimal = /^(\d*)(?:\.(\d*))?$/;
// 整数部の桁数の上限。100 分の 1 時間にして 2^53 を超えないようにする(仕様に上限はない。届く値ではない)。
const maxWholeDigits = 9;

export type ParsedHours = { ok: true; hundredths: number } | { ok: false; message: string };

const fail = (message: string): ParsedHours => ({ ok: false, message });

// parseHours は、時間の入力を 100 分の 1 時間の整数にする。空・数でない・0 以下は「時間を入力してください」、
// 小数第 3 位以下に 0 でない数字があれば「時間は小数第 2 位までで入力してください」(末尾の 0 は数えない)。
export function parseHours(input: string): ParsedHours {
  const m = decimal.exec(input.trim());
  const intPart = m?.[1] ?? "";
  const fracPart = m?.[2] ?? "";
  if (!m || (intPart === "" && fracPart === "")) return fail(empty);
  const whole = intPart.replace(/^0+/, "");
  const frac = fracPart.replace(/0+$/, "");
  if (frac.length > 2) return fail(scale);
  if (whole.length > maxWholeDigits) return fail(empty);
  const hundredths = Number(whole === "" ? "0" : whole) * 100 + Number(`${frac}00`.slice(0, 2));
  return hundredths > 0 ? { ok: true, hundredths } : fail(empty);
}

// formatHours は、100 分の 1 時間の整数を、小数第 2 位まで・末尾の 0 を省いた表記にする(0.3・1.25・2)。「 時間」は付けない。
export function formatHours(hundredths: number): string {
  const whole = Math.floor(hundredths / 100);
  const frac = hundredths % 100;
  if (frac === 0) return String(whole);
  return `${whole}.${String(frac).padStart(2, "0").replace(/0+$/, "")}`;
}

// hoursOf は、見積もり時間のような時間の数を、同じ表記にする。
export const hoursOf = (hours: number): string => formatHours(Math.round(hours * 100));

// stepHours は、時間の欄の入力を 0.25 時間ずつ増減する(steps は 1 か -1)。0 以下になるときは空にする。
// 読めない入力は 0 として数える。
export function stepHours(text: string, steps: number): string {
  const parsed = parseHours(text);
  const next = Math.max(0, (parsed.ok ? parsed.hundredths : 0) + steps * 25);
  return next === 0 ? "" : formatHours(next);
}

function sum(rows: Worklog[]): number {
  let total = 0;
  for (const r of rows) {
    const p = parseHours(r.hours);
    if (p.ok) total += p.hundredths;
  }
  return total;
}

// worklogHeading は、見出しの「N 時間」に、見積もり時間とポイントがあれば続ける(02「作業時間」)。
export function worklogHeading(
  rows: Worklog[],
  item: { estimate: number | null; points: number | null },
): string {
  const parts = [`${formatHours(sum(rows))} 時間`];
  if (item.estimate !== null) parts.push(`見積もり ${hoursOf(item.estimate)} 時間`);
  if (item.points !== null) parts.push(`${item.points} pt`);
  return parts.join(" · ");
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- hours`
Expected: PASS

- [ ] **Step 5: 口の失敗するテストを書く**

`web/packages/api/src/notes.test.ts` に足す(Task 7 が作ったファイルの `respond`・`call` を使う。`Refused` の import は Task 10 が足している)。

```ts
describe("notes port: worklogs", () => {
  const apiWorklog = {
    id: "W1",
    by: "hanako",
    byName: "花子",
    date: "2026-11-13",
    hours: "1.25",
    memo: "下書き",
  };

  it("lists the worklogs of an item, reading null as empty", async () => {
    const fetch = respond(200, [apiWorklog]);
    await expect(createNotesPort().listWorklogs("S1", "I1")).resolves.toEqual([apiWorklog]);
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/worklogs");
    respond(200, null);
    await expect(createNotesPort().listWorklogs("S1", "I1")).resolves.toEqual([]);
  });

  it("adds with the hours as typed, and deletes", async () => {
    const added = respond(200, apiWorklog);
    await createNotesPort().addWorklog("S1", "I1", "1.25", "下書き");
    expect(call(added)[1].method).toBe("POST");
    expect(JSON.parse(String(call(added)[1].body))).toEqual({ hours: "1.25", memo: "下書き" });
    const deleted = respond(204, undefined);
    await expect(createNotesPort().deleteWorklog("S1", "I1", "W1")).resolves.toBeUndefined();
    expect(call(deleted)[0]).toBe("/api/stores/S1/items/I1/worklogs/W1");
    expect(call(deleted)[1].method).toBe("DELETE");
  });

  it("turns a 422 into Refused with the words of the spec", async () => {
    respond(422, { title: "Unprocessable Entity", status: 422, detail: "時間を入力してください" });
    const err = await createNotesPort()
      .addWorklog("S1", "I1", "", "")
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(Refused);
    expect((err as Refused).message).toBe("時間を入力してください");
  });
});
```

- [ ] **Step 6: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/api test -- notes`
Expected: FAIL(`listWorklogs is not a function`、または型エラー)

- [ ] **Step 7: 実装する**

`web/packages/core/src/port/notes.ts` の `NotesPort` に足す(`import type { Comment, Worklog } from "../model/notes";` にする)。

```ts
  // listWorklogs は、アイテムの作業時間の記録を古い順にすべて返す。
  listWorklogs(storeId: string, itemId: string): Promise<Worklog[]>;
  // addWorklog は、作業時間を記録する。hours は入力のままの文字列で、サーバーも検査する(断りは Refused)。
  addWorklog(storeId: string, itemId: string, hours: string, memo: string): Promise<Worklog>;
  deleteWorklog(storeId: string, itemId: string, worklogId: string): Promise<void>;
```

`fakeNotesPort`(`fixtures.test-util.ts`)にも、型を合わせるスタブを足す(呼ばれたら投げる単純なもの)。

```ts
    async listWorklogs() {
      return [];
    },
    async addWorklog() {
      throw new Error("addWorklog is not faked");
    },
    async deleteWorklog() {
      throw new Error("deleteWorklog is not faked");
    },
```

`web/packages/api/src/notes.ts` に足す(import に `addWorklog`・`deleteWorklog`・`listWorklogs` と `type Worklog as ApiWorklog` を足す)。

```ts
function toWorklog(w: ApiWorklog): Worklog {
  return { id: w.id, by: w.by, byName: w.byName, date: w.date, hours: w.hours, memo: w.memo };
}
```

`createNotesPort` の返すオブジェクトに足す。

```ts
    listWorklogs: (storeId, itemId) =>
      call(async () => ((await listWorklogs(storeId, itemId)) ?? []).map(toWorklog)),
    addWorklog: (storeId, itemId, hours, memo) =>
      call(async () => toWorklog(await addWorklog(storeId, itemId, { hours, memo }))),
    deleteWorklog: (storeId, itemId, worklogId) =>
      call(async () => {
        await deleteWorklog(storeId, itemId, worklogId);
      }),
```

`web/packages/core/src/notesIndex.ts` に `Worklog` の型を足す(`export type { Comment, Worklog } from "./model/notes";`)。

- [ ] **Step 8: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/api test -- notes && pnpm -C web check`
Expected: PASS

- [ ] **Step 9: 作業時間の状態の失敗するテストを書く**

`web/packages/core/src/features/notes/worklogs.test.ts`:

```ts
import { describe, expect, it, vi } from "vitest";
import { NotFound, Refused } from "../../model/errors";
import type { Worklog } from "../../model/notes";
import { get } from "../../reactive/store";
import { createToasts } from "../toasts/toasts";
import { createWorklogs, worklogDeleteDialog, worklogDeletedMessage } from "./worklogs";

function entry(id: string, hours: string, date = "2026-11-13", memo = ""): Worklog {
  return { id, by: "hanako", byName: "花子", date, hours, memo };
}

function setup(initial: Worklog[] = [entry("W1", "1.25", "2026-11-13", "下書き")]) {
  let rows = initial;
  const calls: string[] = [];
  const notes = {
    listWorklogs: vi.fn(async () => rows),
    addWorklog: vi.fn(async (_s: string, _i: string, hours: string, memo: string) => {
      calls.push(`add:${hours}:${memo}`);
      const w = entry(`W${rows.length + 1}`, hours, "2026-11-13", memo);
      rows = [...rows, w];
      return w;
    }),
    deleteWorklog: vi.fn(async (_s: string, _i: string, id: string) => {
      calls.push(`delete:${id}`);
      rows = rows.filter((r) => r.id !== id);
    }),
  };
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const worklogs = createWorklogs({ notes, toasts, now: () => new Date(2026, 10, 13, 12, 0, 0) });
  return { worklogs, notes, toasts, calls, setRows: (r: Worklog[]) => (rows = r) };
}

describe("createWorklogs", () => {
  it("opens an item with its worklogs, and keeps the input for the same item", async () => {
    const { worklogs } = setup();
    await worklogs.open("S1", "I1");
    expect(get(worklogs.state).rows.map((r) => r.id)).toEqual(["W1"]);
    worklogs.setHours("2");
    await worklogs.open("S1", "I1");
    expect(get(worklogs.state).hours).toBe("2");
    await worklogs.open("S1", "I2");
    expect(get(worklogs.state)).toMatchObject({ hours: "", memo: "", error: "" });
  });

  it("records the hours as typed with the memo, clears the inputs and reloads", async () => {
    const { worklogs, calls } = setup();
    await worklogs.open("S1", "I1");
    worklogs.setHours(" 0.5 ");
    worklogs.setMemo(" レビュー ");
    expect(await worklogs.add()).toBe(true);
    expect(calls).toEqual(["add:0.5:レビュー"]);
    const s = get(worklogs.state);
    expect(s).toMatchObject({ hours: "", memo: "", error: "" });
    expect(s.rows.map((r) => r.hours)).toEqual(["1.25", "0.5"]);
  });

  it("shows the refusal under the field without sending when the hours are bad", async () => {
    const { worklogs, notes } = setup();
    await worklogs.open("S1", "I1");
    for (const [input, message] of [
      ["", "時間を入力してください"],
      ["0", "時間を入力してください"],
      ["abc", "時間を入力してください"],
      ["0.123", "時間は小数第 2 位までで入力してください"],
    ]) {
      worklogs.setHours(input as string);
      expect(await worklogs.add()).toBe(false);
      expect(get(worklogs.state).error).toBe(message);
    }
    expect(notes.addWorklog).not.toHaveBeenCalled();
    worklogs.setHours("1");
    expect(get(worklogs.state).error).toBe("");
  });

  it("shows a refusal from the server under the field and keeps the input", async () => {
    const { worklogs, notes } = setup();
    notes.addWorklog.mockRejectedValueOnce(new Refused("時間を入力してください"));
    await worklogs.open("S1", "I1");
    worklogs.setHours("1");
    expect(await worklogs.add()).toBe(false);
    expect(get(worklogs.state)).toMatchObject({ hours: "1", error: "時間を入力してください" });
  });

  it("steps the hours by a quarter hour and clears the error", async () => {
    const { worklogs } = setup();
    await worklogs.open("S1", "I1");
    worklogs.setHours("x");
    await worklogs.add();
    worklogs.step(1);
    expect(get(worklogs.state)).toMatchObject({ hours: "0.25", error: "" });
    worklogs.step(1);
    expect(get(worklogs.state).hours).toBe("0.5");
    worklogs.step(-1);
    worklogs.step(-1);
    expect(get(worklogs.state).hours).toBe("");
  });

  it("asks with the date and hours, then deletes, shows the toast and reloads", async () => {
    const { worklogs, calls, toasts } = setup();
    await worklogs.open("S1", "I1");
    worklogs.askDelete("W1");
    expect(get(worklogs.state).deleting).toEqual({ id: "W1", lines: ["11/13 の 1.25 時間の記録を削除します。"] });
    worklogs.cancelDelete();
    expect(get(worklogs.state).deleting).toBeNull();
    worklogs.askDelete("W1");
    await worklogs.confirmDelete();
    expect(calls).toEqual(["delete:W1"]);
    expect(get(worklogs.state)).toMatchObject({ rows: [], deleting: null });
    expect(get(toasts.list)[0]).toMatchObject({ text: worklogDeletedMessage, error: false });
  });

  it("writes the year of a record from another year", async () => {
    const { worklogs } = setup([entry("W1", "2", "2025-12-31")]);
    await worklogs.open("S1", "I1");
    worklogs.askDelete("W1");
    expect(get(worklogs.state).deleting?.lines).toEqual(["2025/12/31 の 2 時間の記録を削除します。"]);
  });

  it("reloads quietly when the record was already deleted elsewhere", async () => {
    const { worklogs, notes, toasts, setRows } = setup();
    await worklogs.open("S1", "I1");
    notes.deleteWorklog.mockImplementationOnce(async () => {
      setRows([]);
      throw new NotFound("not found");
    });
    worklogs.askDelete("W1");
    await worklogs.confirmDelete();
    expect(get(worklogs.state).rows).toEqual([]);
    expect(get(toasts.list)).toEqual([]);
  });

  it("drops everything when closed", async () => {
    const { worklogs } = setup();
    await worklogs.open("S1", "I1");
    worklogs.setHours("2");
    worklogs.close();
    expect(get(worklogs.state)).toMatchObject({ target: null, rows: [], hours: "" });
  });

  it("uses the words of the spec", () => {
    expect(worklogDeleteDialog).toEqual({ title: "作業時間を削除", ok: "削除" });
    expect(worklogDeletedMessage).toBe("作業時間を削除しました");
  });
});
```

- [ ] **Step 10: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- worklogs`
Expected: FAIL(`Failed to resolve import "./worklogs"`)

- [ ] **Step 11: 実装する**

`web/packages/core/src/features/notes/worklogs.ts`:

```ts
import { NotFound, Refused } from "../../model/errors";
import type { Worklog } from "../../model/notes";
import type { NotesPort } from "../../port/notes";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import { formatDay } from "../dates/date";
import type { Toasts } from "../toasts/toasts";
import { parseHours, stepHours } from "./hours";

// 作業時間の区画の状態(02「作業時間」)。アイテムの詳細パネルだけが持つ。

export const worklogDeleteDialog = { title: "作業時間を削除", ok: "削除" };
export const worklogDeletedMessage = "作業時間を削除しました";

export interface WorklogsState {
  target: { storeId: string; itemId: string } | null;
  rows: Worklog[];
  hours: string;
  memo: string;
  // error は、入力欄の下の赤字。時間の入力を変えるか、記録できたら消す。
  error: string;
  // deleting は、削除の確認。出していなければ null。lines は確認の本文。
  deleting: { id: string; lines: string[] } | null;
  busy: boolean;
}

export interface Worklogs {
  state: Readable<WorklogsState>;
  // open は、アイテムの作業時間を読む。アイテムが変わったら入力を空にする。同じアイテムなら入力は残す。
  open(storeId: string, itemId: string): Promise<void>;
  // reload は、画面への反映(06)。入力は書き換えない。
  reload(): Promise<void>;
  close(): void;
  setHours(text: string): void;
  setMemo(text: string): void;
  // step は、時間の欄を 0.25 時間ずつ増減する(1 か -1)。
  step(steps: number): void;
  // add は、記録する。時間が不正なら、赤字を出して送らない(false)。
  add(): Promise<boolean>;
  askDelete(id: string): void;
  cancelDelete(): void;
  confirmDelete(): Promise<void>;
}

export interface WorklogsDeps {
  notes: Pick<NotesPort, "listWorklogs" | "addWorklog" | "deleteWorklog">;
  toasts: Toasts;
  // now は、削除の確認の日付を「今年なら M/D」にするための、このマシンの時刻。
  now: () => Date;
}

const initial = (): WorklogsState => ({
  target: null,
  rows: [],
  hours: "",
  memo: "",
  error: "",
  deleting: null,
  busy: false,
});

export function createWorklogs(deps: WorklogsDeps): Worklogs {
  const state = writable<WorklogsState>(initial());
  const patch = (p: Partial<WorklogsState>) => state.update((s) => ({ ...s, ...p }));
  let latest = 0;

  async function load(storeId: string, itemId: string) {
    latest += 1;
    const mine = latest;
    try {
      const rows = await deps.notes.listWorklogs(storeId, itemId);
      if (mine === latest) patch({ rows });
    } catch (e) {
      // アイテムがなくなっていたら、詳細パネルが閉じるので何もしない。
      if (!(e instanceof NotFound)) throw e;
    }
  }

  async function add(): Promise<boolean> {
    const { target, hours, memo, busy } = get(state);
    if (!target || busy) return false;
    const parsed = parseHours(hours);
    if (!parsed.ok) {
      patch({ error: parsed.message });
      return false;
    }
    patch({ busy: true, error: "" });
    try {
      await deps.notes.addWorklog(target.storeId, target.itemId, hours.trim(), memo.trim());
    } catch (e) {
      if (!(e instanceof Refused)) throw e;
      patch({ error: e.message });
      return false;
    } finally {
      patch({ busy: false });
    }
    const same = get(state).target;
    if (same?.storeId === target.storeId && same.itemId === target.itemId) {
      patch({ hours: "", memo: "" });
    }
    await load(target.storeId, target.itemId);
    return true;
  }

  async function confirmDelete() {
    const { target, deleting } = get(state);
    patch({ deleting: null });
    if (!target || !deleting) return;
    try {
      await deps.notes.deleteWorklog(target.storeId, target.itemId, deleting.id);
    } catch (e) {
      // ほかで消えていたら、読み直すだけにする。
      if (e instanceof NotFound) {
        await load(target.storeId, target.itemId);
        return;
      }
      if (!(e instanceof Refused)) throw e;
      deps.toasts.show(e.message, { error: true });
      return;
    }
    deps.toasts.show(worklogDeletedMessage);
    await load(target.storeId, target.itemId);
  }

  return {
    state: readonly(state),
    open(storeId, itemId) {
      const t = get(state).target;
      if (t?.storeId !== storeId || t.itemId !== itemId) state.set({ ...initial(), target: { storeId, itemId } });
      return load(storeId, itemId);
    },
    async reload() {
      const { target } = get(state);
      if (target) await load(target.storeId, target.itemId);
    },
    close() {
      latest += 1;
      state.set(initial());
    },
    setHours: (text) => patch({ hours: text, error: "" }),
    setMemo: (text) => patch({ memo: text }),
    step: (steps) => patch({ hours: stepHours(get(state).hours, steps), error: "" }),
    add,
    askDelete(id) {
      const row = get(state).rows.find((r) => r.id === id);
      if (!row) return;
      const day = formatDay(row.date, deps.now().getFullYear());
      patch({ deleting: { id, lines: [`${day} の ${row.hours} 時間の記録を削除します。`] } });
    },
    cancelDelete: () => patch({ deleting: null }),
    confirmDelete,
  };
}
```

`web/packages/core/src/notesIndex.ts` に足す(`index.ts` は変えない)。

```ts
export {
  formatHours,
  hoursOf,
  type ParsedHours,
  parseHours,
  stepHours,
  worklogHeading,
} from "./features/notes/hours";
export {
  createWorklogs,
  type Worklogs,
  type WorklogsDeps,
  type WorklogsState,
  worklogDeletedMessage,
  worklogDeleteDialog,
} from "./features/notes/worklogs";
```

- [ ] **Step 12: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- notes && pnpm -C web check`
Expected: PASS

- [ ] **Step 13: 画面を作る**

`web/app/src/screens/item/WorklogSection.svelte`(200 行以下):

```svelte
<script lang="ts">
  import {
    type DetailView,
    formatDay,
    readOnlyNote,
    worklogDeleteDialog,
    worklogHeading,
  } from "@hedgehug/core";
  import { Avatar, Button, ConfirmDialog, IconButton, Input } from "@hedgehug/ui";
  import { onDestroy } from "svelte";
  import { syncNow, worklogs } from "../../wire";

  // 作業時間の区画(02「作業時間」)。行に編集はない。入れ間違いは、削除して入れ直す。
  interface Props {
    v: DetailView;
  }

  let { v }: Props = $props();

  const store = worklogs.state;
  const note = $derived(v.readOnly ? readOnlyNote : undefined);
  const heading = $derived(worklogHeading($store.rows, v.item));
  const year = $derived($syncNow.getFullYear());

  $effect(() => {
    void worklogs.open(v.store.id, v.item.id);
  });
  onDestroy(() => worklogs.close());

  function hoursKeydown(event: KeyboardEvent) {
    if (event.isComposing) return;
    if (event.key === "ArrowUp") {
      event.preventDefault();
      worklogs.step(1);
    } else if (event.key === "ArrowDown") {
      event.preventDefault();
      worklogs.step(-1);
    }
  }

  // メモ欄の Enter でも記録する。日本語入力の変換を確定する Enter では記録しない。
  function memoKeydown(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) void worklogs.add();
  }
</script>

<section class="peek-sec" aria-label="作業時間">
  <h3>作業時間 {heading}</h3>
  {#each $store.rows as w (w.id)}
    <div class="row">
      <Avatar name={w.byName === "不明なユーザー" ? "" : w.byName} title={w.byName} />
      <span class="hours">{w.hours} 時間</span>
      <span class="memo">{w.memo}</span>
      <span class="date">{formatDay(w.date, year)}</span>
      <IconButton
        label="作業時間を削除"
        size="xs"
        disabled={v.readOnly}
        title={note}
        onclick={() => worklogs.askDelete(w.id)}>×</IconButton
      >
    </div>
  {/each}
  <div class="input-row">
    <IconButton label="0.25 時間減らす" size="sm" disabled={v.readOnly} title={note} onclick={() => worklogs.step(-1)}>−</IconButton>
    <Input
      label="時間"
      placeholder="時間"
      size="sm"
      disabled={v.readOnly}
      title={note}
      onkeydown={hoursKeydown}
      bind:value={() => $store.hours, (t) => worklogs.setHours(t)}
    />
    <IconButton label="0.25 時間増やす" size="sm" disabled={v.readOnly} title={note} onclick={() => worklogs.step(1)}>+</IconButton>
    <Input
      label="メモ"
      placeholder="メモ(任意)"
      size="sm"
      disabled={v.readOnly}
      title={note}
      onkeydown={memoKeydown}
      bind:value={() => $store.memo, (t) => worklogs.setMemo(t)}
    />
    <Button size="sm" variant="primary" disabled={v.readOnly || $store.busy} title={note} onclick={() => void worklogs.add()}>記録</Button>
  </div>
  {#if $store.error}
    <p class="error" role="alert">{$store.error}</p>
  {/if}
  {#if $store.deleting}
    <ConfirmDialog
      title={worklogDeleteDialog.title}
      lines={$store.deleting.lines}
      ok={worklogDeleteDialog.ok}
      danger
      onok={() => void worklogs.confirmDelete()}
      oncancel={worklogs.cancelDelete}
    />
  {/if}
</section>

<style>
  .peek-sec {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  h3 {
    font-size: 13px;
    margin: 0;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 30px;
    border-bottom: 1px solid var(--line);
    font-size: 12.5px;
  }
  .hours {
    font-weight: 600;
  }
  .memo {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .date {
    color: var(--muted);
  }
  .input-row {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }
  .error {
    margin: 0;
    color: var(--danger);
    font-size: 12.5px;
  }
</style>
```

`web/app/src/wire.ts`: `createWorklogs` を core の import に足し、`notes`(Task 7 が作る `NotesPort`)を使って、`detail` の定義の近くに足す。

```ts
export const worklogs = createWorklogs({ notes, toasts, now: () => new Date() });
```

画面への反映(`reflect`)で、読み直す。`Promise.all` の配列に `worklogs.reload()` を足す(Task 10 が足した `itemComments.reload()`・`projectComments.reload()` と並べる)。

```ts
          Promise.all([projectScreen.reload(), detail.reload(), worklogs.reload()]),
```

`web/app/src/screens/item/DetailPanel.svelte`: import を足し、GitHub の区画の直後(コメントの区画の前)に足す。

```svelte
  import WorklogSection from "./WorklogSection.svelte";
```

```svelte
    <WorklogSection {v} />
```

- [ ] **Step 14: 確かめる**

Run: `pnpm -C web fix && pnpm -C web check && pnpm -C web test && pnpm -C web lint && pnpm -C web deps && pnpm -C web size`
Expected: すべて PASS(`WorklogSection.svelte` は 200 行以下。超えたら記録の 1 行を `WorklogRow.svelte` に分ける)

画面の動き(記録・削除・「×」の確認・0.25 刻み・閲覧のみの Store)は、Task 22 の e2e で通しで確かめる。

- [ ] **Step 15: コミット**

```bash
git add web
git commit -m "feat(web): 詳細パネルに作業時間の区画を足す(A4a-9)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 13: 関連の規則・操作・候補 API・ブロック中

関連(02「関連」)を、サーバーで読み書きできるようにする。規則は domain の `relation` に、操作は新しい app パッケージ `relations` に置く。アイテムの応答には `blocked` を足す(A4a-13)。画面は Task 14。

**この Task が決めること**(骨組みの決定の細部。決定 A4a-10〜13 に従う):

- 保存する `relations` の値は `{"type": "blocks", "to": 相手の内部 ID}`。`item.Relation` を domain/item に置き、`relation` パッケージが `item` を import する(骨組みの「タスク間の名前」のとおり)。
- 表記 9 つと保存する種類の対応: blocks=(blocks, 持つ側)、blocked by=(blocks, 持たれる側)、causes/caused by=causes、duplicates/duplicated by=duplicates、relates to=relates(向きなし)、clones/cloned by=clones。
- 「ブロック中」の判定には、domain/workspace に `Settings.Finished(status)`(完了か中止の分類か)を足す。
- `Item.Blocked bool` は保存しない派生の値で、`app/items` の `Get`・`List`・`FindByNumber` が埋める。変更の操作の戻り値は `Get` を通るので、どの応答でも `blocked` が正しい。
- 関連を読む処理(ワークスペース → プロジェクト → アイテムの順にすべて読む)は、`app/items` と `app/relations` のどちらにも要る。app の機能どうしは import できないので、それぞれに小さく持つ(`app/items/view.go` の `blockedSet`、`app/relations/snapshot.go` の `readSnapshot`)。
- 一覧の同じ表記の中の順は、相手の内部 ID の順(内部 ID は ULID なので作った順)。仕様に順の指定はない。
- 候補の検索語が空(前後の空白を除いて)のときは、候補を返さない(仕様は「入力すると候補を出す」)。
- 「すでに {表記} の関連があります」の {表記} は、利用者がセレクトで選んでいる表記。
- 解除は、選んだ表記で決まる側のログに `remove` を書く。`relates to` だけは、どちらの側が持っていても外す。持っていなければ何も保存せず、エラーにもしない。
- 候補にならない相手(自分・Step・存在しないアイテム)を指定した追加は `port.ErrNotFound`(404)。知らない表記も 404。
- 削除済みの相手を除くのは A4b(削除がまだない)。

**Files:**
- Modify: `internal/domain/item/item.go`(`Relation`・`Item.Relations`・`Item.Blocked`、`FromLog` が読む)
- Modify: `internal/domain/item/values.go`(`multi` に `relations` を足す)
- Test: `internal/domain/item/relations_test.go`(新規)
- Create: `internal/domain/workspace/finished.go`
- Test: `internal/domain/workspace/finished_test.go`(新規)
- Create: `internal/domain/relation/relation.go`
- Test: `internal/domain/relation/relation_test.go`
- Create: `internal/app/relations/service.go`・`snapshot.go`・`list.go`・`candidates.go`・`add.go`・`remove.go`
- Test: `internal/app/relations/relations_test.go`(補助)・`list_test.go`・`edit_test.go`
- Modify: `internal/app/items/view.go`(`blockedSet`)・`get.go`・`list.go`・`numbers.go`
- Test: `internal/app/items/blocked_test.go`(新規)
- Create: `internal/adapter/httpapi/notes/relations.go`
- Test: `internal/adapter/httpapi/notes/relations_test.go`
- Modify: `internal/adapter/httpapi/notes/`(Task 3 が作った `Services` に `Relations` を足し、`Register` に `registerRelations(api, s.Relations)` を 1 行足す)・`internal/adapter/httpapi/routes.go`(`Services` を組むところに `Relations` を足す)
- Modify: `internal/adapter/httpapi/items/items.go`(`Item.Blocked`)・`items_test.go`
- Generated: `api/openapi.yaml`・`web/packages/api/src/generated.ts`

**Interfaces:**
- Consumes:
  - `change.NewAdd(f string, v any) Draft`、`change.NewRemove(f string, v any) Draft`
  - `port.Reader.Workspaces().List()`・`Settings(id)`、`Projects().List(ws)`、`Items().List(p)`(既存。新しい口は足さない)
  - `notes.Services`・`notes.Register`(Task 3 が作る。このタスクは `Relations` を 1 つ足すだけ)
- Produces:
  - `item.Relation{Type string; To ID}`、`item.Item.Relations []Relation`、`item.Item.Blocked bool`
  - `func (s Settings) Finished(id change.ID) bool`(domain/workspace)
  - `relation.Label`(9 つの定数 `Blocks`・`BlockedBy`・`Causes`・`CausedBy`・`Duplicates`・`DuplicatedBy`・`RelatesTo`・`Clones`・`ClonedBy`)、`relation.Labels`、`relation.TypeBlocks`
  - `func relation.Parse(l Label) (typ string, reverse bool, ok bool)`、`func relation.LabelOf(typ string, reverse bool) (Label, bool)`
  - `type relation.Row struct{ Label Label; Other item.ID }`、`func Rows(self item.ID, all []item.Item) []Row`
  - `func Has(a, b item.ID, typ string, all []item.Item) bool`、`func WouldCycle(from, to item.ID, all []item.Item) bool`
  - `func BlockedSet(all []item.Item, closed func(item.Item) bool) map[item.ID]bool`、`func Blocked(self item.ID, all []item.Item, closed func(item.Item) bool) bool`
  - `relations.Service{Stores port.Stores; Identity port.Identity}` と `List(ctx, store, self) ([]Row, error)`・`Candidates(ctx, store, self, label, query) ([]Candidate, error)`・`Add(ctx, store, self, label, other) error`・`Remove(ctx, store, self, label, other) error`
  - HTTP: `list-relations`・`list-relation-candidates`・`add-relation`・`remove-relation`(骨組みの表のとおり。本文の型は `RelationInput`、行の型は `RelationRow`・`RelationCandidate`)、`Item.blocked`

- [ ] **Step 1: item の関連を読む失敗するテストを書く**

`internal/domain/item/relations_test.go`(新規):

```go
package item_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

func TestFromLogReadsRelations(t *testing.T) {
	it := item.Item{ID: "A", Place: item.Place{Workspace: "W", Project: "P", Number: 1}, Kind: item.Task, Rank: "V", Title: "a"}
	meta := change.Meta{Op: "o", At: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC), Actor: change.User("taro")}
	n := 0
	newID := func() change.ID { n++; return change.ID(fmt.Sprintf("R%02d", n)) }
	log := change.Stamp(nil, []change.Draft{item.Create(it)}, meta, newID)
	log = append(log, change.Stamp(log, []change.Draft{
		change.NewAdd("relations", item.Relation{Type: "blocks", To: "B"}),
		change.NewAdd("relations", item.Relation{Type: "relates", To: "C"}),
		change.NewAdd("relations", map[string]any{"type": "blocks", "to": "bad id!"}), // 内部 ID でない相手は読み飛ばす
		change.NewRemove("relations", item.Relation{Type: "relates", To: "C"}),
	}, meta, newID)...)
	got, ok := item.FromLog(log)
	if !ok {
		t.Fatal("no item")
	}
	if want := []item.Relation{{Type: "blocks", To: "B"}}; !slices.Equal(got.Relations, want) {
		t.Errorf("Relations = %+v, want %+v", got.Relations, want)
	}
	if got.Blocked {
		t.Error("Blocked は保存しない値なので、FromLog では false")
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/item/ -run TestFromLogReadsRelations`
Expected: FAIL(`undefined: item.Relation`)

- [ ] **Step 3: item に関連を足す**

`internal/domain/item/item.go` の `Stamp` の下に足す。

```go
// Relation は、関連の 1 つ(保存形式「アイテムの例」の relations の値)。持つのは足した側のアイテムで、
// To が相手の内部 ID。Type は blocks・causes・duplicates・relates・clones で、意味は domain/relation が決める。
type Relation struct {
	Type string `json:"type"`
	To   ID     `json:"to"`
}
```

`Item` の `Watchers` の行の下に足す。

```go
	// Relations は、このアイテムが持つ関連(足した側)。相手の側の逆向きの表記は relation.Rows が作る(A4a-10)。
	Relations []Relation
	// Blocked は、ブロック中か(A4a-13)。保存しない派生の値で、app/items の Get・List が埋める。
	Blocked bool
```

`FromLog` の `watchers` を読む `for` の下に足す。

```go
	for _, raw := range s.Lists["relations"] {
		var r Relation
		if json.Unmarshal(raw, &r) == nil && r.Type != "" && change.ValidID(r.To) {
			it.Relations = append(it.Relations, r)
		}
	}
```

`internal/domain/item/values.go` の `multi` を変える。

```go
	lists := map[string]bool{"watchers": true, "systems": true, "relations": true}
```

Run: `go test ./internal/domain/item/` → PASS

- [ ] **Step 4: `Settings.Finished` の失敗するテストを書く**

`internal/domain/workspace/finished_test.go`(新規):

```go
package workspace_test

import (
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestFinished(t *testing.T) {
	n := 0
	st := workspace.DefaultSettings(func() change.ID { n++; return change.ID(fmt.Sprintf("V%02d", n)) })
	// 振る順: Todo V01、InProgress V02、Done V03、Pending V04、Cancel V05。
	for id, want := range map[change.ID]bool{"V01": false, "V02": false, "V03": true, "V04": false, "V05": true, "V99": false, "": false} {
		if got := st.Finished(id); got != want {
			t.Errorf("Finished(%q) = %v, want %v", id, got, want)
		}
	}
}
```

Run: `go test ./internal/domain/workspace/ -run TestFinished`
Expected: FAIL(`st.Finished undefined`)

`internal/domain/workspace/finished.go`(新規):

```go
package workspace

import "github.com/kukv/hedgehug/internal/domain/change"

// Finished は、ステータス id が完了か中止の分類か(「ブロック中」の判定。02「上から順の構成」)。
// 一覧にない値と空は false。
func (s Settings) Finished(id change.ID) bool {
	for _, v := range s.Statuses {
		if v.ID == id {
			return v.Category == CategoryDone || v.Category == CategoryCanceled
		}
	}
	return false
}
```

Run: `go test ./internal/domain/workspace/ -run TestFinished` → PASS

- [ ] **Step 5: relation の失敗するテストを書く**

`internal/domain/relation/relation_test.go`:

```go
package relation_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
)

func rel(typ string, to item.ID) item.Relation { return item.Relation{Type: typ, To: to} }

func TestParseAndLabelOf(t *testing.T) {
	cases := []struct {
		label   relation.Label
		typ     string
		reverse bool
	}{
		{relation.Blocks, "blocks", false}, {relation.BlockedBy, "blocks", true},
		{relation.Causes, "causes", false}, {relation.CausedBy, "causes", true},
		{relation.Duplicates, "duplicates", false}, {relation.DuplicatedBy, "duplicates", true},
		{relation.RelatesTo, "relates", false},
		{relation.Clones, "clones", false}, {relation.ClonedBy, "clones", true},
	}
	for _, c := range cases {
		typ, reverse, ok := relation.Parse(c.label)
		if !ok || typ != c.typ || reverse != c.reverse {
			t.Errorf("Parse(%q) = %q, %v, %v", c.label, typ, reverse, ok)
		}
		if back, ok := relation.LabelOf(c.typ, c.reverse); !ok || back != c.label {
			t.Errorf("LabelOf(%q, %v) = %q, %v", c.typ, c.reverse, back, ok)
		}
	}
	if _, _, ok := relation.Parse("friends with"); ok {
		t.Error("知らない表記を通した")
	}
	if l, ok := relation.LabelOf("relates", true); !ok || l != relation.RelatesTo {
		t.Errorf("relates は向きがなく、持たれる側でも relates to: %q, %v", l, ok)
	}
	if _, ok := relation.LabelOf("friends", false); ok {
		t.Error("知らない種類を通した")
	}
}

func TestRowsShowBothSidesInLabelOrder(t *testing.T) {
	all := []item.Item{
		{ID: "A", Relations: []item.Relation{rel("blocks", "B"), rel("relates", "C"), rel("clones", "D")}},
		{ID: "B"},
		{ID: "C", Relations: []item.Relation{rel("relates", "A")}}, // 両側が持っても 1 行
		{ID: "D"},
		{ID: "E", Relations: []item.Relation{rel("blocks", "A"), rel("friends", "A")}}, // 知らない種類は読み飛ばす
		{ID: "F", Relations: []item.Relation{rel("blocks", "GONE")}},                    // 相手がいない関連は出さない
	}
	want := []relation.Row{
		{Label: relation.Blocks, Other: "B"},
		{Label: relation.BlockedBy, Other: "E"},
		{Label: relation.RelatesTo, Other: "C"},
		{Label: relation.Clones, Other: "D"},
	}
	if got := relation.Rows("A", all); !slices.Equal(got, want) {
		t.Errorf("Rows(A) = %+v, want %+v", got, want)
	}
	if got := relation.Rows("B", all); !slices.Equal(got, []relation.Row{{Label: relation.BlockedBy, Other: "A"}}) {
		t.Errorf("Rows(B) = %+v", got)
	}
	if got := relation.Rows("D", all); !slices.Equal(got, []relation.Row{{Label: relation.ClonedBy, Other: "A"}}) {
		t.Errorf("Rows(D) = %+v", got)
	}
	if got := relation.Rows("F", all); len(got) != 0 {
		t.Errorf("Rows(F) = %+v, want none", got)
	}
}

func TestHasIgnoresDirection(t *testing.T) {
	all := []item.Item{{ID: "A", Relations: []item.Relation{rel("blocks", "B")}}, {ID: "B"}}
	if !relation.Has("A", "B", "blocks", all) || !relation.Has("B", "A", "blocks", all) {
		t.Error("向きが逆でも同じ種類は 1 つと数える")
	}
	if relation.Has("A", "B", "causes", all) {
		t.Error("種類が違えば別")
	}
	if relation.Has("A", "C", "blocks", all) {
		t.Error("相手が違えば別")
	}
}

func TestWouldCycle(t *testing.T) {
	all := []item.Item{
		{ID: "A", Relations: []item.Relation{rel("blocks", "B")}},
		{ID: "B", Relations: []item.Relation{rel("blocks", "C"), rel("causes", "A")}},
		{ID: "C"},
	}
	cases := []struct {
		from, to item.ID
		want     bool
	}{
		{"C", "A", true},  // A→B→C に C→A を足すと輪
		{"A", "C", false}, // 近道を足すだけ
		{"B", "A", true},
		{"C", "B", true},
		{"A", "A", true}, // 自分自身
		{"C", "D", false},
	}
	for _, c := range cases {
		if got := relation.WouldCycle(c.from, c.to, all); got != c.want {
			t.Errorf("WouldCycle(%s, %s) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestBlocked(t *testing.T) {
	all := []item.Item{
		{ID: "A", Relations: []item.Relation{rel("blocks", "B"), rel("causes", "D")}},
		{ID: "C", Relations: []item.Relation{rel("blocks", "B"), rel("blocks", "E")}},
		{ID: "B"}, {ID: "D"}, {ID: "E"},
	}
	closed := func(it item.Item) bool { return it.ID == "C" } // C は完了
	got := relation.BlockedSet(all, closed)
	if !got["B"] || got["D"] || got["E"] || got["A"] {
		t.Errorf("BlockedSet = %v: B は A がまだ終わっていないのでブロック中、E は C が終わっているので外れ、causes は関係しない", got)
	}
	if relation.Blocked("E", all, closed) || !relation.Blocked("B", all, closed) {
		t.Error("Blocked は BlockedSet と同じ答え")
	}
	if got := relation.BlockedSet(all, func(item.Item) bool { return true }); len(got) != 0 {
		t.Errorf("ブロッカーがすべて終わっていれば誰もブロック中でない: %v", got)
	}
}
```

- [ ] **Step 6: 失敗を確かめる**

Run: `go test ./internal/domain/relation/`
Expected: FAIL(パッケージがない: `no required module provides package` ではなく、`no non-test Go files` / `undefined: relation.Blocks`)

- [ ] **Step 7: relation を実装する**

`internal/domain/relation/relation.go`:

```go
// Package relation は、アイテムの関連(02「関連」)の規則を持つ: 9 つの表記、一覧、循環、ブロック中。
// 関連はワークスペースをまたぐので、呼ぶ側が Store のすべてのアイテムを渡す。
package relation

import (
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/item"
)

// Label は、画面の表記。
type Label string

const (
	Blocks       Label = "blocks"
	BlockedBy    Label = "blocked by"
	Causes       Label = "causes"
	CausedBy     Label = "caused by"
	Duplicates   Label = "duplicates"
	DuplicatedBy Label = "duplicated by"
	RelatesTo    Label = "relates to"
	Clones       Label = "clones"
	ClonedBy     Label = "cloned by"
)

// Labels は、一覧の見出しの順(02「関連」の「一覧」)。
var Labels = []Label{Blocks, BlockedBy, Causes, CausedBy, Duplicates, DuplicatedBy, RelatesTo, Clones, ClonedBy}

// TypeBlocks は、循環の検査と「ブロック中」に使う種類。
const TypeBlocks = "blocks"

type kind struct {
	typ     string
	reverse bool // 持たれる側(相手が持つ)から見た表記
}

var kinds = map[Label]kind{
	Blocks: {"blocks", false}, BlockedBy: {"blocks", true},
	Causes: {"causes", false}, CausedBy: {"causes", true},
	Duplicates: {"duplicates", false}, DuplicatedBy: {"duplicates", true},
	RelatesTo: {"relates", false},
	Clones:    {"clones", false}, ClonedBy: {"clones", true},
}

// Parse は、表記を、保存する種類と、自分が持たれる側(逆向きの表記)かに分ける。知らない表記は ok=false。
func Parse(l Label) (typ string, reverse bool, ok bool) {
	k, ok := kinds[l]
	return k.typ, k.reverse, ok
}

// LabelOf は、保存した種類を、持つ側(reverse=false)か持たれる側(true)から見た表記にする。
// relates には向きがなく、どちらからも relates to。知らない種類は ok=false。
func LabelOf(typ string, reverse bool) (Label, bool) {
	if typ == "relates" {
		return RelatesTo, true
	}
	for l, k := range kinds {
		if k.typ == typ && k.reverse == reverse {
			return l, true
		}
	}
	return "", false
}

// Row は、self から見た関連の 1 行。
type Row struct {
	Label Label
	Other item.ID
}

// Rows は、self の関連を一覧の順(表記の順、同じ表記では相手の内部 ID の順)で返す。self が持つ関連は自分の側の
// 表記、self を相手にした関連は逆向きの表記で出す。知らない種類と、相手が all にいない関連は出さない。
func Rows(self item.ID, all []item.Item) []Row {
	exists := make(map[item.ID]bool, len(all))
	for _, it := range all {
		exists[it.ID] = true
	}
	seen := map[Row]bool{}
	var rows []Row
	add := func(l Label, ok bool, other item.ID) {
		r := Row{Label: l, Other: other}
		if ok && exists[other] && !seen[r] {
			seen[r] = true
			rows = append(rows, r)
		}
	}
	for _, it := range all {
		for _, rel := range it.Relations {
			switch self {
			case it.ID:
				l, ok := LabelOf(rel.Type, false)
				add(l, ok, rel.To)
			case rel.To:
				l, ok := LabelOf(rel.Type, true)
				add(l, ok, it.ID)
			}
		}
	}
	slices.SortStableFunc(rows, func(a, b Row) int {
		if c := slices.Index(Labels, a.Label) - slices.Index(Labels, b.Label); c != 0 {
			return c
		}
		return strings.Compare(string(a.Other), string(b.Other))
	})
	return rows
}

// Has は、a と b のあいだに種類 typ の関連があるか。向きは問わない(02「関連」の「追加」。種類ごとに 1 つまで)。
func Has(a, b item.ID, typ string, all []item.Item) bool {
	for _, it := range all {
		for _, rel := range it.Relations {
			forward := it.ID == a && rel.To == b
			backward := it.ID == b && rel.To == a
			if rel.Type == typ && (forward || backward) {
				return true
			}
		}
	}
	return false
}

// WouldCycle は、from が to をブロックする関連を足すと、blocks の輪になるか(to から blocks をたどって from に
// 着くか)。from と to が同じなら輪。
func WouldCycle(from, to item.ID, all []item.Item) bool {
	next := map[item.ID][]item.ID{}
	for _, it := range all {
		for _, rel := range it.Relations {
			if rel.Type == TypeBlocks {
				next[it.ID] = append(next[it.ID], rel.To)
			}
		}
	}
	seen := map[item.ID]bool{}
	stack := []item.ID{to}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if cur == from {
			return true
		}
		if !seen[cur] {
			seen[cur] = true
			stack = append(stack, next[cur]...)
		}
	}
	return false
}

// BlockedSet は、ブロック中のアイテムの内部 ID。ブロックしている相手のうち、closed(完了か中止)でないものが
// 1 つでもあるとき(02「上から順の構成」)。削除済みを除くのは呼ぶ側(A4b)。
func BlockedSet(all []item.Item, closed func(item.Item) bool) map[item.ID]bool {
	out := map[item.ID]bool{}
	for _, it := range all {
		if closed(it) {
			continue
		}
		for _, rel := range it.Relations {
			if rel.Type == TypeBlocks {
				out[rel.To] = true
			}
		}
	}
	return out
}

// Blocked は、self がブロック中か。
func Blocked(self item.ID, all []item.Item, closed func(item.Item) bool) bool {
	return BlockedSet(all, closed)[self]
}
```

Run: `go test ./internal/domain/relation/ ./internal/domain/item/ ./internal/domain/workspace/` → PASS

- [ ] **Step 8: コミット(domain)**

```bash
git add internal/domain/item internal/domain/workspace/finished.go internal/domain/workspace/finished_test.go internal/domain/relation
git commit -m "feat(domain): 関連の表記・一覧・循環・ブロック中の規則を足す(A4a-10〜13)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 9: app/relations の補助と一覧・候補の失敗するテストを書く**

`internal/app/relations/relations_test.go`(新規。補助だけ):

```go
package relations_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/relations"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const store port.StoreID = "S1"

// サンプルのほかに足すもの: HH-3(Task、ステータス Done)、HH-4(HH-2 の下の Step)、
// ワークスペース「Mobile」(MB)の MB-1(Epic)。
const (
	wsMobile workspace.ID = "01TESTWS00000000000000002"
	prMobile project.ID   = "01TESTPR00000000000000002"
	itDone   item.ID      = "01TESTIT00000000000000003"
	itStep   item.ID      = "01TESTIT00000000000000004"
	itMobile item.ID      = "01TESTIT00000000000000005"
)

// recorder は、Write に渡したアクティビティの 1 行を覚える。
type recorder struct {
	*apptest.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func (r *recorder) last() string {
	if len(r.summaries) == 0 {
		return ""
	}
	return r.summaries[len(r.summaries)-1]
}

func setup(t *testing.T) (relations.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	if err := apptest.Sample(t.Context(), s, store); err != nil {
		t.Fatal(err)
	}
	seed(t, s)
	rec := &recorder{Stores: s}
	return relations.Service{Stores: rec, Identity: apptest.Identity{Login: "hanako"}}, rec
}

func seed(t *testing.T, s port.Stores) {
	t.Helper()
	n := 0
	valueIDs := func() change.ID { n++; return change.ID(fmt.Sprintf("01TESTVAL%016d", n)) }
	epic, task := apptest.SampleEpic, apptest.SampleTask
	mk := func(id item.ID, number int, kind item.Kind, parent *item.ID, title string, status change.ID) item.Item {
		return item.Item{
			ID: id, Kind: kind, Rank: "W", Title: title, Status: status,
			Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: parent, Number: number},
		}
	}
	done := mk(itDone, 3, item.Task, &epic, "決済", apptest.SampleStatusDone)
	step := mk(itStep, 4, item.Step, &task, "設計", apptest.SampleStatusTodo)
	mobile := item.Item{ID: itMobile, Kind: item.Epic, Rank: "V", Title: "iOS ログイン", Place: item.Place{Workspace: wsMobile, Project: prMobile, Number: 1}}
	ws := workspace.Workspace{ID: wsMobile, Name: "Mobile", Prefix: "MB"}
	pr := project.Project{ID: prMobile, Place: project.Place{Workspace: wsMobile, Number: 1}, Rank: "V", Title: "アプリ", Status: project.Todo}
	d, err := workspace.CreateWithSettings(ws, workspace.DefaultSettings(valueIDs))
	if err != nil {
		t.Fatal(err)
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "テストデータを作成"}
	err = s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(wsMobile, d),
			tx.WriteProjects().Append(prMobile, project.Create(pr)),
			tx.WriteItems().Append(itDone, item.Create(done)),
			tx.WriteItems().Append(itStep, item.Create(step)),
			tx.WriteItems().Append(itMobile, item.Create(mobile)),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// addRaw は、操作を通さずに、owner のログに関連を足す(読むテストの前提を作る)。
func addRaw(t *testing.T, s port.Stores, owner item.ID, typ string, to item.ID) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "関連を足す"}
	err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(owner, change.NewAdd("relations", item.Relation{Type: typ, To: to}))
	})
	if err != nil {
		t.Fatal(err)
	}
}

func relationsOf(t *testing.T, s port.Stores, id item.ID) []item.Relation {
	t.Helper()
	var rels []item.Relation
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		it, err := r.Items().Get(id)
		rels = it.Relations
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return rels
}

func logLen(t *testing.T, s port.Stores, id item.ID) int {
	t.Helper()
	n := 0
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		n = len(log)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}
```

`internal/app/relations/list_test.go`(新規):

```go
package relations_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/relations"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
)

func TestListShowsBothSidesAcrossWorkspaces(t *testing.T) {
	svc, rec := setup(t)
	addRaw(t, rec, apptest.SampleEpic, "blocks", apptest.SampleTask)
	addRaw(t, rec, itDone, "blocks", apptest.SampleEpic)
	addRaw(t, rec, apptest.SampleEpic, "relates", itMobile)
	addRaw(t, rec, apptest.SampleEpic, "clones", itDone)
	rows, err := svc.List(t.Context(), store, apptest.SampleEpic)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, fmt.Sprintf("%s %s", r.Label, r.Display))
	}
	want := []string{"blocks HH-2", "blocked by HH-3", "relates to MB-1", "clones HH-3"}
	if !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if rows[0].WorkspaceName != "" || rows[2].WorkspaceName != "Mobile" {
		t.Errorf("別のワークスペースの相手にだけ名前を添える: %q, %q", rows[0].WorkspaceName, rows[2].WorkspaceName)
	}
	if !rows[1].HasStatus || rows[1].Status.Name != "Done" || rows[2].HasStatus {
		t.Errorf("相手のステータス: %+v, %+v", rows[1].Status, rows[2].HasStatus)
	}
	if rows[0].Item.Title != "ログイン画面を作る" {
		t.Errorf("Item = %+v", rows[0].Item)
	}
}

func TestListOfNothingAndOfMissing(t *testing.T) {
	svc, _ := setup(t)
	rows, err := svc.List(t.Context(), store, apptest.SampleEpic)
	if err != nil || len(rows) != 0 {
		t.Errorf("関連がなければ空: %+v, %v", rows, err)
	}
	if _, err := svc.List(t.Context(), store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("ないアイテム: %v", err)
	}
}

func shown(cs []relations.Candidate) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Display+"|"+c.DisabledReason)
	}
	return out
}

func TestCandidates(t *testing.T) {
	svc, rec := setup(t)
	addRaw(t, rec, apptest.SampleEpic, "blocks", apptest.SampleTask)
	cases := []struct {
		name  string
		self  item.ID
		label relation.Label
		q     string
		want  []string
	}{
		// 自分(HH-2)と Step(HH-4)は出ない。HH-1 は、向きが逆でも blocks があるので選べない。
		{"ID の前方一致", apptest.SampleTask, relation.Blocks, "hh", []string{"HH-1|すでに blocks の関連があります", "HH-3|"}},
		{"大文字小文字を区別しない", apptest.SampleTask, relation.Blocks, "HH-3", []string{"HH-3|"}},
		{"タイトルの部分一致", apptest.SampleTask, relation.Blocks, "決", []string{"HH-3|"}},
		{"逆向きの表記でも同じ種類は 1 つ", apptest.SampleTask, relation.BlockedBy, "HH-1", []string{"HH-1|すでに blocked by の関連があります"}},
		{"種類が違えば選べる", apptest.SampleTask, relation.Causes, "HH-1", []string{"HH-1|"}},
		{"空の検索語は出さない", apptest.SampleTask, relation.Blocks, "  ", nil},
		{"Step も自分の側からは関連を持てるが、候補には Step を出さない", apptest.SampleEpic, relation.RelatesTo, "設計", nil},
	}
	for _, c := range cases {
		got, err := svc.Candidates(t.Context(), store, c.self, c.label, c.q)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !slices.Equal(shown(got), c.want) {
			t.Errorf("%s: candidates = %v, want %v", c.name, shown(got), c.want)
		}
	}
	got, err := svc.Candidates(t.Context(), store, apptest.SampleEpic, relation.RelatesTo, "ios")
	if err != nil || len(got) != 1 || got[0].Display != "MB-1" || got[0].WorkspaceName != "Mobile" || got[0].Item.ID != itMobile {
		t.Errorf("別のワークスペースの候補にはワークスペース名を添える: %+v, %v", got, err)
	}
	if _, err := svc.Candidates(t.Context(), store, apptest.SampleTask, "friends with", "HH"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("知らない表記: %v", err)
	}
}

func TestCandidatesDisableACycle(t *testing.T) {
	svc, rec := setup(t)
	addRaw(t, rec, apptest.SampleEpic, "blocks", apptest.SampleTask) // HH-1 → HH-2
	addRaw(t, rec, apptest.SampleTask, "blocks", itDone)             // HH-2 → HH-3
	// HH-3 が HH-1 をブロックすると、HH-1 → HH-2 → HH-3 → HH-1 の輪になる。
	got, err := svc.Candidates(t.Context(), store, itDone, relation.Blocks, "HH-1")
	if err != nil || !slices.Equal(shown(got), []string{"HH-1|循環になります"}) {
		t.Errorf("blocks: %v, %v", shown(got), err)
	}
	// HH-1 が HH-3 をブロックするのは近道を足すだけで、輪にならない。
	got, err = svc.Candidates(t.Context(), store, itDone, relation.BlockedBy, "HH-1")
	if err != nil || !slices.Equal(shown(got), []string{"HH-1|"}) {
		t.Errorf("blocked by: %v, %v", shown(got), err)
	}
	// ほかの種類は循環を検査しない。
	got, err = svc.Candidates(t.Context(), store, itDone, relation.Causes, "HH-1")
	if err != nil || !slices.Equal(shown(got), []string{"HH-1|"}) {
		t.Errorf("causes: %v, %v", shown(got), err)
	}
}

func TestCandidatesStopAtTwenty(t *testing.T) {
	svc, rec := setup(t)
	err := rec.Write(t.Context(), store, port.Operation{Actor: change.User("taro"), Summary: "まとめて作る"}, func(tx port.Tx) error {
		epic := apptest.SampleEpic
		for i := range 25 {
			it := item.Item{
				ID: item.ID(fmt.Sprintf("01TESTBK%018d", i)), Kind: item.Task, Rank: "W", Title: fmt.Sprintf("まとめ %d", i),
				Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &epic, Number: 10 + i},
			}
			if err := tx.WriteItems().Append(it.ID, item.Create(it)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Candidates(t.Context(), store, apptest.SampleTask, relation.RelatesTo, "まとめ")
	if err != nil || len(got) != 20 || got[0].Display != "HH-10" || got[19].Display != "HH-29" {
		t.Errorf("20 件で打ち切り、番号の順: %d 件, %v", len(got), err)
	}
}
```

- [ ] **Step 10: 失敗を確かめる**

Run: `go test ./internal/app/relations/`
Expected: FAIL(`no non-test Go files` または `undefined: relations.Service`)

- [ ] **Step 11: 読む処理を実装する**

`internal/app/relations/service.go`:

```go
// Package relations は、アイテムの関連(02「関連」)の操作を持つ。1 ファイル 1 操作。
package relations

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// Service は、関連の操作が使う口。
type Service struct {
	Stores   port.Stores
	Identity port.Identity
}

// summary は、アクティビティの 1 行を保存の前に決める。2 つのアイテムの画面の ID(HH-1)が要る。
// どちらかがなければ port.ErrNotFound。
func (s Service) summary(ctx context.Context, store port.StoreID, self, other item.ID, text func(a, b string) string) (string, error) {
	var out string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		snap, err := readSnapshot(r)
		if err != nil {
			return err
		}
		a, okA := snap.byID[self]
		b, okB := snap.byID[other]
		if !okA || !okB {
			return port.ErrNotFound
		}
		out = text(snap.display(a), snap.display(b))
		return nil
	})
	return out, err
}
```

`internal/app/relations/snapshot.go`:

```go
package relations

import (
	"errors"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// snapshot は、Store のすべてのアイテムと、表示に要るワークスペースの情報(A4a-11)。
// ワークスペース → プロジェクト → アイテムの順に、既存の口で読む。
type snapshot struct {
	order    []workspace.ID // ワークスペースの並び
	names    map[workspace.ID]string
	prefixes map[workspace.ID]string
	settings map[workspace.ID]workspace.Settings
	all      []item.Item
	byID     map[item.ID]item.Item
}

func readSnapshot(r port.Reader) (snapshot, error) {
	wss, err := r.Workspaces().List()
	if err != nil {
		return snapshot{}, err
	}
	s := snapshot{
		names: map[workspace.ID]string{}, prefixes: map[workspace.ID]string{},
		settings: map[workspace.ID]workspace.Settings{}, byID: map[item.ID]item.Item{},
	}
	for _, w := range wss {
		st, err := r.Workspaces().Settings(w.ID)
		if err != nil && !errors.Is(err, port.ErrNotFound) {
			return snapshot{}, err
		}
		s.order = append(s.order, w.ID)
		s.names[w.ID], s.prefixes[w.ID], s.settings[w.ID] = w.Name, w.Prefix, st
		ps, err := r.Projects().List(w.ID)
		if err != nil {
			return snapshot{}, err
		}
		for _, p := range ps {
			its, err := r.Items().List(p.ID)
			if err != nil {
				return snapshot{}, err
			}
			for _, it := range its {
				s.byID[it.ID] = it
			}
			s.all = append(s.all, its...)
		}
	}
	return s, nil
}

// display は、画面の ID(HH-12)。
func (s snapshot) display(it item.Item) string { return it.DisplayID(s.prefixes[it.Place.Workspace]) }

// workspaceNameFor は、other が self と別のワークスペースにあるときだけ、そのワークスペース名を返す。
func (s snapshot) workspaceNameFor(self, other item.Item) string {
	if other.Place.Workspace == self.Place.Workspace {
		return ""
	}
	return s.names[other.Place.Workspace]
}

// status は、アイテムのステータスの値(名前と色)。値がなければ ok=false。
func (s snapshot) status(it item.Item) (workspace.Value, bool) {
	for _, v := range s.settings[it.Place.Workspace].Statuses {
		if v.ID == it.Status {
			return v, true
		}
	}
	return workspace.Value{}, false
}

// reason は、self が other と label の関連を足せない理由。足せるなら空(02「関連」の「追加」)。
// 同じ種類の関連は、向きが逆でも 1 つまで。blocks は、足すと循環になるものも断る。
func (s snapshot) reason(self, other item.ID, label relation.Label, typ string, reverse bool) string {
	if relation.Has(self, other, typ, s.all) {
		return fmt.Sprintf("すでに %s の関連があります", label)
	}
	if typ == relation.TypeBlocks {
		from, to := self, other
		if reverse {
			from, to = other, self
		}
		if relation.WouldCycle(from, to, s.all) {
			return "循環になります"
		}
	}
	return ""
}

// holder は、self から見た label の関連を、実際に持っているアイテムと、その値。持っていなければ ok=false。
// blocked by などの逆向きは相手が持ち、relates to だけは、どちらが持っていてもよい。
func (s snapshot) holder(self, other item.ID, typ string, reverse bool) (item.ID, item.Relation, bool) {
	type pair struct{ owner, to item.ID }
	cands := []pair{{self, other}}
	switch {
	case reverse:
		cands = []pair{{other, self}}
	case typ == "relates":
		cands = append(cands, pair{other, self})
	}
	for _, c := range cands {
		rel := item.Relation{Type: typ, To: c.to}
		if it, ok := s.byID[c.owner]; ok && slices.Contains(it.Relations, rel) {
			return c.owner, rel, true
		}
	}
	return "", item.Relation{}, false
}
```

`internal/app/relations/list.go`:

```go
package relations

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Row は、関連の一覧の 1 行。
type Row struct {
	Label         relation.Label
	Item          item.Item // 相手
	Display       string    // 相手の画面の ID
	WorkspaceName string    // 相手が別のワークスペースのときだけ
	Status        workspace.Value
	HasStatus     bool
}

// List は、self の関連を、一覧の順(表記の順)で返す。逆向きの表記を含む。self がなければ port.ErrNotFound。
func (s Service) List(ctx context.Context, store port.StoreID, self item.ID) ([]Row, error) {
	var out []Row
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		snap, err := readSnapshot(r)
		if err != nil {
			return err
		}
		me, ok := snap.byID[self]
		if !ok {
			return port.ErrNotFound
		}
		for _, row := range relation.Rows(self, snap.all) {
			other := snap.byID[row.Other]
			v := Row{Label: row.Label, Item: other, Display: snap.display(other), WorkspaceName: snap.workspaceNameFor(me, other)}
			v.Status, v.HasStatus = snap.status(other)
			out = append(out, v)
		}
		return nil
	})
	return out, err
}
```

`internal/app/relations/candidates.go`:

```go
package relations

import (
	"context"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
)

// maxCandidates は、候補の件数の上限(A4a-12。仕様にないので、画面に収まる数)。
const maxCandidates = 20

// Candidate は、相手の候補。DisabledReason が空でなければ選べない。
type Candidate struct {
	Item           item.Item
	Display        string
	WorkspaceName  string // 別のワークスペースの候補にだけ
	DisabledReason string
}

// Candidates は、self が label の関連を足せる相手の候補を返す(02「関連」の「追加」)。自分と Step を除き、
// ID(前方一致)かタイトル(部分一致)で絞る。どちらも大文字小文字を区別しない。ワークスペースの並び → 番号の順で
// 20 件まで。検索語が空なら返さない。知らない表記か self がなければ port.ErrNotFound。
func (s Service) Candidates(ctx context.Context, store port.StoreID, self item.ID, label relation.Label, query string) ([]Candidate, error) {
	typ, reverse, ok := relation.Parse(label)
	if !ok {
		return nil, port.ErrNotFound
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var out []Candidate
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		snap, err := readSnapshot(r)
		if err != nil {
			return err
		}
		me, ok := snap.byID[self]
		if !ok {
			return port.ErrNotFound
		}
		if q == "" {
			return nil
		}
		its := slices.Clone(snap.all)
		slices.SortStableFunc(its, func(a, b item.Item) int {
			if c := slices.Index(snap.order, a.Place.Workspace) - slices.Index(snap.order, b.Place.Workspace); c != 0 {
				return c
			}
			return a.Place.Number - b.Place.Number
		})
		for _, it := range its {
			display := snap.display(it)
			matches := strings.HasPrefix(strings.ToLower(display), q) || strings.Contains(strings.ToLower(it.Title), q)
			if it.ID == self || it.Kind == item.Step || !matches {
				continue
			}
			out = append(out, Candidate{
				Item: it, Display: display, WorkspaceName: snap.workspaceNameFor(me, it),
				DisabledReason: snap.reason(self, it.ID, label, typ, reverse),
			})
			if len(out) == maxCandidates {
				break
			}
		}
		return nil
	})
	return out, err
}
```

(追加と解除は Step 13 で書く。このステップのテストは `Add`・`Remove` を使わないので通る。`edit_test.go` はまだない。)

Run: `go test ./internal/app/relations/` → PASS

- [ ] **Step 12: 追加・解除の失敗するテストを書く**

`internal/app/relations/edit_test.go`(新規):

```go
package relations_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
)

func TestAddWritesToTheSideThatHoldsTheRelation(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	// blocks: 自分の側に持つ。
	if err := svc.Add(ctx, store, apptest.SampleEpic, relation.Blocks, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if got := relationsOf(t, rec, apptest.SampleEpic); !slices.Equal(got, []item.Relation{{Type: "blocks", To: apptest.SampleTask}}) || rec.last() != "HH-1 blocks HH-2" {
		t.Errorf("blocks: %+v, %q", got, rec.last())
	}
	// blocked by: 相手がこのアイテムをブロックする関連を、相手のログに足す。
	if err := svc.Add(ctx, store, apptest.SampleTask, relation.BlockedBy, itDone); err != nil {
		t.Fatal(err)
	}
	if got := relationsOf(t, rec, itDone); !slices.Equal(got, []item.Relation{{Type: "blocks", To: apptest.SampleTask}}) || rec.last() != "HH-2 blocked by HH-3" {
		t.Errorf("blocked by: %+v, %q", got, rec.last())
	}
	if got := relationsOf(t, rec, apptest.SampleTask); len(got) != 0 {
		t.Errorf("blocked by は自分の側に書かない: %+v", got)
	}
	// relates to は足した側に置く。ワークスペースをまたぐ相手でもよい。
	if err := svc.Add(ctx, store, apptest.SampleEpic, relation.RelatesTo, itMobile); err != nil {
		t.Fatal(err)
	}
	if got := relationsOf(t, rec, apptest.SampleEpic); !slices.Contains(got, item.Relation{Type: "relates", To: itMobile}) || rec.last() != "HH-1 relates to MB-1" {
		t.Errorf("relates: %+v, %q", got, rec.last())
	}
	// Step も、Step 以外のアイテムとの関連を持てる。
	if err := svc.Add(ctx, store, itStep, relation.Causes, apptest.SampleEpic); err != nil {
		t.Errorf("Step から: %v", err)
	}
}

func TestAddRefuses(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	if err := svc.Add(ctx, store, apptest.SampleEpic, relation.Blocks, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := svc.Add(ctx, store, apptest.SampleTask, relation.Blocks, itDone); err != nil {
		t.Fatal(err)
	}
	before := logLen(t, rec, apptest.SampleEpic) + logLen(t, rec, apptest.SampleTask) + logLen(t, rec, itDone)
	cases := []struct {
		name  string
		self  item.ID
		label relation.Label
		other item.ID
		want  string
	}{
		{"向きが逆でも同じ種類は 1 つ", apptest.SampleTask, relation.BlockedBy, apptest.SampleEpic, "すでに blocked by の関連があります"},
		{"同じ向きの重複", apptest.SampleEpic, relation.Blocks, apptest.SampleTask, "すでに blocks の関連があります"},
		{"循環", itDone, relation.Blocks, apptest.SampleEpic, "循環になります"},
	}
	for _, c := range cases {
		err := svc.Add(ctx, store, c.self, c.label, c.other)
		if message(err) != c.want {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
	}
	for name, err := range map[string]error{
		"自分自身":   svc.Add(ctx, store, apptest.SampleEpic, relation.Causes, apptest.SampleEpic),
		"相手が Step": svc.Add(ctx, store, apptest.SampleEpic, relation.Causes, itStep),
		"ないアイテム": svc.Add(ctx, store, apptest.SampleEpic, relation.Causes, "01NOPE00000000000000000000"),
		"知らない表記": svc.Add(ctx, store, apptest.SampleEpic, "friends with", itDone),
	} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if after := logLen(t, rec, apptest.SampleEpic) + logLen(t, rec, apptest.SampleTask) + logLen(t, rec, itDone); after != before {
		t.Errorf("断ったときは何も保存しない: ログの行 %d → %d", before, after)
	}
}

func TestRemove(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	if err := svc.Add(ctx, store, apptest.SampleEpic, relation.Blocks, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	// 持たれる側の表記で外すと、持っている側のログに remove を書く。
	if err := svc.Remove(ctx, store, apptest.SampleTask, relation.BlockedBy, apptest.SampleEpic); err != nil {
		t.Fatal(err)
	}
	if got := relationsOf(t, rec, apptest.SampleEpic); len(got) != 0 || rec.last() != "HH-2 と HH-1 の blocked by を解除" {
		t.Errorf("remove: %+v, %q", got, rec.last())
	}
	// 持っていなければ何も保存しない。
	n := logLen(t, rec, apptest.SampleEpic) + logLen(t, rec, apptest.SampleTask)
	if err := svc.Remove(ctx, store, apptest.SampleTask, relation.BlockedBy, apptest.SampleEpic); err != nil {
		t.Errorf("もう外れている関連を外す: %v", err)
	}
	if after := logLen(t, rec, apptest.SampleEpic) + logLen(t, rec, apptest.SampleTask); after != n {
		t.Errorf("外れている関連を外しても何も保存しない: ログの行 %d → %d", n, after)
	}
	// 向きが違う表記では外れない(blocks の持ち主は逆)。
	if err := svc.Add(ctx, store, apptest.SampleEpic, relation.Blocks, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	n = logLen(t, rec, apptest.SampleEpic)
	if err := svc.Remove(ctx, store, apptest.SampleTask, relation.Blocks, apptest.SampleEpic); err != nil {
		t.Fatal(err)
	}
	if after := logLen(t, rec, apptest.SampleEpic); after != n || len(relationsOf(t, rec, apptest.SampleEpic)) != 1 {
		t.Errorf("HH-2 が blocks している関連はないので何も外れない: ログの行 %d → %d", n, after)
	}
	// relates to は、どちらの側が持っていても外れる。
	if err := svc.Add(ctx, store, apptest.SampleEpic, relation.RelatesTo, itMobile); err != nil {
		t.Fatal(err)
	}
	if err := svc.Remove(ctx, store, itMobile, relation.RelatesTo, apptest.SampleEpic); err != nil {
		t.Fatal(err)
	}
	if got := relationsOf(t, rec, apptest.SampleEpic); slices.Contains(got, item.Relation{Type: "relates", To: itMobile}) || rec.last() != "MB-1 と HH-1 の relates to を解除" {
		t.Errorf("relates の解除: %+v, %q", got, rec.last())
	}
	if err := svc.Remove(ctx, store, apptest.SampleEpic, relation.Blocks, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("ないアイテム: %v", err)
	}
}
```

- [ ] **Step 13: 失敗を確かめる**

Run: `go test ./internal/app/relations/ -run 'TestAdd|TestRemove'`
Expected: FAIL(`svc.Add undefined`、`svc.Remove undefined`)

- [ ] **Step 14: 追加と解除を実装する**

`internal/app/relations/add.go`:

```go
package relations

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/relation"
)

// Add は、self から見て、other とのあいだに label の関連を足す(02「関連」の「追加」)。たとえば self が
// 「blocked by HH-3」を足すと、HH-3 のログに「このアイテムをブロックする」関連を書く。アクティビティは
// 「{ID} {表記} {ID}」。
// 相手が自分・Step・ないアイテム、知らない表記なら port.ErrNotFound。同じ種類の関連がある(向きが逆でも)か、
// blocks で循環になるときは断る。条件は保存の中で読み直して確かめる。
func (s Service) Add(ctx context.Context, store port.StoreID, self item.ID, label relation.Label, other item.ID) error {
	typ, reverse, ok := relation.Parse(label)
	if !ok {
		return port.ErrNotFound
	}
	summary, err := s.summary(ctx, store, self, other, func(a, b string) string { return fmt.Sprintf("%s %s %s", a, label, b) })
	if err != nil {
		return err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: summary}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		snap, err := readSnapshot(tx)
		if err != nil {
			return err
		}
		o, okO := snap.byID[other]
		if _, okS := snap.byID[self]; !okS || !okO || self == other || o.Kind == item.Step {
			return port.ErrNotFound
		}
		if reason := snap.reason(self, other, label, typ, reverse); reason != "" {
			return refusal.New(reason)
		}
		owner, rel := self, item.Relation{Type: typ, To: other}
		if reverse {
			owner, rel = other, item.Relation{Type: typ, To: self}
		}
		return tx.WriteItems().Append(owner, change.NewAdd("relations", rel))
	})
}
```

`internal/app/relations/remove.go`:

```go
package relations

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
)

// Remove は、self から見た label の関連を外す(02「関連」の「解除」。確認なし)。関連を持っているほうのログに
// remove を書く。持っていなければ何も保存しない。アクティビティは「{ID} と {ID} の {表記} を解除」。
// どちらかのアイテムがないか、知らない表記なら port.ErrNotFound。
func (s Service) Remove(ctx context.Context, store port.StoreID, self item.ID, label relation.Label, other item.ID) error {
	typ, reverse, ok := relation.Parse(label)
	if !ok {
		return port.ErrNotFound
	}
	summary, err := s.summary(ctx, store, self, other, func(a, b string) string { return fmt.Sprintf("%s と %s の %s を解除", a, b, label) })
	if err != nil {
		return err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: summary}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		snap, err := readSnapshot(tx)
		if err != nil {
			return err
		}
		owner, rel, ok := snap.holder(self, other, typ, reverse)
		if !ok {
			return nil // 行のない Write は何も残さない
		}
		return tx.WriteItems().Append(owner, change.NewRemove("relations", rel))
	})
}
```

Run: `go test ./internal/app/relations/` → PASS
Run: `go vet ./internal/app/relations/ ./internal/domain/...` → 出力なし

- [ ] **Step 15: コミット(app/relations)**

```bash
git add internal/app/relations
git commit -m "feat(app): 関連の一覧・候補・追加・解除を足す(A4a-10〜12)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 16: 「ブロック中」の失敗するテストを書く**

`internal/app/items/blocked_test.go`(新規。`items_test.go` の `setup`・`store` を使う):

```go
package items_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

func writeLog(t *testing.T, s port.Stores, id item.ID, d change.Draft) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "テスト"}
	err := s.Write(t.Context(), store, op, func(tx port.Tx) error { return tx.WriteItems().Append(id, d) })
	if err != nil {
		t.Fatal(err)
	}
}

// ブロックしている相手が完了か中止でなければ、ブロック中(A4a-13)。Get・List・FindByNumber のどれでも同じ。
func TestBlockedFollowsTheBlockersStatus(t *testing.T) {
	svc := setup(t)
	ctx := t.Context()
	blocked := func() [3]bool {
		got, err := svc.Get(ctx, store, apptest.SampleTask)
		if err != nil {
			t.Fatal(err)
		}
		list, err := svc.List(ctx, store, apptest.SampleProject)
		if err != nil {
			t.Fatal(err)
		}
		found, err := svc.FindByNumber(ctx, store, apptest.SampleWorkspace, 2)
		if err != nil {
			t.Fatal(err)
		}
		return [3]bool{got.Blocked, list[1].Blocked, found.Blocked}
	}
	if blocked() != [3]bool{} {
		t.Fatalf("関連がなければブロック中でない: %v", blocked())
	}
	// HH-1(未着手)が HH-2 をブロックする。
	writeLog(t, svc.Stores, apptest.SampleEpic, change.NewAdd("relations", item.Relation{Type: "blocks", To: apptest.SampleTask}))
	if blocked() != [3]bool{true, true, true} {
		t.Errorf("未着手のブロッカーがいればブロック中: %v", blocked())
	}
	if list, _ := svc.List(ctx, store, apptest.SampleProject); list[0].Blocked {
		t.Error("ブロックしている側(HH-1)はブロック中でない")
	}
	// ほかの種類の関連は関係しない。
	writeLog(t, svc.Stores, apptest.SampleEpic, change.NewRemove("relations", item.Relation{Type: "blocks", To: apptest.SampleTask}))
	writeLog(t, svc.Stores, apptest.SampleEpic, change.NewAdd("relations", item.Relation{Type: "causes", To: apptest.SampleTask}))
	if blocked() != [3]bool{} {
		t.Errorf("causes ではブロック中にならない: %v", blocked())
	}
	// ブロッカーが完了になったら外れる。
	writeLog(t, svc.Stores, apptest.SampleEpic, change.NewAdd("relations", item.Relation{Type: "blocks", To: apptest.SampleTask}))
	writeLog(t, svc.Stores, apptest.SampleEpic, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	if blocked() != [3]bool{} {
		t.Errorf("完了のブロッカーでは外れる: %v", blocked())
	}
	writeLog(t, svc.Stores, apptest.SampleEpic, change.NewSet("status", apptest.SampleStatusDone, apptest.SampleStatusCancel))
	if blocked() != [3]bool{} {
		t.Errorf("中止のブロッカーでも外れる: %v", blocked())
	}
	writeLog(t, svc.Stores, apptest.SampleEpic, change.NewSet("status", apptest.SampleStatusCancel, apptest.SampleStatusInProgress))
	if blocked() != [3]bool{true, true, true} {
		t.Errorf("進行中なら再びブロック中: %v", blocked())
	}
}
```

- [ ] **Step 17: 失敗を確かめる**

Run: `go test ./internal/app/items/ -run TestBlockedFollowsTheBlockersStatus`
Expected: FAIL(`blocked() = [false false false]` が「未着手のブロッカーがいればブロック中」で出る)

- [ ] **Step 18: `Get`・`List`・`FindByNumber` が `Blocked` を埋める**

`internal/app/items/view.go` に足す(import に `"errors"` と `"github.com/kukv/hedgehug/internal/domain/relation"` を足す)。

```go
// blockedSet は、Store のブロック中のアイテムの内部 ID(A4a-13)。関連はワークスペースをまたぐので、
// ワークスペース → プロジェクト → アイテムの順にすべて読む。ステータスの分類は、そのアイテムのワークスペースの設定で決める。
func blockedSet(r port.Reader) (map[item.ID]bool, error) {
	wss, err := r.Workspaces().List()
	if err != nil {
		return nil, err
	}
	var all []item.Item
	finished := map[item.ID]bool{}
	for _, w := range wss {
		st, err := r.Workspaces().Settings(w.ID)
		if err != nil && !errors.Is(err, port.ErrNotFound) {
			return nil, err
		}
		ps, err := r.Projects().List(w.ID)
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			its, err := r.Items().List(p.ID)
			if err != nil {
				return nil, err
			}
			for _, it := range its {
				finished[it.ID] = st.Finished(it.Status)
			}
			all = append(all, its...)
		}
	}
	return relation.BlockedSet(all, func(it item.Item) bool { return finished[it.ID] }), nil
}
```

`internal/app/items/get.go` の `Get` を変える。

```go
func (s Service) Get(ctx context.Context, store port.StoreID, id item.ID) (item.Item, error) {
	var it item.Item
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		if it, err = r.Items().Get(id); err != nil {
			return err
		}
		blocked, err := blockedSet(r)
		it.Blocked = blocked[it.ID]
		return err
	})
	return it, err
}
```

`internal/app/items/list.go` の `List` を変える。

```go
func (s Service) List(ctx context.Context, store port.StoreID, p project.ID) ([]item.Item, error) {
	var out []item.Item
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := r.Projects().Get(p); err != nil {
			return err
		}
		var err error
		if out, err = r.Items().List(p); err != nil {
			return err
		}
		blocked, err := blockedSet(r)
		for i := range out {
			out[i].Blocked = blocked[out[i].ID]
		}
		return err
	})
	return out, err
}
```

`internal/app/items/numbers.go` の `FindByNumber` の `found = it` の直後、`return nil` の前を変える。

```go
				if it.Place.Workspace == ws && it.Place.Number == number {
					found = it
					blocked, err := blockedSet(r)
					found.Blocked = blocked[it.ID]
					return err
				}
```

Run: `go test ./internal/app/items/` → PASS(既存のテストも通ること)

- [ ] **Step 19: コミット(blocked)**

```bash
git add internal/app/items
git commit -m "feat(app): アイテムの Get・List・FindByNumber で「ブロック中」を埋める(A4a-13)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 20: API の失敗するテストを書く**

`internal/adapter/httpapi/notes/relations_test.go`(新規。`notes.Services` の他のフィールドは使わないので、ゼロ値のまま渡す):

```go
package notes_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/notes"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/relations"
)

func newRelationsAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	notes.Register(api, notes.Services{Relations: relations.Service{Stores: s, Identity: apptest.Identity{Login: "taro"}}})
	return api
}

func TestRelationsRoundTrip(t *testing.T) {
	api := newRelationsAPI(t)
	epic, task := string(apptest.SampleEpic), string(apptest.SampleTask)
	if res := api.Get("/api/stores/S1/items/" + epic + "/relations"); res.Code != http.StatusOK || strings.TrimSpace(res.Body.String()) != "[]" {
		t.Fatalf("empty: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items/"+epic+"/relations", map[string]any{"label": "blocks", "other": task}); res.Code != http.StatusNoContent {
		t.Fatalf("add: %d %s", res.Code, res.Body)
	}
	res := api.Get("/api/stores/S1/items/" + epic + "/relations")
	body := res.Body.String()
	for _, want := range []string{`"label":"blocks"`, `"display":"HH-2"`, `"title":"ログイン画面を作る"`, `"id":"` + task + `"`, `"statusName":"Todo"`, `"workspaceName":""`} {
		if !strings.Contains(body, want) {
			t.Errorf("epic relations lack %s: %s", want, body)
		}
	}
	if res := api.Get("/api/stores/S1/items/" + task + "/relations"); !strings.Contains(res.Body.String(), `"label":"blocked by"`) || !strings.Contains(res.Body.String(), `"display":"HH-1"`) {
		t.Errorf("task relations: %s", res.Body)
	}
	res = api.Post("/api/stores/S1/items/"+epic+"/relations", map[string]any{"label": "blocks", "other": task})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "すでに blocks の関連があります") {
		t.Errorf("duplicate: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items/"+task+"/relations/remove", map[string]any{"label": "blocked by", "other": epic}); res.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/S1/items/" + epic + "/relations"); strings.TrimSpace(res.Body.String()) != "[]" {
		t.Errorf("after remove: %s", res.Body)
	}
}

func TestRelationCandidatesAndNotFound(t *testing.T) {
	api := newRelationsAPI(t)
	epic, task := string(apptest.SampleEpic), string(apptest.SampleTask)
	res := api.Get("/api/stores/S1/items/" + task + "/relation-candidates?label=blocks&q=hh")
	body := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(body, `"display":"HH-1"`) || !strings.Contains(body, `"disabledReason":""`) || strings.Contains(body, `"display":"HH-2"`) {
		t.Errorf("candidates: %d %s", res.Code, body)
	}
	if res := api.Get("/api/stores/S1/items/" + task + "/relation-candidates?label=blocks&q="); strings.TrimSpace(res.Body.String()) != "[]" {
		t.Errorf("empty query: %s", res.Body)
	}
	if res := api.Get("/api/stores/S1/items/" + task + "/relation-candidates?label=friends&q=hh"); res.Code != http.StatusNotFound {
		t.Errorf("bad label: %d", res.Code)
	}
	if res := api.Post("/api/stores/S1/items/"+epic+"/relations", map[string]any{"label": "blocks", "other": "01NOPE00000000000000000000"}); res.Code != http.StatusNotFound {
		t.Errorf("no such item: %d", res.Code)
	}
}
```

`internal/adapter/httpapi/items/items_test.go` の `TestCreateGetList` の get の検査に `blocked` を足す(既存の `if res := api.Get(...Epic...)` の条件の `&&` の並びに `|| !strings.Contains(res.Body.String(), `"blocked":false`)` を足す)。

```go
	if res := api.Get("/api/stores/S1/items/" + string(apptest.SampleEpic)); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"parent":null`) || !strings.Contains(res.Body.String(), `"blocked":false`) {
```

- [ ] **Step 21: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/notes/ ./internal/adapter/httpapi/items/`
Expected: FAIL(`notes.Services` に `Relations` がない、`"blocked":false` がない)

- [ ] **Step 22: API を実装する**

`internal/adapter/httpapi/notes/relations.go`:

```go
package notes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	apprelations "github.com/kukv/hedgehug/internal/app/relations"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
)

// RelationRow は、関連の一覧の 1 行。
type RelationRow struct {
	Label         string `json:"label" doc:"表記(blocks・blocked by・causes・caused by・duplicates・duplicated by・relates to・clones・cloned by)"`
	ID            string `json:"id" doc:"相手の内部 ID"`
	Display       string `json:"display" doc:"相手の画面の ID(HH-3)"`
	Title         string `json:"title"`
	WorkspaceName string `json:"workspaceName" doc:"相手が別のワークスペースのときだけ。同じなら空"`
	StatusName    string `json:"statusName" doc:"相手のステータスの名前。値がなければ空"`
	StatusColor   string `json:"statusColor" doc:"ステータスの色の名前。値がなければ空"`
}

// RelationCandidate は、相手の候補。
type RelationCandidate struct {
	ID             string `json:"id" doc:"相手の内部 ID"`
	Display        string `json:"display"`
	Title          string `json:"title"`
	WorkspaceName  string `json:"workspaceName" doc:"別のワークスペースの候補だけ。同じなら空"`
	DisabledReason string `json:"disabledReason" doc:"選べない理由。選べるなら空"`
}

// RelationInput は、関連の追加・解除の本文。Label は、このアイテムから見た表記。
type RelationInput struct {
	Label string `json:"label"`
	Other string `json:"other" doc:"相手の内部 ID"`
}

type relationsPath struct {
	Store string `path:"store"`
	Item  string `path:"item"`
}

type candidatesInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Label string `query:"label"`
	Q     string `query:"q"`
}

type relationEditInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  RelationInput
}

type relationRowsOutput struct{ Body []RelationRow }

type relationCandidatesOutput struct{ Body []RelationCandidate }

func registerRelations(api huma.API, svc apprelations.Service) {
	huma.Register(api, huma.Operation{OperationID: "list-relations", Method: http.MethodGet, Path: "/api/stores/{store}/items/{item}/relations", Summary: "アイテムの関連を、表記の順で返す(逆向きの表記を含む)"},
		func(ctx context.Context, in *relationsPath) (*relationRowsOutput, error) {
			rows, err := svc.List(ctx, port.StoreID(in.Store), item.ID(in.Item))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &relationRowsOutput{Body: []RelationRow{}}
			for _, r := range rows {
				out.Body = append(out.Body, RelationRow{
					Label: string(r.Label), ID: string(r.Item.ID), Display: r.Display, Title: r.Item.Title,
					WorkspaceName: r.WorkspaceName, StatusName: r.Status.Name, StatusColor: string(r.Status.Color),
				})
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "list-relation-candidates", Method: http.MethodGet, Path: "/api/stores/{store}/items/{item}/relation-candidates", Summary: "関連の相手の候補を返す(ID かタイトルで絞る。20 件まで)"},
		func(ctx context.Context, in *candidatesInput) (*relationCandidatesOutput, error) {
			cs, err := svc.Candidates(ctx, port.StoreID(in.Store), item.ID(in.Item), relation.Label(in.Label), in.Q)
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &relationCandidatesOutput{Body: []RelationCandidate{}}
			for _, c := range cs {
				out.Body = append(out.Body, RelationCandidate{
					ID: string(c.Item.ID), Display: c.Display, Title: c.Item.Title,
					WorkspaceName: c.WorkspaceName, DisabledReason: c.DisabledReason,
				})
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "add-relation", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/relations", Summary: "関連を足す", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *relationEditInput) (*struct{}, error) {
			err := svc.Add(ctx, port.StoreID(in.Store), item.ID(in.Item), relation.Label(in.Body.Label), item.ID(in.Body.Other))
			return nil, apierr.From(err)
		})
	huma.Register(api, huma.Operation{OperationID: "remove-relation", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/relations/remove", Summary: "関連を外す", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *relationEditInput) (*struct{}, error) {
			err := svc.Remove(ctx, port.StoreID(in.Store), item.ID(in.Item), relation.Label(in.Body.Label), item.ID(in.Body.Other))
			return nil, apierr.From(err)
		})
}
```

(import の並びは `goimports` の順に直す: `apprelations` は `app/port` の後ろ。)

`internal/adapter/httpapi/notes/` の Task 3 が作った `Services` と `Register` に足す。

```go
// Services に足す
	Relations apprelations.Service
// Register に足す
	registerRelations(api, s.Relations)
```

`internal/adapter/httpapi/routes.go`: `notes.Services{...}` を組む箇所に足す(import に `apprelations "github.com/kukv/hedgehug/internal/app/relations"` を足す)。

```go
		Relations: apprelations.Service{Stores: deps.Stores, Identity: deps.Identity},
```

`internal/adapter/httpapi/items/items.go` の `Item` に足し、`toAPI` に写す。

```go
	Blocked   bool     `json:"blocked" doc:"ブロック中か。ブロックしている相手に、完了・中止でないものがある"`
```

`Watchers` の行の下に置く。`toAPI` の `Watchers: it.Watchers,` の行を `Watchers: it.Watchers, Blocked: it.Blocked,` に変える。

Run: `go test ./internal/adapter/httpapi/...`
Expected: `TestOpenAPIYAMLIsUpToDate` だけ FAIL(openapi.yaml が古い)、ほかは PASS

- [ ] **Step 23: 生成物を作り直す**

Run: `make generate`
Expected: `api/openapi.yaml` に `list-relations`・`list-relation-candidates`・`add-relation`・`remove-relation`、`RelationRow`・`RelationCandidate`・`RelationInput`、`Item.blocked` が増える(`git diff --stat api/openapi.yaml` で確かめる)。

Run: `cd web && pnpm gen`
Expected: `web/packages/api/src/generated.ts` に `listRelations`・`listRelationCandidates`・`addRelation`・`removeRelation`、`RelationRow`・`RelationCandidate`・`RelationInput` と `Item` の `blocked: boolean` が増える。

Run: `go test ./internal/adapter/httpapi/...` → PASS

`blocked` が `Item` に増えたので、画面の型検査が落ちる(`blocked` を持たない Item の書き方が `ports.test.ts` などにある)のは Task 14 で直す。この Task では `cd web && pnpm -C packages/api check` を確かめず、Go だけを確かめる。

- [ ] **Step 24: 全体の確認とコミット(API)**

Run: `go test ./internal/... && go vet ./internal/...`
Expected: PASS

```bash
git add internal/adapter/httpapi api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat(api): 関連の一覧・候補・追加・解除と、アイテムの blocked を足す(A4a-11〜13)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 14: 関連の区画と「ブロック中」の表示(詳細パネル・ツリー)

詳細パネルに「関連」の区画(一覧・解除・追加)を出し、種類の行とツリーの行に「ブロック中」を出す。サーバーは Task 13。

**この Task が決めること:**

- 画面の状態は core の `createRelations`(`features/notes/relations.ts`)に置く。詳細パネルの `DetailView`(`detail.ts` は 256 行で上限に近い)には足さない。区画は開いているアイテムが変わるたび、また詳細パネルが読み直されるたびに、`relations.load` で一覧を読み直す。
- 相手の候補は、入力のたびにサーバーへ問い合わせる(`relationCandidates`)。古い問い合わせの答えは捨てる。空の入力は問い合わせない。
- 表記のセレクトを変えたときは、選んだ相手を、新しい表記の規則で調べ直す。まだ選べるなら選んだままにし、選べなくなったら選択を外して、理由つきの候補を見せる。
- 関連を足す・外したら、一覧を読み直し、`onChanged`(開いている詳細パネルとツリーの読み直し。ブロック中が変わるため)を呼ぶ。トーストは出さない(02「関連」に書いていない)。断られたら(`Refused`)エラーのトーストを出す。
- 「ブロック中」は、ui の `BlockedBadge`(文言は固定)。詳細パネルの種類の行とツリーの行に出す。`TreeRow.svelte` は 196 行で上限(200 行)に近いので、チップの中身は `BlockedBadge` に置き、`TreeRow` には 3 行だけ足す。
- 候補の入力欄は入力のたびに知らせる必要があるので、ui の `Input` に `oninput` を足す(他のタスクが先に足していたら、重ねて足さない)。
- `Item` に `blocked: boolean` を足す。`Item` を書いているテストの補助(`fixtures.test-util.ts`・`tree.test.ts`・`ports.test.ts`)も合わせる。
- `NotesPort`(`web/packages/core/src/port/notes.ts`)は Task 7 が作る。このタスクは関連の 4 つのメソッドを足す。api の実装は、衝突を避けるため別ファイル `relationsPort.ts` に書き、`createNotesPort` の戻り値に `...relationCalls()` を 1 行足す。

**Files:**
- Create: `web/packages/core/src/model/relation.ts`
- Modify: `web/packages/core/src/model/item.ts`(`blocked`)・`web/packages/core/src/port/notes.ts`(4 メソッド)・`web/packages/core/src/notesIndex.ts`
- Create: `web/packages/core/src/features/notes/relations.ts`・`relations.test.ts`
- Modify: `web/packages/core/src/features/items/fixtures.test-util.ts`・`tree.test.ts`(`blocked: false`)
- Modify: `web/packages/api/src/items.ts`(`toItem`)・`ports.test.ts`
- Create: `web/packages/api/src/relationsPort.ts`・`relationsPort.test.ts`
- Modify: `web/packages/api/src/notes.ts`(`...relationCalls()` を 1 行)
- Create: `web/packages/ui/src/patterns/BlockedBadge.svelte`
- Modify: `web/packages/ui/src/patterns/patterns.test.ts`・`web/packages/ui/src/index.ts`
- Modify: `web/packages/ui/src/primitives/Input.svelte`・`inputs.test.ts`(`oninput`)
- Create: `web/app/src/screens/item/RelationsSection.svelte`
- Modify: `web/app/src/screens/item/DetailPanel.svelte`(種類の行に「ブロック中」、「関連」の区画)・`web/app/src/screens/project/TreeRow.svelte`・`web/app/src/wire.ts`
- Create: `web/e2e/relations.spec.ts`

**Interfaces:**
- Consumes:
  - `listRelations(store, item)`・`listRelationCandidates(store, item, {label, q})`・`addRelation(store, item, {label, other})`・`removeRelation(store, item, {label, other})`、型 `RelationRow`・`RelationCandidate`(`generated.ts`。Task 13 で生成済み)、`Item.blocked`
  - `NotesPort`(Task 7)、`call`(api の `errors.ts`)、`Toasts`、`NotFound`・`Refused`
- Produces:
  - `RelationLabel`・`relationLabels`・`RelationRow{label,id,display,title,workspaceName,statusName,statusColor}`・`RelationCandidate{id,display,title,workspaceName,disabledReason}`(core の model)
  - `Item.blocked: boolean`
  - `NotesPort` の `listRelations(storeId, itemId): Promise<RelationRow[]>`・`relationCandidates(storeId, itemId, label, query): Promise<RelationCandidate[]>`・`addRelation(storeId, itemId, label, otherId): Promise<void>`・`removeRelation(storeId, itemId, label, otherId): Promise<void>`
  - `createRelations(deps: RelationsDeps): Relations`、`groupRelations(rows): RelationGroup[]`
  - `BlockedBadge`(props なし)、`Input` の `oninput?: (value: string) => void`
  - `wire.ts` の `relations`

- [ ] **Step 1: 型を足す(`blocked`・関連の型)**

`web/packages/core/src/model/relation.ts`(新規):

```ts
// 関連(02「関連」)。表記は画面のとおり英語のまま。

export type RelationLabel =
  | "blocks"
  | "blocked by"
  | "causes"
  | "caused by"
  | "duplicates"
  | "duplicated by"
  | "relates to"
  | "clones"
  | "cloned by";

// relationLabels は、一覧の見出しの順であり、追加のセレクトの順(02「関連」)。
export const relationLabels: readonly RelationLabel[] = [
  "blocks",
  "blocked by",
  "causes",
  "caused by",
  "duplicates",
  "duplicated by",
  "relates to",
  "clones",
  "cloned by",
];

// RelationRow は、一覧の 1 行。workspaceName は、相手が別のワークスペースのときだけ(同じなら空)。
// statusName・statusColor は、相手のステータス(値がなければ空)。
export interface RelationRow {
  label: RelationLabel;
  id: string;
  display: string;
  title: string;
  workspaceName: string;
  statusName: string;
  statusColor: string;
}

// RelationCandidate は、相手の候補。disabledReason が空でなければ選べない(その理由)。
export interface RelationCandidate {
  id: string;
  display: string;
  title: string;
  workspaceName: string;
  disabledReason: string;
}
```

`web/packages/core/src/model/item.ts` の `Item` の `watchers: string[];` の下に足す。

```ts
  // blocked は、ブロック中か(ブロックしている相手に、完了・中止でないものがある。サーバーが決める)。
  blocked: boolean;
```

`Item` を書いている箇所に `blocked: false` を足す。`watchers: ["taro"],` の行の直後に 1 行ずつ。

```bash
sed -i 's/^\( *\)watchers: \["taro"\],$/&\n\1blocked: false,/' \
  web/packages/core/src/features/items/fixtures.test-util.ts \
  web/packages/core/src/features/items/tree.test.ts
git diff --stat web/packages/core/src/features/items
```

Expected: `fixtures.test-util.ts` が 3 行、`tree.test.ts` が 1 行増える。

`web/packages/api/src/items.ts` の `toItem` の `watchers: it.watchers ?? [],` の下に足す。

```ts
    blocked: it.blocked,
```

`web/packages/api/src/ports.test.ts` の `apiItem` の `watchers: null,` の下に `blocked: false,` を足す。

`web/packages/core/src/port/notes.ts`(Task 7 が作る)の先頭に import を足し、`export interface NotesPort {` の直後に 4 つのメソッドを足す。

```ts
import type { RelationCandidate, RelationLabel, RelationRow } from "../model/relation";
```

```ts
  // 関連(02「関連」)。label は、このアイテムから見た表記。
  listRelations(storeId: string, itemId: string): Promise<RelationRow[]>;
  relationCandidates(
    storeId: string,
    itemId: string,
    label: RelationLabel,
    query: string,
  ): Promise<RelationCandidate[]>;
  addRelation(storeId: string, itemId: string, label: RelationLabel, otherId: string): Promise<void>;
  removeRelation(
    storeId: string,
    itemId: string,
    label: RelationLabel,
    otherId: string,
  ): Promise<void>;
```

`web/packages/core/src/notesIndex.ts`(Task 5 が作る。`index.ts` が `export * from "./notesIndex"` で再公開する)に足す。`index.ts` には足さない(300 行の上限に近い)。

```ts
export {
  createRelations,
  groupRelations,
  type RelationGroup,
  type Relations,
  type RelationsDeps,
  type RelationsState,
} from "./features/notes/relations";
export {
  type RelationCandidate,
  type RelationLabel,
  type RelationRow,
  relationLabels,
} from "./model/relation";
```

Run: `pnpm -C web --filter @hedgehug/core check && pnpm -C web --filter @hedgehug/api check`
Expected: core は `./features/notes/relations` が見つからず FAIL(次のステップで作る)。`Item` の `blocked` に関する型の食い違いは出ない。

- [ ] **Step 2: core の失敗するテストを書く**

`web/packages/core/src/features/notes/relations.test.ts`(新規):

```ts
import { describe, expect, it } from "vitest";
import { NotFound, Refused } from "../../model/errors";
import type { RelationCandidate, RelationLabel, RelationRow } from "../../model/relation";
import { get } from "../../reactive/store";
import { flush } from "../items/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createRelations, groupRelations, type RelationsDeps } from "./relations";

const row = (label: RelationLabel, id: string, display: string): RelationRow => ({
  label,
  id,
  display,
  title: `${display} のタイトル`,
  workspaceName: "",
  statusName: "Todo",
  statusColor: "gray",
});
const cand = (id: string, display: string, disabledReason = ""): RelationCandidate => ({
  id,
  display,
  title: `${display} のタイトル`,
  workspaceName: "",
  disabledReason,
});
const first = <T>(xs: T[]): T => {
  const x = xs[0];
  if (x === undefined) throw new Error("empty");
  return x;
};

function setup(initial: RelationRow[] = [row("blocks", "I2", "HH-2")]) {
  const calls: string[] = [];
  let rows = initial;
  // answers は、`${表記}|${検索語}` ごとの候補。hold に入れた検索語の問い合わせは、waiting の関数を呼ぶまで答えない。
  const answers = new Map<string, RelationCandidate[]>();
  const hold = new Set<string>();
  const waiting = new Map<string, (cs: RelationCandidate[]) => void>();
  let refusal = "";
  let missing = false;
  const notes: RelationsDeps["notes"] = {
    async listRelations(storeId, itemId) {
      calls.push(`list ${storeId}/${itemId}`);
      if (missing) throw new NotFound("not found");
      return rows;
    },
    relationCandidates(_s, _i, label, query) {
      calls.push(`candidates ${label} ${query}`);
      if (hold.has(query)) return new Promise((resolve) => waiting.set(query, resolve));
      return Promise.resolve(answers.get(`${label}|${query}`) ?? []);
    },
    async addRelation(_s, _i, label, other) {
      calls.push(`add ${label} ${other}`);
      if (refusal) throw new Refused(refusal);
      rows = [...rows, row(label, other, other)];
    },
    async removeRelation(_s, _i, label, other) {
      calls.push(`remove ${label} ${other}`);
      rows = rows.filter((r) => !(r.label === label && r.id === other));
    },
  };
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  let changed = 0;
  const relations = createRelations({ notes, toasts, onChanged: () => void changed++ });
  return {
    relations,
    toasts,
    calls,
    answers,
    hold,
    waiting,
    changed: () => changed,
    refuse: (m: string) => {
      refusal = m;
    },
    vanish: () => {
      missing = true;
    },
  };
}

describe("groupRelations", () => {
  it("表記の順に見出しをまとめ、関連のない表記は出さない", () => {
    const groups = groupRelations([
      row("clones", "I5", "HH-5"),
      row("blocked by", "I1", "HH-1"),
      row("blocks", "I2", "HH-2"),
      row("blocks", "I3", "HH-3"),
    ]);
    expect(groups.map((g) => [g.label, g.rows.map((r) => r.display)])).toEqual([
      ["blocks", ["HH-2", "HH-3"]],
      ["blocked by", ["HH-1"]],
      ["clones", ["HH-5"]],
    ]);
  });
});

describe("createRelations", () => {
  it("読む前は loaded が false、読んだあとは一覧と見出しを持つ", async () => {
    const { relations } = setup();
    expect(get(relations.state)).toMatchObject({ loaded: false, rows: [], groups: [] });
    await relations.load("S1", "I1");
    const s = get(relations.state);
    expect(s.loaded).toBe(true);
    expect(s.groups.map((g) => g.label)).toEqual(["blocks"]);
    expect(s.label).toBe("blocks");
  });

  it("関連がなければ、読んだあとも一覧は空", async () => {
    const { relations } = setup([]);
    await relations.load("S1", "I1");
    expect(get(relations.state)).toMatchObject({ loaded: true, rows: [], groups: [] });
  });

  it("アイテムがなくなっていたら、空の一覧にする", async () => {
    const { relations, vanish } = setup();
    vanish();
    await relations.load("S1", "I1");
    expect(get(relations.state)).toMatchObject({ loaded: true, rows: [] });
  });

  it("検索して選ぶと「追加」が押せる。選べない候補は選べない", async () => {
    const { relations, answers } = setup();
    answers.set("blocks|hh", [cand("I3", "HH-3"), cand("I4", "HH-4", "すでに blocks の関連があります")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    expect(get(relations.state).candidates.map((c) => c.id)).toEqual(["I3", "I4"]);
    relations.choose(get(relations.state).candidates[1] as RelationCandidate);
    expect(get(relations.state)).toMatchObject({ selected: null, canAdd: false });
    relations.choose(first(get(relations.state).candidates));
    expect(get(relations.state)).toMatchObject({
      selected: { id: "I3" },
      query: "HH-3 HH-3 のタイトル",
      candidates: [],
      canAdd: true,
    });
  });

  it("入力し直すと、選んだ相手を外す。空の入力は問い合わせない", async () => {
    const { relations, answers, calls } = setup();
    answers.set("blocks|hh", [cand("I3", "HH-3")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    relations.choose(first(get(relations.state).candidates));
    await relations.setQuery("h");
    expect(get(relations.state)).toMatchObject({ selected: null, canAdd: false });
    await relations.setQuery("  ");
    expect(get(relations.state).candidates).toEqual([]);
    expect(calls.filter((c) => c.startsWith("candidates")).length).toBe(2);
  });

  it("追加すると保存し、一覧を読み直し、入力を空にして、画面に知らせる", async () => {
    const { relations, answers, calls, changed } = setup();
    answers.set("blocks|hh", [cand("I3", "HH-3")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    relations.choose(first(get(relations.state).candidates));
    expect(await relations.add()).toBe(true);
    expect(calls).toContain("add blocks I3");
    expect(calls.filter((c) => c.startsWith("list")).length).toBe(2);
    expect(get(relations.state)).toMatchObject({ query: "", candidates: [], selected: null, label: "blocks" });
    expect(get(relations.state).rows.map((r) => r.id)).toEqual(["I2", "I3"]);
    expect(changed()).toBe(1);
  });

  it("相手を選んでいなければ追加しない", async () => {
    const { relations, calls, changed } = setup();
    await relations.load("S1", "I1");
    expect(await relations.add()).toBe(false);
    expect(calls.some((c) => c.startsWith("add"))).toBe(false);
    expect(changed()).toBe(0);
  });

  it("断られたら、エラーのトーストを出し、選んだ相手は残す", async () => {
    const { relations, answers, toasts, refuse, changed } = setup();
    answers.set("blocks|hh", [cand("I3", "HH-3")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    relations.choose(first(get(relations.state).candidates));
    refuse("循環になります");
    expect(await relations.add()).toBe(false);
    expect(get(toasts.list)[0]).toMatchObject({ text: "循環になります", error: true });
    expect(get(relations.state).selected?.id).toBe("I3");
    expect(changed()).toBe(0);
  });

  it("表記を変えると、選んだ相手を新しい表記で調べ直す", async () => {
    const { relations, answers, calls } = setup();
    answers.set("blocks|hh", [cand("I3", "HH-3")]);
    answers.set("causes|HH-3", [cand("I3", "HH-3")]);
    answers.set("blocked by|HH-3", [cand("I3", "HH-3", "すでに blocked by の関連があります")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    relations.choose(first(get(relations.state).candidates));
    // まだ選べるなら、選んだまま。
    await relations.setLabel("causes");
    expect(calls).toContain("candidates causes HH-3");
    expect(get(relations.state)).toMatchObject({ label: "causes", canAdd: true, selected: { id: "I3" } });
    // 選べなくなったら、選択を外して、理由つきの候補を見せる。
    await relations.setLabel("blocked by");
    const s = get(relations.state);
    expect(s).toMatchObject({ label: "blocked by", selected: null, canAdd: false, query: "HH-3" });
    expect(s.candidates.map((c) => c.disabledReason)).toEqual(["すでに blocked by の関連があります"]);
  });

  it("古い検索の答えは捨てる", async () => {
    const { relations, hold, waiting } = setup();
    hold.add("h");
    hold.add("hh");
    await relations.load("S1", "I1");
    const slow = relations.setQuery("h");
    const fast = relations.setQuery("hh");
    await flush();
    waiting.get("hh")?.([cand("I3", "HH-3")]);
    await fast;
    waiting.get("h")?.([cand("I9", "HH-9")]);
    await slow;
    expect(get(relations.state).candidates.map((c) => c.id)).toEqual(["I3"]);
  });

  it("解除すると、一覧を読み直して画面に知らせる", async () => {
    const { relations, calls, changed } = setup([row("blocks", "I2", "HH-2"), row("clones", "I3", "HH-3")]);
    await relations.load("S1", "I1");
    await relations.remove(first(get(relations.state).rows));
    expect(calls).toContain("remove blocks I2");
    expect(get(relations.state).rows.map((r) => r.id)).toEqual(["I3"]);
    expect(changed()).toBe(1);
  });

  it("同じアイテムの読み直しでは入力を残し、別のアイテムでは追加の行を戻す", async () => {
    const { relations, answers } = setup();
    answers.set("blocks|hh", [cand("I3", "HH-3")]);
    await relations.load("S1", "I1");
    await relations.setQuery("hh");
    await relations.load("S1", "I1");
    expect(get(relations.state)).toMatchObject({ query: "hh" });
    expect(get(relations.state).candidates).toHaveLength(1);
    await relations.setLabel("causes");
    await relations.load("S1", "I9");
    expect(get(relations.state)).toMatchObject({ label: "blocks", query: "", candidates: [], selected: null, loaded: true });
  });

  it("reset は、状態を最初に戻す", async () => {
    const { relations } = setup();
    await relations.load("S1", "I1");
    relations.reset();
    expect(get(relations.state)).toMatchObject({ loaded: false, rows: [], query: "" });
  });
});
```

- [ ] **Step 3: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core exec vitest run src/features/notes/relations.test.ts`
Expected: FAIL(`Failed to resolve import "./relations"`)

- [ ] **Step 4: core を実装する**

`web/packages/core/src/features/notes/relations.ts`(新規):

```ts
import { NotFound, Refused } from "../../model/errors";
import type { RelationCandidate, RelationLabel, RelationRow } from "../../model/relation";
import { relationLabels } from "../../model/relation";
import type { NotesPort } from "../../port/notes";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Toasts } from "../toasts/toasts";

// 詳細パネルの「関連」(02「関連」)の状態: 一覧、解除、追加の行(表記・相手の検索・選んだ相手)。

export interface RelationGroup {
  label: RelationLabel;
  rows: RelationRow[];
}

// groupRelations は、一覧を表記ごとの見出しにまとめる。表記の順で、関連のない表記は出さない。
export function groupRelations(rows: RelationRow[]): RelationGroup[] {
  return relationLabels
    .map((label) => ({ label, rows: rows.filter((r) => r.label === label) }))
    .filter((g) => g.rows.length > 0);
}

export interface RelationsState {
  // loaded は、一覧を読み終えたか(読むまでは「関連はありません」を出さない)。
  loaded: boolean;
  rows: RelationRow[];
  groups: RelationGroup[];
  // 追加の行: 表記のセレクト、相手の入力欄、候補、選んだ相手。
  label: RelationLabel;
  query: string;
  candidates: RelationCandidate[];
  selected: RelationCandidate | null;
  // canAdd は、「追加」が押せるか(選べる相手を選んでいる)。
  canAdd: boolean;
}

export interface Relations {
  state: Readable<RelationsState>;
  // load は、アイテムの関連を読む。同じアイテムの読み直しでは、追加の行を残す。
  load(storeId: string, itemId: string): Promise<void>;
  setLabel(label: RelationLabel): Promise<void>;
  setQuery(query: string): Promise<void>;
  // choose は、候補を選ぶ。選べない候補は無視する。
  choose(candidate: RelationCandidate): void;
  // add は、選んだ相手を、いまの表記で足す。足せたら true。
  add(): Promise<boolean>;
  remove(row: RelationRow): Promise<void>;
  reset(): void;
}

export interface RelationsDeps {
  notes: Pick<NotesPort, "listRelations" | "relationCandidates" | "addRelation" | "removeRelation">;
  toasts: Toasts;
  // onChanged は、関連を足す・外したあと(ブロック中が変わるので、開いている詳細パネルとツリーを読み直す)。
  onChanged(): void;
}

const blank: RelationsState = {
  loaded: false,
  rows: [],
  groups: [],
  label: "blocks",
  query: "",
  candidates: [],
  selected: null,
  canAdd: false,
};

export function createRelations(deps: RelationsDeps): Relations {
  const state = writable<RelationsState>(blank);
  let target: { storeId: string; itemId: string } | null = null;
  // loads・searches は、古い答えを捨てるための通し番号。
  let loads = 0;
  let searches = 0;

  const patch = (p: Partial<RelationsState>) =>
    state.update((s) => {
      const next = { ...s, ...p };
      next.groups = groupRelations(next.rows);
      next.canAdd = next.selected !== null && next.selected.disabledReason === "";
      return next;
    });

  async function loadRows() {
    if (!target) return;
    const t = target;
    loads += 1;
    const mine = loads;
    try {
      const rows = await deps.notes.listRelations(t.storeId, t.itemId);
      if (mine === loads) patch({ rows, loaded: true });
    } catch (e) {
      // アイテムがなくなっていたら空にする。ほかの失敗は出さない(画面への反映は失敗を出さない。A2 の決定 29)。
      if (e instanceof NotFound && mine === loads) patch({ rows: [], loaded: true });
    }
  }

  // runSearch は、候補を問い合わせる。古くなった答えと失敗は null(検索の失敗は、次の入力でやり直せる)。
  async function runSearch(query: string): Promise<RelationCandidate[] | null> {
    if (!target) return null;
    searches += 1;
    const mine = searches;
    if (query.trim() === "") return [];
    try {
      const list = await deps.notes.relationCandidates(
        target.storeId,
        target.itemId,
        get(state).label,
        query,
      );
      return mine === searches ? list : null;
    } catch (e) {
      if (e instanceof NotFound && mine === searches) return [];
      return null;
    }
  }

  function refusedToast(e: unknown) {
    if (!(e instanceof Refused)) throw e;
    deps.toasts.show(e.message, { error: true });
  }

  return {
    state: readonly(state),
    async load(storeId, itemId) {
      const same = target?.storeId === storeId && target.itemId === itemId;
      target = { storeId, itemId };
      if (!same) {
        searches += 1;
        state.set(blank);
      }
      await loadRows();
    },
    async setLabel(label) {
      patch({ label });
      const picked = get(state).selected;
      const list = await runSearch(picked ? picked.display : get(state).query);
      if (!list) return;
      if (!picked) {
        patch({ candidates: list });
        return;
      }
      const again = list.find((c) => c.id === picked.id);
      if (again && again.disabledReason === "") patch({ selected: again });
      else patch({ selected: null, query: picked.display, candidates: list });
    },
    async setQuery(query) {
      patch({ query, selected: null });
      const list = await runSearch(query);
      if (list) patch({ candidates: list });
    },
    choose(candidate) {
      if (candidate.disabledReason !== "") return;
      searches += 1;
      patch({ selected: candidate, query: `${candidate.display} ${candidate.title}`, candidates: [] });
    },
    async add() {
      const { selected, label } = get(state);
      if (!target || !selected || selected.disabledReason !== "") return false;
      try {
        await deps.notes.addRelation(target.storeId, target.itemId, label, selected.id);
      } catch (e) {
        refusedToast(e);
        await loadRows();
        return false;
      }
      searches += 1;
      patch({ query: "", candidates: [], selected: null });
      await loadRows();
      deps.onChanged();
      return true;
    },
    async remove(row) {
      if (!target) return;
      try {
        await deps.notes.removeRelation(target.storeId, target.itemId, row.label, row.id);
      } catch (e) {
        refusedToast(e);
      }
      await loadRows();
      deps.onChanged();
    },
    reset() {
      target = null;
      loads += 1;
      searches += 1;
      state.set(blank);
    },
  };
}
```

Run: `pnpm -C web --filter @hedgehug/core exec vitest run src/features/notes/relations.test.ts` → PASS
Run: `pnpm -C web --filter @hedgehug/core check` → 型の誤りなし

- [ ] **Step 5: コミット(core)**

```bash
git add web/packages/core web/packages/api/src/items.ts web/packages/api/src/ports.test.ts
git commit -m "feat(core): 関連の区画の状態と、Item の blocked を足す(A4a-13)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 6: api の失敗するテストを書く**

`web/packages/api/src/relationsPort.test.ts`(新規):

```ts
import { Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createItemsPort } from "./items";
import { relationCalls } from "./relationsPort";

afterEach(() => {
  vi.unstubAllGlobals();
});

// respond は、fetch を 1 つの応答に差し替え、呼び出しを返す。
function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async () =>
      // 204 の応答は本文を持てない(Response は null の本文だけを受ける)。
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetch);
  setToken("tok");
  return fetch;
}

function call(fetch: ReturnType<typeof respond>, n = 0) {
  return fetch.mock.calls[n] as unknown as [string, RequestInit];
}

describe("relations calls", () => {
  it("reads null lists as empty", async () => {
    const fetch = respond(200, null);
    expect(await relationCalls().listRelations("S1", "I1")).toEqual([]);
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/relations");
  });

  it("asks for candidates with the label and the query", async () => {
    const fetch = respond(200, [
      { id: "I3", display: "HH-3", title: "決済", workspaceName: "", disabledReason: "" },
    ]);
    const got = await relationCalls().relationCandidates("S1", "I1", "blocked by", "決済");
    expect(got).toEqual([
      { id: "I3", display: "HH-3", title: "決済", workspaceName: "", disabledReason: "" },
    ]);
    const url = new URL(call(fetch)[0], "http://x");
    expect(url.pathname).toBe("/api/stores/S1/items/I1/relation-candidates");
    expect(url.searchParams.get("label")).toBe("blocked by");
    expect(url.searchParams.get("q")).toBe("決済");
  });

  it("adds and removes with the label and the other item", async () => {
    const fetch = respond(204, undefined);
    await relationCalls().addRelation("S1", "I1", "blocks", "I3");
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/relations");
    expect(call(fetch)[1].method).toBe("POST");
    expect(JSON.parse(String(call(fetch)[1].body))).toEqual({ label: "blocks", other: "I3" });
    await relationCalls().removeRelation("S1", "I1", "blocked by", "I3");
    expect(call(fetch, 1)[0]).toBe("/api/stores/S1/items/I1/relations/remove");
    expect(JSON.parse(String(call(fetch, 1)[1].body))).toEqual({ label: "blocked by", other: "I3" });
  });

  it("turns a 422 into Refused with the words of the spec", async () => {
    respond(422, { title: "Unprocessable Entity", status: 422, detail: "循環になります" });
    const err = await relationCalls()
      .addRelation("S1", "I1", "blocks", "I3")
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(Refused);
    expect((err as Refused).message).toBe("循環になります");
  });
});

describe("items port: blocked", () => {
  it("reads blocked from the item", async () => {
    respond(200, {
      id: "I1",
      workspace: "W1",
      project: "P1",
      parent: null,
      number: 1,
      kind: "Task",
      rank: "V",
      title: "画面",
      watchers: [],
      createdAt: "2026-11-13T09:00:00.000Z",
      createdBy: "taro",
      blocked: true,
    });
    expect(await createItemsPort().get("S1", "I1")).toMatchObject({ id: "I1", blocked: true });
  });
});
```

- [ ] **Step 7: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/api exec vitest run src/relationsPort.test.ts`
Expected: FAIL(`Failed to resolve import "./relationsPort"`)

- [ ] **Step 8: api を実装する**

`web/packages/api/src/relationsPort.ts`(新規):

```ts
import type { NotesPort, RelationLabel } from "@hedgehug/core";
import { call } from "./errors";
import { addRelation, listRelationCandidates, listRelations, removeRelation } from "./generated";

export type RelationCalls = Pick<
  NotesPort,
  "listRelations" | "relationCandidates" | "addRelation" | "removeRelation"
>;

// relationCalls は、NotesPort の関連のメソッド(createNotesPort の戻り値に広げる)。
export function relationCalls(): RelationCalls {
  return {
    listRelations: (storeId, itemId) =>
      call(async () =>
        ((await listRelations(storeId, itemId)) ?? []).map((r) => ({
          ...r,
          label: r.label as RelationLabel,
        })),
      ),
    relationCandidates: (storeId, itemId, label, query) =>
      call(async () => (await listRelationCandidates(storeId, itemId, { label, q: query })) ?? []),
    addRelation: (storeId, itemId, label, otherId) =>
      call(async () => {
        await addRelation(storeId, itemId, { label, other: otherId });
      }),
    removeRelation: (storeId, itemId, label, otherId) =>
      call(async () => {
        await removeRelation(storeId, itemId, { label, other: otherId });
      }),
  };
}
```

`web/packages/api/src/notes.ts`(Task 7 が作る)の `createNotesPort` の戻り値のオブジェクトの先頭に 1 行足し、import を足す。

```ts
import { relationCalls } from "./relationsPort";
```

```ts
    ...relationCalls(),
```

Run: `pnpm -C web --filter @hedgehug/api exec vitest run src/relationsPort.test.ts` → PASS
Run: `pnpm -C web --filter @hedgehug/api check` → 型の誤りなし

- [ ] **Step 9: コミット(api)**

```bash
git add web/packages/api/src/relationsPort.ts web/packages/api/src/relationsPort.test.ts web/packages/api/src/notes.ts
git commit -m "feat(api): 関連の一覧・候補・追加・解除を画面の口につなぐ(A4a-12)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 10: ui の失敗するテストを書く**

`web/packages/ui/src/patterns/patterns.test.ts` の import に `import BlockedBadge from "./BlockedBadge.svelte";` を足し(`KindIcon` の import の上)、末尾に足す。

```ts
describe("BlockedBadge", () => {
  it("says the item is blocked", () => {
    render(BlockedBadge);
    expect(screen.getByText("ブロック中")).toBeInTheDocument();
  });
});
```

`web/packages/ui/src/primitives/inputs.test.ts` の `describe("Input", ...)` の中に足す。

```ts
  it("reports the typed value as it changes", async () => {
    const oninput = vi.fn();
    render(Input, { props: { label: "相手", oninput } });
    await fireEvent.input(screen.getByRole("textbox", { name: "相手" }), { target: { value: "HH" } });
    expect(oninput).toHaveBeenCalledWith("HH");
  });
```

- [ ] **Step 11: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/ui exec vitest run src/patterns/patterns.test.ts src/primitives/inputs.test.ts`
Expected: FAIL(`./BlockedBadge.svelte` がない。`oninput` が呼ばれない)

- [ ] **Step 12: ui を実装する**

`web/packages/ui/src/patterns/BlockedBadge.svelte`(新規):

```svelte
<span class="blocked">ブロック中</span>

<style>
  .blocked {
    display: inline-flex;
    align-items: center;
    height: 18px;
    padding: 0 6px;
    border-radius: 9px;
    background: var(--danger-weak);
    color: var(--danger);
    font-size: 11px;
    font-weight: 600;
    white-space: nowrap;
  }
</style>
```

`web/packages/ui/src/index.ts` の `Avatar` の行の下に足す。

```ts
export { default as BlockedBadge } from "./patterns/BlockedBadge.svelte";
```

`web/packages/ui/src/primitives/Input.svelte` を変える: `Props` の `onblur` の上に足す。

```ts
    // oninput は、入力のたびに、いまの値を渡す(候補を探すときなど)。
    oninput?: (value: string) => void;
```

分割代入の `onblur,` の上に `oninput,` を足し、`<input>` の `{onblur}` の上に足す。

```svelte
  oninput={(event) => oninput?.(event.currentTarget.value)}
```

Run: `pnpm -C web --filter @hedgehug/ui exec vitest run src/patterns/patterns.test.ts src/primitives/inputs.test.ts` → PASS

- [ ] **Step 13: コミット(ui)**

```bash
git add web/packages/ui
git commit -m "feat(ui): 「ブロック中」のチップと、入力欄の oninput を足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 14: app に区画と「ブロック中」を結び付ける**

`web/app/src/wire.ts`: `detail` を作ったあと(`export const detail = createDetail({...});` の下)に足す。`createRelations` を core からの import に足し、`notes` は Task 7 が `wire.ts` に作った `NotesPort` の定数。

```ts
export const relations = createRelations({
  notes,
  toasts,
  // 関連を足す・外すと「ブロック中」が変わるので、開いている詳細パネルとツリーを読み直す。
  onChanged: () => void Promise.all([detail.reload(), projectScreen.reload()]),
});
```

`web/app/src/screens/item/RelationsSection.svelte`(新規):

```svelte
<script lang="ts">
  import { type DetailView, readOnlyNote, type RelationLabel, type RelationRow, relationLabels } from "@hedgehug/core";
  import { Button, IconButton, Input, Select } from "@hedgehug/ui";
  import { onDestroy } from "svelte";
  import { detail, links, relations } from "../../wire";

  // 詳細パネルの「関連」(02「関連」)。一覧は表記ごとに見出しを付け、相手を押すとその詳細パネルを開く。
  // 追加は、表記のセレクト・相手の検索・「追加」。閲覧のみの Store では、足す・外すを無効にする。
  interface Props {
    v: DetailView;
  }

  let { v }: Props = $props();
  const model = relations.state;
  const note = $derived(v.readOnly ? readOnlyNote : undefined);
  const options = relationLabels.map((l) => ({ value: l, label: l }));
  const route = (r: RelationRow) => ({ name: "item" as const, store: v.store.name, item: r.display });

  // 開いているアイテムが変わるたびと、詳細パネルが読み直されるたびに、一覧を読み直す(v が新しくなる)。
  $effect(() => {
    void relations.load(v.store.id, v.item.id);
  });
  onDestroy(() => relations.reset());

  function onkeydown(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing && $model.canAdd) void relations.add();
  }
</script>

<section class="peek-sec" aria-label="関連">
  <h3>関連</h3>
  {#if $model.loaded && $model.groups.length === 0}
    <p class="muted">関連はありません</p>
  {/if}
  {#each $model.groups as g (g.label)}
    <h4>{g.label}</h4>
    {#each g.rows as r (r.id)}
      <div class="rel-row">
        <code class="id">{r.display}</code>
        <a
          class="row-title"
          href={links.href(route(r))}
          onclick={(e) => links.click(e, route(r), () => void detail.open(v.store.id, r.id))}
          onauxclick={(e) => links.click(e, route(r))}
          >{r.title}</a
        >
        {#if r.workspaceName}
          <span class="muted">{r.workspaceName}</span>
        {/if}
        {#if r.statusName}
          <span class="status" style="--bg: var(--c-{r.statusColor}-bg); --fg: var(--c-{r.statusColor}-fg)">{r.statusName}</span>
        {/if}
        <IconButton
          label="{g.label} {r.display} を解除"
          size="xs"
          disabled={v.readOnly}
          title={note}
          onclick={() => void relations.remove(r)}
          >×</IconButton
        >
      </div>
    {/each}
  {/each}
  <div class="add-row">
    <Select
      label="関連の表記"
      size="sm"
      {options}
      value={$model.label}
      disabled={v.readOnly}
      title={note}
      onchange={(l) => void relations.setLabel(l as RelationLabel)}
    />
    <Input
      label="相手"
      size="sm"
      placeholder="ID またはタイトルで検索"
      value={$model.query}
      disabled={v.readOnly}
      title={note}
      oninput={(q) => void relations.setQuery(q)}
      {onkeydown}
    />
    <Button size="sm" disabled={v.readOnly || !$model.canAdd} title={note} onclick={() => void relations.add()}>追加</Button>
  </div>
  {#if $model.candidates.length > 0}
    <ul class="cands" aria-label="候補">
      {#each $model.candidates as c (c.id)}
        <li>
          <button type="button" class="cand" disabled={c.disabledReason !== ""} onclick={() => relations.choose(c)}>
            <code class="id">{c.display}</code>
            <span class="cand-title">{c.title}</span>
            {#if c.workspaceName}
              <span class="muted">{c.workspaceName}</span>
            {/if}
            {#if c.disabledReason}
              <span class="reason">{c.disabledReason}</span>
            {/if}
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .peek-sec {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  h3 {
    font-size: 13px;
    margin: 0;
  }
  h4 {
    font-size: 11.5px;
    font-weight: 600;
    color: var(--muted);
    margin: 4px 0 0;
  }
  .muted {
    color: var(--muted);
    font-size: 12.5px;
    margin: 0;
  }
  .rel-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 30px;
    border-bottom: 1px solid var(--line);
  }
  .row-title {
    flex: 1;
    min-width: 0;
    color: var(--ink);
    text-decoration: none;
  }
  .row-title:hover {
    text-decoration: underline;
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .status {
    padding: 0 6px;
    border-radius: 9px;
    font-size: 11.5px;
    background: var(--bg);
    color: var(--fg);
    white-space: nowrap;
  }
  .add-row {
    display: flex;
    gap: 6px;
    align-items: center;
  }
  .add-row :global(input) {
    flex: 1;
  }
  .cands {
    list-style: none;
    margin: 0;
    padding: 0;
    border: 1px solid var(--line);
    border-radius: 7px;
  }
  .cand {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 5px 8px;
    border: 0;
    background: none;
    text-align: left;
    cursor: pointer;
  }
  .cand:hover:not(:disabled) {
    background: var(--hover);
  }
  .cand:disabled {
    cursor: not-allowed;
    color: var(--muted);
  }
  .cand-title {
    flex: 1;
    min-width: 0;
  }
  .reason {
    font-size: 11.5px;
    color: var(--danger);
  }
</style>
```

`web/app/src/screens/item/DetailPanel.svelte` を変える。import に `BlockedBadge` と `RelationsSection` を足す。

```ts
  import { BlockedBadge, IconButton, Input, KindIcon, ReadOnlyBand, SidePanel } from "@hedgehug/ui";
```

```ts
  import RelationsSection from "./RelationsSection.svelte";
```

種類の行の ID をコピーするボタンの閉じタグ `</IconButton>` の直後(`<div class="kind-row">` の中)に足す。

```svelte
      {#if v.item.blocked}
        <BlockedBadge />
      {/if}
```

「子」の区画のあと、GitHub の区画の前に足す。

```svelte
    <RelationsSection {v} />
```

`web/app/src/screens/project/TreeRow.svelte` を変える(196 行 → 199 行)。import を変える。

```ts
  import { Avatar, BlockedBadge, IconButton, KindIcon } from "@hedgehug/ui";
```

タイトルの `<a>` の閉じ(`>{it.title}</a\n  >`)の直後に足す。

```svelte
  {#if it.blocked}
    <BlockedBadge />
  {/if}
```

Run: `wc -l web/app/src/screens/project/TreeRow.svelte web/app/src/screens/item/RelationsSection.svelte web/app/src/screens/item/DetailPanel.svelte`
Expected: `TreeRow.svelte` は 200 行以下。`RelationsSection.svelte` も 200 行以下(超えたら、候補の一覧を `RelationCandidates.svelte` に分ける)。

Run: `pnpm -C web check`
Expected: 型の誤りなし(`svelte-check --fail-on-warnings` も通る)。`pnpm -C web lint` で書式を確かめ、崩れていたら `pnpm -C web fix`。

- [ ] **Step 15: 通しのテストを書く**

`web/e2e/relations.spec.ts`(新規。`helpers.ts` の `newTask` を使う):

```ts
import { expect, test } from "@playwright/test";
import { newTask } from "./helpers";

test("adds a blocked by relation, shows Blocked in the panel and the tree, and removes it", async ({
  page,
}, info) => {
  const blocker = await newTask(page, info, "ブロックする");
  const blocked = await newTask(page, info, "ブロックされる");
  const panel = blocked.panel;
  const rel = panel.getByRole("region", { name: "関連" });
  await expect(rel.getByText("関連はありません")).toBeVisible();

  await rel.getByRole("combobox", { name: "関連の表記" }).selectOption("blocked by");
  await rel.getByRole("textbox", { name: "相手" }).fill(blocker.name);
  await rel.getByRole("button", { name: blocker.name }).click();
  await rel.getByRole("button", { name: "追加", exact: true }).click();

  await expect(rel.getByRole("heading", { name: "blocked by" })).toBeVisible();
  await expect(rel.getByRole("link", { name: blocker.name })).toBeVisible();
  await expect(panel.getByText("ブロック中")).toBeVisible();
  const treeRow = page
    .getByRole("list", { name: "ツリー" })
    .getByRole("listitem")
    .filter({ hasText: blocked.name });
  await expect(treeRow).toContainText("ブロック中");

  // 同じ種類の関連がある相手は、候補に出ても選べず、理由が付く。
  await rel.getByRole("textbox", { name: "相手" }).fill(blocker.name);
  await expect(rel.getByRole("button", { name: blocker.name })).toBeDisabled();
  await expect(rel.getByText("すでに blocked by の関連があります")).toBeVisible();

  await rel.getByRole("button", { name: new RegExp("を解除$") }).click();
  await expect(rel.getByText("関連はありません")).toBeVisible();
  await expect(panel.getByText("ブロック中")).toBeHidden();
  await expect(treeRow).not.toContainText("ブロック中");
});
```

Run: `make e2e-bin`
Run: `pnpm -C web exec playwright test relations`
Expected: 3 つの幅(phone・tablet・desktop)で PASS

- [ ] **Step 16: 全体の確認とコミット**

Run: `pnpm -C web lint && pnpm -C web check && pnpm -C web test && pnpm -C web deps && pnpm -C web size`
Expected: すべて通る(`size` は Svelte 200 行・TS 300 行の上限)。

```bash
git add web/app web/e2e/relations.spec.ts
git commit -m "feat(app): 詳細パネルの関連の区画と、ブロック中の表示を足す(A4a-13)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 15: ウォッチの操作と API・ヘッダーのボタン

**Files:**
- Create: `internal/app/watching/service.go`・`set.go`
- Test: `internal/app/watching/watching_test.go`
- Create: `internal/adapter/httpapi/items/watch.go`
- Test: `internal/adapter/httpapi/items/watch_test.go`
- Modify: `internal/adapter/httpapi/routes.go`
- 生成物: `api/openapi.yaml`・`web/packages/api/src/generated.ts`
- Modify: `web/packages/core/src/port/notes.ts`(Task 7 が作る。`setWatch` を足す)
- Modify: `web/packages/core/src/features/notes/fixtures.test-util.ts`(Task 7 の `fakeNotesPort` に `setWatch` を足す。同じコミットで)
- Modify: `web/packages/api/src/notes.ts`・`notes.test.ts`(Task 7 が作る。`setWatch` を足す)
- Create: `web/packages/core/src/features/notes/watch.ts`、Test: `watch.test.ts`
- Modify: `web/packages/core/src/notesIndex.ts`(core の export はここに足す。`index.ts` は変えない)
- Create: `web/app/src/screens/item/WatchButton.svelte`
- Modify: `web/app/src/screens/item/DetailHeader.svelte`・`web/app/src/wire.ts`

**Interfaces:**
- Consumes: 既存の `items.Service.Get`(返すアイテムを読む)、`items` の `oneOutput`・`toAPI`・`one`、`port.Identity`、`apierr.From`、`Item.Watchers`(`item.FromLog` が `watchers` の項目から読む)、`readOnlyNote`、`DetailView`(`item`・`people.me`・`readOnly`)、`detail.reload()`
- Produces:
  - `watching.Service{Stores port.Stores; Identity port.Identity}`、`func (s Service) Set(ctx context.Context, store port.StoreID, id item.ID, on bool) error`
  - `items.RegisterWatch(api huma.API, svc appitems.Service, w appwatching.Service)`、API `set-watch`(PUT `/api/stores/{store}/items/{item}/watch`、本文 `items.SetWatch{on}`、応答 `Item`)
  - `NotesPort.setWatch(storeId: string, itemId: string, on: boolean): Promise<Item>`
  - `watchView(item: Pick<Item, "watchers">, me: string, readOnly: boolean): WatchView`(`WatchView = { watching: boolean; label: string; disabled: boolean; note: string | undefined }`)
  - `createWatch(deps: WatchDeps): Watch`(`WatchDeps = { notes: Pick<NotesPort, "setWatch">; toasts: Toasts; onChanged(): void }`、`Watch = { busy: Readable<boolean>; toggle(item: Item, me: string): Promise<void> }`)
  - `WatchButton.svelte`(props `{ v: DetailView }`)、`wire.ts` の `watch`

**行の形(A4a-14):** 既存の `watchers` の項目(複数の値)に、あなたのログイン名を `add`・`remove` する。すでにその状態なら何も保存しない。ログイン名は大文字と小文字を区別せずに比べ(01)、取り除くときは保存してある綴りで `remove` を書く(行の値が同じでないと、取り除きが効かないため)。アクティビティは「{ID} のウォッチを開始」「{ID} のウォッチを解除」。トーストは出さない。通知は作らない(C4)。

- [ ] **Step 1: 操作の失敗するテストを書く**

`internal/app/watching/watching_test.go`:

```go
package watching_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/watching"
	"github.com/kukv/hedgehug/internal/domain/change"
)

const store port.StoreID = "S1"

// recorder は、Write に渡したアクティビティの 1 行を覚える。
type recorder struct {
	*apptest.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func setup(t *testing.T, login string) (watching.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	if err := apptest.Sample(t.Context(), s, store); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return watching.Service{Stores: rec, Identity: apptest.Identity{Login: login}}, rec
}

func watchers(t *testing.T, s port.Stores) []string {
	t.Helper()
	var out []string
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		it, err := r.Items().Get(apptest.SampleTask)
		out = it.Watchers
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func log(t *testing.T, s port.Stores) []change.Record {
	t.Helper()
	var out []change.Record
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		var err error
		out, err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestStartAndStopWatching(t *testing.T) {
	svc, rec := setup(t, "hanako")
	// サンプルのタスクは taro がウォッチ中。
	if err := svc.Set(t.Context(), store, apptest.SampleTask, true); err != nil {
		t.Fatal(err)
	}
	if got := watchers(t, rec); !slices.Equal(got, []string{"hanako", "taro"}) {
		t.Errorf("watchers = %v", got)
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "HH-2 のウォッチを開始" {
		t.Errorf("summary = %q", got)
	}
	if err := svc.Set(t.Context(), store, apptest.SampleTask, false); err != nil {
		t.Fatal(err)
	}
	if got := watchers(t, rec); !slices.Equal(got, []string{"taro"}) {
		t.Errorf("watchers = %v", got)
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "HH-2 のウォッチを解除" {
		t.Errorf("summary = %q", got)
	}
}

func TestSettingTheStateItAlreadyHasSavesNothing(t *testing.T) {
	svc, rec := setup(t, "hanako")
	before := len(log(t, rec))
	if err := svc.Set(t.Context(), store, apptest.SampleTask, false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Set(t.Context(), store, apptest.SampleTask, true); err != nil {
		t.Fatal(err)
	}
	if err := svc.Set(t.Context(), store, apptest.SampleTask, true); err != nil {
		t.Fatal(err)
	}
	if got := len(log(t, rec)); got != before+1 {
		t.Errorf("log rows %d → %d, want one new row (the first start only)", before, got)
	}
}

// ログイン名は大文字と小文字を区別せずに比べる。解除は、保存してある綴りで書く。
func TestLoginsAreComparedWithoutCase(t *testing.T) {
	svc, rec := setup(t, "hanako")
	err := rec.Stores.Write(t.Context(), store, port.Operation{Actor: change.User("taro"), Summary: "ウォッチを足す"}, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewAdd("watchers", "Hanako"))
	})
	if err != nil {
		t.Fatal(err)
	}
	before := len(log(t, rec))
	if err := svc.Set(t.Context(), store, apptest.SampleTask, true); err != nil || len(log(t, rec)) != before {
		t.Errorf("starting while watching as Hanako wrote a row (%v)", err)
	}
	if err := svc.Set(t.Context(), store, apptest.SampleTask, false); err != nil {
		t.Fatal(err)
	}
	rows := log(t, rec)
	if last := rows[len(rows)-1]; last.T != change.Remove || string(last.From) != `"Hanako"` {
		t.Errorf("last row = %+v, want a remove of the stored spelling", last)
	}
	if got := watchers(t, rec); !slices.Equal(got, []string{"taro"}) {
		t.Errorf("watchers = %v", got)
	}
}

func TestUnknownItem(t *testing.T) {
	svc, _ := setup(t, "hanako")
	if err := svc.Set(t.Context(), store, "01NOPE00000000000000000000", true); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/watching/`
Expected: FAIL(`no non-test Go files in …/internal/app/watching`)

- [ ] **Step 3: 実装する**

`internal/app/watching/service.go`:

```go
// Package watching は、アイテムのウォッチの操作(02「ウォッチ」)を持つ。
package watching

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// Service は、ウォッチの操作が使う口。
type Service struct {
	Stores   port.Stores
	Identity port.Identity
}

// displayOf は、アクティビティの 1 行に書くアイテムの ID(HH-12)。保存の前に読む。
func (s Service) displayOf(ctx context.Context, store port.StoreID, id item.ID) (string, error) {
	var shown string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := r.Items().Get(id)
		if err != nil {
			return err
		}
		w, err := r.Workspaces().Get(it.Place.Workspace)
		if err != nil {
			return err
		}
		shown = it.DisplayID(w.Prefix)
		return nil
	})
	return shown, err
}
```

`internal/app/watching/set.go`:

```go
package watching

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// Set は、あなたがアイテムをウォッチ中かどうかを on にそろえる(02「ウォッチ」)。すでにその状態なら何も保存しない。
// アクティビティは「{ID} のウォッチを開始」「{ID} のウォッチを解除」。アイテムがなければ port.ErrNotFound。
func (s Service) Set(ctx context.Context, store port.StoreID, id item.ID, on bool) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	shown, err := s.displayOf(ctx, store, id)
	if err != nil {
		return err
	}
	verb := "解除"
	if on {
		verb = "開始"
	}
	op := port.Operation{Actor: change.User(me), Summary: shown + " のウォッチを" + verb}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		it, err := tx.Items().Get(id)
		if err != nil {
			return err
		}
		return tx.WriteItems().Append(id, drafts(it.Watchers, me, on)...)
	})
}

// drafts は、あなたのウォッチ中の状態を on にそろえる行。ログイン名は大文字と小文字を区別せずに比べ(01)、
// 取り除くときは保存してある綴りで書く(複数の値の取り除きは、値が同じでないと効かない)。
func drafts(watchers []string, me string, on bool) []change.Draft {
	var stored []string
	for _, w := range watchers {
		if strings.EqualFold(w, me) {
			stored = append(stored, w)
		}
	}
	if on {
		if len(stored) > 0 {
			return nil
		}
		return []change.Draft{change.NewAdd("watchers", me)}
	}
	out := make([]change.Draft, 0, len(stored))
	for _, w := range stored {
		out = append(out, change.NewRemove("watchers", w))
	}
	return out
}
```

- [ ] **Step 4: 通ることを確かめて、コミットする**

Run: `go test ./internal/app/watching/ ./internal/archtest/`
Expected: PASS

```bash
git add internal/app/watching
git commit -m "feat(app): ウォッチの開始と解除(A4a-14)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 5: API の失敗するテストを書く**

`internal/adapter/httpapi/items/watch_test.go`(`items_test` パッケージ。既存の `newAPI` は `items.Register` だけなので、ウォッチ用の準備を別に書く):

```go
package items_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/items"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	appwatching "github.com/kukv/hedgehug/internal/app/watching"
)

func newWatchAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	me := apptest.Identity{Login: "hanako"}
	_, api := humatest.New(t)
	svc := appitems.Service{Stores: s, IDs: ids, Identity: me, Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))}
	items.RegisterWatch(api, svc, appwatching.Service{Stores: s, Identity: me})
	return api
}

func TestSetWatch(t *testing.T) {
	api := newWatchAPI(t)
	path := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/watch"
	res := api.Put(path, map[string]any{"on": true})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"watchers":["hanako","taro"]`) {
		t.Errorf("start: %d %s", res.Code, res.Body)
	}
	res = api.Put(path, map[string]any{"on": false})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"watchers":["taro"]`) {
		t.Errorf("stop: %d %s", res.Code, res.Body)
	}
	if res := api.Put(path, map[string]any{"on": false}); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"watchers":["taro"]`) {
		t.Errorf("stop again: %d %s", res.Code, res.Body)
	}
	if res := api.Put("/api/stores/S1/items/01NOPE00000000000000000000/watch", map[string]any{"on": true}); res.Code != http.StatusNotFound {
		t.Errorf("unknown item: %d", res.Code)
	}
}
```

- [ ] **Step 6: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/items/ -run SetWatch`
Expected: FAIL(ビルドエラー `undefined: items.RegisterWatch`)

- [ ] **Step 7: 実装する**

`internal/adapter/httpapi/items/watch.go`:

```go
package items

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	appwatching "github.com/kukv/hedgehug/internal/app/watching"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// SetWatch は、ウォッチの入り切りの本文。
type SetWatch struct {
	On bool `json:"on" doc:"あなたがウォッチ中にするか"`
}

type watchInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  SetWatch
}

// RegisterWatch は、ウォッチの API を登録する。ウォッチを書くのは watching、返すアイテムは items が読む。
func RegisterWatch(api huma.API, svc appitems.Service, w appwatching.Service) {
	huma.Register(api, huma.Operation{OperationID: "set-watch", Method: http.MethodPut, Path: "/api/stores/{store}/items/{item}/watch", Summary: "あなたのウォッチを入り切りする"},
		func(ctx context.Context, in *watchInput) (*oneOutput, error) {
			store, id := port.StoreID(in.Store), item.ID(in.Item)
			if err := w.Set(ctx, store, id, in.Body.On); err != nil {
				return nil, apierr.From(err)
			}
			return one(svc.Get(ctx, store, id))
		})
}
```

`internal/adapter/httpapi/routes.go`: import に `appwatching "github.com/kukv/hedgehug/internal/app/watching"` を足し、アイテムの行を次にする(同じ `appitems.Service` を 2 か所で使うため、変数に取る)。

```go
	itemsSvc := appitems.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity, Clock: deps.Clock}
	items.Register(api, itemsSvc)
	items.RegisterWatch(api, itemsSvc, appwatching.Service{Stores: deps.Stores, Identity: deps.Identity})
```

- [ ] **Step 8: 生成して、通ることを確かめて、コミットする**

Run: `make generate && pnpm -C web gen`
Expected: `api/openapi.yaml` と `generated.ts` に `setWatch`(`PUT …/items/{item}/watch`)とスキーマ `SetWatch` が出る。

Run: `go test ./internal/... && pnpm -C web check`
Expected: PASS

```bash
git add internal api web/packages/api/src/generated.ts
git commit -m "feat(api): ウォッチの入り切りの API を足す(A4a-14)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 9: 画面の口と状態の失敗するテストを書く**

`web/packages/api/src/notes.test.ts` に足す(Task 7 が作ったファイルの `respond`・`call` を使う。`NotFound` は Task 10 が import に足している。`apiItem` は `ports.test.ts` の `apiItem` と同じ形のアイテムの応答を、このファイルにも置く)。

```ts
describe("notes port: watch", () => {
  const apiItem = {
    id: "I1",
    workspace: "W1",
    project: "P1",
    parent: null,
    number: 13,
    kind: "Epic",
    rank: "V",
    title: "認証",
    watchers: ["hanako", "taro"],
    createdAt: "2026-11-13T09:00:00.000Z",
    createdBy: "taro",
    status: "st1",
    priority: "pr2",
    assignee: "",
    start: "",
    end: "",
    points: null,
    estimate: null,
    systems: [],
    choices: {},
    fields: {},
  };

  it("sends the state and returns the item", async () => {
    const fetch = respond(200, apiItem);
    const it = await createNotesPort().setWatch("S1", "I1", true);
    expect(it).toMatchObject({ id: "I1", storeId: "S1", watchers: ["hanako", "taro"] });
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/watch");
    expect(call(fetch)[1].method).toBe("PUT");
    expect(JSON.parse(String(call(fetch)[1].body))).toEqual({ on: true });
  });

  it("turns a 404 into NotFound", async () => {
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    await expect(createNotesPort().setWatch("S1", "I1", false)).rejects.toBeInstanceOf(NotFound);
  });
});
```

`web/packages/core/src/features/notes/watch.test.ts`:

```ts
import { describe, expect, it, vi } from "vitest";
import { Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import { get } from "../../reactive/store";
import { item as makeItem } from "../items/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createWatch, watchView } from "./watch";

const watched = (watchers: string[]): Item => ({
  ...makeItem("S1", "S1-P1", 2, "Task", "S1-I1", "画面"),
  watchers,
});

describe("watchView", () => {
  it("shows which state it is in, without caring about case", () => {
    expect(watchView(watched(["taro"]), "taro", false)).toEqual({
      watching: true,
      label: "ウォッチ中",
      disabled: false,
      note: undefined,
    });
    expect(watchView(watched(["Taro"]), "taro", false).watching).toBe(true);
    expect(watchView(watched(["hanako"]), "taro", false)).toMatchObject({ watching: false, label: "ウォッチ" });
  });

  it("is disabled in a read-only Store, with the note, and when me is not decided", () => {
    expect(watchView(watched([]), "taro", true)).toMatchObject({ disabled: true, note: "閲覧のみの Store です" });
    expect(watchView(watched(["taro"]), "", false)).toMatchObject({ watching: false, disabled: true });
  });
});

function setup() {
  const notes = { setWatch: vi.fn(async (_s: string, _i: string, _on: boolean) => watched([])) };
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const changed = vi.fn();
  const watch = createWatch({ notes, toasts, onChanged: changed });
  return { watch, notes, toasts, changed };
}

describe("createWatch", () => {
  it("starts watching when not watching, then reloads, with no toast", async () => {
    const { watch, notes, toasts, changed } = setup();
    await watch.toggle(watched(["hanako"]), "taro");
    expect(notes.setWatch).toHaveBeenCalledWith("S1", "S1-I2", true);
    expect(changed).toHaveBeenCalledOnce();
    expect(get(toasts.list)).toEqual([]);
  });

  it("stops watching when watching, whatever the case of the login", async () => {
    const { watch, notes } = setup();
    await watch.toggle(watched(["Taro"]), "taro");
    expect(notes.setWatch).toHaveBeenCalledWith("S1", "S1-I2", false);
  });

  it("ignores a second press while sending, and when me is not decided", async () => {
    const { watch, notes } = setup();
    const first = watch.toggle(watched([]), "taro");
    expect(get(watch.busy)).toBe(true);
    await watch.toggle(watched([]), "taro");
    await first;
    expect(notes.setWatch).toHaveBeenCalledOnce();
    expect(get(watch.busy)).toBe(false);
    await watch.toggle(watched([]), "");
    expect(notes.setWatch).toHaveBeenCalledOnce();
  });

  it("shows a refusal in a toast and does not reload", async () => {
    const { watch, notes, toasts, changed } = setup();
    notes.setWatch.mockRejectedValueOnce(new Refused("断り"));
    await watch.toggle(watched([]), "taro");
    expect(get(toasts.list)[0]).toMatchObject({ text: "断り", error: true });
    expect(changed).not.toHaveBeenCalled();
    expect(get(watch.busy)).toBe(false);
  });
});
```

- [ ] **Step 10: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/api test -- notes` と `pnpm -C web --filter @hedgehug/core test -- watch`
Expected: FAIL(api は `setWatch is not a function`、core は `Failed to resolve import "./watch"`)

- [ ] **Step 11: 実装する**

`web/packages/core/src/port/notes.ts` の `NotesPort` に足す(`import type { Item } from "../model/item";` を足す)。

```ts
  // setWatch は、あなたのウォッチを入り切りして、変えたあとのアイテムを返す。
  setWatch(storeId: string, itemId: string, on: boolean): Promise<Item>;
```

`fakeNotesPort`(`fixtures.test-util.ts`)にも、型を合わせるスタブを足す。

```ts
    async setWatch() {
      throw new Error("setWatch is not faked");
    },
```

`web/packages/api/src/notes.ts` に足す(`import { toItem } from "./items";` と、生成された `setWatch` の import を足す)。

```ts
    setWatch: (storeId, itemId, on) =>
      call(async () => toItem(storeId, await setWatch(storeId, itemId, { on }))),
```

`web/packages/core/src/features/notes/watch.ts`:

```ts
import { Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import type { NotesPort } from "../../port/notes";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import { readOnlyNote } from "../catalog/readonly";
import type { Toasts } from "../toasts/toasts";

// ヘッダーのウォッチのボタン(02「ウォッチ」)。あなたがそのアイテムをウォッチ中かどうかを入り切りする。

export interface WatchView {
  watching: boolean;
  // label は、いまどちらかが分かる表示。
  label: string;
  disabled: boolean;
  // note は、無効にしたときの案内(閲覧のみの Store。ポインターを置いたときに出す)。
  note: string | undefined;
}

// watchView は、ボタンの表示。ログイン名は大文字と小文字を区別せずに比べる(01)。
// あなたが決まらないとき(me が空)は、ウォッチ中にならず、押せない。
export function watchView(item: Pick<Item, "watchers">, me: string, readOnly: boolean): WatchView {
  const watching = me !== "" && item.watchers.some((w) => w.toLowerCase() === me.toLowerCase());
  return {
    watching,
    label: watching ? "ウォッチ中" : "ウォッチ",
    disabled: readOnly || me === "",
    note: readOnly ? readOnlyNote : undefined,
  };
}

export interface Watch {
  // busy は、送信中(二重に押さない)。
  busy: Readable<boolean>;
  // toggle は、ウォッチの状態を逆にする。トーストは出さない。断られたときだけ、エラーのトーストを出す。
  toggle(item: Item, me: string): Promise<void>;
}

export interface WatchDeps {
  notes: Pick<NotesPort, "setWatch">;
  toasts: Toasts;
  // onChanged は、保存したあと(開いている詳細パネルを読み直す)。
  onChanged(): void;
}

export function createWatch(deps: WatchDeps): Watch {
  const busy = writable(false);
  return {
    busy: readonly(busy),
    async toggle(item, me) {
      if (get(busy) || me === "") return;
      const on = !watchView(item, me, false).watching;
      busy.set(true);
      try {
        await deps.notes.setWatch(item.storeId, item.id, on);
      } catch (e) {
        if (!(e instanceof Refused)) throw e;
        deps.toasts.show(e.message, { error: true });
        return;
      } finally {
        busy.set(false);
      }
      deps.onChanged();
    },
  };
}
```

`web/packages/core/src/notesIndex.ts` に足す(`index.ts` は変えない)。

```ts
export { createWatch, type Watch, type WatchDeps, type WatchView, watchView } from "./features/notes/watch";
```

- [ ] **Step 12: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/api test -- notes && pnpm -C web --filter @hedgehug/core test -- watch && pnpm -C web check`
Expected: PASS

- [ ] **Step 13: ヘッダーのボタンを作る**

`web/app/src/screens/item/WatchButton.svelte`:

```svelte
<script lang="ts">
  import { type DetailView, watchView } from "@hedgehug/core";
  import { Button } from "@hedgehug/ui";
  import { watch } from "../../wire";

  // ヘッダーのウォッチのボタン(02「ウォッチ」)。いまウォッチ中かどうかが、文字と印で分かる。
  interface Props {
    v: DetailView;
  }

  let { v }: Props = $props();

  const view = $derived(watchView(v.item, v.people.me, v.readOnly));
  const busy = watch.busy;
</script>

<Button size="sm" disabled={view.disabled || $busy} title={view.note} onclick={() => void watch.toggle(v.item, v.people.me)}>
  <span aria-hidden="true">{view.watching ? "★" : "☆"}</span>
  {view.label}
</Button>
```

`web/app/src/screens/item/DetailHeader.svelte`: import を足し、全画面のボタンの前に置く。コメントの「ウォッチと、メニューのほかの項目は A4。」は「メニューのほかの項目は A4b。」にする。

```svelte
  import WatchButton from "./WatchButton.svelte";
```

```svelte
<span class="grow"></span>
<WatchButton {v} />
<IconButton label={full ? "パネルで表示" : "全画面で表示"} size="sm" onclick={detail.toggleFull}
```

`web/app/src/wire.ts`: `createWatch` を core の import に足し、`notes`(Task 7 が作る `NotesPort`)を使って、`detail` の定義のあとに足す。

```ts
export const watch = createWatch({
  notes,
  toasts,
  // ウォッチを変えたら、開いている詳細パネルを読み直す(ウォッチ中の人の一覧が変わる)。
  onChanged: () => void detail.reload(),
});
```

- [ ] **Step 14: 確かめる**

Run: `pnpm -C web fix && pnpm -C web check && pnpm -C web test && pnpm -C web lint && pnpm -C web deps && pnpm -C web size`
Expected: すべて PASS(`DetailHeader.svelte` は 200 行以下)

画面の動き(ボタンの表示の切り替わり、閲覧のみの Store で無効、ポインターを置いたときの案内)は、Task 22 の e2e で通しで確かめる。

- [ ] **Step 15: コミット**

```bash
git add web
git commit -m "feat(web): 詳細パネルのヘッダーにウォッチのボタンを足す(A4a-14)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 16: 履歴の文と API(アイテム・プロジェクト、B2-9)

アイテムとプロジェクトのログから、履歴の行(操作した人・文・時刻)を作り、API で返す。行の文はサーバーが作る(A4a-15)。画面は Task 17。

**この Task が決めること**(決定 A4a-15〜17 の細部):

- 並びは新しい順。時刻が同じ行(1 回の保存で書いた複数の行)は、ログの後ろにある行を先にする。
- アイテムの行は、`create`=「作成」、`desc`=「説明を変更」、`set` のうち次の 4 つ。
  - `title`: 「タイトルを「{新}」に変更」
  - `kind`: 「種類を {旧} から {新} に変更」
  - `place`: 親かプロジェクトが変わっているときだけ「親を変更」(番号だけが変わる `place` の行は、同期の修復が書くので出さない)
  - プロパティ(組み込みは項目名がキー、追加は `fields/<内部 ID>`): 「{プロパティ名}を {旧} → {新}」。値は `item.Display`(空は「なし」、消えた値は「(削除された値)」、日付は今年なら M/D)。プロパティが設定にない(消した)ものは、名前が分からないので出さない。
  - 関連システムと複数選択のプロパティの `add`・`remove`: 「{プロパティ名}を {旧} → {新}」(裁定済み)。旧は、ログを先頭からその行の直前までで、新は、その行までで、それぞれ `item.FromLog` が決めた一覧(`prev` のグラフの規則がそのまま効く)から `item.Current` で作る。旧と新が同じ(結果が変わらない行)は出さない。関連(`relations`)・ウォッチ(`watchers`)・リンク・リポジトリの `add`・`remove` は、プロパティでないので出さない。
  - `rank`(順位)・`delete`・`comment`・`worklog`・知らない項目の `set` は出さない。
- ワークスペースのログに同じ `op` の `delete` の行があるアイテムの行は出さない(A4a-16。B2-9)。
- 値の名前は、いまのワークスペースの設定で引く(A4a-17)。保存形式の「行を書いたときのワークスペースの定義で引く」とは違うが、移動(別のワークスペースへ)は A4b で、それまで起きない。
- 操作した人は、`actor` が自動化=「自動化」、GitHub=「GitHub」、同期=「同期」、人=メンバーの表示名(いなければ「不明なユーザー」)。`none` は仕様にないので「不明なユーザー」とする。
- プロジェクトの行は、`create`=「作成」、`desc`=「説明を変更」、`set` の `title`・`status`・`owner`・`start`・`end`=「{項目名}を {値} に変更」(項目名は「タイトル」「ステータス」「オーナー」「開始日」「終了日」、値は、ステータスは名前のまま、オーナーは人の名前、日付は空なら「なし」)。ほかの行(移動・復元・場所・順位)は、その行を書くトラックが文を足す。
- 同期で受け取った行の「同期で受信」(C4)と、GitHub のリンクの行(D)は出さない。

**Files:**
- Create: `internal/domain/history/history.go`(`Row`・`Context`・`HiddenOps`・`Who`・`NamesOf`)・`item.go`(`ItemRows`)・`project.go`(`ProjectRows`)
- Test: `internal/domain/history/history_test.go`・`project_test.go`
- Create: `internal/app/history/service.go`
- Test: `internal/app/history/history_test.go`
- Create: `internal/adapter/httpapi/notes/history.go`
- Test: `internal/adapter/httpapi/notes/history_test.go`
- Modify: `internal/adapter/httpapi/notes/`(`Services` に `History`、`Register` に `registerHistory(api, s.History)`)・`internal/adapter/httpapi/routes.go`
- Generated: `api/openapi.yaml`・`web/packages/api/src/generated.ts`

**Interfaces:**
- Consumes:
  - `change.Desc`(Task 2)、`change.Create`・`Set`・`Delete`、`change.Record`・`Actor`(`ActorUser`・`ActorAutomation`・`ActorGitHub`・`ActorSync`・`ActorNone`)
  - `item.FromLog(log) (Item, bool)`、`item.Current(it Item, p workspace.Property) json.RawMessage`
  - `item.Display(p workspace.Property, v json.RawMessage, st workspace.Settings, names item.Names, today time.Time) string`、`item.Names`、`item.FieldKey(id)`、`item.Place`
  - `workspace.Settings.Property(id)`、`workspace.DefaultSettings`
  - `port.Reader` の `Items().Get`・`Projects().Get`・`Workspaces().Settings`・`Members().List`・`Changes().Of`、`port.Clock`
- Produces:
  - `history.Row{Who, Text string; At time.Time}`
  - `history.Context{Settings workspace.Settings; Names item.Names; Hidden map[change.ID]bool; Today time.Time}`、`history.ProjectContext{Names item.Names; Today time.Time}`
  - `func HiddenOps(workspaceLog []change.Record) map[change.ID]bool`
  - `func Who(a change.Actor, names item.Names) string`、`func NamesOf(ms []member.Member) item.Names`
  - `func ItemRows(log []change.Record, c Context) []Row`、`func ProjectRows(log []change.Record, c ProjectContext) []Row`(どちらも新しい順)
  - `history.Service{Stores port.Stores; Clock port.Clock}`(app)と `Item(ctx, store, id item.ID) ([]history.Row, error)`・`Project(ctx, store, id project.ID) ([]history.Row, error)`
  - HTTP: `get-item-history`・`get-project-history` → `HistoryRow{who, text, at}[]`(新しい順。`at` は `change.TimeLayout`)

- [ ] **Step 1: domain の失敗するテストを書く**

`internal/domain/history/history_test.go`(新規):

```go
package history_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func at(h int) time.Time { return time.Date(2026, 11, 13, 9+h, 0, 0, 0, time.UTC) }

type step struct {
	op    change.ID
	at    time.Time
	actor change.Actor
	d     change.Draft
}

// logOf は、steps を順に 1 行ずつ書いたログ。
func logOf(steps ...step) []change.Record {
	var log []change.Record
	n := 0
	for _, s := range steps {
		log = append(log, change.Stamp(log, []change.Draft{s.d}, change.Meta{Op: s.op, At: s.at, Actor: s.actor}, func() change.ID {
			n++
			return change.ID(fmt.Sprintf("R%03d", n))
		})...)
	}
	return log
}

// settings は、既定の設定。値の内部 ID は、ステータス V01〜V05(Todo・InProgress・Done・Pending・Cancel)、
// 優先度 V06〜V10、関連システム V11〜V16。
func settings() workspace.Settings {
	n := 0
	return workspace.DefaultSettings(func() change.ID { n++; return change.ID(fmt.Sprintf("V%02d", n)) })
}

func names(login string) string {
	if login == "taro" {
		return "太郎"
	}
	return "不明なユーザー"
}

func texts(rows []history.Row) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Who+": "+r.Text)
	}
	return out
}

func ctx() history.Context {
	return history.Context{Settings: settings(), Names: names, Today: at(0)}
}

var (
	taro   = change.User("taro")
	hanako = change.User("hanako")
)

func TestItemRowsNewestFirst(t *testing.T) {
	parent := item.ID("E1")
	other := item.ID("E2")
	base := item.Item{ID: "T", Place: item.Place{Workspace: "W", Project: "P", Parent: &parent, Number: 2}, Kind: item.Task, Rank: "V", Title: "ログイン", Status: "V01"}
	moved := base.Place
	moved.Parent = &other
	renumbered := base.Place
	renumbered.Number = 9
	log := logOf(
		step{"O1", at(0), taro, item.Create(base)},
		step{"O2", at(1), taro, change.NewSet("status", "V01", "V02")},
		step{"O3", at(2), hanako, change.NewSet("title", "ログイン", "ログイン画面")},
		step{"O4", at(3), taro, change.NewDesc()},
		step{"O5", at(4), change.Actor{Kind: change.ActorAutomation, Login: "taro", Rule: "RULE1"}, change.NewSet("kind", "Task", "Bug")},
		step{"O6", at(5), change.Actor{Kind: change.ActorSync}, change.NewSet("place", base.Place, moved)},
		step{"O7", at(6), taro, change.NewSet("place", base.Place, renumbered)},
		step{"O8", at(7), taro, change.NewSet("rank", "V", "W")},
		step{"O9", at(8), taro, change.NewSet("points", nil, 5)},
		step{"O10", at(9), taro, change.NewSet("assignee", nil, "taro")},
		step{"O11", at(10), taro, change.NewSet("start", nil, "2026-11-20")},
		step{"O12", at(11), taro, change.NewSet("status", "V02", "V99")},
		step{"O13", at(12), taro, change.NewAdd("watchers", "hanako")},
		step{"O14", at(13), taro, change.Draft{T: change.Type("comment"), F: "C1", To: change.Value(map[string]any{"body": "確認"})}},
		step{"O15", at(14), change.Actor{Kind: change.ActorGitHub}, change.NewSet("links/pr:org/web#1/state", "open", "merged")},
		step{"O16", at(15), taro, change.NewSet("fields/01GONE0000000000000000001", nil, "x")},
	)
	rows := history.ItemRows(log, ctx())
	want := []string{
		"太郎: ステータスを InProgress → (削除された値)",
		"太郎: 開始日を なし → 11/20",
		"太郎: 担当者を なし → 太郎",
		"太郎: ポイントを なし → 5",
		"同期: 親を変更",
		"自動化: 種類を Task から Bug に変更",
		"太郎: 説明を変更",
		"不明なユーザー: タイトルを「ログイン画面」に変更",
		"太郎: ステータスを Todo → InProgress",
		"太郎: 作成",
	}
	if got := texts(rows); !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
	if !rows[0].At.Equal(at(11)) || !rows[len(rows)-1].At.Equal(at(0)) {
		t.Errorf("At = %v ... %v", rows[0].At, rows[len(rows)-1].At)
	}
}

// 関連システムの追加・取り除きは「旧 → 新」で出す。旧と新は、その行の直前までと、その行までのログから決める。
// 同時の変更(prev が同じ)は、後勝ちの規則で決まった一覧になる。
func TestItemRowsOfASystemsListIncludingAConcurrentChange(t *testing.T) {
	st := settings()
	web, api := st.Systems[0], st.Systems[1]
	base := item.Item{ID: "T", Place: item.Place{Workspace: "W", Project: "P", Number: 1}, Kind: item.Task, Rank: "V", Title: "画面", Systems: []change.ID{web.ID}}
	log := logOf(
		step{"O1", at(0), taro, item.Create(base)},
		step{"O2", at(1), taro, change.NewAdd("systems", api.ID)}, // R002。prev は作成の R001
		step{"O9", at(5), taro, change.NewAdd("relations", item.Relation{Type: "blocks", To: "X"})}, // 出さない
	)
	// hanako が、taro の追加を見ずに(prev が作成だけ)api を取り除く。時刻はあと。
	log = append(log, change.Record{
		ID: "R100", Op: "O3", At: at(2), By: "hanako", Actor: hanako, T: change.Remove, F: "systems",
		From: change.Value(api.ID), Prev: []change.ID{"R001"},
	})
	got := texts(history.ItemRows(log, ctx()))
	want := []string{
		// 同時の取り除きが後勝ちで、api は外れる。旧は、見えていた [web, api]。
		fmt.Sprintf("不明なユーザー: 関連システムを %s, %s → %s", web.Name, api.Name, web.Name),
		fmt.Sprintf("太郎: 関連システムを %s → %s, %s", web.Name, web.Name, api.Name),
		"太郎: 作成",
	}
	if !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
	// 同じ一覧になる行(すでにある値の追加)は出さない。
	log = append(log, change.Stamp(log, []change.Draft{change.NewAdd("systems", web.ID)}, change.Meta{Op: "O4", At: at(3), Actor: taro}, func() change.ID { return "R200" })...)
	if got := texts(history.ItemRows(log, ctx())); len(got) != len(want) {
		t.Errorf("結果が変わらない追加は出さない: %q", got)
	}
}

func TestItemRowsOfOneSaveShowLaterRowsFirst(t *testing.T) {
	base := item.Item{ID: "T", Place: item.Place{Workspace: "W", Project: "P", Number: 1}, Kind: item.Epic, Rank: "V", Title: "認証", Status: "V01"}
	log := logOf(
		step{"O1", at(0), taro, item.Create(base)},
		step{"O2", at(1), taro, change.NewSet("title", "認証", "認可")},
		step{"O2", at(1), taro, change.NewSet("status", "V01", "V02")},
	)
	want := []string{"太郎: ステータスを Todo → InProgress", "太郎: タイトルを「認可」に変更", "太郎: 作成"}
	if got := texts(history.ItemRows(log, ctx())); !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

// 値やプロパティの削除と同じ保存で書いたアイテムの行は出さない(A4a-16。B2-9)。
func TestItemRowsHideTheRowsOfAValueDeletion(t *testing.T) {
	base := item.Item{ID: "T", Place: item.Place{Workspace: "W", Project: "P", Number: 1}, Kind: item.Task, Rank: "V", Title: "画面", Status: "V05"}
	log := logOf(
		step{"O1", at(0), taro, item.Create(base)},
		step{"O2", at(1), taro, change.NewSet("status", "V05", "V01")},
		step{"O3", at(2), taro, change.NewSet("title", "画面", "画面 2")},
	)
	wsLog := logOf(
		step{"W1", at(0), taro, change.NewCreate(map[string]any{"id": "W", "name": "Web", "prefix": "HH"})},
		step{"O2", at(1), taro, change.NewDelete("statuses/V05")},
	)
	hidden := history.HiddenOps(wsLog)
	if !hidden["O2"] || hidden["W1"] || len(hidden) != 1 {
		t.Fatalf("HiddenOps = %v", hidden)
	}
	c := ctx()
	c.Hidden = hidden
	want := []string{"太郎: タイトルを「画面 2」に変更", "太郎: 作成"}
	if got := texts(history.ItemRows(log, c)); !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

func TestWho(t *testing.T) {
	cases := []struct {
		actor change.Actor
		want  string
	}{
		{taro, "太郎"},
		{hanako, "不明なユーザー"},
		{change.Actor{Kind: change.ActorAutomation, Login: "taro", Rule: "R"}, "自動化"},
		{change.Actor{Kind: change.ActorGitHub}, "GitHub"},
		{change.Actor{Kind: change.ActorSync}, "同期"},
		{change.Actor{Kind: change.ActorNone}, "不明なユーザー"},
	}
	for _, c := range cases {
		if got := history.Who(c.actor, names); got != c.want {
			t.Errorf("Who(%+v) = %q, want %q", c.actor, got, c.want)
		}
	}
}

func TestNamesOfIgnoresCaseAndFallsBack(t *testing.T) {
	n := history.NamesOf([]member.Member{{Login: "Taro", Name: "太郎"}})
	if n("taro") != "太郎" || n("TARO") != "太郎" || n("jiro") != "不明なユーザー" {
		t.Errorf("NamesOf: %q %q %q", n("taro"), n("TARO"), n("jiro"))
	}
}
```

`internal/domain/history/project_test.go`(新規):

```go
package history_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/project"
)

func TestProjectRows(t *testing.T) {
	base := project.Project{ID: "P", Place: project.Place{Workspace: "W", Number: 1}, Rank: "V", Title: "ログイン", Owner: "", Status: project.Todo}
	log := logOf(
		step{"O1", at(0), taro, project.Create(base)},
		step{"O2", at(1), taro, change.NewSet("title", "ログイン", "認証")},
		step{"O3", at(2), taro, change.NewSet("status", "Todo", "InProgress")},
		step{"O4", at(3), hanako, change.NewSet("owner", "", "taro")},
		step{"O5", at(4), taro, change.NewSet("owner", "taro", "")},
		step{"O6", at(5), taro, change.NewSet("start", "", "2026-12-01")},
		step{"O7", at(6), taro, change.NewSet("end", "2026-12-01", "")},
		step{"O8", at(7), taro, change.NewDesc()},
		step{"O9", at(8), taro, change.NewSet("place", base.Place, project.Place{Workspace: "W2", Number: 3})},
		step{"O10", at(9), taro, change.NewSet("rank", "V", "W")},
		step{"O11", at(10), taro, change.Draft{T: change.Type("comment"), F: "C1", To: change.Value(map[string]any{"body": "確認"})}},
		step{"O12", at(11), taro, change.NewAdd("repos", "org/web")},
	)
	rows := history.ProjectRows(log, history.ProjectContext{Names: names, Today: at(0)})
	want := []string{
		"太郎: 説明を変更",
		"太郎: 終了日を なし に変更",
		"太郎: 開始日を 12/1 に変更",
		"太郎: オーナーを なし に変更",
		"不明なユーザー: オーナーを 太郎 に変更",
		"太郎: ステータスを InProgress に変更",
		"太郎: タイトルを 認証 に変更",
		"太郎: 作成",
	}
	if got := texts(rows); !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/history/`
Expected: FAIL(`no non-test Go files` または `undefined: history.ItemRows`)

- [ ] **Step 3: domain を実装する**

`internal/domain/history/history.go`(新規):

```go
// Package history は、アイテムとプロジェクトの履歴(02「履歴」)の行を、ログから作る。
// 文は保存形式「アクティビティと履歴」の行の種類ごとに決め、知らない種類の行は出さない(その行を書くトラックが足す)。
package history

import (
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Row は、履歴の 1 行。
type Row struct {
	Who  string
	Text string
	At   time.Time
}

// Context は、アイテムの履歴の文を作る材料。
type Context struct {
	Settings workspace.Settings // いまのワークスペースの設定。値の名前をここで引く(A4a-17)
	Names    item.Names
	Hidden   map[change.ID]bool // 出さない操作(HiddenOps)
	Today    time.Time          // 日付の年を決める
}

const unknownUser = "不明なユーザー"

// HiddenOps は、ワークスペースのログの delete の行の操作(op)。値やプロパティの削除と同じ保存で書いた
// アイテムの行は、履歴に出さない(A4a-16。B2-9)。
func HiddenOps(workspaceLog []change.Record) map[change.ID]bool {
	out := map[change.ID]bool{}
	for _, r := range workspaceLog {
		if r.T == change.Delete {
			out[r.Op] = true
		}
	}
	return out
}

// Who は、操作した人の表示(02「履歴」)。人はメンバーの表示名で、メンバーにいなければ「不明なユーザー」。
func Who(a change.Actor, names item.Names) string {
	switch a.Kind {
	case change.ActorAutomation:
		return "自動化"
	case change.ActorGitHub:
		return "GitHub"
	case change.ActorSync:
		return "同期"
	case change.ActorUser:
		return names(a.Login)
	}
	return unknownUser
}

// NamesOf は、メンバーの一覧から、ログイン名(大文字小文字を区別しない)で表示名を引く関数を作る。
func NamesOf(ms []member.Member) item.Names {
	byLogin := make(map[string]string, len(ms))
	for _, m := range ms {
		byLogin[strings.ToLower(m.Login)] = m.Name
	}
	return func(login string) string {
		if n, ok := byLogin[strings.ToLower(login)]; ok {
			return n
		}
		return unknownUser
	}
}

// newestFirst は、新しい順に並べる。時刻が同じ行(1 回の保存で書いた行)は、ログの後ろにあるものを先にする。
func newestFirst(rows []Row) []Row {
	slices.Reverse(rows)
	slices.SortStableFunc(rows, func(a, b Row) int { return b.At.Compare(a.At) })
	return rows
}
```

`internal/domain/history/item.go`(新規):

```go
package history

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ItemRows は、アイテムの履歴を新しい順で返す(02「履歴」の表)。出すのは、作成・説明の変更・タイトル・種類・親・
// プロパティの変更(関連システムと複数選択の追加・取り除きを含む)だけで、コメント・作業時間・関連・ウォッチ・削除と、知らない種類の行は出さない。
func ItemRows(log []change.Record, c Context) []Row {
	var rows []Row
	for i, r := range log {
		if c.Hidden[r.Op] {
			continue
		}
		if text, ok := itemText(log, i, c); ok {
			rows = append(rows, Row{Who: Who(r.Actor, c.Names), Text: text, At: r.At})
		}
	}
	return newestFirst(rows)
}

func itemText(log []change.Record, i int, c Context) (string, bool) {
	r := log[i]
	switch r.T {
	case change.Create:
		return "作成", true
	case change.Desc:
		return "説明を変更", true
	case change.Set:
		return setText(r, c)
	case change.Add, change.Remove:
		return listText(log, i, c)
	}
	return "", false
}

// listText は、関連システムと複数選択の追加・取り除きを「{プロパティ名}を {旧} → {新}」にする。旧は log[:i]、
// 新は log[:i+1] から決めた一覧なので、同時の変更も、prev のグラフの規則で決まった結果になる。
// プロパティでない項目(関連・ウォッチなど)と、一覧が変わらない行は出さない。
func listText(log []change.Record, i int, c Context) (string, bool) {
	p, ok := propertyOf(log[i].F, c.Settings)
	if !ok || (p.Type != workspace.TypeSystems && p.Type != workspace.TypeMulti) {
		return "", false
	}
	before, okBefore := item.FromLog(log[:i])
	after, okAfter := item.FromLog(log[:i+1])
	if !okBefore || !okAfter {
		return "", false
	}
	from, to := item.Current(before, p), item.Current(after, p)
	if bytes.Equal(from, to) {
		return "", false
	}
	return fmt.Sprintf("%sを %s → %s", p.Name, item.Display(p, from, c.Settings, c.Names, c.Today), item.Display(p, to, c.Settings, c.Names, c.Today)), true
}

func setText(r change.Record, c Context) (string, bool) {
	switch r.F {
	case "title":
		var to string
		if json.Unmarshal(r.To, &to) != nil {
			return "", false
		}
		return fmt.Sprintf("タイトルを「%s」に変更", to), true
	case "kind":
		var from, to string
		if json.Unmarshal(r.From, &from) != nil || json.Unmarshal(r.To, &to) != nil {
			return "", false
		}
		return fmt.Sprintf("種類を %s から %s に変更", from, to), true
	case "place":
		return placeText(r)
	case "rank":
		return "", false
	}
	p, ok := propertyOf(r.F, c.Settings)
	if !ok {
		return "", false
	}
	from := item.Display(p, r.From, c.Settings, c.Names, c.Today)
	to := item.Display(p, r.To, c.Settings, c.Names, c.Today)
	return fmt.Sprintf("%sを %s → %s", p.Name, from, to), true
}

// placeText は、親かプロジェクトが変わった場所の変更を「親を変更」にする。番号だけが変わる行
// (同期の修復)は出さない。
func placeText(r change.Record) (string, bool) {
	var from, to item.Place
	if json.Unmarshal(r.From, &from) != nil || json.Unmarshal(r.To, &to) != nil {
		return "", false
	}
	sameParent := (from.Parent == nil && to.Parent == nil) ||
		(from.Parent != nil && to.Parent != nil && *from.Parent == *to.Parent)
	if sameParent && from.Project == to.Project {
		return "", false
	}
	return "親を変更", true
}

// propertyOf は、項目名 f が指すプロパティ。組み込みは項目名がキー(= 内部 ID)、追加は fields/<内部 ID>。
// 設定にない(消えた)プロパティは、名前が分からないので ok=false。
func propertyOf(f string, st workspace.Settings) (workspace.Property, bool) {
	id := change.ID(strings.TrimPrefix(f, item.FieldKey("")))
	return st.Property(id)
}
```

`internal/domain/history/project.go`(新規):

```go
package history

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ProjectContext は、プロジェクトの履歴の文を作る材料。
type ProjectContext struct {
	Names item.Names
	Today time.Time
}

// projectFields は、履歴に出すプロジェクトの項目(項目名 → 画面の名前)。
var projectFields = map[string]string{
	"title": "タイトル", "status": "ステータス", "owner": "オーナー", "start": "開始日", "end": "終了日",
}

// ProjectRows は、プロジェクトの履歴を新しい順で返す(02「概要の編集」の履歴)。作成・説明の変更・項目の変更だけ。
// 移動と復元は、その行を書くトラックが足す。
func ProjectRows(log []change.Record, c ProjectContext) []Row {
	var rows []Row
	for _, r := range log {
		if text, ok := projectText(r, c); ok {
			rows = append(rows, Row{Who: Who(r.Actor, c.Names), Text: text, At: r.At})
		}
	}
	return newestFirst(rows)
}

func projectText(r change.Record, c ProjectContext) (string, bool) {
	switch r.T {
	case change.Create:
		return "作成", true
	case change.Desc:
		return "説明を変更", true
	case change.Set:
		if name, ok := projectFields[r.F]; ok {
			return fmt.Sprintf("%sを %s に変更", name, projectValue(r.F, r.To, c)), true
		}
	}
	return "", false
}

// projectValue は、{値}。ステータスと題名は名前のまま、オーナーは人の名前、日付は 01 の表示。空は「なし」。
func projectValue(f string, to json.RawMessage, c ProjectContext) string {
	var s string
	_ = json.Unmarshal(to, &s)
	switch f {
	case "owner":
		if s == "" {
			return "なし"
		}
		return c.Names(s)
	case "start", "end":
		if s == "" {
			return "なし"
		}
		return item.Display(workspace.Property{Type: workspace.TypeDate}, to, workspace.Settings{}, c.Names, c.Today)
	}
	return s
}
```

Run: `go test ./internal/domain/history/` → PASS
Run: `go vet ./internal/domain/history/` → 出力なし

- [ ] **Step 4: コミット(domain)**

```bash
git add internal/domain/history
git commit -m "feat(domain): アイテムとプロジェクトの履歴の行を作る(A4a-15〜17)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 5: app の失敗するテストを書く**

`internal/app/history/history_test.go`(新規):

```go
package history_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	apphistory "github.com/kukv/hedgehug/internal/app/history"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	dhistory "github.com/kukv/hedgehug/internal/domain/history"
)

const store port.StoreID = "S1"

func setup(t *testing.T) (apphistory.Service, *apptest.Stores) {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	if err := apptest.Sample(t.Context(), s, store); err != nil {
		t.Fatal(err)
	}
	return apphistory.Service{Stores: s, Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))}, s
}

func write(t *testing.T, s port.Stores, login string, fn func(tx port.Tx) error) {
	t.Helper()
	op := port.Operation{Actor: change.User(login), Summary: "テスト"}
	if err := s.Write(t.Context(), store, op, fn); err != nil {
		t.Fatal(err)
	}
}

func texts(rows []dhistory.Row) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Who+": "+r.Text)
	}
	return out
}

func TestItemHistory(t *testing.T) {
	svc, s := setup(t)
	write(t, s, "hanako", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "ログイン"))
	})
	// 値の削除と同じ保存のアイテムの行は出さない(B2-9)。
	write(t, s, "taro", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewDelete("systems/"+string(apptest.SampleSystemWeb))),
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone)),
		)
	})
	write(t, s, "taro", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", apptest.SampleStatusDone, apptest.SampleStatusInProgress))
	})
	rows, err := svc.Item(t.Context(), store, apptest.SampleTask)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"taro: ステータスを Done → InProgress", "hanako: タイトルを「ログイン」に変更", "taro: 作成"}
	if got := texts(rows); !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
	if _, err := svc.Item(t.Context(), store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("ないアイテム: %v", err)
	}
}

func TestProjectHistory(t *testing.T) {
	svc, s := setup(t)
	write(t, s, "hanako", func(tx port.Tx) error {
		return tx.WriteProjects().Append(apptest.SampleProject,
			change.NewSet("title", "ログイン", "認証"),
			change.NewSet("owner", "taro", "hanako"),
		)
	})
	rows, err := svc.Project(t.Context(), store, apptest.SampleProject)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"hanako: オーナーを hanako に変更", "hanako: タイトルを 認証 に変更", "taro: 作成"}
	if got := texts(rows); !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
	if _, err := svc.Project(t.Context(), store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("ないプロジェクト: %v", err)
	}
}
```

- [ ] **Step 6: 失敗を確かめる**

Run: `go test ./internal/app/history/`
Expected: FAIL(`no non-test Go files` または `undefined: apphistory.Service`)

- [ ] **Step 7: app を実装する**

`internal/app/history/service.go`(新規):

```go
// Package history は、アイテムとプロジェクトの履歴(02「履歴」)を読む操作を持つ。
package history

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	dhistory "github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// Service は、履歴を読む口。
type Service struct {
	Stores port.Stores
	Clock  port.Clock
}

// Item は、アイテムの履歴を新しい順で返す。なければ port.ErrNotFound。値の名前は、いまのワークスペースの設定で引く。
// ワークスペースのログに同じ操作の delete の行があるアイテムの行は出さない(B2-9)。
func (s Service) Item(ctx context.Context, store port.StoreID, id item.ID) ([]dhistory.Row, error) {
	var rows []dhistory.Row
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := r.Items().Get(id)
		if err != nil {
			return err
		}
		st, err := r.Workspaces().Settings(it.Place.Workspace)
		if err != nil {
			return err
		}
		wsLog, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: it.Place.Workspace})
		if err != nil {
			return err
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		if err != nil {
			return err
		}
		ms, err := r.Members().List()
		if err != nil {
			return err
		}
		rows = dhistory.ItemRows(log, dhistory.Context{
			Settings: st, Names: dhistory.NamesOf(ms), Hidden: dhistory.HiddenOps(wsLog), Today: s.Clock.Now(),
		})
		return nil
	})
	return rows, err
}

// Project は、プロジェクトの履歴を新しい順で返す。なければ port.ErrNotFound。
func (s Service) Project(ctx context.Context, store port.StoreID, id project.ID) ([]dhistory.Row, error) {
	var rows []dhistory.Row
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := r.Projects().Get(id); err != nil {
			return err
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindProject, ID: id})
		if err != nil {
			return err
		}
		ms, err := r.Members().List()
		if err != nil {
			return err
		}
		rows = dhistory.ProjectRows(log, dhistory.ProjectContext{Names: dhistory.NamesOf(ms), Today: s.Clock.Now()})
		return nil
	})
	return rows, err
}
```

Run: `go test ./internal/app/history/` → PASS
Run: `go test ./internal/app/... ./internal/domain/...` → PASS

- [ ] **Step 8: コミット(app)**

```bash
git add internal/app/history
git commit -m "feat(app): アイテムとプロジェクトの履歴を読む(A4a-15・A4a-16)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 9: API の失敗するテストを書く**

`internal/adapter/httpapi/notes/history_test.go`(新規。`notes.Services` の他のフィールドは使わない):

```go
package notes_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/notes"
	"github.com/kukv/hedgehug/internal/app/apptest"
	apphistory "github.com/kukv/hedgehug/internal/app/history"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

func newHistoryAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	op := port.Operation{Actor: change.User("hanako"), Summary: "タイトルを変更"}
	err := s.Write(t.Context(), "S1", op, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "ログイン"))
	})
	if err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	notes.Register(api, notes.Services{History: apphistory.Service{Stores: s, Clock: clock}})
	return api
}

func TestItemHistory(t *testing.T) {
	api := newHistoryAPI(t)
	res := api.Get("/api/stores/S1/items/" + string(apptest.SampleTask) + "/history")
	body := res.Body.String()
	if res.Code != http.StatusOK {
		t.Fatalf("%d %s", res.Code, body)
	}
	newer, older := strings.Index(body, `"text":"タイトルを「ログイン」に変更"`), strings.Index(body, `"text":"作成"`)
	if newer < 0 || older < 0 || newer > older {
		t.Errorf("新しい順に、タイトルの変更、作成: %s", body)
	}
	for _, want := range []string{`"who":"hanako"`, `"who":"taro"`, `"at":"2026-11-13T`} {
		if !strings.Contains(body, want) {
			t.Errorf("lacks %s: %s", want, body)
		}
	}
}

func TestProjectHistoryAndNotFound(t *testing.T) {
	api := newHistoryAPI(t)
	res := api.Get("/api/stores/S1/projects/" + string(apptest.SampleProject) + "/history")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"text":"作成"`) {
		t.Errorf("project: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/S1/items/01NOPE00000000000000000000/history"); res.Code != http.StatusNotFound {
		t.Errorf("no such item: %d", res.Code)
	}
	if res := api.Get("/api/stores/S1/projects/01NOPE00000000000000000000/history"); res.Code != http.StatusNotFound {
		t.Errorf("no such project: %d", res.Code)
	}
}
```

- [ ] **Step 10: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/notes/ -run 'TestItemHistory|TestProjectHistory'`
Expected: FAIL(`notes.Services` に `History` がない)

- [ ] **Step 11: API を実装する**

`internal/adapter/httpapi/notes/history.go`(新規):

```go
package notes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	apphistory "github.com/kukv/hedgehug/internal/app/history"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	dhistory "github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// HistoryRow は、履歴の 1 行。文はサーバーが作る(A4a-15)。
type HistoryRow struct {
	Who  string `json:"who" doc:"操作した人の表示(メンバーの名前・自動化・GitHub・同期・不明なユーザー)"`
	Text string `json:"text" doc:"変更の文(作成・タイトルを「…」に変更 など)"`
	At   string `json:"at" doc:"時刻(ミリ秒までの RFC 3339)"`
}

type itemHistoryInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
}

type projectHistoryInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
}

type historyOutput struct{ Body []HistoryRow }

func historyRows(rows []dhistory.Row) *historyOutput {
	out := &historyOutput{Body: []HistoryRow{}}
	for _, r := range rows {
		out.Body = append(out.Body, HistoryRow{Who: r.Who, Text: r.Text, At: r.At.Format(change.TimeLayout)})
	}
	return out
}

func registerHistory(api huma.API, svc apphistory.Service) {
	huma.Register(api, huma.Operation{OperationID: "get-item-history", Method: http.MethodGet, Path: "/api/stores/{store}/items/{item}/history", Summary: "アイテムの履歴を新しい順で返す"},
		func(ctx context.Context, in *itemHistoryInput) (*historyOutput, error) {
			rows, err := svc.Item(ctx, port.StoreID(in.Store), item.ID(in.Item))
			if err != nil {
				return nil, apierr.From(err)
			}
			return historyRows(rows), nil
		})
	huma.Register(api, huma.Operation{OperationID: "get-project-history", Method: http.MethodGet, Path: "/api/stores/{store}/projects/{project}/history", Summary: "プロジェクトの履歴を新しい順で返す"},
		func(ctx context.Context, in *projectHistoryInput) (*historyOutput, error) {
			rows, err := svc.Project(ctx, port.StoreID(in.Store), project.ID(in.Project))
			if err != nil {
				return nil, apierr.From(err)
			}
			return historyRows(rows), nil
		})
}
```

`internal/adapter/httpapi/notes/` の `Services` と `Register` に足す(Task 3 が作ったもの)。

```go
// Services に足す
	History apphistory.Service
// Register に足す
	registerHistory(api, s.History)
```

`internal/adapter/httpapi/routes.go`: `notes.Services{...}` に足す(import に `apphistory "github.com/kukv/hedgehug/internal/app/history"` を足す)。

```go
		History: apphistory.Service{Stores: deps.Stores, Clock: deps.Clock},
```

Run: `go test ./internal/adapter/httpapi/...`
Expected: `TestOpenAPIYAMLIsUpToDate` だけ FAIL、ほかは PASS

- [ ] **Step 12: 生成物を作り直す**

Run: `make generate`
Expected: `api/openapi.yaml` に `get-item-history`・`get-project-history`・`HistoryRow` が増える(`git diff --stat api/openapi.yaml`)。

Run: `cd web && pnpm gen`
Expected: `web/packages/api/src/generated.ts` に `getItemHistory`・`getProjectHistory`・`HistoryRow` が増える。

Run: `go test ./internal/... && go vet ./internal/...` → PASS

- [ ] **Step 13: コミット(API)**

```bash
git add internal/adapter/httpapi api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat(api): アイテムとプロジェクトの履歴を返す(A4a-15)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 17: 履歴の区画(アイテム・プロジェクト)

詳細パネルの「履歴」の区画を出す。部品 `HistorySection.svelte` は `target` で、アイテムとプロジェクトを切り替える(プロジェクトの概要への置き方は Task 19)。サーバーは Task 16。

**この Task が決めること:**

- 画面の状態は core の `createHistory`(`features/notes/history.ts`)に置く。新しい順の行をすべて受け取り、画面には 40 件(`historyLimit`)まで出す。それより古いものがあるときだけ、一覧の末尾に「すべて表示」を置く。押したら、同じ対象を読み直しても、全件の表示を保つ。対象が変わったら、40 件に戻す。
- 履歴は読むだけなので、書き込みを無効にする場面はない(骨組みの `HistorySection` の props `{ target, readonly }` から `readonly` を除き、`refresh` を足す)。`refresh` は、詳細パネルの読み直し(保存したあと・画面への反映)のたびに履歴を読み直すための印で、アイテムなら `v.item` を渡す(読み直しのたびに新しいオブジェクトになる)。
- 説明を保存したら履歴も読み直す(「説明を変更」の行が増えるため。裁定済み)。説明の保存はアイテムの再読み込みを起こさないので、wire に保存回数の数 `descriptionSaves` を置き、Task 7 の説明の区画が保存のたびに 1 増やす。`HistorySection` の `refresh` には、アイテムと保存回数を並べて渡す。Task 7 の側の足し方は Step 10 に書く。
- 時刻は、01「日付と時刻」の相対時間(`relativeTime`)で出し、ポインターを置くと日時を出す。「同期で受信」の添え書きは C4 が足す。
- 読めなかったときは何も出さない(画面への反映は失敗を出さない。A2 の決定 29)。アイテム(プロジェクト)がなくなっていたら、空の一覧にする。
- `wire.ts` の定数名は `historyList`(`history` は、ブラウザの `history` を隠してしまうので使わない)。

**Files:**
- Create: `web/packages/core/src/model/history.ts`
- Modify: `web/packages/core/src/port/notes.ts`(`history` を 1 メソッド)・`web/packages/core/src/notesIndex.ts`
- Create: `web/packages/core/src/features/notes/history.ts`・`history.test.ts`
- Create: `web/packages/api/src/historyPort.ts`・`historyPort.test.ts`
- Modify: `web/packages/api/src/notes.ts`(`...historyCalls()` を 1 行)
- Create: `web/app/src/screens/notes/HistorySection.svelte`
- Modify: `web/app/src/screens/item/DetailPanel.svelte`・`web/app/src/wire.ts`
- Modify(Task 7 の成果物に 1 行ずつ): `web/packages/core/src/features/notes/description.ts`・`description.test.ts`・`web/app/src/screens/notes/DescriptionSection.svelte`
- Create: `web/e2e/history.spec.ts`

**Interfaces:**
- Consumes:
  - `getItemHistory(store, item)`・`getProjectHistory(store, project)`、型 `HistoryRow{who, text, at}`(`generated.ts`。Task 16 で生成済み)
  - `NotesPort`・`NoteTarget = { kind: "item" | "project"; storeId: string; id: string }`(Task 7。`notesIndex.ts` から出ている)
  - `relativeTime(at: Date, now: Date): string`(core)、`call`(api の `errors.ts`)
- Produces:
  - `HistoryRow{who: string; text: string; at: string}`(core の model)
  - `NotesPort.history(target: NoteTarget): Promise<HistoryRow[]>`(新しい順)
  - `historyLimit = 40`、`createHistory(deps: HistoryDeps): History`、`HistoryState{rows: HistoryRow[]; more: boolean; loaded: boolean}`
  - `HistorySection.svelte` の props `{ target: NoteTarget; refresh?: unknown }`
  - `wire.ts` の `historyList`

- [ ] **Step 1: 型と口を足す**

`web/packages/core/src/model/history.ts`(新規):

```ts
// 履歴の 1 行(02「履歴」)。文はサーバーが作る。who は、操作した人の表示。at は、ミリ秒までの RFC 3339。
export interface HistoryRow {
  who: string;
  text: string;
  at: string;
}
```

`web/packages/core/src/port/notes.ts`(Task 7 が作る)の先頭に import を足し、`export interface NotesPort {` の直後にメソッドを足す。

```ts
import type { HistoryRow } from "../model/history";
```

```ts
  // history は、アイテムかプロジェクトの履歴を、新しい順にすべて返す。
  history(target: NoteTarget): Promise<HistoryRow[]>;
```

`web/packages/core/src/notesIndex.ts`(Task 5 が作る。`index.ts` が再公開する)に足す。`index.ts` には足さない。

```ts
export {
  createHistory,
  type History,
  type HistoryDeps,
  type HistoryState,
  historyLimit,
} from "./features/notes/history";
export type { HistoryRow } from "./model/history";
```

- [ ] **Step 2: core の失敗するテストを書く**

`web/packages/core/src/features/notes/history.test.ts`(新規):

```ts
import { describe, expect, it } from "vitest";
import { NotFound } from "../../model/errors";
import type { HistoryRow } from "../../model/history";
import { get } from "../../reactive/store";
import { flush } from "../items/fixtures.test-util";
import { createHistory, type HistoryDeps, historyLimit } from "./history";

const rowOf = (i: number): HistoryRow => ({
  who: "太郎",
  text: `変更 ${i}`,
  at: new Date(Date.UTC(2026, 10, 13, 9, 0, 0) - i * 60_000).toISOString(),
});
const rowsOf = (n: number) => Array.from({ length: n }, (_, i) => rowOf(i));
const item = { kind: "item", storeId: "S1", id: "I1" } as const;

function setup(initial: HistoryRow[]) {
  let rows = initial;
  let missing = false;
  const calls: string[] = [];
  const waiting: ((rows: HistoryRow[]) => void)[] = [];
  let hold = false;
  const notes: HistoryDeps["notes"] = {
    history(target) {
      calls.push(`${target.kind} ${target.storeId} ${target.id}`);
      if (missing) return Promise.reject(new NotFound("not found"));
      if (hold) return new Promise((resolve) => waiting.push(resolve));
      return Promise.resolve(rows);
    },
  };
  return {
    history: createHistory({ notes }),
    calls,
    waiting,
    set: (next: HistoryRow[]) => {
      rows = next;
    },
    vanish: () => {
      missing = true;
    },
    holdAnswers: () => {
      hold = true;
    },
  };
}

describe("createHistory", () => {
  it("読む前は loaded が false。40 件までを出し、それより古いものがあるときだけ more が true", async () => {
    const { history } = setup(rowsOf(45));
    expect(get(history.state)).toEqual({ rows: [], more: false, loaded: false });
    await history.load(item);
    const s = get(history.state);
    expect(s.loaded).toBe(true);
    expect(s.rows).toHaveLength(historyLimit);
    expect(s.rows[0]).toEqual(rowOf(0));
    expect(s.more).toBe(true);
  });

  it("ちょうど 40 件なら more は false", async () => {
    const { history } = setup(rowsOf(historyLimit));
    await history.load(item);
    expect(get(history.state)).toMatchObject({ more: false });
    expect(get(history.state).rows).toHaveLength(historyLimit);
  });

  it("履歴がなければ、空の一覧を読み終えた状態になる", async () => {
    const { history } = setup([]);
    await history.load(item);
    expect(get(history.state)).toEqual({ rows: [], more: false, loaded: true });
  });

  it("すべて表示を押すと全件を出し、同じ対象の読み直しでも保つ。別の対象では 40 件に戻す", async () => {
    const { history, set } = setup(rowsOf(45));
    await history.load(item);
    history.showAll();
    expect(get(history.state)).toMatchObject({ more: false });
    expect(get(history.state).rows).toHaveLength(45);
    set(rowsOf(46));
    await history.load(item);
    expect(get(history.state).rows).toHaveLength(46);
    await history.load({ kind: "project", storeId: "S1", id: "P1" });
    expect(get(history.state).rows).toHaveLength(historyLimit);
    expect(get(history.state).more).toBe(true);
  });

  it("読む前のすべて表示は、何もしない", () => {
    const { history } = setup(rowsOf(45));
    history.showAll();
    expect(get(history.state)).toEqual({ rows: [], more: false, loaded: false });
  });

  it("対象の種類をそのまま口に渡す", async () => {
    const { history, calls } = setup([]);
    await history.load(item);
    await history.load({ kind: "project", storeId: "S1", id: "P1" });
    expect(calls).toEqual(["item S1 I1", "project S1 P1"]);
  });

  it("古い読み取りの答えは捨てる", async () => {
    const { history, waiting, holdAnswers } = setup([]);
    holdAnswers();
    const slow = history.load(item);
    const fast = history.load(item);
    await flush();
    waiting[1]?.(rowsOf(2));
    await fast;
    waiting[0]?.(rowsOf(5));
    await slow;
    expect(get(history.state).rows).toHaveLength(2);
  });

  it("なくなっていたら、空にする", async () => {
    const { history, vanish } = setup(rowsOf(3));
    await history.load(item);
    vanish();
    await history.load(item);
    expect(get(history.state)).toEqual({ rows: [], more: false, loaded: true });
  });

  it("reset は、最初に戻す", async () => {
    const { history } = setup(rowsOf(3));
    await history.load(item);
    history.reset();
    expect(get(history.state)).toEqual({ rows: [], more: false, loaded: false });
  });
});
```

- [ ] **Step 3: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core exec vitest run src/features/notes/history.test.ts`
Expected: FAIL(`Failed to resolve import "./history"`)

- [ ] **Step 4: core を実装する**

`web/packages/core/src/features/notes/history.ts`(新規):

```ts
import { NotFound } from "../../model/errors";
import type { HistoryRow } from "../../model/history";
import type { NotesPort, NoteTarget } from "../../port/notes";
import { type Readable, readonly, writable } from "../../reactive/store";

// 履歴の区画の状態(02「履歴」): 新しい順の行を 40 件まで出し、古いものがあれば「すべて表示」を置く。

export const historyLimit = 40;

export interface HistoryState {
  // rows は、いま出す行。more は、「すべて表示」を置くか(40 件より古いものがあって、まだ全件を出していない)。
  rows: HistoryRow[];
  more: boolean;
  // loaded は、読み終えたか(読むまでは「履歴はありません」を出さない)。
  loaded: boolean;
}

export interface History {
  state: Readable<HistoryState>;
  // load は、対象の履歴を読む。同じ対象の読み直しでは、「すべて表示」を保つ。
  load(target: NoteTarget): Promise<void>;
  showAll(): void;
  reset(): void;
}

export interface HistoryDeps {
  notes: Pick<NotesPort, "history">;
}

const blank: HistoryState = { rows: [], more: false, loaded: false };

export function createHistory(deps: HistoryDeps): History {
  const state = writable<HistoryState>(blank);
  let all: HistoryRow[] = [];
  let expanded = false;
  let key = "";
  // loads は、古い答えを捨てるための通し番号。
  let loads = 0;

  const publish = () =>
    state.set({
      rows: expanded ? all : all.slice(0, historyLimit),
      more: !expanded && all.length > historyLimit,
      loaded: true,
    });

  return {
    state: readonly(state),
    async load(target) {
      const next = `${target.kind}/${target.storeId}/${target.id}`;
      if (next !== key) {
        key = next;
        expanded = false;
        all = [];
        state.set(blank);
      }
      loads += 1;
      const mine = loads;
      try {
        const rows = await deps.notes.history(target);
        if (mine !== loads) return;
        all = rows;
        publish();
      } catch (e) {
        // なくなっていたら空にする。ほかの失敗は出さない(画面への反映は失敗を出さない。A2 の決定 29)。
        if (e instanceof NotFound && mine === loads) {
          all = [];
          publish();
        }
      }
    },
    showAll() {
      expanded = true;
      state.update((s) => (s.loaded ? { rows: all, more: false, loaded: true } : s));
    },
    reset() {
      key = "";
      loads += 1;
      expanded = false;
      all = [];
      state.set(blank);
    },
  };
}
```

Run: `pnpm -C web --filter @hedgehug/core exec vitest run src/features/notes/history.test.ts` → PASS
Run: `pnpm -C web --filter @hedgehug/core check` → 型の誤りなし

- [ ] **Step 5: コミット(core)**

```bash
git add web/packages/core
git commit -m "feat(core): 履歴の区画の状態(40 件と「すべて表示」)を足す(A4a-15)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 6: api の失敗するテストを書く**

`web/packages/api/src/historyPort.test.ts`(新規):

```ts
import { NotFound } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { historyCalls } from "./historyPort";

afterEach(() => {
  vi.unstubAllGlobals();
});

// respond は、fetch を 1 つの応答に差し替え、呼び出しを返す。
function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async () =>
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetch);
  setToken("tok");
  return fetch;
}

const rows = [{ who: "太郎", text: "作成", at: "2026-11-13T09:00:00.000+09:00" }];

describe("history calls", () => {
  it("reads the history of an item", async () => {
    const fetch = respond(200, rows);
    expect(await historyCalls().history({ kind: "item", storeId: "S1", id: "I1" })).toEqual(rows);
    expect(fetch.mock.calls[0]?.[0]).toBe("/api/stores/S1/items/I1/history");
  });

  it("reads the history of a project", async () => {
    const fetch = respond(200, rows);
    await historyCalls().history({ kind: "project", storeId: "S1", id: "P1" });
    expect(fetch.mock.calls[0]?.[0]).toBe("/api/stores/S1/projects/P1/history");
  });

  it("reads a null list as empty, and a 404 as NotFound", async () => {
    respond(200, null);
    expect(await historyCalls().history({ kind: "item", storeId: "S1", id: "I1" })).toEqual([]);
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    const err = await historyCalls()
      .history({ kind: "item", storeId: "S1", id: "I1" })
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(NotFound);
  });
});
```

- [ ] **Step 7: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/api exec vitest run src/historyPort.test.ts`
Expected: FAIL(`Failed to resolve import "./historyPort"`)

- [ ] **Step 8: api を実装する**

`web/packages/api/src/historyPort.ts`(新規):

```ts
import type { NotesPort } from "@hedgehug/core";
import { call } from "./errors";
import { getItemHistory, getProjectHistory } from "./generated";

export type HistoryCalls = Pick<NotesPort, "history">;

// historyCalls は、NotesPort の履歴のメソッド(createNotesPort の戻り値に広げる)。
export function historyCalls(): HistoryCalls {
  return {
    history: (target) =>
      call(
        async () =>
          (target.kind === "item"
            ? await getItemHistory(target.storeId, target.id)
            : await getProjectHistory(target.storeId, target.id)) ?? [],
      ),
  };
}
```

`web/packages/api/src/notes.ts`(Task 7 が作る)の `createNotesPort` の戻り値のオブジェクトの先頭に 1 行足し、import を足す。

```ts
import { historyCalls } from "./historyPort";
```

```ts
    ...historyCalls(),
```

Run: `pnpm -C web --filter @hedgehug/api exec vitest run src/historyPort.test.ts` → PASS
Run: `pnpm -C web --filter @hedgehug/api check` → 型の誤りなし

- [ ] **Step 9: コミット(api)**

```bash
git add web/packages/api/src/historyPort.ts web/packages/api/src/historyPort.test.ts web/packages/api/src/notes.ts
git commit -m "feat(api): 履歴を画面の口につなぐ(A4a-15)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 10: app に区画を結び付ける**

Task 7 の説明の編集に、保存できたときの知らせを足す(`web/packages/core/src/features/notes/description.ts`)。`DescriptionDeps` に足す。

```ts
  // onSaved は、説明を保存できたあと(履歴の読み直しなどに使う)。保存しなかったとき(変更なし・断り)は呼ばない。
  onSaved?(): void;
```

`save()` の `if (result.conflict) deps.toasts.show(descriptionConflictMessage);` の直前に足す。

```ts
    deps.onSaved?.();
```

`description.test.ts` の `setup` に、`onSaved` を渡せるようにする(Task 7 の `setup` は引数なしで、`createDescriptionEditor({ port, toasts, target, members })` を呼んでいる)。

```ts
function setup(onSaved?: () => void) {
  const notes = fakeNotesPort({ [keyOf(target)]: first });
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const editor = createDescriptionEditor({
    port: notes,
    toasts,
    target: () => target,
    members: async () => [{ login: "taro", name: "太郎" }],
    onSaved,
  });
```

(残りの `const shown = ...` と `return` は変えない。)そのあと、`describe("description editor", ...)` の中に足す。

```ts
  it("calls onSaved after a save, and not when nothing was saved", async () => {
    let saved = 0;
    const { editor } = setup(() => void saved++);
    await editor.load();
    editor.start();
    await editor.save(); // 変更なし
    expect(saved).toBe(0);
    editor.start();
    editor.setInput("書き換えた");
    await editor.save();
    expect(saved).toBe(1);
  });
```

Run: `pnpm -C web/packages/core exec vitest run src/features/notes/description.test.ts`
Expected: 実装の前は FAIL(`saved` が 0 のまま)、`deps.onSaved?.()` を足したあとは PASS

`web/app/src/screens/notes/DescriptionSection.svelte` の `createDescriptionEditor({ ... })` に 1 行足し、import に `descriptionSaves` を足す。

```ts
    onSaved: () => descriptionSaves.update((n) => n + 1),
```

`web/app/src/wire.ts` に足す。`createHistory` を core からの import に足し、`notes` は Task 7 が `wire.ts` に作った `NotesPort` の定数。`detail` を作る前でよい。

```ts
export const historyList = createHistory({ notes });
// 説明を保存した回数。履歴の区画が、説明の保存のあとに読み直すための印(アイテムの再読み込みは起きない)。
export const descriptionSaves = writable(0);
```

(`writable` は core からの import に足す。)

`web/app/src/screens/notes/HistorySection.svelte`(新規):

```svelte
<script lang="ts">
  import { type NoteTarget, relativeTime } from "@hedgehug/core";
  import { Button } from "@hedgehug/ui";
  import { onDestroy } from "svelte";
  import { historyList } from "../../wire";

  // 履歴(02「履歴」)。アイテムの詳細パネルとプロジェクトの概要で同じ。新しい順に 40 件まで出し、
  // それより古いものがあるときは「すべて表示」を置く。1 行に、操作した人・変更の文・時刻を出す。
  // refresh は、画面の読み直し(保存・反映)のたびに新しくなる値(アイテムなら v.item)。履歴も読み直す。
  interface Props {
    target: NoteTarget;
    refresh?: unknown;
  }

  let { target, refresh }: Props = $props();
  const model = historyList.state;

  $effect(() => {
    void refresh;
    void historyList.load({ kind: target.kind, storeId: target.storeId, id: target.id });
  });
  onDestroy(() => historyList.reset());

  const when = (at: string) => relativeTime(new Date(at), new Date());
  const exact = (at: string) => new Date(at).toLocaleString("ja-JP");
</script>

<section class="peek-sec" aria-label="履歴">
  <h3>履歴</h3>
  {#if $model.loaded && $model.rows.length === 0}
    <p class="muted">履歴はありません</p>
  {/if}
  {#if $model.rows.length > 0}
    <ol class="rows">
      {#each $model.rows as r, i (i)}
        <li>
          <span class="who">{r.who}</span>
          <span class="text">{r.text}</span>
          <time datetime={r.at} title={exact(r.at)}>{when(r.at)}</time>
        </li>
      {/each}
    </ol>
  {/if}
  {#if $model.more}
    <div>
      <Button size="sm" onclick={() => historyList.showAll()}>すべて表示</Button>
    </div>
  {/if}
</section>

<style>
  .peek-sec {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  h3 {
    font-size: 13px;
    margin: 0;
  }
  .muted {
    color: var(--muted);
    font-size: 12.5px;
    margin: 0;
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  li {
    display: flex;
    gap: 8px;
    align-items: baseline;
    padding: 4px 0;
    border-bottom: 1px solid var(--line);
    font-size: 12.5px;
  }
  .who {
    flex: none;
    color: var(--muted);
  }
  .text {
    flex: 1;
    min-width: 0;
  }
  time {
    flex: none;
    color: var(--muted);
    font-size: 12px;
    white-space: nowrap;
  }
</style>
```

`web/app/src/screens/item/DetailPanel.svelte` を変える。import に足す。

```ts
  import HistorySection from "../notes/HistorySection.svelte";
```

`descriptionSaves` を `../../wire` からの import に足す。区画の並びのいちばん下(ほかの区画のあと、`{/if}` と `</SidePanel>` の前)に足す。

```svelte
    <HistorySection
      target={{ kind: "item", storeId: v.store.id, id: v.item.id }}
      refresh={[v.item, $descriptionSaves]}
    />
```

Run: `wc -l web/app/src/screens/notes/HistorySection.svelte web/app/src/screens/item/DetailPanel.svelte`
Expected: どちらも 200 行以下。

Run: `pnpm -C web check`
Expected: 型の誤りなし(`svelte-check --fail-on-warnings` も通る)。`pnpm -C web lint` で書式を確かめ、崩れていたら `pnpm -C web fix`。

- [ ] **Step 11: 通しのテストを書く**

`web/e2e/history.spec.ts`(新規):

```ts
import { expect, test } from "@playwright/test";
import { newTask } from "./helpers";

test("the history shows the creation and a title change, newest first", async ({ page }, info) => {
  const { name, panel } = await newTask(page, info, "履歴の確認");
  const history = panel.getByRole("region", { name: "履歴" });
  await expect(history.getByRole("listitem")).toHaveCount(1);
  await expect(history.getByRole("listitem").first()).toContainText("作成");

  const title = panel.getByRole("textbox", { name: "タイトル" });
  await title.fill(`${name} 改`);
  await title.press("Enter");
  await expect(history.getByRole("listitem")).toHaveCount(2);
  await expect(history.getByRole("listitem").first()).toContainText(`タイトルを「${name} 改」に変更`);
  await expect(history.getByRole("listitem").last()).toContainText("作成");
  await expect(history.getByRole("button", { name: "すべて表示" })).toBeHidden();

  // 説明を保存すると、履歴に「説明を変更」が増える(説明の編集の操作は Task 7 の e2e と同じ)。
  const description = panel.getByRole("region", { name: "説明" });
  await description.getByRole("button", { name: "説明を追加" }).click();
  await description.getByRole("textbox").fill("説明です");
  await description.getByRole("button", { name: "保存" }).click();
  await expect(history.getByRole("listitem").first()).toContainText("説明を変更");
});
```

Run: `make e2e-bin`
Run: `pnpm -C web exec playwright test history`
Expected: 3 つの幅で PASS

- [ ] **Step 12: 全体の確認とコミット**

Run: `pnpm -C web lint && pnpm -C web check && pnpm -C web test && pnpm -C web deps && pnpm -C web size`
Expected: すべて通る。

```bash
git add web/app web/e2e/history.spec.ts
git commit -m "feat(app): 詳細パネルの履歴の区画を足す(A4a-15)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 18: プロジェクトの概要の編集とリポジトリ(サーバー)

決定 A4a-18・19・25 を実装する。タイトル・ステータス・オーナー・開始日・終了日の変更と、リポジトリの追加・削除を、1 操作 1 保存で書く。説明(`Describe`)は Task 3 の持ち場なので、ここでは書かない(「骨組みへの指摘」1)。Task 2 の成果には依存しない。

**Files:**
- Create: `internal/domain/project/repo.go`・`repo_test.go`
- Create: `internal/domain/project/text.go`・`text_test.go`
- Modify: `internal/domain/project/check.go`(`CheckRename`・`CheckDates` を足し、`CheckCreate` が同じ検査を使う)・`check_test.go`
- Modify: `internal/domain/project/project.go`(`Repos`・`FromLog`)・`project_test.go`
- Modify(`Project` が `==` で比べられなくなる直し): `internal/app/apptest/stores_contract_cases.go`・`internal/app/apptest/stores_contract_lists.go`・`internal/app/projects/projects_test.go`
- Modify: `internal/infra/gitstore/yaml.go`
- Create: `internal/infra/gitstore/yaml_project_test.go`
- Modify: `internal/app/projects/service.go`(`Clock`)
- Create: `internal/app/projects/edit.go`・`rename.go`・`setstatus.go`・`setowner.go`・`setdate.go`・`addrepo.go`・`removerepo.go`・`edit_test.go`
- Modify: `internal/adapter/httpapi/projects/projects.go`(`Repos`、`checkDates`)・`projects_test.go`(`newAPI` に `Clock`)、Create: `internal/adapter/httpapi/projects/edit.go`・`edit_test.go`
- Modify: `internal/adapter/httpapi/routes.go`(`projects.Register` の `Service` に `Clock`)
- Generated: `api/openapi.yaml`・`web/packages/api/src/generated.ts`

**Interfaces:**
- Consumes: `port.Stores`・`port.Identity`・`port.Clock`、`change.NewSet`・`NewAdd`・`NewRemove`、`refusal.New`、`member.Member`(`Members().List()`)、`project.ValidStatus`
- Produces(domain):
  - `project.Project.Repos []string`(大文字小文字を区別しない辞書順。ログの `repos` の `add`・`remove` から決める)
  - `project.NormalizeRepo(s string) (string, error)`(断りは「リポジトリは owner/name の形式で入力してください」)
  - `project.IndexRepo(repos []string, repo string) int`(大文字小文字を区別せず、なければ -1)
  - `project.CheckRename(title string, self project.ID, siblings []project.Project) error`(`title` は前後の空白を除いたもの)
  - `project.CheckDates(start, end string) error`
  - `project.DateText(s string, today time.Time) string`(空は「なし」、今年は `M/D`、ほかの年は `YYYY/MM/DD`)
- Produces(app): `projects.Service` に `Clock port.Clock` を足し、次を持つ。どれも、変えたあとの `project.Project` を返す。前と同じ値なら何も保存しない。
  - `Rename(ctx, store port.StoreID, id project.ID, title string) (project.Project, error)`
  - `SetStatus(ctx, store, id, status project.Status) (project.Project, error)`
  - `SetOwner(ctx, store, id, login string) (project.Project, error)`(空は「なし」。メンバーでなければ `port.ErrNotFound`)
  - `SetDate(ctx, store, id, field projects.DateField, value string) (project.Project, error)`(`projects.StartDate`・`projects.EndDate`、`value` が空なら「なし」)
  - `AddRepo(ctx, store, id, repo string) (project.Project, error)`・`RemoveRepo(ctx, store, id, repo string) (project.Project, error)`
- Produces(ログに書く行。Task 16 の履歴が読む): プロジェクトのログに、`set` の行 `f` = `title`・`status`・`owner`・`start`・`end`(`from`・`to` は文字列)と、`add`・`remove` の行 `f` = `repos`(値は `owner/name` の文字列)。履歴には `repos` の行を出さない(02「リポジトリの追加と削除」)。
- Produces(アクティビティ。変える前のタイトルを {タイトル} に書く。「骨組みへの指摘」2):
  - 「{タイトル} のタイトルを {新しいタイトル} に変更」「{タイトル} のステータスを {ステータス} に変更」「{タイトル} のオーナーを {名前または なし} に変更」「{タイトル} の開始日を {日付または なし} に変更」(終了日も同じ)
  - 「{タイトル} にリポジトリ {owner/name} を追加」「{タイトル} からリポジトリ {owner/name} を削除」
- Produces(HTTP。`Project` の応答に `repos: string[]` を足す):
  - `rename-project` PUT `/api/stores/{store}/projects/{project}/title` `{title}`
  - `set-project-status` PUT `…/status` `{status}`
  - `set-project-owner` PUT `…/owner` `{owner}`
  - `set-project-date` PUT `…/date` `{field: "start"|"end", value}`
  - `add-project-repo` POST `…/repos` `{repo}`
  - `remove-project-repo` POST `…/repos/remove` `{repo}`
  - 応答はどれも `Project`。断りは 422(detail に仕様の文言)、ないプロジェクトとメンバーでないオーナーは 404。

#### 18-1 ドメイン

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/project/repo_test.go`(新規):

```go
package project_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/project"
)

// 07 の owner/name の形。前後の空白は除いてから見る。
func TestNormalizeRepo(t *testing.T) {
	const bad = "リポジトリは owner/name の形式で入力してください"
	cases := []struct{ name, in, want, refused string }{
		{"plain", "example-org/web", "example-org/web", ""},
		{"trims", " example-org/web \n", "example-org/web", ""},
		{"dots, underscores and hyphens in the name", "a-b/c.d_e-f", "a-b/c.d_e-f", ""},
		{"no slash", "web", "", bad},
		{"two slashes", "a/b/c", "", bad},
		{"a dot in the owner", "a.b/c", "", bad},
		{"a star in the name", "a/b*", "", bad},
		{"a space inside", "a/b c", "", bad},
		{"a newline inside", "a/b\nc", "", bad},
		{"empty", "  ", "", bad},
	}
	for _, c := range cases {
		got, err := project.NormalizeRepo(c.in)
		if message(err) != c.refused || got != c.want {
			t.Errorf("%s: NormalizeRepo(%q) = %q, %v; want %q, %q", c.name, c.in, got, err, c.want, c.refused)
		}
	}
}

func TestIndexRepoIgnoresCase(t *testing.T) {
	repos := []string{"acme/api", "Example-Org/Web"}
	if got := project.IndexRepo(repos, "example-org/web"); got != 1 {
		t.Errorf("IndexRepo = %d, want 1", got)
	}
	if got := project.IndexRepo(repos, "acme/other"); got != -1 {
		t.Errorf("IndexRepo = %d, want -1", got)
	}
}
```

`internal/domain/project/text_test.go`(新規):

```go
package project_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/project"
)

// 01「日付と時刻」: 今年なら M/D、ほかの年は YYYY/MM/DD。空は「なし」。
func TestDateText(t *testing.T) {
	today := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	cases := []struct{ in, want string }{
		{"2026-11-20", "11/20"},
		{"2026-01-05", "1/5"},
		{"2027-01-05", "2027/01/05"},
		{"", "なし"},
	}
	for _, c := range cases {
		if got := project.DateText(c.in, today); got != c.want {
			t.Errorf("DateText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
```

`internal/domain/project/check_test.go` の末尾に足す(`message` は同じファイルの先頭にある):

```go
// 02「概要の編集」: タイトルは、空だと「タイトルは空にできません」(作成の文言とは違う)。
// 同じワークスペースの別のプロジェクトと同じ名前は断り、自分と同じ名前は断らない。
func TestCheckRename(t *testing.T) {
	siblings := []project.Project{{ID: "A", Title: "ログイン"}, {ID: "B", Title: "決済"}}
	cases := []struct {
		name, title string
		self        project.ID
		want        string
	}{
		{"ok", "請求", "A", ""},
		{"empty", "", "A", "タイトルは空にできません"},
		{"duplicate of another", "決済", "A", "このワークスペースには同じ名前のプロジェクトがあります"},
		{"the same title as itself", "ログイン", "A", ""},
	}
	for _, c := range cases {
		if got := message(project.CheckRename(c.title, c.self, siblings)); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// 開始日と終了日が同じ日は認める。どちらかが空なら比べない。
func TestCheckDates(t *testing.T) {
	cases := []struct{ name, start, end, want string }{
		{"same day", "2026-11-01", "2026-11-01", ""},
		{"only start", "2026-11-01", "", ""},
		{"only end", "", "2026-11-01", ""},
		{"end before start", "2026-11-02", "2026-11-01", "終了日は開始日以降にしてください"},
	}
	for _, c := range cases {
		if got := message(project.CheckDates(c.start, c.end)); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
```

`internal/domain/project/project_test.go` を、先頭の import を次にし、`TestCreateAndFromLog` の比べ方を直し、末尾にテストと補助関数を足す。

```go
import (
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)
```

`TestCreateAndFromLog` の中の `if !ok || got != p {` を次に直す(`Repos` を持つと `==` で比べられない)。

```go
	if !ok || !reflect.DeepEqual(got, p) {
```

末尾に足す:

```go
// stamp は、drafts を log の続きに、1 秒ずつ進む時刻で書く。
func stamp(log []change.Record, drafts ...change.Draft) []change.Record {
	n := len(log)
	meta := change.Meta{Op: change.ID(fmt.Sprintf("op%d", n)), At: time.Unix(int64(n), 0), Actor: change.User("taro")}
	return append(slices.Clone(log), change.Stamp(log, drafts, meta, func() change.ID {
		n++
		return change.ID(fmt.Sprintf("c%d", n))
	})...)
}

// リポジトリは add・remove の行で持ち、大文字小文字を区別しない辞書順で返す。
func TestFromLogReadsRepos(t *testing.T) {
	p := project.Project{ID: "P", Place: project.Place{Workspace: "W", Number: 1}, Rank: "V", Title: "ログイン", Status: project.Todo}
	log := stamp(nil, project.Create(p))
	log = stamp(log, change.NewAdd("repos", "example-org/web"), change.NewAdd("repos", "Acme/api"), change.NewAdd("repos", "tmp/x"))
	log = stamp(log, change.NewRemove("repos", "tmp/x"))
	got, ok := project.FromLog(log)
	if want := []string{"Acme/api", "example-org/web"}; !ok || !slices.Equal(got.Repos, want) {
		t.Errorf("Repos = %v, %v; want %v", got.Repos, ok, want)
	}
}

func TestFromLogWithoutReposHasNone(t *testing.T) {
	p := project.Project{ID: "P", Place: project.Place{Workspace: "W", Number: 1}, Rank: "V", Title: "ログイン", Status: project.Todo}
	got, ok := project.FromLog(stamp(nil, project.Create(p)))
	if !ok || len(got.Repos) != 0 {
		t.Errorf("Repos = %v, %v", got.Repos, ok)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/project/`
Expected: FAIL(コンパイルエラー: `undefined: project.NormalizeRepo`・`project.IndexRepo`・`project.DateText`・`project.CheckRename`・`project.CheckDates`、`got.Repos undefined`)

- [ ] **Step 3: 実装する**

`internal/domain/project/repo.go`(新規):

```go
package project

import (
	"regexp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// repoPattern は、07 の owner/name の形(owner は英数字とハイフン、name は英数字・ピリオド・アンダースコア・ハイフン)。
var repoPattern = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`)

// NormalizeRepo は、前後の空白を除いて owner/name の形かを見る(02「リポジトリの追加と削除」)。
func NormalizeRepo(s string) (string, error) {
	r := strings.TrimSpace(s)
	if !repoPattern.MatchString(r) {
		return "", refusal.New("リポジトリは owner/name の形式で入力してください")
	}
	return r, nil
}

// IndexRepo は、repos の中で repo と大文字小文字を区別せずに同じものの位置を返す。なければ -1。
func IndexRepo(repos []string, repo string) int {
	return slices.IndexFunc(repos, func(r string) bool { return strings.EqualFold(r, repo) })
}
```

`internal/domain/project/text.go`(新規):

```go
package project

import (
	"strconv"
	"time"
)

// DateText は、アクティビティの {値} に書く日付(01「日付と時刻」)。空は「なし」、今年なら M/D、
// ほかの年は YYYY/MM/DD。item の同じ規則(item は project を import するので、ここに置く)。
func DateText(s string, today time.Time) string {
	if s == "" {
		return "なし"
	}
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return s
	}
	if d.Year() == today.Year() {
		return strconv.Itoa(int(d.Month())) + "/" + strconv.Itoa(d.Day())
	}
	return d.Format("2006/01/02")
}
```

`internal/domain/project/check.go` を次の全文にする(`CheckCreate` の振る舞いは変えない。日付と重複の検査を取り出して、変更でも使う)。

```go
package project

import (
	"strings"

	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// CheckCreate は、プロジェクトの作成の検査(02「プロジェクト」の「作成」の表の順)。
// siblings は同じワークスペースのプロジェクト(削除済みを除く)。名前は前後の空白を除いて完全一致で比べる。
// 日付は YYYY-MM-DD の形で来る(API の型で確かめる)ので、文字列の比較で前後が分かる。
func CheckCreate(title, start, end string, siblings []Project) error {
	t := strings.TrimSpace(title)
	if t == "" {
		return refusal.New("タイトルを入力してください")
	}
	if err := CheckDates(start, end); err != nil {
		return err
	}
	return checkSameName(t, "", siblings)
}

// CheckRename は、タイトルの変更の検査(02「概要の編集」)。title は前後の空白を除いたもの。
// 空は断り(作成とは文言が違う)、自分(self)を除く同じワークスペースの別のプロジェクトと同じ名前も断る。
func CheckRename(title string, self ID, siblings []Project) error {
	if title == "" {
		return refusal.New("タイトルは空にできません")
	}
	return checkSameName(title, self, siblings)
}

// CheckDates は、開始日と終了日の組を検査する。どちらかが空なら比べない。同じ日は認める。
func CheckDates(start, end string) error {
	if start != "" && end != "" && end < start {
		return refusal.New("終了日は開始日以降にしてください")
	}
	return nil
}

func checkSameName(title string, self ID, siblings []Project) error {
	for _, s := range siblings {
		if (self == "" || s.ID != self) && s.Title == title { // self が空なら(作成)、誰も除かない
			return refusal.New("このワークスペースには同じ名前のプロジェクトがあります")
		}
	}
	return nil
}
```

`internal/domain/project/project.go`: import に `"slices"` と `"strings"` を足し、`Project` に `Repos` を足し、`FromLog` を次にする。

```go
// Project は、プロジェクトの固定の項目(02「プロジェクト」)。説明は description.md に置き、ここには持たない(A4a-1)。
type Project struct {
	ID     ID
	Place  Place
	Rank   string
	Title  string
	Owner  string // ログイン名。空は「なし」
	Status Status
	Start  string // YYYY-MM-DD。空は「なし」
	End    string
	Repos  []string // owner/name。大文字小文字を区別しない辞書順(A4a-19)
}
```

```go
// FromLog は、プロジェクトのログからいまの値を作る。作成の行がなければ false。
func FromLog(log []change.Record) (Project, bool) {
	s := change.Resolve(log, change.Options{Multi: func(f string) bool { return f == "repos" }})
	if s.Created == nil {
		return Project{}, false
	}
	var p Project
	for f, dst := range map[string]any{
		"id": &p.ID, "place": &p.Place, "rank": &p.Rank, "title": &p.Title,
		"owner": &p.Owner, "status": &p.Status, "start": &p.Start, "end": &p.End,
	} {
		if v, ok := s.Values[f]; ok {
			_ = json.Unmarshal(v, dst)
		}
	}
	for _, raw := range s.Lists["repos"] {
		var repo string
		if json.Unmarshal(raw, &repo) == nil {
			p.Repos = append(p.Repos, repo)
		}
	}
	slices.SortFunc(p.Repos, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	return p, true
}
```

- [ ] **Step 4: ほかの `Project` の比べ方を直す**

`Repos` を持つ `Project` は `==` で比べられない。コンパイルで落ちる場所を見つける。

Run: `go vet ./...`
Expected: FAIL。次の 4 つのファイルの 5 箇所が `invalid operation: … (struct containing []string cannot be compared)` で出る(この計画を書いた時点)。

直す(どれも `reflect.DeepEqual` にする。`Repos` が両方 nil なら等しい)。import に `"reflect"` を足す。

- `internal/app/apptest/stores_contract_cases.go` の `if err != nil || p != cProject {` → `if err != nil || !reflect.DeepEqual(p, cProject) {`
- `internal/app/apptest/stores_contract_lists.go` の `if err != nil || len(ps) != 2 || ps[0] != early || ps[1] != cProject {` → `if err != nil || len(ps) != 2 || !reflect.DeepEqual(ps[0], early) || !reflect.DeepEqual(ps[1], cProject) {`
- `internal/app/projects/projects_test.go` の `list[1] != p` → `!reflect.DeepEqual(list[1], p)`、`got != p` → `!reflect.DeepEqual(got, p)`

Run: `go vet ./... && go test ./internal/domain/project/ ./internal/app/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/project internal/app/apptest/stores_contract_cases.go internal/app/apptest/stores_contract_lists.go internal/app/projects/projects_test.go
git commit -m "feat(domain): プロジェクトのリポジトリ・タイトルの変更の検査・日付の表記(A4a-18・19)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

#### 18-2 保存形式(project.yaml)

- [ ] **Step 1: 失敗するテストを書く**

`internal/infra/gitstore/yaml_project_test.go`(新規。`yWS`・`yProj`・`yIDs` は同じパッケージの `yaml_settings_test.go` にある):

```go
package gitstore

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// project.yaml には、結びつけたリポジトリを書き出す(読む用の写し。正本は log.jsonl)。
func TestProjectYAMLListsRepos(t *testing.T) {
	p := project.Project{ID: yProj, Place: project.Place{Workspace: yWS, Number: 1}, Rank: "V", Title: "ログイン", Status: project.Todo}
	meta := change.Meta{Op: "o", At: time.Unix(0, 0), Actor: change.User("taro")}
	log := change.Stamp(nil, []change.Draft{project.Create(p), change.NewAdd("repos", "example-org/web")}, meta, yIDs())
	out, err := renderYAML(change.KindProject, log, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "repos: [example-org/web]") {
		t.Errorf("project.yaml =\n%s", out)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/infra/gitstore/ -run TestProjectYAMLListsRepos`
Expected: FAIL(`project.yaml =` の出力に `repos` がない)

- [ ] **Step 3: 実装する**

`internal/infra/gitstore/yaml.go` の `projectYAML` の末尾に足す(`itemYAML` の `Watchers` と同じ書き方)。

```go
	Repos     []string `yaml:"repos,flow"`
```

`renderYAML` の `projectYAML{…}` の末尾に `Repos: p.Repos,` を足す。

```go
		v = projectYAML{
			ID: string(p.ID), Workspace: string(p.Place.Workspace), Number: p.Place.Number, Rank: p.Rank,
			Title: p.Title, Owner: p.Owner, Status: string(p.Status), Start: p.Start, End: p.End, Repos: p.Repos,
		}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/infra/gitstore/`
Expected: PASS(`repos: []` の書き出しが増えるだけなので、既存の `project.yaml` の確認は変わらない。表記が違って落ちたときは、実際の出力に合わせて Step 1 の文字列を直す)

- [ ] **Step 5: コミット**

```bash
git add internal/infra/gitstore/yaml.go internal/infra/gitstore/yaml_project_test.go
git commit -m "feat(gitstore): project.yaml にリポジトリを書き出す(A4a-19)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

#### 18-3 操作(app)

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/projects/edit_test.go`(新規。`store`・`message` は `projects_test.go` にある):

```go
package projects_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/projects"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// recorder は、Write に渡したアクティビティの 1 行を覚える。
type recorder struct {
	*apptest.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func (r *recorder) last() string { return r.summaries[len(r.summaries)-1] }

func setupEdit(t *testing.T) (projects.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	now := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	s := apptest.NewStores(apptest.NewClock(now), ids)
	s.AddStore(store)
	if err := apptest.Sample(context.Background(), s, store); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return projects.Service{Stores: rec, IDs: ids, Identity: apptest.Identity{Login: "hanako"}, Clock: apptest.NewClock(now)}, rec
}

// logOf は、サンプルのプロジェクトのログ。何も保存しなかったことを確かめるのに使う。
func logOf(t *testing.T, s port.Stores) []change.Record {
	t.Helper()
	var log []change.Record
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		var err error
		log, err = r.Changes().Of(change.Ref{Kind: change.KindProject, ID: apptest.SampleProject})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return log
}

func TestRenameTrimsAndSavesOneRow(t *testing.T) {
	svc, rec := setupEdit(t)
	p, err := svc.Rename(t.Context(), store, apptest.SampleProject, "  決済  ")
	if err != nil || p.Title != "決済" {
		t.Fatalf("Rename = %+v, %v", p, err)
	}
	if got, want := rec.last(), "ログイン のタイトルを 決済 に変更"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	log := logOf(t, svc.Stores)
	row := log[len(log)-1]
	if row.T != change.Set || row.F != "title" || row.By != "hanako" || string(row.From) != `"ログイン"` || string(row.To) != `"決済"` {
		t.Errorf("row = %+v", row)
	}
}

func TestRenameRefuses(t *testing.T) {
	svc, _ := setupEdit(t)
	if _, err := svc.Create(t.Context(), projects.CreateInput{Store: store, Workspace: apptest.SampleWorkspace, Title: "決済"}); err != nil {
		t.Fatal(err)
	}
	before := len(logOf(t, svc.Stores))
	for _, c := range []struct{ title, want string }{
		{"  ", "タイトルは空にできません"},
		{"決済", "このワークスペースには同じ名前のプロジェクトがあります"},
	} {
		if _, err := svc.Rename(t.Context(), store, apptest.SampleProject, c.title); message(err) != c.want {
			t.Errorf("Rename(%q) = %v, want %q", c.title, err, c.want)
		}
	}
	if n := len(logOf(t, svc.Stores)); n != before {
		t.Errorf("a refused rename saved %d rows", n-before)
	}
	if _, err := svc.Rename(t.Context(), store, "01NOPE00000000000000000000", "請求"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Rename of a missing project = %v", err)
	}
}

func TestRenameToTheSameTitleSavesNothing(t *testing.T) {
	svc, _ := setupEdit(t)
	before := len(logOf(t, svc.Stores))
	if _, err := svc.Rename(t.Context(), store, apptest.SampleProject, " ログイン "); err != nil {
		t.Fatal(err)
	}
	if n := len(logOf(t, svc.Stores)); n != before {
		t.Errorf("saved %d rows for the same title", n-before)
	}
}

func TestSetStatus(t *testing.T) {
	svc, rec := setupEdit(t)
	p, err := svc.SetStatus(t.Context(), store, apptest.SampleProject, project.InProgress)
	if err != nil || p.Status != project.InProgress {
		t.Fatalf("SetStatus = %+v, %v", p, err)
	}
	if got, want := rec.last(), "ログイン のステータスを InProgress に変更"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	before := len(logOf(t, svc.Stores))
	if _, err := svc.SetStatus(t.Context(), store, apptest.SampleProject, project.InProgress); err != nil || len(logOf(t, svc.Stores)) != before {
		t.Errorf("the same status saved something: %v", err)
	}
	if _, err := svc.SetStatus(t.Context(), store, apptest.SampleProject, "Review"); err == nil {
		t.Error("an unknown status was accepted")
	}
}

func TestSetOwner(t *testing.T) {
	svc, rec := setupEdit(t)
	p, err := svc.SetOwner(t.Context(), store, apptest.SampleProject, "HANAKO")
	if err != nil || p.Owner != "hanako" {
		t.Fatalf("SetOwner = %+v, %v", p, err) // メンバーのログイン名で保存する
	}
	if got, want := rec.last(), "ログイン のオーナーを hanako に変更"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	p, err = svc.SetOwner(t.Context(), store, apptest.SampleProject, "")
	if err != nil || p.Owner != "" {
		t.Fatalf("SetOwner none = %+v, %v", p, err)
	}
	if got, want := rec.last(), "ログイン のオーナーを なし に変更"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	if _, err := svc.SetOwner(t.Context(), store, apptest.SampleProject, "nobody"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("a non-member owner = %v, want ErrNotFound", err)
	}
}

func TestSetDate(t *testing.T) {
	svc, rec := setupEdit(t)
	set := func(field projects.DateField, value string) project.Project {
		t.Helper()
		p, err := svc.SetDate(t.Context(), store, apptest.SampleProject, field, value)
		if err != nil {
			t.Fatalf("SetDate(%s, %q) = %v", field, value, err)
		}
		return p
	}
	if p := set(projects.StartDate, "2026-11-20"); p.Start != "2026-11-20" {
		t.Errorf("Start = %q", p.Start)
	}
	if got, want := rec.last(), "ログイン の開始日を 11/20 に変更"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	if p := set(projects.EndDate, "2027-01-05"); p.End != "2027-01-05" {
		t.Errorf("End = %q", p.End)
	}
	if got, want := rec.last(), "ログイン の終了日を 2027/01/05 に変更"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	set(projects.EndDate, "2026-11-20") // 開始日と同じ日は認める
	for _, c := range []struct {
		field projects.DateField
		value string
	}{{projects.StartDate, "2026-11-21"}, {projects.EndDate, "2026-11-19"}} {
		if _, err := svc.SetDate(t.Context(), store, apptest.SampleProject, c.field, c.value); message(err) != "終了日は開始日以降にしてください" {
			t.Errorf("SetDate(%s, %q) = %v", c.field, c.value, err)
		}
	}
	if p := set(projects.StartDate, ""); p.Start != "" { // 日付を空にすることは常にできる
		t.Errorf("Start = %q", p.Start)
	}
	if got, want := rec.last(), "ログイン の開始日を なし に変更"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	before := len(logOf(t, svc.Stores))
	set(projects.EndDate, "2026-11-20")
	if n := len(logOf(t, svc.Stores)); n != before {
		t.Errorf("the same date saved %d rows", n-before)
	}
}

func TestAddAndRemoveRepo(t *testing.T) {
	svc, rec := setupEdit(t)
	ctx := t.Context()
	p, err := svc.AddRepo(ctx, store, apptest.SampleProject, " example-org/web ")
	if err != nil || len(p.Repos) != 1 || p.Repos[0] != "example-org/web" {
		t.Fatalf("AddRepo = %+v, %v", p, err)
	}
	if got, want := rec.last(), "ログイン にリポジトリ example-org/web を追加"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	for _, c := range []struct{ in, want string }{
		{"Example-Org/WEB", "もう追加されています"},
		{"web", "リポジトリは owner/name の形式で入力してください"},
		{"a/b/c", "リポジトリは owner/name の形式で入力してください"},
	} {
		if _, err := svc.AddRepo(ctx, store, apptest.SampleProject, c.in); message(err) != c.want {
			t.Errorf("AddRepo(%q) = %v, want %q", c.in, err, c.want)
		}
	}
	if p, err = svc.AddRepo(ctx, store, apptest.SampleProject, "acme/api"); err != nil || len(p.Repos) != 2 || p.Repos[0] != "acme/api" {
		t.Fatalf("AddRepo second = %+v, %v", p, err)
	}
	// 削除は、保存してある綴りで行を書く。アクティビティには入力のとおりに書く。
	p, err = svc.RemoveRepo(ctx, store, apptest.SampleProject, "EXAMPLE-ORG/web")
	if err != nil || len(p.Repos) != 1 || p.Repos[0] != "acme/api" {
		t.Fatalf("RemoveRepo = %+v, %v", p, err)
	}
	if got, want := rec.last(), "ログイン からリポジトリ EXAMPLE-ORG/web を削除"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	log := logOf(t, svc.Stores)
	if row := log[len(log)-1]; row.T != change.Remove || row.F != "repos" || string(row.From) != `"example-org/web"` {
		t.Errorf("remove row = %+v", row)
	}
	before := len(log)
	if _, err := svc.RemoveRepo(ctx, store, apptest.SampleProject, "none/none"); err != nil || len(logOf(t, svc.Stores)) != before {
		t.Errorf("removing a repo that is not there saved something: %v", err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/projects/`
Expected: FAIL(コンパイルエラー: `unknown field Clock`、`svc.Rename undefined`、`undefined: projects.DateField` ほか)

- [ ] **Step 3: 実装する**

`internal/app/projects/service.go` に `Clock` を足す。

```go
// Service は、プロジェクトの操作が使う口。Clock はアクティビティの日付の年を決める。
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
	Clock    port.Clock
}
```

`internal/app/projects/edit.go`(新規):

```go
package projects

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// save は、保存の中でいまのプロジェクトを読み直し、drafts が返す行を 1 回で保存する(02「概要の編集」)。
// 行がなければ何も残さない。drafts が断ったら何も保存しない。summary はアクティビティの 1 行で、保存の前に決める。
func (s Service) save(ctx context.Context, store port.StoreID, id project.ID, summary string, drafts func(tx port.Tx, p project.Project) ([]change.Draft, error)) (project.Project, error) {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return project.Project{}, err
	}
	op := port.Operation{Actor: change.User(me), Summary: summary}
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		p, err := tx.Projects().Get(id)
		if err != nil {
			return err
		}
		ds, err := drafts(tx, p)
		if err != nil {
			return err
		}
		return tx.WriteProjects().Append(id, ds...)
	})
	if err != nil {
		return project.Project{}, err
	}
	return s.Get(ctx, store, id)
}

// label は、アクティビティの 1 行に書くタイトル。タイトルは改行を含められる(作成と同じ)ので、
// 改行を含む連続する空白を 1 つの空白にまとめる(1 行でないと保存できない)。
func label(title string) string { return strings.Join(strings.Fields(title), " ") }
```

`internal/app/projects/rename.go`(新規):

```go
package projects

import (
	"context"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// Rename は、タイトルを変える(02「概要の編集」)。前後の空白を除いた形で保存し、前と同じなら何も保存しない。
// アクティビティは「{変える前のタイトル} のタイトルを {新しいタイトル} に変更」。
func (s Service) Rename(ctx context.Context, store port.StoreID, id project.ID, title string) (project.Project, error) {
	title = strings.TrimSpace(title)
	p, err := s.Get(ctx, store, id)
	if err != nil {
		return project.Project{}, err
	}
	summary := fmt.Sprintf("%s のタイトルを %s に変更", label(p.Title), label(title))
	return s.save(ctx, store, id, summary, func(tx port.Tx, cur project.Project) ([]change.Draft, error) {
		siblings, err := tx.Projects().List(cur.Place.Workspace)
		if err != nil {
			return nil, err
		}
		if err := project.CheckRename(title, cur.ID, siblings); err != nil {
			return nil, err
		}
		if cur.Title == title {
			return nil, nil
		}
		return []change.Draft{change.NewSet("title", cur.Title, title)}, nil
	})
}
```

`internal/app/projects/setstatus.go`(新規):

```go
package projects

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// SetStatus は、ステータスを変える(02「概要の編集」)。下のアイテムには影響しない。前と同じなら何も保存しない。
func (s Service) SetStatus(ctx context.Context, store port.StoreID, id project.ID, status project.Status) (project.Project, error) {
	if !project.ValidStatus(status) {
		return project.Project{}, fmt.Errorf("unknown project status %q", status)
	}
	p, err := s.Get(ctx, store, id)
	if err != nil {
		return project.Project{}, err
	}
	summary := fmt.Sprintf("%s のステータスを %s に変更", label(p.Title), status)
	return s.save(ctx, store, id, summary, func(_ port.Tx, cur project.Project) ([]change.Draft, error) {
		if cur.Status == status {
			return nil, nil
		}
		return []change.Draft{change.NewSet("status", cur.Status, status)}, nil
	})
}
```

`internal/app/projects/setowner.go`(新規):

```go
package projects

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// SetOwner は、オーナーを変える(02「概要の編集」)。login が空なら「なし」。メンバーでなければ port.ErrNotFound。
// ログイン名は大文字と小文字を区別せずに比べ、保存するのはメンバーのログイン名。前と同じなら何も保存しない。
func (s Service) SetOwner(ctx context.Context, store port.StoreID, id project.ID, login string) (project.Project, error) {
	p, err := s.Get(ctx, store, id)
	if err != nil {
		return project.Project{}, err
	}
	owner, name := "", "なし"
	if login != "" {
		m, err := s.memberOf(ctx, store, login)
		if err != nil {
			return project.Project{}, err
		}
		owner, name = m.Login, m.Name
	}
	summary := fmt.Sprintf("%s のオーナーを %s に変更", label(p.Title), label(name))
	return s.save(ctx, store, id, summary, func(_ port.Tx, cur project.Project) ([]change.Draft, error) {
		if cur.Owner == owner {
			return nil, nil
		}
		return []change.Draft{change.NewSet("owner", cur.Owner, owner)}, nil
	})
}

func (s Service) memberOf(ctx context.Context, store port.StoreID, login string) (member.Member, error) {
	var found member.Member
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		ms, err := r.Members().List()
		if err != nil {
			return err
		}
		i := slices.IndexFunc(ms, func(m member.Member) bool { return strings.EqualFold(m.Login, login) })
		if i < 0 {
			return port.ErrNotFound
		}
		found = ms[i]
		return nil
	})
	return found, err
}
```

`internal/app/projects/setdate.go`(新規):

```go
package projects

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// DateField は、日付の項目。値はログの項目名と同じ。
type DateField string

const (
	StartDate DateField = "start"
	EndDate   DateField = "end"
)

// SetDate は、開始日か終了日を 1 つ変える(02「概要の編集」)。value は YYYY-MM-DD か空(なし)。
// 保存の中で読み直したもう片方と比べ、終了日が開始日より前なら断る。前と同じなら何も保存しない。
func (s Service) SetDate(ctx context.Context, store port.StoreID, id project.ID, field DateField, value string) (project.Project, error) {
	name, ok := map[DateField]string{StartDate: "開始日", EndDate: "終了日"}[field]
	if !ok {
		return project.Project{}, fmt.Errorf("unknown date field %q", field)
	}
	p, err := s.Get(ctx, store, id)
	if err != nil {
		return project.Project{}, err
	}
	summary := fmt.Sprintf("%s の%sを %s に変更", label(p.Title), name, project.DateText(value, s.Clock.Now()))
	return s.save(ctx, store, id, summary, func(_ port.Tx, cur project.Project) ([]change.Draft, error) {
		from, start, end := cur.Start, value, cur.End
		if field == EndDate {
			from, start, end = cur.End, cur.Start, value
		}
		if from == value {
			return nil, nil
		}
		if err := project.CheckDates(start, end); err != nil {
			return nil, err
		}
		return []change.Draft{change.NewSet(string(field), from, value)}, nil
	})
}
```

`internal/app/projects/addrepo.go`(新規):

```go
package projects

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// AddRepo は、リポジトリを 1 つ結びつける(02「リポジトリの追加と削除」)。owner/name の形でなければ断り、
// すでにあるもの(大文字と小文字を区別しない)は、保存の中で読み直した一覧で見て断る。
// 履歴には残らず、アクティビティは「{タイトル} にリポジトリ {owner/name} を追加」。
func (s Service) AddRepo(ctx context.Context, store port.StoreID, id project.ID, input string) (project.Project, error) {
	repo, err := project.NormalizeRepo(input)
	if err != nil {
		return project.Project{}, err
	}
	p, err := s.Get(ctx, store, id)
	if err != nil {
		return project.Project{}, err
	}
	summary := fmt.Sprintf("%s にリポジトリ %s を追加", label(p.Title), repo)
	return s.save(ctx, store, id, summary, func(_ port.Tx, cur project.Project) ([]change.Draft, error) {
		if project.IndexRepo(cur.Repos, repo) >= 0 {
			return nil, refusal.New("もう追加されています")
		}
		return []change.Draft{change.NewAdd("repos", repo)}, nil
	})
}
```

`internal/app/projects/removerepo.go`(新規):

```go
package projects

import (
	"context"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// RemoveRepo は、リポジトリを 1 つ外す(02「リポジトリの追加と削除」。確認はしない)。ないものは何も保存しない。
// 行は、保存してある綴りで書く(大文字と小文字を区別しない)。アクティビティは「{タイトル} からリポジトリ {owner/name} を削除」。
func (s Service) RemoveRepo(ctx context.Context, store port.StoreID, id project.ID, input string) (project.Project, error) {
	repo := strings.TrimSpace(input)
	p, err := s.Get(ctx, store, id)
	if err != nil {
		return project.Project{}, err
	}
	summary := fmt.Sprintf("%s からリポジトリ %s を削除", label(p.Title), repo)
	return s.save(ctx, store, id, summary, func(_ port.Tx, cur project.Project) ([]change.Draft, error) {
		i := project.IndexRepo(cur.Repos, repo)
		if i < 0 {
			return nil, nil
		}
		return []change.Draft{change.NewRemove("repos", cur.Repos[i])}, nil
	})
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/projects/ ./internal/archtest/`
Expected: PASS(`internal/app/projects` は 13 ファイルで、パッケージ 20 ファイルの上限の中)

- [ ] **Step 5: コミット**

```bash
git add internal/app/projects
git commit -m "feat(app): プロジェクトのタイトル・ステータス・オーナー・日付・リポジトリを変える(A4a-18)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

#### 18-4 API と生成物

- [ ] **Step 1: 失敗するテストを書く**

`internal/adapter/httpapi/projects/projects_test.go` の `newAPI` の `Service` に時計を足す(日付の操作がアクティビティの年に使う)。

```go
	projects.Register(api, appprojects.Service{
		Stores: s, IDs: ids, Identity: apptest.Identity{Login: "taro"},
		Clock: apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)),
	})
```

`internal/adapter/httpapi/projects/edit_test.go`(新規。`newAPI`・`base` は同じパッケージの `projects_test.go` にある):

```go
package projects_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
)

const one = "/api/stores/S1/projects/" + string(apptest.SampleProject)

func check(t *testing.T, name string, res *httptest.ResponseRecorder, code int, contains string) {
	t.Helper()
	if res.Code != code || !strings.Contains(res.Body.String(), contains) {
		t.Errorf("%s: %d %s, want %d containing %q", name, res.Code, res.Body, code, contains)
	}
}

func TestEditEndpoints(t *testing.T) {
	api := newAPI(t)
	check(t, "get has no repos", api.Get(one), http.StatusOK, `"repos":[]`)

	check(t, "rename", api.Put(one+"/title", map[string]any{"title": " 決済 "}), http.StatusOK, `"title":"決済"`)
	check(t, "rename to empty", api.Put(one+"/title", map[string]any{"title": " "}), http.StatusUnprocessableEntity, "タイトルは空にできません")
	check(t, "rename with a newline", api.Put(one+"/title", map[string]any{"title": "a\nb"}), http.StatusUnprocessableEntity, "")
	api.Post(base, map[string]any{"title": "請求"})
	check(t, "rename to a duplicate", api.Put(one+"/title", map[string]any{"title": "請求"}), http.StatusUnprocessableEntity, "このワークスペースには同じ名前のプロジェクトがあります")

	check(t, "status", api.Put(one+"/status", map[string]any{"status": "InProgress"}), http.StatusOK, `"status":"InProgress"`)
	check(t, "unknown status", api.Put(one+"/status", map[string]any{"status": "Review"}), http.StatusUnprocessableEntity, "")

	check(t, "owner", api.Put(one+"/owner", map[string]any{"owner": "hanako"}), http.StatusOK, `"owner":"hanako"`)
	check(t, "no owner", api.Put(one+"/owner", map[string]any{"owner": ""}), http.StatusOK, `"owner":""`)
	check(t, "a non-member owner", api.Put(one+"/owner", map[string]any{"owner": "nobody"}), http.StatusNotFound, "")

	check(t, "start", api.Put(one+"/date", map[string]any{"field": "start", "value": "2026-11-20"}), http.StatusOK, `"start":"2026-11-20"`)
	check(t, "end before start", api.Put(one+"/date", map[string]any{"field": "end", "value": "2026-11-19"}), http.StatusUnprocessableEntity, "終了日は開始日以降にしてください")
	check(t, "not a date", api.Put(one+"/date", map[string]any{"field": "end", "value": "2026-02-30"}), http.StatusUnprocessableEntity, "not a date")
	check(t, "unknown field", api.Put(one+"/date", map[string]any{"field": "middle", "value": "2026-11-20"}), http.StatusUnprocessableEntity, "")
	check(t, "clear start", api.Put(one+"/date", map[string]any{"field": "start", "value": ""}), http.StatusOK, `"start":""`)

	check(t, "add repo", api.Post(one+"/repos", map[string]any{"repo": " example-org/web "}), http.StatusOK, `"repos":["example-org/web"]`)
	check(t, "add the same repo", api.Post(one+"/repos", map[string]any{"repo": "Example-Org/WEB"}), http.StatusUnprocessableEntity, "もう追加されています")
	check(t, "add a bad repo", api.Post(one+"/repos", map[string]any{"repo": "web"}), http.StatusUnprocessableEntity, "リポジトリは owner/name の形式で入力してください")
	check(t, "remove repo", api.Post(one+"/repos/remove", map[string]any{"repo": "example-org/web"}), http.StatusOK, `"repos":[]`)
}

func TestEditMissingProjectIs404(t *testing.T) {
	missing := "/api/stores/S1/projects/01NOPE00000000000000000000"
	check(t, "rename", newAPI(t).Put(missing+"/title", map[string]any{"title": "請求"}), http.StatusNotFound, "")
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/projects/`
Expected: FAIL(`unknown field Clock` は Step 1 で直した。残りは、登録していないパスへの PUT が 404 か 405 になって `rename`・`status` ほかの確認が落ちる。`get has no repos` も落ちる)

- [ ] **Step 3: 実装する**

`internal/adapter/httpapi/projects/projects.go` を直す。

1. `Project` に `repos` を足し、`toAPI` が `[]` を返すようにする。

```go
	End       string   `json:"end"`
	Repos     []string `json:"repos" doc:"結びつけた GitHub のリポジトリ(owner/name)。大文字小文字を区別しない辞書順"`
}

func toAPI(p project.Project) Project {
	return Project{
		ID: string(p.ID), Workspace: string(p.Place.Workspace), Number: p.Place.Number, Rank: p.Rank,
		Title: p.Title, Owner: p.Owner, Status: string(p.Status), Start: p.Start, End: p.End,
		Repos: append([]string{}, p.Repos...),
	}
}
```

2. 作成のハンドラーの日付の確かめを、変更でも使える関数に取り出す。

```go
// checkDates は、形(pattern)だけでは通る 2026-02-30 のような無い日付を、暦の上の日付かで断る。
func checkDates(ds ...string) error {
	for _, d := range ds {
		if _, err := time.Parse(time.DateOnly, d); d != "" && err != nil {
			return huma.Error422UnprocessableEntity("not a date: " + d)
		}
	}
	return nil
}
```

作成のハンドラーの `for _, d := range []string{in.Body.Start, in.Body.End} { … }` の 4 行を、次の 3 行に置き換える。

```go
			if err := checkDates(in.Body.Start, in.Body.End); err != nil {
				return nil, err
			}
```

3. `Register` の最後に `registerEdit(api, svc)` を足す。

`internal/adapter/httpapi/projects/edit.go`(新規):

```go
package projects

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	appprojects "github.com/kukv/hedgehug/internal/app/projects"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// 本文は名前のある型にする(名前のない型だと、OpenAPI のスキーマ名が登録の順で決まる)。

// RenameProject は、タイトルの変更の本文。前後の空白は除いて保存する。
type RenameProject struct {
	Title string `json:"title" pattern:"^[^\\r\\n]*$" doc:"1 行。改行は入れられない"`
}

// SetProjectStatus は、ステータスの変更の本文。
type SetProjectStatus struct {
	Status string `json:"status" enum:"Todo,InProgress,Done,Pending,Cancel"`
}

// SetProjectOwner は、オーナーの変更の本文。
type SetProjectOwner struct {
	Owner string `json:"owner" doc:"ログイン名。空は「なし」。メンバーでなければ 404"`
}

// SetProjectDate は、開始日か終了日の変更の本文。
type SetProjectDate struct {
	Field string `json:"field" enum:"start,end"`
	Value string `json:"value" pattern:"^(\\d{4}-\\d{2}-\\d{2})?$" doc:"YYYY-MM-DD。空は「なし」"`
}

// ProjectRepo は、リポジトリの追加・削除の本文。
type ProjectRepo struct {
	Repo string `json:"repo" doc:"owner/name。前後の空白は除く"`
}

// 入力の型は、パスの項目を埋め込みにせず直接持つ(埋め込むと huma がパスの値を入れない)。
type renameInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Body    RenameProject
}

type statusInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Body    SetProjectStatus
}

type ownerInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Body    SetProjectOwner
}

type dateInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Body    SetProjectDate
}

type repoInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
	Body    ProjectRepo
}

func one(p project.Project, err error) (*oneOutput, error) {
	if err != nil {
		return nil, apierr.From(err)
	}
	return &oneOutput{Body: toAPI(p)}, nil
}

func registerEdit(api huma.API, svc appprojects.Service) {
	const path = "/api/stores/{store}/projects/{project}"
	huma.Register(api, huma.Operation{OperationID: "rename-project", Method: http.MethodPut, Path: path + "/title", Summary: "プロジェクトのタイトルを変える"},
		func(ctx context.Context, in *renameInput) (*oneOutput, error) {
			return one(svc.Rename(ctx, port.StoreID(in.Store), project.ID(in.Project), in.Body.Title))
		})
	huma.Register(api, huma.Operation{OperationID: "set-project-status", Method: http.MethodPut, Path: path + "/status", Summary: "プロジェクトのステータスを変える"},
		func(ctx context.Context, in *statusInput) (*oneOutput, error) {
			return one(svc.SetStatus(ctx, port.StoreID(in.Store), project.ID(in.Project), project.Status(in.Body.Status)))
		})
	huma.Register(api, huma.Operation{OperationID: "set-project-owner", Method: http.MethodPut, Path: path + "/owner", Summary: "プロジェクトのオーナーを変える"},
		func(ctx context.Context, in *ownerInput) (*oneOutput, error) {
			return one(svc.SetOwner(ctx, port.StoreID(in.Store), project.ID(in.Project), in.Body.Owner))
		})
	huma.Register(api, huma.Operation{OperationID: "set-project-date", Method: http.MethodPut, Path: path + "/date", Summary: "プロジェクトの開始日か終了日を変える"},
		func(ctx context.Context, in *dateInput) (*oneOutput, error) {
			if err := checkDates(in.Body.Value); err != nil {
				return nil, err
			}
			return one(svc.SetDate(ctx, port.StoreID(in.Store), project.ID(in.Project), appprojects.DateField(in.Body.Field), in.Body.Value))
		})
	huma.Register(api, huma.Operation{OperationID: "add-project-repo", Method: http.MethodPost, Path: path + "/repos", Summary: "プロジェクトにリポジトリを結びつける"},
		func(ctx context.Context, in *repoInput) (*oneOutput, error) {
			return one(svc.AddRepo(ctx, port.StoreID(in.Store), project.ID(in.Project), in.Body.Repo))
		})
	huma.Register(api, huma.Operation{OperationID: "remove-project-repo", Method: http.MethodPost, Path: path + "/repos/remove", Summary: "プロジェクトからリポジトリを外す"},
		func(ctx context.Context, in *repoInput) (*oneOutput, error) {
			return one(svc.RemoveRepo(ctx, port.StoreID(in.Store), project.ID(in.Project), in.Body.Repo))
		})
}
```

`internal/adapter/httpapi/routes.go` の 69 行目(`projects.Register`)に `Clock` を足す(`items.Register` と同じ `deps.Clock`)。

```go
	projects.Register(api, appprojects.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity, Clock: deps.Clock})
```

- [ ] **Step 4: 生成物を作り直す**

Run: `make generate`
Expected: `api/openapi.yaml` が変わる(`/api/stores/{store}/projects/{project}/title` ほか 6 つのパスと、`Project` の `repos`)

Run: `pnpm -C web gen`
Expected: `web/packages/api/src/generated.ts` に `renameProject`・`setProjectStatus`・`setProjectOwner`・`setProjectDate`・`addProjectRepo`・`removeProjectRepo` と、`Project` の `repos: string[] | null`(`Item` の `watchers` と同じ)が増える。

- [ ] **Step 5: 通ることを確かめる**

Run: `go test ./internal/...`
Expected: PASS(`TestOpenAPIYAMLIsUpToDate` を含む)

Run: `pnpm -C web check`
Expected: PASS(`toProject` は `repos` を読まないので、型は崩れない)

- [ ] **Step 6: コミット**

```bash
git add internal/adapter/httpapi api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat(httpapi): プロジェクトの概要の編集とリポジトリの API(A4a-18)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 19: プロジェクトの概要の画面(編集・説明・リポジトリ・コメント・履歴・Epic 0 件)

概要の画面で、タイトル・ステータス・オーナー・開始日・終了日を直接編集し、説明・コメント・履歴の区画(他タスクの部品)と、リポジトリの区画、Epic が 0 件のときの文言、注記を出す。サーバーは Task 18。説明は Task 7 の `DescriptionSection.svelte`、コメントは Task 10 の `CommentsSection.svelte`、履歴は Task 17 の `HistorySection.svelte` を、`target = {kind: "project", storeId, id}` で置く。

app には部品のテストの基盤がない(A3b の記録)ので、編集の状態と規則は core の `createProjectEditor` に置いて vitest で確かめ、画面は svelte-check・Biome・ビルドと、Task 22 の e2e で確かめる。範囲は Epic が 0 件の文言まで。Epic の一覧の期限・進捗・ステータス・担当者の列と、「PR と Issue」の区画は足さない(前者は A2 から続く不足、後者は D3・D4。「骨組みへの指摘」8)。

**Files:**
- Modify: `web/packages/core/src/model/project.ts`(`repos`)
- Modify: `web/packages/core/src/port/projects.ts`(6 メソッド)
- Create: `web/packages/core/src/features/projects/editor.ts`・`editor.test.ts`
- Modify: `web/packages/core/src/index.ts`(export)
- Modify: `web/packages/core/src/features/catalog/fixtures.test-util.ts`(`project()` に `repos: []`)
- Modify: `web/packages/core/src/features/projects/createDialog.test.ts`(偽の `ProjectsPort` に 6 メソッド)
- Modify: `web/packages/api/src/projects.ts`
- Create: `web/packages/api/src/projects.ports.test.ts`
- Modify: `web/app/src/wire.ts`(`projectsPort` を取り出し、`projectEditor` を作る)
- Modify: `web/app/src/screens/project/ProjectOverview.svelte`(全文を置き換える)
- Create: `web/app/src/screens/project/OverviewProps.svelte`・`OverviewRepos.svelte`

**Interfaces:**
- Consumes: Task 18 の API(`web/packages/api/src/generated.ts` の `renameProject`・`setProjectStatus`・`setProjectOwner`・`setProjectDate`・`addProjectRepo`・`removeProjectRepo`)、`Catalog.load()`、`ProjectScreen.reload()`、`Toasts.show`、`readOnlyNote`・`ownerName`・`projectStatuses`(core)、`DescriptionSection`・`CommentsSection`・`HistorySection`(`web/app/src/screens/notes/`。props は `{ target: NoteTarget; readonly: boolean }`)
- Produces:
  - `Project.repos: string[]`(core のモデル)
  - `ProjectsPort` に `rename(storeId, projectId, title): Promise<Project>`・`setStatus(storeId, projectId, status: ProjectStatus): Promise<Project>`・`setOwner(storeId, projectId, owner: string): Promise<Project>`・`setDate(storeId, projectId, field: "start" | "end", value: string): Promise<Project>`・`addRepo(storeId, projectId, repo: string): Promise<Project>`・`removeRepo(storeId, projectId, repo: string): Promise<Project>`
  - `createProjectEditor(deps: ProjectEditorDeps): ProjectEditor`(`@hedgehug/core`)
    ```ts
    interface ProjectEditor {
      // 次の 4 つは、保存できたら true。false なら、画面は入力欄を保存済みの値に戻す(断りはトーストに出す)。
      rename(project: Project, title: string): Promise<boolean>;
      setStatus(project: Project, status: ProjectStatus): Promise<boolean>;
      setOwner(project: Project, owner: string): Promise<boolean>;
      setDate(project: Project, field: "start" | "end", value: string): Promise<boolean>;
      // addRepo は、追加できたら ""。断られたら、その文言(入力欄の下に出す。トーストは出さない)を返す。
      addRepo(project: Project, input: string): Promise<string>;
      // removeRepo は、外してトースト「{owner/name} を削除しました」を出す。
      removeRepo(project: Project, repo: string): Promise<void>;
    }
    interface ProjectEditorDeps {
      projects: ProjectsPort;
      toasts: Toasts;
      // 保存できたあと(一覧と開いているプロジェクトの読み直し)。
      onChanged(): Promise<void>;
    }
    ```
  - `projectEditor`(`wire.ts` の export)
  - 画面の名前(Task 22 の e2e が使う): 概要の「プロパティ」の区画の入力欄「タイトル」・「ステータス」・「オーナー」・「開始日」・「終了日」、「リポジトリと PR」の区画のボタン「リポジトリを追加」、入力欄「リポジトリ」、ボタン「追加」「キャンセル」、一覧「リポジトリ」、各行のボタン「{owner/name} を削除」

#### 19-1 core と api(口・編集の状態)

- [ ] **Step 1: 失敗するテストを書く**

`web/packages/core/src/features/projects/editor.test.ts`(新規):

```ts
import { describe, expect, it } from "vitest";
import { Refused } from "../../model/errors";
import type { Project } from "../../model/project";
import type { ProjectsPort } from "../../port/projects";
import { get } from "../../reactive/store";
import { project } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createProjectEditor } from "./editor";

// 偽の口: 呼び出しを覚え、断らせることができる。
function setup() {
  const calls: string[] = [];
  let refusal: string | null = null;
  const base: Project = { ...project("S1", "S1-W1", 1), repos: [] };
  const done = async (call: string) => {
    calls.push(call);
    if (refusal !== null) {
      const message = refusal;
      refusal = null;
      throw new Refused(message);
    }
    return base;
  };
  const projects: ProjectsPort = {
    get: async () => {
      throw new Error("unused");
    },
    create: async () => {
      throw new Error("unused");
    },
    rename: (s, p, title) => done(`rename ${s} ${p} ${title}`),
    setStatus: (s, p, status) => done(`setStatus ${s} ${p} ${status}`),
    setOwner: (s, p, owner) => done(`setOwner ${s} ${p} ${owner}`),
    setDate: (s, p, field, value) => done(`setDate ${s} ${p} ${field} ${value}`),
    addRepo: (s, p, repo) => done(`addRepo ${s} ${p} ${repo}`),
    removeRepo: (s, p, repo) => done(`removeRepo ${s} ${p} ${repo}`),
  };
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  let changed = 0;
  const editor = createProjectEditor({
    projects,
    toasts,
    onChanged: async () => {
      changed += 1;
    },
  });
  return {
    editor,
    base,
    calls,
    toasts,
    changed: () => changed,
    refuseNext: (message: string) => {
      refusal = message;
    },
  };
}

describe("createProjectEditor", () => {
  it("renames with the trimmed title, then reloads", async () => {
    const { editor, base, calls, changed } = setup();
    expect(await editor.rename(base, "  決済  ")).toBe(true);
    expect(calls).toEqual(["rename S1 S1-P1 決済"]);
    expect(changed()).toBe(1);
  });

  it("sends nothing when the value is the same as the saved one", async () => {
    const { editor, base, calls, changed } = setup();
    expect(await editor.rename(base, ` ${base.title} `)).toBe(true);
    expect(await editor.setStatus(base, base.status)).toBe(true);
    expect(await editor.setOwner(base, base.owner)).toBe(true);
    expect(await editor.setDate(base, "start", base.start)).toBe(true);
    expect(calls).toEqual([]);
    expect(changed()).toBe(0);
  });

  it("shows the server's refusal as an error toast and returns false", async () => {
    const { editor, base, toasts, changed, refuseNext } = setup();
    refuseNext("タイトルは空にできません");
    expect(await editor.rename(base, "   ")).toBe(false);
    const first = get(toasts.list)[0];
    expect(first?.text).toBe("タイトルは空にできません");
    expect(first?.error).toBe(true);
    expect(changed()).toBe(0);
  });

  it("sends the status, the owner and each date", async () => {
    const { editor, base, calls, changed } = setup();
    expect(await editor.setStatus(base, "InProgress")).toBe(true);
    expect(await editor.setOwner(base, "")).toBe(true);
    expect(await editor.setDate(base, "end", "2026-11-30")).toBe(true);
    expect(calls).toEqual([
      "setStatus S1 S1-P1 InProgress",
      "setOwner S1 S1-P1 ",
      "setDate S1 S1-P1 end 2026-11-30",
    ]);
    expect(changed()).toBe(3);
  });

  it("refuses a date range through the toast and leaves the form to be reset", async () => {
    const { editor, base, toasts, refuseNext } = setup();
    refuseNext("終了日は開始日以降にしてください");
    expect(await editor.setDate({ ...base, start: "2026-11-02" }, "end", "2026-11-01")).toBe(false);
    expect(get(toasts.list)[0]?.text).toBe("終了日は開始日以降にしてください");
  });

  it("adds a repo with the typed text and returns an empty message", async () => {
    const { editor, base, calls, changed, toasts } = setup();
    expect(await editor.addRepo(base, " example-org/web ")).toBe("");
    expect(calls).toEqual(["addRepo S1 S1-P1  example-org/web "]);
    expect(changed()).toBe(1);
    expect(get(toasts.list)).toEqual([]); // 追加ではトーストを出さない
  });

  it("returns the refusal of an add as the message, without a toast or a reload", async () => {
    const { editor, base, changed, toasts, refuseNext } = setup();
    refuseNext("もう追加されています");
    expect(await editor.addRepo(base, "example-org/web")).toBe("もう追加されています");
    expect(get(toasts.list)).toEqual([]);
    expect(changed()).toBe(0);
  });

  it("removes a repo, reloads and tells it with a toast", async () => {
    const { editor, base, calls, changed, toasts } = setup();
    await editor.removeRepo(base, "example-org/web");
    expect(calls).toEqual(["removeRepo S1 S1-P1 example-org/web"]);
    expect(changed()).toBe(1);
    const first = get(toasts.list)[0];
    expect(first?.text).toBe("example-org/web を削除しました");
    expect(first?.error).toBe(false);
  });
});
```

`web/packages/api/src/projects.ports.test.ts`(新規。`respond`・`call` は、ほかの口のテストと同じ書き方をこのファイルに持つ):

```ts
import type { Project, ProjectsPort } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createProjectsPort } from "./projects";

afterEach(() => {
  vi.unstubAllGlobals();
});

// respond は、fetch を 1 つの応答に差し替え、呼び出しを返す。
function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async () =>
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetch);
  setToken("tok");
  return fetch;
}

function call(fetch: ReturnType<typeof respond>, n = 0) {
  return fetch.mock.calls[n] as unknown as [string, RequestInit];
}

const apiProject = {
  id: "P1",
  workspace: "W1",
  number: 1,
  rank: "V",
  title: "ログイン",
  owner: "taro",
  status: "Todo",
  start: "",
  end: "",
  repos: ["example-org/web"],
};

interface Case {
  name: string;
  run: (port: ProjectsPort) => Promise<Project>;
  method: string;
  url: string;
  body: unknown;
}

const cases: Case[] = [
  {
    name: "rename",
    run: (p) => p.rename("S1", "P1", "決済"),
    method: "PUT",
    url: "/api/stores/S1/projects/P1/title",
    body: { title: "決済" },
  },
  {
    name: "setStatus",
    run: (p) => p.setStatus("S1", "P1", "InProgress"),
    method: "PUT",
    url: "/api/stores/S1/projects/P1/status",
    body: { status: "InProgress" },
  },
  {
    name: "setOwner",
    run: (p) => p.setOwner("S1", "P1", ""),
    method: "PUT",
    url: "/api/stores/S1/projects/P1/owner",
    body: { owner: "" },
  },
  {
    name: "setDate",
    run: (p) => p.setDate("S1", "P1", "end", "2026-11-30"),
    method: "PUT",
    url: "/api/stores/S1/projects/P1/date",
    body: { field: "end", value: "2026-11-30" },
  },
  {
    name: "addRepo",
    run: (p) => p.addRepo("S1", "P1", "example-org/web"),
    method: "POST",
    url: "/api/stores/S1/projects/P1/repos",
    body: { repo: "example-org/web" },
  },
  {
    name: "removeRepo",
    run: (p) => p.removeRepo("S1", "P1", "example-org/web"),
    method: "POST",
    url: "/api/stores/S1/projects/P1/repos/remove",
    body: { repo: "example-org/web" },
  },
];

describe("projects port: overview edits", () => {
  it.each(cases)("$name goes to its own endpoint and returns the project with its repos", async (c) => {
    const fetch = respond(200, apiProject);
    const project = await c.run(createProjectsPort());
    const [url, init] = call(fetch);
    expect(url).toBe(c.url);
    expect(init.method).toBe(c.method);
    expect(JSON.parse(String(init.body))).toEqual(c.body);
    expect(project.repos).toEqual(["example-org/web"]);
  });

  it("reads a null list of repos as empty", async () => {
    respond(200, { ...apiProject, repos: null });
    const project = await createProjectsPort().get("S1", "P1");
    expect(project.repos).toEqual([]);
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core exec vitest run src/features/projects/editor.test.ts`
Expected: FAIL(`Failed to resolve import "./editor"`)

Run: `pnpm -C web --filter @hedgehug/api exec vitest run src/projects.ports.test.ts`
Expected: FAIL(型が合わない、または `port.rename is not a function`)

- [ ] **Step 3: 実装する**

`web/packages/core/src/model/project.ts` の `Project` の末尾に足す。

```ts
  // repos は、結びつけた GitHub のリポジトリ(owner/name)。大文字小文字を区別しない辞書順。
  repos: string[];
```

`web/packages/core/src/port/projects.ts` を次にする。

```ts
import type { NewProject, Project, ProjectStatus } from "../model/project";

// プロジェクトの口。実装は api パッケージにある。
export interface ProjectsPort {
  get(storeId: string, projectId: string): Promise<Project>;
  create(storeId: string, workspaceId: string, input: NewProject): Promise<Project>;
  // 概要の編集(02「概要の編集」)。1 項目を変えるごとに 1 回の保存。検査と断りはサーバーが行う。
  rename(storeId: string, projectId: string, title: string): Promise<Project>;
  setStatus(storeId: string, projectId: string, status: ProjectStatus): Promise<Project>;
  // setOwner の owner は、ログイン名。空は「なし」。
  setOwner(storeId: string, projectId: string, owner: string): Promise<Project>;
  // setDate の value は YYYY-MM-DD。空は「なし」。
  setDate(
    storeId: string,
    projectId: string,
    field: "start" | "end",
    value: string,
  ): Promise<Project>;
  addRepo(storeId: string, projectId: string, repo: string): Promise<Project>;
  removeRepo(storeId: string, projectId: string, repo: string): Promise<Project>;
}
```

`web/packages/api/src/projects.ts` を次にする(`toProject` に `repos` を足し、口に 6 メソッドを足す)。

```ts
import type { Project, ProjectsPort } from "@hedgehug/core";
import { call } from "./errors";
import {
  type Project as ApiProject,
  addProjectRepo,
  createProject,
  getProject,
  removeProjectRepo,
  renameProject,
  setProjectDate,
  setProjectOwner,
  setProjectStatus,
} from "./generated";

export function toProject(storeId: string, p: ApiProject): Project {
  return {
    id: p.id,
    storeId,
    workspaceId: p.workspace,
    number: p.number,
    rank: p.rank,
    title: p.title,
    owner: p.owner,
    status: p.status,
    start: p.start,
    end: p.end,
    repos: p.repos ?? [],
  };
}

export function createProjectsPort(): ProjectsPort {
  return {
    get: (storeId, projectId) =>
      call(async () => toProject(storeId, await getProject(storeId, projectId))),
    create: (storeId, workspaceId, input) =>
      call(async () => toProject(storeId, await createProject(storeId, workspaceId, input))),
    rename: (storeId, projectId, title) =>
      call(async () => toProject(storeId, await renameProject(storeId, projectId, { title }))),
    setStatus: (storeId, projectId, status) =>
      call(async () => toProject(storeId, await setProjectStatus(storeId, projectId, { status }))),
    setOwner: (storeId, projectId, owner) =>
      call(async () => toProject(storeId, await setProjectOwner(storeId, projectId, { owner }))),
    setDate: (storeId, projectId, field, value) =>
      call(async () =>
        toProject(storeId, await setProjectDate(storeId, projectId, { field, value })),
      ),
    addRepo: (storeId, projectId, repo) =>
      call(async () => toProject(storeId, await addProjectRepo(storeId, projectId, { repo }))),
    removeRepo: (storeId, projectId, repo) =>
      call(async () => toProject(storeId, await removeProjectRepo(storeId, projectId, { repo }))),
  };
}
```

`web/packages/core/src/features/projects/editor.ts`(新規):

```ts
import { Refused } from "../../model/errors";
import type { Project, ProjectStatus } from "../../model/project";
import type { ProjectsPort } from "../../port/projects";
import type { Toasts } from "../toasts/toasts";

// プロジェクトの概要の編集(02「概要の編集」「リポジトリの追加と削除」)。検査と断りの文言はサーバーが持つので、
// ここは、値が前と同じなら送らないことと、断りの出し方(トーストか、入力欄の下か)だけを決める。

export interface ProjectEditor {
  // 次の 4 つは、保存できたら true。false なら、画面は入力欄を保存済みの値に戻す(断りはトーストに出す)。
  rename(project: Project, title: string): Promise<boolean>;
  setStatus(project: Project, status: ProjectStatus): Promise<boolean>;
  setOwner(project: Project, owner: string): Promise<boolean>;
  setDate(project: Project, field: "start" | "end", value: string): Promise<boolean>;
  // addRepo は、追加できたら ""。断られたら、その文言(入力欄の下に出す。トーストは出さない)を返す。
  addRepo(project: Project, input: string): Promise<string>;
  // removeRepo は、確認なしで外し、トースト「{owner/name} を削除しました」を出す。
  removeRepo(project: Project, repo: string): Promise<void>;
}

export interface ProjectEditorDeps {
  projects: ProjectsPort;
  toasts: Toasts;
  // onChanged は、保存できたあと(一覧と、開いているプロジェクトの読み直し)。
  onChanged(): Promise<void>;
}

export function createProjectEditor(deps: ProjectEditorDeps): ProjectEditor {
  // attempt は、保存を 1 回行う。断られたらその文言を返し(読み直さない)、保存できたら読み直して null を返す。
  // 断り以外のエラーはそのまま投げる。
  async function attempt(save: () => Promise<unknown>): Promise<string | null> {
    try {
      await save();
    } catch (e) {
      if (e instanceof Refused) return e.message;
      throw e;
    }
    await deps.onChanged();
    return null;
  }

  // attemptWithToast は、断りをエラーのトーストで出す。保存できたら true。
  async function attemptWithToast(save: () => Promise<unknown>): Promise<boolean> {
    const message = await attempt(save);
    if (message !== null) deps.toasts.show(message, { error: true });
    return message === null;
  }

  return {
    async rename(p, title) {
      const next = title.trim();
      if (next === p.title) return true;
      return attemptWithToast(() => deps.projects.rename(p.storeId, p.id, next));
    },
    async setStatus(p, status) {
      if (status === p.status) return true;
      return attemptWithToast(() => deps.projects.setStatus(p.storeId, p.id, status));
    },
    async setOwner(p, owner) {
      if (owner === p.owner) return true;
      return attemptWithToast(() => deps.projects.setOwner(p.storeId, p.id, owner));
    },
    async setDate(p, field, value) {
      if (value === p[field]) return true;
      return attemptWithToast(() => deps.projects.setDate(p.storeId, p.id, field, value));
    },
    async addRepo(p, input) {
      return (await attempt(() => deps.projects.addRepo(p.storeId, p.id, input))) ?? "";
    },
    async removeRepo(p, repo) {
      if (await attemptWithToast(() => deps.projects.removeRepo(p.storeId, p.id, repo))) {
        deps.toasts.show(`${repo} を削除しました`);
      }
    },
  };
}
```

`web/packages/core/src/index.ts` の `./features/projects/createDialog` の export の次(`screen` の前)に足す。

```ts
export {
  createProjectEditor,
  type ProjectEditor,
  type ProjectEditorDeps,
} from "./features/projects/editor";
```

型の合わせ(`Project` に `repos` を足し、口を広げたので、リテラルと偽物が型で落ちる):

- `web/packages/core/src/features/catalog/fixtures.test-util.ts` の `project()` の返り値の `end: "",` の次に `repos: [],` を足す。
- `web/packages/core/src/features/projects/createDialog.test.ts` の偽の `ProjectsPort`(`const projects: ProjectsPort = {` … `get`・`create`)に、使わないメソッドを足す。

```ts
    rename: unused,
    setStatus: unused,
    setOwner: unused,
    setDate: unused,
    addRepo: unused,
    removeRepo: unused,
```

`unused` は、`setup` の前に置く。

```ts
const unused = async (): Promise<never> => {
  throw new Error("unused");
};
```

(`get` の `async () => { throw new Error("unused"); }` は、そのままにしてよい。)

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test`
Expected: PASS(core・api・ui の全部。`editor.test.ts` の 8 件と `projects.ports.test.ts` の 7 件を含む)

Run: `pnpm -C web check`
Expected: PASS(ほかに `Project` を作っているテストの型が落ちたら、`repos: []` を足す)

- [ ] **Step 5: コミット**

```bash
git add web/packages
git commit -m "feat(core): プロジェクトの概要の編集の状態と口(A4a-18)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

#### 19-2 画面

- [ ] **Step 1: 組み立てる**

`web/app/src/wire.ts` を 3 か所直す。

1. `@hedgehug/core` の import に `createProjectEditor,` を足す(`createProjectDialog,` の次)。
2. `const catalogPort = createCatalogPort();` の次に足し、`projectDialog` の `projects:` をそれにする。

```ts
const projectsPort = createProjectsPort();
```

```ts
export const projectDialog = createProjectDialog({
  catalog,
  catalogPort,
  projects: projectsPort,
```

3. `projectDialog` の定義の次に足す。

```ts
export const projectEditor = createProjectEditor({
  projects: projectsPort,
  toasts,
  // 保存したら、一覧(サイドバーと見出しはここから出る)と開いているプロジェクトを読み直す。
  onChanged: async () => {
    await catalog.load();
    await projectScreen.reload();
  },
});
```

- [ ] **Step 2: 編集の区画を作る**

`web/app/src/screens/project/OverviewProps.svelte`(新規):

```svelte
<script lang="ts">
  import {
    ownerName,
    type ProjectStatus,
    type ProjectView,
    projectStatuses,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Input, Select } from "@hedgehug/ui";
  import { projectEditor } from "../../wire";

  // 概要の項目の編集(02「概要の編集」)。1 項目を変えるごとに 1 回保存する。検査と断りは core の projectEditor が持つ。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  const p = $derived(view.project);
  const disabled = $derived(view.store.readOnly);
  const hint = $derived(disabled ? readOnlyNote : undefined);
  const statuses = projectStatuses.map((s) => ({ value: s, label: s }));
  // オーナーは「なし」とメンバー。メンバーにいない人がオーナーのままのときは、その人も選択肢に出す。
  const owners = $derived([
    { value: "", label: "なし" },
    ...view.members.map((m) => ({ value: m.login, label: m.name })),
    ...(p.owner !== "" && !view.members.some((m) => m.login.toLowerCase() === p.owner.toLowerCase())
      ? [{ value: p.owner, label: ownerName(p.owner, view.members) }]
      : []),
  ]);

  // 入力欄の値。フォーカスしている欄は、画面への反映(保存して読み直したもの)で書き換えない(06)。
  // editing は $state にしない(フォーカスが外れたときに $effect が走り直し、保存の前に古い値へ戻すため)。
  const fields = $state({ title: "", start: "", end: "" });
  let editing = "";
  $effect(() => {
    const saved = { title: p.title, start: p.start, end: p.end };
    for (const key of ["title", "start", "end"] as const) {
      if (editing !== key) fields[key] = saved[key];
    }
  });

  // 保存できなかった(空・断り)ときは、保存済みの値に戻す。
  async function saveTitle() {
    editing = "";
    if (!(await projectEditor.rename(p, fields.title))) fields.title = p.title;
  }

  async function saveDate(key: "start" | "end") {
    editing = "";
    if (!(await projectEditor.setDate(p, key, fields[key]))) fields[key] = p[key];
  }

  // Enter でもフォーカスを外して保存する。日本語の変換を確定する Enter は除く。
  function onTitleKey(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) (event.currentTarget as HTMLElement).blur();
  }
</script>

<section class="props" aria-label="プロパティ">
  <div class="prop">
    <span class="prop-k">タイトル</span>
    <Input
      label="タイトル"
      size="sm"
      {disabled}
      title={hint}
      bind:value={fields.title}
      onfocus={() => (editing = "title")}
      onblur={() => void saveTitle()}
      onkeydown={onTitleKey}
    />
  </div>
  <div class="prop">
    <span class="prop-k">ステータス</span>
    <Select
      label="ステータス"
      size="sm"
      options={statuses}
      value={p.status}
      {disabled}
      title={hint}
      onchange={(v) => void projectEditor.setStatus(p, v as ProjectStatus)}
    />
  </div>
  <div class="prop">
    <span class="prop-k">オーナー</span>
    <Select
      label="オーナー"
      size="sm"
      options={owners}
      value={p.owner}
      {disabled}
      title={hint}
      onchange={(v) => void projectEditor.setOwner(p, v)}
    />
  </div>
  <div class="prop">
    <span class="prop-k">開始日</span>
    <Input
      label="開始日"
      type="date"
      size="sm"
      {disabled}
      title={hint}
      bind:value={fields.start}
      onfocus={() => (editing = "start")}
      onblur={() => void saveDate("start")}
    />
  </div>
  <div class="prop">
    <span class="prop-k">終了日</span>
    <Input
      label="終了日"
      type="date"
      size="sm"
      {disabled}
      title={hint}
      bind:value={fields.end}
      onfocus={() => (editing = "end")}
      onblur={() => void saveDate("end")}
    />
  </div>
  <p class="note">プロジェクトのプロパティは固定です。ステータスの値も変更できません。</p>
</section>

<style>
  .props {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .prop {
    display: grid;
    grid-template-columns: 90px minmax(0, 1fr);
    align-items: center;
  }
  .prop-k {
    color: var(--muted);
    font-size: 12.5px;
  }
  .note {
    margin: 4px 0 0;
    color: var(--muted);
    font-size: 12px;
  }
</style>
```

- [ ] **Step 3: リポジトリの区画を作る**

`web/app/src/screens/project/OverviewRepos.svelte`(新規。「PR と Issue」の行は D3・D4 が、この区画の下に足す):

```svelte
<script lang="ts">
  import { type ProjectView, readOnlyNote } from "@hedgehug/core";
  import { Button, IconButton, Input } from "@hedgehug/ui";
  import { projectEditor } from "../../wire";

  // 「リポジトリと PR」の区画の上段(02「概要の編集」「リポジトリの追加と削除」)。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  const p = $derived(view.project);
  const disabled = $derived(view.store.readOnly);
  const hint = $derived(disabled ? readOnlyNote : undefined);

  // 追加の入力欄は、「リポジトリを追加」で開く。断られたら開いたまま、断りの文言を欄の下に出す。
  let adding = $state(false);
  let input = $state("");
  let error = $state("");

  function open() {
    adding = true;
    error = "";
  }

  function cancel() {
    adding = false;
    input = "";
    error = "";
  }

  async function add() {
    error = await projectEditor.addRepo(p, input);
    if (error === "") cancel();
  }

  // Enter で追加する。日本語の変換を確定する Enter は除く。
  function onkeydown(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) void add();
  }
</script>

<section class="card-sec" aria-label="リポジトリと PR">
  <div class="sec-head">
    <h2>リポジトリと PR</h2>
    <span class="grow"></span>
    <Button size="sm" {disabled} title={hint} onclick={open}>リポジトリを追加</Button>
  </div>
  {#if adding}
    <div class="add">
      <Input
        label="リポジトリ"
        size="sm"
        placeholder="owner/name"
        focusOnMount
        bind:value={input}
        {onkeydown}
      />
      <Button size="sm" variant="primary" onclick={() => void add()}>追加</Button>
      <Button size="sm" onclick={cancel}>キャンセル</Button>
    </div>
    {#if error !== ""}
      <p class="error" role="alert">{error}</p>
    {/if}
  {/if}
  {#if p.repos.length === 0}
    <p class="empty">リポジトリはまだありません</p>
  {:else}
    <ul class="repos" aria-label="リポジトリ">
      {#each p.repos as repo (repo)}
        <li class="repo">
          <code>{repo}</code>
          <span class="grow"></span>
          <IconButton
            label="{repo} を削除"
            size="xs"
            {disabled}
            title={hint}
            onclick={() => void projectEditor.removeRepo(p, repo)}>×</IconButton
          >
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .sec-head {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .grow {
    flex: 1;
  }
  h2 {
    font-size: 14px;
    margin: 0 0 8px;
  }
  .add {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
  }
  .error {
    color: var(--danger);
    margin: 0 0 8px;
    font-size: 12.5px;
  }
  .empty {
    color: var(--muted);
    margin: 0;
  }
  .repos {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .repo {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    border-bottom: 1px solid var(--line);
  }
  code {
    font-family: var(--font-mono);
    font-size: 12.5px;
  }
</style>
```

- [ ] **Step 4: 概要の画面を組み直す**

`web/app/src/screens/project/ProjectOverview.svelte` を次の全文に置き換える(130 行 → 約 120 行。編集とリポジトリは別の部品に出した)。Epic の区画は、0 件の文言を足した以外は変えない。

```svelte
<script lang="ts">
  import {
    type Item,
    itemDisplayId,
    type ProjectView,
    type Route,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Button, KindIcon } from "@hedgehug/ui";
  import { detail, links } from "../../wire";
  import CommentsSection from "../notes/CommentsSection.svelte";
  import DescriptionSection from "../notes/DescriptionSection.svelte";
  import HistorySection from "../notes/HistorySection.svelte";
  import OverviewProps from "./OverviewProps.svelte";
  import OverviewRepos from "./OverviewRepos.svelte";

  // プロジェクトの「概要」のタブ(02「概要の編集」)。左の列に説明・Epic・リポジトリ・コメント・履歴、右の列に項目の編集。
  // 「PR と Issue」の区画は D3・D4 が、Epic の期限・進捗・担当者の列は別のトラックが足す。
  interface Props {
    view: ProjectView;
    onaddepic: () => void;
  }

  let { view, onaddepic }: Props = $props();

  const p = $derived(view.project);
  const readonly = $derived(view.store.readOnly);
  const target = $derived({ kind: "project" as const, storeId: view.store.id, id: p.id });
  // 項目を変えて保存したら、履歴を読み直す(HistorySection の refresh に渡す)。
  const historyKey = $derived([p.title, p.status, p.owner, p.start, p.end].join("\n"));
  const id = (it: Item) => itemDisplayId(view.workspace.prefix, it.number);
  const route = (it: Item): Route => ({ name: "item", store: view.store.name, item: id(it) });
</script>

<div class="overview-frame">
  <div class="overview">
    <div class="main">
      <DescriptionSection {target} {readonly} />
      <section class="card-sec" aria-label="Epic">
        <div class="sec-head">
          <h2>Epic</h2>
          <span class="grow"></span>
          <Button
            size="sm"
            disabled={readonly}
            title={readonly ? readOnlyNote : undefined}
            onclick={onaddepic}
            ><span aria-hidden="true">+</span>
            Epic を追加</Button
          >
        </div>
        {#each view.epics as e (e.id)}
          <div class="list-row">
            <KindIcon kind="Epic" />
            <code class="id">{id(e)}</code>
            <a
              class="row-title"
              href={links.href(route(e))}
              onclick={(ev) => links.click(ev, route(e), () => void detail.open(view.store.id, e.id))}
              onauxclick={(ev) => links.click(ev, route(e))}
              >{e.title}</a
            >
          </div>
        {:else}
          <p class="empty">Epic はまだありません。Epic の下に Story・Task・Bug を作れます。</p>
        {/each}
      </section>
      <OverviewRepos {view} />
      <CommentsSection {target} {readonly} />
      <!-- 履歴は項目の変更と説明の保存で読み直す(Task 17 の refresh。descriptionSaves は ../../wire から import する) -->
      <HistorySection {target} refresh={[historyKey, $descriptionSaves]} />
    </div>
    <OverviewProps {view} />
  </div>
</div>

<style>
  .overview-frame {
    container: overview / inline-size;
  }
  .overview {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 260px;
    gap: 24px;
    align-items: start;
  }
  @container overview (max-width: 640px) {
    .overview {
      grid-template-columns: minmax(0, 1fr);
    }
  }
  .main {
    display: flex;
    flex-direction: column;
    gap: 24px;
    min-width: 0;
  }
  .sec-head {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .grow {
    flex: 1;
  }
  h2 {
    font-size: 14px;
    margin: 0 0 8px;
  }
  .list-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    border-bottom: 1px solid var(--line);
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .row-title {
    color: var(--ink);
    text-decoration: none;
    flex: 1;
    min-width: 0;
  }
  .empty {
    color: var(--muted);
    margin: 0;
  }
</style>
```

(`formatDay`・`ownerName` の import と、`year`・`.props`・`.prop`・`.prop-k` は、編集の区画に移ったので、この全文にはない。)

- [ ] **Step 5: 確かめる**

Run: `pnpm -C web fix`(Biome の書式を直す)
Run: `pnpm -C web lint && pnpm -C web check && pnpm -C web test && pnpm -C web deps && pnpm -C web size && pnpm -C web build`
Expected: すべて PASS(Svelte の警告は `--fail-on-warnings` で落ちる。`ProjectOverview.svelte`・`OverviewProps.svelte`・`OverviewRepos.svelte` は 200 行以下)。他タスクの `notes/` の部品の props が `{ target, readonly }` と違うと、ここで型が落ちる。そのときは、部品の側の名前に合わせる(部品は変えない)。

Run: `make e2e-bin`
Run: `cd web && pnpm exec playwright test project.spec.ts`
Expected: PASS(3 幅。既存の概要の確認「Epic の区画に認証が出る」「閲覧のみの Store の帯」が、新しい概要でも通る)

- [ ] **Step 6: コミット**

```bash
git add web/app/src
git commit -m "feat(app): プロジェクトの概要で項目を編集し、リポジトリ・説明・コメント・履歴を出す(A4a-18・19・25)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 20: 読む用の item.yaml に関連・コメント・作業時間を書き出す

保存形式「アイテムの例」の `relations`・`comments`・`worklogs` を、`item.yaml` に書く。`item.yaml` はログから作る読む用の写しなので、ログ(正)は変えない。書く道(`Tx` の保存)と同期の作り直し(`rebuildYAML`)は、どちらも `renderYAML` を通るので、ここを直せば両方に出る。説明(`description.md`)は別のファイルなので `item.yaml` には書かない。

`gitstore` はテストを除くファイルが 20 個で上限なので、新しいファイルは足さず、`yaml.go`(構造体)と `yaml_notes.go`(`itemToYAML` と節の組み立て)に足す。

**Files:**
- Modify: `internal/infra/gitstore/yaml.go`(`itemYAML` に 3 つの項目)
- Modify: `internal/infra/gitstore/yaml_notes.go`(`itemToYAML` と、`relationsNode`・`commentsNode`・`worklogsNode`)
- Create: `internal/infra/gitstore/yaml_notes_test.go`

**Interfaces:**
- Consumes:
  - Task 9: `comment.FromLog(log []change.Record) []comment.Comment`(`ID`・`By`・`At`・`Body`・`Edited`。投稿の古い順、削除済みを除く)、`comment.Value(body string) any`、`change.NewComment`
  - Task 11: `worklog.FromLog(log []change.Record) []worklog.Entry`(`ID`・`By`・`Date`・`Hundredths`・`Memo`。記録の古い順、削除済みを除く)、`worklog.Value(hundredths int, memo, date string) any`、`worklog.Format`、`change.NewWorklog`
  - Task 13: `item.Item.Relations []item.Relation`(`Type`・`To`)。足した側のアイテムだけが持つ
  - 既存: `mapping`・`key`・`strNode`・`boolNode`・`seqOf`(`yaml_settings.go`)、`change.TimeLayout`、`renderYAML`・`rebuildYAML`・`ySeed`・`readFileString`・`mustContain`・`worktreeStatus`(テストの道具)
- Produces: `item.yaml` に、`fields` の次に `relations`、`watchers` の次に `comments`・`worklogs` の順で、次の形の行が出る(行は 1 件 1 行、なければ `[]`)。
  ```yaml
  relations:
    - {type: blocks, to: <相手の内部 ID>}
  comments:
    - {id: <内部 ID>, by: hanako, at: "2026-11-12T09:02:00.000+09:00", body: "…", edited: true}
  worklogs:
    - {id: <内部 ID>, by: taro, date: "2026-11-12", hours: 1.5, memo: "下書き"}
  ```

**書き方の決め(「仕様で決まっていなくて決めたこと」1〜3)**
- 関連の行末に、相手の表示の ID(`# HH-14`)は書かない。相手は別のワークスペースのこともあり、表示の ID はそのワークスペースの接頭辞に決まる。`parent`・`project` も、いまは表示の ID を書いていない。書くと、相手の接頭辞が変わったとき、別のワークスペースの `item.yaml` まで作り直す経路(`staleItems` は設定が変わったワークスペースの中だけを数える)が要る。
- 時刻は `change.TimeLayout`(`created.at` と同じ。このマシンの時刻帯付き)、`hours` は小数(`1.5`・`2`)で書く。
- `by` はログインの文字列のまま(表示名には直さない。メンバーの表示名を変えても作り直しが要らないように)。

- [ ] **Step 1: 失敗するテストを書く**

`internal/infra/gitstore/yaml_notes_test.go` を新しく作る。

```go
package gitstore

import (
	"fmt"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

// noteID は、26 文字の内部 ID(先頭に 01N を付け、s を右に寄せて 0 で埋める)。
func noteID(s string) change.ID { return change.ID(fmt.Sprintf("01N%023s", s)) }

// notesYAML は、item.yaml のうち、関連・コメント・作業時間の部分を読み戻す形。
type notesYAML struct {
	Relations []struct{ Type, To string } `yaml:"relations"`
	Comments  []struct {
		ID, By, At, Body string
		Edited           bool
	} `yaml:"comments"`
	Worklogs []struct {
		ID, By, Date string
		Hours        float64
		Memo         string
	} `yaml:"worklogs"`
}

// notesLog は、関連 1 つ・コメント 2 つ(1 つは編集、1 つは削除)・作業時間 3 つ(1 つは削除)を足したアイテムのログ。
func notesLog() []change.Record {
	jst := time.FixedZone("JST", 9*3600)
	n := 0
	newID := func() change.ID { n++; return noteID(fmt.Sprintf("R%d", n)) }
	var log []change.Record
	add := func(by string, minute int, ds ...change.Draft) {
		meta := change.Meta{Op: noteID(fmt.Sprintf("O%d", minute)), At: time.Date(2026, 11, 12, 9, minute, 0, 0, jst), Actor: change.User(by)}
		log = append(log, change.Stamp(log, ds, meta, newID)...)
	}
	it := item.Item{ID: noteID("A1"), Place: item.Place{Workspace: yWS, Project: yProj, Number: 1}, Kind: item.Task, Rank: "V", Title: "t"}
	first := comment.Value("@taro デザイン見てください\n二行目")
	add("hanako", 0, item.Create(it))
	add("hanako", 1, change.NewAdd("relations", item.Relation{Type: "blocks", To: noteID("B1")}))
	add("hanako", 2, change.NewComment(noteID("C1"), nil, first))
	add("hanako", 3, change.NewComment(noteID("C2"), nil, comment.Value("消すコメント")))
	add("hanako", 4, change.NewComment(noteID("C2"), comment.Value("消すコメント"), nil))
	add("hanako", 5, change.NewComment(noteID("C1"), first, comment.Value("直しました\n二行目も")))
	add("taro", 6, change.NewWorklog(noteID("W1"), nil, worklog.Value(150, "下書き", "2026-11-12")))
	add("taro", 7, change.NewWorklog(noteID("W2"), nil, worklog.Value(200, "", "2026-11-13")))
	add("taro", 8, change.NewWorklog(noteID("W3"), nil, worklog.Value(25, "消す", "2026-11-13")))
	add("taro", 9, change.NewWorklog(noteID("W3"), worklog.Value(25, "消す", "2026-11-13"), nil))
	return log
}

func TestItemYAMLHasRelationsCommentsWorklogs(t *testing.T) {
	out, err := renderYAML(change.KindItem, notesLog(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got notesYAML
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatalf("読み戻せない: %v\n%s", err, out)
	}
	if len(got.Relations) != 1 || got.Relations[0].Type != "blocks" || got.Relations[0].To != string(noteID("B1")) {
		t.Errorf("relations = %+v\n%s", got.Relations, out)
	}
	if len(got.Comments) != 1 {
		t.Fatalf("comments = %+v(削除したコメントが残っている、または出ていない)\n%s", got.Comments, out)
	}
	c := got.Comments[0]
	if c.ID != string(noteID("C1")) || c.By != "hanako" || c.At != "2026-11-12T09:02:00.000+09:00" || c.Body != "直しました\n二行目も" || !c.Edited {
		t.Errorf("comment = %+v(投稿者と時刻は最初の行、本文は編集のあと、複数行も保つ)\n%s", c, out)
	}
	if len(got.Worklogs) != 2 {
		t.Fatalf("worklogs = %+v(削除した記録が残っている、または出ていない)\n%s", got.Worklogs, out)
	}
	w1, w2 := got.Worklogs[0], got.Worklogs[1]
	if w1.ID != string(noteID("W1")) || w1.By != "taro" || w1.Date != "2026-11-12" || w1.Hours != 1.5 || w1.Memo != "下書き" {
		t.Errorf("worklog 1 = %+v\n%s", w1, out)
	}
	if w2.ID != string(noteID("W2")) || w2.Date != "2026-11-13" || w2.Hours != 2 || w2.Memo != "" {
		t.Errorf("worklog 2 = %+v\n%s", w2, out)
	}
	mustContain(t, "item.yaml", string(out),
		"relations:\n  - {type: blocks, to: "+string(noteID("B1"))+"}\n",
		"edited: true}\n",
	)
}

func TestItemYAMLNotesAreEmptyLists(t *testing.T) {
	it := item.Item{ID: noteID("A1"), Place: item.Place{Workspace: yWS, Project: yProj, Number: 1}, Kind: item.Task, Rank: "V", Title: "t"}
	meta := change.Meta{Op: noteID("O1"), At: time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC), Actor: change.User("taro")}
	log := change.Stamp(nil, []change.Draft{item.Create(it)}, meta, yIDs())
	out, err := renderYAML(change.KindItem, log, nil)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "item.yaml", string(out), "relations: []\n", "comments: []\n", "worklogs: []\n")
}

// 書く道で足した行が item.yaml に出て、同期の作り直し(rebuildYAML)も同じ中身を作る。
func TestWritingNotesShowsInItemYAML(t *testing.T) {
	s, id, dir, _ := ySeed(t)
	op := port.Operation{Actor: change.User("hanako"), Summary: "コメントと作業時間を足す"}
	err := s.Write(t.Context(), id, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(yItem,
			change.NewAdd("relations", item.Relation{Type: "blocks", To: noteID("B1")}),
			change.NewComment(noteID("C1"), nil, comment.Value("確認")),
			change.NewWorklog(noteID("W1"), nil, worklog.Value(150, "下書き", "2026-11-12")),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	name := "items/" + string(yItem) + "/item.yaml"
	written := readFileString(t, dir, name)
	var got notesYAML
	if err := yaml.Unmarshal([]byte(written), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Relations) != 1 || len(got.Comments) != 1 || got.Comments[0].By != "hanako" || len(got.Worklogs) != 1 || got.Worklogs[0].Hours != 1.5 {
		t.Errorf("item.yaml に足した行が出ていない:\n%s", written)
	}
	if !worktreeStatus(t, dir).IsClean() {
		t.Error("作業ツリーがきれいでない(item.yaml が同じコミットに入っていない)")
	}
	tr := tree{}
	for _, rel := range []string{"workspaces/" + string(yWS), "projects/" + string(yProj), "items/" + string(yItem)} {
		tr[rel+"/log.jsonl"] = []byte(readFileString(t, dir, rel+"/log.jsonl"))
	}
	if err := rebuildYAML(tr); err != nil {
		t.Fatal(err)
	}
	if string(tr[name]) != written {
		t.Errorf("同期の作り直しが書く道と違う:\n--- 同期\n%s\n--- 書く道\n%s", tr[name], written)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/infra/gitstore/ -run 'TestItemYAMLHasRelationsCommentsWorklogs|TestItemYAMLNotesAreEmptyLists|TestWritingNotesShowsInItemYAML'`
Expected: FAIL(`relations = []`、`comments = []`、`relations: []` がない。コンパイルは通る。コンパイルが通らないときは、Task 9・11・13 が入っていない)

- [ ] **Step 3: 構造体に 3 つの項目を足す**

`internal/infra/gitstore/yaml.go` の `itemYAML` を、`Fields` の次に `Relations`、`Watchers` の次に `Comments`・`Worklogs` が来る形にする(保存形式「アイテムの例」の並び)。

```go
	Fields    *yaml.Node  `yaml:"fields"`
	Relations *yaml.Node  `yaml:"relations"`
	Estimate  *float64    `yaml:"estimate"`
	Watchers  []string    `yaml:"watchers,flow"`
	Comments  *yaml.Node  `yaml:"comments"`
	Worklogs  *yaml.Node  `yaml:"worklogs"`
}
```

(`yaml.go` の `// 読む用の YAML の形。…行末の注釈(値の名前・表示の ID)はトラック A・B が足す。` の注意書きは、そのままにする。)

- [ ] **Step 4: 組み立てを足す**

`internal/infra/gitstore/yaml_notes.go` の import に 2 行足す。

```go
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
	"github.com/kukv/hedgehug/internal/domain/worklog"
```

`itemToYAML` の中の `Systems:` から `Watchers:` までの 3 行を、次の 6 行に置き換える。

```go
		Systems:   systemsNode(it.Systems, st),
		Fields:    fieldsNode(it, log, st),
		Relations: relationsNode(it.Relations),
		Watchers:  it.Watchers,
		Comments:  commentsNode(comment.FromLog(log)),
		Worklogs:  worklogsNode(worklog.FromLog(log)),
```

(`Points: it.Points, Estimate: it.Estimate,` の行はそのまま。揃え方が合わなければ `gofmt -w internal/infra/gitstore/yaml_notes.go` で直す。)

ファイルの末尾に足す。

```go
// relationsNode は、このアイテムが持つ関連を 1 件 1 行で書く(足した側だけが持つ。相手の側の逆向きの表記は
// ログから作らず、ここには書かない)。行末の注釈は付けない(相手は別のワークスペースのこともある)。
func relationsNode(rs []item.Relation) *yaml.Node {
	rows := make([]*yaml.Node, len(rs))
	for i, r := range rs {
		rows[i] = mapping(true, key("type"), strNode(r.Type), key("to"), strNode(string(r.To)))
	}
	return seqOf(rows)
}

// commentsNode は、いまのコメントを投稿の古い順に 1 件 1 行で書く。by・at は最初の行のもの。
func commentsNode(cs []comment.Comment) *yaml.Node {
	rows := make([]*yaml.Node, len(cs))
	for i, c := range cs {
		rows[i] = mapping(true,
			key("id"), strNode(string(c.ID)),
			key("by"), strNode(c.By),
			key("at"), strNode(c.At.Format(change.TimeLayout)),
			key("body"), strNode(c.Body),
			key("edited"), boolNode(c.Edited),
		)
	}
	return seqOf(rows)
}

// worklogsNode は、いまの作業時間を記録の古い順に 1 件 1 行で書く。時間は小数(1.5・2)。
func worklogsNode(es []worklog.Entry) *yaml.Node {
	rows := make([]*yaml.Node, len(es))
	for i, e := range es {
		hours := &yaml.Node{}
		_ = hours.Encode(float64(e.Hundredths) / 100)
		rows[i] = mapping(true,
			key("id"), strNode(string(e.ID)),
			key("by"), strNode(e.By),
			key("date"), strNode(e.Date),
			key("hours"), hours,
			key("memo"), strNode(e.Memo),
		)
	}
	return seqOf(rows)
}
```

(`seqOf` は 0 件のとき `[]` の 1 行にする。`strNode` は文字列の型を付けるので、`date: "2026-11-12"` のように日付らしい文字列は引用符が付き、読み戻しても文字列のまま。)

- [ ] **Step 5: 通ることを確かめる**

Run: `go test ./internal/infra/gitstore/`
Expected: PASS(新しい 3 つのテストと、既存の `TestRebuildYAMLMatchesWritePath`・`TestItemYAMLValues` ほか。`item.yaml` 全体を文字で比べる既存のテストがあって落ちたら、`relations: []`・`comments: []`・`worklogs: []` の 3 行が増えただけなので、期待のほうを直す)

Run: `gofmt -l internal/infra/gitstore/ ; go vet ./internal/infra/gitstore/ && make check`
Expected: gofmt の出力なし、`make check` PASS(`yaml_notes.go` は 200 行未満、`gitstore` のテストを除くファイルは 20 個のまま)

- [ ] **Step 6: コミット**

```bash
git add internal/infra/gitstore/yaml.go internal/infra/gitstore/yaml_notes.go internal/infra/gitstore/yaml_notes_test.go
git commit -m "feat(gitstore): item.yaml に関連・コメント・作業時間を書き出す(A4a-10)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 21: 概要の Epic の一覧に期限・進捗・ステータス・担当者を出す

02「概要の編集」の Epic の一覧を、仕様の形にする。各行は、ID・タイトル(押すと詳細パネル)・期限・進捗・ステータスのボタン・担当者の順。進捗は、その Epic の下の Story・Task・Bug を数え、棒と「N%」で出す。棒の上にポインターを置くと「{完了} / {対象}」が出る。中止は分母にも分子にも入れず、Step は数えない(04「集計の定義」)。ステータスのボタンは、ツリーの行と同じ `StatusButton`(ステータスのメニュー)を使う。

数え方は core に置き(vitest で確かめる)、棒は ui の新しい部品 `ProgressBar.svelte`(見た目だけ)、行は app の新しい部品 `EpicRow.svelte` に出す。`ProjectOverview.svelte` は Task 19 のあとの形(全文は Task 19 の Step 4)から、Epic の行の部分と、そこだけが使っていた CSS・import を入れ替える。期限・ステータス・担当者の出し方は、ツリーの行(`TreeRow.svelte`)と同じにする。

**Files:**
- Modify: `web/packages/core/src/features/items/progress.ts`・`progress.test.ts`(`epicProgress`・`progressPercent`)
- Modify: `web/packages/core/src/index.ts`(export)
- Create: `web/packages/ui/src/patterns/ProgressBar.svelte`・`ProgressBar.test.ts`
- Modify: `web/packages/ui/src/index.ts`(export)
- Create: `web/app/src/screens/project/EpicRow.svelte`
- Modify: `web/app/src/screens/project/ProjectOverview.svelte`(Task 19 のあとの形から、Epic の行を `EpicRow` に)

**Interfaces:**
- Consumes: 既存の `childCount`・`categoryOf`・`dueLabel`(`progress.ts`)、`StatusButton`(`web/app/src/screens/item/StatusButton.svelte`。props `{ status, settings, disabled, onchange }`)、`Avatar`・`KindIcon`(ui)、`projectScreen.setStatus(itemId, statusId)`(`wire.ts`)、`ProjectView.items`・`settings`・`members`(core の `screen.ts`)、Task 19 の `ProjectOverview.svelte`(`readonly`・`id`・`route` を持つ)
- Produces:
  - `epicProgress(items: Item[], epicId: string, settings: WorkspaceSettings): { done: number; total: number }`(`@hedgehug/core`。Epic の下のすべての子孫のうち、Step を除き、中止を除いた数と、完了の数。対象が 0 件なら `{ done: 0, total: 0 }`)
  - `progressPercent(p: { done: number; total: number }): number`(`@hedgehug/core`。四捨五入、対象が 0 件なら 0)
  - `ProgressBar`(`@hedgehug/ui`)。props `{ percent: number; detail: string }`。棒に `role="progressbar"`・名前「進捗」・`title={detail}`、棒のとなりに「N%」
  - `EpicRow.svelte`(app 内)。props `{ epic, displayId, href, items, settings, members, today, readOnly, onopen, onauxopen, onstatus }`

- [ ] **Step 1: core の失敗するテストを書く**

`web/packages/core/src/features/items/progress.test.ts` の import を置き換える。

```ts
import { describe, expect, it } from "vitest";
import type { Kind } from "../../model/item";
import { item } from "./fixtures.test-util";
import { categoryOf, childCount, dueLabel, epicProgress, progressPercent, stepProgress } from "./progress";
import { settings } from "./settings.test-util";
```

同じファイルの、最後の `});`(`describe("progress", …)` を閉じる行)の直前に足す。

```ts
  it("counts what is under an Epic except Steps and canceled ones", () => {
    const at = (n: number, kind: Kind, parentId: string, status: string) => ({
      ...item("S1", "S1-P1", n, kind, parentId),
      status,
    });
    const items = [
      item("S1", "S1-P1", 2, "Epic", null),
      at(3, "Story", "S1-I2", "st3"),
      at(4, "Task", "S1-I2", "st1"),
      at(5, "Bug", "S1-I2", "st4"),
      at(6, "Step", "S1-I4", "st3"),
      item("S1", "S1-P1", 8, "Epic", null),
      at(9, "Task", "S1-I8", "st3"),
    ];
    // 完了 1(Story)、未完了 1(Task)。Bug は中止、Step は数えない。別の Epic の Task は入れない。
    expect(epicProgress(items, "S1-I2", settings)).toEqual({ done: 1, total: 2 });
    expect(epicProgress(items, "S1-I8", settings)).toEqual({ done: 1, total: 1 });
    expect(epicProgress(items, "S1-I99", settings)).toEqual({ done: 0, total: 0 });
  });

  it("rounds the percent and gives 0 when there is nothing to count", () => {
    expect(progressPercent({ done: 0, total: 0 })).toBe(0);
    expect(progressPercent({ done: 1, total: 3 })).toBe(33);
    expect(progressPercent({ done: 2, total: 3 })).toBe(67);
    expect(progressPercent({ done: 1, total: 8 })).toBe(13);
    expect(progressPercent({ done: 4, total: 4 })).toBe(100);
  });
```

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core exec vitest run src/features/items/progress.test.ts`
Expected: FAIL(`epicProgress`・`progressPercent` が export されていない)

- [ ] **Step 3: core に数え方を足す**

`web/packages/core/src/features/items/progress.ts` の、`stepProgress` の下に足す。

```ts
// epicProgress は、概要の Epic の行の「完了/対象」(04「集計の定義」)。Epic の下の、Step を除くすべてのアイテムを数え、
// 中止は分母にも分子にも入れない。対象が 0 件なら { done: 0, total: 0 }。
export function epicProgress(items: Item[], epicId: string, settings: WorkspaceSettings) {
  const byParent = new Map<string, Item[]>();
  for (const it of items) {
    if (it.parentId === null) continue;
    byParent.set(it.parentId, [...(byParent.get(it.parentId) ?? []), it]);
  }
  const found: Item[] = [];
  const seen = new Set([epicId]);
  const queue = [epicId];
  for (let id = queue.pop(); id !== undefined; id = queue.pop()) {
    for (const c of byParent.get(id) ?? []) {
      if (seen.has(c.id)) continue;
      seen.add(c.id);
      queue.push(c.id);
      if (c.kind !== "Step") found.push(c);
    }
  }
  return childCount(found, settings);
}

// progressPercent は、進捗のパーセント(四捨五入。対象が 0 件なら 0。04「集計の定義」)。
export function progressPercent(p: { done: number; total: number }): number {
  return p.total === 0 ? 0 : Math.round((p.done / p.total) * 100);
}
```

`web/packages/core/src/index.ts` の `./features/items/progress` の export を置き換える。

```ts
export {
  categoryOf,
  childCount,
  dueLabel,
  epicProgress,
  progressPercent,
  stepProgress,
} from "./features/items/progress";
```

- [ ] **Step 4: 通ることを確かめて、コミットする**

Run: `pnpm -C web --filter @hedgehug/core exec vitest run src/features/items/progress.test.ts`
Expected: PASS(4 つのテスト)

Run: `pnpm -C web fix && pnpm -C web lint && pnpm -C web --filter @hedgehug/core exec tsc --noEmit`
Expected: エラーなし(`progress.ts` は 70 行未満)

```bash
git add web/packages/core/src/features/items/progress.ts web/packages/core/src/features/items/progress.test.ts web/packages/core/src/index.ts
git commit -m "feat(core): Epic の進捗(Step と中止を除く完了/対象)とパーセントの数え方を足す(A4a)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 5: ui の失敗するテストを書く**

`web/packages/ui/src/patterns/ProgressBar.test.ts` を新しく作る。

```ts
import { render, screen } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import ProgressBar from "./ProgressBar.svelte";

describe("ProgressBar", () => {
  it("shows the bar and N%, with 完了 / 対象 as the title of the bar", () => {
    const { container } = render(ProgressBar, { props: { percent: 38, detail: "3 / 8" } });
    const bar = screen.getByRole("progressbar", { name: "進捗" });
    expect(bar).toHaveAttribute("title", "3 / 8");
    expect(bar).toHaveAttribute("aria-valuenow", "38");
    expect(bar).toHaveAttribute("aria-valuetext", "3 / 8");
    expect(screen.getByText("38%")).toBeInTheDocument();
    expect(container.querySelector(".fill")).toHaveStyle({ width: "38%" });
  });

  it("is empty at 0%", () => {
    const { container } = render(ProgressBar, { props: { percent: 0, detail: "0 / 0" } });
    expect(screen.getByText("0%")).toBeInTheDocument();
    expect(container.querySelector(".fill")).toHaveStyle({ width: "0%" });
  });
});
```

Run: `pnpm -C web --filter @hedgehug/ui exec vitest run src/patterns/ProgressBar.test.ts`
Expected: FAIL(`ProgressBar.svelte` がない)

- [ ] **Step 6: ProgressBar を作る**

`web/packages/ui/src/patterns/ProgressBar.svelte`:

```svelte
<script lang="ts">
  // 進捗の棒と「N%」(04「集計の定義」)。棒の上にポインターを置くと detail(「{完了} / {対象}」)が出る。
  interface Props {
    percent: number;
    detail: string;
  }

  let { percent, detail }: Props = $props();
</script>

<span class="progress">
  <span
    class="bar"
    role="progressbar"
    aria-label="進捗"
    aria-valuemin={0}
    aria-valuemax={100}
    aria-valuenow={percent}
    aria-valuetext={detail}
    title={detail}
  >
    <span class="fill" style:width="{percent}%"></span>
  </span>
  <span class="pct">{percent}%</span>
</span>

<style>
  .progress {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    flex: none;
  }
  .bar {
    width: 56px;
    height: 6px;
    border-radius: 3px;
    background: var(--line);
    overflow: hidden;
  }
  .fill {
    display: block;
    height: 100%;
    background: var(--accent);
  }
  .pct {
    min-width: 3ch;
    text-align: right;
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
  }
</style>
```

`web/packages/ui/src/index.ts` の `OfflineBand` の行の次に足す。

```ts
export { default as ProgressBar } from "./patterns/ProgressBar.svelte";
```

- [ ] **Step 7: ui を確かめて、コミットする**

Run: `pnpm -C web --filter @hedgehug/ui exec vitest run src/patterns/ProgressBar.test.ts`
Expected: PASS(2 つのテスト)

Run: `pnpm -C web fix && pnpm -C web lint && pnpm -C web check`
Expected: エラー・警告なし

```bash
git add web/packages/ui/src/patterns/ProgressBar.svelte web/packages/ui/src/patterns/ProgressBar.test.ts web/packages/ui/src/index.ts
git commit -m "feat(ui): 進捗の棒と「N%」の部品を足す(A4a)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 8: Epic の行の部品を作る**

`web/app/src/screens/project/EpicRow.svelte`(ツリーの行 `TreeRow.svelte` と、期限・ステータス・担当者の出し方を同じにする。列の並びは 02 のとおり ID・タイトル・期限・進捗・ステータス・担当者)。

```svelte
<script lang="ts">
  import {
    categoryOf,
    dueLabel,
    epicProgress,
    type Item,
    type Member,
    progressPercent,
    type WorkspaceSettings,
  } from "@hedgehug/core";
  import { Avatar, KindIcon, ProgressBar } from "@hedgehug/ui";
  import StatusButton from "../item/StatusButton.svelte";

  // 概要の Epic の 1 行(02「概要の編集」の Epic の一覧)。進捗は Epic の下の Story・Task・Bug(Step と中止を除く)。
  interface Props {
    epic: Item;
    displayId: string;
    href: string;
    items: Item[];
    settings: WorkspaceSettings;
    members: Member[];
    today: Date;
    readOnly: boolean;
    onopen: (event: MouseEvent) => void;
    onauxopen: (event: MouseEvent) => void;
    onstatus: (statusId: string) => void;
  }

  let {
    epic,
    displayId,
    href,
    items,
    settings,
    members,
    today,
    readOnly,
    onopen,
    onauxopen,
    onstatus,
  }: Props = $props();

  const progress = $derived(epicProgress(items, epic.id, settings));
  const due = $derived(dueLabel(epic.end, categoryOf(settings, epic.status), today));
  const member = $derived(
    members.find((m) => m.login.toLowerCase() === epic.assignee.toLowerCase()),
  );
</script>

<div class="list-row">
  <KindIcon kind="Epic" />
  <code class="id">{displayId}</code>
  <a class="row-title" {href} onclick={onopen} onauxclick={onauxopen}>{epic.title}</a>
  {#if due}
    <span
      class="due"
      class:overdue={due.emphasis === "overdue"}
      class:today={due.emphasis === "today"}
      >{due.text}</span
    >
  {/if}
  <ProgressBar
    percent={progressPercent(progress)}
    detail="{progress.done} / {progress.total}"
  />
  <StatusButton status={epic.status} {settings} disabled={readOnly} onchange={onstatus} />
  {#if epic.assignee}
    <Avatar name={member?.name ?? ""} title={member?.name ?? "不明なユーザー"} />
  {/if}
</div>

<style>
  .list-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    padding: 2px 0;
    border-bottom: 1px solid var(--line);
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .row-title {
    color: var(--ink);
    text-decoration: none;
    flex: 1 1 10em;
    min-width: 0;
  }
  .due {
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
  }
  .overdue {
    color: var(--danger);
  }
  .today {
    font-weight: 600;
  }
</style>
```

(狭い幅では、タイトルが先に縮み、期限・進捗・ステータス・担当者は次の行に折り返す。)

- [ ] **Step 9: 概要を入れ替える**

`web/app/src/screens/project/ProjectOverview.svelte`(Task 19 のあとの形)を、次の 6 か所だけ変える。

(1) ui の import から `KindIcon` を外す(行は `EpicRow` に移った)。

```svelte
  import { Button } from "@hedgehug/ui";
```

(2) `wire` の import に `projectScreen` を足し、`EpicRow` を import する(import の並びは `pnpm -C web fix` が直す)。

```svelte
  import { detail, links, projectScreen } from "../../wire";
```
```svelte
  import EpicRow from "./EpicRow.svelte";
```

(3) 注記のコメントを置き換える。

```svelte
  // 「PR と Issue」の区画は D3・D4 が足す。Epic の行(期限・進捗・ステータス・担当者)は EpicRow.svelte。
```

(4) `const p = $derived(view.project);` の上に足す(期限の判定に使う今日は、画面を出したときの日付。ツリーと同じ)。

```svelte
  const today = new Date();
```

(5) `{#each view.epics as e (e.id)}` から `{:else}` の直前までを置き換える。

```svelte
        {#each view.epics as e (e.id)}
          <EpicRow
            epic={e}
            displayId={id(e)}
            href={links.href(route(e))}
            items={view.items}
            settings={view.settings}
            members={view.members}
            {today}
            readOnly={readonly}
            onopen={(ev) => links.click(ev, route(e), () => void detail.open(view.store.id, e.id))}
            onauxopen={(ev) => links.click(ev, route(e))}
            onstatus={(statusId) => void projectScreen.setStatus(e.id, statusId)}
          />
```

(6) `<style>` から、行の部品に移した 3 つの規則を消す(`.sec-head`・`.grow`・`h2`・`.main`・`.overview`・`.empty` は残す)。消さないと、使っていない CSS の警告で `--fail-on-warnings` が落ちる。

```css
  .list-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    border-bottom: 1px solid var(--line);
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .row-title {
    color: var(--ink);
    text-decoration: none;
    flex: 1;
    min-width: 0;
  }
```

- [ ] **Step 10: 確かめる**

Run: `pnpm -C web fix`
Run: `pnpm -C web lint && pnpm -C web check && pnpm -C web test && pnpm -C web deps && pnpm -C web size && pnpm -C web build`
Expected: すべて PASS(`ProjectOverview.svelte` は 120 行ほど、`EpicRow.svelte` は 110 行ほどで、どちらも 200 行以下。`deps` は ui が core を import していないことを見る。`ProgressBar` は props だけで core の型を使わない)

Run: `make e2e-bin`
Run: `cd web && pnpm exec playwright test project.spec.ts`
Expected: PASS(3 幅。既存の「Epic の区画に認証が出る」が、新しい行でも通る)

画面の見た目は、`make e2e-bin` で作った実行ファイルを動かし、サンプルのプロジェクトの概要を開いて目で確かめる(Epic の行に期限・棒と「N%」・ステータスのボタン・担当者が並ぶ。スマホの幅で折り返す。ステータスのボタンを押すと、ステータスのメニューが開き、選ぶと概要が読み直される。閲覧のみの Store ではボタンが無効)。

- [ ] **Step 11: コミット**

```bash
git add web/app/src/screens/project/EpicRow.svelte web/app/src/screens/project/ProjectOverview.svelte
git commit -m "feat(app): 概要の Epic の一覧に期限・進捗・ステータス・担当者を出す(A4a)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---


---

### Task 22: 通しのテスト(e2e)と PR

ここまでの全部を、Playwright で 3 つの幅(スマホ・タブレット・デスクトップ)の通しで確かめ、`make check` を通し、PR を 1 本出す。

e2e のサーバーは 3 つの幅とすべてのファイルで共有で、テストは並列に走る。サンプルのプロジェクト HH-P1 とアイテム HH-2 を書き換えると、`project.spec.ts`・`detail.spec.ts` ほかが壊れる。書くテストは、`createWorkspace` で毎回新しいワークスペースを作り、その中のプロジェクトとアイテムだけを触る(読み取りの確認だけ、閲覧のみの Store のサンプルを使う)。

**Files:**
- Modify: `web/e2e/helpers.ts`(`saved`・`newProject`・`projectWithEpic`・`newTaskUnder`・`taskUnderEpic` を足す)
- Modify: `web/e2e/create-template.spec.ts`(移した関数を import に変える)
- Create: `web/e2e/description.spec.ts`・`comments.spec.ts`・`worklog-relations.spec.ts`・`create-description.spec.ts`・`project-overview.spec.ts`・`notes-readonly.spec.ts`
- Modify: `docs/superpowers/plans/2026-11-13-hedgehug-track-a4a.md`(末尾に「実装で計画から変えたこと」と「A4b・C4 への申し送り」)

**Interfaces:**
- Consumes: ここまでの全部。`web/e2e/helpers.ts` の `createWorkspace`・`openSettings`・`prefixOf`・`tokenUrl`・`unique`
- Produces: `helpers.ts` に `saved(page, path)`・`newProject(page, info, prefix, n = 1): Promise<string>`(プロジェクト `接頭辞-Pn` を作り、概要を開いた状態でタイトルを返す)・`projectWithEpic(page, info, prefix): Promise<Locator>`(Epic `接頭辞-1` の詳細パネル)・`newTaskUnder(page, info, prefix): Promise<Locator>`(Task `接頭辞-2` の詳細パネル)・`taskUnderEpic(page, info, epicId, title): Promise<Locator>`(Epic の下に Task を作り、その詳細パネル)

**このタスクが前提にするロケーター(アクセシブルな名前)**

骨組みは、区画の名前を決めていない(「骨組みへの指摘」4)。次の表は、仕様の文言と既存の画面の書き方(`aria-label` が区画の名前、入力欄の名前は `label`)から決めたもので、Step 2 で実際の部品と突き合わせる。**合わないときは、このタスクのテストの側を直す**(部品の名前は変えない)。ただし、表の「要る属性」は部品に無いとテストが書けないので、無ければ該当タスクに戻して足す。

| 画面 | 名前・属性 | 部品 |
|---|---|---|
| 説明の区画 | region「説明」(`exact`)。空のとき button「説明を追加」。編集は、表示している本文の部分をクリックして始める。入力欄 textbox「説明」、下の文「Markdown を使用できます。@ログイン名 でメンション」、button「保存」「キャンセル」 | Task 7 |
| 入力の破棄の確認 | dialog「入力を破棄」(説明は「保存していない説明の変更を破棄します。」、コメントは「保存していないコメントを破棄します。」)、button「破棄」「キャンセル」。詳細パネルの閉じるは button「閉じる」(既存) | Task 6・7・10 |
| Markdown | 見出しは role heading、箇条書きは listitem、`strong`、行内の `code`(`p code`)、コードブロックは `pre`、リンクは `target="_blank"` と `rel` に `noopener`。メンションは **`data-mention="{小文字のログイン名}"` を持つ要素**(Store のメンバーと一致したものだけ) | Task 5 |
| コメントの区画 | region「コメント」(`exact`)。入力欄はプレースホルダ「コメントを書く。@ログイン名 でメンション。Ctrl+Enter で送信」。メンションのボタンは button「@hanako」(あなた以外)。1 件は listitem。自分のコメントに button「編集」「削除」。編集中は textbox と button「保存」「キャンセル」。「(編集済み)」。削除の確認は dialog「コメントを削除」 | Task 10 |
| ウォッチ | ヘッダーの button。ウォッチ中は名前が「ウォッチ中」、そうでなければ「ウォッチ」(`exact`)。閲覧のみでは無効 | Task 15 |
| 作業時間の区画 | region「作業時間」(`exact`)。入力は label「時間」(`exact`)・label「メモ」(`exact`)・button「記録」。合計は区画の見出しの中の「N 時間」。1 行は listitem(古い順)。行に button「削除」。確認は dialog「作業時間を削除」 | Task 12 |
| 関連の区画 | region「関連」(`exact`)。表記のセレクトは combobox「関連の種類」。相手の検索欄はプレースホルダ「ID またはタイトルで検索」。候補は role option(無効な候補は `aria-disabled` と理由の文)。button「追加」。表記ごとの見出しは role heading「blocks」など(`exact`)。行は相手の ID を含む link。解除は button「{相手の ID} の関連を解除」。ないときは「関連はありません」 | Task 14 |
| ブロック中 | 詳細パネルの種類の行と、ツリーの行の「ブロック中」の文字 | Task 14 |
| 履歴の区画 | region「履歴」(`exact`) | Task 17 |
| 概要の編集 | 本タスクの Task 19 のとおり(「プロパティ」の区画の入力欄「タイトル」「ステータス」「オーナー」「開始日」「終了日」、「リポジトリと PR」の区画) | Task 19 |

- [ ] **Step 1: 共通の手順を `helpers.ts` に移す**

`web/e2e/create-template.spec.ts` にある `saved`・`projectWithEpic`・`newTaskUnder` を `helpers.ts` に移す(A3b の Task 9 で `prefixOf`・`createWorkspace`・`openSettings` を移したのと同じ)。

`helpers.ts` の先頭の import を次にする。

```ts
import {
  type APIRequestContext,
  expect,
  type Locator,
  type Page,
  type TestInfo,
} from "@playwright/test";
```

`helpers.ts` の末尾に足す。

```ts
// saved は、設定の保存の応答を待つ。待たずにアドレスで移ると、保存が終わる前に次の画面を読むことがある。
export const saved = (page: Page, path: string) =>
  page.waitForResponse((r) => r.url().includes(path) && r.request().method() !== "GET" && r.ok());

// newProject は、ワークスペース prefix に n 番目のプロジェクト(接頭辞-Pn)を作り、その概要を開いた状態にする。
// 作ったプロジェクトのタイトルを返す。
export async function newProject(page: Page, info: TestInfo, prefix: string, n = 1) {
  const title = unique(info, `計画 ${prefix} ${n}`);
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}`));
  await page.getByRole("button", { name: "プロジェクト", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "プロジェクトを作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(title);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("heading", { name: `${prefix}-P${n} ${title}` })).toBeVisible();
  return title;
}

// projectWithEpic は、プロジェクト(接頭辞-P1)と Epic(接頭辞-1)を作り、Epic の詳細パネルを開いた状態にする。
export async function projectWithEpic(page: Page, info: TestInfo, prefix: string) {
  await newProject(page, info, prefix);
  const epic = unique(info, "通知");
  await page.getByRole("button", { name: "Epic", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(epic);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(dialog).toBeHidden();
  await expect(page.getByText(`${prefix}-1 を作成しました`)).toBeVisible();
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(epic);
  return panel;
}

// createTaskFrom は、Epic の詳細パネルの「Task」から Task を作り、作った Task の詳細パネルが開くのを待つ。
async function createTaskFrom(page: Page, panel: Locator, name: string) {
  await panel.getByRole("button", { name: "Task" }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(name);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(name);
}

// newTaskUnder は、projectWithEpic のあと、Epic の下に Task(接頭辞-2)を作り、その詳細パネルを開いた状態にする。
export async function newTaskUnder(page: Page, info: TestInfo, prefix: string) {
  const panel = await projectWithEpic(page, info, prefix);
  await createTaskFrom(page, panel, unique(info, "ログイン"));
  return panel;
}

// taskUnderEpic は、Epic(epicId は表示の ID)の詳細パネルを開き、その下に Task を作って、その詳細パネルを開いた状態にする。
export async function taskUnderEpic(page: Page, info: TestInfo, epicId: string, title: string) {
  await page.goto(tokenUrl(`/s/e2e/i/${epicId}`));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await createTaskFrom(page, panel, unique(info, title));
  return panel;
}
```

`create-template.spec.ts` から、`saved`・`projectWithEpic`・`newTaskUnder` の定義(コメントを含む)を消し、先頭の import を次にする(使わなくなった import は `pnpm -C web fix` が教える)。

```ts
import { expect, type Page, test } from "@playwright/test";
import {
  createWorkspace,
  newTaskUnder,
  openSettings,
  prefixOf,
  projectWithEpic,
  saved,
  tokenUrl,
  unique,
} from "./helpers";
```

Run: `pnpm -C web fix && pnpm -C web lint`
Expected: PASS

Run: `make e2e-bin && cd web && pnpm exec playwright test create-template`
Expected: PASS(3 幅。移しただけなので、A3b のテストが同じように通る。プロジェクトのタイトルだけ「テンプレの計画 …」から「計画 … 1」に変わる)

```bash
git add web/e2e
git commit -m "test(e2e): プロジェクトと Task を作る手順を helpers.ts に移す(A4a)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 2: ロケーターを実際の部品と突き合わせる**

Run: `grep -rn "aria-label\|label=\|placeholder=\|data-mention" web/app/src/screens/notes web/app/src/screens/item/Worklog*.svelte web/app/src/screens/item/Relations*.svelte web/app/src/screens/item/WatchButton.svelte web/packages/ui/src/patterns/Markdown.svelte`

上の表の名前と 1 つずつ照らす。違う名前は、このタスクの以降の spec の該当行を実際の名前に直す。表の「要る属性」(特に `data-mention`)が部品に無いときは、先に Task 5 などに戻して足す。

- [ ] **Step 3: 説明の通しのテスト**

`web/e2e/description.spec.ts`(新規):

```ts
import { expect, test } from "@playwright/test";
import { createWorkspace, newTaskUnder, prefixOf, tokenUrl, unique } from "./helpers";

// 詳細パネルの説明の通しのテスト(A4a)。ワークスペースは毎回作る(サーバーは 3 つの幅で共有)。

const markdown = [
  "# 見出し",
  "- 項目",
  "- [x] 済み",
  "**太字** と `code` と https://example.com/a",
  "@taro と @nobody",
  "```",
  "コード行",
  "```",
].join("\n");

test("edits a description, shows it as Markdown and asks before discarding", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "説明"), prefix);
  const panel = await newTaskUnder(page, info, prefix);
  const section = panel.getByRole("region", { name: "説明", exact: true });
  const box = section.getByRole("textbox", { name: "説明" });
  const heading = section.getByRole("heading", { name: "見出し" });

  // 空のときは「説明を追加」。入力欄の下に案内と 2 つのボタンが出る。Ctrl+Enter で保存する。
  await section.getByRole("button", { name: "説明を追加" }).click();
  await expect(
    section.getByText("Markdown を使用できます。@ログイン名 でメンション"),
  ).toBeVisible();
  await expect(section.getByRole("button", { name: "保存" })).toBeVisible();
  await box.fill(markdown);
  await box.press("Control+Enter");
  await expect(box).toBeHidden();

  // Markdown の表示。リンクは新しいタブで開き、メンションはメンバーのログイン名だけ。
  await expect(heading).toBeVisible();
  await expect(section.getByRole("listitem")).toHaveCount(2);
  await expect(section.locator("strong")).toHaveText("太字");
  await expect(section.locator("p code")).toHaveText("code");
  await expect(section.locator("pre")).toContainText("コード行");
  const link = section.getByRole("link", { name: "https://example.com/a" });
  await expect(link).toHaveAttribute("target", "_blank");
  await expect(link).toHaveAttribute("rel", /noopener/);
  await expect(section.locator('[data-mention="taro"]')).toHaveText("@taro");
  await expect(section.locator("[data-mention]")).toHaveCount(1);

  // 入力欄の中で Esc を押すと、中身が保存してあるものと違うので確認する。「キャンセル」で入力欄に戻る。
  await heading.click();
  await box.fill("# 見出し\n変えた");
  await box.press("Escape");
  const discard = page.getByRole("dialog", { name: "入力を破棄" });
  await expect(discard).toContainText("保存していない説明の変更を破棄します。");
  await discard.getByRole("button", { name: "キャンセル" }).click();
  await expect(box).toHaveValue("# 見出し\n変えた");
  await box.press("Escape");
  await discard.getByRole("button", { name: "破棄" }).click();
  await expect(box).toBeHidden();
  await expect(heading).toBeVisible();

  // 入力欄の下の「キャンセル」は、確認なしで捨てる。
  await heading.click();
  await box.fill("別の文");
  await section.getByRole("button", { name: "キャンセル" }).click();
  await expect(discard).toBeHidden();
  await expect(box).toBeHidden();

  // パネルを閉じるときも確認する。
  await heading.click();
  await box.fill("閉じる前の文");
  await panel.getByRole("button", { name: "閉じる" }).click();
  await expect(discard).toBeVisible();
  await discard.getByRole("button", { name: "破棄" }).click();
  await expect(panel).toBeHidden();
});

test("saving over a line another save changed keeps both and says so", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "説明の競合"), prefix);
  const panel = await newTaskUnder(page, info, prefix);
  const section = panel.getByRole("region", { name: "説明", exact: true });
  const box = section.getByRole("textbox", { name: "説明" });
  await section.getByRole("button", { name: "説明を追加" }).click();
  await box.fill("1行目\n2行目");
  await box.press("Control+Enter");
  await expect(box).toBeHidden();

  // 別のタブ(同じサーバー)で同じアイテムを開く。
  const other = await page.context().newPage();
  await other.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  const otherSection = other
    .getByRole("region", { name: "詳細パネル" })
    .getByRole("region", { name: "説明", exact: true });
  const otherBox = otherSection.getByRole("textbox", { name: "説明" });

  // このタブは編集を始める(共通の祖先は「1行目\n2行目」)。
  await section.getByText("2行目").click();
  await box.fill("1行目\n2行目 (A)");
  // 別のタブが、同じ行を変えて先に保存する。
  await otherSection.getByText("2行目").click();
  await otherBox.fill("1行目\n2行目 (B)");
  await otherBox.press("Control+Enter");
  await expect(otherSection).toContainText("2行目 (B)");

  // このタブの保存で、両方の版が残り、トーストで知らせる。
  await box.press("Control+Enter");
  await expect(
    page.getByText("説明の同じ箇所が同時に編集されたため、両方を残しました"),
  ).toBeVisible();
  await expect(section).toContainText("2行目 (A)");
  await expect(section).toContainText("2行目 (B)");
  await other.close();
});
```

Run: `cd web && pnpm exec playwright test description`
Expected: PASS(3 幅)。落ちたら、画面の振る舞いが仕様(02「説明」「Markdown」)どおりかを先に確かめ、テストの誤りか実装の誤りかを見分ける。

```bash
git add web/e2e/description.spec.ts
git commit -m "test(e2e): 説明の編集・Markdown・入力の破棄・同じ行の保存の通しのテスト(A4a)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 4: コメント・ウォッチ・履歴の通しのテスト**

`web/e2e/comments.spec.ts`(新規):

```ts
import { expect, test } from "@playwright/test";
import { createWorkspace, newTaskUnder, prefixOf, tokenUrl, unique } from "./helpers";

// コメント・ウォッチ・履歴の通しのテスト(A4a)。

test("posts, edits and deletes a comment, and the mention button adds the login", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "コメント"), prefix);
  const panel = await newTaskUnder(page, info, prefix);
  const section = panel.getByRole("region", { name: "コメント", exact: true });
  const box = section.getByPlaceholder(
    "コメントを書く。@ログイン名 でメンション。Ctrl+Enter で送信",
  );
  const comments = section.getByRole("listitem");

  // 書きかけのまま Esc を押すと確認する。
  await box.fill("書きかけ");
  await box.press("Escape");
  const discard = page.getByRole("dialog", { name: "入力を破棄" });
  await expect(discard).toContainText("保存していないコメントを破棄します。");
  await discard.getByRole("button", { name: "キャンセル" }).click();
  await expect(box).toHaveValue("書きかけ");

  // メンションのボタンは、あなた(taro)以外のメンバーだけ。押すと末尾に「@ログイン名 」が入る。
  await box.fill("確認をお願いします ");
  await expect(section.getByRole("button", { name: "@taro" })).toHaveCount(0);
  await section.getByRole("button", { name: "@hanako" }).click();
  await expect(box).toHaveValue("確認をお願いします @hanako ");
  await box.press("Control+Enter");
  await expect(box).toHaveValue("");
  await expect(comments).toHaveCount(1);
  await expect(comments.first()).toContainText("taro");
  await expect(comments.first()).toContainText("確認をお願いします");
  await expect(comments.first().locator('[data-mention="hanako"]')).toBeVisible();
  await expect(comments.first()).not.toContainText("(編集済み)");

  // 編集。前と同じなら、何も保存せずに閉じる(「(編集済み)」は付かない)。
  await comments.first().getByRole("button", { name: "編集" }).click();
  await comments.first().getByRole("button", { name: "保存" }).click();
  await expect(comments.first()).not.toContainText("(編集済み)");
  await comments.first().getByRole("button", { name: "編集" }).click();
  await comments.first().getByRole("textbox").fill("確認しました");
  await comments.first().getByRole("button", { name: "保存" }).click();
  await expect(comments.first()).toContainText("確認しました");
  await expect(comments.first()).toContainText("(編集済み)");

  // 削除は確認してから。
  await comments.first().getByRole("button", { name: "削除" }).click();
  const dialog = page.getByRole("dialog", { name: "コメントを削除" });
  await expect(dialog).toContainText("このコメントを削除します。");
  await dialog.getByRole("button", { name: "削除", exact: true }).click();
  await expect(page.getByText("コメントを削除しました")).toBeVisible();
  await expect(comments).toHaveCount(0);
});

test("toggles watching from the header", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "ウォッチ"), prefix);
  const panel = await newTaskUnder(page, info, prefix);
  // 作った人は、最初からウォッチ中。
  await expect(panel.getByRole("button", { name: "ウォッチ中" })).toBeVisible();
  await panel.getByRole("button", { name: "ウォッチ中" }).click();
  await expect(panel.getByRole("button", { name: "ウォッチ", exact: true })).toBeVisible();
  // 読み直しても残る。
  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  await expect(
    page.getByRole("region", { name: "詳細パネル" }).getByRole("button", { name: "ウォッチ", exact: true }),
  ).toBeVisible();
  await page.getByRole("region", { name: "詳細パネル" }).getByRole("button", { name: "ウォッチ", exact: true }).click();
  await expect(
    page.getByRole("region", { name: "詳細パネル" }).getByRole("button", { name: "ウォッチ中" }),
  ).toBeVisible();
});

test("the history lists the creation and a title change, not comments", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "履歴"), prefix);
  const panel = await newTaskUnder(page, info, prefix);
  const renamed = unique(info, "名前を変えた Task");
  const title = panel.getByRole("textbox", { name: "タイトル" });
  await title.fill(renamed);
  await title.press("Enter");
  await expect(title).toHaveValue(renamed);
  const box = panel
    .getByRole("region", { name: "コメント", exact: true })
    .getByPlaceholder("コメントを書く。@ログイン名 でメンション。Ctrl+Enter で送信");
  await box.fill("履歴には出ない");
  await box.press("Control+Enter");
  await expect(
    panel.getByRole("region", { name: "コメント", exact: true }).getByRole("listitem"),
  ).toHaveCount(1);

  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  const history = page
    .getByRole("region", { name: "詳細パネル" })
    .getByRole("region", { name: "履歴", exact: true });
  await expect(history).toContainText("作成");
  await expect(history).toContainText(`タイトルを「${renamed}」に変更`);
  await expect(history).toContainText("taro");
  await expect(history).not.toContainText("履歴には出ない");
});
```

Run: `cd web && pnpm exec playwright test comments`
Expected: PASS(3 幅)

```bash
git add web/e2e/comments.spec.ts
git commit -m "test(e2e): コメント・ウォッチ・履歴の通しのテスト(A4a)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 5: 作業時間と関連の通しのテスト**

`web/e2e/worklog-relations.spec.ts`(新規):

```ts
import { expect, test } from "@playwright/test";
import { createWorkspace, newTaskUnder, prefixOf, taskUnderEpic, tokenUrl, unique } from "./helpers";

// 作業時間と関連(ブロック中を含む)の通しのテスト(A4a)。

test("records, checks and deletes work time", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "作業時間"), prefix);
  const panel = await newTaskUnder(page, info, prefix);
  const section = panel.getByRole("region", { name: "作業時間", exact: true });
  const hours = section.getByLabel("時間", { exact: true });
  const memo = section.getByLabel("メモ", { exact: true });
  const record = section.getByRole("button", { name: "記録" });
  const rows = section.getByRole("listitem");

  // 0 以下・空は「時間を入力してください」、小数第 3 位以下は別の文言。どちらも記録しない。
  await hours.fill("0");
  await record.click();
  await expect(section.getByText("時間を入力してください")).toBeVisible();
  await hours.fill("0.123");
  await record.click();
  await expect(section.getByText("時間は小数第 2 位までで入力してください")).toBeVisible();
  await expect(rows).toHaveCount(0);

  // 記録する。行には「N 時間」とメモ、見出しには合計。メモ欄の Enter でも記録する。
  await hours.fill("1.25");
  await memo.fill("調査");
  await record.click();
  await expect(rows).toHaveCount(1);
  await expect(rows.first()).toContainText("1.25 時間");
  await expect(rows.first()).toContainText("調査");
  await expect(section).toContainText("1.25 時間");
  await hours.fill("0.75");
  await memo.press("Enter");
  await expect(rows).toHaveCount(2);
  await expect(section).toContainText("2 時間");

  // 削除は確認してから。古い順なので、先頭が 1.25 時間の記録。合計は 0.75 時間になる。
  await rows.first().getByRole("button", { name: "削除" }).click();
  const dialog = page.getByRole("dialog", { name: "作業時間を削除" });
  await expect(dialog).toContainText("の 1.25 時間の記録を削除します。");
  await dialog.getByRole("button", { name: "削除", exact: true }).click();
  await expect(page.getByText("作業時間を削除しました")).toBeVisible();
  await expect(rows).toHaveCount(1);
  await expect(section).toContainText("0.75 時間");
});

test("adds a relation, shows it from the other side, blocks and checks candidates", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "関連"), prefix);
  await newTaskUnder(page, info, prefix); // Task 接頭辞-2 (Epic は接頭辞-1)
  await taskUnderEpic(page, info, `${prefix}-1`, "別の Task"); // Task 接頭辞-3
  const a = `${prefix}-2`;
  const b = `${prefix}-3`;
  const panel = page.getByRole("region", { name: "詳細パネル" });
  const tree = page.getByRole("list", { name: "ツリー" });
  const section = panel.getByRole("region", { name: "関連", exact: true });
  const kind = section.getByRole("combobox", { name: "関連の種類" });
  const search = section.getByPlaceholder("ID またはタイトルで検索");

  // A に「blocks B」を足す。
  await page.goto(tokenUrl(`/s/e2e/i/${a}`));
  await expect(section).toContainText("関連はありません");
  await kind.selectOption({ label: "blocks" });
  await search.fill(b);
  await section.getByRole("option", { name: new RegExp(b) }).click();
  await section.getByRole("button", { name: "追加" }).click();
  await expect(section.getByRole("heading", { name: "blocks", exact: true })).toBeVisible();
  await expect(section.getByRole("link", { name: new RegExp(b) })).toBeVisible();

  // B 側には逆向きの表記で出て、B は「ブロック中」になる(詳細パネルとツリーの行)。
  await page.goto(tokenUrl(`/s/e2e/i/${b}`));
  await expect(section.getByRole("heading", { name: "blocked by", exact: true })).toBeVisible();
  await expect(section.getByRole("link", { name: new RegExp(a) })).toBeVisible();
  await expect(panel.getByText("ブロック中")).toBeVisible();
  await expect(tree.getByText("ブロック中")).toBeVisible();

  // 候補の検査。B から A を blocks にすると循環、blocked by にすると同じ種類の関連がすでにある。
  await kind.selectOption({ label: "blocks" });
  await search.fill(a);
  await expect(section.getByRole("option", { name: new RegExp(a) })).toBeDisabled();
  await expect(section.getByText("循環になります")).toBeVisible();
  await kind.selectOption({ label: "blocked by" });
  await search.fill(a);
  await expect(section.getByRole("option", { name: new RegExp(a) })).toBeDisabled();
  await expect(section.getByText("すでに blocked by の関連があります")).toBeVisible();

  // 解除は確認なし。解除すると、B は「ブロック中」でなくなる。
  await page.goto(tokenUrl(`/s/e2e/i/${a}`));
  await section.getByRole("button", { name: `${b} の関連を解除` }).click();
  await expect(section).toContainText("関連はありません");
  await page.goto(tokenUrl(`/s/e2e/i/${b}`));
  await expect(section).toContainText("関連はありません");
  await expect(panel.getByText("ブロック中")).toHaveCount(0);
});
```

Run: `cd web && pnpm exec playwright test worklog-relations`
Expected: PASS(3 幅)

```bash
git add web/e2e/worklog-relations.spec.ts
git commit -m "test(e2e): 作業時間・関連・ブロック中の通しのテスト(A4a)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 6: 作成ダイアログの説明とテンプレートの通しのテスト**

`web/e2e/create-description.spec.ts`(新規):

```ts
import { expect, test } from "@playwright/test";
import { createWorkspace, openSettings, prefixOf, projectWithEpic, saved, unique } from "./helpers";

// 作成ダイアログの説明の欄と、テンプレートの説明の当てはめの通しのテスト(A4a)。

test("fills the description from the template and keeps what the user typed", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "作成の説明"), prefix);
  // テンプレート「説明つき」: 種類 Task、説明「テンプレートの説明」。Task の既定にする。
  await openSettings(page, prefix, "テンプレート");
  await page.getByRole("button", { name: "テンプレートを作成" }).click();
  const template = page.getByRole("dialog", { name: "テンプレートを作成" });
  await template.getByRole("textbox", { name: "名前" }).fill("説明つき");
  await template.getByRole("combobox", { name: "種類" }).selectOption({ label: "Task" });
  await template.getByRole("textbox", { name: "説明" }).fill("テンプレートの説明");
  await template.getByRole("button", { name: "保存", exact: true }).click();
  await expect(template).toBeHidden();
  const taskDefault = page.getByRole("combobox", { name: "Task", exact: true });
  await Promise.all([
    saved(page, "/template-defaults/"),
    taskDefault.selectOption({ label: "説明つき" }),
  ]);

  const panel = await projectWithEpic(page, info, prefix);
  await page.keyboard.press("Escape");
  await expect(panel).toBeHidden();

  // 開くと、既定のテンプレートの説明が入っている。
  await page.keyboard.press("c");
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  const description = dialog.getByRole("textbox", { name: "説明" });
  const picker = dialog.getByRole("combobox", { name: "テンプレート" });
  await expect(description).toHaveValue("テンプレートの説明");

  // 「なし」に戻すと、テンプレート由来の説明は空に戻る。選び直すと、テンプレートの説明で置き換わる。
  await picker.selectOption({ label: "なし" });
  await expect(description).toHaveValue("");
  await picker.selectOption({ label: "説明つき" });
  await expect(description).toHaveValue("テンプレートの説明");

  // 利用者が入力した説明は、「なし」に戻しても残る。
  await description.fill("# 手順\n自分の説明");
  await picker.selectOption({ label: "なし" });
  await expect(description).toHaveValue("# 手順\n自分の説明");

  // 説明つきで作ると、詳細パネルに Markdown で出る。
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("説明つきの Task");
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(page.getByText(`${prefix}-2 を作成しました`)).toBeVisible();
  await expect(
    panel
      .getByRole("region", { name: "説明", exact: true })
      .getByRole("heading", { name: "手順" }),
  ).toBeVisible();

  // 「続けて作成」: 作ったあと、タイトルと説明だけが空になり、テンプレートは残る。
  await page.keyboard.press("Escape");
  await page.keyboard.press("c");
  await dialog.getByRole("checkbox", { name: "続けて作成" }).check();
  await picker.selectOption({ label: "説明つき" });
  await expect(description).toHaveValue("テンプレートの説明");
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("続けて作る Task");
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(page.getByText(`${prefix}-3 を作成しました`)).toBeVisible();
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("textbox", { name: "タイトル" })).toHaveValue("");
  await expect(description).toHaveValue("");
  await expect(picker.locator("option:checked")).toHaveText("説明つき");
});
```

Run: `cd web && pnpm exec playwright test create-description`
Expected: PASS(3 幅)

```bash
git add web/e2e/create-description.spec.ts
git commit -m "test(e2e): 作成ダイアログの説明とテンプレートの説明の当てはめの通しのテスト(A4a)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 7: プロジェクトの概要と閲覧のみの通しのテスト**

`web/e2e/project-overview.spec.ts`(新規):

```ts
import { expect, test } from "@playwright/test";
import { createWorkspace, newProject, prefixOf, tokenUrl, unique } from "./helpers";

// プロジェクトの概要の編集・リポジトリの通しのテスト(A4a)。HH-P1 は他のテストが使うので触らず、
// ワークスペースとプロジェクトを毎回作る。

test("edits the overview's properties, one save per item, and shows the history", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "概要"), prefix);
  await newProject(page, info, prefix);
  const other = await newProject(page, info, prefix, 2);
  await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/overview`));
  const props = page.getByRole("region", { name: "プロパティ", exact: true });
  const title = props.getByRole("textbox", { name: "タイトル" });

  // Epic が 0 件の文言と注記。
  await expect(
    page
      .getByRole("region", { name: "Epic", exact: true })
      .getByText("Epic はまだありません。Epic の下に Story・Task・Bug を作れます。"),
  ).toBeVisible();
  await expect(
    page.getByText("プロジェクトのプロパティは固定です。ステータスの値も変更できません。"),
  ).toBeVisible();

  // タイトル: 空は断って元に戻す。ほかのプロジェクトと同じ名前も断る。Enter で保存し、見出しも変わる。
  const before = await title.inputValue();
  await title.fill("   ");
  await title.press("Enter");
  await expect(page.getByText("タイトルは空にできません")).toBeVisible();
  await expect(title).toHaveValue(before);
  await title.fill(other);
  await title.press("Enter");
  await expect(
    page.getByText("このワークスペースには同じ名前のプロジェクトがあります"),
  ).toBeVisible();
  await expect(title).toHaveValue(before);
  const renamed = unique(info, "新しい名前");
  await title.fill(renamed);
  await title.press("Enter");
  await expect(page.getByRole("heading", { name: `${prefix}-P1 ${renamed}` })).toBeVisible();

  // ステータスとオーナー。
  await props.getByRole("combobox", { name: "ステータス" }).selectOption("InProgress");
  await expect(page.locator(".status-pill")).toContainText("InProgress");
  await props.getByRole("combobox", { name: "オーナー" }).selectOption("hanako");

  // 日付: 終了日が開始日より前なら断って元に戻す。同じ日は認める。空にすることは常にできる。
  const start = props.getByLabel("開始日", { exact: true });
  const end = props.getByLabel("終了日", { exact: true });
  await start.fill("2026-11-20");
  await start.blur();
  await end.fill("2026-11-19");
  await end.blur();
  await expect(page.getByText("終了日は開始日以降にしてください")).toBeVisible();
  await expect(end).toHaveValue("");
  await end.fill("2026-11-20");
  await end.blur();
  await start.fill("");
  await start.blur();

  // 読み直しても残り、履歴に「作成」と項目の変更が出る。
  await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/overview`));
  await expect(props.getByRole("combobox", { name: "ステータス" })).toHaveValue("InProgress");
  await expect(props.getByRole("combobox", { name: "オーナー" })).toHaveValue("hanako");
  await expect(start).toHaveValue("");
  await expect(end).toHaveValue("2026-11-20");
  const history = page.getByRole("region", { name: "履歴", exact: true });
  await expect(history).toContainText("作成");
  await expect(history).toContainText("ステータスを InProgress に変更");
});

test("edits the description and posts a comment on the overview", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "概要の説明"), prefix);
  await newProject(page, info, prefix);
  const description = page.getByRole("region", { name: "説明", exact: true });
  await description.getByRole("button", { name: "説明を追加" }).click();
  await description.getByRole("textbox", { name: "説明" }).fill("プロジェクトの説明");
  await description.getByRole("textbox", { name: "説明" }).press("Control+Enter");
  await expect(description).toContainText("プロジェクトの説明");
  const comments = page.getByRole("region", { name: "コメント", exact: true });
  const box = comments.getByPlaceholder(
    "コメントを書く。@ログイン名 でメンション。Ctrl+Enter で送信",
  );
  await box.fill("プロジェクトのコメント");
  await box.press("Control+Enter");
  await expect(comments.getByRole("listitem")).toHaveCount(1);
  // ウォッチ中の人はいないので、ウォッチのボタンはない。
  await expect(page.getByRole("button", { name: /^ウォッチ/ })).toHaveCount(0);
  await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/overview`));
  await expect(description).toContainText("プロジェクトの説明");
  await expect(comments).toContainText("プロジェクトのコメント");
});

test("adds and removes repositories", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "リポジトリ"), prefix);
  await newProject(page, info, prefix);
  const repos = page.getByRole("region", { name: "リポジトリと PR", exact: true });
  const input = repos.getByRole("textbox", { name: "リポジトリ" });
  const list = repos.getByRole("list", { name: "リポジトリ" });
  await expect(repos.getByText("リポジトリはまだありません")).toBeVisible();

  // 形が違うと、入力欄の下に断りが出て、欄は開いたまま。前後の空白は除いて追加する。
  await repos.getByRole("button", { name: "リポジトリを追加" }).click();
  await input.fill("web");
  await input.press("Enter");
  await expect(repos.getByText("リポジトリは owner/name の形式で入力してください")).toBeVisible();
  await input.fill(" example-org/web ");
  await input.press("Enter");
  await expect(list.getByText("example-org/web")).toBeVisible();
  await expect(input).toBeHidden();

  // 大文字と小文字を区別せずに、同じものは断る。「キャンセル」で欄を閉じる。
  await repos.getByRole("button", { name: "リポジトリを追加" }).click();
  await input.fill("Example-Org/WEB");
  await repos.getByRole("button", { name: "追加" }).click();
  await expect(repos.getByText("もう追加されています")).toBeVisible();
  await repos.getByRole("button", { name: "キャンセル" }).click();
  await expect(input).toBeHidden();

  // 読み直しても残る。「×」は確認なしで外し、トーストを出す。
  await page.reload();
  await expect(list.getByText("example-org/web")).toBeVisible();
  await repos.getByRole("button", { name: "example-org/web を削除" }).click();
  await expect(page.getByText("example-org/web を削除しました")).toBeVisible();
  await expect(repos.getByText("リポジトリはまだありません")).toBeVisible();
});
```

`web/e2e/notes-readonly.spec.ts`(新規。閲覧のみの Store のサンプル HH-2・HH-P1 は読むだけ):

```ts
import { expect, test } from "@playwright/test";
import { tokenUrl } from "./helpers";

// 閲覧のみの Store では、書く操作を無効にして「閲覧のみの Store です」を添える(02「詳細パネル」の冒頭)。

const note = "閲覧のみの Store です";

test("a read-only store's panel disables every write control of the notes", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/i/HH-2"));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(panel.getByRole("button", { name: "説明を追加" })).toBeDisabled();
  await expect(
    panel
      .getByRole("region", { name: "コメント", exact: true })
      .getByPlaceholder("コメントを書く。@ログイン名 でメンション。Ctrl+Enter で送信"),
  ).toBeDisabled();
  const worklog = panel.getByRole("region", { name: "作業時間", exact: true });
  await expect(worklog.getByLabel("時間", { exact: true })).toBeDisabled();
  await expect(worklog.getByRole("button", { name: "記録" })).toBeDisabled();
  await expect(worklog.getByRole("button", { name: "記録" })).toHaveAttribute("title", note);
  const relations = panel.getByRole("region", { name: "関連", exact: true });
  await expect(relations.getByRole("combobox", { name: "関連の種類" })).toBeDisabled();
  await expect(relations.getByRole("button", { name: "追加" })).toBeDisabled();
  await expect(panel.getByRole("button", { name: /^ウォッチ/ })).toBeDisabled();
});

test("a read-only store's overview disables the property fields and the repositories", async ({
  page,
}) => {
  await page.goto(tokenUrl("/s/readonly/p/HH-P1/overview"));
  const props = page.getByRole("region", { name: "プロパティ", exact: true });
  await expect(props.getByRole("textbox", { name: "タイトル" })).toBeDisabled();
  await expect(props.getByRole("combobox", { name: "ステータス" })).toBeDisabled();
  await expect(props.getByRole("combobox", { name: "オーナー" })).toBeDisabled();
  await expect(props.getByLabel("開始日", { exact: true })).toBeDisabled();
  await expect(props.getByLabel("終了日", { exact: true })).toBeDisabled();
  const add = page
    .getByRole("region", { name: "リポジトリと PR", exact: true })
    .getByRole("button", { name: "リポジトリを追加" });
  await expect(add).toBeDisabled();
  await expect(add).toHaveAttribute("title", note);
  await expect(page.getByRole("region", { name: "説明", exact: true }).getByRole("button", { name: "説明を追加" })).toBeDisabled();
});
```

Run: `pnpm -C web fix && pnpm -C web lint`
Expected: PASS(長い行は Biome が整える)

Run: `cd web && pnpm exec playwright test project-overview notes-readonly`
Expected: PASS(3 幅)

```bash
git add web/e2e
git commit -m "test(e2e): プロジェクトの概要の編集・リポジトリ・閲覧のみの無効の通しのテスト(A4a)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 8: 全部を確かめる**

古いバイナリを使い回さないように、`make e2e` が画面とバイナリを作り直す。

Run: `make check`
Expected: PASS(`tidy-check`・`lint`・`fmt-check`・`test`・`web-check`。`web-check` は `pnpm gen` のあと `generated.ts` に差分がないことも確かめる)

Run: `make e2e`
Expected: すべて PASS(3 幅 × 全ファイル。この PR で足した spec は `description`・`comments`・`worklog-relations`・`create-description`・`project-overview`・`notes-readonly`)。落ちたものは、画面の振る舞いが仕様どおりかを先に確かめる。まれに落ちるテストは、3 幅の並列と共有のサーバーが原因のことがある(名前の重なり、保存の応答を待たない移動)。`unique`・`saved` で直し、`retries` で隠さない。

結果(コマンド、通ったテストの数、落ちて直したもの)は、PR の本文の「確認の結果」にそのまま書く。

- [ ] **Step 9: 最終レビュー**

superpowers:requesting-code-review(または `code-review high`)で、`develop` との差分をレビューする。指摘は、直すものと先送りするもの(理由つき)に分ける。直すものは、タスクごとのコミットとして足す。

- [ ] **Step 10: 計画に「実装で計画から変えたこと」と申し送りを足す**

この計画の末尾に、次の 2 節を足す。実装した結果で書く(この計画を書いた時点では、まだ中身がない)。

- 「実装で計画から変えたこと」: テスト・補助関数の名前を既存に合わせたこと、タスクの分け方を変えたこと、仕様で決めていなくて実装で決めたこと(「骨組みへの指摘」と「仕様で決まっていなくて決めたこと」に書いた、概要の編集の断りの出し方、リポジトリの並び、アクティビティの {タイトル} の旧新、`project.yaml` の `repos`、e2e のロケーターの実際の名前)。
- 「A4b・C4 への申し送り」: A4b は、プロジェクトのメニューの「名前を変更…」ダイアログと設定の「名前」の区画が `projectEditor.rename`(`rename-project`)をそのまま使えること、上限の 20 ファイルにいる `gitstore`・`app/workspaces`・`apptest`、`detail.ts` などの行数。C4 は、書いた行(`desc`・`comment`・`worklog`・`add`・`remove`・`repos`)と、通知を作る材料(メンション・ウォッチ・オーナー・「上書き」)がどの行にあるか、`textmerge.Merge` の `conflict` が「上書き」の通知の材料であること。

```bash
git add docs/superpowers/plans/2026-11-13-hedgehug-track-a4a.md
git commit -m "docs: A4 前半の計画に、実装で変えたことと申し送りを足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 11: PR を 1 本出す**

向き先は `develop`、ブランチは `feat/track-a4a`。force push は拒否されるので、載せ直すときは別名のブランチで出す。

```bash
git push -u origin feat/track-a4a
gh pr create --base develop --head feat/track-a4a \
  --title "feat: トラック A4 前半(詳細パネルの残り)" \
  --body "$(cat <<'EOF'
## できること

- アイテムとプロジェクトの説明を、Markdown とメンションの表示つきで編集できる。入力中は確認を出し、保存のときは編集を始めたときの説明を共通の祖先に行ごとに合わせ、同じ行なら両方を残してトーストで知らせる。
- 作成ダイアログに説明の欄を足し、テンプレートの説明を当てはめる。テンプレートの子の説明を保存し、アイテムから作るテンプレートへ説明を写す。
- 詳細パネルに、関連(追加・解除・候補の検査・逆向きの表記・ブロック中)、作業時間(記録・検査・削除・合計)、コメント(投稿・編集・削除・メンションのボタン)、ウォッチ、履歴(40 件と「すべて表示」)を足す。
- プロジェクトの概要で、タイトル・ステータス・オーナー・開始日・終了日を直接編集し、リポジトリを追加・削除できる。概要にも説明・コメント・履歴を出し、Epic が 0 件のときの文言と注記を足した。

## 決めたこと(A4a-1〜26)

- A4a-1: 説明は `description.md` に置く(空はファイルを置かない)。保存の口に `Description`・`Describe` を足し、履歴の `desc` の行は app が別に足す。
- A4a-2: 行マージを domain の `textmerge` に移し、両方を残したかを返す `Merge` を足した。同期と保存で同じ規則。
- A4a-3: 説明の保存は、画面が編集を始めたときの説明と入力を送り、サーバーが保存の中で合わせる。同じなら何も保存せず、両方を残したら `conflict: true` で知らせる。
- A4a-4: 作成の入力に説明を足し、テンプレートの子の説明も同じ保存で書く。Step は説明を持たない。
- A4a-5: アイテムからテンプレートを作るときに、アイテムと子の説明を写す。
- A4a-6: 作成ダイアログの説明は、自動では空かテンプレートと同じときだけ入れ、セレクトでは置き換える。「なし」に戻すときはテンプレート由来なら空にする。
- A4a-7: コメントは `comment` の行で持つ(`f` は内部 ID、削除は `to: null`)。投稿者と時刻は最初の行、2 行以上なら「(編集済み)」。
- A4a-8: 作業時間は `worklog` の行で持つ。日付は保存したマシンの今日。
- A4a-9: 作業時間の検査は domain に置き、同じ規則を core にも置く。時間は 100 分の 1 時間の整数で持つ。
- A4a-10: 関連は、足した側のログの `relations` の `add`・`remove` で持つ。逆向きの表記は読むときに作る。
- A4a-11: 関連の一覧・ブロック中・候補・循環の検査は、Store のすべてのアイテムを読んで作る。
- A4a-12: 関連の候補は API が返す(最大 20 件)。同じ種類の関連と循環は、無効にして理由を添える。
- A4a-13: 「ブロック中」は、アイテムの応答の `blocked` で返す。
- A4a-14: ウォッチは `watchers` に自分のログイン名を `add`・`remove` する。すでにその状態なら何も保存しない。
- A4a-15: 履歴は、サーバーが行の文まで作って新しい順に返し、画面が 40 件で切る。知らない種類の行は出さない。
- A4a-16: ワークスペースのログに同じ `op` の削除の行があるアイテムの行は、履歴に出さない。
- A4a-17: 履歴の操作した人は、自動化・GitHub・同期・メンバーの表示名・「不明なユーザー」。値の名前は、いまの設定で引く。
- A4a-18: プロジェクトの概要の編集を `app/projects` に足した。
- A4a-19: リポジトリは `repos` の `add`・`remove` で持つ。形は 07 の `owner/name`、重複は大文字小文字を区別しない。
- A4a-20: Markdown は core が字句に分け、ui が Svelte の要素で描く(`{@html}` は使わない)。リンクは http・https だけ。
- A4a-21: 書きかけは、自動の同期と画面の反映を待たせる。
- A4a-22: 入力の破棄の確認は、core の `drafts` が答える。
- A4a-23: メンションのボタンは、あなた以外のメンバーのログイン名を並べる。
- A4a-24: 読み取りの API は区画ごとに分けた。
- A4a-25: 概要でのプロジェクトのタイトルの編集は A4a に入れた。
- A4a-26: 作業時間の時間は、API では文字列で受け取り、サーバーが検査する。

仕様で決まっていなくて実装で決めたこと: 概要の編集の断りは、項目はトースト、リポジトリは入力欄の下に出す。リポジトリの一覧は大文字小文字を区別しない辞書順。プロジェクトの変更のアクティビティの {タイトル} は変える前のタイトル。`project.yaml` に `repos` を書き出す。

## 回したもの

- A4b: 削除・移動・種類の変更・複製・メニューの残りの項目・上下移動・ツリーのキー操作・Step の「×」・プロジェクトの名前の変更ダイアログと削除、削除済みを件数・候補・一覧から除くこと
- C4: 通知と受信箱(メンション・ウォッチ・オーナー・「上書き」・担当者)、履歴の「同期で受信」
- F・A4b・A5・G: 履歴の「スプリントを変更」「リリースを変更」「復元」「{旧 ID} から移動」の文
- D3・D4: リポジトリの「PR を探せませんでした」、PR と Issue の区画の中身
- E: 関連の検索のキー(`blocks:` など)

## 確認の結果

(ここに、`make check` と `make e2e` の実行結果、通ったテストの数、落ちて直したものを、実行した結果のとおりに書く)

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

「確認の結果」の括弧の 1 行は、実行した結果で置き換えてから出す(置き換えずに出さない)。`gh pr create` が返す URL を、この計画の末尾の記録に足す。

---


---

## 各タスクで決めた細部(仕様に書かれていない点)

- **空白だけの説明は空として扱う**: 説明の保存(`base` と `text` の両方)と、作成の説明(`CreateInput.Description`、テンプレートの子の説明)で、空白と改行だけの説明は「説明なし」にして、`description.md` を置かない(あれば消す)。仕様は「空のときは「説明を追加」を出す」とだけ書いていて、空白だけの扱いは決めていない。
- **説明の大きさの上限は設けない**: 仕様に上限がない。
- **サーバーはテンプレートの説明をもとになるアイテムに入れ直さない**: 作成の `description` をそのまま保存する(A3b-8 と同じ考え方。当てはめは作成ダイアログが A4a-6 の規則で行う)。
- **アイテムからテンプレートを作るときに読む説明**: アイテム自身と、Epic の直下の子だけ。「子と Step を含める」がオフなら、アイテム自身だけ。プロジェクト全体のアイテムの説明は読まない(読む回数を増やさない)。
- **編集していない保存は何もしない**: `text` が `base` と同じなら、保存してある説明を返して何も保存しない(`textmerge.Merge` の「newer が base と同じなら older」)。別の人の保存があっても、自分が何も変えていないので上書きしない。
- **プロジェクトの説明のアクティビティ**: 骨組みの A4a-18 どおり「{タイトル} の説明を変更」。タイトルは保存の前に読む。

- Markdown: `1.` で始まる行は `<ol>`、`-` `*` は `<ul>`(同じ種類が続くあいだで 1 つの list。`\d+.` を番号として読む)。見出しは `#` の後に空白が要り、`####` 以上は段落。印は `[ ]` と小文字の `[x]` だけ。空行は読み飛ばす。閉じていないコードブロックは最後まで。URL の末尾の `.,;:!?)'*` はリンクに含めず、ホストのない `http://` はリンクにしない。メンションのログイン名は `[A-Za-z0-9_-]{1,39}`(`internal/domain/member` と同じ)で、表示は打ったとおりの綴り。見出しは区画の見出し(`h3`)の下になるよう `h4`〜`h6` で出す。
- 入力欄の中の Esc は、違う入力があるときだけ確認を出す(違わないときは、ふつうの Esc と同じにパネルを閉じる)。確認の「破棄」は入力欄を閉じるだけでパネルは閉じない。「キャンセル」は入力欄にフォーカスを戻す。確認は、その入力欄の分だけを聞く(`confirmLeave(key)`)。
- 戻る・進むの確認で「キャンセル」したときは、いまの画面のアドレスを `history.push` で積み直して留まる(履歴の先の項目は失われる)。`replace`(アイテムの入口の置き換え)は確認しない。
- 説明の区画は、読み終わるまで中身を出さない。保存が断られたら、エラーのトーストを出して入力欄と入力を残す。閲覧のみの Store では、説明の部分と「説明を追加」を押せなくし、「閲覧のみの Store です」を添える。
- 説明の「押す場所」は、内容の下に敷いた本物のボタン(読み上げの名前「説明を編集」)。内容の中のリンクは押せる。
- 作成ダイアログの説明の欄: ラベル「説明」(案内文なし)、位置は 02 の表の順(プロパティの次、一緒に作るものの前)。作成の入力の `description` は、空なら送らない。
- `applyToForm` は、いま当てはめているテンプレートを設定から引かず、フォームが持つ `templateDescription`(いま当てはめているテンプレートの説明)と比べる。`applyDefault` が先に「なし」にする動きは、そのため意味がなくなったので外した。

1. **コメントのアクティビティの文。** 仕様は「{ID} にコメント」「コメントを編集」「コメントを削除」。文字どおり、編集と削除には ID を付けない(`コメントを編集`・`コメントを削除`)。作業時間の「{ID} の作業時間を削除」と形が違うので、ID を付けるなら「{ID} のコメントを編集」にする(`internal/app/comments/edit.go`・`delete.go` の `Summary` の 1 行)。
2. **他人のコメントの編集と削除は、サーバーが `port.ErrNotFound`(404)で断る。** 仕様は「自分が書いたコメントにだけ、編集と削除のボタンを出す」(画面の規則)で、断りの文言がない。画面からは起きない操作なので、B2-12 と同じく、ないものとして扱う(ログイン名は大文字と小文字を区別しない)。ボタンの出し分けは画面の `isMine`。
3. **空のコメントは、サーバーが「コメントを入力してください」で断る。** 画面は空のときは送らない(仕様「空のときは何もしない」)ので、実際には届かない。投稿でも編集でも同じ文言。編集で本文が前と同じなら何も保存しない。本文の前後の空白は取る。
4. **コメントの削除の確認ダイアログは「危険な操作の見た目」にした。** 仕様は作業時間だけに明記し、コメントは「ボタンは「削除」である」。ほかの削除と合わせた。
5. **コメントの投稿ボタンの文言は「投稿」。** 仕様は「ボタンか Ctrl+Enter で投稿する」とだけ。入力欄の下に「投稿」を置く。
6. **作業時間の日付の表示は、01 の `formatDay`(今年なら M/D、それ以外は YYYY/MM/DD)。** 仕様は「{日付}」とだけ。行と削除の確認の本文で同じ表記。
7. **作業時間の時間の入力: `1.250` は 1.25 として受ける。** 仕様の「小数第 3 位以下があるとき」を「0 でない数字があるとき」と読み、末尾の 0 は数えない。符号・指数・カンマは数として扱わない(`1e2`・`-1`・`1,5` は「時間を入力してください」)。整数部が 10 桁以上は、100 分の 1 時間の整数が 2^53 を超えるので「時間を入力してください」で断る(仕様は上限なしだが届かない値)。
8. **作業時間の「0.25 時間刻みで増減」は「−」「+」のボタンと上下キーで行う。** `type="number"` の入力欄は値が数に変わり、文字列で検査する規則(A4a-26)と合わないため。読めない入力は 0 から数える。0 以下になるときは空にする。
9. **ウォッチのボタンの表示は「☆ ウォッチ」と「★ ウォッチ中」。** 仕様は「表示で、いまどちらか分かるように」とだけ。あなた(`me`)が決まらないときは無効にする(案内は出さない)。サーバーは `Identity.Me` のエラーをそのまま返す。
10. **コメントと作業時間の「メンバーにいない人」の判定は、サーバーが返す `byName` が「不明なユーザー」かどうか。** アバターは中身のない丸にする(01)。
11. **作業時間の応答の `hours` は文字列(`worklog.Format` の表記)。** 画面の合計は、この文字列を `parseHours` で 100 分の 1 時間にして足す。

- 関連の一覧の、同じ表記の中の並び: 相手の内部 ID の順(Task 13)。
- 関連の候補の検索語が空(空白だけを含む)のとき: 候補を返さない(Task 13)。02 は「入力すると候補を出す」とだけ書いている。
- 「すでに {表記} の関連があります」の {表記}: 利用者がセレクトで選んでいる表記(Task 13)。向きが逆の関連があるときも、選んでいる表記で出す。
- 候補にならない相手(自分・Step・ないアイテム)を指定した追加と、知らない表記: `port.ErrNotFound`(404)。画面からは送らないので文言は持たない(Task 13)。
- 解除で、持っていない関連を外そうとしたとき: エラーにせず、何も保存しない。`relates to` はどちらの側が持っていても外す(Task 13)。
- 関連の候補が 20 件(骨組みの A4a-12 のとおり)に達したとき、それ以上は出さない。20 件を超えることを画面は知らせない(Task 13・14)。
- 関連の追加・解除でトーストは出さない(02「関連」にトーストの記述がない)。断られたときだけエラーのトースト(Task 14)。
- 履歴の「親を変更」: 親かプロジェクトが変わったときだけ(番号だけの `place` の変更は出さない)(Task 16)。
- 履歴で、設定にないプロパティ(消したプロパティ)の `set` の行: 名前が分からないので出さない(Task 16)。
- 履歴で、実行者が `none` の行の「操作した人」: 「不明なユーザー」(Task 16)。
- 履歴の値の名前の引き方: 行を書いたときの定義ではなく、いまのワークスペースの設定(骨組みの A4a-17 のとおり。別のワークスペースへの移動が入る A4b で見直す)(Task 16)。
- プロジェクトの履歴の値の書き方: ステータスは名前のまま(「InProgress」など)、オーナーは人の名前、日付は 01 の書き方(今年なら M/D)、空は「なし」(Task 16)。
- 履歴の時刻の表示: 相対時間と、ポインターを置いたときの日時(Task 17)。

1. **概要の断りの出し方**: 02「概要の編集」は「表示して断る」「元に戻す」とだけ書く。タイトル・日付・ステータス・オーナーは、エラーのトーストで出して入力欄を保存済みの値に戻す(詳細パネルのタイトルと同じ)。リポジトリの追加は、入力欄の下に断りを出し、トーストは出さない(02「リポジトリの追加と削除」は追加ではトーストを出さないとする)。
2. **保存のタイミング**: タイトルは、入力欄から外れたとき(Enter でもフォーカスを外して保存)。日付も入力欄から外れたとき。ステータスとオーナーは選んだとき。
3. **アクティビティの {タイトル}**: 変える前のタイトル(「骨組みへの指摘」2)。
4. **リポジトリの一覧の並び**: 大文字小文字を区別しない辞書順(仕様は並びを決めていない)。
5. **オーナーの選択肢**: 「なし」とメンバー。メンバーにいない人がオーナーのままなら、その人も「不明なユーザー」として選択肢に出す。メンバーでないログイン名を送ると 404。保存するのはメンバーのログイン名(大文字小文字を区別しない比べ方)。
6. **リポジトリの削除の行**: 保存してある綴りで書く(入力と大文字小文字が違っても外せる)。アクティビティには入力のとおりに書く。ないものを外しても何も保存せず、断らない。
7. **`project.yaml` に `repos` を書き出す**: 保存形式の設計書の `project.yaml` の項目には書かれていない。読む用の写しなので、`item.yaml` の `watchers` と同じ書き方(`repos: [..]`)で足した。
8. **リポジトリの追加の操作**: 「リポジトリを追加」で入力行(入力欄・「追加」・「キャンセル」)を開き、Enter か「追加」で追加する。断られたら開いたまま。仕様は「入力欄に入れる」としか書かない。
9. **アクティビティの日付**: 01「日付と時刻」の表記(今年は M/D、ほかの年は YYYY/MM/DD、空は「なし」)に合わせた。仕様の 02 は「日付が空のときは「なし」」としか書かない。

1. **関連の行末に相手の表示の ID(`# HH-14`)を書かない**: 保存形式の例は注釈を付けるが、相手は別のワークスペースにも居て、接頭辞が変わると別のワークスペースの `item.yaml` まで作り直す経路が要る(`parent`・`project` も、いまは書いていない)。ログが正で、画面は表示の ID を API から作るので、機能への影響はない。
2. **`by` は表示名ではなくログインのまま**、`at` は `change.TimeLayout`、`hours` は小数で書く(1.5・2。末尾の 0 は付かない)。
3. **`relations`・`comments`・`worklogs` が 0 件のときは `[]` を書く**(`fields: {}`・`systems: []` と同じ。キーを消さない)。
4. **Epic の進捗の分母は、Epic の下のすべての子孫のうち Step 以外**(深さを問わない。04 の「Epic の下の、Step を除くすべてのアイテム」)。今の階層(Epic → Story・Task・Bug → Step)では直下の子と同じになる。
5. **進捗の棒の「ポインターを置く」は、棒に `title` を付けて出す**(Avatar と同じ、ブラウザの標準の表示)。「N%」の文字の上には出ない(仕様は棒の上)。
6. **Epic の行は狭い幅で折り返す**(タイトルが先に縮み、期限・進捗・ステータス・担当者が次の行に回る)。ツリーの行は折り返さないが、概要の Epic の行はもともと幅が狭いカラムにあるため。
7. **Epic の行のステータスは `projectScreen.setStatus`**(ツリーの行と同じ経路。変えたあとプロジェクトを読み直し、開いている詳細パネルも読み直す)。

---

## 実装で計画から変えたこと

実装の記録(`.superpowers/sdd/` の台帳)から、計画の文と違う結果になったものを挙げる。

**上の「各タスクで決めた細部」の文言を、実装で置き換えたもの**

- コメントの投稿ボタンは「投稿」でなく「送信」(モック版 19 に合わせた)。
- ウォッチのボタンは「☆ ウォッチ」「★ ウォッチ中」でなく、星なしの「ウォッチ」「ウォッチ中」。押した状態は `aria-pressed` で示す。
- 作業時間の入力は、「−」「+」のボタンでなく `<input type="number" step="0.25" min="0">`。0.25 時間刻みの増減はブラウザの矢印とキーに任せ、検査は入力の文字列から行う(A4a-26)。
- 作業時間の区画の名前は、入力欄が label「時間」「メモ」、行の削除が IconButton「作業時間を削除」(行は `listitem` でなく `div`)。コメント 1 件は `article`。

**計画から変えたこと**

- `data-mention`: Markdown の描画が `class` だけを出していたので、小文字のログイン名を持つ `data-mention` を足し、テストも足した(通しのテストが使う)。
- ドラフトの確認: 計画の引数なしの `confirmLeave` を、書きかけごとのキー付き(`confirmLeave(key)`)にした。入力欄の Esc は `stopPropagation` で、パネルを閉じる Esc と重ならない。
- 説明の区画の状態は、対象が変わったときに `reset()` する(概要と詳細パネルが同じ部品を使い回すため)。古い対象の保存の結果は、`reset()` のあと無視する。
- 履歴: 履歴の `desc`・プロパティの値の追加・削除の行も出す(仕様 02 の履歴に含まれるため)。行を `at` の並びで切るときは、同期のマージで順序が入れ替わっても、前の値に合うように並べてから切る(計画の `log[:i]` では、時計のずれで古い値が出た)。
- 他人のコメントの編集・削除は、サーバーが `ErrNotFound` で断る(B2-12 と同じ)。
- プロジェクトのタイトルは、ほかのプロジェクトのタイトルの正規化と同じに整えて Summary に入れる(複数行の Summary が断られないように)。
- `Input.svelte` の `bind:value` を、値を親が変える欄(接頭辞の大文字化など)で壊さない形に直した(作業時間の入力を作るときに見つけた既存画面の回帰)。
- 概要の履歴の読み直しは、`[p.title, p.status, p.owner, p.start, p.end]` と説明の保存の回数だけで行う。リポジトリの追加・削除は、仕様 02 でプロジェクトの履歴に残らないので含めない。
- 通しのテスト: 計画の名前の表は、部品ができる前の推測だった。実際の名前に合わせた(関連の種類のセレクトは「関連の表記」、相手の検索は textbox「相手」など)。関連・履歴は A4a の途中で `relations.spec.ts`・`history.spec.ts` として足してあるので、最後のタスクでは作り直さない。作業時間は `worklog.spec.ts`、説明の Markdown・同じ行の保存は `description-merge.spec.ts` に分けた(計画の `worklog-relations.spec.ts` の名前は使わない)。
- 閲覧のみの Store のサンプル(HH-2・HH-P1)にはコメントも作業時間の記録もないため、既存の行の「編集」「削除」の無効は通しのテストで確かめていない(入力欄と「送信」「記録」「関連」「ウォッチ」「リポジトリを追加」などの無効は確かめた)。

**仮の文言(仕様にない。決まったら差し替える)**

- コメントを保存できなかったとき(通信の失敗など): 「コメントを保存できませんでした」(`commentFailedMessage`)。
- 作業時間を保存できなかったとき: 「作業時間を保存できませんでした」(`worklogFailedMessage`)。

## A4b・C4 への申し送り

**A4b**

- プロジェクトのメニューの「名前を変更…」ダイアログと設定の「名前」の区画は、`projectEditor.rename`(`rename-project`)をそのまま使える(空・重複の断りも同じ)。
- 上限の 20 ファイル(テストを除く)にいるのは `internal/infra/gitstore`・`internal/app/workspaces`・`internal/app/apptest` の 3 つ。`internal/app/items` は 14、`internal/app/projects` は 11。ここに足すときは既存のファイルに足すか、パッケージを分ける。
- `web/app/src/wire.ts` と `web/packages/core/src/index.ts` は、どちらもちょうど 300 行(TS の上限)。A4b の配線と export は、別のモジュール(`notesIndex.ts` のように束ねたもの)に置くか、分けてから足す。`detail.ts` は 269 行。
- 画面の全幅の 2 列の詳細(左に本文、右にプロパティ)は、SidePanel にはない。概要の 2 列(`ProjectOverview`)は幅の container query で作ってある。
- 削除済みを件数・候補・一覧から除くこと(PR の「回したもの」のとおり)。
- 履歴の「スプリントを変更」「リリースを変更」「復元」「{旧 ID} から移動」の文は、元になる行(A4b・F・A5・G)が入ったときに足す。知らない種類の行は、いまは出さない。
- 先送りした軽微な指摘のうち、A4b で触るもの: 履歴の `each` が index キー、`detail.ts` の `requestClose` が `this.close()` を使うこと(切り離して渡すと壊れる)、作成ダイアログの古いコメント(`createDialog.ts` の「テンプレートを先に「なし」にする」)。

**C4**

- 書いた行: アイテムのログの `desc`・`comment`・`worklog`・`add`(関連)・`remove`(関連)、プロジェクトのログの `repos`。コメントは `comment` の行の `f`(内部 ID)と `to: null`(削除)、作業時間は `worklog` の行。
- 通知の材料: メンションはコメントと説明の本文(`@ログイン名`、メンバーに一致するものだけ)、ウォッチは `item.yaml` の `watchers`(`add`・`remove` の行)、オーナーはプロジェクトの `owner`、担当者はアイテムの `assignee`。
- 「上書き」の通知の材料は `textmerge.Merge` の `conflict`(同じ行が同時に編集され、両方を残したとき)。説明の保存の応答の `conflict: true` が同じ印。
- 履歴の「同期で受信」は C4 が足す(A4a の履歴は、同期で入った行の操作した人を A4a-17 の表示の規則で出す)。

**A4b・C4 共通**

- Store が大きいと、`sync.spec.ts` の 3 秒・5 秒の窓が足りなくなる。アイテムを読むたびに Store 全体を読む作り(A4a-11)のため、e2e の Store にワークスペースとアイテムが増えるほど 1 回の操作が遅くなる(新しい通しを足す前の 180 件は workers=2 で 3.5 分で全部通り、足したあとは 6 分余りで、後ろの desktop で時間の窓があるテストが落ちることがある)。通しの数を増やす前に、読み方を軽くするか窓を見直す。
- 概要で項目を続けて編集すると、直前の保存の読み直しが終わる前の値と比べるので、「変わっていない」と判定されて保存が送られないことがある(Task 19 の「rapid double edits ordering」)。人の操作の速さでは起きにくい。通しのテストは、履歴に行が出るのを待ってから次を編集している。
