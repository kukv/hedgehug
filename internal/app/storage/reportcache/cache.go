// Package reportcache reads what the reports need of every version of the
// task files, and keeps it in .hedgehug/local, by commit and path, so that
// only the commits made since are read from git.
package reportcache

import (
	"bytes"
	"encoding/gob"
	"errors"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/report"
	"github.com/kukv/hedgehug/internal/app/domain/upgrade"
	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

const (
	fileName = "report" // in .hedgehug/local
	format   = 4        // bump when row changes, or when a task is read in another way (3: past commits are upgraded to the newest store format; 4: points is a field of the task)
)

// OpMoveIn is the op of the commit that brings a task from another store.
const OpMoveIn = "move-in"

// row is what is kept of one version. A version that does not decode has Bad
// and is left out of the reports, and is not read again.
type row struct {
	Commit    plumbing.Hash
	Status    string
	Sprint    string
	Workspace string
	Created   int64
	Points    float64
	HasPoints bool
	Deleted   bool
	Gone      bool
	Bad       bool
}

type file struct {
	Format int
	Rows   map[string][]row // by path
}

// load reads the cache; an empty one when it is missing or of another format.
func load(s *store.Store) file {
	empty := file{Format: format, Rows: map[string][]row{}}
	b, err := s.ReadLocal(fileName)
	if err != nil {
		return empty
	}
	var f file
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&f); err != nil || f.Format != format {
		return empty
	}
	if f.Rows == nil {
		f.Rows = map[string][]row{}
	}
	return f
}

func save(s *store.Store, f file) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&f); err != nil {
		return
	}
	_ = s.WriteLocal(fileName, buf.Bytes()) // a failure costs only the next read
}

// upgrades read the task files of a commit in the newest store format, one
// reader for each commit, which many tasks share.
type upgrades map[plumbing.Hash]*upgrade.Commit

func (u upgrades) of(s *store.Store, commit plumbing.Hash) *upgrade.Commit {
	c, ok := u[commit]
	if !ok {
		c = upgrade.At(func(p string) ([]byte, error) { return s.Repo().ReadAt(commit, p) })
		u[commit] = c
	}
	return c
}

// readRow reads the file p as the commit has it.
func readRow(s *store.Store, ups upgrades, commit plumbing.Hash, p string, ulid model.ULID) (row, error) {
	r := row{Commit: commit}
	b, err := s.Repo().ReadAt(commit, p)
	if errors.Is(err, fs.ErrNotExist) {
		r.Gone = true
		return r, nil
	}
	if err != nil {
		return row{}, err
	}
	if model.IsForward(b) {
		r.Gone = true
		return r, nil
	}
	// A version from before a conversion is read in the newest format.
	if b, err = ups.of(s, commit).Task(b); err != nil {
		r.Bad = true
		return r, nil //nolint:nilerr // a version that does not upgrade is left out
	}
	t, err := model.DecodeTask(b)
	if err != nil || t.ULID != ulid {
		r.Bad = true
		return r, nil //nolint:nilerr // a version that does not decode is left out
	}
	r.Status, r.Workspace = t.Status, t.Workspace.String()
	r.Created = t.Created.Unix()
	r.Deleted = t.DeletedAt != nil
	if t.Sprint != nil {
		r.Sprint = t.Sprint.String()
	}
	if t.Points != nil {
		r.Points, r.HasPoints = *t.Points, true
	}
	return r, nil
}

func versionsOf(ulid string, commits []history.Commit, rows map[plumbing.Hash]row) report.Task {
	t := report.Task{ULID: ulid}
	// the history lists them newest first
	for i := len(commits) - 1; i >= 0; i-- {
		c := commits[i]
		r := rows[c.Hash]
		if r.Bad {
			continue
		}
		t.Versions = append(t.Versions, report.Version{
			Commit: c.Hash.String(), When: c.When, Status: r.Status, Sprint: r.Sprint, Workspace: r.Workspace,
			Created: time.Unix(r.Created, 0), Points: r.Points, HasPoints: r.HasPoints,
			Deleted: r.Deleted, Gone: r.Gone, MovedIn: c.Op == OpMoveIn,
		})
	}
	return t
}

// Read gives every task of the history h with its versions, by ULID.
func Read(s *store.Store, h *history.History) ([]report.Task, error) {
	old := load(s)
	next := file{Format: format, Rows: map[string][]row{}}
	changed := false
	ups := upgrades{}
	var out []report.Task
	for _, p := range h.Paths("tasks/") {
		ulid, err := model.ParseTaskPath(p)
		if err != nil {
			continue
		}
		commits := h.Path(p)
		known := make(map[plumbing.Hash]row, len(commits))
		for _, r := range old.Rows[p] {
			known[r.Commit] = r
		}
		rows := make([]row, 0, len(commits))
		for _, c := range commits {
			r, ok := known[c.Hash]
			if !ok {
				if r, err = readRow(s, ups, c.Hash, p, ulid); err != nil {
					return nil, err
				}
				known[c.Hash] = r
				changed = true
			}
			rows = append(rows, r)
		}
		changed = changed || len(rows) != len(old.Rows[p])
		next.Rows[p] = rows
		out = append(out, versionsOf(ulid.String(), commits, known))
	}
	if changed || len(next.Rows) != len(old.Rows) {
		save(s, next)
	}
	slices.SortFunc(out, func(a, b report.Task) int { return strings.Compare(a.ULID, b.ULID) })
	return out, nil
}

// Versions reads the versions of one task from git, without the cache, for a
// store whose other tasks do not matter.
func Versions(s *store.Store, h *history.History, ulid model.ULID) (report.Task, error) {
	p := model.TaskPath(ulid)
	commits := h.Path(p)
	rows := make(map[plumbing.Hash]row, len(commits))
	ups := upgrades{}
	for _, c := range commits {
		r, err := readRow(s, ups, c.Hash, p, ulid)
		if err != nil {
			return report.Task{}, err
		}
		rows[c.Hash] = r
	}
	return versionsOf(ulid.String(), commits, rows), nil
}

// Rows is how many versions the cache holds, for the tests.
func Rows(s *store.Store) int {
	n := 0
	for rows := range maps.Values(load(s).Rows) {
		n += len(rows)
	}
	return n
}
