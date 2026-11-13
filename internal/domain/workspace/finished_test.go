package workspace_test

import (
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestFinished(t *testing.T) {
	n := 0
	st := workspace.DefaultSettings(func() change.ID { n++; return change.ID(fmt.Sprintf("V%02d", n)) })
	// 振る順: Todo V01、InProgress V02、Done V03、Pending V04、Cancel V05。
	for id, want := range map[change.ID]bool{"V01": false, "V02": false, "V03": true, "V04": false, "V05": true, "V99": false, "": false} {
		if got := st.Finished(id); got != want {
			t.Errorf("Finished(%q) = %v, want %v", id, got, want)
		}
	}
}
