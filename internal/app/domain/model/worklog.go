package model

import (
	"errors"
	"time"
)

// Worklog is one entry of time spent on a task. The task is known from the
// file's path.
type Worklog struct {
	ULID      ULID
	Author    string
	Date      Date
	Duration  Duration
	Created   time.Time
	Updated   time.Time
	DeletedAt *time.Time
	Extra     Fields
	Body      string // a note

	doc *Doc
}

var worklogKeys = []string{"ulid", "author", "date", "duration", "created", "updated", "deleted_at"}

func DecodeWorklog(data []byte) (*Worklog, error) {
	front, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	d, err := ParseDoc(front, 1)
	if err != nil {
		return nil, err
	}
	w := &Worklog{Body: body, doc: d}
	if err := decodeAll(d, map[string]any{
		"ulid": &w.ULID, "author": &w.Author, "date": &w.Date, "duration": &w.Duration,
		"created": &w.Created, "updated": &w.Updated, "deleted_at": &w.DeletedAt,
	}); err != nil {
		return nil, err
	}
	if w.ULID.IsZero() {
		return nil, &DecodeError{Field: "ulid", Err: errors.New("missing")}
	}
	w.Extra = d.Extra(worklogKeys)
	return w, nil
}

func (w *Worklog) Encode() ([]byte, error) {
	if w.doc == nil {
		w.doc = NewDoc()
	}
	e := w.doc.Encoder()
	e.Put("ulid", w.ULID, false)
	e.Put("author", w.Author, w.Author == "")
	e.Put("date", w.Date, w.Date.IsZero())
	e.Put("duration", w.Duration, w.Duration == 0)
	e.Put("created", w.Created, w.Created.IsZero())
	e.Put("updated", w.Updated, w.Updated.IsZero())
	e.Put("deleted_at", w.DeletedAt, w.DeletedAt == nil)
	front, err := e.Bytes(w.Extra)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, w.Body), nil
}
