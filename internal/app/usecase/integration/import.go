package integration

import (
	"errors"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/current"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// ImportInput says which issues to take in, and where to put them.
type ImportInput struct {
	Repo, Label       string
	Parent, Workspace string
	DryRun            bool
}

// ImportResult is what ImportIssues did. With DryRun, Count is how many it
// would make and Created is empty.
type ImportResult struct {
	Repo     string   `json:"repo"`
	Count    int      `json:"count"`
	Skipped  int      `json:"skipped" doc:"The issues that are linked already"`
	Created  []string `json:"created" nullable:"false" doc:"The ids of the tasks made"`
	DryRun   bool     `json:"dry_run"`
	Warnings []string `json:"warnings" nullable:"false"`
}

// importTarget is where the new tasks go.
type importTarget struct {
	parent     *model.ULID
	parentKind string
	workspace  model.ULID
}

func importPlace(ss *usecase.Session, ix *index.Index, in ImportInput) (importTarget, error) {
	var to importTarget
	var p *index.Entry
	if in.Parent != "" {
		var err error
		if p, err = ss.ResolveRef(ix, in.Parent); err != nil {
			return to, err
		}
		if err := usecase.LiveParent(ix, p); err != nil {
			return to, err
		}
	} else if p = useProject(ss, ix); p == nil {
		return to, usecase.NewErr(usecase.KindInvalid, "import.parent_required", errors.New("the parent is needed: give --parent, or set a project with hh use"))
	}
	to.parent, to.parentKind, to.workspace = &p.ULID, p.Kind, p.Workspace
	if in.Workspace != "" {
		w, err := usecase.ResolveWorkspace(ix, in.Workspace)
		if err != nil {
			return to, err
		}
		if w.ULID != to.workspace {
			return to, usecase.NewErr(usecase.KindInvalid, "task.workspace_mismatch", errors.New("the parent is in another workspace"))
		}
		to.workspace = w.ULID
	}
	return to, nil
}

// useProject is the project hh use set, when it is in this store and live.
func useProject(ss *usecase.Session, ix *index.Index) *index.Entry {
	_, p, err := ss.Config()
	if err != nil {
		return nil
	}
	st := current.Load(p.DataDir, nil)
	if st.Use == nil {
		return nil
	}
	e, ok := ix.Task(st.Use.Project)
	if !ok || e.DeletedAt != nil || e.Kind != usecase.ProjectKind {
		return nil
	}
	return e
}

// linkedIssues lists the issues that have a task: in use or unlinked, which
// stays unlinked.
func linkedIssues(ix *index.Index) []string {
	var out []string
	for _, e := range ix.Tasks() {
		for _, l := range e.Links {
			out = append(out, model.CanonRef(l.Ref))
		}
	}
	return out
}

// ImportIssues makes a task in mirror mode for each open issue of the
// repository that has the label (any open issue when Label is empty) and no
// task yet. The body is written once,
// here; the next read of the repository brings the rest. Each task is a commit
// of its own, so that a run that stopped goes on by running it again.
func ImportIssues(ss *usecase.Session, in ImportInput) (ImportResult, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return ImportResult{}, err
	}
	host, err := ss.WriterHost()
	if err != nil {
		return ImportResult{}, err
	}
	repo, err := repoID(in.Repo, host)
	if err != nil {
		return ImportResult{}, err
	}
	out := ImportResult{Repo: repo, Created: []string{}, Warnings: []string{}, DryRun: in.DryRun}
	c, se, err := ss.Entry()
	if err != nil {
		return out, err
	}
	cl, acct, err := ss.GitHubForRepo(c, se, repo)
	if err != nil {
		return out, err
	}
	owner, name := ownerName(repo)
	issues, _, err := cl.OpenIssues(owner, name, in.Label)
	if err != nil {
		return out, usecase.GithubErr(err, acct, "read the issues of "+repo)
	}
	linked := linkedIssues(ix)
	var todo []github.Issue
	for _, x := range issues {
		ref := model.GitHubRef{Repo: repo, Kind: model.LinkIssue, Number: x.Number}
		if slices.Contains(linked, ref.String()) {
			out.Skipped++
			continue
		}
		todo = append(todo, x)
	}
	out.Count = len(todo)
	if len(todo) == 0 {
		return out, nil
	}
	to, err := importPlace(ss, ix, in)
	if err != nil {
		return out, err
	}
	if in.DryRun {
		return out, nil
	}
	sc := ix.Schema()
	kind := schema.DefaultKindUnder(to.parentKind)
	status, err := usecase.DefaultTodo(sc)
	if err != nil {
		return out, err
	}
	me, err := ss.AccountID()
	if err != nil {
		return out, err
	}
	for _, x := range todo {
		ref := model.GitHubRef{Repo: repo, Kind: model.LinkIssue, Number: x.Number}
		t := &model.Task{
			ULID: model.NewULID(), Title: x.Title, Kind: kind, Status: status, Body: model.StripMark(x.Body),
			Parent: to.parent, Workspace: to.workspace, Repos: []string{repo}, CreatedBy: me,
			Links: []model.Link{{Type: model.LinkIssue, Ref: ref.String(), Sync: model.SyncMirror}},
		}
		if t.Rank, err = ss.RankAt(ix, t.ULID, t.Parent, t.Workspace, "", "", "add.position"); err != nil {
			return out, err
		}
		if me != "" {
			usecase.AddWatchers(t, me)
		}
		if _, err := ss.UpdateCommit(s, store.Op{Name: "add", Task: t.ULID}, func(tx *store.Tx) error { return tx.PutTask(t) }); err != nil {
			return out, fmt.Errorf("%s: %w", ref.Label(), err)
		}
		if err := ix.Refresh(); err != nil {
			return out, err
		}
		if e, ok := ix.Task(t.ULID); ok {
			out.Created = append(out.Created, e.ID)
		}
	}
	out.Warnings = append(out.Warnings, fetchNow(ss, c, se, s, repo)...)
	return out, nil
}
