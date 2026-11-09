package automation

import (
	"reflect"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

func TestFieldsOfALink(t *testing.T) {
	f := Default().IssueFields
	on := f
	on.Props = map[string]string{"milestone": "target"}
	on.Push = []string{"title", "milestone"}
	for _, c := range []struct {
		name string
		f    IssueFields
		l    model.Link
		want []string
	}{
		{"mirror default", f, model.Link{Sync: model.SyncMirror}, []string{"title", "status", "assignees"}},
		{"push default", f, model.Link{Sync: model.SyncPush}, []string{"title", "body", "status", "assignees"}},
		{"the link comes first", f, model.Link{Sync: model.SyncPush, Fields: []string{"status"}}, []string{"status"}},
		{"link mode has none", f, model.Link{Sync: model.SyncLink, Fields: []string{"status"}}, nil},
		{"off has none", f, model.Link{Sync: model.SyncOff}, nil},
		{"an unknown name goes", f, model.Link{Sync: model.SyncPush, Fields: []string{"nope", "title", "title"}}, []string{"title"}},
		{"labels are not synced", f, model.Link{Sync: model.SyncPush, Fields: []string{"labels", "title"}}, []string{"title"}},
		{"comments are a name of their own", f, model.Link{Sync: model.SyncPush, Fields: []string{"title", "comments"}}, []string{"title", "comments"}},
		{"no milestone without props", f, model.Link{Sync: model.SyncPush, Fields: []string{"milestone"}}, nil},
		{"milestone with props", on, model.Link{Sync: model.SyncPush}, []string{"title", "milestone"}},
	} {
		if got := c.f.For(c.l); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCommentsAreNotInTheDefaults(t *testing.T) {
	f := Default().IssueFields
	if !slices.Contains(IssueFieldNames, "comments") || slices.Contains(f.Push, "comments") || slices.Contains(f.Mirror, "comments") {
		t.Errorf("names %v, push %v, mirror %v", IssueFieldNames, f.Push, f.Mirror)
	}
}

func TestScalarFieldsLeaveOutComments(t *testing.T) {
	f := Default().IssueFields
	l := model.Link{Sync: model.SyncPush, Fields: []string{"comments", "title", "comments"}}
	if got := f.Scalars(l); !reflect.DeepEqual(got, []string{"title"}) {
		t.Errorf("scalars = %v", got)
	}
	if got := f.Scalars(model.Link{Sync: model.SyncLink, Fields: []string{"comments"}}); got != nil {
		t.Errorf("scalars of a link that does not sync = %v", got)
	}
}

func TestStatusTokens(t *testing.T) {
	for c, want := range map[schema.Category]string{
		schema.CategoryTodo: IssueOpen, schema.CategoryDoing: IssueOpen,
		schema.CategoryDone: IssueCompleted, schema.CategoryCanceled: IssueNotPlanned,
	} {
		if got := StatusToken(c); got != want {
			t.Errorf("%s: %s", c, got)
		}
	}
	for token, want := range map[string]schema.Category{
		IssueOpen: schema.CategoryTodo, IssueCompleted: schema.CategoryDone, IssueNotPlanned: schema.CategoryCanceled,
	} {
		if got := TokenCategory(token); got != want {
			t.Errorf("%s: %s", token, got)
		}
	}
}

func TestReflect(t *testing.T) {
	v := func(s string) model.IssueValues { return model.IssueValues{"title": {s}} }
	none := model.IssueValues{}
	for _, c := range []struct {
		name          string
		mode          string
		task, gh, was model.IssueValues
		toTask, toGH  model.IssueValues
		synced        model.IssueValues
		lost          []Overwrite
	}{
		{name: "mirror: all the same", mode: model.SyncMirror, task: v("a"), gh: v("a"), was: v("a"), toTask: none, toGH: none, synced: v("a")},
		{name: "mirror: GitHub changed", mode: model.SyncMirror, task: v("a"), gh: v("b"), was: v("a"), toTask: v("b"), toGH: none, synced: v("b")},
		{name: "mirror: the task changed", mode: model.SyncMirror, task: v("x"), gh: v("a"), was: v("a"), toTask: v("a"), toGH: none, synced: v("a"), lost: []Overwrite{{"title", SideTask, []string{"x"}}}},
		{name: "mirror: both changed", mode: model.SyncMirror, task: v("x"), gh: v("b"), was: v("a"), toTask: v("b"), toGH: none, synced: v("b"), lost: []Overwrite{{"title", SideTask, []string{"x"}}}},
		{name: "mirror: both to the same", mode: model.SyncMirror, task: v("b"), gh: v("b"), was: v("a"), toTask: none, toGH: none, synced: v("b")},
		{name: "mirror: no base", mode: model.SyncMirror, task: v("x"), gh: v("a"), was: none, toTask: v("a"), toGH: none, synced: v("a")},
		{name: "push: all the same", mode: model.SyncPush, task: v("a"), gh: v("a"), was: v("a"), toTask: none, toGH: none, synced: v("a")},
		{name: "push: the task changed", mode: model.SyncPush, task: v("b"), gh: v("a"), was: v("a"), toTask: none, toGH: v("b"), synced: v("b")},
		{name: "push: GitHub changed", mode: model.SyncPush, task: v("a"), gh: v("x"), was: v("a"), toTask: none, toGH: v("a"), synced: v("a"), lost: []Overwrite{{"title", SideGitHub, []string{"x"}}}},
		{name: "push: both changed", mode: model.SyncPush, task: v("b"), gh: v("x"), was: v("a"), toTask: none, toGH: v("b"), synced: v("b"), lost: []Overwrite{{"title", SideGitHub, []string{"x"}}}},
		{name: "push: no base", mode: model.SyncPush, task: v("a"), gh: v("x"), was: none, toTask: none, toGH: v("a"), synced: v("a")},
		{name: "link", mode: model.SyncLink, task: v("a"), gh: v("x"), was: v("y"), toTask: none, toGH: none, synced: none},
	} {
		r := Reflect(c.mode, []string{"title"}, c.task, c.gh, c.was)
		if !reflect.DeepEqual(r.ToTask, c.toTask) || !reflect.DeepEqual(r.ToGitHub, c.toGH) || !reflect.DeepEqual(r.Synced, c.synced) || !reflect.DeepEqual(r.Overwritten, c.lost) {
			t.Errorf("%s: %+v", c.name, r)
		}
	}
}

// A field that is not asked for is left alone.
func TestReflectOnlyTheFields(t *testing.T) {
	task := model.IssueValues{"title": {"a"}, "assignees": {"x"}}
	gh := model.IssueValues{"title": {"b"}, "assignees": {}}
	r := Reflect(model.SyncMirror, []string{"title"}, task, gh, nil)
	if !reflect.DeepEqual(r.ToTask, model.IssueValues{"title": {"b"}}) {
		t.Errorf("%+v", r)
	}
	r = Reflect(model.SyncPush, []string{"assignees"}, task, gh, nil)
	if !reflect.DeepEqual(r.ToGitHub, model.IssueValues{"assignees": {"x"}}) {
		t.Errorf("%+v", r)
	}
}

// The rules start where the sync of an issue left the task.
func TestEvaluateFromAnotherStatus(t *testing.T) {
	f := &File{Rules: []Rule{{ID: "r", On: IssueClosed, Set: Set{Category: "done"}, Enabled: true}}}
	task := &model.Task{Status: "todo"}
	in := Input{Events: []string{IssueClosed}, Task: task, Status: "done", Schema: schema.Default()}
	if o := f.Evaluate(in); o.Change != nil || o.Rejected != nil {
		t.Errorf("from done: %+v", o)
	}
	in.Status = ""
	if o := f.Evaluate(in); o.Change == nil || o.Change.Status != "done" {
		t.Errorf("from the task: %+v", o)
	}
	// A status the rule takes back to where the task started is still a move
	// from where the rules started.
	task.Status = "done"
	in.Status = "in_progress"
	if o := f.Evaluate(in); o.Change == nil || o.Change.Status != "done" {
		t.Errorf("from in_progress: %+v", o)
	}
}

func TestIssueOverwrittenTrailer(t *testing.T) {
	u := model.NewULID()
	ref := "github.com/a/b/issues/1"
	o := Overwrite{"body", SideGitHub, []string{"line one\nline two \"x\""}}
	gotU, gotRef, got, ok := ParseIssueOverwritten(IssueOverwrittenTrailer(u, ref, o))
	if !ok || gotU != u || gotRef != ref || !reflect.DeepEqual(got, o) {
		t.Errorf("%v %v %s %+v", ok, gotU, gotRef, got)
	}
	if _, _, _, ok := ParseIssueOverwritten(IssueOverwrittenTrailer(u, ref, Overwrite{Field: "assignees", Side: SideTask})); !ok {
		t.Error("an empty value does not read")
	}
	for _, bad := range []string{"", "nope a b c []", u.String() + " a b c not-json"} {
		if _, _, _, ok := ParseIssueOverwritten(bad); ok {
			t.Errorf("%q reads", bad)
		}
	}
}
