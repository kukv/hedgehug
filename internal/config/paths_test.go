package config

import (
	"path/filepath"
	"testing"
)

// envOf returns a getenv that reads m.
func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestResolve(t *testing.T) {
	h := filepath.Join(t.TempDir(), "home")
	hh := filepath.Join(t.TempDir(), "hh")
	xc := filepath.Join(t.TempDir(), "xc")
	xd := filepath.Join(t.TempDir(), "xd")
	for _, tc := range []struct {
		name       string
		goos       string
		env        map[string]string
		wantConfig string
		wantStores string
	}{
		{
			"HEDGEHUG_HOME wins over XDG", "linux",
			map[string]string{"HOME": h, "HEDGEHUG_HOME": hh, "XDG_CONFIG_HOME": xc, "XDG_DATA_HOME": xd},
			filepath.Join(hh, "config.yaml"), filepath.Join(hh, "stores"),
		},
		{
			"both XDG", "linux",
			map[string]string{"HOME": h, "XDG_CONFIG_HOME": xc, "XDG_DATA_HOME": xd},
			filepath.Join(xc, "hedgehug", "config.yaml"), filepath.Join(xd, "hedgehug", "stores"),
		},
		{
			"only XDG_CONFIG_HOME", "linux",
			map[string]string{"HOME": h, "XDG_CONFIG_HOME": xc},
			filepath.Join(xc, "hedgehug", "config.yaml"), filepath.Join(h, ".local", "share", "hedgehug", "stores"),
		},
		{
			"only XDG_DATA_HOME", "linux",
			map[string]string{"HOME": h, "XDG_DATA_HOME": xd},
			filepath.Join(h, ".config", "hedgehug", "config.yaml"), filepath.Join(xd, "hedgehug", "stores"),
		},
		{
			"relative values are ignored", "linux",
			map[string]string{"HOME": h, "HEDGEHUG_HOME": "rel", "XDG_CONFIG_HOME": "rel", "XDG_DATA_HOME": "rel"},
			filepath.Join(h, ".config", "hedgehug", "config.yaml"), filepath.Join(h, ".local", "share", "hedgehug", "stores"),
		},
		{
			"macOS defaults", "darwin",
			map[string]string{"HOME": h},
			filepath.Join(h, "Library", "Application Support", "hedgehug", "config.yaml"), filepath.Join(h, ".local", "share", "hedgehug", "stores"),
		},
		{
			"Windows defaults", "windows",
			map[string]string{"USERPROFILE": h, "APPDATA": xc, "LOCALAPPDATA": xd},
			filepath.Join(xc, "hedgehug", "config.yaml"), filepath.Join(xd, "hedgehug", "stores"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := goos
			goos = tc.goos
			defer func() { goos = old }()
			p, err := Resolve(envOf(tc.env))
			if err != nil {
				t.Fatal(err)
			}
			if p.ConfigFile != tc.wantConfig {
				t.Errorf("ConfigFile = %q, want %q", p.ConfigFile, tc.wantConfig)
			}
			if p.DataDir != filepath.Dir(tc.wantStores) {
				t.Errorf("DataDir = %q, want %q", p.DataDir, filepath.Dir(tc.wantStores))
			}
			if p.StoresDir != tc.wantStores {
				t.Errorf("StoresDir = %q, want %q", p.StoresDir, tc.wantStores)
			}
		})
	}
}

func TestResolveNeedsHome(t *testing.T) {
	old := goos
	goos = "linux"
	defer func() { goos = old }()
	if _, err := Resolve(envOf(nil)); err == nil {
		t.Error("Resolve with no HOME succeeded")
	}
}
