//go:build !e2e

package main

import (
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 重なった名前・接頭辞・キーの修復 (11) と、その通知を、本物の gitstore と repairing.Run と state.db の受信箱で、2 台の
// マシンの場面として確かめる(C5c1)。A のログイン名は taro、B は hanako。ワークスペース「メイン」と「サブ」は
// rulesFirstMachine のもの。どの同期もエラーにならない(m.sync が確かめる。C5c1-15)。

const (
	devA        workspace.ID = "01DUPWS000000000000000DEVA"
	devB        workspace.ID = "01DUPWS000000000000000DEVB"
	devOld      workspace.ID = "01DUPWS00000000000000DEVOL"
	devNew      workspace.ID = "01DUPWS00000000000000DEVNW"
	devPRA      project.ID   = "01DUPPR000000000000000DEVA"
	devPRB      project.ID   = "01DUPPR000000000000000DEVB"
	devItemA    item.ID      = "01DUPIT000000000000000DEVA"
	devItemB    item.ID      = "01DUPIT000000000000000DEVB"
	devReview   change.ID    = "01DUPVALUE0000000000REVIEW"
	devAValues               = "01DEVAVALUE"
	devBValues               = "01DEVBVALUE"
	devOldValue              = "01DEVOVALUE"
	devNewValue              = "01DEVNVALUE"
)

// newDevAs は、login の操作として、ワークスペース「開発」(WEB。値の内部 ID の接頭辞 values)を 1 回の保存で作る。
// pr が空でなければ、プロジェクト「アプリ」(WEB-P1)とアイテム it(WEB-1)と番号の行も一緒に書く。
func newDevAs(t *testing.T, m machine, login string, ws workspace.ID, values string, pr project.ID, it item.ID) {
	t.Helper()
	d, err := workspace.CreateWithSettings(workspace.Workspace{ID: ws, Name: "開発", Prefix: "WEB"}, ruleSettings(values, ""))
	if err != nil {
		t.Fatal(err)
	}
	m.writeAs(t, login, "ワークスペース「開発」を作成", func(tx port.Tx) error {
		if pr == "" {
			return tx.WriteWorkspaces().Append(ws, d)
		}
		p := project.Project{ID: pr, Place: project.Place{Workspace: ws, Number: 1}, Rank: "V", Title: "アプリ", Owner: login, Status: project.Todo}
		epic := item.Item{
			ID: it, Place: item.Place{Workspace: ws, Project: pr, Number: 1}, Kind: item.Epic, Rank: rank.First, Title: "画面",
			Status: ruleValue(values, 1), Priority: ruleValue(values, 8),
		}
		return errors.Join(
			tx.WriteWorkspaces().Append(ws, d, numbering.Draft(numbering.Projects, 1), numbering.Draft(numbering.Items, 1)),
			tx.WriteProjects().Append(pr, project.Create(p)),
			tx.WriteItems().Append(it, item.Create(epic)),
		)
	})
}

// wsOf は、m のワークスペース ws のいまの値(名前・接頭辞・削除)。
func wsOf(t *testing.T, m machine, ws workspace.ID) workspace.Workspace {
	t.Helper()
	w, ok := workspace.FromLog(m.wsLog(t, ws))
	if !ok {
		t.Fatalf("%s: no workspace %s", m.dir, ws)
	}
	return w
}

// wantWorkspace は、m のワークスペース ws が削除済みでなく、名前が name、接頭辞が prefix なことを確かめる。
func wantWorkspace(t *testing.T, m machine, ws workspace.ID, name, prefix string) {
	t.Helper()
	if w := wsOf(t, m, ws); w.Deleted != nil || w.Name != name || w.Prefix != prefix {
		t.Errorf("workspace %s on %s = %q / %q (deleted %v), want %q / %q", ws, m.dir, w.Name, w.Prefix, w.Deleted != nil, name, prefix)
	}
}

// wantKey は、m のアイテム id の表示の番号(ワークスペースの接頭辞と番号)が key なことを確かめる。
func wantKey(t *testing.T, m machine, id item.ID, key string) {
	t.Helper()
	p := m.item(t, id).Place
	if got := wsOf(t, m, p.Workspace).Prefix + "-" + strconv.Itoa(p.Number); got != key {
		t.Errorf("item %s on %s = %s, want %s", id, m.dir, got, key)
	}
}

// duplicateText は、修復 (11) の通知の本文。
func duplicateText(from, to string) string {
	return "2 台で同時に「" + from + "」が作られたため、あとから届いた方を「" + to + "」に変更しました"
}

// A と B が同時に「開発」(WEB)を作り、A・B・A の順に同期すると、あとから届いた B の「開発」が「開発 (2)」・WEB2 になり、
// B のアイテムは WEB2-1 になる。B(作った人)に名前と接頭辞の 1 行ずつ、A には作らない。
func TestRepairDuplicatesWorkspace(t *testing.T) {
	remote, a, b := rulesPair(t)
	newDevAs(t, a, "taro", devA, devAValues, devPRA, devItemA)
	newDevAs(t, b, "hanako", devB, devBValues, devPRB, devItemB)
	syncAll(t, remote, a, b)
	sameFiles(t, a, b)
	for _, m := range []machine{a, b} {
		wantWorkspace(t, m, devA, "開発", "WEB")
		wantWorkspace(t, m, devB, "開発 (2)", "WEB2")
		wantKey(t, m, devItemA, "WEB-1")
		wantKey(t, m, devItemB, "WEB2-1")
	}
	wantRepairs(t, b, duplicateText("開発", "開発 (2)"), duplicateText("WEB", "WEB2"))
	wantNoRepair(t, a)
}

// A が「開発」(WEB)を作って同期したあと、A が「開発」を削除して新しい「開発」(WEB)を作り、B は同時に古い「開発」に
// ステータス「Review」を足す。A・B・A の順に同期すると、古い「開発」の削除は取り消され(C5c1-13)、A の新しい
// 「開発」が「開発 (2)」・WEB2 になる。A(新しい「開発」を作った人)に名前と接頭辞の 1 行ずつ、B には作らない。
func TestRepairDuplicatesRevivedWorkspace(t *testing.T) {
	remote, a, b := rulesPair(t)
	newDevAs(t, a, "taro", devOld, devOldValue, "", "")
	a.sync(t, remote)
	b.sync(t, remote)
	a.writeAs(t, "taro", "ワークスペース「開発」を削除", func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(devOld, change.NewDeleteEntity(true))
	})
	newDevAs(t, a, "taro", devNew, devNewValue, "", "")
	st, _ := workspace.SettingsFromLog(b.wsLog(t, devOld))
	review := workspace.Value{ID: devReview, Name: "Review", Color: "blue", Category: workspace.CategoryDoing, Rank: rank.After(st.Statuses[len(st.Statuses)-1].Rank)}
	b.writeAs(t, "hanako", "ステータス「Review」を追加", func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(devOld, workspace.ValueDrafts(workspace.ParseList("statuses"), review)...)
	})
	syncAll(t, remote, a, b)
	sameFiles(t, a, b)
	for _, m := range []machine{a, b} {
		wantWorkspace(t, m, devOld, "開発", "WEB")
		wantWorkspace(t, m, devNew, "開発 (2)", "WEB2")
	}
	// 削除の取り消しの通知(C4b2)は、削除した A に別に 1 行。
	wantRepairs(t, a,
		duplicateText("開発", "開発 (2)"), duplicateText("WEB", "WEB2"),
		"ワークスペース「開発」は hanako が同時に変更したため、削除を取り消しました",
	)
	wantNoRepair(t, b)
}

// 「サブ」の設定に足すものの内部 ID。
const (
	custA   change.ID = "01DUPPROP0000000000000CUSA"
	custB   change.ID = "01DUPPROP0000000000000CUSB"
	revA    change.ID = "01DUPVALUE00000000000REVWA"
	revB    change.ID = "01DUPVALUE00000000000REVWB"
	sprintA change.ID = "01DUPSPRINT000000000000SPA"
	sprintB change.ID = "01DUPSPRINT000000000000SPB"
)

// addSubSettingsAs は、login の操作として、「サブ」にプロパティ「顧客」(キー customer、内部 ID prop)、ステータス
// status(内部 ID rev)、スプリント「Sprint 1」(内部 ID sprint)を 1 回の保存で足す。
func addSubSettingsAs(t *testing.T, m machine, login string, prop, rev, sprint change.ID, status string) {
	t.Helper()
	st, _ := workspace.SettingsFromLog(m.wsLog(t, subWS))
	p := workspace.Property{
		ID: prop, Key: "customer", Name: "顧客", Type: workspace.TypeText, Kinds: slices.Clone(workspace.Kinds),
		Rank: rank.After(st.Properties[len(st.Properties)-1].Rank),
	}
	v := workspace.Value{ID: rev, Name: status, Color: "blue", Category: workspace.CategoryDoing, Rank: rank.After(st.Statuses[len(st.Statuses)-1].Rank)}
	ds := workspace.PropertyDrafts(p)
	ds = append(ds, workspace.ValueDrafts(workspace.ParseList("statuses"), v)...)
	ds = append(ds, planning.SprintDrafts(planning.Sprint{}, planned(sprint, "Sprint 1", "2026-11-16", "2026-11-29"))...)
	m.writeAs(t, login, "設定を追加", func(tx port.Tx) error { return tx.WriteWorkspaces().Append(subWS, ds...) })
}

// A と B が同時に、「サブ」にプロパティ「顧客」(キー customer)、ステータス「Review」と「review」、スプリント
// 「Sprint 1」を足す。A・B・A の順に同期すると、B の側のものが「顧客 (2)」・customer2・「review (2)」・「Sprint 1 (2)」に
// なる。B(足した人)にキーを含めて 4 行、A には作らない。
func TestRepairDuplicatesSettings(t *testing.T) {
	remote, a, b := rulesPair(t)
	addSubSettingsAs(t, a, "taro", custA, revA, sprintA, "Review")
	addSubSettingsAs(t, b, "hanako", custB, revB, sprintB, "review")
	syncAll(t, remote, a, b)
	sameFiles(t, a, b)
	for _, m := range []machine{a, b} {
		log := m.wsLog(t, subWS)
		st, _ := workspace.SettingsFromLog(log)
		wantProperty(t, m, st, custA, "顧客", "customer")
		wantProperty(t, m, st, custB, "顧客 (2)", "customer2")
		wantStatusName(t, m, st, revA, "Review")
		wantStatusName(t, m, st, revB, "review (2)")
		sprints := planning.FromLog(log).LiveSprints()
		for id, name := range map[change.ID]string{sprintA: "Sprint 1", sprintB: "Sprint 1 (2)"} {
			if i := slices.IndexFunc(sprints, func(s planning.Sprint) bool { return s.ID == id }); i < 0 || sprints[i].Name != name {
				t.Errorf("sprint %s on %s: want %q in %+v", id, m.dir, name, sprints)
			}
		}
	}
	wantRepairs(t, b,
		duplicateText("顧客", "顧客 (2)"), duplicateText("customer", "customer2"),
		duplicateText("review", "review (2)"), duplicateText("Sprint 1", "Sprint 1 (2)"),
	)
	wantNoRepair(t, a)
}

// wantProperty は、設定 st のプロパティ id の名前とキーが name・key なことを確かめる。
func wantProperty(t *testing.T, m machine, st workspace.Settings, id change.ID, name, key string) {
	t.Helper()
	i := slices.IndexFunc(st.Properties, func(p workspace.Property) bool { return p.ID == id })
	if i < 0 || st.Properties[i].Name != name || st.Properties[i].Key != key {
		t.Errorf("property %s on %s: want %q / %q in %+v", id, m.dir, name, key, st.Properties)
	}
}

// wantStatusName は、設定 st のステータス id の名前が name なことを確かめる。
func wantStatusName(t *testing.T, m machine, st workspace.Settings, id change.ID, name string) {
	t.Helper()
	i := slices.IndexFunc(st.Statuses, func(v workspace.Value) bool { return v.ID == id })
	if i < 0 || st.Statuses[i].Name != name {
		t.Errorf("status %s on %s: want %q in %+v", id, m.dir, name, st.Statuses)
	}
}
