# フェーズ 4e-3 の引き継ぎ（2026-11-03 時点）

4e-3（カンバン・テーブル・保存済みビュー・クエリの画面・マイタスク）は実装まで終わった。残りは PR。

## いまの状態

- ブランチ `phase4e3-views`（main の 79a5277 から）。`git log origin/main..HEAD` が実装の全部
- plan: `docs/superpowers/plans/2026-11-03-phase4e3-views.md`（Task 1〜7、末尾に「決めたこと」と「組み上げで直した点」。レビューでの直しもそこにある）
- 概要 plan `2026-11-03-phase4e-screens.md` の細部 36・39・48〜50・58 を利用者の決定に合わせて直し、細部 81〜90 を足した。spec `docs/superpowers/specs/2026-11-01-phase4-gui-design.md` に 4e-3 の決定の段落がある

## やったこと

- Task 1〜7 をすべて実装し、Task ごとに spec とコード品質のレビューを受けて直した。最後に最終レビューを受け、その指摘も直した（カテゴリの定義の一本化、`defaultStatus` の削除、`useListScope` への共通化、フィルタをタブをまたいで残す、`PUT` の記述を `PATCH` に揃える、operationId を `update-view` にする、など）
- 検査: `go vet`・`gofmt`・`go test -race ./...`、`pnpm fix`・`pnpm check`・`biome ci`、`vitest` 全件、`make e2e`（84 件が PASS）

## 実装中の利用者の決定（2026-11-03）

plan の「決めたこと」と「組み上げで直した点」、spec に記録済み。

1. `TaskView.scope`: サーバーが、タスクのスキーマを決める scoped の上書きの祖先の ULID を返す。カンバンの事前の判定はこれで引く
2. `PATCH /api/views/{name}` を、改名に加えて設定の変更（`layout`・`query`・`columns`・`group_by`・`kanban_columns`・`swimlane`）にも使えるように広げる。ビューの画面のレイアウト切り替えと「ビューを更新」はこれで送る。ビューが無ければ作らず 404（`view.not_found`）。CLI の `hh view save` は変えない

## 次

PR を作る（`live.spec.ts` の既存の競合は PR #42 `fix-poll-per-commit` で別に直している。`a task added in the page does not get the mark` が `make e2e` で1度落ちることがあるのは、その競合）。
