package transfers

import (
	"errors"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// crossSource は、別の Store へ移すときに (0) で移動元から読むもの。set の Outside は、移動元の Store 全体
// (削除済みを含む)との関連。logs・descs は移すアイテムごとの行と説明。vocab は moved_in の names を引く設定
// (移動元のワークスペースが先頭、続いて行に現れる同じ Store のほかのワークスペース)。moves は移すアイテムごとの、
// 移動元の Store の中で前に別のワークスペースへ移した記録。
type crossSource struct {
	source
	project project.Project
	logs    map[item.ID][]change.Record
	descs   map[item.ID]string
	members []member.Member
	vocab   []workspace.Settings
	moves   map[item.ID][]transfer.Move
}

// logList は、移すアイテムの行を、items の順で返す。
func (c crossSource) logList() [][]change.Record {
	out := make([][]change.Record, 0, len(c.logs))
	for _, it := range c.items() {
		out = append(out, c.logs[it.ID])
	}
	return out
}

// gather は、移動元から移すもの・行・説明・メンバーを読む。検査は readMovable と同じ(別の画面で削除されたものも読む)。
func gather(r port.Reader, id item.ID) (crossSource, error) {
	src, err := readMovable(r, id)
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
	c := crossSource{source: src, logs: map[item.ID][]change.Record{}, descs: map[item.ID]string{}, moves: map[item.ID][]transfer.Move{}}
	if c.project, err = r.Projects().Get(src.set.Root.Place.Project); err != nil {
		return crossSource{}, err
	}
	if c.members, err = r.Members().List(); err != nil {
		return crossSource{}, err
	}
	recs, err := r.Transfers().List()
	if err != nil {
		return crossSource{}, err
	}
	for _, it := range src.items() {
		if c.logs[it.ID], err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: it.ID}); err != nil {
			return crossSource{}, err
		}
		if c.descs[it.ID], err = r.Items().Description(it.ID); err != nil {
			return crossSource{}, err
		}
		if c.moves[it.ID], err = earlierMoves(r, it.ID, c.logs[it.ID], recs); err != nil {
			return crossSource{}, err
		}
	}
	c.vocab, err = vocabOf(r, src.settings, c.logList())
	return c, err
}

// earlierMoves は、アイテム id の行 log の、ワークスペースが変わる場所の set のうち、同じ操作の id の転送記録が recs に
// あるもの(この Store の中で別のワークスペースへ移したとき)の、旧 ID と移動元の名前。名前が引けなければ空にする。
func earlierMoves(r port.Reader, id item.ID, log []change.Record, recs []transfer.Record) ([]transfer.Move, error) {
	var out []transfer.Move
	for _, row := range log {
		from, to, ok := placesOf(row)
		i := slices.IndexFunc(recs, func(t transfer.Record) bool {
			return t.Op == row.Op && t.Entity == transfer.EntityItem && t.Target == id
		})
		if !ok || from.Workspace == to.Workspace || i < 0 {
			continue
		}
		mv := transfer.Move{Op: row.Op, Old: recs[i].Old}
		ws, err := r.Workspaces().Get(from.Workspace)
		if err != nil && !errors.Is(err, port.ErrNotFound) {
			return nil, err
		}
		pr, err := r.Projects().Get(from.Project)
		if err != nil && !errors.Is(err, port.ErrNotFound) {
			return nil, err
		}
		mv.FromWorkspaceName, mv.FromProjectTitle = ws.Name, pr.Title
		out = append(out, mv)
	}
	return out, nil
}

// vocabOf は、moved_in の names を引く設定: 移動元のワークスペースの設定 from と、行 logs の作成・場所の set に現れる、
// この Store のほかのワークスペースの設定(現れた順)。この Store にないワークスペース(前にいた別の Store)は飛ばす。
func vocabOf(r port.Reader, from workspace.Settings, logs [][]change.Record) ([]workspace.Settings, error) {
	out := []workspace.Settings{from}
	seen := map[workspace.ID]bool{}
	for _, log := range logs {
		for _, ws := range workspacesIn(log) {
			if seen[ws] {
				continue
			}
			seen[ws] = true
			st, err := r.Workspaces().Settings(ws)
			if errors.Is(err, port.ErrNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
			out = append(out, st)
		}
	}
	return out, nil
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
