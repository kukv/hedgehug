package apptest

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func contractWriteRead(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	mustRead(t, s, id, func(r port.Reader) error {
		w, err := r.Workspaces().Get(cWorkspace.ID)
		if err != nil || w != cWorkspace {
			t.Errorf("workspace = %+v, %v", w, err)
		}
		p, err := r.Projects().Get(cProject.ID)
		if err != nil || !reflect.DeepEqual(p, cProject) {
			t.Errorf("project = %+v, %v", p, err)
		}
		it, err := r.Items().Get(cItem.ID)
		if err != nil || !sameItem(it, cItem) || it.Created.By != "taro" || it.Created.At.IsZero() {
			t.Errorf("item = %+v, %v", it, err)
		}
		ms, err := r.Members().List()
		if err != nil || !slices.Equal(ms, []member.Member{{Login: "taro", Name: "taro"}}) {
			t.Errorf("members = %+v, %v", ms, err)
		}
		return nil
	})
}

func contractStamped(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, port.Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: "修復"}, setTitle(cItem.ID, "認証", "ログイン"))
	mustRead(t, s, id, func(r port.Reader) error {
		ws, _ := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: cWorkspace.ID})
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: cItem.ID})
		if err != nil || len(log) != 2 || len(ws) != 1 {
			t.Fatalf("logs = %+v / %+v, %v", ws, log, err)
		}
		c, set := log[0], log[1]
		if c.T != change.Create || len(c.Prev) != 0 || c.By != "taro" || c.Actor != change.User("taro") {
			t.Errorf("create = %+v", c)
		}
		if c.Op != ws[0].Op || !c.At.Equal(ws[0].At) {
			t.Errorf("rows of one write differ: %+v / %+v", c, ws[0])
		}
		if set.Op == c.Op || !slices.Equal(set.Prev, []change.ID{c.ID}) || set.By != change.BotLogin || set.Actor.Kind != change.ActorSync {
			t.Errorf("set = %+v", set)
		}
		if it, _ := r.Items().Get(cItem.ID); it.Title != "ログイン" {
			t.Errorf("title = %q", it.Title)
		}
		return nil
	})
}

func contractFailedWrite(t *testing.T, s port.Stores, id port.StoreID) {
	boom := errors.New("boom")
	err := s.Write(t.Context(), id, taro, func(tx port.Tx) error {
		if err := writeFixture(tx); err != nil {
			return err
		}
		return boom
	})
	// fn のエラーは、errors.Is で同じエラーと分かればよい(包んでよい)。
	if !errors.Is(err, boom) {
		t.Fatalf("Write error = %v, want boom", err)
	}
	mustRead(t, s, id, func(r port.Reader) error {
		if n, err := fixtureSeen(r); n != 0 || err != nil {
			t.Errorf("after failed write: %d of %d parts seen, %v", n, fixtureParts, err)
		}
		return nil
	})
}

func contractTxSeesOwn(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		if err := writeFixture(tx); err != nil {
			return err
		}
		if err := tx.WriteItems().Append(cItem.ID, change.NewSet("title", "認証", "二段階認証")); err != nil {
			return err
		}
		it, err := tx.Items().Get(cItem.ID)
		if err != nil || it.Title != "二段階認証" {
			t.Errorf("item in tx = %+v, %v", it, err)
		}
		return nil
	})
}

// 削除済みの保持日数は、変えていない Store では 30(08「保持日数」)。変える側は G で、
// 本物の store.yaml を書き換えるテストは gitstore に、偽物は Stores.SetRetentionDays にある。
func contractRetentionDays(t *testing.T, s port.Stores, id port.StoreID) {
	mustRead(t, s, id, func(r port.Reader) error {
		if days, err := r.RetentionDays(); err != nil || days != 30 {
			t.Errorf("RetentionDays = %d, %v; want 30", days, err)
		}
		return nil
	})
}

func contractNotFound(t *testing.T, s port.Stores, id port.StoreID) {
	mustRead(t, s, id, func(r port.Reader) error {
		if _, err := r.Workspaces().Get("01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("workspace: %v", err)
		}
		if _, err := r.Projects().Get("01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("project: %v", err)
		}
		if _, err := r.Items().Get("01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("item: %v", err)
		}
		if ms, err := r.Members().List(); err != nil || len(ms) != 0 {
			t.Errorf("members of an empty store = %+v, %v", ms, err)
		}
		if _, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: "01NOPE00000000000000000000"}); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("changes: %v", err)
		}
		return nil
	})
	if err := s.Read(t.Context(), "no-such-store", func(port.Reader) error { return nil }); !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("Read of unknown store: %v", err)
	}
	if err := s.Write(t.Context(), "no-such-store", taro, func(port.Tx) error { return nil }); !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("Write to unknown store: %v", err)
	}
}

func contractSerialized(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Go(func() {
			if err := s.Write(t.Context(), id, taro, setTitle(cItem.ID, "", fmt.Sprint(i))); err != nil {
				t.Errorf("Write %d: %v", i, err)
			}
		})
	}
	wg.Wait()
	mustRead(t, s, id, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: cItem.ID})
		if err != nil || len(log) == 0 {
			t.Fatalf("log = %+v, %v", log, err)
		}
		prevs := map[change.ID]bool{}
		for _, rec := range log[1:] {
			if len(rec.Prev) != 1 {
				t.Errorf("concurrent tips: %+v", rec)
				continue
			}
			prevs[rec.Prev[0]] = true
		}
		if len(log) != 11 || len(prevs) != 10 {
			t.Errorf("writes were not serialized: %d rows, %d distinct prev", len(log), len(prevs))
		}
		return nil
	})
}

func contractList(t *testing.T, s port.Stores, id port.StoreID) {
	second := workspace.Workspace{ID: "01CONTRACTWS00000000000000", Name: "API", Prefix: "API"}
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		d, err := workspace.Create(second)
		if err != nil {
			return err
		}
		return tx.WriteWorkspaces().Append(second.ID, d)
	})
	mustRead(t, s, id, func(r port.Reader) error {
		ws, err := r.Workspaces().List()
		if err != nil || !slices.Equal(ws, []workspace.Workspace{second, cWorkspace}) {
			t.Errorf("List = %+v, %v", ws, err)
		}
		return nil
	})
}

// contractListFollowsMove は、Items().List で一度読んだアイテムを同じ Tx で別のプロジェクトへ動かすと、
// そのあとの List が動いた先で返すことを確かめる(実装がアイテムのプロジェクトを覚えていても)。
func contractListFollowsMove(t *testing.T, s port.Stores, id port.StoreID) {
	const other project.ID = "01CONTRACTPR00000000000009"
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		if its, err := tx.Items().List(other); err != nil || len(its) != 0 {
			t.Errorf("other before = %+v, %v", its, err)
		}
		before, err := tx.Items().List(cProject.ID)
		if err != nil || len(before) != 1 {
			t.Fatalf("before = %+v, %v", before, err)
		}
		place := before[0].Place
		place.Project = other
		if err := tx.WriteItems().Append(cItem.ID, change.NewSet("place", before[0].Place, place)); err != nil {
			return err
		}
		if its, err := tx.Items().List(other); err != nil || len(its) != 1 || its[0].ID != cItem.ID {
			t.Errorf("other after = %+v, %v", its, err)
		}
		if its, err := tx.Items().List(cProject.ID); err != nil || len(its) != 0 {
			t.Errorf("left = %+v, %v", its, err)
		}
		return nil
	})
}

// contractProjectListFollowsMove は、Projects().List で一度読んだプロジェクトを同じ Tx で別のワークスペースへ動かすと、
// そのあとの List が動いた先で返すことを確かめる(実装がプロジェクトのワークスペースを覚えていても)。
func contractProjectListFollowsMove(t *testing.T, s port.Stores, id port.StoreID) {
	const other workspace.ID = "01CONTRACTWS00000000000009"
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		if ps, err := tx.Projects().List(other); err != nil || len(ps) != 0 {
			t.Errorf("other before = %+v, %v", ps, err)
		}
		before, err := tx.Projects().List(cWorkspace.ID)
		if err != nil || len(before) != 1 {
			t.Fatalf("before = %+v, %v", before, err)
		}
		place := before[0].Place
		place.Workspace = other
		if err := tx.WriteProjects().Append(cProject.ID, change.NewSet("place", before[0].Place, place)); err != nil {
			return err
		}
		if ps, err := tx.Projects().List(other); err != nil || len(ps) != 1 || ps[0].ID != cProject.ID {
			t.Errorf("other after = %+v, %v", ps, err)
		}
		if ps, err := tx.Projects().List(cWorkspace.ID); err != nil || len(ps) != 0 {
			t.Errorf("left = %+v, %v", ps, err)
		}
		return nil
	})
}
