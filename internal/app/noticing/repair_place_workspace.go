package noticing

import (
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// workspaceText は、親と別のワークスペースの本文と鍵。{旧 ID} は移動元、{新 ID} は移動先の、いまの接頭辞。
// 空になった項目があれば末尾に足す。
func (m placeMove) workspaceText(r port.Reader, b batch, i int) (string, string, bool, error) {
	if m.to.Parent == nil {
		return "", "", false, nil
	}
	parent, ok, err := movedParentID(r, b, i, *m.to.Parent)
	if !ok {
		return "", "", false, err
	}
	src, ok, err := workspaceOf(r, b, m.from.Workspace)
	if !ok {
		return "", "", false, err
	}
	dst, ok, err := workspaceOf(r, b, m.to.Workspace)
	if !ok {
		return "", "", false, err
	}
	emptied, err := m.emptied(r, b, i)
	if err != nil {
		return "", "", false, err
	}
	text := withParticle(parent, "が") + " " + withParticle(oneLine(dst.Name), "へ") + "移動したため、" +
		withParticle(m.display(src.Prefix, m.from.Number), "を") + " " + withParticle(m.display(dst.Prefix, m.to.Number), "に") + "移動しました"
	if len(emptied) > 0 {
		text += "。合う値がなく空になった項目: " + strings.Join(emptied, "・")
	}
	return text, m.placeKey(), true, nil
}

// movedParentID は、親と別のワークスペースの本文の {親の ID}。親が移動先のワークスペースへ入る前の ID で、
// {旧 ID} と対になる。同じ Store の中の、親の転送記録のうち、i 番目の修復の保存の時刻までの最後のものの old を
// 使い(同じ判定のあとの保存の移動は見ない)、なければ親のいまの表示。
func movedParentID(r port.Reader, b batch, i int, parent item.ID) (string, bool, error) {
	recs, err := r.Transfers().List()
	if err = notFoundIsSkip(err); err != nil {
		return "", false, err
	}
	at := b.saves[i].Key.At
	for j := len(recs) - 1; j >= 0; j-- {
		c := recs[j]
		if c.Entity == transfer.EntityItem && c.Target == parent && c.ToStore == string(b.store) && c.Old != "" && !c.At.After(at) {
			return c.Old, true, nil
		}
	}
	e, ok, err := entityOf(r, b, change.Ref{Kind: change.KindItem, ID: parent})
	return e.text, ok, err
}

// emptied は、修復の直前のアイテムに、移動元・移動先の修復の直前の設定と、移動先のプロジェクトのルールで
// transfer.Remap をかけ直した、空になった項目(C3b-7)。設定を作れなければなし。
func (m placeMove) emptied(r port.Reader, b batch, i int) ([]string, error) {
	it, ok := item.FromLog(m.before)
	if !ok {
		return nil, nil
	}
	fromLog, err := beforeRead(r, b, change.Ref{Kind: change.KindWorkspace, ID: m.from.Workspace}, i)
	if err != nil {
		return nil, err
	}
	toLog, err := beforeRead(r, b, change.Ref{Kind: change.KindWorkspace, ID: m.to.Workspace}, i)
	if err != nil {
		return nil, err
	}
	from, ok := workspace.SettingsFromLog(fromLog)
	if !ok {
		return nil, nil
	}
	to, ok := workspace.SettingsFromLog(toLog)
	if !ok {
		return nil, nil
	}
	return transfer.Remap(it, from, to, projectrule.FromLog(toLog).Of(m.to.Project), nil, nil).Emptied, nil
}
