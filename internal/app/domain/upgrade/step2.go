package upgrade

import (
	"fmt"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

// Format 1 had the kinds in schema.yaml, each with a role and a default
// template; format 2 has the six kinds of the code. What one task decides in
// that change is its kind.

// OldKind is one entry of the kinds of a schema.yaml in format 1.
type OldKind struct {
	Name     string `yaml:"name"`
	Role     string `yaml:"role"`
	Template string `yaml:"template"`
}

// IsGroup reports whether the kind holds other tasks (role container).
func (k OldKind) IsGroup() bool { return k.Role == "container" }

// OldKinds are the kinds of a schema.yaml in format 1, in file order.
func OldKinds(sc *schema.Schema) ([]OldKind, error) {
	var out []OldKind
	if n := sc.Extra.Get("kinds"); n != nil {
		if err := n.Decode(&out); err != nil {
			return nil, fmt.Errorf("%s: kinds: %w", schema.Path, err)
		}
	}
	return out, nil
}

// FixedKind is the fixed kind a kind of format 1 goes to, for a task at the
// root or not: a name that is a fixed kind stays; a custom group is a project
// at the root and an epic below it; a custom item is a task. The first kind of
// a name counts.
func FixedKind(kinds []OldKind, name string, atRoot bool) string {
	if _, ok := schema.KindOf(name); ok {
		return name
	}
	for _, k := range kinds {
		if k.Name == name && k.IsGroup() {
			if atRoot {
				return "project"
			}
			return "epic"
		}
		if k.Name == name {
			break
		}
	}
	return "task"
}

// task1to2 gives the task a fixed kind. Where it goes in the tree is the job
// of hh store convert.
func task1to2(raw []byte, cx Cx) ([]byte, error) {
	t, err := model.DecodeTask(raw)
	if err != nil {
		return nil, err
	}
	if _, ok := schema.KindOf(t.Kind); ok {
		return raw, nil
	}
	sc, err := schema.Decode(cx.Schema)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", schema.Path, err)
	}
	kinds, err := OldKinds(sc)
	if err != nil {
		return nil, err
	}
	t.Kind = FixedKind(kinds, t.Kind, t.Parent == nil)
	return t.Encode()
}
