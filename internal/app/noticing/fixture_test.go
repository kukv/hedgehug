package noticing_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/noticing"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const store port.StoreID = "S1"

// fixture は、サンプルの Store と偽物の受信箱。あなたは hanako。
type fixture struct {
	t       *testing.T
	stores  *apptest.Stores
	inbox   *apptest.Inbox
	svc     noticing.Service
	version int
}

func setup(t *testing.T) *fixture {
	t.Helper()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore(store)
	if err := apptest.Sample(t.Context(), s, store); err != nil {
		t.Fatal(err)
	}
	inbox := &apptest.Inbox{}
	return &fixture{t: t, stores: s, inbox: inbox, svc: noticing.Service{
		Inbox:    inbox,
		Identity: apptest.Identity{Login: "hanako"},
		List:     apptest.StoreList{Stores: []port.StoreInfo{{ID: store, Name: "main"}}},
	}}
}

// snapshot は、Store のエンティティごとのログの行数と説明の本文。
type snapshot struct {
	lens  map[change.Ref]int
	texts map[change.Ref]string
	logs  map[change.Ref][]change.Record
}

func (f *fixture) snapshot() snapshot {
	f.t.Helper()
	sn := snapshot{lens: map[change.Ref]int{}, texts: map[change.Ref]string{}, logs: map[change.Ref][]change.Record{}}
	err := f.stores.Read(f.t.Context(), store, func(r port.Reader) error {
		if err := sn.add(r, change.Ref{Kind: change.KindMembers}, nil); err != nil {
			return err
		}
		wss, err := r.Workspaces().List()
		if err != nil {
			return err
		}
		for _, ws := range wss {
			if err := sn.add(r, change.Ref{Kind: change.KindWorkspace, ID: ws.ID}, nil); err != nil {
				return err
			}
			if err := sn.addProjects(r, ws.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return sn
}

// addProjects は、ワークスペース ws のプロジェクトと、その下のアイテムを覚える。
func (sn snapshot) addProjects(r port.Reader, ws workspace.ID) error {
	ps, err := r.Projects().List(ws)
	if err != nil {
		return err
	}
	for _, p := range ps {
		if err := sn.add(r, change.Ref{Kind: change.KindProject, ID: p.ID}, func() (string, error) { return r.Projects().Description(p.ID) }); err != nil {
			return err
		}
		its, err := r.Items().List(p.ID)
		if err != nil {
			return err
		}
		for _, it := range its {
			if err := sn.add(r, change.Ref{Kind: change.KindItem, ID: it.ID}, func() (string, error) { return r.Items().Description(it.ID) }); err != nil {
				return err
			}
		}
	}
	return nil
}

// add は、ref のログと説明(desc が nil なら説明なし)を覚える。
func (sn snapshot) add(r port.Reader, ref change.Ref, desc func() (string, error)) error {
	log, err := r.Changes().Of(ref)
	if err != nil {
		return err
	}
	sn.lens[ref], sn.logs[ref] = len(log), log
	if desc != nil {
		sn.texts[ref], err = desc()
	}
	return err
}

// save は、actor の操作で fn をふつうに保存し、その保存を受け取った保存にする。
func (f *fixture) save(actor change.Actor, fn func(tx port.Tx) error) port.ReceivedSave {
	f.t.Helper()
	before := f.snapshot()
	if err := f.stores.Write(f.t.Context(), store, port.Operation{Actor: actor, Summary: "テスト"}, fn); err != nil {
		f.t.Fatal(err)
	}
	after := f.snapshot()
	var refs []change.Ref
	for ref := range after.lens {
		refs = append(refs, ref)
	}
	slices.SortFunc(refs, func(a, b change.Ref) int {
		return strings.Compare(string(a.Kind)+string(a.ID), string(b.Kind)+string(b.ID))
	})
	f.version++
	s := port.ReceivedSave{Key: port.SaveKey{Version: fmt.Sprintf("v%d", f.version), Author: actor.By()}}
	for _, ref := range refs {
		e := port.EntityRows{Ref: ref, Rows: after.logs[ref][before.lens[ref]:]}
		if text, ok := after.texts[ref]; ok && text != before.texts[ref] {
			e.Description = &text
		}
		if len(e.Rows) == 0 && e.Description == nil {
			continue
		}
		if len(e.Rows) > 0 && s.Key.At.IsZero() {
			s.Key.At, s.Key.Op = e.Rows[0].At, e.Rows[0].Op
		}
		s.Entities = append(s.Entities, e)
	}
	return s
}

// judge は、saves を判定する。偽物の read は Stores.Read。
func (f *fixture) judge(saves ...port.ReceivedSave) {
	f.t.Helper()
	if err := f.judgeErr(saves...); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) judgeErr(saves ...port.ReceivedSave) error {
	ctx := f.t.Context()
	return f.svc.Judge(ctx, store, saves, func(fn func(r port.Reader) error) error {
		return f.stores.Read(ctx, store, fn)
	})
}

func (f *fixture) notices() []port.Notice { return f.inbox.Notices() }

// rows は、受信箱の行を新しい順に返す。
func (f *fixture) rows() []port.NoticeRow {
	f.t.Helper()
	rows, err := f.inbox.List(f.t.Context(), port.NoticeQuery{Stores: []port.StoreID{store}})
	if err != nil {
		f.t.Fatal(err)
	}
	return rows
}

func (f *fixture) wantNone() {
	f.t.Helper()
	if got := f.notices(); len(got) != 0 {
		f.t.Errorf("notices = %+v, want none", got)
	}
}

func (f *fixture) wantOne() port.Notice {
	f.t.Helper()
	got := f.notices()
	if len(got) != 1 {
		f.t.Fatalf("notices = %+v, want one", got)
	}
	return got[0]
}

func user(login string) change.Actor { return change.User(login) }

// noIdentity は、「あなた」を決められない Identity。
type noIdentity struct{}

func (noIdentity) Me(context.Context, port.StoreID) (string, error) {
	return "", errors.New("no identity")
}
