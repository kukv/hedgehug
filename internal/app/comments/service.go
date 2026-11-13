// Package comments は、アイテムとプロジェクトのコメントの操作(02「コメント」)を持つ。1 ファイル 1 操作。
package comments

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
)

const (
	emptyBody   = "コメントを入力してください"
	unknownUser = "不明なユーザー" // 01「画面の移り方と、共通の表示」
)

// Service は、コメントの操作が使う口。
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
}

// Target は、コメントの持ち主。Kind は change.KindItem か change.KindProject。
type Target struct {
	Kind change.Kind
	ID   change.ID
}

func (t Target) ref() change.Ref { return change.Ref{Kind: t.Kind, ID: t.ID} }

// View は、画面に出すコメント。ByName は投稿者の表示名で、メンバーにいなければ「不明なユーザー」。
type View struct {
	comment.Comment
	ByName string
}

// display は、持ち主の画面の ID(HH-12・HH-P1)。持ち主がなければ port.ErrNotFound。
func display(r port.Reader, t Target) (string, error) {
	switch t.Kind {
	case change.KindItem:
		it, err := r.Items().Get(t.ID)
		if err != nil {
			return "", err
		}
		w, err := r.Workspaces().Get(it.Place.Workspace)
		if err != nil {
			return "", err
		}
		return it.DisplayID(w.Prefix), nil
	case change.KindProject:
		p, err := r.Projects().Get(t.ID)
		if err != nil {
			return "", err
		}
		w, err := r.Workspaces().Get(p.Place.Workspace)
		if err != nil {
			return "", err
		}
		return p.DisplayID(w.Prefix), nil
	}
	return "", port.ErrNotFound
}

// appendTo は、持ち主のログに行を足す。
func appendTo(tx port.Tx, t Target, drafts ...change.Draft) error {
	switch t.Kind {
	case change.KindItem:
		return tx.WriteItems().Append(t.ID, drafts...)
	case change.KindProject:
		return tx.WriteProjects().Append(t.ID, drafts...)
	}
	return port.ErrNotFound
}

// displayOf は、アクティビティの 1 行に書く持ち主の ID。保存の前に読む。
func (s Service) displayOf(ctx context.Context, store port.StoreID, t Target) (string, error) {
	var shown string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		shown, err = display(r, t)
		return err
	})
	return shown, err
}

// currentOf は、持ち主のログのいまのコメント。ないか削除されていれば port.ErrNotFound。
func currentOf(r port.Reader, t Target, id change.ID) (comment.Comment, error) {
	log, err := r.Changes().Of(t.ref())
	if err != nil {
		return comment.Comment{}, err
	}
	c, ok := comment.Find(log, id)
	if !ok {
		return comment.Comment{}, port.ErrNotFound
	}
	return c, nil
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

// find は、書いたあとのコメントを読み直して返す。
func (s Service) find(ctx context.Context, store port.StoreID, t Target, id change.ID) (View, error) {
	list, err := s.List(ctx, store, t)
	if err != nil {
		return View{}, err
	}
	for _, v := range list {
		if v.ID == id {
			return v, nil
		}
	}
	return View{}, port.ErrNotFound
}
