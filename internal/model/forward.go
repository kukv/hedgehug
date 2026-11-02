package model

import (
	"errors"
	"fmt"
	"time"
)

// ErrForward means a task file is a forward: the task moved to another
// store and only its ulid, moved_to and moved_at are left.
var ErrForward = errors.New("the task moved to another store")

// Forward is a task file left behind by a move to another store.
type Forward struct {
	ULID    ULID
	MovedTo ULID      // the ulid of the store it moved to (store.yaml's ulid)
	MovedAt time.Time // when the task moved; purge --older-than goes by it

	doc *Doc
}

var forwardKeys = []string{"ulid", "moved_to", "moved_at"}

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

// DecodeForward reads a forward. ulid, moved_to and moved_at are required, and any
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
	if err := decodeAll(d, map[string]any{"ulid": &f.ULID, "moved_to": &f.MovedTo, "moved_at": &f.MovedAt}); err != nil {
		return nil, err
	}
	if f.ULID.IsZero() {
		return nil, &DecodeError{Field: "ulid", Err: errors.New("missing")}
	}
	if f.MovedTo.IsZero() {
		return nil, &DecodeError{Field: "moved_to", Err: errors.New("missing")}
	}
	if f.MovedAt.IsZero() {
		return nil, &DecodeError{Field: "moved_at", Err: errors.New("missing")}
	}
	if extra := d.Extra(forwardKeys); len(extra) > 0 {
		return nil, &DecodeError{Field: extra[0].Key, Err: fmt.Errorf("a forward holds only ulid, moved_to and moved_at")}
	}
	return f, nil
}

// Encode writes the forward: front matter with ulid, moved_to and moved_at, no body.
func (f *Forward) Encode() ([]byte, error) {
	if f.doc == nil {
		f.doc = NewDoc()
	}
	e := f.doc.Encoder()
	e.Put("ulid", f.ULID, false)
	e.Put("moved_to", f.MovedTo, false)
	e.Put("moved_at", f.MovedAt, false)
	front, err := e.Bytes(nil)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, ""), nil
}
