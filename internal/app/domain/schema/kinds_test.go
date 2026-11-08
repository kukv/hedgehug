package schema

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

func TestCanPlaceEveryPair(t *testing.T) {
	t.Parallel()
	parents := []string{"", "project", "epic", "story", "task", "bug", "step"}
	allowed := map[string][]string{
		"project": {""},
		"epic":    {"project"},
		"story":   {"project", "epic"},
		"task":    {"project", "epic"},
		"bug":     {"project", "epic"},
		"step":    {"story", "task", "bug"},
	}
	if len(Kinds) != len(allowed) {
		t.Fatalf("Kinds has %d entries, want %d", len(Kinds), len(allowed))
	}
	for _, k := range Kinds {
		for _, p := range parents {
			want := slices.Contains(allowed[k.Name], p)
			if got := CanPlace(k.Name, p); got != want {
				t.Errorf("CanPlace(%q, %q) = %v, want %v", k.Name, p, got, want)
			}
		}
	}
	if CanPlace("nope", "") || CanPlace("task", "nope") {
		t.Error("an unknown kind is never placed")
	}
}

func TestKindOrderRolesAndDefault(t *testing.T) {
	t.Parallel()
	var names []string
	for _, k := range Kinds {
		names = append(names, k.Name)
	}
	if !slices.Equal(names, []string{"project", "epic", "story", "task", "bug", "step"}) {
		t.Errorf("Kinds = %v", names)
	}
	for parent, want := range map[string]string{"": "project", "project": "task", "epic": "task", "story": "step", "task": "step", "bug": "step"} {
		if got := DefaultKindUnder(parent); got != want {
			t.Errorf("DefaultKindUnder(%q) = %q, want %q", parent, got, want)
		}
		if !CanPlace(DefaultKindUnder(parent), parent) {
			t.Errorf("the default kind under %q cannot be placed there", parent)
		}
	}
	for name, group := range map[string]bool{"project": true, "epic": true, "story": false, "task": false, "bug": false, "step": false, "nope": false} {
		if IsGroup(name) != group {
			t.Errorf("IsGroup(%q) = %v", name, !group)
		}
	}
	for name, counts := range map[string]bool{"project": false, "epic": false, "story": true, "task": true, "bug": true, "step": false, "nope": false} {
		if CountsForProgress(name) != counts {
			t.Errorf("CountsForProgress(%q) = %v", name, !counts)
		}
	}
	if k, ok := KindOf("bug"); !ok || k.Label != "Bug" || k.Role != RoleItem {
		t.Errorf("KindOf(bug) = %+v, %v", k, ok)
	}
	if _, ok := KindOf("nope"); ok {
		t.Error("KindOf(nope) found a kind")
	}
}

func TestStoreTemplatesMergeWithScoped(t *testing.T) {
	t.Parallel()
	s := Default()
	s.Templates = &Templates{Default: map[string]string{"bug": "bug-report", "task": "plain"}}
	where := "project:01JBZ8K3QW7M2X9V4T6R5N8P0B"
	s.Scoped = []Scoped{{Where: where, Templates: &Templates{Only: []string{"a"}, Default: map[string]string{"task": "scoped-task"}}}}

	only, def := s.For(nil).Templates()
	if only != nil || def["bug"] != "bug-report" || def["task"] != "plain" {
		t.Errorf("store: %v, %v", only, def)
	}
	only, def = s.For([]model.ULID{mustULID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0B")}).Templates()
	if !slices.Equal(only, []string{"a"}) || def["bug"] != "bug-report" || def["task"] != "scoped-task" {
		t.Errorf("scoped wins: %v, %v", only, def)
	}
	if s.Templates.Default["task"] != "plain" {
		t.Error("the store's map was changed by Templates()")
	}
}

func TestCheckTemplatesDefaultKinds(t *testing.T) {
	t.Parallel()
	s := Default()
	s.Templates = &Templates{Default: map[string]string{"nope": "x", "task": "../y"}}
	var unknown, badName bool
	for _, p := range s.Check() {
		switch p.Code {
		case "schema.unknown_kind":
			unknown = p.Path == "templates.default" && len(p.Args) == 1 && p.Args[0] == "nope"
		case "schema.bad_template_name":
			badName = p.Path == "templates.default"
		}
	}
	if !unknown || !badName {
		t.Errorf("unknown_kind=%v bad_template_name=%v in %v", unknown, badName, s.Check())
	}
	s.Templates = &Templates{Default: map[string]string{"step": "s"}}
	if ps := s.Check(); len(ps) != 0 {
		t.Errorf("a fixed kind is fine: %v", ps)
	}
}

func TestOldKindsStayInExtraAndDoNotFailCheck(t *testing.T) {
	t.Parallel()
	s, err := Decode([]byte("statuses:\n- {name: todo, category: todo, default: true}\n- {name: in_progress, category: doing, default: true}\n- {name: done, category: done, default: true}\n- {name: canceled, category: canceled, default: true}\nkinds:\n- {name: x, role: work, default: true}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Extra.Get("kinds") == nil {
		t.Error("kinds is not kept in Extra")
	}
	if ps := s.Check(); len(ps) != 0 {
		t.Errorf("problems: %v", ps)
	}
}
