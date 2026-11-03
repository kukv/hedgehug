package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The modules of the binary all have a license file, so a new dependency
// without one fails here before it fails a release.
func TestRunWritesEveryModule(t *testing.T) {
	t.Parallel()
	web := filepath.Join(t.TempDir(), "web")
	if err := os.WriteFile(web, []byte("==== svelte 5 (npm) ====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run(&out, []string{web}); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{
		"==== Go ",                          // the standard library
		"==== github.com/go-git/go-git/v5 ", // Apache-2.0
		"Apache License",
		"==== github.com/godbus/dbus/v5 ", // linked on Linux only
		"==== svelte 5 (npm) ====",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output has no %q", want)
		}
	}
	if strings.Contains(s, "==== github.com/kukv/hedgehug ") {
		t.Error("output has the main module")
	}
}

func TestSectionFailsWithoutALicense(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "NOTICE"), []byte("n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := section(&strings.Builder{}, "m v1", dir); err == nil {
		t.Fatal("section = nil, want an error for a NOTICE without a license")
	}
}
