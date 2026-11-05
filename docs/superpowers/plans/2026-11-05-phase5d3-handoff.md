# フェーズ 5d-3 の引き継ぎ（2026-11-05 時点）

5d-3（設定）は plan の Task 1〜8 を実装した。`web/e2e/settings-github.spec.ts` が fakegh を相手に、ルールの追加・回数・停止・削除、Issue の同期、設定からの取り込み、プロジェクトの停止と専用のルール、`repo_accounts` の追加と並べ替え（ドラッグを含む）、App の警告、タスクからの Issue の作成を確かめる。

## plan から変えた点

1. `gh のトークンで代わりに使う` は Device Flow のアカウントに限らず、gh でないアカウントに出す。E2E のアカウントは keychain で、App のトークンを貼った人もいる
2. `repo_accounts` の並べ替えは自前の関数でなく、pragmatic-drag-and-drop の `reorder` と `getReorderDestinationIndex` を使った
3. Go を 1 箇所直した。`automations.List` が空のとき `null` を返していたので `[]` にした（`hh automation list --json`）。server の穴埋めは要らなくなった
4. プロジェクトの設定の「すべて Store の値に戻す」は、自動化の停止と専用のルールを外さない。自動化は別の API で即時に書くため（plan の決定 10）

## 実装で分かったこと

- Svelte の変数に `set` と付けて `bind:value` すると、生成コードの `$.set` と衝突して落ちる（`source.equals is not a function`）
- 依頼のリンクは Org の設定ページなので、個人 owner の repo では開けない。GitHub のエラーページになる
- `gh のトークンで代わりに使う` の後も、警告は次の取得が成功するまで残る

## 手の確認（5a から持ち越し）

検証用 Org で次を行い、結果をここに書く。

- [ ] `hh auth login <name>` の Device Flow でログインし、8 時間の期限の前にトークンが更新されることを `hh auth status` の残り時間で確かめる
- [ ] Hedgehug の GitHub App を入れた Org で、`hh sync` が `hh-live` の PR を取り込む
- [ ] App を外した Org の repo で、repo 名付きの警告が出て同期が成功のままで、`app_missing` が立って設定の画面に警告が出る（`Resource not accessible by integration` の応答）
- [ ] リポジトリに secret `HH_LIVE_ORG`・`HH_LIVE_TOKEN` を入れ、`hh-live` に PR を 1 件以上置き、`github-live` を `workflow_dispatch` で流して通る
