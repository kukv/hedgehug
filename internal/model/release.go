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
	ULID      ULID
	Workspace ULID // zero when not set
	Name      string
	Date      Date
	State     string
	Created   time.Time
	Updated   time.Time
	Extra     Fields
	Body      string

	doc *Doc
}

var releaseKeys = []string{"ulid", "workspace", "name", "date", "state", "created", "updated"}

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
		"ulid": &r.ULID, "workspace": &r.Workspace, "name": &r.Name, "date": &r.Date, "state": &r.State,
		"created": &r.Created, "updated": &r.Updated,
	}); err != nil {
		return nil, err
	}
	if r.ULID.IsZero() {
		return nil, &DecodeError{Field: "ulid", Err: errors.New("missing")}
	}
	r.Extra = d.Extra(releaseKeys)
	return r, nil
}

func (r *Release) Encode() ([]byte, error) {
	if r.doc == nil {
		r.doc = NewDoc()
	}
	e := r.doc.Encoder()
	e.Put("ulid", r.ULID, false)
	e.Put("workspace", r.Workspace, r.Workspace.IsZero())
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
