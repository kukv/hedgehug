package gitstore

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"github.com/go-git/go-git/v5/storage/filesystem"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// テストの remote は、https://remote.invalid/<名前> を、プロセスの中の git サーバーで、一時フォルダの
// 空のリポジトリ(bare)につなぐ。git の実行ファイルを使わない(CI の test-without-git でも動く)。
// 同期のテストは、ほかのテストと並べて走らせない。

var (
	remotesMu sync.Mutex
	remotes   = map[string]string{} // URL のパス → bare のフォルダ
	installed sync.Once
)

type remoteLoader struct{}

func (remoteLoader) Load(ep *transport.Endpoint) (storer.Storer, error) {
	remotesMu.Lock()
	dir, ok := remotes[ep.Path]
	remotesMu.Unlock()
	if !ok {
		return nil, transport.ErrRepositoryNotFound
	}
	return filesystem.NewStorage(osfs.New(dir), cache.NewObjectLRUDefault()), nil
}

// serverOf は、go-git の git サーバーが、受け取る側の知らない「持っているコミット」(have)で失敗しないようにする。
// go-git のクライアントは、手元のブランチのコミットをすべて have として送る。git や GitHub は知らない have を
// 読み飛ばすが、go-git のサーバーは object not found で失敗するので、サーバーが持つものだけを渡す。
type serverOf struct{ transport.Transport }

func (s serverOf) NewUploadPackSession(ep *transport.Endpoint, auth transport.AuthMethod) (transport.UploadPackSession, error) {
	sess, err := s.Transport.NewUploadPackSession(ep, auth)
	if err != nil {
		return nil, err
	}
	st, err := remoteLoader{}.Load(ep)
	if err != nil {
		return nil, err
	}
	return knownHaves{UploadPackSession: sess, st: st}, nil
}

type knownHaves struct {
	transport.UploadPackSession
	st storer.Storer
}

func (k knownHaves) UploadPack(ctx context.Context, req *packp.UploadPackRequest) (*packp.UploadPackResponse, error) {
	var haves []plumbing.Hash
	for _, h := range req.Haves {
		if k.st.HasEncodedObject(h) == nil {
			haves = append(haves, h)
		}
	}
	req.Haves = haves
	return k.UploadPackSession.UploadPack(ctx, req)
}

func newRemote(t *testing.T) port.Remote {
	t.Helper()
	installed.Do(func() { client.InstallProtocol("https", serverOf{server.NewClient(remoteLoader{})}) })
	dir := t.TempDir()
	if _, err := initMain(dir, true); err != nil {
		t.Fatal(err)
	}
	p := "/" + filepath.Base(dir) + ".git"
	remotesMu.Lock()
	remotes[p] = dir
	remotesMu.Unlock()
	return port.Remote{URL: "https://remote.invalid" + p}
}

// initMain は、ブランチ main で始まるリポジトリを作る(go-git の既定は master。GitHub と Create は main)。
func initMain(dir string, bare bool) (*git.Repository, error) {
	return git.PlainInitWithOptions(dir, &git.PlainInitOptions{
		Bare:        bare,
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
}

// letterIDs は、マシンごとに重ならない ID を順に返す。apptest.IDs は 2 台で同じ ID を返すので使わない。
type letterIDs struct {
	mu     sync.Mutex
	letter string
	n      int
}

func (g *letterIDs) New() change.ID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return change.ID(fmt.Sprintf("01%s%023d", g.letter, g.n))
}

// machine は、1 台のマシンの Store。
type machine struct {
	stores *Stores
	rep    *Replicator
	id     port.StoreID
	dir    string
	clock  *apptest.Clock
}

func newMachine(t *testing.T, dir, letter string, start time.Time) machine {
	t.Helper()
	clock := apptest.NewClock(start)
	s := New(clock, &letterIDs{letter: letter})
	id, err := s.Attach(dir)
	if err != nil {
		t.Fatal(err)
	}
	return machine{stores: s, rep: NewReplicator(s), id: id, dir: dir, clock: clock}
}

// firstMachine は、Store を作り、サンプルデータを保存して、空の remote へ最初に送ったマシン。
func firstMachine(t *testing.T, remote port.Remote) machine {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "a")
	if err := Create(dir, "01STORE0000000000000000001", t0); err != nil {
		t.Fatal(err)
	}
	m := newMachine(t, dir, "A", t0)
	if err := apptest.Sample(t.Context(), m.stores, m.id); err != nil {
		t.Fatal(err)
	}
	if _, err := m.rep.Sync(t.Context(), m.id, remote); err != nil {
		t.Fatal(err)
	}
	return m
}

// cloneMachine は、remote を写して取り付けたマシン(Store の追加の仕組みはトラック G が作る)。
func cloneMachine(t *testing.T, remote port.Remote, letter string, start time.Time) machine {
	t.Helper()
	dir := filepath.Join(t.TempDir(), letter)
	if _, err := git.PlainClone(dir, false, &git.CloneOptions{URL: remote.URL}); err != nil {
		t.Fatal(err)
	}
	return newMachine(t, dir, letter, start)
}

func (m machine) sync(t *testing.T, remote port.Remote) port.SyncResult {
	t.Helper()
	res, err := m.rep.Sync(t.Context(), m.id, remote)
	if err != nil {
		t.Fatalf("sync of %s: %v", m.dir, err)
	}
	return res
}

func (m machine) write(t *testing.T, summary string, fn func(tx port.Tx) error) {
	t.Helper()
	if err := m.stores.Write(t.Context(), m.id, port.Operation{Actor: change.User("taro"), Summary: summary}, fn); err != nil {
		t.Fatal(err)
	}
}

// files は、作業ツリーのファイル(.git を除く)を読む。
func files(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// snapshot は、Store のファイルと先端のコミット。断る同期の前後で比べる。
type snapshot struct {
	files map[string]string
	head  plumbing.Hash
}

func snap(t *testing.T, m machine) snapshot {
	t.Helper()
	return snapshot{files: files(t, m.dir), head: headCommit(t, m.dir).Hash}
}

// unchanged は、断った同期が Store を変えなかったこと(ファイル・先端・作業ツリー)を確かめる。
func unchanged(t *testing.T, m machine, before snapshot) {
	t.Helper()
	after := snap(t, m)
	if after.head != before.head {
		t.Errorf("head moved: %s -> %s", before.head, after.head)
	}
	if len(after.files) != len(before.files) {
		t.Errorf("files: %d -> %d", len(before.files), len(after.files))
	}
	for p, v := range before.files {
		if after.files[p] != v {
			t.Errorf("%s changed", p)
		}
	}
	if st := worktreeStatus(t, m.dir); !st.IsClean() {
		t.Errorf("worktree is not clean:\n%s", st)
	}
}

// sameFiles は、2 台の Store のファイルがバイト単位で同じで、先端のコミットも同じで、どちらの作業ツリーも
// コミットと同じことを確かめる(保存形式「テスト」)。
func sameFiles(t *testing.T, a, b machine) {
	t.Helper()
	if ha, hb := headCommit(t, a.dir).Hash, headCommit(t, b.dir).Hash; ha != hb {
		t.Errorf("heads differ: a %s, b %s", ha, hb)
	}
	for _, m := range []machine{a, b} {
		if st := worktreeStatus(t, m.dir); !st.IsClean() {
			t.Errorf("%s: worktree is not clean:\n%s", m.dir, st)
		}
	}
	fa, fb := files(t, a.dir), files(t, b.dir)
	for p, v := range fa {
		if fb[p] != v {
			t.Errorf("%s differs:\n--- a\n%s\n--- b\n%s", p, v, fb[p])
		}
	}
	for p := range fb {
		if _, ok := fa[p]; !ok {
			t.Errorf("%s only on b", p)
		}
	}
}
