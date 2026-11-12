package workspaces_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
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
