package gitstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

func describeTask(t *testing.T, s *Stores, id port.StoreID, text string) error {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "HH-2 の説明を変更"}
	return s.Write(t.Context(), id, op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Describe(apptest.SampleTask, text),
			tx.WriteItems().Append(apptest.SampleTask, change.NewDesc()),
		)
	})
}

func TestDescribeWritesDescriptionMdInTheSameCommit(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	if err := describeTask(t, s, id, "## 手順\n"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "items", string(apptest.SampleTask), "description.md")
	if b, err := os.ReadFile(file); err != nil || string(b) != "## 手順\n" {
		t.Errorf("description.md = %q, %v", b, err)
	}
	if st := worktreeStatus(t, dir); !st.IsClean() {
		t.Errorf("worktree after describing:\n%s", st)
	}
	if n := headCount(t, dir); n != 3 {
		t.Errorf("commits = %d, want 3 (store.yaml, sample, describe)", n)
	}
	// 空にするとファイルを消し、同じコミットに含める。
	if err := describeTask(t, s, id, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("description.md must be removed: %v", err)
	}
	if st := worktreeStatus(t, dir); !st.IsClean() || headCount(t, dir) != 4 {
		t.Errorf("after emptying: status\n%s\ncommits = %d", st, headCount(t, dir))
	}
}

func TestDescribeFailedCommitLeavesTheOldDescription(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	if err := describeTask(t, s, id, "古い\n"); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	s.beforeCommit = func() error { return boom }
	if err := describeTask(t, s, id, "新しい\n"); !errors.Is(err, boom) {
		t.Fatalf("Write error = %v", err)
	}
	file := filepath.Join(dir, "items", string(apptest.SampleTask), "description.md")
	if b, _ := os.ReadFile(file); string(b) != "古い\n" {
		t.Errorf("description.md = %q", b)
	}
	if st := worktreeStatus(t, dir); !st.IsClean() {
		t.Errorf("worktree after a failed commit:\n%s", st)
	}
	// 消す途中で失敗しても、消した説明が戻る。
	if err := describeTask(t, s, id, ""); !errors.Is(err, boom) {
		t.Fatalf("Write error = %v", err)
	}
	if b, _ := os.ReadFile(file); string(b) != "古い\n" {
		t.Errorf("description.md after a failed removal = %q", b)
	}
}
