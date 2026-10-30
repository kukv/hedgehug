package model

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// StoreMetaPath is where a store keeps its metadata.
const StoreMetaPath = ".hedgehug/store.yaml"

// shard splits files into 32×32 directories by the last and the second to
// last character of the ULID. The first characters are a timestamp and
// cluster, so the random tail is used instead.
func shard(id ULID) string {
	s := id.String()
	return s[25:] + "/" + s[24:25]
}

// TaskPath returns the path of a task file, relative to the store root.
func TaskPath(id ULID) string { return "tasks/" + shard(id) + "/" + id.String() + ".md" }

// CommentPath returns the path of a comment on task.
func CommentPath(task, comment ULID) string {
	return "comments/" + shard(task) + "/" + task.String() + "/" + comment.String() + ".md"
}

// WorklogPath returns the path of a worklog entry on task.
func WorklogPath(task, log ULID) string {
	return "worklogs/" + shard(task) + "/" + task.String() + "/" + log.String() + ".md"
}

func SprintPath(id ULID) string    { return "sprints/" + id.String() + ".md" }
func ReleasePath(id ULID) string   { return "releases/" + id.String() + ".md" }
func WorkspacePath(id ULID) string { return "workspaces/" + id.String() + ".md" }

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
	id, err := parseFileULID(parts[3])
	if err != nil || TaskPath(id) != p {
		return ULID{}, fmt.Errorf("%q is not a task path", p)
	}
	return id, nil
}

// ParseWorkspacePath returns the ULID of the workspace file at p.
func ParseWorkspacePath(p string) (ULID, error) {
	parts := strings.Split(p, "/")
	if len(parts) != 2 || parts[0] != "workspaces" {
		return ULID{}, fmt.Errorf("%q is not a workspace path", p)
	}
	id, err := parseFileULID(parts[1])
	if err != nil || WorkspacePath(id) != p {
		return ULID{}, fmt.Errorf("%q is not a workspace path", p)
	}
	return id, nil
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
