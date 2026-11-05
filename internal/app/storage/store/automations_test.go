package store

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

func TestAutomations(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	f, err := s.Automations()
	if err != nil || len(f.Rules) != 0 || f.IssueFields.DefaultSync != "link" {
		t.Fatalf("no file: %+v, %v", f, err)
	}
	mustWrite(t, s, model.AutomationsPath, "rules:\n- {id: a, on: pr.opened, set: {status: in_progress}}\n")
	if f, err = s.Automations(); err != nil || len(f.Rules) != 1 || f.Rules[0].ID != "a" {
		t.Fatalf("file: %+v, %v", f, err)
	}
	mustWrite(t, s, model.AutomationsPath, "rules: [\n")
	var fe *FileError
	if _, err = s.Automations(); !errors.As(err, &fe) || fe.Path != model.AutomationsPath {
		t.Errorf("err = %v", err)
	}
}
