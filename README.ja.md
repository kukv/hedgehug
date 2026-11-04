# hedgehug

[English](README.md)

git を保存と同期の層に使う、ローカルファーストのタスク管理ツールです。CLI と GUI を1つのバイナリで提供します。

Linux と macOS で動きます。Windows では WSL の中で使ってください。

> **状態: セットアップ中。** CI、lint、リリースの設定を整えた段階で、機能はまだありません。まだ使える状態ではありません。

## 開発

GUI は `web/` にあります。Vite でビルドする Svelte のアプリで、バイナリに埋め込みます。Node と pnpm が要ります。`mise install` で `mise.toml` に固定した版が入ります。

```bash
mise install
make build          # web/dist を作り、GUI を埋め込んだ bin/hedgehug を作る
bin/hedgehug ui     # 127.0.0.1 で GUI を配り、ブラウザを開く
```

GUI をリロードしながら直すときは、サーバーと Vite の開発サーバーを並べて動かします。開発サーバーは `/api` をサーバーへ中継し、トークン付きでブラウザを開きます。

```bash
bin/hedgehug ui --no-open       # 1行目: Hedgehug UI: http://127.0.0.1:<port>/#token=<token>
cd web
HEDGEHUG_UI='http://127.0.0.1:<port>/#token=<token>' pnpm dev
```

```bash
make web-check      # Biome、svelte-check、API の型、Vitest（make check にも含まれます）
make e2e            # bin/hedgehug-e2e（-tags e2e でビルド。OS の keychain ではなくファイルの keyring を使う）と bin/fakegh に対する Playwright（Chromium が無ければ入れます）
make e2e-bin        # その2つのバイナリだけを作る
```

`make web` を通さずに作ったバイナリ（`go build`、`go install`）には GUI が入りません。`hh ui` は、GUI をビルドするよう伝えるページを返します。

## コントリビュート

[CONTRIBUTING.md](CONTRIBUTING.md) を参照してください。セキュリティの問題の報告方法は [SECURITY.md](SECURITY.md) にあります。

## ライセンス

[MIT](LICENSE)
