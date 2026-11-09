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
		if err := apply(t, task, "due=2026-11-01", "points=3", "estimate=2h", "priority=high", "flag=true", "at=2026-10-30T10:00:00Z", "note=123", "tags+=a"); err != nil {
			t.Fatal(err)
		}
		for key, want := range map[string]string{"due": "2026-11-01", "points": "3", "estimate": "2h", "priority": "high", "flag": "true", "at": "2026-10-30T10:00:00Z", "note": "123"} {
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
		for _, s := range []string{"due: 2026-11-01\n", "points: 3\n", "flag: true\n", "note: '123'\n"} {
			if !strings.Contains(string(b), s) {
				t.Errorf("the file does not contain %q:\n%s", s, b)
			}
		}
		if err := apply(t, task, "due="); err != nil || task.Props.Get("due") != nil {
			t.Errorf("due= : err %v, due %v", err, task.Props.Get("due"))
		}
		if err := apply(t, task, "tags-=a"); err != nil || task.Props.Get("tags") != nil {
			t.Errorf("tags-= : err %v, %v", err, task.Props.Get("tags"))
		}
	})
	t.Run("errors", func(t *testing.T) {
		for _, s := range []string{
			"parent=x", "rank=x", "ulid=x", "created=x", "deleted_at=x", "id=x", "sprint=x", "props=x",
			"nope=1", "due=tomorrow", "points=many", "flag=maybe", "at=today", "estimate=long",
			"due+=2026-11-01", "duplicates+=nope", "duplicates+=", "duplicates-=", "blocks+=other", "tags+=", "tags-=",
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
