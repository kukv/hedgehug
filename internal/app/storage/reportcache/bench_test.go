package reportcache

import (
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/history/historytest"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

func benchStore(b *testing.B) (*store.Store, *history.History) {
	b.Helper()
	dir := filepath.Join(historytest.WorkDir(b), "store")
	historytest.Clone(b, historytest.BenchSource(b), dir)
	s, err := store.Open(dir)
	if err != nil {
		b.Fatal(err)
	}
	h, err := history.Open(s, history.Options{})
	if err != nil {
		b.Fatal(err)
	}
	return s, h
}

// BenchmarkReadFirst reads every version from git.
func BenchmarkReadFirst(b *testing.B) {
	s, h := benchStore(b)
	for b.Loop() {
		b.StopTimer()
		if err := s.WriteLocal(fileName, nil); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		if _, err := Read(s, h); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkReadCached reads them from the cache.
func BenchmarkReadCached(b *testing.B) {
	s, h := benchStore(b)
	if _, err := Read(s, h); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := Read(s, h); err != nil {
			b.Fatal(err)
		}
	}
}
