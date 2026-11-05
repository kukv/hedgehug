# フェーズ 6a の引き継ぎ（2026-11-05 時点）

6a（計画の土台）は plan の Task 1〜9 を実装した。CLI と API でスプリントの作成から開始、終了、持ち越しまでを回せる。リリース、作業時間、`sprint:`・`release:` のクエリも動く。`internal/app/cli/scenario/sprint_test.go` が2人のユーザーで通しを確かめる。

## plan から変えた点

- `planning` と `worklogs` を足したので、構成見直しの spec の領域の数は 16 にした。spec の 18 は 6b の `templates` と 6c の `reports` を足した後の数である
- `WorkspaceRemove` は、スプリントかリリースが残っていれば `workspace.not_empty` で拒否する。応答の `sprints`・`releases` に件数が入る
- `hh show` の作業時間は `log` の件数で出す（例: `2h15m (2 logs)`）

## 実装で分かったこと

- `sprint`・`releases` の検査は `Tx.sprintReleaseViolations` が担うので、usecase では同じ検査を持たない
- `Tx.RemoveFile` は `tx.sprints` に入らない。`checkWorkspacesEmptyOfSprints` はファイルから数え直すので、同じ操作の中で影響しない
- `hh undo` は Op を問わず変更したファイルを戻すので、`sprint-*`・`release-*`・`worklog-*` に追加の実装は要らなかった

## 手の確認

実際のバイナリで、一時的な `HEDGEHUG_HOME` に Store を作り、スプリントの作成、開始、作業時間の記録、完了、`--to next` での終了、`hh release notes`、`hh undo` を流した（2026-11-05）。結果は期待どおりで、`hh undo` は閉じる操作とタスクの移動を1回で戻した。

## 6d で直すもの

`web/app/workspace/counts.ts` は `tasks + deleted == 0` で削除を有効にする。6a ではサーバーがスプリントとリリースの残りを拒否するので、今の画面はボタンが押せてサーバーの文言を出す。spec の 6d のとおり、`sprints`・`releases` の件数を無効の理由に足す。

## 次の回

6b はこの branch がマージされた後の main から切り、最初のコミットでこの文書に直すところがあれば直す。
