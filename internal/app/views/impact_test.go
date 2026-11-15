package views

import (
	"context"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func impactOf(t *testing.T, s *Service, held bool, query string) bool {
	t.Helper()
	if _, err := s.Create(t.Context(), "v", Content{Query: query}); err != nil {
		t.Fatal(err)
	}
	s.HeldElsewhere = func(context.Context, port.StoreID, workspace.ID, string) (bool, error) { return held, nil }
	got, err := s.KeyImpact(t.Context(), "S1", "W1", "customer")
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestKeyImpactWhenLastHolder(t *testing.T) {
	s, _ := newService()
	if !impactOf(t, s, false, "customer:A") {
		t.Error("最後の 1 つで、ビューが使っているので true のはず")
	}
}

func TestKeyImpactHeldElsewhere(t *testing.T) {
	s, _ := newService()
	if impactOf(t, s, true, "customer:A") {
		t.Error("ほかのワークスペースにあるので false のはず")
	}
}

func TestKeyImpactUnused(t *testing.T) {
	s, _ := newService()
	if impactOf(t, s, false, "status:Todo") {
		t.Error("どのビューも使っていないので false のはず")
	}
}
