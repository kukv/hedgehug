package transfers

import (
	"errors"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// writeDestination は、移動先の保存の fn(B5c-6)。同じ接頭辞・同じ名前の断り、rank、足す人と綴りの対応は、毎回 tx から
// 決める(Review Focus 1)。移動元の Store は読まない。足す人の join、ワークスペース・プロジェクト・アイテムの写しと行を、
// 1 つの操作で書く。
func (m *crossWorkspaceMove) writeDestination(tx port.Tx) error {
	live, err := port.LiveWorkspaces(tx)
	if err != nil {
		return err
	}
	if err := refuseWorkspaceMove(live, m.src.ws, m.to.Name); err != nil {
		return err
	}
	next, err := nextWorkspaceRank(tx)
	if err != nil {
		return err
	}
	toMembers, err := tx.Members().List()
	if err != nil {
		return err
	}
	add, people := m.src.people(toMembers)
	m.added = add
	errs := []error{join(tx, add), m.importWorkspace(tx, next, people)}
	for _, p := range m.src.set.Projects {
		errs = append(errs, m.importProject(tx, p, people))
	}
	for _, it := range m.src.set.Items {
		errs = append(errs, m.importItem(tx, it, people))
	}
	return errors.Join(errs...)
}

// nextWorkspaceRank は、Store のワークスペース(削除済みを含む。B5b-4)の最後の rank の後ろ。
func nextWorkspaceRank(r port.Reader) (string, error) {
	wss, err := r.Workspaces().List()
	if err != nil {
		return "", err
	}
	ranks := make([]string, 0, len(wss))
	for _, w := range wss {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: w.ID})
		if err != nil {
			return "", err
		}
		ranks = append(ranks, workspace.RankOf(log))
	}
	return workspace.NextRank(ranks...), nil
}

// importWorkspace は、ワークスペースのフォルダ(行)をそのまま写し、moved_in(旧 ID は接頭辞)、rank、設定とルールの人の綴りの
// 書き直し、別の画面での削除の取り消しを続けて書く。
func (m *crossWorkspaceMove) importWorkspace(tx port.Tx, next string, people transfer.PeopleMap) error {
	ws := m.src.ws
	if err := tx.WriteWorkspaces().Import(ws.ID, m.src.wsLog); err != nil {
		return err
	}
	var from any
	if r := workspace.RankOf(m.src.wsLog); r != "" {
		from = r
	}
	drafts := []change.Draft{m.movedIn(ws.Prefix, "", "", nil, nil), change.NewSet("rank", from, next)}
	drafts = append(drafts, transfer.RemapSettingsPeople(m.src.settings, automation.FromLog(m.src.wsLog), people)...)
	drafts = append(drafts, m.undoDeletion(m.src.wsLog)...)
	return tx.WriteWorkspaces().Append(ws.ID, drafts...)
}

// importProject は、プロジェクト p のフォルダ(行と説明)をそのまま写し、moved_in、オーナーの綴りが移動先で変わるなら owner、
// 別の画面での削除の取り消しを続けて書く。
func (m *crossWorkspaceMove) importProject(tx port.Tx, p project.Project, people transfer.PeopleMap) error {
	log := m.src.logs[p.ID]
	if err := tx.WriteProjects().Import(p.ID, log, m.src.descs[p.ID]); err != nil {
		return err
	}
	drafts := []change.Draft{m.movedIn(p.DisplayID(m.src.ws.Prefix), p.ID, p.Title, nil, m.src.moves[p.ID])}
	if to, ok := people[strings.ToLower(p.Owner)]; ok && to != p.Owner {
		drafts = append(drafts, change.NewSet("owner", p.Owner, to))
	}
	drafts = append(drafts, m.undoDeletion(log)...)
	return tx.WriteProjects().Append(p.ID, drafts...)
}

// importItem は、アイテム it のフォルダ(行と説明)をそのまま写し、moved_in、人の綴りの書き直し、外の相手との関連の
// remove(持つ側が it のもの)、別の画面での削除の取り消しを続けて書く。
func (m *crossWorkspaceMove) importItem(tx port.Tx, it item.Item, people transfer.PeopleMap) error {
	log := m.src.logs[it.ID]
	if err := tx.WriteItems().Import(it.ID, log, m.src.descs[it.ID]); err != nil {
		return err
	}
	title := ""
	if i := slices.IndexFunc(m.src.set.Projects, func(p project.Project) bool { return p.ID == it.Place.Project }); i >= 0 {
		title = m.src.set.Projects[i].Title
	}
	drafts := []change.Draft{m.movedIn(it.DisplayID(m.src.ws.Prefix), it.Place.Project, title, namesOf(log, m.src.vocab), m.src.moves[it.ID])}
	drafts = append(drafts, transfer.RemapPeople(it, m.src.settings, people)...)
	for _, c := range m.src.set.Outside {
		if c.From == it.ID {
			drafts = append(drafts, change.NewRemove("relations", item.Relation{Type: c.Type, To: c.To}))
		}
	}
	drafts = append(drafts, m.undoDeletion(log)...)
	return tx.WriteItems().Append(it.ID, drafts...)
}

// movedIn は、旧 ID old のものの moved_in の行。移動元のワークスペースは移すワークスペース自身、プロジェクトは project
// (ワークスペースの行なら空)。
func (m *crossWorkspaceMove) movedIn(old string, project change.ID, title string, names map[change.ID]string, moves []transfer.Move) change.Draft {
	return transfer.NewMovedIn(transfer.MovedInInfo{
		Old: old, FromStore: string(m.from.ID), FromStoreName: m.from.Name,
		FromWorkspace: m.src.ws.ID, FromWorkspaceName: m.src.ws.Name, FromProject: project, FromProjectTitle: title,
		At: m.at, Names: names, Moves: moves,
	})
}

// undoDeletion は、ワークスペースが別の画面で削除されていたとき、写した行 log のうちその削除の操作の行を取り消す行
// (B5c-10。port.ReviveWorkspace と同じく、その操作の行を持つものすべて)。その操作の行がなければ空。
func (m *crossWorkspaceMove) undoDeletion(log []change.Record) []change.Draft {
	if m.revive == "" || !slices.ContainsFunc(log, func(r change.Record) bool { return r.Op == m.revive }) {
		return nil
	}
	return change.Undo(log, m.revive)
}

// writeSource は、移動元の保存の fn(B5c-8): アイテム・プロジェクト・ワークスペースのフォルダを消し、外のアイテムが持つ関連
// (tx から集め直したもの。prepare のあとに足された関連も含む)を remove し、ワークスペースの転送記録を 1 行書く。
// ワークスペースがもうなければ port.ErrNotFound で、保存しない。
func (m *crossWorkspaceMove) writeSource(tx port.Tx) error {
	now, err := gatherWorkspace(tx, m.src.ws.ID)
	if err != nil {
		return err
	}
	for _, it := range m.src.set.Items {
		if err := tx.WriteItems().Remove(it.ID); err != nil {
			return err
		}
	}
	for _, p := range m.src.set.Projects {
		if err := tx.WriteProjects().Remove(p.ID); err != nil {
			return err
		}
	}
	if err := tx.WriteWorkspaces().Remove(m.src.ws.ID); err != nil {
		return err
	}
	inside := map[item.ID]bool{}
	for _, it := range now.set.Items {
		inside[it.ID] = true
	}
	for _, c := range now.set.Outside {
		if inside[c.From] {
			continue
		}
		if err := tx.WriteItems().Append(c.From, change.NewRemove("relations", item.Relation{Type: c.Type, To: c.To})); err != nil {
			return err
		}
	}
	return tx.WriteTransfers().Append(transfer.Record{
		Entity: transfer.EntityWorkspace, Target: m.src.ws.ID, Old: m.src.ws.Prefix, New: m.src.ws.Prefix,
		ToStore: string(m.to.ID), ToStoreName: m.to.Name, ToRemote: m.remote, At: m.at,
	})
}
