package integration

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// slugMax is the most bytes of a title a branch name takes.
const slugMax = 40

// StartResult is what Start did.
type StartResult struct {
	usecase.TaskResult
	Branch  string // the branch of the task
	Created bool   // Start made the branch
	Switch  bool   // the branch is there, and the developer is on another
}

// StartWrite is the answer of Start: the write, and the branch.
type StartWrite struct {
	usecase.TaskWrite
	Branch  string `json:"branch"`
	Created bool   `json:"created"`
	Switch  bool   `json:"switch"`
}

// Write is r as the answer.
func (r StartResult) Write() (StartWrite, error) {
	w, err := r.TaskResult.Write()
	if err != nil {
		return StartWrite{}, err
	}
	return StartWrite{TaskWrite: w, Branch: r.Branch, Created: r.Created, Switch: r.Switch}, nil
}

// Slug is the part of a branch name that comes from title: its runs of ASCII
// letters and digits in lower case, joined by hyphens, cut at 40 bytes.
func Slug(title string) string {
	words := strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	s := strings.Join(words, "-")
	if len(s) > slugMax {
		s = strings.TrimRight(s[:slugMax], "-")
	}
	return s
}

// branchOf is the name of a new branch for the task e.
func branchOf(e *index.Entry) string {
	if s := Slug(e.Title); s != "" {
		return "hh/" + e.ID + "-" + s
	}
	return "hh/" + e.ID
}

// ofTask reports whether the branch name is of the task e. A title that
// changed leaves the task with a branch of its old slug, and a prefix that
// changed with one of a former id. Ids compare without case, as they do when
// a task is resolved.
func ofTask(e *index.Entry) func(string) bool {
	ids := append([]string{e.ID}, e.FormerIDs...)
	return func(name string) bool {
		rest, ok := strings.CutPrefix(name, "hh/")
		if !ok {
			return false
		}
		rest = strings.ToLower(rest)
		return slices.ContainsFunc(ids, func(id string) bool {
			id = strings.ToLower(id)
			return rest == id || strings.HasPrefix(rest, id+"-")
		})
	}
}

// openCode opens the code repository at dir or above it.
func openCode(dir string) (*gitrepo.Code, error) {
	c, err := gitrepo.OpenCode(dir)
	if errors.Is(err, gitrepo.ErrNoRepository) {
		return nil, usecase.NewErr(usecase.KindInvalid, "code.no_repository",
			fmt.Errorf("%s is not in a git repository; run this in the repository of the code", dir), "dir", dir)
	}
	return c, err
}

// startBranch puts the developer on the branch of e, if that is safe: a
// branch that is there is left for them to switch to.
func startBranch(code *gitrepo.Code, e *index.Entry) (name string, created, sw bool, err error) {
	cur, on, err := code.Branch()
	if err != nil {
		return "", false, false, err
	}
	have, err := code.Branches(ofTask(e))
	if err != nil {
		return "", false, false, err
	}
	name = branchOf(e)
	switch {
	case on && slices.Contains(have, cur):
		return cur, false, false, nil
	case slices.Contains(have, name):
		return name, false, true, nil
	case len(have) > 0:
		return have[0], false, true, nil
	}
	err = code.StartBranch(name)
	if errors.Is(err, gitrepo.ErrNoCommit) {
		return "", false, false, usecase.NewErr(usecase.KindInvalid, "code.no_commit",
			errors.New("the repository has no commit to start a branch at; commit something first"))
	}
	return name, err == nil, false, err
}

// doingStatus is the status of a task with the status cur once it is started:
// cur itself when it is in the doing category already.
func doingStatus(sc *schema.Schema, cur string) (string, error) {
	if st, ok := sc.Status(cur); ok && st.Category == schema.CategoryDoing {
		return cur, nil
	}
	st, ok := sc.DefaultStatus(schema.CategoryDoing)
	if !ok {
		return "", usecase.NewErr(usecase.KindInvalid, "schema.no_default", errors.New("the schema has no default doing status"), "what", "doing_status")
	}
	return st.Name, nil
}

// Start puts the developer on the branch of the task arg names, in the code
// repository at dir, then assigns the task to them and moves it to doing, as
// one write. A write the store refuses leaves the branch.
func Start(ss *usecase.Session, arg, dir string) (StartResult, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return StartResult{}, err
	}
	if ts != ss {
		return Start(ts, arg, dir)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return StartResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return StartResult{}, err
	}
	if err := usecase.NotDeleted(ix, e); err != nil {
		return StartResult{}, err
	}
	me, err := ss.AccountID()
	if err != nil {
		return StartResult{}, err
	}
	if me == "" {
		return StartResult{}, usecase.ErrNoAccounts
	}
	status, err := doingStatus(ix.Schema(), e.Status)
	if err != nil {
		return StartResult{}, err
	}
	code, err := openCode(dir)
	if err != nil {
		return StartResult{}, err
	}
	defer func() { _ = code.Close() }()
	branch, created, sw, err := startBranch(code, e)
	if err != nil {
		return StartResult{}, err
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: "start", Task: e.ULID}, func(tx *store.Tx) error {
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		take(t, me)
		t.Status = status
		return tx.PutTask(t)
	})
	if err != nil {
		return StartResult{}, err
	}
	return StartResult{TaskResult: usecase.NewTaskResult(ix, e.ULID, hash), Branch: branch, Created: created, Switch: sw}, nil
}

// take assigns t to the account me, as hh set assignees+= does.
func take(t *model.Task, me string) {
	if slices.ContainsFunc(t.Assignees, func(a string) bool { return config.SameAccount(a, me) }) {
		return
	}
	before := slices.Clone(t.Assignees)
	t.Assignees = append(t.Assignees, me)
	usecase.WatchNewAssignees(t, before)
}
