package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kukv/hedgehug/internal/store"
)

// stateFile is where .hedgehug/local keeps the sync state of this machine.
const stateFile = "sync.json"

// State is how the syncs of the store went on this machine.
type State struct {
	PendingSince  time.Time `json:"pending_since,omitzero"`
	LastAttempt   time.Time `json:"last_attempt,omitzero"`
	LastSuccess   time.Time `json:"last_success,omitzero"`
	LastError     string    `json:"last_error,omitempty"`
	ErrorReported bool      `json:"error_reported,omitempty"`
}

// UpdateState reads sync.json, lets fn change it and writes it back, under
// .hedgehug/local/state.lock.
func UpdateState(s *store.Store, fn func(*State)) (State, error) {
	var st State
	err := s.WithLocalLock("state.lock", 10*time.Second, func() error {
		var err error
		if st, err = ReadState(s); err != nil {
			return err
		}
		fn(&st)
		b, err := json.Marshal(st)
		if err != nil {
			return err
		}
		return s.WriteLocal(stateFile, append(b, '\n'))
	})
	return st, err
}

// ReadState reads sync.json; a missing file is the zero State.
func ReadState(s *store.Store) (State, error) {
	var st State
	b, err := s.ReadLocal(stateFile)
	if errors.Is(err, store.ErrNotFound) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return State{}, fmt.Errorf("%s: %w", stateFile, err)
	}
	return st, nil
}
