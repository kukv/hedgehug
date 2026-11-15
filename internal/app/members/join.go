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
	login := op.Actor.Login
	var joined bool
	err := j.Stores.Write(ctx, id, op, func(tx port.Tx) error {
		var err error
		if joined, err = j.join(ctx, tx, id, login); err != nil {
			return err
		}
		if !joined {
			return fn(tx)
		}
		c := &counting{Tx: tx}
		if err := fn(c); err != nil {
			return err
		}
		if c.writes == 0 {
			return errNothingWritten
		}
		return nil
	})
	if errors.Is(err, errNothingWritten) {
		return nil
	}
	if err == nil && joined {
		port.NoteJoined(ctx, login)
	}
	return err
}

// join は、メンバーに login(大文字と小文字を区別しない)がいなくて、id が閲覧のみの Store でなければ足し、足したかを
// 返す。Store の一覧(config.yaml)は、メンバーにいないと分かったときだけ読む(読めなくても、メンバーの書き込みは通す)。
func (j joining) join(ctx context.Context, tx port.Tx, id port.StoreID, login string) (bool, error) {
	ms, err := tx.Members().List()
	if err != nil {
		return false, err
	}
	if slices.ContainsFunc(ms, func(m member.Member) bool { return strings.EqualFold(m.Login, login) }) {
		return false, nil
	}
	if ro, err := j.readOnly(ctx, id); err != nil || ro {
		return false, err
	}
	return true, tx.WriteMembers().Append(member.Join(login))
}

// readOnly は、id が閲覧のみの Store か。一覧にない Store は閲覧のみではないとする。
func (j joining) readOnly(ctx context.Context, id port.StoreID) (bool, error) {
	infos, err := j.list.List(ctx)
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
