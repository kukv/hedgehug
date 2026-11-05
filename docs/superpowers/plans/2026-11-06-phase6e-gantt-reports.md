# フェーズ 6e: ガントとグラフ Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** `layout: gantt`、ガントとロードマップ、レポートの画面、スプリントボードのバーンダウンをブラウザで動かし、フェーズ6の CLI の操作が GUI でもできるようにする（spec の 6e の完了条件。E2E で確かめる）。

**Architecture:** 日付の計算、親の集計、警告の判定は `web/core/gantt.ts`、レポートの値の変換は `web/core/reports.ts` に純粋関数で置く。画面は `web/app/gantt/` と `web/app/reports/`。Go の変更は保存済みビューの `gantt:` の節だけで、レポートの API は 6c のまま使う。

**Tech Stack:** Go（huma）、Svelte 5、SVG、uPlot、vitest、Playwright。

**Spec:** `docs/superpowers/specs/2026-11-05-phase6-planning-design.md` の「画面」の 6e。**キャンバス:** Hedgehug GUI デザインの Gantt・Reports・SprintBoard。

**PR:** この plan と引き継ぎは実装の PR に入れる。

## Global Constraints

- `make` は `MAKEFLAGS=-j10`。E2E は `make e2e`。`make web e2e-bin` を並べない
- Go を変えたら `go generate ./internal/app/server` と `cd web && pnpm gen` を走らせる
- 既存のテストの期待値は、この plan が明示したものの他は変えない
- 依存は完全なバージョンで固定する。`minimumReleaseAge` は 7 日

## 確かめたこと

| 項目 | 結果 |
|---|---|
| 一覧に `blocked_by`・`sprint`・`props` があるか | ある。ガントは `TaskView` と `watchTree` だけで描ける |
| `/api/reports/*` の型 | 6c で足りる。`workspace` はワークスペースの名前で渡す。計画中のスプリントのバーンダウンは `report.not_started` になる |
| バーンダウンの日 | 今日までしか返らない。理想線と x 軸はスプリントの `start`・`end` から画面で作る |
| ライセンス | `vite.config.ts` のプラグインが束ねた依存の文面を書き出す。uPlot の文面が入ることを `make licenses` で見る |
| `hh doctor` | `view.Problems` と `MissingColumns` を読む。ガントの検査も同じ経路に足す |

## この回で決めたこと

| # | 項目 | 決定 | 理由 |
|---|---|---|---|
| 1 | プロジェクトのガント | プロジェクトにもタブを置く。出す条件はワークスペースと同じ `hasPlans`。レポートはワークスペースのタブだけ | キャンバスのガントはプロジェクトの画面にある。レポートの API はワークスペース単位 |
| 2 | 全 Store のスコープの帯と縦線 | 行のワークスペースが1つのときだけ描く。複数なら描かず、その旨を出す | スプリントはワークスペースに属し、重ねると読めない |
| 3 | 日付のないタスク | スプリントがあればその期間を点線で描く。どちらもなければバーなし | 親設計のバーは日付が前提。キャンバスの凡例に従う |
| 4 | ドラッグ | 中央で移動、両端で伸縮。キーボードは ←→ で動かし Enter で確定、Esc で戻す。どれも確定で1コミット | spec の「ドロップで1コミット」。ドラッグだけの操作にしない |
| 5 | `sort:`・`group:` | ツリーと同じく保存で拒否する | 左はツリーで rank 順。`view.tree_order` を共用する |
| 6 | レポートの条件 | バーンダウン: スプリント、`--by`。ベロシティ: 件数。累積フロー図: 期間、`--by`。サイクルタイム: 期間。見積もり: スプリント。各節に同じ意味の `hh report` の行を出す | 「CLI の操作が GUI でもできる」 |
| 7 | ボードのバーンダウン | 切り替えで選んでいるスプリントを描く。残り points と途中で追加も同じ応答から出す | 6d の切り替えに合わせる |
| 8 | グラフの道具 | バーンダウン、ベロシティ、累積フロー図は uPlot。サイクルタイムは数値のカード、見積もりと実績は行の HTML | 時系列でない2つは uPlot の出番がない。キャンバスも同じ形。spec の文を直す |
| 9 | 色 | uPlot には `tokens.css` の変数を `getComputedStyle` で読んで渡す | canvas は CSS 変数を解決しない。ダークテーマはまだない |
| 10 | uPlot の読み込み | 動的 import | jsdom に `matchMedia` がなく、初期の bundle も増やさない |
| 11 | キャンバスにあり spec にない表示 | ガントの目盛り（日・週・月）、バーのプロパティの選択、警告の文の一覧は作る | 色だけの警告は読めない。プロパティの選択は `gantt:` の GUI の操作そのもの。目盛りがないとロードマップが画面に収まらない |

## spec になく、キャンバスにだけあるもの

次は作らない。API に値がなく、spec の 6e の行にも入っていない。

- ベロシティの「確定（開始時）」のバーと達成率
- サイクルタイムの分布のヒストグラム
- レポートの「対象: プロジェクト」の絞り込み
- バーンダウンの「ダウン／アップ」の切り替え

## File Structure

| ファイル | 役割 |
|---|---|
| `internal/app/storage/view/view.go` | `Gantt` の節、値の検査、スキーマとの照合 |
| `internal/app/usecase/views/views.go`、`usecase/maintenance/doctor.go` | `gantt` の変更、`tree_order`、`hh doctor` |
| `internal/app/server/views.go`、`internal/app/cli/view.go` | `gantt_*` の項目、`--gantt-start`・`--gantt-end`・`--roadmap` |
| `web/core/views.ts`、`web/core/cli.ts` | `GanttSettings`、下書き、CLI の表記 |
| `web/core/gantt.ts`（新規） | 日付、行、集計、警告、矢印、ドラッグの結果 |
| `web/core/reports.ts`（新規） | レポートの応答からグラフの系列と文言を作る |
| `web/app/gantt/*`（新規） | ガントの画面 |
| `web/app/reports/*`（新規） | uPlot の部品、レポートの画面、ボードのバーンダウン |
| `web/app/router.ts`、`web/app/workspace/*`、`web/app/project/*`、`web/app/views/*`、`web/app/list/*` | タブ、レイアウトの分岐 |
| `web/e2e/gantt.spec.ts`、`web/e2e/reports.spec.ts`（新規） | 通しの確認 |

## Tasks

### Task 1: 保存済みビューの `gantt:`

- [x] `view_test.go` に、`gantt` の往復、未知のキーの保持、`layout: gantt` の受理、date 型でないプロパティの検出のテストを書き、落ちるのを見る
- [x] `Layouts` に `gantt`、`Gantt{Start, End, Roadmap}`、`GanttStart()`・`GanttEnd()`（既定 `start`・`due`）、`BadGanttProps(sc)` を足す
- [x] `views.go` に `SavedGantt`・`ViewChange` の項目を足し、`tree_order` を `gantt` にも効かせ、保存でスキーマと照合する（`view.bad_gantt_prop`）
- [x] `doctor.go` で `BadGanttProps` を `MissingColumns` と同じように警告する
- [x] `server/views.go` の enum と `gantt_start`・`gantt_end`・`gantt_roadmap`、`cli/view.go` のフラグを足す。ゴールデンテストと usecase のテストを足す
- [x] `go generate ./internal/app/server`、`pnpm gen`、`go test ./internal/app/...`、コミット

### Task 2: web の土台

- [x] uPlot を完全なバージョンで足す
- [x] `core/views.test.ts`・`core/cli.test.ts`・`app/router.test.ts`・`app/views/draft` のテストを足す
- [x] `Layout`、`GanttSettings`、`DEFAULT_GANTT`、`draftOf`・`saveBodyOf`・`sameDraft`、`cliViewSave`、`cliReport*`、`gantt`・`reports` のタブ、`/query/gantt`、`summaryOf`、`LAYOUTS`、`treeOrderProblem` を足す
- [x] `pnpm test`、`pnpm check`、コミット

### Task 3: `core/gantt.ts`

- [x] `core/gantt.test.ts` に次を表で書く: 日付の変換、開始か終了だけのタスク、終了が開始より前のタスク、スプリントの期間の点線、親の集計（孫まで）、親の期間からのはみ出し、ロードマップの絞り込みでも子の日付で集計すること、依存の遅れの判定、表示範囲、移動と伸縮（開始が終了を越えない）、警告の文
- [x] 実装し、`pnpm test core/gantt`、コミット

### Task 4: ガントの画面

- [x] `app/gantt/GanttView.test.ts` を足す: 行とバー、折りたたみ、ロードマップの切り替え、プロパティの選択、読み取り専用の Store でドラッグを止めること、キーボードの確定で PATCH が1回
- [x] `GanttView.svelte`（左のツリーと SVG）、`GanttTab.svelte`、`SaveViewButton`、`WorkspacePage`、`ProjectPage`、`ProjectHeader`、`WorkspaceHeader`、`ViewPage`、`QueryPage`、`ViewSave` を実装する。200 行を超えたら見える行だけ描く
- [x] `pnpm test`、`pnpm check`、`pnpm lint`、コミット

### Task 5: レポートの画面

- [x] `core/reports.test.ts` を足す: 理想線、残りの日がない未来の日、累積の積み上げ、`unknown` の列、平均のベロシティ、見積もりと実績の幅と超過、`1h30m` の日数の表示
- [x] `core/reports.ts`、`app/reports/Plot.svelte`（uPlot の包み。図の代わりの `aria-label` と表）、`useReport`、`ReportsPage.svelte` を実装し、`WorkspacePage` に足す
- [x] `pnpm test`、`pnpm check`、`pnpm lint`、コミット

### Task 6: スプリントボードのバーンダウン

- [x] `SprintBurndown.svelte` を足し、`SprintsPage` の上部に「残り points」「途中で追加」と図を出す。`SprintsPage` のテストを足す
- [x] `pnpm test`、コミット

### Task 7: E2E と CLI との突き合わせ

- [x] `web/e2e/gantt.spec.ts`: タブ、バー、ドラッグで1コミット、キーボード、依存の警告、ロードマップ、`layout: gantt` のビューの保存と再表示、`/query/gantt`
- [x] `web/e2e/reports.spec.ts`: スプリントの作成から開始、完了、終了までを GUI で行い、バーンダウン、ベロシティ、累積フロー図、サイクルタイム、見積もりと実績が出ること。件数と期間の切り替え、CLI の行。日数に依存するアサーションは書かない
- [x] `web/e2e/screens.ts` の語の一覧を足す
- [x] 6a〜6c のコマンドを1つずつ GUI の操作と突き合わせ、足りないものを足す。表を引き継ぎに書く
- [x] `make e2e`、`make web-check`、`make lint`、`make test`、`make licenses`

### Task 8: 文書と仕上げ

- [x] 1440px の画面を開いて見る
- [x] spec の 6e の文、6d の引き継ぎを直し、6e の引き継ぎを書く
- [x] 親設計の `layout:` の例を直す
- [x] PR を出し、CI の `github-live` がテストを実行したことまで確かめる
