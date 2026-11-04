package browse_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/history/historytest"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func BenchmarkShow(b *testing.B) {
	dir := historytest.Indexed(b, historytest.BenchSource(b))
	for _, c := range []struct{ name, id string }{{"old", "ME-1"}, {"new", "ME-4999"}} {
		b.Run(c.name, func(b *testing.B) {
			warm := usecasetest.BenchSession(b, dir, "alice")
			if _, err := browse.Show(warm, c.id); err != nil { // makes the task index
				b.Fatal(err)
			}
			for b.Loop() {
				ss := usecasetest.BenchSession(b, dir, "alice")
				r, err := browse.Show(ss, c.id)
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
	src := historytest.Indexed(b, historytest.BenchSource(b))
	for b.Loop() {
		b.StopTimer()
		dir := filepath.Join(historytest.WorkDir(b), "store")
		historytest.Clone(b, src, dir)
		ss := usecasetest.BenchSession(b, dir, "alice")
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
		fresh := usecasetest.BenchSession(b, dir, "alice") // a new process: nothing in memory
		b.StartTimer()
		r, err := browse.Show(fresh, task.String())
		b.StopTimer()
		if err != nil || len(r.History) == 0 {
			b.Fatalf("show: %d entries, %v", len(r.History), err)
		}
		_ = os.RemoveAll(filepath.Dir(dir))
		b.StartTimer()
	}
}

func BenchmarkLogTask(b *testing.B) {
	dir := historytest.Indexed(b, historytest.BenchSource(b))
	for _, c := range []struct{ name, id string }{{"old", "ME-1"}, {"new", "ME-4999"}} {
		b.Run(c.name, func(b *testing.B) {
			if _, err := browse.Log(usecasetest.BenchSession(b, dir, "alice"), []string{c.id}, 0); err != nil {
				b.Fatal(err)
			}
			for b.Loop() {
				ss := usecasetest.BenchSession(b, dir, "alice")
				r, err := browse.Log(ss, []string{c.id}, 0)
				if err != nil || len(r.Logs) == 0 {
					b.Fatalf("log: %d entries, %v", len(r.Logs), err)
				}
			}
		})
	}
}
