package store

import (
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"path"
	"path/filepath"
	"strings"

	"github.com/kukv/hedgehug/internal/atomicfile"
	"github.com/kukv/hedgehug/internal/model"
)

// ErrULIDMismatch means a file holds a task whose ULID is not the one its path
// names, as a copy of another task's file would.
var ErrULIDMismatch = errors.New("the ulid does not match the file name")

// FileError means the file at Path, relative to the store, could not be
// decoded, or holds what its path does not name.
type FileError struct {
	Path string
	Err  error
}

func (e *FileError) Error() string { return e.Path + ": " + e.Err.Error() }

func (e *FileError) Unwrap() error { return e.Err }

// readDecode reads the file rel under dir and decodes it. A missing file is
// ErrNotFound; a decode error is a *FileError.
func readDecode[T any](dir, rel string, decode func([]byte) (T, error)) (T, error) {
	var zero T
	b, err := atomicfile.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if errors.Is(err, fs.ErrNotExist) {
		return zero, fmt.Errorf("%s: %w", rel, ErrNotFound)
	}
	if err != nil {
		return zero, err
	}
	v, err := decode(b)
	if err != nil {
		return zero, &FileError{Path: rel, Err: err}
	}
	return v, nil
}

// walk decodes every file under root that match accepts. It yields decode
// errors and goes on; a missing root yields nothing. The order is not fixed.
func walk[T any](s *Store, root string, match func(rel string) bool, decode func(rel string, b []byte) (T, error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T
		err := filepath.WalkDir(s.path(root), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(s.dir, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if !match(rel) {
				return nil
			}
			v, err := readDecode(s.dir, rel, func(b []byte) (T, error) { return decode(rel, b) })
			if !yield(v, err) {
				return fs.SkipAll
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			yield(zero, err)
		}
	}
}

// flatMatch accepts rel when it is the file build gives for the ULID in its
// name, like sprints/<ULID>.md.
func flatMatch(build func(model.ULID) string) func(string) bool {
	return func(rel string) bool {
		ulid, err := model.ParseULID(strings.TrimSuffix(path.Base(rel), ".md"))
		return err == nil && build(ulid) == rel
	}
}

func ignoreRel[T any](decode func([]byte) (T, error)) func(string, []byte) (T, error) {
	return func(_ string, b []byte) (T, error) { return decode(b) }
}

func (s *Store) Task(ulid model.ULID) (*model.Task, error) {
	return readDecode(s.dir, model.TaskPath(ulid), model.DecodeTask)
}

// Tasks returns every task file. One that does not decode, or holds a task
// of another ULID, yields a *FileError and no task.
func (s *Store) Tasks() iter.Seq2[*model.Task, error] {
	match := func(rel string) bool {
		_, err := model.ParseTaskPath(rel)
		return err == nil
	}
	decode := func(rel string, b []byte) (*model.Task, error) {
		t, err := model.DecodeTask(b)
		if err != nil {
			return nil, err
		}
		if model.TaskPath(t.ULID) != rel {
			return nil, fmt.Errorf("%w (ulid %s)", ErrULIDMismatch, t.ULID)
		}
		return t, nil
	}
	return walk(s, "tasks", match, decode)
}

// Comments returns the comments on task.
func (s *Store) Comments(task model.ULID) iter.Seq2[*model.Comment, error] {
	match := func(rel string) bool {
		t, _, err := model.ParseCommentPath(rel)
		return err == nil && t == task
	}
	return walk(s, path.Dir(model.CommentPath(task, model.ULID{})), match, ignoreRel(model.DecodeComment))
}

// Worklogs returns the worklog entries on task.
func (s *Store) Worklogs(task model.ULID) iter.Seq2[*model.Worklog, error] {
	match := func(rel string) bool {
		t, _, err := model.ParseWorklogPath(rel)
		return err == nil && t == task
	}
	return walk(s, path.Dir(model.WorklogPath(task, model.ULID{})), match, ignoreRel(model.DecodeWorklog))
}

func (s *Store) Sprint(ulid model.ULID) (*model.Sprint, error) {
	return readDecode(s.dir, model.SprintPath(ulid), model.DecodeSprint)
}

func (s *Store) Sprints() iter.Seq2[*model.Sprint, error] {
	return walk(s, "sprints", flatMatch(model.SprintPath), ignoreRel(model.DecodeSprint))
}

func (s *Store) Workspace(ulid model.ULID) (*model.Workspace, error) {
	return readDecode(s.dir, model.WorkspacePath(ulid), model.DecodeWorkspace)
}

func (s *Store) Workspaces() iter.Seq2[*model.Workspace, error] {
	return walk(s, "workspaces", flatMatch(model.WorkspacePath), ignoreRel(model.DecodeWorkspace))
}

// Numbers returns the record of purged numbers; empty when there is none.
func (s *Store) Numbers() (model.Numbers, error) {
	b, err := atomicfile.ReadFile(s.path(model.NumbersPath))
	if errors.Is(err, fs.ErrNotExist) {
		return model.Numbers{}, nil
	}
	if err != nil {
		return nil, err
	}
	n, err := model.DecodeNumbers(b)
	if err != nil {
		return nil, &FileError{Path: model.NumbersPath, Err: err}
	}
	return n, nil
}

func (s *Store) Release(ulid model.ULID) (*model.Release, error) {
	return readDecode(s.dir, model.ReleasePath(ulid), model.DecodeRelease)
}

func (s *Store) Releases() iter.Seq2[*model.Release, error] {
	return walk(s, "releases", flatMatch(model.ReleasePath), ignoreRel(model.DecodeRelease))
}

func (s *Store) Template(name string) (*model.Template, error) {
	if err := model.ValidTemplateName(name); err != nil {
		return nil, err
	}
	return readDecode(s.dir, model.TemplatePath(name), func(b []byte) (*model.Template, error) {
		return model.DecodeTemplate(name, b)
	})
}

func (s *Store) Templates() iter.Seq2[*model.Template, error] {
	match := func(rel string) bool {
		name, ok := templateName(rel)
		return ok && model.ValidTemplateName(name) == nil
	}
	decode := func(rel string, b []byte) (*model.Template, error) {
		name, _ := templateName(rel)
		return model.DecodeTemplate(name, b)
	}
	return walk(s, "templates", match, decode)
}

// TaskBytes returns the file of task ulid as it is, without decoding it.
func (s *Store) TaskBytes(ulid model.ULID) ([]byte, error) {
	rel := model.TaskPath(ulid)
	b, err := atomicfile.ReadFile(s.path(rel))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s: %w", rel, ErrNotFound)
	}
	return b, err
}

// CommentRef names one comment file.
type CommentRef struct{ Task, Comment model.ULID }

// CommentULIDs lists every comment in the store, in path order. Files under
// comments/ whose names are not comment paths are left out.
func (s *Store) CommentULIDs() ([]CommentRef, error) {
	var out []CommentRef
	err := filepath.WalkDir(s.path("comments"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(s.dir, p)
		if err != nil {
			return err
		}
		task, comment, err := model.ParseCommentPath(filepath.ToSlash(rel))
		if err != nil {
			return nil //nolint:nilerr // a file that is not a comment path is skipped
		}
		out = append(out, CommentRef{Task: task, Comment: comment})
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return out, nil
}

func templateName(rel string) (string, bool) {
	if path.Dir(rel) != "templates" || path.Ext(rel) != ".md" {
		return "", false
	}
	return strings.TrimSuffix(path.Base(rel), ".md"), true
}
