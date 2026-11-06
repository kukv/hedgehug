# hedgehug

[English](README.md)

git を保存と同期の層に使う、ローカルファーストのタスク管理ツールです。CLI と GUI を1つのバイナリで提供します。コマンドは `hedgehug`、短縮形は `hh` です。

- タスクは git リポジトリ(Store)の中の Markdown ファイルです。Store の履歴が、そのままタスクの履歴になります。
- すべての操作がオフラインで済みます。remote のある Store は裏で同期し、`hh sync` ですぐに同期できます。
- 同時に編集しても、フィールドごとにマージします。同じフィールドを2人が変えたときは、あとの変更が残り、負けた側には `hh inbox` で知らせます。
- Store は共有する相手ごとに分けます。自分用に1つ、チームごとに1つ、といった形です。`hh ls` はすべての Store をまとめて表示します。
- タスクは GitHub の pull request や Issue とリンクできます。

Linux と macOS で動きます。Windows では WSL の中で使ってください。

## インストール

アーカイブと `mise` のどちらでも、`hedgehug` と `hh` の両方が入ります。`hh` は古い版の HSTR(シェル履歴の検索ツール)のコマンド名と重なります。両方を入れている環境では `hedgehug` を使ってください。

1. [mise](https://mise.jdx.dev/) で入れます。

   ```bash
   mise use -g github:kukv/hedgehug@latest
   ```

2. 手で入れます。[Releases](https://github.com/kukv/hedgehug/releases) から、お使いの OS と CPU 向けのアーカイブと `checksums.txt` をダウンロードし、検証してから展開します。

   ```bash
   sha256sum -c checksums.txt --ignore-missing
   tar xzf <アーカイブ>.tar.gz
   install hedgehug hh ~/.local/bin
   ```

3. ソースからビルドします。Go、Node、pnpm が要ります(Node と pnpm は `mise install` で、固定した版が入ります)。

   ```bash
   git clone https://github.com/kukv/hedgehug && cd hedgehug
   make build          # GUI を埋め込んだ bin/hedgehug を作る
   ```

`go install` では入れないでください。`make build` を通さずに作ったバイナリには GUI が入りません。

`hh --version` で確かめます。

## クイックスタート

```bash
hh auth add me --login <GitHub のログイン名> --token gh   # gh CLI のトークンを使う
hh store init me --prefix ME                              # 自分だけが使う Store
hh add "README を書く"
hh ls
hh done ME-1
hh ui                                                     # 同じタスクをブラウザで見る
```

チームのタスクを見るには `hh store discover` を実行します。インストールからチームのタスクを見るまでの手順は、[はじめに](docs/guide/ja/getting-started.md)にまとめています。

## ガイド

| ガイド | 内容 |
| --- | --- |
| [はじめに](docs/guide/ja/getting-started.md) | インストール、サインイン、最初のタスク、チームのタスク |
| [同期](docs/guide/ja/sync.md) | 同期、マージ、上書きされた値、`hh undo`、`hh restore --at` |
| [Store](docs/guide/ja/stores.md) | 複数の Store、アカウント、タスクを足す先のプロジェクト、Store 間の移動 |
| [GitHub](docs/guide/ja/github.md) | リンク、自動化、Issue の同期 |
| [計画](docs/guide/ja/planning.md) | スプリント、リリース、レポート、テンプレート |
| [メンテナンス](docs/guide/ja/maintenance.md) | `hh doctor`、bare リポジトリ、履歴から内容を消す手順、シェルの補完 |

コマンドのオプションは `hh <command> --help` で確かめられます。ガイドには[英語版](docs/guide/getting-started.md)もあります。

## コントリビュート

ソースからのビルド、GUI の開発サーバー、各種チェックは [CONTRIBUTING.md](CONTRIBUTING.md) にあります。セキュリティの問題の報告方法は [SECURITY.md](SECURITY.md) にあります。

## ライセンス

[MIT](LICENSE)
