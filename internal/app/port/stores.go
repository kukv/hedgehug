// Package port は、usecase が外に触るための口を決める。口は「何をしたいか」の言葉で決め、
// git の言葉(コミット、ブランチ、clone)を使わない(設計書 3 節)。
// 口は 1 つ 1 ファイルに置く。
package port

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
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

// CheckTransfer は、転送記録の行を書けるかを確かめる。Entity が transfer.EntityItem・EntityProject・
// EntityWorkspace のどれでもない・Target が change.ValidID でない・ToStore が空なら、ErrInvalidOperation を包んだエラー。
func CheckTransfer(r transfer.Record) error {
	switch {
	case r.Entity != transfer.EntityItem && r.Entity != transfer.EntityProject && r.Entity != transfer.EntityWorkspace,
		!change.ValidID(r.Target), r.ToStore == "":
		return fmt.Errorf("%w: transfer record %+v", ErrInvalidOperation, r)
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
// ワークスペースも同じで、workspace.Workspace.Deleted の印つきで返す(B5b)。
// 利用者に見せる・操作する側は、下の Live… で読む。削除済みを返さず、Get は port.ErrNotFound にする(A4b-4)。
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

// LiveWorkspace は、削除済みでないワークスペースを返す。ないか削除済みなら ErrNotFound。
func LiveWorkspace(r Reader, id workspace.ID) (workspace.Workspace, error) {
	w, err := r.Workspaces().Get(id)
	if err == nil && w.Deleted != nil {
		return workspace.Workspace{}, ErrNotFound
	}
	return w, err
}

// EditableWorkspace は、利用者が変更を保存できるワークスペースを返す。削除済みも返し(保存で削除を取り消す。
// 決定 R11)、ないときだけ ErrNotFound。
func EditableWorkspace(r Reader, id workspace.ID) (workspace.Workspace, error) {
	return r.Workspaces().Get(id)
}

// DeletedWithWorkspace は、プロジェクト p がワークスペースの削除で一緒に削除された(root: false で、ワークスペースの
// 削除と同じ op)なら true。そのワークスペースへの保存で、プロジェクトも一緒に戻る(決定 R11)。
func DeletedWithWorkspace(r Reader, p project.Project) (bool, error) {
	if p.Deleted == nil || p.Deleted.Root {
		return false, nil
	}
	w, err := r.Workspaces().Get(p.Place.Workspace)
	return err == nil && w.Deleted != nil && w.Deleted.Op == p.Deleted.Op, err
}

// LiveWorkspaces は、削除済みでないワークスペースを、内部 ID の順で返す。
func LiveWorkspaces(r Reader) ([]workspace.Workspace, error) {
	ws, err := r.Workspaces().List()
	return workspace.Live(ws), err
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
	// 行を 1 つ以上書くとき、説明を書き換えるとき、アイテムのフォルダを消すとき、または転送記録を書くときに
	// op.Check がエラーなら、何も残さずそのエラーを返す。どれもない Write は何も残さず、op を確かめない(Summary が空でもよい)。
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
	Transfers() TransferReader
	Purged() PurgedReader
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
	WriteTransfers() TransferWriter
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
	// Import と Remove は、ItemWriter と同じ(対象がワークスペースのフォルダ。説明はないので Import は取らない)。
	// Remove は、下のプロジェクト・アイテムのフォルダは消さない(呼ぶ側が 1 つずつ消す)。
	Import(id workspace.ID, log []change.Record) error
	Remove(id workspace.ID) error
}

type ProjectWriter interface {
	Append(id project.ID, drafts ...change.Draft) error
	// Describe は、説明の本文を書く。空の本文はファイルを消す。いまと同じなら何もしない。
	// 履歴の desc の行は書かない(Append で change.NewDesc を足す)。ないプロジェクトは ErrNotFound、
	// change.ValidID でない ID は ErrInvalidOperation を包んだエラー。ほかの Append と同じ 1 回の Write で残る。
	Describe(id project.ID, text string) error
	// Import と Remove は、ItemWriter と同じ(対象がプロジェクトのフォルダ)。Remove は、下のアイテムのフォルダは
	// 消さない(呼ぶ側がアイテムごとに消す)。
	Import(id project.ID, log []change.Record, description string) error
	Remove(id project.ID) error
}

type ItemWriter interface {
	Append(id item.ID, drafts ...change.Draft) error
	// Describe は、アイテムの説明を書く。空の本文・ないアイテム・不正な ID の扱いは ProjectWriter.Describe と同じ。
	Describe(id item.ID, text string) error
	// Import は、別の Store のアイテムの行(log)を、id のフォルダに変えずに(id・op・at・by・actor・prev を
	// そのまま)書く。description が空でなければ説明にする。同じ Tx で続けて Append した行は、写した行の先端に
	// つながる。id が change.ValidID でない・log が空・そのアイテムの行がもうある(同じ Tx で書いたものを含む)・
	// 同じ Tx で Remove したものは、何も書かず ErrInvalidOperation を包んだエラー。log の中身は調べない。
	Import(id item.ID, log []change.Record, description string) error
	// Remove は、アイテムのフォルダ(行・説明)を消す。同じ Tx で書いた行と説明も捨てる。同じ Tx から読むと、
	// もうない。ないアイテムは ErrNotFound、change.ValidID でない ID は ErrInvalidOperation を包んだエラー。
	Remove(id item.ID) error
}

type MemberWriter interface {
	Append(drafts ...change.Draft) error
}

// TransferReader は、転送記録(transfers.jsonl)を読む。
type TransferReader interface {
	// List は、ファイルの並びで返す。同じ Tx で書いた分は、そのあとに並ぶ。
	List() ([]transfer.Record, error)
}

// TransferWriter は、転送記録に行を足す。
type TransferWriter interface {
	// Append は、行を足す。ID・Op・By は Tx が埋める(Op はその操作のもの、By は実行者)。At は、空ならその操作の
	// 時刻を、渡せばその時刻を書く(別の Store への移動は、移動先の moved_in と同じ時刻を渡す)。
	// Entity が transfer.EntityItem・EntityProject・EntityWorkspace のどれでもない・Target が change.ValidID で
	// ない・ToStore が空なら、何も足さず ErrInvalidOperation を包んだエラー。
	Append(r transfer.Record) error
}

// PurgedReader は、完全に削除したものの記録(purged.jsonl)を読む。書くのは設定の「削除済み」(G)。
type PurgedReader interface {
	// List は、ファイルの並びで返す。読めない行は飛ばす。ファイルがなければ空。
	List() ([]Purged, error)
}

// Purged は、purged.jsonl の 1 行。
type Purged struct {
	Entity  string // item・project・workspace・sprint・release
	Target  change.ID
	Display string
	At      time.Time
}
