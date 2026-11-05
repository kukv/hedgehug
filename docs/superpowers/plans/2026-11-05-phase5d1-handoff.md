# フェーズ 5d-1 の引き継ぎ（2026-11-05 時点）

5d-1（Go の API）は plan の Task 1〜9 を実装した。全エンドポイントを Go のテストで確かめ、`api/openapi.yaml`・`web/core/api/schema.d.ts` に差分がない。キャンバスの 3 点と、親設計の `title`・`branch`・`via` も直した。

## plan から変えた点

1. `ghstate.Repo.NextAt` は `now` によらない値を返す。watcher が 300ms ごとに `SyncStatusView` を比べるので、`now` を返すと `sync.state` が流れ続ける。待ちがなければゼロの時刻で、API では省く
2. 表示用に持つのは `Slowed` でなく `SlowedUntil`。画面が回復の時刻を出すため。レート制限の拒否（`NotBefore`）も含む
3. 取り込みは、どのタスクも指さなくなった repo を `ghstate` から消す（`dropGone`）。残すと警告と取得時刻が要約に出続ける
4. `AuthSource` は存在しないアカウントに既存のコード `account.not_found` を返す
5. `ActivityPage.svelte` は `sync=exclude` に `automation=exclude` を足して問い合わせる。「人」のタブに自動化のコミットが混ざらないようにするため。「同期」のタブは `merge`・`repair` だけになる。「自動化」のタブは 5d-2
6. `gh-push` は、書いたタイトルも `gh` に写す。次の取り込みで差分のコミットが出ないようにするため
7. `fakegh` に `UnrestrictApp` を足した。読めるようになる流れのテストに使う
8. `GET /api/github` は閲覧のみの Store の repo を返さない。取り込まないので、未取得のまま出続けるため

## 5d-2・5d-3 が使うもの

| 画面 | API |
|---|---|
| チップ、PR 列、タスク詳細 | 一覧と `GET /api/tasks/{id}` の `links[]`（`title`・`branch`・`via`・`fields`・`state`・`review`・`checks`） |
| リンク、リンクを外す、モードの切り替え | `POST`・`DELETE /api/tasks/{id}/links`。切り替えは `POST` に `sync` |
| Issue の作成 | `POST /api/tasks/{id}/issue`。repo が決まらなければ `issue.repo_required` |
| 取得時刻、延長の表示 | `sync.state` と `GET /api/sync` の `github`（要約）、`GET /api/github`（repo ごと） |
| Issue の取り込み | `POST /api/github/import`。件数は `dry_run`。repo の候補は `GET /api/github` |
| 自動化の設定 | `/api/automations` 一式、`/api/automations/stats`、`/api/automations/fields`。変更は `automations.changed` |
| プロジェクトの設定 | `GET /api/automations?project=`、`PUT /api/automations/{id}/enabled` の `project` |
| `repo を追加` | `PATCH /api/tasks/{id}` の `repos` |
| `repo_accounts` | `GET`・`PUT /api/repo-accounts` |
| App の警告 | `GET /api/github` の `app_missing`・`account`。`gh のトークンで代わりに使う` は `PUT /api/accounts/{name}/token-source`、`管理者にインストールを依頼` は web が GitHub の URL を作る |
| アクティビティ | `?automation=only\|exclude`。拒否は `gh-fetch` などのコミットの `Hedgehug-Automation-Rejected` の trailer |
| ライブ表示 | `tasks.changed` の `automated`。`cache.ts` は対応済み |

E2E で App の未インストールを作るときは `fakegh repo restrict <owner>`。

## 未実施のこと

5a の引き継ぎの「未実施のこと」（検証用 Org での手の確認と、`github-live.yaml` の secret・PR）は 5d-1 でも実施していない。利用者が席にいるときに行う。App を外した Org の確認で、`Resource not accessible by integration` の応答で `app_missing` が立つことも確かめる。`go vet -tags live` までは通した。実際に接続はしていない。
