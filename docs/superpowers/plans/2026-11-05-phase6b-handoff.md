# フェーズ 6b の引き継ぎ（2026-11-05 時点）

6b（テンプレートと一括更新）は plan の Task 1〜6 を実装した。`hh add --template`、`hh template`、`hh schema scope set --templates`、`hh set --query` と、それぞれの API が動く。`internal/app/cli/scenario/template_test.go` が2人のユーザーで通しを確かめる。

## plan から変えた点

- `POST /api/tasks/bulk` の本文は、`PATCH /api/tasks/{id}` と同じ項目にした。spec を直した
- 新しい領域は `templates` だけで、構成見直しの spec の領域の数は 17 にした。spec の 18 は 6c の `reports` を足した後の数である

## 実装で分かったこと

- `schema.yaml` の `kinds` はフロー形式（`{name: task, ...}`）で書かれている。行で置き換えるテストは効かない
- `Op.CheckOnly` は fn を実行するので、`fn` は検査と書き込みで2回呼ばれる。`Bulk` の `write` は毎回 Tx から読み直す
- 展開のとき、子には種類の既定テンプレートを適用しない

## 手の確認

実際のバイナリで、一時的な `HEDGEHUG_HOME` に Store を作り、`hh template edit`、`hh add --template`、`hh set --query` の `--dry-run`・違反・`--yes`、`hh undo` を流した（2026-11-05）。孫まで1コミットで作られ、違反は全件出て何も書かれず、`hh undo` は一括更新と展開をそれぞれ1回で戻した。

このとき `hh log` の件名で、一括更新の件数が最初のタスクの ID に置き換わる不具合が見つかった。`subjectNow` は件名の2語目をタスクの名前と見なしていたので、`bulk-set` を除いた。同じ理由で、ログの `task` と `id` も一括更新では出さない。

## 次の回

6c はこの branch がマージされた後の main から切り、最初のコミットでこの文書に直すところがあれば直す。
