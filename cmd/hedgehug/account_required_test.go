package main

import (
	"strings"
	"testing"
)

const noAccountMessage = "hedgehug: no account is set up; add one with hedgehug auth add <name> --login <login> (or hedgehug auth login)\n"

// Without an account only the commands that set one up run (spec §5.1);
// every other one stops before it does anything.
func TestCommandsNeedAnAccount(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"ls"}, {"add", "x"}, {"show", "ME-1"}, {"store", "init", "me", "--prefix", "ME"}, {"store", "list"},
		{"sync"}, {"doctor"}, {"inbox"}, {"undo"}, {"ui"}, {"auth", "logout", "x"}, {"workspace", "ls"},
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
