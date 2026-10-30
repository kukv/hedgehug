package model

import (
	"errors"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestNewIDIsParseable(t *testing.T) {
	a, b := NewID(), NewID()
	if a.String() >= b.String() {
		t.Errorf("NewID is not increasing: %s then %s", a, b)
	}
	got, err := ParseID(a.String())
	if err != nil || got != a {
		t.Errorf("ParseID(%s) = %v, %v", a, got, err)
	}
}

func TestParseID(t *testing.T) {
	const s = "01JBZ8K3QW7M2X9V4T6R5N8P0A"
	for _, tc := range []struct {
		in      string
		want    string
		wantErr bool
	}{
		{s, s, false},
		{strings.ToLower(s), s, false},
		{"", "", true},
		{"01JBZ8K3QW7M2X9V4T6R5N8P0", "", true},  // 25 chars
		{"01JBZ8K3QW7M2X9V4T6R5N8P0U", "", true}, // U is not Crockford
		{"81JBZ8K3QW7M2X9V4T6R5N8P0A", "", true}, // overflows 128 bits
	} {
		got, err := ParseID(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseID(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got.String() != tc.want {
			t.Errorf("ParseID(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestIDYAML(t *testing.T) {
	type doc struct {
		ID ID `yaml:"id"`
	}
	var d doc
	if err := yaml.Load([]byte("id: 01JBZ8K3QW7M2X9V4T6R5N8P0A\n"), &d); err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Dump(d)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "id: 01JBZ8K3QW7M2X9V4T6R5N8P0A\n"; got != want {
		t.Errorf("Dump = %q, want %q", got, want)
	}
	if err := yaml.Load([]byte("id: nope\n"), &d); err == nil {
		t.Error("Load of a bad ID succeeded")
	}
	if !(ID{}).IsZero() || d.ID.IsZero() {
		t.Error("IsZero is wrong")
	}
}

func TestDecodeError(t *testing.T) {
	inner := errors.New("boom")
	e := &DecodeError{Field: "status", Line: 3, Err: inner}
	if got, want := e.Error(), "status (line 3): boom"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(e, inner) {
		t.Error("DecodeError does not unwrap")
	}
	if got, want := (&DecodeError{Field: "id", Err: inner}).Error(), "id: boom"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
