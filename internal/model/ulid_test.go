package model

import (
	"bytes"
	"encoding/gob"
	"errors"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestNewULIDIsParseable(t *testing.T) {
	a, b := NewULID(), NewULID()
	if a.String() >= b.String() {
		t.Errorf("NewULIDis not increasing: %s then %s", a, b)
	}
	got, err := ParseULID(a.String())
	if err != nil || got != a {
		t.Errorf("ParseULID(%s) = %v, %v", a, got, err)
	}
}

func TestULIDGob(t *testing.T) {
	type holder struct {
		ULID   ULID
		Parent *ULID
		List   []ULID
		Zero   ULID
	}
	a, b := NewULID(), NewULID()
	in := holder{ULID: a, Parent: &b, List: []ULID{b, a}}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(in); err != nil {
		t.Fatal(err)
	}
	var out holder
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.ULID != a || out.Parent == nil || *out.Parent != b || len(out.List) != 2 || out.List[0] != b || out.List[1] != a || !out.Zero.IsZero() {
		t.Errorf("round trip = %+v, want %+v", out, in)
	}
}

func TestParseULID(t *testing.T) {
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
		got, err := ParseULID(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseULID(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got.String() != tc.want {
			t.Errorf("ParseULID(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestULIDYAML(t *testing.T) {
	type doc struct {
		ULID ULID `yaml:"ulid"`
	}
	var d doc
	if err := yaml.Load([]byte("ulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\n"), &d); err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Dump(d)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "ulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\n"; got != want {
		t.Errorf("Dump = %q, want %q", got, want)
	}
	if err := yaml.Load([]byte("ulid: nope\n"), &d); err == nil {
		t.Error("Load of a bad ULID succeeded")
	}
	if !(ULID{}).IsZero() || d.ULID.IsZero() {
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
	if got, want := (&DecodeError{Field: "ulid", Err: inner}).Error(), "ulid: boom"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
