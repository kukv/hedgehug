package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
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
			Use:   "edit <comment-ulid>",
			Short: "Edit the body of a comment in $EDITOR",
			Args:  exactArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return a.commentEdit(args[0]) },
		},
		&cobra.Command{
			Use:   "rm <comment-ulid>",
			Short: "Delete a comment",
			Args:  exactArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return a.commentRm(args[0]) },
		},
	)
	return cmd
}

// commentIndex lists the comments of a store, deleted ones included, to
// resolve their ULIDs.
type commentIndex struct {
	refs []store.CommentRef
}

func loadComments(s *store.Store) (*commentIndex, error) {
	refs, err := s.CommentULIDs()
	if err != nil {
		return nil, err
	}
	return &commentIndex{refs: refs}, nil
}

// Resolve finds the comment arg names: a full ULID, in any case.
func (ci *commentIndex) Resolve(arg string) (store.CommentRef, error) {
	if len(arg) != 26 {
		return store.CommentRef{}, fmt.Errorf("comment %q: give the full ULID of the comment: %w", arg, store.ErrNotFound)
	}
	u := strings.ToUpper(arg)
	for _, r := range ci.refs {
		if r.Comment.String() == u {
			return r, nil
		}
	}
	return store.CommentRef{}, fmt.Errorf("comment %q: %w", arg, store.ErrNotFound)
}

// commentOut is a comment as --json prints it.
type commentOut struct {
	ULID      string     `json:"ulid"`
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
func (a *app) openComment(arg string) (*store.Store, store.CommentRef, *model.Comment, error) {
	s, _, err := a.ss.Open()
	if err != nil {
		return nil, store.CommentRef{}, nil, err
	}
	ci, err := loadComments(s)
	if err != nil {
		return nil, store.CommentRef{}, nil, err
	}
	ref, err := ci.Resolve(arg)
	if err != nil {
		return nil, ref, nil, err
	}
	c, err := findComment(s, ref)
	return s, ref, c, err
}

func findComment(s *store.Store, ref store.CommentRef) (*model.Comment, error) {
	for c, err := range s.Comments(ref.Task) {
		if err != nil {
			return nil, err
		}
		if c.ULID == ref.Comment {
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
	s, ix, err := a.ss.Open()
	if err != nil {
		return err
	}
	e, err := a.ss.Resolve(ix, arg)
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
		if path, edited, err = a.editFile(ops.TaskFileName(e), nil); err != nil {
			return err
		}
		if body = commentBody(string(edited)); body == "" {
			_ = os.Remove(path)
		}
	}
	if body == "" {
		return errEmptyComment
	}
	c := &model.Comment{ULID: model.NewULID(), Body: body}
	if c.Author, err = a.ss.AccountID(); err != nil {
		return err
	}
	host, err := a.writerHost()
	if err != nil {
		return err
	}
	accts := newMentions("", c.Body, host)
	if c.Author != "" {
		accts = append([]string{c.Author}, accts...)
	}
	if _, err := a.ss.Update(s, store.Op{Name: "comment", Task: e.ULID}, func(tx *store.Tx) error {
		if err := tx.PutComment(e.ULID, c); err != nil {
			return err
		}
		if e.Broken != "" {
			return nil
		}
		return watchInTx(tx, e.ULID, accts...)
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
	return a.printComment(s, store.CommentRef{Task: e.ULID, Comment: c.ULID}, "Added")
}

func (a *app) commentEdit(arg string) error {
	s, ref, c, err := a.openComment(arg)
	if err != nil {
		return err
	}
	commentULID := ref.Comment.String()
	if c.DeletedAt != nil {
		return fmt.Errorf("comment %s is deleted and cannot be edited", commentULID)
	}
	path, edited, err := a.editFile("comment-"+commentULID, []byte(c.Body))
	if err != nil {
		return err
	}
	body := commentBody(string(edited))
	if body == "" || body == c.Body {
		if err := os.Remove(path); err != nil {
			return err
		}
		if body == "" {
			return fmt.Errorf("the comment is empty; delete it with hedgehug comment rm %s", commentULID)
		}
		return a.printComment(s, ref, verb(false, ""))
	}
	host, err := a.writerHost()
	if err != nil {
		return err
	}
	mentioned := newMentions(c.Body, body, host)
	if len(mentioned) > 0 {
		// A broken task has no watchers to add to.
		_, ix, err := a.ss.Open()
		if err != nil {
			return err
		}
		if e, ok := ix.Task(ref.Task); ok && e.Broken != "" {
			mentioned = nil
		}
	}
	committed, err := a.ss.Update(s, store.Op{Name: "comment", Task: ref.Task}, func(tx *store.Tx) error {
		cur, err := findComment(s, ref)
		if err != nil {
			return err
		}
		if cur.Body != c.Body || cur.DeletedAt != nil {
			return errCommentChanged
		}
		cur.Body = body
		if err := tx.PutComment(ref.Task, cur); err != nil {
			return err
		}
		return watchInTx(tx, ref.Task, mentioned...)
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
	s, ref, c, err := a.openComment(arg)
	if err != nil {
		return err
	}
	errDeleted := fmt.Errorf("comment %s is already deleted", ref.Comment.String())
	if c.DeletedAt != nil {
		return errDeleted
	}
	now := a.now()
	// The comment is read again under the lock, so that an edit made since
	// is not lost.
	committed, err := a.ss.Update(s, store.Op{Name: "comment", Task: ref.Task}, func(tx *store.Tx) error {
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
// line of verb and its ULID.
func (a *app) printComment(s *store.Store, ref store.CommentRef, verb string) error {
	var err error
	if a.jsonOut {
		c, err := findComment(s, ref)
		if err != nil {
			return err
		}
		return a.printJSON(commentOut{
			ULID: c.ULID.String(), Task: ref.Task.String(), Author: c.Author,
			Created: c.Created, Updated: c.Updated, DeletedAt: c.DeletedAt, Body: c.Body,
		})
	}
	if verb == "No changes" {
		_, err = fmt.Fprintln(a.stdout, verb)
	} else {
		_, err = fmt.Fprintf(a.stdout, "%s %s\n", verb, a.style().id(ref.Comment.String()))
	}
	return err
}
