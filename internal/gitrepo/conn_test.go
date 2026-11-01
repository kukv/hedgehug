package gitrepo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
)

func TestRewriteFetchPush(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	bare := filepath.Join(d, "remotes", "org", "x.git")
	if err := InitBare(bare); err != nil {
		t.Fatal(err)
	}
	r := initWithCommit(t, filepath.Join(d, "work"))
	if err := r.SetRemotes([]string{"https://github.com/org/x.git"}); err != nil {
		t.Fatal(err)
	}
	r.SetConn(Conn{Rewrite: func(u string) string {
		if u == "https://github.com/org/x.git" {
			return bare
		}
		return u
	}})
	if err := r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	h, err := r.Fetch("origin")
	if err != nil {
		t.Fatal(err)
	}
	local, _ := r.Head()
	rh, _ := r.RemoteHead("origin")
	br, err := git.PlainOpen(bare)
	if err != nil {
		t.Fatal(err)
	}
	bh, err := br.Head()
	if err != nil {
		t.Fatal(err)
	}
	if h.IsZero() || h != local || h != rh || h != bh.Hash() {
		t.Errorf("fetch = %s, local HEAD = %s, RemoteHead = %s, bare HEAD = %s; want all the same", h, local, rh, bh.Hash())
	}
	// .git/config keeps the URL as written.
	cfg, _ := r.r.Config()
	if got := cfg.Remotes["origin"].URLs[0]; got != "https://github.com/org/x.git" {
		t.Errorf("origin = %q", got)
	}
	c, err := CloneWith("https://github.com/org/x.git", filepath.Join(d, "clone"), Conn{Rewrite: func(string) string { return bare }})
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ = c.r.Config()
	if got := cfg.Remotes["origin"].URLs[0]; got != "https://github.com/org/x.git" {
		t.Errorf("the clone's origin = %q", got)
	}
}

func TestHTTPSAuthAndDenied(t *testing.T) {
	d := t.TempDir()
	if err := InitBare(filepath.Join(d, "org", "x.git")); err != nil {
		t.Fatal(err)
	}
	var auths []transport.AuthMethod
	denied := false
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), &auths, func(string) bool { return denied })
	r := initWithCommit(t, filepath.Join(d, "work"))
	if err := r.SetRemotes([]string{"https://github.com/org/x.git"}); err != nil {
		t.Fatal(err)
	}
	r.SetConn(Conn{Auth: func(string) (transport.AuthMethod, error) {
		return &githttp.BasicAuth{Username: "x-access-token", Password: "tok"}, nil
	}})
	if err := r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	if len(auths) == 0 {
		t.Fatal("no session")
	}
	if b, ok := auths[0].(*githttp.BasicAuth); !ok || b.Username != "x-access-token" || b.Password != "tok" {
		t.Errorf("auth = %#v", auths[0])
	}
	denied = true
	commitFile(t, r, "b.txt", "b")
	if err := r.Push("origin"); !errors.Is(err, ErrPushDenied) {
		t.Errorf("denied push = %v, want ErrPushDenied", err)
	}
}

// Auth is given the URL after the rewrite, and its failure stops the
// fetch, the push, the clone and the listing before any session starts.
func TestAuthGetsRewrittenURLAndItsErrorStops(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	bare := filepath.Join(d, "x.git")
	if err := InitBare(bare); err != nil {
		t.Fatal(err)
	}
	r := initWithCommit(t, filepath.Join(d, "work"))
	const u = "https://github.com/org/x.git"
	if err := r.SetRemotes([]string{u}); err != nil {
		t.Fatal(err)
	}
	var seen []string
	r.SetConn(Conn{
		Rewrite: func(string) string { return bare },
		Auth: func(x string) (transport.AuthMethod, error) {
			seen = append(seen, x)
			return nil, nil
		},
	})
	if err := r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0] != bare {
		t.Errorf("Auth was given %v, want the rewritten %s", seen, bare)
	}

	boom := errors.New("no token")
	failing := Conn{Auth: func(string) (transport.AuthMethod, error) { return nil, boom }}
	r.SetConn(failing)
	if _, err := r.Fetch("origin"); !errors.Is(err, boom) {
		t.Errorf("Fetch = %v, want the auth error", err)
	}
	commitFile(t, r, "b.txt", "b")
	if err := r.Push("origin"); !errors.Is(err, boom) || errors.Is(err, ErrPushDenied) {
		t.Errorf("Push = %v, want the auth error alone", err)
	}
	dir := filepath.Join(d, "clone")
	if _, err := CloneWith(u, dir, failing); !errors.Is(err, boom) {
		t.Errorf("CloneWith = %v, want the auth error", err)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Errorf("the failed clone left %s (%v)", dir, err)
	}
	if _, err := RemoteEmpty(u, failing); !errors.Is(err, boom) {
		t.Errorf("RemoteEmpty = %v, want the auth error", err)
	}
	if _, err := r.Fetch("nope"); err == nil {
		t.Error("a fetch from a remote that is not there succeeded")
	}
}

func TestPushDeniedByReadOnlyRemote(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	bare := filepath.Join(d, "x.git")
	if err := InitBare(bare); err != nil {
		t.Fatal(err)
	}
	r := initWithCommit(t, filepath.Join(d, "work"))
	if err := r.SetRemotes([]string{bare}); err != nil {
		t.Fatal(err)
	}
	gitrepotest.LockObjects(t, bare)
	if err := r.Push("origin"); !errors.Is(err, ErrPushDenied) {
		t.Errorf("push to a read-only remote = %v, want ErrPushDenied", err)
	}
	if h, _ := r.RemoteHead("origin"); !h.IsZero() {
		t.Error("a denied push moved refs/remotes/origin/main")
	}
}

func TestPushDenied(t *testing.T) {
	t.Parallel()
	ssh := errors.New("ssh: handshake failed: ssh: unable to authenticate: Permission denied (publickey)")
	perm := &os.PathError{Op: "open", Path: "x", Err: os.ErrPermission}
	for _, c := range []struct {
		name  string
		err   error
		local bool
		want  bool
	}{
		{"403 over http", transport.ErrAuthorizationFailed, false, true},
		{"403 wrapped", fmt.Errorf("x: %w", transport.ErrAuthorizationFailed), false, true},
		{"ssh login failure", ssh, false, false},
		{"permission text, remote", errors.New("permission denied"), false, false},
		{"permission error, remote", perm, false, false},
		{"permission text, local", errors.New("open objects/x: Permission denied"), true, true},
		{"permission error, local", perm, true, true},
		{"another error, local", errors.New("boom"), true, false},
	} {
		if got := pushDenied(c.err, c.local); got != c.want {
			t.Errorf("%s: pushDenied = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLocalPathAndWritable(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]bool{
		"/tmp/x.git": true, "file:///tmp/x.git": true, "https://h/x": false, "git@h:o/r.git": false,
		`C:\x\y.git`: true, "file:///C:/x/y.git": true, "ssh://git@h/o/r.git": false, "": false, "rel/x.git": false,
	} {
		if _, ok := LocalPath(in); ok != want {
			t.Errorf("LocalPath(%q) ok = %v", in, ok)
		}
	}
	if p, _ := LocalPath("file:///tmp/x.git"); p != filepath.FromSlash("/tmp/x.git") {
		t.Errorf("LocalPath(file URL) = %q", p)
	}
	if !IsHTTP("https://h/x") || !IsHTTP("http://h/x") || IsHTTP("/x") || IsHTTP("ssh://h/x") || IsHTTP("https:///x") || IsHTTP("git@h:o/r.git") {
		t.Error("IsHTTP is wrong")
	}
	d := filepath.Join(t.TempDir(), "x.git")
	if Writable(d) {
		t.Error("a directory that is not there is writable")
	}
	if err := InitBare(d); err != nil {
		t.Fatal(err)
	}
	if !Writable(d) {
		t.Error("a fresh bare is not writable")
	}
	gitrepotest.LockObjects(t, d) // skips the rest where there are no read-only directories
	if Writable(d) {
		t.Error("a read-only bare is writable")
	}
}

func TestRemoteEmpty(t *testing.T) {
	d := t.TempDir()
	bare := filepath.Join(d, "org", "x.git")
	if err := InitBare(bare); err != nil {
		t.Fatal(err)
	}
	var auths []transport.AuthMethod
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), &auths, nil)
	conn := Conn{Auth: func(string) (transport.AuthMethod, error) {
		return &githttp.BasicAuth{Username: "x-access-token", Password: "tok"}, nil
	}}
	const u = "https://github.com/org/x.git"
	if empty, err := RemoteEmpty(u, conn); err != nil || !empty {
		t.Errorf("a bare with no ref = %v, %v", empty, err)
	}
	if len(auths) == 0 {
		t.Fatal("the listing sent no auth")
	}
	if b, ok := auths[0].(*githttp.BasicAuth); !ok || b.Password != "tok" {
		t.Errorf("auth = %#v", auths[0])
	}
	r := initWithCommit(t, filepath.Join(d, "work"))
	if err := r.SetRemotes([]string{bare}); err != nil {
		t.Fatal(err)
	}
	if err := r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	if empty, err := RemoteEmpty(u, conn); err != nil || empty {
		t.Errorf("a bare with a commit = %v, %v", empty, err)
	}
	// The rewrite applies: the URL as written is not served at all.
	rw := Conn{Rewrite: func(string) string { return bare }}
	if empty, err := RemoteEmpty("https://example.invalid/org/x.git", rw); err != nil || empty {
		t.Errorf("through a rewrite = %v, %v", empty, err)
	}
	// A repository that is not there (here: a host nothing serves) is an
	// error, not an empty one.
	if empty, err := RemoteEmpty("https://elsewhere.test/org/x.git", conn); err == nil || empty {
		t.Errorf("a missing repository = %v, %v", empty, err)
	}
}

func TestHeadHasFile(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	r := initWithCommit(t, filepath.Join(d, "work"))
	bare := filepath.Join(d, "x.git")
	if err := InitBare(bare); err != nil {
		t.Fatal(err)
	}
	if err := r.SetRemotes([]string{bare}); err != nil {
		t.Fatal(err)
	}
	if err := r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	if ok, err := HeadHasFile(bare, "a.txt"); err != nil || !ok {
		t.Errorf("a.txt = %v, %v", ok, err)
	}
	if ok, err := HeadHasFile(filepath.Join(d, "work"), "a.txt"); err != nil || !ok {
		t.Errorf("a.txt in the work tree's repository = %v, %v", ok, err)
	}
	if ok, err := HeadHasFile(bare, "nope"); err != nil || ok {
		t.Errorf("nope = %v, %v", ok, err)
	}
	empty := filepath.Join(d, "empty.git")
	if err := InitBare(empty); err != nil {
		t.Fatal(err)
	}
	if ok, err := HeadHasFile(empty, "a.txt"); err != nil || ok {
		t.Errorf("empty = %v, %v", ok, err)
	}
	if _, err := HeadHasFile(filepath.Join(d, "missing.git"), "a.txt"); err == nil {
		t.Error("a directory that is no repository gave no error")
	}
}

func initWithCommit(t *testing.T, dir string) *Repo {
	t.Helper()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	commitFiles(t, r, dir, map[string]string{"a.txt": "a"})
	return r
}

func commitFile(t *testing.T, r *Repo, name, content string) {
	t.Helper()
	commitFiles(t, r, r.dir, map[string]string{name: content})
}
