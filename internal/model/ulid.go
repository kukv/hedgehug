package model

import (
	"fmt"
	"strings"

	"github.com/oklog/ulid/v2"
	"go.yaml.in/yaml/v4"
)

// ULID identifies a task, comment, worklog, sprint, release or store. It is a
// ULID, written as 26 upper-case Crockford base32 characters.
type ULID struct{ u ulid.ULID }

// NewULID returns a new ULID. ULIDs made in one process increase monotonically.
func NewULID() ULID { return ULID{ulid.Make()} }

// ParseULID parses a ULID in either case.
func ParseULID(s string) (ULID, error) {
	u, err := ulid.ParseStrict(strings.ToUpper(s))
	if err != nil {
		return ULID{}, fmt.Errorf("invalid ULID %q: %w", s, err)
	}
	return ULID{u}, nil
}

func (id ULID) String() string { return id.u.String() }

// IsZero reports whether id is the zero ULID, which no file uses.
func (id ULID) IsZero() bool { return id.u == ulid.ULID{} }

// MarshalBinary writes id as its 16 bytes, so that the index can keep it in
// a gob cache.
func (id ULID) MarshalBinary() ([]byte, error) { return id.u.MarshalBinary() }

// UnmarshalBinary reads what MarshalBinary wrote.
func (id *ULID) UnmarshalBinary(b []byte) error { return id.u.UnmarshalBinary(b) }

func (id ULID) MarshalYAML() (any, error) { return id.String(), nil }

// UnmarshalYAML reads the scalar as written, so a ULID of digits only is not
// taken for a number.
func (id *ULID) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: want a ULID", n.Line)
	}
	p, err := ParseULID(n.Value)
	if err != nil {
		return err
	}
	*id = p
	return nil
}
