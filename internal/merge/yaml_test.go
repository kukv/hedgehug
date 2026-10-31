package merge

import (
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func TestNumbersNeverGoDown(t *testing.T) {
	const base = "WEB: 3\nAPI: 2\n"
	for _, c := range []struct {
		name         string
		ours, theirs []byte
		want         string
	}{
		{"one side lowers", []byte("WEB: 1\nAPI: 2\n"), []byte(base), base},
		{"one side removes the file, the other lowers", nil, []byte("WEB: 1\nAPI: 2\n"), base},
		{"one side removes the file", nil, []byte(base), base},
		{"both sides remove a prefix", []byte("WEB: 4\n"), []byte("WEB: 5\n"), "WEB: 5\n"},
	} {
		r := mergeSym(t, model.NumbersPath, Side{Data: []byte(base)}, Side{Data: c.ours, Author: "a"}, Side{Data: c.theirs, Author: "b"})
		if string(r.Data) != c.want || len(r.Overwritten) != 0 || len(r.Notices) != 0 {
			t.Errorf("%s: %q %v %v", c.name, r.Data, r.Overwritten, r.Notices)
		}
	}
}

func TestYAMLDuplicateItemKeysMergeWhole(t *testing.T) {
	base := []byte("statuses:\n- {name: a, x: 1}\n")
	ours := []byte("statuses:\n- {name: a, x: 1}\n- {name: a, x: 2}\n")
	theirs := []byte("statuses:\n- {name: a, x: 3}\n")
	r := mergeSym(t, schema.Path, Side{Data: base}, Side{Data: ours, Author: "a"}, Side{Data: theirs, Author: "b", When: whenAt(time.Unix(1, 0))})
	if string(r.Data) != string(theirs) {
		t.Errorf("r =\n%s", r.Data)
	}
	want := []Overwritten{{Path: schema.Path, Field: "statuses", Value: `[{"name":"a","x":1},{"name":"a","x":2}]`, Author: "a"}}
	if !sameOverwritten(r.Overwritten, want) {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

// The result writes its lists as the side it is built on does.
func TestYAMLKeepsListIndent(t *testing.T) {
	const (
		indented = "rules:\n  - id: r1\n    do: b\n"
		flush    = "rules:\n- id: r1\n  do: a\n- id: r2\n  do: a\n"
	)
	for _, c := range []struct {
		name, top, other, want string
	}{
		{"indented top", indented, flush, "rules:\n  - id: r1\n    do: b\n  - id: r2\n    do: a\n"},
		{"flush top", "rules:\n- id: r1\n  do: b\n", "rules:\n  - id: r1\n    do: a\n  - id: r2\n    do: a\n", "rules:\n- id: r1\n  do: b\n- id: r2\n  do: a\n"},
		{"top without block lists", "rules: [{id: r1, do: b}]\n", "rules:\n  - id: r1\n    do: a\n  - id: r2\n    do: a\n", "rules: [{id: r1, do: b}, {id: r2, do: a}]\n"},
	} {
		r := mergeSym(t, model.AutomationsPath, Side{Data: []byte("rules:\n- id: r1\n  do: a\n")}, Side{Data: []byte(c.top), Author: "b"}, Side{Data: []byte(c.other), Author: "a"})
		if string(r.Data) != c.want {
			t.Errorf("%s:\n%s\nwant\n%s", c.name, r.Data, c.want)
		}
	}
}

// yaml.v4 indents all the lists of a file alike, so a file that mixes the
// styles comes out in the style of its first block list.
func TestYAMLMixedListIndent(t *testing.T) {
	base := []byte("a:\n- x\nb:\n- y\nc: 0\n")
	ours := []byte("a:\n  - x\nb:\n- y\nc: 1\n")
	theirs := []byte("a:\n- x\nb:\n- y\nc: 2\n")
	r := mergeSym(t, model.StoreMetaPath, Side{Data: base}, Side{Data: ours, Author: "b"}, Side{Data: theirs, Author: "a"})
	if want := "a:\n  - x\nb:\n  - y\nc: 1\n"; string(r.Data) != want {
		t.Errorf("r =\n%s", r.Data)
	}
}

func TestNumbersUnreadableSide(t *testing.T) {
	broken := []byte("WEB: [\n")
	r := mergeSym(t, model.NumbersPath, Side{Data: []byte("WEB: 3\nAPI: 2\n")}, Side{Data: broken, Author: "a"}, Side{Data: []byte("WEB: 1\n"), Author: "b"})
	n, err := model.DecodeNumbers(r.Data)
	if err != nil || n["WEB"] != 3 || n["API"] != 2 || len(n) != 2 {
		t.Errorf("numbers = %v, %v", n, err)
	}
	if len(r.Overwritten) != 1 || r.Overwritten[0].Field != "*" || r.Overwritten[0].Author != "a" || len(r.Overwritten[0].Value) != 40 {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

func TestWhenOf(t *testing.T) {
	t1, t2 := time.Unix(1, 0), time.Unix(2, 0)
	when := WhenOf(map[string]time.Time{PathKey([]string{"statuses", "a", "x"}): t2, PathKey([]string{"props"}): t1})
	for _, c := range []struct {
		path []string
		want time.Time
	}{
		{[]string{"statuses"}, t2},           // a value under it
		{[]string{"statuses", "a", "x"}, t2}, // the value
		{[]string{"props", "p", "type"}, t1}, // a value above it
		{[]string{"statusesx"}, time.Time{}}, // not a path under statuses
		{[]string{"x"}, time.Time{}},
		{nil, t2}, // the root is above all
	} {
		if got := when(c.path); !got.Equal(c.want) {
			t.Errorf("%q: %s, want %s", c.path, got, c.want)
		}
	}
}

// A list whose items share an identity key is one value: When is asked
// about the list, and answers with the changes under it.
func TestYAMLDuplicateItemKeysThroughWhen(t *testing.T) {
	at := func(u int64) func([]string) time.Time {
		return WhenOf(map[string]time.Time{PathKey([]string{"statuses", "a", "x"}): time.Unix(u, 0)})
	}
	base := []byte("statuses:\n- {name: a, x: 1}\n")
	ours := []byte("statuses:\n- {name: a, x: 1}\n- {name: a, x: 2}\n")
	theirs := []byte("statuses:\n- {name: a, x: 3}\n")
	// ours changed it later; the authors alone would pick theirs.
	r := mergeSym(t, schema.Path, Side{Data: base}, Side{Data: ours, Author: "a", When: at(2)}, Side{Data: theirs, Author: "b", When: at(1)})
	if string(r.Data) != string(ours) || len(r.Overwritten) != 1 || r.Overwritten[0].Author != "b" {
		t.Errorf("r = %s %v", r.Data, r.Overwritten)
	}
}

func TestYAMLBlankSide(t *testing.T) {
	base := []byte("a: 1\n")
	theirs := []byte("a: 2\nb: 1\n")
	for _, c := range []struct{ ours, want string }{
		{"", "a: 2\nb: 1\n"},
		{"# only a comment\n", "# only a comment\na: 2\nb: 1\n"},
		// Each line is kept as it is, with LF line ends.
		{"#a\r\n   #b\r\n\r\n# c", "#a\n   #b\n\n# c\na: 2\nb: 1\n"},
	} {
		r := mergeSym(t, schema.Path, Side{Data: base}, Side{Data: []byte(c.ours), Author: "z"}, Side{Data: theirs, Author: "a"})
		// ours removed a, which theirs changed: a stays, and ours is told.
		if string(r.Data) != c.want {
			t.Errorf("%q: r = %q", c.ours, r.Data)
		}
		if want := []Notice{{Kind: Revived, Path: schema.Path, Author: "z"}}; !slices.Equal(r.Notices, want) {
			t.Errorf("%q: notices = %v", c.ours, r.Notices)
		}
	}
}

// When is asked about the paths YAMLValues lists, and only those.
func TestYAMLValuesAreWhatWhenIsAsked(t *testing.T) {
	doc := func(v string) []byte {
		return []byte("statuses:\n- {name: todo, category: " + v + "}\n- {name: done, category: " + v + "}\nprops:\n  p: {type: " + v +
			", values: [" + v + "]}\nworkflow:\n  transitions: {todo: [" + v + "]}\nx: " + v + "\n")
	}
	base, ours, theirs := doc("a"), doc("b"), doc("c")
	values, err := YAMLValues(ours)
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, v := range values {
		listed[strings.Join(v.Path, ".")] = true
	}
	asked := map[string]bool{}
	record := func(p []string) time.Time {
		asked[strings.Join(p, ".")] = true
		return time.Time{}
	}
	mergeSym(t, schema.Path, Side{Data: base}, Side{Data: ours, Author: "a", When: record}, Side{Data: theirs, Author: "b", When: record})
	want := map[string]bool{
		"statuses.todo.category": true, "statuses.done.category": true,
		"props.p.type": true, "props.p.values": true, "workflow.transitions.todo": true, "x": true,
	}
	if !maps.Equal(listed, want) || !maps.Equal(asked, want) {
		t.Errorf("listed %v\nasked %v", listed, asked)
	}
}

func TestYAMLValuesOfBlankAndBroken(t *testing.T) {
	if v, err := YAMLValues([]byte("# c\n")); err != nil || len(v) != 0 {
		t.Errorf("blank: %v %v", v, err)
	}
	if _, err := YAMLValues([]byte("a: [\n")); err == nil {
		t.Error("broken yaml read")
	}
}
