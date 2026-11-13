package apptest

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// refuseWrite は、op で writeFixture と extra を書く Write が port.ErrInvalidOperation で断られ、
// 何も残らないことを確かめる。extra が返したエラーも port.ErrInvalidOperation でなければならない
// (wantExtraErr のとき)。
func refuseWrite(t *testing.T, s port.Stores, id port.StoreID, name string, op port.Operation, extra func(tx port.Tx) error, wantExtraErr bool) {
	t.Helper()
	var extraErr error
	err := s.Write(t.Context(), id, op, func(tx port.Tx) error {
		if err := writeFixture(tx); err != nil {
			return err
		}
		if extra != nil {
			extraErr = extra(tx)
		}
		return extraErr
	})
	if !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("%s: Write error = %v, want ErrInvalidOperation", name, err)
	}
	if wantExtraErr && !errors.Is(extraErr, port.ErrInvalidOperation) {
		t.Errorf("%s: Append error = %v, want ErrInvalidOperation", name, extraErr)
	}
	mustRead(t, s, id, func(r port.Reader) error {
		if n, err := fixtureSeen(r); n != 0 || err != nil {
			t.Errorf("%s: after a refused write: %d of %d parts seen, %v", name, n, fixtureParts, err)
		}
		return nil
	})
}

// 行を書く Write の Summary は、空でない 1 行。
func contractSummary(t *testing.T, s port.Stores, id port.StoreID) {
	for _, summary := range []string{"", "1 行目\n2 行目", "改行\r"} {
		refuseWrite(t, s, id, fmt.Sprintf("summary %q", summary), port.Operation{Actor: taro.Actor, Summary: summary}, nil, false)
	}
}

// 行のない Write(行のない Append だけのものを含む)は、Summary が空でもよく、何も残さない。
func contractEmptyWrite(t *testing.T, s port.Stores, id port.StoreID) {
	empty := port.Operation{Actor: taro.Actor}
	mustWrite(t, s, id, empty, func(port.Tx) error { return nil })
	mustWrite(t, s, id, empty, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteMembers().Append(),
			tx.WriteWorkspaces().Append(cWorkspace.ID),
			tx.WriteProjects().Append(cProject.ID),
			tx.WriteItems().Append(cItem.ID),
		)
	})
	mustRead(t, s, id, func(r port.Reader) error {
		if n, err := fixtureSeen(r); n != 0 || err != nil {
			t.Errorf("after empty writes: %d of %d parts seen, %v", n, fixtureParts, err)
		}
		if ws, err := r.Workspaces().List(); err != nil || len(ws) != 0 {
			t.Errorf("workspaces after empty writes = %+v, %v", ws, err)
		}
		return nil
	})
}

// 人の操作のログイン名は member.ValidLogin でなければ書かない(行の by になる)。
func contractActorLogin(t *testing.T, s port.Stores, id port.StoreID) {
	for _, login := range []string{"", "two words", "../x", "toolong-toolong-toolong-toolong-toolong-x"} {
		refuseWrite(t, s, id, fmt.Sprintf("login %q", login), port.Operation{Actor: change.User(login), Summary: "準備"}, nil, false)
	}
}

// 実行者は、行の actor と by に書いて読み戻すと同じになるものだけを書く。読み戻せない行は、読むときに
// 読み飛ばされて消えるため。
func contractActorRoundTrip(t *testing.T, s port.Stores, id port.StoreID) {
	for _, a := range []change.Actor{
		{},
		{Kind: "robot"},
		{Kind: change.ActorAutomation, Login: "taro"},
		{Kind: change.ActorAutomation, Login: "taro", Rule: "../x"},
		{Kind: change.ActorSync, Login: "taro"},
		{Kind: change.ActorGitHub, Rule: "01RULE"},
		{Kind: change.ActorUser, Login: "taro", Rule: "01RULE"},
	} {
		refuseWrite(t, s, id, fmt.Sprintf("actor %+v", a), port.Operation{Actor: a, Summary: "準備"}, nil, false)
	}
	for i, a := range []change.Actor{
		{Kind: change.ActorAutomation, Login: "taro", Rule: "01RULE"},
		{Kind: change.ActorGitHub},
		{Kind: change.ActorNone},
	} {
		wsID := change.ID(fmt.Sprintf("01ROUNDTRIP%d", i))
		mustWrite(t, s, id, port.Operation{Actor: a, Summary: "準備"}, func(tx port.Tx) error {
			return tx.WriteWorkspaces().Append(wsID, change.NewCreate(map[string]any{"id": string(wsID)}))
		})
		mustRead(t, s, id, func(r port.Reader) error {
			log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: wsID})
			if err != nil || len(log) != 1 || log[0].Actor != a {
				t.Errorf("actor %+v: log = %+v, %v", a, log, err)
			}
			return nil
		})
	}
}

var malformedIDs = []change.ID{"", "../x", "a/b", ".", "あ"}

// 使えない ID(change.ValidID でないもの)は、読むと見つからず、書くと断られる。
func contractMalformedIDs(t *testing.T, s port.Stores, id port.StoreID) {
	for _, bad := range malformedIDs {
		create := change.NewCreate(map[string]any{"id": string(bad)})
		for kind, appendBad := range map[string]func(tx port.Tx) error{
			"workspace": func(tx port.Tx) error { return tx.WriteWorkspaces().Append(bad, create) },
			"project":   func(tx port.Tx) error { return tx.WriteProjects().Append(bad, create) },
			"item":      func(tx port.Tx) error { return tx.WriteItems().Append(bad, create) },
		} {
			refuseWrite(t, s, id, fmt.Sprintf("%s %q", kind, bad), taro, appendBad, true)
		}
	}
	mustWrite(t, s, id, taro, writeFixture)
	mustRead(t, s, id, func(r port.Reader) error {
		for _, bad := range malformedIDs {
			expectNotFound(t, fmt.Sprintf("malformed %q", bad), r, bad)
			// 一覧は、見つからない ID と同じく空を返す。
			if ps, err := r.Projects().List(bad); err != nil || len(ps) != 0 {
				t.Errorf("Projects().List(%q) = %+v, %v", bad, ps, err)
			}
			if its, err := r.Items().List(bad); err != nil || len(its) != 0 {
				t.Errorf("Items().List(%q) = %+v, %v", bad, its, err)
			}
		}
		return nil
	})
}

// expectNotFound は、bad を指す読み取りがどれも port.ErrNotFound になることを確かめる。
func expectNotFound(t *testing.T, name string, r port.Reader, bad change.ID) {
	t.Helper()
	_, err1 := r.Workspaces().Get(bad)
	_, err2 := r.Projects().Get(bad)
	_, err3 := r.Items().Get(bad)
	errs := []error{err1, err2, err3}
	for _, kind := range []change.Kind{change.KindWorkspace, change.KindProject, change.KindItem} {
		_, err := r.Changes().Of(change.Ref{Kind: kind, ID: bad})
		errs = append(errs, err)
	}
	for i, err := range errs {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: read %d: %v, want ErrNotFound", name, i, err)
		}
	}
}

// メンバーの一覧のログは ID が空の Ref だけで指せる。知らない種類の Ref も見つからない。
func contractRefs(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	mustRead(t, s, id, func(r port.Reader) error {
		if log, err := r.Changes().Of(change.Ref{Kind: change.KindMembers}); err != nil || len(log) != 2 {
			t.Errorf("members log = %+v, %v", log, err)
		}
		for _, ref := range []change.Ref{
			{Kind: change.KindMembers, ID: "x"},
			{Kind: change.KindMembers, ID: cWorkspace.ID},
			{Kind: "robot", ID: cItem.ID},
			{Kind: "", ID: cItem.ID},
		} {
			if _, err := r.Changes().Of(ref); !errors.Is(err, port.ErrNotFound) {
				t.Errorf("Of(%+v) = %v, want ErrNotFound", ref, err)
			}
		}
		return nil
	})
}

// contractNestedStores は、Store A の Write の中から Store B に書けることを確かめる(別の Store の
// Write は互いを待たない)。
func contractNestedStores(t *testing.T, s port.Stores, a, b port.StoreID) {
	done := make(chan error, 1)
	go func() {
		done <- s.Write(t.Context(), a, taro, func(tx port.Tx) error {
			if err := writeFixture(tx); err != nil {
				return err
			}
			return s.Write(t.Context(), b, taro, writeFixture)
		})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Write: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a write to store B inside a write to store A did not finish")
	}
	for _, sid := range []port.StoreID{a, b} {
		mustRead(t, s, sid, func(r port.Reader) error {
			if n, err := fixtureSeen(r); n != fixtureParts || err != nil {
				t.Errorf("store %s: %d of %d parts seen, %v", sid, n, fixtureParts, err)
			}
			return nil
		})
	}
}

// 自動化の行は、同じ保存の中で automation:<ルール> の実行者として読める(B4-1)。
func contractRuleRows(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	const rule change.ID = "01RULE0000000000000000001"
	op := port.Operation{Actor: change.User("taro"), Summary: "HH-1 のステータスを変更"}
	mustWrite(t, s, id, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(cItem.ID,
			change.NewSet("status", nil, "Done"),
			change.NewSet("assignee", nil, "taro").ByRule(rule))
	})
	mustRead(t, s, id, func(r port.Reader) error {
		recs, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: cItem.ID})
		if err != nil || len(recs) < 3 {
			t.Fatalf("log = %+v, %v", recs, err)
		}
		last, before := recs[len(recs)-1], recs[len(recs)-2]
		if last.Actor.Kind != change.ActorAutomation || last.Actor.Rule != rule || last.By != "taro" {
			t.Errorf("rule row = %+v", last)
		}
		if before.Actor != change.User("taro") || before.Op != last.Op {
			t.Errorf("the rule row must be in the same operation as the user row: %+v, %+v", before, last)
		}
		return nil
	})
}

// 人でない実行者の操作と、使えないルールの ID では、自動化の行を書けない。
func contractRuleRowsRefused(t *testing.T, s port.Stores, id port.StoreID) {
	cases := []struct {
		name string
		op   port.Operation
		rule change.ID
	}{
		{"sync actor", port.Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: "同期の修復"}, "01RULE0000000000000000001"},
		{"malformed rule id", taro, "bad/id"},
	}
	for _, c := range cases {
		extra := func(tx port.Tx) error {
			return tx.WriteItems().Append(cItem.ID, change.NewSet("status", nil, "X").ByRule(c.rule))
		}
		refuseWrite(t, s, id, c.name, c.op, extra, true)
	}
}
