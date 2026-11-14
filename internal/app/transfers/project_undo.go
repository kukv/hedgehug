package transfers

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// rollbackProject は、プロジェクトの移動先の保存を戻す(A5b-14): 写したフォルダを消し、足したメンバーを外し、移動先の
// ワークスペースに書いたルールの行を戻す。number の行は戻さない。戻せたら cause を返す。戻しが送れなかった・失敗したら
// *UndoError(Old はプロジェクトの旧 ID、NewDisplay はプロジェクトの新 ID)。
func (s Service) rollbackProject(ctx context.Context, m *crossProjectMove, cause error) error {
	display := m.plan.display()
	op := port.Operation{Actor: change.User(m.meTo), Summary: display + " の移動を取り消し"}
	reason, err := s.Delivery.Deliver(ctx, m.in.ToStore, op, m.undo)
	if err == nil && reason != "" {
		err = &SendError{Store: m.to.Name, Reason: reason}
	}
	if err != nil {
		return &UndoError{Old: m.old(), Store: m.to.Name, NewDisplay: display, Cause: cause, Undo: err}
	}
	return cause
}

// undo は、戻しの保存の fn。移動の操作(移動先のプロジェクトの最後の moved_in の行の操作)を引いて、足した人を外す行と
// ルールを戻す行を決めてから、写したフォルダを消す(消すと moved_in の行も読めなくなる)。移動先に移動が保存されていなければ
// (moved_in がない)、行は書かない。保存されなかったフォルダは飛ばす。
func (m *crossProjectMove) undo(tx port.Tx) error {
	op, err := lastMovedIn(tx, change.Ref{Kind: change.KindProject, ID: m.src.project.ID})
	if err != nil {
		return err
	}
	var members, rules []change.Draft
	if op != "" {
		if members, err = unjoinDrafts(tx, op, m.added); err != nil {
			return err
		}
		rules = inverseRules(m.plan.carried, m.src.project.ID)
	}
	for _, it := range m.src.set.Items() {
		if err := tx.WriteItems().Remove(it.ID); err != nil && !errors.Is(err, port.ErrNotFound) {
			return err
		}
	}
	if err := tx.WriteProjects().Remove(m.src.project.ID); err != nil && !errors.Is(err, port.ErrNotFound) {
		return err
	}
	if len(rules) > 0 {
		if err := tx.WriteWorkspaces().Append(m.plan.dst.ws.ID, rules...); err != nil {
			return err
		}
	}
	return tx.WriteMembers().Append(members...)
}

// inverseRules は、移動先のワークスペースに書いたルールの行 c.Destination を戻す行: add は remove、種類ごとの既定の set は
// set(なし)、作った専用ルールは削除(A5b-14)。専用ルールの作成の set は、ルールの削除で戻るので書かない。
func inverseRules(c transfer.Carried, p project.ID) []change.Draft {
	defaults := projectrule.TemplateDefaultField(p, "")
	var out []change.Draft
	for _, d := range c.Destination {
		switch {
		case d.T == change.Add:
			out = append(out, change.Draft{T: change.Remove, F: d.F, From: d.To})
		case d.T == change.Set && strings.HasPrefix(d.F, defaults):
			out = append(out, change.Draft{T: change.Set, F: d.F, From: d.To, To: change.Value(nil)})
		}
	}
	for _, r := range c.Created {
		out = append(out, automation.DeleteDraft(r.ID))
	}
	return out
}

// checkSource は、移動元のプロジェクトを tx から読み直し、ワークスペース・移すアイテム(削除済みを含む)・プロジェクト別のルール・
// 専用ルールが prepare で読んだものと同じかを確かめて、集め直したものを返す。プロジェクトがもうなければ port.ErrNotFound。
func (m *crossProjectMove) checkSource(tx port.Tx) (transfer.ProjectSet, error) {
	now, err := readProjectSource(tx, m.src.project.ID, port.EditableProject)
	if err != nil {
		return transfer.ProjectSet{}, err
	}
	if now.ws.ID != m.src.ws.ID || !slices.Equal(projectItemIDs(now.set), projectItemIDs(m.src.set)) ||
		!reflect.DeepEqual(now.rules, m.src.rules) || !slices.Equal(now.own, m.src.own) {
		return transfer.ProjectSet{}, errSourceChanged
	}
	return now.set, nil
}

// projectItemIDs は、移すアイテムの内部 ID(辞書順)。
func projectItemIDs(s transfer.ProjectSet) []item.ID {
	var ids []item.ID
	for _, it := range s.Items() {
		ids = append(ids, it.ID)
	}
	slices.Sort(ids)
	return ids
}
