package index

import (
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
)

func TestTaskRefs(t *testing.T) {
	t.Parallel()
	if got := TaskRefs("see #WEB-42, #api-3 and #12\n"); !slices.Equal(got, []string{"WEB-42"}) {
		t.Errorf("TaskRefs = %v", got)
	}
}

func TestTaskRefsOnceInOrderOutsideCode(t *testing.T) {
	t.Parallel()
	body := "(#B-1) #AB-2 #AB-1 #AB-2\nx#AB-3 #AB-4x #AB-05 `#AB-6`\n```\n#AB-7\n```\n#AB-8.\n"
	if got := TaskRefs(body); !slices.Equal(got, []string{"AB-2", "AB-1", "AB-8"}) {
		t.Errorf("TaskRefs = %v", got)
	}
}

func TestMentionNames(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		mention, account string
		want             bool
	}{
		{"alice", "github.com/alice", true},
		{"Alice", "github.com/ALICE", true},
		{"alice", "gitlab.com/alice", true},
		{"alice", "ALICE", true},
		{"alice", "github.com/malice", false},
		{"github.com/alice", "github.com/Alice", true},
		{"github.com/alice", "gitlab.com/alice", false},
		{"github.com/alice", "alice", true},
		{"github.com/ALICE", "Alice", true},
		{"github.com/malice", "alice", false},
	} {
		if got := MentionNames(c.mention, c.account); got != c.want {
			t.Errorf("MentionNames(%q, %q) = %v, want %v", c.mention, c.account, got, c.want)
		}
	}
}

// A reference by a former id points at the task that has it, since no task
// has it as its id; the task itself and deleted tasks are no backlinks.
func TestBacklinksByFormerID(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t)
	a := newTask("a")
	put(t, s, a) // TST-1
	ix := mustOpen(t, s)
	b, gone := newTask("b"), newTask("gone")
	b.Body = "after #" + a.ID + "\n"
	gone.Body = "#" + a.ID + "\n"
	put(t, s, b, gone)
	at := time.Now()
	gone.DeletedAt = &at
	put(t, s, gone)
	a.Body = "#" + a.ID + "\n"
	renumber(t, s, a, "NEW")
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}
	if e, _ := ix.Task(a.ULID); e.ID != "NEW-1" || !slices.Equal(e.FormerIDs, []string{"TST-1"}) {
		t.Fatalf("a = %s %v, want NEW-1 with the former id TST-1", e.ID, e.FormerIDs)
	}
	if got := titles(ix.Backlinks(a.ULID)); !slices.Equal(got, []string{"b"}) {
		t.Errorf("Backlinks = %v, want [b]", got)
	}
}

// A reference goes to the task with that id before one with it as a former
// id, and to none when two tasks have it; the backlinks are in ULID order.
func TestBacklinksResolveOrder(t *testing.T) {
	t.Parallel()
	s, dir := newStore(t)
	former, current := model.NewULID(), model.NewULID()
	commitRaw(t, s, dir, model.TaskPath(former), rawTask(former, "kind: task\nid: TST-9\nformer_ids: [TST-1]\ntitle: former\nstatus: todo\n"))
	commitRaw(t, s, dir, model.TaskPath(current), rawTask(current, "kind: task\nid: TST-1\ntitle: current\nstatus: todo\n"))
	twin1, twin2 := model.NewULID(), model.NewULID()
	commitRaw(t, s, dir, model.TaskPath(twin1), rawTask(twin1, "kind: task\nid: TST-5\ntitle: twin\nstatus: todo\n"))
	commitRaw(t, s, dir, model.TaskPath(twin2), rawTask(twin2, "kind: task\nid: TST-5\ntitle: twin\nstatus: todo\n"))
	x, y := model.NewULID(), model.NewULID()
	// y is written first, so that the ULID order is not the order of writing.
	commitRaw(t, s, dir, model.TaskPath(y), rawTask(y, "kind: task\ntitle: y\nstatus: todo\n")+"#TST-1 #TST-5\n")
	commitRaw(t, s, dir, model.TaskPath(x), rawTask(x, "kind: task\ntitle: x\nstatus: todo\n")+"#TST-1\n")
	ix := mustOpen(t, s)
	if got := titles(ix.Backlinks(current)); !slices.Equal(got, []string{"x", "y"}) {
		t.Errorf("Backlinks(current) = %v, want [x y]", got)
	}
	if got := ix.Backlinks(former); len(got) != 0 {
		t.Errorf("Backlinks(former) = %v, want none", titles(got))
	}
	if got := ix.Backlinks(twin1); len(got) != 0 {
		t.Errorf("Backlinks(twin1) = %v, want none: TST-5 is ambiguous", titles(got))
	}
}

func TestMentionsSkipCode(t *testing.T) {
	t.Parallel()
	body := "hi @alice and @github.com/bob\n`@carol`\n```\n@dave\n```\nmail a@b.c\n"
	if got := Mentions(body); !slices.Equal(got, []string{"alice", "github.com/bob"}) {
		t.Errorf("Mentions = %v", got)
	}
}

func TestMentionsOnceInOrder(t *testing.T) {
	t.Parallel()
	// A backtick that closes nothing is text; a login does not end with a hyphen.
	body := "(@bob), @alice. @bob\n@ken-1 `x` @eve `@mallory\n@trent-\n"
	if got := Mentions(body); !slices.Equal(got, []string{"bob", "alice", "ken-1", "eve", "mallory", "trent"}) {
		t.Errorf("Mentions = %v", got)
	}
}

func TestMentionsCode(t *testing.T) {
	t.Parallel()
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
		{"a list item goes on after a blank line", "- item\n\n    @c goes on\n", []string{"c"}},
		{"an ordered list item goes on", "1. item\n\n   @c\n", []string{"c"}},
		{"code in a list item", "- item\n\n      @a\n@c\n", []string{"c"}},
		{"a nested list item", "- a\n  - @b\n\n      @c\n\n          @a\n", []string{"b", "c"}},
		{"a line left of the item ends the list", "- item\n\n@c\n\n    @a\n", []string{"c"}},
		{"items of one list", "- @b\n- @c\n", []string{"b", "c"}},
		{"a heading ends the list", "- a\n# H\n\n    @a\n", nil},
		{"a thematic break ends the list", "- a\n***\n\n    @a\n", nil},
		{"a fence ends the list", "- a\n```\n@a\n```\n\n    @b\n", nil},
		{"a lazy line goes on the item", "- a\nlazy @c\n\n    @b\n", []string{"c", "b"}},
		{"after an ATX heading", "# Title\n    @a\n## @c\n", []string{"c"}},
		{"no heading without a space", "#Title\n    @c\n", []string{"c"}},
	} {
		if got := Mentions(c.body); !slices.Equal(got, c.want) {
			t.Errorf("%s: Mentions = %v, want %v", c.name, got, c.want)
		}
	}
}
