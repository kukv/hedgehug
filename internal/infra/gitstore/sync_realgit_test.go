package gitstore

import (
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"

	"github.com/kukv/hedgehug/internal/app/port"
)

// 本物の git サーバー(git http-backend)を相手に同期する。プロセスの中の go-git のサーバーと違い、知らない have を
// 読み飛ばし、送信をサーバーの側で断る。git の実行ファイルがなければ飛ばす(CI の test-without-git)。

// gitEnv は、利用者の git の設定を読まないための環境変数。
var gitEnv = []string{"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1"}

func runGit(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec // G204: テストが決めた引数で git を動かす
	cmd.Env = append(os.Environ(), gitEnv...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// newRealRemote は、git http-backend で bare のリポジトリを配る remote と、その bare のフォルダを返す。
func newRealRemote(t *testing.T) (port.Remote, string) {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not on PATH")
	}
	root := t.TempDir()
	bare := filepath.Join(root, "store.git")
	runGit(t, "init", "--bare", "--initial-branch=main", bare)
	runGit(t, "-C", bare, "config", "http.receivepack", "true")
	srv := httptest.NewServer(&cgi.Handler{
		Path: gitPath,
		Args: []string{"http-backend"},
		Env:  append([]string{"GIT_PROJECT_ROOT=" + root, "GIT_HTTP_EXPORT_ALL=1"}, gitEnv...),
	})
	t.Cleanup(srv.Close)
	return port.Remote{URL: srv.URL + "/store.git"}, bare
}

func TestRealGitSyncConverges(t *testing.T) {
	remote, _ := newRealRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	a.write(t, "タイトル", set("title", "ログイン画面を作る", "ログイン"))
	b.write(t, "順位", set("rank", "a", "b"))
	a.sync(t, remote)
	if res := b.sync(t, remote); res != (port.SyncResult{Sent: 1, Received: 1}) {
		t.Errorf("b sync = %+v", res)
	}
	a.sync(t, remote)
	sameFiles(t, a, b)
	if got := title(t, a); got != "ログイン" {
		t.Errorf("title = %q", got)
	}
}

// go-git が送る前に見つける重なり("non-fast-forward")で、受け取りからやり直す。
func TestRealGitSyncRetriesWhenSendOverlaps(t *testing.T) {
	remote, _ := newRealRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	a.write(t, "a", set("title", "", "A"))
	once := false
	a.rep.beforeSend = func() error {
		if !once {
			once = true
			b.write(t, "b", set("rank", "a", "z"))
			b.sync(t, remote)
		}
		return nil
	}
	if res := a.sync(t, remote); res.Received != 1 {
		t.Errorf("a sync = %+v, want the overlapping save received", res)
	}
	b.sync(t, remote)
	sameFiles(t, a, b)
}

// サーバーの側で断られ(文は "non-fast-forward" を含まない)、そのあいだに remote の先端が動いていたら、
// 重なりとして受け取りからやり直す。pre-receive のフックが、他の人の送信が先に入った場面を作る:
// 1 回目だけ、b が別のブランチに送っておいた保存へ main を進め、a の送信を断る。
func TestRealGitSyncRetriesWhenServerRefuses(t *testing.T) {
	remote, bare := newRealRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	b.write(t, "b", set("rank", "a", "z"))
	repo, err := git.PlainOpen(b.dir)
	if err != nil {
		t.Fatal(err)
	}
	rm := git.NewRemote(repo.Storer, &config.RemoteConfig{Name: "raw", URLs: []string{remote.URL}})
	if err := rm.Push(&git.PushOptions{RemoteName: "raw", RefSpecs: []config.RefSpec{"refs/heads/main:refs/heads/pending"}}); err != nil {
		t.Fatal(err)
	}
	hook := "#!/bin/sh\n" +
		"git rev-parse -q --verify refs/heads/pending >/dev/null || exit 0\n" +
		"env -u GIT_QUARANTINE_PATH git update-ref refs/heads/main refs/heads/pending || exit 2\n" +
		"env -u GIT_QUARANTINE_PATH git update-ref -d refs/heads/pending || exit 2\n" +
		"echo 'someone else updated main' >&2\n" +
		"exit 1\n"
	if err := os.WriteFile(filepath.Join(bare, "hooks", "pre-receive"), []byte(hook), 0o755); err != nil { //nolint:gosec // G306: フックは実行できなければならない
		t.Fatal(err)
	}
	a.write(t, "a", set("title", "", "A"))
	if res := a.sync(t, remote); res.Received != 1 {
		t.Errorf("a sync = %+v, want the overlapping save received", res)
	}
	b.sync(t, remote)
	sameFiles(t, a, b)
}
