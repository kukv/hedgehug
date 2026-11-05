package usecase_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/history/historytest"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// BenchmarkFixedCost is what go-git costs every command, apart from the
// indexes (spec decision 9): "firstRead" opens the repository and reads the
// HEAD commit, which reads the pack's .idx; "write" is one write of a task
// title from a new session, opened as a command opens it to write.
func BenchmarkFixedCost(b *testing.B) {
	src := historytest.Indexed(b, historytest.BenchSource(b))
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
			dir := filepath.Join(historytest.WorkDir(b), "store")
			historytest.Clone(b, src, dir)
			ss := usecasetest.BenchSession(b, dir, "alice")
			b.StartTimer()
			s, _, err := ss.OpenForWrite() // with the task index, as a command writes
			if err != nil {
				b.Fatal(err)
			}
			var task model.ULID
			for t, err := range s.Tasks() {
				if err != nil {
					b.Fatal(err)
				}
				if t.Parent != nil { // under a project, as the tasks undo takes back
					task = t.ULID
					break
				}
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
