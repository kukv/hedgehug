package githublink_test

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
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

// memScans は、port.PRScans のメモリ上の偽物。
type memScans struct {
	rows map[port.StoreID]map[string]port.ScanRecord
}

func newMemScans() *memScans { return &memScans{rows: map[port.StoreID]map[string]port.ScanRecord{}} }

func (m *memScans) Scans(_ context.Context, store port.StoreID) (map[string]port.ScanRecord, error) {
	out := map[string]port.ScanRecord{}
	for k, v := range m.rows[store] {
		out[k] = v
	}
	return out, nil
}

func (m *memScans) SaveScan(_ context.Context, store port.StoreID, repo string, r port.ScanRecord) (bool, error) {
	if m.rows[store] == nil {
		m.rows[store] = map[string]port.ScanRecord{}
	}
	key := strings.ToLower(repo)
	old, ok := m.rows[store][key]
	m.rows[store][key] = r
	return !ok || old != r, nil
}

// memMissing は、port.AppMissing のメモリ上の偽物。
type memMissing struct {
	rows map[account.ID][]port.MissingOrg
}

func newMemMissing() *memMissing { return &memMissing{rows: map[account.ID][]port.MissingOrg{}} }

func (m *memMissing) List(context.Context) (map[account.ID][]port.MissingOrg, error) {
	out := map[account.ID][]port.MissingOrg{}
	for k, v := range m.rows {
		out[k] = append([]port.MissingOrg(nil), v...)
	}
	return out, nil
}

func (m *memMissing) Put(_ context.Context, a account.ID, o port.MissingOrg) (bool, error) {
	for i, x := range m.rows[a] {
		if strings.EqualFold(x.Org, o.Org) {
			m.rows[a][i].Repo = o.Repo
			return false, nil
		}
	}
	m.rows[a] = append(m.rows[a], o)
	return true, nil
}

func (m *memMissing) Clear(_ context.Context, a account.ID, org string) (bool, error) {
	for i, x := range m.rows[a] {
		if strings.EqualFold(x.Org, org) {
			m.rows[a] = append(m.rows[a][:i:i], m.rows[a][i+1:]...)
			return true, nil
		}
	}
	return false, nil
}
