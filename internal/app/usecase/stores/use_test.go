package stores_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/current"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestUseSetsTheProjectOfAnyStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	id := usecasetest.AddProjectIn(t, ts, "org-a", "Release")
	r, err := stores.Use(ts.Session, "org-a:"+id)
	if err != nil {
		t.Fatal(err)
	}
	if r.Store != "org-a" || r.Project == nil || r.Project.Title != "Release" || !r.Changed {
		t.Errorf("Use = %+v", r)
	}
	st := usecasetest.LoadContext(t, ts)
	if st.Use == nil || st.Use.Project != r.Project.ULID {
		t.Fatalf("context.json use = %+v; want the project %s", st.Use, r.Project.ULID)
	}
	m, _ := ts.Mount()
	a, _ := m.Get("org-a")
	if st.Use.Store != a.ULID {
		t.Errorf("use.store = %s; want org-a's store ULID %s", st.Use.Store, a.ULID)
	}
	// without the store name, and again: nothing changes
	r, err = stores.Use(ts.Session, id)
	if err != nil || r.Changed {
		t.Errorf("Use again = %+v, %v; want Changed false", r, err)
	}
	// another project in another store changes it
	idB := usecasetest.AddProjectIn(t, ts, "org-b", "Other")
	if r, err = stores.Use(ts.Session, idB); err != nil || !r.Changed || r.Store != "org-b" {
		t.Errorf("Use(%s) = %+v, %v", idB, r, err)
	}
}

func TestUseKeepsRecent(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	_, p, _ := ts.Config()
	rec := current.Ref{Store: model.NewULID(), Project: model.NewULID()}
	if err := current.Save(p.DataDir, current.State{Recent: map[string]current.Ref{"github.com/x/y": rec}}); err != nil {
		t.Fatal(err)
	}
	id := usecasetest.AddProjectIn(t, ts, "org-a", "Release")
	if _, err := stores.Use(ts.Session, id); err != nil {
		t.Fatal(err)
	}
	if got := usecasetest.LoadContext(t, ts).Recent["github.com/x/y"]; got != rec {
		t.Errorf("recent = %+v; want it kept as %+v", got, rec)
	}
}

func TestUseRefusesWhatIsNotAProject(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	_, err := stores.Use(ts.Session, "org-a:API-1")
	usecasetest.WantCode(t, "Use(task)", err, usecase.KindInvalid, "use.not_project")
	if st := usecasetest.LoadContext(t, ts); st.Use != nil {
		t.Errorf("use = %+v; want none", st.Use)
	}
	_, err = stores.Use(ts.Session, "NOPE-9")
	usecasetest.WantCode(t, "Use(unknown)", err, usecase.KindNotFound, "task.not_found")
}

func TestUseRefusesADeletedProject(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	id := usecasetest.AddProjectIn(t, ts, "org-a", "Release")
	if _, err := tasks.Remove(ts.Session, id); err != nil {
		t.Fatal(err)
	}
	_, err := stores.Use(ts.Session, id)
	usecasetest.WantCode(t, "Use(deleted)", err, usecase.KindConflict, "task.deleted")
	if err == nil || !strings.Contains(err.Error(), "restore") {
		t.Errorf("Use(deleted) = %v; want a hint to restore", err)
	}
}

func TestUseAmbiguousAcrossStores(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStoreWith(t, "SAME", "SAME")
	usecasetest.AddProjectIn(t, ts, "org-a", "A")
	usecasetest.AddProjectIn(t, ts, "org-b", "B")
	_, err := stores.Use(ts.Session, "SAME-2")
	usecasetest.WantCode(t, "Use(ambiguous)", err, usecase.KindAmbiguous, "task.ambiguous")
}

func TestUseShow(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	r, err := stores.UseShow(ts.Session)
	if err != nil || r.Project != nil || r.Store != "" {
		t.Fatalf("UseShow with none = %+v, %v", r, err)
	}
	id := usecasetest.AddProjectIn(t, ts, "org-a", "Release")
	set, err := stores.Use(ts.Session, id)
	if err != nil {
		t.Fatal(err)
	}
	r, err = stores.UseShow(ts.Session)
	if err != nil || r.Project == nil || r.Project.ULID != set.Project.ULID || r.Store != "org-a" || r.ID != id {
		t.Errorf("UseShow = %+v, %v", r, err)
	}
	if len(ts.Warns) != 0 {
		t.Errorf("warns = %q", ts.Warns)
	}
}

func TestUseShowWarnsOfAProjectThatIsGone(t *testing.T) {
	t.Parallel()
	deleted := func(t *testing.T, ts *usecasetest.Session, id string) { // deleted
		if _, err := tasks.Remove(ts.Session, id); err != nil {
			t.Fatal(err)
		}
	}
	notProject := func(t *testing.T, ts *usecasetest.Session, id string) { // kind changed
		if _, err := tasks.Set(ts.Session, id, []usecase.Assign{{Key: "kind", Op: "=", Value: "task"}}); err != nil {
			t.Fatal(err)
		}
	}
	unmounted := func(t *testing.T, ts *usecasetest.Session, _ string) { // the store is not in config.yaml
		c, _, err := ts.Config()
		if err != nil {
			t.Fatal(err)
		}
		if err := c.RemoveStore("org-a"); err != nil {
			t.Fatal(err)
		}
		if err := c.Save(); err != nil {
			t.Fatal(err)
		}
	}
	missing := func(t *testing.T, ts *usecasetest.Session, _ string) { // the use names a task that is not there
		_, p, _ := ts.Config()
		st := current.Load(p.DataDir, nil)
		st.Use.Project = model.NewULID()
		if err := current.Save(p.DataDir, st); err != nil {
			t.Fatal(err)
		}
	}
	for name, gone := range map[string]func(*testing.T, *usecasetest.Session, string){
		"deleted": deleted, "not a project": notProject, "store not mounted": unmounted, "task not there": missing,
	} {
		t.Run(name, func(t *testing.T) {
			ts := usecasetest.NewTwoStore(t)
			id := usecasetest.AddProjectIn(t, ts, "org-a", "Release")
			if _, err := stores.Use(ts.Session, id); err != nil {
				t.Fatal(err)
			}
			gone(t, ts, id)
			ts.Renew() // the stores are opened again, as a new run does
			r, err := stores.UseShow(ts.Session)
			if err != nil || r.Project != nil || r.Store != "" {
				t.Errorf("UseShow = %+v, %v; want none", r, err)
			}
			n := 0
			for _, w := range ts.Warns {
				if w == usecasetest.UseGoneWarning {
					n++
				}
			}
			if n != 1 {
				t.Errorf("warns = %q; want %q once", ts.Warns, usecasetest.UseGoneWarning)
			}
		})
	}
}

func TestUseClear(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	r, err := stores.UseClear(ts.Session)
	if err != nil || r.Changed {
		t.Errorf("UseClear with none = %+v, %v; want Changed false", r, err)
	}
	if _, err := stores.Use(ts.Session, usecasetest.AddProjectIn(t, ts, "org-a", "Release")); err != nil {
		t.Fatal(err)
	}
	r, err = stores.UseClear(ts.Session)
	if err != nil || !r.Changed || r.Project != nil {
		t.Errorf("UseClear = %+v, %v; want Changed", r, err)
	}
	if st := usecasetest.LoadContext(t, ts); st.Use != nil {
		t.Errorf("use = %+v; want null", st.Use)
	}
	_, p, _ := ts.Config()
	b, err := os.ReadFile(filepath.Join(p.DataDir, "context.json"))
	if err != nil || !strings.Contains(string(b), `"use": null`) {
		t.Errorf("context.json = %s, %v", b, err)
	}
	if r, err = stores.UseClear(ts.Session); err != nil || r.Changed {
		t.Errorf("UseClear again = %+v, %v; want Changed false", r, err)
	}
}

// hh use --store looks in that store only.
func TestUseWithStoreLooksInThatStoreOnly(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStoreWith(t, "API", "API")
	idA := usecasetest.AddProjectIn(t, ts, "org-a", "Release A")
	idB := usecasetest.AddProjectIn(t, ts, "org-b", "Release B")
	if idA != idB {
		t.Fatalf("ids %s and %s; want the same id in both stores", idA, idB)
	}
	// without --store the id is in both
	if _, err := stores.Use(ts.Session, idA); usecase.KindOf(err) != usecase.KindAmbiguous {
		t.Fatalf("Use(%s) err = %v; want ambiguous", idA, err)
	}
	ts.StoreName = "org-b"
	r, err := stores.Use(ts.Session, idA)
	if err != nil || r.Store != "org-b" || r.Project.Title != "Release B" {
		t.Fatalf("Use(%s) --store org-b = %+v, %v; want org-b's project", idA, r, err)
	}
	// the same store spelled in the id is fine
	if r, err = stores.Use(ts.Session, "org-b:"+idA); err != nil || r.Store != "org-b" {
		t.Errorf("Use(org-b:%s) --store org-b = %+v, %v", idA, r, err)
	}
	// another store's name is a usage error
	_, err = stores.Use(ts.Session, "org-a:"+idA)
	usecasetest.WantCode(t, "Use(org-a:...) --store org-b", err, usecase.KindUsage, "store.conflict")
	// a project that is only in the other store is not found
	ts.StoreName = "org-a"
	other := usecasetest.AddProjectIn(t, ts, "org-b", "Only B")
	_, err = stores.Use(ts.Session, other)
	usecasetest.WantCode(t, "Use(project of org-b) --store org-a", err, usecase.KindNotFound, "task.not_found")
	ts.StoreName = "nope"
	_, err = stores.Use(ts.Session, idA)
	usecasetest.WantCode(t, "Use --store nope", err, usecase.KindNotFound, "store.not_found")
}

// An ambiguity found by hh use --store names the input as the user typed it.
func TestUseWithStoreAmbiguityKeepsTheInput(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	s, err := ts.ForStore("org-a").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	_, ix, err := ts.ForStore("org-a").Open()
	if err != nil {
		t.Fatal(err)
	}
	ws := ix.Workspaces()[0].ULID
	for _, title := range []string{"one", "two"} {
		task := &model.Task{ULID: model.NewULID(), Title: title, Kind: "project", Status: "todo", Workspace: ws, FormerIDs: []string{"OLD-1"}}
		if _, err := ts.ForStore("org-a").Update(s, store.Op{Name: "add", Task: task.ULID}, func(tx *store.Tx) error { return tx.PutTask(task) }); err != nil {
			t.Fatal(err)
		}
	}
	ts.StoreName = "org-a"
	_, err = stores.Use(ts.Session, "OLD-1")
	usecasetest.WantCode(t, "Use(OLD-1) --store org-a", err, usecase.KindAmbiguous, "task.ambiguous")
	if e := usecase.AsError(err); e == nil || e.Args["input"] != "OLD-1" || strings.Contains(err.Error(), "org-a:OLD-1") {
		t.Errorf("err = %v; want the input OLD-1 as typed", err)
	}
}
