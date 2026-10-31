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

func (x ULID) String() string { return x.u.String() }

// IsZero reports whether x is the zero ULID, which no file uses.
func (x ULID) IsZero() bool { return x.u == ulid.ULID{} }

// MarshalBinary writes x as its 16 bytes, so that the index can keep it in
// a gob cache.
func (x ULID) MarshalBinary() ([]byte, error) { return x.u.MarshalBinary() }

// UnmarshalBinary reads what MarshalBinary wrote.
func (x *ULID) UnmarshalBinary(b []byte) error { return x.u.UnmarshalBinary(b) }

func (x ULID) MarshalYAML() (any, error) { return x.String(), nil }

// UnmarshalYAML reads the scalar as written, so a ULID of digits only is not
// taken for a number.
func (x *ULID) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: want a ULID", n.Line)
	}
	p, err := ParseULID(n.Value)
	if err != nil {
		return err
	}
	*x = p
	return nil
}
