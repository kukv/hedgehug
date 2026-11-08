package schemas_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// brokenUnder writes a project under parent past the store's checks.
func brokenUnder(t *testing.T, ts *usecasetest.Session, parent model.ULID) model.ULID {
	t.Helper()
	tk := ts.AddTask("nested", nil)
	tk.Kind = "project"
	tk.Parent = &parent
	b, err := tk.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ts.CommitRawFile(model.TaskPath(tk.ULID), string(b))
	return tk.ULID
}

// violatesNow reports whether the task u breaks the default schema as stored.
func violatesNow(t *testing.T, ts *usecasetest.Session, u model.ULID) bool {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	sc := schema.Default()
	tk, err := s.Task(u)
	if err != nil {
		t.Fatal(err)
	}
	pk, anc, err := store.Lineage(tk, sc, s.Task)
	if err != nil {
		t.Fatal(err)
	}
	return len(schema.ValidatePlacement(tk.Kind, pk, sc.For(anc))) > 0
}

func TestEditAddsInOneCommit(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	before := ts.Head()
	w, err := schemas.Edit(ts.Session, func(sc *schema.Schema) (*schema.Schema, error) {
		return sc.AddStatus(schema.Status{Name: "review", Category: schema.CategoryDoing, Color: "teal"}), nil
	}, nil, false)
	if err != nil || w.Commit == nil {
		t.Fatalf("Edit = %+v, %v", w, err)
	}
	if ts.Head() == before || usecasetest.LastOp(t, ts) != "schema" {
		t.Errorf("want one commit of op schema")
	}
	last := w.Schema.Statuses[len(w.Schema.Statuses)-1]
	if last.Name != "review" || last.Color != "teal" {
		t.Errorf("last status = %+v", last)
	}
}

func TestEditRefusesInUse(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddDeleted("old", time.Now(), func(t *model.Task) { t.Status = "on_hold" })
	_, err := schemas.Edit(ts.Session, func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveStatus("on_hold") },
		&schemas.Use{Target: schema.TargetStatus, Name: "on_hold"}, false)
	if usecase.ProblemCode(err) != "schema.in_use" {
		t.Fatalf("err = %v", err)
	}
	var ue *usecase.Error
	if !errors.As(err, &ue) || ue.Args["count"] != 1 || ue.Args["deleted"] != 1 {
		t.Errorf("args = %v", ue.Args)
	}
}

func TestEditRefusesNewViolations(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddProject("Web")
	ts.AddTask("bug", func(t *model.Task) { t.Kind = "bug"; t.Parent = &p })
	_, err := schemas.Edit(ts.Session, func(sc *schema.Schema) (*schema.Schema, error) {
		ps := []string{schema.Root}
		return sc.ChangeKind("bug", schema.KindChange{Parents: &ps})
	}, nil, true)
	if usecase.ProblemCode(err) != "schema.tasks_invalid" {
		t.Fatalf("err = %v", err)
	}
	var ue *usecase.Error
	if !errors.As(err, &ue) {
		t.Fatal(err)
	}
	lines, _ := ue.Args["violations"].([]string)
	if len(lines) != 1 || !strings.Contains(lines[0], "bug") {
		t.Errorf("violations = %v", lines)
	}
}

func TestEditIgnoresOldViolations(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddProject("Web")
	// A project under a project breaks the default schema (project sits at
	// the root only). Write it past the checks, as a sync can leave it.
	if !violatesNow(t, ts, brokenUnder(t, ts, p)) {
		t.Fatal("brokenUnder makes no violation under the current schema")
	}
	l := "タスク"
	if _, err := schemas.Edit(ts.Session, func(sc *schema.Schema) (*schema.Schema, error) {
		return sc.ChangeKind("task", schema.KindChange{Label: &l})
	}, nil, true); err != nil {
		t.Fatalf("an old violation refuses the edit: %v", err)
	}
}

func TestEditErrors(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	cases := []struct {
		name string
		fn   func(*schema.Schema) (*schema.Schema, error)
		code string
	}{
		{"not defined", func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveStatus("nope") }, "schema.not_defined"},
		{"stale", func(sc *schema.Schema) (*schema.Schema, error) { return sc.OrderStatuses([]string{"todo"}) }, "schema.order_stale"},
		{"invalid", func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveStatus("todo") }, "schema.invalid"},
		{"container", func(sc *schema.Schema) (*schema.Schema, error) {
			return sc.ChangeKind("epic", schema.KindChange{Default: true})
		}, "schema.default_container"},
		{"parents", func(sc *schema.Schema) (*schema.Schema, error) {
			sc = sc.AddKind(schema.Kind{Name: "sub", Role: schema.RoleWork, Parents: []string{"epic"}})
			return sc.RemoveKind("epic")
		}, "schema.parents_empty"},
	}
	for _, c := range cases {
		if _, err := schemas.Edit(ts.Session, c.fn, nil, false); usecase.ProblemCode(err) != c.code {
			t.Errorf("%s: err = %v (%s)", c.name, err, usecase.ProblemCode(err))
		}
	}
}
