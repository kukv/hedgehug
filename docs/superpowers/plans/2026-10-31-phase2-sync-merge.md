# フェーズ 2: 同期とマージ 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 複数のクローンの Store を bare remote 経由で同期し、フィールド単位の3-wayマージ、マージ後の修復（ID の衝突の振り直しを含む）、上書きの記録と inbox、merge driver、undo / restore --at、バックグラウンド同期を作り、プロパティテストで収束と値の保存を確かめる。

**Architecture:** `gitrepo` に通信（インプロセスの file transport）とマージの材料（merge-base、差分、履歴）を足す。`merge` は git を知らない純粋な関数で、1ファイルの3-wayマージ（`Merge`）とファイル集合の修復（`Repair`）を持つ。`sync` が fetch → merge → repair → `store.CommitMerge` → push を回し、`inbox` が届いたコミットから通知を作る。CLI は `app.update` の後にバックグラウンド同期を起こす。

**Tech Stack:** Go 1.27、go-git v5.19（`plumbing/transport/server`、`plumbing/transport/client`）、`github.com/sergi/go-diff`（go-git の間接依存を直接の依存にする）、go.yaml.in/yaml/v4、cobra

**Spec:** `docs/superpowers/specs/2026-10-31-phase2-sync-merge-design.md`（必ず先に全部読む）。前段は `docs/superpowers/specs/2026-10-31-phase1-5-workspace-keys-design.md`。親設計は https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc

## Global Constraints

- 呼び方: 人が読める識別子は ID（`id:`、`model.Task.ID`）、内部は ULID（`ulid:`、`model.Task.ULID`）、`former_ids`、ワークスペースの `id_prefix`。短縮 ULID はどこにも使わない
- 表示はタスクを「ID (ULID) タイトル」で示す。入力の案内は ID だけ
- trailer: `Hedgehug-Op`（足す値: `merge`・`repair`・`setup`・`undo`・`revert-overwritten`）、`Hedgehug-Task`、`Hedgehug-Workspace`、`Hedgehug-Author`、`Hedgehug-Overwritten`、`Hedgehug-Repaired`、`Hedgehug-Undo`
- コミットの author: accounts のある Store は `login <login@users.noreply.<host>>`、ない Store は git の設定（`config.Author`）
- マージの結果は対称（ours と theirs を入れ替えても同じバイト列と同じ上書きの集合）で、修復は決定的
- Hedgehug 自身の同期は git の実行ファイルを呼ばない（go-git だけ）。git を呼ぶテストは `exec.LookPath("git")` が失敗したら skip
- テストでは `HEDGEHUG_AUTO_SYNC=0` を既定にし、子プロセスの起動はパッケージ変数の差し替えで止める（テストのバイナリを起動しない）
- エラーメッセージとコードのコメントは英語で、周りのコードの密度と書き方に合わせる
- 足すコマンドはすべて `--json` に対応する（隠しコマンドを除く）
- 各タスクの最後に `go test ./...`、`go vet ./...`、`make lint` が通ること
- コミットメッセージは日本語の Conventional Commits（例 `feat(sync): ...`）で、末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`
- 範囲内のレビュー指摘は残さない（リポジトリの CLAUDE.md）。直すか、コントローラーに一覧で上げる

---

## ファイル構成

```
internal/gitrepo/
  transport.go      新規: file スキームのインプロセス transport
  remote.go         新規: SetRemotes、Fetch、Push、Clone、ErrRejected、RemoteHead
  merge.go          新規: MergeBase、IsAncestor、LastChange、FastForward、Head
  maintain.go       新規: LooseObjects、Repack
  lock.go           新規: .git/index.lock（ErrGitBusy）
  commit.go         Commit.Parents、index.lock を取る
  log.go            LogFrom（任意のコミットから first-parent をたどる）
internal/config/
  config.go         StoreEntry に Remotes・Account、Accounts、Sync、SetRemotes、RemoveStore、Writer
  author.go         CommitAuthor（アカウントか git の設定）
internal/model/
  taskid.go         CountIDs、RaiseNumbers（store/ids.go から移す）
internal/store/
  ids.go            model の CountIDs を使う
  merge.go          新規: CommitMerge、ErrHeadMoved
  rank.go           新規: 書き込み時の同じ rank の振り直し
  tx.go             Op.Account（Hedgehug-Author）、rank の振り直しの呼び出し
  validate.go       validateWorkspaces を変えたフィールドに限る
internal/index/
  index.go          sortByRank を rank・ID・ULID に
  comments.go       新規: コメント・作業時間のインデックス
  refs.go           新規: メンション、#ID、Backlinks
internal/query/
  eval.go           byRank、@me、mentions:、watching:、is:overwritten
internal/merge/
  merge.go          新規: Side、Result、Policy、Merge、種類の振り分け、LWW の比較
  task.go           新規: タスク
  record.go         新規: コメント・作業時間（ファイル全体の LWW）
  plan.go           新規: スプリント・リリース・テンプレート・ワークスペース
  yaml.go           新規: schema.yaml / automations.yaml / store.yaml、numbers.yaml
  text.go           新規: diff3
  other.go          新規: その他のファイル
  repair.go         新規: Repair
internal/sync/
  history.go        新規: Side.When と Side.Author を履歴から
  sync.go           新規: Run、Result、ErrDirty
  state.go          新規: sync.json
  background.go     新規: 起動、待ち合わせ、失敗の知らせ
  spawn_unix.go spawn_windows.go
internal/inbox/
  inbox.go          新規: Item、Store（inbox.jsonl / inbox-read.json）、Collect
internal/doctor/
  doctor.go         merge driver、overwritten の記録、--fix
cmd/hedgehug/
  sync.go store_add.go inbox.go watch.go undo.go merge_driver.go   新規
  app.go            update で author・pending・バックグラウンド同期、開始時の失敗の知らせ
  store_init.go restore.go(rm.go) doctor.go show.go add.go comment.go assign.go set.go
tools/devenv/main.go  bare remote、accounts、共有 Store
.github/workflows/ci.yaml  git なしのジョブ
```

---

### Task 1: gitrepo — インプロセスの file transport と remote の操作

**Files:**
- Create: `internal/gitrepo/transport.go`, `internal/gitrepo/remote.go`, `internal/gitrepo/remote_test.go`
- Modify: `go.mod`（変えない。transport/server は go-git に含まれる）

**Interfaces:**
- Produces:
  - `var ErrRejected = errors.New("the remote rejected the push: it has commits this clone does not")`
  - `func InitBare(dir string) error` — テストと devenv 用の bare リポジトリ（既定のブランチ main）
  - `func (r *Repo) SetRemotes(urls []string) error` — `origin`、`mirror1`… として `.git/config` に書き、余分な remote を消す
  - `func (r *Repo) Remotes() []string` — 名前を `origin`、`mirror1`… の順で
  - `func (r *Repo) Fetch(remote string) (plumbing.Hash, error)` — `refs/remotes/<remote>/main` を返す。remote が空ならゼロハッシュ
  - `func (r *Repo) Push(remote string) error` — fast-forward でなければ `ErrRejected`。送るものがなければ nil
  - `func Clone(url, dir string) (*Repo, error)`
  - `func (r *Repo) RemoteHead(remote string) (plumbing.Hash, error)` — 直近の fetch の結果（ゼロなら未取得か空）

- [ ] **Step 1: テストを書く**（`remote_test.go`）

```go
package gitrepo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// noGit empties PATH so that a call to the git binary fails the test.
func noGit(t *testing.T) { t.Setenv("PATH", "") }

func newBare(t *testing.T) string {
	t.Helper()
	d := filepath.Join(t.TempDir(), "remote.git")
	if err := InitBare(d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestPushFetchCloneWithoutGit(t *testing.T) {
	noGit(t)
	remote := newBare(t)
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	h := commitFiles(t, r, dir, map[string]string{"a.md": "a\n"})
	if err := r.SetRemotes([]string{remote}); err != nil {
		t.Fatal(err)
	}
	if err := r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "clone")
	c, err := Clone(remote, other)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := c.Head(); got != h {
		t.Errorf("clone HEAD = %s, want %s", got, h)
	}
	if b, err := os.ReadFile(filepath.Join(other, "a.md")); err != nil || string(b) != "a\n" {
		t.Errorf("work tree a.md = %q, %v", b, err)
	}
	h2 := commitFiles(t, c, other, map[string]string{"b.md": "b\n"})
	if err := c.Push("origin"); err != nil {
		t.Fatal(err)
	}
	got, err := r.Fetch("origin")
	if err != nil || got != h2 {
		t.Errorf("Fetch = %s, %v; want %s", got, err, h2)
	}
}

func TestPushRejected(t *testing.T) {
	remote := newBare(t)
	a, b := cloneTwo(t, remote) // both clones of one pushed commit
	commitFiles(t, a.r, a.dir, map[string]string{"x.md": "a\n"})
	commitFiles(t, b.r, b.dir, map[string]string{"y.md": "b\n"})
	if err := a.r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	if err := b.r.Push("origin"); !errors.Is(err, ErrRejected) {
		t.Errorf("second push = %v, want ErrRejected", err)
	}
}

func TestPushNothing(t *testing.T) {
	remote := newBare(t)
	a, _ := cloneTwo(t, remote)
	if err := a.r.Push("origin"); err != nil {
		t.Errorf("push with nothing new = %v", err)
	}
}

func TestFetchEmptyRemote(t *testing.T) {
	remote := newBare(t)
	dir := t.TempDir()
	r, _ := Init(dir)
	if err := r.SetRemotes([]string{remote}); err != nil {
		t.Fatal(err)
	}
	if h, err := r.Fetch("origin"); err != nil || !h.IsZero() {
		t.Errorf("Fetch of an empty remote = %s, %v", h, err)
	}
}

func TestSetRemotes(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	if err := r.SetRemotes([]string{"/a.git", "/b.git", "/c.git"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetRemotes([]string{"/a.git", "/c.git"}); err != nil {
		t.Fatal(err)
	}
	if got := r.Remotes(); !slices.Equal(got, []string{"origin", "mirror1"}) {
		t.Errorf("Remotes = %v", got)
	}
	cfg, _ := r.r.Config()
	if u := cfg.Remotes["mirror1"].URLs[0]; u != "/c.git" {
		t.Errorf("mirror1 = %s", u)
	}
}

func TestMissingRemote(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	_ = r.SetRemotes([]string{filepath.Join(t.TempDir(), "nope.git")})
	if _, err := r.Fetch("origin"); err == nil {
		t.Error("Fetch of a missing remote succeeded")
	}
}
```

`cloneTwo` は `repo_test.go` に足す補助: bare に1コミットを push し、2つのクローン（`struct{ r *Repo; dir string }`）を返す。`Head()` は Task 2 の関数だが、ここで先に足す（`HEAD` の参照を引いてハッシュを返し、コミットがなければゼロハッシュ）。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/gitrepo/ -run 'Push|Fetch|Clone|SetRemotes|MissingRemote'`
Expected: FAIL（`InitBare` などが未定義）

- [ ] **Step 3: transport.go を書く**

```go
package gitrepo

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// go-git's file transport runs git-upload-pack and git-receive-pack. Hedgehug
// must not need git, so the file scheme is served in-process instead.
func init() {
	client.InstallProtocol("file", server.NewClient(fileLoader{}))
}

// driveRE matches /C:/... that a file URL of a Windows path turns into.
var driveRE = regexp.MustCompile(`^/[A-Za-z]:[/\\]`)

type fileLoader struct{}

// Load opens the bare repository at the endpoint's path.
func (fileLoader) Load(ep *transport.Endpoint) (storer.Storer, error) {
	p := ep.Path
	if driveRE.MatchString(p) {
		p = p[1:]
	}
	p = filepath.FromSlash(p)
	if !isDir(filepath.Join(p, "objects")) {
		return nil, transport.ErrRepositoryNotFound
	}
	return filesystem.NewStorage(osfs.New(p), cache.NewObjectLRUDefault()), nil
}
```

`isDir` は `os.Stat` で dir かを返す小さな関数。`go-billy` は go-git の依存なので go.mod の `require` に直接の依存として上がる（`go mod tidy` で確かめる）。

- [ ] **Step 4: remote.go を書く**

- `InitBare`: `git.PlainInitWithOptions(dir, &git.PlainInitOptions{Bare: true, InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")}})`
- `SetRemotes`: `r.r.Config()` を読み、`remoteName(i)`（0 は `origin`、i は `mirror<i>`）で `config.RemoteConfig{Name, URLs: []string{u}, Fetch: []config.RefSpec{"+refs/heads/main:refs/remotes/<name>/main"}}` を入れ、`origin`/`mirror*` のうち余ったものを消して `r.r.SetConfig(cfg)`。他の名前の remote（利用者が手で足したもの）は残す
- `Fetch`: `r.r.Fetch(&git.FetchOptions{RemoteName: remote})`。`git.NoErrAlreadyUpToDate` と `transport.ErrEmptyRemoteRepository` は成功扱い。その後 `RemoteHead(remote)`
- `RemoteHead`: `refs/remotes/<remote>/main` の参照。なければゼロハッシュ
- `Push`: `r.r.Push(&git.PushOptions{RemoteName: remote, RefSpecs: []config.RefSpec{"refs/heads/main:refs/heads/main"}})`。`NoErrAlreadyUpToDate` は nil、`git.ErrNonFastForwardUpdate` か、メッセージに `non-fast-forward` を含む `*packp.ReportStatus` のエラーは `ErrRejected`。成功したら `refs/remotes/<remote>/main` を HEAD に更新する（次の `RemoteHead` が正しいように）
- `Clone`: `git.PlainClone(dir, false, &git.CloneOptions{URL: url, RemoteName: "origin", ReferenceName: "refs/heads/main", SingleBranch: true})`。fetch の refspec を `SetRemotes` と同じ形にそろえる

- [ ] **Step 5: テストを通す**

Run: `go test ./internal/gitrepo/ && go mod tidy && git diff --stat go.mod`
Expected: PASS。go.mod に `github.com/go-git/go-billy/v5` が直接の依存として入る

- [ ] **Step 6: コミット**

```bash
git add internal/gitrepo go.mod go.sum
git commit -m "feat(gitrepo): インプロセスの file transport と fetch・push・clone を足す"
```

---

### Task 2: gitrepo — マージの材料、index.lock、repack

**Files:**
- Create: `internal/gitrepo/merge.go`, `internal/gitrepo/lock.go`, `internal/gitrepo/maintain.go`, `internal/gitrepo/merge_test.go`, `internal/gitrepo/lock_test.go`
- Modify: `internal/gitrepo/commit.go`（`Parents`、ロック）, `internal/gitrepo/log.go`（`LogFrom`）

**Interfaces:**
- Consumes: Task 1 の `Head`
- Produces:
  - `func (r *Repo) Head() (plumbing.Hash, error)` — ゼロハッシュはコミットなし
  - `func (r *Repo) MergeBase(a, b plumbing.Hash) (plumbing.Hash, error)` — 共通の祖先がなければゼロハッシュ
  - `func (r *Repo) IsAncestor(a, b plumbing.Hash) (bool, error)` — a が b の祖先（a == b を含む）
  - `func (r *Repo) TreeOf(commit plumbing.Hash) (plumbing.Hash, error)` — ゼロのコミットはゼロのツリー
  - `func (r *Repo) ReadAt(commit plumbing.Hash, path string) ([]byte, error)` — ファイルがなければ `fs.ErrNotExist`
  - `func (r *Repo) LastChange(from, base plumbing.Hash, path string) (LogEntry, bool, error)` — from から base までの間（base を含まない）で path を最後に変えたコミット。マージコミットは、どの親とも内容が違うときだけ数える
  - `func (r *Repo) LogFrom(from plumbing.Hash, limit int) ([]LogEntry, error)` — first-parent をたどる
  - `func (r *Repo) Between(from, base plumbing.Hash) ([]LogEntry, error)` — base から届かない from 側のコミットを古い順に（全親をたどる）
  - `Commit.Parents []plumbing.Hash` — 空なら HEAD を親にする（今と同じ）
  - `func (r *Repo) FastForward(h plumbing.Hash, paths []string) error` — `refs/heads/main` を h にし、paths の作業ツリーのファイルと index を h のツリーに合わせる
  - `var ErrGitBusy = errors.New("git is running in the store (.git/index.lock exists); try again when it ends, or remove the file if it is left over")`
  - `func (r *Repo) LooseObjects() (int, error)`、`func (r *Repo) Repack() error`

- [ ] **Step 1: テストを書く**（`merge_test.go`）

```go
func TestMergeBaseAndAncestor(t *testing.T) {
	remote := newBare(t)
	a, b := cloneTwo(t, remote)
	base, _ := a.r.Head()
	ha := commitFiles(t, a.r, a.dir, map[string]string{"x.md": "a\n"})
	hb := commitFiles(t, b.r, b.dir, map[string]string{"y.md": "b\n"})
	_ = a.r.Push("origin")
	theirs, _ := b.r.Fetch("origin")
	if theirs != ha {
		t.Fatalf("fetched %s", theirs)
	}
	mb, err := b.r.MergeBase(hb, theirs)
	if err != nil || mb != base {
		t.Errorf("MergeBase = %s, %v; want %s", mb, err, base)
	}
	if ok, _ := b.r.IsAncestor(base, hb); !ok {
		t.Error("base is not an ancestor of hb")
	}
	if ok, _ := b.r.IsAncestor(hb, theirs); ok {
		t.Error("hb is an ancestor of theirs")
	}
}

func TestCommitWithParentsAndLastChange(t *testing.T) {
	remote := newBare(t)
	a, b := cloneTwo(t, remote)
	base, _ := a.r.Head()
	commitFiles(t, a.r, a.dir, map[string]string{"x.md": "a1\n"})
	ha := commitFiles(t, a.r, a.dir, map[string]string{"y.md": "a2\n"})
	_ = a.r.Push("origin")
	theirs, _ := b.r.Fetch("origin")
	hb := commitFiles(t, b.r, b.dir, map[string]string{"z.md": "b\n"})
	writeFile(t, b.dir, "x.md", "a1\n")
	writeFile(t, b.dir, "y.md", "a2\n")
	m, err := b.r.Commit(Commit{Paths: []string{"x.md", "y.md"}, Subject: "merge", Parents: []plumbing.Hash{hb, theirs}, Author: Signature{"b", "b@x"}, When: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := b.r.r.CommitObject(m)
	if c.NumParents() != 2 {
		t.Errorf("parents = %d", c.NumParents())
	}
	e, ok, err := b.r.LastChange(ha, base, "x.md")
	if err != nil || !ok || e.Subject == "" {
		t.Errorf("LastChange = %+v, %v, %v", e, ok, err)
	}
	if _, ok, _ := b.r.LastChange(hb, base, "x.md"); ok {
		t.Error("x.md was not changed on b's side")
	}
}

func TestFastForward(t *testing.T) {
	remote := newBare(t)
	a, b := cloneTwo(t, remote)
	h := commitFiles(t, a.r, a.dir, map[string]string{"x.md": "a\n"})
	_ = a.r.Push("origin")
	theirs, _ := b.r.Fetch("origin")
	if err := b.r.FastForward(theirs, []string{"x.md"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := b.r.Head(); got != h {
		t.Errorf("HEAD = %s", got)
	}
	if st, _ := b.r.Status(); len(st) != 0 {
		t.Errorf("status after fast-forward = %+v", st)
	}
}

func TestBetween(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	base := commitFiles(t, r, dir, map[string]string{"a.md": "1\n"})
	commitFiles(t, r, dir, map[string]string{"a.md": "2\n"})
	h := commitFiles(t, r, dir, map[string]string{"a.md": "3\n"})
	es, err := r.Between(h, base)
	if err != nil || len(es) != 2 || es[1].Hash != h {
		t.Errorf("Between = %v, %v", es, err)
	}
}
```

`lock_test.go`:

```go
func TestCommitWaitsForIndexLock(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	lock := filepath.Join(dir, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	indexLockWait = 50 * time.Millisecond
	t.Cleanup(func() { indexLockWait = 2 * time.Second })
	writeFile(t, dir, "a.md", "a\n")
	if _, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "s", Author: Signature{"a", "a@x"}, When: time.Now()}); !errors.Is(err, ErrGitBusy) {
		t.Errorf("Commit with index.lock = %v", err)
	}
	_ = os.Remove(lock)
	if _, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "s", Author: Signature{"a", "a@x"}, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lock); !errors.Is(err, fs.ErrNotExist) {
		t.Error("index.lock is left after the commit")
	}
}

func TestRepack(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	for i := range 5 {
		commitFiles(t, r, dir, map[string]string{"a.md": fmt.Sprint(i)})
	}
	n, err := r.LooseObjects()
	if err != nil || n == 0 {
		t.Fatalf("LooseObjects = %d, %v", n, err)
	}
	if err := r.Repack(); err != nil {
		t.Fatal(err)
	}
	if n, _ := r.LooseObjects(); n != 0 {
		t.Errorf("loose objects after repack = %d", n)
	}
	if err := r.Verify(); err != nil {
		t.Errorf("Verify after repack: %v", err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/gitrepo/`
Expected: FAIL（未定義）

- [ ] **Step 3: 実装する**

- `lock.go`: `var indexLockWait = 2 * time.Second`、`const indexLockPoll = 100 * time.Millisecond`。`func (r *Repo) lockIndex() (unlock func(), err error)` は `os.OpenFile(<dir>/.git/index.lock, O_CREATE|O_EXCL|O_WRONLY, 0o600)` を待ち時間まで繰り返し、取れなければ `ErrGitBusy`。unlock はファイルを閉じて消す。go-git は自分では index.lock を作らないので、自分で作ったファイルが go-git の書き込みを止めることはない
- `commit.go`: `Commit` の最初で `lockIndex`、`defer unlock()`。`Parents` が空でなければ `CommitOptions.Parents = c.Parents`（go-git v5 の `CommitOptions.Parents`）。`AllowEmptyCommits` は付けない。マージコミットで paths が HEAD と同じでも、親が2つなら空ではないので、`Parents` が2つ以上のときは `AllowEmptyCommits: true` にする
- `merge.go`: `MergeBase` は `object.Commit.MergeBase`（最初の1つ）。`IsAncestor` は `a.IsAncestor(b)`（a == b は true を返すよう先に比べる）。`LastChange` は from から `object.NewCommitPreorderIter` で全親をたどり、base の祖先（`IsAncestor(c, base)`）に入ったら止める。各コミットで path の blob ハッシュを親と比べる（親がないときは「ある」なら変更）。最初に見つかったもの（新しい順）を返す。走査の順は時刻ではなく preorder なので、見つかった候補のうち `When` が最大のものを返す
- `FastForward`: `lockIndex` を取り、`refs/heads/main` を `storer.SetReference` で h にし、paths の各ファイルを h のツリーから作業ツリーに書く（なければ消す）。書き込みは `os.WriteFile` ではなく、一時ファイルと rename（store の `writeFile` と同じ手順。gitrepo に小さな `atomicWrite` を置く）。最後に `w.AddWithOptions(SkipStatus)` / `unstage` で index を合わせる（`Commit` と同じ）
- `LogFrom`: `Log` の中身を、HEAD でなく from から始める形に切り出す。`Log` は `LogFrom(head, ...)` を呼ぶ
- `Between`: `object.NewCommitPreorderIter(from)` で全親をたどり、base の祖先を除いて集め、`When` と Hash の順で古い順に並べる
- `maintain.go`: `LooseObjects` は `.git/objects/??/*` を数える。`Repack` は `r.r.RepackObjects(&git.RepackConfig{})`

- [ ] **Step 4: テストを通す**

Run: `go test -race ./internal/gitrepo/ ./internal/store/`
Expected: PASS（store の既存テストも通る: Commit の挙動は変わらない）

- [ ] **Step 5: コミット**

```bash
git add internal/gitrepo
git commit -m "feat(gitrepo): merge-base・履歴・fast-forward・index.lock・repack を足す"
```

---

### Task 3: config — accounts、remotes、sync の設定と、アカウントから作る author

**Files:**
- Modify: `internal/config/config.go`, `internal/config/author.go`, `internal/config/config_test.go`, `internal/config/author_test.go`

**Interfaces:**
- Produces:
  - `type Account struct{ Name, Host, Login string }`、`func (a Account) ID() string`（`host/login`）
  - `StoreEntry` に `Remotes []string`、`Account string`（`stores[].account` の名前）
  - `Config.Accounts []Account`、`Config.Sync SyncConfig`、`type SyncConfig struct{ Interval, PushInterval time.Duration }`（既定 2m と 30s）
  - `func (c *Config) Writer(e StoreEntry) (Account, bool)` — `e.Account` の指す、なければアカウントが1つならそれ
  - `func (c *Config) Me() []string` — 全アカウントの ID
  - `func (c *Config) AddStore(name, path string, remotes []string) error`（引数を足す。既存の呼び出しは nil を渡す）
  - `func (c *Config) RemoveStore(name string) error`
  - `func CommitAuthor(acct Account, ok bool, getenv func(string) string) (gitrepo.Signature, error)` — ok なら `Signature{acct.Login, acct.Login + "@users.noreply." + acct.Host}`、でなければ `Author(getenv)`

- [ ] **Step 1: テストを書く**（`config_test.go` に足す）

```go
func TestLoadAccountsRemotesSync(t *testing.T) {
	p := writeConfig(t, `accounts:
  - name: personal
    host: github.com
    login: alice
stores:
  - name: org-a
    path: /s/org-a
    remotes: [/r/a.git, /r/b.git]
    account: personal
sync:
  interval: 5m
  push_interval: 10s
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Accounts) != 1 || c.Accounts[0].ID() != "github.com/alice" {
		t.Errorf("accounts = %+v", c.Accounts)
	}
	e := c.Stores[0]
	if !slices.Equal(e.Remotes, []string{"/r/a.git", "/r/b.git"}) || e.Account != "personal" {
		t.Errorf("store = %+v", e)
	}
	if c.Sync.Interval != 5*time.Minute || c.Sync.PushInterval != 10*time.Second {
		t.Errorf("sync = %+v", c.Sync)
	}
	if a, ok := c.Writer(e); !ok || a.Login != "alice" {
		t.Errorf("Writer = %+v, %v", a, ok)
	}
}

func TestSyncDefaults(t *testing.T) {
	c, _ := Load(writeConfig(t, "stores: []\n"))
	if c.Sync.Interval != 2*time.Minute || c.Sync.PushInterval != 30*time.Second {
		t.Errorf("defaults = %+v", c.Sync)
	}
}

func TestWriterRules(t *testing.T) {
	two := `accounts:
  - {name: a, host: github.com, login: a}
  - {name: b, host: github.com, login: b}
stores:
  - {name: s, path: /s}
`
	c, _ := Load(writeConfig(t, two))
	if _, ok := c.Writer(c.Stores[0]); ok {
		t.Error("two accounts and no stores[].account: want no writer")
	}
	one := strings.Replace(two, "  - {name: b, host: github.com, login: b}\n", "", 1)
	c, _ = Load(writeConfig(t, one))
	if a, ok := c.Writer(c.Stores[0]); !ok || a.Name != "a" {
		t.Errorf("one account: Writer = %+v, %v", a, ok)
	}
}

func TestBadAccountAndSync(t *testing.T) {
	for _, src := range []string{
		"accounts:\n  - {name: a, host: github.com}\n",                 // no login
		"stores:\n  - {name: s, path: /s, account: nobody}\naccounts: []\n", // unknown account
		"sync:\n  interval: soon\n",
	} {
		if _, err := Load(writeConfig(t, src)); err == nil {
			t.Errorf("Load(%q) succeeded", src)
		}
	}
}

func TestAddAndRemoveStoreKeepsComments(t *testing.T) {
	p := writeConfig(t, "# mine\nstores:\n  - name: a # first\n    path: /a\n")
	c, _ := Load(p)
	if err := c.AddStore("b", "/b", []string{"/r/b.git"}); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveStore("a"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p.ConfigFile)
	if !strings.Contains(string(b), "# mine") || strings.Contains(string(b), "name: a") || !strings.Contains(string(b), "/r/b.git") {
		t.Errorf("config.yaml =\n%s", b)
	}
}
```

`author_test.go`:

```go
func TestCommitAuthorFromAccount(t *testing.T) {
	sig, err := CommitAuthor(Account{Host: "github.com", Login: "alice"}, true, func(string) string { return "" })
	if err != nil || sig != (gitrepo.Signature{Name: "alice", Email: "alice@users.noreply.github.com"}) {
		t.Errorf("CommitAuthor = %+v, %v", sig, err)
	}
}
```

（`writeConfig` は既存のテスト補助を使う。なければ `config_test.go` に、一時ディレクトリに config.yaml を書いて `Paths` を返す関数を足す。）

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/config/`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `Load` で `accounts`（各要素に `name`・`host`・`login` が必須。`token` などほかのキーは無視して残す）、`stores[].remotes`（文字列の列。`~/` は展開しない。URL かもしれないため）、`stores[].account`（`accounts` にない名前はエラー）、`sync.interval`・`sync.push_interval`（`time.ParseDuration`、0 以下はエラー）を読む
- `AddStore` に `remotes` を足し、空でなければ `remotes: [...]` をフロースタイルの列で書く
- `RemoveStore` は `stores` の列から要素の Node を取り除く（前後のコメントは要素に付いたものだけ消える）
- `author.go` のコメント「Accounts replace this in phase 3.」を「Stores with an account use CommitAuthor instead.」に直す

- [ ] **Step 4: 既存の呼び出しを直してテストを通す**

Run: `grep -rn 'AddStore(' --include=*.go . && go test ./...`
Expected: 呼び出し（`cmd/hedgehug/store_init.go`、`tools/devenv/main.go`、テスト）を `AddStore(name, path, nil)` に直して PASS

- [ ] **Step 5: コミット**

```bash
git add internal/config cmd tools
git commit -m "feat(config): accounts・remotes・sync を読み、アカウントからコミットの author を作る"
```

---

### Task 4: store と CLI — 書き手のアカウント、Hedgehug-Author、created_by とコメントの author

**Files:**
- Modify: `internal/store/tx.go`（`Op.Account`）, `cmd/hedgehug/app.go`（`update`）, `cmd/hedgehug/add.go`, `cmd/hedgehug/comment.go`, `cmd/hedgehug/helpers_test.go`, 関係するテスト

**Interfaces:**
- Consumes: Task 3 の `Writer`、`CommitAuthor`、`Account.ID`
- Produces:
  - `Op.Account string` — 書き手のアカウント ID。空でなければ `Hedgehug-Author: <id>` を付ける
  - `func (a *app) writer() (config.StoreEntry, config.Account, bool, error)` — 選んだ Store とその書き手
  - `app.update` は author を `CommitAuthor` で作り、`op.Account` を入れる
  - add は `CreatedBy` を、comment は `Author` を書き手の ID にする（書き手がいなければ空のまま）

- [ ] **Step 1: テストを書く**

`internal/store/commit_test.go` に:

```go
func TestAuthorTrailer(t *testing.T) {
	s := newTestStore(t)
	x := &model.Task{ULID: model.NewULID(), Workspace: testWorkspace(t, s).ULID, Kind: "task", Title: "x", Status: "todo"}
	if _, err := s.Update(Op{Name: "add", Task: x.ULID, Author: testAuthor, Account: "github.com/alice"}, func(tx *Tx) error { return tx.PutTask(x) }); err != nil {
		t.Fatal(err)
	}
	if msg := headLog(t, s.dir)[0].Message; !strings.Contains(msg, "\nHedgehug-Author: github.com/alice") {
		t.Errorf("message = %q", msg)
	}
}
```

`cmd/hedgehug/add_test.go` に:

```go
func TestAddWithAccount(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.writeAccounts("github.com", "alice") // appends accounts: to config.yaml
	te.ok("add", "x")
	out := te.ok("show", "ME-1", "--json")
	if !strings.Contains(out, `"created_by": "github.com/alice"`) {
		t.Errorf("show = %s", out)
	}
	c := te.headCommit("me")
	if c.Author.Name != "alice" || c.Author.Email != "alice@users.noreply.github.com" {
		t.Errorf("author = %+v", c.Author)
	}
	te.ok("comment", "ME-1", "hi")
	if out := te.ok("show", "ME-1", "--json"); !strings.Contains(out, `"author": "github.com/alice"`) {
		t.Errorf("comment author missing: %s", out)
	}
}

func TestAddWithoutAccountUsesGitConfig(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "x")
	if c := te.headCommit("me"); c.Author.Email != "alice@example.com" || strings.Contains(c.Message, "Hedgehug-Author") {
		t.Errorf("commit = %+v", c)
	}
}
```

`helpers_test.go` に `writeAccounts(host, login string)`（config.yaml の先頭に `accounts:` の要素を書く）と `headCommit(store string) *object.Commit` を足す。あわせて `testEnv.getenv` で `HEDGEHUG_AUTO_SYNC` が `te.env` になければ `"0"` を返す（Task 15 の起動を止める既定）。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/store/ ./cmd/hedgehug/ -run 'Author|Account'`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `tx.go` の `commitFor`: `op.Account != ""` なら `Hedgehug-Op` の後に `Hedgehug-Author` を足す
- `app.go`: `update` を次に置き換える

```go
// update runs one operation as one commit. The author is the store's
// account, or the git config when the store has none.
func (a *app) update(s *store.Store, op store.Op, fn func(*store.Tx) error) (bool, error) {
	_, acct, ok, err := a.writer()
	if err != nil {
		return false, err
	}
	author, err := config.CommitAuthor(acct, ok, a.getenv)
	if err != nil {
		return false, err
	}
	op.Author = author
	if ok {
		op.Account = acct.ID()
	}
	beforeUpdate()
	return s.Update(op, fn)
}
```

  `writer` は `a.config()` と `c.Select(a.storeName)` から `c.Writer(e)` を返す
- add（テンプレートの子も含む）で `CreatedBy`、comment で `Author` に書き手の ID を入れる。値はロックの外で決めてよい（設定は操作の間に変わらない）

- [ ] **Step 4: テストを通す**

Run: `go test ./...`
Expected: PASS（`created_by` を空と期待していた既存のテスト・ゴールデンがあれば、accounts のない環境では空のままなので変わらない）

- [ ] **Step 5: コミット**

```bash
git add internal/store cmd/hedgehug
git commit -m "feat: 書き手のアカウントで author・Hedgehug-Author・created_by・コメントの author を埋める"
```

---

### Task 5: rank の重複 — ID の順で並べ、書き込みで振り直す

**Files:**
- Create: `internal/store/rank.go`, `internal/store/rank_test.go`
- Modify: `internal/index/index.go`（`sortByRank`）, `internal/query/eval.go`（`byRank`）, `internal/model/taskid.go`（`CompareIDs`）, `internal/store/tx.go`（`Update` から呼ぶ）

**Interfaces:**
- Produces:
  - `func CompareIDs(a, b string) int`（model）— 接頭辞の辞書順、次に番号の数値順。ID の形でないものは後ろで、文字列の順
  - 並びの決着: rank → `CompareIDs(ID)` → ULID（index の `sortByRank`、query の `byRank` の両方）
  - `tx.rerank() error` — `Update` で `assignIDs` の後、`validate` の前に呼ぶ

- [ ] **Step 1: テストを書く**

`internal/model/taskid_test.go`:

```go
func TestCompareIDs(t *testing.T) {
	ids := []string{"WEB-10", "API-3", "WEB-9", "", "WEB-1"}
	slices.SortFunc(ids, CompareIDs)
	if want := []string{"API-3", "WEB-1", "WEB-9", "WEB-10", ""}; !slices.Equal(ids, want) {
		t.Errorf("sorted = %v", ids)
	}
}
```

`internal/store/rank_test.go`:

```go
func TestRerankOnWrite(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	mk := func(title string, r model.Rank) *model.Task {
		return &model.Task{ULID: model.NewULID(), Workspace: ws.ULID, Kind: "task", Title: title, Status: "todo", Rank: r}
	}
	before, a, b, c, after := mk("before", "a0"), mk("a", "a5"), mk("b", "a5"), mk("c", "a5"), mk("after", "a9")
	mustPut(t, s, before, a, b, c, after) // a, b, c get TST-2..4: their order is a, b, c
	mustPut(t, s, a)                       // a is first of the a5 group: goes between a0 and a5
	if !(a.Rank > "a0" && a.Rank < "a5") {
		t.Errorf("a.Rank = %s", a.Rank)
	}
	mustPut(t, s, b) // b is not first: goes between a5 and a9
	if !(b.Rank > "a5" && b.Rank < "a9") {
		t.Errorf("b.Rank = %s", b.Rank)
	}
	if got, _ := s.Task(c.ULID); got.Rank != "a5" {
		t.Errorf("c was rewritten: %s", got.Rank)
	}
}

func TestRerankKeepsUniqueRank(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	x := &model.Task{ULID: model.NewULID(), Workspace: ws.ULID, Kind: "task", Title: "x", Status: "todo", Rank: "a5"}
	mustPut(t, s, x)
	x.Title = "y"
	mustPut(t, s, x)
	if x.Rank != "a5" {
		t.Errorf("rank changed without a duplicate: %s", x.Rank)
	}
}
```

index と query は既存の並びのテストに、同じ rank の兄弟が ID の順（`WEB-9` が `WEB-10` より前）に並ぶケースを足す。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/model/ ./internal/store/ ./internal/index/ ./internal/query/ -run 'CompareIDs|Rerank|Rank'`
Expected: FAIL

- [ ] **Step 3: 実装する**

`rank.go`:

```go
package store

// rerank gives a task put in the operation a rank of its own when a sibling
// has the same rank: the design orders such siblings by id and fixes the rank
// at the next write. Only the written task changes. The first of the group
// moves before it, any other after it, so the order against the siblings
// outside the group stays.
func (tx *Tx) rerank() error {
	for _, ulid := range tx.taskOrder {
		t := tx.tasks[ulid]
		if t == nil || t.DeletedAt != nil {
			continue
		}
		sibs, err := tx.siblings(t) // live siblings as the operation leaves them, t excluded
		if err != nil {
			return err
		}
		var lower, higher model.Rank
		group := []*model.Task{t}
		for _, o := range sibs {
			switch {
			case o.Rank == t.Rank:
				group = append(group, o)
			case o.Rank < t.Rank && o.Rank > lower:
				lower = o.Rank
			case o.Rank > t.Rank && (higher == "" || o.Rank < higher):
				higher = o.Rank
			}
		}
		if len(group) == 1 {
			continue
		}
		slices.SortFunc(group, func(a, b *model.Task) int {
			if c := model.CompareIDs(a.ID, b.ID); c != 0 {
				return c
			}
			return cmp.Compare(a.ULID.String(), b.ULID.String())
		})
		var r model.Rank
		if group[0] == t {
			r, err = model.Between(lower, t.Rank)
		} else {
			r, err = model.Between(t.Rank, higher)
		}
		if err != nil {
			return err
		}
		t.Rank = r
	}
	return nil
}
```

テストの `mustPut(t, s, tasks...)` は、既存の `put(s, tasks...) error`（`refs_test.go`）を呼んで失敗なら `t.Fatal` する補助として `helpers_test.go` に足す。

`tx.siblings(t)` は同じ親（`Parent` が同じ、ルートなら parent なし）で同じワークスペースのタスクを、`tx.s.idNumbers` と同じく index があれば使い、なければ `tx.s.Tasks()` を歩いて集める。index を使うため `store` に `type Siblings func(parent *model.ULID, ws model.ULID) ([]model.ULID, error)` と `SetSiblings` を足し、`index.Open` が `s.SetSiblings(ix.siblingULIDs)` を登録する（`Refresh` してから `Children`/`Roots` を引く）。読んだ ULID は `tx.task(ulid)` で読み直す（同じ操作で書いたものを優先）。`model.Between("", x)` が先頭の前を返すことを `rank_test.go` で確かめておく。

`Update` で `tx.assignIDs()` の直後に `tx.rerank()` を呼ぶ。rerank で変えた rank は失敗時に戻す（`oldIDs` と同じく `oldRanks` に積み、`restoreIDs` で一緒に戻す）。

- [ ] **Step 4: テストを通す**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal
git commit -m "feat: 同じ rank の兄弟を ID の順で並べ、書き込むタスクの rank を振り直す"
```

---
### Task 6: merge — 入口、LWW、diff3、タスクのマージ

**Files:**
- Create: `internal/merge/merge.go`, `internal/merge/text.go`, `internal/merge/task.go`, `internal/merge/text_test.go`, `internal/merge/task_test.go`, `internal/merge/helpers_test.go`
- Modify: `go.mod`（`github.com/sergi/go-diff` を直接の依存に）

**Interfaces:**
- Produces（`package merge`）:

```go
// Side is one side of a three-way merge of one file.
type Side struct {
	Data   []byte                        // nil when the file is not there
	When   func(path []string) time.Time // when the side last changed the yaml value at path; nil outside yaml files
	Author string                        // the account (or email) that last changed the file on this side
	Time   time.Time                     // when the side last changed the file (for "other files")
}

// Overwritten is a value that lost to the other side.
type Overwritten struct {
	Path   string // the file
	Field  string // title, props.priority...; * for the whole file
	Value  string // the losing value as one line of JSON (the SHA-1 for other files)
	Author string // who wrote the losing value
}

type NoticeKind string

const Revived NoticeKind = "revived"

// Notice is something the inbox shows besides overwritten values.
type Notice struct {
	Kind   NoticeKind
	Path   string
	Author string // whom to tell
}

type Result struct {
	Data        []byte // nil removes the file
	Overwritten []Overwritten
	Notices     []Notice
}

type Policy int

const (
	LWW       Policy = iota // sync and the merge driver
	OursWins                // undo: ours takes every field both sides changed
)

func Merge(path string, sc *schema.Schema, base []byte, ours, theirs Side, p Policy) (Result, error)

// Diff3 merges text line by line. Overlapping changes keep both, older first.
func Diff3(base, a, b string, aFirst bool) string
```

- [ ] **Step 1: diff3 のテストを書く**（`text_test.go`）

```go
func TestDiff3(t *testing.T) {
	for _, c := range []struct{ name, base, a, b, want string }{
		{"only a", "1\n2\n3\n", "1\nA\n3\n", "1\n2\n3\n", "1\nA\n3\n"},
		{"apart", "1\n2\n3\n4\n", "A\n2\n3\n4\n", "1\n2\n3\nB\n", "A\n2\n3\nB\n"},
		{"same", "1\n2\n", "1\nX\n", "1\nX\n", "1\nX\n"},
		{"overlap", "1\n2\n3\n", "1\nA\n3\n", "1\nB\n3\n", "1\nA\n-----\nB\n3\n"},
		{"both append", "1\n", "1\nA\n", "1\nB\n", "1\nA\n-----\nB\n"},
		{"newline at end", "1", "1\n", "1", "1\n"},
		{"empty base", "", "A\n", "B\n", "A\n-----\nB\n"},
	} {
		if got := Diff3(c.base, c.a, c.b, true); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	if got := Diff3("1\n", "1\nA\n", "1\nB\n", false); got != "1\nB\n-----\nA\n" {
		t.Errorf("b first: %q", got)
	}
}
```

- [ ] **Step 2: タスクのテストを書く**（`task_test.go`）

`helpers_test.go` に、frontmatter を組み立てる補助を置く:

```go
// task builds a task file. Fields are given as "key: value" lines.
func task(updated string, lines ...string) []byte {
	return []byte("---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nid: WEB-1\nworkspace: 01JC0A1B2C3D4E5F6G7H8J9K0M\nkind: task\n" +
		strings.Join(lines, "\n") + "\nupdated: " + updated + "\n---\n")
}

const taskPath = "tasks/0A/P0/01JBZ8K3QW7M2X9V4T6R5N8P0A.md" // shard per model.TaskPath

func side(data []byte, author string) Side { return Side{Data: data, Author: author} }

func mergeTask(t *testing.T, base, ours, theirs Side) Result {
	t.Helper()
	r, err := Merge(taskPath, schema.Default(), base.Data, ours, theirs, LWW)
	if err != nil {
		t.Fatal(err)
	}
	// Every merge must be symmetric.
	r2, err := Merge(taskPath, schema.Default(), base.Data, theirs, ours, LWW)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r.Data, r2.Data) || !sameOverwritten(r.Overwritten, r2.Overwritten) {
		t.Fatalf("not symmetric:\n%s\n---\n%s\n%v\n%v", r.Data, r2.Data, r.Overwritten, r2.Overwritten)
	}
	return r
}

func decoded(t *testing.T, b []byte) *model.Task {
	t.Helper()
	x, err := model.DecodeTask(b)
	if err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	return x
}
```

`taskPath` のシャードは `model.TaskPath` で作る（定数でなく `var taskPath = model.TaskPath(mustULID("01JBZ8K3QW7M2X9V4T6R5N8P0A"))`）。

```go
func TestTaskOneSide(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a", "status: todo")
	ours := task("2026-10-31T11:00:00Z", "title: b", "status: todo")
	theirs := task("2026-10-31T12:00:00Z", "title: a", "status: doing")
	r := mergeTask(t, side(base, ""), side(ours, "github.com/a"), side(theirs, "github.com/b"))
	x := decoded(t, r.Data)
	if x.Title != "b" || x.Status != "doing" || len(r.Overwritten) != 0 {
		t.Errorf("title %s status %s overwritten %v", x.Title, x.Status, r.Overwritten)
	}
	if !x.Updated.Equal(time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("updated = %s, want the larger", x.Updated)
	}
}

func TestTaskLWW(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a")
	ours := task("2026-10-31T11:00:00Z", "title: ours")
	theirs := task("2026-10-31T12:00:00Z", "title: theirs")
	r := mergeTask(t, side(base, ""), side(ours, "github.com/a"), side(theirs, "github.com/b"))
	if x := decoded(t, r.Data); x.Title != "theirs" {
		t.Errorf("title = %s", x.Title)
	}
	want := []Overwritten{{Path: taskPath, Field: "title", Value: `"ours"`, Author: "github.com/a"}}
	if !sameOverwritten(r.Overwritten, want) {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

func TestTaskLWWTieBreaks(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a")
	same := "2026-10-31T11:00:00Z"
	r := mergeTask(t, side(base, ""), side(task(same, "title: x"), "github.com/a"), side(task(same, "title: y"), "github.com/b"))
	if x := decoded(t, r.Data); x.Title != "y" {
		t.Errorf("author tie-break: %s", x.Title) // github.com/b > github.com/a
	}
	r = mergeTask(t, side(base, ""), side(task(same, "title: x"), "u"), side(task(same, "title: y"), "u"))
	if x := decoded(t, r.Data); x.Title != "y" {
		t.Errorf("value tie-break: %s", x.Title) // "y" > "x"
	}
}

func TestTaskSameChangeNotOverwritten(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a")
	r := mergeTask(t, side(base, ""), side(task("2026-10-31T11:00:00Z", "title: z"), "a"), side(task("2026-10-31T12:00:00Z", "title: z"), "b"))
	if len(r.Overwritten) != 0 {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

func TestTaskSets(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "labels: [a, b]", "former_ids: [OLD-1]")
	ours := task("2026-10-31T11:00:00Z", "labels: [b, c]", "former_ids: [OLD-1, X-1]")
	theirs := task("2026-10-31T12:00:00Z", "labels: [a, b, d]", "former_ids: [OLD-1, Y-1]")
	x := decoded(t, mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b")).Data)
	if !slices.Equal(x.Labels, []string{"b", "c", "d"}) {
		t.Errorf("labels = %v", x.Labels) // a removed by ours, c and d added (added ones sorted)
	}
	if !slices.Equal(x.FormerIDs, []string{"OLD-1", "X-1", "Y-1"}) {
		t.Errorf("former_ids = %v", x.FormerIDs)
	}
}

func TestTaskIDBothChangedKeepsLoserInFormer(t *testing.T) {
	base := task("2026-10-31T10:00:00Z")
	ours := bytes.Replace(task("2026-10-31T11:00:00Z", "former_ids: [WEB-1]"), []byte("id: WEB-1"), []byte("id: API-1"), 1)
	theirs := bytes.Replace(task("2026-10-31T12:00:00Z", "former_ids: [WEB-1]"), []byte("id: WEB-1"), []byte("id: BE-1"), 1)
	x := decoded(t, mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b")).Data)
	if x.ID != "BE-1" || !slices.Contains(x.FormerIDs, "API-1") || !slices.Contains(x.FormerIDs, "WEB-1") {
		t.Errorf("id = %s, former = %v", x.ID, x.FormerIDs)
	}
}

func TestTaskProps(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "props:", "  priority: low", "  points: 1")
	ours := task("2026-10-31T11:00:00Z", "props:", "  priority: high", "  points: 1")
	theirs := task("2026-10-31T12:00:00Z", "props:", "  priority: low", "  points: 3")
	x := decoded(t, mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b")).Data)
	if v := x.Props.Get("priority"); v == nil || v.Value != "high" {
		t.Errorf("priority = %v", v)
	}
	if v := x.Props.Get("points"); v == nil || v.Value != "3" {
		t.Errorf("points = %v", v)
	}
}

func TestTaskBodyDiff3(t *testing.T) {
	base := append(task("2026-10-31T10:00:00Z"), "1\n2\n"...)
	ours := append(task("2026-10-31T11:00:00Z"), "1\nA\n"...)
	theirs := append(task("2026-10-31T12:00:00Z"), "1\nB\n"...)
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	if x := decoded(t, r.Data); x.Body != "1\nA\n-----\nB\n" {
		t.Errorf("body = %q", x.Body) // older (ours) first
	}
	if len(r.Overwritten) != 0 {
		t.Errorf("overlap recorded as overwritten: %v", r.Overwritten)
	}
}

func TestTaskDeleteVsEdit(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a")
	ours := task("2026-10-31T11:00:00Z", "title: a", "deleted_at: 2026-10-31T11:00:00Z")
	theirs := task("2026-10-31T12:00:00Z", "title: edited")
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	x := decoded(t, r.Data)
	if x.DeletedAt != nil || x.Title != "edited" {
		t.Errorf("deleted_at %v title %s", x.DeletedAt, x.Title)
	}
	if len(r.Notices) != 1 || r.Notices[0] != (Notice{Kind: Revived, Path: taskPath, Author: "a"}) {
		t.Errorf("notices = %v", r.Notices)
	}
}

func TestTaskPurgeVsEdit(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a")
	r := mergeTask(t, side(base, ""), side(nil, "a"), side(task("2026-10-31T12:00:00Z", "title: b"), "b"))
	if r.Data == nil || len(r.Notices) != 1 {
		t.Errorf("data %q notices %v", r.Data, r.Notices)
	}
	r = mergeTask(t, side(base, ""), side(nil, "a"), side(base, "b"))
	if r.Data != nil {
		t.Errorf("purge of an unchanged file kept it")
	}
}

func TestTaskKeepsCommentsAndUnknownKeys(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a # keep", "zz: 1")
	ours := task("2026-10-31T11:00:00Z", "title: a # keep", "zz: 1", "status: doing")
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(base, "b"))
	if !bytes.Contains(r.Data, []byte("# keep")) || !bytes.Contains(r.Data, []byte("zz: 1")) {
		t.Errorf("lost comment or key:\n%s", r.Data)
	}
}

func TestBrokenSideFallsBackToOtherFile(t *testing.T) {
	base := task("2026-10-31T10:00:00Z", "title: a")
	broken := []byte("---\nulid: [\n---\n")
	r, err := Merge(taskPath, schema.Default(), base, Side{Data: broken, Author: "a", Time: time.Unix(10, 0)}, Side{Data: task("2026-10-31T12:00:00Z", "title: b"), Author: "b", Time: time.Unix(20, 0)}, LWW)
	if err != nil || r.Data == nil || bytes.Equal(r.Data, broken) || len(r.Overwritten) != 1 || r.Overwritten[0].Field != "*" {
		t.Errorf("r = %+v, %v", r, err)
	}
}
```

`sameOverwritten` は両方を `(Path, Field, Value, Author)` で並べて比べる。

- [ ] **Step 3: 失敗を確かめる**

Run: `go test ./internal/merge/`
Expected: FAIL（パッケージがない）

- [ ] **Step 4: text.go を書く**

- `DiffLinesToChars` で3つの文字列を1文字1行に写し、`diffmatchpatch.DiffMain(baseChars, aChars, false)` と同じく b で差分を取る
- 各差分を base の行の範囲 `[start, end)` と置き換える行の列（hunk）に直す（挿入は `start == end`）
- 2つの hunk の列を base の位置で前から歩く。重ならない hunk はそのまま当てる。重なる hunk（範囲が交わる、または同じ位置への挿入）は、両方の範囲の和をとって1つの衝突にまとめ、同じ置き換えなら1つ、違えば `aFirst` の順で「a の行、`-----`、b の行」を出す
- 末尾の改行: 行に分けるときに最後の改行を行の一部として持つ（`"1"` と `"1\n"` は別の行）。`DiffLinesToChars` はこの扱いなので、そのまま使える

- [ ] **Step 5: merge.go と task.go を書く**

`merge.go`:

```go
// Merge merges one file of the store. The kind of the file comes from its
// path. Both orders of ours and theirs give the same result.
func Merge(path string, sc *schema.Schema, base []byte, ours, theirs Side, p Policy) (Result, error) {
	if bytes.Equal(ours.Data, theirs.Data) {
		return Result{Data: ours.Data}, nil
	}
	if bytes.Equal(base, ours.Data) {
		return Result{Data: theirs.Data}, nil
	}
	if bytes.Equal(base, theirs.Data) {
		return Result{Data: ours.Data}, nil
	}
	m := &merger{path: path, sc: sc, p: p}
	switch kindOf(path) {
	case kindTask:
		return m.task(base, ours, theirs)
	case kindRecord: // comments/, worklogs/
		return m.record(base, ours, theirs)
	case kindPlan: // sprints/, releases/, templates/, workspaces/
		return m.plan(base, ours, theirs)
	case kindYAML: // schema.yaml, automations.yaml, .hedgehug/store.yaml
		return m.yaml(base, ours, theirs)
	case kindNumbers: // .hedgehug/numbers.yaml
		return m.numbers(base, ours, theirs)
	}
	return m.other(base, ours, theirs)
}
```

LWW の比較を1か所に置く:

```go
// later reports whether side a beats side b for a value both changed:
// the newer time, then the larger author, then the larger encoded value.
func later(at, bt time.Time, aAuthor, bAuthor string, aVal, bVal []byte) bool {
	if !at.Equal(bt) {
		return at.After(bt)
	}
	if aAuthor != bAuthor {
		return aAuthor > bAuthor
	}
	return bytes.Compare(aVal, bVal) > 0
}
```

`OursWins` のときは、両側が変えた値で常に ours を採り、上書きとして記録しない。

`task.go` の流れ:

1. 3つを `model.DecodeTask` で読む。ours か theirs が読めなければ `m.other` に落とす。base が読めない・ない場合は空の `model.Task` を base にする（両側で作った場合）
2. ファイルの有無（片側が nil）: もう片側が base から変わっていなければ消す（Result.Data = nil）。変わっていれば変えた側を残し、消した側の author に `Notice{Revived}`
3. 土台を決める: `Author` が大きい側、同じならバイト列が大きい側。結果はその側の `*model.Task` を書き換えて `Encode` する（コメントと未知のキーと書式が残る）
4. フィールドごとに、base・ours・theirs の値を比べる関数を表で持つ:

```go
type field struct {
	name string
	get  func(*model.Task) any       // the value, compared with reflect.DeepEqual (Node values with model.NodeEqual)
	set  func(dst *model.Task, v any)
	kind fieldKind // scalar (LWW), set (set merge), body, updated, deleted
}
```

   スカラー（title・status・kind・parent・rank・sprint・workspace・created・created_by・id）は片側だけ変わればその側、両側が別に変えれば `later(ours.Updated, theirs.Updated, …, json(値))`。負けた値を `Overwritten{Field: name, Value: json(値)}`。`id` の負けた値は `former_ids` の集合マージの結果に足す（重複は足さない）。

   集合（assignees・watchers・labels・repos・blocked_by・releases・links・former_ids）は `mergeSet(base, ours, theirs []string)`: base の順で両側に残った要素、次に両側で足された要素（ours と theirs の和、重複なし）を辞書順。links は `Ref` を要素の名前にして、要素の値（type・sync）が両側で違えばスカラーと同じく LWW。`blocked_by`・`releases` は ULID の文字列で扱う。

   props・gh・未知のキー（Extra）は、キーごとに同じ規則（スカラーとして LWW、フィールド名は `props.<key>`・`gh.<key>`・`<key>`）。props のうち `sc` で multi（`multi_enum`、multi の `task_ref`）のものは、値の列を集合マージ。片側でキーが消え、もう片側で変わっていれば、変わった側を残す。

   本文は片側だけならその側。両側なら `Diff3(base, ours, theirs, oursFirst)`。`oursFirst` は `later(theirs.Updated, ours.Updated, theirs.Author, ours.Author, theirs.Data, ours.Data)`（ours が古いなら ours が先）。

   `updated` は大きい方。`deleted_at`: 片側だけが `deleted_at` を変え、もう片側が `deleted_at` 以外を変えていたら、`deleted_at` を外し、消した側の author に `Notice{Revived}`。両側が変えたら LWW。

5. `ulid` は変えない

- [ ] **Step 6: テストを通す**

Run: `go test ./internal/merge/ && go mod tidy`
Expected: PASS。go.mod に `github.com/sergi/go-diff` が直接の依存として上がる

- [ ] **Step 7: コミット**

```bash
git add internal/merge go.mod go.sum
git commit -m "feat(merge): タスクのフィールド単位の3-wayマージと行単位の diff3 を作る"
```

---

### Task 7: merge — コメント・作業時間、計画のファイル、yaml、numbers.yaml、その他、対称性の fuzz

**Files:**
- Create: `internal/merge/record.go`, `internal/merge/plan.go`, `internal/merge/yaml.go`, `internal/merge/other.go`, `internal/merge/kinds_test.go`, `internal/merge/fuzz_test.go`

**Interfaces:**
- Consumes: Task 6 の `Merge`、`later`、`Diff3`、`mergeSet`
- Produces: `Merge` がすべての種類を扱う

- [ ] **Step 1: テストを書く**（`kinds_test.go`）

```go
func TestCommentWholeFileLWW(t *testing.T) {
	p := "comments/0A/P0/01JBZ8K3QW7M2X9V4T6R5N8P0A/01JC1111111111111111111111.md"
	c := func(updated, body string) []byte {
		return []byte("---\nulid: 01JC1111111111111111111111\nauthor: github.com/a\ncreated: 2026-10-31T10:00:00Z\nupdated: " + updated + "\n---\n" + body)
	}
	r, _ := Merge(p, nil, c("2026-10-31T10:00:00Z", "x\n"), Side{Data: c("2026-10-31T11:00:00Z", "a\n"), Author: "a"}, Side{Data: c("2026-10-31T12:00:00Z", "b\n"), Author: "b"}, LWW)
	if !bytes.HasSuffix(r.Data, []byte("b\n")) || len(r.Overwritten) != 1 || r.Overwritten[0].Field != "*" {
		t.Errorf("r = %s %v", r.Data, r.Overwritten)
	}
}

func TestNewFilesFromOneSide(t *testing.T) {
	p := "comments/0A/P0/01JBZ8K3QW7M2X9V4T6R5N8P0A/01JC1111111111111111111111.md"
	r, _ := Merge(p, nil, nil, Side{Data: []byte("x")}, Side{}, LWW)
	if string(r.Data) != "x" {
		t.Errorf("new file lost: %q", r.Data)
	}
}

func TestWorkspacePrefixPairFollowsPrefixSet(t *testing.T) {
	p := "workspaces/01JC0A1B2C3D4E5F6G7H8J9K0M.md"
	ws := func(prefix, prefixSet, updated, name string) []byte {
		return []byte("---\nulid: 01JC0A1B2C3D4E5F6G7H8J9K0M\nname: " + name + "\nid_prefix: " + prefix + "\nprefix_set: " + prefixSet + "\ncreated: 2026-10-30T00:00:00Z\nupdated: " + updated + "\n---\n")
	}
	base := ws("WEB", "2026-10-30T00:00:00Z", "2026-10-30T00:00:00Z", "web")
	ours := ws("FE", "2026-10-31T12:00:00Z", "2026-10-31T12:00:00Z", "web")      // prefix set later
	theirs := ws("UI", "2026-10-31T11:00:00Z", "2026-10-31T13:00:00Z", "front") // renamed later
	r, _ := Merge(p, nil, base, Side{Data: ours, Author: "a"}, Side{Data: theirs, Author: "b"}, LWW)
	w, err := model.DecodeWorkspace(r.Data)
	if err != nil || w.IDPrefix != "FE" || w.Name != "front" {
		t.Errorf("workspace = %+v, %v", w, err)
	}
}

func TestSchemaYAMLByKeyAndElement(t *testing.T) {
	base := []byte("statuses:\n  - {name: todo, category: todo}\nprops:\n  priority: {type: enum, values: [low, high]}\n")
	ours := []byte("statuses:\n  - {name: todo, category: todo}\n  - {name: review, category: doing}\nprops:\n  priority: {type: enum, values: [low, high]}\n")
	theirs := []byte("statuses:\n  - {name: todo, category: todo}\n  - {name: qa, category: doing}\nprops:\n  priority: {type: enum, values: [low, mid, high]}\n")
	when := func(t time.Time) func([]string) time.Time { return func([]string) time.Time { return t } }
	r, err := Merge("schema.yaml", nil, base, Side{Data: ours, Author: "a", When: when(time.Unix(1, 0))}, Side{Data: theirs, Author: "b", When: when(time.Unix(2, 0))}, LWW)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"review", "qa", "mid"} {
		if !bytes.Contains(r.Data, []byte(s)) {
			t.Errorf("lost %s:\n%s", s, r.Data)
		}
	}
}

func TestNumbersTakeMax(t *testing.T) {
	r, _ := Merge(model.NumbersPath, nil, []byte("WEB: 3\n"), Side{Data: []byte("WEB: 5\nAPI: 1\n")}, Side{Data: []byte("WEB: 4\nBE: 2\n")}, LWW)
	n, err := model.DecodeNumbers(r.Data)
	if err != nil || n["WEB"] != 5 || n["API"] != 1 || n["BE"] != 2 || len(r.Overwritten) != 0 {
		t.Errorf("numbers = %v, %v, %v", n, err, r.Overwritten)
	}
}

func TestOtherFileByTime(t *testing.T) {
	r, _ := Merge(".gitattributes", nil, []byte("a\n"), Side{Data: []byte("o\n"), Author: "a", Time: time.Unix(2, 0)}, Side{Data: []byte("t\n"), Author: "b", Time: time.Unix(1, 0)}, LWW)
	if string(r.Data) != "o\n" || len(r.Overwritten) != 1 || r.Overwritten[0].Author != "b" {
		t.Errorf("r = %q %v", r.Data, r.Overwritten)
	}
}
```

automations.yaml の `rules`（`id`）・`scoped`（`where`）と、schema の `scoped`（`where`）について、片側で足した要素・片側で消した要素・消された要素の中を変えた（`Notice{Revived}`）の3つの表テストも同じ形で足す。

`fuzz_test.go`:

```go
// FuzzSymmetric checks that swapping ours and theirs changes nothing.
func FuzzSymmetric(f *testing.F) {
	f.Add("a", "b", "c", "1\n2\n", "1\nx\n", "y\n2\n", int64(1), int64(2))
	mk := func(title, body string, u int64) []byte {
		updated := time.Unix(u%1e9, 0).UTC().Format(time.RFC3339)
		return append(task(updated, "title: "+strconv.Quote(title), "labels: ["+strconv.Quote(title)+"]"), body...)
	}
	f.Fuzz(func(t *testing.T, bt, ot, tt, bb, ob, tb string, ou, tu int64) {
		base, ours, theirs := mk(bt, bb, 0), mk(ot, ob, ou), mk(tt, tb, tu)
		a, err1 := Merge(taskPath, schema.Default(), base, Side{Data: ours, Author: "a"}, Side{Data: theirs, Author: "b"}, LWW)
		b, err2 := Merge(taskPath, schema.Default(), base, Side{Data: theirs, Author: "b"}, Side{Data: ours, Author: "a"}, LWW)
		if (err1 == nil) != (err2 == nil) || !bytes.Equal(a.Data, b.Data) || !sameOverwritten(a.Overwritten, b.Overwritten) {
			t.Fatalf("not symmetric")
		}
	})
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/merge/`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `record.go`: 両側を読み（読めなければ `other`）、`later(ours.Updated, theirs.Updated, …, bytes)` で勝った側のファイル全体。負けた側を `Overwritten{Field: "*", Value: json(負けた側のファイルの文字列)}`
- `plan.go`: スプリント・リリース・テンプレート・ワークスペースを `model` の型で読み、フィールドごとに Task 6 と同じ表の仕組み（スカラーは `updated` で LWW、本文は diff3、Extra はキーごと）。ワークスペースは `id_prefix` と `prefix_set` を1つのフィールド `id_prefix` として扱い、時刻は `prefix_set`。ファイルの有無はタスクと同じ規則
- `yaml.go`: `yaml.Load` で3つの Node を読み、`mergeNode(path []string, b, o, t *yaml.Node)` を再帰で。mapping はキーの和をとってキーごとに再帰。列は、`elemKey(path)` が見分けるキーを返す場合（`rules`→`id`、`scoped`→`where`、`statuses`・`kinds`・`categories`→`name`）だけ要素ごとに再帰し、並びは集合マージと同じ。それ以外は値として比べ、両側が変えたら `later(ours.When(path), theirs.When(path), …, 値のバイト列)`。片側が要素やキーを消し、もう片側がその中を変えたら残して `Notice{Revived}`。土台はタスクと同じ規則で選び、`yaml.Dump` で書く（コメントは土台の Node に付いたもの）。`numbers` は `model.DecodeNumbers` で読み、接頭辞ごとの max を土台の Node に書く（土台の Node のキーを書き換え、足りないキーは末尾に辞書順で足す）
- `other.go`: 両側で変わっていれば `Side.Time` が新しい側、同じなら author、バイト列。負けた側を `Overwritten{Field: "*", Value: sha1 の16進}`

- [ ] **Step 4: テストを通す**

Run: `go test ./internal/merge/ && go test ./internal/merge/ -fuzz FuzzSymmetric -fuzztime 30s`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/merge
git commit -m "feat(merge): コメント・計画のファイル・yaml・numbers.yaml・その他のファイルのマージを足す"
```

---

### Task 8: merge — マージ後の修復（Repair）と、番号の数え方の共有

**Files:**
- Create: `internal/merge/repair.go`, `internal/merge/repair_test.go`
- Modify: `internal/model/taskid.go`（`CountIDs`、`RaiseNumbers`）, `internal/store/ids.go`（model の関数を使う）

**Interfaces:**
- Produces:

```go
// model
func CountIDs(used map[string]int, t *Task)      // raises used to the numbers of t.ID and t.FormerIDs
func RaiseNumbers(used, more map[string]int)    // used[p] = max(used[p], more[p])

// merge
// Files is the set of files the repair looks at: HEAD with the merge laid over it.
type Files interface {
	Paths(prefix string) []string // sorted
	Read(path string) []byte      // nil when absent
}

type RepairInput struct {
	Files    Files
	Base     Files                                 // the merge base; nil for a repair commit
	OnOrigin func(ulid model.ULID, id string) bool // origin's main has this task with this id
	Changed  func(path string) bool                // the merge changed the file (parent_cycle picks among these)
	Author   func(path string) string              // who last changed the file
	Sc       *schema.Schema
	Now      time.Time
}

type Repaired struct {
	ULID   model.ULID // task or workspace
	Kind   string     // workspace_revived, id_prefix, parent_cycle, parent_deleted, workspace_mismatch, id_collision, id_renumbered, prefix_duplicate, name_duplicate
	Value  string     // the old value as one line of JSON (for id_collision: {"old":"WEB-43","new":"WEB-45"})
	Author string     // whom to tell; empty for id_renumbered
}

// Repair returns the files to write (nil data removes) and what it did.
func Repair(in RepairInput) (map[string][]byte, []Repaired, error)
```

- [ ] **Step 1: テストを書く**（`repair_test.go`）

`memFiles` は `map[string][]byte` で `Files` を満たす補助。`taskFile(ulid, id, ws string, parent string, updated string, extra ...string)` で作る。

```go
const wsA, wsB = "01JC0A1B2C3D4E5F6G7H8J9K0A", "01JC0A1B2C3D4E5F6G7H8J9K0B"

func wsFile(ulid, name, prefix string) []byte {
	return []byte("---\nulid: " + ulid + "\nname: " + name + "\nid_prefix: " + prefix + "\nprefix_set: 2026-10-30T00:00:00Z\ncreated: 2026-10-30T00:00:00Z\nupdated: 2026-10-30T00:00:00Z\n---\n")
}

func repair(t *testing.T, in RepairInput) (memFiles, []Repaired) {
	t.Helper()
	if in.Sc == nil {
		in.Sc = schema.Default()
	}
	if in.Author == nil {
		in.Author = func(string) string { return "github.com/x" }
	}
	if in.OnOrigin == nil {
		in.OnOrigin = func(model.ULID, string) bool { return false }
	}
	if in.Changed == nil {
		in.Changed = func(string) bool { return true }
	}
	out, rs, err := Repair(in)
	if err != nil {
		t.Fatal(err)
	}
	after := maps.Clone(in.Files.(memFiles))
	for p, b := range out {
		if b == nil {
			delete(after, p)
		} else {
			after[p] = b
		}
	}
	// Repair must be deterministic.
	out2, rs2, _ := Repair(in)
	if !maps.EqualFunc(out, out2, bytes.Equal) || !slices.Equal(rs, rs2) {
		t.Fatal("not deterministic")
	}
	return after, rs
}

func TestRepairIDCollisionKeepsOrigin(t *testing.T) {
	x, y := "01JD0000000000000000000001", "01JD0000000000000000000002"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		taskPath(x): taskFile(x, "WEB-43", wsA, "", "2026-10-31T10:00:00Z"),
		taskPath(y): taskFile(y, "WEB-43", wsA, "", "2026-10-31T10:00:00Z", "former_ids: [WEB-44]"),
		model.NumbersPath: []byte("WEB: 50\n"),
	}
	after, rs := repair(t, RepairInput{Files: f, OnOrigin: func(u model.ULID, id string) bool { return u.String() == y && id == "WEB-43" }})
	gx, gy := decodeTask(t, after[taskPath(x)]), decodeTask(t, after[taskPath(y)])
	if gy.ID != "WEB-43" || gx.ID != "WEB-51" || !slices.Contains(gx.FormerIDs, "WEB-43") {
		t.Errorf("x %s %v, y %s", gx.ID, gx.FormerIDs, gy.ID)
	}
	if len(rs) != 1 || rs[0].Kind != "id_collision" || rs[0].ULID.String() != x || rs[0].Value != `{"old":"WEB-43","new":"WEB-51"}` {
		t.Errorf("repaired = %+v", rs)
	}
}

func TestRepairIDCollisionNeitherOnOriginKeepsSmallerULID(t *testing.T) {
	x, y, z := "01JD0000000000000000000001", "01JD0000000000000000000002", "01JD0000000000000000000003"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		taskPath(x): taskFile(x, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
		taskPath(y): taskFile(y, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
		taskPath(z): taskFile(z, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
	}
	after, _ := repair(t, RepairInput{Files: f})
	ids := []string{decodeTask(t, after[taskPath(x)]).ID, decodeTask(t, after[taskPath(y)]).ID, decodeTask(t, after[taskPath(z)]).ID}
	if !slices.Equal(ids, []string{"WEB-1", "WEB-2", "WEB-3"}) {
		t.Errorf("ids = %v", ids) // the others numbered in ULID order
	}
}

func TestRepairCountsBrokenFiles(t *testing.T) {
	x, y, bad := "01JD0000000000000000000001", "01JD0000000000000000000002", "01JD0000000000000000000009"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		taskPath(x):   taskFile(x, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
		taskPath(y):   taskFile(y, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
		taskPath(bad): []byte("---\nid: WEB-77\n: [\n"),
	}
	after, _ := repair(t, RepairInput{Files: f})
	if got := decodeTask(t, after[taskPath(y)]).ID; got != "WEB-78" {
		t.Errorf("renumbered to %s, want WEB-78", got)
	}
}

func TestRepairParentCycle(t *testing.T) {
	a, b := "01JD0000000000000000000001", "01JD0000000000000000000002"
	base := memFiles{
		taskPath(a): taskFile(a, "WEB-1", wsA, "", "2026-10-31T10:00:00Z"),
		taskPath(b): taskFile(b, "WEB-2", wsA, "", "2026-10-31T10:00:00Z"),
	}
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		taskPath(a): taskFile(a, "WEB-1", wsA, b, "2026-10-31T11:00:00Z"),
		taskPath(b): taskFile(b, "WEB-2", wsA, a, "2026-10-31T12:00:00Z"), // the later move
	}
	after, rs := repair(t, RepairInput{Files: f, Base: base})
	if p := decodeTask(t, after[taskPath(b)]).Parent; p != nil {
		t.Errorf("b.parent = %v, want back to root", p)
	}
	if p := decodeTask(t, after[taskPath(a)]).Parent; p == nil || p.String() != b {
		t.Errorf("a.parent changed: %v", p)
	}
	if len(rs) != 1 || rs[0].Kind != "parent_cycle" {
		t.Errorf("repaired = %+v", rs)
	}
	after, _ = repair(t, RepairInput{Files: f}) // no base: to the root
	if decodeTask(t, after[taskPath(b)]).Parent != nil {
		t.Error("no base: b not moved to the root")
	}
}

func TestRepairDeletedParent(t *testing.T) {
	p, c := "01JD0000000000000000000001", "01JD0000000000000000000002"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		taskPath(p): taskFile(p, "WEB-1", wsA, "", "2026-10-31T10:00:00Z", "deleted_at: 2026-10-31T10:00:00Z"),
		taskPath(c): taskFile(c, "WEB-2", wsA, p, "2026-10-31T10:00:00Z"),
	}
	after, rs := repair(t, RepairInput{Files: f})
	if decodeTask(t, after[taskPath(c)]).Parent != nil || len(rs) != 1 || rs[0].Kind != "parent_deleted" {
		t.Errorf("child not moved to the root: %+v", rs)
	}
}

func TestRepairWorkspaceMismatchFollowsParent(t *testing.T) {
	p, c := "01JD0000000000000000000001", "01JD0000000000000000000002"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		model.WorkspacePath(mustULID(wsB)): wsFile(wsB, "api", "API"),
		taskPath(p): taskFile(p, "API-1", wsB, "", "2026-10-31T10:00:00Z", "former_ids: [WEB-1]"),
		taskPath(c): taskFile(c, "WEB-2", wsA, p, "2026-10-31T10:00:00Z"),
	}
	after, rs := repair(t, RepairInput{Files: f})
	got := decodeTask(t, after[taskPath(c)])
	if got.Workspace.String() != wsB || got.ID != "API-2" || !slices.Contains(got.FormerIDs, "WEB-2") {
		t.Errorf("child = %s %s %v", got.Workspace, got.ID, got.FormerIDs)
	}
	if len(rs) != 1 || rs[0].Kind != "workspace_mismatch" {
		t.Errorf("repaired = %+v", rs) // id_renumbered of the same task is not told
	}
}

func TestRepairRevivesRemovedWorkspace(t *testing.T) {
	x := "01JD0000000000000000000001"
	base := memFiles{model.WorkspacePath(mustULID(wsB)): wsFile(wsB, "api", "API")}
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		taskPath(x): taskFile(x, "API-1", wsB, "", "2026-10-31T10:00:00Z"),
	}
	after, rs := repair(t, RepairInput{Files: f, Base: base})
	if after[model.WorkspacePath(mustULID(wsB))] == nil || len(rs) != 1 || rs[0].Kind != "workspace_revived" {
		t.Errorf("workspace not revived: %+v", rs)
	}
}

func TestRepairInvalidPrefixBackToBase(t *testing.T) {
	base := memFiles{model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB")}
	f := memFiles{model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "web!")}
	after, rs := repair(t, RepairInput{Files: f, Base: base})
	w, _ := model.DecodeWorkspace(after[model.WorkspacePath(mustULID(wsA))])
	if w.IDPrefix != "WEB" || len(rs) != 1 || rs[0].Kind != "id_prefix" {
		t.Errorf("prefix = %s, %+v", w.IDPrefix, rs)
	}
}

func TestRepairPrefixAndNameDuplicatesOnlyTell(t *testing.T) {
	later := strings.Replace(string(wsFile(wsB, "web", "WEB")), "prefix_set: 2026-10-30", "prefix_set: 2026-10-31", 1)
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "WEB"),
		model.WorkspacePath(mustULID(wsB)): []byte(strings.Replace(later, "created: 2026-10-30", "created: 2026-10-31", 1)),
	}
	out, rs, err := Repair(RepairInput{Files: f, Sc: schema.Default(), Author: func(string) string { return "x" }, OnOrigin: func(model.ULID, string) bool { return false }, Changed: func(string) bool { return true }})
	if err != nil || len(out) != 0 {
		t.Fatalf("out = %v, err = %v; duplicates must not change data", out, err)
	}
	kinds := []string{}
	for _, r := range rs {
		if r.ULID.String() != wsB {
			t.Errorf("told the earlier workspace: %+v", r)
		}
		kinds = append(kinds, r.Kind)
	}
	if !slices.Equal(kinds, []string{"prefix_duplicate", "name_duplicate"}) {
		t.Errorf("kinds = %v", kinds)
	}
}

func TestRepairIDPrefixMismatchRenumbersSilently(t *testing.T) {
	x := "01JD0000000000000000000001"
	f := memFiles{
		model.WorkspacePath(mustULID(wsA)): wsFile(wsA, "web", "FE"),
		taskPath(x): taskFile(x, "WEB-3", wsA, "", "2026-10-31T10:00:00Z"),
	}
	after, rs := repair(t, RepairInput{Files: f})
	if got := decodeTask(t, after[taskPath(x)]); got.ID != "FE-1" || !slices.Contains(got.FormerIDs, "WEB-3") {
		t.Errorf("task = %s %v", got.ID, got.FormerIDs)
	}
	if len(rs) != 1 || rs[0].Kind != "id_renumbered" || rs[0].Author != "" {
		t.Errorf("repaired = %+v", rs)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/merge/ -run Repair`
Expected: FAIL

- [ ] **Step 3: model に数え方を移す**

`store/ids.go` の `countIDs` と `raise` を `model.CountIDs`・`model.RaiseNumbers` として `taskid.go` に移し、store はそれを呼ぶ。`go test ./internal/store/` で挙動が変わらないことを確かめる。

- [ ] **Step 4: repair.go を書く**

順番は spec の表どおり: `workspace_revived` → `id_prefix` → `parent_cycle` → `parent_deleted` → `workspace_mismatch` → `id_collision` → `id_renumbered` → `prefix_duplicate` → `name_duplicate`。

- 最初に `Files` の `workspaces/` と `tasks/` をすべて読み、`ws map[ULID]*model.Workspace`、`tasks map[ULID]*model.Task`（読めないものは `broken map[path][]byte`）を作る。書き換えるたびに `dirty[path] = true` とし、最後に dirty なものを `Encode` して返す（`updated` は `in.Now`）
- `workspace_revived`: タスクの `Workspace` が `ws` にない。`Base` にそのワークスペースのファイルがあれば戻し、`Author` は `in.Author(ワークスペースのパス)`（消した側の最後の変更）。Base になければ直さない
- `id_prefix`: `model.ValidIDPrefix` が失敗するワークスペース。Base の値が正しければ `IDPrefix` と `PrefixSet` を Base の値に戻す
- `parent_cycle`: `parent` をたどって循環を見つける（ULID の順に始点を選び、訪問済みを使う）。循環の中で `in.Changed(path)` が真のタスクのうち `Updated`、`Author`、ULID が最大のもの。Base があり、Base のそのタスクの parent に戻して循環が解けるならそれ、でなければ parent を nil に。rank はルートの末尾（ルートのタスクの最大の rank と `""` の `model.Between`）。解けるまで繰り返す
- `parent_deleted`: `DeletedAt == nil` で、parent がない・`DeletedAt != nil` のタスクを、ルートの末尾へ
- `workspace_mismatch`: parent のワークスペースと違う子（とその配下、ULID の順に深さ優先）の `Workspace` を親のものに
- `id_collision`: 削除済みを含む全タスクを `ID` でまとめ、2つ以上の組ごとに、`OnOrigin(ulid, id)` が真のタスクがちょうど1つならそれ、でなければ ULID の最小を残す。ほかは「振り直す」印を付ける
- 番号を振る: 使用済み番号を、全タスクの `CountIDs`、`broken` の中身の `model.IDNumbersIn`、`Files.Read(model.NumbersPath)` の `RaiseNumbers` で数える。振り直す印のあるタスクと、`ParseTaskID(ID)` の接頭辞が所属ワークスペースの `IDPrefix` と違う（または ID がない）タスクを、組と ULID の順に並べて番号を振る。古い ID は `FormerIDs` の末尾に足す（重複は足さない）。所属ワークスペースの接頭辞が不正なタスクは振らない
- `Repaired` の `Author` は `in.Author(そのタスクのパス)`。`id_renumbered` は、同じタスクに `id_collision` か `workspace_mismatch` が出ていれば出さず、ほかは Author を空にして出す
- `prefix_duplicate`・`name_duplicate`: ワークスペースを `IDPrefix`（名前は小文字）でまとめ、後の側（`PrefixSet`、名前は `Created`、同じなら ULID の大きい側）ごとに1件。データは変えない

- [ ] **Step 5: テストを通す**

Run: `go test ./internal/merge/ ./internal/store/`
Expected: PASS

- [ ] **Step 6: コミット**

```bash
git add internal/merge internal/model internal/store
git commit -m "feat(merge): マージ後の修復（ID の衝突の振り直しを含む）を作り、番号の数え方を model で共有する"
```

---

### Task 9: store — CommitMerge と、ワークスペースの重複の検査を変えたフィールドに限る

**Files:**
- Create: `internal/store/merge.go`, `internal/store/merge_test.go`
- Modify: `internal/store/validate.go`, `internal/store/workspace_test.go`

**Interfaces:**
- Consumes: Task 2 の `Commit.Parents`、`Head`
- Produces:
  - `var ErrHeadMoved = errors.New("HEAD moved while the merge was computed")`
  - `func (s *Store) CommitMerge(head plumbing.Hash, parents []plumbing.Hash, files map[string][]byte, op Op, trailers []gitrepo.Trailer) (plumbing.Hash, error)` — ロックの中で、HEAD が head と違えば `ErrHeadMoved`。files（nil は削除）を書き、`Commit{Parents: parents}`。検査も採番もしない。`trailers` は `Hedgehug-Op`・`Hedgehug-Author` の後に足す（`Hedgehug-Overwritten` など）。件名は `hedgehug: <op.Name> <op.Detail>`
  - `func (s *Store) FastForward(head, to plumbing.Hash, paths []string) error` — ロックの中で HEAD を確かめて `repo.FastForward`

- [ ] **Step 1: テストを書く**

```go
func TestCommitMerge(t *testing.T) {
	s := newTestStore(t)
	head, _ := s.repo.Head()
	files := map[string][]byte{"notes/a.md": []byte("a\n")}
	h, err := s.CommitMerge(head, []plumbing.Hash{head}, files, Op{Name: "repair", Detail: "1 issues", Author: testAuthor, Account: "github.com/a"}, []gitrepo.Trailer{{Key: "Hedgehug-Repaired", Value: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	c := headLog(t, s.dir)[0]
	if c.Hash != h || !strings.HasPrefix(c.Message, "hedgehug: repair 1 issues\n") || !strings.Contains(c.Message, "Hedgehug-Repaired: x") {
		t.Errorf("commit = %s %q", c.Hash, c.Message)
	}
	if _, err := s.CommitMerge(head, []plumbing.Hash{head}, files, Op{Name: "repair", Author: testAuthor}, nil); !errors.Is(err, ErrHeadMoved) {
		t.Errorf("stale head = %v", err)
	}
}

func TestCommitMergeSkipsValidation(t *testing.T) {
	s := newTestStore(t)
	head, _ := s.repo.Head()
	x := model.NewULID()
	bad := []byte("---\nulid: " + x.String() + "\nid: TST-1\nworkspace: " + model.NewULID().String() + "\nkind: task\ntitle: x\nstatus: todo\n---\n")
	if _, err := s.CommitMerge(head, []plumbing.Hash{head}, map[string][]byte{model.TaskPath(x): bad}, Op{Name: "merge", Author: testAuthor}, nil); err != nil {
		t.Errorf("a merge with a broken invariant was refused: %v", err)
	}
}
```

`workspace_test.go`:

```go
func TestRenameWorkspaceWithDuplicatePrefixFromSync(t *testing.T) {
	s := newTestStore(t)
	a := testWorkspace(t, s) // prefix TST
	b := &model.Workspace{ULID: model.NewULID(), Name: "b", IDPrefix: "TST", PrefixSet: time.Now()}
	mustWrite(t, s, model.WorkspacePath(b.ULID), string(mustEncode(t, b))) // as a merge would leave it
	b.Name = "bee"
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(b) }); err != nil {
		t.Errorf("rename of a workspace with a duplicate prefix was refused: %v", err)
	}
	b.IDPrefix = a.IDPrefix
	b.Name = a.Name
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(b) }); err == nil {
		t.Error("taking the name of another workspace was allowed")
	}
}
```

（`mustWrite` は既存の補助で、ファイルを書いてコミットする。`mustEncode` は `Encode` してエラーで失敗する補助を足す。）

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/store/ -run 'CommitMerge|DuplicatePrefixFromSync'`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `merge.go`: `Update` と同じロック（`s.mu` と `acquire`）を取り、`s.repo.Head()` を比べ、`change` を作って `s.apply`、`s.commit(gitrepo.Commit{...Parents})`。失敗したら `rollback`。schema.yaml を書いたら `s.schema` を読み直す
- `validate.go` の `validateWorkspaces`: 各ワークスペースについて、ディスクの今の値（`tx.s.Workspace(ulid)`、なければ新規）と比べて、`IDPrefix` が変わったときだけ接頭辞の重複を、`Name` が変わったときだけ名前の重複を調べる。形式の検査（`ValidIDPrefix`、`ValidWorkspaceName`）は今までどおり常に行う

- [ ] **Step 4: テストを通す**

Run: `go test ./internal/store/ ./cmd/hedgehug/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/store
git commit -m "feat(store): マージコミットを書く CommitMerge を足し、ワークスペースの重複は変えたフィールドだけ検査する"
```

---
### Task 10: sync — 履歴から各側を作り、1回の同期（Run）を回す

**Files:**
- Create: `internal/sync/history.go`, `internal/sync/sync.go`, `internal/sync/state.go`, `internal/sync/sync_test.go`, `internal/sync/helpers_test.go`

**Interfaces:**
- Consumes: Task 1・2 の gitrepo、Task 6〜8 の `merge.Merge`・`merge.Repair`、Task 9 の `CommitMerge`・`Store.FastForward`
- Produces:

```go
var ErrDirty = errors.New("the store has uncommitted changes where the sync would write; check them with hedgehug doctor, then commit or restore them and run hedgehug sync")

type Options struct {
	Remotes  []string          // config.yaml stores[].remotes
	Author   gitrepo.Signature // for merge and repair commits
	Account  string            // the writer's account id; empty if none
	Me       []string          // all accounts, for the inbox
	Now      func() time.Time
	Others   []OtherStore       // the other stores in config, for prefix duplicates across stores
	BeforePush func()           // a test hook: another clone pushes here
}

type OtherStore struct{ Name, Path string }

type Result struct {
	Merges      int
	Overwritten []merge.Overwritten
	Repaired    []merge.Repaired
	Invalid     []model.ULID // tasks left with problems (is:invalid)
	Pushed      []string     // remote names
	Notices     []inbox.Item // made in step 7 (Task 11 fills this in)
}

// Run syncs the store once. A store with no remote does nothing.
func Run(s *store.Store, opt Options) (Result, error)

// sides builds the two Sides of path for a merge of ours and theirs over base.
func sides(r *gitrepo.Repo, base, ours, theirs plumbing.Hash, path string) (b []byte, o, t merge.Side, err error)
```

`state.go`:

```go
type State struct {
	PendingSince  time.Time `json:"pending_since,omitzero"`
	LastAttempt   time.Time `json:"last_attempt,omitzero"`
	LastSuccess   time.Time `json:"last_success,omitzero"`
	LastError     string    `json:"last_error,omitempty"`
	ErrorReported bool      `json:"error_reported,omitempty"`
}

// UpdateState reads sync.json, lets fn change it and writes it back, under
// .hedgehug/local/state.lock.
func UpdateState(s *store.Store, fn func(*State)) (State, error)
func ReadState(s *store.Store) (State, error)
```

`store` の `acquire` は非公開なので、`store` に `func (s *Store) WithLocalLock(name string, wait time.Duration, fn func() error) error`（`.hedgehug/local/<name>` のファイルロック）を足して使う。

- [ ] **Step 1: テストの補助を書く**（`helpers_test.go`）

```go
// cluster is a bare remote and n clones of one store, each with its own
// writer account.
type cluster struct {
	t      *testing.T
	remote string
	stores []*store.Store
	opts   []Options
}

func newCluster(t *testing.T, n int) *cluster {
	t.Helper()
	c := &cluster{t: t, remote: filepath.Join(t.TempDir(), "r.git")}
	if err := gitrepo.InitBare(c.remote); err != nil {
		t.Fatal(err)
	}
	first, err := store.Create(filepath.Join(t.TempDir(), "s0"), &model.StoreMeta{ULID: model.NewULID(), Name: "s"}, nil,
		&model.Workspace{ULID: model.NewULID(), Name: "web", IDPrefix: "WEB"}, gitrepo.Signature{Name: "u0", Email: "u0@x"})
	if err != nil {
		t.Fatal(err)
	}
	c.add(first, 0)
	if _, err := Run(first, c.opts[0]); err != nil { // the first push
		t.Fatal(err)
	}
	for i := 1; i < n; i++ {
		dir := filepath.Join(t.TempDir(), fmt.Sprint("s", i))
		if _, err := gitrepo.Clone(c.remote, dir); err != nil {
			t.Fatal(err)
		}
		s, err := store.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		c.add(s, i)
	}
	return c
}

func (c *cluster) add(s *store.Store, i int) {
	acct := fmt.Sprintf("github.com/u%d", i)
	c.stores = append(c.stores, s)
	c.opts = append(c.opts, Options{
		Remotes: []string{c.remote}, Author: gitrepo.Signature{Name: fmt.Sprint("u", i), Email: fmt.Sprintf("u%d@users.noreply.github.com", i)},
		Account: acct, Me: []string{acct}, Now: time.Now,
	})
}

func (c *cluster) sync(i int) Result {
	c.t.Helper()
	r, err := Run(c.stores[i], c.opts[i])
	if err != nil {
		c.t.Fatalf("sync %d: %v", i, err)
	}
	return r
}

func (c *cluster) op(i int, name string, task model.ULID) store.Op {
	return store.Op{Name: name, Task: task, Author: c.opts[i].Author, Account: c.opts[i].Account}
}

// addTask creates a root task in the first workspace of clone i.
func (c *cluster) addTask(i int, title string) *model.Task {
	c.t.Helper()
	s := c.stores[i]
	var ws model.ULID
	for w, err := range s.Workspaces() {
		if err == nil {
			ws = w.ULID
			break
		}
	}
	x := &model.Task{ULID: model.NewULID(), Workspace: ws, Kind: "task", Title: title, Status: "todo", Rank: "a0"}
	if _, err := s.Update(c.op(i, "add", x.ULID), func(tx *store.Tx) error { return tx.PutTask(x) }); err != nil {
		c.t.Fatal(err)
	}
	return x
}

func (c *cluster) set(i int, ulid model.ULID, fn func(*model.Task)) {
	c.t.Helper()
	s := c.stores[i]
	if _, err := s.Update(c.op(i, "update", ulid), func(tx *store.Tx) error {
		x, err := tx.Task(ulid)
		if err != nil {
			return err
		}
		fn(x)
		return tx.PutTask(x)
	}); err != nil {
		c.t.Fatal(err)
	}
}

// task reads the task from clone i; nil when it is not there.
func (c *cluster) task(i int, ulid model.ULID) *model.Task {
	x, err := c.stores[i].Task(ulid)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		c.t.Fatal(err)
	}
	return x
}

func (c *cluster) headTree(i int) plumbing.Hash {
	h, err := c.stores[i].Repo().HeadTree()
	if err != nil {
		c.t.Fatal(err)
	}
	return h
}

func mustHead(t *testing.T, s *store.Store) plumbing.Hash {
	t.Helper()
	h, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	return h
}
```

`Store.Dir() string` がなければ足す（作業ツリーの絶対パス）。

- [ ] **Step 2: テストを書く**（`sync_test.go`）

```go
func TestFastForward(t *testing.T) {
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	c.sync(0)
	c.sync(1)
	if got := c.task(1, x.ULID); got.Title != "x" {
		t.Errorf("clone 1 has %v", got)
	}
}

func TestMergeBothSidesAndConverge(t *testing.T) {
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	c.sync(0)
	c.sync(1)
	c.set(0, x.ULID, func(t *model.Task) { t.Title = "from 0" })
	c.set(1, x.ULID, func(t *model.Task) { t.Status = "doing" })
	c.sync(0)
	r := c.sync(1)
	c.sync(0)
	if r.Merges != 1 {
		t.Errorf("merges = %d", r.Merges)
	}
	got := c.task(0, x.ULID)
	if got.Title != "from 0" || got.Status != "doing" {
		t.Errorf("merged = %s %s", got.Title, got.Status)
	}
	if c.headTree(0) != c.headTree(1) {
		t.Error("clones differ")
	}
}

func TestOverwrittenTrailer(t *testing.T) {
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	c.sync(0)
	c.sync(1)
	c.set(0, x.ULID, func(t *model.Task) { t.Title = "a" })
	c.set(1, x.ULID, func(t *model.Task) { t.Title = "b" }) // later: wins
	c.sync(0)
	r := c.sync(1)
	if len(r.Overwritten) != 1 || r.Overwritten[0].Author != "github.com/u0" || r.Overwritten[0].Value != `"a"` {
		t.Errorf("overwritten = %+v", r.Overwritten)
	}
	e, _ := c.stores[1].Repo().LogFrom(mustHead(t, c.stores[1]), 1)
	if v, ok := e[0].Trailer("Hedgehug-Overwritten"); !ok || !strings.HasPrefix(v, model.TaskPath(x.ULID)+" title \"a\" github.com/u0") {
		t.Errorf("trailer = %q", v)
	}
}

func TestIDCollisionRenumbersUnpushedSide(t *testing.T) {
	c := newCluster(t, 2)
	a := c.addTask(0, "a") // WEB-1 in both clones
	b := c.addTask(1, "b")
	if a.ID != "WEB-1" || b.ID != "WEB-1" {
		t.Fatalf("ids %s %s", a.ID, b.ID)
	}
	c.sync(0) // a reaches origin first
	r := c.sync(1)
	if got := c.task(1, b.ULID); got.ID != "WEB-2" || !slices.Contains(got.FormerIDs, "WEB-1") {
		t.Errorf("b = %s %v", got.ID, got.FormerIDs)
	}
	if got := c.task(1, a.ULID); got.ID != "WEB-1" {
		t.Errorf("a = %s", got.ID)
	}
	if len(r.Repaired) != 1 || r.Repaired[0].Kind != "id_collision" || r.Repaired[0].Author != "github.com/u1" {
		t.Errorf("repaired = %+v", r.Repaired)
	}
	c.sync(0)
	if c.headTree(0) != c.headTree(1) {
		t.Error("clones differ")
	}
}

func TestPushRejectedRetries(t *testing.T) {
	c := newCluster(t, 3)
	c.addTask(0, "a")
	c.addTask(2, "c")
	pushed := false
	c.opts[0].BeforePush = func() {
		if !pushed {
			pushed = true
			c.sync(2) // another clone pushes between our merge and our push
		}
	}
	r := c.sync(0)
	if !slices.Contains(r.Pushed, "origin") {
		t.Errorf("pushed = %v", r.Pushed)
	}
	c.sync(1)
	c.sync(2)
	if c.headTree(0) != c.headTree(1) || c.headTree(1) != c.headTree(2) {
		t.Error("clones differ")
	}
}

func TestLocalWriteDuringPushIsKept(t *testing.T) {
	c := newCluster(t, 1)
	c.addTask(0, "a")
	var during *model.Task
	c.opts[0].BeforePush = func() { during = c.addTask(0, "during") } // the store lock is free here
	c.sync(0)
	c.opts[0].BeforePush = nil
	c.sync(0)
	if got := c.task(0, during.ULID); got == nil {
		t.Error("the write during the push was lost")
	}
}

func TestDirty(t *testing.T) {
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	c.sync(0)
	c.sync(1)
	c.set(0, x.ULID, func(t *model.Task) { t.Title = "a" })
	c.sync(0)
	p := filepath.Join(c.stores[1].Dir(), filepath.FromSlash(model.TaskPath(x.ULID)))
	if err := os.WriteFile(p, []byte("hand edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := c.headTree(1)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrDirty) {
		t.Errorf("Run = %v", err)
	}
	if c.headTree(1) != before {
		t.Error("a dirty sync changed HEAD")
	}
}

func TestSetupAddsGitattributes(t *testing.T) {
	c := newCluster(t, 1)
	b, err := os.ReadFile(filepath.Join(c.stores[0].Dir(), ".gitattributes"))
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range []string{"tasks/**/*.md merge=hedgehug", "comments/**/*.md merge=hedgehug", "worklogs/**/*.md merge=hedgehug", "workspaces/*.md merge=hedgehug", "*.yaml merge=hedgehug"} {
		if !strings.Contains(string(b), l) {
			t.Errorf(".gitattributes lacks %q", l)
		}
	}
	cfg, _ := os.ReadFile(filepath.Join(c.stores[0].Dir(), ".git", "config"))
	if !strings.Contains(string(cfg), "driver = hedgehug merge-driver %O %A %B %P") {
		t.Errorf("driver not registered:\n%s", cfg)
	}
}

func TestMirrorPush(t *testing.T) {
	c := newCluster(t, 1)
	mirror := filepath.Join(t.TempDir(), "m.git")
	_ = gitrepo.InitBare(mirror)
	c.opts[0].Remotes = append(c.opts[0].Remotes, mirror)
	c.addTask(0, "x")
	r := c.sync(0)
	if !slices.Equal(r.Pushed, []string{"origin", "mirror1"}) {
		t.Errorf("pushed = %v", r.Pushed)
	}
}

func TestRepairCommitAfterHandMerge(t *testing.T) {
	c := newCluster(t, 1)
	a := c.addTask(0, "a")
	b := c.addTask(0, "b")
	// A hand-edited file gives b the id of a.
	c.set(0, b.ULID, func(t *model.Task) { t.ID = a.ID })  // store.Update keeps an id that fits the prefix
	r := c.sync(0)
	if len(r.Repaired) != 1 || r.Repaired[0].Kind != "id_collision" {
		t.Errorf("repaired = %+v", r.Repaired)
	}
	e, _ := c.stores[0].Repo().LogFrom(mustHead(t, c.stores[0]), 1)
	if op, _ := e[0].Trailer("Hedgehug-Op"); op != "repair" {
		t.Errorf("HEAD is not the repair commit: %+v", e[0])
	}
}

func TestPrefixDuplicateAcrossStores(t *testing.T) {
	for _, c := range []struct {
		name      string
		otherSet  time.Time
		wantNotice bool
	}{
		{"other set earlier", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), true},
		{"other set later", time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), false},
	} {
		t.Run(c.name, func(t *testing.T) {
			cl := newCluster(t, 1)
			dir := filepath.Join(t.TempDir(), "other")
			ws := &model.Workspace{ULID: model.NewULID(), Name: "o", IDPrefix: "WEB", PrefixSet: c.otherSet}
			if _, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "other"}, nil, ws, gitrepo.Signature{Name: "o", Email: "o@x"}); err != nil {
				t.Fatal(err)
			}
			cl.opts[0].Others = []OtherStore{{Name: "other", Path: dir}}
			r := cl.sync(0)
			got := slices.ContainsFunc(r.Repaired, func(x merge.Repaired) bool {
				return x.Kind == "prefix_duplicate" && strings.Contains(x.Value, `"store":"other"`)
			})
			if got != c.wantNotice {
				t.Errorf("notice = %v, want %v (%+v)", got, c.wantNotice, r.Repaired)
			}
		})
	}
}
```

（`store.Create` は `ws.PrefixSet` がゼロのときだけ今の時刻を入れる。今そうでなければ、Create をそう直す。）

- [ ] **Step 3: 失敗を確かめる**

Run: `go test ./internal/sync/`
Expected: FAIL

- [ ] **Step 4: history.go を書く**

- `sides`: `b = ReadAt(base, path)`、`o.Data = ReadAt(ours, path)`、`t.Data = ReadAt(theirs, path)`（ないなら nil）。`LastChange(ours, base, path)` から `o.Author`（`Hedgehug-Author`、なければ author のメール）と `o.Time`。yaml のファイル（`schema.yaml`、`automations.yaml`、`.hedgehug/store.yaml`）なら `o.When` を作る: `Between(ours, base)` のうち path を変えたコミットを古い順に読み、各版を yaml でキーのパスごとの値に平らにし（`[]string` を `\x00` でつないだキー）、値が前の版から変わったコミットの時刻を記録する。`When(path)` は記録した時刻を返す（変わっていなければゼロ時刻）
- `author(e LogEntry) string`: `Hedgehug-Author` の値、なければ `e.Author.Email`

- [ ] **Step 5: sync.go を書く**

`Run` の手順は spec の「1回の同期」どおり。要点:

```go
func Run(s *store.Store, opt Options) (Result, error) {
	var res Result
	if len(opt.Remotes) == 0 {
		return res, nil
	}
	r := s.Repo()
	if err := r.SetRemotes(opt.Remotes); err != nil {
		return res, err
	}
	if err := setup(s, opt); err != nil { // .gitattributes and the driver
		return res, err
	}
	wait := 500 * time.Millisecond
	for round := 0; ; round++ {
		for _, name := range r.Remotes() {
			if _, err := r.Fetch(name); err != nil {
				return res, fmt.Errorf("fetch %s: %w", name, err)
			}
		}
		for _, name := range r.Remotes() {
			if err := mergeRemote(s, name, opt, &res); err != nil {
				return res, err
			}
		}
		if err := repairHead(s, opt, &res); err != nil {
			return res, err
		}
		if opt.BeforePush != nil {
			opt.BeforePush()
		}
		err := r.Push("origin")
		if errors.Is(err, gitrepo.ErrRejected) && round < 5 {
			time.Sleep(wait)
			wait *= 2
			continue
		}
		if err != nil {
			return res, fmt.Errorf("push origin: %w", err)
		}
		res.Pushed = append(res.Pushed, "origin")
		for _, name := range r.Remotes()[1:] {
			if err := r.Push(name); err == nil {
				res.Pushed = append(res.Pushed, name)
			} else if !errors.Is(err, gitrepo.ErrRejected) {
				return res, fmt.Errorf("push %s: %w", name, err)
			}
			// A rejected mirror is merged in the next round; the rounds are capped.
		}
		if len(res.Pushed) == len(r.Remotes()) || round >= 5 {
			break
		}
	}
	// step 7 (inbox) is added in Task 11
	if n, err := r.LooseObjects(); err == nil && n > 6700 {
		if err := r.Repack(); err != nil {
			return res, err
		}
	}
	return res, nil
}
```

（Task 11 で、`setup` の前に `before, _ := r.Head()` を取り、repack の前に inbox の手順を足す。）

- `mergeRemote`: `head := r.Head()`、`remote := r.RemoteHead(name)`。ゼロ、または `IsAncestor(remote, head)` なら何もしない。`IsAncestor(head, remote)` なら、`DiffTrees(head の木, remote の木)` のパスで `checkDirty` してから `s.FastForward(head, remote, paths)`。それ以外は `base := MergeBase(head, remote)`、`DiffTrees` の両方に出たパスを `checkDirty`、schema.yaml を先にマージして `sc`（`schema.Decode`、失敗したら HEAD のスキーマ）、ほかのパスを `merge.Merge`。HEAD の木にマージの結果を重ねた `merge.Files` を作り（`Read` は結果にあればそれ、なければ HEAD の木から）、`merge.Repair` を `Base`（base の木）・`OnOrigin`（origin の main の木でタスクのファイルを読み ID を比べる）・`Changed`（マージで書いたパス）付きで呼ぶ。trailer を作って `s.CommitMerge(head, []plumbing.Hash{head, remote}, files, store.Op{Name: "merge", Detail: name, Author: opt.Author, Account: opt.Account}, trailers)`。`ErrHeadMoved` なら同じ remote でやり直す
- trailer: `Hedgehug-Overwritten: <path> <field> <value> <author>`、`Hedgehug-Repaired: <ulid> <kind> <value> <author>`（author が空なら `-`）
- `checkDirty(paths)`: `r.Status()` のパスと交われば `ErrDirty`
- `repairHead`: HEAD の木全体で `merge.Repair`（`Base` nil、`Changed` は常に true）。何か書くなら `CommitMerge(head, []plumbing.Hash{head}, …, Op{Name: "repair", Detail: fmt.Sprintf("%d issues", n)})`。何も書かないが通知だけある（`prefix_duplicate` など）場合はコミットせず、`res.Repaired` に入れる（Task 11 が inbox に出す）
- Store 間の接頭辞の重複: `repairHead` の後、`opt.Others` の各 Store を `store.Open` し（失敗は飛ばす）、そのワークスペースとこの Store のワークスペースの `IDPrefix` を比べ、この Store の側の `PrefixSet` が後なら `Repaired{Kind: "prefix_duplicate", Value: json({"store": 相手の名前, "workspace": 相手の名前})}`
- `setup`: `.gitattributes` に足りない行があれば `s.Update(store.Op{Name: "setup", Detail: "merge driver", ...}, ...)` で足す。`Tx` にファイルを書く口がなければ `Tx.PutFile(rel string, data []byte)`（Store の中の任意の追跡ファイル、`.hedgehug/local/` は拒否）を足す。driver の登録は `.git/config` を go-git の `config.Config.Raw` で `merge "hedgehug"` の節に `name` と `driver` を書く（`RegisterMergeDriver(r *gitrepo.Repo) error` を gitrepo に置く）

- [ ] **Step 6: state.go を書く**

`UpdateState` は `s.WithLocalLock("state.lock", 10*time.Second, ...)` の中で `ReadLocal("sync.json")` → fn → `WriteLocal`。`Run` の呼び出し側（CLI と background）が `LastAttempt`・`LastSuccess`・`LastError`・`PendingSince` を書く（Task 13・14）。

- [ ] **Step 7: テストを通す**

Run: `go test -race ./internal/sync/ ./internal/store/`
Expected: PASS

- [ ] **Step 8: コミット**

```bash
git add internal/sync internal/store internal/gitrepo
git commit -m "feat(sync): fetch・マージ・修復・push の1回の同期を作る"
```

---

### Task 11: inbox — 届いたコミットから通知を作り、保存する

**Files:**
- Create: `internal/inbox/inbox.go`, `internal/inbox/collect.go`, `internal/inbox/inbox_test.go`, `internal/inbox/collect_test.go`
- Modify: `internal/sync/sync.go`（手順 7）, `internal/sync/sync_test.go`

**Interfaces:**
- Produces:

```go
type Kind string

const (
	Mention         Kind = "mention"
	Assigned        Kind = "assigned"
	Watch           Kind = "watch"
	Overwritten     Kind = "overwritten"
	Revived         Kind = "revived"
	Repaired        Kind = "repaired"
	Renumbered      Kind = "renumbered"
	PrefixDuplicate Kind = "prefix_duplicate"
)

type Item struct {
	ULID    model.ULID      `json:"ulid"`
	Kind    Kind            `json:"kind"`
	Task    model.ULID      `json:"task,omitzero"`      // zero for a workspace item
	Path    string          `json:"path,omitempty"`     // the file, for overwritten
	Commit  string          `json:"commit"`
	Author  string          `json:"author"`             // who caused it
	Time    time.Time       `json:"time"`
	Detail  json.RawMessage `json:"detail,omitempty"`   // field and value, old and new id, repair kind...
}

// Box is the inbox of one store on this machine.
type Box struct{ s *store.Store }

func Open(s *store.Store) *Box
func (b *Box) Add(items []Item) ([]Item, error)   // drops duplicates; returns the ones added
func (b *Box) List(all bool) ([]Item, error)       // newest first; unread only unless all
func (b *Box) MarkRead(ulids []model.ULID) error
func (b *Box) MarkAllRead() error
func (b *Box) Item(ulid model.ULID) (Item, bool, error)

// Collect makes the items for me from the commits between before and after.
func Collect(s *store.Store, before, after plumbing.Hash, me []string) ([]Item, error)

// Import adds the merge driver's records in .hedgehug/local/overwritten/
// and removes the files.
func (b *Box) Import(me []string) error

// Record is what the merge driver writes for one overwritten value or notice.
type Record struct {
	Kind   Kind      `json:"kind,omitempty"` // empty means overwritten
	Path   string    `json:"path"`
	Field  string    `json:"field,omitempty"`
	Value  string    `json:"value,omitempty"`
	Author string    `json:"author"`
	Commit string    `json:"commit"`
	Time   time.Time `json:"time"`
}
```

重複の鍵: overwritten は `(Path, field, value, 負けた側)`、prefix_duplicate は `(workspace, 相手, kind)`、ほかは `(Commit, Kind, Task)`。鍵は `inbox.jsonl` の各行から計算する（別ファイルは持たない）。

- [ ] **Step 1: テストを書く**

`inbox_test.go`: `Add` で同じ鍵の Item を二度足しても1件、`List(false)` は未読だけを新しい順、`MarkRead` 後は `List(true)` にだけ出る。

`collect_test.go`（`internal/sync` のテスト補助は使えないので、`store.Create` と `gitrepo.Clone` で2つのクローンを作る小さな補助を置く）:

```go
func TestCollectMentionAssignWatch(t *testing.T) {
	a, b := twoClones(t) // accounts github.com/a and github.com/b
	x := addTask(t, a, "x", func(t *model.Task) { t.Watchers = []string{"github.com/b"} })
	syncBoth(t, a, b)
	before := head(t, b)
	update(t, a, x.ULID, func(t *model.Task) {
		t.Body = "ping @b and `@b` in code\n"
		t.Assignees = []string{"github.com/b"}
		t.Status = "doing"
	})
	syncBoth(t, a, b)
	items, err := Collect(b.s, before, head(t, b), []string{"github.com/b"})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[Kind]int{}
	for _, it := range items {
		kinds[it.Kind]++
	}
	if kinds[Mention] != 1 || kinds[Assigned] != 1 || kinds[Watch] != 1 {
		t.Errorf("kinds = %v", kinds) // the mention in inline code does not count
	}
}

func TestCollectSkipsOwnCommits(t *testing.T) {
	a, _ := twoClones(t)
	before := head(t, a)
	addTask(t, a, "x", func(t *model.Task) { t.Assignees = []string{"github.com/a"} })
	if items, _ := Collect(a.s, before, head(t, a), []string{"github.com/a"}); len(items) != 0 {
		t.Errorf("own write made %v", items)
	}
}

func TestCollectOverwrittenAndRenumberedFromOwnMerge(t *testing.T) {
	_, b := twoClones(t)
	x := addTask(t, b, "x", nil)
	before := head(t, b)
	p := model.TaskPath(x.ULID)
	trailers := []gitrepo.Trailer{
		{Key: "Hedgehug-Overwritten", Value: p + ` title "mine" github.com/b`},
		{Key: "Hedgehug-Repaired", Value: x.ULID.String() + ` id_collision {"old":"WEB-1","new":"WEB-2"} github.com/b`},
		{Key: "Hedgehug-Overwritten", Value: p + ` status "todo" github.com/a`}, // someone else's: no item
	}
	h := head(t, b)
	if _, err := b.s.CommitMerge(h, []plumbing.Hash{h}, map[string][]byte{"notes.md": []byte("n\n")},
		store.Op{Name: "merge", Detail: "origin", Author: gitrepo.Signature{Name: "b", Email: "b@x"}, Account: "github.com/b"}, trailers); err != nil {
		t.Fatal(err)
	}
	items, err := Collect(b.s, before, head(t, b), []string{"github.com/b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Kind != Overwritten || items[1].Kind != Renumbered || string(items[1].Detail) != `{"old":"WEB-1","new":"WEB-2"}` {
		t.Errorf("items = %+v", items)
	}
}

func TestImportMergeDriverRecords(t *testing.T) {
	_, b := twoClones(t)
	rec := `{"path":"tasks/0A/P0/01JBZ8K3QW7M2X9V4T6R5N8P0A.md","field":"title","value":"\"mine\"","author":"github.com/b","commit":"","time":"2026-10-31T10:00:00Z"}`
	name := "overwritten/" + model.NewULID().String() + ".json"
	if err := b.s.WriteLocal(name, []byte(rec)); err != nil {
		t.Fatal(err)
	}
	box := Open(b.s)
	if err := box.Import([]string{"github.com/b"}); err != nil {
		t.Fatal(err)
	}
	items, _ := box.List(false)
	if len(items) != 1 || items[0].Kind != Overwritten {
		t.Errorf("items = %+v", items)
	}
	if _, err := b.s.ReadLocal(name); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the record is left: %v", err)
	}
}
```

`WriteLocal` / `ReadLocal` が `overwritten/` のような下のディレクトリの名前を受け付けなければ、`validLocalName` を「`/` 区切りで、`..` を含まない」に広げる。merge driver が書く記録の形（上の JSON）は、Task 13 の merge driver と共有する `inbox.Record` 型として `inbox.go` に置く。

sync の手順 7 のテスト（`sync_test.go`）: `TestOverwrittenTrailer` の続きで、`inbox.Open(c.stores[0]).List(false)` に u0 の `Overwritten` が1件届く（u0 の次の同期で）。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/inbox/ ./internal/sync/`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `Collect`: `r.Between(after, before)` の各コミットについて
  - trailer の `Hedgehug-Overwritten` / `Hedgehug-Repaired`: 対象の author が me に入れば作る（コミットの author が自分でも）。Repaired の kind で `Repaired` / `Renumbered`（`id_collision`）/ `PrefixDuplicate`（`prefix_duplicate`・`name_duplicate`）。`id_renumbered` は作らない
  - それ以外は、`Hedgehug-Author` が me に入るコミットを飛ばす。親（first-parent）と比べて変わったタスクのファイルを読み、`assignees` に me が足された → Assigned、本文で新しく足されたメンションが me に当たる → Mention、`watchers` に me がいて status / assignees が変わった → Watch。コメントのファイルが新しく足された: 本文のメンション → Mention、タスクの watchers に me → Watch
  - メンションの抽出は `index` の `Mentions(body string) []string`（Task 12 で作る。この Task では先に `internal/index/refs.go` に関数だけ置き、Task 12 でインデックスに載せる）
  - 当たり判定: `@login` は me のうち `/login` で終わるもの、`@host/login` は完全一致
- `.hedgehug/local/overwritten/*.json`: `func (b *Box) Import(me []string) error` が、author が me に入る記録を Item にして `Add` し、`Add` が済んでからファイルを消す（me に入らない記録も消す。merge driver は誰の値かを問わず書くため）。`Collect` はこのファイルを読まない。sync の手順 7 と `hh inbox` が `Import` を呼ぶ
- Notice{Revived}: マージコミットに trailer として残すため、Task 10 の trailer に `Hedgehug-Repaired: <ulid> revived - <author>` として入れる（spec の Notice はマージの結果。Repaired と同じ trailer に `revived` の種類で載せる）。`Collect` は kind `revived` を `Revived` にする
- sync の手順 7: `Run` の最後で `items := Collect(s, before, head, opt.Me)`、`inbox.Open(s).Add(items)`、`res.Notices` に足す。同期で作った「データを変えない通知」（`repairHead` のコミットしない `prefix_duplicate`、Store 間の重複）は、コミットがないので直接 Item にして足す（Commit は HEAD、重複の鍵が同じなら二度出ない）

- [ ] **Step 4: テストを通す**

Run: `go test -race ./internal/inbox/ ./internal/sync/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/inbox internal/sync internal/index
git commit -m "feat(inbox): 同期で届いたコミットからメンション・割り当て・ウォッチ・上書き・修復の通知を作る"
```

---

### Task 12: index と query — コメント・作業時間、メンション、#ID、バックリンク、@me、mentions:、watching:、is:overwritten

**Files:**
- Create: `internal/index/comments.go`, `internal/index/comments_test.go`, `internal/index/refs_test.go`
- Modify: `internal/index/refs.go`（Task 11 の `Mentions` に足す）, `internal/index/entry.go`, `internal/index/build.go`（cacheFormat を上げる）, `internal/index/index.go`, `internal/query/eval.go`, `internal/query/eval_test.go`

**Interfaces:**
- Produces:
  - `Entry.Watchers []string`、`Entry.Mentions []string`（本文と削除されていないコメントの `@...` を、書かれたまま `login` か `host/login` で）、`Entry.Refs []string`（`#<ID>` の ID）
  - `func Mentions(body string) []string` — コードブロック（```）とインラインコード（`）の中を除く
  - `func TaskRefs(body string) []string` — `#WEB-42` の形
  - `type CommentEntry struct{ ULID model.ULID; Author string; Created, Updated time.Time; DeletedAt *time.Time }`、`func (ix *Index) Comments(task model.ULID) []CommentEntry`、`func (ix *Index) Worklogs(task model.ULID) []WorklogEntry`
  - `func (ix *Index) Backlinks(ulid model.ULID) []*Entry` — `Refs` の ID（今の ID と古い ID の両方で引く）がこのタスクを指すもの。ULID の順
  - query: `Options.Me`（cmd が `config.Me()` を入れる）、`Options.Overwritten map[model.ULID]bool`（cmd が inbox の未読の overwritten から作る）、`mentions:<account|@me>`、`watching:<account|@me>`、`is:overwritten`

- [ ] **Step 1: テストを書く**

`refs_test.go`:

```go
func TestMentionsSkipCode(t *testing.T) {
	body := "hi @alice and @github.com/bob\n`@carol`\n```\n@dave\n```\nmail a@b.c\n"
	if got := Mentions(body); !slices.Equal(got, []string{"alice", "github.com/bob"}) {
		t.Errorf("Mentions = %v", got)
	}
}

func TestTaskRefs(t *testing.T) {
	if got := TaskRefs("see #WEB-42, #api-3 and #12\n"); !slices.Equal(got, []string{"WEB-42"}) {
		t.Errorf("TaskRefs = %v", got)
	}
}

func TestBacklinksByFormerID(t *testing.T) {
	s, ix := newIndexedStore(t) // helper in helpers_test.go
	a := addTask(t, s, "a")
	addTask(t, s, "b", func(x *model.Task) { x.Body = "after #" + a.ID + "\n" })
	renumber(t, s, a.ULID) // workspace prefix change: a gets a new id, the old in former_ids
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}
	if bl := ix.Backlinks(a.ULID); len(bl) != 1 || bl[0].Title != "b" {
		t.Errorf("Backlinks = %v", bl)
	}
}
```

`index_test.go` に、1c からの持ち越しのテストを足す: `update`（差分の更新）の途中でファイルの読み込みを失敗させる（テスト用のフック `readHook` を `build.go` に置く）と `Refresh` はエラーを返し、その後に `Open` し直したインデックスが全件の再構築と同じ `Tasks()` を返す。

`comments_test.go`: コメントを足した後の `Refresh` で `Comments(task)` に載る、削除済みのコメントのメンションは `Entry.Mentions` に入らない、キャッシュから読み直しても同じ（`Open` し直して比べる）。

`eval_test.go` に: `mentions:@me`（コメントでのメンション、`@login` が `github.com/login` に当たる）、`watching:github.com/b`、`is:overwritten`（`Options.Overwritten` にある ULID だけ）、`assignee:@me` が `Options.Me` を使う。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/index/ ./internal/query/`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `build.go`/`index.go`: `update`（HEAD の差分）で `comments/` と `worklogs/` のパスも見て、変わったタスクの `Comments`・`Worklogs`・`Mentions` を作り直す。全件の再構築でも同じ。`cacheFormat` を1つ上げる
- `Entry.Mentions` は本文の `Mentions` と、削除されていないコメントの本文の `Mentions` の和（重複なし、出た順）
- query: `mentions:` の値が `@me` なら `Options.Me`、ほかはその値。比べ方は inbox と同じ（`@login` は host を問わず login、`host/login` は完全一致）。`watching:` は `Entry.Watchers` に値が入るか。`is:overwritten` は `Options.Overwritten[e.ULID]`
- 1c の spec の「フェーズ2」の項目を、これで満たす

- [ ] **Step 4: テストを通す**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/index internal/query
git commit -m "feat(index,query): コメントのインデックス・メンション・#ID のバックリンクと mentions:・watching:・is:overwritten を足す"
```

---

### Task 13: CLI — sync、store init --remote、store add / list / remove、merge driver

**Files:**
- Create: `cmd/hedgehug/sync.go`, `cmd/hedgehug/store_add.go`, `cmd/hedgehug/merge_driver.go`, `cmd/hedgehug/sync_test.go`, `cmd/hedgehug/store_add_test.go`, `cmd/hedgehug/merge_driver_test.go`
- Modify: `cmd/hedgehug/store_init.go`, `cmd/hedgehug/app.go`（`syncOptions`）, `cmd/hedgehug/helpers_test.go`, `cmd/hedgehug/ls.go`（`Options.Me`・`Overwritten`）

**Interfaces:**
- Consumes: Task 3 の config、Task 10 の `sync.Run`・`State`、Task 11 の inbox
- Produces:
  - `func (a *app) syncOptions(c *config.Config, e config.StoreEntry) (sync.Options, error)`
  - `func (a *app) runSync(s *store.Store, opt sync.Options) (sync.Result, error)` — `sync.lock` を取り、`Run`、`State`（`LastAttempt`・`LastSuccess`・`LastError`、成功したら `PendingSince` を、同期を始めた時刻より前なら空に）を書く
  - `hh sync [--status]`、`hh store init <name> --prefix <p> [--remote <url>]...`、`hh store add <name> --remote <url>... [--path <dir>]`、`hh store list`、`hh store remove <name> [--purge]`、隠しコマンド `hedgehug merge-driver <base> <ours> <theirs> <path>`

- [ ] **Step 1: テストを書く**

`helpers_test.go` に、bare remote を作る `te.bare(name string) string` と、同じ remote を見る2人目の利用者 `te.other()`（別の HEDGEHUG_HOME と HOME）を足す。

`sync_test.go`:

```go
func TestSyncTwoUsers(t *testing.T) {
	alice := newTestEnv(t)
	remote := alice.bare("shared")
	alice.writeAccounts("github.com", "alice")
	alice.ok("store", "init", "org", "--prefix", "ORG", "--remote", remote)
	alice.ok("add", "x")
	alice.ok("sync")
	bob := alice.other()
	bob.writeAccounts("github.com", "bob")
	bob.ok("store", "add", "org", "--remote", remote)
	if out := bob.ok("ls"); !strings.Contains(out, "ORG-1") {
		t.Errorf("bob ls = %s", out)
	}
	alice.ok("set", "ORG-1", "title=from alice")
	bob.ok("status", "ORG-1", "doing")
	alice.ok("sync")
	bob.ok("sync")
	alice.ok("sync")
	for _, te := range []*testEnv{alice, bob} {
		out := te.ok("show", "ORG-1", "--json")
		if !strings.Contains(out, `"title": "from alice"`) || !strings.Contains(out, `"status": "doing"`) {
			t.Errorf("show = %s", out)
		}
	}
}

func TestSyncShowsRenumbered(t *testing.T) {
	alice, bob, _ := twoUsers(t) // both on one remote, synced once
	alice.ok("add", "a")
	bob.ok("add", "b")
	alice.ok("sync")
	out := bob.ok("sync")
	if !regexp.MustCompile(`ORG-2 \([0-9A-Z]{26}\) → ORG-3`).MatchString(out) {
		t.Errorf("sync output = %s", out)
	}
}

func TestSyncStatus(t *testing.T) {
	alice, _, _ := twoUsers(t)
	alice.ok("add", "a")
	out := alice.ok("sync", "--status")
	if !strings.Contains(out, "1 commit not pushed") {
		t.Errorf("status = %s", out)
	}
	alice.ok("sync")
	if out := alice.ok("sync", "--status", "--json"); !strings.Contains(out, `"unpushed": 0`) {
		t.Errorf("status json = %s", out)
	}
}

func TestStoreInitRemoteNotEmpty(t *testing.T) {
	alice, _, remote := twoUsers(t)
	code, _, errs := alice.run("store", "init", "again", "--prefix", "AG", "--remote", remote)
	if code != 1 || !strings.Contains(errs, "hedgehug store add") {
		t.Errorf("code %d, stderr %s", code, errs)
	}
}

func TestStoreAddWarnsPrefixDuplicate(t *testing.T) {
	alice, _, remote := twoUsers(t) // the shared store uses ORG
	carol := alice.other()
	carol.ok("store", "init", "mine", "--prefix", "ORG") // later than the shared ORG
	code, out, errs := carol.run("store", "add", "org", "--remote", remote)
	if code != 0 || !strings.Contains(errs, "ORG is also the id prefix of workspace") || !strings.Contains(errs, "hedgehug workspace prefix") {
		t.Errorf("code %d out %s errs %s", code, out, errs)
	}
}

func TestStoreListAndRemove(t *testing.T) {
	alice, _, _ := twoUsers(t)
	out := alice.ok("store", "list")
	if !strings.Contains(out, "org") || !strings.Contains(out, "shared.git") {
		t.Errorf("list = %s", out)
	}
	alice.ok("add", "unpushed")
	if code, _, errs := alice.run("store", "remove", "org", "--purge"); code != 1 || !strings.Contains(errs, "hedgehug sync") {
		t.Errorf("purge with unpushed commits: %d %s", code, errs)
	}
	alice.ok("store", "remove", "org")
	if out := alice.ok("store", "list"); strings.Contains(out, "org") {
		t.Errorf("list after remove = %s", out)
	}
}
```

`merge_driver_test.go`（本物の git を使う。なければ skip）:

```go
func TestMergeDriverWithGitPull(t *testing.T) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	// Build the binary once (TestMain may share it) and put it on PATH as hedgehug.
	bin := buildHedgehug(t)
	alice, bob, _ := twoUsers(t)
	alice.ok("set", "ORG-1", "title=alice")
	alice.ok("sync")
	bob.ok("status", "ORG-1", "doing")
	dir := bob.storeDir("org")
	cmd := exec.Command(gitBin, "-c", "user.name=bob", "-c", "user.email=bob@x", "pull", "--no-rebase", "origin", "main")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(bin)+string(os.PathListSeparator)+filepath.Dir(gitBin), "HEDGEHUG_AUTO_SYNC=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git pull: %v\n%s", err, out)
	}
	out := bob.ok("show", "ORG-1", "--json")
	if !strings.Contains(out, `"title": "alice"`) || !strings.Contains(out, `"status": "doing"`) {
		t.Errorf("after git pull: %s", out)
	}
}
```

`twoUsers`・`buildHedgehug`（`go build -o <tmp>/hedgehug ./` を1回）・`storeDir` は `helpers_test.go` に足す。`twoUsers` は `TestSyncTwoUsers` の前半（alice が init して add・sync、bob が store add）をまとめたもの。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./cmd/hedgehug/ -run 'Sync|StoreInitRemote|StoreAdd|StoreList|MergeDriver'`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `syncOptions`: `Remotes` は `e.Remotes`、author と account は Task 4 の `writer`/`CommitAuthor`、`Me` は `c.Me()`、`Others` は `c.Stores` のうち e 以外、`Now` は `a.now`
- `hh sync`: `--store` があればその Store、なければ remote のある全 Store を順に（`Select` の「複数あれば --store」とは違い、sync は全部を対象にする）。出力は Store ごとに `Synced <name>: <n> merges, pushed to origin, mirror1`、振り直した ID を `  ORG-2 (ULID) → ORG-3`、上書きを `  overwritten: <ID> <field> (<author>'s value was replaced; hedgehug inbox revert <ULID>)`、修復を `  repaired: <ID> (ULID) <kind>`、`is:invalid` の件数。`--json` は `sync.Result` をそのまま（Store 名を足す）
- `hh sync --status`: Store ごとに `sync.json` の各時刻、`LastError`、未 push の数（`Between(HEAD, RemoteHead("origin"))` の数。remote がなければ `no remote`）
- `store init --remote`: 既存の `store.Create` の後、config に remotes 付きで登録し、`.gitattributes`（最初のコミットに含めるため `store.Create` に `attributes bool` を渡すか、`Create` が常に書く。常に書く方を採る: remote のない Store でも、後で remote を足したときに同じになるため）、driver を登録し、`runSync`。push が `ErrRejected` なら「the remote is not empty; to use the store there, run hedgehug store add <name> --remote <url>」で 1。config の登録と作ったディレクトリは、失敗したら消す（`store.Create` の `undoCreate` と同じ扱い）
- `store add`: `gitrepo.Clone(remotes[0], path)`、`store.Open`、`config.AddStore`、driver の登録、`.gitattributes` が足りなければ `runSync`（setup のコミットを作って push）。他の Store との接頭辞の照合は Phase 1.5 の `workspace.go` の照合関数（Store 名と相手のワークスペースを返すもの）を使い、重複ごとに stderr に `warning: <PREFIX> is also the id prefix of workspace <name> in store <store>; the one set later (<which>) should change it with hedgehug workspace prefix`
- `store list` / `store remove`: config から。`--purge` は `Between(HEAD, RemoteHead)` が空でなければ拒否、空なら `os.RemoveAll`
- `merge-driver`: spec どおり。`MERGE_HEAD` は `<gitdir>/MERGE_HEAD` を読む。`sides` 相当（Task 10 の `sync.Sides` を公開して使う）で各側を作り、`merge.Merge`。上書きと通知を `.hedgehug/local/overwritten/<ULID>.json` に書く。終了コード 0。失敗は 1（git が既定の衝突にする）。`HEDGEHUG_AUTO_SYNC` に関係なく、merge driver はバックグラウンド同期を起こさない
- `ls`・`query` を使うコマンドで `query.Options{Me: c.Me(), Overwritten: <inbox の未読の overwritten の Task>}` を渡す

- [ ] **Step 4: テストを通す**

Run: `go test -race ./cmd/hedgehug/`
Expected: PASS（git がなければ merge driver のテストは skip）

- [ ] **Step 5: コミット**

```bash
git add cmd/hedgehug internal
git commit -m "feat(cli): sync・store init --remote・store add/list/remove・merge driver を足す"
```

---

### Task 14: バックグラウンド同期

**Files:**
- Create: `internal/sync/background.go`, `internal/sync/spawn_unix.go`, `internal/sync/spawn_windows.go`, `internal/sync/background_test.go`, `cmd/hedgehug/background_test.go`
- Modify: `cmd/hedgehug/app.go`（`update` の後と、コマンドの開始時）, `cmd/hedgehug/sync.go`（`--background`）

**Interfaces:**
- Produces:

```go
// MarkPending sets pending_since if it is empty. Writing commands call it
// after a commit.
func MarkPending(s *store.Store, now time.Time) error

// Due reports whether a sync should start for a command: something waits to
// be pushed, or the last success is older than interval.
func Due(st State, now time.Time, interval time.Duration) bool

// Start starts "hedgehug sync --background --store <name>" detached, unless
// HEDGEHUG_AUTO_SYNC=0 or another background sync holds bg.lock.
func Start(s *store.Store, name string, getenv func(string) string) error

var spawn = spawnDetached // replaced in tests

// Background is the body of sync --background: wait for push_interval after
// pending_since, run, repeat while something is pending.
func Background(s *store.Store, opt Options, pushInterval time.Duration, run func() error) error

// TakeError returns the last background error once (it sets error_reported).
func TakeError(s *store.Store) (string, error)
```

- [ ] **Step 1: テストを書く**

`background_test.go`（sync）:

```go
func TestStartRespectsAutoSyncAndLock(t *testing.T) {
	c := newCluster(t, 1)
	calls := 0
	spawn = func(name string, args []string, logPath string) error { calls++; return nil }
	t.Cleanup(func() { spawn = spawnDetached })
	_ = Start(c.stores[0], "s", func(k string) string { return map[string]string{"HEDGEHUG_AUTO_SYNC": "0"}[k] })
	if calls != 0 {
		t.Error("started with HEDGEHUG_AUTO_SYNC=0")
	}
	_ = Start(c.stores[0], "s", func(string) string { return "" })
	if calls != 1 {
		t.Errorf("calls = %d", calls)
	}
	release := holdLocal(t, c.stores[0], "bg.lock") // as a waiting child would
	defer release()
	_ = Start(c.stores[0], "s", func(string) string { return "" })
	if calls != 1 {
		t.Error("started while another background sync waits")
	}
}

func TestBackgroundWaitsAndRuns(t *testing.T) {
	c := newCluster(t, 1)
	now := time.Now()
	_ = MarkPending(c.stores[0], now)
	runs := 0
	err := Background(c.stores[0], c.opts[0], 50*time.Millisecond, func() error {
		runs++
		if time.Since(now) < 50*time.Millisecond {
			t.Error("ran before push_interval")
		}
		_, err := UpdateState(c.stores[0], func(st *State) { st.PendingSince = time.Time{} })
		return err
	})
	if err != nil || runs != 1 {
		t.Errorf("runs = %d, err = %v", runs, err)
	}
}

func TestDue(t *testing.T) {
	now := time.Now()
	if !Due(State{PendingSince: now}, now, time.Minute) {
		t.Error("pending: not due")
	}
	if Due(State{LastSuccess: now.Add(-time.Second)}, now, time.Minute) {
		t.Error("fresh: due")
	}
	if !Due(State{LastSuccess: now.Add(-2 * time.Minute)}, now, time.Minute) {
		t.Error("stale: not due")
	}
}

func TestTakeErrorOnce(t *testing.T) {
	c := newCluster(t, 1)
	_, _ = UpdateState(c.stores[0], func(st *State) { st.LastError = "boom" })
	if e, _ := TakeError(c.stores[0]); e != "boom" {
		t.Errorf("first = %q", e)
	}
	if e, _ := TakeError(c.stores[0]); e != "" {
		t.Errorf("second = %q", e)
	}
}
```

`cmd/hedgehug/background_test.go`:

```go
func TestWriteStartsBackgroundSync(t *testing.T) {
	alice, _, _ := twoUsers(t)
	var started []string
	restore := stubSpawn(func(args []string) { started = append(started, strings.Join(args, " ")) })
	defer restore()
	alice.env["HEDGEHUG_AUTO_SYNC"] = "1"
	alice.ok("add", "x")
	if len(started) != 1 || !strings.Contains(started[0], "sync --background --store org") {
		t.Errorf("started = %v", started)
	}
}

func TestStartReportsBackgroundError(t *testing.T) {
	alice, _, _ := twoUsers(t)
	alice.setSyncError("org", "remote unreachable")
	_, _, errs := alice.run("ls")
	if !strings.Contains(errs, "hedgehug: background sync of org failed: remote unreachable (run hedgehug sync)") {
		t.Errorf("stderr = %s", errs)
	}
	if _, _, errs := alice.run("ls"); strings.Contains(errs, "background sync") {
		t.Error("reported twice")
	}
}

func TestBackgroundSyncReachesRemote(t *testing.T) {
	// The real binary: a write starts a detached child that pushes after push_interval.
	bin := buildHedgehug(t)
	alice, bob, _ := twoUsers(t)
	alice.writeSync("50ms", "50ms") // sync.interval and sync.push_interval in config.yaml
	alice.runBinary(bin, "add", "bg") // with HEDGEHUG_AUTO_SYNC unset
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		bob.ok("sync")
		if strings.Contains(bob.ok("ls"), "bg") {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Error("the background sync did not push")
}
```

`stubSpawn`・`setSyncError`・`writeSync`・`runBinary` は `helpers_test.go` に足す。`stubSpawn` は `sync` パッケージの `spawn` を差し替える口として `sync.SetSpawn(f) (restore func())` を公開する（テスト専用と doc comment に書く）。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/sync/ ./cmd/hedgehug/ -run 'Start|Background|Due|TakeError'`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `Start`: `getenv("HEDGEHUG_AUTO_SYNC") == "0"` なら何もしない。`bg.lock` を待たずに試し（`WithLocalLock(name, 0, ...)` が取れなければ `ErrLocked`）、取れたらすぐ放して `spawn(os.Executable(), []string{"sync", "--background", "--store", name}, <dir>/.hedgehug/local/sync.log)`。子が `bg.lock` を取り直す（親と子の間で取り合いになっても、どちらかが待つだけで二重に同期しない）
- `spawn_unix.go`: `exec.Command` に `SysProcAttr{Setsid: true}`、stdin/stdout を `/dev/null`、stderr を `sync.log`（開く前に 1 MiB を超えていれば後ろ半分だけ残して書き直す）、`HEDGEHUG_AUTO_SYNC=0` を環境に足さない（子は自分では起こさない作りなので不要）。`cmd.Start()` して `cmd.Process.Release()`
- `spawn_windows.go`: `SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP}`（`golang.org/x/sys/windows`）
- `Background`: `WithLocalLock("bg.lock", lockWait, …)` の中で、`ReadState` の `PendingSince + pushInterval` まで眠り（空なら待たない）、`WithLocalLock("sync.lock", …, run)`、終わって `PendingSince` が残っていれば繰り返す
- `cmd`: `update` が committed なら `sync.MarkPending` と、remote があれば `sync.Start`。`rootCmd` の `PersistentPreRunE` で（`merge-driver` と `sync` を除き）、選んだ Store（`--store` がなければ config の全 Store）について `TakeError` を出し、`Due` なら `Start`。Store が開けないなどの失敗は黙って飛ばす（コマンドの本体のエラーで知らせる）
- `hh sync --background`: `sync.Background(s, opt, c.Sync.PushInterval, func() error { _, err := a.runSync(s, opt); return err })`。終了コードは常に 0（失敗は `sync.json` に）

- [ ] **Step 4: テストを通す**

Run: `go test -race ./internal/sync/ ./cmd/hedgehug/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/sync cmd/hedgehug go.mod go.sum
git commit -m "feat(sync): 書き込みの後と起動時にバックグラウンドの子プロセスで同期する"
```

---
### Task 15: CLI — inbox、watch / unwatch、watchers の自動追加、show の Referenced by

**Files:**
- Create: `cmd/hedgehug/inbox.go`, `cmd/hedgehug/watch.go`, `cmd/hedgehug/inbox_test.go`, `cmd/hedgehug/watch_test.go`
- Modify: `cmd/hedgehug/add.go`, `cmd/hedgehug/set.go`, `cmd/hedgehug/assign.go`, `cmd/hedgehug/comment.go`, `cmd/hedgehug/edit.go`, `cmd/hedgehug/show.go`, `cmd/hedgehug/app.go`（rootCmd に登録）

**Interfaces:**
- Consumes: Task 11 の `inbox.Box`、Task 12 の `index.Mentions`・`Backlinks`、Task 4 の `writer`
- Produces:
  - `hh inbox [--all]`、`hh inbox read <ULID>... | --all`、`hh inbox revert <ULID>`、`hh watch <id>`、`hh unwatch <id>`
  - `func addWatchers(t *model.Task, accts ...string)` — 重複なしで末尾に足す
  - `func mentionedAccounts(body string, writerHost string) []string` — `@login` に書き手の host を補い `host/login` に

- [ ] **Step 1: テストを書く**

`inbox_test.go`:

```go
func TestInboxOverwrittenAndRevert(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	bob.ok("sync")
	bob.ok("set", "ORG-1", "title=bob")
	alice.ok("set", "ORG-1", "title=alice") // later: wins
	bob.ok("sync")
	alice.ok("sync")
	bob.ok("sync")
	out := bob.ok("inbox")
	m := regexp.MustCompile(`([0-9A-Z]{26})\s+overwritten\s+ORG-1 \([0-9A-Z]{26}\) alice\s+title: "bob" was replaced by github.com/alice`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("inbox = %s", out)
	}
	bob.ok("inbox", "revert", m[1])
	if out := bob.ok("show", "ORG-1", "--json"); !strings.Contains(out, `"title": "bob"`) {
		t.Errorf("after revert: %s", out)
	}
	if out := bob.ok("inbox"); strings.Contains(out, m[1]) {
		t.Errorf("reverted item is still unread: %s", out)
	}
	if out := bob.ok("inbox", "--all", "--json"); !strings.Contains(out, m[1]) {
		t.Errorf("--all lacks it: %s", out)
	}
}

func TestInboxRenumbered(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("add", "a")
	bob.ok("add", "b")
	alice.ok("sync")
	bob.ok("sync")
	if out := bob.ok("inbox"); !regexp.MustCompile(`renumbered\s+ORG-3 \([0-9A-Z]{26}\) b\s+ORG-2 was renumbered to ORG-3`).MatchString(out) {
		t.Errorf("inbox = %s", out)
	}
}

func TestInboxReadAll(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("assign", "ORG-1", "github.com/bob")
	alice.ok("sync")
	bob.ok("sync")
	bob.ok("inbox", "read", "--all")
	if out := bob.ok("inbox"); strings.TrimSpace(out) != "No unread items" {
		t.Errorf("inbox = %q", out)
	}
}

func TestInboxRejectsShortULID(t *testing.T) {
	_, bob, _ := twoUsers(t)
	if code, _, _ := bob.run("inbox", "read", "ABCD"); code != 2 {
		t.Errorf("a short ULID was accepted")
	}
}

func TestInboxWithoutAccounts(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	_, _, errs := te.run("inbox")
	if !strings.Contains(errs, "accounts are not set up") {
		t.Errorf("stderr = %s", errs)
	}
}
```

`watch_test.go`:

```go
func TestAutoWatchers(t *testing.T) {
	alice, _, _ := twoUsers(t)
	alice.ok("add", "x")                            // ORG-2: the writer watches
	alice.ok("assign", "ORG-2", "github.com/bob")   // the assignee watches
	alice.ok("comment", "ORG-2", "ping @carol")     // the mentioned account (host of the writer) watches
	out := alice.ok("show", "ORG-2", "--json")
	for _, w := range []string{"github.com/alice", "github.com/bob", "github.com/carol"} {
		if !strings.Contains(out, `"`+w+`"`) {
			t.Errorf("watchers lack %s: %s", w, out)
		}
	}
}

func TestWatchUnwatch(t *testing.T) {
	_, bob, _ := twoUsers(t)
	bob.ok("watch", "ORG-1")
	if out := bob.ok("ls", "watching:@me"); !strings.Contains(out, "ORG-1") {
		t.Errorf("watching = %s", out)
	}
	bob.ok("unwatch", "ORG-1")
	if out := bob.ok("ls", "watching:@me"); strings.Contains(out, "ORG-1") {
		t.Errorf("still watching: %s", out)
	}
}

func TestShowReferencedBy(t *testing.T) {
	alice, _, _ := twoUsers(t)
	alice.ok("add", "b", "--body", "after #ORG-1")
	out := alice.ok("show", "ORG-1")
	if !regexp.MustCompile(`Referenced by\n\s+ORG-2 \([0-9A-Z]{26}\) b`).MatchString(out) {
		t.Errorf("show = %s", out)
	}
	if out := alice.ok("show", "ORG-1", "--json"); !strings.Contains(out, `"referenced_by": [{"ulid"`) {
		t.Errorf("json = %s", out)
	}
}
```

`add --body` がなければ、本文は `edit` で書く既存の補助（`editWith`）を使う。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./cmd/hedgehug/ -run 'Inbox|Watch|ReferencedBy'`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `hh inbox`: 全 Store（`--store` があればそれ）の `Box.Import` の後に `List`。1件1行: `<ULID>  <kind>  <ID> (<ULID>) <title>  <内容>`。内容は種類ごと: overwritten は `<field>: <負けた値> was replaced by <勝った側>`、renumbered は `<old> was renumbered to <new>; #<old> in text still points to the task that kept it`、repaired は `<kind>`（`parent_cycle` なら `moved back to <親の ID か root>` など、Detail から）、prefix_duplicate は `<PREFIX> is also used by workspace <name>[ in store <store>]; change it with hedgehug workspace prefix <ws> <new prefix>`、mention・assigned・watch は「by <author>」と変更の要約。未読がなければ `No unread items`。`--json` は Item の配列（Store 名を足し、タスクは `{"ulid","id","title"}`）
- accounts がなければ stderr に `hedgehug: warning: accounts are not set up in config.yaml; mentions and assignments cannot be told` を出す（終了コード 0）
- `inbox read`: 引数は `model.ParseULID` で完全な ULID だけ（違えば usage error）。`--all` と同時は usage error
- `inbox revert`: Item が overwritten でなければエラー。タスクを読み、`Field` を Value（JSON）に戻す: `title` など組み込みのフィールドは `set` の処理（`applySet`）に `field=value` を渡す。`props.<k>` も同じ。`*`（ファイル全体）は revert できないと言う。`Op{Name: "revert-overwritten", Task: ...}`。その後 `MarkRead`
- watchers の自動追加: add で書き手、assign と set の `assignees` で足された人、comment で書き手、本文（add・edit）とコメントで新しく足されたメンション（`mentionedAccounts`）。書き手がいない（accounts なし）なら書き手は足さない
- `hh watch` / `unwatch`: 書き手がいなければ「accounts are not set up」でエラー
- `show`: `Referenced by` の節を `Backlinks` で（なければ節を出さない）、`--json` の `referenced_by`

- [ ] **Step 4: テストを通す**

Run: `go test -race ./cmd/hedgehug/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add cmd/hedgehug
git commit -m "feat(cli): inbox・watch・watchers の自動追加・show の Referenced by を足す"
```

---

### Task 16: CLI — undo と restore --at

**Files:**
- Create: `cmd/hedgehug/undo.go`, `cmd/hedgehug/undo_test.go`, `cmd/hedgehug/restore_at_test.go`
- Modify: `cmd/hedgehug/rm.go`（`restoreCmd` に `--at`）, `cmd/hedgehug/app.go`

**Interfaces:**
- Consumes: Task 6 の `merge.Merge(..., merge.OursWins)`、Task 2 の `LogFrom`・`ReadAt`
- Produces: `hh undo`、`hh restore <id> --at <時点>`

- [ ] **Step 1: テストを書く**

`undo_test.go`:

```go
func TestUndoKeepsLaterChangesOfOthers(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	bob.ok("sync")
	alice.ok("set", "ORG-1", "title=alice")
	alice.ok("sync")
	bob.ok("sync")
	bob.ok("status", "ORG-1", "doing")
	bob.ok("sync")
	alice.ok("sync")
	out := alice.ok("undo")
	if !strings.Contains(out, "Undid") || !strings.Contains(out, "kept status (changed later by github.com/bob)") {
		t.Errorf("undo = %s", out)
	}
	got := alice.ok("show", "ORG-1", "--json")
	if !strings.Contains(got, `"title": "x"`) || !strings.Contains(got, `"status": "doing"`) {
		t.Errorf("after undo: %s", got) // twoUsers made ORG-1 with the title x
	}
	if out := alice.ok("undo"); !strings.Contains(out, "Undid") {
		t.Errorf("second undo = %s", out) // the add of ORG-1 by alice
	}
}

func TestUndoNothing(t *testing.T) {
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	if out := te.ok("undo"); strings.TrimSpace(out) != "Nothing to undo" {
		t.Errorf("undo = %q", out)
	}
}
```

ワークスペースをまたぐ移動の取り消し:

```go
func TestUndoMoveAcrossWorkspaces(t *testing.T) {
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "WEB", "--workspace", "web")
	te.ok("workspace", "add", "api", "--prefix", "API")
	te.ok("add", "p", "--workspace", "web")    // WEB-1
	te.ok("mv", "WEB-1", "--workspace", "api") // API-1 (former WEB-1)
	te.withOtherWriter("github.com/bob", func() {
		te.ok("add", "c", "--parent", "API-1") // API-2, by someone else after the move
	})
	te.ok("undo") // alice's last own operation: the mv
	p := te.ok("show", "WEB-1", "--json")
	// p is back in web with a new number; WEB-1 and API-1 stay in former_ids
	if !regexp.MustCompile(`"id": "WEB-2"`).MatchString(p) || !strings.Contains(p, `"WEB-1"`) || !strings.Contains(p, `"API-1"`) {
		t.Errorf("p = %s", p)
	}
	c := te.ok("show", "API-2", "--json")
	if !strings.Contains(c, `"id": "WEB-3"`) {
		t.Errorf("the child did not follow its parent: %s", c)
	}
}
```

`withOtherWriter` は、config.yaml の `stores[].account` を一時的に別のアカウントにして fn を走らせ、戻す補助（`helpers_test.go`）。`show WEB-1` は `former_ids` から p を引く。

`restore_at_test.go`:

```go
func TestRestoreAt(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.now = time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)
	te.ok("add", "v1")
	te.now = time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	te.ok("set", "ME-1", "title=v2")
	te.ok("rm", "ME-1")
	te.ok("restore", "ME-1", "--at", "2026-10-30")
	out := te.ok("show", "ME-1", "--json")
	if !strings.Contains(out, `"title": "v1"`) || strings.Contains(out, `"deleted_at": "2`) {
		t.Errorf("show = %s", out)
	}
	if _, _, errs := te.run("restore", "ME-1", "--at", "2026-10-29"); !strings.Contains(errs, "did not exist") {
		t.Errorf("before creation: %s", errs)
	}
}

func TestRestoreAtCommitAndWorkspaceNote(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "WEB", "--workspace", "web")
	te.ok("workspace", "add", "api", "--prefix", "API")
	te.ok("add", "x", "--workspace", "web")
	h := te.headCommit("me").Hash.String()[:7]
	te.ok("mv", "WEB-1", "--workspace", "api")
	te.ok("set", "API-1", "title=later")
	out := te.ok("restore", "API-1", "--at", h)
	if !strings.Contains(out, "the workspace stays api; move it with hedgehug mv API-1 --workspace web") {
		t.Errorf("restore = %s", out)
	}
	if got := te.ok("show", "API-1", "--json"); !strings.Contains(got, `"title": "x"`) || !strings.Contains(got, `"id": "API-1"`) {
		t.Errorf("show = %s", got)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./cmd/hedgehug/ -run 'Undo|RestoreAt'`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `undo`: `LogFrom(HEAD, 0)`（first-parent）を新しい順に見て、`Hedgehug-Undo` の値の集合を作りながら、`Hedgehug-Author` が書き手で、`Hedgehug-Op` が merge・repair・setup・undo でなく、集合に入っていない最初のコミット c を選ぶ。c が変えたパス（`DiffTrees(c の親の木, c の木)`）ごとに `merge.Merge(path, sc, base=ReadAt(c), ours=Side{Data: ReadAt(HEAD)}, theirs=Side{Data: ReadAt(c^)}, merge.OursWins)`。タスクのファイルは結果を `model.DecodeTask` し、`ID` と `FormerIDs` は HEAD の値に戻す。ours（HEAD）と base（c の版）で違っていたフィールドの名前を「kept」として集め、そのフィールドを最後に変えたコミットの書き手を `LastChange` で添える
- `workspace` が変わるタスクは、`Update` の中で `ix.Refresh()` してから今の配下を集め、同じワークスペースに移す（`mv --workspace` と同じ関数を使う）。すべてを1つの `s.Update(Op{Name: "undo"}, …)` で書き、`Hedgehug-Undo: <c のハッシュ>` を付ける（`Op` に `Trailers []gitrepo.Trailer` を足す）。件名の Detail は c の件名から `hedgehug: ` を除いたもの
- 出力: `Undid <c の短いハッシュ> <c の件名の残り>` と、`  kept <field> (changed later by <author>)` を1行ずつ
- `restore --at`: 時点の解釈は RFC 3339 → 日付（その日の 23:59:59 のローカル時刻）→ コミットハッシュ（`r.r.ResolveRevision`、短縮可）の順。`LogFrom(HEAD)` で時点以前の最新のコミットを探し、`ReadAt(そのコミット, TaskPath)`。なければ `<ID> did not exist at <時点>` で 1。読んだ内容の `ID`・`FormerIDs`・`Workspace` を今の値にし、`DeletedAt` はその時点の値、`Updated` は書き込みで今になる。ワークスペースが違っていたら `the workspace stays <今>; move it with hedgehug mv <ID> --workspace <その時点の名前>` を出す
- 既存の `restore <id>`（`--at` なし）はそのまま

- [ ] **Step 4: テストを通す**

Run: `go test -race ./cmd/hedgehug/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add cmd/hedgehug internal/store
git commit -m "feat(cli): 他人の後の変更を残す undo と restore --at を足す"
```

---

### Task 17: doctor — --fix、merge driver の登録、取り込まれていない記録

**Files:**
- Modify: `internal/doctor/doctor.go`, `internal/doctor/doctor_test.go`, `cmd/hedgehug/doctor.go`, `cmd/hedgehug/doctor_test.go`

**Interfaces:**
- Consumes: Task 8 の `merge.Repair`、Task 10 の `sync` の repairHead（`sync.RepairHead(s, opt) ([]merge.Repaired, error)` として公開する）
- Produces:
  - doctor の Finding の check に `merge_driver`・`inbox` を足す（`checkOrder` の `git` の前）
  - `func Fix(s *store.Store, opt sync.Options) ([]merge.Repaired, error)`（cmd から呼ぶ。`sync.RepairHead` の薄い包み）
  - 直せる問題の Hint に「hedgehug doctor --fix で直る」を足す（`duplicate_id`・`id_prefix`・`workspace_mismatch`・親の循環・削除済みの親）。`duplicate_id` には「push 済みの側を残して hedgehug sync でも振り直る」も

- [ ] **Step 1: テストを書く**

`doctor_test.go`（internal）:

```go
func TestMergeDriverChecks(t *testing.T) {
	dir := newStoreWithAttributes(t) // .gitattributes with merge=hedgehug, no driver in .git/config
	fs, _ := Run(dir)
	if !hasFinding(fs, "merge_driver", "not registered") {
		t.Errorf("findings = %v", fs)
	}
	registerDriver(t, dir, "hedgehug merge-driver %O %A %B %P")
	t.Setenv("PATH", t.TempDir()) // no hedgehug on PATH
	fs, _ = Run(dir)
	if !hasFinding(fs, "merge_driver", "hedgehug is not on PATH") {
		t.Errorf("findings = %v", fs)
	}
}

func TestUnimportedRecords(t *testing.T) {
	dir := newStoreWithAttributes(t)
	writeLocal(t, dir, "overwritten/01JC1111111111111111111111.json", "{}")
	fs, _ := Run(dir)
	if !hasFinding(fs, "inbox", "hedgehug inbox") {
		t.Errorf("findings = %v", fs)
	}
}
```

`cmd/hedgehug/doctor_test.go`:

```go
func TestDoctorFixWithoutRemote(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "a")
	te.ok("add", "b")
	te.handEdit("me", "ME-2", "id: ME-2", "id: ME-1") // writes the file and commits it as a hand edit would
	if code, _, _ := te.run("doctor"); code != 1 {
		t.Error("doctor found nothing")
	}
	out := te.ok("doctor", "--fix")
	if !regexp.MustCompile(`repaired: ME-3 \([0-9A-Z]{26}\) b: id_collision ME-1 → ME-3`).MatchString(out) {
		t.Errorf("fix = %s", out)
	}
	if code, out, _ := te.run("doctor"); code != 0 {
		t.Errorf("after fix: %d %s", code, out)
	}
	if out := te.ok("doctor", "--fix"); !strings.Contains(out, "Nothing to repair") {
		t.Errorf("second fix = %s", out)
	}
}

func TestDoctorFixLeavesBrokenFiles(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "a")
	te.handEdit("me", "ME-1", "title: a", "title: [")
	code, out, _ := te.run("doctor", "--fix")
	if code != 1 || !strings.Contains(out, "does not decode") {
		t.Errorf("code %d out %s", code, out)
	}
}
```

`handEdit` は、作業ツリーのタスクファイルの文字列を置き換えて go-git でコミットする補助（`helpers_test.go`）。`newStoreWithAttributes`・`registerDriver`・`writeLocal`・`hasFinding` は internal/doctor のテスト補助に足す。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/doctor/ ./cmd/hedgehug/ -run 'MergeDriver|Unimported|DoctorFix'`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `merge_driver` の検査: `.gitattributes` に `merge=hedgehug` がある Store で、`.git/config` の `merge "hedgehug"` の `driver` が `hedgehug merge-driver %O %A %B %P` でない（ないを含む）→ `not registered`、`exec.LookPath("hedgehug")` が失敗 → `hedgehug is not on PATH`。Hint は `run hedgehug sync once`（PATH の問題は「hedgehug を PATH に入れる」）
- `inbox` の検査: `.hedgehug/local/overwritten/` にファイルがあれば `<n> merge driver records are not in the inbox yet`、Hint `run hedgehug inbox`
- `doctor --fix`: `sync.RepairHead`（remote がなくても動く。`OnOrigin` は `origin` の `RemoteHead` があればその木、なければ常に false）。直した内容を `repaired: <ID> (ULID) <title>: <kind> <要約>` で1行ずつ（id_collision は `<old> → <new>`）。なければ `Nothing to repair`。その後に通常の doctor を走らせ、残った Finding を出し、あれば終了コード 1。`--json` は `{"repaired": [...], "findings": [...]}`

- [ ] **Step 4: テストを通す**

Run: `go test -race ./internal/doctor/ ./cmd/hedgehug/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/doctor internal/sync cmd/hedgehug
git commit -m "feat(doctor): --fix で同期と同じ修復をかけ、merge driver と取り込み前の記録を検査する"
```

---

### Task 18: プロパティテスト（完了条件）

**Files:**
- Create: `internal/sync/property_test.go`, `internal/sync/ops_test.go`

**Interfaces:**
- Consumes: Task 10 の `cluster`、すべての同期の部品

- [ ] **Step 1: 操作と記録を書く**（`ops_test.go`）

```go
// write is one write a clone made, kept to check that no value is lost.
type write struct {
	clone  int
	commit plumbing.Hash
	task   model.ULID
	field  string // title, status, parent, deleted_at, body, labels, id, ...
	value  string // JSON; for body, the lines written; for labels, the label added
}

type op struct {
	name string
	do   func(c *cluster, i int, rnd *rand.Rand) []write
}

var ops = []op{
	{"add", opAdd},
	{"set title", opSetTitle},
	{"status", opStatus},
	{"mv", opMove},               // a random parent in the same workspace, or root
	{"mv --workspace", opMoveWS},  // to another workspace, with the subtree
	{"rm", opRemove},
	{"restore", opRestore},
	{"purge", opPurge},           // only a deleted task with no live descendant
	{"comment", opComment},
	{"body", opBody},             // appends or replaces a random line
	{"label", opLabel},           // adds a random label
	{"schema", opSchema},         // adds a status or a prop
	{"workspace add", opWSAdd},
	{"workspace prefix", opWSPrefix},
	{"workspace rename", opWSRename},
	{"workspace rm", opWSRm},     // only an empty workspace
}
```

各 `opXxx` は `c.stores[i]` を `store.Update` で書き、成功したら `write` を返す（拒否された書き込みは何も返さない。拒否は正しい動作）。CLI の処理（`mv --workspace` の配下の集め方など）は `cmd/hedgehug` にあって呼べないので、テストの中で同じ規則（配下を同じワークスペースへ）で書く。

- [ ] **Step 2: プロパティテストを書く**（`property_test.go`）

```go
func TestPropertyConverges(t *testing.T) {
	rounds, steps := 30, 40
	if testing.Short() {
		rounds, steps = 5, 20
	}
	for seed := int64(1); seed <= int64(rounds); seed++ {
		t.Run(fmt.Sprint("seed", seed), func(t *testing.T) {
			rnd := rand.New(rand.NewPCG(uint64(seed), 0))
			c := newCluster(t, 3)
			var log []string
			var writes []write
			for range steps {
				i := rnd.IntN(3)
				if rnd.IntN(4) == 0 {
					log = append(log, fmt.Sprintf("sync %d", i))
					c.sync(i)
					continue
				}
				o := ops[rnd.IntN(len(ops))]
				log = append(log, fmt.Sprintf("%s on %d", o.name, i))
				writes = append(writes, o.do(c, i, rnd)...)
			}
			// Several clones add without syncing: ids must collide.
			for i := range 3 {
				writes = append(writes, opAdd(c, i, rnd)...)
			}
			settle(t, c) // sync round-robin until no HEAD changes
			if err := checkAll(c, writes); err != nil {
				t.Fatalf("seed %d: %v\nops:\n  %s", seed, err, strings.Join(log, "\n  "))
			}
		})
	}
}
```

`checkAll` は spec の完了条件の3つを調べる:

1. `c.headTree(0) == c.headTree(1) == c.headTree(2)`
2. 値が失われていない。HEAD（全クローンで同じ）の履歴から、各コミットの trailer（`Hedgehug-Overwritten`、`Hedgehug-Repaired`）を集める。各 `write` について:
   - frontmatter のフィールド: 最終の値と等しい、または、そのコミットを祖先に持つコミットで同じタスクの同じフィールドへの write がある（因果的に後の書き込み）、または `Hedgehug-Overwritten` に `(タスクのパス, field, value)` がある、または `Hedgehug-Repaired` にそのタスクがある
   - `id`: 最終の `id` か `former_ids` に入っている
   - `deleted_at`: 上に加えて、`revived` の trailer があってもよい
   - 本文: 書いた各行が最終の本文にある、または因果的に後の本文の write がある
   - labels: 足したラベルが最終にある、または因果的に後の labels の write がある
   - purge されたタスクは、purge の write より前の write を調べない
3. 不変条件: 親の循環がない、削除されていないタスクの親は削除されていない、`id` が重複しない、どの ID も2つのタスクの `id`/`former_ids` に現れない（再利用なし。`former_ids` の重複は同じタスクの中だけ）、全タスクのワークスペースが存在する、子は親と同じワークスペース、形式の正しい接頭辞のワークスペースではタスクの ID の接頭辞が一致する

「因果的に後」は `IsAncestor(write.commit, other.commit)`。

- [ ] **Step 3: 走らせ、失敗を直す**

Run: `go test -race ./internal/sync/ -run Property -count=1`
Expected: PASS。失敗したら、表示された seed と操作の列で再現するテストを `sync_test.go` か `merge` に足して原因を直す（テストの条件を緩めて通すのは禁止。条件が spec と合わないと考えたら、コントローラーに上げる）

- [ ] **Step 4: -short でも通ることを確かめる**

Run: `go test -short ./...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal
git commit -m "test(sync): 3つのクローンのランダムな同時編集が収束し値を失わないことのプロパティテストを足す"
```

---

### Task 19: 開発環境 — devenv、git を外した CI、E2E

**Files:**
- Modify: `tools/devenv/main.go`, `tools/devenv/main_test.go`, `.github/workflows/ci.yaml`
- Create: `cmd/hedgehug/e2e_test.go`

**Interfaces:**
- Consumes: すべて

- [ ] **Step 1: devenv のテストを書く**

```go
func TestInitMakesSharedStore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "hh-dev")
	if err := run([]string{"init", "--dir", dir}, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"remotes/kukv/hh-tasks.git/HEAD", "remotes/org-a/hh-shared.git/HEAD"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
	for _, u := range []string{"alice", "bob"} {
		cfg, _ := os.ReadFile(filepath.Join(dir, "users", u, "config", "hedgehug", "config.yaml"))
		if !strings.Contains(string(cfg), "login: "+u) || !strings.Contains(string(cfg), "name: org-a") || !strings.Contains(string(cfg), "hh-shared.git") {
			t.Errorf("%s config:\n%s", u, cfg)
		}
	}
	// bob's clone of org-a has alice's first commit
	bobOrg := filepath.Join(dir, "users", "bob", "data", "hedgehug", "stores", "org-a", ".hedgehug", "store.yaml")
	if _, err := os.Stat(bobOrg); err != nil {
		t.Errorf("bob has no org-a: %v", err)
	}
}
```

（config.yaml とストアの実際のパスは `config.Resolve` が `HEDGEHUG_HOME` から作るものに合わせる。既存の devenv のテストの書き方に従う。）

- [ ] **Step 2: devenv を直す**

- `init` で2つの bare を `gitrepo.InitBare` で作る
- 各ユーザーの config.yaml に `accounts: [{name: personal, host: github.com, login: <u>}]` を書き、個人 Store の `account: personal`
- alice の個人 Store に remotes `remotes/kukv/hh-tasks.git` を付け、初回の push（`sync.Run`、`HEDGEHUG_AUTO_SYNC` に関係なく直接呼ぶ）
- alice が `org-a`（接頭辞 `ORG`、remote `hh-shared.git`）を `store.Create` → config → `sync.Run`、bob が `gitrepo.Clone` → config（`AddStore("org-a", path, remotes)`）→ merge driver の登録
- `env` の出力は今のまま（`HEDGEHUG_AUTO_SYNC` を出さない）

- [ ] **Step 3: E2E を書く**（`e2e_test.go`。ビルドしたバイナリを `exec` で動かす）

```go
func TestE2EAliceAndBob(t *testing.T) {
	bin := buildHedgehug(t)
	dev := filepath.Join(t.TempDir(), "hh-dev")
	runDevenv(t, "init", "--dir", dev)
	alice, bob := devUser(t, bin, dev, "alice"), devUser(t, bin, dev, "bob") // env from devenv env, HEDGEHUG_AUTO_SYNC=0
	alice.ok("add", "shared", "--store", "org-a")
	alice.ok("sync")
	bob.ok("sync")
	alice.ok("set", "ORG-1", "title=alice", "--store", "org-a")
	bob.ok("set", "ORG-1", "title=bob", "--store", "org-a")
	bob.ok("sync")
	alice.ok("sync")
	bob.ok("sync")
	if a, b := alice.ok("show", "ORG-1", "--store", "org-a", "--json"), bob.ok("show", "ORG-1", "--store", "org-a", "--json"); a != b {
		t.Errorf("not converged:\n%s\n%s", a, b)
	}
	inbox := bob.ok("inbox")
	id := regexp.MustCompile(`([0-9A-Z]{26})\s+overwritten`).FindStringSubmatch(inbox)
	if id == nil {
		t.Fatalf("bob's inbox: %s", inbox)
	}
	bob.ok("inbox", "revert", id[1])
	bob.ok("undo") // takes the revert back
	alice.ok("add", "a", "--store", "org-a")
	bob.ok("add", "b", "--store", "org-a")
	alice.ok("sync")
	bob.ok("sync")
	if !strings.Contains(bob.ok("inbox"), "renumbered") {
		t.Error("bob was not told of the renumbering")
	}
}
```

`buildHedgehug` は Task 13 で作ったもの。`devUser` は `devenv env <u> --dir <dev>` の出力（`export K=V` の行）を環境にして bin を動かす補助。

- [ ] **Step 4: git を外した CI ジョブを足す**（`.github/workflows/ci.yaml`）

```yaml
  # Hedgehug は git なしで動く（親設計の「外部依存ゼロ」）。git を PATH から外して全テストを流し、
  # どこかが git の実行ファイルを呼ぶようになったらここで気付く。
  test-without-git:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
          cache: false
      - name: Install gotestsum
        run: go install gotest.tools/gotestsum@v1.13.0
      - name: Test with no git on PATH
        shell: bash
        run: |
          mkdir -p "$RUNNER_TEMP/nogit"
          ln -s "$(command -v go)" "$RUNNER_TEMP/nogit/go"
          ln -s "$(go env GOPATH)/bin/gotestsum" "$RUNNER_TEMP/nogit/gotestsum"
          ! PATH="$RUNNER_TEMP/nogit" command -v git
          PATH="$RUNNER_TEMP/nogit" gotestsum --format testdox -- ./...
```

`go` の実体が使うツールチェーンが PATH の別のコマンドを要るなら（`go env GOROOT` の bin を指すだけで足りる）、`GOROOT/bin` の `go` と `gofmt` へのリンクにする。ローカルで同じ手順（`PATH=<そのディレクトリ> go test ./...`）を走らせて確かめる。

- [ ] **Step 5: テストを通す**

Run: `go test -race ./... && mkdir -p /tmp/nogit-check && ln -sf "$(command -v go)" /tmp/nogit-check/go && PATH=/tmp/nogit-check go test ./...`
Expected: どちらも PASS（git を使うテストは skip と表示される）。一時ディレクトリはスクラッチパッドに作る

- [ ] **Step 6: コミット**

```bash
git add tools/devenv cmd/hedgehug .github/workflows/ci.yaml
git commit -m "feat(devenv): bare remote と共有 Store を作り、E2E と git を外した CI ジョブを足す"
```

---

### Task 20: 仕上げ — 親設計の反映、spec との突き合わせ、全体の確認

**Files:**
- Modify: 親設計ドキュメント（Claude Docs、https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）, `docs/superpowers/specs/2026-10-31-phase2-sync-merge-design.md`（実装で決めた細部）

この Task はコントローラーが行う（サブエージェントは Claude Docs を編集しない）。

- [ ] **Step 1: spec の各節を実装と突き合わせる**

spec を頭から読み、各項目に当たるコードとテストを挙げる。実装で決めた細部（`Hedgehug-Repaired` の `revived`、`Tx.PutFile`、`WithLocalLock`、`Store.Create` が常に `.gitattributes` を書くこと、`Op.Trailers`）を spec に書き足す。項目に当たるものがなければ、その場で作る（残さない）。

- [ ] **Step 2: 親設計を書き換える**

spec の「親設計ドキュメントへの反映」の一覧をすべて反映する（Claude Docs の `read` で節を読み、`update` で書き換える）。

- [ ] **Step 3: 全体を確かめる**

Run: `make check && go test -race -count=1 ./... && go test ./internal/merge/ -fuzz FuzzSymmetric -fuzztime 60s`
Expected: すべて PASS

- [ ] **Step 4: devenv で手で確かめる**

```bash
go run ./tools/devenv reset --dir <スクラッチパッド>/hh-dev
eval "$(go run ./tools/devenv env alice --dir <スクラッチパッド>/hh-dev)"
go run ./cmd/hedgehug add "devenv check" --store org-a   # バックグラウンド同期が起きる
sleep 35
eval "$(go run ./tools/devenv env bob --dir <スクラッチパッド>/hh-dev)"
go run ./cmd/hedgehug sync && go run ./cmd/hedgehug ls --store org-a
```

Expected: bob の `ls` に `devenv check` が出る（`go run` の一時バイナリは子プロセスの起動で消えることがあるので、`go build -o bin/hedgehug ./cmd/hedgehug` したものを PATH に入れて行う）

- [ ] **Step 5: コミット**

```bash
git add docs
git commit -m "docs: フェーズ2の spec を実装の細部に合わせる"
```
