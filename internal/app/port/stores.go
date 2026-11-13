// Package port は、usecase が外に触るための口を決める。口は「何をしたいか」の言葉で決め、
// git の言葉(コミット、ブランチ、clone)を使わない(設計書 3 節)。
// 口は 1 つ 1 ファイルに置く。新しい口は、使うトラックが足す。
package port

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	// ErrInvalidOperation は、書けない操作(1 行でない Summary、使えないログイン名、使えない ID)。
	ErrInvalidOperation = errors.New("invalid operation")
)

// Stores は Store のデータの口。1 回の操作 = 1 回の Write。
type Stores interface {
	Read(ctx context.Context, storeID StoreID, fn func(r Reader) error) error
	// Write は fn がエラーなら何も残さず、errors.Is で同じエラーと分かるエラーを返す(包んでよい)。
	// 行を 1 つ以上書くとき、または説明を書き換えるときに op.Check がエラーなら、何も残さずそのエラーを返す。
	// 行も説明の書き換えもない Write は何も残さず、op を確かめない(Summary が空でもよい)。
	// 同じ Store への Write は 1 つずつ順に行う。別の Store への Write は互いを待たない(Write の中から
	// 別の Store に書いてよい)。
	Write(ctx context.Context, storeID StoreID, op Operation, fn func(tx Tx) error) error
}

// Operation は 1 回の操作。
type Operation struct {
	Actor   change.Actor // あなた・自動化・同期・GitHub
	Summary string       // アクティビティの 1 行
}

// Check は、行を書ける操作かを確かめる。Summary が空か複数行(\n か \r を含む)、実行者が行の actor・by に
// 書いて読み戻すと同じにならない(知らない種類、ルールの ID が change.ValidID でない自動化、種類に要らない
// Login・Rule を持つ)、または人の操作のログイン名が member.ValidLogin でなければ、ErrInvalidOperation を
// 包んだエラーを返す。読み戻せない行は、読むときに読み飛ばされて消えるため。
func (op Operation) Check() error {
	if op.Summary == "" || strings.ContainsAny(op.Summary, "\r\n") {
		return fmt.Errorf("%w: summary must be one non-empty line: %q", ErrInvalidOperation, op.Summary)
	}
	a := op.Actor
	if back, err := change.ParseActor(a.Code(), a.By()); err != nil || back != a ||
		(a.Kind == change.ActorAutomation && !change.ValidID(a.Rule)) {
		return fmt.Errorf("%w: actor %+v cannot be written", ErrInvalidOperation, a)
	}
	if op.Actor.Kind == change.ActorUser && !member.ValidLogin(op.Actor.Login) {
		return fmt.Errorf("%w: login %q", ErrInvalidOperation, op.Actor.Login)
	}
	return nil
}

// Reader は Store の読み取り。種類ごとの関数は、各トラックが使うものだけを足す。
// ID が change.ValidID でない Get は ErrNotFound を返す。
type Reader interface {
	Workspaces() WorkspaceReader
	Projects() ProjectReader
	Items() ItemReader
	Members() MemberReader
	Changes() ChangeReader // 履歴とアクティビティ
}

// Tx は 1 回の Write の中の読み書き。読み取りは、同じ Tx で書いたものを含む。
// ID が change.ValidID でない Append は、何も足さず ErrInvalidOperation を包んだエラーを返す。
// 行のない Append は何も足さない。
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
	// Description は、プロジェクトの説明の本文を返す。アイテムと同じ決まり。
	Description(id project.ID) (string, error)
}

type ItemReader interface {
	Get(id item.ID) (item.Item, error)
	// List は、プロジェクトのすべてのアイテムを、番号の順で返す。
	List(p project.ID) ([]item.Item, error)
	// Description は、アイテムの説明の本文を返す(description.md)。説明がなければ ""。
	// アイテムがなければ ErrNotFound。同じ Tx で書いた説明を含む。
	Description(id item.ID) (string, error)
}

type MemberReader interface {
	List() ([]member.Member, error)
}

type ChangeReader interface {
	// Of は、エンティティのログを、書いた順に返す。ログがないか、ref が change.Ref.Valid でなければ
	// ErrNotFound。メンバーの一覧のログは Store に 1 つで、ID が空の Ref{Kind: change.KindMembers} で指す。
	Of(ref change.Ref) ([]change.Record, error)
}

type WorkspaceWriter interface {
	Append(id workspace.ID, drafts ...change.Draft) error
}

type ProjectWriter interface {
	Append(id project.ID, drafts ...change.Draft) error
	// Describe は、説明の本文を書く。空の本文はファイルを消す。いまと同じなら何もしない。
	// 履歴の desc の行は書かない(Append で change.NewDesc を足す)。ないプロジェクトは ErrNotFound、
	// change.ValidID でない ID は ErrInvalidOperation を包んだエラー。ほかの Append と同じ 1 回の Write で残る。
	Describe(id project.ID, text string) error
}

type ItemWriter interface {
	Append(id item.ID, drafts ...change.Draft) error
	// Describe は ProjectWriter.Describe と同じ決まりで、アイテムの説明を書く。
	Describe(id item.ID, text string) error
}

type MemberWriter interface {
	Append(drafts ...change.Draft) error
}
