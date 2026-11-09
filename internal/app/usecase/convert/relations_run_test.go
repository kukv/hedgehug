package convert

import (
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// withRelationProps adds to the store of makeOld (format 1) the task_ref
// properties of format 2 and values in the props of the tasks: the story and the
// chore point at each other through relates, the chore also through fixes.
func withRelationProps(t *testing.T, ts *usecasetest.Session, o *oldTasks, extra map[*model.Task]string) {
	t.Helper()
	props := map[*model.Task]string{
		o.story: "relates: [" + o.sharp.ULID.String() + "]\n",
		o.sharp: "relates: [" + o.story.ULID.String() + "]\nfixes: " + o.root.ULID.String() + "\nduplicates: [" + o.story.ULID.String() + "]\n",
	}
	for tk, p := range extra {
		props[tk] = p
	}
	files := map[string]string{"schema.yaml": oldSchema + "props:\n  duplicates: {type: task_ref, multi: true}\n  relates: {type: task_ref, multi: true, symmetric: true}\n  fixes: {type: task_ref}\n"}
	for tk, p := range props {
		if err := yaml.Unmarshal([]byte(p), &tk.Props); err != nil {
			t.Fatal(err)
		}
		b, err := tk.Encode()
		if err != nil {
			t.Fatal(err)
		}
		files[model.TaskPath(tk.ULID)] = string(b)
	}
	commitRaw(t, ts, "relations in props", files)
	ts.DropMount()
}

func TestRunConvertsFromFormat1ToTheRelationsInOneRunAndNothingTheSecondTime(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	o := makeOld(t, ts)
	withRelationProps(t, ts, o, nil)
	reports, err := Run(ts.Session, Options{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 3 || reports[0].To != 2 || reports[1].From != 2 || reports[1].To != 3 {
		t.Fatalf("reports = %+v", reports)
	}
	if l := strings.Join(reports[1].Lines, "\n"); !strings.Contains(l, "fixes: 1 件を relates へ") || !strings.Contains(l, "相互の組 1 件") {
		t.Errorf("the report of step 2 to 3:\n%s", l)
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	sharp, _ := ix.Task(o.sharp.ULID)
	story, _ := ix.Task(o.story.ULID)
	if !slices.Equal(sharp.Relates, []model.ULID{o.story.ULID, o.root.ULID}) || !slices.Equal(sharp.Duplicates, []model.ULID{o.story.ULID}) || !slices.Equal(story.Relates, []model.ULID{o.sharp.ULID}) {
		t.Errorf("sharp: relates %v, duplicates %v; story: relates %v", sharp.Relates, sharp.Duplicates, story.Relates)
	}
	if len(sharp.Props) != 0 || len(story.Props) != 0 {
		t.Errorf("props are left: %v, %v", sharp.Props, story.Props)
	}
	// the step does not move updated; the story had no other change
	if !story.Updated.Equal(o.ws.Created) {
		t.Errorf("updated of the story = %v, want %v", story.Updated, o.ws.Created)
	}
	// the kind task_ref is gone from schema.yaml, and nothing is left to tell
	if rep, err := Run(ts.Session, Options{}, false); err != nil || rep != nil {
		t.Errorf("second run = %v, %v", rep, err)
	}
	r, err := ts.DoctorOne(false)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range r.Findings {
		if f.Check != "merge_driver" {
			t.Errorf("finding after the conversion: %+v", f)
		}
	}
}

func TestRunStopsAtAValueThatIsNoULIDEvenInADryRunAndDropsItWhenToldTo(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	o := makeOld(t, ts)
	withRelationProps(t, ts, o, map[*model.Task]string{o.root: "fixes: [WEB-9]\n"})
	before := headOf(t, ts)
	for _, dry := range []bool{true, false} {
		_, err := Run(ts.Session, Options{}, dry)
		wantCode(t, err, usecase.KindInvalid, "convert.bad_value")
		if !strings.Contains(err.Error(), "root task") || !strings.Contains(err.Error(), "props.fixes: WEB-9") {
			t.Errorf("dry run %v: the error does not list the value: %v", dry, err)
		}
		if headOf(t, ts) != before || formatOf(t, ts) != 1 {
			t.Errorf("dry run %v: the store changed", dry)
		}
	}
	reports, err := Run(ts.Session, Options{DropBadValues: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if l := strings.Join(reports[1].Lines, "\n"); !strings.Contains(l, "root task: props.fixes: WEB-9") {
		t.Errorf("the dry run does not list the dropped value:\n%s", l)
	}
	if headOf(t, ts) != before {
		t.Error("a dry run wrote")
	}
	if _, err := Run(ts.Session, Options{DropBadValues: true}, false); err != nil {
		t.Fatal(err)
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	if root, _ := ix.Task(o.root.ULID); len(root.Relates) != 0 || len(root.Props) != 0 {
		t.Errorf("root: relates %v, props %v", root.Relates, root.Props)
	}
}
