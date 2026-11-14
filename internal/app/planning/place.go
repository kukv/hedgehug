package planning

import (
	"context"
	"errors"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// errOtherFrame は、基準の行なしで別の枠へ移すこと(SetSprint に任せる)を伝える。
var errOtherFrame = errors.New("基準なしで別の枠へ移す")

// PlaceItem は、作業項目 id を、バックログの画面の枠 sprint(nil ならバックログ)の行 anchor の直前(after なら直後)に
// 置く(F2a-3・F2a-4)。枠が変わるなら同じ保存でスプリントも変える。アクティビティの文は「{ID} の順位を変更」。
// anchor が nil なら枠の末尾に置く。枠が変わるときは SetSprint と同じ。同じ位置なら何も保存しない。
func (s Service) PlaceItem(ctx context.Context, store port.StoreID, id item.ID, sprint *change.ID, anchor *item.ID, after bool) error {
	err := s.assign(ctx, store, id, func(r port.Reader, it item.Item, p planning.Plan) (assignment, error) {
		return placement(r, it, p, sprint, anchor, after)
	})
	if errors.Is(err, errOtherFrame) {
		return s.SetSprint(ctx, store, id, sprint)
	}
	return err
}

// placement は、PlaceItem の保存の行。
func placement(r port.Reader, it item.Item, p planning.Plan, sprint *change.ID, anchor *item.ID, after bool) (assignment, error) {
	to, _, err := sprintTarget(it, p, sprint)
	if err != nil {
		return assignment{}, err
	}
	same := sameFrame(it, p, to)
	if anchor == nil && !same {
		return assignment{}, errOtherFrame
	}
	ws := it.Place.Workspace
	st, err := r.Workspaces().Settings(ws)
	if err != nil {
		return assignment{}, err
	}
	its, err := items(r, ws)
	if err != nil {
		return assignment{}, err
	}
	rows := frameRows(its, ws, p, st, to)
	var at string
	if anchor == nil {
		at, err = frameTail(its, it, to, p, st, rows)
	} else {
		at, err = beside(its, it, rows, same, *anchor, after)
	}
	if err != nil {
		return assignment{}, err
	}
	var a assignment
	if !same {
		a.drafts = append(a.drafts, change.NewSet(item.SprintField, refValue(it.Sprint), refValue(to)))
	}
	a.drafts = append(a.drafts, change.NewSet("rank", it.Rank, at))
	return a, nil
}

// frameRows は、バックログの画面の枠 to(空ならバックログ)の行の ID を、並び順に。枠がなければ空。
func frameRows(its []item.Item, ws workspace.ID, p planning.Plan, st workspace.Settings, to change.ID) []item.ID {
	var out []item.ID
	for _, f := range planning.BacklogFrames(its, ws, p, st) {
		if f.Sprint == nil && to == "" || f.Sprint != nil && f.Sprint.ID == to {
			for _, it := range f.Items {
				out = append(out, it.ID)
			}
		}
	}
	return out
}

// beside は、同じ枠 rows の行 anchor の直前(after なら直後)の順位(F2a-2)。同じ位置なら errSameRef、
// anchor が rows にないなら port.ErrNotFound。
func beside(its []item.Item, it item.Item, rows []item.ID, same bool, anchor item.ID, after bool) (string, error) {
	if same && samePosition(rows, it.ID, anchor, after) {
		return "", errSameRef
	}
	if !slices.Contains(rows, anchor) {
		return "", port.ErrNotFound
	}
	at, ok := planning.Placement(its, it.Place.Workspace, it.ID, anchor, after)
	if !ok {
		return "", port.ErrNotFound
	}
	return at, nil
}

// samePosition は、rows の中で id を anchor の直前(after なら直後)に置いても位置が変わらないか(F2a-4)。
func samePosition(rows []item.ID, id, anchor item.ID, after bool) bool {
	if id == anchor {
		return true
	}
	i, j := slices.Index(rows, id), slices.Index(rows, anchor)
	if i < 0 || j < 0 {
		return false
	}
	if after {
		return j == i-1
	}
	return j == i+1
}

// frameTail は、同じ枠 to の末尾の順位(rank.After(自分を除いた FrameEnd))。すでに末尾か、ほかに行がなければ errSameRef。
func frameTail(its []item.Item, it item.Item, to change.ID, p planning.Plan, st workspace.Settings, rows []item.ID) (string, error) {
	if len(rows) > 0 && rows[len(rows)-1] == it.ID {
		return "", errSameRef
	}
	others := slices.DeleteFunc(slices.Clone(its), func(o item.Item) bool { return o.ID == it.ID })
	end := planning.FrameEnd(others, it.Place.Workspace, to, p, st)
	if end == "" {
		return "", errSameRef
	}
	return rank.After(end), nil
}
