package repairing_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
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
	fixed := e.workspaceOf(devWS1)
	wantWorkspace(t, fixed, "開発 (2)", "WEB2")
	wantWorkspace(t, e.workspaceOf(devWS2), "開発", "WEB")
	if got := e.item(devEpic).DisplayID(fixed.Prefix); got != "WEB2-1" {
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
func TestRenameDuplicatesProjects(t *testing.T) {
	e := newEnv(t)
	addMobile(e)
	e.newProject(mobileWS, webProject3, 2, "Web")
	e.newProject(apptest.SampleWorkspace, webProject4, 5, "web")
	theirs := e.newProject(apptest.SampleWorkspace, webProject1, 3, "Web")
	ours := e.newProject(apptest.SampleWorkspace, webProject2, 4, "Web")
	wantOps(t, e.run(ours, theirs), "プロジェクト「Web」を「Web (2)」に変更")
	want := map[project.ID]string{webProject1: "Web", webProject2: "Web (2)", webProject3: "Web", webProject4: "web"}
	for id, title := range want {
		if got := e.project(id).Title; got != title {
			t.Errorf("%s title = %q, want %q", id, got, title)
		}
	}
	log := e.logOf(change.Ref{Kind: change.KindProject, ID: webProject2})
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
