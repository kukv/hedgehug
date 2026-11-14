package transfers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
)

// UndoError は、別の Store への移動で、移動先の保存のあとに失敗し、移動先の保存の取り消しもできなかったこと(移したアイテムが
// 移動先と移動元の両方にある。02「保存」)。Old は旧 ID、Store は移動先の Store 名、NewDisplay は移動先の新 ID。
// ワークスペースの移動では、Old はワークスペースの名前で NewDisplay は空(B5c-14 の文になる)。
// Cause は取り消しの前の失敗(errors.Is で辿れる)、Undo は取り消しの失敗(送れなかったときは *SendError)。
type UndoError struct {
	Old, Store, NewDisplay string
	Cause, Undo            error
}

func (e *UndoError) Error() string {
	if e.NewDisplay == "" {
		return fmt.Sprintf("ワークスペース「%s」を %s へ移しましたが、移動元から取り除けず、取り消しもできませんでした。どちらかを削除してください",
			e.Old, e.Store)
	}
	return fmt.Sprintf("%s を %s の %s に移しましたが、移動元から取り除けず、取り消しもできませんでした。どちらかを削除してください",
		e.Old, e.Store, e.NewDisplay)
}

// Unwrap は、取り消しの前の失敗だけを返す(取り消しの *SendError を、移動の送信の失敗として読ませない)。
func (e *UndoError) Unwrap() error { return e.Cause }

// rollback は、移動先の保存を戻す(A5a-6): 写したフォルダを消し、足したメンバーを外す。number の行は戻さない。
// 戻せたら cause を返す。戻しが送れなかった・失敗したら *UndoError。
func (s Service) rollback(ctx context.Context, m *crossMove, cause error) error {
	op := port.Operation{Actor: change.User(m.meTo), Summary: m.moved.Display + " の移動を取り消し"}
	reason, err := s.Delivery.Deliver(ctx, m.in.ToStore, op, m.undo)
	if err == nil && reason != "" {
		err = &SendError{Store: m.to.Name, Reason: reason}
	}
	if err != nil {
		return &UndoError{Old: m.src.set.Root.DisplayID(m.src.ws.Prefix), Store: m.to.Name, NewDisplay: m.moved.Display, Cause: cause, Undo: err}
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
// moved_in の行の操作。移動先に移動が保存されていなければ何もしない。
func (m *crossMove) unjoin(tx port.Tx) ([]change.Draft, error) {
	op, err := lastMovedIn(tx, change.Ref{Kind: change.KindItem, ID: m.src.set.Root.ID})
	if err != nil || op == "" {
		return nil, err
	}
	return unjoinDrafts(tx, op, m.added)
}

// lastMovedIn は、移動先の ref のログの最後の moved_in の行の操作。ログがないか moved_in がなければ空。
func lastMovedIn(tx port.Tx, ref change.Ref) (change.ID, error) {
	log, err := tx.Changes().Of(ref)
	if errors.Is(err, port.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var op change.ID
	for _, r := range log {
		if r.T == change.MovedIn {
			op = r.Op
		}
	}
	return op, nil
}

// unjoinDrafts は、移動の操作 op で足した人 added を外す行と、書いた表示名をログイン名に戻す行。移動のあとに別の操作が
// 同じ人を足していれば外さない。
func unjoinDrafts(tx port.Tx, op change.ID, added []member.Member) ([]change.Draft, error) {
	members, err := tx.Changes().Of(change.Ref{Kind: change.KindMembers})
	if err != nil {
		return nil, err
	}
	var ds []change.Draft
	for _, p := range added {
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
