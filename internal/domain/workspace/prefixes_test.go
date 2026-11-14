package workspace_test

import (
	"reflect"
	"testing"
	"time"

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

// 接頭辞ごとの、ログに最初に出た日時(B5c-13 のワークスペースの記録の照合で、移した時点までに使った接頭辞に絞る)。
func TestPrefixTimes(t *testing.T) {
	d, err := workspace.Create(workspace.Workspace{ID: "W", Name: "Web", Prefix: "HH"})
	if err != nil {
		t.Fatal(err)
	}
	log := stamp(nil, d, "c", 0)
	log = stamp(log, change.NewSet("prefix", "HH", "WEB"), "s1", 1)
	log = stamp(log, change.NewSet("prefix", "WEB", "HH"), "s2", 2) // 戻しても最初の日時のまま
	log = stamp(log, change.NewSet("prefix", "HH", "bad!"), "s3", 3)
	at := func(sec int) time.Time { return time.Date(2026, 11, 12, 9, 0, sec, 0, time.UTC) }
	want := map[string]time.Time{"HH": at(0), "WEB": at(1)}
	if got := workspace.PrefixTimes(log); !reflect.DeepEqual(got, want) {
		t.Errorf("PrefixTimes = %v, want %v", got, want)
	}
}
