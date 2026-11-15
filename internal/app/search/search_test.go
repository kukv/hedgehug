package search

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func searchOf(t *testing.T, f fixture, text string) Result {
	t.Helper()
	res, err := f.svc.Search(t.Context(), text, TableOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func rowDisplays(res Result) []string {
	out := make([]string, 0, len(res.Rows))
	for _, r := range res.Rows {
		out = append(out, string(r.Store.ID)+"/"+r.Facts.Display)
	}
	return out
}

func pts(v float64) *float64 { return &v }

// kind:bug sort:-points は、Bug だけをポイントの降順で並べる。
func TestSearchMatchesAndOrders(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	ids := []item.ID{item3, item4, item5}
	for i, p := range []float64{3, 8, 5} {
		it := task(ids[i], 3+i, item.Bug, nil, "バグ")
		it.Points = pts(p)
		addItem(t, f.stores, it)
	}
	res := searchOf(t, f, "kind:bug sort:-points")
	if len(res.Errors) != 0 {
		t.Fatalf("Errors = %v", res.Errors)
	}
	if got, want := rowDisplays(res), []string{"S1/HH-4", "S1/HH-5", "S1/HH-3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
	if res.Rows[0].Store.Name != "main" {
		t.Errorf("Store = %+v", res.Rows[0].Store)
	}
}

// countingStores は、Items().List の呼び出しを数える。
type countingStores struct {
	port.Stores
	lists *int
}

func (c countingStores) Read(ctx context.Context, id port.StoreID, fn func(r port.Reader) error) error {
	return c.Stores.Read(ctx, id, func(r port.Reader) error { return fn(countingReader{r, c.lists}) })
}

type countingReader struct {
	port.Reader
	lists *int
}

func (r countingReader) Items() port.ItemReader { return countingItems{r.Reader.Items(), r.lists} }

type countingItems struct {
	port.ItemReader
	lists *int
}

func (i countingItems) List(p project.ID) ([]item.Item, error) {
	*i.lists++
	return i.ItemReader.List(p)
}

// 解析のエラーがあれば、アイテムは読まない。
func TestSearchErrorsSkipItems(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	n := 0
	f.svc.Stores = countingStores{f.stores, &n}
	res := searchOf(t, f, "status:")
	if len(res.Errors) != 1 || len(res.Rows) != 0 {
		t.Fatalf("Errors = %v, Rows = %d", res.Errors, len(res.Rows))
	}
	if n != 0 {
		t.Errorf("Items().List を %d 回呼んだ", n)
	}
	searchOf(t, f, "")
	if n == 0 {
		t.Error("エラーがなければ読むはず(数え方の確認)")
	}
}

// 非表示の Store のワークスペースにだけある追加プロパティのキーも、エラーにならない。
func TestSearchKeyOfHiddenStoreIsKnown(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	settings := workspace.DefaultSettings(func() change.ID { return "01TESTVAL00000000000000000" })
	settings.Properties = append(settings.Properties, workspace.Property{
		ID: "01TESTPROP00000000000000001", Key: "Customer", Name: "顧客", Type: workspace.TypeText, Kinds: workspace.Kinds, Rank: "z",
	})
	ws := workspace.Workspace{ID: "01TESTWS00000000000000009", Name: "Old", Prefix: "OLD"}
	d, err := workspace.CreateWithSettings(ws, settings)
	must(t, err)
	must(t, f.stores.Write(t.Context(), store3, port.Operation{Actor: change.User("taro"), Summary: "テスト"}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(ws.ID, d, numbering.Draft(numbering.Projects, 1))
	}))
	res := searchOf(t, f, "customer:acme")
	if len(res.Errors) != 0 || len(res.Rows) != 0 {
		t.Errorf("Errors = %v, Rows = %d", res.Errors, len(res.Rows))
	}
	if res := searchOf(t, f, "nothing:acme"); len(res.Errors) != 1 {
		t.Errorf("未知のキー Errors = %v", res.Errors)
	}
}

// 削除済みのワークスペースにだけある追加プロパティのキーは、知らないキーとして断る。
func TestSearchKeyOfDeletedWorkspaceIsUnknown(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	settings := workspace.DefaultSettings(func() change.ID { return "01TESTVAL00000000000000000" })
	settings.Properties = append(settings.Properties, workspace.Property{
		ID: "01TESTPROP00000000000000001", Key: "Customer", Name: "顧客", Type: workspace.TypeText, Kinds: workspace.Kinds, Rank: "z",
	})
	ws := workspace.Workspace{ID: "01TESTWS00000000000000009", Name: "Old", Prefix: "OLD"}
	d, err := workspace.CreateWithSettings(ws, settings)
	must(t, err)
	write(t, f.stores, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(ws.ID, d, numbering.Draft(numbering.Projects, 1))
	})
	if res := searchOf(t, f, "customer:acme"); len(res.Errors) != 0 {
		t.Fatalf("削除する前 Errors = %v", res.Errors)
	}
	must(t, apptest.DeleteWorkspace(t.Context(), f.stores, store1, ws.ID))
	if res := searchOf(t, f, "customer:acme"); len(res.Errors) != 1 {
		t.Errorf("削除したあと Errors = %v", res.Errors)
	}
}

// 削除済みは、保持日数 30・11/01 に削除・今日 11/15 で、あと 16 日。
func TestSearchDeletedDaysLeft(t *testing.T) {
	f := newFixture(t)
	stores := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	for _, id := range []port.StoreID{store1, store2, store3} {
		stores.AddStore(id)
	}
	f.stores, f.svc.Stores = stores, stores
	f.svc.Clock = apptest.NewClock(time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC))
	must(t, apptest.Sample(t.Context(), stores, store1))
	addItem(t, stores, task(item3, 3, item.Task, nil, "消すもの"))
	must(t, apptest.DeleteItems(t.Context(), stores, store1, item3))
	stores.SetRetentionDays(store1, 30)
	res := searchOf(t, f, "is:deleted")
	if got, want := rowDisplays(res), []string{"S1/HH-3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if !res.Deleted || res.Rows[0].DaysLeft == nil || *res.Rows[0].DaysLeft != 16 {
		t.Errorf("Deleted = %v, DaysLeft = %v", res.Deleted, res.Rows[0].DaysLeft)
	}
	if live := searchOf(t, f, ""); live.Rows[0].DaysLeft != nil {
		t.Errorf("削除済みでなければ DaysLeft はない")
	}
}

type countingClock struct {
	now   time.Time
	calls int
}

func (c *countingClock) Now() time.Time { c.calls++; return c.now }

// 1 回の検索は、時計を 1 回だけ読む(照合の今日と、材料・残り日数の時刻をそろえる)。
func TestSearchReadsClockOnce(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	clock := &countingClock{now: clockStart}
	f.svc.Clock = clock
	searchOf(t, f, "is:deleted")
	if clock.calls != 1 {
		t.Errorf("Now の呼び出し = %d, want 1", clock.calls)
	}
}

type loginByStore map[port.StoreID]string

func (l loginByStore) Me(_ context.Context, id port.StoreID) (string, error) { return l[id], nil }

// @me は、アイテムの Store の「あなた」。
func TestSearchMeIsPerStore(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	op := port.Operation{Actor: change.User("taro"), Summary: "テスト"}
	must(t, apptest.Sample(ctx, f.stores, store1))
	must(t, apptest.SampleTeam(ctx, f.stores, store2))
	t1 := task(item3, 3, item.Task, nil, "S1 の tanaka")
	t1.Assignee = "tanaka"
	t2 := task(item4, 4, item.Task, nil, "S1 の suzuki")
	t2.Assignee = "suzuki"
	addItem(t, f.stores, t1)
	addItem(t, f.stores, t2)
	s1 := item.Item{
		ID: item5, Place: item.Place{Workspace: apptest.SampleTeamWorkspace, Project: apptest.SampleTeamProject, Number: 2},
		Kind: item.Task, Rank: "V", Title: "S2 の suzuki", Assignee: "suzuki", Status: apptest.SampleStatusTodo,
	}
	s2 := s1
	s2.ID, s2.Place.Number, s2.Title, s2.Assignee = item6, 3, "S2 の tanaka", "tanaka"
	for _, it := range []item.Item{s1, s2} {
		must(t, f.stores.Write(ctx, store2, op, func(tx port.Tx) error {
			return tx.WriteItems().Append(it.ID, item.Create(it), numbering.Draft(numbering.Items, it.Place.Number))
		}))
	}
	f.svc.Identity = loginByStore{store1: "tanaka", store2: "suzuki"}
	res := searchOf(t, f, "assignee:@me")
	got := map[string]bool{}
	for _, r := range res.Rows {
		got[r.Facts.Item.Title] = true
	}
	if want := map[string]bool{"S1 の tanaka": true, "S2 の suzuki": true}; !reflect.DeepEqual(got, want) {
		t.Errorf("hit = %v, want %v", got, want)
	}
}

func TestSearchGroupAndHasKey(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	res := searchOf(t, f, "group:kind foo")
	if res.Group != "kind" || !res.HasKey || len(res.Errors) != 0 {
		t.Errorf("Group = %q, HasKey = %v, Errors = %v", res.Group, res.HasKey, res.Errors)
	}
	if res := searchOf(t, f, "foo"); res.HasKey || res.Group != "" {
		t.Errorf("キーなし: %+v", res)
	}
}

func TestSearchHasSort(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	for _, c := range []struct {
		text string
		want bool
	}{
		{"sort:id", true},
		{"kind:bug", false},
		{"-sort:id", true},
		{"sort:id kind:nosuchvalue zzz:1", true},
		{"zzz:1", false},
	} {
		if got := searchOf(t, f, c.text).HasSort; got != c.want {
			t.Errorf("%q: HasSort = %v, want %v", c.text, got, c.want)
		}
	}
}
