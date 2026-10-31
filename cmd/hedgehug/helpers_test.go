package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/gitrepo"
)

// testEnv runs the command with HEDGEHUG_HOME and HOME in temporary
// directories, and an author in HOME/.gitconfig.
type testEnv struct {
	t        *testing.T
	home     string
	hhHome   string
	stdin    string
	terminal bool // what isTerminal reports for every stream
	now      time.Time
	env      map[string]string // more variables, looked up after the fixed ones
	tmp      string            // the directory of temporary files, set by editWith
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	d := t.TempDir()
	te := &testEnv{
		t:      t,
		home:   filepath.Join(d, "home"),
		hhHome: filepath.Join(d, "hh"),
		now:    time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC),
		env:    map[string]string{},
	}
	if err := os.MkdirAll(te.home, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(te.home, ".gitconfig"), []byte("[user]\n\tname = Alice\n\temail = alice@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return te
}

func (te *testEnv) getenv(k string) string {
	switch k {
	case "HOME", "USERPROFILE":
		return te.home
	case "HEDGEHUG_HOME":
		return te.hhHome
	}
	if v, ok := te.env[k]; ok {
		return v
	}
	if k == "HEDGEHUG_AUTO_SYNC" {
		return "0"
	}
	return ""
}

func (te *testEnv) run(args ...string) (code int, stdout, stderr string) {
	te.t.Helper()
	var out, errb bytes.Buffer
	e := &env{
		stdin:      strings.NewReader(te.stdin),
		stdout:     &out,
		stderr:     &errb,
		getenv:     te.getenv,
		isTerminal: func(any) bool { return te.terminal },
		now:        func() time.Time { return te.now },
	}
	code = run(args, e)
	return code, out.String(), errb.String()
}

// ok runs the command, fails the test unless it exits 0, and returns stdout.
func (te *testEnv) ok(args ...string) string {
	te.t.Helper()
	code, out, errs := te.run(args...)
	if code != 0 {
		te.t.Fatalf("hedgehug %s: exit %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), code, out, errs)
	}
	return out
}

func (te *testEnv) storeDir(name string) string { return filepath.Join(te.hhHome, "stores", name) }

// log returns the commits of the store, newest first.
func (te *testEnv) log(name string) []*object.Commit { //nolint:unparam // every test uses the one store "me"; the name keeps call sites readable
	te.t.Helper()
	r, err := git.PlainOpen(te.storeDir(name))
	if err != nil {
		te.t.Fatal(err)
	}
	ref, err := r.Head()
	if err != nil {
		te.t.Fatal(err)
	}
	it, err := r.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		te.t.Fatal(err)
	}
	var out []*object.Commit
	if err := it.ForEach(func(c *object.Commit) error {
		out = append(out, c)
		return nil
	}); err != nil {
		te.t.Fatal(err)
	}
	return out
}

// headCommit returns the newest commit of the store.
func (te *testEnv) headCommit(name string) *object.Commit {
	te.t.Helper()
	return te.log(name)[0]
}

// writeAccounts puts one account at the start of config.yaml.
func (te *testEnv) writeAccounts(host, login string) { //nolint:unparam // every test uses github.com; the host keeps call sites readable
	te.t.Helper()
	if err := os.MkdirAll(te.hhHome, 0o750); err != nil {
		te.t.Fatal(err)
	}
	p := filepath.Join(te.hhHome, "config.yaml")
	old, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		te.t.Fatal(err)
	}
	head := fmt.Sprintf("accounts:\n  - name: %s\n    host: %s\n    login: %s\n", login, host, login)
	if err := os.WriteFile(p, append([]byte(head), old...), 0o600); err != nil {
		te.t.Fatal(err)
	}
}

// TestMain runs the tests, or acts as the editor when HH_TEST_EDITOR_SCRIPT
// is set: editWith points $EDITOR at this test binary.
func TestMain(m *testing.M) {
	if script := os.Getenv("HH_TEST_EDITOR_SCRIPT"); script != "" {
		os.Exit(testEditor(script, os.Args[len(os.Args)-1]))
	}
	code := m.Run()
	if builtDir != "" {
		_ = os.RemoveAll(builtDir)
	}
	os.Exit(code)
}

var (
	buildOnce sync.Once
	builtDir  string // the directory of the binary buildHedgehug made
	builtBin  string
	buildErr  error
)

// buildHedgehug builds the command once for all tests and returns the path of
// the binary, which is named hedgehug so that a merge driver finds it on PATH.
func buildHedgehug(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		builtDir, buildErr = os.MkdirTemp("", "hedgehug-bin-")
		if buildErr != nil {
			return
		}
		name := "hedgehug"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		builtBin = filepath.Join(builtDir, name)
		if out, err := exec.Command("go", "build", "-o", builtBin, ".").CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("go build: %w\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return builtBin
}

// bare makes an empty bare repository next to the test's directories and
// returns its path.
func (te *testEnv) bare(name string) string {
	te.t.Helper()
	dir := filepath.Join(filepath.Dir(te.hhHome), name+".git")
	if err := gitrepo.InitBare(dir); err != nil {
		te.t.Fatal(err)
	}
	return dir
}

// other is a second user: a HEDGEHUG_HOME and a HOME of their own, the same
// clock.
func (te *testEnv) other() *testEnv {
	te.t.Helper()
	o := newTestEnv(te.t)
	o.now = te.now
	return o
}

// twoUsers sets up alice and bob on one remote: alice makes the store org
// (prefix ORG) with a task, syncs, and bob adds the store. The remote is
// returned too.
func twoUsers(t *testing.T) (alice, bob *testEnv, remote string) {
	t.Helper()
	alice = newTestEnv(t)
	remote = alice.bare("shared")
	alice.writeAccounts("github.com", "alice")
	alice.ok("store", "init", "org", "--prefix", "ORG", "--remote", remote)
	alice.ok("add", "x")
	alice.ok("sync")
	bob = alice.other()
	bob.writeAccounts("github.com", "bob")
	bob.ok("store", "add", "org", "--remote", remote)
	return alice, bob, remote
}

// editorStepSep separates the steps of a test editor script, and the
// contents in its .seen file.
const editorStepSep = "\n---next---\n"

var errExit1 = errors.New("exit 1")

// testEditor takes the first step of script and does it to file: it
// appends what file holds to <script>.seen, then writes the step to file.
// A step that starts with EXIT1 writes nothing and exits 1. A line
// TOUCH <path> in a step is left out of file, and x is appended to path
// after file is written.
func testEditor(script, file string) int {
	err := doTestEditor(script, file)
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errExit1):
		return 1
	}
	fmt.Fprintln(os.Stderr, "test editor:", err)
	return 2
}

func doTestEditor(script, file string) error {
	b, err := os.ReadFile(script) //nolint:gosec // the script of the test
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return errors.New("no steps left")
	}
	steps := strings.Split(string(b), editorStepSep)
	if err := os.WriteFile(script, []byte(strings.Join(steps[1:], editorStepSep)), 0o600); err != nil {
		return err
	}
	cur, err := os.ReadFile(file) //nolint:gosec // the file hedgehug opens in the editor
	if err != nil {
		return err
	}
	if err := appendFile(script+".seen", string(cur)+editorStepSep); err != nil {
		return err
	}
	step := steps[0]
	if strings.HasPrefix(step, "EXIT1") {
		return errExit1
	}
	var touch, lines []string
	for _, l := range strings.SplitAfter(step, "\n") {
		if p, ok := strings.CutPrefix(l, "TOUCH "); ok {
			touch = append(touch, strings.TrimSuffix(p, "\n"))
			continue
		}
		lines = append(lines, l)
	}
	if err := os.WriteFile(file, []byte(strings.Join(lines, "")), 0o600); err != nil {
		return err
	}
	for _, p := range touch {
		if err := appendFile(p, "x"); err != nil {
			return err
		}
	}
	return nil
}

func appendFile(name, s string) error {
	f, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // a path the test gave
	if err != nil {
		return err
	}
	if _, err := f.WriteString(s); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// editWith makes $EDITOR the test editor, which writes contents in turn,
// one each time it is opened, and puts the temporary files of the command
// in te.tmp. seen returns what the file held each time the editor opened
// it. It sets process variables, so the test must not be parallel.
func (te *testEnv) editWith(contents ...string) (seen func() []string) {
	te.t.Helper()
	d := te.t.TempDir()
	script := filepath.Join(d, "script")
	if err := os.WriteFile(script, []byte(strings.Join(contents, editorStepSep)), 0o600); err != nil {
		te.t.Fatal(err)
	}
	te.t.Setenv("HH_TEST_EDITOR_SCRIPT", script)
	te.tmp = filepath.Join(d, "tmp")
	if err := os.Mkdir(te.tmp, 0o750); err != nil {
		te.t.Fatal(err)
	}
	for _, k := range []string{"TMPDIR", "TMP", "TEMP"} {
		te.t.Setenv(k, te.tmp)
	}
	te.env["EDITOR"] = os.Args[0]
	return func() []string {
		b, err := os.ReadFile(script + ".seen") //nolint:gosec // the file of the test editor
		if err != nil {
			te.t.Fatal(err)
		}
		return strings.Split(strings.TrimSuffix(string(b), editorStepSep), editorStepSep)
	}
}

// tempFiles lists the files left in te.tmp.
func (te *testEnv) tempFiles() []string {
	te.t.Helper()
	es, err := os.ReadDir(te.tmp)
	if err != nil {
		te.t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, filepath.Join(te.tmp, e.Name()))
	}
	return out
}
