# フェーズ 4f-3 の引き継ぎ（2026-11-05 時点）

4f-3（サイドバー・マイタスク・閲覧のみの表示・Store 間の移動）の実装は終わった。PR はまだ出していない。

## いまの状態

- ブランチ `phase4f3-stores`。4f-2 のマージ後の main（1775b2e）から切った。4f-2 の引き継ぎを完了にした docs（d56b47c）も、このブランチに入っている（文書だけの PR は出さない、という利用者の決定による）
- PR は未作成。main の取り込みもまだしていない
- plan: `docs/superpowers/plans/2026-11-04-phase4f3-stores.md`（Task 1〜7。末尾に「検査の結果」「Task 7 の Step 1・2 の結果」「実装で plan から変えた点」「細部 34〜51 と 92〜106 の実装とテスト」の表）
- 概要 plan `2026-11-04-phase4f-screens.md` に「4f-3 の plan で足した細部」（細部 92〜117）を足した

## やったこと

- Task 1 core（Store の色・非表示・`sharedPrefixes`・`displayId`・`cliMvStore`・パレットの `<store>:<ID>` の判定・`store.read_only` の文言）、Task 2a〜2c 閲覧のみ（判定・印・帯・TaskDetail、ツリー・カンバン・テーブル、作成・設定・ビュー・通知・パレット）、Task 3 非表示の Store、Task 4 マイタスクのワークスペース別と非表示の案内、Task 5 Store 名付きの ID とコメントの本人確認、Task 6 Store 間の移動、Task 7 仕上げ
- Go のファイルは変えていない。API の形も変えていない（web と docs だけ）。E2E は harness に `readOnlyStore` を足し、`readonly`・`hidden`・`ids`・`store-move` を書き、`my` に足した
- 検査: `go vet ./...`・`go vet -tags e2e ./internal/app/cli`・`gofmt -l .`（出力なし）、`golangci-lint run ./...`（0 issues）、`go test -race -count=1 ./...`（39 パッケージ ok）、`pnpm gen`・`go generate ./internal/app/server` のあと生成物に差分なし、`pnpm check`（845 ファイル、エラー 0・警告 0）、`biome ci .`（指摘 0）、`vitest run`（154 ファイル・2133 件 PASS）、`pnpm build`、`make e2e` を3回（3回とも 134 件 PASS、flaky 0）
- 実装中に直した点（plan 末尾に項目）: `sharedPrefixes()` は読み取り専用、`StoreBadge` は `useReadOnly` で決める、閲覧のみの拒否に Store 名を出す、カンバンの列の「+」の条件、無効の理由を `Button` の `why` で読み上げにも出す、非表示の判定を `app/hidden.ts` に集約、パレットの検索は `-store:<名前>` で外す、ID の出し漏れと型での渡し忘れの検出、移動先の親がワークスペースを決める、移動の送信中は選択を止める、移動のあとにも選べなくなった移動先を外す、移動の予定の ID も Store 名付き

## 実装中の決定

plan の細部 92〜106、概要 plan の細部 107〜117、plan 末尾に記録済み。次の回が従う事実は次のとおり。

1. 閲覧のみの判定は `useReadOnly(store)`（`web/app/read-only.ts`）と、`StoreDirectory.readOnly()`。無効の理由は `ui/Button` の `why` で読み上げにも出す
2. 非表示の判定は `web/app/hidden.ts` の `storeShown`・`hiddenStores`。外すのは画面の一覧だけで、取得の問い合わせは変えない（パレットの語の検索・クエリだけは `-store:<名前>` を足してサーバーで外す）
3. Store 名付きの ID は `StoreDirectory.displayId(store, id)`。`IdLabel` の `store` は省略できない。本文の `#<ID>` とメンションの挿入は Store 名なしのまま
4. Store 間の移動はキャッシュの `refreshTrees()` で取り直す（応答に `commit` がないため。移動で起きる `tasks.changed` は外からの変更として出る）

## 次

- この回の作業と引き継ぎを1つにまとめて PR を出す。出す前に main を取り込み、`make e2e` を流して通ることを確かめる
- マージのあと、4f は完了する。phase4 の spec（`docs/superpowers/specs/2026-11-01-phase4-gui-design.md`）の小フェーズは 4a〜4f で、4f が最後。次はフェーズ4の完了の確認（spec の「フェーズ4全体の完了条件」と「画面と範囲」の各画面が GUI で動くことの確認）。spec は 4f のあとに続く小フェーズを書いていない。SettingsAutomations などフェーズ5の画面は、そのフェーズの完了条件に含まれる
- 4f-3 で残っている作業はない
