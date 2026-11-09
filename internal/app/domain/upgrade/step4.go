package upgrade

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

// Format 3 kept priority, start, due, estimate and points in props, as
// properties the schema.yaml of the store defined (of any type), and held
// labels as a free list; format 4 has the five as fields of the task and of the
// template items, typed (a priority of the list priorities:, two dates, a
// duration, a number), and labels as a list the schema gives (labels:). What
// one task decides in that change is where the five go, what becomes of a value
// that is not of the type, which labels the list does not have, and the traces
// of the GitHub label sync that are gone.

// LegacyBuiltinNames are the names that a task of format 3 could hold in props
// and that are fields of the task in format 4.
var LegacyBuiltinNames = []string{"priority", "start", "due", "estimate", "points"}

// BuiltinDefs is what the schema.yaml of format 3 says that the move needs:
// the values of priority, and the labels there are.
type BuiltinDefs struct {
	Priorities []string
	Labels     []string
	keep       bool // leave what cannot be moved in props

	// What was made of the lists to give a schema of format 4 that checks.
	PrioritiesFrom   string   // list, definition or default: where Priorities come from
	PrioritiesEmpty  bool     // priorities: was there and empty, so the next source was used
	PrioritiesFolded []string // spellings that differ from an earlier one in case only, and were folded into it
	LabelsFolded     []string
}

// foldList keeps the first spelling of each value, comparing without regard to
// case, and returns the spellings it left out.
func foldList(in []string) (out, folded []string) {
	for _, v := range in {
		if _, ok := schema.MatchValue(out, v); ok {
			folded = append(folded, v)
			continue
		}
		out = append(out, v)
	}
	return out, folded
}

// KeepingBad is d for what reads a task of a past commit: a value that cannot be
// moved (it is not of the type, or the task has the field already) stays in
// props under its own name, where nothing is lost and doctor finds it (hh store
// convert drops it only when it is told to).
func (d BuiltinDefs) KeepingBad() BuiltinDefs {
	d.keep = true
	return d
}

// BuiltinDefsOf reads the schema.yaml of format 3, which may be nil (a store
// without one). The priorities are the priorities: it has, else the values of
// a property priority of type enum, else the default (an empty priorities: is
// as if it was not there); the labels are its labels:, none without. Values
// that differ in case only are folded into the one written first, which format
// 4 requires of the lists. The file is read as it is, for it still holds the
// property definitions that the schema no longer keeps.
func BuiltinDefsOf(raw []byte) (BuiltinDefs, error) {
	d := BuiltinDefs{Priorities: slices.Clone(schema.DefaultPriorities), PrioritiesFrom: "default"}
	if raw == nil {
		return d, nil
	}
	var doc struct {
		Priorities *[]string `yaml:"priorities"`
		Labels     []string  `yaml:"labels"`
		Props      map[string]struct {
			Type   string   `yaml:"type"`
			Values []string `yaml:"values"`
		} `yaml:"props"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return BuiltinDefs{}, fmt.Errorf("%s: %w", schema.Path, err)
	}
	d.Labels, d.LabelsFolded = foldList(doc.Labels)
	d.PrioritiesEmpty = doc.Priorities != nil && len(*doc.Priorities) == 0
	src := d.Priorities
	switch p := doc.Props["priority"]; {
	case doc.Priorities != nil && len(*doc.Priorities) > 0:
		src, d.PrioritiesFrom = *doc.Priorities, "list"
	case schema.PropType(p.Type) == schema.TypeEnum && len(p.Values) > 0:
		src, d.PrioritiesFrom = p.Values, "definition"
	}
	d.Priorities, d.PrioritiesFolded = foldList(src)
	return d, nil
}

// BadValue is a value of a built-in property that cannot be moved.
type BadValue struct {
	Prop  string
	Value string // as written in props
	// Twice is set when the value is readable but the field of the task has a
	// value already, so that which of the two is meant cannot be told.
	Twice bool
	// Field is the value the field of the task has, as written, when Twice.
	Field string
}

// Builtins is what became of the built-in properties of one task or of a
// template item and the items under it.
type Builtins struct {
	Moved  []string   // the names that were moved from props, in order
	Bad    []BadValue // the values that were not moved
	Time   []BadValue // the dates that lost a time other than 00:00: Value is the value as written
	Labels []string   // the labels that the list does not have, as written
	GitHub bool       // a label of the GitHub sync was taken out
	Links  []string   // refs of the links that synced labels alone: they are not synced now

	changed bool // the file is not what it was
}

// Changed is whether anything in the task was rewritten. A value that could not
// be moved and stays in props is not a rewrite.
func (b Builtins) Changed() bool { return b.changed }

// builtinTarget is the fields of a task or of a template item that format 4
// adds, so that the move is written once.
type builtinTarget struct {
	priority *string
	start    **model.Date
	due      **model.Date
	estimate **model.Duration
	points   **float64
	labels   *[]string
	props    *model.Fields
	extra    *model.Fields // the keys of the file this version does not read; nil for a template item
	links    *[]model.Link
}

func targetOfTask(t *model.Task) builtinTarget {
	return builtinTarget{&t.Priority, &t.Start, &t.Due, &t.Estimate, &t.Points, &t.Labels, &t.Props, &t.Extra, &t.Links}
}

func targetOfItem(it *model.TemplateItem) builtinTarget {
	return builtinTarget{&it.Priority, &it.Start, &it.Due, &it.Estimate, &it.Points, &it.Labels, &it.Props, nil, &it.Links}
}

// has is whether the field called name has a value already, or the file has
// the key with a value this version cannot read.
func (g builtinTarget) has(name string) bool {
	if g.extra != nil && g.extra.Get(name) != nil {
		return true
	}
	switch name {
	case "priority":
		return *g.priority != ""
	case "start":
		return *g.start != nil
	case "due":
		return *g.due != nil
	case "estimate":
		return *g.estimate != nil
	}
	return *g.points != nil
}

// text is the value of the field called name as it reads in a list: the key of
// the file when this version cannot read it.
func (g builtinTarget) text(name string) string {
	if g.extra != nil {
		if n := g.extra.Get(name); n != nil {
			return nodeText(n)
		}
	}
	switch name {
	case "priority":
		return *g.priority
	case "start":
		return (*g.start).String()
	case "due":
		return (*g.due).String()
	case "estimate":
		return (*g.estimate).String()
	}
	return strconv.FormatFloat(**g.points, 'f', -1, 64)
}

// set puts the value of the property name, which is readable, in the field.
func (g builtinTarget) set(name string, v builtinValue) {
	if g.extra != nil {
		g.extra.Delete(name)
	}
	switch name {
	case "priority":
		*g.priority = v.text
	case "start":
		*g.start = &v.date
	case "due":
		*g.due = &v.date
	case "estimate":
		*g.estimate = &v.dur
	default:
		*g.points = &v.num
	}
}

// builtinValue is a value of props read as the type of its field.
type builtinValue struct {
	text string
	date model.Date
	dur  model.Duration
	num  float64
	cut  bool // the date had a time other than 00:00, which was left out
}

// readBuiltin reads the property name, written in props as n, as the type of
// its field. empty is a value that holds nothing (null, an empty string, an
// empty list). ok is false for a value that is not of the type and cannot be
// made one without losing something.
func readBuiltin(name string, n *yaml.Node, d BuiltinDefs) (v builtinValue, empty, ok bool) {
	n, kind := single(n)
	switch kind {
	case valueEmpty:
		return v, true, true
	case valueMany:
		return v, false, false
	}
	s := n.Value
	var err error
	switch name {
	case "priority":
		var spelled string
		if spelled, ok = schema.MatchValue(d.Priorities, s); ok {
			v.text = spelled
		}
		return v, false, ok
	case "start", "due":
		v.date, v.cut, err = dateOf(s)
	case "estimate":
		v.dur, err = model.ParseDuration(s)
	default:
		v.num, err = strconv.ParseFloat(s, 64)
		if err == nil && (math.IsNaN(v.num) || math.IsInf(v.num, 0)) {
			err = fmt.Errorf("not finite")
		}
	}
	return v, false, err == nil
}

const (
	valueOne = iota
	valueEmpty
	valueMany
)

// single is the one scalar a value of props holds: the value itself, or the
// item of a list of one. valueEmpty is null, an empty string or an empty list;
// valueMany is anything that is not one scalar.
func single(n *yaml.Node) (*yaml.Node, int) {
	for n.Kind == yaml.SequenceNode {
		items := valueNodes(n)
		switch len(items) {
		case 0:
			return n, valueEmpty
		case 1:
			n = items[0]
		default:
			return n, valueMany
		}
	}
	switch {
	case n.Kind != yaml.ScalarNode:
		return n, valueMany
	case n.ShortTag() == "!!null" || n.Value == "":
		return n, valueEmpty
	}
	return n, valueOne
}

// dateOf reads a date, or a time of RFC 3339 as the day it is written in, in the
// offset it is written with. cut is whether the time was other than 00:00:00.
func dateOf(s string) (d model.Date, cut bool, err error) {
	if d, err = model.ParseDate(s); err == nil {
		return d, false, nil
	}
	tm, terr := time.Parse(time.RFC3339, s)
	if terr != nil {
		return model.Date{}, false, err
	}
	d, err = model.ParseDate(tm.Format(time.DateOnly))
	return d, tm.Hour() != 0 || tm.Minute() != 0 || tm.Second() != 0 || tm.Nanosecond() != 0, err
}

// moveBuiltins does what one task or template item decides of the move of the
// five properties, and adds what became of it to out.
func moveBuiltins(g builtinTarget, d BuiltinDefs, out *Builtins) {
	for _, name := range LegacyBuiltinNames {
		n := g.props.Get(name)
		if n == nil {
			continue
		}
		v, empty, ok := readBuiltin(name, n, d)
		if empty {
			g.props.Delete(name)
			out.changed = true
			continue
		}
		text := nodeText(n)
		twice := ok && g.has(name)
		if !ok || twice {
			b := BadValue{Prop: name, Value: text, Twice: twice}
			if twice {
				b.Field = g.text(name)
			}
			out.Bad = append(out.Bad, b)
			if d.keep {
				continue // stays in props, and the field keeps its value
			}
		}
		out.changed = true
		g.props.Delete(name)
		if !ok { // a value that cannot be read goes with the property
			continue
		}
		g.set(name, v)
		out.Moved = append(out.Moved, name)
		if v.cut {
			out.Time = append(out.Time, BadValue{Prop: name, Value: text})
		}
	}
}

// filterLabels keeps the labels that the list has, spelled as the list has
// them, once each, and says which it took out.
func filterLabels(labels *[]string, list []string, out *Builtins) {
	if len(*labels) == 0 {
		return
	}
	kept := make([]string, 0, len(*labels))
	for _, l := range *labels {
		spelled, ok := schema.MatchValue(list, l)
		switch {
		case !ok:
			out.Labels = append(out.Labels, l)
		case !slices.Contains(kept, spelled):
			kept = append(kept, spelled)
		}
	}
	if slices.Equal(kept, *labels) {
		return
	}
	out.changed = true
	if len(kept) == 0 {
		kept = nil
	}
	*labels = kept
}

// dropLabelFields takes labels out of the fields of the links, which is what
// the sync of a link pushed or mirrored. A link that synced labels alone would
// be left with no fields, which means the default ones, so it is made a link
// that does not sync (sync: link), and is told in out.Links.
func dropLabelFields(links *[]model.Link, out *Builtins) {
	for i := range *links {
		l := &(*links)[i]
		if !slices.Contains(l.Fields, "labels") {
			continue
		}
		l.Fields = slices.DeleteFunc(slices.Clone(l.Fields), func(f string) bool { return f == "labels" })
		if len(l.Fields) == 0 {
			l.Fields = nil
			if l.Sync == model.SyncPush || l.Sync == model.SyncMirror {
				l.Sync = model.SyncLink
				out.Links = append(out.Links, l.Ref)
			}
		}
		out.GitHub, out.changed = true, true
	}
}

// dropSyncedLabels takes labels out of what the issues in gh were last made
// the same as.
func dropSyncedLabels(gh *yaml.Node, out *Builtins) {
	if gh == nil || gh.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(gh.Content); i += 2 {
		if gh.Content[i].Value != "issues" || gh.Content[i+1].Kind != yaml.SequenceNode {
			continue
		}
		for _, issue := range gh.Content[i+1].Content {
			if issue.Kind != yaml.MappingNode {
				continue
			}
			for j := 0; j+1 < len(issue.Content); j += 2 {
				synced := issue.Content[j+1]
				if issue.Content[j].Value != "synced" || synced.Kind != yaml.MappingNode {
					continue
				}
				for k := 0; k+1 < len(synced.Content); k += 2 {
					if synced.Content[k].Value == "labels" {
						synced.Content = slices.Delete(synced.Content, k, k+2)
						out.GitHub, out.changed = true, true
						break
					}
				}
			}
		}
	}
}

// TaskBuiltins does for t what the change to format 4 decides of one task: the
// five built-in properties move out of props to the fields, labels the list
// does not have go, and the traces of the GitHub label sync (the labels of
// gh.issues[].synced and of the fields of the links) go. The moves are told in
// the result.
func TaskBuiltins(t *model.Task, d BuiltinDefs) Builtins {
	var out Builtins
	moveBuiltins(targetOfTask(t), d, &out)
	filterLabels(&t.Labels, d.Labels, &out)
	dropLabelFields(&t.Links, &out)
	dropSyncedLabels(t.GH, &out)
	return out
}

// ItemBuiltins is TaskBuiltins for a template item and, one after the other,
// the items under it (the labels of a template come out as those of a task do;
// a template has no gh). The result of the item comes before those of its
// children.
func ItemBuiltins(it *model.TemplateItem, d BuiltinDefs) Builtins {
	var out Builtins
	moveBuiltins(targetOfItem(it), d, &out)
	filterLabels(&it.Labels, d.Labels, &out)
	dropLabelFields(&it.Links, &out)
	for i := range it.Children {
		c := ItemBuiltins(&it.Children[i], d)
		out.Moved = append(out.Moved, c.Moved...)
		out.Bad = append(out.Bad, c.Bad...)
		out.Time = append(out.Time, c.Time...)
		out.Labels = append(out.Labels, c.Labels...)
		out.GitHub = out.GitHub || c.GitHub
		out.Links = append(out.Links, c.Links...)
		out.changed = out.changed || c.changed
	}
	return out
}

// task3to4 moves the five built-in properties of a task out of props and does
// the rest of what one task decides. A value that cannot be moved stays in
// props under its own name (and doctor tells of it, as of any property left
// there), so that reading a past version loses nothing.
func task3to4(raw []byte, cx Cx) ([]byte, error) {
	t, err := model.DecodeTask(raw)
	if err != nil {
		return nil, err
	}
	d, err := BuiltinDefsOf(cx.Schema)
	if err != nil {
		return nil, err
	}
	if !TaskBuiltins(t, d.KeepingBad()).Changed() {
		return raw, nil
	}
	return t.Encode()
}

// template3to4 is task3to4 for the items of a template.
func template3to4(name string, raw []byte, cx Cx) ([]byte, error) {
	tp, err := model.DecodeTemplate(name, raw)
	if err != nil {
		return nil, err
	}
	d, err := BuiltinDefsOf(cx.Schema)
	if err != nil {
		return nil, err
	}
	if !ItemBuiltins(&tp.Item, d.KeepingBad()).Changed() {
		return raw, nil
	}
	return tp.Encode()
}

// LegacyBuiltinConflicts are the names of the five that t holds in props, as a
// property that format 4 does not have, and in the field of its own too: which
// of the two is newer cannot be told, so neither is moved.
func LegacyBuiltinConflicts(t *model.Task) []string {
	g := targetOfTask(t)
	var out []string
	for _, name := range LegacyBuiltinNames {
		if n := t.Props.Get(name); n != nil && g.has(name) && !emptyValue(n) {
			out = append(out, name)
		}
	}
	return out
}

// emptyValue is whether a value of props holds nothing: null, an empty string, an
// empty list.
func emptyValue(n *yaml.Node) bool {
	_, kind := single(n)
	return kind == valueEmpty
}

// LegacyBuiltinBadValues are the names of the five that t holds in props with a
// value that is not of the type of its field (a priority is checked against
// priorities).
func LegacyBuiltinBadValues(t *model.Task, priorities []string) []string {
	var out []string
	for _, name := range LegacyBuiltinNames {
		n := t.Props.Get(name)
		if n == nil {
			continue
		}
		if _, _, ok := readBuiltin(name, n, BuiltinDefs{Priorities: priorities}); !ok {
			out = append(out, name)
		}
	}
	return out
}

// MoveLegacyBuiltins moves the five built-in properties that a task of the newest
// format still holds in props, as an older hedgehug writes them, to the fields
// of the task. It is what hh doctor --fix does; priorities is the list of the
// schema of the store. A task with a value that cannot be read, or whose field
// has a value already (LegacyBuiltinConflicts), comes back as it is: neither
// can be moved without losing something. It touches nothing else: not labels,
// not gh. The dates that lost a time are returned, as written, for the caller
// to tell of.
func MoveLegacyBuiltins(raw []byte, priorities []string) ([]byte, []BadValue, error) {
	t, err := model.DecodeTask(raw)
	if err != nil {
		return nil, nil, err
	}
	if len(LegacyBuiltinConflicts(t)) > 0 || len(LegacyBuiltinBadValues(t, priorities)) > 0 {
		return raw, nil, nil
	}
	var out Builtins
	moveBuiltins(targetOfTask(t), BuiltinDefs{Priorities: priorities, keep: true}, &out)
	if !out.changed {
		return raw, nil, nil
	}
	b, err := t.Encode()
	return b, out.Time, err
}

// BuiltinText reads n, a value that a task of format 3 held in props under the
// name of a built-in property (a priority, start, due, estimate or points), as
// the value of the field, fixing its type as the step to format 4 does, and
// gives it as hedgehug set reads it. priorities is the list of the schema now.
// empty is a value that holds nothing, which clears the field; ok is false for
// one that cannot be read as the field's type.
func BuiltinText(name string, n *yaml.Node, priorities []string) (text string, empty, ok bool) {
	v, empty, ok := readBuiltin(name, n, BuiltinDefs{Priorities: priorities})
	if empty || !ok {
		return "", empty, ok
	}
	switch name {
	case "priority":
		text = v.text
	case "start", "due":
		text = v.date.String()
	case "estimate":
		text = v.dur.String()
	default:
		text = strconv.FormatFloat(v.num, 'f', -1, 64)
	}
	return text, false, true
}
