package items

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

// ChangeKind は、アイテムの種類を変える(02「種類の変更」)。
// Story・Task・Bug は自分以外の Story・Task・Bug へ、Step は Story・Task・Bug へ変えられる。Step からは、
// 親の親(Epic)の子の末尾へ place と rank も同じ操作で書く(番号は変えない)。ステータスやプロパティの値は変えず、
// ワークフローの制約も検査しない。アクティビティは「{ID} の種類を {種類} に変更」。
// Epic、Epic・Step への変更、いまと同じ種類への変更は、画面から送らないので port.ErrNotFound。
func (s Service) ChangeKind(ctx context.Context, store port.StoreID, id item.ID, kind item.Kind) error {
	if kind != item.Story && kind != item.Task && kind != item.Bug {
		return port.ErrNotFound
	}
	_, display, err := s.withID(ctx, store, id)
	if err != nil {
		return err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("%s の種類を %s に変更", display, kind)}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		it, err := port.LiveItem(tx, id)
		if err != nil {
			return err
		}
		if it.Kind == item.Epic || it.Kind == kind {
			return port.ErrNotFound
		}
		drafts := []change.Draft{change.NewSet("kind", string(it.Kind), string(kind))}
		if it.Kind == item.Step {
			promote, err := promotion(tx, it)
			if err != nil {
				return err
			}
			drafts = append(drafts, promote...)
		}
		return tx.WriteItems().Append(id, drafts...)
	})
}

// promotion は、Step を親の親(Epic)の子の末尾へ移す place と rank。
func promotion(tx port.Tx, step item.Item) ([]change.Draft, error) {
	if step.Place.Parent == nil {
		return nil, port.ErrNotFound
	}
	parent, err := port.LiveItem(tx, *step.Place.Parent)
	if err != nil {
		return nil, err
	}
	if parent.Place.Parent == nil {
		return nil, port.ErrNotFound
	}
	epic, err := port.LiveItem(tx, *parent.Place.Parent)
	if err != nil {
		return nil, err
	}
	live, err := port.LiveItems(tx, step.Place.Project)
	if err != nil {
		return nil, err
	}
	place := step.Place
	place.Parent = &epic.ID
	return []change.Draft{
		change.NewSet("place", step.Place, place),
		change.NewSet("rank", step.Rank, rank.After(lastRank(live, &epic.ID))),
	}, nil
}
