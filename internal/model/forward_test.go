package model

import (
	"errors"
	"strings"
	"testing"
)

const (
	fwdULID  = "01JBZ8K3QW7M2X9V4T6R5N8P0A"
	fwdStore = "01JC0M3R5W7M2X9V4T6R5N8P1B"
)

func TestDecodeForward(t *testing.T) {
	t.Parallel()
	f, err := DecodeForward([]byte("---\nulid: " + fwdULID + "\nmoved_to: " + fwdStore + "\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if f.ULID.String() != fwdULID || f.MovedTo.String() != fwdStore {
		t.Errorf("got %+v", f)
	}
}

func TestDecodeForwardErrors(t *testing.T) {
	t.Parallel()
	for name, src := range map[string]string{
		"no moved_to":   "---\nulid: " + fwdULID + "\n---\n",
		"null moved_to": "---\nulid: " + fwdULID + "\nmoved_to:\n---\n",
		"no ulid":       "---\nmoved_to: " + fwdStore + "\n---\n",
		"other key":     "---\nulid: " + fwdULID + "\nmoved_to: " + fwdStore + "\ntitle: x\n---\n",
		"bad moved_to":  "---\nulid: " + fwdULID + "\nmoved_to: nope\n---\n",
		"no front":      "moved_to: x",
	} {
		_, err := DecodeForward([]byte(src))
		var de *DecodeError
		if !errors.As(err, &de) {
			t.Errorf("%s: err = %v, want *DecodeError", name, err)
		}
	}
	_, err := DecodeForward([]byte("---\nulid: " + fwdULID + "\nmoved_to: " + fwdStore + "\ntitle: x\n---\n"))
	if err == nil || !strings.Contains(err.Error(), "title") {
		t.Errorf("err = %v, want it to name title", err)
	}
}

func TestForwardEncodeRoundTrip(t *testing.T) {
	t.Parallel()
	src := "---\nulid: " + fwdULID + "\nmoved_to: " + fwdStore + "\n---\n"
	f, err := DecodeForward([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != src {
		t.Errorf("got %q, want %q", out, src)
	}
	ulid, _ := ParseULID(fwdULID)
	store, _ := ParseULID(fwdStore)
	out, err = (&Forward{ULID: ulid, MovedTo: store}).Encode()
	if err != nil || string(out) != src {
		t.Errorf("new forward = %q, %v", out, err)
	}
}

func TestIsForward(t *testing.T) {
	t.Parallel()
	for src, want := range map[string]bool{
		"---\nulid: " + fwdULID + "\nmoved_to: " + fwdStore + "\n---\n": true,
		"---\nmoved_to:\n---\n":                       true,
		"---\nulid: " + fwdULID + "\ntitle: t\n---\n": false,
		"no front matter":                             false,
		"---\n[unclosed\n---\n":                       false,
	} {
		if got := IsForward([]byte(src)); got != want {
			t.Errorf("IsForward(%q) = %v, want %v", src, got, want)
		}
	}
}
