package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// testPaths returns paths under a temporary directory.
func testPaths(t *testing.T) Paths {
	t.Helper()
	d := t.TempDir()
	return Paths{
		Home:       filepath.Join(d, "home"),
		ConfigFile: filepath.Join(d, "cfg", "config.yaml"),
		StoresDir:  filepath.Join(d, "stores"),
	}
}

func writeConfig(t *testing.T, p Paths, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p.ConfigFile), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ConfigFile, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	c, err := Load(testPaths(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Stores) != 0 {
		t.Errorf("Stores = %v, want none", c.Stores)
	}
}

func TestLoadExpandsHome(t *testing.T) {
	p := testPaths(t)
	writeConfig(t, p, "stores:\n  - name: me\n    path: ~/data/me\n  - name: abs\n    path: "+filepath.ToSlash(filepath.Join(p.Home, "x"))+"\n")
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	want := []StoreEntry{{Name: "me", Path: filepath.Join(p.Home, "data", "me")}, {Name: "abs", Path: filepath.Join(p.Home, "x")}}
	if !slices.EqualFunc(c.Stores, want, func(a, b StoreEntry) bool { return a.Name == b.Name && a.Path == b.Path }) {
		t.Errorf("Stores = %v, want %v", c.Stores, want)
	}
}

func TestLoadErrors(t *testing.T) {
	for _, tc := range []struct{ name, yaml, want string }{
		{"duplicate name", "stores:\n  - {name: a, path: /a}\n  - {name: a, path: /b}\n", ":3:"},
		{"missing path", "stores:\n  - name: a\n", ":2:"},
		{"missing name", "stores:\n  - path: /a\n", ":2:"},
		{"stores is not a list", "stores: x\n", ":1:"},
		{"not a mapping", "- a\n", ":1:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testPaths(t)
			writeConfig(t, p, tc.yaml)
			_, err := Load(p)
			if err == nil {
				t.Fatal("Load succeeded")
			}
			if !strings.Contains(err.Error(), p.ConfigFile+tc.want) {
				t.Errorf("err = %q, want it to name %q", err, p.ConfigFile+tc.want)
			}
		})
	}
}

func TestAddStoreKeepsTheRest(t *testing.T) {
	p := testPaths(t)
	writeConfig(t, p, `# my settings
accounts:
  - name: personal # the main one
    host: github.com
    login: alice
    token: gh
stores:
  - name: me
    path: ~/me
    account: personal
sync:
  interval: 2m
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("work", filepath.Join(p.Home, "stores", "work"), nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, s := range []string{"# my settings", "# the main one", "token: gh", "account: personal", "interval: 2m", "name: work", "path: ~/stores/work"} {
		if !strings.Contains(got, s) {
			t.Errorf("config.yaml lost %q:\n%s", s, got)
		}
	}
	c2, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(c2.Stores) != 2 || !reflect.DeepEqual(c2.Stores[1], StoreEntry{Name: "work", Path: filepath.Join(p.Home, "stores", "work")}) {
		t.Errorf("Stores after reload = %v", c2.Stores)
	}
}

func TestAddStoreToMissingFile(t *testing.T) {
	p := testPaths(t)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(p.Home), "elsewhere", "a")
	if err := c.AddStore("a", outside, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	c2, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(c2.Stores) != 1 || !reflect.DeepEqual(c2.Stores[0], StoreEntry{Name: "a", Path: outside}) {
		t.Errorf("Stores = %v", c2.Stores)
	}
}

func TestNullStoresIsEmpty(t *testing.T) {
	p := testPaths(t)
	writeConfig(t, p, "stores:\n")
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Stores) != 0 {
		t.Errorf("Stores = %v, want none", c.Stores)
	}
	outside := filepath.Join(filepath.Dir(p.Home), "elsewhere", "a")
	if err := c.AddStore("a", outside, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	c2, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(c2.Stores) != 1 || !reflect.DeepEqual(c2.Stores[0], StoreEntry{Name: "a", Path: outside}) {
		t.Errorf("Stores = %v", c2.Stores)
	}
}

func TestAddStoreRefusesDuplicates(t *testing.T) {
	p := testPaths(t)
	c, _ := Load(p)
	if err := c.AddStore("a", "/a", nil); err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("a", "/b", nil); err == nil {
		t.Error("a second store named a was added")
	}
}

func TestSelect(t *testing.T) {
	one := &Config{Stores: []StoreEntry{{Name: "me", Path: "/me"}}}
	two := &Config{Stores: []StoreEntry{{Name: "me", Path: "/me"}, {Name: "work", Path: "/work"}}}
	none := &Config{}
	for _, tc := range []struct {
		name    string
		c       *Config
		arg     string
		want    string
		wantErr string
	}{
		{"the only store", one, "", "me", ""},
		{"by name", two, "work", "work", ""},
		{"unknown name", two, "x", "", "me, work"},
		{"several and no name", two, "", "", "--store"},
		{"none", none, "", "", "store init"},
		{"none with a name", none, "me", "", "store init"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.c.Select(tc.arg)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("err = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Name != tc.want {
				t.Errorf("Select(%q) = %q, want %q", tc.arg, got.Name, tc.want)
			}
		})
	}
	if _, err := none.Select(""); !errors.Is(err, ErrNoStores) {
		t.Errorf("err = %v, want ErrNoStores", err)
	}
}

func TestValidStoreName(t *testing.T) {
	for _, ok := range []string{"me", "org-a", "a_1", "0x"} {
		if err := ValidStoreName(ok); err != nil {
			t.Errorf("ValidStoreName(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "-a", "_a", "A", "a b", "a/b", "..", "日本"} {
		if err := ValidStoreName(bad); err == nil {
			t.Errorf("ValidStoreName(%q) succeeded", bad)
		}
	}
}

// addAndRead loads the config, adds a store, saves, and returns the file.
func addAndRead(t *testing.T, p Paths) string {
	t.Helper()
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("b", "/elsewhere/b", nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSaveKeepsSequenceIndent(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"indented", "stores:\n  - name: a\n    path: /x\n", "stores:\n  - name: a\n    path: /x\n  - name: b\n    path: /elsewhere/b\n"},
		{"flush", "stores:\n- name: a\n  path: /x\n", "stores:\n- name: a\n  path: /x\n- name: b\n  path: /elsewhere/b\n"},
		{"no file", "", "stores:\n  - name: b\n    path: /elsewhere/b\n"},
		{"other list flush, no stores", "other:\n- x\n", "other:\n- x\nstores:\n- name: b\n  path: /elsewhere/b\n"},
		{"other list indented, no stores", "other:\n  - x\n", "other:\n  - x\nstores:\n  - name: b\n    path: /elsewhere/b\n"},
		{"empty stores", "stores:\n", "stores:\n  - name: b\n    path: /elsewhere/b\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testPaths(t)
			if tc.in != "" {
				writeConfig(t, p, tc.in)
			}
			if got := addAndRead(t, p); got != tc.want {
				t.Errorf("config.yaml =\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

func TestSaveFollowsSymlink(t *testing.T) {
	p := testPaths(t)
	target := filepath.Join(filepath.Dir(p.ConfigFile), "real", "dotfiles.yaml")
	writeConfig(t, p, "")
	if err := os.Remove(p.ConfigFile); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("stores:\n  - name: a\n    path: /x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, p.ConfigFile); err != nil {
		t.Skipf("cannot create a symlink: %v", err)
	}
	addAndRead(t, p)
	if fi, err := os.Lstat(p.ConfigFile); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("config.yaml is no longer a symlink: %v, %v", fi, err)
	}
	b, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "name: b") {
		t.Errorf("target was not rewritten:\n%s", b)
	}
}

func TestSaveKeepsCommentOnlyFile(t *testing.T) {
	p := testPaths(t)
	writeConfig(t, p, "# my config\n")
	got := addAndRead(t, p)
	if !strings.Contains(got, "# my config") || !strings.Contains(got, "name: b") {
		t.Errorf("config.yaml =\n%s", got)
	}
}

func TestRelativePathIsUnderConfigDir(t *testing.T) {
	p := testPaths(t)
	writeConfig(t, p, "stores:\n  - name: me\n    path: stores/me\n")
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	s, err := c.Select("")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(filepath.Dir(p.ConfigFile), "stores", "me"); s.Path != want {
		t.Errorf("Path = %q, want %q", s.Path, want)
	}
}

// cfgPaths writes src as config.yaml under a temporary directory.
func cfgPaths(t *testing.T, src string) Paths {
	t.Helper()
	p := testPaths(t)
	writeConfig(t, p, src)
	return p
}

func TestLoadAccountsRemotesSync(t *testing.T) {
	p := cfgPaths(t, `accounts:
  - name: personal
    host: github.com
    login: alice
    token: secret
stores:
  - name: org-a
    path: /s/org-a
    remotes: [/r/a.git, /r/b.git]
    account: personal
sync:
  interval: 5m
  push_interval: 10s
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Accounts) != 1 || c.Accounts[0].ID() != "github.com/alice" {
		t.Errorf("accounts = %+v", c.Accounts)
	}
	if !slices.Equal(c.Me(), []string{"github.com/alice"}) {
		t.Errorf("Me = %v", c.Me())
	}
	e := c.Stores[0]
	if !slices.Equal(e.Remotes, []string{"/r/a.git", "/r/b.git"}) || e.Account != "personal" {
		t.Errorf("store = %+v", e)
	}
	if c.Sync.Interval != 5*time.Minute || c.Sync.PushInterval != 10*time.Second {
		t.Errorf("sync = %+v", c.Sync)
	}
	if a, ok := c.Writer(e); !ok || a.Login != "alice" {
		t.Errorf("Writer = %+v, %v", a, ok)
	}
}

func TestSyncDefaults(t *testing.T) {
	c, err := Load(cfgPaths(t, "stores: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Sync.Interval != 2*time.Minute || c.Sync.PushInterval != 30*time.Second {
		t.Errorf("defaults = %+v", c.Sync)
	}
	c, err = Load(testPaths(t))
	if err != nil || c.Sync.Interval != 2*time.Minute {
		t.Errorf("missing file: %+v, %v", c.Sync, err)
	}
}

func TestWriterRules(t *testing.T) {
	two := `accounts:
  - {name: a, host: github.com, login: a}
  - {name: b, host: github.com, login: b}
stores:
  - {name: s, path: /s}
  - {name: t, path: /t, account: b}
`
	c, err := Load(cfgPaths(t, two))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Writer(c.Stores[0]); ok {
		t.Error("two accounts and no stores[].account: want no writer")
	}
	if a, ok := c.Writer(c.Stores[1]); !ok || a.Name != "b" {
		t.Errorf("named account: Writer = %+v, %v", a, ok)
	}
	one := strings.Replace(two, "  - {name: b, host: github.com, login: b}\n", "", 1)
	one = strings.Replace(one, ", account: b", "", 1)
	c, err = Load(cfgPaths(t, one))
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := c.Writer(c.Stores[0]); !ok || a.Name != "a" {
		t.Errorf("one account: Writer = %+v, %v", a, ok)
	}
	c, _ = Load(cfgPaths(t, "stores:\n  - {name: s, path: /s}\n"))
	if _, ok := c.Writer(c.Stores[0]); ok {
		t.Error("no accounts: want no writer")
	}
}

func TestBadAccountAndSync(t *testing.T) {
	for _, src := range []string{
		"accounts:\n  - {name: a, host: github.com}\n",                                                     // no login
		"accounts:\n  - {name: a, host: h, login: x}\n  - {name: a, host: h, login: y}\n",                  // twice
		"stores:\n  - {name: s, path: /s, account: nobody}\naccounts: []\n",                                // unknown account
		"stores:\n  - {name: s, path: /s, remotes: x}\n",                                                   // remotes not a list
		"accounts:\n  - {name: a, host: h, login: x}\nstores:\n  - {name: s, path: /s, account: \"\"}\n",   // empty account
		"accounts:\n  - {name: a, host: h, login: x}\nstores:\n  - {name: s, path: /s, account: {x: y}}\n", // account is a mapping
		"sync:\n  interval: soon\n",
		"sync:\n  push_interval: 0s\n",
		"sync:\n  interval: -1m\n",
	} {
		if _, err := Load(cfgPaths(t, src)); err == nil {
			t.Errorf("Load(%q) succeeded", src)
		}
	}
}

func TestAddAndRemoveStoreKeepsComments(t *testing.T) {
	p := cfgPaths(t, "# mine\nstores:\n  - name: a # first\n    path: /a\n  - name: c\n    path: /c # third\nlog_level: debug # keep\n")
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("b", "/b", []string{"/r/b.git"}); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveStore("a"); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveStore("nope"); err == nil {
		t.Error("RemoveStore of an unknown store succeeded")
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p.ConfigFile)
	want := "# mine\nstores:\n  - name: c\n    path: /c # third\n  - name: b\n    path: /b\n    remotes: [/r/b.git]\nlog_level: debug # keep\n"
	if string(b) != want {
		t.Errorf("config.yaml =\n%s\nwant\n%s", b, want)
	}
	c, err = Load(p)
	if err != nil || len(c.Stores) != 2 || c.Stores[1].Remotes[0] != "/r/b.git" {
		t.Errorf("reload: %+v, %v", c.Stores, err)
	}
}

func TestNullAccountIsNoAccount(t *testing.T) {
	c, err := Load(cfgPaths(t, "accounts:\n  - {name: a, host: h, login: x}\n  - {name: b, host: h, login: y}\nstores:\n  - {name: s, path: /s, account: null}\n"))
	if err != nil || c.Stores[0].Account != "" {
		t.Errorf("null account: %+v, %v", c.Stores, err)
	}
}

func TestWriterNamedButMissingAccount(t *testing.T) {
	c := &Config{Accounts: []Account{{Name: "a", Host: "h", Login: "x"}}}
	if a, ok := c.Writer(StoreEntry{Name: "s", Account: "gone"}); ok {
		t.Errorf("Writer = %+v, true; want no writer", a)
	}
}

func TestAddStoreRefusesEmptyRemote(t *testing.T) {
	c, _ := Load(testPaths(t))
	if err := c.AddStore("a", "/a", []string{"/r", ""}); err == nil {
		t.Error("AddStore with an empty remote succeeded")
	}
	if len(c.Stores) != 0 {
		t.Errorf("Stores = %v", c.Stores)
	}
}

func TestRemoveOnlyStoreKeepsKey(t *testing.T) {
	p := cfgPaths(t, "# top\nstores:\n  - name: a\n    path: /a\nlog: 1\n")
	c, _ := Load(p)
	if err := c.RemoveStore("a"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p.ConfigFile)
	if want := "# top\nstores: []\nlog: 1\n"; string(b) != want {
		t.Errorf("config.yaml =\n%s\nwant\n%s", b, want)
	}
	c, err := Load(p)
	if err != nil || len(c.Stores) != 0 {
		t.Errorf("reload: %+v, %v", c.Stores, err)
	}
}

func TestRemoveStoreKeepsNeighborComments(t *testing.T) {
	p := cfgPaths(t, "stores:\n  # about a\n  - name: a\n    path: /a\n  # about b\n  - name: b\n    path: /b\n  # about c\n  - name: c\n    path: /c\n")
	c, _ := Load(p)
	if err := c.RemoveStore("b"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p.ConfigFile)
	want := "stores:\n  # about a\n  - name: a\n    path: /a\n  # about c\n  - name: c\n    path: /c\n"
	if string(b) != want {
		t.Errorf("config.yaml =\n%s\nwant\n%s", b, want)
	}
	if strings.Contains(string(b), "# about b") {
		t.Error("the comment of the removed item is still there")
	}
}
