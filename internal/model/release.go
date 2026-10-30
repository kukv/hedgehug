package model

import (
	"errors"
	"time"
)

// Release states.
const (
	ReleaseUnreleased = "unreleased"
	ReleaseReleased   = "released"
	ReleaseArchived   = "archived"
)

// Release is one release file. Date is the planned date.
type Release struct {
	ID      ID
	Name    string
	Date    Date
	State   string
	Created time.Time
	Updated time.Time
	Extra   Fields
	Body    string

	doc *Doc
}

var releaseKeys = []string{"id", "name", "date", "state", "created", "updated"}

func DecodeRelease(data []byte) (*Release, error) {
	front, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	d, err := ParseDoc(front, 1)
	if err != nil {
		return nil, err
	}
	r := &Release{Body: body, doc: d}
	if err := decodeAll(d, map[string]any{
		"id": &r.ID, "name": &r.Name, "date": &r.Date, "state": &r.State,
		"created": &r.Created, "updated": &r.Updated,
	}); err != nil {
		return nil, err
	}
	if r.ID.IsZero() {
		return nil, &DecodeError{Field: "id", Err: errors.New("missing")}
	}
	r.Extra = d.Extra(releaseKeys)
	return r, nil
}

func (r *Release) Encode() ([]byte, error) {
	if r.doc == nil {
		r.doc = NewDoc()
	}
	e := r.doc.Encoder()
	e.Put("id", r.ID, false)
	e.Put("name", r.Name, r.Name == "")
	e.Put("date", r.Date, r.Date.IsZero())
	e.Put("state", r.State, r.State == "")
	e.Put("created", r.Created, r.Created.IsZero())
	e.Put("updated", r.Updated, r.Updated.IsZero())
	front, err := e.Bytes(r.Extra)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, r.Body), nil
}
