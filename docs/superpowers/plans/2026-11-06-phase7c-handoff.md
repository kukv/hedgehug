# フェーズ 7c の引き継ぎ（2026-11-06 時点）

`v0.1.0` を edb7fec に打って公開した（https://github.com/kukv/hedgehug/releases/tag/v0.1.0）。フェーズ7の完了条件はすべて満たした。

## 確かめたこと

| 項目 | 結果 |
|---|---|
| スナップショット（`goreleaser release --snapshot --clean`） | 4 つのアーカイブ、`checksums.txt` が一致。darwin/arm64 を展開し、`hedgehug` と `hh` が動く、補完がそれぞれの名前で出る、`hh ui` が GUI を返す |
| Release | 4 つのアーカイブと `checksums.txt`、prerelease ではない |
| `mise use github:kukv/hedgehug@0.1.0` | 空の mise の環境で 8 秒。`hedgehug` と `hh` の両方が PATH に入り、`v0.1.0` を出す |
| 5 分の手順 | 新しい `HEDGEHUG_HOME` と Release のバイナリで、`hh auth add --token gh`、`hh store discover`、`--all`、`hh ls` の実行が合計 7 秒。実 GitHub の private な `bright-room-sandbox/hedgehug-sandbox` のタスクが見えた |

## 実装で分かったこと

- この環境の git の設定は注釈付きのタグを求めるので、`git tag -m "v0.1.0" v0.1.0 <commit>` で打つ。CONTRIBUTING の手順を `git tag -m` に直した
- darwin のバイナリは adhoc 署名である。quarantine を付けても端末からは動くので、文書に注意書きは足さない
- `git fetch -q origin` の直後でも `origin/main` が古いままのことがあった。ブランチを切る前に `git fetch origin main` の出力で確かめる

## 文書の手順の確認

Debian のコンテナで、Release の linux/arm64 のアーカイブと git-filter-repo を使い、`maintenance.md` の「履歴から内容を消す」をそのまま実行した。同期が止まり、`hh doctor --fix` が未 push の 1 コミットを戻し、次の同期が通り、remote の履歴から対象のファイルが消えた。

## 確かめていないこと

macOS のブラウザで実際にダウンロードしたときの挙動。quarantine 属性は手で付けて確かめた。
