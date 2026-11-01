package store

import (
	"errors"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
)

// A CommitMerge that waits for the file lock held by a Locked of this
// process does not keep the holder out of the store: the holder's own
// Update, which needs s.mu, goes through at once.
func TestCommitMergeWaitsWithoutHoldingTheMutex(t *testing.T) {
	// 直列: パッケージ変数 onLockWait の差し替え
	s := newTestStore(t)
	s.lockWait = 3 * time.Second
	head, err := s.repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	l, err := LockInOrder(s)
	if err != nil {
		t.Fatal(err)
	}
	waiting := make(chan struct{}, 1)
	onLockWait = func() {
		select {
		case waiting <- struct{}{}:
		default:
		}
	}
	t.Cleanup(func() { onLockWait = func() {} })
	merged := make(chan error, 1)
	go func() {
		_, err := s.CommitMerge(head, []plumbing.Hash{head}, map[string][]byte{"m.txt": []byte("m")}, Op{Name: "repair", Author: testAuthor}, nil)
		merged <- err
	}()
	<-waiting // CommitMerge is waiting for the file lock, without s.mu

	done := make(chan error, 1)
	go func() {
		_, err := s.Update(Op{Name: "x", Author: testAuthor, Locked: l}, func(tx *Tx) error { return tx.PutFile("x.txt", []byte("x")) })
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("the holder's Update: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("the holder's Update waited for the CommitMerge")
	}

	l.Release()
	select {
	case err := <-merged:
		if !errors.Is(err, ErrHeadMoved) { // the holder's commit moved HEAD
			t.Errorf("CommitMerge = %v, want ErrHeadMoved", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the CommitMerge did not go on after Release")
	}
}
