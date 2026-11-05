package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRebuildPhaseIn(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		data string
		want RebuildPhase
	}{
		"none":          {"", RebuildNone},
		"apply":         {`{"phase":"apply"}`, RebuildApply},
		"prune":         {`{"phase":"prune"}`, RebuildPrune},
		"broken":        {"not json", RebuildApply},
		"unknown phase": {`{"phase":"later"}`, RebuildApply},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			local := filepath.Join(dir, filepath.FromSlash(LocalDir))
			if err := os.MkdirAll(local, 0o750); err != nil {
				t.Fatal(err)
			}
			if c.data != "" {
				if err := os.WriteFile(filepath.Join(local, RebuildFile), []byte(c.data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if got := RebuildPhaseIn(dir); got != c.want {
				t.Errorf("phase = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRebuildPhaseInOfAMissingStore(t *testing.T) {
	t.Parallel()
	if got := RebuildPhaseIn(filepath.Join(t.TempDir(), "missing")); got != RebuildNone {
		t.Errorf("phase = %q", got)
	}
}
