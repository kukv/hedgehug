package views

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"sync"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// memStore は、map で持つ Store の偽物。
type memStore struct {
	mu sync.Mutex
	m  map[string]View
}

func (s *memStore) List(context.Context) ([]View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []View
	for _, v := range s.m {
		out = append(out, v)
	}
	return out, nil
}

func (s *memStore) Get(_ context.Context, id string) (View, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[id]
	return v, ok, nil
}

func (s *memStore) Put(_ context.Context, v View) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[v.ID] = v
	return nil
}

func (s *memStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

// recorder は、流れた出来事を記録する Events の偽物。
type recorder struct{ events []port.Event }

func (r *recorder) Publish(e port.Event) { r.events = append(r.events, e) }

func newService(keys ...string) (*Service, *recorder) {
	rec := &recorder{}
	set := map[string]bool{}
	for _, k := range keys {
		set[k] = true
	}
	return &Service{
		Store:  &memStore{m: map[string]View{}},
		IDs:    apptest.NewIDs(),
		Keys:   func(context.Context) (map[string]bool, error) { return set, nil },
		Events: rec,
	}, rec
}

func mustCreate(t *testing.T, s *Service, name string) View {
	t.Helper()
	v, err := s.Create(t.Context(), name, Content{})
	if err != nil {
		t.Fatalf("Create(%q): %v", name, err)
	}
	return v
}

func wantRefusal(t *testing.T, err error, msg string) {
	t.Helper()
	var r *refusal.Refusal
	if !errors.As(err, &r) || r.Message != msg {
		t.Fatalf("err = %v, want 断り %q", err, msg)
	}
}

func TestCreateUsesLowercaseID(t *testing.T) {
	s, _ := newService()
	v := mustCreate(t, s, "バグ")
	if !regexp.MustCompile(`^[0-9a-z]{26}$`).MatchString(v.ID) {
		t.Errorf("ID = %q", v.ID)
	}
}

func TestCreateTrimsName(t *testing.T) {
	s, _ := newService()
	if v := mustCreate(t, s, " バグ "); v.Name != "バグ" {
		t.Errorf("name = %q", v.Name)
	}
}

func TestCreateRefusesEmptyName(t *testing.T) {
	s, rec := newService()
	_, err := s.Create(t.Context(), "   ", Content{})
	wantRefusal(t, err, "名前を入力してください")
	if len(rec.events) != 0 {
		t.Errorf("events = %v", rec.events)
	}
}

func TestCreateRefusesSameName(t *testing.T) {
	s, _ := newService()
	mustCreate(t, s, "Bugs")
	_, err := s.Create(t.Context(), " bugs ", Content{})
	wantRefusal(t, err, "同じ名前のビューがあります")
}

func TestSavePrunesDeadColumns(t *testing.T) {
	s, _ := newService("customer")
	cols := []string{"id", "p:customer", "p:gone", "title"}
	want := []string{"id", "p:customer", "title"}

	created, err := s.Create(t.Context(), "a", Content{Table: Table{Columns: cols}})
	if err != nil || !reflect.DeepEqual(created.Content.Table.Columns, want) {
		t.Fatalf("Create: %+v err=%v", created, err)
	}
	saved, err := s.Save(t.Context(), created.ID, Content{Query: "x", Table: Table{Columns: cols}})
	if err != nil || !reflect.DeepEqual(saved.Content.Table.Columns, want) || saved.Content.Query != "x" {
		t.Fatalf("Save: %+v err=%v", saved, err)
	}
	if got, _, _ := s.Store.Get(t.Context(), created.ID); !reflect.DeepEqual(got, saved) {
		t.Errorf("保存した内容 = %+v", got)
	}
}

func TestListOrder(t *testing.T) {
	s, _ := newService()
	mustCreate(t, s, "b")
	mustCreate(t, s, "A")
	mustCreate(t, s, "a2")
	list, err := s.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, v := range list {
		names = append(names, v.Name)
	}
	if !reflect.DeepEqual(names, []string{"A", "a2", "b"}) {
		t.Errorf("names = %v", names)
	}
}
