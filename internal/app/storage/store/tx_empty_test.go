package store

import (
	"os"
	"strings"
	"testing"
)

func TestRemoveFile(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	put := func(tx *Tx) error { return tx.PutFile("notes.txt", []byte("x\n")) }
	if _, err := s.Update(Op{Name: "setup", Author: testAuthor}, put); err != nil {
		t.Fatal(err)
	}
	committed, err := s.Update(Op{Name: "setup", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveFile("notes.txt") })
	if err != nil || !committed {
		t.Fatalf("committed %v, %v", committed, err)
	}
	if _, err := os.Stat(s.path("notes.txt")); !os.IsNotExist(err) {
		t.Errorf("notes.txt is still there: %v", err)
	}
	// A file that is not there is left alone, and nothing is committed.
	if committed, err := s.Update(Op{Name: "setup", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveFile("notes.txt") }); err != nil || committed {
		t.Errorf("second removal: committed %v, %v", committed, err)
	}
	for _, bad := range []string{".hedgehug/local/lock", ".git/config", "../x"} {
		if _, err := s.Update(Op{Name: "x", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveFile(bad) }); err == nil {
			t.Errorf("RemoveFile(%q) succeeded", bad)
		}
	}
}

func TestAllowEmptyAndTrailers(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	if committed, err := s.Update(Op{Name: "undo", Author: testAuthor}, func(*Tx) error { return nil }); err != nil || committed {
		t.Fatalf("no change: committed %v, %v", committed, err)
	}
	committed, err := s.Update(Op{Name: "undo", Author: testAuthor, Account: "github.com/alice", Detail: "set X y"}, func(tx *Tx) error {
		tx.AddTrailer("Hedgehug-Undo", "abc")
		tx.AllowEmpty()
		return nil
	})
	if err != nil || !committed {
		t.Fatalf("committed %v, %v", committed, err)
	}
	c := headLog(t, s.dir)[0]
	want := "hedgehug: undo set X y\n\nHedgehug-Op: undo\nHedgehug-Author: github.com/alice\nHedgehug-Undo: abc\n"
	if c.Message != want {
		t.Errorf("message = %q, want %q", c.Message, want)
	}
	if strings.Contains(c.Message, "Hedgehug-Task") {
		t.Errorf("message = %q", c.Message)
	}
}
