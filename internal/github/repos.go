package github

import "net/url"

// Repo is a repository as the API gives it.
type Repo struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	CloneURL string `json:"clone_url"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
	Private     bool     `json:"private"`
	Topics      []string `json:"topics"`
	Permissions struct {
		Admin bool `json:"admin"`
		Push  bool `json:"push"`
		Pull  bool `json:"pull"`
	} `json:"permissions"`
}

// Collaborator is a person who may read the repository, and as what.
type Collaborator struct {
	Login    string `json:"login"`
	RoleName string `json:"role_name"`
}

// User returns the login of the token.
func (c *Client) User() (string, error) {
	var u struct {
		Login string `json:"login"`
	}
	_, err := c.do("GET", c.api+"/user", nil, &u)
	return u.Login, err
}

// CreateRepo makes the private repository owner/name: as the token's own
// when user is true, in the organization owner otherwise.
func (c *Client) CreateRepo(owner, name string, user bool) (*Repo, error) {
	path := c.api + "/orgs/" + url.PathEscape(owner) + "/repos"
	if user {
		path = c.api + "/user/repos"
	}
	var r Repo
	_, err := c.do("POST", path, map[string]any{"name": name, "private": true, "auto_init": false}, &r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// SetTopics replaces the topics of owner/name.
func (c *Client) SetTopics(owner, name string, topics []string) error {
	_, err := c.do("PUT", c.repoURL(owner, name)+"/topics", map[string][]string{"names": topics}, nil)
	return err
}

// UserRepos lists every repository the token can read: its own, those it
// collaborates on, and those of its organizations.
func (c *Client) UserRepos() ([]Repo, error) {
	return getAll[Repo](c, c.api+"/user/repos?affiliation=owner,collaborator,organization_member&per_page=100")
}

// Repo gets owner/name with the token's permissions.
func (c *Client) Repo(owner, name string) (*Repo, error) {
	var r Repo
	if _, err := c.do("GET", c.repoURL(owner, name), nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// Collaborators lists who may read owner/name. GitHub answers 403 to a token
// without push access.
func (c *Client) Collaborators(owner, name string) ([]Collaborator, error) {
	return getAll[Collaborator](c, c.repoURL(owner, name)+"/collaborators?per_page=100")
}

func (c *Client) repoURL(owner, name string) string {
	return c.api + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
}
