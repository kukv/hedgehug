package usecase

import (
	"time"

	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/query"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// ParseQuery parses the words of a query. A query that does not parse is a
// usage error.
func ParseQuery(s string) (*query.Query, error) {
	q, err := query.Parse(s)
	if err != nil {
		return nil, NewErr(KindUsage, "query.invalid", err)
	}
	return q, nil
}

// HasTerm reports whether q has a term with the key.
func HasTerm(q *query.Query, key string) bool {
	for _, t := range q.Terms {
		if t.Key == key {
			return true
		}
	}
	return false
}

// CommentView is a comment as --json prints it.
type CommentView struct {
	ULID    string    `json:"ulid"`
	Author  string    `json:"author"`
	Created time.Time `json:"created"`
	Body    string    `json:"body"`
}

// ShowView is a task as show --json prints it.
type ShowView struct {
	*TaskView
	Body     string           `json:"body"`
	Watchers []string         `json:"watchers"`
	Repos    []string         `json:"repos"`
	Comments []CommentView    `json:"comments"`
	Reverse  map[string][]Ref `json:"reverse"`
	// ReferencedBy are the tasks whose body or comments write this one as #<ID>.
	ReferencedBy []Ref     `json:"referenced_by"`
	History      []LogView `json:"history"`
}

// LogView is a commit as --json prints it.
type LogView struct {
	Hash   string    `json:"hash"`
	Date   time.Time `json:"date"`
	Author struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"author"`
	Store string `json:"store"`
	// Workspace is left out for a commit of no workspace (logWorkspace),
	// rather than null: the OpenAPI generator cannot make an object nullable.
	Workspace *WorkspaceRef `json:"workspace,omitempty"`
	Op        *string       `json:"op"`
	Task      *string       `json:"task"`
	ID        *string       `json:"id"` // the current id of the task
	Subject   string        `json:"subject"`
	// Trailers are all the trailers of the commit, in the order they are
	// written (Hedgehug-Overwritten, Hedgehug-Moved-From...); [] with none.
	Trailers []LogTrailer `json:"trailers" nullable:"false"`
}

// OpenHistory opens the history index of s, telling ss.Progress how far the
// work of bringing it up to HEAD has come.
func (ss *Session) OpenHistory(s *store.Store, name string) (*history.History, error) {
	var opt history.Options
	if ss.Progress != nil {
		opt.Progress = func(done, total int) { ss.Progress(name, done, total) }
	}
	return history.Open(s, opt)
}

// LogTrailer is a trailer of a commit message: "Key: Value".
type LogTrailer struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
