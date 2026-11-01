package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/zalando/go-keyring"

	"github.com/kukv/hedgehug/internal/gitrepo"
	hhsync "github.com/kukv/hedgehug/internal/sync"
)

// testEnv runs the command with HEDGEHUG_HOME and HOME in temporary
// directories, and an author in HOME/.gitconfig.
type testEnv struct {
	t        *testing.T
	home     string
	hhHome   string
	stdin    string
	terminal bool               // what isTerminal reports for every stream
	openURL  func(string) error // what the browser is opened with; nil for none
	now      time.Time
	env      map[string]string // more variables, looked up after the fixed ones
	tmp      string            // the directory of temporary files, set by editWith
	dir      string            // the current directory of the runs; "" for none

	beforeUpdate func() // Session.BeforeUpdate of the next runs; meanwhile sets it
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
	case "HOME":
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
		openURL:    te.openURL,
		getwd:      func() (string, error) { return te.dir, nil },

		beforeUpdate: te.beforeUpdate,
	}
	if args == nil {
		args = []string{} // cobra reads os.Args, the flags of go test, for nil
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

// writeConfigYAML puts s at the start of config.yaml.
func (te *testEnv) writeConfigYAML(s string) {
	te.t.Helper()
	if err := os.MkdirAll(te.hhHome, 0o750); err != nil {
		te.t.Fatal(err)
	}
	p := filepath.Join(te.hhHome, "config.yaml")
	old, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		te.t.Fatal(err)
	}
	if err := os.WriteFile(p, append([]byte(s), old...), 0o600); err != nil {
		te.t.Fatal(err)
	}
}

// withOtherWriter runs fn with account (host/login) as the only account in
// config.yaml, so the commands in fn are written by someone else, and puts
// the config back after.
func (te *testEnv) withOtherWriter(account string, fn func()) {
	te.t.Helper()
	host, login, _ := strings.Cut(account, "/")
	p := filepath.Join(te.hhHome, "config.yaml")
	old, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil {
		te.t.Fatal(err)
	}
	rest, skipping := []string{}, false
	for _, l := range strings.SplitAfter(string(old), "\n") {
		switch {
		case strings.HasPrefix(l, "accounts:"):
			skipping = true
		case skipping && (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "-")):
		case strings.HasPrefix(strings.TrimSpace(l), "account: "):
			// stores[].account names the account that is being replaced; the
			// other writer's stores have none.
		default:
			skipping = false
			rest = append(rest, l)
		}
	}
	head := fmt.Sprintf("accounts:\n  - name: %s\n    host: %s\n    login: %s\n", login, host, login)
	if err := os.WriteFile(p, []byte(head+strings.Join(rest, "")), 0o600); err != nil {
		te.t.Fatal(err)
	}
	defer func() {
		if err := os.WriteFile(p, old, 0o600); err != nil {
			te.t.Fatal(err)
		}
	}()
	fn()
}

// TestMain runs the tests, or acts as the editor when HH_TEST_EDITOR_SCRIPT
// is set: editWith points $EDITOR at this test binary.
func TestMain(m *testing.M) {
	if script := os.Getenv("HH_TEST_EDITOR_SCRIPT"); script != "" {
		os.Exit(testEditor(script, os.Args[len(os.Args)-1]))
	}
	// No test reaches the keychain of the machine.
	keyring.MockInit()
	// A program built with -race sleeps a second when it exits, by default,
	// so that goroutines still running at the exit can be caught in a race.
	// The children of the tests (this binary as the editor, the hedgehug
	// buildHedgehug makes) leave no goroutine of their own work running when
	// they exit, so for them the sleep only makes each run a second longer.
	_ = os.Setenv("GORACE", strings.TrimSpace(os.Getenv("GORACE")+" atexit_sleep_ms=0"))
	// hedgehug is on PATH, as doctor expects of a store with a merge driver.
	pathDir, err := os.MkdirTemp("", "hedgehug-path-")
	if err == nil {
		name := "hedgehug"
		if err := os.WriteFile(filepath.Join(pathDir, name), nil, 0o700); err == nil { //nolint:gosec // an empty file that only has to be found
			_ = os.Setenv("PATH", pathDir+string(os.PathListSeparator)+os.Getenv("PATH"))
		}
	}
	code := m.Run()
	if pathDir != "" {
		_ = os.RemoveAll(pathDir)
	}
	if builtDir != "" {
		_ = os.RemoveAll(builtDir)
	}
	if devenvDir != "" {
		_ = os.RemoveAll(devenvDir)
	}
	os.Exit(code)
}

// handEdit changes the file of the task id in the work tree of the store
// name, as an editor would (old becomes new), and commits it as a hand edit
// would, past every check of the commands.
func (te *testEnv) handEdit(name, id, old, replacement string) { //nolint:unparam // every test uses the one store "me"; the name keeps call sites readable
	te.t.Helper()
	dir := te.storeDir(name)
	var rel string
	err := filepath.WalkDir(filepath.Join(dir, "tasks"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p) //nolint:gosec // a file of the test's own store
		if err == nil && strings.Contains(string(b), "\nid: "+id+"\n") {
			rel, err = filepath.Rel(dir, p)
		}
		return err
	})
	if err != nil || rel == "" {
		te.t.Fatalf("no file of %s: %v", id, err)
	}
	b, err := os.ReadFile(filepath.Join(dir, rel)) //nolint:gosec // a file of the test's own store
	if err != nil {
		te.t.Fatal(err)
	}
	if !strings.Contains(string(b), old) {
		te.t.Fatalf("%s has no %q", rel, old)
	}
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(strings.Replace(string(b), old, replacement, 1)), 0o600); err != nil {
		te.t.Fatal(err)
	}
	r, err := gitrepo.Open(dir)
	if err != nil {
		te.t.Fatal(err)
	}
	c := gitrepo.Commit{Paths: []string{filepath.ToSlash(rel)}, Subject: "hand edit", Author: gitrepo.Signature{Name: "Alice", Email: "alice@example.com"}, When: te.now}
	if _, err := r.Commit(c); err != nil {
		te.t.Fatal(err)
	}
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
		builtBin = filepath.Join(builtDir, name)
		if out, err := exec.Command("go", goBuildArgs(builtBin, ".")...).CombinedOutput(); err != nil { //nolint:gosec // the arguments are ours
			buildErr = fmt.Errorf("go build: %w\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return builtBin
}

// goBuildArgs returns the arguments of go build for pkg into out, with -race
// when this test binary has it: the packages are then in the build cache
// from the build of the tests, and are not compiled again.
func goBuildArgs(out, pkg string) []string {
	args := []string{"build"}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "-race" && s.Value == "true" {
				args = append(args, "-race")
			}
		}
	}
	return append(args, "-o", out, pkg)
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

// writeSync adds the sync section to config.yaml.
func (te *testEnv) writeSync(interval, pushInterval string) {
	te.t.Helper()
	p := filepath.Join(te.hhHome, "config.yaml")
	old, err := os.ReadFile(p) //nolint:gosec // the config of the test
	if err != nil {
		te.t.Fatal(err)
	}
	sec := fmt.Sprintf("sync:\n  interval: %s\n  push_interval: %s\n", interval, pushInterval)
	if err := os.WriteFile(p, append(old, sec...), 0o600); err != nil {
		te.t.Fatal(err)
	}
}

// setSyncError makes the last background sync of the store look failed and
// not yet reported.
func (te *testEnv) setSyncError(name, msg string) {
	te.t.Helper()
	b, err := json.Marshal(map[string]any{"last_error": msg, "last_attempt": te.now, "last_success": te.now})
	if err != nil {
		te.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(te.storeDir(name), ".hedgehug", "local", "sync.json"), b, 0o600); err != nil {
		te.t.Fatal(err)
	}
}

// stubSpawn replaces the start of the background sync with f, which gets the
// arguments of the child.
func stubSpawn(f func(args []string)) (restore func()) {
	return hhsync.SetSpawn(func(_ string, args []string, _ string) error { f(args); return nil })
}

// runBinary runs the built command with the variables of the test, and
// HEDGEHUG_AUTO_SYNC as te.env sets it.
func (te *testEnv) runBinary(bin string, args ...string) {
	te.t.Helper()
	cmd := exec.Command(bin, args...) //nolint:gosec // the binary the test built
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "HOME=") && !strings.HasPrefix(kv, "HEDGEHUG_") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.Env = append(cmd.Env, "HOME="+te.home, "HEDGEHUG_HOME="+te.hhHome)
	for k, v := range te.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		te.t.Fatalf("hedgehug %s: %v\n%s", strings.Join(args, " "), err, out)
	}
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
