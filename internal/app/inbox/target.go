package inbox

import (
	"errors"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// TargetKind は、行の対象の種類(C4a-8)。
type TargetKind string

const (
	TargetItem             TargetKind = "item"              // 生きているアイテム。詳細パネルを開く
	TargetMoved            TargetKind = "moved"             // Store にない(別の Store へ移った)。旧 ID で転送記録を引く
	TargetDeletedItem      TargetKind = "deleted_item"      // 削除されたアイテム(リンクなし)
	TargetWorkspace        TargetKind = "workspace"         // アイテムのない通知。ワークスペースの設定の区画を開く
	TargetDeletedWorkspace TargetKind = "deleted_workspace" // 削除されたワークスペース(リンクなし)
)

// Target は、行の対象。いまの値(接頭辞・タイトル・ワークスペース名)で出す。
type Target struct {
	Kind          TargetKind
	Item          item.ID
	ItemKind      item.Kind
	Display       string // アイテムの ID(HH-12)。moved は通知を作ったときの ID
	Title         string
	Workspace     workspace.ID
	WorkspaceName string
	Section       string // workspace のとき、開くワークスペースの設定の区画
}

// Action は、止まった通知の 2 つのボタンの状態(C4a-10)。
type Action struct {
	Status       change.ID // 変える先のステータス
	StatusName   string
	Visible      bool // ボタンを出す
	ApplyEnabled bool // 「制約を無視して…」
	KeepEnabled  bool // 「変更しない」
	Note         string
	Result       port.NoticeResult // 空でなければ、ボタンの代わりに結果を出す
}

// resolve は、通知 n を行にする。r は n の Store の読み取り。
func resolve(r port.Reader, info port.StoreInfo, n port.NoticeRow) (Row, error) {
	row := Row{ID: n.ID, Kind: n.Kind, At: n.At, Read: n.Read, Text: n.Text, Reason: n.Reason, Sender: n.Sender, Store: info}
	var it *item.Item
	var err error
	if n.Item != "" {
		row.Target, it, err = itemTarget(r, n)
	} else {
		row.Target, err = workspaceTarget(r, n.Workspace)
	}
	if err != nil {
		return Row{}, err
	}
	if n.Kind == port.NoticeAutomationStopped {
		a, err := action(r, info, n, it)
		if err != nil {
			return Row{}, err
		}
		row.Action = &a
	}
	return row, nil
}

// itemTarget は、アイテムの対象。生きているときだけ、アイテムも返す。
func itemTarget(r port.Reader, n port.NoticeRow) (Target, *item.Item, error) {
	it, err := r.Items().Get(n.Item)
	switch {
	case errors.Is(err, port.ErrNotFound):
		return Target{Kind: TargetMoved, Item: n.Item, Display: n.Display}, nil, nil
	case err != nil:
		return Target{}, nil, err
	case it.Deleted != nil:
		return Target{Kind: TargetDeletedItem, Item: n.Item}, nil, nil
	}
	ws, err := r.Workspaces().Get(it.Place.Workspace)
	if err != nil {
		return Target{}, nil, err
	}
	return Target{
		Kind: TargetItem, Item: it.ID, ItemKind: it.Kind, Display: it.DisplayID(ws.Prefix), Title: it.Title,
		Workspace: ws.ID, WorkspaceName: ws.Name,
	}, &it, nil
}

// workspaceTarget は、アイテムのない通知の対象(ワークスペースの設定の「自動化」)。
func workspaceTarget(r port.Reader, id workspace.ID) (Target, error) {
	ws, err := port.LiveWorkspace(r, id)
	if errors.Is(err, port.ErrNotFound) {
		return Target{Kind: TargetDeletedWorkspace, Workspace: id}, nil
	}
	if err != nil {
		return Target{}, err
	}
	return Target{Kind: TargetWorkspace, Workspace: ws.ID, WorkspaceName: ws.Name, Section: "automation"}, nil
}

// action は、止まった通知のボタンの状態を上から順に決める(C4a-10)。it は生きているアイテム(なければ nil)。
func action(r port.Reader, info port.StoreInfo, n port.NoticeRow, it *item.Item) (Action, error) {
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: n.Workspace})
	if err != nil {
		return Action{}, err
	}
	a := Action{Status: n.Status, StatusName: statusName(log, n.Status), Result: n.Result}
	switch {
	case info.ReadOnly:
		a.Visible, a.Note = true, "閲覧のみの Store です"
	case it == nil:
	case it.Status != n.From:
		// いまのステータスは、アイテムのいまのワークスペースの値(別のワークスペースへ移ったことがある)
		now, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: it.Place.Workspace})
		if err != nil {
			return Action{}, err
		}
		a.Visible, a.KeepEnabled = true, true
		a.Note = "すでに変更されています(いまは " + statusName(now, it.Status) + ")"
	default:
		s, err := r.Workspaces().Settings(n.Workspace)
		if err != nil {
			return Action{}, err
		}
		a.Visible, a.KeepEnabled = true, true
		if slices.ContainsFunc(s.Statuses, func(v workspace.Value) bool { return v.ID == n.Status }) {
			a.ApplyEnabled = true
		} else {
			a.Note = "「" + a.StatusName + "」は削除されています"
		}
	}
	return a, nil
}

// statusName は、ステータスの名前。削除済みの値も引く。なければ空。
func statusName(log []change.Record, id change.ID) string {
	return workspace.NameOfDeleted(log, "statuses/"+string(id))
}
