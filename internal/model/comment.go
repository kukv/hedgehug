package model

import (
	"errors"
	"time"
)

// Comment is one comment on a task. The task is known from the file's path.
type Comment struct {
	ID        ID
	Author    string
	Created   time.Time
	Updated   time.Time
	DeletedAt *time.Time
	Extra     Fields
	Body      string

	doc *Doc
}

var commentKeys = []string{"id", "author", "created", "updated", "deleted_at"}

func DecodeComment(data []byte) (*Comment, error) {
	front, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	d, err := ParseDoc(front, 1)
	if err != nil {
		return nil, err
	}
	c := &Comment{Body: body, doc: d}
	if err := decodeAll(d, map[string]any{
		"id": &c.ID, "author": &c.Author, "created": &c.Created,
		"updated": &c.Updated, "deleted_at": &c.DeletedAt,
	}); err != nil {
		return nil, err
	}
	if c.ID.IsZero() {
		return nil, &DecodeError{Field: "id", Err: errors.New("missing")}
	}
	c.Extra = d.Extra(commentKeys)
	return c, nil
}

func (c *Comment) Encode() ([]byte, error) {
	if c.doc == nil {
		c.doc = NewDoc()
	}
	e := c.doc.Encoder()
	e.Put("id", c.ID, false)
	e.Put("author", c.Author, c.Author == "")
	e.Put("created", c.Created, c.Created.IsZero())
	e.Put("updated", c.Updated, c.Updated.IsZero())
	e.Put("deleted_at", c.DeletedAt, c.DeletedAt == nil)
	front, err := e.Bytes(c.Extra)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, c.Body), nil
}

// decodeAll decodes each key into its target. Map order does not matter:
// the first error found is returned.
func decodeAll(d *Doc, targets map[string]any) error {
	for key, out := range targets {
		if err := d.Decode(key, out); err != nil {
			return err
		}
	}
	return nil
}
