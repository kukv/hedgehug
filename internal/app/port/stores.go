// Package port は、usecase が外に触るための口を決める。口は「何をしたいか」の言葉で決め、
// git の言葉(コミット、ブランチ、clone)を使わない(設計書 3 節)。
// 口は 1 つ 1 ファイルに置く。
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

// CheckRuleDrafts は、自動化の行(Draft.Rule)を書けるかを確かめる(B4-1)。自動化の行は人の操作と、
// GitHub の操作の中でだけ書け(D4a-15。by は hedgehug[bot])、ルールの ID は change.ValidID。
// 合わなければ ErrInvalidOperation を包んだエラー。
func CheckRuleDrafts(actor change.Actor, drafts []change.Draft) error {
	for _, d := range drafts {
		if d.Rule == "" {
			continue
		}
		if (actor.Kind != change.ActorUser && actor.Kind != change.ActorGitHub) || !change.ValidID(d.Rule) {
			return fmt.Errorf("%w: rule row %q by %+v", ErrInvalidOperation, d.Rule, actor)
		}
	}
	return nil
}

// CheckOpDrafts は、操作の op と別の op を持つ行(Draft.Op)を書けるかを確かめる(C3a-11)。
// 書けるのは同期の操作だけで、op は change.ValidID。合わなければ ErrInvalidOperation を包んだエラー。
func CheckOpDrafts(actor change.Actor, drafts []change.Draft) error {
	for _, d := range drafts {
		if d.Op == "" {
			continue
		}
		if actor.Kind != change.ActorSync || !change.ValidID(d.Op) {
			return fmt.Errorf("%w: op row %q by %+v", ErrInvalidOperation, d.Op, actor)
		}
	}
	return nil
}

// 口は削除済みのアイテム・プロジェクトも返す(item.Item.Deleted・project.Project.Deleted の印つき。A4b-3)。
// 利用者に見せる・操作する側は、下の 4 つで読む。削除済みを返さず、Get は port.ErrNotFound にする(A4b-4)。
// 削除済みも要る読み方(番号の採番、プロジェクトが変わる移動の下のもの、値の置き換え、自動化の最近の実行、
// 自動化で親が削除済みかの確認、Issue がすでに別のアイテムにリンクされているかの確認(07「追加」))は、口を直接呼ぶ。

// LiveItem は、削除済みでないアイテムを返す。ないか削除済みなら ErrNotFound。
func LiveItem(r Reader, id item.ID) (item.Item, error) {
	it, err := r.Items().Get(id)
	if err == nil && it.Deleted != nil {
		return item.Item{}, ErrNotFound
	}
	return it, err
}

// LiveItems は、プロジェクトの削除済みでないアイテムを、番号の順で返す。
func LiveItems(r Reader, p project.ID) ([]item.Item, error) {
	its, err := r.Items().List(p)
	return item.Live(its), err
}

// LiveProject は、削除済みでないプロジェクトを返す。ないか削除済みなら ErrNotFound。
func LiveProject(r Reader, id project.ID) (project.Project, error) {
	p, err := r.Projects().Get(id)
	if err == nil && p.Deleted != nil {
		return project.Project{}, ErrNotFound
	}
	return p, err
}

// LiveProjects は、ワークスペースの削除済みでないプロジェクトを、並び順で返す。
func LiveProjects(r Reader, ws workspace.ID) ([]project.Project, error) {
	ps, err := r.Projects().List(ws)
	return project.Live(ps), err
}

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
	// RetentionDays は、削除済みの保持日数(.hedgehug/store.yaml の retention_days。ないときは 30)。
	// 変えるのは設定の「削除済み」(G)で、ここは読むだけ。
	RetentionDays() (int, error)
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
	// Description は、プロジェクトの説明の本文を返す(description.md)。説明がなければ ""。
	// プロジェクトがなければ ErrNotFound。同じ Tx で書いた説明を含む。
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
	// Describe は、アイテムの説明を書く。空の本文・ないアイテム・不正な ID の扱いは ProjectWriter.Describe と同じ。
	Describe(id item.ID, text string) error
}

type MemberWriter interface {
	Append(drafts ...change.Draft) error
}
