package fakegh

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
)

// The operations on comments that RefuseComments can make fail.
const (
	CommentCreate = "create"
	CommentUpdate = "update"
	CommentDelete = "delete"
	// CommentList fails each page of a list but the first, so that a list is
	// read in part and then breaks.
	CommentList = "list"
)

// RefuseComments makes op (CommentCreate, CommentUpdate, CommentDelete or
// CommentList) answer 502, or stops that.
func (s *Server) RefuseComments(op string, on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.noComments == nil {
		s.noComments = map[string]bool{}
	}
	s.noComments[op] = on
}

// nextCommentID is one more than the greatest ID of any comment.
func (s *Server) nextCommentID() int {
	n := 0
	for _, r := range s.st.Repos {
		for _, i := range r.Issues {
			for _, c := range i.Comments {
				n = max(n, c.ID)
			}
		}
	}
	return n + 1
}

// AddComment adds a comment by author (empty for a user that is gone) to an
// issue and returns its ID.
func (s *Server) AddComment(repo string, number int, author, body string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(repo)
	if r == nil {
		return 0, fmt.Errorf("no repository %s", repo)
	}
	at := slices.IndexFunc(r.Issues, func(i *Issue) bool { return i.Number == number })
	if at < 0 {
		return 0, fmt.Errorf("no issue %s#%d", repo, number)
	}
	i := r.Issues[at]
	id := s.addComment(i, author, body).ID
	i.UpdatedAt = s.now()
	return id, nil
}

func (s *Server) addComment(i *Issue, author, body string) *Comment {
	now := s.now()
	c := &Comment{ID: s.nextCommentID(), Author: author, Body: body, CreatedAt: now, UpdatedAt: now}
	i.Comments = append(i.Comments, c)
	return c
}

// EditComment changes the body of the comment id of repo.
func (s *Server) EditComment(repo string, id int, body string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, c, err := s.findComment(repo, id)
	if err != nil {
		return err
	}
	c.Body, c.UpdatedAt = body, s.now()
	i.UpdatedAt = c.UpdatedAt
	return nil
}

// RemoveComment deletes the comment id of repo.
func (s *Server) RemoveComment(repo string, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, c, err := s.findComment(repo, id)
	if err != nil {
		return err
	}
	dropComment(i, c)
	i.UpdatedAt = s.now()
	return nil
}

func dropComment(i *Issue, c *Comment) {
	i.Comments = slices.DeleteFunc(i.Comments, func(x *Comment) bool { return x == c })
}

func (s *Server) findComment(repo string, id int) (*Issue, *Comment, error) {
	r := s.find(repo)
	if r == nil {
		return nil, nil, fmt.Errorf("no repository %s", repo)
	}
	if i, c := commentOf(r, id); c != nil {
		return i, c, nil
	}
	return nil, nil, fmt.Errorf("no comment %d in %s", id, repo)
}

// commentOf is the comment id of x and its issue, or nil.
func commentOf(x *Repo, id int) (*Issue, *Comment) {
	for _, i := range x.Issues {
		for _, c := range i.Comments {
			if c.ID == id {
				return i, c
			}
		}
	}
	return nil, nil
}

func commentJSON(c *Comment) map[string]any {
	var user any
	if c.Author != "" {
		user = map[string]string{"login": c.Author}
	}
	return map[string]any{
		"id": c.ID, "body": c.Body, "user": user,
		"created_at": c.CreatedAt, "updated_at": c.UpdatedAt,
	}
}

// commentBody reads the body of a write, or answers why it cannot.
func commentBody(w http.ResponseWriter, r *http.Request) (string, bool) {
	var in struct {
		Body *string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return "", false
	}
	if in.Body == nil || *in.Body == "" {
		unprocessable(w, "body: missing")
		return "", false
	}
	return *in.Body, true
}

// issueComments serves GET and POST of the comments of an issue.
func (s *Server) issueComments(w http.ResponseWriter, r *http.Request, x *Repo, number, login string) {
	n, _ := strconv.Atoi(number)
	at := slices.IndexFunc(x.Issues, func(i *Issue) bool { return i.Number == n })
	if at < 0 {
		fail(w, http.StatusNotFound, "Not Found")
		return
	}
	i := x.Issues[at]
	if r.Method == "GET" {
		if pg, _ := strconv.Atoi(r.URL.Query().Get("page")); pg > 1 && s.noComments[CommentList] {
			fail(w, http.StatusBadGateway, "Server Error")
			return
		}
		s.page(w, r, len(i.Comments), func(k int) any { return commentJSON(i.Comments[k]) })
		return
	}
	if s.noComments[CommentCreate] {
		fail(w, http.StatusBadGateway, "Server Error")
		return
	}
	body, ok := commentBody(w, r)
	if !ok {
		return
	}
	c := s.addComment(i, login, body)
	i.UpdatedAt = s.now()
	writeJSON(w, http.StatusCreated, commentJSON(c))
}

// issueComment serves PATCH and DELETE of one comment.
func (s *Server) issueComment(w http.ResponseWriter, r *http.Request, x *Repo, id, login string) {
	op := CommentUpdate
	if r.Method == "DELETE" {
		op = CommentDelete
	}
	if s.noComments[op] {
		fail(w, http.StatusBadGateway, "Server Error")
		return
	}
	if rank(perm(x, login)) < rank("write") {
		fail(w, http.StatusForbidden, "Resource not accessible by personal access token")
		return
	}
	n, _ := strconv.Atoi(id)
	issue, c := commentOf(x, n)
	if c == nil {
		fail(w, http.StatusNotFound, "Not Found")
		return
	}
	if r.Method == "DELETE" {
		dropComment(issue, c)
		issue.UpdatedAt = s.now()
		w.WriteHeader(http.StatusNoContent)
		return
	}
	body, ok := commentBody(w, r)
	if !ok {
		return
	}
	c.Body, c.UpdatedAt = body, s.now()
	issue.UpdatedAt = c.UpdatedAt
	writeJSON(w, 200, commentJSON(c))
}
