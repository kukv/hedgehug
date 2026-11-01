package mount

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/query"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// newStoreWith creates a store like newStoreAt, with the schema sc.
func newStoreWith(t *testing.T, name, prefix string, sc *schema.Schema) config.StoreEntry {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	ws := &model.Workspace{ULID: model.NewULID(), Name: name, IDPrefix: prefix}
	if _, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: name}, sc, ws, testAuthor, "", time.Now); err != nil {
		t.Fatal(err)
	}
	return config.StoreEntry{Name: name, Path: dir}
}

func statuses(ss ...schema.Status) []schema.Status { return ss }

// twoSchemas is org-a (a review status at place 2, a size property, the kind
// project before task) and org-b (a wip status at place 1, the kind task
// before epic).
func twoSchemas(t *testing.T) (a, b config.StoreEntry) {
	t.Helper()
	root := []string{schema.Root}
	sa := &schema.Schema{
		Statuses: statuses(
			schema.Status{Name: "todo", Category: schema.CategoryTodo, Default: true},
			schema.Status{Name: "in_progress", Category: schema.CategoryDoing, Default: true},
			schema.Status{Name: "review", Category: schema.CategoryDoing},
			schema.Status{Name: "done", Category: schema.CategoryDone, Default: true},
			schema.Status{Name: "canceled", Category: schema.CategoryCanceled, Default: true},
		),
		Kinds: []schema.Kind{
			{Name: "project", Role: schema.RoleContainer, Parents: root},
			{Name: "task", Role: schema.RoleWork, Parents: root, Default: true},
		},
		Props: schema.Props{
			{Name: "size", Type: schema.TypeNumber},
			{Name: "tier", Type: schema.TypeEnum, Values: []string{"gold", "silver"}},
		},
	}
	sb := &schema.Schema{
		Statuses: statuses(
			schema.Status{Name: "todo", Category: schema.CategoryTodo, Default: true},
			schema.Status{Name: "wip", Category: schema.CategoryDoing, Default: true},
			schema.Status{Name: "done", Category: schema.CategoryDone, Default: true},
			schema.Status{Name: "canceled", Category: schema.CategoryCanceled, Default: true},
		),
		Kinds: []schema.Kind{
			{Name: "task", Role: schema.RoleWork, Parents: root, Default: true},
			{Name: "epic", Role: schema.RoleContainer, Parents: root},
		},
		Props: schema.Props{{Name: "tier", Type: schema.TypeEnum, Values: []string{"silver", "gold"}}},
	}
	return newStoreWith(t, "org-a", "API", sa), newStoreWith(t, "org-b", "WEB", sb)
}

type spec struct{ title, kind, status, size, tier string }

func putSpecs(t *testing.T, e config.StoreEntry, specs ...spec) {
	t.Helper()
	putTasks(t, e, len(specs), func(i int, task *model.Task) {
		sp := specs[i]
		task.Title, task.Kind, task.Status = sp.title, sp.kind, sp.status
		for k, v := range map[string]string{"size": sp.size, "tier": sp.tier} {
			if v != "" {
				task.Props.Set(k, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v})
			}
		}
	})
}

func twoStoreSet(t *testing.T) *Set {
	t.Helper()
	a, b := twoSchemas(t)
	putSpecs(t, a,
		spec{title: "a-todo", kind: "task", status: "todo", size: "5", tier: "silver"},
		spec{title: "a-review", kind: "task", status: "review", size: "20"},
		spec{title: "a-project", kind: "project", status: "todo"},
	)
	putSpecs(t, b,
		spec{title: "b-wip", kind: "task", status: "wip", tier: "silver"},
		spec{title: "b-todo", kind: "task", status: "todo", tier: "gold"},
		spec{title: "b-epic", kind: "epic", status: "todo"},
	)
	return openSet(t, a, b)
}

func noOptions(*Mounted) (query.Options, error) { return query.Options{}, nil }

func queryTitles(t *testing.T, set *Set, q, only string) []string {
	t.Helper()
	parsed, err := query.Parse(q)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Query(set, parsed, only, noOptions)
	if err != nil {
		t.Fatalf("Query(%q): %v", q, err)
	}
	var out []string
	for _, e := range res.Entries {
		out = append(out, e.Entry.Title)
	}
	return out
}

func TestQueryStoreFilter(t *testing.T) {
	set := twoStoreSet(t)
	tests := []struct {
		q    string
		want []string
	}{
		{"", []string{"a-todo", "a-review", "a-project", "b-wip", "b-todo", "b-epic"}},
		{"store:org-a kind:task", []string{"a-todo", "a-review"}},
		{"-store:org-a kind:task", []string{"b-wip", "b-todo"}},
		{"store:org-b kind:epic", []string{"b-epic"}},
		{"store:org-a -store:org-a", nil},
	}
	for _, tt := range tests {
		// the order within a store is the order of the ranks, so compare as sets of lines
		got := queryTitles(t, set, tt.q, "")
		if !sameSet(got, tt.want) {
			t.Errorf("%q = %v; want %v", tt.q, got, tt.want)
		}
	}
	// the stores come in config order whatever the order within them
	if got := queryTitles(t, set, "", ""); !strings.HasPrefix(got[0], "a-") || !strings.HasPrefix(got[5], "b-") {
		t.Errorf("without sort: = %v; want the stores in order", got)
	}
	if got := queryTitles(t, set, "store:org-a", "org-b"); got != nil {
		t.Errorf("--store org-b with store:org-a = %v; want nothing", got)
	}
	if got := queryTitles(t, set, "kind:task", "org-b"); !sameSet(got, []string{"b-wip", "b-todo"}) {
		t.Errorf("only org-b = %v", got)
	}
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]int{}
	for _, s := range a {
		m[s]++
	}
	for _, s := range b {
		m[s]--
	}
	for _, n := range m {
		if n != 0 {
			return false
		}
	}
	return true
}

func TestQueryUnknownStore(t *testing.T) {
	set := twoStoreSet(t)
	for _, q := range []string{"store:nope", "-store:nope", "store:org-a,nope"} {
		parsed, _ := query.Parse(q)
		_, err := Query(set, parsed, "", noOptions)
		var qe *query.Error
		if !errors.As(err, &qe) || !strings.Contains(qe.Msg, `unknown store "nope"`) {
			t.Errorf("%q: err = %v; want unknown store \"nope\"", q, err)
		}
	}
	parsed, _ := query.Parse("store>org-a")
	if _, err := Query(set, parsed, "", noOptions); err == nil {
		t.Error("store>org-a: no error")
	}
}

func TestQueryUnknownProperty(t *testing.T) {
	set := twoStoreSet(t)
	// only org-a has size
	if got := queryTitles(t, set, "size>=5", ""); !sameSet(got, []string{"a-todo", "a-review"}) {
		t.Errorf("size>=5 = %v; want org-a's", got)
	}
	if got := queryTitles(t, set, "size:5 sort:size", ""); !sameSet(got, []string{"a-todo"}) {
		t.Errorf("size:5 = %v", got)
	}
	// nobody has zzz: the error of the query
	parsed, _ := query.Parse("zzz:1")
	_, err := Query(set, parsed, "", noOptions)
	var qe *query.Error
	if !errors.As(err, &qe) || !qe.Unknown {
		t.Errorf("zzz:1: err = %v; want an unknown key", err)
	}
	// the one store that has it is the only one asked
	if got := queryTitles(t, set, "size>=5", "org-a"); len(got) != 2 {
		t.Errorf("size>=5 on org-a = %v", got)
	}
	// a store that has the property but not the value is an error, not a miss
	parsed, _ = query.Parse("status:review")
	if _, err = Query(set, parsed, "", noOptions); err == nil {
		t.Error("status:review: no error, but org-b has no review status")
	}
}

func TestQuerySortStatusByCategoryThenPlaceThenStore(t *testing.T) {
	set := twoStoreSet(t)
	// the todo category first (place 0 in both: store order), then the
	// doing one: b-wip (place 1) before a-review (place 2)
	want := []string{"a-todo", "a-project", "b-todo", "b-epic", "b-wip", "a-review"}
	if got := queryTitles(t, set, "sort:status", ""); !reflect.DeepEqual(got, want) {
		t.Fatalf("sort:status = %v; want %v", got, want)
	}
	desc := queryTitles(t, set, "sort:-status", "")
	if desc[0] != "a-review" || desc[1] != "b-wip" {
		t.Errorf("sort:-status = %v; want the doing tasks first, a-review first", desc)
	}
}

func TestQuerySortKindByRoleThenPlaceThenStore(t *testing.T) {
	set := twoStoreSet(t)
	got := queryTitles(t, set, "status:todo sort:kind", "")
	// containers first: a-project (place 0), b-epic (place 1); then the work: b-todo (task, place 0), a-todo (task, place 1)
	want := []string{"a-project", "b-epic", "b-todo", "a-todo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sort:kind = %v; want %v", got, want)
	}
}

func TestQuerySortEnumByPlaceOfEachSchema(t *testing.T) {
	set := twoStoreSet(t)
	// tier: gold is place 0 in org-a and 1 in org-b
	got := queryTitles(t, set, "tier:gold,silver sort:tier", "")
	// b-wip is silver at place 0; b-todo (gold) and a-todo (silver) are at
	// place 1, which the value tells apart
	want := []string{"b-wip", "b-todo", "a-todo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sort:tier = %v; want %v", got, want)
	}
}

func TestQuerySortTiesAreInStoreOrder(t *testing.T) {
	a, b := twoSchemas(t)
	putSpecs(t, b, spec{title: "same", kind: "task", status: "todo"}, spec{title: "same", kind: "task", status: "todo"})
	putSpecs(t, a, spec{title: "same", kind: "task", status: "todo"})
	set := openSet(t, a, b)
	parsed, _ := query.Parse("sort:title")
	res, err := Query(set, parsed, "", noOptions)
	if err != nil {
		t.Fatal(err)
	}
	var stores []string
	for _, e := range res.Entries {
		stores = append(stores, e.Store.Name)
	}
	if !reflect.DeepEqual(stores, []string{"org-a", "org-b", "org-b"}) {
		t.Errorf("stores = %v; want ties in store order", stores)
	}
}

func TestQueryGroupJoinsAcrossStores(t *testing.T) {
	set := twoStoreSet(t)
	parsed, _ := query.Parse("group:status")
	res, err := Query(set, parsed, "", noOptions)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	sizes := map[string]int{}
	for _, g := range res.Groups {
		keys = append(keys, g.Key)
		sizes[g.Key] = len(g.Entries)
	}
	// org-a's groups come first (todo, review), then org-b's new one (wip)
	if want := []string{"todo", "review", "wip"}; !reflect.DeepEqual(keys, want) {
		t.Errorf("group keys = %v; want %v", keys, want)
	}
	if sizes["todo"] != 4 {
		t.Errorf("todo has %d tasks; want 4 from both stores", sizes["todo"])
	}
	if len(res.Entries) != 6 {
		t.Errorf("entries = %d; want the groups' tasks in order, 6", len(res.Entries))
	}
	if len(res.Stores) != 2 {
		t.Errorf("stores = %d", len(res.Stores))
	}
}

func TestQueryOpensOnlyTheStoresItNeeds(t *testing.T) {
	set := twoStoreSet(t)
	parsed, _ := query.Parse("kind:task")
	if _, err := Query(set, parsed, "org-b", noOptions); err != nil {
		t.Fatal(err)
	}
	if set.Stores[0].ix != nil {
		t.Error("org-a's index was opened for --store org-b")
	}
	parsed, _ = query.Parse("store:org-b")
	set2 := twoStoreSet(t)
	if _, err := Query(set2, parsed, "", noOptions); err != nil {
		t.Fatal(err)
	}
	if set2.Stores[0].ix != nil || set2.Stores[1].ix == nil {
		t.Error("store:org-b did not open org-b only")
	}
}

func TestQueryOptionsErrorAndUnknownStoreOnly(t *testing.T) {
	set := twoStoreSet(t)
	parsed, _ := query.Parse("")
	boom := errors.New("boom")
	if _, err := Query(set, parsed, "", func(*Mounted) (query.Options, error) { return query.Options{}, boom }); !errors.Is(err, boom) {
		t.Errorf("err = %v; want boom", err)
	}
	if _, err := Query(set, parsed, "nope", noOptions); err == nil {
		t.Error("--store nope: no error")
	}
}
