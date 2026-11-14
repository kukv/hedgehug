package search

import (
	"context"
	"errors"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
)

// customKeys は、このマシンのすべての Store(非表示を含む)の、削除されていないワークスペースの追加プロパティの
// キー(小文字)。組み込みのプロパティは含めない(E1-3)。
func (s Service) customKeys(ctx context.Context) (map[string]bool, error) {
	targets, err := s.SyncTargets.List(ctx)
	if err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	for _, t := range targets {
		if err := s.Stores.Read(ctx, t.ID, func(r port.Reader) error { return addCustomKeys(r, keys) }); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

// addCustomKeys は、Store r の削除されていないワークスペースの追加プロパティのキーを keys に足す。
func addCustomKeys(r port.Reader, keys map[string]bool) error {
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
		for _, p := range st.Properties {
			if !p.Builtin {
				keys[strings.ToLower(p.Key)] = true
			}
		}
	}
	return nil
}
