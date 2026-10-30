# フェーズ 1a: model とファイル形式 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Store のファイル（タスク・コメント・作業時間・スプリント・リリース・テンプレート・`store.yaml`・`schema.yaml`）を、コメントと未知のキーを保ったまま読み書きする純粋な Go のコードと、スキーマの検証を作る。

**Architecture:** `internal/model` が型と codec を持ち、`yaml.Node` を保持する `Doc` と、書き戻し用の `Encoder` で「既知キーは決まった順・未知キーは元の順・変わっていない値は書いたままの形」を実現する。`internal/schema` は `model` の上に `schema.yaml` の型・検査・実効スキーマ・タスクの検証を載せる。ディスク I/O、ロック、git には触れない。

**Tech Stack:** Go 1.27.3、`go.yaml.in/yaml/v4@v4.0.0-rc.6`、`github.com/oklog/ulid/v2@v2.1.2`、標準の `testing`（table-driven、testify は使わない）。

**Spec:** `docs/superpowers/specs/2026-10-30-phase1a-model-format-design.md`

## Global Constraints

- 依存ライブラリは `go.yaml.in/yaml/v4` と `github.com/oklog/ulid/v2` だけ。`internal/model` は `internal/schema` に依存しない。
- ディスク I/O・ロック・git を使わない（テストの `testdata` 読み込みを除く）。
- コードのコメントと識別子は英語（既存の `cmd/hedgehug/main.go` に合わせる）。テストは標準の `testing` で table-driven に書く。
- YAML の書き出しは `yaml.Dump(root, yaml.WithLineWidth(-1))`（長いタイトルを折り返さない）。v4 の既定はインデント2・`- ` をキーと同じ桁に置く compact 形式。
- 改行は LF。読み込み時に CRLF を LF にそろえる。
- 日時は RFC 3339 でオフセット付き、日付は `YYYY-MM-DD`。duration は `h` と `m` だけ。
- Store 内のパスは常に `/` 区切り（git のパス）。
- 各タスクの最後に `make lint` と `go test -race ./...` を通してからコミットする。コミットメッセージは日本語で、末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` を付ける。

## 事前に確認済みの v4 の挙動（スパイク結果）

- `yaml.Load(src, &node)` と `yaml.Dump(&node)` の往復で、行末コメント・キー順・flow 形式（`[a, b]`、`{k: v}`）が保たれる。行末コメントの前の空白は1つに詰められる。
- 値 Node だけを差し替えると、キー側のコメントは残る。値側の `LineComment` は新しい Node に写せば残る。
- `Node.Encode([]string{...})` の Style は 0（block）。flow にしたいときは `Style = yaml.FlowStyle` を明示する。
- `2026-11-03` は `!!timestamp` に解決され、`string` へのデコードは失敗する。文字列のフィールドは Node の `Value` を直接読む。日付は Tag `!!timestamp` の Scalar Node を返す `MarshalYAML` で、引用符なしに書ける。
- `time.Time` は `2026-10-29T10:00:00+09:00` のまま往復する。
- `Marshaler` は `MarshalYAML() (any, error)`、`Unmarshaler` は `UnmarshalYAML(*yaml.Node) error`。
- 空の入力を `yaml.Load` すると `no documents in stream` のエラーになる。
- flow の中のスカラーに `:` が含まれると、変更がなくても `Dump` が引用符で囲む（`{where: 'project:…'}`）。block の中や、`github.com/…` のように `:` を含まない値はそのまま。正準形のテストデータを書くときはこの規則に合わせる。

## 前提ツール

`make lint` と `make check` は `golangci-lint`（v2）と `gotestsum` を使う。このマシンには mise で入っている（2026-10-30 に確認）。別の環境で実行する場合は、Task 1 の前に `which golangci-lint gotestsum` で確かめ、無ければ `go install gotest.tools/gotestsum@latest` と golangci-lint v2 の公式手順で入れる。

## File Structure

```
internal/model/
  errors.go        DecodeError
  id.go            ID（ULID）
  path.go          ID ⇔ Store 内の相対パス
  rank.go          Rank（fractional index）
  duration.go      Duration（h と m）
  date.go          Date（YYYY-MM-DD）
  frontmatter.go   splitFrontmatter / joinFrontmatter
  node.go          Field / Fields / NodeEqual / Doc / Encoder
  task.go          Task / Link / DecodeTask / TaskKeys
  comment.go       Comment / DecodeComment
  worklog.go       Worklog / DecodeWorklog
  sprint.go        Sprint / DecodeSprint
  release.go       Release / DecodeRelease
  template.go      Template / TemplateItem / Assignees / DecodeTemplate
  storemeta.go     StoreMeta / DecodeStoreMeta / StoreFormat
  *_test.go        各ファイルのテスト
  fuzz_test.go     frontmatter と rank の fuzz
internal/schema/
  schema.go        型・Lists・Props・DecodeSchema・Encode・Default・Dev・参照用メソッド
  check.go         (*Schema).Check
  effective.go     Effective / (*Schema).For
  validate.go      Violation / ValidateTask / ValidatePlacement / ValidateTransition
  *_test.go
```

---

### Task 1: 依存の追加と ID・DecodeError

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/model/errors.go`, `internal/model/id.go`
- Test: `internal/model/id_test.go`

**Interfaces:**
- Produces: `type ID struct`、`NewID() ID`、`ParseID(string) (ID, error)`、`(ID) String() string`、`(ID) IsZero() bool`、`(ID) MarshalYAML`、`(*ID) UnmarshalYAML`。`type DecodeError struct{ Field string; Line int; Err error }`。

- [ ] **Step 1: 依存を追加する**

Run:
```bash
go get go.yaml.in/yaml/v4@v4.0.0-rc.6 github.com/oklog/ulid/v2@v2.1.2
```
Expected: `go.mod` に2行が追加される（この時点では `// indirect`。Step 5 の `go mod tidy` で直接依存になる）。

- [ ] **Step 2: 失敗するテストを書く**

`internal/model/id_test.go`:
```go
package model

import (
	"errors"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestNewIDIsParseable(t *testing.T) {
	a, b := NewID(), NewID()
	if a.String() >= b.String() {
		t.Errorf("NewID is not increasing: %s then %s", a, b)
	}
	got, err := ParseID(a.String())
	if err != nil || got != a {
		t.Errorf("ParseID(%s) = %v, %v", a, got, err)
	}
}

func TestParseID(t *testing.T) {
	const s = "01JBZ8K3QW7M2X9V4T6R5N8P0A"
	for _, tc := range []struct {
		in      string
		want    string
		wantErr bool
	}{
		{s, s, false},
		{strings.ToLower(s), s, false},
		{"", "", true},
		{"01JBZ8K3QW7M2X9V4T6R5N8P0", "", true},   // 25 chars
		{"01JBZ8K3QW7M2X9V4T6R5N8P0U", "", true},  // U is not Crockford
		{"81JBZ8K3QW7M2X9V4T6R5N8P0A", "", true},  // overflows 128 bits
	} {
		got, err := ParseID(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseID(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got.String() != tc.want {
			t.Errorf("ParseID(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestIDYAML(t *testing.T) {
	type doc struct {
		ID ID `yaml:"id"`
	}
	var d doc
	if err := yaml.Load([]byte("id: 01JBZ8K3QW7M2X9V4T6R5N8P0A\n"), &d); err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Dump(d)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "id: 01JBZ8K3QW7M2X9V4T6R5N8P0A\n"; got != want {
		t.Errorf("Dump = %q, want %q", got, want)
	}
	if err := yaml.Load([]byte("id: nope\n"), &d); err == nil {
		t.Error("Load of a bad ID succeeded")
	}
	if !(ID{}).IsZero() || d.ID.IsZero() {
		t.Error("IsZero is wrong")
	}
}

func TestDecodeError(t *testing.T) {
	inner := errors.New("boom")
	e := &DecodeError{Field: "status", Line: 3, Err: inner}
	if got, want := e.Error(), "status (line 3): boom"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(e, inner) {
		t.Error("DecodeError does not unwrap")
	}
	if got, want := (&DecodeError{Field: "id", Err: inner}).Error(), "id: boom"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 3: テストが失敗することを確かめる**

Run: `go test ./internal/model/`
Expected: FAIL（`undefined: NewID` などのコンパイルエラー）

- [ ] **Step 4: 実装する**

`internal/model/errors.go`:
```go
// Package model holds Hedgehug's file types and their codecs. It reads and
// writes bytes only: no disk, locks or git.
package model

import "fmt"

// DecodeError reports a field that could not be decoded. Line is the line in
// the file, or 0 when unknown.
type DecodeError struct {
	Field string
	Line  int
	Err   error
}

func (e *DecodeError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s (line %d): %v", e.Field, e.Line, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Field, e.Err)
}

func (e *DecodeError) Unwrap() error { return e.Err }
```

`internal/model/id.go`:
```go
package model

import (
	"fmt"
	"strings"

	"github.com/oklog/ulid/v2"
	"go.yaml.in/yaml/v4"
)

// ID identifies a task, comment, worklog, sprint, release or store. It is a
// ULID, written as 26 upper-case Crockford base32 characters.
type ID struct{ u ulid.ULID }

// NewID returns a new ID. IDs made in one process increase monotonically.
func NewID() ID { return ID{ulid.Make()} }

// ParseID parses an ID in either case.
func ParseID(s string) (ID, error) {
	u, err := ulid.ParseStrict(strings.ToUpper(s))
	if err != nil {
		return ID{}, fmt.Errorf("invalid ID %q: %w", s, err)
	}
	return ID{u}, nil
}

func (id ID) String() string { return id.u.String() }

// IsZero reports whether id is the zero ID, which no file uses.
func (id ID) IsZero() bool { return id.u == ulid.ULID{} }

func (id ID) MarshalYAML() (any, error) { return id.String(), nil }

// UnmarshalYAML reads the scalar as written, so an ID of digits only is not
// taken for a number.
func (id *ID) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: want an ID", n.Line)
	}
	p, err := ParseID(n.Value)
	if err != nil {
		return err
	}
	*id = p
	return nil
}
```

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go mod tidy && go test -race ./internal/model/`
Expected: PASS。`go.mod` の2行から `// indirect` が消える。

- [ ] **Step 6: lint を通してコミットする**

Run: `make lint`
Expected: エラーなし

```bash
git add go.mod go.sum internal/model/errors.go internal/model/id.go internal/model/id_test.go
git commit -m "feat: タスク ID（ULID）とデコードエラーの型を追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Store 内のパス

**Files:**
- Create: `internal/model/path.go`
- Test: `internal/model/path_test.go`

**Interfaces:**
- Consumes: `ID`、`ParseID`（Task 1）
- Produces: `TaskPath(ID) string`、`CommentPath(task, comment ID) string`、`WorklogPath(task, log ID) string`、`SprintPath(ID) string`、`ReleasePath(ID) string`、`TemplatePath(name string) string`、`const StoreMetaPath = ".hedgehug/store.yaml"`、`ParseTaskPath(string) (ID, error)`、`ParseCommentPath(string) (task, comment ID, err error)`、`ParseWorklogPath(string) (task, log ID, err error)`

- [ ] **Step 1: 失敗するテストを書く**

`internal/model/path_test.go`:
```go
package model

import "testing"

func mustID(t *testing.T, s string) ID {
	t.Helper()
	id, err := ParseID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestPaths(t *testing.T) {
	task := mustID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0A")
	c := mustID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0B")
	for _, tc := range []struct{ got, want string }{
		{TaskPath(task), "tasks/A/0/01JBZ8K3QW7M2X9V4T6R5N8P0A.md"},
		{CommentPath(task, c), "comments/A/0/01JBZ8K3QW7M2X9V4T6R5N8P0A/01JBZ8K3QW7M2X9V4T6R5N8P0B.md"},
		{WorklogPath(task, c), "worklogs/A/0/01JBZ8K3QW7M2X9V4T6R5N8P0A/01JBZ8K3QW7M2X9V4T6R5N8P0B.md"},
		{SprintPath(task), "sprints/01JBZ8K3QW7M2X9V4T6R5N8P0A.md"},
		{ReleasePath(task), "releases/01JBZ8K3QW7M2X9V4T6R5N8P0A.md"},
		{TemplatePath("bug-report"), "templates/bug-report.md"},
		{StoreMetaPath, ".hedgehug/store.yaml"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}

func TestParseTaskPath(t *testing.T) {
	want := mustID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0A")
	got, err := ParseTaskPath(TaskPath(want))
	if err != nil || got != want {
		t.Errorf("ParseTaskPath = %v, %v; want %v", got, err, want)
	}
	for _, bad := range []string{
		"tasks/01JBZ8K3QW7M2X9V4T6R5N8P0A.md",           // flat
		"tasks/B/0/01JBZ8K3QW7M2X9V4T6R5N8P0A.md",       // wrong shard
		"tasks/A/0/01JBZ8K3QW7M2X9V4T6R5N8P0A.txt",      // wrong extension
		"comments/A/0/01JBZ8K3QW7M2X9V4T6R5N8P0A.md",    // wrong root
		"tasks/A/0/nope.md",
	} {
		if _, err := ParseTaskPath(bad); err == nil {
			t.Errorf("ParseTaskPath(%q) succeeded", bad)
		}
	}
}

func TestParseCommentAndWorklogPath(t *testing.T) {
	task := mustID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0A")
	c := mustID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0B")
	gt, gc, err := ParseCommentPath(CommentPath(task, c))
	if err != nil || gt != task || gc != c {
		t.Errorf("ParseCommentPath = %v, %v, %v", gt, gc, err)
	}
	gt, gc, err = ParseWorklogPath(WorklogPath(task, c))
	if err != nil || gt != task || gc != c {
		t.Errorf("ParseWorklogPath = %v, %v, %v", gt, gc, err)
	}
	if _, _, err := ParseCommentPath(WorklogPath(task, c)); err == nil {
		t.Error("ParseCommentPath accepted a worklog path")
	}
	if _, _, err := ParseCommentPath("comments/B/0/01JBZ8K3QW7M2X9V4T6R5N8P0A/01JBZ8K3QW7M2X9V4T6R5N8P0B.md"); err == nil {
		t.Error("ParseCommentPath accepted a wrong shard")
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/model/ -run 'Path'`
Expected: FAIL（`undefined: TaskPath`）

- [ ] **Step 3: 実装する**

`internal/model/path.go`:
```go
package model

import (
	"fmt"
	"strings"
)

// StoreMetaPath is where a store keeps its metadata.
const StoreMetaPath = ".hedgehug/store.yaml"

// shard splits files into 32×32 directories by the last and the second to
// last character of the ULID. The first characters are a timestamp and
// cluster, so the random tail is used instead.
func shard(id ID) string {
	s := id.String()
	return s[25:] + "/" + s[24:25]
}

// TaskPath returns the path of a task file, relative to the store root.
func TaskPath(id ID) string { return "tasks/" + shard(id) + "/" + id.String() + ".md" }

// CommentPath returns the path of a comment on task.
func CommentPath(task, comment ID) string {
	return "comments/" + shard(task) + "/" + task.String() + "/" + comment.String() + ".md"
}

// WorklogPath returns the path of a worklog entry on task.
func WorklogPath(task, log ID) string {
	return "worklogs/" + shard(task) + "/" + task.String() + "/" + log.String() + ".md"
}

func SprintPath(id ID) string  { return "sprints/" + id.String() + ".md" }
func ReleasePath(id ID) string { return "releases/" + id.String() + ".md" }

// TemplatePath returns the path of the template called name.
func TemplatePath(name string) string { return "templates/" + name + ".md" }

// ParseTaskPath returns the ID of the task file at p.
func ParseTaskPath(p string) (ID, error) {
	parts := strings.Split(p, "/")
	if len(parts) != 4 || parts[0] != "tasks" {
		return ID{}, fmt.Errorf("%q is not a task path", p)
	}
	id, err := parseFileID(parts[3])
	if err != nil || TaskPath(id) != p {
		return ID{}, fmt.Errorf("%q is not a task path", p)
	}
	return id, nil
}

// ParseCommentPath returns the task and comment IDs of the comment file at p.
func ParseCommentPath(p string) (task, comment ID, err error) {
	return parseChildPath(p, "comments", CommentPath)
}

// ParseWorklogPath returns the task and entry IDs of the worklog file at p.
func ParseWorklogPath(p string) (task, log ID, err error) {
	return parseChildPath(p, "worklogs", WorklogPath)
}

func parseChildPath(p, root string, build func(ID, ID) string) (ID, ID, error) {
	parts := strings.Split(p, "/")
	if len(parts) != 5 || parts[0] != root {
		return ID{}, ID{}, fmt.Errorf("%q is not a %s path", p, root)
	}
	task, err1 := ParseID(parts[3])
	child, err2 := parseFileID(parts[4])
	if err1 != nil || err2 != nil || build(task, child) != p {
		return ID{}, ID{}, fmt.Errorf("%q is not a %s path", p, root)
	}
	return task, child, nil
}

func parseFileID(name string) (ID, error) {
	s, ok := strings.CutSuffix(name, ".md")
	if !ok {
		return ID{}, fmt.Errorf("%q is not a .md file", name)
	}
	return ParseID(s)
}
```

注: `TaskPath(id) != p` の比較で、分割ディレクトリの食い違いと小文字の ULID をまとめて弾く。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/model/`
Expected: PASS

- [ ] **Step 5: lint を通してコミットする**

Run: `make lint`

```bash
git add internal/model/path.go internal/model/path_test.go
git commit -m "feat: ID と Store 内のパスを相互に変換する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: rank（fractional index）

rocicorp/fractional-indexing（https://github.com/rocicorp/fractional-indexing/blob/main/src/index.js）の base62 版を移植する。テストの期待値は元実装の `index.test.js` と同じ値。実装後、元のテストファイルを開いて、下の表に無い base62 のケースがあれば足すこと。

**Files:**
- Create: `internal/model/rank.go`
- Test: `internal/model/rank_test.go`

**Interfaces:**
- Produces: `type Rank string`、`Between(a, b Rank) (Rank, error)`、`NBetween(a, b Rank, n int) ([]Rank, error)`、`(Rank) Validate() error`、`(*Rank) UnmarshalYAML`

- [ ] **Step 1: 失敗するテストを書く**

`internal/model/rank_test.go`:
```go
package model

import (
	"strings"
	"testing"
)

func TestBetween(t *testing.T) {
	z26, z27 := strings.Repeat("0", 26), strings.Repeat("0", 27)
	zz := strings.Repeat("z", 26)
	for _, tc := range []struct{ a, b, want string }{
		{"", "", "a0"},
		{"", "a0", "Zz"},
		{"", "Zz", "Zy"},
		{"a0", "", "a1"},
		{"a1", "", "a2"},
		{"a0", "a1", "a0V"},
		{"a1", "a2", "a1V"},
		{"a0V", "a1", "a0l"},
		{"Zz", "a0", "ZzV"},
		{"Zz", "a1", "a0"},
		{"", "Y00", "Xzzz"},
		{"bzz", "", "c000"},
		{"a0", "a0V", "a0G"},
		{"a0", "a0G", "a08"},
		{"b125", "b129", "b127"},
		{"a0", "a1V", "a1"},
		{"Zz", "a01", "a0"},
		{"", "a0V", "a0"},
		{"", "b999", "b99"},
		{"", "A" + z26 + "1", "A" + z27 + "V"},
		{"z" + zz[:25] + "y", "", "z" + zz},
		{"z" + zz, "", "z" + zz + "V"},
	} {
		got, err := Between(Rank(tc.a), Rank(tc.b))
		if err != nil || string(got) != tc.want {
			t.Errorf("Between(%q, %q) = %q, %v; want %q", tc.a, tc.b, got, err, tc.want)
		}
	}
}

func TestBetweenErrors(t *testing.T) {
	for _, tc := range []struct{ a, b string }{
		{"a00", ""},              // trailing zero
		{"a00", "a1"},
		{"0", "1"},               // bad head
		{"a1", "a0"},             // a >= b
		{"a1", "a1"},
		{"", "A" + strings.Repeat("0", 26)}, // smallest integer
		{"a0!", ""},              // not base62
		{"b1", ""},               // integer part too short
	} {
		if got, err := Between(Rank(tc.a), Rank(tc.b)); err == nil {
			t.Errorf("Between(%q, %q) = %q, want an error", tc.a, tc.b, got)
		}
	}
}

func TestNBetween(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		n    int
		want string
	}{
		{"", "", 5, "a0 a1 a2 a3 a4"},
		{"a4", "", 10, "a5 a6 a7 a8 a9 aA aB aC aD aE"},
		{"", "a0", 5, "Zv Zw Zx Zy Zz"},
		{"a0", "a1", 0, ""},
	} {
		got, err := NBetween(Rank(tc.a), Rank(tc.b), tc.n)
		if err != nil {
			t.Fatalf("NBetween(%q, %q, %d): %v", tc.a, tc.b, tc.n, err)
		}
		s := make([]string, len(got))
		for i, r := range got {
			s[i] = string(r)
		}
		if strings.Join(s, " ") != tc.want {
			t.Errorf("NBetween(%q, %q, %d) = %v, want %s", tc.a, tc.b, tc.n, s, tc.want)
		}
	}
	// Between two neighbours, every key is strictly inside and increasing.
	got, err := NBetween("a0", "a1", 20)
	if err != nil || len(got) != 20 {
		t.Fatalf("NBetween = %v, %v", got, err)
	}
	prev := Rank("a0")
	for _, r := range got {
		if r <= prev || r >= "a1" {
			t.Fatalf("NBetween out of order: %v", got)
		}
		prev = r
	}
}

func TestRankValidate(t *testing.T) {
	if err := Rank("a0V").Validate(); err != nil {
		t.Error(err)
	}
	if err := Rank("a00").Validate(); err == nil {
		t.Error("Validate accepted a trailing zero")
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/model/ -run 'Between|Rank'`
Expected: FAIL（`undefined: Between`）

- [ ] **Step 3: 実装する**

`internal/model/rank.go`:
```go
package model

import (
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v4"
)

// Rank orders siblings. It is a key in the format of
// rocicorp/fractional-indexing with base62 digits: one head character that
// gives the length of the integer part, then the integer and fraction
// digits. Comparing ranks as strings gives their order, and a new rank can
// always be made between two others without touching them.
type Rank string

const rankDigits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var smallestInteger = "A" + strings.Repeat("0", 26)

// Validate reports whether r is a well-formed rank.
func (r Rank) Validate() error { return validateKey(string(r)) }

// UnmarshalYAML reads the scalar as written, so no rank is taken for another
// type.
func (r *Rank) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: want a rank", n.Line)
	}
	*r = Rank(n.Value)
	return nil
}

// Between returns a rank strictly between a and b. An empty a means before
// everything, an empty b after everything.
func Between(a, b Rank) (Rank, error) {
	sa, sb := string(a), string(b)
	if sa != "" {
		if err := validateKey(sa); err != nil {
			return "", err
		}
	}
	if sb != "" {
		if err := validateKey(sb); err != nil {
			return "", err
		}
	}
	if sa != "" && sb != "" && sa >= sb {
		return "", fmt.Errorf("rank %q is not before %q", sa, sb)
	}
	if sa == "" {
		if sb == "" {
			return "a0", nil
		}
		ib := integerPart(sb)
		fb := sb[len(ib):]
		if ib == smallestInteger {
			m, err := midpoint("", fb)
			return Rank(ib + m), err
		}
		if ib < sb {
			return Rank(ib), nil
		}
		r, ok := decrementInteger(ib)
		if !ok {
			return "", errors.New("rank: cannot make a rank before the smallest one")
		}
		return Rank(r), nil
	}
	ia := integerPart(sa)
	fa := sa[len(ia):]
	if sb == "" {
		i, ok := incrementInteger(ia)
		if !ok {
			m, err := midpoint(fa, "")
			return Rank(ia + m), err
		}
		return Rank(i), nil
	}
	ib := integerPart(sb)
	fb := sb[len(ib):]
	if ia == ib {
		m, err := midpoint(fa, fb)
		return Rank(ia + m), err
	}
	i, ok := incrementInteger(ia)
	if !ok {
		return "", errors.New("rank: cannot make a rank after the largest one")
	}
	if i < sb {
		return Rank(i), nil
	}
	m, err := midpoint(fa, "")
	return Rank(ia + m), err
}

// NBetween returns n ranks in increasing order, strictly between a and b.
func NBetween(a, b Rank, n int) ([]Rank, error) {
	switch {
	case n <= 0:
		return nil, nil
	case n == 1:
		r, err := Between(a, b)
		return []Rank{r}, err
	case b == "":
		out := make([]Rank, 0, n)
		c := a
		for range n {
			var err error
			if c, err = Between(c, b); err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, nil
	case a == "":
		out := make([]Rank, n)
		c := b
		for i := n - 1; i >= 0; i-- {
			var err error
			if c, err = Between(a, c); err != nil {
				return nil, err
			}
			out[i] = c
		}
		return out, nil
	}
	mid := n / 2
	c, err := Between(a, b)
	if err != nil {
		return nil, err
	}
	left, err := NBetween(a, c, mid)
	if err != nil {
		return nil, err
	}
	right, err := NBetween(c, b, n-mid-1)
	if err != nil {
		return nil, err
	}
	return append(append(left, c), right...), nil
}

// midpoint returns a fraction strictly between a and b, where an empty b
// means 1. Neither may end in the zero digit.
func midpoint(a, b string) (string, error) {
	if b != "" && a >= b {
		return "", fmt.Errorf("rank: %q is not before %q", a, b)
	}
	if strings.HasSuffix(a, "0") || strings.HasSuffix(b, "0") {
		return "", errors.New("rank: trailing zero")
	}
	if b != "" {
		n := 0
		for digitAt(a, n) == b[n] {
			n++
		}
		if n > 0 {
			m, err := midpoint(tail(a, n), b[n:])
			return b[:n] + m, err
		}
	}
	da := 0
	if a != "" {
		da = strings.IndexByte(rankDigits, a[0])
	}
	db := len(rankDigits)
	if b != "" {
		db = strings.IndexByte(rankDigits, b[0])
	}
	if db-da > 1 {
		return string(rankDigits[(da+db+1)/2]), nil
	}
	if len(b) > 1 {
		return b[:1], nil
	}
	m, err := midpoint(tail(a, 1), "")
	return string(rankDigits[da]) + m, err
}

// digitAt returns s[i], or the zero digit past the end of s.
func digitAt(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return '0'
}

func tail(s string, n int) string {
	if n >= len(s) {
		return ""
	}
	return s[n:]
}

// integerLength returns the length of the integer part that head starts.
func integerLength(head byte) (int, error) {
	switch {
	case head >= 'a' && head <= 'z':
		return int(head-'a') + 2, nil
	case head >= 'A' && head <= 'Z':
		return int('Z'-head) + 2, nil
	}
	return 0, fmt.Errorf("rank: invalid head %q", head)
}

// integerPart returns the integer part of a key that validateKey accepted.
func integerPart(key string) string {
	l, _ := integerLength(key[0])
	return key[:l]
}

func validateKey(key string) error {
	if key == "" {
		return errors.New("rank: empty")
	}
	if key == smallestInteger {
		return fmt.Errorf("rank: %q is reserved", key)
	}
	l, err := integerLength(key[0])
	if err != nil {
		return err
	}
	if l > len(key) {
		return fmt.Errorf("rank: %q is too short", key)
	}
	for i := 1; i < len(key); i++ {
		if strings.IndexByte(rankDigits, key[i]) < 0 {
			return fmt.Errorf("rank: %q has a character outside base62", key)
		}
	}
	if strings.HasSuffix(key[l:], "0") {
		return fmt.Errorf("rank: %q ends in a zero", key)
	}
	return nil
}

func incrementInteger(x string) (string, bool) {
	head, digs := x[0], []byte(x[1:])
	carry := true
	for i := len(digs) - 1; carry && i >= 0; i-- {
		d := strings.IndexByte(rankDigits, digs[i]) + 1
		if d == len(rankDigits) {
			digs[i] = '0'
		} else {
			digs[i] = rankDigits[d]
			carry = false
		}
	}
	if !carry {
		return string(head) + string(digs), true
	}
	switch head {
	case 'Z':
		return "a0", true
	case 'z':
		return "", false
	}
	h := head + 1
	if h > 'a' {
		digs = append(digs, '0')
	} else {
		digs = digs[:len(digs)-1]
	}
	return string(h) + string(digs), true
}

func decrementInteger(x string) (string, bool) {
	head, digs := x[0], []byte(x[1:])
	borrow := true
	for i := len(digs) - 1; borrow && i >= 0; i-- {
		d := strings.IndexByte(rankDigits, digs[i]) - 1
		if d == -1 {
			digs[i] = 'z'
		} else {
			digs[i] = rankDigits[d]
			borrow = false
		}
	}
	if !borrow {
		return string(head) + string(digs), true
	}
	switch head {
	case 'a':
		return "Zz", true
	case 'A':
		return "", false
	}
	h := head - 1
	if h < 'Z' {
		digs = append(digs, 'z')
	} else {
		digs = digs[:len(digs)-1]
	}
	return string(h) + string(digs), true
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/model/`
Expected: PASS。失敗したケースがあれば、元実装の `index.js` と1行ずつ突き合わせる（期待値の表は元実装のテストと同じ）。

- [ ] **Step 5: lint を通してコミットする**

Run: `make lint`

```bash
git add internal/model/rank.go internal/model/rank_test.go
git commit -m "feat: 兄弟の並び順に使う rank（fractional index）を追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Duration と Date

**Files:**
- Create: `internal/model/duration.go`, `internal/model/date.go`
- Test: `internal/model/duration_test.go`, `internal/model/date_test.go`

**Interfaces:**
- Produces: `type Duration time.Duration`、`ParseDuration(string) (Duration, error)`、`(Duration) String() string`、YAML の Marshal/Unmarshal。`type Date struct`、`ParseDate(string) (Date, error)`、`(Date) String() string`、`(Date) IsZero() bool`、`(Date) Time() time.Time`、YAML の Marshal/Unmarshal。

- [ ] **Step 1: 失敗するテストを書く**

`internal/model/duration_test.go`:
```go
package model

import (
	"testing"
	"time"

	"go.yaml.in/yaml/v4"
)

func TestParseDuration(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want time.Duration
		str  string
	}{
		{"6h", 6 * time.Hour, "6h"},
		{"1h30m", 90 * time.Minute, "1h30m"},
		{"45m", 45 * time.Minute, "45m"},
		{"90m", 90 * time.Minute, "1h30m"},
		{"0m", 0, "0m"},
	} {
		got, err := ParseDuration(tc.in)
		if err != nil || time.Duration(got) != tc.want || got.String() != tc.str {
			t.Errorf("ParseDuration(%q) = %v (%s), %v; want %v (%s)", tc.in, time.Duration(got), got, err, tc.want, tc.str)
		}
	}
	for _, bad := range []string{"", "1d", "1w", "1.5h", "h", "30m1h", "-1h", "1h 30m", "99999999999999999999h"} {
		if _, err := ParseDuration(bad); err == nil {
			t.Errorf("ParseDuration(%q) succeeded", bad)
		}
	}
}

func TestDurationYAML(t *testing.T) {
	var v struct {
		D Duration `yaml:"d"`
	}
	if err := yaml.Load([]byte("d: 90m\n"), &v); err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Dump(v)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "d: 1h30m\n"; got != want {
		t.Errorf("Dump = %q, want %q", got, want)
	}
}
```

`internal/model/date_test.go`:
```go
package model

import (
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestParseDate(t *testing.T) {
	d, err := ParseDate("2026-11-03")
	if err != nil || d.String() != "2026-11-03" || d.IsZero() {
		t.Errorf("ParseDate = %v, %v", d, err)
	}
	for _, bad := range []string{"", "2026-13-01", "2026/11/03", "2026-11-03T00:00:00Z"} {
		if _, err := ParseDate(bad); err == nil {
			t.Errorf("ParseDate(%q) succeeded", bad)
		}
	}
	if !(Date{}).IsZero() {
		t.Error("zero Date is not zero")
	}
}

func TestDateYAML(t *testing.T) {
	var v struct {
		D Date `yaml:"d"`
	}
	if err := yaml.Load([]byte("d: 2026-11-03\n"), &v); err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Dump(v)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "d: 2026-11-03\n"; got != want {
		t.Errorf("Dump = %q, want %q (no quotes, no time)", got, want)
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/model/ -run 'Duration|Date'`
Expected: FAIL（`undefined: ParseDuration`）

- [ ] **Step 3: 実装する**

`internal/model/duration.go`:
```go
package model

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"go.yaml.in/yaml/v4"
)

// Duration is a length of work, written with hours and minutes only: "6h",
// "1h30m", "45m". Days and weeks are left out because a day could mean 8 or
// 24 hours.
type Duration time.Duration

var durationRE = regexp.MustCompile(`^(?:(\d+)h)?(?:(\d+)m)?$`)

// ParseDuration parses a duration such as "1h30m".
func ParseDuration(s string) (Duration, error) {
	m := durationRE.FindStringSubmatch(s)
	if s == "" || m == nil {
		return 0, fmt.Errorf("invalid duration %q: use hours and minutes, like 1h30m", s)
	}
	var total time.Duration
	for i, unit := range []time.Duration{time.Hour, time.Minute} {
		if m[i+1] == "" {
			continue
		}
		v, err := strconv.ParseInt(m[i+1], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		total += time.Duration(v) * unit
	}
	return Duration(total), nil
}

// String writes d as "1h30m", "6h" or "45m".
func (d Duration) String() string {
	total := int64(time.Duration(d) / time.Minute)
	h, m := total/60, total%60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

func (d Duration) MarshalYAML() (any, error) { return d.String(), nil }

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: want a duration", n.Line)
	}
	p, err := ParseDuration(n.Value)
	if err != nil {
		return err
	}
	*d = p
	return nil
}
```

`internal/model/date.go`:
```go
package model

import (
	"fmt"
	"time"

	"go.yaml.in/yaml/v4"
)

// Date is a calendar date with no time or zone, written as 2006-01-02.
type Date struct{ t time.Time }

// ParseDate parses a date such as "2026-11-03".
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q: use YYYY-MM-DD", s)
	}
	return Date{t}, nil
}

func (d Date) String() string { return d.t.Format(time.DateOnly) }
func (d Date) IsZero() bool    { return d.t.IsZero() }

// Time returns midnight UTC of d.
func (d Date) Time() time.Time { return d.t }

// MarshalYAML writes d as a plain timestamp scalar, so it is not quoted.
func (d Date) MarshalYAML() (any, error) {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!timestamp", Value: d.String()}, nil
}

func (d *Date) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: want a date", n.Line)
	}
	p, err := ParseDate(n.Value)
	if err != nil {
		return err
	}
	*d = p
	return nil
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/model/`
Expected: PASS

- [ ] **Step 5: lint を通してコミットする**

Run: `make lint`

```bash
git add internal/model/duration.go internal/model/date.go internal/model/duration_test.go internal/model/date_test.go
git commit -m "feat: duration 型と日付型を追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: frontmatter の分割と結合

**Files:**
- Create: `internal/model/frontmatter.go`
- Test: `internal/model/frontmatter_test.go`

**Interfaces:**
- Produces（パッケージ内）: `splitFrontmatter(data []byte) (front []byte, body string, err error)`、`joinFrontmatter(front []byte, body string) []byte`、`normalizeNewlines(s string) string`

- [ ] **Step 1: 失敗するテストを書く**

`internal/model/frontmatter_test.go`:
```go
package model

import "testing"

func TestSplitFrontmatter(t *testing.T) {
	for _, tc := range []struct {
		name, in, front, body string
	}{
		{"basic", "---\na: 1\n---\nbody\n", "a: 1\n", "body\n"},
		{"crlf", "---\r\na: 1\r\n---\r\nbody\r\n", "a: 1\n", "body\n"},
		{"empty front", "---\n---\nbody", "", "body"},
		{"no body", "---\na: 1\n---\n", "a: 1\n", ""},
		{"closing at EOF", "---\na: 1\n---", "a: 1\n", ""},
		{"body with a rule", "---\na: 1\n---\nx\n---\ny\n", "a: 1\n", "x\n---\ny\n"},
	} {
		front, body, err := splitFrontmatter([]byte(tc.in))
		if err != nil || string(front) != tc.front || body != tc.body {
			t.Errorf("%s: split = %q, %q, %v; want %q, %q", tc.name, front, body, err, tc.front, tc.body)
		}
	}
	for _, bad := range []string{"a: 1\n", "---\na: 1\n", ""} {
		if _, _, err := splitFrontmatter([]byte(bad)); err == nil {
			t.Errorf("split(%q) succeeded", bad)
		}
	}
}

func TestJoinFrontmatter(t *testing.T) {
	if got, want := string(joinFrontmatter([]byte("a: 1\n"), "body\r\n")), "---\na: 1\n---\nbody\n"; got != want {
		t.Errorf("join = %q, want %q", got, want)
	}
	if got, want := string(joinFrontmatter([]byte("a: 1\n"), "")), "---\na: 1\n---\n"; got != want {
		t.Errorf("join = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/model/ -run Frontmatter`
Expected: FAIL（`undefined: splitFrontmatter`）

- [ ] **Step 3: 実装する**

`internal/model/frontmatter.go`:
```go
package model

import (
	"errors"
	"strings"
)

const fence = "---\n"

// normalizeNewlines turns CRLF into LF. Editors on Windows may save CRLF.
func normalizeNewlines(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

// splitFrontmatter splits a Markdown file into its YAML frontmatter and its
// body. The file must start with a "---" line, and the next "---" line ends
// the frontmatter.
func splitFrontmatter(data []byte) (front []byte, body string, err error) {
	s := normalizeNewlines(string(data))
	rest, ok := strings.CutPrefix(s, fence)
	if !ok {
		return nil, "", errors.New("missing frontmatter: the file must start with ---")
	}
	if b, ok := strings.CutPrefix(rest, fence); ok {
		return nil, b, nil
	}
	if rest == "---" {
		return nil, "", nil
	}
	if i := strings.Index(rest, "\n"+fence); i >= 0 {
		return []byte(rest[:i+1]), rest[i+1+len(fence):], nil
	}
	if f, ok := strings.CutSuffix(rest, "\n---"); ok {
		return []byte(f + "\n"), "", nil
	}
	return nil, "", errors.New("unterminated frontmatter: no closing ---")
}

// joinFrontmatter is the reverse of splitFrontmatter. front must end in a
// newline.
func joinFrontmatter(front []byte, body string) []byte {
	return []byte(fence + string(front) + fence + normalizeNewlines(body))
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/model/`
Expected: PASS

- [ ] **Step 5: lint を通してコミットする**

Run: `make lint`

```bash
git add internal/model/frontmatter.go internal/model/frontmatter_test.go
git commit -m "feat: frontmatter と本文を分割・結合する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Doc と Encoder（案A の中核）

**Files:**
- Create: `internal/model/node.go`
- Test: `internal/model/node_test.go`

**Interfaces:**
- Consumes: `DecodeError`（Task 1）
- Produces:
  - `type Field struct{ Key string; Value *yaml.Node }`、`type Fields []Field`、`(Fields) Get(key) *yaml.Node`、`(*Fields) Set(key string, v *yaml.Node)`、`(*Fields) Delete(key)`、`(*Fields) UnmarshalYAML`、`(Fields) MarshalYAML`
  - `NodeEqual(a, b *yaml.Node) bool`
  - `type Doc`、`ParseDoc(src []byte, lineOffset int) (*Doc, error)`、`NewDoc() *Doc`、`(*Doc) Get(key) *yaml.Node`、`(*Doc) Decode(key string, out any) error`、`(*Doc) Extra(known []string) Fields`、`(*Doc) Encoder() *Encoder`
  - `type Encoder`、`(*Encoder) Put(key string, v any, zero bool)`、`(*Encoder) Bytes(extra Fields) ([]byte, error)`
  - パッケージ内: `mappingOf(old *yaml.Node, f Fields) *yaml.Node`

`Put` の規則（spec の「YAML の往復」）:
1. ファイルにキーがなく `zero` が true なら書かない。
2. 新しい値がファイルの値と `NodeEqual` なら、ファイルの Node をそのまま使う（引用符などの書き方が残る）。
3. 変わった値は、元の Node の Style（block/flow）とコメントを引き継ぐ。
4. 元の Node がないときは、スカラーだけの列を flow（`[a, b]`）にし、列の中の「値がスカラーかスカラーだけの集まり」の mapping を flow（`{k: v}`）にする。

`Bytes` は `Put` した順に既知キーを並べ、その後ろに `extra` を置く。キーの Node は元のものを再利用するので、キーに付いたコメントが残る。

- [ ] **Step 1: 失敗するテストを書く**

`internal/model/node_test.go`:
```go
package model

import (
	"errors"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func node(t *testing.T, v any) *yaml.Node {
	t.Helper()
	var n yaml.Node
	if err := n.Encode(v); err != nil {
		t.Fatal(err)
	}
	return &n
}

func parse(t *testing.T, src string) *Doc {
	t.Helper()
	d, err := ParseDoc([]byte(src), 0)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func encode(t *testing.T, e *Encoder, extra Fields) string {
	t.Helper()
	out, err := e.Bytes(extra)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestPutKeepsUnchangedNode(t *testing.T) {
	d := parse(t, "rank: \"a0V\"\n")
	e := d.Encoder()
	e.Put("rank", "a0V", false)
	if got, want := encode(t, e, nil), "rank: \"a0V\"\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPutKeepsComments(t *testing.T) {
	d := parse(t, "# head\nkey: WEB-42 # note\n")
	e := d.Encoder()
	e.Put("key", "WEB-43", false)
	if got, want := encode(t, e, nil), "# head\nkey: WEB-43 # note\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPutZero(t *testing.T) {
	d := parse(t, "labels: [a]\n")
	e := d.Encoder()
	e.Put("labels", []string{}, true) // present: kept, written empty
	e.Put("repos", []string{}, true)  // absent and zero: left out
	e.Put("title", "x", false)        // absent: added
	if got, want := encode(t, e, nil), "labels: []\ntitle: x\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPutNilNodeWritesNull(t *testing.T) {
	d := parse(t, "gh: {}\n")
	e := d.Encoder()
	e.Put("gh", (*yaml.Node)(nil), false)
	if got, want := encode(t, e, nil), "gh: null\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPutStyles(t *testing.T) {
	type link struct {
		Type string `yaml:"type"`
		Ref  string `yaml:"ref"`
	}
	type deep struct {
		Where string              `yaml:"where"`
		W     map[string][]string `yaml:"w"`
	}
	e := NewDoc().Encoder()
	e.Put("labels", []string{"x", "y"}, false)
	e.Put("links", []link{{"t", "r"}}, false)
	e.Put("scoped", []deep{{"p", map[string][]string{"a": {"b"}}}}, false)
	got := encode(t, e, nil)
	if want := "labels: [x, y]\nlinks:\n- {type: t, ref: r}\nscoped:\n"; !strings.HasPrefix(got, want) {
		t.Errorf("got %q, want it to start with %q", got, want)
	}
	// A mapping with a nested collection inside a list stays in block style.
	if !strings.Contains(got, "\n- where: p\n") {
		t.Errorf("deep item is not in block style: %q", got)
	}
}

func TestPutKeepsBlockStyle(t *testing.T) {
	d := parse(t, "labels:\n- a\n")
	e := d.Encoder()
	e.Put("labels", []string{"a", "b"}, false)
	if got, want := encode(t, e, nil), "labels:\n- a\n- b\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBytesOrdersKeys(t *testing.T) {
	d := parse(t, "b: 1\nz: 9 # unknown\na: 2\n")
	e := d.Encoder()
	e.Put("a", 2, false)
	e.Put("b", 1, false)
	if got, want := encode(t, e, d.Extra([]string{"a", "b"})), "a: 2\nb: 1\nz: 9 # unknown\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDocDecode(t *testing.T) {
	d, err := ParseDoc([]byte("title: 2026-11-03\nlabels: [a, 2026-01-01]\nn: abc\nnone: null\n"), 1)
	if err != nil {
		t.Fatal(err)
	}
	var title string
	var labels []string
	if err := d.Decode("title", &title); err != nil || title != "2026-11-03" {
		t.Errorf("title = %q, %v", title, err)
	}
	if err := d.Decode("labels", &labels); err != nil || len(labels) != 2 || labels[1] != "2026-01-01" {
		t.Errorf("labels = %q, %v", labels, err)
	}
	none := "kept"
	if err := d.Decode("none", &none); err != nil || none != "kept" {
		t.Errorf("null changed the value: %q, %v", none, err)
	}
	if err := d.Decode("missing", &none); err != nil || none != "kept" {
		t.Errorf("absent key changed the value: %q, %v", none, err)
	}
	var n int
	err = d.Decode("n", &n)
	var de *DecodeError
	if !errors.As(err, &de) || de.Field != "n" || de.Line != 4 {
		t.Errorf("Decode(n) error = %#v, want a DecodeError at line 4", err)
	}
}

func TestParseDocRejectsNonMapping(t *testing.T) {
	if _, err := ParseDoc([]byte("- a\n"), 0); err == nil {
		t.Error("ParseDoc accepted a list")
	}
	d, err := ParseDoc([]byte("  \n"), 0)
	if err != nil || d.Get("x") != nil {
		t.Errorf("ParseDoc(blank) = %v, %v", d, err)
	}
}

func TestFields(t *testing.T) {
	var f Fields
	f.Set("a", node(t, 1))
	f.Set("b", node(t, 2))
	f.Set("a", node(t, 3))
	if len(f) != 2 || f.Get("a").Value != "3" || f.Get("x") != nil {
		t.Errorf("Fields = %v", f)
	}
	f.Delete("a")
	if len(f) != 1 || f[0].Key != "b" {
		t.Errorf("after Delete: %v", f)
	}
}

func TestNodeEqual(t *testing.T) {
	d := parse(t, "a: \"x\"\nb: [1, 2]\nc: {k: v}\n")
	for _, tc := range []struct {
		key  string
		v    any
		want bool
	}{
		{"a", "x", true},
		{"a", "y", false},
		{"b", []int{1, 2}, true},
		{"b", []int{2, 1}, false},
		{"b", []string{"1", "2"}, false}, // !!str is not !!int
		{"c", map[string]string{"k": "v"}, true},
	} {
		if got := NodeEqual(d.Get(tc.key), node(t, tc.v)); got != tc.want {
			t.Errorf("NodeEqual(%s, %v) = %v, want %v", tc.key, tc.v, got, tc.want)
		}
	}
	if !NodeEqual(nil, nil) || NodeEqual(nil, node(t, 1)) {
		t.Error("NodeEqual mishandles nil")
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/model/ -run 'Put|Bytes|Doc|Fields|NodeEqual'`
Expected: FAIL（`undefined: ParseDoc`）

- [ ] **Step 3: 実装する**

`internal/model/node.go`:
```go
package model

import (
	"bytes"
	"errors"
	"fmt"
	"slices"

	"go.yaml.in/yaml/v4"
)

// Field is one key and value of a YAML mapping.
type Field struct {
	Key   string
	Value *yaml.Node
}

// Fields is an ordered list of YAML keys and values. Props and unknown keys
// use it, so that their order and comments survive a round trip.
type Fields []Field

// Get returns the value of key, or nil when the key is absent.
func (f Fields) Get(key string) *yaml.Node {
	for _, x := range f {
		if x.Key == key {
			return x.Value
		}
	}
	return nil
}

// Set replaces the value of key, or appends the key when it is absent.
func (f *Fields) Set(key string, v *yaml.Node) {
	for i := range *f {
		if (*f)[i].Key == key {
			(*f)[i].Value = v
			return
		}
	}
	*f = append(*f, Field{key, v})
}

// Delete removes key.
func (f *Fields) Delete(key string) {
	*f = slices.DeleteFunc(*f, func(x Field) bool { return x.Key == key })
}

// UnmarshalYAML reads a mapping and keeps each value node as written.
func (f *Fields) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: want a mapping", n.Line)
	}
	*f = fieldsOf(n)
	return nil
}

func (f Fields) MarshalYAML() (any, error) { return mappingOf(nil, f), nil }

func fieldsOf(m *yaml.Node) Fields {
	f := make(Fields, 0, len(m.Content)/2)
	for i := 0; i+1 < len(m.Content); i += 2 {
		f = append(f, Field{m.Content[i].Value, m.Content[i+1]})
	}
	return f
}

// mappingOf builds a mapping node that holds f. The key nodes and the look
// of old are reused, so that comments survive.
func mappingOf(old *yaml.Node, f Fields) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	keys := map[string]*yaml.Node{}
	if old != nil && old.Kind == yaml.MappingNode {
		m.Style = old.Style
		m.HeadComment, m.LineComment, m.FootComment = old.HeadComment, old.LineComment, old.FootComment
		for i := 0; i+1 < len(old.Content); i += 2 {
			keys[old.Content[i].Value] = old.Content[i]
		}
	}
	for _, x := range f {
		k, ok := keys[x.Key]
		if !ok {
			k = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: x.Key}
		}
		m.Content = append(m.Content, k, x.Value)
	}
	return m
}

// NodeEqual reports whether a and b hold the same YAML value. Styles,
// comments and positions are ignored.
func NodeEqual(a, b *yaml.Node) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind == yaml.AliasNode || b.Kind == yaml.AliasNode {
		return a.Kind == b.Kind && NodeEqual(a.Alias, b.Alias)
	}
	if a.Kind != b.Kind || a.ShortTag() != b.ShortTag() || len(a.Content) != len(b.Content) {
		return false
	}
	if a.Kind == yaml.ScalarNode && a.Value != b.Value {
		return false
	}
	for i := range a.Content {
		if !NodeEqual(a.Content[i], b.Content[i]) {
			return false
		}
	}
	return true
}

// Doc is a YAML mapping kept from decode to encode, so that comments, key
// order and unknown keys survive.
type Doc struct {
	root *yaml.Node // document node holding one mapping
	line int        // added to node lines to give lines in the file
}

// ParseDoc parses src, which must be a YAML mapping or blank. lineOffset is
// the number of file lines before src, used in DecodeError.
func ParseDoc(src []byte, lineOffset int) (*Doc, error) {
	if len(bytes.TrimSpace(src)) == 0 {
		return NewDoc(), nil
	}
	var root yaml.Node
	if err := yaml.Load(src, &root); err != nil {
		return nil, err
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("want a YAML mapping")
	}
	return &Doc{root: &root, line: lineOffset}, nil
}

// NewDoc returns an empty Doc, for values made in code.
func NewDoc() *Doc {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	return &Doc{root: &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{m}}}
}

func (d *Doc) mapping() *yaml.Node { return d.root.Content[0] }

// Get returns the value of key, or nil when it is absent.
func (d *Doc) Get(key string) *yaml.Node { return fieldsOf(d.mapping()).Get(key) }

// Decode reads the value of key into out. An absent key or null leaves out
// unchanged. Strings and lists of strings are read as written, so a title
// such as 2026-11-03 stays a string. A **yaml.Node gets the node itself.
func (d *Doc) Decode(key string, out any) error {
	n := d.Get(key)
	if n == nil || n.ShortTag() == "!!null" {
		return nil
	}
	var err error
	switch o := out.(type) {
	case *string:
		*o, err = scalar(n)
	case *[]string:
		*o, err = scalars(n)
	case **yaml.Node:
		*o = n
	default:
		err = n.Decode(out)
	}
	if err != nil {
		return &DecodeError{Field: key, Line: n.Line + d.line, Err: err}
	}
	return nil
}

func scalar(n *yaml.Node) (string, error) {
	if n.Kind != yaml.ScalarNode {
		return "", errors.New("want a single value")
	}
	return n.Value, nil
}

func scalars(n *yaml.Node) ([]string, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, errors.New("want a list")
	}
	s := make([]string, 0, len(n.Content))
	for _, it := range n.Content {
		v, err := scalar(it)
		if err != nil {
			return nil, err
		}
		s = append(s, v)
	}
	return s, nil
}

// Extra returns the keys not in known, in file order.
func (d *Doc) Extra(known []string) Fields {
	var f Fields
	for _, x := range fieldsOf(d.mapping()) {
		if !slices.Contains(known, x.Key) {
			f = append(f, x)
		}
	}
	return f
}

// Encoder writes values back into a Doc, known keys first in the order they
// are put, then the extra keys.
type Encoder struct {
	d      *Doc
	fields Fields
	err    error
}

func (d *Doc) Encoder() *Encoder { return &Encoder{d: d} }

// Put sets key to v. A *yaml.Node is used as is; a nil one writes null. When
// the key is absent from the file and zero is true, the key is left out.
// When v equals the value in the file, the file's node is kept as written.
func (e *Encoder) Put(key string, v any, zero bool) {
	if e.err != nil {
		return
	}
	old := e.d.Get(key)
	if old == nil && zero {
		return
	}
	nv, ok := v.(*yaml.Node)
	if !ok || nv == nil {
		if ok {
			v = nil
		}
		nv = new(yaml.Node)
		if err := nv.Encode(v); err != nil {
			e.err = fmt.Errorf("%s: %w", key, err)
			return
		}
	}
	if NodeEqual(old, nv) {
		nv = old
	} else {
		styleLike(nv, old)
	}
	e.fields = append(e.fields, Field{key, nv})
}

// Bytes returns the YAML, with extra after the known keys.
func (e *Encoder) Bytes(extra Fields) ([]byte, error) {
	if e.err != nil {
		return nil, e.err
	}
	e.d.root.Content[0] = mappingOf(e.d.mapping(), slices.Concat(e.fields, extra))
	return yaml.Dump(e.d.root, yaml.WithLineWidth(-1))
}

// styleLike gives a new value node the look of the node it replaces. With no
// node to follow, lists of scalars and the short mappings inside lists are
// written in flow style, as in [a, b] and {type: x, ref: y}.
func styleLike(nv, old *yaml.Node) {
	if nv.Kind == yaml.SequenceNode {
		for _, it := range nv.Content {
			if it.Kind == yaml.MappingNode && short(it) {
				it.Style = yaml.FlowStyle
			}
		}
	}
	switch {
	case old != nil && old.Kind == nv.Kind && nv.Kind != yaml.ScalarNode:
		nv.Style = old.Style
	case nv.Kind == yaml.SequenceNode && flat(nv):
		nv.Style = yaml.FlowStyle
	}
	if old != nil {
		nv.HeadComment, nv.LineComment, nv.FootComment = old.HeadComment, old.LineComment, old.FootComment
	}
}

// flat reports whether n is a scalar or a collection of scalars.
func flat(n *yaml.Node) bool {
	for _, c := range n.Content {
		if c.Kind != yaml.ScalarNode {
			return false
		}
	}
	return true
}

// short reports whether every value of the mapping m is flat.
func short(m *yaml.Node) bool {
	for i := 1; i < len(m.Content); i += 2 {
		if !flat(m.Content[i]) {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/model/`
Expected: PASS。`TestPutStyles` が落ちた場合は、v4 の emitter の実際の出力を確かめる。意図（スカラーの列と短い mapping は flow、入れ子のある mapping は block）が合っていて空白だけが違うなら、期待値を実際の出力に合わせてよい。意図と違う場合は `styleLike` を直す。

- [ ] **Step 5: lint を通してコミットする**

Run: `make lint`

```bash
git add internal/model/node.go internal/model/node_test.go
git commit -m "feat: コメントと未知のキーを保って YAML を書き戻す Doc を追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: タスクファイル

**Files:**
- Create: `internal/model/task.go`, `internal/model/testdata/task/full.md`
- Test: `internal/model/task_test.go`

**Interfaces:**
- Consumes: `ID`、`Rank`、`Doc`、`Encoder`、`Fields`、`mappingOf`、`splitFrontmatter`、`joinFrontmatter`、`DecodeError`
- Produces: `type Task struct{...}`（下記）、`type Link struct{ Type, Ref, Sync string }`、`DecodeTask([]byte) (*Task, error)`、`(*Task) Encode() ([]byte, error)`、`TaskKeys() []string`

- [ ] **Step 1: 正準形のテストデータを書く**

`internal/model/testdata/task/full.md`（このファイルは「既に正準形」なので、デコードしてエンコードするとバイト単位で同じになる）:
```markdown
---
id: 01JBZ8K3QW7M2X9V4T6R5N8P0A
key: WEB-42 # 人が読めるキー
former_keys: [API-17]
key_prefix: WEB
kind: story
title: ログイン画面の改修
parent: 01JBZ8K3QW7M2X9V4T6R5N8P0B
rank: a0V
status: in_progress
assignees: [github.com/yourname]
watchers: [github.com/yourname, github.com/teammate]
labels: [frontend]
repos: [github.com/org-a/web]
blocked_by: [01JBZ8K3QW7M2X9V4T6R5N8P0C]
sprint: 01JBZ8K3QW7M2X9V4T6R5N8P0D
releases: [01JBZ8K3QW7M2X9V4T6R5N8P0E]
links:
- {type: github_pr, ref: github.com/org-a/web/pull/123, sync: mirror}
props:
  priority: high # 優先度
  start: 2026-11-03
  due: 2026-11-10
  estimate: 3h
  points: 5
  duplicates: [01JBZ8K3QW7M2X9V4T6R5N8P0F]
gh: {}
created: 2026-10-29T10:00:00+09:00
created_by: github.com/yourname
updated: 2026-10-29T12:30:00+09:00
deleted_at: null
custom: kept # 未知のキー
---
メモ本文（Markdown）

- [ ] チェックリストの項目
```

- [ ] **Step 2: 失敗するテストを書く**

`internal/model/task_test.go`:
```go
package model

import (
	"os"
	"strings"
	"testing"
	"time"
)

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestTaskRoundTrip(t *testing.T) {
	src := readFile(t, "testdata/task/full.md")
	task, err := DecodeTask(src)
	if err != nil {
		t.Fatal(err)
	}
	if task.Key != "WEB-42" || task.Title != "ログイン画面の改修" || task.Rank != "a0V" ||
		task.Parent == nil || task.Sprint == nil || len(task.BlockedBy) != 1 ||
		len(task.Links) != 1 || task.Links[0].Sync != "mirror" ||
		task.Props.Get("points").Value != "5" || task.GH == nil || task.DeletedAt != nil ||
		task.Extra.Get("custom").Value != "kept" || !strings.HasPrefix(task.Body, "メモ本文") {
		t.Errorf("decoded task = %+v", task)
	}
	if want := time.Date(2026, 10, 29, 3, 30, 0, 0, time.UTC); !task.Updated.Equal(want) {
		t.Errorf("Updated = %v, want %v", task.Updated, want)
	}
	out, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(src) {
		t.Errorf("round trip changed the file:\n%s", out)
	}
}

func TestTaskEncodeChanges(t *testing.T) {
	task, err := DecodeTask(readFile(t, "testdata/task/full.md"))
	if err != nil {
		t.Fatal(err)
	}
	task.Key = "WEB-43"
	task.Labels = append(task.Labels, "api")
	task.Props.Delete("points")
	task.Extra.Delete("custom")
	out, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"key: WEB-43 # 人が読めるキー\n", "labels: [frontend, api]\n", "  priority: high # 優先度\n"} {
		if !strings.Contains(s, want) {
			t.Errorf("output lacks %q:\n%s", want, s)
		}
	}
	for _, gone := range []string{"points:", "custom:"} {
		if strings.Contains(s, gone) {
			t.Errorf("output still has %q", gone)
		}
	}
}

func TestTaskReordersKnownKeys(t *testing.T) {
	src := "---\ntitle: A\ncustom: x\nid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\n---\n"
	task, err := DecodeTask([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "---\nid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\ntitle: A\ncustom: x\n---\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTaskCRLF(t *testing.T) {
	src := "---\r\nid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\r\ntitle: A\r\n---\r\nline\r\n"
	task, err := DecodeTask([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := task.Encode()
	if got, want := string(out), "---\nid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\ntitle: A\n---\nline\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNewTaskEncode(t *testing.T) {
	id, _ := ParseID("01JBZ8K3QW7M2X9V4T6R5N8P0A")
	created := time.Date(2026, 10, 29, 10, 0, 0, 0, time.FixedZone("", 9*3600))
	task := &Task{ID: id, Kind: "task", Title: "新しいタスク", Rank: "a0", Status: "todo",
		Labels: []string{"x"}, Created: created, CreatedBy: "github.com/me", Updated: created, Body: "本文\n"}
	out, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	want := "---\nid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nkind: task\ntitle: 新しいタスク\nrank: a0\nstatus: todo\nlabels: [x]\n" +
		"created: 2026-10-29T10:00:00+09:00\ncreated_by: github.com/me\nupdated: 2026-10-29T10:00:00+09:00\n---\n本文\n"
	if string(out) != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestTaskForwardingFile(t *testing.T) {
	// Phase 3 leaves files with only an id and moved_to. They must decode.
	task, err := DecodeTask([]byte("---\nid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nmoved_to: org-a\n---\n"))
	if err != nil || task.Extra.Get("moved_to") == nil {
		t.Errorf("DecodeTask = %+v, %v", task, err)
	}
}

func TestDecodeTaskErrors(t *testing.T) {
	for _, src := range []string{
		"---\ntitle: no id\n---\n",
		"---\nid: nope\n---\n",
		"---\nid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nparent: [1]\n---\n",
		"no frontmatter",
	} {
		if _, err := DecodeTask([]byte(src)); err == nil {
			t.Errorf("DecodeTask(%q) succeeded", src)
		}
	}
}

func TestTaskKeysIsACopy(t *testing.T) {
	k := TaskKeys()
	k[0] = "changed"
	if TaskKeys()[0] != "id" {
		t.Error("TaskKeys returned the shared slice")
	}
}
```

- [ ] **Step 3: テストが失敗することを確かめる**

Run: `go test ./internal/model/ -run Task`
Expected: FAIL（`undefined: DecodeTask`）

- [ ] **Step 4: 実装する**

`internal/model/task.go`:
```go
package model

import (
	"errors"
	"slices"
	"time"

	"go.yaml.in/yaml/v4"
)

// Task is one task file. A project is a task too.
type Task struct {
	ID         ID
	Key        string   // human-readable key such as WEB-42, fixed by a sync
	FormerKeys []string // keys before renumbering
	KeyPrefix  string   // prefix for descendants; container kinds only
	Kind       string
	Title      string
	Parent     *ID
	Rank       Rank
	Status     string
	Assignees  []string
	Watchers   []string
	Labels     []string
	Repos      []string
	BlockedBy  []ID
	Sprint     *ID
	Releases   []ID
	Links      []Link
	Props      Fields     // typed by the schema, not here
	GH         *yaml.Node // values from GitHub, read-only; typed in phase 5
	Created    time.Time
	CreatedBy  string
	Updated    time.Time
	DeletedAt  *time.Time
	Extra      Fields // keys this version does not know, in file order
	Body       string

	doc *Doc
}

// Link ties a task to a pull request or an issue.
type Link struct {
	Type string `yaml:"type"`
	Ref  string `yaml:"ref"`
	Sync string `yaml:"sync,omitempty"`
}

var taskKeys = []string{
	"id", "key", "former_keys", "key_prefix", "kind", "title", "parent", "rank", "status",
	"assignees", "watchers", "labels", "repos", "blocked_by", "sprint", "releases", "links",
	"props", "gh", "created", "created_by", "updated", "deleted_at",
}

// TaskKeys returns the keys of a task file, in the order they are written.
func TaskKeys() []string { return slices.Clone(taskKeys) }

// DecodeTask reads a task file. Only id is required; other checks belong to
// the schema.
func DecodeTask(data []byte) (*Task, error) {
	front, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	d, err := ParseDoc(front, 1)
	if err != nil {
		return nil, err
	}
	t := &Task{Body: body, doc: d}
	for _, f := range []struct {
		key string
		out any
	}{
		{"id", &t.ID}, {"key", &t.Key}, {"former_keys", &t.FormerKeys}, {"key_prefix", &t.KeyPrefix},
		{"kind", &t.Kind}, {"title", &t.Title}, {"parent", &t.Parent}, {"rank", &t.Rank},
		{"status", &t.Status}, {"assignees", &t.Assignees}, {"watchers", &t.Watchers},
		{"labels", &t.Labels}, {"repos", &t.Repos}, {"blocked_by", &t.BlockedBy},
		{"sprint", &t.Sprint}, {"releases", &t.Releases}, {"links", &t.Links},
		{"props", &t.Props}, {"gh", &t.GH}, {"created", &t.Created},
		{"created_by", &t.CreatedBy}, {"updated", &t.Updated}, {"deleted_at", &t.DeletedAt},
	} {
		if err := d.Decode(f.key, f.out); err != nil {
			return nil, err
		}
	}
	if t.ID.IsZero() {
		return nil, &DecodeError{Field: "id", Err: errors.New("missing")}
	}
	t.Extra = d.Extra(taskKeys)
	return t, nil
}

// Encode writes the task file. Comments, unknown keys and values that did
// not change keep the form they had in the file.
func (t *Task) Encode() ([]byte, error) {
	if t.doc == nil {
		t.doc = NewDoc()
	}
	e := t.doc.Encoder()
	e.Put("id", t.ID, false)
	e.Put("key", t.Key, t.Key == "")
	e.Put("former_keys", t.FormerKeys, len(t.FormerKeys) == 0)
	e.Put("key_prefix", t.KeyPrefix, t.KeyPrefix == "")
	e.Put("kind", t.Kind, t.Kind == "")
	e.Put("title", t.Title, t.Title == "")
	e.Put("parent", t.Parent, t.Parent == nil)
	e.Put("rank", string(t.Rank), t.Rank == "")
	e.Put("status", t.Status, t.Status == "")
	e.Put("assignees", t.Assignees, len(t.Assignees) == 0)
	e.Put("watchers", t.Watchers, len(t.Watchers) == 0)
	e.Put("labels", t.Labels, len(t.Labels) == 0)
	e.Put("repos", t.Repos, len(t.Repos) == 0)
	e.Put("blocked_by", t.BlockedBy, len(t.BlockedBy) == 0)
	e.Put("sprint", t.Sprint, t.Sprint == nil)
	e.Put("releases", t.Releases, len(t.Releases) == 0)
	e.Put("links", t.Links, len(t.Links) == 0)
	e.Put("props", mappingOf(t.doc.Get("props"), t.Props), len(t.Props) == 0)
	e.Put("gh", t.GH, t.GH == nil)
	e.Put("created", t.Created, t.Created.IsZero())
	e.Put("created_by", t.CreatedBy, t.CreatedBy == "")
	e.Put("updated", t.Updated, t.Updated.IsZero())
	e.Put("deleted_at", t.DeletedAt, t.DeletedAt == nil)
	front, err := e.Bytes(t.Extra)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, t.Body), nil
}
```

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test -race ./internal/model/`
Expected: PASS。`TestTaskRoundTrip` が落ちたら、差分の行を見て「入力が正準形でなかった」のか「コードが書き方を変えた」のかを切り分ける。前者ならテストデータを v4 の出力に合わせ、後者なら `Put`/`styleLike` を直す。

- [ ] **Step 6: lint を通してコミットする**

Run: `make lint`

```bash
git add internal/model/task.go internal/model/task_test.go internal/model/testdata/task/full.md
git commit -m "feat: タスクファイルの読み書きを追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: コメントと作業時間

**Files:**
- Create: `internal/model/comment.go`, `internal/model/worklog.go`, `internal/model/testdata/comment/basic.md`, `internal/model/testdata/worklog/basic.md`
- Test: `internal/model/comment_test.go`

**Interfaces:**
- Consumes: Task 6 の `Doc`/`Encoder`、Task 4 の `Date`/`Duration`、Task 5 の frontmatter
- Produces: `type Comment struct{ ID ID; Author string; Created, Updated time.Time; DeletedAt *time.Time; Extra Fields; Body string }`、`DecodeComment([]byte) (*Comment, error)`、`(*Comment) Encode()`。`type Worklog struct{ ID ID; Author string; Date Date; Duration Duration; Created, Updated time.Time; DeletedAt *time.Time; Extra Fields; Body string }`、`DecodeWorklog`、`(*Worklog) Encode()`。

- [ ] **Step 1: テストデータを書く**

`internal/model/testdata/comment/basic.md`:
```markdown
---
id: 01JBZ8K3QW7M2X9V4T6R5N8P0B
author: github.com/yourname
created: 2026-10-29T10:00:00+09:00
updated: 2026-10-29T10:05:00+09:00
---
@teammate レビューお願いします
```

`internal/model/testdata/worklog/basic.md`:
```markdown
---
id: 01JBZ8K3QW7M2X9V4T6R5N8P0C
author: github.com/yourname
date: 2026-10-29
duration: 1h30m
created: 2026-10-29T18:00:00+09:00
updated: 2026-10-29T18:00:00+09:00
---
ログイン API の調査
```

- [ ] **Step 2: 失敗するテストを書く**

`internal/model/comment_test.go`:
```go
package model

import (
	"strings"
	"testing"
	"time"
)

func TestCommentRoundTrip(t *testing.T) {
	src := readFile(t, "testdata/comment/basic.md")
	c, err := DecodeComment(src)
	if err != nil {
		t.Fatal(err)
	}
	if c.Author != "github.com/yourname" || c.Body != "@teammate レビューお願いします\n" || c.DeletedAt != nil {
		t.Errorf("decoded = %+v", c)
	}
	out, err := c.Encode()
	if err != nil || string(out) != string(src) {
		t.Errorf("round trip = %q, %v", out, err)
	}
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.FixedZone("", 9*3600))
	c.DeletedAt = &now
	out, _ = c.Encode()
	if want := "updated: 2026-10-29T10:05:00+09:00\ndeleted_at: 2026-10-30T09:00:00+09:00\n---\n"; !contains(out, want) {
		t.Errorf("deleted_at not written after updated:\n%s", out)
	}
}

func TestWorklogRoundTrip(t *testing.T) {
	src := readFile(t, "testdata/worklog/basic.md")
	w, err := DecodeWorklog(src)
	if err != nil {
		t.Fatal(err)
	}
	if w.Date.String() != "2026-10-29" || w.Duration.String() != "1h30m" || w.Body != "ログイン API の調査\n" {
		t.Errorf("decoded = %+v", w)
	}
	out, err := w.Encode()
	if err != nil || string(out) != string(src) {
		t.Errorf("round trip = %q, %v", out, err)
	}
	w.Duration, _ = ParseDuration("120m")
	out, _ = w.Encode()
	if !contains(out, "duration: 2h\n") {
		t.Errorf("duration not normalized:\n%s", out)
	}
}

func TestCommentAndWorklogNeedID(t *testing.T) {
	if _, err := DecodeComment([]byte("---\nauthor: x\n---\n")); err == nil {
		t.Error("DecodeComment without id succeeded")
	}
	if _, err := DecodeWorklog([]byte("---\nduration: 1d\nid: 01JBZ8K3QW7M2X9V4T6R5N8P0C\n---\n")); err == nil {
		t.Error("DecodeWorklog accepted 1d")
	}
}

// contains is shared by the tests of this package.
func contains(b []byte, s string) bool { return strings.Contains(string(b), s) }
```

- [ ] **Step 3: テストが失敗することを確かめる**

Run: `go test ./internal/model/ -run 'Comment|Worklog'`
Expected: FAIL（`undefined: DecodeComment`）

- [ ] **Step 4: 実装する**

`internal/model/comment.go`:
```go
package model

import (
	"errors"
	"time"
)

// Comment is one comment on a task. The task is known from the file's path.
type Comment struct {
	ID        ID
	Author    string
	Created   time.Time
	Updated   time.Time
	DeletedAt *time.Time
	Extra     Fields
	Body      string

	doc *Doc
}

var commentKeys = []string{"id", "author", "created", "updated", "deleted_at"}

func DecodeComment(data []byte) (*Comment, error) {
	front, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	d, err := ParseDoc(front, 1)
	if err != nil {
		return nil, err
	}
	c := &Comment{Body: body, doc: d}
	if err := decodeAll(d, map[string]any{
		"id": &c.ID, "author": &c.Author, "created": &c.Created,
		"updated": &c.Updated, "deleted_at": &c.DeletedAt,
	}); err != nil {
		return nil, err
	}
	if c.ID.IsZero() {
		return nil, &DecodeError{Field: "id", Err: errors.New("missing")}
	}
	c.Extra = d.Extra(commentKeys)
	return c, nil
}

func (c *Comment) Encode() ([]byte, error) {
	if c.doc == nil {
		c.doc = NewDoc()
	}
	e := c.doc.Encoder()
	e.Put("id", c.ID, false)
	e.Put("author", c.Author, c.Author == "")
	e.Put("created", c.Created, c.Created.IsZero())
	e.Put("updated", c.Updated, c.Updated.IsZero())
	e.Put("deleted_at", c.DeletedAt, c.DeletedAt == nil)
	front, err := e.Bytes(c.Extra)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, c.Body), nil
}

// decodeAll decodes each key into its target. Map order does not matter:
// the first error found is returned.
func decodeAll(d *Doc, targets map[string]any) error {
	for key, out := range targets {
		if err := d.Decode(key, out); err != nil {
			return err
		}
	}
	return nil
}
```

注: `decodeAll` は map の走査順が不定なので、複数のキーが同時に壊れているときに返るエラーがどれかは決まらない。どれか1つを返せば足りるので、これで良い。Task 7 の `DecodeTask` は書き出し順に並べるため slice のままにしている。

`internal/model/worklog.go`:
```go
package model

import (
	"errors"
	"time"
)

// Worklog is one entry of time spent on a task. The task is known from the
// file's path.
type Worklog struct {
	ID        ID
	Author    string
	Date      Date
	Duration  Duration
	Created   time.Time
	Updated   time.Time
	DeletedAt *time.Time
	Extra     Fields
	Body      string // a note

	doc *Doc
}

var worklogKeys = []string{"id", "author", "date", "duration", "created", "updated", "deleted_at"}

func DecodeWorklog(data []byte) (*Worklog, error) {
	front, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	d, err := ParseDoc(front, 1)
	if err != nil {
		return nil, err
	}
	w := &Worklog{Body: body, doc: d}
	if err := decodeAll(d, map[string]any{
		"id": &w.ID, "author": &w.Author, "date": &w.Date, "duration": &w.Duration,
		"created": &w.Created, "updated": &w.Updated, "deleted_at": &w.DeletedAt,
	}); err != nil {
		return nil, err
	}
	if w.ID.IsZero() {
		return nil, &DecodeError{Field: "id", Err: errors.New("missing")}
	}
	w.Extra = d.Extra(worklogKeys)
	return w, nil
}

func (w *Worklog) Encode() ([]byte, error) {
	if w.doc == nil {
		w.doc = NewDoc()
	}
	e := w.doc.Encoder()
	e.Put("id", w.ID, false)
	e.Put("author", w.Author, w.Author == "")
	e.Put("date", w.Date, w.Date.IsZero())
	e.Put("duration", w.Duration, false)
	e.Put("created", w.Created, w.Created.IsZero())
	e.Put("updated", w.Updated, w.Updated.IsZero())
	e.Put("deleted_at", w.DeletedAt, w.DeletedAt == nil)
	front, err := e.Bytes(w.Extra)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, w.Body), nil
}
```

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test -race ./internal/model/`
Expected: PASS

- [ ] **Step 6: lint を通してコミットする**

Run: `make lint`

```bash
git add internal/model/comment.go internal/model/worklog.go internal/model/comment_test.go internal/model/testdata/comment internal/model/testdata/worklog
git commit -m "feat: コメントと作業時間の記録の読み書きを追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: スプリントとリリース

**Files:**
- Create: `internal/model/sprint.go`, `internal/model/release.go`, `internal/model/testdata/sprint/basic.md`, `internal/model/testdata/release/basic.md`
- Test: `internal/model/sprint_test.go`

**Interfaces:**
- Consumes: `decodeAll`（Task 8）、`Date`、`Doc`
- Produces: `type Sprint struct{ ID ID; Name string; Start, End Date; State, Goal string; Created, Updated time.Time; Extra Fields; Body string }`、`const SprintPlanned/SprintActive/SprintClosed`、`DecodeSprint`、`(*Sprint) Encode()`。`type Release struct{ ID ID; Name string; Date Date; State string; Created, Updated time.Time; Extra Fields; Body string }`、`const ReleaseUnreleased/ReleaseReleased/ReleaseArchived`、`DecodeRelease`、`(*Release) Encode()`。

- [ ] **Step 1: テストデータを書く**

`internal/model/testdata/sprint/basic.md`:
```markdown
---
id: 01JBZ8K3QW7M2X9V4T6R5N8P0D
name: Sprint 24
start: 2026-11-02
end: 2026-11-13
state: active
goal: ログイン改修を出す
created: 2026-10-30T10:00:00+09:00
updated: 2026-11-02T09:00:00+09:00
---
## 振り返り
```

`internal/model/testdata/release/basic.md`:
```markdown
---
id: 01JBZ8K3QW7M2X9V4T6R5N8P0E
name: v2.4
date: 2026-11-20
state: unreleased
created: 2026-10-30T10:00:00+09:00
updated: 2026-10-30T10:00:00+09:00
---
```

- [ ] **Step 2: 失敗するテストを書く**

`internal/model/sprint_test.go`:
```go
package model

import "testing"

func TestSprintRoundTrip(t *testing.T) {
	src := readFile(t, "testdata/sprint/basic.md")
	s, err := DecodeSprint(src)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Sprint 24" || s.Start.String() != "2026-11-02" || s.End.String() != "2026-11-13" ||
		s.State != SprintActive || s.Goal != "ログイン改修を出す" || s.Body != "## 振り返り\n" {
		t.Errorf("decoded = %+v", s)
	}
	out, err := s.Encode()
	if err != nil || string(out) != string(src) {
		t.Errorf("round trip = %q, %v", out, err)
	}
}

func TestReleaseRoundTrip(t *testing.T) {
	src := readFile(t, "testdata/release/basic.md")
	r, err := DecodeRelease(src)
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "v2.4" || r.Date.String() != "2026-11-20" || r.State != ReleaseUnreleased || r.Body != "" {
		t.Errorf("decoded = %+v", r)
	}
	out, err := r.Encode()
	if err != nil || string(out) != string(src) {
		t.Errorf("round trip = %q, %v", out, err)
	}
	r.State = ReleaseReleased
	out, _ = r.Encode()
	if !contains(out, "state: released\n") {
		t.Errorf("state not written:\n%s", out)
	}
}

func TestSprintAndReleaseNeedID(t *testing.T) {
	if _, err := DecodeSprint([]byte("---\nname: x\n---\n")); err == nil {
		t.Error("DecodeSprint without id succeeded")
	}
	if _, err := DecodeRelease([]byte("---\nname: x\n---\n")); err == nil {
		t.Error("DecodeRelease without id succeeded")
	}
}
```

- [ ] **Step 3: テストが失敗することを確かめる**

Run: `go test ./internal/model/ -run 'Sprint|Release'`
Expected: FAIL（`undefined: DecodeSprint`）

- [ ] **Step 4: 実装する**

`internal/model/sprint.go`:
```go
package model

import (
	"errors"
	"time"
)

// Sprint states.
const (
	SprintPlanned = "planned"
	SprintActive  = "active"
	SprintClosed  = "closed"
)

// Sprint is one sprint file. The body holds notes such as a retrospective.
type Sprint struct {
	ID      ID
	Name    string
	Start   Date
	End     Date
	State   string
	Goal    string
	Created time.Time
	Updated time.Time
	Extra   Fields
	Body    string

	doc *Doc
}

var sprintKeys = []string{"id", "name", "start", "end", "state", "goal", "created", "updated"}

func DecodeSprint(data []byte) (*Sprint, error) {
	front, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	d, err := ParseDoc(front, 1)
	if err != nil {
		return nil, err
	}
	s := &Sprint{Body: body, doc: d}
	if err := decodeAll(d, map[string]any{
		"id": &s.ID, "name": &s.Name, "start": &s.Start, "end": &s.End, "state": &s.State,
		"goal": &s.Goal, "created": &s.Created, "updated": &s.Updated,
	}); err != nil {
		return nil, err
	}
	if s.ID.IsZero() {
		return nil, &DecodeError{Field: "id", Err: errors.New("missing")}
	}
	s.Extra = d.Extra(sprintKeys)
	return s, nil
}

func (s *Sprint) Encode() ([]byte, error) {
	if s.doc == nil {
		s.doc = NewDoc()
	}
	e := s.doc.Encoder()
	e.Put("id", s.ID, false)
	e.Put("name", s.Name, s.Name == "")
	e.Put("start", s.Start, s.Start.IsZero())
	e.Put("end", s.End, s.End.IsZero())
	e.Put("state", s.State, s.State == "")
	e.Put("goal", s.Goal, s.Goal == "")
	e.Put("created", s.Created, s.Created.IsZero())
	e.Put("updated", s.Updated, s.Updated.IsZero())
	front, err := e.Bytes(s.Extra)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, s.Body), nil
}
```

`internal/model/release.go`:
```go
package model

import (
	"errors"
	"time"
)

// Release states.
const (
	ReleaseUnreleased = "unreleased"
	ReleaseReleased   = "released"
	ReleaseArchived   = "archived"
)

// Release is one release file. Date is the planned date.
type Release struct {
	ID      ID
	Name    string
	Date    Date
	State   string
	Created time.Time
	Updated time.Time
	Extra   Fields
	Body    string

	doc *Doc
}

var releaseKeys = []string{"id", "name", "date", "state", "created", "updated"}

func DecodeRelease(data []byte) (*Release, error) {
	front, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	d, err := ParseDoc(front, 1)
	if err != nil {
		return nil, err
	}
	r := &Release{Body: body, doc: d}
	if err := decodeAll(d, map[string]any{
		"id": &r.ID, "name": &r.Name, "date": &r.Date, "state": &r.State,
		"created": &r.Created, "updated": &r.Updated,
	}); err != nil {
		return nil, err
	}
	if r.ID.IsZero() {
		return nil, &DecodeError{Field: "id", Err: errors.New("missing")}
	}
	r.Extra = d.Extra(releaseKeys)
	return r, nil
}

func (r *Release) Encode() ([]byte, error) {
	if r.doc == nil {
		r.doc = NewDoc()
	}
	e := r.doc.Encoder()
	e.Put("id", r.ID, false)
	e.Put("name", r.Name, r.Name == "")
	e.Put("date", r.Date, r.Date.IsZero())
	e.Put("state", r.State, r.State == "")
	e.Put("created", r.Created, r.Created.IsZero())
	e.Put("updated", r.Updated, r.Updated.IsZero())
	front, err := e.Bytes(r.Extra)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, r.Body), nil
}
```

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test -race ./internal/model/`
Expected: PASS

- [ ] **Step 6: lint を通してコミットする**

Run: `make lint`

```bash
git add internal/model/sprint.go internal/model/release.go internal/model/sprint_test.go internal/model/testdata/sprint internal/model/testdata/release
git commit -m "feat: スプリントとリリースの読み書きを追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: テンプレート

**Files:**
- Create: `internal/model/template.go`, `internal/model/testdata/template/release-checklist.md`
- Test: `internal/model/template_test.go`

**Interfaces:**
- Consumes: `Fields`、`Link`、`ID`、`decodeAll`、`Doc`
- Produces:
  - `type Assignees struct{ Inherit bool; Logins []string }`（`IsZero`、YAML の Marshal/Unmarshal。`inherit` は親の担当者を引き継ぐ）
  - `type TemplateItem struct{ KeyPrefix, Kind, Title, Status string; Assignees Assignees; Watchers, Labels, Repos []string; BlockedBy []ID; Sprint *ID; Releases []ID; Links []Link; Props Fields; Body string; Children []TemplateItem }`（YAML タグ付き）
  - `type Template struct{ Name string; Item TemplateItem; Created, Updated time.Time; Extra Fields }`（`Item.Body` はファイルの本文）
  - `DecodeTemplate(name string, data []byte) (*Template, error)`、`(*Template) Encode() ([]byte, error)`

テンプレートには `id` がない（名前はファイル名で決まる）ので、`DecodeTemplate` は何も必須にしない。

- [ ] **Step 1: テストデータを書く**

`internal/model/testdata/template/release-checklist.md`:
```markdown
---
kind: task
labels: [release]
props:
  priority: high
children:
- {title: 切り替え前日の確認, assignees: inherit}
- {title: 切り替え作業, assignees: inherit, props: {points: 2}}
- {title: 切り替え後の監視（24 時間）}
created: 2026-10-30T10:00:00+09:00
updated: 2026-10-30T10:00:00+09:00
---
## 手順
- [ ] 切り替え手順を確認する
```

- [ ] **Step 2: 失敗するテストを書く**

`internal/model/template_test.go`:
```go
package model

import (
	"strings"
	"testing"
)

func TestTemplateRoundTrip(t *testing.T) {
	src := readFile(t, "testdata/template/release-checklist.md")
	tp, err := DecodeTemplate("release-checklist", src)
	if err != nil {
		t.Fatal(err)
	}
	if tp.Name != "release-checklist" || tp.Item.Kind != "task" || len(tp.Item.Children) != 3 ||
		!tp.Item.Children[0].Assignees.Inherit || tp.Item.Children[1].Props.Get("points").Value != "2" ||
		!tp.Item.Children[2].Assignees.IsZero() || !strings.HasPrefix(tp.Item.Body, "## 手順") {
		t.Errorf("decoded = %+v", tp)
	}
	out, err := tp.Encode()
	if err != nil || string(out) != string(src) {
		t.Errorf("round trip = %q, %v", out, err)
	}
}

func TestNewTemplateEncode(t *testing.T) {
	var points Fields
	points.Set("points", node(t, 2))
	tp := &Template{Name: "x", Item: TemplateItem{Kind: "task", Children: []TemplateItem{
		{Title: "a", Assignees: Assignees{Inherit: true}},
		{Title: "b", Props: points, Children: []TemplateItem{{Title: "c"}}},
	}}}
	out, err := tp.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !contains(out, "- {title: a, assignees: inherit}\n") {
		t.Errorf("short child is not in flow style:\n%s", out)
	}
	back, err := DecodeTemplate("x", out)
	if err != nil {
		t.Fatal(err)
	}
	b := back.Item.Children[1]
	if b.Title != "b" || b.Props.Get("points").Value != "2" || len(b.Children) != 1 || b.Children[0].Title != "c" {
		t.Errorf("re-decoded = %+v", back.Item)
	}
}

func TestAssigneesYAML(t *testing.T) {
	tp, err := DecodeTemplate("x", []byte("---\nassignees: [github.com/a]\n---\n"))
	if err != nil || tp.Item.Assignees.Inherit || len(tp.Item.Assignees.Logins) != 1 {
		t.Errorf("decoded = %+v, %v", tp, err)
	}
	if _, err := DecodeTemplate("x", []byte("---\nassignees: {a: b}\n---\n")); err == nil {
		t.Error("DecodeTemplate accepted a mapping for assignees")
	}
}
```

- [ ] **Step 3: テストが失敗することを確かめる**

Run: `go test ./internal/model/ -run 'Template|Assignees'`
Expected: FAIL（`undefined: DecodeTemplate`）

- [ ] **Step 4: 実装する**

`internal/model/template.go`:
```go
package model

import (
	"fmt"
	"time"

	"go.yaml.in/yaml/v4"
)

// Assignees are the assignees of a template item: a list of logins, or
// "inherit" to take the parent's assignees when the template is expanded.
type Assignees struct {
	Inherit bool
	Logins  []string
}

func (a Assignees) IsZero() bool { return !a.Inherit && len(a.Logins) == 0 }

func (a Assignees) MarshalYAML() (any, error) {
	if a.Inherit {
		return "inherit", nil
	}
	return a.Logins, nil
}

func (a *Assignees) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode && n.Value == "inherit" {
		*a = Assignees{Inherit: true}
		return nil
	}
	logins, err := scalars(n)
	if err != nil {
		return fmt.Errorf("line %d: assignees: %w", n.Line, err)
	}
	*a = Assignees{Logins: logins}
	return nil
}

// TemplateItem is a task to create from a template. The parent and the rank
// are decided when the template is expanded.
type TemplateItem struct {
	KeyPrefix string         `yaml:"key_prefix,omitempty"`
	Kind      string         `yaml:"kind,omitempty"`
	Title     string         `yaml:"title,omitempty"`
	Status    string         `yaml:"status,omitempty"`
	Assignees Assignees      `yaml:"assignees,omitempty"`
	Watchers  []string       `yaml:"watchers,omitempty"`
	Labels    []string       `yaml:"labels,omitempty"`
	Repos     []string       `yaml:"repos,omitempty"`
	BlockedBy []ID           `yaml:"blocked_by,omitempty"`
	Sprint    *ID            `yaml:"sprint,omitempty"`
	Releases  []ID           `yaml:"releases,omitempty"`
	Links     []Link         `yaml:"links,omitempty"`
	Props     Fields         `yaml:"props,omitempty"`
	Body      string         `yaml:"body,omitempty"`
	Children  []TemplateItem `yaml:"children,omitempty"`
}

// Template is one template file. Its name is the file name. Item.Body is the
// body of the file.
type Template struct {
	Name    string
	Item    TemplateItem
	Created time.Time
	Updated time.Time
	Extra   Fields

	doc *Doc
}

var templateKeys = []string{
	"key_prefix", "kind", "title", "status", "assignees", "watchers", "labels", "repos",
	"blocked_by", "sprint", "releases", "links", "props", "children", "created", "updated",
}

func DecodeTemplate(name string, data []byte) (*Template, error) {
	front, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	d, err := ParseDoc(front, 1)
	if err != nil {
		return nil, err
	}
	t := &Template{Name: name, doc: d}
	it := &t.Item
	if err := decodeAll(d, map[string]any{
		"key_prefix": &it.KeyPrefix, "kind": &it.Kind, "title": &it.Title, "status": &it.Status,
		"assignees": &it.Assignees, "watchers": &it.Watchers, "labels": &it.Labels,
		"repos": &it.Repos, "blocked_by": &it.BlockedBy, "sprint": &it.Sprint,
		"releases": &it.Releases, "links": &it.Links, "props": &it.Props,
		"children": &it.Children, "created": &t.Created, "updated": &t.Updated,
	}); err != nil {
		return nil, err
	}
	it.Body = body
	t.Extra = d.Extra(templateKeys)
	return t, nil
}

func (t *Template) Encode() ([]byte, error) {
	if t.doc == nil {
		t.doc = NewDoc()
	}
	it := t.Item
	e := t.doc.Encoder()
	e.Put("key_prefix", it.KeyPrefix, it.KeyPrefix == "")
	e.Put("kind", it.Kind, it.Kind == "")
	e.Put("title", it.Title, it.Title == "")
	e.Put("status", it.Status, it.Status == "")
	e.Put("assignees", it.Assignees, it.Assignees.IsZero())
	e.Put("watchers", it.Watchers, len(it.Watchers) == 0)
	e.Put("labels", it.Labels, len(it.Labels) == 0)
	e.Put("repos", it.Repos, len(it.Repos) == 0)
	e.Put("blocked_by", it.BlockedBy, len(it.BlockedBy) == 0)
	e.Put("sprint", it.Sprint, it.Sprint == nil)
	e.Put("releases", it.Releases, len(it.Releases) == 0)
	e.Put("links", it.Links, len(it.Links) == 0)
	e.Put("props", mappingOf(t.doc.Get("props"), it.Props), len(it.Props) == 0)
	e.Put("children", it.Children, len(it.Children) == 0)
	e.Put("created", t.Created, t.Created.IsZero())
	e.Put("updated", t.Updated, t.Updated.IsZero())
	front, err := e.Bytes(t.Extra)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, it.Body), nil
}
```

注: `children` の要素の文字列は yaml の通常のデコードで読むので、`title: 2026-11-03` のような日付に見えるタイトルはデコードエラーになる。トップレベルのタスクファイルでは起きない（`Doc.Decode` が生の値を読む）。テンプレートの子でこれが問題になったら、`TemplateItem` に `UnmarshalYAML` を足して `Doc` と同じ読み方にする。

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test -race ./internal/model/`
Expected: PASS

- [ ] **Step 6: lint を通してコミットする**

Run: `make lint`

```bash
git add internal/model/template.go internal/model/template_test.go internal/model/testdata/template
git commit -m "feat: 子タスクを含むテンプレートの読み書きを追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: `store.yaml`

**Files:**
- Create: `internal/model/storemeta.go`
- Test: `internal/model/storemeta_test.go`

**Interfaces:**
- Consumes: `Doc`、`decodeAll`、`normalizeNewlines`
- Produces: `const StoreFormat = 1`、`var ErrNewerFormat`、`type StoreMeta struct{ ID ID; Name string; Format int; KeyPrefix, Audience string; Extra Fields }`、`DecodeStoreMeta([]byte) (*StoreMeta, error)`、`(*StoreMeta) Encode() ([]byte, error)`

- [ ] **Step 1: 失敗するテストを書く**

`internal/model/storemeta_test.go`:
```go
package model

import (
	"errors"
	"testing"
)

func TestStoreMetaRoundTrip(t *testing.T) {
	src := "id: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nname: shared\nformat: 1\nkey_prefix: SH\naudience: backend と frontend の両チーム\n"
	m, err := DecodeStoreMeta([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "shared" || m.Format != 1 || m.KeyPrefix != "SH" || m.Audience == "" {
		t.Errorf("decoded = %+v", m)
	}
	out, err := m.Encode()
	if err != nil || string(out) != src {
		t.Errorf("round trip = %q, %v", out, err)
	}
}

func TestStoreMetaErrors(t *testing.T) {
	_, err := DecodeStoreMeta([]byte("id: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nformat: 2\n"))
	if !errors.Is(err, ErrNewerFormat) {
		t.Errorf("format 2: err = %v, want ErrNewerFormat", err)
	}
	for _, src := range []string{
		"format: 1\n",                          // no id
		"id: 01JBZ8K3QW7M2X9V4T6R5N8P0A\n",     // no format
		"id: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nformat: x\n",
	} {
		if _, err := DecodeStoreMeta([]byte(src)); err == nil {
			t.Errorf("DecodeStoreMeta(%q) succeeded", src)
		}
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/model/ -run StoreMeta`
Expected: FAIL（`undefined: DecodeStoreMeta`）

- [ ] **Step 3: 実装する**

`internal/model/storemeta.go`:
```go
package model

import (
	"errors"
	"fmt"
)

// StoreFormat is the newest store format this version reads and writes.
const StoreFormat = 1

// ErrNewerFormat means the store was written by a newer Hedgehug.
var ErrNewerFormat = errors.New("the store uses a newer format; update Hedgehug")

// StoreMeta is .hedgehug/store.yaml.
type StoreMeta struct {
	ID        ID
	Name      string
	Format    int
	KeyPrefix string // default key prefix for tasks outside any project
	Audience  string // who shares the store, shown in the GUI
	Extra     Fields

	doc *Doc
}

var storeMetaKeys = []string{"id", "name", "format", "key_prefix", "audience"}

// DecodeStoreMeta reads store.yaml. id and format are required, and a format
// newer than StoreFormat is refused with ErrNewerFormat.
func DecodeStoreMeta(data []byte) (*StoreMeta, error) {
	d, err := ParseDoc([]byte(normalizeNewlines(string(data))), 0)
	if err != nil {
		return nil, err
	}
	m := &StoreMeta{doc: d}
	if err := decodeAll(d, map[string]any{
		"id": &m.ID, "name": &m.Name, "format": &m.Format,
		"key_prefix": &m.KeyPrefix, "audience": &m.Audience,
	}); err != nil {
		return nil, err
	}
	switch {
	case m.ID.IsZero():
		return nil, &DecodeError{Field: "id", Err: errors.New("missing")}
	case m.Format == 0:
		return nil, &DecodeError{Field: "format", Err: errors.New("missing")}
	case m.Format > StoreFormat:
		return nil, fmt.Errorf("format %d (this version reads up to %d): %w", m.Format, StoreFormat, ErrNewerFormat)
	}
	m.Extra = d.Extra(storeMetaKeys)
	return m, nil
}

func (m *StoreMeta) Encode() ([]byte, error) {
	if m.doc == nil {
		m.doc = NewDoc()
	}
	e := m.doc.Encoder()
	e.Put("id", m.ID, false)
	e.Put("name", m.Name, m.Name == "")
	e.Put("format", m.Format, false)
	e.Put("key_prefix", m.KeyPrefix, m.KeyPrefix == "")
	e.Put("audience", m.Audience, m.Audience == "")
	return e.Bytes(m.Extra)
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/model/`
Expected: PASS

- [ ] **Step 5: lint を通してコミットする**

Run: `make lint`

```bash
git add internal/model/storemeta.go internal/model/storemeta_test.go
git commit -m "feat: store.yaml の読み書きとフォーマット番号の検査を追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: スキーマの型・codec・既定値

**Files:**
- Create: `internal/schema/schema.go`, `internal/schema/testdata/full.yaml`
- Test: `internal/schema/schema_test.go`

**Interfaces:**
- Consumes: `model.Doc`、`model.ParseDoc`、`model.NewDoc`、`model.Fields`、`model.NodeEqual`
- Produces:
  - `type Category string`（`CategoryTodo/Doing/Done/Canceled`、`var Categories []Category`）
  - `type Status struct{ Name, Label string; Category Category; Color string; Default bool }`
  - `type Role string`（`RoleContainer/RoleWork`）、`const Root = "root"`
  - `type Kind struct{ Name, Label string; Role Role; Parents []string; Icon, Template string; Default bool }`
  - `type PropType string`（`TypeText/Number/Bool/Date/Datetime/Duration/Enum/MultiEnum/User/TaskRef/URL`、`var PropTypes []PropType`）
  - `type Prop struct{ Name string; Type PropType; Values []string; Multi bool; Label, Inverse string; Symmetric bool }`、`type Props []Prop`
  - `type List struct{ Key string; Values []string }`、`type Lists []List`、`(Lists) Get(key) ([]string, bool)`
  - `type Workflow struct{ Transitions, Required Lists }`、`type Templates struct{ Only []string; Default map[string]string }`
  - `type Scoped struct{ Where string; Statuses []string; Workflow *Workflow; Templates *Templates }`
  - `type Schema struct{ Statuses []Status; Kinds []Kind; Props Props; Workflow Workflow; Scoped []Scoped; Extra model.Fields }`
  - `const Path = "schema.yaml"`、`Decode([]byte) (*Schema, error)`、`(*Schema) Encode() ([]byte, error)`、`Default() *Schema`、`Dev() *Schema`
  - 参照: `(*Schema) Status(name) (Status, bool)`、`Kind(name) (Kind, bool)`、`Prop(name) (Prop, bool)`、`DefaultStatus(Category) (Status, bool)`、`DefaultKind() (Kind, bool)`

- [ ] **Step 1: テストデータを書く**

`internal/schema/testdata/full.yaml`（親設計の例をまとめたもの。正準形なので往復で変わらない）:
```yaml
# kinds は「種類と階層」、workflow は「ワークフローの制約」を参照
statuses:
- {name: backlog, label: バックログ, category: todo}
- {name: todo, label: 未着手, category: todo, default: true}
- {name: on_hold, label: 保留, category: todo}
- {name: in_progress, label: 進行中, category: doing, color: '#3451D1', default: true}
- {name: waiting, label: 待機中, category: doing}
- {name: done, label: 完了, category: done, default: true}
- {name: canceled, label: 中止, category: canceled, default: true}
kinds:
- {name: project, label: プロジェクト, role: container}
- {name: epic, label: エピック, role: container, parents: [root, project]}
- {name: story, label: ストーリー, role: work, parents: [root, project, epic]}
- {name: task, label: タスク, role: work, parents: [root, project, epic, story, task], default: true}
- {name: bug, label: バグ, role: work, parents: [root, project, epic, story], template: bug-report}
props:
  priority: {type: enum, values: [low, mid, high, urgent]}
  start: {type: date}
  estimate: {type: duration}
  area: {type: multi_enum, values: [backend, frontend, infra, ops]}
  duplicates: {type: task_ref, multi: true, label: 重複している, inverse: 重複されている}
  relates: {type: task_ref, multi: true, label: 関連, symmetric: true}
workflow:
  transitions:
    backlog: [todo, canceled]
    done: [todo]
  required:
    in_progress: [assignees]
    done: [points]
scoped:
- where: project:01JBZ8K3QW7M2X9V4T6R5N8P0B # リリース準備 v2.4
  workflow:
    transitions:
      waiting: [in_progress, canceled]
    required:
      done: [points, estimate]
  templates:
    only: [release-checklist, api-change]
    default:
      task: release-checklist
- {where: 'project:01JBZ8K3QW7M2X9V4T6R5N8P0C', statuses: [todo, on_hold, in_progress, done, canceled]}
```

注: flow（`{…}`・`[…]`）の中では、`:` を含むスカラーを emitter が必ず引用符で囲む（スパイクで確認済み）。正準形のテストデータでは最初から `'project:…'` と書いておく。block の中の `where: project:…` は引用符なしのまま残る。

- [ ] **Step 2: 失敗するテストを書く**

`internal/schema/schema_test.go`:
```go
package schema

import (
	"os"
	"strings"
	"testing"
)

func readFull(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/full.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSchemaRoundTrip(t *testing.T) {
	src := readFull(t)
	s, err := Decode(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Statuses) != 7 || s.Statuses[3].Color != "#3451D1" || len(s.Kinds) != 5 ||
		s.Kinds[4].Template != "bug-report" || len(s.Props) != 6 || s.Props[0].Name != "priority" ||
		!s.Props[5].Symmetric || len(s.Scoped) != 2 || s.Scoped[0].Templates.Default["task"] != "release-checklist" {
		t.Errorf("decoded = %+v", s)
	}
	if got, ok := s.Workflow.Transitions.Get("backlog"); !ok || strings.Join(got, ",") != "todo,canceled" {
		t.Errorf("transitions[backlog] = %v, %v", got, ok)
	}
	out, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(src) {
		t.Errorf("round trip changed the file:\n%s", out)
	}
}

func TestSchemaEditKeepsComments(t *testing.T) {
	s, err := Decode(readFull(t))
	if err != nil {
		t.Fatal(err)
	}
	s.Statuses = append(s.Statuses, Status{Name: "review", Label: "レビュー", Category: CategoryDoing})
	out, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# kinds は「種類と階層」",
		"- {name: review, label: レビュー, category: doing}\n",
		"# リリース準備 v2.4",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestDefaultEncodesAndDecodes(t *testing.T) {
	for name, s := range map[string]*Schema{"Default": Default(), "Dev": Dev()} {
		out, err := s.Encode()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		back, err := Decode(out)
		if err != nil {
			t.Fatalf("%s: %v\n%s", name, err, out)
		}
		if len(back.Statuses) != len(s.Statuses) || len(back.Kinds) != 5 || len(back.Props) != 8 {
			t.Errorf("%s: re-decoded = %+v", name, back)
		}
		if !strings.Contains(string(out), "- {name: todo, label: 未着手, category: todo, default: true}\n") {
			t.Errorf("%s: statuses are not in flow style:\n%s", name, out)
		}
	}
	if len(Default().Statuses) != 7 || len(Dev().Statuses) != 8 {
		t.Error("Default must have 7 statuses and Dev 8")
	}
	if st, ok := Dev().Status("review"); !ok || st.Category != CategoryDoing {
		t.Errorf("Dev review = %+v, %v", st, ok)
	}
	if k, ok := Default().Kind("project"); !ok || k.Parents != nil {
		t.Errorf("project kind = %+v: parents must be omitted (anywhere)", k)
	}
}

func TestLookups(t *testing.T) {
	s := Default()
	if st, ok := s.DefaultStatus(CategoryDoing); !ok || st.Name != "in_progress" {
		t.Errorf("DefaultStatus(doing) = %+v, %v", st, ok)
	}
	if k, ok := s.DefaultKind(); !ok || k.Name != "task" {
		t.Errorf("DefaultKind = %+v, %v", k, ok)
	}
	if p, ok := s.Prop("estimate"); !ok || p.Type != TypeDuration {
		t.Errorf("Prop(estimate) = %+v, %v", p, ok)
	}
	if _, ok := s.Status("nope"); ok {
		t.Error("Status(nope) found")
	}
}

func TestDecodeErrors(t *testing.T) {
	for _, src := range []string{
		"statuses: {a: b}\n",
		"props: [a]\n",
		"workflow:\n  transitions: [a]\n",
	} {
		if _, err := Decode([]byte(src)); err == nil {
			t.Errorf("Decode(%q) succeeded", src)
		}
	}
}
```

- [ ] **Step 3: テストが失敗することを確かめる**

Run: `go test ./internal/schema/`
Expected: FAIL（`undefined: Decode`）

- [ ] **Step 4: 実装する**

`internal/schema/schema.go`:
```go
// Package schema reads schema.yaml and checks tasks against it.
package schema

import (
	"fmt"

	"github.com/kukv/hedgehug/internal/model"
	"go.yaml.in/yaml/v4"
)

// Path is where a store keeps its schema.
const Path = "schema.yaml"

// Category groups statuses so that views across stores line up.
type Category string

const (
	CategoryTodo     Category = "todo"
	CategoryDoing    Category = "doing"
	CategoryDone     Category = "done"
	CategoryCanceled Category = "canceled"
)

// Categories lists every category.
var Categories = []Category{CategoryTodo, CategoryDoing, CategoryDone, CategoryCanceled}

type Status struct {
	Name     string   `yaml:"name"`
	Label    string   `yaml:"label,omitempty"`
	Category Category `yaml:"category"`
	Color    string   `yaml:"color,omitempty"`
	Default  bool     `yaml:"default,omitempty"`
}

// Role says what a kind of task is for.
type Role string

const (
	RoleContainer Role = "container" // groups work, like a project or an epic
	RoleWork      Role = "work"      // a work item
)

// Root stands for "no parent" in Kind.Parents.
const Root = "root"

type Kind struct {
	Name     string   `yaml:"name"`
	Label    string   `yaml:"label,omitempty"`
	Role     Role     `yaml:"role"`
	Parents  []string `yaml:"parents,omitempty"` // nil: anywhere
	Icon     string   `yaml:"icon,omitempty"`
	Template string   `yaml:"template,omitempty"`
	Default  bool     `yaml:"default,omitempty"`
}

type PropType string

const (
	TypeText      PropType = "text"
	TypeNumber    PropType = "number"
	TypeBool      PropType = "bool"
	TypeDate      PropType = "date"
	TypeDatetime  PropType = "datetime"
	TypeDuration  PropType = "duration"
	TypeEnum      PropType = "enum"
	TypeMultiEnum PropType = "multi_enum"
	TypeUser      PropType = "user"
	TypeTaskRef   PropType = "task_ref"
	TypeURL       PropType = "url"
)

// PropTypes lists every property type.
var PropTypes = []PropType{
	TypeText, TypeNumber, TypeBool, TypeDate, TypeDatetime, TypeDuration,
	TypeEnum, TypeMultiEnum, TypeUser, TypeTaskRef, TypeURL,
}

type Prop struct {
	Name      string   `yaml:"-"`
	Type      PropType `yaml:"type"`
	Values    []string `yaml:"values,omitempty"`
	Multi     bool     `yaml:"multi,omitempty"`
	Label     string   `yaml:"label,omitempty"`
	Inverse   string   `yaml:"inverse,omitempty"`
	Symmetric bool     `yaml:"symmetric,omitempty"`
}

// Props keeps properties in file order. In YAML it is a mapping from name
// to definition.
type Props []Prop

func (p *Props) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: props must be a mapping", n.Line)
	}
	out := make(Props, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		var pr Prop
		if err := n.Content[i+1].Decode(&pr); err != nil {
			return err
		}
		pr.Name = n.Content[i].Value
		out = append(out, pr)
	}
	*p = out
	return nil
}

func (p Props) MarshalYAML() (any, error) {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, pr := range p {
		var v yaml.Node
		if err := v.Encode(pr); err != nil {
			return nil, err
		}
		v.Style = yaml.FlowStyle
		m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: pr.Name}, &v)
	}
	return m, nil
}

// List is one key of Lists.
type List struct {
	Key    string
	Values []string
}

// Lists is an ordered mapping from a name to a list of names, as in
// workflow.transitions and workflow.required.
type Lists []List

// Get returns the list for key.
func (l Lists) Get(key string) ([]string, bool) {
	for _, x := range l {
		if x.Key == key {
			return x.Values, true
		}
	}
	return nil, false
}

func (l *Lists) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: want a mapping of lists", n.Line)
	}
	out := make(Lists, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		var v []string
		if err := n.Content[i+1].Decode(&v); err != nil {
			return err
		}
		out = append(out, List{n.Content[i].Value, v})
	}
	*l = out
	return nil
}

func (l Lists) MarshalYAML() (any, error) {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, x := range l {
		var v yaml.Node
		if err := v.Encode(x.Values); err != nil {
			return nil, err
		}
		v.Style = yaml.FlowStyle
		m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: x.Key}, &v)
	}
	return m, nil
}

type Workflow struct {
	Transitions Lists `yaml:"transitions,omitempty"` // status → statuses it may move to
	Required    Lists `yaml:"required,omitempty"`    // status → fields needed to enter it
}

func (w Workflow) IsZero() bool { return len(w.Transitions) == 0 && len(w.Required) == 0 }

type Templates struct {
	Only    []string          `yaml:"only,omitempty"`
	Default map[string]string `yaml:"default,omitempty"` // kind → template
}

// Scoped overrides behavior for one project and its descendants.
type Scoped struct {
	Where     string     `yaml:"where"` // project:<ULID>
	Statuses  []string   `yaml:"statuses,omitempty"`
	Workflow  *Workflow  `yaml:"workflow,omitempty"`
	Templates *Templates `yaml:"templates,omitempty"`
}

// Schema is schema.yaml.
type Schema struct {
	Statuses []Status
	Kinds    []Kind
	Props    Props
	Workflow Workflow
	Scoped   []Scoped
	Extra    model.Fields

	doc *model.Doc
}

var schemaKeys = []string{"statuses", "kinds", "props", "workflow", "scoped"}

// Decode reads schema.yaml. It does not check the schema; see Check.
func Decode(data []byte) (*Schema, error) {
	d, err := model.ParseDoc(data, 0)
	if err != nil {
		return nil, err
	}
	s := &Schema{doc: d}
	for _, f := range []struct {
		key string
		out any
	}{
		{"statuses", &s.Statuses}, {"kinds", &s.Kinds}, {"props", &s.Props},
		{"workflow", &s.Workflow}, {"scoped", &s.Scoped},
	} {
		if err := d.Decode(f.key, f.out); err != nil {
			return nil, err
		}
	}
	s.Extra = d.Extra(schemaKeys)
	return s, nil
}

// Encode writes schema.yaml, keeping comments and unchanged parts as written.
func (s *Schema) Encode() ([]byte, error) {
	if s.doc == nil {
		s.doc = model.NewDoc()
	}
	e := s.doc.Encoder()
	e.Put("statuses", s.Statuses, len(s.Statuses) == 0)
	e.Put("kinds", s.Kinds, len(s.Kinds) == 0)
	e.Put("props", s.Props, len(s.Props) == 0)
	e.Put("workflow", s.Workflow, s.Workflow.IsZero())
	e.Put("scoped", s.Scoped, len(s.Scoped) == 0)
	return e.Bytes(s.Extra)
}

// Default returns the schema of a new store: seven statuses in four
// categories, five kinds and the common properties.
func Default() *Schema {
	return &Schema{
		Statuses: []Status{
			{Name: "backlog", Label: "バックログ", Category: CategoryTodo},
			{Name: "todo", Label: "未着手", Category: CategoryTodo, Default: true},
			{Name: "on_hold", Label: "保留", Category: CategoryTodo},
			{Name: "in_progress", Label: "進行中", Category: CategoryDoing, Default: true},
			{Name: "waiting", Label: "待機中", Category: CategoryDoing},
			{Name: "done", Label: "完了", Category: CategoryDone, Default: true},
			{Name: "canceled", Label: "中止", Category: CategoryCanceled, Default: true},
		},
		Kinds: []Kind{
			{Name: "project", Label: "プロジェクト", Role: RoleContainer},
			{Name: "epic", Label: "エピック", Role: RoleContainer, Parents: []string{Root, "project"}},
			{Name: "story", Label: "ストーリー", Role: RoleWork, Parents: []string{Root, "project", "epic"}},
			{Name: "task", Label: "タスク", Role: RoleWork, Parents: []string{Root, "project", "epic", "story", "task"}, Default: true},
			{Name: "bug", Label: "バグ", Role: RoleWork, Parents: []string{Root, "project", "epic", "story"}},
		},
		Props: Props{
			{Name: "priority", Type: TypeEnum, Values: []string{"low", "mid", "high", "urgent"}},
			{Name: "start", Type: TypeDate},
			{Name: "due", Type: TypeDate},
			{Name: "estimate", Type: TypeDuration},
			{Name: "points", Type: TypeNumber},
			{Name: "duplicates", Type: TypeTaskRef, Multi: true, Label: "重複している", Inverse: "重複されている"},
			{Name: "causes", Type: TypeTaskRef, Multi: true, Label: "引き起こしている", Inverse: "起因している"},
			{Name: "relates", Type: TypeTaskRef, Multi: true, Label: "関連", Symmetric: true},
		},
	}
}

// Dev returns Default plus a review status, for stores that track
// development (hh store init --template dev).
func Dev() *Schema {
	s := Default()
	s.Statuses = append(s.Statuses[:4:4], append([]Status{
		{Name: "review", Label: "レビュー", Category: CategoryDoing},
	}, s.Statuses[4:]...)...)
	return s
}

func (s *Schema) Status(name string) (Status, bool) {
	for _, st := range s.Statuses {
		if st.Name == name {
			return st, true
		}
	}
	return Status{}, false
}

func (s *Schema) Kind(name string) (Kind, bool) {
	for _, k := range s.Kinds {
		if k.Name == name {
			return k, true
		}
	}
	return Kind{}, false
}

func (s *Schema) Prop(name string) (Prop, bool) {
	for _, p := range s.Props {
		if p.Name == name {
			return p, true
		}
	}
	return Prop{}, false
}

// DefaultStatus returns the default status of category c. hh start,
// automations and moves between stores use it.
func (s *Schema) DefaultStatus(c Category) (Status, bool) {
	for _, st := range s.Statuses {
		if st.Category == c && st.Default {
			return st, true
		}
	}
	return Status{}, false
}

// DefaultKind returns the default kind of the work role.
func (s *Schema) DefaultKind() (Kind, bool) {
	for _, k := range s.Kinds {
		if k.Role == RoleWork && k.Default {
			return k, true
		}
	}
	return Kind{}, false
}
```

注: `Dev` の `s.Statuses[:4:4]` は容量を4に絞り、`append` が元の配列の後半を上書きしないようにするため。

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test -race ./internal/schema/`
Expected: PASS。`TestSchemaRoundTrip` がスタイルの差だけで落ちたら、Task 7 Step 5 と同じ手順で切り分ける（`#3451D1` の引用符は v4 の `QuoteSingle` 既定に合わせて `'#3451D1'` にしてある）。

- [ ] **Step 6: lint を通してコミットする**

Run: `make lint`

```bash
git add internal/schema/schema.go internal/schema/schema_test.go internal/schema/testdata/full.yaml
git commit -m "feat: schema.yaml の読み書きと既定のスキーマを追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: スキーマ自体の検査

**Files:**
- Create: `internal/schema/check.go`
- Test: `internal/schema/check_test.go`

**Interfaces:**
- Consumes: Task 12 の型、`model.TaskKeys()`、`model.ParseID`
- Produces: `type Problem struct{ Path, Msg string }`、`(*Schema) Check() []Problem`、パッケージ内 `parseWhere(string) (model.ID, error)`、`var requirableFields []string`、`builtinFilled(*model.Task, string) (filled, ok bool)`（Task 15 も使う）

- [ ] **Step 1: 失敗するテストを書く**

`internal/schema/check_test.go`:
```go
package schema

import (
	"strings"
	"testing"
)

func TestCheckDefaults(t *testing.T) {
	for name, s := range map[string]*Schema{"Default": Default(), "Dev": Dev()} {
		if ps := s.Check(); len(ps) != 0 {
			t.Errorf("%s has problems: %v", name, ps)
		}
	}
	s, err := Decode(readFull(t))
	if err != nil {
		t.Fatal(err)
	}
	s.Props = append(s.Props, Prop{Name: "points", Type: TypeNumber}) // full.yaml requires points
	if ps := s.Check(); len(ps) != 0 {
		t.Errorf("full.yaml has problems: %v", ps)
	}
}

func TestCheckProblems(t *testing.T) {
	where := "project:01JBZ8K3QW7M2X9V4T6R5N8P0B"
	for _, tc := range []struct {
		name   string
		edit   func(s *Schema)
		inPath string
	}{
		{"duplicate status", func(s *Schema) { s.Statuses = append(s.Statuses, s.Statuses[0]) }, "statuses[7]"},
		{"empty status name", func(s *Schema) { s.Statuses[0].Name = "" }, "statuses[0]"},
		{"unknown category", func(s *Schema) { s.Statuses[0].Category = "x" }, "statuses[0]"},
		{"two defaults", func(s *Schema) { s.Statuses[0].Default = true }, "statuses"},
		{"no default", func(s *Schema) { s.Statuses[6].Default = false }, "statuses"},
		{"duplicate kind", func(s *Schema) { s.Kinds = append(s.Kinds, s.Kinds[0]) }, "kinds[5]"},
		{"unknown role", func(s *Schema) { s.Kinds[0].Role = "x" }, "kinds[0]"},
		{"unknown parent", func(s *Schema) { s.Kinds[1].Parents = []string{"nope"} }, "kinds[1].parents"},
		{"no work default", func(s *Schema) { s.Kinds[3].Default = false }, "kinds"},
		{"built-in prop name", func(s *Schema) { s.Props = append(s.Props, Prop{Name: "status", Type: TypeText}) }, "props.status"},
		{"duplicate prop", func(s *Schema) { s.Props = append(s.Props, s.Props[0]) }, "props.priority"},
		{"unknown type", func(s *Schema) { s.Props[0].Type = "x" }, "props.priority"},
		{"enum without values", func(s *Schema) { s.Props[0].Values = nil }, "props.priority"},
		{"inverse and symmetric", func(s *Schema) { s.Props[7].Inverse = "x" }, "props.relates"},
		{"inverse on text", func(s *Schema) { s.Props = append(s.Props, Prop{Name: "t", Type: TypeText, Inverse: "x"}) }, "props.t"},
		{"transition from unknown", func(s *Schema) { s.Workflow.Transitions = Lists{{"nope", []string{"todo"}}} }, "workflow.transitions"},
		{"transition to unknown", func(s *Schema) { s.Workflow.Transitions = Lists{{"todo", []string{"nope"}}} }, "workflow.transitions"},
		{"required unknown field", func(s *Schema) { s.Workflow.Required = Lists{{"done", []string{"nope"}}} }, "workflow.required"},
		{"bad where", func(s *Schema) { s.Scoped = []Scoped{{Where: "project:nope"}} }, "scoped[0].where"},
		{"not a project where", func(s *Schema) { s.Scoped = []Scoped{{Where: "store:x"}} }, "scoped[0].where"},
		{"duplicate where", func(s *Schema) { s.Scoped = []Scoped{{Where: where}, {Where: where}} }, "scoped[1].where"},
		{"hidden default status", func(s *Schema) { s.Scoped = []Scoped{{Where: where, Statuses: []string{"todo", "done", "canceled"}}} }, "scoped[0].statuses"},
		{"unknown scoped status", func(s *Schema) {
			s.Scoped = []Scoped{{Where: where, Statuses: []string{"todo", "in_progress", "done", "canceled", "nope"}}}
		}, "scoped[0].statuses"},
		{"scoped workflow", func(s *Schema) {
			s.Scoped = []Scoped{{Where: where, Workflow: &Workflow{Required: Lists{{"done", []string{"nope"}}}}}}
		}, "scoped[0].workflow.required"},
		{"scoped template kind", func(s *Schema) {
			s.Scoped = []Scoped{{Where: where, Templates: &Templates{Default: map[string]string{"nope": "x"}}}}
		}, "scoped[0].templates.default"},
	} {
		s := Default()
		tc.edit(s)
		ps := s.Check()
		found := false
		for _, p := range ps {
			if strings.HasPrefix(p.Path, tc.inPath) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no problem at %q in %v", tc.name, tc.inPath, ps)
		}
	}
}

func TestRequiredAcceptsBuiltinsAndProps(t *testing.T) {
	s := Default()
	s.Workflow.Required = Lists{{"in_progress", []string{"assignees"}}, {"done", []string{"points", "sprint"}}}
	if ps := s.Check(); len(ps) != 0 {
		t.Errorf("problems: %v", ps)
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/schema/ -run 'Check|Required'`
Expected: FAIL（`s.Check undefined`）

- [ ] **Step 3: 実装する**

`internal/schema/check.go`:
```go
package schema

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/kukv/hedgehug/internal/model"
)

// Problem is a defect in a schema.
type Problem struct {
	Path string // where, like "statuses[2]" or "scoped[0].where"
	Msg  string
}

func (p Problem) String() string { return p.Path + ": " + p.Msg }

// requirableFields are the built-in task fields that workflow.required may
// name, besides properties.
var requirableFields = []string{
	"key_prefix", "parent", "assignees", "watchers", "labels", "repos",
	"blocked_by", "sprint", "releases", "links",
}

// builtinFilled reports whether the built-in field name of t has a value.
// ok is false when name is not a requirable built-in field.
func builtinFilled(t *model.Task, name string) (filled, ok bool) {
	switch name {
	case "key_prefix":
		return t.KeyPrefix != "", true
	case "parent":
		return t.Parent != nil, true
	case "assignees":
		return len(t.Assignees) > 0, true
	case "watchers":
		return len(t.Watchers) > 0, true
	case "labels":
		return len(t.Labels) > 0, true
	case "repos":
		return len(t.Repos) > 0, true
	case "blocked_by":
		return len(t.BlockedBy) > 0, true
	case "sprint":
		return t.Sprint != nil, true
	case "releases":
		return len(t.Releases) > 0, true
	case "links":
		return len(t.Links) > 0, true
	}
	return false, false
}

// parseWhere reads the target of a scoped override: project:<ULID>.
func parseWhere(w string) (model.ID, error) {
	s, ok := strings.CutPrefix(w, "project:")
	if !ok {
		return model.ID{}, errors.New(`where must be "project:<ULID>"`)
	}
	return model.ParseID(s)
}

// Check lists the defects of s. A schema with defects may still be read, so
// that hh doctor and hh schema check can show them.
func (s *Schema) Check() []Problem {
	var ps []Problem
	add := func(path, format string, args ...any) {
		ps = append(ps, Problem{path, fmt.Sprintf(format, args...)})
	}

	statuses := map[string]bool{}
	defaults := map[Category]int{}
	for i, st := range s.Statuses {
		p := fmt.Sprintf("statuses[%d]", i)
		switch {
		case st.Name == "":
			add(p, "name is empty")
		case statuses[st.Name]:
			add(p, "status %q is defined twice", st.Name)
		}
		statuses[st.Name] = true
		if !slices.Contains(Categories, st.Category) {
			add(p, "unknown category %q", st.Category)
		}
		if st.Default {
			defaults[st.Category]++
		}
	}
	for _, c := range Categories {
		if defaults[c] != 1 {
			add("statuses", "category %s needs exactly one default status, has %d", c, defaults[c])
		}
	}

	kinds := map[string]bool{}
	workDefaults := 0
	for i, k := range s.Kinds {
		p := fmt.Sprintf("kinds[%d]", i)
		switch {
		case k.Name == "":
			add(p, "name is empty")
		case kinds[k.Name]:
			add(p, "kind %q is defined twice", k.Name)
		}
		kinds[k.Name] = true
		if k.Role != RoleContainer && k.Role != RoleWork {
			add(p, "unknown role %q", k.Role)
		}
		if k.Default && k.Role == RoleWork {
			workDefaults++
		}
	}
	for i, k := range s.Kinds {
		for _, parent := range k.Parents {
			if parent != Root && !kinds[parent] {
				add(fmt.Sprintf("kinds[%d].parents", i), "unknown kind %q", parent)
			}
		}
	}
	if workDefaults != 1 {
		add("kinds", "the work role needs exactly one default kind, has %d", workDefaults)
	}

	props := map[string]bool{}
	for _, pr := range s.Props {
		p := "props." + pr.Name
		if slices.Contains(model.TaskKeys(), pr.Name) {
			add(p, "%q is a built-in field", pr.Name)
		}
		if props[pr.Name] {
			add(p, "property %q is defined twice", pr.Name)
		}
		props[pr.Name] = true
		if !slices.Contains(PropTypes, pr.Type) {
			add(p, "unknown type %q", pr.Type)
		}
		if (pr.Type == TypeEnum || pr.Type == TypeMultiEnum) && len(pr.Values) == 0 {
			add(p, "values is empty")
		}
		if pr.Inverse != "" && pr.Symmetric {
			add(p, "inverse and symmetric cannot be used together")
		}
		if (pr.Inverse != "" || pr.Symmetric) && pr.Type != TypeTaskRef {
			add(p, "inverse and symmetric are only for task_ref")
		}
	}

	checkWorkflow := func(path string, w Workflow) {
		for _, tr := range w.Transitions {
			if !statuses[tr.Key] {
				add(path+".transitions", "unknown status %q", tr.Key)
			}
			for _, to := range tr.Values {
				if !statuses[to] {
					add(path+".transitions", "unknown status %q", to)
				}
			}
		}
		for _, rq := range w.Required {
			if !statuses[rq.Key] {
				add(path+".required", "unknown status %q", rq.Key)
			}
			for _, f := range rq.Values {
				if !props[f] && !slices.Contains(requirableFields, f) {
					add(path+".required", "%q is neither a property nor a field that can be required", f)
				}
			}
		}
	}
	checkWorkflow("workflow", s.Workflow)

	wheres := map[model.ID]bool{}
	for i, sc := range s.Scoped {
		p := fmt.Sprintf("scoped[%d]", i)
		id, err := parseWhere(sc.Where)
		switch {
		case err != nil:
			add(p+".where", "%v", err)
		case wheres[id]:
			add(p+".where", "%s has two overrides", sc.Where)
		}
		wheres[id] = true
		for _, name := range sc.Statuses {
			if !statuses[name] {
				add(p+".statuses", "unknown status %q", name)
			}
		}
		if sc.Statuses != nil {
			for _, c := range Categories {
				if d, ok := s.DefaultStatus(c); ok && !slices.Contains(sc.Statuses, d.Name) {
					add(p+".statuses", "%q is the default of %s and cannot be hidden", d.Name, c)
				}
			}
		}
		if sc.Workflow != nil {
			checkWorkflow(p+".workflow", *sc.Workflow)
		}
		if sc.Templates != nil {
			keys := make([]string, 0, len(sc.Templates.Default))
			for k := range sc.Templates.Default {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if !kinds[k] {
					add(p+".templates.default", "unknown kind %q", k)
				}
			}
		}
	}
	return ps
}
```

注: テンプレート名（`kinds[].template`、`templates.only`）が `templates/` に実在するかは、ディスクを読まないと分からないので 1a では検査しない。1d の `hh schema check` で足す。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/schema/`
Expected: PASS

- [ ] **Step 5: lint を通してコミットする**

Run: `make lint`。`gocritic` が `Check` の長さや `hugeParam` を指摘したら、指摘の内容に従って直す（関数の分割は、検査の単位ごと: statuses / kinds / props / workflow / scoped）。

```bash
git add internal/schema/check.go internal/schema/check_test.go
git commit -m "feat: スキーマ自体の不整合を検査する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 14: プロジェクトの実効スキーマ

**Files:**
- Create: `internal/schema/effective.go`
- Test: `internal/schema/effective_test.go`

**Interfaces:**
- Consumes: `parseWhere`（Task 13）、Task 12 の型
- Produces: `type Effective`、`(*Schema) For(ancestors []model.ID) *Effective`、`(*Effective) Schema() *Schema`、`(*Effective) Statuses() []Status`、`(*Effective) CanTransition(from, to string) bool`、`(*Effective) Required(status string) []string`、`(*Effective) Templates() (only []string, defaults map[string]string)`

- [ ] **Step 1: 失敗するテストを書く**

`internal/schema/effective_test.go`:
```go
package schema

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

func id(t *testing.T, s string) model.ID {
	t.Helper()
	v, err := model.ParseID(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func fullSchema(t *testing.T) *Schema {
	t.Helper()
	s, err := Decode(readFull(t))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestForPicksNearestScopedAncestor(t *testing.T) {
	s := fullSchema(t)
	release := id(t, "01JBZ8K3QW7M2X9V4T6R5N8P0B") // has an override
	epic := id(t, "01JBZ8K3QW7M2X9V4T6R5N8P0F")    // has none
	ops := id(t, "01JBZ8K3QW7M2X9V4T6R5N8P0C")     // has an override

	e := s.For([]model.ID{epic, release})
	if e.Required("done") == nil || !slices.Contains(e.Required("done"), "estimate") {
		t.Errorf("an override two levels up did not apply: %v", e.Required("done"))
	}
	if e := s.For([]model.ID{ops, release}); len(e.Statuses()) != 5 {
		t.Errorf("the nearest override must win, statuses = %v", e.Statuses())
	}
	if e := s.For(nil); len(e.Statuses()) != 7 {
		t.Errorf("no ancestors: statuses = %v", e.Statuses())
	}
}

func TestCanTransition(t *testing.T) {
	s := fullSchema(t)
	store := s.For(nil)
	release := s.For([]model.ID{id(t, "01JBZ8K3QW7M2X9V4T6R5N8P0B")})
	for _, tc := range []struct {
		e        *Effective
		from, to string
		want     bool
	}{
		{store, "backlog", "todo", true},
		{store, "backlog", "done", false},  // store limits backlog
		{store, "todo", "done", true},      // todo is not limited
		{store, "done", "done", true},      // same status is always allowed
		{release, "waiting", "done", false}, // project limits waiting
		{release, "waiting", "in_progress", true},
		{release, "backlog", "done", false}, // store limits still apply
	} {
		if got := tc.e.CanTransition(tc.from, tc.to); got != tc.want {
			t.Errorf("CanTransition(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestRequiredIsUnion(t *testing.T) {
	s := fullSchema(t)
	e := s.For([]model.ID{id(t, "01JBZ8K3QW7M2X9V4T6R5N8P0B")})
	got := e.Required("done")
	slices.Sort(got)
	if !slices.Equal(got, []string{"estimate", "points"}) {
		t.Errorf("Required(done) = %v, want [estimate points]", got)
	}
	if got := e.Required("in_progress"); !slices.Equal(got, []string{"assignees"}) {
		t.Errorf("Required(in_progress) = %v", got)
	}
}

func TestStatusesKeepStoreOrder(t *testing.T) {
	s := fullSchema(t)
	e := s.For([]model.ID{id(t, "01JBZ8K3QW7M2X9V4T6R5N8P0C")})
	var names []string
	for _, st := range e.Statuses() {
		names = append(names, st.Name)
	}
	if !slices.Equal(names, []string{"todo", "on_hold", "in_progress", "done", "canceled"}) {
		t.Errorf("Statuses = %v", names)
	}
}

func TestTemplates(t *testing.T) {
	s := fullSchema(t)
	only, def := s.For([]model.ID{id(t, "01JBZ8K3QW7M2X9V4T6R5N8P0B")}).Templates()
	if !slices.Equal(only, []string{"release-checklist", "api-change"}) || def["task"] != "release-checklist" {
		t.Errorf("Templates = %v, %v", only, def)
	}
	if only, def := s.For(nil).Templates(); only != nil || def["bug"] != "bug-report" {
		t.Errorf("store Templates = %v, %v", only, def)
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/schema/ -run 'For|CanTransition|Required|Statuses|Templates'`
Expected: FAIL（`s.For undefined`）

- [ ] **Step 3: 実装する**

`internal/schema/effective.go`:
```go
package schema

import (
	"slices"

	"github.com/kukv/hedgehug/internal/model"
)

// Effective is the schema as it applies to the tasks of one project: the
// store's definitions plus that project's override, if any.
type Effective struct {
	s     *Schema
	scope *Scoped // nil when no override applies
}

// For returns the schema for a task whose ancestors are ancestors, nearest
// first (include the task itself first when it is a container). The override
// of the nearest ancestor that has one applies; overrides of farther
// ancestors are not combined with it.
func (s *Schema) For(ancestors []model.ID) *Effective {
	for _, a := range ancestors {
		for i := range s.Scoped {
			if w, err := parseWhere(s.Scoped[i].Where); err == nil && w == a {
				return &Effective{s: s, scope: &s.Scoped[i]}
			}
		}
	}
	return &Effective{s: s}
}

func (e *Effective) Schema() *Schema { return e.s }

// Statuses returns the statuses to show and offer, in store order.
func (e *Effective) Statuses() []Status {
	if e.scope == nil || e.scope.Statuses == nil {
		return slices.Clone(e.s.Statuses)
	}
	var out []Status
	for _, st := range e.s.Statuses {
		if slices.Contains(e.scope.Statuses, st.Name) {
			out = append(out, st)
		}
	}
	return out
}

// CanTransition reports whether a task may move from one status to another.
// Both the store and the project must allow it. Staying in the same status
// is always allowed.
func (e *Effective) CanTransition(from, to string) bool {
	if from == to {
		return true
	}
	if !allows(e.s.Workflow, from, to) {
		return false
	}
	return e.scope == nil || e.scope.Workflow == nil || allows(*e.scope.Workflow, from, to)
}

// allows applies one workflow: only the statuses it lists are limited.
func allows(w Workflow, from, to string) bool {
	targets, ok := w.Transitions.Get(from)
	return !ok || slices.Contains(targets, to)
}

// Required returns the fields a task needs to enter status: the union of the
// store's and the project's lists.
func (e *Effective) Required(status string) []string {
	out, _ := e.s.Workflow.Required.Get(status)
	out = slices.Clone(out)
	if e.scope != nil && e.scope.Workflow != nil {
		more, _ := e.scope.Workflow.Required.Get(status)
		for _, f := range more {
			if !slices.Contains(out, f) {
				out = append(out, f)
			}
		}
	}
	return out
}

// Templates returns the templates the project may use (nil: all) and the
// default template of each kind.
func (e *Effective) Templates() (only []string, defaults map[string]string) {
	defaults = map[string]string{}
	for _, k := range e.s.Kinds {
		if k.Template != "" {
			defaults[k.Name] = k.Template
		}
	}
	if e.scope == nil || e.scope.Templates == nil {
		return nil, defaults
	}
	for k, v := range e.scope.Templates.Default {
		defaults[k] = v
	}
	return slices.Clone(e.scope.Templates.Only), defaults
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/schema/`
Expected: PASS

- [ ] **Step 5: lint を通してコミットする**

Run: `make lint`

```bash
git add internal/schema/effective.go internal/schema/effective_test.go
git commit -m "feat: プロジェクトの上書きを適用した実効スキーマを追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 15: タスクの検証

**Files:**
- Create: `internal/schema/validate.go`
- Test: `internal/schema/validate_test.go`

**Interfaces:**
- Consumes: `Effective`（Task 14）、`builtinFilled`（Task 13）、`model.Task`、`model.ParseID`、`model.ParseDate`、`model.ParseDuration`
- Produces: `type Violation struct{ Code, Field, Msg string }`、コード定数 `CodeUnknownKind/CodeUnknownStatus/CodeMissingTitle/CodeBadValue/CodePlacement/CodeTransition/CodeRequired`、`ValidateTask(*model.Task, *Effective) (violations []Violation, undefined []string)`、`ValidatePlacement(kind, parentKind string, *Effective) []Violation`、`ValidateTransition(from, to string, *model.Task, *Effective) []Violation`

違反を拒否するか警告にとどめるかは呼び出し側が決める。`task_ref` は ULID の形か、`user` は `<host>/<login>` の形かだけを見る（参照先の存在は 1c）。

- [ ] **Step 1: 失敗するテストを書く**

`internal/schema/validate_test.go`:
```go
package schema

import (
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"go.yaml.in/yaml/v4"
)

func taskWith(t *testing.T, props string) *model.Task {
	t.Helper()
	src := "---\nid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nkind: task\ntitle: x\nstatus: todo\n"
	if props != "" {
		src += "props:\n" + props
	}
	task, err := model.DecodeTask([]byte(src + "---\n"))
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func codes(vs []Violation) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Code + ":" + v.Field
	}
	return out
}

func TestValidateTaskOK(t *testing.T) {
	s := Default()
	s.Props = append(s.Props,
		Prop{Name: "area", Type: TypeMultiEnum, Values: []string{"backend", "frontend"}},
		Prop{Name: "owner", Type: TypeUser},
		Prop{Name: "spec", Type: TypeURL},
		Prop{Name: "flag", Type: TypeBool},
		Prop{Name: "at", Type: TypeDatetime},
		Prop{Name: "note", Type: TypeText},
		Prop{Name: "blocker", Type: TypeTaskRef},
	)
	task := taskWith(t, `  priority: high
  start: 2026-11-03
  estimate: 1h30m
  points: 2.5
  duplicates: [01JBZ8K3QW7M2X9V4T6R5N8P0B]
  area: [backend]
  owner: github.com/yourname
  spec: https://example.com/spec
  flag: true
  at: 2026-11-03T10:00:00+09:00
  note: anything
  blocker: 01JBZ8K3QW7M2X9V4T6R5N8P0C
  custom: 1
`)
	vs, undefined := ValidateTask(task, s.For(nil))
	if len(vs) != 0 {
		t.Errorf("violations: %v", codes(vs))
	}
	if len(undefined) != 1 || undefined[0] != "custom" {
		t.Errorf("undefined = %v, want [custom]", undefined)
	}
}

func TestValidateTaskViolations(t *testing.T) {
	s := Default()
	s.Props = append(s.Props,
		Prop{Name: "area", Type: TypeMultiEnum, Values: []string{"backend"}},
		Prop{Name: "owner", Type: TypeUser},
		Prop{Name: "spec", Type: TypeURL},
		Prop{Name: "flag", Type: TypeBool},
		Prop{Name: "blocker", Type: TypeTaskRef},
	)
	for _, tc := range []struct{ props, want string }{
		{"  priority: none\n", "bad_value:priority"},
		{"  start: 2026-13-01\n", "bad_value:start"},
		{"  estimate: 1d\n", "bad_value:estimate"},
		{"  points: many\n", "bad_value:points"},
		{"  duplicates: 01JBZ8K3QW7M2X9V4T6R5N8P0B\n", "bad_value:duplicates"}, // multi needs a list
		{"  duplicates: [nope]\n", "bad_value:duplicates"},
		{"  area: [ops]\n", "bad_value:area"},
		{"  owner: yourname\n", "bad_value:owner"},
		{"  spec: not a url\n", "bad_value:spec"},
		{"  flag: maybe\n", "bad_value:flag"},
		{"  blocker: [01JBZ8K3QW7M2X9V4T6R5N8P0C]\n", "bad_value:blocker"}, // single needs a scalar
	} {
		vs, _ := ValidateTask(taskWith(t, tc.props), s.For(nil))
		if got := codes(vs); len(got) != 1 || got[0] != tc.want {
			t.Errorf("%q: violations = %v, want [%s]", tc.props, got, tc.want)
		}
	}

	task := taskWith(t, "")
	task.Kind, task.Status, task.Title = "nope", "nope", ""
	vs, _ := ValidateTask(task, s.For(nil))
	want := map[string]bool{"unknown_kind:kind": true, "unknown_status:status": true, "missing_title:title": true}
	if got := codes(vs); len(got) != 3 || !want[got[0]] || !want[got[1]] || !want[got[2]] {
		t.Errorf("violations = %v", got)
	}
}

func TestValidatePlacement(t *testing.T) {
	e := Default().For(nil)
	for _, tc := range []struct {
		kind, parent string
		ok           bool
	}{
		{"task", "", true},       // root
		{"task", "task", true},
		{"epic", "story", false},
		{"bug", "task", false},
		{"project", "epic", true}, // parents omitted: anywhere
		{"nope", "", false},
	} {
		vs := ValidatePlacement(tc.kind, tc.parent, e)
		if (len(vs) == 0) != tc.ok {
			t.Errorf("ValidatePlacement(%q, %q) = %v, want ok=%v", tc.kind, tc.parent, codes(vs), tc.ok)
		}
	}
}

func TestValidateTransition(t *testing.T) {
	s := Default()
	s.Workflow = Workflow{
		Transitions: Lists{{"backlog", []string{"todo"}}},
		Required:    Lists{{"in_progress", []string{"assignees"}}, {"done", []string{"points"}}},
	}
	e := s.For(nil)
	task := taskWith(t, "")

	if vs := ValidateTransition("backlog", "done", task, e); len(vs) == 0 || vs[0].Code != CodeTransition {
		t.Errorf("backlog→done = %v", codes(vs))
	}
	if vs := ValidateTransition("todo", "in_progress", task, e); len(vs) != 1 || vs[0].Field != "assignees" {
		t.Errorf("todo→in_progress without assignees = %v", codes(vs))
	}
	task.Assignees = []string{"github.com/me"}
	if vs := ValidateTransition("todo", "in_progress", task, e); len(vs) != 0 {
		t.Errorf("todo→in_progress with assignees = %v", codes(vs))
	}
	if vs := ValidateTransition("todo", "done", task, e); len(vs) != 1 || vs[0].Field != "points" {
		t.Errorf("todo→done without points = %v", codes(vs))
	}
	task.Props.Set("points", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "3"})
	if vs := ValidateTransition("todo", "done", task, e); len(vs) != 0 {
		t.Errorf("todo→done with points = %v", codes(vs))
	}
	task.Assignees = nil
	if vs := ValidateTransition("in_progress", "in_progress", task, e); len(vs) != 0 {
		t.Errorf("staying must be allowed: %v", codes(vs))
	}
	if vs := ValidateTransition("todo", "nope", task, e); len(vs) != 1 || vs[0].Code != CodeUnknownStatus {
		t.Errorf("to an unknown status = %v", codes(vs))
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/schema/ -run Validate`
Expected: FAIL（`undefined: ValidateTask`）

- [ ] **Step 3: 実装する**

`internal/schema/validate.go`:
```go
package schema

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/kukv/hedgehug/internal/model"
	"go.yaml.in/yaml/v4"
)

// Violation codes.
const (
	CodeUnknownKind   = "unknown_kind"
	CodeUnknownStatus = "unknown_status"
	CodeMissingTitle  = "missing_title"
	CodeBadValue      = "bad_value"
	CodePlacement     = "placement"
	CodeTransition    = "transition"
	CodeRequired      = "required"
)

// Violation is a way a task breaks its schema. Writes refuse violations;
// after a sync they are shown as warnings.
type Violation struct {
	Code  string
	Field string
	Msg   string
}

func (v Violation) String() string { return v.Field + ": " + v.Msg }

// ValidateTask checks the kind, status, title and property values of t.
// Properties the schema does not define are not violations; their names are
// returned in undefined, in file order.
func ValidateTask(t *model.Task, e *Effective) (violations []Violation, undefined []string) {
	s := e.Schema()
	if _, ok := s.Kind(t.Kind); !ok {
		violations = append(violations, Violation{CodeUnknownKind, "kind", fmt.Sprintf("unknown kind %q", t.Kind)})
	}
	if _, ok := s.Status(t.Status); !ok {
		violations = append(violations, Violation{CodeUnknownStatus, "status", fmt.Sprintf("unknown status %q", t.Status)})
	}
	if t.Title == "" {
		violations = append(violations, Violation{CodeMissingTitle, "title", "title is empty"})
	}
	for _, f := range t.Props {
		p, ok := s.Prop(f.Key)
		if !ok {
			undefined = append(undefined, f.Key)
			continue
		}
		if f.Value.ShortTag() == "!!null" {
			continue
		}
		if err := checkValue(p, f.Value); err != nil {
			violations = append(violations, Violation{CodeBadValue, f.Key, err.Error()})
		}
	}
	return violations, undefined
}

// ValidatePlacement checks that a task of kind may sit under a parent of
// parentKind. An empty parentKind means the root.
func ValidatePlacement(kind, parentKind string, e *Effective) []Violation {
	k, ok := e.Schema().Kind(kind)
	if !ok {
		return []Violation{{CodeUnknownKind, "kind", fmt.Sprintf("unknown kind %q", kind)}}
	}
	parent := parentKind
	if parent == "" {
		parent = Root
	}
	if k.Parents == nil || slices.Contains(k.Parents, parent) {
		return nil
	}
	return []Violation{{CodePlacement, "parent", fmt.Sprintf("a %s cannot be placed under %s", kind, parent)}}
}

// ValidateTransition checks a move of t from one status to another: that
// the workflow allows it, and that t has the fields the new status needs.
// Use an empty from for a new task.
func ValidateTransition(from, to string, t *model.Task, e *Effective) []Violation {
	if _, ok := e.Schema().Status(to); !ok {
		return []Violation{{CodeUnknownStatus, "status", fmt.Sprintf("unknown status %q", to)}}
	}
	if from == to {
		return nil
	}
	var vs []Violation
	if from != "" && !e.CanTransition(from, to) {
		vs = append(vs, Violation{CodeTransition, "status", fmt.Sprintf("cannot move from %s to %s", from, to)})
	}
	for _, f := range e.Required(to) {
		if !filled(t, f) {
			vs = append(vs, Violation{CodeRequired, f, fmt.Sprintf("%s is required to enter %s", f, to)})
		}
	}
	return vs
}

func filled(t *model.Task, name string) bool {
	if ok, builtin := builtinFilled(t, name); builtin {
		return ok
	}
	n := t.Props.Get(name)
	switch {
	case n == nil || n.ShortTag() == "!!null":
		return false
	case n.Kind == yaml.ScalarNode:
		return n.Value != ""
	}
	return len(n.Content) > 0
}

var userRE = regexp.MustCompile(`^[A-Za-z0-9.-]+/[A-Za-z0-9-]+$`)

// checkValue checks one property value against its definition.
func checkValue(p Prop, n *yaml.Node) error {
	if p.Type == TypeMultiEnum || p.Multi {
		if n.Kind != yaml.SequenceNode {
			return errors.New("want a list")
		}
		for _, it := range n.Content {
			if err := checkScalar(p, it); err != nil {
				return err
			}
		}
		return nil
	}
	return checkScalar(p, n)
}

func checkScalar(p Prop, n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return errors.New("want a single value")
	}
	v := n.Value
	var err error
	switch p.Type {
	case TypeText:
	case TypeNumber:
		_, err = strconv.ParseFloat(v, 64)
	case TypeBool:
		if n.ShortTag() != "!!bool" {
			err = errors.New("want true or false")
		}
	case TypeDate:
		_, err = model.ParseDate(v)
	case TypeDatetime:
		_, err = time.Parse(time.RFC3339, v)
	case TypeDuration:
		_, err = model.ParseDuration(v)
	case TypeEnum, TypeMultiEnum:
		if !slices.Contains(p.Values, v) {
			err = fmt.Errorf("%q is not one of %v", v, p.Values)
		}
	case TypeUser:
		if !userRE.MatchString(v) {
			err = fmt.Errorf("%q is not <host>/<login>", v)
		}
	case TypeTaskRef:
		_, err = model.ParseID(v)
	case TypeURL:
		u, perr := url.Parse(v)
		if perr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			err = fmt.Errorf("%q is not an http(s) URL", v)
		}
	default:
		err = fmt.Errorf("unknown type %q", p.Type)
	}
	return err
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/schema/`
Expected: PASS

- [ ] **Step 5: lint を通してコミットする**

Run: `make lint`

```bash
git add internal/schema/validate.go internal/schema/validate_test.go
git commit -m "feat: タスクの値・配置・ステータス遷移をスキーマで検証する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 16: fuzz テストと全体の検証

**Files:**
- Create: `internal/model/fuzz_test.go`
- Test: 同上

**Interfaces:**
- Consumes: `splitFrontmatter`、`joinFrontmatter`、`Between`、`DecodeTask`

- [ ] **Step 1: fuzz テストを書く**

`internal/model/fuzz_test.go`:
```go
package model

import "testing"

func FuzzFrontmatter(f *testing.F) {
	for _, s := range []string{"---\na: 1\n---\nbody\n", "---\n---\n", "---\r\na: 1\r\n---", "x", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		front, body, err := splitFrontmatter([]byte(s))
		if err != nil {
			return
		}
		f2, b2, err := splitFrontmatter(joinFrontmatter(front, body))
		if err != nil || string(f2) != string(front) || b2 != body {
			t.Errorf("join/split changed %q: %q %q -> %q %q (%v)", s, front, body, f2, b2, err)
		}
	})
}

func FuzzBetween(f *testing.F) {
	for _, p := range [][2]string{{"", ""}, {"a0", "a1"}, {"Zz", "a0"}, {"a0V", ""}, {"", "Y00"}} {
		f.Add(p[0], p[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		if (a != "" && Rank(a).Validate() != nil) || (b != "" && Rank(b).Validate() != nil) || (a != "" && b != "" && a >= b) {
			return
		}
		r, err := Between(Rank(a), Rank(b))
		if err != nil {
			return // only at the ends of the key space
		}
		if r.Validate() != nil || (a != "" && string(r) <= a) || (b != "" && string(r) >= b) {
			t.Errorf("Between(%q, %q) = %q", a, b, r)
		}
	})
}

func FuzzDecodeTask(f *testing.F) {
	f.Add("---\nid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\ntitle: x\n---\nbody\n")
	f.Add("---\nid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nlabels: [a, b]\nprops: {p: 1}\n---\n")
	f.Fuzz(func(t *testing.T, s string) {
		task, err := DecodeTask([]byte(s))
		if err != nil {
			return
		}
		out, err := task.Encode()
		if err != nil {
			t.Fatalf("Encode after Decode(%q): %v", s, err)
		}
		task2, err := DecodeTask(out)
		if err != nil {
			t.Fatalf("Decode of our own output %q: %v", out, err)
		}
		out2, err := task2.Encode()
		if err != nil || string(out2) != string(out) {
			t.Errorf("Encode is not idempotent:\n%s\n---\n%s", out, out2)
		}
	})
}
```

- [ ] **Step 2: fuzz を短時間ずつ回す**

Run:
```bash
go test ./internal/model/ -run '^$' -fuzz FuzzFrontmatter -fuzztime 30s
go test ./internal/model/ -run '^$' -fuzz FuzzBetween -fuzztime 30s
go test ./internal/model/ -run '^$' -fuzz FuzzDecodeTask -fuzztime 60s
```
Expected: どれも `PASS`。失敗すると `testdata/fuzz/<名前>/` に再現ケースが保存される。原因を直したうえで、その再現ケースは回帰テストとして残してコミットに含める。`FuzzDecodeTask` が「エンコード結果を再デコードできない」で落ちた場合は、`Encoder.Put` か `styleLike` の不具合なので、そちらを直す（テストを緩めない）。ただし原因がアンカー・エイリアス（`&a` / `*a`）、明示的なタグ（`!!str` など）、マージキー（`<<`）といった、Hedgehug のファイルでは使わない YAML の機能だった場合は、1a の範囲外とする。その場合は `ParseDoc` でエイリアスとマージキーを含む文書をエラーにして読み込みを拒否し、再現ケースはエラーになることを確かめる回帰テストとして残す。

- [ ] **Step 3: CI と同じ検査を通す**

Run: `make check`
Expected: `tidy-check`、`lint`、`fmt-check`、`test` がすべて通る。`fmt-check` が落ちたら `make fmt` を実行して差分を確認する。

- [ ] **Step 4: git を使わない環境でも動くことを確かめる**

1a のコードは git も外部コマンドも使わない。`grep -rn 'os/exec' internal/` の結果が空であることを確かめる。

Run: `grep -rn 'os/exec' internal/ || echo none`
Expected: `none`

- [ ] **Step 5: コミットする**

```bash
git add internal/model/fuzz_test.go
[ -d internal/model/testdata/fuzz ] && git add internal/model/testdata/fuzz
git commit -m "test: frontmatter・rank・タスクファイルの fuzz テストを追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

## spec との対応

| spec の項目 | タスク |
| --- | --- |
| パッケージ構成、依存2つ、`model` が `schema` に依存しない | 1、12 |
| YAML の往復（案A）、`Extra`、`NodeEqual`、`id` だけ必須 | 6、7（転送ファイルのテスト） |
| タスク・コメント・作業時間・スプリント・リリース・テンプレート・`store.yaml` の形式 | 7〜11 |
| duration（h と m）、日付、RFC 3339 | 4、7 |
| ID、パス（32×32、大文字、逆変換） | 1、2 |
| rank（rocicorp 形式、`Between`、`NBetween`、テストベクタ） | 3 |
| スキーマの codec、各要素のフィールド、`Default`/`Dev`、`DefaultStatus`/`DefaultKind` | 12 |
| スキーマ自体の検査 | 13 |
| 実効スキーマ（祖先の列、最も近い上書き、遷移は積・必須は和） | 14 |
| タスクの検査（値の型、`task_ref`・`user` は形だけ、配置、遷移、同じステータスは常に許す） | 15 |
| `DecodeError` と行番号 | 1、6 |
| ゴールデン（正準形の往復）、冪等性、fuzz、`make check` | 7〜12、16 |

注: spec の「ゴールデンテスト（`-update` で期待ファイルを更新）」は、「正準形の `testdata` を読んで書くとバイト単位で同じになる」テストに置き換えた。入力と期待が同じファイルなので、期待ファイルを別に持つ必要も `-update` も要らない。並べ替えや CRLF のように入力と出力が違うケースは、テストの中に期待値の文字列を直接書いている。
| v4 が RC であることの確認 | 事前のスパイク（本書冒頭） |
