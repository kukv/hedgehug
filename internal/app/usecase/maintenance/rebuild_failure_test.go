package maintenance_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/maintenance"
)

// A failed clone is told in the finding of the force push, not in a second
// finding of the same check.
func TestWithRebuildFailureKeepsOneFinding(t *testing.T) {
	t.Parallel()
	rerr := errors.New("disk full")
	have := []maintenance.Finding{{Check: "unexpected_force_push", Where: "me", Msg: "the history differs", Hint: "run doctor --fix"}}
	got := maintenance.WithRebuildFailure(have, "me", rerr)
	if len(got) != 1 || !strings.HasPrefix(got[0].Msg, "the history differs; ") || !strings.Contains(got[0].Msg, "cannot clone the store again: disk full") || !strings.Contains(got[0].Hint, "doctor --fix again") {
		t.Errorf("findings = %+v; want the one finding to tell of both", got)
	}
	got = maintenance.WithRebuildFailure(nil, "me", rerr)
	if len(got) != 1 || got[0].Check != "unexpected_force_push" || got[0].Where != "me" || !strings.Contains(got[0].Msg, "disk full") {
		t.Errorf("findings = %+v; want a finding of its own", got)
	}
}
