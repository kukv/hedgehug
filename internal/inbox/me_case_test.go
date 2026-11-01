package inbox

import (
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

// An account ID is compared without case (spec: 大文字小文字): Me written as
// github.com/B still catches what is addressed to github.com/b.
func TestCatchComparesMeWithoutCase(t *testing.T) {
	t.Parallel()
	a, b := twoClones(t)
	x := addTask(t, a, "x", nil)
	syncBoth(t, a, b)
	before := head(t, b)
	update(t, a, x.ULID, func(t *model.Task) { t.Assignees = []string{"github.com/b"} })
	syncBoth(t, a, b)
	added, err := Open(b.s).Catch([]string{"GitHub.com/B"}, before)
	if err != nil || len(added) != 1 || added[0].Kind != Assigned {
		t.Errorf("Catch added %+v, %v", added, err)
	}
}
