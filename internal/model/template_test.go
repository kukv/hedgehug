package model

import (
	"strings"
	"testing"
)

func TestTemplateRoundTrip(t *testing.T) {
	src := readFile(t, "testdata/template/release-checklist.md")
	tp, err := DecodeTemplate("release-checklist", src)
	if err != nil {
		t.Fatal(err)
	}
	if tp.Name != "release-checklist" || tp.Item.Kind != "task" || len(tp.Item.Children) != 3 ||
		!tp.Item.Children[0].Assignees.Inherit || tp.Item.Children[1].Props.Get("points").Value != "2" ||
		!tp.Item.Children[2].Assignees.IsZero() || !strings.HasPrefix(tp.Item.Body, "## 手順") {
		t.Errorf("decoded = %+v", tp)
	}
	out, err := tp.Encode()
	if err != nil || string(out) != string(src) {
		t.Errorf("round trip = %q, %v", out, err)
	}
}

func TestNewTemplateEncode(t *testing.T) {
	var points Fields
	points.Set("points", node(t, 2))
	tp := &Template{Name: "x", Item: TemplateItem{Kind: "task", Children: []TemplateItem{
		{Title: "a", Assignees: Assignees{Inherit: true}},
		{Title: "b", Props: points, Children: []TemplateItem{{Title: "c"}}},
	}}}
	out, err := tp.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !contains(out, "- {title: a, assignees: inherit}\n") {
		t.Errorf("short child is not in flow style:\n%s", out)
	}
	back, err := DecodeTemplate("x", out)
	if err != nil {
		t.Fatal(err)
	}
	b := back.Item.Children[1]
	if b.Title != "b" || b.Props.Get("points").Value != "2" || len(b.Children) != 1 || b.Children[0].Title != "c" {
		t.Errorf("re-decoded = %+v", back.Item)
	}
}

func TestAssigneesYAML(t *testing.T) {
	tp, err := DecodeTemplate("x", []byte("---\nassignees: [github.com/a]\n---\n"))
	if err != nil || tp.Item.Assignees.Inherit || len(tp.Item.Assignees.Logins) != 1 {
		t.Errorf("decoded = %+v, %v", tp, err)
	}
	if _, err := DecodeTemplate("x", []byte("---\nassignees: {a: b}\n---\n")); err == nil {
		t.Error("DecodeTemplate accepted a mapping for assignees")
	}
}
