package schemas

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/osfs"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

// SchemaCheck opens the store and checks its schema. A schema.yaml that does
// not decode keeps the store from opening; that is then the one finding.
func SchemaCheck(ss *usecase.Session) ([]usecase.Finding, error) {
	_, e, err := ss.Entry()
	if err != nil {
		return nil, err
	}
	s, err := ss.OpenStoreAt(e.Path)
	if err != nil {
		if data, rerr := osfs.ReadFile(filepath.Join(e.Path, schema.Path)); rerr == nil {
			if sc, fs := usecase.CheckSchemaFile(data); sc == nil {
				return fs, nil
			}
		}
		return nil, err
	}
	defer func() { _ = s.Repo().Close() }()
	// plain ids: the report is the store's own, under its heading
	ix, err := index.Open(s)
	if err != nil {
		return nil, err
	}
	return usecase.CheckSchema(s, ix)
}

// CheckResult is what schema check answers, in the CLI's --json and in the
// server's response.
type CheckResult struct {
	Findings []usecase.Finding `json:"findings"`
}

// CheckResultOf is fs as the answer; no findings are [], not null.
func CheckResultOf(fs []usecase.Finding) CheckResult {
	return CheckResult{Findings: append([]usecase.Finding{}, fs...)}
}

// MigrateWrite is what migrate answers, in the CLI's --json and in the
// server's response.
type MigrateWrite struct {
	Changed []*usecase.TaskView `json:"changed"`
	Skipped []string            `json:"skipped"`
	Commit  *string             `json:"commit"`
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
// it writes nothing. The tasks changed are read from the store after a
// write, and are the migrated tasks in a dry run.
func Migrate(ss *usecase.Session, m schema.Migration, dryRun bool) (MigrateWrite, error) {
	ix, p, err := migrate(ss, m, dryRun)
	if err != nil {
		return MigrateWrite{}, err
	}
	out := MigrateWrite{Changed: []*usecase.TaskView{}, Skipped: []string{}, Commit: usecase.CommitPtr(p.Commit)}
	for _, t := range p.Changed {
		e, ok := ix.Task(t.ULID)
		if dryRun || !ok {
			e = index.EntryOf(t) // the task as migrated
		}
		out.Changed = append(out.Changed, usecase.ListViewOf(ix, e))
	}
	for _, u := range p.Skipped {
		out.Skipped = append(out.Skipped, u.String())
	}
	return out, nil
}

// migrate is Migrate before the answer is made. The index is the one from
// before the write in a dry run, and refreshed after a write.
func migrate(ss *usecase.Session, m schema.Migration, dryRun bool) (*index.Index, *MigrationPlan, error) {
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
			return nil, nil, usecase.NewErr(usecase.KindInvalid, "migrate.invalid", fmt.Errorf("%s: %v", usecase.RefName(ix, ve.Task), usecase.ViolationsText(ve.Violations)))
		}
		return nil, nil, err
	}
	p.Commit = usecase.CommitHex(hash)
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
			refused = append(refused, usecase.RefName(ix, t.ULID)+": "+err.Error())
		case ok:
			p.Changed = append(p.Changed, t)
		}
	}
	if len(refused) > 0 {
		slices.Sort(refused)
		return nil, usecase.NewErr(usecase.KindInvalid, "migrate.invalid", fmt.Errorf("cannot migrate %s:\n  %s", TaskCount(len(refused)), strings.Join(refused, "\n  ")))
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
	return plural.Count(n, "task")
}
