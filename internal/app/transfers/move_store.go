package transfers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
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
	added   []string // 移動先のメンバーに足した人のログイン名
	numbers map[item.ID]int
	prefix  string // 移動先のワークスペースの接頭辞
}

// crossSource は、別の Store へ移すときに (0) で移動元から読むもの。set の Outside は、移動元の Store 全体
// (削除済みを含む)との関連。logs・descs は移すアイテムごとの行と説明。
type crossSource struct {
	source
	project project.Project
	logs    map[item.ID][]change.Record
	descs   map[item.ID]string
	members []member.Member
}

// logList は、移すアイテムの行を、items の順で返す。
func (c crossSource) logList() [][]change.Record {
	out := make([][]change.Record, 0, len(c.logs))
	for _, it := range c.items() {
		out = append(out, c.logs[it.ID])
	}
	return out
}

// gather は、移動元から移すもの・行・説明・メンバーを読む。検査は readSource と同じ。
func gather(r port.Reader, id item.ID) (crossSource, error) {
	src, err := readSource(r, id)
	if err != nil {
		return crossSource{}, err
	}
	all, err := storeItems(r)
	if err != nil {
		return crossSource{}, err
	}
	if src.set, err = transfer.Collect(id, all); err != nil {
		return crossSource{}, err
	}
	c := crossSource{source: src, logs: map[item.ID][]change.Record{}, descs: map[item.ID]string{}}
	if c.project, err = r.Projects().Get(src.set.Root.Place.Project); err != nil {
		return crossSource{}, err
	}
	if c.members, err = r.Members().List(); err != nil {
		return crossSource{}, err
	}
	for _, it := range src.items() {
		if c.logs[it.ID], err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: it.ID}); err != nil {
			return crossSource{}, err
		}
		if c.descs[it.ID], err = r.Items().Description(it.ID); err != nil {
			return crossSource{}, err
		}
	}
	return c, nil
}

// storeItems は、Store のすべてのアイテム(全ワークスペース・全プロジェクト・削除済みを含む)。
func storeItems(r port.Reader) ([]item.Item, error) {
	wss, err := r.Workspaces().List()
	if err != nil {
		return nil, err
	}
	var all []item.Item
	for _, ws := range wss {
		ps, err := r.Projects().List(ws.ID)
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			its, err := r.Items().List(p.ID)
			if err != nil {
				return nil, err
			}
			all = append(all, its...)
		}
	}
	return all, nil
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
	m.added = m.added[:0]
	for _, p := range add {
		m.added = append(m.added, p.Login)
	}
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

// writeSource は、移動元の保存の fn: 移すアイテムのフォルダを消し、外のアイテムが持つ関連を remove し、転送記録を
// アイテムごとに 1 行足す。移すアイテムがもうない(別の経路で消えた・移った)なら port.ErrNotFound で、保存しない。
func (m *crossMove) writeSource(tx port.Tx) error {
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

// rollback は、移動先の保存を戻す(A5a-6): 写したフォルダを消し、足したメンバーを外す。number の行は戻さない。
// cause を返す。戻しも失敗したら errors.Join(cause, 戻しのエラー)。
func (s Service) rollback(ctx context.Context, m *crossMove, cause error) error {
	op := port.Operation{Actor: change.User(m.meTo), Summary: m.moved.Display + " の移動を取り消し"}
	reason, err := s.Delivery.Deliver(ctx, m.in.ToStore, op, m.undo)
	if err == nil && reason != "" {
		err = &SendError{Store: m.to.Name, Reason: reason}
	}
	if err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

// undo は、戻しの保存の fn。保存されなかった(port.ErrSaveNotDiscarded で、捨てられていた)フォルダは飛ばす。
func (m *crossMove) undo(tx port.Tx) error {
	for _, it := range m.src.items() {
		if err := tx.WriteItems().Remove(it.ID); err != nil && !errors.Is(err, port.ErrNotFound) {
			return err
		}
	}
	ds := make([]change.Draft, 0, len(m.added))
	for _, login := range m.added {
		ds = append(ds, change.NewRemove("members", login))
	}
	return tx.WriteMembers().Append(ds...)
}
