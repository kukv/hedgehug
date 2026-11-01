package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

func TestReadOnlyRefusesWrites(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "one")
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sync.WriteAccess(s, sync.Access{ReadOnly: true, Reason: sync.ReasonGitHub, Checked: te.now}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "two"},
		{"set", "ME-1", "title=x"},
		{"set", "ME-99", "title=x"},
		{"done", "ME-1"},
		{"rm", "ME-1"},
		{"comment", "ME-1", "hi"},
		{"workspace", "add", "w", "--prefix", "WW"},
		{"purge", "ME-1", "--yes"},
		{"doctor", "--fix"},
		{"undo"},
		{"mv", "ME-1", "--root"},
	} {
		code, _, errs := te.run(args...)
		if code != 1 || !strings.Contains(errs, "me is read-only (the GitHub permission is read)") {
			t.Errorf("%v: exit %d, %q", args, code, errs)
		}
	}
	if out := te.ok("ls"); !strings.Contains(out, "one") {
		t.Errorf("reading still works: %q", out)
	}
}

func TestStoreAddAccountChecksAccess(t *testing.T) {
	te := newTestEnv(t)
	d := t.TempDir()
	// alice makes the store in a bare, which bob reaches as github.com.
	alice := newTestEnv(t)
	alice.ok("store", "init", "org", "--prefix", "ORG", "--remote", filepath.Join(d, "org-a", "x.git"))
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), nil, nil)
	// bob may only read it on GitHub.
	te.withFake(fakegh.State{
		Tokens: map[string]string{"t-bob": "bob"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "x", Perms: map[string]string{"bob": "read"}}},
	})
	te.env["HH_TOKEN"] = "t-bob"
	te.writeConfigYAML("accounts:\n  - {name: me, host: github.com, login: bob, token: \"env:HH_TOKEN\"}\n")
	te.ok("store", "add", "org", "--remote", "https://github.com/org-a/x.git", "--account", "me")
	code, _, errs := te.run("add", "x", "--store", "org")
	if code != 1 || !strings.Contains(errs, "org is read-only (the GitHub permission is read)") {
		t.Errorf("exit %d, %q", code, errs)
	}
	out := te.ok("sync", "--status")
	if !strings.Contains(out, "read-only:") || !strings.Contains(out, "the GitHub permission is read") {
		t.Errorf("status = %q", out)
	}
	var rows []struct {
		Store    string `json:"store"`
		ReadOnly string `json:"read_only"`
	}
	if err := json.Unmarshal([]byte(te.ok("sync", "--status", "--json")), &rows); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Store == "org" && r.ReadOnly != "the GitHub permission is read" {
			t.Errorf("--json read_only = %q", r.ReadOnly)
		}
	}
	// An account that is not there stops the command, and clones nothing.
	code, _, errs = te.run("store", "add", "again", "--remote", "https://github.com/org-a/x.git", "--account", "nope")
	if code != 1 || !strings.Contains(errs, `no account is named "nope"`) {
		t.Errorf("unknown account: exit %d, %q", code, errs)
	}
}

func TestSyncShowsUnpushable(t *testing.T) {
	te := newTestEnv(t)
	bare := filepath.Join(t.TempDir(), "x.git")
	if err := gitrepo.InitBare(bare); err != nil {
		t.Fatal(err)
	}
	te.ok("store", "init", "me", "--prefix", "ME", "--remote", bare)
	te.ok("add", "one")
	gitrepotest.LockObjects(t, bare) // skips the test where there are no read-only directories
	code, _, errs := te.run("sync")
	if code != 1 || !strings.Contains(errs, "read-only (the remote denied the push)") {
		t.Errorf("sync: exit %d, %q", code, errs)
	}
	te.now = te.now.Add(time.Minute)
	out := te.ok("sync", "--status")
	if !strings.Contains(out, "read-only:") || !strings.Contains(out, "1 commit not pushed") {
		t.Errorf("status = %q", out)
	}
}

func TestSyncPrintsUnpushable(t *testing.T) {
	te := newTestEnv(t)
	bare := filepath.Join(t.TempDir(), "x.git")
	if err := gitrepo.InitBare(bare); err != nil {
		t.Fatal(err)
	}
	te.ok("store", "init", "me", "--prefix", "ME", "--remote", bare)
	te.ok("add", "one")
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sync.WriteAccess(s, sync.Access{ReadOnly: true, Reason: sync.ReasonGitHub, Checked: te.now}); err != nil {
		t.Fatal(err)
	}
	out := te.ok("sync")
	if !strings.Contains(out, "read-only: 1 commit not pushed") {
		t.Errorf("sync = %q", out)
	}
	var reps []struct {
		Unpushable int `json:"unpushable"`
	}
	if err := json.Unmarshal([]byte(te.ok("sync", "--json")), &reps); err != nil || len(reps) != 1 || reps[0].Unpushable != 1 {
		t.Errorf("--json = %+v, %v", reps, err)
	}
}
