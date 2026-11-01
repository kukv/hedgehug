package view

import (
	"strings"
	"testing"
)

func TestValidName(t *testing.T) {
	good := []string{"a", "今週のリリース作業", "Sprint 12 (web)", "a.b", strings.Repeat("あ", 64), "CONSOLE", "con-x"}
	for _, n := range good {
		if err := ValidName(n); err != nil {
			t.Errorf("ValidName(%q) = %v", n, err)
		}
	}
	bad := []string{
		"", strings.Repeat("a", 65), "a/b", `a\b`, "a:b", "a*b", "a?b", `a"b`, "a<b", "a>b", "a|b",
		"a\tb", "a\x00b", " a", "a ", "　a", ".a", "a.", "CON", "con", "Nul.txt", "com1", "LPT9", "aux.tar.gz", "CON .txt", "\xff",
	}
	for _, n := range bad {
		if err := ValidName(n); err == nil {
			t.Errorf("ValidName(%q) = nil; want an error", n)
		}
	}
}

func TestSameName(t *testing.T) {
	if !SameName("Release", "release") || SameName("a", "b") {
		t.Error("SameName folds case and nothing else")
	}
}

func TestIsBuiltin(t *testing.T) {
	for _, c := range []string{"id", "title", "kind", "status", "assignee", "label", "parent", "created", "updated"} {
		if !IsBuiltin(c) {
			t.Errorf("%s is not builtin", c)
		}
	}
	for _, c := range []string{"ulid", "due", "assignees", ""} {
		if IsBuiltin(c) {
			t.Errorf("%q is builtin", c)
		}
	}
}
