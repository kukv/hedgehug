package ops

import (
	"errors"
	"fmt"
	"slices"

	hhctx "github.com/kukv/hedgehug/internal/context"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/mount"
)

// AddHereInput is hh add with the context: what Resolve takes, and the task.
type AddHereInput struct {
	Context hhctx.Input
	NoRepo  bool
	Task    AddInput // Store, Parent and Workspace are set from the destination
}

// AddHereResult is the task added and where it went.
type AddHereResult struct {
	TaskResult
	Destination *hhctx.Destination
}

// AddHere resolves the destination (its Confirm and Choice are returned for
// the CLI to ask), refuses a read-only store there, adds the task with the
// repo ID of the current repository unless NoRepo, and sets recent when
// stage 4 picked a project (after the commit).
//
// The session's own --store is the Store of the context when that has none,
// and the config's default_store is its DefaultStore.
func (ss *Session) AddHere(in AddHereInput) (AddHereResult, *hhctx.Confirm, *hhctx.Choice, error) {
	c, p, err := ss.Config()
	if err != nil {
		return AddHereResult{}, nil, nil, err
	}
	ctx := in.Context
	if ctx.Store == "" {
		ctx.Store = ss.StoreName
	}
	if ctx.Store != "" {
		// an unknown store is said as the other commands say it
		if _, err := c.Select(ctx.Store); err != nil {
			return AddHereResult{}, nil, nil, err
		}
	}
	if ctx.DefaultStore == "" {
		ctx.DefaultStore = c.DefaultStore
	}
	set, err := ss.Mount()
	if err != nil {
		return AddHereResult{}, nil, nil, err
	}
	st := hhctx.Load(p.DataDir, ss.warn)
	dest, confirm, choice, err := hhctx.Resolve(set, st, ctx, ss.warn)
	if err != nil {
		return AddHereResult{}, nil, nil, ss.contextError(set, err)
	}
	if confirm != nil || choice != nil {
		return AddHereResult{}, confirm, choice, nil
	}
	task := in.Task
	task.Store = dest.Store.Name
	task.Workspace = dest.Workspace.ULID.String()
	task.Parent = ""
	if dest.Parent != nil {
		task.Parent = dest.Parent.ULID.String()
		// a --workspace given with the parent is for Add to check against it
		task.Workspace = ctx.Workspace
	}
	if dest.RepoID != "" && !in.NoRepo && !slices.Contains(task.Repos, dest.RepoID) {
		task.Repos = append([]string{dest.RepoID}, task.Repos...)
	}
	res, err := ss.Add(task)
	if err != nil {
		return AddHereResult{}, nil, nil, err
	}
	if dest.Recent && dest.RepoID != "" {
		if st.Recent == nil {
			st.Recent = map[string]hhctx.Ref{}
		}
		st.Recent[dest.RepoID] = hhctx.Ref{Store: dest.Store.ULID, Project: dest.Parent.ULID}
		if err := hhctx.Save(p.DataDir, st); err != nil {
			// the task is added; only the memory of the project is lost
			ss.warn("the project was not recorded for this repository: " + err.Error())
		}
	}
	return AddHereResult{TaskResult: res, Destination: dest}, nil, nil, nil
}

// contextError gives the error of hhctx.Resolve its Kind and Code.
func (ss *Session) contextError(set *mount.Set, err error) error {
	var le *hhctx.LookupError
	var ce *hhctx.Error
	switch {
	case errors.As(err, &le):
		if le.What == "task" {
			return ss.mountResolveError(set, le.Input, le.Err)
		}
		if le.Store != nil {
			// let ResolveWorkspace spell it out
			var werr error
			if err := le.Store.With(func(ix *index.Index) { _, werr = ResolveWorkspace(ix, le.Input) }); err != nil {
				return err
			}
			if werr != nil {
				return werr
			}
			return le.Err
		}
		return newErr(KindNotFound, "workspace.not_found", le.Err, "input", le.Input)
	case errors.As(err, &ce):
		var k Kind
		switch ce.Kind {
		case hhctx.KindUsage:
			k = KindUsage
		case hhctx.KindInvalid:
			k = KindInvalid
		case hhctx.KindAmbiguous:
			k = KindAmbiguous
		case hhctx.KindConflict:
			k = KindConflict
		default:
			return fmt.Errorf("context: unknown kind %d: %w", ce.Kind, err)
		}
		e := newErr(k, ce.Code, ce.Err)
		e.Args = ce.Args
		return e
	}
	return err
}
