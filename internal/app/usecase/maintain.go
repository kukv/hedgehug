package usecase

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/domain/merge"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/mount"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase/maintenance"
)

// Fixed is something doctor --fix repaired. Line is the text line, made when
// the repair is read, with the names the store has after it.
type Fixed struct {
	ULID   string          `json:"ulid"`
	ID     string          `json:"id,omitempty"`
	Title  string          `json:"title"`
	Kind   string          `json:"kind"`
	Value  json.RawMessage `json:"value"`
	Author string          `json:"author,omitempty"`
	Line   string          `json:"-"`
}

// DoctorReport is what Doctor found in one store: what --fix repaired, and
// the findings that are left, as doctor --fix --json prints them. Without
// --fix, Repaired is empty and only Findings is of interest.
type DoctorReport struct {
	Store    string                `json:"store"`
	Rebuilt  *Rebuilt              `json:"rebuilt,omitempty"`
	Repaired []Fixed               `json:"repaired"`
	Findings []maintenance.Finding `json:"findings"`
}

// Doctor finds what is wrong with the files, the work tree and git of the
// stores: the one --store names, or else every store of config.yaml, in its
// order, each with the references that lead to another mounted store. With
// fix it first repairs, in one commit per store, what a sync repairs; a
// store that is read-only is not repaired, and a warning says so.
func (ss *Session) Doctor(fix bool) ([]DoctorReport, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, err
	}
	entries := c.Stores
	if ss.StoreName != "" || len(entries) == 0 {
		e, err := c.Select(ss.StoreName)
		if err != nil {
			return nil, SelectError(ss.StoreName, err)
		}
		entries = []config.StoreEntry{e}
	}
	// A swap that stopped is finished first, so the store opens. A store a
	// sync found force pushed is then cloned again, before the repairs: they
	// are made on the new history. One that fails is that store's finding;
	// the others go on.
	finishErr := map[string]error{}
	rebuildErr := map[string]error{}
	rebuilt := make([]*Rebuilt, len(entries))
	if fix {
		for i, e := range entries {
			if err := ss.FinishRebuild(e); err != nil {
				finishErr[e.Name] = err
				continue
			}
			var err error
			if rebuilt[i], err = ss.rebuildStore(c, e); err != nil {
				rebuildErr[e.Name] = err
			}
		}
	}
	// The reports tell of a store that does not open; the mount need not.
	var broken []string
	for _, e := range entries {
		if s, err := ss.OpenStoreAt(e.Path); err != nil {
			broken = append(broken, e.Name)
		} else {
			_ = s.Repo().Close()
		}
	}
	defer ss.MuteOpenWarnings(broken)()
	// the duplicates are the same for every store: found once, when first needed
	var dups []Duplicate
	var dupsDone bool
	duplicatesOnce := func() []Duplicate {
		if !dupsDone {
			dupsDone = true
			var err error
			if dups, err = ss.Duplicates(); err != nil {
				ss.AddWarning("could not check duplicates across stores: " + err.Error())
			}
		}
		return dups
	}
	// Every store is repaired before any is looked at: a repair can renumber
	// a task, and the duplicates name the ids.
	repaired := make([][]Fixed, len(entries))
	if fix {
		for i, e := range entries {
			var err error
			if repaired[i], err = ss.doctorFix(c, e); err != nil {
				return nil, err
			}
		}
	}
	out := make([]DoctorReport, 0, len(entries))
	for i, e := range entries {
		r, err := ss.doctorStore(e, len(entries) == 1, repaired[i], duplicatesOnce)
		if ferr := finishErr[e.Name]; ferr != nil {
			if err != nil {
				// the store did not open for want of the finish: say that
				r, err = DoctorReport{Store: e.Name, Repaired: repaired[i]}, nil
			}
			r.Findings = append(r.Findings, maintenance.Finding{Check: "git", Where: e.Path, Msg: "cannot finish the rebuild after a history rewrite: " + ferr.Error(), Hint: "fix the cause and run hedgehug doctor --fix again"})
		}
		if rerr := rebuildErr[e.Name]; rerr != nil {
			if err != nil {
				r, err = DoctorReport{Store: e.Name, Repaired: repaired[i]}, nil
			}
			r.Findings = withRebuildFailure(r.Findings, e.Name, rerr)
		}
		if err != nil {
			return nil, err
		}
		r.Rebuilt = rebuilt[i]
		out = append(out, r)
	}
	return out, nil
}

// doctorFix repairs the store e for Doctor and returns what it fixed. A store
// that does not open is not repaired (the findings tell of it), nor is a
// read-only one (a warning says so).
func (ss *Session) doctorFix(c *config.Config, e config.StoreEntry) ([]Fixed, error) {
	s, err := ss.OpenStoreAt(e.Path)
	if err != nil {
		return nil, nil
	}
	defer func() { _ = s.Repo().Close() }()
	repaired, err := ss.fixStore(c, e, s)
	var ro *ReadOnlyError
	switch {
	case errors.As(err, &ro):
		ss.AddWarning(fmt.Sprintf("%s is read-only, so it is not fixed (%s)", e.Name, ro.Access.Why()))
		return nil, nil
	case err != nil:
		return nil, err
	case len(repaired) > 0:
		ss.DropMount() // the repair went in through a handle of its own
	}
	return repaired, nil
}

// doctorStore is Doctor for the store e, after the repairs. A store that does
// not open and has no finding is the error when it is the only one asked for,
// else a finding of its own, so that the reports of the others stand.
func (ss *Session) doctorStore(e config.StoreEntry, only bool, repaired []Fixed, duplicates func() []Duplicate) (DoctorReport, error) {
	r := DoctorReport{Store: e.Name, Repaired: repaired}
	// A store that does not open is told of by the findings, which say why;
	// its references are not looked at.
	opened, openErr := ss.OpenStoreAt(e.Path)
	if openErr == nil {
		defer func() { _ = opened.Repo().Close() }()
	}
	fs, err := maintenance.Run(e.Path)
	if err != nil {
		return DoctorReport{}, err
	}
	if openErr != nil && len(fs) == 0 {
		if only {
			return DoctorReport{}, openErr
		}
		fs = []maintenance.Finding{{Check: "git", Where: e.Path, Msg: "cannot open the store: " + openErr.Error()}}
	}
	if openErr == nil {
		fs = append(fs, ss.crossStoreFindings(e.Name)...)
		fs = append(fs, duplicateFindings(e.Name, duplicates())...)
		fs = append(fs, ss.forwardTargetFindings(e.Name)...)
		f, err := forcePushFinding(e.Name, opened)
		if err != nil {
			return DoctorReport{}, err
		}
		fs = append(fs, f...)
	}
	r.Findings = fs
	return r, nil
}

// forcePushFinding is unexpected_force_push (D23): the last sync found the
// history of origin differing from the store's, and the file it left is
// still there.
func forcePushFinding(name string, s *store.Store) ([]maintenance.Finding, error) {
	_, ok, err := replicate.ReadForcePush(s)
	if err != nil || !ok {
		return nil, err
	}
	return []maintenance.Finding{{Check: "unexpected_force_push", Where: name, Msg: replicate.ErrForcePush.Error(), Hint: "hedgehug doctor --fix clones the store again from origin and puts the unpushed changes back; the old store is kept when some could not be put back"}}, nil
}

// withRebuildFailure tells, in the unexpected_force_push finding the store
// already has, that cloning it again failed; a store without one gets a
// finding of its own. There is one finding of the force push either way.
func withRebuildFailure(fs []maintenance.Finding, name string, rerr error) []maintenance.Finding {
	const hint = "fix the cause and run hedgehug doctor --fix again"
	for i := range fs {
		if fs[i].Check == "unexpected_force_push" {
			fs[i].Msg += "; cannot clone the store again: " + rerr.Error()
			fs[i].Hint = hint
			return fs
		}
	}
	return append(fs, maintenance.Finding{Check: "unexpected_force_push", Where: name, Msg: "cannot clone the store again: " + rerr.Error(), Hint: hint})
}

// fixStore repairs the files of HEAD of the store e, opened as s, as a sync
// does, under the lock of a sync, and describes what it fixed.
func (ss *Session) fixStore(c *config.Config, e config.StoreEntry, s *store.Store) ([]Fixed, error) {
	if err := ss.CheckWritable(s); err != nil {
		return nil, err
	}
	opt, err := ss.SyncOptions(c, e)
	if err != nil {
		return nil, err
	}
	var repaired []merge.Repaired
	err = s.WithLocalLock("sync.lock", SyncLockWait, func() error {
		var err error
		repaired, err = maintenance.Fix(s, opt)
		return err
	})
	if err != nil || len(repaired) == 0 {
		return nil, err
	}
	ss.MarkWritten(s)
	ix, err := ss.IndexOf(e.Name, s)
	if err != nil {
		ix = nil // the ULIDs name what was repaired
	}
	out := make([]Fixed, 0, len(repaired))
	for _, x := range repaired {
		f := Fixed{ULID: x.ULID.String(), Kind: x.Kind, Value: JSONValue(x.Value), Author: x.Author}
		name := x.ULID.String()
		if ix != nil {
			if t, ok := ix.Task(x.ULID); ok {
				f.ID, f.Title, name = t.ID, t.Title, RefName(ix, x.ULID)
			} else if w, ok := ix.Workspace(x.ULID); ok {
				f.Title, name = w.Name, w.Name+" ("+x.ULID.String()+")"
			}
		}
		x.Value = DetachedName(s, x.Kind, x.Value)
		f.Line = fmt.Sprintf("repaired: %s: %s", name, fixedContent(ix, x))
		out = append(out, f)
	}
	return out, nil
}

// fixedContent says what a repair did: the id a clash was given, or the
// kind with what it took from, as the inbox tells of it.
func fixedContent(ix *index.Index, x merge.Repaired) string {
	if x.Kind == "id_collision" {
		var v struct{ Old, New string }
		if json.Unmarshal([]byte(x.Value), &v) == nil && v.Old != "" && v.New != "" {
			if ix != nil {
				v.Old, v.New = ix.QualifyID(v.Old), ix.QualifyID(v.New)
			}
			return fmt.Sprintf("id_collision %s → %s", v.Old, v.New)
		}
	}
	detail, _ := json.Marshal(struct {
		Kind  string          `json:"kind"`
		Value json.RawMessage `json:"value"`
	}{x.Kind, JSONValue(x.Value)}) // a string and JSON always encode
	return RepairedContent(ix, inbox.Item{Detail: detail})
}

// crossStoreFindings are the references of the store name to another mounted
// store, as warnings of doctor. A store whose references cannot be read is a
// warning of the session, not a finding: the other checks stand without it.
func (ss *Session) crossStoreFindings(name string) []maintenance.Finding {
	refs, err := ss.CrossRefs(name)
	if err != nil {
		ss.AddWarning("could not check cross-store references: " + err.Error())
		return nil
	}
	var out []maintenance.Finding
	for _, r := range refs {
		// the same words as the Where of a check of the store's own tasks
		where := FormatTaskLabel(r.From.ID, r.From.ID, r.From.ULID) + " " + r.From.Title
		out = append(out, maintenance.Finding{Check: "cross_store_ref", Where: where, Task: r.From.ULID.String(), Msg: CrossRefMsg(r), Severity: maintenance.Warning})
	}
	maintenance.Sort(out)
	return out
}

// duplicateFindings are the tasks of the store name that live in another
// mounted store too, as warnings of doctor. ds is what duplicates found.
func duplicateFindings(name string, ds []Duplicate) []maintenance.Finding {
	var out []maintenance.Finding
	for _, d := range ds {
		if d.In(name) {
			out = append(out, maintenance.Finding{Check: "duplicate_across_stores", Where: d.ULID.String(), Task: d.ULID.String(), Msg: d.Text(), Severity: maintenance.Warning})
		}
	}
	return out
}

// forwardTargetFindings are the forwards of the store name whose moved_to is
// no mounted store, as infos of doctor: the store may just not be mounted.
func (ss *Session) forwardTargetFindings(name string) []maintenance.Finding {
	set, err := ss.Mount()
	if err != nil {
		ss.AddWarning("could not check the forwards: " + err.Error())
		return nil
	}
	m, ok := set.Get(name)
	if !ok {
		return nil
	}
	var out []maintenance.Finding
	err = m.With(func(ix *index.Index) {
		for _, f := range ix.Forwards() {
			if f.Broken != "" || slices.ContainsFunc(set.Stores, func(o *mount.Mounted) bool { return o.ULID == f.MovedTo }) {
				continue
			}
			where := f.ULID.String()
			if f.ID != "" {
				where = fmt.Sprintf("%s (%s)", f.ID, f.ULID)
			}
			out = append(out, maintenance.Finding{Check: "forward_target", Where: where, Msg: fmt.Sprintf("moved to a store that is not mounted (ULID %s)", f.MovedTo), Severity: maintenance.Info})
		}
	})
	if err != nil {
		ss.WarnOnce("forward_target:"+name, fmt.Sprintf("could not read the index of %s, so its forwards are not checked: %v", name, err))
	}
	maintenance.Sort(out)
	return out
}
