package gitstore

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

var _ port.StoreVersions = (*Stores)(nil)

func TestVersionsChangeWithEverySave(t *testing.T) {
	m := localMachine(t)
	v1, err := m.stores.Versions(t.Context())
	if err != nil || v1[m.id] == "" {
		t.Fatalf("Versions = %v, %v", v1, err)
	}
	rename(t, m, "新しい名前")
	v2, err := m.stores.Versions(t.Context())
	if err != nil || v2[m.id] == v1[m.id] || v2[m.id] != headCommit(t, m.dir).Hash.String() {
		t.Errorf("after a save: %v -> %v, %v", v1, v2, err)
	}
}

// Commits はロックを HEAD を読むあいだしか持たない。辿っているあいだに Write が終わる。
func TestCommitsDoesNotHoldLock(t *testing.T) {
	m := localMachine(t)
	rename(t, m, "一つ目")
	wrote := make(chan error, 1)
	var pending bool
	// 終わらなかったときも、Write の goroutine を待ってからテストを終える。
	t.Cleanup(func() {
		if pending {
			<-wrote
		}
	})
	m.stores.duringCommits = func() {
		pending = true
		go func() {
			wrote <- m.stores.Write(t.Context(), m.id, port.Operation{Actor: change.User("taro"), Summary: "辿っている間の保存"}, set("title", "一つ目", "二つ目"))
		}()
		select {
		case err := <-wrote:
			pending = false
			if err != nil {
				t.Errorf("Write during Commits: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("Write did not finish while Commits was walking (the lock is held)")
		}
	}
	got, err := m.stores.Commits(t.Context(), m.id)
	if err != nil {
		t.Fatal(err)
	}
	m.stores.duringCommits = nil
	// 辿り始めたときの HEAD までを返す。辿っているあいだの保存は入らない。
	after, err := m.stores.Commits(t.Context(), m.id)
	if err != nil || len(after) != len(got)+1 {
		t.Errorf("commits: during = %d, after = %d, %v", len(got), len(after), err)
	}
}

// 読んだコミットは SHA で覚え、2 回目は同じ値を返す。
func TestCommitsRemembersBySHA(t *testing.T) {
	m := localMachine(t)
	rename(t, m, "一つ目")
	first, err := m.stores.Commits(t.Context(), m.id)
	if err != nil || len(first) == 0 {
		t.Fatalf("Commits = %v, %v", first, err)
	}
	m.stores.mu.Lock()
	n := len(m.stores.commits)
	m.stores.mu.Unlock()
	if n != len(first) {
		t.Errorf("remembered %d commits, want %d", n, len(first))
	}
	second, err := m.stores.Commits(t.Context(), m.id)
	if err != nil || len(second) != len(first) || second[0].SHA != first[0].SHA {
		t.Errorf("second = %v, %v", second, err)
	}
}

func TestCommitsUnknownStore(t *testing.T) {
	m := localMachine(t)
	if _, err := m.stores.Commits(t.Context(), "01NOSUCH000000000000000001"); !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("Commits = %v, want ErrStoreNotFound", err)
	}
}
