package model

import (
	"fmt"
	"strings"

	"github.com/oklog/ulid/v2"
	"go.yaml.in/yaml/v4"
)

// ID identifies a task, comment, worklog, sprint, release or store. It is a
// ULID, written as 26 upper-case Crockford base32 characters.
type ID struct{ u ulid.ULID }

// NewID returns a new ID. IDs made in one process increase monotonically.
func NewID() ID { return ID{ulid.Make()} }

// ParseID parses an ID in either case.
func ParseID(s string) (ID, error) {
	u, err := ulid.ParseStrict(strings.ToUpper(s))
	if err != nil {
		return ID{}, fmt.Errorf("invalid ID %q: %w", s, err)
	}
	return ID{u}, nil
}

func (id ID) String() string { return id.u.String() }

// IsZero reports whether id is the zero ID, which no file uses.
func (id ID) IsZero() bool { return id.u == ulid.ULID{} }

// MarshalBinary writes id as its 16 bytes, so that the index can keep it in
// a gob cache.
func (id ID) MarshalBinary() ([]byte, error) { return id.u.MarshalBinary() }

// UnmarshalBinary reads what MarshalBinary wrote.
func (id *ID) UnmarshalBinary(b []byte) error { return id.u.UnmarshalBinary(b) }

func (id ID) MarshalYAML() (any, error) { return id.String(), nil }

// UnmarshalYAML reads the scalar as written, so an ID of digits only is not
// taken for a number.
func (id *ID) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: want an ID", n.Line)
	}
	p, err := ParseID(n.Value)
	if err != nil {
		return err
	}
	*id = p
	return nil
}
