package inbox

import (
	"time"

	appinbox "github.com/kukv/hedgehug/internal/app/inbox"
)

// InboxCounts は、タブごとの未読数。
type InboxCounts struct {
	All        int `json:"all"`
	Mention    int `json:"mention"`
	Assign     int `json:"assign"`
	Watch      int `json:"watch"`
	Overwrite  int `json:"overwrite"`
	Automation int `json:"automation"`
	Repair     int `json:"repair"`
}

// InboxStore は、通知の Store。
type InboxStore struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color" doc:"画面のトークン --t-<色> の名前"`
	ReadOnly bool   `json:"readOnly"`
}

// InboxTarget は、行の対象。いまの値で出す。
type InboxTarget struct {
	Kind          string `json:"kind" enum:"item,moved,deleted_item,workspace,deleted_workspace"`
	Item          string `json:"item" doc:"アイテムの内部 ID。workspace・deleted_workspace では空"`
	ItemKind      string `json:"itemKind" enum:",Epic,Story,Task,Bug,Step" doc:"item のときのアイテムの種類。ほかは空"`
	Display       string `json:"display" doc:"アイテムの ID(HH-12)。moved は通知を作ったときの ID"`
	Title         string `json:"title"`
	Workspace     string `json:"workspace"`
	WorkspaceName string `json:"workspaceName"`
	Section       string `json:"section" doc:"workspace のとき、開くワークスペースの設定の区画"`
}

// InboxAction は、止まった通知の 2 つのボタンの状態。
type InboxAction struct {
	Status       string `json:"status" doc:"変える先のステータスの ID"`
	StatusName   string `json:"statusName"`
	Visible      bool   `json:"visible" doc:"ボタンを出す"`
	ApplyEnabled bool   `json:"applyEnabled" doc:"「制約を無視して…」を押せる"`
	KeepEnabled  bool   `json:"keepEnabled" doc:"「変更しない」を押せる"`
	Note         string `json:"note"`
	Result       string `json:"result" enum:",applied,kept" doc:"空でなければ、ボタンの代わりに結果を出す"`
}

// InboxRow は、受信箱の 1 行。
type InboxRow struct {
	ID     int64        `json:"id"`
	Kind   string       `json:"kind" enum:"automation,automation_stopped,mention,assign,watch,overwrite,repair"`
	At     time.Time    `json:"at"`
	Read   bool         `json:"read"`
	Text   string       `json:"text"`
	Reason string       `json:"reason"`
	Sender string       `json:"sender" doc:"差出人のログイン名。空は「hedgehug」"`
	Store  InboxStore   `json:"store"`
	Target InboxTarget  `json:"target"`
	Action *InboxAction `json:"action,omitempty" doc:"止まった通知だけ"`
}

// InboxPage は、受信箱の一覧の 1 回分。
type InboxPage struct {
	Rows   []InboxRow  `json:"rows"`
	More   bool        `json:"more" doc:"続きがある"`
	Unread InboxCounts `json:"unread"`
}

// InboxUnread は、タブごとの未読数の本文。
type InboxUnread struct {
	Counts InboxCounts `json:"counts"`
}

// SetInboxRead は、既読・未読の本文。
type SetInboxRead struct {
	Read bool `json:"read"`
}

// ReadAllInbox は、すべて既読の本文。
type ReadAllInbox struct {
	Tab string `json:"tab" doc:"all・mention・assign・watch・overwrite・automation・repair のどれか。ほかは 400"`
}

func toCounts(m map[appinbox.Tab]int) InboxCounts {
	return InboxCounts{
		All: m[appinbox.TabAll], Mention: m[appinbox.TabMention], Assign: m[appinbox.TabAssign], Watch: m[appinbox.TabWatch],
		Overwrite: m[appinbox.TabOverwrite], Automation: m[appinbox.TabAutomation], Repair: m[appinbox.TabRepair],
	}
}

func toPage(p appinbox.Page) InboxPage {
	out := InboxPage{Rows: []InboxRow{}, More: p.More, Unread: toCounts(p.Unread)}
	for _, r := range p.Rows {
		out.Rows = append(out.Rows, toRow(r))
	}
	return out
}

func toRow(r appinbox.Row) InboxRow {
	t := r.Target
	row := InboxRow{
		ID: int64(r.ID), Kind: string(r.Kind), At: r.At, Read: r.Read, Text: r.Text, Reason: r.Reason, Sender: r.Sender,
		Store: InboxStore{ID: string(r.Store.ID), Name: r.Store.Name, Color: r.Store.Color, ReadOnly: r.Store.ReadOnly},
		Target: InboxTarget{
			Kind: string(t.Kind), Item: string(t.Item), ItemKind: string(t.ItemKind), Display: t.Display, Title: t.Title,
			Workspace: string(t.Workspace), WorkspaceName: t.WorkspaceName, Section: t.Section,
		},
	}
	if a := r.Action; a != nil {
		row.Action = &InboxAction{
			Status: string(a.Status), StatusName: a.StatusName, Visible: a.Visible, ApplyEnabled: a.ApplyEnabled,
			KeepEnabled: a.KeepEnabled, Note: a.Note, Result: string(a.Result),
		}
	}
	return row
}
