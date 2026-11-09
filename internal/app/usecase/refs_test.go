package usecase_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/storage/mount/mounttest"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// commitTask writes task over its file and commits it directly, as a hand
// edit or a sync would, which the store's own writes refuse for a reference
// to a task it does not hold.
func commitTask(t *testing.T, e config.StoreEntry, task *model.Task) {
	t.Helper()
	s, err := store.Open(e.Path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	rel := model.TaskPath(task.ULID)
	if err := os.WriteFile(filepath.Join(e.Path, filepath.FromSlash(rel)), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: mounttest.Author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

func TestCrossRefsFindsParentAndTheFiveRelations(t *testing.T) {
	t.Parallel()
	a, b := mounttest.NewStoreAt(t, "org-a", "API"), mounttest.NewStoreAt(t, "org-b", "WEB")
	target := mounttest.PutTasks(t, b, 1, nil)[0]
	fields := []string{"parent"}
	for _, rel := range schema.Relations {
		fields = append(fields, rel.Name)
	}
	from := mounttest.PutTasks(t, a, len(fields), nil)
	from[0].Parent = &target.ULID
	for i, rel := range schema.Relations {
		rel.Set(from[i+1], []model.ULID{target.ULID})
	}
	for _, x := range from {
		commitTask(t, a, x)
	}
	set := usecasetest.OpenSet(t, a, b)
	for _, name := range []string{"", "org-a"} {
		got, err := usecase.CrossRefs(set, name)
		if err != nil || len(got) != len(fields) {
			t.Fatalf("CrossRefs(%q) = %+v, %v; want %d", name, got, err, len(fields))
		}
		for i, field := range fields {
			g := got[i]
			if g.Field != field || g.From.ULID != from[i].ULID || g.From.ID != from[i].ID || g.From.Title != from[i].Title || g.From.Store.Name != "org-a" ||
				g.Target.ULID != target.ULID || g.Target.ID != "WEB-1" || g.Target.RawID() != "WEB-1" || g.Target.Display != "WEB-1" || g.Target.Store.Name != "org-b" {
				t.Errorf("CrossRefs(%q)[%d] = %+v; want %s of %s to org-b's WEB-1", name, i, g, field, from[i].ID)
			}
		}
	}
	if got, err := usecase.CrossRefs(set, "org-b"); err != nil || len(got) != 0 {
		t.Errorf("CrossRefs(org-b) = %+v, %v; want none: org-b refers to nothing", got, err)
	}
}

func TestCrossRefsIgnoresWhatIsNotAReferenceAcrossStores(t *testing.T) {
	t.Parallel()
	a, b := mounttest.NewStoreAt(t, "org-a", "API"), mounttest.NewStoreAt(t, "org-b", "WEB")
	gone := model.NewULID()
	deleted := mounttest.PutTasks(t, b, 1, nil)[0]
	now := time.Now()
	deleted.DeletedAt = &now
	commitTask(t, b, deleted)
	own := mounttest.PutTasks(t, a, 1, nil)[0]
	tasks := mounttest.PutTasks(t, a, 4, nil)
	tasks[0].Parent = &gone                         // no store holds it
	tasks[1].BlockedBy = []model.ULID{deleted.ULID} // deleted in the other store
	tasks[2].Parent = &own.ULID                     // in its own store
	tasks[3].BlockedBy = []model.ULID{deleted.ULID}
	now = time.Now()
	tasks[3].DeletedAt = &now // a deleted task is not looked at
	for _, x := range tasks {
		commitTask(t, a, x)
	}
	got, err := usecase.CrossRefs(usecasetest.OpenSet(t, a, b), "")
	if err != nil || len(got) != 0 {
		t.Errorf("CrossRefs = %+v, %v; want none", got, err)
	}
}

func TestCrossRefsSeesAWriteAfterTheIndexWasOpened(t *testing.T) {
	t.Parallel()
	a, b := mounttest.NewStoreAt(t, "org-a", "API"), mounttest.NewStoreAt(t, "org-b", "WEB")
	target := mounttest.PutTasks(t, b, 1, nil)[0]
	from := mounttest.PutTasks(t, a, 1, nil)[0]
	set := usecasetest.OpenSet(t, a, b)
	if got, err := usecase.CrossRefs(set, ""); err != nil || len(got) != 0 {
		t.Fatalf("before = %+v, %v", got, err)
	}
	from.Parent = &target.ULID
	commitTask(t, a, from)
	if got, err := usecase.CrossRefs(set, ""); err != nil || len(got) != 1 {
		t.Errorf("after = %+v, %v; want 1", got, err)
	}
}

func TestCrossRefsUnknownStore(t *testing.T) {
	t.Parallel()
	if got, err := usecase.CrossRefs(usecasetest.OpenSet(t, mounttest.NewStoreAt(t, "org-a", "API")), "nope"); err != nil || len(got) != 0 {
		t.Errorf("CrossRefs(nope) = %+v, %v; want none", got, err)
	}
}
