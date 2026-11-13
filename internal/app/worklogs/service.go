// Package worklogs は、アイテムの作業時間の操作(02「作業時間」)を持つ。1 ファイル 1 操作。
package worklogs

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

const unknownUser = "不明なユーザー" // 01「画面の移り方と、共通の表示」

// Service は、作業時間の操作が使う口。Clock は、記録の日付(保存したマシンの今日)を決める。
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
	Clock    port.Clock
}

// View は、画面に出す作業時間の記録。ByName は記録した人の表示名で、メンバーにいなければ「不明なユーザー」。
type View struct {
	worklog.Entry
	ByName string
}

// display は、アイテムの画面の ID(HH-12)。アイテムがなければ port.ErrNotFound。
func display(r port.Reader, id item.ID) (string, error) {
	it, err := port.LiveItem(r, id)
	if err != nil {
		return "", err
	}
	w, err := r.Workspaces().Get(it.Place.Workspace)
	if err != nil {
		return "", err
	}
	return it.DisplayID(w.Prefix), nil
}

// displayOf は、アクティビティの 1 行に書くアイテムの ID。保存の前に読む(書き込みだけが使う)。
// 別の画面で削除された(root: true)アイテムも読める。
func (s Service) displayOf(ctx context.Context, store port.StoreID, id item.ID) (string, error) {
	var shown string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := port.EditableItem(r, id)
		if err != nil {
			return err
		}
		w, err := r.Workspaces().Get(it.Place.Workspace)
		if err != nil {
			return err
		}
		shown = it.DisplayID(w.Prefix)
		return nil
	})
	return shown, err
}

// revive は、保存の中で最初に呼ぶ。削除済み(root: true)のアイテムなら削除を取り消す。
// 保存できない(ない・root: false・祖先が削除済み)なら port.ErrNotFound。
func revive(ctx context.Context, tx port.Tx, id item.ID) error {
	if _, err := port.EditableItem(tx, id); err != nil {
		return err
	}
	return port.ReviveItem(ctx, tx, id)
}

// names は、ログイン名から表示名を引く関数(大文字と小文字を区別しない。メンバーにいなければ「不明なユーザー」)。
func names(r port.Reader) (func(login string) string, error) {
	ms, err := r.Members().List()
	if err != nil {
		return nil, err
	}
	byLogin := map[string]string{}
	for _, m := range ms {
		byLogin[strings.ToLower(m.Login)] = m.Name
	}
	return func(login string) string {
		if n, ok := byLogin[strings.ToLower(login)]; ok {
			return n
		}
		return unknownUser
	}, nil
}

// find は、書いたあとの記録を読み直して返す。
func (s Service) find(ctx context.Context, store port.StoreID, id item.ID, w change.ID) (View, error) {
	list, err := s.List(ctx, store, id)
	if err != nil {
		return View{}, err
	}
	for _, v := range list {
		if v.ID == w {
			return v, nil
		}
	}
	return View{}, port.ErrNotFound
}
