package model

import (
	"errors"
	"fmt"
)

// StoreFormat is the newest store format this version reads and writes. It is
// a variable only so that the tests of hh store convert can stand in a newer
// format; nothing else assigns it.
var StoreFormat = 4

// ErrNewerFormat means the store was written by a newer Hedgehug.
var ErrNewerFormat = errors.New("the store uses a newer format; update Hedgehug")

// ErrOldFormat means the store was written by an older Hedgehug and has to be
// converted (hh store convert) before it opens.
var ErrOldFormat = errors.New("hh store convert で新しい形式にしてください")

// StoreMeta is .hedgehug/store.yaml.
type StoreMeta struct {
	ULID     ULID
	Name     string
	Format   int
	Audience string // who shares the store, shown in the GUI
	Extra    Fields

	doc *Doc
}

var storeMetaKeys = []string{"ulid", "name", "format", "audience"}

// DecodeStoreMeta reads store.yaml. ulid and format are required, and a format
// newer than StoreFormat is refused with ErrNewerFormat. An older format
// decodes: whoever opens the store decides what to do with it (see
// OldFormat).
func DecodeStoreMeta(data []byte) (*StoreMeta, error) {
	d, err := ParseDoc([]byte(normalizeNewlines(string(data))), 0)
	if err != nil {
		return nil, err
	}
	m := &StoreMeta{doc: d}
	if err := decodeAll(d, map[string]any{
		"ulid": &m.ULID, "name": &m.Name, "format": &m.Format,
		"audience": &m.Audience,
	}); err != nil {
		return nil, err
	}
	switch {
	case m.ULID.IsZero():
		return nil, &DecodeError{Field: "ulid", Err: errors.New("missing")}
	case m.Format == 0:
		return nil, &DecodeError{Field: "format", Err: errors.New("missing")}
	case m.Format > StoreFormat:
		return nil, fmt.Errorf("format %d (this version reads up to %d): %w", m.Format, StoreFormat, ErrNewerFormat)
	}
	m.Extra = d.Extra(storeMetaKeys)
	return m, nil
}

// OldFormat reports whether the store is in a format older than StoreFormat.
func (m *StoreMeta) OldFormat() bool { return m.Format < StoreFormat }

func (m *StoreMeta) Encode() ([]byte, error) {
	if m.doc == nil {
		m.doc = NewDoc()
	}
	e := m.doc.Encoder()
	e.Put("ulid", m.ULID, false)
	e.Put("name", m.Name, m.Name == "")
	e.Put("format", m.Format, false)
	e.Put("audience", m.Audience, m.Audience == "")
	return e.Bytes(m.Extra)
}
