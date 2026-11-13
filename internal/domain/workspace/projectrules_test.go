package workspace

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

func TestWorkspaceMultiCoversProjectRules(t *testing.T) {
	meta := change.Meta{Op: "OP", At: time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC), Actor: change.User("taro")}
	n := 0
	log := change.Stamp(nil, []change.Draft{
		change.NewCreate(map[string]any{"id": "W", "prefix": "HH"}),
		change.NewAdd("project_rules/P1/hidden_statuses", "S2"),
		change.NewAdd("project_rules/P1/hidden_statuses", "S3"),
	}, meta, func() change.ID { n++; return change.ID("R" + string(rune('A'+n))) })
	if got := change.Resolve(log, opts).Lists["project_rules/P1/hidden_statuses"]; len(got) != 2 {
		t.Errorf("Lists = %v", got)
	}
}
