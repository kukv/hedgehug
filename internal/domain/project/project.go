// Package project は、プロジェクト(Epic をまとめる単位)を持つ。
package project

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ID はプロジェクトの内部 ID。
type ID = change.ID

// Status は、プロジェクトのステータス。5 つに固定で、利用者は変えられない(01・02)。
type Status string

const (
	Todo       Status = "Todo"
	InProgress Status = "InProgress"
	Done       Status = "Done"
	Pending    Status = "Pending"
	Cancel     Status = "Cancel"
)

// ValidStatus は、5 つのどれかか。
func ValidStatus(s Status) bool {
	switch s {
	case Todo, InProgress, Done, Pending, Cancel:
		return true
	}
	return false
}

// Place は、プロジェクトの場所(ワークスペースと番号)。1 つの項目として変える。
type Place struct {
	Workspace workspace.ID `json:"workspace"`
	Number    int          `json:"number"`
}

// Project は、プロジェクトの固定の項目(02「プロジェクト」)。説明はトラック A の A4 で足す。
type Project struct {
	ID     ID
	Place  Place
	Rank   string
	Title  string
	Owner  string // ログイン名。空は「なし」
	Status Status
	Start  string // YYYY-MM-DD。空は「なし」
	End    string
	Repos  []string // owner/name。大文字小文字を区別しない辞書順(A4a-19)
	// Deleted は、削除済みの印(A4b-2)。保存の口は削除済みも返し、除くのは app の側で Live を通す(A4b-3)。
	Deleted *change.Deletion
}

// DisplayID は、画面の ID(HH-P1)。
func (p Project) DisplayID(prefix string) string {
	return prefix + "-P" + strconv.Itoa(p.Place.Number)
}

// Create は、プロジェクトの作成の変更。
func Create(p Project) change.Draft {
	return change.NewCreate(map[string]any{
		"id": p.ID, "place": p.Place, "rank": p.Rank, "title": p.Title,
		"owner": p.Owner, "status": p.Status, "start": p.Start, "end": p.End,
	})
}

// FromLog は、プロジェクトのログからいまの値を作る。作成の行がなければ false。
func FromLog(log []change.Record) (Project, bool) {
	s := change.Resolve(log, change.Options{Multi: func(f string) bool { return f == "repos" }})
	if s.Created == nil {
		return Project{}, false
	}
	var p Project
	for f, dst := range map[string]any{
		"id": &p.ID, "place": &p.Place, "rank": &p.Rank, "title": &p.Title,
		"owner": &p.Owner, "status": &p.Status, "start": &p.Start, "end": &p.End,
	} {
		if v, ok := s.Values[f]; ok {
			_ = json.Unmarshal(v, dst)
		}
	}
	for _, raw := range s.Lists["repos"] {
		var repo string
		if json.Unmarshal(raw, &repo) == nil {
			p.Repos = append(p.Repos, repo)
		}
	}
	slices.SortFunc(p.Repos, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	p.Deleted = change.DeletionOf(log)
	return p, true
}

// Live は、削除済みを除いたプロジェクトを、順を保って返す。
func Live(ps []Project) []Project {
	out := make([]Project, 0, len(ps))
	for _, p := range ps {
		if p.Deleted == nil {
			out = append(out, p)
		}
	}
	return out
}
