package model

import (
	"strings"
	"testing"
	"time"
)

func TestCommentRoundTrip(t *testing.T) {
	t.Parallel()
	src := readFile(t, "testdata/comment/basic.md")
	c, err := DecodeComment(src)
	if err != nil {
		t.Fatal(err)
	}
	if c.Author != "github.com/yourname" || c.Body != "@teammate レビューお願いします\n" || c.DeletedAt != nil {
		t.Errorf("decoded = %+v", c)
	}
	out, err := c.Encode()
	if err != nil || string(out) != string(src) {
		t.Errorf("round trip = %q, %v", out, err)
	}
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.FixedZone("", 9*3600))
	c.DeletedAt = &now
	out, _ = c.Encode()
	if want := "updated: 2026-10-29T10:05:00+09:00\ndeleted_at: 2026-10-30T09:00:00+09:00\n---\n"; !contains(out, want) {
		t.Errorf("deleted_at not written after updated:\n%s", out)
	}
}

func TestWorklogRoundTrip(t *testing.T) {
	t.Parallel()
	src := readFile(t, "testdata/worklog/basic.md")
	w, err := DecodeWorklog(src)
	if err != nil {
		t.Fatal(err)
	}
	if w.Date.String() != "2026-10-29" || w.Duration.String() != "1h30m" || w.Body != "ログイン API の調査\n" {
		t.Errorf("decoded = %+v", w)
	}
	out, err := w.Encode()
	if err != nil || string(out) != string(src) {
		t.Errorf("round trip = %q, %v", out, err)
	}
	w.Duration, _ = ParseDuration("120m")
	out, _ = w.Encode()
	if !contains(out, "duration: 2h\n") {
		t.Errorf("duration not normalized:\n%s", out)
	}
}

func TestWorklogWithoutDuration(t *testing.T) {
	t.Parallel()
	src := "---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0C\nauthor: x\n---\n"
	w, err := DecodeWorklog([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := w.Encode()
	if err != nil || string(out) != src {
		t.Errorf("round trip = %q, %v; want %q", out, err, src)
	}
}

func TestCommentAndWorklogNeedULID(t *testing.T) {
	t.Parallel()
	if _, err := DecodeComment([]byte("---\nauthor: x\n---\n")); err == nil {
		t.Error("DecodeComment without ulid succeeded")
	}
	if _, err := DecodeWorklog([]byte("---\nduration: 1d\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0C\n---\n")); err == nil {
		t.Error("DecodeWorklog accepted 1d")
	}
}

// contains is shared by the tests of this package.
func contains(b []byte, s string) bool { return strings.Contains(string(b), s) }
