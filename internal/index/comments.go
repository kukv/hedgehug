package index

import (
	"path"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/model"
)

// CommentEntry is what the index keeps of one comment.
type CommentEntry struct {
	ULID      model.ULID
	Author    string
	Created   time.Time
	Updated   time.Time
	DeletedAt *time.Time
}

// WorklogEntry is what the index keeps of one worklog entry: the same as of
// a comment.
type WorklogEntry CommentEntry

// Comments returns the comments of task in ULID order, deleted ones
// included.
func (ix *Index) Comments(task model.ULID) []CommentEntry {
	if e, ok := ix.tasks[task]; ok {
		return slices.Clone(e.Comments)
	}
	return nil
}

// Worklogs returns the worklog entries of task in ULID order, deleted ones
// included.
func (ix *Index) Worklogs(task model.ULID) []WorklogEntry {
	if e, ok := ix.tasks[task]; ok {
		return slices.Clone(e.Worklogs)
	}
	return nil
}

// notes are the comment and worklog files of some tasks, by task.
type notes struct {
	comments map[model.ULID][]*model.Comment
	worklogs map[model.ULID][]*model.Worklog
}

// noteDirs returns the directories of the comment and the worklog files of
// task, as prefixes.
func noteDirs(task model.ULID) []string {
	return []string{
		path.Dir(model.CommentPath(task, model.ULID{})) + "/",
		path.Dir(model.WorklogPath(task, model.ULID{})) + "/",
	}
}

// readNotes reads the comment and worklog files under prefixes in tree of
// the tasks want takes. A file that does not decode, or whose ULID is not
// the one of its path, is left out.
func (ix *Index) readNotes(tree plumbing.Hash, prefixes []string, want func(model.ULID) bool) (notes, error) {
	n := notes{comments: map[model.ULID][]*model.Comment{}, worklogs: map[model.ULID][]*model.Worklog{}}
	for _, prefix := range prefixes {
		if err := ix.readNotesUnder(tree, prefix, want, n); err != nil {
			return n, err
		}
	}
	return n, nil
}

// readNotesUnder reads into n the files under one prefix, as readNotes does.
func (ix *Index) readNotesUnder(tree plumbing.Hash, prefix string, want func(model.ULID) bool, n notes) error {
	if listHook != nil {
		listHook(prefix)
	}
	for p, err := range ix.repo.Files(tree, prefix) {
		if err != nil {
			return err
		}
		if task, ulid, err := model.ParseCommentPath(p); err == nil && want(task) {
			b, err := ix.read(tree, p)
			if err != nil {
				return err
			}
			if c, err := model.DecodeComment(b); err == nil && c.ULID == ulid {
				n.comments[task] = append(n.comments[task], c)
			}
		} else if task, ulid, err := model.ParseWorklogPath(p); err == nil && want(task) {
			b, err := ix.read(tree, p)
			if err != nil {
				return err
			}
			if w, err := model.DecodeWorklog(b); err == nil && w.ULID == ulid {
				n.worklogs[task] = append(n.worklogs[task], w)
			}
		}
	}
	return nil
}

// noteTask returns the task of the comment or worklog file at p.
func noteTask(p string) (model.ULID, bool) {
	if task, _, err := model.ParseCommentPath(p); err == nil {
		return task, true
	}
	if task, _, err := model.ParseWorklogPath(p); err == nil {
		return task, true
	}
	return model.ULID{}, false
}

// attach puts the comments and worklog entries of n into e, and adds the
// mentions and refs of the comments that are not deleted to those of the
// body. e must be newly made, its Mentions and Refs those of the body.
func (n notes) attach(e *Entry) {
	cs := slices.Clone(n.comments[e.ULID])
	slices.SortFunc(cs, func(a, b *model.Comment) int { return strings.Compare(a.ULID.String(), b.ULID.String()) })
	e.Comments = nil
	for _, c := range cs {
		e.Comments = append(e.Comments, CommentEntry{ULID: c.ULID, Author: c.Author, Created: c.Created, Updated: c.Updated, DeletedAt: c.DeletedAt})
		if c.DeletedAt == nil {
			e.Mentions = appendNew(e.Mentions, Mentions(c.Body))
			e.Refs = appendNew(e.Refs, TaskRefs(c.Body))
		}
	}
	ws := slices.Clone(n.worklogs[e.ULID])
	slices.SortFunc(ws, func(a, b *model.Worklog) int { return strings.Compare(a.ULID.String(), b.ULID.String()) })
	e.Worklogs = nil
	for _, w := range ws {
		e.Worklogs = append(e.Worklogs, WorklogEntry{ULID: w.ULID, Author: w.Author, Created: w.Created, Updated: w.Updated, DeletedAt: w.DeletedAt})
	}
}

// appendNew appends the values of add that out does not hold yet.
func appendNew(out, add []string) []string {
	for _, v := range add {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}
