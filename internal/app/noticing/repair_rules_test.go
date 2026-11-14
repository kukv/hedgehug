package noticing_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

const (
	// webP は、この Store にないプロジェクト「Web」(旧 ID HH-P1)。
	webP     project.ID = "01SAMPLEPR000000000000000A"
	reviewSt change.ID  = "01SAMPLEVAL000000000000099"
	ownRule  change.ID  = "01RULE00000000000000000002"
	template change.ID  = "01TEMPLATE0000000000000001"
	movedWeb            = "HH-P1 は 太郎 が team-b へ移動したため、あなたの変更は反映されませんでした。移動先は WEB-P2 です"
)

// wsRules は、by が ds をワークスペース「Web」のログに書く保存。
func wsRules(f *fixture, by string, ds ...change.Draft) {
	f.t.Helper()
	f.save(user(by), func(tx port.Tx) error { return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...) })
}

// wsRepair は、ds をワークスペース「Web」のログに書く同期の修復の保存。
func wsRepair(f *fixture, ds ...change.Draft) port.ReceivedSave {
	f.t.Helper()
	return f.repairSave(func(tx port.Tx) error { return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...) })
}

// movingWeb は、taro(表示名 太郎)が p を Store「team-b」の WEB-P2 へ移した転送記録を書く。
func movingWeb(f *fixture, p project.ID) {
	f.t.Helper()
	movingRecord(f, transfer.Record{
		Entity: transfer.EntityProject, Target: p, Old: "HH-P1", New: "WEB-P2", ToStore: "S2", ToStoreName: "team-b",
	})
}

// hideReview は、hanako が p の使わないステータス「Review」を足し、修復でそれを外す保存を返す。
func hideReview(f *fixture, p project.ID) port.ReceivedSave {
	f.t.Helper()
	wsRules(f, "hanako", change.NewAdd(projectrule.HiddenField(p), reviewSt))
	return wsRepair(f, change.NewRemove(projectrule.HiddenField(p), reviewSt))
}

func TestOrphanRulesMovedStore(t *testing.T) {
	f := setup(t)
	movingWeb(f, webP)
	f.judge(hideReview(f, webP))
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Project: webP, Display: "HH-P1",
		Text: movedWeb, Source: "repair:moved:" + string(webP) + ":" + f.lastTransfer(),
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

// 移動先の Store が表示中なら、修復の保存だけでも、このマシンでの名前を使う。
func TestOrphanRulesShownStoreName(t *testing.T) {
	f := setup(t)
	f.svc.List = apptest.StoreList{Stores: []port.StoreInfo{{ID: store, Name: "main"}, {ID: "S2", Name: "team-local"}}}
	movingWeb(f, webP)
	f.judge(hideReview(f, webP))
	if n := f.wantOne(); n.Text != "HH-P1 は 太郎 が team-local へ移動したため、あなたの変更は反映されませんでした。移動先は WEB-P2 です" {
		t.Errorf("text = %q", n.Text)
	}
}

func TestOrphanRulesPurged(t *testing.T) {
	const p3 project.ID = "01SAMPLEPR000000000000000C"
	f := setup(t)
	f.purge(transfer.EntityProject, p3, "HH-P3")
	f.judge(hideReview(f, p3))
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Project: p3, Display: "HH-P3",
		Text: "HH-P3 は完全に削除されたため、あなたの変更は反映されませんでした", Source: "repair:purged:" + string(p3),
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

// 記録の ID が空なら(手での書き換え)、作らない。
func TestOrphanRulesEmptyDisplay(t *testing.T) {
	f := setup(t)
	f.purge(transfer.EntityProject, webP, "")
	f.judge(hideReview(f, webP))
	f.wantNone()
}

// プロジェクト「アプリ」は同じ Store のワークスペース「Mobile」へ移った(同じ Store の中の転送記録は原因に数えない)。
func TestOrphanRulesSameStoreNoNotice(t *testing.T) {
	f := setup(t)
	if err := apptest.SampleMobile(t.Context(), f.stores, store); err != nil {
		t.Fatal(err)
	}
	movingRecord(f, transfer.Record{
		Entity: transfer.EntityProject, Target: apptest.SampleMobileProject, Old: "HH-P2", New: "MB-P1", ToStore: string(store),
	})
	f.judge(hideReview(f, apptest.SampleMobileProject))
	f.wantNone()
}

func TestOrphanRulesRecipient(t *testing.T) {
	tdField := projectrule.TemplateDefaultField(webP, "Task")
	rule := automation.Rule{ID: ownRule, Name: "完了で閉じる", Trigger: automation.TriggerPRMerged, Action: automation.ActionStatus, Value: string(apptest.SampleStatusDone), Enabled: true, Scope: webP}
	renamed := rule
	renamed.Name = "完了なら閉じる"
	for name, c := range map[string]struct {
		prepare func(f *fixture)
		repair  []change.Draft
		want    bool
	}{
		"外した値を足したのが taro": {func(f *fixture) {
			wsRules(f, "taro", change.NewAdd(projectrule.HiddenField(webP), reviewSt))
		}, []change.Draft{change.NewRemove(projectrule.HiddenField(webP), reviewSt)}, false},
		"削除した専用ルールを最後に保存したのが hanako": {func(f *fixture) {
			wsRules(f, "taro", automation.Drafts(automation.Rule{}, rule)...)
			wsRules(f, "hanako", automation.Drafts(rule, renamed)...)
		}, []change.Draft{automation.DeleteDraft(ownRule)}, true},
		"削除した専用ルールを最後に保存したのが taro": {func(f *fixture) {
			wsRules(f, "hanako", automation.Drafts(automation.Rule{}, rule)...)
			wsRules(f, "taro", automation.Drafts(rule, renamed)...)
		}, []change.Draft{automation.DeleteDraft(ownRule)}, false},
		"null にした既定のテンプレートを最後に選んだのが hanako": {func(f *fixture) {
			wsRules(f, "taro", change.NewSet(tdField, nil, projectrule.None))
			wsRules(f, "hanako", change.NewSet(tdField, projectrule.None, template))
		}, []change.Draft{change.NewSet(tdField, template, nil)}, true},
	} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			movingWeb(f, webP)
			c.prepare(f)
			f.judge(wsRepair(f, c.repair...))
			if got := len(f.notices()) == 1; got != c.want {
				t.Errorf("notices = %+v, want one: %v", f.notices(), c.want)
			}
		})
	}
}

// 外したものを足した人が 2 回あなたでも、プロジェクトごとに 1 件。
func TestOrphanRulesOnePerProject(t *testing.T) {
	f := setup(t)
	movingWeb(f, webP)
	hidden, templates := projectrule.HiddenField(webP), projectrule.TemplatesField(webP)
	wsRules(f, "hanako", change.NewAdd(hidden, reviewSt), change.NewAdd(templates, template))
	f.judge(wsRepair(f, change.NewRemove(hidden, reviewSt), change.NewRemove(templates, template)))
	if n := f.wantOne(); n.Text != movedWeb {
		t.Errorf("text = %q", n.Text)
	}
}

// goneProjectLog は、別の Store「S9」に作ったプロジェクト webP(番号 1)のログ。この Store にはない
// プロジェクトの、消える前のログにする。
func goneProjectLog(f *fixture) []change.Record {
	f.t.Helper()
	const other port.StoreID = "S9"
	f.stores.AddStore(other)
	ref := change.Ref{Kind: change.KindProject, ID: webP}
	web := project.Project{ID: webP, Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 1}, Rank: "W", Title: "Web", Owner: "taro", Status: project.Todo}
	err := f.stores.Write(f.t.Context(), other, port.Operation{Actor: user("taro"), Summary: "テスト"}, func(tx port.Tx) error {
		if err := tx.WriteProjects().Append(webP, project.Create(web)); err != nil {
			return err
		}
		return tx.WriteProjects().Append(webP, change.NewSet("title", "Web", "直した"))
	})
	if err != nil {
		f.t.Fatal(err)
	}
	var log []change.Record
	if err := f.stores.Read(f.t.Context(), other, func(r port.Reader) (err error) {
		log, err = r.Changes().Of(ref)
		return err
	}); err != nil {
		f.t.Fatal(err)
	}
	for j := range log { // タイトルの行は hanako が書いた
		if log[j].F == "title" {
			log[j].By, log[j].Actor = "hanako", user("hanako")
		}
	}
	return log
}

// 合わせで消えた「Web」に hanako が書いた行と、(9) の修復の保存は、同じ鍵なので受信箱は 1 行。
func TestOrphanRulesSharesMovedKey(t *testing.T) {
	f := setup(t)
	log := goneProjectLog(f)
	movingWeb(f, webP)
	merge := port.ReceivedSave{
		Key: port.SaveKey{Version: "merge"}, Merge: true,
		Removed: []port.RemovedEntity{{Ref: change.Ref{Kind: change.KindProject, ID: webP}, Log: log, New: log[len(log)-1:]}},
	}
	f.judge(merge, hideReview(f, webP))
	if rows := f.rows(); len(rows) != 1 || rows[0].Text != movedWeb {
		t.Errorf("rows = %+v, want one", rows)
	}
}

// ワークスペースにあるプロジェクトのルールから値を外した修復の保存(C5b2 の形)は、(9) に数えない。
func TestOrphanRulesSkipsValueRepair(t *testing.T) {
	f := setup(t)
	movingWeb(f, apptest.SampleProject)
	f.judge(hideReview(f, apptest.SampleProject))
	f.wantNone()
}
