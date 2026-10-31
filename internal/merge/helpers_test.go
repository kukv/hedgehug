package merge

import (
	"bytes"
	"cmp"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// task builds a task file. Fields are given as "key: value" lines.
func task(updated string, lines ...string) []byte {
	return []byte("---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nid: WEB-1\nworkspace: 01JC0A1B2C3D4E5F6G7H8J9K0M\nkind: task\n" +
		strings.Join(lines, "\n") + "\nupdated: " + updated + "\n---\n")
}

var taskPath = model.TaskPath(mustULID("01JBZ8K3QW7M2X9V4T6R5N8P0A"))

func mustULID(s string) model.ULID {
	u, err := model.ParseULID(s)
	if err != nil {
		panic(err)
	}
	return u
}

func side(data []byte, author string) Side { return Side{Data: data, Author: author} }

func mergeTask(t *testing.T, base, ours, theirs Side) Result {
	t.Helper()
	return mergeWith(t, LWW, base, ours, theirs)
}

// mergeWith merges taskPath under p. Under LWW it checks that swapping
// ours and theirs gives the same result.
func mergeWith(t *testing.T, p Policy, base, ours, theirs Side) Result {
	t.Helper()
	r, err := Merge(taskPath, schema.Default(), base.Data, ours, theirs, p)
	if err != nil {
		t.Fatal(err)
	}
	if p != LWW {
		return r
	}
	// Every merge must be symmetric.
	r2, err := Merge(taskPath, schema.Default(), base.Data, theirs, ours, LWW)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r.Data, r2.Data) || !sameOverwritten(r.Overwritten, r2.Overwritten) || !slices.Equal(r.Notices, r2.Notices) {
		t.Fatalf("not symmetric:\n%s\n---\n%s\n%v\n%v\n%v\n%v", r.Data, r2.Data, r.Overwritten, r2.Overwritten, r.Notices, r2.Notices)
	}
	return r
}

func decoded(t *testing.T, b []byte) *model.Task {
	t.Helper()
	x, err := model.DecodeTask(b)
	if err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	return x
}

// sameOverwritten compares a and b as sets.
func sameOverwritten(a, b []Overwritten) bool {
	sorted := func(s []Overwritten) []Overwritten {
		s = slices.Clone(s)
		slices.SortFunc(s, func(x, y Overwritten) int {
			return cmp.Or(cmp.Compare(x.Path, y.Path), cmp.Compare(x.Field, y.Field),
				cmp.Compare(x.Value, y.Value), cmp.Compare(x.Author, y.Author))
		})
		return s
	}
	return slices.Equal(sorted(a), sorted(b))
}
