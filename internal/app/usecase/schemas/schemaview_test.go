package schemas_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
)

func TestSchemaViewOfDefault(t *testing.T) {
	t.Parallel()
	v := schemas.SchemaViewOf(schema.Default())
	if len(v.Statuses) == 0 {
		t.Fatalf("view = %+v, want the statuses of the default", v)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"statuses", "props", "workflow", "scoped"} {
		if m[k] == nil {
			t.Errorf("key %q missing or null in %s", k, b)
		}
	}
	if st := v.Statuses[0]; st.Name == "" || st.Category == "" {
		t.Errorf("status = %+v", st)
	}
	if _, ok := m["kinds"]; ok {
		t.Errorf("kinds is in the view: %s", b)
	}
	if _, ok := m["templates"]; ok {
		t.Errorf("a schema with no store templates has templates in the view: %s", b)
	}
	w, _ := m["workflow"].(map[string]any)
	if w["transitions"] == nil || w["required"] == nil {
		t.Errorf("workflow = %v, want {} for empty mappings, not null", w)
	}
}

func TestSchemaViewOfPropOrder(t *testing.T) {
	t.Parallel()
	sc := schema.Default()
	sc.Props = schema.Props{{Name: "level", Type: schema.TypeText}, {Name: "begin", Type: schema.TypeDate}}
	got := strings.Join(schemas.SchemaViewOf(sc).PropOrder, " ")
	if want := "level begin"; got != want {
		t.Errorf("PropOrder = %q, want %q", got, want)
	}
}

// The two lists of the schema are in the view, in order, and are never null.
func TestSchemaViewOfLists(t *testing.T) {
	t.Parallel()
	v := schemas.SchemaViewOf(schema.Default())
	if strings.Join(v.Priorities, " ") != "low mid high urgent" || v.Labels == nil || len(v.Labels) != 0 {
		t.Errorf("priorities = %v, labels = %#v", v.Priorities, v.Labels)
	}
	sc := schema.Default()
	sc.Labels = []string{"bug", "ui"}
	b, err := json.Marshal(schemas.SchemaViewOf(sc))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"labels":["bug","ui"]`) || !strings.Contains(string(b), `"priorities":["low","mid","high","urgent"]`) {
		t.Errorf("json = %s", b)
	}
}

func TestSchemaViewOfScoped(t *testing.T) {
	t.Parallel()
	sc := schema.Default()
	sc.Scoped = []schema.Scoped{{
		Where:     "project:01J0000000000000000000000A",
		Statuses:  []string{"todo", "done"},
		Workflow:  &schema.Workflow{Transitions: schema.Lists{{Key: "todo", Values: []string{"done"}}}},
		Templates: &schema.Templates{Only: []string{"bug"}, Default: map[string]string{"task": "bug"}},
	}}
	v := schemas.SchemaViewOf(sc)
	b, err := json.Marshal(v.Scoped)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"where":"project:01J0000000000000000000000A","statuses":["todo","done"],` +
		`"workflow":{"transitions":{"todo":["done"]},"required":{}},` +
		`"templates":{"only":["bug"],"default":{"task":"bug"}}}]`
	if string(b) != want {
		t.Errorf("scoped = %s\nwant     %s", b, want)
	}
}

func TestSchemaViewOfStoreTemplates(t *testing.T) {
	t.Parallel()
	sc := schema.Default()
	sc.Templates = &schema.Templates{Default: map[string]string{"bug": "bug-report"}}
	v := schemas.SchemaViewOf(sc)
	if v.Templates == nil || v.Templates.Default["bug"] != "bug-report" {
		t.Errorf("templates = %+v", v.Templates)
	}
}
