package index

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// AmbiguousWorkspaceError means an input names more than one workspace.
type AmbiguousWorkspaceError struct {
	Input      string
	Candidates []*model.Workspace // in name order
}

func (e *AmbiguousWorkspaceError) Error() string {
	return fmt.Sprintf("%q matches %d workspaces", e.Input, len(e.Candidates))
}

// Workspaces returns every workspace, by name then ULID.
func (ix *Index) Workspaces() []*model.Workspace {
	out := slices.Collect(maps.Values(ix.workspaces))
	slices.SortFunc(out, func(a, b *model.Workspace) int {
		if c := cmp.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return cmp.Compare(a.ULID.String(), b.ULID.String())
	})
	return out
}

// Workspace returns the workspace of id.
func (ix *Index) Workspace(id model.ULID) (*model.Workspace, bool) {
	w, ok := ix.workspaces[id]
	return w, ok
}

// ResolveWorkspace finds the workspace s names, ignoring case: by a full
// ULID, or by its name or its key prefix.
func (ix *Index) ResolveWorkspace(s string) (*model.Workspace, error) {
	var hits []*model.Workspace
	for _, w := range ix.Workspaces() {
		if len(s) == 26 && strings.EqualFold(w.ULID.String(), s) ||
			s != "" && (strings.EqualFold(w.Name, s) || strings.EqualFold(w.KeyPrefix, s)) {
			hits = append(hits, w)
		}
	}
	switch len(hits) {
	case 0:
		return nil, fmt.Errorf("workspace %q: %w", s, store.ErrNotFound)
	case 1:
		return hits[0], nil
	}
	return nil, &AmbiguousWorkspaceError{Input: s, Candidates: hits}
}

// readWorkspaces decodes every workspace file of tree. A file that does not
// decode is skipped; doctor reports it. Workspaces are read from the tree
// each time and not cached, since they hold YAML nodes that gob cannot carry.
func (ix *Index) readWorkspaces(tree plumbing.Hash) error {
	out := map[model.ULID]*model.Workspace{}
	for p, err := range ix.repo.Files(tree, "workspaces/") {
		if err != nil {
			return err
		}
		if _, err := model.ParseWorkspacePath(p); err != nil {
			continue
		}
		b, err := ix.repo.ReadFile(tree, p)
		if err != nil {
			return err
		}
		if w, err := model.DecodeWorkspace(b); err == nil {
			out[w.ULID] = w
		}
	}
	ix.workspaces = out
	return nil
}
