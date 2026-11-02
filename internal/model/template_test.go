package model

import (
	"reflect"
	"strings"
	"testing"
)

func TestTemplateRoundTrip(t *testing.T) {
	t.Parallel()
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

func TestTemplateChildrenKeepUnknownKeys(t *testing.T) {
	t.Parallel()
	src := readFile(t, "testdata/template/children.md")
	tp, err := DecodeTemplate("children", src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := tp.Encode()
	if err != nil || string(out) != string(src) {
		t.Fatalf("round trip changed the file (%v):\n%s", err, out)
	}
	tp.Item.Children = append(tp.Item.Children, TemplateItem{Title: "公開"})
	out, err = tp.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"- {title: 設計, assignees: inherit, owner: lead} # 未知のキー\n",
		"# ブロックの子\n- title: 実装\n  estimate_hint: 3d # 未知のキー\n",
		"  - {title: API, custom: 1}\n",
		"- {title: レビュー}\n- {title: 公開}\n---\n",
	} {
		if !contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

// Encoder.Put compares values with reflect.DeepEqual. Values holding Fields
// must compare equal when decoded twice from the same node.
func TestTemplateItemsDecodeEqualTwice(t *testing.T) {
	t.Parallel()
	d := parse(t, "children:\n- {title: a, props: {points: 2, area: [x]}}\n")
	var a, b []TemplateItem
	if err := d.Get("children").Decode(&a); err != nil {
		t.Fatal(err)
	}
	if err := d.Get("children").Decode(&b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("decoded twice: %+v != %+v", a, b)
	}
}

func TestNewTemplateEncode(t *testing.T) {
	t.Parallel()
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

func TestTemplateChildrenReadStringsRaw(t *testing.T) {
	t.Parallel()
	src := "---\nchildren: [{title: 2026-11-03, labels: [2026-11-05], body: 2026-11-06, children: [{title: 2026-11-04}]}]\n---\n"
	tp, err := DecodeTemplate("x", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	c := tp.Item.Children[0]
	if c.Title != "2026-11-03" || len(c.Labels) != 1 || c.Labels[0] != "2026-11-05" || c.Body != "2026-11-06" ||
		len(c.Children) != 1 || c.Children[0].Title != "2026-11-04" {
		t.Errorf("children = %+v", tp.Item.Children)
	}
	out, err := tp.Encode()
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeTemplate("x", out)
	if err != nil || back.Item.Children[0].Title != "2026-11-03" || back.Item.Children[0].Children[0].Title != "2026-11-04" {
		t.Errorf("re-decoded %q = %+v, %v", out, back, err)
	}
	if _, err := DecodeTemplate("x", []byte("---\nchildren: [{title: a, sprint: nope}]\n---\n")); err == nil {
		t.Error("DecodeTemplate accepted a bad sprint in a child")
	}
	if _, err := DecodeTemplate("x", []byte("---\nchildren: [a]\n---\n")); err == nil {
		t.Error("DecodeTemplate accepted a scalar child")
	}
	if _, err := DecodeTemplate("x", []byte("---\nchildren: [&a {title: x}]\n---\n")); err == nil {
		t.Error("DecodeTemplate accepted an anchor in a child")
	}
}

func TestDecodeTemplateRejectsBadName(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", "../x", "a/b"} {
		if _, err := DecodeTemplate(name, []byte("---\nkind: task\n---\n")); err == nil {
			t.Errorf("DecodeTemplate(%q) succeeded", name)
		}
	}
}

func TestAssigneesYAML(t *testing.T) {
	t.Parallel()
	tp, err := DecodeTemplate("x", []byte("---\nassignees: [github.com/a]\n---\n"))
	if err != nil || tp.Item.Assignees.Inherit || len(tp.Item.Assignees.Logins) != 1 {
		t.Errorf("decoded = %+v, %v", tp, err)
	}
	if _, err := DecodeTemplate("x", []byte("---\nassignees: {a: b}\n---\n")); err == nil {
		t.Error("DecodeTemplate accepted a mapping for assignees")
	}
}
