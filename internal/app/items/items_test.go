package items_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const store port.StoreID = "S1"

func setup(t *testing.T) items.Service {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	if err := apptest.Sample(context.Background(), s, store); err != nil {
		t.Fatal(err)
	}
	return items.Service{Stores: s, IDs: ids, Identity: apptest.Identity{Login: "hanako"}, Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))}
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func ptr(id item.ID) *item.ID { return &id }

func TestCreateTaskUnderEpic(t *testing.T) {
	svc := setup(t)
	it, err := svc.Create(t.Context(), items.CreateInput{Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: item.Task, Title: " API を\n 作る "})
	if err != nil {
		t.Fatal(err)
	}
	// サンプルは HH-1(Epic)と HH-2(Task, 並び V)を持つ。次の番号は 3、並びは兄弟の末尾。
	if it.DisplayID("HH") != "HH-3" || it.Title != "API を 作る" || it.Rank != "W" || it.Place.Workspace != apptest.SampleWorkspace {
		t.Errorf("created %+v", it)
	}
	if !slices.Equal(it.Watchers, []string{"hanako"}) || it.Created.By != "hanako" {
		t.Errorf("watchers %v, created %+v", it.Watchers, it.Created)
	}
	list, err := svc.List(t.Context(), store, apptest.SampleProject)
	if err != nil || len(list) != 3 || list[2].ID != it.ID {
		t.Errorf("List = %+v, %v", list, err)
	}
}

func TestCreateEpicAndStep(t *testing.T) {
	svc := setup(t)
	epic, err := svc.Create(t.Context(), items.CreateInput{Store: store, Project: apptest.SampleProject, Kind: item.Epic, Title: "決済"})
	if err != nil || epic.Place.Parent != nil || epic.Rank != "W" || epic.Place.Number != 3 {
		t.Fatalf("epic %+v, %v", epic, err)
	}
	step, err := svc.Create(t.Context(), items.CreateInput{Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleTask), Kind: item.Step, Title: "下書き"})
	if err != nil || step.Place.Number != 4 || step.Rank != "V" {
		t.Fatalf("step %+v, %v", step, err)
	}
}

func TestCreateRefuses(t *testing.T) {
	svc := setup(t)
	cases := []struct {
		in   items.CreateInput
		want string
	}{
		{items.CreateInput{Project: apptest.SampleProject, Kind: item.Task, Title: " \n "}, "タイトルを入力してください"},
		{items.CreateInput{Kind: item.Task, Title: "画面"}, "プロジェクトを選んでください"},
		{items.CreateInput{Parent: ptr(apptest.SampleEpic), Kind: item.Task, Title: "画面"}, "プロジェクトを選んでください"},
		{items.CreateInput{Project: apptest.SampleProject, Parent: ptr("01NOPE00000000000000000000"), Kind: item.Task, Title: " "}, "タイトルを入力してください"},
		{items.CreateInput{Parent: ptr("01NOPE00000000000000000000"), Kind: item.Task, Title: "画面"}, "プロジェクトを選んでください"},
		{items.CreateInput{Project: apptest.SampleProject, Kind: item.Step, Title: "下書き"}, "親になる Story・Task・Bug を選んでください"},
		{items.CreateInput{Project: apptest.SampleProject, Kind: item.Task, Title: "画面"}, "親になる Epic を選んでください"},
		{items.CreateInput{Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: item.Step, Title: "下書き"}, "Step は Epic の下に置けません"},
	}
	for _, c := range cases {
		c.in.Store = store
		if _, err := svc.Create(t.Context(), c.in); message(err) != c.want {
			t.Errorf("%+v: %v, want %q", c.in, err, c.want)
		}
	}
	if list, _ := svc.List(t.Context(), store, apptest.SampleProject); len(list) != 2 {
		t.Errorf("a refused create saved something: %d items", len(list))
	}
}

func TestCreateWithParentInAnotherProject(t *testing.T) {
	_, err := setup(t).Create(t.Context(), items.CreateInput{Store: store, Project: "01OTHERPR00000000000000001", Parent: ptr(apptest.SampleEpic), Kind: item.Task, Title: "画面"})
	if !errors.Is(err, items.ErrParentElsewhere) {
		t.Errorf("err = %v", err)
	}
}

func TestGetUnknown(t *testing.T) {
	if _, err := setup(t).Get(t.Context(), store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestCreateUsesInitialStatusAndPriority(t *testing.T) {
	svc, _ := setupRec(t)
	it, err := svc.Create(t.Context(), items.CreateInput{Store: store, Project: apptest.SampleProject, Kind: item.Epic, Title: "新しい"})
	if err != nil || it.Status != apptest.SampleStatusTodo || it.Priority != apptest.SamplePriorityMedium {
		t.Errorf("Create = %+v, %v", it, err)
	}
}

func TestCreateWritesGivenValues(t *testing.T) {
	svc, _ := setupRec(t)
	epic := apptest.SampleEpic
	it, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Parent: &epic, Kind: item.Task, Title: "x",
		Values: map[change.ID]json.RawMessage{
			"status":   raw(`"` + string(apptest.SampleStatusInProgress) + `"`),
			"assignee": raw(`"taro"`), "end": raw(`"2026-11-20"`), "estimate": raw(`"2"`),
			"systems": raw(`["` + string(apptest.SampleSystemWeb) + `"]`),
		},
	})
	if err != nil || it.Status != apptest.SampleStatusInProgress || it.Assignee != "taro" ||
		it.End != "2026-11-20" || it.Estimate == nil || *it.Estimate != 2 || len(it.Systems) != 1 {
		t.Errorf("Create = %+v, %v", it, err)
	}
}

func TestCreateChecksValuesAfterTheParent(t *testing.T) {
	svc, _ := setupRec(t)
	_, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Kind: item.Task, Title: "x",
		Values: map[change.ID]json.RawMessage{"points": raw(`"-1"`)},
	})
	if message(err) != "親になる Epic を選んでください" {
		t.Errorf("err = %v", err)
	}
}

func TestAddStepTakesTheParentsAssignee(t *testing.T) {
	svc, _ := setupRec(t)
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "assignee", raw(`"taro"`)); err != nil {
		t.Fatal(err)
	}
	st, err := svc.AddStep(t.Context(), store, apptest.SampleTask, "確認")
	if err != nil || st.Assignee != "taro" || st.Status != apptest.SampleStatusTodo || st.Priority != apptest.SamplePriorityMedium {
		t.Errorf("AddStep = %+v, %v", st, err)
	}
}
