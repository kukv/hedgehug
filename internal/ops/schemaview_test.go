package ops

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/schema"
)

func TestSchemaViewOfDefault(t *testing.T) {
	v := SchemaViewOf(schema.Default())
	if len(v.Statuses) == 0 || len(v.Kinds) == 0 {
		t.Fatalf("view = %+v, want the statuses and kinds of the default", v)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"statuses", "kinds", "props", "workflow", "scoped"} {
		if m[k] == nil {
			t.Errorf("key %q missing or null in %s", k, b)
		}
	}
	if st := v.Statuses[0]; st.Name == "" || st.Category == "" {
		t.Errorf("status = %+v", st)
	}
	w, _ := m["workflow"].(map[string]any)
	if w["transitions"] == nil || w["required"] == nil {
		t.Errorf("workflow = %v, want {} for empty mappings, not null", w)
	}
}

func TestSchemaViewOfScoped(t *testing.T) {
	sc := schema.Default()
	sc.Scoped = []schema.Scoped{{
		Where:     "project:01J0000000000000000000000A",
		Statuses:  []string{"todo", "done"},
		Workflow:  &schema.Workflow{Transitions: schema.Lists{{Key: "todo", Values: []string{"done"}}}},
		Templates: &schema.Templates{Only: []string{"bug"}, Default: map[string]string{"task": "bug"}},
	}}
	v := SchemaViewOf(sc)
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
