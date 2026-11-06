package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/storage/history/historytest"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// BenchmarkLsProcess runs hedgehug ls as a command, the start of the process
// included. The targets of phase 7a are judged by it.
func BenchmarkLsProcess(b *testing.B) {
	src := historytest.Indexed(b, historytest.TaskBenchSource(b))
	bin := filepath.Join(b.TempDir(), "hedgehug")
	if out, err := exec.CommandContext(context.Background(), "go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		b.Fatalf("go build: %v\n%s", err, out)
	}
	for _, c := range usecasetest.LsCases() {
		b.Run(c.Name, func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				dir := filepath.Join(historytest.WorkDir(b), "store")
				historytest.Clone(b, src, dir)
				c.Prepare(b, dir)
				cmd := exec.CommandContext(context.Background(), bin, c.Args...) //nolint:gosec // the binary built above
				for k, v := range usecasetest.BenchEnv(b, dir, "alice") {
					cmd.Env = append(cmd.Env, k+"="+v)
				}
				b.StartTimer()
				err := cmd.Run() // no stdout: as much formatting, and no pipe to read
				b.StopTimer()
				if err != nil {
					b.Fatal(err)
				}
				_ = os.RemoveAll(filepath.Dir(dir))
				b.StartTimer()
			}
		})
	}
}
