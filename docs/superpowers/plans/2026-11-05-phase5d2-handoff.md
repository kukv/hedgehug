# フェーズ 5d-2 の引き継ぎ（2026-11-05 時点）

5d-2（表示）は plan の Task 1〜8 を実装した。`web/e2e/github.spec.ts` が fakegh を相手に、チップ、カンバンの説明、タスク詳細、概要、拒否の通知、アクティビティ、Issue の取り込み、パレットを確かめる。

## plan から変えた点

1. アクティビティの「自動化」は `?sync=automation` にした。`ActivitySync` に値を足すだけで済み、`Route` の形が変わらない。API へは `automation=only` で問い合わせる
2. パレットの `hh start` は、タスクを開いているときだけでなく、ID で見つけたタスクの「<ID> に対して」にも出る。キャンバスの置き場所に合わせた
3. 5d-1 の引き継ぎは直すところがなかった

## 実装で分かったこと

- `repos` の値は `<host>/<owner>/<repo>` でないと取得の対象にならない。`repo を追加` は `fullRepo` で `github.com/` を補う
- `hh add --repo` は値を取らず、カレントのリポジトリを使う。E2E で repo を指定するタスクは `POST /api/tasks` で作る
- 取得の間隔があるので、E2E で 2 回目の取得をするときは `hh.expireFetch(store)` を呼ぶ
- テストの fake の `api.GET` が配列でない値を返すと、`useGitHubRepos` の読み手が落ちる。`GitHubSection` を描く画面のテストは `/api/github` に `[]` を返す

## 5d-3 が使うもの

| 画面 | 使うもの |
|---|---|
| 既存の Issue をまとめて取り込む | `web/app/states/IssueImportDialog.svelte` のフォーム。今はプロジェクトを親に固定している。置き場所の選択を足して使い回す |
| ルールと Issue の同期 | `web/app/github.svelte.ts` の `useRules`・`useIssueFields`。`automations.changed` で読み直す |
| 取得時刻 | `useGitHubRepos`、`web/core/github.ts` の `fetchTimes`・`fetchLine`・`slowedText` |
| ルールのイベントの文言 | `eventLabel` |
| Issue の作成 | `POST /api/tasks/{id}/issue` の GUI はまだない。`GitHubSection` に足す。`issue.repo_required` の文言も `messages.ts` に足す |

## 未実施のこと

5a の引き継ぎの「未実施のこと」（検証用 Org での手の確認と、`github-live.yaml` の secret・PR）は 5d-2 でも実施していない。利用者が席にいるときに行う。
