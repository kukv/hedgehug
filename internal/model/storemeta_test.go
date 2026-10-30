package model

import (
	"errors"
	"testing"
)

func TestStoreMetaRoundTrip(t *testing.T) {
	src := "id: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nname: shared\nformat: 1\naudience: backend と frontend の両チーム\n"
	m, err := DecodeStoreMeta([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "shared" || m.Format != 1 || m.Audience == "" {
		t.Errorf("decoded = %+v", m)
	}
	out, err := m.Encode()
	if err != nil || string(out) != src {
		t.Errorf("round trip = %q, %v", out, err)
	}
}

func TestStoreMetaErrors(t *testing.T) {
	_, err := DecodeStoreMeta([]byte("id: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nformat: 2\n"))
	if !errors.Is(err, ErrNewerFormat) {
		t.Errorf("format 2: err = %v, want ErrNewerFormat", err)
	}
	for _, src := range []string{
		"format: 1\n",                      // no id
		"id: 01JBZ8K3QW7M2X9V4T6R5N8P0A\n", // no format
		"id: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nformat: x\n",
	} {
		if _, err := DecodeStoreMeta([]byte(src)); err == nil {
			t.Errorf("DecodeStoreMeta(%q) succeeded", src)
		}
	}
}
