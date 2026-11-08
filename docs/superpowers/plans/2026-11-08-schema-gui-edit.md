# 画面からスキーマを編集する Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** スキーマ画面から、ステータス・種類・プロパティ・関連・enum の値を追加・削除・変更・並べ替えできるようにし、色とアイコンを全画面の表示に使う。

**Architecture:** ドメイン（`internal/app/domain/schema/edit.go`）に、スキーマの複製を返す純粋な編集関数を置く。usecase（`internal/app/usecase/schemas/edit.go`）が Store のロックの中で編集を当て、使用中の判定とタスクの検証をして 1 コミットで書く。サーバーは定義の種類ごとの REST を足し、フロントは生成クライアントを通して、タブごとに分けたスキーマ画面のダイアログから呼ぶ。

**Tech Stack:** Go（huma v2、go.yaml.in/yaml/v4）、Svelte 5、Orval、@atlaskit/pragmatic-drag-and-drop、Vitest、Playwright

**Spec:** `docs/superpowers/specs/2026-11-08-schema-gui-edit-design.md`

## Global Constraints

- 1 操作 1 コミット。op 名は `schema`（`store.Op{Name: "schema"}`）
- 使用中の削除は 422 `schema.in_use`。数えるのは Store のすべてのタスク（削除済みを含む）
- 既存タスクが新たに違反する変更は 422 `schema.tasks_invalid`（もとからの違反は数えない）
- 置ける親が空になる削除は 422 `schema.parents_empty`
- 並べ替えの名前集合の不一致は 409 `schema.order_stale`
- 書いたあとのスキーマに新しい `Check()` の問題があれば 422 `schema.invalid`
- 名前のない定義は 404 `schema.not_defined`
- 型は変えられない。名前は migrate で変える。`blocked_by` は対象外
- 色の値: `gray`・`blue`・`green`・`amber`・`red`・`teal`・`orange`・`purple`・`sky`・`pink`・`olive`・`brown`。アイコンの値: `project`・`epic`・`story`・`task`・`bug`
- CLI は足さない。画面の文言はすべて日本語
- 全テスト（`make check` 相当）は仕上げに 1 回。途中は対象のパッケージだけを回す
- spec と違う挙動にしたくなったら、実装者は止めて報告する

## この plan で埋める spec にない細部

1. 新しい `Check()` の問題かどうかは、`Path` ではなく `Code` と `Args` で比べる（`AddedProblems`）。削除や並べ替えで添字が動くと、もとからあった問題が新しいものに見えるため。scoped の `NewProblems` は変えない。
2. 種類の `PATCH` で「どこにでも」は `anywhere: true` で送る。`parents: []` は 400。spec が親のない状態を禁じているため、空の配列に意味を持たせない。
3. container の種類を既定にする要求は 422 `schema.default_container`。
4. 関連以外のプロパティの `PATCH` で、表示名以外（`multi`・`inverse`・`symmetric`）を送ると 422 `schema.not_task_ref`。enum でないプロパティへの値の操作は 422 `schema.not_enum`。
5. 削除で外れる参照の一覧は、フロントが `SchemaView` から計算する（`core/schema-edit.ts` の `referencesOf`）。サーバーに dry run は足さない。
6. プロパティの並びは、スキーマ画面だけでなく、プロパティを並べるすべての箇所（詳細のプロパティと関連、テーブルの列の候補、ガント、上書きの必須の候補）で `prop_order` に従う（2026-11-08、利用者が承認し spec に反映済み）。
7. `StatusIcon` と `KindIcon` は `color` / `icon` を受け取るだけにし、スキーマは引かない。呼び出し元はいまも行ごとにスキーマから `category` / `role` を引いている（`TreeRow.svelte:51-52` など）ので、同じ箇所で `color` / `icon` も引いて渡す。アイコンに context を読ませると、App の外で描くテストで `core()` が投げ、1 万行のツリーで購読が行の数だけ増えるため。Store をまたぐ列（カテゴリでそろえたカンバンの列など）は渡さず、今までどおりカテゴリの色で描く。

---

## ファイル

- 作成: `internal/app/domain/schema/edit.go` 編集の関数
- 作成: `internal/app/domain/schema/edit_test.go`
- 作成: `internal/app/domain/schema/testdata/edit-notes.golden.yaml`
- 作成: `internal/app/usecase/schemas/edit.go` 書き込み
- 作成: `internal/app/usecase/schemas/edit_test.go`
- 修正: `internal/app/usecase/schemas/schemaview.go` `prop_order`
- 作成: `internal/app/server/schemaedit.go`、`internal/app/server/schemaedit_test.go`
- 修正: `internal/app/server/server.go`（登録）、`api/openapi.yaml`、`web/core/api/generated.ts`
- 作成: `web/core/schema-edit.ts`、`web/core/schema-edit.test.ts`
- 修正: `web/app/icons/StatusIcon.svelte`、`web/app/icons/KindIcon.svelte` と呼び出し元 16 ファイル
- 修正: プロパティを並べる箇所（`web/app/table/columns.ts`、`web/app/detail/relation-groups.ts`、`web/app/detail/prop-rows.ts`、`web/app/gantt/GanttView.svelte`、`web/core/scoped.ts`）
- 分割: `web/app/settings/SettingsSchema.svelte` → `SchemaStatuses.svelte`・`SchemaKinds.svelte`・`SchemaProps.svelte`・`SchemaRelations.svelte`・`SchemaWorkflow.svelte`
- 作成: `web/app/settings/SchemaDefDialog.svelte`、`SchemaDeleteDialog.svelte`、`SchemaValueDialog.svelte`、`ColorPicker.svelte`、`IconPicker.svelte`、`reorder.ts`
- 修正: `web/e2e/` にスキーマ編集の E2E
- 修正: `docs/superpowers/specs/2026-11-01-phase4-gui-design.md`

---

### Task 1: ドメインの編集関数

**Files:**
- Create: `internal/app/domain/schema/edit.go`
- Test: `internal/app/domain/schema/edit_test.go`、`internal/app/domain/schema/testdata/edit-notes.golden.yaml`

**Interfaces:**
- Produces（Task 2 が使う）:

```go
var (
	ErrNotDefined = errors.New("not defined")       // errors.Is で判定。メッセージに名前を入れて wrap する
	ErrOrderStale = errors.New("the names differ from the schema")
	ErrDefaultContainer = errors.New("only a kind of the work role can be the default")
	ErrNotTaskRef = errors.New("only a task_ref property has multi, inverse and symmetric")
	ErrNotEnum    = errors.New("not an enum or multi_enum property")
)
type ParentsEmptyError struct{ Kinds []string }    // Error(): "kinds left with no parent: a, b"

type StatusChange struct {
	Label, Color *string    // "" で消す
	Category     *Category
	Default      bool       // true だけ意味を持つ
}
type KindChange struct {
	Label, Icon, Template *string
	Role                  *Role
	Parents               *[]string // nil: 変えない。&nil: どこにでも
	Default               bool
}
type PropChange struct {
	Label, Inverse *string
	Multi, Symmetric *bool
}

func (s *Schema) AddStatus(st Status) *Schema
func (s *Schema) ChangeStatus(name string, ch StatusChange) (*Schema, error)
func (s *Schema) RemoveStatus(name string) (*Schema, error)
func (s *Schema) OrderStatuses(names []string) (*Schema, error)
func (s *Schema) AddKind(k Kind) *Schema
func (s *Schema) ChangeKind(name string, ch KindChange) (*Schema, error)
func (s *Schema) RemoveKind(name string) (*Schema, error)
func (s *Schema) OrderKinds(names []string) (*Schema, error)
func (s *Schema) AddProp(p Prop) *Schema
func (s *Schema) ChangeProp(name string, ch PropChange) (*Schema, error)
func (s *Schema) RemoveProp(name string) (*Schema, error)
func (s *Schema) OrderProps(names []string) (*Schema, error) // task_ref の全体か、それ以外の全体
func (s *Schema) AddValue(prop, value string) (*Schema, error)
func (s *Schema) RemoveValue(prop, value string) (*Schema, error)
func (s *Schema) OrderValues(prop string, names []string) (*Schema, error)
func (s *Schema) AddedProblems(before []Problem) []Problem
func TaskUses(t *model.Task, target Target, prop, name string) bool
```

どの関数も `s.clone()` に当てた結果を返し、`s` は変えない（`Migrate` と同じく、結果は `s` のファイルを共有する）。

- [ ] **Step 1: 失敗するテストを書く**

`edit_test.go`（package `schema`）。`scopedBase(t, extra)` を使って Default に上書きを足したスキーマを作る。

```go
func TestRemoveStatusDropsReferences(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, `workflow:
  transitions:
    backlog: [todo, on_hold]
    on_hold: [todo]
    waiting: [on_hold]
  required:
    on_hold: [assignees]
scoped:
- where: project:`+scopedULID+`
  statuses: [todo, on_hold, in_progress, done, canceled]
`)
	got, err := s.RemoveStatus("on_hold")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Status("on_hold"); ok {
		t.Error("on_hold is still defined")
	}
	tr := got.Workflow.Transitions
	if v, _ := tr.Get("backlog"); !slices.Equal(v, []string{"todo"}) {
		t.Errorf("backlog = %v", v)
	}
	if _, ok := tr.Get("on_hold"); ok {
		t.Error("the key on_hold stays")
	}
	if _, ok := tr.Get("waiting"); ok {
		t.Error("waiting, left with no target, stays")
	}
	if _, ok := got.Workflow.Required.Get("on_hold"); ok {
		t.Error("required on_hold stays")
	}
	if !slices.Equal(got.Scoped[0].Statuses, []string{"todo", "in_progress", "done", "canceled"}) {
		t.Errorf("scoped statuses = %v", got.Scoped[0].Statuses)
	}
	if _, ok := s.Status("on_hold"); !ok {
		t.Error("s was changed")
	}
}

func TestRemoveKindRefusesAKindLeftWithNoParent(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, "")
	s.Kinds = append(s.Kinds, Kind{Name: "sub", Role: RoleWork, Parents: []string{"epic"}})
	_, err := s.RemoveKind("epic")
	var pe *ParentsEmptyError
	if !errors.As(err, &pe) || !slices.Equal(pe.Kinds, []string{"sub"}) {
		t.Fatalf("err = %v", err)
	}
}

func TestRemoveKindDropsParentsAndTemplateDefaults(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, `scoped:
- where: project:`+scopedULID+`
  templates:
    default: {epic: plan, task: work}
`)
	got, err := s.RemoveKind("epic")
	if err != nil {
		t.Fatal(err)
	}
	story, _ := got.Kind("story")
	if !slices.Equal(story.Parents, []string{Root, "project"}) {
		t.Errorf("story parents = %v", story.Parents)
	}
	if _, ok := got.Scoped[0].Templates.Default["epic"]; ok {
		t.Error("the default template of epic stays")
	}
}

func TestRemovePropDropsRequired(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, "workflow:\n  required:\n    done: [points]\n    in_progress: [points, assignees]\n")
	got, err := s.RemoveProp("points")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Workflow.Required.Get("done"); ok {
		t.Error("done, left with nothing, stays")
	}
	if v, _ := got.Workflow.Required.Get("in_progress"); !slices.Equal(v, []string{"assignees"}) {
		t.Errorf("in_progress = %v", v)
	}
}

func TestChangeStatusDefaultMovesIt(t *testing.T) {
	t.Parallel()
	got, err := Default().ChangeStatus("backlog", StatusChange{Default: true})
	if err != nil {
		t.Fatal(err)
	}
	if d, _ := got.DefaultStatus(CategoryTodo); d.Name != "backlog" {
		t.Errorf("default of todo = %s", d.Name)
	}
	if ps := got.Check(); len(ps) > 0 {
		t.Errorf("Check = %v", ps)
	}
}

func TestChangeKindDefaultRefusesAContainer(t *testing.T) {
	t.Parallel()
	if _, err := Default().ChangeKind("epic", KindChange{Default: true}); !errors.Is(err, ErrDefaultContainer) {
		t.Errorf("err = %v", err)
	}
}

func TestChangeKindParentsAnywhere(t *testing.T) {
	t.Parallel()
	var anywhere []string
	got, err := Default().ChangeKind("bug", KindChange{Parents: &anywhere})
	if err != nil {
		t.Fatal(err)
	}
	if k, _ := got.Kind("bug"); k.Parents != nil {
		t.Errorf("parents = %v", k.Parents)
	}
}

func TestChangePropRefusesTaskRefFieldsOnOthers(t *testing.T) {
	t.Parallel()
	multi := true
	if _, err := Default().ChangeProp("priority", PropChange{Multi: &multi}); !errors.Is(err, ErrNotTaskRef) {
		t.Errorf("err = %v", err)
	}
}

func TestOrderProps(t *testing.T) {
	t.Parallel()
	s := Default() // priority start due estimate points | duplicates causes relates
	got, err := s.OrderProps([]string{"relates", "duplicates", "causes"})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range got.Props {
		names = append(names, p.Name)
	}
	want := []string{"priority", "start", "due", "estimate", "points", "relates", "duplicates", "causes"}
	if !slices.Equal(names, want) {
		t.Errorf("props = %v", names)
	}
	if _, err := s.OrderProps([]string{"relates", "duplicates"}); !errors.Is(err, ErrOrderStale) {
		t.Errorf("a missing name: err = %v", err)
	}
	if _, err := s.OrderProps([]string{"priority", "relates", "duplicates", "causes"}); !errors.Is(err, ErrOrderStale) {
		t.Errorf("a mix of the two groups: err = %v", err)
	}
}

func TestOrderStatusesStale(t *testing.T) {
	t.Parallel()
	if _, err := Default().OrderStatuses([]string{"todo", "done"}); !errors.Is(err, ErrOrderStale) {
		t.Errorf("err = %v", err)
	}
}

func TestValues(t *testing.T) {
	t.Parallel()
	s, err := Default().AddValue("priority", "none")
	if err != nil {
		t.Fatal(err)
	}
	if s, err = s.OrderValues("priority", []string{"none", "low", "mid", "high", "urgent"}); err != nil {
		t.Fatal(err)
	}
	if s, err = s.RemoveValue("priority", "urgent"); err != nil {
		t.Fatal(err)
	}
	p, _ := s.Prop("priority")
	if !slices.Equal(p.Values, []string{"none", "low", "mid", "high"}) {
		t.Errorf("values = %v", p.Values)
	}
	if _, err := s.AddValue("start", "x"); !errors.Is(err, ErrNotEnum) {
		t.Errorf("err = %v", err)
	}
	if _, err := s.RemoveValue("priority", "nope"); !errors.Is(err, ErrNotDefined) {
		t.Errorf("err = %v", err)
	}
}

func TestNotDefined(t *testing.T) {
	t.Parallel()
	s := Default()
	for name, err := range map[string]error{
		"status": func() error { _, err := s.RemoveStatus("x"); return err }(),
		"kind":   func() error { _, err := s.ChangeKind("x", KindChange{}); return err }(),
		"prop":   func() error { _, err := s.RemoveProp("x"); return err }(),
	} {
		if !errors.Is(err, ErrNotDefined) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestAddedProblemsIgnoresThePath(t *testing.T) {
	t.Parallel()
	s := Default()
	s.Statuses = append(s.Statuses, Status{Name: "todo", Category: CategoryTodo}) // status_twice at statuses[7]
	before := s.Check()
	got, err := s.RemoveStatus("backlog") // the second todo moves to statuses[6]
	if err != nil {
		t.Fatal(err)
	}
	if ps := got.AddedProblems(before); len(ps) > 0 {
		t.Errorf("AddedProblems = %v", ps)
	}
	if ps := got.NewProblems(before); len(ps) == 0 {
		t.Error("NewProblems, which compares the paths, finds nothing: the test does not shift the index")
	}
	if _, err := s.OrderStatuses([]string{"backlog", "todo", "on_hold", "in_progress", "waiting", "done", "canceled", "todo"}); !errors.Is(err, ErrOrderStale) {
		t.Errorf("a schema with a name twice is ordered: err = %v", err)
	}
}

func TestTaskUses(t *testing.T) {
	t.Parallel()
	var n yaml.Node
	if err := n.Encode([]string{"backend", "infra"}); err != nil {
		t.Fatal(err)
	}
	tk := &model.Task{Status: "todo", Kind: "task", Props: model.Fields{{Key: "area", Value: &n}}}
	cases := []struct {
		target     Target
		prop, name string
		want       bool
	}{
		{TargetStatus, "", "todo", true},
		{TargetStatus, "", "done", false},
		{TargetKind, "", "task", true},
		{TargetProp, "", "area", true},
		{TargetProp, "", "points", false},
		{TargetValue, "area", "infra", true},
		{TargetValue, "area", "ops", false},
	}
	for _, c := range cases {
		if got := TaskUses(tk, c.target, c.prop, c.name); got != c.want {
			t.Errorf("TaskUses(%s %s %s) = %v", c.target, c.prop, c.name, got)
		}
	}
}

func TestEditKeepsComments(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("testdata/notes.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	s, err = s.OrderStatuses([]string{"done", "todo", "doing"})
	if err != nil {
		t.Fatal(err)
	}
	s = s.AddStatus(Status{Name: "review", Category: CategoryDoing, Color: "teal"})
	if s, err = s.ChangeStatus("done", StatusChange{Label: ptr("完了")}); err != nil {
		t.Fatal(err)
	}
	if s, err = s.OrderProps([]string{"points", "priority"}); err != nil {
		t.Fatal(err)
	}
	out, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "testdata/edit-notes.golden.yaml", out)
}

func ptr[T any](v T) *T { return &v }
```

`golden` は、このパッケージにゴールデンの補助があればそれを使う（`grep -n "func golden\|update" internal/app/domain/schema/*_test.go` で確かめる）。なければ `edit_test.go` に、`-update` フラグで書き出し、なければ比べるだけの補助を足す:

```go
var update = flag.Bool("update", false, "write the golden files")

func golden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
```

名前が重複したスキーマは名前の集合で並べ替えられないので、`OrderStatuses` は `ErrOrderStale` を返す（名前の重複を含む要求も、重複した定義も拒否する）。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/domain/schema/ -run 'Remove|Change|Order|Values|NotDefined|AddedProblems|TaskUses|EditKeeps'`
Expected: コンパイルエラー（未定義）

- [ ] **Step 3: `edit.go` を書く**

```go
package schema

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"go.yaml.in/yaml/v4"
)

var (
	ErrNotDefined       = errors.New("not defined")
	ErrOrderStale       = errors.New("the names differ from the schema")
	ErrDefaultContainer = errors.New("only a kind of the work role can be the default")
	ErrNotTaskRef       = errors.New("only a task_ref property has multi, inverse and symmetric")
	ErrNotEnum          = errors.New("not an enum or multi_enum property")
)

// ParentsEmptyError is a removal of a kind that would leave Kinds with no
// parent. An empty parents list is left out of the file, where it means
// "anywhere", so the removal is refused instead.
type ParentsEmptyError struct{ Kinds []string }

func (e *ParentsEmptyError) Error() string {
	return "kinds left with no parent: " + strings.Join(e.Kinds, ", ")
}

func notDefined(what, name string) error { return fmt.Errorf("%s %q: %w", what, name, ErrNotDefined) }

type StatusChange struct {
	Label, Color *string
	Category     *Category
	Default      bool
}

type KindChange struct {
	Label, Icon, Template *string
	Role                  *Role
	Parents               *[]string
	Default               bool
}

type PropChange struct {
	Label, Inverse   *string
	Multi, Symmetric *bool
}

func set[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

// AddStatus returns s with st at the end. Check finds a name that is
// empty or already defined.
func (s *Schema) AddStatus(st Status) *Schema {
	out := s.clone()
	out.Statuses = append(out.Statuses, st)
	return out
}

// ChangeStatus returns s with the status name changed as ch says. A default
// takes it from the other default of its category.
func (s *Schema) ChangeStatus(name string, ch StatusChange) (*Schema, error) {
	out := s.clone()
	i := slices.IndexFunc(out.Statuses, func(x Status) bool { return x.Name == name })
	if i < 0 {
		return nil, notDefined("status", name)
	}
	st := &out.Statuses[i]
	set(&st.Label, ch.Label)
	set(&st.Color, ch.Color)
	set(&st.Category, ch.Category)
	if ch.Default {
		for j := range out.Statuses {
			if out.Statuses[j].Category == st.Category {
				out.Statuses[j].Default = j == i
			}
		}
	}
	return out, nil
}

// RemoveStatus returns s without the status name and without the references
// to it in the workflows and the overrides. A transition left with no
// target, and a required list left with nothing, go too, as in Carry.
func (s *Schema) RemoveStatus(name string) (*Schema, error) {
	out := s.clone()
	i := slices.IndexFunc(out.Statuses, func(x Status) bool { return x.Name == name })
	if i < 0 {
		return nil, notDefined("status", name)
	}
	out.Statuses = slices.Delete(out.Statuses, i, i+1)
	for _, w := range out.workflows() {
		w.Transitions = dropKey(w.Transitions, name)
		w.Transitions = dropValue(w.Transitions, name)
		w.Required = dropKey(w.Required, name)
	}
	for j := range out.Scoped {
		if out.Scoped[j].Statuses != nil {
			out.Scoped[j].Statuses = slices.DeleteFunc(slices.Clone(out.Scoped[j].Statuses), func(x string) bool { return x == name })
		}
	}
	return out, nil
}

// dropKey is l without the key name.
func dropKey(l Lists, name string) Lists {
	return nilIfEmpty(slices.DeleteFunc(slices.Clone(l), func(x List) bool { return x.Key == name }))
}

// dropValue is l without the value name; a key that had values and is left
// with none goes.
func dropValue(l Lists, name string) Lists {
	var out Lists
	for _, x := range l {
		vs := slices.DeleteFunc(slices.Clone(x.Values), func(v string) bool { return v == name })
		if len(vs) == 0 && len(x.Values) > 0 {
			continue
		}
		out = append(out, List{x.Key, vs})
	}
	return out
}

// reorder returns xs in the order of names, which has to name each item of
// xs once.
func reorder[T any](xs []T, nameOf func(T) string, names []string) ([]T, error) {
	if len(names) != len(xs) {
		return nil, ErrOrderStale
	}
	by := make(map[string]T, len(xs))
	for _, x := range xs {
		by[nameOf(x)] = x
	}
	if len(by) != len(xs) {
		return nil, ErrOrderStale // a name defined twice
	}
	out := make([]T, 0, len(xs))
	seen := map[string]bool{}
	for _, n := range names {
		x, ok := by[n]
		if !ok || seen[n] {
			return nil, ErrOrderStale
		}
		seen[n] = true
		out = append(out, x)
	}
	return out, nil
}

func (s *Schema) OrderStatuses(names []string) (*Schema, error) {
	out := s.clone()
	var err error
	if out.Statuses, err = reorder(out.Statuses, func(x Status) string { return x.Name }, names); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Schema) AddKind(k Kind) *Schema {
	out := s.clone()
	out.Kinds = append(out.Kinds, k)
	return out
}

// ChangeKind returns s with the kind name changed as ch says. Only a kind
// of the work role can be made the default; it takes it from the other.
func (s *Schema) ChangeKind(name string, ch KindChange) (*Schema, error) {
	out := s.clone()
	i := slices.IndexFunc(out.Kinds, func(x Kind) bool { return x.Name == name })
	if i < 0 {
		return nil, notDefined("kind", name)
	}
	k := &out.Kinds[i]
	set(&k.Label, ch.Label)
	set(&k.Icon, ch.Icon)
	set(&k.Template, ch.Template)
	set(&k.Role, ch.Role)
	if ch.Parents != nil {
		k.Parents = slices.Clone(*ch.Parents)
	}
	if ch.Default {
		if k.Role != RoleWork {
			return nil, ErrDefaultContainer
		}
		for j := range out.Kinds {
			if out.Kinds[j].Role == RoleWork {
				out.Kinds[j].Default = j == i
			}
		}
	}
	return out, nil
}

// RemoveKind returns s without the kind name, without it in the parents of
// the other kinds and in the default templates of the overrides. A kind
// left with no parent is a ParentsEmptyError.
func (s *Schema) RemoveKind(name string) (*Schema, error) {
	out := s.clone()
	i := slices.IndexFunc(out.Kinds, func(x Kind) bool { return x.Name == name })
	if i < 0 {
		return nil, notDefined("kind", name)
	}
	out.Kinds = slices.Delete(out.Kinds, i, i+1)
	var empty []string
	for j := range out.Kinds {
		ps := out.Kinds[j].Parents
		if !slices.Contains(ps, name) {
			continue
		}
		ps = slices.DeleteFunc(slices.Clone(ps), func(x string) bool { return x == name })
		if len(ps) == 0 {
			empty = append(empty, out.Kinds[j].Name)
		}
		out.Kinds[j].Parents = ps
	}
	if len(empty) > 0 {
		return nil, &ParentsEmptyError{Kinds: empty}
	}
	for j := range out.Scoped {
		if t := out.Scoped[j].Templates; t != nil && t.Default != nil {
			d := maps.Clone(t.Default)
			delete(d, name)
			nt := *t
			nt.Default = d
			out.Scoped[j].Templates = &nt
		}
	}
	return out, nil
}

func (s *Schema) OrderKinds(names []string) (*Schema, error) {
	out := s.clone()
	var err error
	if out.Kinds, err = reorder(out.Kinds, func(x Kind) string { return x.Name }, names); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Schema) AddProp(p Prop) *Schema {
	out := s.clone()
	out.Props = append(out.Props, p)
	return out
}

// ChangeProp returns s with the property name changed as ch says. multi,
// inverse and symmetric are for a task_ref property only.
func (s *Schema) ChangeProp(name string, ch PropChange) (*Schema, error) {
	out := s.clone()
	i := slices.IndexFunc(out.Props, func(x Prop) bool { return x.Name == name })
	if i < 0 {
		return nil, notDefined("property", name)
	}
	p := &out.Props[i]
	if p.Type != TypeTaskRef && (ch.Inverse != nil || ch.Multi != nil || ch.Symmetric != nil) {
		return nil, ErrNotTaskRef
	}
	set(&p.Label, ch.Label)
	set(&p.Inverse, ch.Inverse)
	set(&p.Multi, ch.Multi)
	set(&p.Symmetric, ch.Symmetric)
	return out, nil
}

// RemoveProp returns s without the property name and without it in the
// required lists; a list left with nothing goes.
func (s *Schema) RemoveProp(name string) (*Schema, error) {
	out := s.clone()
	i := slices.IndexFunc(out.Props, func(x Prop) bool { return x.Name == name })
	if i < 0 {
		return nil, notDefined("property", name)
	}
	out.Props = slices.Delete(out.Props, i, i+1)
	for _, w := range out.workflows() {
		w.Required = nilIfEmpty(dropValue(w.Required, name))
	}
	return out, nil
}

// OrderProps puts the task_ref properties, or the others, in the order of
// names, which names the whole of one group. The other group keeps its
// places.
func (s *Schema) OrderProps(names []string) (*Schema, error) {
	out := s.clone()
	if len(names) == 0 {
		return nil, ErrOrderStale
	}
	first, ok := out.Prop(names[0])
	if !ok {
		return nil, ErrOrderStale
	}
	rel := first.Type == TypeTaskRef
	var slots []int
	var group Props
	for i, p := range out.Props {
		if (p.Type == TypeTaskRef) == rel {
			slots = append(slots, i)
			group = append(group, p)
		}
	}
	sorted, err := reorder(group, func(x Prop) string { return x.Name }, names)
	if err != nil {
		return nil, err
	}
	props := slices.Clone(out.Props)
	for k, i := range slots {
		props[i] = sorted[k]
	}
	out.Props = props
	return out, nil
}

// enumProp is the enum or multi_enum property prop of out, to change in place.
func enumProp(out *Schema, prop string) (*Prop, error) {
	i := slices.IndexFunc(out.Props, func(x Prop) bool { return x.Name == prop })
	if i < 0 {
		return nil, notDefined("property", prop)
	}
	p := &out.Props[i]
	if p.Type != TypeEnum && p.Type != TypeMultiEnum {
		return nil, fmt.Errorf("%s: %w", prop, ErrNotEnum)
	}
	return p, nil
}

// AddValue returns s with value at the end of the values of prop. Check
// does not look at values twice, so a value already there is refused here.
func (s *Schema) AddValue(prop, value string) (*Schema, error) {
	out := s.clone()
	p, err := enumProp(out, prop)
	if err != nil {
		return nil, err
	}
	if value == "" || slices.Contains(p.Values, value) {
		return nil, fmt.Errorf("value %q of %s is empty or already defined: %w", value, prop, ErrValueInvalid)
	}
	p.Values = append(slices.Clone(p.Values), value)
	return out, nil
}

func (s *Schema) RemoveValue(prop, value string) (*Schema, error) {
	out := s.clone()
	p, err := enumProp(out, prop)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(p.Values, value) {
		return nil, notDefined("value", prop+"."+value)
	}
	p.Values = slices.DeleteFunc(slices.Clone(p.Values), func(v string) bool { return v == value })
	return out, nil
}

func (s *Schema) OrderValues(prop string, names []string) (*Schema, error) {
	out := s.clone()
	p, err := enumProp(out, prop)
	if err != nil {
		return nil, err
	}
	if p.Values, err = reorder(p.Values, func(v string) string { return v }, names); err != nil {
		return nil, err
	}
	return out, nil
}

// AddedProblems lists the problems of s that are not in before. Problems are
// the same when their Code and Args are: a removal or a move shifts the
// indexes in Path.
func (s *Schema) AddedProblems(before []Problem) []Problem {
	var out []Problem
	for _, p := range s.Check() {
		if !slices.ContainsFunc(before, func(b Problem) bool { return b.Code == p.Code && slices.Equal(b.Args, p.Args) }) {
			out = append(out, p)
		}
	}
	return out
}

// TaskUses reports whether t uses the definition name: its status or kind,
// a value for the property, or the value name of the property prop.
func TaskUses(t *model.Task, target Target, prop, name string) bool {
	switch target {
	case TargetStatus:
		return t.Status == name
	case TargetKind:
		return t.Kind == name
	case TargetProp:
		return t.Props.Get(name) != nil
	case TargetValue:
		n := t.Props.Get(prop)
		if n == nil {
			return false
		}
		if n.Kind == yaml.ScalarNode {
			return n.Value == name
		}
		return slices.ContainsFunc(n.Content, func(x *yaml.Node) bool { return x.Kind == yaml.ScalarNode && x.Value == name })
	}
	return false
}
```

`ErrValueInvalid = errors.New("invalid value")` も `var` の組に足す（Task 2 で 422 `schema.value_invalid` にする）。

`clone()` が `Statuses`・`Kinds`・`Props`・各 `Values`・`Parents`・`Scoped` をどこまで深く写すかを `migrate.go:60-96` で確かめる。浅い部分（たとえば `Props[i].Values` を共有している）があれば、上のコードのとおり `slices.Clone` してから書き換える（`s` を変えないため）。

- [ ] **Step 4: ゴールデンを作って中身を確かめる**

Run: `go test ./internal/app/domain/schema/ -run TestEditKeepsComments -update`

`testdata/edit-notes.golden.yaml` を開き、次を目で確かめる: `# 未着手` `# 最初` `# 作業中` `# 完了` のコメントが、それぞれ todo・doing・done の項目について動いている。`x-note: keep` が残っている。props の `# 優先度` `# 選択` `# 数値` が priority・points についている。review が末尾にある。崩れていれば `mergeSeq`・`mergeMapping` の対応づけを調べ、直せないときは止めて報告する。

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test ./internal/app/domain/schema/`
Expected: PASS

- [ ] **Step 6: コミット**

```bash
git add internal/app/domain/schema/edit.go internal/app/domain/schema/edit_test.go internal/app/domain/schema/testdata/edit-notes.golden.yaml
git commit -m "feat(schema): 定義の追加・変更・削除・並べ替えを足す"
```

---

### Task 2: usecase の書き込み

**Files:**
- Create: `internal/app/usecase/schemas/edit.go`
- Modify: `internal/app/usecase/schemas/schemaview.go`（`PropOrder`）
- Test: `internal/app/usecase/schemas/edit_test.go`

**Interfaces:**
- Consumes: Task 1 の関数と誤り
- Produces（Task 3 が使う）:

```go
// EditWrite is what a schema edit returns: the schema after it and the commit.
type EditWrite struct {
	Schema SchemaView `json:"schema"`
	Commit *string    `json:"commit"`
}

// Use names the definition a removal takes away, for the check that no task uses it.
type Use struct {
	Target schema.Target
	Prop   string // for TargetValue
	Name   string
}

// Edit applies fn to the schema of the store under its lock and writes the
// result in one commit (op schema). use, when given, refuses the edit when a
// task uses that definition (schema.in_use); check, when true, refuses it when
// a task breaks the new schema where it did not break the old (schema.tasks_invalid).
func Edit(ss *usecase.Session, fn func(*schema.Schema) (*schema.Schema, error), use *Use, check bool) (EditWrite, error)
```

`SchemaView` に `PropOrder []string \`json:"prop_order"\`` を足し、`SchemaViewOf` で `sc.Props` の順に名前を入れる。

- [ ] **Step 1: 失敗するテストを書く**

`edit_test.go`（package `schemas_test`）。`usecasetest.New(t)` の Store は既定のスキーマ。

```go
func TestEditAddsInOneCommit(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	before := ts.Head()
	w, err := schemas.Edit(ts.Session, func(sc *schema.Schema) (*schema.Schema, error) {
		return sc.AddStatus(schema.Status{Name: "review", Category: schema.CategoryDoing, Color: "teal"}), nil
	}, nil, false)
	if err != nil || w.Commit == nil {
		t.Fatalf("Edit = %+v, %v", w, err)
	}
	if ts.Head() == before || usecasetest.LastOp(t, ts) != "schema" {
		t.Errorf("want one commit of op schema")
	}
	last := w.Schema.Statuses[len(w.Schema.Statuses)-1]
	if last.Name != "review" || last.Color != "teal" {
		t.Errorf("last status = %+v", last)
	}
}

func TestEditRefusesInUse(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddDeleted("old", time.Now(), func(t *model.Task) { t.Status = "on_hold" })
	_, err := schemas.Edit(ts.Session, func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveStatus("on_hold") },
		&schemas.Use{Target: schema.TargetStatus, Name: "on_hold"}, false)
	if usecase.CodeOf(err) != "schema.in_use" {
		t.Fatalf("err = %v", err)
	}
	var ue *usecase.Error
	if !errors.As(err, &ue) || ue.Args["count"] != 1 || ue.Args["deleted"] != 1 {
		t.Errorf("args = %v", ue.Args)
	}
}

func TestEditRefusesNewViolations(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddProject("Web")
	ts.AddTask("bug", func(t *model.Task) { t.Kind = "bug"; t.Parent = &p })
	_, err := schemas.Edit(ts.Session, func(sc *schema.Schema) (*schema.Schema, error) {
		ps := []string{schema.Root}
		return sc.ChangeKind("bug", schema.KindChange{Parents: &ps})
	}, nil, true)
	if usecase.CodeOf(err) != "schema.tasks_invalid" {
		t.Fatalf("err = %v", err)
	}
	var ue *usecase.Error
	if !errors.As(err, &ue) {
		t.Fatal(err)
	}
	lines, _ := ue.Args["violations"].([]string)
	if len(lines) != 1 || !strings.Contains(lines[0], "bug") {
		t.Errorf("violations = %v", lines)
	}
}

func TestEditIgnoresOldViolations(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddProject("Web")
	// A project under a project breaks the default schema (project sits at
	// the root only). Write it past the checks, as a sync can leave it.
	brokenUnder(t, ts, p)
	l := "タスク"
	if _, err := schemas.Edit(ts.Session, func(sc *schema.Schema) (*schema.Schema, error) {
		return sc.ChangeKind("task", schema.KindChange{Label: &l})
	}, nil, true); err != nil {
		t.Fatalf("an old violation refuses the edit: %v", err)
	}
}

func TestEditErrors(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	cases := []struct {
		name string
		fn   func(*schema.Schema) (*schema.Schema, error)
		code string
	}{
		{"not defined", func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveStatus("nope") }, "schema.not_defined"},
		{"stale", func(sc *schema.Schema) (*schema.Schema, error) { return sc.OrderStatuses([]string{"todo"}) }, "schema.order_stale"},
		{"invalid", func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveStatus("todo") }, "schema.invalid"},
		{"container", func(sc *schema.Schema) (*schema.Schema, error) {
			return sc.ChangeKind("epic", schema.KindChange{Default: true})
		}, "schema.default_container"},
		{"parents", func(sc *schema.Schema) (*schema.Schema, error) {
			sc = sc.AddKind(schema.Kind{Name: "sub", Role: schema.RoleWork, Parents: []string{"epic"}})
			return sc.RemoveKind("epic")
		}, "schema.parents_empty"},
	}
	for _, c := range cases {
		if _, err := schemas.Edit(ts.Session, c.fn, nil, false); usecase.CodeOf(err) != c.code {
			t.Errorf("%s: err = %v (%s)", c.name, err, usecase.CodeOf(err))
		}
	}
}
```

`TestEditRefusesInUse` は、`s.Tasks()` が削除済みのタスクを含むかどうかを確かめるテストでもある。件数 0 で落ちたら、テストを直さずに止めて報告する（spec の「削除済みも数える」を満たす読み方を決め直す必要があるため）。

`usecase.CodeOf` がなければ、`internal/app/usecase/errors.go` で誤りのコードを取る既存の関数を grep して使う。`brokenUnder(t, ts, parent)` はこのテストファイルに書く補助で、kind が project で parent が `parent` のタスクを、検証を通さずに Store に書いてコミットする。書き方は `grep -rn "WriteFile\|CheckOnly\|SkipRequired" internal/app/usecase/usecasetest/ internal/app/usecase/schemas/*_test.go` で、検証を通さずにタスクを置く既存の方法を探して合わせる（`store.Op` の検証を外すフラグか、ファイルを直接書いてコミットする補助）。見つからなければ止めて報告する。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/usecase/schemas/ -run Edit`
Expected: コンパイルエラー

- [ ] **Step 3: `edit.go` を書く**

```go
package schemas

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

type EditWrite struct {
	Schema SchemaView `json:"schema"`
	Commit *string    `json:"commit"`
}

type Use struct {
	Target schema.Target
	Prop   string
	Name   string
}

func Edit(ss *usecase.Session, fn func(*schema.Schema) (*schema.Schema, error), use *Use, check bool) (EditWrite, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return EditWrite{}, err
	}
	var after *schema.Schema
	hash, err := ss.UpdateCommit(s, store.Op{Name: "schema"}, func(tx *store.Tx) error {
		cur := s.Schema()
		next, err := fn(cur)
		if err != nil {
			return editErr(err)
		}
		if ps := next.AddedProblems(cur.Check()); len(ps) > 0 {
			return schemaInvalid(ps)
		}
		if use != nil {
			if err := notInUse(s, *use); err != nil {
				return err
			}
		}
		if check {
			if err := noNewViolations(s, ix, cur, next); err != nil {
				return err
			}
		}
		after = next
		return tx.PutSchema(next)
	})
	var se *store.SchemaError
	if errors.As(err, &se) {
		err = schemaInvalid(se.Problems)
	}
	if err != nil {
		return EditWrite{}, err
	}
	return EditWrite{Schema: SchemaViewOf(after), Commit: usecase.CommitPtr(usecase.CommitHex(hash))}, nil
}

// editErr gives the errors of the schema package their codes.
func editErr(err error) error {
	var pe *schema.ParentsEmptyError
	switch {
	case errors.As(err, &pe):
		return usecase.NewErr(usecase.KindInvalid, "schema.parents_empty", err, "kinds", pe.Kinds)
	case errors.Is(err, schema.ErrNotDefined):
		return usecase.NewErr(usecase.KindNotFound, "schema.not_defined", err)
	case errors.Is(err, schema.ErrOrderStale):
		return usecase.NewErr(usecase.KindConflict, "schema.order_stale", err)
	case errors.Is(err, schema.ErrDefaultContainer):
		return usecase.NewErr(usecase.KindInvalid, "schema.default_container", err)
	case errors.Is(err, schema.ErrNotTaskRef):
		return usecase.NewErr(usecase.KindInvalid, "schema.not_task_ref", err)
	case errors.Is(err, schema.ErrNotEnum):
		return usecase.NewErr(usecase.KindInvalid, "schema.not_enum", err)
	case errors.Is(err, schema.ErrValueInvalid):
		return usecase.NewErr(usecase.KindInvalid, "schema.value_invalid", err)
	}
	return err
}

func schemaInvalid(ps []schema.Problem) error {
	msgs := make([]string, len(ps))
	for i, p := range ps {
		msgs[i] = p.String()
	}
	return usecase.NewErr(usecase.KindInvalid, "schema.invalid", fmt.Errorf("the change breaks the schema: %s", strings.Join(msgs, "; ")), "problems", msgs)
}

// notInUse refuses a removal of a definition that a task uses, deleted
// tasks counted: they can be restored.
func notInUse(s *store.Store, u Use) error {
	count, deleted := 0, 0
	for t, err := range s.Tasks() {
		if err != nil {
			if _, ok := brokenTask(err); ok {
				continue
			}
			return err
		}
		if schema.TaskUses(t, u.Target, u.Prop, u.Name) {
			count++
			if t.DeletedAt != nil {
				deleted++
			}
		}
	}
	if count == 0 {
		return nil
	}
	return usecase.NewErr(usecase.KindInvalid, "schema.in_use",
		fmt.Errorf("%s uses %s; merge it into another first", TaskCount(count), u.Name), "count", count, "deleted", deleted)
}

// noNewViolations refuses a schema under which a task breaks a rule it did
// not break under the old one.
func noNewViolations(s *store.Store, ix *index.Index, cur, next *schema.Schema) error {
	read := func(u model.ULID) (*model.Task, error) {
		if e, ok := ix.Task(u); ok && e.Broken == "" {
			return &model.Task{ULID: e.ULID, Kind: e.Kind, Parent: e.Parent}, nil
		}
		return nil, store.ErrNotFound
	}
	var lines []string
	for t, err := range s.Tasks() {
		if err != nil {
			if _, ok := brokenTask(err); ok {
				continue
			}
			return err
		}
		oldKind, oldAnc, err := store.Lineage(t, cur, read)
		if err != nil {
			return err
		}
		newKind, newAnc, err := store.Lineage(t, next, read)
		if err != nil {
			return err
		}
		for _, v := range schema.ValidateWrite(t, oldKind, cur.For(oldAnc), t, newKind, next.For(newAnc), true) {
			lines = append(lines, usecase.RefName(ix, t.ULID)+": "+v.String())
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return usecase.NewErr(usecase.KindInvalid, "schema.tasks_invalid",
		fmt.Errorf("the change breaks %s:\n  %s", TaskCount(len(lines)), strings.Join(lines, "\n  ")), "violations", lines)
}
```

`ix` は書き込みの前に開いたものなので、ロックの中で親の種類を引くには十分（親子は schema の変更で変わらない）。`store.Lineage` の `read` の使い方は `internal/app/usecase/stores/movestore.go:900` と同じ。`usecase.CommitPtr` が空文字列で nil を返すことを確かめる。

- [ ] **Step 4: `prop_order` を足す**

`schemaview.go` の `SchemaView` に `PropOrder []string \`json:"prop_order"\`` を足し、`SchemaViewOf` で `v.PropOrder = append(v.PropOrder, p.Name)` を props のループに入れる（初期値は `[]string{}`）。`schemaview_test.go` に、`Default()` の `PropOrder` が `priority start due estimate points duplicates causes relates` になるテストを足す。

- [ ] **Step 5: 通ることを確かめる**

Run: `go test ./internal/app/usecase/schemas/`
Expected: PASS

- [ ] **Step 6: コミット**

```bash
git add internal/app/usecase/schemas/
git commit -m "feat(schemas): スキーマの編集を 1 コミットで書く"
```

---

### Task 3: サーバーのエンドポイントと生成物

**Files:**
- Create: `internal/app/server/schemaedit.go`、`internal/app/server/schemaedit_test.go`
- Modify: 登録の呼び出し（`registerScoped()` を呼んでいる箇所に `registerSchemaEdit()` を足す。`grep -n registerScoped internal/app/server/*.go`）
- Modify: `api/openapi.yaml`（`make generate`）、`web/core/api/generated.ts`（`cd web && pnpm gen`）

**Interfaces:**
- Consumes: `schemas.Edit`、`schemas.Use`、`schemas.EditWrite`、Task 1 の Change 型
- Produces（Task 4 以降が使う。Orval の名前は operationId から生成される）:

| operationId | メソッドとパス | 本文 |
|---|---|---|
| `add-status` | `POST /api/schema/statuses` | `{name, label?, category, color?}` |
| `change-status` | `PATCH /api/schema/statuses/{name}` | `{label?, category?, color?, default?}` |
| `delete-status` | `DELETE /api/schema/statuses/{name}` | なし |
| `order-statuses` | `PUT /api/schema/statuses/order` | `{names}` |
| `add-kind` | `POST /api/schema/kinds` | `{name, label?, role, parents?, anywhere?, icon?, template?}` |
| `change-kind` | `PATCH /api/schema/kinds/{name}` | `{label?, role?, parents?, anywhere?, icon?, template?, default?}` |
| `delete-kind` | `DELETE /api/schema/kinds/{name}` | なし |
| `order-kinds` | `PUT /api/schema/kinds/order` | `{names}` |
| `add-prop` | `POST /api/schema/props` | `{name, label?, type, values?, multi?, inverse?, symmetric?}` |
| `change-prop` | `PATCH /api/schema/props/{name}` | `{label?, multi?, inverse?, symmetric?}` |
| `delete-prop` | `DELETE /api/schema/props/{name}` | なし |
| `order-props` | `PUT /api/schema/props/order` | `{names}` |
| `add-value` | `POST /api/schema/props/{name}/values` | `{value}` |
| `delete-value` | `DELETE /api/schema/props/{name}/values/{value}` | なし |
| `order-values` | `PUT /api/schema/props/{name}/values/order` | `{names}` |

どれも `?store=` を取り、応答は `EditWrite`。生成されるクライアントの関数は `api.addStatus(body, {store})`、`api.changeStatus(name, body, {store})`、`api.deleteStatus(name, {store})`、`api.orderStatuses(body, {store})`、以下同じ形（`addValue(name, body, query)`、`deleteValue(name, value, query)`、`orderValues(name, body, query)`）。

- [ ] **Step 1: 失敗するテストを書く**

`schemaedit_test.go` は `scoped_test.go` の作りに合わせる（テスト用サーバーの立て方、要求の送り方、`libopenapi-validator` での照合）。最低限のケース:

```go
func TestSchemaEditRoundTrip(t *testing.T) {
	// 1. POST /api/schema/statuses {name: review, category: doing, color: teal} → 200、schema.statuses の末尾が review
	// 2. PATCH /api/schema/statuses/review {label: レビュー, default: true} → 200、doing の既定が review
	// 3. PUT /api/schema/statuses/order に名前の全体を逆順で → 200、その順
	// 4. PATCH /api/schema/statuses/in_progress {default: true} で既定を戻し、DELETE /api/schema/statuses/review → 200、review がない
	// 5. POST /api/schema/props/priority/values {value: none} → 200、DELETE …/values/none → 200
	// 6. GET /api/schema の prop_order が既定の順
	// 各応答を openapi と照合し、各書き込みで schema.changed が出ることを TestScopedPutPublishesSchemaChanged と同じ方法で 1 回確かめる
}

func TestSchemaEditErrors(t *testing.T) {
	// DELETE /api/schema/statuses/nope → 404 schema.not_defined
	// PUT /api/schema/statuses/order {names: [todo]} → 409 schema.order_stale
	// DELETE /api/schema/statuses/todo → 422 schema.invalid（既定）
	// backlog のタスクを 1 件作ってから DELETE /api/schema/statuses/backlog → 422 schema.in_use
	// PATCH /api/schema/kinds/bug {parents: []} → 400
	// PATCH /api/schema/kinds/bug {parents: [root], anywhere: true} → 400
}
```

ケースの本体は `scoped_test.go` の `TestScopedPutAndDelete` と `TestScopedPutErrors` の書き方をそのまま使って書く（要求の補助関数・状態コードとコードの確かめ方）。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/server/ -run SchemaEdit`
Expected: 404 または コンパイルエラー

- [ ] **Step 3: `schemaedit.go` を書く**

```go
package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
)

type DefParam struct {
	StoreParam
	Name string `path:"name" doc:"The name of the definition"`
}

type ValueParam struct {
	DefParam
	Value string `path:"value" doc:"The value of the enum or multi_enum property"`
}

type editOutput struct{ Body schemas.EditWrite }

type orderBody struct {
	Names []string `json:"names" doc:"Every name of the list, in the new order; a list that differs from the schema is 409 schema.order_stale"`
}

type statusAddInput struct {
	StoreParam
	Body struct {
		Name     string `json:"name"`
		Label    string `json:"label,omitempty"`
		Category string `json:"category" enum:"todo,doing,done,canceled"`
		Color    string `json:"color,omitempty"`
	}
}

type statusChangeInput struct {
	DefParam
	Body struct {
		Label    *string `json:"label,omitempty" doc:"Empty takes it off"`
		Category *string `json:"category,omitempty" enum:"todo,doing,done,canceled"`
		Color    *string `json:"color,omitempty" doc:"Empty takes it off"`
		Default  bool    `json:"default,omitempty" doc:"true makes it the default of its category; false is not a change"`
	}
}

type kindAddInput struct {
	StoreParam
	Body struct {
		Name     string   `json:"name"`
		Label    string   `json:"label,omitempty"`
		Role     string   `json:"role" enum:"container,work"`
		Parents  []string `json:"parents,omitempty" doc:"The kinds it may sit under, root for the top; leave it out with anywhere"`
		Anywhere bool     `json:"anywhere,omitempty"`
		Icon     string   `json:"icon,omitempty"`
		Template string   `json:"template,omitempty"`
	}
}

type kindChangeInput struct {
	DefParam
	Body struct {
		Label    *string   `json:"label,omitempty"`
		Role     *string   `json:"role,omitempty" enum:"container,work"`
		Parents  *[]string `json:"parents,omitempty" doc:"Not empty; anywhere takes the limit off"`
		Anywhere bool      `json:"anywhere,omitempty"`
		Icon     *string   `json:"icon,omitempty"`
		Template *string   `json:"template,omitempty"`
		Default  bool      `json:"default,omitempty"`
	}
}

type propAddInput struct {
	StoreParam
	Body struct {
		Name      string   `json:"name"`
		Label     string   `json:"label,omitempty"`
		Type      string   `json:"type" enum:"text,number,bool,date,datetime,duration,enum,multi_enum,user,task_ref,url"`
		Values    []string `json:"values,omitempty"`
		Multi     bool     `json:"multi,omitempty"`
		Inverse   string   `json:"inverse,omitempty"`
		Symmetric bool     `json:"symmetric,omitempty"`
	}
}

type propChangeInput struct {
	DefParam
	Body struct {
		Label     *string `json:"label,omitempty"`
		Multi     *bool   `json:"multi,omitempty"`
		Inverse   *string `json:"inverse,omitempty"`
		Symmetric *bool   `json:"symmetric,omitempty"`
	}
}

type valueAddInput struct {
	DefParam
	Body struct {
		Value string `json:"value"`
	}
}

type orderInput struct {
	StoreParam
	Body orderBody
}

type valuesOrderInput struct {
	DefParam
	Body orderBody
}

// parentsOf is the parents a kind is given: nil for anywhere. Both, or an
// empty list, are a bad request.
func parentsOf(parents []string, anywhere bool) ([]string, error) {
	switch {
	case anywhere && parents != nil:
		return nil, errors.New("parents and anywhere cannot be used together")
	case anywhere:
		return nil, nil
	case len(parents) == 0:
		return nil, errors.New("parents is empty; use anywhere to take the limit off")
	}
	return parents, nil
}

func (s *Server) edit(store string, fn func(*schema.Schema) (*schema.Schema, error), use *schemas.Use, check bool) (*editOutput, error) {
	ss := s.session(store)
	defer ss.Close()
	w, err := schemas.Edit(ss, fn, use, check)
	if err != nil {
		return nil, problemOf(err)
	}
	return &editOutput{Body: w}, nil
}

func (s *Server) registerSchemaEdit() {
	reg := func(id, method, path, summary string) huma.Operation {
		return huma.Operation{OperationID: id, Method: method, Path: path, Summary: summary, Tags: []string{"schema"}}
	}

	huma.Register(s.api, reg("add-status", http.MethodPost, "/api/schema/statuses", "Add a status at the end"),
		func(_ context.Context, in *statusAddInput) (*editOutput, error) {
			b := in.Body
			st := schema.Status{Name: b.Name, Label: b.Label, Category: schema.Category(b.Category), Color: b.Color}
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.AddStatus(st), nil }, nil, false)
		})
	huma.Register(s.api, reg("change-status", http.MethodPatch, "/api/schema/statuses/{name}", "Change a status"),
		func(_ context.Context, in *statusChangeInput) (*editOutput, error) {
			b := in.Body
			ch := schema.StatusChange{Label: b.Label, Color: b.Color, Default: b.Default}
			if b.Category != nil {
				c := schema.Category(*b.Category)
				ch.Category = &c
			}
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.ChangeStatus(in.Name, ch) }, nil, false)
		})
	huma.Register(s.api, reg("delete-status", http.MethodDelete, "/api/schema/statuses/{name}", "Remove a status no task uses"),
		func(_ context.Context, in *DefParam) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveStatus(in.Name) },
				&schemas.Use{Target: schema.TargetStatus, Name: in.Name}, false)
		})
	huma.Register(s.api, reg("order-statuses", http.MethodPut, "/api/schema/statuses/order", "Put the statuses in order"),
		func(_ context.Context, in *orderInput) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.OrderStatuses(in.Body.Names) }, nil, false)
		})

	huma.Register(s.api, reg("add-kind", http.MethodPost, "/api/schema/kinds", "Add a kind at the end"),
		func(_ context.Context, in *kindAddInput) (*editOutput, error) {
			b := in.Body
			ps, err := parentsOf(b.Parents, b.Anywhere)
			if err != nil {
				return nil, badRequest(err)
			}
			k := schema.Kind{Name: b.Name, Label: b.Label, Role: schema.Role(b.Role), Parents: ps, Icon: b.Icon, Template: b.Template}
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.AddKind(k), nil }, nil, false)
		})
	huma.Register(s.api, reg("change-kind", http.MethodPatch, "/api/schema/kinds/{name}", "Change a kind"),
		func(_ context.Context, in *kindChangeInput) (*editOutput, error) {
			b := in.Body
			ch := schema.KindChange{Label: b.Label, Icon: b.Icon, Template: b.Template, Default: b.Default}
			if b.Role != nil {
				r := schema.Role(*b.Role)
				ch.Role = &r
			}
			if b.Parents != nil || b.Anywhere {
				var given []string
				if b.Parents != nil {
					given = *b.Parents
					if given == nil {
						given = []string{}
					}
				}
				ps, err := parentsOf(given, b.Anywhere)
				if err != nil {
					return nil, badRequest(err)
				}
				ch.Parents = &ps
			}
			// role and parents can break the tasks there are
			check := ch.Role != nil || ch.Parents != nil
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.ChangeKind(in.Name, ch) }, nil, check)
		})
	huma.Register(s.api, reg("delete-kind", http.MethodDelete, "/api/schema/kinds/{name}", "Remove a kind no task uses"),
		func(_ context.Context, in *DefParam) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveKind(in.Name) },
				&schemas.Use{Target: schema.TargetKind, Name: in.Name}, false)
		})
	huma.Register(s.api, reg("order-kinds", http.MethodPut, "/api/schema/kinds/order", "Put the kinds in order"),
		func(_ context.Context, in *orderInput) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.OrderKinds(in.Body.Names) }, nil, false)
		})

	huma.Register(s.api, reg("add-prop", http.MethodPost, "/api/schema/props", "Add a property or a relation at the end"),
		func(_ context.Context, in *propAddInput) (*editOutput, error) {
			b := in.Body
			p := schema.Prop{Name: b.Name, Label: b.Label, Type: schema.PropType(b.Type), Values: b.Values, Multi: b.Multi, Inverse: b.Inverse, Symmetric: b.Symmetric}
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.AddProp(p), nil }, nil, false)
		})
	huma.Register(s.api, reg("change-prop", http.MethodPatch, "/api/schema/props/{name}", "Change a property or a relation"),
		func(_ context.Context, in *propChangeInput) (*editOutput, error) {
			b := in.Body
			ch := schema.PropChange{Label: b.Label, Inverse: b.Inverse, Multi: b.Multi, Symmetric: b.Symmetric}
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.ChangeProp(in.Name, ch) }, nil, b.Multi != nil)
		})
	huma.Register(s.api, reg("delete-prop", http.MethodDelete, "/api/schema/props/{name}", "Remove a property no task uses"),
		func(_ context.Context, in *DefParam) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveProp(in.Name) },
				&schemas.Use{Target: schema.TargetProp, Name: in.Name}, false)
		})
	huma.Register(s.api, reg("order-props", http.MethodPut, "/api/schema/props/order", "Put the relations, or the other properties, in order"),
		func(_ context.Context, in *orderInput) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.OrderProps(in.Body.Names) }, nil, false)
		})

	huma.Register(s.api, reg("add-value", http.MethodPost, "/api/schema/props/{name}/values", "Add a value at the end"),
		func(_ context.Context, in *valueAddInput) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.AddValue(in.Name, in.Body.Value) }, nil, false)
		})
	huma.Register(s.api, reg("delete-value", http.MethodDelete, "/api/schema/props/{name}/values/{value}", "Remove a value no task uses"),
		func(_ context.Context, in *ValueParam) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveValue(in.Name, in.Value) },
				&schemas.Use{Target: schema.TargetValue, Prop: in.Name, Name: in.Value}, false)
		})
	huma.Register(s.api, reg("order-values", http.MethodPut, "/api/schema/props/{name}/values/order", "Put the values in order"),
		func(_ context.Context, in *valuesOrderInput) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.OrderValues(in.Name, in.Body.Names) }, nil, false)
		})
}
```

huma が `*[]string` の `parents: []` と未指定を区別できるかを、テストの `{parents: []}` → 400 で確かめる。区別できなければ、本文を `json.RawMessage` で受けずに、`Parents` を `[]string` と `ParentsGiven`（huma の `omitempty` と既定値の扱い）で表す方法を探し、どれも無理なら止めて報告する。`badRequest` と `problemOf` は `admin.go` にある既存の関数。`/api/schema/statuses/order` と `/api/schema/statuses/{name}` のパスがぶつかる（`order` という名前のステータスを PATCH できなくなる）かを確かめる。huma のルーターが固定のパスを優先するなら、`PUT …/order` と `PATCH …/{name}` はメソッドも違うので問題ない。ぶつかる場合は止めて報告する。

- [ ] **Step 4: 生成物を作り直す**

Run: `make generate && cd web && pnpm gen`
`api/openapi.yaml` に 15 の操作と `prop_order` が入り、`web/core/api/generated.ts` に `addStatus` などが出ることを確かめる。

- [ ] **Step 5: 通ることを確かめる**

Run: `go test ./internal/app/server/ -run 'SchemaEdit|Scoped|OpenAPI'`
Expected: PASS

- [ ] **Step 6: コミット**

```bash
git add internal/app/server/ api/openapi.yaml web/core/api/generated.ts
git commit -m "feat(server): スキーマの編集の API を足す"
```

---

### Task 4: フロントの土台（並び・色・アイコン・参照）

**Files:**
- Create: `web/core/schema-edit.ts`、`web/core/schema-edit.test.ts`
- Modify: `web/app/table/columns.ts`、`web/app/detail/relation-groups.ts`、`web/app/detail/prop-rows.ts`、`web/app/gantt/GanttView.svelte`、`web/core/scoped.ts`、`web/core/schema-usage.ts`（並び）、それぞれのテスト

**Interfaces:**
- Produces:

```ts
export const STATUS_COLORS: { value: string; label: string; token: string }[]; // 12 色。value は YAML の名前
export const KIND_ICONS: { value: string; label: string }[]; // project プロジェクト・epic エピック・story ストーリー・task タスク・bug バグ
/** The props of the schema in the order of schema.yaml. */
export function propNames(schema: SchemaView): string[];
/** The color to draw: a token for a palette name, the value for another plain color, undefined otherwise. */
export function statusColor(color: string | undefined): string | undefined;
/** Where a removal takes the definition out of, one line each, in Japanese. */
export function referencesOf(schema: SchemaView, target: "status" | "kind" | "prop" | "value", name: string): string[];
/** Kinds whose only parent is name (a removal of name is refused). */
export function orphanedBy(schema: SchemaView, kind: string): string[];
```

色の対応:

| value | label | token |
|---|---|---|
| gray | グレー | `var(--muted)` |
| blue | ブルー | `var(--accent)` |
| green | グリーン | `var(--success)` |
| amber | アンバー | `var(--amber)` |
| red | レッド | `var(--danger)` |
| teal | ティール | `var(--store-teal-dot)` |
| orange | オレンジ | `var(--store-orange-dot)` |
| purple | パープル | `var(--store-purple-dot)` |
| sky | スカイ | `var(--store-sky-dot)` |
| pink | ピンク | `var(--store-pink-dot)` |
| olive | オリーブ | `var(--store-olive-dot)` |
| brown | ブラウン | `var(--store-brown-dot)` |

- [ ] **Step 1: 失敗するテストを書く**

```ts
import { describe, expect, it } from "vitest";
import { orphanedBy, propNames, referencesOf, statusColor } from "./schema-edit";
import type { SchemaView } from "./schema";

const schema = {
  statuses: [
    { name: "todo", category: "todo", default: true },
    { name: "on_hold", category: "todo" },
  ],
  kinds: [
    { name: "epic", role: "container", parents: ["root"] },
    { name: "sub", role: "work", parents: ["epic"] },
    { name: "task", role: "work", parents: ["root", "epic"], default: true },
  ],
  props: { b: { type: "number" }, a: { type: "text" } },
  prop_order: ["b", "a"],
  workflow: { transitions: { todo: ["on_hold"], on_hold: ["todo"] }, required: { on_hold: ["b"] } },
  scoped: [{ where: "project:01J", statuses: ["todo", "on_hold"], templates: { default: { epic: "plan" } } }],
} as unknown as SchemaView;

describe("schema-edit", () => {
  it("orders the props as the file does", () => {
    expect(propNames(schema)).toEqual(["b", "a"]);
  });
  it("draws a palette name with its token and keeps a plain color", () => {
    expect(statusColor("teal")).toBe("var(--store-teal-dot)");
    expect(statusColor("#3451D1")).toBe("#3451D1");
    expect(statusColor("url(x)")).toBeUndefined();
    expect(statusColor(undefined)).toBeUndefined();
  });
  it("lists the references a removal takes out", () => {
    expect(referencesOf(schema, "status", "on_hold")).toEqual([
      "ワークフロー: todo から移せる先",
      "ワークフロー: on_hold からの遷移の制約",
      "ワークフロー: on_hold に入るときの必須の項目",
      "プロジェクト 01J の上書き: 表示するステータス",
    ]);
    expect(referencesOf(schema, "kind", "epic")).toEqual([
      "種類 sub の置ける親",
      "種類 task の置ける親",
      "プロジェクト 01J の上書き: 既定のテンプレート",
    ]);
    expect(referencesOf(schema, "prop", "b")).toEqual(["ワークフロー: on_hold に入るときの必須の項目"]);
  });
  it("finds the kinds a removal leaves with no parent", () => {
    expect(orphanedBy(schema, "epic")).toEqual(["sub"]);
  });
});
```

上書きのプロジェクトは、`referencesOf` の呼び出し元（画面）が ID とタイトルに置き換えられるよう、ULID のままの文にする。画面は `where` の ULID からタスクを引けたら「プロジェクト WEB-1 リリース準備 の上書き」と出す（`SettingsSchema.svelte` の `taskByUlid` を使う既存の書き方に合わせる）。そのため `referencesOf` の戻り値は `{ text: string; project?: string }[]` にしてよい。その場合はテストもそれに合わせる。

- [ ] **Step 2〜4: 失敗を確かめ、`schema-edit.ts` を書き、通す**

`statusColor` は `STATUS_COLORS` の value なら token を、そうでなければ `StatusIcon.svelte` の `safeColor` と同じ正規表現 `/^(#[0-9a-f]{3,8}|[a-z]+|var\(--[a-z0-9-]+\))$/i` を通った値を返す。`safeColor` は `StatusIcon` から消し、`statusColor` を使う（同じ規則を 2 か所に持たない）。

Run: `cd web && pnpm vitest run core/schema-edit.test.ts`

- [ ] **Step 5: プロパティの並びを `propNames` に替える**

次の箇所の `Object.keys(sc.props).sort()` と `Object.entries(sc.props)` を、`propNames(sc)` の順に替える: `web/app/table/columns.ts:49`、`web/app/detail/relation-groups.ts:59,100`、`web/app/detail/prop-rows.ts:21`、`web/app/gantt/GanttView.svelte:181`、`web/core/scoped.ts:96`。`prop_order` がない（古いサーバーやテストの偽物）ときは、`propNames` が `Object.keys(schema.props)` を名前の順で返す。各ファイルのテストで名前の順を前提にしている期待値は、`prop_order` を与えたうえで、その順を期待するように直す。

Run: `cd web && pnpm vitest run`
Expected: PASS

- [ ] **Step 6: コミット**

```bash
git add web/core web/app/table web/app/detail web/app/gantt
git commit -m "feat(web): プロパティをスキーマの順に並べ、色と参照の補助を足す"
```

---

### Task 5: 色とアイコンを全画面で使う

**Files:**
- Modify: `web/app/icons/StatusIcon.svelte`、`web/app/icons/KindIcon.svelte`、`web/app/icons/icons.test.ts`
- Modify: 呼び出し元（`Sidebar`・`MyTasks`・`CreateTask`・`CommandPalette`・`ProjectSettings`・`TableCell`・`ProjectHeader`・`Relations`・`KanbanCard`・`GanttView`・`TreeRow`・`SettingsSchema`・`WorkspaceOverview`・`KanbanView`・`PlanRow`・`ReleasesPage`）

**Interfaces:**
- Consumes: `statusColor`、`KIND_ICONS`（Task 4）
- Produces:

```ts
// StatusIcon（color は前からある。中で statusColor を通す）
type Props = { status: string; category?: string; color?: string; size?: number };
// KindIcon
type Props = { kind: string; role?: string; icon?: string; size?: number };
```

アイコンはスキーマも context も読まない（「この plan で埋める spec にない細部」の 7）。

`KindIcon` の形は、`icon` が組み込みの形の名前ならその形、そうでなければ今までどおり名前、役割の順で決める。bug の赤（`--danger`）は、形が bug のときに付ける。

- [ ] **Step 1: 失敗するテストを書く**

`icons.test.ts` に足す:

```ts
it("draws a palette color", () => {
  const { container } = render(StatusIcon, { status: "review", category: "doing", color: "teal" });
  expect(container.querySelector("circle")?.getAttribute("stroke")).toBe("var(--store-teal-dot)");
});
it("draws the shape the schema names", () => {
  const { container } = render(KindIcon, { kind: "incident", role: "work", icon: "bug" });
  expect(container.querySelector("svg")?.dataset.kind).toBe("bug");
  expect(container.querySelector("svg")?.getAttribute("style")).toContain("var(--danger)");
});
it("ignores an icon it does not know", () => {
  const { container } = render(KindIcon, { kind: "incident", role: "container", icon: "rocket" });
  expect(container.querySelector("svg")?.dataset.kind).toBe("container");
});
```

属性の名前（`stroke` か `style` か）は、今の `StatusIcon.svelte` の描き方（80 行目以降）に合わせて確かめる。

呼び出し元のテスト（`TreeRow` があればそのテスト、なければ `tree` のテスト）に、スキーマの `color: "pink"` と `icon: "bug"` がその行のアイコンに出ることを 1 つ足す。

- [ ] **Step 2〜4: 失敗を確かめ、2 つのアイコンを書き換え、通す**

Run: `cd web && pnpm vitest run app/icons`

- [ ] **Step 5: 呼び出し元から色とアイコンを渡す**

各呼び出し元で、いま `category` / `role` を引いている箇所と同じところで、同じ定義から `color` / `icon` を引いて渡す（例: `TreeRow.svelte` は `const def = $derived(schema?.statuses?.find(…))` にまとめて `category={def?.category} color={def?.color}`）。`categoryOf(...)` のような補助を使っている箇所（`MyTasks`・`Relations`・`WorkspaceOverview`）は、その補助の隣に色を返す補助を足す（同じ引き方を 2 回書かない）。スキーマの定義そのものを描く箇所（`SettingsSchema`・`ProjectSettings`・`CreateTask`）は、定義の `color` / `icon` を直接渡す。Store をまたいでカテゴリでそろえた列（`KanbanView` で列が複数の Store にまたがるとき、`WorkspaceOverview` のカテゴリの見出し）は渡さない。`KanbanView` の列が 1 つの Store のものかどうかは、その列の作り方（`col` の型）を読んで決める。`Sidebar` と `ProjectHeader` の `kind="project"` は、プロジェクトの Store のスキーマを持っていれば `icon` を渡す。

Run: `cd web && pnpm vitest run && pnpm check`
Expected: PASS

- [ ] **Step 6: コミット**

```bash
git add web/app
git commit -m "feat(web): スキーマの色とアイコンで描く"
```

---

### Task 6: スキーマ画面をタブごとに分ける

**Files:**
- Modify: `web/app/settings/SettingsSchema.svelte`
- Create: `web/app/settings/SchemaStatuses.svelte`、`SchemaKinds.svelte`、`SchemaProps.svelte`、`SchemaRelations.svelte`、`SchemaWorkflow.svelte`

**Interfaces:**
- 各タブは `{ store, schema, usage, off, onaction }` を受ける。`usage` は `schemaUsage` の結果（null 可）、`off` は書き込めない理由（null 可）、`onaction(a: SchemaAction)` は親にダイアログを開かせる。

```ts
export type SchemaAction =
  | { kind: "migrate"; target: MigrateTarget; prop?: string; old: string; mode: "rename" | "merge"; names: string[] }
  | { kind: "add"; target: "status" | "kind" | "prop" | "relation" }
  | { kind: "edit"; target: "status" | "kind" | "prop" | "relation"; name: string }
  | { kind: "delete"; target: "status" | "kind" | "prop" | "value"; name: string; prop?: string }
  | { kind: "add-value"; prop: string };
```

この型は `web/app/settings/schema-action.ts` に置く。

- [ ] **Step 1: 分ける**

見た目と挙動を変えずに、各タブの `<section>` を対応するコンポーネントへ移す。`actions` スニペットは各タブが使う小さなコンポーネント `SchemaRowActions.svelte` に移す。`ask`（migrate のダイアログ）は親に残し、`onaction` の `migrate` で開く。

- [ ] **Step 2: 既存のテストが通ることを確かめる**

Run: `cd web && pnpm vitest run app/settings && pnpm check`
Expected: PASS（テストを変えずに）

- [ ] **Step 3: コミット**

```bash
git add web/app/settings
git commit -m "refactor(web): スキーマ画面をタブごとのコンポーネントに分ける"
```

---

### Task 7: 追加・編集・削除のダイアログ

**Files:**
- Create: `web/app/settings/SchemaDefDialog.svelte`、`SchemaDeleteDialog.svelte`、`SchemaValueDialog.svelte`、`ColorPicker.svelte`、`IconPicker.svelte`
- Modify: 各タブと `SettingsSchema.svelte`、`SchemaRowActions.svelte`
- Test: `web/app/settings/SchemaDefDialog.test.ts`、`SchemaDeleteDialog.test.ts`、`SettingsSchema.test.ts`

**Interfaces:**
- Consumes: 生成クライアントの関数（Task 3）、`STATUS_COLORS`・`KIND_ICONS`・`referencesOf`・`orphanedBy`（Task 4）、`SchemaAction`（Task 6）

**画面の決まり（spec のとおり）:**

- 表の上に「追加…」。行に「編集…」「名前…」「マージ…」「削除…」。enum の値の行に「名前…」「マージ…」「削除…」、プロパティの行に「値を追加…」。
- `SchemaDefDialog` は追加と編集を兼ねる。タイトルは「ステータスを追加」「ステータス「todo」を編集」の形。
  - ステータス: 名前（追加だけ）・表示名・カテゴリ（`<select>`、`CATEGORY_LABEL`）・色（`ColorPicker`）・既定（チェックボックス）。編集で既定のステータスは、カテゴリを無効にし「既定のステータスはカテゴリを変えられません。先に別のステータスを既定にしてください」、既定のチェックを外せなくし「既定を外すには、別のステータスを既定にしてください」と出す。
  - 種類: 名前（追加だけ）・表示名・役割・置ける親（「どこにでも」のラジオと、「指定する」のラジオ＋ルートとほかの種類のチェックボックス）・アイコン（`IconPicker`）・既定のテンプレート（`SettingsTemplates` が使うテンプレートの一覧から選ぶ `<select>`、空は「なし」）・既定（work のときだけ）。「指定する」で 1 つも選ばれていなければ保存を無効にし「置ける親を 1 つ以上選んでください」。既定の種類は役割を無効にし理由を出す。
  - プロパティ: 名前・型（追加だけ。関連タブからは `task_ref` に固定して型の欄を出さない。プロパティタブの型の候補に `task_ref` を出さない）・表示名・値（enum と multi_enum の追加だけ。1 行に 1 つのテキストエリア）。関連は加えて、複数・逆方向の表示名・双方向（双方向と逆方向の表示名は同時に使えないので、双方向をオンにすると逆方向の欄を無効にする）。
  - 送るのは変わった欄だけ（`PATCH`）。成功したら `c.schema.reload(store)`、トーストに「ステータス「review」を追加しました」「…を保存しました」、閉じる。
  - 失敗はダイアログの中に `FormError`。`schema.tasks_invalid` は `violations` を一覧し、各行の先頭のタスクの ID をタスク詳細へのリンクにする（`router.href({ name: "task", … })` の既存の書き方）。
- `ColorPicker`: `STATUS_COLORS` の 12 色と「自動」のボタン群（`role="radiogroup"`）。今の値が候補にない値なら、先頭に「カスタム（値）」を選択状態で出す。何も選び直さなければ `color` を送らない。
- `IconPicker`: `KIND_ICONS` の 5 つと「自動」。候補にない値は「カスタム（値）」として同じ扱い。
- `SchemaDeleteDialog`: 「ステータス「on_hold」を削除」。`referencesOf` の一覧があれば「次の参照も外れます」と並べる。`orphanedBy` が空でなければ削除ボタンを無効にし「種類 sub の置ける親がなくなるため削除できません。先に sub の置ける親を変えてください」。サーバーの `schema.in_use` は文をそのまま出す。
- 削除ボタンの無効と理由（行のボタン）:
  - 件数が 1 以上: 「タスクが使っているため削除できません。マージ… で別の定義へ移してから削除してください」
  - 既定のステータス・既定の種類: 「既定は削除できません。別のものを既定にしてから削除してください」
  - 書き込めない Store: 今の `off` の文
- `SchemaValueDialog`: 「priority に値を追加」。値の欄だけ。
- リード文を「この Store で使う共通の定義です。ここで追加・編集・並べ替えができます。」に変える。
- `SchemaRelations` の `blocked_by` の行にはボタンを出さない（今のまま）。

- [ ] **Step 1: 失敗するテストを書く**

`SchemaDefDialog.test.ts`（`MigrateDialog` のテストがあればその書き方、なければ `SettingsSchema.test.ts` の偽の API の書き方に合わせる）:

```ts
it("adds a status with a palette color", async () => {
  // ダイアログを status の追加で開く → 名前 review、表示名 レビュー、カテゴリ 進行中、色 ティール を選び「追加」
  // → api.addStatus が { name: "review", label: "レビュー", category: "doing", color: "teal" } と { store } で 1 回呼ばれ、
  //   schema.reload が呼ばれ、トースト「ステータス「review」を追加しました」が出る
});
it("sends only what changed", async () => {
  // todo を編集で開き、表示名だけ変えて「保存」 → api.changeStatus("todo", { label: "新" }, { store })
});
it("keeps a custom color unless another is picked", async () => {
  // color: "#3451D1" の in_progress を編集で開く → 「カスタム（#3451D1）」が選択状態。表示名だけ変えて保存 → body に color がない
});
it("does not let the category of a default status change", async () => {
  // 既定の todo を開く → カテゴリの select が disabled、理由の文が出る
});
it("refuses a kind with no parent chosen", async () => {
  // 種類の追加で「指定する」を選び何もチェックしない → 「追加」が disabled、「置ける親を 1 つ以上選んでください」
});
it("lists the tasks a change would break", async () => {
  // api.changeKind が ApiError(422, code schema.tasks_invalid, args.violations ["WEB-2 バグ: parent: …"]) を投げる
  // → ダイアログに WEB-2 のリンクと文が出る
});
```

`SchemaDeleteDialog.test.ts`:

```ts
it("lists the references and deletes", async () => { /* on_hold → 参照 4 行 → 「削除」 → api.deleteStatus("on_hold", { store }) */ });
it("refuses a kind that leaves another with no parent", async () => { /* epic → 削除ボタン disabled と理由 */ });
it("shows the server's in_use", async () => { /* ApiError 422 schema.in_use → 文がそのまま出る */ });
```

`SettingsSchema.test.ts` に足す:

```ts
it("disables delete for a definition in use and for a default", () => { /* 件数 1 の backlog と 既定の todo の削除ボタンが disabled、理由が出る */ });
it("shows the new lead", () => { /* 「ここで追加・編集・並べ替えができます」 */ });
```

テストの本体は、各ファイルの既存のテストの補助（描画、偽の API、`ApiError` の作り方）を使って、上のコメントのとおりに書く。

- [ ] **Step 2〜4: 失敗を確かめ、実装し、通す**

Run: `cd web && pnpm vitest run app/settings && pnpm check && pnpm lint`

- [ ] **Step 5: コミット**

```bash
git add web/app/settings
git commit -m "feat(web): スキーマの定義を画面から追加・編集・削除する"
```

---

### Task 8: 並べ替え（ドラッグとキーボード）

**Files:**
- Create: `web/app/settings/reorder.ts`、`web/app/settings/reorder.test.ts`
- Modify: 各タブ（ステータス・種類・プロパティ・関連と enum の値）

**Interfaces:**
- Produces:

```ts
/** names with the item at from moved to to (indexes in names). */
export function moved(names: readonly string[], from: number, to: number): string[];
/** A Svelte action for a row: drag by its handle, drop on another row of the same list. */
export function sortableRow(node: HTMLElement, opts: { list: string; name: string; index: number; onmove: (from: number, to: number) => void }): { update(o: typeof opts): void; destroy(): void };
```

- [ ] **Step 1: 失敗するテストを書く**

```ts
import { describe, expect, it } from "vitest";
import { moved } from "./reorder";

describe("moved", () => {
  it("moves down and up", () => {
    expect(moved(["a", "b", "c"], 0, 2)).toEqual(["b", "c", "a"]);
    expect(moved(["a", "b", "c"], 2, 0)).toEqual(["c", "a", "b"]);
  });
  it("leaves the list alone at the edges", () => {
    expect(moved(["a", "b"], 0, -1)).toEqual(["a", "b"]);
    expect(moved(["a", "b"], 1, 2)).toEqual(["a", "b"]);
  });
});
```

タブのテスト（`SettingsSchema.test.ts`）に:

```ts
it("moves a status down with Alt+ArrowDown on its handle", async () => {
  // 「todo を並べ替え」のつまみ（button、aria-label="並べ替え（todo）"）にフォーカスして Alt+ArrowDown
  // → api.orderStatuses({ names: [backlog, on_hold, todo, …] の順 }, { store }) 。名前の並びは既定のスキーマの todo と on_hold を入れ替えたもの
  // 送ったあともつまみにフォーカスが残る
});
it("reloads and says so when the order is stale", async () => {
  // orderStatuses が ApiError(409, schema.order_stale) → schema.reload が呼ばれ、
  // 「ほかの変更と重なったため並べ替えられませんでした。読み込み直したので、もう一度並べ替えてください」が出る
});
it("moves a value within its property", async () => { /* priority › mid のつまみで Alt+ArrowUp → api.orderValues("priority", { names: ["mid","low","high","urgent"] }, { store }) */ });
```

- [ ] **Step 2〜4: 失敗を確かめ、実装し、通す**

- つまみは各行の先頭の `<button>`（アイコン `Icon` の既存の名前から、なければ縦の点の SVG を `Icon.svelte` に足す）。`aria-label="並べ替え（名前）"`、`aria-describedby` で「⌥↑ / ⌥↓ で移動」を読ませる。
- キーボード: `e.altKey && (e.key === "ArrowUp" || e.key === "ArrowDown")`。ツリーの並べ替えのキー判定（`web/app/tree/` の ⌥↑↓ の処理）を読み、同じ判定の関数があればそれを使う。
- ドラッグ: `@atlaskit/pragmatic-drag-and-drop` の `draggable`（`dragHandle` につまみ）と `dropTargetForElements`、`@atlaskit/pragmatic-drag-and-drop-hitbox` の `attachClosestEdge` / `extractClosestEdge`。カンバンの `web/app/kanban/dnd.ts` の使い方に合わせる。`getData` に `{ list, name, index }` を入れ、`canDrop` は同じ `list` だけを受ける（ステータスの行に種類を落とせない、ほかのプロパティの値の行に落とせない）。
- 動かしたら、楽観的に並べずに、`order-*` を送って `c.schema.reload(store)` を待って描き直す（409 のときに戻す処理を持たないため）。送っている間はその表のつまみを無効にする。
- 書き込めない Store ではつまみを無効にし、`off` の理由を出す。
- プロパティタブと関連タブは、それぞれのタブの名前の全体を `orderProps` に送る。

Run: `cd web && pnpm vitest run app/settings && pnpm check && pnpm lint`

- [ ] **Step 5: コミット**

```bash
git add web/app/settings web/app/icons
git commit -m "feat(web): スキーマの定義をドラッグとキーボードで並べ替える"
```

---

### Task 9: E2E と spec の更新、仕上げ

**Files:**
- Create: `web/e2e/schema-edit.spec.ts`
- Modify: `docs/superpowers/specs/2026-11-01-phase4-gui-design.md`

- [ ] **Step 1: E2E を書く**

既存のスキーマ画面の E2E（`grep -ln "schema" web/e2e/*.ts`）の準備（Store の作り方、画面の開き方）に合わせて、1 本の通しを書く:

1. `/settings/schema/<store>` を開く
2. 「追加…」でステータス review（進行中、ティール）を追加し、表の末尾に出る
3. review の「編集…」で表示名を「レビュー」にし、表に出る
4. review のつまみで Alt+ArrowUp を 1 回押し、1 つ上に動く
5. ツリーの画面で、review にしたタスクのステータスのアイコンがティールで描かれる（`stroke` の値を確かめる）。タスクは手順 2 のあと、既存の補助で作って review にする
6. タスクを別のステータスに戻してから、review の「削除…」で削除し、表から消える

- [ ] **Step 2: E2E を回す**

Run: `make e2e`（なければ `cd web && pnpm exec playwright test e2e/schema-edit.spec.ts`。手順は Makefile の `e2e` を読む）
Expected: PASS

- [ ] **Step 3: GUI spec を直す**

`docs/superpowers/specs/2026-11-01-phase4-gui-design.md` の 88 行目の SettingsSchema の行を「スキーマの閲覧・編集、schema check、migrate」に変え、59 行目（スキーマ画面、2026-11-08）の下に次の行を足す:

```markdown
- スキーマの編集（2026-11-08、利用者の決定）: スキーマ画面から、ステータス・種類・プロパティ・関連・enum の値の追加・削除・型以外の属性の変更・並べ替えができる。使用中の定義は削除できずマージへ案内し、既存のタスクが違反する変更は拒否する。色はパレットから選び、色とアイコンは全画面の表示に使う。CLI は足さない。詳細は `docs/superpowers/specs/2026-11-08-schema-gui-edit-design.md`
```

- [ ] **Step 3b: 親設計に 1 行足す（コントローラーが行う）**

親設計（Claude Docs `https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc`、本文ノード `def60e43-5b84`）のスキーマ節の箇条書き（リスト `mz4fg2vqpzh.2517`）の末尾、「スキーマ変更時は hh schema check で…」の項目の後に、「GUI のスキーマ画面から、定義の追加・削除・属性の変更・並べ替えができる（CLI では YAML を編集する）」を足す。docs のツールが使えるのはコントローラーだけなので、実装者には渡さない。

- [ ] **Step 4: 全体の検査を 1 回回す**

Run: `make check`（Go のテスト・vet・lint、web の check・lint・test、生成物の古さの検出。Makefile の `check` の中身を読み、含まれないものがあれば足りない分を回す）
Expected: すべて PASS。落ちたテストは、失敗の出力と実行条件（コマンド、並列数、OS）を残してから直す。

- [ ] **Step 5: コミット**

```bash
git add web/e2e docs/superpowers/specs/2026-11-01-phase4-gui-design.md
git commit -m "test(e2e): スキーマの編集を通しで確かめ、GUI の spec を直す"
```
