package history_test

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/history/historytest"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

func BenchmarkBuild(b *testing.B) {
	src := historytest.BenchSource(b)
	for b.Loop() {
		b.StopTimer()
		dir := filepath.Join(historytest.WorkDir(b), "store")
		historytest.Clone(b, src, dir)
		s, err := store.Open(dir)
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		if _, err := history.Open(s, history.Options{}); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		if info, err := os.Stat(filepath.Join(dir, ".hedgehug", "local", "history")); err == nil {
			b.ReportMetric(float64(info.Size()), "file-bytes")
		}
		_ = s.Repo().Close()
		_ = os.RemoveAll(filepath.Dir(dir))
		b.StartTimer()
	}
}

// BenchmarkOpenFresh is the load of an up-to-date index and one lookup (the
// commits of a task). Reading the commits from git is not in it: that is the
// work of whoever asks (see BenchmarkShow).
func BenchmarkOpenFresh(b *testing.B) {
	dir := historytest.Indexed(b, historytest.BenchSource(b))
	s, err := store.Open(dir)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = s.Repo().Close() }()
	var task model.ULID
	for t, err := range s.Tasks() {
		if err != nil {
			b.Fatal(err)
		}
		task = t.ULID
		break
	}
	for b.Loop() {
		h, err := history.Open(s, history.Options{})
		if err != nil {
			b.Fatal(err)
		}
		if len(h.Task(task)) == 0 {
			b.Fatal("the task has no commit")
		}
	}
}

// BenchmarkOpenAfterOneCommit is Open when one commit came after the index
// was saved, with the save.
func BenchmarkOpenAfterOneCommit(b *testing.B) {
	src := historytest.Indexed(b, historytest.BenchSource(b))
	for b.Loop() {
		b.StopTimer()
		dir := filepath.Join(historytest.WorkDir(b), "store")
		historytest.Clone(b, src, dir)
		s, err := store.Open(dir)
		if err != nil {
			b.Fatal(err)
		}
		var task model.ULID
		for t, err := range s.Tasks() {
			if err != nil {
				b.Fatal(err)
			}
			task = t.ULID
			break
		}
		a := historytest.Accounts[0]
		op := store.Op{Name: "edit", Task: task, Author: gitrepo.Signature{Name: a.Login, Email: a.Login + "@users.noreply.github.com"}}
		if _, err := s.Update(op, func(tx *store.Tx) error {
			t, err := tx.Task(task)
			if err != nil {
				return err
			}
			t.Title += " (changed)"
			return tx.PutTask(t)
		}); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		if _, err := history.Open(s, history.Options{}); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		_ = s.Repo().Close()
		_ = os.RemoveAll(filepath.Dir(dir))
		b.StartTimer()
	}
}

// addPack writes a pack of n random blobs of size bytes into the repository
// in dir, as a fetch does.
func addPack(b *testing.B, dir string, n, size int) {
	b.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		b.Fatal(err)
	}
	mem := memory.NewStorage()
	var hs []plumbing.Hash
	for range n {
		o := &plumbing.MemoryObject{}
		o.SetType(plumbing.BlobObject)
		buf := make([]byte, size)
		_, _ = rand.Read(buf)
		_, _ = o.Write(buf)
		h, err := mem.SetEncodedObject(o)
		if err != nil {
			b.Fatal(err)
		}
		hs = append(hs, h)
	}
	w, err := repo.Storer.(storer.PackfileWriter).PackfileWriter()
	if err != nil {
		b.Fatal(err)
	}
	if _, err := packfile.NewEncoder(w, mem, false).Encode(hs, 0); err != nil {
		b.Fatal(err)
	}
	if err := w.Close(); err != nil {
		b.Fatal(err)
	}
}

// BenchmarkConsolidate: "usual" is 20 packs of a fetch each; "toCap" adds
// packs of 1, 2, 4, 8, 16 and 24 MiB, so that the merge reaches about the
// size at which packs are left alone.
func BenchmarkConsolidate(b *testing.B) {
	src := historytest.BenchSource(b)
	for _, c := range []struct {
		name string
		mibs []int
	}{{"usual", nil}, {"toCap", []int{1, 2, 4, 8, 16, 24}}} {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				dir := filepath.Join(historytest.WorkDir(b), "store")
				historytest.Clone(b, src, dir)
				for _, m := range c.mibs {
					addPack(b, dir, m, 1<<20)
				}
				for range 20 {
					addPack(b, dir, 20, 4<<10)
				}
				r, err := gitrepo.Open(dir)
				if err != nil {
					b.Fatal(err)
				}
				b.StartTimer()
				err = r.Consolidate()
				b.StopTimer()
				if err != nil {
					b.Fatal(err)
				}
				n, _ := r.SmallPacks()
				b.ReportMetric(float64(n), "packs-left")
				_ = r.Close()
				_ = os.RemoveAll(filepath.Dir(dir))
				b.StartTimer()
			}
		})
	}
}
