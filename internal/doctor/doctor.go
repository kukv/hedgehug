package doctor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// fileRoots are the directories of a store whose files doctor reads.
var fileRoots = []string{"tasks", "comments", "worklogs", "sprints", "releases", "templates"}

// indexCodes are the problems of a task that come from the shape of the
// store, not from the schema.
var indexCodes = []string{schema.CodeMissingRef, schema.CodeCycle, index.CodeBroken}

// Run checks the store in dir without opening it with store.Open, which
// refuses a store whose schema.yaml is broken. The findings are sorted. The
// error is for files that cannot be read at all.
func Run(dir string) ([]Finding, error) {
	var out []Finding

	// 1. store.yaml and schema.yaml.
	metaOK := true
	if err := readCheck(dir, model.StoreMetaPath, model.DecodeStoreMeta); err != nil {
		metaOK = false
		out = append(out, Finding{
			Check: "file", Where: model.StoreMetaPath,
			Msg:  err.Error() + "; the tasks are not checked against the schema",
			Hint: "restore it with `git restore " + model.StoreMetaPath + "`, or fix it by hand and commit",
		})
	}
	schemaOK := false
	if data, err := os.ReadFile(filepath.Join(dir, schema.Path)); err != nil { //nolint:gosec // G304: the path is built from the store root and a fixed or walked name
		out = append(out, Finding{Check: "schema", Where: schema.Path, Msg: err.Error(), Hint: brokenHint})
	} else {
		sc, fs := SchemaFile(data)
		out = append(out, fs...)
		schemaOK = sc != nil
	}
	if !schemaOK {
		out = append(out, Finding{
			Check: "schema", Where: schema.Path,
			Msg: "the tasks are not checked against the schema, because schema.yaml is not readable; that check is skipped",
		})
	}

	// 2. the files of the work tree.
	fileFs, err := checkFiles(dir)
	if err != nil {
		return nil, err
	}
	out = append(out, fileFs...)

	// 3 and 4. the work tree and the git objects.
	repo, err := gitrepo.Open(dir)
	if err != nil {
		out = append(out, Finding{Check: "git", Where: ".git", Msg: err.Error()})
		Sort(out)
		return out, nil
	}
	st, err := repo.Status()
	if err != nil {
		out = append(out, Finding{Check: "git", Where: ".git", Msg: "cannot read the status: " + err.Error()})
	}
	for _, f := range st {
		if f.Staged {
			out = append(out, Finding{
				Check: "worktree", Where: f.Path,
				Msg:  "has a staged change; it would go into the commit of the next operation",
				Hint: "git restore --staged " + f.Path,
			})
		}
		if f.Unstaged {
			out = append(out, Finding{
				Check: "worktree", Where: f.Path,
				Msg:  "has a change that is not committed",
				Hint: "git restore " + f.Path + ", or remove the file if it is new",
			})
		}
	}
	if err := repo.Verify(); err != nil {
		out = append(out, Finding{Check: "git", Where: ".git", Msg: "a git object cannot be read: " + err.Error()})
	}

	// 5. the schema against the tasks.
	if metaOK && schemaOK {
		out = append(out, checkStore(dir)...)
	}
	Sort(out)
	return out, nil
}

// readCheck reads the file at rel and decodes it.
func readCheck[T any](dir, rel string, decode func([]byte) (T, error)) error {
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel))) //nolint:gosec // G304: the path is built from the store root and a fixed or walked name
	if err != nil {
		return err
	}
	if _, err := decode(data); err != nil {
		return fmt.Errorf("does not decode: %w", err)
	}
	return nil
}

// checkStore opens the store and the index and finds what step 5 of Run
// finds. A failure to open becomes a finding.
func checkStore(dir string) []Finding {
	s, err := store.Open(dir)
	if err != nil {
		return []Finding{{Check: "git", Where: dir, Msg: "cannot open the store: " + err.Error()}}
	}
	ix, err := index.Open(s)
	if err != nil {
		return []Finding{{
			Check: "git", Where: "HEAD", Msg: "cannot build the index: " + err.Error(),
			Hint: "the schema.yaml that is committed may be broken; restore a good one and commit it",
		}}
	}
	fs, _ := Schema(s, ix) // the error is for checks that read files; it is nil
	// schema.yaml was checked in step 1.
	fs = slices.DeleteFunc(fs, func(f Finding) bool { return f.Check == "schema" })
	for _, e := range ix.Tasks() {
		for _, p := range ix.Problems(e.ID) {
			if !slices.Contains(indexCodes, p.Code) {
				continue
			}
			msg := p.Msg
			if p.Field != "" {
				msg = p.Field + ": " + msg
			}
			fs = append(fs, Finding{Check: "task", Where: strings.TrimSpace(ix.ShortID(e.ID) + " " + e.Title), Msg: msg})
		}
	}
	return fs
}

// checkFiles reads every file under the directories of fileRoots.
func checkFiles(dir string) ([]Finding, error) {
	var out []Finding
	for _, root := range fileRoots {
		err := filepath.WalkDir(filepath.Join(dir, root), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if p == filepath.Join(dir, root) && errors.Is(err, fs.ErrNotExist) {
					return fs.SkipAll
				}
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(dir, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			msg, err := checkFile(dir, root, rel)
			if err != nil {
				return err
			}
			if msg != "" {
				out = append(out, Finding{Check: "file", Where: rel, Msg: msg, Hint: "fix the file by hand and commit, or restore it with `git restore " + rel + "`"})
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.SkipAll) {
			return nil, err
		}
	}
	return out, nil
}

// checkFile returns what is wrong with the file at rel, or "" if nothing is.
func checkFile(dir, root, rel string) (string, error) {
	bad := func(format string, a ...any) (string, error) { return fmt.Sprintf(format, a...), nil }
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel))) //nolint:gosec // G304: the path is built from the store root and a fixed or walked name
	if err != nil {
		return "", err
	}
	switch root {
	case "tasks":
		id, err := model.ParseTaskPath(rel)
		if err != nil {
			return bad("the name does not follow the rule of task files")
		}
		t, err := model.DecodeTask(data)
		if err != nil {
			return bad("does not decode: %v", err)
		}
		if t.ID != id {
			return bad("the id in the file is %s, which differs from the id in the path", t.ID)
		}
	case "comments", "worklogs":
		var task, child, got model.ID
		if root == "comments" {
			task, child, err = model.ParseCommentPath(rel)
		} else {
			task, child, err = model.ParseWorklogPath(rel)
		}
		if err != nil {
			return bad("the name does not follow the rule of %s files", root)
		}
		if root == "comments" {
			c, derr := model.DecodeComment(data)
			if derr != nil {
				return bad("does not decode: %v", derr)
			}
			got = c.ID
		} else {
			w, derr := model.DecodeWorklog(data)
			if derr != nil {
				return bad("does not decode: %v", derr)
			}
			got = w.ID
		}
		if got != child {
			return bad("the id in the file is %s, which differs from the id in the path", got)
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(model.TaskPath(task)))); errors.Is(err, fs.ErrNotExist) {
			return bad("there is no task file for task %s", task)
		} else if err != nil {
			return "", err
		}
	case "sprints", "releases":
		name := path.Base(rel)
		id, perr := model.ParseID(strings.TrimSuffix(name, ".md"))
		build := model.SprintPath
		if root == "releases" {
			build = model.ReleasePath
		}
		if perr != nil || build(id) != rel {
			return bad("the name does not follow the rule of %s files", root)
		}
		var got model.ID
		if root == "sprints" {
			sp, derr := model.DecodeSprint(data)
			if derr != nil {
				return bad("does not decode: %v", derr)
			}
			got = sp.ID
		} else {
			r, derr := model.DecodeRelease(data)
			if derr != nil {
				return bad("does not decode: %v", derr)
			}
			got = r.ID
		}
		if got != id {
			return bad("the id in the file is %s, which differs from the id in the path", got)
		}
	case "templates":
		name, ok := strings.CutSuffix(path.Base(rel), ".md")
		if !ok || path.Dir(rel) != "templates" || model.ValidTemplateName(name) != nil {
			return bad("the name does not follow the rule of template files")
		}
		if _, err := model.DecodeTemplate(name, data); err != nil {
			return bad("does not decode: %v", err)
		}
	}
	return "", nil
}
