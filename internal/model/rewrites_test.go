package model

import (
	"testing"
	"time"
)

func TestRewritesRoundTrip(t *testing.T) {
	t.Parallel()
	rs := []Rewrite{
		{At: time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC), Before: "abc123", ULIDs: []ULID{NewULID(), NewULID()}},
		{At: time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC), Before: "def456", ULIDs: []ULID{NewULID()}},
	}
	data, err := EncodeRewrites(rs)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeRewrites(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].At.Equal(rs[0].At) || got[1].Before != "def456" || got[0].ULIDs[1] != rs[0].ULIDs[1] {
		t.Errorf("got %+v\nfrom %s", got, data)
	}
}

func TestDecodeRewritesEmptyAndBroken(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "# only a comment\n", "[]\n"} {
		rs, err := DecodeRewrites([]byte(in))
		if err != nil || len(rs) != 0 {
			t.Errorf("%q: %v, %v", in, rs, err)
		}
	}
	for _, in := range []string{"- at: 2026-11-02T09:00:00Z\n", "- before: abc\n", "not: a list\n"} {
		if _, err := DecodeRewrites([]byte(in)); err == nil {
			t.Errorf("%q decoded", in)
		}
	}
}
