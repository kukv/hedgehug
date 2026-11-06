# フェーズ 7: 配布と仕上げの設計

- 日付: 2026-11-06
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「実装フェーズ」7. 配布と仕上げ、「名前と識別子」、「インデックス」、「技術選定」
- 前段: `docs/superpowers/specs/2026-10-30-phase1c-index-query-design.md`、`docs/superpowers/specs/2026-11-03-history-growth-design.md`、`docs/superpowers/specs/2026-11-03-gogit-fixed-cost-design.md`

## 位置づけ

フェーズ7は1つの spec と、小フェーズごとの plan・ブランチ・PR で作る。

| 小フェーズ | 内容 | 完了条件 |
|---|---|---|
| 7a 性能 | 1万タスクのベンチ、目標を超えた箇所の修正、結果の記録 | 下の「7a の目標」をすべて満たし、結果が本 spec にある |
| 7b 補完と文書 | `hh completion`、動的な補完、README、`docs/guide/` | 3つのシェルで静的・動的な補完が動く。下の「先送りされた文書」がすべて文書にある |
| 7c 配布 | スナップショットの検証、`v0.1.0`、導入手順の計測 | `mise` で `hedgehug` と `hh` が入る。`hh store discover` からチームのタスクを見るまでが5分以内 |

フェーズ7全体の完了条件は親設計のとおり。ただし Homebrew は配布から外す（下の「親設計の修正」）。10万コミットの目標は history-growth と gogit-fixed-cost の spec で達成して記録済みなので、測り直さない。

## 利用者の決定（2026-11-06）

- Homebrew の tap は作らない。配布は GitHub Releases のアーカイブで、`mise` の `github:` バックエンドと手動の展開を案内する（octoscope と同じ）。
- 文書は README に加えて `docs/guide/` を置き、英語と日本語の両方を書く。
- 最初のリリース `v0.1.0` をフェーズ7の中で出す。タグを打つ前に利用者の確認を取る。
- 補完は動的な補完まで作る。
- 全件の作り直しと全件の再検査の目標は 1 秒とする。

## 7a 性能

### 計測

`internal/app/storage/history/historytest` の合成の仕組みを使い、1万タスクの Store を作る。合成した Store はこれまでと同じく `$HH_BENCH_CACHE`（既定はユーザーのキャッシュディレクトリの `hh-bench`）に条件ごとに置いて使い回す。`make bench-tasks` を足す。`check` と CI には入れない。

タスクの形は、ワークスペース 1 つ、プロジェクトの下に 2 階層まで、を基本にする。親を持つタスクを 8 割、残りをルートにする。本文とコメントは 10 万コミットの合成と同じ分布にする。

### 7a の目標

| 項目 | 目標 |
|---|---|
| `hh ls`（キャッシュが効いている） | 200ms |
| `hh ls --tree`（キャッシュが効いている） | 200ms |
| `hh ls`（1 タスクのコミットの直後） | 200ms |
| `hh ls`（子孫を持つタスクの親を変えた直後） | 200ms |
| `hh ls`（インデックスのキャッシュを消した後の全件の作り直し） | 1 秒 |
| `hh ls`（`schema.yaml` を変えた直後の全件の再検査） | 1 秒 |

ベンチは usecase の層で測り、最後に実プロセス（`bin/hedgehug ls`）の起動を含む時間も測る。判定は実プロセスの値で行う。計測は mac と WSL の両方で行う。WSL は利用者に実行を頼む。

目標を超えた項目は、`-cpuprofile` で原因を調べて直す。直し方は結果を見てから plan に足す。

結果は本 spec の末尾に「計測の結果」として足す。形は history-growth の 8 節に合わせる。

## 7b 補完と文書

### `hh completion`

cobra の既定の `completion` コマンドは使わず、`completion <bash|zsh|fish>` を自前で足す。

- 既定のコマンドは root の名前で生成するので、`hh` で実行しても `hedgehug` 用のスクリプトになる。実行したときの名前（`os.Args[0]` の base）が `hh` なら、root の名前を `hh` にして生成する。それ以外は `hedgehug` で生成する。
- PowerShell は作らない（Windows は対象外）。
- `app.go` のコメント「completion comes in phase 7」を消す。`CompletionOptions.DisableDefaultCmd` は true のまま残す。

`completion` と cobra の `__complete` では `beforeRun` を動かさない。今のままだとアカウントのない環境で失敗し、Tab のたびに裏の同期を起動する。`completion` はアカウントがなくても動く。`__complete` はアカウントや Store がなければ候補なしを返す。

### 動的な補完

候補は読み取りだけで作り、失敗したら候補なしを返す。エラーを表示しない。書き込み、同期、GitHub への通信はしない。

| 対象 | 候補 | 使う場所 |
|---|---|---|
| タスク ID | 削除していないタスクの ID。説明にタイトル | `<id>` と `<project>` を取るコマンド、`--parent`、`--after`、`--before`、`--blocked-by` |
| ステータス | スキーマのステータス | `hh status` の第2引数 |
| 担当者 | Store のメンバー一覧 | `hh set` の `assignees=`・`watchers=`、`hh ls` の `assignee:` |
| `hh set` のキー | `key=` の形のフィールド名 | `hh set` の第2引数以降 |
| Store | マウント中の Store 名 | `--store`、`hh store` の引数 |
| ワークスペース | ワークスペースのキー | `--workspace` |
| スプリント、リリース | 名前 | `--sprint`、`--release`、`hh sprint`・`hh release` の引数 |
| ビュー、テンプレート | 名前 | `hh view`、`--template` |

クエリの語（`status:` など）は `assignee:` だけを候補に出し、ほかの語は作らない。フェーズ3の spec がここに送ったのは担当者の補完だけである。

候補の取得はキャッシュが効いた `hh ls` と同じ読み方にする。7a の目標の範囲で返る。

### 文書

README（英語と日本語）から「状態: セットアップ中」を消し、概要、インストール、クイックスタート、文書への案内を書く。開発の節は CONTRIBUTING に移す。インストールは次の順に書く。

1. `mise use -g github:kukv/hedgehug@latest`
2. Releases のアーカイブの手動の展開と `checksums.txt` の検証
3. ソースからの `make build`

`go install` は GUI が入らないので案内しない。

`docs/guide/`（英語）と `docs/guide/ja/`（日本語）に同じ構成で置く。

| ファイル | 内容 |
|---|---|
| `getting-started.md` | インストールから最初のタスクまで。`hh store discover` からチームのタスクを見るまでの5分の手順 |
| `sync.md` | 同期、マージ、上書きの記録、`hh undo`、`hh restore --at` |
| `stores.md` | 複数 Store、アカウント、コンテキスト、Store 間の移動 |
| `github.md` | リンク、自動化、Issue の同期 |
| `planning.md` | スプリント、リリース、レポート、テンプレート |
| `maintenance.md` | `hh doctor`、下の「先送りされた文書」、補完の入れ方 |

### 先送りされた文書

過去の spec がフェーズ7のドキュメントに送ったもの。すべて `maintenance.md` に書く。

- 手元の bare リポジトリを remote に使うときは、利用者が `git gc` を実行すること（history-growth）
- 履歴から内容を消す手順。`git filter-repo` で手で書き換え、ほかのクローンは次の同期で食い違いを見つけて止まり、`hedgehug doctor --fix` で clone し直す（cli-acceptance-fixes）

## 7c 配布

`.goreleaser.yaml` と `release.yaml` は今の設定を使う。tar.gz に `hedgehug` と `hh` が両方入ることを `goreleaser release --snapshot --clean` で確かめる。問題が見つかれば直す。

手順は次のとおり。

1. 7a と 7b がマージされた `main` で、スナップショットを確かめる。
2. 利用者の確認を取り、`v0.1.0` のタグを push する。
3. Release のアーカイブを確かめる（4 つの OS と CPU の組み合わせ、`checksums.txt`、`THIRD_PARTY_LICENSES`）。
4. `mise use github:kukv/hedgehug@0.1.0` で `hedgehug` と `hh` の両方が動くことを確かめる。
5. `reference-live-github` の検証環境で、新しい `HEDGEHUG_HOME` から `hh store discover` でチームのタスクを見るまでの時間を測る。手順は `getting-started.md` のとおりに行い、5分以内であることを確かめる。

文書と実際の手順が食い違えば、文書か実装を直す。

## テスト

- 7a: `make bench-tasks` と実プロセスの計測。合成の Store のタスク数を確かめる
- 7b: `completion` の 3 シェルの生成、`hh` と `hedgehug` の名前の切り替え、`__complete` の候補のテーブルテスト、アカウントなしで失敗しないこと、裏の同期を起動しないこと。シェルでの手の確認（bash、zsh、fish）
- 7c: 上の手順

## 親設計の修正

- 「名前と識別子」の配布を、`mise use -g github:kukv/hedgehug` と Releases のアーカイブに直す。Homebrew と Scoop を消す
- 「技術選定」の配布の Homebrew / Scoop を消す
- 「実装フェーズ」7 の Homebrew を消し、`mise` に直す
- CLI 仕様の `hh completion` から PowerShell を消す
- 全件の作り直しと全件の再検査の目標（1 秒）を、「実装フェーズ」7 の完了条件に足す

spec の承認後、plan を書く前に反映する。

## 計測の結果

2026-11-06 に `make bench-tasks`（`-benchtime 5x`）で、8553700 のコードを 2 つの環境で測った。判定は `BenchmarkLsProcess`（実プロセス）の値で行う。

| 項目 | WSL | mac |
| --- | --- | --- |
| CPU | AMD Ryzen 7 5700X 8-Core Processor（16 スレッド） | Apple M5（10 コア） |
| OS | WSL2 | macOS 27.0（APFS） |

合成した Store は、3 万コミット（初期コミットを除く。2% はマージ、5% はコメント）、1 万タスク（プロジェクト 100 件と、その下のタスク約 100 件ずつ）。

| 項目 | 目標 | WSL | mac | 判定 |
| --- | --- | --- | --- | --- |
| `hh ls`（キャッシュが効いている） | 200ms | 92ms | 100ms | 達成 |
| `hh ls --tree`（キャッシュが効いている） | 200ms | 91ms | 36ms | 達成 |
| `hh ls`（1 タスクのコミットの直後） | 200ms | 118ms | 49ms | 達成 |
| `hh ls`（子孫を持つタスクの親を変えた直後） | 200ms | 120ms | 49ms | 達成 |
| `hh ls`（全件の作り直し） | 1 秒 | 598ms | 210ms | 達成 |
| `hh ls`（`schema.yaml` を変えた直後の全件の再検査） | 1 秒 | 492ms | 167ms | 達成 |

最初の計測では、全件の作り直しが WSL で 1,033ms（mac は 354ms）、全件の再検査が 807ms（mac は 307ms）で、作り直しが目標を超えた。mac のプロファイルでは、時間のほぼすべてがタスクのファイルを 1 件ずつ読む pack の `pread` だった。そこで `rebuild` と `revalidate` の読み込みと解析を並列にした（`index.readEach`）。

mac の `hh ls`（キャッシュが効いている）が `--tree` より遅いのは、この場面を最初に測り、その最初の 1 回だけが遅いからである。修正前に単独で測ると平均は 5 回で 98ms、10 回で 66ms だった。ここから逆算すると、最初の 1 回は約 350ms、2 回目以降は約 34ms になる。
