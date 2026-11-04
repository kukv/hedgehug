package stores_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestStoreSet(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	teal, orange, none := "teal", "orange", ""
	yes, no := true, false

	v, err := stores.StoreSet(ts.Session, "org-a", &teal, nil)
	if err != nil || v.Name != "org-a" || v.Color != "teal" || v.Hidden {
		t.Fatalf("color: %+v, %v", v, err)
	}
	// only what is given changes
	v, err = stores.StoreSet(ts.Session, "org-a", nil, &yes)
	if err != nil || v.Color != "teal" || !v.Hidden {
		t.Fatalf("hidden: %+v, %v", v, err)
	}
	list, err := stores.StoreList(ts.Session)
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if a, b := list[0], list[1]; a.Color != "teal" || !a.Hidden || b.Color != "" || b.Hidden {
		t.Errorf("list = %+v", list)
	}
	if v, err = stores.StoreSet(ts.Session, "org-b", &orange, &yes); err != nil || v.Color != "orange" || !v.Hidden {
		t.Fatalf("both: %+v, %v", v, err)
	}
	// an empty color and hidden=false take the keys out of config.yaml
	if v, err = stores.StoreSet(ts.Session, "org-a", &none, &no); err != nil || v.Color != "" || v.Hidden {
		t.Fatalf("reset: %+v, %v", v, err)
	}
	_, p, _ := ts.Config()
	b, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(b), "color:") != 1 || strings.Count(string(b), "hidden:") != 1 { // org-b's
		t.Errorf("config.yaml:\n%s", b)
	}
}

func TestStoreSetWritesNothingWhenUnchanged(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	teal := "teal"
	no := false
	if _, err := stores.StoreSet(ts.Session, "org-a", &teal, nil); err != nil {
		t.Fatal(err)
	}
	ts.BeforeUpdate = func() { t.Error("config.yaml is updated for the same values") }
	if v, err := stores.StoreSet(ts.Session, "org-a", &teal, &no); err != nil || v.Color != "teal" || v.Hidden {
		t.Errorf("same values: %+v, %v", v, err)
	}
	if _, err := stores.StoreSet(ts.Session, "org-a", nil, nil); err != nil {
		t.Errorf("nothing given: %v", err)
	}
}

func TestStoreSetErrors(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	ts.BeforeUpdate = func() { t.Error("a refused call updates config.yaml") }
	red, grey, teal := "red", "grey", "teal"

	_, err := stores.StoreSet(ts.Session, "org-a", &red, nil)
	usecasetest.WantErr(t, err, usecase.KindInvalid, "store.bad_color")
	if e := usecase.AsError(err); e.Args["value"] != "red" || len(e.Args["candidates"].([]string)) != 7 {
		t.Errorf("args = %v", e.Args)
	}
	// grey is how a read-only store is drawn, not a color to pick
	_, err = stores.StoreSet(ts.Session, "org-a", &grey, nil)
	usecasetest.WantErr(t, err, usecase.KindInvalid, "store.bad_color")

	_, err = stores.StoreSet(ts.Session, "nope", &teal, nil)
	usecasetest.WantErr(t, err, usecase.KindNotFound, "store.not_found")
	if !strings.Contains(err.Error(), "org-a, org-b") {
		t.Errorf("message = %v", err)
	}
	if e := usecase.AsError(err); e.Args["name"] != "nope" || len(e.Args["candidates"].([]string)) != 2 {
		t.Errorf("args = %v", e.Args)
	}
}

func TestStoreViewReadOnly(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	usecasetest.MakeStoreReadOnly(t, ts, "org-b")
	list, err := stores.StoreList(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	if list[0].ReadOnly != "" || list[1].ReadOnly != "test" { // Access.Why of the reason "test"
		t.Errorf("read_only = %q, %q", list[0].ReadOnly, list[1].ReadOnly)
	}
	// StoreSet returns the view with the reason as well
	teal := "teal"
	if v, err := stores.StoreSet(ts.Session, "org-b", &teal, nil); err != nil || v.ReadOnly != "test" {
		t.Errorf("StoreSet: %+v, %v", v, err)
	}
	// a broken access.json is a reason too, in the words of SyncStatus
	s, err := ts.ForStore("org-a").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteLocal("access.json", []byte("{not json")); err != nil {
		t.Fatal(err)
	}
	list, err = stores.StoreList(ts.Session)
	if err != nil || list[0].ReadOnly != "access.json is broken; run hedgehug sync" {
		t.Errorf("broken: %+v, %v", list, err)
	}
}

func TestStoreViewJSON(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	list, err := stores.StoreList(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(list[0])
	if err != nil {
		t.Fatal(err)
	}
	// hidden is always there; color and read_only are left out when empty
	if !strings.Contains(string(b), `"hidden":false`) || strings.Contains(string(b), "color") || strings.Contains(string(b), "read_only") {
		t.Errorf("json = %s", b)
	}
}

// default_store of config.yaml is on the store it names and on no other.
func TestStoreListDefaultStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	if _, err := stores.StoreDefault(ts.Session, "org-b"); err != nil {
		t.Fatal(err)
	}
	list, err := stores.StoreList(ts.Session)
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	for _, v := range list {
		if v.DefaultStore != (v.Name == "org-b") {
			t.Errorf("%s: default_store = %v", v.Name, v.DefaultStore)
		}
	}
	// the store write responses carry it too
	teal := "teal"
	if v, err := stores.StoreSet(ts.Session, "org-b", &teal, nil); err != nil || !v.DefaultStore {
		t.Errorf("StoreSet(org-b) = %+v, %v", v, err)
	}
}
