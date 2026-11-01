package ops

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/doctor"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/merge"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// ViolationsText is the text of a refused write: what it breaks, and the
// violations on the lines after it.
func ViolationsText(ve *store.ValidationError) string {
	var b strings.Builder
	b.WriteString("the change breaks the schema:")
	for _, v := range ve.Violations {
		b.WriteString("\n  " + v.String())
	}
	return b.String()
}

// SchemaCheck opens the store and checks its schema. A schema.yaml that does
// not decode keeps the store from opening; that is then the one finding.
func (ss *Session) SchemaCheck() ([]doctor.Finding, error) {
	_, e, err := ss.Entry()
	if err != nil {
		return nil, err
	}
	s, err := ss.openStoreAt(e.Path)
	if err != nil {
		if data, rerr := os.ReadFile(filepath.Join(e.Path, schema.Path)); rerr == nil {
			if sc, fs := doctor.SchemaFile(data); sc == nil {
				return fs, nil
			}
		}
		return nil, err
	}
	// plain ids: the report is the store's own, under its heading
	ix, err := index.Open(s)
	if err != nil {
		return nil, err
	}
	return doctor.Schema(s, ix)
}

// MigrationPlan is what a migration writes: the new schema and the tasks
// it changes, and the task files it leaves since they do not decode.
type MigrationPlan struct {
	Schema  *schema.Schema
	Changed []*model.Task
	Skipped []model.ULID
	// Commit is the hex hash of the commit a migration made; "" for a dry
	// run and when nothing changed.
	Commit string
}

// Migrate applies m to the schema of the store and to its tasks. With dryRun
// it writes nothing. The index is the one from before the write in a dry
// run, and refreshed after a write.
func (ss *Session) Migrate(m schema.Migration, dryRun bool) (*index.Index, *MigrationPlan, error) {
	// A dry run writes nothing, so a read-only store may be planned.
	open := ss.OpenForWrite
	if dryRun {
		open = ss.Open
	}
	s, ix, err := open()
	if err != nil {
		return nil, nil, err
	}
	if dryRun {
		p, err := planMigration(s, ix, m)
		if err != nil {
			return nil, nil, err
		}
		return ix, p, nil
	}
	// The plan is made under the store lock, from the schema Update has
	// just read, so that no write in between is lost.
	var p *MigrationPlan
	hash, err := ss.UpdateCommit(s, store.Op{Name: "migrate", SkipTransitions: true}, func(tx *store.Tx) error {
		var err error
		if p, err = planMigration(s, ix, m); err != nil {
			return err
		}
		for _, t := range p.Changed {
			if err := tx.PutTask(t); err != nil {
				return err
			}
		}
		return tx.PutSchema(p.Schema)
	})
	if err != nil {
		// Name the task: the migration writes many.
		var ve *store.ValidationError
		if errors.As(err, &ve) {
			return nil, nil, newErr(KindInvalid, "migrate.invalid", fmt.Errorf("%s: %v", RefName(ix, ve.Task), ViolationsText(ve)))
		}
		return nil, nil, err
	}
	p.Commit = commitHex(hash)
	if err := ix.Refresh(); err != nil {
		return nil, nil, err
	}
	return ix, p, nil
}

// planMigration applies m to the schema of s and to its tasks, without
// writing anything. ix only names the tasks it refuses.
func planMigration(s *store.Store, ix *index.Index, m schema.Migration) (*MigrationPlan, error) {
	sc := s.Schema()
	next, err := sc.Migrate(m)
	if err != nil {
		return nil, err
	}
	if ps := next.Check(); len(ps) > 0 {
		return nil, &store.SchemaError{Problems: ps}
	}
	p := &MigrationPlan{Schema: next}
	var refused []string
	for t, err := range s.Tasks() {
		if err != nil {
			ulid, ok := brokenTask(err)
			if !ok {
				return nil, err
			}
			p.Skipped = append(p.Skipped, ulid)
			continue
		}
		ok, err := m.ApplyTask(t, sc)
		switch {
		case err != nil:
			refused = append(refused, RefName(ix, t.ULID)+": "+err.Error())
		case ok:
			p.Changed = append(p.Changed, t)
		}
	}
	if len(refused) > 0 {
		slices.Sort(refused)
		return nil, newErr(KindInvalid, "migrate.invalid", fmt.Errorf("cannot migrate %s:\n  %s", TaskCount(len(refused)), strings.Join(refused, "\n  ")))
	}
	slices.SortFunc(p.Changed, func(x, y *model.Task) int { return strings.Compare(x.ULID.String(), y.ULID.String()) })
	slices.SortFunc(p.Skipped, func(x, y model.ULID) int { return strings.Compare(x.String(), y.String()) })
	return p, nil
}

// brokenTask returns the ULID of the task file that err, from Store.Tasks,
// says does not decode or holds another task.
func brokenTask(err error) (model.ULID, bool) {
	var fe *store.FileError
	if !errors.As(err, &fe) || !errors.As(err, new(*model.DecodeError)) && !errors.Is(err, store.ErrULIDMismatch) {
		return model.ULID{}, false
	}
	ulid, perr := model.ParseTaskPath(fe.Path)
	return ulid, perr == nil
}

// TaskCount is n tasks in words: "1 task", "3 tasks".
func TaskCount(n int) string {
	if n == 1 {
		return "1 task"
	}
	return fmt.Sprintf("%d tasks", n)
}

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
	Store    string           `json:"store"`
	Repaired []Fixed          `json:"repaired"`
	Findings []doctor.Finding `json:"findings"`
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
			return nil, selectError(ss.StoreName, err)
		}
		entries = []config.StoreEntry{e}
	}
	// A swap after a history rewrite that stopped is finished first, so the
	// store opens (rewrite_leftover).
	// One that fails is that store's finding; the others go on.
	finishErr := map[string]error{}
	if fix {
		for _, e := range entries {
			if err := ss.finishRebuild(e); err != nil {
				finishErr[e.Name] = err
			}
		}
	}
	// The reports tell of a store that does not open; the mount need not.
	var broken []string
	for _, e := range entries {
		if _, err := ss.openStoreAt(e.Path); err != nil {
			broken = append(broken, e.Name)
		}
	}
	defer ss.muteOpenWarnings(broken)()
	// the duplicates are the same for every store: found once, when first needed
	var dups []duplicate
	var dupsDone bool
	duplicatesOnce := func() []duplicate {
		if !dupsDone {
			dupsDone = true
			var err error
			if dups, err = ss.duplicates(); err != nil {
				ss.warn("could not check duplicates across stores: " + err.Error())
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
			r.Findings = append(r.Findings, doctor.Finding{Check: "rewrite_leftover", Where: e.Path, Msg: "cannot finish the rebuild after a history rewrite: " + ferr.Error(), Hint: "fix the cause and run hedgehug doctor --fix again"})
		}
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// doctorFix repairs the store e for Doctor and returns what it fixed. A store
// that does not open is not repaired (the findings tell of it), nor is a
// read-only one (a warning says so).
func (ss *Session) doctorFix(c *config.Config, e config.StoreEntry) ([]Fixed, error) {
	s, err := ss.openStoreAt(e.Path)
	if err != nil {
		return nil, nil
	}
	repaired, err := ss.fixStore(c, e, s)
	var ro *ReadOnlyError
	switch {
	case errors.As(err, &ro):
		ss.warn(fmt.Sprintf("%s is read-only, so it is not fixed (%s)", e.Name, ro.Access.Why()))
		return nil, nil
	case err != nil:
		return nil, err
	case len(repaired) > 0:
		ss.dropMount() // the repair went in through a handle of its own
	}
	return repaired, nil
}

// doctorStore is Doctor for the store e, after the repairs. A store that does
// not open and has no finding is the error when it is the only one asked for,
// else a finding of its own, so that the reports of the others stand.
func (ss *Session) doctorStore(e config.StoreEntry, only bool, repaired []Fixed, duplicates func() []duplicate) (DoctorReport, error) {
	r := DoctorReport{Store: e.Name, Repaired: repaired}
	left, err := rewriteLeftoverFinding(e)
	if err != nil {
		return DoctorReport{}, err
	}
	if _, serr := os.Lstat(e.Path); os.IsNotExist(serr) && len(left) > 0 {
		// The swap after a history rewrite stopped before the rebuilt store
		// took the place: nothing else can be said until it is finished.
		r.Findings = left
		return r, nil
	}
	// A store that does not open is told of by the findings, which say why;
	// its references are not looked at.
	opened, openErr := ss.openStoreAt(e.Path)
	fs, err := doctor.Run(e.Path)
	if err != nil {
		return DoctorReport{}, err
	}
	fs = append(fs, left...)
	if openErr != nil && len(fs) == 0 {
		if only {
			return DoctorReport{}, openErr
		}
		fs = []doctor.Finding{{Check: "git", Where: e.Path, Msg: "cannot open the store: " + openErr.Error()}}
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
// history of origin rewritten without a record, and the file it left is
// still there.
func forcePushFinding(name string, s *store.Store) ([]doctor.Finding, error) {
	_, ok, err := sync.ReadForcePush(s)
	if err != nil || !ok {
		return nil, err
	}
	return []doctor.Finding{{Check: "unexpected_force_push", Where: name, Msg: sync.ErrForcePush.Error(), Hint: "check the history of origin; if it is right, clone the store again"}}, nil
}

// fixStore repairs the files of HEAD of the store e, opened as s, as a sync
// does, under the lock of a sync, and describes what it fixed.
func (ss *Session) fixStore(c *config.Config, e config.StoreEntry, s *store.Store) ([]Fixed, error) {
	if err := ss.checkWritable(s); err != nil {
		return nil, err
	}
	opt, err := ss.SyncOptions(c, e)
	if err != nil {
		return nil, err
	}
	var repaired []merge.Repaired
	err = s.WithLocalLock("sync.lock", syncLockWait, func() error {
		var err error
		repaired, err = doctor.Fix(s, opt)
		return err
	})
	if err != nil || len(repaired) == 0 {
		return nil, err
	}
	ss.markWritten(s)
	ix, err := ss.indexOf(e.Name, s)
	if err != nil {
		ix = nil // the ULIDs name what was repaired
	}
	out := make([]Fixed, 0, len(repaired))
	for _, x := range repaired {
		f := Fixed{ULID: x.ULID.String(), Kind: x.Kind, Value: jsonValue(x.Value), Author: x.Author}
		name := x.ULID.String()
		if ix != nil {
			if t, ok := ix.Task(x.ULID); ok {
				f.ID, f.Title, name = t.ID, t.Title, RefName(ix, x.ULID)
			} else if w, ok := ix.Workspace(x.ULID); ok {
				f.Title, name = w.Name, w.Name+" ("+x.ULID.String()+")"
			}
		}
		x.Value = detachedName(s, x.Kind, x.Value)
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
	}{x.Kind, jsonValue(x.Value)}) // a string and JSON always encode
	return RepairedContent(ix, inbox.Item{Detail: detail})
}
