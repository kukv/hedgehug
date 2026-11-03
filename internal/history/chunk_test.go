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

// TestChunkOnAFileOfTheSameCountsIsNotRead checks that a chunk is not read
// on another file of the same tip that has as many commits and strings but
// numbers them otherwise: one brought up to the tip and written whole has
// the new strings after the paths, one written from nothing has them before.
func TestChunkOnAFileOfTheSameCountsIsNotRead(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	u := model.NewULID()
	for range 30 { // so that a chunk is far from half the file, and is added
		writeTask(t, s, model.NewULID(), "t")
	}
	writeTask(t, s, u, "a")
	if _, err := history.Open(s, history.Options{}); err != nil {
		t.Fatal(err)
	}
	// a comment brings a new string (its op) and a new path; the damaged end
	// makes the next save write the file whole
	writeComment(t, s, u, "b")
	old, err := s.ReadLocal("history")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteLocal("history", append(bytes.Clone(old), 1, 2, 3)); err != nil {
		t.Fatal(err)
	}
	if _, err := history.Open(s, history.Options{}); err != nil {
		t.Fatal(err)
	}
	brought, err := s.ReadLocal("history")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteLocal("history", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := history.Open(s, history.Options{}); err != nil {
		t.Fatal(err)
	}
	whole, err := s.ReadLocal("history")
	if err != nil {
		t.Fatal(err)
	}
	if len(brought) != len(whole) || bytes.Equal(brought, whole) {
		t.Fatalf("want two files of one length that differ: %d and %d bytes", len(brought), len(whole))
	}
	// the chunk the handle of the first file adds for the next commit
	if err := s.WriteLocal("history", brought); err != nil {
		t.Fatal(err)
	}
	// the op of this commit has another number in the other file
	writeComment(t, s, u, "c")
	h, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := s.ReadLocal("history")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, brought) || len(data) == len(brought) {
		t.Fatal("the commit was not added to the file")
	}
	if err := s.WriteLocal("history", append(bytes.Clone(whole), data[len(brought):]...)); err != nil {
		t.Fatal(err)
	}
	got, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	same := func(a, b history.Commit) bool {
		return a.Hash == b.Hash && a.Task == b.Task && a.Author == b.Author && a.Email == b.Email &&
			a.Op == b.Op && a.Workspace == b.Workspace
	}
	if !slices.EqualFunc(got.All(), h.All(), same) || !slices.Equal(hashes(got.Task(u)), hashes(h.Task(u))) {
		t.Fatal("the chunk was read with the numbers of another file")
	}
}

// TestChunkOnAnotherTableIsNotRead checks that a chunk added by a handle
// that read one file is not read on top of another file of the same tip
// whose strings are numbered otherwise (the file was written again from
// nothing in between): its numbers would name other strings.
func TestChunkOnAnotherTableIsNotRead(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	u := model.NewULID()
	for range 30 { // so that a chunk is far from half the file, and is added
		writeTask(t, s, model.NewULID(), "t")
	}
	writeTask(t, s, u, "a")
	if _, err := history.Open(s, history.Options{}); err != nil {
		t.Fatal(err)
	}
	// a new task brings new strings, added after the paths of the file
	writeTask(t, s, model.NewULID(), "b")
	if _, err := history.Open(s, history.Options{}); err != nil {
		t.Fatal(err)
	}
	added, err := s.ReadLocal("history")
	if err != nil {
		t.Fatal(err)
	}
	// the same tip written from nothing: the strings come before the paths
	if err := s.WriteLocal("history", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := history.Open(s, history.Options{}); err != nil {
		t.Fatal(err)
	}
	whole, err := s.ReadLocal("history")
	if err != nil {
		t.Fatal(err)
	}
	// the chunk the first handle adds for the next commit
	if err := s.WriteLocal("history", added); err != nil {
		t.Fatal(err)
	}
	u3 := model.NewULID()
	writeTask(t, s, u3, "c")
	h, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := s.ReadLocal("history")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, added) || len(data) == len(added) {
		t.Fatal("the commit was not added to the file")
	}
	chunk := data[len(added):]
	if err := s.WriteLocal("history", append(bytes.Clone(whole), chunk...)); err != nil {
		t.Fatal(err)
	}
	got, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	same := func(a, b history.Commit) bool {
		return a.Hash == b.Hash && a.Task == b.Task && a.Author == b.Author && a.Email == b.Email &&
			a.Op == b.Op && a.Workspace == b.Workspace
	}
	if !slices.EqualFunc(got.All(), h.All(), same) || !slices.Equal(hashes(got.Task(u3)), hashes(h.Task(u3))) {
		t.Fatal("the chunk was read with the numbers of another file")
	}
}
