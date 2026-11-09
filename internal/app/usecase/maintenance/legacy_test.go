package maintenance_test

import (
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

func legacyFindings(fs []usecase.Finding) map[string]usecase.Finding {
	out := map[string]usecase.Finding{}
	for _, f := range fs {
		if f.Code == "task.legacy_field" {
			out[f.Task+" "+f.Args[0]] = f
		}
	}
	return out
}

// An older hedgehug that pushes writes a relation in props: doctor tells of it
// (task.legacy_field), and --fix moves it to the field of the task, except where
// the field has values already.
func TestDoctorTellsOfARelationInPropsAndFixMovesIt(t *testing.T) {
	ts := newCheckedSession(t)
	props := func(src string) func(*model.Task) {
		return func(tk *model.Task) {
			if err := yaml.Unmarshal([]byte(src), &tk.Props); err != nil {
				t.Fatal(err)
			}
		}
	}
	b := ts.AddTask("b", nil)
	a := ts.AddTask("a", props("relates: ["+b.ULID.String()+"]\nduplicates: "+b.ULID.String()+"\n"))
	c := ts.AddTask("c", func(tk *model.Task) {
		props("causes: [" + a.ULID.String() + "]\n")(tk)
		tk.Causes = []model.ULID{b.ULID}
	})
	d := ts.AddTask("d", props("clones: [not-a-ulid]\n"))
	before := ts.TaskByULID("me", a.ULID).Updated

	rep, err := ts.DoctorOne(false)
	if err != nil {
		t.Fatal(err)
	}
	got := legacyFindings(rep.Findings)
	if len(got) != 4 {
		t.Fatalf("findings = %+v; want a (relates, duplicates), c, d", rep.Findings)
	}
	if f := got[c.ULID.String()+" props.causes"]; !strings.Contains(f.Hint, "has values of its own") {
		t.Errorf("c: %+v", f)
	}
	if f := got[d.ULID.String()+" props.clones"]; !strings.Contains(f.Hint, "not the ULID") {
		t.Errorf("d: %+v", f)
	}
	if !usecase.FindingsFail(rep.Findings) {
		t.Error("a relation in props does not fail doctor")
	}

	rep, err = ts.DoctorOne(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Repaired) != 1 || rep.Repaired[0].Kind != "legacy_field" || rep.Repaired[0].ULID != a.ULID.String() ||
		!strings.Contains(rep.Repaired[0].Line, "props.relates, props.duplicates") {
		t.Fatalf("repaired = %+v", rep.Repaired)
	}
	na := ts.TaskByULID("me", a.ULID)
	if !slices.Equal(na.Relates, []model.ULID{b.ULID}) || !slices.Equal(na.Duplicates, []model.ULID{b.ULID}) || len(na.Props) != 0 {
		t.Errorf("a: relates %v, duplicates %v, props %v", na.Relates, na.Duplicates, na.Props)
	}
	if !na.Updated.Equal(before) {
		t.Errorf("updated moved: %v to %v", before, na.Updated)
	}
	if nc := ts.TaskByULID("me", c.ULID); nc.Props.Get("causes") == nil || !slices.Equal(nc.Causes, []model.ULID{b.ULID}) {
		t.Errorf("c was changed: causes %v, props %v", nc.Causes, nc.Props)
	}
	if nd := ts.TaskByULID("me", d.ULID); nd.Props.Get("clones") == nil {
		t.Errorf("d was changed: props %v", nd.Props)
	}
	got = legacyFindings(rep.Findings)
	if len(got) != 2 || got[c.ULID.String()+" props.causes"].Task == "" || got[d.ULID.String()+" props.clones"].Task == "" {
		t.Errorf("findings left = %+v; want c and d", rep.Findings)
	}
	// the second --fix has nothing to move
	if rep, err = ts.DoctorOne(true); err != nil || len(rep.Repaired) != 0 {
		t.Errorf("second fix = %+v, %v", rep.Repaired, err)
	}
}
