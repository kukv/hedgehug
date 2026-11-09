package convert

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/domain/upgrade"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// Step 3 to 4 fixes the built-in properties. Format 3 kept priority, start,
// due, estimate and points in props, as properties the store defined, and held
// labels as a free list; format 4 has the five as typed fields of the task and
// of the template items, priority from the list priorities:, and labels from the
// list labels:. The values of each task move by upgrade.TaskBuiltins, the same
// function that reads a task of a past commit; what is here is what needs the
// whole store: the values that cannot be moved, the templates, schema.yaml, the
// traces of the GitHub label sync in automations.yaml and the report. The old
// form of a task is read in domain/upgrade.

// builtinBad is a value that is not moved, with the task or template it is in.
type builtinBad struct {
	where string
	upgrade.BadValue
}

// labelStat counts what the labels that the list lacks were on.
type labelStat struct {
	name             string // as first written
	tasks, templates int
}

type step4State struct {
	t    *Tree
	opt  Options
	sc   *schema.Schema
	defs upgrade.BuiltinDefs

	tasks     map[string]*model.Task // by path, the ones that changed
	paths     []string
	templates map[string]*model.Template
	tpPaths   []string

	moved   int
	tpMoved map[string]int // by template
	github  int            // tasks and templates that lost a trace of the label sync
	bad     []builtinBad
	cut     []builtinBad // dates that lost a time other than 00:00
	labels  map[string]*labelStat
	unread  []string
	notes   []string
}

// step4 is the step of format 3.
func step4(t *Tree, opt Options) (Report, error) {
	sc, err := schema.Decode(t.Read(schema.Path))
	if err != nil {
		return Report{}, fmt.Errorf("%s: %w", schema.Path, err)
	}
	defs, err := upgrade.BuiltinDefsOf(t.Read(schema.Path))
	if err != nil {
		return Report{}, err
	}
	c := &step4State{t: t, opt: opt, sc: sc, defs: defs, tpMoved: map[string]int{}, labels: map[string]*labelStat{}}
	c.convertTasks()
	c.convertTemplates()
	if err := c.checkValues(); err != nil {
		return Report{}, err
	}
	if err := c.writeTasks(); err != nil {
		return Report{}, err
	}
	c.convertAutomations()
	if err := c.convertSchema(); err != nil {
		return Report{}, err
	}
	return Report{Lines: c.lines()}, nil
}

// take adds what became of one task or template to the state.
func (c *step4State) take(where string, r upgrade.Builtins, template bool) {
	for _, b := range r.Bad {
		c.bad = append(c.bad, builtinBad{where, b})
	}
	for _, b := range r.Time {
		c.cut = append(c.cut, builtinBad{where, b})
	}
	if r.GitHub {
		c.github++
	}
	seen := map[string]bool{}
	for _, l := range r.Labels {
		key := strings.ToLower(l)
		st := c.labels[key]
		if st == nil {
			st = &labelStat{name: l}
			c.labels[key] = st
		}
		if seen[key] && !template {
			continue // a label twice on a task is one
		}
		seen[key] = true
		if template {
			st.templates++
		} else {
			st.tasks++
		}
	}
}

// convertTasks moves the values of every task.
func (c *step4State) convertTasks() {
	c.tasks = map[string]*model.Task{}
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
		r := upgrade.TaskBuiltins(tk, c.defs)
		c.take(taskLabel(tk), r, false)
		if !r.Changed() {
			continue
		}
		c.moved += len(r.Moved)
		c.tasks[p] = tk
		c.paths = append(c.paths, p)
	}
}

// convertTemplates moves the values of the items of every template.
func (c *step4State) convertTemplates() {
	c.templates = map[string]*model.Template{}
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
		r := upgrade.ItemBuiltins(&tp.Item, c.defs)
		c.take("template "+name, r, true)
		if !r.Changed() {
			continue
		}
		c.tpMoved[name] += len(r.Moved)
		c.templates[p] = tp
		c.tpPaths = append(c.tpPaths, p)
	}
}

// listed is how a value that is not moved reads in a list.
func (b builtinBad) listed() string {
	s := fmt.Sprintf("%s: props.%s: %s", b.where, b.Prop, b.Value)
	if b.Twice {
		s += fmt.Sprintf("（トップレベルの %s にも値がある）", b.Prop)
	}
	return s
}

// checkValues stops the conversion at the values that are not of the type of
// their field, and at those the task holds in the field too, and lists them;
// with DropBadValues it drops them (a value that cannot be read goes; of a
// value held twice, the one of the field goes and the one of props moves) and
// says so in the report.
func (c *step4State) checkValues() error {
	if len(c.bad) == 0 {
		return nil
	}
	list := make([]string, len(c.bad))
	for i, b := range c.bad {
		list[i] = b.listed()
	}
	if !c.opt.DropBadValues {
		return usecase.NewErr(usecase.KindInvalid, "convert.bad_value",
			fmt.Errorf("these values cannot be the built-in priority, start, due, estimate or points: they are not of the type of the field, or the task has the field at the top level too; fix them in the files, or drop them with --drop-bad-values (the value of props is kept for a field that is twice, and the one at the top level is dropped):\n  %s", strings.Join(list, "\n  ")),
			"values", list)
	}
	for _, b := range c.bad {
		if b.Twice {
			c.notes = append(c.notes, "トップレベルの値を外し、props の値を移した: "+b.listed())
		} else {
			c.notes = append(c.notes, "読めない値を外した: "+b.listed())
		}
	}
	return nil
}

func (c *step4State) writeTasks() error {
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

// convertAutomations takes labels out of what issue_fields pushes and mirrors:
// the labels of an issue are not synced any more.
func (c *step4State) convertAutomations() {
	raw := c.t.Read(model.AutomationsPath)
	if raw == nil {
		return
	}
	f, err := automation.Decode(raw)
	if err != nil {
		c.unread = append(c.unread, model.AutomationsPath)
		return
	}
	drop := func(l []string) []string {
		return slices.DeleteFunc(slices.Clone(l), func(n string) bool { return n == "labels" })
	}
	push, mirror := drop(f.IssueFields.Push), drop(f.IssueFields.Mirror)
	if len(push) == len(f.IssueFields.Push) && len(mirror) == len(f.IssueFields.Mirror) {
		return
	}
	f.IssueFields.Push, f.IssueFields.Mirror = push, mirror
	b, err := f.Encode()
	if err != nil {
		c.unread = append(c.unread, model.AutomationsPath)
		return
	}
	c.t.Write(model.AutomationsPath, b)
	c.notes = append(c.notes, "automations.yaml の issue_fields の push・mirror から labels を外した（GitHub のラベルは同期しない）")
}

// convertSchema gives schema.yaml the list priorities: it was lacking, and
// drops the properties of the names that are fields now.
func (c *step4State) convertSchema() error {
	sc := c.sc
	changed := false
	if !slices.Equal(sc.Priorities, c.defs.Priorities) {
		changed = true
		sc.Priorities = slices.Clone(c.defs.Priorities)
		c.notes = append(c.notes, "schema.yaml の priorities: を priority の定義の values から作った: "+strings.Join(sc.Priorities, ", "))
	}
	var dropped []string
	sc.Props = slices.DeleteFunc(sc.Props, func(p schema.Prop) bool {
		if slices.Contains(upgrade.LegacyBuiltinNames, p.Name) {
			dropped = append(dropped, p.Name)
			return true
		}
		return false
	})
	if len(dropped) > 0 {
		changed = true
		c.notes = append(c.notes, "schema.yaml の組み込みと同じ名前のプロパティを消した（値は同名の項目へ移した）: "+strings.Join(dropped, ", "))
	}
	if !changed {
		return nil
	}
	b, err := sc.Encode()
	if err != nil {
		return fmt.Errorf("%s: %w", schema.Path, err)
	}
	c.t.Write(schema.Path, b)
	return nil
}

// lines is the report.
func (c *step4State) lines() []string {
	var out []string
	if c.moved > 0 {
		out = append(out, fmt.Sprintf("props の %s の値を、タスクの同名の項目へ移した: %d 件", strings.Join(upgrade.LegacyBuiltinNames, "・"), c.moved))
	}
	for _, p := range c.tpPaths {
		name := strings.TrimSuffix(strings.TrimPrefix(p, "templates/"), ".md")
		if n := c.tpMoved[name]; n > 0 {
			out = append(out, fmt.Sprintf("テンプレート %s の項目へ移した: %d 件", name, n))
		}
	}
	for _, b := range c.cut {
		out = append(out, fmt.Sprintf("日付にして時刻を落とした: %s: props.%s: %s", b.where, b.Prop, b.Value))
	}
	for _, key := range slices.Sorted(maps.Keys(c.labels)) {
		st := c.labels[key]
		out = append(out, fmt.Sprintf("ラベル %s を外した（schema.yaml の labels: にない）: タスク %d 件・テンプレート %d 件", st.name, st.tasks, st.templates))
	}
	if c.github > 0 {
		out = append(out, fmt.Sprintf("GitHub のラベルの同期の跡（gh.issues[].synced と links の fields の labels）を外した: %d 件", c.github))
	}
	out = append(out, c.notes...)
	for _, p := range c.unread {
		out = append(out, "読めないのでそのままにした: "+p)
	}
	return out
}
