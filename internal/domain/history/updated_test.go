package history_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func updatedBase() item.Item {
	return item.Item{ID: "T", Place: item.Place{Workspace: "W", Project: "P", Number: 1}, Kind: item.Task, Rank: "V", Title: "画面", Status: "V01"}
}

func TestUpdatedSkipsGitHubSyncAndRank(t *testing.T) {
	base := updatedBase()
	parent := item.ID("E1")
	moved := base.Place
	moved.Parent = &parent
	gh := change.Actor{Kind: change.ActorGitHub}
	pr := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 41}
	log := logOf(
		step{"O1", at(0), taro, item.Create(base)},
		step{"O2", at(1), taro, change.NewSet("status", "V01", "V02")},
		step{"O3", at(2), change.Actor{Kind: change.ActorAutomation, Login: "taro", Rule: "R1"}, change.NewSet("status", "V02", "V03")},
		step{"O4", at(3), gh, change.NewSet(itemlink.ValueField(pr, "title"), "A", "B")},
		step{"O5", at(4), change.Actor{Kind: change.ActorSync}, change.NewSet("place", base.Place, moved)},
		step{"O6", at(5), taro, change.NewSet("rank", "V", "W")},
	)
	if got, want := history.Updated(log, ctx()), at(2); !got.Equal(want) {
		t.Errorf("Updated = %v, want %v", got, want)
	}
}

func TestUpdatedCreateOnly(t *testing.T) {
	log := logOf(step{"O1", at(0), taro, item.Create(updatedBase())})
	if got := history.Updated(log, ctx()); !got.Equal(at(0)) {
		t.Errorf("Updated = %v, want %v", got, at(0))
	}
}

func TestUpdatedHiddenValueDeletion(t *testing.T) {
	log := logOf(
		step{"O1", at(0), taro, item.Create(updatedBase())},
		step{"O2", at(1), taro, change.NewSet("status", "V01", "V02")},
		step{"O3", at(2), taro, change.NewSet("status", "V02", "V03")},
	)
	c := ctx()
	c.Hidden = map[change.ID]bool{"O3": true}
	if got := history.Updated(log, c); !got.Equal(at(1)) {
		t.Errorf("Updated = %v, want %v", got, at(1))
	}
}
