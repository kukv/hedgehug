// Package comments is the operations on the comments of a task.
package comments

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

var (
	errEmptyComment   = errors.New("the comment is empty")
	errCommentChanged = usecase.NewErr(usecase.KindConflict, "comment.changed", errors.New("the comment changed while editing"))
)

// CommentRef names one comment file.
type CommentRef = store.CommentRef

// CommentResult is what a comment operation did: the comment ref and the
// comment as written (as read, when nothing changed).
type CommentResult struct {
	Store     *store.Store
	Ref       store.CommentRef
	Comment   *model.Comment
	Committed bool
	Commit    string // hex hash of the commit made; "" when nothing was committed
}

// CommentWrite is what a write to a comment answers, in the CLI's --json and
// in the server's response.
type CommentWrite struct {
	Comment usecase.CommentView `json:"comment" doc:"The comment as written; for a removal, as it was"`
	Commit  *string             `json:"commit" doc:"The commit made; null when nothing changed"`
}

// Write is the answer to the write: the comment as written, and the commit.
func (r CommentResult) Write() CommentWrite {
	w := CommentWrite{Commit: usecase.CommitPtr(r.Commit)}
	if c := r.Comment; c != nil {
		w.Comment = usecase.CommentView{ULID: c.ULID.String(), Task: r.Ref.Task.String(), Author: c.Author, Created: c.Created, GitHub: c.Mirrored(), Body: c.Body}
		if !c.Updated.IsZero() {
			w.Comment.Updated = &c.Updated
		}
		w.Comment.DeletedAt = c.DeletedAt
	}
	return w
}

// CommentULID is the ULID of the comment written.
func (r CommentResult) CommentULID() string { return r.Ref.Comment.String() }

// CommentEditSession is a comment found and read from its file, before it is
// edited or removed.
type CommentEditSession struct {
	Store   *store.Store
	Ref     store.CommentRef
	Comment *model.Comment

	ss *usecase.Session // the session that opened it, which writes
}

// OfTask reports whether the comment is a comment of the task.
func (ces *CommentEditSession) OfTask(task model.ULID) bool { return ces.Ref.Task == task }

// CommentULID is the ULID of the comment.
func (ces *CommentEditSession) CommentULID() string { return ces.Ref.Comment.String() }

// Body is the body of the comment as it is now.
func (ces *CommentEditSession) Body() string { return ces.Comment.Body }

// Result is the result of an edit that changed nothing: the comment as it
// is, and no commit.
func (ces *CommentEditSession) Result() CommentResult {
	return CommentResult{Store: ces.Store, Ref: ces.Ref, Comment: ces.Comment}
}

// OpenComment opens the store and resolves arg to a comment that is read
// from its file.
func OpenComment(ss *usecase.Session, arg string) (*CommentEditSession, error) {
	ts, err := ss.ForComment(arg)
	if err != nil {
		return nil, err
	}
	if ts != ss {
		return OpenComment(ts, arg)
	}
	s, _, err := ss.OpenForWrite()
	if err != nil {
		return nil, err
	}
	refs, err := s.CommentULIDs()
	if err != nil {
		return nil, err
	}
	ref, err := usecase.ResolveComment(refs, arg)
	if err != nil {
		return nil, err
	}
	c, err := FindComment(s, ref)
	if err != nil {
		return nil, err
	}
	return &CommentEditSession{Store: s, Ref: ref, Comment: c, ss: ss}, nil
}

// FindComment reads the comment ref from the comments of its task.
func FindComment(s *store.Store, ref store.CommentRef) (*model.Comment, error) {
	for c, err := range s.Comments(ref.Task) {
		if err != nil {
			return nil, err
		}
		if c.ULID == ref.Comment {
			return c, nil
		}
	}
	return nil, usecase.CommentNotFound(ref.Comment.String(), fmt.Errorf("comment %s: %w", ref.Comment, store.ErrNotFound))
}

// CommentBody trims the end of a comment body and ends it with a newline.
// It is "" for a body of white space only.
func CommentBody(b string) string {
	b = strings.TrimRight(b, " \t\r\n")
	if strings.TrimSpace(b) == "" {
		return ""
	}
	return b + "\n"
}

func commentDeleted(msg string) error {
	return usecase.NewErr(usecase.KindConflict, "comment.deleted", errors.New(msg))
}

// CheckCommentEditable refuses to edit a deleted comment, one taken from
// GitHub, and one that none of the accounts me wrote, in this order. verb is
// "edited" or "removed".
func CheckCommentEditable(ref store.CommentRef, c *model.Comment, me []string, verb string) error {
	if c.DeletedAt != nil {
		return commentDeleted(fmt.Sprintf("comment %s is deleted and cannot be edited", ref.Comment.String()))
	}
	if c.Mirrored() {
		return usecase.NewErr(usecase.KindInvalid, "comment.mirrored",
			fmt.Errorf("%s's comment was taken from GitHub and cannot be %s; change it on GitHub", strings.TrimPrefix(c.Author, model.MirroredAuthorPrefix), verb), "author", c.Author)
	}
	if c.Author == "" {
		return usecase.NewErr(usecase.KindConflict, "comment.no_author",
			fmt.Errorf("a comment whose author is unknown cannot be %s", verb))
	}
	if !slices.ContainsFunc(me, func(a string) bool { return config.SameAccount(a, c.Author) }) {
		return usecase.NewErr(usecase.KindConflict, "comment.not_author",
			fmt.Errorf("%s's comment cannot be %s", c.Author, verb), "author", c.Author)
	}
	return nil
}

// CheckMyComment is CheckCommentEditable with the accounts of config.yaml as
// me.
func CheckMyComment(ss *usecase.Session, ces *CommentEditSession, verb string) error {
	c, _, err := ss.Config()
	if err != nil {
		return err
	}
	return CheckCommentEditable(ces.Ref, ces.Comment, c.Me(), verb)
}

// CommentSession is a live task checked to take a comment.
type CommentSession struct {
	Store *store.Store
	Index *index.Index
	Entry *index.Entry

	ss *usecase.Session // the session that began it, which writes
}

// FileName names the temporary file that takes the body of the comment: the
// id of the task, or its ULID when it has no id.
func (cs *CommentSession) FileName() string { return usecase.TaskFileName(cs.Entry) }

// BeginComment resolves arg to a live task; it is done before the body is
// taken from the user.
func BeginComment(ss *usecase.Session, arg string) (*CommentSession, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return nil, err
	}
	if ts != ss {
		return BeginComment(ts, arg)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return nil, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return nil, err
	}
	if err := usecase.NotDeleted(ix, e); err != nil {
		return nil, err
	}
	return &CommentSession{Store: s, Index: ix, Entry: e, ss: ss}, nil
}

// PendingComment is a comment ready to be written.
type PendingComment struct {
	cs      *CommentSession
	ss      *usecase.Session
	Comment *model.Comment
	accts   []string
}

// PrepareComment builds the comment with the body, which CommentBody cleans;
// one of white space only is refused. Its errors come before anything is
// written.
func PrepareComment(ss *usecase.Session, cs *CommentSession, body string) (*PendingComment, error) {
	if cs.ss != nil {
		ss = cs.ss
	}
	if body = CommentBody(body); body == "" {
		return nil, usecase.NewErr(usecase.KindInvalid, "comment.empty", errEmptyComment)
	}
	c := &model.Comment{ULID: model.NewULID(), Body: body}
	var err error
	if c.Author, err = ss.AccountID(); err != nil {
		return nil, err
	}
	host, err := ss.WriterHost()
	if err != nil {
		return nil, err
	}
	accts := usecase.NewMentions("", c.Body, host)
	if c.Author != "" {
		accts = append([]string{c.Author}, accts...)
	}
	return &PendingComment{cs: cs, ss: ss, Comment: c, accts: accts}, nil
}

// WriteComment writes the comment to the task of its session. The task is
// not checked again.
func WriteComment(ss *usecase.Session, p *PendingComment) (CommentResult, error) {
	if p.ss != nil {
		ss = p.ss
	}
	s, e, c := p.cs.Store, p.cs.Entry, p.Comment
	hash, err := ss.UpdateCommit(s, store.Op{Name: "comment", Task: e.ULID}, func(tx *store.Tx) error {
		if err := tx.PutComment(e.ULID, c); err != nil {
			return err
		}
		if e.Broken != "" {
			return nil
		}
		return usecase.WatchInTx(tx, e.ULID, p.accts...)
	})
	if err != nil {
		return CommentResult{}, err
	}
	if !hash.IsZero() {
		ss.WarnMentions(s, "", c.Body)
	}
	return CommentResult{Store: s, Ref: store.CommentRef{Task: e.ULID, Comment: c.ULID}, Comment: c, Committed: !hash.IsZero(), Commit: usecase.CommitHex(hash)}, nil
}

// CommentAdd adds a comment to the live task arg. It is the one call for the
// server; the CLI takes the body between BeginComment and PrepareComment.
func CommentAdd(ss *usecase.Session, arg, body string) (CommentResult, error) {
	cs, err := BeginComment(ss, arg)
	if err != nil {
		return CommentResult{}, err
	}
	p, err := PrepareComment(ss, cs, body)
	if err != nil {
		return CommentResult{}, err
	}
	return WriteComment(ss, p)
}

// CommentEdit sets the body of the comment arg; CommentBody cleans it first.
// A body that is empty is refused, one that is the same as before changes
// nothing. It is the one call for the server; the CLI takes the body between
// OpenComment and PrepareCommentEdit.
func CommentEdit(ss *usecase.Session, arg, body string) (CommentResult, error) {
	ces, err := OpenComment(ss, arg)
	if err != nil {
		return CommentResult{}, err
	}
	if err := CheckMyComment(ss, ces, "edited"); err != nil {
		return CommentResult{}, err
	}
	p, err := PrepareCommentEdit(ss, ces, body)
	if err != nil {
		return CommentResult{}, err
	}
	return WriteCommentEdit(ss, p)
}

// PendingCommentEdit is an edit of a comment ready to be written.
type PendingCommentEdit struct {
	ss        *usecase.Session
	s         *store.Store
	ref       store.CommentRef
	c         *model.Comment
	body      string
	mentioned []string
}

// PrepareCommentEdit checks the body, which CommentBody cleans, for the
// comment of ces, as OpenComment returned it before the body was taken from
// the user. One of white space only is refused; one that is the same as
// before makes WriteCommentEdit change nothing. Its errors come before
// anything is written.
func PrepareCommentEdit(ss *usecase.Session, ces *CommentEditSession, body string) (*PendingCommentEdit, error) {
	if ces.ss != nil {
		ss = ces.ss
	}
	s, ref, c := ces.Store, ces.Ref, ces.Comment
	if body = CommentBody(body); body == "" {
		return nil, usecase.NewErr(usecase.KindInvalid, "comment.empty",
			fmt.Errorf("the comment is empty; delete it with hedgehug comment rm %s", ref.Comment.String()))
	}
	p := &PendingCommentEdit{ss: ss, s: s, ref: ref, c: c, body: body}
	if body == c.Body {
		return p, nil
	}
	host, err := ss.WriterHost()
	if err != nil {
		return nil, err
	}
	p.mentioned = usecase.NewMentions(c.Body, body, host)
	if len(p.mentioned) > 0 {
		// A broken task has no watchers to add to.
		_, ix, err := ss.Open()
		if err != nil {
			return nil, err
		}
		if e, ok := ix.Task(ref.Task); ok && e.Broken != "" {
			p.mentioned = nil
		}
	}
	return p, nil
}

// WriteCommentEdit writes the edit; it refuses when the comment is not as c
// any more.
func WriteCommentEdit(ss *usecase.Session, p *PendingCommentEdit) (CommentResult, error) {
	if p.ss != nil {
		ss = p.ss
	}
	if p.body == p.c.Body {
		return CommentResult{Store: p.s, Ref: p.ref, Comment: p.c}, nil
	}
	var written *model.Comment
	hash, err := ss.UpdateCommit(p.s, store.Op{Name: "comment", Task: p.ref.Task}, func(tx *store.Tx) error {
		cur, err := FindComment(p.s, p.ref)
		if err != nil {
			return err
		}
		if cur.Body != p.c.Body || cur.DeletedAt != nil {
			return errCommentChanged
		}
		cur.Body = p.body
		written = cur
		if err := tx.PutComment(p.ref.Task, cur); err != nil {
			return err
		}
		return usecase.WatchInTx(tx, p.ref.Task, p.mentioned...)
	})
	if err != nil {
		return CommentResult{}, err
	}
	if !hash.IsZero() {
		ss.WarnMentions(p.s, p.c.Body, p.body)
	}
	return CommentResult{Store: p.s, Ref: p.ref, Comment: written, Committed: !hash.IsZero(), Commit: usecase.CommitHex(hash)}, nil
}

// CommentRemove deletes the comment arg.
func CommentRemove(ss *usecase.Session, arg string) (CommentResult, error) {
	ces, err := OpenComment(ss, arg)
	if err != nil {
		return CommentResult{}, err
	}
	if ces.ss != nil {
		ss = ces.ss
	}
	s, ref, c := ces.Store, ces.Ref, ces.Comment
	errDeleted := commentDeleted(fmt.Sprintf("comment %s is already deleted", ref.Comment.String()))
	if c.DeletedAt != nil {
		return CommentResult{}, errDeleted
	}
	if err := CheckMyComment(ss, ces, "removed"); err != nil {
		return CommentResult{}, err
	}
	now := ss.Now()
	var written *model.Comment
	// The comment is read again under the lock, so that an edit made since
	// is not lost.
	hash, err := ss.UpdateCommit(s, store.Op{Name: "comment", Task: ref.Task}, func(tx *store.Tx) error {
		cur, err := FindComment(s, ref)
		if err != nil {
			return err
		}
		if cur.DeletedAt != nil {
			return errDeleted
		}
		cur.DeletedAt = &now
		written = cur
		return tx.PutComment(ref.Task, cur)
	})
	if err != nil {
		return CommentResult{}, err
	}
	return CommentResult{Store: s, Ref: ref, Comment: written, Committed: !hash.IsZero(), Commit: usecase.CommitHex(hash)}, nil
}
