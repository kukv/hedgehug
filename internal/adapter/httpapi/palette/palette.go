// Package palette は、コマンドパレットの索引と、最近開いたものの API(A5c-17)。
package palette

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	apppalette "github.com/kukv/hedgehug/internal/app/palette"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// PaletteAlias は、旧 ID。at は移動した日時で、旧い接頭辞から作ったものは null。
type PaletteAlias struct {
	ID string  `json:"id" doc:"旧 ID(HH-12)"`
	At *string `json:"at" required:"true" nullable:"true" doc:"移動した日時。旧い接頭辞から作ったものは null"`
}

// PaletteItem は、パレットのアイテムの候補 1 件(表示中の Store の、削除されていないアイテム)。
type PaletteItem struct {
	StoreID      string         `json:"storeId"`
	WorkspaceID  string         `json:"workspaceId"`
	ProjectID    string         `json:"projectId"`
	ItemID       string         `json:"itemId"`
	Number       int            `json:"number" doc:"ワークスペースの中の番号(並びに使う)"`
	Display      string         `json:"display" doc:"いまの ID(HH-12)"`
	Title        string         `json:"title"`
	ProjectTitle string         `json:"projectTitle"`
	Aliases      []PaletteAlias `json:"aliases" doc:"旧 ID(いまの ID と同じものを除く)"`
}

// PaletteProject は、パレットのプロジェクトの候補 1 件(表示中の Store の、削除されていないプロジェクト)。
type PaletteProject struct {
	StoreID       string         `json:"storeId"`
	WorkspaceID   string         `json:"workspaceId"`
	ProjectID     string         `json:"projectId"`
	Display       string         `json:"display" doc:"いまの ID(HH-P1)"`
	Title         string         `json:"title"`
	WorkspaceName string         `json:"workspaceName"`
	Aliases       []PaletteAlias `json:"aliases" doc:"旧 ID(いまの ID と同じものを除く)"`
}

// PaletteIndex は、パレットのアイテム・プロジェクトの候補(A5c-3)。並びは問わない(画面が並べる)。
type PaletteIndex struct {
	Items    []PaletteItem    `json:"items"`
	Projects []PaletteProject `json:"projects"`
}

// RecentItem は、最近開いたアイテム 1 件(新しい順。A5c-13)。storeId・display は、いま見つかった Store とそこでの ID。
type RecentItem struct {
	StoreID      string `json:"storeId"`
	ItemID       string `json:"itemId"`
	Display      string `json:"display"`
	Title        string `json:"title"`
	ProjectTitle string `json:"projectTitle"`
}

// OpenedItem は、開いたアイテム。
type OpenedItem struct {
	StoreID string `json:"storeId"`
	ItemID  string `json:"itemId"`
}

type indexOutput struct{ Body PaletteIndex }

type recentOutput struct{ Body []RecentItem }

type openedInput struct{ Body OpenedItem }

// Register は、パレットの API を登録する。
func Register(api huma.API, svc apppalette.Service) {
	huma.Register(api, huma.Operation{OperationID: "palette-index", Method: http.MethodGet, Path: "/api/palette-index", Summary: "パレットのアイテム・プロジェクトの候補を返す"},
		func(ctx context.Context, _ *struct{}) (*indexOutput, error) {
			idx, err := svc.Index(ctx)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &indexOutput{Body: toIndex(idx)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "list-recent-items", Method: http.MethodGet, Path: "/api/recent-items", Summary: "最近開いたアイテムを新しい順に返す"},
		func(ctx context.Context, _ *struct{}) (*recentOutput, error) {
			rs, err := svc.Recent(ctx)
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &recentOutput{Body: []RecentItem{}}
			for _, r := range rs {
				out.Body = append(out.Body, RecentItem{StoreID: string(r.Store), ItemID: string(r.Item), Display: r.Display, Title: r.Title, ProjectTitle: r.ProjectTitle})
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "open-recent-item", Method: http.MethodPost, Path: "/api/recent-items", Summary: "アイテムを開いたと、このマシンに覚える", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *openedInput) (*struct{}, error) {
			if err := svc.Opened(ctx, port.StoreID(in.Body.StoreID), change.ID(in.Body.ItemID)); err != nil {
				return nil, apierr.From(err)
			}
			return nil, nil
		})
}

func toIndex(idx apppalette.Index) PaletteIndex {
	out := PaletteIndex{Items: []PaletteItem{}, Projects: []PaletteProject{}}
	for _, it := range idx.Items {
		out.Items = append(out.Items, PaletteItem{
			StoreID: string(it.Store), WorkspaceID: string(it.Workspace), ProjectID: string(it.Project), ItemID: string(it.Item),
			Number: it.Number, Display: it.Display, Title: it.Title, ProjectTitle: it.ProjectTitle, Aliases: toAliases(it.Aliases),
		})
	}
	for _, p := range idx.Projects {
		out.Projects = append(out.Projects, PaletteProject{
			StoreID: string(p.Store), WorkspaceID: string(p.Workspace), ProjectID: string(p.Project),
			Display: p.Display, Title: p.Title, WorkspaceName: p.WorkspaceName, Aliases: toAliases(p.Aliases),
		})
	}
	return out
}

func toAliases(as []transfer.Alias) []PaletteAlias {
	out := []PaletteAlias{}
	for _, a := range as {
		pa := PaletteAlias{ID: a.ID}
		if a.At != nil {
			s := a.At.Format(change.TimeLayout)
			pa.At = &s
		}
		out = append(out, pa)
	}
	return out
}
