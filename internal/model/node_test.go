package model

import (
	"errors"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func node(t *testing.T, v any) *yaml.Node {
	t.Helper()
	var n yaml.Node
	if err := n.Encode(v); err != nil {
		t.Fatal(err)
	}
	return &n
}

func parse(t *testing.T, src string) *Doc {
	t.Helper()
	d, err := ParseDoc([]byte(src), 0)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func encode(t *testing.T, e *Encoder, extra Fields) string {
	t.Helper()
	out, err := e.Bytes(extra)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestPutKeepsUnchangedNode(t *testing.T) {
	d := parse(t, "rank: \"a0V\"\n")
	e := d.Encoder()
	e.Put("rank", "a0V", false)
	if got, want := encode(t, e, nil), "rank: \"a0V\"\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPutKeepsComments(t *testing.T) {
	d := parse(t, "# head\nkey: WEB-42 # note\n")
	e := d.Encoder()
	e.Put("key", "WEB-43", false)
	if got, want := encode(t, e, nil), "# head\nkey: WEB-43 # note\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPutZero(t *testing.T) {
	d := parse(t, "labels: [a]\n")
	e := d.Encoder()
	e.Put("labels", []string{}, true) // present: kept, written empty
	e.Put("repos", []string{}, true)  // absent and zero: left out
	e.Put("title", "x", false)        // absent: added
	if got, want := encode(t, e, nil), "labels: []\ntitle: x\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPutNilNodeWritesNull(t *testing.T) {
	d := parse(t, "gh: {}\n")
	e := d.Encoder()
	e.Put("gh", (*yaml.Node)(nil), false)
	if got, want := encode(t, e, nil), "gh: null\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPutStyles(t *testing.T) {
	type link struct {
		Type string `yaml:"type"`
		Ref  string `yaml:"ref"`
	}
	type deep struct {
		Where string              `yaml:"where"`
		W     map[string][]string `yaml:"w"`
	}
	e := NewDoc().Encoder()
	e.Put("labels", []string{"x", "y"}, false)
	e.Put("links", []link{{"t", "r"}}, false)
	e.Put("scoped", []deep{{"p", map[string][]string{"a": {"b"}}}}, false)
	got := encode(t, e, nil)
	if want := "labels: [x, 'y']\nlinks:\n- {type: t, ref: r}\nscoped:\n"; !strings.HasPrefix(got, want) {
		t.Errorf("got %q, want it to start with %q", got, want)
	}
	// A mapping with a nested collection inside a list stays in block style.
	if !strings.Contains(got, "\n- where: p\n") {
		t.Errorf("deep item is not in block style: %q", got)
	}
}

func TestPutKeepsBlockStyle(t *testing.T) {
	d := parse(t, "labels:\n- a\n")
	e := d.Encoder()
	e.Put("labels", []string{"a", "b"}, false)
	if got, want := encode(t, e, nil), "labels:\n- a\n- b\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBytesOrdersKeys(t *testing.T) {
	d := parse(t, "b: 1\nz: 9 # unknown\na: 2\n")
	e := d.Encoder()
	e.Put("a", 2, false)
	e.Put("b", 1, false)
	if got, want := encode(t, e, d.Extra([]string{"a", "b"})), "a: 2\nb: 1\nz: 9 # unknown\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDocDecode(t *testing.T) {
	d, err := ParseDoc([]byte("title: 2026-11-03\nlabels: [a, 2026-01-01]\nn: abc\nnone: null\n"), 1)
	if err != nil {
		t.Fatal(err)
	}
	var title string
	var labels []string
	if err := d.Decode("title", &title); err != nil || title != "2026-11-03" {
		t.Errorf("title = %q, %v", title, err)
	}
	if err := d.Decode("labels", &labels); err != nil || len(labels) != 2 || labels[1] != "2026-01-01" {
		t.Errorf("labels = %q, %v", labels, err)
	}
	none := "kept"
	if err := d.Decode("none", &none); err != nil || none != "kept" {
		t.Errorf("null changed the value: %q, %v", none, err)
	}
	if err := d.Decode("missing", &none); err != nil || none != "kept" {
		t.Errorf("absent key changed the value: %q, %v", none, err)
	}
	var n int
	err = d.Decode("n", &n)
	var de *DecodeError
	if !errors.As(err, &de) || de.Field != "n" || de.Line != 4 {
		t.Errorf("Decode(n) error = %#v, want a DecodeError at line 4", err)
	}
}

func TestParseDocRejectsNonMapping(t *testing.T) {
	if _, err := ParseDoc([]byte("- a\n"), 0); err == nil {
		t.Error("ParseDoc accepted a list")
	}
	d, err := ParseDoc([]byte("  \n"), 0)
	if err != nil || d.Get("x") != nil {
		t.Errorf("ParseDoc(blank) = %v, %v", d, err)
	}
}

func TestFields(t *testing.T) {
	var f Fields
	f.Set("a", node(t, 1))
	f.Set("b", node(t, 2))
	f.Set("a", node(t, 3))
	if len(f) != 2 || f.Get("a").Value != "3" || f.Get("x") != nil {
		t.Errorf("Fields = %v", f)
	}
	f.Delete("a")
	if len(f) != 1 || f[0].Key != "b" {
		t.Errorf("after Delete: %v", f)
	}
}

func TestNodeEqual(t *testing.T) {
	d := parse(t, "a: \"x\"\nb: [1, 2]\nc: {k: v}\n")
	for _, tc := range []struct {
		key  string
		v    any
		want bool
	}{
		{"a", "x", true},
		{"a", "y", false},
		{"b", []int{1, 2}, true},
		{"b", []int{2, 1}, false},
		{"b", []string{"1", "2"}, false}, // !!str is not !!int
		{"c", map[string]string{"k": "v"}, true},
	} {
		if got := NodeEqual(d.Get(tc.key), node(t, tc.v)); got != tc.want {
			t.Errorf("NodeEqual(%s, %v) = %v, want %v", tc.key, tc.v, got, tc.want)
		}
	}
	if !NodeEqual(nil, nil) || NodeEqual(nil, node(t, 1)) {
		t.Error("NodeEqual mishandles nil")
	}
}

type pair struct {
	Type string `yaml:"type"`
	Ref  string `yaml:"ref"`
}

type named struct {
	Name  string `yaml:"name"`
	Label string `yaml:"label,omitempty"`
}

func TestPutMergesChangedItemByName(t *testing.T) {
	src := "items:\n# head\n- name: a # first\n  x: keep\n  label: A\n- {name: b, x: keep} # second\n"
	e := parse(t, src).Encoder()
	e.Put("items", []named{{"b", "B"}, {"a", "AA"}}, false)
	want := "items:\n- {name: b, label: B, x: keep} # second\n# head\n- name: a # first\n  x: keep\n  label: AA\n"
	if got := encode(t, e, nil); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPutMergesRenamedItemByPosition(t *testing.T) {
	src := "items:\n- name: a # first\n  x: keep\n- {name: b, x: keep} # second\n"
	e := parse(t, src).Encoder()
	e.Put("items", []named{{Name: "z"}, {Name: "b"}}, false)
	want := "items:\n- name: z # first\n  x: keep\n- {name: b, x: keep} # second\n"
	if got := encode(t, e, nil); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPutMergesMapValues(t *testing.T) {
	src := "m:\n  # head\n  a: {name: a, x: keep} # first\n  b: {name: b} # second\n"
	e := parse(t, src).Encoder()
	e.Put("m", map[string]named{"a": {"a", "A"}, "b": {Name: "b"}}, false)
	want := "m:\n  # head\n  a: {name: a, label: A, x: keep} # first\n  b: {name: b} # second\n"
	if got := encode(t, e, nil); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPutMergesRenamedMapKeyByPosition(t *testing.T) {
	src := "m:\n  # head\n  a: {name: a, x: keep} # first\n  b: {name: b} # second\n"
	e := parse(t, src).Encoder()
	e.Put("m", map[string]named{"z": {Name: "a"}, "b": {Name: "b"}}, false)
	want := "m:\n  # head\n  z: {name: a, x: keep} # first\n  b: {name: b} # second\n"
	if got := encode(t, e, nil); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTaskLinkChangeKeepsOtherLinks(t *testing.T) {
	src := "---\nid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nlinks:\n- type: github_pr # PR\n  ref: r1\n  sync: \"\"\n  x: keep\n- {type: github_issue, ref: r2, x: keep} # issue\n---\n"
	task, err := DecodeTask([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	task.Links[1].Ref = "r3"
	out, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(src, "ref: r2", "ref: r3", 1)
	if string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestTaskPropChangeKeepsOtherComments(t *testing.T) {
	task, err := DecodeTask(readFile(t, "testdata/task/full.md"))
	if err != nil {
		t.Fatal(err)
	}
	task.Props.Set("points", node(t, 8))
	out, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"  priority: high # 優先度\n", "  points: 8\n", "custom: kept # 未知のキー\n"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestPutKeepsBlockMappingsInList(t *testing.T) {
	d := parse(t, "links:\n- type: x\n  ref: r\n")
	e := d.Encoder()
	e.Put("links", []pair{{"x", "r"}, {"a", "b"}}, false)
	want := "links:\n- type: x\n  ref: r\n- type: a\n  ref: b\n"
	if got := encode(t, e, nil); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPutAppendsFlowMappingToFlowList(t *testing.T) {
	d := parse(t, "links:\n- {type: x, ref: r}\n")
	e := d.Encoder()
	e.Put("links", []pair{{"x", "r"}, {"a", "b"}}, false)
	want := "links:\n- {type: x, ref: r}\n- {type: a, ref: b}\n"
	if got := encode(t, e, nil); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseDocCommentOnly(t *testing.T) {
	d, err := ParseDoc([]byte("# just a comment\n"), 5)
	if err != nil || d.Get("x") != nil || d.line != 5 {
		t.Errorf("ParseDoc(comment) = %+v, %v", d, err)
	}
}

func TestParseDocErrorLines(t *testing.T) {
	_, err := ParseDoc([]byte("\n- a\n"), 3)
	var de *DecodeError
	if !errors.As(err, &de) || de.Line != 5 {
		t.Errorf("shape error = %#v, want DecodeError at line 5", err)
	}
	_, err = ParseDoc([]byte("a: [\n"), 3)
	if !errors.As(err, &de) || de.Field != "frontmatter" || de.Line != 4 || !strings.Contains(err.Error(), "did not find expected") {
		t.Errorf("syntax error = %#v (%v), want DecodeError frontmatter at line 4 with the parser's message", err, err)
	}
}

func TestDecodeErrorString(t *testing.T) {
	inner := errors.New("bad")
	for _, tc := range []struct {
		e    DecodeError
		want string
	}{
		{DecodeError{Field: "a", Line: 3, Err: inner}, "a (line 3): bad"},
		{DecodeError{Field: "a", Err: inner}, "a: bad"},
		{DecodeError{Line: 3, Err: inner}, "line 3: bad"},
		{DecodeError{Err: inner}, "bad"},
	} {
		if got := tc.e.Error(); got != tc.want {
			t.Errorf("Error() = %q, want %q", got, tc.want)
		}
	}
}

func TestParseDocRejects(t *testing.T) {
	for name, src := range map[string]string{
		"anchor":    "a: &x 1\n",
		"alias":     "a: &x 1\nb: *x\n",
		"merge":     "m:\n  <<: {k: v}\n",
		"dup":       "a: 1\nb: 2\na: 3\n",
		"dupnested": "x:\n  k: 1\n  k: 2\n",
	} {
		if _, err := ParseDoc([]byte(src), 0); err == nil {
			t.Errorf("%s: ParseDoc accepted %q", name, src)
		}
	}
	_, err := ParseDoc([]byte("a: 1\nb: 2\na: 3\n"), 10)
	var de *DecodeError
	if !errors.As(err, &de) || de.Field != "a" || de.Line != 13 {
		t.Errorf("dup error = %#v, want DecodeError a at line 13", err)
	}
}

func TestParseDocRejectsTags(t *testing.T) {
	for name, src := range map[string]string{
		"binary": "a: 1\ntitle: !!binary aGk=\n",
		"str":    "a: 1\nx: !!str 12\n",
		"custom": "a: 1\nx: !custom v\n",
		"root":   "!!map {a: 1}\n",
	} {
		_, err := ParseDoc([]byte(src), 10)
		var de *DecodeError
		if !errors.As(err, &de) || !strings.Contains(err.Error(), "YAML tags are not supported") {
			t.Errorf("%s: err = %v, want tag DecodeError", name, err)
			continue
		}
		want := 12
		if name == "root" {
			want = 11
		}
		if de.Line != want {
			t.Errorf("%s: Line = %d, want %d", name, de.Line, want)
		}
	}
	for _, src := range []string{"x: 12\n", "x: \"12\"\n"} {
		if _, err := ParseDoc([]byte(src), 0); err != nil {
			t.Errorf("ParseDoc(%q) = %v", src, err)
		}
	}
}

func TestNilFieldValueWritesNull(t *testing.T) {
	if _, err := yaml.Dump(Fields{{"a", nil}}); err != nil {
		t.Fatalf("Dump(Fields with nil) = %v", err)
	}
	e := NewDoc().Encoder()
	e.Put("k", "v", false)
	if got, want := encode(t, e, Fields{{"x", nil}}), "k: v\nx: null\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBytesRejectsDuplicateKeys(t *testing.T) {
	for name, extra := range map[string]Fields{
		"extra repeats a put key": {{"k", node(t, 1)}},
		"extra has a key twice":   {{"x", node(t, 1)}, {"x", node(t, 2)}},
	} {
		e := NewDoc().Encoder()
		e.Put("k", "v", false)
		if out, err := e.Bytes(extra); err == nil {
			t.Errorf("%s: Bytes = %q, want an error", name, out)
		}
	}
}
