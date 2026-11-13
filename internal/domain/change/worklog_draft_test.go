package change_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
)

func TestNewWorklogWritesNullForAddAndDelete(t *testing.T) {
	add := change.NewWorklog("W1", nil, map[string]any{"hours": 1.25})
	del := change.NewWorklog("W1", map[string]any{"hours": 1.25}, nil)
	if add.T != change.Worklog || add.F != "W1" || string(add.From) != "null" || string(add.To) != `{"hours":1.25}` {
		t.Errorf("add = %+v", add)
	}
	if !change.Absent(del.To) || change.Absent(add.To) {
		t.Errorf("delete = %+v", del)
	}
}
