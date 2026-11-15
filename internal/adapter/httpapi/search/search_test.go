package search_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	apisearch "github.com/kukv/hedgehug/internal/adapter/httpapi/search"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	appsearch "github.com/kukv/hedgehug/internal/app/search"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
)

type memScreens struct{ m map[string]string }

func (s *memScreens) Get(_ context.Context, screen string) (string, bool, error) {
	v, ok := s.m[screen]
	return v, ok, nil
}

func (s *memScreens) Put(_ context.Context, screen, body string) error {
	s.m[screen] = body
	return nil
}

const (
	bugID   item.ID = "01TESTIT000000000000000003"
	ghostID item.ID = "01TESTIT000000000000000004"
)

// newAPI は、S1「main」にサンプルと、Bug の HH-3(担当者 hanako・Medium)、Story の HH-4(メンバーにいない ghost・ステータスなし)を入れた API。
func newAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	api, _ := newAPIWithStores(t)
	return api
}

func newAPIWithStores(t *testing.T) (humatest.TestAPI, *apptest.Stores) {
	t.Helper()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	bug := item.Item{
		ID: bugID, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 3},
		Kind: item.Bug, Rank: "V", Title: "落ちる", Status: apptest.SampleStatusTodo, Priority: apptest.SamplePriorityMedium,
		Assignee: "hanako", End: "2026-11-30", Start: "2026-11-20", Systems: []change.ID{apptest.SampleSystemWeb},
	}
	ghost := item.Item{
		ID: ghostID, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 4},
		Kind: item.Story, Rank: "W", Title: "迷子", Assignee: "ghost",
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "テスト"}
	if err := s.Write(t.Context(), "S1", op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(bug.ID, item.Create(bug)),
			tx.WriteItems().Append(ghost.ID, item.Create(ghost)),
			tx.WriteWorkspaces().Append(bug.Place.Workspace, numbering.Draft(numbering.Items, 3), numbering.Draft(numbering.Items, 4)),
		)
	}); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	svc := appsearch.Service{
		Stores:      s,
		StoreList:   &apptest.StoreList{Stores: []port.StoreInfo{{ID: "S1", Name: "main", Color: "sky"}}},
		SyncTargets: &apptest.SyncTargets{Targets: []port.SyncTarget{{ID: "S1", Name: "main"}}},
		Identity:    apptest.Identity{Login: "taro"},
		Inbox:       &apptest.Inbox{},
		Clock:       apptest.NewClock(time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)),
	}
	apisearch.Register(api, svc, &memScreens{m: map[string]string{}})
	return api, s
}

type searchBody struct {
	Errors  []string
	HasKey  bool
	HasSort bool
	Group   string
	Deleted bool
	Rows    []struct {
		StoreID, StoreName, StoreColor string
		ReadOnly                       bool
		WorkspaceID, ProjectID, ItemID string
		Display, Title, Kind, End      string
		StatusID                       string
		OldIDHit                       bool
		Status                         *struct{ Name, Category string }
		Assignee                       *struct {
			Login string
			Name  *string
		}
		Priority *struct{ Name string }
		Points   *float64
		Deleted  *struct{ DaysLeft int }
	}
}

// searchOf は、検索して 200 の応答を読む。生の本文も返す。
func searchOf(t *testing.T, api humatest.TestAPI, text string) (searchBody, string) {
	t.Helper()
	res := api.Post("/api/search", map[string]string{"query": text})
	var got searchBody
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &got) != nil {
		t.Fatalf("%q: %d %s", text, res.Code, res.Body)
	}
	return got, res.Body.String()
}

func TestSearchAPI(t *testing.T) {
	got, raw := searchOf(t, newAPI(t), "kind:bug")
	if len(got.Rows) != 1 || !got.HasKey {
		t.Fatalf("rows = %d hasKey=%v", len(got.Rows), got.HasKey)
	}
	r := got.Rows[0]
	if r.Kind != "Bug" || r.StoreID != "S1" || r.StoreName != "main" || r.StoreColor != "sky" || r.ReadOnly ||
		r.ItemID != string(bugID) || r.Display != "HH-3" || r.Title != "落ちる" || r.End != "2026-11-30" || r.Points != nil || r.Deleted != nil ||
		r.WorkspaceID != string(apptest.SampleWorkspace) || r.ProjectID != string(apptest.SampleProject) {
		t.Errorf("row = %+v", r)
	}
	if r.Status == nil || r.Status.Name != "Todo" || r.Status.Category != "todo" {
		t.Errorf("status = %+v", r.Status)
	}
	// ステータス・優先度の色は画面で使わない(ステータスの点は分類で色を決める)ので、応答に含めない。
	var keys struct {
		Rows []struct{ Status, Priority map[string]any }
	}
	if err := json.Unmarshal([]byte(raw), &keys); err != nil || len(keys.Rows) != 1 || len(keys.Rows[0].Status) != 2 || len(keys.Rows[0].Priority) != 1 {
		t.Errorf("status・priority keys = %+v (%v)", keys, err)
	}
	if r.Priority == nil || r.Priority.Name != "Medium" {
		t.Errorf("priority = %+v", r.Priority)
	}
	if r.Assignee == nil || r.Assignee.Login != "hanako" || r.Assignee.Name == nil || *r.Assignee.Name != "hanako" {
		t.Errorf("assignee = %+v", r.Assignee)
	}
}

// メンバーにいない人は name が null。ステータス・優先度がなければ null。
func TestSearchAPINulls(t *testing.T) {
	got, raw := searchOf(t, newAPI(t), "kind:story")
	if len(got.Rows) != 1 {
		t.Fatalf("rows = %d", len(got.Rows))
	}
	g := got.Rows[0]
	if g.Assignee == nil || g.Assignee.Login != "ghost" || g.Assignee.Name != nil || g.Status != nil || g.Priority != nil {
		t.Errorf("ghost = %+v", g)
	}
	if !strings.Contains(raw, `"name":null`) || !strings.Contains(raw, `"status":null`) {
		t.Errorf("null で出す: %s", raw)
	}
}

// 削除済みの行は deleted: {daysLeft}(削除の日 + 保持日数 - 今日)。11/12 に削除・保持日数 10・今日 11/14 で 8。
func TestSearchAPIDeleted(t *testing.T) {
	api, s := newAPIWithStores(t)
	if err := apptest.DeleteItems(t.Context(), s, "S1", ghostID); err != nil {
		t.Fatal(err)
	}
	s.SetRetentionDays("S1", 10)
	got, raw := searchOf(t, api, "is:deleted")
	if !got.Deleted || len(got.Rows) != 1 || got.Rows[0].ItemID != string(ghostID) {
		t.Fatalf("rows = %+v: %s", got.Rows, raw)
	}
	d := got.Rows[0].Deleted
	if d == nil || d.DaysLeft != 8 {
		t.Errorf("deleted = %+v", d)
	}
	// 削除されていない行の deleted は null。
	if live, raw := searchOf(t, api, "kind:bug"); len(live.Rows) != 1 || live.Rows[0].Deleted != nil || !strings.Contains(raw, `"deleted":null`) {
		t.Errorf("live: %s", raw)
	}
}

// エラーがあれば rows は空([]、null ではない)。
func TestSearchAPIErrors(t *testing.T) {
	_, raw := searchOf(t, newAPI(t), "status:")
	if !strings.Contains(raw, `"errors":["「status:」に値がありません"]`) || !strings.Contains(raw, `"rows":[]`) {
		t.Errorf("status:: %s", raw)
	}
}

func TestScreenStateAPI(t *testing.T) {
	api := newAPI(t)

	if res := api.Get("/api/screen-states/search"); res.Code != 200 || strings.TrimSpace(res.Body.String()) != `{"body":null}` {
		t.Errorf("empty: %d %s", res.Code, res.Body)
	}
	body := `{"query":"x"}`
	if res := api.Put("/api/screen-states/search", map[string]string{"body": body}); res.Code != 204 {
		t.Fatalf("put: %d %s", res.Code, res.Body)
	}
	var got struct{ Body *string }
	res := api.Get("/api/screen-states/search")
	if json.Unmarshal(res.Body.Bytes(), &got) != nil || got.Body == nil || *got.Body != body {
		t.Errorf("get: %d %s", res.Code, res.Body)
	}

	if res := api.Put("/api/screen-states/A%20B", map[string]string{"body": "{}"}); res.Code != 422 {
		t.Errorf("A B: %d", res.Code)
	}
	if res := api.Get("/api/screen-states/A%20B"); res.Code != 422 {
		t.Errorf("get A B: %d", res.Code)
	}
	if res := api.Put("/api/screen-states/search", map[string]string{"body": strings.Repeat("a", 64*1024+1)}); res.Code != 422 {
		t.Errorf("65537 bytes: %d", res.Code)
	}
	if res := api.Put("/api/screen-states/search", map[string]string{"body": strings.Repeat("a", 64*1024)}); res.Code != 204 {
		t.Errorf("65536 bytes: %d", res.Code)
	}
}

type tableBody struct {
	GroupBy string
	Columns []struct {
		Key, Name, Type string
		Sortable        bool
	}
	Groups []struct {
		Label string
		Rows  []int
	}
	Rows []struct {
		Kind, Title, Start, Created, Updated string
		Systems, Unused, Removed             []string
		Estimate                             *float64
		Sprint, Release                      string
		Project                              struct{ Display, Title string }
		Parent                               *struct{ ItemID, Display, Title string }
		Custom                               map[string]struct{ Text, Type string }
	}
}

func tableOf(t *testing.T, api humatest.TestAPI, body any) (tableBody, string) {
	t.Helper()
	res := api.Post("/api/search", body)
	var got tableBody
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &got) != nil {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
	return got, res.Body.String()
}

func TestSearchAPITable(t *testing.T) {
	got, _ := tableOf(t, newAPI(t), map[string]any{"query": "", "sort": map[string]any{"key": "title", "desc": true}, "group": "kind"})
	if got.GroupBy != "kind" || len(got.Groups) == 0 {
		t.Fatalf("groupBy=%q groups=%+v", got.GroupBy, got.Groups)
	}
	for _, g := range got.Groups {
		for _, i := range g.Rows {
			if got.Rows[i].Kind != g.Label {
				t.Errorf("group %q の行 %d は %q", g.Label, i, got.Rows[i].Kind)
			}
		}
	}
	if len(got.Columns) < 3 {
		t.Fatalf("columns = %+v", got.Columns)
	}
	for i, k := range []string{"id", "kind", "title"} {
		if got.Columns[i].Key != k {
			t.Errorf("columns[%d] = %q, want %q", i, got.Columns[i].Key, k)
		}
	}
}

// 列見出しの並べ替えはタイトルの降順で並べる。sort が null の本文も通る。
func TestSearchAPISort(t *testing.T) {
	got, _ := tableOf(t, newAPI(t), map[string]any{"query": "", "sort": map[string]any{"key": "title", "desc": true}})
	titles := make([]string, 0, len(got.Rows))
	for _, r := range got.Rows {
		titles = append(titles, r.Title)
	}
	if len(titles) < 2 || !slices.IsSortedFunc(titles, func(a, b string) int { return strings.Compare(b, a) }) {
		t.Errorf("タイトルの降順でない: %v", titles)
	}
	if got, _ := tableOf(t, newAPI(t), map[string]any{"query": "", "sort": nil}); len(got.Rows) == 0 {
		t.Error("sort が null で行がない")
	}
}

func TestSearchAPIRowValues(t *testing.T) {
	sgot, _ := searchOf(t, newAPI(t), "kind:bug")
	if len(sgot.Rows) != 1 || sgot.Rows[0].StatusID != string(apptest.SampleStatusTodo) {
		t.Errorf("statusId = %+v", sgot.Rows)
	}
	got, _ := tableOf(t, newAPI(t), map[string]string{"query": "kind:bug"})
	if len(got.Rows) != 1 {
		t.Fatalf("rows = %d", len(got.Rows))
	}
	r := got.Rows[0]
	if r.Start != "2026-11-20" || len(r.Systems) != 1 || r.Systems[0] != "Web" || r.Created == "" || r.Updated == "" || r.Project.Display == "" || r.Project.Title != "ログイン" {
		t.Errorf("bug = %+v", r)
	}
	if _, err := time.Parse(time.RFC3339, r.Created); err != nil {
		t.Errorf("created = %q", r.Created)
	}
	got, _ = tableOf(t, newAPI(t), map[string]string{"query": "kind:task"})
	p := got.Rows[0].Parent
	if p == nil || p.Display != "HH-1" || p.Title != "認証" || p.ItemID != string(apptest.SampleEpic) {
		t.Errorf("parent = %+v", p)
	}
}

// E1 の本文のままでも通る。グループは空の配列、groupBy は空。
func TestSearchAPIWithoutTable(t *testing.T) {
	_, raw := tableOf(t, newAPI(t), map[string]string{"query": "kind:bug"})
	if !strings.Contains(raw, `"groups":[]`) || !strings.Contains(raw, `"groupBy":""`) || !strings.Contains(raw, `"columns":[`) {
		t.Errorf("raw: %s", raw)
	}
}

func TestSearchAPIHasSort(t *testing.T) {
	api := newAPI(t)
	for text, want := range map[string]bool{"sort:id": true, "kind:bug": false, "-sort:id": true} {
		if got, _ := searchOf(t, api, text); got.HasSort != want {
			t.Errorf("%q: HasSort = %v, want %v", text, got.HasSort, want)
		}
	}
}

func TestMyTasksAPI(t *testing.T) {
	api, s := newAPIWithStores(t)
	mine := item.Item{
		ID: "01TESTIT000000000000000005", Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 5},
		Kind: item.Task, Rank: "X", Title: "私の", Status: apptest.SampleStatusInProgress, Assignee: "taro",
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "テスト"}
	if err := s.Write(t.Context(), "S1", op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(mine.ID, item.Create(mine)),
			tx.WriteWorkspaces().Append(mine.Place.Workspace, numbering.Draft(numbering.Items, 5)),
		)
	}); err != nil {
		t.Fatal(err)
	}
	res := api.Post("/api/my-tasks", map[string]string{"filter": ""})
	var got searchBody
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &got) != nil {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
	if len(got.Rows) != 1 || got.Rows[0].StatusID != string(apptest.SampleStatusInProgress) {
		t.Errorf("rows = %+v", got.Rows)
	}
	res = api.Post("/api/my-tasks", map[string]string{"filter": "status:"})
	raw := res.Body.String()
	if res.Code != 200 || !strings.Contains(raw, `"errors":["「status:」に値がありません"]`) || !strings.Contains(raw, `"rows":[]`) {
		t.Errorf("status:: %d %s", res.Code, raw)
	}
}
