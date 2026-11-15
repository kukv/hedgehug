package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Rename は、ビューの名前を変える。自分自身との一致は重複としない(E3b-5)。
func (s *Service) Rename(ctx context.Context, id, name string) (View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(ctx, id)
	if err != nil {
		return View{}, err
	}
	name = strings.TrimSpace(name)
	if err := s.checkName(ctx, id, name); err != nil {
		return View{}, err
	}
	v.Name = name
	return s.put(ctx, v)
}

// Duplicate は、保存した内容を「{元の名前} のコピー」(重なれば空いている最小の番号を付けた名前)で複製する(E3b-13)。
func (s *Service) Duplicate(ctx context.Context, id string) (View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(ctx, id)
	if err != nil {
		return View{}, err
	}
	list, err := s.Store.List(ctx)
	if err != nil {
		return View{}, err
	}
	taken := map[string]bool{}
	for _, o := range list {
		taken[NameKey(o.Name)] = true
	}
	name := v.Name + " のコピー"
	for n := 2; taken[NameKey(name)]; n++ {
		name = fmt.Sprintf("%s のコピー %d", v.Name, n)
	}
	return s.put(ctx, View{ID: s.newID(), Name: name, Content: v.Content})
}

// Delete は、ビューを削除する。アイテムには触れない。
func (s *Service) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.get(ctx, id); err != nil {
		return err
	}
	if err := s.Store.Delete(ctx, id); err != nil {
		return err
	}
	s.Events.Publish(port.Event{Kind: port.EventViews})
	return nil
}
