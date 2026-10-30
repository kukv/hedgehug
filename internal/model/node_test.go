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
