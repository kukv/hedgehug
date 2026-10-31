package index

import (
	"slices"
	"testing"
)

func TestMentionsSkipCode(t *testing.T) {
	body := "hi @alice and @github.com/bob\n`@carol`\n```\n@dave\n```\nmail a@b.c\n"
	if got := Mentions(body); !slices.Equal(got, []string{"alice", "github.com/bob"}) {
		t.Errorf("Mentions = %v", got)
	}
}

func TestMentionsOnceInOrder(t *testing.T) {
	// A backtick that closes nothing is text.
	body := "(@bob), @alice. @bob\n~~~go\n@carol\n~~~\n@ken-1 `x` @eve `@mallory\n"
	if got := Mentions(body); !slices.Equal(got, []string{"bob", "alice", "ken-1", "eve", "mallory"}) {
		t.Errorf("Mentions = %v", got)
	}
}
