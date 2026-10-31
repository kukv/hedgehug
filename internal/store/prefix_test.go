package store

import (
	"path/filepath"
	"testing"
)

func TestPrefixUses(t *testing.T) {
	s := newTestStore(t) // workspace "test" with the prefix TST
	var skipped []string
	uses := PrefixUses([]NamedStore{
		{Name: "gone", Path: filepath.Join(t.TempDir(), "gone")},
		{Name: "a", Path: s.dir},
	}, "tst", func(name string, _ error) { skipped = append(skipped, name) })
	if len(uses) != 1 || uses[0].Store != "a" || uses[0].Workspace.Name != "test" {
		t.Errorf("uses = %+v", uses)
	}
	if len(skipped) != 1 || skipped[0] != "gone" {
		t.Errorf("skipped = %v", skipped)
	}
	if uses := PrefixUses([]NamedStore{{Name: "a", Path: s.dir}}, "WEB", nil); len(uses) != 0 {
		t.Errorf("uses of WEB = %+v", uses)
	}
}
