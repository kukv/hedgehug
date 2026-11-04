package stores_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// The store hedgehug picks without --store is marked: the only store is the
// one, and with several none is, as the CLI asks for --store then.
func TestStoreItemsMarkTheDefault(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	items, err := stores.StoreItems(ts.Session)
	if err != nil || len(items) != 1 || items[0].Name != "me" || !items[0].Default {
		t.Fatalf("StoreItems = %+v, %v; want me, which is the default", items, err)
	}
	if _, err := stores.StoreInit(ts.Session, stores.StoreInitInput{Name: "team", Prefix: "TM"}); err != nil {
		t.Fatal(err)
	}
	items, err = stores.StoreItems(ts.Session)
	if err != nil || len(items) != 2 || items[0].Default || items[1].Default {
		t.Fatalf("StoreItems with two stores = %+v, %v; want no default", items, err)
	}
	if !items[0].DefaultStore || items[1].DefaultStore {
		t.Errorf("default_store = %v, %v; want me", items[0].DefaultStore, items[1].DefaultStore)
	}
	if v, err := stores.ViewOf(ts.Session, "team"); err != nil || v.Name != "team" {
		t.Errorf("ViewOf = %+v, %v", v, err)
	}
	if _, err := stores.ViewOf(ts.Session, "nope"); err == nil {
		t.Error("ViewOf of a store that is not listed was no error")
	}
}

// The workspace of a store init is the one the store has, with the time of
// its prefix and the counts, and the arrays are never null.
func TestStoreInitViewOfReadsTheWorkspace(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	r, err := stores.StoreInit(ts.Session, stores.StoreInitInput{Name: "team", Prefix: "TM"})
	if err != nil {
		t.Fatal(err)
	}
	v := stores.StoreInitViewOf(ts.ForStore("team"), r)
	if v.Name != "team" || v.Path != r.Path || v.Remotes == nil || v.Pushed == nil {
		t.Errorf("view = %+v", v)
	}
	if v.Workspace.ULID != r.Workspace.ULID.String() || v.Workspace.IDPrefix != "TM" || v.Workspace.PrefixSet.IsZero() {
		t.Errorf("workspace = %+v; want the prefix with its time", v.Workspace)
	}
}

func TestDefaultViewOf(t *testing.T) {
	t.Parallel()
	if v := stores.DefaultViewOf(""); v.Name != nil {
		t.Errorf("no default = %+v", v)
	}
	if v := stores.DefaultViewOf("me"); v.Name == nil || *v.Name != "me" {
		t.Errorf("me = %+v", v)
	}
}

// A failed candidate is told by the type of its problem and its message.
func TestMountResultView(t *testing.T) {
	t.Parallel()
	v := stores.MountResult{}.View()
	if v.Mounted == nil || v.Failed == nil {
		t.Errorf("empty = %+v; want empty arrays", v)
	}
	r := stores.MountResult{
		Mounted: []string{"aa"},
		Failed: []stores.MountFailure{
			{Name: "bb", Err: usecase.NewErr(usecase.KindInvalid, "store.exists", errors.New("bb is there"))},
			{Name: "cc", Err: errors.New("boom")},
		},
	}.View()
	want := []stores.MountFailed{{Name: "bb", Code: "store.exists", Message: "bb is there"}, {Name: "cc", Code: "internal", Message: "boom"}}
	if len(r.Mounted) != 1 || len(r.Failed) != 2 || r.Failed[0] != want[0] || r.Failed[1] != want[1] {
		t.Errorf("view = %+v; want %+v", r, want)
	}
}
