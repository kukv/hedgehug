package maintenance_test

import (
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/maintenance"
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
	// e: the conflict of causes keeps the whole task, duplicates included
	e := ts.AddTask("e", func(tk *model.Task) {
		props("causes: [" + a.ULID.String() + "]\nduplicates: [" + a.ULID.String() + "]\n")(tk)
		tk.Causes = []model.ULID{b.ULID}
	})
	// g: two names with a value that is no ULID are both told
	g := ts.AddTask("g", props("clones: [x]\ncauses: [y]\n"))
	// f: an empty value is no bad value; --fix takes the key away
	f := ts.AddTask("f", props("relates: []\n"))
	before := ts.TaskByULID("me", a.ULID).Updated

	rep, err := ts.DoctorOne(false)
	if err != nil {
		t.Fatal(err)
	}
	got := legacyFindings(rep.Findings)
	if len(got) != 9 {
		t.Fatalf("findings = %+v; want a (relates, duplicates), c, d, e (causes, duplicates), f", rep.Findings)
	}
	if f := got[c.ULID.String()+" props.causes"]; !strings.Contains(f.Hint, "leaves this task as it is, because the field causes has values of its own") {
		t.Errorf("c: %+v", f)
	}
	if f := got[g.ULID.String()+" props.causes"]; !strings.Contains(f.Hint, "because props.clones, props.causes has a value") && !strings.Contains(f.Hint, "because props.causes, props.clones has a value") {
		t.Errorf("g: %+v", f)
	}
	if f := got[d.ULID.String()+" props.clones"]; !strings.Contains(f.Hint, "leaves this task as it is, because props.clones has a value that is not the ULID") {
		t.Errorf("d: %+v", f)
	}
	if f := got[e.ULID.String()+" props.duplicates"]; !strings.Contains(f.Hint, "leaves this task as it is, because the field causes has values of its own") {
		t.Errorf("e duplicates: %+v", f)
	}
	for _, k := range []string{a.ULID.String() + " props.relates", a.ULID.String() + " props.duplicates", f.ULID.String() + " props.relates"} {
		if !strings.Contains(got[k].Hint, "--fix moves it") {
			t.Errorf("%s: %+v", k, got[k])
		}
	}
	if !usecase.FindingsFail(rep.Findings) {
		t.Error("a relation in props does not fail doctor")
	}

	rep, err = ts.DoctorOne(true)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(rep.Repaired, func(x maintenance.Fixed) bool { return x.ULID == a.ULID.String() })
	if len(rep.Repaired) != 2 || i < 0 || rep.Repaired[i].Kind != "legacy_field" ||
		!strings.Contains(rep.Repaired[i].Line, "props.relates, props.duplicates") ||
		!slices.ContainsFunc(rep.Repaired, func(x maintenance.Fixed) bool { return x.ULID == f.ULID.String() }) {
		t.Fatalf("repaired = %+v; want a and f", rep.Repaired)
	}
	if nf := ts.TaskByULID("me", f.ULID); len(nf.Props) != 0 {
		t.Errorf("f: props %v", nf.Props)
	}
	if ne := ts.TaskByULID("me", e.ULID); ne.Props.Get("duplicates") == nil {
		t.Errorf("e was changed: props %v", ne.Props)
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
	if len(got) != 6 || got[c.ULID.String()+" props.causes"].Task == "" || got[d.ULID.String()+" props.clones"].Task == "" {
		t.Errorf("findings left = %+v; want c, d and e", rep.Findings)
	}
	// the second --fix has nothing to move
	if rep, err = ts.DoctorOne(true); err != nil || len(rep.Repaired) != 0 {
		t.Errorf("second fix = %+v, %v", rep.Repaired, err)
	}
}
