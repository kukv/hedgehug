package inbox_test

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/inbox"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appinbox "github.com/kukv/hedgehug/internal/app/inbox"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const s1 port.StoreID = "s1"

const rule = "01SAMPLERULE00000000000001"

type fixture struct {
	api    humatest.TestAPI
	book   *apptest.Inbox
	events *apptest.Events
	// applyErr は、偽物の「制約を無視して…」が返すエラー。
	applyErr error
}

// setup は、Store s1 に Sample を書き、表示中にした受信箱の API を組む。通知の時刻は 1 ナノ秒ずつ進む。
func setup(t *testing.T) *fixture {
	t.Helper()
	clock := apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC))
	st := apptest.NewStores(clock, apptest.NewIDs())
	st.AddStore(s1)
	if err := apptest.Sample(context.Background(), st, s1); err != nil {
		t.Fatal(err)
	}
	at := apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 123456789, time.UTC))
	at.Step = time.Nanosecond
	f := &fixture{book: &apptest.Inbox{Clock: at}, events: &apptest.Events{}}
	_, f.api = humatest.New(t)
	inbox.Register(f.api, appinbox.Service{
		Book: f.book, Stores: st, Events: f.events,
		List: &apptest.StoreList{Stores: []port.StoreInfo{{ID: s1, Name: "web", Color: "sky"}}},
		ApplyIgnoringWorkflow: func(context.Context, port.StoreID, item.ID, change.ID, change.ID, change.ID) error {
			return f.applyErr
		},
	})
	return f
}

func (f *fixture) add(t *testing.T, ns ...port.Notice) {
	t.Helper()
	for _, n := range ns {
		if err := f.book.Add(t.Context(), n); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *fixture) row(t *testing.T, id port.NoticeID) port.NoticeRow {
	t.Helper()
	r, err := f.book.Get(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return r
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

type page struct {
	Rows []struct {
		ID     int64     `json:"id"`
		Kind   string    `json:"kind"`
		At     time.Time `json:"at"`
		Read   bool      `json:"read"`
		Text   string    `json:"text"`
		Reason string    `json:"reason"`
		Sender string    `json:"sender"`
		Store  struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Color    string `json:"color"`
			ReadOnly bool   `json:"readOnly"`
		} `json:"store"`
		Target struct {
			Kind          string `json:"kind"`
			Item          string `json:"item"`
			ItemKind      string `json:"itemKind"`
			Display       string `json:"display"`
			Title         string `json:"title"`
			Workspace     string `json:"workspace"`
			WorkspaceName string `json:"workspaceName"`
		} `json:"target"`
		Action *struct {
			Status       string `json:"status"`
			StatusName   string `json:"statusName"`
			Visible      bool   `json:"visible"`
			ApplyEnabled bool   `json:"applyEnabled"`
			KeepEnabled  bool   `json:"keepEnabled"`
			Note         string `json:"note"`
			Result       string `json:"result"`
		} `json:"action"`
	} `json:"rows"`
	More   bool           `json:"more"`
	Unread map[string]int `json:"unread"`
}

func (f *fixture) list(t *testing.T, query string) page {
	t.Helper()
	res := f.api.Get("/api/inbox?" + query)
	if res.Code != http.StatusOK {
		t.Fatalf("list %s: %d %s", query, res.Code, res.Body)
	}
	var p page
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestListInbox(t *testing.T) {
	f := setup(t)
	f.add(t, changed("変えた"), stopped())
	p := f.list(t, "tab=all")
	if len(p.Rows) != 2 || p.More {
		t.Fatalf("rows = %+v more = %v", p.Rows, p.More)
	}
	top, second := p.Rows[0], p.Rows[1]
	if top.Kind != "automation_stopped" || top.Text != "止まった" || top.Reason != "ワークフロー" || top.Read || top.Sender != "" {
		t.Errorf("top = %+v", top)
	}
	if top.Store.ID != "s1" || top.Store.Name != "web" || top.Store.Color != "sky" || top.Store.ReadOnly {
		t.Errorf("store = %+v", top.Store)
	}
	if tg := top.Target; tg.Kind != "item" || tg.Item != string(apptest.SampleTask) || tg.ItemKind != "Task" || tg.Display != "HH-2" ||
		tg.Title == "" || tg.Workspace != string(apptest.SampleWorkspace) || tg.WorkspaceName == "" {
		t.Errorf("target = %+v", tg)
	}
	if a := top.Action; a == nil || a.Status != string(apptest.SampleStatusDone) || a.StatusName == "" || !a.Visible || !a.ApplyEnabled || !a.KeepEnabled || a.Result != "" {
		t.Errorf("action = %+v", a)
	}
	if second.Action != nil || second.Text != "変えた" {
		t.Errorf("second = %+v", second)
	}
	if want := wantUnread(); !maps.Equal(p.Unread, want) {
		t.Errorf("unread = %v, want %v", p.Unread, want)
	}
}

// wantUnread は、changed と stopped を 1 件ずつ足したときの、7 つのタブの未読数。
func wantUnread() map[string]int {
	return map[string]int{"all": 2, "mention": 0, "assign": 0, "watch": 0, "overwrite": 0, "automation": 2, "repair": 0}
}

func TestInboxUnread(t *testing.T) {
	f := setup(t)
	f.add(t, changed("変えた"), stopped())
	res := f.api.Get("/api/inbox/unread")
	var u struct{ Counts map[string]int }
	if res.Code != http.StatusOK || json.Unmarshal(res.Body.Bytes(), &u) != nil || !maps.Equal(u.Counts, wantUnread()) {
		t.Errorf("unread: %d %s", res.Code, res.Body)
	}
}

// 通知がなくても rows は空の配列(null にしない)。
func TestListInboxEmpty(t *testing.T) {
	f := setup(t)
	res := f.api.Get("/api/inbox?tab=mention")
	var raw map[string]json.RawMessage
	if res.Code != http.StatusOK || json.Unmarshal(res.Body.Bytes(), &raw) != nil || string(raw["rows"]) != "[]" {
		t.Errorf("empty: %d %s", res.Code, res.Body)
	}
}

// 返った行の at と id を before_at・before_id に渡すと、それより古い行だけが返る(ナノ秒まで)。
func TestListInboxCursor(t *testing.T) {
	f := setup(t)
	f.add(t, changed("1"), changed("2"), changed("3"))
	top := f.list(t, "tab=all").Rows[0]
	q := url.Values{"tab": {"all"}, "before_at": {top.At.Format(time.RFC3339Nano)}, "before_id": {strconv.FormatInt(top.ID, 10)}}
	p := f.list(t, q.Encode())
	got := []string{}
	for _, r := range p.Rows {
		got = append(got, r.Text)
	}
	if !slices.Equal(got, []string{"2", "1"}) {
		t.Errorf("rows = %v, want [2 1]", got)
	}
}

func TestListInboxBadTab(t *testing.T) {
	f := setup(t)
	for _, q := range []string{"tab=nope", "", "tab=all&before_at=yesterday&before_id=1"} {
		if res := f.api.Get("/api/inbox?" + q); res.Code != http.StatusBadRequest {
			t.Errorf("%q: %d %s", q, res.Code, res.Body)
		}
	}
	if res := f.api.Post("/api/inbox/read-all", map[string]any{"tab": "nope"}); res.Code != http.StatusBadRequest {
		t.Errorf("read-all: %d %s", res.Code, res.Body)
	}
}

func TestSetRead(t *testing.T) {
	f := setup(t)
	f.add(t, changed("1"))
	if res := f.api.Put("/api/inbox/1/read", map[string]any{"read": true}); res.Code != http.StatusNoContent {
		t.Fatalf("read: %d %s", res.Code, res.Body)
	}
	if !f.row(t, 1).Read {
		t.Error("not read")
	}
	if res := f.api.Put("/api/inbox/1/read", map[string]any{"read": false}); res.Code != http.StatusNoContent || f.row(t, 1).Read {
		t.Errorf("unread: %d %s", res.Code, res.Body)
	}
	if res := f.api.Put("/api/inbox/99/read", map[string]any{"read": true}); res.Code != http.StatusNotFound {
		t.Errorf("unknown: %d %s", res.Code, res.Body)
	}
	if got := f.events.List(); len(got) != 2 || got[0] != (port.Event{Kind: port.EventInbox, Store: s1}) {
		t.Errorf("events = %v", got)
	}
}

func TestReadAll(t *testing.T) {
	f := setup(t)
	f.add(t, changed("1"), stopped())
	if res := f.api.Post("/api/inbox/read-all", map[string]any{"tab": "automation"}); res.Code != http.StatusNoContent {
		t.Fatalf("read-all: %d %s", res.Code, res.Body)
	}
	if !f.row(t, 1).Read || !f.row(t, 2).Read {
		t.Error("not all read")
	}
	if got := f.events.List(); !slices.Equal(got, []port.Event{{Kind: port.EventInbox}}) {
		t.Errorf("events = %v", got)
	}
}

func TestApplyRefusal(t *testing.T) {
	f := setup(t)
	f.add(t, stopped())
	f.applyErr = refusal.New("HH-2 は先に変更されたため、適用しませんでした")
	res := f.api.Post("/api/inbox/1/apply")
	var body struct{ Detail string }
	if res.Code != http.StatusUnprocessableEntity || json.Unmarshal(res.Body.Bytes(), &body) != nil ||
		body.Detail != "HH-2 は先に変更されたため、適用しませんでした" {
		t.Errorf("apply: %d %s", res.Code, res.Body)
	}
	if r := f.row(t, 1); r.Result != "" || r.Read {
		t.Errorf("row = %+v", r)
	}
	f.applyErr = nil
	if res = f.api.Post("/api/inbox/1/apply"); res.Code != http.StatusNoContent || f.row(t, 1).Result != port.NoticeApplied {
		t.Errorf("apply: %d %s", res.Code, res.Body)
	}
}

func TestKeep(t *testing.T) {
	f := setup(t)
	f.add(t, changed("1"), stopped())
	if res := f.api.Post("/api/inbox/2/keep"); res.Code != http.StatusNoContent {
		t.Fatalf("keep: %d %s", res.Code, res.Body)
	}
	if r := f.row(t, 2); r.Result != port.NoticeKept || !r.Read {
		t.Errorf("row = %+v", r)
	}
	if res := f.api.Post("/api/inbox/1/keep"); res.Code != http.StatusBadRequest {
		t.Errorf("keep not stopped: %d %s", res.Code, res.Body)
	}
}
