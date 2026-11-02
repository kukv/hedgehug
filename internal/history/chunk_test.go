package history_test

import (
	"bytes"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/history"
	"github.com/kukv/hedgehug/internal/model"
)

// TestCommitsAreAddedToTheFile checks that bringing a saved index to a newer
// HEAD adds to the file instead of writing it again, that what is read back
// is what was built, and that a file damaged anywhere in what was added is
// read as far as it is sound (the rest is brought up from git).
func TestCommitsAreAddedToTheFile(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	u := model.NewULID()
	for range 30 {
		writeTask(t, s, model.NewULID(), "t")
	}
	writeTask(t, s, u, "a")
	if _, err := history.Open(s, history.Options{}); err != nil {
		t.Fatal(err)
	}
	base, err := s.ReadLocal("history")
	if err != nil {
		t.Fatal(err)
	}
	writeTask(t, s, u, "b")
	writeComment(t, s, u, "again")
	if _, err := history.Open(s, history.Options{}); err != nil {
		t.Fatal(err)
	}
	writeTask(t, s, u, "c")
	h, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := s.ReadLocal("history")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, base) || len(data) <= len(base) {
		t.Fatalf("the file was not added to: %d bytes after %d", len(data), len(base))
	}
	want := hashes(h.Task(u))
	if len(want) != 4 {
		t.Fatalf("the task has %d commits, want 4", len(want))
	}

	check := func(label string, file []byte) {
		t.Helper()
		if err := s.WriteLocal("history", file); err != nil {
			t.Fatal(err)
		}
		got, err := history.Open(s, history.Options{})
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		if !slices.Equal(hashes(got.All()), hashes(h.All())) || !slices.Equal(hashes(got.Task(u)), want) || got.Tip() != h.Tip() {
			t.Fatalf("%s: the index differs from the one built", label)
		}
	}
	check("whole", data)
	for n := len(base); n < len(data); n++ {
		check("cut", data[:n])
	}
	for i := len(base); i < len(data); i++ {
		bad := bytes.Clone(data)
		bad[i] ^= 0x55
		check("flipped", bad)
	}

	// after a damaged end, the next save writes the file again
	if err := s.WriteLocal("history", append(bytes.Clone(data), 1, 2, 3)); err != nil {
		t.Fatal(err)
	}
	writeTask(t, s, u, "d")
	if _, err := history.Open(s, history.Options{}); err != nil {
		t.Fatal(err)
	}
	fixed, err := s.ReadLocal("history")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(fixed, append(bytes.Clone(data[len(data)-4:]), 1, 2, 3)) {
		t.Fatal("the damaged end was kept")
	}
}

// TestFileIsWrittenAgainAfterManyChunks checks that the file does not grow
// by a chunk for every commit for ever.
func TestFileIsWrittenAgainAfterManyChunks(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	u := model.NewULID()
	const n = 150
	for range n {
		writeTask(t, s, u, "x")
		if _, err := history.Open(s, history.Options{}); err != nil {
			t.Fatal(err)
		}
	}
	h, err := history.Open(s, history.Options{})
	if err != nil || len(h.Task(u)) != n {
		t.Fatalf("%v, %d commits, want %d", err, len(h.Task(u)), n)
	}
	data, err := s.ReadLocal("history")
	if err != nil {
		t.Fatal(err)
	}
	// every commit as a chunk would be 150 times the 100 or so bytes of a
	// chunk plus the first part
	if len(data) > 40*1024 {
		t.Fatalf("the file has %d bytes", len(data))
	}
}
