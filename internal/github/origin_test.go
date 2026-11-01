package github

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/config"
)

// A next page on the same host but another port is another origin: the token
// is not sent there.
func TestNextPageOtherPort(t *testing.T) {
	t.Parallel()
	var gotAuth string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer other.Close()
	otherURL, _ := url.Parse(other.URL)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Link", "<"+other.URL+"/user/repos?page=2>; rel=\"next\"")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	srvURL, _ := url.Parse(srv.URL)
	if srvURL.Hostname() != otherURL.Hostname() || srvURL.Port() == otherURL.Port() {
		t.Fatalf("the servers should differ only by port: %s %s", srv.URL, other.URL)
	}
	_, err := New(config.Host{API: srv.URL}, "secret", nil).UserRepos()
	if err == nil || !strings.Contains(err.Error(), "not sending the token") {
		t.Errorf("err = %v", err)
	}
	if gotAuth != "" {
		t.Errorf("the token was sent to the other port: %q", gotAuth)
	}
}

func TestSameOrigin(t *testing.T) {
	t.Parallel()
	c := &Client{api: "https://api.github.com"}
	for u, want := range map[string]bool{
		"https://api.github.com/user/repos?page=2":     true,
		"https://API.github.com/user/repos?page=2":     true,
		"https://api.github.com:443/user/repos?page=2": true,
		"https://api.github.com:8443/user/repos":       false,
		"http://api.github.com/user/repos":             false,
		"https://github.com/user/repos":                false,
		"https://api.github.com.evil.test/user/repos":  false,
		"://bad": false,
	} {
		if got := c.sameOrigin(u); got != want {
			t.Errorf("sameOrigin(%q) = %v; want %v", u, got, want)
		}
	}
	c = &Client{api: "http://ghe.example.com:8080/api/v3"}
	if !c.sameOrigin("http://ghe.example.com:8080/api/v3/repos?page=2") || c.sameOrigin("http://ghe.example.com/api/v3/repos") {
		t.Error("an explicit port is part of the origin")
	}
}

// redirecting answers every request with a redirect to to+path.
func redirecting(to string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, to+r.URL.Path, http.StatusTemporaryRedirect)
	}))
}

// A redirect to another origin is refused, for the REST client (with the
// default client and with an injected one) and for the device flow, and the
// other server sees no request at all.
func TestRedirectToAnotherOriginIsRefused(t *testing.T) {
	t.Parallel()
	var seen []string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization")+"|"+r.URL.Path)
		_, _ = w.Write([]byte(`{"login":"x"}`))
	}))
	defer other.Close()
	srv := redirecting(other.URL)
	defer srv.Close()

	injected := &http.Client{}
	for name, hc := range map[string]*http.Client{"default": nil, "injected": injected} {
		_, err := New(config.Host{API: srv.URL}, "secret", hc).User()
		if err == nil || !strings.Contains(err.Error(), "redirect") {
			t.Errorf("%s: err = %v; want a refused redirect", name, err)
		}
	}
	if injected.CheckRedirect != nil {
		t.Error("the injected client was changed")
	}
	if _, err := RequestDeviceCode(nil, srv.URL, "c"); err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Errorf("device flow: err = %v", err)
	}
	if len(seen) != 0 {
		t.Errorf("the other origin got requests: %v", seen)
	}
}

func TestRedirectToTheSameOriginIsFollowed(t *testing.T) {
	t.Parallel()
	var auths []string
	mux := http.NewServeMux()
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/user2", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/user2", func(w http.ResponseWriter, r *http.Request) {
		auths = append(auths, r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"login":"alice"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	for _, hc := range []*http.Client{nil, {}} {
		login, err := New(config.Host{API: srv.URL}, "secret", hc).User()
		if err != nil || login != "alice" {
			t.Errorf("User = %q, %v", login, err)
		}
	}
	for _, a := range auths {
		if a != "Bearer secret" {
			t.Errorf("Authorization = %q", a)
		}
	}
}

// A client of the caller that has a policy of its own keeps it.
func TestRedirectKeepsTheCallersPolicy(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			http.Redirect(w, r, "/user2", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(`{"login":"alice"}`))
	}))
	defer srv.Close()
	hc := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("no redirects") }}
	if _, err := New(config.Host{API: srv.URL}, "t", hc).User(); err == nil || !strings.Contains(err.Error(), "no redirects") {
		t.Errorf("the caller's policy was not kept: %v", err)
	}
}
