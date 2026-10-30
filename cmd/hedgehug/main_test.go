package main

import (
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
