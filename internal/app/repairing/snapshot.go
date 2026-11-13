package repairing

import (
	"errors"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// entity は、ログを持つエンティティ 1 つ(ワークスペース・プロジェクト・アイテム)。
type entity struct {
	ref     change.Ref
	display string // 画面の ID(HH-12・HH-P1)。ワークスペースは空
	log     []change.Record
}

// snapshot は、修復が読む Store の中身。削除済みのプロジェクト・アイテムを含む。
type snapshot struct {
	// entities は、ワークスペース、プロジェクト、アイテムの順で、それぞれ内部 ID の順。
	entities []entity
}

// readSnapshot は、すべてのワークスペース・プロジェクト・アイテムと、そのログを読む。
func readSnapshot(w port.RepairWriter) (snapshot, error) {
	var s snapshot
	err := w.Read(func(r port.Reader) error {
		var err error
		if s, err = readEntities(r); err != nil {
			return err
		}
		for i := range s.entities {
			log, err := r.Changes().Of(s.entities[i].ref)
			if err != nil && !errors.Is(err, port.ErrNotFound) {
				return err
			}
			s.entities[i].log = log
		}
		return nil
	})
	return s, err
}

// readEntities は、ログを持つエンティティを並べる。ログはまだ読まない。
func readEntities(r port.Reader) (snapshot, error) {
	var s snapshot
	wss, err := r.Workspaces().List()
	if err != nil {
		return s, err
	}
	var projects, items []entity
	for _, ws := range wss {
		s.entities = append(s.entities, entity{ref: change.Ref{Kind: change.KindWorkspace, ID: ws.ID}})
		ps, err := r.Projects().List(ws.ID)
		if err != nil {
			return s, err
		}
		for _, p := range ps {
			projects = append(projects, entity{ref: change.Ref{Kind: change.KindProject, ID: p.ID}, display: p.DisplayID(ws.Prefix)})
			its, err := r.Items().List(p.ID)
			if err != nil {
				return s, err
			}
			for _, it := range its {
				items = append(items, entity{ref: change.Ref{Kind: change.KindItem, ID: it.ID}, display: it.DisplayID(ws.Prefix)})
			}
		}
	}
	byID := func(a, b entity) int { return strings.Compare(string(a.ref.ID), string(b.ref.ID)) }
	slices.SortFunc(s.entities, byID)
	slices.SortFunc(projects, byID)
	slices.SortFunc(items, byID)
	s.entities = slices.Concat(s.entities, projects, items)
	return s, nil
}

// appendTo は、ref のログに行を足す。
func appendTo(tx port.Tx, ref change.Ref, drafts ...change.Draft) error {
	switch ref.Kind {
	case change.KindWorkspace:
		return tx.WriteWorkspaces().Append(ref.ID, drafts...)
	case change.KindProject:
		return tx.WriteProjects().Append(ref.ID, drafts...)
	default:
		return tx.WriteItems().Append(ref.ID, drafts...)
	}
}
