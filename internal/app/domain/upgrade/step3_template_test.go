package upgrade

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

// Every relation that format 2 could hold in props is a relation of the table,
// and TaskRelations puts its values where the table reads them, for a template
// item too. A relation added to the table without a place in the moves fails
// here, not silently into relates.
func TestRelationMovesFollowTheTable(t *testing.T) {
	t.Parallel()
	for _, name := range LegacyRelationNames {
		rel, _, ok := schema.RelationOf(name)
		if !ok {
			t.Errorf("%s is not a relation of the table", name)
			continue
		}
		u := model.NewULID()
		tk := &model.Task{}
		tk.Props = propsOf(t, name+": ["+u.String()+"]\n")
		TaskRelations(tk, RelationDefsOf(nil))
		if got := rel.Get(tk); !eq(got, []model.ULID{u}) || len(tk.Props) != 0 {
			t.Errorf("task: %s = %v, props %v", name, got, tk.Props)
		}
		tp := &model.Template{Name: "tpl"}
		tp.Item.Props = propsOf(t, name+": ["+u.String()+"]\n")
		ItemRelations(&tp.Item, RelationDefsOf(nil))
		b, err := tp.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "\n"+name+":") || strings.Contains(string(b), "props:") {
			t.Errorf("template item: %s is not a field of the item:\n%s", name, b)
		}
	}
}

func propsOf(t *testing.T, y string) model.Fields {
	t.Helper()
	tk := task2(t, y, nil)
	got, err := model.DecodeTask(tk)
	if err != nil {
		t.Fatal(err)
	}
	return got.Props
}

func meta(format string) string {
	return "ulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nname: x\nformat: " + format + "\n"
}

func TestTemplateTo3MovesTheValuesOfTheItems(t *testing.T) {
	t.Parallel()
	id := newIDs()
	raw := []byte("---\nkind: task\ntitle: t\nprops:\n  causes: [" + id.a.String() + "]\n  fixes: [" + id.b.String() + "]\n  note: keep\nchildren:\n  - title: c\n    props:\n      relates: [" + id.c.String() + "]\n---\n")
	got, err := At(files{model.StoreMetaPath: meta("2"), schema.Path: schema2}.read, files{}.read).Template("tpl", raw)
	if err != nil {
		t.Fatal(err)
	}
	tp, err := model.DecodeTemplate("tpl", got)
	if err != nil {
		t.Fatal(err)
	}
	if !eq(tp.Item.Causes, []model.ULID{id.a}) || !eq(tp.Item.Relates, []model.ULID{id.b}) {
		t.Errorf("causes %v, relates %v", tp.Item.Causes, tp.Item.Relates)
	}
	if len(tp.Item.Props) != 1 || tp.Item.Props[0].Key != "note" {
		t.Errorf("props = %v; want note alone", tp.Item.Props)
	}
	if kid := tp.Item.Children[0]; !eq(kid.Relates, []model.ULID{id.c}) || len(kid.Props) != 0 {
		t.Errorf("child: relates %v, props %v", kid.Relates, kid.Props)
	}
	// a template with nothing to move, and one of the newest format, come back as they are
	same := []byte("---\nkind: task\ntitle: t\nprops:\n  note: keep\n---\n")
	for name, read := range map[string]func(string) ([]byte, error){
		"nothing to move": files{model.StoreMetaPath: meta("2"), schema.Path: schema2}.read,
		"newest format":   files{model.StoreMetaPath: meta("3")}.read,
	} {
		if out, err := At(read, files{}.read).Template("tpl", same); err != nil || !bytes.Equal(out, same) {
			t.Errorf("%s: %q, %v", name, out, err)
		}
	}
	// a commit whose format cannot be read is an error, never a guess
	if out, err := At(files{}.read, files{}.read).Template("tpl", raw); err == nil || out != nil {
		t.Errorf("no store.yaml: %q, %v", out, err)
	}
}
