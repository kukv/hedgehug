# force push 復旧を同じディレクトリ内で行う 設計

- 日付: 2026-11-05
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の同期の節と「パッケージ構成」
- 前段: 2026-11-03 の提案 E。`docs/superpowers/specs/2026-11-04-codebase-restructure-design.md` の決定 9 で別の spec に分けた
- 置き換える部分: `docs/superpowers/specs/2026-11-01-phase3-stores-accounts-design.md` の「作り直しと未 push の変更の取り込み」の手順 2・4・5 とその補足、`docs/superpowers/specs/2026-11-02-cli-acceptance-fixes-design.md` の `doctor --fix` の作り直しのうち入れ替えに関わる部分

## 目的

`hh doctor --fix` の force push 復旧で、Store のディレクトリを rename で差し替えるのをやめる。Windows では、開いているファイルを含むディレクトリを rename できないためである。代わりに同じディレクトリの中で直し、進みを `.hedgehug/local/rebuild.json` に記録して、途中で落ちても続きから完了できるようにする。

Windows 版の実装は含めない。この spec は、Windows で壊れやすいディレクトリの差し替えをなくすところまでを扱う。

## 決定の一覧（2026-11-03〜05 に利用者が決定）

| # | 対象 | 決定 |
| --- | --- | --- |
| 1 | 方式 | `<path>` の中で直す。元のディレクトリが無傷で残る保証はなくなる（2026-11-03） |
| 2 | 復旧の入口 | 作り直しを始めるのは `doctor --fix` だけ。同期の中では始めない（2026-11-02 の決定のまま）。途中で止まったものの続きは、今の `FinishRebuild` と同じく同期の開始時にも行う |
| 3 | 古いオブジェクト | 新しい履歴から届かないオブジェクトを消す。filter-repo で消した内容が手元の `.git` に残らないことを、clone し直す今の方式と同じく保証する（2026-11-05） |
| 4 | 戻せなかった変更 | 今と同じ `<path>.kept-<UTC 時刻>` に、書き換えの前の Store をコピーして残す（2026-11-05） |
| 5 | ロック | `sync.lock` と Store のロックを最初から適用の終わりまで持つ（2026-11-05 の設計の承認） |
| 6 | 途中の失敗 | 適用を始めたら元に戻さず、前に進めて完了させる（同上） |

## 流れ

`doctor --fix` が `force-push.json` のある Store に対して、次の順に行う。

| 段階 | すること | 落ちた・失敗したときの状態 |
| --- | --- | --- |
| 準備 | 作業ツリーに変更がないことを確かめる。origin を fetch し、追跡 ref を `tracked` に戻す。origin が `tracked` の子孫に戻っていれば `force-push.json` を消して終わる。取り込みコミットを `main` の外に作る | 手元の履歴・作業ツリーは変わらない。作ったオブジェクトは次の掃除で消える |
| 保全 | 戻せない変更（`Purged`）があれば Store を `<path>.kept-new` にコピーし、完了したら `<path>.kept-<時刻>` に改名する | 次の作り直しの開始時に `.kept-new` を消す |
| 記録 | `rebuild.json` を `phase: "apply"` で書く | 書く前なら手元は変わらない |
| 適用 | 作業ツリーを目標コミットに揃える → index と `main` を動かし、`origin` の追跡 ref を新しい先頭にする → inbox の mark と通知を書く → `index` キャッシュと `force-push.json` を消す | Store は開けない。続きから再開する |
| 掃除 | `phase: "prune"` に更新する。`refs/heads/main` と `refs/remotes/origin/*` 以外の ref を消す。届かないオブジェクトを消す | Store は警告つきで開ける。掃除だけを再試行する |
| 完了 | `rebuild.json` を消す | なし |

push はしない。次の同期に任せる（今と同じ）。読み取り専用の Store も同じ手順で直す（今と同じ）。

## `rebuild.json`

`.hedgehug/local/rebuild.json` に `store.WriteLocal` で書く。項目は次のとおり。

| 項目 | 内容 |
| --- | --- |
| `phase` | `apply` または `prune` |
| `old_head` | 書き換えの前の HEAD |
| `target` | `main` の移動先。未 push の変更がなければ origin の新しい先頭 |
| `origin` | origin の新しい先頭 |
| `mark` | inbox の mark（今の `FindSameCommit` の結果か新しい先頭） |
| `items` | inbox に足す通知。`inbox.Box.Add` は同じキーを足さないので、再開で重ならない |
| `kept` | 古い Store をコピーした先。なければ空 |

型は `replicate` に置く。`store` は `phase` だけを読む小さな関数を持つ（`store` は `inbox` より下の層なので `items` の型を知らない）。

`rebuild.json` が読めない・壊れているときは、適用の途中として扱う（開けないほうに倒す）。`doctor --fix` はその理由をエラーとして報告し、自動では直さない。

## 途中の状態の扱い

判定は親ディレクトリの走査をやめ、Store 自身の `rebuild.json` だけを見る。

| 状態 | Store を開く入口（`Session.OpenStore`・`mount.Open`・doctor の検査） | 同期・`doctor --fix` |
| --- | --- | --- |
| `apply` | 開かない。`LeftoverError`（今の `<path>` がないときと同じ扱い）で `Skipped` | 開始時に続きを行う |
| `prune` | 開いて、今と同じ警告（`Unfinished`） | 開始時に掃除を再試行する |

`store.Open` 自体は今までどおり状態を知らない。再開の処理は `store.Open` で開いて `sync.lock`、Store のロックの順に取る。

doctor の `--fix` なしの検査も `apply` の Store を開かない。半端な作業ツリーへの検査結果を出さないためである。`rewrite_leftover` の検査は 2026-11-02 に消しており、足し直さない。

## 適用の規則

- 対象のパスは `old_head` と `target` のツリーの差分。
- 各パスの内容は、`old_head` か `target` のどちらかと同じでなければならない。別の内容なら `gitrepo.ErrWorkTreeChanged` で止める。今の `checkFastForward` と同じ規則で、途中まで進んだ状態をそのまま続けられ、落ちた後の手作業の編集は上書きしない。
- 書き込みは通常のコミットと同じ `osfs` の処理を通す。Windows で開いているファイルへの書き込みが失敗する点は通常のコミットと同じで、再実行で続きから直る。
- index の書き込みと `main` の移動を作業ツリーの後に行う。ここまでの全段階は、もう一度実行しても同じ結果になる。

## 掃除の規則

- `refs/heads/main` と `refs/remotes/origin/*` から届く commit・tree・blob を 1 つの pack に書き、他の pack と loose object を消す。
- 消せないファイルがあれば `phase: "prune"` のまま warning を出す。次の `doctor --fix` か同期の開始時に再試行する。
- 他の remote の追跡 ref とタグも消す（clone し直す今の方式と同じ結果）。mirror の追跡 ref は次の fetch で作り直される。
- `<path>.kept-<時刻>` は消さない。戻せなかった変更がある場合に限り、消した内容がここに残るのは今と同じである。

## 部品の変更

### `internal/lib/gitrepo`

3 つ足す。

- HEAD に触らず、指定の親の上にコミットを作る関数。`store` のマージコミットの組み立てを共用する
- 早送りでない `main` の移動。適用の規則で書いた確認、作業ツリー、index、ref の順に行う。`FastForward` の確認の部分は共有する
- 届かないオブジェクトを消す関数。`writePack` を使う

### `internal/lib/osfs`

ディレクトリのコピーを足す。通常ファイル・ディレクトリ・シンボリックリンクを写し、`.hedgehug/local` のロックのファイルは写さない。

### `internal/app/storage/store`

`leftover.go` を `rebuild.json` の `phase` を読む処理に置き換える。`LeftoverMessage`・`LeftoverError` は残す。`RewriteNewSuffix`・`BeforeRewritePrefix`・`Leftover`・`FindLeftover`・`ReadParent`・`LeftoverIn` は消す。

`CommitMerge` のコミットの組み立てを、HEAD を動かさずに使える形に分ける。

### `internal/app/remote/replicate`

`MakeRebuild`・`Swap`・`FinishLeftoverKept`・`Discard` を、全段階を行う回復と、途中からの再開に置き換える。ロックを取る順は今と同じ。

消すもの: `newSuffix`・`beforePrefix`・`staleMarker`・`keepMarker`・`checkUnchanged`・`ErrRebuildStale`・`copyLocal`・`syncLocal`・`carryMissing`・`beforeRename`・`Rebuild.Dir`・`Rebuild.seen`。`Rebuild.Warnings` は掃除の warning のために残す。

`.hedgehug/local` は同じ場所にあるのでコピーが要らない。`skipLocal` のうちコピーのためのものも消す。

### `internal/app/usecase`・`server`

- `Session.BeforeSwap` を `BeforeRebuild` に改名して残す。GUI サーバーが開いたままの Store を手放し、作り直しの後に開き直して変更を通知するために必要である。
- `FinishRebuild` は `phase` を見て再開する。`rebuild.json` がなければ何もしない。
- `maintenance.rebuildStore` は `Rebuilt` の形（`replayed`・`dropped`・`kept`）を変えない。`--json` と GUI の表示は変わらない。
- 再開で `kept` があるときの warning は今と同じ文言で出す。

### 文言

`ErrForcePush` の `to clone it again`、`Rebuilt` の説明、`cli/doctor.go` の説明など、実態が変わる箇所を直す。`--json` の項目名と GUI の表示は変えない。`ErrForcePush` の文言が変わるので、golden と web のテストの文字列も同じ PR で確かめる。

## 文書の更新

- 親設計: 「`hh doctor --fix` で clone し直します」の文を実態に合わせて直す。パッケージ構成の `store/` の説明（途中残りの検出）を直す。
- phase3 spec: 手順 2・4・5 と補足に、本 spec に置き換えたことを書く。`rewrite_leftover` の行も同様。
- 2026-11-02 の spec の `doctor --fix` の作り直しの行に、本 spec への参照を足す。

## テスト

本番に差し込み口は足さない（決定 2026-11-05 の回 5 の方針）。途中の状態は、実際の API と Store のファイルの操作で作る。

- 回復の正常系: 未 push の変更あり・なし、`Purged` あり（`kept` にコピーされ、古い履歴が読める）、origin が子孫に戻った場合。今の `rebuild_test.go`・`maintenance/rebuild_test.go` の確かめ方を引き継ぐ。
- 再開: `apply` の各段階（一部のパスだけ書かれた、index まで、`main` まで、inbox まで）の状態から `FinishRebuild` と `doctor --fix` で再開し、中断なしと同じ結果になる。通知が重ならない。
- 手作業の編集: `apply` の途中で別の内容になったパスがあれば `ErrWorkTreeChanged` で止まり、そのファイルが残る。
- 開く入口: `apply` の Store は `OpenStore`・`mount.Open`・doctor で開かず、`prune` の Store は警告つきで開く。壊れた `rebuild.json` は `apply` と同じに扱う。
- 掃除: 消した内容の文字列が `.git` のどのファイルにも残らない。`refs/heads/main` と `refs/remotes/origin/*` だけが残る。実際の `git fsck` が通る。ファイルを消せないときは `prune` のまま残り、次で完了する。
- 同時実行: 適用の開始前に Store を開いていた書き込みはロック待ちになり、完了後のコミットが新しい履歴に載る。適用中に新しく起動したコマンドは待たず、`LeftoverError` で止まる（`rebuild.json` だけでは、適用中と落ちた後を区別できないため）。適用が終われば再実行で動く。
- GUI サーバー: 作り直しの前に Store を手放し、後で開き直して `tasks.changed` が届く（今の `TestSessionBeforeSwapReleasesTheStore` の引き継ぎ）。

対象の既存テスト: `replicate/rebuild*_test.go`、`maintenance/rebuild*_test.go`、`mount/leftover_test.go`、`cli/doctor_rewrite_test.go`、`usecasetest/edit.go` の `NoLeftover`。

## 完了の条件

- 本番コードに `osfs.Rename` によるディレクトリの差し替えがない（`<path>.kept-new` の改名を除く）。`.rewrite-new`・`.before-rewrite-` の文字列がコードにない。
- `go test -race ./...`・`make lint`・`make fmt-check`・`make tidy-check`・`make web-check`・`make e2e` が通る。
- 上の文書の更新が終わっている。
- PR は 1 つ。
