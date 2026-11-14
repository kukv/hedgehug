package inbox

import (
	"encoding/json"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// itemDrafts は、アイテム it の項目 f(ログの項目名 field)を、いまの値 now から mine に戻す行(C4c2-8)。
// 同じ値なら行はない。
func itemDrafts(it item.Item, f itemField, field string, now, mine json.RawMessage) func(tx port.Tx) ([]change.Draft, error) {
	return func(tx port.Tx) ([]change.Draft, error) {
		if sameValue(now, mine) {
			return nil, nil
		}
		if f.kind == fieldProperty && (f.prop.Type == workspace.TypeSystems || f.prop.Type == workspace.TypeMulti) {
			return multiDrafts(field, now, mine), nil
		}
		ds := []change.Draft{change.NewSet(field, now, orNull(mine))}
		if f.kind != fieldSprint {
			return ds, nil
		}
		end, err := frameEnd(tx, it, change.ID(textOf(mine)))
		if err != nil || end == "" {
			return ds, err
		}
		return append(ds, change.NewSet("rank", it.Rank, rank.After(end))), nil
	}
}

// multiDrafts は、複数の値のいまの値 now と mine の差の行。remove が先で、それぞれ値の JSON の辞書順。
func multiDrafts(field string, now, mine json.RawMessage) []change.Draft {
	have, _ := idsOf(now)
	want, _ := idsOf(mine)
	var removes, adds []change.ID
	for _, id := range have {
		if !slices.Contains(want, id) {
			removes = append(removes, id)
		}
	}
	for _, id := range want {
		if !slices.Contains(have, id) {
			adds = append(adds, id)
		}
	}
	slices.Sort(removes)
	slices.Sort(adds)
	var ds []change.Draft
	for _, id := range slices.Compact(removes) {
		ds = append(ds, change.NewRemove(field, id))
	}
	for _, id := range slices.Compact(adds) {
		ds = append(ds, change.NewAdd(field, id))
	}
	return ds
}

// orNull は、アイテムに書く値。なし・空文字は null(チェックの false はそのまま)。
func orNull(v json.RawMessage) json.RawMessage {
	if len(v) == 0 || string(v) == `""` {
		return json.RawMessage("null")
	}
	return v
}

// frameEnd は、アイテム it のワークスペースの、移動先の枠(sprint が空ならバックログ)の末尾の rank(SetSprint と同じ)。
func frameEnd(r port.Reader, it item.Item, sprint change.ID) (string, error) {
	ws := it.Place.Workspace
	st, err := r.Workspaces().Settings(ws)
	if err != nil {
		return "", err
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	if err != nil {
		return "", err
	}
	ps, err := r.Projects().List(ws)
	if err != nil {
		return "", err
	}
	var its []item.Item
	for _, p := range ps {
		list, err := r.Items().List(p.ID)
		if err != nil {
			return "", err
		}
		its = append(its, list...)
	}
	return planning.FrameEnd(its, ws, sprint, planning.FromLog(log), st), nil
}
