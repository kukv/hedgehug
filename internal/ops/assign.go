package ops

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
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
		return Assign{}, newErr(KindUsage, "assign.form", fmt.Errorf("%q: want key=value, key+=value or key-=value", s), "input", s)
	}
	a := Assign{Key: s[:i], Op: "=", Value: s[i+1:]}
	if k, ok := strings.CutSuffix(a.Key, "+"); ok {
		a.Key, a.Op = k, "+="
	} else if k, ok := strings.CutSuffix(a.Key, "-"); ok {
		a.Key, a.Op = k, "-="
	}
	if a.Key == "" {
		return Assign{}, newErr(KindUsage, "assign.form", fmt.Errorf("%q: the key is missing", s), "input", s)
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
// resolve turns task names into ULIDs, for blocked_by and task_ref properties.
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
		return setList(&t.Labels, a)
	case "repos":
		return setList(&t.Repos, a)
	case "blocked_by":
		return setULIDs(&t.BlockedBy, a, resolve)
	case "parent", "rank":
		return newErr(KindInvalid, "assign.readonly", fmt.Errorf("%s cannot be set; use hedgehug mv", a.Key), "key", a.Key)
	}
	if slices.Contains(model.TaskKeys(), a.Key) {
		return newErr(KindInvalid, "assign.readonly", fmt.Errorf("%s cannot be set", a.Key), "key", a.Key)
	}
	p, ok := sc.Prop(a.Key)
	if !ok {
		return newErr(KindInvalid, "assign.unknown_key", fmt.Errorf("%s is not a field or a property of the schema", a.Key), "key", a.Key)
	}
	return setProp(t, p, a, resolve)
}

func setScalar(dst *string, a Assign) error {
	if a.Op != "=" {
		return newErr(KindInvalid, "assign.single", fmt.Errorf("%s holds one value; use %s=<value>", a.Key, a.Key), "key", a.Key)
	}
	*dst = a.Value
	return nil
}

func setList(dst *[]string, a Assign) error {
	switch {
	case a.Op == "=" && a.Value == "":
		*dst = nil
	case a.Value == "":
		return newErr(KindInvalid, "assign.missing_value", fmt.Errorf("%s%s: the value is missing", a.Key, a.Op), "key", a.Key)
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

func setProp(t *model.Task, p schema.Prop, a Assign, resolve func(string) (model.ULID, error)) error {
	multi := p.Multi || p.Type == schema.TypeMultiEnum
	if a.Op == "=" && a.Value == "" {
		t.Props.Delete(p.Name)
		return nil
	}
	if !multi && a.Op != "=" {
		return newErr(KindInvalid, "assign.single", fmt.Errorf("%s holds one value; use %s=<value>", a.Key, a.Key), "key", a.Key)
	}
	if multi && a.Op != "=" && a.Value == "" {
		return newErr(KindInvalid, "assign.missing_value", fmt.Errorf("%s%s: the value is missing", a.Key, a.Op), "key", a.Key)
	}
	n, err := propValue(p, a.Value, resolve)
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
func propValue(p schema.Prop, v string, resolve func(string) (model.ULID, error)) (*yaml.Node, error) {
	tag := "!!str"
	bad := func(what string) error {
		return newErr(KindInvalid, "assign.bad_value", fmt.Errorf("%s: %q is not %s", p.Name, v, what), "key", p.Name, "value", v)
	}
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
	case schema.TypeTaskRef:
		ulid, err := resolve(v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Name, err)
		}
		v = ulid.String()
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: v}, nil
}
