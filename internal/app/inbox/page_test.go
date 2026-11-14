package inbox_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/inbox"
	"github.com/kukv/hedgehug/internal/app/port"
)

const (
	s1   port.StoreID = "s1"
	s2   port.StoreID = "s2"
	rule              = "01SAMPLERULE00000000000001"
)

type fixture struct {
	svc    inbox.Service
	stores *apptest.Stores
	book   *apptest.Inbox
	list   *apptest.StoreList
}

// setup は、Store s1・s2 に Sample を書き、2 つとも表示中にした受信箱を組む。
func setup(t *testing.T) fixture {
	t.Helper()
	clock := apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC))
	st := apptest.NewStores(clock, apptest.NewIDs())
	for _, id := range []port.StoreID{s1, s2} {
		st.AddStore(id)
		if err := apptest.Sample(context.Background(), st, id); err != nil {
			t.Fatal(err)
		}
	}
	list := &apptest.StoreList{Stores: []port.StoreInfo{{ID: s1, Name: "web", Color: "sky"}, {ID: s2, Name: "team", Color: "grass"}}}
	book := &apptest.Inbox{}
	return fixture{svc: inbox.Service{Book: book, Stores: st, List: list}, stores: st, book: book, list: list}
}

// changed は、s1 の Task HH-2 を自動化が変えた通知。
func changed(text string) port.Notice {
	return port.Notice{Kind: port.NoticeAutomation, Store: s1, Workspace: apptest.SampleWorkspace, Item: apptest.SampleTask, Rule: rule, Text: text, Display: "HH-2"}
}

// stopped は、s1 の Task HH-2 を Todo から Done に変えようとして止まった通知。
func stopped() port.Notice {
	return port.Notice{
		Kind: port.NoticeAutomationStopped, Store: s1, Workspace: apptest.SampleWorkspace, Item: apptest.SampleTask, Rule: rule,
		Text: "止まった", Reason: "ワークフロー", Status: apptest.SampleStatusDone, From: apptest.SampleStatusTodo, Display: "HH-2",
	}
}

func (f fixture) add(t *testing.T, ns ...port.Notice) {
	t.Helper()
	for _, n := range ns {
		if err := f.book.Add(t.Context(), n); err != nil {
			t.Fatal(err)
		}
	}
}

func (f fixture) page(t *testing.T, tab inbox.Tab, before *port.NoticeCursor) inbox.Page {
	t.Helper()
	p, err := f.svc.Page(t.Context(), tab, before)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func texts(p inbox.Page) []string {
	out := []string{}
	for _, r := range p.Rows {
		out = append(out, r.Text)
	}
	return out
}

func TestPageNewestFirstAndCounts(t *testing.T) {
	f := setup(t)
	f.add(t, changed("1"), changed("2"), changed("3"))
	p := f.page(t, inbox.TabAll, nil)
	if got := texts(p); len(got) != 3 || got[0] != "3" || got[1] != "2" || got[2] != "1" {
		t.Errorf("rows = %v, want [3 2 1]", got)
	}
	if p.Unread[inbox.TabAll] != 3 || p.Unread[inbox.TabAutomation] != 3 || p.Unread[inbox.TabMention] != 0 {
		t.Errorf("unread = %v", p.Unread)
	}
	if len(p.Unread) != 7 {
		t.Errorf("unread has %d tabs, want 7: %v", len(p.Unread), p.Unread)
	}
	if p.More {
		t.Error("More = true, want false")
	}
}

func TestPageTabFilters(t *testing.T) {
	f := setup(t)
	f.add(t, changed("1"), stopped())
	if p := f.page(t, inbox.TabMention, nil); len(p.Rows) != 0 {
		t.Errorf("mention rows = %v, want none", texts(p))
	}
	if p := f.page(t, inbox.TabAutomation, nil); len(p.Rows) != 2 {
		t.Errorf("automation rows = %v, want 2", texts(p))
	}
}

func TestKindsOfAndTabOf(t *testing.T) {
	if inbox.KindsOf(inbox.TabAll) != nil {
		t.Error("KindsOf(all) is not nil")
	}
	if got := inbox.KindsOf(inbox.TabAutomation); len(got) != 2 || got[0] != port.NoticeAutomation || got[1] != port.NoticeAutomationStopped {
		t.Errorf("KindsOf(automation) = %v", got)
	}
	if got := inbox.KindsOf(inbox.TabRepair); len(got) != 1 || got[0] != port.NoticeRepair {
		t.Errorf("KindsOf(repair) = %v", got)
	}
	if inbox.TabOf(port.NoticeAutomationStopped) != inbox.TabAutomation || inbox.TabOf(port.NoticeWatch) != inbox.TabWatch {
		t.Error("TabOf does not map kinds to their tabs")
	}
}

func TestPageMoreAt300(t *testing.T) {
	f := setup(t)
	for k := range 301 {
		f.add(t, changed(strconv.Itoa(k)))
	}
	p := f.page(t, inbox.TabAll, nil)
	if len(p.Rows) != 300 || !p.More {
		t.Fatalf("rows = %d, More = %v, want 300 and true", len(p.Rows), p.More)
	}
	last := p.Rows[len(p.Rows)-1]
	next := f.page(t, inbox.TabAll, &port.NoticeCursor{At: last.At, ID: last.ID})
	if len(next.Rows) != 1 || next.More || next.Rows[0].Text != "0" {
		t.Errorf("next rows = %v, More = %v, want [0] and false", texts(next), next.More)
	}
}

func TestPageSkipsHiddenStore(t *testing.T) {
	f := setup(t)
	other := changed("s2")
	other.Store = s2
	f.add(t, changed("s1"), other)
	shown := f.list.Stores
	f.list.Stores = shown[:1]
	p := f.page(t, inbox.TabAll, nil)
	if got := texts(p); len(got) != 1 || got[0] != "s1" || p.Unread[inbox.TabAll] != 1 {
		t.Errorf("rows = %v, unread = %v, want only s1", got, p.Unread)
	}
	f.list.Stores = shown
	p = f.page(t, inbox.TabAll, nil)
	if got := texts(p); len(got) != 2 || p.Rows[0].Read || p.Rows[0].Store.Name != "team" || p.Unread[inbox.TabAll] != 2 {
		t.Errorf("rows = %v, unread = %v, want s2 back unread", got, p.Unread)
	}
}

func TestPagePrunesPurged(t *testing.T) {
	f := setup(t)
	epic := changed("epic")
	epic.Item = apptest.SampleEpic
	ruleChanged := port.Notice{Kind: port.NoticeAutomation, Store: s1, Workspace: apptest.SampleWorkspace, Rule: rule, Text: "rule"}
	f.add(t, changed("task"), epic, ruleChanged)
	f.stores.AddPurged(s1, port.Purged{Entity: "item", Target: apptest.SampleTask, Display: "HH-2"})
	f.stores.AddPurged(s1, port.Purged{Entity: "sprint", Target: apptest.SampleWorkspace, Display: "Sprint 1"})
	p := f.page(t, inbox.TabAll, nil)
	if got := texts(p); len(got) != 2 || got[0] != "rule" || got[1] != "epic" {
		t.Errorf("rows = %v, want [rule epic]", got)
	}
	if _, err := f.book.Get(t.Context(), 1); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("purged row: err = %v, want port.ErrNotFound", err)
	}
	f.stores.AddPurged(s1, port.Purged{Entity: "workspace", Target: apptest.SampleWorkspace, Display: "Web"})
	if p := f.page(t, inbox.TabAll, nil); len(p.Rows) != 0 || p.Unread[inbox.TabAll] != 0 {
		t.Errorf("rows = %v, unread = %v, want none", texts(p), p.Unread)
	}
	for id := port.NoticeID(2); id <= 3; id++ {
		if _, err := f.book.Get(t.Context(), id); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("row %d: err = %v, want port.ErrNotFound", id, err)
		}
	}
}

func TestPagePrunesPurgedProject(t *testing.T) {
	f := setup(t)
	byProject := changed("project")
	byProject.Item = apptest.SampleProject
	f.add(t, changed("task"), byProject)
	f.stores.AddPurged(s1, port.Purged{Entity: "project", Target: apptest.SampleProject, Display: "ログイン"})
	if got := texts(f.page(t, inbox.TabAll, nil)); len(got) != 1 || got[0] != "task" {
		t.Errorf("rows = %v, want [task]", got)
	}
	if _, err := f.book.Get(t.Context(), 2); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("purged row: err = %v, want port.ErrNotFound", err)
	}
}

func TestCountsPrunesPurged(t *testing.T) {
	f := setup(t)
	f.add(t, changed("task"), stopped())
	f.stores.AddPurged(s1, port.Purged{Entity: "item", Target: apptest.SampleTask, Display: "HH-2"})
	c, err := f.svc.Counts(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(c) != 7 || c[inbox.TabAll] != 0 || c[inbox.TabAutomation] != 0 {
		t.Errorf("counts = %v, want 7 tabs of 0", c)
	}
}

func TestRowSender(t *testing.T) {
	f := setup(t)
	f.add(t, changed("1"))
	r := f.page(t, inbox.TabAll, nil).Rows[0]
	if r.Sender != "" || r.Action != nil || r.Kind != port.NoticeAutomation || r.Store.ID != s1 {
		t.Errorf("row = %+v, want sender empty and no action", r)
	}
}

func TestTabs(t *testing.T) {
	want := []inbox.Tab{"all", "mention", "assign", "watch", "overwrite", "automation", "repair"}
	if got := inbox.Tabs(); !slices.Equal(got, want) {
		t.Errorf("Tabs() = %v, want %v", got, want)
	}
}
