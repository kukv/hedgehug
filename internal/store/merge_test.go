package store

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
)

func TestCommitMerge(t *testing.T) {
	s := newTestStore(t)
	head, _ := s.repo.Head()
	files := map[string][]byte{"notes/a.md": []byte("a\n")}
	h, err := s.CommitMerge(head, []plumbing.Hash{head}, files, Op{Name: "repair", Detail: "1 issues", Author: testAuthor, Account: "github.com/a"}, []gitrepo.Trailer{{Key: "Hedgehug-Repaired", Value: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	c := headLog(t, s.dir)[0]
	if c.Hash != h || !strings.HasPrefix(c.Message, "hedgehug: repair 1 issues\n") || !strings.Contains(c.Message, "Hedgehug-Repaired: x") {
		t.Errorf("commit = %s %q", c.Hash, c.Message)
	}
	if !strings.Contains(c.Message, "Hedgehug-Op: repair\nHedgehug-Author: github.com/a\nHedgehug-Repaired: x") {
		t.Errorf("trailer order: %q", c.Message)
	}
	if _, err := s.CommitMerge(head, []plumbing.Hash{head}, files, Op{Name: "repair", Author: testAuthor}, nil); !errors.Is(err, ErrHeadMoved) {
		t.Errorf("stale head = %v", err)
	}
}

func TestCommitMergeSkipsValidation(t *testing.T) {
	s := newTestStore(t)
	head, _ := s.repo.Head()
	x := model.NewULID()
	bad := []byte("---\nulid: " + x.String() + "\nid: TST-1\nworkspace: " + model.NewULID().String() + "\nkind: task\ntitle: x\nstatus: todo\n---\n")
	if _, err := s.CommitMerge(head, []plumbing.Hash{head}, map[string][]byte{model.TaskPath(x): bad}, Op{Name: "merge", Author: testAuthor}, nil); err != nil {
		t.Errorf("a merge with a broken invariant was refused: %v", err)
	}
}

func TestCommitMergeRollsBack(t *testing.T) {
	s := newTestStore(t)
	mustWrite(t, s, "notes/old.md", "old\n")
	head, _ := s.repo.Head()
	files := map[string][]byte{"notes/old.md": nil, "notes/new.md": []byte("n\n")}
	s.commit = func(gitrepo.Commit) error { return errors.New("boom") }
	if _, err := s.CommitMerge(head, []plumbing.Hash{head}, files, Op{Name: "merge", Author: testAuthor}, nil); err == nil {
		t.Fatal("a failed commit was not reported")
	}
	if b, err := os.ReadFile(s.path("notes/old.md")); err != nil || string(b) != "old\n" {
		t.Errorf("old.md = %q, %v", b, err)
	}
	if _, err := os.Stat(s.path("notes/new.md")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("new.md stayed: %v", err)
	}
}

func TestFastForwardChecksHead(t *testing.T) {
	s := newTestStore(t)
	head, _ := s.repo.Head()
	other := plumbing.NewHash("0123456789012345678901234567890123456789")
	if err := s.FastForward(other, head, nil); !errors.Is(err, ErrHeadMoved) {
		t.Errorf("stale head = %v", err)
	}
	if err := s.FastForward(head, other, nil); err == nil {
		t.Error("a missing commit was accepted")
	}
}
