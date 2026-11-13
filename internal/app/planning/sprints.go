package planning

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// SprintInput は、スプリントの作成・編集の入力。日付は YYYY-MM-DD か空。
type SprintInput struct{ Name, Start, End, Goal string }

func (in SprintInput) sprint(id change.ID, state planning.SprintState) planning.Sprint {
	return planning.Sprint{
		ID: id, Name: planning.NormalizeName(in.Name), State: state,
		Start: in.Start, End: in.End, Goal: in.Goal,
	}
}

// CreateSprint は、計画中のスプリントを作る。
func (s Service) CreateSprint(ctx context.Context, store port.StoreID, ws workspace.ID, in SprintInput) (planning.Sprint, error) {
	sp := in.sprint(s.IDs.New(), planning.Planned)
	err := s.editPlan(ctx, store, ws,
		func(planning.Plan) (string, error) {
			return fmt.Sprintf("スプリント「%s」を作成", oneLine(sp.Name)), nil
		},
		func(p planning.Plan) ([]change.Draft, error) {
			if err := planning.CheckSprint(sp, p.LiveSprints(), planning.Today(s.Clock.Now())); err != nil {
				return nil, err
			}
			return planning.SprintDrafts(planning.Sprint{}, sp), nil
		})
	if err != nil {
		return planning.Sprint{}, err
	}
	return sp, nil
}

// UpdateSprint は、スプリントの名前・日付・目標を直す。状態は変えない。
func (s Service) UpdateSprint(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, in SprintInput) error {
	name := planning.NormalizeName(in.Name)
	return s.editPlan(ctx, store, ws,
		func(p planning.Plan) (string, error) {
			if _, ok := p.UsableSprint(id); !ok {
				return "", port.ErrNotFound
			}
			return fmt.Sprintf("スプリント「%s」を編集", oneLine(name)), nil
		},
		func(p planning.Plan) ([]change.Draft, error) {
			old, ok := p.UsableSprint(id)
			if !ok {
				return nil, port.ErrNotFound
			}
			sp := in.sprint(id, old.State)
			sp.Confirmed = old.Confirmed
			if err := planning.CheckSprint(sp, p.LiveSprints(), planning.Today(s.Clock.Now())); err != nil {
				return nil, err
			}
			return planning.SprintDrafts(old, sp), nil
		})
}

// StartSprint は、計画中のスプリントを実行中にする。計画中以外は ErrNotFound。
func (s Service) StartSprint(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, start, end string) error {
	return s.editPlan(ctx, store, ws,
		func(p planning.Plan) (string, error) {
			old, ok := p.UsableSprint(id)
			if !ok || old.State != planning.Planned {
				return "", port.ErrNotFound
			}
			return fmt.Sprintf("スプリント「%s」を開始", oneLine(old.Name)), nil
		},
		func(p planning.Plan) ([]change.Draft, error) {
			old, ok := p.UsableSprint(id)
			if !ok || old.State != planning.Planned {
				return nil, port.ErrNotFound
			}
			for _, o := range p.LiveSprints() {
				if o.State == planning.Active {
					return nil, refusal.New("実行中のスプリントがあります。先に完了してください")
				}
			}
			if err := planning.CheckStart(start, end, planning.Today(s.Clock.Now())); err != nil {
				return nil, err
			}
			sp := old
			sp.State, sp.Start, sp.End = planning.Active, start, end
			return planning.SprintDrafts(old, sp), nil
		})
}
