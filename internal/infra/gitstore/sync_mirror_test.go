package gitstore

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
)

// 保存形式「同期」の手順 7: 空のミラーには最初の送信として送り、遅れているミラーには続きを送る。
func TestMirrorSendsToEmptyAndBehindMirrors(t *testing.T) {
	remote, mirror := newRemote(t), newRemote(t)
	a := firstMachine(t, remote)
	if err := a.rep.Mirror(t.Context(), a.id, mirror); err != nil {
		t.Fatalf("empty mirror: %v", err)
	}
	if remoteHead(t, mirror) != headCommit(t, a.dir).Hash {
		t.Error("the mirror did not get the head")
	}
	rename(t, a, "新しい名前")
	if err := a.rep.Mirror(t.Context(), a.id, mirror); err != nil {
		t.Fatalf("behind mirror: %v", err)
	}
	if remoteHead(t, mirror) != headCommit(t, a.dir).Hash {
		t.Error("the mirror did not catch up")
	}
	if err := a.rep.Mirror(t.Context(), a.id, mirror); err != nil {
		t.Errorf("same head: %v", err)
	}
}

// ミラーとの通信のあいだ、Store のロックを持たない(Store の読み書きを最大 60 秒止めない)。
func TestMirrorDoesNotHoldTheStoreLockWhileTalkingToTheMirror(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(entered) })
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(srv.Close)
	a := firstMachine(t, newRemote(t))
	done := make(chan error, 1)
	go func() { done <- a.rep.Mirror(t.Context(), a.id, port.Remote{URL: srv.URL + "/o/m.git"}) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Mirror did not reach the mirror")
	}
	wrote := make(chan struct{})
	go func() {
		rename(t, a, "ミラーの通信中")
		close(wrote)
	}()
	select {
	case <-wrote:
	case <-time.After(2 * time.Second):
		t.Error("a write was blocked while Mirror talked to the mirror")
	}
	close(release)
	if err := <-done; err != nil {
		t.Logf("Mirror: %v", err)
	}
	<-wrote
}

// ミラーにだけ保存があれば、上書きしないため送らない。
func TestMirrorRefusesMirrorAhead(t *testing.T) {
	remote, mirror := newRemote(t), newRemote(t)
	a := firstMachine(t, remote)
	if err := a.rep.Mirror(t.Context(), a.id, mirror); err != nil {
		t.Fatal(err)
	}
	b := cloneMachine(t, mirror, "B", t0.Add(time.Minute))
	rename(t, b, "ミラーだけの名前")
	b.sync(t, mirror)
	before := remoteHead(t, mirror)
	rename(t, a, "A の名前")
	if err := a.rep.Mirror(t.Context(), a.id, mirror); !errors.Is(err, port.ErrMirrorAhead) {
		t.Errorf("Mirror = %v", err)
	}
	if remoteHead(t, mirror) != before {
		t.Error("the mirror was overwritten")
	}
}

// 空でなく、共通の履歴がないミラーには送らない。
func TestMirrorRefusesUnrelatedMirror(t *testing.T) {
	remote, mirror := newRemote(t), newRemote(t)
	a := firstMachine(t, remote)
	pushRaw(t, mirror, map[string]string{"README.md": "x\n"})
	if err := a.rep.Mirror(t.Context(), a.id, mirror); !errors.Is(err, port.ErrMirrorUnrelated) {
		t.Errorf("Mirror = %v", err)
	}
}

// ミラーに送っても、remote の受け取った先端(未送信の数え方)は変わらない。
func TestMirrorDoesNotTouchTheRemoteRefs(t *testing.T) {
	remote, mirror := newRemote(t), newRemote(t)
	a := firstMachine(t, remote)
	rename(t, a, "未送信")
	if err := a.rep.Mirror(t.Context(), a.id, mirror); err != nil {
		t.Fatal(err)
	}
	if n, err := a.rep.Unsent(t.Context(), a.id); err != nil || n != 1 {
		t.Errorf("Unsent = %d, %v", n, err)
	}
}
