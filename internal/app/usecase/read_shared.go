package usecase

import (
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/query"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
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
	ULID string `json:"ulid"`
	// Task, Updated and DeletedAt are only in the answer to a write to a
	// comment (show leaves them out).
	Task      string     `json:"task,omitempty" doc:"The ULID of the task the comment is on"`
	Author    string     `json:"author"`
	Created   time.Time  `json:"created"`
	Updated   *time.Time `json:"updated,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" doc:"When the comment was removed"`
	Body      string     `json:"body"`
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
	// Worklog adds up the time logged on the task; left out when none is.
	Worklog *WorklogTotal `json:"worklog,omitempty"`

	// For the text.
	CreatedBy string         `json:"-"` // who created the task; empty when the file does not say
	Relations []RelationView `json:"-"` // Reverse in the order the text lists them
}

// WorklogTotal is what the worklog entries of a task add up to.
type WorklogTotal struct {
	Total string `json:"total" doc:"The time spent, such as 1h30m"`
	Count int    `json:"count"`
}

// RelationView is one kind of task pointing at the task shown.
type RelationView struct {
	Key   string // the name in JSON: blocks, or the property
	Label string // the name in the text
	Tasks []Ref
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
	// CurrentSubject is Subject with the current id of the task in place of
	// the label it was written with (subjectNow); for the text.
	CurrentSubject string `json:"-"`
}

// Trailer is the value of the first trailer named key.
func (l LogView) Trailer(key string) (string, bool) {
	for _, t := range l.Trailers {
		if t.Key == key {
			return t.Value, true
		}
	}
	return "", false
}

// LogViewOf is the commit e of the store ix is of, as --json prints it.
func LogViewOf(ix *index.Index, e gitrepo.LogEntry) LogView {
	j := LogView{Hash: e.Hash.String(), Date: e.When, Subject: e.Subject, Store: ix.StoreName(), CurrentSubject: subjectNow(ix, e)}
	j.Author.Name, j.Author.Email = e.Author.Name, e.Author.Email
	j.Trailers = make([]LogTrailer, len(e.Trailers))
	for i, t := range e.Trailers {
		j.Trailers[i] = LogTrailer{Key: t.Key, Value: t.Value}
	}
	if v, ok := e.Trailer("Hedgehug-Op"); ok {
		j.Op = &v
	}
	if v, ok := e.Trailer("Hedgehug-Task"); ok {
		j.Task = &v
		if ulid, err := model.ParseULID(v); err == nil {
			if t, ok := ix.Task(ulid); ok && t.ID != "" {
				j.ID = &t.ID
			}
		}
	}
	if ws, ok := logWorkspace(ix, e); ok {
		j.Workspace = &WorkspaceRef{ULID: ws.String()}
		if w, ok := ix.Workspace(ws); ok {
			j.Workspace.Name, j.Workspace.IDPrefix, j.Workspace.Known = w.Name, w.IDPrefix, true
		}
	}
	return j
}

// logWorkspace is the workspace of a commit: the one its Hedgehug-Workspace
// trailer names, or else the one the task of its Hedgehug-Task is in now.
// A commit with neither, or of a task the index does not have, has none.
func logWorkspace(ix *index.Index, e gitrepo.LogEntry) (model.ULID, bool) {
	ws, _ := e.Trailer("Hedgehug-Workspace")
	task, _ := e.Trailer("Hedgehug-Task")
	return CommitWorkspace(ix, ws, task)
}

// CommitWorkspace is the workspace of a commit whose Hedgehug-Workspace and
// Hedgehug-Task trailers are ws and task (either may be empty).
func CommitWorkspace(ix *index.Index, ws, task string) (model.ULID, bool) {
	if ws != "" {
		ulid, err := model.ParseULID(ws)
		return ulid, err == nil
	}
	if task != "" {
		if ulid, err := model.ParseULID(task); err == nil {
			if t, ok := ix.Task(ulid); ok && !t.Workspace.IsZero() {
				return t.Workspace, true
			}
		}
	}
	return model.ULID{}, false
}

// subjectNow gives the subject of a Hedgehug commit with the task's current
// id and ULID in place of the label it was written with: the subject is
// "hedgehug: <op> <label> <title>". Other subjects, and tasks the index does
// not have, keep the subject as it is.
func subjectNow(ix *index.Index, le gitrepo.LogEntry) string {
	op, ok1 := le.Trailer("Hedgehug-Op")
	taskULID, ok2 := le.Trailer("Hedgehug-Task")
	head := "hedgehug: " + op + " "
	rest, ok3 := strings.CutPrefix(le.Subject, head)
	if !ok1 || !ok2 || !ok3 {
		return le.Subject
	}
	ulid, err := model.ParseULID(taskULID)
	if err != nil {
		return le.Subject
	}
	e, ok := ix.Task(ulid)
	if !ok {
		return le.Subject
	}
	_, title, _ := strings.Cut(rest, " ")
	return strings.TrimRight(head+TaskLabel(ix, e)+" "+title, " ")
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

// LinkView is a link in use and what GitHub said of it when it was last read.
type LinkView struct {
	Type  string `json:"type" enum:"github_pr,github_issue"`
	Ref   string `json:"ref"`
	Label string `json:"label" doc:"The ref as people write it: org-a/web#123"`
	Sync  string `json:"sync" enum:"mirror,push,link"`
	// Fields are the fields this link syncs, when it says so itself.
	Fields []string `json:"fields,omitempty" nullable:"false"`
	Via    string   `json:"via,omitempty" enum:"branch,mark,id,comment" doc:"What linked it automatically; absent for a link made by a person"`
	Title  string   `json:"title,omitempty" doc:"The title on GitHub; empty until GitHub is read"`
	Branch string   `json:"branch,omitempty" doc:"The branch of the pull request"`
	State  string   `json:"state,omitempty" doc:"Empty until GitHub is read"`
	Review string   `json:"review,omitempty"`
	Checks string   `json:"checks,omitempty"`
	// MergedAt is when GitHub merged the pull request.
	MergedAt *time.Time `json:"merged_at,omitempty"`
	// StateReason is why an issue closed: completed or not_planned.
	StateReason string `json:"state_reason,omitempty"`
}

// LinkViewsOf lists the links of t in use, in file order. A gh that does not
// read shows the links without state.
func LinkViewsOf(t *model.Task) []LinkView {
	g, _ := t.GitHub()
	return linkViews(t.Links, g)
}

func linkViews(links []model.Link, g model.GitHubState) []LinkView {
	var out []LinkView
	for _, l := range links {
		if !l.Live() {
			continue
		}
		v := LinkView{Type: l.Type, Ref: l.Ref, Label: l.Ref, Sync: l.Sync, Fields: l.Fields, Via: l.Via}
		if r, ok := model.ParseGitHubRef(l.Ref); ok {
			v.Ref, v.Label = r.String(), r.Label()
		}
		ref := model.CanonRef(l.Ref)
		for _, p := range g.PRs {
			if model.CanonRef(p.Ref) == ref {
				v.Title, v.Branch = p.Title, p.Branch
				v.State, v.Review, v.Checks, v.MergedAt = p.State, p.Review, p.Checks, p.MergedAt
			}
		}
		for _, x := range g.Issues {
			if model.CanonRef(x.Ref) == ref {
				v.Title, v.State = x.Title, x.State
				if x.StateReason != nil {
					v.StateReason = *x.StateReason
				}
			}
		}
		out = append(out, v)
	}
	return out
}
