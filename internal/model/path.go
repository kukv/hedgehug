package model

import (
	"fmt"
	"strings"
)

// StoreMetaPath is where a store keeps its metadata.
const StoreMetaPath = ".hedgehug/store.yaml"

// shard splits files into 32×32 directories by the last and the second to
// last character of the ULID. The first characters are a timestamp and
// cluster, so the random tail is used instead.
func shard(id ID) string {
	s := id.String()
	return s[25:] + "/" + s[24:25]
}

// TaskPath returns the path of a task file, relative to the store root.
func TaskPath(id ID) string { return "tasks/" + shard(id) + "/" + id.String() + ".md" }

// CommentPath returns the path of a comment on task.
func CommentPath(task, comment ID) string {
	return "comments/" + shard(task) + "/" + task.String() + "/" + comment.String() + ".md"
}

// WorklogPath returns the path of a worklog entry on task.
func WorklogPath(task, log ID) string {
	return "worklogs/" + shard(task) + "/" + task.String() + "/" + log.String() + ".md"
}

func SprintPath(id ID) string  { return "sprints/" + id.String() + ".md" }
func ReleasePath(id ID) string { return "releases/" + id.String() + ".md" }

// TemplatePath returns the path of the template called name.
func TemplatePath(name string) string { return "templates/" + name + ".md" }

// ParseTaskPath returns the ID of the task file at p.
func ParseTaskPath(p string) (ID, error) {
	parts := strings.Split(p, "/")
	if len(parts) != 4 || parts[0] != "tasks" {
		return ID{}, fmt.Errorf("%q is not a task path", p)
	}
	id, err := parseFileID(parts[3])
	if err != nil || TaskPath(id) != p {
		return ID{}, fmt.Errorf("%q is not a task path", p)
	}
	return id, nil
}

// ParseCommentPath returns the task and comment IDs of the comment file at p.
func ParseCommentPath(p string) (task, comment ID, err error) {
	return parseChildPath(p, "comments", CommentPath)
}

// ParseWorklogPath returns the task and entry IDs of the worklog file at p.
func ParseWorklogPath(p string) (task, log ID, err error) {
	return parseChildPath(p, "worklogs", WorklogPath)
}

func parseChildPath(p, root string, build func(ID, ID) string) (ID, ID, error) {
	parts := strings.Split(p, "/")
	if len(parts) != 5 || parts[0] != root {
		return ID{}, ID{}, fmt.Errorf("%q is not a %s path", p, root)
	}
	task, err1 := ParseID(parts[3])
	child, err2 := parseFileID(parts[4])
	if err1 != nil || err2 != nil || build(task, child) != p {
		return ID{}, ID{}, fmt.Errorf("%q is not a %s path", p, root)
	}
	return task, child, nil
}

func parseFileID(name string) (ID, error) {
	s, ok := strings.CutSuffix(name, ".md")
	if !ok {
		return ID{}, fmt.Errorf("%q is not a .md file", name)
	}
	return ParseID(s)
}
