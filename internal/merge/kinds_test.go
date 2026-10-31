package merge

import (
	"bytes"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

const commentPath = "comments/0A/P0/01JBZ8K3QW7M2X9V4T6R5N8P0A/01JC1111111111111111111111.md"

func comment(updated, body string) []byte {
	return []byte("---\nulid: 01JC1111111111111111111111\nauthor: github.com/a\ncreated: 2026-10-31T10:00:00Z\nupdated: " + updated + "\n---\n" + body)
}

func TestCommentWholeFileLWW(t *testing.T) {
	ours := comment("2026-10-31T11:00:00Z", "a\n")
	r := mergeSym(t, commentPath, Side{Data: comment("2026-10-31T10:00:00Z", "x\n")}, Side{Data: ours, Author: "a"}, Side{Data: comment("2026-10-31T12:00:00Z", "b\n"), Author: "b"})
	if !bytes.HasSuffix(r.Data, []byte("b\n")) || len(r.Overwritten) != 1 || r.Overwritten[0].Field != "*" {
		t.Fatalf("r = %s %v", r.Data, r.Overwritten)
	}
	if o := r.Overwritten[0]; o.Author != "a" || o.Value != jsonString(ours) || o.Path != commentPath {
		t.Errorf("overwritten = %+v", o)
	}
}

func jsonString(b []byte) string { return string(jsonOf(string(b))) }

func TestWorklogWholeFileLWW(t *testing.T) {
	p := "worklogs/0A/P0/01JBZ8K3QW7M2X9V4T6R5N8P0A/01JC1111111111111111111111.md"
	w := func(updated, duration string) []byte {
		return []byte("---\nulid: 01JC1111111111111111111111\nauthor: github.com/a\ndate: 2026-10-31\nduration: " + duration + "\ncreated: 2026-10-31T10:00:00Z\nupdated: " + updated + "\n---\n")
	}
	// The same updated: the larger author wins.
	r := mergeSym(t, p, Side{Data: w("2026-10-31T10:00:00Z", "1h")}, Side{Data: w("2026-10-31T11:00:00Z", "2h"), Author: "z"}, Side{Data: w("2026-10-31T11:00:00Z", "3h"), Author: "b"})
	if !bytes.Contains(r.Data, []byte("duration: 2h")) || len(r.Overwritten) != 1 || r.Overwritten[0].Author != "b" {
		t.Errorf("r = %s %v", r.Data, r.Overwritten)
	}
}

func TestRecordOursWins(t *testing.T) {
	ours := comment("2026-10-31T11:00:00Z", "a\n")
	r := mergeAt(t, commentPath, nil, OursWins, Side{Data: comment("2026-10-31T10:00:00Z", "x\n")}, Side{Data: ours, Author: "a"}, Side{Data: comment("2026-10-31T12:00:00Z", "b\n"), Author: "b"})
	if !bytes.Equal(r.Data, ours) || len(r.Overwritten) != 0 {
		t.Errorf("r = %s %v", r.Data, r.Overwritten)
	}
}

func TestBrokenRecordFallsBackToOtherFile(t *testing.T) {
	r := mergeSym(t, commentPath, Side{Data: comment("2026-10-31T10:00:00Z", "x\n")},
		Side{Data: []byte("---\nulid: [\n---\n"), Author: "a", Time: time.Unix(2, 0)}, Side{Data: comment("2026-10-31T12:00:00Z", "b\n"), Author: "b", Time: time.Unix(1, 0)})
	if !bytes.HasPrefix(r.Data, []byte("---\nulid: [")) || len(r.Overwritten) != 1 || len(r.Overwritten[0].Value) != 40 {
		t.Errorf("r = %s %v", r.Data, r.Overwritten)
	}
}

func TestNewFilesFromOneSide(t *testing.T) {
	r := mergeSym(t, commentPath, Side{}, Side{Data: []byte("x")}, Side{})
	if string(r.Data) != "x" {
		t.Errorf("new file lost: %q", r.Data)
	}
}

// A side that removes a file the other side changed loses the removal, for
// every kind read by fields. numbers.yaml has rules of its own.
func TestRemovedAndChanged(t *testing.T) {
	for _, c := range []struct{ path, base, changed string }{
		{commentPath, string(comment("2026-10-31T10:00:00Z", "x\n")), string(comment("2026-10-31T11:00:00Z", "y\n"))},
		{"sprints/01JC0A1B2C3D4E5F6G7H8J9K0M.md", string(sprint("2026-10-31T10:00:00Z", "name: s1")), string(sprint("2026-10-31T11:00:00Z", "name: s2"))},
		{"schema.yaml", "statuses:\n  - {name: todo, category: todo}\n", "statuses:\n  - {name: todo, category: doing}\n"},
	} {
		r := mergeSym(t, c.path, Side{Data: []byte(c.base)}, Side{Author: "a"}, Side{Data: []byte(c.changed), Author: "b"})
		if string(r.Data) != c.changed || !slices.Equal(r.Notices, []Notice{{Kind: Revived, Path: c.path, Author: "a"}}) {
			t.Errorf("%s: %s %v", c.path, r.Data, r.Notices)
		}
	}
}

func workspace(prefix, prefixSet, updated, name string) []byte {
	return []byte("---\nulid: 01JC0A1B2C3D4E5F6G7H8J9K0M\nname: " + name + "\nid_prefix: " + prefix + "\nprefix_set: " + prefixSet + "\ncreated: 2026-10-30T00:00:00Z\nupdated: " + updated + "\n---\n")
}

const workspacePath = "workspaces/01JC0A1B2C3D4E5F6G7H8J9K0M.md"

func TestWorkspacePrefixPairFollowsPrefixSet(t *testing.T) {
	base := workspace("WEB", "2026-10-30T00:00:00Z", "2026-10-30T00:00:00Z", "web")
	ours := workspace("FE", "2026-10-31T12:00:00Z", "2026-10-31T12:00:00Z", "web")     // prefix set later
	theirs := workspace("UI", "2026-10-31T11:00:00Z", "2026-10-31T13:00:00Z", "front") // renamed later
	r := mergeSym(t, workspacePath, Side{Data: base}, Side{Data: ours, Author: "a"}, Side{Data: theirs, Author: "b"})
	w, err := model.DecodeWorkspace(r.Data)
	if err != nil || w.IDPrefix != "FE" || w.Name != "front" {
		t.Fatalf("workspace = %+v, %v", w, err)
	}
	if !w.PrefixSet.Equal(time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)) || !w.Updated.Equal(time.Date(2026, 10, 31, 13, 0, 0, 0, time.UTC)) {
		t.Errorf("prefix_set %s updated %s", w.PrefixSet, w.Updated)
	}
	want := []Overwritten{{Path: workspacePath, Field: "id_prefix", Value: `"UI"`, Author: "b"}}
	if !sameOverwritten(r.Overwritten, want) {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

func TestWorkspaceSamePrefixNotOverwritten(t *testing.T) {
	base := workspace("WEB", "2026-10-30T00:00:00Z", "2026-10-30T00:00:00Z", "web")
	ours := workspace("FE", "2026-10-31T12:00:00Z", "2026-10-31T12:00:00Z", "web")
	theirs := workspace("FE", "2026-10-31T11:00:00Z", "2026-10-31T11:00:00Z", "web")
	r := mergeSym(t, workspacePath, Side{Data: base}, Side{Data: ours, Author: "a"}, Side{Data: theirs, Author: "b"})
	w, err := model.DecodeWorkspace(r.Data)
	if err != nil || w.IDPrefix != "FE" || !w.PrefixSet.Equal(time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)) || len(r.Overwritten) != 0 {
		t.Errorf("workspace = %+v, %v, %v", w, err, r.Overwritten)
	}
}

func sprint(updated string, lines ...string) []byte {
	return []byte("---\nulid: 01JC0A1B2C3D4E5F6G7H8J9K0M\n" + strings.Join(lines, "\n") + "\nupdated: " + updated + "\n---\n")
}

func TestSprintFieldsAndBody(t *testing.T) {
	p := "sprints/01JC0A1B2C3D4E5F6G7H8J9K0M.md"
	base := append(sprint("2026-10-31T10:00:00Z", "name: s", "state: planned", "goal: g", "# keep", "x-note: n"), "1\n2\n3\n"...)
	ours := append(sprint("2026-10-31T11:00:00Z", "name: s2", "state: planned", "goal: ours", "# keep", "x-note: n"), "1x\n2\n3\n"...)
	theirs := append(sprint("2026-10-31T12:00:00Z", "name: s", "state: active", "goal: theirs", "# keep", "x-note: m"), "1\n2\n3x\n"...)
	r := mergeSym(t, p, Side{Data: base}, Side{Data: ours, Author: "a"}, Side{Data: theirs, Author: "b"})
	s, err := model.DecodeSprint(r.Data)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "s2" || s.State != model.SprintActive || s.Goal != "theirs" || s.Body != "1x\n2\n3x\n" || s.Extra.Get("x-note").Value != "m" {
		t.Errorf("sprint = %+v\n%s", s, r.Data)
	}
	if !bytes.Contains(r.Data, []byte("# keep")) {
		t.Errorf("comment lost:\n%s", r.Data)
	}
	want := []Overwritten{{Path: p, Field: "goal", Value: `"ours"`, Author: "a"}}
	if !sameOverwritten(r.Overwritten, want) {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

func TestReleaseFields(t *testing.T) {
	p := "releases/01JC0A1B2C3D4E5F6G7H8J9K0M.md"
	base := sprint("2026-10-31T10:00:00Z", "name: v1", "date: 2026-11-01")
	ours := sprint("2026-10-31T12:00:00Z", "name: v1.0", "date: 2026-11-01")
	theirs := sprint("2026-10-31T11:00:00Z", "name: v1", "date: 2026-11-02")
	r := mergeSym(t, p, Side{Data: base}, Side{Data: ours, Author: "a"}, Side{Data: theirs, Author: "b"})
	x, err := model.DecodeRelease(r.Data)
	if err != nil || x.Name != "v1.0" || x.Date.String() != "2026-11-02" || len(r.Overwritten) != 0 {
		t.Errorf("release = %+v, %v, %v", x, err, r.Overwritten)
	}
}

func TestTemplateFields(t *testing.T) {
	p := "templates/bug.md"
	tmpl := func(updated string, lines ...string) []byte {
		return []byte("---\n" + strings.Join(lines, "\n") + "\nupdated: " + updated + "\n---\n")
	}
	base := tmpl("2026-10-31T10:00:00Z", "title: t", "labels: [a]", "props:", "  priority: low", "  size: s")
	ours := tmpl("2026-10-31T11:00:00Z", "title: ours", "labels: [a, b]", "props:", "  priority: high", "  size: s")
	theirs := tmpl("2026-10-31T12:00:00Z", "title: theirs", "labels: [a, c]", "props:", "  priority: low", "  size: m")
	r := mergeSym(t, p, Side{Data: base}, Side{Data: ours, Author: "a"}, Side{Data: theirs, Author: "b"})
	x, err := model.DecodeTemplate("bug", r.Data)
	if err != nil {
		t.Fatal(err)
	}
	// Lists are fields too: the later side's list wins whole.
	if x.Item.Title != "theirs" || !slices.Equal(x.Item.Labels, []string{"a", "c"}) || x.Item.Props.Get("priority").Value != "high" || x.Item.Props.Get("size").Value != "m" {
		t.Errorf("template = %+v\n%s", x.Item, r.Data)
	}
	want := []Overwritten{
		{Path: p, Field: "title", Value: `"ours"`, Author: "a"},
		{Path: p, Field: "labels", Value: `["a","b"]`, Author: "a"},
	}
	if !sameOverwritten(r.Overwritten, want) {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

func whenAt(t time.Time) func([]string) time.Time { return func([]string) time.Time { return t } }

func TestSchemaYAMLByKeyAndElement(t *testing.T) {
	base := []byte("statuses:\n  - {name: todo, category: todo}\nprops:\n  priority: {type: enum, values: [low, high]}\n")
	ours := []byte("statuses:\n  - {name: todo, category: todo}\n  - {name: review, category: doing}\nprops:\n  priority: {type: enum, values: [low, high]}\n")
	theirs := []byte("statuses:\n  - {name: todo, category: todo}\n  - {name: qa, category: doing}\nprops:\n  priority: {type: enum, values: [low, mid, high]}\n")
	r := mergeSym(t, "schema.yaml", Side{Data: base}, Side{Data: ours, Author: "a", When: whenAt(time.Unix(1, 0))}, Side{Data: theirs, Author: "b", When: whenAt(time.Unix(2, 0))})
	for _, s := range []string{"review", "qa", "mid"} {
		if !bytes.Contains(r.Data, []byte(s)) {
			t.Errorf("lost %s:\n%s", s, r.Data)
		}
	}
	if bytes.Index(r.Data, []byte("qa")) > bytes.Index(r.Data, []byte("review")) {
		t.Errorf("added elements are not in key order:\n%s", r.Data)
	}
}

func TestYAMLValueByWhen(t *testing.T) {
	base := []byte("# head\nprops:\n  priority: {type: enum, values: [low, high]} # p\n")
	ours := []byte("# head\nprops:\n  priority: {type: enum, values: [low, mid]} # p\n")
	theirs := []byte("# head\nprops:\n  priority: {type: enum, values: [low, top]} # p\n")
	at := func(path []string, t time.Time) func([]string) time.Time {
		return func(p []string) time.Time {
			if slices.Equal(p, path) {
				return t
			}
			return time.Time{}
		}
	}
	values := []string{"props", "priority", "values"}
	r := mergeSym(t, "schema.yaml", Side{Data: base}, Side{Data: ours, Author: "b", When: at(values, time.Unix(2, 0))}, Side{Data: theirs, Author: "a", When: at(values, time.Unix(1, 0))})
	if !bytes.Contains(r.Data, []byte("[low, mid]")) || !bytes.Contains(r.Data, []byte("# head")) || !bytes.Contains(r.Data, []byte("# p")) {
		t.Errorf("r =\n%s", r.Data)
	}
	want := []Overwritten{{Path: "schema.yaml", Field: "props.priority.values", Value: `["low","top"]`, Author: "a"}}
	if !sameOverwritten(r.Overwritten, want) {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

func TestYAMLElements(t *testing.T) {
	// Lists are written flush with their key, as the encoders of model write them.
	rules := func(items ...string) string { return "rules:\n" + strings.Join(items, "") + "other: 1\n" }
	scoped := func(items ...string) string { return "scoped:\n" + strings.Join(items, "") }
	const (
		r1  = "- id: r1\n  do: a\n"
		r1x = "- id: r1\n  do: b\n"
		r2  = "- id: r2\n  do: a\n"
		r3  = "- id: r3\n  do: a\n"
		s1  = "- where: project:01JC0A1B2C3D4E5F6G7H8J9K0M\n  statuses: [todo]\n"
		s1x = "- where: project:01JC0A1B2C3D4E5F6G7H8J9K0M\n  statuses: [todo, done]\n"
		s2  = "- where: project:01JC0A1B2C3D4E5F6G7H8J9K0N\n  statuses: [todo]\n"
		s3  = "- where: project:01JC0A1B2C3D4E5F6G7H8J9K0P\n  statuses: [todo]\n"
	)
	for _, c := range []struct {
		name, path, base, ours, theirs, want string
		revived                              bool
	}{
		{"rules added", "automations.yaml", rules(r1), rules(r1, r3), rules(r1, r2), rules(r1, r2, r3), false},
		{"rules removed", "automations.yaml", rules(r1, r2), rules(r2), rules(r1, r2, r3), rules(r2, r3), false},
		{"rules removed and changed", "automations.yaml", rules(r1, r2), rules(r2, r3), rules(r1x, r2), rules(r1x, r2, r3), true},
		{"automations scoped added", "automations.yaml", scoped(s1), scoped(s1, s3), scoped(s1, s2), scoped(s1, s2, s3), false},
		{"automations scoped removed", "automations.yaml", scoped(s1, s2), scoped(s2), scoped(s1, s2, s3), scoped(s2, s3), false},
		{"automations scoped removed and changed", "automations.yaml", scoped(s1, s2), scoped(s2, s3), scoped(s1x, s2), scoped(s1x, s2, s3), true},
		{"schema scoped added", "schema.yaml", scoped(s1), scoped(s1, s3), scoped(s1, s2), scoped(s1, s2, s3), false},
		{"schema scoped removed", "schema.yaml", scoped(s1, s2), scoped(s2), scoped(s1, s2, s3), scoped(s2, s3), false},
		{"schema scoped removed and changed", "schema.yaml", scoped(s1, s2), scoped(s2, s3), scoped(s1x, s2), scoped(s1x, s2, s3), true},
	} {
		r := mergeSym(t, c.path, Side{Data: []byte(c.base)}, Side{Data: []byte(c.ours), Author: "a"}, Side{Data: []byte(c.theirs), Author: "b"})
		if string(r.Data) != c.want {
			t.Errorf("%s:\n%s\nwant\n%s", c.name, r.Data, c.want)
		}
		var want []Notice
		if c.revived {
			want = []Notice{{Kind: Revived, Path: c.path, Author: "a"}}
		}
		if !slices.Equal(r.Notices, want) || len(r.Overwritten) != 0 {
			t.Errorf("%s: notices %v overwritten %v", c.name, r.Notices, r.Overwritten)
		}
	}
}

func TestStoreYAMLByKey(t *testing.T) {
	base := []byte("ulid: 01JC0A1B2C3D4E5F6G7H8J9K0M\nname: a\nformat: 1\n# who reads it\naudience: team\n")
	ours := []byte("ulid: 01JC0A1B2C3D4E5F6G7H8J9K0M\nname: b\nformat: 1\n# who reads it\naudience: team\n")
	theirs := []byte("ulid: 01JC0A1B2C3D4E5F6G7H8J9K0M\nname: a\nformat: 1\n# who reads it\naudience: public\n")
	r := mergeSym(t, model.StoreMetaPath, Side{Data: base}, Side{Data: ours, Author: "a"}, Side{Data: theirs, Author: "b"})
	want := "ulid: 01JC0A1B2C3D4E5F6G7H8J9K0M\nname: b\nformat: 1\n# who reads it\naudience: public\n"
	if string(r.Data) != want {
		t.Errorf("r =\n%s", r.Data)
	}
}

func TestYAMLKeyRemovedAndChanged(t *testing.T) {
	base := []byte("name: a\naudience: team\nx: 1\n")
	ours := []byte("name: b\n")
	theirs := []byte("name: a\naudience: public\nx: 2\n")
	r := mergeSym(t, model.StoreMetaPath, Side{Data: base}, Side{Data: ours, Author: "a"}, Side{Data: theirs, Author: "b"})
	if string(r.Data) != "name: b\naudience: public\nx: 2\n" || len(r.Overwritten) != 0 {
		t.Errorf("r = %q %v", r.Data, r.Overwritten)
	}
	// Two keys revived tell the remover once.
	if want := []Notice{{Kind: Revived, Path: model.StoreMetaPath, Author: "a"}}; !slices.Equal(r.Notices, want) {
		t.Errorf("notices = %v", r.Notices)
	}
}

func TestYAMLOursWins(t *testing.T) {
	r := mergeAt(t, schema.Path, nil, OursWins, Side{Data: []byte("a: 1\nb: 1\n")}, Side{Data: []byte("a: 2\nb: 1\n"), Author: "a"}, Side{Data: []byte("a: 3\nb: 2\n"), Author: "b", When: whenAt(time.Unix(9, 0))})
	if string(r.Data) != "a: 2\nb: 2\n" || len(r.Overwritten) != 0 {
		t.Errorf("r = %q %v", r.Data, r.Overwritten)
	}
}

func TestBrokenYAMLFallsBackToOtherFile(t *testing.T) {
	r := mergeSym(t, "schema.yaml", Side{Data: []byte("a: 1\n")}, Side{Data: []byte("a: [\n"), Author: "a", Time: time.Unix(1, 0)}, Side{Data: []byte("a: 2\n"), Author: "b", Time: time.Unix(2, 0)})
	if string(r.Data) != "a: 2\n" || len(r.Overwritten) != 1 || r.Overwritten[0].Field != "*" {
		t.Errorf("r = %q %v", r.Data, r.Overwritten)
	}
}

func TestNumbersTakeMax(t *testing.T) {
	r := mergeSym(t, model.NumbersPath, Side{Data: []byte("WEB: 3\n")}, Side{Data: []byte("WEB: 5\nAPI: 1\n")}, Side{Data: []byte("WEB: 4\nBE: 2\n")})
	n, err := model.DecodeNumbers(r.Data)
	if err != nil || n["WEB"] != 5 || n["API"] != 1 || n["BE"] != 2 || len(r.Overwritten) != 0 {
		t.Errorf("numbers = %v, %v, %v", n, err, r.Overwritten)
	}
}

func TestNumbersKeepComments(t *testing.T) {
	ours := []byte("# record\nWEB: 5 # web\nAPI: 1\n")
	theirs := []byte("# record\nWEB: 4 # web\nBE: 2\nAA: 1\n")
	r := mergeSym(t, model.NumbersPath, Side{Data: []byte("# record\nWEB: 3 # web\n")}, Side{Data: ours, Author: "b"}, Side{Data: theirs, Author: "a"})
	if want := "# record\nWEB: 5 # web\nAPI: 1\nAA: 1\nBE: 2\n"; string(r.Data) != want {
		t.Errorf("r =\n%s", r.Data)
	}
}

func TestOtherFileByTime(t *testing.T) {
	r := mergeSym(t, ".gitattributes", Side{Data: []byte("a\n")}, Side{Data: []byte("o\n"), Author: "a", Time: time.Unix(2, 0)}, Side{Data: []byte("t\n"), Author: "b", Time: time.Unix(1, 0)})
	if string(r.Data) != "o\n" || len(r.Overwritten) != 1 || r.Overwritten[0].Author != "b" {
		t.Errorf("r = %q %v", r.Data, r.Overwritten)
	}
}
