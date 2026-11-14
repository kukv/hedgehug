// Package search は、検索と、画面の状態の API(E1-24)。Store をまたぐので /api/stores/{store} の下に置かない。
package search

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appsearch "github.com/kukv/hedgehug/internal/app/search"
	"github.com/kukv/hedgehug/internal/domain/query"
)

// SearchQuery は、検索のクエリ。
type SearchQuery struct {
	Query string `json:"query" doc:"クエリの文字列(04「検索」)。空なら絞り込まない"`
}

// SearchStatus は、行のステータス。色は返さない(画面の点は分類で色を決める)。
type SearchStatus struct {
	_        struct{} `nullable:"true"`
	Name     string   `json:"name"`
	Category string   `json:"category"`
}

// SearchAssignee は、行の担当者。name が null はメンバーにいない人。
type SearchAssignee struct {
	_     struct{} `nullable:"true"`
	Login string   `json:"login"`
	Name  *string  `json:"name" required:"true" nullable:"true" doc:"表示名。メンバーにいない人は null"`
}

// SearchPriority は、行の優先度。色は返さない(画面は名前だけを出す)。
type SearchPriority struct {
	_    struct{} `nullable:"true"`
	Name string   `json:"name"`
}

// SearchDeleted は、削除済みの行の印。
type SearchDeleted struct {
	_        struct{} `nullable:"true"`
	DaysLeft int      `json:"daysLeft" doc:"完全に削除されるまでの日数。0 以下なら保持日数を過ぎている"`
}

// SearchRow は、検索の結果の 1 行。
type SearchRow struct {
	StoreID     string          `json:"storeId"`
	StoreName   string          `json:"storeName"`
	StoreColor  string          `json:"storeColor"`
	ReadOnly    bool            `json:"readOnly"`
	WorkspaceID string          `json:"workspaceId"`
	ProjectID   string          `json:"projectId"`
	ItemID      string          `json:"itemId"`
	Display     string          `json:"display" doc:"いまの ID(HH-12)"`
	OldIDHit    bool            `json:"oldIdHit" doc:"旧 ID だけで一致した(画面は「旧 ID」と添える)"`
	Title       string          `json:"title"`
	Kind        string          `json:"kind"`
	Status      *SearchStatus   `json:"status"`
	Assignee    *SearchAssignee `json:"assignee"`
	Priority    *SearchPriority `json:"priority"`
	End         string          `json:"end" doc:"終了日(YYYY-MM-DD)。なければ空"`
	Points      *float64        `json:"points" required:"true" nullable:"true"`
	Deleted     *SearchDeleted  `json:"deleted" doc:"削除済みの行だけ"`
}

// SearchResult は、検索の結果。エラーがあれば rows は空。件数は rows の数。
type SearchResult struct {
	Errors  []string    `json:"errors" nullable:"false" doc:"クエリのエラー(1 語 1 行)"`
	HasKey  bool        `json:"hasKey" doc:"キーのある語が 1 つ以上あるか"`
	Group   string      `json:"group" doc:"group: の値(グループ分けは画面が行う)。なければ空"`
	Deleted bool        `json:"deleted" doc:"is:deleted の検索か"`
	Rows    []SearchRow `json:"rows" nullable:"false"`
}

type searchInput struct{ Body SearchQuery }

type searchOutput struct{ Body SearchResult }

// Register は、検索と画面の状態の API を登録する。
func Register(api huma.API, svc appsearch.Service, screens appsearch.ScreenStates) {
	huma.Register(api, huma.Operation{OperationID: "search", Method: http.MethodPost, Path: "/api/search", Summary: "表示中のすべての Store のアイテムをクエリで検索する"},
		func(ctx context.Context, in *searchInput) (*searchOutput, error) {
			res, err := svc.Search(ctx, in.Body.Query)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &searchOutput{Body: toResult(res)}, nil
		})
	registerScreens(api, screens)
}

func toResult(res appsearch.Result) SearchResult {
	out := SearchResult{Errors: append([]string{}, res.Errors...), HasKey: res.HasKey, Group: res.Group, Deleted: res.Deleted, Rows: []SearchRow{}}
	for _, r := range res.Rows {
		out.Rows = append(out.Rows, toRow(r))
	}
	return out
}

func toRow(r appsearch.Row) SearchRow {
	f := r.Facts
	it := f.Item
	row := SearchRow{
		StoreID: string(r.Store.ID), StoreName: r.Store.Name, StoreColor: r.Store.Color, ReadOnly: r.Store.ReadOnly,
		WorkspaceID: string(it.Place.Workspace), ProjectID: string(it.Place.Project), ItemID: string(it.ID),
		Display: f.Display, OldIDHit: r.OldID, Title: it.Title, Kind: string(it.Kind),
		Status: toStatus(f), Assignee: toAssignee(f), Priority: toPriority(f), End: it.End, Points: it.Points,
	}
	if r.DaysLeft != nil {
		row.Deleted = &SearchDeleted{DaysLeft: *r.DaysLeft}
	}
	return row
}

func toStatus(f query.Facts) *SearchStatus {
	for _, v := range f.Settings.Statuses {
		if f.Item.Status != "" && v.ID == f.Item.Status {
			return &SearchStatus{Name: v.Name, Category: string(v.Category)}
		}
	}
	return nil
}

func toPriority(f query.Facts) *SearchPriority {
	for _, v := range f.Settings.Priorities {
		if f.Item.Priority != "" && v.ID == f.Item.Priority {
			return &SearchPriority{Name: v.Name}
		}
	}
	return nil
}

func toAssignee(f query.Facts) *SearchAssignee {
	login := f.Item.Assignee
	if login == "" {
		return nil
	}
	a := &SearchAssignee{Login: login}
	if name, ok := f.Names[strings.ToLower(login)]; ok {
		a.Name = &name
	}
	return a
}
