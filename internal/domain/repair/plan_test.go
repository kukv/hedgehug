package repair_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// put は、drafts を op の行としてログの続きにする。行の ID は行数から決める。
func put(log []change.Record, op string, at time.Time, by string, ds ...change.Draft) []change.Record {
	n := len(log)
	meta := change.Meta{Op: change.ID(op), At: at, Actor: change.User(by)}
	return append(log, change.Stamp(log, ds, meta, func() change.ID {
		n++
		return change.ID(fmt.Sprintf("r%02d", n))
	})...)
}

var day = time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)

func wsBase() []change.Record {
	return put(nil, "o0", day, "taro", change.NewCreate(map[string]any{"id": "W1", "name": "W1", "prefix": "HH"}))
}

// addSprint は、スプリント id を計画中で作る。
func addSprint(log []change.Record, id, start string) []change.Record {
	s := planning.Sprint{ID: change.ID(id), Name: "Sprint " + id, State: planning.Planned, Start: start, End: "2026-12-31"}
	return put(log, "c"+id, day, "taro", planning.SprintDrafts(planning.Sprint{}, s)...)
}

// activate は、スプリント id を at に by が実行中にする。
func activate(log []change.Record, id string, at time.Time, by string) []change.Record {
	f := "sprints/" + id + "/state"
	return put(log, "a"+id, at, by, change.NewSet(f, planning.Planned, planning.Active))
}

func twoSprints(start1, start2 string) []change.Record {
	return addSprint(addSprint(wsBase(), "S1", start1), "S2", start2)
}

func TestExtraActive(t *testing.T) {
	log := activate(activate(twoSprints("2026-11-16", "2026-11-17"), "S1", day, "taro"), "S2", day, "taro")
	if got := repair.ExtraActive(log); !reflect.DeepEqual(got, []change.ID{"S2"}) {
		t.Fatalf("開始日が遅い方を計画中に戻す: %v", got)
	}
}

func TestExtraActiveTieBreak(t *testing.T) {
	for _, c := range []struct {
		name     string
		at1, at2 time.Time
		by1, by2 string
		want     []change.ID
	}{
		{"開始の行の at が早い方を残す", day, day.Add(time.Minute), "taro", "taro", []change.ID{"S2"}},
		{"at が同じなら by を小文字で比べる", day, day, "taro", "Hanako", []change.ID{"S1"}},
		{"by も同じなら行の ID が小さい方を残す", day, day, "taro", "taro", []change.ID{"S2"}},
	} {
		log := activate(activate(twoSprints("2026-11-16", "2026-11-16"), "S1", c.at1, c.by1), "S2", c.at2, c.by2)
		if got := repair.ExtraActive(log); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestExtraActiveNeedsTwoLive(t *testing.T) {
	one := activate(addSprint(wsBase(), "S1", "2026-11-16"), "S1", day, "taro")
	if got := repair.ExtraActive(one); len(got) != 0 {
		t.Errorf("実行中が 1 つなら空: %v", got)
	}
	both := activate(activate(twoSprints("2026-11-16", "2026-11-17"), "S1", day, "taro"), "S2", day, "taro")
	both = put(both, "d2", day, "taro", planning.DeleteSprint("S2"))
	if got := repair.ExtraActive(both); len(got) != 0 {
		t.Errorf("削除済みの実行中は数えない: %v", got)
	}
}

func TestExtraActiveThree(t *testing.T) {
	log := addSprint(addSprint(addSprint(wsBase(), "S1", "2026-11-18"), "S2", "2026-11-16"), "S3", "2026-11-17")
	for _, id := range []string{"S1", "S2", "S3"} {
		log = activate(log, id, day, "taro")
	}
	if got := repair.ExtraActive(log); !reflect.DeepEqual(got, []change.ID{"S1", "S3"}) {
		t.Fatalf("開始日が最も早い S2 を残し、残りを内部 ID の順で戻す: %v", got)
	}
}

var settings = workspace.Settings{Statuses: []workspace.Value{
	{ID: "todo", Category: workspace.CategoryTodo},
	{ID: "doing", Category: workspace.CategoryDoing},
	{ID: "done", Category: workspace.CategoryDone},
}}

// closedLog は、S1 を実行中にして完了した保存(op oc)までのログ。
func closedLog() []change.Record {
	log := activate(addSprint(wsBase(), "S1", "2026-11-16"), "S1", day, "taro")
	return put(log, "oc", day, "taro",
		planning.CloseDraft("S1", planning.Confirmed{}),
		change.NewSet("sprints/S1/state", planning.Active, planning.Closed))
}

// itemLog は、アイテム id の作成の行と、drafts の行。
func itemLog(id string, kind item.Kind, status string, ds ...change.Draft) repair.ItemLog {
	it := item.Item{
		ID: item.ID(id), Kind: kind, Place: item.Place{Workspace: "W1", Project: "p1", Number: 9}, Rank: "a", Title: id,
		Status: change.ID(status), Sprint: "S1",
	}
	log := put(nil, "i"+id, day, "taro", append([]change.Draft{item.Create(it)}, ds...)...)
	return repair.ItemLog{Item: it, Log: log}
}

// rowOf は、f の最後の行の ID。
func rowOf(log []change.Record, f string) change.ID {
	var id change.ID
	for _, r := range log {
		if r.F == f {
			id = r.ID
		}
	}
	if id == "" {
		panic("row not found: " + f)
	}
	return id
}

func closeRows(log []change.Record) []change.ID {
	var ids []change.ID
	for _, r := range log {
		if r.Op == "oc" {
			ids = append(ids, r.ID)
		}
	}
	return ids
}

func TestClosedSprintItems(t *testing.T) {
	ws := closedLog()
	set := change.NewSet(item.SprintField, nil, "S1")
	todo := itemLog("HH9", item.Task, "todo", set)
	f := facts(closeRows(ws), []change.ID{rowOf(todo.Log, item.SprintField)})
	if got := repair.ClosedSprintItems(ws, settings, []repair.ItemLog{todo}, f); !reflect.DeepEqual(got, []item.ID{"HH9"}) {
		t.Fatalf("未着手の作業項目は戻す: %v", got)
	}
	deleted := itemLog("HH9", item.Task, "todo", set)
	deleted.Item.Deleted = &change.Deletion{}
	for name, it := range map[string]repair.ItemLog{
		"分類が完了": itemLog("HH9", item.Task, "done", set),
		"Epic":  itemLog("HH9", item.Epic, "todo", set),
		"削除済み":  deleted,
	} {
		f := facts(closeRows(ws), []change.ID{rowOf(it.Log, item.SprintField)})
		if got := repair.ClosedSprintItems(ws, settings, []repair.ItemLog{it}, f); len(got) != 0 {
			t.Errorf("%s は戻さない: %v", name, got)
		}
	}
}

func TestClosedSprintItemsDeletedSprint(t *testing.T) {
	ws := put(closedLog(), "d1", day, "taro", planning.DeleteSprint("S1"))
	todo := itemLog("HH9", item.Task, "todo", change.NewSet(item.SprintField, nil, "S1"))
	f := facts(closeRows(ws), []change.ID{rowOf(todo.Log, item.SprintField)})
	if got := repair.ClosedSprintItems(ws, settings, []repair.ItemLog{todo}, f); !reflect.DeepEqual(got, []item.ID{"HH9"}) {
		t.Errorf("削除済みの完了したスプリントも対象: %v", got)
	}
}

func TestClosedSprintItemsPlanned(t *testing.T) {
	ws := addSprint(wsBase(), "S1", "2026-11-16")
	todo := itemLog("HH9", item.Task, "todo", change.NewSet(item.SprintField, nil, "S1"))
	f := facts(nil, []change.ID{rowOf(todo.Log, item.SprintField)})
	if got := repair.ClosedSprintItems(ws, settings, []repair.ItemLog{todo}, f); len(got) != 0 {
		t.Errorf("計画中のスプリントは戻さない: %v", got)
	}
}

func TestClosedSprintItemsNeedsConcurrency(t *testing.T) {
	ws := closedLog()
	// 完了の行も HH-7 のスプリントの行も Facts にない。ステータスの行だけが Ours。
	hh7 := itemLog("HH7", item.Task, "doing", change.NewSet("status", "done", "doing"))
	only := facts([]change.ID{rowOf(hh7.Log, "status")}, nil)
	if got := repair.ClosedSprintItems(ws, settings, []repair.ItemLog{hh7}, only); len(got) != 0 {
		t.Errorf("完了を受け取ったあとの再オープンは戻さない: %v", got)
	}
	// 完了の行が Ours、ステータスを戻す行が Theirs。
	reopened := facts(closeRows(ws), []change.ID{rowOf(hh7.Log, "status")})
	if got := repair.ClosedSprintItems(ws, settings, []repair.ItemLog{hh7}, reopened); !reflect.DeepEqual(got, []item.ID{"HH7"}) {
		t.Errorf("完了と同時の再オープンは戻す: %v", got)
	}
	// 完了の行と HH-9 のスプリントの行が両方 Ours。
	hh9 := itemLog("HH9", item.Task, "todo", change.NewSet(item.SprintField, nil, "S1"))
	same := facts(append(closeRows(ws), rowOf(hh9.Log, item.SprintField)), nil)
	if got := repair.ClosedSprintItems(ws, settings, []repair.ItemLog{hh9}, same); len(got) != 0 {
		t.Errorf("同じ側なら戻さない: %v", got)
	}
	// スプリント S1 を選んで作った HH-10 の作成の行(set sprint の行はない)が Theirs。
	hh10 := itemLog("HH10", item.Task, "todo")
	created := facts(closeRows(ws), []change.ID{hh10.Log[0].ID})
	if got := repair.ClosedSprintItems(ws, settings, []repair.ItemLog{hh10}, created); !reflect.DeepEqual(got, []item.ID{"HH10"}) {
		t.Errorf("作成の行も欄の行として見る: %v", got)
	}
}

func refItem(id, sprint, release string) item.Item {
	return item.Item{ID: item.ID(id), Place: item.Place{Workspace: "W1"}, Sprint: change.ID(sprint), Release: change.ID(release)}
}

func plan(sprints, releases []string, deleted bool) planning.Plan {
	var p planning.Plan
	var d *change.Deletion
	if deleted {
		d = &change.Deletion{}
	}
	for _, s := range sprints {
		p.Sprints = append(p.Sprints, planning.Sprint{ID: change.ID(s), Deleted: d})
	}
	for _, r := range releases {
		p.Releases = append(p.Releases, planning.Release{ID: change.ID(r), State: planning.Archived})
	}
	return p
}

func TestDanglingPlanRefs(t *testing.T) {
	plans := map[workspace.ID]planning.Plan{"W1": plan([]string{"S1"}, []string{"R1"}, false), "W2": plan([]string{"S2"}, nil, false)}
	gone := refItem("HH1", "S9", "")
	other := refItem("HH3", "S2", "")
	got := repair.DanglingPlanRefs([]item.Item{gone, other}, plans)
	want := []repair.PlanRef{{Item: "HH1", Field: item.SprintField, Target: "S9"}, {Item: "HH3", Field: item.SprintField, Target: "S2"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DanglingPlanRefs = %v, want %v", got, want)
	}
	del := map[workspace.ID]planning.Plan{"W1": plan([]string{"S1"}, nil, true)}
	if got := repair.DanglingPlanRefs([]item.Item{refItem("HH1", "S1", "")}, del); len(got) != 0 {
		t.Errorf("削除済みのスプリントはある扱い: %v", got)
	}
	if got := repair.DanglingPlanRefs([]item.Item{refItem("HH1", "", "R1")}, plans); len(got) != 0 {
		t.Errorf("アーカイブのリリースはある扱い: %v", got)
	}
	dead := refItem("HH1", "S9", "R9")
	dead.Deleted = &change.Deletion{}
	if got := repair.DanglingPlanRefs([]item.Item{dead}, plans); len(got) != 0 {
		t.Errorf("削除済みのアイテムは見ない: %v", got)
	}
}

func TestDanglingBothFields(t *testing.T) {
	got := repair.DanglingPlanRefs([]item.Item{refItem("HH4", "S9", "R9")}, map[workspace.ID]planning.Plan{"W1": {}})
	want := []repair.PlanRef{{Item: "HH4", Field: item.SprintField, Target: "S9"}, {Item: "HH4", Field: item.ReleaseField, Target: "R9"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DanglingPlanRefs = %v, want %v", got, want)
	}
}

func TestDanglingUnknownWorkspace(t *testing.T) {
	got := repair.DanglingPlanRefs([]item.Item{refItem("HH5", "S1", "R1")}, map[workspace.ID]planning.Plan{"W2": plan([]string{"S1"}, []string{"R1"}, false)})
	want := []repair.PlanRef{{Item: "HH5", Field: item.SprintField, Target: "S1"}, {Item: "HH5", Field: item.ReleaseField, Target: "R1"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("計画のないワークスペースのアイテムは、スプリントもリリースもない扱い: %v, want %v", got, want)
	}
}
