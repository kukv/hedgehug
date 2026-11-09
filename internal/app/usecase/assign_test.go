package usecase_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestParseAssign(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in   string
		want usecase.Assign
	}{
		{"title=Hello", usecase.Assign{"title", "=", "Hello"}},
		{"title=", usecase.Assign{"title", "=", ""}},
		{"labels+=ui", usecase.Assign{"labels", "+=", "ui"}},
		{"labels-=ui", usecase.Assign{"labels", "-=", "ui"}},
		{"title=a=b", usecase.Assign{"title", "=", "a=b"}},
	} {
		got, err := usecase.ParseAssign(tc.in)
		if err != nil {
			t.Errorf("ParseAssign(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseAssign(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
	for _, bad := range []string{"title", "=x", "+=x", "-=x"} {
		_, err := usecase.ParseAssign(bad)
		if usecase.KindOf(err) != usecase.KindUsage {
			t.Errorf("ParseAssign(%q) err = %v, want KindUsage", bad, err)
		}
	}
}

func TestApplyAssign(t *testing.T) {
	t.Parallel()
	other := model.NewULID()
	resolve := func(s string) (model.ULID, error) {
		if s == "other" {
			return other, nil
		}
		return model.ULID{}, errors.New("no such task")
	}
	apply := func(t *testing.T, task *model.Task, ss ...string) error {
		t.Helper()
		for _, s := range ss {
			a, err := usecase.ParseAssign(s)
			if err != nil {
				t.Fatal(err)
			}
			if err := usecase.ApplyAssign(task, a, usecasetest.TestSchema(), resolve); err != nil {
				return err
			}
		}
		return nil
	}
	newTask := func() *model.Task {
		return &model.Task{ULID: model.NewULID(), Kind: "task", Title: "t", Status: "todo", Labels: []string{"x"}}
	}

	t.Run("built-in scalars", func(t *testing.T) {
		task := newTask()
		if err := apply(t, task, "title=New", "kind=bug", "status=done"); err != nil {
			t.Fatal(err)
		}
		if task.Title != "New" || task.Kind != "bug" || task.Status != "done" {
			t.Errorf("task = %+v", task)
		}
		if err := apply(t, task, "title+=x"); err == nil {
			t.Error("title+= succeeded")
		}
	})
	t.Run("built-in lists", func(t *testing.T) {
		task := newTask()
		if err := apply(t, task, "labels+=ui", "labels+=ui", "labels-=x", "assignees=github.com/alice", "watchers+=github.com/bob"); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(task.Labels, []string{"ui"}) {
			t.Errorf("labels = %v, want [ui]", task.Labels)
		}
		if !slices.Equal(task.Assignees, []string{"github.com/alice"}) || !slices.Equal(task.Watchers, []string{"github.com/bob"}) {
			t.Errorf("assignees = %v, watchers = %v", task.Assignees, task.Watchers)
		}
		if err := apply(t, task, "labels="); err != nil || task.Labels != nil {
			t.Errorf("labels= : err %v, labels %v", err, task.Labels)
		}
	})
	t.Run("relations resolve ULIDs", func(t *testing.T) {
		for _, rel := range schema.Relations {
			task := newTask()
			if err := apply(t, task, rel.Name+"+=other"); err != nil {
				t.Fatal(err)
			}
			if got := rel.Get(task); !slices.Equal(got, []model.ULID{other}) {
				t.Errorf("%s = %v", rel.Name, got)
			}
			if task.Props.Get(rel.Name) != nil {
				t.Errorf("%s landed in props", rel.Name)
			}
			if err := apply(t, task, rel.Name+"-=other"); err != nil || len(rel.Get(task)) != 0 {
				t.Errorf("%s-= : err %v, %v", rel.Name, err, rel.Get(task))
			}
			if err := apply(t, task, rel.Name+"+=nope"); err == nil {
				t.Errorf("an unknown task was added to %s", rel.Name)
			}
			if err := apply(t, task, rel.Name+"=other"); err != nil || len(rel.Get(task)) != 1 {
				t.Errorf("%s= : err %v, %v", rel.Name, err, rel.Get(task))
			}
			if err := apply(t, task, rel.Name+"="); err != nil || len(rel.Get(task)) != 0 {
				t.Errorf("%s= (clear): err %v, %v", rel.Name, err, rel.Get(task))
			}
		}
	})
	t.Run("the other direction is read-only", func(t *testing.T) {
		for _, rel := range schema.Relations {
			a, err := usecase.ParseAssign(rel.Inverse + "+=other")
			if err != nil {
				t.Fatal(err)
			}
			usecasetest.WantErr(t, usecase.ApplyAssign(newTask(), a, usecasetest.TestSchema(), resolve), usecase.KindInvalid, "assign.readonly")
		}
	})
	t.Run("props by type", func(t *testing.T) {
		task := newTask()
		if err := apply(t, task, "begin=2026-11-01", "score=3", "effort=2h", "level=high", "flag=true", "at=2026-10-30T10:00:00Z", "note=123", "tags+=a"); err != nil {
			t.Fatal(err)
		}
		for key, want := range map[string]string{"begin": "2026-11-01", "score": "3", "effort": "2h", "level": "high", "flag": "true", "at": "2026-10-30T10:00:00Z", "note": "123"} {
			n := task.Props.Get(key)
			if n == nil || n.Kind != yaml.ScalarNode || n.Value != want {
				t.Errorf("props.%s = %+v, want %q", key, n, want)
			}
		}
		if n := task.Props.Get("tags"); n == nil || n.Kind != yaml.SequenceNode || len(n.Content) != 1 || n.Content[0].Value != "a" {
			t.Errorf("props.tags = %+v", n)
		}
		b, err := task.Encode()
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range []string{"begin: 2026-11-01\n", "score: 3\n", "flag: true\n", "note: '123'\n"} {
			if !strings.Contains(string(b), s) {
				t.Errorf("the file does not contain %q:\n%s", s, b)
			}
		}
		if err := apply(t, task, "begin="); err != nil || task.Props.Get("begin") != nil {
			t.Errorf("begin= : err %v, begin %v", err, task.Props.Get("begin"))
		}
		if err := apply(t, task, "tags-=a"); err != nil || task.Props.Get("tags") != nil {
			t.Errorf("tags-= : err %v, %v", err, task.Props.Get("tags"))
		}
	})
	t.Run("built-in properties", func(t *testing.T) {
		task := newTask()
		if err := apply(t, task, "priority=HIGH", "start=2026-11-01", "due=2026-11-20", "estimate=1h30m", "points=2.5"); err != nil {
			t.Fatal(err)
		}
		if task.Priority != "high" { // the spelling of the list
			t.Errorf("priority = %q, want high", task.Priority)
		}
		if task.Start == nil || task.Start.String() != "2026-11-01" || task.Due == nil || task.Due.String() != "2026-11-20" ||
			task.Estimate == nil || task.Estimate.String() != "1h30m" || task.Points == nil || *task.Points != 2.5 {
			t.Errorf("task = %+v", task)
		}
		if task.Props.Get("priority") != nil || task.Props.Get("due") != nil {
			t.Error("a built-in landed in props")
		}
		if err := apply(t, task, "priority=nope"); err != nil || task.Priority != "nope" {
			t.Errorf("a priority the list lacks is the store's to refuse: err %v, priority %q", err, task.Priority)
		}
		if err := apply(t, task, "priority=", "start=", "due=", "estimate=", "points="); err != nil {
			t.Fatal(err)
		}
		if task.Priority != "" || task.Start != nil || task.Due != nil || task.Estimate != nil || task.Points != nil {
			t.Errorf("not cleared: %+v", task)
		}
	})
	t.Run("clearing a built-in drops the value that could not be read", func(t *testing.T) {
		task, err := model.DecodeTask([]byte("---\nulid: " + model.NewULID().String() + "\ndue: next week\npoints: lots\n---\n"))
		if err != nil {
			t.Fatal(err)
		}
		if task.Extra.Get("due") == nil || task.Extra.Get("points") == nil {
			t.Fatalf("the unreadable values were not set aside: %+v", task.Extra)
		}
		if err := apply(t, task, "due=", "points=3"); err != nil {
			t.Fatal(err)
		}
		b, err := task.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "next week") || strings.Contains(string(b), "lots") || strings.Contains(string(b), "due:") && !strings.Contains(string(b), "due: null") {
			t.Errorf("the file keeps the unreadable value:\n%s", b)
		}
	})
	t.Run("labels take the spelling of the list", func(t *testing.T) {
		task := newTask()
		task.Labels = nil
		if err := apply(t, task, "labels+=BUG", "labels+=Ui"); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(task.Labels, []string{"bug", "ui"}) {
			t.Errorf("labels = %v, want [bug ui]", task.Labels)
		}
		if err := apply(t, task, "labels-=BUG"); err != nil || !slices.Equal(task.Labels, []string{"ui"}) {
			t.Errorf("labels-=BUG: err %v, labels %v", err, task.Labels)
		}
		if err := apply(t, task, "labels=API"); err != nil || !slices.Equal(task.Labels, []string{"api"}) {
			t.Errorf("labels=API: err %v, labels %v", err, task.Labels)
		}
		if err := apply(t, task, "labels+=unlisted"); err != nil || !slices.Equal(task.Labels, []string{"api", "unlisted"}) {
			t.Errorf("a label the list lacks is kept for the store to refuse: err %v, labels %v", err, task.Labels)
		}
	})
	t.Run("errors", func(t *testing.T) {
		for _, s := range []string{
			"parent=x", "rank=x", "ulid=x", "created=x", "deleted_at=x", "id=x", "sprint=x", "props=x",
			"nope=1", "begin=tomorrow", "score=many", "flag=maybe", "at=today", "effort=long",
			"due=tomorrow", "start=2026-13-01", "points=many", "points=NaN", "points=Inf", "estimate=long", "estimate+=1h", "priority+=high",
			"begin+=2026-11-01", "duplicates+=nope", "duplicates+=", "duplicates-=", "blocks+=other", "tags+=", "tags-=",
		} {
			if err := apply(t, newTask(), s); err == nil {
				t.Errorf("%s succeeded", s)
			}
		}
		err := apply(t, newTask(), "parent=x")
		if err == nil || !strings.Contains(err.Error(), "mv") {
			t.Errorf("parent=x err = %v, want advice to use mv", err)
		}
	})
}
