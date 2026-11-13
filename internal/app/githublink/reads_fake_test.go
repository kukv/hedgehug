package githublink_test

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghread"
)

// memReads は、port.GitHubReads のメモリ上の偽物。このパッケージのテストは取り直しの失敗の文言だけを使う。
type memReads struct {
	failures map[port.StoreID]map[string]string
}

func newMemReads() *memReads { return &memReads{failures: map[port.StoreID]map[string]string{}} }

func (m *memReads) Records(context.Context, port.StoreID) (map[string]ghread.Record, error) {
	return map[string]ghread.Record{}, nil
}

func (m *memReads) SaveRecord(context.Context, port.StoreID, string, ghread.Record) error {
	return nil
}

func (m *memReads) Failures(_ context.Context, store port.StoreID) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range m.failures[store] {
		out[k] = v
	}
	return out, nil
}

func (m *memReads) SetFailure(_ context.Context, store port.StoreID, key, text string) (bool, error) {
	rows := m.failures[store]
	if rows == nil {
		rows = map[string]string{}
		m.failures[store] = rows
	}
	if rows[key] == text {
		return false, nil
	}
	if text == "" {
		delete(rows, key)
	} else {
		rows[key] = text
	}
	return true, nil
}
