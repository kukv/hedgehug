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

// Place は、プロジェクトの場所(ワークスペースと番号)。1 つの項目として変える。
type Place struct {
	Workspace workspace.ID `json:"workspace"`
	Number    int          `json:"number"`
}

// Project は、土台が扱う項目だけを持つ。オーナー・日付・ステータス・説明はトラック A が足す。
type Project struct {
	ID    ID
	Place Place
	Title string
}

// DisplayID は、画面の ID(HH-P1)。
func (p Project) DisplayID(prefix string) string {
	return prefix + "-P" + strconv.Itoa(p.Place.Number)
}

// Create は、プロジェクトの作成の変更。
func Create(p Project) change.Draft {
	return change.NewCreate(map[string]any{"id": p.ID, "place": p.Place, "title": p.Title})
}

// FromLog は、プロジェクトのログからいまの値を作る。作成の行がなければ false。
func FromLog(log []change.Record) (Project, bool) {
	s := change.Resolve(log, change.Options{})
	if s.Created == nil {
		return Project{}, false
	}
	var p Project
	_ = json.Unmarshal(s.Values["id"], &p.ID)
	_ = json.Unmarshal(s.Values["place"], &p.Place)
	_ = json.Unmarshal(s.Values["title"], &p.Title)
	return p, true
}
