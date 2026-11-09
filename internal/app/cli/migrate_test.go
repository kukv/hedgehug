package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// putSchema writes the schema of the store me after edit changes the
// default one.
func (te *testEnv) putSchema(edit func(*schema.Schema)) {
	te.t.Helper()
	sc := schema.Default()
	edit(sc)
	te.uc().MustEdit("me", "update", usecasetest.AliceAuthor, func(tx *usecasetest.Tx) error { return tx.PutSchema(sc) })
}

func (te *testEnv) schemaFile() string {
	te.t.Helper()
	b, err := os.ReadFile(filepath.Join(te.storeDir("me"), "schema.yaml"))
	if err != nil {
		te.t.Fatal(err)
	}
	return string(b)
}

type migrateOut struct {
	Changed []usecase.TaskView `json:"changed"`
	Skipped []string           `json:"skipped"`
	Commit  *string            `json:"commit"`
}

func TestSchemaMigrateStatus(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	b := te.addJSON("Beta")
	te.ok("rm", b.ID)
	c := te.addJSON("Gamma")
	te.ok("status", c.ID, "done")
	d := te.addJSON("Broken")
	te.commitFile(d.ULID, "not a task\n")
	te.putSchema(func(sc *schema.Schema) {
		// todo may not move to ready, but a migration is not a move.
		sc.Workflow.Transitions = schema.Lists{{Key: "todo", Values: []string{"in_progress"}}}
	})
	before := len(te.log("me"))

	code, out, stderr := te.run("schema", "migrate", "status", "todo", "ready")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.HasPrefix(out, "Migrated 2 tasks\n") || !strings.Contains(out, label(a)+"  Alpha\n") || !strings.Contains(out, label(b)+"  Beta\n") || strings.Contains(out, "Gamma") {
		t.Errorf("stdout = %q", out)
	}
	if !strings.Contains(stderr, "warning: not migrated, since they do not decode: "+d.ULID+"\n") {
		t.Errorf("stderr = %q, want a warning about %s", stderr, d.ULID)
	}
	log := te.log("me")
	if len(log) != before+1 || !strings.Contains(log[0].Message, "Hedgehug-Op: migrate") {
		t.Errorf("commits = %d (want %d), message %q", len(log), before+1, log[0].Message)
	}
	if st := te.task(a.ULID).Status; st != "ready" {
		t.Errorf("Alpha status = %q, want ready", st)
	}
	if st := te.task(b.ULID).Status; st != "ready" {
		t.Errorf("deleted Beta status = %q, want ready", st)
	}
	if got := te.raw(d.ULID); got != "not a task\n" {
		t.Errorf("broken task = %q", got)
	}
	sf := te.schemaFile()
	if strings.Contains(sf, "name: todo") || !strings.Contains(sf, "{name: ready, label: Todo, category: todo, default: true}") || !strings.Contains(sf, "ready: [in_progress]") {
		t.Errorf("schema.yaml:\n%s", sf)
	}
	if out := te.ok("ls", "status:ready"); !strings.Contains(out, "Alpha") {
		t.Errorf("ls status:ready = %q", out)
	}
}

// A file at the path of one task that holds another, as a stale copy, is
// left alone: writing it would overwrite the real file of the other task.
func TestSchemaMigrateSkipsMismatchedULID(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	d := te.addJSON("Delta")
	stale := te.raw(a.ULID)
	te.commitFile(d.ULID, stale)
	te.ok("set", a.ID, "title=Alpha2")

	code, out, stderr := te.run("schema", "migrate", "status", "todo", "ready")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.HasPrefix(out, "Migrated 1 task\n") || !strings.Contains(out, label(a)+"  Alpha2\n") {
		t.Errorf("stdout = %q", out)
	}
	if !strings.Contains(stderr, "warning: not migrated, since they do not decode: "+d.ULID+"\n") {
		t.Errorf("stderr = %q, want a warning about %s only", stderr, d.ULID)
	}
	if x := te.task(a.ULID); x.Title != "Alpha2" || x.Status != "ready" {
		t.Errorf("Alpha = title %q, status %q; want Alpha2, ready", x.Title, x.Status)
	}
	if got := te.raw(d.ULID); got != stale {
		t.Errorf("the file at Delta's path = %q, want it unchanged", got)
	}
}

func TestSchemaMigrateJSON(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	te.ok("set", a.ID, "priority=mid")
	te.addJSON("Beta")
	d := te.addJSON("Broken")
	te.commitFile(d.ULID, "not a task\n")

	var got migrateOut
	out := te.ok("schema", "migrate", "priority", "mid", "medium", "--json")
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if got.Commit == nil {
		t.Errorf("commit = nil, want the commit made")
	}
	if len(got.Changed) != 1 || got.Changed[0].ID != a.ID || got.Changed[0].Priority == nil || *got.Changed[0].Priority != "medium" {
		t.Errorf("changed = %+v", got.Changed)
	}
	if len(got.Skipped) != 1 || got.Skipped[0] != d.ULID {
		t.Errorf("skipped = %v, want [%s]", got.Skipped, d.ULID)
	}
	if !strings.Contains(te.schemaFile(), "priorities: [low, medium, high, urgent]") {
		t.Errorf("schema.yaml:\n%s", te.schemaFile())
	}
}

// The priorities and the labels are renamed, or merged into another, with the
// tasks that have them; a dry run writes nothing.
func TestSchemaMigratePriorityAndLabel(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.labels("bug", "ui")
	a := te.addJSON("Alpha")
	b := te.addJSON("Beta")
	te.ok("set", a.ID, "priority=high", "labels+=bug", "labels+=ui")
	te.ok("set", b.ID, "priority=urgent", "labels+=BUG")
	before := len(te.log("me"))
	sf := te.schemaFile()

	out := te.ok("schema", "migrate", "--dry-run", "priority", "high", "urgent")
	if !strings.HasPrefix(out, "Would migrate 1 task\n") || !strings.Contains(out, label(a)+"  Alpha\n") {
		t.Errorf("dry run: stdout = %q", out)
	}
	if len(te.log("me")) != before || te.schemaFile() != sf {
		t.Error("a dry run wrote")
	}

	out = te.ok("schema", "migrate", "priority", "high", "urgent")
	if !strings.HasPrefix(out, "Migrated 1 task\n") {
		t.Errorf("stdout = %q", out)
	}
	if got := te.task(a.ULID).Priority; got != "urgent" {
		t.Errorf("Alpha priority = %q, want urgent", got)
	}
	if !strings.Contains(te.schemaFile(), "priorities: [low, mid, urgent]") {
		t.Errorf("schema.yaml:\n%s", te.schemaFile())
	}

	// Beta has the label in another case, and is found all the same
	out = te.ok("schema", "migrate", "label", "bug", "ui")
	if !strings.HasPrefix(out, "Migrated 2 tasks\n") {
		t.Errorf("stdout = %q", out)
	}
	if got := te.task(a.ULID).Labels; !slices.Equal(got, []string{"ui"}) {
		t.Errorf("Alpha labels = %v, want [ui] (the merge leaves one)", got)
	}
	if got := te.task(b.ULID).Labels; !slices.Equal(got, []string{"ui"}) {
		t.Errorf("Beta labels = %v, want [ui]", got)
	}
	if !strings.Contains(te.schemaFile(), "labels: [ui]") {
		t.Errorf("schema.yaml:\n%s", te.schemaFile())
	}
	if code, _, stderr := te.run("schema", "migrate", "label", "nope", "x"); code == 0 {
		t.Errorf("a label the list lacks was migrated: stderr %q", stderr)
	}
}

// The kinds are fixed, so there is nothing to migrate for them.
func TestSchemaMigrateProp(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.putSchema(func(sc *schema.Schema) {
		sc.Props = append(sc.Props, schema.Prop{Name: "since", Type: schema.TypeDate, Label: "開始"}, schema.Prop{Name: "until", Type: schema.TypeDate})
	})
	a := te.addJSON("Alpha")
	te.ok("set", a.ID, "since=2026-11-01")
	if code, _, _ := te.run("schema", "migrate", "kind", "bug", "task"); code != 2 {
		t.Errorf("migrate kind: exit %d, want 2", code)
	}
	te.ok("schema", "migrate", "prop", "since", "begin")
	x := te.task(a.ULID)
	if x.Props.Get("begin") == nil || x.Props.Get("since") != nil {
		t.Errorf("task = props %v", x.Props)
	}
	if sf := te.schemaFile(); !strings.Contains(sf, "begin: {type: date, label: 開始}") {
		t.Errorf("schema.yaml:\n%s", sf)
	}
}

// Properties are not merged: the CLI refuses, naming both, and writes nothing.
func TestSchemaMigratePropIntoAnother(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.putSchema(func(sc *schema.Schema) {
		sc.Props = append(sc.Props, schema.Prop{Name: "since", Type: schema.TypeDate}, schema.Prop{Name: "until", Type: schema.TypeDate})
	})
	a := te.addJSON("Alpha")
	te.ok("set", a.ID, "since=2026-11-01", "until=2026-11-30")
	before := len(te.log("me"))
	for _, args := range [][]string{{"schema", "migrate", "prop", "since", "until"}, {"schema", "migrate", "--dry-run", "prop", "since", "until"}} {
		code, _, stderr := te.run(args...)
		if code != 1 || !strings.Contains(stderr, "since into until") || !strings.Contains(stderr, "cannot be merged") {
			t.Errorf("%v: exit %d, stderr %q", args, code, stderr)
		}
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

// Merging into a status with required fields keeps the tasks that lack
// them: a migration is not a transition, and they did not have them before.
func TestSchemaMigrateMergeIntoRequired(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	te.ok("status", a.ID, "backlog")
	te.putSchema(func(sc *schema.Schema) {
		sc.Workflow.Required = schema.Lists{{Key: "todo", Values: []string{"points"}}}
	})
	te.ok("schema", "migrate", "status", "backlog", "todo")
	if st := te.task(a.ULID).Status; st != "todo" {
		t.Errorf("status = %q, want todo", st)
	}
}

// A rename keeps a task that already lacked a required field of the status.
func TestSchemaMigrateKeepsMissingRequired(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	te.ok("status", a.ID, "in_progress")
	te.putSchema(func(sc *schema.Schema) {
		sc.Workflow.Required = schema.Lists{{Key: "in_progress", Values: []string{"assignees"}}}
	})
	code, out, stderr := te.run("schema", "migrate", "status", "in_progress", "doing")
	if code != 0 || !strings.HasPrefix(out, "Migrated 1 task\n") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	if st := te.task(a.ULID).Status; st != "doing" {
		t.Errorf("status = %q, want doing", st)
	}
	if sf := te.schemaFile(); !strings.Contains(sf, "doing: [assignees]") {
		t.Errorf("schema.yaml:\n%s", sf)
	}
}

func TestSchemaMigrateRejectsBadSchema(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.addJSON("Alpha")
	before := len(te.log("me"))
	code, _, stderr := te.run("schema", "migrate", "status", "todo", "in_progress")
	if code != 1 || !strings.Contains(stderr, "needs exactly one default status") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestSchemaMigrateDryRun(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	before := len(te.log("me"))
	sf := te.schemaFile()
	out := te.ok("schema", "migrate", "status", "todo", "ready", "--dry-run")
	if !strings.HasPrefix(out, "Would migrate 1 task\n") || !strings.Contains(out, label(a)+"  Alpha\n") {
		t.Errorf("stdout = %q", out)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
	if te.schemaFile() != sf || te.task(a.ULID).Status != "todo" {
		t.Error("the store changed")
	}

	var got migrateOut
	out = te.ok("schema", "migrate", "status", "todo", "ready", "--dry-run", "--json")
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if got.Commit != nil {
		t.Errorf("commit = %v, want null for a dry run", *got.Commit)
	}
	if len(got.Changed) != 1 || got.Changed[0].ID != a.ID || got.Changed[0].Status != "ready" {
		t.Errorf("changed = %+v, want Alpha as migrated", got.Changed)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestSchemaMigrateArgs(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"schema"}, 2},
		{[]string{"schema", "migrate"}, 2},
		{[]string{"schema", "migrate", "nope", "a", "b"}, 2},
		{[]string{"schema", "migrate", "status", "todo"}, 2},
		{[]string{"schema", "migrate", "status", "todo", "a", "b"}, 2},
		{[]string{"schema", "migrate", "value", "priority", "low"}, 2},
		{[]string{"schema", "migrate", "status", "nope", "x"}, 1},
		{[]string{"schema", "migrate", "status", "todo", "todo"}, 1},
	} {
		if code, _, stderr := te.run(tc.args...); code != tc.code {
			t.Errorf("%v: exit %d, want %d (stderr %q)", tc.args, code, tc.code, stderr)
		}
	}
}

// A rename that makes a task break the schema is refused, naming the task: a
// value that was bad under the old name is a new violation under the new one.
func TestSchemaMigrateRejectsNewViolation(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.putSchema(func(sc *schema.Schema) {
		sc.Props = append(sc.Props, schema.Prop{Name: "score", Type: schema.TypeNumber})
	})
	a := te.addJSON("Alpha")
	te.ok("set", a.ID, "score=1")
	te.commitFile(a.ULID, strings.Replace(te.raw(a.ULID), "score: 1", "score: abc", 1))
	before := len(te.log("me"))
	sf := te.schemaFile()
	code, _, stderr := te.run("schema", "migrate", "prop", "score", "size")
	if code != 1 || !strings.Contains(stderr, "breaks the schema") || !strings.Contains(stderr, label(a)+" Alpha") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if n := len(te.log("me")); n != before || te.schemaFile() != sf {
		t.Errorf("commits = %d (want %d), or the schema changed", n, before)
	}
}

// A task that holds the new name of a property as one the schema does not
// define is refused, naming the task.
func TestSchemaMigrateRejectsATaskWithTheNewName(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.putSchema(func(sc *schema.Schema) {
		sc.Props = append(sc.Props, schema.Prop{Name: "note", Type: schema.TypeText})
	})
	a := te.addJSON("Alpha")
	te.ok("set", a.ID, "note=y")
	te.commitFile(a.ULID, strings.Replace(te.raw(a.ULID), "  note: y\n", "  note: y\n  custom: z\n", 1))
	before := len(te.log("me"))
	code, _, stderr := te.run("schema", "migrate", "prop", "note", "custom")
	if code != 1 || !strings.Contains(stderr, "both note and custom") || !strings.Contains(stderr, label(a)+" Alpha") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

// The priorities and the labels are found without regard to case, in the old
// name and in the new one; the list's spelling is what stays.
func TestSchemaMigrateIgnoresCase(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.labels("bug", "ui")
	a := te.addJSON("Alpha")
	te.ok("set", a.ID, "priority=low", "labels+=bug")
	te.ok("schema", "migrate", "label", "Bug", "UI")
	if got := te.task(a.ULID).Labels; !slices.Equal(got, []string{"ui"}) {
		t.Errorf("labels = %v, want [ui]", got)
	}
	if !strings.Contains(te.schemaFile(), "labels: [ui]") {
		t.Errorf("schema.yaml:\n%s", te.schemaFile())
	}
	te.ok("schema", "migrate", "priority", "LOW", "High")
	if got := te.task(a.ULID).Priority; got != "high" {
		t.Errorf("priority = %q, want high", got)
	}
	if !strings.Contains(te.schemaFile(), "priorities: [high, mid, urgent]") {
		t.Errorf("schema.yaml:\n%s", te.schemaFile())
	}
	for _, args := range [][]string{{"label", "ui", "UI"}, {"priority", "high", "HIGH"}, {"status", "todo", "todo"}} {
		code, _, stderr := te.run(append([]string{"schema", "migrate"}, args...)...)
		if code != 1 || !strings.Contains(stderr, "are the same") {
			t.Errorf("%v: exit %d, stderr %q", args, code, stderr)
		}
	}
	for _, args := range [][]string{
		{"label", "nope", "x"}, {"priority", "nope", "x"}, {"status", "nope", "x"}, {"prop", "nope", "x"}, {"value", "nope", "a", "b"},
	} {
		code, _, stderr := te.run(append([]string{"schema", "migrate"}, args...)...)
		if code != 1 || !strings.Contains(stderr, "nope") || !strings.Contains(stderr, "not defined") {
			t.Errorf("%v: exit %d, stderr %q", args, code, stderr)
		}
	}
}
