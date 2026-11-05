package tasks_test

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func targets(us ...*model.Task) []usecase.TaskTarget {
	out := make([]usecase.TaskTarget, len(us))
	for i, t := range us {
		out[i] = usecase.TaskTarget{Store: "me", ULID: t.ULID}
	}
	return out
}

func set(key, value string) []usecase.Assign {
	return []usecase.Assign{{Key: key, Op: "=", Value: value}}
}

func TestBulkIsOneCommit(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	a, b := ts.AddTask("a", nil), ts.AddTask("b", nil)
	r, err := tasks.Bulk(ts.Session, tasks.BulkInput{Targets: targets(a, b), Assigns: set("labels", "x")})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Tasks) != 2 || len(r.Commits) != 1 || r.Commits[0].Store != "me" {
		t.Fatalf("result = %+v", r)
	}
	for _, x := range []*model.Task{a, b} {
		if got := taskIn(t, ts, x.ULID); len(got.Labels) != 1 || got.Labels[0] != "x" {
			t.Errorf("labels = %v", got.Labels)
		}
	}
	if got := ts.HeadSubject(); !strings.Contains(got, "bulk-set 2 tasks labels=x") {
		t.Errorf("subject = %q", got)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	es, err := usecasetest.LogOf(s.Repo(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := es[0].TrailerValues("Hedgehug-Task"); len(got) != 2 {
		t.Errorf("Hedgehug-Task = %v", got)
	}

	// hedgehug undo takes all of it back.
	if _, err := tasks.Undo(ts.Session); err != nil {
		t.Fatal(err)
	}
	if got := taskIn(t, ts, b.ULID); len(got.Labels) != 0 {
		t.Errorf("after undo: %v", got.Labels)
	}
}

func TestBulkViolationWritesNothing(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	a := ts.AddTask("a", nil)
	b := ts.AddTask("b", nil)
	head := ts.Head()
	in := tasks.BulkInput{Targets: targets(a, b), Assigns: set("status", "nope")}

	in.DryRun = true
	r, err := tasks.Bulk(ts.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Violations) != 2 || r.Violations[0].Field != "status" {
		t.Fatalf("violations = %+v", r.Violations)
	}

	in.DryRun = false
	_, err = tasks.Bulk(ts.Session, in)
	usecasetest.WantErr(t, err, usecase.KindInvalid, "bulk.invalid")
	if !strings.Contains(err.Error(), a.ID) || !strings.Contains(err.Error(), b.ID) {
		t.Errorf("error lacks the tasks: %v", err)
	}
	if ts.Head() != head {
		t.Error("a refused update made a commit")
	}
}

func TestBulkDryRunWritesNothing(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	a := ts.AddTask("a", nil)
	head := ts.Head()
	r, err := tasks.Bulk(ts.Session, tasks.BulkInput{Targets: targets(a), Assigns: set("labels", "x"), DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Tasks) != 1 || len(r.Commits) != 0 || ts.Head() != head {
		t.Fatalf("result = %+v", r)
	}
}

func TestBulkAcrossStores(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	a := ts.AddedInStore("org-a", "a")
	b := ts.AddedInStore("org-b", "b")
	r, err := tasks.Bulk(ts.Session, tasks.BulkInput{
		Targets: []usecase.TaskTarget{{Store: "org-a", ULID: a}, {Store: "org-b", ULID: b}},
		Assigns: set("labels", "x"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Commits) != 2 {
		t.Fatalf("commits = %+v", r.Commits)
	}
}

func TestBulkViolationInOneStoreWritesNoStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	a := ts.AddedInStore("org-a", "a")
	b := ts.AddedInStore("org-b", "b")
	// the status is in the first store only
	ts.ForStoreEditSchema("org-a", func(sc string) string {
		return strings.Replace(sc, "statuses:\n", "statuses:\n- {name: only_a, category: doing}\n", 1)
	})
	_, err := tasks.Bulk(ts.Session, tasks.BulkInput{
		Targets: []usecase.TaskTarget{{Store: "org-a", ULID: a}, {Store: "org-b", ULID: b}},
		Assigns: set("status", "only_a"),
	})
	usecasetest.WantErr(t, err, usecase.KindInvalid, "bulk.invalid")
	if got := ts.TaskByULID("org-a", a); got.Status == "only_a" {
		t.Error("the first store was written")
	}
}

func TestBulkRefusesADeletedTask(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	a := ts.AddTask("a", nil)
	if _, err := tasks.Remove(ts.Session, a.ULID.String()); err != nil {
		t.Fatal(err)
	}
	_, err := tasks.Bulk(ts.Session, tasks.BulkInput{Targets: targets(a), Assigns: set("labels", "x")})
	if err == nil {
		t.Fatal("a deleted task was updated")
	}
}
