package port

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
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

// aliveAbove は、it の祖先のアイテム・プロジェクト・ワークスペースがどれも削除済みでないことを確かめる。
func aliveAbove(r Reader, it item.Item) error {
	if _, err := LiveWorkspace(r, it.Place.Workspace); err != nil {
		return err
	}
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
	if p.Deleted != nil {
		// ワークスペースが別の操作で削除済みなら取り消さない(08「親が削除済みのときの復元」。B5b)。
		if _, err := LiveWorkspace(r, p.Place.Workspace); err != nil {
			return project.Project{}, err
		}
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
	NoteRevived(ctx, Revival{Kind: RevivedID, Text: it.DisplayID(ws.Prefix)})
	return nil
}

// ReviveProject は、tx の中で読み直し、削除済み(root: true)のプロジェクトなら、その削除の操作の行を、
// プロジェクト自身とそのアイテムのログから取り消す。同じ名前の削除されていないプロジェクトがあれば断る(決定 S11)。
func ReviveProject(ctx context.Context, tx Tx, id project.ID) error {
	return ReviveProjectAs(ctx, tx, id, "")
}

// ReviveProjectAs は、ReviveProject の同じ名前の判定を、title(空なら、いまのタイトル)で行う版。
// タイトルを変える保存が、変更後のタイトルで判定するのに使う(決定 S11)。
func ReviveProjectAs(ctx context.Context, tx Tx, id project.ID, title string) error {
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
	if title == "" {
		title = p.Title
	}
	if err := project.CheckRevive(title, live); err != nil {
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
	NoteRevived(ctx, Revival{Kind: RevivedID, Text: p.DisplayID(ws.Prefix)})
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

// ReviveWorkspace は、tx の中で読み直し、削除済みのワークスペースなら、その削除の操作の行を、その行がある
// ログ(ワークスペース、そのプロジェクトとアイテム)すべてで取り消し、削除の前の名前を WithRevived の入れ物に積んで
// true を返す。名前 name・接頭辞 prefix(空なら、いまの値)が削除されていないワークスペースと重なれば断る(決定 R11)。
// 削除されていなければ何もせず false。
func ReviveWorkspace(ctx context.Context, tx Tx, id workspace.ID, name, prefix string) (bool, error) {
	w, err := tx.Workspaces().Get(id)
	if err != nil || w.Deleted == nil {
		return false, err
	}
	next := workspace.Workspace{ID: id, Name: cmp.Or(name, w.Name), Prefix: cmp.Or(prefix, w.Prefix)}
	live, err := LiveWorkspaces(tx)
	if err != nil {
		return false, err
	}
	if err := workspace.CheckRevive(next, live); err != nil {
		return false, err
	}
	refs, err := workspaceRefs(tx, id)
	if err != nil {
		return false, err
	}
	refs = append([]change.Ref{{Kind: change.KindWorkspace, ID: id}}, refs...)
	if err := undoAcross(tx, refs, w.Deleted.Op); err != nil {
		return false, err
	}
	NoteRevived(ctx, Revival{Kind: RevivedName, Text: strings.Join(strings.Fields(w.Name), " ")})
	return true, nil
}

// ReviveSetting は、tx の中でワークスペース ws のログを読み直し、項目名 f を覆う削除の行(workspace.Covering)の
// op ごとに、その op の行があるログ(ワークスペース、そのプロジェクトとアイテム)へ取り消しの行を書く
// (値の削除で置き換えたアイテムの値も、先端なら戻る)。名前は WithRevived の入れ物に積む。
// 取り消したものがあれば true。
func ReviveSetting(ctx context.Context, tx Tx, ws workspace.ID, f string) (bool, error) {
	wref := change.Ref{Kind: change.KindWorkspace, ID: ws}
	log, err := tx.Changes().Of(wref)
	if err != nil {
		return false, err
	}
	covering := workspace.Covering(log, f)
	if len(covering) == 0 {
		return false, nil
	}
	refs, err := workspaceRefs(tx, ws)
	if err != nil {
		return false, err
	}
	refs = append([]change.Ref{wref}, refs...)
	done := map[change.ID]bool{}
	for _, d := range covering {
		if done[d.Op] {
			continue
		}
		done[d.Op] = true
		if err := undoAcross(tx, refs, d.Op); err != nil {
			return false, err
		}
		NoteRevived(ctx, Revival{Kind: RevivedName, Text: strings.Join(strings.Fields(workspace.NameOfDeleted(log, d.F)), " ")})
	}
	return true, nil
}

// workspaceRefs は、ワークスペースのプロジェクトとアイテム(削除済みも含む)のログを指す Ref。
func workspaceRefs(tx Tx, ws workspace.ID) ([]change.Ref, error) {
	ps, err := tx.Projects().List(ws)
	if err != nil {
		return nil, err
	}
	var refs []change.Ref
	for _, p := range ps {
		refs = append(refs, change.Ref{Kind: change.KindProject, ID: p.ID})
		its, err := tx.Items().List(p.ID)
		if err != nil {
			return nil, err
		}
		for _, it := range its {
			refs = append(refs, change.Ref{Kind: change.KindItem, ID: it.ID})
		}
	}
	return refs, nil
}

// undoAcross は、refs のログのうち操作 op の行があるものへ、読み直したログで、その取り消しの行を書く。
func undoAcross(tx Tx, refs []change.Ref, op change.ID) error {
	for _, ref := range refs {
		log, err := tx.Changes().Of(ref)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(log, func(r change.Record) bool { return r.Op == op }) {
			continue
		}
		drafts := change.Undo(log, op)
		switch ref.Kind {
		case change.KindWorkspace:
			err = tx.WriteWorkspaces().Append(ref.ID, drafts...)
		case change.KindProject:
			err = tx.WriteProjects().Append(ref.ID, drafts...)
		default:
			err = tx.WriteItems().Append(ref.ID, drafts...)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
