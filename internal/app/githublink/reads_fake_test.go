package githublink_test

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghread"
)

// memReads は、port.GitHubReads のメモリ上の偽物。
type memReads struct {
	records  map[port.StoreID]map[string]ghread.Record
	failures map[port.StoreID]map[string]string
}

func newMemReads() *memReads {
	return &memReads{records: map[port.StoreID]map[string]ghread.Record{}, failures: map[port.StoreID]map[string]string{}}
}

func (m *memReads) Records(_ context.Context, store port.StoreID) (map[string]ghread.Record, error) {
	out := map[string]ghread.Record{}
	for k, v := range m.records[store] {
		out[k] = v
	}
	return out, nil
}

func (m *memReads) SaveRecord(_ context.Context, store port.StoreID, repo string, r ghread.Record) error {
	if m.records[store] == nil {
		m.records[store] = map[string]ghread.Record{}
	}
	m.records[store][strings.ToLower(repo)] = r
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
