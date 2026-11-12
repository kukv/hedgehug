package workspace_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func seqIDs() func() change.ID {
	n := 0
	return func() change.ID {
		n++
		return change.ID(fmt.Sprintf("01VAL%021d", n))
	}
}

func names(vs []workspace.Value) []string {
	var out []string
	for _, v := range vs {
		out = append(out, v.Name+":"+string(v.Color)+":"+string(v.Category))
	}
	return out
}

// 03「ワークスペースの作成」の初期値。
func TestDefaultSettings(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	if got := names(s.Statuses); !reflect.DeepEqual(got, []string{
		"Todo:gray:todo", "InProgress:blue:doing", "Done:green:done", "Pending:yellow:todo", "Cancel:gray:canceled",
	}) {
		t.Errorf("statuses = %v", got)
	}
	if got := names(s.Priorities); !reflect.DeepEqual(got, []string{
		"Highest:red:", "High:orange:", "Medium:yellow:", "Low:blue:", "Lowest:gray:",
	}) {
		t.Errorf("priorities = %v", got)
	}
	if got := names(s.Systems); !reflect.DeepEqual(got, []string{
		"Web:blue:", "API:teal:", "iOS:gray:", "Android:green:", "DB:purple:", "CI:yellow:",
	}) {
		t.Errorf("systems = %v", got)
	}
	if s.InitialStatus != s.Statuses[0].ID || s.InitialPriority != s.Priorities[2].ID {
		t.Errorf("initial = %s / %s", s.InitialStatus, s.InitialPriority)
	}
	want := map[workspace.Category]change.ID{
		workspace.CategoryTodo: s.Statuses[0].ID, workspace.CategoryDoing: s.Statuses[1].ID,
		workspace.CategoryDone: s.Statuses[2].ID, workspace.CategoryCanceled: s.Statuses[4].ID,
	}
	if !reflect.DeepEqual(s.CategoryDefaults, want) {
		t.Errorf("category defaults = %v", s.CategoryDefaults)
	}
	var keys []string
	for _, p := range s.Properties {
		keys = append(keys, fmt.Sprintf("%s:%s:%s:%v", p.Key, p.Name, p.Type, p.Builtin))
		if string(p.ID) != p.Key || !reflect.DeepEqual(p.Kinds, workspace.Kinds) || p.Hidden {
			t.Errorf("property %+v", p)
		}
	}
	if !reflect.DeepEqual(keys, []string{
		"status:ステータス:status:true", "assignee:担当者:user:true", "priority:優先度:priority:true",
		"start:開始日:date:true", "end:終了日:date:true", "points:ポイント:number:true",
		"estimate:見積もり時間:duration:true", "systems:関連システム:systems:true",
	}) {
		t.Errorf("properties = %v", keys)
	}
}

func TestColorsAreTheEighteenOfSpec03(t *testing.T) {
	if len(workspace.Colors) != 18 || workspace.Colors[0] != "gray" || workspace.Colors[17] != "orange" {
		t.Errorf("colors = %v", workspace.Colors)
	}
}

func stamp(log []change.Record, d change.Draft, id string, sec int) []change.Record {
	meta := change.Meta{Op: change.ID(id), At: time.Date(2026, 11, 12, 9, 0, sec, 0, time.UTC), Actor: change.User("taro")}
	return append(log, change.Stamp(log, []change.Draft{d}, meta, func() change.ID { return change.ID(id) })...)
}

func TestSettingsRoundTripThroughTheLog(t *testing.T) {
	w := workspace.Workspace{ID: "01WS0000000000000000000001", Name: "Web", Prefix: "HH"}
	s := workspace.DefaultSettings(seqIDs())
	d, err := workspace.CreateWithSettings(w, s)
	if err != nil {
		t.Fatal(err)
	}
	log := stamp(nil, d, "c", 0)
	got, ok := workspace.SettingsFromLog(log)
	if !ok || !reflect.DeepEqual(got, s) {
		t.Fatalf("SettingsFromLog =\n%+v\nwant\n%+v", got, s)
	}
	if back, ok := workspace.FromLog(log); !ok || back != w {
		t.Errorf("FromLog = %+v", back)
	}
	// 値の名前を変え、使う種類から Step を外すと、その項目だけが変わる。
	todo := s.Statuses[0].ID
	log = stamp(log, change.NewSet("statuses/"+string(todo)+"/name", "Todo", "未着手"), "s1", 1)
	log = stamp(log, change.NewRemove("properties/assignee/kinds", "Step"), "s2", 2)
	got, _ = workspace.SettingsFromLog(log)
	if got.Statuses[0].Name != "未着手" || got.Statuses[1].Name != "InProgress" {
		t.Errorf("statuses = %v", names(got.Statuses))
	}
	if !reflect.DeepEqual(got.Properties[1].Kinds, []string{"Epic", "Story", "Task", "Bug"}) {
		t.Errorf("assignee kinds = %v", got.Properties[1].Kinds)
	}
}

func TestInvalidColorAndCategoryRowsAreIgnored(t *testing.T) {
	w := workspace.Workspace{ID: "01WS0000000000000000000001", Name: "Web", Prefix: "HH"}
	s := workspace.DefaultSettings(seqIDs())
	d, _ := workspace.CreateWithSettings(w, s)
	todo := "statuses/" + string(s.Statuses[0].ID)
	log := stamp(nil, d, "c", 0)
	log = stamp(log, change.NewSet(todo+"/color", "gray", "chartreuse"), "s1", 1)
	log = stamp(log, change.NewSet(todo+"/category", "todo", "someday"), "s2", 2)
	got, _ := workspace.SettingsFromLog(log)
	if got.Statuses[0].Color != "gray" || got.Statuses[0].Category != workspace.CategoryTodo {
		t.Errorf("todo = %+v", got.Statuses[0])
	}
}

func TestCreateWithSettingsChecksLikeCreate(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	if _, err := workspace.CreateWithSettings(workspace.Workspace{ID: "W", Name: "Web", Prefix: "hh"}, s); err == nil {
		t.Error("bad prefix accepted")
	}
	if _, err := workspace.CreateWithSettings(workspace.Workspace{ID: "W", Name: " ", Prefix: "HH"}, s); err == nil {
		t.Error("empty name accepted")
	}
}
