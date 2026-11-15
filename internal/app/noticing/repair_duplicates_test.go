package noticing_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const devWorkspace workspace.ID = "01DEVWS0000000000000000001"

var devRef = change.Ref{Kind: change.KindWorkspace, ID: devWorkspace}

// createDev は、by がワークスペース「開発」(接頭辞 WEB)を作る。
func createDev(f *fixture, by string) {
	f.t.Helper()
	f.save(user(by), func(tx port.Tx) error {
		d, err := workspace.Create(workspace.Workspace{ID: devWorkspace, Name: "開発", Prefix: "WEB"})
		if err != nil {
			return err
		}
		return tx.WriteWorkspaces().Append(devWorkspace, d)
	})
}

// fixDuplicate は、ref の項目 field を from から to に直す、重なりの修復の保存。
func fixDuplicate(f *fixture, ref change.Ref, field, from, to string) port.ReceivedSave {
	f.t.Helper()
	return f.repairSave(appendRef(ref, change.NewSet(field, from, to)))
}

func TestDuplicatesWorkspace(t *testing.T) {
	f := setup(t)
	createDev(f, "hanako")
	create := f.lastRow(devRef, "name")
	f.judge(fixDuplicate(f, devRef, "name", "開発", "開発 (2)"))
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Workspace: devWorkspace, Section: "general",
		Text:   "2 台で同時に「開発」が作られたため、あとから届いた方を「開発 (2)」に変更しました",
		Source: "repair:duplicate:" + string(devWorkspace) + ":name:" + create,
	}
	if n := f.wantOne(); !reflect.DeepEqual(n, want) {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestDuplicatesPrefix(t *testing.T) {
	f := setup(t)
	createDev(f, "hanako")
	f.judge(fixDuplicate(f, devRef, "prefix", "WEB", "WEB2"))
	n := f.wantOne()
	if want := "2 台で同時に「WEB」が作られたため、あとから届いた方を「WEB2」に変更しました"; n.Text != want || n.Section != "general" {
		t.Errorf("notice = %+v, want text %q in general", n, want)
	}
}

func TestDuplicatesProject(t *testing.T) {
	f := setup(t)
	f.save(user("hanako"), appendRef(projectRef, change.NewSet("title", "ログイン", "設計")))
	set := f.lastRow(projectRef, "title")
	f.judge(fixDuplicate(f, projectRef, "title", "設計", "設計 (2)"))
	n := f.wantOne()
	if n.Project != apptest.SampleProject || n.Workspace != apptest.SampleWorkspace {
		t.Errorf("notice = %+v, want project %s", n, apptest.SampleProject)
	}
	if want := "repair:duplicate:" + string(apptest.SampleProject) + ":title:" + set; n.Source != want {
		t.Errorf("source = %q, want %q", n.Source, want)
	}
}

func TestDuplicatesRule(t *testing.T) {
	f := setup(t)
	field := "rules/" + string(notifyRule) + "/name"
	f.save(user("hanako"), appendRef(wsRef, change.NewSet(field, nil, "通知")))
	f.judge(fixDuplicate(f, wsRef, field, "通知", "通知 (2)"))
	n := f.wantOne()
	if n.Rule != notifyRule || n.Section != "automation" || n.Workspace != apptest.SampleWorkspace {
		t.Errorf("notice = %+v, want rule %s in automation", n, notifyRule)
	}
}

func TestDuplicatesValue(t *testing.T) {
	tests := []struct {
		name, field, section string
	}{
		{"ステータス", pendingRef + "/name", "statuses"},
		{"優先度", "priorities/01PRIO000000000000000000001/name", "priorities"},
		{"関連システム", "systems/01SYSTEM000000000000000001/name", "systems"},
		{"プロパティ", "properties/01PROP000000000000000001/name", "properties"},
		{"選択肢", "choices/01PROP000000000000000001/01CHOICE000000000000000001/name", "properties"},
		{"キー", "properties/01PROP000000000000000001/key", "properties"},
		{"テンプレート", "templates/01TEMPLATE0000000000000001/name", "templates"},
		{"スプリント", "sprints/" + string(sprintA) + "/name", "sprints"},
		{"リリース", "releases/" + string(releaseA) + "/name", "releases"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := setup(t)
			f.save(user("hanako"), appendRef(wsRef, change.NewSet(tt.field, nil, "x")))
			f.judge(fixDuplicate(f, wsRef, tt.field, "x", "x (2)"))
			if n := f.wantOne(); n.Section != tt.section || n.Workspace != apptest.SampleWorkspace {
				t.Errorf("notice = %+v, want section %q", n, tt.section)
			}
		})
	}
}

// 名前の変更で重なった: 名前を変えた人に知らせ、作った人には知らせない(決定 C5c1-10)。
func TestDuplicatesRenamer(t *testing.T) {
	for _, me := range []string{"hanako", "taro"} {
		t.Run(me, func(t *testing.T) {
			f := setupAs(t, me)
			f.save(user("hanako"), appendRef(wsRef, change.NewSet("name", "Web", "開発")))
			f.judge(fixDuplicate(f, wsRef, "name", "開発", "開発 (2)"))
			if me == "taro" {
				f.wantNone()
				return
			}
			if n, want := f.wantOne(), "2 台で同時に「開発」が作られたため、あとから届いた方を「開発 (2)」に変更しました"; n.Text != want {
				t.Errorf("text = %q, want %q", n.Text, want)
			}
		})
	}
}

func TestDuplicatesRecipient(t *testing.T) {
	f := setup(t)
	createDev(f, "taro")
	f.judge(fixDuplicate(f, devRef, "name", "開発", "開発 (2)"))
	f.wantNone()
}

func TestDuplicatesSkipsOtherRepairs(t *testing.T) {
	enabled := "rules/" + string(notifyRule) + "/enabled"
	name := pendingRef + "/name"
	tests := []struct {
		name string
		prep []change.Draft
		fix  []change.Draft
	}{
		{"ルールの無効化", []change.Draft{change.NewSet(enabled, nil, true)}, []change.Draft{change.NewSet(enabled, true, false)}},
		{"削除の取り消し", []change.Draft{change.NewSet(name, "Pending", "Review")}, []change.Draft{
			change.NewRestore(pendingRef, "01OP0000000000000000000001"), change.NewSet(name, "Review", "Pending"),
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := setup(t)
			f.save(user("hanako"), appendRef(wsRef, tt.prep...))
			f.judge(f.repairSave(appendRef(wsRef, tt.fix...)))
			for _, n := range f.notices() {
				if strings.HasPrefix(n.Source, "repair:duplicate:") {
					t.Errorf("notice = %+v, want no duplicate notice", n)
				}
			}
		})
	}
}

// 2 台が同じ重なりを直した: 行の ID が違う 2 つの修復の保存は、どちらも作った行が鍵になる。
// このマシンのログに自分の修復の行(other)があるまま、もう 1 台の修復(s)を受け取っても、作った行を鍵に作る。
func TestDuplicatesOnceAcrossMachines(t *testing.T) {
	f := setup(t)
	createDev(f, "hanako")
	create := f.lastRow(devRef, "name")
	s := fixDuplicate(f, devRef, "name", "開発", "開発 (2)")
	other := fixDuplicate(f, devRef, "name", "開発", "開発 (2)")
	other.Entities[0].Rows[0].Prev = nil // 偽物の Store は順に書くので、同時に書いた行の prev をここで作る
	f.judge(s)
	if n := f.wantOne(); n.Source != "repair:duplicate:"+string(devWorkspace)+":name:"+create {
		t.Errorf("source = %q", n.Source)
	}
	f.judge(s, other)
	f.judge(other)
	if rows := f.rows(); len(rows) != 1 {
		t.Errorf("rows = %+v, want one", rows)
	}
}

// 2 台が同じ重なりを直し(ra・rb)、その間に hanako が名前を変えた(prev は ra)。rb は ra も名前の変更も見ずに
// 書いたので、rb の行の prev は空(作った行のまま)。rb を判定しても、名前を決めた行は作った行で、鍵は ra と同じ。
// hanako に 2 件目は出さない(決定 C5c1-10・C5c1-11)。
func TestDuplicatesRenamedBetweenMachines(t *testing.T) {
	f := setup(t)
	createDev(f, "hanako")
	create := f.lastRow(devRef, "name")
	f.judge(fixDuplicate(f, devRef, "name", "開発", "開発 (2)"))
	f.save(user("hanako"), appendRef(devRef, change.NewSet("name", "開発 (2)", "Web")))
	rb := fixDuplicate(f, devRef, "name", "開発", "開発 (2)")
	rb.Entities[0].Rows[0].Prev = nil // 偽物の Store は順に書くので、同時に書いた行の prev をここで作る
	f.judge(rb)
	rows := f.rows()
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want one", rows)
	}
	if want := "repair:duplicate:" + string(devWorkspace) + ":name:" + create; rows[0].Source != want {
		t.Errorf("source = %q, want %q", rows[0].Source, want)
	}
}
