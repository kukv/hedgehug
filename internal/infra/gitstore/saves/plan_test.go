package saves

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// planMerge は、スプリント S1 の行を片側で取り除いて purged.jsonl に足し、もう片側のログを side にした合わせの
// Pending の結果(合わせが渡されなければ false)。
func planMerge(t *testing.T, side []change.Record) (port.ReceivedSave, bool) {
	t.Helper()
	const purged = `{"id":"P1","at":"2026-11-12T09:00:00+09:00","entity":"sprint","target":"S1"}` + "\n"
	keep := rec("R0", "OP0", t0, change.Set, "title")
	create := rec("R1", "OP0", t0, change.Set, "sprints/S1/name")
	r := newRepo(t)
	j, known := judged(t)
	c0 := r.save(map[string]string{wsLog: logOf(t, keep, create)}, "taro", t0, "OP0")
	drop := r.save(map[string]string{wsLog: logOf(t, keep), "purged.jsonl": purged}, "taro", t0.Add(time.Minute), "OP1")
	r.reset(c0)
	other := r.save(map[string]string{wsLog: logOf(t, side...)}, "hanako", t0.Add(2*time.Minute), "OP2")
	m := r.save(map[string]string{wsLog: logOf(t, keep), "purged.jsonl": purged}, BotName, t0.Add(3*time.Minute), "", drop, other)
	r.mark(j, c0)
	s, ok := pending(t, r, m, m, known)[m.String()]
	return s, ok
}

// 片側でスプリント S1 の行を取り除いて purged.jsonl に足し、もう片側で S1 の名前を変えた合わせは、DroppedPlan に
// {W1, S1}(New は名前の 1 行)を持つ。
func TestDroppedPlanInMerge(t *testing.T) {
	keep := rec("R0", "OP0", t0, change.Set, "title")
	create := rec("R1", "OP0", t0, change.Set, "sprints/S1/name")
	rename := rec("R2", "OP2", t0.Add(2*time.Minute), change.Set, "sprints/S1/name")
	s, ok := planMerge(t, []change.Record{keep, create, rename})
	if !ok || !s.Merge || len(s.DroppedPlan) != 1 {
		t.Fatalf("merge = %+v, want one DroppedPlan", s)
	}
	d := s.DroppedPlan[0]
	if d.Workspace != "01WS000000000000000000001" || d.Target != "S1" || len(d.Log) != 2 || len(d.New) != 1 || d.New[0].ID != "R2" {
		t.Errorf("DroppedPlan = %+v", d)
	}
}

// S1 の名前を変えていなければ、合わせを渡さない。
func TestDroppedPlanNotRenamed(t *testing.T) {
	keep := rec("R0", "OP0", t0, change.Set, "title")
	create := rec("R1", "OP0", t0, change.Set, "sprints/S1/name")
	other := rec("R3", "OP2", t0.Add(2*time.Minute), change.Set, "title")
	if s, ok := planMerge(t, []change.Record{keep, create, other}); ok {
		t.Errorf("merge = %+v, want none", s)
	}
}
