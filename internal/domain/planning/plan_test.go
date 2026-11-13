package planning_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// stampOp は、drafts を op の行としてログの続きにする(テスト用)。at は行数から決める。
func stampOp(log []change.Record, op change.ID, ds ...change.Draft) []change.Record {
	n := len(log)
	meta := change.Meta{Op: op, At: time.Date(2026, 11, 14, 9, 0, n, 0, time.UTC), Actor: change.User("taro")}
	return append(log, change.Stamp(log, ds, meta, func() change.ID {
		n++
		return change.ID("R" + string(rune('A'+n)))
	})...)
}

func base() []change.Record {
	return stampOp(nil, "o0", change.NewCreate(map[string]any{"id": "W", "name": "W", "prefix": "WS"}))
}

func sprint1() planning.Sprint {
	return planning.Sprint{
		ID: "S1", Name: "Sprint 1", State: planning.Planned,
		Start: "2026-11-02", End: "2026-11-15", Goal: "ログイン",
	}
}

func TestFromLogSprint(t *testing.T) {
	log := stampOp(base(), "o1", planning.SprintDrafts(planning.Sprint{}, sprint1())...)
	got := planning.FromLog(log).Sprints
	if len(got) != 1 || got[0].ID != "S1" || got[0].Name != "Sprint 1" || got[0].State != planning.Planned ||
		got[0].Start != "2026-11-02" || got[0].End != "2026-11-15" || got[0].Goal != "ログイン" ||
		got[0].Confirmed != nil || got[0].Deleted != nil {
		t.Fatalf("Sprints = %+v", got)
	}
	renamed := sprint1()
	renamed.Name = "Sprint 1b"
	ds := planning.SprintDrafts(sprint1(), renamed)
	if len(ds) != 1 {
		t.Fatalf("変わった項目だけ書く: %+v", ds)
	}
	log = stampOp(log, "o2", ds...)
	if got := planning.FromLog(log).Sprints; len(got) != 1 || got[0].Name != "Sprint 1b" {
		t.Fatalf("Sprints = %+v", got)
	}
}

func TestFromLogRelease(t *testing.T) {
	r := planning.Release{ID: "R1", Name: "v1", State: planning.Unreleased, Date: "2026-12-01", Description: "初回"}
	log := stampOp(base(), "o1", planning.ReleaseDrafts(planning.Release{}, r)...)
	got := planning.FromLog(log).Releases
	if len(got) != 1 || got[0] != r {
		t.Fatalf("Releases = %+v", got)
	}
}

func TestFromLogSkipsNameless(t *testing.T) {
	s := sprint1()
	s.Name = ""
	log := stampOp(base(), "o1", planning.SprintDrafts(planning.Sprint{}, s)...)
	if got := planning.FromLog(log).Sprints; len(got) != 0 {
		t.Fatalf("名前のないものは除く: %+v", got)
	}
}

func TestFromLogDeleteAndRestore(t *testing.T) {
	r := planning.Release{ID: "R1", Name: "v1", State: planning.Unreleased}
	log := stampOp(base(), "o0b", planning.SprintDrafts(planning.Sprint{}, sprint1())...)
	log = stampOp(log, "o0c", planning.ReleaseDrafts(planning.Release{}, r)...)
	log = stampOp(log, "o1", planning.DeleteSprint("S1"), planning.DeleteRelease("R1"))
	p := planning.FromLog(log)
	if p.Sprints[0].Deleted == nil || p.Sprints[0].Deleted.Op != "o1" || !p.Sprints[0].Deleted.Root || p.Sprints[0].Deleted.At.IsZero() {
		t.Fatalf("Deleted = %+v", p.Sprints[0].Deleted)
	}
	if p.Releases[0].Deleted == nil || p.Releases[0].Deleted.Op != "o1" {
		t.Fatalf("Deleted = %+v", p.Releases[0].Deleted)
	}
	if len(p.LiveSprints()) != 0 || len(p.LiveReleases()) != 0 {
		t.Fatal("削除済みは Live に出ない")
	}
	if _, ok := p.UsableSprint("S1"); ok {
		t.Fatal("削除済みは使えない")
	}
	if _, ok := p.UsableRelease("R1"); ok {
		t.Fatal("削除済みは使えない")
	}
	log = stampOp(log, "o2", change.NewRestore("sprints/S1", "o1"), change.NewRestore("releases/R1", "o1"))
	p = planning.FromLog(log)
	if p.Sprints[0].Deleted != nil || p.Releases[0].Deleted != nil {
		t.Fatal("restore で Deleted は nil に戻る")
	}
	if s, ok := p.UsableSprint("S1"); !ok || s.Name != "Sprint 1" {
		t.Fatalf("UsableSprint = %+v %v", s, ok)
	}
	if _, ok := p.UsableRelease("R1"); !ok {
		t.Fatal("復元したら使える")
	}
	if _, ok := p.UsableSprint("none"); ok {
		t.Fatal("ないものは使えない")
	}
}

func TestFromLogClose(t *testing.T) {
	log := stampOp(base(), "o1", planning.SprintDrafts(planning.Sprint{}, sprint1())...)
	c := planning.Confirmed{Committed: planning.Totals{Points: 8, Count: 3}, Completed: planning.Totals{Points: 5, Count: 2}}
	closed := stampOp(log, "o2", planning.CloseDraft("S1", c))
	if got := planning.FromLog(closed).Sprints[0].Confirmed; got == nil || *got != c {
		t.Fatalf("Confirmed = %+v", got)
	}
	// prev でつながらない close の行が 2 つ(同時の完了)。at があとの行の値。
	c2 := planning.Confirmed{Committed: planning.Totals{Points: 9, Count: 4}}
	a := stampOp(log, "oa", planning.CloseDraft("S1", c))
	b := stampOp(log, "ob", planning.CloseDraft("S1", c2))
	later := b[len(log)]
	later.At = later.At.Add(time.Hour)
	merged := append(append([]change.Record{}, a...), later)
	if got := planning.FromLog(merged).Sprints[0].Confirmed; got == nil || *got != c2 {
		t.Fatalf("あとの行の値: %+v", got)
	}
}

func TestPlanFieldsWorkWithRepair(t *testing.T) {
	log := stampOp(base(), "o1", planning.SprintDrafts(planning.Sprint{}, sprint1())...)
	if got := workspace.NameOfDeleted(stampOp(log, "o2", planning.DeleteSprint("S1")), "sprints/S1"); got != "Sprint 1" {
		t.Fatalf("NameOfDeleted = %q", got)
	}
	log = stampOp(log, "o2", planning.DeleteSprint("S1"))
	del := log[len(log)-1]
	log = stampOp(log, "o3", change.NewSet("sprints/S1/name", "Sprint 1", "x"))
	set := log[len(log)-1]
	facts := repair.Facts{Ours: map[change.ID]bool{del.ID: true}, Theirs: map[change.ID]bool{set.ID: true}}
	got := repair.Overlaps(log, facts, nil)
	if len(got) != 1 || got[0] != (repair.Overlap{Op: "o2", F: "sprints/S1"}) {
		t.Fatalf("Overlaps = %+v", got)
	}
	undo := change.Undo(log, "o2")
	if len(undo) != 1 || undo[0].T != change.Restore || undo[0].F != "sprints/S1" {
		t.Fatalf("Undo = %+v", undo)
	}
}
