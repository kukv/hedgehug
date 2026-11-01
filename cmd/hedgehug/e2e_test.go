package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	devenvOnce sync.Once
	devenvDir  string // the directory of the binary buildDevenv made; TestMain removes it
	devenvBin  string
	devenvErr  error
)

// buildDevenv builds tools/devenv once for all tests and returns the path of
// the binary.
func buildDevenv(t *testing.T) string {
	t.Helper()
	devenvOnce.Do(func() {
		var err error
		devenvDir, err = os.MkdirTemp("", "devenv-bin-")
		if err != nil {
			devenvErr = err
			return
		}
		name := "devenv"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		devenvBin = filepath.Join(devenvDir, name)
		if out, err := exec.Command("go", goBuildArgs(devenvBin, "../../tools/devenv")...).CombinedOutput(); err != nil { //nolint:gosec // the arguments are ours
			devenvErr = fmt.Errorf("go build devenv: %w\n%s", err, out)
		}
	})
	if devenvErr != nil {
		t.Fatal(devenvErr)
	}
	return devenvBin
}

// devUser is a user of a devenv environment: the command, run with the
// variables `devenv env` prints for them.
type devUser struct {
	t    *testing.T
	bin  string
	home string // the user's HEDGEHUG_HOME
	env  []string
}

// exportLine is a line of `devenv env`: export NAME='value'.
var exportLine = regexp.MustCompile(`^export ([A-Z_]+)='(.*)'$`)

func devUserOf(t *testing.T, bin, devenv, dir, user string) *devUser {
	t.Helper()
	out, err := exec.Command(devenv, "env", user, "--dir", dir).Output() //nolint:gosec // the binary the test built
	if err != nil {
		t.Fatalf("devenv env %s: %v", user, err)
	}
	u := &devUser{t: t, bin: bin}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "HOME=") && !strings.HasPrefix(kv, "USERPROFILE=") && !strings.HasPrefix(kv, "HEDGEHUG_") && !strings.HasPrefix(kv, "GIT_CONFIG_") {
			u.env = append(u.env, kv)
		}
	}
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		m := exportLine.FindStringSubmatch(l)
		if m == nil {
			t.Fatalf("devenv env %s: unexpected line %q", user, l)
		}
		v := strings.ReplaceAll(m[2], `'\''`, "'")
		u.env = append(u.env, m[1]+"="+v)
		if m[1] == "HEDGEHUG_HOME" {
			u.home = v
		}
	}
	// The home of the user is HEDGEHUG_HOME too, so that nothing of the
	// real home is read.
	u.env = append(u.env, "HOME="+u.home, "USERPROFILE="+u.home, "HEDGEHUG_AUTO_SYNC=0")
	return u
}

// ok runs hedgehug with args, fails the test if it does not succeed, and
// returns what it printed on stdout.
func (u *devUser) ok(args ...string) string {
	u.t.Helper()
	cmd := exec.Command(u.bin, args...) //nolint:gosec // the binary the test built
	cmd.Env = u.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		u.t.Fatalf("hedgehug %s: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return stdout.String()
}

func TestE2EAliceAndBob(t *testing.T) {
	t.Parallel()
	bin, devenv := buildHedgehug(t), buildDevenv(t)
	dev, _, _ := initDevenv(t, devenv)
	alice, bob := devUserOf(t, bin, devenv, dev, "alice"), devUserOf(t, bin, devenv, dev, "bob")

	// Both change the title of ORG-1 apart. Commit times have a precision of
	// a second, so alice's later write is a second apart and wins.
	alice.ok("add", "shared", "--store", "org-a")
	alice.ok("sync")
	bob.ok("sync")
	bob.ok("set", "ORG-1", "title=bob", "--store", "org-a")
	time.Sleep(1100 * time.Millisecond)
	alice.ok("set", "ORG-1", "title=alice", "--store", "org-a")
	bob.ok("sync")
	alice.ok("sync")
	bob.ok("sync")
	a, b := alice.ok("show", "ORG-1", "--store", "org-a", "--json"), bob.ok("show", "ORG-1", "--store", "org-a", "--json")
	if a != b {
		t.Errorf("not converged:\n%s\n%s", a, b)
	}
	if !strings.Contains(a, `"title": "alice"`) {
		t.Errorf("alice's later write did not win:\n%s", a)
	}

	// Bob lost the title; he is told, can write the value back, and take that back.
	inbox := bob.ok("inbox")
	id := regexp.MustCompile(`([0-9A-Z]{26})\s+overwritten`).FindStringSubmatch(inbox)
	if id == nil {
		t.Fatalf("bob's inbox: %s", inbox)
	}
	bob.ok("inbox", "revert", id[1])
	if got := bob.ok("show", "ORG-1", "--store", "org-a", "--json"); !strings.Contains(got, `"title": "bob"`) {
		t.Errorf("revert did not write bob's title back:\n%s", got)
	}
	bob.ok("undo", "--store", "org-a")
	if got := bob.ok("show", "ORG-1", "--store", "org-a", "--json"); !strings.Contains(got, `"title": "alice"`) {
		t.Errorf("undo did not take the revert back:\n%s", got)
	}

	// Two adds apart take the same ID; the one who syncs later is renumbered.
	alice.ok("add", "a", "--store", "org-a")
	bob.ok("add", "b", "--store", "org-a")
	alice.ok("sync")
	bob.ok("sync")
	renumbered := regexp.MustCompile(`renumbered\s+ORG-\d+ \([0-9A-Z]{26}\) b\s+ORG-\d+ was renumbered to ORG-\d+;`)
	if out := bob.ok("inbox"); !renumbered.MatchString(out) {
		t.Errorf("bob was not told of the renumbering of b:\n%s", out)
	}
}
