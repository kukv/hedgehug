package ops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

var (
	errEmptyComment   = errors.New("the comment is empty")
	errCommentChanged = newErr(KindConflict, "comment.changed", errors.New("the comment changed while editing"))
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

func commentNotFound(arg string, err error) error {
	return newErr(KindNotFound, "comment.not_found", err, "input", arg)
}

// resolveComment finds the comment arg names among refs: a full ULID, in any
// case.
func resolveComment(refs []store.CommentRef, arg string) (store.CommentRef, error) {
	if len(arg) != 26 {
		return store.CommentRef{}, commentNotFound(arg, fmt.Errorf("comment %q: give the full ULID of the comment: %w", arg, store.ErrNotFound))
	}
	u := strings.ToUpper(arg)
	for _, r := range refs {
		if r.Comment.String() == u {
			return r, nil
		}
	}
	return store.CommentRef{}, commentNotFound(arg, fmt.Errorf("comment %q: %w", arg, store.ErrNotFound))
}

// CommentEditSession is a comment found and read from its file, before it is
// edited or removed.
type CommentEditSession struct {
	Store   *store.Store
	Ref     store.CommentRef
	Comment *model.Comment

	ss *Session // the session that opened it, which writes
}

// OpenComment opens the store and resolves arg to a comment that is read
// from its file.
func (ss *Session) OpenComment(arg string) (*CommentEditSession, error) {
	ts, err := ss.forComment(arg)
	if err != nil {
		return nil, err
	}
	if ts != ss {
		return ts.OpenComment(arg)
	}
	s, _, err := ss.OpenForWrite()
	if err != nil {
		return nil, err
	}
	refs, err := s.CommentULIDs()
	if err != nil {
		return nil, err
	}
	ref, err := resolveComment(refs, arg)
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
	return nil, commentNotFound(ref.Comment.String(), fmt.Errorf("comment %s: %w", ref.Comment, store.ErrNotFound))
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
	return newErr(KindConflict, "comment.deleted", errors.New(msg))
}

// CheckCommentEditable refuses to edit a deleted comment.
func CheckCommentEditable(ref store.CommentRef, c *model.Comment) error {
	if c.DeletedAt != nil {
		return commentDeleted(fmt.Sprintf("comment %s is deleted and cannot be edited", ref.Comment.String()))
	}
	return nil
}

// CommentSession is a live task checked to take a comment.
type CommentSession struct {
	Store *store.Store
	Index *index.Index
	Entry *index.Entry

	ss *Session // the session that began it, which writes
}

// BeginComment resolves arg to a live task; it is done before the body is
// taken from the user.
func (ss *Session) BeginComment(arg string) (*CommentSession, error) {
	ts, arg, err := ss.forTask(arg)
	if err != nil {
		return nil, err
	}
	if ts != ss {
		return ts.BeginComment(arg)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return nil, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return nil, err
	}
	if err := NotDeleted(ix, e); err != nil {
		return nil, err
	}
	return &CommentSession{Store: s, Index: ix, Entry: e, ss: ss}, nil
}

// PendingComment is a comment ready to be written.
type PendingComment struct {
	cs      *CommentSession
	ss      *Session
	Comment *model.Comment
	accts   []string
}

// PrepareComment builds the comment with the body, which CommentBody cleans;
// one of white space only is refused. Its errors come before anything is
// written.
func (ss *Session) PrepareComment(cs *CommentSession, body string) (*PendingComment, error) {
	if cs.ss != nil {
		ss = cs.ss
	}
	if body = CommentBody(body); body == "" {
		return nil, newErr(KindInvalid, "comment.empty", errEmptyComment)
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
	accts := NewMentions("", c.Body, host)
	if c.Author != "" {
		accts = append([]string{c.Author}, accts...)
	}
	return &PendingComment{cs: cs, ss: ss, Comment: c, accts: accts}, nil
}

// WriteComment writes the comment to the task of its session. The task is
// not checked again.
func (ss *Session) WriteComment(p *PendingComment) (CommentResult, error) {
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
		return WatchInTx(tx, e.ULID, p.accts...)
	})
	if err != nil {
		return CommentResult{}, err
	}
	return CommentResult{Store: s, Ref: store.CommentRef{Task: e.ULID, Comment: c.ULID}, Comment: c, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}

// CommentAdd adds a comment to the live task arg. It is the one call for the
// server; the CLI takes the body between BeginComment and PrepareComment.
func (ss *Session) CommentAdd(arg, body string) (CommentResult, error) {
	cs, err := ss.BeginComment(arg)
	if err != nil {
		return CommentResult{}, err
	}
	p, err := ss.PrepareComment(cs, body)
	if err != nil {
		return CommentResult{}, err
	}
	return ss.WriteComment(p)
}

// CommentEdit sets the body of the comment arg; CommentBody cleans it first.
// A body that is empty is refused, one that is the same as before changes
// nothing. It is the one call for the server; the CLI takes the body between
// OpenComment and PrepareCommentEdit.
func (ss *Session) CommentEdit(arg, body string) (CommentResult, error) {
	ces, err := ss.OpenComment(arg)
	if err != nil {
		return CommentResult{}, err
	}
	if err := CheckCommentEditable(ces.Ref, ces.Comment); err != nil {
		return CommentResult{}, err
	}
	p, err := ss.PrepareCommentEdit(ces, body)
	if err != nil {
		return CommentResult{}, err
	}
	return ss.WriteCommentEdit(p)
}

// PendingCommentEdit is an edit of a comment ready to be written.
type PendingCommentEdit struct {
	ss        *Session
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
func (ss *Session) PrepareCommentEdit(ces *CommentEditSession, body string) (*PendingCommentEdit, error) {
	if ces.ss != nil {
		ss = ces.ss
	}
	s, ref, c := ces.Store, ces.Ref, ces.Comment
	if body = CommentBody(body); body == "" {
		return nil, newErr(KindInvalid, "comment.empty",
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
	p.mentioned = NewMentions(c.Body, body, host)
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
func (ss *Session) WriteCommentEdit(p *PendingCommentEdit) (CommentResult, error) {
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
		return WatchInTx(tx, p.ref.Task, p.mentioned...)
	})
	if err != nil {
		return CommentResult{}, err
	}
	return CommentResult{Store: p.s, Ref: p.ref, Comment: written, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}

// CommentRemove deletes the comment arg.
func (ss *Session) CommentRemove(arg string) (CommentResult, error) {
	ces, err := ss.OpenComment(arg)
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
	return CommentResult{Store: s, Ref: ref, Comment: written, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}
