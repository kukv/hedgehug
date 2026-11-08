// Package convert turns a store written in an older format into the newest
// one (hh store convert). The conversion is a row of steps, each of which
// rewrites the files of format N into those of format N+1 and knows nothing
// of the others; a new format adds a step and leaves the earlier ones alone.
// The old forms of the files are read in the steps and nowhere else.
package convert

import (
	"maps"
	"slices"
	"strings"
	"time"
)

// Step is one step of the conversion. From is the format it reads; Apply
// rewrites the files of a tree of that format into those of format From+1,
// except for the format number, which Run sets.
type Step struct {
	From  int
	Apply func(t *Tree, opt Options) (Report, error)
}

// Options is what the steps are told besides the tree.
type Options struct {
	// Kinds moves the tasks of a kind to another one (--kind old=new). The
	// new kinds are the fixed ones.
	Kinds map[string]string
	// Now is the time the tasks a step changes are stamped with; zero leaves
	// their time as it is.
	Now time.Time
}

// Report tells what a step changed, one line for each kind of change.
type Report struct {
	From  int      `json:"from" doc:"The format the step read; it wrote the next one"`
	Lines []string `json:"lines" doc:"What the step changed, one line for each kind of change"`
}

// Source is the files a tree starts from.
type Source interface {
	Paths(prefix string) []string // sorted
	Read(path string) []byte      // nil when absent
}

// Tree is the files of a store while it is converted: what Source has, with
// the writes of the steps laid over it. Nothing reaches the store until Run
// commits Changes.
type Tree struct {
	src    Source
	writes map[string][]byte // nil removes
}

// NewTree starts a tree from src.
func NewTree(src Source) *Tree { return &Tree{src: src, writes: map[string][]byte{}} }

// Read is the file at path, nil when there is none.
func (t *Tree) Read(path string) []byte {
	if b, ok := t.writes[path]; ok {
		return b
	}
	return t.src.Read(path)
}

// Write replaces the file at path, or makes it.
func (t *Tree) Write(path string, data []byte) {
	if data == nil {
		data = []byte{}
	}
	t.writes[path] = data
}

// Remove removes the file at path.
func (t *Tree) Remove(path string) { t.writes[path] = nil }

// Paths are the files whose path starts with prefix, sorted.
func (t *Tree) Paths(prefix string) []string {
	have := map[string]bool{}
	for _, p := range t.src.Paths(prefix) {
		if b, ok := t.writes[p]; !ok || b != nil {
			have[p] = true
		}
	}
	for p, b := range t.writes {
		if b != nil && strings.HasPrefix(p, prefix) {
			have[p] = true
		}
	}
	return slices.Sorted(maps.Keys(have))
}

// Changes are the files that differ from the source (nil removes a file), in
// the form Store.CommitMerge takes.
func (t *Tree) Changes() map[string][]byte {
	out := map[string][]byte{}
	for p, b := range t.writes {
		was := t.src.Read(p)
		if b == nil && was == nil || b != nil && was != nil && string(b) == string(was) {
			continue
		}
		out[p] = b
	}
	return out
}
