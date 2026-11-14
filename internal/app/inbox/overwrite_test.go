package inbox_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/inbox"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

const closedSprint change.ID = "01SAMPLESPRINT000000000025"

// overwritten は、s1 の Task HH-2 の項目 field が上書きされた通知。mine と kept は値(nil は null)。
func overwritten(field string, mine, kept any) port.Notice {
	return port.Notice{
		Kind: port.NoticeOverwrite, Store: s1, Workspace: apptest.SampleWorkspace, Item: apptest.SampleTask,
		Text: "上書きされました", Display: "HH-2", Field: field, Mine: change.Value(mine), Kept: change.Value(kept),
	}
}

// projectOverwritten は、s1 のプロジェクト HH-P1 の項目 field が上書きされた通知。
func projectOverwritten(field string, mine, kept any) port.Notice {
	n := overwritten(field, mine, kept)
	n.Item, n.Project, n.Display = "", apptest.SampleProject, "HH-P1"
	return n
}

// setTask は、HH-2 に行を書く。
func (f fixture) setTask(t *testing.T, ds ...change.Draft) {
	t.Helper()
	f.write(t, func(tx port.Tx) error { return tx.WriteItems().Append(apptest.SampleTask, ds...) })
}

func (f fixture) overwrite(t *testing.T) *inbox.Overwrite {
	t.Helper()
	return f.first(t).Overwrite
}

func TestOverwriteStatusEnabled(t *testing.T) {
	f := setup(t)
	f.setTask(t, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	f.add(t, overwritten("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	got := f.overwrite(t)
	want := inbox.Overwrite{Mine: "Todo", Kept: "Done", Visible: true, Enabled: true}
	if got == nil || *got != want {
		t.Errorf("overwrite = %+v, want %+v", got, want)
	}
}

func TestOverwriteReadOnly(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		f := setup(t)
		f.list.Stores[0].ReadOnly = true
		f.setTask(t, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
		if deleted {
			if err := apptest.DeleteItems(t.Context(), f.stores, s1, apptest.SampleTask); err != nil {
				t.Fatal(err)
			}
		}
		f.add(t, overwritten("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
		got := f.overwrite(t)
		if got == nil || !got.Visible || got.Enabled || got.Note != "閲覧のみの Store です" {
			t.Errorf("deleted=%v: overwrite = %+v, want visible, disabled, read-only note", deleted, got)
		}
	}
}

func TestOverwriteDeleted(t *testing.T) {
	f := setup(t)
	f.setTask(t, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	if err := apptest.DeleteItems(t.Context(), f.stores, s1, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	f.add(t, overwritten("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	got := f.overwrite(t)
	want := inbox.Overwrite{Mine: "Todo", Kept: "Done"}
	if got == nil || *got != want {
		t.Errorf("overwrite = %+v, want %+v", got, want)
	}
}

func TestOverwriteMoved(t *testing.T) {
	f := setup(t)
	n := overwritten("status", apptest.SampleStatusTodo, apptest.SampleStatusDone)
	n.Item = "01SAMPLEIT0000000000000009"
	f.add(t, n)
	if got := f.overwrite(t); got != nil {
		t.Errorf("overwrite = %+v, want nil", got)
	}
}

func TestOverwriteChangedAfter(t *testing.T) {
	f := setup(t)
	f.setTask(t, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusInProgress))
	f.add(t, overwritten("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	got := f.overwrite(t)
	if got == nil || !got.Visible || got.Enabled || got.Note != "そのあとさらに変更されています(いまは InProgress)" {
		t.Errorf("overwrite = %+v, want disabled with the current value", got)
	}
}

func TestOverwriteReasons(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(f fixture)
		notice  port.Notice
		want    string
	}{
		{
			name: "削除されたステータス",
			prepare: func(f fixture) {
				f.setTask(t, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
				f.write(t, func(tx port.Tx) error {
					return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewDelete("statuses/"+string(apptest.SampleStatusTodo)))
				})
			},
			notice: overwritten("status", apptest.SampleStatusTodo, apptest.SampleStatusDone),
			want:   "「Todo」は削除されています",
		},
		{
			name:    "メンバーにいない担当者",
			prepare: func(fixture) {},
			notice:  overwritten("assignee", "ghost", nil),
			want:    "担当者 ghost はこの Store のメンバーにいません",
		},
		{
			name: "完了したスプリント",
			prepare: func(f fixture) {
				sp := planning.Sprint{ID: closedSprint, Name: "Sprint 25", State: planning.Closed, Start: "2026-10-01", End: "2026-10-14"}
				f.write(t, func(tx port.Tx) error {
					return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, planning.SprintDrafts(planning.Sprint{}, sp)...)
				})
			},
			notice: overwritten("sprint", closedSprint, nil),
			want:   "スプリント「Sprint 25」は完了しています",
		},
		{
			name:    "終了日よりあとの開始日",
			prepare: func(f fixture) { f.setTask(t, change.NewSet("end", nil, "2026-11-10")) },
			notice:  overwritten("start", "2026-11-20", nil),
			want:    "終了日は開始日以降にしてください",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setup(t)
			c.prepare(f)
			f.add(t, c.notice)
			got := f.overwrite(t)
			if got == nil || !got.Visible || got.Enabled || got.Note != c.want {
				t.Errorf("overwrite = %+v, want disabled with %q", got, c.want)
			}
		})
	}
}

func TestOverwriteProject(t *testing.T) {
	f := setup(t)
	f.write(t, func(tx port.Tx) error { return tx.WriteMembers().Append(member.Rename("hanako", "hanako", "花子")) })
	f.add(t, projectOverwritten("owner", "hanako", "taro"))
	got := f.overwrite(t)
	want := inbox.Overwrite{Mine: "花子", Kept: "taro", Visible: true, Enabled: true}
	if got == nil || *got != want {
		t.Errorf("overwrite = %+v, want %+v", got, want)
	}
	f = setup(t)
	f.add(t, projectOverwritten("owner", "", "taro"))
	if got := f.overwrite(t); got == nil || got.Mine != "なし" || !got.Enabled {
		t.Errorf("overwrite = %+v, want なし and enabled", got)
	}
}

func TestOverwriteBodyOnly(t *testing.T) {
	cases := map[string]port.Notice{
		"place":      overwritten("place", nil, nil),
		"rank":       overwritten("rank", "a", "b"),
		"kind":       overwritten("kind", "Task", "Bug"),
		"relations":  overwritten("relations", nil, nil),
		"links":      overwritten("links", nil, nil),
		"comment":    overwritten("comment:01SAMPLECOMMENT00000000001", "a", "b"),
		"desc":       overwritten("desc", nil, nil),
		"rule":       projectOverwritten("project_rules/"+string(apptest.SampleProject)+"/hidden_statuses", nil, nil),
		"workspace":  {Kind: port.NoticeOverwrite, Store: s1, Workspace: apptest.SampleWorkspace, Text: "ワークスペース名", Field: "name", Mine: change.Value("A"), Kept: change.Value("B")},
		"member":     {Kind: port.NoticeOverwrite, Store: s1, Text: "表示名", Field: "name/hanako", Mine: change.Value("A"), Kept: change.Value("B")},
		"no field":   {Kind: port.NoticeOverwrite, Store: s1, Workspace: apptest.SampleWorkspace, Item: apptest.SampleTask, Text: "上書き"},
		"other kind": {Kind: port.NoticeAutomation, Store: s1, Workspace: apptest.SampleWorkspace, Item: apptest.SampleTask, Text: "a", Field: "status"},
	}
	for name, n := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			f.add(t, n)
			if got := f.overwrite(t); got != nil {
				t.Errorf("overwrite = %+v, want nil", got)
			}
		})
	}
}

func TestOverwriteResult(t *testing.T) {
	f := setup(t)
	f.setTask(t, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	f.add(t, overwritten("status", apptest.SampleStatusTodo, apptest.SampleStatusDone))
	if err := f.book.Resolve(t.Context(), f.first(t).ID, port.NoticeApplied); err != nil {
		t.Fatal(err)
	}
	if got := f.overwrite(t); got == nil || got.Result != port.NoticeApplied {
		t.Errorf("overwrite = %+v, want result applied", got)
	}
}
