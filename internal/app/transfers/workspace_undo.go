package transfers

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// rollbackWorkspace は、ワークスペースの移動先の保存を戻す: 写したフォルダを消し、足したメンバーを外す。戻せたら cause を
// 返す。戻しが送れなかった・失敗したら *UndoError(Old はワークスペースの名前、NewDisplay は空)。
func (s Service) rollbackWorkspace(ctx context.Context, m *crossWorkspaceMove, cause error) error {
	summary := fmt.Sprintf("ワークスペース「%s」の移動を取り消し", m.name())
	return s.deliverUndo(ctx, m.to, m.meTo, summary, m.undo, cause, UndoError{Old: m.name(), Workspace: true})
}

// undo は、戻しの保存の fn。移動の操作(移動先のワークスペースの最後の moved_in の行の操作)を引いて、足した人を外す行を
// 決めてから、移動先のワークスペースの中のプロジェクトとアイテムのフォルダ(tx から読む。移動のあとに移動先で作られたものも)
// と、ワークスペースのフォルダを消す(消すと moved_in の行も読めなくなる)。移動先に移動が保存されていなければ(moved_in が
// ない)、メンバーの行は書かない。保存されなかったワークスペースのフォルダは飛ばす。
func (m *crossWorkspaceMove) undo(tx port.Tx) error {
	id := m.src.ws.ID
	op, err := lastMovedIn(tx, change.Ref{Kind: change.KindWorkspace, ID: id})
	if err != nil {
		return err
	}
	var members []change.Draft
	if op != "" {
		if members, err = unjoinDrafts(tx, op, m.added); err != nil {
			return err
		}
	}
	projects, err := tx.Projects().List(id)
	if err != nil {
		return err
	}
	for _, p := range projects {
		if err := removeProjectFolders(tx, p.ID); err != nil {
			return err
		}
	}
	if err := tx.WriteWorkspaces().Remove(id); err != nil && !errors.Is(err, port.ErrNotFound) {
		return err
	}
	return tx.WriteMembers().Append(members...)
}

// removeProjectFolders は、プロジェクト p の下のアイテム(削除済みを含む。tx から読む)と p のフォルダを消す。p のフォルダが
// なければ(保存されなかった)飛ばす。
func removeProjectFolders(tx port.Tx, p project.ID) error {
	items, err := tx.Items().List(p)
	if err != nil {
		return err
	}
	for _, it := range items {
		if err := tx.WriteItems().Remove(it.ID); err != nil {
			return err
		}
	}
	if err := tx.WriteProjects().Remove(p); err != nil && !errors.Is(err, port.ErrNotFound) {
		return err
	}
	return nil
}

// logMark は、1 つのログの行数と最後の行の id。
type logMark struct {
	rows int
	last change.ID
}

func markOf(log []change.Record) logMark {
	if len(log) == 0 {
		return logMark{}
	}
	return logMark{rows: len(log), last: log[len(log)-1].ID}
}

// sourceStamp は、移動元で移すものの印(B5c-9): ワークスペース・プロジェクト・アイテムの参照ごとの、ログの行数と最後の行の id。
// 参照の集まりが違うか、どれかのログに行が足されていれば、印は違う。
type sourceStamp map[change.Ref]logMark

// setRefs は、ワークスペース set のワークスペース・プロジェクト・アイテムの参照。
func setRefs(set transfer.WorkspaceSet) []change.Ref {
	refs := []change.Ref{{Kind: change.KindWorkspace, ID: set.Workspace}}
	for _, p := range set.Projects {
		refs = append(refs, change.Ref{Kind: change.KindProject, ID: p.ID})
	}
	for _, it := range set.Items {
		refs = append(refs, change.Ref{Kind: change.KindItem, ID: it.ID})
	}
	return refs
}

// stamp は、prepare で読んだものの印。
func (g workspaceGathered) stamp() sourceStamp {
	s := sourceStamp{}
	for _, ref := range setRefs(g.set) {
		if ref.Kind == change.KindWorkspace {
			s[ref] = markOf(g.wsLog)
		} else {
			s[ref] = markOf(g.logs[ref.ID])
		}
	}
	return s
}

// checkSource は、移動元のワークスペースを tx から読み直し、印が prepare で読んだものと同じかを確かめて、集め直したもの
// (外との関連は読み直した値)を返す。説明・設定・前の移動は読まない(印で変わっていないと分かるため)。ワークスペースがもう
// なければ port.ErrNotFound、印が違えば errSourceChanged。
func (m *crossWorkspaceMove) checkSource(tx port.Tx) (transfer.WorkspaceSet, error) {
	id := m.src.ws.ID
	if _, err := port.EditableWorkspace(tx, id); err != nil {
		return transfer.WorkspaceSet{}, err
	}
	projects, err := tx.Projects().List(id)
	if err != nil {
		return transfer.WorkspaceSet{}, err
	}
	all, err := storeItems(tx)
	if err != nil {
		return transfer.WorkspaceSet{}, err
	}
	set := transfer.CollectWorkspace(id, projects, all)
	now := sourceStamp{}
	for _, ref := range setRefs(set) {
		log, err := tx.Changes().Of(ref)
		if err != nil {
			return transfer.WorkspaceSet{}, err
		}
		now[ref] = markOf(log)
	}
	if !maps.Equal(now, m.src.stamp()) {
		return transfer.WorkspaceSet{}, errSourceChanged
	}
	return set, nil
}
