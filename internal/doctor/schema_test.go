package doctor

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func TestSchemaFileBroken(t *testing.T) {
	t.Parallel()
	sc, fs := SchemaFile([]byte("statuses: [\n"))
	if sc != nil {
		t.Errorf("schema = %v, want nil", sc)
	}
	if len(fs) != 1 || fs[0].Check != "schema" || fs[0].Where != "schema.yaml" || !strings.Contains(fs[0].Hint, "git restore schema.yaml") {
		t.Fatalf("findings = %+v", fs)
	}
}

func TestSchemaFileProblems(t *testing.T) {
	t.Parallel()
	sc := schema.Default()
	sc.Statuses = append(sc.Statuses, sc.Statuses[0])
	data, err := sc.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, fs := SchemaFile(data)
	if got == nil || len(fs) == 0 {
		t.Fatalf("schema = %v, findings = %+v", got, fs)
	}
	for _, f := range fs {
		if f.Check != "schema" || !strings.HasPrefix(f.Where, "schema.yaml: ") {
			t.Errorf("finding = %+v", f)
		}
	}
}

func TestSchemaClean(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t)
	put(t, s, newTask("A"))
	fs, err := Schema(s, mustOpen(t, s))
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 0 {
		t.Errorf("findings = %+v, want none", fs)
	}
}

func TestSchemaEachCheck(t *testing.T) {
	t.Parallel()
	s, dir := newStore(t)
	bad := newTask("Bad")
	bad.Status = "weird"
	putRaw(t, s, dir, bad)
	s = rewriteSchema(t, dir, func(sc *schema.Schema) {
		sc.Kinds[0].Template = "nope"
		sc.Props = append(sc.Props, schema.Prop{Name: "label", Type: schema.TypeText})
		sc.Scoped = []schema.Scoped{{Where: "project:" + model.NewULID().String()}}
	})
	fs, err := Schema(s, mustOpen(t, s))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"template", "prop_name", "override", "task"}
	if got := checks(fs); !slices.Equal(got, want) {
		t.Fatalf("checks = %v, want %v\n%+v", got, want, fs)
	}
	if fs[0].Where != "schema.yaml: kinds[0].template" || !strings.Contains(fs[0].Msg, "nope") {
		t.Errorf("template = %+v", fs[0])
	}
	if fs[1].Where != "schema.yaml: props.label" || fs[1].Msg != "queries cannot select by a property with this name" {
		t.Errorf("prop_name = %+v", fs[1])
	}
	if fs[2].Where != "schema.yaml: scoped[0].where" {
		t.Errorf("override = %+v", fs[2])
	}
	if !strings.HasSuffix(fs[3].Where, " Bad") || !strings.Contains(fs[3].Msg, "weird") {
		t.Errorf("task = %+v", fs[3])
	}
}

func TestSchemaScopedTemplates(t *testing.T) {
	t.Parallel()
	s, dir := newStore(t)
	p := newTask("Proj")
	p.Kind = "project"
	put(t, s, p)
	s = rewriteSchema(t, dir, func(sc *schema.Schema) {
		sc.Scoped = []schema.Scoped{{
			Where:     "project:" + p.ULID.String(),
			Templates: &schema.Templates{Only: []string{"a"}, Default: map[string]string{"task": "b"}},
		}}
	})
	fs, err := Schema(s, mustOpen(t, s))
	if err != nil {
		t.Fatal(err)
	}
	var wheres []string
	for _, f := range fs {
		wheres = append(wheres, f.Check+" "+f.Where)
	}
	want := []string{"template schema.yaml: scoped[0].templates.default.task", "template schema.yaml: scoped[0].templates.only[0]"}
	if !slices.Equal(wheres, want) {
		t.Errorf("findings = %v, want %v", wheres, want)
	}
}

func TestSchemaOverrideOfDeletedProject(t *testing.T) {
	t.Parallel()
	s, dir := newStore(t)
	gone := newTask("Gone")
	gone.Kind = "project"
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	gone.DeletedAt = &at
	live := newTask("Live")
	live.Kind = "project"
	put(t, s, gone, live)
	s = rewriteSchema(t, dir, func(sc *schema.Schema) {
		sc.Scoped = []schema.Scoped{{Where: "project:" + gone.ULID.String()}, {Where: "project:" + live.ULID.String()}}
	})
	fs, err := Schema(s, mustOpen(t, s))
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 || fs[0].Check != "override" || fs[0].Where != "schema.yaml: scoped[0].where" {
		t.Errorf("findings = %+v", fs)
	}
}

func TestSort(t *testing.T) {
	t.Parallel()
	fs := []Finding{{Check: "task", Where: "b"}, {Check: "template", Where: "z"}, {Check: "schema", Where: "y"}, {Check: "task", Where: "a"}, {Check: "override", Where: "q"}}
	Sort(fs)
	want := []string{"schema", "template", "override", "task", "task"}
	if got := checks(fs); !slices.Equal(got, want) {
		t.Fatalf("checks = %v, want %v", got, want)
	}
	if fs[3].Where != "a" || fs[4].Where != "b" {
		t.Errorf("where order = %v %v", fs[3].Where, fs[4].Where)
	}
}
