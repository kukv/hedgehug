package gitstore

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/infra/gitstore/saves"
)

// opOf は、m の先端の保存の op。
func opOf(t *testing.T, m machine) change.ID {
	t.Helper()
	op := saves.KeyOf(headCommit(t, m.dir)).Op
	if op == "" {
		t.Fatal("the head has no op")
	}
	return op
}

// wantAsked は、最後の呼び出しで in.Own が ask の op に返した答えが want であること。
func wantAsked(t *testing.T, j *judgeRecorder, want map[change.ID]bool) {
	t.Helper()
	if len(j.own) == 0 {
		t.Fatal("judge was not called")
	}
	got := j.own[len(j.own)-1]
	for op, w := range want {
		if got[op] != w {
			t.Errorf("in.Own(%s) = %v, want %v", op, got[op], w)
		}
	}
}

// wantOwn は、m の判定の記録で、op がこのマシンの保存か(Own)が want であること。
func wantOwn(t *testing.T, j *judgeRecorder, m machine, op change.ID, want bool) {
	t.Helper()
	got, err := j.judged.Own(t.Context(), m.id, op)
	if err != nil || got != want {
		t.Errorf("Own(%s) = %v, %v; want %v", op, got, err, want)
	}
}

// 送った自分の保存は、ほかのマシンが合わせて送り返したあとの判定でも、このマシンの保存として渡る。
func TestOwnAfterPush(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(a)
	a.sync(t, remote)
	a.write(t, "a", set("title", "ログイン画面を作る", "ログイン"))
	s1 := opOf(t, a)
	a.sync(t, remote)
	b.write(t, "b", set("rank", "a", "b"))
	bop := opOf(t, b)
	b.sync(t, remote)
	j.ask = []change.ID{s1, bop}
	a.sync(t, remote)
	wantAsked(t, j, map[change.ID]bool{s1: true, bop: false})
}

// まだ送っていない保存は、判定の記録になくても、その同期の判定でこのマシンの保存として渡り、そのあと記録に残る。
func TestOwnPendingSave(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(a)
	a.sync(t, remote)
	b.write(t, "b", set("rank", "a", "b"))
	bop := opOf(t, b)
	b.sync(t, remote)
	a.write(t, "a", set("title", "ログイン画面を作る", "ログイン"))
	s2 := opOf(t, a)
	wantOwn(t, j, a, s2, false)
	j.ask = []change.ID{s2, bop}
	a.sync(t, remote)
	wantAsked(t, j, map[change.ID]bool{s2: true, bop: false})
	wantOwn(t, j, a, s2, true)
}

// 判定の記録がないマシンの最初の同期で、最初の印付けが判定済みにした未送信の保存も、このマシンの保存として残る。
func TestOwnFirstSync(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(a)
	a.write(t, "a", set("title", "ログイン画面を作る", "ログイン"))
	s3 := opOf(t, a)
	s3v := headOf(t, a)
	a.sync(t, remote)
	if len(j.calls) != 0 || j.known(t, a, s3v) != port.SameVersion {
		t.Fatalf("calls = %d, known = %v", len(j.calls), j.known(t, a, s3v))
	}
	wantOwn(t, j, a, s3, true)
	b.write(t, "b", set("rank", "a", "b"))
	b.sync(t, remote)
	j.ask = []change.ID{s3}
	a.sync(t, remote)
	wantAsked(t, j, map[change.ID]bool{s3: true})
}

// ほかのマシンの保存は、受け取って判定したあとも、このマシンの保存にならない。
func TestOwnOnlyThisMachine(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(a)
	a.sync(t, remote)
	b.write(t, "b", set("rank", "a", "b"))
	bop := opOf(t, b)
	b.sync(t, remote)
	j.ask = []change.ID{bop}
	a.sync(t, remote)
	wantAsked(t, j, map[change.ID]bool{bop: false})
	wantOwn(t, j, a, bop, false)
	a.write(t, "a", set("title", "ログイン画面を作る", "ログイン"))
	a.sync(t, remote)
	wantOwn(t, j, a, bop, false)
}

// 閲覧のみの同期では、記録にあるこのマシンの保存も、まだ送っていない保存も、in.Own が false。
func TestOwnReadOnly(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(b)
	b.sync(t, remote)
	b.write(t, "b", set("rank", "a", "b"))
	sent := opOf(t, b)
	b.sync(t, remote)
	wantOwn(t, j, b, sent, true)
	b.write(t, "b2", set("rank", "b", "c"))
	local := opOf(t, b)
	a.write(t, "a", set("title", "ログイン画面を作る", "ログイン"))
	a.sync(t, remote)
	j.ask = []change.ID{sent, local}
	if _, err := b.rep.Sync(t.Context(), b.id, port.Remote{URL: remote.URL, ReceiveOnly: true}); err != nil {
		t.Fatal(err)
	}
	if len(j.own) != 1 || !j.readOnly[0] {
		t.Fatalf("calls = %d, readOnly = %v; want 1 read-only call", len(j.own), j.readOnly)
	}
	wantAsked(t, j, map[change.ID]bool{sent: false, local: false})
}

// SyncSave で作って送った保存も、このマシンの保存として記録に残る。
func TestOwnOnSyncSave(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(a)
	a.sync(t, remote)
	b.write(t, "b", set("rank", "a", "b"))
	b.sync(t, remote)
	if _, err := a.rep.SyncSave(t.Context(), a.id, remote, saveOp, createMoved); err != nil {
		t.Fatal(err)
	}
	wantOwn(t, j, a, opOf(t, a), true)
}

// 同期の修復の保存は、このマシンが書いても、このマシンの保存にしない。
func TestOwnExcludesRepair(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(a)
	repairOnce(a)
	a.sync(t, remote)
	b.write(t, "b", set("rank", "a", "b"))
	b.sync(t, remote)
	a.write(t, "a", set("title", "ログイン画面を作る", "ログイン"))
	s := opOf(t, a)
	j.ask = []change.ID{s}
	a.sync(t, remote)
	fix := opOf(t, a)
	wantRepair(t, j.passed(t, headOf(t, a)))
	wantAsked(t, j, map[change.ID]bool{s: true})
	wantOwn(t, j, a, s, true)
	wantOwn(t, j, a, fix, false)
}

// 判定がエラーを返しても、最初の印付けだけが判定済みにした自分の保存は、このマシンの保存として記録に残る。
// 受け取った保存より後の自分の保存(late)は、判定済みにできない(決定 C4b2-5)ので記録しない。
func TestOwnKeptOnJudgeError(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(a)
	b.write(t, "b", set("rank", "a", "b"))
	b.sync(t, remote)
	a.write(t, "a", set("title", "ログイン画面を作る", "ログイン"))
	early := opOf(t, a)
	a.rep.beforeSend = func() error { return port.ErrWriteDenied }
	if _, err := a.rep.Sync(t.Context(), a.id, remote); !errors.Is(err, port.ErrWriteDenied) {
		t.Fatalf("sync err = %v, want ErrWriteDenied", err)
	}
	a.rep.beforeSend = nil
	wantOwn(t, j, a, early, false) // 最初の印付けだけが判定済みにした
	a.write(t, "a2", set("title", "ログイン", "ログイン画面"))
	late := opOf(t, a)
	j.err = errors.New("通知を書けない")
	j.ask = []change.ID{early, late}
	a.sync(t, remote)
	wantAsked(t, j, map[change.ID]bool{early: true, late: true})
	wantOwn(t, j, a, early, true)
	wantOwn(t, j, a, late, false)
}

// wantReceived は、m の判定の記録で、保存 k が同期で受け取ったもの(記録にあり、このマシンの保存でない。決定 C4d-16)
// かが want であること。
func wantReceived(t *testing.T, j *judgeRecorder, m machine, k port.SaveKey, want bool) {
	t.Helper()
	match, err := j.judged.Known(t.Context(), m.id, k)
	if err != nil {
		t.Fatal(err)
	}
	own, err := j.judged.Own(t.Context(), m.id, k.Op)
	if err != nil {
		t.Fatal(err)
	}
	if got := match != port.NotJudged && !own; got != want {
		t.Errorf("received(%s) = %v (known %v, own %v), want %v", k.Op, got, match, own, want)
	}
}

// remote なしで作って保存した Store に、空の remote を付けて最初の同期をすると、その保存はこのマシンの保存として
// 記録に残る(同期で受信にならない)。そのあと受け取ったほかのマシンの保存は、同期で受信になる(決定 C4d-16)。
func TestOwnBeforeRemote(t *testing.T) {
	remote := newRemote(t)
	dir := filepath.Join(t.TempDir(), "a")
	if err := Create(dir, "01STORE0000000000000000001", t0); err != nil {
		t.Fatal(err)
	}
	a := newMachine(t, dir, "A", t0)
	if err := apptest.Sample(t.Context(), a.stores, a.id); err != nil {
		t.Fatal(err)
	}
	j := withJudge(a)
	a.write(t, "a", set("title", "ログイン画面を作る", "ログイン"))
	mine := saves.KeyOf(headCommit(t, a.dir))
	a.sync(t, remote)
	wantOwn(t, j, a, mine.Op, true)
	wantReceived(t, j, a, mine, false)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	b.write(t, "b", set("rank", "a", "b"))
	theirs := saves.KeyOf(headCommit(t, b.dir))
	b.sync(t, remote)
	a.sync(t, remote)
	wantReceived(t, j, a, theirs, true)
	wantReceived(t, j, a, mine, false)
}
