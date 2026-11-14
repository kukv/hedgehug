package repairing

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strconv"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// matchWorkspaces は、親とワークスペースが違う子を、親のワークスペースへ 1 件ずつ移す(決定 C3b-1)。
// 1 件ごとに 1 回保存し、なくなるまで読み直して繰り返す。孫は次の回で直る。
func matchWorkspaces(ctx context.Context, w port.RepairWriter) error {
	var prev repair.Stray
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		all, err := readAllItems(w)
		if err != nil {
			return err
		}
		s, ok := repair.WorkspaceMismatch(all)
		if !ok {
			return nil
		}
		if s == prev {
			return errors.New("親とワークスペースが違う子の修復が進みません")
		}
		prev = s
		if err := moveToWorkspace(w, s); err != nil {
			return err
		}
	}
}

// strayMove は、親とワークスペースが違う子の移動の材料。items は移すもの(直す子が先、続いて削除済みのものが
// 新しい番号の順)、from・froms は移すもののワークスペースごとの接頭辞と設定、dst・to・rules は移動先。
type strayMove struct {
	items   []item.Item
	numbers map[item.ID]int
	from    map[workspace.ID]workspace.Workspace
	froms   map[workspace.ID]workspace.Settings
	dst     workspace.Workspace
	to      workspace.Settings
	rules   projectrule.Rules
	project project.ID
}

// readStrayMove は、子 id の移動の材料を、1 回の読みで r から読む。移動先は親の場所から決める。
// 移動先は、親の場所を Get で読む(削除済みの移動先でも断らない。決定 C3b-9)。
func readStrayMove(r port.Reader, id item.ID) (strayMove, error) {
	m := strayMove{from: map[workspace.ID]workspace.Workspace{}, froms: map[workspace.ID]workspace.Settings{}}
	child, err := r.Items().Get(id)
	if err != nil {
		return m, err
	}
	parent, err := r.Items().Get(*child.Place.Parent)
	if err != nil {
		return m, err
	}
	if m.dst, err = r.Workspaces().Get(parent.Place.Workspace); err != nil {
		return m, err
	}
	if m.to, err = r.Workspaces().Settings(m.dst.ID); err != nil {
		return m, err
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: m.dst.ID})
	if err != nil {
		return m, err
	}
	m.project = parent.Place.Project
	m.rules = projectrule.FromLog(log).Of(m.project)
	all, err := allItemsOf(r)
	if err != nil {
		return m, err
	}
	deleted := repair.DeletedBelow(child.ID, all)
	m.numbers = repair.NumberByOld(child, deleted, numbering.Next(log, numbering.Items))
	m.items = append([]item.Item{child}, deleted...)
	slices.SortFunc(m.items, func(a, b item.Item) int { return m.numbers[a.ID] - m.numbers[b.ID] })
	for _, it := range m.items {
		if _, ok := m.from[it.Place.Workspace]; ok {
			continue
		}
		ws, err := r.Workspaces().Get(it.Place.Workspace)
		if err != nil {
			return m, err
		}
		st, err := r.Workspaces().Settings(ws.ID)
		if err != nil {
			return m, err
		}
		m.from[ws.ID], m.froms[ws.ID] = ws, st
	}
	return m, nil
}

// oldID は it の移す前の画面の ID、newID は移したあとの画面の ID({接頭辞}-{番号})。
func (m strayMove) oldID(it item.Item) string { return it.DisplayID(m.from[it.Place.Workspace].Prefix) }

func (m strayMove) newID(it item.Item) string {
	return m.dst.Prefix + "-" + strconv.Itoa(m.numbers[it.ID])
}

// moveToWorkspace は、s.Child と、その下の削除済みのものを、親のワークスペースの次の番号へ移す修復を 1 回保存する。
// 行は transfers の同じ Store の中の移動(writeWithin)と同じ形。親と順位は変えない。
func moveToWorkspace(w port.RepairWriter, s repair.Stray) error {
	var m strayMove
	err := w.Read(func(r port.Reader) error {
		var err error
		m, err = readStrayMove(r, s.Child)
		return err
	})
	if err != nil {
		return err
	}
	store, err := w.Store()
	if err != nil {
		return err
	}
	root := m.items[0]
	op := port.Operation{Actor: syncActor, Summary: m.oldID(root) + " を " + m.newID(root) + " に移動(親に合わせる)"}
	return w.Write(op, func(tx port.Tx) error {
		var errs []error
		for _, it := range m.items {
			to := item.Place{Workspace: m.dst.ID, Project: m.project, Parent: it.Place.Parent, Number: m.numbers[it.ID]}
			rm := transfer.Remap(it, m.froms[it.Place.Workspace], m.to, m.rules, nil, nil)
			ds := append([]change.Draft{change.NewSet(placeField, it.Place, to)}, rm.Drafts...)
			errs = append(errs, tx.WriteItems().Append(it.ID, ds...))
		}
		errs = append(errs, tx.WriteWorkspaces().Append(m.dst.ID, numbering.Drafts(numbering.Items, slices.Collect(maps.Values(m.numbers)))...))
		for _, it := range m.items {
			errs = append(errs, tx.WriteTransfers().Append(transfer.Record{
				Entity: transfer.EntityItem, Target: it.ID, Old: m.oldID(it), New: m.newID(it), ToStore: string(store),
			}))
		}
		return errors.Join(errs...)
	})
}
