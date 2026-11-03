package model

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"unicode"
)

// StoreMetaPath is where a store keeps its metadata.
const StoreMetaPath = ".hedgehug/store.yaml"

// AutomationsPath is where a store keeps its automations.
const AutomationsPath = "automations.yaml"

// shard splits files into 32×32 directories by the last and the second to
// last character of the ULID. The first characters are a timestamp and
// cluster, so the random tail is used instead.
func shard(ulid ULID) string {
	s := ulid.String()
	return s[25:] + "/" + s[24:25]
}

// TaskPath returns the path of a task file, relative to the store root.
func TaskPath(ulid ULID) string { return "tasks/" + shard(ulid) + "/" + ulid.String() + ".md" }

// CommentPath returns the path of a comment on task.
func CommentPath(task, comment ULID) string {
	return "comments/" + shard(task) + "/" + task.String() + "/" + comment.String() + ".md"
}

// WorklogPath returns the path of a worklog entry on task.
func WorklogPath(task, log ULID) string {
	return "worklogs/" + shard(task) + "/" + task.String() + "/" + log.String() + ".md"
}

func SprintPath(ulid ULID) string    { return "sprints/" + ulid.String() + ".md" }
func ReleasePath(ulid ULID) string   { return "releases/" + ulid.String() + ".md" }
func WorkspacePath(ulid ULID) string { return "workspaces/" + ulid.String() + ".md" }

// TemplatePath returns the path of the template called name. name must
// pass ValidTemplateName, or the path may point outside templates/.
func TemplatePath(name string) string { return "templates/" + name + ".md" }

// ValidTemplateName reports whether name can be used as a template file
// name: not empty, no path separators, no "..", no leading "." and no
// control characters.
func ValidTemplateName(name string) error {
	switch {
	case name == "":
		return errors.New("template name is empty")
	case strings.ContainsAny(name, `/\`):
		return fmt.Errorf("template name %q contains a path separator", name)
	case strings.Contains(name, ".."):
		return fmt.Errorf("template name %q contains \"..\"", name)
	case strings.HasPrefix(name, "."):
		return fmt.Errorf("template name %q starts with a dot", name)
	case strings.IndexFunc(name, unicode.IsControl) >= 0:
		return fmt.Errorf("template name %q contains a control character", name)
	}
	return nil
}

// ParseTaskPath returns the ULID of the task file at p.
func ParseTaskPath(p string) (ULID, error) {
	parts := strings.Split(p, "/")
	if len(parts) != 4 || parts[0] != "tasks" {
		return ULID{}, fmt.Errorf("%q is not a task path", p)
	}
	ulid, err := parseFileULID(parts[3])
	if err != nil || TaskPath(ulid) != p {
		return ULID{}, fmt.Errorf("%q is not a task path", p)
	}
	return ulid, nil
}

// ParseWorkspacePath returns the ULID of the workspace file at p.
func ParseWorkspacePath(p string) (ULID, error) {
	parts := strings.Split(p, "/")
	if len(parts) != 2 || parts[0] != "workspaces" {
		return ULID{}, fmt.Errorf("%q is not a workspace path", p)
	}
	ulid, err := parseFileULID(parts[1])
	if err != nil || WorkspacePath(ulid) != p {
		return ULID{}, fmt.Errorf("%q is not a workspace path", p)
	}
	return ulid, nil
}

// ParseCommentPath returns the task and comment ULIDs of the comment file at p.
func ParseCommentPath(p string) (task, comment ULID, err error) {
	return parseChildPath(p, "comments", CommentPath)
}

// ParseWorklogPath returns the task and entry ULIDs of the worklog file at p.
func ParseWorklogPath(p string) (task, log ULID, err error) {
	return parseChildPath(p, "worklogs", WorklogPath)
}

func parseChildPath(p, root string, build func(ULID, ULID) string) (ULID, ULID, error) {
	parts := strings.Split(p, "/")
	if len(parts) != 5 || parts[0] != root {
		return ULID{}, ULID{}, fmt.Errorf("%q is not a %s path", p, root)
	}
	task, err1 := ParseULID(parts[3])
	child, err2 := parseFileULID(parts[4])
	if err1 != nil || err2 != nil || build(task, child) != p {
		return ULID{}, ULID{}, fmt.Errorf("%q is not a %s path", p, root)
	}
	return task, child, nil
}

func parseFileULID(name string) (ULID, error) {
	s, ok := strings.CutSuffix(name, ".md")
	if !ok {
		return ULID{}, fmt.Errorf("%q is not a .md file", name)
	}
	return ParseULID(s)
}

// ViewsDir holds the saved views of a store, one YAML file each.
const ViewsDir = "views"

// ViewPath is the file of the saved view called name.
func ViewPath(name string) string { return ViewsDir + "/" + name + ".yaml" }

// IsViewPath reports whether p is a file directly under views/ with the
// .yaml extension, and gives the name it stands for. The name is not
// checked; see view.ValidName.
func IsViewPath(p string) (name string, ok bool) {
	if path.Dir(p) != ViewsDir || path.Ext(p) != ".yaml" {
		return "", false
	}
	return strings.TrimSuffix(path.Base(p), ".yaml"), true
}
