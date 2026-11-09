package schema

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

func TestRelationsTable(t *testing.T) {
	t.Parallel()
	want := [][4]string{
		{"blocked_by", "Blocked by", "blocks", "Blocks"},
		{"causes", "Causes", "caused_by", "Caused by"},
		{"relates", "Relates to", "related_by", "Related by"},
		{"duplicates", "Duplicates", "duplicated_by", "Duplicated by"},
		{"clones", "Clones", "cloned_by", "Cloned by"},
	}
	if len(Relations) != len(want) {
		t.Fatalf("%d relations", len(Relations))
	}
	u := model.NewULID()
	for i, r := range Relations {
		if got := [4]string{r.Name, r.Label, r.Inverse, r.InverseLabel}; got != want[i] {
			t.Errorf("Relations[%d] = %v, want %v", i, got, want[i])
		}
		var task model.Task
		r.Set(&task, []model.ULID{u})
		if got := r.Get(&task); len(got) != 1 || got[0] != u {
			t.Errorf("%s: Get after Set = %v", r.Name, got)
		}
		// The value lands in the field of that name and in no other.
		n := 0
		for _, o := range Relations {
			n += len(o.Get(&task))
		}
		if n != 1 {
			t.Errorf("%s: %d values across the relations, want 1", r.Name, n)
		}
	}
}

func TestRelationOf(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, want string
		inverse    bool
		ok         bool
	}{
		{"blocked_by", "blocked_by", false, true},
		{"blocks", "blocked_by", true, true},
		{"causes", "causes", false, true},
		{"caused_by", "causes", true, true},
		{"relates", "relates", false, true},
		{"related_by", "relates", true, true},
		{"duplicates", "duplicates", false, true},
		{"duplicated_by", "duplicates", true, true},
		{"clones", "clones", false, true},
		{"cloned_by", "clones", true, true},
		{"priority", "", false, false},
		{"", "", false, false},
	} {
		r, inverse, ok := RelationOf(tc.name)
		if ok != tc.ok || inverse != tc.inverse || r.Name != tc.want {
			t.Errorf("RelationOf(%q) = %q, %v, %v", tc.name, r.Name, inverse, ok)
		}
	}
}

func TestTaskRefTypeIsRefusedWithGuidance(t *testing.T) {
	t.Parallel()
	s := Default()
	s.Props = append(s.Props, Prop{Name: "ref", Type: "task_ref"})
	for _, p := range s.Check() {
		if p.Code == "schema.unknown_type" && p.Path == "props.ref" {
			if p.Msg != "関連は固定です。hh dep か詳細の関連の欄で付けます" {
				t.Errorf("msg = %q", p.Msg)
			}
			return
		}
	}
	t.Errorf("no schema.unknown_type: %v", s.Check())
}

func TestRelationsAreRequirable(t *testing.T) {
	t.Parallel()
	s := Default()
	for _, r := range Relations {
		s.Workflow.Required = Lists{{Key: "done", Values: []string{r.Name}}}
		if ps := s.Check(); len(ps) != 0 {
			t.Errorf("%s: %v", r.Name, ps)
		}
		var task model.Task
		if filled, ok := builtinFilled(&task, r.Name); !ok || filled {
			t.Errorf("%s empty: filled=%v ok=%v", r.Name, filled, ok)
		}
		r.Set(&task, []model.ULID{model.NewULID()})
		if filled, ok := builtinFilled(&task, r.Name); !ok || !filled {
			t.Errorf("%s set: filled=%v ok=%v", r.Name, filled, ok)
		}
	}
	// The other direction is not a field a task has.
	if _, ok := builtinFilled(&model.Task{}, "blocks"); ok {
		t.Error("blocks is requirable")
	}
}
