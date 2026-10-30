package main

import (
	"bytes"
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
		{"-h prints usage and succeeds", []string{"-h"}, 0, "", "-version"},
		{"no arguments prints usage and fails", nil, 2, "", "-version"},
		{"an unknown flag fails", []string{"--nope"}, 2, "", "not defined: -nope"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tc.args, &stdout, &stderr); got != tc.wantCode {
				t.Errorf("run(%q) = %d, want %d", tc.args, got, tc.wantCode)
			}
			if got := stdout.String(); got != tc.wantStdout {
				t.Errorf("stdout = %q, want %q", got, tc.wantStdout)
			}
			if got := stderr.String(); !strings.Contains(got, tc.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", got, tc.wantStderr)
			}
		})
	}
}
