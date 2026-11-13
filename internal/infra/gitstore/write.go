package gitstore

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
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
	t := &tx{
		view: view{root: root, pending: map[change.Ref][]change.Record{}, texts: map[change.Ref]string{}},
		meta: change.Meta{Op: s.ids.New(), At: s.clock.Now(), Actor: op.Actor},
		ids:  s.ids,
	}
	if err := fn(t); err != nil {
		return err
	}
	if len(t.pending) == 0 && len(t.texts) == 0 {
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
	log, err := t.log(ref)
	if err != nil {
		return err
	}
	t.pending[ref] = append(t.pending[ref], change.Stamp(log, drafts, t.meta, t.ids.New)...)
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
