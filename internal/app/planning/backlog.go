package planning

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// BacklogRow は、バックログの画面の枠の 1 行(F2a-7)。Project は、ステータスのメニューを絞るのに使う。
type BacklogRow struct {
	Row
	Priority change.ID
	Project  project.ID
}

// BacklogFrame は、バックログの画面の枠。Sprint が nil ならバックログの枠(F2a-1)。
type BacklogFrame struct {
	Sprint *planning.Sprint
	Total  planning.Totals
	Rows   []BacklogRow
}

// Backlog は、ワークスペース ws のバックログの画面の枠を読む。ワークスペースがないか削除済みなら port.ErrNotFound。
func (s Service) Backlog(ctx context.Context, store port.StoreID, ws workspace.ID) ([]BacklogFrame, error) {
	var out []BacklogFrame
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		p, its, st, prefix, err := cardSource(r, ws)
		if err != nil {
			return err
		}
		for _, f := range planning.BacklogFrames(its, ws, p, st) {
			bf := BacklogFrame{Sprint: f.Sprint, Total: f.Total}
			for _, it := range f.Items {
				bf.Rows = append(bf.Rows, BacklogRow{Row: row(it, prefix), Priority: it.Priority, Project: it.Place.Project})
			}
			out = append(out, bf)
		}
		return nil
	})
	return out, err
}
