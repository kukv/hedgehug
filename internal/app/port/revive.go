package port

import (
	"context"
	"errors"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// 別の画面で削除されたものへの保存は、変更を残して削除を取り消す(06「他の書き手との重なり」)。
// 下の関数は、その取り消しの口。呼ぶ側は、先に EditableItem・EditableProject で断るものを断ってから Write に入る。

// EditableItem は、利用者が変更を保存できるアイテムを返す。削除されていないか、利用者が選んで削除した
// (Deleted.Root が true)アイテム。ないか、親と一緒に削除された(root: false)か、削除済みで祖先のアイテム
// (Place.Parent をたどる)かプロジェクトが削除済みなら ErrNotFound(決定 S9)。
func EditableItem(r Reader, id item.ID) (item.Item, error) {
	it, err := r.Items().Get(id)
	if err != nil {
		return item.Item{}, err
	}
	if it.Deleted == nil {
		return it, nil // 削除されていないものは、祖先が削除済みでも、いままでどおり変更できる(03「実行の規則」の Step など)
	}
	if !it.Deleted.Root {
		return item.Item{}, ErrNotFound
	}
	if err := aliveAbove(r, it); err != nil {
		return item.Item{}, err
	}
	return it, nil
}

// aliveAbove は、it の祖先のアイテムとプロジェクトがどれも削除済みでないことを確かめる。
func aliveAbove(r Reader, it item.Item) error {
	if _, err := LiveProject(r, it.Place.Project); err != nil {
		return err
	}
	seen := map[item.ID]bool{it.ID: true}
	for parent := it.Place.Parent; parent != nil && !seen[*parent]; parent = it.Place.Parent {
		seen[*parent] = true
		var err error
		if it, err = LiveItem(r, *parent); err != nil {
			return err
		}
	}
	return nil
}

// EditableProject は、利用者が変更を保存できるプロジェクトを返す。削除されていないか、利用者が選んで削除した
// (Deleted.Root が true)プロジェクト。ないか、root: false なら ErrNotFound。
func EditableProject(r Reader, id project.ID) (project.Project, error) {
	p, err := r.Projects().Get(id)
	if err != nil {
		return project.Project{}, err
	}
	if p.Deleted != nil && !p.Deleted.Root {
		return project.Project{}, ErrNotFound
	}
	return p, nil
}

// Unchanged は、削除の印 d が nil(削除されていない)なら nil、そうでなければ ErrNotFound を返す。
// 削除を取り消さない操作が、削除済みのものを断るのに使う(決定 S4)。
func Unchanged(d *change.Deletion) error {
	if d == nil {
		return nil
	}
	return ErrNotFound
}

// ReviveItem は、tx の中で読み直し、削除済み(root: true)のアイテムなら、その削除の操作の行を取り消す。
// 取り消すのは、そのプロジェクトのアイテムのうち、ログにその操作の行があるすべて。表示 ID は WithRevived の
// 入れ物に積む。削除されていないか、EditableItem を通らなければ何もしない。
func ReviveItem(ctx context.Context, tx Tx, id item.ID) error {
	it, err := EditableItem(tx, id)
	if errors.Is(err, ErrNotFound) || (err == nil && it.Deleted == nil) {
		return nil
	}
	if err != nil {
		return err
	}
	ws, err := tx.Workspaces().Get(it.Place.Workspace)
	if err != nil {
		return err
	}
	if err := reviveItemsOf(tx, it.Place.Project, it.Deleted.Op); err != nil {
		return err
	}
	noteRevived(ctx, it.DisplayID(ws.Prefix))
	return nil
}

// ReviveProject は、tx の中で読み直し、削除済み(root: true)のプロジェクトなら、その削除の操作の行を、
// プロジェクト自身とそのアイテムのログから取り消す。同じ名前の削除されていないプロジェクトがあれば断る(決定 S11)。
func ReviveProject(ctx context.Context, tx Tx, id project.ID) error {
	p, err := EditableProject(tx, id)
	if errors.Is(err, ErrNotFound) || (err == nil && p.Deleted == nil) {
		return nil
	}
	if err != nil {
		return err
	}
	ws, err := tx.Workspaces().Get(p.Place.Workspace)
	if err != nil {
		return err
	}
	live, err := LiveProjects(tx, p.Place.Workspace)
	if err != nil {
		return err
	}
	if err := project.CheckRevive(p.Title, live); err != nil {
		return err // 同じ名前の削除されていないプロジェクトがあれば、取り消さない(決定 S11)
	}
	log, err := tx.Changes().Of(change.Ref{Kind: change.KindProject, ID: p.ID})
	if err != nil {
		return err
	}
	if err := tx.WriteProjects().Append(p.ID, change.Undo(log, p.Deleted.Op)...); err != nil {
		return err
	}
	if err := reviveItemsOf(tx, p.ID, p.Deleted.Op); err != nil {
		return err
	}
	noteRevived(ctx, p.DisplayID(ws.Prefix))
	return nil
}

// reviveItemsOf は、プロジェクトのアイテムのうち、ログに操作 op の行があるものへ、その取り消しの行を書く。
func reviveItemsOf(tx Tx, p project.ID, op change.ID) error {
	its, err := tx.Items().List(p)
	if err != nil {
		return err
	}
	for _, it := range its {
		log, err := tx.Changes().Of(change.Ref{Kind: change.KindItem, ID: it.ID})
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(log, func(r change.Record) bool { return r.Op == op }) {
			continue
		}
		if err := tx.WriteItems().Append(it.ID, change.Undo(log, op)...); err != nil {
			return err
		}
	}
	return nil
}

type revivedKey struct{}

// revived は、取り消したものの表示 ID を、積んだ順に持つ。
type revived struct{ ids []string }

// WithRevived は、取り消したものの表示 ID を積む入れ物を置いた context を返す。
func WithRevived(ctx context.Context) context.Context {
	return context.WithValue(ctx, revivedKey{}, &revived{})
}

// Revived は、入れ物に積んだ表示 ID を、積んだ順に返す。入れ物がなければ nil。
func Revived(ctx context.Context) []string {
	v, _ := ctx.Value(revivedKey{}).(*revived)
	if v == nil {
		return nil
	}
	return slices.Clone(v.ids)
}

// noteRevived は、入れ物に表示 ID を積む(同じ ID は 1 つ)。入れ物がなければ捨てる。
func noteRevived(ctx context.Context, display string) {
	if v, _ := ctx.Value(revivedKey{}).(*revived); v != nil && !slices.Contains(v.ids, display) {
		v.ids = append(v.ids, display)
	}
}
