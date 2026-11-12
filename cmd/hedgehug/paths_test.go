//go:build !e2e

package main

import (
	"path/filepath"
	"testing"
)

func TestStatePathFollowsXDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data")
	got, err := statePath()
	if err != nil || got != filepath.Join("/tmp/xdg-data", "hedgehug", "state.db") {
		t.Errorf("statePath = %q, %v", got, err)
	}
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "/tmp/home")
	got, err = statePath()
	if err != nil || got != filepath.Join("/tmp/home", ".local", "share", "hedgehug", "state.db") {
		t.Errorf("statePath without XDG = %q, %v", got, err)
	}
}
