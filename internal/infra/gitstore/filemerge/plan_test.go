package filemerge

import (
	"strings"
	"testing"
)

func TestDropPurgedPlan(t *testing.T) {
	log := strings.Join([]string{
		`{"id":"1","at":"2026-01-01T00:00:00Z","by":"taro","actor":"user","t":"set","f":"sprints/S1/name","to":"a"}`,
		`{"id":"2","at":"2026-01-01T00:00:01Z","by":"taro","actor":"user","t":"delete","f":"sprints/S1"}`,
		`{"id":"3","at":"2026-01-01T00:00:02Z","by":"taro","actor":"user","t":"close_sprint","f":"sprints/S1/close"}`,
		`{"id":"4","at":"2026-01-01T00:00:03Z","by":"taro","actor":"user","t":"set","f":"sprints/S2/name","to":"b"}`,
		`{"id":"5","at":"2026-01-01T00:00:04Z","by":"taro","actor":"user","t":"set","f":"releases/R1/name","to":"c"}`,
		`{"id":"6","at":"2026-01-01T00:00:05Z","by":"taro","actor":"user","t":"set","f":"sprint","to":"S1"}`,
		``,
	}, "\n")
	want := strings.Join([]string{
		`{"id":"4","at":"2026-01-01T00:00:03Z","by":"taro","actor":"user","t":"set","f":"sprints/S2/name","to":"b"}`,
		`{"id":"5","at":"2026-01-01T00:00:04Z","by":"taro","actor":"user","t":"set","f":"releases/R1/name","to":"c"}`,
		`{"id":"6","at":"2026-01-01T00:00:05Z","by":"taro","actor":"user","t":"set","f":"sprint","to":"S1"}`,
		``,
	}, "\n")
	if got := string(DropPurgedPlan([]byte(log), map[string]bool{"S1": true})); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if got := string(DropPurgedPlan([]byte(log), nil)); got != log {
		t.Errorf("no targets must keep the log")
	}
}

func TestPurgedPlanTargets(t *testing.T) {
	purged := strings.Join([]string{
		`{"id":"1","at":"2026-01-01T00:00:00Z","entity":"sprint","target":"S1"}`,
		`{"id":"2","at":"2026-01-01T00:00:00Z","entity":"release","target":"R1"}`,
		`{"id":"3","at":"2026-01-01T00:00:00Z","entity":"item","target":"I1"}`,
		`{"id":"4","at":"2026-01-01T00:00:00Z","entity":"project","target":"P1"}`,
		`{"id":"","at":"2026-01-01T00:00:00Z","entity":"sprint","target":"S2"}`,
		`{"id":"7","at":"yesterday","entity":"sprint","target":"S3"}`,
		`broken`,
		``,
	}, "\n")
	got := PurgedPlanTargets([]byte(purged))
	if len(got) != 2 || !got["S1"] || !got["R1"] {
		t.Errorf("got %v", got)
	}
}
