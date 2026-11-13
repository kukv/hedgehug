package statedb

import (
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func TestChecksContract(t *testing.T) {
	apptest.RunChecksContract(t, func(t *testing.T) port.Checks { return openTemp(t).Checks() })
}

func TestChecksSurviveReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Checks().Save(t.Context(), "S1", "pr:org/web#1", itemlink.Success); err != nil {
		t.Fatal(err)
	}
	_ = d.Close()
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	if got, _ := d.Checks().Read(t.Context(), "S1"); got["pr:org/web#1"] != itemlink.Success {
		t.Errorf("after reopen: %v", got)
	}
}

// 読んで比べてから書くまでを 1 つのトランザクションで行うので、同時に同じ値を置いても「変わった」は 1 回だけ。
func TestChecksSaveIsAtomic(t *testing.T) {
	c := openTemp(t).Checks()
	var changed atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			ok, err := c.Save(t.Context(), "S1", "pr:org/web#1", itemlink.Success)
			if err != nil {
				t.Error(err)
			}
			if ok {
				changed.Add(1)
			}
		})
	}
	wg.Wait()
	if changed.Load() != 1 {
		t.Errorf("changed %d times, want 1", changed.Load())
	}
}
