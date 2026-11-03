package model

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// ValidName reports whether name can name a thing of the kind what, such as
// a workspace or a store: lower-case letters, digits, - and _, starting with
// a letter or a digit.
func ValidName(what, name string) error {
	if !nameRE.MatchString(name) {
		return fmt.Errorf("%q cannot name a %s: use lower-case letters, digits, - and _, starting with a letter or a digit", name, what)
	}
	return nil
}

// ValidWorkspaceName is ValidName for a workspace.
func ValidWorkspaceName(name string) error { return ValidName("workspace", name) }

// Workspace is one workspace file. It owns an id prefix, and the tasks in
// it are numbered under that prefix. The body holds notes.
type Workspace struct {
	ULID      ULID
	Name      string
	IDPrefix  string
	PrefixSet time.Time // when the prefix was last set
	Created   time.Time
	Updated   time.Time
	Extra     Fields
	Body      string

	doc *Doc
}

var workspaceKeys = []string{"ulid", "name", "id_prefix", "prefix_set", "created", "updated"}

// DecodeWorkspace decodes a workspace file. Only the ulid is required; the
// other fields are checked by those who write the file (Store.Update) and
// by doctor, so a file with a bad prefix or name can still be read and
// fixed.
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
		"ulid": &w.ULID, "name": &w.Name, "id_prefix": &w.IDPrefix, "prefix_set": &w.PrefixSet,
		"created": &w.Created, "updated": &w.Updated,
	}); err != nil {
		return nil, err
	}
	if w.ULID.IsZero() {
		return nil, &DecodeError{Field: "ulid", Err: errors.New("missing")}
	}
	w.Extra = d.Extra(workspaceKeys)
	return w, nil
}

func (w *Workspace) Encode() ([]byte, error) {
	if w.doc == nil {
		w.doc = NewDoc()
	}
	e := w.doc.Encoder()
	e.Put("ulid", w.ULID, false)
	e.Put("name", w.Name, w.Name == "")
	e.Put("id_prefix", w.IDPrefix, w.IDPrefix == "")
	e.Put("prefix_set", w.PrefixSet, w.PrefixSet.IsZero())
	e.Put("created", w.Created, w.Created.IsZero())
	e.Put("updated", w.Updated, w.Updated.IsZero())
	front, err := e.Bytes(w.Extra)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, w.Body), nil
}
