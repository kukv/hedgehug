package main

import (
	"bytes"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/store"
	hhsync "github.com/kukv/hedgehug/internal/sync"
)

// initDevenv runs devenv init with the fake GitHub of this test as the one the
// users' config.yaml point at, and loads the seed devenv wrote into the fake.
func initDevenv(t *testing.T, devenv string) (dev string, fake *fakegh.Server, apiURL string) {
	t.Helper()
	fake = fakegh.New(fakegh.State{})
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	dev = filepath.Join(t.TempDir(), "hh-dev")
	if out, err := exec.Command(devenv, "init", "--dir", dev, "--fakegh", srv.URL).CombinedOutput(); err != nil { //nolint:gosec // the binary the test built
		t.Fatalf("devenv init: %v\n%s", err, out)
	}
	seed, err := fakegh.Load(filepath.Join(dev, "fakegh.json"))
	if err != nil {
		t.Fatal(err)
	}
	fake.Reset(seed)
	return dev, fake, srv.URL
}

// fail runs hedgehug and returns its exit code and stderr; it fails the test
// when the command succeeds.
func (u *devUser) fail(args ...string) (int, string) {
	u.t.Helper()
	cmd := exec.Command(u.bin, args...) //nolint:gosec // the binary the test built
	cmd.Env = u.env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		u.t.Fatalf("hedgehug %s succeeded", strings.Join(args, " "))
	}
	return cmd.ProcessState.ExitCode(), stderr.String()
}

// TestE2EAuthLoginAndStatus logs in with the device flow, which the fake
// approves, and checks the login with auth status. It runs the commands in
// this process, with go-keyring's mock as the keychain (see above).
func TestE2EAuthLoginAndStatus(t *testing.T) {
	keyring.MockInit()
	zero := 0
	srv := httptest.NewServer(fakegh.New(fakegh.State{Users: []string{"carol"}, AutoApprove: "carol", DeviceInterval: &zero}))
	defer srv.Close()
	te := newTestEnv(t)
	te.writeConfigYAML("hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\", client_id: Iv23e2e}\n")
	code, out, errs := te.run("auth", "login")
	if code != 0 || !strings.Contains(errs, "enter the code") || !strings.Contains(out, "Logged in as github.com/carol") {
		t.Fatalf("auth login: exit %d\nstdout: %s\nstderr: %s", code, out, errs)
	}
	if out := te.ok("auth", "status"); !strings.Contains(out, "github.com/carol") || !strings.Contains(out, "device") || !strings.Contains(out, " ok") {
		t.Errorf("auth status:\n%s", out)
	}
	if out := te.ok("auth", "status", "--json"); !strings.Contains(out, `"state": "ok"`) {
		t.Errorf("auth status --json:\n%s", out)
	}
}

func TestE2EPhase3a(t *testing.T) {
	t.Parallel()
	bin, devenv := buildHedgehug(t), buildDevenv(t)
	dev, fake, apiURL := initDevenv(t, devenv)
	alice, bob := devUserOf(t, bin, devenv, dev, "alice"), devUserOf(t, bin, devenv, dev, "bob")

	// alice's two accounts each have their token, and GitHub reports that
	// each token belongs to the login named in config.yaml.
	if out := alice.ok("auth", "status"); strings.Count(out, " ok") != 2 {
		t.Errorf("alice's auth status:\n%s", out)
	}

	// store init, local branch: url_rewrites points github.com at a local
	// path, so the command makes the bare there and pushes, and does not call
	// GitHub.
	alice.ok("store", "init", "extra", "--prefix", "EXT", "--account", "personal", "--remote", "https://github.com/kukv/hh-extra.git")
	if ok, err := gitrepo.HeadHasFile(filepath.Join(dev, "remotes", "kukv", "hh-extra.git"), ".hedgehug/store.yaml"); err != nil || !ok {
		t.Errorf("the bare of the local branch was not made and pushed: %v %v", ok, err)
	}
	if _, ok := fake.Repo("kukv/hh-extra"); ok {
		t.Error("the local branch made a repository on the fake GitHub")
	}

	// store init, GitHub branch: with no url_rewrites, the remote is on a
	// GitHub host (127.0.0.1 is one, by hosts), so the command makes the
	// repository there, through the fake, with the topic. The push then fails,
	// as nothing listens on port 1; the repository is left, and the way to use
	// it is told.
	dave := filepath.Join(t.TempDir(), "dave")
	if err := os.MkdirAll(dave, 0o750); err != nil {
		t.Fatal(err)
	}
	cfg := "accounts:\n  - {name: personal, host: 127.0.0.1, login: alice, token: \"env:HH_DEV_TOKEN\"}\nhosts:\n  - {host: 127.0.0.1, api: \"" + apiURL + "\", web: \"" + apiURL + "\"}\n"
	if err := os.WriteFile(filepath.Join(dave, "config.yaml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	d := &devUser{t: t, bin: bin, home: dave, env: append(slicesWithout(alice.env, "HEDGEHUG_HOME=", "HOME=", "USERPROFILE="), "HEDGEHUG_HOME="+dave, "HOME="+dave, "USERPROFILE="+dave)}
	code, errs := d.fail("store", "init", "gh", "--prefix", "GH", "--remote", "http://127.0.0.1:1/alice/hh-gh.git")
	if code != 1 || !strings.Contains(errs, "hedgehug store add gh") {
		t.Errorf("store init on the GitHub branch: exit %d, %q", code, errs)
	}
	if r, ok := fake.Repo("alice/hh-gh"); !ok || !r.Private || len(r.Topics) != 1 || r.Topics[0] != "hedgehug-store" {
		t.Errorf("the repository on the fake GitHub = %+v %v", r, ok)
	}

	// bob may only read org-b: the sync finds out, and writes are refused.
	bob.ok("sync")
	if code, errs := bob.fail("add", "x", "--store", "org-b"); code != 1 || !strings.Contains(errs, "org-b is read-only (the GitHub permission is read)") {
		t.Errorf("bob's add: exit %d, %q", code, errs)
	}
	bob.ok("add", "y", "--store", "org-a") // org-a he may write

	// The sync of a read-only store only fetches: what alice pushes comes in,
	// and nothing of bob's goes out.
	alice.ok("add", "from alice", "--store", "org-b")
	alice.ok("sync")
	bob.ok("sync")
	if out := bob.ok("ls", "--store", "org-b"); !strings.Contains(out, "from alice") {
		t.Errorf("bob's sync did not fetch alice's task:\n%s", out)
	}
	s, err := store.Open(filepath.Join(bob.home, "stores", "org-b"))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := hhsync.Unpushed(s.Repo()); err != nil || n != 0 {
		t.Errorf("bob has %d commits of org-b that origin lacks (%v), want none", n, err)
	}

	// Given write, he may write after the next check.
	if err := fake.SetPerm("org-b/hh-ops", "bob", "write"); err != nil {
		t.Fatal(err)
	}
	acc, err := hhsync.ReadAccess(s)
	if err != nil {
		t.Fatal(err)
	}
	acc.Checked = time.Now().Add(-10 * time.Minute) // the 5 minutes have passed
	if err := hhsync.WriteAccess(s, acc); err != nil {
		t.Fatal(err)
	}
	bob.ok("sync")
	acc, err = hhsync.ReadAccess(s)
	if err != nil {
		t.Fatal(err)
	}
	if acc.ReadOnly {
		t.Errorf("org-b is still read-only: %+v", acc)
	}
	bob.ok("add", "x", "--store", "org-b")

	// discover: a new store repository on GitHub (the code repository
	// org-a/web has no topic and is not offered), and the bares under remotes.
	if err := fake.AddRepo("org-a/hh-more", []string{"hedgehug-store"}, false); err != nil {
		t.Fatal(err)
	}
	if err := fake.SetPerm("org-a/hh-more", "alice-corp", "write"); err != nil {
		t.Fatal(err)
	}
	if out := alice.ok("store", "discover"); !strings.Contains(out, "https://github.com/org-a/hh-more.git") || strings.Contains(out, "hh-shared") || strings.Contains(out, "web") {
		t.Errorf("alice's discover:\n%s", out)
	}
	carol := filepath.Join(t.TempDir(), "carol")
	if err := os.MkdirAll(carol, 0o750); err != nil {
		t.Fatal(err)
	}
	c := &devUser{t: t, bin: bin, home: carol, env: append(slicesWithout(bob.env, "HEDGEHUG_HOME=", "HOME=", "USERPROFILE="), "HEDGEHUG_HOME="+carol, "HOME="+carol, "USERPROFILE="+carol)}
	c.ok("store", "discover", "--from", filepath.Join(dev, "remotes"), "--all")
	if out := c.ok("store", "list"); !strings.Contains(out, "hh-ops") || !strings.Contains(out, "hh-shared") || !strings.Contains(out, "hh-tasks") {
		t.Errorf("carol's stores:\n%s", out)
	}
}

// slicesWithout drops the entries of env that start with one of prefixes.
func slicesWithout(env []string, prefixes ...string) []string {
	var out []string
	for _, kv := range env {
		keep := true
		for _, p := range prefixes {
			if strings.HasPrefix(kv, p) {
				keep = false
			}
		}
		if keep {
			out = append(out, kv)
		}
	}
	return out
}
