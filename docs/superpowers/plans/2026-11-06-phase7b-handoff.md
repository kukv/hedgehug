# フェーズ 7b の引き継ぎ（2026-11-06 時点）

7b（補完と文書）は plan の Task 1〜5 を実装した。`hh completion` と動的な補完、README、`docs/guide/` の英日 12 本がある。

## 手の確認

| シェル | 環境 | 確かめたこと |
|---|---|---|
| zsh 5.9 | mac。pty で実際に Tab を押した | `show`、`status <id>`、`set <id> st`、`set <id> status=`、`ls --store` |
| bash 5.2 | Debian のコンテナ（bash-completion あり）。同じ pty | 同じ |
| fish 4.0 | 同じコンテナ。`complete -C` | 同じ |

macOS の bash 3.2 は bash-completion がなく、補完が動かない。`maintenance.md` に書いた。

## 実装で分かったこと

- スプリントとリリースの一覧はワークスペースが複数あると失敗するので、補完はワークスペースごとに読む
- `--store` が root の持続フラグなので、`__complete` でも `a.ss.StoreName` が効く
- `hh add` の後の裏の同期は GUI がなくても動く。文書はこれに合わせた（下書きの「`hh sync` まで届かない」は誤りで、実機で確かめて直した）

## 文書で確かめられなかったもの

実 GitHub が要るもの（`hh store discover` の GitHub 側、Device Flow、`hh pr`・`hh issue`・`hh start`・`hh gh import`、自動化の発火）は、親設計と `--help` に合わせて書いた。`hh store discover` の手順は 7c で実 GitHub で確かめる。`mise use` とアーカイブも 7c で確かめる。`git filter-repo` は未導入で動かしていない（同期の停止と `doctor --fix` は `reset` と force push で確かめた）。

## 次の回

7c（配布）を `main` から切ったブランチで始める。
