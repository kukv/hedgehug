# フェーズ 5d-3 の引き継ぎ（2026-11-05 時点）

5d-3（設定）は plan の Task 1〜8 を実装した。`web/e2e/settings-github.spec.ts` が fakegh を相手に、ルールの追加・回数・停止・削除、Issue の同期、設定からの取り込み、プロジェクトの停止と専用のルール、`repo_accounts` の追加と並べ替え（ドラッグを含む）、App の警告、タスクからの Issue の作成を確かめる。

## plan から変えた点

実装中に変えた点（決定 14、`reorder`、Go の `[]`）は plan に反映した。プロジェクトの設定の「すべて Store の値に戻す」は、自動化の停止と専用のルールを外さない（決定 10 の帰結）。

## 実装で分かったこと

- GitHub は App の入っていない private repo を 403 でなく 404 で隠す。`GET /user/installations` は GitHub App のトークン以外に 403 を返す（gh のトークンで確認）ので、トークンの種類で分けなくても判定できる
- `fakegh repo restrict` は 403、`fakegh repo uninstall` は 404 の応答を作る
- Svelte の変数に `set` と付けて `bind:value` すると、生成コードの `$.set` と衝突して落ちる（`source.equals is not a function`）
- 依頼のリンクは Org の設定ページなので、個人 owner の repo では開けない。GitHub のエラーページになる
- `gh のトークンで代わりに使う` の後も、警告は次の取得が成功するまで残る

## 手の確認（5a から持ち越し）

検証用 Org で次を行い、結果をここに書く。

- [x] `hh auth login <name>` の Device Flow でログインし、期限を過ぎた後の `hh sync` でトークンが更新されることを確かめる（2026-11-06）。結果は次のとおり
  - 実際の期限切れ（11-06 04:29）の後の 09:45 に `hh auth status` を実行すると更新が走り、期限は 8 時間後の 17:45 になった。`auth status` もトークンを使う操作なので、確認の順番としては `hh sync` より先に実行してはいけなかった
  - そこでキーチェーンの `expires_at` だけを過去に書き換え、`auth status` を挟まずに `hh sync --store live` を実行した。09:51:37 の実行で期限は 09:50:37 から 17:51:37 に延び、`refresh_expires_at` も 2027-05-06 に延びた
  - 更新はトークンを使う操作の中で、期限の 5 分前から行われる。`hh sync` が GitHub を読むのは repo ごとに 5 分に 1 回なので、前回の読み取りから 5 分以内の `hh sync` はトークンを使わず、更新も起きない（09:49 の実行で確認）
  - remote のない Store で、GitHub を読んでも変更がなかった `hh sync` が `Nothing to sync` と表示していたので、`Read GitHub for <store>: nothing new (no remote, nothing to push)` と表示するようにした
- [x] Hedgehug の GitHub App を入れた Org で、`hh sync` が `hh-live` の PR を取り込む（2026-11-05。Device Flow のトークンで `bright-room-sandbox/hh-live#1` のタイトル・ブランチ・状態が入った）
- [x] App を外した Org の repo で、repo 名付きの警告が出て同期が成功のままで、`app_missing` が立つ（2026-11-05）。実物の private repo は 403 でなく 404 を返したので判定を足した（plan の決定 17）。App を入れ直すと警告は消え、対象の repo から `hh-live` を外すと 404 で再び `app_missing` が立つことも確かめた。画面の表示は E2E で確かめた
- [x] リポジトリに secret `HH_LIVE_ORG`・`HH_LIVE_TOKEN` を入れ、`hh-live` に PR を 1 件以上置き、`github-live` を `workflow_dispatch` で流して通る（2026-11-05、run 42022483828）。`bright-room-sandbox/hh-live` は `bright-room-sandbox/structure` の Terraform で管理する
