package workspaces

import (
	"context"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

var (
	statusList   = workspace.List{Kind: workspace.ListStatuses}
	priorityList = workspace.List{Kind: workspace.ListPriorities}
)

// SetStatusCategory は、ステータスの分類を変える。知らない分類は ErrNotFound、同じなら何もしない。
func (s Service) SetStatusCategory(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, c workspace.Category) error {
	if !slices.Contains(workspace.Categories, c) {
		return port.ErrNotFound
	}
	return s.editValue(ctx, store, ws, statusList, id,
		func(_ workspace.Settings, v workspace.Value) string {
			return fmt.Sprintf("ステータス「%s」の分類を変更", oneLine(v.Name))
		},
		func(_ port.Tx, st workspace.Settings, _ []workspace.Value, v workspace.Value) ([]change.Draft, error) {
			if err := workspace.CheckStatusCategory(st, id, c); err != nil {
				return nil, err
			}
			if v.Category == c {
				return nil, nil
			}
			return []change.Draft{change.NewSet(statusList.Field(id, "category"), v.Category, c)}, nil
		})
}

// SetInitialStatus は、初期ステータスを変える。いまと同じなら何もしない。
func (s Service) SetInitialStatus(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID) error {
	return s.editValue(ctx, store, ws, statusList, id,
		func(_ workspace.Settings, v workspace.Value) string {
			return fmt.Sprintf("ステータス「%s」を初期ステータスに設定", oneLine(v.Name))
		},
		func(_ port.Tx, st workspace.Settings, _ []workspace.Value, _ workspace.Value) ([]change.Draft, error) {
			if err := workspace.CheckInitialStatus(st, id); err != nil {
				return nil, err
			}
			if st.InitialStatus == id {
				return nil, nil
			}
			return []change.Draft{change.NewSet("initial_status", st.InitialStatus, id)}, nil
		})
}

// SetInitialPriority は、初期優先度を変える。いまと同じなら何もしない。
func (s Service) SetInitialPriority(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID) error {
	return s.editValue(ctx, store, ws, priorityList, id,
		func(_ workspace.Settings, v workspace.Value) string {
			return fmt.Sprintf("優先度「%s」を初期優先度に設定", oneLine(v.Name))
		},
		func(_ port.Tx, st workspace.Settings, _ []workspace.Value, _ workspace.Value) ([]change.Draft, error) {
			if st.InitialPriority == id {
				return nil, nil
			}
			return []change.Draft{change.NewSet("initial_priority", st.InitialPriority, id)}, nil
		})
}

// SetCategoryDefault は、分類 c の既定のステータスを変える。分類の違うステータスは ErrNotFound、同じなら何もしない。
func (s Service) SetCategoryDefault(ctx context.Context, store port.StoreID, ws workspace.ID, c workspace.Category, id change.ID) error {
	return s.editValue(ctx, store, ws, statusList, id,
		func(_ workspace.Settings, v workspace.Value) string {
			return fmt.Sprintf("分類「%s」の既定を「%s」に変更", workspace.CategoryName(c), oneLine(v.Name))
		},
		func(_ port.Tx, st workspace.Settings, _ []workspace.Value, v workspace.Value) ([]change.Draft, error) {
			if v.Category != c {
				return nil, port.ErrNotFound
			}
			cur := st.CategoryDefaults[c]
			if cur == id {
				return nil, nil
			}
			return []change.Draft{change.NewSet("category_default/"+string(c), cur, id)}, nil
		})
}
