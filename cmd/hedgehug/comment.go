package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func (a *app) commentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment <id> [text...]",
		Short: "Add a comment to a task, or edit or delete one",
		Long:  "Add a comment to a task. Without text, the body is written in $EDITOR.",
		Args:  minArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.commentAdd(args[0], args[1:]) },
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "edit <comment-id>",
			Short: "Edit the body of a comment in $EDITOR",
			Args:  exactArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return a.commentEdit(args[0]) },
		},
		&cobra.Command{
			Use:   "rm <comment-id>",
			Short: "Delete a comment",
			Args:  exactArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return a.commentRm(args[0]) },
		},
	)
	return cmd
}

// commentIndex lists the comments of a store, deleted ones included, to
// resolve their IDs and to give the short ones.
type commentIndex struct {
	refs  []store.CommentRef
	short map[model.ID]int
	// title names the task of a comment in an ambiguity error.
	title func(model.ID) string
}

func loadComments(s *store.Store) (*commentIndex, error) {
	refs, err := s.CommentIDs()
	if err != nil {
		return nil, err
	}
	ids := make([]model.ID, len(refs))
	for i, r := range refs {
		ids[i] = r.Comment
	}
	return &commentIndex{refs: refs, short: index.ShortLengths(ids)}, nil
}

// Short returns the end of the ID of a comment that tells it apart from the
// other comments.
func (ci *commentIndex) Short(id model.ID) string {
	s := id.String()
	n, ok := ci.short[id]
	if !ok {
		return s
	}
	return s[len(s)-n:]
}

// minCommentInput is the shortest input that names a comment by its end.
const minCommentInput = 4

// Resolve finds the comment arg names: a full ULID or the end of one, in
// any case.
func (ci *commentIndex) Resolve(arg string) (store.CommentRef, error) {
	u := strings.ToUpper(arg)
	var hits []store.CommentRef
	if len(u) == 26 {
		hits = ci.matching(func(id string) bool { return id == u })
	} else if len(u) >= minCommentInput {
		hits = ci.matching(func(id string) bool { return strings.HasSuffix(id, u) })
	}
	switch len(hits) {
	case 0:
		return store.CommentRef{}, fmt.Errorf("comment %q: %w", arg, store.ErrNotFound)
	case 1:
		return hits[0], nil
	}
	lines := make([]string, len(hits))
	for i, h := range hits {
		title := ""
		if ci.title != nil {
			title = ci.title(h.Task)
		}
		lines[i] = ci.Short(h.Comment) + "  " + title
	}
	return store.CommentRef{}, &ambiguousError{input: arg, noun: "comments", lines: lines}
}

func (ci *commentIndex) matching(f func(string) bool) []store.CommentRef {
	var hits []store.CommentRef
	for _, r := range ci.refs {
		if f(r.Comment.String()) {
			hits = append(hits, r)
		}
	}
	slices.SortFunc(hits, func(a, b store.CommentRef) int { return strings.Compare(a.Comment.String(), b.Comment.String()) })
	return hits
}

// commentOut is a comment as --json prints it.
type commentOut struct {
	ID        string     `json:"id"`
	Short     string     `json:"short"`
	Task      string     `json:"task"`
	Author    string     `json:"author"`
	Created   time.Time  `json:"created"`
	Updated   time.Time  `json:"updated"`
	DeletedAt *time.Time `json:"deleted_at"`
	Body      string     `json:"body"`
}

var errCommentChanged = errors.New("the comment changed while editing")

// openComment opens the store and resolves arg to a comment that is read
// from its file.
func (a *app) openComment(arg string) (*store.Store, *commentIndex, store.CommentRef, *model.Comment, error) {
	s, ix, err := a.open()
	if err != nil {
		return nil, nil, store.CommentRef{}, nil, err
	}
	ci, err := loadComments(s)
	if err != nil {
		return nil, nil, store.CommentRef{}, nil, err
	}
	ci.title = func(id model.ID) string {
		if e, ok := ix.Task(id); ok {
			return e.Title
		}
		return ""
	}
	ref, err := ci.Resolve(arg)
	if err != nil {
		return nil, nil, ref, nil, err
	}
	c, err := findComment(s, ref)
	return s, ci, ref, c, err
}

func findComment(s *store.Store, ref store.CommentRef) (*model.Comment, error) {
	for c, err := range s.Comments(ref.Task) {
		if err != nil {
			return nil, err
		}
		if c.ID == ref.Comment {
			return c, nil
		}
	}
	return nil, fmt.Errorf("comment %s: %w", ref.Comment, store.ErrNotFound)
}

// commentBody trims the end of a comment body and ends it with a newline.
// It is "" for a body of white space only.
func commentBody(b string) string {
	b = strings.TrimRight(b, " \t\r\n")
	if strings.TrimSpace(b) == "" {
		return ""
	}
	return b + "\n"
}

var errEmptyComment = errors.New("the comment is empty")

func (a *app) commentAdd(arg string, text []string) error {
	s, ix, err := a.open()
	if err != nil {
		return err
	}
	e, err := a.resolve(ix, arg)
	if err != nil {
		return err
	}
	if err := notDeleted(ix, e); err != nil {
		return err
	}
	var body, path string // path is the file of the editor, if any
	if len(text) > 0 {
		body = commentBody(strings.Join(text, " "))
	} else {
		var edited []byte
		if path, edited, err = a.editFile(ix.ShortID(e.ID), nil); err != nil {
			return err
		}
		if body = commentBody(string(edited)); body == "" {
			_ = os.Remove(path)
		}
	}
	if body == "" {
		return errEmptyComment
	}
	c := &model.Comment{ID: model.NewID(), Body: body}
	if _, err := a.update(s, store.Op{Name: "comment", Task: e.ID}, func(tx *store.Tx) error {
		return tx.PutComment(e.ID, c)
	}); err != nil {
		if path != "" {
			return fmt.Errorf("%s; the comment is kept at %s", describe(err), path)
		}
		return err
	}
	if path != "" {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return a.printComment(s, store.CommentRef{Task: e.ID, Comment: c.ID}, "Added")
}

func (a *app) commentEdit(arg string) error {
	s, ci, ref, c, err := a.openComment(arg)
	if err != nil {
		return err
	}
	short := ci.Short(ref.Comment)
	if c.DeletedAt != nil {
		return fmt.Errorf("comment %s is deleted and cannot be edited", short)
	}
	path, edited, err := a.editFile("comment-"+short, []byte(c.Body))
	if err != nil {
		return err
	}
	body := commentBody(string(edited))
	if body == "" || body == c.Body {
		if err := os.Remove(path); err != nil {
			return err
		}
		if body == "" {
			return fmt.Errorf("the comment is empty; delete it with hedgehug comment rm %s", short)
		}
		return a.printComment(s, ref, verb(false, ""))
	}
	committed, err := a.update(s, store.Op{Name: "comment", Task: ref.Task}, func(tx *store.Tx) error {
		cur, err := findComment(s, ref)
		if err != nil {
			return err
		}
		if cur.Body != c.Body || cur.DeletedAt != nil {
			return errCommentChanged
		}
		cur.Body = body
		return tx.PutComment(ref.Task, cur)
	})
	if err != nil {
		return fmt.Errorf("%s; the edited comment is kept at %s", describe(err), path)
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return a.printComment(s, ref, verb(committed, "Updated"))
}

func (a *app) commentRm(arg string) error {
	s, ci, ref, c, err := a.openComment(arg)
	if err != nil {
		return err
	}
	errDeleted := fmt.Errorf("comment %s is already deleted", ci.Short(ref.Comment))
	if c.DeletedAt != nil {
		return errDeleted
	}
	now := a.now()
	// The comment is read again under the lock, so that an edit made since
	// is not lost.
	committed, err := a.update(s, store.Op{Name: "comment", Task: ref.Task}, func(tx *store.Tx) error {
		cur, err := findComment(s, ref)
		if err != nil {
			return err
		}
		if cur.DeletedAt != nil {
			return errDeleted
		}
		cur.DeletedAt = &now
		return tx.PutComment(ref.Task, cur)
	})
	if err != nil {
		return err
	}
	return a.printComment(s, ref, verb(committed, "Deleted"))
}

// printComment prints the comment ref after a write: as JSON, or as one
// line of verb and its short ID.
func (a *app) printComment(s *store.Store, ref store.CommentRef, verb string) error {
	ci, err := loadComments(s)
	if err != nil {
		return err
	}
	short := ci.Short(ref.Comment)
	if a.jsonOut {
		c, err := findComment(s, ref)
		if err != nil {
			return err
		}
		return a.printJSON(commentOut{
			ID: c.ID.String(), Short: short, Task: ref.Task.String(), Author: c.Author,
			Created: c.Created, Updated: c.Updated, DeletedAt: c.DeletedAt, Body: c.Body,
		})
	}
	if verb == "No changes" {
		_, err = fmt.Fprintln(a.stdout, verb)
	} else {
		_, err = fmt.Fprintf(a.stdout, "%s %s\n", verb, a.style().id(short))
	}
	return err
}
