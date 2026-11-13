package planning

import (
	"context"
	"errors"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// errSameRef は、アイテムのスプリントかリリースがすでに同じで保存しないことを伝える。
var errSameRef = errors.New("アイテムの参照は変わらない")

// assignment は、アイテムのスプリントかリリースを変える保存の中身。
type assignment struct {
	label  string // 「スプリント」か「リリース」
	name   string // 移動先の名前、または「バックログ」「なし」
	drafts []change.Draft
}

// SetSprint は、アイテム id のスプリントを sprint(nil ならバックログ)にする(F1-11・F1-15・F1-16)。
// 同じ保存で、rank を移動先の枠の末尾にする。枠が空なら rank は変えない。
func (s Service) SetSprint(ctx context.Context, store port.StoreID, id item.ID, sprint *change.ID) error {
	return s.assign(ctx, store, id, func(r port.Reader, it item.Item, p planning.Plan) (assignment, error) {
		if !planning.IsWork(it.Kind) {
			return assignment{}, fmt.Errorf("%w: %s はスプリントに入れられない", port.ErrInvalidOperation, it.Kind)
		}
		a := assignment{label: "スプリント", name: "バックログ"}
		var to change.ID
		if sprint != nil {
			sp, err := sprintFor(p, it, *sprint)
			if err != nil {
				return assignment{}, err
			}
			to, a.name = sp.ID, oneLine(sp.Name)
		}
		if to == it.Sprint {
			return assignment{}, errSameRef
		}
		end, err := frameEnd(r, it, to, p)
		if err != nil {
			return assignment{}, err
		}
		a.drafts = []change.Draft{change.NewSet(item.SprintField, refValue(it.Sprint), refValue(to))}
		if end != "" {
			a.drafts = append(a.drafts, change.NewSet("rank", it.Rank, rank.After(end)))
		}
		return a, nil
	})
}

// SetRelease は、アイテム id のリリースを release(nil ならなし)にする(F1-15・F1-16)。rank は変えない。
func (s Service) SetRelease(ctx context.Context, store port.StoreID, id item.ID, release *change.ID) error {
	return s.assign(ctx, store, id, func(_ port.Reader, it item.Item, p planning.Plan) (assignment, error) {
		if it.Kind == item.Step {
			return assignment{}, fmt.Errorf("%w: Step はリリースに入れられない", port.ErrInvalidOperation)
		}
		a := assignment{label: "リリース", name: "なし"}
		var to change.ID
		if release != nil {
			r, ok := p.UsableRelease(*release)
			if !ok {
				return assignment{}, port.ErrNotFound
			}
			if r.State == planning.Archived && it.Release != r.ID {
				return assignment{}, fmt.Errorf("%w: アーカイブのリリースには入れられない", port.ErrInvalidOperation)
			}
			to, a.name = r.ID, oneLine(r.Name)
		}
		if to == it.Release {
			return assignment{}, errSameRef
		}
		a.drafts = []change.Draft{change.NewSet(item.ReleaseField, refValue(it.Release), refValue(to))}
		return a, nil
	})
}

// sprintFor は、アイテム it を入れられるスプリント id。削除済みかなければ ErrNotFound。
// 完了したスプリントは、いま入っているときだけ返す。
func sprintFor(p planning.Plan, it item.Item, id change.ID) (planning.Sprint, error) {
	sp, ok := p.UsableSprint(id)
	if !ok {
		return planning.Sprint{}, port.ErrNotFound
	}
	if sp.State == planning.Closed && it.Sprint != sp.ID {
		return planning.Sprint{}, refusal.New("完了したスプリントには入れられません")
	}
	return sp, nil
}

// frameEnd は、アイテム it を除いた、移動先の枠(sprint が空ならバックログ)の末尾の rank。
func frameEnd(r port.Reader, it item.Item, sprint change.ID, p planning.Plan) (string, error) {
	ws := it.Place.Workspace
	st, err := r.Workspaces().Settings(ws)
	if err != nil {
		return "", err
	}
	all, err := works(r, ws)
	if err != nil {
		return "", err
	}
	others := make([]planning.Work, 0, len(all))
	for _, w := range all {
		if w.Item.ID != it.ID {
			others = append(others, w)
		}
	}
	return planning.FrameEnd(others, ws, sprint, p, st), nil
}

// refValue は、参照の値。空は null。
func refValue(id change.ID) any {
	if id == "" {
		return nil
	}
	return id
}

// assign は、decide が決めた変更を、アクティビティの文「{ID} の {label} を {name} に変更」で 1 回保存する。
// decide は保存の前と保存の中の両方で、そのときのアイテムと計画を見て呼ぶ。同じ値なら何も保存しない。
func (s Service) assign(ctx context.Context, store port.StoreID, id item.ID,
	decide func(r port.Reader, it item.Item, p planning.Plan) (assignment, error),
) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	var text string
	err = s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		text, _, err = assignmentOf(r, id, decide)
		return err
	})
	if err == nil {
		err = s.Stores.Write(ctx, store, port.Operation{Actor: change.User(me), Summary: text}, func(tx port.Tx) error {
			_, ds, err := assignmentOf(tx, id, decide)
			if err != nil {
				return err
			}
			return tx.WriteItems().Append(id, ds...)
		})
	}
	if errors.Is(err, errSameRef) {
		return nil
	}
	return err
}

// assignmentOf は、アクティビティの文と、書く行。
func assignmentOf(r port.Reader, id item.ID,
	decide func(r port.Reader, it item.Item, p planning.Plan) (assignment, error),
) (string, []change.Draft, error) {
	it, err := port.LiveItem(r, id)
	if err != nil {
		return "", nil, err
	}
	w, err := r.Workspaces().Get(it.Place.Workspace)
	if err != nil {
		return "", nil, err
	}
	p, err := planOf(r, it.Place.Workspace)
	if err != nil {
		return "", nil, err
	}
	a, err := decide(r, it, p)
	if err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("%s の %s を %s に変更", it.DisplayID(w.Prefix), a.label, a.name), a.drafts, nil
}
