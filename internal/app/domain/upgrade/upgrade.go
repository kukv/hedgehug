// Package upgrade reads the task files of an older store format as the newest
// one. Each format has a step that rewrites one task file into the next
// format; Upgrade runs them in order. Everything that reads a task file of a
// past commit (restore --at, show --at, undo, the inbox, the report cache, the
// base of a sync merge) goes through here, and the old forms of a task file
// are read in the steps and nowhere else.
//
// A step holds only what one task decides: moving a key, mapping a name,
// fixing the type of a value. What depends on the whole tree (moving a task to
// another place, making the 未分類 project) is left to the steps of hh store
// convert, which call these for what they share. A step does not move updated.
//
// The package is below the store and the sync so that they can use it; the
// steps of hh store convert (usecase/convert) are the other half of the same
// formats, and a test there checks that the two lists name the same formats.
package upgrade

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

// Cx is what a step is told besides the file: the files of the store as it
// was in the format the step reads.
type Cx struct {
	Schema []byte // schema.yaml of that version; nil when it had none
}

// Step rewrites a task file of format From into the form of format From+1.
type Step struct {
	From  int
	Apply func(raw []byte, cx Cx) ([]byte, error)
}

// Steps is every step, in order of From. A new format adds one at the end.
// The tests put steps of their own here, with model.StoreFormat raised to
// match.
var Steps = []Step{
	{From: 1, Apply: task1to2},
}

// Upgrade rewrites the task file raw, which is in format from, into the newest
// format.
func Upgrade(from int, raw []byte, cx Cx) ([]byte, error) {
	return To(from, model.StoreFormat, raw, cx)
}

// To is Upgrade up to format to. A forward (the stub of a task that moved to
// another store) is not a task and comes back as it is.
func To(from, to int, raw []byte, cx Cx) ([]byte, error) {
	if raw == nil || model.IsForward(raw) {
		return raw, nil
	}
	for f := from; f < to; f++ {
		i := slices.IndexFunc(Steps, func(s Step) bool { return s.From == f })
		if i < 0 {
			return nil, fmt.Errorf("there is no step that upgrades a task of format %d", f)
		}
		var err error
		if raw, err = Steps[i].Apply(raw, cx); err != nil {
			return nil, fmt.Errorf("task format %d to %d: %w", f, f+1, err)
		}
	}
	return raw, nil
}

// Commit upgrades the task files that one commit (or tree) of the store holds.
// It learns the format of the commit from its store.yaml, and the schema the
// steps need from its schema.yaml, the first time a file needs them.
type Commit struct {
	read   func(path string) ([]byte, error)
	format int   // 0 until read
	ferr   error // of reading the format
	cx     *Cx
	err    error // of reading the schema
}

// At is the Commit whose files read gives. read returns an error that is
// fs.ErrNotExist for a file the commit lacks.
func At(read func(path string) ([]byte, error)) *Commit { return &Commit{read: read} }

// Format is the store format of the commit: the format of its store.yaml. A
// store.yaml that is missing, does not decode, or is of a newer format than
// this version reads is an error; the format of a commit is never guessed.
func (c *Commit) Format() (int, error) {
	if c.format != 0 || c.ferr != nil {
		return c.format, c.ferr
	}
	b, err := c.read(model.StoreMetaPath)
	if err == nil {
		var m *model.StoreMeta
		if m, err = model.DecodeStoreMeta(b); err == nil {
			c.format = m.Format
			return c.format, nil
		}
	}
	c.ferr = fmt.Errorf("%s: %w", model.StoreMetaPath, err)
	return 0, c.ferr
}

// Task is raw, a task file of the commit, in the newest format.
func (c *Commit) Task(raw []byte) ([]byte, error) { return c.TaskTo(model.StoreFormat, raw) }

// TaskTo is Task up to format to, for what merges with a side in that format.
func (c *Commit) TaskTo(to int, raw []byte) ([]byte, error) {
	from, err := c.Format()
	if err != nil {
		return nil, err
	}
	if from >= to || raw == nil {
		return raw, nil
	}
	if c.cx == nil && c.err == nil {
		cx := Cx{}
		switch b, err := c.read(schema.Path); {
		case err == nil:
			cx.Schema = b
		case !errors.Is(err, fs.ErrNotExist):
			c.err = err
		}
		c.cx = &cx
	}
	if c.err != nil {
		return nil, c.err
	}
	return To(from, to, raw, *c.cx)
}
