package cli

import (
	"bytes"
	"testing"
	"time"
)

func TestProgress(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	var out bytes.Buffer
	p := newProgress(&out, func() time.Time { return now })
	step := func(d time.Duration, store string, done, total int) string {
		out.Reset()
		now = now.Add(d)
		p(store, done, total)
		return out.String()
	}
	if got := step(0, "me", 0, 5000); got != "" {
		t.Errorf("first call wrote %q", got)
	}
	if got := step(500*time.Millisecond, "me", 2000, 5000); got != "" {
		t.Errorf("call before a second wrote %q", got)
	}
	if got, want := step(600*time.Millisecond, "me", 3000, 5000), "Indexing the history of me: 3000/5000 commits\n"; got != want {
		t.Errorf("call after a second wrote %q, want %q", got, want)
	}
	if got := step(200*time.Millisecond, "me", 4000, 5000); got != "" {
		t.Errorf("call within a second of the last line wrote %q", got)
	}
	if got, want := step(10*time.Millisecond, "me", 5000, 5000), "Indexing the history of me: 5000/5000 commits\n"; got != want {
		t.Errorf("the end wrote %q, want %q", got, want)
	}
}

func TestProgressStartsAtTheStartOfTheWork(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	var out bytes.Buffer
	p := newProgress(&out, func() time.Time { return now })
	p("me", 0, 5000)
	now = now.Add(1100 * time.Millisecond) // the first 1000 commits took longer than a second
	p("me", 1000, 5000)
	if got, want := out.String(), "Indexing the history of me: 1000/5000 commits\n"; got != want {
		t.Errorf("wrote %q, want %q", got, want)
	}
}

func TestProgressSaysNothingForQuickWork(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	var out bytes.Buffer
	p := newProgress(&out, func() time.Time { return now })
	p("me", 0, 2000)
	now = now.Add(100 * time.Millisecond)
	p("me", 2000, 2000)
	if out.Len() != 0 {
		t.Errorf("quick work wrote %q", out.String())
	}
}

func TestProgressEndWithoutAnEarlierLineSaysNothing(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	var out bytes.Buffer
	p := newProgress(&out, func() time.Time { return now })
	p("me", 0, 1001)
	now = now.Add(2 * time.Second)
	p("me", 1001, 1001)
	if out.Len() != 0 {
		t.Errorf("wrote %q for an end that no line led up to", out.String())
	}
}
