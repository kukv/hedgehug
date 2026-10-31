package store

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/atomicfile"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// Codes of the violations validate adds to the schema's.
const (
	CodeNoWorkspace       = "no_workspace"
	CodeWorkspaceMismatch = "workspace_mismatch"
)

// SchemaError means a schema written with PutSchema has problems.
type SchemaError struct{ Problems []schema.Problem }

func (e *SchemaError) Error() string {
	msgs := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		msgs[i] = p.String()
	}
	return "invalid schema: " + strings.Join(msgs, "; ")
}

// ValidationError means a write would bring violations into a task.
type ValidationError struct {
	Task       model.ULID
	Violations []schema.Violation
}

func (e *ValidationError) Error() string {
	msgs := make([]string, len(e.Violations))
	for i, v := range e.Violations {
		msgs[i] = v.String()
	}
	return fmt.Sprintf("task %s: %s", e.Task, strings.Join(msgs, "; "))
}

// validate checks every task put in the operation against sc. The rules are
// schema.ValidateWrite's; this only gathers what it needs: the task as it is
// on disk, and for both it and the task being written the kind of the
// parent and the ancestors. The old task's are read from disk, so a move is
// checked in its new place.
func (tx *Tx) validate(sc *schema.Schema) error {
	if err := tx.validateWorkspaces(); err != nil {
		return err
	}
	for _, ulid := range tx.taskOrder {
		t := tx.tasks[ulid]
		if t == nil {
			continue // removed in the same operation
		}
		old, err := tx.s.Task(ulid)
		// A file that does not decode is checked like a new task, so a
		// write can repair it.
		if errors.Is(err, ErrNotFound) || errors.As(err, new(*model.DecodeError)) {
			old = nil
		} else if err != nil {
			return err
		}
		parentKind, ancestors, err := lineage(t, sc, tx.task)
		if err != nil {
			return err
		}
		var oldParentKind string
		var oldE *schema.Effective
		if old != nil {
			var oldAncestors []model.ULID
			if oldParentKind, oldAncestors, err = lineage(old, sc, tx.s.Task); err != nil {
				return err
			}
			oldE = sc.For(oldAncestors)
		}
		vs := schema.ValidateWrite(old, oldParentKind, oldE, t, parentKind, sc.For(ancestors), tx.op.SkipTransitions)
		wvs, err := tx.workspaceViolations(t)
		if err != nil {
			return err
		}
		rvs, err := tx.checkRefs(old, t, sc)
		if err != nil {
			return err
		}
		vs = append(vs, wvs...)
		if vs = append(vs, rvs...); len(vs) > 0 {
			return &ValidationError{Task: ulid, Violations: vs}
		}
	}
	return nil
}

// workspaceViolations checks that t belongs to a workspace that exists, the
// one of its parent.
func (tx *Tx) workspaceViolations(t *model.Task) ([]schema.Violation, error) {
	if t.Workspace.IsZero() {
		return []schema.Violation{{Code: CodeNoWorkspace, Field: "workspace", Msg: "missing"}}, nil
	}
	if _, err := tx.workspace(t.Workspace); errors.Is(err, ErrNotFound) {
		return []schema.Violation{{Code: CodeNoWorkspace, Field: "workspace", Msg: fmt.Sprintf("workspace %s is not in this store", t.Workspace)}}, nil
	} else if err != nil {
		return nil, err
	}
	if t.Parent == nil {
		return nil, nil
	}
	p, err := tx.task(*t.Parent)
	if errors.Is(err, ErrNotFound) || errors.As(err, new(*model.DecodeError)) {
		return nil, nil // checkRefs deals with a missing parent
	}
	if err != nil {
		return nil, err
	}
	if p.Workspace != t.Workspace {
		return []schema.Violation{{Code: CodeWorkspaceMismatch, Field: "workspace", Msg: "differs from the workspace of the parent; move the task with hedgehug mv --workspace"}}, nil
	}
	return nil, nil
}

// validateWorkspaces checks the workspaces put in the operation: the form of
// the prefix and the name, and that no other workspace, as the operation
// leaves them, has the same prefix or the same name (ignoring case). A
// workspace removed in the operation must have no task left.
func (tx *Tx) validateWorkspaces() error {
	if len(tx.workspaces) == 0 {
		return nil
	}
	all := map[model.ULID]*model.Workspace{}
	for w, err := range tx.s.Workspaces() {
		if err != nil {
			continue // a broken file cannot be compared with
		}
		all[w.ULID] = w
	}
	removed := false
	for ulid, w := range tx.workspaces {
		if w == nil {
			delete(all, ulid)
			removed = true
		} else {
			all[ulid] = w
		}
	}
	for ulid, w := range tx.workspaces {
		if w == nil {
			continue
		}
		if err := model.ValidIDPrefix(w.IDPrefix); err != nil {
			return err
		}
		if err := model.ValidWorkspaceName(w.Name); err != nil {
			return err
		}
		for otherULID, o := range all {
			if otherULID == ulid {
				continue
			}
			if o.IDPrefix == w.IDPrefix {
				return fmt.Errorf("the id prefix %s is already used by workspace %s", w.IDPrefix, o.Name)
			}
			if strings.EqualFold(o.Name, w.Name) {
				return fmt.Errorf("a workspace named %s already exists", o.Name)
			}
		}
	}
	if !removed {
		return nil
	}
	// The tasks written in the operation count as they will be; the others
	// as they are on disk.
	count := map[model.ULID]int{}
	for _, ulid := range tx.taskOrder {
		if t := tx.tasks[ulid]; t != nil {
			count[t.Workspace]++
		}
	}
	broken := map[model.ULID]string{} // a task file that does not decode and names the workspace
	for t, err := range tx.s.Tasks() {
		var fe *FileError
		if errors.As(err, &fe) {
			if ulid, perr := model.ParseTaskPath(fe.Path); perr == nil {
				if _, put := tx.tasks[ulid]; put {
					continue // written or removed in the operation
				}
			}
			// Its workspace cannot be read. Removing the workspace it names
			// would leave it pointing nowhere once it is fixed, so a file
			// that holds the ULID anywhere, in any case, is taken to be in
			// that workspace: refusing too often is the safe side.
			b, err := atomicfile.ReadFile(tx.s.path(fe.Path))
			if err != nil {
				return err
			}
			b = bytes.ToUpper(b)
			for ulid, w := range tx.workspaces {
				if w == nil && bytes.Contains(b, []byte(ulid.String())) {
					broken[ulid] = fe.Path
				}
			}
			continue
		}
		if err != nil {
			continue
		}
		if _, put := tx.tasks[t.ULID]; !put {
			count[t.Workspace]++
		}
	}
	for ulid, w := range tx.workspaces {
		if w != nil {
			continue
		}
		if count[ulid] > 0 {
			return fmt.Errorf("workspace %s still has %d tasks", ulid, count[ulid])
		}
		if rel, ok := broken[ulid]; ok {
			return fmt.Errorf("workspace %s may still have a task: %s does not decode and names it; fix or remove the file first", ulid, rel)
		}
	}
	return nil
}

// lineage returns the kind of t's parent, empty at the root, and the ULIDs
// For needs: t's ancestors nearest first, led by t itself when t is a
// container. Parents are read with read: as the operation leaves them for
// the task being written, from disk for the task as it was.
//
// The walk stops at the first task that is missing or does not decode, and
// keeps the ancestors found so far, with an empty parentKind when the direct
// parent is the one missing. So a task whose parent was purged or broken can
// still be edited; checkRefs refuses a write that adds a missing parent.
// It also stops when it comes back to a task it has seen.
func lineage(t *model.Task, sc *schema.Schema, read func(model.ULID) (*model.Task, error)) (parentKind string, ancestors []model.ULID, err error) {
	if k, ok := sc.Kind(t.Kind); ok && k.Role == schema.RoleContainer {
		ancestors = append(ancestors, t.ULID)
	}
	seen := map[model.ULID]bool{t.ULID: true}
	for cur := t; cur.Parent != nil; {
		ulid := *cur.Parent
		if seen[ulid] {
			break // a cycle the store already had; checkRefs refuses new ones
		}
		seen[ulid] = true
		p, err := read(ulid)
		if errors.Is(err, ErrNotFound) || errors.As(err, new(*model.DecodeError)) {
			break
		}
		if err != nil {
			return "", nil, fmt.Errorf("parent of %s: %w", cur.ULID, err)
		}
		if cur == t {
			parentKind = p.Kind
		}
		ancestors = append(ancestors, ulid)
		cur = p
	}
	return parentKind, ancestors, nil
}
