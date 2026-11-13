package projects

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// DateField は、日付の項目。値はログの項目名と同じ。
type DateField string

const (
	StartDate DateField = "start"
	EndDate   DateField = "end"
)

// SetDate は、開始日か終了日を 1 つ変える(02「概要の編集」)。value は YYYY-MM-DD か空(なし)。
// 保存の中で読み直したもう片方と比べ、終了日が開始日より前なら断る。前と同じなら何も保存しない。
func (s Service) SetDate(ctx context.Context, store port.StoreID, id project.ID, field DateField, value string) (project.Project, error) {
	name, ok := map[DateField]string{StartDate: "開始日", EndDate: "終了日"}[field]
	if !ok {
		return project.Project{}, fmt.Errorf("unknown date field %q", field)
	}
	p, err := s.editable(ctx, store, id)
	if err != nil {
		return project.Project{}, err
	}
	summary := fmt.Sprintf("%s の%sを %s に変更", label(p.Title), name, project.DateText(value, s.Clock.Now()))
	return s.save(ctx, store, id, summary, func(_ port.Tx, cur project.Project) ([]change.Draft, error) {
		from, start, end := cur.Start, value, cur.End
		if field == EndDate {
			from, start, end = cur.End, cur.Start, value
		}
		if from == value {
			return nil, nil
		}
		if err := project.CheckDates(start, end); err != nil {
			return nil, err
		}
		return []change.Draft{change.NewSet(string(field), from, value)}, nil
	})
}
