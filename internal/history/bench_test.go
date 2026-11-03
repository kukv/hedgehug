package history_test

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/history"
	"github.com/kukv/hedgehug/internal/history/historytest"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

// The benchmarks run on a synthesized store of 100,000 commits and 5,000
// tasks (see make bench-history). They are skipped unless HH_BENCH is set.
const (
	benchCommits = 100_000
	benchTasks   = 5_000
	benchSeed    = 1
)

func benchSource(b *testing.B) string {
	b.Helper()
	if os.Getenv("HH_BENCH") == "" {
		b.Skip("set HH_BENCH=1 (make bench-history) to measure on a store of 100,000 commits")
	}
	return historytest.Synth(b, benchCommits, benchTasks, benchSeed)
}

// workDir is a directory next to the synthesized stores, so that the packs
// can be linked into the copies.
func workDir(b *testing.B) string {
	b.Helper()
	d, err := os.MkdirTemp(historytest.CacheDir(b), "work-")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = os.RemoveAll(d) })
	return d
}

// indexed is a copy of the synthesized store with its history index and its
// task index (both caches in .hedgehug/local) made, in its own directory.
func indexed(b *testing.B, src string) string {
	b.Helper()
	dir := filepath.Join(workDir(b), "store")
	historytest.Clone(b, src, dir)
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

// session is a session on the store at dir, as the account (by name).
func session(b *testing.B, dir, account string) *ops.Session {
	b.Helper()
	home := b.TempDir()
	env := map[string]string{"HOME": home, "HEDGEHUG_HOME": filepath.Join(home, "hh"), "HEDGEHUG_AUTO_SYNC": "0"}
	now := historytest.Start.Add(2 * benchCommits * time.Minute)
	ss := &ops.Session{
		Getenv: func(k string) string { return env[k] },
		Now:    func() time.Time { return now },
		Warn:   func(string) {},
	}
	c, _, err := ss.Config()
	if err != nil {
		b.Fatal(err)
	}
	if err := c.AddStore("me", dir, nil); err != nil {
		b.Fatal(err)
	}
	if err := c.Save(); err != nil {
		b.Fatal(err)
	}
	var acct config.Account
	for _, a := range append(historytest.Accounts, config.Account{Name: "dave", Host: "github.com", Login: "dave"}) {
		if a.Name == account {
			acct = a
		}
	}
	f, err := os.OpenFile(filepath.Join(env["HEDGEHUG_HOME"], "config.yaml"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // the config of the benchmark
	if err != nil {
		b.Fatal(err)
	}
	_, err = f.WriteString("accounts:\n  - name: " + acct.Name + "\n    host: " + acct.Host + "\n    login: " + acct.Login + "\n")
	if err := errors.Join(err, f.Close()); err != nil {
		b.Fatal(err)
	}
	return ss
}

func BenchmarkBuild(b *testing.B) {
	src := benchSource(b)
	for b.Loop() {
		b.StopTimer()
		dir := filepath.Join(workDir(b), "store")
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
	dir := indexed(b, benchSource(b))
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
	src := indexed(b, benchSource(b))
	for b.Loop() {
		b.StopTimer()
		dir := filepath.Join(workDir(b), "store")
		historytest.Clone(b, src, dir)
		ss := session(b, dir, "alice")
		s, err := ss.OpenStore()
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
		if _, err := ss.Update(s, store.Op{Name: "edit", Task: task}, func(tx *store.Tx) error {
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

func BenchmarkShow(b *testing.B) {
	dir := indexed(b, benchSource(b))
	for _, c := range []struct{ name, id string }{{"old", "ME-1"}, {"new", "ME-4999"}} {
		b.Run(c.name, func(b *testing.B) {
			warm := session(b, dir, "alice")
			if _, err := warm.Show(c.id); err != nil { // makes the task index
				b.Fatal(err)
			}
			for b.Loop() {
				ss := session(b, dir, "alice")
				r, err := ss.Show(c.id)
				if err != nil || len(r.History) == 0 {
					b.Fatalf("show: %d entries, %v", len(r.History), err)
				}
			}
		})
	}
}

// BenchmarkShowAfterOneCommit is show right after a commit came: the index
// is brought to it and saved inside the call, as after every write.
func BenchmarkShowAfterOneCommit(b *testing.B) {
	src := indexed(b, benchSource(b))
	for b.Loop() {
		b.StopTimer()
		dir := filepath.Join(workDir(b), "store")
		historytest.Clone(b, src, dir)
		ss := session(b, dir, "alice")
		s, err := ss.OpenStore()
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
		if _, err := ss.Update(s, store.Op{Name: "edit", Task: task}, func(tx *store.Tx) error {
			t, err := tx.Task(task)
			if err != nil {
				return err
			}
			t.Title += " (changed)"
			return tx.PutTask(t)
		}); err != nil {
			b.Fatal(err)
		}
		_ = s.Repo().Close()
		fresh := session(b, dir, "alice") // a new process: nothing in memory
		b.StartTimer()
		r, err := fresh.Show(task.String())
		b.StopTimer()
		if err != nil || len(r.History) == 0 {
			b.Fatalf("show: %d entries, %v", len(r.History), err)
		}
		_ = os.RemoveAll(filepath.Dir(dir))
		b.StartTimer()
	}
}

func BenchmarkLogTask(b *testing.B) {
	dir := indexed(b, benchSource(b))
	for _, c := range []struct{ name, id string }{{"old", "ME-1"}, {"new", "ME-4999"}} {
		b.Run(c.name, func(b *testing.B) {
			if _, err := session(b, dir, "alice").Log([]string{c.id}, 0); err != nil {
				b.Fatal(err)
			}
			for b.Loop() {
				ss := session(b, dir, "alice")
				r, err := ss.Log([]string{c.id}, 0)
				if err != nil || len(r.Logs) == 0 {
					b.Fatalf("log: %d entries, %v", len(r.Logs), err)
				}
			}
		})
	}
}

// BenchmarkUndo takes back the commit of the writer 100 commits back
// ("mine"), and finds nothing to take back for an account with no commit
// ("nothing"). Each run starts from a fresh copy.
func BenchmarkUndo(b *testing.B) {
	src := indexed(b, benchSource(b))
	for _, c := range []struct {
		name, account string
		want          bool
	}{{"mine", "alice", true}, {"nothing", "dave", false}} {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				dir := filepath.Join(workDir(b), "store")
				historytest.Clone(b, src, dir)
				ss := session(b, dir, c.account)
				b.StartTimer()
				_, err := ss.Undo()
				b.StopTimer()
				if c.want && err != nil || !c.want && err == nil {
					b.Fatalf("undo: %v", err)
				}
				_ = os.RemoveAll(filepath.Dir(dir))
				b.StartTimer()
			}
		})
	}
}

// BenchmarkRestoreAt puts a task back as it was half way through the history.
func BenchmarkRestoreAt(b *testing.B) {
	src := indexed(b, benchSource(b))
	at := historytest.Start.Add(benchCommits / 2 * time.Minute).Format(time.RFC3339)
	for b.Loop() {
		b.StopTimer()
		dir := filepath.Join(workDir(b), "store")
		historytest.Clone(b, src, dir)
		ss := session(b, dir, "alice")
		b.StartTimer()
		_, err := ss.RestoreAt("ME-1", at)
		b.StopTimer()
		if err != nil {
			b.Fatal(err)
		}
		_ = os.RemoveAll(filepath.Dir(dir))
		b.StartTimer()
	}
}

// BenchmarkFixedCost is what go-git costs every command, apart from the
// indexes (spec decision 9): "firstRead" opens the repository and reads the
// HEAD commit, which reads the pack's .idx; "write" is one write of a task
// title from a new session, which builds the tree from the git index.
func BenchmarkFixedCost(b *testing.B) {
	src := indexed(b, benchSource(b))
	b.Run("firstRead", func(b *testing.B) {
		for b.Loop() {
			r, err := gitrepo.Open(src)
			if err != nil {
				b.Fatal(err)
			}
			h, err := r.Head()
			if err != nil {
				b.Fatal(err)
			}
			if _, err := r.Entry(h); err != nil {
				b.Fatal(err)
			}
			b.StopTimer()
			_ = r.Close()
			b.StartTimer()
		}
	})
	b.Run("write", func(b *testing.B) {
		for b.Loop() {
			b.StopTimer()
			dir := filepath.Join(workDir(b), "store")
			historytest.Clone(b, src, dir)
			ss := session(b, dir, "alice")
			b.StartTimer()
			s, err := ss.OpenStore()
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
			if _, err := ss.Update(s, store.Op{Name: "edit", Task: task}, func(tx *store.Tx) error {
				t, err := tx.Task(task)
				if err != nil {
					return err
				}
				t.Title += " (changed)"
				return tx.PutTask(t)
			}); err != nil {
				b.Fatal(err)
			}
			b.StopTimer()
			_ = s.Repo().Close()
			_ = os.RemoveAll(filepath.Dir(dir))
			b.StartTimer()
		}
	})
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
	src := benchSource(b)
	for _, c := range []struct {
		name string
		mibs []int
	}{{"usual", nil}, {"toCap", []int{1, 2, 4, 8, 16, 24}}} {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				dir := filepath.Join(workDir(b), "store")
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
