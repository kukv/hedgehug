package model

import (
	"errors"
	"fmt"
)

// StoreFormat is the newest store format this version reads and writes.
const StoreFormat = 1

// ErrNewerFormat means the store was written by a newer Hedgehug.
var ErrNewerFormat = errors.New("the store uses a newer format; update Hedgehug")

// StoreMeta is .hedgehug/store.yaml.
type StoreMeta struct {
	ID       ID
	Name     string
	Format   int
	Audience string // who shares the store, shown in the GUI
	Extra    Fields

	doc *Doc
}

var storeMetaKeys = []string{"id", "name", "format", "audience"}

// DecodeStoreMeta reads store.yaml. id and format are required, and a format
// newer than StoreFormat is refused with ErrNewerFormat.
func DecodeStoreMeta(data []byte) (*StoreMeta, error) {
	d, err := ParseDoc([]byte(normalizeNewlines(string(data))), 0)
	if err != nil {
		return nil, err
	}
	m := &StoreMeta{doc: d}
	if err := decodeAll(d, map[string]any{
		"id": &m.ID, "name": &m.Name, "format": &m.Format,
		"audience": &m.Audience,
	}); err != nil {
		return nil, err
	}
	switch {
	case m.ID.IsZero():
		return nil, &DecodeError{Field: "id", Err: errors.New("missing")}
	case m.Format == 0:
		return nil, &DecodeError{Field: "format", Err: errors.New("missing")}
	case m.Format > StoreFormat:
		return nil, fmt.Errorf("format %d (this version reads up to %d): %w", m.Format, StoreFormat, ErrNewerFormat)
	}
	m.Extra = d.Extra(storeMetaKeys)
	return m, nil
}

func (m *StoreMeta) Encode() ([]byte, error) {
	if m.doc == nil {
		m.doc = NewDoc()
	}
	e := m.doc.Encoder()
	e.Put("id", m.ID, false)
	e.Put("name", m.Name, m.Name == "")
	e.Put("format", m.Format, false)
	e.Put("audience", m.Audience, m.Audience == "")
	return e.Bytes(m.Extra)
}
