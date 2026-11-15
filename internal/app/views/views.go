// Package views は、保存したビュー(E3b)。名前と内容(検索の画面の状態と同じ形)を持つ。
package views

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/savedview"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 値の型は、infra と共有するため domain/savedview に置く。
type (
	// Sort は、テーブルの並べ替え。
	Sort = savedview.Sort
	// Table は、テーブルの設定。
	Table = savedview.Table
	// Content は、ビューの内容(E3b-1)。
	Content = savedview.Content
	// View は、保存したビュー。ID は ULID を小文字にした 26 文字(E3b-2)。
	View = savedview.View
)

// Store は、ビューの置き場(state.db)。Delete は画面の状態 view:{id} の行も消す。
type Store interface {
	List(ctx context.Context) ([]View, error)
	Get(ctx context.Context, id string) (View, bool, error)
	Put(ctx context.Context, v View) error
	Delete(ctx context.Context, id string) error
}

// Service は、ビューの作成・保存・一覧と、名前の変更・複製・削除。書く操作は 1 つずつ行う(E3b-6)。
type Service struct {
	Store Store
	IDs   port.IDs
	Keys  func(ctx context.Context) (map[string]bool, error)
	// HeldElsewhere は、(store, ws) 以外のワークスペースに追加プロパティのキーがあるか(E3b-18)。
	HeldElsewhere func(ctx context.Context, store port.StoreID, ws workspace.ID, key string) (bool, error)
	Events        port.Events

	mu sync.Mutex
}

// NameKey は、名前の比べの鍵(E3b-5)。
func NameKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// List は、比べの鍵の順(同じなら名前、さらに識別子の順)でビューを返す(E3b-7)。
func (s *Service) List(ctx context.Context) ([]View, error) {
	list, err := s.Store.List(ctx)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(list, func(a, b View) int {
		return cmp.Or(cmp.Compare(NameKey(a.Name), NameKey(b.Name)), cmp.Compare(a.Name, b.Name), cmp.Compare(a.ID, b.ID))
	})
	return list, nil
}

// Create は、名前と内容でビューを作る。
func (s *Service) Create(ctx context.Context, name string, c Content) (View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	if err := s.checkName(ctx, "", name); err != nil {
		return View{}, err
	}
	c, err := s.prune(ctx, c)
	if err != nil {
		return View{}, err
	}
	return s.put(ctx, View{ID: s.newID(), Name: name, Content: c})
}

// Save は、ビューの内容を置き換える。存在しないキーの列は外す(E3b-8)。
func (s *Service) Save(ctx context.Context, id string, c Content) (View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(ctx, id)
	if err != nil {
		return View{}, err
	}
	if v.Content, err = s.prune(ctx, c); err != nil {
		return View{}, err
	}
	return s.put(ctx, v)
}

func (s *Service) newID() string { return strings.ToLower(string(s.IDs.New())) }

// prune は、c の列のうち、どのワークスペースにもないキーの p:{キー} の列を外す(E3b-8)。
func (s *Service) prune(ctx context.Context, c Content) (Content, error) {
	keys, err := s.Keys(ctx)
	if err != nil {
		return Content{}, err
	}
	cols := make([]string, 0, len(c.Table.Columns))
	for _, col := range c.Table.Columns {
		if k, ok := strings.CutPrefix(col, "p:"); ok && !keys[strings.ToLower(k)] {
			continue
		}
		cols = append(cols, col)
	}
	c.Table.Columns = cols
	return c, nil
}

func (s *Service) get(ctx context.Context, id string) (View, error) {
	v, ok, err := s.Store.Get(ctx, id)
	if err != nil {
		return View{}, err
	}
	if !ok {
		return View{}, port.ErrNotFound
	}
	return v, nil
}

// put は、v を置いて知らせる。
func (s *Service) put(ctx context.Context, v View) (View, error) {
	if err := s.Store.Put(ctx, v); err != nil {
		return View{}, err
	}
	s.Events.Publish(port.Event{Kind: port.EventViews})
	return v, nil
}

// checkName は、name(前後の空白を除いたもの)が空か、self 以外のビューと重なれば断る。
func (s *Service) checkName(ctx context.Context, self, name string) error {
	if name == "" {
		return refusal.New("名前を入力してください")
	}
	list, err := s.Store.List(ctx)
	if err != nil {
		return err
	}
	for _, v := range list {
		if v.ID != self && NameKey(v.Name) == NameKey(name) {
			return refusal.New("同じ名前のビューがあります")
		}
	}
	return nil
}
