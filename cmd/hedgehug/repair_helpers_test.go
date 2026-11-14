//go:build !e2e

package main

import (
	"context"
	"errors"
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
	"github.com/kukv/hedgehug/internal/app/noticing"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/repairing"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/infra/gitstore"
	"github.com/kukv/hedgehug/internal/infra/statedb"
)

// 場面テストの remote は、一時フォルダの bare リポジトリをパスで渡す(決定 C3a-20)。go-git のファイルの通信は
// git の実行ファイル(git-upload-pack など)を呼ぶので、CI の test-without-git でも動くように、パスの通信を
// プロセスの中の git サーバーに差し替える。

var fileTransport sync.Once

type pathLoader struct{}

func (pathLoader) Load(ep *transport.Endpoint) (storer.Storer, error) {
	if _, err := os.Stat(ep.Path); err != nil {
		return nil, transport.ErrRepositoryNotFound
	}
	return filesystem.NewStorage(osfs.New(ep.Path), cache.NewObjectLRUDefault()), nil
}

// pathServer は、go-git の git サーバーが、受け取る側の知らない「持っているコミット」(have)で失敗しないようにする
// (gitstore の同期のテストと同じ)。go-git のクライアントは手元のコミットをすべて have として送るが、go-git の
// サーバーは知らない have で object not found になるので、サーバーが持つものだけを渡す。
type pathServer struct{ transport.Transport }

func (s pathServer) NewUploadPackSession(ep *transport.Endpoint, auth transport.AuthMethod) (transport.UploadPackSession, error) {
	sess, err := s.Transport.NewUploadPackSession(ep, auth)
	if err != nil {
		return nil, err
	}
	st, err := pathLoader{}.Load(ep)
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

// newBareRemote は、空の bare リポジトリ(ブランチ main)を作り、そのパスを remote にする。
func newBareRemote(t *testing.T) port.Remote {
	t.Helper()
	fileTransport.Do(func() { client.InstallProtocol("file", pathServer{server.NewClient(pathLoader{})}) })
	dir := t.TempDir()
	_, err := git.PlainInitWithOptions(dir, &git.PlainInitOptions{
		Bare:        true,
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	if err != nil {
		t.Fatal(err)
	}
	return port.Remote{URL: dir}
}

var (
	scenarioStart = time.Date(2026, 11, 14, 9, 0, 0, 0, time.FixedZone("JST", 9*3600))
	scenarioUser  = change.User("taro")
)

// sampleProject2 は、最初のマシンがサンプルと一緒に作る、同じワークスペースのプロジェクト HH-P2。
const sampleProject2 project.ID = "01SAMPLEPR0000000000000002"

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

// machine は、1 台のマシンの Store。同期は本物の gitstore で、修復は repairing.Run。
// inbox は、noticed で通知の判定を付けたマシンの受信箱(付けていなければ nil)。
type machine struct {
	stores *gitstore.Stores
	rep    *gitstore.Replicator
	id     port.StoreID
	dir    string
	inbox  port.InboxBook
}

func newMachine(t *testing.T, dir, letter string, start time.Time) machine {
	t.Helper()
	s := gitstore.New(apptest.NewClock(start), &letterIDs{letter: letter})
	id, err := s.Attach(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep := gitstore.NewReplicator(s)
	rep.Repair = repairing.Run
	return machine{stores: s, rep: rep, id: id, dir: dir}
}

// firstMachine は、Store を作り、サンプルデータとプロジェクト HH-P2 を保存して、空の remote へ最初に送ったマシン。
func firstMachine(t *testing.T, remote port.Remote) machine {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "a")
	if err := gitstore.Create(dir, "01STORE0000000000000000001", scenarioStart); err != nil {
		t.Fatal(err)
	}
	m := newMachine(t, dir, "A", scenarioStart)
	if err := apptest.Sample(t.Context(), m.stores, m.id); err != nil {
		t.Fatal(err)
	}
	p2 := project.Project{ID: sampleProject2, Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, Rank: "W", Title: "管理", Owner: "taro", Status: project.Todo}
	m.write(t, "プロジェクトを作成", func(tx port.Tx) error { return tx.WriteProjects().Append(p2.ID, project.Create(p2)) })
	m.sync(t, remote)
	return m
}

// cloneMachine は、remote を写して取り付けたマシン。
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
	m.writeAs(t, scenarioUser.By(), summary, fn)
}

// writeAs は、ログイン名 login の操作として保存する。
func (m machine) writeAs(t *testing.T, login, summary string, fn func(tx port.Tx) error) {
	t.Helper()
	if err := m.stores.Write(t.Context(), m.id, port.Operation{Actor: change.User(login), Summary: summary}, fn); err != nil {
		t.Fatal(err)
	}
}

// noticed は、m の同期に、テストの一時フォルダの state.db の受信箱と判定の記録と、login を「あなた」にした判定
// (Store を表示中で書き込める)を付ける。本物の組み立て(wire.go)と同じ形。
func noticed(t *testing.T, m machine, login string) machine {
	t.Helper()
	db, err := statedb.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	m.inbox = db.Inbox(apptest.NewClock(scenarioStart))
	svc := noticing.Service{
		Inbox:    m.inbox,
		Identity: apptest.Identity{Login: login},
		List:     apptest.StoreList{Stores: []port.StoreInfo{{ID: m.id, Name: "main"}}},
	}
	m.rep.Judge, m.rep.Judged = svc.Judge, db.JudgedSaves()
	return m
}

// notices は、受信箱の kinds(空はすべて)の行を新しい順に返す。
func (m machine) notices(t *testing.T, kinds ...port.NoticeKind) []port.NoticeRow {
	t.Helper()
	rows, err := m.inbox.List(t.Context(), port.NoticeQuery{Stores: []port.StoreID{m.id}, Kinds: kinds})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func (m machine) item(t *testing.T, id item.ID) item.Item {
	t.Helper()
	var it item.Item
	err := m.stores.Read(t.Context(), m.id, func(r port.Reader) error {
		var err error
		it, err = r.Items().Get(id)
		return err
	})
	if err != nil {
		t.Fatalf("item %s on %s: %v", id, m.dir, err)
	}
	return it
}

// items は、プロジェクトのすべてのアイテム(削除済みを含む、番号の順)。
func (m machine) items(t *testing.T, p project.ID) []item.Item {
	t.Helper()
	var its []item.Item
	err := m.stores.Read(t.Context(), m.id, func(r port.Reader) error {
		var err error
		its, err = r.Items().List(p)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return its
}

// head は、リポジトリ(作業ツリーのあるものでも bare でも)の先端のコミット。
func head(t *testing.T, dir string) plumbing.Hash {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := repo.Reference(plumbing.NewBranchReferenceName("main"), true)
	if err != nil {
		t.Fatal(err)
	}
	return ref.Hash()
}

func cleanWorktree(t *testing.T, dir string) {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	st, err := wt.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsClean() {
		t.Errorf("%s: worktree is not clean:\n%s", dir, st)
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
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// sameFiles は、2 台の Store のファイルがバイト単位で同じで、先端のコミットも同じで、どちらの作業ツリーも
// コミットと同じことを確かめる。
func sameFiles(t *testing.T, a, b machine) {
	t.Helper()
	if ha, hb := head(t, a.dir), head(t, b.dir); ha != hb {
		t.Errorf("heads differ: a %s, b %s", ha, hb)
	}
	cleanWorktree(t, a.dir)
	cleanWorktree(t, b.dir)
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

// newTask は、生の行で、HH-1 の下に番号 3 の Task を作る(番号の行も書く)。手元の HH-1 のプロジェクトに作る。
func newTask(t *testing.T, m machine, id item.ID) {
	t.Helper()
	const n = 3
	parent := apptest.SampleEpic
	p := m.item(t, parent).Place
	it := item.Item{
		ID:    id,
		Place: item.Place{Workspace: p.Workspace, Project: p.Project, Parent: &parent, Number: n},
		Kind:  item.Task, Rank: rank.After(rank.First), Title: "新しい Task",
		Status: apptest.SampleStatusTodo, Priority: apptest.SamplePriorityMedium,
	}
	m.write(t, "アイテムを作成", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(p.Workspace, numbering.Draft(numbering.Items, n)),
			tx.WriteItems().Append(id, item.Create(it)),
		)
	})
}
