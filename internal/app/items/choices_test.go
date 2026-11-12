package items_test

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// custom は、サンプルのワークスペースに追加プロパティを作り、選択肢 names を足す(B2 の行の形で書く。
// app のテストは app の別のパッケージを使えないので、workspaces の操作は呼ばない)。
func custom(t *testing.T, svc items.Service, rec *recorder, key string, typ workspace.PropertyType, names ...string) (workspace.Property, []change.ID) {
	t.Helper()
	p := workspace.Property{ID: svc.IDs.New(), Key: key, Name: key, Type: typ, Kinds: workspace.Kinds, Rank: "z" + key}
	l := workspace.List{Kind: workspace.ListChoices, Property: p.ID}
	ds := workspace.PropertyDrafts(p)
	ids := make([]change.ID, len(names))
	r := ""
	for i, n := range names {
		ids[i], r = svc.IDs.New(), rank.After(r)
		ds = append(ds, workspace.ValueDrafts(l, workspace.Value{ID: ids[i], Name: n, Color: workspace.Colors[0], Rank: r})...)
	}
	err := rec.Stores.Write(t.Context(), store, port.Operation{Actor: change.User("hanako"), Summary: "プロパティを追加"}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...)
	})
	if err != nil {
		t.Fatal(err)
	}
	return p, ids
}

func quoted(id change.ID) json.RawMessage { return raw(`"` + string(id) + `"`) }

func TestSetPropertySingleChoice(t *testing.T) {
	svc, rec := setupRec(t)
	p, vs := custom(t, svc, rec, "customer", workspace.TypeSingle, "A社", "B社")
	it, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, p.ID, quoted(vs[1]))
	if err != nil || !slices.Equal(it.Choices[p.ID], []change.ID{vs[1]}) || last(rec) != "HH-2 のcustomerを B社 に変更" {
		t.Fatalf("set = %+v, %v, %q", it.Choices, err, last(rec))
	}
	n := logLen(t, rec, apptest.SampleTask)
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, p.ID, quoted(vs[1])); err != nil || logLen(t, rec, apptest.SampleTask) != n {
		t.Errorf("the same value must not be saved: %v", err)
	}
	it, err = svc.SetProperty(t.Context(), store, apptest.SampleTask, p.ID, raw(`null`))
	if err != nil || len(it.Choices[p.ID]) != 0 || last(rec) != "HH-2 のcustomerを なし に変更" {
		t.Errorf("clear = %+v, %v, %q", it.Choices, err, last(rec))
	}
	n = logLen(t, rec, apptest.SampleTask)
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, p.ID, raw(`"01NOPE00000000000000000000"`)); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown value: %v", err)
	}
	if logLen(t, rec, apptest.SampleTask) != n {
		t.Error("an unknown value must not be saved")
	}
}

func TestToggleMultiChoice(t *testing.T) {
	svc, rec := setupRec(t)
	p, vs := custom(t, svc, rec, "target", workspace.TypeMulti, "A社", "B社")
	if _, err := svc.AddValue(t.Context(), store, apptest.SampleTask, p.ID, vs[0]); err != nil || last(rec) != "HH-2 のtargetを A社 に変更" {
		t.Fatalf("add A = %v, %q", err, last(rec))
	}
	it, err := svc.AddValue(t.Context(), store, apptest.SampleTask, p.ID, vs[1])
	if err != nil || !slices.Equal(it.Choices[p.ID], vs) || last(rec) != "HH-2 のtargetを A社, B社 に変更" {
		t.Fatalf("add B = %+v, %v, %q", it.Choices, err, last(rec))
	}
	n := logLen(t, rec, apptest.SampleTask)
	if _, err := svc.AddValue(t.Context(), store, apptest.SampleTask, p.ID, vs[0]); err != nil || logLen(t, rec, apptest.SampleTask) != n {
		t.Errorf("adding a present value must not be saved: %v", err)
	}
	it, err = svc.RemoveValue(t.Context(), store, apptest.SampleTask, p.ID, vs[0])
	if err != nil || !slices.Equal(it.Choices[p.ID], vs[1:]) || last(rec) != "HH-2 のtargetを B社 に変更" {
		t.Fatalf("remove A = %+v, %v, %q", it.Choices, err, last(rec))
	}
	n = logLen(t, rec, apptest.SampleTask)
	if _, err := svc.RemoveValue(t.Context(), store, apptest.SampleTask, p.ID, vs[0]); err != nil || logLen(t, rec, apptest.SampleTask) != n {
		t.Errorf("removing an absent value must not be saved: %v", err)
	}
	if _, err := svc.AddValue(t.Context(), store, apptest.SampleTask, p.ID, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown value: %v", err)
	}
}

func TestCreateWithChoices(t *testing.T) {
	svc, rec := setupRec(t)
	single, ss := custom(t, svc, rec, "customer", workspace.TypeSingle, "A社", "B社")
	multi, ms := custom(t, svc, rec, "target", workspace.TypeMulti, "X", "Y")
	vals := map[change.ID]json.RawMessage{single.ID: quoted(ss[0]), multi.ID: raw(`["` + string(ms[1]) + `","` + string(ms[0]) + `"]`)}
	it, err := svc.Create(t.Context(), items.CreateInput{Store: store, Project: apptest.SampleProject, Kind: item.Epic, Title: "決済", Values: vals})
	if err != nil {
		t.Fatal(err)
	}
	// 複数の値は集合として読む(ログを読むときに並べ直すので、送った順は残らない)。
	if !slices.Equal(it.Choices[single.ID], ss[:1]) || !slices.Equal(it.Choices[multi.ID], ms) {
		t.Errorf("choices = %+v", it.Choices)
	}
	// 作成のあとも、複数選択は入り切りで変えられる。
	it, err = svc.RemoveValue(t.Context(), store, it.ID, multi.ID, ms[1])
	if err != nil || !slices.Equal(it.Choices[multi.ID], ms[:1]) || last(rec) != "HH-3 のtargetを X に変更" {
		t.Errorf("remove = %+v, %v, %q", it.Choices, err, last(rec))
	}
}
