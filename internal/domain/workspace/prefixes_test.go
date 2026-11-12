package workspace_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 旧い接頭辞は、ログに出たすべての接頭辞から、いまのものを除いたもの(この計画の決定 8)。
func TestOldPrefixes(t *testing.T) {
	d, err := workspace.Create(workspace.Workspace{ID: "W", Name: "Web", Prefix: "HH"})
	if err != nil {
		t.Fatal(err)
	}
	log := stamp(nil, d, "c", 0)
	if got := workspace.OldPrefixes(log); len(got) != 0 {
		t.Errorf("fresh workspace: %v", got)
	}
	log = stamp(log, change.NewSet("prefix", "HH", "WEB"), "s1", 1)
	log = stamp(log, change.NewSet("prefix", "WEB", "APP"), "s2", 2)
	log = stamp(log, change.NewSet("prefix", "APP", "HH"), "s3", 3)
	log = stamp(log, change.NewSet("prefix", "HH", "bad!"), "s4", 4) // 形の違う行は数えない
	if got := workspace.OldPrefixes(log); !reflect.DeepEqual(got, []string{"APP", "WEB"}) {
		t.Errorf("OldPrefixes = %v", got)
	}
}
