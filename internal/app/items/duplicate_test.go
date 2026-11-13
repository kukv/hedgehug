package items_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestDuplicateCopiesTheValuesAndResetsTheRest(t *testing.T) {
	svc, rec := setupRec(t)
	pick, vs := custom(t, svc, rec, "target", workspace.TypeMulti, "A社", "B社")
	in := items.CreateInput{
		Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: item.Task, Title: "調査",
		Description: "本文です",
		Values: map[change.ID]json.RawMessage{
			"status": quoted(apptest.SampleStatusInProgress), "assignee": raw(`"taro"`), "points": raw(`"3"`), "estimate": raw(`"1.5"`),
			"systems": raw(`["` + string(apptest.SampleSystemWeb) + `"]`),
		},
	}
	src, err := svc.Create(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddValue(t.Context(), store, src.ID, pick.ID, vs[1]); err != nil {
		t.Fatal(err)
	}
	other := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleEpic), item.Bug, "不具合")
	addRelation(t, rec, src.ID, other.ID, "relates")

	id, err := svc.Duplicate(t.Context(), store, src.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := last(rec); !strings.HasSuffix(got, " を複製") {
		t.Errorf("summary = %q", got)
	}
	dup, err := svc.Get(t.Context(), store, id)
	if err != nil {
		t.Fatal(err)
	}
	if dup.Title != "調査 のコピー" || dup.Kind != item.Task || dup.Place.Number != other.Place.Number+1 || dup.Place.Parent == nil || *dup.Place.Parent != apptest.SampleEpic || dup.Rank <= other.Rank {
		t.Errorf("dup = %+v rank %q (last %q)", dup.Place, dup.Rank, other.Rank)
	}
	if dup.Status != apptest.SampleStatusTodo || dup.Priority != src.Priority || dup.Assignee != "taro" || *dup.Points != 3 || *dup.Estimate != 1.5 ||
		!slices.Equal(dup.Systems, []change.ID{apptest.SampleSystemWeb}) || !slices.Equal(dup.Choices[pick.ID], []change.ID{vs[1]}) {
		t.Errorf("values = %+v", dup)
	}
	if len(dup.Watchers) != 1 || dup.Watchers[0] != src.Watchers[0] {
		t.Errorf("watchers = %v", dup.Watchers)
	}
	if len(dup.Relations) != 1 || dup.Relations[0] != (item.Relation{Type: "clones", To: src.ID}) {
		t.Errorf("relations = %+v", dup.Relations)
	}
	if text := descriptionOf(t, rec, id); text != "本文です" {
		t.Errorf("description = %q", text)
	}
	if ops := lastOps(t, rec, id, 2); ops[0] != ops[1] {
		t.Errorf("作成と clones は同じ操作: %v", ops)
	}
}

func TestDuplicateDoesNotCopyChildren(t *testing.T) {
	svc, rec := setupRec(t)
	before, err := svc.List(t.Context(), store, apptest.SampleProject)
	if err != nil {
		t.Fatal(err)
	}
	id, err := svc.Duplicate(t.Context(), store, apptest.SampleEpic)
	if err != nil {
		t.Fatal(err)
	}
	all, err := svc.List(t.Context(), store, apptest.SampleProject)
	if err != nil || len(all) != len(before)+1 {
		t.Fatalf("List = %d (before %d), %v", len(all), len(before), err)
	}
	dup := storedItem(t, rec, id)
	if dup.Place.Parent != nil || dup.Rank <= storedItem(t, rec, apptest.SampleEpic).Rank {
		t.Errorf("dup = %+v", dup)
	}
}

func TestDuplicateNotFound(t *testing.T) {
	svc, rec := setupRec(t)
	gone := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleEpic), item.Task, "消した Task")
	markDeleted(t, rec, gone.ID, true)
	for _, id := range []item.ID{gone.ID, "01NOPE00000000000000000000"} {
		if _, err := svc.Duplicate(t.Context(), store, id); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %v", id, err)
		}
	}
}
