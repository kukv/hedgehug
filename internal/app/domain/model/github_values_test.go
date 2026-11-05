package model

import (
	"reflect"
	"strings"
	"testing"
)

func TestIssueValuesRoundTrip(t *testing.T) {
	in := IssueValues{"title": {"a: b"}, "status": {"open"}, "body": {"one\ntwo"}, "assignees": {"alice", "bob"}, "labels": {}, "milestone": {""}}
	var x IssueState
	if err := x.SetValues(in); err != nil {
		t.Fatal(err)
	}
	got, err := x.Values()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Errorf("got %#v", got)
	}
	if err := x.SetValues(nil); err != nil || x.Synced != nil {
		t.Errorf("no values: %v, %v", err, x.Synced)
	}
}

func TestIssueValuesInTheFile(t *testing.T) {
	task := &Task{ULID: NewULID(), Title: "x", Status: "todo"}
	g := GitHubState{Issues: []IssueState{{Ref: "github.com/a/b/issues/1", State: "open"}}}
	want := IssueValues{"title": {"a"}, "status": {"open"}, "labels": {"x", "y"}}
	if err := g.Issues[0].SetValues(want); err != nil {
		t.Fatal(err)
	}
	if err := task.SetGitHub(g); err != nil {
		t.Fatal(err)
	}
	data, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "synced: {title: a, status: open, labels: [x, y]}") {
		t.Errorf("file:\n%s", data)
	}
	back, err := DecodeTask(data)
	if err != nil {
		t.Fatal(err)
	}
	read, err := back.GitHub()
	if err != nil {
		t.Fatal(err)
	}
	if v, err := read.Issues[0].Values(); err != nil || !reflect.DeepEqual(v, want) {
		t.Errorf("values %#v, %v", v, err)
	}
	// A value that changes in a file that holds one stays in its place.
	if err := read.Issues[0].SetValues(IssueValues{"title": {"b"}}); err != nil {
		t.Fatal(err)
	}
	if err := back.SetGitHub(read); err != nil {
		t.Fatal(err)
	}
	if data, err = back.Encode(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "synced: {title: b}") {
		t.Errorf("file after the change:\n%s", data)
	}
}

func TestStripMark(t *testing.T) {
	u := "hedgehug:01JBZ7XXH2M4Q8R1T5V9W3Y6ZA"
	for _, c := range []struct{ in, want string }{
		{"", ""},
		{u, ""},
		{"text\n\n" + u, "text"},
		{"text\r\n\r\n" + u + "\r\n", "text"},
		{"a\r\nb  \n", "a\nb"},
		{u + "\ntext", u + "\ntext"},
	} {
		if got := StripMark(c.in); got != c.want {
			t.Errorf("StripMark(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMarkedBody(t *testing.T) {
	u := NewULID()
	if got, want := MarkedBody(u, "text\n"), "text\n\nhedgehug:"+u.String(); got != want {
		t.Errorf("got %q", got)
	}
	if got, want := MarkedBody(u, " \n"), "hedgehug:"+u.String(); got != want {
		t.Errorf("got %q", got)
	}
	if got := StripMark(MarkedBody(u, "text")); got != "text" {
		t.Errorf("back: %q", got)
	}
}
