package transfers

import (
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

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
