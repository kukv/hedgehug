package transfers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
)

// rollback は、移動先の保存を戻す(A5a-6): 写したフォルダを消し、足したメンバーを外す。number の行は戻さない。
// cause を返す。戻しも失敗したら errors.Join(cause, 戻しのエラー)。
func (s Service) rollback(ctx context.Context, m *crossMove, cause error) error {
	op := port.Operation{Actor: change.User(m.meTo), Summary: m.moved.Display + " の移動を取り消し"}
	reason, err := s.Delivery.Deliver(ctx, m.in.ToStore, op, m.undo)
	if err == nil && reason != "" {
		err = &SendError{Store: m.to.Name, Reason: reason}
	}
	if err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

// undo は、戻しの保存の fn。足した人を外す行を決めてから、写したフォルダを消す(消すと、移動の操作を引く moved_in の行も
// 読めなくなる)。保存されなかった(port.ErrSaveNotDiscarded で、捨てられていた)フォルダは飛ばす。
func (m *crossMove) undo(tx port.Tx) error {
	ds, err := m.unjoin(tx)
	if err != nil {
		return err
	}
	for _, it := range m.src.items() {
		if err := tx.WriteItems().Remove(it.ID); err != nil && !errors.Is(err, port.ErrNotFound) {
			return err
		}
	}
	return tx.WriteMembers().Append(ds...)
}

// unjoin は、この移動で足した人を外す行と、書いた表示名をログイン名に戻す行。移動の操作は、移動先の移したアイテムの最後の
// moved_in の行の操作。移動先に移動が保存されていなければ何もしない。移動のあとに別の操作が同じ人を足していれば外さない。
func (m *crossMove) unjoin(tx port.Tx) ([]change.Draft, error) {
	log, err := tx.Changes().Of(change.Ref{Kind: change.KindItem, ID: m.src.set.Root.ID})
	if errors.Is(err, port.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var op change.ID
	for _, r := range log {
		if r.T == change.MovedIn {
			op = r.Op
		}
	}
	if op == "" {
		return nil, nil
	}
	members, err := tx.Changes().Of(change.Ref{Kind: change.KindMembers})
	if err != nil {
		return nil, err
	}
	var ds []change.Draft
	for _, p := range m.added {
		if joinedLater(members, op, p.Login) {
			continue
		}
		ds = append(ds, change.NewRemove("members", p.Login))
		if p.Name != p.Login {
			ds = append(ds, member.Rename(p.Login, p.Name, p.Login))
		}
	}
	return ds, nil
}

// joinedLater は、メンバーのログで、操作 op が login を足した行より後に、別の操作が同じ人(大文字小文字を区別しない)を足したか。
func joinedLater(log []change.Record, op change.ID, login string) bool {
	ours := false
	for _, r := range log {
		var l string
		if r.T != change.Add || r.F != "members" || json.Unmarshal(r.To, &l) != nil || !strings.EqualFold(l, login) {
			continue
		}
		if r.Op == op {
			ours = true
		} else if ours {
			return true
		}
	}
	return false
}
