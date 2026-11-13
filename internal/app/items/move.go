package items

import (
	"context"
	"errors"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// MoveInput は、移す先。Parent か Project のどちらか一方を選ぶ。Epic はほかのプロジェクト(Project)、
// それ以外は新しい親(Parent)。Statuses は、プロジェクトが変わるときの、移動先が使わないステータス →
// 置き換え先(B5a-19)。指定のない使わないステータスは、同じ分類の移動先の既定になる。
type MoveInput struct {
	Parent   *item.ID
	Project  *project.ID
	Statuses map[change.ID]change.ID
}

// destination は、移したあとの場所。parent は新しい親(Epic の移動では nil)。
type destination struct {
	project project.Project
	parent  *item.Item
}

// Move は、同じワークスペースの中でアイテムを移す(02「同じワークスペースの中での移動」、A4b-11)。移すアイテムは
// 新しい親の子の末尾に入る。プロジェクトが変わるときは、下のものすべて(削除済みを含む)も同じプロジェクトへ移す。
// 番号・下のものの親子・プロパティ・関連は変えない。いまの親(Epic ならプロジェクト)と同じ、または自分自身の下なら
// 何も保存しない。アクティビティは、プロジェクトが変わるとき(Epic の移動を含む)は「{ID} を {プロジェクト名} に移動」、
// 変わらないときは「{ID} を {ID} の下に移動」(A4b-13)。
// 親が置けない種類なら 02「アイテムの種類と親子」の文言で断る。Parent と Project の両方・どちらもなし、
// ないものや削除済みのもの、別のワークスペースのものは port.ErrNotFound(別のワークスペースへの移動は A5)。
func (s Service) Move(ctx context.Context, store port.StoreID, id item.ID, in MoveInput) error {
	if (in.Parent == nil) == (in.Project == nil) {
		return port.ErrNotFound
	}
	summary, err := s.moveSummary(ctx, store, id, in)
	if err != nil || summary == "" {
		return err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: summary}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		it, err := port.LiveItem(tx, id)
		if err != nil {
			return err
		}
		to, move, err := destinationOf(tx, it, in)
		if err != nil || !move {
			return err
		}
		return writeMove(tx, it, to, in.Statuses)
	})
}

// moveSummary は、アクティビティの 1 行。保存の前に、保存と同じ検査をして読んで決める(A4b-13)。
// 何もしない移動なら空。
func (s Service) moveSummary(ctx context.Context, store port.StoreID, id item.ID, in MoveInput) (string, error) {
	var summary string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := port.LiveItem(r, id)
		if err != nil {
			return err
		}
		to, move, err := destinationOf(r, it, in)
		if err != nil || !move {
			return err
		}
		w, err := r.Workspaces().Get(it.Place.Workspace)
		if err != nil {
			return err
		}
		if to.parent == nil || to.project.ID != it.Place.Project {
			summary = fmt.Sprintf("%s を %s に移動", it.DisplayID(w.Prefix), item.NormalizeTitle(to.project.Title))
		} else {
			summary = fmt.Sprintf("%s を %s の下に移動", it.DisplayID(w.Prefix), to.parent.DisplayID(w.Prefix))
		}
		return nil
	})
	return summary, err
}

// destinationOf は、移す先を検査して決める。何もしない移動(いまと同じ・自分自身の下)は move=false。
func destinationOf(r port.Reader, it item.Item, in MoveInput) (to destination, move bool, err error) {
	if in.Parent != nil {
		return parentDestination(r, it, *in.Parent)
	}
	if it.Kind != item.Epic {
		return destination{}, false, refusal.New(item.PlaceMessage(it.Kind, nil))
	}
	pr, err := port.LiveProject(r, *in.Project)
	if err != nil {
		return destination{}, false, err
	}
	if pr.Place.Workspace != it.Place.Workspace {
		return destination{}, false, port.ErrNotFound
	}
	return destination{project: pr}, pr.ID != it.Place.Project, nil
}

// parentDestination は、親を選んだときの移す先。子孫の下へ動かす検査は書かない。子孫は親より下の種類
// (Epic の下は Story・Task・Bug、その下は Step)で、どれも CanPlace が断る。
func parentDestination(r port.Reader, it item.Item, parent item.ID) (destination, bool, error) {
	if parent == it.ID {
		return destination{}, false, nil // 自分自身の下は何も起きない(02)
	}
	p, err := port.LiveItem(r, parent)
	if err != nil {
		return destination{}, false, err
	}
	if p.Place.Workspace != it.Place.Workspace {
		return destination{}, false, port.ErrNotFound
	}
	if !item.CanPlace(it.Kind, &p.Kind) {
		return destination{}, false, refusal.New(item.PlaceMessage(it.Kind, &p.Kind))
	}
	if it.Place.Parent != nil && *it.Place.Parent == parent {
		return destination{}, false, nil
	}
	pr, err := port.LiveProject(r, p.Place.Project)
	if err != nil {
		return destination{}, false, err
	}
	return destination{project: pr, parent: &p}, true, nil
}

// writeMove は、移すアイテムの place と rank(新しい親の子の末尾)、プロジェクトが変わるときは下のものの place を、
// 同じ操作で書く(A4b-3・11)。プロジェクトが変わるときは、移動先が使わないステータスの置き換えも同じ操作で書く(B5a-19)。
func writeMove(tx port.Tx, it item.Item, to destination, picks map[change.ID]change.ID) error {
	siblings, err := port.LiveItems(tx, to.project.ID)
	if err != nil {
		return err
	}
	var below []item.Item
	if to.project.ID != it.Place.Project {
		// 一緒に移す下のものは、削除済みも含めて読む(A4b-3)。
		old, err := tx.Items().List(it.Place.Project)
		if err != nil {
			return err
		}
		below = item.Descendants(it.ID, old)
	}
	replaced, err := statusReplacements(tx, append([]item.Item{it}, below...), to.project.ID, picks)
	if err != nil {
		return err
	}
	place := it.Place
	place.Project, place.Parent = to.project.ID, nil
	if to.parent != nil {
		place.Parent = &to.parent.ID
	}
	errs := []error{
		tx.WriteItems().Append(it.ID,
			change.NewSet("place", it.Place, place),
			change.NewSet("rank", it.Rank, rank.After(lastRank(siblings, place.Parent)))),
	}
	for _, d := range below {
		moved := d.Place
		moved.Project = to.project.ID
		errs = append(errs, tx.WriteItems().Append(d.ID, change.NewSet("place", d.Place, moved)))
	}
	for _, d := range replaced {
		errs = append(errs, tx.WriteItems().Append(d.id, change.NewSet("status", d.from, d.to)))
	}
	return errors.Join(errs...)
}
