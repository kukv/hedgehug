# go-git の固定費 設計

- 日付: 2026-11-03
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「フェーズ7の完了条件（性能目標）」
- 前段: 長期運用での履歴の増加（`2026-11-03-history-growth-design.md`）の決定 9 で、別のブレストで扱うとした件

## 目的

history-growth の 8 節で WSL だけが「未達（固定費のため）」になっている 2 項目を、WSL でも目標の 200ms 以内にする。

- `show`（HEAD が 1 つ進んだ直後）: WSL 276ms
- `undo`（自分のコミットが 100 件前）: WSL 404ms

mac の値と、8 節のほかの項目の値は悪くしない。

## 決定の一覧（2026-11-03 に利用者が決定）

| # | 対象 | 決定 |
| --- | --- | --- |
| 1 | 目標 | WSL/Linux でも、上の 2 項目を 200ms 以内にする |
| 2 | 最初の読み込み | go-git に pack を読ませず、オブジェクトを自前で読む。go-git のフォーク（go.mod の replace）と常駐プロセスは選ばない |
| 3 | 書き込み | go-git の `Worktree.Commit` をやめ、変わったパスの上にある tree だけを組み立てる |
| 4 | tree の差分 | `Repo.DiffTrees` は go-git の `object.DiffTree` をやめ、hash が違うディレクトリにだけ降りる自前の差分にする（2026-11-03、実装中の計測で追加。3.3） |
| 5 | 計測用の合成 Store | pack を、pack の統合と同じ設定（window 10）で delta 付きで書く。キャッシュの版を上げて作り直し、8 節の項目をすべて測り直す |

## 1. 原因

2026-11-03 に mac（M5）で `BenchmarkFixedCost` をプロファイルした結果。WSL では、どちらもシステムコールとメモリの確保の比重が大きいぶん、さらに重くなる。

### 1.1 最初の読み込み（WSL 約 150ms、mac 約 41ms）

go-git は、オブジェクトを最初に 1 つ読むときに、全部の pack の `.idx` を読み込む（`ObjectStorage.requireIndex`）。60 万個では約 16 MB になり、デコードに mac で約 13ms かかる。これより重いのが、続いて pack からオブジェクトを読むときに作る「オフセット → hash」の逆引き表である（`idxfile.MemoryIndex.genOffsetHash`）。60 万件の `map[int64]plumbing.Hash` を作るので、mac で約 34ms かかる。go-git はキャッシュを引くためにこの表を使う（`decodeObjectAt`、OFS delta の基の解決）。省く設定はない。

コマンドは 1 回ごとに新しいプロセスで動くので、この費用を毎回払う。

### 1.2 書き込み（WSL 200ms 前後、mac 約 130ms。最初の読み込みを含む）

`Worktree.Commit` は、git の index（約 1 万件）から tree をすべて組み立て直す（`buildTreeHelper.BuildTree`）。合成 Store には約 5,000 個のディレクトリがあるので、5,000 個の tree を encode して hash を取る。さらに tree ごとに `HasEncodedObject` を呼び、loose オブジェクトのファイルを開こうとする（`DotGit.Object`）。1 回の commit で約 5,000 回の open になる。mac では `BuildTree` が約 51ms、そのうち `HasEncodedObject` が約 47ms だった。題名を変えるだけなら、書く必要がある tree は 3〜4 個である。

### 1.3 両方を直す必要

WSL の `undo` は、最初の読み込み（約 150ms）と書き込み（200ms 前後）を足した値に近い。片方だけを直しても約 270〜300ms で、200ms には届かない。

## 2. オブジェクトを自前で読む

### 2.1 置き場所

- `internal/gitrepo` の中に、オブジェクトの読み手を置く（loose と pack）。
- `cachedStorage`（`storage.go`）の `EncodedObject`・`HasEncodedObject`・`EncodedObjectSize` を、この読み手で答える。go-git の commit・tree の読み込み、`DiffTree`、merge、log は `Storer` のこの 3 つを通るので、呼び出し側は変わらない。今の hash で引く LRU（`cachedStorage.objects`）はそのまま前に置く。
- go-git のままにするもの: 書き込み（`SetEncodedObject`、fetch の `PackfileWriter`）、全オブジェクトをたどる処理（`IterEncodedObjects`。`doctor` の Verify、Repack、pack の統合）、push の `DeltaObject`。これらは go-git が `.idx` を全部読むが、もともと全体を読む処理か、ネットワークを待つ処理である。

### 2.2 pack を引く

- `objects/pack/` の `.idx` の一覧を、最初に引くときに読む。各 `.idx` は開いたままにし、先頭の fanout（256 × 4 バイト）だけを読んでおく。
- hash を引くときは、hash の先頭 1 バイトに当たる範囲（bucket。60 万個で平均約 2,300 件）の hash とオフセット（4 バイト）を `ReadAt` で読み、二分探索する。読んだ bucket はメモリに持ち続け、次からは読まない。first parent を最後までたどる処理は 10 万個のコミットを引くので、引くたびに読むと、go-git より遅くなった（実装中の計測で、取り消すものがない `undo` が go-git の 924ms に対し 1,251ms）。全部の bucket を読んでも約 14 MB で、go-git の逆引き表（60 万件の map）は作らない。オフセットの最上位のビットが立っていれば、64 ビットのオフセットの表から読む（2 GB を超える pack）。
- mmap は使わない。`ReadAt` だけで、Windows でも同じに動く。
- `.idx` の版 2 だけを読む（go-git も git も版 2 を書く）。版 2 でなければ、その pack は go-git に任せる（2.5）。

### 2.3 pack からオブジェクトを読む

- `.pack` も開いたままにし、オフセットから `ReadAt` で読む。オブジェクトの頭（種類と大きさ）と、続く zlib のデータを 4 KB まで 1 回で読み、足りない分だけを続けて読む。store のオブジェクトの多くは 1 回の読み込みで済む。zlib のリーダーと読み込み用の buffer は使い回す（`sync.Pool`）。
- OFS delta は、基のオブジェクトをオフセットで直接読む。hash を知る必要がないので、逆引き表は作らない。REF delta は、基を hash で引く（どの pack にあってもよい）。
- delta を戻すのは go-git の `packfile.PatchDelta` を使う。
- delta の基は、(pack, オフセット) を鍵にした小さな LRU に持つ。同じ tree の delta の鎖を何度もたどらないためである。
- 返すのは `plumbing.MemoryObject`（種類、大きさ、中身）。hash は引いたときの hash を入れる。

### 2.4 loose オブジェクト

`objects/xx/<残り 38 文字>` を開き、zlib を展開して「種類 大きさ\0」の頭を読む。pack で見つからなかったときに見る。読むオブジェクトの多くは pack にあるので、先に loose を見ると、失敗する open が 1 個ごとに増えるためである。同じ hash が両方にあっても中身は同じなので、順は正しさに関わらない。

### 2.5 見つからないとき

1. pack の一覧を読み直す（fetch で pack が増えた、統合で入れ替わった）。増えた pack を開いて、もう一度引く。消えた pack はその場では閉じず、`Repo.Close` で閉じる。ほかの goroutine がまだその pack から読んでいるかもしれないためである。
2. それでもなければ、go-git の `filesystem.Storage` に任せる。alternates や版 2 でない `.idx` も、ここで go-git が扱う。go-git は `.idx` を全部読むが、存在しない hash を引くのは fetch の交渉くらいで、ネットワークを待つ処理である。

### 2.6 閉じる

`Repo.Close` で、読み手が開いている `.idx` と `.pack` もすべて閉じる（Windows でディレクトリの名前を変えるため。今の `KeepDescriptors` と同じ扱い）。pack の統合が自分でリポジトリを閉じて開き直す流れも変えない。

## 3. 変わった tree だけを組み立てる

### 3.1 `Repo.Commit` の流れ

1. 今までどおり、index のロックを取り、`stage` で `c.Paths` の分だけ git の index を更新する（blob を書き、index の項目を足すか消す）。index は `Status` と git CLI のために保つ。
2. `Worktree.Commit` は呼ばない。HEAD の tree を基にして（HEAD がなければ空の tree）、`c.Paths` の各パスについて、index にあればその hash と mode を入れ、なければそのパスを消す。
3. 組み立て直すのは、`c.Paths` の上にあるディレクトリの tree だけである。それぞれ、今の tree を読んで項目を差し替え、git の順（ディレクトリは名前の後ろに `/` を付けて比べる）に並べて encode し、`SetEncodedObject` で書く。中身が空になったディレクトリは、親の tree から消す。
4. 第 1 親（`c.Parents` が空なら HEAD）の tree と同じで、親が 2 つ以上でも `AllowEmpty` でもなければ `ErrNothingToCommit` を返す（今の go-git と同じ判定）。
5. commit を encode して書き、HEAD（が指す枝）を進める。失敗したら、今までどおり index を元に戻す。

### 3.2 前提と、今との違い

- 前提は「index は、`c.Paths` を除けば HEAD の tree と同じ」である。今のコードも同じ前提で動いている（マージのコミットも、第 1 親を HEAD にして、違うパスを `c.Paths` に入れている）。
- 違いが出るのは、利用者が git CLI で別のパスを手で stage していた場合だけである。今は、その分も commit に入る。これからは入らない。`Repo.Commit` の説明（「`c.Paths` だけを stage して commit する」）どおりの動きになる。

### 3.3 tree の差分

2 と 3.1 を入れた後に mac で測ると、`undo` は 137ms から 71ms になったが、WSL で 200ms に届くには余裕がない。数えると、`undo` 1 回で約 1,200 個のオブジェクトを読み、そのうち約 1,100 個が `Repo.DiffTrees`（取り消すコミットとその親の差分）だった。go-git の `object.DiffTree` は、変わったディレクトリの子を比べるとき、hash が同じ兄弟のディレクトリの tree まで読む（`treeNoder.Children` が子の tree をすべて開く）。

- `Repo.DiffTrees` を、hash（と mode）が違う項目だけを見て、違うディレクトリにだけ降りる自前の差分にする。`ChangedFiles` が使っている `diffTrees`（`changed.go`）を、消えたかどうかも返す形に広げて共用する。
- 返す内容は今と同じにする: 足された・変わったファイルは `Deleted: false`、消えたファイルは `Deleted: true`、ファイルからディレクトリ（またはその逆）に変わったパスは、消えたファイルと足されたファイルとして出る。mode だけが変わったファイルは変わったファイルとして出る。並びは go-git と同じにする（ディレクトリごとに名前の順。ディレクトリ `tasks` はファイル `tasks.md` より前に来るので、パス全体の文字の順とは違う。ディレクトリが同じ名前のファイルに変わったときは、ディレクトリの中の削除が先）。
- 確かめ方: いくつかの tree の組について、go-git の `object.DiffTree` から作った結果と一致すること（足す、変える、消す、ディレクトリごと消す、ファイルとディレクトリの入れ替え、mode の変更、`prefix` での絞り込み）。

## 4. 計測用の合成 Store

- `historytest` の合成で、pack を `packfile.NewEncoder(...).Encode(hashes, 10)` で書く（pack の統合の `cfg.Pack.Window` の既定値と同じ）。今は window 0 で、delta が 1 つもない（`git verify-pack -s` で 599,098 個すべてが delta なし）。実際の Store（GitHub から fetch した pack、統合した pack）には delta がある。
- `layout` を 2 から 3 に上げる。キャッシュのディレクトリ名が変わるので、作り直しになる。
- 合成の時間は延びる（delta を探すため）。手元で一度作れば、以降はキャッシュを使う。

## 5. 確かめ方

### 5.1 正しさ（通常のテスト。CI に含める）

- 読み手: 同じリポジトリのすべてのオブジェクトを、go-git の `filesystem.Storage` で読んだものと比べ、種類と中身が一致することを確かめる。対象は次のとおり。
  - loose だけ
  - delta なしの pack
  - OFS delta の pack（`packfile.NewEncoder(w, s, false).Encode(hs, 10)`）
  - REF delta の pack（`NewEncoder(w, s, true)`）
  - 複数の pack と loose が混ざったもの
- 読み手が一覧を持った後に pack が増えた場合と、統合で pack が入れ替わった場合に、正しく読めること。
- 64 ビットのオフセット: 2 GB の pack は作らず、64 ビットのオフセットを持つ `.idx` を手で作り、引けるオフセットを確かめる。
- 存在しない hash は `plumbing.ErrObjectNotFound` になること。
- 書き込み: 次の各場合で、作った tree の hash が、同じ index から go-git の `Worktree.Commit` で作った tree の hash と一致すること。追加、変更、削除、削除でディレクトリが空になる、新しい深いディレクトリ、1 回で複数のパス、マージ（親 2 つ）、`AllowEmpty`、変更なし（`ErrNothingToCommit`）、最初のコミット。
- 既存のテストがすべて通ること。

### 5.2 速さ（`make bench-history`。CI に含めない）

- 4 の合成 Store で、history-growth の 8 節の項目をすべて測り直し、本 spec の 6 節に記録する。WSL と mac の両方で測る（WSL は利用者が流す）。
- 合格の条件: WSL と mac の両方で、`show`（1 つ進んだ直後）と `undo`（自分のコミット）が 200ms 以内、8 節のほかの目標もすべて達成。`BenchmarkFixedCost` の firstRead と write も記録する。

## 6. 計測の結果

2026-11-03 に `make bench-history`（`-benchtime 5x`）で測った。

| 項目 | WSL | mac |
| --- | --- | --- |
| CPU | AMD Ryzen 7 5700X 8-Core Processor（16 スレッド） | Apple M5（10 コア） |
| OS | Linux 6.18.54.1-microsoft-standard-WSL2（ext4 の上の Store） | macOS 27.0（APFS） |
| Go | 1.27.3 | 1.27.3 |
| コード | （WSL の計測で記入） | 8ad24ec（この後の a907023 は壊れた索引への備えだけで、速さは変わらない） |

合成した Store は layout 3（4 節）で、10 万コミット・5,000 タスク・約 60 万オブジェクトの pack 1 つ（約 181 MB。約 18 万個が delta）。history-growth の 8 節（layout 2、delta なしの約 257 MB）とは同じ条件ではない。合成は mac で約 1 分だった。

| 項目 | 目標 | WSL | mac | mac（前。history-growth 8 節） | 判定 |
| --- | --- | --- | --- | --- | --- |
| 索引の読み込みと 1 回の引き（`history.Open` と `Task`） | 50ms | | 6ms | 5ms | mac は達成 |
| `show`（索引が最新。古いタスク／新しいタスク） | 200ms | | 15ms／15ms | 50ms／52ms | mac は達成 |
| `show`（HEAD が 1 つ進んだ直後） | 200ms | | 34ms | 72ms | mac は達成 |
| `log <id>` の全件（古いタスク／新しいタスク） | 1 秒 | | 15ms／15ms | 51ms／50ms | mac は達成 |
| `undo`（自分のコミットが 100 件前） | 200ms | | 68ms | 137ms | mac は達成 |
| `restore --at`（履歴の中ほどの時刻） | 1 秒 | | 70ms | 124ms | mac は達成 |
| 普段の pack の統合（fetch 20 回分の小さい pack） | 1 秒 | | 385ms | 327ms | mac は達成 |
| `undo`（取り消すものがない。記録のみ） | — | | 752ms | 914ms | — |
| 索引の最初の作成（記録のみ） | — | | 9.1 秒 | 8.3 秒 | — |
| 索引のファイルの大きさ（記録のみ） | — | | 4,736,925 バイト | 4,736,921 バイト | — |
| 64 MiB 近くまでの統合（記録のみ） | — | | 11.5 秒 | 10.8 秒 | — |
| HEAD が 1 つ進んだ後の `history.Open`（記録のみ） | — | | 11ms | 8ms | — |
| 固定費: 開いて最初のコミットを読む（`BenchmarkFixedCost/firstRead`） | — | | 0.2ms | 41ms | — |
| 固定費: 書き込み 1 回（`BenchmarkFixedCost/write`） | — | | 63ms | 130ms | — |

測り方と読み方:

- 測り方は history-growth の 8 節と同じ。mac の値は、ほかのセッションが動いていて負荷の高い（load average 3〜4）ときのもので、同じ項目が続けて流すと 2 倍近くぶれた（索引の最初の作成が 6.7〜14.0 秒）。
- 「前」の列は delta のない合成 Store の値なので、delta を戻す費用のぶんだけ不利な比べ方になる。悪くなって見える項目（普段の統合、索引の最初の作成、64 MiB 近くまでの統合、1 つ進んだ後の `history.Open`）が読み手のせいでないかを確かめるため、同じ layout 3 の Store で読み手を外して go-git で読む場合と交互に測った。索引の最初の作成は go-git で 8.5〜14.0 秒、自前の読み手で 6.7〜11.1 秒、64 MiB 近くまでの統合は go-git で 10.8〜12.5 秒、自前の読み手で 10.6〜14.6 秒で、どちらもぶれの範囲で同じだった。統合は go-git の pack の書き手で delta を探し直す処理が大半で、読み手は関わりが小さい。
- 実装の途中で読み手が go-git より遅かった時期があった（2.2 の bucket と、2.3 の 1 回の読み込みを入れる前。取り消すものがない `undo` が go-git の 924ms に対し 1,480ms）。今は 752ms で、go-git より速い。
- 3.3 の tree の差分を入れる前は、`undo` 1 回で約 1,200 個のオブジェクトを読み、そのうち約 1,100 個が差分だった。入れた後の `undo` は、v2 の Store で 71ms から 60ms になった。

## 7. 親設計への反映

実装と計測が終わったら、親設計の「フェーズ7の完了条件（性能目標）」の、決定 9 の固定費についての記述を、本 spec の結果に置き換える。history-growth spec の 8 節の判定の欄にも、本 spec を指す一文を足す。
