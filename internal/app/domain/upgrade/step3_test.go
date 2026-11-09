package upgrade

import (
	"bytes"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

// schema2 is the props of a schema.yaml of format 2: the three relations as
// task_ref properties, one of the store's own, and a text property.
const schema2 = `props:
  duplicates: {type: task_ref, multi: true, label: Duplicates, inverse: Duplicated by}
  causes: {type: task_ref, multi: true, label: Causes, inverse: Caused by}
  relates: {type: task_ref, multi: true, label: Relates to, symmetric: true}
  fixes: {type: task_ref, label: Fixes}
  note: {type: text}
`

type ids struct{ a, b, c model.ULID }

func newIDs() ids { return ids{model.NewULID(), model.NewULID(), model.NewULID()} }

// task2 is the file of a task of format 2 whose props are the YAML props, with
// the relations the task already has.
func task2(t *testing.T, props string, fn func(*model.Task)) []byte {
	t.Helper()
	tk := &model.Task{
		ULID: model.NewULID(), ID: "ME-1", Workspace: model.NewULID(), Kind: "task", Title: "x", Status: "todo",
		Created: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Updated: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
	}
	if err := yaml.Unmarshal([]byte(props), &tk.Props); err != nil {
		t.Fatal(err)
	}
	if fn != nil {
		fn(tk)
	}
	b, err := tk.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func up(t *testing.T, raw []byte, sc string) *model.Task {
	t.Helper()
	got, err := Upgrade(2, raw, Cx{Schema: []byte(sc)})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := model.DecodeTask(got)
	if err != nil {
		t.Fatal(err)
	}
	return tk
}

func eq(a, b []model.ULID) bool { return slices.Equal(a, b) }

func TestTask2to3MovesTheTaskRefValuesToTheRelations(t *testing.T) {
	t.Parallel()
	id := newIDs()
	props := "duplicates: [" + id.a.String() + "]\ncauses: " + id.b.String() + "\nrelates: [" + id.c.String() + ", " + id.a.String() + "]\nfixes: " + id.b.String() + "\nnote: keep\n"
	got := up(t, task2(t, props, nil), schema2)
	if !eq(got.Duplicates, []model.ULID{id.a}) {
		t.Errorf("duplicates = %v", got.Duplicates)
	}
	if !eq(got.Causes, []model.ULID{id.b}) { // a single value is a list of one
		t.Errorf("causes = %v", got.Causes)
	}
	// relates keeps its own values, then the property of the store's own goes after
	if !eq(got.Relates, []model.ULID{id.c, id.a, id.b}) {
		t.Errorf("relates = %v", got.Relates)
	}
	if len(got.Props) != 1 || got.Props[0].Key != "note" {
		t.Errorf("props = %v; want note alone", got.Props)
	}
}

func TestTask2to3DoesNotMoveUpdatedAndKeepsTheOtherBytes(t *testing.T) {
	t.Parallel()
	id := newIDs()
	raw := task2(t, "causes: ["+id.a.String()+"]\nnote: keep\n", nil)
	was, _ := model.DecodeTask(raw)
	got := up(t, raw, schema2)
	if !got.Updated.Equal(was.Updated) {
		t.Errorf("updated moved: %v to %v", was.Updated, got.Updated)
	}
	if got.Title != was.Title || got.ID != was.ID || got.Kind != was.Kind || got.Status != was.Status || !got.Created.Equal(was.Created) {
		t.Errorf("another field changed: %+v", got)
	}
	// the file of a task with nothing to move comes back as it is
	same := task2(t, "note: keep\n", nil)
	if out, err := Upgrade(2, same, Cx{Schema: []byte(schema2)}); err != nil || !bytes.Equal(out, same) {
		t.Errorf("a task with nothing to move changed: %q, %v", out, err)
	}
}

func TestTask2to3NoValuesOrSchemaIsNothingToDo(t *testing.T) {
	t.Parallel()
	same := task2(t, "note: keep\n", nil)
	if out, err := Upgrade(2, same, Cx{}); err != nil || !bytes.Equal(out, same) {
		t.Errorf("no schema: %q, %v", out, err)
	}
	// a null value goes with its key and moves nothing
	got := up(t, task2(t, "causes: null\nrelates: []\n", nil), schema2)
	if len(got.Props) != 0 || len(got.Causes) != 0 || len(got.Relates) != 0 {
		t.Errorf("props %v, causes %v, relates %v", got.Props, got.Causes, got.Relates)
	}
}

func TestTask2to3JoinsWhatTheFieldHoldsAndCollapsesDuplicates(t *testing.T) {
	t.Parallel()
	id := newIDs()
	raw := task2(t, "relates: ["+id.a.String()+", "+id.b.String()+", "+id.a.String()+"]\n", func(tk *model.Task) { tk.Relates = []model.ULID{id.c, id.b} })
	got := up(t, raw, schema2)
	if !eq(got.Relates, []model.ULID{id.c, id.b, id.a}) {
		t.Errorf("relates = %v; want the field's values, then the new ones, none twice", got.Relates)
	}
}

func TestTask2to3WithoutADefinitionMovesOnlyULIDs(t *testing.T) {
	t.Parallel()
	id := newIDs()
	// the definitions went (a store converted by an older version, or one that
	// removed them); a value that is a ULID is a relation, one that is not stays
	props := "duplicates: [" + id.a.String() + "]\nclones: " + id.b.String() + "\nrelates: [" + id.c.String() + ", not-an-id]\ncauses: not-an-id\n"
	got := up(t, task2(t, props, nil), "")
	if !eq(got.Duplicates, []model.ULID{id.a}) || !eq(got.Clones, []model.ULID{id.b}) {
		t.Errorf("duplicates %v, clones %v", got.Duplicates, got.Clones)
	}
	if len(got.Relates) != 0 || len(got.Causes) != 0 {
		t.Errorf("relates %v, causes %v; a property with a value that is not a ULID stays whole", got.Relates, got.Causes)
	}
	keys := []string{}
	for _, f := range got.Props {
		keys = append(keys, f.Key)
	}
	if !slices.Equal(keys, []string{"relates", "causes"}) {
		t.Errorf("props = %v", keys)
	}
}

func TestTask2to3AProperyOfAnotherTypeIsNotARelation(t *testing.T) {
	t.Parallel()
	id := newIDs()
	sc := "props:\n  relates: {type: text}\n"
	got := up(t, task2(t, "relates: "+id.a.String()+"\n", nil), sc)
	if len(got.Relates) != 0 || got.Props.Get("relates") == nil {
		t.Errorf("relates %v, props %v; a text property stays", got.Relates, got.Props)
	}
}

func TestTask2to3DropsTheValueThatIsNotAULIDOfATaskRef(t *testing.T) {
	t.Parallel()
	id := newIDs()
	got := up(t, task2(t, "causes: ["+id.a.String()+", WEB-3, 7]\n", nil), schema2)
	if !eq(got.Causes, []model.ULID{id.a}) || len(got.Props) != 0 {
		t.Errorf("causes %v, props %v", got.Causes, got.Props)
	}
}

func TestRelationDefsOf(t *testing.T) {
	t.Parallel()
	sc, err := schema.Decode([]byte(schema2))
	if err != nil {
		t.Fatal(err)
	}
	d := RelationDefsOf(sc)
	if got := d.Names(); !slices.Equal(got, []string{"duplicates", "causes", "relates", "fixes"}) {
		t.Errorf("names = %v", got)
	}
	for name, want := range map[string]string{"duplicates": "duplicates", "causes": "causes", "relates": "relates", "fixes": "relates", "clones": "clones"} {
		if to, _, ok := d.Target(name); !ok || to != want {
			t.Errorf("target of %s = %q, %v; want %q", name, to, ok, want)
		}
	}
	if _, _, ok := d.Target("note"); ok {
		t.Error("a text property has a target")
	}
	if _, defined, _ := d.Target("clones"); defined {
		t.Error("clones is defined")
	}
}

func TestLegacyConflictsAndMoveLegacyFields(t *testing.T) {
	t.Parallel()
	id := newIDs()
	both := task2(t, "relates: ["+id.a.String()+"]\ncauses: ["+id.b.String()+"]\n", func(tk *model.Task) { tk.Relates = []model.ULID{id.c} })
	tk, err := model.DecodeTask(both)
	if err != nil {
		t.Fatal(err)
	}
	if got := LegacyConflicts(tk); !slices.Equal(got, []string{"relates"}) {
		t.Errorf("conflicts = %v; want relates only", got)
	}
	out, err := MoveLegacyFields(both)
	if err != nil {
		t.Fatal(err)
	}
	moved, _ := model.DecodeTask(out)
	if len(moved.Causes) != 1 || moved.Props.Get("causes") != nil {
		t.Errorf("causes %v, props %v", moved.Causes, moved.Props)
	}
	if !strings.Contains(string(out), "relates") {
		t.Errorf("file = %q", out)
	}
}

func TestItemRelationsMovesTheChildrenToo(t *testing.T) {
	t.Parallel()
	id := newIDs()
	it := model.TemplateItem{Title: "a", Children: []model.TemplateItem{{Title: "b"}}}
	if err := yaml.Unmarshal([]byte("causes: ["+id.a.String()+"]\nfixes: "+id.b.String()+"\n"), &it.Props); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal([]byte("relates: ["+id.c.String()+"]\n"), &it.Children[0].Props); err != nil {
		t.Fatal(err)
	}
	sc, _ := schema.Decode([]byte(schema2))
	moved := ItemRelations(&it, RelationDefsOf(sc))
	if len(moved) != 3 || !eq(it.Causes, []model.ULID{id.a}) || !eq(it.Relates, []model.ULID{id.b}) || !eq(it.Children[0].Relates, []model.ULID{id.c}) {
		t.Errorf("moved %+v; causes %v, relates %v, child %v", moved, it.Causes, it.Relates, it.Children[0].Relates)
	}
	if len(it.Props) != 0 || len(it.Children[0].Props) != 0 {
		t.Errorf("props are left: %v, %v", it.Props, it.Children[0].Props)
	}
}
