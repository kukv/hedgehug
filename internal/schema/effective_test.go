package schema

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

func id(t *testing.T, s string) model.ULID {
	t.Helper()
	v, err := model.ParseULID(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func fullSchema(t *testing.T) *Schema {
	t.Helper()
	s, err := Decode(readFull(t))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestForPicksNearestScopedAncestor(t *testing.T) {
	s := fullSchema(t)
	release := id(t, "01JBZ8K3QW7M2X9V4T6R5N8P0B") // has an override
	epic := id(t, "01JBZ8K3QW7M2X9V4T6R5N8P0F")    // has none
	ops := id(t, "01JBZ8K3QW7M2X9V4T6R5N8P0C")     // has an override

	e := s.For([]model.ULID{epic, release})
	if e.Required("done") == nil || !slices.Contains(e.Required("done"), "estimate") {
		t.Errorf("an override two levels up did not apply: %v", e.Required("done"))
	}
	if e := s.For([]model.ULID{ops, release}); len(e.Statuses()) != 5 {
		t.Errorf("the nearest override must win, statuses = %v", e.Statuses())
	}
	if e := s.For(nil); len(e.Statuses()) != 7 {
		t.Errorf("no ancestors: statuses = %v", e.Statuses())
	}
}

func TestCanTransition(t *testing.T) {
	s := fullSchema(t)
	store := s.For(nil)
	release := s.For([]model.ULID{id(t, "01JBZ8K3QW7M2X9V4T6R5N8P0B")})
	for _, tc := range []struct {
		e        *Effective
		from, to string
		want     bool
	}{
		{store, "backlog", "todo", true},
		{store, "backlog", "done", false},   // store limits backlog
		{store, "todo", "done", true},       // todo is not limited
		{store, "done", "done", true},       // same status is always allowed
		{release, "waiting", "done", false}, // project limits waiting
		{release, "waiting", "in_progress", true},
		{release, "backlog", "done", false}, // store limits still apply
	} {
		if got := tc.e.CanTransition(tc.from, tc.to); got != tc.want {
			t.Errorf("CanTransition(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestRequiredIsUnion(t *testing.T) {
	s := fullSchema(t)
	e := s.For([]model.ULID{id(t, "01JBZ8K3QW7M2X9V4T6R5N8P0B")})
	got := e.Required("done")
	slices.Sort(got)
	if !slices.Equal(got, []string{"estimate", "points"}) {
		t.Errorf("Required(done) = %v, want [estimate points]", got)
	}
	if got := e.Required("in_progress"); !slices.Equal(got, []string{"assignees"}) {
		t.Errorf("Required(in_progress) = %v", got)
	}
}

func TestStatusesKeepStoreOrder(t *testing.T) {
	s := fullSchema(t)
	e := s.For([]model.ULID{id(t, "01JBZ8K3QW7M2X9V4T6R5N8P0C")})
	var names []string
	for _, st := range e.Statuses() {
		names = append(names, st.Name)
	}
	if !slices.Equal(names, []string{"todo", "on_hold", "in_progress", "done", "canceled"}) {
		t.Errorf("Statuses = %v", names)
	}
}

func TestTemplates(t *testing.T) {
	s := fullSchema(t)
	only, def := s.For([]model.ULID{id(t, "01JBZ8K3QW7M2X9V4T6R5N8P0B")}).Templates()
	if !slices.Equal(only, []string{"release-checklist", "api-change"}) || def["task"] != "release-checklist" {
		t.Errorf("Templates = %v, %v", only, def)
	}
	if only, def := s.For(nil).Templates(); only != nil || def["bug"] != "bug-report" {
		t.Errorf("store Templates = %v, %v", only, def)
	}
}
