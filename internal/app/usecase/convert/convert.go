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
// reads the format before it at the end, and leaves the others alone. The
// tests put steps of their own here, with model.StoreFormat raised to match.
var steps = []Step{
	{From: 1, Apply: step2},
	{From: 2, Apply: step3},
	{From: 3, Apply: step4},
}

// Run converts the store of the session from its format to the newest, one
// step after the other, and returns what each step changed. With dryRun it
// writes nothing. Otherwise it writes the steps as one commit (op convert)
// and, for a store with a remote, pushes it.
//
// A store with a remote is converted in sync with it. When origin is in the
// same format and the two differ (origin has a commit the store lacks, or the
// store has one origin lacks), the store is first synced in that format (fetch,
// merge, push), and then converted and pushed. That sync repairs with the fixed
// table of the newest version, not with --kind: a child whose parent was deleted
// and whose kind is one of your own goes under 未分類 as it is. Only when that sync fails is it
// store.convert_unsynced, with the cause. The clones origin has gone ahead of
// in format, which the fetch shows, are the other case:
//   - With nothing not pushed, the clone goes to origin by fast-forward. When
//     origin is in a format that is not the newest yet, the steps that are
//     left are then converted and pushed as usual.
//   - With changes not pushed (the state a sync reports as
//     sync.format_diverged), the clone converts its own changes to the format
//     of origin, which a sync then merges; the steps that are left follow.
//
// The other way round, origin is in an older format than the store and has a
// commit the store lacks (another clone pushed it while this one converted;
// the sync reports it as sync.format_diverged): the steps are applied to the
// files of origin's head, up to the format of the store, as one commit on that
// head, which is pushed (a fast-forward of origin); the sync then merges it with
// the store like any other history. A store in the newest format with a remote
// looks at origin for this, and does nothing when there is none.
//
// Origin in a format newer than this version reads is store.newer_format. A
// store already in the newest format has nothing else to convert and returns
// no report.
//
// When the sync after a commit fails, the commit stays: the reports come back
// with the error.
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
	remote := len(e.Remotes) > 0
	if from >= model.StoreFormat && !remote {
		return nil, nil
	}
	r := s.Repo()
	opt.Now = ss.Now()
	var syncOpt replicate.Options
	var p plan
	if remote {
		if syncOpt, err = ss.SyncOptions(c, e); err != nil {
			return nil, err
		}
		// A dry run only looks: a remote it cannot reach, or one it is not in
		// sync with, is not its error.
		if err := replicate.FetchOrigin(s, syncOpt); err == nil {
			if p, err = checkSynced(r, from); err != nil && !dryRun && from < model.StoreFormat {
				return nil, err
			}
		} else if !dryRun {
			return nil, err
		}
	}
	// A store in the newest format has nothing of its own to convert; only an
	// origin that keeps changes in an older format is left to do.
	if from >= model.StoreFormat && !p.reverse {
		return nil, nil
	}
	if dryRun {
		return dryRunOf(r, opt, from, p)
	}

	var reports []Report
	// The same old format on both sides: sync first, as the format allows.
	if p.syncFirst {
		if _, err := ss.RunSync(s, syncOpt, true); err != nil {
			return nil, usecase.NewErr(usecase.KindConflict, "store.convert_unsynced",
				fmt.Errorf("the sync before the conversion failed, so the store is not converted: %w", err))
		}
	}
	// A clone with changes it has not pushed converts them to the format of
	// origin first; the sync merges.
	if p.diverged {
		rep, err := convertCommit(ss, s, opt, from, p.format)
		if err != nil {
			return nil, err
		}
		reports = append(reports, rep...)
		from = p.format
		if _, err := ss.RunSync(s, syncOpt, true); err != nil {
			return reports, fmt.Errorf("sync %s: %w", e.Name, err)
		}
	}
	// Origin has changes in an older format than the store (another clone
	// pushed them while this one converted): they are converted on origin's
	// head and pushed, and the sync merges them with the store.
	if p.reverse {
		rep, err := convertRemote(ss, s, opt, p.format, from)
		if err != nil {
			return nil, err
		}
		reports = append(reports, rep...)
		if _, err := ss.RunSync(s, syncOpt, true); err != nil {
			// The converted changes of origin are pushed; only the merge is left.
			return reports, fmt.Errorf("the changes of origin are converted and pushed, but the sync that merges them with this store failed; fix the cause and run hh sync to finish: sync %s: %w", e.Name, err)
		}
	}
	if p.fastForward {
		if err := replicate.FastForwardOrigin(s); err != nil {
			return nil, err
		}
		ss.DropMount()
		reports = append(reports, Report{From: from, To: p.format, Lines: []string{fastForwardLine(p.format, "した")}})
		from = p.format
	}
	if from >= model.StoreFormat {
		return reports, nil
	}
	rep, err := convertCommit(ss, s, opt, from, model.StoreFormat)
	if err != nil {
		return reports, err
	}
	reports = append(reports, rep...)
	if remote {
		if _, err := ss.RunSync(s, syncOpt, true); err != nil {
			return reports, fmt.Errorf("sync %s: %w", e.Name, err)
		}
	}
	return reports, nil
}

// convertCommit applies the steps from format from to format to on HEAD and
// commits the result as one commit.
func convertCommit(ss *usecase.Session, s *store.Store, opt Options, from, to int) ([]Report, error) {
	r := s.Repo()
	head, err := r.Head()
	if err != nil {
		return nil, err
	}
	tr, src, err := treeAt(r, head)
	if err != nil {
		return nil, err
	}
	reports, err := apply(tr, opt, from, to)
	if err != nil {
		return nil, err
	}
	if src.err != nil {
		return nil, src.err
	}
	if err := setFormat(tr, to); err != nil {
		return nil, err
	}
	op := store.Op{Name: "convert", Detail: fmt.Sprintf("format %d to %d", from, to)}
	if _, err := ss.CommitFiles(s, head, tr.Changes(), op); err != nil {
		return nil, err
	}
	return reports, nil
}

// beforeRemotePush runs before convertRemote pushes; the tests make another
// clone push here.
var beforeRemotePush = func() {}

// convertRemote applies the steps from format from to format to on the files
// of origin's head, commits the result as one commit whose parent is that head
// and pushes it, so that origin goes ahead by a fast-forward and the commit
// is the one the store merges with. The first report says what was done.
func convertRemote(ss *usecase.Session, s *store.Store, opt Options, from, to int) ([]Report, error) {
	r := s.Repo()
	remote, err := r.RemoteHead("origin")
	if err != nil {
		return nil, err
	}
	tr, src, err := treeAt(r, remote)
	if err != nil {
		return nil, err
	}
	done, err := apply(tr, opt, from, to)
	if err != nil {
		return nil, err
	}
	if src.err != nil {
		return nil, src.err
	}
	if err := setFormat(tr, to); err != nil {
		return nil, err
	}
	op := store.Op{Name: "convert", Detail: fmt.Sprintf("format %d to %d, the changes of origin", from, to)}
	c, err := ss.CommitOnParent(s, remote, tr.Changes(), op)
	if err != nil {
		return nil, err
	}
	beforeRemotePush()
	err = s.WithLocalLock("sync.lock", usecase.SyncLockWait, func() error {
		return r.PushCommit("origin", c)
	})
	if errors.Is(err, gitrepo.ErrRejected) {
		return nil, usecase.NewErr(usecase.KindConflict, "store.convert_unsynced",
			fmt.Errorf("origin has gone ahead while its changes were converted, so nothing was converted; run hh store convert again: %w", err))
	}
	if err != nil {
		return nil, err
	}
	return append([]Report{{From: from, To: to, Lines: []string{remoteChangesLine(from, to, "した")}}}, done...), nil
}

// remoteChangesLine says that the changes of origin, which are in an older
// format than the store, are converted on origin.
func remoteChangesLine(from, to int, when string) string {
	return fmt.Sprintf("リモートの変更は古い形式 %d のままなので、形式 %d に変換してリモートの先頭の上にコミットし、push %s", from, to, when)
}

// dryRunOf is what Run would report, written nowhere. A clone that would go
// to origin by fast-forward is listed with the steps that would follow, read
// from origin's files; one with changes not pushed, with the steps from its
// own files.
func dryRunOf(r *gitrepo.Repo, opt Options, from int, p plan) ([]Report, error) {
	var reports []Report
	at, err := r.Head()
	if err != nil {
		return nil, err
	}
	if p.syncFirst {
		reports = append(reports, Report{From: from, To: from, Lines: []string{"先に古い形式のまま remote と同期する（--dry-run のため何も書かない）"}})
	}
	if p.reverse {
		remote, err := r.RemoteHead("origin")
		if err != nil {
			return nil, err
		}
		tr, src, err := treeAt(r, remote)
		if err != nil {
			return nil, err
		}
		done, err := apply(tr, opt, p.format, from)
		if err != nil {
			return nil, err
		}
		if src.err != nil {
			return nil, src.err
		}
		reports = append(reports, Report{From: p.format, To: from, Lines: []string{remoteChangesLine(p.format, from, "する（--dry-run のため何も書かない）")}})
		reports = append(reports, done...)
	}
	if p.fastForward {
		if at, err = r.RemoteHead("origin"); err != nil {
			return nil, err
		}
		reports = append(reports, Report{From: from, To: p.format, Lines: []string{fastForwardLine(p.format, "する（--dry-run のため何も書かない）")}})
		from = p.format
	}
	if from >= model.StoreFormat {
		return reports, nil
	}
	tr, src, err := treeAt(r, at)
	if err != nil {
		return nil, err
	}
	rest, err := apply(tr, opt, from, model.StoreFormat)
	if err != nil {
		return nil, err
	}
	if src.err != nil {
		return nil, src.err
	}
	return append(reports, rest...), nil
}

// treeAt is the files of the commit as a tree for the steps.
func treeAt(r *gitrepo.Repo, commit plumbing.Hash) (*Tree, *gitSource, error) {
	tree, err := r.TreeOf(commit)
	if err != nil {
		return nil, nil, err
	}
	src := &gitSource{r: r, tree: tree}
	return NewTree(src), src, nil
}

// fastForwardLine says that a clone with nothing not pushed, which origin has
// gone ahead of in format, goes to the format of origin without converting.
func fastForwardLine(to int, when string) string {
	return fmt.Sprintf("リモートがすでに形式 %d に変換されていて、push していない変更もないので、変換せずにリモートへ fast-forward %s", to, when)
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

// apply runs the steps from format from up to format to, each on what the one
// before made, and gives each report the formats it read and wrote.
func apply(tr *Tree, opt Options, from, to int) ([]Report, error) {
	var reports []Report
	for f := from; f < to; f++ {
		i := slices.IndexFunc(steps, func(st Step) bool { return st.From == f })
		if i < 0 {
			return nil, fmt.Errorf("there is no step that converts format %d", f)
		}
		rep, err := steps[i].Apply(tr, opt)
		if err != nil {
			return nil, fmt.Errorf("format %d to %d: %w", f, f+1, err)
		}
		rep.From, rep.To = f, f+1
		rep.Lines = append(rep.Lines, fmt.Sprintf("store.yaml の format を %d から %d にした", f, f+1))
		reports = append(reports, rep)
	}
	return reports, nil
}

// setFormat makes store.yaml say format to.
func setFormat(tr *Tree, to int) error {
	m, err := model.DecodeStoreMeta(tr.Read(model.StoreMetaPath))
	if err != nil {
		return fmt.Errorf("%s: %w", model.StoreMetaPath, err)
	}
	m.Format = to
	b, err := m.Encode()
	if err != nil {
		return err
	}
	tr.Write(model.StoreMetaPath, b)
	return nil
}

// plan is what the fetch showed about origin, in format terms.
type plan struct {
	fastForward bool // nothing not pushed, origin ahead in format: go to origin
	diverged    bool // changes not pushed, origin ahead in format: convert them to format
	format      int  // the format of origin, for the three of them
	syncFirst   bool // origin in the same format, not in sync: sync before converting
	reverse     bool // origin has commits the store lacks, in a format older than the store's: convert them on origin
}

// checkSynced is the zero plan when the store is in sync with origin, as of
// the fetch that was just made, or when origin has no commit at all (nothing
// the store could differ from). When origin is in a format ahead of from, but
// not past the newest, the plan is fastForward (the store has nothing origin
// lacks) or diverged (it has). Otherwise it is syncFirst, or
// store.newer_format for an origin this version cannot read.
func checkSynced(r *gitrepo.Repo, from int) (plan, error) {
	remote, err := r.RemoteHead("origin")
	if err != nil {
		return plan{}, err
	}
	if remote.IsZero() {
		return plan{}, nil
	}
	rf, ferr := replicate.FormatAt(r, remote)
	if errors.Is(ferr, model.ErrNewerFormat) {
		return plan{}, ferr
	}
	head, err := r.Head()
	if err != nil {
		return plan{}, err
	}
	has, err := r.IsAncestor(remote, head) // false: origin has a commit the store lacks
	if err != nil {
		return plan{}, err
	}
	unpushed, err := replicate.Unpushed(r)
	if err != nil {
		return plan{}, err
	}
	switch {
	case has && unpushed == 0:
		return plan{}, nil
	case !has && ferr == nil && rf > from && rf <= model.StoreFormat:
		return plan{fastForward: unpushed == 0, diverged: unpushed > 0, format: rf}, nil
	}
	if ferr == nil && rf < from && !has {
		// Origin has changes the store lacks, in an older format than the
		// store's: a sync would not merge them. They are converted on origin.
		return plan{reverse: true, format: rf}, nil
	}
	// The same format, or one origin's files do not tell: a sync merges them.
	return plan{syncFirst: true}, nil
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
