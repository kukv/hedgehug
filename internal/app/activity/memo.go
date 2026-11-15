package activity

import (
	"slices"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Memo は、Store ごとに、版の印(port.StoreVersions)と、その版で作ったコミットの行(重なりをまとめたもの)を覚える。
// 版が変わらなければ、行を作り直さず(Store を読まず、コミットも辿らず)そのまま使う。Service は複数の要求から
// 同時に呼ばれ、値で渡されるので、Service はこれをポインタで持つ。ゼロ値で使える。
type Memo struct {
	mu      sync.Mutex
	byStore map[port.StoreID]memoEntry
}

type memoEntry struct {
	version string
	rows    []Row
}

// get は、Store id の版 version の行の写し。覚えていないか版が違えば false。
func (m *Memo) get(id port.StoreID, version string) ([]Row, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.byStore[id]
	if !ok || e.version != version {
		return nil, false
	}
	return slices.Clone(e.rows), true
}

// put は、Store id の版 version の行を覚える。rows はこのあと書き換えない。
func (m *Memo) put(id port.StoreID, version string, rows []Row) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byStore == nil {
		m.byStore = map[port.StoreID]memoEntry{}
	}
	m.byStore[id] = memoEntry{version: version, rows: rows}
}

// keep は、infos(表示中の一覧)にない Store の覚えた行を捨てる。
func (m *Memo) keep(infos []port.StoreInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.byStore {
		if !slices.ContainsFunc(infos, func(i port.StoreInfo) bool { return i.ID == id }) {
			delete(m.byStore, id)
		}
	}
}
