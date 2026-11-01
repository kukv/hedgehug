package github

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/config"
)

// brokenBody answers 200 with a Content-Length it does not deliver, so that
// reading the body fails.
func brokenBody() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"login"`))
	}))
}

func TestReadFailure(t *testing.T) {
	srv := brokenBody()
	defer srv.Close()
	c := New(config.Host{API: srv.URL}, "t", nil)
	if _, err := c.User(); err == nil {
		t.Error("User with a truncated body is an error")
	}
	if err := c.SetTopics("o", "n", nil); err == nil {
		t.Error("SetTopics (no out) with a truncated body is an error")
	}
	if _, err := RequestDeviceCode(nil, srv.URL, "c"); err == nil {
		t.Error("RequestDeviceCode with a truncated body is an error")
	}
}

func TestNextPageOtherHost(t *testing.T) {
	var gotAuth string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Link", "<"+other.URL+"/user/repos?page=2>; rel=\"next\"")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	_, err := New(config.Host{API: srv.URL}, "secret", nil).UserRepos()
	if err == nil || !strings.Contains(err.Error(), "not on the API host") ||
		!strings.Contains(err.Error(), "GET "+srv.URL) || !strings.Contains(err.Error(), other.URL+"/user/repos?page=2") {
		t.Errorf("err = %v, want it to name the page asked and the next page", err)
	}
	if gotAuth != "" {
		t.Errorf("the token was sent to the other host: %q", gotAuth)
	}
}
