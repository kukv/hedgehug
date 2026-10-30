package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/store"
)

func TestStoreInit(t *testing.T) {
	te := newTestEnv(t)
	out := te.ok("store", "init", "me")
	if !strings.Contains(out, "me") || !strings.Contains(out, te.storeDir("me")) {
		t.Errorf("stdout = %q, want the name and the path", out)
	}
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Meta().Name != "me" || s.Meta().ID.IsZero() {
		t.Errorf("meta = %+v", s.Meta())
	}
	if _, ok := s.Schema().Status("review"); ok {
		t.Error("the default store has the review status")
	}
	log := te.log("me")
	if len(log) != 1 || log[0].Author.Name != "Alice" || log[0].Author.Email != "alice@example.com" {
		t.Errorf("log = %v, want one commit by Alice", log)
	}
	b, err := os.ReadFile(filepath.Join(te.hhHome, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "name: me") {
		t.Errorf("config.yaml = %q", b)
	}
	// The only store is picked without --store.
	te.ok("ls")
}

func TestStoreInitDev(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "dev", "--template", "dev")
	s, err := store.Open(te.storeDir("dev"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Schema().Status("review"); !ok {
		t.Error("a dev store has no review status")
	}
}

func TestStoreInitJSON(t *testing.T) {
	te := newTestEnv(t)
	var got struct{ Name, Path string }
	if err := json.Unmarshal([]byte(te.ok("store", "init", "me", "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "me" || got.Path != te.storeDir("me") {
		t.Errorf("json = %+v", got)
	}
}

func TestStoreInitErrors(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me")
	for _, tc := range []struct {
		name     string
		args     []string
		wantCode int
		want     string
	}{
		{"a name twice", []string{"store", "init", "me"}, 1, `"me"`},
		{"a bad name", []string{"store", "init", "Bad"}, 2, "cannot name a store"},
		{"a name with a space", []string{"store", "init", "Bad Name"}, 2, "cannot name a store"},
		{"an unknown template", []string{"store", "init", "x", "--template", "nope"}, 2, "--template"},
		{"no name", []string{"store", "init"}, 2, "accepts 1 arg"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := te.run(tc.args...)
			if code != tc.wantCode {
				t.Errorf("exit = %d, want %d (stderr %q)", code, tc.wantCode, stderr)
			}
			if !strings.HasPrefix(stderr, "hedgehug: ") || !strings.Contains(stderr, tc.want) {
				t.Errorf("stderr = %q, want hedgehug: ... %q", stderr, tc.want)
			}
		})
	}
	if _, err := os.Stat(te.storeDir("x")); err == nil {
		t.Error("a store was made for an unknown template")
	}
}

func TestStoreSelection(t *testing.T) {
	te := newTestEnv(t)
	code, _, stderr := te.run("ls")
	if code != 1 || !strings.Contains(stderr, "store init") {
		t.Errorf("ls with no store: exit %d, stderr %q", code, stderr)
	}
	te.ok("store", "init", "a")
	te.ok("store", "init", "b")
	code, _, stderr = te.run("ls")
	if code != 1 || !strings.Contains(stderr, "--store") {
		t.Errorf("ls with two stores: exit %d, stderr %q", code, stderr)
	}
	te.ok("ls", "--store", "b")
}

func TestStoreInitNoAuthor(t *testing.T) {
	te := newTestEnv(t)
	if err := os.Remove(filepath.Join(te.home, ".gitconfig")); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := te.run("store", "init", "me")
	if code != 1 || !strings.Contains(stderr, "user.name") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if _, err := os.Stat(te.storeDir("me")); err == nil {
		t.Error("a store was made with no author")
	}
}
