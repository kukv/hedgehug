package schema

import "slices"

// Role says what a kind of task is for. It is fixed per kind and is not
// written to schema.yaml.
type Role string

const (
	RoleGroup Role = "group" // groups work, like a project or an epic
	RoleItem  Role = "item"  // a work item
)

// KindDef is one of the fixed kinds of task. Parents are the kinds it may sit
// under; "" stands for the root of the workspace.
type KindDef struct {
	Name, Label string
	Role        Role
	Parents     []string
}

// Kinds are the six kinds of task, in the order shown and sorted by kind.
var Kinds = []KindDef{
	{"project", "Project", RoleGroup, []string{""}},
	{"epic", "Epic", RoleGroup, []string{"project"}},
	{"story", "Story", RoleItem, []string{"project", "epic"}},
	{"task", "Task", RoleItem, []string{"project", "epic"}},
	{"bug", "Bug", RoleItem, []string{"project", "epic"}},
	{"step", "Step", RoleItem, []string{"story", "task", "bug"}},
}

// DefaultKind is the kind of a task made without one.
const DefaultKind = "task"

// KindOf returns the kind called name.
func KindOf(name string) (KindDef, bool) {
	for _, k := range Kinds {
		if k.Name == name {
			return k, true
		}
	}
	return KindDef{}, false
}

// CanPlace reports whether a task of kind may sit under a parent of
// parentKind. An empty parentKind means the root of the workspace.
func CanPlace(kind, parentKind string) bool {
	k, ok := KindOf(kind)
	return ok && slices.Contains(k.Parents, parentKind)
}

// IsGroup reports whether kind groups work: a project or an epic.
func IsGroup(kind string) bool {
	k, ok := KindOf(kind)
	return ok && k.Role == RoleGroup
}

// CountsForProgress reports whether a task of kind counts in the progress of
// its project or epic: a story, a task or a bug. A step does not.
func CountsForProgress(kind string) bool {
	switch kind {
	case "story", "task", "bug":
		return true
	}
	return false
}
