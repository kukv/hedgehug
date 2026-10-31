package main

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func TestParseAssign(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want assign
	}{
		{"title=Hello", assign{"title", "=", "Hello"}},
		{"title=", assign{"title", "=", ""}},
		{"labels+=ui", assign{"labels", "+=", "ui"}},
		{"labels-=ui", assign{"labels", "-=", "ui"}},
		{"title=a=b", assign{"title", "=", "a=b"}},
	} {
		got, err := parseAssign(tc.in)
		if err != nil {
			t.Errorf("parseAssign(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseAssign(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
	for _, bad := range []string{"title", "=x", "+=x", "-=x"} {
		_, err := parseAssign(bad)
		var ue usageError
		if !errors.As(err, &ue) {
			t.Errorf("parseAssign(%q) err = %v, want a usageError", bad, err)
		}
	}
}

// testSchema is the default schema with a property of each other type.
func testSchema() *schema.Schema {
	sc := schema.Default()
	sc.Props = append(sc.Props,
		schema.Prop{Name: "flag", Type: schema.TypeBool},
		schema.Prop{Name: "at", Type: schema.TypeDatetime},
		schema.Prop{Name: "note", Type: schema.TypeText},
		schema.Prop{Name: "tags", Type: schema.TypeMultiEnum, Values: []string{"a", "b"}},
	)
	return sc
}

func TestApplyAssign(t *testing.T) {
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
			a, err := parseAssign(s)
			if err != nil {
				t.Fatal(err)
			}
			if err := applyAssign(task, a, testSchema(), resolve); err != nil {
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
	t.Run("blocked_by resolves ULIDs", func(t *testing.T) {
		task := newTask()
		if err := apply(t, task, "blocked_by+=other"); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(task.BlockedBy, []model.ULID{other}) {
			t.Errorf("blocked_by = %v", task.BlockedBy)
		}
		if err := apply(t, task, "blocked_by-=other"); err != nil || len(task.BlockedBy) != 0 {
			t.Errorf("blocked_by-= : err %v, %v", err, task.BlockedBy)
		}
		if err := apply(t, task, "blocked_by+=nope"); err == nil {
			t.Error("an unknown task was added to blocked_by")
		}
	})
	t.Run("props by type", func(t *testing.T) {
		task := newTask()
		if err := apply(t, task, "due=2026-11-01", "points=3", "estimate=2h", "priority=high", "flag=true", "at=2026-10-30T10:00:00Z", "note=123", "duplicates+=other", "tags+=a"); err != nil {
			t.Fatal(err)
		}
		for key, want := range map[string]string{"due": "2026-11-01", "points": "3", "estimate": "2h", "priority": "high", "flag": "true", "at": "2026-10-30T10:00:00Z", "note": "123"} {
			n := task.Props.Get(key)
			if n == nil || n.Kind != yaml.ScalarNode || n.Value != want {
				t.Errorf("props.%s = %+v, want %q", key, n, want)
			}
		}
		if n := task.Props.Get("duplicates"); n == nil || n.Kind != yaml.SequenceNode || len(n.Content) != 1 || n.Content[0].Value != other.String() {
			t.Errorf("props.duplicates = %+v", n)
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
		if err := apply(t, task, "duplicates-=other"); err != nil || task.Props.Get("duplicates") != nil {
			t.Errorf("duplicates-= : err %v, %v", err, task.Props.Get("duplicates"))
		}
	})
	t.Run("errors", func(t *testing.T) {
		for _, s := range []string{
			"parent=x", "rank=x", "ulid=x", "created=x", "deleted_at=x", "id=x", "sprint=x", "props=x",
			"nope=1", "due=tomorrow", "points=many", "flag=maybe", "at=today", "estimate=long",
			"due+=2026-11-01", "duplicates+=nope", "duplicates+=", "duplicates-=", "tags+=", "tags-=",
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
