package numbering_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/numbering"
)

func row(id string, d change.Draft) change.Record {
	return change.Record{ID: change.ID(id), At: time.Unix(0, 0), By: "t", Actor: change.User("t"), T: d.T, F: d.F, To: d.To}
}

func TestNextIsMaxPlusOne(t *testing.T) {
	log := []change.Record{
		row("c", change.NewCreate(map[string]any{"id": "W"})),
		row("a", numbering.Draft(numbering.Items, 3)),
		row("b", numbering.Draft(numbering.Items, 7)),
		row("p", numbering.Draft(numbering.Projects, 2)),
		row("x", change.Draft{T: numbering.Type, F: "items", To: json.RawMessage(`"bad"`)}),
	}
	if got := numbering.Next(log, numbering.Items); got != 8 {
		t.Errorf("Next(items) = %d, want 8", got)
	}
	if got := numbering.Next(log, numbering.Projects); got != 3 {
		t.Errorf("Next(projects) = %d, want 3", got)
	}
	if got := numbering.Next(nil, numbering.Items); got != 1 {
		t.Errorf("Next(empty) = %d, want 1", got)
	}
}
