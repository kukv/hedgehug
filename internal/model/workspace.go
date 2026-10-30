package model

import (
	"errors"
	"time"
)

// Workspace is one workspace file. It owns a key prefix, and the tasks in
// it are numbered under that prefix. The body holds notes.
type Workspace struct {
	ID        ID
	Name      string
	KeyPrefix string
	PrefixSet time.Time // when the prefix was last set
	Created   time.Time
	Updated   time.Time
	Extra     Fields
	Body      string

	doc *Doc
}

var workspaceKeys = []string{"id", "name", "key_prefix", "prefix_set", "created", "updated"}

func DecodeWorkspace(data []byte) (*Workspace, error) {
	front, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	d, err := ParseDoc(front, 1)
	if err != nil {
		return nil, err
	}
	w := &Workspace{Body: body, doc: d}
	if err := decodeAll(d, map[string]any{
		"id": &w.ID, "name": &w.Name, "key_prefix": &w.KeyPrefix, "prefix_set": &w.PrefixSet,
		"created": &w.Created, "updated": &w.Updated,
	}); err != nil {
		return nil, err
	}
	if w.ID.IsZero() {
		return nil, &DecodeError{Field: "id", Err: errors.New("missing")}
	}
	w.Extra = d.Extra(workspaceKeys)
	return w, nil
}

func (w *Workspace) Encode() ([]byte, error) {
	if w.doc == nil {
		w.doc = NewDoc()
	}
	e := w.doc.Encoder()
	e.Put("id", w.ID, false)
	e.Put("name", w.Name, w.Name == "")
	e.Put("key_prefix", w.KeyPrefix, w.KeyPrefix == "")
	e.Put("prefix_set", w.PrefixSet, w.PrefixSet.IsZero())
	e.Put("created", w.Created, w.Created.IsZero())
	e.Put("updated", w.Updated, w.Updated.IsZero())
	front, err := e.Bytes(w.Extra)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, w.Body), nil
}
