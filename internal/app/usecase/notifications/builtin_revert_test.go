package notifications_test

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// An item recorded before the built-in properties were fields names them as
// properties (props.due, ...). The revert puts the value in the field of the
// same name, read as the step to format 4 reads it.
func TestInboxRevertPutsAFormer3PropertyBackInTheField(t *testing.T) {
	t.Parallel()
	h := newHistory(t)
	task := h.ts.AddTask("t", nil)
	for _, c := range []struct{ field, value string }{
		{"props.due", `"2026-11-08T15:30:00+09:00"`}, // a datetime is the day it is written in
		{"props.start", `"2026-11-02T00:00:00Z"`},
		{"props.points", `"3"`}, // text of a number
		{"props.estimate", `["1h30m"]`},
		{"props.priority", `"HIGH"`}, // spelled as the list has it
	} {
		u := h.ts.AddItem(overwritten(task, h.old, c.field, c.value))
		if _, err := notifications.InboxRevert(h.ts.Session, u.String()); err != nil {
			t.Fatalf("%s %s: %v", c.field, c.value, err)
		}
	}
	_, ix, err := h.ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := ix.Task(task.ULID)
	if e.Due == nil || e.Due.String() != "2026-11-08" || e.Start == nil || e.Start.String() != "2026-11-02" ||
		e.Points == nil || *e.Points != 3 || e.Estimate == nil || e.Estimate.String() != "1h30m" || e.Priority != "high" {
		t.Errorf("due %v, start %v, points %v, estimate %v, priority %q", e.Due, e.Start, e.Points, e.Estimate, e.Priority)
	}
	if len(e.Props) != 0 {
		t.Errorf("props = %v; nothing goes to props", e.Props)
	}
	// null clears the field
	u := h.ts.AddItem(overwritten(task, h.old, "props.points", `null`))
	if _, err := notifications.InboxRevert(h.ts.Session, u.String()); err != nil {
		t.Fatal(err)
	}
	_, ix, _ = h.ts.Open()
	if e, _ = ix.Task(task.ULID); e.Points != nil {
		t.Errorf("points = %v after a null", *e.Points)
	}
}

// A value that cannot be read as the type of the field is not reverted, and
// nothing is written.
func TestInboxRevertRefusesAFormer3ValueThatCannotBeRestored(t *testing.T) {
	t.Parallel()
	h := newHistory(t)
	task := h.ts.AddTask("t", nil)
	for _, c := range []struct{ field, value string }{
		{"props.priority", `"minor"`}, // not in the list now
		{"props.estimate", `3`},
		{"props.points", `"lots"`},
		{"props.due", `"next week"`},
		{"props.start", `["2026-11-01","2026-11-02"]`},
	} {
		u := h.ts.AddItem(overwritten(task, h.old, c.field, c.value))
		head := h.ts.Head()
		_, err := notifications.InboxRevert(h.ts.Session, u.String())
		usecasetest.WantErr(t, err, usecase.KindInvalid, "inbox.not_revertable")
		if err != nil && !strings.Contains(err.Error(), c.field) {
			t.Errorf("%s %s: the reason does not name the field: %v", c.field, c.value, err)
		}
		if h.ts.Head() != head {
			t.Errorf("%s %s: a refused revert wrote", c.field, c.value)
		}
	}
}
