package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// testEnv runs the command with HEDGEHUG_HOME and HOME in temporary
// directories, and an author in HOME/.gitconfig.
type testEnv struct {
	t        *testing.T
	home     string
	hhHome   string
	stdin    string
	terminal bool // what isTerminal reports for every stream
	now      time.Time
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	d := t.TempDir()
	te := &testEnv{
		t:      t,
		home:   filepath.Join(d, "home"),
		hhHome: filepath.Join(d, "hh"),
		now:    time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC),
	}
	if err := os.MkdirAll(te.home, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(te.home, ".gitconfig"), []byte("[user]\n\tname = Alice\n\temail = alice@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return te
}

func (te *testEnv) getenv(k string) string {
	switch k {
	case "HOME", "USERPROFILE":
		return te.home
	case "HEDGEHUG_HOME":
		return te.hhHome
	}
	return ""
}

func (te *testEnv) run(args ...string) (code int, stdout, stderr string) {
	te.t.Helper()
	var out, errb bytes.Buffer
	e := &env{
		stdin:      strings.NewReader(te.stdin),
		stdout:     &out,
		stderr:     &errb,
		getenv:     te.getenv,
		isTerminal: func(any) bool { return te.terminal },
		now:        func() time.Time { return te.now },
	}
	code = run(args, e)
	return code, out.String(), errb.String()
}

// ok runs the command, fails the test unless it exits 0, and returns stdout.
func (te *testEnv) ok(args ...string) string {
	te.t.Helper()
	code, out, errs := te.run(args...)
	if code != 0 {
		te.t.Fatalf("hedgehug %s: exit %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), code, out, errs)
	}
	return out
}

func (te *testEnv) storeDir(name string) string { return filepath.Join(te.hhHome, "stores", name) }

// log returns the commits of the store, newest first.
func (te *testEnv) log(name string) []*object.Commit {
	te.t.Helper()
	r, err := git.PlainOpen(te.storeDir(name))
	if err != nil {
		te.t.Fatal(err)
	}
	ref, err := r.Head()
	if err != nil {
		te.t.Fatal(err)
	}
	it, err := r.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		te.t.Fatal(err)
	}
	var out []*object.Commit
	if err := it.ForEach(func(c *object.Commit) error {
		out = append(out, c)
		return nil
	}); err != nil {
		te.t.Fatal(err)
	}
	return out
}
