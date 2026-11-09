package upgrade

import (
	"fmt"
	"slices"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

// Format 2 had duplicates, causes and relates as properties in props (of type
// task_ref, with a name of the schema.yaml) and let a store add task_ref
// properties of its own; format 3 has the five relations as fields of the task
// (schema.Relations). What one task decides in that change is where its task_ref
// values go.

// TypeTaskRef is the type of a property that points at tasks, which format 2
// had and format 3 does not.
const TypeTaskRef schema.PropType = "task_ref"

// LegacyRelationNames are the names that a task of format 2 could hold in
// props and that are relations of the task in format 3.
var LegacyRelationNames = []string{"duplicates", "causes", "relates", "clones"}

// RelationDefs is what the schema.yaml of format 2 says about the names that
// become relations: which properties are task_ref, and the relation the values
// of each go to.
type RelationDefs struct {
	to    map[string]string // name of a task_ref property to its relation
	other map[string]bool   // names of properties of another type
	names []string          // of the task_ref properties, in file order
}

// RelationDefsOf reads the properties of sc, which may be nil (a store without
// a schema.yaml). duplicates, causes and relates go to the field of the same
// name; a task_ref of the store's own goes to relates.
func RelationDefsOf(sc *schema.Schema) RelationDefs {
	d := RelationDefs{to: map[string]string{}, other: map[string]bool{}}
	if sc == nil {
		return d
	}
	for _, p := range sc.Props {
		if p.Type != TypeTaskRef {
			d.other[p.Name] = true
			continue
		}
		to := "relates"
		if p.Name == "duplicates" || p.Name == "causes" || p.Name == "relates" {
			to = p.Name
		}
		if _, dup := d.to[p.Name]; !dup {
			d.to[p.Name] = to
			d.names = append(d.names, p.Name)
		}
	}
	return d
}

// Names are the task_ref properties, in file order.
func (d RelationDefs) Names() []string { return slices.Clone(d.names) }

// Target is the relation the values of the property name go to. defined says
// the schema has the property as task_ref; a name that duplicates, causes,
// relates or clones has no definition (the value stayed in props after the
// definition went) goes to the relation of its own name, as long as its
// values are ULIDs. A property of another type does not move.
func (d RelationDefs) Target(name string) (to string, defined, ok bool) {
	if to, ok := d.to[name]; ok {
		return to, true, true
	}
	if !d.other[name] && slices.Contains(LegacyRelationNames, name) {
		return name, false, true
	}
	return "", false, false
}

// Moved is what became of one property of a task or of a template item.
type Moved struct {
	Prop   string       // the name in props
	To     string       // the relation
	Values []model.ULID // the values that were added to the relation, none twice
	// Bad are the values that are not a ULID, as written. A property the
	// schema defines as task_ref loses them; a property without a definition
	// is left alone when it has one.
	Bad []string
}

// MoveRelations moves the task_ref values of props into the relations that
// into gives (the list of a relation by its name), after what they already
// hold, and removes the properties. A value that is not a ULID is not moved.
// What was moved is returned, in the order of props.
func MoveRelations(props *model.Fields, d RelationDefs, into func(relation string) *[]model.ULID) []Moved {
	var out []Moved
	for _, f := range slices.Clone(*props) {
		to, defined, ok := d.Target(f.Key)
		if !ok {
			continue
		}
		m := Moved{Prop: f.Key, To: to}
		for _, n := range valueNodes(f.Value) {
			if n.Kind == yaml.ScalarNode {
				if u, err := model.ParseULID(n.Value); err == nil {
					if !slices.Contains(m.Values, u) {
						m.Values = append(m.Values, u)
					}
					continue
				}
			}
			m.Bad = append(m.Bad, nodeText(n))
		}
		if len(m.Bad) > 0 && !defined {
			continue // not known to be a reference: it stays where it is
		}
		props.Delete(f.Key)
		list := into(to)
		for _, u := range m.Values {
			if !slices.Contains(*list, u) {
				*list = append(*list, u)
			}
		}
		out = append(out, m)
	}
	return out
}

// valueNodes are the values of a property: the items of a list, the one value
// of a scalar, none for null.
func valueNodes(n *yaml.Node) []*yaml.Node {
	switch {
	case n == nil || n.Kind == yaml.ScalarNode && n.ShortTag() == "!!null":
		return nil
	case n.Kind == yaml.SequenceNode:
		return slices.DeleteFunc(slices.Clone(n.Content), func(x *yaml.Node) bool {
			return x.Kind == yaml.ScalarNode && x.ShortTag() == "!!null"
		})
	}
	return []*yaml.Node{n}
}

// nodeText is a value as it reads in the file, for a list of what is wrong.
func nodeText(n *yaml.Node) string {
	if n.Kind == yaml.ScalarNode {
		return n.Value
	}
	b, err := yaml.Marshal(n)
	if err != nil {
		return fmt.Sprintf("(line %d)", n.Line)
	}
	return string(b[:len(b)-1])
}

// TaskRelations is MoveRelations for the props and the relations of t.
func TaskRelations(t *model.Task, d RelationDefs) []Moved {
	return MoveRelations(&t.Props, d, func(rel string) *[]model.ULID {
		switch rel {
		case "duplicates":
			return &t.Duplicates
		case "causes":
			return &t.Causes
		case "clones":
			return &t.Clones
		}
		return &t.Relates
	})
}

// ItemRelations is MoveRelations for a template item and, one after the other,
// the items under it. The moves of the item come before those of its
// children.
func ItemRelations(it *model.TemplateItem, d RelationDefs) []Moved {
	out := MoveRelations(&it.Props, d, func(rel string) *[]model.ULID {
		switch rel {
		case "duplicates":
			return &it.Duplicates
		case "causes":
			return &it.Causes
		case "clones":
			return &it.Clones
		}
		return &it.Relates
	})
	for i := range it.Children {
		out = append(out, ItemRelations(&it.Children[i], d)...)
	}
	return out
}

// LegacyConflicts are the names of the relations that t holds in props, as a
// property that format 3 does not have, and in the field of its own too: which
// of the two is newer cannot be told, so neither is moved.
func LegacyConflicts(t *model.Task) []string {
	var out []string
	for _, f := range t.Props {
		if !slices.Contains(LegacyRelationNames, f.Key) {
			continue
		}
		if len(valueNodes(f.Value)) == 0 {
			continue
		}
		if r, _, ok := schema.RelationOf(f.Key); ok && len(r.Get(t)) > 0 {
			out = append(out, f.Key)
		}
	}
	return out
}

// task2to3 moves the task_ref values of a task from props to its relations. A
// value that is not a ULID is dropped from a property the schema defines as
// task_ref: a file that is read as the newest format cannot hold it, and hh
// store convert, which does not drop it unless it is told to, is the one that
// tells of it.
func task2to3(raw []byte, cx Cx) ([]byte, error) {
	t, err := model.DecodeTask(raw)
	if err != nil {
		return nil, err
	}
	var sc *schema.Schema
	if cx.Schema != nil {
		if sc, err = schema.Decode(cx.Schema); err != nil {
			return nil, fmt.Errorf("%s: %w", schema.Path, err)
		}
	}
	if len(TaskRelations(t, RelationDefsOf(sc))) == 0 {
		return raw, nil
	}
	return t.Encode()
}

// MoveLegacyFields moves the relations that a task of the newest format still
// holds in props, as an older hedgehug writes them, to the fields of the task:
// the names format 3 made relations, whose values are ULIDs. It is what hh
// doctor --fix does; it does not look at the schema, which no longer has them.
func MoveLegacyFields(raw []byte) ([]byte, error) { return task2to3(raw, Cx{}) }
