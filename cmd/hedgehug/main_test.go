package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// cobra on Windows lists every process on each run to tell whether Explorer
// started the command, about 40ms a run; hedgehug turns that off.
func TestNoExplorerCheck(t *testing.T) {
	if cobra.MousetrapHelpText != "" {
		t.Errorf("cobra.MousetrapHelpText = %q, want empty", cobra.MousetrapHelpText)
	}
}

func TestRun(t *testing.T) {
	for _, tc := range []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"--version prints the version", []string{"--version"}, 0, "hedgehug dev\n", ""},
		{"-h prints usage and succeeds", []string{"-h"}, 0, "Usage:", ""},
		{"no arguments prints usage and fails", nil, 2, "", "Usage:"},
		{"an unknown flag fails", []string{"--nope"}, 2, "", "unknown flag: --nope"},
		{"an unknown command fails", []string{"nope"}, 2, "", "unknown command"},
		{"store alone prints usage and fails", []string{"store"}, 2, "", "Usage:"},
		{"dep alone prints usage and fails", []string{"dep"}, 2, "", "Usage:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			te := newTestEnv(t)
			code, stdout, stderr := te.run(tc.args...)
			if code != tc.wantCode {
				t.Errorf("run(%q) = %d, want %d", tc.args, code, tc.wantCode)
			}
			if !strings.Contains(stdout, tc.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout, tc.wantStdout)
			}
			if !strings.Contains(stderr, tc.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, tc.wantStderr)
			}
		})
	}
}

// The name is not TestNoShort + "IDs", which the pattern would match. It reads
// the files itself, not with grep, so that it runs where only go is on PATH.
func TestNoShortIdentifiers(t *testing.T) {
	// Built so that this file does not match itself.
	s := "Short"
	re := regexp.MustCompile(s + "ID|" + s + "Lengths|min" + s + `|"` + strings.ToLower(s) + `"|\.` + s + `\b`)
	var found []string
	for _, root := range []string{"../../cmd", "../../internal", "../../tools"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			// repo_test.go calls the method of go-git that shortens a
			// reference name, which is not a short ULID.
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || d.Name() == "repo_test.go" {
				return err
			}
			b, err := os.ReadFile(path) //nolint:gosec // a source file of this repository
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(b), "\n") {
				if re.MatchString(line) {
					found = append(found, fmt.Sprintf("%s:%d:%s", path, i+1, line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(found) > 0 {
		t.Errorf("short ULIDs are still in the code:\n%s", strings.Join(found, "\n"))
	}
}
