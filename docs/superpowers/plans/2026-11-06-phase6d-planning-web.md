# フェーズ 6d: 計画の画面 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** バックログ、スプリント、リリース、タスク詳細の計画の欄、一括編集、WIP 上限、テンプレートをブラウザで動かす（spec の 6d の完了条件。E2E で確かめる）。

**Architecture:** 読み取りと文言は `web/core/planning.ts` に集める。画面は `web/app/planning/` に置き、ワークスペースのタブから開く。Go の変更は、親をまたぐ並べ替えの API と `kanban.wip` だけ。

**Tech Stack:** Go（huma）、Svelte 5、vitest、Playwright。

**Spec:** `docs/superpowers/specs/2026-11-05-phase6-planning-design.md` の「画面」の 6d。**キャンバス:** Hedgehug GUI デザインの Backlog・SprintBoard・SprintDialogs・Release・Kanban・Table・TaskDetail・CreateTask・SettingsSchema・ProjectSettings・CommandPalette・Dialogs。

**PR:** この plan と引き継ぎは実装の PR に入れる。

## Global Constraints

- `make` は `MAKEFLAGS=-j10`。E2E は `make e2e`。`make web e2e-bin` を並べない
- Go を変えたら `go generate ./internal/app/server` と `cd web && pnpm gen` を走らせる
- 既存のテストの期待値は、この plan が明示したものの他は変えない

## 確かめたこと

| 項目 | 結果 |
|---|---|
| スプリント・リリースの変更の通知 | 新しいイベントは要らない。`watch.go` はタスクも schema も変えないコミットで `tasks.changed` を出し、`core/stores.ts` はそれで読み直す |
| カンバンの遷移の表示 | 足りている。`decideDrop` が拒否の理由を返し、ドラッグ中の列に出す（4e）。足すものはない |
| `status:todo,doing` | カテゴリ名を受け付ける。`sprint:none status:todo,doing sort:rank` はそのまま使える |
| 6a の引き継ぎの `counts.ts` | 実際の判定は `web/core/workspace.ts` の `removeBlocker`。引き継ぎを直す |

## この回で決めたこと

| # | 項目 | 決定 | 理由 |
|---|---|---|---|
| 1 | 親をまたぐ並べ替え | `POST /api/tasks/{id}/rank`。本文 `order` はドロップ後の並び全体の ULID。前後の rank の間に置き、同じ rank が続いて間がなければ、後ろの同じ rank のタスクも振り直す。全部で1コミット | 親ごとに rank を振るので、親をまたぐと同じ rank が並ぶ。隣の2件だけでは置けない |
| 2 | バックログの項目 | プロジェクト（`role` が container）を除く | スプリントに入れる作業ではない |
| 3 | スプリントのボード | active が1つなら `sprint:current`、複数ならキャンバスの切り替えで `sprint:"<名前>"` | spec とキャンバスの両方を満たす |
| 4 | タブが隠れているときの URL | 0 件でも `/backlog` などを開ける。空の表示と作成ボタンを出す | 最初の1件を作る導線が1つ増えるだけで、壊れない |
| 5 | WIP の保存 | `kanban_wip` を保存と変更の API、`hh view save --wip <列>=<上限>` に足す。変更は全体の置き換え。ワークスペースとプロジェクトのカンバンでは、列やレーンの設定と同じくページの中の値にする | 保存先はビューだけ（spec） |
| 6 | 既定のテンプレートの編集 | スキーマの種類の表に列を足して見せる。編集は scoped の `template_defaults` だけ | GUI は `schema.yaml` の種類を編集しない（今もない）。`kinds[].template` はファイルの値 |
| 7 | テーブルの選択 | `TableView` の中の状態にする。`selection.svelte.ts` は単一の選択で、作成ダイアログが読むので分ける | 意味が違う |
| 8 | 一括更新のダイアログ | `BulkDialog` を1つ。テーブルは `ulids`、パレットは `query` を渡す。`dry_run` の結果の件数と違反を出してから実行する | spec の「件数を出して実行」 |
| 9 | 作業時間の比較 | `estimate` は既定のスキーマの名前で固定（6c と同じ）。`2h / 見積 6h` と出す | キャンバス |
| 10 | リリースの詳細 | 選択はページの中の値。URL に持たない | タブの中の切り替えで、リンクの要件が spec にない |

## spec になく、キャンバスにだけあるもの

次は作らない。spec の 6d の行に入っていない。

- 終了ダイアログの「振り返りのメモ」と「1 件ずつ選ぶ」（API の `to` は1つ）
- 開始ダイアログとバックログの平均ベロシティのバー
- 作成ダイアログのテンプレートの子タスクのチェックボックス（API は部分展開できない）。子の件数は出す
- テンプレートの「既存のタスクから作る」
- スプリントボードの「途中で追加」とバーンダウン（6e）

CLI の表記はキャンバスの `--carry` でなく、実物の `--to` にする。

## File Structure

| ファイル | 役割 |
|---|---|
| `internal/app/usecase/tasks/rank.go`（新規） | 親をまたぐ並べ替え |
| `internal/app/server/tasks_write.go` | `POST /api/tasks/{id}/rank` |
| `internal/app/storage/view/view.go`、`usecase/views/views.go`、`server/views.go`、`cli/view.go` | `kanban.wip` |
| `web/core/planning.ts`（新規） | スプリントとリリースの読み取り、残り日数、文言、バックログの並び |
| `web/core/stores.ts`、`web/core/workspace.ts` | `sprints`・`releases` の件数、削除の理由 |
| `web/core/cli.ts`、`web/core/views.ts`、`web/core/scoped.ts` | CLI の表記、`wip`、テンプレートの下書き |
| `web/app/router.ts`、`web/app/workspace/*` | タブ、「計画」の節 |
| `web/app/planning/*`（新規） | バックログ、スプリント、リリースの画面とダイアログ |
| `web/app/detail/PlanFields.svelte`・`Worklogs.svelte`（新規） | タスク詳細 |
| `web/app/kanban/*` | WIP 上限、必須の判定 |
| `web/app/table/*`、`web/app/bulk/BulkDialog.svelte`（新規）、`web/app/palette/*` | 一括更新 |
| `web/app/settings/SettingsTemplates.svelte`（新規）、`web/app/settings/SettingsSchema.svelte`、`web/app/project/ProjectSettings.svelte`、`web/app/create/CreateTask.svelte` | テンプレート |
| `web/e2e/planning.spec.ts`（新規） | 通しの確認 |

## Tasks

各 Task はテストを先に書き、`go test`・`pnpm check`・`pnpm test` を通してコミットする。

- [x] Task 1: `POST /api/tasks/{id}/rank`。テスト: 間に置く、同じ rank の並びを振り直す、1コミット、別のワークスペースと閲覧のみを拒否する、`order` に本人がなければ拒否する
- [x] Task 2: `kanban.wip`。テスト: 読み書き、1未満の拒否、API の置き換えと外す操作、`hh view save --wip`
- [x] Task 3: `web/core`（`planning.ts`、件数、削除の理由、`requiredGap`、CLI の表記、`wip`）
- [x] Task 4: タブ、ルーター、ワークスペースの「計画」の節、作成と編集のダイアログ、パレットの「スプリントを作る」「リリースを作る」
- [x] Task 5: バックログ（ドラッグでスプリントに入れる、並べ替え）
- [x] Task 6: スプリントのタブ（一覧、ボード、開始、終了のダイアログ）
- [x] Task 7: リリースのタブ
- [x] Task 8: タスク詳細（スプリント、リリース、作業時間）
- [x] Task 9: 一括更新（テーブル、パレット）
- [x] Task 10: カンバンの WIP 上限
- [x] Task 11: テンプレート（作成ダイアログ、設定、プロジェクトの設定）
- [x] Task 12: E2E と手の確認、引き継ぎ
