# フェーズ 4e-4 の引き継ぎ（2026-11-03 時点）

4e-4（通知・アクティビティ・プロジェクトの設定・同期と保守・スキーマ）の実装が終わった。これでフェーズ 4e の4回がそろい、spec の「画面と範囲」はすべて GUI で動く。

## いまの状態

- ブランチ `phase4e4-settings`（main の e525019 から）。PR はまだ開いていない
- plan: `docs/superpowers/plans/2026-11-03-phase4e4-settings.md`（Task 1〜7。末尾に「画面と範囲の確かめ」「決めたこと」「組み上げで直した点」）
- 概要 plan `2026-11-03-phase4e-screens.md` に「4e-4 の plan で足した細部」（細部 91〜129）を足した。spec `docs/superpowers/specs/2026-11-01-phase4-gui-design.md` に 4e-4 の決定の段落がある

## やったこと

- Task 1 通知、Task 2 アクティビティ、Task 3 プロジェクトの設定、Task 4 同期と保守の骨組みと設定のナビ、Task 5 doctor と修復・purge、Task 6 スキーマ、Task 7 仕上げ。Go は触っていない（web だけ）
- Task 7 で、`grep -rn "4e-4" web/app web/core web/e2e` が 0 件であること、spec の「画面と範囲」の全行と E2E の一覧（作成、編集と自動保存、ツリーのドラッグ、カンバンの移動、CLI の変更が SSE で反映、通知の既読と revert、保存済みビュー、undo、削除と復元）がコードにあることを確かめ、plan の末尾に表で残した。足りないものはなかった
- 最終レビューの直し（plan 末尾の「組み上げで直した点」に項目）: 通知とアクティビティのフォーカス、undo の Store、プロジェクトの設定の再読み込み、スキーマの Store 表示、「2 つ以上の Store」と日付の共通化、e2e の署名
- 検査: `go vet`・`gofmt -l .`（出力なし）・`go test -race ./...`（28 パッケージ ok）、`pnpm fix`・`pnpm check`・`biome ci`、`vitest`（138 ファイル・1707 件 PASS）、`pnpm build`、`make e2e` を3回（3回とも 101 件 PASS）

## 実装中の決定

plan の「決めたこと」と概要 plan の細部 106〜129、spec に記録済み。

1. 通知の足元は `hh inbox --all`。既読にした上書きの行はボタンなし。同期の修復の文は detail の種類ごと
2. 通知（`/inbox?tab=overwritten|mention|repair`）とスキーマ（`/settings/schema/<store>?tab=kinds|props|workflow`）のタブはアドレスに持つ
3. アクティビティはすべての trailer を出し、タスクはあるが ID がないログは ULID を出す。`limit` は 1000 まで。日時を読めないコミットは「日時不明」
4. プロジェクトの上書きは scoped の GET がないので `SchemaView.scoped` から読む。`PUT`・`DELETE` のあとに `SchemaCache.reload(store)`。ステータスへの追加は既存のエントリに併合する。必須にできる項目は `internal/schema/check.go` から取る
5. `/settings` は `/settings/general` に置き換え、窓が隠れているあいだの fetch は `max(interval, 10 分)`、`SyncCard` は任意の `store` を受ける
6. doctor は `DoctorReport[]`、修復のあとは応答の findings を使う。E2E の harness は `hedgehug` を PATH に載せる
7. purge の日数は全角も受ける（NFKC）0〜36500 のテキスト欄
8. スキーマは `schema.changed` で check だけを取り直し、定義が1つなら「併合する…」を出さず、ワークフローは文の一覧で見せる

## 次

PR を開いて main にマージする。4e-4 で残っている作業はない。マージの前に main を取り込み、`make e2e` を流して全件 PASS を確かめる。
