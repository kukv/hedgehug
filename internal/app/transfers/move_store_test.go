package transfers_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

const outsideGoneID item.ID = "01TESTIT000000000000000012"

func toTeam() transfers.MoveInput {
	return transfers.MoveInput{ToStore: store2, ToWorkspace: teamWS, ToProject: teamProject, ToParent: ptr(teamEpic)}
}

// loginsOf は、team(S2)のメンバーのログイン名を、辞書順で返す。
func loginsOf(t *testing.T, s port.Stores) []string {
	t.Helper()
	var logins []string
	for _, m := range membersOf(t, s) {
		logins = append(logins, m.Login)
	}
	slices.Sort(logins)
	return logins
}

// displayNameOf は、team(S2)のメンバー login の表示名(いなければ "")。
func displayNameOf(t *testing.T, s port.Stores, login string) string {
	t.Helper()
	for _, m := range membersOf(t, s) {
		if m.Login == login {
			return m.Name
		}
	}
	return ""
}

// membersOf は、team(S2)のメンバー。
func membersOf(t *testing.T, s port.Stores) []member.Member {
	t.Helper()
	var ms []member.Member
	if err := s.Read(t.Context(), store2, func(r port.Reader) (err error) { ms, err = r.Members().List(); return }); err != nil {
		t.Fatal(err)
	}
	return ms
}

// exists は、Store にアイテム id のフォルダがあるか。
func exists(t *testing.T, s port.Stores, store port.StoreID, id item.ID) bool {
	t.Helper()
	err := s.Read(t.Context(), store, func(r port.Reader) error { _, err := r.Items().Get(id); return err })
	if err != nil && !errors.Is(err, port.ErrNotFound) {
		t.Fatal(err)
	}
	return err == nil
}

// crossFixture は、Task HH-2(担当者 hanako、ウォッチ taro、hanako のコメント、ステータスを Todo から InProgress へ)の下に
// 削除済みの Step HH-3 と生きている Step HH-4 を置き、外との関連(Epic HH-1 が Task を relates、Task が Epic を blocks、
// 削除済みの外の Task HH-5 が Task を relates)と中の関連(Task が Step HH-4 を relates)を足したフィクスチャ。
// 移動元(main)で hanako の表示名は「花子」。
func crossFixture(t *testing.T) fixture {
	t.Helper()
	f := newFixture(t)
	withSteps(t, f)
	outside := addItem(t, f.rec, store1, item.Item{ID: outsideGoneID, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Number: 5}, Kind: item.Task, Rank: "X", Title: "外"})
	addRelation(t, f.rec, apptest.SampleEpic, apptest.SampleTask, "relates")
	addRelation(t, f.rec, apptest.SampleTask, apptest.SampleEpic, "blocks")
	addRelation(t, f.rec, apptest.SampleTask, stepLiveID, "relates")
	addRelation(t, f.rec, outside.ID, apptest.SampleTask, "relates")
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, outside.ID); err != nil {
		t.Fatal(err)
	}
	op := port.Operation{Actor: change.User("hanako"), Summary: "テスト"}
	err := f.rec.Write(t.Context(), store1, op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteMembers().Append(member.Rename("hanako", "hanako", "花子")),
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet("assignee", nil, "hanako"),
				change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusInProgress),
				change.NewComment("01TESTCM000000000000000001", nil, comment.Value("確認しました"))))
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// assertMovedIn は、移動先の Task の行が、写した行(移動元の行 srcLog がそのまま先頭)のあとに moved_in を持つことを確かめる。
func assertMovedIn(t *testing.T, f fixture, srcLog []change.Record) {
	t.Helper()
	dst := logOf(t, f.rec, store2, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
	if len(dst) <= len(srcLog) || dst[0].ID != srcLog[0].ID || dst[len(srcLog)-1].ID != srcLog[len(srcLog)-1].ID {
		t.Fatalf("log が写っていない: %d 行 → %d 行", len(srcLog), len(dst))
	}
	mi := transfer.MovedInOf(dst)
	if len(mi) != 1 || mi[0].Old != "HH-2" || mi[0].FromStore != string(store1) || mi[0].FromStoreName != "main" ||
		mi[0].FromWorkspaceName != "Web" || mi[0].FromProjectTitle != "ログイン" || mi[0].FromWorkspace != apptest.SampleWorkspace {
		t.Fatalf("moved_in = %+v", mi)
	}
	if mi[0].Names[apptest.SampleStatusTodo] != "Todo" || mi[0].Names[apptest.SampleStatusInProgress] != "InProgress" || mi[0].Names["status"] != "ステータス" {
		t.Errorf("names = %v(値の名前とプロパティの名前を持つ)", mi[0].Names)
	}
}

// assertSourceRemoved は、移動元から 3 つのフォルダが消え、転送記録がアイテムごとに 1 件あることを確かめる。
func assertSourceRemoved(t *testing.T, f fixture) {
	t.Helper()
	for _, id := range []item.ID{apptest.SampleTask, stepLiveID, stepGoneID} {
		if exists(t, f.rec, store1, id) {
			t.Errorf("移動元に %s が残っている", id)
		}
	}
	recs := recordsOf(t, f.rec, store1)
	if len(recs) != 3 {
		t.Fatalf("転送記録 = %+v", recs)
	}
	for _, r := range recs {
		if r.Entity != "item" || r.ToStore != string(store2) || r.ToStoreName != "team" || r.ToRemote != "" {
			t.Errorf("記録 = %+v", r)
		}
	}
	if r := recs[0]; r.Target != apptest.SampleTask || r.Old != "HH-2" || r.New != "TM-2" {
		t.Errorf("移すアイテムの記録 = %+v", r)
	}
}

// 移動先にフォルダが写り、移動元からは消え、転送記録が 1 件ずつ残る。移動先の番号は TM-2 から(Epic が TM-1)。
func TestMoveAcrossStoresCopiesTheFolderAndRemovesTheSource(t *testing.T) {
	f := crossFixture(t)
	srcLog := logOf(t, f.rec, store1, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
	got, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam())
	if err != nil {
		t.Fatal(err)
	}
	if got.ToStore != store2 || got.Item != apptest.SampleTask || got.Display != "TM-2" {
		t.Errorf("moved = %+v", got)
	}
	for id, want := range map[item.ID]int{apptest.SampleTask: 2, stepLiveID: 3, stepGoneID: 4} {
		it := itemOf(t, f.rec, store2, id)
		if it.Place.Workspace != teamWS || it.Place.Project != teamProject || it.Place.Number != want {
			t.Errorf("%s = %+v", id, it.Place)
		}
	}
	if task := itemOf(t, f.rec, store2, apptest.SampleTask); task.Place.Parent == nil || *task.Place.Parent != teamEpic {
		t.Errorf("移すアイテムの親 = %v", task.Place.Parent)
	}
	if gone := itemOf(t, f.rec, store2, stepGoneID); gone.Deleted == nil {
		t.Error("削除済みのまま移る")
	}
	assertMovedIn(t, f, srcLog)
	assertSourceRemoved(t, f)
	wantSummary := "HH-2 を main の Web から team の Team へ移動"
	if s := f.rec.summaries(store2); s[len(s)-1] != wantSummary {
		t.Errorf("移動先のアクティビティ = %q", s)
	}
	if s := f.rec.summaries(store1); s[len(s)-1] != wantSummary {
		t.Errorf("移動元のアクティビティ = %q", s)
	}
}

// Review Focus 4: 移すものの外(削除済みを含む)との関連は両側から消え、中の関連は残る。
func TestMoveAcrossStoresCutsRelationsToOutsideItemsOnBothSides(t *testing.T) {
	f := crossFixture(t)
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); err != nil {
		t.Fatal(err)
	}
	moved := itemOf(t, f.rec, store2, apptest.SampleTask)
	if len(moved.Relations) != 1 || moved.Relations[0] != (item.Relation{Type: "relates", To: stepLiveID}) {
		t.Errorf("移したものの関連 = %+v(中の Step との関連だけ残る)", moved.Relations)
	}
	for _, id := range []item.ID{apptest.SampleEpic, outsideGoneID} {
		if rel := itemOf(t, f.rec, store1, id).Relations; len(rel) != 0 {
			t.Errorf("外の %s の関連 = %+v(相手の側からも消える)", id, rel)
		}
	}
}

// Review Focus 5: 移動先のメンバーにいない人(hanako は担当者・コメント・保存の実行者に現れる)は 1 回だけ足され、
// 大文字小文字だけが違う人(taro と Taro)は足さず、移動先の綴りに置き換わる。足す行は移動の操作と同じ操作。
func TestMoveAcrossStoresAddsMissingPeopleOnceAndMapsCaseInsensitively(t *testing.T) {
	f := crossFixture(t)
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); err != nil {
		t.Fatal(err)
	}
	if logins := loginsOf(t, f.rec); !slices.Equal(logins, []string{"Taro", "hanako"}) {
		t.Errorf("移動先のメンバー = %v", logins)
	}
	moved := itemOf(t, f.rec, store2, apptest.SampleTask)
	if moved.Assignee != "hanako" || !slices.Equal(moved.Watchers, []string{"Taro"}) {
		t.Errorf("担当者 = %q, ウォッチ = %v", moved.Assignee, moved.Watchers)
	}
	members := logOf(t, f.rec, store2, change.Ref{Kind: change.KindMembers})
	itemLog := logOf(t, f.rec, store2, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
	var joins int
	var joinOp, movedOp change.ID
	for _, r := range members {
		if r.T == change.Add && strings.Contains(string(r.To), "hanako") {
			joins++
			joinOp = r.Op
		}
	}
	for _, r := range itemLog {
		if r.T == change.MovedIn {
			movedOp = r.Op
		}
	}
	if joins != 1 || joinOp == "" || joinOp != movedOp {
		t.Errorf("メンバーの追加 %d 行、操作 %q と moved_in の操作 %q は同じ", joins, joinOp, movedOp)
	}
	// 足した人の表示名は移動元のメンバーと同じ(02「保存」)。
	if name := displayNameOf(t, f.rec, "hanako"); name != "花子" {
		t.Errorf("移動先の hanako の表示名 = %q", name)
	}
}

// assertUntouched は、移動元がそのまま(行の数・転送記録なし)で、移動先に移すものが残っていないことを確かめる。
func assertUntouched(t *testing.T, f fixture, srcBefore int) {
	t.Helper()
	if after := logOf(t, f.rec, store1, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}); len(after) != srcBefore {
		t.Errorf("移動元の行が変わった: %d → %d", srcBefore, len(after))
	}
	if len(recordsOf(t, f.rec, store1)) != 0 {
		t.Error("転送記録を書いた")
	}
	for _, id := range []item.ID{apptest.SampleTask, stepLiveID, stepGoneID} {
		if exists(t, f.rec, store2, id) {
			t.Errorf("移動先に %s が残っている", id)
		}
		if it := itemOf(t, f.rec, store1, id); it.Place.Workspace != apptest.SampleWorkspace {
			t.Errorf("移動元の %s が変わった: %+v", id, it.Place)
		}
	}
}

// Review Focus 1: 送れなかった(移動先に remote があり、書き込みが断られた)とき、移動先に何も残らず、移動元もそのまま。
func TestMoveAcrossStoresThatCannotBeSentLeavesBothStoresUntouched(t *testing.T) {
	f := crossFixture(t)
	f.deliver.reasons = map[port.StoreID]syncstatus.Reason{store2: syncstatus.WriteDenied}
	srcBefore := len(logOf(t, f.rec, store1, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}))
	nextBefore := nextNumber(t, f.rec, store2, teamWS)
	membersBefore := loginsOf(t, f.rec)
	_, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam())
	var se *transfers.SendError
	if !errors.As(err, &se) || se.Store != "team" || se.Reason != syncstatus.WriteDenied {
		t.Fatalf("err = %v", err)
	}
	if want := "team に送信できないため移動できません: remote への書き込みが許可されていません"; err.Error() != want {
		t.Errorf("文 = %q, want %q", err.Error(), want)
	}
	assertUntouched(t, f, srcBefore)
	if n := nextNumber(t, f.rec, store2, teamWS); n != nextBefore {
		t.Errorf("移動先の次の番号 = %d, want %d", n, nextBefore)
	}
	if got := loginsOf(t, f.rec); !slices.Equal(got, membersBefore) {
		t.Errorf("移動先のメンバー = %v, want %v", got, membersBefore)
	}
}

// 理由で表せない Deliver の失敗(移動先に保存していない)は、そのまま返し、移動元に触らない。
func TestMoveAcrossStoresReturnsADeliverErrorWithoutTouchingTheSource(t *testing.T) {
	f := crossFixture(t)
	srcBefore := len(logOf(t, f.rec, store1, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}))
	f.svc.Delivery = failingDelivery{err: errBoom}
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	assertUntouched(t, f, srcBefore)
}

// 移動先で選べない親(移動先のプロジェクトの Epic でない)は port.ErrNotFound で、どちらの Store にも保存しない。
func TestMoveAcrossStoresRefusesAParentThatIsNotAnEpicOfTheDestination(t *testing.T) {
	f := crossFixture(t)
	before := len(f.rec.summaries())
	in := toTeam()
	in.ToParent = ptr(apptest.SampleEpic)
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, in); !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if len(f.rec.summaries()) != before {
		t.Error("何も保存しない")
	}
}

// failWrite は、Store store への Write を errBoom で失敗させる。before があれば、失敗させる前に呼ぶ。
type failWrite struct {
	port.Stores
	store  port.StoreID
	before func()
}

var errBoom = errors.New("boom")

func (w failWrite) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	if id == w.store {
		if w.before != nil {
			w.before()
		}
		return errBoom
	}
	return w.Stores.Write(ctx, id, op, fn)
}

// failingDelivery は、何も保存せず err を返す Delivery。
type failingDelivery struct{ err error }

func (d failingDelivery) Deliver(context.Context, port.StoreID, port.Operation, func(tx port.Tx) error) (syncstatus.Reason, error) {
	return "", d.err
}

// notDiscarded は、最初の Deliver だけ、保存したうえで「送れず、保存を捨てられなかった」を返す Delivery。
type notDiscarded struct {
	inner transfers.Delivery
	done  *bool
}

var errNotSentKept = errors.New("send conflict")

func (d notDiscarded) Deliver(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) (syncstatus.Reason, error) {
	r, err := d.inner.Deliver(ctx, id, op, fn)
	if *d.done || r != "" || err != nil {
		return r, err
	}
	*d.done = true
	return "", errors.Join(errNotSentKept, port.ErrSaveNotDiscarded)
}

// assertRolledBack は、移動先が移動の前に戻り(写したフォルダがなく、足したメンバーを外した)、移動元がそのままであることを確かめる。
func assertRolledBack(t *testing.T, f fixture, srcBefore int, membersBefore []string) {
	t.Helper()
	assertUntouched(t, f, srcBefore)
	if s := f.rec.summaries(store2); s[len(s)-1] != "TM-2 の移動を取り消し" {
		t.Errorf("移動先のアクティビティ = %q", s)
	}
	if got := loginsOf(t, f.rec); !slices.Equal(got, membersBefore) {
		t.Errorf("移動先のメンバー = %v, want 移動前の %v(足した hanako を外す)", got, membersBefore)
	}
}

// 移動元の保存が失敗したら、移動先に写したフォルダを消し、足したメンバー(hanako)も外して戻し、エラーを返す。
func TestMoveAcrossStoresRollsBackTheDestinationWhenTheSourceSaveFails(t *testing.T) {
	f := crossFixture(t)
	srcBefore := len(logOf(t, f.rec, store1, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}))
	membersBefore := loginsOf(t, f.rec)
	f.svc.Stores = failWrite{Stores: f.rec, store: store1}
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	assertRolledBack(t, f, srcBefore, membersBefore)
	// 外した hanako の表示名(移動で「花子」にした)も戻すので、あとで足し直すとログイン名で出る。
	write(t, f.rec, store2, func(tx port.Tx) error { return tx.WriteMembers().Append(member.Join("hanako")) })
	if name := displayNameOf(t, f.rec, "hanako"); name != "hanako" {
		t.Errorf("足し直した hanako の表示名 = %q, want hanako", name)
	}
}

// 移動先の保存と戻しのあいだに、別の操作が同じ人(大文字小文字だけが違う綴りを含む)を足していたら、戻しでは外さない。
func TestMoveAcrossStoresRollbackKeepsAMemberAddedAgainByAnotherOperation(t *testing.T) {
	f := crossFixture(t)
	f.svc.Stores = failWrite{Stores: f.rec, store: store1, before: func() {
		write(t, f.rec, store2, func(tx port.Tx) error { return tx.WriteMembers().Append(member.Join("Hanako")) })
	}}
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	if got := loginsOf(t, f.rec); !slices.Equal(got, []string{"Taro", "hanako"}) {
		t.Errorf("移動先のメンバー = %v(あとから足した人は外さない)", got)
	}
	if exists(t, f.rec, store2, apptest.SampleTask) {
		t.Error("移動先に Task が残っている")
	}
}

// (0) で読んだあと (4) の前に、移動元で移すものの下に子が足されたら、移動は失敗し、移動先を戻し、移動元はそのまま(子も親も残る)。
func TestMoveAcrossStoresFailsWhenTheSourceGainsAChildMeanwhile(t *testing.T) {
	f := crossFixture(t)
	srcBefore := len(logOf(t, f.rec, store1, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}))
	membersBefore := loginsOf(t, f.rec)
	const newStep item.ID = "01TESTIT000000000000000013"
	added := false
	f.deliver.inside = func(port.StoreID) {
		if added {
			return
		}
		added = true
		addItem(t, f.rec, store1, item.Item{ID: newStep, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: ptr(apptest.SampleTask), Number: 6}, Kind: item.Step, Rank: "Y", Title: "あとから"})
	}
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); err == nil {
		t.Fatal("移動が成功した")
	}
	assertRolledBack(t, f, srcBefore, membersBefore)
	if it := itemOf(t, f.rec, store1, newStep); it.Place.Parent == nil || *it.Place.Parent != apptest.SampleTask {
		t.Errorf("足した子 = %+v", it.Place)
	}
	if exists(t, f.rec, store2, newStep) {
		t.Error("足した子が移動先にある")
	}
}

// 移動先の保存が送れず、捨てられもしなかった(保存されたかもしれない)ときは、移動元を保存せず、移動先を戻してエラーを返す。
func TestMoveAcrossStoresRollsBackASaveThatWasNotDiscarded(t *testing.T) {
	f := crossFixture(t)
	srcBefore := len(logOf(t, f.rec, store1, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}))
	membersBefore := loginsOf(t, f.rec)
	f.svc.Delivery = notDiscarded{inner: f.deliver, done: new(bool)}
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); !errors.Is(err, port.ErrSaveNotDiscarded) {
		t.Fatalf("err = %v", err)
	}
	assertRolledBack(t, f, srcBefore, membersBefore)
}

// retried は、最初の Deliver だけ、fn を一度呼んで保存を捨て(送信が重なった)、between を保存してから、もう一度 inner で
// 保存する Delivery(SyncSave が fn を呼び直す場面)。
type retried struct {
	inner   transfers.Delivery
	stores  port.Stores
	between func(tx port.Tx) error
	done    *bool
}

var errConflict = errors.New("conflict")

func (d retried) Deliver(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) (syncstatus.Reason, error) {
	if !*d.done {
		*d.done = true
		err := d.stores.Write(ctx, id, op, func(tx port.Tx) error { return errors.Join(fn(tx), errConflict) })
		if !errors.Is(err, errConflict) {
			return "", err
		}
		if err := d.stores.Write(ctx, id, port.Operation{Actor: change.User("taro"), Summary: "ほかの人の更新"}, d.between); err != nil {
			return "", err
		}
	}
	return d.inner.Deliver(ctx, id, op, fn)
}

// fn が呼び直されたら、番号とメンバーは呼び直しの tx から決め、戻しは最後の呼び出しで足した人だけを外す。
// あいだにほかの人が TM-2 を使い hanako を足したので、移したものは TM-3 になり、hanako は足さず、戻しても外さない。
func TestMoveAcrossStoresDecidesNumbersAndPeopleFromTheLastCall(t *testing.T) {
	f := crossFixture(t)
	f.svc.Stores = failWrite{Stores: f.rec, store: store1}
	f.svc.Delivery = retried{inner: f.deliver, stores: f.rec, done: new(bool), between: func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(teamWS, numbering.Draft(numbering.Items, 2)),
			tx.WriteMembers().Append(member.Join("hanako")),
		)
	}}
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	if s := f.rec.summaries(store2); s[len(s)-1] != "TM-3 の移動を取り消し" {
		t.Errorf("移動先のアクティビティ = %q", s)
	}
	if got := loginsOf(t, f.rec); !slices.Equal(got, []string{"Taro", "hanako"}) {
		t.Errorf("移動先のメンバー = %v(ほかの人が足した hanako は外さない)", got)
	}
	if exists(t, f.rec, store2, apptest.SampleTask) {
		t.Error("移動先に Task が残っている")
	}
}
