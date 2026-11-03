package schema

import (
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

const scopedULID = "01J00000000000000000000001"

func scopedBase(t *testing.T, extra string) *Schema {
	t.Helper()
	b, err := Default().Encode()
	if err != nil {
		t.Fatal(err)
	}
	s, err := Decode(append(b, extra...))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestWithScopedReplacesOnlyWhatIsGiven(t *testing.T) {
	t.Parallel()
	p, _ := model.ParseULID(scopedULID)
	s := Default()
	list := []string{"todo", "in_progress", "done", "canceled"}
	a := s.WithScoped(p, ScopedChange{Statuses: &list})
	if len(s.Scoped) != 0 || len(a.Scoped) != 1 || !slices.Equal(a.Scoped[0].Statuses, list) || a.Scoped[0].Where != "project:"+scopedULID {
		t.Fatalf("a = %+v", a.Scoped)
	}
	tr := Lists{{"todo", []string{"in_progress"}}}
	b := a.WithScoped(p, ScopedChange{Transitions: &tr})
	if !slices.Equal(b.Scoped[0].Statuses, list) || b.Scoped[0].Workflow == nil || len(b.Scoped[0].Workflow.Transitions) != 1 {
		t.Fatalf("b = %+v", b.Scoped)
	}
	rq := Lists{{"done", []string{"points"}}}
	c := b.WithScoped(p, ScopedChange{Required: &rq})
	if len(c.Scoped[0].Workflow.Transitions) != 1 || len(c.Scoped[0].Workflow.Required) != 1 {
		t.Fatalf("c = %+v", c.Scoped[0].Workflow)
	}
	none := Lists{}
	d := c.WithScoped(p, ScopedChange{Transitions: &none})
	if len(d.Scoped[0].Workflow.Transitions) != 0 || len(d.Scoped[0].Workflow.Required) != 1 {
		t.Fatalf("d = %+v", d.Scoped[0].Workflow)
	}
	e := d.WithScoped(p, ScopedChange{Required: &none})
	if e.Scoped[0].Workflow != nil {
		t.Fatalf("e = %+v", e.Scoped[0])
	}
	noStatuses := []string{}
	f := e.WithScoped(p, ScopedChange{Statuses: &noStatuses})
	if len(f.Scoped) != 0 {
		t.Fatalf("f = %+v", f.Scoped)
	}
	if len(c.Scoped[0].Workflow.Transitions) != 1 {
		t.Error("an earlier schema was changed")
	}
}

func TestWithScopedKeepsTemplatesAndUnknownKeys(t *testing.T) {
	t.Parallel()
	p, _ := model.ParseULID(scopedULID)
	s := scopedBase(t, "scoped:\n  - where: project:"+scopedULID+"\n    statuses: [todo, in_progress, done, canceled]\n    note: keep me\n    templates:\n      only: [bug]\n")
	empty := []string{}
	out := s.WithScoped(p, ScopedChange{Statuses: &empty})
	if len(out.Scoped) != 1 || out.Scoped[0].Templates == nil {
		t.Fatalf("the templates keep the override: %+v", out.Scoped)
	}
	b, err := out.Encode()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("kept:\n%s", b)
	if !strings.Contains(string(b), "note: keep me") || !strings.Contains(string(b), "only: [bug]") || strings.Contains(string(b), "statuses: [todo, in_progress") {
		t.Errorf("schema.yaml:\n%s", b)
	}
	gone := s.WithoutScoped(p)
	b, err = gone.Encode()
	if err != nil {
		t.Fatal(err)
	}
	back, err := Decode(b)
	if err != nil || len(back.Scoped) != 0 {
		t.Fatalf("%v, %+v\n%s", err, back, b)
	}
	t.Logf("after removing the last override:\n%s", b[len(b)-60:])
}

func TestNewProblems(t *testing.T) {
	t.Parallel()
	p, _ := model.ParseULID(scopedULID)
	s := Default()
	hide := []string{"todo", "in_progress", "canceled"}
	next := s.WithScoped(p, ScopedChange{Statuses: &hide})
	got := next.NewProblems(s.Check())
	if len(got) != 1 || !strings.Contains(got[0].Msg, `"done" is the default of done`) {
		t.Fatalf("got %+v", got)
	}
}
