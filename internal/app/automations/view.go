package automations

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// View は、自動化のサブタブの中身。
type View struct {
	Rules  []automation.Rule
	Recent []Run
}

// Run は、最近の実行の 1 行(Task 8)。
type Run struct{}

// List は、ワークスペースのルールと最近の実行。
func (s Service) List(ctx context.Context, store port.StoreID, ws workspace.ID) (View, error) {
	var v View
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		rules, err := rulesOf(r, ws)
		v.Rules = rules
		return err
	})
	return v, err
}
