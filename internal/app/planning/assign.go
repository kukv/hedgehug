package planning

import (
	"cmp"
	"context"
	"errors"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/rank"
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
// 同じ保存で、rank を移動先の枠の末尾にする。枠が空なら rank は変えない。使えないスプリントへの参照は
// バックログと同じ値として扱い、書き換えない(F1-5。スプリントを戻せばアイテムも戻る)。
func (s Service) SetSprint(ctx context.Context, store port.StoreID, id item.ID, sprint *change.ID) error {
	return s.assign(ctx, store, id, func(r port.Reader, it item.Item, p planning.Plan) (assignment, error) {
		if !planning.IsWork(it.Kind) {
			return assignment{}, fmt.Errorf("%w: %s はスプリントに入れられない", port.ErrInvalidOperation, it.Kind)
		}
		a := assignment{label: "スプリント", name: "バックログ"}
		var to change.ID
		if sprint != nil {
			sp, err := planning.AssignableSprint(p, *sprint, it.Kind, it.Sprint)
			if err != nil {
				return assignment{}, sprintErr(err)
			}
			to, a.name = sp.ID, oneLine(sp.Name)
		}
		if _, ok := p.UsableSprint(it.Sprint); to == it.Sprint || to == "" && !ok {
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
// 使えないリリースへの参照は「なし」と同じ値として扱い、書き換えない(F1-5)。
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
		if _, ok := p.UsableRelease(it.Release); to == it.Release || to == "" && !ok {
			return assignment{}, errSameRef
		}
		a.drafts = []change.Draft{change.NewSet(item.ReleaseField, refValue(it.Release), refValue(to))}
		return a, nil
	})
}

// sprintErr は、planning.AssignableSprint の断りでないエラーを、保存の口のエラーにする。
func sprintErr(err error) error {
	switch {
	case errors.Is(err, planning.ErrNotWork):
		return fmt.Errorf("%w: %w", port.ErrInvalidOperation, err)
	case errors.Is(err, planning.ErrNoSprint):
		return port.ErrNotFound
	}
	return err
}

// frameEnd は、移動先の枠(sprint が空ならバックログ)の末尾の rank。アイテム it は移動先の枠に入っていない
// (同じ参照は保存しない)。
func frameEnd(r port.Reader, it item.Item, sprint change.ID, p planning.Plan) (string, error) {
	ws := it.Place.Workspace
	st, err := r.Workspaces().Settings(ws)
	if err != nil {
		return "", err
	}
	its, err := items(r, ws)
	if err != nil {
		return "", err
	}
	return planning.FrameEnd(its, ws, sprint, p, st), nil
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
// 別の画面で削除された(root: true)アイテムは、保存するときだけ削除を取り消す(C3a-screen S1)。
func (s Service) assign(ctx context.Context, store port.StoreID, id item.ID,
	decide func(r port.Reader, it item.Item, p planning.Plan) (assignment, error),
) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	var text string
	err = s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := port.EditableItem(r, id)
		if err != nil {
			return err
		}
		text, _, err = assignmentOf(r, it, decide)
		if errors.Is(err, errSameRef) {
			return cmp.Or(port.Unchanged(it.Deleted), err)
		}
		return err
	})
	if err == nil {
		err = s.Stores.Write(ctx, store, port.Operation{Actor: change.User(me), Summary: text}, func(tx port.Tx) error {
			return reviveAndAssign(ctx, tx, id, decide)
		})
	}
	if errors.Is(err, errSameRef) {
		return nil
	}
	return err
}

// reviveAndAssign は、保存の中で最初に、削除済み(root: true)なら削除を取り消してから、いまのアイテムを読み直し、
// decide が決めた行を書く。同じ値なら、削除の印があれば port.ErrNotFound で断る(決定 S4・S10)。
func reviveAndAssign(ctx context.Context, tx port.Tx, id item.ID,
	decide func(r port.Reader, it item.Item, p planning.Plan) (assignment, error),
) error {
	editable, err := port.EditableItem(tx, id)
	if err != nil {
		return err
	}
	if err := port.ReviveItem(ctx, tx, id); err != nil {
		return err
	}
	it, err := port.LiveItem(tx, id)
	if err != nil {
		return err
	}
	_, ds, err := assignmentOf(tx, it, decide)
	if errors.Is(err, errSameRef) {
		return cmp.Or(port.Unchanged(editable.Deleted), err)
	}
	if err != nil {
		return err
	}
	return tx.WriteItems().Append(id, ds...)
}

// assignmentOf は、アイテム it のアクティビティの文と、書く行。
func assignmentOf(r port.Reader, it item.Item,
	decide func(r port.Reader, it item.Item, p planning.Plan) (assignment, error),
) (string, []change.Draft, error) {
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
