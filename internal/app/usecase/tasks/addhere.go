package tasks

import (
	"cmp"
	"errors"

	"github.com/kukv/hedgehug/internal/app/storage/current"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/mount"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// AddHereInput is hh add with the context: what Resolve takes, and the task.
type AddHereInput struct {
	Context usecase.Input
	Repo    bool     // put the repo ID of the current repository on the task
	Task    AddInput // Store, Parent and Workspace are set from the destination
}

// AddHereResult is the task added and where it went.
type AddHereResult struct {
	usecase.TaskResult
	Destination *usecase.Destination
}

// AddHere resolves the destination (its Confirm and Choice are returned for
// the CLI to ask), refuses a read-only store there, adds the task with the
// repo ID of the current repository when Repo, and sets recent when
// stage 4 picked a project (after the commit).
//
// The session's own --store is the Store of the context when that has none,
// and the config's default_store is its DefaultStore.
func AddHere(ss *usecase.Session, in AddHereInput) (AddHereResult, *usecase.Confirm, *usecase.Choice, error) {
	c, p, err := ss.Config()
	if err != nil {
		return AddHereResult{}, nil, nil, err
	}
	ctx := in.Context
	ctx.NeedRepo = in.Repo
	if ctx.Parent == "" && ctx.Project == "" {
		ctx.Sibling = cmp.Or(in.Task.Before, in.Task.After)
	}
	if ctx.Store == "" {
		ctx.Store = ss.StoreName
	}
	if ctx.Store != "" {
		// an unknown store is said as the other commands say it
		if _, err := c.Select(ctx.Store); err != nil {
			return AddHereResult{}, nil, nil, usecase.SelectError(ctx.Store, err)
		}
	}
	if ctx.DefaultStore == "" {
		ctx.DefaultStore = c.DefaultStore
	}
	set, err := ss.Mount()
	if err != nil {
		return AddHereResult{}, nil, nil, err
	}
	// A call that carries the answer to a question is a repeat of an earlier
	// one, which gave the warnings (context.json, hedgehug use): say them once.
	// (This is for what is wrong with context.json. A store that does not open
	// is told of once by warnSkipped.)
	warn := ss.AddWarning
	if ctx.BranchParent != usecase.Unasked || ctx.PickProject != "" || ctx.PickStore != "" || ctx.PickWorkspace != "" {
		warn = nil
	}
	st := current.Load(p.DataDir, warn)
	dest, confirm, choice, err := usecase.ResolveDestination(set, st, ctx, warn)
	if err != nil {
		return AddHereResult{}, nil, nil, contextError(ss, set, err)
	}
	if confirm != nil || choice != nil {
		return AddHereResult{}, confirm, choice, nil
	}
	task := in.Task
	task.Store = dest.Store.Name
	task.Workspace = dest.Workspace.ULID.String()
	task.Parent = ""
	if ctx.Sibling != "" {
		// the sibling decided the place: Add takes the parent and the workspace from it, and
		// says add.position for a --workspace that does not fit
		task.Workspace = ctx.Workspace
	} else if dest.Parent != nil {
		task.Parent = dest.Parent.ULID.String()
		// a --workspace given with the parent is for Add to check against it
		task.Workspace = ctx.Workspace
	}
	if in.Repo {
		task.FirstRepo = dest.RepoID
	}
	res, err := Add(ss, task)
	if err != nil {
		return AddHereResult{}, nil, nil, err
	}
	if dest.Recent && dest.RepoID != "" {
		if st.Recent == nil {
			st.Recent = map[string]current.Ref{}
		}
		// keyed by the repo ID of the repository: with no origin and several
		// remotes, the first remote's by name, as the repo link is
		st.Recent[dest.RepoID] = current.Ref{Store: dest.Store.ULID, Project: dest.Parent.ULID}
		if err := current.Save(p.DataDir, st); err != nil {
			// the task is added; only the memory of the project is lost
			ss.AddWarning("the project was not recorded for this repository: " + err.Error())
		}
	}
	return AddHereResult{TaskResult: res, Destination: dest}, nil, nil, nil
}

// contextError spells out what ResolveDestination could not find (LookupError).
func contextError(ss *usecase.Session, set *mount.Set, err error) error {
	var le *usecase.LookupError
	if !errors.As(err, &le) {
		return err
	}
	if le.What == "task" {
		return ss.MountResolveError(set, le.Input, le.Err)
	}
	if le.Store != nil {
		// let ResolveWorkspace spell it out
		var werr error
		if err := le.Store.With(func(ix *index.Index) { _, werr = usecase.ResolveWorkspace(ix, le.Input) }); err != nil {
			return err
		}
		if werr != nil {
			return werr
		}
		return le.Err
	}
	return usecase.NewErr(usecase.KindNotFound, "workspace.not_found", le.Err, "input", le.Input)
}
