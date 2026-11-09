package notifications_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// A property of an item was a relation when the commit of the item is of a
// format before 3 and the property is one of the four names or a task_ref in the
// schema.yaml of that commit; a property that is gone, or of another type, is
// not, and neither is anything in a commit of the newest format.
func TestWasRelationReadsTheFormatAndTheSchemaOfTheItemsCommit(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	_, e, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	meta, err := os.ReadFile(filepath.Join(e.Path, filepath.FromSlash(model.StoreMetaPath))) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	newest, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	ts.CommitRawFile("schema.yaml", "props:\n  fixes: {type: task_ref, multi: true, label: Fixes}\n  plain: {type: text}\n")
	ts.CommitRawFile(model.StoreMetaPath, strings.Replace(string(meta), fmt.Sprintf("format: %d", model.StoreFormat), "format: 2", 1))
	old, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		commit, key string
		want        bool
	}{
		{old.String(), "fixes", true},     // a task_ref of the store's own, whatever its value
		{old.String(), "relates", true},   // one of the four names
		{old.String(), "plain", false},    // another type
		{old.String(), "deleted", false},  // not in the schema: not a relation, ULID values or not
		{newest.String(), "fixes", false}, // the newest format
		{newest.String(), "relates", false},
		{"not-a-hash", "fixes", false},
	} {
		if got := notifications.WasRelation(s, c.commit, c.key); got != c.want {
			t.Errorf("WasRelation(%.7s, %s) = %v, want %v", c.commit, c.key, got, c.want)
		}
	}
}
