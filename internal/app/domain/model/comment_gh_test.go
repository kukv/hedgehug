package model

import (
	"slices"
	"testing"
)

const commentGHFront = `---
ulid: 01JBZ8K3QW7M2X9V4T6R5N8P0B
author: github.com/yourname
created: 2026-10-29T10:00:00+09:00
gh:
- ref: github.com/o/r/issues/3
  id: 1001
  body_synced: "hello\n"
  extra: [1.0, 2] # unknown
- ref: github.com/o/r/issues/4
  id: 1002
  body_synced: x
---
hello
`

func TestCommentGitHubRoundTrip(t *testing.T) {
	t.Parallel()
	c, err := DecodeComment([]byte(commentGHFront))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Extra) != 0 {
		t.Errorf("gh is an Extra key: %+v", c.Extra)
	}
	l, err := c.GitHub()
	if err != nil || len(l) != 2 || l[0] != (CommentLink{Ref: "github.com/o/r/issues/3", ID: 1001, BodySynced: "hello\n"}) || l[1].ID != 1002 {
		t.Fatalf("GitHub = %+v, %v", l, err)
	}
	// Unchanged: the bytes stay.
	if err := c.SetGitHub(l); err != nil {
		t.Fatal(err)
	}
	out, err := c.Encode()
	if err != nil || string(out) != commentGHFront {
		t.Errorf("round trip = %q, %v", out, err)
	}
	// A new id on one link: the unknown key of the item stays.
	l[0].ID = 2001
	if err := c.SetGitHub(l); err != nil {
		t.Fatal(err)
	}
	out, _ = c.Encode()
	for _, want := range []string{"id: 2001", "extra: [1.0, 2]", "id: 1002"} {
		if !contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	// No links: gh goes.
	if err := c.SetGitHub(nil); err != nil {
		t.Fatal(err)
	}
	out, _ = c.Encode()
	if contains(out, "gh") {
		t.Errorf("gh stays:\n%s", out)
	}
}

func TestCommentGitHubBodyEditKeepsGH(t *testing.T) {
	t.Parallel()
	c, err := DecodeComment([]byte(commentGHFront))
	if err != nil {
		t.Fatal(err)
	}
	c.Body = "edited\n"
	out, _ := c.Encode()
	if !contains(out, "extra: [1.0, 2]") || !contains(out, "edited") {
		t.Errorf("edit lost gh:\n%s", out)
	}
}

// An older hedgehug does not know gh: it holds the key as an extra one and
// writes it back.
func TestCommentGHKeptByAnOlderReader(t *testing.T) {
	t.Parallel()
	c, err := DecodeComment([]byte(commentGHFront))
	if err != nil {
		t.Fatal(err)
	}
	old := slices.DeleteFunc(slices.Clone(commentKeys), func(k string) bool { return k == "gh" })
	extra := c.doc.Extra(old)
	if len(extra) != 1 || extra[0].Key != "gh" {
		t.Fatalf("an older reader's extra = %+v", extra)
	}
	// What the older hedgehug writes: its known keys, its body, and the extra
	// keys as it read them.
	older := &Comment{ULID: c.ULID, Author: c.Author, Created: c.Created, Body: "edited by old\n", Extra: extra}
	out, err := older.Encode()
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeComment(out)
	if err != nil {
		t.Fatal(err)
	}
	l, err := back.GitHub()
	if err != nil || len(l) != 2 || l[1].ID != 1002 || back.Body != "edited by old\n" {
		t.Errorf("after the older writer: %+v, %v, %q", l, err, back.Body)
	}
}

func TestCommentGitHubWrongShape(t *testing.T) {
	t.Parallel()
	c, err := DecodeComment([]byte("---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0B\ngh: text\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.GitHub(); err == nil {
		t.Error("a scalar reads")
	}
	n, _ := DecodeComment([]byte("---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0B\n---\n"))
	if l, err := n.GitHub(); err != nil || l != nil {
		t.Errorf("none = %+v, %v", l, err)
	}
}

func TestCommentMirrored(t *testing.T) {
	t.Parallel()
	for author, want := range map[string]bool{"github:octocat": true, "github:dependabot[bot]": true, "github.com/octocat": false, "": false} {
		if got := (&Comment{Author: author}).Mirrored(); got != want {
			t.Errorf("Mirrored(%q) = %v", author, got)
		}
	}
}

func TestNormalizeBody(t *testing.T) {
	t.Parallel()
	if got := NormalizeBody("a\r\nb \t\r\n\n"); got != "a\nb" {
		t.Errorf("NormalizeBody = %q", got)
	}
	// The mark stays: only StripMark takes it off.
	if got := NormalizeBody("a\n\nhedgehug:01JBZ8K3QW7M2X9V4T6R5N8P0B\n"); got != "a\n\nhedgehug:01JBZ8K3QW7M2X9V4T6R5N8P0B" {
		t.Errorf("NormalizeBody kept = %q", got)
	}
}
