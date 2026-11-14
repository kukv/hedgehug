package palette_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/palette"
	"github.com/kukv/hedgehug/internal/app/apptest"
	apppalette "github.com/kukv/hedgehug/internal/app/palette"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// memRecent は port.RecentItems の偽物(apptest はファイル数の上限なので、ここに置く)。
type memRecent struct{ rows []port.Recent }

func (m *memRecent) Touch(_ context.Context, store port.StoreID, id change.ID, at time.Time, _ int) error {
	m.rows = slices.DeleteFunc(m.rows, func(r port.Recent) bool { return r.Item == id })
	m.rows = slices.Insert(m.rows, 0, port.Recent{Store: store, Item: id, At: at})
	return nil
}

func (m *memRecent) List(context.Context) ([]port.Recent, error) { return slices.Clone(m.rows), nil }

func (m *memRecent) Forget(_ context.Context, items []change.ID) error {
	m.rows = slices.DeleteFunc(m.rows, func(r port.Recent) bool { return slices.Contains(items, r.Item) })
	return nil
}

var movedAt = time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)

// newAPI は、S1「main」に Sample を入れた API。
func newAPI(t *testing.T) (humatest.TestAPI, *apptest.Stores) {
	t.Helper()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	palette.Register(api, apppalette.Service{
		Stores:      s,
		StoreList:   &apptest.StoreList{Stores: []port.StoreInfo{{ID: "S1", Name: "main"}}},
		SyncTargets: &apptest.SyncTargets{Targets: []port.SyncTarget{{ID: "S1", Name: "main"}}},
		RecentItems: &memRecent{},
		Clock:       apptest.NewClock(time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)),
	})
	return api, s
}

type alias struct {
	ID string  `json:"id"`
	At *string `json:"at"`
}

type indexBody struct {
	Items []struct {
		StoreID, WorkspaceID, ProjectID, ItemID string
		Number                                  int
		Display, Title, ProjectTitle            string
		Aliases                                 []alias
	}
	Projects []struct {
		StoreID, WorkspaceID, ProjectID string
		Display, Title, WorkspaceName   string
		Aliases                         []alias
	}
}

// 旧 ID は、転送記録のものは at に change.TimeLayout の文字列、旧い接頭辞から作ったものは at に null。旧 ID のない
// アイテムの aliases は [](null ではない)。
func TestPaletteIndexReturnsItemsWithAliases(t *testing.T) {
	api, s := newAPI(t)
	op := port.Operation{Actor: change.User("taro"), Summary: "テスト"}
	if err := s.Write(t.Context(), "S1", op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteTransfers().Append(transfer.Record{At: movedAt, Entity: transfer.EntityItem, Target: apptest.SampleTask, Old: "OLD-9", New: "HH-2", ToStore: "S1"}),
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet("prefix", "HH", "HX")),
		)
	}); err != nil {
		t.Fatal(err)
	}

	res := api.Get("/api/palette-index")
	var got indexBody
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &got) != nil {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
	var task, epic bool
	for _, it := range got.Items {
		switch it.ItemID {
		case string(apptest.SampleTask):
			task = true
			at := movedAt.Format(change.TimeLayout)
			want := []alias{{ID: "OLD-9", At: &at}, {ID: "HH-2"}}
			if it.StoreID != "S1" || it.WorkspaceID != string(apptest.SampleWorkspace) || it.ProjectID != string(apptest.SampleProject) ||
				it.Number != 2 || it.Display != "HX-2" || it.Title != "ログイン画面を作る" || it.ProjectTitle != "ログイン" ||
				!reflect.DeepEqual(it.Aliases, want) {
				t.Errorf("HX-2 = %+v", it)
			}
		case string(apptest.SampleEpic):
			epic = true
		}
	}
	if !task || !epic {
		t.Fatalf("アイテムが足りない: %s", res.Body)
	}
	if len(got.Projects) != 1 || got.Projects[0].Display != "HX-P1" || got.Projects[0].WorkspaceName != "Web" ||
		len(got.Projects[0].Aliases) != 1 || got.Projects[0].Aliases[0].ID != "HH-P1" {
		t.Errorf("projects = %+v", got.Projects)
	}
	if !strings.Contains(res.Body.String(), `{"id":"HH-2","at":null}`) {
		t.Errorf("旧い接頭辞の at は null: %s", res.Body)
	}
}

// 旧 ID のないアイテムの aliases と、空の items・projects は [] にする。
func TestPaletteIndexEmptyListsAreArrays(t *testing.T) {
	api, _ := newAPI(t)
	res := api.Get("/api/palette-index")
	if res.Code != 200 || strings.Contains(res.Body.String(), "null") || !strings.Contains(res.Body.String(), `"aliases":[]`) {
		t.Errorf("%d %s", res.Code, res.Body)
	}

	_, empty := humatest.New(t)
	s := apptest.NewStores(apptest.NewClock(time.Now()), apptest.NewIDs())
	palette.Register(empty, apppalette.Service{Stores: s, StoreList: &apptest.StoreList{}, SyncTargets: &apptest.SyncTargets{}, RecentItems: &memRecent{}, Clock: apptest.NewClock(time.Now())})
	if res := empty.Get("/api/palette-index"); res.Code != 200 || strings.TrimSpace(res.Body.String()) != `{"items":[],"projects":[]}` {
		t.Errorf("空: %d %s", res.Code, res.Body)
	}
	if res := empty.Get("/api/recent-items"); res.Code != 200 || strings.TrimSpace(res.Body.String()) != `[]` {
		t.Errorf("最近開いたものの空: %d %s", res.Code, res.Body)
	}
}

// POST で覚えたものを、GET がいまの ID で返す。
func TestRecentItemsRoundTrip(t *testing.T) {
	api, _ := newAPI(t)
	res := api.Post("/api/recent-items", map[string]any{"storeId": "S1", "itemId": string(apptest.SampleTask)})
	if res.Code != 204 {
		t.Fatalf("POST: %d %s", res.Code, res.Body)
	}
	res = api.Get("/api/recent-items")
	want := `[{"storeId":"S1","itemId":"` + string(apptest.SampleTask) + `","display":"HH-2","title":"ログイン画面を作る","projectTitle":"ログイン"}]`
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != want {
		t.Errorf("GET: %d %s", res.Code, res.Body)
	}
}
