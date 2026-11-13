package items

import (
	"context"
	"errors"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/relation"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// editContext は、値の操作が保存の前に読むもの。withID にワークスペースの設定とメンバーの表示名を足したもの。
// アクティビティの 1 行を保存の前に決めるために使う。
type editContext struct {
	item     item.Item
	display  string
	settings workspace.Settings
	names    item.Names
}

func (s Service) editContext(ctx context.Context, store port.StoreID, id item.ID) (editContext, error) {
	var ec editContext
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := r.Items().Get(id)
		if err != nil {
			return err
		}
		w, err := r.Workspaces().Get(it.Place.Workspace)
		if err != nil {
			return err
		}
		st, err := r.Workspaces().Settings(it.Place.Workspace)
		if err != nil {
			return err
		}
		ms, err := r.Members().List()
		if err != nil {
			return err
		}
		names := map[string]string{}
		for _, m := range ms {
			names[strings.ToLower(m.Login)] = m.Name
		}
		ec = editContext{
			item: it, display: it.DisplayID(w.Prefix), settings: st,
			names: func(login string) string {
				if n, ok := names[strings.ToLower(login)]; ok {
					return n
				}
				return "不明なユーザー" // 01「画面の移り方と、共通の表示」
			},
		}
		return nil
	})
	return ec, err
}

// property は、id のプロパティ。なければ port.ErrNotFound。
func (ec editContext) property(id change.ID) (workspace.Property, error) {
	p, ok := ec.settings.Property(id)
	if !ok {
		return workspace.Property{}, port.ErrNotFound
	}
	return p, nil
}

// blockedSet は、Store のブロック中のアイテムの内部 ID(A4a-13)。関連はワークスペースをまたぐので、
// ワークスペース → プロジェクト → アイテムの順にすべて読む。ステータスの分類は、そのアイテムのワークスペースの設定で決める。
// 設定が要るのはブロックしているアイテムの完了の判定だけなので、そのワークスペースの設定を、初めて要るときに 1 回だけ読む。
func blockedSet(r port.Reader) (map[item.ID]bool, error) {
	wss, err := r.Workspaces().List()
	if err != nil {
		return nil, err
	}
	var all []item.Item
	for _, w := range wss {
		ps, err := r.Projects().List(w.ID)
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
	settings := map[workspace.ID]workspace.Settings{}
	var firstErr error
	set := relation.BlockedSet(all, func(it item.Item) bool {
		w := it.Place.Workspace
		st, ok := settings[w]
		if !ok {
			var err error
			st, err = r.Workspaces().Settings(w)
			if err != nil && !errors.Is(err, port.ErrNotFound) && firstErr == nil {
				firstErr = err
			}
			settings[w] = st
		}
		return st.Finished(it.Status)
	})
	return set, firstErr
}
