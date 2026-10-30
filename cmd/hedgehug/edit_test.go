package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/model"
)

// taskFile returns the path of the task file of id in the store me.
func (te *testEnv) taskFile(id string) string {
	te.t.Helper()
	return filepath.Join(te.storeDir("me"), filepath.FromSlash(model.TaskPath(mustID(te.t, id))))
}

// raw returns the task file of id as it is.
func (te *testEnv) raw(id string) string {
	te.t.Helper()
	b, err := os.ReadFile(te.taskFile(id))
	if err != nil {
		te.t.Fatal(err)
	}
	return string(b)
}

// commitFile writes data to the task file of id and commits it by hand.
func (te *testEnv) commitFile(id, data string) {
	te.t.Helper()
	if err := os.WriteFile(te.taskFile(id), []byte(data), 0o600); err != nil {
		te.t.Fatal(err)
	}
	r, err := git.PlainOpen(te.storeDir("me"))
	if err != nil {
		te.t.Fatal(err)
	}
	w, err := r.Worktree()
	if err != nil {
		te.t.Fatal(err)
	}
	if _, err := w.Add(model.TaskPath(mustID(te.t, id))); err != nil {
		te.t.Fatal(err)
	}
	if _, err := w.Commit("by hand", &git.CommitOptions{Author: &object.Signature{Name: "T", Email: "t@example.com", When: time.Now()}}); err != nil {
		te.t.Fatal(err)
	}
}

// setLine sets the line key: value of a task file, adding it after the id
// line when there is none.
func setLine(file, key, value string) string {
	lines := strings.SplitAfter(file, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, key+":") {
			lines[i] = key + ": " + value + "\n"
			return strings.Join(lines, "")
		}
	}
	for i, l := range lines {
		if strings.HasPrefix(l, "id:") {
			return strings.Join(slices.Insert(lines, i+1, key+": "+value+"\n"), "")
		}
	}
	panic("no id line in " + file)
}

func TestEdit(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	before := len(te.log("me"))
	te.editWith(setLine(te.raw(x.ID), "title", "Renamed"))

	out := te.ok("edit", x.Key)
	if !strings.HasPrefix(out, "Updated ") || !strings.Contains(out, "Renamed") {
		t.Errorf("stdout = %q", out)
	}
	log := te.log("me")
	if len(log) != before+1 || !strings.Contains(log[0].Message, "Hedgehug-Op: edit") {
		t.Errorf("commits = %d (want %d), last %q", len(log), before+1, log[0].Message)
	}
	if out := te.ok("show", x.Key); !strings.Contains(out, "Renamed") {
		t.Errorf("show = %q", out)
	}
	if fs := te.tempFiles(); len(fs) != 0 {
		t.Errorf("temporary files left: %v", fs)
	}

	// --json prints the task as written.
	te.editWith(setLine(te.raw(x.ID), "title", "Again"))
	var got taskJSON
	if err := json.Unmarshal([]byte(te.ok("edit", x.Key, "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Title != "Again" {
		t.Errorf("edit --json title = %q, want Again", got.Title)
	}
}

func TestEditNoChanges(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	before := len(te.log("me"))
	seen := te.editWith(te.raw(x.ID))

	out := te.ok("edit", x.Key)
	if !strings.HasPrefix(out, "No changes") {
		t.Errorf("stdout = %q, want No changes", out)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
	if fs := te.tempFiles(); len(fs) != 0 {
		t.Errorf("temporary files left: %v", fs)
	}
	if s := seen(); len(s) != 1 || s[0] != te.raw(x.ID) {
		t.Errorf("the editor saw %q, want the task file", s)
	}
}

func TestEditRefused(t *testing.T) {
	for _, tc := range []struct {
		key, value, want string
	}{
		{"id", "", "id cannot be changed"},
		{"key", "WEB-1", "key cannot be changed"},
		{"created", "2020-01-01T00:00:00Z", "created cannot be changed"},
		{"updated", "2020-01-01T00:00:00Z", "updated cannot be changed"},
		{"deleted_at", "2020-01-01T00:00:00Z", "deleted_at cannot be changed"},
		{"parent", "", "parent cannot be changed here; use hedgehug mv"},
		{"rank", "a5V", "rank cannot be changed here; use hedgehug mv"},
		{"former_keys", "[ME-9]", "former_keys cannot be changed"},
		{"workspace", "", "workspace cannot be changed here; use hedgehug mv --workspace"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			te := newStoreEnv(t)
			x := te.addJSON("Alpha")
			other := te.addJSON("Other")
			value := tc.value
			if value == "" {
				value = other.ID
			}
			edited := setLine(te.raw(x.ID), tc.key, value)
			te.editWith(edited)
			before := len(te.log("me"))

			code, _, stderr := te.run("edit", x.Key)
			if code != 1 || !strings.Contains(stderr, tc.want) {
				t.Errorf("exit %d (want 1), stderr %q (want %q)", code, stderr, tc.want)
			}
			if strings.Contains(stderr, "[Y/n]") {
				t.Errorf("asked again without a terminal: %q", stderr)
			}
			fs := te.tempFiles()
			if len(fs) != 1 || !strings.Contains(stderr, fs[0]) {
				t.Fatalf("temporary files %v, stderr %q: want one, named in stderr", fs, stderr)
			}
			if b, _ := os.ReadFile(fs[0]); string(b) != edited {
				t.Errorf("the kept file = %q, want the edit", b)
			}
			if n := len(te.log("me")); n != before {
				t.Errorf("commits = %d, want %d", n, before)
			}
		})
	}
}

func TestEditRetry(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	other := te.addJSON("Other")
	before := len(te.log("me"))
	bad := setLine(te.raw(x.ID), "id", other.ID)
	good := setLine(te.raw(x.ID), "title", "Fixed")
	seen := te.editWith(bad, good)
	te.terminal, te.stdin = true, "y\n"

	code, out, stderr := te.run("edit", x.Key)
	if code != 0 || !strings.Contains(out, "Fixed") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	if !strings.Contains(stderr, "Open the editor again? [Y/n] ") {
		t.Errorf("stderr = %q, want the question", stderr)
	}
	if s := seen(); len(s) != 2 || s[1] != bad {
		t.Errorf("the editor saw %q, want the last edit the second time", s)
	}
	if n := len(te.log("me")); n != before+1 {
		t.Errorf("commits = %d, want %d", n, before+1)
	}
	if fs := te.tempFiles(); len(fs) != 0 {
		t.Errorf("temporary files left: %v", fs)
	}
}

func TestEditValidation(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	before := len(te.log("me"))
	te.editWith(setLine(te.raw(x.ID), "status", "nope"))
	te.terminal, te.stdin = true, "n\n"

	code, _, stderr := te.run("edit", x.Key)
	if code != 1 || !strings.Contains(stderr, "breaks the schema") || !strings.Contains(stderr, "[Y/n]") {
		t.Errorf("exit %d (want 1), stderr %q", code, stderr)
	}
	if fs := te.tempFiles(); len(fs) != 1 || !strings.Contains(stderr, fs[0]) || !strings.HasPrefix(filepath.Base(fs[0]), "hedgehug-"+x.Key+"-") {
		t.Errorf("temporary files %v, stderr %q: want one named after the key, named in stderr", fs, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestEditDecodeError(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	te.editWith("not a task\n")

	code, _, stderr := te.run("edit", x.Key)
	if code != 1 || !strings.Contains(stderr, "decode") {
		t.Errorf("exit %d (want 1), stderr %q", code, stderr)
	}
}

func TestEditBroken(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	good := te.raw(x.ID)
	broken := "not a task\x00\n: [\n"
	te.commitFile(x.ID, broken)
	before := len(te.log("me"))
	seen := te.editWith(setLine(good, "title", "Mended"))

	te.ok("edit", x.ID) // a broken task has no key to give
	if s := seen(); len(s) != 1 || s[0] != broken {
		t.Errorf("the editor saw %q, want the broken bytes", s)
	}
	if out := te.ok("show", x.Key); strings.Contains(out, "broken:") || !strings.Contains(out, "Mended") {
		t.Errorf("show = %q", out)
	}
	if log := te.log("me"); len(log) != before+1 || !strings.Contains(log[0].Message, "Hedgehug-Op: edit") {
		t.Errorf("commits = %d (want %d), last %q", len(log), before+1, log[0].Message)
	}

	// A broken task still keeps its id.
	other := te.addJSON("Other")
	te.commitFile(x.ID, broken)
	te.editWith(setLine(good, "id", other.ID))
	if code, _, stderr := te.run("edit", x.ID); code != 1 || !strings.Contains(stderr, "the id must stay "+x.ID) {
		t.Errorf("exit %d (want 1), stderr %q", code, stderr)
	}
}

func TestEditChangedWhileEditing(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	before := len(te.log("me"))
	te.editWith(setLine(te.raw(x.ID), "title", "Mine") + "TOUCH " + te.taskFile(x.ID) + "\n")
	te.terminal, te.stdin = true, "y\n"

	code, _, stderr := te.run("edit", x.Key)
	if code != 1 || !strings.Contains(stderr, "changed while editing") {
		t.Errorf("exit %d (want 1), stderr %q", code, stderr)
	}
	if strings.Contains(stderr, "[Y/n]") {
		t.Errorf("asked again: %q", stderr)
	}
	if fs := te.tempFiles(); len(fs) != 1 || !strings.Contains(stderr, fs[0]) {
		t.Errorf("temporary files %v, stderr %q: want one, named in stderr", fs, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestEditDeleted(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	te.ok("rm", x.Key)
	te.editWith()

	code, _, stderr := te.run("edit", x.Key)
	if code != 1 || !strings.Contains(stderr, "is deleted; restore "+label(x)+" first") {
		t.Errorf("exit %d (want 1), stderr %q", code, stderr)
	}
	if fs := te.tempFiles(); len(fs) != 0 {
		t.Errorf("the editor was opened: %v", fs)
	}
}

func TestEditEditorFails(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	before := len(te.log("me"))
	te.editWith("EXIT1")

	code, _, stderr := te.run("edit", x.Key)
	if code != 1 || !strings.Contains(stderr, "editor") {
		t.Errorf("exit %d (want 1), stderr %q", code, stderr)
	}
	if fs := te.tempFiles(); len(fs) != 1 || !strings.Contains(stderr, fs[0]) {
		t.Errorf("temporary files %v, stderr %q: want one, named in stderr", fs, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

// A blank $EDITOR is refused before any temporary file is made.
func TestEditBlankEditor(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	te.editWith()
	te.env["EDITOR"] = "   "

	code, _, stderr := te.run("edit", x.Key)
	if code != 1 || !strings.Contains(stderr, "$EDITOR is blank") {
		t.Errorf("exit %d (want 1), stderr %q", code, stderr)
	}
	if fs := te.tempFiles(); len(fs) != 0 {
		t.Errorf("temporary files left: %v", fs)
	}
}

func TestEditorCommand(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		visual, editor string
		want           []string
	}{
		{"code --wait", "nano", []string{"code", "--wait"}},
		{"", "  nano  -w ", []string{"nano", "-w"}},
		{"", "", []string{defaultEditor}},
	} {
		a := &app{env: &env{getenv: func(k string) string {
			return map[string]string{"VISUAL": tc.visual, "EDITOR": tc.editor}[k]
		}}}
		got, err := a.editor()
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("editor() with VISUAL %q, EDITOR %q = %q, %v; want %q", tc.visual, tc.editor, got, err, tc.want)
		}
	}
}
