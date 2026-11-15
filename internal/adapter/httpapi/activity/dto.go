package activity

import (
	"time"

	appactivity "github.com/kukv/hedgehug/internal/app/activity"
)

// ActivityStore は、行の Store。
type ActivityStore struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color" doc:"画面のトークン --t-<色> の名前"`
}

// ActivityTarget は、行の対象のアイテムかプロジェクト。
type ActivityTarget struct {
	Display string `json:"display" doc:"いまの ID か、旧 ID か、完全に削除したときの ID(HH-12・HH-P1)"`
	Title   string `json:"title" doc:"プロジェクトのときだけ"`
	Link    string `json:"link" enum:",current,moved" doc:"current は、いまの ID で開く。moved は、別の Store へ移した旧 ID で、転送記録から探して開く。空は文字だけ"`
}

// ActivityRow は、アクティビティの 1 行。
type ActivityRow struct {
	Key       string          `json:"key" doc:"行の鍵。コミットの行は {SHA}:{連番}、同期の行は sync:{ID}"`
	At        time.Time       `json:"at"`
	Cursor    string          `json:"cursor" doc:"続きを読むときの before に渡す値。{時刻(RFC 3339、ミリ秒まで)}|{鍵}"`
	Actor     string          `json:"actor" enum:"user,automation,github,sync,none"`
	Who       string          `json:"who" doc:"実行者の名前。none は空"`
	Avatar    string          `json:"avatar" doc:"アバターに渡す名前。人でないか、不明なユーザーは空"`
	Text      string          `json:"text"`
	Store     ActivityStore   `json:"store"`
	Workspace string          `json:"workspace" doc:"ワークスペース名。行のワークスペースが 1 つのときだけ。ほかは空"`
	Item      *ActivityTarget `json:"item,omitempty"`
	Project   *ActivityTarget `json:"project,omitempty"`
	SHA       string          `json:"sha" doc:"7 桁。同期の行は空"`
}

// ActivityPage は、一覧の 1 回分。
type ActivityPage struct {
	Rows []ActivityRow `json:"rows"`
	More bool          `json:"more" doc:"続きがある。最後の行の cursor を before に渡して読む"`
}

func toPage(p appactivity.Page) ActivityPage {
	rows := make([]ActivityRow, len(p.Rows))
	for i, r := range p.Rows {
		rows[i] = ActivityRow{
			Key: r.Key, At: r.At, Cursor: appactivity.Cursor(r), Actor: string(r.Actor), Who: r.Who, Avatar: r.Avatar, Text: r.Text,
			Store:     ActivityStore{ID: string(r.Store.ID), Name: r.Store.Name, Color: r.Store.Color},
			Workspace: r.Workspace, Item: toTarget(r.Item), Project: toTarget(r.Project), SHA: r.SHA,
		}
	}
	return ActivityPage{Rows: rows, More: p.More}
}

func toTarget(t *appactivity.Target) *ActivityTarget {
	if t == nil {
		return nil
	}
	return &ActivityTarget{Display: t.Display, Title: t.Title, Link: t.Link}
}
