package transfers

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// projectGathered は、別の Store へ移すときに prepare で移動元から読むもの。projectLog・projectDesc はプロジェクトの行と
// 説明、projectMoves・moves はプロジェクト・アイテムごとの前に同じ Store の中で移した記録、logs・descs はアイテムごとの行・
// 説明、vocab は moved_in の names を引く設定。
type projectGathered struct {
	projectSource
	projectLog   []change.Record
	projectDesc  string
	projectMoves []transfer.Move
	logs         map[item.ID][]change.Record
	descs        map[item.ID]string
	moves        map[item.ID][]transfer.Move
	members      []member.Member
	vocab        []workspace.Settings
}

// crossProjectMove は、別の Store への 1 回のプロジェクトの移動。src は prepare で移動元から読んだもの。plan・added は、
// 移動先の保存の fn が呼ばれるたびに tx から作り直す(送信が重なると呼び直されるので、最後に呼ばれたときの値が保存された値)。
type crossProjectMove struct {
	src          projectGathered
	in           ProjectMoveInput
	from, to     port.StoreInfo
	remote       string // 移動先の remote(転送記録の to_remote)
	at           time.Time
	summary      string
	meFrom, meTo string
	newID        func() change.ID
	// revive は、プロジェクトが別の画面で削除されていたときの、その削除の操作(取り消す)。削除されていなければ空。
	revive change.ID

	plan  projectPlan
	added []member.Member // 移動先のメンバーに足した人
}

// moveProjectAcross は、プロジェクト id を別の Store のワークスペースへ移す(A5b-12)。移動元を読み、移動先を Delivery.Deliver
// で保存して送り、移動元を Stores.Write で保存する。片方の Store のロックを持ったまま、もう片方に触らない。送れなければ
// *SendError で、どちらの Store も変えない。移動元の保存が失敗したら移動先を戻す(A5b-14。戻せなければ *UndoError)。
func (s Service) moveProjectAcross(ctx context.Context, store port.StoreID, id project.ID, in ProjectMoveInput) (ProjectMoved, error) {
	m, err := s.prepareProject(ctx, store, id, in)
	if err != nil {
		return ProjectMoved{}, err
	}
	op := port.Operation{Actor: change.User(m.meTo), Summary: m.summary}
	reason, err := s.Delivery.Deliver(ctx, in.ToStore, op, m.writeDestination)
	switch {
	case errors.Is(err, port.ErrSaveNotDiscarded):
		return ProjectMoved{}, s.rollbackProject(ctx, m, err) // 保存されたかもしれない。移動元は保存しない
	case err != nil:
		return ProjectMoved{}, err
	case reason != "":
		return ProjectMoved{}, &SendError{Store: m.to.Name, Reason: reason}
	}
	op = port.Operation{Actor: change.User(m.meFrom), Summary: m.summary}
	if err := s.Stores.Write(ctx, store, op, m.writeSource); err != nil {
		return ProjectMoved{}, s.rollbackProject(ctx, m, err)
	}
	if m.revive != "" {
		port.NoteRevived(ctx, port.Revival{Kind: port.RevivedID, Text: m.old()})
	}
	return ProjectMoved{ToStore: in.ToStore, Project: id, Display: m.plan.display(), Destination: m.plan.dstName}, nil
}

// prepareProject は、移動元を読み(ロックは読むあいだだけ)、続けて移動先を読んで断り(A5b-9)を調べ、アクティビティの 1 行と
// 実行者を決める。移動先から読んだものは、アクティビティの文のほかは使わない(fn の中で tx から読み直す)。
func (s Service) prepareProject(ctx context.Context, store port.StoreID, id project.ID, in ProjectMoveInput) (*crossProjectMove, error) {
	m := &crossProjectMove{in: in, at: s.Clock.Now(), newID: s.IDs.New}
	var err error
	if m.from, err = s.storeInfo(ctx, store); err != nil {
		return nil, err
	}
	if m.to, err = s.storeInfo(ctx, in.ToStore); err != nil {
		return nil, err
	}
	if err := s.Stores.Read(ctx, store, func(r port.Reader) (err error) { m.src, err = gatherProject(r, id); return }); err != nil {
		return nil, err
	}
	if d := m.src.project.Deleted; d != nil {
		m.revive = d.Op
	}
	if err := s.Stores.Read(ctx, in.ToStore, func(r port.Reader) error {
		dst, err := readProjectTarget(r, in.ToWorkspace)
		if err != nil {
			return err
		}
		p, err := newProjectPlan(m.src.projectSource, dst, m.srcName(), placeName(m.to, dst.ws, true), nil, nil, noID)
		if err != nil {
			return err
		}
		m.summary = p.summary()
		return nil
	}); err != nil {
		return nil, err
	}
	if m.remote, err = s.remoteOf(ctx, in.ToStore); err != nil {
		return nil, err
	}
	if m.meTo, err = s.Identity.Me(ctx, in.ToStore); err != nil {
		return nil, err
	}
	m.meFrom, err = s.Identity.Me(ctx, store)
	return m, err
}

// gatherProject は、移動元から移すもの・行・説明・メンバーを読む。別の画面で削除されたプロジェクトも読む(A5b-15)。
func gatherProject(r port.Reader, id project.ID) (projectGathered, error) {
	src, err := readProjectSource(r, id, port.EditableProject)
	if err != nil {
		return projectGathered{}, err
	}
	if src.set, err = revivedSet(r, src); err != nil {
		return projectGathered{}, err
	}
	g := projectGathered{projectSource: src, logs: map[item.ID][]change.Record{}, descs: map[item.ID]string{}, moves: map[item.ID][]transfer.Move{}}
	if g.projectLog, err = r.Changes().Of(change.Ref{Kind: change.KindProject, ID: id}); err != nil {
		return projectGathered{}, err
	}
	if g.projectDesc, err = r.Projects().Description(id); err != nil {
		return projectGathered{}, err
	}
	if g.members, err = r.Members().List(); err != nil {
		return projectGathered{}, err
	}
	recs, err := r.Transfers().List()
	if err != nil {
		return projectGathered{}, err
	}
	if g.projectMoves, err = earlierMoves(r, transfer.EntityProject, id, g.projectLog, recs); err != nil {
		return projectGathered{}, err
	}
	for _, it := range src.set.Items() {
		if g.logs[it.ID], err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: it.ID}); err != nil {
			return projectGathered{}, err
		}
		if g.descs[it.ID], err = r.Items().Description(it.ID); err != nil {
			return projectGathered{}, err
		}
		if g.moves[it.ID], err = earlierMoves(r, transfer.EntityItem, it.ID, g.logs[it.ID], recs); err != nil {
			return projectGathered{}, err
		}
	}
	g.vocab, err = vocabOf(r, src.settings, g.itemLogs())
	return g, err
}

// itemLogs は、移すアイテムの行を、set.Items の順で返す。
func (g projectGathered) itemLogs() [][]change.Record {
	var out [][]change.Record
	for _, it := range g.set.Items() {
		out = append(out, g.logs[it.ID])
	}
	return out
}

// extraPeople は、アイテムと行のほかに移動先へ足す人の候補: プロジェクトのオーナーと、専用ルールの担当者の値(A5b-13)。
func (g projectGathered) extraPeople() []string {
	out := []string{g.project.Owner}
	for _, r := range g.own {
		if r.Action == automation.ActionAssignee && r.Value != automation.Unassigned {
			out = append(out, r.Value)
		}
	}
	return out
}

func (m *crossProjectMove) srcName() string { return placeName(m.from, m.src.ws, true) }

// old は、移動前のプロジェクトの ID(HH-P1)。
func (m *crossProjectMove) old() string { return m.src.project.DisplayID(m.src.ws.Prefix) }

// writeDestination は、移動先の保存の fn。同じ名前の断り・番号・メンバー・順位・ルールの引き継ぎは、毎回 tx から決める
// (Review Focus 1)。移動元の Store は読まない。足す人の join、プロジェクトとアイテムの写しと行、移動先のワークスペースの
// 番号の行とルールの行を、1 つの操作で書く。
func (m *crossProjectMove) writeDestination(tx port.Tx) error {
	dst, err := readProjectTarget(tx, m.in.ToWorkspace)
	if err != nil {
		return err
	}
	toMembers, err := tx.Members().List()
	if err != nil {
		return err
	}
	logs := append(m.src.itemLogs(), m.src.projectLog)
	add, people := transfer.People(m.src.settings, m.src.set.Items(), logs, m.src.extraPeople(), m.src.members, toMembers)
	p, err := newProjectPlan(m.src.projectSource, dst, m.srcName(), placeName(m.to, dst.ws, true), m.in.StatusChoices, people, m.newID)
	if err != nil {
		return err
	}
	m.plan, m.added = p, add
	errs := []error{join(tx, add), m.importProject(tx, people)}
	for _, it := range m.src.set.Items() {
		errs = append(errs, m.importItem(tx, it))
	}
	ws := append([]change.Draft{numbering.Draft(numbering.Projects, p.number)}, numberLines(p.numbers)...)
	errs = append(errs, tx.WriteWorkspaces().Append(dst.ws.ID, append(ws, p.carried.Destination...)...))
	return errors.Join(errs...)
}

// importProject は、プロジェクトのフォルダ(行と説明)をそのまま写し、moved_in(FromProject はプロジェクト自身、names は空、moves は前の移動)、
// place と rank、オーナーの綴りが移動先で変わるなら owner、別の画面での削除の取り消しを続けて書く。
func (m *crossProjectMove) importProject(tx port.Tx, people transfer.PeopleMap) error {
	pr := m.src.project
	if err := tx.WriteProjects().Import(pr.ID, m.src.projectLog, m.src.projectDesc); err != nil {
		return err
	}
	drafts := []change.Draft{m.movedIn(m.old(), nil, m.src.projectMoves)}
	drafts = append(drafts, m.plan.projectLines()...)
	if to, ok := people[strings.ToLower(pr.Owner)]; ok && to != pr.Owner {
		drafts = append(drafts, change.NewSet("owner", pr.Owner, to))
	}
	drafts = append(drafts, m.undoDeletion(m.src.projectLog)...)
	return tx.WriteProjects().Append(pr.ID, drafts...)
}

// importItem は、アイテム it のフォルダをそのまま写し、moved_in、place と引き直しの行、外の相手との関連の remove(持つ側が
// it のもの)、別の画面での削除の取り消しを続けて書く。
func (m *crossProjectMove) importItem(tx port.Tx, it item.Item) error {
	log := m.src.logs[it.ID]
	if err := tx.WriteItems().Import(it.ID, log, m.src.descs[it.ID]); err != nil {
		return err
	}
	drafts := []change.Draft{m.movedIn(it.DisplayID(m.src.ws.Prefix), namesOf(log, m.src.vocab), m.src.moves[it.ID])}
	drafts = append(drafts, m.plan.itemLines(it)...)
	for _, c := range m.src.set.Outside {
		if c.From == it.ID {
			drafts = append(drafts, change.NewRemove("relations", item.Relation{Type: c.Type, To: c.To}))
		}
	}
	drafts = append(drafts, m.undoDeletion(log)...)
	return tx.WriteItems().Append(it.ID, drafts...)
}

// movedIn は、旧 ID old のものの moved_in の行。移動元のプロジェクトは移すプロジェクト。
func (m *crossProjectMove) movedIn(old string, names map[change.ID]string, moves []transfer.Move) change.Draft {
	return transfer.NewMovedIn(transfer.MovedInInfo{
		Old: old, FromStore: string(m.from.ID), FromStoreName: m.from.Name,
		FromWorkspace: m.src.ws.ID, FromWorkspaceName: m.src.ws.Name, FromProject: m.src.project.ID, FromProjectTitle: m.src.project.Title,
		At: m.at, Names: names, Moves: moves,
	})
}

// undoDeletion は、プロジェクトが別の画面で削除されていたとき、写した行 log のうちその削除の操作の行を取り消す行
// (A5b-15。同じ操作で一緒に削除された下のものも)。その操作の行がなければ空。
func (m *crossProjectMove) undoDeletion(log []change.Record) []change.Draft {
	if m.revive == "" || !slices.ContainsFunc(log, func(r change.Record) bool { return r.Op == m.revive }) {
		return nil
	}
	return change.Undo(log, m.revive)
}

// writeSource は、移動元の保存の fn: アイテムとプロジェクトのフォルダを消し、外のアイテムが持つ関連(tx から集め直したもの)を
// remove し、移動元のワークスペースのルールを消す行と、プロジェクトとアイテムごとの転送記録を書く。移すものかルールが prepare と
// 違えば errSourceChanged で、保存しない(A5b-12)。
func (m *crossProjectMove) writeSource(tx port.Tx) error {
	now, err := m.checkSource(tx)
	if err != nil {
		return err
	}
	for _, it := range m.src.set.Items() {
		if err := tx.WriteItems().Remove(it.ID); err != nil {
			return err
		}
	}
	if err := tx.WriteProjects().Remove(m.src.project.ID); err != nil {
		return err
	}
	var errs []error
	for _, c := range now.Outside {
		if !now.Contains(c.From) {
			errs = append(errs, tx.WriteItems().Append(c.From, change.NewRemove("relations", item.Relation{Type: c.Type, To: c.To})))
		}
	}
	if len(m.plan.carried.Source) > 0 {
		errs = append(errs, tx.WriteWorkspaces().Append(m.src.ws.ID, m.plan.carried.Source...))
	}
	for _, r := range m.plan.records(m.to.ID) {
		r.ToStoreName, r.ToRemote, r.At = m.to.Name, m.remote, m.at
		errs = append(errs, tx.WriteTransfers().Append(r))
	}
	return errors.Join(errs...)
}
