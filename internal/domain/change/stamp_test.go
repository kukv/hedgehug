package change_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// GitHub の操作の中の自動化の行は、by が hedgehug[bot] になる(D4a-15)。
func TestStampRuleRowInGitHubOperation(t *testing.T) {
	const rule change.ID = "01JF0000000000000000000001"
	meta := change.Meta{Op: "o", At: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC), Actor: change.Actor{Kind: change.ActorGitHub}}
	recs := change.Stamp(nil, []change.Draft{change.NewSet("status", nil, "Done").ByRule(rule)}, meta, func() change.ID { return "r1" })
	if len(recs) != 1 || recs[0].By != change.BotLogin || recs[0].Actor.Code() != "automation:"+string(rule) {
		t.Errorf("records = %+v", recs)
	}
}
