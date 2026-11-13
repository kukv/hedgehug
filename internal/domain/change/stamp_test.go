package change_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Op を持つ行は、操作の op ではなくその op で書く。持たない行は操作の op。
func TestStampUsesDraftOp(t *testing.T) {
	meta := change.Meta{Op: "opW", At: time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC), Actor: change.Actor{Kind: change.ActorSync}}
	n := 0
	ids := func() change.ID { n++; return change.ID(fmt.Sprintf("r%d", n)) }
	got := change.Stamp(nil, []change.Draft{change.NewDeleteEntity(false).InOp("opP"), change.NewSet("title", "a", "b")}, meta, ids)
	if got[0].Op != "opP" || got[1].Op != "opW" {
		t.Fatalf("ops = %s, %s", got[0].Op, got[1].Op)
	}
}

// GitHub の操作の中の自動化の行は、by が hedgehug[bot] になる(D4a-15)。
func TestStampRuleRowInGitHubOperation(t *testing.T) {
	const rule change.ID = "01JF0000000000000000000001"
	meta := change.Meta{Op: "o", At: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC), Actor: change.Actor{Kind: change.ActorGitHub}}
	recs := change.Stamp(nil, []change.Draft{change.NewSet("status", nil, "Done").ByRule(rule)}, meta, func() change.ID { return "r1" })
	if len(recs) != 1 || recs[0].By != change.BotLogin || recs[0].Actor.Code() != "automation:"+string(rule) {
		t.Errorf("records = %+v", recs)
	}
}
