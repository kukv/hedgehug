package maintenance_test

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

// --fix moves a datetime left in props.due as the day it is written in, and
// says which time it left out, with the value as written.
func TestDoctorFixListsTheTimeOfADateItMade(t *testing.T) {
	ts := newCheckedSession(t)
	a := ts.AddTask("a", func(tk *model.Task) {
		if err := yaml.Unmarshal([]byte("due: 2026-11-08T15:30:00+09:00\nstart: 2026-11-04T00:00:00Z\n"), &tk.Props); err != nil {
			t.Fatal(err)
		}
	})
	rep, err := ts.DoctorOne(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Repaired) != 1 {
		t.Fatalf("repaired = %+v", rep.Repaired)
	}
	line := rep.Repaired[0].Line
	if !strings.Contains(line, "the time of props.due was left out: 2026-11-08T15:30:00+09:00") || strings.Contains(line, "props.start was left out") {
		t.Errorf("line = %q", line)
	}
	if na := ts.TaskByULID("me", a.ULID); na.Due == nil || na.Due.String() != "2026-11-08" || len(na.Props) != 0 {
		t.Errorf("a: due %v, props %v", na.Due, na.Props)
	}
}
