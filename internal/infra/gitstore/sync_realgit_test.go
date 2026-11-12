package gitstore

import (
	"errors"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
)

// 本物の git サーバー(git http-backend)を相手に同期する。プロセスの中の go-git のサーバーと違い、知らない have を
// 読み飛ばし、送信をサーバーの側で断る。git の実行ファイルがなければ飛ばす(CI の test-without-git)。

// gitEnv は、利用者の git の設定を読まないための環境変数。
var gitEnv = []string{"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1"}

func gitOutput(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec // G204: テストが決めた引数で git を動かす
	cmd.Env = append(os.Environ(), gitEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// logWriter は、サーバーの標準エラー(フックの出力など)を、テストの出力でなくテストのログへ流す。
type logWriter struct{ t *testing.T }

func (w logWriter) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimSpace(string(p)))
	return len(p), nil
}

// newRealRemote は、git http-backend で bare のリポジトリを配る remote と、その bare のフォルダを返す。
// wrap が nil でなければ、サーバーの手前に置く(応答が届かない場面を作るため)。
func newRealRemote(t *testing.T, wrap func(http.Handler) http.Handler) (port.Remote, string) {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not on PATH")
	}
	if _, err := os.Stat(filepath.Join(gitOutput(t, "--exec-path"), "git-http-backend")); err != nil {
		t.Skipf("git-http-backend is not installed: %v", err)
	}
	root := t.TempDir()
	bare := filepath.Join(root, "store.git")
	gitOutput(t, "init", "--bare", bare)
	gitOutput(t, "-C", bare, "symbolic-ref", "HEAD", "refs/heads/main")
	gitOutput(t, "-C", bare, "config", "http.receivepack", "true")
	var h http.Handler = &cgi.Handler{
		Path:   gitPath,
		Args:   []string{"http-backend"},
		Env:    append([]string{"GIT_PROJECT_ROOT=" + root, "GIT_HTTP_EXPORT_ALL=1"}, gitEnv...),
		Stderr: logWriter{t},
	}
	if wrap != nil {
		h = wrap(h)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return port.Remote{URL: srv.URL + "/store.git"}, bare
}

// writeHook は、bare のリポジトリにフックを置く。
func writeHook(t *testing.T, bare, name, script string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(bare, "hooks", name), []byte("#!/bin/sh\n"+script), 0o755); err != nil { //nolint:gosec // G306: フックは実行できなければならない
		t.Fatal(err)
	}
}

func TestRealGitSyncConverges(t *testing.T) {
	remote, _ := newRealRemote(t, nil)
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
	remote, _ := newRealRemote(t, nil)
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
	if res := a.sync(t, remote); res != (port.SyncResult{Sent: 1, Received: 1}) {
		t.Errorf("a sync = %+v, want the overlapping save received", res)
	}
	b.sync(t, remote)
	sameFiles(t, a, b)
}

// サーバーの側で断られ(文は "non-fast-forward" を含まない)、そのあいだに remote の先端が動いていたら、
// 重なりとして受け取りからやり直す。pre-receive のフックが、他の人の送信が先に入った場面を作る:
// 1 回目だけ、b が別のブランチに送っておいた保存へ main を進め、a の送信を断る。
func TestRealGitSyncRetriesWhenServerRefuses(t *testing.T) {
	remote, bare := newRealRemote(t, nil)
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
	writeHook(t, bare, "pre-receive", "git rev-parse -q --verify refs/heads/pending >/dev/null || exit 0\n"+
		"env -u GIT_QUARANTINE_PATH git update-ref refs/heads/main refs/heads/pending || exit 2\n"+
		"env -u GIT_QUARANTINE_PATH git update-ref -d refs/heads/pending || exit 2\n"+
		"echo 'someone else updated main' >&2\n"+
		"exit 1\n")
	a.write(t, "a", set("title", "", "A"))
	if res := a.sync(t, remote); res != (port.SyncResult{Sent: 1, Received: 1}) {
		t.Errorf("a sync = %+v, want the overlapping save received", res)
	}
	b.sync(t, remote)
	sameFiles(t, a, b)
}

// サーバーが断り、remote の先端が動いていなければ、重なりではないのでやり直さず、そのエラーを返す。
func TestRealGitSyncReturnsRefusalThatIsNotAnOverlap(t *testing.T) {
	remote, bare := newRealRemote(t, nil)
	a := firstMachine(t, remote)
	before := gitOutput(t, "-C", bare, "rev-parse", "refs/heads/main")
	count := filepath.Join(t.TempDir(), "count")
	writeHook(t, bare, "pre-receive", "echo x >> '"+count+"'\nexit 1\n")
	a.write(t, "a", set("title", "", "A"))
	_, err := a.rep.Sync(t.Context(), a.id, remote)
	if err == nil || errors.Is(err, port.ErrSendConflict) || errors.Is(err, errRejected) ||
		!strings.Contains(err.Error(), "pre-receive hook declined") {
		t.Errorf("err = %v", err)
	}
	b, rerr := os.ReadFile(count) //nolint:gosec // G304: テストの一時フォルダのファイル
	if rerr != nil || string(b) != "x\n" {
		t.Errorf("hook ran %q times, %v; want once", b, rerr)
	}
	if after := gitOutput(t, "-C", bare, "rev-parse", "refs/heads/main"); after != before {
		t.Errorf("remote main moved: %s -> %s", before, after)
	}
}

// サーバーは更新したが応答が届かなかったら、送れたものとして数える(やり直して 0 件と報告しない)。
func TestRealGitSyncCountsSendWhoseResponseWasLost(t *testing.T) {
	var armed, lost atomic.Bool
	lose := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/git-receive-pack") && armed.Load() && lost.CompareAndSwap(false, true) {
				next.ServeHTTP(httptest.NewRecorder(), r) // サーバーは更新するが、応答を捨てる
				http.Error(w, "connection lost", http.StatusBadGateway)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	remote, bare := newRealRemote(t, lose)
	a := firstMachine(t, remote)
	a.write(t, "a", set("title", "", "A"))
	armed.Store(true)
	if res := a.sync(t, remote); res != (port.SyncResult{Sent: 1}) {
		t.Errorf("a sync = %+v, want Sent 1", res)
	}
	if !lost.Load() {
		t.Fatal("the send did not go through the lossy server")
	}
	repo, err := git.PlainOpen(a.dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	if got := gitOutput(t, "-C", bare, "rev-parse", "refs/heads/main"); plumbing.NewHash(got) != head.Hash() {
		t.Errorf("remote main = %s, want %s", got, head.Hash())
	}
}
