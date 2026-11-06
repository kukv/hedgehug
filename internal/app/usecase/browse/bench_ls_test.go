package browse_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/storage/history/historytest"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// BenchmarkLs is hedgehug ls without the start of a process, for profiles.
// BenchmarkLsProcess in cmd/hedgehug is what the targets are judged by.
func BenchmarkLs(b *testing.B) {
	src := historytest.Indexed(b, historytest.TaskBenchSource(b))
	for _, c := range usecasetest.LsCases() {
		b.Run(c.Name, func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				dir := filepath.Join(historytest.WorkDir(b), "store")
				historytest.Clone(b, src, dir)
				c.Prepare(b, dir)
				ss := usecasetest.BenchSession(b, dir, "alice") // a new process: nothing in memory
				b.StartTimer()
				n := 0
				if c.Tree {
					if _, err := browse.Tree(ss, ""); err != nil {
						b.Fatal(err)
					}
				} else {
					r, err := browse.List(ss, "")
					if err != nil {
						b.Fatal(err)
					}
					n = len(r.Tasks)
				}
				b.StopTimer()
				if !c.Tree && n != historytest.TaskBenchTasks {
					b.Fatalf("%d tasks listed", n)
				}
				_ = os.RemoveAll(filepath.Dir(dir))
				b.StartTimer()
			}
		})
	}
}
