package tasks_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/storage/history/historytest"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// BenchmarkUndo takes back the commit of the writer 100 commits back
// ("mine"), and finds nothing to take back for an account with no commit
// ("nothing"). Each run starts from a fresh copy.
func BenchmarkUndo(b *testing.B) {
	src := historytest.Indexed(b, historytest.BenchSource(b))
	for _, c := range []struct {
		name, account string
		want          bool
	}{{"mine", "alice", true}, {"nothing", "dave", false}} {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				dir := filepath.Join(historytest.WorkDir(b), "store")
				historytest.Clone(b, src, dir)
				ss := usecasetest.BenchSession(b, dir, c.account)
				b.StartTimer()
				_, err := tasks.Undo(ss)
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
	src := historytest.Indexed(b, historytest.BenchSource(b))
	at := historytest.Start.Add(historytest.BenchCommits / 2 * time.Minute).Format(time.RFC3339)
	for b.Loop() {
		b.StopTimer()
		dir := filepath.Join(historytest.WorkDir(b), "store")
		historytest.Clone(b, src, dir)
		ss := usecasetest.BenchSession(b, dir, "alice")
		b.StartTimer()
		_, err := tasks.RestoreAt(ss, "ME-1", at)
		b.StopTimer()
		if err != nil {
			b.Fatal(err)
		}
		_ = os.RemoveAll(filepath.Dir(dir))
		b.StartTimer()
	}
}
