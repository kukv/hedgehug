// Package port は、usecase が外に触るための口を決める。口は「何をしたいか」の言葉で決め、
// git の言葉(コミット、ブランチ、clone)を使わない(設計書 3 節)。
// 口は 1 つ 1 ファイルに置く。新しい口は、使うトラックが足す。
package port

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// StoreID は Store ID(.hedgehug/store.yaml の store)。
type StoreID string

var (
	ErrNotFound      = errors.New("not found")
	ErrStoreNotFound = errors.New("store not found")
)

// Stores は Store のデータの口。1 回の操作 = 1 回の Write。
type Stores interface {
	Read(ctx context.Context, storeID StoreID, fn func(r Reader) error) error
	// Write は fn がエラーなら何も残さず、errors.Is で同じエラーと分かるエラーを返す(包んでよい)。
	// 同じ Store への Write は 1 つずつ順に行う。
	Write(ctx context.Context, storeID StoreID, op Operation, fn func(tx Tx) error) error
}

// Operation は 1 回の操作。
type Operation struct {
	Actor   change.Actor // あなた・自動化・同期・GitHub
	Summary string       // アクティビティの 1 行
}

// Reader は Store の読み取り。種類ごとの関数は、各トラックが使うものだけを足す。
type Reader interface {
	Workspaces() WorkspaceReader
	Projects() ProjectReader
	Items() ItemReader
	Members() MemberReader
	Changes() ChangeReader // 履歴とアクティビティ
}

// Tx は 1 回の Write の中の読み書き。読み取りは、同じ Tx で書いたものを含む。
type Tx interface {
	Reader
	WriteWorkspaces() WorkspaceWriter
	WriteProjects() ProjectWriter
	WriteItems() ItemWriter
	WriteMembers() MemberWriter
}

type WorkspaceReader interface {
	Get(id workspace.ID) (workspace.Workspace, error)
	List() ([]workspace.Workspace, error) // 内部 ID の順
	Settings(id workspace.ID) (workspace.Settings, error)
}

type ProjectReader interface {
	Get(id project.ID) (project.Project, error)
	// List は、ワークスペースのプロジェクトを、並び順の値の順(同じなら内部 ID の順)で返す。
	List(ws workspace.ID) ([]project.Project, error)
}

type ItemReader interface {
	Get(id item.ID) (item.Item, error)
	// List は、プロジェクトのすべてのアイテムを、番号の順で返す。
	List(p project.ID) ([]item.Item, error)
}

type MemberReader interface {
	List() ([]member.Member, error)
}

type ChangeReader interface {
	// Of は、エンティティのログを、書いた順に返す。
	Of(ref change.Ref) ([]change.Record, error)
}

type WorkspaceWriter interface {
	Append(id workspace.ID, drafts ...change.Draft) error
}

type ProjectWriter interface {
	Append(id project.ID, drafts ...change.Draft) error
}

type ItemWriter interface {
	Append(id item.ID, drafts ...change.Draft) error
}

type MemberWriter interface {
	Append(drafts ...change.Draft) error
}
