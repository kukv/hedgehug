package workspace_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	tplA  change.ID = "01TPL0000000000000000000A0"
	tplB  change.ID = "01TPL0000000000000000000B0"
	child change.ID = "01CHILD00000000000000000C1"
	stepR change.ID = "01STEP000000000000000000R1"
	stepC change.ID = "01STEP000000000000000000C1"
	stepX change.ID = "01STEP000000000000000000X1"
)

func set(f string, v any) change.Draft { return change.NewSet(f, nil, v) }

func tf(id change.ID, rest ...string) string { return workspace.TemplateField(id, rest...) }

// Epic のテンプレート A(子 1 つ、子の Step 1 つ)と、Task のテンプレート B(Step 1 つ)を読む。
// 消した Step と、「なし」に戻した既定は出ない。
func TestTemplatesFromLog(t *testing.T) {
	st := workflowSettings(t, func(s workspace.Settings) []change.Draft {
		c, sc, sr, sx := string(child), string(stepC), string(stepR), string(stepX)
		return []change.Draft{
			set(tf(tplA, "name"), "リリース作業"), set(tf(tplA, "kind"), "Epic"), set(tf(tplA, "title"), "リリース "),
			set(tf(tplA, "values", "priority"), s.Priorities[1].ID),
			set(tf(tplA, "children", c, "kind"), "Story"), set(tf(tplA, "children", c, "title"), "QA"),
			set(tf(tplA, "children", c, "rank"), "V"), set(tf(tplA, "children", c, "values", "assignee"), "hanako"),
			set(tf(tplA, "steps", sc, "parent"), c), set(tf(tplA, "steps", sc, "title"), "確認"),
			set(tf(tplA, "steps", sc, "assignee"), workspace.StepParent), set(tf(tplA, "steps", sc, "rank"), "V"),
			set(tf(tplB, "name"), "調査"), set(tf(tplB, "kind"), "Task"),
			set(tf(tplB, "steps", sr, "parent"), ""), set(tf(tplB, "steps", sr, "title"), "再現"),
			set(tf(tplB, "steps", sr, "assignee"), workspace.StepNone), set(tf(tplB, "steps", sr, "rank"), "V"),
			set(tf(tplB, "steps", sx, "parent"), ""), set(tf(tplB, "steps", sx, "title"), "消す"),
			set(tf(tplB, "steps", sx, "rank"), "W"), change.NewDelete(tf(tplB, "steps", sx)),
			set("template_default/Epic", tplA), set("template_default/Task", tplB),
			change.NewSet("template_default/Task", tplB, nil),
		}
	})
	if len(st.Templates) != 2 || st.Templates[0].ID != tplA || st.Templates[1].ID != tplB {
		t.Fatalf("templates = %+v", st.Templates)
	}
	a := st.Templates[0]
	if a.Name != "リリース作業" || a.Kind != "Epic" || a.Title != "リリース " || len(a.Children) != 1 {
		t.Errorf("A = %+v", a)
	}
	if string(a.Values["priority"]) != `"`+string(st.Priorities[1].ID)+`"` {
		t.Errorf("A priority = %s", a.Values["priority"])
	}
	c := a.Children[0]
	if c.ID != child || c.Kind != "Story" || c.Title != "QA" || string(c.Values["assignee"]) != `"hanako"` {
		t.Errorf("child = %+v", c)
	}
	if len(c.Steps) != 1 || c.Steps[0].Title != "確認" || c.Steps[0].Assignee != workspace.StepParent {
		t.Errorf("child steps = %+v", c.Steps)
	}
	b := st.Templates[1]
	if len(b.Steps) != 1 || b.Steps[0].ID != stepR || b.Steps[0].Assignee != workspace.StepNone {
		t.Errorf("B steps = %+v", b.Steps)
	}
	if st.TemplateDefaults["Epic"] != tplA || st.TemplateDefaults["Task"] != "" {
		t.Errorf("defaults = %v", st.TemplateDefaults)
	}
}

// テンプレートを消すと出ず、その既定も出ない。子を消すと、その子の Step も出ない。null の値は「指定しない」。
func TestTemplatesDeleted(t *testing.T) {
	st := workflowSettings(t, func(workspace.Settings) []change.Draft {
		c, sc := string(child), string(stepC)
		return []change.Draft{
			set(tf(tplA, "name"), "A"), set(tf(tplA, "kind"), "Epic"),
			set(tf(tplA, "values", "systems"), []string{"x"}),
			change.NewSet(tf(tplA, "values", "systems"), []string{"x"}, nil),
			set(tf(tplA, "children", c, "kind"), "Task"), set(tf(tplA, "children", c, "title"), "子"),
			set(tf(tplA, "steps", sc, "parent"), c), set(tf(tplA, "steps", sc, "title"), "Step"),
			change.NewDelete(tf(tplA, "children", c)),
			set(tf(tplB, "name"), "B"), set(tf(tplB, "kind"), "Task"),
			set("template_default/Task", tplB), change.NewDelete(tf(tplB)),
		}
	})
	if len(st.Templates) != 1 || len(st.Templates[0].Children) != 0 || len(st.Templates[0].Values) != 0 {
		t.Fatalf("templates = %+v", st.Templates)
	}
	if _, ok := st.Template(tplB); ok || st.TemplateDefaults["Task"] != "" {
		t.Errorf("deleted template B is still visible: %v", st.TemplateDefaults)
	}
}

func TestHoldsTemplateValue(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	var got []string
	for _, p := range s.Properties {
		if workspace.HoldsTemplateValue(p) {
			got = append(got, p.Key)
		}
	}
	slices.Sort(got)
	if strings.Join(got, ",") != "assignee,priority,systems" {
		t.Errorf("holds = %v", got)
	}
	if !workspace.HoldsTemplateValue(workspace.Property{Key: "customer", Type: workspace.TypeText}) {
		t.Error("added properties are held")
	}
}
