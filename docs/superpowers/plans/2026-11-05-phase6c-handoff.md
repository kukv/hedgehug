# フェーズ 6c の引き継ぎ（2026-11-05 時点）

6c（レポート）は plan の Task 1〜5 を実装した。`hh report burndown/velocity/cfd/cycle/estimate` と `/api/reports/*` が動く。`internal/app/cli/scenario/report_test.go` が、2人のユーザーで閉じたスプリントのバーンダウンとベロシティを履歴から確かめる。

## plan から変えた点

- 手で閉じたスプリントは、`active` の版がなければ閉じたコミットの時刻も使わず、`start` の日の 00:00 から `end` の日の 24:00 までとする。spec のとおりにするため、`window` で終了もやり直す
- `POST /api/sprints/{id}/close` の本文を省略可にした。6a では本文が必須で、spec の「未完了がなければ `to` は要らない」と食い違っていた

## 計測

10万コミット、5,000 タスクの生成器で `reportcache.Read` を測った（Apple M5、`HH_BENCH=1`）。

| | 時間 |
|---|---|
| 初回（全版を git から読む） | 7.9 秒 |
| 2回目（キャッシュ） | 0.05 秒 |

spec の目標（2回目が1秒以内）を満たす。`make bench-history` に `reportcache` を足した。

## 実装で分かったこと

- `usecase.Classify(nil)` は nil でない `error` を返す。成功の経路で呼ばない
- `findSprint` などは `planning` から `usecase` の土台（`plan_find.go`）へ移した。`reports` から使うため
- 1分未満の時間は `model.Duration` の表記で `0m` になる

## 手の確認

実際のバイナリで、一時的な `HEDGEHUG_HOME` に Store を作り、開始の後にタスクを足し、1件を完了して閉じ、5つのレポートを流した（2026-11-05）。閉じた後のバーンダウンは閉じる前と同じ値で、持ち越したタスクも範囲に残った。ベロシティは完了の 3 points、見積もりと実績も期待どおりだった。

## 次の回

6d はこの branch がマージされた後の main から切り、最初のコミットでこの文書に直すところがあれば直す。6d の最初に、6a の引き継ぎの「6d で直すもの」（`counts.ts`）を片付ける。
