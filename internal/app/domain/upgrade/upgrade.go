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
	"sync"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

// Cx is what a step is told besides the file: the files of the store as it
// was in the format the step reads.
type Cx struct {
	Schema []byte // schema.yaml of that version; nil when it had none
	// Labels is the list labels: of the store as it is now (HEAD), which the
	// labels of a task of a past version are matched against, for a version of
	// format 3 has none of its own to speak of (step 3 to 4).
	Labels []string

	memo *defsMemo // the parsed Schema, when the Cx belongs to a Commit
}

// defsMemo is BuiltinDefsOf of the schema.yaml of a Commit, made once for all
// the files of the commit.
type defsMemo struct {
	once sync.Once
	defs BuiltinDefs
	err  error
}

// builtinDefs is BuiltinDefsOf cx.Schema, parsed once when cx belongs to a Commit.
func (cx Cx) builtinDefs() (BuiltinDefs, error) {
	if cx.memo == nil {
		return BuiltinDefsOf(cx.Schema)
	}
	cx.memo.once.Do(func() { cx.memo.defs, cx.memo.err = BuiltinDefsOf(cx.Schema) })
	return cx.memo.defs, cx.memo.err
}

// Step rewrites a task file of format From into the form of format From+1.
type Step struct {
	From  int
	Apply func(raw []byte, cx Cx) ([]byte, error)
	// Template does the same for a template file, named name: the kinds of
	// its items (format 1 to 2), the relations in its props (2 to 3), the
	// built-in properties and the labels (3 to 4); nil when
	// a format changes nothing in a template.
	Template func(name string, raw []byte, cx Cx) ([]byte, error)
}

// Steps is every step, in order of From. A new format adds one at the end.
// The tests put steps of their own here, with model.StoreFormat raised to
// match.
var Steps = []Step{
	{From: 1, Apply: task1to2, Template: template1to2},
	{From: 2, Apply: task2to3, Template: template2to3},
	{From: 3, Apply: task3to4, Template: template3to4},
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
	return run(from, to, raw, "task", func(s Step) func([]byte, Cx) ([]byte, error) { return s.Apply }, cx)
}

// TemplateTo is To for a template file, named name.
func TemplateTo(from, to int, name string, raw []byte, cx Cx) ([]byte, error) {
	if raw == nil {
		return raw, nil
	}
	return run(from, to, raw, "template", func(s Step) func([]byte, Cx) ([]byte, error) {
		if s.Template == nil {
			return nil
		}
		return func(raw []byte, cx Cx) ([]byte, error) { return s.Template(name, raw, cx) }
	}, cx)
}

// run applies the steps from format from to format to, each as pick gives it;
// a step that pick gives nothing for leaves the file as it is.
func run(from, to int, raw []byte, what string, pick func(Step) func([]byte, Cx) ([]byte, error), cx Cx) ([]byte, error) {
	for f := from; f < to; f++ {
		i := slices.IndexFunc(Steps, func(s Step) bool { return s.From == f })
		if i < 0 {
			return nil, fmt.Errorf("there is no step that upgrades a %s of format %d", what, f)
		}
		apply := pick(Steps[i])
		if apply == nil {
			continue
		}
		var err error
		if raw, err = apply(raw, cx); err != nil {
			return nil, fmt.Errorf("%s format %d to %d: %w", what, f, f+1, err)
		}
	}
	return raw, nil
}

// Commit upgrades the task files that one commit (or tree) of the store holds.
// It learns the format of the commit from its store.yaml, and the schema the
// steps need from its schema.yaml, the first time a file needs them.
type Commit struct {
	read   func(path string) ([]byte, error)
	head   func(path string) ([]byte, error) // the store as it is now
	format int                               // 0 until read
	ferr   error                             // of reading the format
	cx     *Cx
	err    error // of reading the schema
}

// At is the Commit whose files read gives. head gives the files of the store as
// it is now (HEAD), for what a step matches against the present (the labels).
// Both return an error that is fs.ErrNotExist for a file that is not there.
func At(read, head func(path string) ([]byte, error)) *Commit {
	return &Commit{read: read, head: head}
}

// FormatOf is the store format of the commit whose files read gives, for what
// asks no more than that.
func FormatOf(read func(path string) ([]byte, error)) (int, error) {
	return (&Commit{read: read}).Format()
}

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
	from, cx, err := c.context(to, raw)
	if err != nil {
		return nil, err
	}
	if cx == nil {
		return raw, nil
	}
	return To(from, to, raw, *cx)
}

// Template is raw, a template file of the commit called name, in the newest
// format.
func (c *Commit) Template(name string, raw []byte) ([]byte, error) {
	from, cx, err := c.context(model.StoreFormat, raw)
	if err != nil {
		return nil, err
	}
	if cx == nil {
		return raw, nil
	}
	return TemplateTo(from, model.StoreFormat, name, raw, *cx)
}

// context is the format of the commit and what the steps need, or a nil Cx when
// there is nothing to upgrade to format to.
func (c *Commit) context(to int, raw []byte) (int, *Cx, error) {
	from, err := c.Format()
	if err != nil {
		return 0, nil, err
	}
	if from >= to || raw == nil {
		return from, nil, nil
	}
	if c.cx == nil && c.err == nil {
		cx := Cx{}
		switch b, err := c.read(schema.Path); {
		case err == nil:
			cx.Schema = b
		case !errors.Is(err, fs.ErrNotExist):
			c.err = err
		}
		if c.err == nil && c.head != nil {
			switch b, err := c.head(schema.Path); {
			case err == nil:
				var d BuiltinDefs
				if d, err = BuiltinDefsOf(b); err != nil {
					c.err = fmt.Errorf("the schema of now: %w", err)
				}
				cx.Labels = d.Labels
			case !errors.Is(err, fs.ErrNotExist):
				c.err = err
			}
		}
		cx.memo = &defsMemo{}
		c.cx = &cx
	}
	if c.err != nil {
		return 0, nil, c.err
	}
	return from, c.cx, nil
}
