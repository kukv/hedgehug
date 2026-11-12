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
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const store port.StoreID = "S1"

func setup(t *testing.T) projects.Service {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	if err := apptest.Sample(context.Background(), s, store); err != nil {
		t.Fatal(err)
	}
	return projects.Service{Stores: s, IDs: ids, Identity: apptest.Identity{Login: "hanako"}}
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func TestCreateNumbersAndPlacesAtTheEnd(t *testing.T) {
	svc := setup(t)
	p, err := svc.Create(t.Context(), projects.CreateInput{Store: store, Workspace: apptest.SampleWorkspace, Title: "  決済 ", Owner: "taro", Start: "2026-11-01", End: "2026-11-30"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Place.Number != 2 || p.DisplayID("HH") != "HH-P2" || p.Title != "決済" || p.Status != project.Todo || p.Rank != "W" {
		t.Errorf("created %+v", p)
	}
	list, err := svc.List(t.Context(), store, apptest.SampleWorkspace)
	if err != nil || len(list) != 2 || list[1] != p {
		t.Errorf("List = %+v, %v", list, err)
	}
	got, err := svc.Get(t.Context(), store, p.ID)
	if err != nil || got != p {
		t.Errorf("Get = %+v, %v", got, err)
	}
}

// タイトルは前後の空白だけを除いて保存する(02)。途中の改行は残し、アクティビティの 1 行には空白にまとめて書く。
func TestCreateKeepsNewlineInTitle(t *testing.T) {
	svc := setup(t)
	p, err := svc.Create(t.Context(), projects.CreateInput{Store: store, Workspace: apptest.SampleWorkspace, Title: " a\nb \r\n c "})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(t.Context(), store, p.ID)
	if err != nil || got.Title != "a\nb \r\n c" {
		t.Errorf("Get = %+v, %v", got, err)
	}
}

func TestCreateWritesOneSaveByMe(t *testing.T) {
	svc := setup(t)
	p, err := svc.Create(t.Context(), projects.CreateInput{Store: store, Workspace: apptest.SampleWorkspace, Title: "決済"})
	if err != nil {
		t.Fatal(err)
	}
	err = svc.Stores.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindProject, ID: p.ID})
		if err != nil || len(log) != 1 || log[0].By != "hanako" {
			t.Errorf("project log = %+v, %v", log, err)
		}
		ws, _ := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
		last := ws[len(ws)-1]
		if last.T != "number" || last.F != "projects" || string(last.To) != "2" || last.Op != log[0].Op {
			t.Errorf("number row = %+v", last)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateRefuses(t *testing.T) {
	svc := setup(t)
	cases := []struct {
		in   projects.CreateInput
		want string
	}{
		{projects.CreateInput{Title: ""}, "タイトルを入力してください"},
		{projects.CreateInput{Title: "決済", Start: "2026-11-02", End: "2026-11-01"}, "終了日は開始日以降にしてください"},
		{projects.CreateInput{Title: "ログイン"}, "このワークスペースには同じ名前のプロジェクトがあります"},
	}
	for _, c := range cases {
		c.in.Store, c.in.Workspace = store, apptest.SampleWorkspace
		if _, err := svc.Create(t.Context(), c.in); message(err) != c.want {
			t.Errorf("%+v: %v, want %q", c.in, err, c.want)
		}
	}
	list, _ := svc.List(t.Context(), store, apptest.SampleWorkspace)
	if len(list) != 1 {
		t.Errorf("a refused create saved something: %+v", list)
	}
}

func TestCreateRefusesUnknownStatus(t *testing.T) {
	svc := setup(t)
	if _, err := svc.Create(t.Context(), projects.CreateInput{Store: store, Workspace: apptest.SampleWorkspace, Title: "決済", Status: "Review"}); err == nil {
		t.Error("an unknown status was accepted")
	}
}

func TestCreateInUnknownWorkspace(t *testing.T) {
	svc := setup(t)
	_, err := svc.Create(t.Context(), projects.CreateInput{Store: store, Workspace: "01NOPE00000000000000000000", Title: "決済"})
	if !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}
