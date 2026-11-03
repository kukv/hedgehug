# 長期運用での履歴の増加 残作業

- 日付: 2026-11-03
- ブランチ: `history-growth`（origin/main 660c880 から）
- spec: `docs/superpowers/specs/2026-11-03-history-growth-design.md`
- plan: `docs/superpowers/plans/2026-11-03-history-growth.md`（末尾に「plan から変えた点」）
- 作業場所を WSL から mac に移すため、ここで中断した。SDD の台帳（`.superpowers/sdd/…`）は gitignore されていて mac には届かないので、続きに要る内容をこのファイルに写した。

## 今の状態

plan の Task 1〜9 は実装とコミットが済んでいる。Task 1〜8 はタスクごとのレビューが通っている。Task 9（計測）だけ、レビューの前に止めた。

| Task | 内容 | コミット | レビュー |
| --- | --- | --- | --- |
| 1 | gitrepo: `ChangedFiles`・`Entry`・`WalkFirstParents` | 6f34df0 | 済み |
| 2 | `internal/history`（索引の本体） | dc35f75 | 済み |
| 3 | 突き合わせのプロパティテスト、Task 2 の細かい指摘の直し | f787f58, 369ef24, 4f9a39d | 済み |
| 4 | ops が索引を使う（log・show・GUI の履歴・人の一覧・移動の再開）、CLI の進みの表示 | 7879220, b2cc7f7 | 済み |
| 5 | 転送ファイルの元の ID と同期の修復の作者を索引で引く | 977d420, 0e5e940 | 済み |
| 6 | undo と restore --at を途中で止める | 1fd686f | 済み |
| 7 | clone の後・doctor --fix・サーバーで索引を先に作る、作成の中断 | 0fdaf54, 8d48a44 | 済み |
| 8 | pack を段階的にまとめる（`Consolidate`） | 685167d, f0ab160 | 済み |
| 9 | 10 万コミットでの計測、索引の読み込みの高速化、①〜③の直し、spec と plan の記録 | caad518, 7119459, 5eaa62f, 33923f6, f70525d, cc7188c | **未** |

計測の結果は spec の 8 節にある。届いていないのは「書き込みの直後の `show`」（276ms、目標 200ms）と「`undo`」（404ms、目標 200ms）の 2 つ。原因は go-git の固定費で、spec の決定 9 によりこのブランチでは直さない。

## 残作業（この順に行う）

1. **全体のテスト**: mac で `make check` を流す（tidy・lint・fmt・`-race` の全テスト・web）。WSL では、PC が重くなったため途中で止めた。その前の 1 回（7119459 の時点）は通っている。落ちたら、失敗の出力と実行条件を記録してから直す。
2. **Task 9 のレビュー**: 範囲は `f0ab160..HEAD`（このファイルのコミットは除く）。見る点:
   - spec 6.2 の計測が全部あるか（`BenchmarkShowAfterOneCommit`、索引のファイルの大きさを含む）。
   - plan にない変更 4 つ:
     - caad518: 索引の読み込みの高速化。ファイルの形式は変えていない。
     - 5eaa62f: restore --at を索引で引く。ファイルの版は 2。
     - 33923f6: `KeepDescriptors`。ディレクトリを rename・削除する経路で、先に閉じているか。Windows で効く。
     - f70525d: 追記の形式（版 3、CRC 付きの塊、`Store.AppendLocal`）。壊れた塊から立ち直れるか。
   - spec 8 節の表と、決定 9 の書き方。
3. **mac での計測のやり直し**（任意）: spec 8 節の数値は WSL（Ryzen 7 5700X）のもの。mac でも `make bench-history` を流し、値が大きく違えば 8 節に mac の列を足す。合成した Store は `$HH_BENCH_CACHE`（なければユーザーのキャッシュディレクトリの `hh-bench`）に置かれる。初回は合成に時間がかかる。
4. **ブランチ全体の最終レビュー**: 範囲は `origin/main..HEAD`。下の「細かい指摘」と「裁定」を渡し、マージ前に直すものを決めてもらう。CLAUDE.md の決まりで残タスクは残さないので、指摘はこのブランチの中で全部片付ける。
5. **親設計への反映**: spec 7 節の 3 点は rev 359 で反映済み。その後に決まった次の点は、まだ反映していない。
   - 履歴の索引に first parent を持たせ、restore --at も索引で引く。
   - 索引の保存は追記の形式。
   - go-git の `KeepDescriptors`。
   - 決定 9: go-git の固定費は別に扱う。
   - フェーズ7の性能目標の、`show`・`undo` の扱い。
6. **PR**: `history-growth` から main への PR を作る。本文には、決定の一覧、計測の結果、決定 9 を書く。
7. **マージの後**: go-git の固定費の件を、ブレストから始める（決定 9）。中身は「pack の idx の読み込み約 150ms」と「書き込みでの tree の組み立て約 200ms」。どちらも全コマンドに効き、フェーズ7の「1 万タスクで `ls` 200ms」にも関わる。

## 裁定（実装中に controller が決めたこと）

| Task | 決めたこと | 理由 | 誤っていたときの影響 |
| --- | --- | --- | --- |
| 2 | plan にコメントだけあった 4 つのテストは、実装者が書き切る | 補助の形がパッケージの中で決まるため | Task 3 のプロパティテストが拾う |
| 9 | 「`store.Init` 相当」は `store.Create` | 公開されている作成関数がそれだけ | なし |
| 4 | ブリーフの「マージコミットを出さない」は、2 クローンが同じタスクの別のフィールドを変えた場合には誤り。spec 2.3 どおりマージも出す | spec が上位 | `log <id>` にマージ行が 1 行多く出るだけ |
| 4 | `logWorkspace` は `logViewOf` が使うので残し、判定は `workspaceOf` に一本化 | 重複を避けられる | なし |
| 4 | 進みの表示の 1 秒は作り始めから数える（`progress(0,total)` を最初に呼ぶ） | spec 2.5 | 表示が早く／遅く出るだけ |
| 5 | `history.Open` 自体が失敗したら、`headAuthor`・`lastChanger` は従来の歩きに戻る | 索引はキャッシュ（spec 2.1） | 壊れた索引に気づきにくいだけ（次の Open で作り直す） |
| 8 | 1 回の統合は、選んだ pack の合計が `packCap` を超えないよう大きい方から外す | spec 5.1「1 回に 64 MiB 程度まで」 | pack が数個多く残るだけ |

利用者の決定（2026-11-03）:
- Task 9 の 3 つの未達のうち①〜③（restore --at を索引で、`KeepDescriptors`、書き込み後の保存を軽く）はこのブランチで直す。go-git の固定費は別に扱う（spec の決定 9）。
- 取り消すものがないときの `undo` は記録だけにする（spec の決定の 2）。

## 細かい指摘（最終レビューで扱いを決める）

- Task 1: `internal/gitrepo/changed.go` が `filemode.Submodule` をファイルとして扱う。Store に submodule はない。
- Task 5: `Index` の `lc`（共有する `lastChanger`）に mutex がない。`update` が `Index` を書き換えるのと同じく、1 つのゴルーチンからしか使わない前提。
- Task 8: `TestConsolidateMergesAboutPackCapAtMost` の 1 つ目のケースは、まとめた pack が `sz0+sz1` より少し小さい（ヘッダと末尾が 1 組減る）ことに頼っている。決定的だが前提が暗黙。
- Task 6: 1fd686f の `Co-Authored-By` は実装したモデルの名前（Haiku 5.5）。ほかのコミットにも Sonnet 5.5 のものがある。直すかどうかは利用者が決める。

ほかの細かい指摘は、各 Task の直しの中で片付けた（台帳で確認済み）。

## 実装を続けるときの注意

- 実装者は sonnet、丸ごと写すだけの作業は haiku を使う（opus は使わない）。終わったサブエージェントは止める。
- 全テストは仕上げに 1 回だけ流す。途中は変えたパッケージのテストに絞る。`make bench-history` は CPU を長く使うので、流す前に利用者に確かめる。
