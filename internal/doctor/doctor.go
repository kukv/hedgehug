package doctor

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/merge"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// lookPath finds hedgehug on PATH; tests replace it.
var lookPath = exec.LookPath

// fileRoots are the directories of a store whose files doctor reads.
var fileRoots = []string{"tasks", "comments", "worklogs", "sprints", "releases", "templates", "workspaces"}

// indexCodes are the problems of a task that come from the shape of the
// store, not from the schema.
var indexCodes = []string{
	schema.CodeMissingRef, schema.CodeCycle, index.CodeBroken, index.CodeParentDeleted,
	index.CodeNoID, index.CodeDuplicateID, index.CodeIDPrefix, store.CodeNoWorkspace, store.CodeWorkspaceMismatch,
}

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
	msg, err := checkNumbers(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		msg = err.Error()
	}
	if msg != "" {
		out = append(out, Finding{
			Check: "file", Where: model.NumbersPath, Msg: msg,
			Hint: "fix the file by hand and commit, or restore it with `git restore " + model.NumbersPath + "`",
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
	out = append(out, checkMergeDriver(dir, repo)...)
	out = append(out, checkInbox(dir)...)
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

// checkMergeDriver finds a store whose .gitattributes send files to the
// hedgehug merge driver while .git/config does not register it, or while
// hedgehug cannot be found on PATH for git to run it.
func checkMergeDriver(dir string, repo *gitrepo.Repo) []Finding {
	data, err := os.ReadFile(filepath.Join(dir, store.AttributesPath)) //nolint:gosec // G304: the path is built from the store root and a fixed name
	if err != nil || !slices.ContainsFunc(strings.Split(string(data), "\n"), func(l string) bool {
		return !strings.HasPrefix(strings.TrimSpace(l), "#") && slices.Contains(strings.Fields(l), "merge=hedgehug")
	}) {
		return nil
	}
	where := store.AttributesPath
	registered, err := repo.MergeDriverRegistered()
	if err != nil {
		return []Finding{{Check: "merge_driver", Where: where, Msg: "cannot read .git/config: " + err.Error()}}
	}
	if !registered {
		return []Finding{{
			Check: "merge_driver", Where: where,
			Msg:  "marks files merge=hedgehug, but the merge driver is not registered in .git/config (expected name = Hedgehug field merge and driver = " + gitrepo.MergeDriver + ")",
			Hint: "run hedgehug sync once",
		}}
	}
	if _, err := lookPath("hedgehug"); err != nil {
		return []Finding{{
			Check: "merge_driver", Where: where,
			Msg:  "hedgehug is not on PATH, so git cannot run the merge driver",
			Hint: "put hedgehug on PATH",
		}}
	}
	return nil
}

// checkInbox finds the records of the merge driver that the inbox has not
// taken in. A record that does not decode is never taken in, so it is told
// apart: the advice for it is to remove the file.
func checkInbox(dir string) []Finding {
	root := filepath.Join(dir, filepath.FromSlash(inbox.RecordsPath))
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return []Finding{{Check: "inbox", Where: inbox.RecordsPath, Msg: err.Error()}}
	}
	var out []Finding
	waiting := 0
	for _, e := range entries { // the inbox leaves other names alone
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), ".json") || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		rel := inbox.RecordsPath + "/" + e.Name()
		data, err := os.ReadFile(filepath.Join(root, e.Name())) //nolint:gosec // G304: the path is built from the store root and a walked name
		if err == nil {
			err = inbox.CheckRecord(data)
		}
		if err != nil {
			out = append(out, Finding{
				Check: "inbox", Where: rel, Msg: "does not decode: " + err.Error() + "; the inbox leaves it where it is",
				Hint: "remove the file",
			})
			continue
		}
		waiting++
	}
	if waiting > 0 {
		out = append(out, Finding{
			Check: "inbox", Where: inbox.RecordsPath,
			Msg:  fmt.Sprintf("%d merge driver records are not in the inbox yet", waiting),
			Hint: "run hedgehug inbox",
		})
	}
	return out
}

// Fix repairs the files of HEAD as a sync does, without fetching or
// pushing, and returns what it fixed. Duplicate prefixes and names the
// repair only tells of are not in the result: they stay findings.
func Fix(s *store.Store, opt sync.Options) ([]merge.Repaired, error) {
	all, err := sync.RepairHead(s, opt)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(x merge.Repaired) bool {
		return x.Kind == "prefix_duplicate" || x.Kind == "name_duplicate"
	}), nil
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

// checkNumbers returns what is wrong with numbers.yaml, or "" if nothing
// is: that it does not decode, or an entry that is not an id prefix with a
// number of 1 or more. The error is for a file that cannot be read.
func checkNumbers(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(model.NumbersPath))) //nolint:gosec // G304: the path is built from the store root and a fixed name
	if err != nil {
		return "", err
	}
	n, err := model.DecodeNumbers(data)
	if err != nil {
		return "does not decode: " + err.Error(), nil
	}
	for _, p := range slices.Sorted(maps.Keys(n)) {
		if err := model.ValidIDPrefix(p); err != nil {
			return fmt.Sprintf("has an invalid entry: %s: %v", p, err), nil
		}
		if n[p] < 1 {
			return fmt.Sprintf("has an invalid entry: %s: the number %d is less than 1", p, n[p]), nil
		}
	}
	return "", nil
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
	fs = append(fs, checkWorkspaces(ix)...)
	for _, e := range ix.Tasks() {
		for _, p := range ix.Problems(e.ULID) {
			if !slices.Contains(indexCodes, p.Code) {
				continue
			}
			msg := p.Msg
			if p.Field != "" {
				msg = p.Field + ": " + msg
			}
			fs = append(fs, Finding{Check: "task", Where: taskWhere(e), Msg: msg, Hint: fixHint(p)})
		}
	}
	return fs
}

// fixHint is the advice for a problem that doctor --fix repairs, or "".
func fixHint(p index.Problem) string {
	const fix = "hedgehug doctor --fix fixes it"
	switch {
	case p.Code == index.CodeDuplicateID:
		return fix + "; hedgehug sync also renumbers it, keeping the task that is pushed"
	case p.Code == index.CodeIDPrefix, p.Code == store.CodeWorkspaceMismatch:
		return fix
	case p.Field == "parent" && (p.Code == schema.CodeCycle || p.Code == schema.CodeMissingRef || p.Code == index.CodeParentDeleted):
		return fix
	}
	return ""
}

// checkWorkspaces finds workspaces that share an id prefix or a name. Each
// of them gets a finding that names the other.
func checkWorkspaces(ix *index.Index) []Finding {
	var out []Finding
	ws := ix.Workspaces()
	for i, a := range ws {
		for _, b := range ws[:i] {
			var what string
			switch {
			case a.IDPrefix == b.IDPrefix:
				what = "id prefix " + a.IDPrefix
			case strings.EqualFold(a.Name, b.Name):
				what = "name " + a.Name
			default:
				continue
			}
			for _, pair := range [][2]*model.Workspace{{a, b}, {b, a}} {
				out = append(out, Finding{
					Check: "workspace", Where: model.WorkspacePath(pair[0].ULID),
					Msg:  fmt.Sprintf("has the same %s as workspace %s (%s)", what, pair[1].Name, pair[1].ULID),
					Hint: "change one with hedgehug workspace prefix (or rename)",
				})
			}
		}
	}
	return out
}

// taskWhere names the task e in a finding: its id, ULID and title, or the
// ULID and title when it has no id (a file that does not decode).
func taskWhere(e *index.Entry) string {
	if e.ID == "" {
		return strings.TrimSpace(e.ULID.String() + " " + e.Title)
	}
	return strings.TrimSpace(e.ID + " (" + e.ULID.String() + ") " + e.Title)
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
		ulid, err := model.ParseTaskPath(rel)
		if err != nil {
			return bad("the name does not follow the rule of task files")
		}
		t, err := model.DecodeTask(data)
		if err != nil {
			return bad("does not decode: %v", err)
		}
		if t.ULID != ulid {
			return bad("the ulid in the file is %s, which differs from the ulid in the path", t.ULID)
		}
	case "comments", "worklogs":
		var task, child, got model.ULID
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
			got = c.ULID
		} else {
			w, derr := model.DecodeWorklog(data)
			if derr != nil {
				return bad("does not decode: %v", derr)
			}
			got = w.ULID
		}
		if got != child {
			return bad("the ulid in the file is %s, which differs from the ulid in the path", got)
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(model.TaskPath(task)))); errors.Is(err, fs.ErrNotExist) {
			return bad("there is no task file for task %s", task)
		} else if err != nil {
			return "", err
		}
	case "sprints", "releases":
		name := path.Base(rel)
		ulid, perr := model.ParseULID(strings.TrimSuffix(name, ".md"))
		build := model.SprintPath
		if root == "releases" {
			build = model.ReleasePath
		}
		if perr != nil || build(ulid) != rel {
			return bad("the name does not follow the rule of %s files", root)
		}
		var got model.ULID
		if root == "sprints" {
			sp, derr := model.DecodeSprint(data)
			if derr != nil {
				return bad("does not decode: %v", derr)
			}
			got = sp.ULID
		} else {
			r, derr := model.DecodeRelease(data)
			if derr != nil {
				return bad("does not decode: %v", derr)
			}
			got = r.ULID
		}
		if got != ulid {
			return bad("the ulid in the file is %s, which differs from the ulid in the path", got)
		}
	case "workspaces":
		ulid, err := model.ParseWorkspacePath(rel)
		if err != nil {
			return bad("the name does not follow the rule of workspace files")
		}
		w, err := model.DecodeWorkspace(data)
		if err != nil {
			return bad("does not decode: %v", err)
		}
		if w.ULID != ulid {
			return bad("the ulid in the file is %s, which differs from the ulid in the path", w.ULID)
		}
		switch {
		case w.Name == "":
			return bad("name is missing")
		case w.IDPrefix == "":
			return bad("id_prefix is missing")
		case w.PrefixSet.IsZero():
			return bad("prefix_set is missing")
		}
		if err := model.ValidWorkspaceName(w.Name); err != nil {
			return bad("%v", err)
		}
		if err := model.ValidIDPrefix(w.IDPrefix); err != nil {
			return bad("%v", err)
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
