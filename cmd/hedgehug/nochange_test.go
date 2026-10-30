package main

import (
	"strings"
	"testing"
)

func TestWriteWithoutChangeSaysSo(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")

	for _, args := range [][]string{
		{"set", a.ID, "title=same"},
		{"status", a.ID, "in_progress"},
	} {
		te.ok(args...)
		before := len(te.log("me"))
		out := te.ok(args...)
		if !strings.HasPrefix(out, "No changes") {
			t.Errorf("%v again: output %q, want it to start with No changes", args, out)
		}
		if n := len(te.log("me")); n != before {
			t.Errorf("%v again: commits = %d, want %d", args, n, before)
		}
	}
}
