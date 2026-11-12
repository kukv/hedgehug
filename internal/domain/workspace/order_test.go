package workspace_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestRankOf(t *testing.T) {
	w := workspace.Workspace{ID: "01WS0000000000000000000001", Name: "Web", Prefix: "HH"}
	plain, _ := workspace.Create(w)
	if got := workspace.RankOf(stamp(nil, plain, "c", 0)); got != "" {
		t.Errorf("no rank = %q", got)
	}
	ranked, err := workspace.CreateWithRank(w, workspace.DefaultSettings(seqIDs()), "V")
	if err != nil {
		t.Fatal(err)
	}
	log := stamp(nil, ranked, "c", 0)
	if got := workspace.RankOf(log); got != "V" {
		t.Errorf("create row rank = %q", got)
	}
	log = stamp(log, change.NewSet("rank", "V", "G"), "r", 1)
	if got := workspace.RankOf(log); got != "G" {
		t.Errorf("after set = %q", got)
	}
	if _, ok := workspace.SettingsFromLog(log); !ok {
		t.Error("settings are unreadable")
	}
}
