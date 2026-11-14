// Package palette は、コマンドパレット(02「コマンドパレット」)の候補のうち、サーバーが返すもの(アイテム・プロジェクトの
// 索引と、最近開いたもの)を持つ。
package palette

import (
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Service は、パレットが使う口(A5c-18)。SyncTargets は、非表示を含むこのマシンの Store(最近開いたものの片付け)。
type Service struct {
	Stores      port.Stores
	StoreList   port.StoreList
	SyncTargets port.SyncTargets
	Recent      port.RecentItems
	Clock       port.Clock
}

// Index は、パレットのアイテム・プロジェクトの候補(A5c-3)。並びは問わない(画面がサイドバーの並びで並べる)。
type Index struct {
	Items    []IndexItem
	Projects []IndexProject
}

// IndexItem は、表示中の Store の、削除されていないアイテム 1 件。Display はいまの ID(HH-12)、Aliases は旧 ID
// (いまの ID と同じものを除く)。
type IndexItem struct {
	Store        port.StoreID
	Workspace    workspace.ID
	Project      project.ID
	Item         item.ID
	Number       int
	Display      string
	Title        string
	ProjectTitle string
	Aliases      []transfer.Alias
}

// IndexProject は、表示中の Store の、削除されていないプロジェクト 1 件。Display はいまの ID(HH-P1)。
type IndexProject struct {
	Store         port.StoreID
	Workspace     workspace.ID
	Project       project.ID
	Display       string
	Title         string
	WorkspaceName string
	Aliases       []transfer.Alias
}
