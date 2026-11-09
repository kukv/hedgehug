package model

import "testing"

func TestMarkedCommentAndSplit(t *testing.T) {
	u := NewULID()
	for _, body := range []string{"one", "one\ntwo", "", "a <!-- hedgehug:" + u.String() + " --> b"} {
		sent := MarkedComment(u, body+"\r\n")
		text, got, ok := SplitComment(sent)
		if !ok || got != u || text != NormalizeBody(body) {
			t.Errorf("body %q: sent %q, text %q, %s %v", body, sent, text, got, ok)
		}
	}
	// What GitHub's web editor writes back: CRLF.
	text, got, ok := SplitComment("edited\r\n\r\n<!-- hedgehug:" + u.String() + " -->\r\n")
	if !ok || got != u || text != "edited" {
		t.Errorf("CRLF: %q %s %v", text, got, ok)
	}
	for _, body := range []string{"plain", "<!-- hedgehug:" + u.String() + " --> and more", "<!-- hedgehug:nope -->", "x<!-- hedgehug:" + u.String() + " -->"} {
		if text, _, ok := SplitComment(body); ok || text != NormalizeBody(body) {
			t.Errorf("body %q: %q %v", body, text, ok)
		}
	}
}
