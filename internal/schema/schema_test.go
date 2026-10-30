package schema

import (
	"os"
	"strings"
	"testing"
)

func readFull(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/full.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSchemaRoundTrip(t *testing.T) {
	src := readFull(t)
	s, err := Decode(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Statuses) != 7 || s.Statuses[3].Color != "#3451D1" || len(s.Kinds) != 5 ||
		s.Kinds[4].Template != "bug-report" || len(s.Props) != 6 || s.Props[0].Name != "priority" ||
		!s.Props[5].Symmetric || len(s.Scoped) != 2 || s.Scoped[0].Templates.Default["task"] != "release-checklist" {
		t.Errorf("decoded = %+v", s)
	}
	if got, ok := s.Workflow.Transitions.Get("backlog"); !ok || strings.Join(got, ",") != "todo,canceled" {
		t.Errorf("transitions[backlog] = %v, %v", got, ok)
	}
	out, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(src) {
		t.Errorf("round trip changed the file:\n%s", out)
	}
}

func TestSchemaEditKeepsComments(t *testing.T) {
	s, err := Decode(readFull(t))
	if err != nil {
		t.Fatal(err)
	}
	s.Statuses = append(s.Statuses, Status{Name: "review", Label: "レビュー", Category: CategoryDoing})
	out, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# kinds は「種類と階層」",
		"- {name: review, label: レビュー, category: doing}\n",
		"# リリース準備 v2.4",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestSchemaKeepsUnknownKeys(t *testing.T) {
	src, err := os.ReadFile("testdata/extras.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Decode(src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.Encode()
	if err != nil || string(out) != string(src) {
		t.Fatalf("round trip changed the file (%v):\n%s", err, out)
	}
	s.Statuses = append(s.Statuses, Status{Name: "review", Category: CategoryDoing})
	s.Kinds = append(s.Kinds, Kind{Name: "bug", Role: RoleWork})
	s.Scoped = append(s.Scoped, Scoped{Where: "project:01JBZ8K3QW7M2X9V4T6R5N8P0C"})
	out, err = s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"- {name: backlog, category: todo, default: false, hint: 未知のキー}\n",
		"# 既定の未着手\n- name: todo\n  label: 未着手 # ラベル\n  category: todo\n  default: true\n  sla: 3d\n",
		"- {name: canceled, category: canceled, default: true}\n- {name: review, category: doing}\n",
		"- {name: project, role: container, default: false, color: blue}\n",
		"  role: work # 作業\n  default: true\n  wip_limit: 5\n",
		"  owner: github.com/yourname # 未知のキー\n",
		"    default:\n      task: release-checklist\n      project: kickoff\n",
		"  priority: {type: enum, values: [low, high], order: desc} # 未知のキー\n",
		"  note: 未知のキー\n",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestDefaultEncodesAndDecodes(t *testing.T) {
	for name, s := range map[string]*Schema{"Default": Default(), "Dev": Dev()} {
		out, err := s.Encode()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		back, err := Decode(out)
		if err != nil {
			t.Fatalf("%s: %v\n%s", name, err, out)
		}
		if len(back.Statuses) != len(s.Statuses) || len(back.Kinds) != 5 || len(back.Props) != 8 {
			t.Errorf("%s: re-decoded = %+v", name, back)
		}
		if !strings.Contains(string(out), "- {name: todo, label: 未着手, category: todo, default: true}\n") {
			t.Errorf("%s: statuses are not in flow style:\n%s", name, out)
		}
	}
	if len(Default().Statuses) != 7 || len(Dev().Statuses) != 8 {
		t.Error("Default must have 7 statuses and Dev 8")
	}
	if st, ok := Dev().Status("review"); !ok || st.Category != CategoryDoing {
		t.Errorf("Dev review = %+v, %v", st, ok)
	}
	if k, ok := Default().Kind("project"); !ok || k.Parents != nil {
		t.Errorf("project kind = %+v: parents must be omitted (anywhere)", k)
	}
}

func TestLookups(t *testing.T) {
	s := Default()
	if st, ok := s.DefaultStatus(CategoryDoing); !ok || st.Name != "in_progress" {
		t.Errorf("DefaultStatus(doing) = %+v, %v", st, ok)
	}
	if k, ok := s.DefaultKind(); !ok || k.Name != "task" {
		t.Errorf("DefaultKind = %+v, %v", k, ok)
	}
	if p, ok := s.Prop("estimate"); !ok || p.Type != TypeDuration {
		t.Errorf("Prop(estimate) = %+v, %v", p, ok)
	}
	if _, ok := s.Status("nope"); ok {
		t.Error("Status(nope) found")
	}
}

func TestDecodeErrors(t *testing.T) {
	for _, src := range []string{
		"statuses: {a: b}\n",
		"props: [a]\n",
		"workflow:\n  transitions: [a]\n",
	} {
		if _, err := Decode([]byte(src)); err == nil {
			t.Errorf("Decode(%q) succeeded", src)
		}
	}
}
