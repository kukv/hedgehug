package noticing_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func issueRef(repo string, n int) itemlink.Ref {
	return itemlink.Ref{Kind: itemlink.Issue, Repo: repo, Number: n}
}

var (
	web5 = issueRef("kukv/web", 5)
	web9 = issueRef("kukv/web", 9)
)

// linkTo は、by がアイテム ref に r をリンクし、その add の行の ID を返す。
func linkTo(f *fixture, by string, ref change.Ref, r itemlink.Ref) string {
	f.t.Helper()
	set := itemlink.FromLog(f.logOfRef(ref))
	f.save(user(by), appendRef(ref, set.Attach(r, itemlink.Values{Title: "t", State: itemlink.Open})...))
	return addRowOf(f, ref, r)
}

// addRowOf は、ref のログで r の最後の add links の行の ID。
func addRowOf(f *fixture, ref change.Ref, r itemlink.Ref) string {
	f.t.Helper()
	var id change.ID
	for _, c := range f.logOfRef(ref) {
		if c.T == change.Add && c.F == "links" && strings.Contains(string(c.To), r.Repo) && strings.Contains(string(c.To), `"number":`+strconv.Itoa(r.Number)) {
			id = c.ID
		}
	}
	return string(id)
}

// unlinkRepair は、ref のアイテムから Issue r のリンクを外した修復の保存(同期の修復の実行者)。
func unlinkRepair(f *fixture, ref change.Ref, r itemlink.Ref) port.ReceivedSave {
	f.t.Helper()
	return f.repairSave(appendRef(ref, itemlink.Set{}.Detach(itemlink.Link{Ref: r})...))
}

func issueSource(ref change.Ref, r itemlink.Ref, add string) string {
	return "repair:issue:" + string(ref.ID) + ":" + r.Key() + ":" + add
}

func TestIssueUnlinksTwoItems(t *testing.T) {
	f := setup(t)
	linkTo(f, "taro", epicRef, web5)
	add := linkTo(f, "hanako", taskRef, web5)
	f.judge(unlinkRepair(f, taskRef, web5))
	n := f.wantOne()
	if want := "Issue kukv/web#5 は HH-1 にリンクされているため、HH-2 からリンクを外しました"; n.Text != want {
		t.Errorf("text = %q, want %q", n.Text, want)
	}
	if n.Item != apptest.SampleTask || n.Display != "HH-2" || n.Workspace != apptest.SampleWorkspace || n.Kind != port.NoticeRepair {
		t.Errorf("notice = %+v", n)
	}
	if want := issueSource(taskRef, web5, add); n.Source != want {
		t.Errorf("source = %q, want %q", n.Source, want)
	}
}

func TestIssueUnlinksOneItem(t *testing.T) {
	f := setup(t)
	linkTo(f, "taro", epicRef, web5)
	linkTo(f, "hanako", epicRef, web9)
	f.judge(unlinkRepair(f, epicRef, web9))
	n := f.wantOne()
	if want := "HH-1 には Issue kukv/web#5 がリンクされているため、Issue kukv/web#9 のリンクを外しました"; n.Text != want {
		t.Errorf("text = %q, want %q", n.Text, want)
	}
	if n.Item != apptest.SampleEpic || n.Display != "HH-1" {
		t.Errorf("notice = %+v", n)
	}
}

// HH-1 の X を外し、HH-2 が X を、HH-1 が Y を持つ: (a) の本文にする。
func TestIssueUnlinksPrefersOtherItem(t *testing.T) {
	f := setup(t)
	linkTo(f, "taro", taskRef, web5)
	linkTo(f, "taro", epicRef, web9)
	linkTo(f, "hanako", epicRef, web5)
	f.judge(unlinkRepair(f, epicRef, web5))
	if got, want := f.wantOne().Text, "Issue kukv/web#5 は HH-2 にリンクされているため、HH-1 からリンクを外しました"; got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestIssueUnlinksNoHolder(t *testing.T) {
	f := setup(t)
	linkTo(f, "hanako", epicRef, web5)
	f.judge(unlinkRepair(f, epicRef, web5))
	f.wantNone()
}

func TestIssueUnlinksRecipient(t *testing.T) {
	f := setup(t)
	linkTo(f, "hanako", epicRef, web5)
	linkTo(f, "taro", taskRef, web5)
	f.judge(unlinkRepair(f, taskRef, web5))
	f.wantNone()
}

// GitHub の実行者が改名を追って付け直したリンクを外した: リンクした人(hanako)に、hanako の add の行を鍵に作る。
func TestIssueUnlinksRenamed(t *testing.T) {
	f := setup(t)
	old, moved := issueRef("kukv/old", 5), issueRef("kukv/new", 5)
	linkTo(f, "taro", epicRef, moved)
	add := linkTo(f, "hanako", taskRef, old)
	l, _ := itemlink.FromLog(f.logOfRef(taskRef)).Find(old)
	f.save(change.Actor{Kind: change.ActorSync}, appendRef(taskRef, itemlink.Set{}.Rename(l, "kukv/new", itemlink.Values{Title: "t", State: itemlink.Open})...))
	f.judge(unlinkRepair(f, taskRef, moved))
	n := f.wantOne()
	if want := issueSource(taskRef, moved, add); n.Source != want {
		t.Errorf("source = %q, want %q", n.Source, want)
	}
}

// remove の行の prev が、taro と hanako の同じ Ref の add の行を指す: ListRow で勝った方の人にだけ作る。
func TestIssueUnlinksBothSidesAdded(t *testing.T) {
	f := setup(t)
	linkTo(f, "taro", epicRef, web5)
	a := linkTo(f, "taro", taskRef, web5)
	b := linkTo(f, "hanako", taskRef, web5)
	s := unlinkRepair(f, taskRef, web5)
	s.Entities[0].Rows[0].Prev = []change.ID{change.ID(a), change.ID(b)}
	// 勝つ add の行は、この 2 行だけに ListRow を当てて決める(どの機械でも同じ)。
	var adds []change.Record
	for _, c := range f.logOfRef(taskRef) {
		if string(c.ID) == a || string(c.ID) == b {
			adds = append(adds, c)
		}
	}
	if len(adds) != 2 {
		t.Fatalf("adds = %d, want 2", len(adds))
	}
	win, ok := change.ListRow(adds, "links", adds[0].To)
	if !ok {
		t.Fatal("ListRow: no winner")
	}
	winner, loser := "taro", "hanako"
	if win.By == "hanako" {
		winner, loser = "hanako", "taro"
	}
	want := issueSource(taskRef, web5, string(win.ID))
	other := map[change.ID][]change.ID{change.ID(a): nil}
	judgeAs := func(me string) []port.Notice {
		f.t.Helper()
		before := len(f.notices())
		f.svc.Identity = apptest.Identity{Login: me}
		ctx := f.t.Context()
		err := f.svc.Judge(ctx, store, port.Received{Saves: []port.ReceivedSave{s}}, func(fn func(r port.Reader) error) error {
			return f.stores.Read(ctx, store, func(r port.Reader) error { return fn(concurrent{r, other}) })
		})
		if err != nil {
			t.Fatal(err)
		}
		return f.notices()[before:]
	}
	if got := judgeAs(loser); len(got) != 0 {
		t.Errorf("loser %s notices = %+v, want none", loser, got)
	}
	first := judgeAs(winner)
	if len(first) != 1 || first[0].Source != want {
		t.Fatalf("winner %s notices = %+v, want one with source %q", winner, first, want)
	}
	// 別の機械で同じ保存を判定しても、鍵は同じ(重複して通知されない)。
	if again := judgeAs(winner); len(again) > 0 && again[0].Source != want {
		t.Errorf("second judgement source = %q, want %q", again[0].Source, want)
	}
}

func TestIssueUnlinksSkipsOthers(t *testing.T) {
	f := setup(t)
	pr := itemlink.Ref{Kind: itemlink.PR, Repo: "kukv/web", Number: 5}
	linkTo(f, "taro", epicRef, web5)
	linkTo(f, "hanako", taskRef, web5)
	linkTo(f, "hanako", taskRef, pr)
	// 人の保存(修復でない)
	human := f.save(user("hanako"), appendRef(taskRef, itemlink.Set{}.Detach(itemlink.Link{Ref: web5})...))
	f.judge(human)
	f.wantNone()
	// PR のリンクを外した修復
	f.judge(unlinkRepair(f, taskRef, pr))
	f.wantNone()
	// restore の行を含む修復
	f.judge(f.repairSave(appendRef(taskRef, change.NewRestore("x", "01OP0000000000000000000001"), change.NewRemove("links", web5))))
	f.wantNone()
}

// 行の ID が違う同じ修復の保存を 2 つ判定しても、受信箱は 1 行。
func TestIssueUnlinksOnceAcrossMachines(t *testing.T) {
	f := setup(t)
	linkTo(f, "taro", epicRef, web5)
	linkTo(f, "hanako", taskRef, web5)
	s := unlinkRepair(f, taskRef, web5)
	other := unlinkRepair(f, taskRef, web5)
	other.Entities[0].Rows[0].Prev = s.Entities[0].Rows[0].Prev
	f.judge(s)
	f.judge(other)
	if rows := f.rows(); len(rows) != 1 {
		t.Errorf("rows = %+v, want one", rows)
	}
}

// メンバーの統合の保存は、どの種類の判定からも通知を作らない。
func TestMemberMergeNoNotice(t *testing.T) {
	f := setup(t)
	f.save(user("hanako"), func(tx port.Tx) error {
		return tx.WriteMembers().Append(change.NewAdd("members", "jiro"))
	})
	merge := f.repairSave(func(tx port.Tx) error {
		return tx.WriteMembers().Append(change.NewRemove("members", "jiro"))
	})
	f.judge(merge)
	f.wantNone()
}

// Issue のリンクを外した修復の保存 1 つで、受信箱は 1 行(ほかの種類の判定が重ねて作らない)。
func TestIssueUnlinkSingleNotice(t *testing.T) {
	f := setup(t)
	linkTo(f, "taro", epicRef, web5)
	linkTo(f, "hanako", taskRef, web5)
	f.judge(unlinkRepair(f, taskRef, web5))
	if rows := f.rows(); len(rows) != 1 {
		t.Errorf("rows = %+v, want one", rows)
	}
}
