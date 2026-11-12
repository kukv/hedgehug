package gitstore

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
)

// bareOf は、テストの remote の bare のフォルダ。
func bareOf(t *testing.T, remote port.Remote) string {
	t.Helper()
	remotesMu.Lock()
	defer remotesMu.Unlock()
	dir, ok := remotes[strings.TrimPrefix(remote.URL, "https://remote.invalid")]
	if !ok {
		t.Fatalf("no bare for %s", remote.URL)
	}
	return dir
}

// setRemoteBranch は、remote の main を h にする(h がゼロなら消す)。利用者が git で書き換えた場面を作る。
func setRemoteBranch(t *testing.T, remote port.Remote, h plumbing.Hash) {
	t.Helper()
	repo, err := git.PlainOpen(bareOf(t, remote))
	if err != nil {
		t.Fatal(err)
	}
	name := plumbing.NewBranchReferenceName("main")
	if h.IsZero() {
		err = repo.Storer.RemoveReference(name)
	} else {
		err = repo.Storer.SetReference(plumbing.NewHashReference(name, h))
	}
	if err != nil {
		t.Fatal(err)
	}
}

func remoteHead(t *testing.T, remote port.Remote) plumbing.Hash {
	t.Helper()
	repo, err := git.PlainOpen(bareOf(t, remote))
	if err != nil {
		t.Fatal(err)
	}
	ref, err := repo.Reference(plumbing.NewBranchReferenceName("main"), true)
	if err != nil {
		t.Fatal(err)
	}
	return ref.Hash()
}

func rename(t *testing.T, m machine, title string) {
	t.Helper()
	m.write(t, "タイトルを変更", set("title", "", title))
}

// 成功した同期は、remote が持つ先端を Seen として返す。次の同期にその Seen を渡しても、続きなら通る。
func TestSyncReturnsSeenAndContinuesFromIt(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Minute))
	resA, err := a.rep.Sync(t.Context(), a.id, remote)
	if err != nil {
		t.Fatal(err)
	}
	if resA.Seen != remoteHead(t, remote).String() {
		t.Fatalf("Seen = %q, remote head %s", resA.Seen, remoteHead(t, remote))
	}
	rename(t, b, "B の名前")
	b.sync(t, remote)
	resA2, err := a.rep.Sync(t.Context(), a.id, port.Remote{URL: remote.URL, Seen: resA.Seen})
	if err != nil || resA2.Received != 1 || resA2.Seen != headCommit(t, a.dir).Hash.String() {
		t.Errorf("second sync = %+v, %v", resA2, err)
	}
}

// removeMainKeepOther は、他のブランチが残っていて main だけがない(remote は空と言われない。決定 C2-12)場面を作る。
func removeMainKeepOther(t *testing.T, remote port.Remote, old plumbing.Hash) {
	t.Helper()
	repo, err := git.PlainOpen(bareOf(t, remote))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("other"), old)); err != nil {
		t.Fatal(err)
	}
	setRemoteBranch(t, remote, plumbing.ZeroHash)
}

func unsentOf(t *testing.T, m machine) int {
	t.Helper()
	n, err := m.rep.Unsent(t.Context(), m.id)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// 保存形式「remote の履歴が書き換えられたとき」: 前に受け取った位置の続きでなければ、Store を変えずに止める。
// 巻き戻したとき、空にしたとき(決定 C2-12)も同じ。
func TestSyncStopsWhenTheRemoteWasRewritten(t *testing.T) {
	cases := map[string]func(t *testing.T, remote port.Remote, old plumbing.Hash){
		"rewound":        func(t *testing.T, remote port.Remote, old plumbing.Hash) { setRemoteBranch(t, remote, old) },
		"emptied":        func(t *testing.T, remote port.Remote, _ plumbing.Hash) { setRemoteBranch(t, remote, plumbing.ZeroHash) },
		"branch removed": removeMainKeepOther,
	}
	for name, rewrite := range cases {
		t.Run(name, func(t *testing.T) {
			remote := newRemote(t)
			a := firstMachine(t, remote)
			old := remoteHead(t, remote)
			rename(t, a, "新しい名前")
			res, err := a.rep.Sync(t.Context(), a.id, remote)
			if err != nil {
				t.Fatal(err)
			}
			rewrite(t, remote, old)
			rename(t, a, "未送信の名前")
			before := snap(t, a)
			unsent := unsentOf(t, a)
			_, err = a.rep.Sync(t.Context(), a.id, port.Remote{URL: remote.URL, Seen: res.Seen})
			if !errors.Is(err, port.ErrRemoteRewritten) {
				t.Errorf("Sync = %v", err)
			}
			unchanged(t, a, before)
			// 止めた同期は、未送信の数を動かさない(06「未送信の保存はそのまま残る」)。
			if n := unsentOf(t, a); n != unsent {
				t.Errorf("Unsent = %d; want %d", n, unsent)
			}
		})
	}
}

// 前に受け取った位置がなければ(付け替えた・追加した直後)、書き換えの検査はしない。読めない位置も同じ。
func TestSyncWithoutSeenSkipsTheRewriteCheck(t *testing.T) {
	for _, seen := range []string{"", "not-a-hash"} {
		remote := newRemote(t)
		a := firstMachine(t, remote)
		old := remoteHead(t, remote)
		rename(t, a, "新しい名前")
		a.sync(t, remote)
		setRemoteBranch(t, remote, old)
		if _, err := a.rep.Sync(t.Context(), a.id, port.Remote{URL: remote.URL, Seen: seen}); err != nil {
			t.Errorf("Seen %q: %v", seen, err)
		}
	}
}

// 決定 C2-13: 受け取りだけの同期は、合わせても送らない。
func TestSyncReceiveOnlyDoesNotSend(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Minute))
	rename(t, a, "A の名前")
	a.sync(t, remote)
	rename(t, b, "B の名前")
	before := remoteHead(t, remote)
	res, err := b.rep.Sync(t.Context(), b.id, port.Remote{URL: remote.URL, ReceiveOnly: true})
	if err != nil || res.Sent != 0 || res.Received != 1 || res.Seen != before.String() {
		t.Errorf("Sync = %+v, %v", res, err)
	}
	if remoteHead(t, remote) != before {
		t.Error("the remote moved")
	}
}

// 決定 C2-14: 未送信は、前に受け取った remote の先端から辿れない保存の数。
func TestUnsentCountsSavesNotSent(t *testing.T) {
	m := localMachine(t)
	n, err := m.rep.Unsent(t.Context(), m.id)
	if err != nil || n == 0 {
		t.Fatalf("never synced: %d, %v", n, err)
	}
	remote := newRemote(t)
	m.sync(t, remote)
	if n, err := m.rep.Unsent(t.Context(), m.id); err != nil || n != 0 {
		t.Errorf("after sync: %d, %v", n, err)
	}
	rename(t, m, "1")
	rename(t, m, "2")
	if n, err := m.rep.Unsent(t.Context(), m.id); err != nil || n != 2 {
		t.Errorf("after 2 saves: %d, %v", n, err)
	}
}
