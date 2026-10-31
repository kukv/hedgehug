package fakegh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type adminBody struct {
	Repo     string   `json:"repo,omitempty"`
	Login    string   `json:"login,omitempty"`
	Perm     string   `json:"perm,omitempty"`
	Topics   []string `json:"topics,omitempty"`
	Public   bool     `json:"public,omitempty"`
	UserCode string   `json:"user_code,omitempty"`
	Token    string   `json:"token,omitempty"`
}

// admin serves /_fakegh/, what the fakegh command calls to change the state.
func (s *Server) admin(w http.ResponseWriter, r *http.Request) {
	var b adminBody
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

func (a Admin) post(path string, b adminBody) error {
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
	return a.post("/_fakegh/perm", adminBody{Repo: repo, Login: login, Perm: perm})
}

// AddRepo adds the repository owner/name.
func (a Admin) AddRepo(repo string, topics []string, public bool) error {
	return a.post("/_fakegh/repo", adminBody{Repo: repo, Topics: topics, Public: public})
}

// ApproveDevice approves the device code shown as userCode for login.
func (a Admin) ApproveDevice(userCode, login string) error {
	return a.post("/_fakegh/device/approve", adminBody{UserCode: userCode, Login: login})
}

// AddToken makes token a token of login.
func (a Admin) AddToken(token, login string) error {
	return a.post("/_fakegh/token", adminBody{Token: token, Login: login})
}

// ExpireTokens makes the access tokens issued to login expired.
func (a Admin) ExpireTokens(login string) error {
	return a.post("/_fakegh/token/expire", adminBody{Login: login})
}
