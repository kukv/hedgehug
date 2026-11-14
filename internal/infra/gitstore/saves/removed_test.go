package saves

import (
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// recBy は、実行者 actor の行。
func recBy(id, op string, at time.Time, actor change.Actor, f string) change.Record {
	return change.Record{ID: change.ID(id), Op: change.ID(op), At: at, By: actor.By(), Actor: actor, T: change.Set, F: f}
}

var (
	syncActor = change.Actor{Kind: change.ActorSync}
	autoActor = change.Actor{Kind: change.ActorAutomation, Login: "taro", Rule: "01RULE0000000000000000001"}
	itemRef   = change.Ref{Kind: change.KindItem, ID: task}
)

// pending は、Pending を呼んで、印から保存を引けるようにする。
func pending(t *testing.T, r *testRepo, theirs, head plumbing.Hash, known func(port.SaveKey) (port.Match, error)) map[string]port.ReceivedSave {
	t.Helper()
	got, err := Pending(r.repo, theirs, head, known)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]port.ReceivedSave{}
	for _, s := range got {
		out[s.Key.Version] = s
	}
	return out
}

// bot の作者で、足した行がすべて実行者 sync の保存は修復。実行者 automation の行や、人の作者の保存は修復でない。
func TestRepairSaveFlag(t *testing.T) {
	r := newRepo(t)
	j, known := judged(t)
	create := rec("R1", "OP0", t0, change.Create, "")
	fix := recBy("R2", "OP1", t0.Add(time.Minute), syncActor, "rank")
	auto := recBy("R3", "OP2", t0.Add(2*time.Minute), autoActor, "status")
	user := rec("R4", "OP3", t0.Add(3*time.Minute), change.Set, "title")
	c0 := r.save(map[string]string{itemLog: logOf(t, create)}, "taro", t0, "OP0")
	s1 := r.save(map[string]string{itemLog: logOf(t, create, fix)}, BotName, t0.Add(time.Minute), "OP1")
	s2 := r.save(map[string]string{itemLog: logOf(t, create, fix, auto)}, BotName, t0.Add(2*time.Minute), "OP2")
	s3 := r.save(map[string]string{itemLog: logOf(t, create, fix, auto, user)}, "taro", t0.Add(3*time.Minute), "OP3")
	r.mark(j, c0)
	got := pending(t, r, s3, s3, known)
	if len(got) != 3 {
		t.Fatalf("Pending = %d saves, want 3", len(got))
	}
	for h, want := range map[plumbing.Hash]bool{s1: true, s2: false, s3: false} {
		s := got[h.String()]
		if s.Repair != want || s.Merge {
			t.Errorf("save %s: Repair %v Merge %v, want Repair %v", h, s.Repair, s.Merge, want)
		}
		if len(s.Entities) != 1 || len(s.Removed) != 0 {
			t.Errorf("save %s: entities %+v removed %+v", h, s.Entities, s.Removed)
		}
	}
}

// アイテムのフォルダを消した修復の保存は、Removed にそのアイテムと消える前のログを持つ。New は空。
func TestRemovedOnRepairPurge(t *testing.T) {
	r := newRepo(t)
	j, known := judged(t)
	create := rec("R1", "OP0", t0, change.Create, "")
	assign := rec("R2", "OP1", t0.Add(time.Minute), change.Set, "assignee")
	c0 := r.save(map[string]string{itemLog: logOf(t, create, assign), itemDesc: "本文"}, "taro", t0, "OP0")
	s1 := r.save(map[string]string{itemLog: "", itemDesc: "", "purged.jsonl": "{}\n"}, BotName, t0.Add(time.Minute), "OP1")
	r.mark(j, c0)
	s, ok := pending(t, r, s1, s1, known)[s1.String()]
	if !ok || !s.Repair || s.Merge {
		t.Fatalf("save = %+v, want a repair", s)
	}
	if len(s.Removed) != 1 {
		t.Fatalf("removed = %+v, want the item", s.Removed)
	}
	e := s.Removed[0]
	if e.Ref != itemRef || len(e.Log) != 2 || e.Log[1].ID != "R2" || len(e.New) != 0 {
		t.Errorf("removed = %+v", e)
	}
}

// mergeRemoving は、c0 から、片側でアイテムのフォルダを消し(人の保存)、もう片側で行を足した(足す行がなければ
// 何も変えない)2 つの保存を作り、合わせる。合わせの結果にはフォルダがない。合わせを返す。
func mergeRemoving(t *testing.T, r *testRepo, c0 plumbing.Hash, added []change.Record) plumbing.Hash {
	t.Helper()
	create := rec("R1", "OP0", t0, change.Create, "")
	purge := r.save(map[string]string{itemLog: "", "purged.jsonl": "{}\n"}, "taro", t0.Add(time.Minute), "OP1")
	r.reset(c0)
	files := map[string]string{"members/log.jsonl": logOf(t, rec("M1", "OP2", t0, change.Create, ""))}
	if len(added) > 0 {
		files[itemLog] = logOf(t, append([]change.Record{create}, added...)...)
	}
	other := r.save(files, "hanako", t0.Add(2*time.Minute), "OP2")
	return r.save(map[string]string{itemLog: "", "purged.jsonl": "{}\n"}, BotName, t0.Add(3*time.Minute), "", purge, other)
}

// 片側でフォルダを消し、もう片側で 1 行足した合わせは、Merge で、Removed の New がその 1 行。
// 足していない側だけなら、合わせは返さない。
func TestRemovedInMerge(t *testing.T) {
	create := rec("R1", "OP0", t0, change.Create, "")
	assign := rec("R2", "OP2", t0.Add(2*time.Minute), change.Set, "assignee")
	t.Run("added", func(t *testing.T) {
		r := newRepo(t)
		j, known := judged(t)
		c0 := r.save(map[string]string{itemLog: logOf(t, create)}, "taro", t0, "OP0")
		m := mergeRemoving(t, r, c0, []change.Record{assign})
		r.mark(j, c0)
		s, ok := pending(t, r, m, m, known)[m.String()]
		if !ok || !s.Merge || s.Repair || len(s.Entities) != 0 || s.Key.Op != "" {
			t.Fatalf("merge = %+v, want a merge without entities", s)
		}
		if len(s.Removed) != 1 {
			t.Fatalf("removed = %+v, want the item", s.Removed)
		}
		e := s.Removed[0]
		if e.Ref != itemRef || len(e.Log) != 2 || len(e.New) != 1 || e.New[0].ID != "R2" {
			t.Errorf("removed = %+v", e)
		}
	})
	t.Run("not added", func(t *testing.T) {
		r := newRepo(t)
		j, known := judged(t)
		c0 := r.save(map[string]string{itemLog: logOf(t, create)}, "taro", t0, "OP0")
		m := mergeRemoving(t, r, c0, nil)
		r.mark(j, c0)
		if s, ok := pending(t, r, m, m, known)[m.String()]; ok {
			t.Errorf("merge = %+v, want none", s)
		}
	})
}

// 自分の先端にだけある修復の保存は渡し、自分の人の保存は渡さない。受け取った保存は渡す。古い順。
func TestOwnRepairPassed(t *testing.T) {
	r := newRepo(t)
	j, known := judged(t)
	create := rec("R1", "OP0", t0, change.Create, "")
	theirs := rec("R2", "OP1", t0.Add(time.Minute), change.Set, "assignee")
	mine := rec("R3", "OP2", t0.Add(2*time.Minute), change.Set, "title")
	fix := recBy("R4", "OP3", t0.Add(4*time.Minute), syncActor, "rank")
	c0 := r.save(map[string]string{itemLog: logOf(t, create)}, "taro", t0, "OP0")
	s1 := r.save(map[string]string{itemLog: logOf(t, create, theirs)}, "hanako", t0.Add(time.Minute), "OP1")
	r.reset(c0)
	s2 := r.save(map[string]string{itemLog: logOf(t, create, mine)}, "taro", t0.Add(2*time.Minute), "OP2")
	m := r.save(map[string]string{itemLog: logOf(t, create, theirs, mine)}, BotName, t0.Add(3*time.Minute), "", s2, s1)
	s3 := r.save(map[string]string{itemLog: logOf(t, create, theirs, mine, fix)}, BotName, t0.Add(4*time.Minute), "OP3")
	r.mark(j, c0)
	got, err := Pending(r.repo, s1, s3, known)
	if err != nil {
		t.Fatal(err)
	}
	if g, w := versions(got), want(s1, s3); strings.Join(g, ",") != strings.Join(w, ",") {
		t.Errorf("Pending = %v, want %v (not own %s, not merge %s)", g, w, s2, m)
	}
	if len(got) == 2 && (got[0].Repair || !got[1].Repair || len(got[1].Entities) != 1) {
		t.Errorf("saves = %+v", got)
	}
}
