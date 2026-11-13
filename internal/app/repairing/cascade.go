package repairing

import (
	"context"
	"errors"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// cascade は、削除済みに入れるプロジェクトかアイテム 1 つと、削除に使う op(親の削除の op)と、画面の ID。
type cascade struct {
	ref     change.Ref
	op      change.ID
	display string
}

// cascadeDeleted は、削除済みの親の下に残ったプロジェクトとアイテムを、親の削除の op で削除済みにする
// (決定 C3a-10・C3a-11・B5b-6)。1 件ごとに 1 回保存し、プロジェクトを先に書く。子の子(削除済みにした
// プロジェクトの下のアイテムを含む)は、読み直した次の回で見つかるので、なくなるまで繰り返す。
func cascadeDeleted(ctx context.Context, w port.RepairWriter) error {
	var prev []cascade
	for {
		cs, err := readCascades(w)
		if err != nil || len(cs) == 0 {
			return err
		}
		if slices.Equal(cs, prev) {
			return errors.New("削除済みの親の下の子の修復が進みません")
		}
		prev = cs
		for _, c := range cs {
			if err := ctx.Err(); err != nil {
				return err
			}
			op := port.Operation{Actor: syncActor, Summary: c.display + " を削除"}
			err := w.Write(op, func(tx port.Tx) error {
				return appendTo(tx, c.ref, change.NewDeleteEntity(false).InOp(c.op))
			})
			if err != nil {
				return err
			}
		}
	}
}

// readCascades は、いまの Store から、削除に入れるプロジェクト(先)とアイテム(後)を読む。
func readCascades(w port.RepairWriter) ([]cascade, error) {
	var out []cascade
	err := w.Read(func(r port.Reader) error {
		wss, err := r.Workspaces().List()
		if err != nil {
			return err
		}
		var projects []project.Project
		var items []item.Item
		prefixes := map[workspace.ID]string{}
		for _, ws := range wss {
			prefixes[ws.ID] = ws.Prefix
			ps, err := r.Projects().List(ws.ID)
			if err != nil {
				return err
			}
			projects = append(projects, ps...)
			its, err := itemsOf(r, ps)
			if err != nil {
				return err
			}
			items = append(items, its...)
		}
		out = cascadesOf(wss, projects, items, prefixes)
		return nil
	})
	return out, err
}

// cascadesOf は、repair の ProjectCascades と Cascades を、画面の ID を付けて 1 つの並びにする。
func cascadesOf(wss []workspace.Workspace, projects []project.Project, items []item.Item, prefixes map[workspace.ID]string) []cascade {
	var out []cascade
	byProject := map[project.ID]project.Project{}
	for _, p := range projects {
		byProject[p.ID] = p
	}
	for _, c := range repair.ProjectCascades(wss, projects) {
		p := byProject[c.Project]
		out = append(out, cascade{ref: change.Ref{Kind: change.KindProject, ID: p.ID}, op: c.Op, display: p.DisplayID(prefixes[p.Place.Workspace])})
	}
	byItem := map[item.ID]item.Item{}
	for _, it := range items {
		byItem[it.ID] = it
	}
	for _, c := range repair.Cascades(items, projects) {
		it := byItem[c.Child]
		out = append(out, cascade{ref: change.Ref{Kind: change.KindItem, ID: it.ID}, op: c.Op, display: it.DisplayID(prefixes[it.Place.Workspace])})
	}
	return out
}

// itemsOf は、プロジェクト ps のアイテムを読む。
func itemsOf(r port.Reader, ps []project.Project) ([]item.Item, error) {
	var items []item.Item
	for _, p := range ps {
		its, err := r.Items().List(p.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, its...)
	}
	return items, nil
}
