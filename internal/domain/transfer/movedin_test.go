package transfer_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

func movedIn(old string) transfer.MovedInInfo {
	return transfer.MovedInInfo{
		Old: old, FromStore: "S1", FromStoreName: "team-a", FromWorkspace: "W1", FromWorkspaceName: "Web",
		FromProject: "P1", FromProjectTitle: "ログイン", At: at, Names: map[change.ID]string{"V2": "High", "V1": "Todo"},
	}
}

func stamp(ds ...change.Draft) []change.Record {
	n := 0
	return change.Stamp(nil, ds, change.Meta{Op: "O", At: at, Actor: change.User("taro")}, func() change.ID {
		n++
		return change.ID("r" + string(rune('0'+n)))
	})
}

func TestNewMovedInWritesTheStoredShape(t *testing.T) {
	d := transfer.NewMovedIn(movedIn("HH-12"))
	if d.T != change.MovedIn || d.F != "" {
		t.Fatalf("draft = %+v", d)
	}
	b, err := json.Marshal(stamp(d)[0])
	want := `{"id":"r1","op":"O","at":"2026-11-14T09:00:00.000+09:00","by":"taro","actor":"user","t":"moved_in",` +
		`"to":{"old":"HH-12","from_store":"S1","from_store_name":"team-a","from_workspace":"W1","from_workspace_name":"Web",` +
		`"from_project":"P1","from_project_title":"ログイン","at":"2026-11-14T09:00:00.000+09:00","names":{"V1":"Todo","V2":"High"}},"prev":[]}`
	if err != nil || string(b) != want {
		t.Errorf("row =\n%s, %v\nwant\n%s", b, err, want)
	}
}

func TestMovedInOfReadsInWrittenOrderAndSkipsBrokenRows(t *testing.T) {
	log := stamp(transfer.NewMovedIn(movedIn("HH-12")), transfer.NewMovedIn(movedIn("WEB-31")),
		change.Draft{T: change.MovedIn, To: json.RawMessage(`"壊れた行"`)})
	got := transfer.MovedInOf(log)
	if len(got) != 2 || got[0].Old != "HH-12" || got[1].Old != "WEB-31" {
		t.Fatalf("MovedInOf = %+v", got)
	}
	g := got[0]
	if g.FromStore != "S1" || g.FromStoreName != "team-a" || g.FromWorkspace != "W1" || g.FromWorkspaceName != "Web" ||
		g.FromProject != "P1" || g.FromProjectTitle != "ログイン" || !g.At.Equal(at) || g.Names["V2"] != "High" {
		t.Errorf("first = %+v", g)
	}
}

// moved_in の行は、アイテムの値を変えない。
func TestMovedInRowDoesNotChangeTheItem(t *testing.T) {
	it := item.Item{ID: "I1", Place: item.Place{Workspace: "W2", Project: "P2", Number: 3}, Kind: item.Task, Rank: "a", Title: "移ってきた", Watchers: []string{}}
	log := stamp(item.Create(it), transfer.NewMovedIn(movedIn("HH-12")))
	got, ok := item.FromLog(log)
	if !ok || got.Title != "移ってきた" || got.Place != it.Place {
		t.Errorf("FromLog = %+v, %v", got, ok)
	}
}
