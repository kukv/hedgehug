package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	hhctx "github.com/kukv/hedgehug/internal/context"
	"github.com/kukv/hedgehug/internal/model"
)

const useGoneWarning = "the project of hedgehug use is not found (hedgehug use --clear removes it)"

// addProjectIn adds a project to the store name and returns its id.
func addProjectIn(t *testing.T, ts *testSession, name, title string) string {
	t.Helper()
	r, err := ts.ForStore(name).Add(AddInput{Title: title, Kind: "project"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Index.Refresh(); err != nil {
		t.Fatal(err)
	}
	e, ok := r.Index.Task(r.ULID)
	if !ok {
		t.Fatalf("the project %s is not in the index", r.ULID)
	}
	return e.ID
}

func loadContext(t *testing.T, ts *testSession) hhctx.State {
	t.Helper()
	_, p, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	return hhctx.Load(p.DataDir, func(m string) { t.Errorf("context.json warned: %s", m) })
}

func TestUseSetsTheProjectOfAnyStore(t *testing.T) {
	ts := newTwoStoreSession(t)
	id := addProjectIn(t, ts, "org-a", "Release")
	r, err := ts.Use("org-a:" + id)
	if err != nil {
		t.Fatal(err)
	}
	if r.Store != "org-a" || r.Project == nil || r.Project.Title != "Release" || !r.Changed {
		t.Errorf("Use = %+v", r)
	}
	st := loadContext(t, ts)
	if st.Use == nil || st.Use.Project != r.Project.ULID {
		t.Fatalf("context.json use = %+v; want the project %s", st.Use, r.Project.ULID)
	}
	m, _ := ts.Mount()
	a, _ := m.Get("org-a")
	if st.Use.Store != a.ULID {
		t.Errorf("use.store = %s; want org-a's store ULID %s", st.Use.Store, a.ULID)
	}
	// without the store name, and again: nothing changes
	r, err = ts.Use(id)
	if err != nil || r.Changed {
		t.Errorf("Use again = %+v, %v; want Changed false", r, err)
	}
	// another project in another store changes it
	idB := addProjectIn(t, ts, "org-b", "Other")
	if r, err = ts.Use(idB); err != nil || !r.Changed || r.Store != "org-b" {
		t.Errorf("Use(%s) = %+v, %v", idB, r, err)
	}
}

func TestUseKeepsRecent(t *testing.T) {
	ts := newTwoStoreSession(t)
	_, p, _ := ts.Config()
	rec := hhctx.Ref{Store: model.NewULID(), Project: model.NewULID()}
	if err := hhctx.Save(p.DataDir, hhctx.State{Recent: map[string]hhctx.Ref{"github.com/x/y": rec}}); err != nil {
		t.Fatal(err)
	}
	id := addProjectIn(t, ts, "org-a", "Release")
	if _, err := ts.Use(id); err != nil {
		t.Fatal(err)
	}
	if got := loadContext(t, ts).Recent["github.com/x/y"]; got != rec {
		t.Errorf("recent = %+v; want it kept as %+v", got, rec)
	}
}

func TestUseRefusesWhatIsNotAProject(t *testing.T) {
	ts := newTwoStoreSession(t)
	_, err := ts.Use("org-a:API-1")
	wantCode(t, "Use(task)", err, KindInvalid, "use.not_project")
	if st := loadContext(t, ts); st.Use != nil {
		t.Errorf("use = %+v; want none", st.Use)
	}
	_, err = ts.Use("NOPE-9")
	wantCode(t, "Use(unknown)", err, KindNotFound, "task.not_found")
}

func TestUseRefusesADeletedProject(t *testing.T) {
	ts := newTwoStoreSession(t)
	id := addProjectIn(t, ts, "org-a", "Release")
	if _, err := ts.Remove(id); err != nil {
		t.Fatal(err)
	}
	_, err := ts.Use(id)
	wantCode(t, "Use(deleted)", err, KindNotFound, "task.not_found")
}

func TestUseAmbiguousAcrossStores(t *testing.T) {
	ts := newTwoStoreSessionWith(t, "SAME", "SAME")
	addProjectIn(t, ts, "org-a", "A")
	addProjectIn(t, ts, "org-b", "B")
	_, err := ts.Use("SAME-2")
	wantCode(t, "Use(ambiguous)", err, KindAmbiguous, "task.ambiguous")
}

func TestUseShow(t *testing.T) {
	ts := newTwoStoreSession(t)
	r, err := ts.UseShow()
	if err != nil || r.Project != nil || r.Store != "" {
		t.Fatalf("UseShow with none = %+v, %v", r, err)
	}
	id := addProjectIn(t, ts, "org-a", "Release")
	set, err := ts.Use(id)
	if err != nil {
		t.Fatal(err)
	}
	r, err = ts.UseShow()
	if err != nil || r.Project == nil || r.Project.ULID != set.Project.ULID || r.Store != "org-a" || r.Index == nil {
		t.Errorf("UseShow = %+v, %v", r, err)
	}
	if len(ts.warns) != 0 {
		t.Errorf("warns = %q", ts.warns)
	}
}

func TestUseShowWarnsOfAProjectThatIsGone(t *testing.T) {
	deleted := func(t *testing.T, ts *testSession, id string) { // deleted
		if _, err := ts.Remove(id); err != nil {
			t.Fatal(err)
		}
	}
	notProject := func(t *testing.T, ts *testSession, id string) { // kind changed
		if _, err := ts.Set(id, []Assign{{Key: "kind", Op: "=", Value: "task"}}); err != nil {
			t.Fatal(err)
		}
	}
	unmounted := func(t *testing.T, ts *testSession, _ string) { // the store is not in config.yaml
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
	missing := func(t *testing.T, ts *testSession, _ string) { // the use names a task that is not there
		_, p, _ := ts.Config()
		st := hhctx.Load(p.DataDir, nil)
		st.Use.Project = model.NewULID()
		if err := hhctx.Save(p.DataDir, st); err != nil {
			t.Fatal(err)
		}
	}
	for name, gone := range map[string]func(*testing.T, *testSession, string){
		"deleted": deleted, "not a project": notProject, "store not mounted": unmounted, "task not there": missing,
	} {
		t.Run(name, func(t *testing.T) {
			ts := newTwoStoreSession(t)
			id := addProjectIn(t, ts, "org-a", "Release")
			if _, err := ts.Use(id); err != nil {
				t.Fatal(err)
			}
			gone(t, ts, id)
			ts.mounts = nil // the stores are opened again, as a new run does
			r, err := ts.UseShow()
			if err != nil || r.Project != nil || r.Store != "" {
				t.Errorf("UseShow = %+v, %v; want none", r, err)
			}
			n := 0
			for _, w := range ts.warns {
				if w == useGoneWarning {
					n++
				}
			}
			if n != 1 {
				t.Errorf("warns = %q; want %q once", ts.warns, useGoneWarning)
			}
		})
	}
}

func TestUseClear(t *testing.T) {
	ts := newTwoStoreSession(t)
	r, err := ts.UseClear()
	if err != nil || r.Changed {
		t.Errorf("UseClear with none = %+v, %v; want Changed false", r, err)
	}
	if _, err := ts.Use(addProjectIn(t, ts, "org-a", "Release")); err != nil {
		t.Fatal(err)
	}
	r, err = ts.UseClear()
	if err != nil || !r.Changed || r.Project != nil {
		t.Errorf("UseClear = %+v, %v; want Changed", r, err)
	}
	if st := loadContext(t, ts); st.Use != nil {
		t.Errorf("use = %+v; want null", st.Use)
	}
	_, p, _ := ts.Config()
	b, err := os.ReadFile(filepath.Join(p.DataDir, "context.json"))
	if err != nil || !strings.Contains(string(b), `"use": null`) {
		t.Errorf("context.json = %s, %v", b, err)
	}
	if r, err = ts.UseClear(); err != nil || r.Changed {
		t.Errorf("UseClear again = %+v, %v; want Changed false", r, err)
	}
}
