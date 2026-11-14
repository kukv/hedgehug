package saves

import (
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
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

const (
	epicID  item.ID = "01ITEM00000000000000000009"
	childID item.ID = "01ITEM00000000000000000002"
	epicDir         = "items/" + string(epicID) + "/log.jsonl"
	gone    item.ID = "01ITEM00000000000000000008"
	wsLog           = "workspaces/01WS000000000000000000001/log.jsonl"
)

// createUnder は、親 parent(空ならワークスペースの直下)の下にアイテム id を作る taro の行。
func createUnder(t *testing.T, rid string, id, parent item.ID) change.Record {
	t.Helper()
	p := item.Place{Workspace: "01WS000000000000000000001", Number: 1}
	if parent != "" {
		p.Parent = &parent
	}
	r := rec(rid, "OP0", t0, change.Create, "")
	r.To = item.Create(item.Item{ID: id, Place: p, Kind: item.Task, Rank: "V", Title: "t"}).To
	return r
}

// アイテムのフォルダを消した修復の保存は、Removed にそのアイテムと消える前のログを持つ。New は空。
// 親(gone)のフォルダは、前の版にもうない(完全に削除された親の下の修復 (1))。
func TestRemovedOnRepairPurge(t *testing.T) {
	r := newRepo(t)
	j, known := judged(t)
	create := createUnder(t, "R1", task, gone)
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

// 行のない bot の保存が修復 (1) なのは、消したアイテムの祖先のどれかのフォルダが、前の版にもうないときだけ。
// Epic とその下の子を一緒に消した保存(保持期間のあとの完全な削除など)は修復でない。祖先を辿るので、親が
// まだあっても、その上が消えていれば修復(修復 (1) は内部 ID の順に 1 件ずつ消すので、孫が先に消えることがある)。
func TestRowlessBotSaveRepair(t *testing.T) {
	childLog := "items/" + string(childID) + "/log.jsonl"
	for name, c := range map[string]struct {
		epicParent item.ID
		remove     []string
		want       bool
	}{
		"親と一緒に消した": {"", []string{epicDir, childLog, itemLog}, false},
		"祖先が前にない":  {gone, []string{itemLog}, true},
	} {
		t.Run(name, func(t *testing.T) {
			r := newRepo(t)
			j, known := judged(t)
			c0 := r.save(map[string]string{
				epicDir:  logOf(t, createUnder(t, "E1", epicID, c.epicParent)),
				childLog: logOf(t, createUnder(t, "C1", childID, epicID)),
				itemLog:  logOf(t, createUnder(t, "R1", task, childID)),
				wsLog:    logOf(t, rec("W1", "OP0", t0, change.Create, "")),
			}, "taro", t0, "OP0")
			files := map[string]string{"purged.jsonl": "{}\n"}
			for _, f := range c.remove {
				files[f] = ""
			}
			s1 := r.save(files, BotName, t0.Add(time.Minute), "OP1")
			r.mark(j, c0)
			s, ok := pending(t, r, s1, s1, known)[s1.String()]
			if !ok || s.Repair != c.want {
				t.Errorf("passed %v, save = %+v, want repair %v", ok, s, c.want)
			}
		})
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

// entityDelete は、taro のエンティティの削除の行。root は、利用者が選んだものか。
func entityDelete(id, op string, at time.Time, root bool) change.Record {
	r := rec(id, op, at, change.Delete, change.DeletedField)
	r.Root = &root
	return r
}

// 片側が親と子を同じ op で削除し(子は root: false)、もう片側で hanako が子のタイトルを変えた合わせは、Concurrent に
// 子の Ref、Deleted に root: false の削除、Changed にタイトルの行を持つ。子を変えていなければ、合わせは渡さない。
// 変えた側が 1 つ目の親の合わせ(変えたマシンが合わせた)と、2 つ目の親の合わせ(削除したマシンが合わせた)の両方。
func TestConcurrentInMerge(t *testing.T) {
	for _, changerFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "changer merges", false: "deleter merges"}[changerFirst], func(t *testing.T) {
			concurrentInMerge(t, changerFirst)
		})
	}
}

// ordered2 は、first なら a, b、そうでなければ b, a。
func ordered2(first bool, a, b plumbing.Hash) []plumbing.Hash {
	if first {
		return []plumbing.Hash{a, b}
	}
	return []plumbing.Hash{b, a}
}

// concurrentInMerge は、TestConcurrentInMerge の 1 つの向き。changerFirst なら、変えた側の保存を 1 つ目の親にする。
func concurrentInMerge(t *testing.T, changerFirst bool) {
	const epicLog = "items/01ITEM00000000000000000009/log.jsonl"
	create := rec("R1", "OP0", t0, change.Create, "")
	epic := rec("E1", "OP0", t0, change.Create, "")
	del := entityDelete("R2", "OP1", t0.Add(time.Minute), false)
	epicDel := entityDelete("E2", "OP1", t0.Add(time.Minute), true)
	title := recBy("R3", "OP2", t0.Add(2*time.Minute), change.User("hanako"), "title")
	for _, changed := range []bool{true, false} {
		r := newRepo(t)
		j, known := judged(t)
		c0 := r.save(map[string]string{itemLog: logOf(t, create), epicLog: logOf(t, epic)}, "taro", t0, "OP0")
		s1 := r.save(map[string]string{itemLog: logOf(t, create, del), epicLog: logOf(t, epic, epicDel)}, "taro", t0.Add(time.Minute), "OP1")
		r.reset(c0)
		files := map[string]string{"members/log.jsonl": logOf(t, rec("M1", "OP2", t0, change.Create, ""))}
		merged := []change.Record{create, del}
		if changed {
			files[itemLog] = logOf(t, create, title)
			merged = append(merged, title)
		}
		s2 := r.save(files, "hanako", t0.Add(2*time.Minute), "OP2")
		m := r.save(map[string]string{itemLog: logOf(t, merged...), epicLog: logOf(t, epic, epicDel)}, BotName, t0.Add(3*time.Minute), "", ordered2(changerFirst, s2, s1)...)
		r.mark(j, c0)
		s, ok := pending(t, r, m, m, known)[m.String()]
		if !changed {
			if ok {
				t.Errorf("merge = %+v, want none", s)
			}
			continue
		}
		if !ok || !s.Merge || len(s.Removed) != 0 || len(s.Concurrent) != 1 {
			t.Fatalf("merge = %+v, want one concurrent entity", s)
		}
		c := s.Concurrent[0]
		if c.Ref != itemRef || len(c.Deleted) != 1 || c.Deleted[0].ID != "R2" || len(c.Changed) != 1 || c.Changed[0].ID != "R3" {
			t.Errorf("concurrent = %+v", c)
		}
	}
}

// 合わせの結果から消えた log.jsonl も、両側の版と違う log.jsonl もない合わせは、共通の祖先を探さない(消えたものも、
// 同時の削除と変更もない)。片側だけで変わったログは、合わせの結果がその側の版と同じになる。
func TestMergeNeedsBase(t *testing.T) {
	create := rec("R1", "OP0", t0, change.Create, "")
	other := "items/" + string(childID) + "/log.jsonl"
	ours := rec("R2", "OP1", t0.Add(time.Minute), change.Set, "title")
	theirs := rec("R3", "OP2", t0.Add(2*time.Minute), change.Set, "assignee")
	for name, c := range map[string]struct {
		theirs map[string]string
		merged map[string]string
		want   bool
	}{
		"別のログ":  {map[string]string{other: logOf(t, theirs)}, map[string]string{other: logOf(t, theirs)}, false},
		"同じログ":  {map[string]string{itemLog: logOf(t, create, theirs)}, map[string]string{itemLog: logOf(t, create, ours, theirs)}, true},
		"消えたログ": {map[string]string{itemLog: "", "purged.jsonl": "{}\n"}, map[string]string{itemLog: ""}, true},
	} {
		t.Run(name, func(t *testing.T) {
			r := newRepo(t)
			c0 := r.save(map[string]string{itemLog: logOf(t, create)}, "taro", t0, "OP0")
			s1 := r.save(map[string]string{itemLog: logOf(t, create, ours)}, "taro", t0.Add(time.Minute), "OP1")
			r.reset(c0)
			s2 := r.save(c.theirs, "hanako", t0.Add(2*time.Minute), "OP2")
			r.reset(s1)
			m := r.save(c.merged, BotName, t0.Add(3*time.Minute), "", s1, s2)
			c, ok := r.commit(m), c.want
			got, err := needsBase(c)
			if err != nil || got != ok {
				t.Errorf("needsBase = %v, %v, want %v", got, err, ok)
			}
		})
	}
}

// 削除を取り消した修復の保存は、前のいちばん近い合わせ(間の修復の保存を通る)の両側の、共通の祖先にない行を
// Sides に持つ。月曜の hanako の変更は 1 つ目の親の側、水曜の jiro の変更と木曜の taro の削除は 2 つ目の親の側。
func TestRepairSaveSides(t *testing.T) {
	r := newRepo(t)
	j, known := judged(t)
	create := rec("R1", "OP0", t0, change.Create, "")
	hanako := recBy("H1", "OP1", t0.Add(time.Minute), change.User("hanako"), "title")
	jiro := recBy("J1", "OP2", t0.Add(2*time.Minute), change.User("jiro"), "status")
	del := entityDelete("D1", "OP3", t0.Add(3*time.Minute), true)
	fix := recBy("F1", "OP4", t0.Add(4*time.Minute), syncActor, "rank")
	undo := recBy("U1", "OP5", t0.Add(5*time.Minute), syncActor, "")
	undo.T, undo.To = change.Restore, []byte(`"OP3"`)
	c0 := r.save(map[string]string{itemLog: logOf(t, create)}, "taro", t0, "OP0")
	s1 := r.save(map[string]string{itemLog: logOf(t, create, hanako)}, "hanako", t0.Add(time.Minute), "OP1")
	r.reset(c0)
	r.save(map[string]string{itemLog: logOf(t, create, jiro)}, "jiro", t0.Add(2*time.Minute), "OP2")
	s3 := r.save(map[string]string{itemLog: logOf(t, create, jiro, del)}, "taro", t0.Add(3*time.Minute), "OP3")
	r.reset(s1)
	merged := []change.Record{create, hanako, jiro, del}
	m := r.save(map[string]string{itemLog: logOf(t, merged...)}, BotName, t0.Add(4*time.Minute), "", s1, s3)
	r.save(map[string]string{itemLog: logOf(t, append(merged, fix)...)}, BotName, t0.Add(4*time.Minute), "OP4")
	rs := r.save(map[string]string{itemLog: logOf(t, append(merged, fix, undo)...)}, BotName, t0.Add(5*time.Minute), "OP5")
	r.mark(j, c0)
	s, ok := pending(t, r, s3, rs, known)[rs.String()]
	if !ok || !s.Repair {
		t.Fatalf("save = %+v, want a repair (merge %s)", s, m)
	}
	f := s.Sides
	if f.Side("H1") != 1 || f.Side("D1") != 2 || f.Side("J1") != 2 || f.Side("R1") != 0 || f.Side("F1") != 0 {
		t.Errorf("sides = %+v", f)
	}
}

// rulesMerge は、プロジェクト P1 のルールの行を片側で取り除いて purged.jsonl に足し、もう片側で row を足した合わせの
// Pending の結果(合わせが渡されなければ false)。共通の祖先は、専用ルール RX(scope が P1)と P1 の使わないステータスを
// 持つ。kept なら、row は合わせの結果に残る。
func rulesMerge(t *testing.T, row change.Record, kept bool) (port.ReceivedSave, bool) {
	t.Helper()
	const purged = `{"id":"PG1","at":"2026-11-12T09:00:00+09:00","entity":"project","target":"P1"}` + "\n"
	create := rec("R0", "OP0", t0, change.Create, "") // scope を読むのに、ワークスペースの作成の行が要る
	create.To = []byte(`{}`)
	scope := rec("R1", "OP0", t0, change.Set, "rules/RX/scope")
	scope.To = []byte(`"P1"`)
	hidden := rec("R2", "OP0", t0, change.Add, "project_rules/P1/hidden")
	r := newRepo(t)
	j, known := judged(t)
	c0 := r.save(map[string]string{wsLog: logOf(t, create, scope, hidden)}, "taro", t0, "OP0")
	drop := r.save(map[string]string{wsLog: logOf(t, create), "purged.jsonl": purged}, "taro", t0.Add(time.Minute), "OP1")
	r.reset(c0)
	other := r.save(map[string]string{wsLog: logOf(t, create, scope, hidden, row)}, "hanako", t0.Add(2*time.Minute), "OP2")
	merged := []change.Record{create}
	if kept {
		merged = append(merged, row)
	}
	m := r.save(map[string]string{wsLog: logOf(t, merged...), "purged.jsonl": purged}, BotName, t0.Add(3*time.Minute), "", drop, other)
	r.mark(j, c0)
	s, ok := pending(t, r, m, m, known)[m.String()]
	return s, ok
}

// 片側でプロジェクト P1 のルールの行を取り除いて purged.jsonl に足し、もう片側で P1 のルールを変えた合わせは、
// DroppedRules に {W1, P1}(New はその側の共通の祖先にない 1 行)を持つ。専用ルール RX の scope の行は共通の祖先に
// あり、その側のログから読む。
func TestDroppedRulesInMerge(t *testing.T) {
	for name, row := range map[string]change.Record{
		"使わないステータスに足した": rec("R3", "OP2", t0.Add(2*time.Minute), change.Add, "project_rules/P1/hidden"),
		"専用ルールの名前を変えた":  rec("R3", "OP2", t0.Add(2*time.Minute), change.Set, "rules/RX/name"),
	} {
		t.Run(name, func(t *testing.T) {
			s, ok := rulesMerge(t, row, false)
			if !ok || !s.Merge || len(s.DroppedRules) != 1 {
				t.Fatalf("merge = %+v, want one DroppedRules", s)
			}
			d := s.DroppedRules[0]
			if d.Workspace != "01WS000000000000000000001" || d.Project != "P1" || len(d.Log) != 3 || len(d.New) != 1 || d.New[0].ID != "R3" {
				t.Errorf("DroppedRules = %+v", d)
			}
		})
	}
}

// P1 のルールを変えていなければ(共通の祖先にあるルールの行だけが取り除かれた)、合わせを渡さない。
func TestDroppedRulesNotChanged(t *testing.T) {
	if s, ok := rulesMerge(t, rec("R3", "OP2", t0.Add(2*time.Minute), change.Set, "title"), true); ok {
		t.Errorf("merge = %+v, want none", s)
	}
}
