package usecase

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/config"
)

// Assign is one key=value of set and of add -p: = sets (an empty value
// clears), += adds to a list and -= removes from it.
type Assign struct {
	Key   string
	Op    string // "=", "+=" or "-="
	Value string
}

// ParseAssign reads s as key=value, key+=value or key-=value.
func ParseAssign(s string) (Assign, error) {
	i := strings.IndexByte(s, '=')
	if i < 0 {
		return Assign{}, NewErr(KindUsage, "assign.form", fmt.Errorf("%q: want key=value, key+=value or key-=value", s), "input", s)
	}
	a := Assign{Key: s[:i], Op: "=", Value: s[i+1:]}
	if k, ok := strings.CutSuffix(a.Key, "+"); ok {
		a.Key, a.Op = k, "+="
	} else if k, ok := strings.CutSuffix(a.Key, "-"); ok {
		a.Key, a.Op = k, "-="
	}
	if a.Key == "" {
		return Assign{}, NewErr(KindUsage, "assign.form", fmt.Errorf("%q: the key is missing", s), "input", s)
	}
	return a, nil
}

// ParseAssigns reads each of ss with ParseAssign.
func ParseAssigns(ss []string) ([]Assign, error) {
	out := make([]Assign, 0, len(ss))
	for _, s := range ss {
		a, err := ParseAssign(s)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// ApplyAssign changes t as a says. Built-in fields come first; any other key
// must be a property of sc, and its value is read by the property's type.
// resolve turns task names into ULIDs, for the relations.
// Whether the result fits the schema is the store's to check.
func ApplyAssign(t *model.Task, a Assign, sc *schema.Schema, resolve func(string) (model.ULID, error)) error {
	switch a.Key {
	case "title":
		return setScalar(&t.Title, a)
	case "kind":
		return setScalar(&t.Kind, a)
	case "status":
		return setScalar(&t.Status, a)
	case "assignees":
		return setList(&t.Assignees, a)
	case "watchers":
		return setList(&t.Watchers, a)
	case "labels":
		return setLabels(&t.Labels, a, sc.Labels)
	case "priority", "start", "due", "estimate", "points":
		return setTyped(t, a, sc)
	case "repos":
		return setList(&t.Repos, a)
	case "parent", "rank":
		return NewErr(KindInvalid, "assign.readonly", fmt.Errorf("%s cannot be set; use hedgehug mv", a.Key), "key", a.Key)
	}
	if rel, inverse, ok := schema.RelationOf(a.Key); ok {
		if inverse {
			return NewErr(KindInvalid, "assign.readonly", fmt.Errorf("%s cannot be set; set %s on the other task", a.Key, rel.Name), "key", a.Key)
		}
		list := slices.Clone(rel.Get(t))
		err := setULIDs(&list, a, resolve)
		rel.Set(t, list)
		return err
	}
	if slices.Contains(model.TaskKeys(), a.Key) {
		return NewErr(KindInvalid, "assign.readonly", fmt.Errorf("%s cannot be set", a.Key), "key", a.Key)
	}
	p, ok := sc.Prop(a.Key)
	if !ok {
		return NewErr(KindInvalid, "assign.unknown_key", fmt.Errorf("%s is not a field or a property of the schema", a.Key), "key", a.Key)
	}
	return setProp(t, p, a)
}

// WatchNewAssignees adds to the watchers of t the assignees it has now and
// did not in before.
func WatchNewAssignees(t *model.Task, before []string) {
	for _, as := range t.Assignees {
		if !slices.ContainsFunc(before, func(b string) bool { return config.SameAccount(b, as) }) {
			AddWatchers(t, as)
		}
	}
}

func setScalar(dst *string, a Assign) error {
	if a.Op != "=" {
		return NewErr(KindInvalid, "assign.single", fmt.Errorf("%s holds one value; use %s=<value>", a.Key, a.Key), "key", a.Key)
	}
	*dst = a.Value
	return nil
}

// SpellAsListed gives the priority and the labels of t the spelling of the
// lists of sc, which match them without regard to case; one the list lacks is
// left as it is, for the check of the schema to refuse.
func SpellAsListed(t *model.Task, sc *schema.Schema) {
	if v, ok := schema.MatchValue(sc.Priorities, t.Priority); ok {
		t.Priority = v
	}
	t.Labels = slices.Clone(t.Labels)
	for i, l := range t.Labels {
		if v, ok := schema.MatchValue(sc.Labels, l); ok {
			t.Labels[i] = v
		}
	}
}

// setLabels is setList for labels, which are matched with the list of the
// schema without regard to case: the spelling of the list is the one stored,
// and -= takes away the label whatever the case it is written in. A label the
// list lacks is kept as written, for the store to refuse.
func setLabels(dst *[]string, a Assign, list []string) error {
	if v, ok := schema.MatchValue(list, a.Value); ok {
		a.Value = v
	}
	if a.Op == "-=" {
		if i := slices.IndexFunc(*dst, func(v string) bool { return strings.EqualFold(v, a.Value) }); i >= 0 {
			a.Value = (*dst)[i]
		}
	}
	return setList(dst, a)
}

// setTyped sets the built-in priority, start, due, estimate or points. An
// empty value clears it, and with it a value the file has that could not be
// read.
func setTyped(t *model.Task, a Assign, sc *schema.Schema) error {
	if a.Op != "=" {
		return NewErr(KindInvalid, "assign.single", fmt.Errorf("%s holds one value; use %s=<value>", a.Key, a.Key), "key", a.Key)
	}
	if a.Value == "" {
		switch a.Key {
		case "priority":
			t.Priority = ""
		case "start":
			t.Start = nil
		case "due":
			t.Due = nil
		case "estimate":
			t.Estimate = nil
		case "points":
			t.Points = nil
		}
		t.Extra.Delete(a.Key)
		return nil
	}
	switch a.Key {
	case "priority":
		t.Priority = a.Value
		if v, ok := schema.MatchValue(sc.Priorities, a.Value); ok {
			t.Priority = v
		}
	case "start", "due":
		d, err := model.ParseDate(a.Value)
		if err != nil {
			return badAssign(a.Key, a.Value, "a date (YYYY-MM-DD)")
		}
		if a.Key == "start" {
			t.Start = &d
		} else {
			t.Due = &d
		}
	case "estimate":
		d, err := model.ParseDuration(a.Value)
		if err != nil {
			return badAssign(a.Key, a.Value, "a duration (like 2h30m)")
		}
		t.Estimate = &d
	case "points":
		f, err := strconv.ParseFloat(a.Value, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return badAssign(a.Key, a.Value, "a number")
		}
		t.Points = &f
	}
	return nil
}

func badAssign(name, v, what string) error {
	return NewErr(KindInvalid, "assign.bad_value", fmt.Errorf("%s: %q is not %s", name, v, what), "key", name, "value", v)
}

func setList(dst *[]string, a Assign) error {
	switch {
	case a.Op == "=" && a.Value == "":
		*dst = nil
	case a.Value == "":
		return NewErr(KindInvalid, "assign.missing_value", fmt.Errorf("%s%s: the value is missing", a.Key, a.Op), "key", a.Key)
	case a.Op == "=":
		*dst = []string{a.Value}
	case a.Op == "+=":
		if !slices.Contains(*dst, a.Value) {
			*dst = append(*dst, a.Value)
		}
	default:
		*dst = slices.DeleteFunc(*dst, func(v string) bool { return v == a.Value })
	}
	return nil
}

func setULIDs(dst *[]model.ULID, a Assign, resolve func(string) (model.ULID, error)) error {
	if a.Op == "=" && a.Value == "" {
		*dst = nil
		return nil
	}
	ulid, err := resolve(a.Value)
	if err != nil {
		return fmt.Errorf("%s: %w", a.Key, err)
	}
	switch a.Op {
	case "=":
		*dst = []model.ULID{ulid}
	case "+=":
		if !slices.Contains(*dst, ulid) {
			*dst = append(*dst, ulid)
		}
	default:
		*dst = slices.DeleteFunc(*dst, func(v model.ULID) bool { return v == ulid })
	}
	return nil
}

func setProp(t *model.Task, p schema.Prop, a Assign) error {
	multi := p.Multi || p.Type == schema.TypeMultiEnum
	if a.Op == "=" && a.Value == "" {
		t.Props.Delete(p.Name)
		return nil
	}
	if !multi && a.Op != "=" {
		return NewErr(KindInvalid, "assign.single", fmt.Errorf("%s holds one value; use %s=<value>", a.Key, a.Key), "key", a.Key)
	}
	if multi && a.Op != "=" && a.Value == "" {
		return NewErr(KindInvalid, "assign.missing_value", fmt.Errorf("%s%s: the value is missing", a.Key, a.Op), "key", a.Key)
	}
	n, err := propValue(p, a.Value)
	if err != nil {
		return err
	}
	if !multi {
		t.Props.Set(p.Name, n)
		return nil
	}
	var items []*yaml.Node
	if cur := t.Props.Get(p.Name); cur != nil && cur.Kind == yaml.SequenceNode {
		items = slices.Clone(cur.Content)
	}
	same := func(x *yaml.Node) bool { return x.Value == n.Value }
	switch a.Op {
	case "=":
		items = []*yaml.Node{n}
	case "+=":
		if !slices.ContainsFunc(items, same) {
			items = append(items, n)
		}
	default:
		items = slices.DeleteFunc(items, same)
	}
	if len(items) == 0 {
		t.Props.Delete(p.Name)
		return nil
	}
	t.Props.Set(p.Name, &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: items})
	return nil
}

// propValue reads v as a value of p. The tag makes the file show the value
// as its type: a text that looks like a number is quoted, a date is not.
func propValue(p schema.Prop, v string) (*yaml.Node, error) {
	tag := "!!str"
	bad := func(what string) error { return badAssign(p.Name, v, what) }
	switch p.Type {
	case schema.TypeNumber:
		_, ierr := strconv.ParseInt(v, 10, 64)
		f, ferr := strconv.ParseFloat(v, 64)
		switch {
		case ierr == nil:
			tag = "!!int"
		case ferr == nil && !math.IsNaN(f) && !math.IsInf(f, 0):
			tag = "!!float"
		default:
			return nil, bad("a number")
		}
	case schema.TypeBool:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, bad("true or false")
		}
		v, tag = strconv.FormatBool(b), "!!bool"
	case schema.TypeDate:
		if _, err := model.ParseDate(v); err != nil {
			return nil, bad("a date (YYYY-MM-DD)")
		}
		tag = "!!timestamp"
	case schema.TypeDatetime:
		if _, err := time.Parse(time.RFC3339, v); err != nil {
			return nil, bad("a time (RFC 3339)")
		}
		tag = "!!timestamp"
	case schema.TypeDuration:
		if _, err := model.ParseDuration(v); err != nil {
			return nil, bad("a duration (like 2h30m)")
		}
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: v}, nil
}
