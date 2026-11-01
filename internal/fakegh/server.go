package fakegh

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	accessLife  = 8 * time.Hour
	refreshLife = 4416 * time.Hour // about six months
)

type device struct {
	code, userCode, login string
	expires               time.Time
}

type issued struct {
	login   string
	expires time.Time
}

// Server is the fake. It is an http.Handler.
type Server struct {
	mu      sync.Mutex
	st      State
	now     func() time.Time
	devices map[string]*device // device code → device
	access  map[string]issued  // access tokens it issued
	refresh map[string]string  // refresh token → login
	nextID  int
}

// New makes a fake with the state st.
func New(st State) *Server {
	s := &Server{now: time.Now}
	s.Reset(st)
	return s
}

// Reset replaces the state and forgets the device codes and issued tokens.
func (s *Server) Reset(st State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Keep our own copy, so the caller changing what it passed in (or a
	// second Reset with the same State) does not reach into the server.
	st.Users = slices.Clone(st.Users)
	st.Orgs = cloneLists(st.Orgs)
	st.Tokens = maps.Clone(st.Tokens)
	if st.Tokens == nil {
		st.Tokens = map[string]string{}
	}
	repos := make([]*Repo, 0, len(st.Repos))
	s.nextID = 0
	for _, r := range st.Repos {
		c := *r
		c.Topics = slices.Clone(r.Topics)
		c.Perms = maps.Clone(r.Perms)
		if c.Perms == nil {
			c.Perms = map[string]string{}
		}
		s.nextID++
		c.id = s.nextID
		repos = append(repos, &c)
	}
	st.Repos = repos
	s.st = st
	s.devices, s.access, s.refresh = map[string]*device{}, map[string]issued{}, map[string]string{}
}

// SetNow replaces the clock.
func (s *Server) SetNow(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = now
}

// Repo returns a copy of the repository owner/name.
func (s *Server) Repo(full string) (*Repo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(full)
	if r == nil {
		return nil, false
	}
	c := *r
	c.Topics = slices.Clone(r.Topics)
	c.Perms = map[string]string{}
	for k, v := range r.Perms {
		c.Perms[k] = v
	}
	return &c, true
}

func (s *Server) find(full string) *Repo {
	for _, r := range s.st.Repos {
		if strings.EqualFold(r.Owner+"/"+r.Name, full) {
			return r
		}
	}
	return nil
}

func cloneLists(m map[string][]string) map[string][]string {
	out := make(map[string][]string, len(m))
	for k, v := range m {
		out[k] = slices.Clone(v)
	}
	return out
}

// addRepo takes x in and numbers it.
func (s *Server) addRepo(x *Repo) {
	s.nextID++
	x.id = s.nextID
	s.st.Repos = append(s.st.Repos, x)
}

func (s *Server) isUser(login string) bool { return slices.Contains(s.st.Users, login) }

// perm is what login may do with r: admin, write, read or "".
func perm(r *Repo, login string) string {
	if r.Owner == login {
		return "admin"
	}
	switch p := r.Perms[login]; p {
	case "read", "write", "admin":
		return p
	}
	return ""
}

func rank(p string) int { return map[string]int{"read": 1, "write": 2, "admin": 3}[p] }

func token() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"message": msg})
}

// caller is the login of the bearer token, or "".
func (s *Server) caller(r *http.Request) string {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		tok, _ = strings.CutPrefix(r.Header.Get("Authorization"), "token ")
	}
	if tok == "" {
		return ""
	}
	if l, ok := s.st.Tokens[tok]; ok {
		return l
	}
	if is, ok := s.access[tok]; ok && s.now().Before(is.expires) {
		return is.login
	}
	return ""
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/_fakegh/") {
		s.admin(w, r)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case r.Method == "POST" && r.URL.Path == "/login/device/code":
		s.deviceCode(w, r)
		return
	case r.Method == "POST" && r.URL.Path == "/login/oauth/access_token":
		s.accessToken(w, r)
		return
	}
	login := s.caller(r)
	if login == "" {
		fail(w, http.StatusUnauthorized, "Bad credentials")
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case r.Method == "GET" && r.URL.Path == "/user":
		writeJSON(w, 200, map[string]string{"login": login})
	case r.Method == "GET" && r.URL.Path == "/user/repos":
		var out []*Repo
		for _, x := range s.st.Repos {
			if perm(x, login) != "" {
				out = append(out, x)
			}
		}
		s.page(w, r, len(out), func(i int) any { return s.repoJSON(out[i], login) })
	case r.Method == "POST" && r.URL.Path == "/user/repos":
		s.create(w, r, login, login)
	case r.Method == "POST" && len(parts) == 3 && parts[0] == "orgs" && parts[2] == "repos":
		if !slices.Contains(s.st.Orgs[parts[1]], login) {
			fail(w, http.StatusForbidden, "You need admin access to the organization before adding a repository to it.")
			return
		}
		s.create(w, r, parts[1], login)
	case len(parts) >= 3 && parts[0] == "repos":
		s.repoRoute(w, r, parts, login)
	default:
		fail(w, http.StatusNotFound, "Not Found")
	}
}

func (s *Server) repoRoute(w http.ResponseWriter, r *http.Request, parts []string, login string) {
	x := s.find(parts[1] + "/" + parts[2])
	if x == nil || perm(x, login) == "" {
		fail(w, http.StatusNotFound, "Not Found")
		return
	}
	switch {
	case r.Method == "GET" && len(parts) == 3:
		writeJSON(w, 200, s.repoJSON(x, login))
	case r.Method == "PUT" && len(parts) == 4 && parts[3] == "topics":
		if perm(x, login) != "admin" {
			fail(w, http.StatusForbidden, "Must have admin rights to Repository.")
			return
		}
		var body struct {
			Names []string `json:"names"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		x.Topics = body.Names
		writeJSON(w, 200, map[string][]string{"names": x.Topics})
	case r.Method == "GET" && len(parts) == 4 && parts[3] == "collaborators":
		if rank(perm(x, login)) < rank("write") {
			fail(w, http.StatusForbidden, "Must have push access to view repository collaborators.")
			return
		}
		type collab struct {
			Login    string `json:"login"`
			RoleName string `json:"role_name"`
		}
		var out []collab
		if s.isUser(x.Owner) {
			out = append(out, collab{x.Owner, "admin"})
		}
		names := make([]string, 0, len(x.Perms))
		for l := range x.Perms {
			names = append(names, l)
		}
		slices.Sort(names)
		for _, l := range names {
			if p := perm(x, l); p != "" && l != x.Owner {
				out = append(out, collab{l, p})
			}
		}
		s.page(w, r, len(out), func(i int) any { return out[i] })
	default:
		fail(w, http.StatusNotFound, "Not Found")
	}
}

// page writes items [from, to) of n as a JSON array, with a Link to the next
// page as GitHub does.
func (s *Server) page(w http.ResponseWriter, r *http.Request, n int, item func(int) any) {
	per, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if per <= 0 {
		per = 30
	}
	per = min(per, 100)
	pg, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if pg <= 0 {
		pg = 1
	}
	from, to := min((pg-1)*per, n), min(pg*per, n)
	out := make([]any, 0, to-from)
	for i := from; i < to; i++ {
		out = append(out, item(i))
	}
	if to < n {
		q := r.URL.Query()
		q.Set("page", strconv.Itoa(pg+1))
		w.Header().Set("Link", fmt.Sprintf(`<http://%s%s?%s>; rel="next"`, r.Host, r.URL.Path, q.Encode()))
	}
	writeJSON(w, 200, out)
}

func (s *Server) repoJSON(x *Repo, login string) map[string]any {
	p := perm(x, login)
	topics := x.Topics
	if topics == nil {
		topics = []string{}
	}
	return map[string]any{
		"id": x.id, "node_id": fmt.Sprintf("R_%d", x.id),
		"name": x.Name, "full_name": x.Owner + "/" + x.Name,
		"owner":       map[string]string{"login": x.Owner},
		"private":     x.Private,
		"html_url":    "https://github.com/" + x.Owner + "/" + x.Name,
		"clone_url":   "https://github.com/" + x.Owner + "/" + x.Name + ".git",
		"topics":      topics,
		"permissions": map[string]bool{"admin": p == "admin", "push": rank(p) >= rank("write"), "pull": p != ""},
	}
}

func (s *Server) create(w http.ResponseWriter, r *http.Request, owner, login string) {
	var body struct {
		Name    string `json:"name"`
		Private bool   `json:"private"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		fail(w, http.StatusUnprocessableEntity, "name is missing")
		return
	}
	if s.find(owner+"/"+body.Name) != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"message": "Repository creation failed.",
			"errors":  []map[string]string{{"resource": "Repository", "code": "custom", "field": "name", "message": "name already exists on this account"}},
		})
		return
	}
	x := &Repo{Owner: owner, Name: body.Name, Private: body.Private, Perms: map[string]string{}}
	if owner != login {
		x.Perms[login] = "admin"
	}
	s.addRepo(x)
	writeJSON(w, http.StatusCreated, s.repoJSON(x, login))
}

func (s *Server) deviceCode(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || r.PostForm.Get("client_id") == "" {
		writeJSON(w, 200, map[string]string{"error": "unauthorized_client"})
		return
	}
	d := &device{code: token(), userCode: strings.ToUpper(token()[:4] + "-" + token()[:4]), expires: s.now().Add(15 * time.Minute)}
	if s.st.AutoApprove != "" {
		d.login = s.st.AutoApprove
	}
	s.devices[d.code] = d
	interval := 5
	if s.st.DeviceInterval != nil {
		interval = *s.st.DeviceInterval
	}
	writeJSON(w, 200, map[string]any{
		"device_code": d.code, "user_code": d.userCode,
		"verification_uri": "http://" + r.Host + "/login/device",
		"expires_in":       900, "interval": interval,
	})
}

func (s *Server) issue(w http.ResponseWriter, login string) {
	at, rt := "ghu_"+token(), "ghr_"+token()
	s.access[at] = issued{login, s.now().Add(accessLife)}
	s.refresh[rt] = login
	writeJSON(w, 200, map[string]any{
		"access_token": at, "token_type": "bearer", "scope": "",
		"expires_in": int(accessLife.Seconds()), "refresh_token": rt, "refresh_token_expires_in": int(refreshLife.Seconds()),
	})
}

func (s *Server) accessToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, 200, map[string]string{"error": "invalid_request"})
		return
	}
	f := r.PostForm
	switch f.Get("grant_type") {
	case "urn:ietf:params:oauth:grant-type:device_code":
		d, ok := s.devices[f.Get("device_code")]
		switch {
		case !ok:
			writeJSON(w, 200, map[string]string{"error": "bad_verification_code"})
		case s.now().After(d.expires):
			writeJSON(w, 200, map[string]string{"error": "expired_token"})
		case d.login == "":
			writeJSON(w, 200, map[string]string{"error": "authorization_pending"})
		default:
			delete(s.devices, d.code)
			s.issue(w, d.login)
		}
	case "refresh_token":
		login, ok := s.refresh[f.Get("refresh_token")]
		if !ok {
			writeJSON(w, 200, map[string]string{"error": "bad_refresh_token"})
			return
		}
		delete(s.refresh, f.Get("refresh_token"))
		s.issue(w, login)
	default:
		writeJSON(w, 200, map[string]string{"error": "unsupported_grant_type"})
	}
}

// SetPerm sets what login may do with repo (owner/name); none takes it away.
func (s *Server) SetPerm(repo, login, p string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	x := s.find(repo)
	if x == nil {
		return fmt.Errorf("no repository %s", repo)
	}
	if !slices.Contains([]string{"read", "write", "admin", "none"}, p) {
		return fmt.Errorf("perm %q: want read, write, admin or none", p)
	}
	x.Perms[login] = p
	return nil
}

// AddRepo adds the repository owner/name.
func (s *Server) AddRepo(repo string, topics []string, public bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return fmt.Errorf("repo %q: want owner/name", repo)
	}
	if s.find(repo) != nil {
		return fmt.Errorf("%s exists", repo)
	}
	s.addRepo(&Repo{Owner: owner, Name: name, Private: !public, Topics: slices.Clone(topics), Perms: map[string]string{}})
	return nil
}

// ApproveDevice approves the device code shown as userCode for login.
func (s *Server) ApproveDevice(userCode, login string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.devices {
		if d.userCode == userCode {
			d.login = login
			return nil
		}
	}
	return errors.New("no device code " + userCode)
}

// AddToken makes token a token of login.
func (s *Server) AddToken(tok, login string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.Tokens[tok] = login
}

// ExpireTokens makes the access tokens issued to login expired, so a
// refresh is needed.
func (s *Server) ExpireTokens(login string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, is := range s.access {
		if is.login == login {
			is.expires = s.now().Add(-time.Second)
			s.access[k] = is
		}
	}
}
