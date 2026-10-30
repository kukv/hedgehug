package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// assign is one key=value of set and of add -p: = sets (an empty value
// clears), += adds to a list and -= removes from it.
type assign struct {
	key   string
	op    string // "=", "+=" or "-="
	value string
}

func parseAssign(s string) (assign, error) {
	i := strings.IndexByte(s, '=')
	if i < 0 {
		return assign{}, usageErrorf("%q: want key=value, key+=value or key-=value", s)
	}
	a := assign{key: s[:i], op: "=", value: s[i+1:]}
	if k, ok := strings.CutSuffix(a.key, "+"); ok {
		a.key, a.op = k, "+="
	} else if k, ok := strings.CutSuffix(a.key, "-"); ok {
		a.key, a.op = k, "-="
	}
	if a.key == "" {
		return assign{}, usageErrorf("%q: the key is missing", s)
	}
	return a, nil
}

func parseAssigns(ss []string) ([]assign, error) {
	out := make([]assign, 0, len(ss))
	for _, s := range ss {
		a, err := parseAssign(s)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// applyAssign changes t as a says. Built-in fields come first; any other key
// must be a property of sc, and its value is read by the property's type.
// resolve turns task names into IDs, for blocked_by and task_ref properties.
// Whether the result fits the schema is the store's to check.
func applyAssign(t *model.Task, a assign, sc *schema.Schema, resolve func(string) (model.ID, error)) error {
	switch a.key {
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
		return setList(&t.Labels, a)
	case "repos":
		return setList(&t.Repos, a)
	case "blocked_by":
		return setIDs(&t.BlockedBy, a, resolve)
	case "parent", "rank":
		return fmt.Errorf("%s cannot be set; use hedgehug mv", a.key)
	}
	if slices.Contains(model.TaskKeys(), a.key) {
		return fmt.Errorf("%s cannot be set", a.key)
	}
	p, ok := sc.Prop(a.key)
	if !ok {
		return fmt.Errorf("%s is not a field or a property of the schema", a.key)
	}
	return setProp(t, p, a, resolve)
}

func setScalar(dst *string, a assign) error {
	if a.op != "=" {
		return fmt.Errorf("%s holds one value; use %s=<value>", a.key, a.key)
	}
	*dst = a.value
	return nil
}

func setList(dst *[]string, a assign) error {
	switch {
	case a.op == "=" && a.value == "":
		*dst = nil
	case a.value == "":
		return fmt.Errorf("%s%s: the value is missing", a.key, a.op)
	case a.op == "=":
		*dst = []string{a.value}
	case a.op == "+=":
		if !slices.Contains(*dst, a.value) {
			*dst = append(*dst, a.value)
		}
	default:
		*dst = slices.DeleteFunc(*dst, func(v string) bool { return v == a.value })
	}
	return nil
}

func setIDs(dst *[]model.ID, a assign, resolve func(string) (model.ID, error)) error {
	if a.op == "=" && a.value == "" {
		*dst = nil
		return nil
	}
	id, err := resolve(a.value)
	if err != nil {
		return fmt.Errorf("%s: %w", a.key, err)
	}
	switch a.op {
	case "=":
		*dst = []model.ID{id}
	case "+=":
		if !slices.Contains(*dst, id) {
			*dst = append(*dst, id)
		}
	default:
		*dst = slices.DeleteFunc(*dst, func(v model.ID) bool { return v == id })
	}
	return nil
}

func setProp(t *model.Task, p schema.Prop, a assign, resolve func(string) (model.ID, error)) error {
	multi := p.Multi || p.Type == schema.TypeMultiEnum
	if a.op == "=" && a.value == "" {
		t.Props.Delete(p.Name)
		return nil
	}
	if !multi && a.op != "=" {
		return fmt.Errorf("%s holds one value; use %s=<value>", a.key, a.key)
	}
	if multi && a.op != "=" && a.value == "" {
		return fmt.Errorf("%s%s: the value is missing", a.key, a.op)
	}
	n, err := propValue(p, a.value, resolve)
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
	switch a.op {
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
func propValue(p schema.Prop, v string, resolve func(string) (model.ID, error)) (*yaml.Node, error) {
	tag := "!!str"
	bad := func(what string) error { return fmt.Errorf("%s: %q is not %s", p.Name, v, what) }
	switch p.Type {
	case schema.TypeNumber:
		_, ierr := strconv.ParseInt(v, 10, 64)
		_, ferr := strconv.ParseFloat(v, 64)
		switch {
		case ierr == nil:
			tag = "!!int"
		case ferr == nil:
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
	case schema.TypeTaskRef:
		id, err := resolve(v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Name, err)
		}
		v = id.String()
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: v}, nil
}
