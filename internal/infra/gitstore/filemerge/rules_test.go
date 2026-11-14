package filemerge

import (
	"strings"
	"testing"
)

func TestDropPurgedRules(t *testing.T) {
	rec := func(id, f, to string) string {
		return `{"id":"` + id + `","at":"2026-01-01T00:00:00Z","by":"taro","actor":"user","t":"set","f":"` + f + `","to":"` + to + `"}`
	}
	// R1 の scope は P1、R3 の scope は P2。R2 は scope がなく、ワークスペース全体のルール。
	create := `{"id":"0","at":"2026-01-01T00:00:00Z","by":"taro","actor":"user","t":"create","to":{"id":"W1","name":"W1","prefix":"HH"}}`
	log := strings.Join([]string{
		create,
		rec("1", "project_rules/P1/hidden_statuses", "x"),
		rec("2", "rules/R1/scope", "P1"),
		rec("3", "rules/R1/name", "x"),
		rec("4", "rules/R1", "x"),
		rec("5", "project_rules/P2/hidden_statuses", "x"),
		rec("6", "rules/R2/name", "x"),
		rec("7", "rules/R3/scope", "P2"),
		rec("8", "sprints/S1/name", "x"),
		``,
	}, "\n")
	want := strings.Join([]string{
		create,
		rec("5", "project_rules/P2/hidden_statuses", "x"),
		rec("6", "rules/R2/name", "x"),
		rec("7", "rules/R3/scope", "P2"),
		rec("8", "sprints/S1/name", "x"),
		``,
	}, "\n")
	if got := string(DropPurgedRules([]byte(log), map[string]bool{"P1": true})); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if got := string(DropPurgedRules([]byte(log), nil)); got != log {
		t.Errorf("no projects must keep the log")
	}
}

func TestPurgedProjects(t *testing.T) {
	purged := strings.Join([]string{
		`{"id":"1","at":"2026-01-01T00:00:00Z","entity":"project","target":"P1"}`,
		`{"id":"2","at":"2026-01-01T00:00:00Z","entity":"item","target":"I1"}`,
		`{"id":"3","at":"2026-01-01T00:00:00Z","entity":"sprint","target":"S1"}`,
		`{"id":"","at":"2026-01-01T00:00:00Z","entity":"project","target":"P2"}`,
		`broken`,
		``,
	}, "\n")
	got := PurgedProjects([]byte(purged))
	if len(got) != 1 || !got["P1"] {
		t.Errorf("got %v", got)
	}
}
