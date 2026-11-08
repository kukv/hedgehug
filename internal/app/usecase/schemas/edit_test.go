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
	"go.yaml.in/yaml/v4"
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
	pk, anc, err := store.Lineage(tk, s.Task)
	if err != nil {
		t.Fatal(err)
	}
	return len(schema.ValidatePlacement(tk.Kind, pk, sc.For(anc))) > 0
}

// plain is a change that carries no migrations.
func plain(fn func(*schema.Schema) (*schema.Schema, error)) func(*schema.Schema) (*schema.Schema, []schema.Migration, error) {
	return func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
		next, err := fn(sc)
		return next, nil, err
	}
}

func TestEditAddsInOneCommit(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	before := ts.Head()
	w, err := schemas.Edit(ts.Session, plain(func(sc *schema.Schema) (*schema.Schema, error) {
		return sc.AddStatus(schema.Status{Name: "review", Category: schema.CategoryDoing, Color: "teal"}), nil
	}), schemas.EditOpts{})
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
	_, err := schemas.Edit(ts.Session, plain(func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveStatus("on_hold") }),
		schemas.EditOpts{Use: []schemas.Use{{Target: schema.TargetStatus, Name: "on_hold"}}})
	if usecase.ProblemCode(err) != "schema.in_use" {
		t.Fatalf("err = %v", err)
	}
	var ue *usecase.Error
	if !errors.As(err, &ue) {
		t.Fatal(err)
	}
	if ue.Args["count"] != 1 || ue.Args["deleted"] != 1 {
		t.Errorf("args = %v", ue.Args)
	}
}

func TestEditCountsANormalTaskAndTheDeletedOnes(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("live", func(t *model.Task) { t.Status = "on_hold" })
	ts.AddDeleted("old", time.Now(), func(t *model.Task) { t.Status = "on_hold" })
	use := []schemas.Use{{Target: schema.TargetStatus, Name: "on_hold"}}
	remove := plain(func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveStatus("on_hold") })

	_, err := schemas.Edit(ts.Session, remove, schemas.EditOpts{Use: use})
	var ue *usecase.Error
	if !errors.As(err, &ue) || usecase.ProblemCode(err) != "schema.in_use" {
		t.Fatalf("err = %v", err)
	}
	if ue.Args["count"] != 2 || ue.Args["deleted"] != 1 {
		t.Errorf("args = %v", ue.Args)
	}

	// use and check together: the use is looked at first.
	if _, err := schemas.Edit(ts.Session, remove, schemas.EditOpts{Use: use, Check: true}); usecase.ProblemCode(err) != "schema.in_use" {
		t.Errorf("use + check: err = %v", err)
	}
}

// dropHigh takes the value high off priority, which a task may still have.
func dropHigh(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
	keep := []schema.ValueEdit{{Name: "low", From: []string{"low"}}, {Name: "mid", From: []string{"mid"}}, {Name: "urgent", From: []string{"urgent"}}}
	next, ms, _, err := sc.ChangeProp("priority", schema.PropChange{Values: &keep})
	return next, ms, err
}

func TestEditUseAndCheckTogether(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddProject("Web")
	ts.AddTask("bug", func(t *model.Task) {
		t.Parent = &p
		t.Props = model.Fields{{Key: "priority", Value: propScalar("high")}}
	})
	// Nothing uses on_hold, so the use passes; the check finds the task
	// whose priority is gone.
	_, err := schemas.Edit(ts.Session, dropHigh, schemas.EditOpts{Use: []schemas.Use{{Target: schema.TargetStatus, Name: "on_hold"}}, Check: true})
	if usecase.ProblemCode(err) != "schema.tasks_invalid" {
		t.Fatalf("err = %v", err)
	}
}

func TestEditRefusesNewViolations(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddProject("Web")
	ts.AddTask("bug", func(t *model.Task) {
		t.Parent = &p
		t.Props = model.Fields{{Key: "priority", Value: propScalar("high")}}
	})
	_, err := schemas.Edit(ts.Session, dropHigh, schemas.EditOpts{Check: true})
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
	if _, err := schemas.Edit(ts.Session, func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
		return sc.ChangeStatus("todo", schema.StatusChange{Label: &l})
	}, schemas.EditOpts{Check: true}); err != nil {
		t.Fatalf("an old violation refuses the edit: %v", err)
	}
}

func TestEditErrors(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	cases := []struct {
		name string
		fn   func(*schema.Schema) (*schema.Schema, []schema.Migration, error)
		code string
	}{
		{"not defined", plain(func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveStatus("nope") }), "schema.not_defined"},
		{"stale", plain(func(sc *schema.Schema) (*schema.Schema, error) { return sc.OrderStatuses([]string{"todo"}) }), "schema.order_stale"},
		{"invalid", plain(func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveStatus("todo") }), "schema.invalid"},
		{"not task_ref", func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
			m := true
			next, ms, _, err := sc.ChangeProp("priority", schema.PropChange{Multi: &m})
			return next, ms, err
		}, "schema.not_task_ref"},
		{"not enum", func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
			next, ms, _, err := sc.ChangeProp("start", schema.PropChange{Values: &[]schema.ValueEdit{{Name: "x"}}})
			return next, ms, err
		}, "schema.not_enum"},
		{"value twice", func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
			next, ms, _, err := sc.ChangeProp("priority", schema.PropChange{Values: &[]schema.ValueEdit{{Name: "a"}, {Name: "a"}}})
			return next, ms, err
		}, "schema.value_invalid"},
		{"value empty", func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
			next, ms, _, err := sc.ChangeProp("priority", schema.PropChange{Values: &[]schema.ValueEdit{{Name: ""}}})
			return next, ms, err
		}, "schema.value_invalid"},
		{"name taken", func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
			n := "done"
			return sc.ChangeStatus("todo", schema.StatusChange{Name: &n})
		}, "schema.name_taken"},
		{"prop values repeated", plain(func(sc *schema.Schema) (*schema.Schema, error) {
			return sc.AddProp(schema.Prop{Name: "area", Type: schema.TypeEnum, Values: []string{"a", "a"}})
		}), "schema.value_invalid"},
	}
	for _, c := range cases {
		if _, err := schemas.Edit(ts.Session, c.fn, schemas.EditOpts{}); usecase.ProblemCode(err) != c.code {
			t.Errorf("%s: err = %v (%s)", c.name, err, usecase.ProblemCode(err))
		}
	}
}

func propScalar(v string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Value: v} }

func taskNow(t *testing.T, ts *usecasetest.Session, u model.ULID) *model.Task {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	tk, err := s.Task(u)
	if err != nil {
		t.Fatal(err)
	}
	return tk
}

// parentOfHead is the hash of the commit before the head.
func parentOfHead(t *testing.T, ts *usecasetest.Session) string {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	head, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	log, err := s.Repo().LogFrom(head, 2)
	if err != nil || len(log) < 2 {
		t.Fatalf("log = %v, %v", log, err)
	}
	return log[1].Hash.String()
}

func removedUses(prop string, removed *[]string) func() []schemas.Use {
	return func() []schemas.Use {
		var us []schemas.Use
		for _, v := range *removed {
			us = append(us, schemas.Use{Target: schema.TargetValue, Prop: prop, Name: v})
		}
		return us
	}
}

func TestEditRenamesAStatusAndRewritesTasksInOneCommit(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	a := ts.AddTask("a", nil)
	b := ts.AddTask("b", func(t *model.Task) { t.Status = "done" })
	before := ts.Head()
	n := "open"
	w, err := schemas.Edit(ts.Session, func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
		return sc.ChangeStatus("todo", schema.StatusChange{Name: &n, Label: &n})
	}, schemas.EditOpts{})
	if err != nil || w.Commit == nil {
		t.Fatalf("Edit = %+v, %v", w, err)
	}
	if got := taskNow(t, ts, a.ULID).Status; got != "open" {
		t.Errorf("a.status = %s", got)
	}
	if got := taskNow(t, ts, b.ULID).Status; got != "done" {
		t.Errorf("b.status = %s", got)
	}
	if usecasetest.LastOp(t, ts) != "schema" || parentOfHead(t, ts) != before {
		t.Errorf("want one commit of op schema on top of %s", before)
	}
}

func TestEditValuesRewriteTasksAndRefuseInUse(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	u := ts.AddTask("urgent", func(t *model.Task) { t.Props = model.Fields{{Key: "priority", Value: propScalar("urgent")}} })
	m := ts.AddTask("mid", func(t *model.Task) { t.Props = model.Fields{{Key: "priority", Value: propScalar("mid")}} })

	// urgent goes into high, mid is renamed, low is dropped (nothing uses it)
	edit := []schema.ValueEdit{
		{Name: "high", From: []string{"high", "urgent"}},
		{Name: "medium", From: []string{"mid"}},
	}
	var removed []string
	_, err := schemas.Edit(ts.Session, func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
		next, ms, r, err := sc.ChangeProp("priority", schema.PropChange{Values: &edit})
		removed = r
		return next, ms, err
	}, schemas.EditOpts{UseAfter: removedUses("priority", &removed)})
	if err != nil {
		t.Fatal(err)
	}
	if got := taskNow(t, ts, u.ULID).Props.Get("priority").Value; got != "high" {
		t.Errorf("u.priority = %s", got)
	}
	if got := taskNow(t, ts, m.ULID).Props.Get("priority").Value; got != "medium" {
		t.Errorf("m.priority = %s", got)
	}

	// medium is in use now, so dropping it is refused with the value
	before := ts.Head()
	drop := []schema.ValueEdit{{Name: "high", From: []string{"high"}}}
	_, err = schemas.Edit(ts.Session, func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
		next, ms, r, err := sc.ChangeProp("priority", schema.PropChange{Values: &drop})
		removed = r
		return next, ms, err
	}, schemas.EditOpts{UseAfter: removedUses("priority", &removed)})
	var ue *usecase.Error
	if usecase.ProblemCode(err) != "schema.in_use" || !errors.As(err, &ue) || ue.Args["value"] != "medium" || ue.Args["count"] != 1 {
		t.Fatalf("err = %v", err)
	}
	if ts.Head() != before {
		t.Error("a refused edit committed")
	}
}

func TestEditValueSwapRewritesTasks(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	l := ts.AddTask("l", func(t *model.Task) { t.Props = model.Fields{{Key: "priority", Value: propScalar("low")}} })
	h := ts.AddTask("h", func(t *model.Task) { t.Props = model.Fields{{Key: "priority", Value: propScalar("high")}} })
	edit := []schema.ValueEdit{
		{Name: "high", From: []string{"low"}},
		{Name: "low", From: []string{"high"}},
		{Name: "mid", From: []string{"mid"}},
		{Name: "urgent", From: []string{"urgent"}},
	}
	if _, err := schemas.Edit(ts.Session, func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
		next, ms, _, err := sc.ChangeProp("priority", schema.PropChange{Values: &edit})
		return next, ms, err
	}, schemas.EditOpts{}); err != nil {
		t.Fatal(err)
	}
	if got := taskNow(t, ts, l.ULID).Props.Get("priority").Value; got != "high" {
		t.Errorf("l.priority = %s", got)
	}
	if got := taskNow(t, ts, h.ULID).Props.Get("priority").Value; got != "low" {
		t.Errorf("h.priority = %s", got)
	}
}

func TestEditRenamesAProp(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	a := ts.AddTask("a", func(t *model.Task) { t.Props = model.Fields{{Key: "priority", Value: propScalar("low")}} })
	n := "importance"
	if _, err := schemas.Edit(ts.Session, func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
		next, ms, _, err := sc.ChangeProp("priority", schema.PropChange{Name: &n})
		return next, ms, err
	}, schemas.EditOpts{}); err != nil {
		t.Fatal(err)
	}
	if taskNow(t, ts, a.ULID).Props.Get("importance") == nil {
		t.Error("the property was not renamed in the task")
	}
}

func refList(us ...model.ULID) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode}
	for _, u := range us {
		n.Content = append(n.Content, propScalar(u.String()))
	}
	return n
}

// A rename saved with a change that is checked: the tasks are looked at
// under their new names.
func TestEditRenameWithCheckedChange(t *testing.T) {
	t.Parallel()
	t.Run("prop name and multi", func(t *testing.T) {
		t.Parallel()
		ts := usecasetest.New(t)
		// no task has the property: a multi change cannot hold for one that
		// has it, a list being a list
		a := ts.AddTask("a", func(t *model.Task) { t.Props = model.Fields{{Key: "priority", Value: propScalar("low")}} })
		n, m := "cause", false
		if _, err := schemas.Edit(ts.Session, func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
			next, ms, _, err := sc.ChangeProp("causes", schema.PropChange{Name: &n, Multi: &m})
			return next, ms, err
		}, schemas.EditOpts{Check: true}); err != nil {
			t.Fatal(err)
		}
		if taskNow(t, ts, a.ULID).Props.Get("priority") == nil {
			t.Error("the task lost a property")
		}
	})
	t.Run("prop name and a new violation", func(t *testing.T) {
		t.Parallel()
		ts := usecasetest.New(t)
		x, y := ts.AddTask("x", nil), ts.AddTask("y", nil)
		ts.AddTask("a", func(t *model.Task) { t.Props = model.Fields{{Key: "causes", Value: refList(x.ULID, y.ULID)}} })
		n, m := "cause", false
		_, err := schemas.Edit(ts.Session, func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
			next, ms, _, err := sc.ChangeProp("causes", schema.PropChange{Name: &n, Multi: &m})
			return next, ms, err
		}, schemas.EditOpts{Check: true})
		if usecase.ProblemCode(err) != "schema.tasks_invalid" {
			t.Fatalf("err = %v", err)
		}
	})
}
