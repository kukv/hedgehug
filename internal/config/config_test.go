package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	want := []StoreEntry{{"me", filepath.Join(p.Home, "data", "me")}, {"abs", filepath.Join(p.Home, "x")}}
	if len(c.Stores) != 2 || c.Stores[0] != want[0] || c.Stores[1] != want[1] {
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
	if err := c.AddStore("work", filepath.Join(p.Home, "stores", "work")); err != nil {
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
	if len(c2.Stores) != 2 || c2.Stores[1] != (StoreEntry{"work", filepath.Join(p.Home, "stores", "work")}) {
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
	if err := c.AddStore("a", outside); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	c2, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(c2.Stores) != 1 || c2.Stores[0] != (StoreEntry{"a", outside}) {
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
	if err := c.AddStore("a", outside); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	c2, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(c2.Stores) != 1 || c2.Stores[0] != (StoreEntry{"a", outside}) {
		t.Errorf("Stores = %v", c2.Stores)
	}
}

func TestAddStoreRefusesDuplicates(t *testing.T) {
	p := testPaths(t)
	c, _ := Load(p)
	if err := c.AddStore("a", "/a"); err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("a", "/b"); err == nil {
		t.Error("a second store named a was added")
	}
}

func TestSelect(t *testing.T) {
	one := &Config{Stores: []StoreEntry{{"me", "/me"}}}
	two := &Config{Stores: []StoreEntry{{"me", "/me"}, {"work", "/work"}}}
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
	if err := c.AddStore("b", "/elsewhere/b"); err != nil {
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
