# フェーズ 3b（マウントと統合ビュー、コンテキスト解決）実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** マウント中のすべての Store を1つの統合ビューとして扱えるようにし（ID の解決、一覧、Store 名付きの表示、Store をまたぐ参照の検査、コメントの本人確認、メンションの警告）、`hh add` の入れ先をコンテキスト解決で決め、`hh use` を作る。

**Architecture:** 新しい `internal/mount` が config の `stores[]` を開いた集合（`Set`）を持ち、`ops.Session` がそれを持つ（決15）。ID を受け取る操作は、全 Store から ID を引いて当たった Store の `ForStore` セッションに処理を渡し直す。一覧は `mount.Query` が Store ごとに `query.Run` を呼んで結合する。`internal/context` はコンテキスト解決と `context.json` を持ち、CLI の `hh add` だけが使う（決16。ops の作成は入れ先を受け取る）。依存の向きは `cmd/hedgehug → ops → context → mount → index, store, sync`。

**Tech Stack:** Go、go-git v5、cobra、go.yaml.in/yaml/v4

**Spec:** `docs/superpowers/specs/2026-11-01-phase3-stores-accounts-design.md`（実装前に必ず全体を読む。3b の本体は「マウントと統合ビュー」「コンテキスト解決」、加えて「共通の決まり」「doctor の追加」の 3b の行、「devenv」の 3b、「テスト」の 3b）。見直しの経緯は `docs/superpowers/specs/2026-11-01-phase3-design-review.md`（決15・決16）。食い違えば spec が正。spec と違う挙動になりそうなら、実装を止めてコントローラーに報告する。

## Global Constraints

- git の実行ファイルを使わない（go-git だけ）。トークンは通信するときにだけ取る。config.yaml は `Config.Update` で書く（知らないキーとコメントを残す）。
- spec の「共通の決まり」に従う。とくに: 処理は `internal/ops`（`cmd/hedgehug` は端末の入出力と表示だけ）、アカウント ID は `config.SameAccount`・ホストは `config.SameHost` で比べる、警告は `Session.Warn`（`ss.warn`）か結果の `Warnings`、エラーは `ops.Error` の Kind/Code（`newErr`）、ops は端末に聞かない（足りなければ `choiceRequired`・`confirmRequired` を返す）。
- 人に見せる文言は英語（spec の日本語は意味）。この plan に英語の文面があればそれを使う。
- 終了コード: 1 は実行の失敗、2 は使い方の誤り。`KindAmbiguous` は 2（Task 1 で変える。利用者の決定）。
- `--json` は値のないキーを省く。表では値のない欄を `-`。
- 各 Task の最後に `go test -race ./...`（約2分）と `golangci-lint run ./...` を通す。テストと lint はフォアグラウンドで、Bash の timeout を 600000 にして回す（バックグラウンドに回さない）。
- シェルのコマンドにパス `internal/gitrepo` を書くとサンドボックスに拒否される。`internal/g*repo/` と書く。
- 既存のゴールデンが変わるときは、変わる理由がこの plan にあるものだけ `-update` で書き直し、差分を確かめる。
- Minor の指摘もその Task のうちに直す（CLAUDE.md の残タスク禁止）。
- コミットメッセージの末尾に `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`（実装したモデルの名前）を付ける。利用者が離席中と言ったらコミットせず、`git add` まで（Task ごとのパッチを残す）。

## spec にない細部（この plan で決めたもの）

実装者はこのとおりに作る。利用者には plan と一緒に一覧で見せる。

| # | 細部 | 決め方 |
| --- | --- | --- |
| D1 | `--store x` と、別の Store 名を付けた ID（`org-b:API-1`）を一緒に渡した | 使い方の誤り（`store.conflict`、終了コード2）。`--store` と同じ Store 名なら通す |
| D2 | 保存済みビュー（`hh view`） | Store 単位（ビューは Store の中に保存されるため）。spec の表の「Store 単位」と同じ扱い |
| D3 | Store をまたぐ参照の拒否のエラー | `KindInvalid`、`ref.cross_store`（spec の表）。文面 `org-b's WEB-4 cannot be referred to (references stay inside a store)`。終了コード1 |
| D4 | コメントの本人確認のエラー | `KindConflict`、`comment.not_author`（Args: `author`）。文面 `github.com/bob's comment cannot be edited`（rm は `removed`）。author が空なら `comment.no_author`、`a comment whose author is unknown cannot be edited`（rm は `removed`） |
| D5 | メンションの警告の文面 | `bob cannot read org-a`（`Session.Warn` に出す。CLI は `warning: bob cannot read org-a`）。同じ login は1回 |
| D6 | 作成の1行 | `→ added to org-a / <親のタイトル>`（ルートならワークスペース名） |
| D7 | `hh use` の出力 | 設定: `→ org-a / <project のタイトル> is the default for new tasks`。引数なし: `org-a / <ID> <タイトル>`、なければ `-`。`--clear`: `Cleared`（設定がなければ `No changes`）。`--json`: `{store, project: {id, ulid, title}}`、なければ `{}` |
| D8 | ブランチ名の段の確認文 | `Add as a child of WEB-42 <タイトル>? [Y/n]` |
| D9 | repo の段の選択 | 候補を `1) org-a:WEB-3 <タイトル>` の形で並べ、`Project [<既定の番号>]: ` を聞く。既定がなければ `Project: ` |
| D10 | 既定の Store・ワークスペースの選択 | 番号で `1) org-a` … を並べ、`Store: `・`Workspace: ` を聞く |
| D11 | context.json の壊れの警告 | `context.json is broken, so it is read as empty: <理由>` |
| D12 | `hh use` の project が見つからない警告 | `the project of hedgehug use is not found (hedgehug use --clear removes it)` |
| D13 | 開けない Store の警告 | `cannot open org-b: <理由>`（spec の「マウント」） |
| D14 | `ls` の `STORE` 列 | 先頭の列。見出しは `STORE`、値は Store 名 |
| D15 | `ls --tree` の Store の見出し | `== org-a` の行（`group:` の見出しと同じ形）。Store の間に空行 |
| D16 | `log`（ID なし）の Store 名 | 各行の先頭に `<Store 名>  `（`ls` の列と同じく幅をそろえる） |
| D17 | `doctor` の Store の見出し | `== org-a`。`--json` は `[{"store": "org-a", "repaired": [...], "findings": [...]}]` |
| D18 | `doctor --fix` で読み取り専用の Store | 直さずに `org-a is read-only, so it is not fixed (<理由>)` を警告。検査結果は出す |
| D19 | `cross_store_ref` の文面 | Where はタスク（`TaskLabel`）、Msg は `<フィールド> points at org-b's WEB-4; references stay inside a store`。同期では `org-a: WEB-1 <フィールド> points at org-b's WEB-4; references stay inside a store` を Warnings に入れる |

---

### Task 1: `internal/mount` の土台と Store をまたぐ ID の解決

**Files:**
- Create: `internal/mount/mount.go`、`internal/mount/resolve.go`、`internal/mount/mount_test.go`、`internal/mount/resolve_test.go`
- Modify: `internal/index/index.go`（Store 名を持たせる）、`internal/ops/session.go`（`Mount`）、`cmd/hedgehug/main.go`（`KindAmbiguous` を終了コード2）、既存のテストで曖昧さの終了コード1を確かめているもの

**Interfaces:**
- Produces:

```go
package mount

// Mounted is a store of stores[] that opened.
type Mounted struct {
	Name   string            // as config.yaml names it
	Entry  config.StoreEntry
	Store  *store.Store
	ULID   model.ULID        // store.yaml's ulid
	Access sync.Access       // read-only state; the zero value when access.json is missing or does not read
	// index is opened by Index on first use and kept.
}

// Index opens the index of the store on first use and keeps it. The index
// knows the store's name (index.SetStoreName).
func (m *Mounted) Index() (*index.Index, error)

// Skipped is a store of stores[] that did not open.
type Skipped struct {
	Name string
	Err  error
}

// Set is the stores of config.yaml, in its order.
type Set struct {
	Stores  []*Mounted
	Skipped []Skipped
}

// Options is how Open opens a store.
type Options struct {
	Now func() time.Time // given to store.SetNow; nil keeps the store's clock
}

// Open opens every store of c.Stores in order. A store that does not open
// goes to Skipped with why; Open itself fails only for what is not about
// one store.
func Open(c *config.Config, opt Options) (*Set, error)

// Get is the mounted store named name (case counts).
func (s *Set) Get(name string) (*Mounted, bool)

// SplitStore reads "<store>:<rest>": when the part before the first ":" is
// the name of a mounted store, it returns that store and rest. Otherwise
// ok is false and the input is used whole.
func (s *Set) SplitStore(arg string) (m *Mounted, rest string, ok bool)

// Hit is a task found in a store.
type Hit struct {
	Store *Mounted
	Entry *index.Entry
}

// AmbiguousError is an input that matches tasks in more than one store.
// Candidates are "<store>:<id>" (the ULID when the task has no id), in the
// order of the stores.
type AmbiguousError struct {
	Input      string
	Candidates []Hit
}

// Resolve finds the task arg names in every store, or in the store its
// "<store>:" names. A match in one store is that task; none is
// store.ErrNotFound; in several stores is *AmbiguousError. An input that
// matches more than one task within one store is that index's
// *index.AmbiguousError, as now.
func (s *Set) Resolve(arg string) (Hit, error)
```

- `index`: `func (ix *Index) SetStoreName(name string)`、`func (ix *Index) StoreName() string`（Refresh しても保つ）。
- `ops`: `func (ss *Session) Mount() (*mount.Set, error)`: config を読んで `mount.Open` を1回だけ行い、結果を持つ（mutex で守る。失敗は持たない）。`Skipped` があれば最初の1回だけ `cannot open <name>: <理由>` を `ss.warn` に出す（D13）。`ForStore` で作ったセッションは同じ `*mount.Set` を共有する（`tokens` と同じ形で親から渡す）。
- `--store` で名指しした Store が `Skipped` にあれば、その Store を使う操作はエラー（`store.open`、`KindInvalid`、`cannot open org-b: <理由>`）。今の `Session.OpenStore` が開くときのエラーと同じ文にする。
- 転送ファイル（移動済みのタスク）への当たりと「移動の案内」は 3c で足す。3b の `Resolve` は転送ファイルを知らない。
- `cmd/hedgehug/main.go`: `ops.KindOf(err) == ops.KindAmbiguous` も終了コード2。

- [ ] **Step 1: 失敗するテストを書く**（`internal/mount`。Store の作り方は `internal/ops/helpers_test.go` と `internal/store` のテストのヘルパーを見て、mount の中に小さなヘルパーを作る）
  - 3つの Store（`me`・`org-a`・`org-b`）を config に書いて `Open` → `Stores` が config の順、各 `ULID` が store.yaml の値
  - パスがない Store と store.yaml が壊れた Store → `Skipped` に名前と理由、残りは開く
  - `Index()` を2回呼ぶと同じ `*index.Index`、`StoreName()` が Store 名
  - `SplitStore("org-a:API-12")` → org-a と `API-12`。`SplitStore("nope:API-12")`（マウントされていない名前）→ ok が false。`SplitStore("Org-A:API-12")` → ok が false（Store 名は大文字小文字を区別する）
  - `Resolve`: org-a だけにある `API-12` → org-a のタスク。`api-12`（小文字）でも当たる。どこにもない → `errors.Is(err, store.ErrNotFound)`。org-a と org-b の両方に `API-12` → `*AmbiguousError`、候補が `org-a`・`org-b` の順。`org-b:API-12` → org-b のタスク。ULID の入力も同じ規則。org-a の中で2つに当たる古い ID → `*index.AmbiguousError`
  - ops: `StoreName = "org-b"`（パスがない Store）で `Show` → `store.open` のエラー。`StoreName` なしなら org-b の警告だけで、ほかの Store は使える
  - ops: `Mount()` を2回呼んでも `mount.Open` は1回（開いた Set が同じポインタ）。Skipped の警告は1回だけ。`ForStore` のセッションの `Mount()` が同じ Set
  - CLI: 1つの Store の中で曖昧な ID（`TestShowAmbiguous` など既存のテスト）の終了コードが2
- [ ] **Step 2: 失敗を確かめる**（`go test ./internal/mount/ ./internal/ops/ ./cmd/hedgehug/`）
- [ ] **Step 3: 実装する**（`Mounted.Access` は `sync.ReadAccess`。読めなければゼロ値にし、書き込みの拒否は今の `checkWritable` が `access.json` を読み直して行うので、ここでは判定に使わない）
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat(mount): マウント中の Store の集合と、Store をまたぐ ID の解決`）

---

### Task 2: ID を受け取る操作を全 Store から引く

**Files:**
- Modify: `internal/ops/session.go`、`internal/ops/task.go`、`internal/ops/edit.go`、`internal/ops/move.go`、`internal/ops/delete.go`、`internal/ops/comment.go`、`internal/ops/read.go`（`Show`、`Log` の ID あり）、`internal/ops/assign.go`（`ResolveULID` の差し替え先）、`internal/ops/readonly.go`、対応する `cmd/hedgehug/*.go`（`BeginEdit` などのセッションを持ち回るところ）
- Test: `internal/ops/mount_ops_test.go`（新規）

**Interfaces:**
- Consumes: `(*Session).Mount`、`(*mount.Set).Resolve`・`SplitStore`、`*mount.AmbiguousError`（Task 1）
- Produces:

```go
// forTask is the session that works on the task arg names, and arg as that
// session reads it. With StoreName set it is ss itself; a "<store>:" naming
// another store is a usage error (store.conflict). Otherwise the task is
// looked up in every store (mount.Set.Resolve) and the session is
// ss.ForStore(<its store>), with the "<store>:" taken off arg. A task in
// several stores is KindAmbiguous, task.ambiguous, with the candidates as
// "<store>:<id> (<ULID>) <title>" lines and Args "candidates".
// Not found is task.not_found, as Resolve says now.
func (ss *Session) forTask(arg string) (*Session, string, error)

// ResolveRef resolves a second id (a blocker, a parent, a task_ref value)
// within the store of ix. An input not in the store that some other mounted
// store has is refused: KindInvalid, ref.cross_store, Args "store", "id".
// A "<store>:" naming this store is taken off; naming another store is
// ref.cross_store too.
func (ss *Session) ResolveRef(ix *index.Index, arg string) (*index.Entry, error)
```

- 各操作の頭で次の形にする（`Set` の例。`SetStatus`・`Dep`・`Watch`・`Edit`・`BeginEdit`・`Move`・`Remove`（rm）・`Restore`・`RestoreAt`・`Show`・`Log`（ID あり）・`TaskLog` を使う側・`BeginComment`・`CommentAdd`・`CommentEdit`・`CommentRemove`・`OpenComment` に同じことをする）:

```go
func (ss *Session) Set(arg string, assigns []Assign) (TaskResult, error) {
	ts, arg, err := ss.forTask(arg)
	if err != nil {
		return TaskResult{}, err
	}
	if ts != ss {
		return ts.Set(arg, assigns)
	}
	// 今の処理（OpenForWrite → Resolve → …）
}
```

- コメントの ID（コメントの ULID）は、`StoreName` がなければ全 Store のコメントから探す（`forComment(arg)`。1つの Store で当たればその `ForStore`、0なら今の `comment.not_found`、2つ以上（ULID の重複）なら `KindAmbiguous`、`comment.ambiguous`）。
- 2段に分かれた操作（`BeginEdit` → `PrepareEdit`/`WriteEdit`、`BeginComment` → `PrepareComment`/`WriteComment`、`OpenComment` → `PrepareCommentEdit`/`WriteCommentEdit`）は、`EditSession`・`CommentSession`・`CommentEditSession` に、始めたセッション（非公開のフィールド `ss *Session`）を持たせ、後の段はそれで書く（呼んだ側のセッションが `--store` なしでも、当たった Store のアカウントで書くため）。
- 2つ目の ID（spec の「ID の解決」の一覧: `dep` の blocker、`mv --parent`/`--before`/`--after`、`set parent=`、`task_ref` のプロパティ、`add --parent`、`set blocked_by=`、`edit` で書いた参照）は `ResolveRef` で引く。`ResolveULID` は `ResolveRef` を使うように変える（`ApplyAssign`・`CheckEdit` の参照はここを通る）。
- 読み取り専用の拒否の順（利用者の決定）: `forTask` は読むだけ。渡し直した先の操作が今の `OpenForWrite` で拒否するので、「Store が決まった直後、ほかの検査より前」になる。`--store` か `org-a:` で指したときは今どおり ID を引く前に拒否する（`forTask` は `StoreName` があれば引かない。`org-a:` のときは `ForStore("org-a")` に渡すだけで引かない）。
- 「Store 単位」の操作（`workspace`・`schema`・`undo`・`purge`・`view`）と `Add` はこの Task では変えない（今の `Select`: Store が1つならそれ、2つ以上なら `--store` を求めるエラー）。

- [ ] **Step 1: 失敗するテストを書く**（2つの Store `org-a`（接頭辞 API）・`org-b`（接頭辞 WEB）を持つセッションのヘルパーを `internal/ops/helpers_test.go` に足す）
  - `--store` なしで `Set("WEB-1", ...)` → org-b に書かれ、コミットの author が org-b のアカウント
  - `Show("API-1")` → org-a のタスク。`Show("org-b:WEB-1")` → org-b
  - 両方に `X-1` がある構成で `Show("X-1")` → `KindAmbiguous`、メッセージに `org-a:X-1`・`org-b:X-1`
  - `StoreName = "org-a"` で `Show("WEB-1")` → `task.not_found`（org-b にあっても引かない）。`StoreName = "org-a"` で `Show("org-b:WEB-1")` → `store.conflict`、`KindUsage`
  - org-b を読み取り専用（`access.json`）にして `Set("WEB-1", ...)` → `store.read_only`。`Set("WEB-999", ...)`（ない ID）→ `task.not_found`（ID を引いてから拒否するため）。`StoreName = "org-b"` で `Set("WEB-999", ...)` → `store.read_only`（引く前に拒否）
  - `Dep("API-1", "WEB-1", true)` → `ref.cross_store`、文面が D3。`Dep("API-1", "org-b:WEB-1", true)` も同じ。`Dep("API-1", "org-a:API-2", true)` は通る
  - `Set("API-1", parent=WEB-1)`、task_ref のプロパティに `WEB-1`、`Move` の `--parent WEB-1`・`--before WEB-1`、`Edit` で `blocked_by: [<WEB-1 の ULID>]` を書く → どれも `ref.cross_store`
  - `BeginEdit("WEB-1")` → `WriteEdit` が org-b に、org-b のアカウントで書く。`BeginComment`/`WriteComment`、`OpenComment`/`WriteCommentEdit` も同じ
  - `CommentEdit("<org-b のコメントの ULID>", ...)` が `--store` なしで通る
  - Store が1つの構成で、既存の ops のテストがすべて今のまま通る
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat(ops): ID を受け取る操作を全 Store から引き、参照を Store の中に閉じる`）

---

### Task 3: Store をまたぐ一覧（`ls`・`ls --tree`・`show`・`log`・`store:`）

**Files:**
- Create: `internal/mount/query.go`、`internal/mount/query_test.go`
- Modify: `internal/query/eval.go`・`internal/query/sort.go`・`internal/query/parse.go`（下記）、`internal/ops/read.go`（`List`・`Tree`・`Log`）、`internal/ops/view.go`（`TaskView.Store`）、`cmd/hedgehug/ls.go`・`tree.go`・`show.go`・`log.go`
- Test: `internal/query/sort_test.go`、`internal/ops/read_test.go`、`cmd/hedgehug/ls_test.go`・`tree_test.go`・`show_test.go`・`log_test.go`（ゴールデン）

**Interfaces:**
- Consumes: `mount.Set`、`Mounted.Index`、`ix.StoreName()`（Task 1）
- Produces（`internal/query`）:

```go
// Error gains Unknown: the term names a property the schema does not have
// (an unknown key, a sort:/group: field that is not there, a comparison on
// a property that is not there). mount.Query treats such a store as
// having no match.
type Error struct {
	Pos     int
	Term    string
	Msg     string
	Err     error
	Unknown bool
}

// SortKey is the value of the sort: field of one task, which mount.Query
// compares across stores.
type SortKey struct {
	None  bool    // no value; last in either direction
	Class int     // status: the category's place (todo 0, doing 1, done 2, canceled 3); kind: the role's place (container 0, work 1); 0 for the other fields
	Place int     // status, kind, enum, multi_enum: the value's place in the task's own schema (len when not in it); 0 for the other fields
	Num   float64 // a number, date or datetime that parses
	NumOK bool
	Text  string  // the value as written
	By    string  // "place", "number" or "text": how Text and Num compare, as the field compares now
}

// CompareSortKeys orders two keys from any stores: None last, then Class,
// then by By as the sort compares now (place: Place then Text; number: Num
// (parsed before not) then Text; text: lower-cased Text). desc reverses all
// but None.
func CompareSortKeys(a, b SortKey, desc bool) int

// Result gains the sort: of the query, for mount.Query.
type Result struct {
	Groups []Group
	// SortKey gives a task's key when the query has sort:; nil without it.
	SortKey func(*index.Entry) SortKey
	Desc    bool
}
```

- `query.Parse` は今のまま `store:` を通す。`query.Run` は `store` の項を受け取らない（来たら今の `unknown key`。`mount.Query` が取り除いてから渡す）。
- `internal/mount/query.go`:

```go
// QueryOptions gives the options of the query of one store (its inbox
// tells is:overwritten).
type QueryOptions func(m *Mounted) (query.Options, error)

// Entry is a task found, with its store.
type Entry struct {
	Store *Mounted
	Entry *index.Entry
}

// Group is a group of the result: the tasks of every store with the
// value Key.
type Group struct {
	Key     string
	Entries []Entry
}

// Result is what Query found.
type Result struct {
	Stores  []*Mounted // the stores the query ran on, in config order
	Entries []Entry    // in the order of the groups
	Groups  []Group
}

// Query runs q on each store store: and only (a name) keep, as spec's
// "Store をまたぐ評価" says, and joins what they found.
//   - store:a,b keeps those; -store:a drops it; a name that is not mounted
//     is a *query.Error ("unknown store \"x\"").
//   - a *query.Error with Unknown in one store makes it find nothing, unless
//     every store has it, which is that error.
//   - no sort:: the stores in order, each in its order. sort:: CompareSortKeys
//     across all, ties by store order then the order within the store.
//   - group:: one group per Key across stores; the groups in the order they
//     first come, store by store.
// With only keep, only those stores are opened.
func Query(set *Set, q *query.Query, only string, opt QueryOptions) (*Result, error)
```

- `only` は `--store`（`Session.StoreName`）。空なら全 Store。
- `ops.ListResult` を `{Stores []*mount.Mounted; Entries []mount.Entry; Groups []mount.Group}` に変える。`TreeResult` は Store ごと: `{Stores []TreeStore}`、`TreeStore{Store *mount.Mounted; Index *index.Index; Roots, Cycle []*TreeNode}`（ツリーは Store の中で作る）。`Many bool`（対象の Store が2つ以上）を両方に持たせ、CLI はそれで列・見出しを決める。
- `TaskView` に `Store string \`json:"store"\``（`ix.StoreName()`）を足す。`TaskViewOf(ix, e)` が埋める。`--format` のマップにも入る。
- `ShowResult` に `Store string`。`show` のテキストは `id` と `ulid` の行の後に `store` の行（Store が1つでも）。
- `Log`（ID なし）: 対象の Store ごとに `Repo().Log(nil, limit)` を取り、`When` の新しい順に混ぜ（同じ時刻は Store の順）、`limit` 件で切る（`limit` 0 は全部）。戻り値を `[]StoreLog{Store string; Index *index.Index; Entry gitrepo.LogEntry}` の形に変える（ID ありは当たった Store の1つ）。CLI は Store が2つ以上なら各行の先頭に Store 名（D16）、`--json` の各コミットに `store`。
- CLI `ls`: `Many` なら先頭に `STORE` 列（D14）。`group:` の見出しは今の形。`ls --tree`: `Many` なら Store ごとに `== <name>` の見出し（D15）。`--ulids-only` は変えない。

- [ ] **Step 1: 失敗するテストを書く**
  - `query`: `CompareSortKeys` の表（None が両方向で最後、Class が先、place・number・text の各比べ方、desc）。1つの Store の `Run` の並びが今と同じ（既存のテストがそのまま通る）。`Unknown` が立つのは「ない key」「ない sort:/group: のフィールド」「ないプロパティの比較」だけで、`unknown kind`・`unknown status` では立たない
  - `mount.Query`: `store:org-a` で org-a だけ。`-store:org-a` で残り。`store:nope` → `unknown store "nope"`。org-a だけが持つプロパティ `size:` で org-a の当たりだけ返る、全 Store が持たない `zzz:` は今のエラー。`sort:status` が カテゴリ → 位置 → Store の順（org-a の `review`（doing、位置2）と org-b の `wip`（doing、位置1）で org-b が先）。`sort:kind` が役割 → 位置 → Store の順。`sort:` なしは Store の順。`group:status` で同じ値が1つのグループ、グループの順が最初に現れた順。`only = "org-b"` で org-a の index を開かない（`Mounted` の index が nil のまま）
  - ops: `List` の結果に Store が付く。`Tree` が Store ごと。`Log(nil)` が2つの Store のコミットを時刻順に混ぜ、`limit` が全体
  - CLI ゴールデン: 2つの Store の `ls`（STORE 列）、Store 1つの `ls`（列なし。今と同じ）、`ls --tree`（見出し）、`ls --json`（`store`）、`show` の `store` の行、`log`（Store 名の列）
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**（ゴールデンの差分は `store` の行・`store` のキーだけであることを確かめる）
- [ ] **Step 5: コミット**（`feat: ls・tree・show・log を全 Store で出し、store: で絞る`）

---

### Task 4: 接頭辞が重なっている間の Store 名付きの表示

**Files:**
- Modify: `internal/index/index.go`（表示の ID）、`internal/mount/mount.go`（重なる接頭辞の計算）、`internal/ops/view.go`（`TaskLabel`・`RefName`）と `TaskLabel(` `RefName(` を呼ぶすべての所（`grep -rn 'TaskLabel(\|RefName(\|SubjectNow(' cmd internal`）、`internal/ops/inbox.go`（`InboxContent`）、`internal/ops/read.go`（`SubjectNow`）
- Test: `internal/mount/mount_test.go`、`internal/ops/view_test.go`、`cmd/hedgehug` のゴールデン（新しいテスト `TestPrefixAcrossStores`）

**Interfaces:**
- Consumes: Task 1〜3
- Produces:
  - `index`: `func (ix *Index) SetQualified(fn func(prefix string) bool)`（Refresh しても保つ）、`func (ix *Index) DisplayID(e *Entry) string`: `e.ID` の接頭辞（`model` の ID の分け方を使う）について `fn` が true なら `<StoreName>:<ID>`、そうでなければ `e.ID`。`e.ID` が空なら空。
  - `mount`: `Open` が、開いた Store のワークスペース（`store.Store.Workspaces()`。index を開かずに読む）の接頭辞を集め、2つ以上の Store に現れる接頭辞（大文字小文字を区別しない）の集合を作り、`Mounted.Index()` が開いた index に `SetQualified` で渡す。
  - `ops.TaskLabel(ix *index.Index, e *index.Entry) string`（引数に `ix` を足す）: `DisplayID` を使う。`RefName` もそれを通る。
  - 人が見る出力（一覧、`show`、確認メッセージ、エラー、inbox、`log` の件名の置き換え）で ID を出す所はすべて `DisplayID` か `TaskLabel` を通す。入力の案内（`restore ME-1 first` など）も `DisplayID`（重なっている間は `org-a:ME-1` でないと入力として通らないため）。一時ファイルの名前（`TaskFileName`）は今のまま（`:` をファイル名に入れない）。
  - `--json` の `id` は変えない（`store` があるため）。`InboxContent` の `manyStores` の Store 名の付け方は、この規則と同じ結果になるならそのまま、違うなら `DisplayID` にそろえる（inbox の既存のゴールデン `TestInboxPrefixDuplicate` が変わるなら理由を報告する）。

- [ ] **Step 1: 失敗するテストを書く**
  - `mount`: org-a と org-b の両方に接頭辞 `API`（片方は `api`）→ 両方の index で `DisplayID` が `org-a:API-1`・`org-b:API-1`。接頭辞 `WEB` は org-b だけ → `WEB-1` のまま。開けない Store の接頭辞は数えない
  - ops: `TaskLabel` が重なりの有無で変わる。`Show` の `NotDeleted` のエラー文の案内が `restore org-a:API-1 first`
  - CLI ゴールデン `TestPrefixAcrossStores`: `ls`、`show`、`set` の確認メッセージ、`log` の件名、`inbox` で `org-a:API-1` が出て、`--json` の `id` は `API-1`
  - Store が1つの構成で、既存のゴールデンが変わらない
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat: 接頭辞が Store 間で重なっている間は Store 名付きの ID を出す`）

---

### Task 5: Store をまたぐ参照の検査（同期・doctor）と、全 Store の doctor

**Files:**
- Create: `internal/mount/refs.go`、`internal/mount/refs_test.go`
- Modify: `internal/doctor/finding.go`（`checkOrder` に `cross_store_ref`）、`internal/ops/maintain.go`（`Doctor`）、`internal/ops/sync.go`（`SyncStore` の後の警告）、`cmd/hedgehug/doctor.go`
- Test: `internal/ops/maintain_test.go`、`internal/ops/sync_test.go`、`cmd/hedgehug/doctor_test.go`（ゴールデン）

**Interfaces:**
- Consumes: Task 1〜4
- Produces:

```go
// CrossRef is a reference of a task to a task that is not in its store
// but in another mounted store.
type CrossRef struct {
	From   Entry       // the task that refers
	Field  string      // "parent", "blocked_by" or the task_ref property
	Target Entry       // the task in the other store
}

// CrossRefs finds the cross-store references of the live tasks of the
// store name (all stores when name is ""), against every mounted store.
// The data is not changed.
func CrossRefs(set *Set, name string) ([]CrossRef, error)
```

- doctor: `func (ss *Session) Doctor(fix bool) ([]DoctorReport, error)`。`DoctorReport` に `Store string \`json:"store"\`` を足す。対象は `--store` があればその Store、なければ config の全 Store（開けない Store も今の `doctor.Run(path)` の検査結果を出す）。各 Store の `Findings` に `CrossRefs` の結果を `doctor.Finding{Check: "cross_store_ref", Where: TaskLabel(...), Msg: D19}` で足す（警告の重さは今の Finding の扱いに合わせる。終了コードの判定に「警告」の区別がなければ、`cross_store_ref` だけでは終了コード1にしない形を既存の warning の扱いから探して合わせ、なければ止めて報告する）。`--fix` は各 Store で今の `fixStore`。読み取り専用の Store は直さずに D18 を `ss.warn`。
- CLI `doctor`: Store が2つ以上なら `== <name>` の見出しの下にその Store の出力（D17）。`--json` は配列（Store が1つでも）。
- 同期: `SyncStore` が成功した後、`CrossRefs(set, e.Name)` の結果を D19 の形で `SyncReport.Warnings` に足す（`hh sync` の出力に出る）。失敗は警告にしない（同期は成功している）が、`CrossRefs` 自体のエラーは `could not check cross-store references: <理由>` の警告にする。

- [ ] **Step 1: 失敗するテストを書く**
  - `CrossRefs`: org-a の `API-1` の `blocked_by` に org-b の `WEB-1` の ULID（ファイルを直接書いてコミットして作る）→ 1件、Field が `blocked_by`。`parent`・task_ref も同じ。参照先がどこにもない → 0件（index の「参照先がない」の問題のまま）。削除済みのタスクは見ない
  - ops `Doctor`: 2つの Store で配列、各要素に `store`。org-a に上の参照 → org-a の Findings に `cross_store_ref`。`--store org-b` で org-b だけ。org-b を読み取り専用にして `Doctor(true)` → org-b は直さず D18 の警告、org-a は直す
  - ops `Sync`: 上の参照がある org-a の同期 → `Warnings` に D19 の行
  - CLI ゴールデン: `doctor`（見出し）、`doctor --json`（配列。Store が1つでも）
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**（Store が1つの構成で `doctor --json` の形が配列に変わるので、既存のテストとゴールデンを直す。それ以外の既存のテストは変えない）
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat: Store をまたぐ参照を同期と doctor で知らせ、doctor を全 Store で行う`）

---

### Task 6: コメントの本人確認とメンションの権限の警告

**Files:**
- Modify: `internal/ops/comment.go`（`CheckCommentEditable`、`OpenComment`、`CommentRemove`）、`internal/ops/task.go`（`Add`・`Set` の body）、`internal/ops/edit.go`（`WriteEdit`）、`internal/ops/comment.go`（`WriteComment`・`WriteCommentEdit`）
- Create: `internal/ops/mention.go`、`internal/ops/mention_test.go`
- Test: `internal/ops/comment_test.go`、`cmd/hedgehug/comment_test.go`

**Interfaces:**
- Consumes: `sync.ReadMembers`（3a）、`index.Mentions`・`index.MentionNames`、`config.SameAccount`
- Produces:
  - `func CheckCommentEditable(ref store.CommentRef, c *model.Comment, me []string, verb string) error`: 今の削除済みの検査に加え、`c.Author` が空なら `comment.no_author`、`me` のどれとも `config.SameAccount` で一致しなければ `comment.not_author`（D4。`verb` は `edited`・`removed`）。`OpenComment` と `CommentRemove` が config の `Me()` を渡す。削除済みの検査が先。
  - `func (ss *Session) warnMentions(s *store.Store, before, after string)`: `index.Mentions(after)` のうち `before` に無かったもの（`MentionNames` で比べる）について、その Store の `members.json` の `collaborators` が `null` でなく、誰にも当たらない（`@host/login` はアカウント ID を `SameAccount` で、`@login` はコラボレーターの login と `EqualFold` で比べる）なら D5 を `ss.warn`。`members.json` が読めなければ何もしない（判断できない）。書き込みの後に呼ぶ。
  - 呼ぶ所: `Add`（`before` は空、`after` は body）、`Set` で body を変えたとき、`WriteEdit`（前後の body）、`WriteComment`（`before` は空）、`WriteCommentEdit`（前後のコメント本文）。コミットが作られなかったときは呼ばない。

- [ ] **Step 1: 失敗するテストを書く**
  - config のアカウントが `github.com/Alice` で、author が `github.com/alice` のコメント → 編集・削除できる。author が `github.com/bob` → `comment.not_author`、文面が D4。author が空 → `comment.no_author`。削除済みでかつ他人 → 今の「削除済み」のエラー
  - `members.json` の `collaborators` に `github.com/bob` がいる Store で、body に `@bob` と `@carol` を書く → `carol cannot read org-a` だけが警告。`@github.com/carol` も同じ。もともと body にあった `@carol` を残したまま別の行を足す → 警告しない。`collaborators` が `null` → 警告しない。`members.json` がない → 警告しない。コメントの追加・編集でも同じ
  - CLI: `comment edit` で他人のコメント → 終了コード1、`hedgehug: github.com/bob's comment cannot be edited`。`add` でメンションの警告が標準エラーに `warning: carol cannot read org-a`
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat: コメントの編集・削除を書いた本人に限り、読めない人へのメンションを警告する`）

---

### Task 7: `context.json` と `hh use`

**Files:**
- Modify: `internal/config/paths.go`（`DataDir`）、`internal/context/repoid.go`（パッケージのコメント）
- Create: `internal/context/state.go`、`internal/context/state_test.go`、`internal/ops/use.go`、`internal/ops/use_test.go`、`cmd/hedgehug/use.go`、`cmd/hedgehug/use_test.go`
- Modify: `cmd/hedgehug/app.go`（`useCmd` を足す）

**Interfaces:**
- Produces:
  - `config.Paths.DataDir`: `HEDGEHUG_HOME` があればそれ、なければ `XDG_DATA_HOME/hedgehug`（今の `StoresDir` の親と同じ決め方。`StoresDir` は `DataDir/stores` のまま変えない）。

```go
package context

// Ref names a project in a store, by ULIDs.
type Ref struct {
	Store   model.ULID `json:"store"`
	Project model.ULID `json:"project"`
}

// State is context.json.
type State struct {
	Use    *Ref           `json:"use"`
	Recent map[string]Ref `json:"recent"` // by repo ID
}

// Load reads <dataDir>/context.json. A missing file is the empty state. A
// file that does not read is the empty state too, and warn is told why
// (D11).
func Load(dataDir string, warn func(string)) State

// Save writes st to <dataDir>/context.json through a temporary file and a
// rename, with no lock (the last write wins).
func Save(dataDir string, st State) error
```

- `ops`:

```go
// UseResult is the project hh use set or shows; Project is nil when there is none.
type UseResult struct {
	Store   string         // the store's name
	Project *index.Entry
	Index   *index.Index
	Changed bool
}

// Use makes the project arg names (any store, "<store>:" allowed) the
// default parent of new tasks. A task whose kind is not project is
// KindInvalid, use.not_project.
func (ss *Session) Use(arg string) (UseResult, error)

// UseShow is the project hh use set, or a zero result. A project that is
// gone (its store is not mounted, it is not there, deleted or a forward)
// is warned of (D12) and shown as none.
func (ss *Session) UseShow() (UseResult, error)

// UseClear removes it; Changed is false when there was none.
func (ss *Session) UseClear() (UseResult, error)
```

- CLI `hh use [<project>] [--clear]`: 出力は D7。`<project>` と `--clear` の併用は使い方の誤り。`--json` 対応。

- [ ] **Step 1: 失敗するテストを書く**
  - `Paths.DataDir` が `HEDGEHUG_HOME`・`XDG_DATA_HOME` で決まる
  - `Load`: ない → 空。壊れた JSON → 空と D11 の警告。`Save` → `Load` で同じ値。`Save` が一時ファイルを残さない
  - `Use("org-a:API-3")`（kind が project）→ context.json の `use` に org-a の ULID と project の ULID。kind が task → `use.not_project`。`UseShow` で同じ project。project を削除してから `UseShow` → D12 の警告と空。`UseClear` → `use` が null。もう一度 → `Changed` が false
  - CLI ゴールデン: `use org-a:API-3`、`use`、`use --json`、`use --clear`
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat: hh use と context.json`）

---

### Task 8: `hh add` のコンテキスト解決

**Files:**
- Create: `internal/context/resolve.go`、`internal/context/resolve_test.go`
- Modify: `internal/ops/task.go`（`AddInput`）、`internal/ops/` に `addhere.go`（新規）と `addhere_test.go`、`cmd/hedgehug/add.go`、`cmd/hedgehug/add_test.go`

**Interfaces:**
- Consumes: Task 1〜7
- Produces（`internal/context`）:

```go
// Input is what Resolve decides the destination from.
type Input struct {
	Dir                               string // the current directory; "" skips stages 3 and 4
	Store, Project, Parent, Workspace string // --store, --project, --parent, --workspace
	DefaultStore                      string // config's default_store
	Interactive                       bool   // stdin is a terminal: stages 3–5 may ask
	// The answers to what an earlier call asked; zero when not asked.
	BranchParent Answer // stage 3
	PickProject  string // stage 4: the store-qualified id picked
	PickStore    string // stage 5
	PickWorkspace string // stage 5
}

type Answer int

const (
	Unasked Answer = iota
	Yes
	No
)

// Destination is where the new task goes.
type Destination struct {
	Store     *mount.Mounted
	Parent    *index.Entry   // nil at a workspace's root
	Workspace *model.Workspace
	Stage     int            // 1–5, the stage that decided
	RepoID    string         // the repo ID of the current repository, "" when none (stage 4's rule)
	Recent    bool           // stage 4 picked a project: recent[RepoID] is to be set to it
}

// Confirm is what stage 3 asks: whether to put the task under Parent.
type Confirm struct {
	Store  *mount.Mounted
	Parent *index.Entry
}

// Choice is what stages 4 and 5 ask: one of Options, Default ("" when none).
type Choice struct {
	What    string   // "project", "store" or "workspace"
	Options []string // store-qualified ids, store names or workspace names
	Default string
}

// Resolve decides the destination by the five stages of the spec. When a
// stage needs an answer it returns (nil, *Confirm, nil) or
// (nil, nil, *Choice) when Interactive; the caller asks and calls again with
// the answer. Not Interactive: stage 3 is skipped, stage 4 uses recent or is
// an error, stage 5 is an error, as the spec says.
func Resolve(set *mount.Set, st State, in Input, warn func(string)) (*Destination, *Confirm, *Choice, error)
```

- ブランチ名の段: go-git の `git.PlainOpenWithOptions(in.Dir, &git.PlainOpenOptions{DetectDotGit: true})`、HEAD がブランチを指していて、名前が `hh/<ID>-…` か `hedgehug/<ID>-…`（`<ID>` は接頭辞と `-` と数字。数字の後は `-` か終わり）。ID は全 Store から引く（`set.Resolve`）。当たらなければ飛ばす。
- repo の段: 同じリポジトリの remote を `RepoID` にし（`origin` があればそれだけ、なければ全 remote を名前順）、そのどれかを `repos` に持つ、削除されていない kind `project` を全 Store から探す。remote は `url_rewrites` の前の値（`.git/config` の値そのもの）。
- 各段のエラー（spec の各段の文言を英語に）: `--parent` と `--project` の併用は `KindUsage`。`--project` が project でない → `KindInvalid`、`add.not_project`。`--workspace` が2つ以上の Store で当たった → `KindAmbiguous`、Store 名付きで並べる。段 4 で端末でなく `recent` もない → `choiceRequired("project", 候補)`。段 5 で端末でない → `choiceRequired("store", 名前)` に `--store` か `hedgehug store default` の案内、ワークスペースは `choiceRequired("workspace", 名前)` に `--workspace` の案内。
- `ops`:

```go
// AddInput gains Store (the store to add to; "" selects as now) and Repos
// (repo IDs to put on the task). Parent and Workspace are read in that store;
// Parent goes through ResolveRef.
type AddInput struct {
	Store                                  string
	Title, Parent, Workspace, Kind, Status string
	Repos                                  []string
	Assigns                                []Assign
}

// AddHereInput is hh add with the context: what Resolve takes, and the task.
type AddHereInput struct {
	Context hhctx.Input
	NoRepo  bool
	Task    AddInput // Store, Parent and Workspace are set from the destination
}

// AddHereResult is the task added and where it went.
type AddHereResult struct {
	TaskResult
	Destination *hhctx.Destination
}

// AddHere resolves the destination (its Confirm and Choice are returned for
// the CLI to ask), refuses a read-only store there, adds the task with the
// repo ID of the current repository unless NoRepo, and sets recent when
// stage 4 picked a project (after the commit).
func (ss *Session) AddHere(in AddHereInput) (AddHereResult, *hhctx.Confirm, *hhctx.Choice, error)
```

- GUI（4f）は `AddHere` を使わず、`Add` に `Store`・`Parent`・`Workspace` を渡す（決16）。
- CLI `hh add <title> [--project p] [--parent p] [--workspace w] [--no-repo] ...`: `Interactive` は標準入力が端末か（今の端末の判定を使う）。`Confirm` なら D8 を聞いて `BranchParent` を入れて呼び直す。`Choice` なら D9・D10 で番号を聞き、`Pick…` を入れて呼び直す。出力は今の出力の後に D6 の1行。`--json` に `store`・`parent`（ULID か null）・`workspace` を足す。

- [ ] **Step 1: 失敗するテストを書く**（`internal/context`。git のリポジトリは go-git の `PlainInit` で作り、remote とブランチを足す）
  - 段1: `--store org-b` だけ → org-b のワークスペースが1つならそのルート。`--parent WEB-1`（`--store` なし）→ org-b の WEB-1 の下。`--project API-3`（project）→ その下、kind が task → `add.not_project`。`--parent` と `--project` → `KindUsage`。`--workspace web`（org-b にだけある）→ org-b のルート。両方にある → `KindAmbiguous`
  - 段2: context.json の `use` → その project。project が削除済み → D12 の警告と次の段へ
  - 段3: ブランチ `hh/WEB-1-login` → `Confirm`。`BranchParent = Yes` で WEB-1 の下、`No` で次の段。`Interactive = false` で飛ばす。`feature/x` → 飛ばす。`hh/WEB-999-x`（ない ID）→ 飛ばす
  - 段4: remote `https://github.com/Org-A/Web.git` と、`repos: [github.com/org-a/web]` を持つ project が1件 → その下、`Recent` が true、`RepoID` が `github.com/org-a/web`。2件 → `Interactive` なら `Choice`（既定は recent）、`Interactive = false` で recent があればそれ、なければ `choice.required`
  - 段5: `default_store` → そのワークスペースのルート。なし・Store 2つ → `Interactive` なら `Choice`、でなければ `choice.required`。ワークスペース2つも同じ
  - repo の段で決まらなくても、`RepoID` は入る（段1で決まったときも）
  - ops `AddHere`: 決まった Store が読み取り専用 → `store.read_only`。repo ID がタスクの `repos` に入る、`NoRepo` で入らない。段4で決まったら context.json の `recent[<repo ID>]` が project
  - CLI ゴールデン: web の clone の中での `add`（D6 の1行、`--json` の `store`・`parent`・`workspace`）、端末の入力でのブランチの確認と project の選択（今の端末の入力のテストの方法で）
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat: hh add の入れ先をコンテキスト解決で決める`）

---

### Task 9: devenv の 3b（コードのリポジトリ web）

**Files:**
- Modify: `tools/devenv/main.go`、`tools/devenv/main_test.go`

**Interfaces:**
- Consumes: Task 8（`repos` を持つ project）
- Produces（spec の「devenv」の 3b）:
  - `remotes/org-a/web.git`（コードのリポジトリ。中身は README 1つのコミット）
  - org-a の Store に、`repos: [github.com/org-a/web]` を持つ kind `project` のタスクを1つ（タイトル `Web release`）
  - `users/alice/src/web` に web を clone（remote `origin` は `https://github.com/org-a/web.git`。`.gitconfig` の `insteadOf` で remotes に向く。clone は go-git で、書き換え後のローカルのパスから行い、その後 `origin` の URL を `https://github.com/org-a/web.git` に書き直す）
  - fakegh.json の `org-a/web`（topic なし）は 3a のまま
  - `init` の最後の案内に `cd ~/hh-dev/users/alice/src/web && hedgehug add "..."` の例を足す

- [ ] **Step 1: 失敗するテストを書く**: `init` の後に `remotes/org-a/web.git` が bare で HEAD を持つ、org-a の Store に `repos` を持つ project がある、`users/alice/src/web` の `origin` が `https://github.com/org-a/web.git`
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
- [ ] **Step 4: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 5: コミット**（`feat(devenv): コードのリポジトリ web と repos を持つ project を作る`）

---

### Task 10: 3b の E2E

**Files:**
- Create: `cmd/hedgehug/e2e_3b_test.go`

**Interfaces:**
- Consumes: Task 1〜9。`cmd/hedgehug/e2e_3a_test.go` の作り（バイナリの起動、fakegh を httptest で、devenv の `init --fakegh <url>`）をそのまま使う。

- [ ] **Step 1: テストを書く**（spec の「テスト」の E2E 3b。alice の構成で）
  - 3つの Store（me・org-a・org-b）の `ls` に STORE 列、`ls store:org-a`、`ls assignee:@me`（alice と alice-corp の両方の担当が出る）
  - ID の解決: `show WEB-1`、接頭辞が重なる構成での `org-a:X-1`、曖昧な ID の終了コード2
  - Store をまたぐ参照の拒否: `dep API-1 --blocked-by WEB-1` が終了コード1
  - web の clone の中での `add "x"` → org-a の `Web release` の下、`repos` に `github.com/org-a/web`、D6 の1行
  - `hh use org-a:<project>` の後に別のディレクトリで `add` → その project の下。`use --clear`
  - ブランチ `hh/<ID>-x` の段は、端末でないので飛ばされる（E2E はバイナリで標準入力が端末でない）
  - コメントの本人確認: bob のコメントを alice が `comment edit` → 終了コード1
  - メンションの警告: bob が読めない Store（fakegh で collaborators から外し、`sync` で members.json を取る）で `@bob` を書く → 標準エラーに警告
- [ ] **Step 2: 通るまで直す**（直すのが plan の前の Task のコードなら、その Task の規則に従って直す）
- [ ] **Step 3: `go test -race ./...` と `golangci-lint run ./...` を通す**
- [ ] **Step 4: コミット**（`test: 3b の E2E`）

---

### Task 11: 仕上げ（コントローラーが行う）

- [ ] spec の「コマンドの一覧」と「doctor の追加」の 3b の行が、すべて実装とテストに当たっているかを見直す（spec の各節から Task への対応を表にして、抜けがあれば Task を足して直す）
- [ ] 親設計ドキュメントに、3b で決めたこと（利用者の決定4件: `log` の混ぜ方、読み取り専用の拒否の順、`KindAmbiguous` の終了コード2、`doctor` の Store ごとの見出し。`sort:kind` の役割の順。この plan の D1〜D19 のうち親設計の章に当たるもの）を書き足す
- [ ] 全体のレビュー（`superpowers:requesting-code-review`）の指摘を直す
- [ ] PR を作る（`superpowers:finishing-a-development-branch`）
