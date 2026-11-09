package convert

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/domain/upgrade"
	"github.com/kukv/hedgehug/internal/app/storage/query"
	"github.com/kukv/hedgehug/internal/app/storage/view"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// Step 2 to 3 fixes the relations. Format 2 kept duplicates, causes and relates
// in props, as properties of type task_ref, and let a store define task_ref
// properties of its own; format 3 has the five relations as fields of the
// task and of the template items. The values of each task move by
// upgrade.TaskRelations, the same function that reads a task of a past commit;
// what is here is what needs the whole store: the names that clash, the values
// that are not ULIDs, the templates, the views, schema.yaml and the report.
// This is the only place that reads the old form of the store's files; the old
// form of a task is read in domain/upgrade.

// relationNames are the names a property of your own may not have in format 3.
var relationNames = upgrade.LegacyRelationNames

// step3 is the step of format 2.
func step3(t *Tree, opt Options) (Report, error) {
	sc, err := schema.Decode(t.Read(schema.Path))
	if err != nil {
		return Report{}, fmt.Errorf("%s: %w", schema.Path, err)
	}
	if err := checkNamesTaken(sc); err != nil {
		return Report{}, err
	}
	c := &step3State{t: t, opt: opt, sc: sc, defs: upgrade.RelationDefsOf(sc), moved: map[string]int{}}
	c.convertTasks()
	c.convertTemplates()
	if err := c.checkValues(); err != nil {
		return Report{}, err
	}
	if err := c.writeTasks(); err != nil {
		return Report{}, err
	}
	c.convertViews()
	if err := c.convertSchema(); err != nil {
		return Report{}, err
	}
	return Report{Lines: c.lines()}, nil
}

// checkNamesTaken stops the conversion when a property that is not a task_ref
// has the name of a relation: its values would sit in the way of the field.
func checkNamesTaken(sc *schema.Schema) error {
	var taken []string
	for _, p := range sc.Props {
		if p.Type != upgrade.TypeTaskRef && slices.Contains(relationNames, p.Name) {
			taken = append(taken, fmt.Sprintf("%s (%s)", p.Name, p.Type))
		}
	}
	if len(taken) == 0 {
		return nil
	}
	return usecase.NewErr(usecase.KindInvalid, "convert.prop_name_taken",
		fmt.Errorf("these properties of schema.yaml have the name of a relation (%s), which are fields of a task in the new format and no longer properties: %s; rename them in schema.yaml and in the props of the tasks, commit, and convert again",
			strings.Join(relationNames, ", "), strings.Join(taken, ", ")),
		"props", taken)
}

type badValue struct {
	where string // the task or the template
	prop  string
	value string
}

type step3State struct {
	t    *Tree
	opt  Options
	sc   *schema.Schema
	defs upgrade.RelationDefs

	tasks   map[string]*model.Task // by path, the ones that changed
	paths   []string               // of tasks, in order
	relates map[model.ULID][]model.ULID
	moved   map[string]int // the values moved, by the name in props
	bad     []badValue
	unread  []string
	notes   []string

	templates map[string]*model.Template // by path, the ones that changed
	tpPaths   []string
	tpMoved   map[string]int // by template
}

// convertTasks moves the values of every task.
func (c *step3State) convertTasks() {
	c.tasks = map[string]*model.Task{}
	c.relates = map[model.ULID][]model.ULID{}
	for _, p := range c.t.Paths("tasks/") {
		if _, err := model.ParseTaskPath(p); err != nil {
			continue
		}
		b := c.t.Read(p)
		if model.IsForward(b) {
			continue
		}
		tk, err := model.DecodeTask(b)
		if err != nil {
			c.unread = append(c.unread, p)
			continue
		}
		moved := upgrade.TaskRelations(tk, c.defs)
		c.relates[tk.ULID] = tk.Relates
		if len(moved) == 0 {
			continue
		}
		c.tasks[p] = tk
		c.paths = append(c.paths, p)
		for _, m := range moved {
			c.moved[m.Prop] += len(m.Values)
			for _, v := range m.Bad {
				c.bad = append(c.bad, badValue{where: taskLabel(tk), prop: m.Prop, value: v})
			}
		}
	}
}

func taskLabel(tk *model.Task) string {
	if tk.ID == "" {
		return tk.ULID.String()
	}
	return tk.ID + " " + tk.Title
}

// convertTemplates moves the values of the items of every template.
func (c *step3State) convertTemplates() {
	c.templates = map[string]*model.Template{}
	c.tpMoved = map[string]int{}
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
		moved := upgrade.ItemRelations(&tp.Item, c.defs)
		if len(moved) == 0 {
			continue
		}
		c.templates[p] = tp
		c.tpPaths = append(c.tpPaths, p)
		for _, m := range moved {
			c.tpMoved[name] += len(m.Values)
			for _, v := range m.Bad {
				c.bad = append(c.bad, badValue{where: "template " + name, prop: m.Prop, value: v})
			}
		}
	}
}

// checkValues stops the conversion at the values that are not a ULID, which
// cannot be a relation, and lists them; with DropBadValues it drops them and
// says so in the report. A value that names no task, or a deleted task, or the
// task itself, is moved as it is, as blocked_by is.
func (c *step3State) checkValues() error {
	if len(c.bad) == 0 {
		return nil
	}
	list := make([]string, len(c.bad))
	for i, b := range c.bad {
		list[i] = fmt.Sprintf("%s: props.%s: %s", b.where, b.prop, b.value)
	}
	if !c.opt.DropBadValues {
		return usecase.NewErr(usecase.KindInvalid, "convert.bad_value",
			fmt.Errorf("these values are not the ULID of a task, so they cannot be relations; fix them in the files, or drop them with --drop-bad-values:\n  %s", strings.Join(list, "\n  ")),
			"values", list)
	}
	for _, l := range list {
		c.notes = append(c.notes, "ULID の形でない値を外した: "+l)
	}
	return nil
}

func (c *step3State) writeTasks() error {
	for _, p := range c.paths {
		b, err := c.tasks[p].Encode()
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		c.t.Write(p, b)
	}
	for _, p := range c.tpPaths {
		b, err := c.templates[p].Encode()
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		c.t.Write(p, b)
	}
	return nil
}

// custom are the task_ref properties of the store's own: the ones whose
// values go to relates under another name.
func (c *step3State) custom() []string {
	var out []string
	for _, n := range c.defs.Names() {
		if to, _, _ := c.defs.Target(n); to == "relates" && n != "relates" {
			out = append(out, n)
		}
	}
	return out
}

// mutualPairs counts the pairs of tasks that each hold the other in relates.
func (c *step3State) mutualPairs() int {
	n := 0
	for a, rs := range c.relates {
		for _, b := range rs {
			if a.String() < b.String() && slices.Contains(c.relates[b], a) {
				n++
			}
		}
	}
	return n
}

// convertViews rewrites the query and the columns of each view.
func (c *step3State) convertViews() {
	custom := c.custom()
	gone := append(slices.Clone(custom), "duplicates", "causes", "relates")
	for _, p := range c.t.Paths(model.ViewsDir + "/") {
		name, ok := model.IsViewPath(p)
		if !ok {
			continue
		}
		v, err := view.Decode(name, c.t.Read(p))
		if err != nil {
			c.unread = append(c.unread, p)
			continue
		}
		changed := false
		for _, term := range ambiguousTerms(v.Query) {
			c.notes = append(c.notes, fmt.Sprintf("ビュー %s の query の %s は、これまで両方向に当たっていたが、これからは付けた側にだけ当たる（書き換えていない。逆方向も含めるなら related_by: も使う）", name, term))
		}
		if q := rewriteQuery(v.Query, custom); q != v.Query {
			c.notes = append(c.notes, fmt.Sprintf("ビュー %s の query を書き換えた: %s → %s", name, v.Query, q))
			v.Query, changed = q, true
		}
		if v.Table != nil {
			var removed []string
			cols := slices.DeleteFunc(slices.Clone(v.Table.Columns), func(col string) bool {
				if slices.Contains(gone, col) {
					removed = append(removed, col)
					return true
				}
				return false
			})
			if len(removed) > 0 {
				v.Table.Columns, changed = cols, true
				c.notes = append(c.notes, fmt.Sprintf("ビュー %s の table.columns から、関連になった %s を外した（関連は列にならない）", name, strings.Join(removed, ", ")))
			}
		}
		if !changed {
			continue
		}
		if b, err := v.Encode(); err != nil {
			c.unread = append(c.unread, p)
		} else {
			c.t.Write(p, b)
		}
	}
}

// rewriteQuery gives the query in the keys of format 3: <relation>.inverse
// becomes the name of the other direction, and a key that was a task_ref of
// the store's own becomes relates (and its .inverse related_by). The spacing
// of what is not rewritten stays.
func rewriteQuery(q string, custom []string) string {
	var out strings.Builder
	quoted, start := false, -1
	flush := func(end int) {
		if start < 0 {
			return
		}
		tok := q[start:end]
		out.WriteString(rewriteTerm(tok, custom))
		start = -1
	}
	for i, r := range q {
		switch {
		case r == '"':
			quoted = !quoted
			if start < 0 {
				start = i
			}
		case unicode.IsSpace(r) && !quoted:
			flush(i)
			out.WriteRune(r)
		default:
			if start < 0 {
				start = i
			}
		}
	}
	flush(len(q))
	return out.String()
}

// rewriteTerm is one term of a query in the keys of format 3; a term that
// does not parse is left as it is.
func rewriteTerm(tok string, custom []string) string {
	pq, err := query.Parse(tok)
	if err != nil || len(pq.Terms) != 1 || pq.Terms[0].Key == "" {
		return tok
	}
	key := pq.Terms[0].Key
	at := 0
	if pq.Terms[0].Neg {
		at = 1
	}
	to := key
	switch name, inv := strings.CutSuffix(key, ".inverse"); {
	case inv && slices.Contains(custom, name):
		to = "related_by"
	case inv:
		if r, other, ok := schema.RelationOf(name); ok && !other {
			to = r.Inverse
		}
	case slices.Contains(custom, key):
		to = "relates"
	}
	return tok[:at] + to + tok[at+len(key):]
}

// ambiguousTerms are the terms of the query (as it was written) whose key is
// relates: in format 2 relates matched in both directions, and it does not now.
func ambiguousTerms(q string) []string {
	pq, err := query.Parse(q)
	if err != nil {
		return nil
	}
	var out []string
	for _, t := range pq.Terms {
		if t.Key == "relates" && !slices.Contains(out, t.Raw) {
			out = append(out, t.Raw)
		}
	}
	return out
}

// convertSchema drops the task_ref properties from schema.yaml, the
// definitions of duplicates, causes and relates among them, and gives
// workflow.required the names of format 3.
func (c *step3State) convertSchema() error {
	sc := c.sc
	var dropped []string
	sc.Props = slices.DeleteFunc(sc.Props, func(p schema.Prop) bool {
		if p.Type == upgrade.TypeTaskRef {
			dropped = append(dropped, p.Name)
			return true
		}
		return false
	})
	custom := c.custom()
	renamed := false
	fix := func(w *schema.Workflow) {
		for i, l := range w.Required {
			var out []string
			for _, f := range l.Values {
				if slices.Contains(custom, f) {
					f, renamed = "relates", true
				}
				if !slices.Contains(out, f) {
					out = append(out, f)
				}
			}
			w.Required[i].Values = out
		}
	}
	fix(&sc.Workflow)
	for i := range sc.Scoped {
		if sc.Scoped[i].Workflow != nil {
			fix(sc.Scoped[i].Workflow)
		}
	}
	if len(dropped) > 0 {
		c.notes = append(c.notes, "schema.yaml の task_ref のプロパティを消した（値は関連の項目へ移した）: "+strings.Join(dropped, ", "))
	}
	if renamed {
		c.notes = append(c.notes, "workflow.required の独自の task_ref の名前を relates に直した")
	}
	b, err := sc.Encode()
	if err != nil {
		return fmt.Errorf("%s: %w", schema.Path, err)
	}
	c.t.Write(schema.Path, b)
	return nil
}

// lines is the report.
func (c *step3State) lines() []string {
	var out []string
	var own int
	for _, n := range relationNames {
		own += c.moved[n]
	}
	if own > 0 {
		out = append(out, fmt.Sprintf("props の %s の値を、タスクの同名の項目へ移した（ULID の関連）: %d 件", strings.Join(relationNames, "・"), own))
	}
	for _, n := range c.custom() {
		out = append(out, fmt.Sprintf("%s: %d 件を relates へ", n, c.moved[n]))
	}
	if n := c.mutualPairs(); n > 0 {
		out = append(out, fmt.Sprintf("相互の組 %d 件（A に B・B に A の relates を、どちらも向きありの relates として残した）", n))
	}
	for _, p := range c.tpPaths {
		name := strings.TrimSuffix(strings.TrimPrefix(p, "templates/"), ".md")
		out = append(out, fmt.Sprintf("テンプレート %s の関連を項目へ移した: %d 件", name, c.tpMoved[name]))
	}
	out = append(out, c.notes...)
	for _, p := range c.unread {
		out = append(out, "読めないのでそのままにした: "+p)
	}
	return out
}
