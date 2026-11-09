package convert

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/merge"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/domain/upgrade"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// Step 1 to 2 fixes the kinds of tasks. In format 1 schema.yaml listed the
// kinds, with a role (container or work) and a default template for each, and
// said where each could sit; format 2 has the six kinds of the code
// (schema.Kinds), and the default templates are templates.default. This is
// the only place that reads the old form.

// oldKind is one entry of the kinds of a schema.yaml in format 1.
type oldKind = upgrade.OldKind

// isItem reports whether kind is a story, a task or a bug.
func isItem(kind string) bool { return kind == "story" || kind == "task" || kind == "bug" }

// step2 is the step of format 1.
func step2(t *Tree, opt Options) (Report, error) {
	c, err := loadStep2(t, opt)
	if err != nil {
		return Report{}, err
	}
	if err := c.checkKindFlags(); err != nil {
		return Report{}, err
	}
	c.mapKinds()
	c.placeAll()
	if err := c.writeTasks(); err != nil {
		return Report{}, err
	}
	if err := c.convertSchema(); err != nil {
		return Report{}, err
	}
	c.convertTemplates()
	return Report{Lines: c.lines()}, nil
}

type rankKey struct {
	ws     model.ULID
	parent model.ULID // zero for the root
}

type step2State struct {
	t    *Tree
	opt  Options
	sc   *schema.Schema
	kind map[string]oldKind // by name, in the old schema
	file []oldKind          // in file order

	ws     map[model.ULID]*model.Workspace
	tasks  map[model.ULID]*model.Task
	order  []model.ULID // of tasks, by ULID
	placed map[model.ULID]int
	dirty  map[model.ULID]bool
	used   map[string]int // the numbers in use, by id prefix
	last   map[rankKey]model.Rank
	unread []string // files that do not decode, and why a task stays as it is

	kindCount  map[[2]string]int // old, new
	toStep     int
	stepToTask int
	liftStep   int
	liftGroup  int
	toUncat    map[model.ULID]int // by workspace
	madeUncat  map[model.ULID]*model.Task
	notes      []string
}

func loadStep2(t *Tree, opt Options) (*step2State, error) {
	c := &step2State{
		t: t, opt: opt, kind: map[string]oldKind{},
		ws: map[model.ULID]*model.Workspace{}, tasks: map[model.ULID]*model.Task{},
		placed: map[model.ULID]int{}, dirty: map[model.ULID]bool{}, used: map[string]int{},
		last: map[rankKey]model.Rank{}, kindCount: map[[2]string]int{},
		toUncat: map[model.ULID]int{}, madeUncat: map[model.ULID]*model.Task{},
	}
	sc, err := schema.Decode(t.Read(schema.Path))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", schema.Path, err)
	}
	c.sc = sc
	if c.file, err = upgrade.OldKinds(sc); err != nil {
		return nil, err
	}
	for _, k := range c.file {
		if _, dup := c.kind[k.Name]; !dup {
			c.kind[k.Name] = k
		}
	}
	for _, p := range t.Paths("workspaces/") {
		u, err := model.ParseWorkspacePath(p)
		if err != nil {
			continue
		}
		if w, err := model.DecodeWorkspace(t.Read(p)); err == nil && w.ULID == u {
			c.ws[u] = w
		} else {
			c.unread = append(c.unread, p)
		}
	}
	for _, p := range t.Paths("tasks/") {
		u, err := model.ParseTaskPath(p)
		if err != nil {
			continue
		}
		b := t.Read(p)
		if model.IsForward(b) {
			continue
		}
		if tk, err := model.DecodeTask(b); err == nil && tk.ULID == u {
			c.tasks[u] = tk
			model.CountIDs(c.used, tk)
		} else {
			c.unread = append(c.unread, p)
			model.RaiseNumbers(c.used, model.IDNumbersIn(b))
		}
	}
	c.order = slices.SortedFunc(maps.Keys(c.tasks), compareULIDs)
	if b := t.Read(model.NumbersPath); b != nil {
		if n, err := model.DecodeNumbers(b); err == nil {
			model.RaiseNumbers(c.used, n)
		}
	}
	return c, nil
}

func compareULIDs(a, b model.ULID) int { return strings.Compare(a.String(), b.String()) }

// checkKindFlags refuses a --kind that moves a kind the store does not have
// or to a kind that is not one of the fixed ones.
func (c *step2State) checkKindFlags() error {
	have := map[string]bool{}
	for name := range c.kind {
		have[name] = true
	}
	for _, tk := range c.tasks {
		have[tk.Kind] = true
	}
	for _, from := range slices.Sorted(maps.Keys(c.opt.Kinds)) {
		to := c.opt.Kinds[from]
		if _, ok := schema.KindOf(to); !ok {
			return usecase.NewErr(usecase.KindUsage, "convert.bad_kind",
				fmt.Errorf("--kind %s=%s: %s is not a kind (project, epic, story, task, bug or step)", from, to, to), "from", from, "to", to)
		}
		if !have[from] {
			return usecase.NewErr(usecase.KindUsage, "convert.bad_kind",
				fmt.Errorf("--kind %s=%s: the store has no kind %s", from, to, from), "from", from, "to", to)
		}
	}
	return nil
}

// newKind is the fixed kind an old kind goes to, for a task at the root or
// not. --kind comes first; a name that is a fixed kind stays; a custom group
// is a project at the root and an epic below it; a custom item is a task.
func (c *step2State) newKind(name string, atRoot bool) string {
	if to, ok := c.opt.Kinds[name]; ok {
		return to
	}
	return upgrade.FixedKind(c.file, name, atRoot)
}

// kindWide is newKind for what has no place in a tree, such as the default
// template of a kind: a group counts as below the root, an epic.
func (c *step2State) kindWide(name string) string { return c.newKind(name, false) }

// mapKinds gives every task a fixed kind.
func (c *step2State) mapKinds() {
	for _, u := range c.order {
		tk := c.tasks[u]
		to := c.newKind(tk.Kind, tk.Parent == nil)
		if to != tk.Kind {
			c.kindCount[[2]string{tk.Kind, to}]++
			tk.Kind = to
			c.dirty[u] = true
		}
	}
}

// placeAll gives every task a place the fixed hierarchy allows, parents first.
// The tasks go in the order of their ranks, so that the ones that move go to
// the end of their new parent in the order they had.
func (c *step2State) placeAll() {
	order := slices.SortedFunc(maps.Keys(c.tasks), func(a, b model.ULID) int {
		return cmp.Or(cmp.Compare(c.tasks[a].Rank, c.tasks[b].Rank), compareULIDs(a, b))
	})
	for _, u := range order {
		c.place(c.tasks[u])
	}
}

const (
	placing = 1
	done    = 2
)

// place moves tk, after its parent, until the hierarchy allows where it is:
//   - a child of a step goes up to the parent of the step;
//   - a story, task or bug below one becomes a step;
//   - a step below a project or an epic, or at the root, becomes a task;
//   - a project or an epic below anything it cannot sit under goes up to the
//     parent of that;
//   - a story, task, bug or epic at the root goes under the 未分類 project of
//     its workspace.
//
// A task whose parent is not a task that was read, or whose parents loop, is
// left where it is.
func (c *step2State) place(tk *model.Task) {
	if c.placed[tk.ULID] != 0 {
		return
	}
	c.placed[tk.ULID] = placing
	defer func() { c.placed[tk.ULID] = done }()
	var parent *model.Task
	if tk.Parent != nil {
		parent = c.tasks[*tk.Parent]
		if parent == nil || c.placed[parent.ULID] == placing {
			return
		}
		c.place(parent)
	}
	for range len(c.tasks) + 3 { // each turn goes up or settles; the bound is for the loops the checks above did not see
		pk := ""
		if parent != nil {
			pk = parent.Kind
		}
		if schema.CanPlace(tk.Kind, pk) {
			return
		}
		switch {
		case pk == "step":
			c.setParent(tk, parent.Parent)
			c.liftStep++
		case isItem(tk.Kind) && isItem(pk):
			tk.Kind = "step"
			c.toStep++
			c.dirty[tk.ULID] = true
			continue
		case tk.Kind == "step":
			tk.Kind = "task"
			c.stepToTask++
			c.dirty[tk.ULID] = true
			continue
		case schema.IsGroup(tk.Kind) && pk != "":
			c.setParent(tk, parent.Parent)
			c.liftGroup++
		default: // a story, task, bug or epic at the root
			u := c.uncategorized(tk.Workspace)
			if u == nil {
				c.unread = append(c.unread, model.TaskPath(tk.ULID)+"（ワークスペースを読めない）")
				return
			}
			c.setParent(tk, &u.ULID)
			c.toUncat[tk.Workspace]++
		}
		parent = nil
		if tk.Parent != nil {
			parent = c.tasks[*tk.Parent]
		}
	}
}

// setParent puts tk at the end of the tasks under parent (nil for the root).
func (c *step2State) setParent(tk *model.Task, parent *model.ULID) {
	tk.Parent = parent
	key := rankKey{ws: tk.Workspace}
	if parent != nil {
		key.parent = *parent
	}
	last, ok := c.last[key]
	if !ok {
		for _, u := range c.order {
			if o := c.tasks[u]; o != tk && o.Workspace == tk.Workspace && sameParent(o.Parent, parent) && o.DeletedAt == nil && o.Rank.Validate() == nil && o.Rank > last {
				last = o.Rank
			}
		}
	}
	rank, err := model.Between(last, "")
	if err != nil { // a rank that cannot be followed: keep the one it has
		rank = tk.Rank
	}
	tk.Rank, c.last[key] = rank, rank
	c.dirty[tk.ULID] = true
}

func sameParent(a, b *model.ULID) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }

// uncategorized is the 未分類 project of the workspace w, made when there is
// none; the same one a sync's repair makes. nil when w cannot be read.
func (c *step2State) uncategorized(w model.ULID) *model.Task {
	ws := c.ws[w]
	if ws == nil {
		return nil
	}
	u, exists := merge.PickUncategorized(ws, c.order, func(u model.ULID) *model.Task { return c.tasks[u] },
		func(u model.ULID) bool { return c.t.Read(model.TaskPath(u)) != nil })
	if exists {
		return c.tasks[u]
	}
	p := merge.UncategorizedTask(u, ws, merge.TodoStatus(c.sc))
	p.Updated = ws.Created // the same file in every clone that converts
	if model.ValidIDPrefix(ws.IDPrefix) == nil {
		c.used[ws.IDPrefix]++
		p.ID = model.FormatTaskID(ws.IDPrefix, c.used[ws.IDPrefix])
	}
	c.tasks[u] = p
	c.placed[u] = done
	c.order = append(c.order, u)
	slices.SortFunc(c.order, compareULIDs)
	c.setParent(p, nil)
	c.dirty[u] = true
	c.madeUncat[w] = p
	return p
}

func (c *step2State) writeTasks() error {
	for _, u := range c.order {
		if !c.dirty[u] {
			continue
		}
		tk := c.tasks[u]
		if c.madeUncat[tk.Workspace] != tk && !c.opt.Now.IsZero() {
			tk.Updated = c.opt.Now
		}
		b, err := tk.Encode()
		if err != nil {
			return fmt.Errorf("%s: %w", model.TaskPath(u), err)
		}
		c.t.Write(model.TaskPath(u), b)
	}
	return nil
}

// convertSchema drops kinds from schema.yaml and moves the default template of
// each kind to templates.default; the kinds of the templates.default of the
// projects go to the fixed ones too.
func (c *step2State) convertSchema() error {
	sc := c.sc
	had := sc.Extra.Get("kinds") != nil
	sc.Extra.Delete("kinds")

	// A kind that is a fixed kind by name goes first, so that when several
	// kinds go to one, its template stays.
	byName := slices.SortedStableFunc(slices.Values(c.file), func(a, b oldKind) int {
		return fixedFirst(a.Name, b.Name)
	})
	var moved []string
	for _, k := range byName {
		if k.Template == "" {
			continue
		}
		to := c.kindWide(k.Name)
		if sc.Templates == nil {
			sc.Templates = &schema.Templates{}
		}
		if sc.Templates.Default == nil {
			sc.Templates.Default = map[string]string{}
		}
		if have, ok := sc.Templates.Default[to]; ok {
			if have != k.Template {
				c.notes = append(c.notes, fmt.Sprintf("種類 %s の既定のテンプレート %s は、%s のものがすでにあるため移さなかった", k.Name, k.Template, to))
			}
			continue
		}
		sc.Templates.Default[to] = k.Template
		moved = append(moved, fmt.Sprintf("%s=%s", to, k.Template))
	}
	var scoped int
	for i := range sc.Scoped {
		if tp := sc.Scoped[i].Templates; tp != nil && len(tp.Default) > 0 {
			out := map[string]string{}
			for _, k := range slices.SortedStableFunc(maps.Keys(tp.Default), fixedFirst) {
				to := c.kindWide(k)
				if _, ok := out[to]; ok {
					c.notes = append(c.notes, fmt.Sprintf("プロジェクトの上書きの種類 %s の既定のテンプレート %s は、%s のものがすでにあるため移さなかった", k, tp.Default[k], to))
					continue
				}
				out[to] = tp.Default[k]
				if to != k {
					scoped++
				}
			}
			tp.Default = out
		}
	}
	if sc.Templates != nil && len(sc.Templates.Only) > 0 {
		// Only of the store was checked and never in effect; Check refuses it now.
		c.notes = append(c.notes, "Store 全体の templates.only を消した（効いていなかった）: "+strings.Join(sc.Templates.Only, ", "))
		sc.Templates.Only = nil
		if len(sc.Templates.Default) == 0 {
			sc.Templates = nil
		}
	}
	if had {
		c.notes = append([]string{"schema.yaml の kinds を消した"}, c.notes...)
	}
	if len(moved) > 0 {
		c.notes = append(c.notes, "kinds の template を templates.default へ移した: "+strings.Join(moved, ", "))
	}
	if scoped > 0 {
		c.notes = append(c.notes, fmt.Sprintf("プロジェクトの上書きの templates.default の種類を固定の種類に直した: %d 件", scoped))
	}
	b, err := sc.Encode()
	if err != nil {
		return fmt.Errorf("%s: %w", schema.Path, err)
	}
	c.t.Write(schema.Path, b)
	return nil
}

// fixedFirst orders the names of the fixed kinds before the others.
func fixedFirst(a, b string) int {
	switch {
	case isFixed(a) && !isFixed(b):
		return -1
	case !isFixed(a) && isFixed(b):
		return 1
	}
	return 0
}

func isFixed(name string) bool {
	_, ok := schema.KindOf(name)
	return ok
}

// convertTemplates rewrites the kind of the template files.
func (c *step2State) convertTemplates() {
	for _, p := range c.t.Paths("templates/") {
		name, ok := strings.CutSuffix(strings.TrimPrefix(p, "templates/"), ".md")
		if !ok || strings.Contains(name, "/") {
			continue
		}
		tp, err := model.DecodeTemplate(name, c.t.Read(p))
		if err != nil {
			c.unread = append(c.unread, p)
			continue
		}
		n := c.rewriteKinds(&tp.Item)
		if n == 0 {
			continue
		}
		b, err := tp.Encode()
		if err != nil {
			c.unread = append(c.unread, p)
			continue
		}
		c.t.Write(p, b)
		c.notes = append(c.notes, fmt.Sprintf("テンプレート %s の種類を固定の種類に直した: %d 件", name, n))
	}
}

// rewriteKinds gives the item and the items under it fixed kinds and counts
// the ones it changed. An item with no kind keeps none.
func (c *step2State) rewriteKinds(it *model.TemplateItem) int {
	n := 0
	if it.Kind != "" {
		if to := c.kindWide(it.Kind); to != it.Kind {
			it.Kind = to
			n++
		}
	}
	for i := range it.Children {
		n += c.rewriteKinds(&it.Children[i])
	}
	return n
}

// lines is the report, one line for each kind of change.
func (c *step2State) lines() []string {
	var out []string
	for _, k := range slices.SortedFunc(maps.Keys(c.kindCount), func(a, b [2]string) int {
		return cmp.Or(cmp.Compare(a[0], b[0]), cmp.Compare(a[1], b[1]))
	}) {
		out = append(out, fmt.Sprintf("種類 %s を %s にした: %d 件", k[0], k[1], c.kindCount[k]))
	}
	add := func(n int, what string) {
		if n > 0 {
			out = append(out, fmt.Sprintf("%s: %d 件", what, n))
		}
	}
	add(c.toStep, "ストーリー・タスク・バグの下のストーリー・タスク・バグを step にした")
	add(c.liftStep, "step の下の子を、いちばん近いストーリー・タスク・バグの直下へ上げた")
	add(c.stepToTask, "プロジェクト・エピックの下かワークスペース直下の step を task にした")
	add(c.liftGroup, "置けない場所のプロジェクト・エピックを、親の親の直下へ上げた")
	for _, w := range slices.SortedFunc(maps.Keys(c.toUncat), compareULIDs) {
		name := w.String()
		if ws := c.ws[w]; ws != nil {
			name = ws.Name
		}
		line := fmt.Sprintf("ワークスペース %s の直下のタスクを「%s」プロジェクトの下へ移した: %d 件", name, merge.Uncategorized, c.toUncat[w])
		if p := c.madeUncat[w]; p != nil {
			line += fmt.Sprintf("（「%s」を作った: %s）", merge.Uncategorized, p.ID)
		}
		out = append(out, line)
	}
	out = append(out, c.notes...)
	for _, p := range c.unread {
		out = append(out, "読めないのでそのままにした: "+p)
	}
	return out
}
