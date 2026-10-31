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
	// A backtick that closes nothing is text; a login does not end with a hyphen.
	body := "(@bob), @alice. @bob\n@ken-1 `x` @eve `@mallory\n@trent-\n"
	if got := Mentions(body); !slices.Equal(got, []string{"bob", "alice", "ken-1", "eve", "mallory", "trent"}) {
		t.Errorf("Mentions = %v", got)
	}
}

func TestMentionsCode(t *testing.T) {
	for _, c := range []struct {
		name, body string
		want       []string
	}{
		{"a shorter fence does not close", "```\n@a\n``\n@b\n```\n@c\n", []string{"c"}},
		{"a longer fence closes", "```\n@a\n`````\n@c\n", []string{"c"}},
		{"the closing fence is as long", "````\n@a\n```\n@b\n````\n@c\n", []string{"c"}},
		{"the closing fence is of the same character", "~~~\n@a\n```\n@b\n~~~\n@c\n", []string{"c"}},
		{"up to three spaces open a fence", "   ```\n@a\n   ```\n@c\n", []string{"c"}},
		{"four spaces open no fence", "    ```\n@c\n", []string{"c"}},
		{"text after a closing fence does not close", "```\n@a\n``` x\n@b\n```\n@c\n", []string{"c"}},
		{"a backtick fence has no backtick after it", "``` a`b\n@c\n", []string{"c"}},
		{"an indented block after a blank line", "x\n\n    @a\n\n    @b\n@c\n", []string{"c"}},
		{"an indented block at the start", "\t@a\n@c\n", []string{"c"}},
		{"an indented line goes on a paragraph", "x\n    @c\n", []string{"c"}},
		{"double backticks", "``@a ` b`` and `@b` @c\n", []string{"c"}},
		{"runs of another length", "``x` @c\n", []string{"c"}},
	} {
		if got := Mentions(c.body); !slices.Equal(got, c.want) {
			t.Errorf("%s: Mentions = %v, want %v", c.name, got, c.want)
		}
	}
}
