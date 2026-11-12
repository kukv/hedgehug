package item_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

func kind(k item.Kind) *item.Kind { return &k }

// 01「用語と階層」の親子の表。
func TestCanPlace(t *testing.T) {
	cases := []struct {
		child  item.Kind
		parent *item.Kind
		want   bool
	}{
		{item.Epic, nil, true},
		{item.Epic, kind(item.Epic), false},
		{item.Story, kind(item.Epic), true},
		{item.Task, kind(item.Epic), true},
		{item.Bug, kind(item.Epic), true},
		{item.Task, nil, false},
		{item.Task, kind(item.Story), false},
		{item.Step, kind(item.Story), true},
		{item.Step, kind(item.Task), true},
		{item.Step, kind(item.Bug), true},
		{item.Step, kind(item.Epic), false},
		{item.Step, kind(item.Step), false},
		{item.Kind("Saga"), nil, false},
	}
	for _, c := range cases {
		if got := item.CanPlace(c.child, c.parent); got != c.want {
			t.Errorf("CanPlace(%s, %v) = %v", c.child, c.parent, got)
		}
	}
}

func TestCreateAndFromLog(t *testing.T) {
	parent := item.ID("E")
	it := item.Item{
		ID:    "T",
		Place: item.Place{Workspace: "W", Project: "P", Parent: &parent, Number: 12},
		Kind:  item.Task, Rank: "a0V", Title: "ログイン画面を作る",
		Watchers: []string{"hanako"},
	}
	ts := time.Date(2026, 11, 10, 9, 0, 0, 0, time.FixedZone("JST", 9*3600))
	log := change.Stamp(nil, []change.Draft{item.Create(it)}, change.Meta{Op: "o", At: ts, Actor: change.User("hanako")}, func() change.ID { return "c" })
	log = append(log, change.Stamp(log, []change.Draft{change.NewSet("title", it.Title, "ログイン画面")}, change.Meta{Op: "o2", At: ts.Add(time.Hour), Actor: change.User("taro")}, func() change.ID { return "s" })...)
	got, ok := item.FromLog(log)
	if !ok {
		t.Fatal("no item")
	}
	if got.Title != "ログイン画面" || got.Kind != item.Task || *got.Place.Parent != "E" || got.Place.Number != 12 {
		t.Errorf("FromLog = %+v", got)
	}
	if got.Created.By != "hanako" || !got.Created.At.Equal(ts) {
		t.Errorf("Created = %+v", got.Created)
	}
	if got.DisplayID("HH") != "HH-12" {
		t.Errorf("DisplayID = %s", got.DisplayID("HH"))
	}
	if len(got.Watchers) != 1 || got.Watchers[0] != "hanako" {
		t.Errorf("Watchers = %v", got.Watchers)
	}
}
