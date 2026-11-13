package items

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// AddStep は、parent の子の末尾に Step を作る(02「子と Step」の「Step の追加」)。作り方は作成と同じで、
// アクティビティだけ「{親の ID} に Step「{タイトル}」を追加」にする。担当者は親と同じ(空なら入れない)。
// ステータスと優先度は作成の初期値で入る。
// 親が Story・Task・Bug でなければ、作成と同じ文言で断る。
func (s Service) AddStep(ctx context.Context, store port.StoreID, parent item.ID, title string) (item.Item, error) {
	p, display, err := s.withID(ctx, store, parent)
	if err != nil {
		return item.Item{}, err
	}
	in := CreateInput{Store: store, Project: p.Place.Project, Parent: &parent, Kind: item.Step, Title: title}
	if p.Assignee != "" {
		in.Values = map[change.ID]json.RawMessage{"assignee": change.Value(p.Assignee)}
	}
	return s.create(ctx, in, nil, fmt.Sprintf("%s に Step「%s」を追加", display, item.NormalizeTitle(title)))
}
