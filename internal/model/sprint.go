package model

import (
	"errors"
	"time"
)

// Sprint states.
const (
	SprintPlanned = "planned"
	SprintActive  = "active"
	SprintClosed  = "closed"
)

// Sprint is one sprint file. The body holds notes such as a retrospective.
type Sprint struct {
	ULID    ULID
	Name    string
	Start   Date
	End     Date
	State   string
	Goal    string
	Created time.Time
	Updated time.Time
	Extra   Fields
	Body    string

	doc *Doc
}

var sprintKeys = []string{"ulid", "name", "start", "end", "state", "goal", "created", "updated"}

func DecodeSprint(data []byte) (*Sprint, error) {
	front, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	d, err := ParseDoc(front, 1)
	if err != nil {
		return nil, err
	}
	s := &Sprint{Body: body, doc: d}
	if err := decodeAll(d, map[string]any{
		"ulid": &s.ULID, "name": &s.Name, "start": &s.Start, "end": &s.End, "state": &s.State,
		"goal": &s.Goal, "created": &s.Created, "updated": &s.Updated,
	}); err != nil {
		return nil, err
	}
	if s.ULID.IsZero() {
		return nil, &DecodeError{Field: "ulid", Err: errors.New("missing")}
	}
	s.Extra = d.Extra(sprintKeys)
	return s, nil
}

func (s *Sprint) Encode() ([]byte, error) {
	if s.doc == nil {
		s.doc = NewDoc()
	}
	e := s.doc.Encoder()
	e.Put("ulid", s.ULID, false)
	e.Put("name", s.Name, s.Name == "")
	e.Put("start", s.Start, s.Start.IsZero())
	e.Put("end", s.End, s.End.IsZero())
	e.Put("state", s.State, s.State == "")
	e.Put("goal", s.Goal, s.Goal == "")
	e.Put("created", s.Created, s.Created.IsZero())
	e.Put("updated", s.Updated, s.Updated.IsZero())
	front, err := e.Bytes(s.Extra)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, s.Body), nil
}
