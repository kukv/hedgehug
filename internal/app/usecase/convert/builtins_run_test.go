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

// withBuiltinProps adds to the store of makeOld (format 1) the properties of the
// five names that format 3 had, values in the props of the tasks, labels and a
// list of labels: the story has points, priority and a label of the list, the
// chore has a due and a label the list lacks.
func withBuiltinProps(t *testing.T, ts *usecasetest.Session, o *oldTasks, extra map[*model.Task]string) {
	t.Helper()
	props := map[*model.Task]string{
		o.story: "points: \"5\"\npriority: High\n",
		o.sharp: "due: 2026-11-08T15:30:00+09:00\n",
	}
	for tk, p := range extra {
		props[tk] = p
	}
	o.story.Labels = []string{"bug"}
	o.sharp.Labels = []string{"wip"}
	files := map[string]string{"schema.yaml": oldSchema + "props:\n  points: {type: text}\n  due: {type: datetime}\nlabels: [bug]\n"}
	for _, tk := range []*model.Task{o.story, o.sharp, o.root} {
		if err := yaml.Unmarshal([]byte(props[tk]), &tk.Props); err != nil {
			t.Fatal(err)
		}
		b, err := tk.Encode()
		if err != nil {
			t.Fatal(err)
		}
		files[model.TaskPath(tk.ULID)] = string(b)
	}
	commitRaw(t, ts, "builtin props", files)
	ts.DropMount()
}

func TestRunConvertsFromFormat1ToTheBuiltinsInOneRunAndNothingTheSecondTime(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	o := makeOld(t, ts)
	withBuiltinProps(t, ts, o, nil)
	reports, err := Run(ts.Session, Options{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 3 || reports[2].From != 3 || reports[2].To != 4 {
		t.Fatalf("reports = %+v", reports)
	}
	l := strings.Join(reports[2].Lines, "\n")
	for _, w := range []string{"同名の項目へ移した: 3 件", "ラベル wip を外した", "日付にして時刻を落とした"} {
		if !strings.Contains(l, w) {
			t.Errorf("the report of step 3 to 4 lacks %q:\n%s", w, l)
		}
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	story, _ := ix.Task(o.story.ULID)
	sharp, _ := ix.Task(o.sharp.ULID)
	if story.Points == nil || *story.Points != 5 || story.Priority != "high" || !slices.Equal(story.Labels, []string{"bug"}) || len(story.Props) != 0 {
		t.Errorf("story: points %v, priority %q, labels %v, props %v", story.Points, story.Priority, story.Labels, story.Props)
	}
	if sharp.Due == nil || sharp.Due.String() != "2026-11-08" || len(sharp.Labels) != 0 {
		t.Errorf("chore: due %v, labels %v", sharp.Due, sharp.Labels)
	}
	// the step does not move updated; the story had no other change
	if !story.Updated.Equal(o.ws.Created) {
		t.Errorf("updated of the story = %v, want %v", story.Updated, o.ws.Created)
	}
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

func TestRunStopsAtAnEstimateThatIsNoDurationEvenInADryRunAndDropsItWhenToldTo(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	o := makeOld(t, ts)
	withBuiltinProps(t, ts, o, map[*model.Task]string{o.root: "estimate: 3\n"})
	before := headOf(t, ts)
	for _, dry := range []bool{true, false} {
		_, err := Run(ts.Session, Options{}, dry)
		wantCode(t, err, usecase.KindInvalid, "convert.bad_value")
		if !strings.Contains(err.Error(), "root task") || !strings.Contains(err.Error(), "props.estimate: 3") {
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
	if l := strings.Join(reports[2].Lines, "\n"); !strings.Contains(l, "root task: props.estimate: 3") {
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
	if root, _ := ix.Task(o.root.ULID); root.Estimate != nil || len(root.Props) != 0 {
		t.Errorf("root: estimate %v, props %v", root.Estimate, root.Props)
	}
}
