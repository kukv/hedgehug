# フェーズ 3c（Store 間の移動、履歴の書き換え）実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `hh mv --store` でタスクを子孫ごと別の Store へ移し（移動元に転送ファイルを残す）、`hh purge --moved` で転送ファイルを掃除し、`hh purge --rewrite-history` で移動元の git 履歴からタスクを消して、他のクローンがそれを自動で取り込めるようにする。同期のマージは転送ファイルを勝たせる。完了条件の E2E（個人 Store と2つの Org の Store を別々のアカウントで同時に扱う）を通し、最後に親設計へ反映する。

**Architecture:** 転送ファイルは `internal/model` の `Forward`。index はタスクと別に持ち、古い ID は履歴から引く。移動の両側の書き込みは `internal/store/move.go`（Tx の部品と、2つの Store のロックを ULID の順に取る仕組み）に置き、`internal/ops` が集める・検査する・確認する・2コミットを書く流れを持つ（`internal/sync` のプロパティテストも store の部品を直接使うため）。マージの規則はファイル単位を `internal/merge` の `taskKind` に、ファイルをまたぐもの（転送ファイルの下のコメント・作業時間、`parent_moved`・`ref_moved`）を `merge.Repair` に足す。履歴の書き換えは `internal/gitrepo/rewrite.go` がオブジェクトを直接書く。他のクローンでの検出・作り直し・取り込みは `internal/sync/rewrite.go`、ディレクトリの入れ替えは ops がロックを外してから行う。

**Tech Stack:** Go、go-git v5（v5.19.3）、cobra、go.yaml.in/yaml/v4

**Spec:** `docs/superpowers/specs/2026-11-01-phase3-stores-accounts-design.md`（実装前に必ず全体を読む。3c の本体は「Store 間の移動」「履歴の書き換え」。加えて「共通の決まり」、「doctor の追加」の 3c の行、「テスト」の 3c とプロパティテスト、「親設計ドキュメントへの反映」）。見直しの経緯は `docs/superpowers/specs/2026-11-01-phase3-design-review.md`（決8・決17〜20、4.1、4.2）。食い違えば spec が正。spec と違う挙動になりそうなら、実装を止めてコントローラーに報告する。

## Global Constraints

- git の実行ファイルを使わない（go-git だけ）。トークンは通信するときにだけ取る。config.yaml は `Config.Update` で書く。
- spec の「共通の決まり」に従う。とくに: 処理は `internal/ops`（`cmd/hedgehug` は端末の入出力と表示だけ）、警告は `Session.Warn`（`ss.warn`）か結果の `Warnings`、エラーは `ops.Error` の Kind/Code（`newErr`）、ops は端末に聞かない（足りなければ `choiceRequired`・`confirmRequired` を返す）、2つ以上の Store のロックは Store の ULID の小さい順。
- 1つの Store に書くのは、その Store の名前の `ss.ForStore(name)` のセッションの `UpdateCommit`（author・`markWritten` がセッションの Store に結びつくため）。
- 人に見せる文言は英語（spec の日本語は意味）。この plan に英語の文面があればそれを使う。
- 終了コード: 1 は実行の失敗、2 は使い方の誤り。`KindAmbiguous` は 2（3b）。
- `--json` は値のないキーを省く。表では値のない欄を `-`。
- HTTP の入出力の型を変えたら `go generate ./internal/server/` で `api/openapi.yaml` を作り直す。
- 各 Task の最後に `go test -race ./...`（約2分）と `golangci-lint run ./...` を通す。テストと lint はフォアグラウンドで、Bash の timeout を 600000 にして回す（バックグラウンドに回さない）。
- シェルのコマンドにパス `internal/gitrepo` を書くとサンドボックスに拒否される。`internal/g*repo/` と書く（Read・Edit ツールは普通のパスでよい）。
- 既存のゴールデンが変わるときは、変わる理由がこの plan にあるものだけ `-update` で書き直し、差分を確かめる。
- Minor の指摘もその Task のうちに直す（CLAUDE.md の残タスク禁止）。
- コミットメッセージの末尾に `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`（実装したモデルの名前）を付ける。利用者が離席中と言ったらコミットせず、`git add` まで（Task ごとのパッチを残す）。

## 利用者の決定（2026-11-01、spec に反映済み）

- 古い ID からの移動の案内: index が転送ファイルごとに、タスクだった最後の版を履歴から読んで `id`・`former_ids` を覚える（転送ファイルの形は spec のまま）。
- 履歴の書き換えの取り込みの入れ替え: ロック（Store のロック・`sync.lock`）を外し、git のファイルを閉じてから行う。直前に古い方の HEAD が変わっていないかを確かめる。
- 書き換え前の履歴を持つミラーはマージせず、手元の HEAD を force push する。

## spec にない細部（この plan で決めたもの）

実装者はこのとおりに作る。利用者には plan と一緒に一覧で見せる。

| # | 細部 | 決め方 |
| --- | --- | --- |
| D1 | `DecodeTask` と転送ファイル | `DecodeTask` は `moved_to` を持つファイルに `model.ErrForward` を包んだエラーを返す（タスクとして読まない）。転送ファイルを読むのは `model.DecodeForward`、見分けるのは `model.IsForward(data)`。今の `internal/model/task_test.go` の「`moved_to` を持つファイルがタスクとして読める」テストは、この決まりのテストに置き換える |
| D2 | `store.Tasks()` と転送ファイル | 転送ファイルは飛ばす（タスクでもエラーでもない）。`store.Forwards()` を足す。`store.Task(ulid)` は転送ファイルなら `ErrForward` を包んだエラー |
| D3 | 古い ID の読み方（利用者の決定の細部） | `LastChange(HEAD, 0, パス)` でそのパスを最後に変えたコミットを探し、その最初の親での中身を読む。転送ファイルならさらに前へたどる（最大 20 回）。タスクが見つからなければ ID なし。index のキャッシュの形式を上げる（`cacheFormat` 7） |
| D4 | 転送ファイルに当たったときの案内 | `KindNotFound`、`task.moved`、Args `input`・`store`（移動先の Store 名。マウントしていなければ空）・`store_ulid`・`id`（移動先での今の ID。分からなければ空）。文面: マウント中 `API-12 moved to org-b (org-b:WEB-45)`、マウントしていない `API-12 moved to another store (ULID 01JC0…); mount that store to follow it`。終了コード1 |
| D5 | 転送ファイルに当たる入力 | 転送ファイルの ULID、または D3 で覚えた ID・古い ID。全 Store から引くとき、タスクに当たれば（移動先で古い ID が `former_ids` にあるなど）それを使い、どの Store のタスクにも当たらないときだけ案内にする。`--store` で絞ったときはその Store の中で同じ |
| D6 | `mv` の `--store` | `mv` に限り `--store` は移動先（spec の `hh mv <id> --store <dest>`。spec の「コマンドごとの対象」と 3b の D1 では `--store` は ID の絞り込みなので、`mv` だけ意味が変わる）。`mv` は自分の `--store` を持ち、全体フラグ（`ss.StoreName`）は空のまま。移動元は ID を全 Store から引いて決める（`org-a:API-12` も可）。移動元と同じ Store を渡したら `move.same_store`。`--store` と一緒に `--root`・`--before`・`--after` は使い方の誤り（終了コード2）。`--parent` と `--workspace` は移動先の中で引く |
| D7 | `--scoped` が要らないときに渡した | 無視する（エラーにしない） |
| D8 | 移動の確認の文面 | 外す参照を `unlink: API-12 blocked_by API-3` の形で1行ずつ、続けて履歴の警告 `The content before the move stays in the git history of org-a, and anyone who can read org-a can see it; use hedgehug purge --rewrite-history to remove it`、見えなくなる人がいれば `No longer visible to: github.com/bob, github.com/carol`、最後に `Move N tasks to org-b? [y/N] `。拒否は `Canceled`（終了コード1） |
| D9 | `--scoped` の端末の質問 | `The scoped schema of <project の ID> <タイトル> — [c]arry / [d]rop: `。端末でなければ `choice.required`（what `--scoped`、候補 `carry`・`drop`） |
| D10 | 外した `scoped` の部分の一覧 | `scoped: dropped status "review" (not in org-b)` の形で1行ずつ。`--json` の `scoped.dropped` は `["status review", "template bug", ...]` |
| D11 | 移動のコミットの件名と trailer | move-in: `Op{Name: "move-in", Task: <根の ULID>}`（件名は今の形で、移動先での新しい ID とタイトル）。move-out: `Op{Name: "move-out", Detail: "<移動元での ID> <タイトル>"}`（タスクファイルが転送ファイルになるため Task は使わない）。どちらも M の各タスクに `Hedgehug-Task` を1行ずつ（根が先、ほかは ULID 順。move-in の根は `Op.Task` が書くので足さない）、外した参照ごとに `Hedgehug-Unlinked: <ULID> <フィールド> <ULID>`（フィールドは `blocked_by` か `props.<key>`） |
| D12 | 再実行の判定 | M の ULID がすべて移動先に生きたタスクとしてあり、移動先の HEAD の履歴に、`Hedgehug-Op: move-in` かつ `Hedgehug-Moved-From` が移動元の ULID で、`Hedgehug-Task` が M と同じ集合のコミットがあれば、7 を飛ばして 8 だけを行う。M の一部だけが移動先にある、または move-in の記録がないのに移動先にあるときは、何も変えずにエラー（`KindConflict`、`move.exists`、`01JBZ… is already in org-b`） |
| D13 | 移動元の `numbers.yaml` | M の各タスクの `id` と `former_ids` のうち、移動元のいずれかのワークスペースの接頭辞を持つものの番号の最大を、接頭辞ごとに記録する（今の purge の `recordNumbers` と同じ「大きい方を残す」） |
| D14 | 転送ファイルの下のコメント・作業時間の同期 | `merge.Repair` に「転送ファイルの下にあるコメント・作業時間のファイルを消す」を足す。消したファイルは `Repaired` ではなく、新しい戻り値 `Dropped []merge.Overwritten`（`Field: "*"`、値はファイル全体の JSON、author はそのファイルを書いた人）で返し、同期は `Hedgehug-Overwritten` に書く。merge driver（git の CLI のマージ）は1ファイルしか見ないので、タスクファイルの規則だけが効き、コメントは次の同期の `repairHead` で消える |
| D15 | 転送ファイルとタスクファイルのマージ | 片側が転送ファイルなら転送ファイルを採る。負けた側がタスクファイルなら `Overwritten{Path, Field: "*", Value: <ファイル全文の JSON 文字列>, Author}`、削除なら `Value: "null"`。両側が転送ファイルなら `moved_to` をスカラーの LWW（今の `pick`）で決める |
| D16 | `parent_moved`・`ref_moved` の値 | `parent_moved`: `{"from": "<元の親の ULID>", "to": null}`。`ref_moved`: `{"field": "blocked_by" か "props.<key>", "target": "<転送ファイルの ULID>"}`。どちらも author はその参照を書いた人（今の `parent_deleted` と同じ決め方） |
| D17 | inbox の表示 | 移動済みのタスクへの上書きの通知に、`→ apply it to org-b's WEB-45`（マウントしていなければ `→ apply it in the store 01JC0…`）を足す。`parent_moved` は `moved to the root because its parent moved to another store`、`ref_moved` は `<field> to <ID> was removed because it moved to another store` |
| D18 | 移動済みのタスクの `inbox revert` | `KindNotFound`、`task.moved`（D4 と同じ Kind）、`<ID> moved to org-b; fix it there (org-b:WEB-45)`。何も書かない |
| D19 | `duplicate_across_stores`・`forward_target` の文面 | `duplicate_across_stores`: Where は ULID、Msg `in org-b (WEB-45) and org-c (OPS-3); remove the one not needed with hedgehug rm and hedgehug purge`。同じ文を同期の `Warnings` に `01JBZ… is in org-b (WEB-45) and org-c (OPS-3); …` で入れる。`forward_target`: Where は転送ファイルの ULID（覚えた ID があれば `API-12 (01JBZ…)`）、Msg `moved to a store that is not mounted (ULID 01JC0…)`、重さ info |
| D20 | `purge --moved` の出力 | 一覧は `01JBZ…  API-12 → org-b WEB-45`（移動先がマウントされていなければ `→ 01JC0…`、今の ID が分からなければ ID の欄を省く）。確認は `Remove N forward files? [y/N] `。結果 `Removed N forward files`。0件は `No forward files`。`--json` は `{removed: [{ulid, id, to_store, to_id}], commit}` |
| D21 | `purge --rewrite-history` の確認 | 一覧のあと `This rewrites the history of every remote of org-a and force pushes it. The clones of other members are rebuilt at their next sync.` と `Type the store name to go ahead: `。違う名前は `Canceled`（終了コード1）。端末でなければ `KindUsage`、`confirm.required`（`--yes` でも通さない） |
| D22 | 書き換えのコミットの扱い | 元のコミットの author・committer（名前・メール・時刻）を保ち、ツリーとメッセージだけを変える。マージコミットは親をすべて書き換え後のものにする。`(purged)` の置き換えは件名の `hedgehug: <操作> <ID>` の後ろ（タイトル）だけ |
| D23 | 予期しない force push の記録 | 同期で見つけたら、fetch で上書きされた `refs/remotes/origin/main` を fetch 前の値に戻し、`.hedgehug/local/force-push.json` に `{"origin": <新しい先頭>, "tracked": <手元で追っていた値>, "checked": 時刻}` を書いてエラー `the history of origin was rewritten without a record of Hedgehug; check it with hedgehug doctor`。doctor の `unexpected_force_push`（エラー）はこのファイルがあれば出す。次の同期で解消していれば（記録のある書き換えだった、origin が戻った）ファイルを消す |
| D24 | 作り直しの途中の状態の文面 | Store を開いたとき: `the rebuild of org-a after a history rewrite is unfinished (hedgehug doctor --fix finishes it)`。doctor の `rewrite_leftover`: Where は Store のパス、Msg `<残っているディレクトリ> is left from a rebuild after a history rewrite`、Hint `run hedgehug doctor --fix` |
| D25 | 取り込みの inbox | `rewrite_applied`（`the history rewrite of org-a was taken in (N unpushed changes put back)`）と `purged_change`（`unpushed changes to the purged task WEB-45 were not taken in`）を `inbox.Kind` に足す。宛先は自分 |
| D26 | GUI サーバーでの入れ替え | `ops.Session.BeforeSwap func(path string)` を足し、サーバーはそこでその Store の index と状態を捨てる（spec の「入れ替えの前にその Store の index を閉じる」） |
| D27 | E2E の `purge --rewrite-history` | バイナリでは端末の入力を渡せないので、その1手だけこのプロセスの中で `run` を呼び、`te.terminal` と標準入力に Store 名を与える（3a の `auth login` と同じ形）。ほかの手はバイナリで行う |
| D28 | devenv | 変えない（spec「3c で変更なし」。3b の E2E が Store の数を数えている） |
| D29 | `purge --rewrite-history` の再開 | 6 の直前に `.hedgehug/local/rewrite-in-progress.json`（`{before, ulids, at}`）を書き、8 の origin への push が通ったら消す。このファイルがあるときに実行したら 1〜6 を飛ばし（書き換えた HEAD は origin の子孫でないので、3 の同期と未 push の検査はできない）、HEAD に `rewrites.yaml` の記録（`at` が同じ）があれば 7 も飛ばして 8 へ進む。5〜8 は Store のロックと `sync.lock` を持ったまま行い、このファイルがある間は同期（`RunSync`）を拒む（`KindConflict`、`rewrite.in_progress`、`a history rewrite of org-a is unfinished; run hedgehug purge --rewrite-history again`） |
| D31 | undo と Store 間の移動（利用者の決定。2026-11-01） | 自分の最後の操作が `move-in`・`move-out` なら undo せず、`KindConflict`、`undo.blocked`、`a move between stores cannot be undone; move it back with hedgehug mv --store`。`rewrite-history` なら `undo.blocked`、`a history rewrite cannot be undone`。undo の対象の操作が変えたタスクが、その後に転送ファイルになっていたら、何も変えずに `task.moved`（D4 の文面と Args） |
| D32 | sprint・releases（利用者の決定。2026-11-01） | M のタスクの `sprint` と `releases` は移動先で外す。`store.Unlinks` が `Unlink{Task, Field: "sprint" か "releases", Target: <sprint・release の ULID>}` として返し、確認の一覧（`unlink: API-12 sprint <sprint の名前か ULID>`）と `Hedgehug-Unlinked` に出る。`links` はそのまま |
| D30 | 新しい操作の名前の表示 | `log` とアクティビティは `Hedgehug-Op` の `move-in`・`move-out`・`purge-moved`・`rewrite-history`・`replay` をそのまま出す（今の操作と同じ扱い）。`ops/read.go` と `server/activity.go` が知らない操作名で落ちないことをテストで確かめる |

---

### Task 1: 転送ファイルの型と、index・store での扱い

**Files:**
- Create: `internal/model/forward.go`、`internal/model/forward_test.go`
- Modify: `internal/model/task.go`（`DecodeTask` が転送ファイルを拒む）、`internal/model/task_test.go`（D1 の置き換え）、`internal/store/read.go`（`Tasks` が飛ばす、`Forwards`）、`internal/index/build.go`・`index.go`・`entry.go`（`Forwards`、キャッシュ）、`internal/index/resolve.go`（転送ファイルの案内の材料）、`DecodeTask` を呼ぶすべての場所（下記）
- Test: `internal/index/forward_test.go`（新規）、`internal/store/read_test.go`

**Interfaces:**
- Produces（`internal/model`）:

```go
// ErrForward means a task file is a forward: the task moved to another
// store and only its ulid and moved_to are left.
var ErrForward = errors.New("the task moved to another store")

// Forward is a task file left behind by a move to another store.
type Forward struct {
	ULID    ULID
	MovedTo ULID // the ulid of the store it moved to (store.yaml's ulid)
	doc     *Doc
}

// IsForward reports whether a task file is a forward: its front matter has
// the key moved_to. It does not check the rest.
func IsForward(data []byte) bool

// DecodeForward reads a forward. ulid and moved_to are required, and any
// other key makes it broken (a *DecodeError naming the key).
func DecodeForward(data []byte) (*Forward, error)

// Encode writes the forward: front matter with ulid and moved_to, no body.
func (f *Forward) Encode() ([]byte, error)
```

- `DecodeTask` は `IsForward` なら `fmt.Errorf("%w", ErrForward)` を返す（D1）。
- `internal/store`: `func (s *Store) Forwards() iter.Seq2[*model.Forward, error]`。`Tasks` は転送ファイルを飛ばす（D2）。`Task(ulid)` は転送ファイルなら `ErrForward` を包んだエラー。
- `internal/index`:

```go
// ForwardEntry is what the index keeps of a forward.
type ForwardEntry struct {
	ULID      model.ULID
	MovedTo   model.ULID
	ID        string   // the id the task had when it was last a task here; "" when not found
	FormerIDs []string
	Broken    string   // why the forward did not decode; empty when it did
}

func (ix *Index) Forwards() []*ForwardEntry          // in ULID order
func (ix *Index) Forward(ulid model.ULID) (*ForwardEntry, bool)

// ResolveForward finds the forward s names, ignoring case: its ULID, its
// id, or a former id, in that order of stages as Resolve. Only forwards are
// looked at. An input that matches more than one is *AmbiguousError.
func (ix *Index) ResolveForward(s string) (*ForwardEntry, error)
```

- index の `rebuild`・`update` は、`model.IsForward` のファイルを `ForwardEntry` にする（`tasks` には入れない）。ID は D3 で読む。`IDNumbers` は `ForwardEntry` の `ID`・`FormerIDs` も数える（番号を再び使わないため）。壊れた転送ファイルは `Broken` を持ち、index の問題（`problems`）に `broken` として今の壊れたタスクと同じ形で出す。
- `DecodeTask` の呼び出し元（`store/read.go`、`inbox/collect.go`、`doctor/doctor.go`、`index/entry.go`、`index/build.go`、`ops/undo.go`、`merge/repair.go`、`sync/sync.go`、`ops/delete.go`、`ops/edit.go`）を1つずつ見て、転送ファイルを「壊れたタスク」と取り違えないようにする。doctor の `checkFiles` は転送ファイルを `DecodeForward` で検査する。`merge/repair.go` の転送ファイルの扱いは Task 5 で足す（この Task では「タスクではない」として読み飛ばす）。

- [ ] **Step 1: 失敗するテストを書く**
  - `model`: `DecodeForward` が ulid と moved_to を読む。moved_to がない・ulid がない・ほかのキーがある → `*DecodeError`。`Encode` → `DecodeForward` で戻る。`IsForward` は moved_to の有無だけで決まる。`DecodeTask` に転送ファイル → `errors.Is(err, ErrForward)`
  - `store`: タスク2つと転送ファイル1つ → `Tasks` は2つ、`Forwards` は1つ、`Task(<転送ファイルの ULID>)` は `ErrForward`
  - `index`: タスク `API-12`（former_ids `OLD-3`）をコミット → そのファイルを転送ファイルにしてコミット → `Open` で `Tasks` に入らず、`Forwards` に `ID: API-12`・`FormerIDs: [OLD-3]`。キャッシュを消して開き直しても同じ（履歴から読む）。`Refresh` で転送ファイルになった差分を取り込んでも同じ。`Resolve("API-12")` は `store.ErrNotFound`、`ResolveForward("api-12")`・`ResolveForward("OLD-3")`・`ResolveForward(<ULID>)` は当たる。`IDNumbers` の `API` が 12 以上。moved_to が別の値に変わった転送ファイル（転送ファイル → 転送ファイルの変更）でも ID をたどれる
  - doctor: 壊れた転送ファイル（ほかのキーがある）→ `file` の検査で見つかる。正しい転送ファイル → 何も出ない
- [ ] **Step 2: 失敗を確かめる**（`go test ./internal/model/ ./internal/store/ ./internal/index/ ./internal/doctor/`）
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat: 転送ファイルの型と、index・store での扱い`）

---

### Task 2: 移動済みのタスクの案内（`task.moved`）

**Files:**
- Modify: `internal/mount/resolve.go`、`internal/ops/session.go`（`Resolve`）、`internal/ops/mount_ops.go`（`forTask`・`ResolveRef`）、`internal/ops/errors.go`
- Test: `internal/mount/resolve_test.go`、`internal/ops/moved_test.go`（新規）、`cmd/hedgehug/show_test.go`

**Interfaces:**
- Consumes: `ix.ResolveForward`、`ix.Forward`（Task 1）
- Produces:

```go
package mount

// MovedError is an input that names only a forward: the task moved to
// another store. To is the store it moved to when it is mounted (nil when
// not), and Entry the task there when found.
type MovedError struct {
	Input   string
	From    *Mounted
	Forward *index.ForwardEntry
	To      *Mounted
	Entry   *index.Entry
}

func (e *MovedError) Error() string // D4's text

// Moved looks for the task a forward of from points at, in the mounted
// store whose ulid is f.MovedTo.
func (s *Set) Moved(from *Mounted, input string, f *index.ForwardEntry) *MovedError
```

- `Set.Resolve`: タスクの当たりが0のとき、同じ Store の範囲（Store 名付きならその Store、なければ全 Store）で `ResolveForward` を引き、当たれば `*MovedError`（D5）。
- `ops`: `mountResolveError` と `Session.Resolve` が `*mount.MovedError` を `newErr(KindNotFound, "task.moved", err, "input", …, "store", …, "store_ulid", …, "id", …)` にする（D4）。1つの Store の `Session.Resolve`（Store 単位の操作と、Store が1つの構成）も、`ix.Resolve` が `ErrNotFound` のとき `ResolveForward` を見て同じエラーにする。`ResolveRef`（2つ目の ID）も同じ。
- サーバーは `problemOf` が Code と Args をそのまま写すので変えない（404 になる）。
- undo（D31。`internal/ops/undo.go`）: `UndoTarget` が選んだコミットの `Hedgehug-Op` が `move-in`・`move-out`・`rewrite-history` なら拒む。対象のコミットが変えたタスクファイルが HEAD で転送ファイルなら `task.moved`（`mount.Set.Moved` で移動先を引く）。

- [ ] **Step 1: 失敗するテストを書く**
  - undo: 最後の操作が move-in・move-out・rewrite-history のコミット（`store.Update` で `Op.Name` を付けて作る）→ D31 の `undo.blocked`、HEAD が変わらない。`set` の後にそのタスクを転送ファイルにする（別の書き手のコミット）→ undo が `task.moved`
  - mount: org-a に `API-12` の転送ファイル（moved_to が org-b の ULID）、org-b に同じ ULID のタスク `WEB-45`（former_ids に `API-12`）→ `Resolve("API-12")` は org-b の `WEB-45`（D5。タスクに当たるため）。`Resolve("org-a:API-12")` → `*MovedError`、`To` が org-b、`Entry.ID` が `WEB-45`。org-b をマウントから外す → `Resolve("API-12")` → `*MovedError`、`To` が nil。`Resolve(<ULID>)` も同じ
  - ops: `Show("org-a:API-12")` → `KindNotFound`、Code `task.moved`、文面が D4。org-b なしの構成で文面が D4 の2つ目
  - ops: `Dep("API-1", "API-12", true)`（API-12 が転送ファイル）→ `task.moved`
  - CLI: `show org-a:API-12` の終了コードが1、標準エラーが D4 の文面
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat: 移動済みのタスクを指したときに移動先を案内する`）

---

### Task 3: 移動の両側の書き込み（`internal/store/move.go`）

**Files:**
- Create: `internal/store/move.go`、`internal/store/move_test.go`
- Modify: `internal/store/tx.go`・`lock.go`（ロックを持ったまま Update する仕組み）

**Interfaces:**
- Produces:

```go
package store

// Unlink is a reference a move cuts: from Task's Field to Target.
// Field is "blocked_by" or "props.<key>".
type Unlink struct {
	Task   model.ULID
	Field  string
	Target model.ULID
}

// String is the value of a Hedgehug-Unlinked trailer: "<task> <field> <target>".
func (u Unlink) String() string

// Unlinks are the references between the tasks moved (in) and the tasks
// that stay (rest), both ways, blocked_by and task_ref properties (as sc
// types them), in the order of Task, Field, Target.
func Unlinks(sc *schema.Schema, moved, rest []*model.Task) []Unlink

// Locked holds the locks of stores in the order of their ULIDs. While it is
// held, Update and UpdateCommit of those stores in this process use it and
// do not take the lock again.
type Locked struct{ /* … */ }

// LockInOrder takes the store lock of each of ss, smallest ULID first.
func LockInOrder(ss ...*Store) (*Locked, error)

// Release lets the locks go. It may be called more than once.
func (l *Locked) Release()

// MoveIn is what the destination store writes (spec step 7).
type MoveIn struct {
	From      model.ULID       // the source store's ulid
	Tasks     []*model.Task    // the tasks as the source has them; the root first, every parent before its children
	Workspace model.ULID       // where they go in this store
	Parent    *model.ULID      // the new parent of the root; nil for the workspace's root
	Notes     map[string][]byte // the comment and worklog files of the tasks, by path (same paths here)
	Unlinked  []Unlink
	Scoped    *schema.Scoped   // to add to this store's schema.yaml, or nil
}

// MoveIn writes in in tx: each task with its workspace set to in.Workspace,
// the root under in.Parent and last among its new siblings, references
// to tasks outside in.Tasks taken off (in.Unlinked), updated set to now,
// and its id given again by this store (the old one to former_ids, which
// keeps what it had). It adds the trailers of spec step 7 and D11.
func (tx *Tx) MoveIn(in MoveIn) error

// MoveOut is what the source store writes (spec step 8).
type MoveOut struct {
	To       model.ULID   // the destination store's ulid
	Tasks    []model.ULID // the tasks moved; the root first
	Unlinked []Unlink
	Scoped   string       // the where (project:<ULID>) of the scoped entry to remove, or ""
}

// MoveOut writes out in tx: each task file becomes a forward to out.To, its
// comments and worklog entries go, the tasks that stay lose their
// references to them, numbers.yaml keeps their numbers (D13), and the
// scoped entry goes. It adds the trailers of spec step 8 and D11.
func (tx *Tx) MoveOut(out MoveOut) error
```

- ID の振り直しは、移動先の Tx に新しい `Workspace` のタスクを `PutTask` すれば今の `assignIDs` が行う（接頭辞が違うため）。接頭辞が同じ（同じ接頭辞の Store どうし）でも振り直すよう、`MoveIn` は `ID` を空にしてから置き、古い ID を `FormerIDs` の末尾へ移す。
- `Locked` の中の `Update` は `s.mu` を取り、ファイルのロックは取らない（同じプロセスで同じロックのファイルを2つ目のハンドルで取ると待ち続けるため）。
- 移動先の検査は今の `Tx.validate` に任せる（新規のタスクとして `ValidateWrite(nil, …)` になり、遷移は見ないが必須項目は見る＝spec の 3）。M の中の親・`blocked_by` は同じ Tx の中にあるので `checkRefs` が通ることをテストで確かめる。

- [ ] **Step 1: 失敗するテストを書く**
  - `Unlinks`: M の中の参照は入らない、M → 残る・残る → M の両方向、task_ref のプロパティ、順序
  - `LockInOrder(b, a)` が ULID の小さい方から取る（取る順を記録するフックで確かめる）。持っている間に別の goroutine の `a.Update` が待ち、`Release` で進む。持っている間の同じ goroutine の `a.Update` と `b.Update` は待たない
  - `MoveIn`: 親・子・孫と、子の `blocked_by` が孫、根の `blocked_by` が M の外 → 移動先に3つ、ID が移動先の接頭辞で振り直され、古い ID が `former_ids` の末尾、M の外への参照が外れ、コメントと作業時間が同じパスにあり、trailer に `Hedgehug-Moved-From`・`Hedgehug-Task` 3行・`Hedgehug-Unlinked`。同じ接頭辞の Store どうしでも新しい番号になる。移動先のスキーマにない kind → `*ValidationError`
  - `MoveOut`: タスクファイルが転送ファイルに、コメントと作業時間が消え、残るタスクの M への参照が外れ、`numbers.yaml` に移動元の接頭辞の最大、`scoped` が消え、trailer に `Hedgehug-Moved-To`
- [ ] **Step 2: 失敗を確かめる**（`go test ./internal/store/`）
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat(store): Store 間の移動の両側の書き込みと、ULID の順のロック`）

---

### Task 4: `hh mv --store`

**Files:**
- Create: `internal/ops/movestore.go`、`internal/ops/movestore_test.go`、`cmd/hedgehug/mv_store_test.go`
- Modify: `cmd/hedgehug/mv.go`、`internal/schema`（scoped の写しの検査の部品が要れば）

**Interfaces:**
- Consumes: `store.LockInOrder`・`Tx.MoveIn`・`Tx.MoveOut`・`store.Unlinks`（Task 3）、`ss.forTask`・`ss.Mount`
- Produces:

```go
// MoveStoreInput is hedgehug mv <id> --store <dest>.
type MoveStoreInput struct {
	Arg       string // the task, looked up in every store
	To        string // the destination store
	Workspace string // in To; "" chooses as spec step 2 says
	Parent    string // in To; "" for the workspace's root
	Scoped    string // "carry", "drop" or "" (D7, D9)
	Yes       bool
}

// MovePlan is what a move will do, for the confirmation (spec step 5).
type MovePlan struct {
	From, To  string
	Tasks     []*index.Entry // the root first
	Unlinked  []store.Unlink
	Hidden    []string       // accounts that can read From and not To; nil when either list is unknown
	Scoped    *ScopedPlan    // nil when no scoped entry applies
}

type ScopedPlan struct {
	Project *index.Entry
	Dropped []string // D10, filled when carried
}

type MovedTask struct {
	ULID   model.ULID
	OldID  string
	NewID  string
}

type MoveStoreResult struct {
	Plan     MovePlan
	Moved    []MovedTask
	Carried  bool
	Resumed  bool // step 7 was done before; only step 8 ran (D12)
}

// PrepareMoveStore does steps 1 to 4 and 6's checks, and returns the plan
// to confirm. A task the destination schema refuses is KindInvalid,
// move.invalid, with one line per task and violation.
func (ss *Session) PrepareMoveStore(in MoveStoreInput) (MovePlan, error)

// MoveStore does the move. Without in.Yes it is confirm.required with the
// plan in the error (Args "plan"); a needed --scoped is choice.required.
func (ss *Session) MoveStore(in MoveStoreInput) (MoveStoreResult, error)
```

- 流れは spec の 1〜9 のとおり。7 の前に `LockInOrder(移動元, 移動先)` を取り、ロックの中で index を読み直して 1〜4 をやり直す（2つの Store の index は順に読む。`Mounted.With` を入れ子にしない。`mount/refs.go` の注意と同じ）（確認の後に変わっていたら `move.changed`、`KindConflict`）。7 は `ss.ForStore(to)` の `UpdateCommit`、8 は `ss.ForStore(from)` の `UpdateCommit`。8 が失敗したら spec の文（`moved into <to>; run the same command again to finish the source`）を添える。
- 移動元と移動先のどちらかが読み取り専用なら、ロックを取る前に `store.read_only`。移動元と移動先が同じなら `KindUsage`、`move.same_store`（`use hedgehug mv --workspace`）。
- 移動先のワークスペースが2つ以上で `--workspace` がなければ `choice.required`（what `--workspace`）。
- scoped の持っていき方: 移動元の `schema.yaml` の `scoped` のうち `where: project:<M の中の project の ULID>` のもの。carry は移動先のスキーマにないステータス・テンプレートの名前と、そのステータスを指す `workflow` の遷移を外し（D10）、移動先の `Schema.Check()` を通るものを `MoveIn.Scoped` にする。
- CLI `mv`: `--store` があれば D6 のとおり `MoveStore` へ。`--yes`・`--scoped carry|drop` を足す。端末なら `confirm.required` で D8 の確認、`choice.required` の `--scoped` で D9 を聞いて再実行する（`store_init.go` の `askChoice` と同じ型）。出力は spec の 9（`Moved API-12 (<ULID>) → org-b WEB-45` を1行ずつ）、`--json` は spec の形。

- [ ] **Step 1: 失敗するテストを書く**（2つの Store のセッションのヘルパー（3b の `internal/ops/helpers_test.go`）を使う）
  - 子孫2つと、コメント・作業時間を持つ project を `MoveStore(Yes)` → 移動先に3つ（新しい ID、古い ID が former_ids）、移動元は転送ファイル3つ、コメントは移動先にだけある、2コミット（`move-in`・`move-out`）の author がそれぞれの Store のアカウント
  - 外す参照: 移動元に残るタスクの `blocked_by` が M のタスク → `Plan.Unlinked` に出て、移動後に外れている
  - 移動先のスキーマにない kind・ステータス・必須項目が空 → `move.invalid`、何もコミットされない（両方の HEAD が変わらない）、タスクごとの行
  - `Yes` なし → `confirm.required`。`--scoped` が要るのに空 → `choice.required`。`carry` で移動先にないステータスが外れ `Dropped` に出る。`drop` で移動元からだけ消える
  - 再実行: `BeforeUpdate` のフックで 8 を失敗させる → エラーに再実行の案内。同じ入力で `MoveStore` → `Resumed`、移動先のコミットは増えず、移動元が仕上がる
  - D12 の `move.exists`、`move.same_store`、読み取り専用
  - `MoveStore` を A→B と B→A で同時に（goroutine）→ どちらも終わる（待ち合わない）
  - CLI ゴールデン: 確認の文面（D8、端末）、`--yes` の出力、`--json`、`--scoped` の質問（D9）、`--store` と `--before` の併用で終了コード2
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat: hh mv --store で Store 間を移動する`）

---

### Task 5: 同期での転送ファイルの扱い（マージ、修復、inbox）

**Files:**
- Modify: `internal/merge/task.go`・`fields.go`・`merge.go`（転送ファイルの規則）、`internal/merge/repair.go`（`parent_moved`・`ref_moved`・転送ファイルの下のファイル）、`internal/sync/sync.go`（`Dropped` の記録）、`internal/inbox/collect.go`・`inbox.go`、`internal/ops/inbox.go`（revert）、`cmd/hedgehug/inbox.go`（D17）、`cmd/hedgehug/merge_driver.go`（必要なら）
- Test: `internal/merge/forward_test.go`（新規）、`internal/merge/repair_test.go`、`internal/merge/fuzz_test.go`（転送ファイルを種に足す）、`internal/sync/forward_test.go`（新規）、`internal/inbox/collect_test.go`、`internal/ops/inbox_test.go`

**Interfaces:**
- Produces:

```go
package merge

// RepairInput gains nothing; Repair reads the forwards among tasks/ itself
// and the comment and worklog files under them via Files.Paths.

// Repair's result gains Dropped: the comment and worklog files under a
// forward that it removes, each as an Overwritten with Field "*" and the
// whole file as a JSON string (D14).
func Repair(in RepairInput) (map[string][]byte, []Repaired, []Overwritten, error)
```

- `Merge` のタスクファイル（D15）: 片側でも `model.IsForward` なら、`mergeFile` の前に転送ファイルの規則で決める。両側が転送ファイルなら `moved_to` のスカラーの LWW（負けは `Overwritten{Field: "moved_to"}`）。`FuzzSymmetric` が転送ファイルでも対称であること。
- `Repair`: `parent_deleted` の条件に「親が転送ファイル」を足し、その場合の Kind を `parent_moved` にする（D16）。`blocked_by` と task_ref の値が転送ファイルの ULID なら外して `ref_moved`。転送ファイルの下の `comments/`・`worklogs/` のファイルを消して `Dropped` に入れる。`Repair` の呼び出し元（`sync.mergeCommit`、`repairHead`、`doctor.Fix`）は `Dropped` を `Hedgehug-Overwritten` に書く（`mergeCommit` の今の Overwritten と同じ形）。
- inbox: `repairItem` が `parent_moved`・`ref_moved` を通知にする。上書きの通知の Task が転送ファイルのとき、表示に D17 の移動先を添える（CLI は mount から移動先を引く。ops に `InboxView` があればそこで埋める）。`InboxRevertOf` は移動済みなら D18。

- [ ] **Step 1: 失敗するテストを書く**
  - merge: 転送ファイル × 編集したタスク → 転送ファイル、`Overwritten{Field:"*"}` に全文。転送ファイル × 削除 → 転送ファイル、`Value "null"`、Revived の通知は出ない。転送ファイル × 転送ファイル（`moved_to` が違う）→ 新しい方、負けが `moved_to`。入れ替えても同じ結果（対称）
  - repair: 親が転送ファイルの生きたタスク → ルートの末尾、`parent_moved` の値が D16。`blocked_by` が転送ファイル → 外れて `ref_moved`。転送ファイルの下のコメント → 消えて `Dropped` に全文と author
  - sync（2つのクローンのクラスター）: A が `MoveOut` 相当（`Tx.MoveOut`）を push、B が同じタスクを編集・コメント追加・子の追加・`blocked_by` を足して同期 → 収束し、B の編集とコメントが `Hedgehug-Overwritten` に、子が `parent_moved`、参照が `ref_moved`
  - inbox: 上書きの通知に移動先が付く（D17）、`parent_moved`・`ref_moved` の通知。revert → D18
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat: 同期で転送ファイルを勝たせ、負けた変更を記録する`）

---

### Task 6: `hh purge --moved` と、Store をまたぐ ULID の重複・転送先の検査

**Files:**
- Create: `internal/ops/purgemoved.go`、`internal/ops/purgemoved_test.go`、`internal/ops/duplicates.go`
- Modify: `cmd/hedgehug/rm.go`（`purge --moved`）、`internal/ops/maintain.go`（doctor に2つの検査）、`internal/ops/sync.go`（同期の後の重複の警告）、`internal/doctor/finding.go`（`checkOrder` に `duplicate_across_stores`・`forward_target`・`unexpected_force_push`・`rewrite_leftover`）
- Test: `cmd/hedgehug/purge_moved_test.go`、`internal/ops/duplicates_test.go`、`cmd/hedgehug/doctor_test.go`

**Interfaces:**
- Produces:

```go
type ForwardView struct {
	ULID    model.ULID
	ID      string // as ForwardEntry knows it
	ToStore string // the mounted store it moved to; "" when not mounted
	ToULID  model.ULID
	ToID    string // its id there, when found
}

// PurgeMovedList lists the forwards of the store (a store-unit command).
func (ss *Session) PurgeMovedList() ([]ForwardView, error)

// PurgeMoved removes them in one commit (Hedgehug-Op: purge-moved). Without
// yes it is confirm.required. None is no commit and no error.
func (ss *Session) PurgeMoved(yes bool) (removed []ForwardView, commit string, err error)
```

- `purge --moved` は Store 単位（`--store`、なければ Store が1つならそれ）。`purge <id>` と `--moved` の併用は使い方の誤り。
- `duplicate_across_stores`: マウント中の全 Store の生きたタスクの ULID を集め、2つ以上の Store にあるものを警告（D19）。doctor は Store ごとの見出しの下に、その Store が関わるものを出す。同期の後の警告は `crossStoreWarnings` と同じ場所。
- `forward_target`: その Store の転送ファイルのうち、`moved_to` がマウント中のどの Store の ULID でもないもの（info）。

- [ ] **Step 1: 失敗するテストを書く**
  - `PurgeMovedList` が D20 の内容、`PurgeMoved(false)` → `confirm.required`、`PurgeMoved(true)` → 1コミットで消え、`Hedgehug-Op: purge-moved`。0件 → コミットなし
  - CLI ゴールデン: 一覧と確認（端末）、`--yes`、0件、`--json`
  - 同じ ULID の生きたタスクを org-b と org-c に置く → doctor に `duplicate_across_stores`（警告、終了コード1）、同期の Warnings に D19 の文
  - 移動先をマウントしていない転送ファイル → `forward_target`（info、終了コード0）
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat: hh purge --moved と、Store をまたぐ重複・転送先の検査`）

---

### Task 7: 履歴の書き換えの git の部品（`internal/gitrepo`）

**Files:**
- Create: `internal/gitrepo/rewrite.go`、`internal/gitrepo/rewrite_test.go`
- Modify: `internal/gitrepo/remote.go`（force push）、`internal/gitrepo/merge.go` か `repo.go`（main を任意のコミットへ動かす、`Close`）

**Interfaces:**
- Produces:

```go
package gitrepo

// Rewrite says how each commit is made again.
type Rewrite struct {
	// Drop reports whether a path leaves every tree.
	Drop func(path string) bool
	// Message gives the new message of a commit from its whole message;
	// it returns it unchanged when there is nothing to change.
	Message func(msg string) string
}

// RewriteHistory makes again every commit reachable from head, parents
// first, with trees without the dropped paths and messages as w says;
// authors, committers and their times stay (D22). A commit whose tree
// becomes its parent's stays. It writes the objects only, moves no ref,
// and returns the new head and the number of commits made again.
func (r *Repo) RewriteHistory(head plumbing.Hash, w Rewrite) (plumbing.Hash, int, error)

// ResetMain moves refs/heads/main to h, which need not descend from it, and
// makes the work tree and the index those of h. The work tree must be
// clean (ErrWorkTreeChanged otherwise).
func (r *Repo) ResetMain(h plumbing.Hash) error

// ForcePush sends main to the remote even when the remote is not its
// ancestor, and sets refs/remotes/<remote>/main.
func (r *Repo) ForcePush(remote string) error

// SetTracking sets refs/remotes/<remote>/main to h (D23: put it back).
func (r *Repo) SetTracking(remote string, h plumbing.Hash) error

// Close lets go of the files the repository holds open (the object
// storage), so its directory can be renamed on Windows.
func (r *Repo) Close() error
```

- オブジェクトは `r.r.Storer.SetEncodedObject` で書く（`object.Tree`・`object.Commit` の `Encode`。ツリーのエントリは git の順に並べてから Encode する）。親が先の順は `Between(head, plumbing.ZeroHash)` で得る。ツリーはサブツリーごとに作り直し、空になったサブツリーは親から外す。go-git の file transport で force push が通ることは plan の作成時に確かめた（`Force: true` と `+` の refspec）。

- [ ] **Step 1: 失敗するテストを書く**
  - 3コミット（1つ目でファイル a・b、2つ目で b を変える、3つ目で c）を `Drop: b` で書き換え → 3コミットのまま、どのツリーにも b がない、2つ目のツリーは親と同じ、author・committer・時刻が元と同じ、Message が呼ばれて置き換わる。マージコミットを含む履歴でも親が書き換え後のもの
  - `ResetMain` で非子孫へ動かす → `Head`・作業ツリー・`Status` が新しいツリー。作業ツリーが汚れていれば `ErrWorkTreeChanged`
  - `ForcePush` で非子孫を bare へ → bare の main が新しい先頭、追跡も。ふつうの `Push` は今どおり `ErrRejected`
  - `Close` の後に `Open` し直して読める
- [ ] **Step 2: 失敗を確かめる**（`go test ./internal/g*repo/`）
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat(gitrepo): 履歴の書き換え、main の付け替え、force push`）

---

### Task 8: `hh purge --rewrite-history`

**Files:**
- Create: `internal/model/rewrites.go`（`rewrites.yaml`）、`internal/ops/rewrite.go`、`internal/ops/rewrite_test.go`、`cmd/hedgehug/purge_rewrite.go`、`cmd/hedgehug/purge_rewrite_test.go`
- Modify: `cmd/hedgehug/rm.go`（`purge` のフラグ）、`internal/index`（書き換えの後の `Refresh`）

**Interfaces:**
- Produces:

```go
package model

const RewritesPath = ".hedgehug/rewrites.yaml"

// Rewrite is one record of rewrites.yaml.
type Rewrite struct {
	At     time.Time  `yaml:"at"`
	Before string     `yaml:"before"` // the head of origin before the rewrite
	ULIDs  []ULID     `yaml:"ulids"`
}

func DecodeRewrites(data []byte) ([]Rewrite, error)
func EncodeRewrites(rs []Rewrite) ([]byte, error)
```

```go
package ops

type RewriteInput struct {
	Arg       string
	Typed     string // the store name typed to confirm (decision 17); "" asks for it
}

type RewritePlan struct {
	Store string
	Tasks []RewriteTarget // the tasks to remove from the history
}

type RewriteTarget struct {
	ULID  model.ULID
	ID    string
	Title string
	Live  bool // still a task here (not a forward): purged first (spec step 5)
}

type RewriteResult struct {
	Plan      RewritePlan
	Rewritten int  // commits made again
	Pushed    []string
	Warnings  []string
}

// PrepareRewrite does steps 1 to 3 (it syncs the store first).
func (ss *Session) PrepareRewrite(in RewriteInput) (RewritePlan, error)

// RewriteHistory does steps 1 to 9. An empty Typed is KindUsage
// confirm.required (with the plan); one that is not the store's name is
// Canceled.
func (ss *Session) RewriteHistory(in RewriteInput) (RewriteResult, error)
```

- 1〜3: spec のとおり。移動済みなら移動先で子孫を引き（Task 2 の `MovedError` と `mount`）、対象の Store に転送ファイルかタスクファイルがあるものに絞る。同期は `ss.SyncStore(…, true)`、その後 `ops.Unpushed`・`Status` が0であること。
- 5: 生きているタスクは今の purge の `RemoveTask` で1コミット（`Hedgehug-Op: purge`）。
- 6: `RewriteHistory` の `Drop` は対象の ULID の3つのパス、`Message` は spec の 6（件名のタイトルを `(purged)`、対象の `Hedgehug-Overwritten`・`Hedgehug-Repaired` の行を消す）。
- 7〜8: `ResetMain` → `rewrites.yaml` を足すコミット（`Hedgehug-Op: rewrite-history`）→ 全 remote に `ForcePush`。再開は D29。Store のロックの中で行い、index は `Refresh`（古いツリーのオブジェクトは残るので差分で更新できる）。
- CLI: `purge --rewrite-history <id>`。端末でなければ D21 のエラー（`--yes` でも）。端末なら一覧と D21 の文面で Store 名を読み、`RewriteHistory` に渡す。出力は消したタスクと書き換えたコミット数（`Removed N tasks from the history (M commits rewritten)`）。`--json` は `{store, tasks: [{ulid, id}], rewritten, pushed}`。

- [ ] **Step 1: 失敗するテストを書く**
  - 転送ファイルになったタスク（移動済み）と、まだ生きているタスクのそれぞれで `RewriteHistory` → origin の bare のどのコミットのツリーにも対象のパスがない、件名が `(purged)`、対象の trailer の行がない、`rewrites.yaml` の記録、ほかのタスクの履歴は残る
  - 未 push のコミットがある・未コミットの変更がある・読み取り専用 → 拒否
  - 入力された Store 名が違う → `Canceled`、空 → `confirm.required`
  - origin への force push を失敗させる → エラー、`rewrite-in-progress.json` が残り、`hh sync` は `rewrite.in_progress` で拒まれる。もう一度実行すると 1〜7 を飛ばして push だけ行い、ファイルが消える（D29）
  - ミラーへの失敗 → 警告
  - CLI: 端末でない → 終了コード2（`--yes` でも）、端末で Store 名 → 成功のゴールデン
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat: hh purge --rewrite-history で履歴からタスクを消す`）

---

### Task 9: 他のクローンでの書き換えの検出と、古い履歴のミラー

**Files:**
- Create: `internal/sync/rewrite.go`、`internal/sync/rewrite_test.go`
- Modify: `internal/sync/sync.go`（fetch の前の追跡の値、検出、ミラーの規則）、`internal/ops/maintain.go`（`unexpected_force_push`）

**Interfaces:**
- Produces:

```go
package sync

// RewriteState is what a fetch of origin found.
type RewriteState int

const (
	NoRewrite  RewriteState = iota
	Recorded                 // a rewrite with a record not yet taken in (spec)
	Unexpected               // a force push without a record (D23)
)

// Detect compares the head of origin after a fetch with tracked, the
// value refs/remotes/origin/main had before it, as spec says.
func Detect(r *gitrepo.Repo, tracked, head plumbing.Hash, applied []time.Time) (RewriteState, *model.Rewrite, error)

// ReadApplied and WriteApplied are .hedgehug/local/rewrites-applied.json.
func ReadApplied(s *store.Store) ([]time.Time, error)
func WriteApplied(s *store.Store, at []time.Time) error

// ReadForcePush is .hedgehug/local/force-push.json (D23); ok is false when
// there is none.
func ReadForcePush(s *store.Store) (ForcePush, bool, error)
```

- `Run` は origin の fetch の前に `RemoteHead("origin")` を控え、fetch の後に `Detect`。`Unexpected` なら D23（追跡を戻し、記録を書き、エラー）。`Recorded` なら Task 10 の作り直しへ（この Task では `Result.Rewrite` に記録を入れて止める。Task 10 で続ける）。`NoRewrite` で `force-push.json` があれば消す。
- ミラーの規則（利用者の決定）: ミラーの fetch の後、HEAD の `rewrites.yaml` の各 `before` について `IsAncestor(before, ミラーの先頭)` かつ `!IsAncestor(before, HEAD)` ならそのミラーをマージせず、push の段で `ForcePush` する。
- doctor: `force-push.json` があれば `unexpected_force_push`（エラー。Where は Store、Msg は D23 の文、Hint `check the history of origin; if it is right, clone the store again`）。

- [ ] **Step 1: 失敗するテストを書く**
  - クラスター（2つのクローン）で A が履歴を書き換えて force push（Task 7・8 の部品で直接）→ B の `Run` が `Recorded` を見つける（`Result.Rewrite`）
  - A が記録なしで force push → B の `Run` がエラー（D23 の文）、B の HEAD・追跡は前のまま、`force-push.json` がある、doctor が `unexpected_force_push`。origin を元に戻して `Run` → 記録が消える
  - `before` が手元で追っている origin の子孫でない記録 → `Unexpected`
  - ミラー: A が書き換え、ミラーへの force push だけ失敗 → A の次の `Run` でミラーをマージせず force push し、ミラーの先頭が A の HEAD になる、対象のパスが戻らない
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat(sync): 履歴の書き換えを見分け、古い履歴のミラーをマージしない`）

---

### Task 10: 作り直しと未 push の変更の取り込み、入れ替え、途中の状態

**Files:**
- Modify: `internal/sync/rewrite.go`（作り直しと取り込み）、`internal/sync/sync.go`（`mergeCommit` を、base を明示して3つのツリーをマージする関数に分ける）、`internal/ops/sync.go`（入れ替えと push）、`internal/ops/session.go`（`BeforeSwap`）、`internal/mount/mount.go`（途中の状態の警告と、`<path>` がない Store）、`internal/ops/maintain.go`（`rewrite_leftover` と `--fix`）、`internal/server`（D26）、`internal/inbox/inbox.go`（D25）
- Test: `internal/sync/rewrite_test.go`、`internal/ops/rewrite_take_test.go`（新規）、`internal/mount/mount_test.go`、`cmd/hedgehug/doctor_test.go`

**Interfaces:**
- Produces:

```go
package sync

// Rebuild is a clone made again after a recorded rewrite, ready to take
// the place of the store (spec steps 2 to 4 done).
type Rebuild struct {
	Dir     string        // <path>.rewrite-new
	OldHead plumbing.Hash // HEAD of the store when it was made
	Record  model.Rewrite
	Replayed int          // unpushed changes put back
	Purged   []string     // ids of purged tasks whose unpushed changes were not taken in
}

// Result gains Rebuild: set when Run made one. Run does not swap it in.

// mergeTrees is mergeCommit's merge of three commits with an explicit base
// (the same file set, rules and repair), for the replay of spec step 3.
func mergeTrees(r *gitrepo.Repo, base, ours, theirs plumbing.Hash, opt Options) (files map[string][]byte, lost []merge.Overwritten, repaired []merge.Repaired, err error)

// Swap puts b in the place of the store at dir (spec step 5): the old one
// to <dir>.before-rewrite-<time>, the new one to dir, the old one removed.
// It checks first that the HEAD of the old one is still b.OldHead (else it
// removes b.Dir and returns ErrRebuildStale).
func Swap(dir string, b *Rebuild) error

var ErrRebuildStale = errors.New("the store changed during the rebuild")

// Leftover reports what a rebuild left at dir: the directories beside it.
type Leftover struct {
	New    string   // <dir>.rewrite-new, "" when none
	Before []string // <dir>.before-rewrite-*
}
func FindLeftover(dir string) (Leftover, error)

// FinishLeftover completes a swap that stopped (spec: "途中で失敗した状態").
func FinishLeftover(dir string, l Leftover) error
```

- 作り直し（spec の 1〜4）は `Run` の中、Store のロックの中で行う。取り込みの計算は古いリポジトリで行い（両方の履歴のオブジェクトがある）、結果のファイルを `.rewrite-new` の Store に `CommitMerge`（親は新しい先頭の1つ、`Hedgehug-Op: replay`）で書く。対象の ULID のパスは戻さず、そのタスクへの未 push の変更があれば `Purged` に入れる。
- 入れ替え（利用者の決定）: `ops.RunSync` が `sync.lock` を外した後、`Result.Rebuild` があれば、`BeforeSwap(path)` を呼び、自分の持つその Store の index・`Repo` を閉じ（`Repo.Close`、mount の `Mounted` を捨てる）、`sync.Swap` する。`ErrRebuildStale` なら警告して終わる（次の同期でやり直す）。入れ替えたら、新しい `<path>` で `RunSync` をもう一度行って push し、inbox に D25 を足す。
- `mount.Open` と `ss.OpenStore`: `FindLeftover` で残りがあれば D24 の警告。`<path>` がなければ開けない Store として扱う。
- doctor: `doctorStore` が `openStoreAt` の前に `FindLeftover` を見て、あれば `rewrite_leftover`（警告）。`--fix` ならその場で `FinishLeftover`（Store を開く前なので中のロックは取らない）してから今の処理へ進む。同期の最初（`RunSync` の前）も同じく `FinishLeftover`。
- バックグラウンドの同期は `hh sync` のプロセスを起こすので同じ道を通る。GUI サーバーは `BeforeSwap` で D26。

- [ ] **Step 1: 失敗するテストを書く**
  - クラスター: A が書き換えて push。B に未 push のコミット（別のタスクの編集、消したタスクへの編集、新しいコメント）→ B の同期 → 新しい `<path>` の HEAD の親が origin の新しい先頭、別のタスクの編集とコメントは戻る、消したタスクのパスはない、inbox に `rewrite_applied`（N）と `purged_change`、`rewrites-applied.json` に `at`、origin に B の取り込みが push されている、`.before-rewrite-*` も `.rewrite-new` も残らない
  - 取り込み中の衝突（A の書き換え後に A がしたタスクの編集と、B の未 push の同じフィールドの編集）→ LWW と `Hedgehug-Overwritten`
  - `Swap` の直前に古い方へコミットを足す → `ErrRebuildStale`、`.rewrite-new` が消え、警告、次の同期で作り直して入れ替わる
  - 途中の状態: (1) `<path>` がなく `.before-rewrite-*` と `.rewrite-new` がある → `mount.Open` で開けない Store と D24 の警告、`doctor` で `rewrite_leftover`、`doctor --fix` で入れ替わって開ける。(2) `<path>` と `.before-rewrite-*` がある → `--fix` で古い方が消える。(3) 同期の最初でも (1) が直る
  - ops: `BeforeSwap` が入れ替えの前に呼ばれる
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**（Windows の rename は CI の Windows ジョブで確かめる）
- [ ] **Step 5: コミット**（`feat(sync): 履歴の書き換えを取り込み、未 push の変更を戻す`）

---

### Task 11: マージのプロパティテストに Store 間の移動を足す

**Files:**
- Modify: `internal/sync/property_test.go`・`ops_test.go`・`helpers_test.go`

**Interfaces:**
- Consumes: `store.LockInOrder`・`Tx.MoveIn`・`Tx.MoveOut`・`store.Unlinks`（Task 3）

- 2つの Store（`a`・`b`）それぞれに3つのクローンを持つクラスター（`newCluster` を Store ごとに作り、クローン i どうしを同じ利用者とみなす）。操作に「クローン i で、a のタスクを子孫ごと b へ移す」（`LockInOrder` → b の `MoveIn` → a の `MoveOut`。ops の流れと同じ順）と、b → a を足す。今の操作（編集・子の追加・コメント・`blocked_by`）は両方の Store で行う。
- 確かめること（spec）:
  - 全クローンが Store ごとに収束する
  - 書いた値が残るか記録される（今の `checkWrites`）。転送ファイルに負けた変更は `Hedgehug-Overwritten` にある
  - 不変条件: 転送ファイルを親に持つ生きたタスクがない、転送ファイルを指す `blocked_by`・task_ref がない、転送ファイルの下にコメント・作業時間がない（`checkInvariants` に足す）
  - 2つの Store では移動先が1つに決まるので、Store をまたぐ ULID の重複は起きない。重複は Task 6 の単体テストで確かめる

- [ ] **Step 1: 操作と不変条件を足す**
- [ ] **Step 2: `go test ./internal/sync/ -run TestProperty -count=1`（`-short` なしでも）で通るまで直す**。落ちたら種を固定したテストに切り出し、原因（merge・repair の規則）を Task 5 の範囲で直す
- [ ] **Step 3: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 4: コミット**（`test(sync): マージのプロパティテストに Store 間の移動を足す`）

---

### Task 12: 完了条件の E2E

**Files:**
- Create: `cmd/hedgehug/e2e_3c_test.go`

**Interfaces:**
- Consumes: `buildHedgehug`・`buildDevenv`・`initDevenv`・`devUserOf`・`(*devUser).ok`・`fail`・`run`・`okIn`・`taskOf`（既存）

- 流れ（spec「テスト」の 3c、D27・D28）:
  1. alice が `me`（personal）・`org-a`・`org-b`（alice-corp）のそれぞれに `add` と `sync` → `ls`（STORE の列に3つ）、各 Store のコミットの `Hedgehug-Author` がその Store のアカウント
  2. alice が org-a に project とその子、子に `blocked_by` で org-a に残るタスク → `mv <project> --store org-b --yes` → 外す参照が出力にある、org-b で新しい ID、org-a に転送ファイル、`show org-a:<古い ID>` が D4、`show <古い ID>` が org-b のタスク
  3. bob（org-b は read）を `fake.SetPerm` で write にし、移動の前に bob が org-a の同じ子を編集・コメント（push しない）→ alice の移動の後に bob が `sync` → bob の inbox に上書きの通知と移動先（D17）、`Hedgehug-Overwritten` が org-a の履歴にある
  4. alice が `purge --moved --store org-a --yes` → 転送ファイルが消える
  5. bob が org-a で別のタスクを編集（push しない）。alice が `purge --rewrite-history <移動したタスクの ID>`（D27。このプロセスの中で端末と Store 名を与える）→ bob が `sync` → bob の org-a が作り直され、bob の未 push の編集が戻り、消したタスクのパスは bob の org-a のどのコミットにもない、bob の inbox に `rewrite_applied`
- [ ] **Step 1: テストを書く**
- [ ] **Step 2: `go test ./cmd/hedgehug/ -run TestE2EPhase3c -count=1` で通るまで直す**（直すのは E2E が見つけた本体の不具合。テストを緩めない）
- [ ] **Step 3: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 4: コミット**（`test: 3c の完了条件の E2E`）

---

### Task 13: 親設計への反映と記録

**Files:**
- Modify: 親設計（Claude Docs https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）、`docs/superpowers/specs/2026-11-01-phase3-stores-accounts-design.md`（実装で決めたことの追記）

- コントローラー（このセッション）が行う。サブエージェントには渡さない。
- spec の「親設計ドキュメントへの反映」の箇条をすべて書く（3a・3b の分は済んでいるかを読んで確かめ、足りないものだけ足す）。加えて、この plan の利用者の決定3つと D1〜D28 のうち親設計の粒度のもの（転送ファイルの古い ID、入れ替えとロック、古い履歴のミラー、`task.moved`、予期しない force push の記録）。
- 書く前に親設計の該当する節を読み、既存の文を変えるときは変える箇所だけにする。
- 終わったら、メモ `phase3-progress.md` を更新する。

- [ ] **Step 1: 親設計の該当する節を読む**
- [ ] **Step 2: 書き足す**
- [ ] **Step 3: spec に、実装中に利用者が決めたことが残らず書いてあるかを確かめる**
- [ ] **Step 4: コミット**（`docs: 3c の決定を spec に書く`）
