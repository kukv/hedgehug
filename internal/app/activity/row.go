// Package activity は、アクティビティ(06「アクティビティ」)の行を作る操作を持つ。
package activity

import (
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ActorKind は、行の実行者の種類。
type ActorKind string

const (
	ActorUser       ActorKind = "user"
	ActorAutomation ActorKind = "automation"
	ActorGitHub     ActorKind = "github"
	ActorSync       ActorKind = "sync"
	ActorNone       ActorKind = "none"
)

// Target の Link の値。
const (
	LinkCurrent = "current" // いまの ID。開ける
	LinkMoved   = "moved"   // 別の Store へ移したものの旧 ID。転送記録から探して開く
)

// Target は、行の対象のアイテムかプロジェクト(C4d-8)。
type Target struct {
	Display string // いまの ID か旧 ID か完全に削除したときの ID
	Title   string // プロジェクトだけ
	Link    string // LinkCurrent・LinkMoved か、""(文字だけ)
}

// Row は、アクティビティの 1 行。
type Row struct {
	Key        string // "{SHA}:{連番(6 桁の 0 埋め)}" か "sync:{ID}"
	At         time.Time
	Store      port.StoreInfo
	Actor      ActorKind
	Who        string // 実行者の名前(C4d-12)
	Avatar     string // アバターに渡す名前。人でないか、不明なユーザーは ""
	Text       string
	Workspaces []workspace.ID
	Storewide  bool
	Workspace  string // ワークスペース名。Workspaces が 1 つのときだけ
	Item       *Target
	Project    *Target
	SHA        string // 7 桁。同期の行は ""
	dedup      string // 重なりの鍵(C4d-11)。人の行は ""
}
