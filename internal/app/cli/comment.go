package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/lib/osfs"
)

func (a *app) commentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Add a comment to a task, or edit or delete one",
		// hedgehug comment <id> was the way to add one; say so rather than
		// fail with an unknown command.
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usageErrorf("%q is not a subcommand; add a comment with hedgehug comment add <id> [text...] (the others are edit and rm)", args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "add <id> [text...]",
			Short: "Add a comment to a task",
			Long:  "Add a comment to a task. Without text, the body is written in $EDITOR.",
			Args:  minArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return a.commentAdd(args[0], args[1:]) },
		},
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

func (a *app) commentAdd(arg string, text []string) error {
	cs, err := comments.BeginComment(a.ss, arg)
	if err != nil {
		return err
	}
	var body, path string // path is the file of the editor, if any
	if len(text) > 0 {
		body = strings.Join(text, " ")
	} else {
		var edited []byte
		if path, edited, err = a.editFile(usecase.TaskFileName(cs.Entry), nil); err != nil {
			return err
		}
		if body = comments.CommentBody(string(edited)); body == "" {
			_ = osfs.Remove(path)
		}
	}
	p, err := comments.PrepareComment(a.ss, cs, body)
	if err != nil {
		return err
	}
	r, err := comments.WriteComment(a.ss, p)
	if err != nil {
		if path != "" {
			return fmt.Errorf("%s; the comment is kept at %s", usecase.Describe(err), path)
		}
		return err
	}
	if path != "" {
		if err := osfs.Remove(path); err != nil {
			return err
		}
	}
	return a.printComment(r.Store, r.Ref, "Added")
}

func (a *app) commentEdit(arg string) error {
	ces, err := comments.OpenComment(a.ss, arg)
	if err != nil {
		return err
	}
	s, ref, c := ces.Store, ces.Ref, ces.Comment
	if err := comments.CheckMyComment(a.ss, ces, "edited"); err != nil {
		return err
	}
	commentULID := ref.Comment.String()
	path, edited, err := a.editFile("comment-"+commentULID, []byte(c.Body))
	if err != nil {
		return err
	}
	body := comments.CommentBody(string(edited))
	p, err := comments.PrepareCommentEdit(a.ss, ces, body)
	if body == "" || body == c.Body {
		if rerr := osfs.Remove(path); rerr != nil {
			return rerr
		}
		if err != nil {
			return err
		}
		return a.printComment(s, ref, verb(false, ""))
	}
	if err != nil {
		return err
	}
	r, err := comments.WriteCommentEdit(a.ss, p)
	if err != nil {
		return fmt.Errorf("%s; the edited comment is kept at %s", usecase.Describe(err), path)
	}
	if err := osfs.Remove(path); err != nil {
		return err
	}
	return a.printComment(r.Store, r.Ref, verb(r.Committed, "Updated"))
}

func (a *app) commentRm(arg string) error {
	r, err := comments.CommentRemove(a.ss, arg)
	if err != nil {
		return err
	}
	return a.printComment(r.Store, r.Ref, verb(r.Committed, "Deleted"))
}

// printComment prints the comment ref after a write: as JSON, or as one
// line of verb and its ULID.
func (a *app) printComment(s *store.Store, ref store.CommentRef, verb string) error {
	var err error
	if a.jsonOut {
		c, err := comments.FindComment(s, ref)
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
