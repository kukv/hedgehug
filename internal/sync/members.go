package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/store"
)

const membersFile = "members.json"

// Collaborator is a person GitHub lets read the store, as an account ID.
type Collaborator struct {
	ID         string `json:"id"`
	Permission string `json:"permission"`
}

// Members is who is known to use the store: the collaborators on GitHub
// (nil when they could not be read) and the writers of its commits.
type Members struct {
	Collaborators []Collaborator `json:"collaborators"`
	Authors       []string       `json:"authors"`
	Head          string         `json:"head,omitempty"` // the HEAD the authors were read up to
	Checked       time.Time      `json:"checked,omitzero"`
}

// ReadMembers reads members.json; a missing file has no one.
func ReadMembers(s *store.Store) (Members, error) {
	var m Members
	b, err := s.ReadLocal(membersFile)
	if errors.Is(err, store.ErrNotFound) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return Members{}, fmt.Errorf("%s: %w", membersFile, err)
	}
	return m, nil
}

// WriteMembers replaces members.json. A sync writes it under sync.lock, so a
// caller outside the sync holds that lock too.
func WriteMembers(s *store.Store, m Members) error {
	if m.Authors == nil {
		m.Authors = []string{}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return s.WriteLocal(membersFile, append(b, '\n'))
}

// UpdateAuthors adds the Hedgehug-Author of the commits HEAD gained since
// the last update. Account IDs are the same without case, so an author that
// differs from a known one only in case is not added again.
func UpdateAuthors(s *store.Store) error {
	m, err := ReadMembers(s)
	if err != nil {
		return err
	}
	r := s.Repo()
	head, err := r.Head()
	if err != nil || head.IsZero() {
		return err
	}
	from := plumbing.ZeroHash
	if m.Head != "" {
		from = plumbing.NewHash(m.Head)
		if ok, err := r.IsAncestor(from, head); err != nil || !ok {
			from = plumbing.ZeroHash // history changed: read it all
		}
	}
	es, err := r.Between(head, from)
	if err != nil {
		return err
	}
	for _, e := range es {
		id, ok := e.Trailer("Hedgehug-Author")
		// Account IDs are compared without case (spec: 共通の決まり, 大文字小文字); sync does not import config, so strings.EqualFold, like config.SameAccount.
		if ok && !slices.ContainsFunc(m.Authors, func(a string) bool { return strings.EqualFold(a, id) }) {
			m.Authors = append(m.Authors, id)
		}
	}
	slices.Sort(m.Authors)
	m.Head = head.String()
	return WriteMembers(s, m)
}
