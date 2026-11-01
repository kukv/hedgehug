package query

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// fixture is a store with tasks P, E, a, b, c and the deleted d, made in
// that order so that their ULIDs, and with no rank their order, follow it.
//
//	P (project, todo)
//	└── E (epic, todo)
//	    └── a (task, in_progress, label backend, assignee alice,
//	           priority high, due 2026-11-05, estimate 2h, points 3,
//	           blocked by b, duplicates c, body "検索語 で探す")
//	b (task, backlog, id WEB-1, priority urgent, due 2026-11-20,
//	   relates c, body "Fix LOGIN 検索語")
//	c (task, done, label wip, priority low)
//	d (task, todo, deleted)
type fixture struct {
	ix  *index.Index
	ids map[string]model.ULID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	s := newStore(t)
	p, e, a, b, c, d := newTask("P"), newTask("E"), newTask("a"), newTask("b"), newTask("c"), newTask("d")
	p.Kind = "project"
	e.Kind, e.Parent = "epic", &p.ULID
	a.Parent, a.Status = &e.ULID, "in_progress"
	a.Labels, a.Assignees = []string{"backend"}, []string{"github.com/alice"}
	setProp(a, "priority", "high")
	setProp(a, "due", "2026-11-05")
	setProp(a, "estimate", "2h")
	setProp(a, "points", "3")
	a.BlockedBy = []model.ULID{b.ULID}
	setRefs(a, "duplicates", c.ULID)
	a.Body = "検索語 で探す\n"
	b.Status, b.ID = "backlog", "WEB-1"
	setProp(b, "priority", "urgent")
	setProp(b, "due", "2026-11-20")
	setRefs(b, "relates", c.ULID)
	b.Body = "Fix LOGIN 検索語\n"
	c.Status, c.Labels = "done", []string{"wip"}
	setProp(c, "priority", "low")
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	d.DeletedAt = &at
	put(t, s, p, e, a, b, c, d)
	f := &fixture{ix: mustOpen(t, s), ids: map[string]model.ULID{}}
	for _, task := range []*model.Task{p, e, a, b, c, d} {
		f.ids[task.Title] = task.ULID
	}
	return f
}

func TestRunFilters(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	me := Options{Me: []string{"github.com/alice"}}
	tests := []struct {
		q    string
		opt  Options
		want []string
	}{
		{"", Options{}, []string{"P", "E", "a", "b", "c"}},
		{"status:in_progress", Options{}, []string{"a"}},
		{"status:doing", Options{}, []string{"a"}},
		{"status:todo", Options{}, []string{"P", "E", "b"}}, // the category, which holds backlog
		{"status:in_progress,done", Options{}, []string{"a", "c"}},
		{"-status:done", Options{}, []string{"P", "E", "a", "b"}},
		{"kind:task", Options{}, []string{"a", "b", "c"}},
		{"label:BACKEND", Options{}, []string{"a"}},
		{"label:none", Options{}, []string{"P", "E", "b"}},
		{"assignee:@me", me, []string{"a"}},
		{"assignee:@me", Options{Me: []string{"github.com/Alice"}}, []string{"a"}}, // an account ID is compared without case
		{"assignee:github.com/alice", Options{}, []string{"a"}},
		{"parent:" + f.ids["E"].String(), Options{}, []string{"a"}},
		{"parent:none", Options{}, []string{"P", "b", "c"}},
		{"project:" + f.ids["P"].String(), Options{}, []string{"E", "a"}},
		{"priority:high,urgent", Options{}, []string{"a", "b"}},
		{"due<2026-11-10", Options{}, []string{"a"}},
		{"due>=2026-11-05", Options{}, []string{"a", "b"}},
		{"due:2026-11-20", Options{}, []string{"b"}},
		{"due:none", Options{}, []string{"P", "E", "c"}},
		{"estimate>1h30m", Options{}, []string{"a"}},
		{"points:3", Options{}, []string{"a"}},
		{"blocked_by:" + f.ids["b"].String(), Options{}, []string{"a"}},
		{"blocked_by:web-1", Options{}, []string{"a"}},
		{"blocks:" + f.ids["a"].String(), Options{}, []string{"b"}},
		{"duplicates:" + f.ids["c"].String(), Options{}, []string{"a"}},
		{"重複されている:" + f.ids["a"].String(), Options{}, []string{"c"}},
		{"relates:" + f.ids["c"].String(), Options{}, []string{"b"}},
		{"relates:" + f.ids["b"].String(), Options{}, []string{"c"}}, // symmetric
		{"relates:none", Options{}, []string{"P", "E", "a"}},
		{"is:blocked", Options{}, []string{"a"}},
		{"is:invalid", Options{}, []string{}},
		{"is:deleted", Options{}, []string{"d"}},
		{"-is:deleted", Options{}, []string{"P", "E", "a", "b", "c"}},
		{"検索語", Options{}, []string{"a", "b"}},
		{`"fix login"`, Options{}, []string{"b"}},
		{`-"検索語"`, Options{}, []string{"P", "E", "c"}},
		// The example of the parent design, within phase 1c.
		{`status:doing priority:high,urgent due<2026-11-10 -label:wip kind:task is:blocked "検索語"`, Options{}, []string{"a"}},
	}
	for _, tt := range tests {
		got := run(t, f.ix, tt.q, tt.opt)
		if len(got) != 1 || !slices.Equal(got[0], tt.want) {
			t.Errorf("%q = %v, want [%v]", tt.q, got, tt.want)
		}
	}
}

func TestRunMentionsWatchingOverwritten(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	body, comment, gone, other, watched, over := newTask("body"), newTask("comment"), newTask("gone"), newTask("other"), newTask("watched"), newTask("over")
	body.Body = "for @Alice\n"
	other.Body = "for @gitlab.com/alice\n"
	watched.Watchers = []string{"github.com/b"}
	put(t, s, body, comment, gone, other, watched, over)
	now := time.Now().Truncate(time.Second)
	if _, err := s.Update(store.Op{Name: "comment", Author: testAuthor}, func(tx *store.Tx) error {
		return errors.Join(
			tx.PutComment(comment.ULID, &model.Comment{ULID: model.NewULID(), Author: "github.com/x", Created: now, Body: "ask @github.com/alice\n"}),
			tx.PutComment(gone.ULID, &model.Comment{ULID: model.NewULID(), Author: "github.com/x", Created: now, DeletedAt: &now, Body: "ask @alice\n"}),
		)
	}); err != nil {
		t.Fatal(err)
	}
	ix := mustOpen(t, s)
	alice := Options{Me: []string{"github.com/alice"}}
	tests := []struct {
		q    string
		opt  Options
		want []string
	}{
		// @login names an account of any host with that login.
		{"mentions:@me", alice, []string{"body", "comment"}},
		{"mentions:GitHub.com/alice", Options{}, []string{"body", "comment"}},
		{"mentions:gitlab.com/alice", Options{}, []string{"body", "other"}},
		// A login with no host names that login on any host.
		{"mentions:ALICE", Options{}, []string{"body", "comment", "other"}},
		{"mentions:none", Options{}, []string{"gone", "watched", "over"}},
		{"-mentions:@me", alice, []string{"gone", "other", "watched", "over"}},
		{"watching:github.com/b", Options{}, []string{"watched"}},
		{"watching:GITHUB.COM/B", Options{}, []string{"watched"}},
		{"watching:@me", Options{Me: []string{"github.com/a", "github.com/b"}}, []string{"watched"}},
		{"watching:@me", alice, []string{}},
		{"is:overwritten", Options{Overwritten: map[model.ULID]bool{over.ULID: true}}, []string{"over"}},
		{"is:overwritten", Options{}, []string{}},
	}
	for _, tt := range tests {
		if got := run(t, ix, tt.q, tt.opt); len(got) != 1 || !slices.Equal(got[0], tt.want) {
			t.Errorf("%q = %v, want [%v]", tt.q, got, tt.want)
		}
	}
}

func TestRunIsInvalid(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ok := newTask("ok")
	put(t, s, ok)
	bad := model.NewULID()
	writeRaw(t, s, model.TaskPath(bad), "---\nulid: "+bad.String()+"\nkind: nope\ntitle: bad\nstatus: todo\n---\n")
	if got := run(t, mustOpen(t, s), "is:invalid", Options{}); len(got) != 1 || !slices.Equal(got[0], []string{"bad"}) {
		t.Errorf("is:invalid = %v, want [[bad]]", got)
	}
}

func TestRunErrors(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	tests := []struct {
		q   string
		opt Options
		pos int
	}{
		{"foo:bar", Options{}, 1},
		{"kind:task status:nope", Options{}, 2},
		{"kind:nope", Options{}, 1},
		{"priority:nope", Options{}, 1},
		{"label<3", Options{}, 1},
		{"priority>high", Options{}, 1},
		{"due<tomorrow", Options{}, 1},
		{"due<none", Options{}, 1},
		{"parent:ZZZZ", Options{}, 1},
		{"assignee:@me", Options{}, 1},
		{"mentions:@me", Options{}, 1},
		{"watching:@me", Options{}, 1},
		{"is:nope", Options{}, 1},
		{"project:none", Options{}, 1},
	}
	for _, tt := range tests {
		q, err := Parse(tt.q)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.q, err)
		}
		_, err = Run(f.ix, q, tt.opt)
		var qe *Error
		if !errors.As(err, &qe) || qe.Pos != tt.pos {
			t.Errorf("Run(%q) err = %v, want an Error at term %d", tt.q, err, tt.pos)
		}
	}
}

func TestRunWorkspace(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	api := &model.Workspace{ULID: model.NewULID(), Name: "api", IDPrefix: "API"}
	if _, err := s.Update(store.Op{Name: "workspace", Author: testAuthor}, func(tx *store.Tx) error { return tx.PutWorkspace(api) }); err != nil {
		t.Fatal(err)
	}
	a, b := newTask("a"), newTask("b")
	b.Workspace = api.ULID
	put(t, s, a, b)
	ix := mustOpen(t, s)
	tests := []struct {
		q    string
		want []string
	}{
		{"workspace:api", []string{"b"}},
		{"workspace:API", []string{"b"}},
		{"workspace:test", []string{"a"}},
		{"workspace:api,test", []string{"b", "a"}}, // same rank: API-1 before TST-1
		{"-workspace:api", []string{"a"}},
	}
	for _, tt := range tests {
		if got := run(t, ix, tt.q, Options{}); len(got) != 1 || !slices.Equal(got[0], tt.want) {
			t.Errorf("%q = %v, want [%v]", tt.q, got, tt.want)
		}
	}
	parsed, err := Parse("workspace:nope")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ix, parsed, Options{}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("workspace:nope err = %v, want ErrNotFound", err)
	}
}
