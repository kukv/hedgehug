package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/kukv/hedgehug/internal/store"
)

const accessFile = "access.json"

// ErrBrokenAccess is what ReadAccess wraps when access.json cannot be
// decoded. The writes refuse such a store; a sync takes the file as absent.
var ErrBrokenAccess = errors.New("access.json is broken")

// The reasons a store is read-only.
const (
	ReasonGitHub      = "github_permission"
	ReasonNotWritable = "not_writable"
	ReasonPushDenied  = "push_denied"
)

// Access is whether this machine may write to the store, as last checked.
type Access struct {
	ReadOnly bool      `json:"read_only"`
	Reason   string    `json:"reason,omitempty"`
	Checked  time.Time `json:"checked,omitzero"`
}

// Why is the reason in words.
func (a Access) Why() string {
	switch a.Reason {
	case ReasonGitHub:
		return "the GitHub permission is read"
	case ReasonNotWritable:
		return "the remote cannot be written"
	case ReasonPushDenied:
		return "the remote denied the push"
	}
	return a.Reason
}

// ReadAccess reads access.json; a missing file is a writable store.
func ReadAccess(s *store.Store) (Access, error) {
	var a Access
	b, err := s.ReadLocal(accessFile)
	if errors.Is(err, store.ErrNotFound) {
		return a, nil
	}
	if err != nil {
		return a, err
	}
	if err := json.Unmarshal(b, &a); err != nil {
		return Access{}, fmt.Errorf("%s: %w: %w", accessFile, ErrBrokenAccess, err)
	}
	return a, nil
}

// readAccessOrReset is ReadAccess for a sync, which holds sync.lock: a broken
// access.json is taken as absent and replaced by an empty one, so that the
// check that follows judges again, and the warning says so.
func readAccessOrReset(s *store.Store) (a Access, warning string, err error) {
	a, err = ReadAccess(s)
	if !errors.Is(err, ErrBrokenAccess) {
		return a, "", err
	}
	path := filepath.Join(s.Dir(), store.LocalDir, accessFile)
	if err := WriteAccess(s, Access{}); err != nil {
		return Access{}, "", err
	}
	return Access{}, path + " is broken; judging access again", nil
}

// WriteAccess replaces access.json. A sync writes it under sync.lock, so a
// caller outside the sync holds that lock too.
func WriteAccess(s *store.Store, a Access) error {
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return s.WriteLocal(accessFile, append(b, '\n'))
}
