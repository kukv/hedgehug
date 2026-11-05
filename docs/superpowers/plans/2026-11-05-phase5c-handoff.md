# フェーズ 5c の引き継ぎ（2026-11-05 時点）

5c（Issue 同期）は plan の Task 1〜10 を実装した。3 つのモードと `hh issue`・`hh gh import` が fakegh で動き、手編集の上書きが inbox に出る（`internal/app/cli/scenario/github_issue_test.go`）。

## plan から変えた点

1. `IssueValues` は `model/github.go` でなく `model/github_values.go` に置いた
2. `yaml` のデコーダは `*yaml.Node` のフィールドを空のまま返す。`Task.GitHub()` が `gh` の元のノードから `Synced` を取り直す。5a の `Synced` は読めていなかった（値を書く処理がなかったので表に出なかった）
3. `IssueEdit` は作らず、`fakegh.Server.EditIssue` はそのままにして `TagIssue` を足した。`ReopenIssue`・`DropIssue`・`AddLabel`・`AddMilestone` も足した
4. `resolver` は `usecase.IssueResolver` として公開した。`hh issue` が使う
5. `gh-push` は、GitHub を基準へ戻しただけで `gh` が変わらないときも通知のために空のコミットを書く（`AllowEmpty`）
6. `push` の本文に印がなければ、本文が同じでも書き直す。spec の「末尾に入れる」を保つため
7. Task 7〜9 は 1 つのコミットにまとめた
8. 最終レビューで直した点: `hh issue` で閉じる処理に失敗してもリンクを書く（決定 17 の二重作成を防ぐ）。タイトルは前後の空白を落として比べる（GitHub が落とすため）。読めなかった `mirror` のリンクも `taken` にフィールドを持ち、次のリンクへ渡さない

## 5d が使うもの

- inbox の種類 `issue_overwritten`、詳細は `{ref, field, side, lost}`。CLI の文言は `notifications/inbox.go` の `issueOverwrittenContent`。web のタブと文言は 5d-2
- `gh-mirror`・`gh-push` は「同期」に入れた（`syncOps`・`SYNC_OPS`）。`mirror` のステータスの変更もここに入る。「自動化」の絞り込みに移すのは 5d-1
- `integration.CreateIssue`・`ImportIssues` と `IssueWrite`・`ImportResult`。API は 5d-1、`hh gh import` の GUI は 5d-3
- `usecase.GitHubIssueValues`・`TaskIssueValues`

## 未実施のこと

5a の引き継ぎの「未実施のこと」（検証用 Org での手の確認と、`github-live.yaml` の secret・PR）は、5c でも実施していない。利用者が席にいるときに行う。Issue の `PATCH`・`POST`、`labels`・`assignees`・`milestones` の読み取り、`OpenIssues` の実 API も、そのときに確かめる。`go vet -tags live` までは通した。実際に接続はしていない。
