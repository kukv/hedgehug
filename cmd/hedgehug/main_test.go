package main

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

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

// The name is not TestNoShort + "IDs", which the pattern would match.
func TestNoShortIdentifiers(t *testing.T) {
	// Built so that this file does not match itself.
	s := "Short"
	pattern := s + "ID|" + s + "Lengths|min" + s + `|"` + strings.ToLower(s) + `"|\.` + s + `\b`
	// repo_test.go calls the method of go-git that shortens a reference
	// name, which is not a short ULID.
	out, err := exec.Command("grep", "-rnE", pattern, "--include=*.go", "--exclude=repo_test.go", "../../cmd", "../../internal", "../../tools").CombinedOutput()
	// grep exits 1 when nothing matches, and 2 on an error.
	if ee := (*exec.ExitError)(nil); !errors.As(err, &ee) || ee.ExitCode() != 1 {
		t.Errorf("short ULIDs are still in the code, or grep failed: %v\n%s", err, out)
	}
}
