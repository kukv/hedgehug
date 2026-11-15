package members

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// errNothingWritten は、fn が何も書かなかったので、足した「あなた」ごと巻き戻すための印。外へは返さない。
var errNothingWritten = errors.New("nothing written")

// JoinOnWrite は、人の書き込みのとき、その Store のメンバーに実行者がいなければ、同じ保存で足す port.Stores を返す
// (01「その書き込みと一緒に追加する」。決定 G1-6・G1-7)。閲覧のみの Store では足さない。足したのは fn を呼ぶ前で、
// fn が何も書かなかったら、足した分も保存しない。保存できたときだけ port.NoteJoined で ctx に積む。Write 以外はそのまま渡す。
func JoinOnWrite(s port.Stores, list port.StoreGitHub) port.Stores {
	return joining{Stores: s, list: list}
}

type joining struct {
	port.Stores
	list port.StoreGitHub
}

func (j joining) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	if op.Actor.Kind != change.ActorUser {
		return j.Stores.Write(ctx, id, op, fn)
	}
	w := &joinWrite{list: j.list, id: id, login: op.Actor.Login, fn: fn}
	err := j.Stores.Write(ctx, id, op, w.run(ctx))
	if errors.Is(err, errNothingWritten) {
		return nil
	}
	return w.done(ctx, err)
}

// JoinOnSyncSave は、remote のある Store への人の保存(SyncSave。移動先への送り届け)でも、JoinOnWrite と同じく、
// 実行者がメンバーにいなければ同じ保存で足す port.Replicator を返す(決定 G1-6)。メンバーかは fn の中で、試みごとに
// 受け取ったあとの Tx から決める。足したのに fn が何も書かなかったら、足した分を捨て、包まない fn で内側の SyncSave を
// 1 回呼び直す(送り・Seen・数を包まない SyncSave と同じにする。成功を記録して LastAttempt が進むので、待っていた
// ローカルの保存もここで送る)。SyncSave 以外はそのまま渡す。
func JoinOnSyncSave(r port.Replicator, list port.StoreGitHub) port.Replicator {
	return joiningReplicator{Replicator: r, list: list}
}

type joiningReplicator struct {
	port.Replicator
	list port.StoreGitHub
}

func (j joiningReplicator) SyncSave(ctx context.Context, id port.StoreID, remote port.Remote, op port.Operation, fn func(tx port.Tx) error) (port.SyncResult, error) {
	if op.Actor.Kind != change.ActorUser {
		return j.Replicator.SyncSave(ctx, id, remote, op, fn)
	}
	w := &joinWrite{list: j.list, id: id, login: op.Actor.Login, fn: fn}
	res, err := j.Replicator.SyncSave(ctx, id, remote, op, w.run(ctx))
	if errors.Is(err, errNothingWritten) { // fn のエラーは *port.SaveError に包まれて返る
		return j.Replicator.SyncSave(ctx, id, remote, op, fn)
	}
	return res, w.done(ctx, err)
}

// joinWrite は、1 回の人の書き込みの fn を包み、「あなた」を足したかを覚える(JoinOnWrite と JoinOnSyncSave で共通)。
type joinWrite struct {
	list  port.StoreGitHub
	id    port.StoreID
	login string
	fn    func(tx port.Tx) error
	// joined は、最後に呼ばれた fn で足したか。SyncSave は送信が重なると fn を呼び直すので、呼ぶたびに決め直す。
	joined bool
}

// run は、保存の fn。メンバーにいなければ、利用者の fn を呼ぶ前に足す。足したのに fn が何も書かなかったら
// errNothingWritten を返して、足した分も保存させない。
func (w *joinWrite) run(ctx context.Context) func(tx port.Tx) error {
	return func(tx port.Tx) error {
		var err error
		if w.joined, err = w.join(ctx, tx); err != nil {
			return err
		}
		if !w.joined {
			return w.fn(tx)
		}
		c := &counting{Tx: tx}
		if err := w.fn(c); err != nil {
			return err
		}
		if c.writes == 0 {
			return errNothingWritten
		}
		return nil
	}
}

// done は、保存の結果 err を受け、保存できて足していたら port.NoteJoined で ctx に積む。errNothingWritten は呼ぶ側が先に扱う。
func (w *joinWrite) done(ctx context.Context, err error) error {
	if err == nil && w.joined {
		port.NoteJoined(ctx, w.login)
	}
	return err
}

// join は、メンバーに login(大文字と小文字を区別しない)がいなくて、id が閲覧のみの Store でなければ足し、足したかを
// 返す。Store の一覧(config.yaml)は、メンバーにいないと分かったときだけ読む(読めなくても、メンバーの書き込みは通す)。
func (w *joinWrite) join(ctx context.Context, tx port.Tx) (bool, error) {
	ms, err := tx.Members().List()
	if err != nil {
		return false, err
	}
	if slices.ContainsFunc(ms, func(m member.Member) bool { return strings.EqualFold(m.Login, w.login) }) {
		return false, nil
	}
	if ro, err := readOnly(ctx, w.list, w.id); err != nil || ro {
		return false, err
	}
	return true, tx.WriteMembers().Append(member.Join(w.login))
}

// readOnly は、id が閲覧のみの Store か。一覧にない Store は閲覧のみではないとする。
func readOnly(ctx context.Context, list port.StoreGitHub, id port.StoreID) (bool, error) {
	infos, err := list.List(ctx)
	if err != nil {
		return false, err
	}
	i := slices.IndexFunc(infos, func(in port.StoreGitHubInfo) bool { return in.ID == id })
	return i >= 0 && infos[i].ReadOnly, nil
}

// counting は、fn が書き口を呼んで何かを書いた回数を数える Tx。行のない Append は何も書かないので数えない。
type counting struct {
	port.Tx
	writes int
}

func (c *counting) WriteWorkspaces() port.WorkspaceWriter {
	return countWorkspaces{c.Tx.WriteWorkspaces(), &c.writes}
}

func (c *counting) WriteProjects() port.ProjectWriter {
	return countProjects{c.Tx.WriteProjects(), &c.writes}
}

func (c *counting) WriteItems() port.ItemWriter { return countItems{c.Tx.WriteItems(), &c.writes} }

func (c *counting) WriteMembers() port.MemberWriter {
	return countMembers{c.Tx.WriteMembers(), &c.writes}
}

func (c *counting) WriteTransfers() port.TransferWriter {
	return countTransfers{c.Tx.WriteTransfers(), &c.writes}
}

type countWorkspaces struct {
	port.WorkspaceWriter
	n *int
}

func (w countWorkspaces) Append(id workspace.ID, drafts ...change.Draft) error {
	if len(drafts) > 0 {
		*w.n++
	}
	return w.WorkspaceWriter.Append(id, drafts...)
}

func (w countWorkspaces) Import(id workspace.ID, log []change.Record) error {
	*w.n++
	return w.WorkspaceWriter.Import(id, log)
}

func (w countWorkspaces) Remove(id workspace.ID) error {
	*w.n++
	return w.WorkspaceWriter.Remove(id)
}

type countProjects struct {
	port.ProjectWriter
	n *int
}

func (w countProjects) Append(id project.ID, drafts ...change.Draft) error {
	if len(drafts) > 0 {
		*w.n++
	}
	return w.ProjectWriter.Append(id, drafts...)
}

func (w countProjects) Describe(id project.ID, text string) error {
	*w.n++
	return w.ProjectWriter.Describe(id, text)
}

func (w countProjects) Import(id project.ID, log []change.Record, description string) error {
	*w.n++
	return w.ProjectWriter.Import(id, log, description)
}

func (w countProjects) Remove(id project.ID) error {
	*w.n++
	return w.ProjectWriter.Remove(id)
}

type countItems struct {
	port.ItemWriter
	n *int
}

func (w countItems) Append(id item.ID, drafts ...change.Draft) error {
	if len(drafts) > 0 {
		*w.n++
	}
	return w.ItemWriter.Append(id, drafts...)
}

func (w countItems) Describe(id item.ID, text string) error {
	*w.n++
	return w.ItemWriter.Describe(id, text)
}

func (w countItems) Import(id item.ID, log []change.Record, description string) error {
	*w.n++
	return w.ItemWriter.Import(id, log, description)
}

func (w countItems) Remove(id item.ID) error {
	*w.n++
	return w.ItemWriter.Remove(id)
}

type countMembers struct {
	port.MemberWriter
	n *int
}

func (w countMembers) Append(drafts ...change.Draft) error {
	if len(drafts) > 0 {
		*w.n++
	}
	return w.MemberWriter.Append(drafts...)
}

type countTransfers struct {
	port.TransferWriter
	n *int
}

func (w countTransfers) Append(r transfer.Record) error {
	*w.n++
	return w.TransferWriter.Append(r)
}
