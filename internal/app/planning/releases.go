package planning

import (
	"context"
	"errors"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ReleaseInput は、リリースの作成・編集の入力。予定日は YYYY-MM-DD か空。
type ReleaseInput struct{ Name, Date, Description string }

func (in ReleaseInput) release(id change.ID, state planning.ReleaseState) planning.Release {
	return planning.Release{
		ID: id, Name: planning.NormalizeName(in.Name), State: state,
		Date: in.Date, Description: in.Description,
	}
}

// releaseStateLabel は、アクティビティの文に入れる状態の呼び名。
var releaseStateLabel = map[planning.ReleaseState]string{
	planning.Unreleased: "未リリース",
	planning.Released:   "リリース済み",
	planning.Archived:   "アーカイブ",
}

// CreateRelease は、未リリースのリリースを作る。
func (s Service) CreateRelease(ctx context.Context, store port.StoreID, ws workspace.ID, in ReleaseInput) (planning.Release, error) {
	r := in.release(s.IDs.New(), planning.Unreleased)
	err := s.editPlan(ctx, store, ws, revival{workspace: true},
		func(planning.Plan) (string, error) {
			return fmt.Sprintf("リリース「%s」を作成", oneLine(r.Name)), nil
		},
		func(p planning.Plan) ([]change.Draft, error) {
			if err := planning.CheckRelease(r, p.LiveReleases()); err != nil {
				return nil, err
			}
			return planning.ReleaseDrafts(planning.Release{}, r), nil
		})
	if err != nil {
		return planning.Release{}, err
	}
	return r, nil
}

// UpdateRelease は、リリースの名前・予定日・説明を直す。状態は変えない。
func (s Service) UpdateRelease(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, in ReleaseInput) error {
	name := planning.NormalizeName(in.Name)
	return s.editPlan(ctx, store, ws, revival{workspace: true, kind: releaseKind, id: id, name: &name},
		func(p planning.Plan) (string, error) {
			if _, ok := p.UsableRelease(id); !ok {
				return "", port.ErrNotFound
			}
			return fmt.Sprintf("リリース「%s」を編集", oneLine(name)), nil
		},
		func(p planning.Plan) ([]change.Draft, error) {
			old, ok := p.UsableRelease(id)
			if !ok {
				return nil, port.ErrNotFound
			}
			r := in.release(id, old.State)
			if err := planning.CheckRelease(r, p.LiveReleases()); err != nil {
				return nil, err
			}
			return planning.ReleaseDrafts(old, r), nil
		})
}

// SetReleaseState は、リリースの状態を変える。同じ状態なら何も保存しない。
// リリース済みにするとき予定日が空なら、今日を同じ保存で入れる。確認は画面が出す。
func (s Service) SetReleaseState(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, to planning.ReleaseState) error {
	label, known := releaseStateLabel[to]
	if !known {
		return fmt.Errorf("%w: release state %q", port.ErrInvalidOperation, to)
	}
	err := s.editPlan(ctx, store, ws, revival{workspace: true, kind: releaseKind, id: id},
		func(p planning.Plan) (string, error) {
			old, ok := p.UsableRelease(id)
			if !ok {
				return "", port.ErrNotFound
			}
			if old.State == to {
				return "", errNoRows
			}
			return fmt.Sprintf("リリース「%s」を%sにする", oneLine(old.Name), label), nil
		},
		func(p planning.Plan) ([]change.Draft, error) {
			old, ok := p.UsableRelease(id)
			if !ok {
				return nil, port.ErrNotFound
			}
			r := old
			r.State = to
			if to == planning.Released && r.Date == "" {
				r.Date = planning.Today(s.Clock.Now())
			}
			return planning.ReleaseDrafts(old, r), nil
		})
	if errors.Is(err, errNoRows) {
		return nil
	}
	return err
}
