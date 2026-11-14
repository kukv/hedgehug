package search

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// catalog は、追加プロパティのキー(小文字。このマシンのすべての Store、非表示を含む、削除されていないワークスペース。
// 組み込みのプロパティは含めない。E1-3)と、テーブルの列(表示中の Store のワークスペースだけ。E2a-2)を、
// Store を 1 回ずつ読んで求める。
func (s Service) catalog(ctx context.Context) (map[string]bool, []Column, error) {
	targets, err := s.SyncTargets.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	infos, err := s.StoreList.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	order := map[port.StoreID]int{}
	for i, info := range infos {
		order[info.ID] = i
	}
	keys := map[string]bool{}
	shown := make([][]*space, len(infos))
	for _, t := range targets {
		i, ok := order[t.ID]
		if err := s.Stores.Read(ctx, t.ID, func(r port.Reader) error {
			if !ok {
				return addHiddenKeys(r, keys)
			}
			spaces, err := readLiveSpaces(r)
			shown[i] = spaces
			addCustomKeys(spaces, keys)
			return err
		}); err != nil {
			return nil, nil, err
		}
	}
	return keys, columnsOf(slices.Concat(shown...)), nil
}

// addCustomKeys は、spaces の追加プロパティのキー(小文字)を keys に足す。
func addCustomKeys(spaces []*space, keys map[string]bool) {
	for _, sp := range spaces {
		addKeys(sp.settings, keys)
	}
}

// addHiddenKeys は、非表示の Store r の削除されていないワークスペースの追加プロパティのキーを keys に足す。
// 列に要らないので、ワークスペースのログは読まず設定だけ読む。
func addHiddenKeys(r port.Reader, keys map[string]bool) error {
	wss, err := r.Workspaces().List()
	if err != nil {
		return err
	}
	for _, w := range wss {
		if w.Deleted != nil {
			continue
		}
		st, err := r.Workspaces().Settings(w.ID)
		if errors.Is(err, port.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		addKeys(st, keys)
	}
	return nil
}

// addKeys は、設定 st の追加プロパティのキー(小文字)を keys に足す。
func addKeys(st workspace.Settings, keys map[string]bool) {
	for _, p := range st.Properties {
		if !p.Builtin {
			keys[strings.ToLower(p.Key)] = true
		}
	}
}

// readLiveSpaces は、Store の削除されていないワークスペースのうち設定のあるものを、サイドバーの並び
// (workspace.RankOf、同じなら内部 ID)で返す。
func readLiveSpaces(r port.Reader) ([]*space, error) {
	wss, err := r.Workspaces().List()
	if err != nil {
		return nil, err
	}
	var out []*space
	for _, w := range wss {
		if w.Deleted != nil {
			continue
		}
		sp, err := readSpace(r, w)
		if err != nil {
			return nil, err
		}
		if sp.hasSettings {
			out = append(out, sp)
		}
	}
	slices.SortFunc(out, func(a, b *space) int { return cmp.Or(cmp.Compare(a.rank, b.rank), cmp.Compare(a.ws.ID, b.ws.ID)) })
	return out, nil
}
