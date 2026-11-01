package mount

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// newRefStoreAt is newStoreAt with a task_ref property "rel" in the schema.
func newRefStoreAt(t *testing.T, name, prefix string) config.StoreEntry {
	t.Helper()
	sc := schema.Default()
	sc.Props = append(sc.Props, schema.Prop{Name: "rel", Type: schema.TypeTaskRef})
	dir := filepath.Join(t.TempDir(), name)
	ws := &model.Workspace{ULID: model.NewULID(), Name: name, IDPrefix: prefix}
	if _, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: name}, sc, ws, testAuthor, "", time.Now); err != nil {
		t.Fatal(err)
	}
	return config.StoreEntry{Name: name, Path: dir}
}

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
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: testAuthor, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

func scalar(v string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Value: v} }

func TestCrossRefsFindsParentBlockedByAndTaskRef(t *testing.T) {
	t.Parallel()
	a, b := newRefStoreAt(t, "org-a", "API"), newRefStoreAt(t, "org-b", "WEB")
	target := putTasks(t, b, 1, nil)[0]
	from := putTasks(t, a, 3, nil)
	from[0].Parent = &target.ULID
	from[1].BlockedBy = []model.ULID{target.ULID}
	from[2].Props = model.Fields{{Key: "rel", Value: scalar(target.ULID.String())}}
	for _, x := range from {
		commitTask(t, a, x)
	}
	set := openSet(t, a, b)
	for _, name := range []string{"", "org-a"} {
		got, err := CrossRefs(set, name)
		if err != nil || len(got) != 3 {
			t.Fatalf("CrossRefs(%q) = %+v, %v; want 3", name, got, err)
		}
		for i, field := range []string{"parent", "blocked_by", "rel"} {
			g := got[i]
			if g.Field != field || g.From.ULID != from[i].ULID || g.From.ID != from[i].ID || g.From.Title != from[i].Title || g.From.Store.Name != "org-a" ||
				g.Target.ULID != target.ULID || g.Target.ID != "WEB-1" || g.Target.RawID() != "WEB-1" || g.Target.Display != "WEB-1" || g.Target.Store.Name != "org-b" {
				t.Errorf("CrossRefs(%q)[%d] = %+v; want %s of %s to org-b's WEB-1", name, i, g, field, from[i].ID)
			}
		}
	}
	if got, err := CrossRefs(set, "org-b"); err != nil || len(got) != 0 {
		t.Errorf("CrossRefs(org-b) = %+v, %v; want none: org-b refers to nothing", got, err)
	}
}

func TestCrossRefsIgnoresWhatIsNotAReferenceAcrossStores(t *testing.T) {
	t.Parallel()
	a, b := newRefStoreAt(t, "org-a", "API"), newRefStoreAt(t, "org-b", "WEB")
	gone := model.NewULID()
	deleted := putTasks(t, b, 1, nil)[0]
	now := time.Now()
	deleted.DeletedAt = &now
	commitTask(t, b, deleted)
	own := putTasks(t, a, 1, nil)[0]
	tasks := putTasks(t, a, 4, nil)
	tasks[0].Parent = &gone                         // no store holds it
	tasks[1].BlockedBy = []model.ULID{deleted.ULID} // deleted in the other store
	tasks[2].Parent = &own.ULID                     // in its own store
	tasks[3].BlockedBy = []model.ULID{deleted.ULID}
	now = time.Now()
	tasks[3].DeletedAt = &now // a deleted task is not looked at
	for _, x := range tasks {
		commitTask(t, a, x)
	}
	got, err := CrossRefs(openSet(t, a, b), "")
	if err != nil || len(got) != 0 {
		t.Errorf("CrossRefs = %+v, %v; want none", got, err)
	}
}

func TestCrossRefsSeesAWriteAfterTheIndexWasOpened(t *testing.T) {
	t.Parallel()
	a, b := newRefStoreAt(t, "org-a", "API"), newRefStoreAt(t, "org-b", "WEB")
	target := putTasks(t, b, 1, nil)[0]
	from := putTasks(t, a, 1, nil)[0]
	set := openSet(t, a, b)
	if got, err := CrossRefs(set, ""); err != nil || len(got) != 0 {
		t.Fatalf("before = %+v, %v", got, err)
	}
	from.Parent = &target.ULID
	commitTask(t, a, from)
	if got, err := CrossRefs(set, ""); err != nil || len(got) != 1 {
		t.Errorf("after = %+v, %v; want 1", got, err)
	}
}

func TestCrossRefsUnknownStore(t *testing.T) {
	t.Parallel()
	if got, err := CrossRefs(openSet(t, newStoreAt(t, "org-a", "API")), "nope"); err != nil || len(got) != 0 {
		t.Errorf("CrossRefs(nope) = %+v, %v; want none", got, err)
	}
}
