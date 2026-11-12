package gitstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
)

// pushRaw は、hedgehug を通さずに、ファイルを書いたコミットを remote へ送る(利用者が git で行った操作の代わり)。
func pushRaw(t *testing.T, remote port.Remote, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	repo, err := initMain(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, _ := repo.Worktree()
	for p, v := range files {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(v), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := wt.Add(p); err != nil {
			t.Fatal(err)
		}
	}
	sig := &object.Signature{Name: "someone", Email: "someone@example.invalid", When: t0}
	if _, err := wt.Commit("raw", &git.CommitOptions{Author: sig, Committer: sig}); err != nil {
		t.Fatal(err)
	}
	rm := git.NewRemote(repo.Storer, &config.RemoteConfig{Name: "raw", URLs: []string{remote.URL}})
	if err := rm.Push(&git.PushOptions{RemoteName: "raw", RefSpecs: []config.RefSpec{"refs/heads/main:refs/heads/main"}}); err != nil {
		t.Fatal(err)
	}
}

func TestSyncRefusesRemoteThatIsNotAStore(t *testing.T) {
	remote := newRemote(t)
	pushRaw(t, remote, map[string]string{"README.md": "hello"})
	_, _, dir := newStore(t)
	m := newMachine(t, dir, "A", t0)
	before := snap(t, m)
	if _, err := m.rep.Sync(t.Context(), m.id, remote); !errors.Is(err, port.ErrRemoteNotStore) {
		t.Errorf("err = %v", err)
	}
	unchanged(t, m, before)
}

// store.yaml に format がなければ、Attach と同じく Store ではないとして断る。
func TestSyncRefusesRemoteStoreWithoutFormat(t *testing.T) {
	remote := newRemote(t)
	pushRaw(t, remote, map[string]string{".hedgehug/store.yaml": "store: 01STORE0000000000000000001\n"})
	_, _, dir := newStore(t)
	m := newMachine(t, dir, "A", t0)
	before := snap(t, m)
	if _, err := m.rep.Sync(t.Context(), m.id, remote); !errors.Is(err, port.ErrRemoteNotStore) {
		t.Errorf("err = %v", err)
	}
	unchanged(t, m, before)
}

func TestSyncRefusesUnrelatedHistory(t *testing.T) {
	remote := newRemote(t)
	firstMachine(t, remote)
	// 同じ Store ID でも、別に作った Store は履歴を共有しない。同じ時刻に作ると最初のコミットまで同じになるので、
	// 作った時刻をずらす。
	dir := filepath.Join(t.TempDir(), "z")
	if err := Create(dir, "01STORE0000000000000000001", t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	m := newMachine(t, dir, "Z", t0)
	before := snap(t, m)
	if _, err := m.rep.Sync(t.Context(), m.id, remote); !errors.Is(err, port.ErrNoCommonHistory) {
		t.Errorf("err = %v", err)
	}
	unchanged(t, m, before)
}

func TestSyncRefusesNewerFormat(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0)
	// b が、新しい版の hedgehug で保存したことにする。
	if err := os.WriteFile(filepath.Join(b.dir, ".hedgehug", "store.yaml"), []byte("store: 01STORE0000000000000000001\nformat: 2\nretention_days: 30\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo, _ := git.PlainOpen(b.dir)
	wt, _ := repo.Worktree()
	if _, err := wt.Add(".hedgehug/store.yaml"); err != nil {
		t.Fatal(err)
	}
	sig := &object.Signature{Name: "newer", Email: "newer@hedgehug.invalid", When: t0}
	if _, err := wt.Commit("newer", &git.CommitOptions{Author: sig, Committer: sig}); err != nil {
		t.Fatal(err)
	}
	b.sync(t, remote)
	before := snap(t, a)
	if _, err := a.rep.Sync(t.Context(), a.id, remote); !errors.Is(err, port.ErrRemoteNewerFormat) {
		t.Errorf("err = %v", err)
	}
	unchanged(t, a, before)
}

func TestSyncRefusesRemoteWithoutTheBranch(t *testing.T) {
	remote := newRemote(t)
	repo := t.TempDir()
	r, _ := initMain(repo, false)
	wt, _ := r.Worktree()
	sig := &object.Signature{Name: "x", Email: "x@example.invalid", When: t0}
	if _, err := wt.Commit("empty", &git.CommitOptions{Author: sig, Committer: sig, AllowEmptyCommits: true}); err != nil {
		t.Fatal(err)
	}
	rm := git.NewRemote(r.Storer, &config.RemoteConfig{Name: "raw", URLs: []string{remote.URL}})
	if err := rm.Push(&git.PushOptions{RemoteName: "raw", RefSpecs: []config.RefSpec{"refs/heads/main:refs/heads/other"}}); err != nil {
		t.Fatal(err)
	}
	_, _, dir := newStore(t)
	m := newMachine(t, dir, "A", t0)
	before := snap(t, m)
	if _, err := m.rep.Sync(t.Context(), m.id, remote); !errors.Is(err, port.ErrNoCommonHistory) {
		t.Errorf("err = %v", err)
	}
	unchanged(t, m, before)
}

// 保存が途中で止まった跡が残っていても、同期はそれを戻してから合わせる。跡を合わせたコミットに入れて送らない。
func TestSyncRecoversFromInterruptedSave(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	a.write(t, "a", set("title", "", "A"))
	a.sync(t, remote)
	b.write(t, "b", set("rank", "a", "b"))
	if err := os.WriteFile(markerPath(b.dir), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(b.dir, "items", strayItem)
	if err := os.MkdirAll(stray, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stray, "log.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	b.sync(t, remote)
	if _, err := os.Stat(markerPath(b.dir)); !os.IsNotExist(err) {
		t.Errorf("marker left: %v", err)
	}
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Errorf("stray folder left: %v", err)
	}
	if st := worktreeStatus(t, b.dir); !st.IsClean() {
		t.Errorf("worktree is not clean:\n%s", st)
	}
	a.sync(t, remote)
	if _, ok := files(t, a.dir)["items/"+strayItem+"/log.jsonl"]; ok {
		t.Error("the stray file was sent")
	}
}

// 合わせる途中で失敗したら印を残し、次の操作がいまのコミットに戻す。
func TestSyncFailureWhileChangingFilesLeavesMarker(t *testing.T) {
	s, id, dir := newStore(t)
	boom := errors.New("boom")
	err := syncRun{root: dir}.guarded(func() error {
		if err := os.WriteFile(filepath.Join(dir, "half.yaml"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(markerPath(dir)); err != nil {
		t.Fatalf("marker: %v", err)
	}
	if err := s.Read(t.Context(), id, func(port.Reader) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if st := worktreeStatus(t, dir); !st.IsClean() {
		t.Errorf("worktree is not clean:\n%s", st)
	}
}
