package cli

import (
	"strings"
	"testing"
	"time"
)

const noAccountMessage = "hedgehug: no account is set up; add one with hedgehug auth add <name> --login <login> (or hedgehug auth login)\n"

// Without an account only the commands that set one up run (spec §5.1);
// every other one stops before it does anything.
func TestCommandsNeedAnAccount(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"ls"},
		{"add", "x"},
		{"show", "ME-1"},
		{"store", "init", "me", "--prefix", "ME"},
		{"store", "list"},
		{"sync"},
		{"doctor"},
		{"inbox"},
		{"undo"},
		{"ui"},
		{"auth", "logout", "x"},
		{"workspace", "ls"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			te := newTestEnv(t)
			te.dropDefaultAccount()
			code, out, errs := te.run(args...)
			if code != 1 || out != "" || errs != noAccountMessage {
				t.Errorf("exit %d, stdout %q, stderr %q; want exit 1 and %q", code, out, errs, noAccountMessage)
			}
		})
	}
}

// A command that runs without an account starts no background sync, even for
// a store with a remote that is due: the child would stop at its own entry.
func TestNoAccountStartsNoBackgroundSync(t *testing.T) {
	alice := settled(t)
	alice.now = alice.now.Add(time.Hour) // the store is due
	alice.dropDefaultAccount()
	started := 0
	defer stubSpawn(func([]string) { started++ })()
	alice.env["HEDGEHUG_AUTO_SYNC"] = "1"
	for _, args := range [][]string{{"auth", "status"}, {"help"}, {"ls", "--help"}} {
		alice.run(args...)
	}
	if started != 0 {
		t.Errorf("started %d background syncs without an account", started)
	}
}

func TestCommandsThatRunWithoutAnAccount(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"--help"}, {"help"}, {"help", "ls"}, {"ls", "--help"}, {"--version"}, {"auth", "status"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			te := newTestEnv(t)
			te.dropDefaultAccount()
			if code, _, errs := te.run(args...); code != 0 || strings.Contains(errs, "no account is set up") {
				t.Errorf("exit %d, stderr %q", code, errs)
			}
		})
	}
	// auth login and merge-driver get to their own checks.
	te := newTestEnv(t)
	te.dropDefaultAccount()
	if code, _, errs := te.run("auth", "login", "--with-token"); code != 1 || !strings.Contains(errs, "no token on stdin") {
		t.Errorf("auth login: exit %d, stderr %q", code, errs)
	}
	if code, _, errs := te.run("merge-driver"); code == 0 || strings.Contains(errs, "no account is set up") {
		t.Errorf("merge-driver: exit %d, stderr %q; want its own argument error", code, errs)
	}
}
