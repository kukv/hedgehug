package cli

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// complete asks for the candidates of the last of args, as a shell does.
func (te *testEnv) complete(args ...string) []string {
	te.t.Helper()
	code, out, errs := te.run(append([]string{cobra.ShellCompRequestCmd}, args...)...)
	if code != 0 {
		te.t.Fatalf("complete %v: exit %d\n%s", args, code, errs)
	}
	var got []string
	for line := range strings.Lines(out) {
		line = strings.TrimRight(line, "\n")
		if strings.HasPrefix(line, ":") {
			break // the directive
		}
		name, _, _ := strings.Cut(line, "\t")
		got = append(got, name)
	}
	return got
}

func TestCompletionScript(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		for _, name := range []string{"hedgehug", "hh"} {
			t.Run(shell+"/"+name, func(t *testing.T) {
				te := newTestEnv(t)
				te.progName = name
				out := te.ok("completion", shell)
				if !strings.Contains(out, name+" ") || !strings.Contains(out, "__complete") {
					t.Fatalf("no script for %s:\n%.300s", name, out)
				}
				if name == "hh" && strings.Contains(out, "hedgehug") {
					t.Fatalf("the script of hh names hedgehug")
				}
			})
		}
	}
}

func TestCompletionOfUnknownShell(t *testing.T) {
	te := newTestEnv(t)
	if code, _, _ := te.run("completion", "powershell"); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

func TestCompletionNeedsNoAccount(t *testing.T) {
	te := newTestEnv(t)
	te.dropDefaultAccount()
	te.ok("completion", "zsh")
	if got := te.complete("show", ""); len(got) != 0 {
		t.Fatalf("candidates without an account: %v", got)
	}
}

func TestCompletionCandidates(t *testing.T) {
	te := newStoreEnv(t)
	task := te.addJSON("Write the spec")
	te.ok("workspace", "add", "work", "--prefix", "WK")
	te.ok("sprint", "add", "s1", "--workspace", "work")
	te.ok("release", "add", "r1", "--workspace", "work")
	te.ok("view", "save", "mine", "--query", "status:todo")
	te.uc().WriteMembers("me", []usecasetest.Member{{ID: "github.com/bob", Permission: "write"}})

	for _, c := range []struct {
		name string
		args []string
		want string
	}{
		{"id", []string{"show", ""}, task.ID},
		{"id of a later word", []string{"status", ""}, task.ID},
		{"status", []string{"status", task.ID, ""}, "todo"},
		{"key", []string{"set", task.ID, ""}, "status="},
		{"status value", []string{"set", task.ID, "status="}, "status=todo"},
		{"assignee value", []string{"set", task.ID, "assignees="}, "assignees=github.com/bob"},
		{"key after a key", []string{"set", task.ID, "title=x", ""}, "assignees="},
		{"query", []string{"ls", "assignee:"}, "assignee:github.com/bob"},
		{"parent", []string{"add", "x", "--parent", ""}, task.ID},
		{"store", []string{"ls", "--store", ""}, "me"},
		{"workspace", []string{"add", "x", "--workspace", ""}, "work"},
		{"workspace of a command", []string{"workspace", "rm", ""}, "work"},
		{"sprint", []string{"sprint", "start", ""}, "s1"},
		{"sprint flag", []string{"add", "x", "--sprint", ""}, "s1"},
		{"release", []string{"release", "notes", ""}, "r1"},
		{"release flag", []string{"add", "x", "--release", ""}, "r1"},
		{"view", []string{"view", "show", ""}, "mine"},
		{"project", []string{"use", ""}, task.ID},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := te.complete(c.args...); !slices.Contains(got, c.want) {
				t.Fatalf("%v: %v has no %q", c.args, got, c.want)
			}
		})
	}
}

func TestCompletionWithNoStore(t *testing.T) {
	te := newTestEnv(t)
	if got := te.complete("show", ""); len(got) != 0 {
		t.Fatalf("candidates without a store: %v", got)
	}
}

// Every command that takes a task, a project or a workspace completes it.
func TestCompletionCoversTheCommands(t *testing.T) {
	a := &app{env: &env{}, ss: &usecase.Session{}}
	takes := regexp.MustCompile(`^\S+ \[?<(id|project|workspace)>`)
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if takes.MatchString(c.Use) && c.ValidArgsFunction == nil {
			t.Errorf("%s has no completion", c.CommandPath())
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(a.rootCmd())
}
