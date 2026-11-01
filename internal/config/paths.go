// Package config finds where Hedgehug keeps its settings and stores, reads
// and writes config.yaml, and finds the author of commits.
package config

import (
	"errors"
	"path/filepath"
	"runtime"
)

// goos is runtime.GOOS; tests replace it.
var goos = runtime.GOOS

// Paths is where the settings and the stores are, as absolute paths.
type Paths struct {
	Home       string // the user's home directory
	ConfigFile string // config.yaml
	DataDir    string // the data of hedgehug: stores/ and ui.json
	StoresDir  string // the directory new stores go in, one per name
}

// Resolve finds the paths. HEDGEHUG_HOME holds both config.yaml and stores/
// when set; otherwise XDG_CONFIG_HOME and XDG_DATA_HOME are looked at each on
// its own, then the defaults of the OS. Relative values are ignored, as the
// XDG spec says.
func Resolve(getenv func(string) string) (Paths, error) {
	h, err := home(getenv)
	if err != nil {
		return Paths{}, err
	}
	p := Paths{Home: h}
	if hh := abs(getenv("HEDGEHUG_HOME")); hh != "" {
		p.ConfigFile = filepath.Join(hh, "config.yaml")
		p.DataDir = hh
		p.StoresDir = filepath.Join(hh, "stores")
		return p, nil
	}
	cfg := abs(getenv("XDG_CONFIG_HOME"))
	if cfg == "" {
		cfg = defaultConfigDir(getenv, h)
	}
	data := abs(getenv("XDG_DATA_HOME"))
	if data == "" {
		data = defaultDataDir(getenv, h)
	}
	p.ConfigFile = filepath.Join(cfg, "hedgehug", "config.yaml")
	p.DataDir = filepath.Join(data, "hedgehug")
	p.StoresDir = filepath.Join(p.DataDir, "stores")
	return p, nil
}

// home is the user's home directory: USERPROFILE on Windows, HOME elsewhere.
func home(getenv func(string) string) (string, error) {
	key := "HOME"
	if goos == "windows" {
		key = "USERPROFILE"
	}
	h := abs(getenv(key))
	if h == "" {
		return "", errors.New("the home directory is not known: $" + key + " is not set to an absolute path")
	}
	return h, nil
}

func defaultConfigDir(getenv func(string) string, home string) string {
	switch goos {
	case "windows":
		if d := abs(getenv("APPDATA")); d != "" {
			return d
		}
		return filepath.Join(home, "AppData", "Roaming")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support")
	default:
		return filepath.Join(home, ".config")
	}
}

func defaultDataDir(getenv func(string) string, home string) string {
	if goos == "windows" {
		if d := abs(getenv("LOCALAPPDATA")); d != "" {
			return d
		}
		return filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(home, ".local", "share")
}

// abs returns p when it is an absolute path, and "" otherwise.
func abs(p string) string {
	if p == "" || !filepath.IsAbs(p) {
		return ""
	}
	return filepath.Clean(p)
}
