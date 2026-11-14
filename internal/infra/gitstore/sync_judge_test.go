package gitstore

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// judgeRecorder は、判定の差し込み口に渡された保存を覚える。err を返すと判定の失敗になる。
type judgeRecorder struct {
	calls  [][]port.ReceivedSave
	titles []string // 呼ばれたときに read で読んだサンプルのタスクのタイトル
	err    error
	judged *apptest.JudgedSaves
}

// withJudge は、m の同期に判定の差し込み口と、判定済みの記録(空)を付ける。
func withJudge(m machine) *judgeRecorder {
	j := &judgeRecorder{judged: apptest.NewJudgedSaves()}
	m.rep.Judged = j.judged
	m.rep.Judge = func(_ context.Context, store port.StoreID, saves []port.ReceivedSave, read func(fn func(r port.Reader) error) error) error {
		if store != m.id {
			return errors.New("wrong store")
		}
		j.calls = append(j.calls, saves)
		err := read(func(r port.Reader) error {
			it, err := r.Items().Get(apptest.SampleTask)
			j.titles = append(j.titles, it.Title)
			return err
		})
		if err != nil {
			return err
		}
		return j.err
	}
	return j
}

// last は、最後の呼び出しで渡された保存の印。呼ばれていなければ nil。
func (j *judgeRecorder) last() []string {
	if len(j.calls) == 0 {
		return nil
	}
	var out []string
	for _, s := range j.calls[len(j.calls)-1] {
		out = append(out, s.Key.Version)
	}
	return out
}

func (j *judgeRecorder) known(t *testing.T, m machine, h plumbing.Hash) port.Match {
	t.Helper()
	match, err := j.judged.Known(t.Context(), m.id, port.SaveKey{Version: h.String()})
	if err != nil {
		t.Fatal(err)
	}
	return match
}

func headOf(t *testing.T, m machine) plumbing.Hash {
	t.Helper()
	return headCommit(t, m.dir).Hash
}

func only(t *testing.T, got []string, h plumbing.Hash) {
	t.Helper()
	if len(got) != 1 || got[0] != h.String() {
		t.Errorf("judged saves = %v, want only %s", got, h)
	}
}

// B の保存を A が同期で受け取ると、判定の差し込み口にその保存だけが渡る。もう一度同期しても渡らない。
func TestJudgeReceivesOthersSave(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(a)
	a.sync(t, remote)
	if len(j.calls) != 0 {
		t.Fatalf("judged %v with nothing received", j.last())
	}
	b.write(t, "割り当て", set("assignee", "", "hanako"))
	b.sync(t, remote)
	bs := headOf(t, b)
	a.sync(t, remote)
	only(t, j.last(), bs)
	save := j.calls[len(j.calls)-1][0]
	if save.Key.Author != "taro" || save.Key.Op == "" || len(save.Entities) != 1 ||
		save.Entities[0].Ref != (change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}) || len(save.Entities[0].Rows) != 1 {
		t.Errorf("save = %+v", save)
	}
	if j.titles[len(j.titles)-1] != "ログイン画面を作る" {
		t.Errorf("read = %q", j.titles[len(j.titles)-1])
	}
	n := len(j.calls)
	a.sync(t, remote)
	if len(j.calls) != n {
		t.Errorf("judged again: %v", j.last())
	}
}

// 記録のないマシンの最初の同期では、同期の前から持っている保存を渡さず、そのあと受け取った保存だけを渡す。
func TestJudgeBaselineOnFirstSync(t *testing.T) {
	remote := newRemote(t)
	firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	b.write(t, "1", set("rank", "a", "b"))
	b.sync(t, remote)
	c := cloneMachine(t, remote, "C", t0.Add(2*time.Hour))
	b.write(t, "2", set("rank", "b", "c"))
	b.sync(t, remote)
	j := withJudge(c)
	c.sync(t, remote)
	only(t, j.last(), headOf(t, b))
}

// 記録のないマシンの最初の同期が送信で失敗しても、同期の前から持っていた保存は判定済みのまま。次の同期で
// 受け取った保存が渡る。
func TestBaselineSurvivesFailedFirstSync(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	b.write(t, "b", set("rank", "a", "b"))
	b.sync(t, remote)
	j := withJudge(a)
	a.write(t, "a", set("title", "ログイン画面を作る", "ログイン"))
	a.rep.beforeSend = func() error { return port.ErrWriteDenied }
	if _, err := a.rep.Sync(t.Context(), a.id, remote); !errors.Is(err, port.ErrWriteDenied) {
		t.Fatalf("sync err = %v, want ErrWriteDenied", err)
	}
	if len(j.calls) != 0 {
		t.Fatalf("judged %v on a failed sync", j.last())
	}
	a.rep.beforeSend = nil
	a.sync(t, remote)
	only(t, j.last(), headOf(t, b))
}

// 自分の保存は、ほかのマシンが合わせて送り返しても渡らない。
func TestOwnSaveNeverJudged(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(a)
	a.write(t, "a", set("title", "ログイン画面を作る", "ログイン"))
	as := headOf(t, a)
	a.sync(t, remote)
	b.write(t, "b", set("rank", "a", "b"))
	bs := headOf(t, b)
	b.sync(t, remote) // A の保存と合わせて送る
	a.sync(t, remote)
	only(t, j.last(), bs)
	for _, call := range j.calls {
		for _, s := range call {
			if s.Key.Version == as.String() {
				t.Errorf("own save %s was judged", as)
			}
		}
	}
}

// 送信の失敗で同期が失敗したら判定済みにせず、次の成功した同期で同じ保存が渡る。
func TestJudgeRetriedAfterFailedPush(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(a)
	a.sync(t, remote)
	b.write(t, "b", set("rank", "a", "b"))
	b.sync(t, remote)
	a.rep.beforeSend = func() error { return port.ErrWriteDenied }
	if _, err := a.rep.Sync(t.Context(), a.id, remote); !errors.Is(err, port.ErrWriteDenied) {
		t.Fatalf("sync err = %v, want ErrWriteDenied", err)
	}
	if len(j.calls) != 0 || j.known(t, a, headOf(t, b)) != port.NotJudged {
		t.Fatalf("judged on a failed sync: %v", j.last())
	}
	a.rep.beforeSend = nil
	a.sync(t, remote)
	only(t, j.last(), headOf(t, b))
}

// 判定がエラーを返しても同期は成功し、受け取った保存を判定済みにしない。次の同期でもう一度渡る。
// このマシンが送った保存は判定済みにするので、次の同期で渡らない。
func TestJudgeErrorKeepsSync(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(a)
	a.sync(t, remote)
	b.write(t, "b", set("rank", "a", "b"))
	b.sync(t, remote)
	bs := headOf(t, b)
	a.write(t, "a", set("title", "ログイン画面を作る", "ログイン"))
	as := headOf(t, a)
	j.err = errors.New("通知を書けない")
	if res := a.sync(t, remote); res != (port.SyncResult{Sent: 1, Received: 1}) {
		t.Errorf("sync = %+v, want Sent 1, Received 1", res)
	}
	only(t, j.last(), bs)
	if j.known(t, a, bs) != port.NotJudged {
		t.Error("the save was marked although judging failed")
	}
	if j.known(t, a, as) != port.SameVersion {
		t.Error("own sent save was not marked")
	}
	j.err = nil
	a.sync(t, remote)
	if len(j.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(j.calls))
	}
	only(t, j.last(), bs)
	if j.known(t, a, bs) != port.SameVersion {
		t.Error("the save was not marked after judging")
	}
}

// 閲覧のみの同期でも、受け取った保存が渡る。
func TestJudgeReceiveOnly(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(b)
	view := port.Remote{URL: remote.URL, ReceiveOnly: true}
	if _, err := b.rep.Sync(t.Context(), b.id, view); err != nil {
		t.Fatal(err)
	}
	a.write(t, "a", set("rank", "a", "b"))
	a.sync(t, remote)
	if _, err := b.rep.Sync(t.Context(), b.id, view); err != nil {
		t.Fatal(err)
	}
	only(t, j.last(), headOf(t, a))
}

// SyncSave で受け取った保存も渡り、自分の保存は渡らない。
func TestJudgeOnSyncSave(t *testing.T) {
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
	only(t, j.last(), headOf(t, b))
	if j.known(t, a, headOf(t, a)) != port.SameVersion {
		t.Error("own save was not marked")
	}
}

// SyncSave で判定がエラーを返しても、受け取った保存は次の同期で渡る(受け取ったあとの保存を判定済みにして、
// 受け取った保存に辿れなくなることがない)。
func TestJudgeErrorOnSyncSaveKeepsReceived(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(a)
	a.sync(t, remote)
	b.write(t, "b", set("rank", "a", "b"))
	b.sync(t, remote)
	bs := headOf(t, b)
	j.err = errors.New("通知を書けない")
	if _, err := a.rep.SyncSave(t.Context(), a.id, remote, saveOp, createMoved); err != nil {
		t.Fatal(err)
	}
	j.err = nil
	a.sync(t, remote)
	if len(j.calls) != 2 || !slices.Contains(j.last(), bs.String()) {
		t.Errorf("calls = %d, last = %v, want the received save %s again", len(j.calls), j.last(), bs)
	}
}
