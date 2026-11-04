package scenario

import (
	"bytes"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
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
	if ok, err := gitrepotest.MainHasFile(filepath.Join(dev, "remotes", "kukv", "hh-extra.git"), ".hedgehug/store.yaml"); err != nil || !ok {
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
	d := &devUser{t: t, bin: bin, home: dave, env: append(slicesWithout(alice.env, "HEDGEHUG_HOME=", "HOME="), "HEDGEHUG_HOME="+dave, "HOME="+dave)}
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
	if n := bob.uc().Unpushed("org-b"); n != 0 {
		t.Errorf("bob has %d commits of org-b that origin lacks, want none", n)
	}

	// Given write, he may write after the next check.
	if err := fake.SetPerm("org-b/hh-ops", "bob", "write"); err != nil {
		t.Fatal(err)
	}
	acc := bob.uc().ReadAccess("org-b")
	acc.Checked = time.Now().Add(-10 * time.Minute) // the 5 minutes have passed
	bob.uc().WriteAccess("org-b", acc)
	bob.ok("sync")
	acc = bob.uc().ReadAccess("org-b")
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
	// Task 3 replaces this with hedgehug auth add.
	if err := os.WriteFile(filepath.Join(carol, "config.yaml"), []byte("accounts:\n  - {name: carol, host: github.com, login: carol}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &devUser{t: t, bin: bin, home: carol, env: append(slicesWithout(bob.env, "HEDGEHUG_HOME=", "HOME="), "HEDGEHUG_HOME="+carol, "HOME="+carol)}
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
