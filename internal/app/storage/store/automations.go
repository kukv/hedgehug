package store

import (
	"errors"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
)

// Automations reads automations.yaml of the work tree. A store without the
// file has no rules. A file that does not decode is a *FileError.
func (s *Store) Automations() (*automation.File, error) {
	f, err := readDecode(s.dir, model.AutomationsPath, automation.Decode)
	if errors.Is(err, ErrNotFound) {
		return automation.Default(), nil
	}
	return f, err
}
