package gitstore

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// open は、Store のロックを取り、途中で止まった保存が残っていれば戻す。
func open(root string) (unlock func(), err error) {
	unlock, err = lock(root)
	if err != nil {
		return nil, err
	}
	if err := recoverInterrupted(root); err != nil {
		unlock()
		return nil, err
	}
	return unlock, nil
}

func (s *Stores) Read(_ context.Context, id port.StoreID, fn func(r port.Reader) error) error {
	root, err := s.dir(id)
	if err != nil {
		return err
	}
	unlock, err := open(root)
	if err != nil {
		return err
	}
	defer unlock()
	return fn(&view{root: root})
}

func (s *Stores) Write(_ context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	root, err := s.dir(id)
	if err != nil {
		return err
	}
	unlock, err := open(root)
	if err != nil {
		return err
	}
	defer unlock()
	return s.writeLocked(root, op, fn)
}

// writeLocked は、Store のロックを持っているあいだの Write の本体。
func (s *Stores) writeLocked(root string, op port.Operation, fn func(tx port.Tx) error) error {
	t := &tx{
		view: view{root: root, pending: map[change.Ref][]change.Record{}, texts: map[change.Ref]string{}, removed: map[change.Ref]bool{}},
		meta: change.Meta{Op: s.ids.New(), At: s.clock.Now(), Actor: op.Actor},
		ids:  s.ids,
	}
	if err := fn(t); err != nil {
		return err
	}
	if len(t.pending) == 0 && len(t.texts) == 0 && len(t.removed) == 0 && len(t.transfers) == 0 {
		return nil
	}
	// Summary はコミットメッセージの 1 行目、ログイン名はコミットの作者になる。
	if err := op.Check(); err != nil {
		return err
	}
	return s.commit(root, t, op)
}

// tx と種類ごとの Writer は、偽物(app/apptest)にもほぼ同じものがある。gitstore は apptest を
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
	// 行がなければ pending に足さない。足すと、空のコミットや空の log.jsonl ができる。
	if len(drafts) == 0 {
		return nil
	}
	if err := port.CheckRuleDrafts(t.meta.Actor, drafts); err != nil {
		return err
	}
	log, err := t.log(ref)
	if err != nil {
		return err
	}
	t.pending[ref] = append(t.pending[ref], change.Stamp(log, drafts, t.meta, t.ids.New)...)
	switch ref.Kind {
	case change.KindItem:
		delete(t.places, ref.ID) // 動いたかもしれない
	case change.KindProject:
		delete(t.projectPlaces, ref.ID) // 動いたかもしれない
	}
	return nil
}

// describe は、説明を書く(コミットのときにファイルにする)。作業ツリーと同じなら何もしない。
func (t *tx) describe(ref change.Ref, text string) error {
	if !ref.Valid() {
		return fmt.Errorf("%w: id %q", port.ErrInvalidOperation, ref.ID)
	}
	log, err := t.log(ref)
	if err != nil {
		return err
	}
	if len(log) == 0 {
		return port.ErrNotFound
	}
	saved, err := readDescription(t.root, ref)
	if err != nil {
		return err
	}
	if text == saved {
		delete(t.texts, ref)
		return nil
	}
	t.texts[ref] = text
	return nil
}

func (t *tx) WriteWorkspaces() port.WorkspaceWriter { return idWriter{t, change.KindWorkspace} }
func (t *tx) WriteProjects() port.ProjectWriter     { return idWriter{t, change.KindProject} }
func (t *tx) WriteItems() port.ItemWriter           { return itemWriter{idWriter{t, change.KindItem}} }
func (t *tx) WriteMembers() port.MemberWriter       { return membersWriter{t} }
func (t *tx) WriteTransfers() port.TransferWriter   { return transferWriter{t} }

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

type itemWriter struct{ idWriter }

// Import は、行を写して pending にする(コミットのときにフォルダを作る)。説明があれば texts にする。
func (w itemWriter) Import(id item.ID, log []change.Record, description string) error {
	ref := change.Ref{Kind: change.KindItem, ID: id}
	if !ref.Valid() || len(log) == 0 || w.t.removed[ref] {
		return fmt.Errorf("%w: cannot import item %q", port.ErrInvalidOperation, id)
	}
	if have, err := w.t.log(ref); err != nil {
		return err
	} else if len(have) > 0 {
		return fmt.Errorf("%w: item %q already exists", port.ErrInvalidOperation, id)
	}
	recs := make([]change.Record, 0, len(log))
	for _, r := range log {
		recs = append(recs, cloneRecord(r))
	}
	w.t.pending[ref] = recs
	delete(w.t.places, id)
	if description != "" {
		w.t.texts[ref] = description
	}
	return nil
}

// Remove は、同じ Tx で書いた行と説明を捨て、フォルダを消す印を付ける(コミットのときに消す)。
func (w itemWriter) Remove(id item.ID) error {
	ref := change.Ref{Kind: change.KindItem, ID: id}
	if !ref.Valid() {
		return fmt.Errorf("%w: id %q", port.ErrInvalidOperation, id)
	}
	log, err := w.t.log(ref)
	if err != nil {
		return err
	}
	if len(log) == 0 {
		return port.ErrNotFound
	}
	delete(w.t.pending, ref)
	delete(w.t.texts, ref)
	delete(w.t.places, id)
	w.t.removed[ref] = true
	return nil
}

type transferWriter struct{ t *tx }

func (w transferWriter) Append(r transfer.Record) error {
	if err := port.CheckTransfer(r); err != nil {
		return err
	}
	r.ID, r.Op, r.At, r.By = w.t.ids.New(), w.t.meta.Op, w.t.meta.At, w.t.meta.Actor.By()
	w.t.transfers = append(w.t.transfers, r)
	return nil
}
