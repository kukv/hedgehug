package apptest

import (
	"context"
	"fmt"
	"maps"
	"strconv"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// Stores は port.Stores のメモリ上の偽物。同じ Store の Read と Write は 1 つずつ順に行い、
// 別の Store どうしは互いを待たない。
type Stores struct {
	mu     sync.Mutex // stores の出し入れだけを守る。Store の中身は store.mu が守る
	clock  port.Clock
	ids    port.IDs
	stores map[port.StoreID]*store
}

// store は 1 つの Store。mu を持っているあいだだけ logs を読み書きする。
type store struct {
	mu   sync.Mutex
	logs map[change.Ref][]change.Record
	// texts は、説明の本文(ない説明は入れない)。
	texts map[change.Ref]string
	// writes は、保存が成功した回数(Versions の印)。
	writes int
}

func NewStores(clock port.Clock, ids port.IDs) *Stores {
	return &Stores{clock: clock, ids: ids, stores: map[port.StoreID]*store{}}
}

// AddStore は、空の Store を足す。同じ ID の Store がもうあれば、テストの書き間違いなので panic する
// (黙って空の Store に替えると、書いたデータが消える)。
func (s *Stores) AddStore(id port.StoreID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.stores[id]; ok {
		panic(fmt.Sprintf("apptest: AddStore: store %q already exists", id))
	}
	s.stores[id] = &store{logs: map[change.Ref][]change.Record{}, texts: map[change.Ref]string{}}
}

// open は、Store のロックを取って返す。呼んだ側が st.mu.Unlock する。
func (s *Stores) open(id port.StoreID) (*store, error) {
	s.mu.Lock()
	st, ok := s.stores[id]
	s.mu.Unlock()
	if !ok {
		return nil, port.ErrStoreNotFound
	}
	st.mu.Lock()
	return st, nil
}

func (s *Stores) Read(_ context.Context, id port.StoreID, fn func(r port.Reader) error) error {
	st, err := s.open(id)
	if err != nil {
		return err
	}
	defer st.mu.Unlock()
	return fn(&view{base: st.logs, baseTexts: st.texts})
}

func (s *Stores) Write(_ context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	st, err := s.open(id)
	if err != nil {
		return err
	}
	defer st.mu.Unlock()
	logs := st.logs
	t := &tx{
		view: view{base: logs, baseTexts: st.texts, pending: map[change.Ref][]change.Record{}, texts: map[change.Ref]string{}},
		meta: change.Meta{Op: s.ids.New(), At: s.clock.Now(), Actor: op.Actor},
		ids:  s.ids,
	}
	if err := fn(t); err != nil {
		return err
	}
	if len(t.pending) == 0 && len(t.texts) == 0 {
		return nil
	}
	if err := op.Check(); err != nil {
		return err
	}
	for ref, recs := range t.pending {
		logs[ref] = append(logs[ref], recs...)
	}
	for ref, text := range t.texts {
		if text == "" {
			delete(st.texts, ref)
		} else {
			st.texts[ref] = text
		}
	}
	st.writes++
	return nil
}

// Versions は port.StoreVersions。版の印は、その Store の保存が成功した回数。
func (s *Stores) Versions(context.Context) (map[port.StoreID]string, error) {
	s.mu.Lock()
	ids := make(map[port.StoreID]*store, len(s.stores))
	maps.Copy(ids, s.stores)
	s.mu.Unlock()
	out := make(map[port.StoreID]string, len(ids))
	for id, st := range ids {
		st.mu.Lock()
		out[id] = strconv.Itoa(st.writes)
		st.mu.Unlock()
	}
	return out, nil
}

// tx と種類ごとの Writer は、本物(infra/gitstore)にもほぼ同じものがある。gitstore は apptest を
// import できず、port は口の定義だけを置くので、まとめない。両者は RunStoresContract で揃える。
type tx struct {
	view
	meta change.Meta
	ids  port.IDs
}

func (t *tx) append(ref change.Ref, drafts []change.Draft) error {
	if !ref.Valid() {
		return fmt.Errorf("%w: id %q", port.ErrInvalidOperation, ref.ID)
	}
	// 行がなければ pending に足さない。足すと、行のない Write が行を書く Write として扱われる。
	if len(drafts) == 0 {
		return nil
	}
	recs := change.Stamp(t.log(ref), drafts, t.meta, t.ids.New)
	t.pending[ref] = append(t.pending[ref], recs...)
	return nil
}

// describe は、説明を書く。保存してある説明と同じなら何もしない。
func (t *tx) describe(ref change.Ref, text string) error {
	if !ref.Valid() {
		return fmt.Errorf("%w: id %q", port.ErrInvalidOperation, ref.ID)
	}
	if len(t.log(ref)) == 0 {
		return port.ErrNotFound
	}
	if text == t.baseTexts[ref] {
		delete(t.texts, ref)
		return nil
	}
	t.texts[ref] = text
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

func (w idWriter) Describe(id change.ID, text string) error {
	return w.t.describe(change.Ref{Kind: w.kind, ID: id}, text)
}

type membersWriter struct{ t *tx }

func (w membersWriter) Append(drafts ...change.Draft) error {
	return w.t.append(change.Ref{Kind: change.KindMembers}, drafts)
}
