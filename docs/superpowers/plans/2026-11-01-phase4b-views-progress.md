# フェーズ 4b: 保存済みビューと進捗 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `views/<name>.yaml` の保存済みビューを、形式（`internal/view`）・マージ・doctor・ops・CLI（`hh view save/list/show/rm/rename`）までそろえ、チェックリストとプロジェクトの進捗を ops と CLI（`hh ls --tree`・`hh show`）に入れる。

**Architecture:** ビューのファイル形式は新しい `internal/view` に置き、既存の `model.Doc`／`Encoder` で未知のキーとコメントを残す。書き込みは `ops.Session.UpdateCommit` と `tx.PutFile`／`tx.RemoveFile` で1操作1コミット。`views/*.yaml` は schema.yaml と同じキー単位のマージ（`kindYAML`）に入れ、同期の `Side.When` も同じ判定で付ける。チェックリストの数はインデックスの `Entry` に持たせ（本文はインデックスにないため）、プロジェクトの進捗は ops がインデックスの子孫から数える。

**Tech Stack:** Go 1.27、cobra、`go.yaml.in/yaml/v4`、既存の internal パッケージ（model・schema・store・index・merge・sync・doctor・ops）

**Spec:** `docs/superpowers/specs/2026-11-01-phase4-gui-design.md`（「4b 保存済みビュー」、「利用者の決定」の 4b の細部）。親設計（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）が正で、spec はそれに合わせてある。

## Global Constraints

- spec・親設計と違う挙動にしない。迷ったら実装を止めてコントローラーに報告する（自分で決めない）
- 既存の CLI の出力・終了コード・エラー文言を、この plan が変えると書いた箇所（Task 6 の進捗の表示）以外で変えない
- `go test ./cmd/hedgehug/... -update` を実行したら、必ず `git diff --stat -- cmd/hedgehug/testdata` を確かめ、進捗の行（`☑ n/m`・`n/m`）と `checklist`・`progress` のキー以外の差分がないことを確認する。それ以外の差分は戻す
- 終了コード: 0 成功、2 使い方の誤り（`ops.KindUsage`）、1 それ以外
- `internal/ops` と `internal/view` は何も印字しない
- 用語: 人が読める識別子は ID、内部の識別子は ULID
- コードのコメント・識別子・CLI の文言は英語（既存に合わせる）。コミットメッセージは日本語で、末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` を付ける
- 各タスクの最後に `go test -race ./...`、`golangci-lint run ./...`、`golangci-lint fmt --diff ./...` を通す。テストは前面で実行し、Bash の timeout は 600000 にする
- コミットの署名は既存の設定（1Password）に任せる。署名に失敗したら止まって知らせる

## 決定済みの細部（spec に反映済み。実装はこのとおり）

| 項目 | 決定 |
|---|---|
| 名前 | 1〜64 文字（rune 数）。`/ \ : * ? " < > |`、制御文字、先頭・末尾の空白（`unicode.IsSpace`）とドット、Windows の予約名（`CON` `PRN` `AUX` `NUL` `COM1`〜`COM9` `LPT1`〜`LPT9`。最初の `.` より前を大文字にして比べる）を拒否。`strings.EqualFold` で同じになる別の名前は「同じ名前」として拒否。名前の変更で大文字小文字だけを変える（`Foo` → `foo`）のも拒否 |
| 組み込みの列 | `id` `title` `kind` `status` `assignee` `label` `parent` `created` `updated`（9個）。ほかはスキーマのプロパティ名 |
| tree と sort | `layout: tree` で、クエリに `sort:`・`group:` の項があれば保存時に使い方の誤り（`view.tree_order`） |
| 進捗の数え方 | 子孫のうち、種類の role が work のタスク。削除済みは数えない。done 数 / canceled を除いた数。プロジェクト（role が container の種類）だけに出す |
| `--json` の進捗 | `hh ls --tree --json` と `hh show --json` だけ。ほかのコマンドの `--json` には足さない |

## この plan で埋める spec にない細部（実装前にコントローラーが利用者に一覧で見せる）

1. 新規作成で `--layout` を省いたら `tree`、`--query` を省いたら空のクエリ（全タスク）
2. ビューのファイルが読めない（YAML が壊れている）ときの `view save` は `view.broken`（Invalid）で拒否し、`hh view rm` か手で直すよう案内する。`rm`・`rename` は壊れたファイルにもできる（バイト列をそのまま移す）
3. `hh view list` は読めないファイルを飛ばし、`Warn` で `views/<file>: <err>; run hedgehug doctor` を出す。名前の規則に合わないファイルも同じ
4. 上書きの保存では、値（layout・group_by・kanban の columns・swimlane）を保存後のビュー全体で検査する。列のスキーマとの照合は `--columns` を指定したときだけ（スキーマから消えた列は保存済みのまま許す。spec「表示で飛ばし、doctor が警告する」）
5. チェックリスト: 行頭の空白のあと `-` `*` `+` `1.` `1)` のいずれか、空白、`[ ]` `[x]` `[X]`、そのあと空白か行末、の行を数える。コードブロックは ```` ``` ```` と `~~~` のフェンスだけを除く（字下げのコードブロックは入れ子のリストと区別できないため数える）
6. `hh show` の見出しの下は1行で `  ☑ 2/5  3/14`（チェックリスト、進捗の順。片方だけならその片方）。`hh ls --tree` は行末に2つの空白に続けて同じ形
7. doctor: 形式の誤りは既存の `file` の検査、スキーマにない列は新しい `view` の検査（`checkOrder` の `file` の次）。大文字小文字だけが違う2つのビューのファイルは `file` の検査で両方を報告する
8. エラーの Code: `view.bad_name`（Usage）、`view.not_found`（NotFound）、`view.name_taken`（Conflict）、`view.bad_value`（Usage）、`view.bad_column`（Invalid）、`view.tree_order`（Usage）、`view.broken`（Invalid）
9. inbox: `views/*.yaml` の上書きは schema.yaml と同じく inbox に出るが revert できない（タスクの値だけが revert の対象。既存の挙動のまま）
10. 進捗は container の種類なら子孫に work のタスクがなくても `0/0` を出す（spec の「項目が1つもなければ出さない」はチェックリストだけ）。既存のゴールデンで project・epic の行に `0/0` が付く。削除済みの子の下にある削除されていない孫は数える（削除済みのタスク自身だけを数えない）

## ファイル構成

```
internal/model/path.go        ViewsDir、ViewPath、IsViewPath を足す
internal/view/
  view.go                     View、Table、Kanban、Decode、Encode、既定値、Problems、MissingColumns
  name.go                     ValidName、SameName、Builtins
  list.go                     Entry、List、Find
  *_test.go
internal/merge/merge.go       kindOf に views/*.yaml、ByKey を足す
internal/sync/history.go      isYAML を merge.ByKey に置き換える
internal/doctor/doctor.go     fileRoots に views、checkFile に views、checkViews
internal/doctor/finding.go    checkOrder に view
internal/ops/views.go         SavedView、ViewChange、ViewResult、ViewSave/List/Get/Remove/Rename
internal/ops/progress.go      Count、ChecklistOf、ProgressOf
internal/ops/view.go          TaskView に Checklist・Progress
internal/ops/read.go          TreeViewOf・ShowViewOf で進捗を入れる
internal/index/checklist.go   Checklist（本文から数える）
internal/index/entry.go       Entry に ChecklistDone・ChecklistTotal
internal/index/build.go       cacheFormat を 6 に
cmd/hedgehug/view.go          hh view
cmd/hedgehug/tree.go          行末の進捗
cmd/hedgehug/show.go          見出しの下の進捗
cmd/hedgehug/app.go           viewCmd の登録
```

---

### Task 1: ビューの形式（internal/view）

**Files:**
- Modify: `internal/model/path.go`（末尾に追加）
- Create: `internal/view/name.go`, `internal/view/view.go`, `internal/view/list.go`
- Test: `internal/model/path_test.go`（追加）, `internal/view/name_test.go`, `internal/view/view_test.go`, `internal/view/list_test.go`

**Interfaces:**
- Produces:
  - `model.ViewsDir = "views"`、`model.ViewPath(name string) string`（`views/<name>.yaml`）、`model.IsViewPath(p string) (name string, ok bool)`（`views/` 直下の `.yaml`。名前の検査はしない）
  - `view.ValidName(name string) error`、`view.SameName(a, b string) bool`、`view.Builtins []string`、`view.IsBuiltin(col string) bool`
  - `view.Layouts`、`view.GroupBys`、`view.KanbanColumnsValues`、`view.Swimlanes`（`[]string`）
  - `view.View{Name, Layout, Query string; Table *Table; Kanban *Kanban; Extra model.Fields}`、`view.Table{Columns []string; GroupBy string}`、`view.Kanban{Columns, Swimlane string}`
  - `view.Decode(name string, data []byte) (*View, error)`、`(*View).Encode() ([]byte, error)`、`view.New(name string) *View`
  - `(*View).TableColumns() []string`、`(*View).GroupBy() string`、`(*View).KanbanColumns() string`、`(*View).Swimlane() string`（既定値を補う）
  - `(*View).Problems() []string`、`(*View).MissingColumns(sc *schema.Schema) []string`
  - `view.Entry{Name, Rel string; View *View; Err error}`、`view.List(storeDir string) ([]Entry, error)`、`view.Find(storeDir, name string) (Entry, bool, error)`、`view.ErrNotFound`

- [ ] **Step 1: model のパスの失敗するテストを書く**

`internal/model/path_test.go` に追加:

```go
func TestViewPath(t *testing.T) {
	if got := ViewPath("今週"); got != "views/今週.yaml" {
		t.Errorf("ViewPath = %q", got)
	}
	for _, c := range []struct {
		p    string
		name string
		ok   bool
	}{
		{"views/a.yaml", "a", true},
		{"views/今週 の作業.yaml", "今週 の作業", true},
		{"views/a.yml", "", false},
		{"views/sub/a.yaml", "", false},
		{"schema.yaml", "", false},
		{"views/.yaml", "", true},
	} {
		name, ok := IsViewPath(c.p)
		if name != c.name || ok != c.ok {
			t.Errorf("IsViewPath(%q) = %q, %v; want %q, %v", c.p, name, ok, c.name, c.ok)
		}
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/model/ -run TestViewPath`
Expected: FAIL（`undefined: ViewPath`）

- [ ] **Step 3: 実装する**

`internal/model/path.go` の末尾:

```go
// ViewsDir holds the saved views of a store, one YAML file each.
const ViewsDir = "views"

// ViewPath is the file of the saved view called name.
func ViewPath(name string) string { return ViewsDir + "/" + name + ".yaml" }

// IsViewPath reports whether p is a file directly under views/ with the
// .yaml extension, and gives the name it stands for. The name is not
// checked; see view.ValidName.
func IsViewPath(p string) (name string, ok bool) {
	if path.Dir(p) != ViewsDir || path.Ext(p) != ".yaml" {
		return "", false
	}
	return strings.TrimSuffix(path.Base(p), ".yaml"), true
}
```

（`path` と `strings` が import されていなければ足す。）

- [ ] **Step 4: 名前の失敗するテストを書く**

`internal/view/name_test.go`:

```go
package view

import (
	"strings"
	"testing"
)

func TestValidName(t *testing.T) {
	good := []string{"a", "今週のリリース作業", "Sprint 12 (web)", "a.b", strings.Repeat("あ", 64), "CONSOLE", "con-x"}
	for _, n := range good {
		if err := ValidName(n); err != nil {
			t.Errorf("ValidName(%q) = %v", n, err)
		}
	}
	bad := []string{"", strings.Repeat("a", 65), "a/b", `a\b`, "a:b", "a*b", "a?b", `a"b`, "a<b", "a>b", "a|b",
		"a\tb", "a\x00b", " a", "a ", "\u3000a", ".a", "a.", "CON", "con", "Nul.txt", "com1", "LPT9", "aux.tar.gz", "\xff"}
	for _, n := range bad {
		if err := ValidName(n); err == nil {
			t.Errorf("ValidName(%q) = nil; want an error", n)
		}
	}
}

func TestSameName(t *testing.T) {
	if !SameName("Release", "release") || SameName("a", "b") {
		t.Error("SameName folds case and nothing else")
	}
}

func TestIsBuiltin(t *testing.T) {
	for _, c := range []string{"id", "title", "kind", "status", "assignee", "label", "parent", "created", "updated"} {
		if !IsBuiltin(c) {
			t.Errorf("%s is not builtin", c)
		}
	}
	for _, c := range []string{"ulid", "due", "assignees", ""} {
		if IsBuiltin(c) {
			t.Errorf("%q is builtin", c)
		}
	}
}
```

- [ ] **Step 5: 失敗を確かめる**

Run: `go test ./internal/view/`
Expected: FAIL（パッケージの関数が未定義）

- [ ] **Step 6: name.go を書く**

```go
// Package view reads, writes and checks the saved views of a store:
// views/<name>.yaml, a layout and a query with the settings of each layout.
package view

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Builtins are the fields of a task that a table column can name besides
// the properties of the schema. They are named as the keys of the query.
var Builtins = []string{"id", "title", "kind", "status", "assignee", "label", "parent", "created", "updated"}

// IsBuiltin reports whether col is one of Builtins.
func IsBuiltin(col string) bool { return slices.Contains(Builtins, col) }

// reserved are the names Windows refuses as file names, whatever the
// extension.
var reserved = []string{"CON", "PRN", "AUX", "NUL",
	"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
	"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"}

// ValidName reports whether name can name a view on every system: 1 to 64
// characters, none of / \ : * ? " < > | or a control character, no space
// or dot at either end, and not a name Windows reserves.
func ValidName(name string) error {
	n := utf8.RuneCountInString(name)
	base, _, _ := strings.Cut(name, ".")
	switch {
	case !utf8.ValidString(name):
		return fmt.Errorf("view name %q is not valid UTF-8", name)
	case n == 0:
		return errors.New("view name is empty")
	case n > 64:
		return fmt.Errorf("view name %q is longer than 64 characters", name)
	case strings.ContainsAny(name, `/\:*?"<>|`):
		return fmt.Errorf(`view name %q contains one of / \ : * ? " < > |`, name)
	case strings.IndexFunc(name, unicode.IsControl) >= 0:
		return fmt.Errorf("view name %q contains a control character", name)
	case strings.TrimFunc(name, unicode.IsSpace) != name:
		return fmt.Errorf("view name %q starts or ends with a space", name)
	case strings.HasPrefix(name, ".") || strings.HasSuffix(name, "."):
		return fmt.Errorf("view name %q starts or ends with a dot", name)
	case slices.Contains(reserved, strings.ToUpper(base)):
		return fmt.Errorf("view name %q is reserved on Windows", name)
	}
	return nil
}

// SameName reports whether a and b name the same view: names that differ
// only in case are one name, since a file system may not tell them apart.
func SameName(a, b string) bool { return strings.EqualFold(a, b) }
```

- [ ] **Step 7: 形式の失敗するテストを書く**

`internal/view/view_test.go`:

```go
package view

import (
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/schema"
)

const full = `# shared by the team
layout: table            # tree | kanban | table
query: "project:SH-3 -status:canceled sort:due"
table:
  columns: [id, title, status, assignee, points, due]
  group_by: parent
  width: wide
kanban:
  columns: status
  swimlane: none
color: blue
`

func TestDecodeEncodeKeepsEverything(t *testing.T) {
	v, err := Decode("今週", []byte(full))
	if err != nil {
		t.Fatal(err)
	}
	if v.Name != "今週" || v.Layout != "table" || v.Query != "project:SH-3 -status:canceled sort:due" {
		t.Errorf("v = %+v", v)
	}
	if !slices.Equal(v.TableColumns(), []string{"id", "title", "status", "assignee", "points", "due"}) || v.GroupBy() != "parent" {
		t.Errorf("table = %+v", v.Table)
	}
	if v.KanbanColumns() != "status" || v.Swimlane() != "none" {
		t.Errorf("kanban = %+v", v.Kanban)
	}
	out, err := v.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != full {
		t.Errorf("round trip =\n%s\nwant\n%s", out, full)
	}
	v.Query = "assignee:@me"
	v.Table.GroupBy = "status"
	out, err = v.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"# shared by the team", "width: wide", "color: blue", "group_by: status", "query: \"assignee:@me\""} {
		if !strings.Contains(string(out), s) {
			t.Errorf("after a change the file lacks %q:\n%s", s, out)
		}
	}
}

func TestDefaults(t *testing.T) {
	v, err := Decode("x", []byte("layout: kanban\nquery: \"\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Table != nil || v.Kanban != nil {
		t.Errorf("sections = %+v %+v; want none", v.Table, v.Kanban)
	}
	if !slices.Equal(v.TableColumns(), []string{"id", "title", "status", "assignee", "due"}) || v.GroupBy() != "none" ||
		v.KanbanColumns() != "status" || v.Swimlane() != "none" {
		t.Errorf("defaults = %v %s %s %s", v.TableColumns(), v.GroupBy(), v.KanbanColumns(), v.Swimlane())
	}
	out, err := v.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "layout: kanban\nquery: \"\"\n" {
		t.Errorf("encode = %q; the absent sections stay absent", out)
	}
}

func TestNewEncode(t *testing.T) {
	v := New("x")
	v.Layout, v.Query = "table", "status:todo"
	v.Table = &Table{Columns: []string{"id", "title"}}
	out, err := v.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if want := "layout: table\nquery: status:todo\ntable:\n  columns: [id, title]\n"; string(out) != want {
		t.Errorf("encode = %q; want %q", out, want)
	}
}

func TestDecodeErrors(t *testing.T) {
	for _, data := range []string{"- a\n", "layout: [a]\n", "table: 3\n", "table:\n  columns: x: y\n", "a: &x 1\n"} {
		if _, err := Decode("x", []byte(data)); err == nil {
			t.Errorf("Decode(%q) = nil error", data)
		}
	}
}

func TestProblems(t *testing.T) {
	v, err := Decode("x", []byte("layout: gantt\ntable:\n  columns: [id, id, \"\"]\n  group_by: owner\nkanban:\n  columns: kind\n  swimlane: label\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`layout "gantt" is not one of tree, kanban, table`,
		`table.columns lists "id" twice`,
		`table.columns has an empty column`,
		`table.group_by "owner" is not one of none, parent, status, assignee, project`,
		`kanban.columns "kind" is not one of status, category`,
		`kanban.swimlane "label" is not one of none, assignee, project`,
	}
	if got := v.Problems(); !slices.Equal(got, want) {
		t.Errorf("Problems =\n%q\nwant\n%q", got, want)
	}
	ok, _ := Decode("x", []byte("layout: tree\nquery: \"\"\n"))
	if ps := ok.Problems(); len(ps) != 0 {
		t.Errorf("Problems of a good view = %q", ps)
	}
}

func TestMissingColumns(t *testing.T) {
	v, _ := Decode("x", []byte("layout: table\ntable:\n  columns: [id, due, gone, label, other]\n"))
	if got := v.MissingColumns(schema.Default()); !slices.Equal(got, []string{"gone", "other"}) {
		t.Errorf("MissingColumns = %q", got)
	}
}
```

（`TestNewEncode` の期待値は `Encoder` の書き方に従う。`query: status:todo` が引用符付きで出るなど、実際の出力が違ったら、その出力が YAML として同じ値に読めることを確かめたうえで期待値をその出力に合わせてよい。`full` の往復だけは1バイトも変えない。）

- [ ] **Step 8: view.go を書く**

```go
package view

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// The values each setting takes.
var (
	Layouts             = []string{"tree", "kanban", "table"}
	GroupBys            = []string{"none", "parent", "status", "assignee", "project"}
	KanbanColumnsValues = []string{"status", "category"}
	Swimlanes           = []string{"none", "assignee", "project"}
)

// The defaults of the settings a view leaves out.
var (
	DefaultColumns = []string{"id", "title", "status", "assignee", "due"}
)

const (
	DefaultGroupBy       = "none"
	DefaultKanbanColumns = "status"
	DefaultSwimlane      = "none"
)

// Table is the table: section of a view.
type Table struct {
	Columns []string `yaml:"columns,omitempty"`
	GroupBy string   `yaml:"group_by,omitempty"`
}

// Kanban is the kanban: section of a view.
type Kanban struct {
	Columns  string `yaml:"columns,omitempty"`
	Swimlane string `yaml:"swimlane,omitempty"`
}

// View is one saved view. A section that the file does not have is nil;
// its settings take the defaults. Unknown keys, at the top and in the
// sections, and comments are kept when the view is written back.
type View struct {
	Name   string // from the file name
	Layout string
	Query  string
	Table  *Table
	Kanban *Kanban
	Extra  model.Fields

	doc *model.Doc
}

var keys = []string{"layout", "query", "table", "kanban"}

// New is an empty view called name.
func New(name string) *View { return &View{Name: name} }

// Decode reads the file of the view called name. It fails only on what
// cannot be read: YAML that is not a mapping, or a value of the wrong shape.
// Values out of their range are for Problems.
func Decode(name string, data []byte) (*View, error) {
	d, err := model.ParseDoc(data, 0)
	if err != nil {
		return nil, err
	}
	v := &View{Name: name, doc: d}
	for key, out := range map[string]any{"layout": &v.Layout, "query": &v.Query, "table": &v.Table, "kanban": &v.Kanban} {
		if err := d.Decode(key, out); err != nil {
			return nil, err
		}
	}
	v.Extra = d.Extra(keys)
	return v, nil
}

// Encode writes the view, keeping what the file it was read from had.
func (v *View) Encode() ([]byte, error) {
	if v.doc == nil {
		v.doc = model.NewDoc()
	}
	e := v.doc.Encoder()
	e.Put("layout", v.Layout, false)
	e.Put("query", v.Query, false)
	if v.Table != nil {
		e.Put("table", v.Table, false)
	}
	if v.Kanban != nil {
		e.Put("kanban", v.Kanban, false)
	}
	return e.Bytes(v.Extra)
}

// TableColumns are the columns of the table, or the default ones.
func (v *View) TableColumns() []string {
	if v.Table == nil || len(v.Table.Columns) == 0 {
		return slices.Clone(DefaultColumns)
	}
	return slices.Clone(v.Table.Columns)
}

// GroupBy is what the table groups by, or none.
func (v *View) GroupBy() string {
	if v.Table == nil || v.Table.GroupBy == "" {
		return DefaultGroupBy
	}
	return v.Table.GroupBy
}

// KanbanColumns is what the columns of the kanban are, or status.
func (v *View) KanbanColumns() string {
	if v.Kanban == nil || v.Kanban.Columns == "" {
		return DefaultKanbanColumns
	}
	return v.Kanban.Columns
}

// Swimlane is what the kanban splits its rows by, or none.
func (v *View) Swimlane() string {
	if v.Kanban == nil || v.Kanban.Swimlane == "" {
		return DefaultSwimlane
	}
	return v.Kanban.Swimlane
}

// Problems lists the values of the view that are out of their range, in the
// order of the file's keys. The columns are checked against the schema by
// MissingColumns.
func (v *View) Problems() []string {
	var out []string
	oneOf := func(what, val string, vals []string) {
		if !slices.Contains(vals, val) {
			out = append(out, fmt.Sprintf("%s %q is not one of %s", what, val, strings.Join(vals, ", ")))
		}
	}
	oneOf("layout", v.Layout, Layouts)
	if v.Table != nil {
		seen := map[string]bool{}
		empty := false
		for _, c := range v.Table.Columns {
			switch {
			case c == "":
				empty = true
			case seen[c]:
				out = append(out, fmt.Sprintf("table.columns lists %q twice", c))
			}
			seen[c] = true
		}
		if empty {
			out = append(out, "table.columns has an empty column")
		}
		if v.Table.GroupBy != "" {
			oneOf("table.group_by", v.Table.GroupBy, GroupBys)
		}
	}
	if v.Kanban != nil {
		if v.Kanban.Columns != "" {
			oneOf("kanban.columns", v.Kanban.Columns, KanbanColumnsValues)
		}
		if v.Kanban.Swimlane != "" {
			oneOf("kanban.swimlane", v.Kanban.Swimlane, Swimlanes)
		}
	}
	return out
}

// MissingColumns lists the columns of the table that are neither a builtin
// field nor a property of sc, in their order. The GUI skips them.
func (v *View) MissingColumns(sc *schema.Schema) []string {
	var out []string
	if v.Table == nil {
		return nil
	}
	for _, c := range v.Table.Columns {
		if c == "" || IsBuiltin(c) {
			continue
		}
		if _, ok := sc.Prop(c); !ok {
			out = append(out, c)
		}
	}
	return out
}
```

- [ ] **Step 9: 一覧の失敗するテストを書く**

`internal/view/list_test.go`:

```go
package view

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeView(t *testing.T, dir, file, data string) {
	t.Helper()
	p := filepath.Join(dir, "views", file)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestListAndFind(t *testing.T) {
	dir := t.TempDir()
	if es, err := List(dir); err != nil || len(es) != 0 {
		t.Fatalf("List without views/ = %v, %v", es, err)
	}
	writeView(t, dir, "b.yaml", "layout: tree\n")
	writeView(t, dir, "a.yaml", "layout: table\n")
	writeView(t, dir, "broken.yaml", "- x\n")
	writeView(t, dir, "CON.yaml", "layout: tree\n")
	writeView(t, dir, "notes.txt", "x")
	writeView(t, dir, "sub/c.yaml", "layout: tree\n")
	es, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range es {
		names = append(names, e.Name)
	}
	if want := []string{"CON", "a", "b", "broken"}; !equal(names, want) {
		t.Fatalf("names = %q; want %q", names, want)
	}
	if es[0].Err == nil || es[1].Err != nil || es[1].View.Layout != "table" || es[1].Rel != "views/a.yaml" || es[3].Err == nil {
		t.Errorf("entries = %+v", es)
	}
	if e, ok, err := Find(dir, "b"); err != nil || !ok || e.View.Layout != "tree" {
		t.Errorf("Find b = %+v %v %v", e, ok, err)
	}
	// Find matches the name exactly, even where the file system folds case.
	if _, ok, err := Find(dir, "B"); err != nil || ok {
		t.Errorf("Find B = %v %v; want not found", ok, err)
	}
	if _, ok, _ := Find(dir, "broken"); !ok {
		t.Error("Find gives a broken view too, with its error")
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestListUnreadableDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "views"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := List(dir); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Errorf("List with a file at views = %v; want an error", err)
	}
}
```

（`equal` は `slices.Equal` で置き換えてよい。Windows では `CON.yaml` を作れないので、`TestListAndFind` の `CON.yaml` の行と期待値の `"CON"` は `runtime.GOOS != "windows"` のときだけにする。）

- [ ] **Step 10: list.go を書く**

```go
package view

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/model"
)

// ErrNotFound means no view has the name.
var ErrNotFound = errors.New("no such view")

// Entry is one file directly under views/ with the .yaml extension. Err
// tells why it is not a view: a name ValidName refuses, or a file that
// does not decode. View is nil then.
type Entry struct {
	Name string
	Rel  string // the path in the store, views/<name>.yaml
	View *View
	Err  error
}

// List reads the views of the store in storeDir, sorted by name. A store
// without views/ has none. Files in subdirectories, and files without the
// .yaml extension, are not views and are left out; doctor reports them.
func List(storeDir string) ([]Entry, error) {
	des, err := os.ReadDir(filepath.Join(storeDir, model.ViewsDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, de := range des {
		rel := model.ViewsDir + "/" + de.Name()
		name, ok := model.IsViewPath(rel)
		if !ok || !de.Type().IsRegular() {
			continue
		}
		out = append(out, read(storeDir, name, rel))
	}
	slices.SortFunc(out, func(a, b Entry) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func read(storeDir, name, rel string) Entry {
	e := Entry{Name: name, Rel: rel}
	if err := ValidName(name); err != nil {
		e.Err = err
		return e
	}
	data, err := os.ReadFile(filepath.Join(storeDir, filepath.FromSlash(rel))) //nolint:gosec // G304: the path is built from the store root and a listed name
	if err != nil {
		e.Err = err
		return e
	}
	if e.View, err = Decode(name, data); err != nil {
		e.View, e.Err = nil, fmt.Errorf("does not decode: %w", err)
	}
	return e
}

// Find returns the view whose file is named exactly name. The directory is
// listed, not opened by name, so that a file system that folds case does
// not hand back another view. ok is false when there is none.
func Find(storeDir, name string) (Entry, bool, error) {
	es, err := List(storeDir)
	if err != nil {
		return Entry{}, false, err
	}
	for _, e := range es {
		if e.Name == name {
			return e, true, nil
		}
	}
	return Entry{}, false, nil
}
```

- [ ] **Step 11: テストを通す**

Run: `go test -race ./internal/view/ ./internal/model/`
Expected: PASS

- [ ] **Step 12: lint と全体のテスト、コミット**

```bash
go test -race ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...
git add internal/model/path.go internal/model/path_test.go internal/view
git commit -m "feat(view): 保存済みビューの形式・名前の規則・一覧を internal/view に足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: views/ をキー単位でマージする

**Files:**
- Modify: `internal/merge/merge.go`（`kindOf`、`kindYAML` のコメント、`ByKey` を追加）
- Modify: `internal/merge/yaml.go:18-19`（コメントに views を足す）
- Modify: `internal/sync/history.go:46,63-65`（`isYAML` を `merge.ByKey` に）
- Test: `internal/merge/kinds_test.go`（追加）, `internal/sync/sync_test.go`（追加）

**Interfaces:**
- Consumes: `model.IsViewPath`（Task 1）
- Produces: `merge.ByKey(path string) bool`（schema.yaml・automations.yaml・store.yaml・`views/*.yaml` で true）

- [ ] **Step 1: マージの失敗するテストを書く**

`internal/merge/kinds_test.go` に追加:

```go
func TestViewYAMLByKey(t *testing.T) {
	const p = "views/今週.yaml"
	base := []byte("layout: table\nquery: \"status:todo\"\ntable:\n  columns: [id, title]\n")
	ours := []byte("layout: kanban\nquery: \"status:todo\"\ntable:\n  columns: [id, title]\n")
	theirs := []byte("layout: table\nquery: \"status:todo\"\ntable:\n  columns: [id, title, due]\n")
	r := mergeSym(t, p, Side{Data: base}, Side{Data: ours, Author: "a", When: whenAt(time.Unix(1, 0))}, Side{Data: theirs, Author: "b", When: whenAt(time.Unix(2, 0))})
	if want := "layout: kanban\nquery: \"status:todo\"\ntable:\n  columns: [id, title, due]\n"; string(r.Data) != want {
		t.Errorf("merged =\n%s\nwant\n%s", r.Data, want)
	}
	if len(r.Overwritten) != 0 {
		t.Errorf("overwritten = %v", r.Overwritten)
	}

	// Both change the query: the later change wins and the other is recorded.
	o2 := []byte("layout: table\nquery: \"status:done\"\ntable:\n  columns: [id, title]\n")
	t2 := []byte("layout: table\nquery: \"status:doing\"\ntable:\n  columns: [id, title]\n")
	r = mergeSym(t, p, Side{Data: base}, Side{Data: o2, Author: "a", When: whenAt(time.Unix(1, 0))}, Side{Data: t2, Author: "b", When: whenAt(time.Unix(2, 0))})
	if !bytes.Contains(r.Data, []byte("status:doing")) {
		t.Errorf("merged =\n%s", r.Data)
	}
	want := []Overwritten{{Path: p, Field: "query", Value: `"status:done"`, Author: "a"}}
	if !sameOverwritten(r.Overwritten, want) {
		t.Errorf("overwritten = %v", r.Overwritten)
	}

	// One side removes the view, the other changes it: the change stays.
	r = mergeSym(t, p, Side{Data: base}, Side{Data: nil, Author: "a"}, Side{Data: theirs, Author: "b"})
	if string(r.Data) != string(theirs) || len(r.Notices) != 1 || r.Notices[0].Kind != Revived || r.Notices[0].Author != "a" {
		t.Errorf("remove vs change = %q %v", r.Data, r.Notices)
	}
}

func TestByKey(t *testing.T) {
	for p, want := range map[string]bool{
		"schema.yaml": true, model.AutomationsPath: true, model.StoreMetaPath: true,
		"views/a.yaml": true, "views/sub/a.yaml": false, "views/a.md": false, "tasks/0/1/x.md": false,
	} {
		if got := ByKey(p); got != want {
			t.Errorf("ByKey(%q) = %v", p, got)
		}
	}
}
```

（`bytes`・`model` の import が kinds_test.go になければ足す。最初のケースの結果は author の大きい側（theirs）の文書を土台に作られる。バイト列が期待値と違ったら、両方を YAML として読んで値が同じことを確かめたうえで期待値をその出力に合わせてよい。`Overwritten` と `Notices` の確認は変えない。）

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/merge/ -run 'TestViewYAMLByKey|TestByKey'`
Expected: FAIL（`undefined: ByKey`。マージは `other` に落ちて丸ごと LWW になる）

- [ ] **Step 3: 実装する**

`internal/merge/merge.go`:

```go
	kindYAML    // schema.yaml, automations.yaml, store.yaml and views/*.yaml, merged key by key
```

```go
func kindOf(path string) kind {
	switch path {
	case schema.Path, model.AutomationsPath, model.StoreMetaPath:
		return kindYAML
	case model.NumbersPath:
		return kindNumbers
	}
	if _, ok := model.IsViewPath(path); ok {
		return kindYAML
	}
	// ...（以下は今のまま）
}

// ByKey reports whether the file at path is merged key by key, as
// schema.yaml is: the merge then asks Side.When when each side changed a
// value.
func ByKey(path string) bool { return kindOf(path) == kindYAML }
```

`internal/merge/yaml.go` の `yaml` のコメントを `// yaml merges schema.yaml, automations.yaml, store.yaml or a view key by key.` にする。`YAMLValues` のコメントの「schema.yaml, automations.yaml or store.yaml」も「a file that ByKey reports」に直す。

`internal/sync/history.go`: `if isYAML(path) {` を `if merge.ByKey(path) {` にし、`isYAML` を削除する。これで使われなくなった import（`schema`・`model`）があれば削除する（ほかで使っていれば残す）。

- [ ] **Step 4: 同期の失敗するテストを書く**

`internal/sync/sync_test.go` に追加（`TestMergeSchemaBothSides` の後）:

```go
func TestMergeViewBothSides(t *testing.T) {
	c := newCluster(t, 2)
	const p = "views/w.yaml"
	put := func(i int, data string) {
		t.Helper()
		if _, err := c.stores[i].Update(c.op(i, "view", model.ULID{}), func(tx *store.Tx) error { return tx.PutFile(p, []byte(data)) }); err != nil {
			t.Fatal(err)
		}
	}
	put(0, "layout: table\nquery: \"\"\n")
	c.sync(0)
	c.sync(1)
	put(0, "layout: kanban\nquery: \"\"\n")
	put(1, "layout: table\nquery: \"status:todo\"\n")
	c.sync(0)
	r := c.sync(1)
	if r.Merges != 1 || len(r.Overwritten) != 0 {
		t.Errorf("result = %+v", r)
	}
	b, err := os.ReadFile(filepath.Join(c.stores[1].Dir(), filepath.FromSlash(p)))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "layout: kanban\nquery: \"status:todo\"\n" {
		t.Errorf("merged view =\n%s", b)
	}
	c.sync(0)
	if c.headTree(0) != c.headTree(1) {
		t.Error("clones differ")
	}
}
```

（`c.op` の時刻が2つの Store で同じでも、変えたキーが違うのでマージは時刻に依存しない。import の `os`・`filepath` がなければ足す。）

- [ ] **Step 5: テストを通す**

Run: `go test -race ./internal/merge/ ./internal/sync/`
Expected: PASS

- [ ] **Step 6: 全体の確認とコミット**

```bash
go test -race ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...
git add internal/merge internal/sync
git commit -m "feat(merge): views/*.yaml を schema.yaml と同じくキー単位でマージする

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: ビューの操作（internal/ops）

**Files:**
- Create: `internal/ops/views.go`
- Test: `internal/ops/views_test.go`

**Interfaces:**
- Consumes: `view.*`（Task 1）、`merge` のキー単位のマージ（Task 2。undo が使う）、`ops.ParseQuery`・`ops.HasTerm`（既存）
- Produces:
  - `ops.SavedView{Name, Layout, Query string; Table SavedTable; Kanban SavedKanban}`（JSON: `name` `layout` `query` `table{columns, group_by}` `kanban{columns, swimlane}`。既定値を補った値）、`ops.SavedViewOf(v *view.View) SavedView`
  - `ops.ViewChange{Layout, Query, GroupBy, KanbanColumns, Swimlane *string; Columns *[]string}`（nil は変えない）
  - `ops.ViewResult{View *view.View; OldName string; Committed bool; Commit string}`
  - `(*Session).ViewSave(name string, ch ViewChange) (ViewResult, error)`
  - `(*Session).ViewList() ([]SavedView, error)`
  - `(*Session).ViewGet(name string) (*view.View, error)`
  - `(*Session).ViewRemove(name string) (ViewResult, error)`
  - `(*Session).ViewRename(oldName, newName string) (ViewResult, error)`

エラー:

| 場面 | Kind | Code | Args |
|---|---|---|---|
| 名前が規則に合わない | Usage | `view.bad_name` | `value` |
| 名前のビューがない | NotFound | `view.not_found` | `name` |
| 大文字小文字だけ違う別のビューがある／rename の先がある／大文字小文字だけの rename | Conflict | `view.name_taken` | `name`, `existing` |
| layout・group_by・kanban columns・swimlane が範囲外、列の重複・空 | Usage | `view.bad_value` | — |
| `--columns` の列が組み込みでもプロパティでもない | Invalid | `view.bad_column` | `column` |
| tree で `sort:`・`group:` | Usage | `view.tree_order` | — |
| 上書きの保存先のファイルが読めない | Invalid | `view.broken` | `name` |
| クエリが読めない | Usage | `query.invalid`（`ParseQuery` のまま） | — |

文言（英語。CLI はこのまま出す）:
- `view.not_found`: `no view named %q: no such view`（`view.ErrNotFound` を包む）
- `view.name_taken`: `view %q already exists` ／ 大文字小文字だけ違う: `view %q already exists as %q; names that differ only in case are the same name`
- `view.bad_value`: `Problems()` の文を `; ` でつないだもの
- `view.bad_column`: `column %q is neither a field (id, title, kind, status, assignee, label, parent, created, updated) nor a property of the schema`
- `view.tree_order`: `a tree view orders tasks by rank; its query takes neither group: nor sort:`
- `view.broken`: `views/%s.yaml does not decode: %v; fix it by hand or remove it with hedgehug view rm`

- [ ] **Step 1: 失敗するテストを書く**

`internal/ops/views_test.go`:

```go
package ops

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/view"
)

func ptr[T any](v T) *T { return &v }

func (ts *testSession) viewFile(name string) string {
	ts.t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(s.Dir(), "views", name+".yaml"))
	if err != nil {
		ts.t.Fatal(err)
	}
	return string(b)
}

func (ts *testSession) headSubject() string {
	ts.t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	es, err := s.Repo().Log(nil, 1)
	if err != nil || len(es) == 0 {
		ts.t.Fatalf("log: %v", err)
	}
	return es[0].Subject
}

func TestViewSaveCreateAndOverwrite(t *testing.T) {
	ts := newTestSession(t)
	r, err := ts.ViewSave("今週", ViewChange{})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Committed || r.Commit == "" || r.View.Layout != "tree" || r.View.Query != "" {
		t.Errorf("create = %+v %+v", r, r.View)
	}
	if got := ts.headSubject(); got != "hedgehug: view save 今週" {
		t.Errorf("subject = %q", got)
	}
	if got := ts.viewFile("今週"); got != "layout: tree\nquery: \"\"\n" {
		t.Errorf("file = %q", got)
	}
	r, err = ts.ViewSave("今週", ViewChange{Layout: ptr("table"), Columns: ptr([]string{"id", "title", "due"})})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Committed || r.View.Layout != "table" || !slices.Equal(r.View.TableColumns(), []string{"id", "title", "due"}) {
		t.Errorf("overwrite = %+v %+v", r, r.View)
	}
	// Only the flags given change; a save that changes nothing commits nothing.
	r, err = ts.ViewSave("今週", ViewChange{Query: ptr("status:todo")})
	if err != nil || r.View.Layout != "table" || r.View.Query != "status:todo" {
		t.Fatalf("query only = %+v, %v", r, err)
	}
	r, err = ts.ViewSave("今週", ViewChange{Query: ptr("status:todo")})
	if err != nil || r.Committed || r.Commit != "" {
		t.Errorf("same again = %+v, %v", r, err)
	}
}

func TestViewSaveKeepsUnknownKeys(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.ViewSave("a", ViewChange{Layout: ptr("table")}); err != nil {
		t.Fatal(err)
	}
	ts.writeAndCommit("views/a.yaml", "# note\nlayout: table\nquery: \"\"\ncolor: red\n")
	if _, err := ts.ViewSave("a", ViewChange{Query: ptr("status:todo")}); err != nil {
		t.Fatal(err)
	}
	got := ts.viewFile("a")
	for _, w := range []string{"# note", "color: red", "status:todo"} {
		if !strings.Contains(got, w) {
			t.Errorf("file lacks %q:\n%s", w, got)
		}
	}
}

func TestViewSaveRefuses(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.ViewSave("Release", ViewChange{}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		ch   ViewChange
		kind Kind
		code string
	}{
		{"a/b", ViewChange{}, KindUsage, "view.bad_name"},
		{"CON", ViewChange{}, KindUsage, "view.bad_name"},
		{"release", ViewChange{}, KindConflict, "view.name_taken"},
		{"x", ViewChange{Layout: ptr("gantt")}, KindUsage, "view.bad_value"},
		{"x", ViewChange{Layout: ptr("table"), GroupBy: ptr("owner")}, KindUsage, "view.bad_value"},
		{"x", ViewChange{KanbanColumns: ptr("kind")}, KindUsage, "view.bad_value"},
		{"x", ViewChange{Swimlane: ptr("label")}, KindUsage, "view.bad_value"},
		{"x", ViewChange{Columns: ptr([]string{"id", "id"})}, KindUsage, "view.bad_value"},
		{"x", ViewChange{Columns: ptr([]string{"id", "nope"})}, KindInvalid, "view.bad_column"},
		{"x", ViewChange{Query: ptr("status:todo sort:due")}, KindUsage, "view.tree_order"},
		{"x", ViewChange{Layout: ptr("tree"), Query: ptr("group:status")}, KindUsage, "view.tree_order"},
		{"x", ViewChange{Query: ptr(`"abc`)}, KindUsage, "query.invalid"},
	} {
		_, err := ts.ViewSave(c.name, c.ch)
		wantErr(t, err, c.kind, c.code)
	}
	// A table may sort; switching it to tree then is refused.
	if _, err := ts.ViewSave("t", ViewChange{Layout: ptr("table"), Query: ptr("sort:due")}); err != nil {
		t.Fatal(err)
	}
	_, err := ts.ViewSave("t", ViewChange{Layout: ptr("tree")})
	wantErr(t, err, KindUsage, "view.tree_order")
	if err.Error() != "a tree view orders tasks by rank; its query takes neither group: nor sort:" {
		t.Errorf("message = %q", err)
	}
}

func TestViewListGetRemoveRename(t *testing.T) {
	ts := newTestSession(t)
	if vs, err := ts.ViewList(); err != nil || len(vs) != 0 {
		t.Fatalf("empty list = %v, %v", vs, err)
	}
	for _, n := range []string{"b", "a"} {
		if _, err := ts.ViewSave(n, ViewChange{Layout: ptr("kanban"), Query: ptr("label:" + n)}); err != nil {
			t.Fatal(err)
		}
	}
	vs, err := ts.ViewList()
	if err != nil {
		t.Fatal(err)
	}
	want := []SavedView{
		{Name: "a", Layout: "kanban", Query: "label:a", Table: SavedTable{Columns: view.DefaultColumns, GroupBy: "none"}, Kanban: SavedKanban{Columns: "status", Swimlane: "none"}},
		{Name: "b", Layout: "kanban", Query: "label:b", Table: SavedTable{Columns: view.DefaultColumns, GroupBy: "none"}, Kanban: SavedKanban{Columns: "status", Swimlane: "none"}},
	}
	if len(vs) != 2 || vs[0].Name != want[0].Name || vs[1].Query != want[1].Query || !slices.Equal(vs[0].Table.Columns, want[0].Table.Columns) || vs[0].Kanban != want[0].Kanban {
		t.Errorf("list = %+v", vs)
	}
	v, err := ts.ViewGet("a")
	if err != nil || v.Query != "label:a" {
		t.Errorf("get = %+v, %v", v, err)
	}
	_, err = ts.ViewGet("A")
	wantErr(t, err, KindNotFound, "view.not_found")

	r, err := ts.ViewRename("a", "c")
	if err != nil || !r.Committed || r.OldName != "a" || r.View.Name != "c" {
		t.Fatalf("rename = %+v, %v", r, err)
	}
	if got := ts.headSubject(); got != "hedgehug: view rename a c" {
		t.Errorf("subject = %q", got)
	}
	if !strings.Contains(ts.viewFile("c"), "label:a") {
		t.Errorf("renamed file = %q", ts.viewFile("c"))
	}
	_, err = ts.ViewRename("c", "B")
	wantErr(t, err, KindConflict, "view.name_taken")
	_, err = ts.ViewRename("c", "C")
	wantErr(t, err, KindConflict, "view.name_taken")
	_, err = ts.ViewRename("nope", "d")
	wantErr(t, err, KindNotFound, "view.not_found")
	_, err = ts.ViewRename("c", "d/e")
	wantErr(t, err, KindUsage, "view.bad_name")

	r, err = ts.ViewRemove("c")
	if err != nil || !r.Committed || r.View.Name != "c" {
		t.Fatalf("remove = %+v, %v", r, err)
	}
	if got := ts.headSubject(); got != "hedgehug: view rm c" {
		t.Errorf("subject = %q", got)
	}
	_, err = ts.ViewRemove("c")
	wantErr(t, err, KindNotFound, "view.not_found")
	if vs, _ := ts.ViewList(); len(vs) != 1 || vs[0].Name != "b" {
		t.Errorf("list after = %+v", vs)
	}
}

func TestViewBrokenFile(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.ViewSave("ok", ViewChange{}); err != nil {
		t.Fatal(err)
	}
	ts.writeAndCommit("views/bad.yaml", "- not a mapping\n")
	ts.writeAndCommit("views/.bad.yaml", "layout: tree\n") // a name ValidName refuses, on every system
	vs, err := ts.ViewList()
	if err != nil || len(vs) != 1 || vs[0].Name != "ok" {
		t.Errorf("list = %+v, %v", vs, err)
	}
	// .bad sorts before bad.
	if len(ts.warns) != 2 || !strings.HasPrefix(ts.warns[0], "views/.bad.yaml: ") || !strings.HasPrefix(ts.warns[1], "views/bad.yaml: does not decode: ") ||
		!strings.HasSuffix(ts.warns[1], "; run hedgehug doctor") {
		t.Errorf("warns = %q", ts.warns)
	}
	_, err = ts.ViewSave("bad", ViewChange{Query: ptr("")})
	wantErr(t, err, KindInvalid, "view.broken")
	if _, err := ts.ViewRename("bad", "worse"); err != nil {
		t.Errorf("rename of a broken view = %v", err)
	}
	if ts.viewFile("worse") != "- not a mapping\n" {
		t.Errorf("the bytes moved = %q", ts.viewFile("worse"))
	}
	if _, err := ts.ViewRemove("worse"); err != nil {
		t.Errorf("remove of a broken view = %v", err)
	}
}
```

`internal/ops/helpers_test.go` に次の helper を足す（`store` は import 済み）:

```go
// writeAndCommit writes data to rel in the test store and commits it, as a
// hand edit that was committed would be.
func (ts *testSession) writeAndCommit(rel, data string) {
	ts.t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	if _, err := ts.Update(s, store.Op{Name: "test"}, func(tx *store.Tx) error { return tx.PutFile(rel, []byte(data)) }); err != nil {
		ts.t.Fatal(err)
	}
}
```

（`wantErr(t, err, kind, code)` は `task_test.go` の既存の helper。`Repo().Log(prefixes, limit)` は `[]gitrepo.LogEntry` を新しい順に返す。）

undo のテストも足す（`undo_test.go` のアカウントの用意に合わせる）:

```go
func TestViewUndo(t *testing.T) {
	ts := newTestSession(t)
	ts.appendConfig(aliceAccount) // as undo_test.go does
	if _, err := ts.ViewSave("a", ViewChange{Layout: ptr("table")}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.ViewSave("a", ViewChange{Query: ptr("status:todo")}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Undo(); err != nil {
		t.Fatal(err)
	}
	if v, err := ts.ViewGet("a"); err != nil || v.Query != "" || v.Layout != "table" {
		t.Errorf("after undo = %+v, %v", v, err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/ops/ -run 'TestView'`
Expected: FAIL（`undefined: ViewSave` など）

- [ ] **Step 3: views.go を書く**

```go
package ops

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/view"
)

// SavedTable and SavedKanban are the sections of a view, with defaults.
type SavedTable struct {
	Columns []string `json:"columns"`
	GroupBy string   `json:"group_by"`
}

type SavedKanban struct {
	Columns  string `json:"columns"`
	Swimlane string `json:"swimlane"`
}

// SavedView is a saved view as --json prints it: every setting, with the
// defaults of the ones the file leaves out.
type SavedView struct {
	Name   string      `json:"name"`
	Layout string      `json:"layout"`
	Query  string      `json:"query"`
	Table  SavedTable  `json:"table"`
	Kanban SavedKanban `json:"kanban"`
}

func SavedViewOf(v *view.View) SavedView {
	return SavedView{
		Name: v.Name, Layout: v.Layout, Query: v.Query,
		Table:  SavedTable{Columns: v.TableColumns(), GroupBy: v.GroupBy()},
		Kanban: SavedKanban{Columns: v.KanbanColumns(), Swimlane: v.Swimlane()},
	}
}

// ViewChange is what a save sets; a nil field is left as it is (or takes
// its default in a new view).
type ViewChange struct {
	Layout, Query, GroupBy, KanbanColumns, Swimlane *string
	Columns                                         *[]string
}

// ViewResult is what a view operation did.
type ViewResult struct {
	View      *view.View // as saved; for ViewRemove, as it was (nil for a file that did not decode)
	OldName   string     // ViewRename
	Committed bool
	Commit    string // hex hash; "" when nothing was committed
}

func viewOp(detail string) store.Op { return store.Op{Name: "view", Detail: detail} }

func badViewName(name string) error {
	if err := view.ValidName(name); err != nil {
		return newErr(KindUsage, "view.bad_name", err, "value", name)
	}
	return nil
}

func viewNotFound(name string) error {
	return newErr(KindNotFound, "view.not_found", fmt.Errorf("no view named %q: %w", name, view.ErrNotFound), "name", name)
}

// sameNameOf finds a view other than name whose name is the same but for
// case.
func sameNameOf(es []view.Entry, name string) (string, bool) {
	for _, e := range es {
		if e.Name != name && view.SameName(e.Name, name) {
			return e.Name, true
		}
	}
	return "", false
}

func nameTaken(name, existing string) error {
	if existing == name {
		return newErr(KindConflict, "view.name_taken", fmt.Errorf("view %q already exists", name), "name", name, "existing", existing)
	}
	return newErr(KindConflict, "view.name_taken",
		fmt.Errorf("view %q already exists as %q; names that differ only in case are the same name", name, existing), "name", name, "existing", existing)
}

// ViewSave creates the view name, or changes the settings ch gives of the
// view that has it.
func (ss *Session) ViewSave(name string, ch ViewChange) (ViewResult, error) {
	if err := badViewName(name); err != nil {
		return ViewResult{}, err
	}
	if ch.Query != nil {
		if _, err := ParseQuery(*ch.Query); err != nil {
			return ViewResult{}, err
		}
	}
	s, err := ss.OpenStore()
	if err != nil {
		return ViewResult{}, err
	}
	var v *view.View
	hash, err := ss.UpdateCommit(s, viewOp("save "+name), func(tx *store.Tx) error {
		es, err := view.List(s.Dir())
		if err != nil {
			return err
		}
		if other, ok := sameNameOf(es, name); ok {
			return nameTaken(name, other)
		}
		i := slices.IndexFunc(es, func(e view.Entry) bool { return e.Name == name })
		switch {
		case i < 0:
			v = view.New(name)
			v.Layout = "tree"
		case es[i].Err != nil:
			return newErr(KindInvalid, "view.broken",
				fmt.Errorf("%s does not decode: %w; fix it by hand or remove it with hedgehug view rm", es[i].Rel, errors.Unwrap(es[i].Err)), "name", name)
		default:
			v = es[i].View
		}
		if err := applyViewChange(v, ch, s); err != nil {
			return err
		}
		data, err := v.Encode()
		if err != nil {
			return err
		}
		return tx.PutFile(model.ViewPath(name), data)
	})
	if err != nil {
		return ViewResult{}, err
	}
	return ViewResult{View: v, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}

// applyViewChange sets what ch gives and checks the view that results.
func applyViewChange(v *view.View, ch ViewChange, s *store.Store) error {
	set := func(dst *string, src *string) {
		if src != nil {
			*dst = *src
		}
	}
	set(&v.Layout, ch.Layout)
	set(&v.Query, ch.Query)
	if ch.Columns != nil || ch.GroupBy != nil {
		if v.Table == nil {
			v.Table = &view.Table{}
		}
		if ch.Columns != nil {
			v.Table.Columns = slices.Clone(*ch.Columns)
		}
		set(&v.Table.GroupBy, ch.GroupBy)
	}
	if ch.KanbanColumns != nil || ch.Swimlane != nil {
		if v.Kanban == nil {
			v.Kanban = &view.Kanban{}
		}
		set(&v.Kanban.Columns, ch.KanbanColumns)
		set(&v.Kanban.Swimlane, ch.Swimlane)
	}
	if ps := v.Problems(); len(ps) > 0 {
		return newErr(KindUsage, "view.bad_value", errors.New(strings.Join(ps, "; ")))
	}
	if ch.Columns != nil {
		if miss := v.MissingColumns(s.Schema()); len(miss) > 0 {
			return newErr(KindInvalid, "view.bad_column", fmt.Errorf("column %q is neither a field (%s) nor a property of the schema", miss[0], strings.Join(view.Builtins, ", ")), "column", miss[0])
		}
	}
	if v.Layout == "tree" {
		q, err := ParseQuery(v.Query)
		if err != nil {
			return err
		}
		if HasTerm(q, "sort") || HasTerm(q, "group") {
			//nolint:revive,staticcheck // the message is a sentence, as the CLI prints it
			return newErr(KindUsage, "view.tree_order", errors.New("a tree view orders tasks by rank; its query takes neither group: nor sort:"))
		}
	}
	return nil
}

// ViewList lists the views of the store by name. A file that is not a view
// it can read is left out with a warning.
func (ss *Session) ViewList() ([]SavedView, error) {
	s, err := ss.OpenStore()
	if err != nil {
		return nil, err
	}
	es, err := view.List(s.Dir())
	if err != nil {
		return nil, err
	}
	out := []SavedView{}
	for _, e := range es {
		if e.Err != nil {
			ss.warn(fmt.Sprintf("%s: %v; run hedgehug doctor", e.Rel, e.Err))
			continue
		}
		out = append(out, SavedViewOf(e.View))
	}
	return out, nil
}

// ViewGet reads the view called exactly name.
func (ss *Session) ViewGet(name string) (*view.View, error) {
	s, err := ss.OpenStore()
	if err != nil {
		return nil, err
	}
	e, ok, err := view.Find(s.Dir(), name)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, viewNotFound(name)
	}
	if e.Err != nil {
		return nil, newErr(KindInvalid, "view.broken", fmt.Errorf("%s: %w; fix it by hand or remove it with hedgehug view rm", e.Rel, e.Err), "name", name)
	}
	return e.View, nil
}

// ViewRemove removes the view called exactly name, readable or not.
func (ss *Session) ViewRemove(name string) (ViewResult, error) {
	s, err := ss.OpenStore()
	if err != nil {
		return ViewResult{}, err
	}
	var v *view.View
	hash, err := ss.UpdateCommit(s, viewOp("rm "+name), func(tx *store.Tx) error {
		e, ok, err := view.Find(s.Dir(), name)
		if err != nil {
			return err
		}
		if !ok {
			return viewNotFound(name)
		}
		v = e.View
		return tx.RemoveFile(e.Rel)
	})
	if err != nil {
		return ViewResult{}, err
	}
	if v == nil {
		v = view.New(name)
	}
	return ViewResult{View: v, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}

// ViewRename moves the view oldName to newName, as its bytes are. A name
// that differs from oldName only in case is refused: on a file system that
// folds case it is the same file.
func (ss *Session) ViewRename(oldName, newName string) (ViewResult, error) {
	if err := badViewName(newName); err != nil {
		return ViewResult{}, err
	}
	s, err := ss.OpenStore()
	if err != nil {
		return ViewResult{}, err
	}
	v := view.New(newName)
	hash, err := ss.UpdateCommit(s, viewOp("rename "+oldName+" "+newName), func(tx *store.Tx) error {
		es, err := view.List(s.Dir())
		if err != nil {
			return err
		}
		i := slices.IndexFunc(es, func(e view.Entry) bool { return e.Name == oldName })
		if i < 0 {
			return viewNotFound(oldName)
		}
		for _, e := range es {
			if view.SameName(e.Name, newName) {
				return nameTaken(newName, e.Name)
			}
		}
		data, err := os.ReadFile(filepath.Join(s.Dir(), filepath.FromSlash(es[i].Rel))) //nolint:gosec // G304: the path is built from the store root and a listed name
		if err != nil {
			return err
		}
		if es[i].View != nil {
			if dv, err := view.Decode(newName, data); err == nil {
				v = dv
			}
		}
		if err := tx.RemoveFile(es[i].Rel); err != nil {
			return err
		}
		return tx.PutFile(model.ViewPath(newName), data)
	})
	if err != nil {
		return ViewResult{}, err
	}
	return ViewResult{View: v, OldName: oldName, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}
```

（`os`・`path/filepath` を import に足す。`ViewRename("c", "C")` は `SameName("c","C")` が真なので `view.name_taken` になる。`ViewRename("a","a")` も同じく拒否される。`view.broken` の `errors.Unwrap(es[i].Err)` は `List` が付けた「does not decode: 」を外すため。`ViewSave` で `ParseQuery` を tx の外で一度呼ぶのは、ロックの前に使い方の誤りを返すため。）

- [ ] **Step 4: テストを通す**

Run: `go test -race ./internal/ops/ -run 'TestView'`
Expected: PASS。`TestViewSaveCreateAndOverwrite` のファイルの中身の期待値（`layout: tree\nquery: ""\n`）が `Encoder` の出力と違ったら、YAML として同じ値であることを確かめて期待値を合わせる。

- [ ] **Step 5: 全体の確認とコミット**

```bash
go test -race ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...
git add internal/ops
git commit -m "feat(ops): 保存済みビューの保存・一覧・取得・削除・名前の変更を足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: `hh view`（CLI）

**Files:**
- Create: `cmd/hedgehug/view.go`
- Modify: `cmd/hedgehug/app.go:55`（`a.viewCmd()` を `a.workspaceCmd()` の後に）
- Test: `cmd/hedgehug/view_test.go`、golden: `cmd/hedgehug/testdata/TestViewCommands/*.golden`

**Interfaces:**
- Consumes: `ops.ViewSave/ViewList/ViewGet/ViewRemove/ViewRename`、`ops.SavedViewOf`（Task 3）、`a.ls`・`a.lsTree`（既存）
- Produces: コマンド `hh view save|list|show|rm|rename`

出力（テキスト）:
- save: 作成・変更したら `Saved view <name> (<layout>)`、変更がなければ `No changes: view <name> is already saved so`
- list: `writeTable` で `NAME  LAYOUT  QUERY`（クエリが空なら `-`）
- show: layout が tree なら `hh ls --tree` と同じ、それ以外は `hh ls` と同じ（`--json` もそれぞれと同じ）
- rm: `Removed view <name>`
- rename: `Renamed view <old> to <new>`
- `--json`: save・rm・rename は `SavedView`（rm で読めないファイルだったときは `{"name": ...}` 以外が既定値の `SavedViewOf(view.New(name))`）、list は `[]SavedView`

- [ ] **Step 1: 失敗するテストを書く**

`cmd/hedgehug/view_test.go`:

```go
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

func TestViewCommands(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("Project", "--kind", "project")
	te.addJSON("Alpha", "--parent", p.ID, "--status", "done")
	te.addJSON("Beta", "--parent", p.ID)

	te.goldenOK("save", "view", "save", "今週", "--layout", "table", "--query", "status:todo", "--columns", "id,title,due", "--group-by", "parent")
	te.goldenOK("save-again", "view", "save", "今週", "--query", "status:todo")
	te.goldenOK("save-tree", "view", "save", "tree", "--query", "kind:task")
	te.goldenOK("list", "view", "list")
	te.goldenOK("list-json", "view", "list", "--json")
	te.goldenOK("show-table", "view", "show", "今週")
	te.goldenOK("show-tree", "view", "show", "tree")
	te.goldenOK("rename", "view", "rename", "tree", "木")
	te.goldenOK("rm", "view", "rm", "木")
	te.goldenRun("not-found", 1, "view", "show", "木")
	te.goldenRun("bad-name", 2, "view", "save", "a/b")
	te.goldenRun("tree-sort", 2, "view", "save", "x", "--query", "sort:due")
	te.ok("view", "save", "Release")
	te.goldenRun("case", 1, "view", "save", "release")
	te.goldenRun("no-sub", 2, "view")

	var v ops.SavedView
	if err := json.Unmarshal([]byte(te.ok("view", "save", "k", "--layout", "kanban", "--kanban-columns", "category", "--swimlane", "assignee", "--json")), &v); err != nil {
		t.Fatal(err)
	}
	if v.Name != "k" || v.Kanban.Columns != "category" || v.Kanban.Swimlane != "assignee" || v.Table.GroupBy != "none" {
		t.Errorf("save --json = %+v", v)
	}
	if c := te.headCommit("me"); !strings.HasPrefix(c.Message, "hedgehug: view save k\n") || !strings.Contains(c.Message, "Hedgehug-Op: view") {
		t.Errorf("commit = %q", c.Message)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./cmd/hedgehug/ -run TestViewCommands`
Expected: FAIL（`unknown command "view"`）

- [ ] **Step 3: view.go を書く**

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/view"
)

func (a *app) viewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Manage the saved views of a store",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	var layout, query, groupBy, kanbanColumns, swimlane string
	var columns []string
	save := &cobra.Command{
		Use:   "save <name>",
		Short: "Save a view, or change the settings given of a saved one",
		Args:  exactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			var ch ops.ViewChange
			str := func(flag string, v *string) *string {
				if c.Flags().Changed(flag) {
					return v
				}
				return nil
			}
			ch.Layout, ch.Query, ch.GroupBy = str("layout", &layout), str("query", &query), str("group-by", &groupBy)
			ch.KanbanColumns, ch.Swimlane = str("kanban-columns", &kanbanColumns), str("swimlane", &swimlane)
			if c.Flags().Changed("columns") {
				ch.Columns = &columns
			}
			return a.viewSave(args[0], ch)
		},
	}
	f := save.Flags()
	f.StringVar(&layout, "layout", "", "tree, kanban or table")
	f.StringVar(&query, "query", "", "the query of the view")
	f.StringSliceVar(&columns, "columns", nil, "the columns of the table, such as id,title,status,due")
	f.StringVar(&groupBy, "group-by", "", "what the table groups by: none, parent, status, assignee or project")
	f.StringVar(&kanbanColumns, "kanban-columns", "", "the columns of the kanban: status or category")
	f.StringVar(&swimlane, "swimlane", "", "the rows of the kanban: none, assignee or project")
	cmd.AddCommand(save,
		&cobra.Command{
			Use:   "list",
			Short: "List the saved views",
			Args:  cobra.NoArgs,
			RunE:  func(_ *cobra.Command, _ []string) error { return a.viewList() },
		},
		&cobra.Command{
			Use:   "show <name>",
			Short: "List the tasks of a saved view",
			Args:  exactArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return a.viewShow(args[0]) },
		},
		&cobra.Command{
			Use:   "rm <name>",
			Short: "Remove a saved view",
			Args:  exactArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return a.viewRm(args[0]) },
		},
		&cobra.Command{
			Use:   "rename <old> <new>",
			Short: "Rename a saved view",
			Args:  exactArgs(2),
			RunE:  func(_ *cobra.Command, args []string) error { return a.viewRename(args[0], args[1]) },
		},
	)
	return cmd
}

func (a *app) viewSave(name string, ch ops.ViewChange) error {
	r, err := a.ss.ViewSave(name, ch)
	if err != nil {
		return err
	}
	line := fmt.Sprintf("Saved view %s (%s)", name, r.View.Layout)
	if !r.Committed {
		line = fmt.Sprintf("%s: view %s is already saved so", verb(false, ""), name)
	}
	return a.printView(r.View, line)
}

func (a *app) viewRm(name string) error {
	r, err := a.ss.ViewRemove(name)
	if err != nil {
		return err
	}
	return a.printView(r.View, "Removed view "+name)
}

func (a *app) viewRename(oldName, newName string) error {
	r, err := a.ss.ViewRename(oldName, newName)
	if err != nil {
		return err
	}
	return a.printView(r.View, fmt.Sprintf("Renamed view %s to %s", oldName, newName))
}

func (a *app) viewList() error {
	vs, err := a.ss.ViewList()
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(vs)
	}
	rows := make([][]string, len(vs))
	for i, v := range vs {
		q := v.Query
		if q == "" {
			q = "-"
		}
		rows[i] = []string{v.Name, v.Layout, q}
	}
	return writeTable(a.stdout, []string{"NAME", "LAYOUT", "QUERY"}, rows, nil)
}

func (a *app) viewShow(name string) error {
	v, err := a.ss.ViewGet(name)
	if err != nil {
		return err
	}
	q, err := ops.ParseQuery(v.Query)
	if err != nil {
		return err
	}
	if v.Layout == "tree" {
		return a.lsTree(q, false, nil)
	}
	return a.ls(q, false, nil)
}

// printView prints v as JSON, or line as text.
func (a *app) printView(v *view.View, line string) error {
	if a.jsonOut {
		return a.printJSON(ops.SavedViewOf(v))
	}
	_, err := fmt.Fprintln(a.stdout, line)
	return err
}

```

（`strings` は使わないので import しない。`exactArgs`・`usageErrorf`・`writeTable`・`verb` は既存。`writeTable` の最後の引数の意味は workspace.go の呼び出しに合わせる。）

`cmd/hedgehug/app.go:55` の `a.workspaceCmd(),` の後に `a.viewCmd(),` を足す。

- [ ] **Step 4: golden を作って中身を確かめる**

Run: `go test ./cmd/hedgehug/ -run TestViewCommands -update && git status --short cmd/hedgehug/testdata`
Expected: `testdata/TestViewCommands/` の下だけが新しくできる。各ファイルを開き、次を確かめる:
- `save.golden` = `Saved view 今週 (table)`、`save-again.golden` = `No changes: view 今週 is already saved so`
- `list.golden` に `今週  table  status:todo` と `tree  tree  kind:task`
- `show-table.golden` が `hh ls status:todo` と同じ形、`show-tree.golden` が `hh ls --tree kind:task` と同じ形（祖先の Project が薄い表示の行として出る）
- `not-found.err.golden` = `hedgehug: no view named "木": no such view`、`bad-name.err.golden` に `contains one of`、`tree-sort.err.golden` に `a tree view orders tasks by rank`、`case.err.golden` に `already exists as "Release"`

- [ ] **Step 5: テストを通す**

Run: `go test -race ./cmd/hedgehug/ -run TestViewCommands`
Expected: PASS

- [ ] **Step 6: 全体の確認とコミット**

```bash
go test -race ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...
git add cmd/hedgehug/view.go cmd/hedgehug/view_test.go cmd/hedgehug/app.go cmd/hedgehug/testdata/TestViewCommands
git commit -m "feat(cli): hh view save/list/show/rm/rename を足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: doctor の検査

**Files:**
- Modify: `internal/doctor/doctor.go`（`fileRoots`、`checkFile` の `views`、`Run` の 5 の後に `checkViews`）
- Modify: `internal/doctor/finding.go`（`checkOrder`）
- Test: `internal/doctor/doctor_test.go`（追加）

**Interfaces:**
- Consumes: `view.ValidName`、`view.Decode`、`(*View).Problems`、`(*View).MissingColumns`、`view.List`、`model.IsViewPath`（Task 1）、`SchemaFile`（既存）

Finding:
- 形式（`Check: "file"`, `Where: views/<file>`、Hint は既存の `file` と同じ `fix the file by hand and commit, or restore it with ...`）
  - `views/` の下のサブディレクトリのファイル、`.yaml` 以外: `is not a view: views holds <name>.yaml files only`
  - 名前の規則: `the name is not a valid view name: <ValidName のエラー>`
  - 読めない: `does not decode: <err>`
  - 範囲外の値: `Problems()` の1つごとに1件（Msg はその文）
  - 大文字小文字だけが違う2つ: 両方に `has the same name as views/<other>.yaml but for case; rename one of them with hedgehug view rename`
- 列（`Check: "view"`, `Where: views/<file>`）: `column "<c>" is not a field or a property of the schema; the GUI skips it`、Hint `change the columns with hedgehug view save <name> --columns ...`。スキーマが読めるときだけ

- [ ] **Step 1: 失敗するテストを書く**

`internal/doctor/doctor_test.go` に追加（`newStore` は helpers_test.go の既存の helper）:

```go
func TestDoctorViews(t *testing.T) {
	s, dir := newStore(t)
	files := map[string]string{
		"views/ok.yaml":        "layout: table\ntable:\n  columns: [id, due]\n",
		"views/cols.yaml":      "layout: table\ntable:\n  columns: [id, gone]\n",
		"views/bad.yaml":       "layout: gantt\n",
		"views/broken.yaml":    "- x\n",
		"views/CON.yaml":       "layout: tree\n",
		"views/Same.yaml":      "layout: tree\n",
		"views/same.yaml":      "layout: tree\n",
		"views/sub/x.yaml":     "layout: tree\n",
		"views/notes.txt":      "x",
	}
	sensitive := caseSensitive(t, dir)
	if !sensitive {
		delete(files, "views/same.yaml")
	}
	if _, err := s.Update(store.Op{Name: "test", Author: gitrepo.Signature{Name: "a", Email: "a@x"}}, func(tx *store.Tx) error {
		for rel, data := range files {
			if err := tx.PutFile(rel, []byte(data)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	fs, err := Run(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, f := range fs {
		if strings.HasPrefix(f.Where, "views/") {
			got[f.Check+" "+f.Where] = append(got[f.Check+" "+f.Where], f.Msg)
		}
	}
	want := map[string][]string{
		"view views/cols.yaml":   {`column "gone" is not a field or a property of the schema; the GUI skips it`},
		"file views/bad.yaml":    {`layout "gantt" is not one of tree, kanban, table`},
		"file views/broken.yaml": nil, // checked by prefix below
		"file views/CON.yaml":    nil,
		"file views/Same.yaml":   {"has the same name as views/same.yaml but for case; rename one of them with hedgehug view rename"},
		"file views/same.yaml":   {"has the same name as views/Same.yaml but for case; rename one of them with hedgehug view rename"},
		"file views/sub/x.yaml":  {"is not a view: views holds <name>.yaml files only"},
		"file views/notes.txt":   {"is not a view: views holds <name>.yaml files only"},
	}
	if !sensitive {
		delete(want, "file views/Same.yaml")
		delete(want, "file views/same.yaml")
	}
	if len(got) != len(want) {
		t.Errorf("findings = %v", got)
	}
	for k, w := range want {
		g, ok := got[k]
		switch {
		case !ok:
			t.Errorf("no finding %s (all: %v)", k, got)
		case k == "file views/broken.yaml":
			if len(g) != 1 || !strings.HasPrefix(g[0], "does not decode: ") {
				t.Errorf("%s = %q", k, g)
			}
		case k == "file views/CON.yaml":
			if len(g) != 1 || !strings.HasPrefix(g[0], "the name is not a valid view name: ") {
				t.Errorf("%s = %q", k, g)
			}
		case !slices.Equal(g, w):
			t.Errorf("%s = %q; want %q", k, g, w)
		}
	}
}
```

同じファイルに helper を足す:

```go
// caseSensitive reports whether the file system of dir tells names apart by
// case. Windows and macOS do not, and cannot hold Same.yaml and same.yaml.
func caseSensitive(t *testing.T, dir string) bool {
	t.Helper()
	p := filepath.Join(dir, ".hedgehug", "local", "caseprobe")
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := os.Stat(filepath.Join(filepath.Dir(p), "CASEPROBE"))
	return err != nil
}
```

（Windows では `CON.yaml` を作れないので、`runtime.GOOS == "windows"` なら `files` と `want` から `views/CON.yaml` を外す。import に `os`・`path/filepath`・`runtime`・`slices`・`strings`・`gitrepo`・`store` を足す。`store.Op` に `Author` が要るかは `helpers_test.go` の `put` の書き方に合わせる。`.hedgehug/local` は gitignore の対象なので worktree の検査に出ない。）

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/doctor/ -run TestDoctorViews`
Expected: FAIL（views の finding がない）

- [ ] **Step 3: 実装する**

`internal/doctor/finding.go`:

```go
var checkOrder = []string{"schema", "template", "prop_name", "override", "task", "file", "view", "worktree", "merge_driver", "inbox", "git"}
```

`internal/doctor/doctor.go`:

```go
var fileRoots = []string{"tasks", "comments", "worklogs", "sprints", "releases", "templates", "workspaces", "views"}
```

`checkFile` の `switch root` に:

```go
	case "views":
		name, ok := model.IsViewPath(rel)
		if !ok {
			return bad("is not a view: views holds <name>.yaml files only")
		}
		if err := view.ValidName(name); err != nil {
			return bad("the name is not a valid view name: %v", err)
		}
		v, err := view.Decode(name, data)
		if err != nil {
			return bad("does not decode: %v", err)
		}
		if ps := v.Problems(); len(ps) > 0 {
			return bad("%s", ps[0])
		}
```

`checkFile` は1ファイル1件しか返さないので、`Problems()` が複数あるときの残りは `checkViews` で出す。`checkFiles` の後（`Run` の「2.」の後）に `out = append(out, checkViewNames(dir)...)` を、`Run` の「5.」の `if metaOK && schemaOK {` の中（`checkStore` の後）に `out = append(out, checkViewColumns(dir, sc)...)` を足す。`sc` は「1.」の `SchemaFile(data)` の戻り値を `Run` のローカル変数に残して使う。`Problems()` の2件目以降は `checkViewNames` で `Check: "file"` として足す。

```go
// checkViewNames finds two views whose names differ only in case, which a
// file system that folds case cannot hold apart, and the second and later
// problems of a view (checkFile reports the first).
func checkViewNames(dir string) []Finding {
	es, err := view.List(dir)
	if err != nil {
		return nil // checkFiles reports what cannot be read
	}
	var out []Finding
	hint := func(rel string) string {
		return "fix the file by hand and commit, or restore it with `git restore " + rel + "`"
	}
	for i, e := range es {
		for j, o := range es {
			if i != j && e.Name != o.Name && view.SameName(e.Name, o.Name) {
				out = append(out, Finding{Check: "file", Where: e.Rel,
					Msg: "has the same name as " + o.Rel + " but for case; rename one of them with hedgehug view rename", Hint: hint(e.Rel)})
			}
		}
		if e.View != nil {
			for _, p := range e.View.Problems()[min(1, len(e.View.Problems())):] {
				out = append(out, Finding{Check: "file", Where: e.Rel, Msg: p, Hint: hint(e.Rel)})
			}
		}
	}
	return out
}

// checkViewColumns finds the columns of views that the schema does not have.
func checkViewColumns(dir string, sc *schema.Schema) []Finding {
	es, err := view.List(dir)
	if err != nil {
		return nil
	}
	var out []Finding
	for _, e := range es {
		if e.View == nil {
			continue
		}
		for _, c := range e.View.MissingColumns(sc) {
			out = append(out, Finding{Check: "view", Where: e.Rel,
				Msg:  fmt.Sprintf("column %q is not a field or a property of the schema; the GUI skips it", c),
				Hint: "change the columns with hedgehug view save " + e.Name + " --columns ..."})
		}
	}
	return out
}
```

（`TestDoctorViews` の `bad.yaml` は Problems が1件なので `checkFile` の1件だけ。`Same.yaml`/`same.yaml` は `checkFile` では問題がなく、`checkViewNames` の1件ずつになる。期待値の数が合わなければ、この割り振りに合わせてテストではなく実装を直す。）

- [ ] **Step 4: テストを通す**

Run: `go test -race ./internal/doctor/ ./internal/ops/ ./cmd/hedgehug/ -run 'Doctor|doctor'`
Expected: PASS（既存の doctor のテスト・ゴールデンも変わらない）

- [ ] **Step 5: 全体の確認とコミット**

```bash
go test -race ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...
git add internal/doctor
git commit -m "feat(doctor): 保存済みビューの形式の誤りとスキーマにない列を検出する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: チェックリストとプロジェクトの進捗

**Files:**
- Create: `internal/index/checklist.go`, `internal/ops/progress.go`
- Modify: `internal/index/entry.go`（`Entry` に2つのフィールド、`EntryOf`）, `internal/index/build.go:164`（`cacheFormat = 6`）
- Modify: `internal/ops/view.go`（`TaskView` に `Checklist`・`Progress`）, `internal/ops/read.go`（`TreeViewOf`・`ShowViewOf`）
- Modify: `cmd/hedgehug/tree.go`（`treeLine`）, `cmd/hedgehug/show.go`（`showText`）
- Test: `internal/index/checklist_test.go`, `internal/ops/progress_test.go`, `cmd/hedgehug/tree_test.go`・`show_test.go`（追加）、更新される golden

**Interfaces:**
- Produces:
  - `index.Checklist(body string) (done, total int)`
  - `index.Entry.ChecklistDone, ChecklistTotal int`
  - `ops.Count{Done int `json:"done"`; Total int `json:"total"`}`
  - `ops.ChecklistOf(e *index.Entry) *Count`（項目がなければ nil）
  - `ops.ProgressOf(ix *index.Index, e *index.Entry) *Count`（container でなければ nil。0/0 でも container なら返す）
  - `ops.ProgressText(c, p *Count) string`（`☑ 2/5  3/14`。両方 nil なら ""）
  - `TaskView.Checklist *Count `json:"checklist,omitempty"``、`TaskView.Progress *Count `json:"progress,omitempty"``

- [ ] **Step 1: チェックリストの失敗するテストを書く**

`internal/index/checklist_test.go`:

```go
package index

import "testing"

func TestChecklist(t *testing.T) {
	for _, c := range []struct {
		name        string
		body        string
		done, total int
	}{
		{"none", "just text\n- a plain item\n", 0, 0},
		{"markers", "- [ ] a\n* [x] b\n+ [X] c\n1. [ ] d\n2) [x] e\n", 3, 5},
		{"nested", "- [ ] a\n  - [x] b\n    - [ ] c\n", 1, 3},
		{"needs a space after", "- [x]b\n-[x] c\n- [x]\n", 1, 1},
		{"not a checkbox", "- [y] a\n- [] b\n- [ x] c\n", 0, 0},
		{"fenced", "- [ ] a\n```\n- [x] in code\n```\n~~~md\n- [ ] in code\n~~~\n- [x] b\n", 1, 2},
		{"longer fence closes only on its length", "````\n```\n- [ ] in code\n````\n- [ ] a\n", 0, 1},
		{"unclosed fence hides the rest", "- [x] a\n```\n- [ ] b\n", 1, 1},
		{"crlf", "- [ ] a\r\n- [x] b\r\n", 1, 2},
	} {
		d, n := Checklist(c.body)
		if d != c.done || n != c.total {
			t.Errorf("%s: Checklist = %d/%d; want %d/%d", c.name, d, n, c.done, c.total)
		}
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/index/ -run TestChecklist`
Expected: FAIL（`undefined: Checklist`）

- [ ] **Step 3: checklist.go を書く**

```go
package index

import (
	"regexp"
	"strings"
)

// checkItem is a list item that starts with a checkbox: a bullet or a
// number, a space, [ ], [x] or [X], then a space or the end of the line.
var checkItem = regexp.MustCompile(`^[ \t]*(?:[-*+]|[0-9]{1,9}[.)])[ \t]+\[([ xX])\](?:[ \t]|$)`)

// fence opens or closes a fenced code block: three or more backticks or
// tildes after at most three spaces.
var fence = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")

// Checklist counts the checkbox items of a Markdown body, and those that are
// checked. Items inside fenced code blocks are not counted. Indented code
// blocks are not told apart from nested lists, and are counted.
func Checklist(body string) (done, total int) {
	var open string // the fence of the code block we are in, or ""
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if m := fence.FindStringSubmatch(line); m != nil {
			f := m[1]
			switch {
			case open == "":
				open = f
			case f[0] == open[0] && len(f) >= len(open) && strings.TrimSpace(line[len(m[0]):]) == "":
				open = ""
			}
			continue
		}
		if open != "" {
			continue
		}
		if m := checkItem.FindStringSubmatch(line); m != nil {
			total++
			if m[1] != " " {
				done++
			}
		}
	}
	return done, total
}
```

- [ ] **Step 4: Entry に持たせる**

`internal/index/entry.go` の `Entry` の `Refs` の後に:

```go
	ChecklistDone  int // the checked items of the body's checklist
	ChecklistTotal int // the items of the body's checklist; 0 when it has none
```

`EntryOf` の `Refs: TaskRefs(t.Body),` の後で:

```go
	e.ChecklistDone, e.ChecklistTotal = Checklist(t.Body)
```

（構造体リテラルの外で代入する。）`internal/index/build.go:164` を `cacheFormat = 6       // bump when Entry or cache changes` にする。`internal/index/build_test.go` か `cache_test.go` に、Entry のフィールドを並べて確かめるテストがあれば、新しいフィールドを足す。

`internal/index/index_test.go`（または適切な既存テスト）に、本文に `- [x] a\n- [ ] b\n` を持つタスクを書いて `Open` し、`e.ChecklistDone == 1 && e.ChecklistTotal == 2` を確かめるテストを足す。キャッシュから読み直した2回目の `Open` でも同じ値になることも確かめる。

- [ ] **Step 5: 進捗の失敗するテストを書く**

`internal/ops/progress_test.go`:

```go
package ops

import (
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

func TestProgressOf(t *testing.T) {
	ts := newTestSession(t)
	p := ts.addTask("P", func(x *model.Task) { x.Kind = "project" })
	e := ts.addTask("E", func(x *model.Task) { x.Kind = "epic"; x.Parent = &p.ULID; x.Status = "done" })
	ts.addTask("a", func(x *model.Task) { x.Parent = &e.ULID; x.Status = "done" })
	ts.addTask("b", func(x *model.Task) { x.Parent = &e.ULID; x.Status = "in_progress" })
	ts.addTask("c", func(x *model.Task) { x.Parent = &p.ULID; x.Status = "canceled" })
	d := ts.addTask("d", func(x *model.Task) { x.Parent = &p.ULID; x.Status = "done" })
	ts.addTask("gone", func(x *model.Task) {
		x.Parent = &p.ULID
		now := ts.Now()
		x.DeletedAt = &now
	})
	ts.addTask("sub", func(x *model.Task) { x.Parent = &d.ULID; x.Kind = "task"; x.Status = "todo" })
	empty := ts.addTask("Empty", func(x *model.Task) { x.Kind = "project" })
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	get := func(u model.ULID) *Count {
		en, _ := ix.Task(u)
		return ProgressOf(ix, en)
	}
	// work descendants of P: a (done), b, c (canceled, left out), d (done), sub; gone is deleted
	if got := get(p.ULID); got == nil || *got != (Count{Done: 2, Total: 4}) {
		t.Errorf("P = %+v; want 2/4", got)
	}
	if got := get(e.ULID); got == nil || *got != (Count{Done: 1, Total: 2}) {
		t.Errorf("E = %+v; want 1/2", got)
	}
	if got := get(empty.ULID); got == nil || *got != (Count{}) {
		t.Errorf("Empty = %+v; want 0/0", got)
	}
	if got := get(d.ULID); got != nil {
		t.Errorf("a task of the work role has no progress: %+v", got)
	}
}

func TestChecklistOfAndText(t *testing.T) {
	ts := newTestSession(t)
	x := ts.addTask("x", func(x *model.Task) { x.Body = "- [x] a\n- [ ] b\n" })
	y := ts.addTask("y", nil)
	_, ix, _ := ts.Open()
	ex, _ := ix.Task(x.ULID)
	ey, _ := ix.Task(y.ULID)
	if got := ChecklistOf(ex); got == nil || *got != (Count{Done: 1, Total: 2}) {
		t.Errorf("x = %+v", got)
	}
	if got := ChecklistOf(ey); got != nil {
		t.Errorf("y = %+v; want nil", got)
	}
	for _, c := range []struct {
		c, p *Count
		want string
	}{
		{nil, nil, ""},
		{&Count{1, 2}, nil, "☑ 1/2"},
		{nil, &Count{3, 14}, "3/14"},
		{&Count{2, 5}, &Count{3, 14}, "☑ 2/5  3/14"},
	} {
		if got := ProgressText(c.c, c.p); got != c.want {
			t.Errorf("ProgressText(%v, %v) = %q; want %q", c.c, c.p, got, c.want)
		}
	}
}
```

（`addTask` は Workspace を設定する。親が別の種類でも `Parents` の制約に当たらない組み合わせにしてある: project の子に epic・task、epic の子に task、task の子に task。`gone` を `DeletedAt` 付きで書けないとき（store が拒否するとき）は、`ts.Remove` 相当の既存の削除操作で消す。）

- [ ] **Step 6: progress.go を書く**

```go
package ops

import (
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// Count is how many of a whole are done.
type Count struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// ChecklistOf is the checklist of the body of e, or nil when it has none.
func ChecklistOf(e *index.Entry) *Count {
	if e.ChecklistTotal == 0 {
		return nil
	}
	return &Count{Done: e.ChecklistDone, Total: e.ChecklistTotal}
}

// ProgressOf is the progress of a project: of the tasks of the work role
// under e that are not deleted, those in the done category over those not
// in the canceled one. It is nil unless e is of the container role.
func ProgressOf(ix *index.Index, e *index.Entry) *Count {
	sc := ix.Schema()
	if k, ok := sc.Kind(e.Kind); !ok || k.Role != schema.RoleContainer {
		return nil
	}
	c := &Count{}
	seen := map[model.ULID]bool{e.ULID: true}
	var walk func(u model.ULID)
	walk = func(u model.ULID) {
		for _, ch := range ix.Children(u) {
			if seen[ch.ULID] {
				continue
			}
			seen[ch.ULID] = true
			walk(ch.ULID)
			if ch.DeletedAt != nil {
				continue
			}
			if k, ok := sc.Kind(ch.Kind); !ok || k.Role != schema.RoleWork {
				continue
			}
			st, _ := sc.Status(ch.Status)
			switch st.Category {
			case schema.CategoryCanceled:
			case schema.CategoryDone:
				c.Done++
				c.Total++
			default:
				c.Total++
			}
		}
	}
	walk(e.ULID)
	return c
}

// ProgressText is the checklist and the progress as the CLI writes them:
// "☑ 2/5  3/14", either alone, or "" with neither.
func ProgressText(checklist, progress *Count) string {
	var parts []string
	if checklist != nil {
		parts = append(parts, fmt.Sprintf("☑ %d/%d", checklist.Done, checklist.Total))
	}
	if progress != nil {
		parts = append(parts, fmt.Sprintf("%d/%d", progress.Done, progress.Total))
	}
	return strings.Join(parts, "  ")
}
```

- [ ] **Step 7: JSON に入れる**

`internal/ops/view.go` の `TaskView` の `Broken` の後に:

```go
	// Only in ls --tree --json and show --json.
	Checklist *Count `json:"checklist,omitempty"`
	Progress  *Count `json:"progress,omitempty"`
```

（「Only in ls --tree --json.」の3つより前に置く。）

`internal/ops/read.go`:

```go
func TreeViewOf(ix *index.Index, n *TreeNode) *TaskView {
	j := TaskViewOf(ix, n.Entry)
	j.Context, j.InCycle = n.Context, n.InCycle
	j.Checklist, j.Progress = ChecklistOf(n.Entry), ProgressOf(ix, n.Entry)
	// ...（children は今のまま）
}
```

`ShowViewOf` の `j := &ShowView{...}` の直後に `j.Checklist, j.Progress = ChecklistOf(r.Entry), ProgressOf(r.Index, r.Entry)`。

- [ ] **Step 8: CLI のテキストに入れる**

`cmd/hedgehug/tree.go` の `treeLine` は index しか受け取らないので、引数に `ix *index.Index` を足し、`writeNode` からも渡す（`lsTree` の `ix` を `writeNode` の引数で回す）:

```go
// treeLine is one task of the tree: id and ULID, status, kind and title,
// then the checklist and the progress of a project.
func treeLine(st style, ix *index.Index, e *index.Entry, context bool) string {
	r := taskRow(e)
	name := r[0] + " " + r[1]
	line := name + "  " + r[2] + "  " + r[3] + "  " + r[6]
	if p := ops.ProgressText(ops.ChecklistOf(e), ops.ProgressOf(ix, e)); p != "" {
		line += "  " + p
	}
	if context {
		return st.dim(line)
	}
	return st.id(name) + line[len(name):]
}
```

`cmd/hedgehug/show.go` の `showText` の `b.WriteString(title + "\n")` の直後:

```go
	if p := ops.ProgressText(ops.ChecklistOf(e), ops.ProgressOf(ix, e)); p != "" {
		b.WriteString("  " + p + "\n")
	}
```

- [ ] **Step 9: CLI のテストを足す**

`cmd/hedgehug/tree_test.go` の `TestLsTree` の後に:

```go
func TestLsTreeProgress(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("Project", "--kind", "project")
	a := te.addJSON("Alpha", "--parent", p.ID, "--status", "done")
	te.addJSON("Beta", "--parent", p.ID)
	te.addJSON("Gamma", "--parent", p.ID, "--status", "canceled")
	te.editWith(te.raw(a.ULID) + "- [x] one\n- [ ] two\n")
	te.ok("edit", a.ID)
	te.goldenOK("tree", "ls", "--tree")
	te.goldenOK("tree-json", "ls", "--tree", "--json")
	te.goldenOK("show", "show", p.ID)
	te.goldenOK("show-a", "show", a.ID)
	te.goldenOK("show-json", "show", a.ID, "--json")
}
```

（`te.raw` はタスクのファイルの中身。末尾に本文の行を足したものをエディタが保存する。）

golden の中身を確かめる: `tree.golden` の Project の行の末尾が `  1/2`、Alpha の行の末尾が `  ☑ 1/2`。`show.golden` の2行目が `  1/2`、`show-a.golden` の2行目が `  ☑ 1/2`。`tree-json.golden` の Project に `"progress": {"done": 1, "total": 2}`、Alpha に `"checklist": {"done": 1, "total": 2}`、`show-json.golden` に `"checklist"`。

- [ ] **Step 10: 既存の golden を更新し、差分を確かめる**

```bash
go test ./cmd/hedgehug/ -update
git diff --stat -- cmd/hedgehug/testdata
git diff -- cmd/hedgehug/testdata | grep '^[-+][^-+]' | grep -v '☑\|  [0-9]*/[0-9]*$\|"checklist"\|"progress"\|"done"\|"total"\|^[-+] *}' 
```

Expected: 最後のコマンドの出力が空（進捗の行と JSON の `checklist`・`progress` 以外に変わった行がない）。空でなければ、その golden を `git checkout -- <file>` で戻し、原因（`-update` で無関係に書き換わったもの）を調べて直す。`TestViewCommands` の `show-tree.golden` は Project の行に `  1/2` が付くので更新されてよい。

- [ ] **Step 11: テストを通す**

Run: `go test -race ./internal/index/ ./internal/ops/ ./cmd/hedgehug/`
Expected: PASS

- [ ] **Step 12: 全体の確認とコミット**

```bash
go test -race ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...
git add internal/index internal/ops cmd/hedgehug
git commit -m "feat: チェックリストとプロジェクトの進捗を ls --tree と show に出す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: 仕上げ

**Files:**
- Modify: なし（見つかったものはこのタスクで直す）

- [ ] **Step 1: `make check` を通す**

Run: `make check`
Expected: PASS（tidy・lint・fmt・test）

- [ ] **Step 2: spec の 4b の完了条件を CLI で確かめる**

一時ディレクトリの `HEDGEHUG_HOME` で `make build` の `bin/hedgehug` を使い、次を手で実行して出力を確かめる: `store init` → `add`（project と子）→ `view save/list/show/rename/rm` → `ls --tree` と `show` に進捗 → `doctor` がスキーマにない列を警告。2つのクローンで同じビューの別のキーを変えて `sync` し、キー単位でマージされること。

- [ ] **Step 3: spec の各項目と突き合わせる**

spec の「4b 保存済みビュー」の各箇条（形式、名前、省略時の既定値、columns、未知のキー、書き込みの件名と trailer、マージ、doctor、CLI の5つのコマンドと `--store`・`--json`、進捗の3項目）について、実装とテストの場所を一覧にし、抜けがあればここで実装する。

- [ ] **Step 4: 残りの差分があればコミット**

```bash
git status --short
```

差分があれば内容に合った件名でコミットする。
