package gitstore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/member"
)

// removeAndCommit は、hedgehug を通さずに、Store のフォルダを消したコミットを作る(利用者が git で消した代わり)。
func removeAndCommit(t *testing.T, dir, rel string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(dir, rel)); err != nil {
		t.Fatal(err)
	}
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		t.Fatal(err)
	}
	sig := &object.Signature{Name: "someone", Email: "someone@example.invalid", When: t0}
	if _, err := wt.Commit("remove "+rel, &git.CommitOptions{Author: sig, Committer: sig}); err != nil {
		t.Fatal(err)
	}
}

// noTask は、サンプルのタスクのフォルダがないことを確かめる。
func noTask(t *testing.T, m machine) {
	t.Helper()
	for p := range files(t, m.dir) {
		if strings.HasPrefix(p, "items/"+string(apptest.SampleTask)+"/") {
			t.Errorf("%s: %s is left", m.dir, p)
		}
	}
}

// 一方で消したファイルは、受け取るだけ(進める)のときも、合わせるときも、両方から消える。
func TestSyncDeletedFilesConverge(t *testing.T) {
	task := "items/" + string(apptest.SampleTask)
	t.Run("fast-forward", func(t *testing.T) {
		remote := newRemote(t)
		a := firstMachine(t, remote)
		b := cloneMachine(t, remote, "B", t0)
		removeAndCommit(t, a.dir, task)
		a.sync(t, remote)
		b.sync(t, remote)
		noTask(t, b)
		sameFiles(t, a, b)
	})
	t.Run("merge", func(t *testing.T) {
		remote := newRemote(t)
		a := firstMachine(t, remote)
		b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
		removeAndCommit(t, a.dir, task)
		b.write(t, "b", func(tx port.Tx) error { return tx.WriteMembers().Append(member.Join("jiro")) })
		a.sync(t, remote)
		b.sync(t, remote)
		a.sync(t, remote)
		noTask(t, a)
		noTask(t, b)
		sameFiles(t, a, b)
	})
}

// remoteDir は、テストの remote の bare のフォルダ。
func remoteDir(t *testing.T, remote port.Remote) string {
	t.Helper()
	remotesMu.Lock()
	defer remotesMu.Unlock()
	return remotes[strings.TrimPrefix(remote.URL, "https://remote.invalid")]
}

// remote で main が消えたら、前に受け取った main の古い先端と合わせず、断る(決定 4)。
func TestSyncRefusesWhenRemoteBranchWasDeleted(t *testing.T) {
	remote := newRemote(t)
	firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0)
	b.sync(t, remote) // b は remote の main を受け取っている
	bare, err := git.PlainOpen(remoteDir(t, remote))
	if err != nil {
		t.Fatal(err)
	}
	main := plumbing.NewBranchReferenceName("main")
	other := plumbing.NewBranchReferenceName("other")
	head, err := bare.Reference(main, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := bare.Storer.SetReference(plumbing.NewHashReference(other, head.Hash())); err != nil {
		t.Fatal(err)
	}
	if err := bare.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, other)); err != nil {
		t.Fatal(err)
	}
	if err := bare.Storer.RemoveReference(main); err != nil {
		t.Fatal(err)
	}
	b.write(t, "b", set("title", "", "B"))
	before := snap(t, b)
	if _, err := b.rep.Sync(t.Context(), b.id, remote); !errors.Is(err, port.ErrNoCommonHistory) {
		t.Errorf("err = %v", err)
	}
	unchanged(t, b, before)
	if _, err := bare.Reference(main, true); !errors.Is(err, plumbing.ErrReferenceNotFound) {
		t.Errorf("main was sent again: %v", err)
	}
}

// committedTree は、先端のコミットのファイル。
func committedTree(t *testing.T, dir string) tree {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := readTree(repo, headCommit(t, dir).Hash)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// keptOnlyLocally は、stray のファイルが m の作業ツリーに残り、コミットにも、送った先の other にもないことを確かめる。
func keptOnlyLocally(t *testing.T, m, other machine, stray []string) {
	t.Helper()
	committed, onOther := committedTree(t, m.dir), files(t, other.dir)
	for _, p := range stray {
		if _, ok := committed[p]; ok {
			t.Errorf("%s was committed", p)
		}
		if _, ok := onOther[p]; ok {
			t.Errorf("%s was sent", p)
		}
		if _, err := os.Stat(filepath.Join(m.dir, p)); err != nil {
			t.Errorf("%s on %s: %v", p, m.dir, err)
		}
	}
	if len(committed) != len(onOther) {
		t.Errorf("commit has %d files, the other has %d", len(committed), len(onOther))
	}
}

// 作業ツリーにある、Store のものでないファイル(OS やエディタが作るもの)は、合わせたコミットに入れず、送らない。
// 受け取るだけ(進める)ときも消さない。
func TestSyncMergeLeavesUntrackedFilesOut(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	a.write(t, "a", set("title", "", "A"))
	a.sync(t, remote)
	b.write(t, "b", set("rank", "a", "b"))
	stray := []string{".DS_Store", "items/" + string(apptest.SampleTask) + "/.DS_Store"}
	for _, p := range stray {
		if err := os.WriteFile(filepath.Join(b.dir, p), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if res := b.sync(t, remote); res != (port.SyncResult{Sent: 1, Received: 1}) {
		t.Errorf("b sync = %+v", res)
	}
	a.sync(t, remote)
	keptOnlyLocally(t, b, a, stray)
	a.write(t, "a2", set("title", "A", "A2"))
	a.sync(t, remote)
	if res := b.sync(t, remote); res != (port.SyncResult{Received: 1}) {
		t.Errorf("fast-forward = %+v", res)
	}
	keptOnlyLocally(t, b, a, stray)
	if got := title(t, b); got != "A2" {
		t.Errorf("title = %q", got)
	}
}
