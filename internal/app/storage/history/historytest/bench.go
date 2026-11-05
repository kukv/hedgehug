package historytest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// The benchmarks of the history index, and of the commands that read it, run
// on a synthesized store of BenchCommits commits and BenchTasks tasks (see
// make bench-history). They are skipped unless HH_BENCH is set.
const (
	BenchCommits = 100_000
	BenchTasks   = 5_000
	benchSeed    = 1
)

// BenchSource is the synthesized store of the benchmarks. It skips b unless
// HH_BENCH is set.
func BenchSource(b *testing.B) string {
	b.Helper()
	if os.Getenv("HH_BENCH") == "" {
		b.Skip("set HH_BENCH=1 (make bench-history) to measure on a store of 100,000 commits")
	}
	return Synth(b, BenchCommits, BenchTasks, benchSeed)
}

// WorkDir is a directory next to the synthesized stores, so that the packs
// can be linked into the copies.
func WorkDir(b *testing.B) string {
	b.Helper()
	d, err := os.MkdirTemp(CacheDir(b), "work-")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = os.RemoveAll(d) })
	return d
}

// Indexed is a copy of the synthesized store with its history index and its
// task index (both caches in .hedgehug/local) made, in its own directory.
func Indexed(b *testing.B, src string) string {
	b.Helper()
	dir := filepath.Join(WorkDir(b), "store")
	Clone(b, src, dir)
	s, err := store.Open(dir)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := history.Open(s, history.Options{}); err != nil {
		b.Fatal(err)
	}
	if _, err := index.Open(s); err != nil {
		b.Fatal(err)
	}
	_ = s.Repo().Close()
	return dir
}
