// Package merge merges the files of a store three ways, field by field. Sync,
// the git merge driver and undo share it.
package merge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// Side is one side of a three-way merge of one file.
type Side struct {
	Data   []byte                        // nil when the file is not there
	When   func(path []string) time.Time // when the side last changed the yaml value at path (see YAMLValues); nil outside yaml files
	Author string                        // the account (or email) that last changed the file on this side
	Time   time.Time                     // when the side last changed the file (for "other files")
}

// Overwritten is a value that lost to the other side.
type Overwritten struct {
	Path   string // the file
	Field  string // title, props.priority...; * for the whole file
	Value  string // the losing value as one line of JSON (the SHA-1 for other files)
	Author string // who wrote the losing value
}

type NoticeKind string

const Revived NoticeKind = "revived"

// Notice is something the inbox shows besides overwritten values.
type Notice struct {
	Kind   NoticeKind
	Path   string
	Author string // whom to tell
}

type Result struct {
	Data        []byte // nil removes the file
	Overwritten []Overwritten
	Notices     []Notice
}

type Policy int

const (
	LWW      Policy = iota // sync and the merge driver
	OursWins               // undo: ours takes every field both sides changed
)

// Merge merges one file of the store. The kind of the file comes from its
// path. Both orders of ours and theirs give the same result.
func Merge(path string, sc *schema.Schema, base []byte, ours, theirs Side, p Policy) (Result, error) {
	if bytes.Equal(ours.Data, theirs.Data) {
		return Result{Data: ours.Data}, nil
	}
	m := &merger{path: path, sc: sc, p: p}
	if kindOf(path) == kindNumbers {
		// Numbers never go down: a side that lowered them or removed the
		// file does not win because the other side left the file alone.
		return m.numbers(base, ours, theirs)
	}
	if bytes.Equal(base, ours.Data) {
		return Result{Data: theirs.Data}, nil
	}
	if bytes.Equal(base, theirs.Data) {
		return Result{Data: ours.Data}, nil
	}
	switch kindOf(path) {
	case kindTask:
		return mergeFile(m, base, ours, theirs, taskKind)
	case kindComment:
		return m.record(ours, theirs, commentUpdated)
	case kindWorklog:
		return m.record(ours, theirs, worklogUpdated)
	case kindSprint, kindRelease, kindTemplate, kindWorkspace:
		return m.plan(base, ours, theirs)
	case kindYAML:
		return m.yaml(base, ours, theirs)
	}
	return m.other(ours, theirs)
}

type kind int

const (
	kindOther kind = iota
	kindTask
	kindComment
	kindWorklog
	kindSprint
	kindRelease
	kindTemplate
	kindWorkspace
	kindYAML    // schema.yaml, automations.yaml, store.yaml and views/*.yaml, merged key by key
	kindNumbers // numbers.yaml, merged by the largest number
)

// ByKey reports whether the file at path is merged key by key, as
// schema.yaml is: the merge then asks Side.When when each side changed a
// value.
func ByKey(path string) bool { return kindOf(path) == kindYAML }

func kindOf(path string) kind {
	switch path {
	case schema.Path, model.AutomationsPath, model.StoreMetaPath:
		return kindYAML
	case model.NumbersPath:
		return kindNumbers
	}
	if _, ok := model.IsViewPath(path); ok {
		return kindYAML
	}
	for _, d := range []struct {
		dir string
		k   kind
	}{
		{"tasks/", kindTask},
		{"comments/", kindComment},
		{"worklogs/", kindWorklog},
		{"sprints/", kindSprint},
		{"releases/", kindRelease},
		{"templates/", kindTemplate},
		{"workspaces/", kindWorkspace},
	} {
		if strings.HasPrefix(path, d.dir) {
			return d.k
		}
	}
	return kindOther
}

// merger holds what the merge of one file needs besides the sides.
type merger struct {
	path string
	sc   *schema.Schema
	p    Policy
}

// later reports whether side a beats side b for a value both changed:
// the newer time, then the larger author, then the larger encoded value.
func later(at, bt time.Time, aAuthor, bAuthor string, aVal, bVal []byte) bool {
	if !at.Equal(bt) {
		return at.After(bt)
	}
	if aAuthor != bAuthor {
		return aAuthor > bAuthor
	}
	return bytes.Compare(aVal, bVal) > 0
}

// top reports whether the result is built on ours: the side with the larger
// author, then the larger bytes. Choosing by the sides and not by their
// order keeps the comments and key order of the result symmetric.
func top(ours, theirs Side) bool {
	if ours.Author != theirs.Author {
		return ours.Author > theirs.Author
	}
	return bytes.Compare(ours.Data, theirs.Data) > 0
}

// revive keeps the side that changed a file the other side removed, and
// tells whoever removed it.
func (m *merger) revive(ours, theirs Side) Result {
	kept, gone := ours, theirs
	if ours.Data == nil {
		kept, gone = theirs, ours
	}
	return Result{Data: kept.Data, Notices: []Notice{{Kind: Revived, Path: m.path, Author: gone.Author}}}
}

// jsonOf encodes v, a Go value of a file or a *yaml.Node, as one line of
// JSON. A nil node is null.
func jsonOf(v any) []byte {
	n, ok := v.(*yaml.Node)
	if !ok {
		n = new(yaml.Node)
		if err := n.Encode(v); err != nil {
			b, _ := json.Marshal(fmt.Sprint(v))
			return b
		}
	}
	if n == nil {
		return []byte("null")
	}
	var x any
	if n.Decode(&x) == nil {
		if b, err := json.Marshal(x); err == nil {
			return b
		}
	}
	// A value with no JSON form, such as a mapping with keys that are not
	// strings, is given as its YAML.
	out, _ := yaml.Dump(n, yaml.WithLineWidth(-1))
	b, _ := json.Marshal(strings.TrimSuffix(string(out), "\n"))
	return b
}
