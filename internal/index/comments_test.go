package index

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func newComment(author, body string) *model.Comment {
	now := time.Now().Truncate(time.Second)
	return &model.Comment{ULID: model.NewULID(), Author: author, Created: now, Updated: now, Body: body}
}

func commentULIDs(cs []CommentEntry) []model.ULID {
	out := make([]model.ULID, len(cs))
	for i, c := range cs {
		out[i] = c.ULID
	}
	return out
}

func TestCommentsAndMentions(t *testing.T) {
	s, _ := newStore(t)
	a := newTask("a")
	a.Body = "for @alice, see #TST-9\n"
	a.Watchers = []string{"github.com/w"}
	put(t, s, a)
	ix := mustOpen(t, s)

	live := newComment("github.com/x", "@bob and @alice, after #TST-8\n")
	gone := newComment("github.com/y", "@carol #TST-7\n")
	at := time.Now().Truncate(time.Second)
	gone.DeletedAt = &at
	putComment(t, s, a.ULID, gone, live)
	now := time.Now().Truncate(time.Second)
	w := &model.Worklog{ULID: model.NewULID(), Author: "github.com/x", Duration: model.Duration(time.Hour), Created: now, Updated: now}
	if _, err := s.Update(store.Op{Name: "log", Task: a.ULID, Author: testAuthor}, func(tx *store.Tx) error { return tx.PutWorklog(a.ULID, w) }); err != nil {
		t.Fatal(err)
	}
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}

	cs := ix.Comments(a.ULID)
	// live was made first, so it comes first in ULID order.
	if !slices.Equal(commentULIDs(cs), []model.ULID{live.ULID, gone.ULID}) {
		t.Fatalf("Comments = %+v, want them in ULID order", cs)
	}
	for _, c := range cs {
		switch c.ULID {
		case live.ULID:
			if c.Author != "github.com/x" || !c.Created.Equal(live.Created) || !c.Updated.Equal(live.Updated) || c.DeletedAt != nil {
				t.Errorf("live comment = %+v", c)
			}
		case gone.ULID:
			if c.Author != "github.com/y" || c.DeletedAt == nil || !c.DeletedAt.Equal(at) {
				t.Errorf("deleted comment = %+v", c)
			}
		}
	}
	if ws := ix.Worklogs(a.ULID); len(ws) != 1 || ws[0].ULID != w.ULID || ws[0].Author != "github.com/x" || !ws[0].Created.Equal(now) {
		t.Errorf("Worklogs = %+v", ws)
	}
	e, _ := ix.Task(a.ULID)
	if !slices.Equal(e.Mentions, []string{"alice", "bob"}) {
		t.Errorf("Mentions = %v, want [alice bob]: not those of a deleted comment", e.Mentions)
	}
	if !slices.Equal(e.Refs, []string{"TST-9", "TST-8"}) {
		t.Errorf("Refs = %v, want [TST-9 TST-8]", e.Refs)
	}
	if !slices.Equal(e.Watchers, []string{"github.com/w"}) {
		t.Errorf("Watchers = %v", e.Watchers)
	}

	// The cache holds the same, and so does a rebuild.
	reopened := mustOpen(t, s)
	if got := reopened.Comments(a.ULID); !reflect.DeepEqual(got, cs) {
		t.Errorf("Comments from the cache = %+v, want %+v", got, cs)
	}
	if got, _ := reopened.Task(a.ULID); !reflect.DeepEqual(got, e) {
		t.Errorf("entry from the cache = %s, want %s", dump([]*Entry{got}), dump([]*Entry{e}))
	}
	if got := rebuilt(t, s); !reflect.DeepEqual(sortedTasks(got), sortedTasks(ix)) {
		t.Errorf("a rebuild = %s, want %s", dump(sortedTasks(got)), dump(sortedTasks(ix)))
	}
}

// A change to the task keeps the mentions of its comments, and a change to
// a comment keeps those of the task.
func TestMentionsFollowTheirFiles(t *testing.T) {
	s, _ := newStore(t)
	a := newTask("a")
	a.Body = "@alice\n"
	put(t, s, a)
	c := newComment("github.com/x", "@bob\n")
	putComment(t, s, a.ULID, c)
	ix := mustOpen(t, s)
	mentions := func() []string {
		t.Helper()
		if err := ix.Refresh(); err != nil {
			t.Fatal(err)
		}
		e, _ := ix.Task(a.ULID)
		return e.Mentions
	}

	a.Body = "@dave\n"
	put(t, s, a)
	if got := mentions(); !slices.Equal(got, []string{"dave", "bob"}) {
		t.Errorf("after the body changed: Mentions = %v, want [dave bob]", got)
	}
	c.Body = "@eve\n"
	putComment(t, s, a.ULID, c)
	if got := mentions(); !slices.Equal(got, []string{"dave", "eve"}) {
		t.Errorf("after the comment changed: Mentions = %v, want [dave eve]", got)
	}
	at := time.Now()
	c.DeletedAt = &at
	putComment(t, s, a.ULID, c)
	if got := mentions(); !slices.Equal(got, []string{"dave"}) {
		t.Errorf("after the comment was deleted: Mentions = %v, want [dave]", got)
	}
	if got := rebuilt(t, s); !reflect.DeepEqual(sortedTasks(got), sortedTasks(ix)) {
		t.Errorf("a rebuild = %s, want %s", dump(sortedTasks(got)), dump(sortedTasks(ix)))
	}
}

// rebuilt opens the index of s with no usable cache, so that it reads every
// file.
func rebuilt(t *testing.T, s *store.Store) *Index {
	t.Helper()
	if err := s.WriteLocal(cacheName, []byte("no cache")); err != nil {
		t.Fatal(err)
	}
	return mustOpen(t, s)
}

// dump shows the entries es.
func dump(es []*Entry) string {
	var b strings.Builder
	for _, e := range es {
		fmt.Fprintf(&b, "\n%+v", *e)
	}
	return b.String()
}

// sortedTasks returns the entries of ix in ULID order.
func sortedTasks(ix *Index) []*Entry {
	es := ix.Tasks()
	slices.SortFunc(es, func(a, b *Entry) int { return strings.Compare(a.ULID.String(), b.ULID.String()) })
	return es
}
