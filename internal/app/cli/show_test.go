package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestShow(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("Project", "--kind", "project")
	other := te.addJSON("Other")
	a := te.addJSON("Write the spec", "--parent", p.ID, "-p", "blocked_by+="+other.ID, "-p", "due=2026-11-01", "-p", "labels+=ui")
	waiting := te.addJSON("Waiting", "-p", "blocked_by+="+a.ID)

	// Add a body and a comment through the store; the CLI for them is 1d-2.
	aULID, _ := model.ParseULID(a.ULID)
	te.uc().MustEdit("me", "update", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		tk, err := tx.Task(aULID)
		if err != nil {
			return err
		}
		tk.Body = "The body.\n"
		if err := tx.PutTask(tk); err != nil {
			return err
		}
		return tx.PutComment(aULID, &model.Comment{ULID: model.NewULID(), Author: "github.com/alice", Body: "Looks good.\nShip it.\n"})
	})

	out := te.ok("show", a.ID)
	// The title is the first field; the id, the ULID and the store follow.
	if want := "  title       Write the spec\n  id          " + a.ID + "\n  ulid        " + a.ULID + "\n  store       me\n  workspace   me (ME, "; !strings.HasPrefix(out, want) {
		t.Errorf("show does not start with %q:\n%s", want, out)
	}
	for _, want := range []string{
		"workspace   me (ME, ",
		"parent      " + label(p) + " Project",
		"blocked_by  " + label(other) + " Other",
		"due         2026-11-01",
		"labels      ui",
		"Related:",
		"Blocks      " + label(waiting) + " Waiting",
		"The body.",
		"Comments:",
		"github.com/alice",
		"    Looks good.\n    Ship it.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("show does not contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Problems:") || strings.Contains(out, "deleted ") || strings.Contains(out, "former_ids") || strings.Contains(out, a.ID+" ("+a.ULID+")  ") {
		t.Errorf("show has sections it should not:\n%s", out)
	}

	var got struct {
		usecase.TaskView
		Body     string `json:"body"`
		Comments []struct {
			Author, Body string
		} `json:"comments"`
		Reverse map[string][]usecase.Ref `json:"reverse"`
	}
	if err := json.Unmarshal([]byte(te.ok("show", a.ID, "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Title != "Write the spec" || got.Body != "The body.\n" || len(got.Comments) != 1 || len(got.Reverse["blocks"]) != 1 || got.Reverse["blocks"][0] != (usecase.Ref{ULID: waiting.ULID, ID: waiting.ID, Title: "Waiting"}) {
		t.Errorf("show --json = %+v", got)
	}
}

// 直列: editWith が t.Setenv（EDITOR と HH_TEST_EDITOR_SCRIPT）を使う
func TestShowTitleChecklistAndProgressAreFields(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("Project", "--kind", "project")
	a := te.addJSON("Alpha", "--parent", p.ID, "--status", "done")
	b := te.addJSON("Beta", "--parent", p.ID)
	te.editWith(te.raw(a.ULID) + "- [x] one\n- [ ] two\n- [ ] three\n")
	te.ok("edit", a.ID)

	// a project: the progress of its tasks (Alpha is done, Beta is not), no checklist
	out := te.ok("show", p.ID)
	if want := "  title       Project\n  id          " + p.ID + "\n"; !strings.HasPrefix(out, want) {
		t.Errorf("show of the project does not start with %q:\n%s", want, out)
	}
	if want := "  status      todo\n  progress    1/2\n  parent      -\n"; !strings.Contains(out, want) {
		t.Errorf("show of the project lacks %q:\n%s", want, out)
	}
	if strings.Contains(out, "checklist") {
		t.Errorf("a project without a checklist has a checklist line:\n%s", out)
	}

	// a task with a checklist in the body: the checklist, no progress
	out = te.ok("show", a.ID)
	if want := "  title       Alpha\n"; !strings.HasPrefix(out, want) {
		t.Errorf("show of the task does not start with %q:\n%s", want, out)
	}
	if want := "  status      done\n  checklist   ☑ 1/3\n  parent      "; !strings.Contains(out, want) {
		t.Errorf("show of the task lacks %q:\n%s", want, out)
	}
	if strings.Contains(out, "progress") {
		t.Errorf("a task of the work role has a progress line:\n%s", out)
	}

	// neither: no line for either
	out = te.ok("show", b.ID)
	if strings.Contains(out, "checklist") || strings.Contains(out, "progress") {
		t.Errorf("a task with neither has a checklist or progress line:\n%s", out)
	}

	// the heading line is gone: the output no longer starts with the title alone
	if strings.HasPrefix(out, "Beta\n") {
		t.Errorf("show still starts with a heading:\n%s", out)
	}
}

func TestShowErrors(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	if code, _, stderr := te.run("show", "zzzzzzzz"); code != 1 || !strings.Contains(stderr, "not found") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if code, _, _ := te.run("show"); code != 2 {
		t.Errorf("show with no id: exit %d, want 2", code)
	}
}

func TestShowBroken(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	x := te.addJSON("X")
	dir := te.storeDir("me")
	rel := model.TaskPath(mustULID(t, x.ULID))
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte("not a task\x00\n: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add(rel); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Commit("break a task", &git.CommitOptions{Author: &object.Signature{Name: "T", Email: "t@example.com", When: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	out := te.ok("show", x.ULID)
	if !strings.HasPrefix(out, "  title       -\n") {
		t.Errorf("show of a broken task does not start with a title of -:\n%s", out)
	}
	if !strings.Contains(out, "\nbroken: ") {
		t.Errorf("show of a broken task has no broken line:\n%s", out)
	}
	var got struct {
		Broken   string        `json:"broken"`
		Body     string        `json:"body"`
		Comments []interface{} `json:"comments"`
	}
	if err := json.Unmarshal([]byte(te.ok("show", x.ULID, "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Broken == "" || got.Body != "" || len(got.Comments) != 0 {
		t.Errorf("show --json of a broken task = %+v", got)
	}
}

func TestShowDeleted(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	x := te.addJSON("X")
	ulid := mustULID(t, x.ULID)
	te.uc().MustEdit("me", "rm", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		tk, err := tx.Task(ulid)
		if err != nil {
			return err
		}
		now := time.Now()
		tk.DeletedAt = &now
		return tx.PutTask(tk)
	})
	if out := te.ok("show", x.ID); !strings.Contains(out, "  deleted     ") {
		t.Errorf("show of a deleted task has no deleted line:\n%s", out)
	}
}

// show lists the relations a task has by their names, and the tasks that
// point at it under the name of the other direction, with its display name.
func TestShowRelations(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	other := te.addJSON("Other")
	a := te.addJSON("A", "-p", "causes+="+other.ID)
	b := te.addJSON("B", "-p", "relates+="+other.ID)
	out := te.ok("show", a.ID)
	if want := "  causes      " + label(other) + " Other"; !strings.Contains(out, want) {
		t.Errorf("show of A does not contain %q:\n%s", want, out)
	}
	if strings.Contains(out, "  relates ") || strings.Contains(out, "  clones ") {
		t.Errorf("show of A has a relation line for an empty relation:\n%s", out)
	}
	out = te.ok("show", other.ID)
	for _, want := range []string{"  Caused by   " + label(a) + " A", "  Related by  " + label(b) + " B"} {
		if !strings.Contains(out, want) {
			t.Errorf("show of Other does not contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Blocks ") || strings.Contains(out, "Duplicated by") {
		t.Errorf("show of Other has a reverse line with no value:\n%s", out)
	}
	if want := "  blocked_by  -"; !strings.Contains(out, want) {
		t.Errorf("show of Other lacks the blocked_by line %q:\n%s", want, out)
	}
}

// A display name wider than the column (Duplicated by is 13 columns, the
// column 12) is followed by one space.
func TestShowWideReverseLabel(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	other := te.addJSON("Other")
	a := te.addJSON("A", "-p", "duplicates+="+other.ID)
	out := te.ok("show", other.ID)
	if want := "  Duplicated by " + label(a) + " A"; !strings.Contains(out, want) {
		t.Errorf("show does not contain %q:\n%s", want, out)
	}
	if want := "  ulid        " + other.ULID; !strings.Contains(out, want) {
		t.Errorf("show does not contain %q:\n%s", want, out)
	}
}

func TestDisplayWidth(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		s    string
		want int
	}{
		{"", 0},
		{"abc", 3},
		{"関連", 4},
		{"ＡＢ", 4},
		{"a関b", 4},
		{"ｶﾅ", 2},
	} {
		if got := displayWidth(tc.s); got != tc.want {
			t.Errorf("displayWidth(%q) = %d, want %d", tc.s, got, tc.want)
		}
	}
}

func mustULID(t *testing.T, s string) model.ULID {
	t.Helper()
	ulid, err := model.ParseULID(s)
	if err != nil {
		t.Fatal(err)
	}
	return ulid
}

// A task that moved to another store is named by where it went, with exit 1.
func TestShowAMovedTask(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.ok("store", "init", "org-a", "--prefix", "API")
	te.ok("store", "init", "org-b", "--prefix", "WEB")
	moved := te.addJSON("--store", "org-a", "Moving")
	ulid, _ := model.ParseULID(moved.ULID)
	if err := te.uc().EditTask("org-b", "move-in", ulid, usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		wss, err := tx.Workspaces()
		if err != nil {
			return err
		}
		if len(wss) == 0 {
			return errors.New("the store has no workspace")
		}
		return tx.PutTask(&model.Task{ULID: ulid, Title: "Moving", Kind: "project", Status: "todo", Workspace: wss[0].ULID, FormerIDs: []string{moved.ID}})
	}); err != nil {
		t.Fatal(err)
	}
	fwd := "---\nulid: " + moved.ULID + "\nmoved_to: " + te.uc().StoreMeta("org-b").ULID.String() + "\nmoved_at: 2026-10-31T10:00:00Z\n---\n"
	te.uc().MustEdit("org-a", "move-out", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		return tx.PutFile(model.TaskPath(ulid), []byte(fwd))
	})

	code, out, stderr := te.run("show", "org-a:"+moved.ID)
	if code != 1 || out != "" || !strings.Contains(stderr, moved.ID+" moved to org-b (org-b:WEB-1)") {
		t.Errorf("show org-a:%s: exit %d, stdout %q, stderr %q; want exit 1 and the move named", moved.ID, code, out, stderr)
	}
}

func TestShowPrintsLinks(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	te.ok("link", a.ID, "--pr", "org-a/web#1")
	if out := te.ok("show", a.ID); !strings.Contains(out, "Links:\n  pull request  org-a/web#1  not read yet\n") {
		t.Errorf("show:\n%s", out)
	}
	if out := te.ok("show", a.ID, "--json"); !strings.Contains(out, `"label": "org-a/web#1"`) {
		t.Errorf("json:\n%s", out)
	}
}

func TestLinkLine(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   usecase.LinkView
		want string
	}{
		{"pull request not read", usecase.LinkView{Type: "github_pr", Label: "o/r#1"}, "pull request  o/r#1  not read yet"},
		{"pull request with checks", usecase.LinkView{Type: "github_pr", Label: "o/r#1", State: "open", Review: "approved", Checks: "success"}, "pull request  o/r#1  open  review: approved  checks: success"},
		{"pull request without checks", usecase.LinkView{Type: "github_pr", Label: "o/r#1", State: "merged", Review: "none"}, "pull request  o/r#1  merged  review: none"},
		{"issue", usecase.LinkView{Type: "github_issue", Label: "o/r#2", State: "closed", Sync: "push"}, "issue  o/r#2  closed  sync: push"},
		{"issue not read", usecase.LinkView{Type: "github_issue", Label: "o/r#2", Sync: "link"}, "issue  o/r#2  not read yet"},
	} {
		if got := linkLine(c.in); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
