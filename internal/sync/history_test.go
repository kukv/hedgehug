package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
)

func TestSidesYAMLWhen(t *testing.T) {
	dir := t.TempDir()
	r, err := gitrepo.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	at := func(m int) time.Time { return time.Date(2026, 10, 31, 12, m, 0, 0, time.UTC) }
	commit := func(data string, when time.Time, parents ...plumbing.Hash) plumbing.Hash {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, model.AutomationsPath), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		h, err := r.Commit(gitrepo.Commit{
			Paths: []string{model.AutomationsPath}, Parents: parents, Subject: "x",
			Author: gitrepo.Signature{Name: "a", Email: "a@x"}, When: when,
		})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	base := commit("a: 1\nb: 1\n", at(0))
	commit("a: 2\nb: 1\n", at(1))
	o2 := commit("# a note\na: 2\nb: 1\n", at(2)) // a comment is not a value
	theirs := commit("a: 1\nb: 2\n", at(3), base)
	// The merge takes b from theirs: b changed when theirs changed it.
	ours := commit("# a note\na: 2\nb: 2\n", at(4), o2, theirs)

	b, o, th, err := sides(r, base, ours, theirs, model.AutomationsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "a: 1\nb: 1\n" || string(th.Data) != "a: 1\nb: 2\n" {
		t.Errorf("data = %q %q", b, th.Data)
	}
	for _, c := range []struct {
		name string
		got  time.Time
		want time.Time
	}{
		{"ours a", o.When([]string{"a"}), at(1)},
		{"ours b", o.When([]string{"b"}), at(3)},
		{"theirs a", th.When([]string{"a"}), time.Time{}},
		{"theirs b", th.When([]string{"b"}), at(3)},
		{"ours file", o.Time, at(4)},
		{"theirs file", th.Time, at(3)},
	} {
		if !c.got.Equal(c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if o.Author != "a@x" {
		t.Errorf("author = %q", o.Author)
	}
}

func TestSidesOtherFile(t *testing.T) {
	c := newCluster(t, 1)
	r := c.stores[0].Repo()
	base := mustHead(t, c.stores[0])
	x := c.addTask(0, "x")
	head := mustHead(t, c.stores[0])
	b, o, th, err := sides(r, base, head, base, model.TaskPath(x.ULID))
	if err != nil {
		t.Fatal(err)
	}
	if b != nil || o.Data == nil || th.Data != nil || o.When != nil {
		t.Errorf("sides = %q %+v %+v", b, o, th)
	}
	if o.Author != "github.com/u0" || o.Time.IsZero() {
		t.Errorf("ours = %q %v", o.Author, o.Time)
	}
}
