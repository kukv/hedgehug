package store

import (
	"path/filepath"
	"testing"
)

func TestPrefixUses(t *testing.T) {
	s := newTestStore(t) // workspace "test" with the prefix TST
	opened := 0
	openStore = func(dir string) (*Store, error) { opened++; return Open(dir) }
	defer func() { openStore = Open }()
	var skipped []string
	uses := PrefixUses([]NamedStore{
		{Name: "gone", Path: filepath.Join(t.TempDir(), "gone")},
		{Name: "a", Path: s.dir},
	}, []string{"web", "tst"}, func(name string, _ error) { skipped = append(skipped, name) })
	if len(uses) != 1 || uses[0].Store != "a" || uses[0].Workspace.Name != "test" {
		t.Errorf("uses = %+v", uses)
	}
	if len(skipped) != 1 || skipped[0] != "gone" {
		t.Errorf("skipped = %v", skipped)
	}
	if opened != 2 {
		t.Errorf("opened %d stores for 2 prefixes and 2 stores, want each once", opened)
	}
	if uses := PrefixUses([]NamedStore{{Name: "a", Path: s.dir}}, []string{"WEB"}, nil); len(uses) != 0 {
		t.Errorf("uses of WEB = %+v", uses)
	}
}
