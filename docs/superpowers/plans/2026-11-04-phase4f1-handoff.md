# フェーズ 4f-1 の引き継ぎ（2026-11-04 時点）

4f-1（Go の不足）の実装が終わった。PR はまだ開いていない。

## いまの状態

- ブランチ `phase4f1-go`。構成見直しの回 1 のあとの main（6d6ce10）から切った。PR は未作成
- plan: `docs/superpowers/plans/2026-11-04-phase4f1-go.md`（Task 1〜5。末尾に「検査の結果」「実装で plan から変えた点」「概要の細部 1〜16 の実装とテスト」の表）
- 概要 plan `2026-11-04-phase4f-screens.md` に「4f-1 の plan で足した細部」（細部 52〜70）を足した。spec `docs/superpowers/specs/2026-11-01-phase4-gui-design.md` の「親設計の修正」に `hh store set` の1行がある（e98cec3）

## やったこと

- Task 1 Store の色と表示（`stores[].color`・`hidden`、`hh store set`、`StoreView`）、Task 2 Store の書き込み・発見・マウントの API と SSE `stores.changed`、Task 3 アカウントの API と Device Flow（開始・やめる・SSE `auth.device`）、Task 4 Store 間の移動の API（`move-store/plan`・`move-store`）、Task 5 仕上げ。web は型の再生成と `web/core/messages.ts`・`web/core/connection.ts` だけ
- 検査: `go vet ./...` と `gofmt -l .`（出力なし）、`go test -race ./...`（28 パッケージ ok）、`go generate ./internal/app/server` と `pnpm -C web gen` のあと生成物に差分なし、`pnpm -C web check`（エラー 0・警告 0）、`biome ci .`（指摘 0）、`vitest run`（138 ファイル・1714 件 PASS）、`golangci-lint run ./...`（0 issues）。`make e2e` は流していない（4f-1 は画面を変えない）
- 実装中に直した点（plan 末尾に項目）: `default_store` の追加、`ReleaseFor` の黙った開き直し、Device Flow の HTTP の context と終わりの `sync.Once`・`WaitGroup`、`MoveStoreFields` の切り出し、`scoped` の enum、移動が途中で止まったときの poller、409 `auth.device_running` の引き継ぎ、`tokenWritten`、`SetStoreLook` の行コメント

## 実装中の決定

plan の「決めたこと」1〜13、概要 plan の細部 52〜70、plan 末尾に記録済み。4f-2・4f-3 の画面が従う API の決まりは次の4つ（作業ではなく事実）。

1. `choice.required` は discover が 422、move-store が 400。GUI は `type` で分岐し、status では分岐しない
2. `move.invalid` の `args.violations` は文字列の配列（`"  <ID> <title>: <violation>"`）、`schema.violation` はオブジェクトの配列。4f-3 の細部 49 は両方を扱う
3. `MovePlanView.unlinked[].field` は `props.<name>`。画面が `props.` を外す
4. Store の追加と `PATCH` の応答は `default` を持たない `StoreView`。GUI は `default_store` を使い、書き込みのあとに取り直す

## 次

- PR を開く
- マージのあと、4f-2 を main から切る
- 構成見直しの回 2 以降は 4f-1 のあとに行い、4f-1 のコードも一緒に移す（2026-11-04 の利用者の決定）。4f-1 で残っている作業はない
