package store

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

const (
	taskA = "01JBZ8K3QW7M2X9V4T6R5N8P0A"
	taskB = "01JBZ8K3QW7M2X9V4T6R5N8P0B"
	cmt1  = "01JBZ8K3QW7M2X9V4T6R5N8P1A"
	cmt2  = "01JBZ8K3QW7M2X9V4T6R5N8P1B"
)

func id(t *testing.T, s string) model.ID {
	t.Helper()
	v, err := model.ParseID(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func taskFile(id, title string) string {
	return "---\nid: " + id + "\nkind: task\ntitle: " + title + "\nstatus: todo\n---\n"
}

func TestReadTask(t *testing.T) {
	s := newTestStore(t)
	a := id(t, taskA)
	mustWrite(t, s, model.TaskPath(a), taskFile(taskA, "A"))

	got, err := s.Task(a)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "A" {
		t.Errorf("title = %q", got.Title)
	}
	if _, err := s.Task(id(t, taskB)); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing task: err = %v, want ErrNotFound", err)
	}
}

func TestReadDecodeError(t *testing.T) {
	s := newTestStore(t)
	a := id(t, taskA)
	mustWrite(t, s, model.TaskPath(a), "---\ntitle: no id\n---\n")
	_, err := s.Task(a)
	var de *model.DecodeError
	if !errors.As(err, &de) {
		t.Errorf("err = %v, want a *model.DecodeError", err)
	}
}

func TestTasksSkipsOtherFiles(t *testing.T) {
	s := newTestStore(t)
	a, b := id(t, taskA), id(t, taskB)
	mustWrite(t, s, model.TaskPath(a), taskFile(taskA, "A"))
	mustWrite(t, s, model.TaskPath(b), taskFile(taskB, "B"))
	mustWrite(t, s, "tasks/README.md", "not a task")
	mustWrite(t, s, model.TaskPath(a)+".tmp-123", "a temporary file")

	var titles []string
	for task, err := range s.Tasks() {
		if err != nil {
			t.Fatal(err)
		}
		titles = append(titles, task.Title)
	}
	slices.Sort(titles)
	if !slices.Equal(titles, []string{"A", "B"}) {
		t.Errorf("titles = %v", titles)
	}
}

// Tasks yields a FileError naming the path for a file that does not decode
// or that holds a task of another id, and no task for it.
func TestTasksBrokenFiles(t *testing.T) {
	s := newTestStore(t)
	a, b := id(t, taskA), id(t, taskB)
	mustWrite(t, s, model.TaskPath(a), taskFile(taskA, "A"))
	mustWrite(t, s, model.TaskPath(b), taskFile(taskA, "stale copy of A"))
	c := id(t, cmt1)
	mustWrite(t, s, model.TaskPath(c), "not a task\n")

	var titles []string
	bad := map[string]error{}
	for task, err := range s.Tasks() {
		if err != nil {
			var fe *FileError
			if !errors.As(err, &fe) {
				t.Fatalf("err = %v, want a *FileError", err)
			}
			if task != nil {
				t.Errorf("%s: task = %+v, want nil", fe.Path, task)
			}
			bad[fe.Path] = fe.Err
			continue
		}
		titles = append(titles, task.Title)
	}
	if !slices.Equal(titles, []string{"A"}) {
		t.Errorf("titles = %v, want [A]", titles)
	}
	if err := bad[model.TaskPath(b)]; !errors.Is(err, ErrIDMismatch) {
		t.Errorf("stale copy: err = %v, want ErrIDMismatch", err)
	}
	if err := bad[model.TaskPath(c)]; !errors.As(err, new(*model.DecodeError)) {
		t.Errorf("not a task: err = %v, want a *model.DecodeError", err)
	}
	if len(bad) != 2 {
		t.Errorf("errors = %v, want two", bad)
	}
}

func TestTasksEmptyStore(t *testing.T) {
	s := newTestStore(t)
	for _, err := range s.Tasks() {
		t.Fatalf("unexpected item, err = %v", err)
	}
}

func TestComments(t *testing.T) {
	s := newTestStore(t)
	a, b := id(t, taskA), id(t, taskB)
	c1, c2 := id(t, cmt1), id(t, cmt2)
	comment := func(id string) string { return "---\nid: " + id + "\nauthor: github.com/me\n---\nbody\n" }
	mustWrite(t, s, model.CommentPath(a, c1), comment(cmt1))
	mustWrite(t, s, model.CommentPath(a, c2), comment(cmt2))
	mustWrite(t, s, model.CommentPath(b, c1), comment(cmt1))

	n := 0
	for c, err := range s.Comments(a) {
		if err != nil {
			t.Fatal(err)
		}
		if c.ID != c1 && c.ID != c2 {
			t.Errorf("unexpected comment %s", c.ID)
		}
		n++
	}
	if n != 2 {
		t.Errorf("comments on A = %d, want 2", n)
	}
}

func TestSprintsReleasesTemplates(t *testing.T) {
	s := newTestStore(t)
	a := id(t, taskA)
	mustWrite(t, s, model.SprintPath(a), "---\nid: "+taskA+"\nname: S1\nstate: planned\n---\n")
	mustWrite(t, s, model.ReleasePath(a), "---\nid: "+taskA+"\nname: v1\nstate: unreleased\n---\n")
	mustWrite(t, s, model.TemplatePath("bug"), "---\nkind: bug\n---\n## 再現手順\n")
	mustWrite(t, s, "sprints/notes.txt", "skipped")
	mustWrite(t, s, "templates/.hidden.md", "skipped")

	if sp, err := s.Sprint(a); err != nil || sp.Name != "S1" {
		t.Errorf("Sprint = %+v, %v", sp, err)
	}
	if r, err := s.Release(a); err != nil || r.Name != "v1" {
		t.Errorf("Release = %+v, %v", r, err)
	}
	if tp, err := s.Template("bug"); err != nil || tp.Name != "bug" || tp.Item.Kind != "bug" {
		t.Errorf("Template = %+v, %v", tp, err)
	}
	if _, err := s.Template("../x"); err == nil {
		t.Error("Template accepted a name outside templates/")
	}

	nSprints, nReleases, nTemplates := 0, 0, 0
	for _, err := range s.Sprints() {
		if err != nil {
			t.Fatal(err)
		}
		nSprints++
	}
	for _, err := range s.Releases() {
		if err != nil {
			t.Fatal(err)
		}
		nReleases++
	}
	for _, err := range s.Templates() {
		if err != nil {
			t.Fatal(err)
		}
		nTemplates++
	}
	if nSprints != 1 || nReleases != 1 || nTemplates != 1 {
		t.Errorf("sprints, releases, templates = %d, %d, %d; want 1 each", nSprints, nReleases, nTemplates)
	}
}
