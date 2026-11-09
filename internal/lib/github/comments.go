package github

import (
	"strconv"
	"time"
)

// IssueUser is the author of a comment.
type IssueUser struct {
	Login string `json:"login"`
}

// IssueComment is a comment of an issue. User is nil for a user that GitHub
// no longer has (it answers null); a bot has its login as it is, such as
// "dependabot[bot]".
type IssueComment struct {
	ID        int64      `json:"id"`
	User      *IssueUser `json:"user"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (c *Client) commentsURL(owner, name string, number int) string {
	return c.repoURL(owner, name) + "/issues/" + strconv.Itoa(number) + "/comments"
}

func (c *Client) commentURL(owner, name string, id int64) string {
	return c.repoURL(owner, name) + "/issues/comments/" + strconv.FormatInt(id, 10)
}

// IssueComments lists the comments of the issue number of owner/name, in the
// order GitHub has them, all pages. When a page cannot be read it returns the
// error and no comments, never a part of the list.
func (c *Client) IssueComments(owner, name string, number int) ([]IssueComment, error) {
	return getAll[IssueComment](c, c.commentsURL(owner, name, number)+"?per_page=100")
}

// CreateIssueComment adds a comment with body to the issue number.
func (c *Client) CreateIssueComment(owner, name string, number int, body string) (IssueComment, error) {
	var out IssueComment
	_, err := c.do("POST", c.commentsURL(owner, name, number), map[string]string{"body": body}, &out)
	return out, err
}

// UpdateIssueComment changes the body of the comment id.
func (c *Client) UpdateIssueComment(owner, name string, id int64, body string) (IssueComment, error) {
	var out IssueComment
	_, err := c.do("PATCH", c.commentURL(owner, name, id), map[string]string{"body": body}, &out)
	return out, err
}

// DeleteIssueComment deletes the comment id.
func (c *Client) DeleteIssueComment(owner, name string, id int64) error {
	_, err := c.do("DELETE", c.commentURL(owner, name, id), nil, nil)
	return err
}
