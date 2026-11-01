package model

import (
	"errors"
	"fmt"
)

// ErrForward means a task file is a forward: the task moved to another
// store and only its ulid and moved_to are left.
var ErrForward = errors.New("the task moved to another store")

// Forward is a task file left behind by a move to another store.
type Forward struct {
	ULID    ULID
	MovedTo ULID // the ulid of the store it moved to (store.yaml's ulid)

	doc *Doc
}

var forwardKeys = []string{"ulid", "moved_to"}

// IsForward reports whether a task file is a forward: its front matter has
// the key moved_to. It does not check the rest.
func IsForward(data []byte) bool {
	front, _, err := splitFrontmatter(data)
	if err != nil {
		return false
	}
	d, err := ParseDoc(front, 1)
	if err != nil {
		return false
	}
	return d.Get("moved_to") != nil
}

// DecodeForward reads a forward. ulid and moved_to are required, and any
// other key makes it broken (a *DecodeError naming the key).
func DecodeForward(data []byte) (*Forward, error) {
	front, _, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	d, err := ParseDoc(front, 1)
	if err != nil {
		return nil, err
	}
	f := &Forward{doc: d}
	if err := decodeAll(d, map[string]any{"ulid": &f.ULID, "moved_to": &f.MovedTo}); err != nil {
		return nil, err
	}
	if f.ULID.IsZero() {
		return nil, &DecodeError{Field: "ulid", Err: errors.New("missing")}
	}
	if f.MovedTo.IsZero() {
		return nil, &DecodeError{Field: "moved_to", Err: errors.New("missing")}
	}
	if extra := d.Extra(forwardKeys); len(extra) > 0 {
		return nil, &DecodeError{Field: extra[0].Key, Err: fmt.Errorf("a forward holds only ulid and moved_to")}
	}
	return f, nil
}

// Encode writes the forward: front matter with ulid and moved_to, no body.
func (f *Forward) Encode() ([]byte, error) {
	if f.doc == nil {
		f.doc = NewDoc()
	}
	e := f.doc.Encoder()
	e.Put("ulid", f.ULID, false)
	e.Put("moved_to", f.MovedTo, false)
	front, err := e.Bytes(nil)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, ""), nil
}
