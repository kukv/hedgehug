package gitstore

import (
	"errors"
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
