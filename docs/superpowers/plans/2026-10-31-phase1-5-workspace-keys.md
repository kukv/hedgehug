# フェーズ 1.5: ワークスペースと人が読めるキー 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ワークスペースを新設し、タスクの作成時に `{接頭辞}-{番号}` のキーを Store のロックの中で振り、CLI のすべての表示をキーと完全な ULID の併記にし、短縮 ID をコードから消す。

**Architecture:** 接頭辞はワークスペース（`workspaces/<ULID>.md`）だけが持ち、タスクは `workspace` フィールドで属す。採番は `store.Update` の中（ロックの中）で、`PutTask` されたタスクのうち「キーがない、またはキーの接頭辞が所属ワークスペースと違う」ものに振る。最大番号は index が `store.KeyNumbers` として供給し、無いときは store が全タスクを読む。CLI は表示と入力を作り直し、短縮 ID を消す。

**Tech Stack:** Go、go-git v5、go.yaml.in/yaml/v4、cobra

**Spec:** `docs/superpowers/specs/2026-10-31-phase1-5-workspace-keys-design.md`（必ず先に読む。親設計は https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc の「人が読めるキー」「ワークスペース」）

## Global Constraints

- 接頭辞の形式: `^[A-Z][A-Z0-9]{1,9}$`。キーの形式: `^([A-Z][A-Z0-9]{1,9})-([1-9][0-9]*)$`
- 番号は接頭辞ごとに Store 全体で数える。次の番号 = max(全タスク（削除済みを含む）の `key` と `former_keys` のその接頭辞の番号, `.hedgehug/numbers.yaml` の値, 同じ操作で振った番号) + 1
- 短縮 ID は表示・入力・コードのどこにも残さない（`ShortID`・`ShortLengths`・`minShort`・`"short"`・コメントの末尾一致）
- 表示はすべてキーと完全な ULID を並べる。`--ids-only` だけは完全な ULID のみ
- 入力はキー・古いキー・完全な ULID（大文字小文字を区別しない）
- 既存 Store の移行は作らない。store.yaml の format は 1 のまま
- コメントの密度・命名・英語のコメントは周りのコードに合わせる。エラーメッセージは英語（既存に合わせる）
- 各タスクの最後に `go test ./...` と `go vet ./...` が通ること。`make lint` があれば通すこと
- コミットメッセージは日本語の Conventional Commits（既存の履歴に合わせる）で、末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`

---

## ファイル構成

```
internal/model/
  workspace.go      新規: Workspace 型、DecodeWorkspace/Encode
  key.go            新規: ValidKeyPrefix、ParseKey、FormatKey
  numbers.go        新規: numbers.yaml（Numbers 型、DecodeNumbers/Encode）、NumbersPath
  path.go           WorkspacePath、ParseWorkspacePath
  task.go           KeyPrefix を削除、Workspace を追加
  storemeta.go      KeyPrefix を削除
internal/schema/
  schema.go         既定のスキーマで project の parents を [root]
internal/store/
  store.go          Create にワークスペースを渡す、SetKeyNumbers
  read.go           Workspaces、Workspace、Numbers
  tx.go             PutWorkspace、RemoveWorkspace、Workspace、RemoveTask で numbers を記録、assignKeys の呼び出し
  keys.go           新規: KeyNumbers 型、maxNumbers、assignKeys
  validate.go       ワークスペースの検査
internal/index/
  index.go          workspaces、Workspace(s)、keyNumbers の登録
  workspace.go      新規: ResolveWorkspace
  resolve.go        新規: shortid.go から Resolve と AmbiguousError を移し、末尾一致を削除
  shortid.go        削除
  entry.go          Entry.Workspace
  problems.go       キーとワークスペースの問題
  build.go          ワークスペースの読み込み、cacheFormat を上げる
internal/query/
  eval.go           workspace: を追加
internal/doctor/
  doctor.go         workspaces を fileRoots に、ワークスペースの重複の検査、indexCodes にキーの問題
cmd/hedgehug/
  store_init.go     --prefix（必須）、--workspace
  workspace.go      新規: workspace add/ls/rename/prefix/rm と、他の Store との照合
  add.go mv.go edit.go
  output.go tree.go show.go log.go ls.go rm.go migrate.go comment.go app.go status.go dep.go set.go
tools/devenv/main.go   store.Create の呼び出しを直す
```

---

### Task 1: model — ワークスペース、キー、numbers.yaml

**Files:**
- Create: `internal/model/workspace.go`, `internal/model/workspace_test.go`, `internal/model/key.go`, `internal/model/key_test.go`, `internal/model/numbers.go`, `internal/model/numbers_test.go`
- Modify: `internal/model/path.go`, `internal/model/task.go`, `internal/model/storemeta.go`, `internal/model/task_test.go`, `internal/model/storemeta_test.go`, `internal/model/testdata/task/full.md`（`key_prefix` があれば消し、`workspace` を足す）、`internal/schema/schema.go`
- Test: 上の `_test.go`

**Interfaces:**
- Produces:
  - `type Workspace struct { ID ID; Name string; KeyPrefix string; PrefixSet time.Time; Created, Updated time.Time; Extra Fields; Body string; doc *Doc }`
  - `func DecodeWorkspace([]byte) (*Workspace, error)`（`id` がなければ `DecodeError`）、`func (*Workspace) Encode() ([]byte, error)`
  - `func WorkspacePath(id ID) string` → `"workspaces/" + id + ".md"`、`func ParseWorkspacePath(p string) (ID, error)`
  - `func ValidKeyPrefix(p string) error`、`func ParseKey(k string) (prefix string, n int, ok bool)`、`func FormatKey(prefix string, n int) string`
  - `type Numbers map[string]int`、`const NumbersPath = ".hedgehug/numbers.yaml"`、`func DecodeNumbers([]byte) (Numbers, error)`、`func (Numbers) Encode() ([]byte, error)`
  - `Task.Workspace ID`（zero なら未設定）。`Task.KeyPrefix` と `StoreMeta.KeyPrefix` は削除

- [ ] **Step 1: key のテストを書く**

```go
package model

import "testing"

func TestValidKeyPrefix(t *testing.T) {
	for _, p := range []string{"WEB", "A1", "ABCDEFGHIJ"} {
		if err := ValidKeyPrefix(p); err != nil {
			t.Errorf("%q: %v", p, err)
		}
	}
	for _, p := range []string{"", "W", "web", "1AB", "ABCDEFGHIJK", "WE-B"} {
		if ValidKeyPrefix(p) == nil {
			t.Errorf("%q is accepted", p)
		}
	}
}

func TestParseKey(t *testing.T) {
	if p, n, ok := ParseKey("WEB-42"); !ok || p != "WEB" || n != 42 {
		t.Errorf("WEB-42 = %q %d %v", p, n, ok)
	}
	for _, k := range []string{"", "WEB", "WEB-0", "WEB-042", "web-1", "W-1", "WEB-1x"} {
		if _, _, ok := ParseKey(k); ok {
			t.Errorf("%q parses", k)
		}
	}
	if FormatKey("API", 7) != "API-7" {
		t.Error(FormatKey("API", 7))
	}
}
```

- [ ] **Step 2: 失敗を確かめる** — `go test ./internal/model/ -run 'TestValidKeyPrefix|TestParseKey'` → コンパイルエラー

- [ ] **Step 3: key.go を書く**

```go
package model

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	prefixRE = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)
	keyRE    = regexp.MustCompile(`^([A-Z][A-Z0-9]{1,9})-([1-9][0-9]*)$`)
)

// ValidKeyPrefix checks a key prefix: 2 to 10 upper-case letters and
// digits, starting with a letter.
func ValidKeyPrefix(p string) error {
	if !prefixRE.MatchString(p) {
		return fmt.Errorf("%q is not a key prefix: use 2 to 10 upper-case letters and digits, starting with a letter", p)
	}
	return nil
}

// ParseKey splits a key such as WEB-42 into its prefix and number.
func ParseKey(k string) (prefix string, n int, ok bool) {
	m := keyRE.FindStringSubmatch(k)
	if m == nil {
		return "", 0, false
	}
	n, err := strconv.Atoi(m[2])
	if err != nil {
		return "", 0, false
	}
	return m[1], n, true
}

// FormatKey makes the key of number n under prefix.
func FormatKey(prefix string, n int) string { return prefix + "-" + strconv.Itoa(n) }
```

- [ ] **Step 4: ワークスペースと numbers のテストを書く**

`workspace_test.go`: frontmatter を Decode → Encode して同じバイト列に戻る（`testdata` は使わず文字列で）。`id` なしは `*DecodeError`。

```go
func TestWorkspaceRoundTrip(t *testing.T) {
	in := "---\nid: 01JC0A1B2C3D4E5F6G7H8J9K0M\nname: web\nkey_prefix: WEB\nprefix_set: 2026-10-31T10:00:00+09:00\ncreated: 2026-10-31T10:00:00+09:00\nupdated: 2026-10-31T10:00:00+09:00\n---\nWeb のタスク。\n"
	w, err := DecodeWorkspace([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != "web" || w.KeyPrefix != "WEB" || w.PrefixSet.IsZero() || w.Body != "Web のタスク。\n" {
		t.Errorf("%+v", w)
	}
	out, err := w.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != in {
		t.Errorf("Encode =\n%s", out)
	}
	if _, err := DecodeWorkspace([]byte("---\nname: x\n---\n")); !errors.As(err, new(*DecodeError)) {
		t.Errorf("no id: %v", err)
	}
	if WorkspacePath(w.ID) != "workspaces/01JC0A1B2C3D4E5F6G7H8J9K0M.md" {
		t.Error(WorkspacePath(w.ID))
	}
	if id, err := ParseWorkspacePath(WorkspacePath(w.ID)); err != nil || id != w.ID {
		t.Errorf("ParseWorkspacePath = %v %v", id, err)
	}
}
```

`numbers_test.go`:

```go
func TestNumbers(t *testing.T) {
	n := Numbers{"WEB": 42, "API": 7}
	b, err := n.Encode()
	if err != nil {
		t.Fatal(err)
	}
	want := "# The largest number of the purged tasks, for each key prefix. Numbering\n# never gives these numbers again.\nAPI: 7\nWEB: 42\n"
	if string(b) != want {
		t.Errorf("Encode =\n%s", b)
	}
	got, err := DecodeNumbers(b)
	if err != nil || got["WEB"] != 42 || got["API"] != 7 {
		t.Errorf("DecodeNumbers = %v %v", got, err)
	}
	if got, err := DecodeNumbers(nil); err != nil || len(got) != 0 {
		t.Errorf("empty = %v %v", got, err)
	}
	if _, err := DecodeNumbers([]byte("WEB: x\n")); err == nil {
		t.Error("a bad number decodes")
	}
}
```

- [ ] **Step 5: 失敗を確かめる** — `go test ./internal/model/` → コンパイルエラー

- [ ] **Step 6: 実装する**

`workspace.go` は `sprint.go` と同じ作り（`workspaceKeys = []string{"id", "name", "key_prefix", "prefix_set", "created", "updated"}`、`decodeAll` のマップに `prefix_set: &w.PrefixSet`、`Encode` は同じ順で `Put`）。`path.go` に `WorkspacePath` と、`sprints/` の解析と同じ作りの `ParseWorkspacePath` を足す（既存のパス解析の関数があればそれに合わせる）。

`numbers.go`:

```go
package model

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"
)

// NumbersPath is the record of the numbers of purged tasks.
const NumbersPath = ".hedgehug/numbers.yaml"

// Numbers is, for each key prefix, the largest number of a purged task.
// Numbering counts it, so a purged task's number is never given again.
type Numbers map[string]int

func DecodeNumbers(data []byte) (Numbers, error) {
	n := Numbers{}
	if err := yaml.Unmarshal(data, &n); err != nil {
		return nil, fmt.Errorf("%s: %w", NumbersPath, err)
	}
	if n == nil {
		n = Numbers{}
	}
	return n, nil
}

// Encode writes the prefixes in order, after a comment saying what the
// file is.
func (n Numbers) Encode() ([]byte, error) {
	var b strings.Builder
	b.WriteString("# The largest number of the purged tasks, for each key prefix. Numbering\n# never gives these numbers again.\n")
	keys := make([]string, 0, len(n))
	for k := range n {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%s: %d\n", k, n[k])
	}
	return []byte(b.String()), nil
}
```

`task.go`: `KeyPrefix` を削除し、`Workspace ID` を `FormerKeys` の後に足す。`taskKeys` は `"id", "key", "former_keys", "workspace", "kind", ...`（`key_prefix` を消す）。`DecodeTask` の表に `{"workspace", &t.Workspace}`、`Encode` に `e.Put("workspace", t.Workspace, t.Workspace.IsZero())`。`Key` のコメントは `// human-readable key such as WEB-42, given when the task is written`。
`storemeta.go`: `KeyPrefix` と `storeMetaKeys` の `key_prefix` と `Put("key_prefix", …)` を削除。
`schema.go`: `Default()` の project を `{Name: "project", Label: "プロジェクト", Role: RoleContainer, Parents: []string{Root}}` にする（`Dev()` は `Default()` から作るので変更不要。確かめる）。

既存テストで `KeyPrefix` を参照しているもの（`task_test.go`、`storemeta_test.go`、`testdata/task/full.md`）を直す。`full.md` に `key_prefix` があれば消し、`workspace: 01JC0A1B2C3D4E5F6G7H8J9K0M` を `former_keys` の次の行に足して、ゴールデンを合わせる。スキーマのテストが project の `parents` を前提にしていれば直す。

- [ ] **Step 7: 通るのを確かめる** — `go build ./... && go test ./internal/model/ ./internal/schema/`。`go build ./...` が `KeyPrefix` 参照で落ちる箇所（`cmd/hedgehug/edit.go` などにあれば）も、この Task で参照を消す

- [ ] **Step 8: コミット**

```bash
git add internal/model internal/schema
git commit -m "feat: ワークスペースとキーと numbers.yaml の型を足す"
```

---

### Task 2: store — ワークスペースの読み書き、Create、purge の記録

**Files:**
- Modify: `internal/store/store.go`, `internal/store/read.go`, `internal/store/tx.go`, `internal/store/helpers_test.go`, `internal/store/store_test.go`
- Create: `internal/store/workspace_test.go`
- Modify（呼び出しを直すだけ）: `cmd/hedgehug/store_init.go`, `tools/devenv/main.go`, `internal/index/helpers_test.go`, `internal/query/helpers_test.go`, `internal/doctor/helpers_test.go`

**Interfaces:**
- Consumes: Task 1 の型
- Produces:
  - `func Create(dir string, meta *model.StoreMeta, sc *schema.Schema, ws *model.Workspace, author gitrepo.Signature) (*Store, error)` — ws を store.yaml などと同じ初回コミットに入れる。ws の `Created`/`Updated`/`PrefixSet` が zero なら作成時刻を入れる。ws は必須（nil ならエラー）、`ValidKeyPrefix` で検査する
  - `func (s *Store) Workspaces() iter.Seq2[*model.Workspace, error]`、`func (s *Store) Workspace(id model.ID) (*model.Workspace, error)`（無ければ `ErrNotFound`）、`func (s *Store) Numbers() (model.Numbers, error)`（ファイルが無ければ空）
  - `func (tx *Tx) PutWorkspace(*model.Workspace) error`、`func (tx *Tx) RemoveWorkspace(id model.ID) error`、`func (tx *Tx) Workspace(id model.ID) (*model.Workspace, error)`（同じ操作で書いたものを優先。消したものは `ErrNotFound`）
  - `Tx.RemoveTask` は消すタスクの `key` と `former_keys` の番号で `numbers.yaml` を更新する（記録より大きいときだけ）

- [ ] **Step 1: テストを書く**（`workspace_test.go`）

```go
func TestCreateWritesWorkspace(t *testing.T) {
	s := newTestStore(t)
	var got []*model.Workspace
	for w, err := range s.Workspaces() {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, w)
	}
	if len(got) != 1 || got[0].KeyPrefix != "TST" || got[0].Name != "test" || got[0].PrefixSet.IsZero() {
		t.Fatalf("workspaces = %+v", got)
	}
}

func TestPutAndRemoveWorkspace(t *testing.T) {
	s := newTestStore(t)
	w := &model.Workspace{ID: model.NewID(), Name: "api", KeyPrefix: "API"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(w) }); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Workspace(w.ID); err != nil || got.KeyPrefix != "API" {
		t.Fatalf("Workspace = %+v %v", got, err)
	}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveWorkspace(w.ID) }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Workspace(w.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("after remove: %v", err)
	}
}

func TestPurgeRecordsNumbers(t *testing.T) {
	s := newTestStore(t)
	task := &model.Task{ID: model.NewID(), Workspace: testWorkspace(t, s).ID, Kind: "task", Title: "x", Status: "todo", Key: "TST-9", FormerKeys: []string{"OLD-12"}}
	put(t, s, task)
	if _, err := s.Update(Op{Name: "purge", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveTask(task.ID) }); err != nil {
		t.Fatal(err)
	}
	n, err := s.Numbers()
	if err != nil || n["TST"] != 9 || n["OLD"] != 12 {
		t.Errorf("Numbers = %v %v", n, err)
	}
}
```

`helpers_test.go` を直す: `newTestStore` は `Create(..., nil, &model.Workspace{ID: model.NewID(), Name: "test", KeyPrefix: "TST"}, testAuthor)`。`testWorkspace(t, s) *model.Workspace` を足す（`s.Workspaces()` の最初の1件を返す）。`put(t, s, tasks...)` が無ければ index の helpers_test.go と同じものを足す。このパッケージの既存テストで `model.Task{...}` を書いているところに `Workspace: testWorkspace(t, s).ID` を足すのは Task 5 で行う（この Task ではまだ必須にしない）。

`store_test.go` の `Create(...)` の呼び出し（7か所）に ws 引数を足す。ws が nil と、接頭辞が不正なとき Create がエラーになるテストを足す:

```go
func TestCreateRefusesBadWorkspace(t *testing.T) {
	meta := &model.StoreMeta{ID: model.NewID(), Name: "x"}
	if _, err := Create(filepath.Join(t.TempDir(), "a"), meta, nil, nil, testAuthor); err == nil {
		t.Error("nil workspace is accepted")
	}
	ws := &model.Workspace{ID: model.NewID(), Name: "x", KeyPrefix: "x"}
	if _, err := Create(filepath.Join(t.TempDir(), "b"), meta, nil, ws, testAuthor); err == nil {
		t.Error("bad prefix is accepted")
	}
}
```

- [ ] **Step 2: 失敗を確かめる** — `go test ./internal/store/` → コンパイルエラー

- [ ] **Step 3: 実装する**

- `store.go` の `Create`: 引数 `ws` を足す。`ws == nil` → `errors.New("the store needs a workspace")`、`model.ValidKeyPrefix(ws.KeyPrefix)` を通す。ws をコピーし、`now := s.now().Truncate(time.Second)` で zero の `Created`/`Updated`/`PrefixSet` を埋め、`model.WorkspacePath(ws.ID)` に書いて `files` に足す。コミットの `When` は同じ `now`
- `read.go`: `Workspace(id)` は `readDecode(s.dir, model.WorkspacePath(id), model.DecodeWorkspace)`（`Sprint` と同じ）。`Workspaces()` は `walk(s, "workspaces", flatMatch(model.WorkspacePath), ignoreRel(model.DecodeWorkspace))`（`Sprints` と同じ）。`Numbers()` は `readFile(s.path(model.NumbersPath))`、`fs.ErrNotExist` なら `model.Numbers{}`、それ以外は `model.DecodeNumbers`
- `tx.go`:
  - `Tx` に `workspaces map[model.ID]*model.Workspace`（nil 値 = 消した）と `numbers model.Numbers`（nil = まだ読んでいない）を足し、`Update` で `workspaces: map[model.ID]*model.Workspace{}` に初期化する
  - `PutWorkspace`: `ID` が zero ならエラー。`tx.put(model.WorkspacePath(w.ID), &pending{encode: w.Encode, created: &w.Created, updated: &w.Updated})` と `tx.workspaces[w.ID] = w`
  - `RemoveWorkspace`: `tx.put(model.WorkspacePath(id), &pending{remove: true})` と `tx.workspaces[id] = nil`
  - `Workspace(id)`: `tx.done` なら `errTxDone`。`tx.workspaces` にあればそれ（nil なら `ErrNotFound`）、なければ `tx.s.Workspace(id)`
  - `RemoveTask` の最初（`rels` を作る前）で、`tx.task(id)` が読めたら（`ErrNotFound` と `DecodeError` は無視）`tx.recordNumbers(t)` を呼ぶ:

```go
// recordNumbers raises the record of purged numbers to the numbers of t's
// keys, so numbering never gives them again.
func (tx *Tx) recordNumbers(t *model.Task) error {
	if tx.numbers == nil {
		n, err := tx.s.Numbers()
		if err != nil {
			return err
		}
		tx.numbers = n
	}
	changed := false
	for _, k := range append([]string{t.Key}, t.FormerKeys...) {
		if p, n, ok := model.ParseKey(k); ok && n > tx.numbers[p] {
			tx.numbers[p] = n
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return tx.put(model.NumbersPath, &pending{encode: tx.numbers.Encode})
}
```

- 呼び出し側: `cmd/hedgehug/store_init.go` は今は `&model.Workspace{ID: model.NewID(), Name: name, KeyPrefix: "ME"}` を仮に渡す（Task 6 で `--prefix` に置き換える。コメントは付けない）。`tools/devenv/main.go` は `&model.Workspace{ID: model.NewID(), Name: storeName, KeyPrefix: "DEV"}`。`internal/index`・`internal/query`・`internal/doctor` の `helpers_test.go` の `newStore` は `&model.Workspace{ID: model.NewID(), Name: "test", KeyPrefix: "TST"}`

- [ ] **Step 4: 通るのを確かめる** — `go build ./... && go test ./...`

- [ ] **Step 5: コミット**

```bash
git add -A internal/store internal/index internal/query internal/doctor cmd/hedgehug/store_init.go tools/devenv
git commit -m "feat: Store にワークスペースの読み書きと purge の番号の記録を足す"
```

---

### Task 3: store — 作成時の採番と振り直し

**Files:**
- Create: `internal/store/keys.go`, `internal/store/keys_test.go`
- Modify: `internal/store/store.go`（`keyNumbers` フィールドと `SetKeyNumbers`）, `internal/store/tx.go`（`Update` で `assignKeys`、`commitFor` の ULID 代用を残すのは Task 5 まで）

**Interfaces:**
- Consumes: Task 2 の `Tx.Workspace`、`Store.Numbers`、`tx.numbers`
- Produces:
  - `type KeyNumbers func() (map[string]int, error)` — 各接頭辞について、HEAD のタスクの `key` と `former_keys` の最大番号
  - `func (s *Store) SetKeyNumbers(f KeyNumbers)`
  - `Update` は `fn` の後、`validate` の前に `tx.assignKeys()` を呼ぶ

- [ ] **Step 1: テストを書く**（`keys_test.go`）

```go
func newTaskIn(ws *model.Workspace, title string) *model.Task {
	return &model.Task{ID: model.NewID(), Workspace: ws.ID, Kind: "task", Title: title, Status: "todo"}
}

func TestAssignKeys(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	a, b := newTaskIn(ws, "a"), newTaskIn(ws, "b")
	put(t, s, a, b) // one operation: numbers given in it count
	c := newTaskIn(ws, "c")
	put(t, s, c)
	if a.Key != "TST-1" || b.Key != "TST-2" || c.Key != "TST-3" {
		t.Fatalf("keys = %s %s %s", a.Key, b.Key, c.Key)
	}
	if got, _ := s.Task(c.ID); got.Key != "TST-3" {
		t.Errorf("file key = %s", got.Key)
	}
}

func TestCommitSubjectUsesNewKey(t *testing.T) {
	s := newTestStore(t)
	x := newTaskIn(testWorkspace(t, s), "x")
	if _, err := s.Update(Op{Name: "add", Task: x.ID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(x) }); err != nil {
		t.Fatal(err)
	}
	if subj := strings.SplitN(headLog(t, s.dir)[0].Message, "\n", 2)[0]; subj != "hedgehug: add TST-1 x" {
		t.Errorf("subject = %q", subj)
	}
}

func TestAssignKeysCountsFormerAndNumbers(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	old := newTaskIn(ws, "old")
	old.Key, old.FormerKeys = "TST-1", []string{"TST-20"}
	put(t, s, old)
	gone := newTaskIn(ws, "gone")
	gone.Key = "TST-30"
	put(t, s, gone)
	if _, err := s.Update(Op{Name: "purge", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveTask(gone.ID) }); err != nil {
		t.Fatal(err)
	}
	n := newTaskIn(ws, "new")
	put(t, s, n)
	if n.Key != "TST-31" {
		t.Errorf("key = %s, want TST-31 (after the purged TST-30)", n.Key)
	}
}

func TestRenumberOnPrefixMismatch(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	api := &model.Workspace{ID: model.NewID(), Name: "api", KeyPrefix: "API"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(api) }); err != nil {
		t.Fatal(err)
	}
	x := newTaskIn(ws, "x")
	put(t, s, x)
	x.Workspace = api.ID
	put(t, s, x)
	if x.Key != "API-1" || !slices.Equal(x.FormerKeys, []string{"TST-1"}) {
		t.Errorf("key = %s, former = %v", x.Key, x.FormerKeys)
	}
	y := newTaskIn(ws, "y")
	put(t, s, y)
	if y.Key != "TST-2" {
		t.Errorf("TST-1 was reused: %s", y.Key)
	}
}

func TestKeyNumbersHook(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	calls := 0
	s.SetKeyNumbers(func() (map[string]int, error) {
		calls++
		return map[string]int{"TST": 41}, nil
	})
	x := newTaskIn(ws, "x")
	put(t, s, x)
	if x.Key != "TST-42" || calls != 1 {
		t.Errorf("key = %s, calls = %d", x.Key, calls)
	}
	put(t, s, x) // has a key that fits: no count
	if calls != 1 {
		t.Errorf("counted for a task that keeps its key")
	}
}

func TestConcurrentAddsGetDistinctKeys(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	other, err := Open(s.dir) // a second handle: only the file lock serializes them
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	tasks := make([]*model.Task, 20)
	for i := range tasks {
		tasks[i] = newTaskIn(ws, fmt.Sprint(i))
		h := s
		if i%2 == 1 {
			h = other
		}
		wg.Add(1)
		go func(h *Store, task *model.Task) {
			defer wg.Done()
			if _, err := h.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(task) }); err != nil {
				t.Error(err)
			}
		}(h, tasks[i])
	}
	wg.Wait()
	seen := map[string]bool{}
	for _, task := range tasks {
		if seen[task.Key] {
			t.Errorf("key %s given twice", task.Key)
		}
		seen[task.Key] = true
	}
}
```

- [ ] **Step 2: 失敗を確かめる** — `go test ./internal/store/ -run 'Assign|Renumber|KeyNumbers|Concurrent'`

- [ ] **Step 3: keys.go を書く**

```go
package store

import (
	"errors"

	"github.com/kukv/hedgehug/internal/model"
)

// KeyNumbers returns, for each key prefix, the largest number used by the
// keys and former keys of the tasks as HEAD has them. Update calls it under
// the lock, so HEAD does not move while it counts.
type KeyNumbers func() (map[string]int, error)

// SetKeyNumbers makes Update count numbers with f instead of reading every
// task file. The index sets it.
func (s *Store) SetKeyNumbers(f KeyNumbers) { s.keyNumbers = f }

// usedNumbers counts the numbers in use: the tasks' keys and former keys,
// and the record of purged numbers.
func (tx *Tx) usedNumbers() (map[string]int, error) {
	var out map[string]int
	if f := tx.s.keyNumbers; f != nil {
		m, err := f()
		if err != nil {
			return nil, err
		}
		out = m
	} else {
		out = map[string]int{}
		for t, err := range tx.s.Tasks() {
			var de *model.DecodeError
			if errors.As(err, &de) {
				continue
			}
			if err != nil {
				return nil, err
			}
			for _, k := range append([]string{t.Key}, t.FormerKeys...) {
				if p, n, ok := model.ParseKey(k); ok && n > out[p] {
					out[p] = n
				}
			}
		}
	}
	if tx.numbers == nil {
		n, err := tx.s.Numbers()
		if err != nil {
			return nil, err
		}
		tx.numbers = n
	}
	for p, n := range tx.numbers {
		if n > out[p] {
			out[p] = n
		}
	}
	return out, nil
}

// assignKeys gives a key to each task put in the operation that has none,
// or whose key's prefix is not its workspace's. The old key goes to the
// former keys. Tasks without a workspace that exists are left to validate.
func (tx *Tx) assignKeys() error {
	var used map[string]int
	for _, id := range tx.taskOrder {
		t := tx.tasks[id]
		if t == nil || t.Workspace.IsZero() {
			continue
		}
		ws, err := tx.workspace(t.Workspace)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if p, _, ok := model.ParseKey(t.Key); ok && p == ws.KeyPrefix {
			continue
		}
		if used == nil {
			if used, err = tx.usedNumbers(); err != nil {
				return err
			}
		}
		used[ws.KeyPrefix]++
		if t.Key != "" && !slices.Contains(t.FormerKeys, t.Key) {
			t.FormerKeys = append(t.FormerKeys, t.Key)
		}
		t.Key = model.FormatKey(ws.KeyPrefix, used[ws.KeyPrefix])
	}
	return nil
}
```

（`tx.workspace` は Task 2 の `Workspace` から `done` の検査を除いた内部版。無ければ Task 2 の実装を `Workspace`/`workspace` に分ける。`slices` を import。`Tasks()` が `DecodeError` 以外の壊れ方を返すなら、既存の `Tasks` の扱いに合わせる）

`store.go` の `Store` に `keyNumbers KeyNumbers` を足す。`tx.go` の `Update` で `err = fn(tx)` の成功後、`tx.done = true` の前に `if err := tx.assignKeys(); err != nil { return false, err }` を入れる（`tx.workspace` は `done` に関係なく読めること）。

- [ ] **Step 4: 通るのを確かめる** — `go test -race ./internal/store/`

- [ ] **Step 5: コミット**

```bash
git add internal/store
git commit -m "feat: タスクを書くときに Store のロックの中でキーを振る"
```

---

### Task 4: index と query — ワークスペース、最大番号、Resolve から末尾一致を外す

**Files:**
- Create: `internal/index/resolve.go`, `internal/index/resolve_test.go`, `internal/index/workspace.go`, `internal/index/workspace_test.go`
- Delete: `internal/index/shortid.go`, `internal/index/shortid_test.go`
- Modify: `internal/index/index.go`, `internal/index/entry.go`, `internal/index/build.go`, `internal/index/promised_test.go`, `internal/index/helpers_test.go`, `internal/query/eval.go`, `internal/query/eval_test.go`
- Modify（短縮 ID を使う呼び出しをこの Task で仮に `id.String()` へ置き換え、コンパイルを通す）: `cmd/hedgehug/*.go` の `ix.ShortID(x)` → `x.String()`、`index.ShortLengths` を使う `commentIndex` は Task 8 で直すので、ここでは `short` を使わず完全な ULID を返すようにする。`internal/doctor/*.go` の `ix.ShortID` も同様。最終的な表示は Task 8 で作る

**Interfaces:**
- Consumes: Task 2 の `Store.Workspaces`、Task 3 の `SetKeyNumbers`
- Produces:
  - `Entry.Workspace model.ID`
  - `func (ix *Index) Workspaces() []*model.Workspace`（名前順）、`func (ix *Index) Workspace(id model.ID) (*model.Workspace, bool)`
  - `func (ix *Index) ResolveWorkspace(s string) (*model.Workspace, error)` — 名前・接頭辞・完全な ULID（大文字小文字を区別しない）。無ければ `store.ErrNotFound` を包む。名前と接頭辞で別々のワークスペースに当たれば `*AmbiguousWorkspaceError{Input string; Candidates []*model.Workspace}`
  - `func (ix *Index) Resolve(s string) (model.ID, error)` — キー → 古いキー → 完全な ULID。末尾一致は無い
  - `func (ix *Index) KeyNumbers() (map[string]int, error)` — `Refresh` してから、全エントリの `Key` と `FormerKeys` の最大番号を返す。`Open` が `s.SetKeyNumbers(ix.KeyNumbers)` を呼ぶ
  - query: `workspace:<ws>`（`,` で OR、`-` で否定）

- [ ] **Step 1: テストを書く**

`resolve_test.go`（`shortid_test.go` の Resolve のテストのうち、キー・古いキー・ULID の部分を移し、末尾一致が引けないことを足す）:

```go
func TestResolve(t *testing.T) {
	s, _ := newStore(t)
	a := newTask("a")
	put(t, s, a) // gets TST-1
	b := newTask("b")
	b.FormerKeys = []string{"OLD-3"}
	put(t, s, b) // gets TST-2
	ix := mustOpen(t, s)
	for in, want := range map[string]model.ID{"TST-1": a.ID, "tst-2": b.ID, "old-3": b.ID, a.ID.String(): a.ID, strings.ToLower(b.ID.String()): b.ID} {
		if got, err := ix.Resolve(in); err != nil || got != want {
			t.Errorf("Resolve(%q) = %v %v", in, got, err)
		}
	}
	tail := a.ID.String()[22:]
	if _, err := ix.Resolve(tail); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("the end of a ULID resolves: %v", err)
	}
}
```

`helpers_test.go` の `newTask` はワークスペースが要るので、`newTask(title)` を `newTaskIn(s, title)` に変えるか、パッケージ変数で作成時のワークスペース ID を持たず、`put` の中で `Workspace` が zero のタスクに Store の最初のワークスペースを入れる（テスト用の補助として。本番の store は Task 5 で必須にする）。後者のほうが既存テストの変更が少ないので、`put` に入れる:

```go
func put(t *testing.T, s *store.Store, tasks ...*model.Task) {
	t.Helper()
	ws := firstWorkspace(t, s)
	if _, err := s.Update(store.Op{Name: "update", Author: testAuthor}, func(tx *store.Tx) error {
		for _, task := range tasks {
			if task.Workspace.IsZero() {
				task.Workspace = ws
			}
			if err := tx.PutTask(task); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func firstWorkspace(t *testing.T, s *store.Store) model.ID {
	t.Helper()
	for w, err := range s.Workspaces() {
		if err != nil {
			t.Fatal(err)
		}
		return w.ID
	}
	t.Fatal("the store has no workspace")
	return model.ID{}
}
```

（`query` と `doctor` の helpers_test.go の `put` も同じに直す。`rawTask` で書くファイルは Task 7 の doctor のテスト以外では `workspace:` を足す必要はない）

`workspace_test.go`:

```go
func TestWorkspacesAndResolveWorkspace(t *testing.T) {
	s, _ := newStore(t)
	api := &model.Workspace{ID: model.NewID(), Name: "api", KeyPrefix: "API"}
	if _, err := s.Update(store.Op{Name: "workspace", Author: testAuthor}, func(tx *store.Tx) error { return tx.PutWorkspace(api) }); err != nil {
		t.Fatal(err)
	}
	ix := mustOpen(t, s)
	if ws := ix.Workspaces(); len(ws) != 2 || ws[0].Name != "api" || ws[1].Name != "test" {
		t.Fatalf("Workspaces = %+v", ws)
	}
	for _, in := range []string{"api", "API", "Api", api.ID.String()} {
		if w, err := ix.ResolveWorkspace(in); err != nil || w.ID != api.ID {
			t.Errorf("ResolveWorkspace(%q) = %v %v", in, w, err)
		}
	}
	if _, err := ix.ResolveWorkspace("nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("nope: %v", err)
	}
	x := newTask("x")
	x.Workspace = api.ID
	put(t, s, x)
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}
	if e, _ := ix.Task(x.ID); e.Workspace != api.ID || e.Key != "API-1" {
		t.Errorf("entry = %+v", e)
	}
	if n, err := ix.KeyNumbers(); err != nil || n["API"] != 1 || n["TST"] != 0 {
		t.Errorf("KeyNumbers = %v %v", n, err)
	}
}
```

query の `eval_test.go` に、ワークスペースを2つ作って `workspace:api`、`workspace:API,test`、`-workspace:api` を確かめるテストを、既存の `project:` のテストと同じ書き方で足す。

- [ ] **Step 2: 失敗を確かめる** — `go test ./internal/index/ ./internal/query/`

- [ ] **Step 3: 実装する**

- `entry.go`: `Entry` に `Workspace model.ID` を `FormerKeys` の後に足し、`EntryOf` で `Workspace: t.Workspace`
- `index.go`: `Index` から `short` を消し、`workspaces map[model.ID]*model.Workspace` を足す。`derive` で `short` を作っている処理を消す。`Open` の最後（成功時）に `s.SetKeyNumbers(ix.KeyNumbers)`
- `build.go`: `cacheFormat` を1つ上げる。`cache` に `Workspaces []*model.Workspace` を足して保存・読み込みする（gob で `model.Workspace` の非公開 `doc` は無視される。表示に使うのは公開フィールドだけなので問題ない）。`rebuild` と `update` で、`workspaces/` の下が変わったとき（`rebuild` では常に）`ix.readWorkspaces(tree)` で全ワークスペースを読み直す:

```go
// readWorkspaces decodes every workspace file of tree. A file that does not
// decode is skipped; doctor reports it.
func (ix *Index) readWorkspaces(tree plumbing.Hash) error {
	out := map[model.ID]*model.Workspace{}
	for p, err := range ix.repo.Files(tree, "workspaces/") {
		if err != nil {
			return err
		}
		if _, err := model.ParseWorkspacePath(p); err != nil {
			continue
		}
		b, err := ix.repo.ReadFile(tree, p)
		if err != nil {
			return err
		}
		if w, err := model.DecodeWorkspace(b); err == nil {
			out[w.ID] = w
		}
	}
	ix.workspaces = out
	return nil
}
```

  `update` では `changes` のループで `strings.HasPrefix(c.Path, "workspaces/")` を見つけたら読み直しの印を立て、ループの後で読み直す
- `workspace.go`: `Workspaces`（名前順、同名は ID 順）、`Workspace`、`ResolveWorkspace`、`AmbiguousWorkspaceError`。`ResolveWorkspace` は、26文字なら ULID 一致、それ以外は `strings.EqualFold` で名前または接頭辞の一致。当たったワークスペースが1つならそれ、2つ以上なら `AmbiguousWorkspaceError`
- `resolve.go`: `shortid.go` から `AmbiguousError` と `Resolve` を移し、4段目（末尾一致）と `minShort` を消す。doc コメントは「a key, a former key, then a full ULID」に直す。`shortid.go` と `shortid_test.go` を削除（`ShortLengths` のテストも消える）
- `KeyNumbers`:

```go
// KeyNumbers returns, for each key prefix, the largest number of the keys
// and former keys of the tasks at HEAD, deleted ones included. It is the
// store's KeyNumbers: Update calls it under the lock.
func (ix *Index) KeyNumbers() (map[string]int, error) {
	if err := ix.Refresh(); err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, e := range ix.tasks {
		for _, k := range append([]string{e.Key}, e.FormerKeys...) {
			if p, n, ok := model.ParseKey(k); ok && n > out[p] {
				out[p] = n
			}
		}
	}
	return out, nil
}
```

- `query/eval.go`: `case "workspace": return c.workspace(t)` を足し、`c.project` と同じ作りで、値ごとに `c.ix.ResolveWorkspace(v)`（解決できなければ `c.fail`）して `e.Workspace` と比べる述語を作る
- 呼び出し側の仮置き（Interfaces の上の Modify 参照）: `cmd/hedgehug` と `internal/doctor` の `ix.ShortID(id)` を `id.String()` に、`commentIndex` は `short` フィールドと `Short` を消して `id.String()` を使い、`Resolve` の末尾一致の分岐を消す。テストで `.Short` を期待しているものは、この時点では `ID` を使うように直す（最終の形は Task 8 で決まるので最小限に）

- [ ] **Step 4: 通るのを確かめる** — `go build ./... && go test ./...`

- [ ] **Step 5: コミット**

```bash
git add -A internal/index internal/query internal/doctor cmd/hedgehug
git commit -m "feat: index にワークスペースと最大番号を足し、Resolve から末尾一致を外す"
```

---

### Task 5: store — ワークスペースの書き込み時の検査

**Files:**
- Modify: `internal/store/validate.go`, `internal/store/tx.go`（`commitFor`）, `internal/store/*_test.go`（タスクにワークスペースを入れる）, `internal/store/validate_test.go`
- Modify（必要なら）: `cmd/hedgehug/add.go`（この時点で add がワークスペースを入れていないとテストが落ちる。Task 6 の前なので、ここでは「親があれば親の、なければ Store のワークスペースが1つならそれ」を入れる最小の実装を add に入れ、Task 6 で `--workspace` を足す）

**Interfaces:**
- Consumes: Task 2〜3
- Produces: `Update` は次を `*ValidationError`（`Violations` に `schema.Violation{Code: ..., Field: ..., Msg: ...}`）または通常のエラーで拒否する。コードは `store` に定数で置く:
  - `CodeNoWorkspace = "no_workspace"`（`workspace` が無い・存在しない）
  - `CodeWorkspaceMismatch = "workspace_mismatch"`（親と違う）
  - ワークスペースの検査（`PutWorkspace` されたもの）: 接頭辞の形式、名前の形式（`config.ValidStoreName` と同じ規則。store から config は import できないので、同じ正規表現を `model.ValidWorkspaceName` として model に置き、config から使うかは触らない）、Store の中での接頭辞の重複、名前の重複（大文字小文字を区別しない）→ `fmt.Errorf` のエラー
  - `RemoveWorkspace` されたワークスペースに属するタスク（削除済みを含む、この操作で書いたものを優先）が残っている → エラー `workspace %s still has %d tasks`

- [ ] **Step 1: テストを書く**（`validate_test.go` に足す）

```go
func TestValidateWorkspace(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	write := func(task *model.Task) error {
		_, err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(task) })
		return err
	}
	var ve *ValidationError
	if err := write(&model.Task{ID: model.NewID(), Kind: "task", Title: "x", Status: "todo"}); !errors.As(err, &ve) || ve.Violations[0].Code != CodeNoWorkspace {
		t.Errorf("no workspace: %v", err)
	}
	if err := write(&model.Task{ID: model.NewID(), Workspace: model.NewID(), Kind: "task", Title: "x", Status: "todo"}); !errors.As(err, &ve) || ve.Violations[0].Code != CodeNoWorkspace {
		t.Errorf("unknown workspace: %v", err)
	}
	api := &model.Workspace{ID: model.NewID(), Name: "api", KeyPrefix: "API"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(api) }); err != nil {
		t.Fatal(err)
	}
	parent := newTaskIn(ws, "p")
	put(t, s, parent)
	child := newTaskIn(api, "c")
	child.Parent = &parent.ID
	if err := write(child); !errors.As(err, &ve) || ve.Violations[0].Code != CodeWorkspaceMismatch {
		t.Errorf("mismatch: %v", err)
	}
	dup := &model.Workspace{ID: model.NewID(), Name: "other", KeyPrefix: "API"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(dup) }); err == nil {
		t.Error("a duplicate prefix is accepted")
	}
	same := &model.Workspace{ID: model.NewID(), Name: "API", KeyPrefix: "APX"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(same) }); err == nil {
		t.Error("a duplicate name is accepted")
	}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveWorkspace(ws.ID) }); err == nil {
		t.Error("a workspace with tasks is removed")
	}
}
```

- [ ] **Step 2: 失敗を確かめる** — `go test ./internal/store/ -run TestValidateWorkspace`

- [ ] **Step 3: 実装する**

`validate.go` の `validate` のタスクのループで、`ValidateWrite` の結果に `tx.workspaceViolations(t)` を足す:

```go
// workspaceViolations checks that t belongs to a workspace that exists, the
// one of its parent.
func (tx *Tx) workspaceViolations(t *model.Task) ([]schema.Violation, error) {
	if t.Workspace.IsZero() {
		return []schema.Violation{{Code: CodeNoWorkspace, Field: "workspace", Msg: "missing"}}, nil
	}
	if _, err := tx.workspace(t.Workspace); errors.Is(err, ErrNotFound) {
		return []schema.Violation{{Code: CodeNoWorkspace, Field: "workspace", Msg: fmt.Sprintf("workspace %s is not in this store", t.Workspace)}}, nil
	} else if err != nil {
		return nil, err
	}
	if t.Parent == nil {
		return nil, nil
	}
	p, err := tx.task(*t.Parent)
	if errors.Is(err, ErrNotFound) || errors.As(err, new(*model.DecodeError)) {
		return nil, nil // checkRefs deals with a missing parent
	}
	if err != nil {
		return nil, err
	}
	if p.Workspace != t.Workspace {
		return []schema.Violation{{Code: CodeWorkspaceMismatch, Field: "workspace", Msg: "differs from the workspace of the parent; move the task with hedgehug mv --workspace"}}, nil
	}
	return nil, nil
}
```

`validate` の最初で `tx.validateWorkspaces()` を呼ぶ: `tx.workspaces` の各エントリについて、値があれば形式（`model.ValidKeyPrefix`、`model.ValidWorkspaceName`）と、`tx.s.Workspaces()` に `tx.workspaces` を重ねた一覧の中での接頭辞・名前の重複を調べる。値が nil（消す）なら、`tx.s.Tasks()` を走査し（この操作で書いたタスクは `tx.tasks` を優先、消したものは除く）、`Workspace` が一致する件数が1以上ならエラー。

`model.ValidWorkspaceName` は `config.ValidStoreName` の規則（`internal/config/config.go:189`）を写して model に置く。

`tx.go` の `commitFor`: `label` は採番の後なので `t.Key` がある。`t.Key == ""` のときに ULID で代用する分岐は残してよい（壊れたタスクを直す書き込みなど、ワークスペースが無く検査で拒否される場合はコミットまで行かない）が、コメントを「has none」から実態に合わせる必要はない。変更しない。

既存テストの直し: `internal/store` のテストで `model.Task{...}` を `PutTask` しているもの全部に、`Workspace: testWorkspace(t, s).ID` を足す（`grep -n 'model.Task{' internal/store/*_test.go` で洗い出す）。`cmd/hedgehug/add.go` は、`t.Workspace` を「親があれば親の `Workspace`、なければ `ix.Workspaces()` が1つならその ID、それ以外は `errors.New("there are several workspaces; give one with --workspace")`」で埋める（index の `Entry.Workspace` を使う）。`cmd/hedgehug` のテストは `store init` が1つのワークスペースを作るので、これで通る。

- [ ] **Step 4: 通るのを確かめる** — `go test ./...`

- [ ] **Step 5: コミット**

```bash
git add -A internal/store internal/model cmd/hedgehug
git commit -m "feat: タスクのワークスペースとワークスペースの定義を書き込み時に検査する"
```

---

### Task 6: CLI — store init --prefix、workspace コマンド、add --workspace、他の Store との照合

**Files:**
- Create: `cmd/hedgehug/workspace.go`, `cmd/hedgehug/workspace_test.go`
- Modify: `cmd/hedgehug/store_init.go`, `cmd/hedgehug/store_init_test.go`, `cmd/hedgehug/add.go`, `cmd/hedgehug/add_test.go`（`newStoreEnv` を `store init me --prefix ME` に）, `cmd/hedgehug/app.go`（`rootCmd` に `workspaceCmd` を登録）

**Interfaces:**
- Consumes: Task 4 の `ix.Workspaces`/`ResolveWorkspace`、Task 5 の検査
- Produces:
  - `hh store init <name> --prefix <P> [--workspace <name>] [--template dev]`。`--prefix` が無ければ usage エラー `--prefix is required: the key prefix of the first workspace, such as WEB`
  - `hh workspace add <name> --prefix <P>`、`ls`、`rename <ws> <name>`、`prefix <ws> <P>`、`rm <ws>`。すべて `--json` 対応
  - `func (a *app) checkPrefixElsewhere(prefix, self string) error` — config の Store のうち名前が `self` でないものを `store.Open` し、`Workspaces()` に同じ接頭辞（大文字小文字を区別しない）があれば `fmt.Errorf("key prefix %s is used by workspace %s of store %s", ...)`。開けない Store は `fmt.Fprintf(a.stderr, "warning: store %s is not checked for key prefixes: %v\n", ...)` で飛ばす
  - `hh add --workspace <ws>`

**JSON の形**（`workspace ls --json` と各コマンドの `--json`）:

```go
type workspaceJSON struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	KeyPrefix string    `json:"key_prefix"`
	PrefixSet time.Time `json:"prefix_set"`
	Tasks     int       `json:"tasks"` // not deleted
}
```

**表示**:
- `workspace ls`: 表 `NAME  PREFIX  ID  TASKS`（`writeTable` を使う）
- `workspace add`: `Added workspace web (WEB, 01J...)`
- `workspace rename`: `Renamed workspace web to front (WEB, 01J...)`
- `workspace prefix`: `Changed the key prefix of web to FE (01J...); renumbered N tasks` と、その後に1行ずつ `  WEB-3 → FE-12`（古いキー順）
- `workspace rm`: `Removed workspace web (WEB, 01J...)`。タスクが残っていれば `workspace web still has N tasks; move them with hedgehug mv --workspace or purge them`

- [ ] **Step 1: テストを書く**（`workspace_test.go` と `store_init_test.go`）

```go
func TestStoreInitNeedsPrefix(t *testing.T) {
	te := newTestEnv(t)
	if code, _, errs := te.run("store", "init", "me"); code == 0 || !strings.Contains(errs, "--prefix") {
		t.Errorf("init without --prefix: %d %s", code, errs)
	}
	te.ok("store", "init", "me", "--prefix", "ME")
	out := te.ok("workspace", "ls")
	if !strings.Contains(out, "me") || !strings.Contains(out, "ME") {
		t.Errorf("workspace ls =\n%s", out)
	}
	if code, _, errs := te.run("store", "init", "other", "--prefix", "me"); code == 0 || !strings.Contains(errs, "prefix") {
		t.Errorf("bad prefix: %d %s", code, errs)
	}
	if code, _, errs := te.run("store", "init", "other", "--prefix", "ME"); code == 0 || !strings.Contains(errs, "store me") {
		t.Errorf("prefix of another store: %d %s", code, errs)
	}
	te.ok("store", "init", "other", "--prefix", "OT", "--workspace", "main")
}

func TestWorkspaceCommands(t *testing.T) {
	te := newStoreEnv(t) // me, prefix ME
	te.ok("workspace", "add", "api", "--prefix", "API")
	if code, _, errs := te.run("add", "x"); code == 0 || !strings.Contains(errs, "--workspace") {
		t.Errorf("add with two workspaces: %d %s", code, errs)
	}
	a := te.addJSON("a", "--workspace", "api")
	if a.Key != "API-1" {
		t.Errorf("key = %s", a.Key)
	}
	child := te.addJSON("c", "--parent", a.Key)
	if child.Key != "API-2" {
		t.Errorf("child key = %s", child.Key)
	}
	if code, _, _ := te.run("add", "y", "--parent", a.Key, "--workspace", "me"); code == 0 {
		t.Error("a parent in another workspace is accepted")
	}
	out := te.ok("workspace", "prefix", "api", "BE")
	if !strings.Contains(out, "API-1 → BE-1") || !strings.Contains(out, "API-2 → BE-2") {
		t.Errorf("prefix =\n%s", out)
	}
	if got := te.task(a.ID); got.Key != "BE-1" || !slices.Equal(got.FormerKeys, []string{"API-1"}) {
		t.Errorf("after prefix: %s %v", got.Key, got.FormerKeys)
	}
	te.ok("show", "API-1") // a former key still resolves
	te.ok("workspace", "rename", "be", "backend")
	if code, _, errs := te.run("workspace", "rm", "backend"); code == 0 || !strings.Contains(errs, "2 tasks") {
		t.Errorf("rm with tasks: %d %s", code, errs)
	}
	te.ok("workspace", "add", "empty", "--prefix", "EM")
	te.ok("workspace", "rm", "EM")
	if code, _, _ := te.run("workspace", "add", "dup", "--prefix", "BE"); code == 0 {
		t.Error("a duplicate prefix is accepted")
	}
}
```

- [ ] **Step 2: 失敗を確かめる** — `go test ./cmd/hedgehug/ -run 'TestStoreInitNeedsPrefix|TestWorkspaceCommands'`

- [ ] **Step 3: 実装する**

- `store_init.go`: `--prefix`（必須）と `--workspace` のフラグを足す。`model.ValidKeyPrefix` を usage エラーで返す。ワークスペース名は省略時 Store 名。`a.checkPrefixElsewhere(prefix, name)` を Create の前に呼ぶ。`store.Create(dir, meta, sc, &model.Workspace{ID: model.NewID(), Name: wsName, KeyPrefix: prefix}, author)`。`--json` の出力に `"workspace": {"id","name","key_prefix"}` を足す。テキストは `Created store %s at %s with workspace %s (%s)`
- `workspace.go`: `workspaceCmd` と5つのサブコマンド。各コマンドは `a.open()` → `ix.ResolveWorkspace(arg)`（`AmbiguousWorkspaceError` は候補を `name (PREFIX, ULID)` で並べたエラーに）→ `a.update(s, store.Op{Name: "workspace"}, ...)`
  - `add`: `ValidKeyPrefix` → `checkPrefixElsewhere(prefix, a.storeName の実際の名前)`（`c.Select(a.storeName)` で得た Entry の Name）→ `tx.PutWorkspace`
  - `rename`: `tx.Workspace(id)` を読み、`Name` を変えて `PutWorkspace`
  - `prefix`: `ValidKeyPrefix` と `checkPrefixElsewhere` → `tx.Workspace(id)` の `KeyPrefix` と `PrefixSet = a.now()`（`env.now` を使う。無ければ `time.Now()`）を変えて `PutWorkspace`。続けて `ix.Tasks()` のうち `e.Workspace == id` の全タスク（削除済みを含む、壊れたものは除く）を `tx.Task(e.ID)` で読んで `PutTask` する（`assignKeys` が振り直す）。振り直した対応を表示するため、`PutTask` の前のキーを控え、`Update` の後に `tx` に渡したタスクの `Key` を読む
  - `rm`: `ix.Tasks()` で件数を数え、1以上なら上のエラー。0 なら `tx.RemoveWorkspace`
  - `ls`: `ix.Workspaces()` と、ワークスペースごとの削除済みでないタスク数
- `add.go`: `--workspace` フラグ。Task 5 の仮の決め方を次に置き換える: `--parent` があれば親の `Workspace`（`--workspace` が別ならエラー `the parent is in workspace %s`）。無ければ `--workspace` を `ix.ResolveWorkspace`。どちらも無ければ `ix.Workspaces()` が1つならそれ、2つ以上なら `there are several workspaces; give one with --workspace: ` に名前と接頭辞を並べたエラー
- `app.go`: `cmd.AddCommand(a.workspaceCmd())`
- `add_test.go` の `newStoreEnv`: `te.ok("store", "init", "me", "--prefix", "ME")`。他のテストで `store init` を直接呼んでいる箇所（`grep -rn '"store", "init"' cmd/hedgehug`）にも `--prefix` を足す

- [ ] **Step 4: 通るのを確かめる** — `go test ./cmd/hedgehug/`

- [ ] **Step 5: コミット**

```bash
git add -A cmd/hedgehug
git commit -m "feat: store init に --prefix を、workspace コマンドと add --workspace を足す"
```

---

### Task 7: CLI — mv --workspace、edit の制限

**Files:**
- Modify: `cmd/hedgehug/mv.go`, `cmd/hedgehug/mv_test.go`, `cmd/hedgehug/edit.go`, `cmd/hedgehug/edit_test.go`

**Interfaces:**
- Consumes: Task 4〜6
- Produces:
  - `hh mv <id> --workspace <ws> [--parent <p>] [--before/--after]` — `<id>` と全子孫（削除済みを含む、`descendants(ix, id, func(*index.Entry) bool { return true })`）の `Workspace` を変え、同じ操作で `PutTask` する。`--parent` が無ければルートへ。`--parent` が別のワークスペースのタスクなら、`--workspace` が無くてもその親のワークスペースへ配下ごと移す。`--workspace` と `--parent` のワークスペースが食い違えばエラー
  - ワークスペースが変わったとき、確認メッセージの後に `  WEB-3 → API-12` を1行ずつ（移したタスクすべて、元の木の順）
  - `checkEdit`: `former_keys` の変更を拒否（`fixed("former_keys")`）、`workspace` の変更を `moved("workspace")` と同じ形で拒否し、メッセージは `workspace cannot be changed here; use hedgehug mv --workspace`

- [ ] **Step 1: テストを書く**

```go
func TestMvWorkspace(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("workspace", "add", "api", "--prefix", "API")
	p := te.addJSON("p", "--workspace", "me")   // ME-1
	c := te.addJSON("c", "--parent", p.Key)     // ME-2
	out := te.ok("mv", p.Key, "--workspace", "api")
	if !strings.Contains(out, "ME-1 → API-1") || !strings.Contains(out, "ME-2 → API-2") {
		t.Errorf("mv =\n%s", out)
	}
	got := te.task(c.ID)
	if got.Key != "API-2" || got.Parent == nil || got.Parent.String() != p.ID {
		t.Errorf("child after mv: %+v", got)
	}
	other := te.addJSON("o", "--workspace", "me") // ME-3: ME-1 and ME-2 are not reused
	if other.Key != "ME-3" {
		t.Errorf("key = %s", other.Key)
	}
	te.ok("mv", other.Key, "--parent", p.Key) // into api by its parent
	if got := te.task(other.ID); got.Key != "API-3" {
		t.Errorf("mv --parent across workspaces: %s", got.Key)
	}
}
```

`edit_test.go` に、エディタで `workspace:` の行を書き換えると `use hedgehug mv --workspace` で拒否され、`former_keys:` を書き換えると `former_keys cannot be changed` で拒否されるテストを、既存の `key cannot be changed` のテストと同じ書き方で足す。

- [ ] **Step 2: 失敗を確かめる** — `go test ./cmd/hedgehug/ -run 'TestMvWorkspace|TestEdit'`

- [ ] **Step 3: 実装する**

`mv.go`: `--workspace` フラグを足す。`mv` の中で、移動先のワークスペース `dest` を決める（`--parent` があればその親の `Workspace`、`--workspace` があれば `ResolveWorkspace` の結果。両方あって違えばエラー `the parent is in workspace %s`。`--workspace` だけで `--parent` も `--root` も無いときは `parent = nil`）。`dest != e.Workspace` なら、`e` と全子孫を対象にし、`Update` の中で各タスクを `tx.Task` で読み、`Workspace = dest` にして `PutTask`（`e` 自身は `Parent` と `Rank` も変える）。`Update` の前に各タスクの古いキーを控え、`Update` の後に `ix.Refresh()` して新しいキーを読み、`a.printTask` の後に対応を表示する。`--json` のときは `printTask` の JSON だけ（対応は出さない）。

`edit.go` の `checkEdit` の `switch` に足す:

```go
	case !slices.Equal(t.FormerKeys, old.FormerKeys):
		return nil, fixed("former_keys")
	case t.Workspace != old.Workspace:
		return nil, moved("workspace", "hedgehug mv --workspace")
```

（`moved` が引数を1つしか取らないなら、`workspace` 用に `fmt.Errorf("workspace cannot be changed here; use hedgehug mv --workspace")` を直接返す）

- [ ] **Step 4: 通るのを確かめる** — `go test ./cmd/hedgehug/`

- [ ] **Step 5: コミット**

```bash
git add cmd/hedgehug
git commit -m "feat: mv --workspace で配下ごと移して振り直し、edit でワークスペースと古いキーを変えさせない"
```

---

### Task 8: CLI — 表示をすべてキーと ULID に、短縮 ID を消す

**Files:**
- Modify: `cmd/hedgehug/output.go`, `tree.go`, `show.go`, `log.go`, `ls.go`, `rm.go`, `migrate.go`, `comment.go`, `app.go`, `edit.go`, `status.go`, `dep.go`, `set.go`、およびそれぞれの `_test.go`（`.Short` を使うテストをすべて直す: `grep -rn 'Short' cmd/hedgehug/*_test.go`）
- Modify: `internal/doctor/doctor.go`, `internal/doctor/schema.go`（`Where` の表示）

**Interfaces:**
- Consumes: Task 4 の `Entry.Workspace`、`ix.Workspace`
- Produces（`output.go`）:

```go
// taskLabel names a task for a reader: its key and full ULID, or the ULID
// alone when the task has no key (a file that does not decode).
func taskLabel(e *index.Entry) string {
	if e.Key == "" {
		return e.ID.String()
	}
	return e.Key + " (" + e.ID.String() + ")"
}
```

  - `refName(ix, id)` は `taskLabel(e) + " " + e.Title`（無ければ `id.String() + " (not found)"`）
  - `taskJSON`: `Short` を削除。`Workspace *workspaceRefJSON \`json:"workspace"\``（`{"id","name","key_prefix"}`。ワークスペースが index に無ければ `{"id": <ULID>, "name": "", "key_prefix": ""}`、ID が zero なら `null`）を `Key` の後に足す。`BlockedBy` は `[]refJSON` に変える（`refJSON{ID, Key, Title}`。`show.go` の `refJSON` を `output.go` に移し、`Key` を足す）
  - `taskHeader = []string{"KEY", "ID", "STATUS", "KIND", "ASSIGNEES", "LABELS", "TITLE"}`、`taskRow` は `key, e.ID.String(), ...`（`key` は `e.Key`、壊れたタスクは `?`）。色付けは列 0 と 1
  - `printTask`: `fmt.Fprintf(a.stdout, "%s %s %s\n", verb, a.style().id(taskLabel(e)), e.Title)`
- `tree.go` の `treeLine`: 先頭を `taskLabel` ではなく `key + " " + ULID`（括弧なし。spec の例 `WEB-1 01JBZ7... [doing] ...`）。キーが無ければ `? ULID`
- `show.go`: 見出し行を `taskLabel(e) + "  " + e.Title`、`field("key", ...)` を消し、`field("workspace", name + " (" + prefix + ", " + ULID + ")")` と、`former_keys` があれば `field("former_keys", ...)` を足す。参照（parent・blocked_by・関連・子・逆参照）はすべて `refName`。`relation` の逆参照の JSON は `refJSON{ID, Key, Title}`
- `log.go`:

```go
// subjectNow gives the subject of a Hedgehug commit with the task's current
// key and ULID in place of the label it was written with: the subject is
// "hedgehug: <op> <label> <title>". Other subjects, and tasks the index does
// not have, keep the subject as it is.
func subjectNow(ix *index.Index, le gitrepo.LogEntry) string {
	op, ok1 := le.Trailer("Hedgehug-Op")
	tid, ok2 := le.Trailer("Hedgehug-Task")
	head := "hedgehug: " + op + " "
	rest, ok3 := strings.CutPrefix(le.Subject, head)
	if !ok1 || !ok2 || !ok3 {
		return le.Subject
	}
	id, err := model.ParseID(tid)
	if err != nil {
		return le.Subject
	}
	e, ok := ix.Task(id)
	if !ok {
		return le.Subject
	}
	_, title, _ := strings.Cut(rest, " ")
	return strings.TrimRight(head+taskLabel(e)+" "+title, " ")
}
```

  `logText(ix, entries)` は `subjectNow` を使う。`logJSON` に `Key *string \`json:"key"\``（引けたときの今のキー）を足し、`Subject` は加工しない。`show` の履歴も `logText` と同じ関数で出す
- `ls.go`: `--ids-only` の説明を `print only the full IDs`、出力を `e.ID.String()`
- `app.go` の `resolve`: 候補の行を `refName(ix, c)`。`ambiguousError.lines` のコメントを `"<key> (<ULID>) <title>" per candidate` に
- `rm.go`・`status.go`・`dep.go`・`set.go`・`migrate.go`: `ix.ShortID`（Task 4 で `id.String()` に仮置きしたもの）を `refName` / `taskLabel` に。`migrate` の `skipped` は ULID の文字列（JSON の `skipped` も ULID）、一覧は `taskLabel`
- `edit.go`・`comment.go` の `a.editFile(name, ...)`: タスクは `e.Key`（無ければ ULID）、コメントは `"comment-" + ULID`
- `comment.go`: `commentJSON.Short` を削除。`Resolve` は 26 文字の ULID だけ（それ以外は `comment %q: give the full ID of the comment: %w` で `ErrNotFound` を包む）。`ambiguousError` は ULID 完全一致で複数当たることは無いので、候補の分岐は消してよい。表示は `c.ID.String()`
- `doctor`: `Where` を `taskLabel` 相当（doctor は cmd を import できないので、`e.Key + " (" + e.ID.String() + ") " + e.Title`、キーが無ければ `e.ID.String() + " " + e.Title`）

- [ ] **Step 1: テストを直す・足す**

`ls_test.go` の `TestLs` を次の期待に変える（`a.Short` → `a.Key` と `a.ID`、ヘッダは `KEY`、`--ids-only` は ULID）:

```go
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "KEY") {
		t.Fatalf("ls =\n%s", out)
	}
	for _, s := range []string{a.Key, a.ID, "Alpha", "ui", b.Key, b.ID, "Beta", "done"} {
		if !strings.Contains(out, s) {
			t.Errorf("ls does not show %q:\n%s", s, out)
		}
	}
	...
	if out := te.ok("ls", "--ids-only"); out != a.ID+"\n"+b.ID+"\n" && out != b.ID+"\n"+a.ID+"\n" {
		t.Errorf("--ids-only = %q", out)
	}
```

`log_test.go` に足す:

```go
func TestLogShowsCurrentKey(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("workspace", "add", "api", "--prefix", "API")
	a := te.addJSON("Alpha", "--workspace", "me") // ME-1
	te.ok("status", a.Key, "done")
	te.ok("mv", a.Key, "--workspace", "api") // now API-1
	out := te.ok("log", "API-1")
	if strings.Contains(out, "ME-1") || !strings.Contains(out, "status API-1 ("+a.ID+") Alpha") {
		t.Errorf("log =\n%s", out)
	}
	var js []logJSON
	if err := json.Unmarshal([]byte(te.ok("log", "API-1", "--json")), &js); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(js[len(js)-1].Subject, "ME-1") || js[0].Key == nil || *js[0].Key != "API-1" {
		t.Errorf("--json keeps the subject and adds the key: %+v", js)
	}
}
```

`show_test.go`・`tree_test.go`・`add_test.go`（確認メッセージ `Added ME-1 (<ULID>) ...`）・`comment_test.go`（短い ID を使う入力が `give the full ID` で拒否される）・`rm_test.go`・`mv_test.go`・`edit_test.go`・`set_test.go`・`dep_test.go`・`status_test.go`・`migrate_test.go`・`scenario_test.go`・`promised_test.go`・`nochange_test.go` の `.Short` を、入力には `.Key`、表示の期待には `.Key` と `.ID` に置き換える。

`app` レベルで短縮 ID が残っていないことを確かめるテストを `main_test.go` に足す:

```go
func TestNoShortIDs(t *testing.T) {
	out, err := exec.Command("grep", "-rnE", `ShortID|ShortLengths|minShort|"short"|\.Short\b`, "--include=*.go", "../../cmd", "../../internal").CombinedOutput()
	if err == nil {
		t.Errorf("short IDs are still in the code:\n%s", out)
	}
}
```

（cobra の `Short:` フィールドに当たらないよう、正規表現は `\.Short\b` で `Short:` を除く。当たってしまう場合は `grep -v 'Short:'` を挟む）

- [ ] **Step 2: 失敗を確かめる** — `go test ./cmd/hedgehug/`

- [ ] **Step 3: 実装する** — 上の Interfaces の通り

- [ ] **Step 4: 通るのを確かめる** — `go test ./... && go vet ./...`

- [ ] **Step 5: コミット**

```bash
git add -A cmd/hedgehug internal/doctor
git commit -m "feat: 表示をすべてキーと ULID の併記にし、短縮 ID を消す"
```

---

### Task 9: doctor — キーとワークスペースの検査

**Files:**
- Modify: `internal/index/problems.go`, `internal/index/problems_test.go`, `internal/doctor/doctor.go`, `internal/doctor/doctor_test.go`

**Interfaces:**
- Consumes: Task 4 の index
- Produces（index の問題コード。`is:invalid` にも出る）:
  - `CodeNoKey = "no_key"`（キーが無い、または形式が違う）
  - `CodeDuplicateKey = "duplicate_key"`（他のタスクの `key` と同じ。両方に付ける）
  - `CodeKeyPrefix = "key_prefix"`（キーの接頭辞が所属ワークスペースと違う。Msg は `the key prefix is not the one of workspace %s (%s); the next write renumbers it`）
  - `store.CodeNoWorkspace`（`workspace` が無い、または index に無い）、`store.CodeWorkspaceMismatch`（親と違う）
  - doctor: `fileRoots` に `"workspaces"`、`.hedgehug/numbers.yaml` を `readCheck(dir, model.NumbersPath, model.DecodeNumbers)` で（無ければ飛ばす）、`indexCodes` に上の5つ、ワークスペース同士の接頭辞と名前の重複（`Check: "workspace"`）

- [ ] **Step 1: テストを書く**（`problems_test.go`）

```go
func TestKeyAndWorkspaceProblems(t *testing.T) {
	s, dir := newStore(t)
	ws := firstWorkspace(t, s)
	a := newTask("a")
	put(t, s, a) // TST-1
	b := model.NewID()
	commitRaw(t, s, dir, model.TaskPath(b), rawTask(b, "key: TST-1\nworkspace: "+ws.String()+"\nkind: task\ntitle: b\nstatus: todo\n"))
	c := model.NewID()
	commitRaw(t, s, dir, model.TaskPath(c), rawTask(c, "kind: task\ntitle: c\nstatus: todo\n"))
	d := model.NewID()
	commitRaw(t, s, dir, model.TaskPath(d), rawTask(d, "key: XX-1\nworkspace: "+ws.String()+"\nkind: task\ntitle: d\nstatus: todo\n"))
	ix := mustOpen(t, s)
	has := func(id model.ID, code string) bool {
		return slices.ContainsFunc(ix.Problems(id), func(p Problem) bool { return p.Code == code })
	}
	if !has(a.ID, CodeDuplicateKey) || !has(b, CodeDuplicateKey) {
		t.Error("duplicate keys are not found")
	}
	if !has(c, CodeNoKey) || !has(c, store.CodeNoWorkspace) {
		t.Error("a task without key and workspace is not found")
	}
	if !has(d, CodeKeyPrefix) {
		t.Error("a key with another prefix is not found")
	}
}
```

doctor のテストに、接頭辞が重複したワークスペースファイルを `commitRaw` 相当で置くと `Check: "workspace"` の Finding が出ること、壊れた `numbers.yaml` で `Check: "file"` が出ることを足す。

- [ ] **Step 2: 失敗を確かめる** — `go test ./internal/index/ ./internal/doctor/`

- [ ] **Step 3: 実装する**

`findProblems` の壊れていないエントリのループで:

```go
		if _, _, ok := model.ParseKey(e.Key); !ok {
			add(e.ID, Problem{CodeNoKey, "key", "missing or not a key such as WEB-42; the next write gives one"})
		}
		ws, wsOK := ix.workspaces[e.Workspace]
		switch {
		case e.Workspace.IsZero() || !wsOK:
			add(e.ID, Problem{store.CodeNoWorkspace, "workspace", "missing or not a workspace of this store; set it with hedgehug edit"})
		case e.Key != "":
			if p, _, ok := model.ParseKey(e.Key); ok && p != ws.KeyPrefix {
				add(e.ID, Problem{CodeKeyPrefix, "key", fmt.Sprintf("the key prefix is not the one of workspace %s (%s); the next write renumbers it", ws.Name, ws.KeyPrefix)})
			}
		}
		if e.Parent != nil {
			if p := ix.tasks[*e.Parent]; p != nil && p.Broken == "" && p.Workspace != e.Workspace {
				add(e.ID, Problem{store.CodeWorkspaceMismatch, "workspace", "differs from the workspace of the parent"})
			}
		}
```

ループの後で、`key` ごとに ID を集め、2件以上の key の全タスクに `CodeDuplicateKey`（`Msg: fmt.Sprintf("key %s is also the key of %s", ...)`）。

`doctor.go`: `fileRoots` に `"workspaces"`。`checkFile` が root ごとの decode を選んでいるなら `workspaces` → `model.DecodeWorkspace` を足す。Run の 1. の後に `numbers.yaml` の検査（`os.ReadFile` が `fs.ErrNotExist` なら飛ばす）。`checkStore` で `ix.Workspaces()` の接頭辞と名前（`strings.EqualFold`）の重複を `Finding{Check: "workspace", Where: model.WorkspacePath(id), Msg: ..., Hint: "change one with hedgehug workspace prefix (or rename)"}` で出す。`indexCodes` に5つのコードを足す。

- [ ] **Step 4: 通るのを確かめる** — `go test ./...`

- [ ] **Step 5: コミット**

```bash
git add -A internal/index internal/doctor
git commit -m "feat: doctor でキーとワークスペースの問題を知らせる"
```

---

### Task 10: 仕上げ — devenv、spec の差分、全体の確認

**Files:**
- Modify: `tools/devenv/main.go`（`store init` 相当の処理に接頭辞が要るなら `DEV` を使う。Task 2 の仮置きを見直し、テストを合わせる）、`tools/devenv/main_test.go`
- Modify: `docs/superpowers/specs/2026-10-31-phase1-5-workspace-keys-design.md`（下の差分）
- Modify: `README.md`, `README.ja.md`（`hh store init` の例があれば `--prefix` を足す）

- [ ] **Step 1: spec の差分を直す**

`## 採番の実装` の「書き込み時の検査」の最後の項目「採番の後にキーが他のタスクの `key` と重複している（採番の誤りに対する最後の砦）」を、次に置き換える:

```
- キーの重複は書き込み時には検査しない。振る番号は使われた最大より大きいので、採番が重複を生むことはない。手で編集したファイルや同期による重複は doctor（`duplicate_key`）で知らせる
```

`## index` 相当の記述として「問題コード」を `## doctor` に追記: `no_key`・`duplicate_key`・`key_prefix`・`no_workspace`・`workspace_mismatch` は index の問題で、`is:invalid` と `ls --json` の `problems` にも出る。

- [ ] **Step 2: 全体の確認**

Run: `go build ./... && go vet ./... && go test -race ./... && (make lint || true)`
Expected: すべて PASS。`make lint` があれば指摘ゼロ

Run: `grep -rnE 'ShortID|ShortLengths|minShort|KeyPrefix string|key_prefix' --include=*.go cmd internal tools | grep -v 'Workspace\|workspace'`
Expected: 何も出ない（`Workspace.KeyPrefix` と `key_prefix` のワークスペースのフィールドだけが残る）

手で動かす（`HEDGEHUG_HOME` を一時ディレクトリにして）:

```bash
export HEDGEHUG_HOME=$(mktemp -d)
go run ./cmd/hedgehug store init me --prefix ME
go run ./cmd/hedgehug add "最初のタスク"
go run ./cmd/hedgehug ls
go run ./cmd/hedgehug workspace add api --prefix API
go run ./cmd/hedgehug mv ME-1 --workspace api
go run ./cmd/hedgehug log
go run ./cmd/hedgehug show API-1
go run ./cmd/hedgehug doctor
```

Expected: `ls` に `KEY ID ...`、`mv` で `ME-1 → API-1`、`log` の件名が `API-1 (<ULID>)`、`show ME-1` も `show API-1` も同じタスク、doctor は問題なし

- [ ] **Step 3: コミット**

```bash
git add -A
git commit -m "docs: フェーズ1.5の spec を実装に合わせ、devenv と README を直す"
```
