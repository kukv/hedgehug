package cli

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
)

func TestSchemaLabelsAddAndRemove(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	before := len(te.log("me"))

	out := te.ok("schema", "labels", "add", "bug", "ui")
	if !strings.Contains(out, "Added 2 labels: bug, ui") {
		t.Errorf("add: stdout = %q", out)
	}
	if !strings.Contains(te.schemaFile(), "labels: [bug, ui]") {
		t.Errorf("schema.yaml:\n%s", te.schemaFile())
	}
	if n := len(te.log("me")); n != before+1 {
		t.Errorf("commits = %d, want one for the two labels (%d)", n, before+1)
	}
	if code, _, stderr := te.run("schema", "labels", "add", "BUG"); code == 0 || !strings.Contains(stderr, "case is ignored") {
		t.Errorf("a twin of a label: exit %d, stderr %q", code, stderr)
	}

	a := te.addJSON("Alpha", "-p", "labels+=Bug")
	if len(a.Labels) != 1 || a.Labels[0] != "bug" {
		t.Errorf("labels = %v, want the spelling of the list", a.Labels)
	}
	// a label a task has stays; the others go, and the answer is the schema
	head := te.headCommit("me").Hash
	code, _, stderr := te.run("schema", "labels", "rm", "bug")
	if code != 1 || !strings.Contains(stderr, "uses bug") {
		t.Errorf("a label in use: exit %d, stderr %q", code, stderr)
	}
	if code, _, stderr := te.run("schema", "labels", "rm", "nope"); code != 1 || !strings.Contains(stderr, "nope") {
		t.Errorf("a label that is not there: exit %d, stderr %q", code, stderr)
	}
	if te.headCommit("me").Hash != head {
		t.Error("a refused removal committed")
	}
	if out := te.ok("schema", "labels", "rm", "UI"); !strings.Contains(out, "Removed 1 label: ui") {
		t.Errorf("rm UI: stdout = %q, want the list's spelling", out)
	}
	te.ok("schema", "labels", "add", "ui")
	var w schemas.EditWrite
	if err := json.Unmarshal([]byte(te.ok("schema", "labels", "rm", "UI", "--json")), &w); err != nil || !slices.Equal(w.Schema.Labels, []string{"bug"}) || w.Commit == nil {
		t.Errorf("rm --json = %+v, %v", w, err)
	}
	te.ok("set", a.ID, "labels-=BUG")
	te.ok("schema", "labels", "rm", "bug")
	if !strings.Contains(te.schemaFile(), "labels: []") {
		t.Errorf("the list is empty, and the key stays:\n%s", te.schemaFile())
	}

	for _, args := range [][]string{{"schema", "labels"}, {"schema", "labels", "add"}, {"schema", "labels", "rm"}} {
		if code, _, _ := te.run(args...); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}

// A task takes the labels of the list and no others; an empty list says where
// to add them.
func TestSetLabelsTakesTheListOnly(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	code, _, stderr := te.run("set", a.ID, "labels+=bug")
	if code != 1 || !strings.Contains(stderr, "設定の「ラベル」で足してください") {
		t.Errorf("an empty list: exit %d, stderr %q", code, stderr)
	}
	te.ok("schema", "labels", "add", "bug")
	if code, _, stderr := te.run("set", a.ID, "labels+=ghost"); code != 1 || !strings.Contains(stderr, "ghost") {
		t.Errorf("a label the list lacks: exit %d, stderr %q", code, stderr)
	}
	te.ok("set", a.ID, "labels+=BUG")
	if got := te.task(a.ULID).Labels; !slices.Equal(got, []string{"bug"}) {
		t.Errorf("labels = %v, want [bug]", got)
	}
	// the query does not check the label
	if out := te.ok("ls", "label:nosuch"); strings.Contains(out, "Alpha") {
		t.Errorf("ls label:nosuch = %q", out)
	}
}

// priority= clears, a value of another case is spelled as the list has it, and
// an unreadable date in the file is put right by set.
func TestSetBuiltinProperties(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	te.ok("set", a.ID, "priority=URGENT", "start=2026-11-02", "due=2026-11-20", "estimate=2h", "points=5")
	tk := te.task(a.ULID)
	if tk.Priority != "urgent" || tk.Due == nil || tk.Start == nil || tk.Estimate == nil || tk.Points == nil || *tk.Points != 5 {
		t.Fatalf("task = %+v", tk)
	}
	out := te.ok("show", a.ID)
	for _, want := range []string{"priority", "urgent", "start", "2026-11-02", "due", "2026-11-20", "estimate", "2h", "points", "5"} {
		if !strings.Contains(out, want) {
			t.Errorf("show lacks %q:\n%s", want, out)
		}
	}
	te.ok("set", a.ID, "priority=", "due=", "points=")
	tk = te.task(a.ULID)
	if tk.Priority != "" || tk.Due != nil || tk.Points != nil || tk.Start == nil {
		t.Errorf("task = %+v", tk)
	}

	// a date that cannot be read is set aside, shown as a bad value, and set right
	raw := te.raw(a.ULID)
	te.commitFile(a.ULID, strings.Replace(raw, "\nstart: 2026-11-02", "\nstart: next week", 1))
	if code, out, _ := te.run("schema", "check"); code != 1 || !strings.Contains(out, "start") {
		t.Errorf("schema check: exit %d, stdout %q; want the bad start", code, out)
	}
	te.ok("set", a.ID, "start=2026-11-03")
	if got := te.task(a.ULID).Start; got == nil || got.String() != "2026-11-03" || strings.Contains(te.raw(a.ULID), "next week") {
		t.Errorf("start = %v; raw:\n%s", got, te.raw(a.ULID))
	}
	te.commitFile(a.ULID, strings.Replace(te.raw(a.ULID), "\nstart: 2026-11-03", "\nstart: next week", 1))
	te.ok("set", a.ID, "start=")
	if strings.Contains(te.raw(a.ULID), "next week") {
		t.Errorf("start= left the unreadable value:\n%s", te.raw(a.ULID))
	}

	for _, bad := range []string{"start=tomorrow", "due=2026-02-30", "estimate=1d", "points=lots", "priority=soon"} {
		if code, _, _ := te.run("set", a.ID, bad); code != 1 {
			t.Errorf("%s: exit %d, want 1", bad, code)
		}
	}
}
