package projects_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/projects"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// recorder は、Write に渡したアクティビティの 1 行を覚える。
type recorder struct {
	*apptest.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func (r *recorder) last() string { return r.summaries[len(r.summaries)-1] }

func setupEdit(t *testing.T) (projects.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	now := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	s := apptest.NewStores(apptest.NewClock(now), ids)
	s.AddStore(store)
	if err := apptest.Sample(context.Background(), s, store); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return projects.Service{Stores: rec, IDs: ids, Identity: apptest.Identity{Login: "hanako"}, Clock: apptest.NewClock(now)}, rec
}

// logOf は、サンプルのプロジェクトのログ。何も保存しなかったことを確かめるのに使う。
func logOf(t *testing.T, s port.Stores) []change.Record {
	t.Helper()
	var log []change.Record
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		var err error
		log, err = r.Changes().Of(change.Ref{Kind: change.KindProject, ID: apptest.SampleProject})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return log
}

func TestRenameTrimsAndSavesOneRow(t *testing.T) {
	svc, rec := setupEdit(t)
	p, err := svc.Rename(t.Context(), store, apptest.SampleProject, "  決済  ")
	if err != nil || p.Title != "決済" {
		t.Fatalf("Rename = %+v, %v", p, err)
	}
	if got, want := rec.last(), "ログイン のタイトルを 決済 に変更"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	log := logOf(t, svc.Stores)
	row := log[len(log)-1]
	if row.T != change.Set || row.F != "title" || row.By != "hanako" || string(row.From) != `"ログイン"` || string(row.To) != `"決済"` {
		t.Errorf("row = %+v", row)
	}
}

func TestRenameRefuses(t *testing.T) {
	svc, _ := setupEdit(t)
	if _, err := svc.Create(t.Context(), projects.CreateInput{Store: store, Workspace: apptest.SampleWorkspace, Title: "決済"}); err != nil {
		t.Fatal(err)
	}
	before := len(logOf(t, svc.Stores))
	for _, c := range []struct{ title, want string }{
		{"  ", "タイトルは空にできません"},
		{"決済", "このワークスペースには同じ名前のプロジェクトがあります"},
	} {
		if _, err := svc.Rename(t.Context(), store, apptest.SampleProject, c.title); message(err) != c.want {
			t.Errorf("Rename(%q) = %v, want %q", c.title, err, c.want)
		}
	}
	if n := len(logOf(t, svc.Stores)); n != before {
		t.Errorf("a refused rename saved %d rows", n-before)
	}
	if _, err := svc.Rename(t.Context(), store, "01NOPE00000000000000000000", "請求"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Rename of a missing project = %v", err)
	}
}

func TestRenameToTheSameTitleSavesNothing(t *testing.T) {
	svc, _ := setupEdit(t)
	before := len(logOf(t, svc.Stores))
	if _, err := svc.Rename(t.Context(), store, apptest.SampleProject, " ログイン "); err != nil {
		t.Fatal(err)
	}
	if n := len(logOf(t, svc.Stores)); n != before {
		t.Errorf("saved %d rows for the same title", n-before)
	}
}

func TestSetStatus(t *testing.T) {
	svc, rec := setupEdit(t)
	p, err := svc.SetStatus(t.Context(), store, apptest.SampleProject, project.InProgress)
	if err != nil || p.Status != project.InProgress {
		t.Fatalf("SetStatus = %+v, %v", p, err)
	}
	if got, want := rec.last(), "ログイン のステータスを InProgress に変更"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	before := len(logOf(t, svc.Stores))
	if _, err := svc.SetStatus(t.Context(), store, apptest.SampleProject, project.InProgress); err != nil || len(logOf(t, svc.Stores)) != before {
		t.Errorf("the same status saved something: %v", err)
	}
	if _, err := svc.SetStatus(t.Context(), store, apptest.SampleProject, "Review"); err == nil {
		t.Error("an unknown status was accepted")
	}
}

func TestSetOwner(t *testing.T) {
	svc, rec := setupEdit(t)
	p, err := svc.SetOwner(t.Context(), store, apptest.SampleProject, "HANAKO")
	if err != nil || p.Owner != "hanako" {
		t.Fatalf("SetOwner = %+v, %v", p, err) // メンバーのログイン名で保存する
	}
	if got, want := rec.last(), "ログイン のオーナーを hanako に変更"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	p, err = svc.SetOwner(t.Context(), store, apptest.SampleProject, "")
	if err != nil || p.Owner != "" {
		t.Fatalf("SetOwner none = %+v, %v", p, err)
	}
	if got, want := rec.last(), "ログイン のオーナーを なし に変更"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	if _, err := svc.SetOwner(t.Context(), store, apptest.SampleProject, "nobody"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("a non-member owner = %v, want ErrNotFound", err)
	}
}

func TestSetDate(t *testing.T) {
	svc, rec := setupEdit(t)
	set := func(field projects.DateField, value string) project.Project {
		t.Helper()
		p, err := svc.SetDate(t.Context(), store, apptest.SampleProject, field, value)
		if err != nil {
			t.Fatalf("SetDate(%s, %q) = %v", field, value, err)
		}
		return p
	}
	if p := set(projects.StartDate, "2026-11-20"); p.Start != "2026-11-20" {
		t.Errorf("Start = %q", p.Start)
	}
	if got, want := rec.last(), "ログイン の開始日を 11/20 に変更"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	if p := set(projects.EndDate, "2027-01-05"); p.End != "2027-01-05" {
		t.Errorf("End = %q", p.End)
	}
	if got, want := rec.last(), "ログイン の終了日を 2027/01/05 に変更"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	set(projects.EndDate, "2026-11-20") // 開始日と同じ日は認める
	for _, c := range []struct {
		field projects.DateField
		value string
	}{{projects.StartDate, "2026-11-21"}, {projects.EndDate, "2026-11-19"}} {
		if _, err := svc.SetDate(t.Context(), store, apptest.SampleProject, c.field, c.value); message(err) != "終了日は開始日以降にしてください" {
			t.Errorf("SetDate(%s, %q) = %v", c.field, c.value, err)
		}
	}
	if p := set(projects.StartDate, ""); p.Start != "" { // 日付を空にすることは常にできる
		t.Errorf("Start = %q", p.Start)
	}
	if got, want := rec.last(), "ログイン の開始日を なし に変更"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	before := len(logOf(t, svc.Stores))
	set(projects.EndDate, "2026-11-20")
	if n := len(logOf(t, svc.Stores)); n != before {
		t.Errorf("the same date saved %d rows", n-before)
	}
}

func TestAddAndRemoveRepo(t *testing.T) {
	svc, rec := setupEdit(t)
	ctx := t.Context()
	p, err := svc.AddRepo(ctx, store, apptest.SampleProject, " example-org/web ")
	if err != nil || len(p.Repos) != 1 || p.Repos[0] != "example-org/web" {
		t.Fatalf("AddRepo = %+v, %v", p, err)
	}
	if got, want := rec.last(), "ログイン にリポジトリ example-org/web を追加"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	for _, c := range []struct{ in, want string }{
		{"Example-Org/WEB", "もう追加されています"},
		{"web", "リポジトリは owner/name の形式で入力してください"},
		{"a/b/c", "リポジトリは owner/name の形式で入力してください"},
	} {
		if _, err := svc.AddRepo(ctx, store, apptest.SampleProject, c.in); message(err) != c.want {
			t.Errorf("AddRepo(%q) = %v, want %q", c.in, err, c.want)
		}
	}
	if p, err = svc.AddRepo(ctx, store, apptest.SampleProject, "acme/api"); err != nil || len(p.Repos) != 2 || p.Repos[0] != "acme/api" {
		t.Fatalf("AddRepo second = %+v, %v", p, err)
	}
	// 削除は、保存してある綴りで行を書く。アクティビティには入力のとおりに書く。
	p, err = svc.RemoveRepo(ctx, store, apptest.SampleProject, "EXAMPLE-ORG/web")
	if err != nil || len(p.Repos) != 1 || p.Repos[0] != "acme/api" {
		t.Fatalf("RemoveRepo = %+v, %v", p, err)
	}
	if got, want := rec.last(), "ログイン からリポジトリ EXAMPLE-ORG/web を削除"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	log := logOf(t, svc.Stores)
	if row := log[len(log)-1]; row.T != change.Remove || row.F != "repos" || string(row.From) != `"example-org/web"` {
		t.Errorf("remove row = %+v", row)
	}
	before := len(log)
	if _, err := svc.RemoveRepo(ctx, store, apptest.SampleProject, "none/none"); err != nil || len(logOf(t, svc.Stores)) != before {
		t.Errorf("removing a repo that is not there saved something: %v", err)
	}
}
