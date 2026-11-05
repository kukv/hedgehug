package fakegh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AdminBody is the JSON body of the /_fakegh/ endpoints.
type AdminBody struct {
	Repo     string   `json:"repo,omitempty"`
	Login    string   `json:"login,omitempty"`
	Perm     string   `json:"perm,omitempty"`
	Topics   []string `json:"topics,omitempty"`
	Public   bool     `json:"public,omitempty"`
	UserCode string   `json:"user_code,omitempty"`
	Token    string   `json:"token,omitempty"`
	Number   int      `json:"number,omitempty"`
	Head     string   `json:"head,omitempty"`
	Title    string   `json:"title,omitempty"`
	Body     string   `json:"body,omitempty"`
	Draft    bool     `json:"draft,omitempty"`
	State    string   `json:"state,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	Request  bool     `json:"request,omitempty"`
	// What fakegh issue edit sets. Nil or empty stays as it is; "-" takes the
	// milestone off.
	Assignees []string `json:"assignees,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Milestone string   `json:"milestone,omitempty"`
	// What fakegh rate set sets.
	Remaining int `json:"remaining,omitempty"`
	Limit     int `json:"limit,omitempty"`
	ResetIn   int `json:"reset_in,omitempty"` // seconds from now
}

// admin serves /_fakegh/, what the fakegh command calls to change the state.
func (s *Server) admin(w http.ResponseWriter, r *http.Request) {
	var b AdminBody
	if r.Method != "POST" || json.NewDecoder(r.Body).Decode(&b) != nil {
		fail(w, http.StatusBadRequest, "want POST with a JSON body")
		return
	}
	var err error
	switch r.URL.Path {
	case "/_fakegh/perm":
		err = s.SetPerm(b.Repo, b.Login, b.Perm)
	case "/_fakegh/repo":
		err = s.AddRepo(b.Repo, b.Topics, b.Public)
	case "/_fakegh/device/approve":
		err = s.ApproveDevice(b.UserCode, b.Login)
	case "/_fakegh/token":
		if b.Token == "" || b.Login == "" {
			fail(w, http.StatusBadRequest, "token and login are required")
			return
		}
		s.AddToken(b.Token, b.Login)
	case "/_fakegh/token/expire":
		s.ExpireTokens(b.Login)
	case "/_fakegh/pr/open":
		_, err = s.OpenPull(b.Repo, b.Head, b.Title, b.Body, b.Draft)
	case "/_fakegh/pr/merge":
		err = s.MergePull(b.Repo, b.Number)
	case "/_fakegh/pr/review":
		if b.Request {
			err = s.RequestReview(b.Repo, b.Number)
		} else {
			err = s.ReviewPull(b.Repo, b.Number, b.State)
		}
	case "/_fakegh/pr/checks":
		err = s.SetChecks(b.Repo, b.Number, b.State)
	case "/_fakegh/pr/comment":
		err = s.CommentPull(b.Repo, b.Number, b.Body)
	case "/_fakegh/issue/open":
		_, err = s.OpenIssue(b.Repo, b.Title, b.Body)
	case "/_fakegh/issue/close":
		err = s.CloseIssue(b.Repo, b.Number, b.Reason)
	case "/_fakegh/issue/edit":
		err = s.EditIssue(b.Repo, b.Number, b.Title, b.Body)
		if err == nil {
			err = s.TagIssue(b.Repo, b.Number, b.Assignees, b.Labels, b.Milestone)
		}
	case "/_fakegh/restrict":
		s.RestrictApp(b.Repo)
	case "/_fakegh/uninstall":
		s.UninstallApp(b.Repo)
	case "/_fakegh/rate":
		s.SetRateLimit(b.Remaining, b.Limit, time.Now().Add(time.Duration(b.ResetIn)*time.Second))
	case "/_fakegh/label":
		err = s.AddLabel(b.Repo, b.Title)
	case "/_fakegh/milestone":
		err = s.AddMilestone(b.Repo, b.Title)
	default:
		fail(w, http.StatusNotFound, "Not Found")
		return
	}
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// Admin calls the /_fakegh/ endpoints of a running fake.
type Admin struct {
	URL  string
	HTTP *http.Client
}

func (a Admin) post(path string, b AdminBody) error {
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	hc := a.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	res, err := hc.Post(a.URL+path, "application/json", bytes.NewReader(data)) //nolint:gosec // G107, G704: the URL is the running fake the developer named with --server
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != 200 {
		msg, _ := io.ReadAll(res.Body)
		return fmt.Errorf("%s: %d %s", path, res.StatusCode, bytes.TrimSpace(msg))
	}
	return nil
}

// SetPerm sets what login may do with repo (owner/name).
func (a Admin) SetPerm(repo, login, perm string) error {
	return a.post("/_fakegh/perm", AdminBody{Repo: repo, Login: login, Perm: perm})
}

// AddRepo adds the repository owner/name.
func (a Admin) AddRepo(repo string, topics []string, public bool) error {
	return a.post("/_fakegh/repo", AdminBody{Repo: repo, Topics: topics, Public: public})
}

// ApproveDevice approves the device code shown as userCode for login.
func (a Admin) ApproveDevice(userCode, login string) error {
	return a.post("/_fakegh/device/approve", AdminBody{UserCode: userCode, Login: login})
}

// AddToken makes token a token of login.
func (a Admin) AddToken(token, login string) error {
	return a.post("/_fakegh/token", AdminBody{Token: token, Login: login})
}

// ExpireTokens makes the access tokens issued to login expired.
func (a Admin) ExpireTokens(login string) error {
	return a.post("/_fakegh/token/expire", AdminBody{Login: login})
}

// AddLabel adds a label to repo.
func (a Admin) AddLabel(repo, name string) error {
	return a.post("/_fakegh/label", AdminBody{Repo: repo, Title: name})
}

// AddMilestone adds a milestone to repo.
func (a Admin) AddMilestone(repo, title string) error {
	return a.post("/_fakegh/milestone", AdminBody{Repo: repo, Title: title})
}

// SetRateLimit sets what GraphQL says of the rate limit; it resets resetIn from now.
func (a Admin) SetRateLimit(remaining, limit int, resetIn time.Duration) error {
	return a.post("/_fakegh/rate", AdminBody{Remaining: remaining, Limit: limit, ResetIn: int(resetIn.Seconds())})
}

// Pull calls /_fakegh/pr/<what> with b; Issue calls /_fakegh/issue/<what>.
// RestrictApp makes GitHub answer as for a GitHub App not installed on owner.
func (a Admin) RestrictApp(owner string) error {
	return a.post("/_fakegh/restrict", AdminBody{Repo: owner})
}

// UninstallApp makes GitHub hide the repositories of owner from the App.
func (a Admin) UninstallApp(owner string) error {
	return a.post("/_fakegh/uninstall", AdminBody{Repo: owner})
}

func (a Admin) Pull(what string, b AdminBody) error  { return a.post("/_fakegh/pr/"+what, b) }
func (a Admin) Issue(what string, b AdminBody) error { return a.post("/_fakegh/issue/"+what, b) }
