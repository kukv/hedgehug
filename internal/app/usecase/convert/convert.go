package convert

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// steps is the whole conversion, in order. A new format adds the step that
// reads the format before it at the end, and leaves the others alone.
var steps = []Step{
	{From: 1, Apply: step2},
}

// Run converts the store of the session from its format to the newest, one
// step after the other, and returns what each step changed. With dryRun it
// writes nothing. Otherwise it writes all the steps as one commit (op
// convert) and, for a store with a remote, pushes it.
//
// A store with a remote is converted only when it is in sync with it: origin
// has no commit the store lacks, and the store has none origin lacks
// (store.convert_unsynced). The one exception is a clone that has changes it
// has not pushed while origin was converted: it converts its own changes, then
// syncs, which merges them (sync.format_diverged, which a sync reports, is this
// state). A clone with nothing it has not pushed, while origin was converted,
// has nothing to convert: it goes to origin by fast-forward. A store already in the newest format has nothing to convert and
// returns no report.
//
// When the sync after the commit fails, the commit stays: the reports come
// back with the error.
func Run(ss *usecase.Session, opt Options, dryRun bool) ([]Report, error) {
	c, e, err := ss.Entry()
	if err != nil {
		return nil, err
	}
	s, err := ss.OpenStoreAtAnyFormat(e.Path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = s.Repo().Close() }()
	from := s.Meta().Format
	if from >= model.StoreFormat {
		return nil, nil
	}
	r := s.Repo()
	synced := len(e.Remotes) > 0 && !dryRun
	var syncOpt replicate.Options
	if len(e.Remotes) > 0 {
		if syncOpt, err = ss.SyncOptions(c, e); err != nil {
			return nil, err
		}
		// A dry run only looks: a remote it cannot reach is not its error.
		if err := replicate.FetchOrigin(s, syncOpt); err != nil && !dryRun {
			return nil, err
		} else if err == nil {
			ff, err := checkSynced(r, from)
			switch {
			case err != nil && !dryRun:
				return nil, err
			case ff && dryRun:
				return []Report{{From: from, Lines: []string{fastForwardLine + "する（--dry-run のため何も書かない）"}}}, nil
			case ff:
				if err := replicate.FastForwardOrigin(s); err != nil {
					return nil, err
				}
				ss.DropMount()
				return []Report{{From: from, Lines: []string{fastForwardLine + "した"}}}, nil
			}
		}
	}

	head, err := r.Head()
	if err != nil {
		return nil, err
	}
	tree, err := r.TreeOf(head)
	if err != nil {
		return nil, err
	}
	src := &gitSource{r: r, tree: tree}
	tr := NewTree(src)
	opt.Now = ss.Now()
	reports, err := apply(tr, opt, from)
	if err != nil {
		return nil, err
	}
	if src.err != nil {
		return nil, src.err
	}
	if dryRun {
		return reports, nil
	}
	if err := setFormat(tr); err != nil {
		return nil, err
	}
	op := store.Op{Name: "convert", Detail: fmt.Sprintf("format %d to %d", from, model.StoreFormat)}
	if _, err := ss.CommitFiles(s, head, tr.Changes(), op); err != nil {
		return nil, err
	}
	if synced {
		if _, err := ss.RunSync(s, syncOpt, true); err != nil {
			return reports, fmt.Errorf("sync %s: %w", e.Name, err)
		}
	}
	return reports, nil
}

// Result is what hh store convert answers with --json.
type Result struct {
	DryRun bool     `json:"dry_run" doc:"Nothing was written"`
	Steps  []Report `json:"steps" doc:"What each step changed; empty when the store was in the newest format already"`
}

// ResultOf is the Result of reports.
func ResultOf(reports []Report, dryRun bool) Result {
	if reports == nil {
		reports = []Report{}
	}
	return Result{DryRun: dryRun, Steps: reports}
}

// apply runs the steps from format from on, each on what the one before made,
// and gives each report the format it read.
func apply(tr *Tree, opt Options, from int) ([]Report, error) {
	var reports []Report
	for f := from; f < model.StoreFormat; f++ {
		i := slices.IndexFunc(steps, func(st Step) bool { return st.From == f })
		if i < 0 {
			return nil, fmt.Errorf("there is no step that converts format %d", f)
		}
		rep, err := steps[i].Apply(tr, opt)
		if err != nil {
			return nil, fmt.Errorf("format %d to %d: %w", f, f+1, err)
		}
		rep.From = f
		rep.Lines = append(rep.Lines, fmt.Sprintf("store.yaml の format を %d から %d にした", f, f+1))
		reports = append(reports, rep)
	}
	return reports, nil
}

// setFormat makes store.yaml say the newest format.
func setFormat(tr *Tree) error {
	m, err := model.DecodeStoreMeta(tr.Read(model.StoreMetaPath))
	if err != nil {
		return fmt.Errorf("%s: %w", model.StoreMetaPath, err)
	}
	m.Format = model.StoreFormat
	b, err := m.Encode()
	if err != nil {
		return err
	}
	tr.Write(model.StoreMetaPath, b)
	return nil
}

// fastForwardLine says what a clone does that has nothing to convert because
// origin was converted already.
const fastForwardLine = "リモートがすでに新しい形式に変換されていて、push していない変更もないので、変換せずにリモートへ fast-forward "

// checkSynced is nil when the store is in sync with origin, as of the fetch
// that was just made, or in the state sync.format_diverged names: changes not
// pushed, and origin in a newer format than from. Origin with no commit at all
// has nothing the store could differ from. fastForward is true when the store
// has nothing that origin lacks and origin is in the newest format: there is
// nothing to convert, the store goes to origin.
func checkSynced(r *gitrepo.Repo, from int) (fastForward bool, err error) {
	remote, err := r.RemoteHead("origin")
	if err != nil {
		return false, err
	}
	if remote.IsZero() {
		return false, nil
	}
	head, err := r.Head()
	if err != nil {
		return false, err
	}
	has, err := r.IsAncestor(remote, head) // false: origin has a commit the store lacks
	if err != nil {
		return false, err
	}
	unpushed, err := replicate.Unpushed(r)
	if err != nil {
		return false, err
	}
	if has && unpushed == 0 {
		return false, nil
	}
	rf, ferr := replicate.FormatAt(r, remote)
	if !has && unpushed > 0 && ferr == nil && rf > from {
		return false, nil // sync.format_diverged: the sync after the conversion merges
	}
	if !has && unpushed == 0 && ferr == nil && rf == model.StoreFormat {
		return true, nil
	}
	return false, usecase.NewErr(usecase.KindConflict, "store.convert_unsynced",
		errors.New("the store is not in sync with its remote, so it is not converted; sync it first (the remote has changes the store lacks, or the store has changes it has not pushed)"))
}

// gitSource is the files of a commit's tree. A file that cannot be read is
// kept in err, for Run to stop on; to the step it is absent.
type gitSource struct {
	r    *gitrepo.Repo
	tree plumbing.Hash
	err  error
}

func (g *gitSource) Read(path string) []byte {
	b, err := g.r.ReadFile(g.tree, path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) && g.err == nil {
			g.err = err
		}
		return nil
	}
	if b == nil {
		b = []byte{}
	}
	return b
}

func (g *gitSource) Paths(prefix string) []string {
	var out []string
	for p, err := range g.r.Files(g.tree, prefix) {
		if err != nil {
			if g.err == nil {
				g.err = err
			}
			break
		}
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}
