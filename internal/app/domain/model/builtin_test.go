package model

import (
	"slices"
	"strings"
	"testing"
)

func taskSrc(front string) []byte {
	return []byte("---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\n" + front + "---\n")
}

// A typed value that cannot be read does not break the task: it stays as an
// unknown key, and the file is written as it was until a value is set.
func TestTaskUnreadableTypedValues(t *testing.T) {
	for _, c := range []struct{ key, line string }{
		{"due", "due: next week\n"},
		{"start", "start: 2026-13-01\n"},
		{"estimate", "estimate: 1d\n"},
		{"points", "points: many\n"},
		{"points", "points: .nan\n"},
		{"points", "points: .inf\n"},
		{"priority", "priority: [a, b]\n"},
		{"due", "due: [2026-11-03]\n"},
	} {
		src := taskSrc("title: x\n" + c.line)
		task, err := DecodeTask(src)
		if err != nil {
			t.Fatalf("%q: %v", c.line, err)
		}
		if task.Due != nil || task.Start != nil || task.Estimate != nil || task.Points != nil || task.Priority != "" {
			t.Errorf("%q: a field was set: %+v", c.line, task)
		}
		if task.Extra.Get(c.key) == nil || task.Title != "x" {
			t.Errorf("%q: Extra = %v, title = %q", c.line, task.Extra, task.Title)
		}
		out, err := task.Encode()
		if err != nil || string(out) != string(src) {
			t.Errorf("%q: round trip = %q, %v", c.line, out, err)
		}
	}
}

func TestTaskUnreadableTypedValueFixed(t *testing.T) {
	task, err := DecodeTask(taskSrc("title: x\ndue: next week\ncustom: 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := ParseDate("2026-11-10")
	if err != nil {
		t.Fatal(err)
	}
	task.Due = &d
	out, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\ntitle: x\ndue: 2026-11-10\ncustom: 1\n---\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	back, err := DecodeTask(out)
	if err != nil || back.Due == nil || back.Due.String() != "2026-11-10" || back.Extra.Get("due") != nil {
		t.Errorf("back = %+v, %v", back, err)
	}
}

func TestTaskBuiltinProperties(t *testing.T) {
	task, err := DecodeTask(taskSrc("priority: urgent\nstart: 2026-11-03\ndue: 2026-11-10\nestimate: 1h30m\npoints: 0\n"))
	if err != nil {
		t.Fatal(err)
	}
	if task.Priority != "urgent" || task.Start.String() != "2026-11-03" || task.Due.String() != "2026-11-10" ||
		task.Estimate.String() != "1h30m" || task.Points == nil || *task.Points != 0 || len(task.Extra) != 0 {
		t.Errorf("task = %+v", task)
	}
	out, err := task.Encode()
	if err != nil || !strings.Contains(string(out), "points: 0\n") {
		t.Errorf("points 0 is not kept: %q, %v", out, err)
	}
	fresh := &Task{ULID: task.ULID, Points: new(float64)}
	out, err = fresh.Encode()
	if err != nil || !strings.Contains(string(out), "points: 0\n") {
		t.Errorf("new task: %q, %v", out, err)
	}
	for _, k := range []string{"priority", "start", "due", "estimate", "points"} {
		if !slices.Contains(TaskKeys(), k) {
			t.Errorf("TaskKeys lacks %s", k)
		}
	}
}

func TestTemplateBuiltinProperties(t *testing.T) {
	t.Parallel()
	src := []byte("---\nkind: task\npriority: high\nstart: 2026-11-03\ndue: 2026-11-10\nestimate: 2h\npoints: 3\n" +
		"children:\n- {title: 子, priority: low, due: 2026-11-04, estimate: 30m, points: 1}\n---\n")
	tp, err := DecodeTemplate("builtin", src)
	if err != nil {
		t.Fatal(err)
	}
	it := tp.Item
	if it.Priority != "high" || it.Start.String() != "2026-11-03" || it.Due.String() != "2026-11-10" || it.Estimate.String() != "2h" || *it.Points != 3 {
		t.Errorf("item = %+v", it)
	}
	ch := it.Children[0]
	if ch.Priority != "low" || ch.Due.String() != "2026-11-04" || ch.Estimate.String() != "30m" || *ch.Points != 1 || ch.Start != nil {
		t.Errorf("child = %+v", ch)
	}
	out, err := tp.Encode()
	if err != nil || string(out) != string(src) {
		t.Fatalf("round trip changed the file (%v):\n%s", err, out)
	}
}
