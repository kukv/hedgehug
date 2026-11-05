# フェーズ 6c: レポート Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** 閉じたスプリントのバーンダウンとベロシティが git 履歴から正しく出るようにする。累積フロー図、サイクルタイム、見積もりと実績も CLI と API で出す（spec の 6c の完了条件）。

**Architecture:** 計算は `domain/report` に置き、入出力は値だけにする。タスクの版は `storage/reportcache` が git から読み、コミットとパスを鍵に `.hedgehug/local/report` へ残す。`usecase/reports` が履歴、スプリントのファイルの履歴、index を組み合わせる。

**Tech Stack:** Go、go-git、encoding/gob、cobra、huma v2。

**Spec:** `docs/superpowers/specs/2026-11-05-phase6-planning-design.md` の「6c レポート」。

**PR:** この plan と実装を1つの PR に入れる。

## Global Constraints

- 検査の完了は `make check > log 2>&1; echo exit=$?` の exit 0 と `make: ***` の行がないことで判断する
- シナリオのテストは PATH にコマンドのないジョブでも通るようにする。補助のスクリプトは外部コマンドを使わない
- API を足したら `go generate ./...` と `pnpm gen` の生成物をコミットに入れる
- 時刻は `ss.Now()` と `ss.Location()` を使う。日の区切りは表示する人のタイムゾーン

## この回で決めたこと

| # | 項目 | 決定 | 理由 |
|---|---|---|---|
| 1 | 名前の解決 | `findSprint`・`pickWorkspace` を `planning` から `usecase` の土台へ移す | `reports` から `planning` を import できない |
| 2 | 版の順 | `history.Path` の順を逆にしたもの | spec のとおり。author time、世代、ハッシュの順が一意に決まる |
| 3 | 閉じる操作の扱い | `Version.Commit` を持ち、終了の時点の状態は閉じるコミットの版を除いて読む | 同じ秒に完了と終了があっても、閉じるときに移されたタスクが範囲に残る |
| 4 | 範囲の日 | 最初の日は `start` と開始の時刻の早い方。最後の日は閉じた日、閉じていなければ今日 | 予定より長引いた日も描く |
| 5 | 移動元の版 | 版の先頭が `move-in` のタスクだけ、マウント中の Store の転送を探して読む | 全 Store のキャッシュを開かずに済む |
| 6 | `points` と `estimate` | プロパティ名は固定（spec のとおり）。`estimate` は今の値だけを読む | 見積もりと実績は今の状態の比較 |

## ファイル

| 操作 | パス | 役割 |
|---|---|---|
| 作成 | `internal/app/domain/report/report.go` | 版、日、バーンダウン、ベロシティ、累積フロー図、サイクルタイム |
| 作成 | `internal/app/storage/reportcache/cache.go` | 版の読み出しとキャッシュ |
| 変更 | `internal/app/storage/history/history.go` | `Paths` |
| 作成 | `internal/app/usecase/plan_find.go` | `planning` から移す解決 |
| 作成 | `internal/app/usecase/reports/reports.go` | 組み立て |
| 作成 | `internal/app/cli/report.go` | `hh report` |
| 作成 | `internal/app/server/reports.go` | API |

## Task 1: domain/report

- [x] 表形式のテストを書く: 初期の範囲、開始後の追加、外れたタスク、閉じるときの移動、開き直し、canceled、削除、ベロシティの `points` なし、累積フロー図の `unknown`、サイクルタイムの中央値と p85
- [x] `Task.at`、`Burndown`、`Velocity`、`CFD`、`Cycle`、`Days` を書く
- [x] コミット: `feat(report): レポートの計算を足す`

## Task 2: 版の読み出し

- [x] テストを書く: 版の順、転送と削除の版、壊れた版を飛ばす、2回目は git を読まない、形式の違いで作り直す、消えたコミットの行を捨てる
- [x] `history.Paths`、`reportcache.Read`、`reportcache.Versions`（1タスクだけ）を書く
- [x] コミット: `feat(reportcache): タスクの版をキャッシュする`

## Task 3: usecase/reports

- [x] `planning` の解決を `usecase/plan_find.go` へ移す
- [x] テストを書く（`te.now` でコミットの時刻を進める）: 閉じたスプリントの通し、途中の追加、手で閉じたスプリント、`planned` の拒否、1つに決まらないときのエラー、`--by count` への切り替え、Store 間の移動、移動元が未マウントの警告
- [x] `Burndown`、`Velocity`、`CFD`、`Cycle`、`Estimate` を書く
- [x] コミット: `feat(reports): レポートを組み立てる`

## Task 4: CLI と API

- [x] `hh report burndown/velocity/cfd/cycle/estimate`。表に文字のバーを添え、`--json` を出す
- [x] `GET /api/reports/burndown`・`/velocity`・`/cfd`・`/cycle`・`/estimate`
- [x] コミット: `feat: hh report とレポートの API を足す`

## Task 5: 仕上げ

- [x] `cli/scenario` に、スプリントを閉じてバーンダウンとベロシティを確かめる通しを足す
- [x] 10万コミットの生成器で初回と2回目の時間を測り、引き継ぎに書く
- [x] 構成見直しの spec の領域の数を 18 にする
- [x] 実際のバイナリで手の確認を流し、引き継ぎを書く
- [x] `make check`
