package tasks_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestShowAt(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	r, err := tasks.Add(ts.Session, tasks.AddInput{Title: "orig", Body: "old body\n"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Set(ts.Session, "ME-1", usecasetest.MustAssigns(t, "title=changed", "status=in_progress", "labels+=x")); err != nil {
		t.Fatal(err)
	}
	later, err := tasks.Add(ts.Session, tasks.AddInput{Title: "added after"})
	if err != nil {
		t.Fatal(err)
	}

	v, err := tasks.ShowAt(ts.Session, "ME-1", r.Commit)
	if err != nil {
		t.Fatal(err)
	}
	if v.ID != "ME-1" || v.ULID != r.ULID.String() || v.Title != "orig" || v.Status != "todo" || v.Body != "old body\n" || len(v.Labels) != 0 {
		t.Errorf("ShowAt = %+v (body %q)", v.TaskView, v.Body)
	}
	if v.Comments == nil || v.History == nil || v.Reverse == nil || v.Watchers == nil || len(v.Comments)+len(v.History) != 0 {
		t.Errorf("comments, history and the rest are empty lists: %+v", v)
	}

	// The task did not exist yet at that commit.
	_, err = tasks.ShowAt(ts.Session, later.ULID.String(), r.Commit)
	usecasetest.WantErr(t, err, usecase.KindNotFound, "restore_at.not_found")
	// Neither a time nor a commit.
	_, err = tasks.ShowAt(ts.Session, "ME-1", "yesterday-ish")
	usecasetest.WantErr(t, err, usecase.KindInvalid, "restore_at.bad_time")
}

// A time picks the last commit at or before it, as RestoreAt's does; the
// session's zone reads a date.
func TestShowAtByTime(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.Loc = time.UTC
	day := func(d int) func() time.Time {
		return func() time.Time { return time.Date(2026, 10, d, 12, 0, 0, 0, time.UTC) }
	}
	ts.Now = day(20)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "v1"}); err != nil {
		t.Fatal(err)
	}
	ts.Now = day(25)
	if _, err := tasks.Set(ts.Session, "ME-1", usecasetest.MustAssigns(t, "title=v2")); err != nil {
		t.Fatal(err)
	}
	ts.Now = day(30)
	if _, err := tasks.Set(ts.Session, "ME-1", usecasetest.MustAssigns(t, "title=v3")); err != nil {
		t.Fatal(err)
	}
	for at, want := range map[string]string{
		"2026-10-20T12:00:00Z": "v1",
		"2026-10-22T00:00:00Z": "v1",
		"2026-10-24":           "v1",
		"2026-10-25":           "v2", // a date is the end of that day
		"2026-10-30T12:00:00Z": "v3",
		"2026-11-30":           "v3",
	} {
		v, err := tasks.ShowAt(ts.Session, "ME-1", at)
		if err != nil || v.Title != want {
			t.Errorf("ShowAt(%q) = %q, %v; want %q", at, titleOf(v), err, want)
		}
	}
	_, err := tasks.ShowAt(ts.Session, "ME-1", "2026-10-01")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "restore_at.not_found")
}

func titleOf(v usecase.ShowView) string {
	if v.TaskView == nil {
		return ""
	}
	return v.Title
}

// The id and the workspace are those of now, not the ones the task had at the
// commit: a move to another workspace renumbers the task.
func TestShowAtKeepsTheIDAndWorkspaceOfNow(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddWorkspace("api", "API")
	r, err := tasks.Add(ts.Session, tasks.AddInput{Title: "t", Workspace: "me"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Move(ts.Session, tasks.MoveInput{Arg: "ME-1", Workspace: "api"}); err != nil {
		t.Fatal(err)
	}
	v, err := tasks.ShowAt(ts.Session, "API-1", r.Commit)
	if err != nil {
		t.Fatal(err)
	}
	if v.ID != "API-1" || v.Workspace == nil || v.Workspace.Name != "api" || len(v.FormerIDs) != 1 || v.FormerIDs[0] != "ME-1" {
		t.Errorf("ShowAt = id %q workspace %+v former %q; want API-1, api, [ME-1]", v.ID, v.Workspace, v.FormerIDs)
	}
}

// A task that was removed is read like any other; before its removal it was
// not deleted.
func TestShowAtDeletedTask(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	r, err := tasks.Add(ts.Session, tasks.AddInput{Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	ts.DeleteTask("ME-1")
	v, err := tasks.ShowAt(ts.Session, "ME-1", r.Commit)
	if err != nil || v.Title != "t" || v.DeletedAt != nil {
		t.Errorf("ShowAt before the removal = %+v, %v; want t, not deleted", v.TaskView, err)
	}
	head, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	h, err := head.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	v, err = tasks.ShowAt(ts.Session, "ME-1", h.String())
	if err != nil || v.DeletedAt == nil {
		t.Errorf("ShowAt after the removal = %+v, %v; want deleted", v.TaskView, err)
	}
}

// With several stores a task is named as Show names it: by ULID, by an id
// that only one store has, or with the store before it.
func TestShowAtOfSeveralStores(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	b := ts.ForStore("org-b")
	r, err := tasks.Add(b, tasks.AddInput{Title: "orig"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Set(b, "WEB-2", usecasetest.MustAssigns(t, "title=changed")); err != nil {
		t.Fatal(err)
	}
	for _, arg := range []string{r.ULID.String(), "WEB-2", "org-b:" + r.ULID.String()} {
		v, err := tasks.ShowAt(ts.Session, arg, r.Commit)
		if err != nil || v.Title != "orig" || v.ID != "WEB-2" {
			t.Errorf("ShowAt(%q) = %+v, %v; want orig, WEB-2", arg, v.TaskView, err)
		}
	}
	// A session of the other store does not see it.
	_, err = tasks.ShowAt(ts.ForStore("org-a"), r.ULID.String(), r.Commit)
	usecasetest.WantErr(t, err, usecase.KindNotFound, "task.not_found")
}
