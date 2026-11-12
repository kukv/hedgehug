// Package project は、プロジェクト(Epic をまとめる単位)を持つ。
package project

import (
	"encoding/json"
	"strconv"

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
	s := change.Resolve(log, change.Options{})
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
	return p, true
}
