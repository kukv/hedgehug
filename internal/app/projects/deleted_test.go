package projects_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/projects"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// setupClock は、setup に、日付の保存が使う時計を足したもの。
func setupClock(t *testing.T) projects.Service {
	svc := setup(t)
	svc.Clock = apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC))
	return svc
}

// deleteWithItems は、プロジェクト(HH-P1)と下のアイテムを同じ操作で削除済みにする。
// root がプロジェクトの Deleted.Root(利用者が選んで削除したか)。
func deleteWithItems(t *testing.T, svc projects.Service, root bool) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "プロジェクトを削除"}
	err := svc.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		errs := []error{tx.WriteProjects().Append(apptest.SampleProject, change.NewDeleteEntity(root))}
		for _, id := range []item.ID{apptest.SampleEpic, apptest.SampleTask} {
			errs = append(errs, tx.WriteItems().Append(id, change.NewDeleteEntity(false)))
		}
		return errors.Join(errs...)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// restored は、プロジェクトと下のアイテムのログに restore の行があるかを、それぞれ返す。
func restored(t *testing.T, svc projects.Service) (proj, epic, task bool) {
	t.Helper()
	has := func(ref change.Ref) bool {
		var rs []change.Record
		err := svc.Stores.Read(t.Context(), store, func(r port.Reader) error {
			var err error
			rs, err = r.Changes().Of(ref)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return slices.ContainsFunc(rs, func(r change.Record) bool { return r.T == change.Restore })
	}
	return has(change.Ref{Kind: change.KindProject, ID: apptest.SampleProject}),
		has(change.Ref{Kind: change.KindItem, ID: apptest.SampleEpic}),
		has(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
}

// 削除済みのプロジェクトは、一覧にも Get にも出ない(A4b-4)。
func TestDeletedProjectIsHidden(t *testing.T) {
	svc := setup(t)
	ctx := t.Context()
	if err := apptest.DeleteProject(ctx, svc.Stores, store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	if list, err := svc.List(ctx, store, apptest.SampleWorkspace); err != nil || len(list) != 0 {
		t.Errorf("List = %+v, %v", list, err)
	}
	if _, err := svc.Get(ctx, store, apptest.SampleProject); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Get of a deleted project: %v, want port.ErrNotFound", err)
	}
}

// 下のアイテムと一緒に削除された(root: false)プロジェクトは、利用者が選んで削除していないので保存できない。
func TestEditProjectDeletedWithoutRootIsNotFound(t *testing.T) {
	svc := setup(t)
	deleteWithItems(t, svc, false)
	ctx := port.WithRevived(t.Context())
	_, renameErr := svc.Rename(ctx, store, apptest.SampleProject, "新しい")
	_, ownerErr := svc.SetOwner(ctx, store, apptest.SampleProject, "hanako")
	for name, err := range map[string]error{"Rename": renameErr, "SetOwner": ownerErr} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s = %v, want port.ErrNotFound", name, err)
		}
	}
	if p, _, _ := restored(t, svc); p || len(port.Revived(ctx)) != 0 {
		t.Error("root: false のプロジェクトの削除を取り消した")
	}
}

// 別のタブで削除された(root: true)プロジェクトに変更を保存すると、変更を残して、下のアイテムと一緒に削除を取り消す。
func TestEditDeletedProjectRevives(t *testing.T) {
	for name, c := range reviveCases() {
		t.Run(name, func(t *testing.T) { checkRevives(t, c) })
	}
}

type editFunc = func(svc projects.Service, ctx context.Context) (project.Project, error)

// reviveCase は、プロジェクトの保存 1 種類。withRepo は削除の前にリポジトリを結びつけておくか、kept は保存した値が残ったか。
type reviveCase struct {
	withRepo bool
	edit     editFunc
	kept     func(p project.Project) bool
}

// reviveCases は、プロジェクトの保存の種類ごとの操作。
func reviveCases() map[string]reviveCase {
	return map[string]reviveCase{
		"タイトル": {edit: func(svc projects.Service, ctx context.Context) (project.Project, error) {
			return svc.Rename(ctx, store, apptest.SampleProject, "新しい")
		}, kept: func(p project.Project) bool { return p.Title == "新しい" }},
		"ステータス": {edit: func(svc projects.Service, ctx context.Context) (project.Project, error) {
			return svc.SetStatus(ctx, store, apptest.SampleProject, project.InProgress)
		}, kept: func(p project.Project) bool { return p.Status == project.InProgress }},
		"オーナー": {edit: func(svc projects.Service, ctx context.Context) (project.Project, error) {
			return svc.SetOwner(ctx, store, apptest.SampleProject, "hanako")
		}, kept: func(p project.Project) bool { return p.Owner == "hanako" }},
		"開始日": {edit: func(svc projects.Service, ctx context.Context) (project.Project, error) {
			return svc.SetDate(ctx, store, apptest.SampleProject, projects.StartDate, "2026-12-01")
		}, kept: func(p project.Project) bool { return p.Start == "2026-12-01" }},
		"リポジトリの追加": {edit: func(svc projects.Service, ctx context.Context) (project.Project, error) {
			return svc.AddRepo(ctx, store, apptest.SampleProject, "kukv/hedgehug")
		}, kept: func(p project.Project) bool { return slices.Equal(p.Repos, []string{"kukv/hedgehug"}) }},
		"リポジトリの解除": {withRepo: true, edit: func(svc projects.Service, ctx context.Context) (project.Project, error) {
			return svc.RemoveRepo(ctx, store, apptest.SampleProject, "kukv/hedgehug")
		}, kept: func(p project.Project) bool { return len(p.Repos) == 0 }},
	}
}

// checkRevives は、プロジェクトを削除し、c.edit で保存して、変更が残り、削除が取り消されたことを確かめる。
func checkRevives(t *testing.T, c reviveCase) {
	svc := setupClock(t)
	if c.withRepo {
		if _, err := svc.AddRepo(t.Context(), store, apptest.SampleProject, "kukv/hedgehug"); err != nil {
			t.Fatal(err)
		}
	}
	deleteWithItems(t, svc, true)
	ctx := port.WithRevived(t.Context())
	got, err := c.edit(svc, ctx)
	if err != nil || got.Deleted != nil || !c.kept(got) {
		t.Fatalf("保存 = %+v, %v", got, err)
	}
	if r := port.Revived(ctx); !slices.Equal(r, []string{"HH-P1"}) {
		t.Errorf("Revived = %v", r)
	}
	if p, e, task := restored(t, svc); !p || !e || !task {
		t.Errorf("restore の行: プロジェクト %v, Epic %v, Task %v", p, e, task)
	}
}

// 同じ名前の削除されていないプロジェクトがあるときは、保存しても削除を取り消さず、断る(決定 S11)。
func TestEditDeletedProjectWithSameNameLiveProjectIsRefused(t *testing.T) {
	svc := setupClock(t)
	ctx := t.Context()
	deleteWithItems(t, svc, true)
	if _, err := svc.Create(ctx, projects.CreateInput{Store: store, Workspace: apptest.SampleWorkspace, Title: "ログイン"}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.SetStatus(ctx, store, apptest.SampleProject, project.InProgress)
	if message(err) != "同じ名前のプロジェクトがあります。先にそのプロジェクトの名前を変更してください" {
		t.Fatalf("err = %v", err)
	}
	if p, e, task := restored(t, svc); p || e || task {
		t.Errorf("断られたのに restore を書いた: %v %v %v", p, e, task)
	}
	if _, err := svc.Get(ctx, store, apptest.SampleProject); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("削除済みのままでない: %v", err)
	}
}

// タイトルの変更は、変更後のタイトルで同じ名前を判定する(決定 S11)。
// 削除済みの「ログイン」と同じ名前の新しいプロジェクトがあっても、「ログイン旧」に変えれば重ならないので取り消せる。
func TestRenameDeletedProjectChecksNewTitle(t *testing.T) {
	svc := setupClock(t)
	ctx := port.WithRevived(t.Context())
	deleteWithItems(t, svc, true)
	if _, err := svc.Create(ctx, projects.CreateInput{Store: store, Workspace: apptest.SampleWorkspace, Title: "ログイン"}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Rename(ctx, store, apptest.SampleProject, "ログイン旧")
	if err != nil || got.Deleted != nil || got.Title != "ログイン旧" {
		t.Fatalf("Rename = %+v, %v", got, err)
	}
	if r := port.Revived(ctx); !slices.Equal(r, []string{"HH-P1"}) {
		t.Errorf("Revived = %v", r)
	}
	if p, _, _ := restored(t, svc); !p {
		t.Error("restore の行がない")
	}
}

// 変更後のタイトルが削除されていないプロジェクトと重なるなら、断り、削除を取り消さない。
func TestRenameDeletedProjectToLiveTitleIsRefused(t *testing.T) {
	svc := setupClock(t)
	ctx := t.Context()
	deleteWithItems(t, svc, true)
	if _, err := svc.Create(ctx, projects.CreateInput{Store: store, Workspace: apptest.SampleWorkspace, Title: "別名"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Rename(ctx, store, apptest.SampleProject, "別名"); message(err) == "" {
		t.Fatalf("err = %v, want 同じ名前の断り", err)
	}
	if p, e, task := restored(t, svc); p || e || task {
		t.Errorf("断られたのに restore を書いた: %v %v %v", p, e, task)
	}
	if _, err := svc.Get(ctx, store, apptest.SampleProject); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("削除済みのままでない: %v", err)
	}
}

// 何も変わらない保存・断られる保存は、削除済み(root: true)なら削除を取り消さず、削除済みのままになる(決定 S4)。
func TestEditDeletedProjectRefusedKeepsDeleted(t *testing.T) {
	cases := map[string]struct {
		edit func(svc projects.Service, ctx context.Context) error
		want func(err error) bool
	}{
		"同じタイトル": {
			func(svc projects.Service, ctx context.Context) error {
				_, err := svc.Rename(ctx, store, apptest.SampleProject, "ログイン")
				return err
			},
			func(err error) bool { return errors.Is(err, port.ErrNotFound) },
		},
		"同じステータス": {
			func(svc projects.Service, ctx context.Context) error {
				_, err := svc.SetStatus(ctx, store, apptest.SampleProject, project.Todo)
				return err
			},
			func(err error) bool { return errors.Is(err, port.ErrNotFound) },
		},
		"終了日が開始日より前": {
			func(svc projects.Service, ctx context.Context) error {
				_, err := svc.SetDate(ctx, store, apptest.SampleProject, projects.EndDate, "2026-12-01")
				return err
			},
			func(err error) bool { return message(err) != "" },
		},
		"ないリポジトリの解除": {
			func(svc projects.Service, ctx context.Context) error {
				_, err := svc.RemoveRepo(ctx, store, apptest.SampleProject, "kukv/none")
				return err
			},
			func(err error) bool { return errors.Is(err, port.ErrNotFound) },
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			svc := setupClock(t)
			if _, err := svc.SetDate(t.Context(), store, apptest.SampleProject, projects.StartDate, "2026-12-10"); err != nil {
				t.Fatal(err)
			}
			deleteWithItems(t, svc, true)
			ctx := port.WithRevived(t.Context())
			if err := c.edit(svc, ctx); err == nil || !c.want(err) {
				t.Errorf("err = %v", err)
			}
			if p, e, task := restored(t, svc); p || e || task {
				t.Errorf("断られたのに restore を書いた: %v %v %v", p, e, task)
			}
		})
	}
}

// 名前の重複と、並びの末尾の決め方は、削除済みのプロジェクトを数えない。番号は削除済みも数える(A4b-5)。
func TestDeletedProjectsAreNotComparedForNamesAndRanks(t *testing.T) {
	svc := setup(t)
	ctx := t.Context()
	in := projects.CreateInput{Store: store, Workspace: apptest.SampleWorkspace, Title: "決済"}
	gone, err := svc.Create(ctx, in) // HH-P2(W)
	if err != nil || gone.Rank != "W" {
		t.Fatalf("created %+v, %v", gone, err)
	}
	if err := apptest.DeleteProject(ctx, svc.Stores, store, gone.ID); err != nil {
		t.Fatal(err)
	}
	// 削除済みの「決済」と同じ名前を作れ、並びは削除済みの W を見ずに末尾(W)になり、番号は再利用しない。
	again, err := svc.Create(ctx, in)
	if err != nil || again.Rank != "W" || again.Place.Number != 3 {
		t.Errorf("created %+v, %v; want rank W and number 3", again, err)
	}
	// 名前の変更でも、削除済みの名前は重複にならない。
	if err := apptest.DeleteProject(ctx, svc.Stores, store, again.ID); err != nil {
		t.Fatal(err)
	}
	renamed, err := svc.Rename(ctx, store, apptest.SampleProject, "決済")
	if err != nil || renamed.Title != "決済" {
		t.Errorf("Rename = %+v, %v", renamed, err)
	}
}

// 削除済みのワークスペースには、プロジェクトを作れず、一覧も読めない(B5b-4)。
func TestProjectsOfDeletedWorkspace(t *testing.T) {
	svc := setup(t)
	ctx := t.Context()
	if err := apptest.DeleteWorkspace(ctx, svc.Stores, store, apptest.SampleWorkspace); err != nil {
		t.Fatal(err)
	}
	_, createErr := svc.Create(ctx, projects.CreateInput{Store: store, Workspace: apptest.SampleWorkspace, Title: "新しい"})
	_, listErr := svc.List(ctx, store, apptest.SampleWorkspace)
	for name, err := range map[string]error{"Create": createErr, "List": listErr} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s in a deleted workspace: %v, want port.ErrNotFound", name, err)
		}
	}
}
