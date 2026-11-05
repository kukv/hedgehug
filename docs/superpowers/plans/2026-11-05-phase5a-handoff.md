# フェーズ 5a の引き継ぎ（2026-11-05 時点）

5a（リンクと取り込み）は PR #61 で main にマージした。検証用 Org での手の確認（Task 16 の Step 5）だけが未実施で、下の「未実施のこと」に残す。「いまの状態」と「次」の 1・2 は、マージの前の記録である。

## いまの状態

- ブランチ `phase5-design`。実装の変更はすべてコミットせず、index に積んである（この引き継ぎと Task 16 の変更、最終レビューの修正も、controller が index に積む）。1Password がロックされる時間帯に作業したため、コミットは利用者が席にいるときに行う
- PR #61 は open。中身は spec と plan だけで、実装は入っていない（文書だけの PR は出さない、という利用者の決定により、この PR に 5a の実装を足す）
- plan: `docs/superpowers/plans/2026-11-05-phase5a-links.md`（Task 1〜16）
- spec: `docs/superpowers/specs/2026-11-05-phase5-github-design.md`
- 通しのテストは、`internal/app/cli/scenario` がまだ main にないため、plan の第2案の場所 `internal/app/cli/github_scenario_test.go` に置いた

## やったこと

- Task 1 `gh` の型とリンクの定数（`model/github.go`）、Task 2 `gh` の配列を `ref` で要素ごとにマージ、Task 3 プロパティテストに `links` と `gh` の同時更新を足す
- Task 4 索引に `links`・`repos` を持たせ `has:pr` を足す、Task 5 `hh edit` で `gh` の変更を拒否
- Task 6 自動化の author（`config.AutomationAuthor`、`store.Op.Automated`）と op `gh-fetch`
- Task 7 `lib/github` の GraphQL クライアント、Task 8 `lib/fakegh` の PR・Issue・GraphQL（と `cmd/fakegh` の操作コマンド）
- Task 9 `repo_accounts` でアカウントを選ぶ、Task 10 取得時刻を持つ `storage/ghstate`
- Task 11 `hh link`・`hh unlink`（`usecase/integration/link.go`、`cli/link.go`）、Task 12 自動リンクと `gh` の更新
- Task 13 取り込みと `hh sync`（`integration.Fetch`、`replication.SyncStore`、server の定期同期）、Task 14 `hh show` にリンクを出す
- Task 15 実 API の定期テスト（`internal/lib/github/live_test.go`、`.github/workflows/github-live.yaml`）
- Task 16 通しのテスト `TestSyncBringsGitHubIntoTheTask`（`sync` で PR が入り、`show` と `has:pr` に出て、`unlink` で消える）、構成見直しの spec の修正、全体の検査
- 全体の検査（2026-11-05、すべて通った）
  - `gofmt -l .`（出力なし）、`go vet ./...`、`go vet -tags e2e ./internal/app/cli`、`go vet -tags live ./internal/lib/github`
  - `golangci-lint run ./...`（0 issues）
  - `go test -race -count=1 ./...`（43 パッケージ ok、失敗 0、テストのないパッケージ 6）
  - `go generate ./internal/app/server` と `pnpm gen` のあと、`api/openapi.yaml` と `web/core/api/schema.d.ts` に差分なし
  - `pnpm check`（845 ファイル、エラー 0・警告 0）、`pnpm biome ci .`（指摘 0）、`vitest run`（2133 件 PASS）、`pnpm build`
  - `make e2e`（134 件 PASS を2回続けて確かめた。最初の1回は 132 件 PASS・2 件 FAIL で、原因は下の「plan から変えた点」の 5）

## plan から変えた点

Task 16 で変えた点は次のとおり。Task 1〜15 で変えた点は、plan の各 Task の本文に反映してある。

1. 通しのテストの先頭にある `te.dropDefaultAccount()` は書かない。`writeConfigYAML` が `accounts:` を含むとき、既定のアカウントを自分で外すため（呼ぶと、外す対象がなく失敗する）
2. plan のテストは `ME-1` を前提にしていたが、Store の作成が書かれていなかった。`store init me --prefix ME --account me` を足し、`add a -p repos=github.com/org-a/web` で `repos` を渡す
3. 構成見直しの spec の 2.3 は、plan にあった「14 個」でなく「13 個」にした。`usecase/automations` と `domain/automation` は 5b で作るので、5a の時点でディレクトリがない。5b で 14 個にする。2.1 の配置には、いま存在する `storage/ghstate` と `usecase/integration` だけを足し、2.3 の表に `integration` の行を足した。plan の Step 3 の記述も同じに直した
5. `web/e2e/hidden.spec.ts` と `web/e2e/store-move.spec.ts` が `hh.json("add", ...)` の結果を `{id, ulid}` として読んでいた。いまの `hh add --json` は `{task, commit}`（`usecase.TaskWrite`）なので `id` が取れず、2 件が FAIL した（5a の変更が原因ではない）。`harness.ts` の `hh.add(...)`（`.task` を返す）に書き換えた。assert は変えていない
6. Step 4 の `git status --short` は、index に積んだ変更が `A`・`M` で出るため、生成物の差分は `git diff --stat -- api web/core/api`（index との差）で確かめた。出力なし

## 実装中の決定

plan の「この回で決めたこと」13 項目と、各 Task の本文に記録済み。5b 以降が従う事実は次のとおり。

1. `Task.GH` は `*yaml.Node` のまま持つ。読み書きは `GitHub()`・`SetGitHub()`。変えない部分は元のノードを残す（知らないキー、`1.0`、日付の形、コメント）。`SetGitHub` が `gh` を空にしたときは `Doc.drop` でキーを消す
2. `gh` の衝突は inbox に出さない。`gh` だけの変更は、削除との競合で編集に数えない（別のメンバーが消したタスクを、取り込みが復活させないため）。`links` の変更は数える
3. 取り込みの author は `config.AutomationAuthor`、op は `gh-fetch`。`store.Op.Automated` を立てると、セッションが author を差し替える。`hh undo` は `gh-fetch` を飛ばし、`Kept` にも `gh` を出さない。活動の分類は `syncOps`（server）と web の `SYNC_OPS` に入れた
4. 走査の起点は `ghstate` の `Since`（GitHub の時計で見た PR の `updatedAt` の最大値）。`LastFetch`（手元の時計）は間隔の判定だけに使う。失敗は `LastTry` と `Warning` に残し、間隔内は問い合わせずに `Warning` を再掲する。タスクに書けなかった repo は `LastFetch` を進めない
5. remote のない Store も取り込む（push はしない）。`HasTargets` で取得対象を持つ Store を決め、`hh sync`（引数なし）の対象に入れる。server の定期同期は、remote のない Store への書き込みを push の待ちにしない
6. 取り込みの多重実行は `github.lock` で1つにする。`sync.lock` とは別
7. `review` の決め方、`state_reason` の写し方（`DUPLICATE` は `not_planned`、理由のない古い closed は `completed`）は spec に反映済み。クライアントが決める
8. GraphQL は、NOT_FOUND で path が2つ以上のものはその要素だけが null になるので、エラーにしない。それ以外のエラーは `data` があっても全体の失敗にする（repo 単位の警告）
9. fakegh の GraphQL は、クライアントが送るクエリを `query.go` で GitHub のスキーマの形と照らしてから答える。誤記や知らないフィールドは `errors` で返る
10. `Link` が `Fields`（スライス）を持ち comparable でなくなったため、`merge/task_test.go` の `Link` の比較の2箇所だけ `sameLink` と `slices.EqualFunc` に直した。期待値は変えていない

## 最終レビューで直した点

1. ref の大文字小文字。`ParseGitHubRef` が小文字にして読み、`String()` は常に小文字を返す。比較は `model.CanonRef` を通す（`apply`、`link`、`unlink`、`LinkViewsOf`）。`links` に `github.com/Org-A/web/pull/1` が手で書かれていても、取り込みで `gh` が入り、`hh link` し直しても重複せず、`hh show` に出る。`merge` の `links` の見分け方と、既存のファイルは変えていない。`show --json` の `ref` と `label` は正規形で出る
2. `hh show --json` の `links[]` に `merged_at` と `state_reason` を足した（`omitempty`）。`api/openapi.yaml` と `web/core/api/schema.d.ts` を再生成した
3. `Fetch` の途中で `UpdateCommit` が失敗したときも、読めなかった repo の `LastTry` と `Warning` を `ghstate` に書く。読めた repo の `LastFetch` は進めない
4. 5b の spec のイベントの節に、`hh unlink` のあと `hh link` し直すと `pr.opened` が発火することを書いた
5. `go test -race` で `TestLoopLogsNoTargetsOnce`（`internal/app/server/loop_test.go`）に data race が出た。remote のない Store も定期同期の対象になった（Task 13）ため、`l.step()` が背景で同期を始め、ログを書く。テストは同期が終わる前に `ts.log` を読んでいた。`lines` の中で `l.waitUntil` を呼んで同期の終了を待つようにした（期待する行数は変えていない）。`-count=20` と `internal/app/server` の `-count=3` で確かめた
6. 「親設計の修正」は、親設計ドキュメントを rev 372 まで反映済みで、この spec からの残りはない

## 未実施のこと

- Task 16 の Step 5（検証用 Org での手の確認）は未実施。利用者が席にいるときに行う。次の3つを1回ずつ確かめ、結果をこの文書に書く
  1. `hh auth login <name>` の Device Flow でログインし、8 時間の期限の前にトークンが更新されることを、`hh auth status` の残り時間で確かめる
  2. Hedgehug の GitHub App を検証用 Org に入れた状態で、`hh sync` が `hh-live` の PR を取り込むことを確かめる
  3. App を外した Org の repo では、repo 名付きの警告が出て、同期が成功のままであることを確かめる
- `github-live.yaml` は、secret `HH_LIVE_ORG`・`HH_LIVE_TOKEN` と、検証用 Org の `hh-live` に PR が1件以上あることが前提。リポジトリに secret を入れたか、PR を置いたかは、この作業では確かめていない（実際に流したのは `go vet -tags live ./internal/lib/github` までで、実 API には接続していない）

## 次

1. 完了。PR #61 でコミットしてマージした
2. 完了。main へ rebase した。`internal/app/cli/github_scenario_test.go` は `newTestEnv` のプロセス内のテストなので `cli` に残す。`cli/scenario` はビルドしたバイナリを動かす場所で、5b の通しのテストはそちらに足す
3. Step 5 の手の確認を行い、結果を書く。そのあと PR #61 に 5a の実装を足してレビューに出す
4. 5b は、5a のマージ後の main から plan を切って書く。5b が使うものは次のとおり
   - `integration.Fetch` の差分の取り方（`Since` 以降の PR と、リンク済みの PR・Issue を毎回すべて読む）。5b の「ルールの実行」は取り込みの後（spec の取り込みの順番の 5）に呼ぶ
   - `store.Op.Automated`（取り込み以外の自動化のコミットにも同じ author を使う）
   - `usecase/integration/link.go` の `defaultIssueSync`。いまは `link` 固定で、5b で `issue_fields.default_sync` を読む形に置き換える（plan の決定 4）。5b で置き換えた
   - `integration.apply` は、5b のイベント（`pr.opened` など）の判定に `gh` の取り込み前後の差分が要るので、`old` と `next`（取り込み前後の `GitHubState`）を返す形に変える。いまは変わったかどうかの `bool` だけを返す
   - `hh unlink` は `gh` から要素を外すので、そのあと同じ PR を `hh link` し直すと `gh` に新しく入り、`pr.opened` が発火する（5b の spec のイベントの節に書いた）
   - `synced`・`state_reason`・`Link.Fields` は型とマージだけを入れてある。5b 以降が値を書く
5. 5a の spec の項目（コマンド、取り込みの順番、自動リンク、fakegh、テスト）は、plan の「spec との対応」の表のとおり Task 1〜16 に割り当て済みで、5a で作ると決めたものに実装から漏れたものはない。残りは上の「未実施のこと」の2点だけ（検証用 Org の手の確認と、`github-live.yaml` の secret・PR）
