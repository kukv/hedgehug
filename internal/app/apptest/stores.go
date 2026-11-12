package apptest

import (
	"context"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// Stores は port.Stores のメモリ上の偽物。Read と Write は 1 つずつ順に行う。
type Stores struct {
	mu     sync.Mutex
	clock  port.Clock
	ids    port.IDs
	stores map[port.StoreID]map[change.Ref][]change.Record
}

func NewStores(clock port.Clock, ids port.IDs) *Stores {
	return &Stores{clock: clock, ids: ids, stores: map[port.StoreID]map[change.Ref][]change.Record{}}
}

// AddStore は、空の Store を足す。
func (s *Stores) AddStore(id port.StoreID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stores[id] = map[change.Ref][]change.Record{}
}

func (s *Stores) Read(_ context.Context, id port.StoreID, fn func(r port.Reader) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	logs, ok := s.stores[id]
	if !ok {
		return port.ErrStoreNotFound
	}
	return fn(&view{base: logs})
}

func (s *Stores) Write(_ context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	logs, ok := s.stores[id]
	if !ok {
		return port.ErrStoreNotFound
	}
	t := &tx{
		view: view{base: logs, pending: map[change.Ref][]change.Record{}},
		meta: change.Meta{Op: s.ids.New(), At: s.clock.Now(), Actor: op.Actor},
		ids:  s.ids,
	}
	if err := fn(t); err != nil {
		return err
	}
	for ref, recs := range t.pending {
		logs[ref] = append(logs[ref], recs...)
	}
	return nil
}

// tx と種類ごとの Writer は、本物(infra/gitstore)にもほぼ同じものがある。gitstore は apptest を
// import できず、port は口の定義だけを置くので、まとめない。両者は RunStoresContract で揃える。
type tx struct {
	view
	meta change.Meta
	ids  port.IDs
}

func (t *tx) append(ref change.Ref, drafts []change.Draft) error {
	recs := change.Stamp(t.log(ref), drafts, t.meta, t.ids.New)
	t.pending[ref] = append(t.pending[ref], recs...)
	return nil
}

func (t *tx) WriteWorkspaces() port.WorkspaceWriter { return idWriter{t, change.KindWorkspace} }
func (t *tx) WriteProjects() port.ProjectWriter     { return idWriter{t, change.KindProject} }
func (t *tx) WriteItems() port.ItemWriter           { return idWriter{t, change.KindItem} }
func (t *tx) WriteMembers() port.MemberWriter       { return membersWriter{t} }

type idWriter struct {
	t    *tx
	kind change.Kind
}

func (w idWriter) Append(id change.ID, drafts ...change.Draft) error {
	return w.t.append(change.Ref{Kind: w.kind, ID: id}, drafts)
}

type membersWriter struct{ t *tx }

func (w membersWriter) Append(drafts ...change.Draft) error {
	return w.t.append(change.Ref{Kind: change.KindMembers}, drafts)
}
