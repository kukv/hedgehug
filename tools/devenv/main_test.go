package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/store"
)

func runDevenv(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = run(args, &out, &errb, t.TempDir())
	return code, out.String(), errb.String()
}

func initDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "devenv")
	if code, _, stderr := runDevenv(t, "init", "--dir", dir); code != 0 {
		t.Fatalf("init = %d, stderr %q", code, stderr)
	}
	return dir
}

func TestInitCreatesUsers(t *testing.T) {
	dir := initDir(t)
	if _, err := os.Stat(filepath.Join(dir, markerFile)); err != nil {
		t.Errorf("marker: %v", err)
	}
	for _, u := range []string{"alice", "bob"} {
		home := filepath.Join(dir, "users", u)
		gc, err := os.ReadFile(filepath.Join(home, ".gitconfig"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(gc), "name = "+u) || !strings.Contains(string(gc), "email = "+u+"@example.com") {
			t.Errorf("%s .gitconfig = %q", u, gc)
		}
		cfg, err := os.ReadFile(filepath.Join(home, "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(cfg), "me") {
			t.Errorf("%s config.yaml = %q", u, cfg)
		}
		s, err := store.Open(filepath.Join(home, "stores", "me"))
		if err != nil {
			t.Fatalf("%s: Open: %v", u, err)
		}
		log, err := s.Repo().Log(nil, 0)
		if err != nil || len(log) == 0 {
			t.Fatalf("%s: log = %v, %v", u, log, err)
		}
		if got := log[len(log)-1].Author.Name; got != u {
			t.Errorf("%s: first commit author = %q, want %q", u, got, u)
		}
	}
}

func TestInitExistingDirFails(t *testing.T) {
	dir := initDir(t)
	code, _, stderr := runDevenv(t, "init", "--dir", dir)
	if code != 1 || !strings.Contains(stderr, "reset") {
		t.Errorf("init again = %d, stderr %q; want 1 mentioning reset", code, stderr)
	}
}

func TestResetRecreates(t *testing.T) {
	dir := initDir(t)
	extra := filepath.Join(dir, "users", "alice", "extra")
	if err := os.WriteFile(extra, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runDevenv(t, "reset", "--dir", dir); code != 0 {
		t.Fatalf("reset = %d, stderr %q", code, stderr)
	}
	if _, err := os.Stat(extra); !os.IsNotExist(err) {
		t.Errorf("extra file survived reset: %v", err)
	}
	if _, err := store.Open(filepath.Join(dir, "users", "bob", "stores", "me")); err != nil {
		t.Errorf("Open after reset: %v", err)
	}
}

func TestResetWithoutMarkerKeepsDir(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "keep")
	if err := os.WriteFile(keep, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runDevenv(t, "reset", "--dir", dir)
	if code != 1 || stderr == "" {
		t.Errorf("reset = %d, stderr %q; want 1", code, stderr)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("file removed: %v", err)
	}
}

func TestResetMissingDirInits(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new")
	if code, _, stderr := runDevenv(t, "reset", "--dir", dir); code != 0 {
		t.Fatalf("reset = %d, stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, markerFile)); err != nil {
		t.Errorf("marker: %v", err)
	}
}

func TestEnv(t *testing.T) {
	dir := initDir(t)
	code, stdout, stderr := runDevenv(t, "env", "alice", "--dir", dir)
	if code != 0 {
		t.Fatalf("env = %d, stderr %q", code, stderr)
	}
	home := filepath.Join(dir, "users", "alice")
	want := "export HEDGEHUG_HOME='" + home + "'\nexport GIT_CONFIG_GLOBAL='" + filepath.Join(home, ".gitconfig") + "'\n"
	if stdout != want {
		t.Errorf("env = %q, want %q", stdout, want)
	}
}

func TestEnvQuotesAndMakesAbsolute(t *testing.T) {
	cwd := t.TempDir()
	var out, errb bytes.Buffer
	if code := run([]string{"init", "--dir", "it's here"}, &out, &errb, cwd); code != 0 {
		t.Fatalf("init = %d, stderr %q", code, errb.String())
	}
	out.Reset()
	if code := run([]string{"env", "bob", "--dir", "it's here"}, &out, &errb, cwd); code != 0 {
		t.Fatalf("env = %d, stderr %q", code, errb.String())
	}
	if !strings.Contains(out.String(), `it'\''s here`) {
		t.Errorf("single quote not escaped: %q", out.String())
	}
	if !strings.Contains(out.String(), cwd) {
		t.Errorf("path not relative to cwd: %q", out.String())
	}
}

func TestEnvUnknownUser(t *testing.T) {
	dir := initDir(t)
	code, stdout, stderr := runDevenv(t, "env", "carol", "--dir", dir)
	if code != 1 || stdout != "" || stderr == "" {
		t.Errorf("env carol = %d, %q, %q; want 1 with only stderr", code, stdout, stderr)
	}
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"nope"}, {"env"}, {"init", "extra"}} {
		if code, _, _ := runDevenv(t, args...); code != 2 {
			t.Errorf("run(%v) = %d, want 2", args, code)
		}
	}
}

func TestFindRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	got, err := findRoot(sub)
	if err != nil || got != root {
		t.Errorf("findRoot = %q, %v; want %q", got, err, root)
	}
}
