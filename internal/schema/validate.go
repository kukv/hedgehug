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
	// The store and the index report these: they need every task of the
	// store, which the checks here do not see.
	CodeMissingRef = "missing_ref"
	CodeCycle      = "cycle"
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
	if t.Rank != "" {
		if err := t.Rank.Validate(); err != nil {
			violations = append(violations, Violation{CodeBadValue, "rank", err.Error()})
		}
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

// ValidateWrite checks a write of cur over old, which is nil for a new task.
// parentKind is the kind of cur's parent, empty at the root, and e the
// schema for cur where it is written; oldParentKind and oldE are the same
// for old where it was, and are unused when old is nil. It returns only the
// violations the write brings in: ones old already had where it was are left
// out, so a task that a sync left broken can still be edited, while a move
// is checked in its new place.
func ValidateWrite(old *model.Task, oldParentKind string, oldE *Effective, cur *model.Task, parentKind string, e *Effective) []Violation {
	vs := taskViolations(cur, parentKind, e)
	switch {
	case old == nil:
		vs = append(vs, ValidateTransition("", cur.Status, cur, e)...)
	case old.Status != cur.Status:
		vs = append(vs, ValidateTransition(old.Status, cur.Status, cur, e)...)
	}
	var had []Violation
	if old != nil {
		had = taskViolations(old, oldParentKind, oldE)
	}
	return newViolations(vs, had)
}

func taskViolations(t *model.Task, parentKind string, e *Effective) []Violation {
	vs, _ := ValidateTask(t, e)
	return append(vs, ValidatePlacement(t.Kind, parentKind, e)...)
}

// newViolations returns vs without the ones in had, and without repeats:
// two checks may report the same unknown kind or status. Violations are the
// same when their Code and Field are.
func newViolations(vs, had []Violation) []Violation {
	seen := make(map[[2]string]bool, len(had))
	for _, v := range had {
		seen[[2]string{v.Code, v.Field}] = true
	}
	var out []Violation
	for _, v := range vs {
		k := [2]string{v.Code, v.Field}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
	}
	return out
}
