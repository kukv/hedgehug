package transfers

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// SendError は、移動先の Store に送れなかったため移さなかったこと(A5a-12)。Store は移動先の Store 名。
type SendError struct {
	Store  string
	Reason syncstatus.Reason
}

func (e *SendError) Error() string {
	return fmt.Sprintf("%s に送信できないため移動できません: %s", e.Store, e.Reason.Short())
}

// crossMove は、別の Store への 1 回の移動。src は (0) で移動元から読んだもの。moved・added・numbers・prefix は、
// 移動先の保存の fn が呼ばれるたびに作り直す(送信が重なると呼び直されるので、最後に呼ばれたときの値が保存された値)。
type crossMove struct {
	src      crossSource
	in       MoveInput
	from, to port.StoreInfo
	remote   string // 移動先の remote(転送記録の to_remote)
	at       time.Time
	summary  string
	meFrom   string
	meTo     string

	moved   Moved
	added   []member.Member // 移動先のメンバーに足した人
	numbers map[item.ID]int
	prefix  string // 移動先のワークスペースの接頭辞
}

// moveAcross は、アイテム id を別の Store のワークスペースへ移す(A5a-4・A5a-6)。(0) 移動元を読み、(1)〜(3) 移動先を
// Delivery.Deliver で保存して送り、(4) 移動元を Stores.Write で保存する。どの段階でも、片方の Store のロックを持ったまま
// もう片方に触らない。送れなければ *SendError で、どちらの Store も変えない。(4) が失敗したら移動先を戻す。
func (s Service) moveAcross(ctx context.Context, store port.StoreID, id item.ID, in MoveInput) (Moved, error) {
	m, err := s.prepare(ctx, store, id, in)
	if err != nil {
		return Moved{}, err
	}
	op := port.Operation{Actor: change.User(m.meTo), Summary: m.summary}
	reason, err := s.Delivery.Deliver(ctx, in.ToStore, op, m.writeDestination)
	switch {
	case errors.Is(err, port.ErrSaveNotDiscarded):
		return Moved{}, s.rollback(ctx, m, err) // 保存されたかもしれない。移動元は保存しない
	case err != nil:
		return Moved{}, err
	case reason != "":
		return Moved{}, &SendError{Store: m.to.Name, Reason: reason}
	}
	op = port.Operation{Actor: change.User(m.meFrom), Summary: m.summary}
	if err := s.Stores.Write(ctx, store, op, m.writeSource); err != nil {
		return Moved{}, s.rollback(ctx, m, err)
	}
	return m.moved, nil
}

// prepare は、移動元を読み(ロックは読むあいだだけ)、続けて移動先を読んで検査し、アクティビティの 1 行と実行者を決める。
func (s Service) prepare(ctx context.Context, store port.StoreID, id item.ID, in MoveInput) (*crossMove, error) {
	m := &crossMove{in: in, at: s.Clock.Now()}
	var err error
	if m.from, err = s.storeInfo(ctx, store); err != nil {
		return nil, err
	}
	if m.to, err = s.storeInfo(ctx, in.ToStore); err != nil {
		return nil, err
	}
	if err := s.Stores.Read(ctx, store, func(r port.Reader) (err error) { m.src, err = gather(r, id); return }); err != nil {
		return nil, err
	}
	var dst destination
	if err := s.Stores.Read(ctx, in.ToStore, func(r port.Reader) (err error) {
		dst, _, err = target(r, m.src.set.Root, in)
		return
	}); err != nil {
		return nil, err
	}
	m.summary = fmt.Sprintf("%s を %s の %s から %s の %s へ移動",
		m.src.set.Root.DisplayID(m.src.ws.Prefix), m.from.Name, m.src.ws.Name, m.to.Name, dst.ws.Name)
	if m.remote, err = s.remoteOf(ctx, in.ToStore); err != nil {
		return nil, err
	}
	if m.meTo, err = s.Identity.Me(ctx, in.ToStore); err != nil {
		return nil, err
	}
	if m.meFrom, err = s.Identity.Me(ctx, store); err != nil {
		return nil, err
	}
	return m, nil
}

// writeDestination は、移動先の保存の fn(A5a-4)。移動先の番号・メンバー・引き直しは、毎回 tx から決める(A5a-5)。
// 移動元の Store は読まない。足す人の join(表示名が違えば名前も)、アイテムごとに写し・moved_in・place・引き直し・外との
// 関連の remove、番号ごとの number の行を、1 つの操作で書く。
func (m *crossMove) writeDestination(tx port.Tx) error {
	root := m.src.set.Root
	dst, newRank, err := target(tx, root, m.in)
	if err != nil {
		return err
	}
	toMembers, err := tx.Members().List()
	if err != nil {
		return err
	}
	add, people := transfer.People(m.src.settings, m.src.items(), m.src.logList(), m.src.members, toMembers)
	numbers := transfer.Number(root, m.src.set.Below, dst.next)
	remaps := map[item.ID]transfer.RemapResult{}
	errs := []error{m.join(tx, add)}
	for _, it := range m.src.items() {
		remaps[it.ID] = transfer.Remap(it, m.src.settings, dst.settings, dst.project.ID, people, m.in.StatusChoices)
		lines := placeLines(it, it.ID == root.ID, dst, m.in.ToParent, numbers[it.ID], newRank)
		errs = append(errs, m.importItem(tx, it, append(lines, remaps[it.ID].Drafts...)))
	}
	errs = append(errs, tx.WriteWorkspaces().Append(dst.ws.ID, numberLines(numbers)...))
	m.numbers, m.prefix = numbers, dst.ws.Prefix
	m.added = add
	m.moved = Moved{ToStore: m.in.ToStore, Item: root.ID, Display: displayID(dst.ws.Prefix, numbers[root.ID]), Emptied: emptiedOf(remaps, m.src.settings)}
	return errors.Join(errs...)
}

// join は、移動先のメンバーに add を足す。表示名がログイン名と違えば、表示名も書く。
func (m *crossMove) join(tx port.Tx, add []member.Member) error {
	var ds []change.Draft
	for _, p := range add {
		ds = append(ds, member.Join(p.Login))
		if p.Name != p.Login {
			ds = append(ds, member.Rename(p.Login, p.Login, p.Name))
		}
	}
	return tx.WriteMembers().Append(ds...)
}

// importItem は、it のフォルダ(行と説明)をそのまま写し、moved_in、place と引き直しの行 lines、外の相手との関連の
// remove(持つ側が it のもの)を続けて書く。
func (m *crossMove) importItem(tx port.Tx, it item.Item, lines []change.Draft) error {
	if err := tx.WriteItems().Import(it.ID, m.src.logs[it.ID], m.src.descs[it.ID]); err != nil {
		return err
	}
	drafts := []change.Draft{transfer.NewMovedIn(transfer.MovedInInfo{
		Old: it.DisplayID(m.src.ws.Prefix), FromStore: string(m.from.ID), FromStoreName: m.from.Name,
		FromWorkspace: m.src.ws.ID, FromWorkspaceName: m.src.ws.Name, FromProject: m.src.project.ID, FromProjectTitle: m.src.project.Title,
		At: m.at, Names: namesOf(m.src.logs[it.ID], m.src.settings),
	})}
	drafts = append(drafts, lines...)
	for _, c := range m.src.set.Outside {
		if c.From == it.ID {
			drafts = append(drafts, change.NewRemove("relations", item.Relation{Type: c.Type, To: c.To}))
		}
	}
	return tx.WriteItems().Append(it.ID, drafts...)
}

// errSourceChanged は、(0) で読んだあと、移動元で移すもの(移すアイテムと下のすべて)が変わったこと。
var errSourceChanged = errors.New("transfers: the items to move changed in the source store")

// writeSource は、移動元の保存の fn: 移すアイテムのフォルダを消し、外のアイテムが持つ関連を remove し、転送記録を
// アイテムごとに 1 行足す。移すアイテムがもうない(別の経路で消えた・移った)なら port.ErrNotFound、移すものが (0) と
// 違う(子が足された・移った)なら errSourceChanged で、保存しない(02「保存」の、失敗したら元のまま)。
func (m *crossMove) writeSource(tx port.Tx) error {
	if err := m.checkSource(tx); err != nil {
		return err
	}
	items := m.src.items()
	for _, it := range items {
		if err := tx.WriteItems().Remove(it.ID); err != nil {
			return err
		}
	}
	var errs []error
	for _, c := range m.src.set.Outside {
		if !m.src.set.Contains(c.From) {
			errs = append(errs, tx.WriteItems().Append(c.From, change.NewRemove("relations", item.Relation{Type: c.Type, To: c.To})))
		}
	}
	for _, it := range items {
		errs = append(errs, tx.WriteTransfers().Append(transfer.Record{
			Entity: transfer.EntityItem, Target: it.ID, Old: it.DisplayID(m.src.ws.Prefix), New: displayID(m.prefix, m.numbers[it.ID]),
			ToStore: string(m.to.ID), ToStoreName: m.to.Name, ToRemote: m.remote,
		}))
	}
	return errors.Join(errs...)
}

// checkSource は、移動元の移すもの(削除済みを含む)を tx から集め直し、(0) で集めたものと同じかを確かめる。
func (m *crossMove) checkSource(tx port.Tx) error {
	all, err := storeItems(tx)
	if err != nil {
		return err
	}
	set, err := transfer.Collect(m.src.set.Root.ID, all)
	if errors.Is(err, transfer.ErrNotFound) {
		return port.ErrNotFound
	}
	if err != nil {
		return err
	}
	if !slices.Equal(idsIn(set), idsIn(m.src.set)) {
		return errSourceChanged
	}
	return nil
}

// idsIn は、移すものの内部 ID(辞書順)。
func idsIn(s transfer.Set) []item.ID {
	ids := []item.ID{s.Root.ID}
	for _, it := range s.Below {
		ids = append(ids, it.ID)
	}
	slices.Sort(ids)
	return ids
}
