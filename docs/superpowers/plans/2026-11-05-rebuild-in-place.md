# force push 復旧を同じディレクトリ内で行う Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `hh doctor --fix` の force push 復旧を、ディレクトリの差し替えから、`.hedgehug/local/rebuild.json` に進みを記録して同じディレクトリ内で直す方式に変える。

**Architecture:** `gitrepo` に HEAD の外のコミット・早送りでない `main` の移動・届かないオブジェクトの削除を足し、`replicate` の作り直しを準備（`prepare`）と完了（`finish`）に分ける。途中の状態は `store.RebuildPhaseIn` で判定し、`apply` の Store は開かない。

**Tech Stack:** Go、go-git v5、golangci-lint（depguard・forbidigo）

**Spec:** `docs/superpowers/specs/2026-11-05-rebuild-in-place-design.md`

## Global Constraints

- PR は 1 つ。
- Windows 版の実装は含めない。
- `--json` の項目名（`replayed`・`dropped`・`kept`）と GUI の表示は変えない。
- 本番にテストのための差し込み口を足さない。途中の状態は実際の API とファイルの操作で作る。`prepare` と `apply` の分割は本番の構造で、差し込み口ではない。
- OS に触る処理は `internal/lib/osfs` を通す（forbidigo）。cli と server のテストは `storage` を import しない（depguard）。
- spec と違う挙動にしない。必要になったら止めて報告する。
- 落ちたテストは失敗の出力と実行条件を記録する。

## 空実行で確かめたこと

全 Task を main（a4d9f7d）の上で空実行し、`go vet`、`make lint`（0 件）、`make fmt-check`、`make tidy-check`、`make web-check`、`make e2e`（134 件）、`go test -race ./...` が通ることと、Task 3 の壊す確認の 3 つが想定どおり落ちることを確かめた。空実行で分かったことは次のとおり。

- `gitrepo.Repo` はメモリーにオブジェクトを持つ（`cachedStorage`）。削除の確認は `gitrepo.Open` で開き直して行う。
- `gitrepotest.LockObjects` には `.git` のパスを渡す（`objects` をその下に探す）。
- 同時実行のテストは、書き込み側に別の `store.Open` を使う。同じ `Store` を共有すると、ファイルロックは Go のメモリーの同期にならないので `-race` が go-git の内部で競合を検出する（2 プロセスの実際の動きとも違う）。
- 作り直しの後、`Store` はスキーマのキャッシュを持ったままなので閉じて開き直す。`rebuildStore` とテストの `reopen` がこれを行う。

## plan で決めた細部

spec にない細部のうち、次のものをここで決めた。

| # | 内容 |
| --- | --- |
| 1 | 関数名は `gitrepo.CommitOn`・`MoveMain`・`KeepMainRefs`・`PruneUnreachable`、`store.CommitOn`・`RebuildPhaseIn`、`replicate.RebuildStore`・`FinishRebuild`、`usecase.Session.ReleaseForRebuild` |
| 2 | `apply` の Store を開かない判定は `Session.OpenStoreAt` に置く。全入口を 1 か所で止めるため。`mount.Open` は `Session` を通らないので同じ判定を持つ |
| 3 | `rebuild.json` が壊れているとき、`doctor --fix` はエラーのまま報告する（`git` の finding）。Store は開かない |
| 4 | `origin` が空になっていたら `rebuild after a force push: origin is empty` で止める（今の clone の失敗に当たる） |
| 5 | `.kept-<時刻>` のコピーからは `.hedgehug/local` のロック（`lock` と `*.lock`）を除く |
| 6 | `doctor --fix` で `BeforeRebuild` は作り直しの前に 1 回呼ぶ。origin が子孫に戻っていた場合も呼ばれ、GUI には変更の通知が 1 回出る。実害はない（最終レビュー後に変更: spec 決定 8） |
| 7 | 文言: `ErrForcePush` は `... to rebuild the store on it`、失敗の finding は `cannot rebuild the store: ...` に変える |

---

### Task 1: gitrepo に履歴の書き換えの部品を足す

**Files:**
- Modify: `internal/lib/gitrepo/commit_tree.go`（`commitStaged` から `writeCommit` を分け、`CommitOn` を足す）
- Modify: `internal/lib/gitrepo/merge.go`（`FastForward` を `moveMain` に分け、`MoveMain` と `checkMoveMain` を足す）
- Modify: `internal/lib/gitrepo/maintain.go`（`KeepMainRefs`・`reachable`・`PruneUnreachable`）
- Create: `internal/lib/gitrepo/rewrite_test.go`

**Interfaces:**
- Produces: `func (r *Repo) CommitOn(parent plumbing.Hash, files map[string][]byte, c Commit) (plumbing.Hash, error)`、`func (r *Repo) MoveMain(from, to plumbing.Hash, paths []string) error`、`func (r *Repo) KeepMainRefs() error`、`func (r *Repo) PruneUnreachable() error`

- [ ] **Step 1: テストを書く**

`internal/lib/gitrepo/rewrite_test.go`:

```go
package gitrepo

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
)

// rewritten is a repository at from with x.md and old.md, and a commit to
// that is not a descendant of it and has x.md changed, new.md added and
// old.md gone.
func rewritten(t *testing.T) (r *Repo, dir string, from, to plumbing.Hash, paths []string) {
	t.Helper()
	remote := newBare(t)
	a, _ := cloneTwo(t, remote)
	root, err := a.r.Head()
	if err != nil {
		t.Fatal(err)
	}
	from = commitHead(t, a.r, a.dir, map[string]string{"x.md": "old secret\n", "old.md": "gone\n"})
	to, err = a.r.CommitOn(root, map[string][]byte{"x.md": []byte("new\n"), "new.md": []byte("added\n")}, Commit{
		Subject: "rewritten", Author: Signature{Name: "a", Email: "a@example.com"}, When: time.Unix(1, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	return a.r, a.dir, from, to, []string{"new.md", "old.md", "x.md"}
}

func TestCommitOnLeavesHeadAndTheWorkTree(t *testing.T) {
	t.Parallel()
	r, dir, from, to, _ := rewritten(t)
	if h, err := r.Head(); err != nil || h != from {
		t.Errorf("Head = %s, %v; want %s", h, err, from)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("new.md on disk: %v", err)
	}
	if st, err := r.Status(); err != nil || len(st) != 0 {
		t.Errorf("Status = %v, %v", st, err)
	}
	if b, err := r.ReadAt(to, "x.md"); err != nil || string(b) != "new\n" {
		t.Errorf("x.md at to = %q, %v", b, err)
	}
}

func TestCommitOnRefusesAPathOutsideTheWorkTree(t *testing.T) {
	t.Parallel()
	r, _, from, _, _ := rewritten(t)
	if _, err := r.CommitOn(from, map[string][]byte{".git/config": []byte("x")}, Commit{Subject: "s"}); err == nil {
		t.Error("CommitOn wrote into .git")
	}
}

func checkMoved(t *testing.T, r *Repo, dir string, to plumbing.Hash) {
	t.Helper()
	if h, err := r.Head(); err != nil || h != to {
		t.Errorf("Head = %s, %v; want %s", h, err, to)
	}
	if st, err := r.Status(); err != nil || len(st) != 0 {
		t.Errorf("Status = %v, %v", st, err)
	}
	for p, want := range map[string]string{"x.md": "new\n", "new.md": "added\n"} {
		if b, err := os.ReadFile(filepath.Join(dir, p)); err != nil || string(b) != want {
			t.Errorf("%s = %q, %v", p, b, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "old.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("old.md: %v", err)
	}
}

func TestMoveMainToACommitThatIsNoDescendant(t *testing.T) {
	t.Parallel()
	r, dir, from, to, paths := rewritten(t)
	if err := r.MoveMain(from, to, paths); err != nil {
		t.Fatal(err)
	}
	checkMoved(t, r, dir, to)
}

// A run that stopped part of the way is finished by running it again: some
// paths are written, or everything and main moved.
func TestMoveMainFinishesARunThatStopped(t *testing.T) {
	t.Parallel()
	r, dir, from, to, paths := rewritten(t)
	if err := os.WriteFile(filepath.Join(dir, "x.md"), []byte("new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "old.md")); err != nil {
		t.Fatal(err)
	}
	if err := r.MoveMain(from, to, paths); err != nil {
		t.Fatal(err)
	}
	checkMoved(t, r, dir, to)
	if err := r.MoveMain(from, to, paths); err != nil {
		t.Fatalf("again: %v", err)
	}
	checkMoved(t, r, dir, to)
}

func TestMoveMainLeavesAFileEditedByHand(t *testing.T) {
	t.Parallel()
	r, dir, from, to, paths := rewritten(t)
	p := filepath.Join(dir, "x.md")
	if err := os.WriteFile(p, []byte("by hand\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := r.MoveMain(from, to, paths); !errors.Is(err, ErrWorkTreeChanged) {
		t.Fatalf("MoveMain = %v, want ErrWorkTreeChanged", err)
	}
	if b, _ := os.ReadFile(p); string(b) != "by hand\n" {
		t.Errorf("x.md = %q", b)
	}
	if h, err := r.Head(); err != nil || h != from {
		t.Errorf("Head = %s, %v; want %s", h, err, from)
	}
}

func TestMoveMainRefusesAHeadOfNeitherHistory(t *testing.T) {
	t.Parallel()
	r, dir, from, to, paths := rewritten(t)
	commitHead(t, r, dir, map[string]string{"z.md": "z\n"})
	if err := r.MoveMain(from, to, paths); !errors.Is(err, ErrWorkTreeChanged) {
		t.Fatalf("MoveMain = %v, want ErrWorkTreeChanged", err)
	}
}

func TestPruneUnreachableRemovesTheOldHistory(t *testing.T) {
	t.Parallel()
	r, dir, from, to, paths := rewritten(t)
	if err := r.Consolidate(); err != nil { // the old history in a pack too
		t.Fatal(err)
	}
	if err := r.MoveMain(from, to, paths); err != nil {
		t.Fatal(err)
	}
	if err := r.SetTracking("origin", to); err != nil {
		t.Fatal(err)
	}
	if err := r.PruneUnreachable(); err != nil {
		t.Fatal(err)
	}
	// Opened again: a repository keeps the objects it read in memory.
	fresh, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fresh.Close() }()
	if err := fresh.Verify(); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if _, err := fresh.TreeOf(from); !errors.Is(err, plumbing.ErrObjectNotFound) {
		t.Errorf("the old head reads: %v", err)
	}
	if n, err := fresh.LooseObjects(); err != nil || n != 0 {
		t.Errorf("loose = %d, %v", n, err)
	}
	if got := len(packFiles(t, dir)); got != 2 {
		t.Errorf("pack files = %v, want one pack and its index", packFiles(t, dir))
	}
	if b, err := fresh.ReadAt(to, "new.md"); err != nil || string(b) != "added\n" {
		t.Errorf("new.md = %q, %v", b, err)
	}
	// A blob is compressed on disk, so the hash stands for its bytes.
	gone := plumbing.ComputeHash(plumbing.BlobObject, []byte("old secret\n"))
	if ok, err := fresh.hasObject(gone); err != nil || ok {
		t.Errorf("the old blob: %v, %v", ok, err)
	}
	// A run that stopped is finished by the next.
	if err := r.PruneUnreachable(); err != nil {
		t.Fatalf("again: %v", err)
	}
}

// hasObject reports whether the object h is stored.
func (r *Repo) hasObject(h plumbing.Hash) (bool, error) {
	err := r.r.Storer.HasEncodedObject(h)
	if errors.Is(err, plumbing.ErrObjectNotFound) {
		return false, nil
	}
	return err == nil, err
}

func TestPruneUnreachableKeepsWhatTheTrackingRefReaches(t *testing.T) {
	t.Parallel()
	r, _, from, to, paths := rewritten(t)
	if err := r.MoveMain(from, to, paths); err != nil {
		t.Fatal(err)
	}
	if err := r.SetTracking("origin", from); err != nil {
		t.Fatal(err)
	}
	if err := r.PruneUnreachable(); err != nil {
		t.Fatal(err)
	}
	if b, err := r.ReadAt(from, "x.md"); err != nil || !bytes.Equal(b, []byte("old secret\n")) {
		t.Errorf("x.md at from = %q, %v", b, err)
	}
}

func TestKeepMainRefs(t *testing.T) {
	t.Parallel()
	r, _, from, _, _ := rewritten(t)
	others := []plumbing.ReferenceName{"refs/remotes/mirror/main", "refs/tags/v1", "refs/heads/other"}
	for _, n := range others {
		if err := r.r.Storer.SetReference(plumbing.NewHashReference(n, from)); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.KeepMainRefs(); err != nil {
		t.Fatal(err)
	}
	for _, n := range others {
		if _, err := r.r.Storer.Reference(n); !errors.Is(err, plumbing.ErrReferenceNotFound) {
			t.Errorf("%s: %v", n, err)
		}
	}
	for _, n := range []plumbing.ReferenceName{plumbing.HEAD, mainRef, trackingRef("origin")} {
		if _, err := r.r.Storer.Reference(n); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
}
```

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/lib/gitrepo/ -run 'CommitOn|MoveMain|Prune|KeepMainRefs'`
Expected: FAIL（`r.CommitOn undefined` など）

- [ ] **Step 3: 実装する**

```diff
diff --git a/internal/lib/gitrepo/commit_tree.go b/internal/lib/gitrepo/commit_tree.go
index 143cca6..8068a5a 100644
--- a/internal/lib/gitrepo/commit_tree.go
+++ b/internal/lib/gitrepo/commit_tree.go
@@ -70,17 +70,66 @@ func (r *Repo) commitStaged(c Commit) (plumbing.Hash, error) {
 			}
 		}
 	}
+	h, err := r.writeCommit(c, tree, parents)
+	if err != nil {
+		return plumbing.ZeroHash, err
+	}
+	return h, r.moveHead(h)
+}
+
+// writeCommit writes the commit c of tree with parents. It moves nothing.
+func (r *Repo) writeCommit(c Commit, tree plumbing.Hash, parents []plumbing.Hash) (plumbing.Hash, error) {
 	sig := object.Signature{Name: badSignature.ReplaceAllString(c.Author.Name, ""), Email: badSignature.ReplaceAllString(c.Author.Email, ""), When: c.When}
 	commit := &object.Commit{Author: sig, Committer: sig, Message: message(c), TreeHash: tree, ParentHashes: parents}
 	o := r.r.Storer.NewEncodedObject()
 	if err := commit.Encode(o); err != nil {
 		return plumbing.ZeroHash, err
 	}
-	h, err := r.r.Storer.SetEncodedObject(o)
+	return r.r.Storer.SetEncodedObject(o)
+}
+
+// CommitOn writes a commit on parent with files changed (nil removes a file)
+// and returns it. HEAD, the index and the work tree stay as they are, so the
+// commit is reachable from nothing until the caller moves main to it
+// (MoveMain). c.Paths and c.Parents are not used.
+func (r *Repo) CommitOn(parent plumbing.Hash, files map[string][]byte, c Commit) (plumbing.Hash, error) {
+	base, err := r.TreeOf(parent)
 	if err != nil {
 		return plumbing.ZeroHash, err
 	}
-	return h, r.moveHead(h)
+	edits := make(map[string]*index.Entry, len(files))
+	for p, data := range files {
+		if err := checkWorkPath(p); err != nil {
+			return plumbing.ZeroHash, err
+		}
+		if data == nil {
+			edits[p] = nil
+			continue
+		}
+		o := r.r.Storer.NewEncodedObject()
+		o.SetType(plumbing.BlobObject)
+		w, err := o.Writer()
+		if err != nil {
+			return plumbing.ZeroHash, err
+		}
+		_, err = w.Write(data)
+		if cerr := w.Close(); err == nil {
+			err = cerr
+		}
+		if err != nil {
+			return plumbing.ZeroHash, err
+		}
+		h, err := r.r.Storer.SetEncodedObject(o)
+		if err != nil {
+			return plumbing.ZeroHash, err
+		}
+		edits[p] = &index.Entry{Hash: h, Mode: filemode.Regular}
+	}
+	tree, _, err := r.editTree(base, edits, true)
+	if err != nil {
+		return plumbing.ZeroHash, err
+	}
+	return r.writeCommit(c, tree, []plumbing.Hash{parent})
 }
 
 // badSignature is what git does not allow in a name or an email; go-git's
diff --git a/internal/lib/gitrepo/maintain.go b/internal/lib/gitrepo/maintain.go
index bee44d1..6f4354b 100644
--- a/internal/lib/gitrepo/maintain.go
+++ b/internal/lib/gitrepo/maintain.go
@@ -9,6 +9,7 @@ import (
 	"strings"
 
 	"github.com/go-git/go-git/v5/plumbing"
+	"github.com/go-git/go-git/v5/plumbing/filemode"
 	"github.com/go-git/go-git/v5/plumbing/format/idxfile"
 	"github.com/go-git/go-git/v5/plumbing/format/packfile"
 	"github.com/go-git/go-git/v5/plumbing/storer"
@@ -221,6 +222,161 @@ func (r *Repo) ConsolidateBelow(packCap int64) error {
 	return nil
 }
 
+// KeepMainRefs removes every reference but HEAD, refs/heads/main and
+// refs/remotes/origin/*, which is what a fresh clone has of a store.
+func (r *Repo) KeepMainRefs() error {
+	it, err := r.r.Storer.IterReferences()
+	if err != nil {
+		return err
+	}
+	defer it.Close()
+	var drop []plumbing.ReferenceName
+	err = it.ForEach(func(ref *plumbing.Reference) error {
+		n := ref.Name()
+		if n != plumbing.HEAD && n != mainRef && !strings.HasPrefix(n.String(), "refs/remotes/origin/") {
+			drop = append(drop, n)
+		}
+		return nil
+	})
+	if err != nil {
+		return err
+	}
+	for _, n := range drop {
+		if err := r.r.Storer.RemoveReference(n); err != nil {
+			return err
+		}
+	}
+	return nil
+}
+
+// reachable lists every object the references reach: commits, trees, blobs.
+func (r *Repo) reachable() ([]plumbing.Hash, error) {
+	it, err := r.r.Storer.IterReferences()
+	if err != nil {
+		return nil, err
+	}
+	defer it.Close()
+	var todo []plumbing.Hash
+	err = it.ForEach(func(ref *plumbing.Reference) error {
+		if ref.Type() == plumbing.HashReference {
+			todo = append(todo, ref.Hash())
+		}
+		return nil
+	})
+	if err != nil {
+		return nil, err
+	}
+	seen := map[plumbing.Hash]bool{}
+	var out []plumbing.Hash
+	add := func(h plumbing.Hash) bool {
+		if seen[h] {
+			return false
+		}
+		seen[h] = true
+		out = append(out, h)
+		return true
+	}
+	var tree func(h plumbing.Hash) error
+	tree = func(h plumbing.Hash) error {
+		if !add(h) {
+			return nil
+		}
+		t, err := r.r.TreeObject(h)
+		if err != nil {
+			return err
+		}
+		for _, e := range t.Entries {
+			switch e.Mode {
+			case filemode.Dir:
+				if err := tree(e.Hash); err != nil {
+					return err
+				}
+			case filemode.Submodule:
+			default:
+				add(e.Hash)
+			}
+		}
+		return nil
+	}
+	for len(todo) > 0 {
+		h := todo[len(todo)-1]
+		todo = todo[:len(todo)-1]
+		if !add(h) {
+			continue
+		}
+		c, err := r.r.CommitObject(h)
+		if err != nil {
+			return nil, err
+		}
+		if err := tree(c.TreeHash); err != nil {
+			return nil, err
+		}
+		todo = append(todo, c.ParentHashes...)
+	}
+	return out, nil
+}
+
+// PruneUnreachable leaves only what the references reach: it writes that as
+// one new pack, then removes every other pack and every loose object. Run it
+// after a history was rewritten, so that what the rewrite removed is gone
+// from the disk too. A file that cannot be removed (Windows, open file) is an
+// error; the others are removed, and running it again finishes it.
+func (r *Repo) PruneUnreachable() error {
+	los, ok := r.r.Storer.(storer.LooseObjectStorer)
+	if !ok {
+		return errors.New("the object storage of the repository cannot be pruned")
+	}
+	objs, err := r.reachable()
+	if err != nil {
+		return err
+	}
+	var loose []plumbing.Hash
+	if err := los.ForEachObjectHash(func(h plumbing.Hash) error {
+		loose = append(loose, h)
+		return nil
+	}); err != nil {
+		return err
+	}
+	base, err := r.writePack(objs)
+	if err != nil {
+		return err
+	}
+	if ri, ok := r.r.Storer.(interface{ Reindex() }); ok {
+		ri.Reindex()
+	}
+	// The packs must not be open when they are removed (Windows).
+	if err := r.Close(); err != nil {
+		return err
+	}
+	files, err := filepath.Glob(filepath.Join(r.dir, ".git", "objects", "pack", "*"))
+	if err != nil {
+		return err
+	}
+	// The .idx first: a .pack alone is never read.
+	sort.Slice(files, func(i, j int) bool {
+		return strings.HasSuffix(files[i], ".idx") && !strings.HasSuffix(files[j], ".idx")
+	})
+	var errs []error
+	for _, f := range files {
+		if strings.TrimSuffix(f, filepath.Ext(f)) == base {
+			continue
+		}
+		_ = osfs.Chmod(f, 0o600)
+		if err := osfs.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
+			errs = append(errs, err)
+		}
+	}
+	if ri, ok := r.r.Storer.(interface{ Reindex() }); ok {
+		ri.Reindex()
+	}
+	for _, h := range loose {
+		if err := los.DeleteLooseObject(h); err != nil && !errors.Is(err, fs.ErrNotExist) {
+			errs = append(errs, err)
+		}
+	}
+	return errors.Join(errs...)
+}
+
 // afterPackWrite runs once writePack has written the temporary pack; tests
 // replace it to make a later step fail.
 var afterPackWrite = func() error { return nil }
diff --git a/internal/lib/gitrepo/merge.go b/internal/lib/gitrepo/merge.go
index 86b45dc..13dbdc2 100644
--- a/internal/lib/gitrepo/merge.go
+++ b/internal/lib/gitrepo/merge.go
@@ -303,7 +303,28 @@ func blobAt(c *object.Commit, path string) (plumbing.Hash, error) {
 // is as in HEAD or already as in h. Otherwise it returns ErrWorkTreeChanged.
 // Every path is read from h before anything is written, so a commit or object
 // that is missing changes nothing either.
-func (r *Repo) FastForward(h plumbing.Hash, paths []string) (err error) {
+func (r *Repo) FastForward(h plumbing.Hash, paths []string) error {
+	return r.moveMain(h, paths, func(data [][]byte, inTree []bool) error {
+		return r.checkFastForward(h, paths, data, inTree)
+	})
+}
+
+// MoveMain moves main from the commit from to the commit to, which need not
+// descend from it, and writes paths from to: the paths that differ between
+// the two. It is for a history that origin rewrote. HEAD must be on main, at
+// from or at to, and every path on disk as in from or as in to; otherwise it
+// returns ErrWorkTreeChanged, with nothing written. Running it again after it
+// stopped part of the way finishes it.
+func (r *Repo) MoveMain(from, to plumbing.Hash, paths []string) error {
+	return r.moveMain(to, paths, func(data [][]byte, inTree []bool) error {
+		return r.checkMoveMain(from, to, paths, data, inTree)
+	})
+}
+
+// moveMain writes paths from h to the work tree and the index, then points
+// main at h, once check, which is given the paths as in h, has passed. The
+// index is put back when it fails.
+func (r *Repo) moveMain(h plumbing.Hash, paths []string, check func(data [][]byte, inTree []bool) error) (err error) {
 	for _, p := range paths {
 		if err := checkWorkPath(p); err != nil {
 			return err
@@ -332,7 +353,7 @@ func (r *Repo) FastForward(h plumbing.Hash, paths []string) (err error) {
 			return err
 		}
 	}
-	if err := r.checkFastForward(h, paths, data, inTree); err != nil {
+	if err := check(data, inTree); err != nil {
 		return err
 	}
 	before, err := r.r.Storer.Index()
@@ -415,6 +436,44 @@ func (r *Repo) checkFastForward(h plumbing.Hash, paths []string, data [][]byte,
 	return nil
 }
 
+// checkMoveMain returns ErrWorkTreeChanged unless HEAD is a symbolic
+// reference to main at from or to, and each path on disk is as in from or as
+// in h, given by data and inTree.
+func (r *Repo) checkMoveMain(from, to plumbing.Hash, paths []string, data [][]byte, inTree []bool) error {
+	ref, err := r.r.Storer.Reference(plumbing.HEAD)
+	if err != nil {
+		return err
+	}
+	if ref.Type() != plumbing.SymbolicReference || ref.Target() != mainRef {
+		return fmt.Errorf("%w: HEAD is not on main", ErrWorkTreeChanged)
+	}
+	head, err := r.Head()
+	if err != nil {
+		return err
+	}
+	if head != from && head != to {
+		return fmt.Errorf("%w: HEAD is at %s", ErrWorkTreeChanged, head)
+	}
+	for i, p := range paths {
+		got, err := osfs.ReadFile(filepath.Join(r.dir, filepath.FromSlash(p)))
+		if err != nil && !errors.Is(err, fs.ErrNotExist) {
+			return err
+		}
+		onDisk := err == nil
+		if onDisk == inTree[i] && bytes.Equal(got, data[i]) {
+			continue
+		}
+		old, err := r.ReadAt(from, p)
+		if err != nil && !errors.Is(err, fs.ErrNotExist) {
+			return err
+		}
+		if onDisk != (err == nil) || !bytes.Equal(got, old) {
+			return fmt.Errorf("%w: %s differs from both histories", ErrWorkTreeChanged, p)
+		}
+	}
+	return nil
+}
+
 // checkWorkPath refuses a path that is not a file of the work tree.
 func checkWorkPath(p string) error {
 	first, _, _ := strings.Cut(path.Clean(p), "/")
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/lib/gitrepo/`
Expected: PASS（既存の `FastForward` のテストも通る）

- [ ] **Step 5: コミットする**

```bash
git add internal/lib/gitrepo
git commit -m "feat(gitrepo): 早送りでない main の移動と、届かないオブジェクトの削除を足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 2: ディレクトリのコピーと、HEAD の外のコミットを足す

**Files:**
- Modify: `internal/lib/osfs/osfs.go`（`CopyTree`）
- Create: `internal/lib/osfs/copytree_test.go`
- Modify: `internal/app/storage/store/merge.go`（`mergeCommit` を分け、`CommitOn` を足す）

**Interfaces:**
- Consumes: `gitrepo.Repo.CommitOn`（Task 1）
- Produces: `func CopyTree(src, dst string, skip func(rel string) bool) error`、`func (s *Store) CommitOn(parent plumbing.Hash, files map[string][]byte, op Op, trailers []gitrepo.Trailer) (plumbing.Hash, error)`

- [ ] **Step 1: テストを書く**

`internal/lib/osfs/copytree_test.go`:

```go
package osfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyTree(t *testing.T) {
	t.Parallel()
	src := filepath.Join(t.TempDir(), "src")
	dst := src + ".copy"
	if err := os.MkdirAll(filepath.Join(src, "a", "b"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a", "b", "f"), []byte("x"), 0o440); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a", "skip"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("b/f", filepath.Join(src, "a", "link")); err != nil {
		t.Fatal(err)
	}
	if err := CopyTree(src, dst, func(rel string) bool { return rel == "a/skip" }); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dst, "a", "b", "f"))
	if err != nil || info.Mode().Perm() != 0o440 {
		t.Fatalf("f = %v, %v", info, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "a", "b", "f")); string(b) != "x" {
		t.Errorf("f = %q", b)
	}
	if l, err := os.Readlink(filepath.Join(dst, "a", "link")); err != nil || l != "b/f" {
		t.Errorf("link = %q, %v", l, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "a", "skip")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("skip = %v", err)
	}
}
```

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/lib/osfs/ -run CopyTree`
Expected: FAIL（`undefined: CopyTree`）

- [ ] **Step 3: 実装する**

```diff
diff --git a/internal/app/storage/store/merge.go b/internal/app/storage/store/merge.go
index c9381be..d3258bd 100644
--- a/internal/app/storage/store/merge.go
+++ b/internal/app/storage/store/merge.go
@@ -79,25 +79,7 @@ func (s *Store) CommitMerge(head plumbing.Hash, parents []plumbing.Hash, files m
 			}
 			changes = append(changes, change{rel: rel, data: files[rel], old: old, existed: err == nil})
 		}
-		subject := "hedgehug: " + op.Name
-		if op.Detail != "" {
-			subject += " " + op.Detail
-		}
-		ts := []gitrepo.Trailer{{Key: "Hedgehug-Op", Value: op.Name}}
-		if op.Account != "" {
-			ts = append(ts, gitrepo.Trailer{Key: "Hedgehug-Author", Value: op.Account})
-		}
-		c := gitrepo.Commit{
-			Paths:    rels,
-			Parents:  parents,
-			Subject:  subject,
-			Trailers: append(ts, trailers...),
-			Author:   op.Author,
-			When:     s.now().Truncate(time.Second),
-			// The replay after a history rewrite records what lost, with no
-			// file changed.
-			AllowEmpty: op.AllowEmpty,
-		}
+		c := s.mergeCommit(rels, parents, op, trailers)
 		if err := s.apply(changes); err != nil {
 			return err
 		}
@@ -111,6 +93,39 @@ func (s *Store) CommitMerge(head plumbing.Hash, parents []plumbing.Hash, files m
 	return h, err
 }
 
+// mergeCommit is the commit of a merge or a replay.
+func (s *Store) mergeCommit(rels []string, parents []plumbing.Hash, op Op, trailers []gitrepo.Trailer) gitrepo.Commit {
+	subject := "hedgehug: " + op.Name
+	if op.Detail != "" {
+		subject += " " + op.Detail
+	}
+	ts := []gitrepo.Trailer{{Key: "Hedgehug-Op", Value: op.Name}}
+	if op.Account != "" {
+		ts = append(ts, gitrepo.Trailer{Key: "Hedgehug-Author", Value: op.Account})
+	}
+	return gitrepo.Commit{
+		Paths:    rels,
+		Parents:  parents,
+		Subject:  subject,
+		Trailers: append(ts, trailers...),
+		Author:   op.Author,
+		When:     s.now().Truncate(time.Second),
+		// The replay after a history rewrite records what lost, with no
+		// file changed.
+		AllowEmpty: op.AllowEmpty,
+	}
+}
+
+// CommitOn writes a commit on parent with files changed (nil removes a file),
+// as CommitMerge words it. HEAD, the index and the work tree stay as they are:
+// it is for the replay after a history rewrite, which moves main later.
+func (s *Store) CommitOn(parent plumbing.Hash, files map[string][]byte, op Op, trailers []gitrepo.Trailer) (plumbing.Hash, error) {
+	if op.Name == "" {
+		return plumbing.ZeroHash, errors.New("the operation has no name")
+	}
+	return s.repo.CommitOn(parent, files, s.mergeCommit(nil, nil, op, trailers))
+}
+
 // FastForward moves HEAD to the commit to and writes paths from it, under the
 // store lock. It returns ErrHeadMoved when HEAD is not head, and
 // gitrepo.ErrWorkTreeChanged as Repo.FastForward does.
diff --git a/internal/lib/osfs/osfs.go b/internal/lib/osfs/osfs.go
index f3666a7..ebab436 100644
--- a/internal/lib/osfs/osfs.go
+++ b/internal/lib/osfs/osfs.go
@@ -6,6 +6,7 @@
 package osfs
 
 import (
+	"io/fs"
 	"os"
 	"path/filepath"
 )
@@ -45,3 +46,47 @@ func WriteAtomicInDir(path string, data []byte) error {
 	}
 	return nil
 }
+
+// CopyTree copies the directory src to dst, which must not exist: its
+// directories, regular files and symbolic links, with their permissions.
+// skip names what is left out, by its slash-separated path relative to src;
+// nil leaves out nothing.
+func CopyTree(src, dst string, skip func(rel string) bool) error {
+	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
+		if err != nil {
+			return err
+		}
+		rel, err := filepath.Rel(src, p)
+		if err != nil {
+			return err
+		}
+		if skip != nil && rel != "." && skip(filepath.ToSlash(rel)) {
+			if d.IsDir() {
+				return filepath.SkipDir
+			}
+			return nil
+		}
+		to := filepath.Join(dst, rel)
+		info, err := d.Info()
+		if err != nil {
+			return err
+		}
+		switch {
+		case d.IsDir():
+			return os.MkdirAll(to, info.Mode().Perm()|0o700)
+		case d.Type()&fs.ModeSymlink != 0:
+			link, err := os.Readlink(p)
+			if err != nil {
+				return err
+			}
+			return os.Symlink(link, to) //nolint:gosec // G122: src is a store this process holds the lock of, not a tree an attacker can swap
+		case d.Type().IsRegular():
+			b, err := ReadFile(p)
+			if err != nil {
+				return err
+			}
+			return WriteFile(to, b, info.Mode().Perm())
+		}
+		return nil
+	})
+}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/lib/osfs/ ./internal/app/storage/store/`
Expected: PASS

`store.CommitOn` は Task 3 の `rebuild_resume_test.go` で確かめる。

- [ ] **Step 5: コミットする**

```bash
git add internal/lib/osfs internal/app/storage/store/merge.go
git commit -m "feat: ディレクトリのコピーと、HEAD の外のコミットを足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 3: 作り直しを同じディレクトリ内の方式に切り替える

型と関数が一度に変わるので、この Task の途中ではビルドが通らない。Step 3 までまとめて行う。

**Files:**
- Rename: `internal/app/storage/store/leftover.go` → `internal/app/storage/store/rebuildstate.go`
- Create: `internal/app/storage/store/rebuildstate_test.go`
- Modify: `internal/app/remote/replicate/rebuild.go`
- Create: `internal/app/remote/replicate/rebuild_resume_test.go`
- Delete: `internal/app/remote/replicate/rebuild_gap_test.go`・`rebuild_stale_test.go`・`rebuild_keep_test.go`・`rebuild_local_test.go`（差し替えの隙間と keep marker のテストで、対象がなくなる）
- Modify: `internal/app/usecase/session.go`・`sync_shared.go`・`maintenance/rebuild.go`・`maintenance/maintain.go`・`usecasetest/session.go`・`usecasetest/edit.go`
- Modify: `internal/app/storage/mount/mount.go`・`leftover_test.go`・`closerepo_test.go`
- Modify: `internal/app/server/server.go`・`poll_release_test.go`
- Modify: `internal/app/remote/replicate/rebuild_test.go`・`internal/app/usecase/maintenance/rebuild_test.go`・`internal/app/cli/doctor_rewrite_test.go`

**Interfaces:**
- Consumes: `gitrepo.Repo.MoveMain`・`KeepMainRefs`・`PruneUnreachable`（Task 1）、`osfs.CopyTree`・`store.Store.CommitOn`（Task 2）
- Produces: `store.RebuildFile`、`store.RebuildPhase`（`RebuildNone`・`RebuildApply`・`RebuildPrune`）、`func store.RebuildPhaseIn(dir string) RebuildPhase`、`func replicate.RebuildStore(s *store.Store, opt Options) (*Rebuild, error)`、`func replicate.FinishRebuild(s *store.Store) (*Rebuild, error)`、`func (ss *Session) ReleaseForRebuild(dir string)`、`Session.BeforeRebuild func(path string)`

- [ ] **Step 1: 新しいテストを書く**

`internal/app/storage/store/rebuildstate_test.go`:

```go
package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRebuildPhaseIn(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		data string
		want RebuildPhase
	}{
		"none":          {"", RebuildNone},
		"apply":         {`{"phase":"apply"}`, RebuildApply},
		"prune":         {`{"phase":"prune"}`, RebuildPrune},
		"broken":        {"not json", RebuildApply},
		"unknown phase": {`{"phase":"later"}`, RebuildApply},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			local := filepath.Join(dir, filepath.FromSlash(LocalDir))
			if err := os.MkdirAll(local, 0o750); err != nil {
				t.Fatal(err)
			}
			if c.data != "" {
				if err := os.WriteFile(filepath.Join(local, RebuildFile), []byte(c.data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if got := RebuildPhaseIn(dir); got != c.want {
				t.Errorf("phase = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRebuildPhaseInOfAMissingStore(t *testing.T) {
	t.Parallel()
	if got := RebuildPhaseIn(filepath.Join(t.TempDir(), "missing")); got != RebuildNone {
		t.Errorf("phase = %q", got)
	}
}
```

`internal/app/remote/replicate/rebuild_resume_test.go`:

```go
package replicate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// stoppedScene is clone 1 with unpushed work, right after prepare wrote
// rebuild.json: nothing of the store has changed yet.
type stoppedScene struct {
	rec        *rebuildRecord
	b          *Rebuild
	mine, gone *model.Task
	other      *model.Task
	paths      []string
}

func newStoppedScene(t *testing.T, c *cluster, dropped bool) *stoppedScene {
	t.Helper()
	sc := &stoppedScene{}
	c.addTask(0, "keep")
	sc.gone = c.addTask(0, "gone")
	sc.other = c.addTask(0, "other")
	c.sync(0)
	c.sync(1)
	sc.mine = c.addTask(1, "mine")
	c.set(1, sc.other.ULID, func(x *model.Task) { x.Title = "other by b" })
	if dropped {
		c.set(1, sc.gone.ULID, func(x *model.Task) { x.Title = "gone by b" })
	}
	c.rewriteOrigin(sc.gone.ULID)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	held, err := store.LockInOrder(c.stores[1])
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	if sc.rec, sc.b, err = prepare(c.stores[1], c.opts[1]); err != nil || sc.rec == nil {
		t.Fatalf("prepare = %+v, %v", sc.rec, err)
	}
	r := c.stores[1].Repo()
	ft, _ := r.TreeOf(plumbing.NewHash(sc.rec.OldHead))
	tt, _ := r.TreeOf(plumbing.NewHash(sc.rec.Target))
	changes, err := r.DiffTrees(ft, tt, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range changes {
		sc.paths = append(sc.paths, ch.Path)
	}
	return sc
}

// checkRebuilt is what a finished rebuild leaves, however it got there.
func checkRebuilt(t *testing.T, c *cluster, sc *stoppedScene) {
	t.Helper()
	dir := c.stores[1].Dir()
	if p := store.RebuildPhaseIn(dir); p != store.RebuildNone {
		t.Errorf("phase = %q", p)
	}
	c.reopen()
	s := c.stores[1]
	if got := mustHead(t, s); got.String() != sc.rec.Target {
		t.Errorf("HEAD = %s, want %s", got, sc.rec.Target)
	}
	if got, err := s.Repo().RemoteHead("origin"); err != nil || got.String() != sc.rec.Origin {
		t.Errorf("origin = %s, %v; want %s", got, err, sc.rec.Origin)
	}
	if st, err := s.Repo().Status(); err != nil || len(st) != 0 {
		t.Errorf("Status = %v, %v", st, err)
	}
	if _, ok, err := ReadForcePush(s); err != nil || ok {
		t.Errorf("force-push.json = %v, %v", ok, err)
	}
	if got := c.task(1, sc.mine.ULID); got == nil {
		t.Error("the unpushed task is lost")
	}
	if got := c.task(1, sc.other.ULID); got == nil || got.Title != "other by b" {
		t.Errorf("other = %+v", got)
	}
	if c.task(1, sc.gone.ULID) != nil {
		t.Error("the task that is gone came back")
	}
	items, err := inbox.Open(s).List(true)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, it := range items {
		if it.Kind == inbox.RewriteApplied {
			n++
		}
	}
	if n != 1 {
		t.Errorf("rewrite_applied items = %d, want 1: %+v", n, items)
	}
	if err := s.Repo().Verify(); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

// However far a rebuild got, FinishRebuild ends in the same place.
func TestFinishRebuildFromEachStep(t *testing.T) {
	t.Parallel()
	for name, stop := range map[string]func(t *testing.T, c *cluster, sc *stoppedScene){
		"nothing done": func(*testing.T, *cluster, *stoppedScene) {},
		"one file written": func(t *testing.T, c *cluster, sc *stoppedScene) {
			p := model.TaskPath(sc.mine.ULID)
			b, err := c.stores[1].Repo().ReadAt(plumbing.NewHash(sc.rec.Target), p)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(c.stores[1].Dir(), filepath.FromSlash(p)), b, 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"main moved": func(t *testing.T, c *cluster, sc *stoppedScene) {
			if err := c.stores[1].Repo().MoveMain(plumbing.NewHash(sc.rec.OldHead), plumbing.NewHash(sc.rec.Target), sc.paths); err != nil {
				t.Fatal(err)
			}
		},
		"applied, phase not written": func(t *testing.T, c *cluster, sc *stoppedScene) {
			if err := apply(c.stores[1], sc.rec, &Rebuild{}); err != nil {
				t.Fatal(err)
			}
		},
		"applied, phase written": func(t *testing.T, c *cluster, sc *stoppedScene) {
			if err := apply(c.stores[1], sc.rec, &Rebuild{}); err != nil {
				t.Fatal(err)
			}
			sc.rec.Phase = store.RebuildPrune
			if err := writeRecord(c.stores[1], sc.rec); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := newCluster(t, 2)
			sc := newStoppedScene(t, c, false)
			stop(t, c, sc)
			c.reopen() // a new process
			b, err := FinishRebuild(c.stores[1])
			if err != nil || b == nil {
				t.Fatalf("FinishRebuild = %+v, %v", b, err)
			}
			checkRebuilt(t, c, sc)
			// And again: nothing is left to do.
			if b, err := FinishRebuild(c.stores[1]); err != nil || b != nil {
				t.Errorf("second FinishRebuild = %+v, %v", b, err)
			}
		})
	}
}

// The old history is gone from the disk with the rebuild; nothing was kept.
func TestRebuildRemovesTheOldHistory(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	sc := newStoppedScene(t, c, false)
	if b, err := FinishRebuild(c.stores[1]); err != nil || b == nil {
		t.Fatalf("FinishRebuild = %+v, %v", b, err)
	}
	r, err := gitrepo.Open(c.stores[1].Dir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	if _, err := r.TreeOf(plumbing.NewHash(sc.rec.OldHead)); !errors.Is(err, plumbing.ErrObjectNotFound) {
		t.Errorf("the old head reads: %v", err)
	}
	if err := r.Verify(); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

// With changes that cannot be put back, a copy of the store as it was is
// kept, with the old history, and a copy cut short is cleared away.
func TestRebuildKeepsACopyOfTheOldStore(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	if err := os.MkdirAll(c.stores[1].Dir()+keptNew, 0o750); err != nil {
		t.Fatal(err)
	}
	sc := newStoppedScene(t, c, true)
	if !strings.HasPrefix(sc.b.Kept, c.stores[1].Dir()+keptPrefix) || sc.b.Kept == c.stores[1].Dir()+keptNew {
		t.Fatalf("Kept = %q", sc.b.Kept)
	}
	if exists(t, c.stores[1].Dir()+keptNew) {
		t.Error("the copy in progress is left")
	}
	r, err := gitrepo.Open(sc.b.Kept)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	if h, err := r.Head(); err != nil || h.String() != sc.rec.OldHead {
		t.Errorf("Head of the copy = %s, %v; want %s", h, err, sc.rec.OldHead)
	}
	b, err := r.ReadAt(plumbing.NewHash(sc.rec.OldHead), model.TaskPath(sc.gone.ULID))
	if err != nil || !strings.Contains(string(b), "gone by b") {
		t.Errorf("the change that was not put back = %q, %v", b, err)
	}
	if exists(t, filepath.Join(sc.b.Kept, filepath.FromSlash(store.LocalDir), "lock")) {
		t.Error("the lock was copied")
	}
	b2, err := FinishRebuild(c.stores[1])
	if err != nil || b2.Kept != sc.b.Kept {
		t.Fatalf("FinishRebuild = %+v, %v", b2, err)
	}
	if !exists(t, sc.b.Kept) {
		t.Error("the copy was removed")
	}
}

// A removal that fails leaves the store whole, with a warning; the next run
// finishes it.
func TestFinishRebuildRetriesThePrune(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	sc := newStoppedScene(t, c, false)
	dir := c.stores[1].Dir()
	unlock := gitrepotest.LockObjects(t, filepath.Join(dir, ".git"))
	b, err := FinishRebuild(c.stores[1])
	unlock()
	if err != nil || b == nil || len(b.Warnings) != 1 {
		t.Fatalf("FinishRebuild = %+v, %v; want one warning", b, err)
	}
	if p := store.RebuildPhaseIn(dir); p != store.RebuildPrune {
		t.Errorf("phase = %q", p)
	}
	if b, err = FinishRebuild(c.stores[1]); err != nil || len(b.Warnings) != 0 {
		t.Fatalf("second FinishRebuild = %+v, %v", b, err)
	}
	checkRebuilt(t, c, sc)
}

// A write that comes during the rebuild waits for the store lock, and lands
// on the new history.
func TestAWriteWaitsForTheRebuild(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	gone := c.addTask(0, "gone")
	keep := c.addTask(0, "keep")
	c.sync(0)
	c.sync(1)
	c.rewriteOrigin(gone.ULID)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	s := c.stores[1]
	// As another process would: a store of its own, opened before the rebuild.
	other, err := store.Open(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = other.Repo().Close() }()
	held, err := store.LockInOrder(s)
	if err != nil {
		t.Fatal(err)
	}
	rec, b, err := prepare(s, c.opts[1])
	if err != nil || rec == nil {
		t.Fatalf("prepare = %+v, %v", rec, err)
	}
	done := make(chan error)
	go func() {
		_, err := other.Update(c.op(1, "update", keep.ULID), func(tx *store.Tx) error {
			x, err := tx.Task(keep.ULID)
			if err != nil {
				return err
			}
			x.Title = "late"
			return tx.PutTask(x)
		})
		done <- err
	}()
	time.Sleep(50 * time.Millisecond) // the write is waiting, or has not begun
	if err := finish(s, rec, b); err != nil {
		t.Fatal(err)
	}
	held.Release()
	if err := <-done; err != nil {
		t.Fatalf("the write: %v", err)
	}
	c.reopen()
	s = c.stores[1]
	if h, err := s.Repo().Between(mustHead(t, s), plumbing.NewHash(rec.Target)); err != nil || len(h) != 1 {
		t.Fatalf("commits above the target = %+v, %v", h, err)
	}
	if got := c.task(1, keep.ULID); got == nil || got.Title != "late" {
		t.Errorf("keep = %+v", got)
	}
}
```

`internal/app/storage/mount/leftover_test.go`:

```go
package mount

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/storage/mount/mounttest"

	"github.com/kukv/hedgehug/internal/app/storage/store"
)

func writeRebuild(t *testing.T, dir, phase string) {
	t.Helper()
	local := filepath.Join(dir, filepath.FromSlash(store.LocalDir))
	if err := os.MkdirAll(local, 0o750); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(local, store.RebuildFile)
	if err := os.WriteFile(p, []byte(`{"phase":"`+phase+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A rebuild that stopped is told of: a store with only the old objects left
// is in Unfinished, one with a work tree part old, part new is skipped with
// the same reason.
func TestOpenTellsOfAnUnfinishedRebuild(t *testing.T) {
	t.Parallel()
	prune := mounttest.NewStoreAt(t, "prune", "PRN")
	apply := mounttest.NewStoreAt(t, "apply", "APL")
	writeRebuild(t, prune.Path, "prune")
	writeRebuild(t, apply.Path, "apply")
	set := openSet(t, prune, apply, mounttest.NewStoreAt(t, "plain", "PLN"))
	if len(set.Stores) != 2 || set.Stores[0].Name != "prune" || set.Stores[1].Name != "plain" {
		t.Errorf("stores = %v", set.Stores)
	}
	var le *store.LeftoverError
	if len(set.Unfinished) != 1 || set.Unfinished[0].Name != "prune" || !errors.As(set.Unfinished[0].Err, &le) || le.Name != "prune" {
		t.Errorf("Unfinished = %+v", set.Unfinished)
	}
	if len(set.Skipped) != 1 || set.Skipped[0].Name != "apply" || !errors.As(set.Skipped[0].Err, &le) || le.Name != "apply" {
		t.Errorf("Skipped = %+v", set.Skipped)
	}
}
```

`internal/app/cli/doctor_rewrite_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// A rebuild that stopped is finished by doctor --fix; until then the store
// does not open.
func TestDoctorFixFinishesAStoppedRebuild(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("add", "Alpha")
	dir := te.storeDir("me")
	h := gitrepotest.HeadHash(t, dir)
	rec := `{"phase":"apply","old_head":"` + h + `","target":"` + h + `","origin":"` + h + `","mark":"` + h + `"}`
	if err := os.WriteFile(filepath.Join(dir, ".hedgehug", "local", "rebuild.json"), []byte(rec), 0o600); err != nil {
		t.Fatal(err)
	}

	if code, out, _ := te.run("doctor"); code == 0 || strings.Contains(out, "No problems") {
		t.Errorf("doctor of a store that stopped: exit %d\n%s", code, out)
	}
	if code, out, stderr := te.run("doctor", "--fix"); code != 0 || !strings.Contains(out, "No problems") {
		t.Errorf("--fix: exit %d\nstdout %s\nstderr %s", code, out, stderr)
	}
	if err := te.uc().NoLeftover(dir); err != nil {
		t.Errorf("leftover = %v", err)
	}
	if out := te.ok("ls"); !strings.Contains(out, "Alpha") {
		t.Errorf("ls after the fix:\n%s", out)
	}
}
```

- [ ] **Step 2: store を切り替える**

```bash
git mv internal/app/storage/store/leftover.go internal/app/storage/store/rebuildstate.go
```

`internal/app/storage/store/rebuildstate.go`:

```go
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/kukv/hedgehug/internal/lib/osfs"
)

// RebuildFile is the file of .hedgehug/local that says a rebuild after a
// history rewrite (replicate.RebuildStore) is under way or stopped.
const RebuildFile = "rebuild.json"

// RebuildPhase is how far a rebuild got.
type RebuildPhase string

const (
	RebuildNone  RebuildPhase = ""      // no rebuild
	RebuildApply RebuildPhase = "apply" // the work tree may be part old, part new
	RebuildPrune RebuildPhase = "prune" // the store is whole; old objects are left
)

// RebuildPhaseIn tells the phase of the rebuild of the store at dir. A
// rebuild.json that cannot be read is taken for RebuildApply: nothing says
// the store is whole.
func RebuildPhaseIn(dir string) RebuildPhase {
	b, err := osfs.ReadFile(filepath.Join(dir, filepath.FromSlash(LocalDir), RebuildFile))
	if errors.Is(err, fs.ErrNotExist) {
		return RebuildNone
	}
	var rec struct {
		Phase RebuildPhase `json:"phase"`
	}
	if err != nil || json.Unmarshal(b, &rec) != nil || rec.Phase != RebuildPrune {
		return RebuildApply
	}
	return RebuildPrune
}

// LeftoverMessage is what Hedgehug tells of an unfinished rebuild of the
// store called name.
func LeftoverMessage(name string) string {
	return fmt.Sprintf("the rebuild of %s after a history rewrite is unfinished (hedgehug doctor --fix finishes it)", name)
}

// LeftoverError says a store has a rebuild left unfinished.
type LeftoverError struct{ Name string }

func (e *LeftoverError) Error() string { return LeftoverMessage(e.Name) }
```

- [ ] **Step 3: replicate を書き換える**

`internal/app/remote/replicate/rebuild.go` の全文を次にする。

```go
package replicate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
	"github.com/kukv/hedgehug/internal/lib/osfs"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

const (
	// keptPrefix names the copy of the old store left beside the store when
	// the rebuild could not put back all its unpushed changes. Nothing
	// removes it.
	keptPrefix = ".kept-"
	// keptNew is the copy while it is written. It is removed when the next
	// rebuild starts.
	keptNew = ".kept-new"
	// indexCache is the cache of the index, which names trees of the old
	// history.
	indexCache = "index"
)

// ErrDirtyRebuild means the store has changes that were not committed, which
// the rebuild would drop. It is an ErrDirty.
var ErrDirtyRebuild error = dirtyRebuildError{}

type dirtyRebuildError struct{}

func (dirtyRebuildError) Error() string {
	return "the store has uncommitted changes, which the rebuild would drop; commit or restore them, then run hedgehug doctor --fix"
}

func (dirtyRebuildError) Is(target error) bool { return target == ErrDirty }

// Rebuild tells what RebuildStore or FinishRebuild did.
type Rebuild struct {
	Replayed int // the unpushed changes (commits) put back
	// Purged are the ids of the tasks whose unpushed changes were not put
	// back, because the new history lacks the task (the ULID for one the
	// old history has no id of), and PurgedTasks their ULIDs, in the same
	// order.
	Purged      []string
	PurgedTasks []model.ULID
	// Notices are the items put in the inbox.
	Notices []inbox.Item
	// Kept is the copy of the old store; "" when none was made. It is made
	// when Purged is not empty.
	Kept string
	// Warnings are what failed after the store was rebuilt: the removal of
	// the old objects, which the next doctor --fix or sync tries again.
	Warnings []string
}

// rebuildRecord is .hedgehug/local/rebuild.json: what it takes to finish a
// rebuild that stopped, without origin.
type rebuildRecord struct {
	Phase   store.RebuildPhase `json:"phase"`
	OldHead string             `json:"old_head"` // HEAD before the rebuild
	Target  string             `json:"target"`   // where main goes
	Origin  string             `json:"origin"`   // the new head of origin
	Mark    string             `json:"mark"`     // the commit the inbox goes on from
	Items   []inbox.Item       `json:"items"`    // for the inbox; Add leaves out what it has
	Kept    string             `json:"kept,omitempty"`
}

func writeRecord(s *store.Store, rec *rebuildRecord) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.WriteLocal(store.RebuildFile, append(b, '\n'))
}

// readRecord reads rebuild.json; ok is false when there is none.
func readRecord(s *store.Store) (rec *rebuildRecord, ok bool, err error) {
	b, err := s.ReadLocal(store.RebuildFile)
	if errors.Is(err, store.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	rec = &rebuildRecord{}
	if err := json.Unmarshal(b, rec); err != nil {
		return nil, false, fmt.Errorf("%s: %w", store.RebuildFile, err)
	}
	if rec.Phase != store.RebuildApply && rec.Phase != store.RebuildPrune {
		return nil, false, fmt.Errorf("%s: unknown phase %q", store.RebuildFile, rec.Phase)
	}
	return rec, true, nil
}

// RebuildStore brings the store onto the new history of origin in its own
// directory, for hedgehug doctor --fix after a sync stopped with ErrForcePush.
// The unpushed changes are put back as one commit; a copy of the store is
// kept when some cannot be. From the moment rebuild.json is written the store
// only goes forward: what stops is finished by FinishRebuild. Before that the
// store is as it was. When origin has become a history the store follows
// again, it removes force-push.json and returns nil. The caller holds
// sync.lock; the store lock is held here for the whole rebuild.
func RebuildStore(s *store.Store, opt Options) (*Rebuild, error) {
	held, err := store.LockInOrder(s)
	if err != nil {
		return nil, err
	}
	defer held.Release()
	rec, b, err := prepare(s, opt)
	if err != nil || rec == nil {
		return nil, err
	}
	if err := finish(s, rec, b); err != nil {
		return nil, err
	}
	return b, nil
}

// prepare does all of a rebuild that leaves main as it is, up to the writing
// of rebuild.json, under the store lock. rec is nil when origin follows the
// store again.
func prepare(s *store.Store, opt Options) (rec *rebuildRecord, b *Rebuild, err error) {
	if opt.Now == nil {
		opt.Now = time.Now
	}
	r := s.Repo()
	r.SetConn(opt.Conn)
	st, err := r.Status()
	if err != nil {
		return nil, nil, err
	}
	if len(st) > 0 {
		return nil, nil, ErrDirtyRebuild
	}
	oldHead, err := r.Head()
	if err != nil {
		return nil, nil, err
	}
	// What the store followed: the sync that found the force push put the
	// tracking ref back.
	tracked, err := r.RemoteHead("origin")
	if err != nil {
		return nil, nil, err
	}
	dir := s.Dir()
	if err := osfs.RemoveAll(dir + keptNew); err != nil {
		return nil, nil, err
	}
	if err := r.SetRemotes(opt.Remotes); err != nil {
		return nil, nil, err
	}
	head, err := r.Fetch("origin")
	if serr := r.SetTracking("origin", tracked); err == nil {
		err = serr
	}
	if err != nil {
		return nil, nil, fmt.Errorf("rebuild after a force push: %w", err)
	}
	if head.IsZero() {
		return nil, nil, errors.New("rebuild after a force push: origin is empty")
	}
	follows, err := r.IsAncestor(tracked, head)
	if err != nil {
		return nil, nil, err
	}
	if follows {
		// origin went back to a history the store follows: the next sync
		// takes it as usual.
		return nil, nil, s.RemoveLocal(forcePushFile)
	}
	b = &Rebuild{}
	s.SetNow(opt.Now)
	target, err := replay(s, tracked, oldHead, head, opt, b)
	if err != nil {
		return nil, nil, err
	}
	// The inbox goes on from where the new history has the commit it had
	// followed: what came before is what it has told of, what comes after is
	// new to it, and so is the replay commit.
	mark, found, err := r.FindSameCommit(head, tracked)
	if err != nil {
		return nil, nil, err
	}
	if !found {
		mark = head
	}
	items, err := rewriteItems(target, opt, b)
	if err != nil {
		return nil, nil, err
	}
	if len(b.Purged) > 0 {
		if b.Kept, err = keepOldStore(dir); err != nil {
			return nil, nil, err
		}
	}
	rec = &rebuildRecord{
		Phase: store.RebuildApply, OldHead: oldHead.String(), Target: target.String(),
		Origin: head.String(), Mark: mark.String(), Items: items, Kept: b.Kept,
	}
	if err := writeRecord(s, rec); err != nil {
		return nil, nil, err
	}
	return rec, b, nil
}

// FinishRebuild finishes a rebuild that stopped, from rebuild.json; it needs
// no connection. It returns nil when there is none. The caller holds
// sync.lock; the store lock is held here.
func FinishRebuild(s *store.Store) (*Rebuild, error) {
	held, err := store.LockInOrder(s)
	if err != nil {
		return nil, err
	}
	defer held.Release()
	rec, ok, err := readRecord(s)
	if err != nil || !ok {
		return nil, err
	}
	b := &Rebuild{Kept: rec.Kept}
	if err := finish(s, rec, b); err != nil {
		return nil, err
	}
	return b, nil
}

// finish does what rebuild.json says is left. Every step may be run again.
func finish(s *store.Store, rec *rebuildRecord, b *Rebuild) error {
	if rec.Phase == store.RebuildApply {
		if err := apply(s, rec, b); err != nil {
			return fmt.Errorf("finish the rebuild: %w", err)
		}
		rec.Phase = store.RebuildPrune
		if err := writeRecord(s, rec); err != nil {
			return fmt.Errorf("finish the rebuild: %w", err)
		}
	}
	r := s.Repo()
	err := r.KeepMainRefs()
	if err == nil {
		err = r.PruneUnreachable()
	}
	if err == nil {
		err = s.RemoveLocal(store.RebuildFile)
	}
	if err != nil {
		// The store is whole: only what the history no longer has is left.
		b.Warnings = append(b.Warnings, fmt.Sprintf("remove the objects of the old history: %v", err))
	}
	return nil
}

// apply makes the store the rebuilt one: the work tree, main and the
// tracking ref, the inbox, and what was about the old history.
func apply(s *store.Store, rec *rebuildRecord, b *Rebuild) error {
	r := s.Repo()
	from, to := plumbing.NewHash(rec.OldHead), plumbing.NewHash(rec.Target)
	ft, err := r.TreeOf(from)
	if err != nil {
		return err
	}
	tt, err := r.TreeOf(to)
	if err != nil {
		return err
	}
	changes, err := r.DiffTrees(ft, tt, "")
	if err != nil {
		return err
	}
	paths := make([]string, len(changes))
	for i, c := range changes {
		paths[i] = c.Path
	}
	if err := r.MoveMain(from, to, paths); err != nil {
		return err
	}
	if err := r.SetTracking("origin", plumbing.NewHash(rec.Origin)); err != nil {
		return err
	}
	box := inbox.Open(s)
	if err := box.SetMark(plumbing.NewHash(rec.Mark)); err != nil {
		return err
	}
	if b.Notices, err = box.Add(rec.Items); err != nil {
		return err
	}
	if err := s.RemoveLocal(indexCache); err != nil {
		return err
	}
	return s.RemoveLocal(forcePushFile)
}

// skipKept leaves the locks out of the copy of the store: they are not its
// content, and another process may have them open.
func skipKept(rel string) bool {
	name := path.Base(rel)
	return strings.HasPrefix(rel, store.LocalDir+"/") && (name == "lock" || strings.HasSuffix(name, ".lock"))
}

// keepOldStore copies the store at dir to dir + keptPrefix + the time, and
// returns where. It holds the changes the rebuild cannot put back.
func keepOldStore(dir string) (string, error) {
	tmp := dir + keptNew
	if err := osfs.CopyTree(dir, tmp, skipKept); err != nil {
		return "", fmt.Errorf("keep the old store: %w", err)
	}
	kept := dir + keptPrefix + time.Now().UTC().Format("20060102T150405.000000000")
	if err := osfs.Rename(tmp, kept); err != nil {
		return "", fmt.Errorf("keep the old store: %w", err)
	}
	return kept, nil
}

// taskOfPath is the task a file of the store belongs to: its own file, or a
// comment or worklog of it.
func taskOfPath(p string) (model.ULID, bool) {
	if u, err := model.ParseTaskPath(p); err == nil {
		return u, true
	}
	if u, _, err := model.ParseCommentPath(p); err == nil {
		return u, true
	}
	if u, _, err := model.ParseWorklogPath(p); err == nil {
		return u, true
	}
	return model.ULID{}, false
}

// replay puts the unpushed changes of oldHead back on head, the new head of
// origin, as one commit off main; it returns that commit, or head when there
// is nothing to put back. The files of
// the tasks the new history lacks are left out of the merge, and a task of
// them that the unpushed commits changed goes to b.Purged.
func replay(s *store.Store, tracked, oldHead, head plumbing.Hash, opt Options, b *Rebuild) (plumbing.Hash, error) {
	r := s.Repo()
	mine, err := r.Between(oldHead, tracked)
	if err != nil {
		return head, err
	}
	if len(mine) == 0 {
		return head, nil
	}
	b.Replayed = len(mine)
	base, err := r.MergeBase(oldHead, tracked)
	if err != nil {
		return head, err
	}
	if base.IsZero() {
		return head, fmt.Errorf("replay: %w", ErrUnrelated)
	}
	dropped, err := findDropped(r, base, oldHead, head, b)
	if err != nil {
		return head, err
	}
	skip := func(p string) bool {
		u, ok := taskOfPath(p)
		return ok && dropped[u]
	}
	// The merge is computed from the new head's side, so that its files are
	// the changes to make to the clone; the rule is symmetric.
	opt.origin, opt.skip = head, skip
	m, err := mergeTrees(r, nil, base, head, oldHead, opt) // head is new on origin: no index has it
	if err != nil {
		return head, err
	}
	// What the new history already has is no change.
	for p, data := range m.files {
		cur, err := r.ReadAt(head, p)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return head, err
		}
		if (data == nil && err != nil) || (data != nil && err == nil && string(cur) == string(data)) {
			delete(m.files, p)
		}
	}
	// A commit without a file records what lost, as a merge commit does.
	if len(m.files) == 0 && len(m.lost) == 0 && len(m.notices) == 0 && len(m.repaired) == 0 {
		return head, nil
	}
	op := store.Op{Name: "replay", Detail: plural.Count(len(mine), "unpushed commit"), Author: opt.Author, Account: opt.Account, AllowEmpty: true}
	return s.CommitOn(head, m.files, op, m.trailers())
}

// findDropped puts in b the tasks that the commits from base to oldHead
// changed (the task, a comment or a worklog entry) and that the new history
// lacks: the base has the task's file and the new head does not. A task
// that only the unpushed commits have is not one of them. It returns them as
// a set.
func findDropped(r *gitrepo.Repo, base, oldHead, head plumbing.Hash, b *Rebuild) (map[model.ULID]bool, error) {
	bt, err := r.TreeOf(base)
	if err != nil {
		return nil, err
	}
	ot, err := r.TreeOf(oldHead)
	if err != nil {
		return nil, err
	}
	changes, err := r.DiffTrees(bt, ot, "")
	if err != nil {
		return nil, err
	}
	dropped := map[model.ULID]bool{}
	seen := map[model.ULID]bool{}
	for _, c := range changes {
		u, ok := taskOfPath(c.Path)
		if !ok || seen[u] {
			continue
		}
		seen[u] = true
		data, err := r.ReadFile(bt, model.TaskPath(u))
		if errors.Is(err, fs.ErrNotExist) {
			continue // made by the unpushed commits
		}
		if err != nil {
			return nil, err
		}
		if _, err := r.ReadAt(head, model.TaskPath(u)); err == nil {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		id := u.String()
		if t, err := model.DecodeTask(data); err == nil && t.ID != "" {
			id = t.ID
		}
		dropped[u] = true
		b.PurgedTasks = append(b.PurgedTasks, u)
		b.Purged = append(b.Purged, id)
	}
	return dropped, nil
}

// rewriteItems are the notices of spec step 7 for the rebuild to the commit
// head.
func rewriteItems(head plumbing.Hash, opt Options, b *Rebuild) ([]inbox.Item, error) {
	to := opt.Account
	if to == "" && len(opt.Me) > 0 {
		to = opt.Me[0]
	}
	now := opt.Now()
	detail, err := json.Marshal(struct {
		Store    string `json:"store"`
		Replayed int    `json:"replayed"`
	}{opt.Name, b.Replayed})
	if err != nil {
		return nil, err
	}
	items := []inbox.Item{{Kind: inbox.RewriteApplied, Commit: head.String(), To: to, Time: now, Detail: detail}}
	for i, u := range b.PurgedTasks {
		items = append(items, inbox.Item{Kind: inbox.PurgedChange, Task: u, ID: b.Purged[i], Commit: head.String(), To: to, Time: now})
	}
	return items, nil
}
```

- [ ] **Step 4: usecase・mount・server を直す**

```diff
diff --git a/internal/app/server/server.go b/internal/app/server/server.go
index 2627c19..4309aca 100644
--- a/internal/app/server/server.go
+++ b/internal/app/server/server.go
@@ -161,9 +161,9 @@ func (s *Server) session(storeName string) *usecase.Session {
 		StoreName:  storeName,
 		Warn:       func(m string) { s.logf("warning: %s", m) },
 		AfterWrite: func(string) { s.loop.Wake() },
-		// doctor --fix swaps a rebuilt store in (after a force push) and
+		// doctor --fix rebuilds a store (after a force push) and
 		// needs the files of the old one closed.
-		BeforeSwap: func(path string) { s.poller.Release(path) },
+		BeforeRebuild: func(path string) { s.poller.Release(path) },
 	}
 }
 
diff --git a/internal/app/storage/mount/mount.go b/internal/app/storage/mount/mount.go
index 9269b95..d3634f7 100644
--- a/internal/app/storage/mount/mount.go
+++ b/internal/app/storage/mount/mount.go
@@ -3,8 +3,6 @@
 package mount
 
 import (
-	"io/fs"
-	"path/filepath"
 	"strings"
 	gosync "sync"
 	"time"
@@ -134,9 +132,9 @@ type Skipped struct {
 type Set struct {
 	Stores  []*Mounted
 	Skipped []Skipped
-	// Unfinished are the stores that opened but have a rebuild after a
-	// history rewrite left beside them (store.LeftoverError). A store that
-	// did not open for want of its directory is in Skipped with the same
+	// Unfinished are the stores that opened with the last step of a rebuild
+	// after a history rewrite left to do (store.LeftoverError). A store whose
+	// rebuild stopped earlier does not open: it is in Skipped with the same
 	// error.
 	Unfinished []Skipped
 
@@ -167,23 +165,18 @@ func (s *Set) Unopened(sk Skipped) {
 // goes to Skipped with why.
 func Open(c *config.Config, opt Options) *Set {
 	set := &Set{unopened: opt.Unopened}
-	parents := map[string][]fs.DirEntry{} // read once for the stores in one directory
-	perr := map[string]error{}
 	for _, e := range c.Stores {
-		s, err := store.Open(e.Path)
-		parent := filepath.Dir(filepath.Clean(e.Path))
-		if _, ok := parents[parent]; !ok {
-			parents[parent], perr[parent] = store.ReadParent(e.Path)
+		phase := store.RebuildPhaseIn(e.Path)
+		if phase == store.RebuildApply {
+			set.Skipped = append(set.Skipped, Skipped{Name: e.Name, Err: &store.LeftoverError{Name: e.Name}})
+			continue
 		}
-		unfinished := perr[parent] == nil && !store.LeftoverIn(e.Path, parents[parent]).None()
+		s, err := store.Open(e.Path)
 		if err != nil {
-			if unfinished {
-				err = &store.LeftoverError{Name: e.Name}
-			}
 			set.Skipped = append(set.Skipped, Skipped{Name: e.Name, Err: err})
 			continue
 		}
-		if unfinished {
+		if phase == store.RebuildPrune {
 			set.Unfinished = append(set.Unfinished, Skipped{Name: e.Name, Err: &store.LeftoverError{Name: e.Name}})
 		}
 		if opt.Now != nil {
diff --git a/internal/app/usecase/maintenance/maintain.go b/internal/app/usecase/maintenance/maintain.go
index 37af3d9..5696582 100644
--- a/internal/app/usecase/maintenance/maintain.go
+++ b/internal/app/usecase/maintenance/maintain.go
@@ -56,8 +56,8 @@ func Doctor(ss *usecase.Session, fix bool) ([]DoctorReport, error) {
 		}
 		entries = []config.StoreEntry{e}
 	}
-	// A swap that stopped is finished first, so the store opens. A store a
-	// sync found force pushed is then cloned again, before the repairs: they
+	// A rebuild that stopped is finished first, so the store opens. A store a
+	// sync found force pushed is then rebuilt, before the repairs: they
 	// are made on the new history. One that fails is that store's finding;
 	// the others go on.
 	finishErr := map[string]error{}
@@ -168,9 +168,14 @@ func doctorStore(ss *usecase.Session, e config.StoreEntry, only bool, repaired [
 	if openErr == nil {
 		defer func() { _ = opened.Repo().Close() }()
 	}
-	fs, err := Run(e.Path, ss.Getenv)
-	if err != nil {
-		return DoctorReport{}, err
+	// A rebuild that stopped leaves a work tree part old, part new: what the
+	// checks would say of it means nothing.
+	var fs []usecase.Finding
+	if left := (*store.LeftoverError)(nil); !errors.As(openErr, &left) {
+		var err error
+		if fs, err = Run(e.Path, ss.Getenv); err != nil {
+			return DoctorReport{}, err
+		}
 	}
 	if openErr != nil && len(fs) == 0 {
 		if only {
@@ -210,12 +215,12 @@ func withRebuildFailure(fs []usecase.Finding, name string, rerr error) []usecase
 	const hint = "fix the cause and run hedgehug doctor --fix again"
 	for i := range fs {
 		if fs[i].Check == "unexpected_force_push" {
-			fs[i].Msg += "; cannot clone the store again: " + rerr.Error()
+			fs[i].Msg += "; cannot rebuild the store: " + rerr.Error()
 			fs[i].Hint = hint
 			return fs
 		}
 	}
-	return append(fs, usecase.Finding{Check: "unexpected_force_push", Where: name, Msg: "cannot clone the store again: " + rerr.Error(), Hint: hint})
+	return append(fs, usecase.Finding{Check: "unexpected_force_push", Where: name, Msg: "cannot rebuild the store: " + rerr.Error(), Hint: hint})
 }
 
 // fixStore repairs the files of HEAD of the store e, opened as s, as a sync
diff --git a/internal/app/usecase/maintenance/rebuild.go b/internal/app/usecase/maintenance/rebuild.go
index 0c80a28..f6b56df 100644
--- a/internal/app/usecase/maintenance/rebuild.go
+++ b/internal/app/usecase/maintenance/rebuild.go
@@ -1,7 +1,6 @@
 package maintenance
 
 import (
-	"errors"
 	"fmt"
 
 	"github.com/kukv/hedgehug/internal/app/remote/replicate"
@@ -11,19 +10,17 @@ import (
 )
 
 // Rebuilt is what doctor --fix did to a store a sync found force pushed: it
-// cloned the store again from origin.
+// brought the store onto the new history of origin.
 type Rebuilt struct {
 	Replayed int      `json:"replayed"`       // the unpushed commits put back, as one commit
 	Dropped  []string `json:"dropped"`        // the ids of the tasks the new history lacks, whose unpushed changes were not put back
 	Kept     string   `json:"kept,omitempty"` // where the old store is, when some changes were not put back
 }
 
-// rebuildStore clones the store e again from origin and puts the unpushed
-// changes back (replicate.MakeRebuild), when a sync found its origin force pushed
-// (force-push.json). It swaps the new store in after the sync lock is let
-// go, and does not push. It returns nil when the store was not force pushed,
-// does not open, origin follows it again, or the store changed while it was
-// made again (a warning says to run doctor --fix again).
+// rebuildStore brings the store e onto the new history of origin and puts the
+// unpushed changes back (replicate.RebuildStore), when a sync found its origin
+// force pushed (force-push.json). It does not push. It returns nil when the
+// store was not force pushed, does not open, or origin follows it again.
 func rebuildStore(ss *usecase.Session, c *config.Config, e config.StoreEntry) (*Rebuilt, error) {
 	s, err := ss.OpenStoreAt(e.Path)
 	if err != nil {
@@ -37,45 +34,32 @@ func rebuildStore(ss *usecase.Session, c *config.Config, e config.StoreEntry) (*
 	if err != nil {
 		return nil, err
 	}
+	dir := s.Dir()
+	ss.ReleaseForRebuild(dir)
 	var b *replicate.Rebuild
 	err = s.WithLocalLock("sync.lock", usecase.SyncLockWait, func() error {
 		var err error
-		b, err = replicate.MakeRebuild(s, opt)
+		b, err = replicate.RebuildStore(s, opt)
 		return err
 	})
 	if err != nil || b == nil {
 		return nil, err
 	}
-	dir := s.Dir()
-	if ss.BeforeSwap != nil {
-		ss.BeforeSwap(dir)
-	}
-	ss.CloseMounted(dir)
-	ss.DropMount()
-	if err := s.Repo().Close(); err != nil {
-		replicate.Discard(b)
-		return nil, err
+	for _, w := range b.Warnings {
+		ss.AddWarning(e.Name + ": " + w)
 	}
-	err = replicate.Swap(dir, b)
-	if errors.Is(err, replicate.ErrRebuildStale) {
-		ss.AddWarning(fmt.Sprintf("%s changed while it was cloned again, so it was not replaced; run hedgehug doctor --fix again", e.Name))
-		return nil, nil
+	if store.RebuildPhaseIn(dir) != store.RebuildNone {
+		ss.AddWarning(store.LeftoverMessage(e.Name))
 	}
-	if err != nil {
+	if err := s.Repo().Close(); err != nil {
 		return nil, err
 	}
-	for _, w := range b.Warnings {
-		ss.AddWarning(e.Name + ": " + w)
-	}
-	// The index is a cache: the swapped-in store works without it.
+	// The index is a cache: the store works without it.
 	if ns, err := ss.OpenStoreAt(dir); err == nil {
 		if _, err := ss.OpenHistory(ns, e.Name); err != nil {
 			ss.AddWarning(fmt.Sprintf("%s: could not index its history: %v", e.Name, err))
 		}
 		_ = ns.Repo().Close()
 	}
-	if left, lerr := store.FindLeftover(dir); lerr == nil && !left.None() {
-		ss.AddWarning(store.LeftoverMessage(e.Name))
-	}
 	return &Rebuilt{Replayed: b.Replayed, Dropped: append([]string{}, b.Purged...), Kept: b.Kept}, nil
 }
diff --git a/internal/app/usecase/session.go b/internal/app/usecase/session.go
index a3b53ee..68f531f 100644
--- a/internal/app/usecase/session.go
+++ b/internal/app/usecase/session.go
@@ -4,6 +4,7 @@ import (
 	"errors"
 	"fmt"
 	"net/http"
+	"path/filepath"
 	"strings"
 	gosync "sync"
 	"time"
@@ -47,11 +48,11 @@ type Session struct {
 	// index and before it writes, or write config.yaml after an operation
 	// read it and before it updates it, as another process could.
 	BeforeUpdate func()
-	// BeforeSwap is told the path of a store before doctor --fix swaps the
+	// BeforeRebuild is told the path of a store before doctor --fix rebuilds the
 	// directory of a rebuild in for it (after a force push): whoever holds
 	// that store open, such as the GUI server's index, lets go of it.
 	// ForStore copies it. nil does nothing.
-	BeforeSwap func(path string)
+	BeforeRebuild func(path string)
 	// Spawn starts the background sync as a process; nil starts it detached.
 	// ForStore copies it.
 	Spawn      replicate.Spawn
@@ -113,7 +114,7 @@ func (ss *Session) ForStore(name string) *Session {
 		AfterWrite:     ss.AfterWrite,
 		BackgroundSync: ss.BackgroundSync,
 		BeforeUpdate:   ss.BeforeUpdate,
-		BeforeSwap:     ss.BeforeSwap,
+		BeforeRebuild:  ss.BeforeRebuild,
 		Spawn:          ss.Spawn,
 		Keyring:        ss.Keyring,
 		HTTPClient:     ss.HTTPClient,
@@ -374,23 +375,35 @@ func (ss *Session) OpenStore() (*store.Store, error) {
 		return nil, err
 	}
 	s, err := ss.OpenStoreAt(e.Path)
-	left, lerr := store.FindLeftover(e.Path)
-	unfinished := lerr == nil && !left.None()
 	if err != nil {
-		if unfinished {
-			err = &store.LeftoverError{Name: e.Name}
-		}
 		return nil, OpenError(e.Name, err)
 	}
-	if unfinished {
+	if store.RebuildPhaseIn(e.Path) != store.RebuildNone {
 		ss.WarnOnce(e.Name, store.LeftoverMessage(e.Name))
 	}
 	return s, nil
 }
 
+// nameAt is the name config.yaml gives the store at dir, or else the name of
+// the directory.
+func (ss *Session) nameAt(dir string) string {
+	if c, _, err := ss.Config(); err == nil {
+		for _, e := range c.Stores {
+			if osfs.SamePath(e.Path, dir) {
+				return e.Name
+			}
+		}
+	}
+	return filepath.Base(dir)
+}
+
 // OpenStoreAt opens the store in dir with the clock of the session, so that
-// what it writes has the time ss.Now tells.
+// what it writes has the time ss.Now tells. A store whose rebuild stopped
+// part of the way does not open (store.LeftoverError).
 func (ss *Session) OpenStoreAt(dir string) (*store.Store, error) {
+	if store.RebuildPhaseIn(dir) == store.RebuildApply {
+		return nil, &store.LeftoverError{Name: ss.nameAt(dir)}
+	}
 	s, err := store.Open(dir)
 	if err != nil {
 		return nil, err
@@ -722,7 +735,7 @@ func (ss *Session) ResolveULID(ix *index.Index) func(string) (model.ULID, error)
 // by it; one made later has not. reported says that the one who ran it is
 // told of a failure, so the next command does not tell of it again. A
 // history of origin that differs from the store's stops it (ErrForcePush);
-// hedgehug doctor --fix clones the store again.
+// hedgehug doctor --fix rebuilds the store.
 func (ss *Session) RunSync(s *store.Store, opt replicate.Options, reported bool) (res replicate.Result, err error) {
 	// The sync writes objects through the handle s; the stores mounted are
 	// opened again after it, so that they see them.
diff --git a/internal/app/usecase/sync_shared.go b/internal/app/usecase/sync_shared.go
index c7834d6..e31c0d0 100644
--- a/internal/app/usecase/sync_shared.go
+++ b/internal/app/usecase/sync_shared.go
@@ -5,7 +5,6 @@ import (
 	"encoding/json"
 	"errors"
 	"fmt"
-	"io/fs"
 	"strings"
 	"time"
 
@@ -15,7 +14,6 @@ import (
 	"github.com/kukv/hedgehug/internal/app/storage/inbox"
 	"github.com/kukv/hedgehug/internal/app/storage/index"
 	"github.com/kukv/hedgehug/internal/app/storage/store"
-	"github.com/kukv/hedgehug/internal/lib/osfs"
 )
 
 // SyncReport is what a sync of one store did, as --json prints it.
@@ -57,31 +55,45 @@ func TimePtr(t time.Time) *time.Time {
 	return &t
 }
 
-// FinishRebuild completes a swap that stopped, or removes what a rebuild
-// left, for the store e, before it is opened. Nothing left is nothing done.
-// While the store is there, the leftover is touched only under sync.lock: a
-// rebuild holds it, so a rebuild in the making is not taken for a leftover.
+// ReleaseForRebuild lets go of what this session and the server hold of the
+// store at dir, before a rebuild changes its files.
+func (ss *Session) ReleaseForRebuild(dir string) {
+	if ss.BeforeRebuild != nil {
+		ss.BeforeRebuild(dir)
+	}
+	ss.CloseMounted(dir)
+	ss.DropMount()
+}
+
+// FinishRebuild finishes a rebuild of the store e that stopped, before it is
+// opened. Nothing left is nothing done. It works under sync.lock, which a
+// rebuild holds, so a rebuild in progress is not taken for one that stopped.
 func (ss *Session) FinishRebuild(e config.StoreEntry) error {
-	l, err := store.FindLeftover(e.Path)
-	if err != nil || l.None() {
-		return err
+	if store.RebuildPhaseIn(e.Path) == store.RebuildNone {
+		return nil
 	}
+	ss.ReleaseForRebuild(e.Path)
 	s, err := store.Open(e.Path)
 	if err != nil {
-		// No store (or a broken one) to lock: the swap is finished as it is.
-		if _, serr := osfs.Lstat(e.Path); errors.Is(serr, fs.ErrNotExist) {
-			return ss.finishLeftover(e, l)
-		}
-		return nil // the store's own problem; the sync tells of it
+		return err
 	}
 	defer func() { _ = s.Repo().Close() }()
-	return s.WithLocalLock("sync.lock", SyncLockWait, func() error {
-		l, err := store.FindLeftover(e.Path)
-		if err != nil || l.None() {
-			return err
-		}
-		return ss.finishLeftover(e, l)
+	var b *replicate.Rebuild
+	err = s.WithLocalLock("sync.lock", SyncLockWait, func() error {
+		var err error
+		b, err = replicate.FinishRebuild(s)
+		return err
 	})
+	if err != nil || b == nil {
+		return err
+	}
+	for _, w := range b.Warnings {
+		ss.AddWarning(e.Name + ": " + w)
+	}
+	if b.Kept != "" {
+		ss.AddWarning(fmt.Sprintf("the old store of %s, with changes that were not put back, is kept at %s", e.Name, b.Kept))
+	}
+	return nil
 }
 
 // SyncTargets are the stores an operation works on: the one StoreName
@@ -172,15 +184,6 @@ func DetachedName(s *store.Store, kind, value string) string {
 	return string(b)
 }
 
-// finishLeftover is replicate.FinishLeftoverKept, telling where an old store was kept.
-func (ss *Session) finishLeftover(e config.StoreEntry, l store.Leftover) error {
-	kept, err := replicate.FinishLeftoverKept(e.Path, l)
-	for _, p := range kept {
-		ss.AddWarning(fmt.Sprintf("the old store of %s, with changes that were not put back, is kept at %s", e.Name, p))
-	}
-	return err
-}
-
 // RepairDetail is the Detail of repaired, revived and prefix_duplicate.
 type RepairDetail struct {
 	Kind      string          `json:"kind"`
diff --git a/internal/app/usecase/usecasetest/edit.go b/internal/app/usecase/usecasetest/edit.go
index 3d88d92..f6efbc3 100644
--- a/internal/app/usecase/usecasetest/edit.go
+++ b/internal/app/usecase/usecasetest/edit.go
@@ -316,22 +316,14 @@ func (ts *Session) LocksFree(storeName string, names ...string) bool {
 	return true
 }
 
-// NoLeftover is nil when a rebuild left nothing beside the store at dir.
+// NoLeftover is nil when no rebuild of the store at dir is unfinished.
 func (ts *Session) NoLeftover(dir string) error {
-	left, err := store.FindLeftover(dir)
-	if err != nil {
-		return err
-	}
-	if !left.None() {
-		return &leftoverError{left}
+	if p := store.RebuildPhaseIn(dir); p != store.RebuildNone {
+		return fmt.Errorf("rebuild phase = %q", p)
 	}
 	return nil
 }
 
-type leftoverError struct{ left store.Leftover }
-
-func (e *leftoverError) Error() string { return fmt.Sprintf("leftover = %+v", e.left) }
-
 // HistoryAtHead fails the test unless the saved history index of the store
 // storeName is there and already at the newest commit: opening it again
 // changes nothing on disk.
diff --git a/internal/app/usecase/usecasetest/session.go b/internal/app/usecase/usecasetest/session.go
index 06fb1d5..248b811 100644
--- a/internal/app/usecase/usecasetest/session.go
+++ b/internal/app/usecase/usecasetest/session.go
@@ -102,7 +102,7 @@ func (ts *Session) Renew() {
 		AfterWrite:     o.AfterWrite,
 		BackgroundSync: o.BackgroundSync,
 		BeforeUpdate:   o.BeforeUpdate,
-		BeforeSwap:     o.BeforeSwap,
+		BeforeRebuild:  o.BeforeRebuild,
 		Spawn:          o.Spawn,
 		Keyring:        o.Keyring,
 		HTTPClient:     o.HTTPClient,
```

- [ ] **Step 5: 既存のテストを直す**

```bash
git rm internal/app/remote/replicate/rebuild_gap_test.go internal/app/remote/replicate/rebuild_stale_test.go internal/app/remote/replicate/rebuild_keep_test.go internal/app/remote/replicate/rebuild_local_test.go
```

```diff
diff --git a/internal/app/remote/replicate/rebuild_test.go b/internal/app/remote/replicate/rebuild_test.go
index 12a4d63..8b6a54a 100644
--- a/internal/app/remote/replicate/rebuild_test.go
+++ b/internal/app/remote/replicate/rebuild_test.go
@@ -17,17 +17,13 @@ import (
 	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
 )
 
-// swap puts the rebuild b in the place of clone 1, as usecase does after it
-// released the locks, and opens the store again.
-func (c *cluster) swap(b *Rebuild) {
+// reopen opens clone 1 again, as usecase does after a rebuild.
+func (c *cluster) reopen() {
 	c.t.Helper()
 	dir := c.stores[1].Dir()
 	if err := c.stores[1].Repo().Close(); err != nil {
 		c.t.Fatal(err)
 	}
-	if err := Swap(dir, b); err != nil {
-		c.t.Fatalf("Swap: %v", err)
-	}
 	s, err := store.Open(dir)
 	if err != nil {
 		c.t.Fatal(err)
@@ -35,12 +31,12 @@ func (c *cluster) swap(b *Rebuild) {
 	c.stores[1] = s
 }
 
-// rebuild makes clone 1 again from origin, as doctor --fix does.
+// rebuild brings clone 1 onto the new history, as doctor --fix does.
 func (c *cluster) rebuild() *Rebuild {
 	c.t.Helper()
-	b, err := MakeRebuild(c.stores[1], c.opts[1])
+	b, err := RebuildStore(c.stores[1], c.opts[1])
 	if err != nil {
-		c.t.Fatalf("MakeRebuild: %v", err)
+		c.t.Fatalf("RebuildStore: %v", err)
 	}
 	if b == nil {
 		c.t.Fatal("origin follows the clone: there is nothing to rebuild")
@@ -100,18 +96,18 @@ func TestRebuildReplaysUnpushedChanges(t *testing.T) {
 	if sc.rb.Replayed != 4 {
 		t.Errorf("Replayed = %d, want 4", sc.rb.Replayed)
 	}
-	if !slices.Equal(sc.rb.Purged, []string{c.task(1, sc.gone.ULID).ID}) || sc.rb.PurgedTasks[0] != sc.gone.ULID {
+	if !slices.Equal(sc.rb.Purged, []string{sc.gone.ID}) || sc.rb.PurgedTasks[0] != sc.gone.ULID {
 		t.Errorf("Purged = %v %v", sc.rb.Purged, sc.rb.PurgedTasks)
 	}
 	if trackOf(t, c) == mustHead(t, c.stores[1]) {
 		t.Fatal("setup: the clone had no unpushed work")
 	}
-	c.swap(sc.rb)
-	if left, err := store.FindLeftover(c.stores[1].Dir()); err != nil || !left.None() {
-		t.Errorf("leftover = %+v, %v", left, err)
+	c.reopen()
+	if p := store.RebuildPhaseIn(c.stores[1].Dir()); p != store.RebuildNone {
+		t.Errorf("phase = %q", p)
 	}
 	if sc.rb.Kept == "" || !exists(t, sc.rb.Kept) {
-		t.Errorf("Kept = %q, want the old store beside the new one", sc.rb.Kept)
+		t.Errorf("Kept = %q, want a copy of the old store", sc.rb.Kept)
 	}
 	if got := c.task(1, sc.other.ULID); got == nil || got.Title != "other by b" {
 		t.Errorf("other = %+v", got)
@@ -171,7 +167,7 @@ func TestRebuildWithoutUnpushedChanges(t *testing.T) {
 	if sc.rb.Replayed != 0 || len(sc.rb.Purged) != 0 {
 		t.Errorf("rebuild = %+v", sc.rb)
 	}
-	c.swap(sc.rb)
+	c.reopen()
 	if got, want := mustHead(t, c.stores[1]), trackOf(t, c); got != want {
 		t.Errorf("HEAD = %s, want the head of origin %s", got, want)
 	}
@@ -194,7 +190,7 @@ func TestRebuildRemovesTheOldStoreWhenNothingWasDropped(t *testing.T) {
 	if sc.rb.Replayed != 1 || len(sc.rb.Purged) != 0 {
 		t.Fatalf("rebuild = %+v", sc.rb)
 	}
-	c.swap(sc.rb)
+	c.reopen()
 	if sc.rb.Kept != "" {
 		t.Errorf("Kept = %q, want none", sc.rb.Kept)
 	}
@@ -219,7 +215,7 @@ func TestRebuildKeepsATaskOnlyTheCloneHas(t *testing.T) {
 	if len(sc.rb.Purged) != 0 {
 		t.Errorf("Purged = %v", sc.rb.Purged)
 	}
-	c.swap(sc.rb)
+	c.reopen()
 	if c.task(1, mine.ULID) == nil {
 		t.Error("the clone's own task is lost")
 	}
@@ -248,7 +244,8 @@ func TestRebuildMergesConflicts(t *testing.T) {
 	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
 		t.Fatalf("sync = %v, want ErrForcePush", err)
 	}
-	c.swap(c.rebuild())
+	c.rebuild()
+	c.reopen()
 	got := c.task(1, other.ULID)
 	if got == nil || got.Title != "a" {
 		t.Fatalf("title = %+v, want the later edit, a", got)
@@ -270,9 +267,9 @@ func TestRebuildMergesConflicts(t *testing.T) {
 	}
 }
 
-// origin that follows the clone again needs no rebuild: MakeRebuild removes
-// force-push.json and makes nothing.
-func TestMakeRebuildWhenOriginFollowsAgain(t *testing.T) {
+// origin that follows the clone again needs no rebuild: RebuildStore removes
+// force-push.json and changes nothing.
+func TestRebuildStoreWhenOriginFollowsAgain(t *testing.T) {
 	t.Parallel()
 	c := newCluster(t, 2)
 	gone := c.addTask(0, "gone")
@@ -283,47 +280,15 @@ func TestMakeRebuildWhenOriginFollowsAgain(t *testing.T) {
 		t.Fatalf("sync = %v, want ErrForcePush", err)
 	}
 	gitrepotest.SetRemoteMain(t, c.remote, old.String())
-	b, err := MakeRebuild(c.stores[1], c.opts[1])
+	b, err := RebuildStore(c.stores[1], c.opts[1])
 	if err != nil || b != nil {
-		t.Fatalf("MakeRebuild = %+v, %v; want nothing", b, err)
+		t.Fatalf("RebuildStore = %+v, %v; want nothing", b, err)
 	}
 	if _, ok, err := ReadForcePush(c.stores[1]); err != nil || ok {
 		t.Errorf("force-push.json = %v, %v", ok, err)
 	}
-	if exists(t, c.stores[1].Dir()+newSuffix) {
-		t.Error("a rebuild is left beside the store")
-	}
-}
-
-// A write that lands between the rebuild and the swap stops the swap.
-func TestSwapStale(t *testing.T) {
-	t.Parallel()
-	c := newCluster(t, 2)
-	sc := newRewriteScene(t, c, func(*rewriteScene) {})
-	late := c.addTask(1, "late")
-	dir := c.stores[1].Dir()
-	if err := c.stores[1].Repo().Close(); err != nil {
-		t.Fatal(err)
-	}
-	if err := Swap(dir, sc.rb); !errors.Is(err, ErrRebuildStale) {
-		t.Fatalf("Swap = %v, want ErrRebuildStale", err)
-	}
-	if _, err := os.Stat(sc.rb.Dir); !errors.Is(err, os.ErrNotExist) {
-		t.Errorf("the rebuild is still there: %v", err)
-	}
-	s, err := store.Open(dir)
-	if err != nil {
-		t.Fatal(err)
-	}
-	c.stores[1] = s
-	// doctor --fix makes it again, and the late task is put back.
-	b := c.rebuild()
-	if b.Replayed != 1 {
-		t.Fatalf("second rebuild = %+v", b)
-	}
-	c.swap(b)
-	if c.task(1, late.ULID) == nil {
-		t.Error("the late task is lost")
+	if p := store.RebuildPhaseIn(c.stores[1].Dir()); p != store.RebuildNone {
+		t.Errorf("phase = %q", p)
 	}
 }
 
@@ -347,11 +312,11 @@ func TestRebuildRefusesUncommittedChanges(t *testing.T) {
 	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
 		t.Fatalf("Run = %v, want ErrForcePush", err)
 	}
-	if _, err := MakeRebuild(c.stores[1], c.opts[1]); !errors.Is(err, ErrDirty) || !errors.Is(err, ErrDirtyRebuild) {
-		t.Fatalf("MakeRebuild = %v, want ErrDirtyRebuild", err)
+	if _, err := RebuildStore(c.stores[1], c.opts[1]); !errors.Is(err, ErrDirty) || !errors.Is(err, ErrDirtyRebuild) {
+		t.Fatalf("RebuildStore = %v, want ErrDirtyRebuild", err)
 	}
-	if left, err := store.FindLeftover(c.stores[1].Dir()); err != nil || !left.None() {
-		t.Errorf("leftover = %+v, %v", left, err)
+	if p := store.RebuildPhaseIn(c.stores[1].Dir()); p != store.RebuildNone {
+		t.Errorf("phase = %q", p)
 	}
 }
 
@@ -364,156 +329,6 @@ func exists(t *testing.T, p string) bool {
 	return err == nil
 }
 
-// A swap that stopped after the old store was moved away is finished.
-func TestFinishLeftoverNoStore(t *testing.T) {
-	t.Parallel()
-	c := newCluster(t, 2)
-	sc := newRewriteScene(t, c, func(*rewriteScene) {})
-	dir := c.stores[1].Dir()
-	if err := c.stores[1].Repo().Close(); err != nil {
-		t.Fatal(err)
-	}
-	before := dir + ".before-rewrite-20260101T000000.000000000"
-	if err := os.Rename(dir, before); err != nil {
-		t.Fatal(err)
-	}
-	l, err := store.FindLeftover(dir)
-	if err != nil || l.New != sc.rb.Dir || !slices.Equal(l.Before, []string{before}) {
-		t.Fatalf("leftover = %+v, %v", l, err)
-	}
-	if _, err := store.Open(dir); err == nil {
-		t.Fatal("a store without a directory opens")
-	}
-	if _, err := FinishLeftoverKept(dir, l); err != nil {
-		t.Fatal(err)
-	}
-	if exists(t, before) || exists(t, l.New) {
-		t.Error("something is left")
-	}
-	if _, err := store.Open(dir); err != nil {
-		t.Errorf("the store does not open: %v", err)
-	}
-}
-
-// With the new store in place, the old one is removed; a rebuild never
-// swapped is removed; with only the old one left, it is put back.
-func TestFinishLeftoverOtherStates(t *testing.T) {
-	t.Parallel()
-	root := t.TempDir()
-	dir := filepath.Join(root, "s")
-	mk := func(p string) {
-		if err := os.MkdirAll(p, 0o750); err != nil {
-			t.Fatal(err)
-		}
-		if err := os.WriteFile(filepath.Join(p, "f"), []byte(filepath.Base(p)), 0o600); err != nil {
-			t.Fatal(err)
-		}
-	}
-	mk(dir)
-	mk(dir + ".before-rewrite-1")
-	mk(dir + ".rewrite-new")
-	l, err := store.FindLeftover(dir)
-	if err != nil || l.None() {
-		t.Fatalf("leftover = %+v, %v", l, err)
-	}
-	if _, err := FinishLeftoverKept(dir, l); err != nil {
-		t.Fatal(err)
-	}
-	if l, err := store.FindLeftover(dir); err != nil || !l.None() || !exists(t, dir) {
-		t.Errorf("leftover = %+v, %v", l, err)
-	}
-	// only the old one: put back
-	if err := os.Rename(dir, dir+".before-rewrite-2"); err != nil {
-		t.Fatal(err)
-	}
-	l, _ = store.FindLeftover(dir)
-	if _, err := FinishLeftoverKept(dir, l); err != nil {
-		t.Fatal(err)
-	}
-	b, err := os.ReadFile(filepath.Join(dir, "f"))
-	if err != nil || string(b) != "s" {
-		t.Errorf("the store = %q, %v", b, err)
-	}
-	if l, _ := store.FindLeftover(dir); !l.None() {
-		t.Errorf("leftover = %+v", l)
-	}
-}
-
-// The local files of an old store the swap did not carry over are brought
-// over before it goes; one that cannot be read keeps the old store.
-func TestFinishLeftoverCarriesLocalFiles(t *testing.T) {
-	t.Parallel()
-	root := t.TempDir()
-	dir := filepath.Join(root, "s")
-	before := dir + ".before-rewrite-1"
-	local := func(d string) string { return filepath.Join(d, ".hedgehug", "local") }
-	for _, d := range []string{dir, before} {
-		if err := os.MkdirAll(local(d), 0o750); err != nil {
-			t.Fatal(err)
-		}
-	}
-	write := func(d, name, data string) {
-		if err := os.WriteFile(filepath.Join(local(d), name), []byte(data), 0o600); err != nil {
-			t.Fatal(err)
-		}
-	}
-	write(before, "sync.json", "old")
-	write(before, "late.json", "late")
-	write(dir, "sync.json", "new")
-	l, err := store.FindLeftover(dir)
-	if err != nil {
-		t.Fatal(err)
-	}
-	if _, err := FinishLeftoverKept(dir, l); err != nil {
-		t.Fatal(err)
-	}
-	if b, err := os.ReadFile(filepath.Join(local(dir), "late.json")); err != nil || string(b) != "late" {
-		t.Errorf("late.json = %q, %v", b, err)
-	}
-	if b, err := os.ReadFile(filepath.Join(local(dir), "sync.json")); err != nil || string(b) != "new" {
-		t.Errorf("sync.json = %q, %v (the new store's file is kept)", b, err)
-	}
-	if exists(t, before) {
-		t.Error("the old store is left")
-	}
-
-	if os.Geteuid() == 0 {
-		t.Skip("a file that cannot be read needs a permission that is enforced")
-	}
-	if err := os.MkdirAll(local(before), 0o750); err != nil {
-		t.Fatal(err)
-	}
-	write(before, "secret.json", "x")
-	if err := os.Chmod(filepath.Join(local(before), "secret.json"), 0); err != nil {
-		t.Fatal(err)
-	}
-	l, _ = store.FindLeftover(dir)
-	if _, err := FinishLeftoverKept(dir, l); err == nil {
-		t.Error("an unreadable local file did not fail")
-	}
-	if !exists(t, before) {
-		t.Error("the old store was removed although its files were not carried over")
-	}
-}
-
-// The neighbours of other names are not a leftover.
-func TestFindLeftoverIgnoresOtherNames(t *testing.T) {
-	t.Parallel()
-	root := t.TempDir()
-	dir := filepath.Join(root, "s")
-	for _, n := range []string{"s", "s2.rewrite-new", "s2.before-rewrite-1", "t.rewrite-new"} {
-		if err := os.MkdirAll(filepath.Join(root, n), 0o750); err != nil {
-			t.Fatal(err)
-		}
-	}
-	if l, err := store.FindLeftover(dir); err != nil || !l.None() {
-		t.Errorf("leftover = %+v, %v", l, err)
-	}
-	if l, err := store.FindLeftover(filepath.Join(root, "missing", "s")); err != nil || !l.None() {
-		t.Errorf("leftover of a missing parent = %+v, %v", l, err)
-	}
-}
-
 // What was done on the new history after the force push is new to this
 // clone's inbox, though the history it comes in is not the one the inbox
 // followed.
@@ -532,7 +347,8 @@ func TestRebuildInboxSeesWhatCameAfterTheRewrite(t *testing.T) {
 	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
 		t.Fatalf("sync = %v, want ErrForcePush", err)
 	}
-	c.swap(c.rebuild())
+	c.rebuild()
+	c.reopen()
 	c.sync(1) // the inbox catches up on the new history
 	items, err := inbox.Open(c.stores[1]).List(true)
 	if err != nil {
diff --git a/internal/app/server/poll_release_test.go b/internal/app/server/poll_release_test.go
index 6235825..e085205 100644
--- a/internal/app/server/poll_release_test.go
+++ b/internal/app/server/poll_release_test.go
@@ -4,10 +4,10 @@ import (
 	"testing"
 )
 
-// The session the server gives lets the poller go of a store before a sync
-// swaps a rebuilt one in (D26); the poller then opens the store again and
+// The session the server gives lets the poller go of a store before
+// doctor --fix rebuilds it (D26); the poller then opens the store again and
 // tells the clients that its tasks changed.
-func TestSessionBeforeSwapReleasesTheStore(t *testing.T) {
+func TestSessionBeforeRebuildReleasesTheStore(t *testing.T) {
 	t.Parallel()
 	ts, ch := pollServer(t)
 	held := func() bool {
@@ -23,9 +23,9 @@ func TestSessionBeforeSwapReleasesTheStore(t *testing.T) {
 		t.Fatal("the poller does not hold the store")
 	}
 
-	before := ts.srv.session("").BeforeSwap
+	before := ts.srv.session("").BeforeRebuild
 	if before == nil {
-		t.Fatal("the server's session has no BeforeSwap")
+		t.Fatal("the server's session has no BeforeRebuild")
 	}
 	before(ts.uc.StoreDir("me"))
 
diff --git a/internal/app/storage/mount/closerepo_test.go b/internal/app/storage/mount/closerepo_test.go
index 7681c16..e582fe3 100644
--- a/internal/app/storage/mount/closerepo_test.go
+++ b/internal/app/storage/mount/closerepo_test.go
@@ -1,7 +1,6 @@
 package mount
 
 import (
-	"errors"
 	"os"
 	"path/filepath"
 	"testing"
@@ -30,36 +29,6 @@ func newStoresIn(t *testing.T, root string, names ...string) []config.StoreEntry
 	return out
 }
 
-// Stores that share a directory read it once; each still gets its own
-// leftovers, and none gets those of a neighbour.
-func TestOpenStoresInOneDirectoryEachGetsItsOwnLeftover(t *testing.T) {
-	t.Parallel()
-	es := newStoresIn(t, t.TempDir(), "one", "two", "three", "onetwo")
-	// a rebuild beside "one", and an old store beside "two"; "three" and
-	// "onetwo" (whose name starts like "one") have none
-	if err := os.MkdirAll(es[0].Path+".rewrite-new", 0o750); err != nil {
-		t.Fatal(err)
-	}
-	if err := os.MkdirAll(es[1].Path+".before-rewrite-1", 0o750); err != nil {
-		t.Fatal(err)
-	}
-	set := openSet(t, es...)
-	if len(set.Stores) != 4 || len(set.Skipped) != 0 {
-		t.Fatalf("stores %d, skipped %+v", len(set.Stores), set.Skipped)
-	}
-	var left []string
-	for _, u := range set.Unfinished {
-		var le *store.LeftoverError
-		if !errors.As(u.Err, &le) || le.Name != u.Name {
-			t.Errorf("unfinished %+v", u)
-		}
-		left = append(left, u.Name)
-	}
-	if len(left) != 2 || left[0] != "one" || left[1] != "two" {
-		t.Errorf("Unfinished = %v, want one and two", left)
-	}
-}
-
 // After CloseRepo the store is not used any more, and a set opened again
 // works; the closed one reopens its store when it is used (With).
 func TestCloseRepoThenTheSetIsOpenedAgain(t *testing.T) {
diff --git a/internal/app/usecase/maintenance/rebuild_failure_test.go b/internal/app/usecase/maintenance/rebuild_failure_test.go
index 34962a1..8ff9cb2 100644
--- a/internal/app/usecase/maintenance/rebuild_failure_test.go
+++ b/internal/app/usecase/maintenance/rebuild_failure_test.go
@@ -9,14 +9,14 @@ import (
 	"github.com/kukv/hedgehug/internal/app/usecase/maintenance"
 )
 
-// A failed clone is told in the finding of the force push, not in a second
+// A failed rebuild is told in the finding of the force push, not in a second
 // finding of the same check.
 func TestWithRebuildFailureKeepsOneFinding(t *testing.T) {
 	t.Parallel()
 	rerr := errors.New("disk full")
 	have := []usecase.Finding{{Check: "unexpected_force_push", Where: "me", Msg: "the history differs", Hint: "run doctor --fix"}}
 	got := maintenance.WithRebuildFailure(have, "me", rerr)
-	if len(got) != 1 || !strings.HasPrefix(got[0].Msg, "the history differs; ") || !strings.Contains(got[0].Msg, "cannot clone the store again: disk full") || !strings.Contains(got[0].Hint, "doctor --fix again") {
+	if len(got) != 1 || !strings.HasPrefix(got[0].Msg, "the history differs; ") || !strings.Contains(got[0].Msg, "cannot rebuild the store: disk full") || !strings.Contains(got[0].Hint, "doctor --fix again") {
 		t.Errorf("findings = %+v; want the one finding to tell of both", got)
 	}
 	got = maintenance.WithRebuildFailure(nil, "me", rerr)
diff --git a/internal/app/usecase/maintenance/rebuild_test.go b/internal/app/usecase/maintenance/rebuild_test.go
index daafae0..5151e12 100644
--- a/internal/app/usecase/maintenance/rebuild_test.go
+++ b/internal/app/usecase/maintenance/rebuild_test.go
@@ -2,7 +2,6 @@ package maintenance_test
 
 import (
 	"errors"
-	"io/fs"
 	"os"
 	"path"
 	"path/filepath"
@@ -74,19 +73,33 @@ func (f *forcePushFx) path(t *testing.T) string {
 	return e.Path
 }
 
-// stoppedSwap leaves the clone's store as a swap that stopped after the old
-// store was moved away: no store at the path, the old one and a copy of it
-// as the rebuild beside it.
-func (f *forcePushFx) stoppedSwap(t *testing.T) (dir string) {
+// stoppedRebuild leaves the clone's store as a rebuild that stopped before it
+// changed anything: rebuild.json says main goes to the head of origin. The
+// sync brings the objects of the new history.
+func (f *forcePushFx) stoppedRebuild(t *testing.T) (dir string) {
 	t.Helper()
+	if _, err := replication.Sync(f.clone.Session, false, true); !errors.Is(err, replicate.ErrForcePush) {
+		t.Fatalf("Sync = %v, want ErrForcePush", err)
+	}
 	dir = f.path(t)
-	if err := os.CopyFS(dir+".rewrite-new", os.DirFS(dir)); err != nil {
+	h := originHead(t, f.remote).String()
+	f.writeRecord(t, dir, `{"phase":"apply","old_head":"`+gitrepotest.HeadHash(t, dir)+`","target":"`+h+`","origin":"`+h+`","mark":"`+h+`"}`)
+	return dir
+}
+
+func (f *forcePushFx) writeRecord(t *testing.T, dir, rec string) {
+	t.Helper()
+	p := filepath.Join(dir, filepath.FromSlash(store.LocalDir), store.RebuildFile)
+	if err := os.WriteFile(p, []byte(rec+"\n"), 0o600); err != nil {
 		t.Fatal(err)
 	}
-	if err := os.Rename(dir, dir+".before-rewrite-20260101T000000.000000000"); err != nil {
-		t.Fatal(err)
+}
+
+func noRebuild(t *testing.T, dir string) {
+	t.Helper()
+	if p := store.RebuildPhaseIn(dir); p != store.RebuildNone {
+		t.Errorf("phase = %q", p)
 	}
-	return dir
 }
 
 // forcePushFindings are the findings of r that tell of anything but the merge
@@ -152,18 +165,19 @@ func TestSyncStopsOnAForcePush(t *testing.T) {
 	}
 }
 
-// doctor --fix clones the store again: BeforeSwap is told first, the unpushed
-// work is put back except what was done to the task that is gone, the old
-// store is kept where the report says, and the next sync pushes the work.
-func TestDoctorFixClonesTheStoreAgain(t *testing.T) {
+// doctor --fix rebuilds the store: BeforeRebuild is told first, the unpushed
+// work is put back except what was done to the task that is gone, a copy of
+// the old store is kept where the report says, and the next sync pushes the
+// work.
+func TestDoctorFixRebuildsTheStore(t *testing.T) {
 	t.Parallel()
 	f := newForcePushFx(t, true)
 	dir := f.path(t)
 	var told []string
-	f.clone.BeforeSwap = func(p string) {
+	f.clone.BeforeRebuild = func(p string) {
 		told = append(told, p)
 		if _, err := os.Stat(p); err != nil {
-			t.Errorf("the store is gone before BeforeSwap: %v", err)
+			t.Errorf("the store is gone before BeforeRebuild: %v", err)
 		}
 	}
 	if _, err := replication.Sync(f.clone.Session, false, true); !errors.Is(err, replicate.ErrForcePush) {
@@ -181,14 +195,12 @@ func TestDoctorFixClonesTheStoreAgain(t *testing.T) {
 		t.Errorf("the old store is not at Kept: %v", err)
 	}
 	if !slices.Equal(told, []string{dir}) {
-		t.Errorf("BeforeSwap was told %v, want %s", told, dir)
+		t.Errorf("BeforeRebuild was told %v, want %s", told, dir)
 	}
 	if fs := forcePushFindings(rs[0]); len(fs) != 0 {
 		t.Errorf("findings after --fix: %+v", fs)
 	}
-	if left, err := store.FindLeftover(dir); err != nil || !left.None() {
-		t.Errorf("leftover = %+v, %v", left, err)
-	}
+	noRebuild(t, dir)
 	s, err := f.clone.OpenStore()
 	if err != nil {
 		t.Fatal(err)
@@ -251,88 +263,47 @@ func TestDoctorFixDoesNotRebuildAStoreThatWasNotForcePushed(t *testing.T) {
 	}
 }
 
-// A write that lands between the rebuild and the swap stops the swap with a
-// warning and no rebuild; the next doctor --fix makes it again.
-func TestDoctorFixRebuildStaleIsAWarning(t *testing.T) {
+// A store whose rebuild stopped does not open, says why, and is finished by
+// the next sync.
+func TestSyncFinishesAStoppedRebuild(t *testing.T) {
 	t.Parallel()
 	f := newForcePushFx(t, true)
-	if _, err := replication.Sync(f.clone.Session, false, true); !errors.Is(err, replicate.ErrForcePush) {
-		t.Fatalf("Sync = %v, want ErrForcePush", err)
-	}
-	var late *model.Task
-	f.clone.BeforeSwap = func(string) {
-		if late == nil {
-			late = f.clone.AddTask("late", nil)
-		}
-	}
-	f.clone.Warns = nil
-	rs, err := maintenance.Doctor(f.clone.Session, true)
-	if err != nil || len(rs) != 1 || rs[0].Rebuilt != nil {
-		t.Fatalf("Doctor = %+v, %v; want no rebuild", rs, err)
-	}
-	if !slices.ContainsFunc(f.clone.Warns, func(w string) bool { return strings.Contains(w, "run hedgehug doctor --fix again") }) {
-		t.Errorf("warnings = %q", f.clone.Warns)
-	}
-	if fs := forcePushFindings(rs[0]); len(fs) != 1 || fs[0].Check != "unexpected_force_push" {
-		t.Errorf("findings = %+v, want the force push still", fs)
-	}
-	if left, _ := store.FindLeftover(f.path(t)); !left.None() {
-		t.Errorf("leftover = %+v", left)
-	}
-	f.clone.BeforeSwap = nil
-	rs, err = maintenance.Doctor(f.clone.Session, true)
-	if err != nil || len(rs) != 1 || rs[0].Rebuilt == nil || rs[0].Rebuilt.Replayed != 3 {
-		t.Fatalf("second Doctor = %+v, %v; want a rebuild of 3 commits", rs, err)
-	}
-	s, err := f.clone.OpenStore()
-	if err != nil {
-		t.Fatal(err)
-	}
-	for _, u := range []model.ULID{late.ULID, f.mine.ULID} {
-		if _, err := s.Task(u); err != nil {
-			t.Errorf("%s: %v", u, err)
-		}
-	}
-}
-
-// A store whose swap stopped does not open, says why, and is finished by the
-// next sync, which then finds the force push as usual.
-func TestSyncFinishesAStoppedSwap(t *testing.T) {
-	t.Parallel()
-	f := newForcePushFx(t, true)
-	dir := f.stoppedSwap(t)
-	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
-		t.Fatalf("the store is there: %v", err)
-	}
+	dir := f.stoppedRebuild(t)
 	f.clone.Warns = nil
 	_, err := f.clone.OpenStore()
 	var le *store.LeftoverError
 	if err == nil || !strings.Contains(err.Error(), store.LeftoverMessage("me")) || !errors.As(err, &le) {
 		t.Errorf("OpenStore = %v", err)
 	}
-	if _, err := replication.Sync(f.clone.Session, false, true); !errors.Is(err, replicate.ErrForcePush) {
-		t.Fatalf("Sync = %v, want ErrForcePush after the swap is finished", err)
+	set, err := f.clone.Mount()
+	if err != nil {
+		t.Fatal(err)
 	}
-	if left, _ := store.FindLeftover(dir); !left.None() {
-		t.Errorf("leftover = %+v", left)
+	if _, ok := set.Get("me"); ok {
+		t.Error("the store is mounted")
+	}
+	if _, err := replication.Sync(f.clone.Session, false, true); err != nil {
+		t.Fatalf("Sync = %v", err)
 	}
+	noRebuild(t, dir)
 	s, err := f.clone.OpenStore()
 	if err != nil {
 		t.Fatal(err)
 	}
-	if _, err := s.Task(f.mine.ULID); err != nil {
+	if _, err := s.Task(f.gone.ULID); !errors.Is(err, store.ErrNotFound) {
+		t.Errorf("the task that is gone = %v, want not found", err)
+	}
+	if _, err := s.Task(f.keep.ULID); err != nil {
 		t.Error(err)
 	}
 }
 
-// A store with a leftover beside it opens with a warning, once.
-func TestOpenStoreWarnsOfALeftover(t *testing.T) {
+// A store with only the old objects left to remove opens with a warning, once.
+func TestOpenStoreWarnsOfAnUnfinishedPrune(t *testing.T) {
 	t.Parallel()
 	f := newForcePushFx(t, true)
 	dir := f.path(t)
-	if err := os.MkdirAll(dir+".before-rewrite-1", 0o750); err != nil {
-		t.Fatal(err)
-	}
+	f.writeRecord(t, dir, `{"phase":"prune"}`)
 	f.clone.Warns = nil
 	for range 2 {
 		if _, err := f.clone.OpenStore(); err != nil {
@@ -356,42 +327,69 @@ func TestOpenStoreWarnsOfALeftover(t *testing.T) {
 	}
 }
 
-// doctor --fix finishes a swap that stopped, also when the store itself is
-// missing; doctor without --fix tells that the store does not open.
-func TestDoctorFixFinishesAStoppedSwap(t *testing.T) {
+// doctor --fix finishes a rebuild that stopped; doctor without --fix says the
+// store does not open, and looks at nothing in it.
+func TestDoctorFixFinishesAStoppedRebuild(t *testing.T) {
 	t.Parallel()
 	f := newForcePushFx(t, true)
-	dir := f.stoppedSwap(t)
-	rs, err := maintenance.Doctor(f.clone.Session, false)
-	if err != nil || len(rs) != 1 || !usecase.FindingsFail(rs[0].Findings) {
-		t.Fatalf("Doctor of a store that is not there = %+v, %v; want findings", rs, err)
+	dir := f.stoppedRebuild(t)
+	var le *store.LeftoverError
+	if _, err := maintenance.Doctor(f.clone.Session, false); !errors.As(err, &le) {
+		t.Fatalf("Doctor of a store that stopped = %v, want a LeftoverError", err)
 	}
 	if _, err := maintenance.Doctor(f.clone.Session, true); err != nil {
 		t.Fatal(err)
 	}
-	if _, err := store.Open(dir); err != nil {
+	noRebuild(t, dir)
+	if _, err := f.clone.OpenStore(); err != nil {
 		t.Errorf("the store does not open: %v", err)
 	}
-	if left, _ := store.FindLeftover(dir); !left.None() {
-		t.Errorf("leftover = %+v", left)
-	}
 }
 
-// With the store there, a leftover is removed: the old store of a finished
-// swap, and a rebuild never swapped in.
-func TestDoctorFixRemovesTheOldStore(t *testing.T) {
+// A file edited by hand since the rebuild stopped is not overwritten: doctor
+// --fix tells of it, and the store stays closed until it is put back.
+func TestDoctorFixStopsAtAFileEditedByHand(t *testing.T) {
 	t.Parallel()
 	f := newForcePushFx(t, true)
-	dir := f.path(t)
-	for _, suffix := range []string{".before-rewrite-1", ".rewrite-new"} {
-		if err := os.MkdirAll(filepath.Join(dir+suffix, "x"), 0o750); err != nil {
-			t.Fatal(err)
-		}
+	dir := f.stoppedRebuild(t)
+	p := filepath.Join(dir, filepath.FromSlash(model.TaskPath(f.gone.ULID)))
+	was, err := os.ReadFile(p)
+	if err != nil {
+		t.Fatal(err)
+	}
+	if err := os.WriteFile(p, []byte("by hand\n"), 0o600); err != nil {
+		t.Fatal(err)
+	}
+	rs, err := maintenance.Doctor(f.clone.Session, true)
+	if err != nil || len(rs) != 1 || !usecase.FindingsFail(rs[0].Findings) {
+		t.Fatalf("Doctor = %+v, %v; want a finding", rs, err)
+	}
+	if got, _ := os.ReadFile(p); string(got) != "by hand\n" {
+		t.Errorf("the file = %q", got)
+	}
+	if store.RebuildPhaseIn(dir) != store.RebuildApply {
+		t.Errorf("phase = %q", store.RebuildPhaseIn(dir))
+	}
+	if err := os.WriteFile(p, was, 0o600); err != nil {
+		t.Fatal(err)
 	}
 	if _, err := maintenance.Doctor(f.clone.Session, true); err != nil {
 		t.Fatal(err)
 	}
-	if left, _ := store.FindLeftover(dir); !left.None() {
-		t.Errorf("leftover = %+v", left)
+	noRebuild(t, dir)
+}
+
+// A rebuild.json that cannot be read keeps the store closed and is told of.
+func TestDoctorFixTellsOfABrokenRecord(t *testing.T) {
+	t.Parallel()
+	f := newForcePushFx(t, true)
+	dir := f.path(t)
+	f.writeRecord(t, dir, "not json")
+	if _, err := f.clone.OpenStore(); err == nil {
+		t.Error("the store opened")
+	}
+	rs, err := maintenance.Doctor(f.clone.Session, true)
+	if err != nil || len(rs) != 1 || !usecase.FindingsFail(rs[0].Findings) {
+		t.Fatalf("Doctor = %+v, %v; want a finding", rs, err)
 	}
 }
```

- [ ] **Step 6: 通ることを確かめる**

Run: `go vet ./internal/... && go test ./internal/app/storage/... ./internal/app/remote/replicate/ ./internal/app/usecase/... ./internal/app/server/ ./internal/app/cli/`
Expected: PASS

- [ ] **Step 7: 壊す確認をする**

次を 1 つずつ行い、挙げたテストが落ちることを確かめて戻す。

| 壊し方 | 落ちるテスト |
| --- | --- |
| `finish` の `r.PruneUnreachable()` を消す | `TestRebuildRemovesTheOldHistory` |
| `checkMoveMain` のパスの確認を消す | `TestDoctorFixStopsAtAFileEditedByHand`・`TestMoveMainLeavesAFileEditedByHand` |
| `OpenStoreAt` の `RebuildApply` の判定を消す | `TestSyncFinishesAStoppedRebuild` |

- [ ] **Step 8: コミットする**

```bash
git add internal/app
git commit -m "feat: force push の復旧を同じディレクトリ内で行う

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 4: 文言と文書を実態に合わせ、全体を確かめる

**Files:**
- Modify: 文言の一覧の各ファイル
- Modify: `docs/superpowers/specs/2026-11-01-phase3-stores-accounts-design.md`
- Modify: `docs/superpowers/specs/2026-11-02-cli-acceptance-fixes-design.md`
- Modify: 親設計（Claude Docs）

**Interfaces:**
- Consumes: Task 3 の全部

- [ ] **Step 1: 文言を直す**

```diff
diff --git a/internal/app/cli/doctor.go b/internal/app/cli/doctor.go
index e60a32b..3fef722 100644
--- a/internal/app/cli/doctor.go
+++ b/internal/app/cli/doctor.go
@@ -100,8 +100,8 @@ func (a *app) writeDoctor(r maintenance.DoctorReport, fix, heading, first bool)
 	return a.writeFindings(r.Findings)
 }
 
-// writeRebuilt tells of a store that was cloned again from origin after a
-// force push, on lines of their own: what was put back, then, when some
+// writeRebuilt tells of a store that was rebuilt on the history of origin
+// after a force push, on lines of their own: what was put back, then, when some
 // changes could not be, where the old store is.
 func (a *app) writeRebuilt(store string, b *maintenance.Rebuilt) error {
 	if _, err := fmt.Fprintf(a.stdout, "Cloned %s again from origin; put back %s\n", store, plural.Count(b.Replayed, "unpushed commit")); err != nil {
diff --git a/internal/app/cli/doctor_forcepush_test.go b/internal/app/cli/doctor_forcepush_test.go
index 9223f2b..dae5ec4 100644
--- a/internal/app/cli/doctor_forcepush_test.go
+++ b/internal/app/cli/doctor_forcepush_test.go
@@ -9,7 +9,7 @@ import (
 )
 
 // The whole round: origin is force pushed, hh sync stops and says why, hh
-// doctor tells of it, hh doctor --fix clones the store again with the
+// doctor tells of it, hh doctor --fix rebuilds the store with the
 // unpushed work put back, and the sync after it pushes that work.
 func TestDoctorFixClonesAForcePushedStoreAgain(t *testing.T) {
 	t.Parallel()
diff --git a/internal/app/cli/history_prebuilt_test.go b/internal/app/cli/history_prebuilt_test.go
index e49a405..e53d92c 100644
--- a/internal/app/cli/history_prebuilt_test.go
+++ b/internal/app/cli/history_prebuilt_test.go
@@ -21,7 +21,7 @@ func TestStoreAddBuildsTheHistoryIndex(t *testing.T) {
 	requireHistoryAtHead(t, bob, "org")
 }
 
-// doctor --fix builds the history index of the store it cloned again.
+// doctor --fix builds the history index of the store it rebuilt.
 func TestDoctorFixBuildsTheHistoryIndex(t *testing.T) {
 	t.Parallel()
 	_, bob, remote := twoUsers(t)
diff --git a/internal/app/remote/replicate/forcepush.go b/internal/app/remote/replicate/forcepush.go
index b8180d8..5b6810c 100644
--- a/internal/app/remote/replicate/forcepush.go
+++ b/internal/app/remote/replicate/forcepush.go
@@ -15,8 +15,8 @@ const forcePushFile = "force-push.json"
 
 // ErrForcePush means the history of origin is not one the store follows: it
 // was rewritten or reset. The store is as it was; hedgehug doctor --fix
-// clones it again.
-var ErrForcePush = errors.New("the history of origin differs from this store's; check origin, then run hedgehug doctor --fix to clone it again")
+// rebuilds it.
+var ErrForcePush = errors.New("the history of origin differs from this store's; check origin, then run hedgehug doctor --fix to rebuild the store on it")
 
 // ForcePush is force-push.json: the force push the last sync found.
 type ForcePush struct {
diff --git a/internal/app/server/poll.go b/internal/app/server/poll.go
index 6963cb1..83fdec0 100644
--- a/internal/app/server/poll.go
+++ b/internal/app/server/poll.go
@@ -76,7 +76,7 @@ func newPoller(s *Server, hub *Hub) *Poller {
 }
 
 // Release lets go of the store at path, whose directory is about to be
-// swapped for a rebuild after a force push (D26); see watch.Watcher.Release.
+// rebuilt after a force push (D26); see watch.Watcher.Release.
 func (p *Poller) Release(path string) { p.w.Release(path) }
 
 // ReleaseFor lets go of the store at path and runs fn before any poll can
diff --git a/internal/app/storage/store/lock.go b/internal/app/storage/store/lock.go
index 3f3c596..37a7830 100644
--- a/internal/app/storage/store/lock.go
+++ b/internal/app/storage/store/lock.go
@@ -27,7 +27,7 @@ func lockFile(path string, wait time.Duration, onMiss func()) (*osfs.Lock, error
 }
 
 // ErrStoreGone means the directory of the store was removed or replaced (by
-// doctor --fix's swap, for one), before the lock was taken or while it was
+// doctor --fix's rebuild, for one), before the lock was taken or while it was
 // waited for.
 var ErrStoreGone = errors.New("the store directory was removed or replaced; run the command again")
 
diff --git a/internal/app/usecase/replication/sync.go b/internal/app/usecase/replication/sync.go
index b5c594c..facf057 100644
--- a/internal/app/usecase/replication/sync.go
+++ b/internal/app/usecase/replication/sync.go
@@ -74,7 +74,7 @@ func SyncStore(ss *usecase.Session, c *config.Config, e config.StoreEntry, expli
 		fetchGitHub(ss, c, e, &r, nil)
 		return r, nil
 	}
-	// A swap that stopped is finished first.
+	// A rebuild that stopped is finished first.
 	if err := ss.FinishRebuild(e); err != nil {
 		return r, err
 	}
diff --git a/internal/app/usecase/watch/export_test.go b/internal/app/usecase/watch/export_test.go
index 9b4c436..b19bd28 100644
--- a/internal/app/usecase/watch/export_test.go
+++ b/internal/app/usecase/watch/export_test.go
@@ -40,7 +40,7 @@ func HeldCount(w *Watcher) int {
 	return len(w.stores)
 }
 
-// Released is whether w marks the store named name as swapped for a rebuild.
+// Released is whether w marks the store named name as rebuilt.
 func Released(w *Watcher, name string) bool {
 	w.mu.Lock()
 	defer w.mu.Unlock()
diff --git a/internal/app/usecase/watch/stores_test.go b/internal/app/usecase/watch/stores_test.go
index f4876c4..0dbcb69 100644
--- a/internal/app/usecase/watch/stores_test.go
+++ b/internal/app/usecase/watch/stores_test.go
@@ -74,7 +74,7 @@ func TestPollBuildsTheHistoryIndex(t *testing.T) {
 }
 
 // Release cancels a build of the history index that is running and returns
-// once it ended, before the repository is closed and the directory swapped.
+// once it ended, before the repository is closed and the store rebuilt.
 func TestReleaseCancelsTheHistoryBuild(t *testing.T) {
 	t.Parallel()
 	f := newFixture(t) // its watcher does not run: Poll is called by hand
@@ -248,6 +248,6 @@ func TestPollerReleaseForKeepsPollsOut(t *testing.T) {
 		t.Error("the poll after ReleaseFor did not open the store again")
 	}
 	if watch.Released(f.w, e.Name) {
-		t.Error("a store taken out for good is still marked as swapped")
+		t.Error("a store taken out for good is still marked as rebuilt")
 	}
 }
diff --git a/internal/app/usecase/watch/watch.go b/internal/app/usecase/watch/watch.go
index 1b755f0..3c4ce66 100644
--- a/internal/app/usecase/watch/watch.go
+++ b/internal/app/usecase/watch/watch.go
@@ -99,7 +99,7 @@ type Watcher struct {
 
 	mu       sync.Mutex // the state below
 	stores   map[string]*polled
-	released map[string]bool // stores swapped for a rebuild, to tell the clients once
+	released map[string]bool // stores rebuilt, to tell the clients once
 	restored map[string]bool // stores let go for a removal that failed; opened again without a word
 	unread   int
 	looked   bool   // a poll has run
@@ -126,7 +126,7 @@ type polled struct {
 
 	// histBusy is true while a goroutine brings the history index of the
 	// store to HEAD; histDone is that goroutine's. Whoever lets go of the
-	// store (its directory may be swapped or removed) waits for histDone.
+	// store (its directory may be rebuilt or removed) waits for histDone.
 	histBusy   atomic.Bool
 	histDone   sync.WaitGroup
 	histCancel chan struct{} // closed to stop the build
@@ -145,7 +145,7 @@ func (st *polled) stopHistory() {
 }
 
 // Release lets go of the store at path, whose directory is about to be
-// swapped for a rebuild after a force push (D26): its index and its
+// rebuilt after a force push (D26): its index and its
 // repository files are closed, and the next poll opens the store again and
 // tells the clients that its tasks changed. It waits for a poll that is
 // reading, and cancels the build of the history index of the store and waits
@@ -191,7 +191,7 @@ func (w *Watcher) releaseLocked(path string) []string {
 // removal of the store: its directory goes) before any poll can look again,
 // so that no poll opens the store again while its config entry is still
 // there. When fn succeeds the store is gone for good, so it is no longer
-// marked as swapped for a rebuild (a store of that name that comes later is
+// marked as rebuilt (a store of that name that comes later is
 // new). When fn fails the store is still there: the next poll opens it again
 // without telling the clients anything (it is the same store, not a rebuilt
 // one). fn must not wait for a poll.
@@ -295,13 +295,13 @@ func (w *Watcher) pollLocked(publish bool) {
 			st = newPolled(e.Path, s)
 			fresh = true
 			w.mu.Lock()
-			swapped := w.released[e.Name]
+			rebuilt := w.released[e.Name]
 			delete(w.released, e.Name)
 			restored := w.restored[e.Name]
 			delete(w.restored, e.Name)
 			scanned := w.scanned
 			w.mu.Unlock()
-			if (swapped || scanned) && !restored && publish {
+			if (rebuilt || scanned) && !restored && publish {
 				// The store was made again, or it appeared after the first look: the
 				// commits made before this look are not known (a page that made a
 				// task in a store just created would never be told), so the clients
diff --git a/web/app/settings/DoctorSection.test.ts b/web/app/settings/DoctorSection.test.ts
index d0a98cd..379c7b1 100644
--- a/web/app/settings/DoctorSection.test.ts
+++ b/web/app/settings/DoctorSection.test.ts
@@ -441,7 +441,7 @@ describe("DoctorSection: the repair", () => {
     expect(within(region()).queryByText("parent: the parent is deleted")).toBeNull();
   });
 
-  it("tells a store cloned again from origin", async () => {
+  it("tells a store rebuilt on the history of origin", async () => {
     const { api } = setup({ reports: { a: report({ findings: [fileFinding] }) } });
     await within(region()).findByText("does not decode");
     api.POST.mockImplementation(() =>
```

- [ ] **Step 2: phase3 spec を直す**

`docs/superpowers/specs/2026-11-01-phase3-stores-accounts-design.md` の「### 作り直しと未 push の変更の取り込み」の直下に次を足す。

```markdown
> 手順 2・4・5 とその補足のディレクトリの入れ替えは、`docs/superpowers/specs/2026-11-05-rebuild-in-place-design.md` で同じディレクトリ内で直す方式に置き換えた。
```

同じ spec の `rewrite_leftover` の行の末尾に `（2026-11-02 に検査を削除）` を足す。

- [ ] **Step 3: 2026-11-02 の spec を直す**

`docs/superpowers/specs/2026-11-02-cli-acceptance-fixes-design.md` の「`doctor --fix` の作り直し:」の行の末尾に次を足す。

```markdown
入れ替えの方式は `2026-11-05-rebuild-in-place-design.md` に置き換えた。
```

- [ ] **Step 4: 親設計を直す**

親設計（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）を docs の `update` で次のとおり直す。

| 元の文 | 直した文 |
| --- | --- |
| `で clone し直します（2026-11-02 決定。手順はドキュメントに書く）` | `で新しい履歴に揃え、手元からも古い履歴を消します（2026-11-02 決定、2026-11-05 変更。手順はドキュメントに書く）` |
| `は、push していない変更を退避し、origin から clone し直し、退避した変更を新しいクローンへ 3-way マージで戻す。戻せなかった変更は退避先のパスを出す` | `は、push していない変更を 3-way マージで新しい履歴に載せ、同じディレクトリの中で作業ツリーと履歴を直す。途中で止まったら次の doctor --fix か同期が続きから完了する。戻せなかった変更があれば、古い Store のコピーを残してパスを出す` |
| パッケージ構成の `store/` の `force push 復旧の途中残りの検出` | `force push 復旧の途中の状態の判定` |

書き換えの履歴の索引の段落の「書き換えや clone し直し」はそのままでよい（履歴の書き換えも `clone` も起こる）。

- [ ] **Step 5: 全体を確かめる**

Run: `go test -race ./... && make lint && make fmt-check && make tidy-check && make web-check && make e2e`
Expected: すべて通る

`grep -rn 'rewrite-new\|before-rewrite' internal` が 0 行であることを確かめる。

- [ ] **Step 6: コミットする**

```bash
git add internal web docs
git commit -m "docs: force push の復旧の文言と spec を同じディレクトリ内の方式に揃える

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
