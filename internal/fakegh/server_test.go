package fakegh

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func seed() State {
	zero := 0
	return State{
		Users:  []string{"alice", "bob"},
		Orgs:   map[string][]string{"org-a": {"alice"}},
		Tokens: map[string]string{"t-alice": "alice", "t-bob": "bob"},
		Repos: []*Repo{
			{Owner: "org-a", Name: "hh-shared", Private: true, Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice": "admin", "bob": "read"}},
			{Owner: "alice", Name: "notes", Private: true},
		},
		DeviceInterval: &zero,
	}
}

// reply is the part of a response the tests look at; do reads and closes the
// body itself.
type reply struct {
	StatusCode int
	Header     http.Header
}

func do(t *testing.T, srv *httptest.Server, method, path, token, body string) (reply, string) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if strings.HasPrefix(body, "{") {
		req.Header.Set("Content-Type", "application/json")
	} else if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return reply{res.StatusCode, res.Header}, string(b)
}

func TestUserAndRepos(t *testing.T) {
	srv := httptest.NewServer(New(seed()))
	defer srv.Close()
	if res, body := do(t, srv, "GET", "/user", "t-bob", ""); res.StatusCode != 200 || !strings.Contains(body, `"login":"bob"`) {
		t.Errorf("/user = %d %s", res.StatusCode, body)
	}
	if res, _ := do(t, srv, "GET", "/user", "nope", ""); res.StatusCode != 401 {
		t.Errorf("unknown token = %d", res.StatusCode)
	}
	res, body := do(t, srv, "GET", "/user/repos?per_page=100", "t-bob", "")
	var repos []map[string]any
	if err := json.Unmarshal([]byte(body), &repos); err != nil || res.StatusCode != 200 || len(repos) != 1 {
		t.Fatalf("bob's repos = %d %s", res.StatusCode, body)
	}
	if repos[0]["full_name"] != "org-a/hh-shared" || repos[0]["permissions"].(map[string]any)["push"] != false {
		t.Errorf("repo = %v", repos[0])
	}
	if res, _ := do(t, srv, "GET", "/repos/org-a/hh-shared/collaborators", "t-bob", ""); res.StatusCode != 403 {
		t.Errorf("collaborators for a reader = %d", res.StatusCode)
	}
	res, body = do(t, srv, "GET", "/repos/org-a/hh-shared/collaborators", "t-alice", "")
	if res.StatusCode != 200 || !strings.Contains(body, `"login":"bob"`) || !strings.Contains(body, `"role_name":"read"`) {
		t.Errorf("collaborators = %d %s", res.StatusCode, body)
	}
	if res, _ := do(t, srv, "GET", "/repos/alice/notes", "t-bob", ""); res.StatusCode != 404 {
		t.Errorf("a repo bob cannot see = %d", res.StatusCode)
	}
}

func TestPages(t *testing.T) {
	st := seed()
	for i := range 3 {
		st.Repos = append(st.Repos, &Repo{Owner: "alice", Name: "r" + string(rune('0'+i))})
	}
	srv := httptest.NewServer(New(st))
	defer srv.Close()
	res, _ := do(t, srv, "GET", "/user/repos?per_page=2", "t-alice", "")
	link := res.Header.Get("Link")
	if !strings.Contains(link, `rel="next"`) || !strings.Contains(link, "page=2") {
		t.Errorf("Link = %q", link)
	}
	res, _ = do(t, srv, "GET", "/user/repos?per_page=2&page=3", "t-alice", "")
	if res.Header.Get("Link") != "" {
		t.Errorf("the last page has a next link")
	}
}

func TestCreateRepoAndTopics(t *testing.T) {
	s := New(seed())
	srv := httptest.NewServer(s)
	defer srv.Close()
	res, body := do(t, srv, "POST", "/orgs/org-a/repos", "t-alice", `{"name":"hh-new","private":true}`)
	if res.StatusCode != 201 || !strings.Contains(body, `"clone_url":"https://github.com/org-a/hh-new.git"`) {
		t.Fatalf("create = %d %s", res.StatusCode, body)
	}
	if res, body := do(t, srv, "POST", "/orgs/org-a/repos", "t-alice", `{"name":"hh-new"}`); res.StatusCode != 422 || !strings.Contains(body, "name already exists") {
		t.Errorf("again = %d %s", res.StatusCode, body)
	}
	if res, _ := do(t, srv, "POST", "/orgs/org-a/repos", "t-bob", `{"name":"x"}`); res.StatusCode != 403 {
		t.Errorf("not a member = %d", res.StatusCode)
	}
	if res, _ := do(t, srv, "PUT", "/repos/org-a/hh-new/topics", "t-alice", `{"names":["hedgehug-store"]}`); res.StatusCode != 200 {
		t.Errorf("topics = %d", res.StatusCode)
	}
	if r, ok := s.Repo("org-a/hh-new"); !ok || len(r.Topics) != 1 || r.Perms["alice"] != "admin" {
		t.Errorf("repo = %+v", r)
	}
	if res, _ := do(t, srv, "POST", "/user/repos", "t-bob", `{"name":"own"}`); res.StatusCode != 201 {
		t.Errorf("user repo = %d", res.StatusCode)
	}
}

func TestDeviceFlow(t *testing.T) {
	s := New(seed())
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	s.SetNow(func() time.Time { return now })
	srv := httptest.NewServer(s)
	defer srv.Close()
	_, body := do(t, srv, "POST", "/login/device/code", "", "client_id=Iv23x")
	var dc struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
	}
	if err := json.Unmarshal([]byte(body), &dc); err != nil || dc.DeviceCode == "" {
		t.Fatalf("device code = %s", body)
	}
	poll := url.Values{"client_id": {"Iv23x"}, "device_code": {dc.DeviceCode}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}.Encode()
	if _, body := do(t, srv, "POST", "/login/oauth/access_token", "", poll); !strings.Contains(body, "authorization_pending") {
		t.Errorf("before approval = %s", body)
	}
	if err := s.ApproveDevice(dc.UserCode, "alice"); err != nil {
		t.Fatal(err)
	}
	_, body = do(t, srv, "POST", "/login/oauth/access_token", "", poll)
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal([]byte(body), &tok); err != nil || !strings.HasPrefix(tok.AccessToken, "ghu_") || tok.ExpiresIn != 28800 {
		t.Fatalf("token = %s", body)
	}
	if _, body := do(t, srv, "GET", "/user", tok.AccessToken, ""); !strings.Contains(body, `"alice"`) {
		t.Errorf("/user with the new token = %s", body)
	}
	now = now.Add(9 * time.Hour)
	if res, _ := do(t, srv, "GET", "/user", tok.AccessToken, ""); res.StatusCode != 401 {
		t.Errorf("expired token = %d", res.StatusCode)
	}
	refresh := url.Values{"client_id": {"Iv23x"}, "refresh_token": {tok.RefreshToken}, "grant_type": {"refresh_token"}}.Encode()
	if _, body := do(t, srv, "POST", "/login/oauth/access_token", "", refresh); !strings.Contains(body, "ghu_") {
		t.Errorf("refresh = %s", body)
	}
	if _, body := do(t, srv, "POST", "/login/oauth/access_token", "", refresh); !strings.Contains(body, "bad_refresh_token") {
		t.Errorf("a used refresh token = %s", body)
	}
}

func TestAdmin(t *testing.T) {
	s := New(seed())
	srv := httptest.NewServer(s)
	defer srv.Close()
	a := Admin{URL: srv.URL}
	if err := a.SetPerm("org-a/hh-shared", "bob", "write"); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Repo("org-a/hh-shared"); r.Perms["bob"] != "write" {
		t.Errorf("perm = %v", r.Perms)
	}
	if err := a.AddRepo("org-a/hh-x", []string{"hedgehug-store"}, false); err != nil {
		t.Fatal(err)
	}
	if err := a.AddToken("t-new", "bob"); err != nil {
		t.Fatal(err)
	}
	if res, _ := do(t, srv, "GET", "/user", "t-new", ""); res.StatusCode != 200 {
		t.Errorf("added token = %d", res.StatusCode)
	}
	if err := a.SetPerm("nope/x", "bob", "read"); err == nil {
		t.Error("perm on a missing repo = nil")
	}
}

// TestAdminDeviceApproveAndExpire goes through the two endpoints the other
// tests reach by calling the server's methods: /_fakegh/device/approve and
// /_fakegh/token/expire.
func TestAdminDeviceApproveAndExpire(t *testing.T) {
	s := New(seed())
	srv := httptest.NewServer(s)
	defer srv.Close()
	a := Admin{URL: srv.URL}
	_, body := do(t, srv, "POST", "/login/device/code", "", "client_id=Iv23x")
	var dc struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
	}
	if err := json.Unmarshal([]byte(body), &dc); err != nil || dc.UserCode == "" {
		t.Fatalf("device code = %s", body)
	}
	poll := url.Values{"client_id": {"Iv23x"}, "device_code": {dc.DeviceCode}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}.Encode()
	if _, body := do(t, srv, "POST", "/login/oauth/access_token", "", poll); !strings.Contains(body, "authorization_pending") {
		t.Errorf("before approval = %s", body)
	}
	if err := a.ApproveDevice("NOPE-0000", "alice"); err == nil {
		t.Error("approving a code nobody has = nil")
	}
	if err := a.ApproveDevice(dc.UserCode, "alice"); err != nil {
		t.Fatal(err)
	}
	_, body = do(t, srv, "POST", "/login/oauth/access_token", "", poll)
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal([]byte(body), &tok); err != nil || !strings.HasPrefix(tok.AccessToken, "ghu_") {
		t.Fatalf("token after approval = %s", body)
	}
	if res, _ := do(t, srv, "GET", "/user", tok.AccessToken, ""); res.StatusCode != 200 {
		t.Errorf("issued token = %d", res.StatusCode)
	}
	// Expiring bob's tokens leaves alice's alone; expiring alice's makes the
	// access token 401 while the refresh token still works.
	if err := a.ExpireTokens("bob"); err != nil {
		t.Fatal(err)
	}
	if res, _ := do(t, srv, "GET", "/user", tok.AccessToken, ""); res.StatusCode != 200 {
		t.Errorf("after expiring bob's tokens = %d", res.StatusCode)
	}
	if err := a.ExpireTokens("alice"); err != nil {
		t.Fatal(err)
	}
	if res, _ := do(t, srv, "GET", "/user", tok.AccessToken, ""); res.StatusCode != 401 {
		t.Errorf("after expiring alice's tokens = %d, want 401", res.StatusCode)
	}
	refresh := url.Values{"client_id": {"Iv23x"}, "refresh_token": {tok.RefreshToken}, "grant_type": {"refresh_token"}}.Encode()
	if _, body := do(t, srv, "POST", "/login/oauth/access_token", "", refresh); !strings.Contains(body, "ghu_") {
		t.Errorf("refresh after expiring = %s", body)
	}
}
