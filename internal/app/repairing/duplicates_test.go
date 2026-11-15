package repairing_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	devWS1      workspace.ID = "01DEVWS0000000000000000001"
	devWS2      workspace.ID = "01DEVWS0000000000000000002"
	devProject  project.ID   = "01DEVPR0000000000000000001"
	devEpic     item.ID      = "01DEVIT0000000000000000001"
	webProject1 project.ID   = "01WEBPR0000000000000000001"
	webProject2 project.ID   = "01WEBPR0000000000000000002"
	webProject3 project.ID   = "01WEBPR0000000000000000003"
	webProject4 project.ID   = "01WEBPR0000000000000000004"
)

func wsRef(id workspace.ID) change.Ref { return change.Ref{Kind: change.KindWorkspace, ID: id} }

// rowIDs は、refs のログの行の ID(ないログは飛ばす)。
func (e *env) rowIDs(refs []change.Ref) map[change.ID]bool {
	e.t.Helper()
	out := map[change.ID]bool{}
	err := e.s.Read(e.ctx, store, func(r port.Reader) error {
		for _, ref := range refs {
			log, err := r.Changes().Of(ref)
			if err != nil && !errors.Is(err, port.ErrNotFound) {
				return err
			}
			for _, row := range log {
				out[row.ID] = true
			}
		}
		return nil
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return out
}

// writeLogs は、1 回の保存を書き、サンプルのログと refs のログに足した行の ID を返す。
func (e *env) writeLogs(refs []change.Ref, fn func(tx port.Tx) error) map[change.ID]bool {
	e.t.Helper()
	before := e.rowIDs(refs)
	added := e.write(taro, fn)
	for id := range e.rowIDs(refs) {
		if !before[id] {
			added[id] = true
		}
	}
	return added
}

// createWorkspace は、既定の設定のワークスペース id を作る行を tx に書く。
func createWorkspace(tx port.Tx, id workspace.ID, name, prefix string) error {
	n := 0
	gen := func() change.ID {
		n++
		return change.ID(fmt.Sprintf("01D%sV%021d", string(id[len(id)-1]), n))
	}
	d, err := workspace.CreateWithSettings(workspace.Workspace{ID: id, Name: name, Prefix: prefix}, workspace.DefaultSettings(gen))
	if err != nil {
		return err
	}
	return tx.WriteWorkspaces().Append(id, d)
}

// newWorkspace は、ワークスペース id を作り、足した行の ID を返す。
func (e *env) newWorkspace(id workspace.ID, name, prefix string) map[change.ID]bool {
	return e.writeLogs([]change.Ref{wsRef(id)}, func(tx port.Tx) error { return createWorkspace(tx, id, name, prefix) })
}

// newDevWithEpic は、ワークスペース devWS1(名前「開発」、接頭辞 WEB)と、その中のプロジェクト WEB-P1・Epic WEB-1 を作る。
func (e *env) newDevWithEpic() map[change.ID]bool {
	refs := []change.Ref{wsRef(devWS1), {Kind: change.KindProject, ID: devProject}, {Kind: change.KindItem, ID: devEpic}}
	return e.writeLogs(refs, func(tx port.Tx) error {
		pr := project.Project{ID: devProject, Place: project.Place{Workspace: devWS1, Number: 1}, Rank: "V", Title: "画面", Owner: "taro", Status: project.Todo}
		epic := item.Item{ID: devEpic, Place: item.Place{Workspace: devWS1, Project: devProject, Number: 1}, Kind: item.Epic, Rank: "a", Title: "一覧"}
		return errors.Join(
			createWorkspace(tx, devWS1, "開発", "WEB"),
			tx.WriteWorkspaces().Append(devWS1, numbering.Draft(numbering.Projects, 1), numbering.Draft(numbering.Items, 1)),
			tx.WriteProjects().Append(pr.ID, project.Create(pr)),
			tx.WriteItems().Append(epic.ID, item.Create(epic)),
		)
	})
}

// workspaceOf は、ワークスペース id を読む(削除済みも)。
func (e *env) workspaceOf(id workspace.ID) workspace.Workspace {
	e.t.Helper()
	var w workspace.Workspace
	err := e.s.Read(e.ctx, store, func(r port.Reader) error {
		var err error
		w, err = r.Workspaces().Get(id)
		return err
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return w
}

func wantWorkspace(t *testing.T, got workspace.Workspace, name, prefix string) {
	t.Helper()
	if got.Name != name || got.Prefix != prefix || got.Deleted != nil {
		t.Errorf("workspace %s = %+v, want %s %s live", got.ID, got, name, prefix)
	}
}

// 2 台で同じ名前・接頭辞のワークスペースを作ったら、自分の側で作った方の名前と接頭辞を、1 つずつの保存で直す。
func TestRenameDuplicatesWorkspaces(t *testing.T) {
	e := newEnv(t)
	ours := e.newDevWithEpic()
	theirs := e.newWorkspace(devWS2, "開発", "WEB")
	wantOps(t, e.run(ours, theirs), "ワークスペース「開発」を「開発 (2)」に変更", "接頭辞「WEB」を「WEB2」に変更")
	wantWorkspace(t, e.workspaceOf(devWS1), "開発 (2)", "WEB2")
	wantWorkspace(t, e.workspaceOf(devWS2), "開発", "WEB")
	if got := e.item(devEpic).DisplayID("WEB2"); got != "WEB2-1" {
		t.Errorf("epic = %s, want WEB2-1", got)
	}
	if old := workspace.OldPrefixes(e.logOf(wsRef(devWS1))); !slices.Contains(old, "WEB") {
		t.Errorf("old prefixes = %v, want WEB", old)
	}
}

// newProject は、ワークスペース ws にタイトル title のプロジェクト id(番号 n)を作り、足した行の ID を返す。
func (e *env) newProject(ws workspace.ID, id project.ID, n int, title string) map[change.ID]bool {
	p := project.Project{ID: id, Place: project.Place{Workspace: ws, Number: n}, Rank: "X", Title: title, Owner: "taro", Status: project.Todo}
	return e.writeLogs([]change.Ref{wsRef(ws), {Kind: change.KindProject, ID: id}}, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(ws, numbering.Draft(numbering.Projects, n)),
			tx.WriteProjects().Append(id, project.Create(p)),
		)
	})
}

// 同じワークスペースの同じ名前のプロジェクトは直す。別のワークスペースの同じ名前と、大文字と小文字だけ違う名前は直さない。
// 自分の側のものは、古く内部 ID が小さくても直す(新しい方や内部 ID の大きい方を直すのではない。C5c1-4)。
func TestRenameDuplicatesProjects(t *testing.T) {
	e := newEnv(t)
	addMobile(e)
	e.newProject(mobileWS, webProject3, 2, "Web")
	e.newProject(apptest.SampleWorkspace, webProject4, 5, "web")
	ours := e.newProject(apptest.SampleWorkspace, webProject1, 3, "Web")
	theirs := e.newProject(apptest.SampleWorkspace, webProject2, 4, "Web")
	wantOps(t, e.run(ours, theirs), "プロジェクト「Web」を「Web (2)」に変更")
	want := map[project.ID]string{webProject1: "Web (2)", webProject2: "Web", webProject3: "Web", webProject4: "web"}
	for id, title := range want {
		if got := e.project(id).Title; got != title {
			t.Errorf("%s title = %q, want %q", id, got, title)
		}
	}
	log := e.logOf(change.Ref{Kind: change.KindProject, ID: webProject1})
	if last := log[len(log)-1]; last.T != change.Set || last.F != "title" {
		t.Errorf("last row = %+v, want set title", last)
	}
}

// 共通の祖先にある「X」を自分の側で「開発」に変え、remote の側で「開発」を作ったら、名前を変えた方を直す。
func TestRenameDuplicatesRenamed(t *testing.T) {
	e := newEnv(t)
	e.newWorkspace(devWS1, "X", "XX")
	ours := e.writeLogs([]change.Ref{wsRef(devWS1)}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(devWS1, change.NewSet("name", "X", "開発"))
	})
	theirs := e.newWorkspace(devWS2, "開発", "DEV")
	wantOps(t, e.run(ours, theirs), "ワークスペース「開発」を「開発 (2)」に変更")
	wantWorkspace(t, e.workspaceOf(devWS1), "開発 (2)", "XX")
	wantWorkspace(t, e.workspaceOf(devWS2), "開発", "DEV")
}

// 共通の祖先にある「開発」の削除を、同期の修復で取り消したら、同時に作った新しい「開発」の方を直す(C5c1-13)。
func TestRenameDuplicatesRevivedWorkspace(t *testing.T) {
	for _, swap := range []bool{false, true} {
		t.Run(fmt.Sprint("swap=", swap), func(t *testing.T) {
			e := newEnv(t)
			e.newWorkspace(devWS1, "開発", "WEB")
			refs := []change.Ref{wsRef(devWS1), wsRef(devWS2)}
			deleted := union(
				e.writeLogs(refs, func(tx port.Tx) error { return tx.WriteWorkspaces().Append(devWS1, change.NewDeleteEntity(true)) }),
				e.newWorkspace(devWS2, "開発", "WEB"),
			)
			added := e.writeLogs(refs, func(tx port.Tx) error {
				rv := workspace.Value{ID: review, Name: "Review", Color: "blue", Category: workspace.CategoryDoing, Rank: "z"}
				return tx.WriteWorkspaces().Append(devWS1, workspace.ValueDrafts(statuses, rv)...)
			})
			ours, theirs := added, deleted
			if swap {
				ours, theirs = deleted, added
			}
			wantOps(t, e.run(ours, theirs),
				"ワークスペース「開発」の削除を取り消し", "ワークスペース「開発」を「開発 (2)」に変更", "接頭辞「WEB」を「WEB2」に変更")
			wantWorkspace(t, e.workspaceOf(devWS1), "開発", "WEB")
			wantWorkspace(t, e.workspaceOf(devWS2), "開発 (2)", "WEB2")
		})
	}
}

const (
	dupProp1   change.ID = "01DUPPROP0000000000000001"
	dupProp2   change.ID = "01DUPPROP0000000000000002"
	dupProp3   change.ID = "01DUPPROP0000000000000003"
	dupValue1  change.ID = "01DUPVALUE000000000000001"
	dupValue2  change.ID = "01DUPVALUE000000000000002"
	dupValue3  change.ID = "01DUPVALUE000000000000003"
	dupValue4  change.ID = "01DUPVALUE000000000000004"
	dupTpl1    change.ID = "01DUPTPL00000000000000001"
	dupTpl2    change.ID = "01DUPTPL00000000000000002"
	dupRule1   change.ID = "01DUPRULE0000000000000001"
	dupRule2   change.ID = "01DUPRULE0000000000000002"
	dupSprint1 change.ID = "01DUPSPRINT00000000000001"
	dupSprint2 change.ID = "01DUPSPRINT00000000000002"
	dupRel1    change.ID = "01DUPRELEASE0000000000001"
	dupRel2    change.ID = "01DUPRELEASE0000000000002"
)

// writeWS は、サンプルのワークスペースのログに ds を書き、足した行の ID を返す。
func (e *env) writeWS(ds ...change.Draft) map[change.ID]bool {
	return e.write(taro, func(tx port.Tx) error { return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...) })
}

// sampleSettings は、サンプルのワークスペースの設定。
func (e *env) sampleSettings() workspace.Settings {
	e.t.Helper()
	s, _ := workspace.SettingsFromLog(e.logOf(wsRef(apptest.SampleWorkspace)))
	return s
}

// sampleNames は、サンプルのワークスペースの名前とキー。鍵は、プロパティ・選択肢・テンプレート・ルール・スプリント・
// リリースの内部 ID(名前)と、「プロパティの ID/key」(キー)。
func (e *env) sampleNames() map[string]string {
	e.t.Helper()
	log := e.logOf(wsRef(apptest.SampleWorkspace))
	s, _ := workspace.SettingsFromLog(log)
	got := map[string]string{}
	for _, p := range s.Properties {
		got[string(p.ID)], got[string(p.ID)+"/key"] = p.Name, p.Key
		for _, v := range p.Choices {
			got[string(v.ID)] = v.Name
		}
	}
	for _, x := range s.Templates {
		got[string(x.ID)] = x.Name
	}
	for _, r := range automation.FromLog(log) {
		got[string(r.ID)] = r.Name
	}
	plan := planning.FromLog(log)
	for _, x := range plan.Sprints {
		got[string(x.ID)] = x.Name
	}
	for _, x := range plan.Releases {
		got[string(x.ID)] = x.Name
	}
	return got
}

func propertyDrafts(id change.ID, key, name string, typ workspace.PropertyType) []change.Draft {
	return workspace.PropertyDrafts(workspace.Property{ID: id, Key: key, Name: name, Type: typ, Kinds: []string{"Task"}, Rank: "z" + string(id)})
}

func valueDrafts(l workspace.List, id change.ID, name string) []change.Draft {
	return workspace.ValueDrafts(l, workspace.Value{ID: id, Name: name, Color: "blue", Category: workspace.CategoryDoing, Rank: "z" + string(id)})
}

func choices(p change.ID) workspace.List {
	return workspace.List{Kind: workspace.ListChoices, Property: p}
}

func templateDrafts(id change.ID, name string) []change.Draft {
	return workspace.TemplateDrafts(workspace.Template{}, workspace.Template{ID: id, Name: name, Kind: "Bug"})
}

// ruleDrafts は、ルール「通知」の作成の行。
func ruleDrafts(id change.ID, enabled bool, scope project.ID) []change.Draft {
	r := automation.Rule{ID: id, Name: "通知", Trigger: automation.TriggerPRMerged, Action: automation.ActionStatus, Value: string(apptest.SampleStatusDone), Enabled: enabled, Scope: scope}
	return automation.Drafts(automation.Rule{}, r)
}

// sprintDrafts は、スプリント「Sprint 1」の作成の行。
func sprintDrafts(id change.ID, state planning.SprintState) []change.Draft {
	return planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: id, Name: "Sprint 1", State: state})
}

func releaseDrafts(id change.ID, name string, state planning.ReleaseState) []change.Draft {
	return planning.ReleaseDrafts(planning.Release{}, planning.Release{ID: id, Name: name, State: state})
}

// wantNames は、名前とキー got(鍵は sampleNames のとおり)が want のとおりか確かめる。
func wantNames(t *testing.T, got, want map[string]string) {
	t.Helper()
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

// 同じワークスペースの設定・ルール・スプリント・リリースで重なった名前とプロパティのキーは、自分の側で足した方を直す。
func TestRenameDuplicatesSettings(t *testing.T) {
	for _, c := range []struct {
		name         string
		base         []change.Draft
		theirs, ours []change.Draft
		want         []string
		names        map[string]string // wantNames の want
	}{
		{
			name:   "プロパティ",
			theirs: propertyDrafts(dupProp1, "customer", "顧客", workspace.TypeText),
			ours:   propertyDrafts(dupProp2, "customer", "顧客", workspace.TypeText),
			want:   []string{"プロパティ「顧客」を「顧客 (2)」に変更", "プロパティのキー「customer」を「customer2」に変更"},
			names: map[string]string{
				string(dupProp2): "顧客 (2)", string(dupProp2) + "/key": "customer2",
				string(dupProp1): "顧客", string(dupProp1) + "/key": "customer",
			},
		},
		{
			name:   "組み込みの名前の変更",
			theirs: []change.Draft{change.NewSet("properties/assignee/name", "担当者", "Owner")},
			ours:   propertyDrafts(dupProp2, "owner", "Owner", workspace.TypeText),
			want:   []string{"プロパティ「Owner」を「Owner (2)」に変更"},
			names:  map[string]string{"assignee": "Owner", string(dupProp2): "Owner (2)"},
		},
		{
			name: "選択肢",
			base: slices.Concat(propertyDrafts(dupProp1, "area", "領域", workspace.TypeSingle),
				propertyDrafts(dupProp3, "team", "チーム", workspace.TypeMulti), valueDrafts(choices(dupProp3), dupValue3, "A")),
			theirs: valueDrafts(choices(dupProp1), dupValue1, "A"),
			ours:   valueDrafts(choices(dupProp1), dupValue2, "A"),
			want:   []string{"選択肢「A」を「A (2)」に変更"},
			names:  map[string]string{string(dupValue1): "A", string(dupValue2): "A (2)", string(dupValue3): "A"},
		},
		{
			name:   "テンプレート",
			theirs: templateDrafts(dupTpl1, "バグ報告"),
			ours:   templateDrafts(dupTpl2, "バグ報告"),
			want:   []string{"テンプレート「バグ報告」を「バグ報告 (2)」に変更"},
			names:  map[string]string{string(dupTpl1): "バグ報告", string(dupTpl2): "バグ報告 (2)"},
		},
		{
			name:   "ルール",
			theirs: ruleDrafts(dupRule1, true, ""),
			ours:   ruleDrafts(dupRule2, true, ""),
			want:   []string{"自動化「通知」を「通知 (2)」に変更"},
			names:  map[string]string{string(dupRule1): "通知", string(dupRule2): "通知 (2)"},
		},
		{
			name:   "スプリント",
			theirs: sprintDrafts(dupSprint1, planning.Closed),
			ours:   sprintDrafts(dupSprint2, planning.Planned),
			want:   []string{"スプリント「Sprint 1」を「Sprint 1 (2)」に変更"},
			names:  map[string]string{string(dupSprint1): "Sprint 1", string(dupSprint2): "Sprint 1 (2)"},
		},
		{
			name:   "リリース",
			theirs: releaseDrafts(dupRel1, "v1", planning.Archived),
			ours:   releaseDrafts(dupRel2, "v1", planning.Unreleased),
			want:   []string{"リリース「v1」を「v1 (2)」に変更"},
			names:  map[string]string{string(dupRel1): "v1", string(dupRel2): "v1 (2)"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			if c.base != nil {
				e.writeWS(c.base...)
			}
			theirs := e.writeWS(c.theirs...)
			ours := e.writeWS(c.ours...)
			wantOps(t, e.run(ours, theirs), c.want...)
			wantNames(t, e.sampleNames(), c.names)
		})
	}
}

// 値の名前は、同じ一覧の中で、大文字と小文字を区別せずに比べる。別の一覧の同じ名前は直さない。
func TestRenameDuplicatesValuesFold(t *testing.T) {
	e := newEnv(t)
	ours := e.writeWS(slices.Concat(
		valueDrafts(statuses, dupValue1, "review"),
		valueDrafts(workspace.ParseList("priorities"), dupValue2, "high"),
		valueDrafts(workspace.ParseList("systems"), dupValue3, "web"),
		valueDrafts(workspace.ParseList("priorities"), dupValue4, "API"),
	)...)
	wantOps(t, e.run(ours, nil),
		"ステータス「review」を「review (2)」に変更", "優先度「high」を「high (2)」に変更", "関連システム「web」を「web (2)」に変更")
	s := e.sampleSettings()
	if i := slices.IndexFunc(s.Priorities, func(v workspace.Value) bool { return v.ID == dupValue4 }); i < 0 || s.Priorities[i].Name != "API" {
		t.Errorf("priorities = %+v", s.Priorities)
	}
}

// 非表示のプロパティの名前も数える(C5c1-2)。
func TestRenameDuplicatesHiddenProperty(t *testing.T) {
	e := newEnv(t)
	theirs := e.writeWS(append(propertyDrafts(dupProp1, "customer", "顧客", workspace.TypeText),
		change.NewSet("properties/"+string(dupProp1)+"/hidden", false, true))...)
	ours := e.writeWS(propertyDrafts(dupProp2, "client", "顧客", workspace.TypeText)...)
	if p, _ := e.sampleSettings().Property(dupProp1); !p.Hidden {
		t.Fatalf("property = %+v, want hidden", p)
	}
	wantOps(t, e.run(ours, theirs), "プロパティ「顧客」を「顧客 (2)」に変更")
	wantNames(t, e.sampleNames(), map[string]string{string(dupProp1): "顧客", string(dupProp2): "顧客 (2)"})
}

// (10) が削除を取り消した値と、同時に足した同じ名前の値が重なったら、足した方を直す(C5c1-6・C5c1-13)。
func TestRenameDuplicatesRestoredValue(t *testing.T) {
	e := newEnv(t)
	theirs := union(e.writeWS(change.NewDelete(reviewRef)), e.writeWS(valueDrafts(statuses, dupValue1, "Review")...))
	ours := e.writeWS(change.NewSet("category_default/doing", apptest.SampleStatusInProgress, review))
	wantOps(t, e.run(ours, theirs), "「Review」の削除を取り消し", "ステータス「Review」を「Review (2)」に変更")
	got := map[change.ID]string{}
	for _, v := range e.sampleSettings().Statuses {
		got[v.ID] = v.Name
	}
	if got[review] != "Review" || got[dupValue1] != "Review (2)" {
		t.Errorf("statuses = %v, want Review kept and the new one Review (2)", got)
	}
}

// 削除済みのスプリントとプロパティは数えない。
func TestRenameDuplicatesSkipsDeleted(t *testing.T) {
	e := newEnv(t)
	theirs := e.writeWS(slices.Concat(
		sprintDrafts(dupSprint1, planning.Planned),
		[]change.Draft{planning.DeleteSprint(dupSprint1)},
		propertyDrafts(dupProp1, "customer", "顧客A", workspace.TypeText),
		[]change.Draft{change.NewDelete("properties/" + string(dupProp1))},
	)...)
	ours := e.writeWS(slices.Concat(
		sprintDrafts(dupSprint2, planning.Planned),
		propertyDrafts(dupProp2, "customer", "顧客B", workspace.TypeText),
	)...)
	wantOps(t, e.run(ours, theirs))
}

// 専用ルールとワークスペース全体のルールの名前の重なりも直す。無効のルールも数える。
func TestRenameDuplicatesScopedRule(t *testing.T) {
	e := newEnv(t)
	theirs := e.writeWS(ruleDrafts(dupRule1, true, project2)...)
	ours := e.writeWS(ruleDrafts(dupRule2, false, "")...)
	wantOps(t, e.run(ours, theirs), "自動化「通知」を「通知 (2)」に変更")
}

// 1 回の Run で、ワークスペースの名前 → プロジェクト → 値 → スプリントの順に直す(C5c1-6)。
func TestRenameDuplicatesOrder(t *testing.T) {
	e := newEnv(t)
	theirs := e.writeWS(sprintDrafts(dupSprint1, planning.Planned)...)
	ours := union(
		e.writeWS(slices.Concat(valueDrafts(statuses, dupValue1, "review"), sprintDrafts(dupSprint2, planning.Planned))...),
		e.newProject(apptest.SampleWorkspace, webProject1, 3, "ログイン"),
		e.newWorkspace(devWS2, "Web", "DEV"),
	)
	wantOps(t, e.run(ours, theirs),
		"ワークスペース「Web」を「Web (2)」に変更", "プロジェクト「ログイン」を「ログイン (2)」に変更",
		"ステータス「review」を「review (2)」に変更", "スプリント「Sprint 1」を「Sprint 1 (2)」に変更")
}
