package repairing_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/member"
)

func issue(repo string, n int) itemlink.Ref {
	return itemlink.Ref{Kind: itemlink.Issue, Repo: repo, Number: n}
}

var (
	web5 = issue("kukv/web", 5)
	web9 = issue("kukv/web", 9)
	epic = change.Ref{Kind: change.KindItem, ID: apptest.SampleEpic}
)

// link は、アイテム id に Issue r をリンクする行を actor で書く。
func (e *env) link(actor change.Actor, id item.ID, r itemlink.Ref) map[change.ID]bool {
	e.t.Helper()
	set := itemlink.FromLog(e.logOf(change.Ref{Kind: change.KindItem, ID: id}))
	return e.write(actor, func(tx port.Tx) error {
		return tx.WriteItems().Append(id, set.Attach(r, itemlink.Values{Title: "t", State: itemlink.Open})...)
	})
}

// links は、アイテム id のいまのリンク。
func (e *env) links(id item.ID) []itemlink.Ref {
	e.t.Helper()
	var out []itemlink.Ref
	for _, l := range itemlink.FromLog(e.logOf(change.Ref{Kind: change.KindItem, ID: id})).Links {
		out = append(out, l.Ref)
	}
	return out
}

func wantLinks(t *testing.T, got []itemlink.Ref, want ...itemlink.Ref) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("links = %v, want %v", got, want)
	}
}

// 1 つの Issue に 2 つのアイテム: あとから同期した(自分の側の)方のリンクを外す。外した行は remove links の 1 行。
func TestUnlinkIssuesTwoItems(t *testing.T) {
	e := newEnv(t)
	theirs := e.link(hanako, apptest.SampleEpic, web5)
	ours := e.link(taro, apptest.SampleTask, web5)
	wantOps(t, e.run(ours, theirs), "HH-2 の kukv/web#5 のリンクを解除")
	wantLinks(t, e.links(apptest.SampleEpic), web5)
	wantLinks(t, e.links(apptest.SampleTask))
	log := e.logOf(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
	if last := log[len(log)-1]; last.T != change.Remove || last.F != "links" {
		t.Errorf("last row = %+v, want remove links", last)
	}
}

// 1 つのアイテムに 2 つの Issue: remote の側の #5 を残し、自分の側の #9 を外す。
func TestUnlinkIssuesOneItem(t *testing.T) {
	e := newEnv(t)
	theirs := e.link(hanako, apptest.SampleEpic, web5)
	ours := e.link(taro, apptest.SampleEpic, web9)
	wantOps(t, e.run(ours, theirs), "HH-1 の kukv/web#9 のリンクを解除")
	wantLinks(t, e.links(apptest.SampleEpic), web5)
}

// 削除済みのアイテムのリンクは数えない。
func TestUnlinkIssuesSkipsDeleted(t *testing.T) {
	e := newEnv(t)
	theirs := e.link(hanako, apptest.SampleTask, web5)
	e.appendItem(taro, apptest.SampleTask, change.NewDeleteEntity(true))
	ours := e.link(taro, apptest.SampleEpic, web5)
	wantOps(t, e.run(ours, theirs))
	wantLinks(t, e.links(apptest.SampleEpic), web5)
}

// 共通の祖先で HH-1 が持つ #5 を、自分の側で外して付け直したら、付け直した行は自分の側なので HH-1 の方を外す(C5c2-3)。
func TestUnlinkIssuesRelinked(t *testing.T) {
	e := newEnv(t)
	e.link(taro, apptest.SampleEpic, web5)
	set := itemlink.FromLog(e.logOf(epic))
	l, _ := set.Find(web5)
	ours := e.write(taro, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleEpic, set.Detach(l)...)
	})
	ours = union(ours, e.link(taro, apptest.SampleEpic, web5))
	theirs := e.link(hanako, apptest.SampleTask, web5)
	wantOps(t, e.run(ours, theirs), "HH-1 の kukv/web#5 のリンクを解除")
	wantLinks(t, e.links(apptest.SampleTask), web5)
}

// 改名を追った行は、リンクした人と時刻を変えない(C5c2-17): 外すのは、あとからリンクした HH-2 の方。
func TestUnlinkIssuesRenamed(t *testing.T) {
	e := newEnv(t)
	e.link(taro, apptest.SampleEpic, web5)
	moved := issue("kukv/web2", 5)
	set := itemlink.FromLog(e.logOf(epic))
	l, _ := set.Find(web5)
	ours := e.write(byGitHub, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleEpic, set.Rename(l, moved.Repo, itemlink.Values{Title: "t", State: itemlink.Open})...)
	})
	theirs := e.link(hanako, apptest.SampleTask, moved)
	wantOps(t, e.run(ours, theirs), "HH-2 の kukv/web2#5 のリンクを解除")
	wantLinks(t, e.links(apptest.SampleEpic), moved)
}

// logins は、メンバーの一覧のログイン名。
func (e *env) logins() []string {
	e.t.Helper()
	var out []string
	err := e.s.Read(e.ctx, store, func(r port.Reader) error {
		ms, err := r.Members().List()
		for _, m := range ms {
			out = append(out, m.Login)
		}
		return err
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return out
}

func (e *env) join(actor change.Actor, login string) map[change.ID]bool {
	return e.write(actor, func(tx port.Tx) error { return tx.WriteMembers().Append(member.Join(login)) })
}

// 大文字小文字だけが違う綴りは、自分の側の綴りを外す。担当者の綴りには触れない。
func TestMergeMembers(t *testing.T) {
	e := newEnv(t)
	e.appendItem(taro, apptest.SampleTask, change.NewSet("assignee", nil, "JIRO"))
	theirs := e.join(hanako, "Jiro")
	ours := e.join(taro, "jiro")
	wantOps(t, e.run(ours, theirs), "メンバー Jiro の重複を統合")
	log := e.logOf(change.Ref{Kind: change.KindMembers})
	if last := log[len(log)-1]; last.T != change.Remove || last.F != "members" || string(last.From) != `"jiro"` {
		t.Errorf("last row = %+v, want remove members \"jiro\"", last)
	}
	got := e.logins()
	if slices.Contains(got, "jiro") || !slices.Contains(got, "Jiro") {
		t.Errorf("members = %v, want only Jiro", got)
	}
	if got := e.item(apptest.SampleTask).Assignee; got != "JIRO" {
		t.Errorf("assignee = %q, want JIRO", got)
	}
}

// 2 台が同じ綴りを足したときは何も書かない。
func TestMergeMembersSameSpelling(t *testing.T) {
	e := newEnv(t)
	theirs := e.join(hanako, "jiro")
	ours := e.join(taro, "jiro")
	wantOps(t, e.run(ours, theirs))
}

// 1 回の Run で、名前・Issue のリンク・メンバーの順に直す(C5c2-7)。
func TestDuplicatesStepsOrder(t *testing.T) {
	e := newEnv(t)
	theirs := union(e.newWorkspace(devWS2, "開発", "DEV"), e.link(hanako, apptest.SampleEpic, web5), e.join(hanako, "Jiro"))
	ours := union(e.newWorkspace(devWS1, "開発", "WEB"), e.link(taro, apptest.SampleTask, web5), e.join(taro, "jiro"))
	wantOps(t, e.run(ours, theirs), "ワークスペース「開発」を「開発 (2)」に変更", "HH-2 の kukv/web#5 のリンクを解除", "メンバー Jiro の重複を統合")
}
