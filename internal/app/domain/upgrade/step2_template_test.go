package upgrade

import (
	"bytes"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

// A template of format 1 is read with fixed kinds, for the item and the items
// under it: a custom group is an epic (a template item has no root), a custom
// item a task, a fixed kind stays.
func TestTemplate1to2GivesTheItemsFixedKinds(t *testing.T) {
	t.Parallel()
	raw := []byte("---\nkind: area\ntitle: t\nchildren:\n  - kind: chore\n    title: c\n  - kind: bug\n    title: b\n  - title: none\n---\n")
	at := At(files{model.StoreMetaPath: meta("1"), schema.Path: oldSchema}.read)
	got, err := at.Template("tpl", raw)
	if err != nil {
		t.Fatal(err)
	}
	tp, err := model.DecodeTemplate("tpl", got)
	if err != nil {
		t.Fatal(err)
	}
	kids := tp.Item.Children
	if tp.Item.Kind != "epic" || kids[0].Kind != "task" || kids[1].Kind != "bug" || kids[2].Kind != "" {
		t.Errorf("kinds: %q, %q, %q, %q", tp.Item.Kind, kids[0].Kind, kids[1].Kind, kids[2].Kind)
	}
	// nothing of its own: the file comes back as it is, even without a schema
	same := []byte("---\nkind: task\ntitle: t\n---\n")
	if out, err := At(files{model.StoreMetaPath: meta("1")}.read).Template("tpl", same); err != nil || !bytes.Equal(out, same) {
		t.Errorf("%q, %v", out, err)
	}
}
