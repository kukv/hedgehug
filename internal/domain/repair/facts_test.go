package repair_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

func TestFactsSide(t *testing.T) {
	f := repair.Facts{Ours: map[change.ID]bool{"o": true}, Theirs: map[change.ID]bool{"t": true}}
	for id, want := range map[change.ID]int{"o": 1, "t": 2, "x": 0} {
		if got := f.Side(id); got != want {
			t.Errorf("Side(%s) = %d, want %d", id, got, want)
		}
	}
	if got := (repair.Facts{}).Side("o"); got != 0 {
		t.Errorf("empty Side = %d", got)
	}
}
