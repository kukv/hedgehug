package workspaces_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
)

func TestGet(t *testing.T) {
	s := setup(t)
	a, _ := s.Create(t.Context(), store, "Web", "HH")
	got, err := s.Get(t.Context(), store, a.ID)
	if err != nil || got.Workspace != a || len(got.Settings.Priorities) != 5 || len(got.OldPrefixes) != 0 {
		t.Errorf("Get = %+v, %v", got, err)
	}
	if _, err := s.Get(t.Context(), store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

// 詳細は、プロジェクト別のルールを載せる(B5a-18)。
func TestGetIncludesProjectRules(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	p := newProject(s, w.ID)
	putItemIn(t, s, p, 1)
	pending := st.Statuses[3].ID
	writeWorkspace(t, s, w.ID, change.NewAdd(projectrule.HiddenField(p.ID), pending))
	got, err := s.Get(t.Context(), store, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if h := got.ProjectRules.Of(p.ID).Hidden; len(h) != 1 || h[0] != pending {
		t.Errorf("ProjectRules = %+v", got.ProjectRules)
	}
}
